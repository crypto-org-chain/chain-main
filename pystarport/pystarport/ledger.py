import socket
import time
import uuid

import docker

ZEMU_HOST = "127.0.0.1"
ZEMU_BUTTON_PORT = 9997
ZEMU_GRPC_SERVER_PORT = 3002
# dockerfile is integration_test/hardware_wallet/Dockerfile
ZEMU_IMAGE = "cryptocom/builder-zemu:latest"


class Ledger:
    def __init__(self):
        self.ledger_name = f"ledger_simulator_{uuid.uuid4().time_mid}"
        self.proxy_name = f"ledger_proxy_{uuid.uuid4().time_mid}"
        self.grpc_name = f"ledger_grpc_server_{uuid.uuid4().time_mid}"
        self.cmds = {
            self.ledger_name: [
                "./speculos/speculos.py",
                "--display=headless",
                f"--button-port={ZEMU_BUTTON_PORT}",
                "./speculos/apps/crypto.elf",
            ],
            self.proxy_name: ["./speculos/tools/ledger-live-http-proxy.py", "-v"],
            self.grpc_name: ["bash", "-c", "RUST_LOG=debug zemu-grpc-server"],
        }
        self.client = docker.from_env()
        self.client.images.pull(ZEMU_IMAGE)
        self.containers = []

    def start(self):
        host_config_ledger = self.client.api.create_host_config(
            auto_remove=True,
            port_bindings={
                ZEMU_BUTTON_PORT: ZEMU_BUTTON_PORT,
                ZEMU_GRPC_SERVER_PORT: ZEMU_GRPC_SERVER_PORT,
            },
        )
        container_ledger = self.client.api.create_container(
            ZEMU_IMAGE,
            self.cmds[self.ledger_name],
            name=self.ledger_name,
            ports=[ZEMU_BUTTON_PORT, ZEMU_GRPC_SERVER_PORT],
            host_config=host_config_ledger,
        )
        self.client.api.start(container_ledger["Id"])
        self.containers.append(container_ledger)
        for name in [self.proxy_name, self.grpc_name]:
            cmd = self.cmds[name]
            try:
                host_config = self.client.api.create_host_config(
                    auto_remove=True, network_mode=f"container:{self.ledger_name}"
                )
                container = self.client.api.create_container(
                    ZEMU_IMAGE,
                    cmd,
                    name=name,
                    host_config=host_config,
                )
                self.client.api.start(container["Id"])
                self.containers.append(container)
                time.sleep(2)
            except Exception as e:
                print(e)

    def stop(self):
        for container in self.containers:
            try:
                self.client.api.remove_container(container["Id"], force=True)
                print("stop docker {}".format(container["Name"]))
            except Exception as e:
                print(e)


class LedgerButton:
    def __init__(self, zemu_address, zemu_button_port):
        self._client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.zemu_address = zemu_address
        self.zemu_button_port = zemu_button_port
        self.connected = False

    @property
    def client(self):
        if not self.connected:
            time.sleep(5)
            self._client.connect((self.zemu_address, self.zemu_button_port))
            self.connected = True
        return self._client

    def press_left(self):
        data = "Ll"
        self.client.send(data.encode())

    def press_right(self):
        data = "Rr"
        self.client.send(data.encode())

    def press_both(self):
        data = "LRlr"
        self.client.send(data.encode())
