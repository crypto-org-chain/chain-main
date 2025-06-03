import hashlib
import json
from pathlib import Path

import pytest

from .ibc_utils import start_and_wait_relayer
from .utils import cluster_fixture, wait_for_fn

pytestmark = pytest.mark.ibc


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/ibc.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def fund_community_pool(self, amt, wait_for_block=True, **kwargs):
    rsp = json.loads(
        self.raw(
            "tx",
            "distribution",
            "fund-community-pool",
            amt,
            "-y",
            home=self.data_dir,
            keyring_backend="test",
            chain_id=self.chain_id,
            node=self.node_rpc,
            **kwargs,
        )
    )
    if rsp["code"] == 0 and wait_for_block:
        rsp = self.event_query_tx_for(rsp["txhash"])
    return rsp


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
    denom = "basecro"
    amt = 10000

    addr_1 = cluster["ibc-1"].address("relayer")
    addr_0_signer = cluster["ibc-0"].address("signer")
    # send A -> C
    rsp = cluster["ibc-0"].ibc_transfer(
        "relayer", addr_1, f"{amt}{denom}", src_channel, 1
    )
    assert rsp["code"] == 0, rsp["raw_log"]
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
    assert new_dst_balance == amt + old_dst_balance, new_dst_balance

    amt2 = 55
    # send B <- C
    rsp = cluster["ibc-1"].ibc_transfer(
        "relayer", addr_0_signer, f"{amt2}{ibc_denom}", dst_channel, 0
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    old_src_balance = cluster["ibc-0"].balance(addr_0_signer, denom)
    new_src_balance = 0

    def check_balance_change():
        nonlocal new_src_balance
        new_src_balance = cluster["ibc-0"].balance(addr_0_signer, denom)
        return new_src_balance != old_src_balance

    wait_for_fn("balance change", check_balance_change)
    assert new_src_balance == amt2 + old_src_balance, new_src_balance

    amt3 = 1
    fund_community_pool(
        cluster["ibc-1"].cosmos_cli(), f"{amt3}{ibc_denom}", from_=addr_1
    )
    cluster["ibc-1"].distribution_community()
    res = cluster["ibc-1"].supply("liquid")
    assert res == {
        "supply": [
            {"denom": denom, "amount": "260000000000"},
            {"denom": ibc_denom, "amount": f"{amt - amt2 - amt3}"},
            {"denom": "ibcfee", "amount": "100000000000"},
        ]
    }
