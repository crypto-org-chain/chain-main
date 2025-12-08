{
  lib,
  stdenv,
  fetchFromGitHub,
  fetchpatch,
  cmake,
  ninja,
  bzip2,
  lz4,
  snappy,
  zlib,
  zstd,
  windows,
  # only enable jemalloc for non-windows platforms
  # see: https://github.com/NixOS/nixpkgs/issues/216479
  enableJemalloc ? !stdenv.hostPlatform.isWindows && !stdenv.hostPlatform.isStatic,
  jemalloc,
  enableLite ? false,
  enableShared ? !stdenv.hostPlatform.isStatic && !stdenv.hostPlatform.isMinGW,
  sse42Support ? stdenv.hostPlatform.sse4_2Support,
}:

let
  publishLz4Flag = "-DLZ4_PUBLISH_STATIC_FUNCTIONS=1";
  ensureList = val: if builtins.isList val then val else [ val ];
  appendPublishFlag =
    attr: old:
    let
      existing = if builtins.hasAttr attr old then ensureList (builtins.getAttr attr old) else [ ];
    in
    lib.concatStringsSep " " (existing ++ [ publishLz4Flag ]);
  lz4Published =
    if stdenv.hostPlatform.isMinGW then
      lz4.overrideAttrs (old: {
        # LZ4_PUBLISH_STATIC_FUNCTIONS makes the streaming API functions visible in the DLL
        # This is needed for RocksDB which uses LZ4's streaming compression API
        NIX_CFLAGS_COMPILE = appendPublishFlag "NIX_CFLAGS_COMPILE" old;
        NIX_CFLAGS_COMPILE_FOR_TARGET = appendPublishFlag "NIX_CFLAGS_COMPILE_FOR_TARGET" old;
        makeFlags = (old.makeFlags or [ ]) ++ [ "CFLAGS=${publishLz4Flag}" ];
      })
    else
      lz4;
in

stdenv.mkDerivation rec {
  pname = "rocksdb";
  version = "9.11.2";

  src = fetchFromGitHub {
    owner = "facebook";
    repo = pname;
    rev = "v${version}";
    sha256 = "sha256-D/FZJw1zwDXvCRHxCxyNxarHlDi5xtt8MddUOr4Pv2c=";
  };

  nativeBuildInputs = [
    cmake
    ninja
  ];

  propagatedBuildInputs = [
    bzip2
    lz4Published
    snappy
    zlib
    zstd
  ];

  buildInputs =
    lib.optional enableJemalloc jemalloc
    ++ lib.optional stdenv.hostPlatform.isMinGW windows.mingw_w64_pthreads;

  outputs = [ "out" ] ++ lib.optional (!stdenv.hostPlatform.isMinGW) "tools";

  NIX_CFLAGS_COMPILE =
    lib.optionals stdenv.cc.isGNU [
      "-Wno-error=deprecated-copy"
      "-Wno-error=pessimizing-move"
      # Needed with GCC 12
      "-Wno-error=format-truncation"
      "-Wno-error=maybe-uninitialized"
    ]
    ++ lib.optionals stdenv.cc.isClang [
      "-Wno-error=unused-private-field"
      "-Wno-error=nontrivial-memcall" # new clang diagnostic on 25.11 toolchain
      "-faligned-allocation"
    ]
    ++ lib.optionals stdenv.hostPlatform.isMinGW [
      # Match the LZ4_PUBLISH_STATIC_FUNCTIONS define we set for lz4Published
      # This tells RocksDB to expect published (exported) LZ4 functions instead of inline ones
      publishLz4Flag
    ];

  NIX_LDFLAGS = lib.optionalString stdenv.hostPlatform.isMinGW "-llz4 -lsnappy -lz -lbz2 -lzstd";

  preConfigure = lib.optionalString stdenv.hostPlatform.isMinGW ''
    export LDFLAGS="$LDFLAGS -L${lib.getLib lz4Published}/lib -L${lib.getLib snappy}/lib -L${lib.getLib zlib}/lib -L${lib.getLib bzip2}/lib -L${lib.getLib zstd}/lib"
  '';

  cmakeFlags = [
    "-DPORTABLE=1"
    "-DWITH_JEMALLOC=${if enableJemalloc then "1" else "0"}"
    "-DWITH_JNI=0"
    "-DWITH_BENCHMARK_TOOLS=0"
    "-DWITH_TESTS=${if stdenv.hostPlatform.isMinGW then "0" else "1"}"
    "-DWITH_TOOLS=0"
    "-DWITH_CORE_TOOLS=${if stdenv.hostPlatform.isMinGW then "0" else "1"}"
    "-DWITH_BZ2=1"
    "-DWITH_LZ4=1"
    "-DWITH_SNAPPY=1"
    "-DWITH_ZLIB=1"
    "-DWITH_ZSTD=1"
    "-DWITH_GFLAGS=0"
    "-DUSE_RTTI=1"
    "-DROCKSDB_INSTALL_ON_WINDOWS=YES" # harmless elsewhere
    (lib.optional sse42Support "-DFORCE_SSE42=1")
    (lib.optional enableLite "-DROCKSDB_LITE=1")
    "-DFAIL_ON_WARNINGS=${if stdenv.hostPlatform.isMinGW then "NO" else "YES"}"
  ]
  ++ lib.optionals stdenv.hostPlatform.isWindows [
    "-DCMAKE_C_FLAGS=-U_WIN32_WINNT -D_WIN32_WINNT=0x0602"
    "-DCMAKE_CXX_FLAGS=-U_WIN32_WINNT -D_WIN32_WINNT=0x0602"
  ]
  ++ lib.optional (!enableShared) "-DROCKSDB_BUILD_SHARED=0"
  ++ lib.optionals stdenv.hostPlatform.isMinGW [
    "-DLZ4_INCLUDE_DIR=${lib.getDev lz4Published}/include"
    "-DLZ4_LIBRARIES=${lib.getLib lz4Published}/lib/liblz4.dll.a"
    "-DSNAPPY_INCLUDE_DIR=${lib.getDev snappy}/include"
    "-DSNAPPY_LIBRARIES=${lib.getLib snappy}/lib/libsnappy.dll.a"
    "-DZLIB_INCLUDE_DIR=${lib.getDev zlib}/include"
    "-DZLIB_LIBRARY=${lib.getLib zlib}/lib/libz.dll.a"
    "-Dbzip2_INCLUDE_DIR=${lib.getDev bzip2}/include"
    "-Dbzip2_LIBRARIES=${lib.getLib bzip2}/lib/libbz2.dll.a"
    "-DZSTD_INCLUDE_DIR=${lib.getDev zstd}/include"
    "-DZSTD_LIBRARY=${lib.getLib zstd}/lib/libzstd.dll.a"
  ];

  # otherwise "cc1: error: -Wformat-security ignored without -Wformat [-Werror=format-security]"
  hardeningDisable = lib.optional stdenv.hostPlatform.isWindows "format";

  preInstall =
    lib.optionalString (!stdenv.hostPlatform.isMinGW) ''
      mkdir -p $tools/bin
      cp tools/{ldb,sst_dump}${stdenv.hostPlatform.extensions.executable} $tools/bin/
    ''
    + lib.optionalString stdenv.isDarwin ''
      ls -1 $tools/bin/* | xargs -I{} ${stdenv.cc.bintools.targetPrefix}install_name_tool -change "@rpath/librocksdb.${lib.versions.major version}.dylib" $out/lib/librocksdb.dylib {}
    ''
    + lib.optionalString (stdenv.isLinux && enableShared) ''
      ls -1 $tools/bin/* | xargs -I{} patchelf --set-rpath $out/lib:${stdenv.cc.cc.lib}/lib {}
    '';

  # Old version doesn't ship the .pc file, new version puts wrong paths in there.
  postFixup = ''
    if [ -f "$out"/lib/pkgconfig/rocksdb.pc ]; then
      substituteInPlace "$out"/lib/pkgconfig/rocksdb.pc \
        --replace '="''${prefix}//' '="/'
    fi
  ''
  + lib.optionalString stdenv.isDarwin ''
    ${stdenv.cc.targetPrefix}install_name_tool -change "@rpath/libsnappy.1.dylib" "${snappy}/lib/libsnappy.1.dylib" $out/lib/librocksdb.dylib
    ${stdenv.cc.targetPrefix}install_name_tool -change "@rpath/librocksdb.${lib.versions.major version}.dylib" "$out/lib/librocksdb.${lib.versions.major version}.dylib" $out/lib/librocksdb.dylib
  '';

  meta = with lib; {
    homepage = "https://rocksdb.org";
    description = "A library that provides an embeddable, persistent key-value store for fast storage";
    changelog = "https://github.com/facebook/rocksdb/raw/v${version}/HISTORY.md";
    license = licenses.asl20;
    platforms = platforms.all;
    maintainers = with maintainers; [
      adev
      magenbluten
    ];
  };
}
