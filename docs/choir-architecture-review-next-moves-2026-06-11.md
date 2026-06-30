# Architecture Review Against the Conjecture Program — Next Moves — 2026-06-11 (v2)

## Status

Conjecture-program artifact, second pass. v1 was written against truncated
companion handoffs; this version is written against the full handoffs
(re-imported 2026-06-11) plus two additional deep code passes: a full dissection
of the continuation system and a complete inventory of the parent/child run
ontology with removal in mind.

Founder direction incorporated: **be aggressive about removing bad ontologies.**
The working claim — supported by what this review found — is that ontology
repair improves agentic engineering pace more than its line-count suggests,
because a bad ontology is a heresy generator: every agent that reads the code
re-derives the wrong invariant, and every mission pays the tax. v1's
"demote-then-replace" is superseded by a single replacement program (§4–5).

Method: claims checked against `internal/`, `cmd/`, `skills/`, prompts, and
tests, with file:line receipts.

Doctrine note (2026-06-13): terms such as `continuation-level` and Trace
references are preserved here as evidence labels from the reviewed system. They
are not current target ontology; current doctrine treats `continuation-level`
as transitional residue and Trace as evidence rather than a user app surface.

---

## 0. Process finding — the companion handoffs were truncated (RESOLVED)

At first review, both companion handoffs ended with a literal Read-tool
truncation banner inside the file, missing ~70% / ~64% of content (the §15
proof mission, MutationTransaction milestones, research backlog, recursion
machinery). Full versions were re-imported the same day
(`handoff-conjecture-learning-fixed-point` now 1014 lines,
`handoff-hybrid-computer-capsule-architecture` now 830 lines).

**Invariant candidate:** documents imported from tool output must be verified by
line count or tail inspection. An unverified import is scope exceeding reach in
the docs layer — a heresy vector by the program's own definition.

---

## 1. The continuation system, dissected

The conjecture program flags "general continuation/orchestration" as
semantically empty. The code review found something sharper: **"continuation"
names two unrelated mechanisms**, and conflating them misdiagnosed the Wire
incident.

### 1.1 Mechanism one: `RunContinuation` (the runtime's "what next?" machinery)

A RunContinuation is a durable record meaning: *"run X finished or blocked;
the next bounded objective is Y; run it as profile P with lease L."*
(Historical description of the continuation record; "lease" is retired in
v1 vocabulary — see the durable-actors doc's lease deferral.)
(`internal/runtime/continuation.go`, `run_continuations` table,
`internal/store/continuations.go`.)

Lifecycle:

1. **Synthesize** (`SynthesizeRunContinuation`, continuation.go:31) —
   deterministic and conservative. Priority: pending AppAdoption work
   (proposed → verify, verified → promote/rollback, blocked → recover;
   continuation.go:238-318) beats the fallback, which is open-ended
   mission-gradient continuation against a mission doc — **hardcoded default
   `docs/archive/mission-choir-grand-deformation-v0.md`** (continuation.go:48).
2. **Select** (continuation.go:77) — requires source run state
   Completed/Blocked (line 85: the run-tree entanglement), runs a
   run-memory **compaction checkpoint** so the successor has operational
   memory, dedups by objective fingerprint
   (owner + trajectory + source run + normalized objective text), records
   status `selected`.
3. **Start** (continuation.go:139) — spawns the objective **as a child run via
   `StartChildRun`** with metadata `request_source: "run_continuation"`,
   status → `started`. Authority is hard-bounded by
   `boundedContinuationProfile` (continuation.go:364-375): **only vsuper,
   co-super, or researcher. Never super.**

Triggers: auto-start from run metadata at completion
(`maybeStartConfiguredContinuation`, continuation.go:213), the
`/api/continuations` endpoints (api.go:568-664), and compaction-recall eval
retries (api_compaction_eval.go:369-401).

### 1.2 What load RunContinuation bears

| Consumer | Dependency | Severity |
|---|---|---|
| **App adoption pipeline** | every state transition (proposed→verified→promoted/rolled-back) is driven by a synthesized continuation; without it adoptions freeze after candidate-apply | critical |
| **Autonomous mission resumption** | the mission-gradient fallback is what keeps long missions moving without manual re-prompting | critical |
| **Acceptance evidence** | "continuation-level" acceptance requires compaction + child-spawn proof (run_acceptance.go:1012-1014; AGENTS.md rule) | high |
| **Compaction-recall evals** | failed evals retry via researcher continuations | medium |
| **Trace evidence / API** | continuation events still project into trace evidence moments (api_trace.go:1421-1430); do not treat this as a Trace app direction | low |

The valuable ideas inside it: deterministic next-goal selection, compaction
before handoff, fingerprint dedup, bounded authority. (Lease clamps are NOT
salvaged — v1 uses step/token budgets and activation caps; see the
durable-actors doc.) None of
these require the run-tree shape — but the record is keyed by `SourceRunID`
and starts its successor via `StartChildRun`, so **the good machinery is
welded to the bad ontology.**

### 1.3 Mechanism two: channel handoff to persistent super (the actual Wire leak)

The "super-owned continuation on the same channel" from the Wire trace was
**not** a RunContinuation. RunContinuations cannot run as super
(continuation.go:370). What happened mechanically: VText called
`request_super_execution` (tools_vtext.go:194-237), which `ChannelCast`s the
objective to the **persistent super agent on the same document channel**. Super
then worked the channel — a coagent, not a child, invisible to both
`ActiveChildRuns` accounting and the RunContinuation ledger.

**Consequence:** Conjecture D's leak is not fixed by tightening
RunContinuation, and Conjecture E's "no vague continuation" rule was aimed at
the wrong mechanism. The leak lives in channel-level coagent work having no
durable trajectory accounting. The word "continuation" being overloaded across
these two mechanisms is itself evidence for the founder's thesis: the
vocabulary confusion produced a misdiagnosis in our own program doc.

---

## 2. Verdicts on conjectures A–G (updated)

### A (sandbox → autoputer) — SUPPORTED; data model already clean

~1,387 occurrences, but **zero `sandbox` database columns** (schema uses
`owner_id`/`computer_id`/`desktop_id`) and `docs/computer-ontology.md` already
disclaims the name. Rename is symbols + service + env vars
(`SANDBOX_ID` etc.), not data. Do product-ontology/docs now (hybrid handoff
Milestone 0); code rename lands with the capsule distinction so the rename
actually marks a real boundary.

### B (capsules) — SUPPORTED; genuine greenfield

No ephemeral execution exists: bash runs directly in the user VM
(`tools_coding.go:369`); the only isolation is heavyweight worker-VM
delegation. Substrate is ready (Firecracker/vmctl/NixOS), promotion machinery
one-third built (`internal/types/app_promotion.go`; patchset path deliberately
pruned per `legacy-promotion-experiments-learnings.md`). Follow hybrid handoff
Milestones 0–8; the research backlog (Nucleus audit, effect capture, snapshot
strategy, Qdrant placement) is design work that doesn't block §5.

### C (platformd → corpusd) — RIGHT, milder than conjectured

`platformd` is already cleanly corpus-scoped (publication lifecycle, 19 tables,
no VM/lifecycle ownership; internal routes auth-gated). Rename is SMALL-MEDIUM
and safe now. The real finding: **no canonical `publication_id` in the
runtime's model** — publication identity is post-hoc metadata strings on
revisions (`platformd_publication_ref`). Promote publication identity to a
design item; it keys the trajectory model (§3).

### D (parent/child wrong as primary causality) — STRONGLY SUPPORTED; upgraded from demote to REPLACE

Load-bearing control uses confirmed:

| Use | Location |
|---|---|
| processor queue completion = terminal AND `ActiveChildRuns == 0` | `cmd/sourcecycled/main.go:590` |
| continuation eligibility gated on source terminal state | `continuation.go:85` |
| processor admission `maxProc=1` stopgap | `api.go:916-926` |
| parent-notify on child completion (`notifyParent`) | `runtime.go:1197,1331,2454-2498` |
| child budgets (`maxVSuperActiveChildRuns=2`, `CountActiveChildRuns`) | `runtime.go:633-646`, `store.go:895-919` |
| cancellation cascade (`CancelRunGraph`) | `runtime.go:830-872` |
| co-super slot sequencing by parent+slot | `runtime.go:549-703` |
| channel wiring (`ensureParentChildChannels`) | `runtime.go:559` |

The last week of fixes (436490f4, 362a0ded, 46a8ece6, e5ec5f74) are all
compensation for this invariant, and §1.3 shows the compensation **cannot
converge**: coagents on a channel are structurally invisible to a run tree.
Replacement inventory is in §3; every control use has a trajectory-shaped
replacement. What parent/child keeps: nothing with control semantics — one
frozen provenance field.

### E (VText delegation too broad) — RESTATED

Tool scope is already code-enforced: `AllowedDelegateTargets = [researcher]`
(`tool_profiles.go:223-229`, enforced by `canDelegateTo`, line 314); no
co-super/vsuper routing is possible. Record this as a done assertion with
receipts. The remaining problem is §1.3 — channel handoffs to persistent super
carry no artifact/authority/settlement accounting. That is solved by the
trajectory model (work items, §3.2), not by further VText restrictions.

### F (over-coupled processor) — SUPPORTED; one missing piece

Every stage of the proposed decoupled pipeline already exists and is durable
(IngestionEvent → ProcessorRequest → VText handoff → autonomous publish →
edition update → derived stories projection) **except** the durable
candidate/decision ledger: dedup conclusions, publish decisions, and spawn
decisions are transient in-run reasoning
(`store/vtext.go:444-490`, `wirepublish/eligibility.go:26-55`,
`vtext_handoff.go:43-102`). The ledger is absorbed into the trajectory model
(§3.3) rather than built as a Wire-only special case.

### G (conjecture-native MissionGradient) - superseded by Parallax at skill layer

Historical wording: this review called for a conjecture-native
MissionGradient. That work became Parallax/paradocs. Per conjecture handoff
section 12 ("start small, do not overbuild") and section 15, the proof vehicle
is now the ordered rearchitecture portfolio run through Parallax. Typed records
only after proof missions show changed action/verifier/stopping-condition.

Side finding: the mission-gradient fallback objective hardcodes a mission doc
path in Go (continuation.go:48) — control state living in a code constant
rather than durable state. The replacement (§3.2) should carry the mission ref
as work-item data.

---

## 3. The replacement ontology: trajectory as the causality object

The aggressive move is not deleting fields — it is making **trajectory** the
first-class object that runs, work, liveness, budgets, and cancellation all key
on, and reducing parent/child to a frozen provenance edge.

Already in place: `trajectory_id` threads through events, channel messages,
continuations, and worker updates (338 occurrences); `channel_id` is on the
runs table. Missing: trajectory on `runs` itself, and durable trajectory state.

### 3.1 Trajectory record (new)

Durable object keyed by `trajectory_id`, carrying: `owner_id`, kind
(publication / adoption / mission / user_session), subject refs
(`doc_id`, `publication_id`, `adoption_id`, `mission_ref` as applicable),
status (live / settled / cancelled), and an explicit **settlement rule**
result. For Wire publication trajectories, settlement = no non-terminal runs
on the trajectory AND publish ref recorded AND edition updated (reviewed after
first real cycle). This is the object the conjecture program calls
"artifact-scoped liveness."

### 3.2 TrajectoryWorkItem replaces RunContinuation

Same machinery, re-keyed: objective, reason, bounded authority profile,
step/token budgets (no lease vocabulary in v1), fingerprint dedup, compaction checkpoint, status
(selected/started/blocked/done) — but `source` is the **trajectory** (with a
provenance ref to the proposing run/agent), not a run-tree edge, and starting
one does not require the proposer to be terminal. Three immediate wins:

- the app-adoption state machine keys on adoption/trajectory, not on
  `SourceRunID` string-matching (`appAdoptionImpactsSource`,
  continuation.go:260-268, currently matches by substring — fragile);
- **channel handoffs to persistent super become work items too** — the §1.3
  invisible-coagent class disappears because giving super work on a document
  channel creates a durable, settle-able record naming artifact + authority +
  objective (Conjecture E's rule, enforced by shape instead of validation);
- the mission-gradient fallback carries `mission_ref` as data.

### 3.3 The Wire candidate ledger = publication trajectories + work items

Conjecture F's missing ledger is not a separate table: a processor evidence
pass opens a **publication trajectory** per candidate story (carrying coverage
decision and publish decision as trajectory data), VText spawn/revision and
publish are work items on it, and `sourcecycled` queue accounting
(`main.go:590`) reconciles against **trajectory settlement** instead of
`isTerminalRuntimeState && ActiveChildRuns == 0`. `maxProc` rises above 1 when
settlement accounting proves leak-free — the stopgap retires on evidence.

### 3.4 Per-class replacements for every parent/child control use

| Class | Today | Replacement |
|---|---|---|
| liveness/completion | terminal state + ActiveChildRuns | trajectory settlement (§3.1) |
| parent-notify | child posts to parent channel keyed by parent run | post to trajectory/document channel (already where coagents talk) |
| budgets | per-parent child caps | per-trajectory + per-owner active-run caps (identity available at spawn) |
| cancellation | `CancelRunGraph` recursion over parent links | cancel-by-trajectory (terminal-izes all non-terminal runs + work items on it) |
| co-super slots | `activeChildRunForCoSuperSlot` by parent+slot | slot registry keyed (trajectory_id, slot) — the riskiest single migration; do it explicitly, with its own test |
| channel wiring | `ensureParentChildChannels` | explicit channel join at spawn (the channel is trajectory/document-scoped already) |
| provenance/UI | `ParentRunID` in API/trace | rename to `spawned_by_run_id`, read-only, no control reads permitted |

Historical data: `runs.parent_loop_id` (10k+ rows) stays as a frozen column —
no backfill, no migration; new code simply stops reading it for control.
Tests asserting on ParentRunID (~50 sites) migrate with their features.

---

## 4. Why aggressive beats gradual (the pace argument)

The founder's claim, now with receipts: bad ontology taxes agentic development
superlinearly, because the primary readers of this codebase are agents that
re-derive invariants from structure.

1. **The compensation series does not converge.** Four commits in one week
   patched run-tree accounting (436490f4, 362a0ded, 46a8ece6, e5ec5f74) and the
   class of leak (channel coagents) remains structurally unrepresentable. Every
   future shared-artifact feature re-pays this.
2. **The ontology already misled its own architects.** The conjecture program
   attributed the Wire leak to "continuation" latitude; §1.3 shows the leaky
   mechanism was a different one wearing the same word. Vocabulary debt
   produced a wrong fix plan inside the very document designed to prevent
   wrong fix plans.
3. **Dual models are the worst state.** "Demote then replace" means every new
   feature decides which causality model to consult, and agents see both shapes
   in context windows. A short, decisive replacement program minimizes the
   double-bookkeeping window instead of institutionalizing it.
4. **The replacement is mostly relocation, not invention.** `trajectory_id`
   is threaded everywhere except `runs`; the continuation machinery's good
   parts (compaction, fingerprints, bounded authority) port unchanged;
   Wire eligibility already uses metadata lineage, not run state.

**Named hyperthesis edges of the aggressive path** (kept, not hidden):

- *Settlement-rule risk:* a wrong settlement definition reproduces the leak
  with new vocabulary. Mitigation: rule is explicit data on the trajectory,
  reviewed after the first real cycle; falsifier defined in §6/N2.
- *Slot-sequencing risk:* co-super slot semantics are the one place a subtle
  break is likely. Mitigation: dedicated migration step with its own tests.
- *Big-bang risk:* the program is one ontology but should land as ~5 PRs with
  the trajectory record first and control-read cutovers gated on tests — one
  program, not one commit.
- *Acceptance-evidence continuity:* "continuation-level" acceptance language
  (AGENTS.md, run_acceptance.go) must be re-pointed at work items in the same
  program or the verifier discipline silently weakens.

---

## 5. The program (one mission, ordered)

Run through Parallax with a paradoc and ledger (this is also the section 15
proof mission: one mission family discharges N5 and the rearchitecture's first
slice).

1. **Docs first** (grand synthesis §6.1): ontology chapter — trajectory,
   work item, settlement, autoputer/capsule/candidate/corpusd vocabulary;
   record Conjecture E's tool-scope half as a receipted assertion; heresy sweep
   for "sandbox" in product contexts and for the overloaded "continuation";
   glossary entries per hybrid Milestone 0.
2. **Trajectory record + `trajectory_id` on runs** (additive; no behavior
   change). Mint/propagate at conductor/processor/VText spawn points.
3. **Wire publication trajectories** (Conjecture F's ledger): processor opens
   trajectories; `sourcecycled` reconciles on settlement; raise `maxProc` when
   a multi-story cycle shows zero accounting leak. This is the falsifier run.
4. **TrajectoryWorkItem cutover**: port RunContinuation machinery; route
   `request_super_execution` through work items; re-point app-adoption
   synthesis and acceptance evidence; `/api/continuations` becomes a
   compatibility shim or is renamed.
5. **Control-read removal**: budgets → per-trajectory/owner caps; cancellation
   → by-trajectory; co-super slots → slot registry; `ParentRunID` →
   `spawned_by_run_id` provenance-only; delete `CountActiveChildRuns` control
   reads.
6. **Side PR, anytime**: `platformd → corpusd` rename with config aliases.

Parallel design track (does not block 1–5): capsule layer per hybrid handoff
research backlog; MutationTransaction as generalization of AppAdoption;
`publication_id` minting moved to candidate-selection time.

---

## 6. Conjecture snapshot (v2)

### Supported / promoted
- N1 (handoff recovery) — resolved, promoted; import-verification invariant
  proposed.
- E-tool-scope — already enforced in code; record as assertion with receipts.

### Active conjectures
- **N2′ (upgraded from N2):** the trajectory model (record + work items +
  settlement) replaces every parent/child control use with no loss of
  budgets/cancellation/provenance function. *Test:* §5 steps 2–5 with the Wire
  multi-story cycle as falsifier. *Edge:* settlement-rule wrongness; slot
  semantics.
- **N3′ (absorbed into N2′):** "general continuation" disappears because work
  items structurally require artifact + authority + objective — enforcement by
  shape, not by validator.
- **N6 (new):** the word "continuation" should be retired from the ontology;
  two mechanisms currently share it and the collision already caused a
  misdiagnosis (§1.3). *Test:* docs/glossary sweep + rename in step 4; recur in
  prompt_defaults. *Edge:* renaming without re-keying would be cosmetic — the
  re-keying (§3.2) is the substance.
- **N4 (unchanged):** corpusd now; autoputer code rename with capsules.
- **N5 (unchanged):** ConjectureRecord v0 as SKILL.md template; the §5 program
  is the proof mission.

### Open questions
1. Settlement rules per trajectory kind (publication settled ≠ adoption
   settled ≠ mission settled) — define each as data, not code constants.
2. Where the trajectory record lives — likely the runtime store (it must answer
   "is this live?" on every reconcile), with sourcecycled reading via the
   existing status API.
3. `publication_id` minting point — platformd's `PublishVTextResponse` ID,
   minted earlier at candidate selection?
4. Cross-level invalidation (grand synthesis §1.3): which existing assertions
   about Wire accounting, made under the run-tree regime, get explicit
   `invalidation_triggers` when settlement lands?
5. Does anything ever genuinely need a run *tree* (vs. provenance edges +
   trajectories)? Current answer from the inventory: no control use survives
   scrutiny; revisit if a bounded-delegation case appears that trajectories
   cannot express.
