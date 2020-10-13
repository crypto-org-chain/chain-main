{ pkgs ? import <nixpkgs> {} }:
with pkgs;
let
  chain = import ../. { inherit pkgs; };
  pystarport = import ../pystarport { inherit pkgs; };
in
  mkShell {
    buildInputs = [
      chain.instrumented
      pystarport
      (with python3Packages; [
        pytest
        pytest_xdist
        flake8
        black
        isort
        pep8-naming
      ])
    ];
  }
