let
  pkgs = import ../nix { };
  released = (import (builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/v1.1.0.tar.gz") { }).chain-maind;
  released2 = (import (builtins.fetchTarball "https://github.com/crypto-org-chain/chain-main/archive/v2.0.1.tar.gz") { }).chain-maind;
  current = (import ../. { inherit pkgs; }).chain-maind;
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = released; }
  { name = "v2.0.0"; path = released2; }
  { name = "v3.0.0"; path = current; }
]
