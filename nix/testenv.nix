{ poetry2nix, python39Packages, python39 }:
poetry2nix.mkPoetryEnv {
  python = python39;
  projectDir = ../integration_tests;
  overrides = poetry2nix.overrides.withDefaults (self: super: {
    pyparsing = super.pyparsing.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.flit-core ];
      }
    );

    platformdirs = python39Packages.platformdirs;

    hdwallets = super.hdwallets.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.poetry ];
      }
    );
    pystarport = super.pystarport.overridePythonAttrs (
      old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [ self.poetry ];
      }
    );
  });
}
