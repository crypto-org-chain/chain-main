from pathlib import Path

import pytest
import requests

from pystarport.ports import rpc_port

from .utils import (
    approve_proposal,
    cluster_fixture,
    get_sync_info,
    module_address,
    query_command,
    wait_for_new_blocks,
)

TIEREDREWARDS_MODULE = "tieredrewards"
BANK_MODULE = "bank"
PARAMS = "params"
DENOM = "basecro"
REWARDS_POOL_NAME = "rewards_pool"
MSG_UPDATE_PARAMS = "/chainmain.tieredrewards.v1.MsgUpdateParams"

pytestmark = pytest.mark.base_rewards


@pytest.fixture(scope="module")
def cluster(worker_index, tmp_path_factory):
    "override cluster fixture for base rewards topup tests"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/base_rewards.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def _pool_balance(cluster):
    """Get the base rewards pool balance in basecro"""
    pool_addr = module_address(REWARDS_POOL_NAME)
    return cluster.balance(pool_addr, DENOM)


def _distr_balance(cluster):
    """Get the distribution module balance in basecro"""
    distr_addr = module_address("distribution")
    return cluster.balance(distr_addr, DENOM)


def _fee_collector_balance(cluster):
    """Get the fee collector balance in basecro"""
    fc_addr = module_address("fee_collector")
    return cluster.balance(fc_addr, DENOM)


def _find_topup_event(cluster, height):
    """Find the EventBaseRewardsTopUp event in block results at the given height."""
    base_port = cluster.config["validators"][0]["base_port"]
    port = rpc_port(base_port)
    url = f"http://127.0.0.1:{port}/block_results?height={height}"
    res = requests.get(url).json()
    # BeginBlocker events are in finalize_block_events (CometBFT v0.38+)
    events = res.get("result", {}).get("finalize_block_events", [])
    if not events:
        # Fallback for older CometBFT versions
        events = res.get("result", {}).get("begin_block_events", [])
    for event in events:
        if event.get("type") == "chainmain.tieredrewards.v1.EventBaseRewardsTopUp":
            attrs = {}
            for attr in event.get("attributes", []):
                attrs[attr["key"]] = attr["value"]
            return attrs
    return None

def _assert_topup_event_emitted(cluster, amount):
    height = int(get_sync_info(cluster.status())["latest_block_height"])
    ev = _find_topup_event(cluster, height)
    assert ev is not None, "EventBaseRewardsTopUp should have been emitted"
    expected = f'{{"denom":"{DENOM}","amount":"{amount}"}}'
    assert ev["top_up"] == expected, (
        f"EventBaseRewardsTopUp topup mismatch: expected {expected}, got {ev['top_up']}"
    )


def test_params_query(cluster):
    """Test querying tieredrewards parameters"""
    params = query_command(cluster, TIEREDREWARDS_MODULE, PARAMS)["params"]
    assert "target_base_rewards_rate" in params
    # Genesis config sets it to 100.0 (10000%)
    rate = float(params["target_base_rewards_rate"])
    assert rate == 100.0


def test_empty_pool_no_panic(cluster):
    """Test that BeginBlocker runs without error when the pool is empty.
    The pool starts empty in genesis, but the chain should still produce blocks.
    """
    pool_balance = _pool_balance(cluster)
    assert pool_balance == 0

    # Chain should still be producing blocks
    wait_for_new_blocks(cluster, 2)

    pool_balance = _pool_balance(cluster)
    assert pool_balance == 0, "pool should still be empty"


# community tax = 2%
# total bonded = 2000000000
# blocks per year = 6311520
# target base rewards rate = 100 // 10000%
# target stakers reward = 2000000000 * 100 / 6311520 = 31688.088638376 basecro
# fee collector balance = 25581
# default stakers reward = 25581 * (1 - 0.02) = 25069.38 basecro
# shortfall per block = 31688.088638376 - 25069.38 ~= 6618 basecro

def test_topup_from_pool(cluster):
    """Test that funding the pool results in rewards being distributed.
    Fund the pool, wait for blocks, verify pool decreased and distribution
    module balance increased.
    """
    # Fund the pool from signer1
    pool_addr = module_address(REWARDS_POOL_NAME)
    # each block require ~6618 basecro, so this covers about 10 blocks
    fund_amount = 70000
    fund_amount_coin = f"{fund_amount}basecro"

    rsp = cluster.transfer(
        cluster.address("signer1"),
        pool_addr,
        fund_amount_coin,
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    wait_for_new_blocks(cluster, 1)

    pool_after_fund = _pool_balance(cluster)
    assert pool_after_fund > 0, "pool should have been funded"

    # Wait for a few blocks so the BeginBlocker runs and tops up
    wait_for_new_blocks(cluster, 3)

    pool_after = _pool_balance(cluster)

    assert pool_after < pool_after_fund, "pool should have been drained"

    _assert_topup_event_emitted(cluster, "6618")

def test_pool_drains_to_zero(cluster):
    """Test that the pool eventually drains to zero as blocks progress.
    With a very high target rate (10000%) and limited pool funds,
    the pool should be fully drained after enough blocks.
    """
    # funded from the previous test_topup_from_pool test
    pool_balance = _pool_balance(cluster)
    assert pool_balance > 0, "pool should still have funds"

    _assert_topup_event_emitted(cluster, "6618")

    # Wait for enough blocks for the pool to be fully drained
    wait_for_new_blocks(cluster, 10)

    pool_final = _pool_balance(cluster)
    assert pool_final == 0, (
        f"pool should be fully drained after enough blocks, "
        f"but still has {pool_final} basecro"
    )

def test_chain_continues_after_pool_empty(cluster):
    """Test that the chain continues producing blocks after the pool is empty.
    This verifies that an empty pool doesn't cause consensus errors.
    """
    # Ensure pool is empty from previous test
    pool_balance = _pool_balance(cluster)
    assert pool_balance == 0, "pool should be empty from previous test"

    height_before = int(get_sync_info(cluster.status())["latest_block_height"])

    # Wait for several more blocks
    wait_for_new_blocks(cluster, 5)

    height_after = int(get_sync_info(cluster.status())["latest_block_height"])
    assert height_after >= height_before + 5, (
        "chain should continue producing blocks with empty pool"
    )

def test_insufficient_pool_partial_drain(cluster):
    """Test that when pool has less than the shortfall, it drains everything available.
    Fund the pool with 1 basecro — guaranteed less than any shortfall at 10000% rate.
    """
    pool_addr = module_address(REWARDS_POOL_NAME)

    rsp = cluster.transfer(
        cluster.address("signer1"),
        pool_addr,
        "1basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # One block should drain it completely
    wait_for_new_blocks(cluster, 2)

    pool_after = _pool_balance(cluster)

    assert pool_after == 0, (
        f"pool should be fully drained when insufficient, "
        f"but still has {pool_after} basecro"
    )

def test_zero_rate_no_topup(cluster):
    """Test that with rate = 0, no top-up occurs regardless of pool balance.
    First fund pool and verify it drains at the current 10000% rate.
    Then set rate to 0 and verify pool stops draining.
    """
    # Fund the pool (rate is still 10000% from previous tests)
    pool_addr = module_address(REWARDS_POOL_NAME)
    rsp = cluster.transfer(
        cluster.address("signer1"),
        pool_addr,
        "200000basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cluster, 1)

    pool_after_fund = _pool_balance(cluster)
    assert pool_after_fund > 0, "pool should have been funded"

    # Verify pool drains at 10000% rate
    wait_for_new_blocks(cluster, 3)
    pool_after_drain = _pool_balance(cluster)
    assert pool_after_drain < pool_after_fund, (
        "pool should have been drained"
    )

    # Now set rate to 0
    params = query_command(cluster, TIEREDREWARDS_MODULE, PARAMS)["params"]
    params["target_base_rewards_rate"] = "0.000000000000000000"

    authority = module_address("gov")
    proposal_src = {
        "messages": [
            {
                "@type": MSG_UPDATE_PARAMS,
                "authority": authority,
                "params": params,
            }
        ],
        "deposit": "100000000basecro",
        "title": "Set rate to zero",
        "summary": "Disable base rewards top-up",
    }
    rsp = cluster.gov_propose_since_cosmos_sdk_v0_50(
        "community", "submit-proposal", proposal_src
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(cluster, rsp, msg=f",{MSG_UPDATE_PARAMS}")

    # Verify rate is 0
    updated_params = query_command(cluster, TIEREDREWARDS_MODULE, PARAMS)["params"]
    assert float(updated_params["target_base_rewards_rate"]) == 0.0

    pool_before = _pool_balance(cluster)

    wait_for_new_blocks(cluster, 3)

    pool_after = _pool_balance(cluster)
    assert pool_after == pool_before, (
        "pool should be untouched when rate is zero"
    )
    

# target base rewards rate = 0.01 // 1%
# target stakers reward = 2000000000 * 0.01 / 6311520 = 3.1688088638376 basecro
# fee collector balance per block = 25581 basecro
def test_fee_collector_sufficient_no_topup(cluster):
    """Test that no top-up occurs when fee collector already covers the target.
    Update params to a very low rate via governance so fee collector is sufficient.
    Then verify pool is untouched.
    """

    # Update rate to something tiny so fee collector covers it
    params = query_command(cluster, TIEREDREWARDS_MODULE, PARAMS)["params"]
    params["target_base_rewards_rate"] = "0.010000000000000000"

    authority = module_address("gov")
    proposal_src = {
        "messages": [
            {
                "@type": MSG_UPDATE_PARAMS,
                "authority": authority,
                "params": params,
            }
        ],
        "deposit": "100000000basecro",
        "title": "Lower rate so fee collector is sufficient",
        "summary": "Set rate very low",
    }
    rsp = cluster.gov_propose_since_cosmos_sdk_v0_50(
        "community", "submit-proposal", proposal_src
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(cluster, rsp, msg=f",{MSG_UPDATE_PARAMS}")

    # Verify params updated
    updated_params = query_command(cluster, TIEREDREWARDS_MODULE, PARAMS)["params"]
    assert float(updated_params["target_base_rewards_rate"]) == 0.01

   # funded from the previous test_zero_rate_no_topup test
    pool_funds = _pool_balance(cluster)
    assert pool_funds > 0, "pool should have funds"

    wait_for_new_blocks(cluster, 3)

    pool_after = _pool_balance(cluster)

    assert pool_after == pool_funds, (
        "pool should be untouched when fee collector is sufficient"
    )
