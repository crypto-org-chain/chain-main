import json

from .utils import wait_for_new_blocks


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
