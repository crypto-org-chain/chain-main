let
  pkgs = import ../nix { };
  genesis = (import ../. { inherit pkgs; }).chain-maind-zemu;
  upgrade-test = genesis.overrideAttrs (old: {
    patches = [ ./upgrade-test.patch ];
  });
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = genesis; }
  { name = "upgrade-test"; path = upgrade-test; }
]
