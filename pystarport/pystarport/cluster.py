import base64
import configparser
import datetime
import hashlib
import json
import os
import re
import subprocess
import sys
import threading
import time
from pathlib import Path
from typing import List

import durations
import jsonmerge
import multitail2
import tomlkit
import yaml
from dateutil.parser import isoparse
from supervisor import xmlrpc
from supervisor.compat import xmlrpclib

from . import ports
from .app import CHAIN, IMAGE, SUPERVISOR_CONFIG_FILE
from .cosmoscli import ChainCommand, CosmosCLI, ModuleAccount, module_address
from .ledger import ZEMU_BUTTON_PORT, ZEMU_HOST
from .utils import format_doc_string, interact, write_ini

COMMON_PROG_OPTIONS = {
    # redirect to supervisord's stdout, easier to collect all logs
    "autostart": "true",
    "autorestart": "true",
    "redirect_stderr": "true",
    "startsecs": "3",
}


def home_dir(data_dir, i):
    return data_dir / f"node{i}"


class ClusterCLI:
    "the apis to interact with wallet and blockchain prepared with Cluster"

    def __init__(
        self,
        data,
        chain_id="chainmaind",
        cmd=CHAIN,
        zemu_address=ZEMU_HOST,
        zemu_button_port=ZEMU_BUTTON_PORT,
    ):
        self.data_root = data
        self.cmd = cmd
        self.zemu_address = zemu_address
        self.zemu_button_port = zemu_button_port
        self.chain_id = chain_id
        self.data_dir = data / self.chain_id
        self.config = json.load((self.data_dir / "config.json").open())

        self._supervisorctl = None
        self.output = None
        self.error = None

    def cosmos_cli(self, i=0):
        return CosmosCLI(
            self.home(i),
            self.node_rpc(i),
            chain_id=self.chain_id,
            cmd=self.cmd,
            zemu_address=self.zemu_address,
            zemu_button_port=self.zemu_button_port,
        )

    @property
    def supervisor(self):
        "http://supervisord.org/api.html"
        # copy from:
        # https://github.com/Supervisor/supervisor/blob/76df237032f7d9fbe80a0adce3829c8b916d5b58/supervisor/options.py#L1718
        if self._supervisorctl is None:
            self._supervisorctl = xmlrpclib.ServerProxy(
                # dumbass ServerProxy won't allow us to pass in a non-HTTP url,
                # so we fake the url we pass into it and
                # always use the transport's
                # 'serverurl' to figure out what to attach to
                "http://127.0.0.1",
                transport=xmlrpc.SupervisorTransport(
                    serverurl=f"unix://{self.data_root}/supervisor.sock"
                ),
            )
        return self._supervisorctl.supervisor

    def reload_supervisor(self):
        subprocess.run(
            [
                sys.executable,
                "-msupervisor.supervisorctl",
                "-c",
                self.data_root / SUPERVISOR_CONFIG_FILE,
                "update",
            ],
            check=True,
        )

    def nodes_len(self):
        "find how many 'node{i}' sub-directories"
        return len(
            [p for p in self.data_dir.iterdir() if re.match(r"^node\d+$", p.name)]
        )

    def copy_validator_key(self, from_node=1, to_node=2):
        "Copy the validtor file in from_node to to_node"
        from_key_file = "{}/node{}/config/priv_validator_key.json".format(
            self.data_dir, from_node
        )
        to_key_file = "{}/node{}/config/priv_validator_key.json".format(
            self.data_dir, to_node
        )
        with open(from_key_file, "r") as f:
            key = f.read()
        with open(to_key_file, "w") as f:
            f.write(key)

    def update_genesis(self, i, genesis_data):
        home = self.home(i)
        genesis_file = home / "config/genesis.json"
        with open(genesis_file, "w") as f:
            f.write(json.dumps(genesis_data, indent=4))

    def stop_node(self, i=0):
        subprocess.run(
            [
                sys.executable,
                "-msupervisor.supervisorctl",
                "-c",
                self.data_root / SUPERVISOR_CONFIG_FILE,
                "stop",
                "{}-node{}".format(self.chain_id, i),
            ]
        )

    def stop_relayer(self, path):
        subprocess.run(
            [
                sys.executable,
                "-msupervisor.supervisorctl",
                "-c",
                self.data_root / SUPERVISOR_CONFIG_FILE,
                "stop",
                "program:relayer-{}".format(path),
            ]
        )

    def restart_relayer(self, path):
        subprocess.run(
            [
                sys.executable,
                "-msupervisor.supervisorctl",
                "-c",
                self.data_root / SUPERVISOR_CONFIG_FILE,
                "restart",
                "program:relayer-{}".format(path),
            ]
        )

    def start_node(self, i):
        subprocess.run(
            [
                sys.executable,
                "-msupervisor.supervisorctl",
                "-c",
                self.data_root / SUPERVISOR_CONFIG_FILE,
                "start",
                "{}-node{}".format(self.chain_id, i),
            ]
        )

    def create_node(
        self,
        base_port=None,
        moniker=None,
        hostname="localhost",
        statesync=False,
        mnemonic=None,
    ):
        """create new node in the data directory,
        process information is written into supervisor config
        start it manually with supervisor commands

        :return: new node index and config
        """
        i = self.nodes_len()

        # default configs
        if base_port is None:
            # use the node0's base_port + i * 10 as default base port for new ndoe
            base_port = self.config["validators"][0]["base_port"] + i * 10
        if moniker is None:
            moniker = f"node{i}"

        # add config
        assert len(self.config["validators"]) == i
        self.config["validators"].append(
            {
                "base_port": base_port,
                "hostname": hostname,
                "moniker": moniker,
            }
        )
        (self.data_dir / "config.json").write_text(json.dumps(self.config))

        # init home directory
        self.init(i)
        home = self.home(i)
        (home / "config/genesis.json").unlink()
        (home / "config/genesis.json").symlink_to("../../genesis.json")
        # use p2p peers from node0's config
        node0 = tomlkit.parse((self.data_dir / "node0/config/config.toml").read_text())

        def custom_edit_tm(doc):
            if statesync:
                info = self.status()["SyncInfo"]
                doc["statesync"].update(
                    {
                        "enable": True,
                        "rpc_servers": ",".join(self.node_rpc(i) for i in range(2)),
                        "trust_height": int(info["earliest_block_height"]),
                        "trust_hash": info["earliest_block_hash"],
                        "temp_dir": str(self.data_dir),
                        "discovery_time": "5s",
                    }
                )

        edit_tm_cfg(
            home / "config/config.toml",
            base_port,
            node0["p2p"]["persistent_peers"],
            custom_edit=custom_edit_tm,
        )
        edit_app_cfg(home / "config/app.toml", base_port)

        # create validator account
        self.create_account("validator", i, mnemonic)

        # add process config into supervisor
        path = self.data_dir / SUPERVISOR_CONFIG_FILE
        ini = configparser.RawConfigParser()
        ini.read_file(path.open())
        chain_id = self.config["chain_id"]
        prgname = f"{chain_id}-node{i}"
        section = f"program:{prgname}"
        ini.add_section(section)
        ini[section].update(
            dict(
                COMMON_PROG_OPTIONS,
                command=f"{self.cmd} start --home %(here)s/node{i}",
                autostart="false",
                stdout_logfile=f"%(here)s/node{i}.log",
            )
        )
        with path.open("w") as fp:
            ini.write(fp)
        self.reload_supervisor()
        return i

    def home(self, i):
        "home directory of i-th node"
        return home_dir(self.data_dir, i)

    def base_port(self, i):
        return self.config["validators"][i]["base_port"]

    def node_rpc(self, i):
        "rpc url of i-th node"
        return "tcp://127.0.0.1:%d" % ports.rpc_port(self.base_port(i))

    # for query
    def ipport_grpc(self, i):
        "grpc url of i-th node"
        return "127.0.0.1:%d" % ports.grpc_port(self.base_port(i))

    # tx broadcast only
    def ipport_grpc_tx(self, i):
        "grpc url of i-th node"
        return "127.0.0.1:%d" % ports.grpc_port_tx_only(self.base_port(i))

    def node_id(self, i):
        "get i-th node's tendermint node id"
        return self.cosmos_cli(i).node_id()

    def delete_account(self, name, i=0):
        "delete account in i-th node's keyring"
        return self.cosmos_cli(i).delete_account(name)

    def create_account(self, name, i=0, mnemonic=None):
        "create new keypair in i-th node's keyring"
        return self.cosmos_cli(i).create_account(name, mnemonic)

    def create_account_ledger(self, name, i=0):
        "create new ledger keypair"
        return self.cosmos_cli(i).create_account_ledger(name)

    def init(self, i):
        "the i-th node's config is already added"
        return self.cosmos_cli(i).init(self.config["validators"][i]["moniker"])

    def export(self, i=0):
        return self.cosmos_cli(i).export()

    def validate_genesis(self, i=0):
        return self.cosmos_cli(i).validate_genesis()

    def add_genesis_account(self, addr, coins, i=0, **kwargs):
        return self.cosmos_cli(i).add_genesis_account(addr, coins, **kwargs)

    def gentx(self, name, coins, i=0, min_self_delegation=1, pubkey=None):
        return self.cosmos_cli(i).gentx(name, coins, min_self_delegation, pubkey)

    def collect_gentxs(self, gentx_dir, i=0):
        return self.cosmos_cli(i).collect_gentxs(gentx_dir)

    def status(self, i=0):
        return self.cosmos_cli(i).status()

    def block_height(self, i=0):
        return self.cosmos_cli(i).block_height()

    def block_time(self, i=0):
        return self.cosmos_cli(i).block_time()

    def balance(self, addr, i=0):
        return self.cosmos_cli(i).balance(addr)

    def query_all_txs(self, addr, i=0):
        return self.cosmos_cli(i).query_all_txs(addr)

    def distribution_commission(self, addr, i=0):
        return self.cosmos_cli(i).distribution_commission(addr)

    def distribution_community(self, i=0):
        return self.cosmos_cli(i).distribution_community()

    def distribution_reward(self, delegator_addr, i=0):
        return self.cosmos_cli(i).distribution_reward(delegator_addr)

    def address(self, name, i=0, bech="acc"):
        return self.cosmos_cli(i).address(name, bech)

    @format_doc_string(
        options=",".join(v.value for v in ModuleAccount.__members__.values())
    )
    def module_address(self, name):
        """
        get address of module accounts

        :param name: name of module account, values: {options}
        """
        return module_address(name)

    def account(self, addr, i=0):
        return self.cosmos_cli(i).account(addr)

    def supply(self, supply_type, i=0):
        return self.cosmos_cli(i).supply(supply_type)

    def validator(self, addr, i=0):
        return self.cosmos_cli(i).validator(addr)

    def validators(self, i=0):
        return self.cosmos_cli(i).validators()

    def staking_params(self, i=0):
        return self.cosmos_cli(i).staking_params()

    def staking_pool(self, bonded=True, i=0):
        return self.cosmos_cli(i).staking_pool(bonded)

    def transfer_offline(self, from_, to, coins, sequence, i=0, fees=None):
        return self.cosmos_cli(i).transfer_offline(from_, to, coins, sequence, fees)

    def transfer(self, from_, to, coins, i=0, generate_only=False, fees=None):
        return self.cosmos_cli(i).transfer(from_, to, coins, generate_only, fees)

    def transfer_from_ledger(
        self, from_, to, coins, i=0, generate_only=False, fees=None
    ):
        return self.cosmos_cli(i).transfer_from_ledger(
            from_,
            to,
            coins,
            generate_only,
            fees,
        )

    def get_delegated_amount(self, which_addr, i=0):
        return self.cosmos_cli(i).get_delegated_amount(which_addr)

    def delegate_amount(self, to_addr, amount, from_addr, i=0):
        return self.cosmos_cli(i).delegate_amount(to_addr, amount, from_addr)

    # to_addr: croclcl1...  , from_addr: cro1...
    def unbond_amount(self, to_addr, amount, from_addr, i=0):
        return self.cosmos_cli(i).unbond_amount(to_addr, amount, from_addr)

    # to_validator_addr: crocncl1...  ,  from_from_validator_addraddr: crocl1...
    def redelegate_amount(
        self, to_validator_addr, from_validator_addr, amount, from_addr, i=0
    ):
        return self.cosmos_cli(i).redelegate_amount(
            to_validator_addr,
            from_validator_addr,
            amount,
            from_addr,
        )

    def withdraw_all_rewards(self, from_delegator, i=0):
        return self.cosmos_cli(i).withdraw_all_rewards(from_delegator)

    def make_multisig(self, name, signer1, signer2, i=0):
        return self.cosmos_cli(i).make_multisig(name, signer1, signer2)

    def sign_multisig_tx(self, tx_file, multi_addr, signer_name, i=0):
        return self.cosmos_cli(i).sign_multisig_tx(tx_file, multi_addr, signer_name)

    def sign_batch_multisig_tx(
        self, tx_file, multi_addr, signer_name, account_num, sequence, i=0
    ):
        return self.cosmos_cli(i).sign_batch_multisig_tx(
            tx_file, multi_addr, signer_name, account_num, sequence
        )

    def encode_signed_tx(self, signed_tx, i=0):
        return self.cosmos_cli(i).encode_signed_tx(signed_tx)

    def sign_single_tx(self, tx_file, signer_name, i=0):
        return self.cosmos_cli(i).sign_single_tx(tx_file, signer_name)

    def combine_multisig_tx(self, tx_file, multi_name, signer1_file, signer2_file, i=0):
        return self.cosmos_cli(i).combine_multisig_tx(
            tx_file,
            multi_name,
            signer1_file,
            signer2_file,
        )

    def combine_batch_multisig_tx(
        self, tx_file, multi_name, signer1_file, signer2_file, i=0
    ):
        return self.cosmos_cli(i).combine_batch_multisig_tx(
            tx_file,
            multi_name,
            signer1_file,
            signer2_file,
        )

    def broadcast_tx(self, tx_file, i=0):
        return self.cosmos_cli(i).broadcast_tx(tx_file)

    def unjail(self, addr, i=0):
        return self.cosmos_cli(i).unjail(addr)

    def create_validator(
        self,
        amount,
        i,
        moniker=None,
        commission_max_change_rate="0.01",
        commission_rate="0.1",
        commission_max_rate="0.2",
        min_self_delegation="1",
        identity="",
        website="",
        security_contact="",
        details="",
    ):
        """MsgCreateValidator
        create the node with create_node before call this"""
        return self.cosmos_cli(i).create_validator(
            amount,
            moniker or self.config["validators"][i]["moniker"],
            commission_max_change_rate,
            commission_rate,
            commission_max_rate,
            min_self_delegation,
            identity,
            website,
            security_contact,
            details,
        )

    def edit_validator(
        self,
        i,
        commission_rate=None,
        moniker=None,
        identity=None,
        website=None,
        security_contact=None,
        details=None,
    ):
        """MsgEditValidator"""
        return self.cosmos_cli(i).edit_validator(
            commission_rate,
            moniker,
            identity,
            website,
            security_contact,
            details,
        )

    def gov_propose(self, proposer, kind, proposal, i=0):
        return self.cosmos_cli(i).gov_propose(proposer, kind, proposal)

    def gov_vote(self, voter, proposal_id, option, i=0):
        return self.cosmos_cli(i).gov_vote(voter, proposal_id, option)

    def gov_deposit(self, depositor, proposal_id, amount, i=0):
        return self.cosmos_cli(i).gov_deposit(depositor, proposal_id, amount)

    def query_proposals(self, depositor=None, limit=None, status=None, voter=None, i=0):
        return self.cosmos_cli(i).query_proposals(depositor, limit, status, voter)

    def query_proposal(self, proposal_id, i=0):
        return self.cosmos_cli(i).query_proposal(proposal_id)

    def query_tally(self, proposal_id, i=0):
        return self.cosmos_cli(i).query_tally(proposal_id)

    def ibc_transfer(
        self,
        from_,
        to,
        amount,
        channel,  # src channel
        target_version,  # chain version number of target chain
        i=0,
    ):
        return self.cosmos_cli(i).ibc_transfer(
            from_,
            to,
            amount,
            channel,
            target_version,
        )


def start_cluster(data_dir):
    cmd = [
        sys.executable,
        "-msupervisor.supervisord",
        "-c",
        data_dir / SUPERVISOR_CONFIG_FILE,
    ]
    return subprocess.Popen(cmd, env=dict(os.environ, PYTHONPATH=":".join(sys.path)))


class TailLogsThread(threading.Thread):
    def __init__(self, base_dir, pats: List[str]):
        self.base_dir = base_dir
        self.tailer = multitail2.MultiTail([str(base_dir / pat) for pat in pats])
        self._stop_event = threading.Event()
        super().__init__()

    def run(self):
        while not self.stopped:
            for (path, _), s in self.tailer.poll():
                print(Path(path).relative_to(self.base_dir), s)

            # TODO Replace this with FAM/inotify for watching filesystem events.
            time.sleep(0.5)

    def stop(self):
        self._stop_event.set()

    @property
    def stopped(self):
        return self._stop_event.is_set()


def start_tail_logs_thread(data_dir):
    t = TailLogsThread(data_dir, ["*/node*.log", "relayer-*.log"])
    t.start()
    return t


def process_config(config, base_port):
    """
    fill default values in config
    """
    for i, val in enumerate(config["validators"]):
        if "moniker" not in val:
            val["moniker"] = f"node{i}"
        if "base_port" not in val:
            val["base_port"] = base_port + i * 10
        if "hostname" not in val:
            val["hostname"] = "localhost"


def init_devnet(
    data_dir,
    config,
    base_port,
    image=IMAGE,
    cmd=None,
    gen_compose_file=False,
):
    """
    init data directory
    """

    def create_account(cli, account, use_ledger=False):
        if use_ledger:
            acct = cli.create_account_ledger(account["name"])
        else:
            acct = cli.create_account(account["name"])
        vesting = account.get("vesting")
        if not vesting:
            cli.add_genesis_account(acct["address"], account["coins"])
        else:
            genesis_time = isoparse(genesis["genesis_time"])
            end_time = genesis_time + datetime.timedelta(
                seconds=durations.Duration(vesting).to_seconds()
            )
            vend = int(end_time.timestamp())
            cli.add_genesis_account(
                acct["address"],
                account["coins"],
                vesting_amount=account["coins"],
                vesting_end_time=vend,
            )
        return acct

    process_config(config, base_port)

    (data_dir / "config.json").write_text(json.dumps(config))

    cmd = config.get("cmd") or cmd or CHAIN

    # init home directories
    for i, val in enumerate(config["validators"]):
        ChainCommand(cmd)(
            "init",
            val["moniker"],
            chain_id=config["chain_id"],
            home=home_dir(data_dir, i),
        )
        if "consensus_key" in val:
            # restore consensus private key
            with (home_dir(data_dir, i) / "config/priv_validator_key.json").open(
                "w"
            ) as fp:
                json.dump(
                    {
                        "address": hashlib.sha256(
                            base64.b64decode(val["consensus_key"]["pub"])
                        )
                        .hexdigest()[:40]
                        .upper(),
                        "pub_key": {
                            "type": "tendermint/PubKeyEd25519",
                            "value": val["consensus_key"]["pub"],
                        },
                        "priv_key": {
                            "type": "tendermint/PrivKeyEd25519",
                            "value": val["consensus_key"]["priv"],
                        },
                    },
                    fp,
                )
    if "genesis_file" in config:
        genesis_bytes = open(
            config["genesis_file"] % {"here": Path(config["path"]).parent}, "rb"
        ).read()
    else:
        genesis_bytes = (data_dir / "node0/config/genesis.json").read_bytes()
    (data_dir / "genesis.json").write_bytes(genesis_bytes)
    (data_dir / "gentx").mkdir()
    for i in range(len(config["validators"])):
        try:
            (data_dir / f"node{i}/config/genesis.json").unlink()
        except OSError:
            pass
        (data_dir / f"node{i}/config/genesis.json").symlink_to("../../genesis.json")
        (data_dir / f"node{i}/config/gentx").symlink_to("../../gentx")

    # now we can create ClusterCLI
    cli = ClusterCLI(data_dir.parent, chain_id=config["chain_id"], cmd=cmd)

    # patch the genesis file
    genesis = jsonmerge.merge(
        json.load(open(data_dir / "genesis.json")),
        config.get("genesis", {}),
    )
    (data_dir / "genesis.json").write_text(json.dumps(genesis))
    cli.validate_genesis()

    # create accounts
    accounts = []
    for i, node in enumerate(config["validators"]):
        mnemonic = node.get("mnemonic")
        account = cli.create_account("validator", i, mnemonic=mnemonic)
        accounts.append(account)
        if "coins" in node:
            cli.add_genesis_account(account["address"], node["coins"], i)
        if "staked" in node:
            cli.gentx(
                "validator",
                node["staked"],
                i=i,
                min_self_delegation=node.get("min_self_delegation", 1),
                pubkey=node.get("pubkey"),
            )

    # create accounts
    for account in config.get("accounts", []):
        account = create_account(cli, account)
        accounts.append(account)

    account_hw = config.get("hw_account")
    if account_hw:
        account = create_account(cli, account_hw, True)
        accounts.append(account)

    # output accounts
    (data_dir / "accounts.json").write_text(json.dumps(accounts))

    # collect-gentxs if directory not empty
    if next((data_dir / "gentx").iterdir(), None) is not None:
        cli.collect_gentxs(data_dir / "gentx", 0)

    # realise the symbolic links, so the node directories can be used independently
    genesis_bytes = (data_dir / "genesis.json").read_bytes()
    for i in range(len(config["validators"])):
        (data_dir / f"node{i}/config/gentx").unlink()
        tmp = data_dir / f"node{i}/config/genesis.json"
        tmp.unlink()
        tmp.write_bytes(genesis_bytes)

    # write tendermint config
    peers = config.get("peers") or ",".join(
        [
            "tcp://%s@%s:%d"
            % (cli.node_id(i), val["hostname"], ports.p2p_port(val["base_port"]))
            for i, val in enumerate(config["validators"])
        ]
    )
    for i, val in enumerate(config["validators"]):
        edit_tm_cfg(data_dir / f"node{i}/config/config.toml", val["base_port"], peers)
        edit_app_cfg(
            data_dir / f"node{i}/config/app.toml",
            val["base_port"],
            val.get("minimum-gas-prices", ""),
        )

    # write supervisord config file
    with (data_dir / SUPERVISOR_CONFIG_FILE).open("w") as fp:
        write_ini(fp, supervisord_ini(cmd, config["validators"], config["chain_id"]))

    if gen_compose_file:
        yaml.dump(
            docker_compose_yml(cmd, config["validators"], data_dir, image),
            (data_dir / "docker-compose.yml").open("w"),
        )


def relayer_chain_config(data_dir, chain):
    cfg = json.load((data_dir / chain["chain_id"] / "config.json").open())
    rpc_port = ports.rpc_port(cfg["validators"][0]["base_port"])
    grpc_port = ports.grpc_port(cfg["validators"][0]["base_port"])
    return {
        "key_name": "relayer",
        "id": chain["chain_id"],
        "rpc_addr": f"http://localhost:{rpc_port}",
        "grpc_addr": f"http://localhost:{grpc_port}",
        "websocket_addr": f"ws://localhost:{rpc_port}/websocket",
        "rpc_timeout": "10s",
        "account_prefix": chain.get("account-prefix", "cro"),
        "store_prefix": "ibc",
        "gas": 300000,
        "fee_denom": "basecro",
        "fee_amount": 0,
        "trusting_period": "336h",
    }


def init_cluster(
    data_dir, config_path, base_port, image=IMAGE, cmd=None, gen_compose_file=False
):
    config = yaml.safe_load(open(config_path))

    # override relayer config in config.yaml
    rly_section = config.pop("relayer", {})
    for chain_id, cfg in config.items():
        cfg["path"] = str(config_path)
        cfg["chain_id"] = chain_id

    chains = list(config.values())
    for chain in chains:
        (data_dir / chain["chain_id"]).mkdir()
        init_devnet(
            data_dir / chain["chain_id"], chain, base_port, image, cmd, gen_compose_file
        )
    paths = rly_section.get("paths", {})
    with (data_dir / SUPERVISOR_CONFIG_FILE).open("w") as fp:
        write_ini(
            fp,
            supervisord_ini_group(config.keys(), paths),
        )
    if len(chains) > 1:
        relayer_config = data_dir / "relayer.toml"
        # write relayer config
        relayer_config.write_text(
            tomlkit.dumps(
                {
                    "global": {
                        "strategy": "naive",
                        "log_level": "info",
                    },
                    "chains": [
                        relayer_chain_config(data_dir, chain) for chain in chains
                    ],
                    "connections": [
                        {
                            "a_chain": path["src"]["chain-id"],
                            "b_chain": path["dst"]["chain-id"],
                            "paths": [
                                {
                                    "a_port": path["src"]["port-id"],
                                    "b_port": path["dst"]["port-id"],
                                }
                            ],
                        }
                        for path in paths.values()
                    ],
                }
            )
        )

        # restore the relayer account in relayer
        for chain in chains:
            mnemonic = find_account(data_dir, chain["chain_id"], "relayer")["mnemonic"]
            subprocess.run(
                [
                    "hermes",
                    "-c",
                    relayer_config,
                    "keys",
                    "restore",
                    chain["chain_id"],
                    "--mnemonic",
                    mnemonic,
                    "--coin-type",
                    str(chain.get("coin-type", 394)),
                ],
                check=True,
            )


def find_account(data_dir, chain_id, name):
    accounts = json.load((data_dir / chain_id / "accounts.json").open())
    return next(acct for acct in accounts if acct["name"] == name)


def supervisord_ini(cmd, validators, chain_id):
    ini = {}
    for i, node in enumerate(validators):
        ini[f"program:{chain_id}-node{i}"] = dict(
            COMMON_PROG_OPTIONS,
            command=f"{cmd} start --home %(here)s/node{i}",
            stdout_logfile=f"%(here)s/node{i}.log",
        )
    return ini


def supervisord_ini_group(chain_ids, paths):
    cfg = {
        "include": {
            "files": " ".join(
                f"%(here)s/{chain_id}/tasks.ini" for chain_id in chain_ids
            )
        },
        "supervisord": {
            "pidfile": "%(here)s/supervisord.pid",
            "nodaemon": "true",
            "logfile": "/dev/null",
            "logfile_maxbytes": "0",
        },
        "rpcinterface:supervisor": {
            "supervisor.rpcinterface_factory": "supervisor.rpcinterface:"
            "make_main_rpcinterface",
        },
        "unix_http_server": {"file": "%(here)s/supervisor.sock"},
        "supervisorctl": {"serverurl": "unix://%(here)s/supervisor.sock"},
    }
    for path, path_cfg in paths.items():
        src = path_cfg["src"]["chain-id"]
        dst = path_cfg["dst"]["chain-id"]
        cfg[f"program:relayer-{path}"] = dict(
            COMMON_PROG_OPTIONS,
            command=(
                f"hermes -c %(here)s/relayer.toml start {src} {dst} "
                "-p transfer -c channel-0"
            ),
            stdout_logfile=f"%(here)s/relayer-{path}.log",
            autostart="false",
        )
    return cfg


def docker_compose_yml(cmd, validators, data_dir, image):
    return {
        "version": "3",
        "services": {
            f"node{i}": {
                "image": image,
                "command": "chaind start",
                "volumes": [f"{data_dir.absolute() / f'node{i}'}:/.chain-maind:Z"],
            }
            for i, val in enumerate(validators)
        },
    }


def edit_tm_cfg(path, base_port, peers, *, custom_edit=None):
    doc = tomlkit.parse(open(path).read())
    # tendermint is start in process, not needed
    # doc['proxy_app'] = 'tcp://127.0.0.1:%d' % abci_port(base_port)
    doc["rpc"]["laddr"] = "tcp://0.0.0.0:%d" % ports.rpc_port(base_port)
    doc["rpc"]["pprof_laddr"] = "localhost:%d" % ports.pprof_port(base_port)
    doc["rpc"]["grpc_laddr"] = "tcp://0.0.0.0:%d" % ports.grpc_port_tx_only(base_port)
    doc["p2p"]["laddr"] = "tcp://0.0.0.0:%d" % ports.p2p_port(base_port)
    doc["p2p"]["persistent_peers"] = peers
    doc["p2p"]["addr_book_strict"] = False
    doc["p2p"]["allow_duplicate_ip"] = True
    doc["consensus"]["timeout_commit"] = "1s"
    doc["rpc"]["timeout_broadcast_tx_commit"] = "30s"
    if custom_edit is not None:
        custom_edit(doc)
    open(path, "w").write(tomlkit.dumps(doc))


def edit_app_cfg(path, base_port, minimum_gas_prices=""):
    doc = tomlkit.parse(open(path).read())
    # enable api server
    doc["api"]["enable"] = True
    doc["api"]["swagger"] = True
    doc["api"]["enabled-unsafe-cors"] = True
    doc["api"]["address"] = "tcp://0.0.0.0:%d" % ports.api_port(base_port)
    doc["grpc"]["address"] = "0.0.0.0:%d" % ports.grpc_port(base_port)
    # take snapshot for statesync
    doc["pruning"] = "nothing"
    doc["state-sync"]["snapshot-interval"] = 5
    doc["state-sync"]["snapshot-keep-recent"] = 10
    doc["minimum-gas-prices"] = minimum_gas_prices
    open(path, "w").write(tomlkit.dumps(doc))


if __name__ == "__main__":
    interact("rm -r data; mkdir data", ignore_error=True)
    data_dir = Path("data")
    init_cluster(data_dir, "config.yaml", 26650)
    supervisord = start_cluster(data_dir)
    t = start_tail_logs_thread(data_dir)
    supervisord.wait()
    t.stop()
    t.join()
