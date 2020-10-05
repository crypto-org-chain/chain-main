import re
from pathlib import Path

import pytest

from .utils import cluster_fixture


def pytest_configure(config):
    config.addinivalue_line("markers", "slow: marks tests as slow")


@pytest.fixture(scope="session")
def cluster(worker_id):
    "default cluster fixture"
    match = re.search(r"\d+", worker_id)
    worker_id = int(match[0]) if match is not None else 0
    base_port = (100 + worker_id) * 100
    yield from cluster_fixture(
        Path(__file__).parent / "configs/default.yaml", base_port
    )
