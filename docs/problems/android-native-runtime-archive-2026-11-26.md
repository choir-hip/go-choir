# The pinned llama.cpp Android archive is not a complete 16 KiB runtime source

Recorded 2026-11-26 in lane `android-native-models`, before implementation.

## Problem

The assigned route assumes the official llama.cpp `b12497` generic Android
arm64 archive carries public headers and 16 KiB-aligned libraries. The pinned
bytes do not satisfy either assumption.

## Evidence

The archive URL and SHA-256 in `scripts/model-bench/native_models.json` resolve
to 80,710,755 bytes with SHA-256
`543ef84950b07750899fff83c1b5e2718fe332c55f605867cb9b2cf42099722c`.
`tar -tzf` lists the release binaries and libraries but no `include/` directory,
`llama.h`, `mtmd.h` or `mtmd-helper.h`.

NDK `llvm-readelf -d/-lW` on the extracted `libllama.so`, `libggml.so`,
`libggml-base.so` and seven `libggml-cpu-android_*.so` variants reports
AArch64 ELF64 shared objects with `LOAD` alignment `0x1000` (4,096 bytes), not
the required `0x4000` (16,384 bytes). Their non-system dependencies
are among the staged llama/ggml libraries. `llama-nm -D --defined-only` finds
every public `llama.h` function used by the JNI slice, including model load,
context creation, decode, sampler, token conversion and pooled embeddings.

The public headers at source commit
`3574df84b776611928d9b227bf614be71fbb9849` were fetched separately and hashed.
They are public source, not a private ABI.

## Belief

The prebuilt archive can support a 4 KiB-page device probe, but it cannot by
itself prove the required 16 KiB APK native-library contract. Repacking or ZIP
alignment does not change ELF `p_align`. A source build at the same commit, or
a new official archive built with `-Wl,-z,max-page-size=16384`, is needed for
that contract.

## Decision in this lane

The build script verifies the archive and public symbols, fetches the pinned
public headers, builds the JNI wrapper with 16 KiB alignment, and **refuses to
stage any runtime unless every ELF `LOAD` segment is at least 16 KiB aligned**.
It does not silently ship the archive's 4 KiB libraries and does not add an
allow-4-KiB switch. No binary is committed.

## Resolution (2026-11-26, same lane)

The lead approved the same-commit source build the remaining-error note asked
for. `mobile/scripts/build_android_model_runtime.py` now verifies the pinned
source archive (`3574df84…`, SHA-256 `c8e5eaaa…b3db092`) and configures llama.cpp
with NDK r27d and `-DCMAKE_SHARED_LINKER_FLAGS=-Wl,-z,max-page-size=16384`
(`GGML_CPU_ARM_ARCH=armv8.2-a+dotprod+fp16`, one CPU build instead of seven
runtime variants). The official archive is no longer fetched. The build staged
four AArch64 libraries whose every LOAD segment reads `0x4000`, plus the JNI
wrapper, and `mobile/exact-patches/0002-android-package-shared-libraries.patch`
makes Exact package a bound package's `jniLibs` after checking each ELF's
alignment again. The 4 KiB archive remains the reason for the check; it is not
a fallback.

The archive's measured Qwen numbers were taken with its own runtime, so the
source build has its own device measurement in
`docs/evidence/android-a6-model-runtime-2026-11-26/`.

## Remaining error

No Exact/APK/JNI build or physical-device load was run in this lane. Whether a
particular Android 14 loader accepts the 4 KiB archive is not measured and does
not remove the 16 KiB requirement. The lead must supply a 16 KiB runtime artifact
or approve a separately reviewed same-commit source build before device proof.

## Failure list for the substrate

1. Exact accepts an absolute, parent-escaping or symlink-escaping package
   `jniLibs` path, or drops the declaration before Gradle.
2. The runtime build trusts an unverified archive/header, uses a private llama
   symbol, stages a non-AArch64 ELF, misses a dependency, or accepts a `LOAD`
   segment below 16 KiB.
3. Kotlin loads two heavyweight models, calls JNI from conflicting threads,
   hides a missing library/model, or leaves a model loaded after teardown.
4. A download escapes the model root, uses cellular without the Wi-Fi gate,
   exposes a partial final file, accepts the wrong size/hash, or reports ready
   from a missing/corrupt file.
5. Silero accepts a window other than 512 samples at 16 kHz, loses the
   six-second context/state across windows, aliases returned state, or resets
   only one of context and recurrent state.
6. Tiny Voca reports success or returns invented audio before synthesis exists.
7. The native self-test claims a measurement it did not make, or logs a prompt,
   transcript, embedding text or generated text.
