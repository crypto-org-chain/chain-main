import asyncio
import socket
import sys
import tempfile
import time
from pathlib import Path

from dateutil.parser import isoparse

from pystarport.cluster import Cluster
from pystarport.ports import rpc_port


async def wait_for_block(cli, height, timeout=60):
    for i in range(timeout * 2):
        try:
            status = await cli.status()
        except BaseException as e:
            print(f"get sync status failed: {e}", file=sys.stderr)
        else:
            if int(status["sync_info"]["latest_block_height"]) >= height:
                break
        await asyncio.sleep(0.5)
    else:
        print(f"wait for block {height} timeout", file=sys.stderr)


async def wait_for_new_blocks(cli, n):
    begin_height = int((await cli.status())["sync_info"]["latest_block_height"])
    while True:
        await asyncio.sleep(0.5)
        cur_height = int((await cli.status())["sync_info"]["latest_block_height"])
        if cur_height - begin_height >= n:
            break


async def wait_for_block_time(cli, time):
    while True:
        if isoparse((await cli.status())["sync_info"]["latest_block_time"]) >= time:
            break
        await asyncio.sleep(0.5)


async def wait_for_port(port, host="127.0.0.1", timeout=40.0):
    start_time = time.perf_counter()
    while True:
        try:
            with socket.create_connection((host, port), timeout=timeout):
                break
        except OSError as ex:
            await asyncio.sleep(0.1)
            if time.perf_counter() - start_time >= timeout:
                raise TimeoutError(
                    "Waited too long for the port {} on host {} to start accepting "
                    "connections.".format(port, host)
                ) from ex


async def cluster_fixture(config, base_port):
    with tempfile.TemporaryDirectory(suffix=config["chain_id"]) as tmpdir:
        cluster = Cluster(config, Path(tmpdir), base_port)
        await cluster.init()
        await cluster.start()
        # wait for first node rpc port available before start testing
        await wait_for_port(rpc_port(cluster.base_port, 0))
        # wait for first block generated before start testing
        await wait_for_block(cluster.cli, 1)

        yield cluster

        await cluster.terminate()
