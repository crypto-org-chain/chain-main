import hashlib
import json
import subprocess
import time
from pathlib import Path

import pytest
from pystarport import ports

from .utils import cluster_fixture, find_balance, wait_for_block, wait_for_port

pytestmark = pytest.mark.ibc


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/ibc_channel_genesis.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def create_ibc_client(data_root, dst_chain, src_chain):
    subprocess.run(
        [
            "hermes",
            "--config",
            data_root / "relayer.toml",
            "create",
            "client",
            "--host-chain",
            dst_chain,
            "--reference-chain",
            src_chain,
        ],
        check=True,
    )


def test_ibc_genesis_channel(cluster):
    for cli in cluster.values():
        for i in range(cli.nodes_len()):
            wait_for_port(ports.grpc_port(cli.base_port(i)))

    for cli in cluster.values():
        # wait for at least 3 blocks, because
        # "proof queries at height <= 2 are not supported"
        wait_for_block(cli, 3)

    data_root = next(iter(cluster.values())).data_root
    create_ibc_client(data_root, "ibc-1", "ibc-0")
    create_ibc_client(data_root, "ibc-0", "ibc-1")
    cluster["ibc-0"].supervisor.startProcess("relayer-demo")

    raw = cluster["ibc-0"].cosmos_cli().raw
    transfer_amount = 10000

    addr_0 = cluster["ibc-0"].address("relayer")
    addr_1 = cluster["ibc-1"].address("relayer")

    assert cluster["ibc-0"].balance(addr_0) == 10000000000
    assert cluster["ibc-1"].balance(addr_1) == 10000000000

    # # do a transfer from ibc-0 to ibc-1
    rsp = cluster["ibc-0"].ibc_transfer(
        "relayer", addr_1, "%dbasecro" % transfer_amount, "channel-0", 1
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    # sender balance decreased
    assert cluster["ibc-0"].balance(addr_0) == 9999990000
    # FIXME more stable way to wait for relaying
    time.sleep(10)
    query_txs_0 = cluster["ibc-0"].query_all_txs(addr_0)
    assert len(query_txs_0["txs"]) == 1
    query_txs_1 = cluster["ibc-0"].query_all_txs(addr_1)
    assert len(query_txs_1["txs"]) == 1
    query_txs_2 = cluster["ibc-1"].query_all_txs(addr_1)
    assert len(query_txs_2["txs"]) == 1

    denom_hash = (
        hashlib.sha256("transfer/channel-0/basecro".encode()).hexdigest().upper()
    )
    assert json.loads(
        raw(
            "query",
            "ibc-transfer",
            "denom-trace",
            denom_hash,
            node=cluster["ibc-1"].node_rpc(0),
            output="json",
        )
    ) == {"denom_trace": {"path": "transfer/channel-0", "base_denom": "basecro"}}
    # # recipient get the coins
    assert json.loads(
        raw(
            "query",
            "bank",
            "balances",
            addr_1,
            output="json",
            node=cluster["ibc-1"].node_rpc(0),
        )
    )["balances"] == [
        {"denom": "basecro", "amount": "10000000000"},
        {
            "denom": "ibc/%s" % denom_hash,
            "amount": "%d" % (transfer_amount + 100),  # 100 is allocated in genesis
        },
    ]

    # transfer back
    rsp = cluster["ibc-1"].ibc_transfer(
        "relayer",
        addr_0,
        "%dibc/%s" % (transfer_amount, denom_hash),
        "channel-0",
        0,
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # FIXME more stable way to wait for relaying
    time.sleep(40)
    query_txs_0 = cluster["ibc-0"].query_all_txs(addr_0)
    assert len(query_txs_0["txs"]) == 2
    query_txs_1 = cluster["ibc-1"].query_all_txs(addr_0)
    assert len(query_txs_1["txs"]) == 1
    query_txs_2 = cluster["ibc-1"].query_all_txs(addr_1)
    assert len(query_txs_2["txs"]) == 2

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
        assert find_balance(balances, "basecro") == 10000000000
