import time
from pathlib import Path

import pytest

from .utils import cluster_fixture, get_ledger

pytestmark = pytest.mark.ledger


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    ledger = get_ledger()
    ledger.start()
    try:
        yield from cluster_fixture(
            Path(__file__).parent / "configs/ledger.jsonnet",
            worker_index,
            tmp_path_factory.mktemp("data"),
        )
    finally:
        ledger.stop()


def test_ledger_transfer(cluster):
    """
    check simple transfer tx success
    - send 1cro from hw to reserve
    """
    reserve_addr = cluster.address("reserve")
    hw_addr = cluster.address("hw")

    reserve_balance = cluster.balance(reserve_addr)
    hw_balance = cluster.balance(hw_addr)

    tx = cluster.transfer_from_ledger("hw", reserve_addr, "1cro")
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
                        {"key": "spender", "value": hw_addr},
                        {"key": "amount", "value": "100000000basecro"},
                    ],
                    "type": "coin_spent",
                },
                {
                    "attributes": [
                        {"key": "action", "value": "/cosmos.bank.v1beta1.MsgSend"},
                        {"key": "sender", "value": hw_addr},
                        {"key": "module", "value": "bank"},
                    ],
                    "type": "message",
                },
                {
                    "attributes": [
                        {"key": "recipient", "value": reserve_addr},
                        {"key": "sender", "value": hw_addr},
                        {"key": "amount", "value": "100000000basecro"},
                    ],
                    "type": "transfer",
                },
            ],
            "log": "",
            "msg_index": 0,
        }
    ]

    assert cluster.balance(hw_addr) == hw_balance - 100000000
    assert cluster.balance(reserve_addr) == reserve_balance + 100000000


def test_wallet_name_for_ledger(cluster):
    def check_account(name):
        cluster.create_account_ledger(name, 0)
        address = cluster.address(name)
        assert len(address) > 0
        cluster.delete_account(name)
        time.sleep(1)

    cluster.delete_account("hw")
    names = [
        "normalwallet",
        "abc 1",
        # there should be a `\` before `&` and `)` or the terminal will
        # trade them as one part of command
        r"\&a\)bcd*^",
        "钱對중ガジÑá",
        # a very long name
        "this_is_a_very_long_long_long_long_long_long_\
long_long_long_long_long_long_long_long_name",
        # a very complex name
        "1 abc &abcd*^ 钱對중ガジÑá  long_long_long_long_long_\
long_long_long_long_long_long_long_name",
    ]
    for name in names:
        print("name: ", name)
        check_account(name)
