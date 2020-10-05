import signal
from pathlib import Path

import fire
import yaml

from .cluster import ClusterCLI, init_cluster, start_cluster
from .utils import interact


def init(data, config, base_port, cmd=None):
    interact(
        f"rm -r {data}; mkdir {data}", ignore_error=True,
    )
    init_cluster(data, yaml.safe_load(open(config)), base_port, cmd)


def start(data):
    supervisord = start_cluster(data)

    # register signal to quit supervisord
    for signame in ("SIGINT", "SIGTERM"):
        signal.signal(getattr(signal, signame), supervisord.terminate)

    supervisord.wait()


def serve(data, config, base_port, cmd=None):
    init(data, config, base_port, cmd)
    start(data)


class CLI:
    def init(self, data="./data", config="./config.yaml", base_port=26650, cmd=None):
        """
        initialize testnet data directory
        """
        init(Path(data), config, base_port, cmd)

    def start(self, data="./data"):
        """
        start testnet processes
        """
        start(Path(data))

    def serve(self, data="./data", config="./config.yaml", base_port=26650, cmd=None):
        """
        init + start
        """
        serve(Path(data), config, base_port, cmd)

    def supervisorctl(self, *args, data="./data"):
        from supervisor.supervisorctl import main

        main(("-c", Path(data) / "tasks.ini", *args))

    def cli(self, *args, data="./data", cmd=None):
        return ClusterCLI(Path(data), cmd)


def main():
    fire.Fire(CLI())


if __name__ == "__main__":
    main()
