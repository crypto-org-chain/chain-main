from datetime import timedelta
from pathlib import Path

import pytest
from dateutil.parser import isoparse
from pystarport.ports import rpc_port

from .utils import cluster_fixture, parse_events, wait_for_block_time, wait_for_port


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/staking.yaml",
        worker_index,
        tmp_path_factory,
        quiet=pytestconfig.getoption("supervisord-quiet"),
    )


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
    old_bonded = cluster.staking_pool()
    rsp = cluster.delegate_amount(
        validator1_operator_address, "2basecro", signer1_address
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cluster.staking_pool() == old_bonded + 2
    new_amount = cluster.balance(signer1_address)
    assert old_amount == new_amount + 2


@pytest.mark.slow
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
    old_bonded = cluster.staking_pool()
    rsp = cluster.delegate_amount(
        validator1_operator_address, "3basecro", signer1_address
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = cluster.delegate_amount(
        validator2_operator_address, "4basecro", signer1_address
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cluster.staking_pool() == old_bonded + 7
    assert cluster.balance(signer1_address) == old_amount - 7

    old_unbonded = cluster.staking_pool(bonded=False)
    rsp = cluster.unbond_amount(
        validator2_operator_address, "2basecro", signer1_address
    )
    assert rsp["code"] == 0, rsp
    assert cluster.staking_pool(bonded=False) == old_unbonded + 2

    wait_for_block_time(
        cluster,
        isoparse(parse_events(rsp["logs"])["unbond"]["completion_time"])
        + timedelta(seconds=1),
    )

    assert cluster.balance(signer1_address) == old_amount - 5


def test_staking_redelegate(cluster):
    signer1_address = cluster.address("signer1", i=0)
    validators = cluster.validators()
    validator1_operator_address = validators[0]["operator_address"]
    validator2_operator_address = validators[1]["operator_address"]
    staking_validator1 = cluster.validator(validator1_operator_address, i=0)
    assert validator1_operator_address == staking_validator1["operator_address"]
    staking_validator2 = cluster.validator(validator2_operator_address, i=1)
    assert validator2_operator_address == staking_validator2["operator_address"]
    rsp = cluster.delegate_amount(
        validator1_operator_address, "3basecro", signer1_address
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = cluster.delegate_amount(
        validator2_operator_address, "4basecro", signer1_address
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    delegation_info = cluster.get_delegated_amount(signer1_address)
    old_output = delegation_info["delegation_responses"][0]["balance"]["amount"]
    cluster.redelegate_amount(
        validator1_operator_address,
        validator2_operator_address,
        "2basecro",
        signer1_address,
    )
    delegation_info = cluster.get_delegated_amount(signer1_address)
    output = delegation_info["delegation_responses"][0]["balance"]["amount"]
    assert int(old_output) + 2 == int(output)


def test_join_validator(cluster):
    i = cluster.create_node(moniker="new joined")
    addr = cluster.address("validator", i)
    # transfer 1cro from ecosystem account
    assert cluster.transfer(cluster.address("ecosystem"), addr, "1cro")["code"] == 0
    assert cluster.balance(addr) == 10 ** 8

    # start the node
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node{i}")
    wait_for_port(rpc_port(cluster.base_port(i)))

    count1 = len(cluster.validators())

    # create validator tx
    assert cluster.create_validator("1cro", i)["code"] == 0

    count2 = len(cluster.validators())
    assert count2 == count1 + 1, "new validator should joined successfully"

    val_addr = cluster.address("validator", i, bech="val")
    val = cluster.validator(val_addr)
    assert not val["jailed"]
    assert val["status"] == "BOND_STATUS_BONDED"
    assert val["tokens"] == str(10 ** 8)
    assert val["description"]["moniker"] == "new joined"
    assert val["commission"]["commission_rates"] == {
        "rate": "0.100000000000000000",
        "max_rate": "0.200000000000000000",
        "max_change_rate": "0.010000000000000000",
    }
    assert (
        cluster.edit_validator(i, commission_rate="0.2")["code"] == 13
    ), "commission cannot be changed more than once in 24h"
    assert cluster.edit_validator(i, moniker="awesome node")["code"] == 0
    assert cluster.validator(val_addr)["description"]["moniker"] == "awesome node"


def test_min_self_delegation(cluster):
    """
    - validator unbond min_self_delegation
    - check not in validator set anymore
    """
    assert len(cluster.validators()) == 4, "wrong validator set"

    oper_addr = cluster.address("validator", i=2, bech="val")
    acct_addr = cluster.address("validator", i=2)
    rsp = cluster.unbond_amount(oper_addr, "90000000basecro", acct_addr, i=2)
    assert rsp["code"] == 0, rsp["raw_log"]

    def find_validator():
        return next(
            iter(
                val
                for val in cluster.validators()
                if val["operator_address"] == oper_addr
            )
        )

    assert (
        find_validator()["status"] == "BOND_STATUS_BONDED"
    ), "validator set not changed yet"

    rsp = cluster.unbond_amount(oper_addr, "1basecro", acct_addr, i=2)
    assert rsp["code"] == 0, rsp["raw_log"]
    assert (
        find_validator()["status"] == "BOND_STATUS_UNBONDING"
    ), "validator get removed"
