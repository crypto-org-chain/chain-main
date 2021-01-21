{ pkgs ? import <nixpkgs> { }
, chaind ? "chain-maind"
}:
pkgs.poetry2nix.mkPoetryApplication {
  projectDir = ./.;
  preBuild = ''
    sed -i -e 's@CHAIN = ""  # edit by nix-build@CHAIN = "${chaind}"@' pystarport/app.py
  '';
}
