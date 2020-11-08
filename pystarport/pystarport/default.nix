{ pkgs ? import ../../nix { }, commit ? "" }:
with pkgs;
pkgs.mkShell {
  buildInputs = [
    python38
    protobuf3_13
    grpc
    git
  ];
  shellHook = ''                                                                                       
  ./convert.sh                                                                                          '';
}
