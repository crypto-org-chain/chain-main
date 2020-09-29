#!/usr/bin/env python
import json

import pytest

from .utils import wait_for_block

# http://doc.pytest.org/en/latest/example/markers.html#marking-whole-classes-or-modules
pytestmark = pytest.mark.asyncio


async def test_simple(cluster):
    """
    - check number of validators
    - check vesting account status
    """
    await wait_for_block(cluster.cli, 1)
    validators = json.loads(
        await cluster.cli("query", "staking", "validators", output="json")
    )
    assert len(validators) == 2

    # check vesting account
    addr = (await cluster.cli.get_account("reserve"))["address"]
    account = json.loads(
        await cluster.cli("query", "auth", "account", addr, output="json")
    )
    assert account["@type"] == "/cosmos.vesting.v1beta1.DelayedVestingAccount"
    assert account["base_vesting_account"]["original_vesting"] == [
        {"denom": "basecro", "amount": "20000000000"}
    ]


async def test_transfer(cluster):
    """
    check simple transfer tx success
    - send 1cro from community to reserve
    """
    await wait_for_block(cluster.cli, 1)

    community_addr = (await cluster.cli.get_account("community"))["address"]
    reserve_addr = (await cluster.cli.get_account("reserve"))["address"]

    community_balance = await cluster.cli.query_balance(community_addr)
    reserve_balance = await cluster.cli.query_balance(reserve_addr)

    tx = json.loads(await cluster.cli.transfer(community_addr, reserve_addr, "1cro"))
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

    assert (
        await cluster.cli.query_balance(community_addr) == community_balance - 100000000
    )
    assert await cluster.cli.query_balance(reserve_addr) == reserve_balance + 100000000
