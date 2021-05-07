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
        cargoSha256 = sha256:0qrdjbjaz0p1dxrhvki8blnrmp354gdp417033aany53j2bh9sfp;
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
  ];
  config = { };
  inherit system;
}
