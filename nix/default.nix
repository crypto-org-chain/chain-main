{ sources ? import ./sources.nix, system ? builtins.currentSystem }:
import sources.nixpkgs {
  overlays = [
    (_: pkgs: {
      cosmovisor = pkgs.buildGoModule rec {
        name = "cosmovisor";
        src = ../third_party/cosmos-sdk/cosmovisor;
        subPackages = [ "./cmd/cosmovisor" ];
        vendorSha256 = sha256:1hb9yxxm41yg21hm6qbjv53i7dr7qgdpis7y93hdibjs1apxc19q;
        doCheck = false;
      };
    })
    (import (sources.gomod2nix + "/overlay.nix"))
    (_: pkgs: {
      hermes = pkgs.rustPlatform.buildRustPackage rec {
        name = "hermes";
        src = sources.ibc-rs;
        cargoSha256 = sha256:1sc4m4bshnjv021ic82c3m36pakf15xr5cw0dsrcjzs8pv3nq9cd;
        cargoBuildFlags = "-p ibc-relayer-cli";
        buildInputs = pkgs.lib.optionals pkgs.stdenv.isDarwin [
          pkgs.darwin.apple_sdk.frameworks.Security
          pkgs.darwin.libiconv
        ];
        doCheck = false;
        RUSTFLAGS = "--cfg ossl111 --cfg ossl110 --cfg ossl101";
        OPENSSL_NO_VENDOR = "1";
        OPENSSL_DIR = pkgs.symlinkJoin {
          name = "openssl";
          paths = with pkgs.openssl; [ out dev ];
        };
      };
    })
    (_: pkgs: {
      libwasmvm = pkgs.rustPlatform.buildRustPackage rec {
        name = "libwasmvm";
        src = sources.wasmvm + "/libwasmvm";
        cargoDepsName = "vendor"; # use a static name to avoid rebuild when name changed
        cargoSha256 = "sha256-m3CtXHAkjNR7t7zie9FWK4k5xvr6/O2BfGQYi+foxCc=";
        doCheck = false;
      };
    })
  ];
  config = { };
  inherit system;
}
