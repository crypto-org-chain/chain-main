import hashlib
import json
import subprocess
import time
from pathlib import Path

import pytest
from pystarport import ports

from .utils import cluster_fixture, wait_for_block, wait_for_port

pytestmark = pytest.mark.ibc


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/ibc.yaml",
        worker_index,
        tmp_path_factory.mktemp("data"),
        quiet=pytestconfig.getoption("supervisord-quiet"),
    )


def start_and_wait_relayer(cluster, init_relayer=True):
    for cli in cluster.values():
        for i in range(cli.nodes_len()):
            wait_for_port(ports.grpc_port(cli.base_port(i)))

    for cli in cluster.values():
        # wait for at least 3 blocks, because
        # "proof queries at height <= 2 are not supported"
        wait_for_block(cli, 3)

    # all clusters share the same root data directory
    data_root = next(iter(cluster.values())).data_root
    relayer = ["hermes", "-j", "-c", data_root / "relayer.toml"]

    if init_relayer:
        # create connection and channel
        subprocess.run(
            relayer
            + [
                "create",
                "channel",
                "ibc-0",
                "ibc-1",
                "--port-a",
                "transfer",
                "--port-b",
                "transfer",
            ],
            check=True,
        )

        # start relaying
        cluster["ibc-0"].supervisor.startProcess("relayer-demo")

    rsp = json.loads(subprocess.check_output(relayer + ["query", "channels", "ibc-0"]))
    src_channel = rsp["result"][0]["channel_id"]

    rsp = json.loads(subprocess.check_output(relayer + ["query", "channels", "ibc-1"]))
    dst_channel = rsp["result"][0]["channel_id"]

    return src_channel, dst_channel


# 3 accounts ibc test
# ibc-0 (A,B)   ibc-1 (C)
# EscrowAddress: sha256-hash of ibc-version,port-id,channel-id
# ----------------------------------------------------
# first, A sends amount to C  <- ibc/....  amount
# second, C sends ibc-amount back to B  <- reclaimed back as basecro amount
# ----------------------------------------------------
# ibc0: A addr_0 (relayer), B addr_0_signer
# ibc1: C addr_1 (relayer), D addr_1_signer


def test_ibc_extended(cluster):
    src_channel, dst_channel = start_and_wait_relayer(cluster)
    raw = cluster["ibc-0"].cosmos_cli().raw

    addr_1 = cluster["ibc-1"].address("relayer")
    addr_0_signer = cluster["ibc-0"].address("signer")
    denom_string = f"transfer/{dst_channel}/basecro"
    # send A -> C
    rsp = cluster["ibc-0"].ibc_transfer(
        "relayer", addr_1, "10000basecro", src_channel, 1
    )
    time.sleep(10)
    res = json.loads(
        raw(
            "query",
            "bank",
            "balances",
            addr_1,
            output="json",
            node=cluster["ibc-1"].node_rpc(0),
        )
    )
    denom_hash = hashlib.sha256(denom_string.encode()).hexdigest().upper()
    assert rsp["code"] == 0, rsp["raw_log"]
    assert res["balances"] == [
        {"denom": "basecro", "amount": "10000000000"},
        {
            "denom": f"ibc/{denom_hash}",
            "amount": "10000",
        },
    ]
    # send B <- C
    rsp = cluster["ibc-1"].ibc_transfer(
        "relayer", addr_0_signer, f"55ibc/{denom_hash}", dst_channel, 0
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    time.sleep(10)
    res = json.loads(
        raw(
            "query",
            "bank",
            "balances",
            addr_0_signer,
            output="json",
            node=cluster["ibc-0"].node_rpc(0),
        )
    )
    assert res["balances"] == [{"denom": "basecro", "amount": "20000000055"}]
