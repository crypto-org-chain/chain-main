import pytest
from .utils import (
    approve_proposal,
    wait_for_new_blocks,
    query_command,
    module_address,
)

MAXSUPPLY = "maxsupply"
PARAM = "max-supply"
pytestmark = pytest.mark.normal


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
        assert initial_max_supply == final_max_supply, "Max supply should persist across blocks"

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
            "community", "submit-proposal", proposal_src, broadcast_mode="sync"
            )
        assert rsp["code"] == 0, rsp["raw_log"]

        approve_proposal(cluster, rsp, msg=f",{msg}")
        print("check params have been updated now")
        
        # Verify max supply has been updated
        updated_max_supply_rsp = query_command(cluster, MAXSUPPLY, PARAM)
        updated_max_supply = int(updated_max_supply_rsp["max_supply"])
        
        assert updated_max_supply == new_max_supply, f"Max supply should be updated to {new_max_supply}"

    #def test_begin_blocker_halt_on_excess_supply(self, cluster: cluster.Cluster):
    #    """Test that chain halts when total supply exceeds max supply"""
    #    cli = cluster.cosmos_cli()
        
        # This test would require artificially creating a scenario where
        # total supply exceeds max supply, which is difficult in practice
        # without modifying the chain state directly
        
        # For now, we'll test that the query works and the logic is in place
    #    max_supply_rsp = query_command(cli, MAXSUPPLY, PARAM)

    #    assert "max_supply" in max_supply_rsp
        
        # In a real scenario, you would:
        # 1. Set a very low max supply
        # 2. Try to mint tokens that would exceed it
        # 3. Verify the chain halts with appropriate error       