{ pkgs, network }:
let
  fetch-src = ref: builtins.fetchTarball "https://github.com/crypto-com/chain-main/archive/${ref}.tar.gz";
  chain-maind-testnet-rc2 = import (fetch-src "v0.7.0-rc2") {
    inherit pkgs;
    network = "testnet";
  };

  cfg =
    if network == "testnet" then {
      chaind = chain-maind-testnet-rc2;
      chain-id = "testnet-croeseid-1";
      genesis = pkgs.fetchurl {
        url = "https://raw.githubusercontent.com/crypto-com/chain-docs/master/docs/getting-started/assets/genesis_file/testnet-croeseid-1/genesis.json";
        sha256 = "55de3738cf6a429d19e234e59e81141af2f0dfa24906d22b949728023c1af382";
      };
      seeds =
        "66a557b8feef403805eb68e6e3249f3148d1a3f2@54.169.58.229:26656,3246d15d34802ca6ade7f51f5a26785c923fb385@54.179.111.207:26656,69c2fbab6b4f58b6cf1f79f8b1f670c7805e3f43@18.141.107.57:26656";
      rpc_servers = "https://testnet-croeseid-1.crypto.com:26657,https://testnet-croeseid-1.crypto.com:26657";
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
    RPCSERVER="$(echo "${cfg.rpc_servers}" | cut -d ',' -f 1)"
    LASTEST_HEIGHT=$(curl -s $RPCSERVER/block | jq -r .result.block.header.height)
    BLOCK_HEIGHT=$((LASTEST_HEIGHT - 1000))
    TRUST_HASH=$(curl -s "$RPCSERVER/block?height=$BLOCK_HEIGHT" | jq -r .result.block_id.hash)
    sed -i.bak -E "s|^(enable[[:space:]]+=[[:space:]]+).*$|\1true| ; \
    s|^(rpc_servers[[:space:]]+=[[:space:]]+).*$|\1\"${cfg.rpc_servers}\"| ; \
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
