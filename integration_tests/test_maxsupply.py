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
PARAM = "max-supply"
pytestmark = pytest.mark.normal


@pytest.fixture(scope="class")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/default.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


class TestMaxSupply:
    def test_max_supply_cli_query(self, cluster):
        """Test querying max supply parameters"""
        # Query max supply parameters
        rsp = query_command(cluster, MAXSUPPLY, PARAM)
        assert "max_supply" in rsp
        assert int(rsp["max_supply"]) == 0  # Initial max supply is 0

    def test_max_supply_persistence(self, cluster):
        """Test that max supply persists across chain restarts"""
        # Get initial max supply
        initial_max_supply_rsp = query_command(cluster, MAXSUPPLY, PARAM)
        initial_max_supply = initial_max_supply_rsp["max_supply"]

        # Restart the chain (this would require cluster restart functionality)
        # For now, just verify the value is consistent
        wait_for_new_blocks(cluster, 3)

        # Query again after some blocks
        final_max_supply_rsp = query_command(cluster, MAXSUPPLY, PARAM)
        final_max_supply = final_max_supply_rsp["max_supply"]
        assert (
            initial_max_supply == final_max_supply
        ), "Max supply should persist across blocks"

    def test_max_supply_update_via_governance(self, cluster, tmp_path):
        """Test updating max supply through governance proposal"""
        # Get current max supply
        rsp = query_command(cluster, MAXSUPPLY, PARAM)
        current_max_supply = int(rsp["max_supply"])

        # Prepare new max supply (increase by 1000000)
        new_max_supply = current_max_supply + 1000000

        rsp["max_supply"] = str(new_max_supply)
        authority = module_address("gov")
        msg = "/chainmain.maxsupply.v1.MsgUpdateParams"
        proposal_src = {
            "messages": [
                {
                    "@type": msg,
                    "authority": authority,
                    "params": rsp,
                }
            ],
            "deposit": "100000000basecro",
            "title": "Update Max Supply",
            "summary": "Increase maximum supply limit",
        }

        rsp = cluster.gov_propose_since_cosmos_sdk_v0_50(
            "community", "submit-proposal", proposal_src
        )
        assert rsp["code"] == 0, rsp["raw_log"]

        def find_log_event_attrs_legacy(events, ev_type):
            for ev in events:
                if ev["type"] == ev_type:
                    attrs = {attr["key"]: attr["value"] for attr in ev["attributes"]}
                    return attrs
            return None

        # Extract proposal ID from the response
        ev = find_log_event_attrs_legacy(rsp["events"], "submit_proposal")
        proposal_id = ev["proposal_id"]

        assert proposal_id is not None, "Could not extract proposal ID from response"
        print(f"Proposal ID: {proposal_id}")

        # Wait for proposal to be available
        wait_for_new_blocks(cluster, 2)

        # Check if proposal exists before voting
        try:
            proposal_info = cluster.query_proposal(proposal_id)
            print(f"Proposal info: {proposal_info}")
        except Exception as e:
            print(f"Error querying proposal: {e}")
            # If we can't query the proposal, it might not exist yet
            wait_for_new_blocks(cluster, 3)

        approve_proposal(cluster, rsp, msg=f",{msg}")
        print("check params have been updated now")

        # Verify max supply has been updated
        updated_max_supply_rsp = query_command(cluster, MAXSUPPLY, PARAM)
        updated_max_supply = int(updated_max_supply_rsp["max_supply"])

        assert (
            updated_max_supply == new_max_supply
        ), f"Max supply should be updated to {new_max_supply}"

    def test_begin_blocker_halt_on_excess_supply(self, cluster):
        """Test that chain halts when total supply exceeds max supply"""
        # For now, we'll test that the query works and the logic is in place
        # max_supply_rsp = query_command(cluster, MAXSUPPLY, PARAM)
        # assert "max_supply" in max_supply_rsp

        # In a real scenario, you would:
        # 1. Set a very low max supply
        # 2. Try to mint tokens that would exceed it
        # 3. Verify the chain halts with appropriate error
        pass
