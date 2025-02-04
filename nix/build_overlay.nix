# some basic overlays necessary for the build
final: super: {
  go_1_23 = super.go_1_23.overrideAttrs (old: rec {
    version = "1.23.4";
    src = final.fetchurl {
      url = "https://go.dev/dl/go${version}.src.tar.gz";
      hash = "sha256-rTRaxCHpCBQpOpaZzKGd1SOCUcP2h5gLvK4oSVsmNTE=";
    };
  });
  rocksdb = final.callPackage ./rocksdb.nix { };
  golangci-lint = final.callPackage ./golangci-lint.nix { };
}
