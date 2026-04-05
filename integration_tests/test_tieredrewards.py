import json
import time
from datetime import timedelta
from decimal import Decimal
from pathlib import Path

import pytest
import requests
from dateutil.parser import isoparse
from pystarport.ports import api_port, rpc_port

from .utils import (
    approve_proposal,
    cluster_fixture,
    find_log_event_attrs,
    module_address,
    query_command,
    wait_for_block_time,
    wait_for_new_blocks,
    wait_for_port,
)

# ──────────────────────────────────────────────
# Constants
# ──────────────────────────────────────────────
MODULE = "tieredrewards"
DENOM = "basecro"
REWARDS_POOL_NAME = "rewards_pool"

# Msg type URLs
MSG_UPDATE_PARAMS = "/chainmain.tieredrewards.v1.MsgUpdateParams"
MSG_ADD_TIER = "/chainmain.tieredrewards.v1.MsgAddTier"
MSG_UPDATE_TIER = "/chainmain.tieredrewards.v1.MsgUpdateTier"
MSG_DELETE_TIER = "/chainmain.tieredrewards.v1.MsgDeleteTier"

# Genesis tiers (from tieredrewards.jsonnet)
TIER_1_ID = 1  # exit_duration=5s,  bonus_apy=4%,  min_lock=1_000_000 basecro
TIER_2_ID = 2  # exit_duration=30s, bonus_apy=2%,  min_lock=5_000_000 basecro
TIER_1_MIN = 1_000_000
TIER_2_MIN = 5_000_000

# Governance tier added/deleted in Group G tests
TIER_3_ID = 3

# Gas slack for the single withdraw transaction in test_full_exit_flow.
# balance_before is captured after lock/trigger/undelegate, so only the
# withdraw gas needs to be covered here (~500_000 gas units).
GAS_ALLOWANCE = 5_000_000  # basecro (conservative upper bound)

pytestmark = [pytest.mark.tieredrewards]


# ──────────────────────────────────────────────
# Cluster fixture
# ──────────────────────────────────────────────


@pytest.fixture(scope="module")
def cluster(worker_index, tmp_path_factory):
    "override cluster fixture for tieredrewards tests"
    yield from cluster_fixture(
        Path(__file__).parent / "configs/tieredrewards.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


@pytest.fixture(scope="function")
def slashing_cluster(worker_index, tmp_path_factory):
    """Use a fresh cluster so validator power shifts from earlier tests do not halt
    consensus.
    """
    yield from cluster_fixture(
        Path(__file__).parent / "configs/tieredrewards.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("d"),
    )


# ──────────────────────────────────────────────
# Helper functions
# ──────────────────────────────────────────────


def _rest_get(cluster, path, i=0):
    """GET from the REST API of validator i and return parsed JSON."""
    port = api_port(cluster.base_port(i))
    resp = requests.get(f"http://127.0.0.1:{port}{path}", timeout=10)
    resp.raise_for_status()
    return resp.json()


def _tx(cluster, *subcmd, from_, i=0, **extra):
    """Execute a tieredrewards tx, wait for inclusion, return response.

    Retries event_query_tx_for up to 3 times to tolerate the WebSocket race
    where the tx lands in a block before the subscription is established,
    causing chain-maind to exit with code 1 and empty stdout.
    """
    cli = cluster.cosmos_cli(i)
    rsp = json.loads(
        cli.raw(
            "tx",
            MODULE,
            *subcmd,
            "-y",
            from_=from_,
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
            chain_id=cli.chain_id,
            output="json",
            gas=500000,
            **extra,
        )
    )
    if rsp["code"] == 0:
        txhash = rsp["txhash"]
        for attempt in range(3):
            try:
                rsp = cli.event_query_tx_for(txhash)
                break
            except Exception:
                if attempt == 2:
                    raise
                time.sleep(3 * (attempt + 1))
    return rsp


def _lock_tier(cluster, owner, tier_id, amount, validator, trigger_exit=False, i=0):
    args = ["lock-tier", str(tier_id), str(amount), validator]
    if trigger_exit:
        args.append("--trigger-exit-immediately")
    return _tx(cluster, *args, from_=owner, i=i)


def _tier_undelegate(cluster, owner, position_id, i=0):
    return _tx(cluster, "tier-undelegate", str(position_id), from_=owner, i=i)


def _tier_delegate(cluster, owner, position_id, validator, i=0):
    return _tx(cluster, "tier-delegate", str(position_id), validator, from_=owner, i=i)


def _tier_redelegate(cluster, owner, position_id, dst_validator, i=0):
    return _tx(
        cluster, "tier-redelegate", str(position_id), dst_validator, from_=owner, i=i
    )


def _trigger_exit(cluster, owner, position_id, i=0):
    return _tx(cluster, "trigger-exit", str(position_id), from_=owner, i=i)


def _claim_rewards(cluster, owner, position_id, i=0):
    return _tx(cluster, "claim-tier-rewards", str(position_id), from_=owner, i=i)


def _add_to_position(cluster, owner, position_id, amount, i=0):
    return _tx(
        cluster, "add-to-tier-position", str(position_id), str(amount), from_=owner, i=i
    )


def _clear_position(cluster, owner, position_id, i=0):
    return _tx(cluster, "clear-position", str(position_id), from_=owner, i=i)


def _withdraw(cluster, owner, position_id, i=0):
    return _tx(cluster, "withdraw-from-tier", str(position_id), from_=owner, i=i)


def _broadcast_msg_tx(cluster, signer_name, msg, i=0, gas=300000, fees="5000basecro"):
    """Broadcast a manually-crafted message tx by re-signing a generate-only tx."""
    cli = cluster.cosmos_cli(i)
    signer_addr = cluster.address(signer_name)

    tx = cli.transfer(
        signer_addr,
        signer_addr,
        f"1{DENOM}",
        generate_only=True,
        wait_tx=False,
        gas=gas,
        fees=fees,
    )
    tx["body"]["messages"] = [msg]

    signed = cli.sign_tx_json(tx, signer_addr)
    return cli.broadcast_tx_json(signed)


def _fund_pool(cluster, from_name, amount_coin):
    """Fund the rewards pool via a bank send to the module account."""
    from_addr = cluster.address(from_name)
    pool_addr = module_address(REWARDS_POOL_NAME)
    return cluster.transfer(from_addr, pool_addr, amount_coin)


def _fund_pool_via_cli(cluster, from_name, amount_coin, i=0):
    return _tx(cluster, "fund-tier-pool", amount_coin, from_=from_name, i=i)


def _fund_pool_via_msg(cluster, from_name, amount, i=0):
    return _broadcast_msg_tx(
        cluster,
        from_name,
        {
            "@type": "/chainmain.tieredrewards.v1.MsgFundTierPool",
            "depositor": cluster.address(from_name),
            "amount": [{"denom": DENOM, "amount": str(amount)}],
        },
        i=i,
    )


def _commit_delegation(
    cluster, delegator, validator, amount, tier_id, trigger_exit=False, i=0
):
    args = ["commit-delegation-to-tier", validator, str(amount), str(tier_id)]
    if trigger_exit:
        args.append("--trigger-exit-immediately")
    return _tx(cluster, *args, from_=delegator, i=i)


def _query_position(cluster, position_id, i=0):
    return _rest_get(cluster, f"/chainmain/tieredrewards/v1/position/{position_id}", i)


def _query_positions_by_owner(cluster, owner, i=0):
    try:
        return _rest_get(cluster, f"/chainmain/tieredrewards/v1/positions/{owner}", i)
    except requests.HTTPError as exc:
        if exc.response.status_code == 404:
            return {"positions": []}
        raise


def _query_tiers(cluster, i=0):
    return _rest_get(cluster, "/chainmain/tieredrewards/v1/tiers", i)


def _query_estimate_rewards(cluster, position_id, i=0):
    return _rest_get(
        cluster, f"/chainmain/tieredrewards/v1/estimate_rewards/{position_id}", i
    )


def _pool_balance(cluster):
    pool_addr = module_address(REWARDS_POOL_NAME)
    return cluster.balance(pool_addr, DENOM)


def _assert_pool_balance_increased(balance_before, balance_after, source):
    assert balance_after > balance_before, (
        f"pool balance should increase after a successful {source} funding tx: "
        f"before={balance_before}, after={balance_after}"
    )


def _assert_pool_received_amount(rsp, amount):
    """Assert the rewards pool received the exact requested amount."""
    pool_addr = module_address(REWARDS_POOL_NAME)
    expected_amount = f"{amount}{DENOM}"

    ev = find_log_event_attrs(
        rsp["events"],
        "coin_received",
        lambda attrs: attrs.get("receiver") == pool_addr
        and attrs.get("amount") == expected_amount,
    )
    assert (
        ev is not None
    ), f"expected rewards pool to receive {expected_amount}: events={rsp['events']}"


def _assert_pool_fund_tx(rsp, depositor, amount):
    """Assert MsgFundTierPool funded the pool with the exact requested amount."""
    expected_amount = f"{amount}{DENOM}"

    ev = find_log_event_attrs(
        rsp["events"],
        "coin_spent",
        lambda attrs: attrs.get("spender") == depositor
        and attrs.get("amount") == expected_amount,
    )
    assert ev is not None, (
        f"expected depositor {depositor} to spend {expected_amount}: "
        f"events={rsp['events']}"
    )

    _assert_pool_received_amount(rsp, amount)

    ev = find_log_event_attrs(
        rsp["events"],
        "message",
        lambda attrs: attrs.get("action")
        == "/chainmain.tieredrewards.v1.MsgFundTierPool",
    )
    assert ev is not None, f"expected MsgFundTierPool event: events={rsp['events']}"


def _submit_gov_tier_proposal(cluster, proposer, msg_type, msg_body, title, summary):
    authority = module_address("gov")
    proposal = {
        "messages": [{"@type": msg_type, "authority": authority, **msg_body}],
        "deposit": "100000000basecro",
        "title": title,
        "summary": summary,
    }
    rsp = cluster.gov_propose_since_cosmos_sdk_v0_50(
        proposer, "submit-proposal", proposal
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    return rsp


def _approve_tieredrewards_proposal(cluster, rsp, msg):
    return approve_proposal(
        cluster,
        rsp,
        msg=msg,
        top_up_deposit_in_voting_period=False,
    )


def _get_validator_addr(cluster, i=0):
    """Return the operator address of validator i."""
    return cluster.validators()[i]["operator_address"]


def _get_node_validator_addr(cluster, i=0):
    """Return the operator address for a specific node index."""
    return cluster.address("validator", i=i, bech="val")


def _before_ids(cluster, owner, i=0):
    """Capture current position IDs for an owner (for before/after diff)."""
    return {
        int(p["id"])
        for p in _query_positions_by_owner(cluster, owner, i).get("positions", [])
    }


def _new_pos_id(cluster, owner, before, i=0):
    """Find the single new position ID created since before was captured.

    Fails if there is not exactly one new position for owner.
    """
    result = _query_positions_by_owner(cluster, owner, i)
    after = {int(p["id"]) for p in result.get("positions", [])}
    new = after - before
    assert len(new) == 1, (
        f"Expected exactly 1 new position for {owner}, got {new} "
        f"(before={before}, after={after})"
    )
    return next(iter(new))


def test_commit_delegation_to_tier(cluster):
    """Convert an existing staking delegation into a tier position without undelegating.

    Verifies:
    - The tier position is created with the committed amount and validator.
    - The owner's x/staking delegation to the same validator decreases by commit_amount.
    """
    owner = cluster.address("signer1")
    validator = _get_validator_addr(cluster, 0)

    # First: delegate normally via x/staking
    staking_amount = TIER_1_MIN * 5
    cli = cluster.cosmos_cli()
    rsp = json.loads(
        cli.raw(
            "tx",
            "staking",
            "delegate",
            validator,
            f"{staking_amount}{DENOM}",
            "-y",
            from_=owner,
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
            chain_id=cli.chain_id,
            output="json",
            gas=300000,
        )
    )
    if rsp["code"] == 0:
        rsp = cli.event_query_tx_for(rsp["txhash"])
    assert rsp["code"] == 0, rsp["raw_log"]

    wait_for_new_blocks(cluster, 1)

    # Record staking delegation before commit
    del_before = json.loads(
        cli.raw(
            "q",
            "staking",
            "delegation",
            owner,
            validator,
            output="json",
            home=cli.data_dir,
            node=cli.node_rpc,
        )
    )
    shares_before = Decimal(del_before["delegation_response"]["delegation"]["shares"])

    # Then: commit part of that delegation to a tier position
    commit_amount = TIER_1_MIN * 2
    before = _before_ids(cluster, owner)
    rsp = _commit_delegation(cluster, owner, validator, commit_amount, TIER_1_ID)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    pos = _query_position(cluster, pos_id)["position"]
    assert pos["validator"] == validator, "committed position should be delegated"
    assert (
        int(pos["amount"]) == commit_amount
    ), f"committed amount mismatch: expected {commit_amount}, got {pos['amount']}"

    # Staking delegation should have decreased by approximately commit_amount
    del_after = json.loads(
        cli.raw(
            "q",
            "staking",
            "delegation",
            owner,
            validator,
            output="json",
            home=cli.data_dir,
            node=cli.node_rpc,
        )
    )
    shares_after = Decimal(del_after["delegation_response"]["delegation"]["shares"])
    assert shares_after < shares_before, (
        f"staking delegation should decrease after commit: "
        f"before={shares_before}, after={shares_after}"
    )


def test_commit_delegation_with_exit(cluster):
    """CommitDelegationToTier with trigger_exit_immediately sets exit_triggered_at."""
    owner = cluster.address("signer1")
    validator = _get_validator_addr(cluster, 0)
    staking_amount = TIER_1_MIN * 5
    commit_amount = TIER_1_MIN * 2

    # First: delegate normally via x/staking
    cli = cluster.cosmos_cli()
    rsp = json.loads(
        cli.raw(
            "tx",
            "staking",
            "delegate",
            validator,
            f"{staking_amount}{DENOM}",
            "-y",
            from_=owner,
            home=cli.data_dir,
            node=cli.node_rpc,
            keyring_backend="test",
            chain_id=cli.chain_id,
            output="json",
            gas=300000,
        )
    )
    if rsp["code"] == 0:
        rsp = cli.event_query_tx_for(rsp["txhash"])
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cluster, 1)

    # Commit with trigger_exit_immediately=true
    before = _before_ids(cluster, owner)
    rsp = _commit_delegation(
        cluster, owner, validator, commit_amount, TIER_1_ID, trigger_exit=True
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    pos = _query_position(cluster, pos_id)["position"]
    assert pos["validator"] == validator, "committed position should be delegated"
    assert (
        pos.get("exit_triggered_at")
        and pos["exit_triggered_at"] != "0001-01-01T00:00:00Z"
    ), "exit_triggered_at should be set when trigger_exit_immediately=true"


# ──────────────────────────────────────────────
# Exit Flow (ADR-006 §5.6, §5.7)
# ──────────────────────────────────────────────


def test_full_exit_flow(cluster):
    """Full exit lifecycle: lock → trigger-exit → wait 5s → undelegate → withdraw.

    Uses Tier 1 (5s exit) + 10s unbonding time from genesis.jsonnet.
    """
    owner = cluster.address("ecosystem")
    validator = _get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 3

    # 1. Lock and delegate
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # 2. Trigger exit; read exit_unlock_at from position
    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])

    # 3. Wait for exit duration (5s) to elapse
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    # 4. Undelegate (allowed because exit was triggered, §5.4)
    rsp = _tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Parse unbonding completion_time from EventPositionUndelegated
    unbond_data = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    assert unbond_data is not None, (
        "EventPositionUndelegated with completion_time not found in "
        "tier-undelegate response"
    )
    completion_time = isoparse(unbond_data["completion_time"].strip('"')) + timedelta(
        seconds=1
    )

    # 5. Wait for unbonding to complete using chain time (unbonding_time = 10s from
    #    genesis.jsonnet)
    wait_for_block_time(cluster, completion_time)
    wait_for_new_blocks(cluster, 1)

    # 6. Withdraw — position deleted, tokens returned
    balance_before = cluster.balance(owner, DENOM)
    rsp = _withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = cluster.balance(owner, DENOM)
    # User receives back approximately `amount` minus gas fees across all txs
    assert balance_after >= balance_before + amount - GAS_ALLOWANCE, (
        f"expected balance increase of ~{amount} after withdraw: "
        f"before={balance_before}, after={balance_after}"
    )

    # Position should no longer exist after withdraw
    try:
        _query_position(cluster, pos_id)
        assert False, f"position {pos_id} should be deleted after withdraw"
    except requests.HTTPError as exc:
        # REST gateway may return 404 (not found) or 500 (wrapping not-found)
        assert exc.response.status_code in (
            404,
            500,
        ), f"expected 404/500 for deleted position, got {exc.response.status_code}"
        assert (
            "not found" in exc.response.text.lower()
        ), f"expected 'not found' error body, got: {exc.response.text}"


# ──────────────────────────────────────────────
# Rewards (ADR-006 §1, §4, §5.8)
# ──────────────────────────────────────────────


def test_fund_pool_via_cli(cluster):
    """fund-tier-pool CLI should fund the rewards pool with the requested amount."""
    fund_amount = 10_000_000

    balance_before = _pool_balance(cluster)
    rsp = _fund_pool_via_cli(cluster, "signer1", f"{fund_amount}{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]
    _assert_pool_fund_tx(rsp, cluster.address("signer1"), fund_amount)

    balance_after = _pool_balance(cluster)
    _assert_pool_balance_increased(balance_before, balance_after, "fund-tier-pool CLI")


def test_claim_rewards_delegated(cluster):
    """claim-tier-rewards on a delegated position distributes nonzero rewards."""
    owner = cluster.address("signer2")
    validator = _get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 5
    fund_amount = 50_000_000

    # Fund pool to ensure bonus rewards are available
    rsp = _fund_pool(cluster, "signer1", f"{fund_amount}{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    # Create delegated position
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Wait for rewards to accrue (more blocks = more rewards)
    wait_for_new_blocks(cluster, 10)

    balance_before = cluster.balance(owner, DENOM)
    rsp = _claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = cluster.balance(owner, DENOM)
    # Rewards must be positive — balance strictly greater after claim
    assert balance_after > balance_before, (
        f"owner balance must increase after claiming rewards: "
        f"before={balance_before}, after={balance_after}"
    )

    # EventTierRewardsClaimed must be present in tx events
    ev = find_log_event_attrs(
        rsp["events"], "chainmain.tieredrewards.v1.EventTierRewardsClaimed"
    )
    assert ev is not None, "EventTierRewardsClaimed not found in tx events"
    assert (
        "position_id" in ev
    ), f"EventTierRewardsClaimed missing position_id field: {ev}"


def test_bonus_stops_after_exit_unlock(cluster):
    """After exit_unlock_at passes, estimated bonus rewards drop to 0.

    Uses a large delegated position to verify:
    1. Bonus > 0 before exit_unlock_at (after initializing LastBonusAccrual)
    2. Bonus = 0 after claiming post-exit_unlock_at (LastBonusAccrual capped)

    Note: bonus requires LastBonusAccrual to be non-zero (set by first claim).
    A fresh position always shows 0 until the first claim initializes it.
    """
    owner = cluster.address("signer1")
    validator = _get_validator_addr(cluster, 0)
    # Large position to accumulate measurable bonus in a few blocks.
    # With 1B basecro at 4% APY, bonus ≈ 1B*0.04*T/31,557,600 basecro.
    # After 15 blocks (~15s worst-case): ~19 basecro → reliably > 0.
    amount = TIER_1_MIN * 1000  # 1_000_000_000 basecro

    # Fund pool to ensure bonus rewards are available
    rsp = _fund_pool(cluster, "signer1", f"1000000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    # Create delegated position in Tier 1 (5s exit, 4% APY bonus)
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # First claim initializes LastBonusAccrual (bonus = 0 here, field was unset).
    # Without this, calculateBonusRaw returns 0 for all subsequent estimates.
    rsp = _claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Wait blocks so bonus accrues since LastBonusAccrual was set
    wait_for_new_blocks(cluster, 15)

    # Verify nonzero bonus estimate BEFORE triggering exit
    est_before = _query_estimate_rewards(cluster, pos_id)
    bonus_before_list = est_before.get("bonus_rewards", [])
    bonus_before = sum(int(c.get("amount", "0")) for c in bonus_before_list)
    assert bonus_before > 0, (
        f"delegated position should have nonzero bonus before exit_unlock_at, "
        f"got bonus_rewards={bonus_before_list}"
    )

    # Trigger exit
    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])

    # Wait past exit_unlock_at (Tier 1 has 5s exit duration)
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 2)

    # Claim after exit_unlock_at: LastBonusAccrual is capped and set to ExitUnlockAt.
    # This settles all remaining bonus (from LastBonusAccrual up to ExitUnlockAt).
    balance_before_claim = cluster.balance(owner, DENOM)
    rsp = _claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Final claim must actually deliver tokens to the owner
    balance_after_claim = cluster.balance(owner, DENOM)
    assert balance_after_claim > balance_before_claim, (
        f"final claim after exit_unlock_at should increase owner balance: "
        f"before={balance_before_claim}, after={balance_after_claim}"
    )

    # Estimate: accrualEnd=ExitUnlockAt, LastBonusAccrual=ExitUnlockAt → bonus=0
    est_after = _query_estimate_rewards(cluster, pos_id)
    bonus_after_list = est_after.get("bonus_rewards", [])
    bonus_after = sum(int(c.get("amount", "0")) for c in bonus_after_list)
    assert bonus_after == 0, (
        f"bonus rewards must be 0 after final claim post-exit_unlock_at, "
        f"got {bonus_after_list}"
    )


def test_clear_exit_then_add_to_position(cluster):
    """Clearing an exited position settles rewards, then allows adding again."""
    owner = cluster.address("signer1")
    validator = _get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 1000
    add_amount = TIER_1_MIN * 2

    rsp = _fund_pool(cluster, "signer1", f"1000000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Initialize LastBonusAccrual, then let rewards build before entering exit mode.
    rsp = _claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cluster, 10)

    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos_before_clear = _query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos_before_clear["exit_unlock_at"])
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    est_before_clear = _query_estimate_rewards(cluster, pos_id)
    bonus_before = sum(
        int(c.get("amount", "0")) for c in est_before_clear.get("bonus_rewards", [])
    )
    assert bonus_before > 0, "bonus should be pending before clearing exit"

    balance_before_clear = cluster.balance(owner, DENOM)
    rsp = _clear_position(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    balance_after_clear = cluster.balance(owner, DENOM)
    assert (
        balance_after_clear > balance_before_clear
    ), "clear-position should settle rewards"

    pos_after_clear = _query_position(cluster, pos_id)["position"]
    assert (
        pos_after_clear["exit_triggered_at"] == "0001-01-01T00:00:00Z"
    ), "exit_triggered_at should be cleared"
    assert (
        pos_after_clear["exit_unlock_at"] == "0001-01-01T00:00:00Z"
    ), "exit_unlock_at should be cleared"

    est_after_clear = _query_estimate_rewards(cluster, pos_id)
    bonus_after = sum(
        int(c.get("amount", "0")) for c in est_after_clear.get("bonus_rewards", [])
    )
    assert (
        bonus_after <= bonus_before
    ), "clear-position should not increase the pending bonus window"

    add_rsp = _add_to_position(cluster, owner, pos_id, add_amount)
    assert add_rsp["code"] == 0, add_rsp["raw_log"]

    pos_after_add = _query_position(cluster, pos_id)["position"]
    assert int(pos_after_add["amount"]) > int(
        pos_after_clear["amount"]
    ), "position amount should grow after add-to-tier-position"


def test_tier_redelegate_flow(cluster):
    """Redelegating moves a delegated position to the destination validator."""
    owner = cluster.address("signer2")
    src_validator = _get_validator_addr(cluster, 0)
    dst_validator = _get_validator_addr(cluster, 1)
    amount = TIER_1_MIN * 1000

    rsp = _fund_pool(cluster, "signer1", f"500000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, amount, validator=src_validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Initialize LastBonusAccrual before checking reward settlement in redelegate.
    rsp = _claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cluster, 5)

    balance_before = cluster.balance(owner, DENOM)
    rsp = _tier_redelegate(cluster, owner, pos_id, dst_validator)
    assert rsp["code"] == 0, rsp["raw_log"]

    ev = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionRedelegated",
        lambda attrs: "completion_time" in attrs,
    )
    assert ev is not None, "EventPositionRedelegated should be emitted"
    assert ev["dst_validator"].strip('"') == dst_validator

    pos = _query_position(cluster, pos_id)["position"]
    assert pos["validator"] == dst_validator, "position should move to dst validator"
    assert (
        pos["delegated_shares"] != "0.000000000000000000"
    ), "position should remain delegated"

    balance_after = cluster.balance(owner, DENOM)
    assert balance_after > balance_before, "redelegation should settle pending rewards"

    wait_for_new_blocks(cluster, 2)
    rsp = _claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]


# ──────────────────────────────────────────────
# Governance (ADR-006 §7)
#
# NOTE: These tests have an intentional ordering dependency:
#   add_tier → update_tier (open) → delete_tier (succeeds)
# pytest runs tests in definition order within a module, so this is stable.
# ──────────────────────────────────────────────


def test_add_tier_via_governance(cluster):
    """MsgAddTier proposal creates a new Tier 3 with close_only=true."""
    rsp = _submit_gov_tier_proposal(
        cluster,
        "community",
        MSG_ADD_TIER,
        {
            "tier": {
                "id": TIER_3_ID,
                "exit_duration": "5s",
                "bonus_apy": "0.050000000000000000",
                "min_lock_amount": "2000000",
                "close_only": True,
            }
        },
        title="Add Tier 3 (close_only)",
        summary="Add testing tier — initially close_only to verify ErrTierIsCloseOnly",
    )
    _approve_tieredrewards_proposal(cluster, rsp, msg=f",{MSG_ADD_TIER}")

    result = _query_tiers(cluster)
    ids = {int(t["id"]) for t in result.get("tiers", [])}
    assert TIER_3_ID in ids, f"Tier {TIER_3_ID} not found after AddTier proposal"

    # Verify it's close_only
    tier3 = next(
        (t for t in result.get("tiers", []) if int(t["id"]) == TIER_3_ID), None
    )
    assert tier3 is not None
    assert tier3.get("close_only") is True, f"Tier 3 should be close_only: {tier3}"


def test_update_tier_via_governance(cluster):
    """MsgUpdateTier proposal updates Tier 3's bonus_apy and sets close_only=false."""
    new_apy = "0.060000000000000000"
    rsp = _submit_gov_tier_proposal(
        cluster,
        "community",
        MSG_UPDATE_TIER,
        {
            "tier": {
                "id": TIER_3_ID,
                "exit_duration": "5s",
                "bonus_apy": new_apy,
                "min_lock_amount": "2000000",
                "close_only": False,
            }
        },
        title="Update Tier 3",
        summary="Update Tier 3 bonus_apy to 6% and remove close_only restriction",
    )
    _approve_tieredrewards_proposal(cluster, rsp, msg=f",{MSG_UPDATE_TIER}")

    result = _query_tiers(cluster)
    tier3 = next(
        (t for t in result.get("tiers", []) if int(t["id"]) == TIER_3_ID), None
    )
    assert tier3 is not None, "Tier 3 not found after update"
    assert (
        tier3["bonus_apy"] == new_apy
    ), f"expected bonus_apy={new_apy}, got {tier3['bonus_apy']}"
    assert (
        tier3.get("close_only") is not True
    ), f"Tier 3 should not be close_only after update: {tier3}"


def test_delete_tier_via_governance(cluster):
    """MsgDeleteTier proposal removes Tier 3 (which has no active positions)."""
    rsp = _submit_gov_tier_proposal(
        cluster,
        "community",
        MSG_DELETE_TIER,
        {"id": TIER_3_ID},
        title="Delete Tier 3",
        summary="Remove Tier 3 — no active positions",
    )
    _approve_tieredrewards_proposal(cluster, rsp, msg=f",{MSG_DELETE_TIER}")

    result = _query_tiers(cluster)
    ids = {int(t["id"]) for t in result.get("tiers", [])}
    assert (
        TIER_3_ID not in ids
    ), f"Tier {TIER_3_ID} should be removed after DeleteTier proposal"


# ──────────────────────────────────────────────
# Params Governance (run last to avoid poisoning earlier tests)
#
# NOTE: This test permanently sets target_base_rewards_rate to 0 for the
# remainder of the test session. It is placed last so that all reward-
# dependent tests run with the original rate.
# ──────────────────────────────────────────────


def test_update_params_via_governance(cluster):
    """MsgUpdateParams proposal sets target_base_rewards_rate to 0."""
    params = query_command(cluster, MODULE, "params")["params"]
    params["target_base_rewards_rate"] = "0.000000000000000000"

    authority = module_address("gov")
    proposal = {
        "messages": [
            {
                "@type": MSG_UPDATE_PARAMS,
                "authority": authority,
                "params": params,
            }
        ],
        "deposit": "100000000basecro",
        "title": "Set rate to zero",
        "summary": "Disable base rewards top-up",
    }
    rsp = cluster.gov_propose_since_cosmos_sdk_v0_50(
        "community", "submit-proposal", proposal
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    _approve_tieredrewards_proposal(cluster, rsp, msg=f",{MSG_UPDATE_PARAMS}")

    updated = query_command(cluster, MODULE, "params")["params"]
    assert (
        float(updated["target_base_rewards_rate"]) == 0.0
    ), "target_base_rewards_rate should be 0 after governance update"


def test_fund_pool_via_msg(cluster):
    """MsgFundTierPool should fund the rewards pool without using the autocli path."""
    fund_amount = 7_000_000

    balance_before = _pool_balance(cluster)
    rsp = _fund_pool_via_msg(cluster, "signer1", fund_amount)
    assert rsp["code"] == 0, rsp["raw_log"]
    _assert_pool_fund_tx(rsp, cluster.address("signer1"), fund_amount)

    balance_after = _pool_balance(cluster)
    _assert_pool_balance_increased(balance_before, balance_after, "MsgFundTierPool")


@pytest.mark.slow
def test_slash_then_withdraw_succeeds(slashing_cluster):
    """Slashed delegated position still exits, undelegates, and withdraws cleanly."""
    cluster = slashing_cluster
    owner = cluster.address("signer1")
    validator = _get_node_validator_addr(cluster, 2)
    amount = TIER_1_MIN * 20

    rsp = _fund_pool(cluster, "signer1", f"100000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]
    assert _pool_balance(cluster) > 0

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(
        cluster,
        owner,
        TIER_1_ID,
        amount,
        validator=validator,
        trigger_exit=True,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    pos_before_slash = _query_position(cluster, pos_id)["position"]
    amount_before_slash = int(pos_before_slash["amount"])
    exit_unlock_at = isoparse(pos_before_slash["exit_unlock_at"])

    val_before = cluster.validator(validator)
    tokens_before = int(val_before["tokens"])

    wait_for_new_blocks(cluster, 5)
    cluster.supervisor.stopProcess(f"{cluster.chain_id}-node2")
    wait_for_new_blocks(cluster, 20)
    cluster.supervisor.startProcess(f"{cluster.chain_id}-node2")
    wait_for_port(rpc_port(cluster.base_port(2)))
    wait_for_new_blocks(cluster, 2)

    val_after = cluster.validator(validator)
    tokens_after = int(val_after["tokens"])
    assert tokens_after == int(
        tokens_before * 0.99
    ), "validator should be slashed by 1%"
    assert val_after.get("jailed"), "validator should be jailed after downtime slash"

    pos_after_slash = _query_position(cluster, pos_id)["position"]
    amount_after_slash = int(pos_after_slash["amount"])
    assert (
        amount_after_slash < amount_before_slash
    ), "position amount should decrease after validator slash"
    assert (
        pos_after_slash["delegated_shares"] != "0.000000000000000000"
    ), "position should remain delegated after slash"

    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    rsp = _tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    unbond_data = find_log_event_attrs(
        rsp["events"],
        "chainmain.tieredrewards.v1.EventPositionUndelegated",
        lambda attrs: "completion_time" in attrs,
    )
    assert unbond_data is not None, "undelegate should emit completion_time"
    completion_time = isoparse(unbond_data["completion_time"].strip('"')) + timedelta(
        seconds=1
    )

    wait_for_block_time(cluster, completion_time)
    wait_for_new_blocks(cluster, 1)

    pos_after_undelegate = _query_position(cluster, pos_id)["position"]
    withdraw_amount = int(pos_after_undelegate["amount"])
    assert withdraw_amount <= amount_after_slash

    balance_before = cluster.balance(owner, DENOM)
    rsp = _withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = cluster.balance(owner, DENOM)
    assert balance_after >= balance_before + withdraw_amount - GAS_ALLOWANCE, (
        f"expected balance increase of ~{withdraw_amount}: "
        f"before={balance_before}, after={balance_after}"
    )

    try:
        _query_position(cluster, pos_id)
        assert False, f"position {pos_id} should be deleted after withdraw"
    except requests.HTTPError as exc:
        assert exc.response.status_code in (404, 500)
        assert "not found" in exc.response.text.lower()


def test_autocli_lock_tier_and_queries(cluster):
    """Smoke test tieredrewards autocli tx/query paths end-to-end."""
    owner = cluster.address("ecosystem")
    validator = _get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 2

    before = _before_ids(cluster, owner)
    rsp = _tx(cluster, "lock-tier", str(TIER_1_ID), str(amount), validator, from_=owner)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    position_rsp = query_command(cluster, MODULE, "position", str(pos_id))
    rest_position_rsp = _query_position(cluster, pos_id)
    assert position_rsp == rest_position_rsp

    position = position_rsp["position"]
    assert position["owner"] == owner
    assert int(position["tier_id"]) == TIER_1_ID
    assert position["validator"] == validator
    assert int(position["amount"]) == amount

    owner_positions_rsp = query_command(cluster, MODULE, "positions-by-owner", owner)
    rest_owner_positions_rsp = _query_positions_by_owner(cluster, owner)
    assert owner_positions_rsp == rest_owner_positions_rsp

    owner_positions = owner_positions_rsp.get("positions", [])
    assert len(owner_positions) == len(before) + 1
    assert any(
        p["owner"] == owner
        and int(p["tier_id"]) == TIER_1_ID
        and p["validator"] == validator
        and int(p["amount"]) == amount
        for p in owner_positions
    ), "positions-by-owner should include the newly created delegated position"

    tiers_rsp = query_command(cluster, MODULE, "tiers")
    rest_tiers_rsp = _query_tiers(cluster)
    cli_tiers = {
        **tiers_rsp,
        "tiers": [
            {
                **tier,
                "close_only": tier.get("close_only", False),
            }
            for tier in tiers_rsp.get("tiers", [])
        ],
    }
    rest_tiers = {
        **rest_tiers_rsp,
        "tiers": [
            {
                **tier,
                "close_only": tier.get("close_only", False),
            }
            for tier in rest_tiers_rsp.get("tiers", [])
        ],
    }
    assert cli_tiers == rest_tiers

    tier_ids = {int(t["id"]) for t in tiers_rsp.get("tiers", [])}
    assert TIER_1_ID in tier_ids
    assert TIER_2_ID in tier_ids

    rewards_rsp = query_command(
        cluster, MODULE, "estimate-position-rewards", str(pos_id)
    )
    rest_rewards_rsp = _query_estimate_rewards(cluster, pos_id)
    assert rewards_rsp == rest_rewards_rsp

    pool_balance_rsp = query_command(cluster, MODULE, "pool-balance")
    cli_pool_amount = sum(
        int(coin["amount"])
        for coin in pool_balance_rsp.get("balance", [])
        if coin["denom"] == DENOM
    )
    assert cli_pool_amount == _pool_balance(cluster)
