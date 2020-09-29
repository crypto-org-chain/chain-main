import asyncio
from pathlib import Path

import fire
import yaml

from .cluster import CHAIN, Cluster
from .utils import interact


class CLI:
    def __init__(
        self, command=CHAIN, data_dir="./data", config="./config.yml", base_port=26650
    ):
        self._cluster = Cluster(
            yaml.safe_load(open(config)), Path(data_dir), base_port, command
        )
        self.cli = self._cluster.cli

    async def _init(self):
        await interact(
            f"rm -r {self._cluster.data_dir}; mkdir {self._cluster.data_dir}",
            ignore_error=True,
        )
        await self._cluster.init()

    def init(self):
        """
        initialize testnet data directory
        """
        asyncio.run(self._init())

    async def _start(self):
        await self._cluster.start()
        await self._cluster.watch_logs()

    def start(self):
        """
        start testnet processes
        """
        asyncio.run(self._start())

    async def _serve(self):
        await self._init()
        await self._start()

    def serve(self):
        """
        init + start
        """
        asyncio.run(self._serve())


def main():
    fire.Fire(CLI)


if __name__ == "__main__":
    main()
