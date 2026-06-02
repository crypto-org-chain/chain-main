from datetime import timedelta

import requests
from dateutil.parser import isoparse

from .tieredrewards_helpers import (
    DENOM,
    GAS_ALLOWANCE,
    MSG_UPDATE_TIER,
    commit_delegation,
    fund_pool,
    get_node_validator_addr,
    lock_tier,
    position_delegator_address,
    query_position,
    query_positions,
    query_positions_by_owner,
    query_tiers,
    tier_undelegate,
    trigger_exit,
    withdraw,
)
from .utils import (
    approve_proposal,
    create_new_address,
    create_permanent_lock_vesting_account,
    find_log_event_attrs,
    query_staking_delegation,
    submit_gov_proposal,
    unwrap_account,
    wait_for_block_time,
    wait_for_new_blocks,
)

V8_PLAN = "v8"
VESTING_TYPE_MARKERS = ("Vesting", "PermanentLocked")


def _vesting_delegated_amounts(account_dict, denom=DENOM):
    """Extract DelegatedVesting / DelegatedFree amounts (in `denom`) from
    a (possibly amino-wrapped) vesting account JSON. Returns a (DV, DF)
    tuple of ints, treating absent denoms as 0.
    """
    bva = account_dict.get("base_vesting_account") or {}
    if not bva and "value" in account_dict:
        bva = account_dict["value"].get("base_vesting_account") or {}

    def _amount(coins):
        for c in coins or []:
            if c.get("denom") == denom:
                return int(c.get("amount", "0"))
        return 0

    return _amount(bva.get("delegated_vesting")), _amount(bva.get("delegated_free"))


def _create_vesting_acc_owned_positions(cluster):
    """
    Creates a PermanentLockedAccount and establishes two distinct positions:
    1. A position initialized via CommitDelegationToTier.
    2. A position initialized via LockTier.

    Funds the rewards pool so the migration's claimRewards step can pay
    out any bonus accrued on the bypass positions.
    """
    val_addr = get_node_validator_addr(cluster)

    tiers = query_tiers(cluster).get("tiers", [])
    assert tiers, "expected at least one tier seeded by the v7 upgrade handler"
    tier_id = int(tiers[0]["id"])
    amount = int(tiers[0]["min_lock_amount"])
    commit_amount = amount
    lock_amount = amount

    # Create a permanent locked account with the commit amount
    owner_addr = create_permanent_lock_vesting_account(
        cluster,
        f"{commit_amount}basecro",
    )

    # Fund account for lock tier
    topup = lock_amount + GAS_ALLOWANCE
    rsp = cluster.transfer(
        cluster.address("signer1"),
        owner_addr,
        f"{topup}basecro",
    )
    assert rsp["code"] == 0, f"gas top-up failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cluster, 1)

    # Vesting owner delegates locked principal — this populates
    # DelegatedVesting via the bank-side TrackDelegation hook
    rsp = cluster.delegate_amount(val_addr, f"{commit_amount}basecro", owner_addr)
    assert rsp["code"] == 0, rsp.get("raw_log", rsp)
    wait_for_new_blocks(cluster, 1)

    # commit vesting account's delegation to a tier position
    rsp = commit_delegation(
        cluster,
        owner_addr,
        val_addr,
        commit_amount,
        tier_id,
    )
    assert (
        rsp["code"] == 0
    ), f"commit-delegation-to-tier failed on v7.2.0: {rsp.get('raw_log', rsp)}"

    # LockTier from the same vesting owner
    rsp = lock_tier(
        cluster,
        owner_addr,
        tier_id,
        lock_amount,
        val_addr,
    )
    assert rsp["code"] == 0, f"lock-tier failed on v7.2.0: {rsp.get('raw_log', rsp)}"

    positions = query_positions_by_owner(cluster, owner_addr).get("positions", [])
    assert len(positions) == 2, f"expected 2 positions pre-upgrade, got {positions}"
    pos_ids = sorted(int(p["id"]) for p in positions)
    commit_pos_id, lock_pos_id = pos_ids[0], pos_ids[1]

    # Fund the rewards pool
    rsp = fund_pool(cluster, "signer1", f"50000000{DENOM}")
    assert rsp["code"] == 0, f"fund_pool failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cluster, 1)

    return {
        "owner_addr": owner_addr,
        "val_addr": val_addr,
        "commit_amount": commit_amount,
        "lock_amount": lock_amount,
        "commit_pos_id": commit_pos_id,
        "lock_pos_id": lock_pos_id,
    }


def _next_position_id(cluster):
    """Predict the next position id by querying all positions and using
    max(id) + 1. Safe in a sequential test where no other LockTier runs
    between the prediction and the LockTier we're about to send.
    """
    entries = query_positions(cluster).get("positions", [])
    ids = [int((entry.get("position") or entry)["id"]) for entry in entries]
    return (max(ids) if ids else 0) + 1


def _precreate_position_delegator_vesting_acc(cluster):
    """Set up the position-delegator-precreate scenario before the v8
    upgrade fires.

    On v7.2.0, the position delegator address is deterministic from the
    position id — `tieredrewards/position/<id>` module address. An
    attacker can pre-create a `PermanentLockedAccount` at that address
    """
    val_addr = get_node_validator_addr(cluster)
    tiers = query_tiers(cluster).get("tiers", [])
    assert tiers, "expected at least one tier seeded by the v7 upgrade handler"
    tier_id = int(tiers[0]["id"])
    lock_amount = int(tiers[0]["min_lock_amount"])

    # Predict next position id; pre-create a PermanentLockedAccount at
    # the v1 delegator address that LockTier will pick.
    next_pos_id = _next_position_id(cluster)
    predicted_del_addr = position_delegator_address(next_pos_id)
    dust = 1
    cli = cluster.cosmos_cli()
    rsp = cli.tx(
        "vesting",
        "create-permanent-locked-account",
        predicted_del_addr,
        f"{dust}{DENOM}",
        from_=cluster.address("signer1"),
        chain_id=cli.chain_id,
    )
    assert (
        rsp["code"] == 0
    ), f"front-run vesting-account create failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cluster, 1)

    # Create a fresh base-account owner and fund it with principal + gas.
    name = "precreate-victim"
    owner_addr = create_new_address(cluster, name)

    rsp = cluster.transfer(
        cluster.address("signer1"),
        owner_addr,
        f"{lock_amount + GAS_ALLOWANCE}{DENOM}",
    )
    assert rsp["code"] == 0, f"owner top-up failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cluster, 1)

    # LockTier — picks up `predicted_del_addr` because the v7.2.0
    # derivation is deterministic from the position id alone.
    rsp = lock_tier(cluster, owner_addr, tier_id, lock_amount, val_addr)
    assert (
        rsp["code"] == 0
    ), f"lock-tier on pre-poisoned address failed: {rsp.get('raw_log', rsp)}"

    positions = query_positions_by_owner(cluster, owner_addr).get("positions", [])
    assert len(positions) == 1, f"expected 1 precreate-victim position, got {positions}"
    actual_pos_id = int(positions[0]["id"])
    assert actual_pos_id == next_pos_id, (
        f"expected position id {next_pos_id} (the front-run target), "
        f"got {actual_pos_id} — front-run prediction stale"
    )

    return {
        "precreate_owner_addr": owner_addr,
        "precreate_val_addr": val_addr,
        "precreate_pos_id": next_pos_id,
        "precreate_predicted_del_addr": predicted_del_addr,
        "precreate_lock_amount": lock_amount,
        "precreate_dust": dust,
    }


def setup_pre_v8_upgrade(cluster):
    ctx = _create_vesting_acc_owned_positions(cluster)
    ctx.update(_precreate_position_delegator_vesting_acc(cluster))
    return ctx


def assert_v8_vesting_acc_owned_positions_exited(cluster, ctx):
    """Verify the v8 migration:
    - both vesting-owned positions are deleted;
    - the vesting owner's staking delegation equals commit + lock amounts;
    - DelegatedVesting saturates at OriginalVesting (=commit_amount), and
      the LockTier amount overflows into DelegatedFree, so DV+DF == Σ
      delegations and the vesting bookkeeping invariant holds.
    """
    owner_addr = ctx["owner_addr"]
    val_addr = ctx["val_addr"]
    commit_amount = ctx["commit_amount"]
    lock_amount = ctx["lock_amount"]
    total_amount = commit_amount + lock_amount

    # Both positions deleted.
    positions_after = query_positions_by_owner(cluster, owner_addr).get("positions", [])
    assert (
        positions_after == []
    ), f"expected zero positions post-upgrade, got {positions_after}"

    # Vesting metadata still intact.
    post_acct = unwrap_account(cluster.cosmos_cli().account(owner_addr))
    assert post_acct["@type"] in (
        "cosmos-sdk/PermanentLockedAccount",
        "/cosmos.vesting.v1beta1.PermanentLockedAccount",
    ), f"vesting metadata must survive, got {post_acct}"

    # Owner has staking delegation restored, equal to commit + lock.
    deleg = query_staking_delegation(cluster, owner_addr, val_addr)
    deleg_amount = int(deleg["balance"]["amount"])
    assert deleg_amount == total_amount, (
        f"owner delegation should be {total_amount} "
        f"(commit={commit_amount} + lock={lock_amount}), got {deleg_amount}"
    )

    # DV saturates at OriginalVesting (=commit_amount); LockTier amount
    # overflows into DF. DV+DF must equal Σ delegations.
    dv, df = _vesting_delegated_amounts(post_acct)
    assert dv == commit_amount, (
        f"DelegatedVesting should saturate at OriginalVesting={commit_amount}; "
        f"got DV={dv}"
    )
    assert df == lock_amount, (
        f"DelegatedFree should equal the LockTier amount={lock_amount}; " f"got DF={df}"
    )
    assert dv + df == total_amount, (
        f"DV+DF must equal Σ delegations: "
        f"DV({dv}) + DF({df}) = {dv + df}, expected {total_amount}"
    )
    print("v8 vesting tier-bypass migration verified")


def assert_v8_precreated_position_delegator_vesting_acc_lifecycle(cluster, ctx):
    """Verify post-v8 that the position whose delegator address was
    front-run with a PermanentLockedAccount on v7.2.0:

    1. Exposes the new `delegator_address` field on the gRPC/REST
       PositionResponse, and it matches the front-run v1 module address.
    2. Completes its full withdraw lifecycle (trigger_exit →
       tier_undelegate → wait unbonding → withdraw) successfully — the
       v8 SpendableCoins-based cleanup ignores the locked vesting
       balance instead of refusing the send.
    3. Preserves the locked vesting dust at the delegator address after
       withdraw — it was never spendable, so the sweep skipped it.
    """
    owner = ctx["precreate_owner_addr"]
    pos_id = ctx["precreate_pos_id"]
    predicted_del_addr = ctx["precreate_predicted_del_addr"]
    lock_amount = ctx["precreate_lock_amount"]
    dust = ctx["precreate_dust"]

    # 1. delegator_address is exposed and matches the v1 derivation.
    pos_resp = query_position(cluster, pos_id)
    pos = pos_resp.get("position") or pos_resp
    assert (
        "delegator_address" in pos
    ), f"PositionResponse missing delegator_address: {pos}"
    assert pos["delegator_address"] == predicted_del_addr, (
        f"delegator_address {pos['delegator_address']!r} does not match "
        f"front-run v1 address {predicted_del_addr!r}"
    )

    # The pre-poisoned vesting account is still there with its dust.
    bal_before = cluster.balance(predicted_del_addr, denom=DENOM)
    assert (
        bal_before == dust
    ), f"expected {dust}{DENOM} at {predicted_del_addr}, got {bal_before}"

    # Shorten exit duration to 5s for faster test execution.
    tier_id = int(pos["tier_id"])
    tiers = query_tiers(cluster).get("tiers", [])
    tier = next((t for t in tiers if int(t["id"]) == tier_id), None)
    assert tier is not None, f"tier {tier_id} not found"
    rsp = submit_gov_proposal(
        cluster,
        "community",
        MSG_UPDATE_TIER,
        {
            "tier": {
                "id": tier_id,
                "exit_duration": "5s",
                "bonus_apy": tier["bonus_apy"],
                "min_lock_amount": tier["min_lock_amount"],
                "close_only": tier.get("close_only", False),
            }
        },
        title=f"Shorten Tier {tier_id} exit_duration",
        summary="Shorten exit_duration to 5s for v8 upgrade test",
    )
    approve_proposal(cluster, rsp, msg=f",{MSG_UPDATE_TIER}")

    # 2. Drive the full withdraw lifecycle.
    rsp = trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, f"trigger_exit failed: {rsp.get('raw_log', rsp)}"

    pos_resp = query_position(cluster, pos_id)
    pos = pos_resp.get("position") or pos_resp
    exit_unlock_at = isoparse(pos["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    rsp = tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, f"tier_undelegate failed: {rsp.get('raw_log', rsp)}"

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

    balance_before = cluster.balance(owner, denom=DENOM)
    rsp = withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, (
        "withdraw on a position whose delegator address holds locked vesting "
        "dust must succeed under v8 (SpendableCoins-based sweep); "
        f"got: {rsp.get('raw_log', rsp)}"
    )
    balance_after = cluster.balance(owner, denom=DENOM)
    assert balance_after >= balance_before + lock_amount - GAS_ALLOWANCE, (
        f"owner balance should increase by ~lock_amount={lock_amount} after withdraw; "
        f"before={balance_before}, after={balance_after}"
    )

    # Position deleted.
    try:
        query_position(cluster, pos_id)
        assert False, f"position {pos_id} should be deleted after withdraw"
    except requests.HTTPError as exc:
        assert exc.response.status_code in (404, 500)
        assert "not found" in exc.response.text.lower()

    # 3. The locked vesting dust survives the sweep.
    bal_after = cluster.balance(predicted_del_addr, denom=DENOM)
    assert bal_after == dust, (
        f"locked vesting dust must remain at delegator address; "
        f"before={bal_before}, after={bal_after}"
    )
    print("v8 precreated-delegator lifecycle verified")


def assert_v8_no_vesting_owned_positions(cluster):
    cli = cluster.cosmos_cli()
    all_positions = query_positions(cluster).get("positions", [])
    for entry in all_positions:
        pos = entry.get("position") or entry
        owner = pos["owner"]
        acct = unwrap_account(cli.account(owner))
        atype = acct.get("@type", "")
        assert not any(
            marker in atype for marker in VESTING_TYPE_MARKERS
        ), f"vesting-owned position survived migration: owner={owner} type={atype}"

    print("v8 no vesting positions verified")


def assert_v8_vesting_filter_active(cluster):
    """Post-upgrade vesting-account filter smoke test:

    1. A fresh PermanentLockedAccount must NOT be able to create a new
       tier position via either MsgLockTier or MsgCommitDelegationToTier
       (rejected by validateNewPosition in the keeper).
    2. A regular base account must STILL be able to create a tier
       position (regression check that the filter only fires on vesting
       accounts).
    """
    val_addr = get_node_validator_addr(cluster)
    tiers = query_tiers(cluster).get("tiers", [])
    assert tiers, "expected at least one tier"
    tier_id = int(tiers[0]["id"])
    amount = int(tiers[0]["min_lock_amount"])

    # ── 1a. MsgCommitDelegationToTier from a vesting account is rejected ──
    vesting_addr = create_permanent_lock_vesting_account(
        cluster,
        f"{amount}basecro",
        name="permanent-locked-account-post-upgrade",
    )
    rsp = cluster.transfer(
        cluster.address("signer1"),
        vesting_addr,
        f"{amount + GAS_ALLOWANCE}basecro",
    )
    assert rsp["code"] == 0, f"top-up failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cluster, 1)

    # Self-delegate first so commit_delegation has a delegation to commit.
    rsp = cluster.delegate_amount(val_addr, f"{amount}basecro", vesting_addr)
    assert rsp["code"] == 0, rsp.get("raw_log", rsp)
    wait_for_new_blocks(cluster, 1)

    rsp = commit_delegation(cluster, vesting_addr, val_addr, amount, tier_id)
    assert (
        rsp["code"] != 0
    ), f"commit_delegation from vesting account must be rejected, got {rsp}"
    raw_log = (rsp.get("raw_log") or "").lower()
    assert (
        "vesting accounts are not allowed to execute this action" in raw_log
    ), f"expected vesting rejection, got raw_log={raw_log!r}"

    # ── 1b. MsgLockTier from a vesting account is rejected ──
    rsp = lock_tier(cluster, vesting_addr, tier_id, amount, val_addr)
    assert (
        rsp["code"] != 0
    ), f"lock_tier from vesting account must be rejected, got {rsp}"
    raw_log = (rsp.get("raw_log") or "").lower()
    assert (
        "vesting accounts are not allowed to execute this action" in raw_log
    ), f"expected vesting rejection, got raw_log={raw_log!r}"

    # ── 2. Regular base account can still lock-tier ──
    base_addr = cluster.address("signer2")
    before = {
        int(p["id"])
        for p in query_positions_by_owner(cluster, base_addr).get("positions", [])
    }
    rsp = lock_tier(cluster, base_addr, tier_id, amount, val_addr)
    assert (
        rsp["code"] == 0
    ), f"lock_tier from base account must succeed: {rsp.get('raw_log', rsp)}"
    after = {
        int(p["id"])
        for p in query_positions_by_owner(cluster, base_addr).get("positions", [])
    }
    new_ids = after - before
    assert len(new_ids) == 1, f"expected exactly one new position, got {new_ids}"

    print("v8 vesting filter verified: vesting blocked, base allowed")
