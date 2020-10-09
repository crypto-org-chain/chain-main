{ pkgs ? import <nixpkgs> {} }:
let chain = import ../. { inherit pkgs; };
in
  pkgs.poetry2nix.mkPoetryApplication {
    projectDir = ./.;
    preBuild = ''
    sed -i -e 's@CHAIN = "chain-maind"  # edit by nix-build@CHAIN = "${chain}/bin/chain-maind"@' pystarport/cluster.py
    '';
  }
