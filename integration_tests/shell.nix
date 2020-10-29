{ pkgs ? import <nixpkgs> { }, ci ? false }:
with pkgs;
let
  chaind = import ../. { inherit pkgs; ledger_zemu = true; };
  pystarport = import ../pystarport {
    inherit pkgs;
    chaind = if ci then "${chaind}/bin/chain-maind" else "chain-maind";
  };
  testenv = poetry2nix.mkPoetryEnv { projectDir = ./.; };
  cosmovisor = buildGoModule rec {
    name = "cosmovisor";
    rev = "d83fc46b970c7bf8cac28b1a5cde5d3218a9f4ea";
    src = fetchTarball "https://github.com/cosmos/cosmos-sdk/archive/${rev}.tar.gz";
    sourceRoot = "source/cosmovisor";
    vendorSha256 = sha256:1mv0id290b4h8wrzq5z5n1bsq5im8glhlb8c5j7lrky30mikzwik;
    doCheck = false;
  };
in
mkShell {
  buildInputs =
    (if ci then [ chaind.instrumented ] else [ ]) ++ [
      pystarport
      testenv
      nixpkgs-fmt
      cosmovisor
    ];
}
