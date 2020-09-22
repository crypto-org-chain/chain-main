{ pkgs ? import <nixpkgs> {} }:
let
  chain = import ./default.nix { inherit pkgs; };
  pystarport = import ./pystarport/default.nix { inherit pkgs; };
in
  pkgs.mkShell {
    buildInputs = [chain pystarport pkgs.python3Packages.pytest-asyncio pkgs.python3Packages.pytest];
  }
