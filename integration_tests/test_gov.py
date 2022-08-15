from datetime import timedelta

import pytest
from dateutil.parser import isoparse

from .utils import parse_events, wait_for_block, wait_for_block_time

pytestmark = pytest.mark.gov


@pytest.mark.parametrize("vote_option", ["yes", "no", "no_with_veto", "abstain", None])
def test_param_proposal(cluster, vote_option):
    """
    - send proposal to change max_validators
    - all validator vote same option (None means don't vote)
    - check the result
    - check deposit refunded
    """
    max_validators = cluster.staking_params()["max_validators"]

    rsp = cluster.gov_propose_legacy(
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
    assert rsp["code"] == 0, rsp["raw_log"]

    # get proposal_id
    ev = parse_events(rsp["logs"])["submit_proposal"]
    assert ev["proposal_messages"] == ",/cosmos.gov.v1.MsgExecLegacyContent", rsp
    proposal_id = ev["proposal_id"]

    proposal = cluster.query_proposal(proposal_id)
    assert proposal["messages"][0]["content"]["changes"] == [
        {
            "subspace": "staking",
            "key": "MaxValidators",
            "value": str(max_validators + 1),
        }
    ], proposal
    assert proposal["status"] == "PROPOSAL_STATUS_DEPOSIT_PERIOD", proposal

    amount = cluster.balance(cluster.address("ecosystem"))
    rsp = cluster.gov_deposit("ecosystem", proposal_id, "1cro")
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cluster.balance(cluster.address("ecosystem")) == amount - 100000000

    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_VOTING_PERIOD", proposal

    if vote_option is not None:
        rsp = cluster.gov_vote("validator", proposal_id, vote_option)
        assert rsp["code"] == 0, rsp["raw_log"]
        rsp = cluster.gov_vote("validator", proposal_id, vote_option, i=1)
        assert rsp["code"] == 0, rsp["raw_log"]
        assert (
            int(cluster.query_tally(proposal_id, i=1)[vote_option + "_count"])
            == cluster.staking_pool()
        ), "all voted"
    else:
        assert cluster.query_tally(proposal_id) == {
            "yes_count": "0",
            "no_count": "0",
            "abstain_count": "0",
            "no_with_veto_count": "0",
        }

    wait_for_block_time(
        cluster, isoparse(proposal["voting_end_time"]) + timedelta(seconds=5)
    )

    proposal = cluster.query_proposal(proposal_id)
    if vote_option == "yes":
        assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal
    else:
        assert proposal["status"] == "PROPOSAL_STATUS_REJECTED", proposal

    new_max_validators = cluster.staking_params()["max_validators"]
    if vote_option == "yes":
        assert new_max_validators == max_validators + 1
    else:
        assert new_max_validators == max_validators

    if vote_option == "no_with_veto":
        # deposit only get burnt for vetoed proposal
        assert cluster.balance(cluster.address("ecosystem")) == amount - 100000000
    else:
        # refunded, no matter passed or rejected
        assert cluster.balance(cluster.address("ecosystem")) == amount


def test_deposit_period_expires(cluster):
    """
    - proposal and partially deposit
    - wait for deposit period end and check
      - proposal deleted
      - no refund
    """
    amount1 = cluster.balance(cluster.address("community"))
    rsp = cluster.gov_propose_legacy(
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
    assert rsp["code"] == 0, rsp["raw_log"]
    ev = parse_events(rsp["logs"])["submit_proposal"]
    assert ev["proposal_messages"] == ",/cosmos.gov.v1.MsgExecLegacyContent", rsp
    proposal_id = ev["proposal_id"]

    proposal = cluster.query_proposal(proposal_id)
    assert proposal["total_deposit"] == [{"denom": "basecro", "amount": "5000"}]

    assert cluster.balance(cluster.address("community")) == amount1 - 5000

    amount2 = cluster.balance(cluster.address("ecosystem"))

    rsp = cluster.gov_deposit("ecosystem", proposal_id, "5000basecro")
    assert rsp["code"] == 0, rsp["raw_log"]
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

    # deposits get refunded
    assert cluster.balance(cluster.address("community")) == amount1
    assert cluster.balance(cluster.address("ecosystem")) == amount2


def test_community_pool_spend_proposal(cluster):
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

    rsp = cluster.gov_propose_legacy(
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
    assert rsp["code"] == 0, rsp["raw_log"]

    # get proposal_id
    ev = parse_events(rsp["logs"])["submit_proposal"]
    assert ev["proposal_messages"] == ",/cosmos.gov.v1.MsgExecLegacyContent", rsp
    proposal_id = ev["proposal_id"]

    # vote
    rsp = cluster.gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = cluster.gov_vote("validator", proposal_id, "yes", i=1)
    assert rsp["code"] == 0, rsp["raw_log"]

    # wait for voting period end
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_VOTING_PERIOD", proposal
    wait_for_block_time(
        cluster, isoparse(proposal["voting_end_time"]) + timedelta(seconds=1)
    )

    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal

    assert cluster.balance(recipient) == old_amount + amount


def test_change_vote(cluster):
    """
    - submit proposal with deposit
    - vote yes
    - check tally
    - change vote
    - check tally
    """
    rsp = cluster.gov_propose_legacy(
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
            "deposit": "10000000basecro",
        },
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    voting_power = int(
        cluster.validator(cluster.address("validator", bech="val"))["tokens"]
    )

    proposal_id = parse_events(rsp["logs"])["submit_proposal"]["proposal_id"]

    rsp = cluster.gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp["raw_log"]

    cluster.query_tally(proposal_id) == {
        "yes_count": str(voting_power),
        "no_count": "0",
        "abstain_count": "0",
        "no_with_veto_count": "0",
    }

    # change vote to no
    rsp = cluster.gov_vote("validator", proposal_id, "no")
    assert rsp["code"] == 0, rsp["raw_log"]

    cluster.query_tally(proposal_id) == {
        "no_count": str(voting_power),
        "yes_count": "0",
        "abstain_count": "0",
        "no_with_veto_count": "0",
    }


def test_inherit_vote(cluster):
    """
    - submit proposal with deposits
    - A delegate to V
    - V vote Yes
    - check tally: {yes: a + v}
    - A vote No
    - change tally: {yes: v, no: a}
    """
    rsp = cluster.gov_propose_legacy(
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
            "deposit": "10000000basecro",
        },
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    proposal_id = parse_events(rsp["logs"])["submit_proposal"]["proposal_id"]

    # non-validator voter
    voter1 = cluster.address("community")
    cluster.delegate_amount(
        cluster.address("validator", bech="val"), "10basecro", voter1
    )

    rsp = cluster.gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cluster.query_tally(proposal_id) == {
        "yes_count": "1000000010",
        "no_count": "0",
        "abstain_count": "0",
        "no_with_veto_count": "0",
    }

    rsp = cluster.gov_vote(voter1, proposal_id, "no")
    assert rsp["code"] == 0, rsp["raw_log"]

    assert cluster.query_tally(proposal_id) == {
        "yes_count": "1000000000",
        "no_count": "10",
        "abstain_count": "0",
        "no_with_veto_count": "0",
    }
