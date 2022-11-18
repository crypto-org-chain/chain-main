import json

import pytest

pytestmark = pytest.mark.normal


def test_group(cluster, tmp_path):
    cli = cluster.cosmos_cli()

    admin = cluster.address("community")
    signer1 = cluster.address("signer1")
    signer2 = cluster.address("signer2")

    # create group
    members_file = tmp_path / "members.json"
    members = {
        "members": [
            {"address": admin, "weight": "1", "metadata": "admin"},
            {"address": signer1, "weight": "1", "metadata": "signer1"},
            {"address": signer2, "weight": "1", "metadata": "signer2"},
        ]
    }

    with open(members_file, "w") as f:
        json.dump(members, f)

    rsp = json.loads(
        cli.raw(
            "tx",
            "group",
            "create-group",
            admin,
            "admin",
            members_file,
            "-y",
            _from="community",
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
            chain_id=cli.chain_id,
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # Get group id from events
    group_id = json.loads(
        json.loads(rsp["raw_log"])[0]["events"][0]["attributes"][0]["value"]
    )

    # create group policy
    policy_file = tmp_path / "policy.json"
    policy = {
        "@type": "/cosmos.group.v1.ThresholdDecisionPolicy",
        "threshold": "2",
        "windows": {"voting_period": "120h", "min_execution_period": "0s"},
    }

    with open(policy_file, "w") as f:
        json.dump(policy, f)

    rsp = json.loads(
        cli.raw(
            "tx",
            "group",
            "create-group-policy",
            admin,
            group_id,
            "group-policy",
            policy_file,
            "-y",
            _from="community",
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
            chain_id=cli.chain_id,
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    group_policy_address = json.loads(
        json.loads(rsp["raw_log"])[0]["events"][0]["attributes"][0]["value"]
    )

    # submit a proposal
    proposal_file = tmp_path / "proposal.json"
    proposal = {
        "group_policy_address": group_policy_address,
        "messages": [
            {
                "@type": "/cosmos.bank.v1beta1.MsgSend",
                "from_address": group_policy_address,
                "to_address": signer1,
                "amount": [{"denom": "basecro", "amount": "100000000"}],
            }
        ],
        "metadata": "proposal",
        "proposers": [admin],
    }

    with open(proposal_file, "w") as f:
        json.dump(proposal, f)

    rsp = json.loads(
        cli.raw(
            "tx",
            "group",
            "submit-proposal",
            proposal_file,
            "-y",
            _from="community",
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
            chain_id=cli.chain_id,
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    proposal_id = json.loads(
        json.loads(rsp["raw_log"])[0]["events"][0]["attributes"][0]["value"]
    )

    # vote on proposal
    rsp = json.loads(
        cli.raw(
            "tx",
            "group",
            "vote",
            proposal_id,
            signer1,
            "VOTE_OPTION_YES",
            "vote-signer1-yes",
            "-y",
            _from="signer1",
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
            chain_id=cli.chain_id,
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    rsp = json.loads(
        cli.raw(
            "tx",
            "group",
            "vote",
            proposal_id,
            signer2,
            "VOTE_OPTION_YES",
            "vote-signer2-yes",
            "-y",
            _from="signer2",
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
            chain_id=cli.chain_id,
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # query proposal votes
    rsp = json.loads(
        cli.raw(
            "query",
            "group",
            "votes-by-proposal",
            proposal_id,
            home=cli.data_dir,
            node=cli.node_rpc,
            chain_id=cli.chain_id,
        )
    )

    assert len(rsp["votes"]) == 2, rsp

    # transfer some amount to group policy address
    cluster.transfer(admin, group_policy_address, "1cro")

    group_policy_balance = cluster.balance(group_policy_address)
    signer1_balance = cluster.balance(signer1)

    # execute proposal
    rsp = json.loads(
        cli.raw(
            "tx",
            "group",
            "exec",
            proposal_id,
            "-y",
            _from="community",
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
            chain_id=cli.chain_id,
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # check if the proposal executed successfully
    assert (
        json.loads(
            list(
                filter(
                    lambda attribute: attribute["key"] == "result",
                    json.loads(rsp["raw_log"])[0]["events"][2]["attributes"],
                )
            )[0]["value"],
        )
        == "PROPOSAL_EXECUTOR_RESULT_SUCCESS"
    ), rsp["raw_log"]

    assert group_policy_balance - 100000000 == cluster.balance(group_policy_address)
    assert signer1_balance + 100000000 == cluster.balance(signer1)
