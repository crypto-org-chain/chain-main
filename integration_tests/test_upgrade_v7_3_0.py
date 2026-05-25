import json

from .tieredrewards_helpers import (
    DENOM,
    commit_delegation,
    fund_pool,
    get_node_validator_addr,
    lock_tier,
    query_positions_by_owner,
    query_tiers,
)
from .utils import (
    create_permanent_lock_vesting_account,
    unwrap_account,
    wait_for_new_blocks,
)

V7_3_PLAN = "v7.3.0"


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


def setup_pre_v7_3_0_upgrade(cluster):
    """Set up the vesting tier-bypass scenario before the v7.3.0 upgrade
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

    # Fund account with gas
    gas_topup = 50_000_000
    rsp = cluster.transfer(
        cluster.address("signer1"),
        owner_addr,
        f"{gas_topup}basecro",
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


def assert_v7_3_vesting_migration(cluster, ctx):
    """Verify the v7.3.0 migration:
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
    cli = cluster.cosmos_cli()
    deleg_raw = cli.raw(
        "query",
        "staking",
        "delegation",
        owner_addr,
        val_addr,
        output="json",
        node=cli.node_rpc,
    )
    deleg = json.loads(deleg_raw)
    deleg_amount = int(deleg["delegation_response"]["balance"]["amount"])
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

    print("v7.3.0 vesting tier-bypass migration verified")
