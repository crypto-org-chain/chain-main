{ pkgs ? import <nixpkgs> {} }:
with pkgs;
let
  chain = import ../. { inherit pkgs; };
  pystarport = import ../pystarport { inherit pkgs; };
  testenv = poetry2nix.mkPoetryEnv { projectDir = ./.; };
in
  mkShell {
    buildInputs = [
      chain.instrumented
      pystarport
      testenv
    ];
  }
