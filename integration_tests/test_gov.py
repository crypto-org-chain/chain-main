import json
from datetime import timedelta

import pytest
from dateutil.parser import isoparse

from .utils import wait_for_block, wait_for_block_time


@pytest.mark.slow
def test_param_proposal(cluster):
    """
    - send proposal to change max_validators
    - vote
    - check the result
    - check deposit refunded
    """
    max_validators = json.loads(
        cluster.raw("q", "staking", "params", output="json", node=cluster.node_rpc(0))
    )["max_validators"]

    rsp = cluster.gov_propose(
        "community",
        "param-change",
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

    # get proposal_id
    log = {
        attr["key"]: attr["value"] for attr in rsp["logs"][0]["events"][2]["attributes"]
    }
    assert log["proposal_type"] == "ParameterChange", rsp
    proposal_id = log["proposal_id"]

    proposal = cluster.query_proposal(proposal_id)
    assert proposal["content"]["changes"] == [
        {
            "subspace": "staking",
            "key": "MaxValidators",
            "value": str(max_validators + 1),
        }
    ], proposal
    assert proposal["status"] == "PROPOSAL_STATUS_DEPOSIT_PERIOD", proposal

    amount = cluster.balance(cluster.address("ecosystem"))
    rsp = cluster.gov_deposit("ecosystem", proposal_id, "1cro")
    assert rsp["code"] == 0, rsp
    assert cluster.balance(cluster.address("ecosystem")) == amount - 100000000

    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_VOTING_PERIOD", proposal

    rsp = cluster.gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp
    rsp = cluster.gov_vote("validator", proposal_id, "yes", i=1)
    assert rsp["code"] == 0, rsp

    wait_for_block_time(
        cluster, isoparse(proposal["voting_end_time"]) + timedelta(seconds=10)
    )

    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal

    new_max_validators = json.loads(
        cluster.raw("q", "staking", "params", output="json", node=cluster.node_rpc(0))
    )["max_validators"]
    assert new_max_validators == max_validators + 1

    # refunded
    assert cluster.balance(cluster.address("ecosystem")) == amount


@pytest.mark.slow
def test_deposit_period_end(cluster):
    """
    - proposal and partially deposit
    - wait for deposit period end and check
      - proposal deleted
      - no refund
    """
    amount1 = cluster.balance(cluster.address("community"))
    rsp = cluster.gov_propose(
        "community",
        "param-change",
        {
            "title": "Increase number of max validators",
            "description": "ditto",
            "changes": [
                {
                    "subspace": "staking",
                    "key": "MaxValidators",
                    "value": 1,
                }
            ],
            "deposit": "5000basecro",
        },
    )
    assert rsp["code"] == 0, rsp
    log = {
        attr["key"]: attr["value"] for attr in rsp["logs"][0]["events"][2]["attributes"]
    }
    assert log["proposal_type"] == "ParameterChange", rsp
    proposal_id = log["proposal_id"]

    proposal = cluster.query_proposal(proposal_id)
    assert proposal["total_deposit"] == [{"denom": "basecro", "amount": "5000"}]

    assert cluster.balance(cluster.address("community")) == amount1 - 5000

    amount2 = cluster.balance(cluster.address("ecosystem"))

    rsp = cluster.gov_deposit("ecosystem", proposal["proposal_id"], "5000basecro")
    assert rsp["code"] == 0, rsp
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["total_deposit"] == [{"denom": "basecro", "amount": "10000"}]

    assert cluster.balance(cluster.address("ecosystem")) == amount2 - 5000

    # wait for deposit period passed
    wait_for_block_time(
        cluster, isoparse(proposal["submit_time"]) + timedelta(seconds=10)
    )

    # proposal deleted
    with pytest.raises(Exception):
        cluster.query_proposal(proposal_id)

    # deposits don't get refunded
    assert cluster.balance(cluster.address("community")) == amount1 - 5000
    assert cluster.balance(cluster.address("ecosystem")) == amount2 - 5000


def test_community_pool_spend(cluster):
    """
    - proposal a community pool spend
    - pass it
    """
    # need at least several blocks to populate community pool
    wait_for_block(cluster, 3)

    amount = int(cluster.distribution_community())
    assert amount > 0, "need positive pool to proceed this test"

    recipient = cluster.address("community")
    old_amount = cluster.balance(recipient)

    rsp = cluster.gov_propose(
        "community",
        "community-pool-spend",
        {
            "title": "Community Pool Spend",
            "description": "Pay me some cro!",
            "recipient": recipient,
            "amount": "%dbasecro" % amount,
            "deposit": "10000001basecro",
        },
    )
    assert rsp["code"] == 0, rsp

    # get proposal_id
    log = {
        attr["key"]: attr["value"] for attr in rsp["logs"][0]["events"][2]["attributes"]
    }
    assert log["proposal_type"] == "CommunityPoolSpend", rsp
    proposal_id = log["proposal_id"]

    # vote
    rsp = cluster.gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp
    rsp = cluster.gov_vote("validator", proposal_id, "yes", i=1)
    assert rsp["code"] == 0, rsp

    # wait for voting period end
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_VOTING_PERIOD", proposal
    wait_for_block_time(
        cluster, isoparse(proposal["voting_end_time"]) + timedelta(seconds=10)
    )

    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal

    assert cluster.balance(recipient) == old_amount + amount
