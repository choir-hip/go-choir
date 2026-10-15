# P5 go-served drill p5-go-served-0825: engineering never freezes, apply not exercised (2026-10-15)

Drill: `node scripts/reeng_tracer.mjs --label p5-go-served-0825 --scenario engineering --change go-served --expect-sha 2eae77de6… --timeout-min 60 --apply-min 30 --restore-min 12`
on staging `2eae77de6` (P5 host build enabled in corpusd: `corpusd host build:
enabled (source /opt/go-choir, installable .#autoputer)`). Test computer
`computer-5c80f0f9e16061255bdc83a3ab52c0fa`, VM `vm-ae39488465e478796b0f68a903099efa`,
trajectory `1ca501d5-e9b2-5602-9850-903a9bc7e9d9`.
Status: **traced** (this revision reads the run ledger). Documentation only; no code change.

Outcome: `pass=false`, `reached=engineering_final_report`; failed: frozen,
final_report_names_bundle, operation_on_request_trajectory,
verified_against_named_bundle, owner_approved, applied, one_planned_restart,
route_receipt, restore_to_pre_apply_head, go_build_job_done,
go_test_job_done, probe_served_after_apply, layered_release_serving,
restore_restarted_into_pre_apply, probe_gone_after_restore.

Consequence: P5's apply path (host build, layered release, restore) has not
been exercised on staging. P5 is deployed and dormant until an operation
reaches `accepted`.

## Evidence source

The computer was stopped by the tracer (09:27:53) and its memory snapshot
discarded, but its data disk (`/var/lib/go-choir/vm-state/<vm>/data.img`, ext4)
was read **read-only** with `debugfs rdump` (stopped computer; no mount, no
write). The runtime store is a Dolt repo under `state.texture/texture`;
`og_objects` holds `choir.event` (568 rows, the run events with tool
arguments and results), `choir.co_super_assignment` (3),
`choir.co_super_assignment_report` (3). Extracts are kept outside the repo in
`~/.config/choir-qa/traces/p5-go-served-0825/ledger/` (`events.json`,
`assign.json`, `p5.timeline.txt`). Guest console: 540 lines, consistent.

## What the ledger says (all times 2026-10-15 UTC)

The change asked of engineering: add package `internal/selfdevprobe`
(2 new files) **and edit the existing `internal/apihandler/routes.go`** to
register the route; run `go test` and `go vet`; build `bin/autoputer` plus the
frontend release tree; Freeze.

All three engineering assignments ended `disposition: failed`,
`capsule_disposition: revoked`. The word "failed" is the reducer's
translation ("reducer-derived from report ...") of an engineering `Complete`
with `result: blocked`, `verdict: none`. **No assignment crashed, timed out
or was cancelled; each one chose to stop** and reported a blocker. No cell
reached `choir.Freeze`.

| # | assignment | window | cast round | network | Complete(blocked) summary (verbatim gist) | root cause |
|---|---|---|---|---|---|---|
| 1 | `assignment-7731751382d9de71391f8662` | 08:26:09-08:26:37 | 1 | ecosystem_proxy | "Cannot implement responsibly: management Cast supplies no exact Texture-requested package contract or route response semantics." | management's Cast carried a one-sentence objective and `{"parent": <texture cast id>, "plan_revision": ...}`; the spec was only in the **parent** record, which engineering cannot read |
| 2 | `assignment-696f5d341caeb66519678ba4` | 08:28:17-08:29:25 | 3 | ecosystem_proxy | "WriteFile to /workspace/platform/internal/apihandler/routes.go was denied (permission denied) ... release copy failed because cp -a could not preserve permissions" | existing source files are read-only in the capsule overlay; `choir.WriteFile` on an existing file is `EACCES` |
| 3 | `assignment-fb0e72a48aa0f6299215b383` | 08:32:43-08:33:06 | 4 | forbidden | "Blocked: current assignment capsule did not provide authorization confirmation for the requested routes.go write or release-copy permissions ... made no changes" | management turned #2's permission bug into an instruction ("if runtime cannot provide authorized permissions, stop and report blocker; do not use unauthorized workaround"); engineering obeyed, only read `routes.go`, asked management, completed |

### Assignment 1 (event ids)

- 08:26:11.95 `choir.Records()` shows exactly one record, management's cast
  `cell:05d3d669...:call_bd744062...:cast:tray-1`: "Implement the
  Texture-requested GET /api/selfdev-probe route and package **exactly as
  specified** ..." plus `parent` = Texture's cast `cell:38049ed7...`. The
  nonce, file names and `routes.go` instruction are only in Texture's cast.
- 08:26:27 (`1aa3b907-de45-4175-b7a4-4143b41319d2`) engineering stages
  `choir.Ask("management", "Cast has no exact ... specification ...")`.
- 08:26:30.7 (`4dab3546-8aee-4d0e-b668-94c3fc049a99`) the completion guard
  `engineering_overlay_terminal_fate_required` fires: an engineering turn may
  not end without a terminal fate, so it cannot wait for the Ask's reply.
- 08:26:36 (`c1c3afbe-385d-4a20-ad14-152a7602cc0b`) `choir.Complete("blocked")`.
  Report `report:sha256:ff2cdd1f...`, 08:26:37. Management's concurrent run
  `9bd37bfb` (the one that read the Ask) "ended without a final Report"
  (console 08:26:51).

### Assignment 2 (the first real capsule error)

- 08:28:31.76 first write cell fails to *compile* (a raw-string backtick in the
  cell source), nothing written.
- 08:28:38.83 (`c9f87a90-98af-419d-a873-3f7cd35e01f1`)
  `choir.WriteFile("/workspace/platform/internal/apihandler/routes.go", ...)` ->
  `choir: write_file: write file failed: open /workspace/platform/internal/apihandler/routes.go: permission denied`
  (exit 1, `receipt_ref capsule-go-eval:sha256:ced88e3b...`). `ReadFile` of the
  same file had succeeded at 08:28:25.
- 08:28:57-08:29:14 (`231360a4-8587-4dc4-9a65-bff5c423c873`) the prescribed
  frontend cell (`npm ci && npm run build && mkdir -p
  /var/lib/artifact/release/frontend && cp -a dist/. ... && rm -rf ...`)
  returns `exit 1`; the 4 KB `error` field is dominated by hundreds of
  `[mutex.cc : 955] RAW: pthread_getschedparam failed: 1` lines, which push the
  actual stderr out of the model's view. 15 registry.npmjs.org CONNECTs in the
  egress log show `npm ci` itself worked.
- 08:29:18 (`d8aa85d8-8a96-4095-923b-599764470600`) `Complete(blocked)`;
  assignment report `report:sha256:a9c5a2f4...`. Its candidate subject digest
  `6622428f...` records an overlay change that is not the requested one (the
  `routes.go` write was the first write and failed), so no bundle.

### Assignment 3

Management's cast (round 4) said "Previous capsule lacked permission to write
routes.go and release copy. If runtime cannot provide authorized permissions,
stop and report blocker; do not use unauthorized workaround." Engineering read
`routes.go` (ok), noted `selfdevprobe.go` absent, never attempted a write,
`Ask`ed management for "authorized writable capsule permissions"
(`ff6d44de-7bce-4102-85e3-7c31a56c5efe`), and completed blocked
(`c437d554-3b1e-45a9-a6d7-cb5cad93a0ae`) 08:33:05.

### After the third report (why the story dead-ends)

08:33:50 management tries `choir.Escalate("owner", ...)` and gets
`tool_error: reduce: escalate tray-1 targets unknown desk "owner"`; it reports
a blocker to Texture "pending explicit authorization" (round 5). Texture
`Write` is refused (`r1.c1 is not a source in your frame`) and waits. No
engineering assignment is cast after 08:33:06. The tracer waited out its
60-minute budget. Nothing is wrong with the host or the P5 code path; the
loop idles because the model layer believes an *owner authorization* is
missing when the cause is a capsule file-permission defect.

## Why Freeze was never called

1. #1: engineering could not obtain the task (spec passed by reference).
2. #2: it could not edit the one existing file the change requires
   (`routes.go`), and its release copy cell exited non-zero.
3. #3: it never tried, by management's instruction.
4. Freeze requires a built release tree and the edit; none existed.

## Root-cause analysis (hypothesis marked where not proven by the ledger)

### A. Existing source files are not writable in an engineering capsule (primary; ledger + code)

`copyImmutableCommitTree` writes the pinned commit into
`<capsule>/source-lower/workspace/platform`; `makeSubjectTreeReadOnly`
(`internal/capsule/source_snapshot.go:175`, asserted by
`source_snapshot_test.go:79`) sets every regular file `0444` (`0555` if
executable) and every directory `0777`. The lower tree is owned by host root;
the broker runs in a user namespace mapping container uid 0 to host uid
65534. `handleWriteFile` (`cmd/capsule-broker/main.go:667`) does
`os.WriteFile(path, content, 0644)`, which `open(O_WRONLY|O_TRUNC)`s the
lower 0444 file owned by an unmapped uid. `CAP_DAC_OVERRIDE` in a user
namespace does not apply to an inode whose owner is unmapped, so the result is
`EACCES`. New files work (directories are 0777 for copy-up creation, the
2026-08-21 fix `013575843`, which only considered *creating* files).
This matches every datum: `routes.go` read ok / write denied; selfdev-jobs-0101
and selfdev-jobs-0654 changed only **new** files (`internal/drillmark/*`) and
wrote them fine. The go-served drill is the **first** drill whose required
change edits an existing file. No test covers editing an existing lower file;
`choir.WriteFile` is the only write surface in the cell
(`internal/yaegikernel/choir.go:505`; `choir.Rename/Chmod/EditFile` do not
exist even though `roles.go` grants `edit_file/rename/chmod` and the broker
switch implements none of them). Not yet verified on a real overlay (needs a
Linux guest). Confidence: high (about 85%).

### B. Passing a task by reference

0101's A3 and P5's #1 both read only "parent: <record id>" and blocked. The
engineering overlay's completion guard makes `Ask` unanswerable inside the
assignment. #2 and #3 only had the spec because a later management turn
pasted it in. This is an independent, intermittent blocker.

### C. Release-copy hostility (aggravator, partly hypothesis)

The overlay prompt prescribes `cp -a dist/. /var/lib/artifact/release/frontend/`
and panics on non-zero exit. In 0101 A2 the first `cp -a` printed `preserving
permissions ... Operation not permitted` and the retry `cp -R` then failed
with `Permission denied` on files the first copy had created `0444`
(vite copies source modes). Hypothesis: `cp -a` copies ACL/xattrs, the
seccomp allowlist (`internal/capsule/seccomp.go`) has no `getxattr`
family, default action is `EPERM`, so `cp -a` always exits 1 although the
data is copied. Also `sched_getparam/sched_getscheduler` are absent, giving
the `pthread_getschedparam failed: 1` flood from every Node process. In 0654
the model recovered (`cp -r` after `rm -rf`, 06:57:03 ok); in #2 it did not
retry.

### D. No path to the owner

`Escalate("owner")` is an unknown desk. A capsule defect looks like a missing
authorization and parks the whole story.

## Comparison with selfdev-jobs-0101 (build e83735f2a, 01:01)

Data: `~/.config/choir-qa/traces/selfdev-jobs-0101/bundle-a1.json` (run events).
Four engineering assignments, three of which blocked:

- A1 `b621ea7d` 01:04:19-01:10:04 blocked: a second `go build/test` was
  refused ("one job at a time"), no build evidence.
- A2 `fb5e1706` 01:13:52-01:15:12 blocked: `cp -a` `Operation not permitted`
  then `cp -R` `Permission denied` into `/var/lib/artifact/release/frontend`.
- A3 `e5d37a29` 01:18:29-01:18:50 blocked: cast was a bare parent reference
  ("No specified parent Texture build/copy actions were included in the Cast
  body"); `/var/lib/artifact/release` absent, uid 65534.
- A4 `3293a4d0` 01:22:11-01:34:28 **completed**: wrote the two new files,
  `go build` job, `go test`, `choir.Freeze` (01:33:58), bundle
  `ef28a69e...`.

So 0101's success was the **fourth** attempt of a change that adds only new
files, i.e. engineering-to-Freeze is intermittent (about 1 in 4 on the
easiest change) and the P5 change (edit an existing file) hits blocker A
deterministically. Between 0101 and today the code change that matters is
`b4f778552` (see the 0654 doc); it makes timeouts fatal. The two failure
families are different, so "something changed" is partly true (0654) and
"intermittent" is partly true (0101); P5's failure is neither.

## Proposed fix (not implemented)

Substrate vs symptom: A is a **substrate** defect in the capsule filesystem
contract (read-only immutable lower + broker-uid write verb); the prompt and
management rules are symptoms of it. No unwired replacement exists for A
(`edit_file`/`rename`/`chmod` are granted but unimplemented; the broker has a
switch with no handler: connecting them is the only "replacement", and it is
partial).

1. Make existing lower files writable through the overlay: simplest is to
   have `handleWriteFile` copy-up explicitly (write a temp file in the same
   directory and `rename` over the target; directories are 0777 so rename is
   permitted and the lower file is untouched) and implement `edit_file`
   the same way. Alternative: stop chmod'ing lower files `0444` and let
   `source patch` extraction decide what changed (the lower tree is
   already outside any write path; the 2026-08-21 rationale was defensive).
   Preferred: the temp+rename write in the broker (keeps the
   `makeSubjectTreeReadOnly` invariant and its test). Red (capsule).
2. Pass the Cast's referenced parent text into the engineering inbox
   (inline the parent's body into the assignment record) so `Ask` is not
   needed; or make the guard allow one answered Ask.
3. Prompt/seccomp: change `cp -a` to `cp -R --no-preserve=mode,ownership`
   (or `cp -r`), allow `sched_getparam/sched_getscheduler` and the xattr
   family read calls, and cap the cell `error` field by the *last* 2 KB.
4. Management: do not convert capsule error text into an "authorization"
   escalation; give `Escalate` a valid owner target or stop with a visible
   blocker on the operation list.

Confidence: root cause A about 85% (needs one Linux-guest confirmation: a
capsule test that `WriteFile` over a lower 0444 file fails, then passes with
the temp+rename write); B high (seen in 2 of 7 engineering assignments across
0101/0825); C medium. Ordering: fix 1, then 2/3, then re-run this drill;
expect a new failure family only after engineering can freeze.

## Heresy delta

discovered: A (existing-file edit has never worked in an engineering
capsule), B, D; introduced: none; repaired: none.

## Failure modes considered for the fix

Written before the code (repo testing rule). Scope: root cause A only
(existing lower files cannot be edited) plus the unimplemented granted verbs.
Class red (capsule filesystem contract). Rollback: git revert of the fix
commit. Chosen fix: when `write_file` hits a permission error on an existing
regular file, write a temp file in the same directory and `rename` it over the
target. The lower tree and `makeSubjectTreeReadOnly` stay as they are
(`0444` files, `0777` directories, the 2026-08-21 invariant).

Confirmation of the cause from code (read, not yet on a Linux overlay): the
write the cell issues lands in `Broker.handleWriteFile`
(`internal/yaegikernel/broker.go`, `os.WriteFile(cleanPath, ...)`; the ledger
error text `write file failed: open ...: permission denied` is this
function's), and the host-side `capsule` broker has a second copy
(`cmd/capsule-broker/main.go`, `handleWriteFile`). Both call `os.WriteFile`,
which opens the existing `0444` lower file for writing.

Ways the fix could fail, and the intended handling:

1. **Existing read-only file** (the bug): the fallback replaces it. Test:
   `0444` file in a `0777` dir, written as non-root: before the fix `EACCES`,
   after the fix new content and the old inode untouched.
2. **Writable existing file keeps its semantics.** The fallback runs only on
   a permission error, so ordinary writes (and hardlinks, ownership) behave as
   before. Test: a `0644` file is rewritten in place (same inode).
3. **Target is a symlink.** Renaming over it would replace the link, not its
   target. The fallback refuses symlinks and returns the original error.
   Test.
4. **Target is a directory.** `os.WriteFile` fails with `EISDIR`, not a
   permission error, so the fallback is not entered. Test.
5. **Parent directory not writable.** The temp file cannot be created; the
   original permission error is returned and the target is untouched.
   (Lower directories are `0777` by design, so this is the abnormal case.)
6. **Failure between temp create and rename** (write error, rename error,
   crash). The temp is removed on error; a crash could leave a
   `.choir-write-*` file, which would appear in the capsule's source patch.
   Accepted: same-directory temp is what keeps the rename atomic, and a
   crashed capsule is destroyed.
7. **Mode.** A replaced `0555` file must stay executable and the result must be
   writable by its owner, so later writes take the plain path. With no
   explicit mode the new file takes `existing perm | 0200`; an explicit mode is
   honoured. Test both.
8. **Rename across layers.** Source and target are in the same merged
   directory; overlayfs renames an upper-only file over a lower file without a
   whiteout for the target. Needs a Linux overlay to prove; the environment
   cannot, so this is staging evidence (below), with a Linux-container check
   when available.
9. **Landlock/seccomp.** The workload Landlock domain grants `MAKE_REG`,
   `REMOVE_FILE` and `REFER` on the merged dir (`internal/capsule/landlock.go`)
   and the seccomp allowlist has `rename`, `renameat`, `renameat2`, `fchmod`
   (`internal/capsule/seccomp.go:18`). No new permission is needed. Not run
   under real Landlock here.
10. **Path confinement.** `resolveSafePath`/`resolveWithin` still validate the
    path; the temp name is a fixed prefix plus random digits created in the
    validated file's own directory, so it cannot escape.
11. **Two implementations drift.** Both write handlers must share one helper.
    Put it in `internal/yaegikernel` (which `capsule` and the broker already
    import) and call it from both.
12. **Granted but unimplemented verbs.** `roles.go` grants `edit_file`,
    `rename`, `chmod` to engineering and no handler exists, so a capability
    for them passes the role check and then fails `unknown verb`. No prompt or
    Go surface teaches them (`choir.Rename/Chmod/EditFile` do not exist;
    repo-wide search found no use outside `roles.go` and its test).
    Decision: remove the three from the grant rather than implement them (the
    replace-write plus `Exec` cover the need). The other granted-but-
    unimplemented verbs (`lstat`, `readlink`, `mkdir_all`, `remove_all`,
    `symlink`, `truncate`, `file_hash`) are left as a named residual. Test:
    `roles_test.go` asserts the three are absent.
13. **Tests run as root.** Root bypasses mode bits, so the permission tests
    skip when `os.Geteuid()==0`.
14. **Not addressed (separate causes):** spec-by-reference casts (B), `cp -a`
    and seccomp release-copy hostility (C), no owner escalation target (D).

## Residual 12, the seven granted verbs without handlers: decision (2026-10-15 19:20 UTC)

Re-checked from code at 3024946a4.

- Engineering is granted `lstat`, `readlink`, `mkdir_all`, `remove_all`,
  `symlink`, `truncate` and `file_hash`. Research is granted `lstat`,
  `readlink` and `file_hash`.
- Neither broker handles any of them. The host broker's verb switch
  (`cmd/capsule-broker/main.go`, `exec` through `job_*`) has no case for
  them, and the guest broker (`internal/yaegikernel/broker.go`) has
  handlers only for exec, read_file, write_file, list_dir, message, emit
  and egress.
- No Go or prompt surface names them outside `internal/capsule/roles.go`.
  The other matches are syscall names in `seccomp.go` and file-type
  strings.

Decision, the same as for `edit_file`, `rename` and `chmod`: **delete the
grants; don't implement the verbs.** `exec` (`ls -l`, `readlink`,
`mkdir -p`, `rm -rf`, `ln -s`, `truncate`, `sha256sum`) covers every one
of them, under the same capsule confinement.

Ceremony (red: the capsule capability surface):

- **Conjecture delta.** A capability names only verbs that can succeed.
  Before, these grants turned a role check into a later `unknown verb`.
  After, they are refused at the role check.
- **Protected surfaces.** `internal/capsule/roles.go` `RoleVerbSets`
  only. No broker, seccomp or executor change.
- **Admissible evidence.** `roles_test.go` asserts that every granted
  verb is in the set the brokers handle, and the focused capsule tests
  pass. The deployed proof is a go-served tracer reaching restore, which
  needs engineering's real verbs (exec, write_file, job_*, go_eval) to
  keep working.
- **Rollback.** git revert of the fix commit.
- **Heresy delta.** Discovered: none new (the residual was named
  earlier). Introduced: none. Repaired: grants without handlers.

### Deployed proof (bb83f3a7e, deployed 19:17:02 UTC): go-served-1920

`node scripts/reeng_tracer.mjs --label go-served-1920 --scenario engineering --change go-served --expect-sha bb83f3a7e… --timeout-min 60 --apply-min 30 --restore-min 12`.
The tracer minted `computer-e2c2e019…` and stopped it (201). Evidence:
`~/.config/choir-qa/traces/go-served-1920/`.

- **`pass=true`, `reached=restore`, 18 of 18 checks.** The go test
  evidence was a job: the completing assignment's 3 jobs were all done.
- **Verb refusals across all run events: 0** (`unknown verb` or
  `capsule operation refused`).
- **Engineering took three assignments.**
  - The first, `90085d0f…`, failed at 19:30. Three things went wrong:
    its own compile error (`undefined: json`, the same slip as
    go-served-1143); one `failed to read broker response: … broker.sock:
    i/o timeout`; and a malformed `choir.Complete` call (`assignment
    mismatch: 2 variables but … returns 1 value`), so the run ended
    `failed`.
  - The second, `cee0c731…`, also ended `failed`, at 19:33:48.
  - The third, `b948d017…`, completed, froze, was verified, applied at
    20:05:14 and restored at 20:05:34.
- **New residual: the broker read-timeout class still occurs.** It was 1
  in this run, after the a64f28e1f re-dial fix. Counted, not chased.
