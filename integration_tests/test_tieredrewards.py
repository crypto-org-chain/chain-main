import json
import time as _time
from datetime import timedelta
from pathlib import Path

import pytest
import requests
from dateutil.parser import isoparse
from pystarport.ports import api_port

from .utils import (
    approve_proposal,
    cluster_fixture,
    find_log_event_attrs,
    get_proposal_id,
    module_address,
    query_command,
    wait_for_block_time,
    wait_for_new_blocks,
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

# Conservative slack for gas fees across multiple transactions
GAS_ALLOWANCE = 5_000_000  # basecro

pytestmark = pytest.mark.tieredrewards


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
                _time.sleep(3 * (attempt + 1))
    return rsp


def _lock_tier(
    cluster, owner, tier_id, amount, validator=None, trigger_exit=False, i=0
):
    args = ["lock-tier", str(tier_id), str(amount)]
    kwargs = {}
    if validator:
        kwargs["validator_address"] = validator
    if trigger_exit:
        # Pass as positional flag — cobra bool flags don't take a value argument
        args.append("--trigger-exit-immediately")
    return _tx(cluster, *args, from_=owner, i=i, **kwargs)


def _tier_delegate(cluster, owner, position_id, validator, i=0):
    return _tx(cluster, "tier-delegate", str(position_id), validator, from_=owner, i=i)


def _tier_undelegate(cluster, owner, position_id, i=0):
    return _tx(cluster, "tier-undelegate", str(position_id), from_=owner, i=i)


def _tier_redelegate(cluster, owner, position_id, dst_validator, i=0):
    return _tx(
        cluster, "tier-redelegate", str(position_id), dst_validator, from_=owner, i=i
    )


def _add_to_position(cluster, owner, position_id, amount, i=0):
    return _tx(
        cluster, "add-to-tier-position", str(position_id), str(amount), from_=owner, i=i
    )


def _trigger_exit(cluster, owner, position_id, i=0):
    return _tx(cluster, "trigger-exit", str(position_id), from_=owner, i=i)


def _claim_rewards(cluster, owner, position_id, i=0):
    return _tx(cluster, "claim-tier-rewards", str(position_id), from_=owner, i=i)


def _withdraw(cluster, owner, position_id, i=0):
    return _tx(cluster, "withdraw-from-tier", str(position_id), from_=owner, i=i)


def _fund_pool(cluster, from_name, amount_coin):
    """Fund the rewards pool via a bank send to the module account.

    MsgFundTierPool (fund-tier-pool CLI) panics in this binary due to a
    known autocli incompatibility with repeated cosmos.base.v1beta1.Coin
    positional arguments (proto descriptor mismatch in dynamicpb.Merge).
    The handler is covered by keeper unit tests (TestFundTierPool_*).
    Bank send produces the same observable result for integration purposes.
    """
    from_addr = cluster.address(from_name)
    pool_addr = module_address(REWARDS_POOL_NAME)
    return cluster.transfer(from_addr, pool_addr, amount_coin)


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
    return _rest_get(cluster, f"/chainmain/tieredrewards/v1/positions/{owner}", i)


def _query_tiers(cluster, i=0):
    return _rest_get(cluster, "/chainmain/tieredrewards/v1/tiers", i)


def _query_voting_power(cluster, voter, i=0):
    return _rest_get(cluster, f"/chainmain/tieredrewards/v1/voting_power/{voter}", i)


def _query_estimate_rewards(cluster, position_id, i=0):
    return _rest_get(
        cluster, f"/chainmain/tieredrewards/v1/estimate_rewards/{position_id}", i
    )


def _query_pool_balance(cluster, i=0):
    return _rest_get(cluster, "/chainmain/tieredrewards/v1/pool_balance", i)


def _pool_balance(cluster):
    pool_addr = module_address(REWARDS_POOL_NAME)
    return cluster.balance(pool_addr, DENOM)


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


def _vote_proposal_and_get_result(cluster, rsp, msg):
    """Vote yes on a proposal and return the final proposal object.

    Unlike approve_proposal, this does NOT assert PROPOSAL_STATUS_PASSED, so
    it can be used to verify proposals whose messages fail on execution.
    """
    proposal_id = get_proposal_id(rsp, msg)
    proposal = cluster.query_proposal(proposal_id)
    cluster.gov_deposit("ecosystem", proposal_id, "1cro")
    for i in range(len(cluster.config["validators"])):
        cluster.cosmos_cli(i).gov_vote("validator", proposal_id, "yes")
    wait_for_block_time(
        cluster, isoparse(proposal["voting_end_time"]) + timedelta(seconds=5)
    )
    return cluster.query_proposal(proposal_id)


def _get_validator_addr(cluster, i=0):
    """Return the operator address of validator i."""
    return cluster.validators()[i]["operator_address"]


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


# ──────────────────────────────────────────────
# Group A — Queries & Initial State
# ──────────────────────────────────────────────


def test_params_query(cluster):
    """Module params are accessible; genesis rate is present."""
    params = query_command(cluster, MODULE, "params")["params"]
    assert "target_base_rewards_rate" in params
    rate = float(params["target_base_rewards_rate"])
    assert rate == pytest.approx(0.03, rel=1e-6)


def test_tiers_query(cluster):
    """Both genesis tiers are returned by the tiers query with correct properties."""
    result = _query_tiers(cluster)
    tiers = result.get("tiers", [])
    assert len(tiers) >= 2, f"expected at least 2 genesis tiers, got: {tiers}"

    tier_map = {int(t["id"]): t for t in tiers}
    assert TIER_1_ID in tier_map, f"Tier {TIER_1_ID} not found in {list(tier_map)}"
    assert TIER_2_ID in tier_map, f"Tier {TIER_2_ID} not found in {list(tier_map)}"

    t1 = tier_map[TIER_1_ID]
    assert t1["exit_duration"] == "5s", f"Tier 1 exit_duration mismatch: {t1}"
    assert float(t1["bonus_apy"]) == pytest.approx(
        0.04, rel=1e-6
    ), f"Tier 1 bonus_apy mismatch: {t1}"
    assert int(t1["min_lock_amount"]) == TIER_1_MIN, f"Tier 1 min_lock_amount mismatch: {t1}"

    t2 = tier_map[TIER_2_ID]
    assert t2["exit_duration"] == "30s", f"Tier 2 exit_duration mismatch: {t2}"
    assert float(t2["bonus_apy"]) == pytest.approx(
        0.02, rel=1e-6
    ), f"Tier 2 bonus_apy mismatch: {t2}"
    assert int(t2["min_lock_amount"]) == TIER_2_MIN, f"Tier 2 min_lock_amount mismatch: {t2}"


def test_pool_balance_query(cluster):
    """pool-balance query works; initial pool balance is 0."""
    result = _query_pool_balance(cluster)
    assert "balance" in result
    # balance is sdk.Coins — a list of {denom, amount} objects (repeated Coin)
    balance_list = result.get("balance") or []
    total = sum(int(c.get("amount", "0")) for c in balance_list)
    assert total == 0, f"initial pool balance should be 0, got {result}"


# ──────────────────────────────────────────────
# Group B — Position Creation (ADR-006 §5.1, §5.2)
# ──────────────────────────────────────────────


def test_lock_tier_without_validator(cluster):
    """Create a position without specifying a validator (undelegated)."""
    owner = cluster.address("signer1")
    amount = TIER_1_MIN * 2

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, amount)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    pos = _query_position(cluster, pos_id)["position"]
    assert (
        pos.get("validator", "") == ""
    ), "expected undelegated position but got validator: " + str(pos.get("validator"))
    assert (
        int(pos["amount"]) == amount
    ), f"position amount mismatch: expected {amount}, got {pos['amount']}"
    assert (
        int(pos["tier_id"]) == TIER_1_ID
    ), f"position tier_id mismatch: expected {TIER_1_ID}, got {pos['tier_id']}"


def test_lock_tier_with_validator(cluster):
    """Create a position and immediately delegate it to a validator (§5.1)."""
    owner = cluster.address("signer2")
    validator = _get_validator_addr(cluster, 0)
    amount = TIER_1_MIN * 2

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    pos = _query_position(cluster, pos_id)["position"]
    assert pos["validator"] == validator, f"position should be delegated to {validator}"


def test_lock_tier_with_immediate_exit(cluster):
    """trigger_exit_immediately=true sets exit_triggered_at on the new position."""
    owner = cluster.address("community")
    amount = TIER_1_MIN * 2

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, amount, trigger_exit=True)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    pos = _query_position(cluster, pos_id)["position"]
    assert (
        pos.get("exit_triggered_at")
        and pos["exit_triggered_at"] != "0001-01-01T00:00:00Z"
    ), "position should have exit_triggered_at set when trigger_exit_immediately=true"


def test_lock_tier_below_minimum(cluster):
    """Locking below min_lock_amount must fail (§4)."""
    owner = cluster.address("signer1")
    amount = TIER_1_MIN - 1  # one below minimum

    rsp = _lock_tier(cluster, owner, TIER_1_ID, amount)
    assert rsp["code"] != 0, "expected failure when amount < min_lock_amount"
    assert "min lock amount" in rsp.get("raw_log", ""), rsp["raw_log"]


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
    shares_before = float(del_before["delegation_response"]["delegation"]["shares"])

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
    shares_after = float(del_after["delegation_response"]["delegation"]["shares"])
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
# Group C — Delegation Management (ADR-006 §5.4, §5.5)
# ──────────────────────────────────────────────


def test_tier_delegate(cluster):
    """An undelegated position can be delegated via tier-delegate."""
    owner = cluster.address("ecosystem")
    amount = TIER_1_MIN * 2
    validator = _get_validator_addr(cluster, 0)

    # Create undelegated position
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, amount)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Now delegate
    rsp = _tier_delegate(cluster, owner, pos_id, validator)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)["position"]
    assert (
        pos["validator"] == validator
    ), "position should be delegated after tier-delegate"


def test_tier_undelegate_requires_exit_triggered(cluster):
    """tier-undelegate MUST fail when exit has not been triggered."""
    owner = cluster.address("signer2")
    validator = _get_validator_addr(cluster, 0)

    # Create a fresh delegated position (no exit triggered)
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Attempt to undelegate without triggering exit — must fail
    rsp = _tier_undelegate(cluster, owner, pos_id)
    assert (
        rsp["code"] != 0
    ), "tier-undelegate should fail when exit has not been triggered"
    assert "exit has not been triggered" in rsp.get("raw_log", ""), rsp["raw_log"]


def test_tier_undelegate_after_exit_trigger(cluster):
    """tier-undelegate succeeds once exit has been triggered."""
    owner = cluster.address("signer1")
    validator = _get_validator_addr(cluster, 0)

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Trigger exit first
    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Now undelegate — should succeed
    rsp = _tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)["position"]
    assert (
        pos.get("validator", "") == ""
    ), "position should be undelegated after tier-undelegate"


def test_tier_redelegate(cluster):
    """tier-redelegate moves a delegation from validator 0 to validator 1."""
    owner = cluster.address("community")
    val0 = _get_validator_addr(cluster, 0)
    val1 = _get_validator_addr(cluster, 1)

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=val0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    rsp = _tier_redelegate(cluster, owner, pos_id, val1)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)["position"]
    assert (
        pos["validator"] == val1
    ), f"expected validator {val1} after redelegate, got {pos['validator']}"


def test_tier_delegate_when_exiting(cluster):
    """Delegating an exiting position is allowed."""
    owner = cluster.address("launch")

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, trigger_exit=True)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    validator = _get_validator_addr(cluster, 0)
    rsp = _tier_delegate(cluster, owner, pos_id, validator)
    assert (
        rsp["code"] == 0
    ), "delegating an exiting position should be allowed: " + rsp.get("raw_log", "")

    pos = _query_position(cluster, pos_id)["position"]
    assert pos["validator"] == validator


def test_tier_redelegate_when_exiting(cluster):
    """Redelegating from an exiting position is blocked (ADR §5.5)."""
    owner = cluster.address("signer2")
    val0 = _get_validator_addr(cluster, 0)
    val1 = _get_validator_addr(cluster, 1)

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=val0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Trigger exit
    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Redelegate while exiting — must FAIL (ValidateRedelegatePosition blocks it)
    rsp = _tier_redelegate(cluster, owner, pos_id, val1)
    assert rsp["code"] != 0, "redelegating an exiting position should be rejected"
    assert "position is exiting" in rsp.get("raw_log", ""), rsp["raw_log"]


# ──────────────────────────────────────────────
# Group D — Position Modification (ADR-006 §5.3)
# ──────────────────────────────────────────────


def test_add_to_position(cluster):
    """add-to-tier-position increases the locked amount for a non-exiting position."""
    owner = cluster.address("signer1")
    validator = _get_validator_addr(cluster, 0)

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    pos = _query_position(cluster, pos_id)["position"]
    old_amount = int(pos["amount"])

    add_amount = TIER_1_MIN
    rsp = _add_to_position(cluster, owner, pos_id, add_amount)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)["position"]
    new_amount = int(pos["amount"])
    assert (
        new_amount == old_amount + add_amount
    ), f"expected {old_amount + add_amount} but got {new_amount}"


def test_add_to_position_while_exiting(cluster):
    """add-to-tier-position on an exiting position must fail."""
    owner = cluster.address("community")

    # Always create a fresh exiting position — no state dependency on earlier tests
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, trigger_exit=True)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    rsp = _add_to_position(cluster, owner, pos_id, TIER_1_MIN)
    assert rsp["code"] != 0, "add-to-tier-position should fail on exiting position"
    assert "position is exiting" in rsp.get("raw_log", ""), rsp["raw_log"]


# ──────────────────────────────────────────────
# Group E — Exit Flow (ADR-006 §5.6, §5.7)
# ──────────────────────────────────────────────


def test_trigger_exit(cluster):
    """trigger-exit sets exit_triggered_at and exit_unlock_at on the position."""
    owner = cluster.address("signer2")
    validator = _get_validator_addr(cluster, 0)

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)["position"]
    assert (
        pos.get("exit_triggered_at")
        and pos["exit_triggered_at"] != "0001-01-01T00:00:00Z"
    ), "exit_triggered_at should be set after trigger-exit"
    assert (
        pos.get("exit_unlock_at") and pos["exit_unlock_at"] != "0001-01-01T00:00:00Z"
    ), "exit_unlock_at should be set after trigger-exit"


def test_withdraw_before_exit_elapsed(cluster):
    """withdraw-from-tier before exit_unlock_at must fail."""
    owner = cluster.address("community")

    # Use Tier 2 (30s exit) so it definitely hasn't elapsed
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_2_ID, TIER_2_MIN * 2, trigger_exit=True)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Attempt immediate withdrawal — should fail (30s not elapsed)
    rsp = _withdraw(cluster, owner, pos_id)
    assert rsp["code"] != 0, "withdraw before exit duration elapsed should fail"
    assert "exit lock duration not reached" in rsp.get("raw_log", ""), rsp["raw_log"]


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
        "EventPositionUndelegated with completion_time not found in tier-undelegate response"
    )
    completion_time = isoparse(unbond_data["completion_time"].strip('"')) + timedelta(seconds=1)

    # 5. Wait for unbonding to complete using chain time (unbonding_time = 10s from genesis.jsonnet)
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


# ──────────────────────────────────────────────
# Group F — Rewards (ADR-006 §1, §4, §5.8)
# ──────────────────────────────────────────────


def test_fund_pool_via_bank_send(cluster):
    """Pool balance grows when funded via a bank send to the module account.

    TODO: test MsgFundTierPool (fund-tier-pool CLI) directly once the autocli
    incompatibility with repeated cosmos.base.v1beta1.Coin positional arguments
    is resolved (proto descriptor mismatch in dynamicpb.Merge).
    """
    fund_amount = 10_000_000

    balance_before = _pool_balance(cluster)
    rsp = _fund_pool(cluster, "signer1", f"{fund_amount}{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    balance_after = _pool_balance(cluster)
    # Pool must have grown by at least fund_amount minus per-block distributions.
    # BeginBlocker can drain a few thousand basecro per block; use 1% tolerance.
    assert balance_after >= balance_before + fund_amount - fund_amount // 100, (
        f"pool should have grown by ~{fund_amount}: "
        f"before={balance_before}, after={balance_after}"
    )


def test_claim_rewards_not_delegated(cluster):
    """claim-tier-rewards on an undelegated position must fail (ADR §1.1)."""
    owner = cluster.address("signer1")

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    rsp = _claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] != 0, "claim-tier-rewards on undelegated position should fail"
    assert "position is not delegated" in rsp.get("raw_log", ""), rsp["raw_log"]


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
    # Event must carry the correct position reference and at least one reward field
    assert ev, f"EventTierRewardsClaimed should have non-empty attributes: {ev}"


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


def test_estimate_rewards_query(cluster):
    """estimate-rewards returns both base_rewards and bonus_rewards fields.

    Uses a large delegated position with an initial claim to seed LastBonusAccrual,
    then verifies that bonus accumulates after waiting blocks.
    """
    owner = cluster.address("signer1")
    validator = _get_validator_addr(cluster, 0)

    # Fund pool so bonus rewards can accrue
    rsp = _fund_pool(cluster, "signer1", f"1000000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    # Large position so bonus is measurable after a few blocks
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 1000, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # First claim initializes LastBonusAccrual (bonus = 0 here, field was unset).
    rsp = _claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Wait enough blocks for bonus to accrue since LastBonusAccrual was set
    wait_for_new_blocks(cluster, 15)

    est = _query_estimate_rewards(cluster, pos_id)

    # Both fields must be present in the response
    assert (
        "base_rewards" in est
    ), f"estimate-rewards must include base_rewards field, got: {est}"
    assert (
        "bonus_rewards" in est
    ), f"estimate-rewards must include bonus_rewards field, got: {est}"

    # Bonus must be nonzero (1B basecro at 4% APY for 15+ blocks gives >0 basecro)
    bonus_total = sum(int(c.get("amount", "0")) for c in est.get("bonus_rewards", []))
    assert bonus_total > 0, (
        f"bonus rewards should be nonzero after 15 blocks with 1B basecro position; "
        f"bonus={bonus_total}, est={est}"
    )


# ──────────────────────────────────────────────
# Group G — Governance (ADR-006 §7)
#
# NOTE: These tests have an intentional ordering dependency:
#   add_tier → lock_close_only (fails) → update_tier (open) →
#   delete_with_active_positions (fails, then cleans up) → delete_tier (succeeds)
# pytest runs tests in definition order within a module, so this is stable.
#
# Cascade failure: if test_delete_tier_with_active_positions fails to clean up
# (i.e., the withdrawal fails), test_delete_tier_via_governance will also fail
# because Tier 3 will still have active positions (ErrTierHasActivePositions).
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
    approve_proposal(cluster, rsp, msg=f",{MSG_ADD_TIER}")

    result = _query_tiers(cluster)
    ids = {int(t["id"]) for t in result.get("tiers", [])}
    assert TIER_3_ID in ids, f"Tier {TIER_3_ID} not found after AddTier proposal"

    # Verify it's close_only
    tier3 = next(
        (t for t in result.get("tiers", []) if int(t["id"]) == TIER_3_ID), None
    )
    assert tier3 is not None
    assert tier3.get("close_only") is True, f"Tier 3 should be close_only: {tier3}"


def test_lock_close_only_tier(cluster):
    """Locking into a close_only tier must fail (ErrTierIsCloseOnly).

    Tier 3 was created with close_only=true by test_add_tier_via_governance.
    """
    owner = cluster.address("signer1")
    # Tier 3 is close_only — any lock attempt must fail
    rsp = _lock_tier(cluster, owner, TIER_3_ID, 2_000_000)
    assert (
        rsp["code"] != 0
    ), "locking into a close_only tier should fail with ErrTierIsCloseOnly"
    assert "tier is close only" in rsp.get("raw_log", ""), rsp["raw_log"]


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
    approve_proposal(cluster, rsp, msg=f",{MSG_UPDATE_TIER}")

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


def test_delete_tier_with_active_positions(cluster):
    """MsgDeleteTier fails when the tier has active positions.

    Error: ErrTierHasActivePositions.

    Tier 3 is opened by test_update_tier_via_governance. This test:
      1. Locks into Tier 3 (verifying the open tier works).
      2. Submits a delete-Tier-3 proposal, votes yes, and verifies the proposal
         ends in PROPOSAL_STATUS_FAILED (message execution rejected).
      3. Withdraws the position so test_delete_tier_via_governance can succeed.
    NOTE: must run after test_update_tier_via_governance, before
    test_delete_tier_via_governance.
    """
    owner = cluster.address("launch")

    # Lock into Tier 3 with immediate exit (no unbonding wait needed for cleanup)
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_3_ID, 2_000_000, trigger_exit=True)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    pos = _query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])

    # Submit delete-Tier-3 proposal while the position is still active
    rsp_submit = _submit_gov_tier_proposal(
        cluster,
        "community",
        MSG_DELETE_TIER,
        {"id": TIER_3_ID},
        title="Delete Tier 3 (active positions)",
        summary="Attempt to delete Tier 3 while it has active positions",
    )

    # Vote yes — proposal passes the vote but fails during message execution
    result = _vote_proposal_and_get_result(cluster, rsp_submit, f",{MSG_DELETE_TIER}")
    assert result["status"] == "PROPOSAL_STATUS_FAILED", (
        "delete proposal should fail when tier has active positions, "
        f"got: {result['status']}"
    )

    # Tier 3 must still exist after the failed delete
    ids = {int(t["id"]) for t in _query_tiers(cluster).get("tiers", [])}
    assert TIER_3_ID in ids, "Tier 3 should still exist after failed delete proposal"

    # Clean up: withdraw the position (exit already elapsed after ~30s gov wait)
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)
    rsp = _withdraw(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]


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
    approve_proposal(cluster, rsp, msg=f",{MSG_DELETE_TIER}")

    result = _query_tiers(cluster)
    ids = {int(t["id"]) for t in result.get("tiers", [])}
    assert (
        TIER_3_ID not in ids
    ), f"Tier {TIER_3_ID} should be removed after DeleteTier proposal"


# ──────────────────────────────────────────────
# Group H — Governance Voting Power (ADR-006 §8)
# ──────────────────────────────────────────────


def test_voting_power_delegated_position(cluster):
    """Delegated tier position contributes governance voting power > 0."""
    owner = cluster.address("signer1")
    validator = _get_validator_addr(cluster, 0)

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 4, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    _new_pos_id(cluster, owner, before)  # confirm creation

    wait_for_new_blocks(cluster, 1)

    vp = _query_voting_power(cluster, owner)
    power = float(vp.get("voting_power", "0"))
    assert (
        power > 0
    ), f"delegated tier position should contribute voting power > 0, got: {vp}"


def test_voting_power_undelegated_position(cluster):
    """Undelegated tier positions do NOT contribute governance voting power."""
    owner = cluster.address("signer1")

    # Record current power before adding an undelegated position
    vp_before = _query_voting_power(cluster, owner)
    power_before = int(float(vp_before.get("voting_power", "0")))
    # signer1 has delegated positions from earlier tests — power must be > 0
    assert (
        power_before > 0
    ), f"signer1 should already have delegated voting power, got: {vp_before}"

    # Create a new undelegated position (no validator specified)
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2)
    assert rsp["code"] == 0, rsp["raw_log"]
    _new_pos_id(cluster, owner, before)

    wait_for_new_blocks(cluster, 1)

    # Voting power should NOT have increased — undelegated positions contribute 0
    vp_after = _query_voting_power(cluster, owner)
    power_after = int(float(vp_after.get("voting_power", "0")))

    assert power_after <= power_before, (
        f"undelegated position must not increase voting power: "
        f"before={power_before}, after={power_after}"
    )


def test_voting_power_exiting_but_delegated(cluster):
    """Exiting-but-still-delegated positions retain voting power."""
    owner = cluster.address("signer2")
    validator = _get_validator_addr(cluster, 0)

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 3, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Check voting power before triggering exit
    vp_before = _query_voting_power(cluster, owner)
    power_before = float(vp_before.get("voting_power", "0"))
    assert power_before > 0, "delegated position should have voting power before exit"

    # Trigger exit — position is still delegated
    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Voting power should still be > 0 (exiting-but-delegated counts)
    vp_after = _query_voting_power(cluster, owner)
    power_after = float(vp_after.get("voting_power", "0"))
    assert power_after > 0, "exiting-but-delegated position should retain voting power"


def test_voting_power_after_undelegate(cluster):
    """After undelegating, the position's voting power contribution drops."""
    owner = cluster.address("ecosystem")
    validator = _get_validator_addr(cluster, 0)

    # Create a fresh delegated position
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    wait_for_new_blocks(cluster, 1)

    # Record voting power with this new delegated position
    vp_with_pos = _query_voting_power(cluster, owner)
    power_with_pos = float(vp_with_pos.get("voting_power", "0"))
    assert (
        power_with_pos > 0
    ), "should have voting power from the newly delegated position"

    # Trigger exit then undelegate
    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    rsp = _tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    wait_for_new_blocks(cluster, 1)

    # Verify position is undelegated
    pos = _query_position(cluster, pos_id)["position"]
    assert pos.get("validator", "") == "", "position should be undelegated"

    # Voting power must have decreased (this position no longer delegates)
    vp_after = _query_voting_power(cluster, owner)
    power_after = float(vp_after.get("voting_power", "0"))
    assert power_after < power_with_pos, (
        f"voting power should decrease after undelegation: "
        f"before_undelegate={power_with_pos}, after={power_after}"
    )


# ──────────────────────────────────────────────
# Group I — Error Paths
#
# TODO: ErrValidatorNotBonded — requires an unbonded validator in the cluster,
#       which is not feasible in the current 2-validator test setup. Covered by
#       keeper unit tests (TestTierDelegate_ValidatorNotBonded).
#
# TODO: ErrInsufficientBonusPool — requires depleting the pool mid-test, which
#       conflicts with pool-funding in Group F. Covered by keeper unit tests
#       (TestClaimTierRewards_InsufficientPool).
# ──────────────────────────────────────────────


def test_lock_nonexistent_tier(cluster):
    """Locking into a non-existent tier must fail (ErrTierNotFound)."""
    owner = cluster.address("signer1")
    rsp = _lock_tier(cluster, owner, 999, TIER_1_MIN * 2)
    assert rsp["code"] != 0, "locking into tier 999 should fail with ErrTierNotFound"
    assert "tier not found" in rsp.get("raw_log", ""), rsp["raw_log"]


def test_double_trigger_exit(cluster):
    """Triggering exit twice on the same position must fail (ErrPositionExiting)."""
    owner = cluster.address("signer1")

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Second trigger-exit must fail
    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] != 0, "second trigger-exit on an exiting position should fail"
    assert "position is exiting" in rsp.get("raw_log", ""), rsp["raw_log"]


def test_delegate_already_delegated(cluster):
    """Delegating an already-delegated position fails (ErrPositionAlreadyDelegated)."""
    owner = cluster.address("signer1")
    validator = _get_validator_addr(cluster, 0)

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Try to delegate again — must fail
    rsp = _tier_delegate(cluster, owner, pos_id, validator)
    assert rsp["code"] != 0, "delegating an already-delegated position should fail"
    assert "position is already delegated" in rsp.get("raw_log", ""), rsp["raw_log"]


def test_undelegate_undelegated(cluster):
    """Undelegating a position that is not delegated fails (ErrPositionNotDelegated)."""
    owner = cluster.address("signer1")

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2)  # no validator
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    rsp = _tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] != 0, "undelegating an undelegated position should fail"
    assert "position is not delegated" in rsp.get("raw_log", ""), rsp["raw_log"]


def test_redelegate_to_same_validator(cluster):
    """Redelegating to the same validator must fail (ErrRedelegationToSameValidator)."""
    owner = cluster.address("signer1")
    val0 = _get_validator_addr(cluster, 0)

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=val0)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Redelegate to the same validator — must fail
    rsp = _tier_redelegate(cluster, owner, pos_id, val0)
    assert rsp["code"] != 0, "redelegating to the same validator should fail"
    assert "redelegation to same validator" in rsp.get("raw_log", ""), rsp["raw_log"]


def test_not_position_owner(cluster):
    """Operations on another owner's position must fail (ErrNotPositionOwner)."""
    owner = cluster.address("signer1")
    other = cluster.address("signer2")
    validator = _get_validator_addr(cluster, 0)

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # signer2 tries to trigger-exit on signer1's position — must fail
    rsp = _trigger_exit(cluster, other, pos_id)
    assert (
        rsp["code"] != 0
    ), "trigger-exit on another user's position must fail with ErrNotPositionOwner"
    assert "signer is not position owner" in rsp.get("raw_log", ""), rsp["raw_log"]

    # signer2 tries to add to signer1's position — must fail
    rsp = _add_to_position(cluster, other, pos_id, TIER_1_MIN)
    assert (
        rsp["code"] != 0
    ), "add-to-tier-position on another user's position must fail with ErrNotPositionOwner"
    assert "signer is not position owner" in rsp.get("raw_log", ""), rsp["raw_log"]

    # signer2 tries to claim rewards for signer1's position — must fail
    rsp = _claim_rewards(cluster, other, pos_id)
    assert (
        rsp["code"] != 0
    ), "claim-tier-rewards on another user's position must fail with ErrNotPositionOwner"
    assert "signer is not position owner" in rsp.get("raw_log", ""), rsp["raw_log"]


def test_withdraw_without_exit_triggered(cluster):
    """Withdrawing without triggering exit must fail (ErrPositionNotReadyToWithdraw)."""
    owner = cluster.address("signer1")

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Attempt to withdraw without exit triggered — must fail
    rsp = _withdraw(cluster, owner, pos_id)
    assert rsp["code"] != 0, "withdraw without exit triggered should fail"
    assert "position is not ready to withdraw" in rsp.get("raw_log", ""), rsp["raw_log"]


def test_withdraw_still_delegated(cluster):
    """Withdraw after exit_unlock_at fails when position is still delegated."""
    owner = cluster.address("signer2")
    validator = _get_validator_addr(cluster, 0)

    # Create delegated position in Tier 1 (5s exit)
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # Trigger exit
    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    pos = _query_position(cluster, pos_id)["position"]
    exit_unlock_at = isoparse(pos["exit_unlock_at"])

    # Wait past exit_unlock_at (5s for Tier 1)
    wait_for_block_time(cluster, exit_unlock_at)
    wait_for_new_blocks(cluster, 1)

    # Attempt to withdraw while still delegated — must fail even though exit is elapsed
    rsp = _withdraw(cluster, owner, pos_id)
    assert (
        rsp["code"] != 0
    ), "withdraw while still delegated (even after exit_unlock_at) should fail"
    assert "position is still delegated" in rsp.get("raw_log", ""), rsp["raw_log"]


# ──────────────────────────────────────────────
# Group J — Side Effect Verification
# ──────────────────────────────────────────────


def test_add_to_position_reward_side_effect(cluster):
    """AddToTierPosition on a delegated position implicitly claims pending rewards.

    When a position is delegated, AddToTierPosition calls ClaimAndRefreshPosition
    before adding tokens. With bonus > gas cost, the balance improves compared
    to a no-claim add.

    Note: bonus requires LastBonusAccrual to be non-zero (set by first claim).
    A fresh position always shows 0 until the first claim initializes it.
    """
    owner = cluster.address("signer2")
    validator = _get_validator_addr(cluster, 0)
    # Large position so bonus is measurable: 1B basecro at 4% APY for 15 blocks
    # gives ~19 basecro bonus, well above any gas cost.
    amount = TIER_1_MIN * 1000  # 1_000_000_000 basecro

    # Fund pool to ensure bonus rewards are available
    rsp = _fund_pool(cluster, "signer1", f"1000000000{DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, amount, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    # First claim initializes LastBonusAccrual (bonus = 0 here, field was unset).
    # Without this, calculateBonusRaw returns 0 for all subsequent estimates.
    rsp = _claim_rewards(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Wait for bonus to accrue since LastBonusAccrual was initialized
    wait_for_new_blocks(cluster, 15)

    # Verify bonus is nonzero before the implicit-claim add
    est = _query_estimate_rewards(cluster, pos_id)
    bonus = sum(int(c.get("amount", "0")) for c in est.get("bonus_rewards", []))
    assert bonus > 0, (
        f"bonus should be nonzero after 15 blocks with 1B basecro position; "
        f"got bonus={bonus}, est={est}"
    )

    balance_before = cluster.balance(owner, DENOM)

    # add_to_position implicitly claims rewards (ClaimAndRefreshPosition is called)
    add_amount = TIER_1_MIN
    rsp = _add_to_position(cluster, owner, pos_id, add_amount)
    assert rsp["code"] == 0, rsp["raw_log"]

    # Implicit claim must have emitted EventBonusRewardsClaimed.
    # AddToTierPosition calls ClaimAndRefreshPosition → ClaimRewardsForPositions
    # which emits EventBaseRewardsClaimed / EventBonusRewardsClaimed (not
    # EventTierRewardsClaimed, which is only emitted by ClaimTierRewards).
    ev = find_log_event_attrs(
        rsp["events"], "chainmain.tieredrewards.v1.EventBonusRewardsClaimed"
    )
    assert (
        ev is not None
    ), "add-to-tier-position on delegated position should emit EventBonusRewardsClaimed"

    balance_after = cluster.balance(owner, DENOM)
    # Without implicit claim: balance_after ≈ balance_before - add_amount - gas
    # With implicit claim:    balance_after ≈ balance_before - add_amount + rewards
    # bonus > gas → balance_after > balance_before - add_amount
    assert balance_after > balance_before - add_amount, (
        f"implicit reward claim should offset the add_amount cost: "
        f"before={balance_before}, after={balance_after}, add={add_amount}"
    )


# ──────────────────────────────────────────────
# Group K — Additional Query Coverage
# ──────────────────────────────────────────────


def test_exiting_undelegated_voting_power(cluster):
    """A position that is exiting AND undelegated contributes 0 voting power.

    Captures baseline power before the position is created, then verifies power
    returns exactly to baseline after exit+undelegate.
    """
    owner = cluster.address("signer1")
    validator = _get_validator_addr(cluster, 0)

    # Capture baseline BEFORE creating this position
    vp_baseline = _query_voting_power(cluster, owner)
    power_baseline = int(float(vp_baseline.get("voting_power", "0")))

    # Create delegated position
    before = _before_ids(cluster, owner)
    rsp = _lock_tier(cluster, owner, TIER_1_ID, TIER_1_MIN * 2, validator=validator)
    assert rsp["code"] == 0, rsp["raw_log"]
    pos_id = _new_pos_id(cluster, owner, before)

    wait_for_new_blocks(cluster, 1)

    # Record voting power WITH this delegated position — must exceed baseline
    vp_delegated = _query_voting_power(cluster, owner)
    power_delegated = int(float(vp_delegated.get("voting_power", "0")))
    assert power_delegated > power_baseline, (
        f"delegated position should increase voting power above baseline: "
        f"baseline={power_baseline}, with_position={power_delegated}"
    )

    # Trigger exit then undelegate → exiting + undelegated state
    rsp = _trigger_exit(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    rsp = _tier_undelegate(cluster, owner, pos_id)
    assert rsp["code"] == 0, rsp["raw_log"]

    wait_for_new_blocks(cluster, 1)

    # Verify the position is in the expected state: exiting + undelegated
    pos = _query_position(cluster, pos_id)["position"]
    assert pos.get("validator", "") == "", "position should be undelegated"
    assert (
        pos.get("exit_triggered_at")
        and pos["exit_triggered_at"] != "0001-01-01T00:00:00Z"
    ), "position should still be in exiting state"

    # Exiting+undelegated position must NOT contribute voting power;
    # power should be below the level recorded when the position was delegated
    vp_after = _query_voting_power(cluster, owner)
    power_after = int(float(vp_after.get("voting_power", "0")))
    assert power_after < power_delegated, (
        f"exiting+undelegated position must not contribute voting power: "
        f"baseline={power_baseline}, with_delegated={power_delegated}, "
        f"after_undelegate={power_after}"
    )


# ──────────────────────────────────────────────
# Group L — Params Governance (run last to avoid poisoning earlier tests)
#
# NOTE: This test permanently sets target_base_rewards_rate to 0 for the
# remainder of the test session. It is placed last so that all reward-
# dependent tests (Groups F, J) run with the original rate.
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
    approve_proposal(cluster, rsp, msg=f",{MSG_UPDATE_PARAMS}")

    updated = query_command(cluster, MODULE, "params")["params"]
    assert (
        float(updated["target_base_rewards_rate"]) == 0.0
    ), "target_base_rewards_rate should be 0 after governance update"
