{ system ? builtins.currentSystem, pkgs ? import ../nix { inherit system; } }:
pkgs.mkShell {
  buildInputs = with pkgs; [
    # build tools
    go_1_18
    rocksdb

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
  shellHook = ''
    export PYTHONPATH=$PWD/pystarport/proto_python/:$PYTHONPATH
    export SOLO_MACHINE_HOME="${pkgs.solomachine}/solomachine"
  '';
}
