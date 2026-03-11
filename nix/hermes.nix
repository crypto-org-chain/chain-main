{
  src,
  lib,
  stdenv,
  darwin,
  rustPlatform,
  symlinkJoin,
  openssl,
  pkg-config,
  apple-sdk_15,
}:
rustPlatform.buildRustPackage rec {
  name = "hermes";
  inherit src;
  cargoBuildFlags = "-p ibc-relayer-cli";
  buildInputs = lib.optionals stdenv.isDarwin [
    apple-sdk_15
    pkg-config
    openssl
    darwin.libiconv
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
