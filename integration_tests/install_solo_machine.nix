{ stdenv, fetchurl, lib }:
let
  version = "v0.1.4";
  srcUrl = {
    x86_64-linux = {
      url =
        "https://github.com/crypto-com/ibc-solo-machine/releases/download/${version}/ubuntu-latest-${version}.tar.gz";
      sha256 = "";
    };
    x86_64-darwin = {
      url =
        "https://github.com/crypto-com/ibc-solo-machine/releases/download/${version}/macos-latest-${version}.tar.gz";
      sha256 = "sha256-NYmm44l5exQiG9DbwUM/UZiEmxc0JriXM8/l/xpc+q4=";
    };
    aarch64-darwin = {
      url =
        "https://github.com/crypto-com/ibc-solo-machine/releases/download/${version}/macos-latest-${version}.tar.gz";
      sha256 = "sha256-NYmm44l5exQiG9DbwUM/UZiEmxc0JriXM8/l/xpc+q4=";
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
