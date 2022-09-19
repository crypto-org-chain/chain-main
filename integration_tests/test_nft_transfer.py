import json
import time
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


def test_nft_transfer(cluster):
    src_channel, dst_channel = start_and_wait_relayer(cluster, "nft")

    cli_src = cluster["ibc-0"].cosmos_cli()
    cli_dst = cluster["ibc-1"].cosmos_cli()

    addr_src = cluster["ibc-0"].address("relayer")
    addr_dst = cluster["ibc-1"].address("relayer")

    denomid = "testdenomid"
    denomname = "testdenomname"
    denomuri = "testdenomuri"

    tokenid = "testtokenid"
    tokenuri = "testtokenuri"

    # mint nft on source chain
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

    raw_log = json.loads(rsp["raw_log"])
    assert raw_log[0]["events"][0]["type"] == "issue_denom"

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

    raw_log = json.loads(rsp["raw_log"])
    assert (
        raw_log[0]["events"][0]["attributes"][0]["value"]
        == "/chainmain.nft.v1.MsgMintNFT"
    )

    # transfer nft on destination chain
    rsp = json.loads(
        cli_src.raw(
            "tx",
            "nft-transfer",
            "transfer",
            "nft",
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
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # FIXME more stable way to wait for relaying
    time.sleep(20)

    # get class hash on destination chain
    class_hash = json.loads(
        cli_dst.raw(
            "query",
            "nft-transfer",
            "class-hash",
            "nft/" + dst_channel + "/" + denomid,
            home=cli_dst.data_dir,
            node=cli_dst.node_rpc,
            output="json",
        )
    )["hash"]

    dst_denom_id = "ibc/" + class_hash

    # query denom on destination chain
    rsp = json.loads(
        cli_dst.raw(
            "query",
            "nft",
            "denom",
            dst_denom_id,
            home=cli_dst.data_dir,
            node=cli_dst.node_rpc,
            output="json",
        )
    )

    assert rsp["uri"] == denomuri, rsp["uri"]

    # query nft on destination chain
    rsp = json.loads(
        cli_dst.raw(
            "query",
            "nft",
            "token",
            "ibc/" + class_hash,
            tokenid,
            home=cli_dst.data_dir,
            node=cli_dst.node_rpc,
            output="json",
        )
    )

    assert rsp["uri"] == tokenuri, rsp
    assert rsp["owner"] == addr_dst, rsp

    # transfer nft back to source chain
    rsp = json.loads(
        cli_dst.raw(
            "tx",
            "nft-transfer",
            "transfer",
            "nft",
            dst_channel,
            addr_src,
            dst_denom_id,
            tokenid,
            "-y",
            home=cli_dst.data_dir,
            from_=addr_dst,
            keyring_backend="test",
            chain_id=cli_dst.chain_id,
            node=cli_dst.node_rpc,
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # FIXME more stable way to wait for relaying
    time.sleep(20)

    # nft should be burnt on destination chain
    rsp = json.loads(
        cli_dst.raw(
            "query",
            "nft",
            "collection",
            dst_denom_id,
            home=cli_dst.data_dir,
            node=cli_dst.node_rpc,
            output="json",
        )
    )["collection"]

    assert len(rsp["nfts"]) == 0, rsp

    # query nft on source chain
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

    assert rsp["uri"] == tokenuri, rsp
    assert rsp["owner"] == addr_src, rsp
