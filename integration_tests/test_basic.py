import time

from .utils import wait_for_block


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

    tx = cluster.transfer(community_addr, reserve_addr, "1cro")
    print("transfer tx", tx["txhash"])
    assert tx["logs"] == [
        {
            "events": [
                {
                    "attributes": [
                        {"key": "action", "value": "send"},
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


def test_liquid_supply(cluster):
    """
    Checks the total liquid supply in the system
    """

    liquid_supply = cluster.supply("liquid")["supply"]
    assert liquid_supply[0]["denom"] == "basecro"
    assert liquid_supply[0]["amount"] == "1240000000000"


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
    time.sleep(5 + 3)
    assert cluster.block_height(i=i) >= 5, "syncing"
