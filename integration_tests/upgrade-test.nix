let
  pkgs = import ../nix { };
  released = (import (builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/v1.1.0.tar.gz") { }).chain-maind;
  released2 = (import (builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/v2.0.1.tar.gz") { }).chain-maind;
  released3 = (import (builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/v3.3.4.tar.gz") { }).chain-maind;
  fetchFlake = repo: rev: (pkgs.flake-compat {
    src = {
      outPath = builtins.fetchTarball "https://github.com/${repo}/archive/${rev}.tar.gz";
      inherit rev;
      shortRev = builtins.substring 0 7 rev;
    };
  }).defaultNix;
  released4 = (fetchFlake "crypto-org-chain/chain-main" "b3226f06fd2a236f9957304c4d83b0ea06ed2604").default;
  current = pkgs.callPackage ../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = released; }
  { name = "v2.0.0"; path = released2; }
  { name = "v3.0.0"; path = released3; }
  { name = "v4.2.0"; path = released4; }
  { name = "sdk47-upgrade"; path = current; }
]
