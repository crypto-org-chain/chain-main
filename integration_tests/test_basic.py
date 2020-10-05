from pathlib import Path

import pytest
import yaml

from .multisig import test_multi_signature  # noqa
from .utils import cluster_fixture

# http://doc.pytest.org/en/latest/example/markers.html#marking-whole-classes-or-modules
pytestmark = pytest.mark.asyncio


@pytest.fixture(scope="module")
async def cluster():
    async for v in cluster_fixture(
        yaml.safe_load(open(Path(__file__).parent / "configs/cluster.yml")), 26650
    ):
        yield v


async def test_simple(cluster):
    """
    - check number of validators
    - check vesting account status
    """
    assert len(await cluster.cli.validators()) == 2

    # check vesting account
    addr = await cluster.cli.address("reserve")
    account = await cluster.cli.account(addr)
    assert account["@type"] == "/cosmos.vesting.v1beta1.DelayedVestingAccount"
    assert account["base_vesting_account"]["original_vesting"] == [
        {"denom": "basecro", "amount": "20000000000"}
    ]


async def test_transfer(cluster):
    """
    check simple transfer tx success
    - send 1cro from community to reserve
    """
    community_addr = await cluster.cli.address("community")
    reserve_addr = await cluster.cli.address("reserve")

    community_balance = await cluster.cli.balance(community_addr)
    reserve_balance = await cluster.cli.balance(reserve_addr)

    tx = await cluster.cli.transfer(community_addr, reserve_addr, "1cro")
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

    assert await cluster.cli.balance(community_addr) == community_balance - 100000000
    assert await cluster.cli.balance(reserve_addr) == reserve_balance + 100000000
