{ sources ? import ./sources.nix, system ? builtins.currentSystem }:
import sources.nixpkgs {
  overlays = [
    (_: pkgs: {
      flake-compat = import sources.flake-compat;
      cosmovisor = pkgs.buildGoModule rec {
        name = "cosmovisor";
        src = sources.cosmos-sdk + "/cosmovisor";
        subPackages = [ "./cmd/cosmovisor" ];
        vendorHash = "sha256-OAXWrwpartjgSP7oeNvDJ7cTR9lyYVNhEM8HUnv3acE=";
        doCheck = false;
      };
      hermes = pkgs.callPackage ./hermes.nix { src = sources.ibc-rs; };
      solomachine = pkgs.callPackage ./solomachine.nix { };
    })
    (import "${sources.poetry2nix}/overlay.nix")
    (import "${sources.gomod2nix}/overlay.nix")
    (import ./build_overlay.nix)
    (pkgs: prev: {
      go = pkgs.go_1_22;
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
      chain-maind-zemu = pkgs.callPackage ../. {
        ledger_zemu = true;
      };
      # chain-maind for integration test
      chain-maind-test = pkgs.callPackage ../. {
        ledger_zemu = true;
        coverage = true;
      };
    })
  ];
  config = { };
  inherit system;
}
