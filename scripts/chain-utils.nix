{ pkgs, network }:
let
  fetch-src = ref: builtins.fetchTarball "https://github.com/crypto-com/chain-main/archive/${ref}.tar.gz";
  chain-maind-testnet = (import (fetch-src "v0.8.0-croeseid") { }).chain-maind-testnet;

  cfg =
    if network == "testnet" then {
      chaind = chain-maind-testnet;
      chain-id = "testnet-croeseid-2";
      genesis = pkgs.fetchurl {
        url = "https://raw.githubusercontent.com/crypto-com/testnets/main/testnet-croeseid-2/genesis.json";
        sha256 = sha256:af7c9828806da4945b1b41d434711ca233c89aedb5030cf8d9ce2d7cd46a948e;
      };
      seeds =
        "b2c6657096aa30c5fafa5bd8ced48ea8dbd2b003@52.76.189.200:26656,ef472367307808b242a0d3f662d802431ed23063@175.41.186.255:26656,d3d2139a61c2a841545e78ff0e0cd03094a5197d@18.136.230.70:26656";
      rpc_server = "https://testnet-croeseid.crypto.com:26657";
      minimum-gas-prices = "0.025basetcro";
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
