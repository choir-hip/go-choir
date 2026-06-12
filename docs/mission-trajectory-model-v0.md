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

status: proposed (not started)

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

**ledger / move log:** (empty — mission not started)

**version / lineage:** v0, compiled 2026-06-11 from portfolio M1 + code
inventory. Predecessors: none. Successors gated on this: M2 (messaging), M5
(wire on settlement — can start once records exist).

**learning state:** retained here.

**settlement:** not settled. Exit requires: tables + column landed; minting
at conductor/processor/VText spawn points; open-obligations query working;
suite green; landing proof (commit/push/CI) recorded here; C1 mapping written
per inventory entry; honest C4 verdict for the Parallax adoption gate.
