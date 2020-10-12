{ pkgs ? import <nixpkgs> {} }:
let chain = import ../. { inherit pkgs; };
in
  pkgs.poetry2nix.mkPoetryApplication {
    projectDir = ./.;
    # FIXME remove after merged: https://github.com/nix-community/poetry2nix/pull/189
    overrides = pkgs.poetry2nix.overrides.withDefaults (self: super: {
      supervisor = super.supervisor.overridePythonAttrs (
        old: {
          propagatedBuildInputs = with pkgs.python3Packages; [ meld3 setuptools ];
        }
      );
    });
    preBuild = ''
    sed -i -e 's@CHAIN = "chain-maind"  # edit by nix-build@CHAIN = "${chain}/bin/chain-maind"@' pystarport/cluster.py
    '';
  }
