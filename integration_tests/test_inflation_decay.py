import math
from pathlib import Path

import pytest

from .utils import cluster_fixture, get_sync_info, query_command, wait_for_block

pytestmark = pytest.mark.inflation

DENOM = "basecro"
CRO = 10**8  # 1 CRO = 10^8 basecro

# Genesis totals (must match inflation.jsonnet)
# validator: 10, supply: 98,700,000,000, burned: 380,000,000
INITIAL_TOTAL_CRO = 10 + 98_700_000_000 + 380_000_000

BASE_RATE = 0.01  # 1% annual inflation (InflationMin = InflationMax = 0.01)
DECAY_RATE = 0.068  # 6.8% monthly decay
MAX_SUPPLY_CRO = 100_000_000_000  # 100B CRO

TARGET_BLOCKS = 1000


@pytest.fixture(scope="module")
def cluster(worker_index, tmp_path_factory):
    yield from cluster_fixture(
        Path(__file__).parent / "configs/inflation.jsonnet",
        worker_index,
        tmp_path_factory.mktemp("data"),
    )


def _theoretical_max_supply(initial_supply, base_rate, decay_rate):
    """
    Closed-form max supply from continuous model:
    S(inf) = S0 * exp(-baseRate / (12 * ln(1 - decayRate)))
    """
    exponent = -base_rate / (12 * math.log(1 - decay_rate))
    return initial_supply * math.exp(exponent)


def test_inflation_decay(cluster):
    """
    Test that with decay-enabled inflation on a running chain:
    1. The burned address is correctly excluded from circulating supply
    2. Circulating supply stays under MaxSupply after many blocks
    3. Total supply is growing (inflation is active)

    Burned address is set in genesis (inflation.jsonnet).
    Uses default blocks_per_year (~6.3M) with 5ms timeout_commit so blocks
    churn quickly.
    """
    burned_addr = cluster.address("burned")

    # Verify burned address is already in inflation params from genesis
    params_rsp = query_command(cluster, "inflation", "params")
    assert (
        burned_addr in params_rsp["params"]["burned_addresses"]
    ), f"burned address {burned_addr} not found in inflation params"

    # Record initial supply
    rsp = query_command(cluster, "bank", "total-supply-of", DENOM)
    initial_supply = int(rsp["amount"]["amount"])

    # Record initial circulating supply
    burned_balance = cluster.balance(burned_addr)
    initial_circulating = initial_supply - burned_balance

    # Let the chain churn through many blocks
    wait_for_block(cluster, TARGET_BLOCKS, timeout=3600)

    # Query final state
    rsp = query_command(cluster, "bank", "total-supply-of", DENOM)
    total_supply = int(rsp["amount"]["amount"])

    burned_balance = cluster.balance(burned_addr)
    circulating = total_supply - burned_balance

    current_height = int(get_sync_info(cluster.status())["latest_block_height"])

    print(f"blocks processed: {current_height}")
    print(f"burned balance: {burned_balance} basecro")
    print(f"initial total supply: {initial_supply} basecro")
    print(f"after total supply: {total_supply} basecro")
    print(f"total supply growth: {total_supply - initial_supply} basecro")
    print(f"initial circulating: {initial_circulating} basecro")
    print(f"after circulating: {circulating} basecro")
    print(f"circulating growth: {circulating - initial_circulating} basecro")
    print(f"hard supply cap: {MAX_SUPPLY_CRO * CRO} basecro")

    # 1. Total supply must have grown (inflation is active)
    assert (
        total_supply > initial_supply
    ), f"Total supply should have grown: {total_supply} <= {initial_supply}"

    # 2. Circulating supply must not exceed MaxSupply
    max_supply_basecro = MAX_SUPPLY_CRO * CRO
    assert (
        circulating <= max_supply_basecro
    ), f"Circulating supply {circulating} exceeded max supply {max_supply_basecro}"

    # 3. Total supply should be below the theoretical max
    #    (we may not have reached convergence yet, but supply must not exceed it)
    theoretical_total = _theoretical_max_supply(
        INITIAL_TOTAL_CRO * CRO, BASE_RATE, DECAY_RATE
    )
    print(f"theoretical max: {theoretical_total:.0f} basecro")

    assert total_supply <= theoretical_total * 1.01, (
        f"Total supply {total_supply} exceeded theoretical max "
        f"{theoretical_total:.0f} by more than 1%"
    )


def test_inflation_rate_decreases_over_time(cluster):
    """
    Verify that the effective inflation rate strictly decreases over time.
    Samples the inflation rate every 5 blocks for 10 intervals and asserts
    each sample is lower than the previous one.
    """
    interval = 5
    samples = 10
    start_height = int(get_sync_info(cluster.status())["latest_block_height"])

    prev_inflation = float(query_command(cluster, "mint", "inflation")["inflation"])
    print(f"height {start_height}: inflation = {prev_inflation}")

    for i in range(1, samples + 1):
        wait_for_block(cluster, start_height + i * interval, timeout=120)
        cur_inflation = float(query_command(cluster, "mint", "inflation")["inflation"])
        cur_height = int(get_sync_info(cluster.status())["latest_block_height"])
        print(f"height {cur_height}: inflation = {cur_inflation}")

        assert cur_inflation < prev_inflation, (
            f"Block {cur_height}: inflation did not decrease: "
            f"{cur_inflation} >= {prev_inflation}"
        )
        prev_inflation = cur_inflation
