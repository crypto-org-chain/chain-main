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

MAXSUPPLY = "maxsupply"
TOTALSUPPLY = "total-supply-of"
BANK_MODULE = "bank"
PARAM = "max-supply"
DENOM = "basecro"
AMOUNT = "amount"
MSG = "/chainmain.maxsupply.v1.MsgUpdateParams"
ERROR = "failed to apply block; error the total supply has exceeded the maximum supply"

pytestmark = pytest.mark.normal


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


def _find_event_proposal_id(events):
    for ev in events:
        if ev["type"] == "submit_proposal":
            attrs = {attr["key"]: attr["value"] for attr in ev["attributes"]}
            p_id = attrs["proposal_id"]
            assert p_id is not None, "Could not extract proposal ID from response"
            print(f"Proposal ID: {p_id}")
            return p_id
    return None


def _check_proposal_exist(cluster, proposal_id, timeout_seconds=60):
    """Check if proposal exists with timeout mechanism"""
    start_time = time.time()

    while time.time() - start_time < timeout_seconds:
        try:
            proposal_info = cluster.query_proposal(proposal_id)
            print(f"Proposal info: {proposal_info}")
            return True
        except Exception as e:
            print(f"Error querying proposal: {e}")
            # If we can't query the proposal, it might not exist yet
            time.sleep(1)
    # If we reach here, timeout occurred
    raise TimeoutError(
        f"Timeout waiting for proposal {proposal_id} to become available "
        f"after {timeout_seconds} seconds"
    )


def test_max_supply_cli_query(cluster):
    """Test querying max supply parameters"""
    # Query max supply parameters
    rsp = query_command(cluster, MAXSUPPLY, PARAM)
    assert "max_supply" in rsp
    assert int(rsp["max_supply"]) == 0  # the max supply is 0 by default


def test_max_supply_persistence(cluster):
    """Test that max supply persists across chain restarts"""
    # Get initial max supply
    initial_max_supply_rsp = query_command(cluster, MAXSUPPLY, PARAM)
    initial_max_supply = initial_max_supply_rsp["max_supply"]

    # Restart the chain (this would require cluster restart functionality)
    # For now, just verify the value is consistent
    wait_for_new_blocks(cluster, 1)

    # Query again after some blocks
    final_max_supply_rsp = query_command(cluster, MAXSUPPLY, PARAM)
    final_max_supply = final_max_supply_rsp["max_supply"]
    assert (
        initial_max_supply == final_max_supply
    ), "Max supply should persist across blocks"


def test_max_supply_update_via_governance(cluster):
    """Test updating max supply through governance proposal"""
    # Get current max supply
    rsp = query_command(cluster, MAXSUPPLY, PARAM)
    current_max_supply = int(rsp["max_supply"])

    # Prepare new max supply (increase by 2000000000000)
    new_max_supply = current_max_supply + 2000000000000

    rsp["max_supply"] = str(new_max_supply)
    proposal_src = _create_max_supply_proposal(rsp)

    rsp = cluster.gov_propose_since_cosmos_sdk_v0_50(
        "community", "submit-proposal", proposal_src
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # Extract proposal ID from the response
    proposal_id = _find_event_proposal_id(rsp["events"])

    # Wait for proposal to be available
    wait_for_new_blocks(cluster, 1)

    # Check if proposal exists before voting
    _check_proposal_exist(cluster, proposal_id)

    # Vote on proposal
    approve_proposal(cluster, rsp, msg=f",{MSG}")
    print("check params have been updated now")

    # Verify max supply has been updated
    updated_max_supply_rsp = query_command(cluster, MAXSUPPLY, PARAM)
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
    rsp = query_command(cluster, MAXSUPPLY, PARAM)
    assert "max_supply" in rsp

    # Prepare new max supply (increase by 450000) and submit a proposal
    # Around 13 blocks should pass before total supply exceeds max supply
    new_max_supply = current_total_supply + 450000
    rsp["max_supply"] = str(new_max_supply)
    proposal_src = _create_max_supply_proposal(rsp)

    rsp = cluster.gov_propose_since_cosmos_sdk_v0_50(
        "community", "submit-proposal", proposal_src
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # Extract proposal ID from the response
    proposal_id = _find_event_proposal_id(rsp["events"])

    # Wait for proposal to be available
    wait_for_new_blocks(cluster, 1)

    # Check if proposal exists before voting
    _check_proposal_exist(cluster, proposal_id)

    # Vote on proposal
    approve_proposal(cluster, rsp, msg=f",{MSG}")

    # Verify max supply has been updated
    updated_max_supply_rsp = query_command(cluster, MAXSUPPLY, PARAM)
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
        assert False, f"Test case failed due to exeception: {e}."
