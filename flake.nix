{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/master";
    flake-utils.url = "github:numtide/flake-utils";
    nix-bundle-exe = {
      url = "github:3noch/nix-bundle-exe";
      flake = false;
    };
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.utils.follows = "flake-utils";
    };
  };

  outputs = { self, nixpkgs, nix-bundle-exe, gomod2nix, flake-utils }:
    let
      rev = self.shortRev or "dirty";
      mkApp = drv: {
        type = "app";
        program = "${drv}/bin/${drv.meta.mainProgram}";
      };
    in
    (flake-utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [
              gomod2nix.overlays.default
              self.overlay
            ];
            config = { };
          };
        in
        rec {
          packages = pkgs.chain-main-matrix;
          apps = {
            chain-maind = mkApp packages.chain-maind;
            chain-maind-testnet = mkApp packages.chain-maind-testnet;
            update-gomod2nix = {
              type = "app";
              program = "${packages.chain-maind.updateScript}";
            };
          };
          defaultPackage = packages.chain-maind;
          defaultApp = apps.chain-maind;
          devShells = {
            chain-maind = pkgs.mkShell {
              buildInputs = with pkgs; [
                go_1_20
                rocksdb
              ];
            };
          };
          devShell = devShells.chain-maind;
          legacyPackages = pkgs;
        }
      )
    ) // {
      overlay = final: prev: {
        bundle-exe = final.pkgsBuildBuild.callPackage nix-bundle-exe { };
        # make-tarball don't follow symbolic links to avoid duplicate file, the bundle should have no external references.
        # reset the ownership and permissions to make the extract result more normal.
        make-tarball = drv: final.runCommand "tarball-${drv.name}"
          {
            nativeBuildInputs = with final.buildPackages; [ gnutar gzip ];
          } ''
          tar cfv - -C "${drv}" \
            --owner=0 --group=0 --mode=u+rw,uga+r --hard-dereference . \
            | gzip -9 > $out
        '';
        bundle-win-exe = drv: final.callPackage ./nix/bundle-win-exe.nix { cronosd = drv; };
        # only enable jemalloc for non-windows platforms
        # see: https://github.com/NixOS/nixpkgs/issues/216479
        rocksdb = final.callPackage ./nix/rocksdb.nix { enableJemalloc = !final.stdenv.hostPlatform.isWindows; };
      } // (with final;
        let
          matrix = lib.cartesianProductOfSets {
            network = [ "mainnet" "testnet" ];
            pkgtype = [
              "nix" # normal nix package
              "bundle" # relocatable bundled package
              "tarball" # tarball of the bundle, for distribution and checksum
            ];
          };
          binaries = builtins.listToAttrs (builtins.map
            ({ network, pkgtype }: {
              name = builtins.concatStringsSep "-" (
                [ "chain-maind" ] ++
                lib.optional (network != "mainnet") network ++
                lib.optional (pkgtype != "nix") pkgtype
              );
              value =
                let
                  chain-maind = callPackage ./. {
                    inherit rev network;
                  };
                  bundle = bundle-exe chain-maind;
                in
                if pkgtype == "bundle" then
                  bundle
                else if pkgtype == "tarball" then
                  make-tarball bundle
                else
                  chain-maind;
            })
            matrix
          );
        in
        {
          chain-main-matrix = binaries;
        }
      );
    };
}
