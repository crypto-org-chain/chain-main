{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/22.05";
    flake-utils.url = "github:numtide/flake-utils";
    nix-bundle-exe = {
      url = "github:3noch/nix-bundle-exe";
      flake = false;
    };
    gomod2nix = {
      url = "github:tweag/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.utils.follows = "flake-utils";
    };
    rocksdb-src = {
      url = "github:facebook/rocksdb/v6.29.5";
      flake = false;
    };
    libwasmvm-src = {
      url = "github:CosmWasm/wasmvm/v1.0.0";
      flake = false;
    };
  };

  outputs = { self, nixpkgs, nix-bundle-exe, gomod2nix, flake-utils, rocksdb-src, libwasmvm-src }:
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
          };
          defaultPackage = packages.chain-maind;
          defaultApp = apps.chain-maind;
          devShells = {
            chain-maind = pkgs.mkShell {
              buildInputs = with pkgs; [
                go_1_17
                rocksdb
                libwasmvm
              ];
            };
          };
          devShell = devShells.chain-maind;
        }
      )
    ) // {
      overlay = final: prev: {
        bundle-exe = import nix-bundle-exe { pkgs = final; };
        # make-tarball don't follow symbolic links to avoid duplicate file, the bundle should have no external references.
        # reset the ownership and permissions to make the extract result more normal.
        make-tarball = drv: with final; runCommand drv.name { } ''
          "${gnutar}/bin/tar" cfv - -C ${drv} \
            --owner=0 --group=0 --mode=u+rw,uga+r --hard-dereference . \
            | "${gzip}/bin/gzip" -9 > $out
        '';
        rocksdb = (prev.rocksdb.overrideAttrs (old: rec {
          pname = "rocksdb";
          version = "6.29.5";
          src = rocksdb-src;
        })).override { enableJemalloc = true; };
        libwasmvm = final.rustPlatform.buildRustPackage rec {
          name = "libwasmvm";
          src = libwasmvm-src + "/libwasmvm";
          cargoDepsName = "vendor"; # use a static name to avoid rebuild when name changed
          cargoSha256 = "sha256-m3CtXHAkjNR7t7zie9FWK4k5xvr6/O2BfGQYi+foxCc=";
          doCheck = false;
        };
      } // (with final;
        let
          matrix = lib.cartesianProductOfSets {
            db_backend = [ "goleveldb" "rocksdb" ];
            network = [ "mainnet" "testnet" ];
            pkgtype = [
              "nix" # normal nix package
              "bundle" # relocatable bundled package
              "tarball" # tarball of the bundle, for distribution and checksum
            ];
          };
          binaries = builtins.listToAttrs (builtins.map
            ({ db_backend, network, pkgtype }: {
              name = builtins.concatStringsSep "-" (
                [ "chain-maind" ] ++
                lib.optional (network != "mainnet") network ++
                lib.optional (db_backend != "rocksdb") db_backend ++
                lib.optional (pkgtype != "nix") pkgtype
              );
              value =
                let
                  chain-maind = callPackage ./. {
                    inherit rev db_backend network;
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
