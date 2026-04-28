import pytest

from .utils import find_log_event_attrs, wait_for_block

pytestmark = pytest.mark.normal


def test_cluster_has_two_validators_and_vesting_account_is_configured(cluster):
    """
    - check number of validators
    - check vesting account status
    """
    assert len(cluster.validators()) == 2

    # check vesting account
    addr = cluster.address("reserve")
    account = cluster.account(addr)["account"]
    assert account["type"] == "cosmos-sdk/DelayedVestingAccount"
    assert account["value"]["base_vesting_account"]["original_vesting"] == [
        {"denom": "basecro", "amount": "20000000000"}
    ]


def test_transfer_one_cro_from_community_to_reserve_updates_balances_and_tx_counts(cluster):
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

    rsp = cluster.transfer(community_addr, reserve_addr, "1cro")
    ev = find_log_event_attrs(rsp["events"], "message")
    assert ev == {
        "action": "/cosmos.bank.v1beta1.MsgSend",
        "sender": community_addr,
        "module": "bank",
        "msg_index": "0",
    }, ev
    ev = find_log_event_attrs(rsp["events"], "coin_spent")
    assert ev == {
        "spender": community_addr,
        "amount": "100000000basecro",
        "msg_index": "0",
    }, ev
    ev = find_log_event_attrs(rsp["events"], "coin_received")
    assert ev == {
        "receiver": reserve_addr,
        "amount": "100000000basecro",
        "msg_index": "0",
    }, ev
    ev = find_log_event_attrs(rsp["events"], "transfer")
    assert ev == {
        "recipient": reserve_addr,
        "sender": community_addr,
        "amount": "100000000basecro",
        "msg_index": "0",
    }, ev
    ev = find_log_event_attrs(
        rsp["events"],
        "message",
        lambda attrs: "action" not in attrs,
    )
    assert ev == {
        "sender": community_addr,
        "msg_index": "0",
    }, ev

    assert cluster.balance(community_addr) == community_balance - 100000000
    assert cluster.balance(reserve_addr) == reserve_balance + 100000000
    updated_community_addr_tx_count = len(cluster.query_all_txs(community_addr)["txs"])
    assert updated_community_addr_tx_count == initial_community_addr_tx_count + 1
    updated_reserve_addr_tx_count = len(cluster.query_all_txs(reserve_addr)["txs"])
    assert updated_reserve_addr_tx_count == initial_reserve_addr_tx_count + 1


def test_liquid_supply_returns_correct_total_excluding_vesting(cluster):
    """
    Checks the total liquid supply in the system
    """

    liquid_supply = cluster.supply("liquid")["supply"]
    assert liquid_supply[0]["denom"] == "basecro"
    # # sum of all coins except the one with vesting time under accounts in config yaml
    assert liquid_supply[0]["amount"] == "1640000000000"
