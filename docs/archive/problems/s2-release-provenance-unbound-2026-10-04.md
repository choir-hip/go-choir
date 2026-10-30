# S2-c: release provenance unbound — caller-supplied commit, no binary check

**Date:** 2026-10-04
**Status:** fixed by this commit series (see Resolution).
**Mutation class of this record:** green (problem documentation only).
**Station receiving it:** S2 layering-runtime-from-release, slice S2-c
(provenance).

## Evidence

Source-traced 2026-10-05 (metamission v4 Orientation):

- `internal/builder/request.go` `CodeCommit` was caller-supplied; nothing
  checked the binary's embedded build identity against the manifest. The
  vm-3dc68688 crash-loop was exactly this: a hand-staged `/tmp/closure.nar`
  with a synthetic `code_commit` minted as a real layered offer.
- The corpusd mint accepted `code_commit` verbatim from the caller.
- Nothing bound a release to a builder evidence receipt; "builder-produced"
  was an assertion, not a checkable fact.

## Resolution (this commit)

- `ReleaseManifest` gains `code_commit` and `builder_receipt_digest`. The
  updater verifies the materialized entrypoint's
  `share/go-choir/build.json` commit equals `code_commit` before the
  pointer swap (`verifyEntrypointBuildCommit` in `internal/updater/closure.go`).
  `buildGoModule` binaries embed their commit there via `mkGoService`.
- `internal/builder` derives `code_commit` from `--source-dir`
  (`git rev-parse HEAD`) instead of trusting the caller; the receipt
  records `code_commit_source` (`derived`|`caller`) and `source_dirty`.
- The corpusd mint carries `code_commit` and `builder_receipt_digest`
  through to the release manifest.
- A manifest that claims a commit the binary does not embed refuses the
  apply before mutation — the hand-staged release shape can no longer
  activate with invented provenance.
