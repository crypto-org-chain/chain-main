{
  src,
  lib,
  stdenv,
  darwin,
  rustPlatform,
  symlinkJoin,
  openssl,
  pkg-config,
}:

rustPlatform.buildRustPackage rec {
  name = "hermes";
  inherit src;
  cargoBuildFlags = "-p ibc-relayer-cli";
  buildInputs = lib.optionals stdenv.isDarwin [
    darwin.apple_sdk.frameworks.Security
    pkg-config
    openssl
    darwin.libiconv
    darwin.apple_sdk.frameworks.SystemConfiguration
  ];
  cargoLock = {
    lockFile = "${src}/Cargo.lock";
  };
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
