{
  pkgs ? import ../nix { },
}:
pkgs.poetry2nix.mkPoetryEnv { projectDir = ./.; }
