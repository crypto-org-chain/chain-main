from cryptopy import Transaction, Wallet
from pystarport import ports
from pystarport.proto_python.api_util import ApiUtil

from .utils import wait_for_new_blocks


def test_sign_offline(cluster):
    """
    check simple transfer tx success
    - send 1cro from community to reserve
    """
    # 1. first create two hd new wallet
    seed = "dune car envelope chuckle elbow slight proud fury remove candy uphold \
    puzzle call select sibling sport gadget please want vault glance verb damage gown"
    wallet_1 = Wallet(seed)
    address_1 = wallet_1.address
    wallet_2 = Wallet.new()
    address_2 = wallet_2.address

    sender_addr = cluster.address("signer1")

    sender_balance = cluster.balance(sender_addr)
    assert sender_balance > 100 * 10 ** 8
    balance_1 = cluster.balance(wallet_1.address)
    assert balance_1 == 0
    balance_2 = cluster.balance(wallet_2.address)
    assert balance_2 == 0

    # 2. transfer some coin to wallet_1
    cluster.transfer(sender_addr, address_1, "100cro")
    wait_for_new_blocks(cluster, 2)

    assert cluster.balance(sender_addr) == sender_balance - 100 * 10 ** 8
    assert cluster.balance(address_1) == 100 * 10 ** 8

    # 3. get the send's account info
    port = ports.api_port(cluster.base_port(0))
    api = ApiUtil(port)

    amount = 1 * 10 ** 8
    # make transaction without/with fee
    for fee in [0, 600000]:
        sender_account_info = api.account_info(address_1)
        balance_1_before = api.balance(address_1)
        balance_2_before = api.balance(address_2)
        tx = Transaction(
            wallet=wallet_1,
            account_num=sender_account_info["account_num"],
            sequence=sender_account_info["sequence"],
            chain_id=cluster.chain_id,
            fee=fee,
        )
        tx.add_transfer(to_address=address_2, amount=amount, base_denom="basecro")
        signed_tx = tx.get_pushable()
        assert isinstance(signed_tx, dict)
        api.broadcast_tx(signed_tx)
        wait_for_new_blocks(cluster, 3)
        balance_1_after = api.balance(address_1)
        balance_2_after = api.balance(address_2)
        assert balance_2_after == balance_2_before + amount
        assert balance_1_after == balance_1_before - amount - fee
