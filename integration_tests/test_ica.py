import json
import subprocess
from pathlib import Path

import pytest
import requests
from pystarport import cluster as c

from .ibc_utils import search_target, wait_for_check_channel_ready, wait_relayer_ready
from .utils import cluster_fixture, wait_for_fn, wait_for_new_blocks

pytestmark = pytest.mark.ibc


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/ica.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


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
    v = json.dumps(
        {
            "version": "ics27-1",
            "encoding": "proto3",
            "tx_type": "sdk_multi_msg",
            "controller_connection_id": controller_connection,
            "host_connection_id": host_connection,
        }
    )

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

    # ibc upgrade channel does not work for the ibc-go-v10.1.1 implementation

    # upgrade to unordered channel
    # authority = module_address("gov")
    # channel = cli_controller.ibc_query_channel(port_id, channel_id)
    # deposit = "0.1cro"
    # version_data = json.loads(channel["channel"]["version"])
    # signer = "signer"
    # proposal_src = cli_controller.ibc_upgrade_channels(
    #     version_data["version"],
    #     signer,
    #     deposit=deposit,
    #     title="channel-upgrade-title",
    #     summary="summary",
    #     port_pattern=port_id,
    #     channel_ids=channel_id,
    # )
    # proposal_src["deposit"] = deposit
    # proposal_src["messages"][0]["signer"] = authority
    # proposal_src["messages"][0]["fields"]["ordering"] = c.ChannelOrder.UNORDERED.value
    # proposal = tmp_path / "proposal.json"
    # proposal.write_text(json.dumps(proposal_src))
    # rsp = cli_controller.submit_gov_proposal(proposal, from_=signer)
    # assert rsp["code"] == 0, rsp["raw_log"]
    # approve_proposal(
    #     controller, rsp, msg=",/ibc.core.channel.v1.MsgChannelUpgradeInit"
    # )
    # wait_for_check_channel_ready(
    #     cli_controller, controller_connection, channel_id, "STATE_FLUSHCOMPLETE"
    # )
    # wait_for_check_channel_ready(cli_controller, controller_connection, channel_id)
    # channel = cli_controller.ibc_query_channel(port_id, channel_id)
    # assert channel["channel"]["ordering"] == c.ChannelOrder.UNORDERED.value, channel

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
    max_retry = 5
    for _ in range(max_retry):
        wait_for_new_blocks(cli_host, 1, sleep=0.1)
        url = f"http://127.0.0.1:26757/block_results?height={height}"
        res = requests.get(url).json()
        height += 1
        txs_results = res.get("result", {}).get("txs_results")
        if txs_results is None:
            continue
        for res in txs_results or []:
            for event in res.get("events", []):
                event_type = event.get("type", "")
                if "ics27_packet" not in event_type:
                    continue
                attrs = {
                    attr["key"]: attr["value"]
                    for attr in event.get("attributes", [])
                }
                error_msg = (
                    attrs.get("ibccallbackerror-error") or attrs.get("error")
                )
                success = (
                    attrs.get("ibccallbackerror-success") or attrs.get("success")
                )
                is_error_event = (
                    event_type.startswith("ibccallbackerror") or success == "false"
                )
                if is_error_event and error_msg:
                    ev = attrs
                    break
            if ev:
                break
        if ev:
            break
    assert ev, "missing ics27 packet error event"
    err_msg = ev.get("ibccallbackerror-error") or ev.get("error", "")
    assert "insufficient funds" in err_msg, f"unexpected ics27 error: {err_msg}"
