# FP4 engineering admission queue: deployed proof on a test computer (2026-10-22)

Run by the self-development monitor (lane/fp4-selfdev-monitor) for FP0,
on deploy J (a84b39bac, FP4 round 2). The target was the test computer
`computer-71afbf1461dffabcce8eeebe558d03bd` (vm-c5c801f8…, warmness
`test`), and no other computer. Calls went through the product path,
`https://choir.news`, with the test computer's key. The host-internal
token was sent to no guest address. There was no SQL and no tape edit,
and no apply was approved. The drill is the `next_action` of
docs/definitions/choir-launch-fp4-selfdev-architecture-2026-10-21.md.
Judged steps use `scripts/proofs/fp4-selfdev-admission.sh` where it has
a step, and otherwise the rows, events and trace.

## Plan and prompts (written before any cast)

- A is held live for at least five minutes with a `sleep 300` job, then
  `go vet`, then a normal release build and a Freeze. A should end
  `completed` with its candidate `awaiting_approval`, and is not
  approved.
- B is cast while A is live. It should queue behind A ("waits behind"),
  bind within seconds of A's terminal_at (the nudge), and then make one
  cheap Freeze. With A's candidate open, that freeze should be refused
  at the operation open ("cannot freeze until the open candidate
  settles").
- B reports `partial`. B's freeze is refused at
  `OpenFreezeCandidate` (internal/agentcore/freeze_candidate.go:57), so
  it opens no operation.

Request A (sent verbatim):

```
Engineering request A (FP4 admission drill FP4AQ-20261022, test computer only): through the engineering desk, make one small, real, reversible source change. Keep this assignment live for at least five minutes: a second engineering request will be cast while it works and must queue behind it.
1. In internal/selfdev/operations.go add the comment line "// Operation is one self-development change, from freeze to its decision." directly above "type Operation struct".
2. Start the job `sh -c 'sleep 300'` and wait for it to end (the drill needs A live for five minutes).
3. Check the change with `go vet ./internal/selfdev`.
4. Build the release as usual (bin/autoputer and the frontend), then Freeze the candidate for my review.
5. Await my decision; do not apply it.
Management: cast this once; a second request B will arrive and should wait in the engineering queue, not replace this one.
```

Request B (sent verbatim):

```
Engineering request B (FP4 admission drill FP4AQ-20261022, test computer only): through the engineering desk, in one assignment. This request is expected to wait in the engineering queue behind request A, which is already working; that is the drill.
1. In internal/selfdev/operations.go add the comment line "// Store keeps self-development operations." directly above "type Store struct".
2. Record three cheap receipts, each in its own cell: the toolchain (`go version`, `node --version`, `npm --version`); the check `gofmt -l internal/selfdev/operations.go` (no output expected); and, as the build stand-in, `sh -c 'test -s internal/selfdev/operations.go'`.
3. Do not run go vet, go build, npm or vite.
4. Call Freeze exactly once, citing those three receipts. It is expected to be refused while A's candidate awaits my decision.
5. When Freeze is refused, do not retry and do not work around it: Complete the assignment as partial, quoting the refusal text.
Management: cast this once and do not re-cast, cancel or replace request A; report B's outcome back to me.
```

## Step 3 (planned refresh while A is live): unreachable by design, on any computer

- The guest's planned-restart endpoint (`HandleInternalPlannedRestart`)
  refuses whenever `plannedRestartWork.unsafe()` holds:
  `SelfdevActive > 0 || LiveCapsules > 0`
  (internal/agentcore/planned_restart.go:348). Its comment says such work
  "always refuse[s]", whatever the host asks (`allow_busy` included).
- A bound, non-terminal assignment whose capsule is active, freezing,
  frozen-undecided or being revoked counts as a live capsule
  (`plannedRestartLiveCapsule`, :388).
- So while A is live, no host refresh can get a marker-backed restart,
  `critical_protected` or not.
- The other marker writer is the self-development apply
  (`markPlannedRestart`, :298). It needs an approved candidate, and this
  drill approves nothing.
- vmctl's deploy refresh of a non-protected computer also defers on a
  live capsule.
- So "B kept and resumed on a planned boot" can only be reached when B is
  queued and no capsule is live:
  - A is terminal and B not yet bound. The nudge closes that window in
    seconds.
  - Or A is open but not yet bound, which lasts about 20 s in practice.
- Neither is forced here, and step 3 is not run.

## Steps


Drill steps are numbered as in FP0's drill (0-6). The script step
(`fp4-selfdev-admission.sh --step N`) is named where one was run. Times
are UTC. The build column is the guest `/health` `build.deployed_commit`
at that time; J is a84b39bac (deployed 09:49:51) and K is 5289387c9
(deployed 10:16:04).

| Step | Time | Call | Code | Rows read | Log line | Build | Result |
|---|---|---|---|---|---|---|---|
| 0 wake | 09:49:43, health 09:52:16 | `POST lifecycle/start` (idempotency `fp4-proof-wake-20261022T0950`), then GET /health, obligations, operations | 201, 200 | busy 0; obligations selfdev {0,0,0}; `selfdev_active_operations` absent; operations: 5e28993f `failed`, bb0cebfa `rejected` | vmctl: snapshot resume refused (launch_config_mismatch), cold boot | J | PASS |
| 1 cast A | 09:53:30 cast, A opened about 09:54:20, bound 09:54:42.188 | `choir run start` (idempotency `fp4aq-cast-A-20261022T0955`), prompt A | exit 0 | A `assignment-a6fa7243…` open/unbound at 09:54:23, then bound/active at 09:54:45 (5 s row poll) | n/a | J | PASS |
| 2 cast B | 09:54:53 cast; B opened 09:55:35; script 09:55:48-51 | `choir run start` (`fp4aq-cast-B-20261022T0955`), prompt B; `--step 2` | exit 0; script "no failed checks" | A bound, capsule active, not terminal, 1 bound event; B open, capsule unbound, no bound event | 09:55:38 "engineering admission: assignment assignment-932f7ac2… waits behind assignment-a6fa7243… (live); retry 1 in 10s"; retries continued to "retry 18 in 1m0s" at 10:10:48 | J | PASS |
| 5 writer gate (on J) | 09:56:04 | GET /api/runtime/observability | 200 | `objectgraph_writer_gate` {"mode":"observe","counts":{"choir.texture_document\|undeclared\|undeclared_writer":2,"choir.texture_revision\|undeclared\|undeclared_writer":2}} | n/a | J | PASS (observe, counts present) |
| A ends | A froze 10:11:16; terminal_at 10:11:20.297; candidate awaiting_approval 10:14:00.9 | (no call; A's own run) | n/a | A completed, capsule revoked; selfdev-0593136f executing (10:11:05), then frozen (10:11:16), then `awaiting_approval` (10:14:02) | 10:11:20 "assignment … terminal report attempt failed: … err=lifecycle command digest conflict" (a retry recorded the report) | J | as planned |
| 3 planned refresh (natural, B queued, A not live) | deferred 10:05:58 to 10:14:57; restart 10:15:57; boot 10:16:07 | none by me: vmctl deploy K retry-sweep | vmctl `outcome=refreshed planned_restart=true busy_allowed=false` | B row at 10:16:10: open, capsule unbound; A completed | guest 10:15:57 "host requested a planned restart (platform update 5289387c9…; allow_busy=false, interrupting 0 running run(s))"; 10:16:06 "restart kind=planned … marker_age_at_process_start=8s consumed_after=1s"; 10:16:07 "planned boot: queued engineering request assignment-932f7ac2… kept and resumed" | J then K | PASS (rows and log) |
| 4 nudge | B bound 10:16:25.618; script 10:19:33 | `--step 5` | script FAIL | A terminal_at 10:11:20.297, B bound 10:16:25.618: **305.3 s** against the 30 s bound | no admission line from 10:10:48 to the restart; 10:15:39 "dispatcher: activation wait agent=management:… waited=4m19s updates=30 slots=2/8" | J (wait) / K (bind) | **FAIL**, recorded in f683cc241 |
| 6 B freezes against A's frozen candidate | 10:16:48.8 | B's `choir.Freeze` (B's own run) | tool_error | operations after the drill: still 3 (no operation for B); selfdev-0593136f `awaiting_approval` | tool error "…cannot freeze until the open candidate settles (another self-development operation is open on this computer: selfdev-0593136f3ac2d894e21abd97132276d1 is awaiting_approval)…"; guest 10:16:49 "selfdev: assignment assignment-932f7ac2… cannot freeze: …" | K | PASS |
| B's end | reports 10:17:02.5 and 10:17:09.8; cancel 10:17:19.08 | B's `choir.Complete("partial", …)` twice | n/a | B `cancelled`, reason `tool loop: completion guard "engineering_overlay_terminal_fate_required" was not satisfied after 2 retries`; capsule revoked | event `engineering-system-cancel:assignment-932f7ac2…:1:1` | K | finding, recorded in f683cc241 |
| 5 writer gate (on K) | 10:27:22 | `--step 1` | script "no failed checks" | {"mode":"observe","counts":{}}: counters reset by the 10:16 cold boot | n/a | K | PASS |
| crash half | not run | none | n/a | n/a | n/a | n/a | skipped: no throwaway computer (FP0 rule) |

Notes:

- Script step 4 (B still open after the boot) was judged from the rows
  read at 10:16:10. It was not run as a script step, because B had bound
  by the time the step could run. The script's mutating step 3 was not
  run either; the refresh was deploy K's own.
- Step 4 failed, so the drill was not judged a pass.
  - Problem record:
    docs/problems/fp4-nudge-waits-behind-management-activation-2026-10-22.md.
    The nudge and the retry are wakes on management's mailbox, which was
    in a 4-minute activation that began at A's terminal_at. The cause is a
    hypothesis, supported by timing.
  - B bound only through deploy K's planned boot.
- B's end follows R3's own "partial" advice. The completion guard
  system-cancelled it.
  - Problem record:
    docs/problems/fp4-r3-partial-advice-ends-in-system-cancel-2026-10-22.md.
- Left open on purpose:
  - candidate selfdev-0593136f3ac2d894e21abd97132276d1 (A's) is
    `awaiting_approval`. It was not approved, and it holds the computer's
    one operation slot until the owner decides.
    - Settled since (added at round 3b): FP0 rejected it on 2026-10-22 at
      11:32 UTC (idempotency key fp0-reject-0593136f-20261022, HTTP 200,
      event 12880a34…, receipt 01a16361…; the computer's head went from
      6726 to 6728). It no longer holds the slot.
  - the computer is up on K: `running_runs` 0 at 10:23, and
    `selfdev_active_operations` absent.
- Evidence files are in the monitor's scratchpad (`mon/fqproof/`):
  - acts.log, poll.log, build.log;
  - step2-out.txt, step5-out.txt, step1-K-102722.txt;
  - s0-*, s5-observability-095604.txt;
  - s4-events-A/B-101817.txt, s4-snap-A/B-101817.txt;
  - trace-A.json, trace-B.json, post-ops-*.txt, post-health-*.txt.

## Re-run plan for steps 4 and 6 after FP4 round 3 (written 2026-10-22, before any cast)

The run above is kept as recorded. Its request B told the desk to
"Complete the assignment as partial", so it cannot exercise round 3's P2
repair, whose primary branch is a failed report. This plan replaces only
request B; request A, the computer, the rules and steps 0 to 2 and 5 are
as above, on the build that carries round 3.

Precondition: no self-development operation is open on 71afbf14 before A
is cast. FP0 checks it with `choir self-dev show/head` just before the
cast. The first run's candidate selfdev-0593136f was rejected at 11:32
UTC, so the slot should be free; if any operation is open, settle it or
use a fresh test computer before casting. Otherwise A's own freeze is
refused, A never produces the candidate B's refusal is meant to name, and
the step 6 expectations below cannot be met.

Request B for the re-run (send verbatim):

```
Engineering request B (FP4 admission drill re-run, test computer only): through the engineering desk, in one assignment. This request is expected to wait in the engineering queue behind request A, which is already working; that is the drill.
1. In internal/selfdev/operations.go add the comment line "// Store keeps self-development operations." directly above "type Store struct".
2. Record three cheap receipts, each in its own cell: the toolchain (`go version`, `node --version`, `npm --version`); the check `gofmt -l internal/selfdev/operations.go` (no output expected); and, as the build stand-in, `sh -c 'test -s internal/selfdev/operations.go'`.
3. Do not run go vet, go build, npm or vite.
4. Call Freeze exactly once, citing those three receipts. It is expected to be refused while A's candidate awaits my decision.
5. When Freeze is refused, do not retry and do not work around it: report the work as FAILED, with the refusal text as the reason.
Management: cast this once and do not re-cast, cancel or replace request A; report B's outcome back to me.
```

What the re-run must show:

- Step 4: B binds within seconds of A's terminal_at while management is
  still reading A's report (`--step 5`, 30 s bound). The guest log has
  "actorruntime: engineering admission wake latency
  mailbox=engineering-admission … waited=…" for B's nudge (content
  carries `"nudge":"<A's assignment id>"`), with waited of a few seconds
  at most. Let A finish on its own; do not use the "or Cancel it"
  variant, which this repair does not cover (see the P1 record's
  residuals).
- Step 6, the refusal: B's freeze tool error reads "cannot freeze until
  the open candidate settles (… <A's operation id> is
  awaiting_approval)" and advises "report the work as failed: Complete
  with result failed and this refusal as the reason". It does not say
  "partial". The guest log has "selfdev: assignment <B> cannot freeze".
- Step 6, B's fate: B's terminal report is `failed`, and its reason
  carries the refusal (A's operation id). There is no
  `engineering-system-cancel` event for B. If the guard still cancels B,
  the cancel reason names A's candidate ("the freeze was refused because
  self-development candidate <A's operation id> was open (state
  awaiting_approval)"), never the bare guard name; that outcome is
  recorded as a finding against the advice, not a pass.
- A's candidate is still `awaiting_approval` and untouched; no operation
  exists for B.

## Re-run of steps 4 and 6 on deploy Q (2026-10-22 13:36-14:05 UTC)

Run by the self-development monitor on FP0's go. Round 3 (cf7ed5072) is
live in deploy Q, d206fdd40 (CI 39785702260). The computer was 71afbf14,
reached through the product path with its own key, `env -u
CHOIR_API_KEY`, and flags before positionals. Request A was sent as the
original text; request B was the plan's re-run text, verbatim. The
operator applied, approved and rejected nothing; at the end no candidate
is open, so nothing needed settling.

Every step ran on d206fdd40: `/health` reported `deployed_at`
13:36:34, and there was no refresh during the run.

| Step | Time (UTC) | Call | Code | Rows read | Log line | Result |
|---|---|---|---|---|---|---|
| 0 wake + precondition | 13:36:38; reads 13:36:45 | `choir computer start --computer … --idempotency-key fp4r3-wake-20261022T1342`; then `self-dev head`, `self-dev show --operation selfdev-0593136f…`, operations, obligations | exit 0, receipt `01a163d3-f9de-7adb-a069-e43dbebcd877` (hibernated to active, epoch 13243) | /health ready, running_runs 0. Head seq 7231, `pending_transition_ref` "". selfdev-0593136f `rejected` (11:32:02.873); 5e28993f `failed`; bb0cebfa `rejected`. Obligations selfdev {0,0,0}. | n/a | PASS: no operation open |
| 1 cast A | cast 13:37:34; opened 13:38:06.02; bound 13:38:25.59 | `choir run start --idempotency-key fp4r3-cast-A-20261022T1340` (trajectory `a85bd572-42c1-5512-89ee-6e000c49c88d`, submission `ec546abf-3971-50b2-a4f6-812b1abeef62`) | exit 0 | A `assignment-6daa96ea503df3de8adbba70` bound, capsule active | 13:38:07 "engineering admission wake latency mailbox=management:… update=36736c3d-… waited=1.611s content={assignment A, attempt 1}" | PASS |
| 2 cast B | cast 13:38:36.9; opened 13:39:23.89; script 13:39:42-44 | `choir run start --idempotency-key fp4r3-cast-B-20261022T1340` (trajectory `f1539984-38b5-58e1-a49c-fb76ddabd1b1`); `--step 2` | exit 0; script "no failed checks" | A bound with a live capsule, one bound event. B `assignment-4f6ef21be59f0ed797963a43` open, unbound, no bound event. | 13:39:26 "… waits behind assignment-6daa96ea… (live); retry 1 in 10s". Retries ran through 24 at 14:00:36. From 13:39:36 each retry's latency line reads `mailbox=engineering-admission … waited=1ms`-`2ms`. | PASS |
| A ends (not as planned) | Freeze 14:00:59; Complete refused 14:01:08; cancel 14:01:29.73 | A's own run | n/a | A's Freeze succeeded: `selfdev-f8778d8a37e0d85be0dc7ff42f45efb4` `frozen` 14:01:03, capsule frozen 14:01:07.80. A's `Complete("completed", …)` cited one `capsule-exec:` ref twice and was refused with "assignment command evidence unavailable after durable freeze: %!w(<nil>)". Every later cell got "assignment fate is not active and bound". `engineering-system-cancel:assignment-6daa96ea…:1:1`, guard reason. Operation `failed` 14:01:29.91. | 14:01:10-29: management fate-deadline update 58a5b36c… dead-lettered three times | **finding**: problem record 702bd1259. A's candidate was lost, so step 6 as planned is not reachable. |
| 4 nudge | A terminal_at 14:01:29.732; B bound 14:01:46.890; script 14:02:25 | `--step 5` | script "no failed checks" | **B bound 17.16 s after A's terminal_at** (bound 30 s). The first drill took 305 s. | **14:01:29 "actorruntime: engineering admission wake latency mailbox=engineering-admission update=0f8ec10a-4fca-5e84-a8ea-2b9e5276291d waited=2ms content={"assignment_id":"assignment-4f6ef21be59f0ed797963a43","attempt":1,"objective":"","nudge":"assignment-6daa96ea503df3de8adbba70"}"**. B's capsule broker was ready at 14:01:46. Management was then handling A's fate wakes (14:01:07-29), and the nudge did not wait for them. | **PASS**. A ended by the guard's system cancel, not by finishing; the plan expected it to finish. |
| 6 refusal | B Freeze 14:03:46.6 | B's `choir.Freeze` | tool_error | No open candidate existed when B froze. The refusal was "reduce: persist tray-1: capsule release contains no frozen runtime artifacts", not the open-candidate refusal. B's operation `selfdev-70598873adff86126d385785032e0178` opened (executing 14:03:51) and closed `failed` at 14:04:00.58: "assignment failed before its candidate froze; last refusal: freeze: capsule release contains no frozen runtime artifacts". | no "selfdev: assignment B cannot freeze" (none expected without an open candidate) | **not reached**. The open-candidate refusal and its "failed" advice were not exercised. |
| 6 B's fate | Complete 14:03:57.97; reported 14:04:00.14 | B's `choir.Complete` with result failed and the refusal as the reason | the tool returned "reduce: persist tray-1: lifecycle command digest conflict", but the report landed | B `failed`, `disposition_reason` "reducer-derived from report report:sha256:95333a0e4755e0dd2d74da57c6637588e3369436526f4edc212dd22c4a1f4b9d". Events: opened, bound, capsule dispositions, `co_super_assignment_reported`. **No `engineering-system-cancel` event for B.** | 14:04:00 "rule precommit … settled no at terminal failed" | **PASS** for "a failed report is terminal, no system cancel", with a different refusal than planned |
| 5 writer gate + capsule memory | 14:05:24 | GET /api/runtime/observability | 200 | `objectgraph_writer_gate` {"mode":"observe","counts":{"choir.texture_document\|undeclared\|undeclared_writer":2,"choir.texture_revision\|undeclared\|undeclared_writer":2}}. `capsule_memory`: see the next table. | 14:01:27 and 14:03:59 "capsule … memory: policy=observe …" | recorded |

`capsule_memory` receipts (policy `observe`):

| Capsule | Assignment | request = admitted = max | peak | events high / max / oom / oom_kill | doctrine_would_throttle |
|---|---|---|---|---|---|
| capsule-3b63529a-9bc6-55e8-a589-e8decdcfe8cb | A (release build) | 2147483648 | **2147594240** (at max) | 0 / **385311** / **73** / **2** | **true** |
| capsule-9ae08362-e606-51c3-8caf-0be3ef789c33 | B (cheap receipts) | 2147483648 | 158109696 | 0 / 0 / 0 / 0 | false |

Findings:

- **New problem: A's frozen candidate is lost because its Complete
  repeats an execution ref.** The record is
  `docs/problems/frozen-candidate-lost-when-complete-cites-a-duplicate-execution-ref-2026-10-22.md`
  (702bd1259).
  - `bindGrantedReceipts` collapses duplicate refs.
  - The post-freeze count check then fails, and wraps a nil error.
  - A frozen capsule refuses every later cell, so the model cannot
    report again.
- **The release build runs at the capsule's 2 GiB ceiling.** It hit
  memory.max 385,311 times and was OOM-killed twice.
  - Three `capsule_go_eval` calls hit the 30 s broker read timeout, at
    13:44:16, 13:44:57 and 13:58:54. Hypothesis: these are related to the
    memory pressure.
- **B's `Complete("failed")` returned "lifecycle command digest
  conflict" to the model although the report landed.**
  - A's terminal report in the first drill (10:11:20) showed the same.
  - After it, B spent two guard retries on cells that could not run. Its
    assignment was already terminal, so no cancel followed.
- **B's report says the literal comment text was missing from the cast it
  received.** B wrote its own one-line comment and disclosed that. Its
  admission content carries only an objective ("obey all specifics in the
  parent request").
- **Step 6, the open-candidate branch, is still unproved on staging.** A
  later run needs A's Complete to land; a ref list without repeats would
  allow that until the duplicate-ref problem is fixed.

Evidence is in the monitor's scratchpad, `mon/fr3proof/`:

- acts.log, poll.log;
- s0-start.json, s0-head.json, s0-show-0593136f.json, s0-ops.txt,
  s0-obl.txt, s0-health-*.txt, s0-observability-*.txt;
- s1-run-start-A.json, s2-run-start-B.json;
- step2-out.txt, step5-out.txt;
- s4-events-A/B-*.txt, s4-opA-*.json;
- s6-events-B-*.txt, s6-snap-B-*.txt, s6-ops-*.json;
- s5-observability-140524.txt;
- trace-A-1402.json, trace-B-1405.json.
