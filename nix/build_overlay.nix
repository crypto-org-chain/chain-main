# some basic overlays necessary for the build
final: super: {
  rocksdb = final.callPackage ./rocksdb.nix { };
}
