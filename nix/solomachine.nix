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
    rev = "49de8071fac70ce03db3088dc52b5f8b00f323d9";
    sha256 = "sha256-j5u5YMslULunqFb9t2TAGNVJ95S1F4GYEprgcajRlBQ=";
  };

  cargoSha256 = "sha256-tJQG4buDfa86g55UazqZVuNpOA09YE/dFSgrA9HopHU=";
  cargoBuildFlags = "-p solo-machine -p mnemonic-signer";
  nativeBuildInputs = [
    protobuf
    rustfmt
  ];
  buildInputs = lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.SystemConfiguration ];
  doCheck = false;
}
