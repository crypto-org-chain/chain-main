{ pkgs ? import <nixpkgs> { } }:
with pkgs;
mkShell {
  inputsFrom = [
    # base env
    (import ./integration_tests/shell.nix { inherit pkgs; })
  ];
  buildInputs = [
    go
    python3Packages.poetry
  ];
  shellHook = ''
    # prefer local pystarport directory for development
    export PYTHONPATH=./pystarport:$PYTHONPATH
  '';
}
