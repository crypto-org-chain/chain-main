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
    rev = "d80b776f631c6a1a641931cac7a9a5fe3d802176";
    hash = "sha256-3s0mGDzuEZZLLo3jxpu7U2N6VfiuRHCceqwCGOfS6yw=";
  };

  cargoHash = "sha256-1SqK8OkaUJ7UHM/w5v0NDb4L7NZzpwy17F11rkXMlJw=";
  cargoBuildFlags = "-p solo-machine -p mnemonic-signer";
  nativeBuildInputs = [
    protobuf
    rustfmt
  ];
  buildInputs = lib.optionals stdenv.isDarwin [ darwin.apple_sdk.frameworks.SystemConfiguration ];
  doCheck = false;
}
