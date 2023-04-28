# some basic overlays nessesary for the build
final: super: {
  # include the fix: https://github.com/NixOS/nixpkgs/pull/211515
  zstd = final.callPackage ./zstd.nix {
    cmake = final.buildPackages.cmakeMinimal;
  };
  rocksdb = final.callPackage ./rocksdb.nix { };
  go_1_20 = super.go_1_20.overrideAttrs (prev: rec {
    version = "1.20.3";
    src = final.fetchurl {
      url = "https://go.dev/dl/go${version}.src.tar.gz";
      hash = "sha256-5Ee0mM3lAhXE92GeUSSw/E4l+10W6kcnHEfyeOeqdjo=";
    };
  });
}
