import json

import pytest

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
    response = cluster.create_nft(singer1_addr, denomid, denomname)
    raw_log = json.loads(response["raw_log"])
    assert raw_log[0]["events"][0]["type"] == "issue_denom"


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
    response = cluster.create_nft_token(
        singer1_addr, singer2_addr, denomid, tokenid, uri
    )
    raw_log = json.loads(response["raw_log"])
    assert (
        raw_log[0]["events"][0]["attributes"][0]["value"]
        == "/chainmain.nft.v1.MsgMintNFT"
    )


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
    response = cluster.transfer_nft_token(singer2_addr, singer1_addr, denomid, tokenid)
    raw_log = json.loads(response["raw_log"])
    assert (
        raw_log[0]["events"][0]["attributes"][0]["value"]
        == "/chainmain.nft.v1.MsgTransferNFT"
    )


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
    response = cluster.edit_nft_token(singer1_addr, denomid, tokenid, newuri, newname)
    raw_log = json.loads(response["raw_log"])
    assert raw_log[0]["events"][0]["type"] == "edit_nft"
    assert raw_log[0]["events"][0]["attributes"][2]["key"] == "token_uri"
    assert raw_log[0]["events"][0]["attributes"][2]["value"] == newuri


def test_burn_nft_token(cluster):
    denomid = "testdenomid"
    tokenid = "testtokenid"
    singer1_addr = cluster.address("signer1")
    response = cluster.burn_nft_token(singer1_addr, denomid, tokenid)
    raw_log = json.loads(response["raw_log"])
    assert raw_log[0]["events"][0]["type"] == "burn_nft"
