import json

import pytest

from .utils import wait_for_new_blocks

pytestmark = pytest.mark.grpc


def test_query_validators(cluster):
    from pystarport.proto_python.grpc_util import GrpcUtil

    wait_for_new_blocks(cluster, 5)
    grpc_ip_port = cluster.ipport_grpc(0)
    grpc = GrpcUtil(grpc_ip_port)
    grpc_response = grpc.get_validators()
    grpc_validators = grpc_response.validators
    validators = cluster.validators()
    operators = {}
    for cli_validator in validators:
        operator_address = cli_validator["operator_address"]
        operators[operator_address] = cli_validator

    for grpc_validator in grpc_validators:
        assert grpc_validator.operator_address in operators


def test_tx_broadcast(cluster, tmp_path):
    from pystarport.proto_python.grpc_util import GrpcUtilTxBroadcast

    tx_txt = tmp_path / "tx.txt"
    sign_txt = tmp_path / "sign.txt"
    encode_txt = tmp_path / "encode.txt"
    signer1_addr = cluster.address("signer1")
    signer2_addr = cluster.address("signer2")
    signer2_balance = cluster.balance(signer2_addr)
    tx_amount = 5
    single_tx = cluster.transfer(
        signer1_addr, signer2_addr, f"{tx_amount}basecro", generate_only=True
    )
    json.dump(single_tx, tx_txt.open("w"))
    signed_tx = cluster.sign_single_tx(tx_txt, signer1_addr)
    json.dump(signed_tx, sign_txt.open("w"))
    encoded_tx = cluster.encode_signed_tx(sign_txt)
    encode_file = open(encode_txt, "wb")
    encode_file.write(encoded_tx)
    encode_file.close()
    grpc_ip_port = cluster.ipport_grpc_tx(0)
    grpc = GrpcUtilTxBroadcast(grpc_ip_port)
    grpc.send_tx_in_base64(encoded_tx)
    wait_for_new_blocks(cluster, 2)
    new_signer2_balance = cluster.balance(signer2_addr)
    assert new_signer2_balance == signer2_balance + tx_amount
