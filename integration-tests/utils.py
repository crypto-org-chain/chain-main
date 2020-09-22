import sys
import json
import asyncio

from pystarport.cli import chaind, BASE_PORT
from pystarport.ports import rpc_port

RPC_NODE0 = 'tcp://localhost:%d' % rpc_port(BASE_PORT, 0)


async def wait_for_block(height, timeout=10):
    for i in range(timeout):
        try:
            status = json.loads(await chaind('status', node=RPC_NODE0))
        except BaseException as e:
            print(f'get sync status failed: {e}', file=sys.stderr)
        else:
            if int(status['sync_info']['latest_block_height']) >= height:
                break
        await asyncio.sleep(1)
    else:
        print(f'wait for block {height} timeout', file=sys.stderr)


async def get_account(name, i=0):
    return json.loads(await chaind('keys', 'show', name, home=f'data/node{i}', keyring_backend='test', output='json'))


async def get_balance(addr):
    coin = json.loads(await chaind('query', 'bank', 'balances', addr, output='json', node=RPC_NODE0))['balances'][0]
    assert coin['denom'] == 'basecro'
    return int(coin['amount'])
