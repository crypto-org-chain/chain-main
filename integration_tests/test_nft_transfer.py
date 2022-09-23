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


# This function tests nft transfer from source chain -> mid chain -> destination chain
# and all the way back to source
# chain following the same path
def test_nft_transfer(cluster):
    src_channel, mid_src_channel = start_and_wait_relayer(cluster, "nft")
    mid_dst_channel, dst_channel = start_and_wait_relayer(
        cluster, "nft", ["ibc-1", "ibc-2"], False
    )

    assert src_channel == "channel-0", src_channel
    assert mid_src_channel == "channel-0", mid_src_channel
    # assert mid_dst_channel == "channel-1", mid_dst_channel
    assert dst_channel == "channel-0", dst_channel

    mid_dst_channel = "channel-1"

    cli_src = cluster["ibc-0"].cosmos_cli()
    cli_mid = cluster["ibc-1"].cosmos_cli()
    cli_dst = cluster["ibc-2"].cosmos_cli()

    addr_src = cluster["ibc-0"].address("relayer")
    addr_mid = cluster["ibc-1"].address("relayer")
    addr_dst = cluster["ibc-2"].address("relayer")

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

    # transfer nft on mid-destination chain
    rsp = json.loads(
        cli_src.raw(
            "tx",
            "nft-transfer",
            "transfer",
            "nft",
            src_channel,
            addr_mid,
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

    # get class hash on mid chain
    mid_class_hash = json.loads(
        cli_mid.raw(
            "query",
            "nft-transfer",
            "class-hash",
            "nft/" + mid_src_channel + "/" + denomid,
            home=cli_mid.data_dir,
            node=cli_mid.node_rpc,
            output="json",
        )
    )["hash"]

    # get class trace on mid chain
    mid_class_trace = json.loads(
        cli_mid.raw(
            "query",
            "nft-transfer",
            "class-trace",
            mid_class_hash,
            home=cli_mid.data_dir,
            node=cli_mid.node_rpc,
            output="json",
        )
    )["class_trace"]

    assert mid_class_trace["base_class_id"] == denomid, mid_class_trace
    assert mid_class_trace["path"] == "nft/" + mid_src_channel, mid_class_trace

    mid_denom_id = "ibc/" + mid_class_hash

    # query denom on mid chain
    rsp = json.loads(
        cli_mid.raw(
            "query",
            "nft",
            "denom",
            mid_denom_id,
            home=cli_mid.data_dir,
            node=cli_mid.node_rpc,
            output="json",
        )
    )

    assert rsp["uri"] == denomuri, rsp["uri"]

    # query nft on mid chain
    rsp = json.loads(
        cli_mid.raw(
            "query",
            "nft",
            "token",
            mid_denom_id,
            tokenid,
            home=cli_mid.data_dir,
            node=cli_mid.node_rpc,
            output="json",
        )
    )

    assert rsp["uri"] == tokenuri, rsp
    assert rsp["owner"] == addr_mid, rsp

    # query nft on source chain's escrow address
    src_escrow_address = str(
        cli_src.raw(
            "query",
            "nft-transfer",
            "escrow-address",
            "nft",
            src_channel,
            home=cli_src.data_dir,
            node=cli_src.node_rpc,
            output="json",
        ),
        "UTF-8",
    ).strip()

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
    assert rsp["owner"] == src_escrow_address, rsp

    # transfer nft to destination chain
    rsp = json.loads(
        cli_mid.raw(
            "tx",
            "nft-transfer",
            "transfer",
            "nft",
            mid_dst_channel,
            addr_dst,
            mid_denom_id,
            tokenid,
            "-y",
            home=cli_mid.data_dir,
            from_=addr_mid,
            keyring_backend="test",
            chain_id=cli_mid.chain_id,
            node=cli_mid.node_rpc,
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # FIXME more stable way to wait for relaying
    time.sleep(20)

    # get class hash on destination chain
    dst_class_hash = json.loads(
        cli_dst.raw(
            "query",
            "nft-transfer",
            "class-hash",
            "nft/" + dst_channel + "/nft/" + mid_src_channel + "/" + denomid,
            home=cli_dst.data_dir,
            node=cli_dst.node_rpc,
            output="json",
        )
    )["hash"]

    # get class trace on destination chain
    dst_class_trace = json.loads(
        cli_dst.raw(
            "query",
            "nft-transfer",
            "class-trace",
            dst_class_hash,
            home=cli_dst.data_dir,
            node=cli_dst.node_rpc,
            output="json",
        )
    )["class_trace"]

    assert dst_class_trace["base_class_id"] == denomid, dst_class_trace
    assert (
        dst_class_trace["path"] == "nft/" + dst_channel + "/nft/" + mid_src_channel
    ), dst_class_trace

    dst_denom_id = "ibc/" + dst_class_hash

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
            dst_denom_id,
            tokenid,
            home=cli_dst.data_dir,
            node=cli_dst.node_rpc,
            output="json",
        )
    )

    assert rsp["uri"] == tokenuri, rsp
    assert rsp["owner"] == addr_dst, rsp

    # quert nft on mid chain's escrow address
    mid_escrow_address = str(
        cli_mid.raw(
            "query",
            "nft-transfer",
            "escrow-address",
            "nft",
            mid_dst_channel,
            home=cli_mid.data_dir,
            node=cli_mid.node_rpc,
            output="json",
        ),
        "UTF-8",
    ).strip()

    rsp = json.loads(
        cli_mid.raw(
            "query",
            "nft",
            "token",
            mid_denom_id,
            tokenid,
            home=cli_mid.data_dir,
            node=cli_mid.node_rpc,
            output="json",
        )
    )

    assert rsp["uri"] == tokenuri, rsp
    assert rsp["owner"] == mid_escrow_address, rsp

    # transfer nft back to mid chain
    rsp = json.loads(
        cli_dst.raw(
            "tx",
            "nft-transfer",
            "transfer",
            "nft",
            dst_channel,
            addr_mid,
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

    # query nft on mid chain
    rsp = json.loads(
        cli_mid.raw(
            "query",
            "nft",
            "token",
            mid_denom_id,
            tokenid,
            home=cli_mid.data_dir,
            node=cli_mid.node_rpc,
            output="json",
        )
    )

    assert rsp["uri"] == tokenuri, rsp
    assert rsp["owner"] == addr_mid, rsp

    # transfer nft back to source chain
    rsp = json.loads(
        cli_mid.raw(
            "tx",
            "nft-transfer",
            "transfer",
            "nft",
            mid_src_channel,
            addr_src,
            mid_denom_id,
            tokenid,
            "-y",
            home=cli_mid.data_dir,
            from_=addr_mid,
            keyring_backend="test",
            chain_id=cli_mid.chain_id,
            node=cli_mid.node_rpc,
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # FIXME more stable way to wait for relaying
    time.sleep(20)

    # nft should be burnt on mid chain
    rsp = json.loads(
        cli_mid.raw(
            "query",
            "nft",
            "collection",
            mid_denom_id,
            home=cli_mid.data_dir,
            node=cli_mid.node_rpc,
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

    # Test packet timeout

    # transfer nft on mid chain (with very less timeout so that the packet times out)
    rsp = json.loads(
        cli_src.raw(
            "tx",
            "nft-transfer",
            "transfer",
            "nft",
            src_channel,
            addr_mid,
            denomid,
            tokenid,
            "-y",
            packet_timeout_height="0-1",
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

    # nft should be not be present on mid chain
    rsp = json.loads(
        cli_mid.raw(
            "query",
            "nft",
            "collection",
            mid_denom_id,
            home=cli_mid.data_dir,
            node=cli_mid.node_rpc,
            output="json",
        )
    )["collection"]

    assert len(rsp["nfts"]) == 0, rsp

    # query nft on source chain (as the transfer should time out)
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
