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
  version = "0.1.5";

  src = fetchFromGitHub {
    owner = "crypto-com";
    repo = pname;
    rev = "v${version}";
    hash = "sha256-3s0mGDzuEZZLLo3jxpu7U2N6VfiuRHCceqwCGOfS6yw=";
  };

  cargoHash = "sha256-6QKORwLDBOKUX7lk/HE3K1PaCaoSad7ZnN5US8aQ05c=";
  cargoBuildFlags = "-p solo-machine -p mnemonic-signer";
  nativeBuildInputs = [
    protobuf
    rustfmt
  ];
  buildInputs = [ ];
  doCheck = false;
}
