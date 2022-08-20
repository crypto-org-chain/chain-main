import sys
import time
from pathlib import Path

import pytest

from .utils import cluster_fixture

MAX_SLEEP_SEC = 600


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/byzantine.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


@pytest.mark.byzantine
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
    val_addr_byzantine = cluster.address("validator", i=from_node, bech="val")
    val_addr_slash = cluster.address("validator", i=to_node, bech="val")
    tokens_byzantine_before = int((cluster.validator(val_addr_byzantine))["tokens"])
    tokens_slash_before = int((cluster.validator(val_addr_slash))["tokens"])
    cluster.stop_node(to_node)
    cluster.copy_validator_key(from_node, to_node)
    cluster.start_node(to_node)

    # it may take 30s to finish the loop
    i = 0
    while i < MAX_SLEEP_SEC:
        time.sleep(1)
        sys.stdout.write(".")
        sys.stdout.flush()
        i += 1
        val1 = cluster.validator(val_addr_byzantine)
        if val1["jailed"]:
            break
    assert val1["jailed"]
    assert any(
        [
            val1["status"] == "BOND_STATUS_UNBONDING",
            val1["status"] == "BOND_STATUS_UNBONDED",
        ]
    )
    print("\n{}s waiting for node 1 jailed".format(i))

    i = 0
    # it may take 2min to finish the loop
    while i < MAX_SLEEP_SEC:
        time.sleep(1)
        i += 1
        sys.stdout.write(".")
        sys.stdout.flush()
        val2 = cluster.validator(val_addr_slash)
        if val2["jailed"]:
            break
    assert val2["jailed"]
    assert any(
        [
            val1["status"] == "BOND_STATUS_UNBONDING",
            val1["status"] == "BOND_STATUS_UNBONDED",
        ]
    )
    print("\n{}s waiting for node 2 jailed".format(i))

    tokens_byzantine_after = int((cluster.validator(val_addr_byzantine))["tokens"])
    tokens_slash_after = int((cluster.validator(val_addr_slash))["tokens"])
    assert tokens_byzantine_before * 0.95 == tokens_byzantine_after
    assert tokens_slash_before * 0.99 == tokens_slash_after
