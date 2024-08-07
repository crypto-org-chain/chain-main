import enum
import hashlib
import json
import socket
import sys
import time
from datetime import timedelta

import bech32
from dateutil.parser import isoparse
from pystarport import cluster, expansion, ledger
from pystarport.ports import rpc_port
from pystarport.utils import format_doc_string

from .cosmoscli import ClusterCLI

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


class ModuleAccount(enum.Enum):
    FeeCollector = "fee_collector"
    Mint = "mint"
    Gov = "gov"
    Distribution = "distribution"
    BondedPool = "bonded_tokens_pool"
    NotBondedPool = "not_bonded_tokens_pool"
    IBCTransfer = "transfer"


@format_doc_string(
    options=",".join(v.value for v in ModuleAccount.__members__.values())
)
def module_address(name):
    """
    get address of module accounts

    :param name: name of module account, values: {options}
    """
    data = hashlib.sha256(ModuleAccount(name).value.encode()).digest()[:20]
    return bech32.bech32_encode("cro", bech32.convertbits(data, 8, 5))


def get_sync_info(s):
    return s.get("SyncInfo") or s.get("sync_info")


def wait_for_block(cli, height, timeout=240):
    for i in range(timeout * 2):
        try:
            status = cli.status()
        except AssertionError as e:
            print(f"get sync status failed: {e}", file=sys.stderr)
        else:
            current_height = int(get_sync_info(status)["latest_block_height"])
            if current_height >= height:
                break
            print("current block height", current_height)
        time.sleep(0.5)
    else:
        raise TimeoutError(f"wait for block {height} timeout")


def wait_for_new_blocks(cli, n, sleep=0.5):
    begin_height = int(get_sync_info((cli.status()))["latest_block_height"])
    while True:
        time.sleep(sleep)
        cur_height = int(get_sync_info((cli.status()))["latest_block_height"])
        if cur_height - begin_height >= n:
            break


def wait_for_block_time(cli, t):
    print("wait for block time", t)
    while True:
        now = isoparse(get_sync_info(cli.status())["latest_block_time"])
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
    post_init=None,
    cmd=None,
):
    """
    init a single devnet
    """
    base_port = gen_base_port(worker_index)
    print("init cluster at", data, ", base port:", base_port)
    cluster.init_cluster(data, config_path, base_port, cmd=cmd)
    config = expansion.expand_jsonnet(config_path, None)
    clis = {}
    for key in config:
        if key == "relayer":
            continue

        chain_id = key
        chain_data = data / chain_id

        if post_init:
            post_init(chain_id, chain_data)

        clis[chain_id] = ClusterCLI(data, chain_id=chain_id, cmd=cmd)

    supervisord = cluster.start_cluster(data)

    try:
        for cli in clis.values():
            # wait for first node rpc port available before start testing
            wait_for_port(rpc_port(cli.config["validators"][0]["base_port"]))
            # wait for the first block generated before start testing
            wait_for_block(cli, 2)

        if len(clis) == 1:
            yield list(clis.values())[0]
        else:
            yield clis

    finally:
        supervisord.terminate()
        supervisord.wait()


def get_ledger():
    return ledger.Ledger()


def find_log_event_attrs(events, ev_type, cond=None):
    for ev in events:
        if ev["type"] == ev_type:
            attrs = {attr["key"]: attr["value"] for attr in ev["attributes"]}
            if cond is None or cond(attrs):
                return attrs
    return None


def get_proposal_id(rsp, msg=",/cosmos.staking.v1beta1.MsgUpdateParams"):
    def cb(attrs):
        return "proposal_id" in attrs

    ev = find_log_event_attrs(rsp["events"], "submit_proposal", cb)
    assert ev["proposal_messages"] == msg, rsp
    return ev["proposal_id"]


def approve_proposal(
    cluster,
    rsp,
    vote_option="yes",
    msg=",/cosmos.staking.v1beta1.MsgUpdateParams",
):
    proposal_id = get_proposal_id(rsp, msg)
    proposal = cluster.query_proposal(proposal_id)
    if msg == ",/cosmos.gov.v1.MsgExecLegacyContent":
        assert proposal["status"] == "PROPOSAL_STATUS_DEPOSIT_PERIOD", proposal
    amount = cluster.balance(cluster.address("ecosystem"))
    rsp = cluster.gov_deposit("ecosystem", proposal_id, "1cro")
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cluster.balance(cluster.address("ecosystem")) == amount - 100000000
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_VOTING_PERIOD", proposal

    if vote_option is not None:
        rsp = cluster.gov_vote("validator", proposal_id, vote_option)
        assert rsp["code"] == 0, rsp["raw_log"]
        rsp = cluster.gov_vote("validator", proposal_id, vote_option, i=1)
        assert rsp["code"] == 0, rsp["raw_log"]
        assert (
            int(cluster.query_tally(proposal_id, i=1)[vote_option + "_count"])
            == cluster.staking_pool()
        ), "all voted"
    else:
        assert cluster.query_tally(proposal_id) == {
            "yes_count": "0",
            "no_count": "0",
            "abstain_count": "0",
            "no_with_veto_count": "0",
        }

    wait_for_block_time(
        cluster, isoparse(proposal["voting_end_time"]) + timedelta(seconds=5)
    )
    proposal = cluster.query_proposal(proposal_id)
    if vote_option == "yes":
        assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal
    else:
        assert proposal["status"] == "PROPOSAL_STATUS_REJECTED", proposal
    return amount


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
    rsp = json.loads(
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
    if GENERATE_ONLY not in k_options and rsp["code"] == 0:
        rsp = cli.cosmos_cli(i).event_query_tx_for(rsp["txhash"])
    return rsp


def grant_fee_allowance(cli, granter_address, grantee, *k_options, i=0, **kv_options):
    rsp = json.loads(
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
    if rsp["code"] == 0:
        rsp = cli.cosmos_cli(i).event_query_tx_for(rsp["txhash"])
    return rsp


def revoke_fee_grant(cli, granter_address, grantee, *k_options, i=0, **kv_options):
    rsp = json.loads(
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
    if rsp["code"] == 0:
        rsp = cli.cosmos_cli(i).event_query_tx_for(rsp["txhash"])
    return rsp


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
    rsp = json.loads(
        cli.cosmos_cli(i).raw(
            "tx",
            "authz",
            "exec",
            tx_file,
            "-y",
            "--gas",
            "300000",
            *k_options,
            from_=grantee,
            home=cli.cosmos_cli(i).data_dir,
            **kv_options,
        )
    )
    if rsp["code"] == 0:
        rsp = cli.cosmos_cli(i).event_query_tx_for(rsp["txhash"])
    return rsp


@throw_error_for_non_success_code
def grant_authorization(
    cli, grantee, authorization_type, granter, *k_options, i=0, **kv_options
):
    rsp = json.loads(
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
    if rsp["code"] == 0:
        rsp = cli.cosmos_cli(i).event_query_tx_for(rsp["txhash"])
    return rsp


@throw_error_for_non_success_code
def revoke_authorization(
    cli, grantee, msg_type, granter, *k_options, i=0, **kv_options
):
    rsp = json.loads(
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
    if rsp["code"] == 0:
        rsp = cli.cosmos_cli(i).event_query_tx_for(rsp["txhash"])
    return rsp


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


def query_block_info(cli, height, i=0):
    return json.loads(
        cli.cosmos_cli(i).raw(
            "query",
            "block",
            height,
            type="height",
            home=cli.cosmos_cli(i).data_dir,
        )
    )


@throw_error_for_non_success_code
def delegate_amount(
    cli, validator_address, amount, from_, *k_options, i=0, **kv_options
):
    rsp = json.loads(
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
    if GENERATE_ONLY not in k_options and rsp["code"] == 0:
        rsp = cli.cosmos_cli(i).event_query_tx_for(rsp["txhash"])
    return rsp


@throw_error_for_non_success_code
def unbond_amount(cli, validator_address, amount, from_, *k_options, i=0, **kv_options):
    rsp = json.loads(
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
    if GENERATE_ONLY not in k_options and rsp["code"] == 0:
        rsp = cli.cosmos_cli(i).event_query_tx_for(rsp["txhash"])
    return rsp


@throw_error_for_non_success_code
def redelegate_amount(
    cli, src_validator, dst_validator, amount, from_, *k_options, i=0, **kv_options
):
    rsp = json.loads(
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
    if GENERATE_ONLY not in k_options and rsp["code"] == 0:
        rsp = cli.cosmos_cli(i).event_query_tx_for(rsp["txhash"])
    return rsp


def query_delegation_amount(cluster, delegator_address, validator_address):
    try:
        delegation_amount = query_command(
            cluster, STAKING, DELEGATION, delegator_address, validator_address
        )
    except AssertionError:
        return {"denom": BASECRO_DENOM, "amount": "0"}
    return delegation_amount["delegation_response"]["balance"]


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
    rsp = json.loads(
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
    if GENERATE_ONLY not in k_options and rsp["code"] == 0:
        rsp = cli.cosmos_cli(i).event_query_tx_for(rsp["txhash"])
    return rsp


def wait_for_fn(name, fn, *, timeout=240, interval=1):
    for i in range(int(timeout / interval)):
        result = fn()
        if result:
            return result
        time.sleep(interval)
    else:
        raise TimeoutError(f"wait for {name} timeout")
