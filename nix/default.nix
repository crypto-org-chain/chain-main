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
    (import "${sources.gomod2nix}/overlay.nix")
    (_: pkgs: {
      hermes = pkgs.stdenv.mkDerivation rec {
        version = "v1.0.0-rc.2";
        name = "hermes";
        src = pkgs.fetchurl {
          x86_64-linux = {
            url =
              "https://github.com/informalsystems/ibc-rs/releases/download/${version}/hermes-${version}-x86_64-unknown-linux-gnu.tar.gz";
            sha256 = "sha256-ms+3Ka8Ijbx63OXQzzNZ1kLrwVJDIVnvyc1TG69bun0=";
          };
          x86_64-darwin = {
            url =
              "https://github.com/informalsystems/ibc-rs/releases/download/${version}/hermes-${version}-x86_64-apple-darwin.tar.gz";
            sha256 = "sha256-ygp49IPTXKqK12gE8OiyXjXhkJvfUZNuXVnS14SVScQ=";
          };
        }.${pkgs.stdenv.system} or (throw
          "Unsupported system: ${pkgs.stdenv.system}");
        sourceRoot = ".";
        installPhase = ''
          echo "hermes"
          echo $out
          install -m755 -D hermes $out/bin/hermes
        '';
        meta = with pkgs.lib; { platforms = with platforms; linux ++ darwin; };
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
