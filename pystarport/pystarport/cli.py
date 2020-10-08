import signal
from pathlib import Path

import fire
import yaml

from .cluster import CHAIN, ClusterCLI, init_cluster, start_cluster
from .bot import BotCLI
from .utils import interact


def init(data, config, base_port, cmd):
    interact(
        f"rm -r {data}; mkdir {data}", ignore_error=True,
    )
    init_cluster(data, yaml.safe_load(open(config)), base_port, cmd)


def start(data):
    supervisord = start_cluster(data)

    # register signal to quit supervisord
    for signame in ("SIGINT", "SIGTERM"):
        signal.signal(getattr(signal, signame), lambda *args: supervisord.terminate())

    supervisord.wait()


def serve(data, config, base_port, cmd):
    init(data, config, base_port, cmd)
    start(data)


class CLI:
    def init(
        self,
        data: str = "./data",
        config: str = "./config.yaml",
        base_port: int = 26650,
        cmd: str = CHAIN,
    ):
        """
        prepare all the configurations of a devnet

        :param data: path to the root data directory
        :param config: path to the configuration file
        :param base_port: the base port to use, the service ports of different nodes
        are calculated based on this
        :param cmd: the chain binary to use
        """
        init(Path(data), config, base_port, cmd)

    def start(self, data: str = "./data"):
        """
        start the prepared devnet

        :param data: path to the root data directory
        """
        start(Path(data))

    def serve(
        self,
        data: str = "./data",
        config: str = "./config.yaml",
        base_port: int = 26650,
        cmd: str = CHAIN,
    ):
        """
        prepare and start a devnet from scatch

        :param data: path to the root data directory
        :param config: path to the configuration file
        :param base_port: the base port to use, the service ports of different nodes
        are calculated based on this
        :param cmd: the chain binary to use
        """
        serve(Path(data), config, base_port, cmd)

    def supervisorctl(self, *args, data: str = "./data"):
        from supervisor.supervisorctl import main

        main(("-c", Path(data) / "tasks.ini", *args))

    def cli(self, *args, data: str = "./data", cmd: str = CHAIN):
        return ClusterCLI(Path(data), cmd)

    def bot(
        self,
        *args,
        data: str = "./data",
        config_path: str = "./bot.yaml",
        cmd: str = CHAIN,
    ):
        """
        transaction bot CLI

        :param data: path to the root data directory
        :param config: path to the bot configuration file
        (refer bot.yaml.example for reference)
        """
        cluster_cli = ClusterCLI(Path(data), cmd)
        return BotCLI(config_path, cluster_cli)


def main():
    fire.Fire(CLI())


if __name__ == "__main__":
    main()
