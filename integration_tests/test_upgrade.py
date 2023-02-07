import configparser
import json
import re
import subprocess
import time
from datetime import datetime, timedelta
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
    reg = re.compile(rf"^program:{chain_id}-node(\d+)")
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
        Path(__file__).parent / "configs/default.jsonnet",
        worker_index,
        data,
        post_init=post_init,
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
    rsp = cluster.gov_propose_legacy(
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
    assert ev["proposal_messages"] == ",/cosmos.gov.v1.MsgExecLegacyContent", rsp
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


def propose_and_pass(cluster, kind, title, proposal, cosmos_sdk_46=True, **kwargs):
    if cosmos_sdk_46:
        rsp = cluster.gov_propose_legacy(
            "community",
            kind,
            proposal,
            **kwargs,
        )
    else:
        rsp = cluster.gov_propose(
            "community",
            kind,
            proposal,
            **kwargs,
        )
    assert rsp["code"] == 0, rsp["raw_log"]

    # get proposal_id
    ev = parse_events(rsp["logs"])["submit_proposal"]
    if cosmos_sdk_46:
        assert ev["proposal_messages"] == ",/cosmos.gov.v1.MsgExecLegacyContent", rsp
    else:
        assert ev["proposal_type"] == "SoftwareUpgrade", rsp
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


def upgrade(cluster, plan_name, target_height, cosmos_sdk_46=True):
    print("upgrade height", target_height)

    propose_and_pass(
        cluster,
        "software-upgrade",
        "SoftwareUpgrade",
        {
            "name": plan_name,
            "title": "upgrade test",
            "description": "ditto",
            "upgrade-height": target_height,
            "deposit": "0.1cro",
        },
        cosmos_sdk_46,
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
                f"%(here)s/node{i}/cosmovisor/upgrades/{plan_name}/bin/chain-maind "
                f"start --home %(here)s/node{i}"
            )
        },
    )
    cluster.reload_supervisor()

    # update the cli cmd to correct binary
    cluster.cmd = cluster.data_root / f"cosmovisor/upgrades/{plan_name}/bin/chain-maind"

    # wait for it to generate new blocks
    wait_for_block(cluster, target_height + 2, 600)


@pytest.mark.skip(reason="tested in test_manual_upgrade_all")
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

    upgrade(cluster, "v2.0.0", target_height, cosmos_sdk_46=False)


def test_manual_upgrade_all(cosmovisor_cluster):
    test_manual_upgrade(cosmovisor_cluster)
    cluster = cosmovisor_cluster
    cli = cluster.cosmos_cli()

    [validator1_operator_address, validator2_operator_address] = list(
        map(
            lambda i: i["operator_address"],
            sorted(
                cluster.validators(),
                key=lambda i: i["commission"]["commission_rates"]["rate"],
            ),
        ),
    )
    default_rate = "0.100000000000000000"

    def assert_commission(adr, expected):

        rsp = json.loads(
            cli.raw(
                "query",
                "staking",
                "validator",
                f"{adr}",
                home=cli.data_dir,
                node=cli.node_rpc,
                output="json",
            )
        )
        rate = rsp["commission"]["commission_rates"]["rate"]
        print(f"{adr} commission", rate)
        assert rate == expected, rsp

    assert_commission(validator1_operator_address, "0.000000000000000000")
    assert_commission(validator2_operator_address, default_rate)

    community_addr = cluster.address("community")
    reserve_addr = cluster.address("reserve")
    # for the fee payment
    cluster.transfer(community_addr, reserve_addr, "10000basecro")

    signer1_address = cluster.address("reserve", i=0)
    staking_validator1 = cluster.validator(validator1_operator_address, i=0)
    assert validator1_operator_address == staking_validator1["operator_address"]
    staking_validator2 = cluster.validator(validator2_operator_address, i=1)
    assert validator2_operator_address == staking_validator2["operator_address"]
    old_bonded = cluster.staking_pool()
    rsp = cluster.delegate_amount(
        validator1_operator_address,
        "2009999498basecro",
        signer1_address,
        0,
        "0.025basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cluster.staking_pool() == old_bonded + 2009999498
    rsp = cluster.delegate_amount(
        validator2_operator_address, "1basecro", signer1_address, 0, "0.025basecro"
    )
    # vesting bug
    assert rsp["code"] != 0, rsp["raw_log"]
    assert cluster.staking_pool() == old_bonded + 2009999498

    target_height = cluster.block_height() + 30
    upgrade(cluster, "v3.0.0", target_height, cosmos_sdk_46=False)

    rsp = cluster.delegate_amount(
        validator2_operator_address, "1basecro", signer1_address, 0, "0.025basecro"
    )
    # vesting bug fixed
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cluster.staking_pool() == old_bonded + 2009999499

    assert_commission(validator1_operator_address, "0.000000000000000000")
    assert_commission(validator2_operator_address, default_rate)

    # create denom before upgrade
    cli = cluster.cosmos_cli()
    rsp = json.loads(
        cli.raw(
            "tx",
            "nft",
            "issue",
            "testdenomid",
            "-y",
            name="testdenomname",
            home=cli.data_dir,
            node=cli.node_rpc,
            output="json",
            from_="community",
            keyring_backend="test",
            chain_id=cli.chain_id,
        )
    )
    raw_log = json.loads(rsp["raw_log"])
    assert raw_log[0]["events"][0]["type"] == "issue_denom"

    target_height = cluster.block_height() + 30
    upgrade(cluster, "v4.2.0", target_height, cosmos_sdk_46=False)

    cli = cluster.cosmos_cli()

    # check denom after upgrade
    rsp = json.loads(
        cli.raw(
            "query",
            "nft",
            "denom",
            "testdenomid",
            home=cli.data_dir,
            node=cli.node_rpc,
            output="json",
        )
    )

    assert rsp["name"] == "testdenomname", rsp
    assert rsp["uri"] == "", rsp

    # check icaauth params
    rsp = json.loads(
        cli.raw(
            "query",
            "icaauth",
            "params",
            home=cli.data_dir,
            node=cli.node_rpc,
            output="json",
        )
    )

    assert rsp["params"]["minTimeoutDuration"] == "3600s", rsp
    # check min commission
    rsp = json.loads(
        cli.raw(
            "query",
            "staking",
            "params",
            home=cli.data_dir,
            node=cli.node_rpc,
            output="json",
        )
    )
    print("min commission", rsp["min_commission_rate"])
    min_commission_rate = "0.050000000000000000"
    assert rsp["min_commission_rate"] == min_commission_rate, rsp

    assert_commission(validator1_operator_address, min_commission_rate)
    assert_commission(validator2_operator_address, default_rate)


def test_cancel_upgrade(cluster):
    """
    use default cluster
    - propose upgrade and pass it
    - cancel the upgrade before execution
    """
    plan_name = "upgrade-test"
    time.sleep(5)  # FIXME the port seems still exists for a while after process stopped
    wait_for_port(rpc_port(cluster.config["validators"][0]["base_port"]))
    upgrade_height = cluster.block_height() + 30
    print("propose upgrade plan")
    print("upgrade height", upgrade_height)
    propose_and_pass(
        cluster,
        "software-upgrade",
        "SoftwareUpgrade",
        {
            "name": plan_name,
            "title": "upgrade test",
            "description": "ditto",
            "upgrade-height": upgrade_height,
            "deposit": "0.1cro",
        },
        no_validate=True,
    )

    print("cancel upgrade plan")
    propose_and_pass(
        cluster,
        "cancel-software-upgrade",
        "CancelSoftwareUpgrade",
        {
            "title": "there is bug, cancel upgrade",
            "description": "there is bug, cancel upgrade",
            "deposit": "0.1cro",
        },
        no_validate=True,
    )

    # wait for blocks after upgrade, should success since upgrade is canceled
    wait_for_block(cluster, upgrade_height + 2)


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
        cluster.validate_genesis()
        cluster.cosmos_cli(i).unsaferesetall()

    cluster.supervisor.startAllProcesses()

    wait_for_new_blocks(cluster, 1)

    cluster.supervisor.stopAllProcesses()

    # check the state of all nodes should be stopped
    for info in cluster.supervisor.getAllProcessInfo():
        assert info["statename"] == "STOPPED"
