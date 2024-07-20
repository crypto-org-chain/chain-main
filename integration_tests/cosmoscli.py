import json
import tempfile

import requests
from pystarport import cluster, cosmoscli


class CosmosCLI(cosmoscli.CosmosCLI):
    def submit_gov_proposal(self, proposal, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "gov",
                "submit-proposal",
                proposal,
                "-y",
                home=self.data_dir,
                **kwargs,
            )
        )
        if rsp["code"] == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def gov_propose_legacy(
        self,
        proposer,
        kind,
        proposal,
        no_validate=False,
        event_query_tx=True,
        **kwargs,
    ):
        if kind == "software-upgrade":
            rsp = json.loads(
                self.raw(
                    "tx",
                    "gov",
                    "submit-legacy-proposal",
                    kind,
                    proposal["name"],
                    "-y",
                    "--no-validate" if no_validate else None,
                    from_=proposer,
                    # content
                    title=proposal.get("title"),
                    description=proposal.get("description"),
                    upgrade_height=proposal.get("upgrade-height"),
                    upgrade_time=proposal.get("upgrade-time"),
                    upgrade_info=proposal.get("upgrade-info", "info"),
                    deposit=proposal.get("deposit"),
                    # basic
                    home=self.data_dir,
                    node=self.node_rpc,
                    keyring_backend="test",
                    chain_id=self.chain_id,
                    **kwargs,
                )
            )
            if rsp["code"] == 0 and event_query_tx:
                rsp = self.event_query_tx_for(rsp["txhash"])
            return rsp
        elif kind == "cancel-software-upgrade":
            rsp = json.loads(
                self.raw(
                    "tx",
                    "gov",
                    "submit-legacy-proposal",
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
                    **kwargs,
                )
            )
            if rsp["code"] == 0 and event_query_tx:
                rsp = self.event_query_tx_for(rsp["txhash"])
            return rsp
        else:
            with tempfile.NamedTemporaryFile("w") as fp:
                json.dump(proposal, fp)
                fp.flush()
                rsp = json.loads(
                    self.raw(
                        "tx",
                        "gov",
                        "submit-legacy-proposal",
                        kind,
                        fp.name,
                        "-y",
                        from_=proposer,
                        # basic
                        home=self.data_dir,
                        node=self.node_rpc,
                        keyring_backend="test",
                        chain_id=self.chain_id,
                        **kwargs,
                    )
                )
                if rsp["code"] == 0 and event_query_tx:
                    rsp = self.event_query_tx_for(rsp["txhash"])
                return rsp

    def gov_propose_new(
        self,
        proposer,
        kind,
        proposal,
        **kwargs,
    ):
        if kind == "software-upgrade":
            rsp = json.loads(
                self.raw(
                    "tx",
                    "upgrade",
                    kind,
                    proposal["name"],
                    "-y",
                    "--no-validate",
                    from_=proposer,
                    # content
                    title=proposal.get("title"),
                    summary=proposal.get("summary"),
                    upgrade_height=proposal.get("upgrade-height"),
                    upgrade_time=proposal.get("upgrade-time"),
                    upgrade_info=proposal.get("upgrade-info", "info"),
                    deposit=proposal.get("deposit"),
                    # basic
                    home=self.data_dir,
                    node=self.node_rpc,
                    keyring_backend="test",
                    chain_id=self.chain_id,
                    **kwargs,
                )
            )
        else:
            rsp = json.loads(
                self.raw(
                    "tx",
                    "upgrade",
                    kind,
                    "-y",
                    from_=proposer,
                    # content
                    title=proposal.get("title"),
                    summary=proposal.get("summary"),
                    deposit=proposal.get("deposit"),
                    # basic
                    home=self.data_dir,
                    node=self.node_rpc,
                    keyring_backend="test",
                    chain_id=self.chain_id,
                    **kwargs,
                )
            )
        if rsp["code"] == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def transfer(
        self,
        from_,
        to,
        coins,
        generate_only=False,
        event_query_tx=True,
        **kwargs,
    ):
        default_kwargs = {
            "home": self.data_dir,
            "keyring_backend": "test",
            "chain_id": self.chain_id,
            "node": self.node_rpc,
        }
        rsp = json.loads(
            self.raw(
                "tx",
                "bank",
                "send",
                from_,
                to,
                coins,
                "-y",
                "--generate-only" if generate_only else None,
                **(default_kwargs | kwargs),
            )
        )
        if not generate_only and rsp["code"] == 0 and event_query_tx:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def sign_tx(self, tx_file, signer):
        return json.loads(
            self.raw(
                "tx",
                "sign",
                tx_file,
                from_=signer,
                home=self.data_dir,
                keyring_backend="test",
                chain_id=self.chain_id,
                node=self.node_rpc,
            )
        )

    def sign_tx_json(self, tx, signer, max_priority_price=None):
        if max_priority_price is not None:
            tx["body"]["extension_options"].append(
                {
                    "@type": "/ethermint.types.v1.ExtensionOptionDynamicFeeTx",
                    "max_priority_price": str(max_priority_price),
                }
            )
        with tempfile.NamedTemporaryFile("w") as fp:
            json.dump(tx, fp)
            fp.flush()
            return self.sign_tx(fp.name, signer)

    def broadcast_tx(self, tx_file, event_query_tx=True, **kwargs):
        kwargs.setdefault("broadcast_mode", "sync")
        kwargs.setdefault("output", "json")
        rsp = json.loads(
            self.raw("tx", "broadcast", tx_file, node=self.node_rpc, **kwargs)
        )
        if event_query_tx and rsp["code"] == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def broadcast_tx_json(self, tx, event_query_tx=True, **kwargs):
        with tempfile.NamedTemporaryFile("w") as fp:
            json.dump(tx, fp)
            fp.flush()
            return self.broadcast_tx(fp.name, event_query_tx, **kwargs)

    def tx_search_rpc(self, events: str):
        node_rpc_http = "http" + self.node_rpc.removeprefix("tcp")
        rsp = requests.get(
            f"{node_rpc_http}/tx_search",
            params={
                "query": f'"{events}"',
            },
        ).json()
        assert "error" not in rsp, rsp["error"]
        return rsp["result"]["txs"]

    def sign_batch_multisig_tx(
        self,
        tx_file,
        multi_addr,
        signer_name,
        account_number,
        sequence_number,
        sigonly=True,
    ):
        r = self.raw(
            "tx",
            "sign-batch",
            "--offline",
            "--signature-only" if sigonly else None,
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

    def query_host_params(self):
        kwargs = {
            "node": self.node_rpc,
            "output": "json",
        }
        return json.loads(
            self.raw(
                "q",
                "interchain-accounts",
                "host",
                "params",
                **kwargs,
            )
        )

    def query_params(self, mod):
        kwargs = {
            "node": self.node_rpc,
            "output": "json",
        }
        return json.loads(
            self.raw(
                "q",
                mod,
                "params",
                **kwargs,
            )
        )


class ClusterCLI(cluster.ClusterCLI):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.cmd = self.cmd or self.config.get("cmd") or "chain-maind"

    def cosmos_cli(self, i=0):
        return CosmosCLI(
            self.home(i),
            self.node_rpc(i),
            chain_id=self.chain_id,
            cmd=self.cmd,
            zemu_address=self.zemu_address,
            zemu_button_port=self.zemu_button_port,
        )

    def submit_gov_proposal(self, proposer, i=0, **kwargs):
        return self.cosmos_cli(i).submit_gov_proposal(proposer, **kwargs)

    def gov_propose_legacy(
        self,
        proposer,
        kind,
        proposal,
        i=0,
        no_validate=False,
        event_query_tx=True,
        **kwargs,
    ):
        return self.cosmos_cli(i).gov_propose_legacy(
            proposer,
            kind,
            proposal,
            no_validate,
            event_query_tx,
            **kwargs,
        )

    def gov_propose_new(
        self,
        proposer,
        kind,
        proposal,
        i=0,
        **kwargs,
    ):
        return self.cosmos_cli(i).gov_propose_new(
            proposer,
            kind,
            proposal,
            **kwargs,
        )

    def transfer(self, from_, to, coins, i=0, generate_only=False, **kwargs):
        return self.cosmos_cli(i).transfer(from_, to, coins, generate_only, **kwargs)

    def sign_batch_multisig_tx(self, *args, i=0, **kwargs):
        return self.cosmos_cli(i).sign_batch_multisig_tx(*args, **kwargs)

    def query_host_params(self, i=0):
        return self.cosmos_cli(i).query_host_params()

    def query_params(self, mod, i=0):
        return self.cosmos_cli(i).query_params(mod)
