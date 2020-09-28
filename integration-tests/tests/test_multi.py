#!/usr/bin/env python
import json
import os
import tempfile

import pytest

from .utils import wait_for_block

# pytest magic
pytestmark = pytest.mark.asyncio


async def test_multi_signature(cluster):
    with tempfile.TemporaryDirectory() as tmpdir:
        await do_test_multi_signature(cluster, tmpdir)


async def do_test_multi_signature(cluster, tmpdir):
    wait_seconds = 60
    # prepare files
    m_txt = os.path.join(tmpdir, "m.txt")
    p1_txt = os.path.join(tmpdir, "p1.txt")
    p2_txt = os.path.join(tmpdir, "p2.txt")
    tx_txt = os.path.join(tmpdir, "tx.txt")
    await wait_for_block(1, wait_seconds)
    info = await cluster.cli("query", "staking", "validators", output="json")
    print("info=", info)
    validators = json.loads(
        await cluster.cli("query", "staking", "validators", output="json")
    )
    assert len(validators) == 2
    # make multi-sig
    await cluster.cli.make_multisig("multitest1", "signer1", "signer2")
    multi_addr = (await cluster.cli.get_account("multitest1"))["address"]
    signer1_addr = (await cluster.cli.get_account("signer1"))["address"]
    signer2_addr = (await cluster.cli.get_account("signer2"))["address"]
    signer2_balance = await cluster.cli.get_balance(signer2_addr)
    # send amount to multi-sig
    await cluster.cli.send_amount(signer1_addr, multi_addr, "205basecro")
    await wait_for_block(3, wait_seconds)
    multi_balance1 = await cluster.cli.get_balance(multi_addr)
    assert 205 == multi_balance1
    # send amount from multisig to signer2
    multi_tx = await cluster.cli.send_amount_generation_only(
        multi_addr, signer2_addr, "80basecro"
    )
    open(m_txt, "wb").write(multi_tx)
    signature1 = await cluster.cli.sign_multisig_tx(m_txt, multi_addr, "signer1")
    open(p1_txt, "wb").write(signature1)
    signature2 = await cluster.cli.sign_multisig_tx(m_txt, multi_addr, "signer2")
    open(p2_txt, "wb").write(signature2)
    final_multi_tx = await cluster.cli.combine_multisig_tx(
        m_txt, "multitest1", p1_txt, p2_txt
    )
    open(tx_txt, "wb").write(final_multi_tx)
    await cluster.cli.broadcast_tx(tx_txt)
    # check multisig balance
    multi_balance = await cluster.cli.get_balance(multi_addr)
    assert 125 == multi_balance
    # check singer2 balance
    signer2_balance = await cluster.cli.get_balance(signer2_addr)
    assert 200000000080 == signer2_balance
