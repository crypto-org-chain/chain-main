{ pkgs, network }:
let
  fetch-src = ref: builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/${ref}.tar.gz";
  chain-maind-testnet = (import (fetch-src "v0.9.1-croeseid") { }).chain-maind-testnet;
  chain-maind-mainnet = (import (fetch-src "v1.2.0") { }).chain-maind;

  cfg =
    if network == "testnet" then {
      chaind = chain-maind-testnet;
      chain-id = "testnet-croeseid-2";
      genesis = pkgs.fetchurl {
        url = "https://raw.githubusercontent.com/crypto-org-chain/testnets/main/testnet-croeseid-2/genesis.json";
        sha256 = sha256:af7c9828806da4945b1b41d434711ca233c89aedb5030cf8d9ce2d7cd46a948e;
      };
      seeds =
        "b2c6657096aa30c5fafa5bd8ced48ea8dbd2b003@52.76.189.200:26656,ef472367307808b242a0d3f662d802431ed23063@175.41.186.255:26656,d3d2139a61c2a841545e78ff0e0cd03094a5197d@18.136.230.70:26656";
      rpc_server = "https://testnet-croeseid.crypto.org:26657";
      minimum-gas-prices = "0.025basetcro";
    } else if network == "mainnet" then {
      chaind = chain-maind-mainnet;
      chain-id = "crypto-org-chain-mainnet-1";
      genesis = pkgs.fetchurl {
        url = "https://raw.githubusercontent.com/crypto-org-chain/mainnet/main/crypto-org-chain-mainnet-1/genesis.json";
        sha256 = sha256:d299dcfee6ae29ca280006eaa065799552b88b978e423f9ec3d8ab531873d882;
      };
      seeds =
        "8dc1863d1d23cf9ad7cbea215c19bcbe8bf39702@p2p.baaa7e56-cc71-4ae4-b4b3-c6a9d4a9596a.cryptodotorg.bison.run:26656,dc2540dabadb8302da988c95a3c872191061aed2@p2p.7d1b53c0-b86b-44c8-8c02-e3b0e88a4bf7.cryptodotorg.herd.run:26656,d2862ef8f86f9976daa0c6f59455b2b1452dc53b@p2p.a088961f-5dfd-4007-a15c-3a706d4be2c0.cryptodotorg.herd.run:26656,87c3adb7d8f649c51eebe0d3335d8f9e28c362f2@seed-0.crypto.org:26656,e1d7ff02b78044795371beb1cd5fb803f9389256@seed-1.crypto.org:26656,2c55809558a4e491e9995962e10c026eb9014655@seed-2.crypto.org:26656";
      rpc_server = "https://mainnet.crypto.org:26657";
      minimum-gas-prices = "0.025basecro";
    } else { };

  init-node = pkgs.writeShellScriptBin "init-node" ''
    set -e
    export PATH=${pkgs.coreutils}/bin:${pkgs.gnused}/bin:${pkgs.jq}/bin:${pkgs.curl}/bin:${cfg.chaind}/bin

    [[ -z "$MONIKER" ]] && { echo "environment variable MONIKER is not set"; exit 1; }
    CHAINHOME="''${CHAINHOME:-"$HOME/.chain-maind"}"

    chain-maind init $MONIKER --chain-id ${cfg.chain-id} --home $CHAINHOME
    ln -sf ${cfg.genesis} $CHAINHOME/config/genesis.json
    sed -i.bak -E 's#^(minimum-gas-prices[[:space:]]+=[[:space:]]+)""$#\1"${cfg.minimum-gas-prices}"#' $CHAINHOME/config/app.toml
    sed -i.bak -E 's#^(seeds[[:space:]]+=[[:space:]]+).*$#\1"${cfg.seeds}"# ; s#^(create_empty_blocks_interval[[:space:]]+=[[:space:]]+).*$#\1"5s"#' $CHAINHOME/config/config.toml
    LASTEST_HEIGHT=$(curl -s ${cfg.rpc_server}/block | jq -r .result.block.header.height)
    BLOCK_HEIGHT=$((LASTEST_HEIGHT - 1000))
    TRUST_HASH=$(curl -s "${cfg.rpc_server}/block?height=$BLOCK_HEIGHT" | jq -r .result.block_id.hash)
    sed -i.bak -E "s|^(enable[[:space:]]+=[[:space:]]+).*$|\1true| ; \
    s|^(rpc_servers[[:space:]]+=[[:space:]]+).*$|\1\"${cfg.rpc_server},${cfg.rpc_server}\"| ; \
    s|^(trust_height[[:space:]]+=[[:space:]]+).*$|\1$BLOCK_HEIGHT| ; \
    s|^(trust_hash[[:space:]]+=[[:space:]]+).*$|\1\"$TRUST_HASH\"|" $CHAINHOME/config/config.toml
  '';

  print-systemd-config = pkgs.writeShellScriptBin "print-systemd-config" ''
    CHAINHOME="''${CHAINHOME:-"$HOME/.chain-maind"}"
    cat << EOF
    # /etc/systemd/system/chain-maind.service
    [Unit]
    Description=Chain-maind
    ConditionPathExists=${cfg.chaind}/bin/chain-maind
    After=network.target

    [Service]
    Type=simple
    User=$USER
    WorkingDirectory=$CHAINHOME
    ExecStart=${cfg.chaind}/bin/chain-maind start --home $CHAINHOME
    Restart=on-failure
    RestartSec=10
    LimitNOFILE=4096

    [Install]
    WantedBy=multi-user.target
    EOF
  '';
in
pkgs.buildEnv {
  name = "chain-utils-" + network;
  pathsToLink = [ "/bin" "/etc" "/share" ];
  paths = [
    cfg.chaind
    init-node
    print-systemd-config
  ];
}
