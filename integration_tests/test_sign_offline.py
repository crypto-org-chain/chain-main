import pytest
from chainlibpy import CROCoin, GrpcClient, NetworkConfig, Transaction, Wallet
from chainlibpy.generated.cosmos.bank.v1beta1.tx_pb2 import MsgSend

from .utils import wait_for_new_blocks

pytestmark = pytest.mark.normal


LOCAL_TESTNET = NetworkConfig(
    # grpc_endpoint from data/chaintest/nodex/config/app.toml
    # Look for "gRPC Configuration" section
    grpc_endpoint="127.0.0.1:10003",
    # chain_id from from data/
    # the directory name under data is the chain_id
    chain_id="chaintest",
    address_prefix="cro",
    coin_denom="cro",
    coin_base_denom="basecro",
    exponent=8,
    derivation_path="m/44'/394'/0'/0/0",
)


def test_sign_offline(cluster):
    """
    check simple transfer tx success
    - send 1cro from community to reserve
    """

    client = GrpcClient(LOCAL_TESTNET)

    # 1. first create two hd new wallet
    seed = "dune car envelope chuckle elbow slight proud fury remove candy uphold \
    puzzle call select sibling sport gadget please want vault glance verb damage gown"
    sender_wallet = Wallet(seed)
    sender_addr = sender_wallet.address
    wallet_receiver = Wallet.new()
    receiver_addr = wallet_receiver.address

    faucet_address = cluster.address("signer1")

    faucet_balance = cluster.balance(faucet_address)
    assert faucet_balance > 100 * 10 ** 8
    sender_balance = cluster.balance(sender_addr)
    assert sender_balance == 0
    receiver_balance = cluster.balance(receiver_addr)
    assert receiver_balance == 0

    # 2. transfer some coin to sender_wallet so it can sign tx offline
    cluster.transfer(faucet_address, sender_addr, "100cro")
    wait_for_new_blocks(cluster, 2)

    assert cluster.balance(faucet_address) == faucet_balance - 100 * 10 ** 8
    assert cluster.balance(sender_addr) == 100 * 10 ** 8

    ten_cro = CROCoin(10, LOCAL_TESTNET.coin_denom, LOCAL_TESTNET)
    # make transaction without/with fee
    for fee in [0, 600000]:
        sender_account = client.query_account(sender_addr)
        sender_balance_bef = client.query_account_balance(sender_addr)
        receiver_balance_bef = client.query_account_balance(receiver_addr)

        sender_cro_bef = CROCoin(
            sender_balance_bef.balance.amount,
            sender_balance_bef.balance.denom,
            LOCAL_TESTNET,
        )

        receiver_cro_bef = CROCoin(
            receiver_balance_bef.balance.amount,
            receiver_balance_bef.balance.denom,
            LOCAL_TESTNET,
        )

        msg_send_10_cro = MsgSend(
            from_address=sender_addr,
            to_address=receiver_addr,
            amount=[ten_cro.protobuf_coin_message],
        )

        fee_cro = CROCoin(fee, LOCAL_TESTNET.coin_base_denom, LOCAL_TESTNET)
        tx = Transaction(
            chain_id=LOCAL_TESTNET.chain_id,
            from_wallets=[sender_wallet],
            msgs=[msg_send_10_cro],
            account_number=sender_account.account_number,
            fee=[fee_cro.protobuf_coin_message],
            client=client,
        )

        signature_alice = sender_wallet.sign(tx.sign_doc.SerializeToString())
        signed_tx = tx.set_signatures(signature_alice).signed_tx
        client.broadcast_transaction(signed_tx.SerializeToString())
        wait_for_new_blocks(cluster, 3)

        sender_balance_aft = client.query_account_balance(sender_addr)
        receiver_balance_aft = client.query_account_balance(receiver_addr)
        sender_cro_aft = CROCoin(
            sender_balance_aft.balance.amount,
            sender_balance_aft.balance.denom,
            LOCAL_TESTNET,
        )

        receiver_cro_aft = CROCoin(
            receiver_balance_aft.balance.amount,
            receiver_balance_aft.balance.denom,
            LOCAL_TESTNET,
        )

        assert sender_cro_aft == sender_cro_bef - ten_cro - fee_cro
        assert receiver_cro_aft == receiver_cro_bef + ten_cro
