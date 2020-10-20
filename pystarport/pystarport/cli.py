import os
import signal
from pathlib import Path

import fire
import yaml

from .bot import BotCLI
from .cluster import (
    CHAIN,
    IMAGE,
    SUPERVISOR_CONFIG_FILE,
    ClusterCLI,
    init_cluster,
    start_cluster,
)
from .utils import build_cli_args, interact


def init(data, config, *args, **kwargs):
    interact(
        f"rm -r {data}; mkdir {data}",
        ignore_error=True,
    )
    return init_cluster(data, yaml.safe_load(open(config)), *args, **kwargs)


def start(data, quiet):
    supervisord = start_cluster(data, quiet=quiet)

    # register signal to quit supervisord
    for signame in ("SIGINT", "SIGTERM"):
        signal.signal(getattr(signal, signame), lambda *args: supervisord.terminate())

    supervisord.wait()


def serve(data, config, base_port, cmd, quiet):
    init(data, config, base_port, cmd)
    start(data, quiet)


class CLI:
    def init(
        self,
        data: str = "./data",
        config: str = "./config.yaml",
        base_port: int = 26650,
        cmd: str = CHAIN,
        image: str = IMAGE,
        gen_compose_file: bool = False,
    ):
        """
        prepare all the configurations of a devnet

        :param data: path to the root data directory
        :param config: path to the configuration file
        :param base_port: the base port to use, the service ports of different nodes
        are calculated based on this
        :param cmd: the chain binary to use
        :param image: the image used in the generated docker-compose.yml
        :param gen_compose_file: generate a docker-compose.yml
        """
        init(Path(data), config, base_port, image, cmd, gen_compose_file)

    def start(self, data: str = "./data", quiet: bool = False):
        """
        start the prepared devnet

        :param data: path to the root data directory
        :param quiet: redirect supervisord stdout to supervisord.log if True
        """
        start(Path(data), quiet)

    def chaind(self, *args, **kwargs):
        """
        start one node whose home directory is already initialized
        can be used to launch chain-maind

        :param home: home directory
        :param cmd: the chain binary to use
        """
        os.execvp(CHAIN, [CHAIN] + build_cli_args(*args, **kwargs))

    def serve(
        self,
        data: str = "./data",
        config: str = "./config.yaml",
        base_port: int = 26650,
        cmd: str = CHAIN,
        quiet: bool = False,
    ):
        """
        prepare and start a devnet from scatch

        :param data: path to the root data directory
        :param config: path to the configuration file
        :param base_port: the base port to use, the service ports of different nodes
        are calculated based on this
        :param cmd: the chain binary to use
        :param quiet: redirect supervisord stdout to supervisord.log if True
        """
        serve(Path(data), config, base_port, cmd, quiet)

    def supervisorctl(self, *args, data: str = "./data"):
        from supervisor.supervisorctl import main

        main(("-c", Path(data) / SUPERVISOR_CONFIG_FILE, *args))

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
        :param config_path: path to the bot configuration file
        (copy bot.yaml.example for reference)
        :param cmd: the chain binary to use
        """
        cluster_cli = ClusterCLI(Path(data), cmd)
        return BotCLI(config_path, cluster_cli)


def main():
    fire.Fire(CLI())


if __name__ == "__main__":
    main()
