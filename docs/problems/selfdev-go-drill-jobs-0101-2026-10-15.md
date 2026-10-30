# Self-Development Go Drill selfdev-jobs-0101: Applied, Restore Refused, Jobs Misread (2026-10-15)

Status 2026-10-30: the restore-refusal analysis names the VM-local Dolt projection witness; that witness is now the guest SQLite store rebuilt from the tape. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Date: 2026-10-15 (drill run 01:01-01:37 UTC). Drill:
`node scripts/reeng_tracer.mjs --label selfdev-jobs-0101 --scenario engineering --change go --expect-sha e83735f2a… --timeout-min 45`
on staging build `e83735f2a` (exit 6: the scenario's claim failed). The build
carries capsule jobs (`choir.Start`/`Poll`/`Stop`, job receipts, job wake;
[design](../design/capsule-long-exec-2026-10-15.md)) and the Freeze
receipt-slot fix
([selfdev-freeze-receipts-refused](../archive/problems/selfdev-freeze-receipts-refused-2026-10-15.md)).

Evidence: owner trace bundle
`~/.config/choir-qa/traces/selfdev-jobs-0101/` (`receipt.json`,
`bundle-a1.json`, 1248 run events). Test computer
`computer-5f77fe6f827af6baa66fb92dcf205bbe`: minted and stopped by the
tracer; started once at 01:44 UTC to read the four engineering inboxes and
the operation through product routes (`GET /api/inboxes/{id}/records`,
`GET …/self-development/operations/{op}`), stopped again at 01:50. No SSH,
no `/internal/*`.

Status: documented. This commit is evidence only; fixes follow in separate
commits on branch `fix/selfdev-go-drill-0101` (not pushed, not deployed).
Item 1 (checkpoint head race) is repaired separately on
`fix/checkpoint-head-race` (not pushed, not deployed).

## Outcome

First `applied` since C1e B1. Operation
`selfdev-0b7e33a2b7a99a8c5ee25518ec7f945f`: executing (01:33:38) → frozen
(01:33:58, digest `ef28a69e…`) → awaiting_approval (01:34:33, digest
`4b296d4e…`, verifier ref `ecba2551…`) → accepted (01:35:21) → planned
restart `self_development_apply` → materializing → applied (01:36:07) →
route receipt (generation 1). Acceptance failed six checks:
`restore_to_pre_apply_head`, `go_build_job_done`, `go_test_job_done`,
`management_cast_engineering_on_records`, `final_report_names_bundle`,
`verified_against_named_bundle`. Two gaps were logged.

| item | verdict | class |
|---|---|---|
| 1 restore 500 | real bug: a busy-computer checkpoint binds one head and witnesses another | substrate (checkpoint) |
| 2 jobs count=1 done=0 | tracer read the wrong inbox; behind it, a capsule RPC desync and busy-polling | tracer: symptom; RPC desync: substrate |
| 3 cast on records | tracer only knew Texture's first Cast; three assignments failed for three different reasons | tracer: symptom; failures: symptom (desk judgment + capsule environment) |
| 4 bundle digest | Verify legitimately re-digests the bundle; tracer check stale | symptom (tracer) |
| 5 gaps | marker target not served (small read gap); projection head after restore (design gap) | symptom (surface) |

## The story as the ledger records it

Texture Cast the request to management four times (rounds 1-4, records in
`management:06a98cef…`). Each management round Cast one engineering
assignment; three failed, the fourth froze.

| assignment | run | round | ended | cause (detail below) |
|---|---|---|---|---|
| A1 `b621ea7d…` | 01:04:19-01:10:03 | 1 | failed, 110 iterations | 20-min Go build job still running after 4.5 min; test job refused (one job per capsule); desk busy-polled, then completed `blocked` |
| A2 `fb5e1706…` | 01:13:51-01:15:11 | 2 | failed | test job passed; frontend copy into `/var/lib/artifact/release/frontend` failed (`cp -a` cannot preserve permissions, then `cp -R` "Permission denied" over the read-only partial copy); desk completed `blocked` |
| A3 `e5d37a29…` | 01:18:29-01:18:49 | 3 | failed, 6 iterations | Cast objective "Resolve the release destination permission denial"; fresh capsule has no `/var/lib/artifact`; desk read `ls` output as "root-owned, not writable" and completed `blocked` without trying `mkdir` |
| A4 `3293a4d0…` | 01:22:10-01:34:27 | 4 | completed | test, build job, Freeze; desync after a cell timeout (item 2) |

The assignment the request called "the first, 01:18:31" is A3, the third.

## 1. `restore_to_pre_apply_head`: restore 500 "reconstructed content root does not match checkpoint"

### Where the message comes from

`selfdevprotocol.WitnessContentMatches` (`internal/selfdevprotocol/rematerialize.go`),
called by `Runtime.RematerializeFromTape` (`internal/agentcore/rematerialize.go`)
after it replays the tape into a staged store through
`checkpoint.Request.AcceptedEventHead`. It compares the replayed Dolt
projection's `content_root` with the checkpoint's
`VMLocalContentWitness.ContentRoot`.

The witness is the VM-local Dolt projection only (database, content root,
schema and table hashes). The workspace, the capsule jobs' files and logs
(`/tmp/choir-jobs`), the release tree, the frontend and the Go change do not
enter it. So none of those can cause this mismatch.

### Root cause (code, confirmed against the tape head movement)

The pre-apply checkpoint was `POST /api/computers/{id}/lifecycle/checkpoint`
(deleted 2026-10-26: checkpoints are bound automatically; read them with
`GET .../lifecycle/checkpoints`) → `BindCheckpointRestoreSet`:

1. `checkpointRestoreBindings` → `ReplayCompleteness` captures head H1 and
   the live Dolt state together under `HoldAppends`, replays the tape to H1
   (desks keep appending meanwhile; eed21ece4, rerun 12 fix B), and checks
   live(H1) == replay(H1). The witness is the state at H1.
2. `publishOwnerRecoveryCheckpoint` then reads the head **again**
   (`rt.store.Head`), H2, and publishes `AcceptedEventHead: H2` with the H1
   witness. The platform authority (`internal/platform/checkpoints.go`)
   requires `AcceptedEventHead` to equal its current canonical head, so the
   guest must name H2; nothing makes the witness H2's.
3. Restore replays to H2 and compares with the H1 witness. Any event between
   H1 and H2 changes the content root, so the restore refuses.

eed21ece4 made a busy computer able to checkpoint and fixed the report's
head, but did not carry that head to the publish. The same skew exists at
the two other publish sites that call `checkpointRestoreBindings` and then
bind a separately read head: the self-development materializer's post-apply
checkpoint (`self_development_materializer.go`, which even re-reads the head
on a CAS retry and keeps the old witness) and the platform-update checkpoint
(`platform_update.go`).

Ledger evidence that the head moved during this checkpoint:

- tracer head reads: seq 3244 (`6e1e5ba3…`) at 01:34:37.043; the checkpoint
  POST started right after and answered 200 at 01:35:21.373 with accepted
  head `051e2ea6…`, which is seq 3312 (read at 01:35:21.616). 68 events in
  the 44 s call.
- obligations at 01:34:37: one run running. Texture's run 01:34:39-01:35:01
  committed `texture_turn_committed` + `work_settled` at 01:35:00.94, inside
  the call, after the capture.

Hypothesis left open: the exact H1. The witness carries a Dolt head (audit
receipt), not an event head; no product route serves the published
checkpoint's witness against a sequence. Only an idle computer (no append
between capture and publish) gets a restorable owner-recovery checkpoint.

Classification: substrate (checkpoint/restore binding, red). Not a drill
artifact: any checkpoint taken while a desk works is unrestorable, and the
post-apply checkpoint that rollback names has the same skew.

### Fix direction (not decided here; red)

- (a) Guard: refuse to publish when the head read at publish differs from
  the report's captured head (`report.LiveHead`); the caller retries when
  idle. Never publishes a false witness; a busy computer gets 409 instead of
  an unrestorable checkpoint.
- (b) Catch-up under the hold: after the slow replay to H1, take
  `HoldAppends`, read H2, re-extract live at H2, advance the same replay
  store H1→H2 (the tail only), compare, publish, release. Restorable and
  busy-tolerant; appends wait for one tail replay, two extracts and one
  publish round trip.
- (c) Authority: accept a checkpoint at an ancestor head on the canonical
  chain (head receipt verified). Changes corpusd's CAS semantics.

(a) is the conservative first step; (b) is the substrate fix. Stopped here
per the brief: the fix is a design choice on a red surface.

### Repair design (decided 2026-10-14; red; branch `fix/checkpoint-head-race`)

**Chosen: (b), bind the head and the witness in one held section.** The
binding invariant: a published checkpoint's `AcceptedEventHead` and its
`VMLocalContentWitness` describe the same projection state. The only way to
hold it on a busy computer is to read the head, compare live with replay at
that head, and publish that head while no event can be appended.

Code evidence for the choice:

- **(c) is not available without a platform redesign.**
  `CheckpointAuthority.Publish` (`internal/platform/checkpoints.go`) locks
  the one `computer_event_heads` row `FOR UPDATE` and requires
  `CanonicalEventHead`, `EffectiveEventHead`, `EffectiveStateCommitment`
  and `ReducerVersion` to equal the request. The row is the current head
  only; an ancestor's effective head and state commitment are not kept
  anywhere the authority reads (`EventResearchUpdate` and the
  materialization events move the effective head, `reducer.go`). Accepting
  an ancestor needs a per-sequence head history in corpusd, a new CAS
  rule, and a platform deploy coupled to the guest change. The route
  projection then joins `AcceptedOrRollbackEventDigest` from the checkpoint
  (`self_development_materializer.go`), so the ancestor head would also
  leak into the promotion evidence. Rejected: larger, crosses the host
  trust boundary, and weakens "a checkpoint names the tip at publish".
- **(a) alone never converges on a busy computer.** The slow part is the
  replay to H1 (seconds to minutes: base install plus tail). The drill saw
  68 events in 44 s; a "retry when idle" on a desk that is working fails
  again on every attempt. It only turns a silent bad checkpoint into a
  loud 409.
- **(b) is cheap and guest-local.** `HoldAppends`
  (`internal/computerevent/appender.go`) takes the appender mutex that every
  append holds for its whole platform round trip (`appendLocked`: platform
  `Head`, `PinEvent`, `CompareAndSwap`, projection finalize), so while it is
  held the guest's head cannot move and one more platform round trip under
  it is the existing lock discipline. The tail catch-up does not need that
  lock: `replayInto` replays through a separate dry-run appender with its
  own mutex, and `reconstruct` resumes from the projection's own head, so
  the same disposable replay store advances H1 → H2 by the tail only
  (no deadlock, no full replay). `ReplayCompleteness` already extracts
  the live store under the hold, so the held section adds one tail replay
  and one replay extract, and only when the head moved.

Shape (one function, `bindCheckpointAtHead`, used by all three publish sites):

1. Probe without the hold, as today: capture H1 + live state together,
   replay the tape to H1 in a disposable store, check eligibility. The
   replay store stays open.
2. Pre-catch-up without the hold: if the live head moved, advance the
   replay store to it, so the held tail is only what lands during this
   step.
3. Under `HoldAppends`: read the head H. If H = H1, the probe's witness is
   H's (idle path, exactly as before). Otherwise extract live at H, advance
   the replay store to H, extract replay, compare and check eligibility at
   H, and derive the witness at H. Pin the replay base under H's checkpoint
   reference. Call the site's publish closure with H (each site builds its
   own request from H: receipt, reconstruction digest, idempotency key).
4. Release. A refusal that says the platform head is not the request's
   (a writer outside this guest's appender, or a pending transition) is
   retried from step 2, at most 3 attempts, then fails loudly with the
   attempt count. Any other refusal returns as is.

The three call sites (all switch to it):

- owner recovery: `BindCheckpointRestoreSet` → `publishOwnerRecoveryCheckpoint`
  (`rematerialize.go`): witness at H1, head re-read at publish. Silent skew
  on every busy checkpoint (the drill).
- post-apply: `self_development_materializer.go`: head read before the
  witness (a busy computer gets a 400 at the platform), and the retry loop
  re-reads the head and republishes the H1 witness (silent skew on retry).
- platform update: `drivePlatformUpdateTail` (`platform_update.go`): head
  read before the witness; a busy computer gets a 400 at the platform.
  Loud, never silent, but never succeeds while a desk works.

Failure modes the fix must hold (written before the tests):

- C1 an append between the witness capture and the publish leaves the
  published head ahead of the witness; restore refuses "reconstructed
  content root does not match checkpoint" (the drill).
- C2 the rebound witness at H is not re-compared at H: a live-only row
  written after H1 slips past eligibility and is attested.
- C3 the held section deadlocks: the tail replay or a publish closure takes
  the appender lock.
- C4 the held tail is long (the whole probe's worth of events replayed
  under the hold), stalling every desk.
- C5 a head that keeps moving outside this guest's appends loops forever
  instead of failing loudly and bounded.
- C6 an idle checkpoint changes: different head, idempotency key, witness
  or request bytes.
- C7 the base pin names a head other than the published checkpoint's.
- C8 the bind-only path (no platform credential) reports a witness for a
  head it does not name.

Protected surfaces: checkpoint publish and restore binding (red),
`HoldAppends` hold time (every desk append waits on it). Not touched:
corpusd checkpoint authority, restore (`RematerializeFromTape`), route
projection, base selection.

Admissible evidence: a focused agentcore test that drives the product
checkpoint route (`BindCheckpointRestoreSet` through a platform fake that
enforces the authority's head equality) with an append between capture and
replay, then restores from the published checkpoint with
`RematerializeFromTape`: before the fix, the restore refuses with the
drill's message; after, the witness matches. Staging proof: the
`--change go` tracer rerun's `restore_to_pre_apply_head` (local tests do
not prove the staging timing).

Rollback: `git revert` of the fix commit restores the old binding (busy
checkpoints silently unrestorable again; no data migration, no format
change: the checkpoint request shape is unchanged).

### Repair (branch `fix/checkpoint-head-race`, not pushed, not deployed)

Code: `internal/agentcore/checkpoint_restore_bindings.go`
(`bindCheckpointAtHead`, `advanceCheckpointReplay`,
`rebindCheckpointWitness`; `checkpointRestoreBindings` deleted).
`replay_completeness.go`: `ReplayCompleteness` is now `openReplayProbe` +
close, so the replay store can stay open for the tail. The three publish
sites build their request inside the held closure from the binding's head:
`rematerialize.go` (owner recovery; the bind report gains
`bound_event_head` and `caught_up_events`), `self_development_materializer.go`
(post-apply), `platform_update.go` (platform-follow). `restore_base.go`:
the base pin takes the bound head.

Tests (`internal/agentcore/checkpoint_head_binding_test.go`, a platform fake
that enforces the authority's current-head equality and signs with the
platform-control key; the product route `BindCheckpointRestoreSet`, then
`RematerializeFromTape` on the published checkpoint):

| test | failure mode | before the fix | after |
|---|---|---|---|
| `TestCheckpointBindBusyComputerPublishesRestorableHead` | C1, C7 | FAIL: `restore to the busy checkpoint refused: rematerialize: reconstructed content root does not match checkpoint` (the drill's message); base pinned under `checkpoint:<H1>` while the checkpoint names H2 | PASS: published head = live head after the append, witness matched |
| `TestCheckpointBindIdleComputerBindsCapturedHead` | C6 | PASS | PASS (same head, key `owner-recovery-<H>`, witness = probe's, one publish request) |
| `TestCheckpointBindRefusesLiveOnlyStateWrittenAfterCapture` | C2 | FAIL: published over the live-only table | PASS: refused at the bound head ("… (at bound sequence N)"), nothing published |
| `TestCheckpointBindCatchesUpBeforeTheHold` | C4, C8 | (new API) | PASS: 2 events caught up, 0 replayed under the hold, bind-only binding names the live head |
| `TestCheckpointBindFailsLoudlyWhenPlatformHeadKeepsMoving` | C5 | FAIL: 1 attempt | PASS: 3 attempts, then "refused the bound head on all 3 attempts" |

C3 (deadlock) is covered by the busy test completing: the tail replay and
the publish run inside `HoldAppends`.

Runs: `go build ./...` ok; `go vet ./internal/agentcore` ok;
`go test ./internal/agentcore -run 'Checkpoint|Restore|Rematerialize'` ok
(64.9 s); `go test ./internal/agentcore -run 'SelfDev|PlatformUpdate|ReplayCompleteness|Materializ|Rollback|Hibernate'`
ok (94.6 s). `internal/platform` and checkpointd are not touched.

Found while rewriting the post-apply site: the old retry loop set
`published = true` on any publish error other than "head CAS conflict" and
fell through with a zero checkpoint (`checkpointRef` of an empty digest)
instead of returning the error. The rewrite returns every publish error.

Tracer restore leg (`scripts/reeng_tracer.mjs` on `fix/selfdev-go-drill-0101`,
`restore_to_pre_apply_head`): it requires the restore report's
`checkpoint_digest` to equal the published pre-apply checkpoint's,
`witness_matched` and `frontend_restaged`, and the checkpoint's
`effective_event_head` to equal the head read at the decision boundary.
None of these change: the accepted head may be later than the tracer's
pre-checkpoint read (it already was), the effective head moves only on
materialization, rollback or research-update events. No tracer change
needed. Rerun expectation: the checkpoint POST answers 200 with
`bound_event_head` = `published_checkpoint.checkpoint.request.accepted_event_head`
and `caught_up_events` > 0 when a desk worked during it; the restore
answers 200 with `witness_matched: true`. A research update between the
boundary read and the checkpoint would still fail the effective-head clause
(pre-existing, not this bug).

Ceremony (red):

- Conjecture delta: "a busy computer cannot get a restorable checkpoint"
  becomes "a checkpoint's head and witness are one state by construction;
  busy costs one tail replay and two extracts under the append hold".
- Protected surfaces touched: checkpoint publish (owner-recovery,
  post-apply, platform-follow), base pin reference, append hold time.
  Untouched: corpusd checkpoint authority, restore, route projection.
- Admissible evidence: the focused tests above (local, guest-side).
  Staging proof is the `--change go` tracer rerun on a build carrying this
  commit; local tests prove nothing about staging timing (extract cost on a
  large store under the hold).
- Rollback: `git revert` of the fix commit; no format or data change.
- Heresy delta: discovered: the post-apply retry loop fell through on
  non-CAS publish errors. Introduced: the publish's platform round trip now
  runs under the append hold (appends already make three platform round
  trips under it; a hung platform delays appends for at most the 15 s
  credential client timeout per attempt). Repaired: checkpoint witness/head
  skew at all three sites (R1); the post-apply fall-through.

Residual R1b: hold time on a large store. The held section adds one live
extract and one replay extract when the head moved (the probe already
extracts live under the hold). Measure on the rerun (the bind report's
`caught_up_events`; guest log timing) before tuning.

## 2. `go_build_job_done`, `go_test_job_done`: tracer saw jobs count=1, done=0

### Why the tracer saw one job, not done

The tracer reads job records from `claim.engineering_casts[0].inbox`: A1's
inbox. A1 has one job record, `job-b8d5a6090d7a`
(`go build -o /var/lib/artifact/release/bin/autoputer -ldflags=-s -w ./cmd/autoputer`),
state `stopped`, reason "the capsule ended before the job did", started
01:05:38, closed 01:10:03 when A1 completed. The completing assignment is
A4. Its inbox holds three job records:

| job | argv | state | receipt | used at Freeze |
|---|---|---|---|---|
| `job-4a11769b55a1` | `go test ./internal/drillmark/` | done, exit 0 (01:22:58-01:23:07) | `capsule-exec:…89ad5118…` | no |
| `job-6768a9a48eae` | `go test ./internal/drillmark/` | done, exit 0, `(cached)` (01:23:16.85-.93) | `capsule-exec:…a6043faa…` | refused |
| `job-32d3f15d9560` | `go build -o /var/lib/artifact/release/bin/autoputer -ldflags=-s -w ./cmd/autoputer` | done, exit 0 (01:23:38-01:33:28) | `capsule-exec:…b039c799…` | build slot |

Read from the right inbox, `go_build_job_done` passes. `go_test_job_done`
passes on state `done`, but the tracer's test match is `argv.includes('test')`,
which misses `sh -c "… go test …"`, and "done" is weaker than what Freeze
accepts (below). Tracer check: symptom, stale.

### What engineering did with jobs (A1, A4)

- **A1:** started the Go build as a job (01:05:38), then `choir.Exec("go test …")`
  failed `go.mod file not found` (`choir.Exec` runs at the capsule root; known,
  see the freeze-receipts doc). Its `choir.Start` of `go test` (01:05:43)
  printed `job-fa5072a0ab9f` and returned no error, but the start was
  refused: `job_results[0].error` = "job job-b8d5a6090d7a is still running;
  one job runs at a time in a capsule". `choir.Start` hands back the id it
  staged; the refusal arrives only in the reduce result. Then about 100
  `choir.Poll` cells, 2 s apart, over 4 min, although `choir.Help("Poll")`
  (read at 01:06:33) says "It never waits: end your turn and you are woken
  when the job ends". Compaction at 01:09:24; a second build start refused
  the same way; `Complete("blocked")` at 01:10:00 while the build ran. A
  cold `go build ./cmd/autoputer` in a capsule takes about 10 min (A4:
  01:23:38-01:33:28), so a 20-min job was the right tool; the desk did not
  wait for its wake.
- **A4:** wrote the two files; then a `choir.Exec("sh -c 'cd /workspace/platform && go test …'")`
  cell ran into its 30 s `timeout_ms` (a cold test compile) and the capsule
  RPC channel desynchronized (next section). It then started test jobs, the
  build job (01:23:38), polled about 40 times, ended its turn at 01:27:43,
  and was woken at 01:33:31 (`loop.retry`, reason `capsule_job_ended`) three
  seconds after the build ended. Wakes work: the other wake fired at
  01:23:10 for `job-4a11`.

### Freeze: which receipts, and how bin/autoputer was built

- First `choir.Freeze` (01:33:37): build `capsule-exec:…b039c799…` (job 32d3),
  test `capsule-exec:…a6043faa…` (job 6768), toolchain
  `capsule-go-eval:…32829c56…` (a cell printing go/node/npm versions).
  Refused: "the workspace changed while job job-6768a9a48eae ran, so it did
  not build or test the tree it names"
  (`capsule.JobReceiptOutcomeRefusal`: start worktree digest ≠ end digest).
- Second `choir.Freeze` (01:33:45), accepted: same build and toolchain;
  test `capsule-go-eval:…748a1931…`, a cell `Exec` of `go test` that printed
  `ok … (cached)` at 01:33:42. The test evidence Freeze accepted is a cell,
  not a job; for a sub-second cached test that is legitimate.
- Freeze builds nothing. `bin/autoputer` came from job 32d3 writing
  `/var/lib/artifact/release/bin/autoputer` inside the capsule;
  `StageGrantedRelease` copies every changed path under the release prefix
  out of the capsule overlay, and an executable `bin/autoputer` gives origin
  `capsule_built`. The frontend came from A4's `npm ci && npm run build && cp -a`
  cell (01:23:22); Freeze refuses a release without `frontend/`.
- **Observation, not one of the five items:** after the apply, the guest
  reported build commit `e83735f2…` and built_at `20261015004740`,
  unchanged. A binary built with `-ldflags=-s -w` stamps commit `local`
  (`internal/buildinfo`). The guest unit execs a release binary only through
  a `layering-entrypoint` pointer (`nix/autoputer-vm.nix`); a plain release
  runs the base binary. Hypothesis: the capsule-built `bin/autoputer` never
  ran, and "applied" for a Go change means source and frontend, not the
  binary. `internal/drillmark` is not imported by `cmd/autoputer`, so this
  drill cannot see the difference either way. Named residual R4.

### Substrate finding: one timed-out cell desynchronizes the capsule RPC channel

`capsule.BrokerClient.call` (`internal/capsule/broker_client.go`) runs every
capsule RPC (`go_eval`, `exec`, `job_start`, the job watcher's 2 s
`job_status` polls, file verbs) over one persistent Unix connection. Responses
carry no request id. On a read error the connection is kept, so the
broker's late response is read by the next RPC. Each call also builds a new
`json.Decoder`, which can drop buffered bytes or start mid-frame.

The trigger is built in: `capsule_go_eval` sets the call's context deadline
to `timeout_ms` and passes the same `timeout_ms` to the broker as the eval
timeout. A cell that runs to its timeout always loses the race: the client
gives up at 30.000 s, the broker answers "session worker eval timed out" a
few milliseconds later.

Ledger (A4 run events):

- 01:22:24 Exec cell (`go test`) → 01:22:54.34 `tool_error: failed to read broker response: read unix …: i/o timeout`.
- 01:22:55.9 `Start(go test, cwd)` cell returned in 0.24 s a response whose
  `duration` is 30 013 ms and whose error is "session worker eval timed
  out": the previous cell's answer.
- 01:22:58 `Records()` cell returned `job-4a11769b55a1` with the job-start
  intent of the 01:22:55 cell; the job started then (record: 01:22:58.60).
- 01:23:00 `Poll` → `failed to read broker response: invalid character '7' in literal false`: a frame read mid-message.
- 01:23:14 `Start(go test)` returned the `Records()` output of 01:23:12.
- 01:23:16 `Start(sh -c "cd … && go test …")` returned `job-6768a9a48eae`
  with argv `["go","test",…]`: the 01:23:14 cell's intent. Job 6768 started
  01:23:16.85.
- Cells at 01:23:02, :05, :07, :12, :18, :20, :22 that print returned empty
  stdout with exit 0, consistent with job-watcher `job_status` responses read
  as cell results.
- 01:23:22.5 the `npm ci && npm run build …` Exec cell returned in 0.29 s,
  empty; the next call (01:23:24.5, `Start(go build)`) took 14.1 s although
  its own eval's `duration` is 11 ms: it waited behind the npm build.

Consequences: the desk is shown other cells' outputs and receipt refs;
intents are reduced on a later call; the job watcher misreads and observes
a job's end late. `finishJob` takes the end worktree digest when the watcher
observes the end, not when the job ends. Hypothesis (receipt fields are not
readable through a product route): job 6768's end digest was taken while
the npm cell was writing `frontend/node_modules`, so its receipt records a
changed workspace, and Freeze refused it.

Classification: substrate (capsule RPC transport; evidence attribution). It
also explains the "missing" outputs the desk chased for a minute.
Busy-polling (A1 100 polls, A4 about 40) is desk behavior (symptom).

## 3. `management_cast_engineering_on_records`, and why the assignments failed

### The check

The tracer records only Texture's first Cast to management
(`cell:b9ad8db3…:cast:tray-1`, round 1) and requires the completing
engineering Cast's `parent_id` to equal it. A4's Cast names Texture's
round-4 Cast (`cell:49c64987…:cast:tray-2`), which is a Texture Cast to
management on records. The provenance chain is intact; the check is stale
for a multi-round story. Symptom (tracer).

### A3 (01:18:29, 20 s)

- Management's round-3 Cast objective: "Resolve the release destination
  permission denial, build the requested release tree, and freeze a
  candidate if safely possible" (management's reading of A2's blocker). Body:
  only `{"parent": …}`.
- Cells: Records + Precommit; read the cognitive-transform skill; `id; ls -ld /var/lib/artifact …`
  → uid 65534, `/var/lib/artifact` absent; `mount; ls -ld / /var /workspace …`
  → `/var` owned by `nobody`, rw overlay root.
- It concluded "root-owned path components are not writable by uid nobody"
  (false: `/var` is nobody's; A1 and A2 created the directory with
  `mkdir -p`) and called `Complete("blocked", …)`. The first `Complete`
  was refused "executor receipt unavailable": it cited
  `capsule-go-eval:sha256:8f4a8a208bfd9574beea6a0680…`, 63 hex digits, a
  mistyped ref. The second, with the ref spelled right (`…beea6ea0680…`),
  was accepted → final Report `stop_reason: failed`, no operation.
- Cause: desk judgment on a false premise inherited from management's Cast,
  plus a release-tree destination the overlay does not explain (absent in
  a fresh capsule, created by the build). Symptom.

### A2 (01:13:51, 80 s)

Test job `cb8772ca122a` done (receipt `a8d3cec7…`). Frontend job
`26bd02b30ac0` failed: `cp -a … : preserving permissions … Operation not
permitted` (the capsule refuses the metadata `cp -a` preserves). The retry
job `654f02c7fb71` (`cp -R`) failed `Permission denied` on files the first
copy left read-only. The desk Reported "destination not writable" and
completed `blocked`. A1's third attempt (`rm -rf` the destination, then
`cp -r`) had worked. Symptom: capsule environment (`cp -a`) plus desk
judgment; management relayed it as "permission denial" (A3).

### A1

See item 2. Symptom: desk did not wait for the job's wake; the one-job rule
and the start refusal arriving only in `job_results` made it think the
capsule refused all work.

## 4. `final_report_names_bundle`, `verified_against_named_bundle`: ef28a69e… → 4b296d4e…

Verify changes the digest by design. Freeze writes `bundle.draft.json` and
records its content digest (`ef28a69e…`) on the operation.
`finalizeVerifiedCapsuleBundle` (`internal/agentcore/tools_capsule.go`)
adds the verifier receipt, digests the canonical final bundle
(`4b296d4e…`), records `bundle-finalizations/ef28a69e… → 4b296d4e…`, appends
the `effect_proposed` event naming the final digest, and moves the
operation's `bundle_digest` to it. The final bundle's own `content_digest`
field stays `ef28a69e…`, and the verification event's payload binds
`bundle_digest: ef28a69e…` (it is written before finalization).

Engineering's final Report names `bundle:ef28a69e…`, the frozen bundle; the
operation the tracer observed in state `frozen` carried that digest. The
tracer compares the Report with the operation's current (final) digest.
Stale check, symptom. The right check: the Report names the digest the
operation had when frozen, and that operation went frozen →
awaiting_approval with a verifier ref.

## 5. The two gaps

- **Restart marker target not served.** The materializer writes the marker
  with `Target: operation.OperationID` (`markPlannedRestart`), but
  `GET /api/runtime/obligations` serves only `kind` and `reason`
  (`obligationsRestart`). A one-field read gap; symptom.
- **No product route serves the projection head after a restore.**
  `GET …/self-development/head` proxies corpusd's tape head, which a
  restore moves forward (a restore is a forward event). The VM-local
  projection's head is not served anywhere. The restore report's
  `witness_matched` and checkpoint binding are the landing proof today.
  Design gap; which surface should carry it is an owner choice. Deferred.

## Root cause clustering

Three substrate defects; the rest are symptoms.

1. Checkpoint publish binds a head other than the one its witness was taken
   at (item 1). Red.
2. Capsule RPC channel desynchronizes after one timeout; responses and
   intents are attributed to the wrong call (item 2). Red: evidence
   attribution.
3. Job end digest is taken at observation, not at the job's end (item 2,
   amplified by 2). Orange; follows from 2 for this drill.

The tracer checks (items 2, 3, 4) and the marker target (item 5) are stale
surfaces. The desk failures (A1-A3) are judgment and environment symptoms;
not tuned here (understand first: the A4 desync and A1 busy-polling should
be re-read on a run without the desync before any prompt change).

## Failure modes the fixes must hold (written before the tests)

Broker client (`internal/capsule/broker_client.go`):

- F1 a response that arrives after the caller's deadline is read by the
  next RPC (off by one);
- F2 a decode error leaves a partial frame that the next RPC decodes from
  mid-message;
- F3 a write error leaves the connection half-used;
- F4 after any of F1-F3, the next RPC fails too instead of reconnecting;
- F5 two goroutines (a cell and the job watcher) interleave frames on a
  reconnect;
- F6 a healthy connection is re-dialed on every call (no reuse).

Tracer (`scripts/reeng_tracer.mjs`, `acceptEngineering`):

- T1 jobs read from an assignment other than the completing one;
- T2 a test job run through `sh -c "… go test …"` is not recognized;
- T3 a Report naming the frozen digest fails because Verify re-digested;
- T4 a Report naming some other digest passes;
- T5 an engineering Cast whose parent is a later Texture round's Cast fails;
- T6 an engineering Cast whose parent is not a Texture Cast passes.

Obligations surface: M1 a planned boot's marker target is dropped; M2 a
crash boot serves a target.

## Fix plan (this branch)

1. Broker client: drop the connection after any transport or decode error
   and re-dial on the next call; keep one decoder per connection. Red
   (evidence attribution). Rollback: git revert.
2. Tracer: the four stale checks (T1-T6). Yellow.
3. Obligations: serve the marker's target. Orange (read surface). Rollback:
   git revert.

Item 1 is repaired on its own branch, `fix/checkpoint-head-race` (design
under "Repair design" in item 1). Not on this branch: the job end-digest timing
(substrate 3, revisit after fix 1), the projection-head surface, desk
behavior.

Landed on the branch (unit evidence only; not pushed, not deployed):

- `5c5db2027` fix 1. The two off-by-one tests failed before the change
  ("call after a timeout read \"late\""), pass after (linux container).
- `f9109003c` fix 2. Replaying this drill's recorded claim (plus the later
  Texture Casts from the bundle and the job records read at 01:44), all
  five stale checks pass; `restore_to_pre_apply_head` stays failed.
- `a39168d78` fix 3.

## Residuals

- R1 restore on a busy computer (item 1), and the same skew in the
  post-apply and platform-update checkpoints. Repaired on
  `fix/checkpoint-head-race` (not deployed); staging proof pending the
  tracer rerun. R1b: hold time on a large store, unmeasured.
- R2 job end digest at observation time.
- R3 `choir.Start` returns an id for a start the reduce refuses; the refusal
  is only in `job_results`.
- R4 the capsule-built `bin/autoputer` may never run after apply (post-apply
  build identity unchanged); needs a drill whose Go change is visible in the
  running binary.
- R5 `cp -a` into the release tree fails in a capsule; the overlay should say
  how to stage `frontend/`.
- R6 no projection-head read after restore.

## Heresy delta

- discovered: checkpoint witness/head skew (R1); capsule RPC desync after a
  timeout; tracer checks stale for multi-round stories and verified bundles.
- introduced: none in the evidence commit; the R1 repair runs one platform
  round trip under the append hold (see item 1, Repair).
- repaired: none in the evidence commit; R1 (and the post-apply
  fall-through found with it) on `fix/checkpoint-head-race`.
