import re
from pathlib import Path

import pytest

from .utils import cluster_fixture


def pytest_configure(config):
    config.addinivalue_line("markers", "slow: marks tests as slow")
    config.addinivalue_line("markers", "ledger: marks tests as ledger hardware test")
    config.addinivalue_line("markers", "grpc: marks grpc tests")
    config.addinivalue_line("markers", "upgrade: marks upgrade tests")
    config.addinivalue_line("markers", "normal: marks normal tests")
    config.addinivalue_line("markers", "ibc: marks ibc tests")
    config.addinivalue_line("markers", "byzantine: marks byzantine tests")
    config.addinivalue_line("markers", "gov: marks gov tests")
    config.addinivalue_line("markers", "solomachine: marks solomachine tests")


@pytest.fixture(scope="session")
def worker_index(worker_id):
    match = re.search(r"\d+", worker_id)
    return int(match[0]) if match is not None else 0


@pytest.fixture(scope="session")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "default cluster fixture"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/default.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


@pytest.fixture(scope="session")
def suspend_capture(pytestconfig):
    "used for pause in testing"

    class SuspendGuard:
        def __init__(self):
            self.capmanager = pytestconfig.pluginmanager.getplugin("capturemanager")

        def __enter__(self):
            self.capmanager.suspend_global_capture(in_=True)

        def __exit__(self, _1, _2, _3):
            self.capmanager.resume_global_capture()

    yield SuspendGuard()
