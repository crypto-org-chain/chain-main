{ pkgs ? import <nixpkgs> { }, ci ? false }:
with pkgs;
let
  chaind = import ../. { inherit pkgs; ledger_zemu = true; };
  pystarport = import ../pystarport {
    inherit pkgs;
    chaind = if ci then "${chaind}/bin/chain-maind" else "chain-maind";
  };
  testenv = poetry2nix.mkPoetryEnv { projectDir = ./.; };
in
mkShell {
  buildInputs =
    (if ci then [ chaind.instrumented ] else [ ]) ++ [
      pystarport
      testenv
      nixpkgs-fmt
    ];
}
