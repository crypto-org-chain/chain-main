import subprocess
from pathlib import Path

import pytest
from pystarport import ports

from .ibc_utils import ibc_transfer_flow
from .utils import cluster_fixture, wait_for_block, wait_for_port

pytestmark = pytest.mark.ibc


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/ibc_channel_genesis.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def create_ibc_client(data_root, dst_chain, src_chain):
    subprocess.run(
        [
            "hermes",
            "--config",
            data_root / "relayer.toml",
            "create",
            "client",
            "--host-chain",
            dst_chain,
            "--reference-chain",
            src_chain,
        ],
        check=True,
    )


def test_ibc_genesis_channel(cluster):
    for cli in cluster.values():
        for i in range(cli.nodes_len()):
            wait_for_port(ports.grpc_port(cli.base_port(i)))

    for cli in cluster.values():
        # wait for at least 3 blocks, because
        # "proof queries at height <= 2 are not supported"
        wait_for_block(cli, 3)

    data_root = next(iter(cluster.values())).data_root
    create_ibc_client(data_root, "ibc-1", "ibc-0")
    create_ibc_client(data_root, "ibc-0", "ibc-1")
    cluster["ibc-0"].supervisor.startProcess("relayer-demo")
    ibc_transfer_flow(cluster, "channel-0", "channel-0")
