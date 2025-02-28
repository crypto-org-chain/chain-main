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
    owner = "mmsqe";
    repo = pname;
    rev = "95a08151c584565a975530c2a52d24082ccf4824";
    sha256 = "sha256-qNifHvgI3dYUVGfbfkDCrTRJeB9+d8zC8gYJba7ATZg=";
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
