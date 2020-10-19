{ system ? "x86_64-linux", pkgs ? import <nixpkgs> { inherit system; } }:
let
  chaind = import ./. { inherit pkgs; };
  pystarport = import ./pystarport { inherit pkgs chaind; };
in
{
  chaindImage =
    pkgs.dockerTools.buildLayeredImage {
      name = "crypto-com/chain-maind";
      config.Entrypoint = [ "${chaind}/bin/chain-maind" ];
    };

  pystarportImage =
    pkgs.dockerTools.buildLayeredImage {
      name = "crypto-com/chain-main-pystarport";
      config.Entrypoint = [ "${pystarport}/bin/pystarport" ];
    };
}
