# SA: projection base watermark never refreshed — recovery tail grows unbounded

**Status:** confirmed on staging, reproducible. Mutation class: orange → red
(recovery path). File under SA slice 0.

## Found

2026-10-07. Owner VM `candidate-fleet-e15cb89f25d963c220319b7b`
(`computer-03335285269bdba4f94377e56879f9e6`) entered a boot loop:

```
autoputer: required projection base refused: refusing genesis fallback:
recovery tail 423720 events exceeds 10000;
publish a fresher base (local=0 W=148431 H=572151)
```

## Root cause

`computer_replay_watermarks` is written only by
`POST /internal/computers/files/watermark` (`HandleFileCASWatermark` →
`RecordReplayWatermark`). The sole caller is the manual `choir-rebuild-base`
CLI via `projectionbase.AdvertiseWatermark`. There is no scheduled job,
timer, or trigger anywhere in `corpusd`, `vmctl`, or `autoputer` that
re-publishes a fresh projection base as the canonical head advances.

The advertised watermark for this computer was last written when a
`choir-rebuild-base` run published seq 148431. Since then ~423k additional
events have been appended. `MaxRecoveryTailEvents = 10000`
(`internal/projectionbase/recovery_plan.go:11`), so the gap `(W, H]` = 423720
far exceeds the tail cap, and the VM can never boot past
`materializeProjectionBaseIfNeeded` without manual operator intervention.

## Fix shape

One of:

1. **Scheduled watermark refresh in corpusd** — a periodic task that runs
   `projectionbase.Rebuilder` (or equivalent) for each active computer and
   calls `RecordReplayWatermark` when the new base is published.
   Cadence: every N events (e.g. every 5000) or every T hours, whichever
   comes first.

2. **Incremental base extension** — rather than full replay, extend the
   last published base by appending only the tail events since the last
   watermark. This requires the rebuilder to accept a seed store instead of
   always starting from scratch.

3. **Watermark-aware event appender gate** — after each append batch, if
   `(canonical_head_seq - watermark_seq) > threshold`, trigger async base
   rebuild. This ties watermark freshness to write volume.

## Immediate remediation (this incident)

`choir-rebuild-base` patched to add `--source http` so it uses
`/internal/computers/events/replay` (which returns correct `TransitionInput`
via `replayTransitionInput` in `event_replay.go`) instead of
`DiskEventSource` (which fabricates `TargetStateCommitment` from
`event.ResultingEffectiveCommitment` — empty for `effect_accepted` events,
causing "accepted effect is not fully bound" at seq 474642).

Run command:
```
choir-rebuild-base \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --target-head 5fdd38ccd576442923d7ec07cc10f0fe15d6fa84956f7ee06449fff7a731bf40 \
  --source http \
  --owner 5bd6de97-3b58-408c-bf89-c42c81b083de \
  --platform-url http://127.0.0.1:8086 \
  --artifacts-root /var/lib/go-choir/platform-artifacts \
  --key-hex <guest-privacy-key>
```

`--advertise` POST requires a real computer capability (event:append scope);
the internal-caller header path only covers `event:read`. Mint one via:
1. `POST /internal/computers/credentials/issue` (X-Internal-Caller) → envelope
2. `POST /internal/computers/credentials/exchange` → Bearer capability token
3. `POST /internal/computers/files/watermark` with `Authorization: Bearer`
```

Guest privacy key extracted from `data.img` at
`/choir-credentials/privacy-key` (ext4, inode 2614).

## Secondary defect — DiskEventSource cannot replay effect_accepted events

`DiskEventSource.EventsPage` (internal/projectionbase/source.go:133)
fabricates `TransitionInput.TargetStateCommitment` as
`event.ResultingEffectiveCommitment`, which is empty on `effect_accepted`
events. The reducer rejects it ("accepted effect is not fully bound").

The correct commitment for `effect_accepted` is
`next.DesiredStateCommitment` — available only from the stored
`event_head_receipt_json` in `computer_event_append_receipts`, not from the
event bytes or the payload. Self-dev commitments additionally require
`operation.BaseHead` which lives only in the guest store.

**The `--source http` flag added to `choir-rebuild-base` sidesteps this by
reading `/internal/computers/events/replay` (corpusd has the receipts).**
`DiskEventSource` remains usable only for chains with no `effect_accepted`
events — a durable fix requires either receipts in the artifact store or a
seeded scratch store from the guest's data.img.

## Tertiary defect — applyOGMigration canonical-ID convergence (2026-10-07)

Rebuild failed at vocabulary migration after all 571,392 events replayed:

```
Error 1062: duplicate primary key given:
[obj:choir.agent:NWJkNmRlOTctM2I1OC00MDhjLWJmODktYzQyYzgxYjA4M2Rl:
 key-f3958e3fc25ccbaccb9c71381f9d324070c8b7d41bc5d572fd442901d71b7a41]
```

**Cause:** `ogRekey` derives key-suffixed canonical IDs from migrated
identity fields. Two V1 `choir.agent` objects whose `agent_id` spellings
differ in V1 ("co-super:X" and "cosuper:X") both migrate to
"engineering:X", producing the same V2 `newID`. `applyOGMigration`
`UPDATE`s both rows to that PK → conflict. The planner had no
convergence detection.

**Fix (this commit):** `planOGMigration` now groups objects by `newID`
after the fixpoint; identical rows dedupe (earliest `created_at`
survives, losers are `DELETE`d at apply, provenance in
`MigrationReport.OGDropped`). Divergent groups fail with both IDs.
A second check catches a migrated `newID` colliding with an untouched
row not retained in the plan.

Re-run: `--scratch-dir /tmp/og-scratch` preserves the store on failure
for inspection (resumed replays skip committed events).

## Fourth defect — replayed V1 row collides with its own later V2 row (2026-10-07)

The `7cf0a438` rebuild (fresh replay, 4h34m, all 572,151 events applied,
scratch store preserved at `/tmp/og-scratch` on Node B, 82G) failed at
the same step through the new divergent-external branch:

```
og migrate: canonical ID convergence conflict on
obj:choir.agent:NWJk…:key-f3958e3f…: migrated obj:choir.agent:NWJk…:key-ad7fb958…
(kind=choir.agent created=2026-08-16T20:07:13Z) collides with untouched live
row with divergent content; cannot auto-resolve
```

Rows in the scratch store (`dolt sql` on `runtime.texture/texture`):

| canonical_id | metadata.agent_id | created_at | updated_at |
|---|---|---|---|
| `…key-ad7fb958…` (V1, migrating) | `super:5bd6de97…` | 2026-08-16 20:07:13 | 2026-09-06 04:04:22 |
| `…key-f3958e3f…` (V2, live) | `management:5bd6de97…` | 2026-08-16 20:07:13 | 2026-10-07 06:47:11 |

**Cause:** one agent at two points in time. On the live computer the V1
row was renamed at the vocabulary cutover, and later events updated the
renamed V2 row. Replay is not cutover-aware: it deposits pre-cutover
events as V1 rows and post-cutover events as V2 rows, then migrates once
at the end. The stale V1 row renames onto its own successor. The planner
treats any content difference as an unresolvable conflict.

**Owner impact:** owner computer `computer-03335285…` (VM
`candidate-fleet-e15cb89f…`) still cannot boot. vmctl's warmness policy
retries every 30 min and marks the VM failed (21:37, 22:07, 22:37 UTC).
Proxy `api.resolve` calls for the owner hang 1800s before erroring.

**Fix shape:** live row wins when its `updated_at` is ≥ the migrating
row's (the order the live computer actually applied: cutover rename,
then later updates). The stale V1 row is dropped, and its original body
and metadata are kept in the migration report so nothing is silently
lost. A migrating row strictly newer than the live V2 row still fails
loudly; that would be a V1 write after the cutover. The durable fix is a
replay that applies the vocabulary migration at the cutover point in the
chain; that is recorded as a residual, not done here.

**Resume:** planning writes nothing (report persist and row writes
happen only after `planOGMigration` succeeds), and `ReconstructThrough`
returns immediately when the local head equals the target
(`appender.go:767`), so a rerun with the same `--scratch-dir` skips the
replay.

## Residuals named by this incident

- **Checkpoint cadence:** nothing republishes the projection base or
  re-advertises the watermark. Publish every N events or on a timer, and
  alert as the tail approaches `MaxRecoveryTailEvents`.
- **vmctl fail-fast:** a computer whose recovery tail exceeds the cap
  should report "rebuild required", not retry every 30 min while proxy
  resolve hangs 1800s.
- **Cutover-aware replay:** apply the vocabulary migration at the chain
  point where the cutover happened, not once at the end.
- **`mergeVocabReports` drops `OGDropped`** when a prior report exists
  (from `7cf0a438`); in-guest reruns lose the dedup audit trail. Fixed
  together with the fourth defect.

## Owner-authorized permanent policy — implementation boundary

Owner: “document and implement”, following the recovery policy panel.
Baseline: main `33f55961`; unrelated dirty `skills/agentic-consensus/*`
is preserved untouched. Mutation class **red**: replay/projection, watermark,
privacy custody, vmctl admission, proxy availability. Conjecture delta:
periodic verified incremental publication plus typed recovery admission can
bound recovery without changing event authority or historical verification.
Heresy delta: discovered missing cadence, blind boot retries, and mixed-version
projection deposits; no claim of repair until deployed acceptance.
Rollback: revert implementation commits, disable the checkpoint scheduler,
retain readable prior bases and the immutable tape; never rewind event heads.

### Research and panel adjudication

Primary sources actually read for this implementation:
[Microsoft event sourcing](https://learn.microsoft.com/en-us/azure/architecture/patterns/event-sourcing)
recommends snapshots plus subsequent events, immutable history, versioned
upcasters and idempotent consumers;
[Axon event versioning](https://docs.axoniq.io/axon-framework-reference/4.12/events/event-versioning/)
describes explicit revision transformations and snapshot compatibility.
Neither supplies Choir's numerical cadence or proves publication throughput.
The previous chat overstated external research and unanimity: the panel
disagreed on 2,500 versus 5,000 events, two versus three bases, and upcasting
versus ordered migration for identity-changing cutovers. Its claimed
one-to-two-minute rebuild and 50ms responses are unmeasured, not acceptance.
Raw panel: `.agentic-consensus/agentic-consensus-20261007-200921/`;
eight successful outputs, two failures. The verified retained-store minority
finding and historical verification boundary take precedence over votes.

### Checkpoint policy

- Reconcile every minute. Queue at cold tail `H-W >= 2500`, or base age
  at least 24h with tail at least 1000. Freeze the target head for each job.
- Host-side isolated, durable, coalesced work; one active job per computer,
  initially one worker per host. Seed from the latest compatible verified
  base and replay only its tail. The first base may bootstrap from genesis
  only while its entire chain fits the 10,000-event bound. Unbounded genesis
  reconstruction requires explicit operator repair; missing seed is never an
  automatic lifetime-replay fallback. Isolated incremental repair may exceed
  the serving boot cap without raising that cap.
- Compact the closed scratch Dolt workspace before packaging. Verify blob,
  descriptor, head, and installed content witness before advertising; read
  back the watermark. Publication is complete only when advertisement advances.
- Retain current and previous verified bases plus explicitly referenced
  restore, reader and repair bases. Never delete the tape or a live pin.
- Warn at 5000, urgent at 7500, and on two failed jobs. Expose scheduler
  heartbeat, tail, job status, errors and publication age. These are starting
  thresholds; observed queue/build/GC/upload time and bursts determine margin.
- Do not put rebuild execution in the request/append path or invent a third
  semantic store. Operational jobs belong to existing platform control state.

Source-inspection finding before implementation landing: the existing watermark
POST (`internal/platform/file_cas_http.go:180-184` at `fa251322`) accepted a
sequence/base string without checking a published descriptor or canonical
sequence binding, then echoed the request even when the monotonic store refused
to advance. This is not a staging failure claim. The permanent publication
boundary must reject missing/foreign/unbound bases and return the stored
watermark, not fabricate advertisement success from an echoed request.

### Admission and repair policy

- Reuse `PlanRecovery`, distinguishing cold `H-W` from verified retained
  `H-L`. Stale W alone must not strand a valid retained store or stop a healthy
  running computer. An unverified heartbeat is not a retained-state witness.
- Before resource allocation where inputs are verifiable, refuse deterministic
  over-cap plans. Preserve guest validation for races and unknown local state.
- Persist a typed recovery condition separately from process state. Blind
  warmness retries stop on deterministic refusal and survive vmctl restart.
  Reevaluate on changed recovery inputs, not a cooldown expiring.
- Resolve returns structured 503 with reason, repair status and Retry-After,
  promptly for known blocked state. Metadata unavailability is distinct from
  tail excess. Release waiters on typed guest refusal.
- Enqueue the same deduplicated asynchronous checkpoint/repair job; vmctl
  does not perform replay or clear another authority's state directly.
- **Owner authority update, this implementation:** owner selected “Authorize
  isolated maintenance worker”: the dedicated worker may unwrap existing
  custodian escrow solely for verified replay/checkpoint jobs, with durable
  key-use transparency and no plaintext key in job APIs. Human key reveal
  still requires two independent approvals. Event capabilities are not keys.
  No debugfs scrape of running guests. Missing escrow is a visible blocked job.

### Replay evolution policy

- Verify original immutable events, receipts, payload digests and reducer
  commitments first. Upcast only the verified projection-deposit view.
- Version vocabulary transformations, cover object IDs/references, SQL rows,
  deletes and partial updates in canonical event order; no wall-clock LWW.
- Identity/authority-changing semantic migrations require ordered, bound
  forward events, not retroactively fabricated cutover markers.
- Remove obsolete end-of-replay mutation for newly reconstructed stores only
  after genesis replay and base-plus-tail replay agree, including cross-version
  delete/recreate and unchanged raw-event verification. Existing retained V1
  stores still need an explicit one-time migration into the new projection.

### Required acceptance

Focused regressions precede code. Run a real replay/publish/install scenario;
prove no prefix reads for incremental work, compact scratch output, matching
head/content and monotonic advertisement. Exercise concurrent refused requests,
restart durability and restored admission after repair on a disposable staging
computer, plus valid retained resume with stale W. Push, monitor CI/deploy,
verify staging identity and archive API evidence. Run frozen-candidate consensus
before declaring the residuals repaired.

### Implementation status (2026-10-08, pre-landing)

Candidate implementation of all three policy sections; residuals are **not**
claimed repaired until deployed acceptance and frozen-candidate consensus.

- Checkpoint: `cmd/checkpointd` (oneshot, 1-minute systemd timer, one job per
  activation, one host worker) claims leased, frozen-target jobs from Store A
  `computer_projection_jobs`; `ReconcileProjectionJobs` applies the
  2500 / (24h ∧ 1000) cadence; seeded tail replay; mandatory scratch
  `DOLT_GC`; blob/head/witness readback before descriptor; advertisement
  readback before `succeeded`; retention = current + previous + explicit pins
  + active-job seeds; alerts at 5000/7500/two failures. Key use is audited in
  escrow transparency before unwrap.
- Advertisement: watermark POST validates published descriptor, blob size and
  canonical event binding (`projection_advertisement.go`).
- Admission: `vmctl/recovery_admission.go` persists typed conditions
  (tail excess / base missing / guest refused / metadata unavailable),
  re-evaluates only on changed inputs, enqueues the deduplicated job; proxy
  renders a structured 503 with `Retry-After` and repair status and never
  retries a typed refusal as transient.
- Replay evolution: `internal/store/vocab_upcast_deposit.go` upcasts the
  verified deposit view in event order with an alias-ledger sidecar.

Finding (test harness, not product): the four red
`TestVocabDepositUpcast*` regressions failed because the test helper
finalized the batchless genesis through live `Finalize`, while the product
replay path (`ComputerEventAppender.finalizeProjection`, `replayProjection`)
finalizes every event — including batchless genesis — through
`FinalizeReplayBatch`. The fresh-reconstruction signal therefore never reached
the upcaster in tests. Fixing the helper to mirror the product path turned all
five green with no assertion changes.

Named residuals from the pre-landing audit:

- Artifact GC grace (30 min mtime) is the only protection for a freshly
  written blob between `PublishDir` and the advertisement row; a
  verify step longer than grace could race GC. Expected minutes; measure.
- A job that fails twice is terminal (`failed`, `repeated_failure` alert)
  until its seed or escrow key changes or an operator re-enqueues; head growth
  alone does not retry it, by design.
- Failed-job scratch directories are retained for diagnostics and are not
  yet swept.

### Staging finding — scheduler reconcile scan error (2026-10-08, `0f7c58ba`)

Evidence: deployed `0f7c58ba` (corpusd/vmctl/proxy/checkpointd build.json
all report it; CI run 37831357213 green). Every timer activation of
`go-choir-checkpointd.service` exits status 1 before claiming a job:

```
checkpoint worker: sql: Scan error on column index 3, name
"COALESCE(w.updated_at,h.created_at)": unsupported Scan, storing
driver.Value type []uint8 into type *time.Time
```

Cause: `ReconcileProjectionJobs` scans `COALESCE(w.updated_at,h.created_at)`
into `time.Time`. Dolt returns the COALESCE expression as bytes, which
`parseTime=true` does not convert (it converts only typed DATETIME columns).
No unit test exercised reconcile against a Dolt server.

Impact: fail-closed. No job is queued, claimed, unwrapped or published; no
watermark moves. But the checkpoint cadence — the core of this policy — is
dead on staging until fixed. Pre-deploy staging state for the acceptance
record: 110 computers with event heads, 1 with a watermark (the repaired
owner VM, tail 534), 11 due by tail ≥ 2500, 1 of those over the 10k
bootstrap bound without a base (`computer-ccb04d4a…`, 15,998 events).

Fix shape: scan the two typed columns separately and coalesce in Go; add a
reconcile regression against the platform test store.

Repaired in `e7b51524` (regression reproduced the scan error first).

### Staging finding — oneshot service had no deploy identity probe (2026-10-08)

Evidence: CI run 37834433720 (deploy of `e7b51524`). Attempt 1 failed on a
`nix build` segfault (status 139) during concurrent Nix auto-GC — infra
flake, the same package built in the prior deploy. Attempt 2 installed and
restarted the selected services, then exited 1 at the activation-receipt step:
`No health identity probe is defined for selected host service checkpointd`.
The identity loop assumes every host service has a health port; checkpointd is
a timer-driven oneshot. Fixed in `11ee8b50` (verify the installed pointer
manifest commit, fail-closed, record the receipt). Documented after the fix:
the deploy-script defect blocked the landing loop, the fix was one
fail-closed branch, and the evidence is preserved in the CI run and
`/var/lib/go-choir/deploy-failures/37834433720-{1,2}.json` on Node B.

### Staging acceptance evidence (2026-10-08, `e7b51524` worker, `11ee8b50` deployed)

Deployed identity: choir.news `/health` `deployed_commit=11ee8b50`; Node B
pointers corpusd/proxy/checkpointd `11ee8b50`, vmctl `0f7c58ba` (unchanged
since). CI runs 37831357213, 37836514564 green.

- First scheduler pass queued 27 jobs (tail ≥ 2500, or no watermark +
  tail ≥ 1000 + chain age ≥ 24h). 26 succeeded (bootstrap from genesis,
  targets 1,023–6,198 events, ~20–30s each); watermark advanced to each
  target and read back.
- The orphaned over-cap chain `computer-ccb04d4a…` (15,998 events, no base,
  no vmctl ownership) was refused `blocked`: "projection base missing beyond
  bootstrap bound; explicit genesis repair required" — before escrow unwrap,
  no lifetime replay.
- Verified first base `computer-0396f4f2…`: blob sha256 equals name, size
  8,638,976 = descriptor, descriptor `canonical_head` equals
  `computer_event_append_receipts` digest at sequence 1,691; key-use
  transparency entry (seq 3) precedes publication.
- Disposable `computer-7e6a9afd…` (fresh passkey account via
  `scripts/s0m_disposable_keydriver.mjs`): gen-1 bootstrap at target 2
  (base `d435410e…`, 81 KB, 1.3s); after real prompt-bar work, gen-2 seeded
  from that base to target 2,318 (`SeedPinned`, `SeedRequired`; replay floor
  = 2 enforced by `boundedReplaySource`, so success proves no prefix reads):
  11.4 MB, 27s, 1.1 GB peak RSS. Watermark 0 → 2 → 2,318, monotonic.
- Node B during jobs: ~15 GiB RAM available.
- Retained resume with stale W: disposable hibernated at head 2,375 with
  W=2,318, then woken through vmctl `resolve` (the cookie-session wake path;
  API-key bearer requests intentionally never wake a computer —
  `internal/proxy/api_key_computer_authority.go`). 200 in 9.25s, boot kind
  `recover`; guest console: `projection recovery resume … (local=2397
  W=2318 H=2397 tail=0)` then `replay complete`. The planner used H−L, not
  the stale H−W; post-resume API request 200.

Not yet exercised on staging: the vmctl admission refusal path (structured
503, concurrent refused requests, restart durability, restored admission
after repair). Correction (panel, verified in `recoveryplan.PlanRecovery`):
this does **not** require >10k events — an existing chain with no watermark
and an empty store is refused `base_missing`, so a fresh realization of a
disposable whose chain has no base yet is a legitimate refusal target.

### Frozen-candidate consensus (2026-10-08, candidate `11ee8b50`)

Raw output: `.agentic-consensus/recovery-frozen-candidate-11ee8b50/`
(convergent mode; 8 substantive of 10 — opencode failed `invalid x-api-key`,
devin's tool calls were refused non-interactively).

| Residual | Verdicts |
|---|---|
| (a) checkpoint cadence | REPAIRED-WITH-RESIDUAL ×6, REPAIRED ×2 |
| (b) vmctl fail-fast/admission | NOT-REPAIRED ×5, REPAIRED-WITH-RESIDUAL ×3 — all 8 require the staging refusal demonstration before "repaired" |
| (c) cutover-aware replay | REPAIRED-WITH-RESIDUAL ×6, REPAIRED ×2 |
| (d) `mergeVocabReports` OGDropped | REPAIRED ×7, REPAIRED-WITH-RESIDUAL ×1 |

Confirmed defects adopted for repair (follow-up commits):

1. Upcaster ledger initialization caches a failed load as loaded
   (`vocab_upcast_deposit.go` `resolve`: `loaded=true` before
   `loadDepositUpcastLedger` succeeds); reproduced by gpt-6.1-sol: replay
   attempt 1 errors `unsupported version`, attempt 2 returns nil. The
   end-of-replay fence still rejected it — contained, but the boundary must
   refuse on every call.
2. Twice-failed checkpoint jobs are terminal even against vmctl's repair
   enqueue, and `urgent_tail`/`warning_tail` mask `repeated_failure`
   (claude): two transient failures silently end cadence for that computer.
3. `ReconcileProjectionJobs` aborts the whole pass on the first per-computer
   enqueue error and the worker records no reconcile error in its heartbeat
   (claude, gpt-6-luna).
4. `go-choir-checkpointd` has no `OOMScoreAdjust`; under host pressure the
   kernel may kill a VM before the worker (claude, gemini, muse).

Named residuals (not repaired in this slice):

- GC grace vs publish→advertise window: the separate checkpointd process is
  not coordinated with corpusd's GC mutex; mtime grace is the only guard.
  Dormant while artifact GC is dry-run on Node B; **must be fixed before
  enabling active GC** (codex, sol, luna, glm, deepseek).
- `WarmUniversalWirePlatformComputer` resumes via `mgr.ResumeVM` without
  recovery admission (muse, glm, deepseek) — platform computer only.
- Lease expiry re-claims a `running` job without fencing on `lease_until`;
  duplicate work, publication is still lease-token fenced (deepseek, muse).
- Failed-job scratch unswept; checkpoint memory unmeasured for large seeded
  bases (owner-scale 8.7 GB base).
- Upcaster ledger sidecar vs projection transaction crash consistency
  (codex); SQL-row ops pass through the upcaster untransformed on the claim
  they are live-only (muse) — needs proof, not assertion.
- Retained-V1 collision repair in `vocab_migrate_og.go` keeps the incident's
  `updated_at` comparison (pre-existing `ca41be90` interim; applies only to
  retained V1 stores' one-time migration), contrary to "no wall-clock LWW".

## Rollback

None — record only. The deployed repair path (manual `bootstrap-chain`
POST) is available to any holder of `computer:lifecycle` scoped to the
computer. The watermark POST is idempotent (monotonic insert).
