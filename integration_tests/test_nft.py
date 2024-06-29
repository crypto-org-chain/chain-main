import pytest

from .utils import find_log_event_attrs

pytestmark = pytest.mark.normal


def test_create_nft(cluster):
    """
    - check number of validators
    - check vesting account status
    """
    assert len(cluster.validators()) == 2
    singer1_addr = cluster.address("signer1")
    denomid = "testdenomid"
    denomname = "testdenomname"
    rsp = cluster.create_nft(singer1_addr, denomid, denomname)
    ev = find_log_event_attrs(rsp["logs"], "issue_denom")
    assert ev == {
        "denom_id": denomid,
        "denom_name": denomname,
        "creator": singer1_addr,
    }, ev


def test_query_nft(cluster):
    denomid = "testdenomid"
    singer1_addr = cluster.address("signer1")
    response = cluster.query_nft(denomid)
    assert response["id"] == denomid
    assert response["creator"] == singer1_addr


def test_query_denom_by_name(cluster):
    denomname = "testdenomname"
    singer1_addr = cluster.address("signer1")
    response = cluster.query_denom_by_name(denomname)
    assert response["name"] == denomname
    assert response["creator"] == singer1_addr


def test_create_nft_token(cluster):
    print("create nft token")
    denomid = "testdenomid"
    tokenid = "testtokenid"
    singer1_addr = cluster.address("signer1")
    singer2_addr = cluster.address("signer2")
    uri = "testuri"
    rsp = cluster.create_nft_token(singer1_addr, singer2_addr, denomid, tokenid, uri)
    ev = find_log_event_attrs(rsp["logs"], "message")
    assert ev["action"] == "/chainmain.nft.v1.MsgMintNFT", ev


def test_query_nft_token(cluster):
    denomid = "testdenomid"
    tokenid = "testtokenid"
    singer2_addr = cluster.address("signer2")
    response = cluster.query_nft_token(denomid, tokenid)
    assert response["id"] == tokenid
    assert response["owner"] == singer2_addr


def test_transfer_nft_token(cluster):
    denomid = "testdenomid"
    tokenid = "testtokenid"
    singer1_addr = cluster.address("signer1")
    singer2_addr = cluster.address("signer2")
    rsp = cluster.transfer_nft_token(singer2_addr, singer1_addr, denomid, tokenid)
    ev = find_log_event_attrs(rsp["logs"], "message")
    assert ev["action"] == "/chainmain.nft.v1.MsgTransferNFT", ev


def test_query_nft_token_again(cluster):
    denomid = "testdenomid"
    tokenid = "testtokenid"
    singer1_addr = cluster.address("signer1")
    response = cluster.query_nft_token(denomid, tokenid)
    assert response["id"] == tokenid
    assert response["owner"] == singer1_addr


def test_edit_nft_token(cluster):
    denomid = "testdenomid"
    tokenid = "testtokenid"
    singer1_addr = cluster.address("signer1")
    newuri = "newuri"
    newname = "newname"
    rsp = cluster.edit_nft_token(singer1_addr, denomid, tokenid, newuri, newname)
    ev = find_log_event_attrs(rsp["logs"], "edit_nft")
    assert ev == {
        "token_id": tokenid,
        "denom_id": denomid,
        "token_uri": newuri,
        "owner": singer1_addr,
    }, ev


def test_burn_nft_token(cluster):
    denomid = "testdenomid"
    tokenid = "testtokenid"
    singer1_addr = cluster.address("signer1")
    rsp = cluster.burn_nft_token(singer1_addr, denomid, tokenid)
    ev = find_log_event_attrs(rsp["logs"], "burn_nft")
    assert ev == {
        "denom_id": denomid,
        "token_id": tokenid,
        "owner": singer1_addr,
    }, ev
