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
    data_dir = tempfile.TemporaryDirectory(suffix="chain-test")
    cluster = Cluster(
        yaml.safe_load(open(CURRENT_DIR / "config.yml")),
        Path(data_dir.name),
        CLUSTER_BASE_PORT,
    )
    await cluster.init()
    await cluster.start()
    log_task = asyncio.create_task(cluster.watch_logs())

    yield cluster

    log_task.cancel()
    try:
        await log_task
    except asyncio.CancelledError:
        pass
    await cluster.terminate()
    data_dir.cleanup()
