import json
import subprocess
import time
from pathlib import Path

import pytest
from pystarport import ports

from .utils import cluster_fixture, wait_for_block, wait_for_port

pytestmark = pytest.mark.ibc


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/ica.yaml",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def start_and_wait_relayer(cluster):
    for cli in cluster.values():
        for i in range(cli.nodes_len()):
            wait_for_port(ports.grpc_port(cli.base_port(i)))

    for cli in cluster.values():
        # wait for at least 3 blocks, because
        # "proof queries at height <= 2 are not supported"
        wait_for_block(cli, 3)

    # all clusters share the same root data directory
    data_root = next(iter(cluster.values())).data_root
    relayer = ["hermes", "-j", "-c", data_root / "relayer.toml"]

    # create connection
    subprocess.run(
        relayer
        + [
            "create",
            "connection",
            "ica-controller-1",
            "ica-host-1",
        ],
        check=True,
    )

    # start relaying
    cluster["ica-controller-1"].supervisor.startProcess("relayer-demo")

    rsp = json.loads(
        subprocess.check_output(relayer + ["query", "connections", "ica-controller-1"])
    )
    controller_connection = rsp["result"][0]

    rsp = json.loads(
        subprocess.check_output(relayer + ["query", "connections", "ica-host-1"])
    )
    host_connection = rsp["result"][0]

    return controller_connection, host_connection


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
            from_="signer",
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
            from_="signer",
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
