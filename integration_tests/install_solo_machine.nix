{
  stdenv,
  fetchurl,
  lib,
}:
let
  version = "v0.1.4";
  srcUrl =
    {
      x86_64-linux = {
        url = "https://github.com/crypto-com/ibc-solo-machine/releases/download/${version}/ubuntu-latest-${version}.tar.gz";
        sha256 = "sha256-b+A8G7HGl1Kv32X0ybV6RODQjqAHqfAo3DQh1DtY6UQ=";
      };
      x86_64-darwin = {
        url = "https://github.com/crypto-com/ibc-solo-machine/releases/download/${version}/macos-latest-${version}.tar.gz";
        sha256 = "sha256-9Zo3sGxnjB05X90FFK/3yGbWokxJqVL0teb1x1z5a0U=";
      };
      aarch64-darwin = {
        url = "https://github.com/crypto-com/ibc-solo-machine/releases/download/${version}/macos-latest-${version}.tar.gz";
        sha256 = "sha256-9Zo3sGxnjB05X90FFK/3yGbWokxJqVL0teb1x1z5a0U=";
      };
    }
    .${stdenv.system} or (throw "Unsupported system: ${stdenv.system}");
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
  meta = with lib; {
    platforms = with platforms; linux ++ darwin;
  };
}
