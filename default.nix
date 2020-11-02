{ pkgs ? import ./nix { }, commit ? "" }:
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
  build-chain-maind = ledger_zemu: pkgs.buildGoModule rec {
    pname = "chain-maind";
    version = "0.0.1";
    src = pkgs.lib.cleanSourceWith {
      name = "src";
      src = pkgs.lib.sourceByRegex ./. src_regexes;
    };
    subPackages = [ "cmd/chain-maind" ];
    vendorSha256 = sha256:0p8i1pj42wlgmqgs50pv6rv84vqk4s9baipjk5zn2xkcbaxx05iz;
    runVend = true;
    outputs = [
      "out"
      "instrumented"
    ];
    buildTags = "cgo,ledger,!test_ledger_mock,!ledger_mock," +
      (if ledger_zemu then "ledger_zemu" else "!ledger_zemu");
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
  chain-maind = build-chain-maind false;
  pystarport = import ./pystarport { inherit pkgs; chaind = "${chain-maind}/bin/chain-maind"; };

  # for testing and dev
  chain-maind-zemu = build-chain-maind true;
  pystarport-unbind = import ./pystarport { inherit pkgs; };

  lint-shell = pkgs.mkShell {
    buildInputs = with pkgs; [
      (poetry2nix.mkPoetryEnv { projectDir = ./integration_tests; })
      nixpkgs-fmt
    ];
  };

  # test in ci-shell will build chain-maind with nix automatically
  ci-shell = pkgs.mkShell {
    inputsFrom = [ lint-shell ];
    buildInputs = with pkgs; [
      chain-maind-zemu.instrumented
      (import ./pystarport { inherit pkgs; chaind = "${chain-maind-zemu}/bin/chain-maind"; })
      cosmovisor
    ];
  };

  # test in dev-shell will use the chain-maind in PATH
  dev-shell = pkgs.mkShell {
    inputsFrom = [ lint-shell ];
    buildInputs = with pkgs; [
      go
      python3Packages.poetry
      pystarport-unbind
      cosmovisor
    ];

    shellHook = ''
      # prefer local pystarport directory for development
      export PYTHONPATH=./pystarport:$PYTHONPATH
    '';
  };
}
