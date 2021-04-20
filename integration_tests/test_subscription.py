from datetime import datetime, timezone
from pathlib import Path

import pytest

from .utils import cluster_fixture, wait_for_block_time


@pytest.fixture(scope="module")
def cluster(worker_index, pytestconfig, tmp_path_factory):
    "override cluster fixture for this test module"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/subscription.yaml",
        worker_index,
        tmp_path_factory,
        quiet=pytestconfig.getoption("supervisord-quiet"),
    )


def test_subscription(cluster):
    cli = cluster.cosmos_cli(0)
    owner = cli.address("community")
    subscriber = cli.address("ecosystem")
    rsp = cli.create_plan(
        owner,
        "test plan",
        "test description",
        "10000000basecro",
        "* * * * *",
        120,
        fees="1693basecro",
        gas=67707,
    )
    assert rsp["code"] == 0, f"create plan failed: {rsp['raw_log']}"
    plan_id = int(rsp["logs"][0]["events"][0]["attributes"][0]["value"])
    print("plan id", plan_id)
    plan = cli.query_plan(plan_id)
    print("plan created", plan)
    rsp = cli.create_subscription(plan_id, subscriber, fees="3420basecro", gas="136778")
    assert rsp["code"] == 0, f"create subscription failed: {rsp['raw_log']}"
    subscription_id = int(rsp["logs"][0]["events"][0]["attributes"][0]["value"])
    print("subscription id", subscription_id)
    subscription = cli.query_subscription(subscription_id)
    print("subscription created", subscription)
    collection_time = int(subscription["next_collection_time"])
    amount = cli.balance(subscriber)
    owner_amount = cli.balance(owner)
    wait_for_block_time(
        cluster,
        datetime.utcfromtimestamp(collection_time + 1).replace(tzinfo=timezone.utc),
    )
    subscription = cli.query_subscription(subscription_id)
    print("subscription updated", subscription)
    assert int(subscription["next_collection_time"]) > collection_time
    assert cli.balance(subscriber) == amount - 10000000
    assert cli.balance(owner) == owner_amount + 10000000
