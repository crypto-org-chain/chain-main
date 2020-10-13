{ pkgs ? import <nixpkgs> {}, commit ? "" }:
with pkgs;
let
  src_regexes = [
    "^x$"
    "^x/.*"
    "^app$"
    "^app/.*"
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
  vendorSha256 = sha256:14nbdviafjpx8a0vc7839nkrs441r9aw4mxjhb6f4213n72kj3zn;
  outputs = [ "out" "instrumented" ];
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
