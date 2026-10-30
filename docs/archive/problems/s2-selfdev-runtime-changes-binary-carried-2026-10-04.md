# S2-f: self-dev runtime changes binary-carried — no source patch reach the builder

**Date:** 2026-10-04
**Status:** join surfaces landed by this commit series (see Resolution);
transport/mint wiring is the recorded residual.
**Mutation class of this record:** green (problem documentation only).
**Station receiving it:** S2 layering-runtime-from-release, slice S2-f
(self-dev → builder join).

## Evidence

Source-traced 2026-10-05 (metamission v4 Orientation):

- The capsule freeze (`internal/capsule/executor.go` `StageGrantedRelease`)
  staged only `var/lib/artifact/release/**` — capsule-compiled binaries —
  as a plain file release. `workspace/platform` source edits (classified
  `LedgerSource`) were recorded in the bundle's `OrderedFileEffects` but
  never materialized for a builder: the effect contract had no way to say
  "rebuild this from source".
- `internal/builder` could evaluate a caller checkout but had no path to
  apply a source patch, so a layered release could not be produced from a
  self-dev bundle even by hand.
- Consequence: the only self-dev runtime path was binary-carried — exactly
  the artifact the station's evidence-quality rule forbids for acceptance
  (builder-produced releases only).

## Resolution (this commit)

**Emit side (guest):**

- `StageGrantedRelease` returns `*StagedRelease` carrying the staged files
  plus `source.patch`: a unified diff of every `workspace/platform`
  change against the capsule's pinned source snapshot (`source-lower`),
  emitted via `emitSourcePatch`. Add/delete files use `/dev/null` markers
  so `git apply` honors them. The patch lands inside the frozen bundle
  dir and travels with it.
- `CapsuleEffectBundle` records `source_patch_sha256` and
  `source_patch_base_commit` (recovered from the capsule's
  `capsule-source-git:<commit>` pin) — provenance for which source the
  patch applies to.

**Consume side (host builder):**

- `builder.Request` gains `SourcePatch` + `SourcePatchBaseCommit`. When
  set, `patchedFlakeRef` clones `--source-dir` into a temp repo, checks
  out the pinned base, `git apply`s the patch, commits a synthetic
  revision, and nix-evaluates `git+file://<clone>` — so the flake's
  `self.rev`, `build.json`, and the builder receipt all name the same
  synthetic commit (`code_commit_source=derived-patch`; the receipt also
  records `source_patch_sha256`/`source_patch_base_commit`).
- `choir-builder --source-patch <file>` exposes it on the CLI.

## Recorded residual

The materializer still mints plain file releases from `RuntimeFiles`;
capsules that stage binaries keep working during the transition. Routing
patch-carrying bundles to the host-builder service endpoint (and minting
the layered offer from the builder's receipt) completes the join end to
end and is the remaining S2-f wiring before station acceptance can use
self-dev-produced releases.
