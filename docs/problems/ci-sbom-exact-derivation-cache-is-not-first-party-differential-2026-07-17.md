# First-Party SBOM Cache Is Exact-Derivation, Not Differential

Date: 2026-07-17 UTC
Status: observed; repair not yet applied
Mutation class: red if repaired (`ci_check_gate`, accepted SBOM identity, checksums, cache publication)
Classification: substrate — the accepted SBOM cache key and package-change authority cannot distinguish semantic dependency stability from per-commit artifact identity

## Problem

The differential SBOM workflow restores and publishes accepted cache bundles, but first-party Choir SBOMs are rebuilt on every selected commit. The implementation reuses a package only when its complete `sbom-<package>.drv` path is byte-identical to the previous accepted manifest. Every first-party derivation embeds the current repository commit and date, and every Go service source includes all production files under `internal/`. Exact derivation identity therefore changes across commits even when a service's dependency inventory is unchanged.

The cache is mechanically healthy but has ineffective first-party granularity. Calling the construction step “Build changed packages and reuse identical SBOMs” overstates the implemented behavior.

## Hosted evidence

- Main run `29559210595` restored cache key `sbom-bundle-v2-ec593c3901a4ff724d6e61e02c0328d31884eade` successfully, then spent 16m34s in the differential script and reported `10 built, 1 reused, 1 unchanged optional failures skipped`.
- Its source delta from baseline `ec593c3901a4ff724d6e61e02c0328d31884eade` changed vmctl production code but no source for most other first-party services. Nevertheless `auth`, `proxy`, `gateway`, `maild`, `maildctl`, `corpusd`, `sandbox`, `sourcecycled`, `vmctl`, and `frontend` all received new SBOM derivations. Only externally pinned `zot` reused an identical derivation.
- Main run `29570035893` repeated the result: the candidate job took 19m30s; its accepted manifest reported the same ten first-party packages as `built`, `zot` as `reused`, and `obscura` as an unchanged optional failure.
- Every rebuilt first-party package in accepted artifact `8403147713` had empty `added` and `removed` dependency sets relative to accepted commit `42e50b6b1fa3ae7461bb789ec173521a768b548d`.
- Comparing accepted `auth` SBOMs across `ec593c3` and `b746fd7` found changes only in root derivation/output paths, generation timestamp, and UUID. The dependency set did not change.

## Source evidence

- `flake.nix` derives `buildCommit` and `buildDate` from `self.rev` and `self.lastModifiedDate`, then injects them into every Go binary and the frontend build.
- `goServiceSrc` includes every production file below `internal/` for every Go service.
- `.github/scripts/build-sboms-differential` reuses only when `previous_derivation == derivation` and the cached checksum matches.
- `.github/workflows/ci.yml` restores and saves the accepted JSON bundle, not current Nix package outputs.

No alternative or replacement differential implementation is present. The exact-derivation implementation is wired and behaving as written.

## Required invariant

An accepted SBOM must remain bound to the current package derivation and output path. Reuse must never copy stale root artifact identity. At the same time, unchanged normalized runtime dependency inventories should not require running `sbomnix` again solely because commit metadata changed.

A safe repair needs two independently verified identities:

1. **Current artifact identity:** package, source commit, current derivation, and current output path.
2. **Semantic dependency identity:** a deterministic fingerprint covering every input that can change the package's runtime dependency inventory.

When semantic identity matches, the workflow may reuse the prior dependency inventory only after rebinding and validating the current root identity. When it does not match or cannot be established, it must regenerate normally. Accepted publication remains downstream of the parent check and exact candidate verification.

## Belief state

- Confirmed: cache restore and accepted-cache save are functioning.
- Confirmed: exact derivation identity makes first-party cache hits structurally rare.
- Confirmed: two accepted hosted runs rebuilt ten first-party SBOMs while reporting zero dependency additions/removals.
- Unknown until candidate proof: whether semantic reuse plus root rebinding reduces selected hosted SBOM construction to a bounded evaluation path without weakening artifact identity.

## Rollback

Revert the eventual repair commit through a pull request to restore exact-derivation-only reuse. Do not direct-push main, dispatch Race, or mutate Node B for this CI-only repair.
