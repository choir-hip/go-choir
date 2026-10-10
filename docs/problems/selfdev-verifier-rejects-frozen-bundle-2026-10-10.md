# Self-development: the independent verifier rejects the frozen bundle (2026-10-10)

Found by the fifth Gate 2 reality rerun (M11 probe) on staging, build
5b851eed, disposable `computer-69454e0a…` (VM `vm-b4faf10b…`), operation
`selfdev-d2cf4897…`. Written 00:01Z. Problem first; cause not yet known.

## What got further

- 23:15:52 operation started. 23:39:09 the implementation assignment
  completed: one evidence file under `docs/evidence/` (a new directory
  in the capsule; the 2c68cc18 freeze fix held), release tree staged,
  **freeze succeeded first try** (bundle `3ebbc78d…`, state `frozen`).
- The report reached Texture (disposition `delivered`; the 028446a5
  binding fix held). 23:47:51 Texture took a supervision turn
  (`wait_for_evidence`).
- The host-side reconcile opened the verification assignment
  (`assignment-4a803a1d…`).
- 23:59:46 the verifier recorded `decision: fail` through the
  verification tool (`tools_capsule.go`); the operation went
  `frozen → failed`, "independent verifier rejected frozen bundle".
  The probe then stopped (5 of 16 legs).

## What the trace shows so far

The verifier wrote a program into the verification capsule's source
tree (`/workspace/platform/cmd/verifydigest/main.go`) and ran it with
`go run` to check the digest. Its reason for `fail` is not in the first
200 run events (the listing cap) and the run is still active at
00:00Z, so its report has not arrived. Hypotheses, not findings:

- H1: the verifier could not reproduce the bundle digest (its own
  digest program disagrees with the freeze's content digest algorithm).
- H2: the verifier judged the change itself (an evidence file plus a
  release tree) as not matching the request.
- H3: something in the verification capsule (network closed, toolchain,
  Go module cache) made its checks fail and it recorded fail rather than
  a blocker.

Next observation: the verifier's report summary, then the decision
cell's arguments.

## Cause (00:02Z; the verifier's report, confirmed in code)

The verifier reported a fail-closed blocker at 00:01:42Z:
`choir.InspectBundle` refuses the mounted draft ("invalid frozen bundle
draft"). `internal/yaegikernel/inspect_bundle.go` decodes the draft into
`bundleDraftMirror` with `DisallowUnknownFields`; the mirror predates
S2-f and lacks `source_patch_sha256` and `source_patch_base_commit`,
which `transaction.CapsuleEffectBundle` (and so every draft with a source
patch) carries. The mirror would also recompute the wrong content digest,
since the declared digest covers those fields. Every payload check the
verifier could run by hand passed (binding, content digest, runtime file
hashes, source patch hash, subject tree digest).

H1 confirmed in a narrower form: the in-cell inspection, not the
verifier's own program, cannot read the draft. H2 and H3 refuted. The
verifier's `fail` was the correct fail-closed call. Every self-development
change that touches platform sources has been unverifiable since S2-f
landed; the September 29 pass predates it.

Fix: add the two fields to the mirror with the same tags
(`omitempty`), so decode and digest match; pin with a test that inspects
a draft carrying both fields. Mutation class orange (verifier input
parsing; the verification decision stays with the verifier).
Rollback: git revert.

## Rerun 10 (07:35Z): second cause, the source patch cannot be applied

Found by M11 rerun 10 on cf0969cf: disposable `computer-90138c40…`
(VM `vm-a507c0fa…`), operation `selfdev-14851a1c…`, bundle `e164a288…`.
5 of 16 legs. The mirror fix held: the verifier inspected the bundle
in-cell.

- 07:01:19 the implementation assignment (`assignment-03a092ff…`)
  started. 07:20:37 it completed. Its change adds three new files: an
  evidence JSON under `docs/evidence/`, plus `episode.go` and
  `episode_test.go` in `internal/selfdevevidence`.
- 07:20:38 the verifier assignment (`assignment-3ffa5762…`) started.
  It took 86 turns.
- 07:35:59 the verifier called `choir.Verify("fail", …)` with this
  summary (finding, quoted from its final cell):
  - source.patch is not applicable: `git apply` (the builder's
    `patchedFlakeRef`) rejects it with "docs/evidence/…json depends on
    old contents".
  - All three `/dev/null` creation hunks read `@@ -1 +1,N @@` (old count
    1, where it must be `-0,0`), and each carries a stray context line.
  - Repairing the headers by hand gives files one byte longer, so the
    hashes differ from the declared candidate hashes.
  - Everything else checked out: candidate bytes, the content digest,
    and `go test` for the new package passed.
- Side friction (finding): the verifier's `choir.Message` to the
  implementation desk was refused by the reducer with
  `update_coagent engineering cannot message engineering`, so its report
  travelled only in its completion.

### Cause (confirmed in code)

`emitSourcePatch` (`internal/capsule/executor.go`) splits lines with
`difflib.SplitLines`, which always appends `"\n"` to the last element
of `strings.SplitAfter`:

- An empty base becomes one phantom empty line, so every added file is
  diffed against `["\n"]`. The result is `@@ -1 +1,N @@` plus a stray
  context line.
- Every non-empty file gains a phantom trailing empty line too, so a
  hunk that touches the end of a file carries a context line that is
  not in the file.
- A file without a final newline is silently given one, with no
  `\ No newline at end of file` marker.
- An empty added file produces no hunk, so it vanishes from the patch.

The only test (`TestEmitSourcePatchSkipsDirectoryChanges`) checks
substrings. Nothing on the freeze path ever applies the patch, so the
freeze claims reconstructability it never checks (standing question:
artifact-verified success).

### Reruns 8 and 9 carried the same defect (finding plus hypothesis)

- Finding: the emitter is unchanged since 2c68cc18 (2026-10-09 22:38Z).
  Reruns 8 and 9 each added a new evidence file, and both reached
  `awaiting_approval`, so their verifiers passed bundles whose patch had
  the same malformed creation hunks.
- Hypothesis: had their apply not stalled on checkpoint starvation, the
  builder's `git apply` would have failed next.
- Verifier variance: rerun 10's verifier ran `git apply`, and the
  earlier verifiers did not. The verifier contract does not require the
  check the builder relies on.

### Proposed fix (red: self-development bundle and release path)

1. Correct the line splitter: no phantom line, and an unterminated last
   line carries git's `\ No newline at end of file` marker. Emit
   `diff --git` headers with `new file mode` / `deleted file mode`, so
   empty files survive.
2. Round-trip check at freeze: apply the emitted patch with `git apply`
   to a scratch copy of the base files, and require the result to match
   the candidate bytes exactly (deleted files absent). A mismatch fails
   the freeze with a named error, before any verifier runs.
3. Tests first, through `git apply`: an added file, an added file in a
   new directory, an empty added file, an edit at end of file, a file
   without a final newline (both directions), a deletion, and a
   multi-hunk modify.

### Fix: a faithful source patch, proven at freeze (red ceremony)

- Conjecture delta: a frozen bundle is reconstructable only if its
  source patch has been applied by the builder's own applier and
  reproduces the candidate bytes. Until now the freeze emitted a diff
  and trusted it. The verifier was the first, and an inconsistent, line
  of defense.
- Changes:
  - `internal/capsule/source_patch.go` (portable, so it is testable off
    linux). `patchLines` has no phantom line and handles an
    unterminated last line with git's marker. `renderSourcePatch` emits
    `diff --git` headers with new/deleted file modes, so empty files
    survive. `verifySourcePatch` runs `git apply` on a scratch tree of
    the base files and compares bytes.
  - `emitSourcePatch` (`executor.go`) builds the file set, renders the
    patch and verifies it before writing `source.patch`. A patch that
    cannot reproduce the candidate fails the freeze with an error that
    names the file.
- Tests first (`source_patch_test.go`): the six failure modes are listed
  at the top, and 11 shapes are proven through real `git apply`. It also
  checks that the round-trip refuses the rerun 10 phantom hunk and a
  byte mismatch.
- Protected surfaces: the self-development freeze and bundle
  (run-acceptance input). The verifier's decision authority is
  unchanged.
- Admissible evidence: an M11 rerun whose bundle reaches
  `awaiting_approval`, and whose apply builds through `patchedFlakeRef`.
- Rollback: git revert. A bundle frozen under the old emitter remains
  unappliable either way.
- Heresy delta:
  - Discovered: "freeze claims reconstructability without applying the
    patch", and "verifier passes an unappliable patch" (reruns 8 and 9).
  - Repaired: the first, on a local proof only, until staging.
  - Introduced: none.
  - The verifier-contract variance stays open, as residual
    `verifier-apply-check`.
