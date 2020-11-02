let
  pkgs = import ../nix { };
  genesis = (import ../. { inherit pkgs; }).chain-maind-zemu;
  # pin to a revision to avoid unnessesary rebuild
  pinned = (import
    (builtins.fetchTarball
      "https://github.com/crypto-com/chain-main/archive/de34e77ef793b0e7975eb3596844245b61b4f652.tar.gz")
    { inherit pkgs; ledger_zemu = true; });
  upgrade-test = pinned.overrideAttrs (old: {
    patches = [ ./upgrade-test.patch ];
  });
in
pkgs.linkFarm "upgrade-test-package" [
  { name = "genesis"; path = genesis; }
  { name = "upgrade-test"; path = upgrade-test; }
]
