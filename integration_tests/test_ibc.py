import hashlib
import json
import subprocess
import time
from pathlib import Path

import pytest

from .utils import cluster_fixture, wait_for_block


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/ibc.yaml",
        worker_index,
        tmp_path_factory,
        quiet=pytestconfig.getoption("supervisord-quiet"),
    )


def test_ibc(cluster):
    for cli in cluster.values():
        # wait for at least 3 blocks, because
        # "proof queries at height <= 2 are not supported"
        wait_for_block(cli, 3)
    # all clusters share the same root data directory
    data_root = next(iter(cluster.values())).data_root
    # call chain-maind directly
    raw = next(iter(cluster.values())).cosmos_cli().raw
    relayer = ["relayer", "--home", data_root / "relayer"]
    # init light clients
    for chain_id in cluster:
        subprocess.run(
            relayer
            + [
                "light",
                "init",
                chain_id,
                "-f",
            ],
            check=True,
        )

    # check status
    rsp = json.loads(subprocess.check_output(relayer + ["chains", "list", "--json"]))
    assert rsp == [
        {
            "key": "relayer",
            "chain-id": "ibc-0",
            "rpc-addr": "http://localhost:26657",
            "account-prefix": "cro",
            "gas-adjustment": 1.5,
            "gas-prices": "0.0basecro",
            "trusting-period": "336h",
        },
        {
            "key": "relayer",
            "chain-id": "ibc-1",
            "rpc-addr": "http://localhost:26757",
            "account-prefix": "cro",
            "gas-adjustment": 1.5,
            "gas-prices": "0.0basecro",
            "trusting-period": "336h",
        },
    ]

    # wait for channel to be setup
    for i in range(60):
        channels = json.loads(
            raw(
                "query",
                "ibc",
                "channel",
                "channels",
                node=cluster["ibc-0"].node_rpc(0),
                output="json",
            )
        )["channels"]
        print("channels", channels)
        if channels and channels[0]["state"] == "STATE_OPEN":
            break
        time.sleep(1)
    else:
        raise TimeoutError("src channel still not setup")

    rsp = json.loads(subprocess.check_output(relayer + ["paths", "list", "--json"]))
    assert rsp == {
        "demo": {
            "src": {
                "chain-id": "ibc-0",
                "client-id": "07-tendermint-0",
                "connection-id": "connection-0",
                "port-id": "transfer",
                "order": "unordered",
                "version": "ics20-1",
            },
            "dst": {
                "chain-id": "ibc-1",
                "client-id": "07-tendermint-0",
                "connection-id": "connection-0",
                "port-id": "transfer",
                "order": "unordered",
                "version": "ics20-1",
            },
            "strategy": {
                "type": "naive",
            },
        }
    }

    for i in range(60):
        channels = json.loads(
            raw(
                "query",
                "ibc",
                "channel",
                "channels",
                node=cluster["ibc-1"].node_rpc(0),
                output="json",
            )
        )["channels"]
        print("channels", channels)
        if channels and channels[0]["state"] == "STATE_OPEN":
            break
        time.sleep(1)
    else:
        raise TimeoutError("dst channel still not setup")

    # query balance of relayer account
    for chain_id in cluster:
        assert (
            subprocess.check_output(relayer + ["query", "balance", chain_id])
            .strip()
            .decode()
        ) == "10000000000basecro"

    # do a transfer from ibc-0 to ibc-1
    recipient = cluster["ibc-1"].address("relayer")
    rsp = cluster["ibc-0"].ibc_transfer(
        "relayer", recipient, "10000basecro", channels[0]["channel_id"], 1
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    # sender balance decreased
    assert cluster["ibc-0"].balance(cluster["ibc-0"].address("relayer")) == 9999990000
    print("ibc transfer")

    # FIXME more stable way to wait for relaying
    time.sleep(10)

    dst_channel = channels[0]["counterparty"]["channel_id"]
    denom_hash = (
        hashlib.sha256(f"transfer/{dst_channel}/basecro".encode()).hexdigest().upper()
    )
    assert (
        json.loads(
            raw(
                "query",
                "ibc-transfer",
                "denom-trace",
                denom_hash,
                node=cluster["ibc-1"].node_rpc(0),
                output="json",
            )
        )
        == {"denom_trace": {"path": f"transfer/{dst_channel}", "base_denom": "basecro"}}
    )
    # recipient get the coins
    assert json.loads(
        raw(
            "query",
            "bank",
            "balances",
            recipient,
            output="json",
            node=cluster["ibc-1"].node_rpc(0),
        )
    )["balances"] == [
        {"denom": "basecro", "amount": "10000000000"},
        {
            "denom": f"ibc/{denom_hash}",
            "amount": "10000",
        },
    ]

    # transfer back
    recipient = cluster["ibc-0"].address("relayer")
    rsp = cluster["ibc-1"].ibc_transfer(
        "relayer", recipient, f"10000ibc/{denom_hash}", dst_channel, 0
    )
    print("ibc transfer back")
    assert rsp["code"] == 0, rsp["raw_log"]

    # FIXME more stable way to wait for relaying
    time.sleep(40)

    # both accounts return to normal
    for i, cli in enumerate(cluster.values()):
        balances = json.loads(
            raw(
                "query",
                "bank",
                "balances",
                cli.address("relayer"),
                output="json",
                node=cli.node_rpc(0),
            )
        )["balances"]
        assert [bal for bal in balances if int(bal["amount"]) > 0] == [
            {"amount": "10000000000", "denom": "basecro"},
        ]
