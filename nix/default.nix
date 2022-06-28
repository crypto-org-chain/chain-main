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
    (pkgs: prev: {
      go = pkgs.go_1_17;
      test-env = pkgs.callPackage ./testenv.nix { };
      lint-ci = pkgs.writeShellScriptBin "lint-ci" ''
        EXIT_STATUS=0
        ${pkgs.go}/bin/go mod verify || EXIT_STATUS=$?
        ${pkgs.test-env}/bin/flake8 --show-source --count --statistics \
          --format="::error file=%(path)s,line=%(row)d,col=%(col)d::%(path)s:%(row)d:%(col)d: %(code)s %(text)s" \
          || EXIT_STATUS=$?
        find . -name "*.nix" -type f | xargs ${pkgs.nixpkgs-fmt}/bin/nixpkgs-fmt --check || EXIT_STATUS=$?
        exit $EXIT_STATUS
      '';
      solomachine = pkgs.callPackage ../integration_tests/install_solo_machine.nix { };
      chain-maind-zemu = pkgs.callPackage ../. {
        ledger_zemu = true;
      };
      rocksdb = (prev.rocksdb.overrideAttrs (old: rec {
        pname = "rocksdb";
        version = "6.29.5";
        src = sources.rocksdb;
      })).override { enableJemalloc = true; };
    })
  ];
  config = { };
  inherit system;
}
