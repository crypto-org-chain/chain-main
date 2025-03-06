{
  fetchFromGitHub,
  lib,
  stdenv,
  darwin,
  rustPlatform,
  protobuf,
  rustfmt,
}:
rustPlatform.buildRustPackage rec {
  pname = "ibc-solo-machine";
  version = "0.1.2";

  src = fetchFromGitHub {
    owner = "crypto-com";
    repo = pname;
    rev = "v${version}";
    hash = "sha256-9eUyljX0Sh/jbM7uiNo78vUevnTBP/MxvpDiiJLZ8Hk=";
  };

  cargoHash = "sha256-cuDc8h0Hb6fiizvhMKe3v2nPXkaIXx+1mgCH68KsB4g=";
  cargoBuildFlags = "-p solo-machine -p mnemonic-signer";
  nativeBuildInputs = [
    protobuf
    rustfmt
  ];
  buildInputs = lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.SystemConfiguration ];
  doCheck = false;
}
