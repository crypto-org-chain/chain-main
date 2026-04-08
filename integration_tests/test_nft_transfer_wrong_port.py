import json
import subprocess
from pathlib import Path

import pytest

from .ibc_utils import start_and_wait_relayer
from .utils import cluster_fixture

pytestmark = pytest.mark.ibc


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/nft_transfer.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def test_nft_transfer_rejects_wrong_source_port(cluster):
    # Create a live ICS-20 channel so the (transfer, channel) pair exists, then
    # ensure nft-transfer refuses to send on the transfer port.
    src_channel, _ = start_and_wait_relayer(cluster, "transfer", start_relaying=False)

    cli_src = cluster["ibc-0"].cosmos_cli()
    addr_src = cluster["ibc-0"].address("relayer")
    addr_dst = cluster["ibc-1"].address("relayer")

    denomid = "wrongportdenomid"
    denomname = "wrongportdenomname"
    denomuri = "wrongportdenomuri"
    tokenid = "wrongporttokenid"
    tokenuri = "wrongporttokenuri"

    rsp = json.loads(
        cli_src.raw(
            "tx",
            "nft",
            "issue",
            denomid,
            "-y",
            name=denomname,
            uri=denomuri,
            home=cli_src.data_dir,
            from_=addr_src,
            keyring_backend="test",
            chain_id=cli_src.chain_id,
            node=cli_src.node_rpc,
        )
    )
    if rsp["code"] == 0:
        rsp = cli_src.event_query_tx_for(rsp["txhash"])
    assert rsp["code"] == 0, rsp.get("raw_log")

    rsp = json.loads(
        cli_src.raw(
            "tx",
            "nft",
            "mint",
            denomid,
            tokenid,
            "-y",
            uri=tokenuri,
            recipient=addr_src,
            home=cli_src.data_dir,
            from_=addr_src,
            keyring_backend="test",
            chain_id=cli_src.chain_id,
            node=cli_src.node_rpc,
        )
    )
    if rsp["code"] == 0:
        rsp = cli_src.event_query_tx_for(rsp["txhash"])
    assert rsp["code"] == 0, rsp.get("raw_log")

    tx_failed = False
    try:
        out = cli_src.raw(
            "tx",
            "nft-transfer",
            "transfer",
            "transfer",
            src_channel,
            addr_dst,
            denomid,
            tokenid,
            "-y",
            home=cli_src.data_dir,
            from_=addr_src,
            keyring_backend="test",
            chain_id=cli_src.chain_id,
            node=cli_src.node_rpc,
        )
        try:
            rsp = json.loads(out)
        except json.JSONDecodeError:
            tx_failed = True
        else:
            if rsp["code"] == 0:
                rsp = cli_src.event_query_tx_for(rsp["txhash"])
            tx_failed = rsp["code"] != 0
    except (subprocess.CalledProcessError, AssertionError):
        tx_failed = True

    assert tx_failed, "wrong-port nft-transfer tx unexpectedly succeeded"

    rsp = json.loads(
        cli_src.raw(
            "query",
            "nft",
            "token",
            denomid,
            tokenid,
            home=cli_src.data_dir,
            node=cli_src.node_rpc,
            output="json",
        )
    )
    assert rsp["owner"] == addr_src, rsp
