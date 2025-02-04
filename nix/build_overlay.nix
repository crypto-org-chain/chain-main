# some basic overlays nessesary for the build
final: super: { rocksdb = final.callPackage ./rocksdb.nix { }; }
