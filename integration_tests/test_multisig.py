import json

import pytest
from pystarport import ports
from pystarport.proto_python.api_util import ApiUtil

from .utils import wait_for_new_blocks

pytestmark = pytest.mark.normal


def test_multi_signature(cluster, tmp_path):
    # prepare files
    m_txt = tmp_path / "m.txt"
    p1_txt = tmp_path / "p1.txt"
    p2_txt = tmp_path / "p2.txt"
    tx_txt = tmp_path / "tx.txt"

    # make multi-sig
    cluster.make_multisig("multitest1", "signer1", "signer2")
    multi_addr = cluster.address("multitest1")
    signer1_addr = cluster.address("signer1")
    signer2_addr = cluster.address("signer2")
    signer2_balance = cluster.balance(signer2_addr)
    # send amount to multi-sig
    cluster.transfer(signer1_addr, multi_addr, "205basecro")
    wait_for_new_blocks(cluster, 1)
    multi_balance1 = cluster.balance(multi_addr)
    assert 205 == multi_balance1
    # send amount from multisig to signer2
    multi_tx = cluster.transfer(
        multi_addr, signer2_addr, "80basecro", generate_only=True
    )
    json.dump(multi_tx, m_txt.open("w"))
    signature1 = cluster.sign_multisig_tx(m_txt, multi_addr, "signer1")
    json.dump(signature1, p1_txt.open("w"))
    signature2 = cluster.sign_multisig_tx(m_txt, multi_addr, "signer2")
    json.dump(signature2, p2_txt.open("w"))
    final_multi_tx = cluster.combine_multisig_tx(m_txt, "multitest1", p1_txt, p2_txt)
    json.dump(final_multi_tx, tx_txt.open("w"))
    cluster.broadcast_tx(tx_txt)
    # check multisig balance
    multi_balance = cluster.balance(multi_addr)
    assert 125 == multi_balance
    # check singer2 balance
    assert cluster.balance(signer2_addr) == signer2_balance + 80


def test_multi_signature_batch(cluster, tmp_path):
    # prepare files
    m_txt = tmp_path / "bm.txt"
    p1_txt = tmp_path / "bp1.txt"
    p2_txt = tmp_path / "bp2.txt"
    tx_txt = tmp_path / "btx.txt"
    multi_wallet_name = "multitest2"

    # make multi-sig
    cluster.make_multisig(multi_wallet_name, "msigner1", "msigner2")
    multi_addr = cluster.address(multi_wallet_name)
    signer1_addr = cluster.address("msigner1")
    signer2_addr = cluster.address("msigner2")
    # send amount to multi-sig
    cluster.transfer(signer1_addr, multi_addr, "500basecro")
    wait_for_new_blocks(cluster, 1)
    multi_balance = cluster.balance(multi_addr)
    assert 500 == multi_balance
    signer1_balance = cluster.balance(signer1_addr)
    signer2_balance = cluster.balance(signer2_addr)
    # send amount from multisig to msigner1 and msigner2
    with open(m_txt, "a") as f:
        multi_tx = cluster.transfer(
            multi_addr, signer1_addr, "100basecro", generate_only=True
        )
        f.write(json.dumps(multi_tx))
        f.write("\n")
        multi_tx = cluster.transfer(
            multi_addr, signer2_addr, "100basecro", generate_only=True
        )
        f.write(json.dumps(multi_tx))

    # multisign the tx
    port = ports.api_port(cluster.base_port(0))
    api = ApiUtil(port)
    multi_account_info = api.account_info(multi_addr)
    account_num = multi_account_info["account_num"]
    sequence = multi_account_info["sequence"]
    signature2 = cluster.sign_batch_multisig_tx(
        m_txt, multi_addr, signer1_addr, account_num, sequence
    )
    with open(p1_txt, "w") as f:
        f.write(signature2)
    signature3 = cluster.sign_batch_multisig_tx(
        m_txt, multi_addr, signer2_addr, account_num, sequence
    )
    with open(p2_txt, "w") as f:
        f.write(signature3)

    final_multi_tx = cluster.combine_batch_multisig_tx(
        m_txt, multi_wallet_name, p1_txt, p2_txt
    )
    for line in final_multi_tx.splitlines():
        with open(tx_txt, "w") as f:
            f.write(line)
        assert len(line) > 0
        cluster.broadcast_tx(tx_txt)
    # check multisig balance
    multi_balance = cluster.balance(multi_addr)
    wait_for_new_blocks(cluster, 4)
    assert 300 == multi_balance
    #     check singer2 balance
    assert cluster.balance(signer1_addr) == signer1_balance + 100
    assert cluster.balance(signer2_addr) == signer2_balance + 100
