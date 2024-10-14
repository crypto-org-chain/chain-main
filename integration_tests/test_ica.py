import json
import subprocess
from pathlib import Path

import pytest
import requests
from pystarport import cluster as c

from .ibc_utils import search_target, wait_relayer_ready
from .utils import (
    approve_proposal,
    cluster_fixture,
    module_address,
    wait_for_fn,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.ibc


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/ica.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def wait_for_check_channel_ready(cli, connid, channel_id, target="STATE_OPEN"):
    print("wait for channel ready", channel_id, target)

    def check_channel_ready():
        channels = cli.ibc_query_channels(connid)["channels"]
        try:
            state = next(
                channel["state"]
                for channel in channels
                if channel["channel_id"] == channel_id
            )
        except StopIteration:
            return False
        return state == target

    wait_for_fn("channel ready", check_channel_ready)


def assert_channel_open_init(rsp):
    assert rsp["code"] == 0, rsp["raw_log"]
    port_id, channel_id = next(
        (
            evt["attributes"][0]["value"],
            evt["attributes"][1]["value"],
        )
        for evt in rsp["events"]
        if evt["type"] == "channel_open_init"
    )
    print("port-id", port_id, "channel-id", channel_id)
    return port_id, channel_id


def wait_for_check_tx(cli, adr, num_txs, timeout=None):
    print("wait for tx arrive")

    def check_tx():
        current = len(cli.query_all_txs(adr)["txs"])
        print("current", current)
        return current > num_txs

    if timeout is None:
        wait_for_fn("transfer tx", check_tx)
    else:
        try:
            print(f"should assert timeout err when pass {timeout}s")
            wait_for_fn("transfer tx", check_tx, timeout=timeout)
        except TimeoutError:
            raised = True
        assert raised


def start_and_wait_relayer(cluster):
    relayer = wait_relayer_ready(cluster)
    chains = ["ica-controller-1", "ica-host-1"]
    # create connection
    subprocess.run(
        relayer
        + [
            "create",
            "connection",
            "--a-chain",
            chains[0],
            "--b-chain",
            chains[1],
        ],
        check=True,
    )

    # start relaying
    cluster[chains[0]].supervisor.startProcess("relayer-demo")

    query = relayer + ["query", "connections", "--chain"]
    return search_target(query, "connection", chains)


def test_ica(cluster, tmp_path):
    controller_connection, host_connection = start_and_wait_relayer(cluster)
    # call chain-maind directly
    controller = cluster["ica-controller-1"]
    cli_controller = controller.cosmos_cli()
    cli_host = cluster["ica-host-1"].cosmos_cli()

    addr_controller = controller.address("signer")
    addr_host = cluster["ica-host-1"].address("signer")

    # create interchain account
    v = json.dumps({"fee_version": "ics29-1", "app_version": ""})
    rsp = cli_controller.ica_register_account(
        controller_connection,
        from_=addr_controller,
        gas="400000",
        version=v,
        ordering=c.ChannelOrder.ORDERED.value,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    port_id, channel_id = assert_channel_open_init(rsp)
    wait_for_check_channel_ready(cli_controller, controller_connection, channel_id)

    # upgrade to unordered channel
    authority = module_address("gov")
    channel = cli_controller.ibc_query_channel(port_id, channel_id)
    deposit = "0.1cro"
    version_data = json.loads(channel["channel"]["version"])
    signer = "signer"
    proposal_src = cli_controller.ibc_upgrade_channels(
        json.loads(version_data["app_version"]),
        signer,
        deposit=deposit,
        title="channel-upgrade-title",
        summary="summary",
        port_pattern=port_id,
        channel_ids=channel_id,
    )
    proposal_src["deposit"] = deposit
    proposal_src["messages"][0]["signer"] = authority
    proposal_src["messages"][0]["fields"]["ordering"] = c.ChannelOrder.UNORDERED.value
    proposal = tmp_path / "proposal.json"
    proposal.write_text(json.dumps(proposal_src))
    rsp = cli_controller.submit_gov_proposal(proposal, from_=signer)
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(controller, rsp, msg=",/ibc.core.channel.v1.MsgChannelUpgradeInit")
    wait_for_check_channel_ready(
        cli_controller, controller_connection, channel_id, "STATE_FLUSHCOMPLETE"
    )
    wait_for_check_channel_ready(cli_controller, controller_connection, channel_id)
    channel = cli_controller.ibc_query_channel(port_id, channel_id)
    assert channel["channel"]["ordering"] == c.ChannelOrder.UNORDERED.value, channel

    # get interchain account address
    ica_address = cli_controller.ica_query_account(
        controller_connection,
        addr_controller,
    )["address"]
    # initial balance of interchain account should be zero
    assert cli_host.balance(ica_address) == 0

    # send some funds to interchain account
    cli_host.transfer("signer", ica_address, "1cro")

    # check if the funds are received in interchain account
    assert cli_host.balance(ica_address) == 100000000

    def gen_send_msg(sender, receiver, denom, amount):
        return {
            "@type": "/cosmos.bank.v1beta1.MsgSend",
            "from_address": sender,
            "to_address": receiver,
            "amount": [{"denom": denom, "amount": f"{amount}"}],
        }

    no_timeout = 60
    num_txs = len(cli_host.query_all_txs(ica_address)["txs"])

    def submit_msgs(amt, timeout_in_s=no_timeout, gas="200000"):
        # generate a transaction to send to host chain
        data = json.dumps([gen_send_msg(ica_address, addr_host, "basecro", amt)])
        packet = cli_controller.ica_generate_packet_data(data)
        # submit transaction on host chain on behalf of interchain account
        rsp = cli_controller.ica_submit_tx(
            controller_connection,
            json.dumps(packet),
            timeout_duration=f"{timeout_in_s}s",
            gas=gas,
            from_=addr_controller,
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        timeout = timeout_in_s + 3 if timeout_in_s < no_timeout else None
        wait_for_check_tx(cli_host, ica_address, num_txs, timeout)
        return rsp["height"]

    submit_msgs(50000000)
    # check if the transaction is submitted
    assert len(cli_host.query_all_txs(ica_address)["txs"]) == num_txs + 1
    # check if the funds are reduced in interchain account
    assert cli_host.balance(ica_address) == 50000000
    height = int(submit_msgs(1000000000000000))

    ev = None
    type = "ibccallbackerror-ics27_packet"
    max_retry = 5
    for _ in range(max_retry):
        wait_for_new_blocks(cli_host, 1, sleep=0.1)
        url = f"http://127.0.0.1:26757/block_results?height={height}"
        res = requests.get(url).json()
        height += 1
        txs_results = res.get("result", {}).get("txs_results")
        if txs_results is None:
            continue
        for res in txs_results:
            ev = next((ev for ev in res.get("events", []) if ev["type"] == type), None)
            if ev:
                ev = {attr["key"]: attr["value"] for attr in ev["attributes"]}
                break
        if ev:
            break
    assert "insufficient funds" in ev["ibccallbackerror-error"], "no ibccallbackerror"
