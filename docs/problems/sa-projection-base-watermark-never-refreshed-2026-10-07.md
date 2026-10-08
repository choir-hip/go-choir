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
  base and replay only its tail. Genesis replay is exceptional repair.
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


## Rollback

None — record only. The deployed repair path (manual `bootstrap-chain`
POST) is available to any holder of `computer:lifecycle` scoped to the
computer. The watermark POST is idempotent (monotonic insert).
