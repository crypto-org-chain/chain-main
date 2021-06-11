{ pkgs ? import <nixpkgs> { }
, chaind ? "chain-maind"
}:
pkgs.poetry2nix.mkPoetryEnv { projectDir = ./.; }

