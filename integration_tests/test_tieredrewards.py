import json
from datetime import timedelta
from pathlib import Path

import pytest
from dateutil.parser import isoparse

from .utils import (
    cluster_fixture,
    query_command,
    wait_for_block_time,
    wait_for_new_blocks,
)

DENOM = "basecro"

pytestmark = pytest.mark.base_rewards


@pytest.fixture(scope="module")
def cluster(worker_index, tmp_path_factory):
    "override cluster fixture for tiered rewards tests"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/tieredrewards.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _lock_tier(cluster, from_name, amount, tier_id="1", validator=None, i=0):
    """Lock tokens into a tier. Returns the tx response."""
    cli = cluster.cosmos_cli(i)
    kwargs = dict(
        from_=from_name,
        tier_id=tier_id,
        amount=f"{amount}{DENOM}",
    )
    if validator:
        kwargs["validator"] = validator
    return cli.tx("tieredrewards", "lock-tier", **kwargs)


def _get_latest_position(cluster, owner_addr):
    """Query the most recently created position for an owner."""
    result = query_command(
        cluster, "tieredrewards", "tier-positions-by-owner", owner_addr
    )
    positions = result["positions"]
    assert len(positions) > 0, "expected at least one position"
    return max(positions, key=lambda p: int(p["position_id"]))


def _query_position(cluster, position_id):
    """Query a single tier position by ID."""
    return query_command(cluster, "tieredrewards", "tier-position", str(position_id))[
        "position"
    ]


def _tier_pool_balance(cluster):
    """Return the tier reward pool balance in basecro (int)."""
    result = query_command(cluster, "tieredrewards", "tier-pool-balance")
    for b in result.get("balance", []):
        if b["denom"] == DENOM:
            return int(b["amount"])
    return 0


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_lock_tier(cluster):
    """Lock tokens into a tier and verify position created."""
    addr = cluster.address("signer1")
    balance_before = cluster.balance(addr, DENOM)

    rsp = _lock_tier(cluster, "signer1", 5000)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _get_latest_position(cluster, addr)
    assert pos["owner"] == addr
    assert int(pos["tier_id"]) == 1
    assert int(pos["amount_locked"]) == 5000

    balance_after = cluster.balance(addr, DENOM)
    assert balance_before - balance_after >= 5000


def test_lock_tier_with_delegate(cluster):
    """Lock tokens with immediate delegation to validator 0."""
    addr = cluster.address("signer1")
    val0 = cluster.validators()[0]["operator_address"]

    rsp = _lock_tier(cluster, "signer1", 5000, validator=val0)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _get_latest_position(cluster, addr)
    assert pos["validator"] == val0
    assert float(pos["delegated_shares"]) > 0


def test_tier_delegate(cluster):
    """Lock without validator, then delegate separately."""
    addr = cluster.address("signer1")
    val0 = cluster.validators()[0]["operator_address"]

    rsp = _lock_tier(cluster, "signer1", 5000)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _get_latest_position(cluster, addr)
    pos_id = pos["position_id"]
    # position should not yet be delegated
    assert not pos.get("validator") or pos["validator"] == ""

    cli = cluster.cosmos_cli()
    rsp = cli.tx(
        "tieredrewards",
        "tier-delegate",
        from_="signer1",
        position_id=str(pos_id),
        validator=val0,
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)
    assert pos["validator"] == val0
    assert float(pos["delegated_shares"]) > 0


def test_tier_redelegate(cluster):
    """Lock+delegate to validator 0, then redelegate to validator 1."""
    addr = cluster.address("signer1")
    validators = cluster.validators()
    val0 = validators[0]["operator_address"]
    val1 = validators[1]["operator_address"]

    rsp = _lock_tier(cluster, "signer1", 5000, validator=val0)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _get_latest_position(cluster, addr)
    pos_id = pos["position_id"]
    assert pos["validator"] == val0

    cli = cluster.cosmos_cli()
    rsp = cli.tx(
        "tieredrewards",
        "tier-redelegate",
        from_="signer1",
        position_id=str(pos_id),
        dst_validator=val1,
        gas="300000",
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)
    assert pos["validator"] == val1


def test_trigger_exit_and_withdraw_lifecycle(cluster):
    """Full lifecycle: lock+delegate -> trigger exit -> undelegate -> withdraw."""
    addr = cluster.address("signer1")
    val0 = cluster.validators()[0]["operator_address"]

    rsp = _lock_tier(cluster, "signer1", 5000, validator=val0)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _get_latest_position(cluster, addr)
    pos_id = pos["position_id"]

    cli = cluster.cosmos_cli()

    # trigger exit
    rsp = cli.tx(
        "tieredrewards",
        "trigger-exit-from-tier",
        from_="signer1",
        position_id=str(pos_id),
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)
    exit_at = pos.get("exit_triggered_at")
    assert exit_at and exit_at != "0001-01-01T00:00:00Z", "exit should be triggered"

    exit_unlock_time = isoparse(pos["exit_unlock_time"])

    # undelegate
    rsp = cli.tx(
        "tieredrewards",
        "tier-undelegate",
        from_="signer1",
        position_id=str(pos_id),
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)
    assert pos["is_unbonding"] is True

    # wait for both unbonding (10s) and exit commitment (15s) to elapse
    wait_for_block_time(cluster, exit_unlock_time + timedelta(seconds=2))

    balance_before = cluster.balance(addr, DENOM)

    # withdraw
    rsp = cli.tx(
        "tieredrewards",
        "withdraw-from-tier",
        from_="signer1",
        position_id=str(pos_id),
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = cluster.balance(addr, DENOM)
    assert balance_after > balance_before, "tokens should be returned"

    # position should be deleted — query returns non-zero exit code
    with pytest.raises(AssertionError):
        _query_position(cluster, pos_id)


def test_withdraw_tier_rewards(cluster):
    """Lock+delegate a large amount and withdraw staking rewards."""
    addr = cluster.address("signer1")
    val0 = cluster.validators()[0]["operator_address"]

    rsp = _lock_tier(cluster, "signer1", 100000, validator=val0)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _get_latest_position(cluster, addr)
    pos_id = pos["position_id"]

    # wait for staking rewards to accrue
    wait_for_new_blocks(cluster, 5)

    balance_before = cluster.balance(addr, DENOM)

    cli = cluster.cosmos_cli()
    rsp = cli.tx(
        "tieredrewards",
        "withdraw-tier-rewards",
        from_="signer1",
        position_id=str(pos_id),
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = cluster.balance(addr, DENOM)
    rewards = balance_after - balance_before
    assert rewards > 0, "rewards should increase balance"
    # 100k delegated for ~5 blocks; rewards should be modest, not astronomical
    assert rewards < 100000, f"rewards look unreasonably large: {rewards}"


def test_fund_tier_pool_rejected_for_non_authority(cluster):
    """Verify that funding the tier pool is rejected for non-authority senders."""
    cli = cluster.cosmos_cli()
    rsp = json.loads(
        cli.raw(
            "tx",
            "tieredrewards",
            "fund-tier-pool",
            "-y",
            from_="signer1",
            amount=f"10000{DENOM}",
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
        )
    )
    if rsp["code"] == 0:
        # tx passed CheckTx; wait for execution result
        rsp = cli.event_query_tx_for(rsp["txhash"])
    assert rsp["code"] != 0, "non-authority should be rejected"
    assert "unauthorized" in rsp.get(
        "raw_log", ""
    ), f"expected unauthorized error, got: {rsp.get('raw_log', '')}"


def test_add_to_tier_position(cluster):
    """Add tokens to an existing position."""
    addr = cluster.address("signer1")
    val0 = cluster.validators()[0]["operator_address"]

    rsp = _lock_tier(cluster, "signer1", 3000, validator=val0)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _get_latest_position(cluster, addr)
    pos_id = pos["position_id"]
    assert int(pos["amount_locked"]) == 3000

    cli = cluster.cosmos_cli()
    rsp = cli.tx(
        "tieredrewards",
        "add-to-tier-position",
        from_="signer1",
        position_id=str(pos_id),
        amount=f"2000{DENOM}",
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)
    assert int(pos["amount_locked"]) == 5000


def test_transfer_tier_position(cluster):
    """Transfer a position from signer1 to signer2."""
    addr1 = cluster.address("signer1")
    addr2 = cluster.address("signer2")

    rsp = _lock_tier(cluster, "signer1", 5000)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _get_latest_position(cluster, addr1)
    pos_id = pos["position_id"]

    cli = cluster.cosmos_cli()
    rsp = cli.tx(
        "tieredrewards",
        "transfer-tier-position",
        from_="signer1",
        position_id=str(pos_id),
        new_owner=addr2,
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # signer2 now owns it
    pos = _query_position(cluster, pos_id)
    assert pos["owner"] == addr2

    # signer1 should no longer have this position
    result = query_command(cluster, "tieredrewards", "tier-positions-by-owner", addr1)
    s1_ids = {str(p["position_id"]) for p in result["positions"]}
    assert str(pos_id) not in s1_ids


def test_commit_delegation_to_tier(cluster):
    """Commit an existing staking delegation to a tier position."""
    addr = cluster.address("signer1")
    val0 = cluster.validators()[0]["operator_address"]

    # normal staking delegation
    rsp = cluster.delegate_amount(val0, f"5000{DENOM}", addr)
    assert rsp["code"] == 0, rsp["raw_log"]

    # ensure delegation is committed before attempting to lock it
    wait_for_new_blocks(cluster, 2)

    # commit it to a tier
    cli = cluster.cosmos_cli()
    rsp = cli.tx(
        "tieredrewards",
        "commit-delegation-to-tier",
        from_="signer1",
        tier_id="1",
        validator=val0,
        amount=f"5000{DENOM}",
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _get_latest_position(cluster, addr)
    assert pos["validator"] == val0
    assert float(pos["delegated_shares"]) > 0


def test_tier_voting_power_query(cluster):
    """Query tier voting power for an address with delegated positions."""
    addr = cluster.address("signer1")
    result = query_command(cluster, "tieredrewards", "tier-voting-power", addr)
    vp = result["voting_power"]
    assert float(vp) > 0, f"expected non-zero voting power, got {vp}"


def test_positions_query_pagination(cluster):
    """Create positions and verify paginated queries work."""
    # ensure at least 3 positions exist (previous tests already created many)
    for _ in range(3):
        rsp = _lock_tier(cluster, "signer1", 1000)
        assert rsp["code"] == 0, rsp["raw_log"]

    cli = cluster.cosmos_cli()
    result = json.loads(
        cli.raw(
            "query",
            "tieredrewards",
            "all-tier-positions",
            home=cli.data_dir,
            output="json",
            page_limit="2",
        )
    )
    assert len(result["positions"]) == 2
    assert result["pagination"].get("next_key"), "expected a next_key for pagination"


def test_wrong_owner_rejected(cluster):
    """Verify that a non-owner cannot trigger exit on someone else's position."""
    addr1 = cluster.address("signer1")

    rsp = _lock_tier(cluster, "signer1", 5000)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _get_latest_position(cluster, addr1)
    pos_id = pos["position_id"]

    # signer2 attempts to trigger exit on signer1's position.
    # Use wait_tx=False to avoid hanging if the tx is rejected at CheckTx.
    cli = cluster.cosmos_cli()
    rsp = json.loads(
        cli.raw(
            "tx",
            "tieredrewards",
            "trigger-exit-from-tier",
            "-y",
            from_="signer2",
            position_id=str(pos_id),
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
        )
    )
    if rsp["code"] == 0:
        # tx passed CheckTx; wait for execution result
        rsp = cli.event_query_tx_for(rsp["txhash"])
    assert rsp["code"] != 0, "non-owner should be rejected"
    assert "not position owner" in rsp.get(
        "raw_log", ""
    ), f"expected ownership error, got: {rsp.get('raw_log', '')}"
