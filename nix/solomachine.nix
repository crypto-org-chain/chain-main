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
    rev = "3a1d8a769edb47c5b665a7b618b940510c0f1283";
    sha256 = "sha256-slo1RTICKzrci0YFUgr/OMtg2VprxlWcZgL+zT6s65k=";
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
