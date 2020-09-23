{ pkgs ? import <nixpkgs> {} }:
let
  chain = import ./default.nix { inherit pkgs; };
  pystarport = pkgs.poetry2nix.mkPoetryEnv {
    projectDir = ./pystarport;
    editablePackageSources = {
      pystarport = ./pystarport;
    };
  };
in
  pkgs.mkShell {
    buildInputs = [chain pystarport pkgs.python3Packages.pytest-asyncio pkgs.python3Packages.pytest];
  }
