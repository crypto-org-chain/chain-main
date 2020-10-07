from .utils import wait_for_new_blocks


def test_staking_delegate(cluster):
    signer1_address = cluster.address("signer1", i=0)
    validators = cluster.validators()
    validator1_operator_address = validators[0]["operator_address"]
    validator2_operator_address = validators[1]["operator_address"]
    staking_validator1 = cluster.validator(validator1_operator_address, i=0)
    assert validator1_operator_address == staking_validator1["operator_address"]
    staking_validator2 = cluster.validator(validator2_operator_address, i=1)
    assert validator2_operator_address == staking_validator2["operator_address"]
    old_amount = cluster.balance(signer1_address)
    cluster.delegate_amount(validator1_operator_address, "2basecro", signer1_address)
    wait_for_new_blocks(cluster, 2)
    new_amount = cluster.balance(signer1_address)
    assert old_amount == new_amount + 2


def test_staking_unbond(cluster):
    signer1_address = cluster.address("signer1", i=0)
    validators = cluster.validators()
    validator1_operator_address = validators[0]["operator_address"]
    validator2_operator_address = validators[1]["operator_address"]
    staking_validator1 = cluster.validator(validator1_operator_address, i=0)
    assert validator1_operator_address == staking_validator1["operator_address"]
    staking_validator2 = cluster.validator(validator2_operator_address, i=1)
    assert validator2_operator_address == staking_validator2["operator_address"]
    old_amount = cluster.balance(signer1_address)
    cluster.delegate_amount(validator1_operator_address, "3basecro", signer1_address)
    cluster.delegate_amount(validator2_operator_address, "4basecro", signer1_address)
    wait_for_new_blocks(cluster, 1)
    new_amount = cluster.balance(signer1_address)
    assert old_amount == new_amount + 7
    cluster.unbond_amount(validator2_operator_address, "2basecro", signer1_address)
    wait_for_new_blocks(cluster, 15)
    new_amount_after_unbond = cluster.balance(signer1_address)
    assert old_amount == new_amount_after_unbond + 5


def test_staking_redelegate(cluster):
    signer1_address = cluster.address("signer1", i=0)
    validators = cluster.validators()
    validator1_operator_address = validators[0]["operator_address"]
    validator2_operator_address = validators[1]["operator_address"]
    staking_validator1 = cluster.validator(validator1_operator_address, i=0)
    assert validator1_operator_address == staking_validator1["operator_address"]
    staking_validator2 = cluster.validator(validator2_operator_address, i=1)
    assert validator2_operator_address == staking_validator2["operator_address"]
    cluster.delegate_amount(validator1_operator_address, "3basecro", signer1_address)
    cluster.delegate_amount(validator2_operator_address, "4basecro", signer1_address)
    delegation_info = cluster.get_delegated_amount(signer1_address)
    old_output = delegation_info["delegation_responses"][0]["balance"]["amount"]
    wait_for_new_blocks(cluster, 1)
    cluster.redelegate_amount(
        validator1_operator_address,
        validator2_operator_address,
        "2basecro",
        signer1_address,
    )
    wait_for_new_blocks(cluster, 12)
    delegation_info = cluster.get_delegated_amount(signer1_address)
    output = delegation_info["delegation_responses"][0]["balance"]["amount"]
    assert int(old_output) + 2 == int(output)
