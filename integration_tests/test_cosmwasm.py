import json
from pathlib import Path

import pytest

from .utils import BLOCK_BROADCASTING

pytestmark = pytest.mark.normal


def test_cosmwasm(cluster):
    """
    - upload a wasm contract (cw_nameservice.wasm from cosmwasm tutorial)
    - instantiate contract
    - execute some contract functions
    - verify results
    """

    signer_addr = cluster.address("community")
    receiver_addr = cluster.address("reserve")

    wasm_path = Path(__file__).parent / "contracts/cw_nameservice.wasm"

    cli = cluster.cosmos_cli()

    # check there aren't any codes uploded yet
    rsp = json.loads(
        cli.raw(
            "query",
            "wasm",
            "list-code",
            home=cli.data_dir,
            node=cli.node_rpc,
            output="json",
        )
    )

    assert len(rsp["code_infos"]) == 0

    # upload a code
    rsp = json.loads(
        cli.raw(
            "tx",
            "wasm",
            "store",
            wasm_path,
            "--gas",
            "5000000",
            "-y",
            from_="community",
            home=cli.data_dir,
            node=cli.node_rpc,
            chain_id=cli.chain_id,
            output="json",
            broadcast_mode=BLOCK_BROADCASTING,
        )
    )

    print(rsp)

    # code_id of uploaded code
    code_id = rsp["logs"][0]["events"][-1]["attributes"][0]["value"]

    # check there aren't any instantiated contracts for above code_id
    rsp = json.loads(
        cli.raw(
            "query",
            "wasm",
            "list-contract-by-code",
            code_id,
            home=cli.data_dir,
            node=cli.node_rpc,
            output="json",
        )
    )

    assert len(rsp["contracts"]) == 0

    # instantiate a contract
    init = {
        "purchase_price": {"amount": "100", "denom": "basecro"},
        "transfer_price": {"amount": "999", "denom": "basecro"},
    }

    rsp = json.loads(
        cli.raw(
            "tx",
            "wasm",
            "instantiate",
            code_id,
            json.dumps(init),
            "--label",
            "awesome nameservice",
            "--admin",
            signer_addr,
            "-y",
            from_="community",
            home=cli.data_dir,
            node=cli.node_rpc,
            chain_id=cli.chain_id,
            output="json",
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # get contract address of instantiated contract
    rsp = json.loads(
        cli.raw(
            "query",
            "wasm",
            "list-contract-by-code",
            code_id,
            home=cli.data_dir,
            node=cli.node_rpc,
            output="json",
        )
    )

    contract = rsp["contracts"][0]

    # register a name by paying its price
    register = {"register": {"name": "fred"}}

    rsp = json.loads(
        cli.raw(
            "tx",
            "wasm",
            "execute",
            contract,
            json.dumps(register),
            "--amount",
            "100basecro",
            "-y",
            from_="community",
            home=cli.data_dir,
            node=cli.node_rpc,
            chain_id=cli.chain_id,
            output="json",
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # check that the name is registered with signer's (community) address
    name_query = {"resolve_record": {"name": "fred"}}

    rsp = json.loads(
        cli.raw(
            "query",
            "wasm",
            "contract-state",
            "smart",
            contract,
            json.dumps(name_query),
            home=cli.data_dir,
            node=cli.node_rpc,
            output="json",
        )
    )

    assert rsp["data"]["address"] == signer_addr

    # transfer the name to receiver's (reserve) address
    transfer = {"transfer": {"name": "fred", "to": receiver_addr}}

    rsp = json.loads(
        cli.raw(
            "tx",
            "wasm",
            "execute",
            contract,
            json.dumps(transfer),
            "--amount",
            "999basecro",
            "-y",
            from_="community",
            home=cli.data_dir,
            node=cli.node_rpc,
            chain_id=cli.chain_id,
            output="json",
        )
    )

    assert rsp["code"] == 0, rsp["raw_log"]

    # check that the name is registered with receiver's (reserve) address
    rsp = json.loads(
        cli.raw(
            "query",
            "wasm",
            "contract-state",
            "smart",
            contract,
            json.dumps(name_query),
            home=cli.data_dir,
            node=cli.node_rpc,
            output="json",
        )
    )

    assert rsp["data"]["address"] == receiver_addr
