import datetime
import json
import os
import subprocess
import sys
from pathlib import Path

import dateutil.parser
import durations
import jsonmerge
import tomlkit
from supervisor import xmlrpc
from supervisor.compat import xmlrpclib

from . import ports
from .utils import interact, local_ip, write_ini

CHAIN = "chain-maind"  # edit by nix-build


def home_dir(data_dir, i):
    return data_dir / f"node{i}"


class ChainCommand:
    def __init__(self, cmd=None):
        self._cmd = cmd or CHAIN

    def __call__(self, cmd, *args, **kwargs):
        "execute chain-maind"
        args = [cmd] + list(args)
        for k, v in kwargs.items():
            args.append("--" + k.strip("_").replace("_", "-"))
            args.append(v)
        return interact(" ".join((self._cmd, *map(str, args))))


class ClusterCLI:
    "the apis to interact with wallet and blockchain prepared with Cluster"

    def __init__(self, data_dir, cmd=None):
        self.data_dir = data_dir
        self._genesis = json.load(open(data_dir / "genesis.json"))
        self.base_port = int((data_dir / "base_port").read_text())
        self.chain_id = self._genesis["chain_id"]
        self.raw = ChainCommand(cmd)
        self._supervisorctl = None

    @property
    def supervisor(self):
        # https://github.com/Supervisor/supervisor/blob/76df237032f7d9fbe80a0adce3829c8b916d5b58/supervisor/options.py#L1718
        if self._supervisorctl is None:
            self._supervisorctl = xmlrpclib.ServerProxy(
                # dumbass ServerProxy won't allow us to pass in a non-HTTP url,
                # so we fake the url we pass into it and
                # always use the transport's
                # 'serverurl' to figure out what to attach to
                "http://127.0.0.1",
                transport=xmlrpc.SupervisorTransport(
                    serverurl=f"unix://{self.data_dir}/supervisor.sock"
                ),
            )
        return self._supervisorctl.supervisor

    def home(self, i):
        "home directory of i-th node"
        return home_dir(self.data_dir, i)

    def node_rpc(self, i):
        "rpc url of i-th node"
        return "tcp://127.0.0.1:%d" % ports.rpc_port(self.base_port, i)

    def node_id(self, i):
        "get i-th node's tendermint node id"
        output = self.raw("tendermint", "show-node-id", home=self.home(i))
        return output.decode().strip()

    def create_account(self, name, i=0):
        "create new keypair in i-th node's keyring"
        output = self.raw(
            "keys",
            "add",
            name,
            home=self.home(i),
            output="json",
            keyring_backend="test",
        )
        return json.loads(output)

    def init(self, i):
        return self.raw("init", f"node{i}", chain_id=self.chain_id, home=self.home(i))

    def validate_genesis(self, i=0):
        return self.raw("validate-genesis", home=self.home(i))

    def add_genesis_account(self, addr, coins, i=0, **kwargs):
        return self.raw("add-genesis-account", addr, coins, home=self.home(i), **kwargs)

    def gentx(self, name, coins, i):
        return self.raw(
            "gentx",
            name,
            amount=coins,
            home=self.home(i),
            chain_id=self.chain_id,
            keyring_backend="test",
        )

    def collect_gentxs(self, gentx_dir, i=0):
        return self.raw("collect-gentxs", gentx_dir, home=self.home(i))

    def status(self, i=0):
        return json.loads(self.raw("status", node=self.node_rpc(i)))

    def balance(self, addr, i=0):
        coin = json.loads(
            self.raw(
                "query", "bank", "balances", addr, output="json", node=self.node_rpc(i)
            )
        )["balances"][0]
        assert coin["denom"] == "basecro"
        return int(coin["amount"])

    def address(self, name, i=0, bech="acc"):
        output = self.raw(
            "keys",
            "show",
            name,
            "-a",
            home=self.home(i),
            keyring_backend="test",
            bech=bech,
        )
        return output.strip().decode()

    def account(self, addr, i=0):
        return json.loads(
            self.raw(
                "query", "auth", "account", addr, output="json", node=self.node_rpc(i)
            )
        )

    def validator(self, addr, i=0):
        return json.loads(
            self.raw(
                "query",
                "staking",
                "validator",
                addr,
                output="json",
                node=self.node_rpc(i),
            )
        )

    def validators(self, i=0):
        return json.loads(
            self.raw(
                "query", "staking", "validators", output="json", node=self.node_rpc(i)
            )
        )

    def transfer(self, from_, to, coins, i=0, generate_only=False):
        return json.loads(
            self.raw(
                "tx",
                "bank",
                "send",
                from_,
                to,
                coins,
                "-y",
                "--generate-only" if generate_only else "",
                home=self.home(i),
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc(0),
            )
        )

    def get_delegated_amount(self, which_addr, i=0):
        return json.loads(
            self.raw(
                "query",
                "staking",
                "delegations",
                which_addr,
                home=self.home(i),
                chain_id=self.chain_id,
                node=self.node_rpc(0),
                output="json",
            )
        )

    def delegate_amount(self, to_addr, amount, from_addr, i=0):
        return json.loads(
            self.raw(
                "tx",
                "staking",
                "delegate",
                to_addr,
                amount,
                "-y",
                home=self.home(i),
                from_=from_addr,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc(0),
            )
        )

    # to_addr: croclcl1...  , from_addr: cro1...
    def unbond_amount(self, to_addr, amount, from_addr, i=0):
        return json.loads(
            self.raw(
                "tx",
                "staking",
                "unbond",
                to_addr,
                amount,
                "-y",
                home=self.home(i),
                from_=from_addr,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc(0),
            )
        )

    # to_validator_addr: crocncl1...  ,  from_from_validator_addraddr: crocl1...
    def redelegate_amount(
        self, to_validator_addr, from_validator_addr, amount, from_addr, i=0
    ):
        return json.loads(
            self.raw(
                "tx",
                "staking",
                "redelegate",
                from_validator_addr,
                to_validator_addr,
                amount,
                "-y",
                home=self.home(i),
                from_=from_addr,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc(0),
            )
        )

    def make_multisig(self, name, signer1, signer2, i=0):
        self.raw(
            "keys",
            "add",
            name,
            multisig=f"{signer1},{signer2}",
            multisig_threshold="2",
            home=self.home(i),
            keyring_backend="test",
            output="json",
        )

    def sign_multisig_tx(self, tx_file, multi_addr, signer_name, i=0):
        return json.loads(
            self.raw(
                "tx",
                "sign",
                tx_file,
                from_=signer_name,
                multisig=multi_addr,
                home=self.home(i),
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc(0),
            )
        )

    def combine_multisig_tx(self, tx_file, multi_name, signer1_file, signer2_file, i=0):
        return json.loads(
            self.raw(
                "tx",
                "multisign",
                tx_file,
                multi_name,
                signer1_file,
                signer2_file,
                home=self.home(i),
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc(0),
            )
        )

    def broadcast_tx(self, tx_file, i=0):
        return json.loads(self.raw("tx", "broadcast", tx_file, node=self.node_rpc(i)))

    def unjail(self, addr, i=0):
        return json.loads(
            self.raw(
                "tx",
                "slashing",
                "unjail",
                "-y",
                from_=addr,
                home=self.home(i),
                node=self.node_rpc(i),
                keyring_backend="test",
                chain_id=self.chain_id,
            )
        )


def start_cluster(data_dir, quiet=False):
    cmd = [sys.executable, "-msupervisor.supervisord", "-c", data_dir / "tasks.ini"]
    env = dict(os.environ, PYTHONPATH=":".join(sys.path))
    if quiet:
        return subprocess.Popen(
            cmd, stdout=(data_dir / "supervisord.log").open("w"), env=env
        )
    else:
        return subprocess.Popen(cmd, env=env)


def init_cluster(data_dir, config, base_port, cmd=None):
    """
    init data directory
    """
    cmd = cmd or CHAIN
    for i in range(len(config["validators"])):
        ChainCommand(cmd)(
            "init",
            config["validators"][i].get("name", f"node{i}"),
            chain_id=config["chain_id"],
            home=home_dir(data_dir, i),
        )

    os.rename(data_dir / "node0/config/genesis.json", data_dir / "genesis.json")
    os.mkdir(data_dir / "gentx")
    for i in range(len(config["validators"])):
        try:
            os.remove(data_dir / f"node{i}/config/genesis.json")
        except OSError:
            pass
        os.symlink("../../genesis.json", data_dir / f"node{i}/config/genesis.json")
        os.symlink("../../gentx", data_dir / f"node{i}/config/gentx")

    (data_dir / "base_port").write_text(str(base_port))

    # now we can create ClusterCLI
    cli = ClusterCLI(data_dir, cmd)

    # patch the genesis file
    genesis = jsonmerge.merge(
        json.load(open(data_dir / "genesis.json")), config.get("genesis", {}),
    )
    json.dump(genesis, open(data_dir / "genesis.json", "w"))
    cli.validate_genesis(i)

    # create accounts
    accounts = []
    for i, node in enumerate(config["validators"]):
        account = cli.create_account("validator", i)
        print(account)
        accounts.append(account)
        cli.add_genesis_account(account["address"], node["coins"], i)
        cli.gentx("validator", node["staked"], i)

    for account in config["accounts"]:
        acct = cli.create_account(account["name"])
        print(acct)
        accounts.append(acct)
        vesting = account.get("vesting")
        if not vesting:
            cli.add_genesis_account(acct["address"], account["coins"])
        else:
            genesis_time = dateutil.parser.isoparse(genesis["genesis_time"])
            end_time = genesis_time + datetime.timedelta(
                seconds=durations.Duration(vesting).to_seconds()
            )
            vend = end_time.replace(tzinfo=None).isoformat("T") + "Z"
            cli.add_genesis_account(
                acct["address"],
                account["coins"],
                vesting_amount=account["coins"],
                vesting_end_time=vend,
            )
    # output accounts
    (data_dir / "accounts.txt").write_text(
        "\n".join(str(account) for account in accounts)
    )

    # collect-gentxs
    cli.collect_gentxs(data_dir / "gentx", i)

    # write tendermint config
    ip = local_ip()
    peers = ",".join(
        [
            "tcp://%s@%s:%d" % (cli.node_id(i), ip, ports.p2p_port(base_port, i))
            for i in range(len(config["validators"]))
        ]
    )
    for i in range(len(config["validators"])):
        edit_tm_cfg(data_dir / f"node{i}/config/config.toml", base_port, i, peers)
        edit_app_cfg(data_dir / f"node{i}/config/app.toml", base_port, i)

    # write supervisord config file
    supervisord_ini = {
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
    for i, node in enumerate(config["validators"]):
        supervisord_ini[f"program:node{i}"] = {
            "command": f"{cmd} start --home %(here)s/node{i}",
            # redirect to supervisord's stdout, easier to collect all logs
            "stdout_logfile": "/dev/fd/1",
            "stdout_logfile_maxbytes": "0",
            "autostart": "true",
            "autorestart": "true",
            "redirect_stderr": "true",
            "startsecs": "3",
        }
    write_ini(open(data_dir / "tasks.ini", "w"), supervisord_ini)


def edit_tm_cfg(path, base_port, i, peers):
    doc = tomlkit.parse(open(path).read())
    doc["moniker"] = "node%d" % i
    # tendermint is start in process, not needed
    # doc['proxy_app'] = 'tcp://127.0.0.1:%d' % abci_port(base_port, i)
    doc["rpc"]["laddr"] = "tcp://0.0.0.0:%d" % ports.rpc_port(base_port, i)
    doc["rpc"]["pprof_laddr"] = "localhost:%d" % ports.pprof_port(base_port, i)
    doc["p2p"]["laddr"] = "tcp://0.0.0.0:%d" % ports.p2p_port(base_port, i)
    doc["p2p"]["persistent_peers"] = peers
    doc["p2p"]["addr_book_strict"] = False
    doc["p2p"]["allow_duplicate_ip"] = True
    doc["consensus"]["timeout_commit"] = "1s"
    open(path, "w").write(tomlkit.dumps(doc))


def edit_app_cfg(path, base_port, i):
    doc = tomlkit.parse(open(path).read())
    # enable api server
    doc["api"]["enable"] = True
    doc["api"]["swagger"] = True
    doc["api"]["enabled-unsafe-cors"] = True
    doc["api"]["address"] = "tcp://0.0.0.0:%d" % ports.api_port(base_port, i)
    doc["grpc"]["address"] = "0.0.0.0:%d" % ports.grpc_port(base_port, i)
    open(path, "w").write(tomlkit.dumps(doc))


if __name__ == "__main__":
    import yaml

    interact("rm -r data; mkdir data", ignore_error=True)
    data_dir = Path("data")
    init_cluster(data_dir, yaml.safe_load(open("config.yaml")), 26650)
    supervisord = start_cluster(data_dir)
    supervisord.wait()
