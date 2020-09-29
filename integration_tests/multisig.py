#!/usr/bin/env python
import json
import tempfile
from pathlib import Path

from .utils import wait_for_new_blocks


async def test_multi_signature(cluster):
    with tempfile.TemporaryDirectory() as tmpdir:
        await do_test_multi_signature(cluster, Path(tmpdir))


async def do_test_multi_signature(cluster, tmpdir):
    # prepare files
    m_txt = tmpdir / "m.txt"
    p1_txt = tmpdir / "p1.txt"
    p2_txt = tmpdir / "p2.txt"
    tx_txt = tmpdir / "tx.txt"

    # make multi-sig
    await cluster.cli.make_multisig("multitest1", "signer1", "signer2")
    multi_addr = await cluster.cli.address("multitest1")
    signer1_addr = await cluster.cli.address("signer1")
    signer2_addr = await cluster.cli.address("signer2")
    signer2_balance = await cluster.cli.balance(signer2_addr)
    # send amount to multi-sig
    await cluster.cli.transfer(signer1_addr, multi_addr, "205basecro")
    await wait_for_new_blocks(cluster.cli, 1)
    multi_balance1 = await cluster.cli.balance(multi_addr)
    assert 205 == multi_balance1
    # send amount from multisig to signer2
    multi_tx = await cluster.cli.transfer(
        multi_addr, signer2_addr, "80basecro", generate_only=True
    )
    json.dump(multi_tx, m_txt.open("w"))
    signature1 = await cluster.cli.sign_multisig_tx(m_txt, multi_addr, "signer1")
    json.dump(signature1, p1_txt.open("w"))
    signature2 = await cluster.cli.sign_multisig_tx(m_txt, multi_addr, "signer2")
    json.dump(signature2, p2_txt.open("w"))
    final_multi_tx = await cluster.cli.combine_multisig_tx(
        m_txt, "multitest1", p1_txt, p2_txt
    )
    json.dump(final_multi_tx, tx_txt.open("w"))
    await cluster.cli.broadcast_tx(tx_txt)
    # check multisig balance
    multi_balance = await cluster.cli.balance(multi_addr)
    assert 125 == multi_balance
    # check singer2 balance
    signer2_balance = await cluster.cli.balance(signer2_addr)
    assert 200000000080 == signer2_balance
