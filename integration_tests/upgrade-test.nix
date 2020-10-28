{ pkgs ? import <nixpkgs> { } }:
let
  genesis = import ../. { inherit pkgs; ledger_zemu = true; };
  # pin to a revision to avoid unnessesary rebuild
  upgrade-test = (import
    (builtins.fetchTarball
      "https://github.com/crypto-com/chain-main/archive/de34e77ef793b0e7975eb3596844245b61b4f652.tar.gz")
    {
      inherit pkgs;
      ledger_zemu = true;
    }).overrideAttrs (old: {
    patches = [ ./upgrade-test.patch ];
  });
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = genesis; }
  { name = "upgrade-test"; path = upgrade-test; }
]
