# Adopt drill adopt-1418: with polling in place the adopter never opens an operation (2026-10-15)

Date: 2026-10-15 14:18-14:49 UTC. Drill: `node scripts/reeng_tracer.mjs --label adopt-1418 --scenario adopt --expect-sha e485b1966… --timeout-min 30 --apply-min 10`
on staging `e485b1966` with the polling tracer change (`yellow(tracer)`, 120 s
poll of `adopter_serves_the_change` and `identity_names_adoption`). Computers:
publisher `computer-2bb1d7e0…`, adopter `computer-3916f9ba…` (VM
`vm-5539585dea4d0bd4d5dae410498f3566`), both stopped by the tracer. Evidence:
`~/.config/choir-qa/traces/adopt-1418/receipt.json`. Status: documented;
nothing patched (the polling did not matter here: the run never reached
`applied`).

## Outcome

Publisher engineering drill reached `applied` (14:30:32; restore skipped by
design). Release published and read back; tampered record refused (422
`release_digest_mismatch`) and unpublished record refused (404
`release_not_published`), both unchanged; adoption requested 14:30:47
(`201`, `same_base: true`, adopter tree `2a5d1772…`). Then no adopter operation
appeared: `acceptance` 14:48:39 `reached=adoption_requested`, failed
`[frozen_patch_is_the_release, adopter_applied, planned_restart,
adopter_serves_the_change, identity_names_adoption]`. No `adopter_operation_state`
phase was ever recorded.

## Evidence (adopter console)

- First adopter engineering assignment `assignment-454e18e5…` ended `terminal
  failed` at 14:32:47 (about 2 min after the adoption request); its precommit
  rules settled `no` / `unresolved`.
- Texture authored its final turn `done:true` at 14:34:19; management's round 1
  logged "stays open ... an assignment it opene[d]" twice (14:31:20, 14:31:54).
  After 14:34:19 the console shows only the once-a-minute `fc_api` 400
  `Device not found` line (hotplug probe noise on a VM without a device) and no
  further runtime work through 14:48. 33 tool-loop iterations in total.
- No `not a directory`, no `mutation authority mismatch`, no
  `dispatcher: poison`, no broker `i/o timeout`.

## Classification (hypothesis; ledger not read)

The adoption request was accepted (`201`) but the adopter's desks did not carry
it to an operation: one engineering assignment failed within two minutes and
the Texture request was closed `done` without a second attempt, so nothing
froze. This looks like the engineering-desk judgment/environment class already
seen in jobs-0101 (A1-A3) and p5-go-served-0825, not an adoption-gate defect
(the gate refused both negative cases correctly and created the adoption). The
adoption's request path to the desks, and why the assignment failed, were not
read from the run ledger. Owed: read the adopter's ledger (turn-commit
reasons, controls, assignment failure reason) before deciding between a desk
prompt/environment problem and an adoption objective the desks cannot satisfy
(for example the release patch not applying to the adopter's source).

Heresy delta: discovered 0; introduced 0; repaired 0.

## Trace findings (ledger read, 2026-10-15)

Source: `~/.config/choir-qa/traces/adopt-1418/ledger/` (events.parsed.json,
timeline.txt). Engineering assignment `assignment-454e18e5…`:

- 14:31:52 `git apply` of the mounted patch ok; 14:31:57 deterministic checks ok.
- 14:32:02 the overlay recipe `... && cp -a dist/. /var/lib/artifact/release/frontend/ && rm -rf node_modules dist`
  ran 18 s and exited 1: 59 lines `cp: preserving permissions for '<dest>': Operation not permitted`,
  for every destination path **including files this same cp had just created**
  (`./choir-drill/choir-drill-d9529485.txt`, `.`, `./assets`, `./index.html`).
- 14:32:25 the model retried with `cp -r dist/. …`: exit 1, three lines
  `cp: cannot create regular file './desktop-bridge.html' | './legal/privacy-policy.md' | './legal/terms-of-service.md': Permission denied`.
- 14:32:45 `choir.Complete("blocked", …)` ("release-tree copy failed … existing protected
  files"). Management and Texture then had nothing to freeze; Texture cast research
  to "capture" the engineering report (below), research got `record not found`.

### Root cause (code, two stacked defects, both in the capsule contract)

1. `cp -a` can never exit 0 in a capsule. `capsuleAllowedSyscalls`
   (`internal/capsule/seccomp.go:15-39`) has no `getxattr`/`lgetxattr`/`fgetxattr`/
   `listxattr`/`llistxattr`/`flistxattr`/`setxattr` family and the default action is
   `ActionErrno` (EPERM, `seccomp.go:12,68`). `cp -a` (`-p`) reads ACLs/xattrs for each
   file and directory it writes; the read gets EPERM, cp reports it as
   `preserving permissions for … Operation not permitted` and exits 1, although the data
   was copied. This is hypothesis C of p5-go-served-0825, now confirmed: the failures
   hit freshly created files, so they are not an ownership/immutable-lower effect.
2. The first (failed) copy leaves 0444 files that the retry cannot overwrite.
   `makeSubjectTreeReadOnly` (`internal/capsule/source_snapshot.go:175-210`) makes every
   source file 0444/0555 (directories 0777). vite copies `frontend/public/*` into `dist/`
   keeping that mode, so `desktop-bridge.html` and `legal/*.md` (exactly the three
   names) land 0444 in the release tree, in the capsule's overlay upper, which the
   capsule uid owns. A second `cp -r` does `open(O_WRONLY|O_TRUNC)` on a 0444 file
   and gets EACCES. Files with hashed names and generated `index.html` are 0644, so
   they overwrote fine: that is why only three files fail. The same retry-over-0444
   shape as Defect B (8a74c15db), but in the upper, not the lower.

Not a pre-seeded lower tree: `/var/lib/artifact/release` is absent from the guest
root (jobs-0101 A3 saw it absent), the destination exists only because the first cp
made it. The recipe is the only copy of `cp -a` in prompts (`rlm_engineering_runtime.yaml:58`,
pinned by `internal/runtimeprompts/prompts_test.go:135`).

### Texture "capture" of an engineering report (separate, small)

`internal/texturewrite/frame.go:175` renders every source of every report that has no
capture as `(not captured: it cannot be cited until research Reads it)`. Management's
report cited the engineering assignment report (`<assignment-report:report:sha256:…>`,
an internal record, not a readable source); Texture obeyed the hint literally and cast
research to Read and capture it (14:33:34); research's `ReadEvidence` on that id returns
`record not found` (14:33:42). The hint is only true for sources research can Read.

## Failure modes considered for the fix

Candidate fixes and how each could fail:

- **Recipe `cp -r --no-preserve=mode,ownership,timestamps` alone**: still fails on a
  retry over the 0444 files the first run left (defect 2) unless `-f` is given; fails
  where the destination already holds read-only files and `-f`'s unlink needs a writable
  parent (upper dirs are capsule-owned, so fine; lower dirs would not be, but none exist
  here). `--no-preserve=mode` must create 0666&~umask files, not source 0444 (verify on
  GNU cp; BSD cp has no such flag, so the test is Linux-only).
- **Recipe `rm -rf dest && mkdir && cp -r`**: fails if any part of the destination is
  lower-layer (rm would need to whiteout in a root-owned lower directory); also deletes a
  partial release the model built across cells (a Go change builds `bin/autoputer` into
  the same release root: `rm -rf` of only `frontend/` is safe, of `release/` is not).
- **Seccomp: allow read-only xattr calls** (get/list families) so `cp -a`, `tar`, `rsync`
  work: removes the class for every tool, but is red (protected surface: the capsule
  sandbox), needs a staging deploy to prove, still leaves defect 2 for any `cp` retry,
  and `setxattr` must stay denied. Larger than the failure needs; recorded as a residual.
- **Broker verb that builds the release tree**: new API surface, a second path next to
  the one the freeze already reads (`ReleaseTreePrefix`); rejected as addition.
- **Making the source tree writable (undo `makeSubjectTreeReadOnly`)**: weakens the
  immutable-lower invariant that Defect B's fix deliberately kept; rejected.
- **Hint change in the Texture frame**: could hide a real "ask research to Read it"
  for a web source. Limit the new text to sources with no http(s) URL; keep the old hint
  otherwise. Could still leave Texture without a way to cite a blocker report; the new
  text must say what to do instead (state the blocker from management's report without a
  citation, route a re-cast through management, or to the owner).

Test plan: a Linux-container test runs the recipe's copy step as a non-root uid over
a source tree whose files are 0444 (what vite produces), twice (a retry), and requires
exit 0 and updated content both times; the same test asserts the old `cp -r` recipe
fails on the second run (the reproduced failure). The `cp -a` seccomp EPERM cannot be
reproduced without the capsule filter and is not claimed by the test; it is covered by
the recipe no longer using `cp -a`. Frame hint: a unit test on the rendered text.

## Fix landed (branch fix/engineering-release-copy, not pushed)

- Overlay recipe: `cp -a` replaced by `cp -rf --no-preserve=mode,ownership,timestamps`
  (yellow; tested in a Linux container as non-root over 0444 output, twice, plus the
  reproduced plain `cp -r` retry failure). Idempotent, so a retry cell is safe.
- Texture frame: an uncaptured source with no http(s) URL and no capture id (an internal
  record such as an assignment report) no longer says "until research Reads it"; it says
  research cannot Read it, to state the outcome uncited, and that a blocker goes to
  management for one bounded re-cast or to the owner.
- Residual (named, not fixed): the capsule seccomp filter still refuses the read-only
  xattr/ACL calls, so any other `cp -a`, `tar`, `rsync -a` or `install -p` a desk
  improvises exits 1 the same way. Allowing `getxattr`/`lgetxattr`/`fgetxattr`/
  `listxattr`/`llistxattr`/`flistxattr` (never the set family) is a red capsule change
  with a staging proof; take it only if a second tool shows up in a ledger.
- A rerun of the adopt drill must show: the adopter's engineering cell with the
  `cp -rf --no-preserve…` copy exits 0 on its first attempt (no `preserving permissions`
  lines), `choir.Freeze` accepted, an `adopter_operation_state` phase recorded, and
  Texture casting no research for an `assignment-report:` source.
