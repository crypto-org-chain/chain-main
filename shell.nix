{ pkgs ? import <nixpkgs> {} }:
with pkgs;
let
  chain = import ./. { inherit pkgs; };
  pystarport = poetry2nix.mkPoetryEnv {
    projectDir = ./pystarport;
    editablePackageSources = {
      pystarport = ./pystarport;
    };
  };
in
  mkShell {
    buildInputs = [
      chain
      pystarport
      python3Packages.poetry
      python3Packages.pytest_xdist
      python3Packages.pytest
      python3Packages.flake8
      python3Packages.black
      python3Packages.isort
    ];
  }
