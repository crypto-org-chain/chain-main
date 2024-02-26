import hashlib
import json
import re
import subprocess

from pystarport import ports

from .utils import wait_for_block, wait_for_fn, wait_for_port


def wait_relayer_ready(cluster):
    for cli in cluster.values():
        for i in range(cli.nodes_len()):
            wait_for_port(ports.grpc_port(cli.base_port(i)))

    for cli in cluster.values():
        # wait for at least 3 blocks, because
        # "proof queries at height <= 2 are not supported"
        wait_for_block(cli, 3)

    # all clusters share the same root data directory
    data_root = next(iter(cluster.values())).data_root
    return ["hermes", "--config", data_root / "relayer.toml"]


def search_target(query, key, chains):
    results = []
    for chain in chains:
        raw = subprocess.check_output(query + [chain]).decode("utf-8")
        results.append(re.search(r"" + key + r"-\d*", raw).group())
    return results


def start_and_wait_relayer(
    cluster,
    port="transfer",
    chains=["ibc-0", "ibc-1"],
    start_relaying=True,
    init_relayer=True,
):
    relayer = wait_relayer_ready(cluster)
    if init_relayer:
        # create connection and channel
        subprocess.run(
            relayer
            + [
                "create",
                "channel",
                "--a-port",
                port,
                "--b-port",
                port,
                "--a-chain",
                chains[0],
                "--b-chain",
                chains[1],
                "--new-client-connection",
                "--yes",
            ],
            check=True,
        )

        # start relaying
        if start_relaying:
            cluster[chains[0]].supervisor.startProcess("relayer-demo")

    query = relayer + ["query", "channels", "--chain"]
    return search_target(query, "channel", chains)


def ibc_transfer_flow(cluster, src_channel, dst_channel):
    # call chain-maind directly
    raw = cluster["ibc-0"].cosmos_cli().raw
    denom = "basecro"
    amt = 10000
    origin = 10000000000

    addr_0 = cluster["ibc-0"].address("relayer")
    addr_1 = cluster["ibc-1"].address("relayer")

    assert cluster["ibc-0"].balance(addr_0) == origin
    assert cluster["ibc-1"].balance(addr_1) == origin

    # do a transfer from ibc-0 to ibc-1
    rsp = cluster["ibc-0"].ibc_transfer(
        "relayer", addr_1, f"{amt}{denom}", src_channel, 1
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    # sender balance decreased
    assert cluster["ibc-0"].balance(addr_0) == origin - amt
    denom_hash = (
        hashlib.sha256(f"transfer/{dst_channel}/{denom}".encode()).hexdigest().upper()
    )
    ibc_denom = f"ibc/{denom_hash}"
    old_dst_balance = cluster["ibc-1"].balance(addr_1, ibc_denom)
    new_dst_balance = 0

    def check_balance_change():
        nonlocal new_dst_balance
        new_dst_balance = cluster["ibc-1"].balance(addr_1, ibc_denom)
        return new_dst_balance != old_dst_balance

    wait_for_fn("balance change", check_balance_change)
    # recipient get the coins
    assert new_dst_balance == amt + old_dst_balance, new_dst_balance
    assert json.loads(
        raw(
            "query",
            "ibc-transfer",
            "denom-trace",
            denom_hash,
            node=cluster["ibc-1"].node_rpc(0),
            output="json",
        )
    ) == {"denom_trace": {"path": f"transfer/{dst_channel}", "base_denom": denom}}

    # transfer back
    rsp = cluster["ibc-1"].ibc_transfer(
        "relayer", addr_0, f"{amt}{ibc_denom}", dst_channel, 0
    )
    print("ibc transfer back")
    assert rsp["code"] == 0, rsp["raw_log"]

    old_src_balance = cluster["ibc-0"].balance(addr_0, denom)
    new_src_balance = 0

    def check_balance_change():
        nonlocal new_src_balance
        new_src_balance = cluster["ibc-0"].balance(addr_0, denom)
        return new_src_balance != old_src_balance

    wait_for_fn("balance change", check_balance_change)

    # both accounts return to normal
    for i, cli in enumerate(cluster.values()):
        assert cli.balance(cli.address("relayer"), denom=denom) == origin
