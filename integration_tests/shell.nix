{ pkgs ? import <nixpkgs> { } }:
with pkgs;
let
  chaind = import ../. { inherit pkgs; ledger_zemu = true; };
  pystarport = import ../pystarport { inherit pkgs chaind; };
  testenv = poetry2nix.mkPoetryEnv { projectDir = ./.; };
in
mkShell {
  buildInputs = [
    chaind.instrumented
    pystarport
    testenv
    nixpkgs-fmt
  ];
}
