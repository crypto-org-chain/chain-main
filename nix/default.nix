{ sources ? import ./sources.nix, system ? builtins.currentSystem }:
import sources.nixpkgs {
  overlays = [
    (_: pkgs: { inherit sources; })
    (_: pkgs: {
      cosmovisor = pkgs.buildGoModule rec {
        name = "cosmovisor";
        src = ../third_party/cosmos-sdk/cosmovisor;
        subPackages = [ "./cmd/cosmovisor" ];
        vendorSha256 = sha256:1hb9yxxm41yg21hm6qbjv53i7dr7qgdpis7y93hdibjs1apxc19q;
        doCheck = false;
      };
      relayer = pkgs.buildGoModule rec {
        name = "relayer";
        src = pkgs.sources.relayer;
        subPackages = [ "." ];
        vendorSha256 = sha256:0sb50nl9nmdy4xlly3cmrb8iiyasdkm2y36s8zr9qw4prjm0ngi0;
        doCheck = false;
      };
    })
  ];
  config = { };
  inherit system;
}
