{ lib
, stdenv
, buildGoApplication
, nix-gitignore
, go_1_18
, writeShellScript
, gomod2nix
, rocksdb ? null
, network ? "mainnet"  # mainnet|testnet
, rev ? "dirty"
, ledger_zemu ? false
}:
let
  inherit (lib) concatStringsSep;
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
    "^third_party$"
    "^third_party/cosmos-sdk$"
    "^third_party/cosmos-sdk/.*"
    "^gomod2nix.toml$"
  ];
in
buildGoApplication rec {
  pname = "chain-maind";
  version = "4.0.0";
  go = go_1_18;
  src = lib.cleanSourceWith {
    name = "src";
    src = lib.sourceByRegex ./. src_regexes;
  };
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/chain-maind" ];
  buildInputs = lib.lists.optional (rocksdb != null) rocksdb;
  CGO_ENABLED = "1";
  outputs = [
    "out"
    "instrumented"
  ];
  tags = [
    "cgo"
    "ledger"
    "!test_ledger_mock"
    "!ledger_mock"
    (if ledger_zemu then "ledger_zemu" else "!ledger_zemu")
    network
  ] ++ lib.lists.optionals (rocksdb != null) [ "rocksdb" "rocksdb_build" ];
  ldflags = ''
    -X github.com/cosmos/cosmos-sdk/version.Name=crypto-org-chain
    -X github.com/cosmos/cosmos-sdk/version.AppName=${pname}
    -X github.com/cosmos/cosmos-sdk/version.Version=${version}
    -X github.com/cosmos/cosmos-sdk/version.Commit=${rev}
    -X github.com/cosmos/cosmos-sdk/version.BuildTags=${concatStringsSep "," tags}
  '';

  instrumentedBinary = "chain-maind-inst";
  postBuild = ''
    echo "Build instrumented binary"
    go test ./cmd/chain-maind ''${tags:+-tags=${concatStringsSep "," tags}}",testbincover" ''${ldflags:+-ldflags="$ldflags"} "''${buildFlagsArray[@]}" -coverpkg=./...,github.com/cosmos/cosmos-sdk/x/... -c -o ${instrumentedBinary}
  '';
  preInstall = ''
    mkdir -p $instrumented/bin
    mv ./${instrumentedBinary} $instrumented/bin/
  '';
  preFixup = ''
    find $instrumented/bin/ -type f 2>/dev/null | xargs -r remove-references-to -t ${go} || true
  '';

  passthru = {
    # update script use the same golang version as the project
    updateScript =
      let helper = gomod2nix.override { inherit go; };
      in
      writeShellScript "${pname}-updater" ''
        exec ${helper}/bin/gomod2nix
      '';
  };

  meta = with lib; {
    description = "Official implementation of the Crypto.org blockchain protocol";
    homepage = "https://crypto.org/";
    license = licenses.asl20;
    mainProgram = "chain-maind";
  };
}
