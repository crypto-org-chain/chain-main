import json
from datetime import timedelta
from decimal import Decimal
from pathlib import Path

import pytest
import requests
from dateutil.parser import isoparse
from pystarport.ports import rpc_port

from .tieredrewards_helpers import (
    DENOM,
    GAS_ALLOWANCE,
    MODULE,
    REWARDS_POOL_NAME,
    TIER_1_ID,
    TIER_1_MIN,
    TIER_2_ID,
    TIER_2_MIN,
    add_to_position,
    before_ids,
    claim_rewards,
    clear_position,
    commit_delegation,
    fund_pool,
    get_node_validator_addr,
    get_validator_addr,
    lock_tier,
    new_pos_id,
    pool_balance,
    query_estimate_rewards,
    query_position,
    query_positions_by_owner,
    query_tiers,
    tier_redelegate,
    tier_undelegate,
    trigger_exit,
    tx,
    withdraw,
)
from .utils import (
    cluster_fixture,
    find_log_event_attrs,
    query_command,
    wait_for_block_time,
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = [pytest.mark.tieredrewards]


# ──────────────────────────────────────────────
# Cluster fixture
# ──────────────────────────────────────────────


@pytest.fixture(scope="module")
def cluster(worker_index, tmp_path_factory):
    "override cluster fixture for tieredrewards tests"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/tieredrewards.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


@pytest.fixture(scope="function")
def slashing_cluster(worker_index, tmp_path_factory):
    """Use a fresh cluster so validator power shifts from earlier tests do not halt
    consensus.
    """
    yield from cluster_fixture(
        Path(__file__).parent / "configs/tieredrewards.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("d"),
    )


# ──────────────────────────────────────────────
# Tests
# ──────────────────────────────────────────────


def test_commit_delegation_to_tier(cluster):
    """Convert an existing staking delegation into a tier position without undelegating.

    Verifies:
    - The tier position is created with the committed amount and validator.
    - The owner's x/staking delegation to the same validator decreases by commit_amount.
    """
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)

    # First: delegate normally via x/staking
    staking_amount = TIER_1_MIN * 5
    cli = cluster.cosmos_cli()
    rsp = json.loads(
        cli.raw(
            "tx",
            "staking",
            "delegate",
            validator,
            f"{staking_amount}{DENOM}",
            "-y",
            from_=owner,
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
            chain_id=cli.chain_id,
            output="json",
            gas=300000,
        )
    )
    if rsp["code"] == 0:
        rsp = cli.event_query_tx_for(rsp["txhash"])
    assert rsp["code"] == 0, rsp["raw_log"]

    wait_for_new_blocks(cluster, 1)

    # Record staking delegation before commit
    del_before = json.loads(
        cli.raw(
            "q",
            "staking",
            "delegation",
            owner,
            validator,
            output="json",
            home=cli.data_dir,
            node=cli.node_rpc,
        )
    )
    shares_before = Decimal(del_before["delegation_response"]["delegation"]["shares"])

    # Then: commit part of that delegation to a tier position
    commit_amount = TIER_1_MIN * 2
    before = before_ids(cluster, owner)
    rsp = commit_delegation(cluster, owner, validator, commit_amount, TIER_1_ID)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    pos = query_position(cluster, pos_id)["position"]
    assert pos["validator"] == validator, "committed position should be delegated"
    assert (
        int(pos["amount"]) == commit_amount
    ), f"committed amount mismatch: expected {commit_amount}, got {pos['amount']}"

    # Staking delegation should have decreased by approximately commit_amount
    del_after = json.loads(
        cli.raw(
            "q",
            "staking",
            "delegation",
            owner,
            validator,
            output="json",
            home=cli.data_dir,
            node=cli.node_rpc,
        )
    )
    shares_after = Decimal(del_after["delegation_response"]["delegation"]["shares"])
    assert shares_after < shares_before, (
        f"staking delegation should decrease after commit: "
        f"before={shares_before}, after={shares_after}"
    )


def test_commit_delegation_with_exit(cluster):
    """CommitDelegationToTier with trigger_exit_immediately sets exit_triggered_at."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    staking_amount = TIER_1_MIN * 5
    commit_amount = TIER_1_MIN * 2

    # First: delegate normally via x/staking
    cli = cluster.cosmos_cli()
    rsp = json.loads(
        cli.raw(
            "tx",
            "staking",
            "delegate",
            validator,
            f"{staking_amount}{DENOM}",
            "-y",
            from_=owner,
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
            chain_id=cli.chain_id,
            output="json",
            gas=300000,
        )
    )
    if rsp["code"] == 0:
        rsp = cli.event_query_tx_for(rsp["txhash"])
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cluster, 1)

    # Commit with trigger_exit_immediately=true
    before = before_ids(cluster, owner)
    rsp = commit_delegation(
        cluster, owner, validator, commit_amount, TIER_1_ID, trigger_exit=True
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    pos = query_position(cluster, pos_id)["position"]
    assert pos["validator"] == validator, "committed position should be delegated"
    assert (
        pos.get("exit_triggered_at")
        and pos["exit_triggered_at"] != "0001-01-01T00:00:00Z"
    ), "exit_triggered_at should be set when trigger_exit_immediately=true"


# ──────────────────────────────────────────────
# Exit Flow (ADR-006 §5.6, §5.7)
# ──────────────────────────────────────────────


def test_full_exit_flow(cluster):
    """Full exit lifecycle: lock → trigger-exit → wait 5s → undelegate → withdraw.

    Uses Tier 1 (5s exit) + 10s unbonding time from genesis.jsonnet.
    """
    owner = cluster.address("ecosystem")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 3

    # 1. Lock and delegate
    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # 2. Trigger exit; read exit_unlock_at from position
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])

    # 3. Wait for exit duration (5s) to elapse
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    # 4. Undelegate (allowed because exit was triggered, §5.4)
    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Parse unbonding completion_time from EventPositionUndelegated
    unbond_data = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    assert unbond_data is not None, (
        "EventPositionUndelegated with completion_time not found in "
        "tier-undelegate response"
    )
    completion_time = isoparse(unbond_data["completion_time"].strip('"')) + timedelta(
        seconds=1
    )

    # 5. Wait for unbonding to complete using chain time (unbonding_time = 10s from
    #    genesis.jsonnet)
    wait_for_block_time(cluster, completion_time)
    wait_for_new_blocks(cluster, 1)

    # 6. Withdraw — position deleted, tokens returned
    balance_before = cluster.balance(owner, DENOM)
    rsp = withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = cluster.balance(owner, DENOM)
    # User receives back approximately `amount` minus gas fees across all txs
    assert balance_after >= balance_before + amount - GAS_ALLOWANCE, (
        f"expected balance increase of ~{amount} after withdraw: "
        f"before={balance_before}, after={balance_after}"
    )

    # Position should no longer exist after withdraw
    try:
        query_position(cluster, pos_id)
        assert False, f"position {pos_id} should be deleted after withdraw"
    except requests.HTTPError as exc:
        # REST gateway may return 404 (not found) or 500 (wrapping not-found)
        assert exc.response.status_code in (
            404,
            500,
        ), f"expected 404/500 for deleted position, got {exc.response.status_code}"
        assert (
            "not found" in exc.response.text.lower()
        ), f"expected 'not found' error body, got: {exc.response.text}"


# ──────────────────────────────────────────────
# Rewards (ADR-006 §1, §4, §5.8)
# ──────────────────────────────────────────────


def test_claim_rewards_delegated(cluster):
    """claim-tier-rewards on a delegated position distributes nonzero rewards."""
    owner = cluster.address("signer2")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 5
    fund_amount = 50_000_000

    # Fund pool to ensure bonus rewards are available
    rsp = fund_pool(cluster, "signer1", f"{fund_amount}{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    # Create delegated position
    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Wait for rewards to accrue (more blocks = more rewards)
    wait_for_new_blocks(cluster, 10)

    balance_before = cluster.balance(owner, DENOM)
    rsp = claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = cluster.balance(owner, DENOM)
    # Rewards must be positive — balance strictly greater after claim
    assert balance_after > balance_before, (
        f"owner balance must increase after claiming rewards: "
        f"before={balance_before}, after={balance_after}"
    )

    # EventTierRewardsClaimed must be present in tx events
    ev = find_log_event_attrs(
        rsp["events"], "chainmain.tieredrewards.v1.EventTierRewardsClaimed"
    )
    assert ev is not None, "EventTierRewardsClaimed not found in tx events"
    assert (
        "position_id" in ev
    ), f"EventTierRewardsClaimed missing position_id field: {ev}"


def test_bonus_stops_after_exit_unlock(cluster):
    """After exit_unlock_at passes, estimated bonus rewards drop to 0.

    Uses a large delegated position to verify:
    1. Bonus > 0 before exit_unlock_at (after initializing LastBonusAccrual)
    2. Bonus = 0 after claiming post-exit_unlock_at (LastBonusAccrual capped)

    Note: bonus requires LastBonusAccrual to be non-zero (set by first claim).
    A fresh position always shows 0 until the first claim initializes it.
    """
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    # Large position to accumulate measurable bonus in a few blocks.
    # With 1B basecro at 4% APY, bonus ≈ 1B*0.04*T/31,557,600 basecro.
    # After 15 blocks (~15s worst-case): ~19 basecro → reliably > 0.
    amount = TIER_1_MIN * 1000  # 1_000_000_000 basecro

    # Fund pool to ensure bonus rewards are available
    rsp = fund_pool(cluster, "signer1", f"1000000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    # Create delegated position in Tier 1 (5s exit, 4% APY bonus)
    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # First claim initializes LastBonusAccrual (bonus = 0 here, field was unset).
    # Without this, calculateBonusRaw returns 0 for all subsequent estimates.
    rsp = claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Wait blocks so bonus accrues since LastBonusAccrual was set
    wait_for_new_blocks(cluster, 15)

    # Verify nonzero bonus estimate BEFORE triggering exit
    est_before = query_estimate_rewards(cluster, pos_id)
    bonus_before_list = est_before.get("bonus_rewards", [])
    bonus_before = sum(int(c.get("amount", "0")) for c in bonus_before_list)
    assert bonus_before > 0, (
        f"delegated position should have nonzero bonus before exit_unlock_at, "
        f"got bonus_rewards={bonus_before_list}"
    )

    # Trigger exit
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])

    # Wait past exit_unlock_at (Tier 1 has 5s exit duration)
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 2)

    # Claim after exit_unlock_at: LastBonusAccrual is capped and set to ExitUnlockAt.
    # This settles all remaining bonus (from LastBonusAccrual up to ExitUnlockAt).
    balance_before_claim = cluster.balance(owner, DENOM)
    rsp = claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Final claim must actually deliver tokens to the owner
    balance_after_claim = cluster.balance(owner, DENOM)
    assert balance_after_claim > balance_before_claim, (
        f"final claim after exit_unlock_at should increase owner balance: "
        f"before={balance_before_claim}, after={balance_after_claim}"
    )

    # Estimate: accrualEnd=ExitUnlockAt, LastBonusAccrual=ExitUnlockAt → bonus=0
    est_after = query_estimate_rewards(cluster, pos_id)
    bonus_after_list = est_after.get("bonus_rewards", [])
    bonus_after = sum(int(c.get("amount", "0")) for c in bonus_after_list)
    assert bonus_after == 0, (
        f"bonus rewards must be 0 after final claim post-exit_unlock_at, "
        f"got {bonus_after_list}"
    )


def test_clear_exit_thenadd_to_position(cluster):
    """Clearing an exited position settles rewards, then allows adding again."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 1000
    add_amount = TIER_1_MIN * 2

    rsp = fund_pool(cluster, "signer1", f"1000000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Initialize LastBonusAccrual, then let rewards build before entering exit mode.
    rsp = claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cluster, 10)

    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos_before_clear = query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos_before_clear["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    est_before_clear = query_estimate_rewards(cluster, pos_id)
    bonus_before = sum(
        int(c.get("amount", "0")) for c in est_before_clear.get("bonus_rewards", [])
    )
    assert bonus_before > 0, "bonus should be pending before clearing exit"

    balance_before_clear = cluster.balance(owner, DENOM)
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    balance_after_clear = cluster.balance(owner, DENOM)
    assert (
        balance_after_clear > balance_before_clear
    ), "clear-position should settle rewards"

    pos_after_clear = query_position(cluster, pos_id)["position"]
    assert (
        pos_after_clear["exit_triggered_at"] == "0001-01-01T00:00:00Z"
    ), "exit_triggered_at should be cleared"
    assert (
        pos_after_clear["exit_unlock_at"] == "0001-01-01T00:00:00Z"
    ), "exit_unlock_at should be cleared"

    est_after_clear = query_estimate_rewards(cluster, pos_id)
    bonus_after = sum(
        int(c.get("amount", "0")) for c in est_after_clear.get("bonus_rewards", [])
    )
    assert (
        bonus_after <= bonus_before
    ), "clear-position should not increase the pending bonus window"

    add_rsp = add_to_position(cluster, owner, pos_id, add_amount)
    assert add_rsp["code"] == 0, add_rsp["raw_log"]

    pos_after_add = query_position(cluster, pos_id)["position"]
    assert int(pos_after_add["amount"]) > int(
        pos_after_clear["amount"]
    ), "position amount should grow after add-to-tier-position"


def test_tier_redelegate_flow(cluster):
    """Redelegating moves a delegated position to the destination validator."""
    owner = cluster.address("signer2")
    src_validator = get_validator_addr(cluster, 0)
    dst_validator = get_validator_addr(cluster, 1)
    amount = TIER_1_MIN * 1000

    rsp = fund_pool(cluster, "signer1", f"500000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=src_validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Initialize LastBonusAccrual before checking reward settlement in redelegate.
    rsp = claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cluster, 5)

    balance_before = cluster.balance(owner, DENOM)
    rsp = tier_redelegate(cluster, owner, pos_id, dst_validator)
    assert rsp["code"] == 0, rsp["raw_log"]

    ev = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionRedelegated",
        lambda attrs: "completion_time" in attrs,
    )
    assert ev is not None, "EventPositionRedelegated should be emitted"
    assert ev["dst_validator"].strip('"') == dst_validator

    pos = query_position(cluster, pos_id)["position"]
    assert pos["validator"] == dst_validator, "position should move to dst validator"
    assert (
        pos["delegated_shares"] != "0.000000000000000000"
    ), "position should remain delegated"

    balance_after = cluster.balance(owner, DENOM)
    assert balance_after > balance_before, "redelegation should settle pending rewards"

    wait_for_new_blocks(cluster, 2)
    rsp = claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]


@pytest.mark.slow
def test_slash_then_withdraw_succeeds(slashing_cluster):
    """Slashed delegated position still exits, undelegates, and withdraws cleanly."""
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator = get_node_validator_addr(cluster, 2)
    amount = TIER_1_MIN * 20

    rsp = fund_pool(cluster, "signer1", f"100000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]
    assert pool_balance(cluster) > 0

    before = before_ids(cluster, owner)
    rsp = lock_tier(
        cluster,
        owner,
        TIER_1_ID,
        amount,
        validator=validator,
        trigger_exit=True,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    pos_before_slash = query_position(cluster, pos_id)["position"]
    amount_before_slash = int(pos_before_slash["amount"])
    exit_unlock_at = isoparse(pos_before_slash["exit_unlock_at"])

    val_before = cluster.validator(validator)
    tokens_before = int(val_before["tokens"])

    wait_for_new_blocks(cluster, 5)
    cluster.supervisor.stopProcess(f"{cluster.chain_id}-node2")
    wait_for_new_blocks(cluster, 20)
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node2")
    wait_for_port(rpc_port(cluster.base_port(2)))
    wait_for_new_blocks(cluster, 2)

    val_after = cluster.validator(validator)
    tokens_after = int(val_after["tokens"])
    assert tokens_after == int(
        tokens_before * 0.99
    ), "validator should be slashed by 1%"
    assert val_after.get("jailed"), "validator should be jailed after downtime slash"

    pos_after_slash = query_position(cluster, pos_id)["position"]
    amount_after_slash = int(pos_after_slash["amount"])
    assert (
        amount_after_slash < amount_before_slash
    ), "position amount should decrease after validator slash"
    assert (
        pos_after_slash["delegated_shares"] != "0.000000000000000000"
    ), "position should remain delegated after slash"

    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    unbond_data = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    assert unbond_data is not None, "undelegate should emit completion_time"
    completion_time = isoparse(unbond_data["completion_time"].strip('"')) + timedelta(
        seconds=1
    )

    wait_for_block_time(cluster, completion_time)
    wait_for_new_blocks(cluster, 1)

    pos_after_undelegate = query_position(cluster, pos_id)["position"]
    withdraw_amount = int(pos_after_undelegate["amount"])
    assert withdraw_amount <= amount_after_slash

    balance_before = cluster.balance(owner, DENOM)
    rsp = withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = cluster.balance(owner, DENOM)
    assert balance_after >= balance_before + withdraw_amount - GAS_ALLOWANCE, (
        f"expected balance increase of ~{withdraw_amount}: "
        f"before={balance_before}, after={balance_after}"
    )

    try:
        query_position(cluster, pos_id)
        assert False, f"position {pos_id} should be deleted after withdraw"
    except requests.HTTPError as exc:
        assert exc.response.status_code in (404, 500)
        assert "not found" in exc.response.text.lower()


def test_autocli_lock_tier_and_queries(cluster):
    """Smoke test tieredrewards autocli tx/query paths end-to-end."""
    owner = cluster.address("ecosystem")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 2

    before = before_ids(cluster, owner)
    rsp = tx(cluster, "lock-tier", str(TIER_1_ID), str(amount), validator, from_=owner)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    position_rsp = query_command(cluster, MODULE, "position", str(pos_id))
    rest_position_rsp = query_position(cluster, pos_id)
    assert position_rsp == rest_position_rsp

    position = position_rsp["position"]
    assert position["owner"] == owner
    assert int(position["tier_id"]) == TIER_1_ID
    assert position["validator"] == validator
    assert int(position["amount"]) == amount

    owner_positions_rsp = query_command(cluster, MODULE, "positions-by-owner", owner)
    rest_owner_positions_rsp = query_positions_by_owner(cluster, owner)
    assert owner_positions_rsp == rest_owner_positions_rsp

    owner_positions = owner_positions_rsp.get("positions", [])
    assert len(owner_positions) == len(before) + 1
    assert any(
        p["owner"] == owner
        and int(p["tier_id"]) == TIER_1_ID
        and p["validator"] == validator
        and int(p["amount"]) == amount
        for p in owner_positions
    ), "positions-by-owner should include the newly created delegated position"

    tiers_rsp = query_command(cluster, MODULE, "tiers")
    rest_tiers_rsp = query_tiers(cluster)
    cli_tiers = {
        **tiers_rsp,
        "tiers": [
            {
                **tier,
                "close_only": tier.get("close_only", False),
            }
            for tier in tiers_rsp.get("tiers", [])
        ],
    }
    rest_tiers = {
        **rest_tiers_rsp,
        "tiers": [
            {
                **tier,
                "close_only": tier.get("close_only", False),
            }
            for tier in rest_tiers_rsp.get("tiers", [])
        ],
    }
    assert cli_tiers == rest_tiers

    tier_ids = {int(t["id"]) for t in tiers_rsp.get("tiers", [])}
    assert TIER_1_ID in tier_ids
    assert TIER_2_ID in tier_ids
