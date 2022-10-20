{ poetry2nix, python39Packages, python39, lib }:
poetry2nix.mkPoetryEnv {
  python = python39;
  projectDir = ../integration_tests;
  overrides = poetry2nix.overrides.withDefaults (lib.composeManyExtensions [
    (self: super:
      let
        buildSystems = {
          pyparsing = [ "flit-core" ];
          hdwallets = [ "poetry" ];
          pystarport = [ "poetry" ];
          durations = [ "setuptools" ];
          multitail2 = [ "setuptools" ];
          pytest-github-actions-annotate-failures = [ "setuptools" ];
          flake8-black = [ "setuptools" ];
        };
      in
      lib.mapAttrs
        (attr: systems: super.${attr}.overridePythonAttrs
          (old: {
            nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ map (a: self.${a}) systems;
          }))
        buildSystems
    )
    (self: super: {
      platformdirs = python39Packages.platformdirs;
    })
  ]);
}
