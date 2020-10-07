import socket
import sys
import tempfile
import time
from pathlib import Path

import yaml
from dateutil.parser import isoparse

from pystarport import cluster
from pystarport.ports import rpc_port


def wait_for_block(cli, height, timeout=60):
    for i in range(timeout * 2):
        try:
            status = cli.status()
        except BaseException as e:
            print(f"get sync status failed: {e}", file=sys.stderr)
        else:
            if int(status["sync_info"]["latest_block_height"]) >= height:
                break
        time.sleep(0.5)
    else:
        print(f"wait for block {height} timeout", file=sys.stderr)


def wait_for_new_blocks(cli, n):
    begin_height = int((cli.status())["sync_info"]["latest_block_height"])
    while True:
        time.sleep(0.5)
        cur_height = int((cli.status())["sync_info"]["latest_block_height"])
        if cur_height - begin_height >= n:
            break


def wait_for_block_time(cli, t):
    while True:
        if isoparse((cli.status())["sync_info"]["latest_block_time"]) >= t:
            break
        time.sleep(0.5)


def wait_for_port(port, host="127.0.0.1", timeout=40.0):
    start_time = time.perf_counter()
    while True:
        try:
            with socket.create_connection((host, port), timeout=timeout):
                break
        except OSError as ex:
            time.sleep(0.1)
            if time.perf_counter() - start_time >= timeout:
                raise TimeoutError(
                    "Waited too long for the port {} on host {} to start accepting "
                    "connections.".format(port, host)
                ) from ex


def cluster_fixture(config_path, base_port, quiet=False):
    config = yaml.safe_load(open(config_path))
    with tempfile.TemporaryDirectory(suffix=config["chain_id"]) as tmpdir:
        print("init cluster at", tmpdir, ", base port:", base_port)
        data = Path(tmpdir)
        cluster.init_cluster(data, config, base_port)
        supervisord = cluster.start_cluster(data, quiet=quiet)
        # wait for first node rpc port available before start testing
        wait_for_port(rpc_port(base_port, 0))
        cli = cluster.ClusterCLI(data)
        # wait for first block generated before start testing
        wait_for_block(cli, 1)

        yield cli

        supervisord.terminate()
        supervisord.wait()
