import os
import re
import shutil
import socket
import sys
import time
import uuid

import yaml
from dateutil.parser import isoparse
from pystarport import cluster, ledger
from pystarport.ports import rpc_port


def wait_for_block(cli, height, timeout=240):
    for i in range(timeout * 2):
        try:
            status = cli.status()
        except AssertionError as e:
            print(f"get sync status failed: {e}", file=sys.stderr)
        else:
            current_height = int(status["SyncInfo"]["latest_block_height"])
            if current_height >= height:
                break
            print("current block height", current_height)
        time.sleep(0.5)
    else:
        raise TimeoutError(f"wait for block {height} timeout")


def wait_for_new_blocks(cli, n):
    begin_height = int((cli.status())["SyncInfo"]["latest_block_height"])
    while True:
        time.sleep(0.5)
        cur_height = int((cli.status())["SyncInfo"]["latest_block_height"])
        if cur_height - begin_height >= n:
            break


def wait_for_block_time(cli, t):
    print("wait for block time", t)
    while True:
        now = isoparse((cli.status())["SyncInfo"]["latest_block_time"])
        print("block time now:", now)
        if now >= t:
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


def cluster_fixture(
    config_path,
    worker_index,
    tmp_path_factory,
    quiet=False,
    post_init=None,
    enable_cov=None,
):
    """
    init a single devnet
    """
    if enable_cov is None:
        enable_cov = os.environ.get("GITHUB_ACTIONS") == "true"
    data = tmp_path_factory.mktemp("data")
    base_port = gen_base_port(worker_index)
    print("init cluster at", data, ", base port:", base_port)
    cluster.init_cluster(data, config_path, base_port)

    config = yaml.safe_load(open(config_path))
    clis = {}
    for key in config:
        if key == "relayer":
            continue

        chain_id = key
        chain_data = data / chain_id

        if post_init:
            post_init(chain_id, chain_data)

        if enable_cov:
            # replace the first node with the instrumented binary
            ini = chain_data / cluster.SUPERVISOR_CONFIG_FILE
            ini.write_text(
                re.sub(
                    r"^command = (.*/)?chain-maind",
                    "command = chain-maind-inst "
                    "-test.coverprofile=%(here)s/coverage.txt",
                    ini.read_text(),
                    count=1,
                    flags=re.M,
                )
            )
        clis[chain_id] = cluster.ClusterCLI(data, chain_id=chain_id)

    supervisord = cluster.start_cluster(data)
    if not quiet:
        tailer = cluster.start_tail_logs_thread(data)

    try:
        begin = time.time()
        for cli in clis.values():
            # wait for first node rpc port available before start testing
            wait_for_port(rpc_port(cli.config["validators"][0]["base_port"]))
            # wait for the first block generated before start testing
            wait_for_block(cli, 2)

        if len(clis) == 1:
            yield list(clis.values())[0]
        else:
            yield clis

        if enable_cov:
            # wait for server startup complete to generate the coverage report
            duration = time.time() - begin
            if duration < 15:
                time.sleep(15 - duration)
    finally:
        supervisord.terminate()
        supervisord.wait()
        if not quiet:
            tailer.stop()
            tailer.join()

    if enable_cov:
        # collect the coverage results
        shutil.move(str(chain_data / "coverage.txt"), f"coverage.{uuid.uuid1()}.txt")


def get_ledger():
    return ledger.Ledger()


def parse_events(logs):
    return {
        ev["type"]: {attr["key"]: attr["value"] for attr in ev["attributes"]}
        for ev in logs[0]["events"]
    }


_next_unique = 0


def gen_base_port(worker_index):
    global _next_unique
    base_port = 10000 + (worker_index * 10 + _next_unique) * 100
    _next_unique += 1
    return base_port
