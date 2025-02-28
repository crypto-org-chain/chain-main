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
    owner = "yihuang";
    repo = pname;
    rev = "04a07839d4d1acadc805f728c505668d3225b70d";
    sha256 = "sha256-jeh0ZO91cME3AaePujn0T7MG3Hsoy4qLWwIbYCspyac=";
  };

  cargoSha256 = "sha256-bFusoI2vmQcjrELn1Kj252NHaWwbVymSuhE2B3M5QiU=";
  cargoBuildFlags = "-p solo-machine -p mnemonic-signer";
  nativeBuildInputs = [
    protobuf
    rustfmt
  ];
  buildInputs = lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.SystemConfiguration ];
  doCheck = false;
}
