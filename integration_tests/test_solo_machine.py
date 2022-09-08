import json
import os
import platform
from pathlib import Path

import pytest
import requests
from pystarport import expansion, ports
from pystarport.utils import interact

from .utils import cluster_fixture, wait_for_block, wait_for_port

pytestmark = pytest.mark.solomachine


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    try:
        yield from cluster_fixture(
            Path(__file__).parent / "configs/solo_machine.jsonnet",
            worker_index,
            tmp_path_factory.mktemp("data"),
        )
    finally:
        pass


def parse_output(output, return_json=True):
    s = output.decode("utf-8")
    s = s.replace("\u001b[0m", "")
    print(s)
    if return_json:
        data = json.loads(s)
        return data
    else:
        return s


CRO_DECIMALS = 10**8
SOLO_HD_PATH = "m/44'/394'/0'/0/0"
SOLO_ACCOUNT_PREFIX = "cro"
SOLO_ADDRESS_ALGO = "secp256k1"
SOLO_FEE_DENOM = "basecro"
SOLO_DENOM = "solotoken"


class SoloMachine(object):
    def __init__(
        self, temp_path, mnemonic, base_port=26650, chain_id="devnet-solomachine"
    ):
        self.chain_id = chain_id
        self.grpc_port = ports.grpc_port(base_port)
        self.rpc_port = ports.rpc_port(base_port)
        self.solomachine_home = os.path.join(os.environ["SOLO_MACHINE_HOME"])
        self.bin_file = os.path.join(self.solomachine_home, "solo-machine")
        self.mnemonic = mnemonic

        os_platform = platform.system()
        if os_platform == "Darwin":
            self.sign_file = os.path.join(
                self.solomachine_home, "libmnemonic_signer.dylib"
            )
        else:
            self.sign_file = os.path.join(
                self.solomachine_home, "libmnemonic_signer.so"
            )
        self.trusted_height = None
        self.trusted_hash = None
        self.db_path = f'sqlite://{temp_path.join("solo-machine.db")}'

    def get_chain_info(self):
        result = requests.get(f"http://127.0.0.1:{self.rpc_port}/block").json()
        self.trusted_hash = result["result"]["block_id"]["hash"]
        self.trusted_height = result["result"]["block"]["header"]["height"]

    def set_env(self):
        os.environ["SOLO_DB_URI"] = self.db_path
        os.environ["SOLO_SIGNER"] = self.sign_file
        os.environ["SOLO_MNEMONIC"] = self.mnemonic
        os.environ["SOLO_HD_PATH"] = SOLO_HD_PATH
        os.environ["SOLO_ACCOUNT_PREFIX"] = SOLO_ACCOUNT_PREFIX
        os.environ["SOLO_ADDRESS_ALGO"] = SOLO_ADDRESS_ALGO
        os.environ["SOLO_FEE_DENOM"] = SOLO_FEE_DENOM
        os.environ["SOLO_FEE_AMOUNT"] = "1000"
        os.environ["SOLO_GAS_LIMIT"] = "300000"
        os.environ["SOLO_GRPC_ADDRESS"] = f"http://0.0.0.0:{self.grpc_port}"
        os.environ["SOLO_RPC_ADDRESS"] = f"http://0.0.0.0:{self.rpc_port}"
        os.environ["SOLO_TRUSTED_HASH"] = self.trusted_hash
        os.environ["SOLO_TRUSTED_HEIGHT"] = self.trusted_height

    def prepare(self):
        self.get_chain_info()
        self.set_env()

    def run_sub_cmd(self, sub_cmd, json_output=True):
        if json_output:
            cmd = f"{self.bin_file} --output json {sub_cmd}"
        else:
            cmd = f"{self.bin_file} {sub_cmd}"
        output = interact(cmd)
        data = parse_output(output, json_output)
        return data

    def init(self):
        sub_cmd = "init"
        return self.run_sub_cmd(sub_cmd)

    def add_chain(self):
        sub_cmd = "chain add"
        return self.run_sub_cmd(sub_cmd)

    def connect_chain(self):
        sub_cmd = f"ibc connect {self.chain_id}"
        result = self.run_sub_cmd(sub_cmd, False)
        if "error" in result:
            raise Exception(result)

    def get_balance(self):
        sub_cmd = f"chain balance {self.chain_id} {SOLO_DENOM}"
        return self.run_sub_cmd(sub_cmd)["data"]["balance"]

    def mint(self, amount=20):
        sub_cmd = f"ibc mint {self.chain_id} {amount} {SOLO_DENOM}"
        return self.run_sub_cmd(sub_cmd)

    def burn(self, amount=10):
        sub_cmd = f"ibc burn {self.chain_id} {amount} {SOLO_DENOM}"
        return self.run_sub_cmd(sub_cmd)


def get_balance(cluster, addr):
    raw = cluster.cosmos_cli(0).raw
    node = cluster.node_rpc(0)
    coin = json.loads(raw("query", "bank", "balances", addr, output="json", node=node))[
        "balances"
    ]
    if len(coin) == 0:
        return None
    return coin[0]


def get_mnemonic(cli):
    config_path = Path(__file__).parent / "configs/solo_machine.jsonnet"
    config = expansion.expand_jsonnet(config_path, None)
    return config[cli.chain_id]["accounts"][0]["mnemonic"]


def test_solo_machine(cluster, tmpdir_factory):
    """
    check solo machine
    """
    cli = cluster
    wait_for_block(cli, 1)

    # get the chain balance
    chain_solo_addr = cluster.address("solo-signer")
    chain_balance_1 = cluster.balance(chain_solo_addr)
    assert chain_balance_1 == 1500 * CRO_DECIMALS

    base_port = cli.base_port(0)
    wait_for_port(ports.grpc_port(base_port))
    tmp_path = tmpdir_factory.mktemp("db")
    mnemonic = get_mnemonic(cli)
    solo_machine = SoloMachine(tmp_path, mnemonic, base_port=base_port)
    solo_machine.prepare()
    data = solo_machine.init()
    assert data["result"] == "success"
    data = solo_machine.add_chain()
    assert data["result"] == "success"
    wait_for_block(cli, 2)
    solo_machine.connect_chain()
    wait_for_block(cli, 2)
    balance_0 = int(solo_machine.get_balance())
    assert balance_0 == 0

    # mint
    data = solo_machine.mint(20)
    assert data["result"] == "success"
    assert int(data["data"]["amount"], 16) == 20
    wait_for_block(cli, 2)
    balance_1 = int(solo_machine.get_balance())
    assert balance_1 == 20

    # check the chain balance

    chain_balance_2 = get_balance(cli, chain_solo_addr)
    assert int(chain_balance_2["amount"]) < chain_balance_1
    assert chain_balance_2["denom"] == SOLO_FEE_DENOM

    # burn
    data = solo_machine.burn(10)
    assert data["result"] == "success"
    assert int(data["data"]["amount"], 16) == 10
    wait_for_block(cli, 2)
    balance_2 = int(solo_machine.get_balance())
    assert balance_2 == balance_1 - 10

    # check the chain balance
    chain_balance_3 = get_balance(cli, chain_solo_addr)
    assert int(chain_balance_3["amount"]) < int(chain_balance_2["amount"])
    assert chain_balance_3["denom"] == SOLO_FEE_DENOM
