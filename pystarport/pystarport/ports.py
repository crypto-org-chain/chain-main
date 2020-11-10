def p2p_port(base_port):
    return base_port


def rpc_port(base_port):
    return base_port + 7


def grpc_port(base_port):
    return base_port + 3


def api_port(base_port):
    return base_port + 4


def pprof_port(base_port):
    return base_port + 5


def grpc_port_tx_only(base_port):
    return base_port + 6
