{
  src,
  lib,
  stdenv,
  darwin,
  rustPlatform,
  symlinkJoin,
  openssl,
}:
rustPlatform.buildRustPackage rec {
  name = "hermes";
  inherit src;
  cargoHash = "sha256-VJbY5Kmhc5NOfPYCdCgKspaa1TqJM13qZuvzsrWkQvI=";
  cargoBuildFlags = "-p ibc-relayer-cli";
  buildInputs = lib.optionals stdenv.isDarwin [
    darwin.apple_sdk.frameworks.SystemConfiguration
    darwin.apple_sdk.frameworks.Security
    darwin.libiconv
  ];
  doCheck = false;
  RUSTFLAGS = "--cfg ossl111 --cfg ossl110 --cfg ossl101";
  OPENSSL_NO_VENDOR = "1";
  OPENSSL_DIR = symlinkJoin {
    name = "openssl";
    paths = with openssl; [
      out
      dev
    ];
  };
}
