{ pkgs ? import <nixpkgs> { }
}:
pkgs.poetry2nix.mkPoetryEnv { projectDir = ./.; }
