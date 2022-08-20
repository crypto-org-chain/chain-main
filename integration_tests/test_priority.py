from pathlib import Path

import pytest

from .cosmoscli import ClusterCLI
from .utils import cluster_fixture, wait_for_new_blocks

PRIORITY_REDUCTION = 1000000


pytestmark = pytest.mark.normal


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/mempool.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def test_priority(cluster: ClusterCLI):
    """
    Check that prioritized mempool works, and the priority is decided by gas price.
    """
    cli = cluster.cosmos_cli()
    test_cases = [
        {
            "from": cli.address("community"),
            "to": cli.address("validator"),
            "amount": "1000aphoton",
            "gas_prices": "10basecro",
            # if the priority is decided by fee, this tx will have the highest priority,
            # if by gas price, it's the lowest.
            "gas": 200000 * 10,
        },
        {
            "from": cli.address("signer1"),
            "to": cli.address("signer2"),
            "amount": "1000aphoton",
            "gas_prices": "20basecro",
            "gas": 200000,
        },
        {
            "from": cli.address("signer2"),
            "to": cli.address("signer1"),
            "amount": "1000aphoton",
            "gas_prices": "30basecro",
            "gas": 200000,
        },
    ]
    txs = []
    for tc in test_cases:
        tx = cli.transfer(
            tc["from"],
            tc["to"],
            tc["amount"],
            gas_prices=tc["gas_prices"],
            generate_only=True,
            gas=tc["gas"],
        )
        txs.append(
            cli.sign_tx_json(
                tx, tc["from"], max_priority_price=tc.get("max_priority_price")
            )
        )

    # wait for the beginning of a new block, so the window of time is biggest
    # before the next block get proposed.
    wait_for_new_blocks(cli, 1)

    txhashes = []
    for tx in txs:
        rsp = cli.broadcast_tx_json(tx, broadcast_mode="sync")
        assert rsp["code"] == 0, rsp["raw_log"]
        txhashes.append(rsp["txhash"])

    print("wait for two new blocks, so the sent txs are all included")
    wait_for_new_blocks(cli, 2)

    tx_results = [cli.tx_search_rpc(f"tx.hash='{txhash}'")[0] for txhash in txhashes]
    tx_indexes = [(int(r["height"]), r["index"]) for r in tx_results]
    print(tx_indexes)
    # the first sent tx are included later, because of reversed priority order
    assert all(i1 > i2 for i1, i2 in zip(tx_indexes, tx_indexes[1:]))
