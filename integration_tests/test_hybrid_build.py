import configparser
import re
import sys
import time
from pathlib import Path
from xmlrpc.client import Fault

import pytest
from pystarport.cluster import SUPERVISOR_CONFIG_FILE
from pystarport.ports import rpc_port

from .utils import cluster_fixture, wait_for_block, wait_for_port


def safe_stop_process(supervisor, process_name):
    """Stop a process, ignoring if it's not running."""
    try:
        supervisor.stopProcess(process_name)
    except Fault as e:
        if "NOT_RUNNING" not in str(e):
            raise


def safe_start_process(supervisor, process_name):
    """Start a process, ignoring if it's already started."""
    try:
        supervisor.startProcess(process_name)
    except Fault as e:
        if "ALREADY_STARTED" not in str(e):
            raise


def edit_chain_program(chain_id, ini_path, callback):
    """Edit node process config in supervisor"""
    ini = configparser.RawConfigParser()
    ini.read_file(ini_path.open())
    reg = re.compile(rf"^program:{chain_id}-node(\d+)")
    for section in ini.sections():
        m = reg.match(section)
        if m:
            i = m.group(1)
            old = ini[section]
            ini[section].update(callback(i, old))
    with ini_path.open("w") as fp:
        ini.write(fp)


# Configuration for different binary builds per validator
# Update these paths to your actual binary locations
VALIDATOR_BINARIES = {
    0: "chain-maind",  # Default binary for node0
    1: "chain-maind",  # Default binary for node1
    2: "chain-maind",  # Default binary for node2
    # Example with different binaries:
    # 0: "/path/to/v5.0.0/chain-maind",
    # 1: "/path/to/v6.0.0/chain-maind",
    # 2: "/path/to/v7.0.0/chain-maind",
}

MAX_WAIT_SEC = 120


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    """Override cluster fixture for hybrid build test"""
    yield from cluster_fixture(
        Path(__file__).parent / "configs/hybrid_build.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
        )


def set_validator_binaries(cluster, binaries: dict):
    """
    Set different binaries for each validator node.
    Args:
        cluster: The cluster CLI instance
        binaries: Dict mapping node index to binary path
                  e.g., {0: "/path/bin1", 1: "/path/bin2", 2: "/path/bin3"}
    """
    def set_binary(i, old):
        node_idx = int(i)
        if node_idx in binaries:
            binary = binaries[node_idx]
            return {"command": f"{binary} start --home %(here)s/node{i}"}
        return {}

    edit_chain_program(
        cluster.chain_id,
        cluster.data_dir / SUPERVISOR_CONFIG_FILE,
        set_binary,
        )


def restart_all_nodes_with_binaries(cluster, binaries: dict, validator_count: int = 3):
    """
    Restart all validator nodes with specified binaries.
    Args:
        cluster: The cluster CLI instance
        binaries: Dict mapping node index to binary path
        validator_count: Number of validators in the cluster
    """
    print(f"\nStopping all {validator_count} nodes...")
    for i in range(validator_count):
        safe_stop_process(cluster.supervisor, f"{cluster.chain_id}-node{i}")

    print("Setting validator binaries...")
    for node_idx, binary in binaries.items():
        print(f"  node{node_idx}: {binary}")

    set_validator_binaries(cluster, binaries)

    print("Reloading supervisor and starting nodes with new binaries...")
    cluster.reload_supervisor()

    # Explicitly start all nodes (handles case where auto-start doesn't trigger)
    for i in range(validator_count):
        safe_start_process(cluster.supervisor, f"{cluster.chain_id}-node{i}")

    # Wait for all nodes to be ready
    for i in range(validator_count):
        wait_for_port(rpc_port(cluster.base_port(i)))

    print("All nodes started successfully")


def start_node_with_binary(cluster, node_idx: int, binary: str):
    """
    Start a specific node with a different binary.
    Args:
        cluster: The cluster CLI instance
        node_idx: Index of the node to restart
        binary: Path to the binary to use
    """
    print(f"\nRestarting node{node_idx} with binary: {binary}")

    # Stop the node
    safe_stop_process(cluster.supervisor, f"{cluster.chain_id}-node{node_idx}")

    # Update the binary
    def set_binary(i, old):
        if int(i) == node_idx:
            return {"command": f"{binary} start --home %(here)s/node{i}"}
        return {}

    edit_chain_program(
        cluster.chain_id,
        cluster.data_dir / SUPERVISOR_CONFIG_FILE,
        set_binary,
        )

    cluster.reload_supervisor()

    # Explicitly start the node (reload doesn't always auto-start single nodes)
    safe_start_process(cluster.supervisor, f"{cluster.chain_id}-node{node_idx}")

    # Wait for node to be ready
    wait_for_port(rpc_port(cluster.base_port(node_idx)))
    print(f"node{node_idx} started successfully")


@pytest.mark.hybrid
def test_hybrid_build_basic(cluster):
    """
    Test that validators can run with different binary builds.
    This test:
    1. Verifies the cluster starts normally
    2. Restarts validators with different binaries (from VALIDATOR_BINARIES)
    3. Verifies all nodes can produce blocks together
    4. Verifies basic transactions work
    """
    validator_count = len(cluster.config["validators"])
    print(f"\n=== Hybrid Build Test with {validator_count} validators ===")

    # Verify initial cluster state
    assert len(cluster.validators()) == validator_count
    initial_height = cluster.block_height()
    print(f"Initial block height: {initial_height}")

    # Restart all nodes with configured binaries
    restart_all_nodes_with_binaries(cluster, VALIDATOR_BINARIES, validator_count)

    # Wait for blocks to be produced with hybrid binaries
    print("\nWaiting for new blocks with hybrid binaries...")
    wait_for_block(cluster, initial_height + 5, timeout=MAX_WAIT_SEC)
    new_height = cluster.block_height()
    print(f"New block height: {new_height}")
    assert new_height > initial_height, "Chain should produce new blocks"

    # Test a basic transfer to verify chain is functional
    print("\nTesting basic transfer...")
    from_addr = cluster.address("community")
    to_addr = cluster.address("signer1")
    initial_balance = cluster.balance(to_addr)

    rsp = cluster.transfer(from_addr, to_addr, "1000basecro")
    assert rsp["code"] == 0, f"Transfer failed: {rsp.get('raw_log', rsp)}"

    # Verify balance changed
    new_balance = cluster.balance(to_addr)
    assert new_balance != initial_balance, "Balance should have changed after transfer"


@pytest.mark.hybrid
def test_hybrid_build_rolling_upgrade(cluster):
    """
    Test rolling upgrade scenario: upgrade validators one by one to different binaries.
    This simulates a real-world rolling upgrade where nodes are upgraded
    one at a time while the network continues to produce blocks.
    """
    validator_count = len(cluster.config["validators"])
    print(f"\n=== Rolling Upgrade Test with {validator_count} validators ===")

    # Verify initial state
    initial_height = cluster.block_height()
    print(f"Initial block height: {initial_height}")

    # Upgrade validators one by one
    for node_idx in range(validator_count):
        binary = VALIDATOR_BINARIES.get(node_idx, "chain-maind")
        print(f"\n--- Upgrading node{node_idx} to: {binary} ---")

        # Record height before upgrade
        height_before = cluster.block_height()

        # Upgrade this node
        start_node_with_binary(cluster, node_idx, binary)

        # Wait for new blocks to ensure chain is still producing
        wait_for_block(cluster, height_before + 3, timeout=MAX_WAIT_SEC)
        height_after = cluster.block_height()
        print(f"Blocks produced after upgrading node{node_idx}: {height_after - height_before}")

        assert height_after > height_before, f"Chain should continue after upgrading node{node_idx}"

    # Final verification
    final_height = cluster.block_height()
    print(f"\nFinal block height: {final_height}")
    print(f"Total blocks produced during rolling upgrade: {final_height - initial_height}")

    assert final_height > initial_height, "Chain should have produced blocks during upgrade"

    # Verify all validators are still active after rolling upgrade
    validators = cluster.validators()
    assert len(validators) == validator_count, "All validators should still exist"
    for i, val in enumerate(validators):
        assert val["status"] == "BOND_STATUS_BONDED", f"Validator {i} should be bonded"
        assert not val.get("jailed", False), f"Validator {i} should not be jailed"