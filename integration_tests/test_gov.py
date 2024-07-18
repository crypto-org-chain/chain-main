import json
from datetime import timedelta

import pytest
from dateutil.parser import isoparse

from .utils import get_proposal_id, module_address, wait_for_block, wait_for_block_time

pytestmark = pytest.mark.gov


@pytest.mark.parametrize("vote_option", ["yes", "no", "no_with_veto", "abstain", None])
def test_param_proposal(cluster, vote_option, tmp_path):
    """
    - send proposal to change max_validators
    - all validator vote same option (None means don't vote)
    - check the result
    - check deposit refunded
    """
    params = cluster.staking_params()
    max_validators = params["max_validators"]
    rsp = change_max_validators(cluster, tmp_path, max_validators + 1)
    amount = approve_proposal(cluster, rsp, vote_option)
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


def change_max_validators(cluster, tmp_path, num, deposit="10000000basecro"):
    params = cluster.staking_params()
    params["max_validators"] = num
    proposal = tmp_path / "proposal.json"
    authority = module_address("gov")
    proposal_src = {
        "messages": [
            {
                "@type": "/cosmos.staking.v1beta1.MsgUpdateParams",
                "authority": authority,
                "params": params,
            }
        ],
        "deposit": deposit,
        "title": "Increase number of max validators",
        "summary": "ditto",
    }
    proposal.write_text(json.dumps(proposal_src))
    rsp = cluster.submit_gov_proposal(proposal, from_="community")
    assert rsp["code"] == 0, rsp["raw_log"]
    return rsp


def approve_proposal(
    cluster,
    rsp,
    vote_option="yes",
    msg=",/cosmos.staking.v1beta1.MsgUpdateParams",
):
    proposal_id = get_proposal_id(rsp, msg)
    proposal = cluster.query_proposal(proposal_id)
    if msg == ",/cosmos.gov.v1.MsgExecLegacyContent":
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
    return amount


def test_deposit_period_expires(cluster, tmp_path):
    """
    - proposal and partially deposit
    - wait for deposit period end and check
      - proposal deleted
      - no refund
    """
    amount1 = cluster.balance(cluster.address("community"))
    denom = "basecro"
    deposit_amt = 100000
    deposit = f"{deposit_amt}{denom}"
    rsp = change_max_validators(cluster, tmp_path, 1, deposit)
    proposal_id = get_proposal_id(rsp)
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["total_deposit"] == [{"denom": denom, "amount": f"{deposit_amt}"}]
    assert cluster.balance(cluster.address("community")) == amount1 - deposit_amt
    amount2 = cluster.balance(cluster.address("ecosystem"))
    rsp = cluster.gov_deposit("ecosystem", proposal_id, deposit)
    assert rsp["code"] == 0, rsp["raw_log"]
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["total_deposit"] == [{"denom": denom, "amount": f"{deposit_amt*2}"}]
    assert cluster.balance(cluster.address("ecosystem")) == amount2 - deposit_amt

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


def test_community_pool_spend_proposal(cluster, tmp_path):
    """
    - proposal a community pool spend
    - pass it
    """
    wait_for_block(cluster, 3)
    amount = int(cluster.distribution_community())
    assert amount > 0, "need positive pool to proceed this test"
    recipient = cluster.address("community")
    old_amount = cluster.balance(recipient)
    proposal = tmp_path / "proposal.json"
    authority = module_address("gov")
    msg = "/cosmos.distribution.v1beta1.MsgCommunityPoolSpend"
    amt = [{"denom": "basecro", "amount": f"{amount}"}]
    proposal_src = {
        "messages": [
            {
                "@type": msg,
                "authority": authority,
                "recipient": recipient,
                "amount": amt,
            }
        ],
        "deposit": "10000001basecro",
        "title": "Community Pool Spend",
        "summary": "Pay me some cro!",
    }
    proposal.write_text(json.dumps(proposal_src))
    rsp = cluster.submit_gov_proposal(proposal, from_="community")
    assert rsp["code"] == 0, rsp["raw_log"]
    proposal_id = get_proposal_id(rsp, f",{msg}")

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


def test_change_vote(cluster, tmp_path):
    """
    - submit proposal with deposit
    - vote yes
    - check tally
    - change vote
    - check tally
    """
    rsp = change_max_validators(cluster, tmp_path, 1)
    proposal_id = get_proposal_id(rsp)
    rsp = cluster.gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp["raw_log"]
    voting_power = int(
        cluster.validator(cluster.address("validator", bech="val"))["tokens"]
    )
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


def test_inherit_vote(cluster, tmp_path):
    """
    - submit proposal with deposits
    - A delegate to V
    - V vote Yes
    - check tally: {yes: a + v}
    - A vote No
    - change tally: {yes: v, no: a}
    """
    rsp = change_max_validators(cluster, tmp_path, 1)
    proposal_id = get_proposal_id(rsp)

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


def test_host_enabled(cluster, tmp_path):
    cli = cluster.cosmos_cli()
    p = cluster.cosmos_cli().query_host_params()
    assert p["host_enabled"]
    p["host_enabled"] = False
    proposal = tmp_path / "proposal.json"
    authority = module_address("gov")
    type = "/ibc.applications.interchain_accounts.host.v1.MsgUpdateParams"
    proposal_src = {
        "messages": [
            {
                "@type": type,
                "signer": authority,
                "params": p,
            }
        ],
        "deposit": "10000000basecro",
        "title": "title",
        "summary": "summary",
    }
    proposal.write_text(json.dumps(proposal_src))
    rsp = cluster.submit_gov_proposal(proposal, from_="community")
    assert rsp["code"] == 0, rsp["raw_log"]
    msg = ",/ibc.applications.interchain_accounts.host.v1.MsgUpdateParams"
    approve_proposal(cluster, rsp, msg=msg)
    p = cli.query_host_params()
    assert not p["host_enabled"]
