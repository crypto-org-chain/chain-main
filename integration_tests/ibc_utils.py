import subprocess
import re

from pystarport import ports
from .utils import wait_for_block, wait_for_port


def start_and_wait_relayer(cluster, init_relayer=True):
    for cli in cluster.values():
        for i in range(cli.nodes_len()):
            wait_for_port(ports.grpc_port(cli.base_port(i)))

    for cli in cluster.values():
        # wait for at least 3 blocks, because
        # "proof queries at height <= 2 are not supported"
        wait_for_block(cli, 3)

    # all clusters share the same root data directory
    data_root = next(iter(cluster.values())).data_root
    relayer = ["hermes", "--config", data_root / "relayer.toml"]
    chains = ["ibc-0", "ibc-1"]
    if init_relayer:
        # create connection and channel
        subprocess.run(
            relayer
            + [
                "create",
                "channel",
                "--a-port",
                "transfer",
                "--b-port",
                "transfer",
                "--a-chain",
                chains[0],
                "--b-chain",
                chains[1],
                "--new-client-connection",
                "--yes",
            ],
            check=True,
        )

        # start relaying
        cluster[chains[0]].supervisor.startProcess("relayer-demo")

    query = relayer + ["query", "channels", "--chain"]
    [src_channel, dst_channel] = [re.search(
        "channel-.",
        subprocess.check_output(query + [chain]).decode("utf-8"),
    ).group() for chain in chains]
    return src_channel, dst_channel
