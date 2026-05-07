import json
from datetime import timedelta
from pathlib import Path

import pytest
import requests
from dateutil.parser import isoparse
from pystarport.ports import rpc_port

from .tieredrewards_helpers import (
    DENOM,
    GAS_ALLOWANCE,
    MODULE,
    TIER_1_ID,
    TIER_1_MIN,
    TIER_2_ID,
    TIER_2_MIN,
    add_to_position,
    before_ids,
    claim_rewards,
    clear_position,
    commit_delegation,
    exit_tier_with_delegation,
    fund_pool,
    get_node_validator_addr,
    get_validator_addr,
    lock_tier,
    new_pos_id,
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
    approve_proposal,
    cluster_fixture,
    find_log_event_attrs,
    query_command,
    submit_gov_proposal,
    wait_for_block_time,
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = [pytest.mark.tieredrewards]

ZERO_TIME = "0001-01-01T00:00:00Z"
MSG_SLASHING_UPDATE_PARAMS = "/cosmos.slashing.v1beta1.MsgUpdateParams"


# ──────────────────────────────────────────────
# Cluster fixtures
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
    """Fresh cluster for tests that modify validator state."""
    yield from cluster_fixture(
        Path(__file__).parent / "configs/tieredrewards.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("d"),
    )


# ──────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────


def _exit_undelegate_withdraw(cluster, owner, pos_id):
    """Drive a position through exit → wait → undelegate → wait → withdraw.

    Returns the withdrawn amount.
    """
    pos = query_position(cluster, pos_id)["position"]

    # Trigger exit if not already triggered
    if pos["exit_triggered_at"] == ZERO_TIME:
        rsp = trigger_exit(cluster, owner, pos_id)
        assert rsp["code"] == 0, rsp["raw_log"]
        pos = query_position(cluster, pos_id)["position"]

    # Wait for exit to elapse
    exit_unlock_at = isoparse(pos["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    # Undelegate (settles pending rewards)
    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    unbond_data = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    assert unbond_data is not None
    completion = isoparse(unbond_data["completion_time"].strip('"')) + timedelta(
        seconds=1
    )
    wait_for_block_time(cluster, completion)
    wait_for_new_blocks(cluster, 1)

    # Withdraw
    balance_before = cluster.balance(owner, DENOM)
    rsp = withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    balance_after = cluster.balance(owner, DENOM)

    # Verify position deleted
    try:
        query_position(cluster, pos_id)
        assert False, f"position {pos_id} should be deleted"
    except requests.HTTPError:
        pass

    return balance_after - balance_before


# Tests that use this function should be marked with flaky as it is
# not guaranteed that the redelegation will land within the 2-block
# window (creation_height >= infractionHeight) to be eligible for
# a slash.


def _setup_redeleg_slash(
    cluster,
    owner,
    validator2,
    validator0,
    slash_fraction="1.000000000000000000",
    trigger_exit_before_redelegate=False,
):
    """Shared setup: gov proposal for downtime slash, lock on v2, stop v2,
    wait, redelegate to v0, wait for slash. Returns position ID.

    Uses signed_blocks_window=20 so we can time the redelegate
    to land within the 2-block window (creation_height >= infractionHeight).

    If trigger_exit_before_redelegate is True, locks with
    --trigger-exit-immediately so exit is in progress when the
    redeleg slash fires.
    """
    slashing_params = query_command(cluster, "slashing", "params")["params"]
    slashing_params["slash_fraction_downtime"] = slash_fraction
    slashing_params["signed_blocks_window"] = "20"
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_SLASHING_UPDATE_PARAMS,
        {"params": slashing_params},
        title=f"{slash_fraction} downtime slash",
        summary="slash via redelegation for test",
    )
    approve_proposal(cluster, rsp, msg=f",{MSG_SLASHING_UPDATE_PARAMS}")

    before = before_ids(cluster, owner)
    # lock in tier 2 (exit duration = 60s) so that position will still be exiting
    # if clearing position later
    rsp = lock_tier(
        cluster,
        owner,
        TIER_2_ID,
        TIER_2_MIN * 2,
        validator=validator2,
        trigger_exit=trigger_exit_before_redelegate,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    amount_before = int(query_position(cluster, pos_id)["position"]["amount"])

    # Stop v2 FIRST, then redelegate so creation_height >= infractionHeight
    cluster.supervisor.stopProcess(f"{cluster.chain_id}-node2")
    wait_for_new_blocks(cluster, 9)

    rsp = tier_redelegate(cluster, owner, pos_id, validator0)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Wait for slash
    wait_for_new_blocks(cluster, 15)
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node2")
    wait_for_port(rpc_port(cluster.base_port(2)))
    wait_for_new_blocks(cluster, 2)

    pos = query_position(cluster, pos_id)["position"]
    amount_after = int(pos["amount"])
    assert amount_after < amount_before, (
        f"redeleg slash should reduce amount: "
        f"before={amount_before}, after={amount_after}"
    )

    return pos_id


# ──────────────────────────────────────────────
# Basic entry - exit flows
# ──────────────────────────────────────────────


def test_basic_entry_exit_flow(cluster):
    """lock → trigger exit → wait for exit →
    undelegate → wait for unbonding → withdraw.
    """
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 3

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)
    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["amount"]) == amount
    assert pos["validator"] == validator

    returned = _exit_undelegate_withdraw(cluster, owner, pos_id)
    assert returned >= amount - GAS_ALLOWANCE


def test_entry_add_then_exit_flow(cluster):
    """lock → add to position → trigger exit →
    wait for exit → undelegate →
    wait for unbonding → withdraw.
    """
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

    returned = _exit_undelegate_withdraw(cluster, owner, pos_id)
    assert returned >= initial + add_amount - GAS_ALLOWANCE


def test_basic_commit_delegation_exit_flow(cluster):
    """commit delegation → trigger exit → wait for exit →
    undelegate → wait for unbonding → withdraw.
    """
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 3

    # Delegate via x/staking first
    cli = cluster.cosmos_cli()

    rsp = json.loads(
        cli.raw(
            "tx",
            "staking",
            "delegate",
            validator,
            f"{amount}{DENOM}",
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

    # Commit to tier
    before = before_ids(cluster, owner)
    rsp = commit_delegation(cluster, owner, validator, amount, TIER_1_ID)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["amount"]) == amount
    assert pos["validator"] == validator

    returned = _exit_undelegate_withdraw(cluster, owner, pos_id)
    assert returned >= amount - GAS_ALLOWANCE


# ──────────────────────────────────────────────
# Clear position flows
# ──────────────────────────────────────────────


def test_clear_position_before_exit_elapsed(cluster):
    """lock → trigger exit (before elapsed) → clear position → back to bonded.

    Verifies ClearPosition works during exit in-progress.
    """
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

    # Clear immediately — don't wait for exit to elapse
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["exit_unlock_at"] == ZERO_TIME
    assert pos["delegated_shares"] != "0.000000000000000000"


def test_clear_position_after_exit_elapsed(cluster):
    """lock → trigger exit → wait for exit → clear position → back to bonded.

    Verifies ClearPosition works after exit has elapsed.
    Position returns to fully delegated with no exit.
    """
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

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["exit_unlock_at"] == ZERO_TIME
    assert pos["delegated_shares"] != "0.000000000000000000"


def test_clear_position_and_reexit_flow(cluster):
    """lock → trigger exit → clear position →
    back to bonded → trigger exit → wait for exit →
    undelegate → wait for unbonding → withdraw.

    Verifies re-exit after cancel works end-to-end.
    """
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 3

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # First exit → cancel
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME

    # Re-exit → full withdraw
    returned = _exit_undelegate_withdraw(cluster, owner, pos_id)
    assert returned >= amount - GAS_ALLOWANCE


def test_clear_position_add_exit_flow(cluster):
    """lock → trigger exit → clear position →
    back to bonded → add to position → trigger exit →
    wait for exit → undelegate →
    wait for unbonding → withdraw.

    Verifies add-to-position after cancel, then full exit.
    """
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    initial = TIER_1_MIN * 2
    add_amount = TIER_1_MIN

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, initial, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Exit → cancel → add
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = add_to_position(cluster, owner, pos_id, add_amount)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Full exit
    returned = _exit_undelegate_withdraw(cluster, owner, pos_id)
    assert returned >= initial + add_amount - GAS_ALLOWANCE


def test_clear_position_exit_elapsed(cluster):
    """lock → trigger exit → wait for exit elapsed →
    clear position → back to bonded with no exit.

    ClearPosition after exit elapsed on a redelegated position
    cancels the exit and returns to bonded on the new validator.
    """
    owner = cluster.address("signer2")
    v0 = get_validator_addr(cluster, 0)

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=v0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 1)

    # Exit elapsed on position → clear
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["exit_unlock_at"] == ZERO_TIME
    assert pos["validator"] == v0
    assert pos["delegated_shares"] != "0.000000000000000000"
    assert int(pos["amount"]) > 0


def test_clear_position_redeleg_exit_elapsed(cluster):
    """lock → redelegate → trigger exit → wait for exit elapsed →
    clear position → back to bonded with no exit.

    ClearPosition after exit elapsed on a redelegated position
    cancels the exit and returns to bonded on the new validator.
    """
    owner = cluster.address("signer2")
    v0 = get_validator_addr(cluster, 0)
    v1 = get_validator_addr(cluster, 1)

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=v0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = tier_redelegate(cluster, owner, pos_id, v1)
    assert rsp["code"] == 0, rsp["raw_log"]

    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 1)

    # Exit elapsed on redelegated position → clear
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["exit_unlock_at"] == ZERO_TIME
    assert pos["validator"] == v1
    assert pos["delegated_shares"] != "0.000000000000000000"
    assert int(pos["amount"]) > 0


# ──────────────────────────────────────────────
# Redelegate flows
# ──────────────────────────────────────────────


def test_redelegate_full_exit(cluster):
    """lock → redelegate → trigger exit →
    wait for exit → undelegate →
    wait for unbonding → withdraw.

    Proves redelegate doesn't break the exit lifecycle.
    """
    owner = cluster.address("signer2")
    v0 = get_validator_addr(cluster, 0)
    v1 = get_validator_addr(cluster, 1)
    amount = TIER_1_MIN * 3

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=v0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Redelegate
    rsp = tier_redelegate(cluster, owner, pos_id, v1)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["validator"] == v1
    assert int(pos["amount"]) == amount

    returned = _exit_undelegate_withdraw(cluster, owner, pos_id)
    assert returned >= amount - GAS_ALLOWANCE


def test_redelegate_during_exit(cluster):
    """lock → trigger exit → redelegate → redelegated with exit in progress.

    Verifies redelegate is allowed during exit in-progress and
    exit state is preserved on the new validator.
    """
    owner = cluster.address("signer2")
    v0 = get_validator_addr(cluster, 0)
    v1 = get_validator_addr(cluster, 1)

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=v0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Redelegate while exit is in progress
    rsp = tier_redelegate(cluster, owner, pos_id, v1)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["validator"] == v1
    assert pos["exit_triggered_at"] != ZERO_TIME
    assert pos["exit_unlock_at"] != ZERO_TIME
    assert pos["delegated_shares"] != "0.000000000000000000"


def test_redelegate_exit_elapsed_blocked(cluster):
    """lock → trigger exit → wait for exit → redelegate → ErrExitElapsed.

    Verifies redelegate is blocked after exit has elapsed.
    """
    owner = cluster.address("signer2")
    v0 = get_validator_addr(cluster, 0)
    v1 = get_validator_addr(cluster, 1)

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=v0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    rsp = tier_redelegate(cluster, owner, pos_id, v1)
    assert rsp["code"] != 0, "redelegate after exit elapsed should fail"
    assert "exit lock duration" in rsp["raw_log"].lower()


def test_redelegate_add_exit(cluster):
    """lock → redelegate → add to position →
    trigger exit → wait for exit → undelegate →
    wait for unbonding → withdraw.

    Verifies add-to-position on a redelegated position, then full exit.
    """
    owner = cluster.address("signer2")
    v0 = get_validator_addr(cluster, 0)
    v1 = get_validator_addr(cluster, 1)
    initial = TIER_1_MIN * 2
    add_amount = TIER_1_MIN

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, initial, validator=v0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = tier_redelegate(cluster, owner, pos_id, v1)
    assert rsp["code"] == 0, rsp["raw_log"]

    rsp = add_to_position(cluster, owner, pos_id, add_amount)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["validator"] == v1
    assert int(pos["amount"]) >= initial + add_amount

    returned = _exit_undelegate_withdraw(cluster, owner, pos_id)
    assert returned >= initial + add_amount - GAS_ALLOWANCE


def test_complex_redelegate_flow(cluster):
    """lock → trigger exit → clear position →
    back to bonded → add to position → redelegate →
    trigger exit → wait for exit → undelegate →
    wait for unbonding → withdraw.

    The most complex happy path: exit, cancel, add, redelegate, re-exit,
    then full withdrawal.
    """
    owner = cluster.address("signer2")
    v0 = get_validator_addr(cluster, 0)
    v1 = get_validator_addr(cluster, 1)
    initial = TIER_1_MIN * 2
    add_amount = TIER_1_MIN

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, initial, validator=v0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Exit → cancel
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Add → redelegate
    rsp = add_to_position(cluster, owner, pos_id, add_amount)
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = tier_redelegate(cluster, owner, pos_id, v1)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["validator"] == v1
    total = initial + add_amount
    assert int(pos["amount"]) >= total

    # Full exit via redelegate
    returned = _exit_undelegate_withdraw(cluster, owner, pos_id)
    assert returned >= total - GAS_ALLOWANCE


def test_redelegate_twice_during_exit(cluster):
    """lock → trigger exit → redelegate →
    redelegated with exit in progress →
    redelegate → SDK block.

    Transitive redelegation is blocked even during exit in-progress.
    """
    owner = cluster.address("signer2")
    v0 = get_validator_addr(cluster, 0)
    v1 = get_validator_addr(cluster, 1)

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=v0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    rsp = tier_redelegate(cluster, owner, pos_id, v1)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Second redelegate back — transitive, should fail
    rsp = tier_redelegate(cluster, owner, pos_id, v0)
    assert rsp["code"] != 0, "transitive redelegate should fail"


# ──────────────────────────────────────────────
# ExitTierWithDelegation
# ──────────────────────────────────────────────


def test_exit_tier_partial_then_undelegate(cluster):
    """Partial ExitTierWithDelegation → TierUndelegate remainder → Withdraw."""
    owner = cluster.address("ecosystem")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 4

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

    fund_pool(cluster, "community", f"10000000{DENOM}")
    pos = query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    # Partial exit: half
    half = amount // 2
    rsp = exit_tier_with_delegation(cluster, owner, pos_id, half)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos_after = query_position(cluster, pos_id)["position"]
    assert pos_after["validator"] != ""
    remaining = int(pos_after["amount"])
    assert remaining > 0

    # Undelegate the remainder
    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos_undel = query_position(cluster, pos_id)["position"]
    assert pos_undel["delegated_shares"] == "0.000000000000000000"

    # Wait for unbonding to complete
    ev = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    completion = isoparse(ev["completion_time"].strip('"')) + timedelta(seconds=1)
    wait_for_block_time(cluster, completion)
    wait_for_new_blocks(cluster, 1)

    # Withdraw
    bal_before = cluster.balance(owner, DENOM)
    rsp = withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    bal_after = cluster.balance(owner, DENOM)
    assert bal_after > bal_before

    # Position should be deleted
    try:
        query_position(cluster, pos_id)
        assert False, "position should be deleted"
    except requests.HTTPError as exc:
        assert exc.response.status_code in (404, 500)


def test_exit_tier_partial_then_full_exit(cluster):
    """Two ExitTierWithDelegation calls: partial then full remainder."""
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 4

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

    fund_pool(cluster, "community", f"10000000{DENOM}")
    pos = query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    # First: partial exit (half)
    half = amount // 2
    rsp = exit_tier_with_delegation(cluster, owner, pos_id, half)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos_after = query_position(cluster, pos_id)["position"]
    remaining = int(pos_after["amount"])
    assert remaining > 0

    # Second: exit the remainder (full exit)
    rsp = exit_tier_with_delegation(cluster, owner, pos_id, remaining)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Verify full_exit in event
    ev = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventExitTierWithDelegation",
    )
    assert ev is not None
    assert ev["full_exit"] == "true"

    # Position should be deleted
    try:
        query_position(cluster, pos_id)
        assert False, "position should be deleted"
    except requests.HTTPError as exc:
        assert exc.response.status_code in (404, 500)


# ──────────────────────────────────────────────
# Reward flows
# ──────────────────────────────────────────────


def test_bonus_stops_after_exit_unlock(cluster):
    """After exit_unlock_at, bonus rewards drop to 0.

    Verifies bonus > 0 before exit, bonus = 0 after claiming post-exit.
    """
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 1000

    rsp = fund_pool(cluster, "signer1", f"1000000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Let bonus accrue
    wait_for_new_blocks(cluster, 15)

    # Bonus > 0 before exit
    est = query_estimate_rewards(cluster, pos_id)
    bonus = sum(int(c.get("amount", "0")) for c in est.get("bonus_rewards", []))
    assert bonus > 0, "bonus should be > 0 before exit"

    # Trigger exit → wait → claim
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 2)

    rsp = claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Bonus = 0 after exit
    est = query_estimate_rewards(cluster, pos_id)
    bonus = sum(int(c.get("amount", "0")) for c in est.get("bonus_rewards", []))
    assert bonus == 0, "bonus should be 0 after exit unlock"


def test_clear_exit_settles_rewards(cluster):
    """ClearPosition settles pending rewards and resets exit.

    lock → claim → trigger exit → wait for exit →
    clear position (rewards paid) →
    back to bonded.
    """
    owner = cluster.address("signer1")
    validator = get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 1000

    rsp = fund_pool(cluster, "signer1", f"1000000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Let bonus accrue
    wait_for_new_blocks(cluster, 10)

    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 1)

    # Verify pending bonus before clear
    est = query_estimate_rewards(cluster, pos_id)
    bonus = sum(int(c.get("amount", "0")) for c in est.get("bonus_rewards", []))
    assert bonus > 0, "bonus should be pending before clear"

    # Clear settles rewards
    balance_before = cluster.balance(owner, DENOM)
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    balance_after = cluster.balance(owner, DENOM)
    assert balance_after > balance_before, "clear should settle rewards"

    # Position back to bonded with no exit
    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["delegated_shares"] != "0.000000000000000000"


def test_rewards_settled_at_each_lifecycle_stage(cluster):
    """Rewards are settled during add, redelegate, clear, and undelegate.

    lock → wait →
    add to position (rewards paid) → wait →
    redelegate (rewards paid) → wait →
    trigger exit → wait for exit →
    clear position (rewards paid) → wait →
    trigger exit → wait for exit →
    undelegate (rewards paid).

    Verifies owner balance increases at each settlement point.
    """
    owner = cluster.address("signer2")
    v0 = get_validator_addr(cluster, 0)
    v1 = get_validator_addr(cluster, 1)
    amount = TIER_1_MIN * 1000

    rsp = fund_pool(cluster, "signer1", f"2000000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_1_ID, amount, validator=v0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Let rewards accrue
    wait_for_new_blocks(cluster, 10)

    # 1. AddToTierPosition should succeed.
    bal = cluster.balance(owner, DENOM)
    rsp = add_to_position(cluster, owner, pos_id, TIER_1_MIN)
    assert rsp["code"] == 0, rsp["raw_log"]
    bal_after = cluster.balance(owner, DENOM)
    assert bal_after > bal - TIER_1_MIN, (
        f"add to position should claim rewards: " f"before={bal}, after={bal_after}"
    )

    wait_for_new_blocks(cluster, 10)

    # 2. Redelegate settles rewards
    bal = cluster.balance(owner, DENOM)
    rsp = tier_redelegate(cluster, owner, pos_id, v1)
    assert rsp["code"] == 0, rsp["raw_log"]
    bal_after = cluster.balance(owner, DENOM)
    assert bal_after > bal, (
        f"redelegate should settle rewards: " f"before={bal}, after={bal_after}"
    )

    wait_for_new_blocks(cluster, 10)

    # 3. ClearPosition settles rewards
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 1)

    bal = cluster.balance(owner, DENOM)
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    bal_after = cluster.balance(owner, DENOM)
    assert bal_after > bal, (
        f"clear position should settle rewards: " f"before={bal}, after={bal_after}"
    )

    wait_for_new_blocks(cluster, 10)

    # 4. Undelegate settles rewards
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 1)

    bal = cluster.balance(owner, DENOM)
    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    bal_after = cluster.balance(owner, DENOM)
    assert bal_after > bal, (
        f"undelegate should settle rewards: " f"before={bal}, after={bal_after}"
    )


# ──────────────────────────────────────────────
# Slash flows
# ──────────────────────────────────────────────


@pytest.mark.slow
@pytest.mark.slow_b1
def test_slash_then_withdraw(slashing_cluster):
    """lock → slash (1%) → trigger exit →
    wait for exit → undelegate →
    wait for unbonding → withdraw.

    Partial slash reduces amount but position exits cleanly.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator = get_node_validator_addr(cluster, 2)
    amount = TIER_2_MIN * 4

    rsp = fund_pool(cluster, "signer1", f"100000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_2_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    amount_before = int(query_position(cluster, pos_id)["position"]["amount"])

    # Slash validator 2 (1% downtime)
    wait_for_new_blocks(cluster, 5)
    cluster.supervisor.stopProcess(f"{cluster.chain_id}-node2")
    wait_for_new_blocks(cluster, 20)
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node2")
    wait_for_port(rpc_port(cluster.base_port(2)))
    wait_for_new_blocks(cluster, 2)

    amount_after = int(query_position(cluster, pos_id)["position"]["amount"])
    assert amount_after < amount_before, "slash should reduce amount"
    assert amount_after > 0, "partial slash should not zero amount"

    # Full exit
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 1)

    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    unbond_data = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    completion = isoparse(unbond_data["completion_time"].strip('"'))
    wait_for_block_time(cluster, completion)
    wait_for_new_blocks(cluster, 1)

    balance_before = cluster.balance(owner, DENOM)
    rsp = withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    balance_after = cluster.balance(owner, DENOM)

    assert balance_after > balance_before, "should receive slashed amount"

    try:
        query_position(cluster, pos_id)
        assert False, "position should be deleted"
    except requests.HTTPError:
        pass


@pytest.mark.slow
@pytest.mark.slow_b1
@pytest.mark.flaky(max_runs=3)
def test_redeleg_slash_then_withdraw(slashing_cluster):
    """lock → redeleg slash (50%) → position still delegated with reduced amount →
    trigger exit → wait for exit →
    undelegate → wait for unbonding → withdraw.

    Partial (non-100%) redelegation slash reduces amount but leaves it > 0.
    Position stays delegated (shares not fully burnt). Exit normally.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator2 = get_node_validator_addr(cluster, 2)
    validator0 = get_node_validator_addr(cluster, 0)
    wait_for_new_blocks(cluster, 2)

    pos_id = _setup_redeleg_slash(
        cluster,
        owner,
        validator2,
        validator0,
        slash_fraction="0.500000000000000000",
    )

    pos = query_position(cluster, pos_id)["position"]
    slashed_amount = int(pos["amount"])
    assert slashed_amount > 0, "partial slash should leave amount > 0"
    assert (
        pos["validator"] == validator0
    ), "partial redeleg slash keeps delegation (shares not fully burnt)"
    assert pos["delegated_shares"] != "0.000000000000000000"

    # Full exit — position is still delegated
    returned = _exit_undelegate_withdraw(cluster, owner, pos_id)
    assert returned > 0, "should receive reduced but non-zero amount"


@pytest.mark.slow
@pytest.mark.slow_b1
@pytest.mark.flaky(max_runs=3)
# This is marked flaky as it is not guaranteed that the unbonding
# will land within the 2-block window
# (creation_height >= infractionHeight) to be eligible for a slash.
def test_unbonding_slash_then_withdraw(slashing_cluster):
    """lock → trigger exit → wait for exit →
    undelegate → partial slash (50%) during unbonding →
    wait for unbonding → withdraw (reduced tokens).

    Partial slash during unbonding reduces the unbonding entry
    but leaves some tokens. Position is deleted with reduced amount.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator2 = get_node_validator_addr(cluster, 2)
    wait_for_new_blocks(cluster, 2)

    # Gov proposal: 50% slash, signed_blocks_window=20
    slashing_params = query_command(cluster, "slashing", "params")["params"]
    slashing_params["slash_fraction_downtime"] = "0.500000000000000000"
    slashing_params["signed_blocks_window"] = "20"
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_SLASHING_UPDATE_PARAMS,
        {"params": slashing_params},
        title="50% downtime slash",
        summary="partial slash during unbonding for test",
    )
    approve_proposal(cluster, rsp, msg=f",{MSG_SLASHING_UPDATE_PARAMS}")

    lock_amount = TIER_2_MIN * 2

    # Lock with immediate exit on v2
    before = before_ids(cluster, owner)
    rsp = lock_tier(
        cluster,
        owner,
        TIER_2_ID,
        lock_amount,
        validator=validator2,
        trigger_exit=True,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Wait for exit to elapse (5s)
    pos = query_position(cluster, pos_id)["position"]
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 1)

    # Stop v2 to start infraction, then undelegate (starts unbonding)
    cluster.supervisor.stopProcess(f"{cluster.chain_id}-node2")
    wait_for_new_blocks(cluster, 9)

    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Wait for slash to fire during unbonding
    wait_for_new_blocks(cluster, 15)
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node2")
    wait_for_port(rpc_port(cluster.base_port(2)))
    wait_for_new_blocks(cluster, 2)

    pos = query_position(cluster, pos_id)["position"]
    slashed_amount = int(pos["amount"])
    assert 0 < slashed_amount < lock_amount, (
        f"partial slash during unbonding should reduce but not zero: "
        f"got {slashed_amount}, original {lock_amount}"
    )

    # Wait for unbonding to complete
    unbond_data = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    if unbond_data:
        completion = isoparse(unbond_data["completion_time"].strip('"'))
        wait_for_block_time(cluster, completion)
        wait_for_new_blocks(cluster, 1)

    # Withdraw — should receive reduced but non-zero tokens
    balance_before = cluster.balance(owner, DENOM)
    rsp = withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    balance_after = cluster.balance(owner, DENOM)
    assert balance_after > balance_before, "should receive partial amount"

    try:
        query_position(cluster, pos_id)
        assert False, "position should be deleted"
    except requests.HTTPError:
        pass


@pytest.mark.slow
@pytest.mark.slow_b1
def test_clear_position_after_slash(slashing_cluster):
    """lock → trigger exit → slash (1%) during exit →
    clear position → back to bonded with reduced amount.

    Slash happens while exit is in progress. ClearPosition cancels
    the exit. Position returns to bonded with reduced amount.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator = get_node_validator_addr(cluster, 2)

    before = before_ids(cluster, owner)
    # lock in tier 2 (exit duration = 60s) so that position will still be exiting
    # when clearing position later
    rsp = lock_tier(cluster, owner, TIER_2_ID, TIER_2_MIN * 4, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    amount_before = int(query_position(cluster, pos_id)["position"]["amount"])

    # Trigger exit first
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] != ZERO_TIME

    # Slash validator 2 (1% downtime) during exit
    wait_for_new_blocks(cluster, 5)
    cluster.supervisor.stopProcess(f"{cluster.chain_id}-node2")
    wait_for_new_blocks(cluster, 20)
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node2")
    wait_for_port(rpc_port(cluster.base_port(2)))
    wait_for_new_blocks(cluster, 2)

    amount_after = int(query_position(cluster, pos_id)["position"]["amount"])
    assert amount_after < amount_before, "slash should reduce amount"
    assert amount_after > 0, "partial slash should not zero amount"

    # Clear position — cancel exit after slash
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["exit_unlock_at"] == ZERO_TIME
    assert int(pos["amount"]) == amount_after
    assert pos["delegated_shares"] != "0.000000000000000000"


@pytest.mark.slow
@pytest.mark.slow_b1
@pytest.mark.flaky(max_runs=3)
def test_clear_position_after_redeleg_slash(slashing_cluster):
    """lock → trigger exit → redeleg slash (50%) during exit →
    clear position → back to bonded with reduced amount.

    Redeleg slash happens while exit is in progress. Position stays
    delegated (shares not fully burnt). ClearPosition cancels the exit.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator2 = get_node_validator_addr(cluster, 2)
    validator0 = get_node_validator_addr(cluster, 0)
    wait_for_new_blocks(cluster, 2)

    pos_id = _setup_redeleg_slash(
        cluster,
        owner,
        validator2,
        validator0,
        slash_fraction="0.500000000000000000",
        trigger_exit_before_redelegate=True,
    )

    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["amount"]) > 0, "partial slash should leave amount > 0"
    assert pos["exit_triggered_at"] != ZERO_TIME
    amount_before_clear = int(pos["amount"])

    # Clear position — cancel exit after redeleg slash
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["exit_unlock_at"] == ZERO_TIME
    assert int(pos["amount"]) == amount_before_clear
    assert pos["delegated_shares"] != "0.000000000000000000"


# ──────────────────────────────────────────────
# 100% Slash flows
# ──────────────────────────────────────────────


@pytest.mark.slow
@pytest.mark.slow_b1
@pytest.mark.flaky(max_runs=3)
def test_slash_all_then_withdraw(slashing_cluster):
    """lock → slash 100% (direct) → trigger exit →
    wait for exit → undelegate →
    wait for unbonding → withdraw (0 tokens).

    Direct 100% slash zeros the amount. The position still goes through
    the full exit lifecycle and is deleted with 0 tokens returned.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator = get_node_validator_addr(cluster, 2)

    wait_for_new_blocks(cluster, 2)

    # Gov proposal: 100% downtime slash
    slashing_params = query_command(cluster, "slashing", "params")["params"]
    slashing_params["slash_fraction_downtime"] = "1.000000000000000000"
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_SLASHING_UPDATE_PARAMS,
        {"params": slashing_params},
        title="100% downtime slash",
        summary="slash to zero for test",
    )
    approve_proposal(cluster, rsp, msg=f",{MSG_SLASHING_UPDATE_PARAMS}")

    # Lock on validator 2
    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_2_ID, TIER_2_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Slash validator 2 to zero
    wait_for_new_blocks(cluster, 5)
    cluster.supervisor.stopProcess(f"{cluster.chain_id}-node2")
    wait_for_new_blocks(cluster, 20)
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node2")
    wait_for_port(rpc_port(cluster.base_port(2)))
    wait_for_new_blocks(cluster, 2)

    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["amount"]) == 0, "100% slash should zero amount"
    assert pos["validator"] == validator, "delegation should not be cleared"

    # Full exit lifecycle on worthless position
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 1)

    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    unbond_data = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    completion = isoparse(unbond_data["completion_time"].strip('"'))
    wait_for_block_time(cluster, completion)
    wait_for_new_blocks(cluster, 1)

    rsp = withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Position deleted
    try:
        query_position(cluster, pos_id)
        assert False, "position should be deleted after withdraw"
    except requests.HTTPError:
        pass


@pytest.mark.slow
@pytest.mark.slow_b1
@pytest.mark.flaky(max_runs=3)
def test_redeleg_slash_all_then_withdraw(slashing_cluster):
    """redeleg-slashed to zero → trigger exit →
    wait for exit →
    withdraw (delete with 0 tokens).

    After redeleg slash zeros the position, trigger exit, wait,
    and withdraw the empty position.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator2 = get_node_validator_addr(cluster, 2)
    validator0 = get_node_validator_addr(cluster, 0)
    wait_for_new_blocks(cluster, 2)

    pos_id = _setup_redeleg_slash(cluster, owner, validator2, validator0)

    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["amount"]) == 0, "redeleg slash should zero amount"

    # redeleg-slashed to zero → trigger exit
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] != ZERO_TIME

    # exit triggered → wait for exit to elapse
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 1)

    # exit elapsed → withdraw (delete with 0 tokens)
    rsp = withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    try:
        query_position(cluster, pos_id)
        assert False, "position should be deleted"
    except requests.HTTPError:
        pass


@pytest.mark.slow
@pytest.mark.slow_b2
@pytest.mark.flaky(max_runs=3)
# This is marked flaky as it is not guaranteed that the unbonding
# will land within the 2-block window
# (creation_height >= infractionHeight) to be eligible for a slash.
def test_slash_all_during_unbonding_then_withdraw(slashing_cluster):
    """lock → trigger exit → wait for exit →
    undelegate → slash during unbonding →
    wait for unbonding → withdraw (0 tokens).

    100% slash during unbonding zeros the unbonding entry.
    Position is deleted with 0 tokens returned.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator2 = get_node_validator_addr(cluster, 2)
    wait_for_new_blocks(cluster, 2)

    # Gov proposal: 100% slash, signed_blocks_window=20
    slashing_params = query_command(cluster, "slashing", "params")["params"]
    slashing_params["slash_fraction_downtime"] = "1.000000000000000000"
    slashing_params["signed_blocks_window"] = "20"
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_SLASHING_UPDATE_PARAMS,
        {"params": slashing_params},
        title="100% downtime slash",
        summary="slash during unbonding for test",
    )
    approve_proposal(cluster, rsp, msg=f",{MSG_SLASHING_UPDATE_PARAMS}")

    # Lock with immediate exit on v2
    before = before_ids(cluster, owner)
    rsp = lock_tier(
        cluster,
        owner,
        TIER_2_ID,
        TIER_2_MIN * 2,
        validator=validator2,
        trigger_exit=True,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Wait for exit to elapse (5s)
    pos = query_position(cluster, pos_id)["position"]
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 1)

    # Stop v2 to start infraction, then undelegate (starts unbonding)
    # Undelegate must land close to slash trigger (creation_height >= infractionHeight)
    cluster.supervisor.stopProcess(f"{cluster.chain_id}-node2")
    wait_for_new_blocks(cluster, 9)

    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Wait for slash to fire during unbonding
    wait_for_new_blocks(cluster, 15)
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node2")
    wait_for_port(rpc_port(cluster.base_port(2)))
    wait_for_new_blocks(cluster, 2)

    pos = query_position(cluster, pos_id)["position"]
    assert (
        int(pos["amount"]) == 0
    ), f"slash during unbonding should zero amount, got {pos['amount']}"

    # Wait for unbonding to complete
    unbond_data = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    if unbond_data:
        completion = isoparse(unbond_data["completion_time"].strip('"'))
        wait_for_block_time(cluster, completion)
        wait_for_new_blocks(cluster, 1)

    # slashed during unbonding, unbonding completed → withdraw (delete with 0 tokens)
    rsp = withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    try:
        query_position(cluster, pos_id)
        assert False, "position should be deleted"
    except requests.HTTPError:
        pass


@pytest.mark.slow
@pytest.mark.slow_b2
@pytest.mark.flaky(max_runs=3)
def test_clear_position_on_redeleg_slashed_all_exiting(slashing_cluster):
    """lock → trigger exit → redeleg slash (100%) during exit →
    clear position → undelegated with no exit.

    100% redeleg slash during exit zeros the position and clears
    delegation. ClearPosition resets the exit.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator2 = get_node_validator_addr(cluster, 2)
    validator0 = get_node_validator_addr(cluster, 0)
    wait_for_new_blocks(cluster, 2)

    pos_id = _setup_redeleg_slash(
        cluster,
        owner,
        validator2,
        validator0,
        trigger_exit_before_redelegate=True,
    )

    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["amount"]) == 0, "100% redeleg slash should zero amount"
    assert pos["exit_triggered_at"] != ZERO_TIME

    # Clear position — reset exit, stay undelegated
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["exit_unlock_at"] == ZERO_TIME
    assert int(pos["amount"]) == 0
    assert pos["validator"] == "", "should remain undelegated"


@pytest.mark.slow
@pytest.mark.slow_b2
def test_clear_position_slashed_all_exiting(slashing_cluster):
    """lock → trigger exit → slash 100% (direct) during exit →
    clear position → delegated with no exit.

    Direct 100% slash during exit zeros the amount but keeps
    delegation. ClearPosition resets the exit.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator2 = get_node_validator_addr(cluster, 2)
    wait_for_new_blocks(cluster, 2)

    # Gov proposal: 100% downtime slash
    slashing_params = query_command(cluster, "slashing", "params")["params"]
    slashing_params["slash_fraction_downtime"] = "1.000000000000000000"
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_SLASHING_UPDATE_PARAMS,
        {"params": slashing_params},
        title="100% downtime slash",
        summary="direct slash during exit for clear test",
    )
    approve_proposal(cluster, rsp, msg=f",{MSG_SLASHING_UPDATE_PARAMS}")

    # Lock on validator 2
    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_2_ID, TIER_2_MIN * 2, validator=validator2)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Trigger exit first
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] != ZERO_TIME

    # Slash validator 2 to zero during exit
    wait_for_new_blocks(cluster, 5)
    cluster.supervisor.stopProcess(f"{cluster.chain_id}-node2")
    wait_for_new_blocks(cluster, 20)
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node2")
    wait_for_port(rpc_port(cluster.base_port(2)))
    wait_for_new_blocks(cluster, 2)

    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["amount"]) == 0, "100% slash should zero amount"
    assert pos["validator"] != "", "direct slash keeps validator set"
    assert pos["exit_triggered_at"] != ZERO_TIME

    # Clear position — reset exit, stay delegated
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["exit_unlock_at"] == ZERO_TIME
    assert int(pos["amount"]) == 0
    assert pos["validator"] != "", "should still be delegated"


@pytest.mark.slow
@pytest.mark.slow_b2
@pytest.mark.flaky(max_runs=3)
def test_clear_position_redeleg_slash_all_undelegated_exiting(
    slashing_cluster,
):
    """redeleg slash (100%) → add tokens (undelegated, amount > 0) →
    trigger exit → clear position → undelegated with no exit.

    ClearPosition on an undelegated position with amount > 0 and
    exit in progress resets the exit fields.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator2 = get_node_validator_addr(cluster, 2)
    validator0 = get_node_validator_addr(cluster, 0)
    wait_for_new_blocks(cluster, 2)

    pos_id = _setup_redeleg_slash(cluster, owner, validator2, validator0)

    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["amount"]) == 0

    # Add tokens to bring amount > 0 (still undelegated)
    rsp = add_to_position(cluster, owner, pos_id, TIER_1_MIN * 2)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["amount"]) == TIER_1_MIN * 2
    assert pos["validator"] == "", "should still be undelegated"

    # Trigger exit on undelegated position with amount > 0
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] != ZERO_TIME

    # Clear position — cancel exit, stay undelegated with amount > 0
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["exit_unlock_at"] == ZERO_TIME
    assert int(pos["amount"]) == TIER_1_MIN * 2
    assert pos["validator"] == "", "should remain undelegated"


@pytest.mark.slow
@pytest.mark.slow_b2
def test_clear_position_slashed_all_exit_elapsed(slashing_cluster):
    """lock → slash 100% (direct) → trigger exit → wait for exit elapsed →
    clear position → delegated with no exit.

    ClearPosition on a worthless delegated position after exit has
    elapsed resets the exit fields.
    """
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator = get_node_validator_addr(cluster, 2)
    wait_for_new_blocks(cluster, 2)

    # Gov proposal: 100% downtime slash
    slashing_params = query_command(cluster, "slashing", "params")["params"]
    slashing_params["slash_fraction_downtime"] = "1.000000000000000000"
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_SLASHING_UPDATE_PARAMS,
        {"params": slashing_params},
        title="100% downtime slash",
        summary="direct slash for clear elapsed test",
    )
    approve_proposal(cluster, rsp, msg=f",{MSG_SLASHING_UPDATE_PARAMS}")

    # Lock on validator 2
    before = before_ids(cluster, owner)
    rsp = lock_tier(cluster, owner, TIER_2_ID, TIER_2_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = new_pos_id(cluster, owner, before)

    # Slash validator 2 to zero
    wait_for_new_blocks(cluster, 5)
    cluster.supervisor.stopProcess(f"{cluster.chain_id}-node2")
    wait_for_new_blocks(cluster, 20)
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node2")
    wait_for_port(rpc_port(cluster.base_port(2)))
    wait_for_new_blocks(cluster, 2)

    pos = query_position(cluster, pos_id)["position"]
    assert int(pos["amount"]) == 0

    # Trigger exit
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Wait for exit to elapse
    pos = query_position(cluster, pos_id)["position"]
    wait_for_block_time(cluster, isoparse(pos["exit_unlock_at"]))
    wait_for_new_blocks(cluster, 1)

    # Exit elapsed on worthless delegated position → clear
    rsp = clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = query_position(cluster, pos_id)["position"]
    assert pos["exit_triggered_at"] == ZERO_TIME
    assert pos["exit_unlock_at"] == ZERO_TIME
    assert int(pos["amount"]) == 0
    assert pos["validator"] != "", "should still be delegated"


# ──────────────────────────────────────────────
# CLI / query parity
# ──────────────────────────────────────────────


def test_autocli_lock_tier_and_queries(cluster):
    """Smoke test tieredrewards autocli tx/query paths."""
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
    assert owner_positions_rsp["positions"] == rest_owner_positions_rsp["positions"]

    tiers_rsp = query_command(cluster, MODULE, "tiers")
    rest_tiers_rsp = query_tiers(cluster)
    cli_tiers = {
        **tiers_rsp,
        "tiers": [
            {**tier, "close_only": tier.get("close_only", False)}
            for tier in tiers_rsp.get("tiers", [])
        ],
    }
    rest_tiers = {
        **rest_tiers_rsp,
        "tiers": [
            {**tier, "close_only": tier.get("close_only", False)}
            for tier in rest_tiers_rsp.get("tiers", [])
        ],
    }
    assert cli_tiers == rest_tiers

    tier_ids = {int(t["id"]) for t in tiers_rsp.get("tiers", [])}
    assert TIER_1_ID in tier_ids
    assert TIER_2_ID in tier_ids
