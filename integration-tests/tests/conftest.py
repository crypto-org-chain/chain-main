import asyncio
import tempfile
from pathlib import Path

import pytest
import yaml

from pystarport.cluster import Cluster

CURRENT_DIR = Path(__file__).parent
CLUSTER_BASE_PORT = 26650


@pytest.yield_fixture(scope="session")
def event_loop(request):
    loop = asyncio.get_event_loop_policy().new_event_loop()
    yield loop
    loop.close()


@pytest.fixture(scope="session")
async def cluster():
    with tempfile.TemporaryDirectory(suffix="chain-test") as tempdir:
        cluster = Cluster(
            yaml.safe_load(open(CURRENT_DIR / "config.yml")),
            Path(tempdir),
            CLUSTER_BASE_PORT,
        )
        await cluster.init()
        await cluster.start()

        yield cluster

        await cluster.terminate()
