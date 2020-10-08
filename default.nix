{ pkgs ? import <nixpkgs> {} }:
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
  pname = "chain-main";
  version = "0.0.1";
  src = lib.cleanSourceWith {
    name = "src";
    src = lib.sourceByRegex ./. src_regexes;
  };
  subPackages = [ "cmd/chain-maind" ];
  vendorSha256 = sha256:05w1v3smx5j9i647bn7rw5fbs04sa356v9qp8pis8aajw6y5pr3l;
  outputs = [ "out" "instrumented"];

  instrumentedBinary = "chain-maind-inst";
  # FIXME remove the ldflags, https://github.com/golang/go/issues/40974
  postBuild = ''
    go test ./cmd/chain-maind --tags testbincover -coverpkg=./...,github.com/cosmos/cosmos-sdk/x/... -ldflags "-w -s" -c -o ${instrumentedBinary}
  '';
  postInstall = ''
    mkdir -p $instrumented/bin
    mv ./${instrumentedBinary} $instrumented/bin/
  '';
  preFixup = ''
    find $instrumented/bin/ -type f 2>/dev/null | xargs -r remove-references-to -t ${go} || true
  '';

}
