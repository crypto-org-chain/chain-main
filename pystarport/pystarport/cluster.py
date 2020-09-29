import asyncio
import datetime
import json
import os
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


class ClusterCLI:
    def __init__(self, data_dir, base_port, chain_id, cmd=CHAIN):
        self.cmd = cmd
        self.data_dir = data_dir
        self.base_port = base_port
        self.chain_id = chain_id

    def home(self, i):
        "home directory of i-th node"
        return self.data_dir / f"node{i}"

    def node_rpc(self, i):
        "rpc url of i-th node"
        return "tcp://127.0.0.1:%d" % ports.rpc_port(self.base_port, i)

    async def __call__(self, *args, **kwargs):
        args = list(args)
        for k, v in kwargs.items():
            args.append("--" + k.replace("_", "-"))
            args.append(v)
        return await interact(" ".join((self.cmd, *map(str, args))))

    async def node_id(self, i):
        output = await self("tendermint", "show-node-id", home=self.home(i))
        return output.decode().strip()

    async def create_account(self, name, i=0):
        output = await self(
            "keys",
            "add",
            name,
            home=self.home(i),
            output="json",
            keyring_backend="test",
        )
        return json.loads(output)

    async def init(self, i):
        return await self("init", f"node{i}", chain_id=self.chain_id, home=self.home(i))

    async def validate_genesis(self, i=0):
        return await self("validate-genesis", home=self.home(i))

    async def add_genesis_account(self, addr, coins, i=0, **kwargs):
        return await self(
            "add-genesis-account", addr, coins, home=self.home(i), **kwargs
        )

    async def gentx(self, name, coins, i):
        return await self(
            "gentx",
            name,
            amount=coins,
            home=self.home(i),
            chain_id=self.chain_id,
            keyring_backend="test",
        )

    async def collect_gentxs(self, gentx_dir, i=0):
        return await self("collect-gentxs", gentx_dir, home=self.home(i))

    async def status(self, i=0):
        return json.loads(await self("status", node=self.node_rpc(i)))

    async def query_balance(self, addr, i=0):
        coin = json.loads(
            await self(
                "query", "bank", "balances", addr, output="json", node=self.node_rpc(i)
            )
        )["balances"][0]
        assert coin["denom"] == "basecro"
        return int(coin["amount"])

    async def get_account(self, name, i=0):
        return json.loads(
            await self(
                "keys",
                "show",
                name,
                home=self.home(i),
                keyring_backend="test",
                output="json",
            )
        )

    async def transfer(self, from_, to, coins, i=0):
        return await self(
            "tx",
            "bank",
            "send",
            from_,
            to,
            coins,
            "-y",
            home=self.home(i),
            keyring_backend="test",
            chain_id=self.chain_id,
            node=self.node_rpc(i),
        )


class Cluster:
    def __init__(self, config, data_dir, base_port, cmd=CHAIN):
        self.cmd = cmd
        self.config = config
        self.data_dir = data_dir
        self.base_port = base_port
        self.cli = ClusterCLI(data_dir, base_port, config["chain_id"], cmd)
        self.supervisord_process = None
        self._supervisorctl = None

    @property
    def supervisorctl(self):
        # https://github.com/Supervisor/supervisor/blob/76df237032f7d9fbe80a0adce3829c8b916d5b58/supervisor/options.py#L1718
        if self._supervisorctl is None:
            self._supervisorctl = xmlrpclib.ServerProxy(
                # dumbass ServerProxy won't allow us to pass in a non-HTTP url,
                # so we fake the url we pass into it and always use the transport's
                # 'serverurl' to figure out what to attach to
                "http://127.0.0.1",
                transport=xmlrpc.SupervisorTransport(
                    serverurl=f"unix://{self.data_dir}/supervisor.sock"
                ),
            )
        return self._supervisorctl

    async def start(self):
        assert not self.supervisord_process, "already started"
        self.supervisord_process = await asyncio.create_subprocess_exec(
            sys.executable,
            "-msupervisor.supervisord",
            "-c",
            self.data_dir / "tasks.ini",
        )

    async def wait(self):
        await self.supervisord_process.wait()

    async def terminate(self):
        self.supervisord_process.terminate()
        await self.supervisord_process.wait()
        self.supervisord_process = None
        self._supervisorctl = None

    async def init(self):
        """
        init data directory
        working directory is already set to data directory
        data directory is empty
        """
        # await interact('rm -r data', ignore_error=True)
        for i in range(len(self.config["validators"])):
            await self.cli.init(i)

        os.rename(
            self.data_dir / "node0/config/genesis.json", self.data_dir / "genesis.json"
        )
        os.mkdir(self.data_dir / "gentx")
        for i in range(len(self.config["validators"])):
            try:
                os.remove(self.data_dir / f"node{i}/config/genesis.json")
            except OSError:
                pass
            os.symlink(
                "../../genesis.json", self.data_dir / f"node{i}/config/genesis.json"
            )
            os.symlink("../../gentx", self.data_dir / f"node{i}/config/gentx")

        # patch the genesis file
        genesis = jsonmerge.merge(
            json.load(open(self.data_dir / "genesis.json")),
            self.config.get("genesis", {}),
        )
        json.dump(genesis, open(self.data_dir / "genesis.json", "w"))
        await self.cli.validate_genesis()

        # create accounts
        for i, node in enumerate(self.config["validators"]):
            account = await self.cli.create_account("validator", i)
            print(account)
            await self.cli.add_genesis_account(account["address"], node["coins"], i)
            await self.cli.gentx("validator", node["staked"], i)

        for account in self.config["accounts"]:
            acct = await self.cli.create_account(account["name"])
            print(acct)
            vesting = account.get("vesting")
            if not vesting:
                await self.cli.add_genesis_account(acct["address"], account["coins"])
            else:
                genesis_time = dateutil.parser.isoparse(genesis["genesis_time"])
                end_time = genesis_time + datetime.timedelta(
                    seconds=durations.Duration(vesting).to_seconds()
                )
                await self.cli.add_genesis_account(
                    acct["address"],
                    account["coins"],
                    vesting_amount=account["coins"],
                    vesting_end_time=end_time.replace(tzinfo=None).isoformat("T") + "Z",
                )

        # collect-gentxs
        await self.cli.collect_gentxs(self.data_dir / "gentx")

        # write tendermint config
        ip = local_ip()
        peers = ",".join(
            [
                "tcp://%s@%s:%d"
                % (await self.cli.node_id(i), ip, ports.p2p_port(self.base_port, i))
                for i in range(len(self.config["validators"]))
            ]
        )
        for i in range(len(self.config["validators"])):
            edit_tm_cfg(
                self.data_dir / f"node{i}/config/config.toml", self.base_port, i, peers
            )
            edit_app_cfg(self.data_dir / f"node{i}/config/app.toml", self.base_port, i)

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
        for i, node in enumerate(self.config["validators"]):
            supervisord_ini[f"program:node{i}"] = {
                "command": f"{self.cmd} start --home {self.cli.home(i)}",
                # redirect to supervisord's stdout, easier to collect all logs
                "stdout_logfile": "/dev/fd/1",
                "stdout_logfile_maxbytes": "0",
                "autostart": "true",
                "autorestart": "true",
                "redirect_stderr": "true",
                "startsecs": "3",
            }
        write_ini(open(self.data_dir / "tasks.ini", "w"), supervisord_ini)


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
    open(path, "w").write(tomlkit.dumps(doc))


def edit_app_cfg(path, base_port, i):
    doc = tomlkit.parse(open(path).read())
    doc["api"]["address"] = "tcp://0.0.0.0:%d" % ports.api_port(base_port, i)
    doc["grpc"]["address"] = "0.0.0.0:%d" % ports.grpc_port(base_port, i)
    open(path, "w").write(tomlkit.dumps(doc))


if __name__ == "__main__":
    import yaml

    async def test():
        await interact("rm -r data; mkdir data", ignore_error=True)
        c = Cluster(
            "chain-maind", yaml.safe_load(open("config.yml")), Path("data"), 26650
        )
        await c.init()
        await c.start()
        await c.wait()

    asyncio.run(test())
