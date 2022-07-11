final: prev: {
  go_1_18 = prev.go_1_18.override {
    inherit (final.darwin.apple_sdk_11_0.frameworks) Security Foundation;
    inherit (final.darwin.apple_sdk_11_0) stdenv;
    xcbuild = prev.xcbuild.override {
      inherit (final.darwin.apple_sdk_11_0) stdenv;
    };
  };
}
