{
  poetry2nix,
  python311,
  lib,
}:
poetry2nix.mkPoetryEnv {
  python = python311;
  projectDir = ../integration_tests;
  preferWheels = true;
  overrides = poetry2nix.overrides.withDefaults (
    lib.composeManyExtensions [
      (
        self: super:
        let
          buildSystems = {
            pyparsing = [ "flit-core" ];
            hdwallets = [ "poetry" ];
            pystarport = [ "poetry" ];
            durations = [ "setuptools" ];
            multitail2 = [ "setuptools" ];
            pytest-github-actions-annotate-failures = [ "setuptools" ];
            flake8-black = [ "setuptools" ];
            isort = [ "hatchling" ];
            flake8-isort = [ "hatchling" ];
          };
        in
        lib.mapAttrs (
          attr: systems:
          super.${attr}.overridePythonAttrs (old: {
            nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ map (a: self.${a}) systems;
          })
        ) buildSystems
      )
    ]
  );
}
