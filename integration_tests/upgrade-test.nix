let
  pkgs = import ../nix { };
  # Fetch a pre-built Linux x86_64 release binary from GitHub.
  # Using pre-built binaries avoids pulling in old nixpkgs closures for
  # historical versions, keeping cachix storage small.
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
  v1 = fetchBinary "1.1.0" "sha256-+KknygtcWrilun443B8nJTO01meOyAsqzRHadUIhFJY=";
  v2 = fetchBinary "2.0.1" "sha256-Xp6fcDy4XHJXMIbjhOGH51JGOy7QzNYSCUofKaE/AVg=";
  v3 = fetchBinary "3.3.4" "sha256-EPkVhe6Gfttrygl9vqlMVyuSP7BHa/j0pTWepLaqIog=";
  v4 = fetchBinary "4.2.0" "sha256-gNreRND2dJDx46TSdoTYMDMJWNgP8qxRpMLbMY61rMg=";
  v5 = fetchBinary "5.0.0" "sha256-hJFRQNhBPDrqREyTL8oOPOmh0F6fNEacYh5TrbijgY4=";
  v6 = fetchBinary "6.0.0" "sha256-d8Z9i0wlkAVKysSkfzvjr4B4GXviNIQ0G0Hotk2bpWA=";
  current = pkgs.callPackage ../. { };
in
# Full upgrade path: v1.1.0 (genesis) → ... → v7.
# To add a new protocol version: add a fetchBinary entry above and append it here.
pkgs.linkFarm "upgrade-test-all-package" [
  {
    name = "genesis";
    path = v1;
  }
  {
    name = "v2.0.0";
    path = v2;
  }
  {
    name = "v3.0.0";
    path = v3;
  }
  {
    name = "v4.2.0";
    path = v4;
  }
  {
    name = "v5.0.0";
    path = v5;
  }
  {
    name = "v6.0.0";
    path = v6;
  }
  {
    name = "v7";
    path = current;
  }
]
