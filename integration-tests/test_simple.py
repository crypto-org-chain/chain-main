#!/usr/bin/env python
import sys
import json
import asyncio
from pystarport.cli import chaind, CHAIN_ID
import pytest

from utils import wait_for_block, get_account, get_balance, RPC_NODE0

# pytest magic
pytestmark = pytest.mark.asyncio

async def test_simple():
    '''
    - check number of validators
    - check vesting account status
    '''
    await wait_for_block(1)
    validators = json.loads(await chaind('query', 'staking', 'validators', output='json'))
    assert len(validators) == 2

    # check vesting account
    addr = (await get_account('reserve'))['address']
    account = json.loads(await chaind('query', 'auth', 'account', addr, output='json'))
    assert account['@type'] == '/cosmos.vesting.v1beta1.DelayedVestingAccount'
    assert account['base_vesting_account']['original_vesting'] == [{"denom":"basecro","amount":"20000000000"}]


async def test_transfer():
    '''
    check simple transfer tx success
    - send 1cro from community to reserve
    '''
    await wait_for_block(1)

    community_addr = (await get_account('community'))['address']
    reserve_addr = (await get_account('reserve'))['address']

    community_balance = await get_balance(community_addr)
    reserve_balance = await get_balance(reserve_addr)

    tx = json.loads(await chaind('tx', 'bank', 'send', community_addr, reserve_addr, '1cro', '-y',
                 home=f'data/node0', keyring_backend='test', chain_id=CHAIN_ID,
                 node=RPC_NODE0))
    print('transfer tx', tx['txhash'])
    assert tx['logs'] == [{'events': [{'attributes': [{'key': 'action', 'value': 'send'},
                                                      {'key': 'sender',
                                                       'value': community_addr},
                                                      {'key': 'module', 'value': 'bank'}],
                                       'type': 'message'},
                                      {'attributes': [{'key': 'recipient',
                                                       'value': reserve_addr},
                                                      {'key': 'sender',
                                                       'value': community_addr},
                                                      {'key': 'amount',
                                                       'value': '100000000basecro'}],
                                       'type': 'transfer'}],
                           'log': '',
                           'msg_index': 0}]

    assert await get_balance(community_addr) == community_balance - 100000000
    assert await get_balance(reserve_addr) == reserve_balance + 100000000
