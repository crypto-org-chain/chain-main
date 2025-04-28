{
  lib,
  stdenv,
  buildGoApplication,
  writeShellScript,
  buildPackages,
  coverage ? false, # https://tip.golang.org/doc/go1.20#cover
  gomod2nix,
  rocksdb ? null,
  network ? "mainnet", # mainnet|testnet
  rev ? "dirty",
  ledger_zemu ? false,
  static ? stdenv.hostPlatform.isStatic,
  nativeByteOrder ? true, # nativeByteOrder mode will panic on big endian machines
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
  version = "5.0.2";
  go = buildPackages.go_1_23;
  src = lib.cleanSourceWith {
    name = "src";
    src = lib.sourceByRegex ./. src_regexes;
  };
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/chain-maind" ];
  buildFlags = lib.optionalString coverage "-cover";
  buildInputs = lib.lists.optional (rocksdb != null) rocksdb;
  CGO_ENABLED = "1";
  CGO_LDFLAGS = lib.optionalString (rocksdb != null) (
    if static then
      "-lrocksdb -pthread -lstdc++ -ldl -lzstd -lsnappy -llz4 -lbz2 -lz"
    else if stdenv.hostPlatform.isWindows then
      "-lrocksdb-shared"
    else
      "-lrocksdb -pthread -lstdc++ -ldl"
  );
  tags =
    [
      "cgo"
      "ledger"
      "!test_ledger_mock"
      "!ledger_mock"
      (if ledger_zemu then "ledger_zemu" else "!ledger_zemu")
      network
    ]
    ++ lib.optionals (rocksdb != null) [
      "rocksdb"
      "grocksdb_no_link"
    ]
    ++ lib.optionals nativeByteOrder [ "nativebyteorder" ];
  ldflags = ''
    -X github.com/cosmos/cosmos-sdk/version.Name=crypto-org-chain
    -X github.com/cosmos/cosmos-sdk/version.AppName=${pname}
    -X github.com/cosmos/cosmos-sdk/version.Version=${version}
    -X github.com/cosmos/cosmos-sdk/version.Commit=${rev}
    -X github.com/cosmos/cosmos-sdk/version.BuildTags=${concatStringsSep "," tags}
  '';
  postFixup = lib.optionalString (stdenv.isDarwin && rocksdb != null) ''
    ${stdenv.cc.bintools.targetPrefix}install_name_tool -change "@rpath/librocksdb.8.dylib" "${rocksdb}/lib/librocksdb.dylib" $out/bin/chain-maind
  '';
  passthru = {
    # update script use the same golang version as the project
    updateScript =
      let
        helper = gomod2nix.override { inherit go; };
      in
      writeShellScript "${pname}-updater" ''
        exec ${helper}/bin/gomod2nix
      '';
  };

  doCheck = false;
  meta = with lib; {
    description = "Official implementation of the Cronos.org blockchain protocol";
    homepage = "https://cronos.org/";
    license = licenses.asl20;
    mainProgram = "chain-maind" + stdenv.hostPlatform.extensions.executable;
    platforms = platforms.all;
  };
}
