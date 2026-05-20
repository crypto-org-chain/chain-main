"""
CAN BE REMOVED ONCE MAINNET IS UPGRADED TO v7.2.0

v7 upgrade integration test — orphan rewards_pool BaseAccount regression.

Purpose
-------
Verify the v7 upgrade handler converts a pre-existing BaseAccount sitting at
the rewards_pool module address into a proper ModuleAccount, preserving the
balance accrued there before the module was registered.

Without the heal added to app/upgrades.go::registerV7UpgradeHandler, the
v7 RunMigrations panics inside InitGenesis with "account is not a module
account" when an external account had pre-funded the derived rewards_pool
address.

Flow
----
  1. Start cluster with cosmovisor on the v6 binary (no tieredrewards
     module — rewards_pool address is unowned).
  2. Transfer funds from the community account to the rewards_pool address.
     The bank keeper auto-creates a BaseAccount there.
  3. Sanity-check: the address now resolves to a BaseAccount with the
     funded balance.
  4. Submit + pass the v7 software-upgrade proposal; cosmovisor swaps in
     the post-upgrade binary.
  5. Assert: rewards_pool address is now a ModuleAccount, and the balance
     funded under v6 is preserved.

Nix
---
Uses integration_tests/upgrade-test-v7.nix, which pins:
  - genesis = v6.0.0 (released)
  - v7      = current branch
"""

import subprocess
import time
from pathlib import Path

import pytest
from pystarport.cluster import SUPERVISOR_CONFIG_FILE
from pystarport.ports import rpc_port

from .test_upgrade import edit_chain_program, post_init, propose_n_execute_v7_upgrade
from .tieredrewards_helpers import (
    get_node_validator_addr,
    lock_tier,
    query_positions_by_owner,
    query_tiers,
)
from .utils import (
    cluster_fixture,
    module_address,
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = pytest.mark.upgrade

REWARDS_POOL_NAME = "rewards_pool"


def _init_cosmovisor_v7(data):
    """Build the v6 → v7 nix derivation for this test."""
    cosmovisor = data / "cosmovisor"
    cosmovisor.mkdir()
    subprocess.run(
        [
            "nix-build",
            Path(__file__).parent / "upgrade-test-v7.nix",
            "-o",
            cosmovisor / "upgrades",
        ],
        check=True,
    )
    (cosmovisor / "genesis").symlink_to("./upgrades/genesis")


@pytest.fixture(scope="function")
def cosmovisor_cluster(worker_index, tmp_path_factory):
    """Cosmovisor cluster starting on v6, with v7 staged for upgrade."""
    data = tmp_path_factory.mktemp("data")
    _init_cosmovisor_v7(data)
    yield from cluster_fixture(
        Path(__file__).parent / "configs/upgrade_v7.jsonnet",
        worker_index,
        data,
        post_init=post_init,
        cmd=str(data / "cosmovisor/genesis/bin/chain-maind"),
    )


def test_v7_upgrade_heals_orphan_rewards_pool(cosmovisor_cluster):
    cluster = cosmovisor_cluster

    # Run via the v6 ("genesis") binary directly — same pattern as
    # test_upgrade.test_manual_upgrade_all when bypassing cosmovisor.
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
    time.sleep(5)  # FIXME: port lingers briefly after process stopped
    wait_for_port(rpc_port(cluster.config["validators"][0]["base_port"]))
    wait_for_new_blocks(cluster, 1)

    # 1. Pre-fund the rewards_pool address from a regular user account.
    # On v6 the tieredrewards module is unregistered, so this leaves a
    # BaseAccount at the address with the funded balance.
    pool_addr = module_address(REWARDS_POOL_NAME)
    prefund_amount = 1_234_567  # basecro
    community_addr = cluster.address("community")
    rsp = cluster.transfer(community_addr, pool_addr, f"{prefund_amount}basecro")
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cluster, 1)

    cli = cluster.cosmos_cli()

    # 2. Sanity: v6 created a BaseAccount at the address with the funds.
    pre_acct = cli.account(pool_addr)
    assert (
        pre_acct["@type"] == "/cosmos.auth.v1beta1.BaseAccount"
    ), f"expected BaseAccount on v6, got {pre_acct}"
    assert cluster.balance(pool_addr, denom="basecro") == prefund_amount

    # 3. Run the v7 upgrade. Without the orphan-account heal, this panics
    # at InitGenesis with "account is not a module account".
    propose_n_execute_v7_upgrade(cluster)

    # 4. Post-upgrade: the address must now hold a ModuleAccount, and the
    # bank balance must survive the conversion.
    cli = cluster.cosmos_cli()
    post_acct = cli.account(pool_addr)
    assert (
        post_acct["@type"] == "/cosmos.auth.v1beta1.ModuleAccount"
    ), f"expected ModuleAccount after v7, got {post_acct}"
    assert post_acct["name"] == REWARDS_POOL_NAME, post_acct
    assert (
        cluster.balance(pool_addr, denom="basecro") == prefund_amount
    ), "rewards_pool balance should survive the BaseAccount→ModuleAccount conversion"

    # 5. Smoke test: lock-tier from a non-vested account. Confirms the
    # module account permissions are wired up correctly post-conversion
    # (staking delegation + bank send-from-module both succeed) and that
    # the v7 handler seeded the default tiers.
    tiers = query_tiers(cluster).get("tiers", [])
    assert len(tiers) > 0, f"v7 handler should have seeded tiers: {tiers}"

    signer1_addr = cluster.address("signer1")
    validator_addr = get_node_validator_addr(cluster)
    tier_id = tiers[0]["id"]
    lock_amount = int(tiers[0]["min_lock_amount"])

    rsp = lock_tier(cluster, signer1_addr, tier_id, lock_amount, validator_addr)
    assert rsp["code"] == 0, f"lock-tier failed: {rsp.get('raw_log', rsp)}"

    positions = query_positions_by_owner(cluster, signer1_addr).get("positions", [])
    assert (
        len(positions) == 1
    ), f"expected 1 position after lock-tier, got {len(positions)}: {positions}"
