{ system ? builtins.currentSystem, pkgs ? import ../nix { inherit system; } }:
pkgs.mkShell {
  buildInputs = with pkgs; [
    # build tools
    go_1_17
    gomod2nix
    rocksdb
    libwasmvm

    # lint tools
    test-env
    nixpkgs-fmt
    lint-ci

    # tools
    cosmovisor
    hermes
    solomachine

    # chain-maind for testing
    chain-maind-zemu
    chain-maind-zemu.instrumented
  ];
}
