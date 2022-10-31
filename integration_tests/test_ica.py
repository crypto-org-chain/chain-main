import json
import subprocess
import time
from pathlib import Path

import pytest

from .ibc_utils import search_target, wait_relayer_ready
from .utils import cluster_fixture

pytestmark = pytest.mark.ibc


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/ica.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


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
    cli_controller = cluster["ica-controller-1"].cosmos_cli()
    cli_host = cluster["ica-host-1"].cosmos_cli()

    addr_controller = cluster["ica-controller-1"].address("signer")
    addr_host = cluster["ica-host-1"].address("signer")

    # create interchain account
    rsp = json.loads(
        cli_controller.raw(
            "tx",
            "icaauth",
            "register-account",
            controller_connection,
            "-y",
            from_=addr_controller,
            home=cli_controller.data_dir,
            node=cli_controller.node_rpc,
            keyring_backend="test",
            chain_id=cli_controller.chain_id,
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # FIXME more stable way to wait for relaying
    time.sleep(20)

    # get interchain account address
    ica_address = json.loads(
        cli_controller.raw(
            "query",
            "icaauth",
            "interchain-account-address",
            controller_connection,
            addr_controller,
            output="json",
            node=cli_controller.node_rpc,
            chain_id=cli_controller.chain_id,
        )
    )["interchainAccountAddress"]

    # initial balance of interchain account should be zero
    assert cli_host.balance(ica_address) == 0

    # send some funds to interchain account
    cli_host.transfer("signer", ica_address, "1cro")

    # check if the funds are received in interchain account
    assert cli_host.balance(ica_address) == 100000000

    # generate a transaction to send to host chain
    generated_tx = tmp_path / "generated_tx.txt"
    generated_tx_msg = cli_host.transfer(
        ica_address, addr_host, "0.5cro", generate_only=True
    )

    print(json.dumps(generated_tx_msg))

    with open(generated_tx, "w") as opened_file:
        json.dump(generated_tx_msg, opened_file)

    num_txs = len(cli_host.query_all_txs(ica_address)["txs"])

    # submit transaction on host chain on behalf of interchain account
    rsp = json.loads(
        cli_controller.raw(
            "tx",
            "icaauth",
            "submit-tx",
            controller_connection,
            generated_tx,
            "-y",
            from_=addr_controller,
            home=cli_controller.data_dir,
            node=cli_controller.node_rpc,
            keyring_backend="test",
            chain_id=cli_controller.chain_id,
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # FIXME more stable way to wait for relaying
    time.sleep(20)

    # check if the transaction is submitted
    assert len(cli_host.query_all_txs(ica_address)["txs"]) == num_txs + 1

    # check if the funds are reduced in interchain account
    assert cli_host.balance(ica_address) == 50000000
