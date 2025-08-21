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

  cargoHash = "sha256-AHK0aGCr2tQpdGhVo61drNnsGjusa6VycXs2neJZQp8=";
  cargoBuildFlags = "-p solo-machine -p mnemonic-signer";
  nativeBuildInputs = [
    protobuf
    rustfmt
  ];
  buildInputs = lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.SystemConfiguration ];
  doCheck = false;
}
