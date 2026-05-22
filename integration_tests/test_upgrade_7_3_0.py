import json
import time

import requests
from pystarport.cluster import SUPERVISOR_CONFIG_FILE

from .tieredrewards_helpers import (
    DENOM,
    commit_delegation,
    fund_pool,
    get_node_validator_addr,
    query_position,
    query_positions_by_owner,
    query_tiers,
)
from .utils import (
    approve_proposal,
    unwrap_account,
    wait_for_block,
    wait_for_new_blocks,
)

V7_3_PLAN = "v7.3.0"
V7_3_LOCK_AMOUNT = 1_000_000  # basecro — covers tier-1 min_lock seeded by v7
V7_3_GAS_TOPUP = 50_000_000   # basecro — gas budget for the vesting owner

# TODO: move this function to utils.py - create_permanent_lock_vesting_account() and use tx() helper to ensure that tx succeeds
def _create_permanent_lock_vesting_account(cluster, name, locked_amount, gas_topup):
    cli = cluster.cosmos_cli()
    cli.raw(
        "keys", "add", name,
        keyring_backend="test", home=cli.data_dir, output="json",
    )
    owner_addr = cli.raw(
        "keys", "show", name, "-a",
        keyring_backend="test", home=cli.data_dir,
    ).decode().strip()

    rsp = json.loads(cli.raw(
        "tx", "vesting", "create-permanent-locked-account",
        owner_addr,
        f"{locked_amount}basecro",
        "-y",
        from_=cluster.address("signer1"),
        keyring_backend="test",
        chain_id=cli.chain_id,
        home=cli.data_dir,
        node=cli.node_rpc,
        output="json",
    ))
    assert rsp["code"] == 0, (
        f"create-permanent-locked-account broadcast failed (CheckTx): "
        f"{rsp.get('raw_log', rsp)}"
    )

    rsp = cluster.transfer(
        cluster.address("signer1"),
        owner_addr,
        f"{gas_topup}basecro",
    )
    assert rsp["code"] == 0, f"gas top-up failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cluster, 1)

    return owner_addr


# TODO: move this to utils.py - upgrade method. old method refactor to upgrade_legacy()
def propose_n_execute_v7_3_upgrade(cluster):
    """Imported lazily to avoid a circular import (test_upgrade.py also
    imports this module to call the orchestration)."""
    from .test_upgrade import edit_chain_program

    target_height = cluster.block_height() + 30
    print("propose v7.3.0 upgrade plan at", target_height)

    rsp = cluster.gov_propose_since_cosmos_sdk_v0_50(
        "community",
        "software-upgrade",
        {
            "name": V7_3_PLAN,
            "title": "v7.3.0 upgrade",
            "summary": "v7.3.0 vesting tier-bypass patch + migration",
            "upgrade-height": target_height,
            "deposit": "0.1cro",
        },
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(cluster, rsp, msg=",/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade")

    wait_for_block(cluster, target_height)
    time.sleep(1)

    for i in range(2):
        assert (
            cluster.supervisor.getProcessInfo(f"{cluster.chain_id}-node{i}")["state"]
            != "RUNNING"
        ), f"node{i} should be stopped after upgrade height"

    js1 = json.load((cluster.home(0) / "data/upgrade-info.json").open())
    js2 = json.load((cluster.home(1) / "data/upgrade-info.json").open())
    expected = {"name": V7_3_PLAN, "height": target_height}
    assert js1 == js2
    assert expected.items() <= js1.items()

    edit_chain_program(
        cluster.chain_id,
        cluster.data_dir / SUPERVISOR_CONFIG_FILE,
        lambda i, _: {
            "command": (
                f"%(here)s/node{i}/cosmovisor/upgrades/{V7_3_PLAN}/bin/chain-maind "
                f"start --home %(here)s/node{i}"
            )
        },
    )
    cluster.reload_supervisor()
    cluster.cmd = cluster.data_root / f"cosmovisor/upgrades/{V7_3_PLAN}/bin/chain-maind"

    wait_for_block(cluster, target_height + 2)
    return target_height


def setup_pre_v7_3_0_upgrade(cluster):
    """Set up the vesting tier-bypass scenario before the v7.3.0 upgrade
    fires. Creates a PermanentLockedAccount, has it delegate the locked
    principal, and commits the delegation to a tier — the bypass leaves
    DelegatedVesting stale and the position holds the delegation. Funds
    the rewards pool so the migration's claimRewards step can pay out the
    bonus accrued on the bypass position.
    """
    val_addr = get_node_validator_addr(cluster)

    tiers = query_tiers(cluster).get("tiers", [])
    assert tiers, "expected at least one tier seeded by the v7 upgrade handler"
    tier_id = int(tiers[0]["id"])
    lock_amount = max(int(tiers[0]["min_lock_amount"]), V7_3_LOCK_AMOUNT)

    owner_addr = _create_permanent_lock_vesting_account(
        cluster, "v7_3_vest_poc", lock_amount, V7_3_GAS_TOPUP,
    )

    # Vesting owner delegates locked principal.
    rsp = cluster.delegate_amount(val_addr, f"{lock_amount}basecro", owner_addr)
    assert rsp["code"] == 0, rsp.get("raw_log", rsp)
    wait_for_new_blocks(cluster, 1)

    # commit vesting account's delegation to a tier position
    rsp = commit_delegation(
        cluster, "v7_3_vest_poc", val_addr, lock_amount, tier_id,
    )
    assert rsp["code"] == 0, (
        f"commit-delegation-to-tier failed on v7.2.0: "
        f"{rsp.get('raw_log', rsp)}"
    )

    positions = query_positions_by_owner(cluster, owner_addr).get("positions", [])
    assert len(positions) == 1, f"expected 1 position pre-upgrade, got {positions}"
    pos_id = int(positions[0]["id"])

    # Fund the rewards pool so the migration's claimRewards can pay out
    # any bonus accrued on the bypass position. Without this, the upgrade
    # handler hits ErrInsufficientBonusPool and halts the chain via a
    # BeginBlocker panic.
    rsp = fund_pool(cluster, "signer1", f"50000000{DENOM}")
    assert rsp["code"] == 0, f"fund_pool failed: {rsp.get('raw_log', rsp)}"
    wait_for_new_blocks(cluster, 1)

    return {
        "owner_addr": owner_addr,
        "val_addr": val_addr,
        "lock_amount": lock_amount,
        "pos_id": pos_id,
    }


def assert_v7_3_vesting_migration(cluster, ctx):
    owner_addr = ctx["owner_addr"]
    val_addr = ctx["val_addr"]
    lock_amount = ctx["lock_amount"]
    pos_id = ctx["pos_id"]


    # Verify that the position is deleted.
    positions_after = query_positions_by_owner(cluster, owner_addr).get("positions", [])
    assert positions_after == [], (
        f"expected zero positions post-upgrade, got {positions_after}"
    )

    # Vesting metadata still intact.
    post_acct = unwrap_account(cluster.cosmos_cli().account(owner_addr))
    assert post_acct["@type"] in (
        "cosmos-sdk/PermanentLockedAccount",
        "/cosmos.vesting.v1beta1.PermanentLockedAccount",
    ), f"vesting metadata must survive, got {post_acct}"

    # Owner has staking delegation restored.
    cli = cluster.cosmos_cli()
    deleg_raw = cli.raw(
        "query", "staking", "delegation",
        owner_addr, val_addr,
        output="json", node=cli.node_rpc,
    )
    deleg = json.loads(deleg_raw)
    deleg_amount = int(deleg["delegation_response"]["balance"]["amount"])
    assert deleg_amount == lock_amount, (
        f"owner delegation should be {lock_amount}, got {deleg_amount}"
    )

    # Ensure that the vesting lock still holds after undelegation.
    rsp = cluster.unbond_amount(val_addr, f"{lock_amount}basecro", owner_addr)
    assert rsp["code"] == 0, rsp.get("raw_log", rsp)
    time.sleep(15)  # genesis.jsonnet sets unbonding_time = 10s
    wait_for_new_blocks(cluster, 2)

    bal = cluster.balance(owner_addr, denom="basecro")
    assert bal >= lock_amount, (
        f"after undelegation, owner bank should hold ≥ {lock_amount}; got {bal}"
    )
    rsp = cluster.transfer(
        owner_addr,
        cluster.address("community"),
        f"{bal}basecro",
    )
    assert rsp["code"] != 0, (
        "vesting lock must hold after migration: send of full bank balance "
        f"should fail (bal={bal}, locked≥{lock_amount}); got {rsp}"
    )

    print("v7.3.0 vesting tier-bypass migration verified")
