{ sources ? import ./sources.nix, system ? builtins.currentSystem }:
import sources.nixpkgs {
  overlays = [
    (_: pkgs: { inherit sources; })
    (_: pkgs: {
      cosmovisor = pkgs.buildGoModule rec {
        name = "cosmovisor";
        src = pkgs.sources.cosmos-sdk;
        sourceRoot = "cosmos-sdk-src/cosmovisor";
        vendorSha256 = sha256:1mv0id290b4h8wrzq5z5n1bsq5im8glhlb8c5j7lrky30mikzwik;
        doCheck = false;
      };
    })
    (_: pkgs: {
      poetry2nix = import pkgs.sources.poetry2nix { inherit (pkgs) pkgs poetry; };
    })
  ];
  config = { };
  inherit system;
}
