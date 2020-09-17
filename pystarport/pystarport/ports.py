def p2p_port(base_port, i):
    return base_port + i * 10


def rpc_port(base_port, i):
    return base_port + i * 10 + 7


def abci_port(base_port, i):
    return base_port + i * 10 + 2


def grpc_port(base_port, i):
    return base_port + i * 10 + 3


def api_port(base_port, i):
    return base_port + i * 10 + 4


def pprof_port(base_port, i):
    return base_port + i * 10 + 5
