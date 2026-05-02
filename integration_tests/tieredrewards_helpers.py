import hashlib
import json
import time

import bech32
import requests
from pystarport.ports import api_port

from .utils import approve_proposal, module_address

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
TIER_2_ID = 2  # exit_duration=60s, bonus_apy=2%,  min_lock=5_000_000 basecro
TIER_1_MIN = 1_000_000
TIER_2_MIN = 5_000_000

# Governance tier added/deleted in Group G tests
TIER_3_ID = 3

# Gas slack for the single withdraw transaction in test_full_exit_flow.
# balance_before is captured after lock/trigger/undelegate, so only the
# withdraw gas needs to be covered here (~500_000 gas units).
GAS_ALLOWANCE = 5_000_000  # basecro (conservative upper bound)


# ──────────────────────────────────────────────
# Helper functions
# ──────────────────────────────────────────────


def rest_get(cluster, path, i=0):
    """GET from the REST API of validator i and return parsed JSON."""
    port = api_port(cluster.base_port(i))
    resp = requests.get(f"http://127.0.0.1:{port}{path}", timeout=10)
    resp.raise_for_status()
    return resp.json()


def tx(cluster, *subcmd, from_, i=0, **extra):
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


def lock_tier(cluster, owner, tier_id, amount, validator, trigger_exit=False, i=0):
    args = ["lock-tier", str(tier_id), str(amount), validator]
    if trigger_exit:
        args.append("--trigger-exit-immediately")
    return tx(cluster, *args, from_=owner, i=i)


def tier_undelegate(cluster, owner, position_id, i=0):
    return tx(cluster, "tier-undelegate", str(position_id), from_=owner, i=i)


def exit_tier_with_delegation(cluster, owner, position_id, amount, i=0):
    return tx(
        cluster,
        "exit-tier-with-delegation",
        str(position_id),
        str(amount),
        from_=owner,
        i=i,
    )


def tier_delegate(cluster, owner, position_id, validator, i=0):
    return tx(cluster, "tier-delegate", str(position_id), validator, from_=owner, i=i)


def tier_redelegate(cluster, owner, position_id, dst_validator, i=0):
    return tx(
        cluster, "tier-redelegate", str(position_id), dst_validator, from_=owner, i=i
    )


def trigger_exit(cluster, owner, position_id, i=0):
    return tx(cluster, "trigger-exit", str(position_id), from_=owner, i=i)


def claim_rewards(cluster, owner, *position_ids, i=0):
    args = ["claim-tier-rewards"] + [str(pid) for pid in position_ids]
    return tx(cluster, *args, from_=owner, i=i)


def add_to_position(cluster, owner, position_id, amount, i=0):
    return tx(
        cluster, "add-to-tier-position", str(position_id), str(amount), from_=owner, i=i
    )


def clear_position(cluster, owner, position_id, i=0):
    return tx(cluster, "clear-position", str(position_id), from_=owner, i=i)


def withdraw(cluster, owner, position_id, i=0):
    return tx(cluster, "withdraw-from-tier", str(position_id), from_=owner, i=i)


def fund_pool(cluster, from_name, amount_coin):
    """Fund the rewards pool via a bank send to the module account."""
    from_addr = cluster.address(from_name)
    pool_addr = module_address(REWARDS_POOL_NAME)
    return cluster.transfer(from_addr, pool_addr, amount_coin)


def commit_delegation(
    cluster, delegator, validator, amount, tier_id, trigger_exit=False, i=0
):
    args = ["commit-delegation-to-tier", validator, str(amount), str(tier_id)]
    if trigger_exit:
        args.append("--trigger-exit-immediately")
    return tx(cluster, *args, from_=delegator, i=i)


def query_position(cluster, position_id, i=0):
    return rest_get(cluster, f"/chainmain/tieredrewards/v1/position/{position_id}", i)


def position_delegator_address(pos_id, prefix="cro"):
    data = hashlib.sha256(f"tieredrewards/position/{pos_id}".encode()).digest()[:20]
    return bech32.bech32_encode(prefix, bech32.convertbits(data, 8, 5))


def query_positions_by_owner(cluster, owner, i=0):
    try:
        return rest_get(cluster, f"/chainmain/tieredrewards/v1/positions/{owner}", i)
    except requests.HTTPError as exc:
        if exc.response.status_code == 404:
            return {"positions": []}
        raise


def query_tiers(cluster, i=0):
    return rest_get(cluster, "/chainmain/tieredrewards/v1/tiers", i)


def query_estimate_rewards(cluster, position_id, i=0):
    return rest_get(
        cluster, f"/chainmain/tieredrewards/v1/estimate_rewards/{position_id}", i
    )


def pool_balance(cluster):
    pool_addr = module_address(REWARDS_POOL_NAME)
    return cluster.balance(pool_addr, DENOM)


def approve_tieredrewards_proposal(cluster, rsp, msg, expect_status=None):
    return approve_proposal(
        cluster,
        rsp,
        msg=msg,
        expect_status=expect_status,
    )


def get_validator_addr(cluster, i=0):
    """Return the operator address of validator i."""
    return cluster.validators()[i]["operator_address"]


def get_node_validator_addr(cluster, i=0):
    """Return the operator address for a specific node index."""
    return cluster.address("validator", i=i, bech="val")


def before_ids(cluster, owner, i=0):
    """Capture current position IDs for an owner (for before/after diff)."""
    return {
        int(p["id"])
        for p in query_positions_by_owner(cluster, owner, i).get("positions", [])
    }


def new_pos_id(cluster, owner, before, i=0):
    """Find the single new position ID created since before was captured.

    Fails if there is not exactly one new position for owner.
    """
    result = query_positions_by_owner(cluster, owner, i)
    after = {int(p["id"]) for p in result.get("positions", [])}
    new = after - before
    assert len(new) == 1, (
        f"Expected exactly 1 new position for {owner}, got {new} "
        f"(before={before}, after={after})"
    )
    return next(iter(new))
