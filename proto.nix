{ pkgs ? import ./nix { }, commit ? "" }:
with pkgs;
pkgs.mkShell {
  buildInputs = [
    buf
    git
  ];
  shellHook = ''                                                                                       
  cd ./pystarport
  ./new-convert.sh                                                                                          '';
}
