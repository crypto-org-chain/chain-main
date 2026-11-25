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
  go_1_26 = super.go_1_26.overrideAttrs (old: rec {
    version = "1.26.11";
    src = final.fetchurl {
      url = "https://go.dev/dl/go${version}.src.tar.gz";
      hash = "sha256-1EEBsP8n7SpMda1GwXucdDWSCeLrLKfJ32y0/baOGWk=";
    };
    # nixpkgs forces external linking for cross builds. Since Go 1.26 that breaks Windows targets:
    # the linker exports a runtime/cgo symbol that the pure Go tools do not link.
    buildPhase =
      let
        forceExternalLink = "export GO_EXTLINK_ENABLED=1";
      in
      if final.stdenv.targetPlatform.isWindows then
        assert final.lib.assertMsg (final.lib.hasInfix forceExternalLink old.buildPhase)
          "nixpkgs go buildPhase no longer contains '${forceExternalLink}'; drop this override";
        builtins.replaceStrings [ forceExternalLink ] [ "" ] old.buildPhase
      else
        old.buildPhase;
  });
  rocksdb = final.callPackage ./rocksdb.nix { };
  golangci-lint = final.callPackage ./golangci-lint.nix { };
  sectrustShim =
    if final.stdenv.targetPlatform.isDarwin then final.callPackage ./sectrust-shim.nix { } else null;
}
