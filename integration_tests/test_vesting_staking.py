from pathlib import Path

import pytest

from .utils import cluster_fixture

pytestmark = pytest.mark.normal


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/staking.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


# one more test for the vesting account bug
# that one can delegate twice with fee + redelegate
def test_staking_vesting_redelegate(cluster):
    community_addr = cluster.address("community")
    reserve_addr = cluster.address("reserve")
    # for the fee payment
    cluster.transfer(community_addr, reserve_addr, "10000basecro")

    signer1_address = cluster.address("reserve", i=0)
    validators = cluster.validators()
    validator1_operator_address = validators[0]["operator_address"]
    validator2_operator_address = validators[1]["operator_address"]
    staking_validator1 = cluster.validator(validator1_operator_address, i=0)
    assert validator1_operator_address == staking_validator1["operator_address"]
    staking_validator2 = cluster.validator(validator2_operator_address, i=1)
    assert validator2_operator_address == staking_validator2["operator_address"]
    old_bonded = cluster.staking_pool()
    rsp = cluster.delegate_amount(
        validator1_operator_address,
        "2009999498basecro",
        signer1_address,
        0,
        "0.025basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cluster.staking_pool() == old_bonded + 2009999498
    rsp = cluster.delegate_amount(
        validator2_operator_address, "1basecro", signer1_address, 0, "0.025basecro"
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cluster.staking_pool() == old_bonded + 2009999499
    # delegation_info = cluster.get_delegated_amount(signer1_address)
    # old_output = delegation_info["delegation_responses"][0]["balance"]["amount"]
    cluster.redelegate_amount(
        validator1_operator_address,
        validator2_operator_address,
        "2basecro",
        signer1_address,
    )
    # delegation_info = cluster.get_delegated_amount(signer1_address)
    # output = delegation_info["delegation_responses"][0]["balance"]["amount"]
    # assert int(old_output) + 2 == int(output)
    assert cluster.staking_pool() == old_bonded + 2009999499
    account = cluster.account(signer1_address)
    assert account["@type"] == "/cosmos.vesting.v1beta1.DelayedVestingAccount"
    assert account["base_vesting_account"]["original_vesting"] == [
        {"denom": "basecro", "amount": "20000000000"}
    ]
