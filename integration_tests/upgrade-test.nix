let
  pkgs = import ../nix { };
  released = (import (builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/v1.1.0.tar.gz") { }).chain-maind;
  released2 = (import (builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/v2.0.1.tar.gz") { }).chain-maind;
  released3 = (import (builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/v3.3.4.tar.gz") { }).chain-maind;
  current = pkgs.callPackage ../. { };
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = released; }
  { name = "v2.0.0"; path = released2; }
  { name = "v3.0.0"; path = released3; }
  { name = "v4.2.0"; path = current; }
]
