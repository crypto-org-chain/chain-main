from pystarport.proto_python.grpc_util import GrpcUtil

from .utils import wait_for_new_blocks


def test_query_validators(cluster):
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
