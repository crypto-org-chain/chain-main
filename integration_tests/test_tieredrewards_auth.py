from datetime import timedelta
from pathlib import Path

import pytest
from dateutil.parser import isoparse

from .tieredrewards_helpers import (
    DENOM,
    MODULE,
    MSG_ADD_TIER,
    MSG_DELETE_TIER,
    MSG_UPDATE_PARAMS,
    MSG_UPDATE_TIER,
    TIER_3_ID,
    approve_tieredrewards_proposal,
    before_ids,
    claim_rewards,
    fund_pool,
    get_validator_addr,
    lock_tier,
    new_pos_id,
    pool_balance,
    query_position,
    query_tiers,
    tier_undelegate,
    trigger_exit,
    withdraw,
)
from .utils import (
    cluster_fixture,
    find_log_event_attrs,
    query_command,
    submit_gov_proposal,
    wait_for_block_time,
    wait_for_new_blocks,
)

pytestmark = [pytest.mark.tieredrewards]


@pytest.fixture(scope="module")
def cluster(worker_index, tmp_path_factory):
    "override cluster fixture for tieredrewards auth tests"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/tieredrewards.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


# ──────────────────────────────────────────────
# AddTier
# ──────────────────────────────────────────────


def test_add_tier(cluster):
    """MsgAddTier proposal creates a new Tier 3 with close_only=true."""
    # Let the fresh cluster settle before submitting governance proposals
    wait_for_new_blocks(cluster, 2)

    bonus_apy = "0.050000000000000000"
    close_only = True
    min_lock_amount = "2000000"
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_ADD_TIER,
        {
            "tier": {
                "id": TIER_3_ID,
                "exit_duration": "5s",
                "bonus_apy": bonus_apy,
                "min_lock_amount": min_lock_amount,
                "close_only": close_only,
            }
        },
        title="Add Tier 3 (close_only)",
        summary="Add testing tier",
    )
    approve_tieredrewards_proposal(cluster, rsp, msg=f",{MSG_ADD_TIER}")

    result = query_tiers(cluster)
    tier3 = next(
        (t for t in result.get("tiers", []) if int(t["id"]) == TIER_3_ID), None
    )
    assert tier3 is not None, f"Tier {TIER_3_ID} not found after AddTier proposal"
    assert (
        tier3.get("close_only") is close_only
    ), f"Tier 3 should be close_only: {close_only}"
    assert tier3["bonus_apy"] == bonus_apy
    assert tier3["min_lock_amount"] == min_lock_amount


def test_add_tier_close_only_rejects_lock(cluster):
    """Locking on a close_only tier fails."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)

    rsp = lock_tier(cluster, owner, TIER_3_ID, 2_000_000, validator=validator)
    assert rsp["code"] != 0, "lock on close_only tier should fail"
    assert "close only" in rsp["raw_log"].lower()


def test_add_tier_already_exists(cluster):
    """Adding a tier with an existing ID fails at execution (PROPOSAL_STATUS_FAILED)."""
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_ADD_TIER,
        {
            "tier": {
                "id": TIER_3_ID,
                "exit_duration": "5s",
                "bonus_apy": "0.050000000000000000",
                "min_lock_amount": "2000000",
                "close_only": False,
            }
        },
        title="Add duplicate Tier 3",
        summary="Should fail",
    )
    # Proposal passes vote but execution fails — expect FAILED status
    approve_tieredrewards_proposal(
        cluster, rsp, msg=f",{MSG_ADD_TIER}", expect_status="PROPOSAL_STATUS_FAILED"
    )

    # Tier 3 should still be the original (close_only=True)
    result = query_tiers(cluster)
    tier3 = next(
        (t for t in result.get("tiers", []) if int(t["id"]) == TIER_3_ID), None
    )
    assert tier3 is not None
    assert tier3.get("close_only") is True, "original tier should be unchanged"


# ──────────────────────────────────────────────
# UpdateTier
# ──────────────────────────────────────────────


def test_update_tier_open_close_only(cluster):
    """MsgUpdateTier opens Tier 3 (close_only=false) and sets bonus_apy to 6%."""
    new_apy = "0.060000000000000000"
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_UPDATE_TIER,
        {
            "tier": {
                "id": TIER_3_ID,
                "exit_duration": "5s",
                "bonus_apy": new_apy,
                "min_lock_amount": "2000000",
                "close_only": False,
            }
        },
        title="Update Tier 3",
        summary="Open Tier 3 and set bonus_apy to 6%",
    )
    approve_tieredrewards_proposal(cluster, rsp, msg=f",{MSG_UPDATE_TIER}")

    result = query_tiers(cluster)
    tier3 = next(
        (t for t in result.get("tiers", []) if int(t["id"]) == TIER_3_ID), None
    )
    assert tier3 is not None, "Tier 3 not found after update"
    assert tier3["bonus_apy"] == new_apy
    assert tier3.get("close_only") is not True


def test_update_tier_lock_succeeds_after_open(cluster):
    """Locking on Tier 3 succeeds now that close_only is false.

    Also funds the pool and claims rewards to sanity-check the flow.
    """
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)

    rsp = fund_pool(cluster, "signer1", f"1000000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_3_ID, 2_000_000, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["tier_id"]) == TIER_3_ID
    assert int(pos["amount"]) == 2_000_000

    # Let some bonus accrue
    wait_for_new_blocks(cluster, 3)

    # Claim rewards succeeds
    rsp = claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]


def test_update_tier_apy_claims_rewards(cluster):
    """Changing bonus_apy triggers claimRewardsForTier at the old rate.

    Tier 3 APY: 6% → 3%. The position owner should receive rewards and
    the pool balance should decrease.
    """
    owner = cluster.address("signer1")

    # Confirm the signer1 position on Tier 3 exists
    result = query_command(cluster, MODULE, "positions-by-owner", owner)
    tier3_pos = next(
        p for p in result.get("positions", []) if int(p["tier_id"]) == TIER_3_ID
    )
    assert tier3_pos is not None

    balance_before = cluster.balance(owner, DENOM)
    pool_before = pool_balance(cluster)
    new_apy = "0.030000000000000000"

    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_UPDATE_TIER,
        {
            "tier": {
                "id": TIER_3_ID,
                "exit_duration": "5s",
                "bonus_apy": new_apy,
                "min_lock_amount": "2000000",
                "close_only": False,
            }
        },
        title="Update Tier 3 APY to 3%",
        summary="Triggers reward claim at old 6% rate",
    )
    approve_tieredrewards_proposal(cluster, rsp, msg=f",{MSG_UPDATE_TIER}")

    balance_after = cluster.balance(owner, DENOM)
    pool_after = pool_balance(cluster)

    assert balance_after > balance_before, (
        f"owner should receive rewards on APY change: "
        f"before={balance_before}, after={balance_after}"
    )
    assert pool_after < pool_before, (
        f"pool should decrease from bonus payout: "
        f"before={pool_before}, after={pool_after}"
    )

    # Verify the APY is updated
    result = query_tiers(cluster)
    tier3 = next(
        (t for t in result.get("tiers", []) if int(t["id"]) == TIER_3_ID), None
    )
    assert tier3["bonus_apy"] == new_apy


def test_update_tier_non_apy_no_claim(cluster):
    """Changing exit_duration without changing APY does not trigger rewards claiming.

    Verified by checking that the owner balance is unchanged.
    """
    owner = cluster.address("signer1")

    # Confirm the signer1 position on Tier 3 exists
    result = query_command(cluster, MODULE, "positions-by-owner", owner)
    tier3_pos = next(
        p for p in result.get("positions", []) if int(p["tier_id"]) == TIER_3_ID
    )
    assert tier3_pos is not None
    balance_before = cluster.balance(owner, DENOM)

    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_UPDATE_TIER,
        {
            "tier": {
                "id": TIER_3_ID,
                "exit_duration": "10s",
                "bonus_apy": "0.030000000000000000",
                "min_lock_amount": "2000000",
                "close_only": False,
            }
        },
        title="Update Tier 3 exit_duration",
        summary="Only exit_duration changes, no APY change",
    )
    approve_tieredrewards_proposal(cluster, rsp, msg=f",{MSG_UPDATE_TIER}")

    # No rewards should have been paid out to the owner.
    assert (
        cluster.balance(owner, DENOM) == balance_before
    ), "owner balance should not change when APY is unchanged"

    # Verify exit_duration is updated
    result = query_tiers(cluster)
    tier3 = next(
        (t for t in result.get("tiers", []) if int(t["id"]) == TIER_3_ID), None
    )
    assert tier3["exit_duration"] == "10s"

    # Verify the new exit_duration applies to new positions
    owner = cluster.address("signer2")
    validator = get_validator_addr(cluster, 0)

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_3_ID, 2_000_000, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    triggered_at = isoparse(pos["exit_triggered_at"])
    unlock_at = isoparse(pos["exit_unlock_at"])
    actual_duration = (unlock_at - triggered_at).total_seconds()

    assert actual_duration == 10, (
        f"new position should use updated exit_duration (10s), "
        f"got {actual_duration}s"
    )


def test_update_tier_min_lock_affects_new_positions(cluster):
    """Increasing min_lock_amount blocks new positions below the threshold
    but does not affect existing ones.
    """
    owner = cluster.address("signer2")
    validator = get_validator_addr(cluster, 0)

    # Lock at 5M succeeds under the current min_lock_amount (2M)
    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_3_ID, 5_000_000, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    new_pos_id(cluster, owner, before)

    # Increase min_lock_amount from 2M to 10M
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_UPDATE_TIER,
        {
            "tier": {
                "id": TIER_3_ID,
                "exit_duration": "10s",
                "bonus_apy": "0.030000000000000000",
                "min_lock_amount": "10000000",
                "close_only": False,
            }
        },
        title="Increase Tier 3 min_lock",
        summary="min_lock_amount 2M -> 10M",
    )
    approve_tieredrewards_proposal(cluster, rsp, msg=f",{MSG_UPDATE_TIER}")

    # Same 5M lock now fails under the new minimum
    rsp = lock_tier(cluster, owner, TIER_3_ID, 5_000_000, validator=validator)
    assert rsp["code"] != 0, "lock below new min_lock_amount should fail"

    # Lock at new minimum succeeds
    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_3_ID, 10_000_000, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    new_pos_id(cluster, owner, before)


# ──────────────────────────────────────────────
# DeleteTier
# ──────────────────────────────────────────────


def test_delete_tier_with_positions_fails(cluster):
    """DeleteTier fails at execution when the tier has active positions."""
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_DELETE_TIER,
        {"id": TIER_3_ID},
        title="Delete Tier 3",
        summary="Should fail — has active positions",
    )
    # Proposal passes vote but execution fails — expect FAILED status
    approve_tieredrewards_proposal(
        cluster, rsp, msg=f",{MSG_DELETE_TIER}", expect_status="PROPOSAL_STATUS_FAILED"
    )

    # Tier 3 should still exist
    result = query_tiers(cluster)
    ids = {int(t["id"]) for t in result.get("tiers", [])}
    assert TIER_3_ID in ids, "Tier 3 should still exist after failed delete"


def test_delete_tier_after_withdraw(cluster):
    """After all positions are withdrawn, DeleteTier succeeds."""
    # Withdraw all Tier 3 positions for both signers
    for signer in ("signer1", "signer2"):
        owner = cluster.address(signer)
        result = query_command(cluster, MODULE, "positions-by-owner", owner)
        for pos in result.get("positions", []):
            if int(pos["tier_id"]) != TIER_3_ID:
                continue
            pos_id = int(pos["id"])

            # Trigger exit if not already triggered
            if pos.get("exit_triggered_at", "0001") == "0001-01-01T00:00:00Z":
                rsp = trigger_exit(cluster, owner, pos_id)
                assert rsp["code"] == 0, rsp["raw_log"]

            # Wait for exit to unlock
            pos_data = query_position(cluster, pos_id)["position"]
            exit_unlock_at = isoparse(pos_data["exit_unlock_at"])
            wait_for_block_time(cluster, exit_unlock_at)
            wait_for_new_blocks(cluster, 1)

            # Undelegate if still delegated
            if pos_data.get("delegated_shares", "0") != "0.000000000000000000":
                rsp = tier_undelegate(cluster, owner, pos_id)
                assert rsp["code"] == 0, rsp["raw_log"]

                unbond_data = find_log_event_attrs(
                    rsp["events"],
                    "chainmain.tieredrewards.v1.EventPositionUndelegated",
                    lambda attrs: "completion_time" in attrs,
                )
                if unbond_data:
                    completion = isoparse(
                        unbond_data["completion_time"].strip('"')
                    ) + timedelta(seconds=1)
                    wait_for_block_time(cluster, completion)
                    wait_for_new_blocks(cluster, 1)

            # Withdraw
            rsp = withdraw(cluster, owner, pos_id)
            assert rsp["code"] == 0, rsp["raw_log"]

    # Now delete should succeed
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_DELETE_TIER,
        {"id": TIER_3_ID},
        title="Delete Tier 3",
        summary="No active positions remaining",
    )
    approve_tieredrewards_proposal(cluster, rsp, msg=f",{MSG_DELETE_TIER}")

    result = query_tiers(cluster)
    ids = {int(t["id"]) for t in result.get("tiers", [])}
    assert (
        TIER_3_ID not in ids
    ), f"Tier {TIER_3_ID} should be removed after DeleteTier proposal"


# ──────────────────────────────────────────────
# UpdateParams
# ──────────────────────────────────────────────


def test_update_params(cluster):
    """MsgUpdateParams sets target_base_rewards_rate to 0.23"""
    params = query_command(cluster, MODULE, "params")["params"]
    params["target_base_rewards_rate"] = "0.230000000000000000"

    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_UPDATE_PARAMS,
        {"params": params},
        title="Set rate to 0.23",
        summary="Set target_base_rewards_rate to 0.23",
    )
    approve_tieredrewards_proposal(cluster, rsp, msg=f",{MSG_UPDATE_PARAMS}")

    updated = query_command(cluster, MODULE, "params")["params"]
    assert (
        float(updated["target_base_rewards_rate"]) == 0.23
    ), "target_base_rewards_rate should be 0.23 after governance update"
