{ stdenv, fetchurl, lib }:
let
  version = "v0.1.2";
  srcUrl = {
    x86_64-linux = {
      url =
        "https://github.com/crypto-com/ibc-solo-machine/releases/download/${version}/ubuntu-latest-${version}.tar.gz";
      sha256 = "sha256-GEfHyUKvq69RAGWv29PAG3pFlBraXXhdkTcG045dePw=";
    };
    x86_64-darwin = {
      url =
        "https://github.com/crypto-com/ibc-solo-machine/releases/download/${version}/macos-latest-${version}.tar.gz";
      sha256 = "sha256-zx4342stMYzgQDXAKwnZKSfdLynGIApOFKZ+CjRCyaE=";
    };
  }.${stdenv.system} or (throw
    "Unsupported system: ${stdenv.system}");
in
stdenv.mkDerivation {
  name = "solomachine";
  inherit version;
  src = fetchurl srcUrl;
  sourceRoot = ".";
  installPhase = ''
    echo "installing solomachine ..."
    echo $out
    mkdir -p $out/solomachine
    install -m 755 -v -D * $out/solomachine
    echo `env`
  '';
  meta = with lib; { platforms = with platforms; linux ++ darwin; };
}
