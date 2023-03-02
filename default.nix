{ lib
, stdenv
, buildGoApplication
, nix-gitignore
, writeShellScript
, buildPackages
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
  version = "4.2.4";
  go = buildPackages.go_1_20;
  src = lib.cleanSourceWith {
    name = "src";
    src = lib.sourceByRegex ./. src_regexes;
  };
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/chain-maind" ];
  buildFlags = "-cover";
  buildInputs = lib.lists.optional (rocksdb != null) rocksdb;
  CGO_ENABLED = "1";
  CGO_LDFLAGS =
    if stdenv.hostPlatform.isWindows
    then "-lrocksdb-shared"
    else "-lrocksdb -pthread -lstdc++ -ldl";
  tags = [
    "cgo"
    "ledger"
    "!test_ledger_mock"
    "!ledger_mock"
    (if ledger_zemu then "ledger_zemu" else "!ledger_zemu")
    network
  ] ++ lib.lists.optionals (rocksdb != null) [ "rocksdb" "grocksdb_no_link" ];
  ldflags = ''
    -X github.com/cosmos/cosmos-sdk/version.Name=crypto-org-chain
    -X github.com/cosmos/cosmos-sdk/version.AppName=${pname}
    -X github.com/cosmos/cosmos-sdk/version.Version=${version}
    -X github.com/cosmos/cosmos-sdk/version.Commit=${rev}
    -X github.com/cosmos/cosmos-sdk/version.BuildTags=${concatStringsSep "," tags}
  '';
  postFixup = lib.optionalString stdenv.isDarwin ''
    ${stdenv.cc.targetPrefix}install_name_tool -change "@rpath/librocksdb.7.dylib" "${rocksdb}/lib/librocksdb.dylib" $out/bin/chain-maind
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

  doCheck = false;
  meta = with lib; {
    description = "Official implementation of the Crypto.org blockchain protocol";
    homepage = "https://crypto.org/";
    license = licenses.asl20;
    mainProgram = "chain-maind" + stdenv.hostPlatform.extensions.executable;
    platforms = platforms.all;
  };
}
