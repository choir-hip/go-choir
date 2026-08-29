# Autoputer Priorities — Agentic Consensus (2026-08-29)

Panel run `.agentic-consensus/agentic-consensus-20260829-085934/` (manifest:
11 ok, 1 failed-to-start `omp-x-preview-f`). Prompt:
`.agentic-consensus/prompt-autoputer-priorities-2026-08-29.md`. Mode:
convergent. Grounding: live survey (§  of the same-dated overview report) plus
two read-only archaeology scouts (prior-mission checkpoint promises; code
checkpoint/snapshot state). Load-bearing panel claims spot-verified in-tree
before acceptance (noted below).

## Panel

- codex (default), cursor (default), opencode (default), devin (`swe-1-6-slow`),
  omp gpt-5.6-sol (medium), omp gpt-5.6-luna (max), omp gemini-3.7-flash,
  omp cursor-grok-4.6, omp muse-spark, omp nemotron-3-ultra, omp hy3: ran.
- omp x-preview-f: failed at start (exit 1, 10 s).

## Mode

- convergent.

## Consensus (2+ agents)

### H1 — dispatched→pending Super gap: ADAPT (root cause is occurrence dedup, not a missing activate)

All twelve agree the gap is real; the panel splits on mechanism, and the split
resolves on evidence:

- Shallow read (gemini, hy3, devin, muse): the rewarm reactivation
  (`reactivateRestartedPersistentSuperControlRun`) sets `RunPending` and
  enqueues the recovery occurrence but never calls `rt.activate(rec)` — so
  "fix it by activating."
- Deep read (gpt-5.6-sol, luna, grok46, codex, cursor): do **not** route
  rewarm through the fresh-wake `activate()`/`initial_dispatch` path — its
  actor update identity is stable and may already be processed; adding a
  second occurrence risks double provider execution. The strongest
  source-grounded stall cut (grok46): an older unprocessed
  `initial_dispatch` can run first and passivate the run; the recovery
  occurrence then fails `ResolvePersistentSuperRecovery`, the handler maps
  the error to `nil, nil` (occurrence marked processed), and later rewarm
  re-emits the **same deterministic UpdateID**, which the actor log drops via
  `ON CONFLICT(update_id) DO NOTHING` — the run stays `pending` forever.

**Locally verified before acceptance:** the rewarm path indeed has no
`rt.activate(rec)` (`internal/agentcore/super_controller.go:619-690`), and the
actor log dedups on `ON CONFLICT(update_id) DO NOTHING`
(`internal/actor/log_sqlite.go:46`). The exact live failure mode for
`fe92ea2b` (processed vs deferred vs absent occurrence) is still
unobserved — the actor SQLite row on the guest must be read before the fix.

**Adopted fix shape:** problem receipt first; then a **store-persisted,
monotonically increasing reactivation generation** bound into the recovery
occurrence identity (CAS with the pending transition), stale-generation
rejection at resolution, passivated-as-seen-by-recovery re-arms the same run
(never processes it away), and skip/mark-processed stale `initial_dispatch`
ahead of recovery. Exactly one unprocessed recovery occurrence per
reactivation generation. Two missing tests: unprocessed `initial_dispatch`
followed by recovery; already-processed recovery occurrence followed by
rewarm.

### H2 — replay cost: wire the existing dead ProjectionBase; never build a second checkpoint mechanism

Unanimous adopt-with-adaptation. The user's instinct is confirmed by the
archaeology: generic checkpointing **landed and is complete** (head witness +
40-table Dolt witness + frontend identity; pre-A fence `99949fe2`), but it
stores no projection bytes. A ProjectionBase (content-addressed projection
snapshot + replay watermark for O(Δ) boots) **exists and is dead**:
`RecordReplayWatermark` has zero production callers; the materializer
(`internal/autoputer/projection_base.go`) has an `artifact_ref` vs
`artifact_digest` param mismatch, a base64-JSON envelope mismatch, a
namespace mismatch, runs **after** `store.Open` (so its emptiness gate
short-circuits exactly when needed), and hydrates the wrong path. The offline
rebuilder failed the real tape at seq 2.

Adaptations required by the panel: (a) materialize **before** store open from
the configured path; (b) watermark recorded only after a **verified** base —
digest alone is not trust; bind computer ID, canonical head, reducer/schema
versions, content witness, and a platform-verifiable head receipt; (c) publish
needs a quiesced or point-in-time consistent store; (d) seed both cold boot
`Reconstruct` and `ReplayCompleteness` disposable workspaces from the same
verified base; (e) **wire-or-delete** the rebuilder in the same mission (one
mechanism). Sol's refinement: prefer a quiesced verified live-projection
snapshot at head H joined to checkpoint evidence; else repair the offline
route onto the production canonical receipt source. Note honestly:
ProjectionBase bounds replay *depth*, not the PF5 per-event 3.2–6.5 s I/O
ceiling, which remains open.

### H3 — residual OG scans: finish the cutover with complete pagination, then delete

Unanimous. Named remaining boot callsites confirmed live
(`ListLifecycleRunsByState`, open-work-item sweep, mailbox-all,
passivated-spawned sweep, `GetLifecycleSnapshot`→`ReadObjectSnapshot`,
`ListLifecycleRunsByChannel` fallback, `ListRunsByAgent`), and the callsite
inventory is **not exhaustive** (evidence, residue, run-memory, cosuper paths
also use the full-scan family). Adaptations: fixed `LIMIT` newest-windows are
not drop-in — they can strand older durable obligations; cursor-complete
pagination is required. The current per-field index still `JSON_EXTRACT`s
every matching body — a transition, not a true predicate index (codex).
`GetLifecycleSnapshot` consumers need a transactionally consistent
replacement (single watermark/transaction), not independent point reads (sol,
cursor). Delete `ogListAllByMetadata`/`ReadObjectSnapshot` predecessors when
production callers reach zero — sol/grok46: in the same mission after parity
proof; codex/cursor allow a short canary for red-class paths but never
indefinite dual authority.

### H4 — mailbox backlog: compact on terminal disposition, NOT on delivery (hypothesis amended)

The original hypothesis (truncate on delivery) is **rejected by the deep
readers** and accepted by the shallower ones; the deep position is
source-grounded and adopted: delivered-pending packets are the **recovery
authority** for persistent-Super rewarm
(`PendingDeliveredWorkerUpdateCanonicalIDsByRun`); `DeliveredToRunID` is a
bind/audit fact, not consumption. Truncating at delivery makes rewarm see
`pending=0` and skip — and would have broken the nine-cancel-report resume.
Compact/tombstone only after terminal disposition (incorporated/rejected/
cancelled/acknowledged) in the existing settlement transaction, retaining
minimal causal fields for idempotency/evidence/replay; advance a bounded
per-target cursor. No new sweeper. Do **not** touch the 1,280 rows before the
Super executes them.

### H5 — disk: classify before acting; do not raise the guest cap first

Unanimous (devin: defer entirely). The 98 GiB host figure vs 32 GiB virtual
cap compares different quantities (guest statfs showed ~11/31 GiB). First a
read-only classification of the host VM directory: tape/CAS, overlays,
retained snapshots, logs, Dolt history, actor SQLite. `data.img` expansion
hides growth rather than fixing it. File-CAS/Track M is the durable file
rematerialization substrate but does not dedup tape/overlays, and
`HydrateIfNeeded` currently no-ops on non-empty trees — not yet a repair
verb. Retained snapshots are rollback artifacts; storage pressure does not
authorize deleting them.

### H6 — simplification rule: adopt

Unanimous: a bounded replacement is complete only when its unbounded
predecessor is deleted (migrate callers → parity proof → delete; codex
permits a brief shadow-read interval for red class). ProjectionBase is wired
in one mission or deleted. The canonical CAS tape and the product OG event
stream stay separate by doctrine; mirrored projection tables must not become
competing authorities. Declare ownership/retirement for every mechanism
family (checkpoint head-witnesses, ProjectionBase bytes, residue snapshots,
actor snapshots, replay checkpoints).

## Dissent / Disagreements

- H1 mechanism (activate vs generation) — resolved by local verification +
  evidence depth in favor of the generation fix; the naive-activate camp is
  right that activation is absent, wrong that adding it back is safe.
- H4 timing (truncate-on-delivery vs terminal-disposition) — resolved in
  favor of terminal disposition; delivery-time truncation breaks the
  currently-live recovery path.
- H5 urgency (devin: defer; gemini: immediate stopgap) — middle position
  adopted: read-only classification immediately (cheap), mutation deliberate.

## Unique High-Value Findings

- grok46's stranded-run causal chain for `fe92ea2b` (processed recovery
  occurrence + deterministic UpdateID dedup) — the best available explanation
  for a 13-h frozen `pending` on a healthy computer; to be confirmed by
  reading the live actor SQLite row (explicitly labeled inference until then).
- cursor/luna: `docs/ACTIVE.md`/`NOW.md` registry drift vs the candidate
  Definition's owner-directed resume — reconcile before the next Definition
  beat; the Overhauls definition's Track F "implemented" status is an
  overclaim (no watermark producer exists).
- codex: the H3 replacement needs body/index write-path coupling — indexed
  fields must be updated transactionally by one canonical write path or they
  drift.
- grok46: `bootDispatches` unused vs `log.Append`+Sweep; `choir.event` Seq is
  still allocated by OG scan while the tape is restore authority — freeze as
  derived.

## Low-Confidence / Unverified Claims

- The exact live state of `fe92ea2b`'s actor occurrence (processed/deferred/
  absent) — read the guest actor SQLite before coding H1.
- Guest-native Dolt v2 performance (PF5 ceiling) — never measured in a real
  Firecracker guest; host-equivalent numbers only.
- `ListJSONBodyFieldsByKindOwner` cost claims (still row-wide JSON evaluation)
  — plausible from source, not benchmarked.

## Recommendation

1. **P0 receipt + H1 fix now**: read `fe92ea2b`'s actor occurrence on the
   guest; write the problem receipt; implement the reactivation-generation
   fix with the two missing tests; live-proof the Super consumes the nine
   cancel reports and binds fresh CoSuper; then run candidate A (effects OFF).
2. **Parallel, read-only**: host VM-dir classification (H5 step 1); registry
   drift reconciliation (ACTIVE.md/NOW.md/Overhauls status).
3. **Next landing after P0**: H3 cursor-complete cutover + H4
   terminal-disposition compaction (shared OG substrate; serial landings).
4. **Then H2** (ProjectionBase wire-or-delete after the trust contract is
   frozen), before the E2 restore work which needs O(Δ).
5. **Then** H5 reclaim and P2s (health debounce, Track M wiring, zombie op
   closure through the canonical path).

## Raw Outputs

- `/Users/wiz/go-choir/.agentic-consensus/agentic-consensus-20260829-085934/`
  (`manifest.tsv`, per-agent `.out`, `.cmd`).
- Archaeology inputs: two scout briefings (prior-mission promises vs landed
  SHAs; code checkpoint/snapshot inventory), summarized in the prompt file.
