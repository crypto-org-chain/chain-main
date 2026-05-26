from .tieredrewards_helpers import (
    DENOM,
    commit_delegation,
    fund_pool,
    get_node_validator_addr,
    lock_tier,
    query_positions,
    query_positions_by_owner,
    query_tiers,
)
from .utils import (
    create_permanent_lock_vesting_account,
    query_staking_delegation,
    unwrap_account,
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


def setup_pre_v8_upgrade(cluster):
    """Set up the vesting tier-bypass scenario before the v8 upgrade
    fires. Creates a PermanentLockedAccount and gives it two positions:

      1. A CommitDelegationToTier-origin position. The owner first
         delegates the locked principal (which populates DelegatedVesting),
         then commits the delegation to a tier — DelegatedVesting is left
         stale-high while the position holds the actual delegation.
      2. A LockTier-origin position, funded from the gas top-up balance
         via bank send. DelegatedVesting/Free are not touched at lock time.

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
    gas_topup = 50_000_000
    topup = lock_amount + gas_topup
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


def assert_v8_vesting_migration(cluster, ctx):
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


def assert_no_vesting_positions(cluster):
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
    gas_topup = 50_000_000

    # ── 1a. MsgCommitDelegationToTier from a vesting account is rejected ──
    vesting_addr = create_permanent_lock_vesting_account(
        cluster,
        f"{amount}basecro",
        name="permanent-locked-account-post-upgrade",
    )
    rsp = cluster.transfer(
        cluster.address("signer1"),
        vesting_addr,
        f"{amount + gas_topup}basecro",
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
