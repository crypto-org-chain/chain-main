{
  poetry2nix,
  python311,
  lib,
}:
poetry2nix.mkPoetryEnv {
  python = python311;
  projectDir = ../integration_tests;
  overrides = poetry2nix.overrides.withDefaults (
    self: super:
    let
      buildSystems = {
        pystarport = [ "poetry-core" ];
        hdwallets = [ "poetry" ];
        durations = [ "setuptools" ];
        multitail2 = [ "setuptools" ];
        pytest-github-actions-annotate-failures = [ "setuptools" ];
        flake8-black = [ "setuptools" ];
        flake8-isort = [ "hatchling" ];
        docker = [
          "hatchling"
          "hatch-vcs"
        ];
        click = [ "flit-core" ];
        isort = [ "hatchling" ];
      };
    in
    lib.mapAttrs (
      attr: systems:
      super.${attr}.overridePythonAttrs (old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ map (a: self.${a}) systems;
      })
    ) buildSystems
  );
}
