{ poetry2nix, python3Packages }:
poetry2nix.mkPoetryEnv {
  projectDir = ../integration_tests;
  overrides = poetry2nix.overrides.withDefaults (self: super: {
    pyparsing = super.pyparsing.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.flit-core ];
      }
    );

    platformdirs = python3Packages.platformdirs;

    hdwallets = super.hdwallets.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.poetry ];
      }
    );
  });
}
