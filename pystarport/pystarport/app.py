import os

CHAIN = ""  # edit by nix-build
if not CHAIN:
    CHAIN = os.environ.get("CHAIN_MAIND", "chain-maind")
IMAGE = "docker.pkg.github.com/crypto-org-chain/chain-main/chain-main-pystarport:latest"

SUPERVISOR_CONFIG_FILE = "tasks.ini"
