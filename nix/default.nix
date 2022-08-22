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
      hermes = pkgs.callPackage ./hermes.nix { src = sources.ibc-rs; };
    })
    (import "${sources.gomod2nix}/overlay.nix")
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
