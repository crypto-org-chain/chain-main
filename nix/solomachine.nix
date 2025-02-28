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

  cargoSha256 = "sha256-cuDc8h0Hb6fiizvhMKe3v2nPXkaIXx+1mgCH68KsB4g=";
  cargoBuildFlags = "-p solo-machine -p mnemonic-signer";
  nativeBuildInputs = [
    protobuf
    rustfmt
  ];
  buildInputs = lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.SystemConfiguration ];
  doCheck = false;
}
