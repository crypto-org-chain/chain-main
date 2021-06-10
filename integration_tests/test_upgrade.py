import configparser
import json
import re
import subprocess
import time
from datetime import datetime, timedelta, timezone
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
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = pytest.mark.upgrade


def edit_chain_program(chain_id, ini_path, callback):
    # edit node process config in supervisor
    ini = configparser.RawConfigParser()
    ini.read_file(ini_path.open())
    reg = re.compile(fr"^program:{chain_id}-node(\d+)")
    for section in ini.sections():
        m = reg.match(section)
        if m:
            i = m.group(1)
            old = ini[section]
            ini[section].update(callback(i, old))
    with ini_path.open("w") as fp:
        ini.write(fp)


def init_cosmovisor(data):
    """
    build and setup cosmovisor directory structure in devnet's data directory
    """
    cosmovisor = data / "cosmovisor"
    cosmovisor.mkdir()
    subprocess.run(
        [
            "nix-build",
            Path(__file__).parent / "upgrade-test.nix",
            "-o",
            cosmovisor / "upgrades",
        ],
        check=True,
    )
    (cosmovisor / "genesis").symlink_to("./upgrades/genesis")


def post_init(chain_id, data):
    """
    change to use cosmovisor
    """

    def prepare_node(i, _):
        # link cosmovisor directory for each node
        home = data / f"node{i}"
        (home / "cosmovisor").symlink_to("../../cosmovisor")
        return {
            "command": f"cosmovisor start --home %(here)s/node{i}",
            "environment": f"DAEMON_NAME=chain-maind,DAEMON_HOME={home.absolute()}",
        }

    edit_chain_program(chain_id, data / SUPERVISOR_CONFIG_FILE, prepare_node)


def migrate_genesis_time(cluster, i=0):
    genesis = json.load(open(cluster.home(i) / "config/genesis.json"))
    genesis["genesis_time"] = cluster.config.get("genesis-time")
    (cluster.home(i) / "config/genesis.json").write_text(json.dumps(genesis))


# use function scope to re-initialize for each test case
@pytest.fixture(scope="function")
def cosmovisor_cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    data = tmp_path_factory.mktemp("data")
    init_cosmovisor(data)
    yield from cluster_fixture(
        Path(__file__).parent / "configs/default.yaml",
        worker_index,
        data,
        quiet=pytestconfig.getoption("supervisord-quiet"),
        post_init=post_init,
        enable_cov=False,
        cmd=(data / "cosmovisor/genesis/bin/chain-maind"),
    )


@pytest.mark.skip(
    reason="CI fail: https://github.com/crypto-org-chain/chain-main/issues/560"
)
def test_cosmovisor(cosmovisor_cluster):
    """
    - propose an upgrade and pass it
    - wait for it to happen
    - it should work transparently
    """
    cluster = cosmovisor_cluster
    height = cluster.block_height()
    target_height = height + 15
    print("upgrade height", target_height)
    plan_name = "v2.0.0"
    rsp = cluster.gov_propose(
        "community",
        "software-upgrade",
        {
            "name": plan_name,
            "title": "upgrade test",
            "description": "ditto",
            "upgrade-height": target_height,
            "deposit": "0.1cro",
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
    wait_for_block(cluster, target_height + 2, 480)


def propose_and_pass(cluster, kind, proposal):
    rsp = cluster.gov_propose(
        "community",
        kind,
        proposal,
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # get proposal_id
    ev = parse_events(rsp["logs"])["submit_proposal"]
    assert ev["proposal_type"] == kind.title().replace("-", ""), rsp
    proposal_id = ev["proposal_id"]

    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_VOTING_PERIOD", proposal

    rsp = cluster.gov_vote("validator", proposal_id, "yes")
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = cluster.gov_vote("validator", proposal_id, "yes", i=1)
    assert rsp["code"] == 0, rsp["raw_log"]

    proposal = cluster.query_proposal(proposal_id)
    wait_for_block_time(
        cluster, isoparse(proposal["voting_end_time"]) + timedelta(seconds=1)
    )
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal

    return proposal


def test_manual_upgrade(cosmovisor_cluster):
    """
    - do the upgrade test by replacing binary manually
    - check the panic do happens
    """
    cluster = cosmovisor_cluster
    # use the normal binary first
    edit_chain_program(
        cluster.chain_id,
        cluster.data_dir / SUPERVISOR_CONFIG_FILE,
        lambda i, _: {
            "command": f"%(here)s/node{i}/cosmovisor/genesis/bin/chain-maind start "
            f"--home %(here)s/node{i}"
        },
    )
    cluster.reload_supervisor()
    time.sleep(5)  # FIXME the port seems still exists for a while after process stopped
    wait_for_port(rpc_port(cluster.config["validators"][0]["base_port"]))
    # wait for a new block to make sure chain started up
    wait_for_new_blocks(cluster, 1)
    target_height = cluster.block_height() + 15
    print("upgrade height", target_height)

    plan_name = "v2.0.0"
    propose_and_pass(
        cluster,
        "software-upgrade",
        {
            "name": plan_name,
            "title": "upgrade test",
            "description": "ditto",
            "upgrade-height": target_height,
            "deposit": "0.1cro",
        },
    )

    # wait for upgrade plan activated
    wait_for_block(cluster, target_height, 600)
    # wait a little bit
    time.sleep(0.5)

    # check nodes are all stopped
    assert (
        cluster.supervisor.getProcessInfo(f"{cluster.chain_id}-node0")["state"]
        != "RUNNING"
    )
    assert (
        cluster.supervisor.getProcessInfo(f"{cluster.chain_id}-node1")["state"]
        != "RUNNING"
    )

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
        cluster.chain_id,
        cluster.data_dir / SUPERVISOR_CONFIG_FILE,
        lambda i, _: {
            "command": (
                f"%(here)s/node{i}/cosmovisor/upgrades/v2.0.0/bin/chain-maind "
                f"start --home %(here)s/node{i}"
            )
        },
    )
    cluster.reload_supervisor()

    # wait for it to generate new blocks
    wait_for_block(cluster, target_height + 2, 600)


def test_cancel_upgrade(cluster):
    """
    use default cluster
    - propose upgrade and pass it
    - cancel the upgrade before execution
    """
    plan_name = "upgrade-test"
    # 25 = voting_period * 2 + 5
    upgrade_time = datetime.utcnow() + timedelta(seconds=25)
    print("propose upgrade plan")
    print("upgrade time", upgrade_time)
    propose_and_pass(
        cluster,
        "software-upgrade",
        {
            "name": plan_name,
            "title": "upgrade test",
            "description": "ditto",
            "upgrade-time": upgrade_time.replace(tzinfo=None).isoformat("T") + "Z",
            "deposit": "0.1cro",
        },
    )

    print("cancel upgrade plan")
    propose_and_pass(
        cluster,
        "cancel-software-upgrade",
        {
            "title": "there is bug, cancel upgrade",
            "description": "there is bug, cancel upgrade",
            "deposit": "0.1cro",
        },
    )

    # wait for blocks after upgrade, should success since upgrade is canceled
    wait_for_block_time(
        cluster, upgrade_time.replace(tzinfo=timezone.utc) + timedelta(seconds=1)
    )


def test_manual_export(cosmovisor_cluster):
    """
    - do chain state export, override the genesis time to the genesis file
    - ,and reset the data set
    - see https://github.com/crypto-org-chain/chain-main/issues/289
    """

    cluster = cosmovisor_cluster
    edit_chain_program(
        cluster.chain_id,
        cluster.data_dir / SUPERVISOR_CONFIG_FILE,
        lambda i, _: {
            "command": f"%(here)s/node{i}/cosmovisor/genesis/bin/chain-maind start "
            f"--home %(here)s/node{i}"
        },
    )

    cluster.reload_supervisor()
    wait_for_port(rpc_port(cluster.config["validators"][0]["base_port"]))
    # wait for a new block to make sure chain started up
    wait_for_new_blocks(cluster, 1)
    cluster.supervisor.stopAllProcesses()

    # check the state of all nodes should be stopped
    for info in cluster.supervisor.getAllProcessInfo():
        assert info["statename"] == "STOPPED"

    # export the state
    cluster.cmd = (
        cluster.data_root
        / cluster.chain_id
        / "node0/cosmovisor/genesis/bin/chain-maind"
    )
    cluster.cosmos_cli(0).export()

    # update the genesis time = current time + 5 secs
    newtime = datetime.utcnow() + timedelta(seconds=5)
    cluster.config["genesis-time"] = newtime.replace(tzinfo=None).isoformat("T") + "Z"

    for i in range(cluster.nodes_len()):
        migrate_genesis_time(cluster, i)
        cluster.validate_genesis(i)
        cluster.cosmos_cli(i).unsaferesetall()

    cluster.supervisor.startAllProcesses()

    wait_for_new_blocks(cluster, 1)

    cluster.supervisor.stopAllProcesses()

    # check the state of all nodes should be stopped
    for info in cluster.supervisor.getAllProcessInfo():
        assert info["statename"] == "STOPPED"
