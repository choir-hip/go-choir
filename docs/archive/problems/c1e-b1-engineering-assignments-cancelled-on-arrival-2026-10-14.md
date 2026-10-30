# C1e B1: Engineering Assignments End "Cancelled" Seconds After Management's Cast

Date: 2026-10-14, 05:20 UTC. Found by the B1 engineering drill
(`node scripts/reeng_tracer.mjs --scenario engineering`, authored tonight
as C1e step 4, commit 6ae126b9 on branch worktree-agent-ab80b7882f2ad21aa,
staged for the next push). Deploy: staging e9d727a7 (C1e B1 + the
homepage change). Status: first cause (the capsule gate) fixed in
bc09dcc1, deployed as 11a94193; the drill after it found three more (see
the last section).

## What the drill saw

Test computer `computer-d8ad5936bca79ed2668330c68419c5db` (build
e9d727a7, self-development mode set to propose_only), label
`c1e-b1-engineering-0444`, receipt in
`~/.config/choir-qa/traces/c1e-b1-engineering-0444/`.

| time (UTC) | phase |
|---|---|
| 04:46:35 | the owner request is submitted through the prompt bar |
| 04:46:45 | Texture Casts to management (round 1) |
| 04:47:12 | management Casts engineering on records: assignment-703ad944..., parent is Texture's Cast |
| 04:47:22 | engineering's final Report: `stop_reason: cancelled`, record id `assignment-report:cancel-report:...`, no operation, no bundle digest |
| 04:58:02 / 04:58:13 | a second Cast (assignment-afc3c5a7...), cancelled 11 s later |
| 05:00:05 / 05:00:16 | a third Cast (assignment-979023d5...), cancelled 11 s later |
| 05:16:35 | the drill times out: no Freeze, no Verify, no decision |

So B1's first half works on staging: management casts engineering on
records, with the right parent. The second half doesn't: each assignment
ends with a cancel report about 10 seconds later, and engineering never
freezes.

## Candidates (hypotheses until the ledger says)

- B1 named these open points (station now card, 01:50 UTC):
  `stopCancelledAssignmentRun` is new; "the reconcile and the run-end
  settle can each mint the final Report". A reconcile that sees no
  engineering run yet, or a run that ended at once, may mint the cancel
  report.
- The engineering run may fail to start or end at once (capsule, mode,
  or module surface), and the run-end settle reports it as cancelled.
- Management may cancel its own Cast on its next turn (B1 gave management
  a cancel).

The next step is to read the trace: the assignment rows, the engineering
runs and their events, and who wrote each cancel report.

## Trace findings (2026-10-14, 05:18 UTC)

Read from the test computer's ledger (started 05:17:55, stopped 05:18:26;
the start was a crash boot, which closed the three Texture runs with
`interrupted_by_restart` and did not touch the engineering evidence).

**The engineering run starts every time; every capsule cell is refused.**
All three runs (`run:assignment-703ad944...`, `run:assignment-afc3c5a7...`,
`run:assignment-979023d5...`) have the same 21 events:

| seq | event |
|---|---|
| 2 | the model calls `capsule_go_eval` (first cell: `choir.Records()`) |
| 4 | `tool.result` is_error: `assigned capsule obligation is no longer executable: assigned work obligation is not open: record not found` |
| 6 | end_turn: "the capsule runtime reports that assignment ... is no longer open" |
| 7 | `loop.retry` reason `engineering_overlay_terminal_fate_required` |
| 11, 18 | the same refusal on the next two cells (`choir.Help("Complete")`, `choir.Records()`) |
| 14 | second `loop.retry` |
| 21 | `loop.cancelled`: completion guard not satisfied after 2 retries |

Run 703ad944: created 04:47:10.291, finished 04:47:19.946 (cancelled). The
run metadata carries `assigned_conversation_id`, `parent_round` and the
capsule id, and no `assigned_work_item_id`, as a records run should.
`/api/runtime/obligations` holds one work item (Texture's,
`to: texture:9383da12...`); no engineering work item exists.

**Who minted the cancel Report.** The final Report in management's inbox
(seq 2, 04:47:19.946, `assignment-report:cancel-report:sha256:be8686...`)
has the claim `tool loop: completion guard "engineering_overlay_terminal_fate_required"
was not satisfied after 2 retries; assignment cancelled; late results are
evidence-only and cannot reopen work`. That reason text is the run's own
error, so the path is the run's failure path, not the reconcile, the
restart, or management: `handleExecutionError` (runtime.go:4221) ->
`terminalizeRun(reason = err)` -> `cancelBoundEngineeringRun`
(engineering_assignment_fate.go:347) -> `persistSystemEngineeringCancellation`
-> store `CancelEngineeringAssignment`, which mints the cancel-report
(engineering_assignments.go:2020, 2065). Seqs 4 and 6 are the same for
the other two assignments. Management's runs call `choir.Cast`,
`choir.Records`, `choir.Judge`, `choir.Note`, `choir.ReportPacket`, and
never `choir.Cancel(`. Hypothesis 3 (management cancels its own Cast) is
ruled out; hypothesis 1 (the reconcile cancels a fresh assignment) is
ruled out; hypothesis 2 is right in a specific form: the run starts, and
the capsule gate refuses it.

**Cause.** `validateAssignedEngineeringExecution`
(internal/agentcore/engineering_assignment_tools_overlay.go:100-148) is the
`ValidateCurrentObligation` that `requireCurrentAssignedCapsule`
(tools_capsule.go:93) runs before every capsule cell. B1 (a06191f9) left
it on the pre-records shape: it loads
`GetLifecycleWorkItem(assignment.Binding.AssignedWorkItemID)` and requires
an open work item assigned to the run's agent. A records binding carries
no work item by construction (`EngineeringAssignmentBinding.Validate`
refuses one; `buildAssignedEngineeringRun` sets none), so the lookup is
`GetLifecycleWorkItem("")` -> `ErrNotFound` on every cell. The engineering
desk can never run a cell, so it can never Freeze or Complete, and the
completion guard ends the run.

**Why the B1 tests missed it.** Every test that builds an engineering
`CapsuleToolCtx` stubs the validator (`ValidateCurrentObligation: func(...)
error { return nil }` in rlm_reduce_test.go:53 and
selfdev_freeze_base_head_test.go:172); no test runs the real validator
against a records assignment. The store and run-end paths were ported
(store `projectEngineeringTerminalRun` branches on `Binding.Records()`);
this one runtime gate was not.

**Substrate vs symptom.** Symptom of an incomplete port, not a substrate
fault: the records model is right and its authority (the assignment row,
the engineering conversation's round 1) exists; one gate still reads the
deleted work-item authority. The records equivalent of "the assigned work
obligation is open" is: the assignment's engineering conversation
(`management:<owner>|engineering:<assignment>`) has an open round whose
recipient is the run's agent. That is what `planEngineeringReportRecord`
already reads before minting a Report.

Other residual pre-records reads of `AssignedWorkItemID` (not on this
drill's path; to be checked once engineering reaches them):
`texture_lifecycle_api.go:569` (producer-report authority compares
`ProducerWorkItemID` to it; both "" on records, so equal),
`engineering_evidence.go:361/485/556` (report events matched by
`WorkItemID`, "" on records both sides), and
`resumeStrandedFrozenAssignmentCommit` metadata.

## After the capsule-gate fix (11a94193, drill 05:49 UTC)

Label `c1e-b1fix-engineering-0549`, computer
`computer-cac0d3508302ce513126d45f215f84fd` (kept for the trace).

The gate fix works: management Cast engineering at 05:50:57, and
engineering ran in its capsule and opened self-development operation
`selfdev-11df3663aee4be09cce8f56f46018397` (executing, 05:52:15). Then
the operation failed at 05:52:59 with `freeze: capsule release contains no
frozen runtime artifacts`, and the run ended with "completion guard
engineering_overlay_terminal_fate_required was not satisfied after 2
retries". The final Report is again a cancel report, now one with an
operation behind it.

Open, to be read from the trace before any claim:
- whether engineering built the runtime artifacts (bin/autoputer, the
  frontend bundle) in the capsule before Freeze, and what Freeze requires
  for a change that only adds a static file;
- whether two retries after one refused Freeze is the intended guard,
  given that a refused Freeze is recoverable (build, then Freeze again);
- whether this freeze precondition predates B1 (the cut-1 drill's earlier
  passes, if any, ran on the packet path).

## Trace findings after the gate fix (2026-10-14, 06:00 UTC)

Read from the kept computer through product routes only: `/api/runs`,
`/api/runs/run:assignment-c05fd947.../events` (118 events), the
management and engineering inboxes, and the operation and its receipts.

**What engineering ran.** All in `capsule_go_eval` cells (model
gpt-6-luna, effort low):

| seq | cell | result |
|---|---|---|
| 11-16 | write `frontend/public/choir-drill/<token>.txt`, run the check | check exit 0 |
| 19-24 | `go build -o bin/autoputer ./cmd/autoputer` in `/workspace/platform` | exit 1: every module zip `Get https://storage.googleapis.com/proxy-golang-org-prod/...: Forbidden` |
| 43-48 | `npm ci && npm run build` in `frontend/` | exit 0 (`frontend/node_modules`, `frontend/dist` now in the workspace) |
| 51-64 | `mkdir -p var/lib/artifact/release/...` **after `cd /workspace/platform`**; copy `frontend/dist`; write a two-line shell stub as `bin/autoputer` | exit 0 |
| 67-68 | Freeze #1 | refused: `capsule execution evidence does not bind the final frozen subject` (the latest cited receipt was the frontend build at seq 48, before the release-tree writes) |
| 75-76 | Freeze #2, latest cited receipt a read-only check after the writes | refused: `capsule release contains no frozen runtime artifacts`; the operation opens here (05:52:15) and records this as `last_intent_error` |
| 79-80 | `go build -o var/lib/artifact/release/bin/autoputer` | same Forbidden module downloads |
| 83-85 | `choir.Complete("partial", ...)`: "could not build the Go runtime; no stub substituted" | accepted: non-final Report at management seq 2 |
| 87-88 | end_turn | guard retry 1 (partial keeps the assignment open, by design) |
| 99-100 | `choir.Complete("failed", ...)` | refused at reduction: `persist reconstructable content-addressed candidate: subject copy refuses escaping symlink "frontend/node_modules/.bin/acorn"` |
| 103-115 | three more cells | each refused: `assignment fate is not active and bound` |
| 107, 118 | guard retry 2, then `loop.cancelled`; cancel Report at management seq 3; the operation fails with the guard's text |

**Q1: did it build runtime artifacts, and what did its surface say?**
It built the frontend bundle and could not build `bin/autoputer`; it
wrote a shell stub, then declined to rely on it. The engineering overlay
(`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`, since
89970ebd9, 2026-09-27) tells it: "before calling Freeze, the cell must
have produced var/lib/artifact/release/ **inside /workspace/platform**
with (1) an executable bin/autoputer and (2) frontend/ artifacts — build
them in-capsule (go build ./cmd/autoputer -> bin/autoputer ...)". The desk
followed that sentence exactly, and the sentence is wrong.

**Q2: what Freeze requires.** `StageGrantedRelease`
(`internal/capsule/executor.go:1385-1600`) takes only capsule diff paths
under `var/lib/artifact/release/` relative to the capsule upperdir, i.e.
`/var/lib/artifact/release/` at the capsule root (`walkUpperdir` paths
are upperdir-relative; `release_secret_test.go` builds the tree at
`<merged>/var/lib/artifact/release`). Files the desk wrote under
`/workspace/platform/var/lib/artifact/release/` have diff paths starting
`workspace/platform/` and are not release files, so the list is empty and
line 1585 refuses with "no frozen runtime artifacts". Had the tree been at
the root, the gate would then require an executable `bin/autoputer` and a
`frontend/` file (1588-1602). A frontend-only change still needs
`bin/autoputer` today: the materializer mints a plain file release from
the frozen runtime files and the updater's `current` swap moves the exec
and the served frontend together (S2-f residual: patch-carrying bundles
are not yet rebuilt by the host builder). For a frontend-only change the
right binary is the unchanged one. The gate accepts any executable file
as `bin/autoputer`, the stub included.

**Q3: the guard.** The refused Freeze was recoverable and was recovered
from: a Freeze refusal is a tool error inside the turn, the capsule
stays thawed, and the desk ran four more cells. The completion guard
(`runtime.go:3496-3511`, `maxCompletionGuardRetries = 2`,
`toolloop.go:103`) counts only end_turns without a terminal fate, and is
unchanged between `reeng-c1e-pre-b1` and HEAD (the B1 diff to
runtime.go touches the reconcile kick, not the guard). Its two retries
went to (1) the desk ending its turn after a `partial` Complete, which
the overlay documents as non-terminal, and (2) a stranded assignment: the
terminal `Complete("failed")` first moved the capsule to freeze-requested
and froze it (`engineering_assignment_fate.go:865-930`), then
`PersistGrantedCandidate` failed on the symlink, so the assignment sat
`frozen` with no Report and every later cell was refused by the capsule
gate. No retry count recovers that; the guard's cancel is what closed
the assignment with a Report. The guard is not the cause.

**Q4: does the precondition predate B1?** Yes. The release-root contract
is recorded in
`docs/problems/m11-freeze-secret-gate-refuses-genuine-binary-2026-09-27.md`
(supporting finding 1: "staging under the subject path yields capsule
release contains no frozen runtime artifacts") and in
`docs/archive/problems/s0-selfdev-executing-wedge-2026-10-04.md` (probe 8 hit
this exact error; probe 9 froze with the tree at `/var/lib/artifact/release/`).
M11 reruns 7-13 (2026-10-10, packet path) froze with "52 files at
var/lib/artifact/release, executable bin/autoputer"
(`docs/evidence/m11-rerun-2026-10-10T14-46-02Z.json`). So the code
contract was met before B1 by desks that found the root path themselves;
the overlay has carried the wrong path since the day the contract was
found. No cut-1 or C1e drill had reached Freeze before this one (cut-1
and cut-2 breakage docs record no freeze).

**Three causes, one of them new to this drill:**

1. *Prompt (the Freeze refusal).* The overlay names the release root as
   inside `/workspace/platform`; the code reads it at the capsule root.
   Green/yellow fix: correct the sentence.
2. *Capsule subject copy (the stranding).* `copyCanonicalSubjectTree`
   (`internal/capsule/source_snapshot.go:143-153`) treats a symlink as
   escaping when its link text starts with `..`, without resolving it
   against the link's directory. `frontend/node_modules/.bin/acorn ->
   ../acorn/bin/acorn` stays inside the tree and is refused. Any desk
   that runs `npm ci` in the workspace cannot commit a terminal fate, and
   the refusal arrives after the terminal freeze, which strands the
   assignment. The two source-snapshot checks (lines 296-300, 360-364)
   use the same lexical test.
3. *L2 egress (the build).* `ecosystemHosts`
   (`internal/capsule/egress.go:61`) allows `proxy.golang.org` and
   `sum.golang.org`, but proxy.golang.org answers module zip requests
   with a redirect to `storage.googleapis.com`, which the proxy refuses
   (403, "not on the L2 ecosystem allowlist"). No Go module that is not
   already cached can be downloaded in an implementation capsule, so
   `go build ./cmd/autoputer` cannot succeed there, whatever the prompt
   says. Widening the list is a reviewed red-class change (allowing all of
   `storage.googleapis.com` admits every GCS bucket); the design's own
   route is a deps cache (`GOPROXY=file:///deps/goproxy`,
   `docs/design/engineering-network-grants-2026-10-10.md`). Not fixed
   here; named as the next blocker for the drill.

Also seen, not on the failure path: build outputs (`frontend/node_modules`,
`frontend/dist`) left in `/workspace/platform` become part of the frozen
subject and the source patch, because the subject is the whole tree and
does not read `.gitignore`.

**Fixed on branch `worktree-agent-aa28287fc393b47d4` (not pushed):**
c85af393 (cause 2: `symlinkEscapesTree` resolves a link against its own
directory; the subject copy and both source-snapshot checks use it) and
123ffa05 (cause 1: the overlay names `/var/lib/artifact/release/` at the
capsule root, pinned by a test to `capsule.ReleaseTreePrefix`).

**Residuals, in the order the next drill will meet them:**
- Cause 3 (Go module zips refused by L2 egress) is open. Until it is
  decided, a desk following the corrected overlay cannot build
  `bin/autoputer` and should end `blocked`; the drill will not reach a
  frozen bundle. Options: allow `storage.googleapis.com` (broad), or land
  the design's deps cache route. Red class; needs review.
- The freeze gate accepts any executable `bin/autoputer`, a stub
  included; the desk declined to rely on its stub, the gate did not.
- A terminal Complete whose candidate persistence fails after the
  terminal freeze still strands the assignment until the run ends; the
  cancel then closes it, and the desk's own failure Report is lost.
- `choir.Exec` does not run in the cell's `cwd` (`/workspace/platform`);
  relative commands failed at seq 12 and 20, and the overlay's worked
  example `choir.Exec("go", {"test", "./internal/capsule/..."})` would
  fail the same way.

## After the Freeze fixes (9ace75e5, drill 06:48 UTC)

Label `c1e-d4-engineering-0648`, computer
`computer-4687411d0a313c0a2c3b78e5c08a21b2` (stopped). Management Cast
engineering five times; each assignment ended with a real final Report,
`stop_reason: failed` (no more cancel reports), naming the one blocker:
`go -C /workspace/platform build -o /var/lib/artifact/release/bin/autoputer
./cmd/autoputer` fails because module zips redirect from proxy.golang.org
to storage.googleapis.com and return 403. Engineering correctly did not
Freeze. The engineering-on-records path works up to the release build;
the build needs the owner's decision
(docs/reports/capsule-go-module-proxy-design-2026-10-14.md).

## After the Go module proxy (865dc152, drill 18:02 UTC)

Label `selfdev-goproxy-1802`. Cause 3 is repaired on the product path:
the capsule fetched all 171 modules for `go build ./cmd/autoputer` from
the host proxy (513 hits, 0 misses). Freeze still not reached: the
capsule has no C compiler or ICU cgo environment on its PATH, and with
1 CPU and 1 GiB the build was still linking when the 30 min drill ended.
Recorded in
[selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md](selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md).
