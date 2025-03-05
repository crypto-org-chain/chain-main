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
  version = "main";

  src = fetchFromGitHub {
    owner = "crypto-com";
    repo = pname;
    rev = "18c80bf993c8715551793c0d881af16f1ed9958a";
    sha256 = "sha256-f9WSerHGT2L6ervgNg2l9LPHwvNxTPvXydcODZ3P2p8=";
  };

  cargoSha256 = "sha256-0A61jlQuJoci7SXh7fEW1dyKr4huDM9teh2ead2vFTI=";
  cargoBuildFlags = "-p solo-machine -p mnemonic-signer";
  nativeBuildInputs = [
    protobuf
    rustfmt
  ];
  buildInputs = lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.SystemConfiguration ];
  doCheck = false;
}
