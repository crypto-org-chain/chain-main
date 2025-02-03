{
  system ? builtins.currentSystem,
  pkgs ? import ../nix { inherit system; },
}:
pkgs.mkShell {
  buildInputs = with pkgs; [
    # build tools
    go_1_22
    rocksdb

    # lint tools
    test-env
    nixfmt-rfc-style
    lint-ci

    # tools
    cosmovisor
    hermes
    solomachine

    # chain-maind for testing
    chain-maind-test
  ];
  shellHook = ''
    export PYTHONPATH=$PWD/pystarport/proto_python/:$PYTHONPATH
    export SOLO_MACHINE_HOME="${pkgs.solomachine}/solomachine"
    mkdir -p "$PWD/coverage"
    export GOCOVERDIR="$PWD/coverage"
  '';
}
