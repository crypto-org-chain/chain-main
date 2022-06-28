import pytest
from pystarport import ports

from .utils import wait_for_new_blocks

pytestmark = pytest.mark.normal


def get_network_config(grpc_port, chain_id):
    from chainlibpy.grpc_client import NetworkConfig

    return NetworkConfig(
        grpc_endpoint=f"127.0.0.1:{grpc_port}",
        chain_id=chain_id,
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
    from chainlibpy import CROCoin, GrpcClient, Transaction, Wallet
    from chainlibpy.generated.cosmos.bank.v1beta1.tx_pb2 import MsgSend

    grpc_port = ports.grpc_port(cluster.base_port(0))
    chain_id = cluster.chain_id

    network_config = get_network_config(grpc_port, chain_id)
    grpc_client = GrpcClient(network_config)

    # 1. first create offline wallet
    offline_wallet = Wallet.new()
    offline_addr = offline_wallet.address

    # 2. transfer some coin to offline_wallet so it can sign tx offline
    alice_addr = cluster.address("signer1")
    cluster.transfer(alice_addr, offline_addr, "100cro")
    wait_for_new_blocks(cluster, 2)
    assert cluster.balance(offline_addr) == 100 * 10**8

    ten_cro = CROCoin(10, network_config.coin_denom, network_config)
    # make transaction without/with fee
    receiver_addr = alice_addr
    for fee in [0, 600000]:
        sender_account = grpc_client.query_account(offline_addr)
        sender_balance_bef = grpc_client.query_account_balance(offline_addr)
        receiver_balance_bef = grpc_client.query_account_balance(receiver_addr)

        sender_cro_bef = CROCoin(
            sender_balance_bef.balance.amount,
            sender_balance_bef.balance.denom,
            network_config,
        )

        receiver_cro_bef = CROCoin(
            receiver_balance_bef.balance.amount,
            receiver_balance_bef.balance.denom,
            network_config,
        )

        msg_send_10_cro = MsgSend(
            from_address=offline_addr,
            to_address=receiver_addr,
            amount=[ten_cro.protobuf_coin_message],
        )

        fee_cro = CROCoin(fee, network_config.coin_base_denom, network_config)
        tx = Transaction(
            chain_id=network_config.chain_id,
            from_wallets=[offline_wallet],
            msgs=[msg_send_10_cro],
            account_number=sender_account.account_number,
            fee=[fee_cro.protobuf_coin_message],
            client=grpc_client,
        )

        signature_offline = offline_wallet.sign(tx.sign_doc.SerializeToString())
        signed_tx = tx.set_signatures(signature_offline).signed_tx
        grpc_client.broadcast_transaction(signed_tx.SerializeToString())
        wait_for_new_blocks(cluster, 3)

        sender_balance_aft = grpc_client.query_account_balance(offline_addr)
        receiver_balance_aft = grpc_client.query_account_balance(receiver_addr)
        sender_cro_aft = CROCoin(
            sender_balance_aft.balance.amount,
            sender_balance_aft.balance.denom,
            network_config,
        )

        receiver_cro_aft = CROCoin(
            receiver_balance_aft.balance.amount,
            receiver_balance_aft.balance.denom,
            network_config,
        )

        assert sender_cro_aft == sender_cro_bef - ten_cro - fee_cro
        assert receiver_cro_aft == receiver_cro_bef + ten_cro
