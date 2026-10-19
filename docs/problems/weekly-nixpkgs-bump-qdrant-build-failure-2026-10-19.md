# Weekly nixpkgs bump: qdrant 1.18.2 does not compile (2026-10-19)

Mutation class: orange (host closure composition; no runtime behavior change
intended). Protected surfaces: none directly; the change only decides which
qdrant build enters the Node B closure when the weekly bump is merged.

## Evidence

- Workflow "Weekly nixpkgs Bump" (`.github/workflows/flake-bump.yml`), run
  39241253148, 2026-10-19. The `open-pr` job pushed PR #74
  (`patch/weekly-nixpkgs`, head `44368ebdd`), moving `nixpkgs` from
  `ec84054698e3875e23d6057a10283eb9dfa41f1b` (nixos-unstable 2026-07-01, the
  deployed lock on `main`) to `fe13242281a67641c0102cdb1087f0ebe586c7df`
  (nixos-unstable 2026-10-15, still the current channel head on 2026-10-19).
- The `build` job's `nix build ...go-choir-b...toplevel .#guest-image` failed:

  ```
  qdrant> Compiling quantization v0.1.0 (/build/source/lib/quantization)
  qdrant> error: intrinsic signature mismatch for `llvm.x86.avx512.vpdpbusd.512`:
          expected signature `<16 x i32> (<16 x i32>, <16 x i32>, <16 x i32>)`,
          found `<16 x i32> (<16 x i32>, <64 x i8>, <64 x i8>)`
   --> .../stdarch/crates/core_arch/src/x86/avx512vnni.rs:892:4
  qdrant> error: could not compile `quantization` (lib) due to 1 previous error
  ```

  Downstream failures: `unit-qdrant.service`, `system-units`, `etc`,
  `activate`, `nixos-system-go-choir-b-26.11.20261015.fe13242`. The guest
  image built fine (it does not carry qdrant).
- The qdrant package is identical on both revisions: `1.18.2`, same source
  and `cargoHash` (`pkgs/by-name/qd/qdrant/package.nix`; last changed
  2026-06-10). The input that changed is the compiler: the old lock has
  rustc 1.95 (`rust/1_95.nix`), the bumped lock has rustc 1.98.1 (log:
  `rustc-1.98.1`) linked against the system `llvm-21.1.8`.

## Cause (upstream nixpkgs breakage)

nixpkgs builds rustc against its own system LLVM (`--enable-llvm-link-shared`).
rustc 1.98's `core::arch` (stdarch) declares `llvm.x86.avx512.vpdpbusd.512`
with the LLVM 22 signature, while nixpkgs still links LLVM 21.1.8, whose
declaration differs. qdrant's `quantization` crate (turboquant SIMD,
`lib/quantization/src/turboquant/simd/**/x64.rs`) is the first crate on the
host closure that calls the AVX-512 VNNI intrinsic, so it is the one that
breaks. This is the same family nixpkgs already hit with vectorchord on
rustc 1.96 (NixOS/nixpkgs PR 538998, which disables avx512vnni via a patch
guarded by `versionOlder rustc.llvm.version "22.0.0"`; PR 544495,
"rust_1_97: build against LLVM 22", was closed unmerged). nixpkgs master has
moved to rustc 1.99.0 (2026-10-01) but `nixos-unstable` has not advanced past
`fe13242`, and nixpkgs master qdrant is still 1.18.2, so no upstream fix is
available through the channel yet.

## Options

1. Upstream fix already in nixpkgs: none reachable. The channel head is the
   broken revision; no newer qdrant exists.
2. Pin qdrant to the currently deployed build: a second, never-bumped nixpkgs
   input at the deployed revision, with an overlay that takes `qdrant` from it
   while the base rustc/LLVM mismatch exists. Same qdrant version (1.18.2), same
   derivation as production, so no data-format question and a binary-cache hit
   (no compile on CI or Node B). Self-retiring when the base rustc is built
   against LLVM 22 or newer.
3. Build the base qdrant with the old rustc (`makeRustPlatform` from the old
   input): keeps new native dependencies, but compiles qdrant (tens of minutes)
   with a mixed toolchain; more moving parts.
4. Patch qdrant to drop avx512vnni (as vectorchord did): changes vector search
   performance characteristics and carries a patch against upstream source.
5. Disable qdrant: not acceptable (the Go qdrant client depends on it).

## Chosen fix

Option 2. Add input `nixpkgs-qdrant` locked at the deployed nixpkgs revision
`ec84054698e3875e23d6057a10283eb9dfa41f1b` and a `choirQdrantModule` overlay
(same pattern as `choirDoltModule`) that substitutes qdrant from that input
whenever the base `rustc.llvm.version` is older than 22. `nixpkgs-qdrant` is
excluded from the weekly bump (the workflow bumps only `nixpkgs`).

Consequences:

- qdrant stays at 1.18.2 on the host: no storage/format change, no behavior
  change versus production.
- qdrant keeps the old revision's dependency closure (openssl, jemalloc) until
  the overlay retires. This is the status quo on `main` today, not a regression.
- Retirement: when a weekly bump brings a base whose rustc uses LLVM >= 22 (or a
  qdrant that builds), the condition flips and the base qdrant is used again;
  then delete the input and the module.

## Rollback

Revert the commit on `patch/weekly-nixpkgs` (or do not merge PR #74). `main`
is untouched.

## Heresy delta

discovered: base rustc/LLVM mismatch in nixos-unstable (upstream);
introduced: one extra flake input, self-retiring; repaired: bump branch builds.
