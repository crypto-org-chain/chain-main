{ pkgs ? import <nixpkgs> { }
, chaind ? "chain-maind"
}:
pkgs.poetry2nix.mkPoetryApplication {
  projectDir = ./.;
  preBuild = ''
    sed -i -e 's@CHAIN = "chain-maind"  # edit by nix-build@CHAIN = "${chaind}"@' pystarport/cluster.py
  '';
}
