import configparser
import functools
import json
import re
import subprocess
import time
from pathlib import Path

import pytest
from dateutil.parser import isoparse
from pystarport.cluster import SUPERVISOR_CONFIG_FILE
from pystarport.ports import rpc_port

from .utils import (
    cluster_fixture,
    parse_events,
    wait_for_block,
    wait_for_block_time,
    wait_for_port,
)


def edit_chain_program(ini_path, callback):
    # edit node process config in supervisor
    ini = configparser.RawConfigParser()
    ini.read_file(ini_path.open())
    reg = re.compile(r"^program:node(\d+)")
    for section in ini.sections():
        m = reg.match(section)
        if m:
            i = m.group(1)
            old = ini[section]
            ini[section].update(callback(i, old))
    with ini_path.open("w") as fp:
        ini.write(fp)


def post_init(config, data, package_path):
    """
    change to use cosmovisor
    """

    def prepare_node(i, _):
        return {
            "command": "cosmovisor start",
            "environment": (
                f"DAEMON_NAME=chain-maind,DAEMON_HOME=%(here)s/node{i},"
                f"PACKAGE_PATH={package_path}"
            ),
        }

    edit_chain_program(data / SUPERVISOR_CONFIG_FILE, prepare_node)


# use function scope to re-initialize for each test case
@pytest.fixture
def cluster(pytestconfig, tmp_path_factory, test_package):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/default.yaml",
        26900,
        tmp_path_factory,
        quiet=pytestconfig.getoption("supervisord-quiet"),
        post_init=functools.partial(post_init, package_path=test_package),
        enable_cov=False,
    )


@pytest.fixture
def test_package():
    return Path(
        subprocess.check_output(
            ["nix-build", "-Q", Path(__file__).parent / "upgrade-test.nix"]
        )
        .strip()
        .decode()
    )


@pytest.mark.slow
def test_cosmovisor(cluster):
    """
    - propose an upgrade and pass it
    - wait for it to happen
    - it should work transparently
    """
    height = cluster.block_height()
    target_height = height + 15
    print("upgrade height", target_height)
    plan_name = "v1"
    rsp = cluster.gov_propose(
        "community",
        "software-upgrade",
        {
            "name": plan_name,
            "title": "upgrade test",
            "description": "ditto",
            "upgrade-height": target_height,
            "deposit": "10000000basecro",
        },
    )
    assert rsp["code"] == 0, rsp

    # get proposal_id
    ev = parse_events(rsp["logs"])["submit_proposal"]
    assert ev["proposal_type"] == "SoftwareUpgrade", rsp
    proposal_id = ev["proposal_id"]

    rsp = cluster.gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = cluster.gov_vote("validator", proposal_id, "yes", i=1)
    assert rsp["code"] == 0, rsp["raw_log"]

    proposal = cluster.query_proposal(proposal_id)
    wait_for_block_time(cluster, isoparse(proposal["voting_end_time"]))
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal

    # block should just pass the target height
    wait_for_block(cluster, target_height + 2)


@pytest.mark.slow
def test_manual_upgrade(cluster, test_package):
    """
    - do the upgrade test by replacing binary manually
    - check the panic do happens
    """
    # use the normal binary first
    edit_chain_program(
        cluster.data_dir / SUPERVISOR_CONFIG_FILE,
        lambda i, _: {
            "command": f"{test_package}/v0/bin/chain-maind start "
            f"--home %(here)s/node{i}"
        },
    )
    cluster.supervisor.stopAllProcesses()
    cluster.restart_supervisor()
    time.sleep(5)  # FIXME the port seems still exists for a while after process stopped
    wait_for_port(rpc_port(cluster.config["validators"][0]["base_port"]))

    target_height = cluster.block_height() + 15

    print("upgrade height", target_height)
    plan_name = "v1"
    rsp = cluster.gov_propose(
        "community",
        "software-upgrade",
        {
            "name": plan_name,
            "title": "upgrade test",
            "description": "ditto",
            "upgrade-height": target_height,
            "deposit": "10000000basecro",
        },
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # get proposal_id
    ev = parse_events(rsp["logs"])["submit_proposal"]
    assert ev["proposal_type"] == "SoftwareUpgrade", rsp
    proposal_id = ev["proposal_id"]

    rsp = cluster.gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = cluster.gov_vote("validator", proposal_id, "yes", i=1)
    assert rsp["code"] == 0, rsp["raw_log"]

    proposal = cluster.query_proposal(proposal_id)
    wait_for_block_time(cluster, isoparse(proposal["voting_end_time"]))
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal

    # wait for upgrade plan activated
    wait_for_block(cluster, target_height)
    # wait a little bit
    time.sleep(0.5)

    # check nodes are all stopped
    assert cluster.supervisor.getProcessInfo("node0")["state"] != "RUNNING"
    assert cluster.supervisor.getProcessInfo("node1")["state"] != "RUNNING"

    # check upgrade-info.json file is written
    assert (
        json.load((cluster.home(0) / "data/upgrade-info.json").open())
        == json.load((cluster.home(1) / "data/upgrade-info.json").open())
        == {
            "name": plan_name,
            "height": target_height,
        }
    )

    # use the upgrade-test binary
    edit_chain_program(
        cluster.data_dir / SUPERVISOR_CONFIG_FILE,
        lambda i, _: {
            "command": (
                f"{test_package}/v1/bin/chain-maind start --home %(here)s/node{i}"
            )
        },
    )
    cluster.restart_supervisor()

    # wait for it to generate new blocks
    wait_for_block(cluster, target_height + 1)
