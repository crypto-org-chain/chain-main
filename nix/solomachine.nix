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
    owner = "devashishdxt";
    repo = pname;
    rev = "fae6e0cb3f49da9da460cc7200378e10d1fd63ce";
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
