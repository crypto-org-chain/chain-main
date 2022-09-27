{ system ? "x86_64-linux", pkgs ? import ./nix { inherit system; } }:
let
  self = import ./. { inherit pkgs; };
  chain-maind = pkgs.callPackage ./. {};
in
{
  chaindImage =
    pkgs.dockerTools.buildLayeredImage {
      name = "crypto-org-chain/chain-maind";
      config.Entrypoint = [ "${chain-maind}/bin/chain-maind" ];
    };

  pystarportImage =
    pkgs.dockerTools.buildLayeredImage {
      name = "crypto-org-chain/chain-main-pystarport";
      config.Entrypoint = [ "${pkgs.test-env}/bin/pystarport" ];
    };
}
