import os
import signal
from pathlib import Path

import fire

from .app import CHAIN, IMAGE, SUPERVISOR_CONFIG_FILE
from .bot import BotCLI, BotClusterCLI
from .cluster import ClusterCLI, init_cluster, start_cluster, start_tail_logs_thread
from .cosmoscli import CosmosCLI
from .utils import build_cli_args, interact


def init(
    data,
    config,
    base_port,
    *args,
    **kwargs,
):
    interact(
        f"rm -r {data}; mkdir {data}",
        ignore_error=True,
    )
    return init_cluster(data, config, base_port, *args, **kwargs)


def start(data, quiet):
    supervisord = start_cluster(data)

    # register signal to quit supervisord
    for signame in ("SIGINT", "SIGTERM"):
        signal.signal(getattr(signal, signame), lambda *args: supervisord.terminate())

    if not quiet:
        tailer = start_tail_logs_thread(data)

    supervisord.wait()

    if not quiet:
        tailer.stop()
        tailer.join()


def serve(data, config, base_port, cmd, quiet):
    init(data, config, base_port, cmd=cmd)
    start(data, quiet)


class CLI:
    def __init__(self, /, cmd=CHAIN):
        """
        :param cmd: path to the chain binary
        """
        self.cmd = cmd

    def init(
        self,
        data: str = "./data",
        config: str = "./config.yaml",
        base_port: int = 26650,
        image: str = IMAGE,
        gen_compose_file: bool = False,
        cmd: str = CHAIN,
    ):
        """
        prepare all the configurations of a devnet

        :param data: path to the root data directory
        :param config: path to the configuration file
        :param base_port: the base port to use, the service ports of different nodes
        are calculated based on this
        :param image: the image used in the generated docker-compose.yml
        :param gen_compose_file: generate a docker-compose.yml
        :param cmd: path to chain binary
        """
        init(Path(data), config, base_port, image, self.cmd, gen_compose_file)

    def start(self, data: str = "./data", quiet: bool = False):
        """
        start the prepared devnet

        :param data: path to the root data directory
        :param quiet: don't print logs of subprocesses
        """
        start(Path(data), quiet)

    def chaind(self, *args, **kwargs):
        """
        start one node whose home directory is already initialized
        can be used to launch chain-maind

        :param home: home directory
        """
        os.execvp(self.cmd, [self.cmd] + build_cli_args(*args, **kwargs))

    def serve(
        self,
        data: str = "./data",
        config: str = "./config.yaml",
        base_port: int = 26650,
        quiet: bool = False,
    ):
        """
        prepare and start a devnet from scatch

        :param data: path to the root data directory
        :param config: path to the configuration file
        :param base_port: the base port to use, the service ports of different nodes
        are calculated based on this
        :param cmd: path to chain binary
        :param quiet: don't print logs of subprocesses
        """
        serve(Path(data), config, base_port, self.cmd, quiet)

    def supervisorctl(self, *args, data: str = "./data"):
        from supervisor.supervisorctl import main

        main(("-c", Path(data) / SUPERVISOR_CONFIG_FILE, *args))

    def cli(self, *args, data: str = "./data", chain_id: str = "chainmaind"):
        """
        pystarport CLI

        :param data: path to the root data directory
        :param chain_id: chain id of the cluster
        """
        return ClusterCLI(Path(data), chain_id=chain_id, cmd=self.cmd)

    def bot(
        self,
        *args,
        data: str = "./data",
        config: str = "./bot.yaml",
        chain_id: str = "chainmaind",
        node_rpc: str = None,
    ):
        """
        transaction bot CLI

        :param data: path to the root data directory if connecting to pystarport
        cluster. Path to the home directory if connecting to a node
        :param config: path to the bot configuration file
        (copy bot.yaml.example for reference)
        :param chain_id: chain id of the cluster
        :param node_rpc: custom Tendermint RPC endpoint to the node
        """
        data_path = Path(data)
        config_path = Path(config)
        if node_rpc is None:
            cluster_cli = ClusterCLI(data_path, chain_id=chain_id, cmd=self.cmd)
            return BotClusterCLI(config_path, cluster_cli)
        else:
            cosmos_cli = CosmosCLI(data_path, node_rpc, cmd=self.cmd)
            return BotCLI(config_path, cosmos_cli)


def main():
    fire.Fire(CLI)


if __name__ == "__main__":
    main()
