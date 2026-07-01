# Mission M1 — Trajectory Model (cutover step 2; the §15 proof mission) — v0

Source: `docs/mission-portfolio-2026-06-11.md` §M1. Program:
`docs/choir-rearchitecture-durable-actors-2026-06-11.md` (cutover step 2,
§2.3). Discipline: `skills/parallax/SKILL.md` (this mission adjudicates
Parallax's promotion — `docs/parallax-design-2026-06-11.md` §5; report
honestly). Vocabulary: `docs/glossary.md` (trajectory, work item, settlement
— glossary.md:541–558, already defined).

## Source form (from the portfolio, verbatim intent)

**Real artifact:** durable trajectory records (kind, subject refs, status,
settlement rule as data) + `trajectory_id` on runs + work items (the ported
continuation mechanics: objective, bounded authority profile, step/token
budgets, fingerprint dedup — **no lease vocabulary in v1**), in the runtime
store. Additive; no control-flow change.

**Bridge conjecture (N2′):** one durable object — trajectory + work items —
replaces every parent/child control use with no loss of budget, cancellation,
or provenance function. *Falsifier:* a control use surfaced in the review-v2
inventory that trajectories cannot express. *Edge (frame_lock):* settlement
rules per trajectory kind may be wrong in ways the current vocabulary cannot
state; define each rule as data and review after first real cycle.

**Settlement:** records exist, are minted at conductor/processor/VText spawn
points, and are queryable ("what is this trajectory waiting on?"); zero
behavior change proven by the existing suite staying green.

**Evidence class:** example tests + the wire_pipeline.tla mapping (model ∀
transfers only via conformance — say so in the report).

**Size:** 1 overnight mission.

## Parallax State

status: settled (2026-06-12; scope: additive witness — see settlement)

**mission conjecture:** if durable trajectory + work-item records are added
to the runtime store, minted at conductor/processor/VText spawn points, with
settlement rules as data and zero behavior change (suite green), then the
deeper rearchitecture goal advances by making parent/child replacement (M2–M4)
and settlement accounting (M5) executable rather than documentary.

**deeper goal (G):** durable actors, evidence-bearing promotion, and
self-development operational instead of documentary (portfolio G).

**witness/spec (A/S):** new store tables `trajectories` + `work_items`, a
`trajectory_id` column on `runs`, minting at the spawn points, and an
open-obligations query. Spec is the §2.3 table in the rearchitecture doc plus
the glossary definitions.

**invariants / qualities / domain ramp (I/Q/D):**
- I: additive only — no control-flow change; existing suite stays green;
  no lease vocabulary; historical `parent_loop_id` data frozen, never migrated.
- Q: settlement rules stored as data, not Go branches; records queryable.
- D: starts at unit/example-test scope; embeds in production via M5 (the
  first real settlement-driven cycle). No claim of settlement *correctness*
  here — only existence + expressibility; correctness is M5's domain.

**authority / bounds:** repo changes on a branch; no canonical mutation; no
behavior change to the running product. Landing proof (commit, push, CI)
required in this document before settlement.

### Position — code inventory (compiled 2026-06-11, pre-start)

What this observer can already see cheaply:

1. **`trajectory_id` already exists — as a derived string, not a record.**
   - `runMetadataTrajectoryID = "trajectory_id"` (runtime.go:2356); minted by
     `ensureTrajectoryID` (runtime.go:2364): explicit metadata → parent's
     metadata → parent.RunID → selfRunID.
   - Trace derives it with a further fallback chain: metadata → ChannelID →
     RunID (`traceTrajectoryIDForRun`, api_trace.go:1508).
   - Already a column on `events`, `channel_messages`, `inbox_deliveries`,
     `worker_updates`, `research_findings` (store.go:93–341) and a field on
     acceptance/evidence/task types (internal/types).
   - **Implication:** M1 promotes a derived ID into a durable record. The
     durable trajectory's ID should adopt the ID the existing derivation
     produces, so every existing column/Trace key joins against the new table
     for free. The `runs` table itself has **no** trajectory_id column today
     (it lives in metadata_json) — adding the column is part of the witness.

2. **Store + migration mechanism:** `internal/store/store.go` (2,229 lines);
   `CREATE TABLE IF NOT EXISTS` schema block + `ensureColumn` ALTER-TABLE
   migrations (store.go:553–646). `run_continuations` (store.go:261) is the
   closest existing record shape and the port source for work items.

3. **Continuation mechanics to port** (`internal/runtime/continuation.go`,
   post-M1a — the synthesis decision layer was deleted 2026-06-12, see
   portfolio §M1a; what remains *is* the port source):
   - objective + reason + Details (ContinuationProposal);
   - bounded authority profile (`boundedContinuationProfile` — vsuper /
     co-super / researcher only);
   - fingerprint dedup (`objectiveFingerprint` — sha256 over owner +
     trajectory + parent + normalized objective; synonym folding removed in
     M1a, resolving the normalizer question by deletion; dedup check
     `existingRunContinuationForObjective`);
   - **not ported:** record-layer lease clamps (retire with the table in
     M4). Step/token budgets come from the actor model's vocabulary
     (rearchitecture §2.1), not from continuation code.

4. **Spawn points to mint at:**
   - conductor: `HandlePromptBar` → `completePromptBarDecisionRun`
     (api.go:359 ff.) and `StartRunWithMetadata` (runtime.go:376);
   - child/worker spawns: `StartChildRun` (runtime.go:508) — callers
     tools_coagent.go:181, vtext_agent_revision.go:347, vtext.go:1983/2103,
     StartRunContinuation (continuation.go:167);
   - processor: sourcecycled dispatches processor requests through the cycle
     queue (cmd/sourcecycled/main.go:567 ff.); the exact runtime-side spawn
     needs one probe (open question Q1 below).

5. **Parent/child control inventory (the N2′ falsifier hunting ground)** —
   M1 changes none of these; it must show trajectories + work items *can
   express* each:
   - liveness: `!isTerminalRuntimeState(run.State) || run.ActiveChildRuns > 0`
     (cmd/sourcecycled/main.go:590, field :48) → trajectory settlement (M5);
   - cancellation cascade: `CancelRunGraph` (runtime.go:828) →
     cancel-by-trajectory (M3);
   - completion signaling: `notifyParent` (runtime.go:1197, 1331, 2342,
     2454) → update_coagent (M2);
   - restart blanket-fail: `recoverInterruptedRuns` (runtime.go:974) → cold
     actors + sweep (M3);
   - co-super slot keying (parent, slot) (runtime.go:534–545 per review v2)
     → (trajectory, slot) (M2);
   - provenance: `parent_loop_id` column (store.go:77) → `spawned_by_run_id`
     provenance-only (M3).

6. **`internal/actor` is ready and waiting:** `actor.Update` already carries
   `TrajectoryID` (actor.go:33–41); Log interface + SQLiteLog exist; no
   trajectory or work-item types exist anywhere yet (grep confirms — only the
   derived-string uses above).

7. **Tests that must stay green:** the runtime suite is the blast radius —
   notably api_trace_test.go (trajectory grouping), continuation_test.go,
   parent_child_channel_test.go, concurrent_workers_test.go,
   run_acceptance + universal_wire tests, and `internal/actor` tests.

Blind spots from this position (edge classes named):
- **missing_oracle:** no query exists for "what is this trajectory waiting
  on?" — building it *is* part of the witness, and until it exists, claims
  about expressibility are untested.
- **frame_lock:** settlement-rule vocabulary per kind (publication, vtext
  revision, coagent task, …) may be wrong in ways the current language can't
  state. Define rules as data; review after M5's first real cycle.
- **independence:** the wire_pipeline.tla mapping transfers only via
  conformance — example tests are existential; say so in every claim.

### Initial conjectures

- **C1 (bridge, N2′):** trajectory + work-item records can express every
  entry in the control inventory above with no loss of budget, cancellation,
  or provenance function. *Test:* a written mapping per inventory entry plus
  example tests; falsified by any inventory entry without an expression.
- **C2 (identity):** adopting the existing derived trajectory_id as the
  durable record's primary key preserves all existing Trace/event/update
  joins with zero data migration. *Test:* Trace renders identically for a run
  chain before/after; events query joins to the new table.
- **C3 (additivity):** the records can be minted at all spawn points with
  zero behavior change. *Test:* full suite green with minting on.
- **C4 (Parallax overlay):** running this mission as a conjecture circuit is
  cheaper to resume/retrospect than MissionGradient, and at least one SHIFT
  changes the route. *Test:* the promotion criteria in
  parallax-design-2026-06-11.md §5; failure = circuit fields filled while
  moves stay identical to old behavior. Report honestly either way.

### Open questions (first probes)

- **Q1:** where exactly does the processor spawn land runtime-side from
  sourcecycled's dispatch queue? (One grep/trace probe; needed for the
  minting list.)
- **Q2:** which trajectory kinds exist in v1 and what is each settlement
  rule as data? Candidate set from current spawn surfaces: `vtext_document`,
  `publication`, `coagent_task`, `mission`. Smallest honest set wins;
  rules reviewable after M5.
- **Q3:** do work items subsume the `run_continuations` table now (write
  path moves, table frozen) or does M1 only add the new table and M4 retires
  the old one? Portfolio says additive — default: add only, freeze decision
  to M4 — but confirm against acceptance-evidence readers. (Post-M1a the
  only writers are the explicit-objective API path and
  `maybeStartConfiguredContinuation`.)

**ledger / move log:**

- 2026-06-12 PROBE (Q1): all spawn surfaces funnel through three CreateRun
  sites — `createRunWithMetadata`/`persistSubmittedRun`,
  `completePromptBarDecisionRun`, `StartChildRun`, plus the email tool's
  direct CreateRun; processor runs enter via `HandleInternalRunSubmission` →
  `StartRunWithMetadata`. Minting at those sites covers conductor,
  processor, VText, coagents, email. *Receipt:* api.go:883→StartRunWithMetadata;
  runtime.go createRunWithMetadata/StartChildRun; tools_email.go.
- 2026-06-12 SETTLE (Q2): v1 kinds = document | publication | task, derived
  from spawn profile (processor → publication; conductor/vtext/email →
  document; else task). Settlement rules as data:
  `types.SettlementRule{RequireNoOpenWorkItems, RequiredSubjectRefs}`;
  publication additionally requires `publish_ref`. Reviewable after M5's
  first real cycle (the frame_lock edge stays open and named).
- 2026-06-12 SETTLE (Q3): additive only — work items are a new table; the
  `run_continuations` write path is untouched; freeze decision stays with M4.
- 2026-06-12 CONSTRUCT: `internal/types/trajectory.go` (TrajectoryRecord,
  WorkItemRecord, SettlementRule as data); `trajectories` + `work_items`
  tables + CRUD (`internal/store/trajectory.go`); `runs.trajectory_id`
  column (CREATE TABLE + ensureColumn migration + post-migration index);
  `RunRecord.TrajectoryID`; minting helper `stampAndMintTrajectory`
  (`internal/runtime/trajectory.go`) wired at all four spawn sites,
  best-effort so a mint failure can never fail a spawn (C3 invariant);
  open-obligations query `TrajectoryObligations` + pure
  `EvaluateTrajectorySettlement`; GET /api/trajectories and
  /api/trajectories/{id} (owner-scoped).
- 2026-06-12 PROBE (C2, C3): example tests green — store CRUD/idempotent
  mint/fingerprint dedup (completed blocks reuse, cancelled releases),
  runtime minting at conductor + processor spawns, child joins parent's
  trajectory without a second record, obligations query answers
  "waiting on". Full default `go test ./...` green except
  `TestIngestionRuntimeDispatcherSubmitsProcessorProfilesOnly`
  (cmd/sourcecycled) — **pre-existing**: fails identically at 52e6baef,
  before any of this session's changes; likely date-sensitive stale-request
  cleanup flake; out of M1 scope, flagged for follow-up.
- 2026-06-12 SHIFT (vocabulary, founder-directed): provenance is tracked
  without parent/child vocabulary entirely — `spawned_by` is a past-tense
  event fact, not a present-tense relationship; the relationship reading is
  where the retired bug classes lived. Glossary updated (new "provenance
  (spawned_by)" entry; parent/child retirement strengthened to cover
  provenance prose). M1's own tests scrubbed. Code identifiers
  (`ParentRunID`, `StartChildRun`) rename in M3 as already scheduled.
- Evidence-class note: all tests are existential (example runs); the
  wire_pipeline.tla mapping transfers only via conformance. No universal
  claim is made.

### C1 mapping — control inventory → trajectory/work-item expression

Vantage shift: each entry read as its consumer mission will use it. This is
the bridge test for N2′ at the additive stage; the *behavioral* falsifier for
each row runs in the mission that flips it.

| Control use (today) | Expression (new records) | Consumer / falsifier |
|---|---|---|
| liveness: `isTerminalRuntimeState && ActiveChildRuns == 0` (sourcecycled main.go:590) | `TrajectoryObligations.SettlementReady` — open work items + rule refs, queryable now | M5 flips the reconcile condition; falsifier: multi-story cycle at maxProc>1 |
| child budgets per parent (`enforceChildSpawnBudget`) | count open work items / live runs per `trajectory_id` (indexed column) | M2/M3; expressible — `idx_runs_trajectory_id` makes it one query |
| cancellation cascade `CancelRunGraph` (runtime.go:828) | cancel-by-trajectory: `UpdateTrajectoryStatus(cancelled)` + cancel runs WHERE trajectory_id | M3; record + column exist, the cascade rewrite is M3's |
| completion signaling `notifyParent` | `update_coagent` carrying `TrajectoryID` (already a field on `actor.Update`) | M2 |
| co-super slots keyed (parent, slot) | re-key (trajectory, slot): trajectory_id now durable on every run | M2 (named riskiest migration) |
| provenance `parent_loop_id` | unchanged — provenance only; trajectory carries control; `spawned_by_run_id` rename is M3's | M3 |
| continuation record (objective/authority/fingerprint) | `WorkItemRecord` — ported with dedup semantics (completed blocks reuse, cancelled releases) | M4 deletes the old table |

No inventory entry found that the records cannot express → C1 **supported at
the expressibility scope only** (existential evidence; behavioral transfer
is per-row, in M2–M5).

**version / lineage:** v0, compiled 2026-06-11 from portfolio M1 + code
inventory. Predecessors: none. Successors gated on this: M2 (messaging), M5
(wire on settlement — can start once records exist).

**learning state:** retained here.

**settlement:** open_handoff → settled pending CI. Evidence so far:

- Witness landed: `trajectories` + `work_items` tables with CRUD
  (internal/store/trajectory.go), `runs.trajectory_id` column + index
  (fresh-DB CREATE TABLE + ensureColumn migration + post-migration index),
  `RunRecord.TrajectoryID`, minting at all four spawn sites
  (internal/runtime/trajectory.go), `TrajectoryObligations` query +
  `EvaluateTrajectorySettlement` (pure), GET /api/trajectories[/{id}].
- Tests: store trajectory/work-item suite green; runtime minting +
  obligations suite green; comprehensive-tag subset (continuation, run
  API, trajectory) green; full default `go test ./...` green except the
  **pre-existing** sourcecycled dispatcher flake (fails at 52e6baef,
  before this session — not an M1 regression; follow-up candidate).
  Live-LLM tests are env-gated and skipped (no credentials set; zero API
  spend). Full comprehensive matrix delegated to CI.
- C1 mapping written per inventory entry (table above); supported at
  expressibility scope only.
- C4 (Parallax adoption) verdict, honestly: the conjecture framing did
  change the route — the C2 identity decision (adopt the derived ID as
  the durable key) came directly from the position probe and avoided a
  data migration; the vocabulary shift (spawned_by, founder-directed)
  changed glossary doctrine and this mission's own artifact prose; M1a
  (pre-mission deletion) came from interrogating the bridge rather than
  executing the plan as written. Moves were not identical to
  MissionGradient behavior. One honest caveat: the SHIFT that "changed
  the route" most was founder input mid-circuit, not an autonomous
  observer move — count it as evidence for the discipline's
  escalation-surface, not for autonomous shift selection.

**Landing proof** (required for settlement):
- commits: b508f71c (M1a synthesis deletion), 14479930 (Parallax v1.2.0
  docs + dead trace.js deletion), a31fd2b5 → amended (M1 witness)
- push: origin/main 52e6baef..554d25de (2026-06-12).
- CI run 27395049664: all four internal/runtime shards green, TLA+ model
  check green, integration smoke green, Go Vet + Build green, frontend
  build green. Failing jobs contain exactly the two failures already red
  on the baseline run (27384348346, commit 52e6baef, before this
  session): `TestIngestionRuntimeDispatcherSubmitsProcessorProfilesOnly`
  (cmd/sourcecycled) and
  `TestHandleInternalWirePlatformPublishPostsToCorpusd`
  (internal/proxy). **No new failures introduced — the zero-behavior-
  change claim holds at CI scope.**

**Settlement verdict:** settled, at the scope claimed: the witness exists
(records, minting, column, queryable obligations), evidence is existential
(example tests) + CI conformance; no claim about settlement *correctness*
on real traffic — that is M5's domain. Residual risks, accepted and named:
1. Two pre-existing CI reds gate the staging-deploy job; until fixed,
   every later mission's landing proof is blocked at the same gate.
   Smallest discharge: fix or quarantine the two tests (follow-up, not
   M1 scope — both predate this session).
2. The frame_lock settlement-rule edge stays open: v1 kinds and rules are
   reviewable after M5's first real cycle.
3. Staging schema migration (`trajectories`, `work_items`,
   `runs.trajectory_id`) has not executed against staging because deploy
   skipped on the pre-existing reds; it runs automatically once the gate
   clears (ensureColumn path is boot-time and idempotent).
