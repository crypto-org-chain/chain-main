import datetime
from pathlib import Path

import pytest
import yaml
from dateutil.parser import isoparse

from .utils import cluster_fixture, wait_for_block_time, wait_for_new_blocks

"""
slashing testing
"""

# http://doc.pytest.org/en/latest/example/markers.html#marking-whole-classes-or-modules
pytestmark = pytest.mark.asyncio


# use custom cluster, use an unique base port
@pytest.fixture(scope="module")
async def cluster():
    async for v in cluster_fixture(
        yaml.safe_load(open(Path(__file__).parent / "configs/slashing_cluster.yml")),
        26700,
    ):
        yield v


async def test_slashing(cluster):
    "stop node2, wait for non-live slashing"
    addr = await cluster.cli.address("validator", i=2)
    val_addr = await cluster.cli.address("validator", i=2, bech="val")
    tokens1 = int((await cluster.cli.validator(val_addr))["tokens"])

    print("tokens before slashing", tokens1)
    print("stop and wait for 10 blocks")
    cluster.supervisor.stopProcess("node2")
    await wait_for_new_blocks(cluster.cli, 10)
    cluster.supervisor.startProcess("node2")

    val = await cluster.cli.validator(val_addr)
    tokens2 = int(val["tokens"])
    print("tokens after slashing", tokens2)
    assert tokens2 == int(tokens1 * 0.99), "slash amount is not correct"

    assert val["jailed"], "validator is jailed"

    # try to unjail
    rsp = await cluster.cli.unjail(addr, i=2)
    assert rsp["code"] == 4, "still jailed, can't be unjailed"

    # wait for 60s and unjail again
    await wait_for_block_time(
        cluster.cli, isoparse(val["unbonding_time"]) + datetime.timedelta(seconds=60)
    )
    rsp = await cluster.cli.unjail(addr, i=2)
    assert rsp["code"] == 0, f"unjail should success {rsp}"

    await wait_for_new_blocks(cluster.cli, 3)
    assert len(await cluster.cli.validators()) == 3
