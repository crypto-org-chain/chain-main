import json
from datetime import timedelta

import pytest

from .utils import wait_for_block_time


@pytest.mark.slow
def test_param_proposal(cluster):
    """
    - send proposal to change max_validators
    - vote
    - check the result
    """
    max_validators = json.loads(
        cluster.raw("q", "staking", "params", output="json", node=cluster.node_rpc(0))
    )["max_validators"]

    proposer = cluster.address("community")
    rsp = cluster.gov_propose(
        proposer,
        {
            "title": "Increase number of max validators",
            "description": "ditto",
            "changes": [
                {
                    "subspace": "staking",
                    "key": "MaxValidators",
                    "value": max_validators + 1,
                }
            ],
        },
    )
    assert rsp["code"] == 0, rsp

    proposal = cluster.query_proposals()["proposals"][0]
    assert proposal["content"]["changes"] == [
        {
            "subspace": "staking",
            "key": "MaxValidators",
            "value": str(max_validators + 1),
        }
    ], proposal
    assert proposal["status"] == "PROPOSAL_STATUS_DEPOSIT_PERIOD", proposal

    depositor = cluster.address("ecosystem")
    rsp = cluster.gov_deposit(depositor, proposal["proposal_id"], "1cro")
    assert rsp["code"] == 0, rsp

    proposal = cluster.query_proposals()["proposals"][0]
    assert proposal["status"] == "PROPOSAL_STATUS_VOTING_PERIOD", proposal

    begin_time = cluster.block_time()

    rsp = cluster.gov_vote(cluster.address("validator"), proposal["proposal_id"], "yes")
    assert rsp["code"] == 0, rsp
    rsp = cluster.gov_vote(
        cluster.address("validator", i=1), proposal["proposal_id"], "yes", i=1
    )
    assert rsp["code"] == 0, rsp

    wait_for_block_time(cluster, begin_time + timedelta(seconds=10))

    proposal = cluster.query_proposals()["proposals"][0]
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal

    new_max_validators = json.loads(
        cluster.raw("q", "staking", "params", output="json", node=cluster.node_rpc(0))
    )["max_validators"]
    assert new_max_validators == max_validators + 1
