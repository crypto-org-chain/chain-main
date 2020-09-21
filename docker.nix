{ system ? "x86_64-linux" }:

let
  pkgs = import <nixpkgs> { inherit system; };

  chaind = import ./default.nix { inherit pkgs; };
  pystarport = import ./pystarport/default.nix { inherit pkgs; };

  chaindImage =
    pkgs.dockerTools.buildImage {
      name = "crypto-com/chain-maind";
      tag = chaind.version;

      contents = [ chaind ];

      config = {
        Cmd = [ "${chaind}/bin/chain-maind" ];
      };
    };

  pystarportImage =
    pkgs.dockerTools.buildImage {
      name = "crypto-com/chain-main-pystarport";
      tag = pystarport.version;

      contents = [ chaind pystarport ];

      config = {
        Cmd = [ "${pystarport}/bin/pystarport" ];
      };
    };

in { inherit chaindImage pystarportImage; }
