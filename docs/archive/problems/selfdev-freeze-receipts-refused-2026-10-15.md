# Self-Development Drill selfdev-ab-2239: Freeze Refused Two Receipts Cited as Three

Date: 2026-10-15 (drill run 2026-10-14 22:39-23:09 UTC). Drill:
`node scripts/reeng_tracer.mjs --label selfdev-ab-2239 --scenario engineering`
on staging build `1ad29fd2a` (exit 6, the scenario's claim failed). That
build carries the capsule build fixes from
[selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md](selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md)
(Freeze reuses the running binary when no Go input changed; gcc and the ICU
cgo env in capsules; 2 GiB capsules). Evidence: owner trace bundle
`~/.config/choir-qa/traces/selfdev-ab-2239/` (`receipt.json`, and
`engineering-run-events-1-88.json`, the engineering run's 88 events read
through `GET /api/runs/{id}/events`). Test computer
`computer-40ac60a339719d7142b0041ad535eab0`: minted and stopped by the
tracer; started once at 23:13 to read the ledger through product routes and
stopped again at 23:19. No SSH, no `/internal/*`.

Status: documented; fix follows in a separate commit on branch
`fix/selfdev-freeze-receipts` (not pushed, not deployed).

## Outcome

The operation opened (`selfdev-4e52a266fc4a7626039674deeae696c5`, state
`executing`) and stayed there with `last_intent_error`
`freeze: distinct build, test, and dependency/toolchain execution receipts
are required`. Engineering's final Report: `stop_reason: failed`, no
bundle. The tracer timed out at `decision_boundary_not_reached`.

## Timeline (UTC; run events, not inference)

Assignment `assignment-5c76fa5306048ded02ee872c`, run
`run:assignment-5c76fa5306048ded02ee872c`. The model failed over from
`cat-jam-free` (circuit open) to `gpt-6-luna`, reasoning effort `low`,
before its first turn. "Cell n" counts capsule_go_eval calls in order.

| time | cell | what it did | result (receipt) |
|---|---|---|---|
| 22:40:30 | 1 | `choir.Records()`: the Cast says "Implement the requested single static frontend file, run its exact-content check, build the release tree, and freeze the candidate." | ok |
| 22:40:33 | 2 | Precommit (tests_pass, p 0.9); `choir.WriteFile("/workspace/platform/frontend/public/choir-drill/choir-drill-7f1daed4.txt", ...)`; `choir.Exec("test", ["$(cat ...)", "=", ...])` (no shell, so the literal string is compared) | wrote 21 bytes; test exit 1 |
| 22:40:35 | 3 | the check through `sh -c` with a relative path | `cat: frontend/public/...: No such file or directory` |
| 22:40:37 | 4 | WriteFile again; the same literal `test` | exit 1 |
| 22:40:40 | 5 | `choir.Exec("pwd")` prints `/`; `sh -c 'mkdir -p frontend/public/choir-drill && printf ... > frontend/... && test ...'` | check 0 (`ae4ae002...`), but against a copy at `/frontend/public/...`, outside the workspace |
| 22:40:42 | 6 | `go build -o /var/lib/artifact/release/bin/autoputer ./cmd/autoputer` (from `/`), then copy `frontend/dist` | exit 1: `go: go.mod file not found` |
| 22:40:47 | 8 | `cd /workspace/platform && go build ... ./cmd/autoputer` | exit -1 after 30.0 s |
| 22:41:19 | 9 | the build in `( ... ) &` | exit -1 after 30.0 s |
| 22:41:52 | 10 | the build under `nohup sh -c "..." >/dev/null` (no trailing `&`) | exit -1 after 30.1 s |
| 22:42:24-33 | 11-13 | poll the background build | "running", empty log, no status file |
| 22:42:35 | 14 | the build again, foreground | exit -1 after 30.1 s (`ffc67f04...`) |
| 22:43:07 | 15 | `mkdir -p /var/lib/artifact/release/{bin,frontend}` | ok |
| 22:43:10 | 16 | **`choir.Freeze("ae4ae002...", ["ae4ae002..."], ["ffc67f04..."])`** | `tool_error: reduce: persist tray-1: distinct build, test, and dependency/toolchain execution receipts are required` |
| 22:43:12 | 17 | the build again | exit -1 after 30.0 s |
| 22:43:44 | 18 | `go test ./cmd/autoputer` | exit -1 after 30.1 s |
| 22:44:16 | 19 | `go env GOMOD` | `/workspace/platform/go.mod` |
| 22:44:18 | 20 | the build again | exit -1 after 30.1 s |
| 22:44:52 | 21 | `choir.Complete("blocked", "none", "... Blocked from freezing: the required Go release build repeatedly hit the capsule's 30-second command timeout and produced no release binary.", ...)` | final Report, `stop_reason: failed` |

## Answers from the ledger

### 1. What engineering did

- **Edit:** yes, through `choir.WriteFile` into
  `/workspace/platform/frontend/public/choir-drill/choir-drill-7f1daed4.txt`
  (cells 2 and 4). No other workspace write.
- **Requested check:** its only exit-0 receipt (`ae4ae002`, cell 5) ran
  from `/` and tested a second copy the cell itself wrote at
  `/frontend/public/...` at the capsule root, not the candidate file.
  `choir.Exec` runs at the capsule root, not `/workspace/platform` (`pwd`
  printed `/`). The previous drill hit the same thing (its first `npm ci`
  failed for want of a `package-lock.json`).
- **bin/autoputer:** it tried to build it seven times (cells 6, 8, 9, 10,
  14, 17, 20) although the change is `frontend/public/` only and the
  overlay it ran under says, for a change that touches no Go input, "do
  not build bin/autoputer ... Freeze stages this computer's running
  binary". The system prompt was 17,660 characters; the engineering overlay
  (10.6 KB) is written unconditionally for the engineering profile
  (`internal/agentcore/tool_profiles.go`), so that sentence was in context.
  The overlay leads with "The release tree holds (1) bin/autoputer and (2)
  frontend/", and the Cast and the request both say "build the release
  tree".
- Every build after cell 6 ended at `choir.Exec`'s fixed 30 s bound
  (`yaegikernel` broker `DefaultTimeout`; `ChoirScope.Exec` sets no
  per-call timeout), exit -1. The two background attempts were malformed:
  in cell 9 the `( ... ) &` subshell kept `choir.Exec`'s stdout pipe open,
  so the call waited for it and the timeout killed the process group; cell
  10 had no `&`.
- **Frontend:** never built. `frontend/dist` was never produced and
  nothing was copied into `/var/lib/artifact/release/frontend/` (cell 6's
  copy was behind a failing `go build` under `set -e`). In the previous
  drill `npm ci` and `npm run build` took about 5 s and 12 s (cell wall time).
- **Tests:** none succeeded. The one `go test` (cell 18) timed out.
- **Freeze arguments:** `build_recipe_ref` = `ae4ae002` (the cell-5
  content check), `test_receipts` = [`ae4ae002`] (the same receipt),
  `dependency_toolchain_refs` = [`ffc67f04`] (a timed-out build, exit -1).
  Two distinct refs for three slots.

### 2. Is the refusal right?

The check: `freezeCapsuleEffectBundle`
(`internal/agentcore/tools_capsule.go`) resolves the union of the three
slots through `Executor.ResolveGrantedExecutionReceipts`
(`internal/capsule/executor.go`). That function skips a ref it has already
seen, refuses any ref that does not bind this run, handle, capsule and
source, and refuses when the latest cited receipt did not observe the
frozen worktree. Freeze then refuses fewer than three resolved receipts.

- **Same receipt twice: yes.** `build_recipe_ref` and `test_receipts[0]`
  are the same receipt. That is the only reason the count was 2. Both refs
  opened and bound, otherwise the error would name the binding.
- **Receipts from before the last write: no.** The digest covers
  `/workspace/platform` only (`digestCapsuleWorktree`); the last workspace
  write was cell 4; the latest cited receipt (cell 14) observed the frozen
  tree, or the binding error would have fired first.
- **Dependency/toolchain receipt omitted: no**, but the one cited is a
  build killed at its timeout. The check counts distinct receipts and does
  not look at exit codes or at what a cell ran.
- **Is the check wrong for the reuse path? No.** Reuse removes the Go
  build, not the build. `StageGrantedRelease` still needs `frontend/` in
  the release tree ("computer-surface frontend artifacts are underivable"
  without it), so a frontend-only change always has a build cell: `npm ci`,
  `npm run build`, copy `dist/` into `/var/lib/artifact/release/frontend/`.
  That cell is the `build_recipe_ref`. Had the desk cited three distinct
  receipts, Freeze would still have failed at the next check: the release
  tree was empty ("capsule release contains no frozen runtime artifacts").
  The refusal was right. Its text is not useful: it does not say which
  slots share a receipt, or what each slot should cite.

This refusal is not new. Probe 6 of
[s0-selfdev-executing-wedge-2026-10-04.md](s0-selfdev-executing-wedge-2026-10-04.md)
hit the same text, and that doc's "Full contract for a successful freeze"
says "Three DISTINCT ... receipt_refs". The overlay never took that up. It
says only "each a capsule-go-eval:sha256: receipt ref", and the in-cell
surface says "each argument is a capsule-go-eval receipt ref (or list of
them)".

### 3. Why "failed" two minutes after the first refusal

- **What it saw:** `tool_error: reduce: persist tray-1: distinct build,
  test, and dependency/toolchain execution receipts are required`, with no
  slot, ref or fix named.
- **What it did:** read the error as "a real build receipt is missing" and
  went back to building (cells 17, 18, 20: all timed out). It never called
  Freeze again. At 22:44:52 it applied the overlay's blocker rule, "If the
  release tree cannot be built (a refused download, a failing build), that
  is a blocker: finish with choir.Complete result="blocked"".
  `reducerAssignmentOutcome` (`internal/store/engineering_assignments.go`)
  maps `blocked` to the failed disposition, hence `stop_reason: failed`.
- **Budget:** not the cause. The tool loop has no iteration cap
  (`internal/toolregistry/toolloop.go`, `for i := 0; ; i++`). The run
  stopped at iteration 22 with about 14k input tokens per turn, and the
  drill's window had about 24 minutes left. The desk stopped by choice, applying
  a rule that fit what it believed: that it had to build Go.
- **Retry rule:** the overlay says "A failed freeze restores the capsule
  (thawed, retry-safe) — fix the precondition and retry". The desk could
  not tell from the error which precondition to fix.

### 4. Substrate or symptom

**Symptom over a correct substrate, with one substrate residual.**

- The Freeze check, the receipt binding and the reuse path did what they
  are specified to do. Nothing in the run points at them.
- The failure is at the contract's surface:
  1. The overlay presents the release tree as "bin/autoputer and
     frontend/" before the no-Go exception, and gives no recipe for the
     frontend build.
  2. It does not say that the three Freeze slots must cite three different
     cells, or what each slot is.
  3. It does not say that `choir.Exec` runs at `/`. Its own worked example
     (`choir.Exec("go", []string{"test", "./internal/capsule/..."})`)
     fails there with `go.mod file not found`.
  4. It does not say that `choir.Exec` stops at 30 s with exit -1.
  5. Freeze's refusal names no slot.
- The residual: `choir.Exec` has a fixed 30 s bound and the desk cannot
  raise it. Any Go change still has to background a build and poll it
  (option D of the 10-14 doc). That is what turned this desk's wrong turn
  (building Go) into a dead end, but it is not needed for this drill's
  change.

**Root Cause Clustering** (three self-development drill failures in this
subsystem since 2026-10-14: c1e-b1 cancelled on arrival, selfdev-goproxy-1802
could not build `bin/autoputer`, selfdev-ab-2239 here). The common thread is
the cost of the Go build inside a capsule whose exec model is 30 s commands
in 60 s cells:

- in goproxy-1802 the build was required and could not finish;
- here it was not required, and the desk built anyway.

The substrate answer for Go changes is still option D (a long-running exec
primitive) with C (a build budget and warm `GOCACHE`). No replacement exists
unwired: `ExecPayload` has a `Dir` field and the broker honours
`BrokerRequest.TimeoutMs`, but `ChoirScope.Exec` sets neither. For changes
with no Go input the reuse path already removes the build. What was missing
is that the desk is told so, plainly, with the receipts that path produces.

## Failure modes of a fix

- F1. The overlay still lets a desk read "build the release tree" as "build
  bin/autoputer" for a no-Go change. Guard: the overlay states the Go-input
  decision first and gives the no-Go build (the frontend) as a concrete
  cell.
- F2. The overlay names Freeze's three slots but not that they must be
  three different cells, so a desk pads them again. Guard: the overlay says
  so, and the refusal names the colliding slots.
- F3. The refusal names slots wrongly (for example it blames a duplicate
  inside `test_receipts` when the three slots still have three distinct
  refs). Guard: the diagnosis is a pure function of the three slots, and
  tests cover each collision shape and the passing shape.
- F4. A changed refusal accepts what it refused before. Guard: the
  decision (fewer than three resolved receipts) is unchanged. Only the
  message changes, at the same point in the flow (after the operation
  opens and the receipts resolve), so `last_intent_error` and operation
  state behave as before.
- F5. The overlay's recipe cites a cell that ran before the last workspace
  write (the build cell writes `frontend/node_modules` and `frontend/dist`).
  Guard: the recipe removes those in the same build cell and runs the test
  cell after it.
- F6. The overlay's commands fail at `/`. Guard: every command in the
  recipe uses an absolute path or `cd /workspace/platform/...`, and the
  worked example uses `go -C /workspace/platform`.
- F7. A desk declares "blocked" at the first Freeze refusal. Guard: an
  explicit retry rule: a refusal names a precondition; fix it and Freeze
  again. Blocked is only for a precondition that cannot be met.
- F8. The background-build advice stays wrong (cells 9 and 10). Guard: give
  the exact form that detaches from `choir.Exec`'s pipes.
- F9. Overlay text drifts from the executor: the release root, the
  Go-input list, the slot names. Guard: the existing overlay tests stay,
  plus a test that the overlay teaches the distinct-receipts rule and names
  the slots by `choir.Freeze`'s parameters.

## Fix options

- **A. Overlay (yellow):**
  - restate the Freeze contract as a decision (Go input or not) with the
    receipts each path produces (no-Go: a toolchain cell, the frontend
    build cell, the check cell after it);
  - say that the three slots must be different cells;
  - say that `choir.Exec` runs at `/` and stops at 30 s with exit -1, and
    fix the worked example;
  - give the exact detached-build form;
  - add a retry rule for a refused Freeze.

  This addresses the cause seen in the trace.
- **B. Refusal text (orange, Freeze path; decision unchanged):** name which
  slots share which receipt and what each slot should cite. This makes a
  retry possible even when the overlay is ignored.
- **C. Check exit codes in the build and toolchain slots (red, semantic
  tightening):** refuse a receipt whose recorded execution failed. It would
  not have changed this run's outcome. Recorded as a residual.
- **D. A long-running exec primitive (red, capsule exec):** carried over
  from the 10-14 doc. Needed for Go changes, not for this drill.
- **E. Default `choir.Exec` to `/workspace/platform` in engineering
  capsules (orange):** changes every existing cell's relative paths.
  Deferred. The overlay tells the desk instead.

Chosen (No Blocking Asks: the most conservative option that addresses the
evidenced cause): **A + B.** C, D and E stay residuals.

Mutation class: **orange.** The overlay is prompt framing (yellow); the
refusal message is on the Freeze path, but the accept/refuse decision is
unchanged. Protected surface touched: the self-development Freeze refusal
message only. Rollback: revert the fix commit. Admissible evidence:
- unit tests for the slot diagnosis and the overlay text;
- the deployed engineering drill after deploy, which is the proof:
  `node scripts/reeng_tracer.mjs --label <label> --scenario engineering --expect-sha <sha> --timeout-min 30`.

## Also seen

- `choir.Exec("test", []string{"$(cat ...)", ...})` (cells 2 and 4):
  argv has no shell expansion. The request's check is a shell expression,
  so it needs `sh -c`.
- Model failover to `gpt-6-luna` at `low` effort for the whole run. Not a
  cause by itself; recorded because a low-effort model follows the
  overlay's first framing over its later exceptions.

## Heresy delta

- discovered: the overlay's worked example cannot run where `choir.Exec`
  runs (`/`); the overlay never carried the distinct-receipts rule from the
  2026-10-04 contract.
- introduced: none.
- repaired: none in this commit.

## Fix (A + B, branch `fix/selfdev-freeze-receipts`, not deployed)

- **A, overlay** (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`):
  - says where `choir.Exec` runs (`/`), that it is killed after 30
    seconds with exit code -1, and that argv is not shell-expanded;
  - asks the Go-input question first;
  - for a change with no Go input, says "do not run go build at all" and
    gives the frontend build cell (`npm ci`, `npm run build`, copy `dist/`
    into the release tree, remove `node_modules` and `dist`);
  - for a Go change, gives the detached build form;
  - says Freeze's three arguments come from three different cells,
    names each, quotes the refusal, and gives the cell order (toolchain,
    build, test);
  - adds the retry rule ("A refused Freeze is not a blocker");
  - fixes the worked example to `go -C /workspace/platform test`.

  The in-cell surface summary for `Freeze` says "from three different
  cells". `yaegikernel.DefaultExecTimeout` names the broker's 30 s bound,
  so the overlay test pins the number to the code.
- **B, refusal text** (`internal/agentcore/tools_capsule.go`,
  `freezeReceiptSlotsError`): keeps the original sentence and appends
  which slots cite the same receipt (for this drill: "build_recipe_ref and
  test_receipts[0] cite the same receipt capsule-go-eval:sha256:ae4ae002…")
  and what each slot should cite. The decision is unchanged and happens at
  the same point.
- Tests (written before the change):
  - `internal/agentcore/freeze_receipt_slots_test.go`: failure modes F2,
    F3, F4;
  - `TestRLMEngineeringOverlayTeachesFreezeReceiptsAndExecLimits` in
    `internal/runtimeprompts/prompts_test.go`: F1, F2, F5-F9.
- Proof is the deployed drill:
  `node scripts/reeng_tracer.mjs --label <label> --scenario engineering --expect-sha <sha> --timeout-min 30`.

Residuals:
- C: a failed or timed-out receipt can still fill the build or toolchain
  slot. Addressed on branch `feat/capsule-long-exec` (not pushed): Freeze
  refuses a cited receipt with a non-zero exit code or a job that did not
  end done, in every slot.
- D: no long-running exec primitive for Go changes. Addressed on the same
  branch: capsule jobs (`choir.Start`, `choir.Poll`, `choir.Stop`),
  [capsule-long-exec-2026-10-15.md](../../design/capsule-long-exec-2026-10-15.md).
- E: `choir.Exec` still defaults to `/`.
- The management Cast and the drill request still say "build the release
  tree". The overlay now says what that means for each path.
