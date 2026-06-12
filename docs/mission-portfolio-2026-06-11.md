# Mission Portfolio — 2026-06-11

## Status

The defined-missions backlog, now governed by Parallax (`skills/parallax/SKILL.md`)
as a portfolio-level conjecture circuit. Each entry below is a mission source
form: witness/spec, deeper goal, bridge conjecture, falsifier, edge,
settlement, and dependencies. At mission start, `/goal docs/<mission>.md`
compiles the relevant entry and references into a mutable mission document
with `Parallax State`.

Sources of truth these missions execute against:
`docs/choir-rearchitecture-durable-actors-2026-06-11.md` (the cutover
program), `docs/choir-promotion-protocol-conjecture-2026-06-11.md`,
`docs/conjecture-learning-proof-theory-2026-06-11.md`, the four green specs
in `specs/`, and `docs/mission-geometry.md` (the layer each mission serves).

Already done (not missions): specs 0–4 + CI; actor core package
(`internal/actor`, cutover step 1); promotion P2 guards (approval gate +
freshness CAS); MissionGradient v2.0.0; Parallax v1.2.0 as candidate
mission discipline.

## Portfolio Parallax State

**status:** working

**mission conjecture:** If the portfolio lands the trajectory/work-item
model, settlement-gated Wire, single-message actor cutover, real promotion
route flip, review UI, Dolt rollback window, and capsule/corpus side tracks
under the named invariants and staging proof requirements, then Choir's
deeper rearchitecture goal advances: durable actors, evidence-bearing
promotion, and self-development become operational instead of documentary.

**deeper goal (G):** make Choir a self-improving persistent-computer system
whose agents can change code, data, docs, and method through typed
conjectures, verifier evidence, owner gates, and rollback-aware promotion.

**witness/spec (A/S):** M1–M8 are the core witness path; M10/M11 are side
tracks. M9 is completed doctrine cleanup. The specs and docs named above are
the source program.

**bridge conjecture + sub-conjectures / position:** The bridge is not assumed:
each mission must test whether its artifact advances the deeper goal or merely
adds scaffolding. Repeated obstacles require revising the mission conjecture,
not spawning a disconnected replacement. Superseded missions link successors
and preserve their learning state in the mission doc.

**learning state:** retained first in mutable mission docs; promoted outward
only when it changes shared assertions, architecture, specs/tests, skills, or
successor missions.

## Dependency graph

```
M1 trajectory model ──► M2 messaging cutover ──► M3 lifecycle cutover ──► M4 continuation deletion
        │                                                                        │
        └────────────► M5 wire on settlement ◄──────────────────────────────────┘   (M5 = route-switch evidence gate)

M6 route-flip consumer ──► M7 changes-app review loop ──► M8 dolt branching + rollback window

M9 docs revision ─ independent, do early
M10 capsule design ─ independent research, parallel anytime
M11 corpusd rename ─ independent side PR, anytime
```

Recommended order of *execution*: M9 → M1 (proof mission) → M5 → M2 → M6+M7
→ M3 → M4 → M8, with M10/M11 parallel. M1 and M5 are the highest-information
pair: together they test whether the trajectory model actually retires the
leaked invariant on real production traffic.

---

## M1a — Continuation synthesis deletion (pre-M1 side PR) — DONE 2026-06-12

**Real artifact:** the synthesis *decision* layer deleted ahead of M1, on the
principle that a deletion costing nothing proven and requiring no replacement
should not wait for its cutover step: `SynthesizeRunContinuation`,
`SelectSynthesizedRunContinuation`, the app-adoption→objective mapping with
its adoption-ID substring match, the hardcoded mission-doc fallback, the
synthesis lease defaults, and the synonym folding inside the fingerprint
normalizer. `POST /api/continuations` now requires an explicit objective (the
caller decides; the record layer only records). The record layer —
`SelectRunContinuation`, `StartRunContinuation`, fingerprint dedup,
compaction-before-handoff — survives until M1 ports it to work items and M4
retires it.

**Rationale receipt:** the only production caller of synthesis was the API
endpoint (api.go:605), reached from a dead frontend export and the agent
product-API allowlist; the autonomous path (`maybeStartConfiguredContinuation`)
always used explicit metadata objectives. Loss = autonomous objective
selection, already accepted as unproven (rearchitecture doc §2.5 named risk).
Suite green (runtime, actor, store; comprehensive continuation/API tests).

## M1 — Trajectory model (cutover step 2; also the §15 proof mission)

**Real artifact:** durable trajectory records (kind, subject refs, status,
settlement rule as data) + `trajectory_id` on runs + work items (the ported
continuation mechanics: objective, bounded authority profile, step/token
budgets, fingerprint dedup — no lease vocabulary in v1), in the runtime store. Additive; no control-flow change.

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

**Parallax overlay:** run this mission from a mutable mission document whose
bridge conjecture is: if trajectory/work-item records are added with no
behavior change, then the deeper rearchitecture goal advances by making
parent/child replacement and settlement accounting executable. Success
criteria: at least one SHIFT changed the route; the bridge was tested rather
than assumed; repeated obstacles updated CLAIM/TEST/EDGE/ΔO/SCOPE instead of
spawning a disconnected mission; the mission doc retained learning state and
was cheaper to resume/retrospect than a MissionGradient doc; no canonical
mutation without gates. Failure criterion: circuit fields filled while moves
stay identical to old MissionGradient behavior. Report honestly — this
adjudicates Parallax's promotion (see docs/parallax-design-2026-06-11.md §5).

**Size:** 1 overnight mission.

## M2 — Messaging cutover (cutover step 3)

**Real artifact:** `update_coagent` (renamed, promoted `submit_coagent_update`)
as the sole agent-to-agent primitive over the `internal/actor` send path;
deletion of `cast_agent`, `cast_agent_update`, `wait_agent`, `notifyParent`,
per-turn inbox polling; co-super slot registry keyed (trajectory, slot) with
atomic claim.

**Bridge conjecture (R4):** one structured message primitive doubling as the
wake primitive makes results single-sourced and control flow legible.
*Falsifier:* a real coordination need that typed kinds + notes cannot express
(watch the kind distribution for prose-stuffing). *Edge (missing_oracle):*
silent stall — liveness now rests on the open-obligations query; a trajectory
that stops progressing while showing zero open obligations kills the design.

**Settlement:** grep-level zero callers of the deleted mechanisms; a vsuper
coordinating two co-supers sees every result exactly once across a process
restart; prompts updated (co-super.md, vsuper.md, vtext.md).

**Dependencies:** M1. **Size:** 1–2 overnight missions; the slot registry is
the riskiest single migration — its own control interval and test.

## M3 — Lifecycle cutover (cutover step 4)

**Real artifact:** `executeRun` goroutine closures replaced by actor
activation loops; `recoverInterruptedRuns` blanket-fail deleted (boot = cold
actors + sweep); cancel-by-trajectory replaces `CancelRunGraph`;
`ParentRunID` → `spawned_by_run_id` provenance-only.

**Bridge conjecture (R1/R2):** activation/passivation/sweep semantics,
already proven at the protocol level (actor_protocol.tla) and package level
(internal/actor tests), survive contact with the real LLM loop. *Falsifier:*
kill -9 mid-activation under multi-agent load; on restart, sends reactivate
with correct memory and zero stranded messages. *Edge (resource):* the LLM
loop's streaming/tool machinery may resist the clean turn boundary; budget
for a shim layer rather than distorting the actor semantics.

**Settlement:** restart amnesia gone (the falsifier passes); ~50 ParentRunID
test sites migrated with their features; acceptance evidence re-pointed.

**Dependencies:** M2. **Size:** 2 overnight missions; the big one.

## M4 — Continuation deletion (cutover step 5)

**Real artifact:** `SynthesizeRunContinuation` and the decision layer
removed; app-adoption progression event-driven (adoption state change → 
update to the owning actor's mailbox); acceptance-evidence
"continuation-level" re-pointed at work items; `/api/continuations` shimmed
or 410.

**Bridge conjecture (R3):** nothing of proven value is lost — every behavior
the synthesis layer provided is unproven (autonomous self-development) or
better expressed as events + work items. *Falsifier:* one app-adoption flow
end-to-end (propose → verify → approve → promote/rollback) with no
SynthesizeRunContinuation in the binary. *Edge (independence):* quiet
dependencies in Trace UI and acceptance records; sweep them in the same
mission or verifier discipline silently weakens.

**Dependencies:** M3 (work items must exist as the replacement first — M1).
**Size:** 1 overnight mission.

## M5 — Wire on settlement (cutover step 6; the route-switch evidence gate)

**Real artifact:** `sourcecycled` reconciles on trajectory settlement instead
of `isTerminalRuntimeState && ActiveChildRuns == 0` (main.go:590); processor
opens publication trajectories carrying coverage/publish decisions; `maxProc`
raised above 1.

**Bridge conjecture:** the wire_pipeline.tla result transfers — with durable
decisions and settlement accounting, parallel processors publish with zero
accounting leaks, retiring the serialization stopgap on evidence rather than
hope. *Falsifier:* a multi-story cycle at maxProc > 1 with a publication
accounting leak, or a trajectory that settles while coagent work is still
mutating its artifact (the settlement-rule edge from N2′).

**Settlement:** one real multi-story production cycle, parallel processors,
front page honest and full, settlement queryable. **This run is the evidence
gate for calling the rearchitecture's core claim supported.**

**Dependencies:** M1 (can run before M2–M4 — settlement accounting does not
require the messaging cutover). **Size:** 1 overnight mission + 1 observed
production cycle.

## M6 — Route-flip consumer (promotion P1's load-bearing unknown)

**Real artifact:** something real consumes `RouteProfile`: vmctl/proxy route
resolution honors the lineage's route pointer, so PromoteAppAdoption's flip
observably changes what the running computer serves; a durable promotion
record + reconciler finishes interrupted promotions from the commit point
alone (promotion_protocol.tla shape).

**Bridge conjecture (P1):** the single-commit-point protocol is
implementable as a thin layer over existing AppAdoption now, without waiting
for capsules. *Falsifier:* kill the coordinator mid-promotion; a reconciler
completes it from the promotion record alone; "Activate" demonstrably changes
served behavior. *Edge (missing_oracle):* nothing reads RouteProfile today —
the consumer's natural home (vmctl route table vs proxy resolution vs both)
is undetermined; the mission's first control interval is answering that, and
the founder may need to arbitrate (escalation point, not silent choice).

**Settlement:** one end-to-end promotion on a real computer where activate →
new behavior served → rollback → old behavior served, all through the
product path. **Size:** 1–2 overnight missions.

## M7 — Changes app review loop

**Real artifact:** `FeaturesApp.svelte` upgraded to the S1–S5 review loop:
headline; **Try-it-now wired to the existing preview endpoint** (cheapest
high-value fix in the system); plan view with destructive items flagged and
rollback-window status; plain-language check badges gating Approve; Approve
visibly the S5 signature; restore-point timeline for rollback.
`platform-os-app-state.md` reconciled with what actually ships.

**Bridge conjecture:** approval becomes an *informed* discharge of the
intent obligation — preview-as-review beats diff-as-review for non-developers.
*Falsifier:* dogfood — the owner reviews a real change end-to-end without
reading a diff or a hash. *Edge (frame_lock):* the headline/plan author has a
conflict of interest if it is the authoring agent; use an independent
summarizer or deterministic plan-diff, and say which in the plan view.

**Dependencies:** M6 for the activate-means-something half; the preview
wiring and plan view need nothing and can start immediately. **Size:** 1
overnight mission (UI) after a half-day design pass.

## M8 — Shared-state promotion: Dolt branching + rollback window (P3/P4)

**Real artifact:** promotions create real Dolt branches at fork, three-way
merge at commit with legible conflict surfacing; rollback window as explicit
durable state, closed by the first N-1-incompatible write, gating both
AutoRevert and the user's Rollback button; contract-phase changes structurally
forced into separate later promotions.

**Bridge conjecture (P3/P4):** versioned, mergeable user data converts the
worst promotion risk class from blue-green prayer into surfaced conflicts;
the rollback window as state prevents the torn-rollback class. *Falsifier:*
a candidate migrating a table while the foreground writes rows — commit
either merges cleanly or blocks with a legible conflict set; never silently
drops either side. *Edge:* merge resolution *policy* (active-wins vs
candidate-wins per data class) is an owner decision, not a default —
escalation point.

**Dependencies:** M6. **Size:** 2 overnight missions.

## M9 — Docs revision + heresy sweep (grand synthesis §6.1) — DONE 2026-06-11

**Completed**: see `docs/mission-docs-revision-v1.md` for the run record and
evidence ledger.

**Real artifact:** canonical docs match the post-cutover ontology: ontology
chapter (actor, mailbox, activation, passivation, rewarm, trajectory, work
item, settlement; "continuation" retired; "channel" disambiguated);
"sandbox"-in-product-contexts sweep; Conjecture E's tool-scope half recorded
as a receipted assertion; the three-level self-improvement table promoted;
overclaim audit of UI/doc language ("verified" never rendered as "safe");
glossary entries per hybrid handoff Milestone 0.

**Bridge conjecture:** docs are heresy vectors — stale assertions regenerate
bad behavior in every agent that reads them; the sweep is consistency
maintenance, not housekeeping. *Falsifier (cheap):* grep-class checks per
heresy named in the review docs. **No dependencies — do this first or in
parallel with M1.** **Size:** 1 session, agent-heavy.

## M10 — Capsule substrate design (research mission, not code)

**Real artifact:** a design doc + decision record answering the hybrid
handoff research backlog: Nucleus maturity audit vs bubblewrap/nsjail/gVisor;
effect-capture mechanism (overlay diff vs fanotify vs eBPF vs seccomp trace);
VM filesystem strategy (Btrfs vs qcow2 overlays vs Firecracker snapshots);
secret/capability delegation into capsules; CapsuleSpec/CapsuleResult
integration with Trace and promotion certificates.

**Bridge conjecture:** Nucleus strict-agent fits the capsule role
(hybrid handoff's claim, currently supported only by README reading —
independence-class edge: no hands-on evidence). *Falsifier:* a hands-on spike
running a real parser job in strict-agent mode with effect capture.

**Dependencies:** none (design track). **Size:** 1 research mission;
unblocks the curl|bash story and hybrid Milestones 1–3.

## M12 — Dead-export and dead-endpoint sweep (side mission, M9-class)

**Real artifact:** the codex ruins accounted for: an export-level sweep of
frontend JS/TS (dead exports inside live files — the `synthesizeContinuation`
class; tooling like `knip` or per-export grep), a Go API-route sweep
(endpoints with no remaining frontend or agent callers — candidate: parts of
`/api/trace/*` after the Trace app was unshipped in 95196069), and a verdict
on the 16 remaining pre-rewrite `.js` files (~2,670 lines: js→ts stragglers
from the TS migration unit — `stores/desktop.js` 688, `vtext.js` 555,
`auth.js` 384, …). File-level reachability already verified clean 2026-06-12
(zero orphaned files/components after `trace.js` deletion); this mission is
the export/endpoint level that scan cannot see.

**Bridge conjecture:** same as M9 — dead surface is a heresy vector
(stability, security, dev velocity), not housekeeping. *Falsifier (cheap):*
the sweep tooling reports zero dead exports/endpoints — then the conjecture
that ruins remain is refuted and the mission settles immediately.
**Dependencies:** none; M4/M7 retire some Trace/continuation surface anyway —
coordinate to avoid double deletion. **Size:** half a session, agent-heavy.

## M11 — corpusd rename (side PR)

`platformd → corpusd` with config-key aliases; zero behavior change;
SMALL-MEDIUM radius per review v2. Promote "canonical publication_id minted
at candidate selection" from open question to a design note inside this
mission. **Dependencies:** none. **Size:** half a session.

---

## Deferred, explicitly (not missions yet)

- **Trace validation** (production traces replayed against the TLA+ specs) —
  after M3, when the protocol surface is real.
- **xVM outbox implementation** — spec exists (actor_protocol_xvm.tla);
  build when the first real cross-VM pair (super↔vsuper under candidate
  computers) is live, likely alongside M10's outputs.
- **Leases as QoS/pricing tiers** — deferred until service-tier requirements
  exist; eviction safety is already proven and implemented.
- **Slides/computational cinematography, vector index service, new source
  families** — per the conjecture program §5; substrate first.

## Portfolio Settlement Conjecture

*Claim:* this portfolio, executed with Parallax mission documents, converts
the rearchitecture from documents into a system whose causality, messaging,
lifecycle, and promotion are each backed by a machine-checked spec, scoped
runtime conformance, and at least one production falsifier run.

*Bridge edge:* the portfolio can still succeed locally while failing the
deeper goal if missions ship isolated scaffolding, silently abandon partial
learning, or replace hard conjectures with new mission names. Each mission
therefore preserves lineage, retained learning state, and successor links
when it is blocked or superseded.

*Resource edge:* this remains ~10–14 overnight missions. The bound is owner
attention at the gates (M5's evidence gate, M6's escalation, M7's dogfood,
M8's policy decision), not agent capacity.

*Scope:* asserted only for the missions as defined; each mission re-scopes at
start inside its own Parallax State.
