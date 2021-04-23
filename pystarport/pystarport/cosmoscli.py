import enum
import hashlib
import json
import tempfile
import threading
import time

import bech32
from dateutil.parser import isoparse

from .app import CHAIN
from .ledger import ZEMU_BUTTON_PORT, ZEMU_HOST, LedgerButton
from .utils import build_cli_args_safe, format_doc_string, interact


class ModuleAccount(enum.Enum):
    FeeCollector = "fee_collector"
    Mint = "mint"
    Gov = "gov"
    Distribution = "distribution"
    BondedPool = "bonded_tokens_pool"
    NotBondedPool = "not_bonded_tokens_pool"
    IBCTransfer = "transfer"


@format_doc_string(
    options=",".join(v.value for v in ModuleAccount.__members__.values())
)
def module_address(name):
    """
    get address of module accounts

    :param name: name of module account, values: {options}
    """
    data = hashlib.sha256(ModuleAccount(name).value.encode()).digest()[:20]
    return bech32.bech32_encode("cro", bech32.convertbits(data, 8, 5))


class ChainCommand:
    def __init__(self, cmd=None):
        self.cmd = cmd or CHAIN

    def __call__(self, cmd, *args, stdin=None, **kwargs):
        "execute chain-maind"
        args = " ".join(build_cli_args_safe(cmd, *args, **kwargs))
        return interact(f"{self.cmd} {args}", input=stdin)


class CosmosCLI:
    "the apis to interact with wallet and blockchain"

    def __init__(
        self,
        data_dir,
        node_rpc,
        chain_id=None,
        cmd=None,
        zemu_address=ZEMU_HOST,
        zemu_button_port=ZEMU_BUTTON_PORT,
    ):
        self.data_dir = data_dir
        if chain_id is None:
            self._genesis = json.load(open(self.data_dir / "config" / "genesis.json"))
            self.chain_id = self._genesis["chain_id"]
        else:
            self.chain_id = chain_id
        self.node_rpc = node_rpc
        self.raw = ChainCommand(cmd)
        self.leger_button = LedgerButton(zemu_address, zemu_button_port)
        self.output = None
        self.error = None

    def node_id(self):
        "get tendermint node id"
        output = self.raw("tendermint", "show-node-id", home=self.data_dir)
        return output.decode().strip()

    def delete_account(self, name):
        "delete wallet account in node's keyring"
        return self.raw(
            "keys",
            "delete",
            name,
            "-y",
            "--force",
            home=self.data_dir,
            output="json",
            keyring_backend="test",
        )

    def create_account(self, name, mnemonic=None):
        "create new keypair in node's keyring"
        if mnemonic is None:
            output = self.raw(
                "keys",
                "add",
                name,
                home=self.data_dir,
                output="json",
                keyring_backend="test",
            )
        else:
            output = self.raw(
                "keys",
                "add",
                name,
                "--recover",
                home=self.data_dir,
                output="json",
                keyring_backend="test",
                stdin=mnemonic.encode() + b"\n",
            )
        return json.loads(output)

    def create_account_ledger(self, name):
        "create new ledger keypair"

        def send_request():
            try:
                self.output = self.raw(
                    "keys",
                    "add",
                    name,
                    "--ledger",
                    home=self.data_dir,
                    output="json",
                    keyring_backend="test",
                )
            except Exception as e:
                self.error = e

        t = threading.Thread(target=send_request)
        t.start()
        time.sleep(3)
        for _ in range(0, 3):
            self.leger_button.press_right()
            time.sleep(0.2)
        self.leger_button.press_both()
        t.join()
        if self.error:
            raise self.error
        return json.loads(self.output)

    def init(self, moniker):
        "the node's config is already added"
        return self.raw(
            "init",
            moniker,
            chain_id=self.chain_id,
            home=self.data_dir,
        )

    def validate_genesis(self):
        return self.raw("validate-genesis", home=self.data_dir)

    def add_genesis_account(self, addr, coins, **kwargs):
        return self.raw(
            "add-genesis-account",
            addr,
            coins,
            home=self.data_dir,
            output="json",
            **kwargs,
        )

    def gentx(self, name, coins, min_self_delegation=1, pubkey=None):
        return self.raw(
            "gentx",
            name,
            coins,
            min_self_delegation=str(min_self_delegation),
            home=self.data_dir,
            chain_id=self.chain_id,
            keyring_backend="test",
            pubkey=pubkey,
        )

    def collect_gentxs(self, gentx_dir):
        return self.raw("collect-gentxs", gentx_dir, home=self.data_dir)

    def status(self):
        return json.loads(self.raw("status", node=self.node_rpc))

    def block_height(self):
        return int(self.status()["SyncInfo"]["latest_block_height"])

    def block_time(self):
        return isoparse(self.status()["SyncInfo"]["latest_block_time"])

    def balance(self, addr):
        coin = json.loads(
            self.raw(
                "query", "bank", "balances", addr, output="json", node=self.node_rpc
            )
        )["balances"]
        if len(coin) == 0:
            return 0
        coin = coin[0]
        return int(coin["amount"])

    def query_all_txs(self, addr):
        txs = self.raw(
            "query",
            "txs-all",
            addr,
            home=self.data_dir,
            keyring_backend="test",
            chain_id=self.chain_id,
            node=self.node_rpc,
        )
        return json.loads(txs)

    def distribution_commission(self, addr):
        coin = json.loads(
            self.raw(
                "query",
                "distribution",
                "commission",
                addr,
                output="json",
                node=self.node_rpc,
            )
        )["commission"][0]
        return float(coin["amount"])

    def distribution_community(self):
        coin = json.loads(
            self.raw(
                "query",
                "distribution",
                "community-pool",
                output="json",
                node=self.node_rpc,
            )
        )["pool"][0]
        return float(coin["amount"])

    def distribution_reward(self, delegator_addr):
        coin = json.loads(
            self.raw(
                "query",
                "distribution",
                "rewards",
                delegator_addr,
                output="json",
                node=self.node_rpc,
            )
        )["total"][0]
        return float(coin["amount"])

    def address(self, name, bech="acc"):
        output = self.raw(
            "keys",
            "show",
            name,
            "-a",
            home=self.data_dir,
            keyring_backend="test",
            bech=bech,
        )
        return output.strip().decode()

    def account(self, addr):
        return json.loads(
            self.raw(
                "query", "auth", "account", addr, output="json", node=self.node_rpc
            )
        )

    def supply(self, supply_type):
        return json.loads(
            self.raw("query", "supply", supply_type, output="json", node=self.node_rpc)
        )

    def validator(self, addr):
        return json.loads(
            self.raw(
                "query",
                "staking",
                "validator",
                addr,
                output="json",
                node=self.node_rpc,
            )
        )

    def validators(self):
        return json.loads(
            self.raw(
                "query", "staking", "validators", output="json", node=self.node_rpc
            )
        )["validators"]

    def staking_params(self):
        return json.loads(
            self.raw("query", "staking", "params", output="json", node=self.node_rpc)
        )

    def staking_pool(self, bonded=True):
        return int(
            json.loads(
                self.raw("query", "staking", "pool", output="json", node=self.node_rpc)
            )["bonded_tokens" if bonded else "not_bonded_tokens"]
        )

    def transfer(self, from_, to, coins, generate_only=False, fees=None):
        return json.loads(
            self.raw(
                "tx",
                "bank",
                "send",
                from_,
                to,
                coins,
                "-y",
                "--generate-only" if generate_only else None,
                home=self.data_dir,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
                fees=fees,
            )
        )

    def transfer_from_ledger(self, from_, to, coins, generate_only=False, fees=None):
        def send_request():
            try:
                self.output = self.raw(
                    "tx",
                    "bank",
                    "send",
                    from_,
                    to,
                    coins,
                    "-y",
                    "--generate-only" if generate_only else "",
                    "--ledger",
                    home=self.data_dir,
                    keyring_backend="test",
                    chain_id=self.chain_id,
                    node=self.node_rpc,
                    fees=fees,
                    sign_mode="amino-json",
                )
            except Exception as e:
                self.error = e

        t = threading.Thread(target=send_request)
        t.start()
        time.sleep(3)
        for _ in range(0, 11):
            self.leger_button.press_right()
            time.sleep(0.4)
        self.leger_button.press_both()
        t.join()
        if self.error:
            raise self.error
        return json.loads(self.output)

    def get_delegated_amount(self, which_addr):
        return json.loads(
            self.raw(
                "query",
                "staking",
                "delegations",
                which_addr,
                home=self.data_dir,
                chain_id=self.chain_id,
                node=self.node_rpc,
                output="json",
            )
        )

    def delegate_amount(self, to_addr, amount, from_addr):
        return json.loads(
            self.raw(
                "tx",
                "staking",
                "delegate",
                to_addr,
                amount,
                "-y",
                home=self.data_dir,
                from_=from_addr,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    # to_addr: croclcl1...  , from_addr: cro1...
    def unbond_amount(self, to_addr, amount, from_addr):
        return json.loads(
            self.raw(
                "tx",
                "staking",
                "unbond",
                to_addr,
                amount,
                "-y",
                home=self.data_dir,
                from_=from_addr,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    # to_validator_addr: crocncl1...  ,  from_from_validator_addraddr: crocl1...
    def redelegate_amount(
        self, to_validator_addr, from_validator_addr, amount, from_addr
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
                home=self.data_dir,
                from_=from_addr,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    # from_delegator can be account name or address
    def withdraw_all_rewards(self, from_delegator):
        return json.loads(
            self.raw(
                "tx",
                "distribution",
                "withdraw-all-rewards",
                "-y",
                from_=from_delegator,
                home=self.data_dir,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def make_multisig(self, name, signer1, signer2):
        self.raw(
            "keys",
            "add",
            name,
            multisig=f"{signer1},{signer2}",
            multisig_threshold="2",
            home=self.data_dir,
            keyring_backend="test",
            output="json",
        )

    def sign_multisig_tx(self, tx_file, multi_addr, signer_name):
        return json.loads(
            self.raw(
                "tx",
                "sign",
                tx_file,
                from_=signer_name,
                multisig=multi_addr,
                home=self.data_dir,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def sign_batch_multisig_tx(
        self, tx_file, multi_addr, signer_name, account_number, sequence_number
    ):
        r = self.raw(
            "tx",
            "sign-batch",
            "--offline",
            tx_file,
            account_number=account_number,
            sequence=sequence_number,
            from_=signer_name,
            multisig=multi_addr,
            home=self.data_dir,
            keyring_backend="test",
            chain_id=self.chain_id,
            node=self.node_rpc,
        )
        return r.decode("utf-8")

    def encode_signed_tx(self, signed_tx):
        return self.raw(
            "tx",
            "encode",
            signed_tx,
        )

    def sign_single_tx(self, tx_file, signer_name):
        return json.loads(
            self.raw(
                "tx",
                "sign",
                tx_file,
                from_=signer_name,
                home=self.data_dir,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def combine_multisig_tx(self, tx_file, multi_name, signer1_file, signer2_file):
        return json.loads(
            self.raw(
                "tx",
                "multisign",
                tx_file,
                multi_name,
                signer1_file,
                signer2_file,
                home=self.data_dir,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def combine_batch_multisig_tx(
        self, tx_file, multi_name, signer1_file, signer2_file
    ):
        r = self.raw(
            "tx",
            "multisign-batch",
            tx_file,
            multi_name,
            signer1_file,
            signer2_file,
            home=self.data_dir,
            keyring_backend="test",
            chain_id=self.chain_id,
            node=self.node_rpc,
        )
        return r.decode("utf-8")

    def broadcast_tx(self, tx_file):
        return json.loads(self.raw("tx", "broadcast", tx_file, node=self.node_rpc))

    def unjail(self, addr):
        return json.loads(
            self.raw(
                "tx",
                "slashing",
                "unjail",
                "-y",
                from_=addr,
                home=self.data_dir,
                node=self.node_rpc,
                keyring_backend="test",
                chain_id=self.chain_id,
            )
        )

    def create_validator(
        self,
        amount,
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
        pubkey = (
            self.raw(
                "tendermint",
                "show-validator",
                home=self.data_dir,
            )
            .strip()
            .decode()
        )
        return json.loads(
            self.raw(
                "tx",
                "staking",
                "create-validator",
                "-y",
                from_=self.address("validator"),
                amount=amount,
                pubkey=pubkey,
                min_self_delegation=min_self_delegation,
                # commision
                commission_rate=commission_rate,
                commission_max_rate=commission_max_rate,
                commission_max_change_rate=commission_max_change_rate,
                # description
                moniker=moniker,
                identity=identity,
                website=website,
                security_contact=security_contact,
                details=details,
                # basic
                home=self.data_dir,
                node=self.node_rpc,
                keyring_backend="test",
                chain_id=self.chain_id,
            )
        )

    def edit_validator(
        self,
        commission_rate=None,
        moniker=None,
        identity=None,
        website=None,
        security_contact=None,
        details=None,
    ):
        """MsgEditValidator"""
        options = dict(
            commission_rate=commission_rate,
            # description
            moniker=moniker,
            identity=identity,
            website=website,
            security_contact=security_contact,
            details=details,
        )
        return json.loads(
            self.raw(
                "tx",
                "staking",
                "edit-validator",
                "-y",
                from_=self.address("validator"),
                home=self.data_dir,
                node=self.node_rpc,
                keyring_backend="test",
                chain_id=self.chain_id,
                **{k: v for k, v in options.items() if v is not None},
            )
        )

    def gov_propose(self, proposer, kind, proposal):
        if kind == "software-upgrade":
            return json.loads(
                self.raw(
                    "tx",
                    "gov",
                    "submit-proposal",
                    kind,
                    proposal["name"],
                    "-y",
                    from_=proposer,
                    # content
                    title=proposal.get("title"),
                    description=proposal.get("description"),
                    upgrade_height=proposal.get("upgrade-height"),
                    upgrade_time=proposal.get("upgrade-time"),
                    upgrade_info=proposal.get("upgrade-info"),
                    deposit=proposal.get("deposit"),
                    # basic
                    home=self.data_dir,
                    node=self.node_rpc,
                    keyring_backend="test",
                    chain_id=self.chain_id,
                )
            )
        elif kind == "cancel-software-upgrade":
            return json.loads(
                self.raw(
                    "tx",
                    "gov",
                    "submit-proposal",
                    kind,
                    "-y",
                    from_=proposer,
                    # content
                    title=proposal.get("title"),
                    description=proposal.get("description"),
                    deposit=proposal.get("deposit"),
                    # basic
                    home=self.data_dir,
                    node=self.node_rpc,
                    keyring_backend="test",
                    chain_id=self.chain_id,
                )
            )
        else:
            with tempfile.NamedTemporaryFile("w") as fp:
                json.dump(proposal, fp)
                fp.flush()
                return json.loads(
                    self.raw(
                        "tx",
                        "gov",
                        "submit-proposal",
                        kind,
                        fp.name,
                        "-y",
                        from_=proposer,
                        # basic
                        home=self.data_dir,
                        node=self.node_rpc,
                        keyring_backend="test",
                        chain_id=self.chain_id,
                    )
                )

    def gov_vote(self, voter, proposal_id, option):
        print(voter)
        print(proposal_id)
        print(option)
        return json.loads(
            self.raw(
                "tx",
                "gov",
                "vote",
                proposal_id,
                option,
                "-y",
                from_=voter,
                home=self.data_dir,
                node=self.node_rpc,
                keyring_backend="test",
                chain_id=self.chain_id,
            )
        )

    def gov_deposit(self, depositor, proposal_id, amount):
        return json.loads(
            self.raw(
                "tx",
                "gov",
                "deposit",
                proposal_id,
                amount,
                "-y",
                from_=depositor,
                home=self.data_dir,
                node=self.node_rpc,
                keyring_backend="test",
                chain_id=self.chain_id,
            )
        )

    def query_proposals(self, depositor=None, limit=None, status=None, voter=None):
        return json.loads(
            self.raw(
                "query",
                "gov",
                "proposals",
                depositor=depositor,
                count_total=limit,
                status=status,
                voter=voter,
                output="json",
                node=self.node_rpc,
            )
        )

    def query_proposal(self, proposal_id):
        return json.loads(
            self.raw(
                "query",
                "gov",
                "proposal",
                proposal_id,
                output="json",
                node=self.node_rpc,
            )
        )

    def query_tally(self, proposal_id):
        return json.loads(
            self.raw(
                "query",
                "gov",
                "tally",
                proposal_id,
                output="json",
                node=self.node_rpc,
            )
        )

    def ibc_transfer(
        self,
        from_,
        to,
        amount,
        channel,  # src channel
        target_version,  # chain version number of target chain
        i=0,
    ):
        return json.loads(
            self.raw(
                "tx",
                "ibc-transfer",
                "transfer",
                "transfer",  # src port
                channel,
                to,
                amount,
                "-y",
                # FIXME https://github.com/cosmos/cosmos-sdk/issues/8059
                "--absolute-timeouts",
                from_=from_,
                home=self.data_dir,
                node=self.node_rpc,
                keyring_backend="test",
                chain_id=self.chain_id,
                packet_timeout_height=f"{target_version}-10000000000",
                packet_timeout_timestamp=0,
            )
        )

    def export(self):
        return self.raw("export", home=self.data_dir)

    def unsaferesetall(self):
        return self.raw("unsafe-reset-all")
