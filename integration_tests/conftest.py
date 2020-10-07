import re
from pathlib import Path

import pytest

from .utils import cluster_fixture


def pytest_configure(config):
    config.addinivalue_line("markers", "slow: marks tests as slow")


def pytest_addoption(parser):
    parser.addoption(
        "--supervisord-quiet",
        dest="supervisord-quiet",
        action="store_true",
        default=False,
        help="redirect supervisord's stdout to file",
    )


@pytest.fixture(scope="session")
def cluster(worker_id, pytestconfig):
    "default cluster fixture"
    match = re.search(r"\d+", worker_id)
    worker_id = int(match[0]) if match is not None else 0
    base_port = (100 + worker_id) * 100
    yield from cluster_fixture(
        Path(__file__).parent / "configs/default.yaml",
        base_port,
        quiet=pytestconfig.getoption("supervisord-quiet"),
    )


@pytest.fixture(scope="session")
def suspend_capture(pytestconfig):
    "used for pause in testing"

    class suspend_guard:
        def __init__(self):
            self.capmanager = pytestconfig.pluginmanager.getplugin("capturemanager")

        def __enter__(self):
            self.capmanager.suspend_global_capture(in_=True)

        def __exit__(self, _1, _2, _3):
            self.capmanager.resume_global_capture()

    yield suspend_guard()
