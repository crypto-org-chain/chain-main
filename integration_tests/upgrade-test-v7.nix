# CAN BE REMOVED ONCE MAINNET IS UPGRADED TO v7.2.0
let
  pkgs = import ../nix { };
  # Pre-built Linux x86_64 release binary for the v6 baseline. Mirrors the
  # fetchBinary pattern in upgrade-test.nix to avoid pulling old nixpkgs
  # closures.
  fetchBinary =
    version: hash:
    pkgs.runCommand "chain-maind-${version}"
      {
        src = pkgs.fetchurl {
          url = "https://github.com/crypto-org-chain/chain-main/releases/download/v${version}/chain-main_${version}_Linux_x86_64.tar.gz";
          inherit hash;
        };
        nativeBuildInputs = [
          pkgs.gnutar
          pkgs.gzip
        ];
      }
      ''
        mkdir -p $out/bin
        tar -xzf $src -C $out
        mkdir -p $out/bin
        tar -xzf $src -C $out
        # If binary extracted to root (not bin/), move it into place.
        [ -f "$out/chain-maind" ] && mv "$out/chain-maind" "$out/bin/chain-maind"
        chmod +x $out/bin/chain-maind
      '';
  v6 = fetchBinary "6.0.0" "sha256-d8Z9i0wlkAVKysSkfzvjr4B4GXviNIQ0G0Hotk2bpWA=";
  current = pkgs.callPackage ../. { };
in
# Focused v6 → v7 upgrade path. Used by test_upgrade_v7.py to verify the
# orphan rewards_pool BaseAccount-to-ModuleAccount heal in the v7 handler.
# See upgrade-test.nix for the full v1.1.0 → v7 path.
pkgs.linkFarm "upgrade-test-v7-package" [
  {
    name = "genesis";
    path = v6;
  }
  {
    name = "v7";
    path = current;
  }
]
