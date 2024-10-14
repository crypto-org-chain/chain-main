import hashlib
import json
import subprocess
import time
from datetime import datetime, timedelta
from pathlib import Path

import pytest
import yaml

from .ibc_utils import (
    ibc_incentivized_transfer,
    ibc_transfer_flow,
    start_and_wait_relayer,
)
from .utils import cluster_fixture

pytestmark = pytest.mark.ibc


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/ibc.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def test_ibc(cluster):
    src_channel, dst_channel = start_and_wait_relayer(cluster, incentivized=True)
    ibc_transfer_flow(cluster, src_channel, dst_channel)
    ibc_incentivized_transfer(cluster)


@pytest.mark.skip(reason="chain-id change don't has effect")
def test_update_chain_id(cluster):
    data_root = next(iter(cluster.values())).data_root
    # call chain-maind directly
    raw = next(iter(cluster.values())).cosmos_cli().raw
    relayer = ["hermes", "--home", data_root / "relayer"]

    channels = json.loads(
        raw(
            "query",
            "ibc",
            "channel",
            "channels",
            node=cluster["ibc-1"].node_rpc(0),
        )
    )["channels"]
    dst_channel = channels[0]["counterparty"]["channel_id"]
    denom_hash = (
        hashlib.sha256(f"transfer/{dst_channel}/basecro".encode()).hexdigest().upper()
    )
    # stop relayer and update chain-id in relayer config
    relayer_config = cluster["ibc-1"].data_root / "relayer" / "config" / "config.yaml"
    config = yaml.safe_load(open(relayer_config))
    for c in config["chains"]:
        if c["chain-id"] == "ibc-1":
            c["chain-id"] = "ibc-2"
    with open(relayer_config, "w") as f:
        f.write(yaml.dump(config))
    yaml.dump(config, relayer_config.open("w"))
    cluster["ibc-1"].restart_relayer()

    # update chain-id
    cluster["ibc-1"].stop_node()
    genesis_data = json.loads(cluster["ibc-1"].export(0))
    genesis_data["chain_id"] = "ibc-2"
    upgrade_time = datetime.utcnow() + timedelta(seconds=5)
    upgrade_time.replace(tzinfo=None).isoformat("T") + "Z"
    genesis_data["genesis-time"] = str(upgrade_time)
    cluster["ibc-1"].update_genesis(0, genesis_data)
    cluster["ibc-1"].start_node(0)

    rsp = json.loads(subprocess.check_output(relayer + ["chains", "list", "--json"]))
    # the chain-id in relayer config is changed
    assert rsp == [
        {
            "key": "relayer",
            "chain-id": "ibc-0",
            "rpc-addr": "http://localhost:26657",
            "account-prefix": "cro",
            "gas-adjustment": 1.5,
            "gas-prices": "0.0basecro",
            "trusting-period": "336h",
        },
        {
            "key": "relayer",
            "chain-id": "ibc-2",
            "rpc-addr": "http://localhost:26757",
            "account-prefix": "cro",
            "gas-adjustment": 1.5,
            "gas-prices": "0.0basecro",
            "trusting-period": "336h",
        },
    ]

    # do a transfer from ibc-0 to ibc-2
    balance_0 = cluster["ibc-0"].balance(cluster["ibc-0"].address("relayer"))
    recipient = cluster["ibc-1"].address("relayer")
    rsp = cluster["ibc-0"].ibc_transfer(
        "relayer", recipient, "10000basecro", channels[0]["channel_id"], 1
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    time.sleep(10)
    # sender balance decreased
    assert (
        cluster["ibc-0"].balance(cluster["ibc-0"].address("relayer"))
        == balance_0 - 10000
    )
    # ibc-2 get the coins
    query = json.loads(
        raw(
            "query",
            "bank",
            "balances",
            recipient,
            node=cluster["ibc-1"].node_rpc(0),
        )
    )["balances"]
    assert query == [
        {"denom": "basecro", "amount": "10000000000"},
        {
            "denom": f"ibc/{denom_hash}",
            "amount": "10000",
        },
    ]
