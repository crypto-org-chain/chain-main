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
PARAMS = "params"
DENOM = "basecro"
REWARDS_POOL_NAME = "rewards_pool"
MSG_UPDATE_PARAMS = "/chainmain.tieredrewards.v1.MsgUpdateParams"

# Per-block top-up when fee collector is ~0: trunc(2e9 * 1.0 / 63115); see abci.go.
TOPUP_FULL_SHORTFALL_BASECRO = "31688"

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


def _find_topup_event_at_height(cluster, height):
    """Find EventBaseRewardsTopUp in block results at the given height (if any)."""
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


def _find_recent_topup_event(cluster, amount, lookback=25):
    """Find a matching EventBaseRewardsTopUp in the last `lookback` blocks."""
    latest = int(get_sync_info(cluster.status())["latest_block_height"])
    expected = f'{{"denom":"{DENOM}","amount":"{amount}"}}'
    for h in range(latest, max(1, latest - lookback), -1):
        ev = _find_topup_event_at_height(cluster, h)
        if ev is not None and ev.get("top_up") == expected:
            return ev
    return None


def _assert_topup_event_emitted(cluster, amount):
    ev = _find_recent_topup_event(cluster, amount)
    assert ev is not None, "EventBaseRewardsTopUp should have been emitted"


def test_params_query(cluster):
    """Test querying tieredrewards parameters"""
    params = query_command(cluster, TIEREDREWARDS_MODULE, PARAMS)["params"]
    assert "target_base_rewards_rate" in params
    # Genesis: max valid rate 1.0; mint BPY 63115 matches prior per-block economics.
    rate = float(params["target_base_rewards_rate"])
    assert rate == 1.0


def test_topup_from_pool(cluster):
    """Fund the rewards pool and verify BeginBlocker drains it toward the fee collector.

    Asserts the pool balance drops and an EventBaseRewardsTopUp is emitted for the
    full per-block shortfall (see TOPUP_FULL_SHORTFALL_BASECRO). Genesis disables
    x/mint inflation (base_rewards.jsonnet), so mint does not refill the fee
    collector.
    """
    # Fund the pool from signer1
    pool_addr = module_address(REWARDS_POOL_NAME)
    sf = int(TOPUP_FULL_SHORTFALL_BASECRO)
    # Enough for several top-ups; leave balance for test_pool_drains_to_zero.
    fund_amount = sf * 10
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

    _assert_topup_event_emitted(cluster, TOPUP_FULL_SHORTFALL_BASECRO)


def test_pool_drains_to_zero(cluster):
    """Test that the pool eventually drains to zero as blocks progress."""
    # funded from the previous test_topup_from_pool test
    pool_balance = _pool_balance(cluster)
    assert pool_balance > 0, "pool should still have funds"

    _assert_topup_event_emitted(cluster, TOPUP_FULL_SHORTFALL_BASECRO)

    # Wait for enough blocks for the pool to be fully drained
    wait_for_new_blocks(cluster, 10)

    pool_final = _pool_balance(cluster)
    assert pool_final == 0, (
        f"pool should be fully drained after enough blocks, "
        f"but still has {pool_final} basecro"
    )


def test_chain_continues_after_pool_empty(cluster):
    """Test that the chain continues producing blocks after the pool is empty.
    This verifies that an empty pool doesn't cause consensus errors / panics.
    """
    # Ensure pool is empty from previous test
    pool_balance = _pool_balance(cluster)
    assert pool_balance == 0, "pool should be empty from previous test"

    height_before = int(get_sync_info(cluster.status())["latest_block_height"])

    # Wait for several more blocks
    wait_for_new_blocks(cluster, 5)

    height_after = int(get_sync_info(cluster.status())["latest_block_height"])
    assert (
        height_after >= height_before + 5
    ), "chain should continue producing blocks with empty pool"


def test_insufficient_pool_partial_drain(cluster):
    """Test that when pool has less than the shortfall, it drains everything available.
    Fund the pool with 1 basecro — less than the per-block shortfall.
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
    """With target rate = 0, the pool is not drained by top-up.

    First fund the pool and verify it drains under the default target rate (1.0).
    After a gov proposal sets target_base_rewards_rate to 0, the pool balance
    stays flat.
    """
    pool_addr = module_address(REWARDS_POOL_NAME)
    sf = int(TOPUP_FULL_SHORTFALL_BASECRO)
    rsp = cluster.transfer(
        cluster.address("signer1"),
        pool_addr,
        f"{max(10_000, sf * 30)}basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cluster, 1)

    pool_after_fund = _pool_balance(cluster)
    assert pool_after_fund > 0, "pool should have been funded"

    # Pool drains while target_base_rewards_rate is still the genesis default (1.0).
    wait_for_new_blocks(cluster, 3)
    pool_after_drain = _pool_balance(cluster)
    assert pool_after_drain < pool_after_fund, "pool should have been drained"

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
    assert pool_after == pool_before, "pool should be untouched when rate is zero"


@pytest.fixture(scope="function")
def inflation_cluster(worker_index, tmp_path_factory):
    """Cluster with mint inflation enabled so the fee collector is funded each block."""
    yield from cluster_fixture(
        Path(__file__).parent / "configs/base_rewards_inflation.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def test_fee_collector_sufficient_no_topup(inflation_cluster):
    """No top-up when the fee collector already covers the per-block target.

    Uses a separate cluster with mint inflation enabled (13%) and a moderate
    target_base_rewards_rate (3%).  The minted coins each block far exceed the
    per-block target, so tieredrewards BeginBlocker never draws from the pool.

    BeginBlocker order: mint → tieredrewards → distribution.  After mint runs,
    the fee collector holds the freshly minted coins.  tieredrewards checks
    this balance and sees no shortfall, so the pool stays untouched.
    """
    cluster = inflation_cluster

    # Fund the rewards pool so we can verify it stays untouched
    pool_addr = module_address(REWARDS_POOL_NAME)
    rsp = cluster.transfer(
        cluster.address("signer1"),
        pool_addr,
        "1000000basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cluster, 1)

    pool_before = _pool_balance(cluster)
    assert pool_before > 0, "pool should have been funded"

    # Wait for several blocks — tieredrewards BeginBlocker runs each block
    # but should not top up because the fee collector is well-funded by inflation
    wait_for_new_blocks(cluster, 5)

    pool_after = _pool_balance(cluster)

    assert pool_after == pool_before, (
        f"pool should be untouched when fee collector is sufficient; "
        f"before={pool_before}, after={pool_after}"
    )
