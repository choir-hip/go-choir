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
  - the computer is up on K: `running_runs` 0 at 10:23, and
    `selfdev_active_operations` absent.
- Evidence files are in the monitor's scratchpad (`mon/fqproof/`):
  - acts.log, poll.log, build.log;
  - step2-out.txt, step5-out.txt, step1-K-102722.txt;
  - s0-*, s5-observability-095604.txt;
  - s4-events-A/B-101817.txt, s4-snap-A/B-101817.txt;
  - trace-A.json, trace-B.json, post-ops-*.txt, post-health-*.txt.

## Round 3 re-drill of steps 4 and 6 (plan, written before any cast)

FP0 asked for this re-drill once FP4 round 3 (lane/fp4-r3-nudge: tests b557b2d64, fix cf7ed5072, record 4a4bf27bb) deploys. The computer is still 71afbf14.

Starting state, read at 11:40 UTC on deploy N (4bc3aa3ec):

- A's earlier candidate selfdev-0593136f is `rejected`. FP0 rejected it with key `fp0-reject-0593136f-20261022`; its `updated_at` is 11:32:02.873. The computer's one operation slot is free.
- No operation is open; `running_runs` is 0.

Judged as round 3's `next_action` states it:

- **Step 4.** B binds within seconds of A's terminal_at, while management is still reading A's report. Script `--step 5` checks the 30 s bound.
  - The guest log has "actorruntime: engineering admission wake latency mailbox=engineering-admission … waited=…" for B's nudge; the content carries `"nudge":"<A>"`.
  - waited is a few seconds at most, and the mailbox is not management's.
- **Step 6.** A completes with its candidate frozen, then B makes one Freeze.
  - B's run shows the refusal naming A's operation, "cannot freeze until" and "Complete with result failed".
  - The guest log has "selfdev: assignment B cannot freeze".
  - B ends `failed` with the refusal as its reason. If the model reports partial, B ends `cancelled` with a reason that names A's candidate, never the bare guard name.
- Steps 0-2 are re-read on the way (wake, A bound, B "waits behind"), and step 5 reads the writer gate once.
- Step 3 is not forced. A natural planned refresh is recorded if one comes.
- The crash half stays skipped.

Prompts:

- Request A is the round 2 text with the drill tag changed to FP4R3-20261022.
- Request B is the round 2 text with that tag change, and step 5 now reads: "When Freeze is refused, do not retry and do not work around it: follow the refusal's advice: Complete the assignment with result failed, with the refusal text as the reason."
  - This follows round 3's refusal advice; round 2's prompt said partial, after round 2's advice.

Risk: since deploy M, guest egress is limited to host service ports, 80/443 and the 8089 package mirror.

- If A's release build cannot fetch what it needs, A fails before it freezes.
- Step 4 can still be judged, because A ending is all it needs. Step 6 then has no frozen candidate to refuse against, and is recorded as not reached, with A's failure.

Request A (sent verbatim):

```
Engineering request A (FP4 admission drill FP4R3-20261022, test computer only): through the engineering desk, make one small, real, reversible source change. Keep this assignment live for at least five minutes: a second engineering request will be cast while it works and must queue behind it.
1. In internal/selfdev/operations.go add the comment line "// Operation is one self-development change, from freeze to its decision." directly above "type Operation struct".
2. Start the job `sh -c 'sleep 300'` and wait for it to end (the drill needs A live for five minutes).
3. Check the change with `go vet ./internal/selfdev`.
4. Build the release as usual (bin/autoputer and the frontend), then Freeze the candidate for my review.
5. Await my decision; do not apply it.
Management: cast this once; a second request B will arrive and should wait in the engineering queue, not replace this one.
```

Request B (sent verbatim):

```
Engineering request B (FP4 admission drill FP4R3-20261022, test computer only): through the engineering desk, in one assignment. This request is expected to wait in the engineering queue behind request A, which is already working; that is the drill.
1. In internal/selfdev/operations.go add the comment line "// Store keeps self-development operations." directly above "type Store struct".
2. Record three cheap receipts, each in its own cell: the toolchain (`go version`, `node --version`, `npm --version`); the check `gofmt -l internal/selfdev/operations.go` (no output expected); and, as the build stand-in, `sh -c 'test -s internal/selfdev/operations.go'`.
3. Do not run go vet, go build, npm or vite.
4. Call Freeze exactly once, citing those three receipts. It is expected to be refused while A's candidate awaits my decision.
5. When Freeze is refused, do not retry and do not work around it: follow the refusal's advice: Complete the assignment with result failed, with the refusal text as the reason.
Management: cast this once and do not re-cast, cancel or replace request A; report B's outcome back to me.
```
