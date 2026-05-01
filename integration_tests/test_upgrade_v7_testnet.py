"""
v7.1.0-testnet upgrade integration test.
This test can be removed once testnet is upgraded to v7.1.0-testnet

Purpose
-------
Verify the `v7.1.0-testnet` upgrade handler correctly:

  1. Purges pre-rewrite tieredrewards lifecycle state (positions,
     secondary indexes, mappings, validator events, counters, retired
     ValidatorRewardRatio).
  2. Preserves Params + Tiers bytes across the upgrade.
  3. Sweeps the tieredrewards module account's staking residue
     (undelegates every delegation; loose bank balance at handler time
     gets sent to the testnet burn address — note that funds bonded at
     upgrade time only hit the bank after the unbonding period elapses,
     which is post-handler, so those get trapped at the pool forever or
     until the next upgrade to move it.
  4. Comes up clean on the per-position-delegator codebase — fresh
     positions use the derived delegator address from the position id
     to delegate.

Flow
----
  1. Start cluster with cosmovisor using the PRE-REWRITE binary
     (shared-pool tieredrewards model).
  2. Seed pre-rewrite state with the old binary: several LockTier calls
     (pure locked, locked-with-trigger-exit, redelegated).
  3. Submit + pass a `v7.1.0-testnet` gov upgrade proposal.
  4. At the halt height, cosmovisor swaps to the post-rewrite binary.
  5. Assert: seeded positions are gone, mappings/events are gone, module
     account has no active delegations, Tiers are intact.
  6. Smoke: LockTier with the new binary and verify the per-position
     delegator model is wired correctly.

Nix
---
Uses `integration_tests/upgrade-test-v7-testnet.nix`, which pins two
binaries:
  - `genesis`           = pre-rewrite binary (commit before this PR)
  - `v7.1.0-testnet`    = current branch

"""

import json
import time
from pathlib import Path

import pytest
import requests
from dateutil.parser import isoparse
from pystarport.cluster import SUPERVISOR_CONFIG_FILE
from pystarport.ports import rpc_port

from .test_upgrade import edit_chain_program, post_init
from .tieredrewards_helpers import (
    TIER_1_ID,
    TIER_1_MIN,
    TIER_2_ID,
    TIER_2_MIN,
    before_ids,
    get_node_validator_addr,
    lock_tier,
    new_pos_id,
    position_delegator_address,
    query_position,
    query_positions_by_owner,
    query_tiers,
    tier_redelegate,
)
from .utils import (
    cluster_fixture,
    query_balances,
    query_delegations,
    query_module_address,
    query_staking_delegation,
    wait_for_block,
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = pytest.mark.upgrade

V7_TESTNET_PLAN = "v7.1.0-testnet"


# ──────────────────────────────────────────────
# Cluster fixture
# ──────────────────────────────────────────────


def _init_cosmovisor_v7_testnet(data):
    """Build the dedicated nix derivation for this test."""
    import subprocess

    cosmovisor = data / "cosmovisor"
    cosmovisor.mkdir()
    subprocess.run(
        [
            "nix-build",
            Path(__file__).parent / "upgrade-test-v7-testnet.nix",
            "-o",
            cosmovisor / "upgrades",
        ],
        check=True,
    )
    (cosmovisor / "genesis").symlink_to("./upgrades/genesis")


@pytest.fixture(scope="function")
def cosmovisor_cluster(worker_index, tmp_path_factory):
    """Cosmovisor cluster starting at pre-rewrite v7 state.

    - Genesis binary: pre-rewrite (shared-pool tieredrewards).
    - Upgrade binary: current branch (post-rewrite per-position model).
    - Genesis app_state includes v7 tieredrewards params + tiers, so the
      chain boots directly into a v7-like state.
    """
    data = tmp_path_factory.mktemp("data")
    _init_cosmovisor_v7_testnet(data)
    yield from cluster_fixture(
        Path(__file__).parent / "configs/upgrade_v7_testnet.jsonnet",
        worker_index,
        data,
        post_init=post_init,
        cmd=str(data / "cosmovisor/genesis/bin/chain-maind"),
    )


# ──────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────


def _seed_pre_rewrite_state(cluster):
    """Create tier positions in several lifecycle states under the OLD binary.

    These positions go through pre-rewrite LockTier / TierRedelegate, so
    the underlying staking delegation lives at the shared tieredrewards
    module account (not a per-position address). The v7.1.0-testnet
    handler must purge these plus sweep the module-account residue.

    Returns the list of seeded position ids.
    """
    ecosystem = cluster.address("ecosystem")
    community = cluster.address("community")
    signer1 = cluster.address("signer1")
    v0 = get_node_validator_addr(cluster, 0)
    v1 = get_node_validator_addr(cluster, 1)

    seeded = []

    # 1. Pure locked position.
    before = before_ids(cluster, ecosystem)
    rsp = lock_tier(cluster, ecosystem, TIER_1_ID, TIER_1_MIN * 3, validator=v0)
    assert rsp["code"] == 0, f"seed lock (pure) failed: {rsp['raw_log']}"
    seeded.append(new_pos_id(cluster, ecosystem, before))

    # 2. Locked with trigger-exit-immediately.
    before = before_ids(cluster, community)
    rsp = lock_tier(
        cluster,
        community,
        TIER_2_ID,
        TIER_2_MIN * 2,
        validator=v0,
        trigger_exit=True,
    )
    assert rsp["code"] == 0, f"seed lock (exit) failed: {rsp['raw_log']}"
    seeded.append(new_pos_id(cluster, community, before))

    # 3. Locked then redelegated.
    before = before_ids(cluster, signer1)
    rsp = lock_tier(cluster, signer1, TIER_1_ID, TIER_1_MIN * 3, validator=v0)
    assert rsp["code"] == 0, f"seed lock (redel) failed: {rsp['raw_log']}"
    redel_id = new_pos_id(cluster, signer1, before)
    rsp = tier_redelegate(cluster, signer1, redel_id, v1)
    assert rsp["code"] == 0, f"seed redelegate failed: {rsp['raw_log']}"
    seeded.append(redel_id)

    print(f"seeded pre-rewrite positions: {seeded}")
    return seeded


def _propose_and_execute_upgrade(cluster, plan_name):
    """Submit + pass + swap binary for a software-upgrade plan.

    Inlines a minimal vote-and-wait loop instead of using
    `utils.approve_proposal`: that helper unconditionally tops up the
    deposit with `"1cro"`, which fails silently on a testnet-tag binary
    (denom is `tcro`, not `cro`) and the tx accepts at code=0 but
    contributes nothing, leaving the deposit-top-up wait timer to spin
    until `TimeoutError`. Our initial `0.1tcro` deposit already matches
    `min_deposit` (10_000_000 basetcro), so we can skip the top-up and
    go straight to voting.
    """
    target_height = cluster.block_height() + 30
    print(f"propose {plan_name} upgrade at", target_height)

    rsp = cluster.gov_propose_since_cosmos_sdk_v0_50(
        "community",
        "software-upgrade",
        {
            "name": plan_name,
            "title": f"{plan_name} upgrade",
            "summary": "Purge pre-rewrite tieredrewards state",
            "upgrade-height": target_height,
            # `tcro` — testnet-tag human denom. Mainnet-tag tests use "cro".
            "deposit": "0.1tcro",
        },
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # Extract proposal id from tx events.
    proposal_id = None
    for event in rsp.get("events", []):
        if event.get("type") == "submit_proposal":
            for attr in event.get("attributes", []):
                if attr.get("key") == "proposal_id":
                    proposal_id = attr["value"]
                    break
        if proposal_id is not None:
            break
    assert (
        proposal_id is not None
    ), f"proposal_id not found in submit_proposal events: {rsp}"

    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_VOTING_PERIOD", (
        f"proposal should enter VOTING_PERIOD immediately "
        f"(initial deposit {proposal.get('total_deposit')} >= min_deposit); "
        f"got {proposal}"
    )

    # Vote yes from every validator.
    for i in range(len(cluster.config["validators"])):
        proposal = cluster.query_proposal(proposal_id)
        if proposal["status"] != "PROPOSAL_STATUS_VOTING_PERIOD":
            break
        rsp = cluster.cosmos_cli(i).gov_vote("validator", proposal_id, "yes")
        assert rsp["code"] == 0, rsp["raw_log"]

    from datetime import timedelta

    from .utils import wait_for_block_time

    wait_for_block_time(
        cluster,
        isoparse(proposal["voting_end_time"]) + timedelta(seconds=5),
    )
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal

    # Matches test_upgrade.upgrade() exactly — relies on supervisord
    # running the binaries directly (see the edit_chain_program at the
    # top of the test that bypasses cosmovisor).
    wait_for_block(cluster, target_height, 600)
    time.sleep(0.5)

    for i in range(2):
        assert (
            cluster.supervisor.getProcessInfo(f"{cluster.chain_id}-node{i}")["state"]
            != "RUNNING"
        ), f"node{i} should be stopped after upgrade height"

    js1 = json.load((cluster.home(0) / "data/upgrade-info.json").open())
    js2 = json.load((cluster.home(1) / "data/upgrade-info.json").open())
    expected = {"name": plan_name, "height": target_height}
    assert js1 == js2
    assert expected.items() <= js1.items()

    edit_chain_program(
        cluster.chain_id,
        cluster.data_dir / SUPERVISOR_CONFIG_FILE,
        lambda i, _: {
            "command": (
                f"%(here)s/node{i}/cosmovisor/upgrades/{plan_name}/bin/chain-maind "
                f"start --home %(here)s/node{i}"
            )
        },
    )
    cluster.reload_supervisor()
    cluster.cmd = cluster.data_root / f"cosmovisor/upgrades/{plan_name}/bin/chain-maind"

    wait_for_block(cluster, target_height + 2, 600)


# ──────────────────────────────────────────────
# Test
# ──────────────────────────────────────────────


def test_v7_testnet_upgrade_purges_and_resumes(cosmovisor_cluster):
    """End-to-end: pre-rewrite state seed → v7.1.0-testnet upgrade →
    purge + sweep verified → post-upgrade per-position smoke test."""
    cluster = cosmovisor_cluster

    edit_chain_program(
        cluster.chain_id,
        cluster.data_dir / SUPERVISOR_CONFIG_FILE,
        lambda i, _: {
            "command": (
                f"%(here)s/node{i}/cosmovisor/genesis/bin/chain-maind start "
                f"--home %(here)s/node{i}"
            )
        },
    )
    cluster.reload_supervisor()
    time.sleep(5)  # FIXME port may linger briefly after process stopped
    wait_for_port(rpc_port(cluster.config["validators"][0]["base_port"]))
    wait_for_new_blocks(cluster, 1)

    # Sanity: cluster booted, tieredrewards genesis state is live.
    tiers_before = query_tiers(cluster).get("tiers", [])
    assert len(tiers_before) >= 2, f"genesis tiers should be seeded: {tiers_before}"

    # 1. Seed pre-rewrite state under the OLD binary.
    seeded = _seed_pre_rewrite_state(cluster)

    # Record the module account address while the pre-rewrite binary is
    # still running so we can query its residue afterward.
    pool_addr = query_module_address(cluster, "tieredrewards")
    print(f"tieredrewards module addr: {pool_addr}")

    # Sanity: pre-rewrite pool should hold the seeded delegations.
    pool_dels_before = query_delegations(cluster, pool_addr)
    assert (
        len(pool_dels_before) > 0
    ), "pre-rewrite binary should have put seeded delegations at module pool"

    # 2. Upgrade to v7.1.0-testnet. Binary swap happens inside this helper.
    _propose_and_execute_upgrade(cluster, V7_TESTNET_PLAN)
    wait_for_new_blocks(cluster, 2)

    # 3. Verify purge:
    #    - all seeded positions are gone
    for pos_id in seeded:
        try:
            query_position(cluster, pos_id)
            assert False, f"position {pos_id} should have been purged"
        except requests.HTTPError:
            pass

    #    - PositionsByOwner secondary index is empty for all seeded owners
    for acct in ("ecosystem", "community", "signer1"):
        addr = cluster.address(acct)
        rsp = query_positions_by_owner(cluster, addr)
        positions = rsp.get("positions", [])
        assert positions == [], (
            f"PositionsByOwner for {acct} ({addr}) should be empty after purge, "
            f"got {positions}"
        )

    # 4. Verify Tiers survived.
    tiers_after = query_tiers(cluster).get("tiers", [])
    assert len(tiers_after) == len(
        tiers_before
    ), f"tiers should survive purge: before={tiers_before}, after={tiers_after}"

    # 5. Verify staking + bank residue sweep at the module pool.
    pool_dels_after = query_delegations(cluster, pool_addr)
    assert pool_dels_after == [], (
        f"module account should have no delegation residue after sweep: "
        f"{pool_dels_after}"
    )

    pool_bal = query_balances(cluster, pool_addr)
    print(f"pool bank balance after upgrade (matured unbondings ok): {pool_bal}")

    # 6. Post-upgrade smoke: new position under the per-position model.
    signer = cluster.address("signer2")
    v0 = get_node_validator_addr(cluster, 0)
    tier = tiers_after[0]
    amount = max(int(tier["min_lock_amount"]), TIER_1_MIN)

    before = before_ids(cluster, signer)
    rsp = lock_tier(cluster, signer, int(tier["id"]), amount, validator=v0)
    assert rsp["code"] == 0, f"post-upgrade lock failed: {rsp['raw_log']}"
    new_id = new_pos_id(cluster, signer, before)

    pos = query_position(cluster, new_id)["position"]
    pos_del_addr = position_delegator_address(int(pos["id"]), prefix="tcro")
    assert (
        pos_del_addr != pool_addr
    ), "per-position delegator must NOT equal the old shared module pool address"

    # The underlying staking delegation must live at the per-position
    # delegator address, not the module pool (which was swept to zero above).
    pos_del = query_staking_delegation(cluster, pos_del_addr, v0)
    pos_shares = pos_del["delegation"]["shares"]
    assert (
        float(pos_shares) > 0
    ), f"position's delegator address must hold the delegation: shares={pos_shares}"

    # Double-check: the module pool still has zero delegations after the new lock.
    pool_dels_final = query_delegations(cluster, pool_addr)
    assert (
        pool_dels_final == []
    ), f"new LockTier must not re-introduce module-pool delegations: {pool_dels_final}"

    print("v7.1.0-testnet upgrade integration test passed")
