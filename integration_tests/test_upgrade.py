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
    approve_proposal,
    assert_gov_params,
    cluster_fixture,
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


def find_log_event_attrs_legacy(logs, ev_type, cond=None):
    for ev in logs[0]["events"]:
        if ev["type"] == ev_type:
            attrs = {attr["key"]: attr["value"] for attr in ev["attributes"]}
            if cond is None or cond(attrs):
                return attrs
    return None


def get_proposal_id_legacy(rsp, msg=",/cosmos.gov.v1.MsgExecLegacyContent"):
    def cb(attrs):
        return "proposal_id" in attrs

    ev = find_log_event_attrs_legacy(rsp["logs"], "submit_proposal", cb)
    assert ev["proposal_messages"] == msg, rsp
    return ev["proposal_id"]


# use function scope to re-initialize for each test case
@pytest.fixture(scope="function")
def cosmovisor_cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    data = tmp_path_factory.mktemp("data")
    init_cosmovisor(data)
    yield from cluster_fixture(
        Path(__file__).parent / "configs/cosmovisor.jsonnet",
        worker_index,
        data,
        post_init=post_init,
        cmd=str(data / "cosmovisor/genesis/bin/chain-maind"),
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
    ev = find_log_event_attrs_legacy(rsp["logs"], "submit_proposal")
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


def upgrade(
    cluster,
    plan_name,
    target_height,
    propose_legacy=True,
    broadcast_mode="block",
):
    print("upgrade height", target_height, plan_name)
    kind = "software-upgrade"
    proposal = {
        "name": plan_name,
        "title": "upgrade test",
        "description": "ditto",
        "upgrade-height": target_height,
        "deposit": "0.1cro",
    }
    event_query_tx = broadcast_mode == "sync"
    if propose_legacy:
        rsp = cluster.gov_propose_legacy(
            "community",
            kind,
            proposal,
            no_validate=True,
            event_query_tx=event_query_tx,
            broadcast_mode=broadcast_mode,
        )
    else:
        rsp = cluster.gov_propose(
            "community",
            kind,
            proposal,
        )
    assert rsp["code"] == 0, rsp["raw_log"]
    # get proposal_id
    if propose_legacy:
        proposal_id = get_proposal_id_legacy(rsp)
    else:
        ev = find_log_event_attrs_legacy(rsp["logs"], "submit_proposal")
        assert ev["proposal_type"] == "SoftwareUpgrade", rsp
        proposal_id = ev["proposal_id"]
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_VOTING_PERIOD", proposal
    for i in range(2):
        rsp = cluster.gov_vote(
            "validator",
            proposal_id,
            "yes",
            i=i,
            event_query_tx=event_query_tx,
            broadcast_mode=broadcast_mode,
        )
        assert rsp["code"] == 0, rsp["raw_log"]

    proposal = cluster.query_proposal(proposal_id)
    wait_for_block_time(
        cluster, isoparse(proposal["voting_end_time"]) + timedelta(seconds=1)
    )
    proposal = cluster.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal

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
    js1 = json.load((cluster.home(0) / "data/upgrade-info.json").open())
    js2 = json.load((cluster.home(1) / "data/upgrade-info.json").open())
    expected = {
        "name": plan_name,
        "height": target_height,
    }
    assert js1 == js2
    assert expected.items() <= js1.items()

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


def test_manual_upgrade_all(cosmovisor_cluster):
    # test_manual_upgrade(cosmovisor_cluster)
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

    upgrade(cluster, "v2.0.0", target_height, propose_legacy=False)
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
        # assert rate == expected, rsp

    assert_commission(validator1_operator_address, "0.000000000000000000")
    assert_commission(validator2_operator_address, default_rate)

    community_addr = cluster.address("community")
    reserve_addr = cluster.address("reserve")
    # for the fee payment
    cluster.transfer(community_addr, reserve_addr, "10000basecro", event_query_tx=False)

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
        event_query_tx=False,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cluster.staking_pool() == old_bonded + 2009999498
    rsp = cluster.delegate_amount(
        validator2_operator_address,
        "1basecro",
        signer1_address,
        0,
        "0.025basecro",
        event_query_tx=False,
    )
    # vesting bug
    assert rsp["code"] != 0, rsp["raw_log"]
    assert cluster.staking_pool() == old_bonded + 2009999498

    target_height = cluster.block_height() + 15
    upgrade(cluster, "v3.0.0", target_height, propose_legacy=False)

    rsp = cluster.delegate_amount(
        validator2_operator_address,
        "1basecro",
        signer1_address,
        0,
        "0.025basecro",
        event_query_tx=False,
    )
    # vesting bug fixed
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cluster.staking_pool() == old_bonded + 2009999499

    assert_commission(validator1_operator_address, "0.000000000000000000")
    assert_commission(validator2_operator_address, default_rate)

    # create denom before upgrade
    cli = cluster.cosmos_cli()
    denomid = "testdenomid"
    denomname = "testdenomname"
    creator = cluster.address("community")
    rsp = cluster.create_nft(creator, denomid, denomname, event_query_tx=False)
    ev = find_log_event_attrs_legacy(rsp["logs"], "issue_denom")
    assert ev == {
        "denom_id": denomid,
        "denom_name": denomname,
        "creator": creator,
    }, ev

    target_height = cluster.block_height() + 15
    upgrade(cluster, "v4.2.0", target_height, propose_legacy=False)

    cli = cluster.cosmos_cli()

    # check denom after upgrade
    rsp = cluster.query_nft(denomid)
    assert rsp["name"] == denomname, rsp
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

    target_height = cluster.block_height() + 15
    # test migrate keystore
    for i in range(2):
        cluster.migrate_keystore(i=i)
    upgrade(cluster, "v4.3.0", target_height)
    cli = cluster.cosmos_cli()

    target_height = cluster.block_height() + 15
    upgrade(cluster, "v5.0", target_height, broadcast_mode="sync")
    cli = cluster.cosmos_cli()

    acct = cli.account("cro1jgt29q28ehyc6p0fd5wqhwswfxv59lhppz3v65")
    assert acct["@type"] == "/cosmos.vesting.v1beta1.PeriodicVestingAccount"
    print(acct)
    vesting_acct = acct["base_vesting_account"]
    assert vesting_acct["original_vesting"] == [
        {"denom": "basecro", "amount": "7000000000000000000"}
    ]
    assert len(acct["vesting_periods"]) == 60
    for period in acct["vesting_periods"][:-1]:
        assert period == {
            "length": "60",
            "amount": [{"denom": "basecro", "amount": "116666666666666666"}],
        }
    assert acct["vesting_periods"][-1] == {
        "length": "60",
        "amount": [{"denom": "basecro", "amount": "116666666666666706"}],
    }

    params = json.loads(
        cli.raw("query", "mint", "params", output="json", node=cli.node_rpc)
    )
    assert params["inflation_max"] == "0.010000000000000000"
    assert params["inflation_min"] == "0.008500000000000000"

    target_height = cluster.block_height() + 15
    gov_param = cli.query_params("gov")
    upgrade(cluster, "v6.0", target_height, broadcast_mode="sync")
    cli = cluster.cosmos_cli()
    with pytest.raises(AssertionError):
        cli.query_params("icaauth")
    assert_gov_params(cli, gov_param)


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
    print("propose upgrade plan at", upgrade_height)
    rsp = cluster.gov_propose_new(
        "community",
        "software-upgrade",
        {
            "name": plan_name,
            "title": "upgrade test",
            "summary": "summary",
            "upgrade-height": upgrade_height,
            "deposit": "0.1cro",
        },
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(cluster, rsp, msg=",/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade")

    print("cancel upgrade plan")
    rsp = cluster.gov_propose_new(
        "community",
        "cancel-software-upgrade",
        {
            "title": "there is bug, cancel upgrade",
            "summary": "summary",
            "deposit": "0.1cro",
        },
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(cluster, rsp, msg=",/cosmos.upgrade.v1beta1.MsgCancelUpgrade")

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
