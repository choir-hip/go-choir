# Adopt drill adopt-1418: with polling in place the adopter never opens an operation (2026-10-15)

Date: 2026-10-15 14:18-14:49 UTC. Drill: `node scripts/reeng_tracer.mjs --label adopt-1418 --scenario adopt --expect-sha e485b1966… --timeout-min 30 --apply-min 10`
on staging `e485b1966` with the polling tracer change (`yellow(tracer)`, 120 s
poll of `adopter_serves_the_change` and `identity_names_adoption`). Computers:
publisher `computer-2bb1d7e0…`, adopter `computer-3916f9ba…` (VM
`vm-5539585dea4d0bd4d5dae410498f3566`), both stopped by the tracer. Evidence:
`~/.config/choir-qa/traces/adopt-1418/receipt.json`. Status: **traced** (see "Trace findings"); nothing patched (the polling did not
matter here: the run never reached `applied`).

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

## Classification (first pass, before the ledger was read; superseded by "Trace findings")

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

## Trace findings (2026-10-15; ledger read)

Source: the adopter's data disk (`vm-5539585dea4d0bd4d5dae410498f3566`) read
**read-only** with `debugfs rdump /state.texture` on Node B, after confirming
the VM was stopped (vmctl journal 14:48:40 "destructive kill ... cause=stop",
"stopped VM"; firecracker pid 153781 gone; no firecracker process for that id
in the process list). The rdump copy on Node B was deleted afterwards; the
image was not mounted or written. `dolt sql` over the copy gave 249
`choir.event` rows plus the assignment, report, adoption and run objects.
Extracts: `~/.config/choir-qa/traces/adopt-1418/ledger/` (`events.json`,
`events.parsed.json`, `timeline.txt`, `objects.json`, `late.txt`). Line numbers
below are `timeline.txt` lines; ids are `event_id` prefixes. Tags: **[E]**
evidence from the ledger, **[H]** hypothesis.

### 1. What management cast to engineering

- [E] The adoption entered as a conductor `open_app texture` decision whose
  seed prompt is the full adoption objective (`50369781`, line 3): release id,
  publisher computer and operation, patch sha256, the changed path, the
  "apply the published patch, change nothing else" instruction, and
  engineering's recipe (`git apply /selfdev/adoption/source.patch`, the
  deterministic check, `git apply --check --reverse`, "build the release tree,
  freeze ... and complete").
- [E] Texture cast that objective verbatim to management (`fc50def0`, line 19),
  with a one-paragraph plan revision. Management then cast it to engineering
  with `choir.Cast("engineering", r.Objective, {"parent": r.RecordID,
  "plan_revision": ...})` (`0ea18952`, line 36): **the objective text is inlined
  verbatim, so the spec travels in the cast**. The patch itself is not inlined;
  it is mounted read only in the capsule (`/selfdev/adoption/source.patch`,
  `release.json`), and the adoption object carries `patch_sha256`,
  `test_command`, `same_base: true`.
- [E] Engineering's first `choir.Records()` shows the cast with the full text
  (`f9f1c6c6`/`3b1b54ab`, lines 47, 49); it read `release.json` and
  `source.patch` successfully (`4c5fc3d1`, line 73). Management also sent a
  `choir.Note` to the assignment ("proceed ... if any ... failure occurs, do not
  freeze; report the blocker", `6d98381e`, line 76).

### 2. How the engineering assignment ended

- [E] `Complete(blocked, verdict none)` chosen by the engineer, not an error,
  crash or timeout (`b636621d` / `55bdb42b`, lines 111, 115; reducer:
  `fate_terminal: true`, `choir.co_super_assignment_report` `result: blocked`,
  `stop_reason: failed`, run state `failed`). Summary: patch applied and both
  checks passed, frontend build completed, "the required release-tree copy
  failed: /var/lib/artifact/release/frontend contains existing protected files,
  and cp reports Operation not permitted/Permission denied; retries with
  ordinary cp also failed ... I did not freeze an incomplete release tree."
- [E] Sequence: `git apply /selfdev/adoption/source.patch` exit 0 (`12745fa1`,
  line 82); the deterministic check and the reverse-apply check exit 0
  (`67f93b63`, line 92; an earlier cell with a trailing `git status --short`
  failed on "not a git repository", `b254f83f`, harmless); then the recipe cell
  `npm ci && npm run build && cp -a dist/. /var/lib/artifact/release/frontend/`
  (`9b700c58`, line 99) built, then `cp: preserving permissions for
  '/var/lib/artifact/release/frontend/./...': Operation not permitted` for every
  file and `.` (`65e4e4ce`, line 100, exit 1, plus ~430 `[mutex.cc : 955] RAW:
  pthread_getschedparam failed: 1` lines from node/vite, not the failing step);
  one retry with `cp -r dist/. ...` (`074bf77e`, line 107) failed with `cp: cannot
  create regular file '.../desktop-bridge.html' / 'legal/privacy-policy.md' /
  'legal/terms-of-service.md': Permission denied` (`34fc4af6`, line 108);
  Complete(blocked).
- [E] No `choir.WriteFile`, no `choir.Freeze`, no broker errors, no `i/o
  timeout`, no `mutation authority mismatch`. Two minor tool errors early
  (a Go-in-cell compile hint at line 60 and a refused `crypto/sha256` import at
  line 64, `diag_kind import_preflight`) were corrected in the next cell. The
  engineering model failed over once (`70d5c8b9`, line 44, opencode-go circuit
  open, to `gpt-6-luna`); no relevance found.
- [E] The recipe that failed is the one the engineering overlay teaches verbatim
  (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml:58`:
  `... && cp -a dist/. /var/lib/artifact/release/frontend/ && rm -rf
  node_modules dist`, "It fits within one choir.Exec").
- [E] The same `cp -a` failure and the same `Operation not permitted` on
  preserve hit **every** engineering assignment that reached the release copy in
  the two drills' publisher computers, read from `bundle-a1.json` (`run_events`):
  adopt-1418 publisher assignments `95a028e7` (14:27-14:28), `cd96097a`
  (14:19-14:20, "failed with permission denied (cp preservation and then file
  creation)"), `6180f535` (14:24-14:25); adopt-1400 publisher assignments
  `37eca2d5` (14:05), `063e2b58` (14:07) and `15eb8ff0` (14:02). The publisher
  eventually froze only on a later attempt that worked around it (adopt-1400
  `063e2b58`, 14:09: `find /var/lib/artifact/release/frontend -mindepth 1
  -maxdepth 1 -exec rm -rf {} +` then `cp -r`, "release-copy permission issue
  avoided by cleaning protected destination contents and copying without
  preservation"; adopt-1418 `95a028e7`, 14:28: ran `npm run build` and froze
  after only checking `index.html` and the drill file exist in the
  already-partly-populated release tree).
- [H] The mechanism of the EPERM is not proven from this ledger: the `cp -a`
  preserve step is denied inside the capsule (ownership or mode of
  `/var/lib/artifact/release/frontend` entries pre-existing from the base
  release, or the capsule's lack of chown/chmod), and the second `cp -r` then
  cannot overwrite the three read-only files (mode copied from `frontend/public`)
  that pass one created. The same symptom is already in p5-go-served-0825
  assignment 2 ("cp -a could not preserve permissions").

### 3. Why Texture ended `done:true` without an operation

- [E] Management turned the blocked report into a blocker packet to Texture
  (`work_disposition: completed`, after two rejected `ReportPacket` shapes:
  an unknown desk and `source_id` vs `source_ids`, lines 128-141): no frozen
  bundle, "the release remains blocked pending an authorized way to update the
  protected destination". Management did **not** re-cast engineering.
- [E] Texture woke at 14:33:08 (`integrate_execution_findings`), tried to read a
  skill file outside `/tmp` (`e69593df`, line 151, refused), then cast **research**
  (`5e30214f`, line 165) to "read and capture the engineering final report" so it
  could cite it. Research has no access to assignment reports
  (`ReadEvidence` returned "record not found", `842cf204`, line 183;
  `ListEvidence` empty; web search returned an unrelated page) and reported the
  outcome "unverified here, not refuted".
- [E] Texture then rewrote its document and called `choir.Done` (`129a79b5`,
  line 242, 14:34:19): "Answered the adoption request with management's reported
  failure outcome; no frozen bundle exists to verify, and continuing requires an
  authorized destination-write path." The document now says the release "remains
  unapplied as a complete release until an authorized path to update the
  protected destination is available."
- Verdict: **dead-ended, not satisfied and not refused.** Texture treated "blocked
  on a permission" as a final, reportable answer to the owner request. The
  trajectory stays `live` with no open work item; the last event in the ledger is
  14:34:19 (`activation.passivated`). Nothing in the desks re-opens it, and the
  adoption (`choir.release_adoption`, `same_base: true`) has no operation bound.

### 4. Which failure class is this

- [E] **Not the 0825 class** (spec passed only by parent reference, engineering
  cannot read it): the objective was inlined in the cast, engineering read the
  patch and release record, applied the patch, and the checks passed.
- [E] **Not an adoption objective the desks cannot satisfy**: the desks applied
  the patch as published (git apply exit 0 on the adopter's own base). The
  objective's recipe ("build the release tree, freeze") fails only at the
  engineering overlay's generic release-copy step.
- [E/H] It **is the engineering release-copy failure that every drill assignment
  hit** (above). It looks like an adoption failure only because the adopter's
  desks stop after one engineering assignment; the publisher drills survive it
  because their story keeps re-casting until one assignment improvises a
  workaround (adopt-1418 publisher: 4 assignments before freeze; adopt-1400
  publisher: at least 3).
- [H] This also explains the dead end: the adoption objective told management
  to "cast it to engineering verbatim; then Verify the frozen bundle", with no
  retry or escalation clause, so the first blocker propagates to Texture as the
  answer. There is no owner escalation target on the adoption path (same gap as
  0825).

### Compared with adopt-1400

- [E] adopt-1400 adopter: request 14:11:19, `adopter_operation_state` `executing`
  at 14:15:11 (3 m 52 s later), `frozen` 14:15:22, approved 14:15:42, `applied`
  14:15:54, with `patch_matches_release: true`. Same build (`e485b1966`), same
  `same_base: true`.
- [H] In adopt-1400 the adopter's engineering hit the same `cp -a` failure and
  worked around it, which would account for the 3 m 52 s before `executing`
  (versus the ~2 m 15 s to a terminal blocked report in 1418). This is not read:
  the adopt-1400 adopter disk was not touched (scope), so the adopt-1400
  adopter ledger is unverified. What is verified is that the publisher in
  adopt-1400 needed three failed engineering attempts first.
- [H] What differed is therefore the model's behavior after the same EPERM, not
  the adoption path: in 1400 an engineering assignment worked around the copy
  (or Texture/management re-cast); in 1418 the adopter's single assignment chose
  `Complete(blocked)` after two copies, honoring management's note "if any ...
  failure occurs, do not freeze; report the blocker", and Texture took the
  blocker as final. Per Convergence Before Patching, this is the 4th+ recurrence
  of one environment fault across p5-go-served-0825, adopt-1400 and adopt-1418
  (cp -a / release tree), a root-cause-clustering trigger.

### Candidate repairs (not implemented)

1. **Substrate (preferred, deletes the failure class).** Make the release-tree
   copy not depend on preserving ownership and mode: change the overlay recipe
   (`rlm_engineering_runtime.yaml:58`) from `cp -a` to a form proven in the
   capsule, for example `cp -r --no-preserve=mode,ownership
   dist/. ...` into a fresh tree, or give the capsule a `choir.BuildRelease`
   verb that populates `/var/lib/artifact/release/frontend` itself. Needs a
   capsule-side read of why chown/chmod and overwrite are denied
   (`internal/capsule`, `ReleaseTreePrefix`), then a staging probe, before
   choosing; the engineering that succeeded used "clean the destination, then
   `cp -r`" (adopt-1400 `063e2b58`).
2. **Adoption path retries.** Give the adoption objective (and management's
   handling of a `blocked` engineering report) one bounded re-cast with the
   blocker text, and an explicit escalation target when it still fails, so a
   single blocked assignment does not become Texture's final answer.
3. **Texture handling.** A blocked execution report should not trigger a research
   cast to "capture" an assignment report (research cannot read it); it should
   cast management again or surface the blocker to the owner and stay open.
4. **Drill-level.** The tracer already reports `adoption_requested`; it could
   also surface the adopter assignment's terminal report so a failed adopter
   attempt is visible at 14:32 rather than only at the 30-minute timeout.

Heresy delta: discovered 1 (the engineering overlay's own release-copy recipe
fails in the capsule for every assignment, and the adoption path has no retry or
escalation after one blocked assignment); introduced 0; repaired 0.
