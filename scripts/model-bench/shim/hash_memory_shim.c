/* std::__ndk1::__hash_memory(const void*, size_t) is a symbol of libc++ from
 * NDK r28 on. The sherpa-onnx v2.0.3 Android binaries were built against such
 * a libc++, and the only libc++_shared.so we have (NDK 27.3) and the phone's
 * own libc++.so (Android 14) do not export it, so the binary refuses to link
 * ("cannot locate symbol _ZNSt6__ndk113__hash_memoryEPKvm"). The function only
 * hashes bytes for unordered containers; any stable 64-bit byte hash serves.
 * FNV-1a, 64 bit. Preloaded with LD_PRELOAD, in the bench directory only. */
#include <stddef.h>
#include <stdint.h>

__attribute__((visibility("default")))
size_t _ZNSt6__ndk113__hash_memoryEPKvm(const void *p, size_t n) {
    const unsigned char *b = (const unsigned char *)p;
    uint64_t h = 1469598103934665603ULL;
    for (size_t i = 0; i < n; i++) {
        h ^= b[i];
        h *= 1099511628211ULL;
    }
    return (size_t)h;
}
