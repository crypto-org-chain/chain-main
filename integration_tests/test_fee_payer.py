import json

import pytest

from .utils import sign_single_tx_with_options

pytestmark = pytest.mark.normal


def test_different_fee_payer(cluster, tmp_path):
    transaction_coins = 100
    fee_coins = 1

    receiver_addr = cluster.address("community")
    sender_addr = cluster.address("signer1")
    fee_payer_addr = cluster.address("signer2")

    unsigned_tx_txt = tmp_path / "unsigned_tx.txt"
    partial_sign_txt = tmp_path / "partial_sign.txt"
    signed_txt = tmp_path / "signed.txt"

    receiver_balance = cluster.balance(receiver_addr)
    sender_balance = cluster.balance(sender_addr)
    fee_payer_balance = cluster.balance(fee_payer_addr)

    unsigned_tx_msg = cluster.transfer(
        sender_addr,
        receiver_addr,
        "%sbasecro" % transaction_coins,
        generate_only=True,
        fees="%sbasecro" % fee_coins,
    )

    unsigned_tx_msg["auth_info"]["fee"]["payer"] = fee_payer_addr
    with open(unsigned_tx_txt, "w") as opened_file:
        json.dump(unsigned_tx_msg, opened_file)
    partial_sign_tx_msg = sign_single_tx_with_options(
        cluster, unsigned_tx_txt, "signer1", sign_mode="amino-json"
    )
    with open(partial_sign_txt, "w") as opened_file:
        json.dump(partial_sign_tx_msg, opened_file)
    signed_tx_msg = sign_single_tx_with_options(
        cluster, partial_sign_txt, "signer2", sign_mode="amino-json"
    )
    with open(signed_txt, "w") as opened_file:
        json.dump(signed_tx_msg, opened_file)
    cluster.broadcast_tx(signed_txt)

    assert cluster.balance(receiver_addr) == receiver_balance + transaction_coins
    assert cluster.balance(sender_addr) == sender_balance - transaction_coins
    assert cluster.balance(fee_payer_addr) == fee_payer_balance - fee_coins
