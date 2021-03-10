import sys
import time
from pathlib import Path

import pytest

from .utils import cluster_fixture


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/byzantine.yaml",
        worker_index,
        tmp_path_factory,
        quiet=pytestconfig.getoption("supervisord-quiet"),
    )


@pytest.mark.slow
def test_byzantine(cluster):
    """
    - 3 nodes
    - node0 has more than 2/3 voting powers
    - stop node2
    - copy node1's validator key to node2
    - start node2
    - check node1 & node2 jailed
    """

    assert len(cluster.validators()) == 3
    from_node = 1
    to_node = 2
    cluster.stop_node(to_node)
    cluster.copy_validator_key(from_node, to_node)
    cluster.start_node(to_node)

    # it may take 30s to finish the loop
    i = 0
    while i < 300:
        time.sleep(1)
        sys.stdout.write(".")
        sys.stdout.flush()
        i += 1
        val_addr = cluster.address("validator", from_node, bech="val")
        val_1 = cluster.validator(val_addr)
        if val_1["jailed"]:
            break
    assert val_1["jailed"]
    assert val_1["status"] == "BOND_STATUS_UNBONDING"
    print("\n{}s waiting for node 1 jailed".format(i))

    i = 0
    # it may take 2min to finish the loop
    while i < 300:
        time.sleep(1)
        i += 1
        sys.stdout.write(".")
        sys.stdout.flush()
        val_addr = cluster.address("validator", to_node, bech="val")
        val_2 = cluster.validator(val_addr)
        if val_2["jailed"]:
            break
    assert val_2["jailed"]
    assert val_2["status"] == "BOND_STATUS_UNBONDING"
    print("\n{}s waiting for node 2 jailed".format(i))
