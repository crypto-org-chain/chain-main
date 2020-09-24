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
  vendorSha256 = sha256:0iv7xh5rhk20jaabrrggj69f3ismw6rv9dyvkiiiwj46j9nkimk4;
}
