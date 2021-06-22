import base64

import cosmos.bank.v1beta1.tx_pb2
import cosmos.bank.v1beta1.tx_pb2_grpc
import cosmos.crypto.ed25519.keys_pb2
import cosmos.staking.v1beta1.query_pb2
import cosmos.staking.v1beta1.query_pb2_grpc
import grpc
import tendermint.rpc.grpc.types_pb2_grpc


# for query only
class GrpcUtil:
    def __init__(self, ip_port):
        self.ip_port = ip_port

    def get_validators(self):
        channel = grpc.insecure_channel(self.ip_port)
        stub = cosmos.staking.v1beta1.query_pb2_grpc.QueryStub(channel)
        response = stub.Validators(
            cosmos.staking.v1beta1.query_pb2.QueryValidatorsRequest()
        )
        return response


# for tx broadcast only
class GrpcUtilTxBroadcast:
    def __init__(self, ip_port):
        self.ip_port = ip_port

    def send_tx_in_base64(self, tx_base64):
        tx_raw_bytes = base64.b64decode(tx_base64)
        channel = grpc.insecure_channel(self.ip_port)
        stub = tendermint.rpc.grpc.types_pb2_grpc.BroadcastAPIStub(channel)
        request = tendermint.rpc.grpc.types_pb2.RequestBroadcastTx()
        request.tx = tx_raw_bytes
        response = stub.BroadcastTx(request)
        return response
