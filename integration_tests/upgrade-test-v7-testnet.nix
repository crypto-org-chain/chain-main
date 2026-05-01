let
  pkgs = import ../nix { };
  fetchFlake =
    repo: rev:
    (pkgs.flake-compat {
      src = {
        outPath = builtins.fetchTarball "https://github.com/${repo}/archive/${rev}.tar.gz";
        inherit rev;
        shortRev = builtins.substring 0 7 rev;
      };
    }).defaultNix;
  # Both binaries are built with `-tags testnet` so the bech32 account
  # prefix is `tcro`.
  #
  # Pre-rewrite v7 binary — release v7 on testnet
  preRewriteV7 =
    (fetchFlake "crypto-org-chain/chain-main" "36febb2f85e0f133c2f173352925f99c9238290a")
    .packages.${builtins.currentSystem}.chain-maind-testnet;
  current = pkgs.callPackage ../. { network = "testnet"; };
in
pkgs.linkFarm "upgrade-test-v7-testnet-package" [
  {
    name = "genesis";
    path = preRewriteV7;
  }
  {
    name = "v7.1.0-testnet";
    path = current;
  }
]
