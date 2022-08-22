{ pkgs ? import ./default.nix { } }:
let
  version = "v1.0.0-rc.2";
  srcUrl = {
    x86_64-linux = {
      url =
        "https://github.com/informalsystems/ibc-rs/releases/download/${version}/hermes-${version}-x86_64-unknown-linux-gnu.tar.gz";
      sha256 = "sha256-ms+3Ka8Ijbx63OXQzzNZ1kLrwVJDIVnvyc1TG69bun0=";
    };
    x86_64-darwin = {
      url =
        "https://github.com/informalsystems/ibc-rs/releases/download/${version}/hermes-${version}-x86_64-apple-darwin.tar.gz";
      sha256 = "sha256-ygp49IPTXKqK12gE8OiyXjXhkJvfUZNuXVnS14SVScQ=";
    };
  }.${pkgs.stdenv.system} or (throw
    "Unsupported system: ${pkgs.stdenv.system}");
in
pkgs.stdenv.mkDerivation {
  name = "hermes";
  inherit version;
  src = pkgs.fetchurl srcUrl;
  sourceRoot = ".";
  installPhase = ''
    echo "hermes"
    echo $out
    install -m755 -D hermes $out/bin/hermes
  '';

  meta = with pkgs.lib; { platforms = with platforms; linux ++ darwin; };

}
