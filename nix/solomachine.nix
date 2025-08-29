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
    owner = "yihuang";
    repo = pname;
    rev = "d34c17e6f79ae1fbb6d02f25ec55ceef426854fd";
    hash = "sha256-fxH2gGVCYe1GAGwNJXeAf1QBZftNCuWG4TCJSiV/mCs=";
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
