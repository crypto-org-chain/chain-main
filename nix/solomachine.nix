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
    owner = "yihuang";
    repo = pname;
    rev = "2c4e1bac0a829dd5f43badfc949d16d8bce2e887";
    hash = "sha256-H9G4uMNVmvH1uwt0/kVlfo5XugIuIfNqFKKTxR9ijcU=";
  };

  cargoHash = "sha256-9Mx70yBoNy711PFC5y2VoXD3kqmcMvDsjP9AaC1VfCM=";
  cargoBuildFlags = "-p solo-machine -p mnemonic-signer";
  nativeBuildInputs = [
    protobuf
    rustfmt
  ];
  buildInputs = lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.SystemConfiguration ];
  doCheck = false;
}
