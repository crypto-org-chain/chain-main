{ stdenv, lib }:

assert stdenv.hostPlatform.isDarwin;

stdenv.mkDerivation {
  pname = "sectrust-shim";
  version = "0.1.0";

  dontUnpack = true;

  buildPhase = ''
    cat > sectrust-shim.c <<'EOF'
    #include <CoreFoundation/CoreFoundation.h>

    #define SecTrustEvaluateWithError SecTrustEvaluateWithError_not_available
    #define SecTrustCopyCertificateChain SecTrustCopyCertificateChain_not_available
    #include <Security/SecTrust.h>
    #undef SecTrustEvaluateWithError
    #undef SecTrustCopyCertificateChain

    #ifndef CF_RETURNS_RETAINED
    #define CF_RETURNS_RETAINED
    #endif

    #ifndef _Nullable
    #define _Nullable
    #endif

    static CFErrorRef createError(OSStatus status) {
      if (status == errSecSuccess) {
        return NULL;
      }
      return CFErrorCreate(kCFAllocatorDefault, kCFErrorDomainOSStatus, status, NULL);
    }

    __attribute__((weak))
    Boolean SecTrustEvaluateWithError(
      SecTrustRef trust,
      CFErrorRef _Nullable * _Nullable CF_RETURNS_RETAINED error
    ) {
      if (trust == NULL) {
        if (error != NULL) {
          *error = createError(errSecParam);
        }
        return false;
      }

      SecTrustResultType result = kSecTrustResultInvalid;
      OSStatus status = SecTrustEvaluate(trust, &result);
      if (status != errSecSuccess) {
        if (error != NULL) {
          *error = createError(status);
        }
        return false;
      }

      switch (result) {
      case kSecTrustResultProceed:
      case kSecTrustResultUnspecified:
        return true;
      default:
        if (error != NULL) {
          *error = createError(errSecNotTrusted);
        }
        return false;
      }
    }

    __attribute__((weak))
    CFArrayRef SecTrustCopyCertificateChain(SecTrustRef trust) {
      if (trust == NULL) {
        return NULL;
      }

      CFIndex count = SecTrustGetCertificateCount(trust);
      CFMutableArrayRef certificates =
        CFArrayCreateMutable(kCFAllocatorDefault, count,
                             &kCFTypeArrayCallBacks);
      if (certificates == NULL) {
        return NULL;
      }

      for (CFIndex idx = 0; idx < count; ++idx) {
        SecCertificateRef cert = SecTrustGetCertificateAtIndex(trust, idx);
        if (cert != NULL) {
          CFArrayAppendValue(certificates, cert);
        }
      }

      return certificates;
    }
    EOF

    $CC -std=c11 -Wall -Wextra -fPIC -c sectrust-shim.c -o sectrust-shim.o
    ar rcs libsectrustshim.a sectrust-shim.o
  '';

  installPhase = ''
    mkdir -p $out/lib
    cp libsectrustshim.a $out/lib/
  '';

  meta = with lib; {
    description =
      "Compatibility shim implementing SecTrustEvaluateWithError for older macOS releases";
    license = licenses.asl20;
    platforms = platforms.darwin;
  };
}
