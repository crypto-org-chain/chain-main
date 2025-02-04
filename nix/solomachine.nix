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
  version = "0.1.4";

  src = fetchFromGitHub {
    owner = "crypto-com";
    repo = pname;
    rev = "v${version}";
    sha256 = "sha256-+jfRbPm31/pBuseUS89cuYSAPw2l/509MVTaUcuyaGY=";
  };

  cargoSha256 = "sha256-9Mx70yBoNy711PFC5y2VoXD3kqmcMvDsjP9AaC1VfCM=";
  cargoBuildFlags = "-p solo-machine -p mnemonic-signer";
  nativeBuildInputs = [
    protobuf
    rustfmt
  ];
  buildInputs = lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.SystemConfiguration ];
  doCheck = false;
}
