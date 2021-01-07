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
            Path(__file__).parent / "configs/ledger.yaml",
            worker_index,
            tmp_path_factory,
            quiet=pytestconfig.getoption("supervisord-quiet"),
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
                        {"key": "action", "value": "send"},
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
