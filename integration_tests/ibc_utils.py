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


def register_fee_payee(src_chain, dst_chain):
    rsp = dst_chain.register_counterparty_payee(
        "transfer",
        "channel-0",
        dst_chain.address("relayer"),
        src_chain.address("relayer"),
        from_="relayer",
        fees="1basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]


def start_and_wait_relayer(
    cluster,
    port="transfer",
    chains=["ibc-0", "ibc-1"],
    start_relaying=True,
    init_relayer=True,
    incentivized=False,
):
    relayer = wait_relayer_ready(cluster)
    version = {"fee_version": "ics29-1", "app_version": "ics20-1"}
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
            ]
            + (
                [
                    "--channel-version",
                    json.dumps(version),
                ]
                if incentivized
                else []
            ),
            check=True,
        )

        # start relaying
        if start_relaying:
            cluster[chains[0]].supervisor.startProcess("relayer-demo")
            if incentivized:
                register_fee_payee(cluster[chains[0]], cluster[chains[1]])

    query = relayer + ["query", "channels", "--chain"]
    return search_target(query, "channel", chains)


def ibc_transfer_flow(cluster, src_channel, dst_channel):
    # call chain-maind directly
    raw = cluster["ibc-0"].cosmos_cli().raw
    denom = "basecro"
    amt = 10000
    addr_0 = cluster["ibc-0"].address("relayer")
    addr_1 = cluster["ibc-1"].address("relayer")
    origin0 = cluster["ibc-0"].balance(addr_0)
    origin1 = cluster["ibc-1"].balance(addr_1)

    # do a transfer from ibc-0 to ibc-1
    rsp = cluster["ibc-0"].ibc_transfer(
        "relayer", addr_1, f"{amt}{denom}", src_channel, 1
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    # sender balance decreased
    assert cluster["ibc-0"].balance(addr_0) == origin0 - amt
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
    assert cluster["ibc-0"].balance(addr_0, denom=denom) == origin0
    assert cluster["ibc-1"].balance(addr_1, denom=denom) == origin1


def find_log_event_attrs(events, ev_type, cond=None):
    for ev in events:
        if ev["type"] == ev_type:
            attrs = {attr["key"]: attr["value"] for attr in ev["attributes"]}
            if cond is None or cond(attrs):
                return attrs
    return None


def ibc_incentivized_transfer(cluster):
    chains = [cluster["ibc-0"].cosmos_cli(), cluster["ibc-1"].cosmos_cli()]
    receiver = chains[1].address("signer")
    sender = chains[0].address("signer2")
    relayer = chains[0].address("relayer")
    amount = 1000
    fee_denom = "ibcfee"
    base_denom = "basecro"
    old_amt_fee = chains[0].balance(relayer, fee_denom)
    old_amt_sender_fee = chains[0].balance(sender, fee_denom)
    old_amt_sender_base = chains[0].balance(sender, base_denom)
    old_amt_receiver_base = chains[1].balance(receiver, "basecro")
    current = chains[1].balances(receiver)
    assert old_amt_sender_base == 200000000000
    assert old_amt_receiver_base == 20000000000
    src_channel = "channel-0"
    dst_channel = "channel-0"
    rsp = chains[0].ibc_transfer(
        sender,
        receiver,
        f"{amount}{base_denom}",
        src_channel,
        1,
        fees="0basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = chains[0].event_query_tx_for(rsp["txhash"])

    def cb(attrs):
        return "packet_sequence" in attrs

    evt = find_log_event_attrs(rsp["events"], "send_packet", cb)
    print("packet event", evt)
    packet_seq = int(evt["packet_sequence"])
    fee = f"10{fee_denom}"
    rsp = chains[0].pay_packet_fee(
        "transfer",
        src_channel,
        packet_seq,
        recv_fee=fee,
        ack_fee=fee,
        timeout_fee=fee,
        from_=sender,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    # fee is locked
    current = chains[0].balance(sender, fee_denom)
    # https://github.com/cosmos/ibc-go/pull/5571
    assert current == old_amt_sender_fee - 20, current

    # wait for relayer receive the fee
    def check_fee():
        amt = chains[0].balance(relayer, fee_denom)
        if amt > old_amt_fee:
            assert amt == old_amt_fee + 20, amt
            return True
        else:
            return False

    wait_for_fn("wait for relayer to receive the fee", check_fee)

    # timeout fee is refunded
    actual = chains[0].balances(sender)
    assert actual == [
        {"denom": base_denom, "amount": f"{old_amt_sender_base - amount}"},
        {"denom": fee_denom, "amount": f"{old_amt_sender_fee - 20}"},
    ], actual
    path = f"transfer/{dst_channel}/{base_denom}"
    denom_hash = hashlib.sha256(path.encode()).hexdigest().upper()
    denom_trace = chains[0].ibc_denom_trace(path, cluster["ibc-1"].node_rpc(0))
    assert denom_trace == {"denom":{"base":base_denom, "trace":[{"port_id":"transfer","channel_id": dst_channel}]}}

    current = chains[1].balances(receiver)
    assert current == [
        {"denom": "basecro", "amount": f"{old_amt_receiver_base}"},
        {"denom": f"ibc/{denom_hash}", "amount": f"{amount}"},
    ], current
    # transfer back
    fee_amount = 100000000
    rsp = chains[1].ibc_transfer(
        receiver,
        sender,
        f"{amount}ibc/{denom_hash}",
        dst_channel,
        1,
        fees=f"{fee_amount}basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    def check_balance_change():
        return chains[0].balance(sender, base_denom) != old_amt_sender_base - amount

    wait_for_fn("balance change", check_balance_change)
    actual = chains[0].balance(sender, base_denom)
    assert actual == old_amt_sender_base, actual
    current = chains[1].balance(receiver, "basecro")
    assert current == old_amt_receiver_base - fee_amount
    return amount, packet_seq


def wait_for_check_channel_ready(cli, connid, channel_id, target="STATE_OPEN"):
    print("wait for channel ready", channel_id, target)

    def check_channel_ready():
        channels = cli.ibc_query_channels(connid)["channels"]
        try:
            state = next(
                channel["state"]
                for channel in channels
                if channel["channel_id"] == channel_id
            )
        except StopIteration:
            return False
        return state == target

    wait_for_fn("channel ready", check_channel_ready)
