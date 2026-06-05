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
  # v5.0.0 — genesis. Earliest version whose Go toolchain (1.22) runs on modern
  # macOS; v4.2.0 (Go 1.19) aborts under dyld (missing LC_UUID), and v1/v2/v3
  # have no flake, so all of them are excluded from the local upgrade chain.
  v5 = (fetchFlake "crypto-org-chain/chain-main" "246ba80c2f0c7e11a0a7b483a349d177ffeb0a9d").default;
  v6_0 =
    (fetchFlake "crypto-org-chain/chain-main" "bcd5bcb30a4bac7a5939a712ac079dd631abf41b").default;
  # v7.2.0
  v7_2 =
    (fetchFlake "crypto-org-chain/chain-main" "ee286fd84c4d43d7452bc366f27dc464cadb164a").default;
  current = pkgs.callPackage ../. { };
in

pkgs.linkFarm "upgrade-test-package" [
  {
    name = "genesis";
    path = v5;
  }
  {
    name = "v6.0.0";
    path = v6_0;
  }
  {
    name = "v7";
    path = v7_2;
  }
  {
    name = "v8";
    path = current;
  }
]
