import json
from datetime import timedelta
from decimal import Decimal
from pathlib import Path

import pytest
import requests
from dateutil.parser import isoparse

from .tieredrewards_helpers import (
    DENOM,
    GAS_ALLOWANCE,
    TIER_1_ID,
    TIER_1_MIN,
    add_to_position,
    before_ids,
    claim_rewards,
    clear_position,
    commit_delegation,
    exit_tier_with_delegation,
    fund_pool,
    get_validator_addr,
    lock_tier,
    new_pos_id,
    query_position,
    tier_redelegate,
    tier_undelegate,
    trigger_exit,
    withdraw,
)
from .utils import (
    cluster_fixture,
    find_log_event_attrs,
    wait_for_block_time,
    wait_for_new_blocks,
)

pytestmark = [pytest.mark.tieredrewards]

ZERO_TIME = "0001-01-01T00:00:00Z"
MSG_SLASHING_UPDATE_PARAMS = "/cosmos.slashing.v1beta1.MsgUpdateParams"


@pytest.fixture(scope="module")
def cluster(worker_index, tmp_path_factory):
    "override cluster fixture for tieredrewards msgs tests"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/tieredrewards.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


@pytest.fixture(scope="function")
def slashing_cluster(worker_index, tmp_path_factory):
    """Fresh cluster for tests that modify validator state."""
    yield from cluster_fixture(
        Path(__file__).parent / "configs/tieredrewards.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("d"),
    )


# ──────────────────────────────────────────────
# MsgLockTier
# ──────────────────────────────────────────────


def test_lock_tier(cluster):
    """Lock tokens into Tier 1. Verify all position fields."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 2

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    pos = query_position(cluster, pos_id)["position"]
    assert pos["owner"] == owner
    assert int(pos["tier_id"]) == TIER_1_ID
    assert int(pos["amount"]) == amount
    assert pos["validator"] == validator
    assert pos["delegated_shares"] != "0.000000000000000000"
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["exit_unlock_at"] == ZERO_TIME


def test_lock_tier_with_trigger_exit(cluster):
    """Lock with --trigger-exit-immediately sets exit fields."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 2

    before = before_ids(cluster, owner)
    rsp = lock_tier(
        cluster, owner, TIER_1_ID, amount, validator=validator, trigger_exit=True
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] != ZERO_TIME
    assert pos["exit_unlock_at"] != ZERO_TIME

    triggered = isoparse(pos["exit_triggered_at"])
    unlock = isoparse(pos["exit_unlock_at"])
    duration = (unlock - triggered).total_seconds()
    assert duration == 5, f"exit duration should be 5s for Tier 1, got {duration}s"


# ──────────────────────────────────────────────
# MsgCommitDelegationToTier
# ──────────────────────────────────────────────


def test_commit_delegation(cluster):
    """Commit an existing x/staking delegation to a tier position."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)

    # First delegate via x/staking
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

    # Record staking shares before commit
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

    # Commit to tier
    commit_amount = TIER_1_MIN * 2
    before = before_ids(cluster, owner)
    rsp = commit_delegation(cluster, owner, validator, commit_amount, TIER_1_ID)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    pos = query_position(cluster, pos_id)["position"]
    assert pos["validator"] == validator
    assert int(pos["amount"]) == commit_amount

    # Staking delegation should have decreased
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
    assert shares_after < shares_before


def test_commit_delegation_with_trigger_exit(cluster):
    """CommitDelegationToTier with trigger_exit sets exit fields."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)

    # Delegate via x/staking
    cli = cluster.cosmos_cli()
    rsp = json.loads(
        cli.raw(
            "tx",
            "staking",
            "delegate",
            validator,
            f"{TIER_1_MIN * 5}{DENOM}",
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

    before = before_ids(cluster, owner)
    rsp = commit_delegation(
        cluster, owner, validator, TIER_1_MIN * 2, TIER_1_ID, trigger_exit=True
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] != ZERO_TIME
    assert pos["exit_unlock_at"] != ZERO_TIME


# ──────────────────────────────────────────────
# MsgTierUndelegate
# ──────────────────────────────────────────────


def test_tier_undelegate(cluster):
    """Undelegate a position after exit lock duration elapses."""
    owner = cluster.address("ecosystem")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 3

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Verify shares cleared
    pos_after = query_position(cluster, pos_id)["position"]
    assert pos_after["delegated_shares"] == "0.000000000000000000"

    # Verify completion_time event
    ev = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    assert ev is not None, "EventPositionUndelegated with completion_time not found"


def test_multiple_positions_unbonding_concurrent(cluster):
    """Multiple positions on the same validator all undelegate concurrently."""
    owner = cluster.address("ecosystem")
    validator = get_validator_addr(cluster, 0)

    # Exceed cosmos-sdk default MaxEntries (7).
    num_positions = 10

    position_ids = []
    for _ in range(num_positions):
        before = before_ids(cluster, owner)
        rsp = lock_tier(
            cluster,
            owner,
            TIER_1_ID,
            TIER_1_MIN * 2,
            validator=validator,
            trigger_exit=True,
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        position_ids.append(new_pos_id(cluster, owner, before))

    # All positions trigger-exited at creation time; wait past exit duration.
    last_pos = query_position(cluster, position_ids[-1])["position"]
    exit_unlock_at = isoparse(last_pos["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    for i, pos_id in enumerate(position_ids):
        rsp = tier_undelegate(cluster, owner, pos_id)
        assert rsp["code"] == 0, (
            f"position {i + 1}/{num_positions} undelegate should not hit "
            f"MaxEntries: {rsp['raw_log']}"
        )

    # Every position should now be undelegated.
    for pos_id in position_ids:
        pos = query_position(cluster, pos_id)["position"]
        assert (
            pos["delegated_shares"] == "0.000000000000000000"
        ), f"position {pos_id} should have zero shares after undelegate"


# ──────────────────────────────────────────────
# MsgTierRedelegate
# ──────────────────────────────────────────────


def test_tier_redelegate(cluster):
    """Redelegate a position to a different validator."""
    owner = cluster.address("signer2")
    src_validator = get_validator_addr(cluster, 0)
    dst_validator = get_validator_addr(cluster, 1)

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=src_validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = tier_redelegate(cluster, owner, pos_id, dst_validator)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["validator"] == dst_validator
    assert pos["delegated_shares"] != "0.000000000000000000"

    ev = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionRedelegated",
        lambda attrs: "completion_time" in attrs,
    )
    assert ev is not None, "EventPositionRedelegated should be emitted"
    assert ev["dst_validator"].strip('"') == dst_validator


def test_tier_redelegate_twice_fails(cluster):
    """Transitive redelegation is not allowed — redelegate back immediately fails."""
    owner = cluster.address("signer2")
    v0 = get_validator_addr(cluster, 0)
    v1 = get_validator_addr(cluster, 1)

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=v0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = tier_redelegate(cluster, owner, pos_id, v1)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Immediately redelegate back — should fail (transitive redelegation)
    rsp = tier_redelegate(cluster, owner, pos_id, v0)
    assert rsp["code"] != 0, "transitive redelegation should fail"


def test_multiple_positions_redelegate_concurrent(cluster):
    """Two positions on two validators each redelegate independently."""
    owner1 = cluster.address("signer1")
    owner2 = cluster.address("signer2")
    val_a = get_validator_addr(cluster, 0)
    val_b = get_validator_addr(cluster, 1)

    before = before_ids(cluster, owner1)
    rsp = lock_tier(cluster, owner1, TIER_1_ID, TIER_1_MIN * 2, validator=val_a)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos1_id = new_pos_id(cluster, owner1, before)

    before = before_ids(cluster, owner2)
    rsp = lock_tier(cluster, owner2, TIER_1_ID, TIER_1_MIN * 2, validator=val_b)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos2_id = new_pos_id(cluster, owner2, before)

    # pos1 redelegates A → B.
    rsp = tier_redelegate(cluster, owner1, pos1_id, val_b)
    assert rsp["code"] == 0, rsp["raw_log"]

    # pos2 redelegates B → A.
    rsp = tier_redelegate(cluster, owner2, pos2_id, val_a)
    assert (
        rsp["code"] == 0
    ), f"pos2 B→A must not be blocked by pos1 A→B: {rsp['raw_log']}"

    pos1 = query_position(cluster, pos1_id)["position"]
    pos2 = query_position(cluster, pos2_id)["position"]
    assert pos1["validator"] == val_b
    assert pos2["validator"] == val_a


# ──────────────────────────────────────────────
# MsgTriggerExitFromTier
# ──────────────────────────────────────────────


def test_trigger_exit(cluster):
    """Trigger exit sets exit_triggered_at and exit_unlock_at."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] != ZERO_TIME
    assert pos["exit_unlock_at"] != ZERO_TIME

    triggered = isoparse(pos["exit_triggered_at"])
    unlock = isoparse(pos["exit_unlock_at"])
    assert (unlock - triggered).total_seconds() == 5


# ──────────────────────────────────────────────
# MsgAddToTierPosition
# ──────────────────────────────────────────────


def test_add_to_position_delegated(cluster):
    """Add tokens to a delegated position."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    initial = TIER_1_MIN * 2
    add_amount = TIER_1_MIN

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, initial, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = add_to_position(cluster, owner, pos_id, add_amount)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["amount"]) == initial + add_amount
    assert pos["delegated_shares"] != "0.000000000000000000"


# ──────────────────────────────────────────────
# MsgClearPosition
# ──────────────────────────────────────────────


def test_clear_position(cluster):
    """Clear exit resets exit fields and allows adding to position again."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos_after = query_position(cluster, pos_id)["position"]
    assert pos_after["exit_triggered_at"] == ZERO_TIME
    assert pos_after["exit_unlock_at"] == ZERO_TIME

    # Can add to position again after clearing
    rsp = add_to_position(cluster, owner, pos_id, TIER_1_MIN)
    assert rsp["code"] == 0, rsp["raw_log"]


# ──────────────────────────────────────────────
# MsgClaimTierRewards
# ──────────────────────────────────────────────


def test_claim_rewards(cluster):
    """Claim rewards on a delegated position."""
    owner = cluster.address("signer2")
    validator = get_validator_addr(cluster, 0)

    rsp = fund_pool(cluster, "signer1", f"50000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 5, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    wait_for_new_blocks(cluster, 10)

    balance_before = cluster.balance(owner, DENOM)
    rsp = claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = cluster.balance(owner, DENOM)
    assert (
        balance_after > balance_before
    ), "balance should increase after claiming rewards"

    ev = find_log_event_attrs(
        rsp["events"], "chainmain.tieredrewards.v1.EventTierRewardsClaimed"
    )
    assert ev is not None, "EventTierRewardsClaimed not found"


def test_claim_rewards_batch(cluster):
    """Batch claim rewards for two positions in a single transaction."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)

    rsp = fund_pool(cluster, "community", f"50000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    # Create two positions
    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 3, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id1 = new_pos_id(cluster, owner, before)

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 3, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id2 = new_pos_id(cluster, owner, before)

    wait_for_new_blocks(cluster, 10)

    balance_before = cluster.balance(owner, DENOM)
    rsp = claim_rewards(cluster, owner, pos_id1, pos_id2)
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = cluster.balance(owner, DENOM)
    assert (
        balance_after > balance_before
    ), "balance should increase after batch claiming rewards"

    ev = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventTierRewardsClaimed",
    )
    assert ev is not None, "EventTierRewardsClaimed not found"


# ──────────────────────────────────────────────
# MsgWithdrawFromTier
# ──────────────────────────────────────────────


def test_withdraw(cluster):
    """Full exit flow: lock → exit → wait → undelegate → wait unbonding → withdraw."""
    owner = cluster.address("ecosystem")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 3

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Trigger exit
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    # Undelegate
    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    unbond_data = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    completion_time = isoparse(unbond_data["completion_time"].strip('"')) + timedelta(
        seconds=1
    )
    wait_for_block_time(cluster, completion_time)
    wait_for_new_blocks(cluster, 1)

    # Withdraw
    balance_before = cluster.balance(owner, DENOM)
    rsp = withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = cluster.balance(owner, DENOM)
    assert balance_after >= balance_before + amount - GAS_ALLOWANCE

    # Position should be deleted
    try:
        query_position(cluster, pos_id)
        assert False, f"position {pos_id} should be deleted after withdraw"
    except requests.HTTPError as exc:
        assert exc.response.status_code in (404, 500)
        assert "not found" in exc.response.text.lower()


def test_exit_tier_with_delegation(cluster):
    """Exit tier position by transferring delegation back to owner (full exit)."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 2

    # Lock with trigger exit
    before = before_ids(cluster, owner)
    rsp = lock_tier(
        cluster, owner, TIER_1_ID, amount, validator=validator, trigger_exit=True
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Fund rewards pool
    fund_pool(cluster, "community", f"10000000{DENOM}")

    # Wait for exit duration to elapse
    pos = query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    # Exit with delegation (full amount)
    rsp = exit_tier_with_delegation(cluster, owner, pos_id, amount)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Position should be deleted
    try:
        query_position(cluster, pos_id)
        assert False, f"position {pos_id} should be deleted"
    except requests.HTTPError as exc:
        assert exc.response.status_code in (404, 500)

    # Verify event
    ev = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventExitTierWithDelegation",
    )
    assert ev is not None, "EventExitTierWithDelegation not found"

    # User should have a staking delegation on the validator
    cli = cluster.cosmos_cli(0)
    del_resp = json.loads(
        cli.raw(
            "query",
            "staking",
            "delegation",
            owner,
            validator,
            node=cli.node_rpc,
            output="json",
        )
    )
    shares = del_resp["delegation_response"]["delegation"]["shares"]
    assert (
        int(shares.split(".")[0]) > 0
    ), "owner should have staking delegation after exit"


def test_exit_tier_with_delegation_partial(cluster):
    """Partial exit: transfer half the position back to owner delegation."""
    owner = cluster.address("signer2")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 4

    # Lock with trigger exit
    before = before_ids(cluster, owner)
    rsp = lock_tier(
        cluster, owner, TIER_1_ID, amount, validator=validator, trigger_exit=True
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Fund rewards pool
    fund_pool(cluster, "community", f"10000000{DENOM}")

    # Wait for exit duration to elapse
    pos = query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    # Partial exit (half)
    half = amount // 2
    rsp = exit_tier_with_delegation(cluster, owner, pos_id, half)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Position should still exist with reduced amount
    pos_after = query_position(cluster, pos_id)["position"]
    assert (
        int(pos_after["amount"]) < amount
    ), "amount should be reduced after partial exit"
    assert pos_after["validator"] != "", "position should still be delegated"

    # User should have a staking delegation
    cli = cluster.cosmos_cli(0)
    del_resp = json.loads(
        cli.raw(
            "query",
            "staking",
            "delegation",
            owner,
            validator,
            node=cli.node_rpc,
            output="json",
        )
    )
    shares = del_resp["delegation_response"]["delegation"]["shares"]
    assert (
        int(shares.split(".")[0]) > 0
    ), "owner should have staking delegation after partial exit"
