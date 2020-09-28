import asyncio
import sys


async def wait_for_block(cli, height, timeout=60):
    for i in range(timeout):
        try:
            status = await cli.status()
        except BaseException as e:
            print(f"get sync status failed: {e}", file=sys.stderr)
        else:
            if int(status["sync_info"]["latest_block_height"]) >= height:
                break
        await asyncio.sleep(1)
    else:
        print(f"wait for block {height} timeout", file=sys.stderr)
