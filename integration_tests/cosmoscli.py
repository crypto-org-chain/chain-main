import json
import tempfile

from pystarport import cluster, cosmoscli


class CosmosCLI(cosmoscli.CosmosCLI):
    def gov_propose_legacy(self, proposer, kind, proposal, **kwargs):
        if kind == "software-upgrade":
            return json.loads(
                self.raw(
                    "tx",
                    "gov",
                    "submit-legacy-proposal",
                    kind,
                    proposal["name"],
                    "-y",
                    from_=proposer,
                    # content
                    title=proposal.get("title"),
                    description=proposal.get("description"),
                    upgrade_height=proposal.get("upgrade-height"),
                    upgrade_time=proposal.get("upgrade-time"),
                    upgrade_info=proposal.get("upgrade-info", "'{}'"),
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

    def gov_propose_legacy(self, proposer, kind, proposal, i=0, **kwargs):
        return self.cosmos_cli(i).gov_propose_legacy(proposer, kind, proposal, **kwargs)
