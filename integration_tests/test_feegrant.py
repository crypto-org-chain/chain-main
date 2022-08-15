import datetime

import pytest
from dateutil.parser import isoparse

from .utils import (
    BASECRO_DENOM,
    SUCCESS_CODE,
    grant_fee_allowance,
    query_block_info,
    revoke_fee_grant,
    transfer,
    wait_for_block,
    wait_for_block_time,
)

pytestmark = pytest.mark.normal


def test_basic_fee_allowance(cluster):
    """
    check basic fee allowance with no limit or grant expiry
    """
    transaction_coins = 100
    fee_coins = 10

    fee_granter_address = cluster.address("community")
    fee_grantee_address = cluster.address("ecosystem")
    receiver_address = cluster.address("reserve")

    fee_granter_balance = cluster.balance(fee_granter_address)
    fee_grantee_balance = cluster.balance(fee_grantee_address)
    receiver_balance = cluster.balance(receiver_address)

    grant_fee_allowance(cluster, fee_granter_address, fee_grantee_address)

    transfer(
        cluster,
        fee_grantee_address,
        receiver_address,
        "%s%s" % (transaction_coins, BASECRO_DENOM),
        fees="%s%s" % (fee_coins, BASECRO_DENOM),
        fee_granter=fee_granter_address,
    )

    assert cluster.balance(fee_granter_address) == fee_granter_balance - fee_coins
    assert (
        cluster.balance(fee_grantee_address) == fee_grantee_balance - transaction_coins
    )
    assert cluster.balance(receiver_address) == receiver_balance + transaction_coins


def test_tx_failed_when_exceeds_grant_fee(cluster):
    """
    check transaction should fail when tx fee exceeds fee limit in basic fee allowance
    """
    transaction_coins = 100
    fee_coins = 10
    fee_grant_spend_limit = 5

    fee_granter_address = cluster.address("community")
    fee_grantee_address = cluster.address("ecosystem")
    receiver_address = cluster.address("reserve")

    fee_granter_balance = cluster.balance(fee_granter_address)
    fee_grantee_balance = cluster.balance(fee_grantee_address)
    receiver_balance = cluster.balance(receiver_address)

    revoke_fee_grant(cluster, fee_granter_address, fee_grantee_address)
    grant_fee_allowance(
        cluster,
        fee_granter_address,
        fee_grantee_address,
        spend_limit="%s%s" % (fee_grant_spend_limit, BASECRO_DENOM),
    )

    tx = transfer(
        cluster,
        fee_grantee_address,
        receiver_address,
        "%s%s" % (transaction_coins, BASECRO_DENOM),
        fees="%s%s" % (fee_coins, BASECRO_DENOM),
        fee_granter=fee_granter_address,
    )
    assert tx["code"] != SUCCESS_CODE, "should fail as fee limit exceeded"

    assert cluster.balance(fee_granter_address) == fee_granter_balance
    assert cluster.balance(fee_grantee_address) == fee_grantee_balance
    assert cluster.balance(receiver_address) == receiver_balance


def test_tx_failed_after_grant_expiration(cluster):
    """
    check transaction should fail when tx happens after grant expiry
    """
    transaction_coins = 100
    fee_coins = 10

    # RFC 3339 timestamp
    grant_expiration = datetime.datetime.utcnow().isoformat() + "Z"

    fee_granter_address = cluster.address("community")
    fee_grantee_address = cluster.address("ecosystem")
    receiver_address = cluster.address("reserve")

    fee_granter_balance = cluster.balance(fee_granter_address)
    fee_grantee_balance = cluster.balance(fee_grantee_address)
    receiver_balance = cluster.balance(receiver_address)

    revoke_fee_grant(cluster, fee_granter_address, fee_grantee_address)
    grant_fee_allowance(
        cluster, fee_granter_address, fee_grantee_address, expiration=grant_expiration
    )

    tx = transfer(
        cluster,
        fee_grantee_address,
        receiver_address,
        "%s%s" % (transaction_coins, BASECRO_DENOM),
        fees="%s%s" % (fee_coins, BASECRO_DENOM),
        fee_granter=fee_granter_address,
    )
    assert tx["code"] != SUCCESS_CODE, "should fail as fee allowance expired"

    assert cluster.balance(fee_granter_address) == fee_granter_balance
    assert cluster.balance(fee_grantee_address) == fee_grantee_balance
    assert cluster.balance(receiver_address) == receiver_balance


def test_periodic_fee_allowance(cluster):
    """
    check periodic fee allowance with no expiration
    """
    transaction_coins = 100
    fee_coins = 10

    period = 5
    period_limit = 11
    number_of_periods = 3

    fee_granter_address = cluster.address("community")
    fee_grantee_address = cluster.address("ecosystem")
    receiver_address = cluster.address("reserve")

    fee_granter_balance = cluster.balance(fee_granter_address)
    fee_grantee_balance = cluster.balance(fee_grantee_address)
    receiver_balance = cluster.balance(receiver_address)

    revoke_fee_grant(cluster, fee_granter_address, fee_grantee_address)
    grant_fee_allowance(
        cluster,
        fee_granter_address,
        fee_grantee_address,
        period_limit="%s%s" % (period_limit, BASECRO_DENOM),
        period=period,
    )

    for _ in range(number_of_periods):
        tx = transfer(
            cluster,
            fee_grantee_address,
            receiver_address,
            "%s%s" % (transaction_coins, BASECRO_DENOM),
            fees="%s%s" % (fee_coins, BASECRO_DENOM),
            fee_granter=fee_granter_address,
        )
        wait_for_block(cluster, int(tx["height"]) + 1)  # wait for next block
        block_info = query_block_info(cluster, tx["height"])
        wait_for_block_time(
            cluster,
            isoparse(block_info["block"]["header"]["time"])
            + datetime.timedelta(seconds=period),
        )

    assert (
        cluster.balance(fee_granter_address)
        == fee_granter_balance - fee_coins * number_of_periods
    )
    assert (
        cluster.balance(fee_grantee_address)
        == fee_grantee_balance - transaction_coins * number_of_periods
    )
    assert (
        cluster.balance(receiver_address)
        == receiver_balance + transaction_coins * number_of_periods
    )


def test_exceed_period_limit_should_not_affect_the_next_period(cluster):
    """
    check exceeding periodic fee should not affect next period
    """
    transaction_coins = 100
    fee_coins = 10

    period = 5
    period_limit = 11

    fee_granter_address = cluster.address("community")
    fee_grantee_address = cluster.address("ecosystem")
    receiver_address = cluster.address("reserve")

    fee_granter_balance = cluster.balance(fee_granter_address)
    fee_grantee_balance = cluster.balance(fee_grantee_address)
    receiver_balance = cluster.balance(receiver_address)

    revoke_fee_grant(cluster, fee_granter_address, fee_grantee_address)
    grant_fee_allowance(
        cluster,
        fee_granter_address,
        fee_grantee_address,
        period_limit="%s%s" % (period_limit, BASECRO_DENOM),
        period=period,
    )

    tx = transfer(
        cluster,
        fee_grantee_address,
        receiver_address,
        "%s%s" % (transaction_coins, BASECRO_DENOM),
        fees="%s%s" % (fee_coins, BASECRO_DENOM),
        fee_granter=fee_granter_address,
    )

    failed_tx = transfer(
        cluster,
        fee_grantee_address,
        receiver_address,
        "%s%s" % (transaction_coins, BASECRO_DENOM),
        fees="%s%s" % (fee_coins, BASECRO_DENOM),
        fee_granter=fee_granter_address,
    )
    assert failed_tx["code"] != SUCCESS_CODE, "should fail as fee exceeds period limit"

    wait_for_block(cluster, int(tx["height"]))
    block_info = query_block_info(cluster, tx["height"])
    wait_for_block_time(
        cluster,
        isoparse(block_info["block"]["header"]["time"])
        + datetime.timedelta(seconds=period),
    )

    transfer(
        cluster,
        fee_grantee_address,
        receiver_address,
        "%s%s" % (transaction_coins, BASECRO_DENOM),
        fees="%s%s" % (fee_coins, BASECRO_DENOM),
        fee_granter=fee_granter_address,
    )

    # transaction only happened two times
    assert cluster.balance(fee_granter_address) == fee_granter_balance - fee_coins * 2
    assert (
        cluster.balance(fee_grantee_address)
        == fee_grantee_balance - transaction_coins * 2
    )
    assert cluster.balance(receiver_address) == receiver_balance + transaction_coins * 2


def test_revoke_fee_grant(cluster):
    """
    check tx should fail after fee grant is revoked
    """
    transaction_coins = 100
    fee_coins = 10

    fee_granter_address = cluster.address("community")
    fee_grantee_address = cluster.address("ecosystem")
    receiver_address = cluster.address("reserve")

    revoke_fee_grant(cluster, fee_granter_address, fee_grantee_address)
    grant_fee_allowance(cluster, fee_granter_address, fee_grantee_address)

    revoke_fee_grant(cluster, fee_granter_address, fee_grantee_address)

    failed_tx = transfer(
        cluster,
        fee_grantee_address,
        receiver_address,
        "%s%s" % (transaction_coins, BASECRO_DENOM),
        fees="%s%s" % (fee_coins, BASECRO_DENOM),
        fee_granter=fee_granter_address,
    )

    assert failed_tx["code"] != SUCCESS_CODE, "should fail as grant is revoked"
