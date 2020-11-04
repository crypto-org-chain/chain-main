{ pkgs ? import <nixpkgs> { }, commit ? "", network ? "mainnet" }:
with pkgs;
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
    "^go\.mod$"
    "^go\.sum$"
  ];
in
buildGoModule rec {
  pname = "chain-maind";
  version = "0.0.1";
  src = lib.cleanSourceWith {
    name = "src";
    src = lib.sourceByRegex ./. src_regexes;
  };
  subPackages = [ "cmd/chain-maind" ];
  vendorSha256 = sha256:0jldi7scw1k114sxpqyh3ljc9qbrp4sax2bcg8v13bya4ji8m7qh;
  outputs = [ "out" "instrumented" ];
  buildFlags = lib.optionalString (network == "testnet") "-tags testnet";
  buildFlagsArray = ''
    -ldflags=
    -X github.com/cosmos/cosmos-sdk/version.Name=crypto-com-chain
    -X github.com/cosmos/cosmos-sdk/version.AppName=${pname}
    -X github.com/cosmos/cosmos-sdk/version.Version=${version}
    -X github.com/cosmos/cosmos-sdk/version.Commit=${commit}
  '';

  instrumentedBinary = "chain-maind-inst";
  # FIXME remove the "-w -s" ldflags, https://github.com/golang/go/issues/40974
  postBuild = ''
    echo "Build instrumented binary"
    go test ./cmd/chain-maind --tags testbincover "${buildFlagsArray} -w -s" -coverpkg=./...,github.com/cosmos/cosmos-sdk/x/... -c -o ${instrumentedBinary}
  '';
  postInstall = ''
    mkdir -p $instrumented/bin
    mv ./${instrumentedBinary} $instrumented/bin/
  '';
  preFixup = ''
    find $instrumented/bin/ -type f 2>/dev/null | xargs -r remove-references-to -t ${go} || true
  '';

}
