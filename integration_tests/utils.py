import datetime
import json
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

#################
# CONSTANTS
# Reponse code
SUCCESS_CODE = 0

# Denomination
CRO_DENOM = "cro"
BASECRO_DENOM = "basecro"

# Command Line Options
GENERATE_ONLY = "--generate-only"

# Authorization Type
AUTHORIZATION_SEND = "send"
AUTHORIZATION_GENERIC = "generic"
AUTHORIZATION_DELEGATE = "delegate"
AUTHORIZATION_UNBOND = "unbond"
AUTHORIZATION_REDELEGATE = "redelegate"

# tx broadcasting mode
# Wait for the tx to pass/fail CheckTx
SYNC_BROADCASTING = "sync"
# (the default) Don't wait for pass/fail CheckTx; send and return tx immediately
ASYNC_BROADCASTING = "async"
# Wait for the tx to pass/fail CheckTx, DeliverTx, and be committed in a block
BLOCK_BROADCASTING = "block"

# Msg Type URL
SEND_MSG_TYPE_URL = "/cosmos.bank.v1beta1.MsgSend"
DELEGATE_MSG_TYPE_URL = "/cosmos.staking.v1beta1.MsgDelegate"
UNBOND_MSG_TYPE_URL = "/cosmos.staking.v1beta1.MsgUndelegate"
REDELEGATE_MSG_TYPE_URL = "/cosmos.staking.v1beta1.MsgBeginRedelegate"
WITHDRAW_DELEGATOR_REWARD_TYPE_URL = (
    "/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward"
)

# Module
STAKING = "staking"
AUTHZ = "authz"

# Querying commands for staking module
DELEGATION = "delegation"
DISTRIBUTION = "distribution"

# Querying commands for authz module
GRANTS = "grants"

# QUerying commands for distribution module
REWARDS = "rewards"


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
    data,
    quiet=False,
    post_init=None,
    enable_cov=None,
    cmd=None,
):
    """
    init a single devnet
    """
    if enable_cov is None:
        enable_cov = os.environ.get("GITHUB_ACTIONS") == "true"
    base_port = gen_base_port(worker_index)
    print("init cluster at", data, ", base port:", base_port)
    cluster.init_cluster(data, config_path, base_port, cmd=cmd)

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
        try:
            shutil.move(
                str(chain_data / "coverage.txt"), f"coverage.{uuid.uuid1()}.txt"
            )
        except FileNotFoundError:
            ts = time.time()
            st = datetime.datetime.fromtimestamp(ts).strftime("%Y-%m-%d %H:%M:%S")
            print(st + " FAILED TO FIND COVERAGE")
            print(os.listdir(chain_data))
            data = [
                (int(p), c)
                for p, c in [
                    x.rstrip("\n").split(" ", 1)
                    for x in os.popen("ps h -eo pid:1,command")
                ]
            ]
            print(data)


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


def sign_single_tx_with_options(
    cli, tx_file, singer_name, *k_options, i=0, **kv_options
):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "sign",
            tx_file,
            *k_options,
            from_=singer_name,
            home=cli.cosmos_cli(i).data_dir,
            keyring_backend="test",
            chain_id=cli.cosmos_cli(i).chain_id,
            node=cli.cosmos_cli(i).node_rpc,
            **kv_options,
        )
    )


def find_balance(balances, denom):
    "find a denom in the coin list, return the amount, if not exists, return 0"
    for balance in balances:
        if balance["denom"] == denom:
            return int(balance["amount"])
    return 0


def transfer(cli, from_, to, coins, *k_options, i=0, **kv_options):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "bank",
            "send",
            from_,
            to,
            coins,
            "-y",
            *k_options,
            home=cli.cosmos_cli(i).data_dir,
            keyring_backend="test",
            chain_id=cli.cosmos_cli(i).chain_id,
            node=cli.cosmos_cli(i).node_rpc,
            **kv_options,
        )
    )


def grant_fee_allowance(cli, granter_address, grantee, *k_options, i=0, **kv_options):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "feegrant",
            "grant",
            granter_address,
            grantee,
            "-y",
            *k_options,
            home=cli.cosmos_cli(i).data_dir,
            keyring_backend="test",
            chain_id=cli.cosmos_cli(i).chain_id,
            node=cli.cosmos_cli(i).node_rpc,
            **kv_options,
        )
    )


def revoke_fee_grant(cli, granter_address, grantee, *k_options, i=0, **kv_options):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "feegrant",
            "revoke",
            granter_address,
            grantee,
            "-y",
            *k_options,
            home=cli.cosmos_cli(i).data_dir,
            keyring_backend="test",
            chain_id=cli.cosmos_cli(i).chain_id,
            node=cli.cosmos_cli(i).node_rpc,
            **kv_options,
        )
    )


def throw_error_for_non_success_code(func):
    def wrapper(*args, **kwargs):
        data = func(*args, **kwargs)
        # commands with --generate-only flag do not return response with code
        if "code" in data and data["code"] != SUCCESS_CODE:
            raise Exception(data)
        return data

    return wrapper


@throw_error_for_non_success_code
def exec_tx_by_grantee(cli, tx_file, grantee, *k_options, i=0, **kv_options):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "authz",
            "exec",
            tx_file,
            "-y",
            *k_options,
            from_=grantee,
            home=cli.cosmos_cli(i).data_dir,
            **kv_options,
        )
    )


@throw_error_for_non_success_code
def grant_authorization(
    cli, grantee, authorization_type, granter, *k_options, i=0, **kv_options
):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "authz",
            "grant",
            grantee,
            authorization_type,
            "-y",
            *k_options,
            from_=granter,
            home=cli.cosmos_cli(i).data_dir,
            **kv_options,
        )
    )


@throw_error_for_non_success_code
def revoke_authorization(
    cli, grantee, msg_type, granter, *k_options, i=0, **kv_options
):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "authz",
            "revoke",
            grantee,
            msg_type,
            "-y",
            *k_options,
            from_=granter,
            home=cli.cosmos_cli(i).data_dir,
            **kv_options,
        )
    )


def query_command(cli, *k_options, i=0, **kv_options):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "query",
            *k_options,
            home=cli.cosmos_cli(i).data_dir,
            output="json",
            *kv_options,
        )
    )


@throw_error_for_non_success_code
def delegate_amount(
    cli, validator_address, amount, from_, *k_options, i=0, **kv_options
):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "staking",
            "delegate",
            validator_address,
            amount,
            "-y",
            *k_options,
            from_=from_,
            home=cli.cosmos_cli(i).data_dir,
            **kv_options,
        )
    )


@throw_error_for_non_success_code
def unbond_amount(cli, validator_address, amount, from_, *k_options, i=0, **kv_options):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "staking",
            "unbond",
            validator_address,
            amount,
            "-y",
            *k_options,
            from_=from_,
            home=cli.cosmos_cli(i).data_dir,
            **kv_options,
        )
    )


@throw_error_for_non_success_code
def redelegate_amount(
    cli, src_validator, dst_validator, amount, from_, *k_options, i=0, **kv_options
):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "staking",
            "redelegate",
            src_validator,
            dst_validator,
            amount,
            "-y",
            *k_options,
            from_=from_,
            home=cli.cosmos_cli(i).data_dir,
            **kv_options,
        )
    )


def query_delegation_amount(cluster, delegator_address, validator_address):
    try:
        delegation_amount = query_command(
            cluster, STAKING, DELEGATION, delegator_address, validator_address
        )
    except AssertionError:
        return {"denom": BASECRO_DENOM, "amount": "0"}

    return delegation_amount["balance"]


def query_total_reward_amount(cluster, delegator_address, validator_address=""):
    try:
        rewards = query_command(
            cluster, DISTRIBUTION, REWARDS, delegator_address, validator_address
        )
    except AssertionError:
        return 0

    if validator_address:
        total_reward = sum(float(r["amount"]) for r in rewards["rewards"])
    else:
        total_reward = (
            sum([float(r["amount"]) for r in rewards["total"]])
            if rewards["total"]
            else 0
        )

    return total_reward


@throw_error_for_non_success_code
def withdraw_all_rewards(cli, from_delegator, *k_options, i=0, **kv_options):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "distribution",
            "withdraw-all-rewards",
            "-y",
            *k_options,
            from_=from_delegator,
            home=cli.cosmos_cli(i).data_dir,
            **kv_options,
        )
    )
