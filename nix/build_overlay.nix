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
  go_1_25 = super.go_1_24.overrideAttrs (old: rec {
    version = "1.25.0";
    src = final.fetchurl {
      url = "https://go.dev/dl/go${version}.src.tar.gz";
      hash = "sha256-S9AekSlyB7+kUOpA1NWpOxtTGl5DhHOyoG4Y4HciciU=";
    };
    patches = builtins.filter (
      patch:
      let
        patchName = builtins.baseNameOf (builtins.toString patch);
      in
      !final.lib.hasSuffix "-iana-etc-1.17.patch" patchName
    ) (old.patches or [ ]);
    postPatch = (old.postPatch or "") + ''
      substituteInPlace src/net/lookup_unix.go \
        --replace 'open("/etc/protocols")' 'open("${final.iana-etc}/etc/protocols")'
      substituteInPlace src/net/port_unix.go \
        --replace 'open("/etc/services")' 'open("${final.iana-etc}/etc/services")'
    '';
  });
  rocksdb = final.callPackage ./rocksdb.nix { };
  golangci-lint = final.callPackage ./golangci-lint.nix { };
  sectrustShim =
    if final.stdenv.targetPlatform.isDarwin then final.callPackage ./sectrust-shim.nix { } else null;
}
