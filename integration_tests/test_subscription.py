from datetime import datetime, timezone

from .utils import wait_for_block_time


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
    )
    assert rsp["code"] == 0, f"create plan failed: {rsp['raw_log']}"
    plan_id = int(rsp["logs"][0]["events"][0]["attributes"][0]["value"])
    print("plan id", plan_id)
    plan = cli.query_plan(plan_id)
    print("plan created", plan)
    rsp = cli.create_subscription(plan_id, subscriber)
    assert rsp["code"] == 0, f"create plan failed: {rsp['raw_log']}"
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
