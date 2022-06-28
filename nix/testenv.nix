{ poetry2nix }:
poetry2nix.mkPoetryEnv {
  projectDir = ../integration_tests;
  overrides = poetry2nix.overrides.withDefaults (self: super: {
    pyparsing = super.pyparsing.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.flit-core ];
      }
    );

    hdwallets = super.hdwallets.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.poetry ];
      }
    );
  });
}
