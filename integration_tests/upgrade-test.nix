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
  released5_0 =
    (fetchFlake "crypto-org-chain/chain-main" "246ba80c2f0c7e11a0a7b483a349d177ffeb0a9d").default;
  released6_0 =
    (fetchFlake "crypto-org-chain/chain-main" "bcd5bcb30a4bac7a5939a712ac079dd631abf41b").default;
  current = pkgs.callPackage ../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  {
    name = "genesis";
    path = released5_0;
  }
  {
    name = "v5.0.0";
    path = released5_0;
  }
  {
    name = "v6.0.0";
    path = released6_0;
  }
  {
    name = "v7.0.0";
    path = current;
  }
]
