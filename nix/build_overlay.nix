# some basic overlays necessary for the build
final: super:
let
  replaceLast =
    newVal: l:
    let
      len = builtins.length l;
    in
    if len == 0 then [ ] else final.lib.lists.take (len - 1) l ++ [ newVal ];
in
{
  go_1_23 = super.go_1_23.overrideAttrs (old: rec {
    version = "1.23.6";
    src = final.fetchurl {
      url = "https://go.dev/dl/go${version}.src.tar.gz";
      hash = "sha256-A5xbBOZSedrO7opvcecL0Fz1uAF4K293xuGeLtBREiI=";
    };
  });
  rocksdb = final.callPackage ./rocksdb.nix { };
  golangci-lint = final.callPackage ./golangci-lint.nix { };
}
