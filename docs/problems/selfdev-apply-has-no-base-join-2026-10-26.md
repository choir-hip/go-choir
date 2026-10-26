# A frontend-only self-development apply never checks its base (2026-10-26)

Status: documented; fix not built. Found by design review 303 (Devin and
DeepSeek) of the U4 stack rebase design. Class of the fix: red (apply path,
security provenance), review tier 2.

## Problem

An owner-approved candidate that changes no Go input is not host-built
(`hostBuildRequired = consensusDecided || len(binaryInputs) > 0`,
`internal/agentcore/self_development_materializer.go:354-360`), so the host's
`baseRefusal` / `ReasonBaseCommitMismatch` (`internal/hostbuild/service.go:383-401`)
never runs for it. The guest updater does have a base check,
`checkStateCompatibility` comparing `manifest.BaseCommit` with
`bootedBaseCommit()` (`internal/updater/updater.go:466-477`), but the plain
manifest never sets `BaseCommit` (`self_development_materializer.go:433-438`;
only the layered host-built manifest does,
`self_development_host_build.go:364`), so `omitempty` skips it. The apply then
installs `bundle.RuntimeFiles` verbatim. The base the candidate was built on is
already recorded and unused: `bundle.SourcePatchBaseCommit`
(`tools_capsule.go:330`, `source_projection.go:355`).

## Severity (the two seats differ; the fix is the same)

- Devin: live; an old-base frontend bundle installed over a newer base reverts
  any frontend fix the newer base carried.
- DeepSeek: narrower today. At `stack_base_moved` the computer already serves
  the old-base frontend (`current/frontend`), and an open operation blocks a
  newer Freeze, so the stale candidate re-applies the same old frontend. The
  revert of a live fix needs a newer-base release already current while an
  old-base candidate is frozen (e.g. a platform follow on a computer with source
  projection off). The underlying live defect is H1 (computers on a
  self-development stack do not receive the platform's frontend fixes).

Security comes first (owner, 2026-10-25 and 2026-10-26), so the join is made
unconditional either way.

## Fix shape

1. The plain release carries `BaseCommit = bundle.SourcePatchBaseCommit`; the
   updater's existing check refuses a candidate whose base is not the booted
   base, as `base_moved_since_freeze`, naming both commits.
2. Guards: an empty `bootedBaseCommit()` (dev builds,
   `source_projection.go:60-79`) and a bundle with no source patch are defined
   and tested, never a silent pass or a refuse-everything; compare with
   `EqualFold`.
3. A crash-boot settlement of an operation already applied (journal terminal
   `applied`) is never refused by the new check.
4. Not gated on `source.rebase_offer`. Until U4 lands, a refused candidate is
   rebuilt by a new round; with U4, the rebase round handles it.
