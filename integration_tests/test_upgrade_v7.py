import json

from .tieredrewards_helpers import (
    get_node_validator_addr,
    lock_tier,
    query_positions_by_owner,
    query_tiers,
)
from .utils import wait_for_new_blocks


def assert_v7_inflation_module_is_working(cluster):
    cli = cluster.cosmos_cli()
    rsp = json.loads(
        cli.raw(
            "query",
            "inflation",
            "params",
            output="json",
            node=cli.node_rpc,
        )
    )

    rsp = rsp["params"]

    expected_max_supply = "10000000000000000000"  # 100B * 10^8
    assert rsp["max_supply"] == expected_max_supply, rsp["max_supply"]

    expected_burned_addresses = ["cro1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqtcgxmv"]
    assert rsp["burned_addresses"] == expected_burned_addresses, rsp["burned_addresses"]

    print("v7 upgrade completed successfully")


def assert_v7_tieredrewards_working(cluster):
    # Bank send smoke test
    community_addr = cluster.address("community")
    reserve_addr = cluster.address("reserve")
    old_balance = cluster.balance(reserve_addr, denom="basecro")
    cluster.transfer(
        community_addr,
        reserve_addr,
        "100000basecro",
    )
    new_balance = cluster.balance(reserve_addr, denom="basecro")
    assert (
        new_balance > old_balance
    ), f"bank send failed: {old_balance} -> {new_balance}"

    wait_for_new_blocks(cluster, 1)

    # Tiers are already created by the upgrade handler.
    tiers = query_tiers(cluster)
    tier_list = tiers.get("tiers", [])

    # Lock tier smoke test
    validator_addr = get_node_validator_addr(cluster)
    tier_id = tier_list[0]["id"]
    lock_amount = max(int(tier_list[0]["min_lock_amount"]), 1000000)
    rsp = lock_tier(cluster, reserve_addr, tier_id, lock_amount, validator_addr)
    assert rsp["code"] == 0, f"lock-tier failed: {rsp.get('raw_log', rsp)}"

    # Query positions
    rsp = query_positions_by_owner(cluster, reserve_addr)
    positions = rsp.get("positions", [])
    assert len(positions) == 1, f"expected 1 position, got {len(positions)}: {rsp}"

    wait_for_new_blocks(cluster, 1)

    print("v7 tieredrewards smoke test passed")
