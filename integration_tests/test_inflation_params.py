import time
from pathlib import Path

import pytest

from .utils import (
    approve_proposal,
    cluster_fixture,
    module_address,
    query_command,
    wait_for_new_blocks,
)

INFLATION_MODULE = "inflation"
TOTALSUPPLY = "total-supply-of"
BANK_MODULE = "bank"
PARAMS = "params"
DENOM = "basecro"
AMOUNT = "amount"
MSG = "/chainmain.inflation.v1.MsgUpdateParams"
ERROR = "the total supply has exceeded the maximum supply"

pytestmark = pytest.mark.inflation


@pytest.fixture(scope="module")
def cluster(worker_index, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/default.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def _create_max_supply_proposal(params):
    """Create a governance proposal to update max supply parameters"""
    authority = module_address("gov")
    proposal_src = {
        "messages": [
            {
                "@type": MSG,
                "authority": authority,
                "params": params,
            }
        ],
        "deposit": "100000000basecro",
        "title": "Update Max Supply",
        "summary": "Increase maximum supply limit",
    }
    return proposal_src


def test_params_cli_query(cluster):
    """Test querying max supply parameters"""
    params = query_command(cluster, INFLATION_MODULE, PARAMS)["params"]
    assert "max_supply" in params
    assert int(params["max_supply"]) == 0
    assert "burned_addresses" in params
    assert len(params["burned_addresses"]) == 0
    assert "decay_rate" in params
    assert float(params["decay_rate"]) == 0.0
    assert int(params["max_supply"]) == 0  # the max supply is 0 by default


def test_max_supply_persistence(cluster):
    """Test that max supply persists across chain restarts"""
    # Get initial max supply
    initial_max_supply_rsp = query_command(cluster, INFLATION_MODULE, PARAMS)["params"]
    initial_max_supply = initial_max_supply_rsp["max_supply"]

    wait_for_new_blocks(cluster, 1)

    # Query again after some blocks
    final_max_supply_rsp = query_command(cluster, INFLATION_MODULE, PARAMS)["params"]
    final_max_supply = final_max_supply_rsp["max_supply"]
    assert (
        initial_max_supply == final_max_supply
    ), "Max supply should persist across blocks"


def test_max_supply_update_via_governance(cluster):
    """Test updating max supply through governance proposal"""
    # Get current max supply
    rsp = query_command(cluster, INFLATION_MODULE, PARAMS)["params"]
    current_max_supply = int(rsp["max_supply"])

    # Prepare new max supply (increase by 2000000000000)
    new_max_supply = current_max_supply + 2000000000000

    rsp["max_supply"] = str(new_max_supply)
    proposal_src = _create_max_supply_proposal(rsp)

    rsp = cluster.gov_propose_since_cosmos_sdk_v0_50(
        "community", "submit-proposal", proposal_src
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # Vote on proposal
    approve_proposal(cluster, rsp, msg=f",{MSG}")

    # Verify max supply has been updated
    updated_max_supply_rsp = query_command(cluster, INFLATION_MODULE, PARAMS)["params"]
    updated_max_supply = int(updated_max_supply_rsp["max_supply"])

    assert (
        updated_max_supply == new_max_supply
    ), f"Max supply should be updated to {new_max_supply}"


def test_begin_blocker_halt_on_excess_supply(cluster):
    """Test that chain halts when total supply exceeds max supply"""

    total_supply_rsp = query_command(cluster, BANK_MODULE, TOTALSUPPLY, DENOM)
    current_total_supply = int(total_supply_rsp[AMOUNT][AMOUNT])
    print("current_total_supply:", current_total_supply)

    # Get current max supply
    rsp = query_command(cluster, INFLATION_MODULE, PARAMS)["params"]
    assert "max_supply" in rsp

    # Prepare new max supply (increase by 480000) and submit a proposal
    new_max_supply = current_total_supply + 480000
    rsp["max_supply"] = str(new_max_supply)
    proposal_src = _create_max_supply_proposal(rsp)

    rsp = cluster.gov_propose_since_cosmos_sdk_v0_50(
        "community", "submit-proposal", proposal_src
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # Vote on proposal
    approve_proposal(cluster, rsp, msg=f",{MSG}")

    # Verify max supply has been updated
    updated_max_supply_rsp = query_command(cluster, INFLATION_MODULE, PARAMS)["params"]
    updated_max_supply = int(updated_max_supply_rsp["max_supply"])
    assert (
        updated_max_supply == new_max_supply
    ), f"Max supply should be updated to {new_max_supply}"

    def _halted_chain():
        print(" halting chain...")
        node0_info = cluster.supervisor.getProcessInfo(f"{cluster.chain_id}-node0")
        halted0 = node0_info["state"] != "RUNNING"
        node1_info = cluster.supervisor.getProcessInfo(f"{cluster.chain_id}-node1")
        halted1 = node1_info["state"] != "RUNNING"
        return halted0 and halted1

    # Wait new blocks in order to reach the halt
    timeout_seconds = 120
    start_time = time.time()
    try:
        while not _halted_chain():
            if time.time() - start_time > timeout_seconds:
                timeout_msg = (
                    f"Timeout after {timeout_seconds} seconds "
                    f"waiting for chain to halt"
                )
                assert False, timeout_msg

            time.sleep(1)

        print("Chain has been halted")
        time.sleep(1)
        # Check the node's log for errors matches the expected message
        log_file = f"{cluster.home(0)}/../node0.log"
        with open(log_file, "r") as f:
            log_content = f.read()
            print("log_content:", log_content)
            assert ERROR in log_content, "Expected error message not found in log"
        pass
    except Exception as e:
        assert False, f"Test case failed due to exception: {e}."
