{ system ? builtins.currentSystem, pkgs ? import ./nix { inherit system; }, commit ? "" }:
let
  src_regexes = [
    "^x$"
    "^x/.*"
    "^app$"
    "^app/.*"
    "^config$"
    "^config/.*"
    "^cmd$"
    "^cmd/.*"
    "^proto$"
    "^proto/.*"
    "^test$"
    "^test/.*"
    "^go.mod$"
    "^go.sum$"
  ];
  lib = pkgs.lib;
  build-chain-maind = { ledger_zemu ? false, network ? "mainnet" }: pkgs.buildGoModule rec {
    pname = "chain-maind";
    version = "0.0.1";
    src = lib.cleanSourceWith {
      name = "src";
      src = lib.sourceByRegex ./. src_regexes;
    };
    subPackages = [ "cmd/chain-maind" ];
    vendorSha256 = sha256:138n1y7bsiy7zss3kpk6nmwg8vkwanhif8mjsd8v25m1cg73d4dc;
    runVend = true;
    outputs = [
      "out"
      "instrumented"
    ];
    buildTags = "cgo,ledger,!test_ledger_mock,!ledger_mock," +
      (if ledger_zemu then "ledger_zemu" else "!ledger_zemu") +
      (lib.optionalString (network == "testnet") ",testnet");
    buildFlags = "-tags ${buildTags}";
    buildFlagsArray = ''
      -ldflags=
      -X github.com/cosmos/cosmos-sdk/version.Name=crypto-com-chain
      -X github.com/cosmos/cosmos-sdk/version.AppName=${pname}
      -X github.com/cosmos/cosmos-sdk/version.Version=${version}
      -X github.com/cosmos/cosmos-sdk/version.Commit=${commit}
      -X github.com/cosmos/cosmos-sdk/version.BuildTags=${buildTags}
    '';

    instrumentedBinary = "chain-maind-inst";
    # FIXME remove the "-w -s" ldflags, https://github.com/golang/go/issues/40974
    postBuild = ''
      echo "Build instrumented binary"
      go test ./cmd/chain-maind $buildFlags",testbincover" "''${buildFlagsArray[@]} -w -s" -coverpkg=./...,github.com/cosmos/cosmos-sdk/x/... -c -o ${instrumentedBinary}
    '';
    postInstall = ''
      mkdir -p $instrumented/bin
      mv ./${instrumentedBinary} $instrumented/bin/
    '';
    preFixup = ''
      find $instrumented/bin/ -type f 2>/dev/null | xargs -r remove-references-to -t ${pkgs.go} || true
    '';
  };
in
rec {
  inherit (pkgs) relayer cosmovisor;

  chain-maind = build-chain-maind { };
  pystarport = import ./pystarport { inherit pkgs; chaind = "${chain-maind}/bin/chain-maind"; };

  chain-maind-testnet = build-chain-maind { network = "testnet"; };

  # for testing and dev
  chain-maind-zemu = build-chain-maind { ledger_zemu = true; };
  # one can set binary with environment variable CHAIN_MAIND, or it'll find chain-maind in PATH
  pystarport-unbind = import ./pystarport { inherit pkgs; };

  # python env for python linter tools and pytest
  test-pyenv = pkgs.poetry2nix.mkPoetryEnv { projectDir = ./integration_tests; };

  # lint tools
  lint-env = pkgs.buildEnv {
    name = "lint-env";
    pathsToLink = [ "/bin" "/share" ];
    paths = with pkgs; [
      test-pyenv
      nixpkgs-fmt
      (pkgs.writeShellScriptBin "lint-ci" ''
        EXIT_STATUS=0
        go mod verify || EXIT_STATUS=$?
        flake8 --show-source --count --statistics \
          --format="::error file=%(path)s,line=%(row)d,col=%(col)d::%(path)s:%(row)d:%(col)d: %(code)s %(text)s" \
          || EXIT_STATUS=$?
        find . -name "*.nix" -type f | xargs nixpkgs-fmt --check || EXIT_STATUS=$?
        exit $EXIT_STATUS
      '')
    ];
  };
  common-env = [
    cosmovisor
    relayer
  ];

  # sources for integration tests
  # it needs the chain source code to build patched binaries on the fly
  tests_src = lib.sourceByRegex ./. ([
    "^integration_tests$"
    "^integration_tests/.*\\.py$"
    "^integration_tests/configs$"
    "^integration_tests/configs/.*"
    "^integration_tests/upgrade-test.nix$"
    "^integration_tests/upgrade-test.patch$"
    "^nix$"
    "^nix/.*"
    "^default.nix$"
  ] ++ src_regexes);

  # an env which can run integration tests
  ci-env = pkgs.buildEnv {
    name = "ci-env";
    pathsToLink = [ "/bin" "/share" ];
    paths = with pkgs; [
      lint-env
      chain-maind-zemu
      chain-maind-zemu.instrumented
    ] ++ common-env;
  };

  # main entrypoint script to run integration tests
  run-integration-tests = pkgs.writeShellScriptBin "run-integration-tests" ''
    set -e
    export PATH=${ci-env}/bin:$PATH
    export TESTS=${tests_src}/integration_tests
    pytest -v -n 3 -m 'not ledger and not upgrade' --dist loadscope $TESTS
    pytest -v -m upgrade $TESTS
    pytest -v -m ledger $TESTS
  '';

  ci-shell = pkgs.mkShell {
    buildInputs = [
      ci-env
      run-integration-tests
    ];
    shellHook = ''
      export TESTS=${tests_src}/integration_tests
    '';
  };

  # test in dev-shell will use the chain-maind in PATH
  dev-shell = pkgs.mkShell {
    buildInputs = with pkgs; [
      lint-env
      go
      python3Packages.poetry
      pystarport-unbind
    ] ++ common-env;

    shellHook = ''
      # prefer local pystarport directory for development
      export PYTHONPATH=./pystarport:$PYTHONPATH
      # convinience for working with remote shell
      export SRC=${chain-maind.src}
    '';
  };

  chain-utils-testnet = import ./scripts/chain-utils.nix {
    inherit pkgs; network = "testnet";
  };
}
