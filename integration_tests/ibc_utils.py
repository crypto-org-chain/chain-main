import re
import subprocess

from pystarport import ports

from .utils import wait_for_block, wait_for_port


def wait_relayer_ready(cluster):
    for cli in cluster.values():
        for i in range(cli.nodes_len()):
            wait_for_port(ports.grpc_port(cli.base_port(i)))

    for cli in cluster.values():
        # wait for at least 3 blocks, because
        # "proof queries at height <= 2 are not supported"
        wait_for_block(cli, 3)

    # all clusters share the same root data directory
    data_root = next(iter(cluster.values())).data_root
    return ["hermes", "--config", data_root / "relayer.toml"]


def search_target(query, key, chains):
    results = []
    for chain in chains:
        raw = subprocess.check_output(query + [chain]).decode("utf-8")
        results.append(re.search(r"" + key + r"-\d*", raw).group())
    return results


def start_and_wait_relayer(cluster, port="transfer", init_relayer=True):
    relayer = wait_relayer_ready(cluster)
    chains = ["ibc-0", "ibc-1"]
    if init_relayer:
        # create connection and channel
        subprocess.run(
            relayer
            + [
                "create",
                "channel",
                "--a-port",
                port,
                "--b-port",
                port,
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
    return search_target(query, "channel", chains)


# def start_and_wait_relayer_nft_transfer(cluster, init_relayer=True):
#     relayer = wait_relayer_ready(cluster)
#     chains = ["ibc-0", "ibc-1"]
#     if init_relayer:
#         # create connection and channel
#         subprocess.run(
#             relayer
#             + [
#                 "create",
#                 "channel",
#                 "--a-port",
#                 "nft-transfer",
#                 "--b-port",
#                 "nft-transfer",
#                 "--a-chain",
#                 chains[0],
#                 "--b-chain",
#                 chains[1],
#                 "--new-client-connection",
#                 "--channel-version",
#                 "ics721-1",
#                 "--yes",
#             ],
#             check=True,
#         )

#         # start relaying
#         cluster[chains[0]].supervisor.startProcess("relayer-demo")

#     query = relayer + ["query", "channels", "--chain"]
#     return search_target(query, "channel", chains)