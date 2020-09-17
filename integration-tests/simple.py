#!/usr/bin/env python
import sys
import json
import asyncio
from pystarport.cli import chaind


async def wait_for_block(height, timeout=10):
    for i in range(timeout):
        try:
            status = json.loads(await chaind('status', home='data/node0'))
        except BaseException as e:
            print(f'get sync status failed: {e}', file=sys.stderr)
        else:
            if int(status['sync_info']['latest_block_height']) >= height:
                break
        await asyncio.sleep(1)
    else:
        print(f'wait for block {height} timeout', file=sys.stderr)


async def main():
    await wait_for_block(1)
    validators = json.loads(await chaind('query', 'staking', 'validators', output='json'))
    assert len(validators) == 2


if __name__ == '__main__':
    asyncio.run(main())
