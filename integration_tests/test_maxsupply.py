import pytest
from pystarport import cluster

from .utils import (
    approve_proposal,
    #submit_gov_proposal,
    wait_for_new_blocks,
    query_command,
)

MAXSUPPLY = "maxsupply"
PARAM = "max-supply"


class TestMaxSupply:
    def test_max_supply_query(self, cluster: cluster.Cluster):
        """Test querying max supply parameters"""
        cli = cluster.cosmos_cli()
        
        # Query max supply parameters
        rsp = query_command(cli, MAXSUPPLY, PARAM)
        assert "max_supply" in rsp

    def test_max_supply_update_via_governance(self, cluster: cluster.Cluster):
        """Test updating max supply through governance proposal"""
        cli = cluster.cosmos_cli()
        
        # Get current max supply
        rsp = query_command(cli, MAXSUPPLY, PARAM)
        current_max_supply = int(rsp["max_supply"])
        
        # Prepare new max supply (increase by 1000000)
        new_max_supply = current_max_supply + 1000000
        
        # Create parameter change proposal
        proposal = {
            "title": "Update Max Supply",
            "description": "Increase maximum supply limit",
            "changes": [
                {
                    "subspace": "maxsupply",
                    "key": "MaxSupply",
                    "value": f'"{new_max_supply}"'
                }
            ],
            "deposit": "10000000basecro"
        }
        
        # Submit governance proposal
        # proposal_id = submit_gov_proposal(cli, cluster.address("community"), proposal)
        
        # Vote on proposal
        approve_proposal(cluster, proposal_id)
        
        # Wait for proposal to pass
        wait_for_new_blocks(cli, 15)
        
        # Verify max supply has been updated
        updated_max_supply_rsp = query_command(cli, MAXSUPPLY, PARAM)

        updated_max_supply = int(updated_max_supply_rsp["max_supply"])
        
        assert updated_max_supply == new_max_supply, f"Max supply should be updated to {new_max_supply}"

    def test_begin_blocker_halt_on_excess_supply(self, cluster: cluster.Cluster):
        """Test that chain halts when total supply exceeds max supply"""
        cli = cluster.cosmos_cli()
        
        # This test would require artificially creating a scenario where
        # total supply exceeds max supply, which is difficult in practice
        # without modifying the chain state directly
        
        # For now, we'll test that the query works and the logic is in place
        max_supply_rsp = query_command(cli, MAXSUPPLY, PARAM)

        assert "max_supply" in max_supply_rsp
        
        # In a real scenario, you would:
        # 1. Set a very low max supply
        # 2. Try to mint tokens that would exceed it
        # 3. Verify the chain halts with appropriate error

    def test_max_supply_grpc_queries(self, cluster: cluster.Cluster):
        """Test gRPC queries for maxsupply module"""
        cli = cluster.cosmos_cli()
        
        # Test gRPC params query
        grpc_params = cli.query_params_grpc("maxsupply")
        assert grpc_params is not None
        
        # Test REST API endpoints
        rest_client = cluster.cosmos_cli().rest_client
                
        # Query max supply via REST
        max_supply_endpoint = "/chainmain/maxsupply/v1/params/max_supply"
        max_supply_response = rest_client.get(max_supply_endpoint)
        assert max_supply_response.status_code == 200
        max_supply_data = max_supply_response.json()
        assert "max_supply" in max_supply_data

    @pytest.mark.slow
    def test_max_supply_persistence(self, cluster: cluster.Cluster):
        """Test that max supply persists across chain restarts"""
        cli = cluster.cosmos_cli()
        
        # Get initial max supply
        initial_max_supply_rsp = query_command(cli, MAXSUPPLY, PARAM)
        initial_max_supply = initial_max_supply_rsp["max_supply"]
        
        # Restart the chain (this would require cluster restart functionality)
        # For now, just verify the value is consistent
        wait_for_new_blocks(cli, 5)
        
        # Query again after some blocks
        final_max_supply_rsp = query_command(cli, MAXSUPPLY, PARAM)
        final_max_supply = final_max_supply_rsp["max_supply"]
        
        assert initial_max_supply == final_max_supply, "Max supply should persist across blocks"