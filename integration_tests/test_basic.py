import pytest

from .utils import BASECRO_DENOM, query_command, wait_for_block

pytestmark = pytest.mark.normal


def test_simple(cluster):
    """
    - check number of validators
    - check vesting account status
    """
    assert len(cluster.validators()) == 2

    # check vesting account
    addr = cluster.address("reserve")
    account = cluster.account(addr)
    assert account["@type"] == "/cosmos.vesting.v1beta1.DelayedVestingAccount"
    assert account["base_vesting_account"]["original_vesting"] == [
        {"denom": "basecro", "amount": "20000000000"}
    ]


def test_transfer(cluster):
    """
    check simple transfer tx success
    - send 1cro from community to reserve
    """
    community_addr = cluster.address("community")
    reserve_addr = cluster.address("reserve")
    community_balance = cluster.balance(community_addr)
    reserve_balance = cluster.balance(reserve_addr)
    initial_community_addr_tx_count = len(cluster.query_all_txs(community_addr)["txs"])
    initial_reserve_addr_tx_count = len(cluster.query_all_txs(reserve_addr)["txs"])

    tx = cluster.transfer(community_addr, reserve_addr, "1cro")
    print("transfer tx", tx["txhash"])
    assert tx["logs"] == [
        {
            "events": [
                {
                    "attributes": [
                        {"key": "receiver", "value": reserve_addr},
                        {"key": "amount", "value": "100000000basecro"},
                    ],
                    "type": "coin_received",
                },
                {
                    "attributes": [
                        {"key": "spender", "value": community_addr},
                        {"key": "amount", "value": "100000000basecro"},
                    ],
                    "type": "coin_spent",
                },
                {
                    "attributes": [
                        {"key": "action", "value": "/cosmos.bank.v1beta1.MsgSend"},
                        {"key": "sender", "value": community_addr},
                        {"key": "module", "value": "bank"},
                    ],
                    "type": "message",
                },
                {
                    "attributes": [
                        {"key": "recipient", "value": reserve_addr},
                        {"key": "sender", "value": community_addr},
                        {"key": "amount", "value": "100000000basecro"},
                    ],
                    "type": "transfer",
                },
            ],
            "log": "",
            "msg_index": 0,
        }
    ]

    assert cluster.balance(community_addr) == community_balance - 100000000
    assert cluster.balance(reserve_addr) == reserve_balance + 100000000
    updated_community_addr_tx_count = len(cluster.query_all_txs(community_addr)["txs"])
    assert updated_community_addr_tx_count == initial_community_addr_tx_count + 1
    updated_reserve_addr_tx_count = len(cluster.query_all_txs(reserve_addr)["txs"])
    assert updated_reserve_addr_tx_count == initial_reserve_addr_tx_count + 1


def test_liquid_supply(cluster):
    """
    Checks the total liquid supply in the system
    """

    # sum of all coins except the one with vesting time under accounts in config yaml
    liquid_supply_exclude_staking = 1640000000000
    # sum of all coins under validators in config yaml
    staking_pool_gentx = 2000000000
    staking_pool = 0
    validators = cluster.validators()
    for v in validators:
        delegations_to = query_command(
            cluster, "staking", "delegations-to", v["operator_address"]
        )
        staking_pool += sum(
            [
                int(i["balance"]["amount"])
                for i in delegations_to["delegation_responses"]
            ]
        )

    assert cluster.supply("liquid")["supply"][0] == {
        "denom": BASECRO_DENOM,
        # minus delegation amount in gentx from calculated staking pool
        "amount": str(
            liquid_supply_exclude_staking - (staking_pool - staking_pool_gentx)
        ),
    }


def test_statesync(cluster):
    """
    - create a new node with statesync enabled
    - check it works
    """
    # wait the first snapshot to be created
    wait_for_block(cluster, 10)

    # add a statesync node
    i = cluster.create_node(moniker="statesync", statesync=True)
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node{i}")

    # discovery_time is set to 5 seconds, add extra seconds for processing
    wait_for_block(cluster.cosmos_cli(i), 10)
    print("succesfully syncing")
