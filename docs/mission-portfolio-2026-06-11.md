# Mission Portfolio — 2026-06-11

## Status

The defined-missions backlog, now governed by Parallax (`skills/parallax/SKILL.md`)
as a portfolio-level conjecture circuit. Each entry below is a mission source
form: witness/spec, deeper goal, bridge conjecture, falsifier, edge,
settlement, and dependencies. At mission start, `/goal docs/<mission>.md`
compiles the relevant entry and references into a mutable mission document
with `Parallax State`.

This portfolio inherits [choir-doctrine.md](./choir-doctrine.md). Read it as a
heresy-reduction and conjecture-learning program, not a roadmap whose visible
product smoke can settle architecture. Each mission must report evidence class,
blocked-by relationships, detector-count target when countable, and heresy
delta split into `discovered`, `introduced`, and `repaired`.

Sources of truth these missions execute against:
`docs/archive/choir-rearchitecture-durable-actors-2026-06-11.md` (the cutover
program), `docs/choir-promotion-protocol-conjecture-2026-06-11.md`,
`docs/conjecture-learning-proof-theory-2026-06-11.md`, the four green specs
in `specs/`, and `docs/archive/mission-geometry.md` (the layer each mission serves).

Already done (not missions): specs 0–4 + CI; actor core package
(`internal/actor`, cutover step 1); promotion P2 guards (approval gate +
freshness CAS); MissionGradient v2.0.0; Parallax v1.3.1 as candidate
mission discipline.

## Portfolio Parallax State

**status:** working

**mission conjecture:** If the portfolio first lands the durable-actor spine
— trajectory/work-item records, one typed update/wake primitive, actor
lifecycle/passivation, and deletion of continuation/parent-child control —
then uses product paths only as falsifiers of that spine, Choir's deeper
rearchitecture goal advances: durable actors, evidence-bearing promotion, and
self-development become operational instead of documentary.

**deeper goal (G):** make Choir a self-improving persistent-computer system
whose agents can change code, data, docs, and method through typed
conjectures, verifier evidence, owner gates, and rollback-aware promotion.

**witness/spec (A/S):** M1-M4 are the architectural cutover spine. M5 is the
first product-path falsifier of that spine, not a product-polish mission.
M6/M8 complete the promotion and rollback substrate; M7 is the review UI on
top. M10/M11/M12 are side tracks. M9 is completed doctrine cleanup. The specs
and docs named above are the source program.

**bridge conjecture + sub-conjectures / position:** The bridge is not assumed:
each mission must test whether its artifact advances the deeper goal or merely
adds scaffolding. Repeated obstacles require revising the mission conjecture,
not spawning a disconnected replacement. Superseded missions link successors
and preserve their learning state in the mission doc.

**learning state:** retained first in mutable mission docs; promoted outward
only when it changes shared assertions, architecture, specs/tests, skills, or
successor missions.

**variant (ranking function) V:** count unsettled architecture spine and
substrate gates: M3.2 Texture prompt register and decision notes; M3 lifecycle
cutover; M4 continuation deletion; M5 durable-actor Wire falsifier; M6
route-profile consumer; M8 Dolt rollback window; M7 review UI on real
promotion substrate. Current V=7. M3.1 is settled as the emergency repair;
M3.2 is the durable prompt/decision-observability gate that keeps M3 from
reopening the same failure mode. Side missions do not decrease this V unless
they remove a heresy that blocks the spine.

**next move:** M3.2 Texture prompt register and decision notes
(`docs/archive/mission-vtext-prompt-decision-notes-m3.2-v0.md`; old v-name mission path), then M3 proper. Do
not spend owner attention on Universal Wire completeness or review UI polish
until M3.2 and M3 protect Texture delegation semantics and remove the old
lifecycle/continuation mechanisms.

**ledger file:** `docs/mission-portfolio-2026-06-11.ledger.md`.

**portfolio heresy accounting:** discovered heresies increase open inventory
but are epistemic progress. They do not count as repaired. A mission decreases
portfolio V only when its named detector count decreases, a non-countable
heresy is explicitly proven unavailable, or a successor paradoc accepts a
remaining edge without pretending it is fixed.

**surface-ontology cleanup cascade:** H027 Trace app residue, H028 raw Terminal
app residue, and H029 Browser-as-source-gathering residue are doctrine-upgrade
discoveries. They are not regressions introduced by this portfolio. Code-bearing
cleanup is deferred to successor missions: Trace evidence/Features cleanup,
Super Console/terminal compatibility cleanup, and Source Viewer/Web Lens naming
cleanup.
Historical mentions of Trace/Terminal/Browser in this portfolio are evidence
labels or successor-scope handles, not target product ontology.

**source-intake product wedge:** `docs/archive/mission-conductor-url-source-routing-h029-v0.md`
now carries the H029 Conductor URL repair plus the broader owner workflow:
URLs/files become durable source artifacts transcluded into Texture, with
YouTube transcript import as the first personal-use slice and podcasts/PDFs/
EPUBs/uploads staged after that. This is a side product wedge and future
product falsifier, not architecture-spine descent, except when it removes a concrete
Browser-as-source blocker for M3-M5.

**Texture structured-document substrate:** `docs/archive/mission-texture-structured-document-transclusion-cutover-v0.md`
now carries the planned hard cutover from markdown-ish Texture bodies plus
source/media sidecars to a ProseMirror/Tiptap-style structured document with
Texture-native source/transclusion nodes and multimedia source entities. This
is a Texture substrate mission, not a change to the current architecture-spine
variant, except when it removes a concrete blocker for M3-M5 or successor
missions.

## Dependency graph

```
M1 trajectory model ──► M2 messaging cutover ──► M3.1 lifecycle recovery ──► M3.2 Texture prompt/decision notes ──► M3 lifecycle cutover ──► M4 continuation deletion ──► M5 wire on settlement
                                                                                         (M5 = route-switch evidence gate)

M4 continuation deletion ──► M6 route-flip consumer ──► M8 dolt branching + rollback window ──► M7 changes-app review loop

M9 docs revision ─ independent, do early
M10 capsule design ─ independent research, parallel anytime
M11 corpusd rename ─ independent side PR, anytime
M12 dead-export sweep ─ independent, coordinate with M4/M7
```

Recommended order of *execution* after the 2026-06-12 sequencing correction,
the 2026-06-14 M3.1 recovery split, and the M3.2 prompt/decision-notes gate:
M9 → M1 (proof mission) → M2 → M3.1 → M3.2 → M3 → M4 → M5 → M6 → M8 → M7,
with M10/M11/M12 parallel only when they remove architectural ambiguity rather
than distract from the spine. Earlier
text treated M5 as runnable immediately after M1 because settlement accounting
can be modeled before the messaging/lifecycle cutover. That remains true for
substrate work, but not for the product gate: do not spend owner attention on
whether Universal Wire is empty or complete until durable actors are working
and the old continuation/parent-child code has been removed. M5 remains the
route-switch evidence gate, now after M2-M4. M7 moves after M8 because owner
review without a real activate/rollback substrate is another product mirage.

## Architecture-first revision — 2026-06-12

This portfolio now treats product surfaces as **falsifiers after substrate**,
not as the substrate. The automatic newspaper and review UI are valuable only
when they reveal whether durable actors, work items, promotion, and rollback
actually hold. Until then, product success is low-information and product
failure is ambiguous.

Operating rules for the remaining missions:

- Numbered core missions are dependency order unless explicitly marked as a
  side track. M9 ran early because stale doctrine is a heresy vector; it was
  preflight cleanup, not an argument for arbitrary ordering.
- Old-code deletion is settlement, not cleanup. A mission that leaves a
  permanent dual model must carry the surviving old path as open variant.
- Product-path acceptance can falsify architecture, but it cannot substitute
  for deletion of old coordination, lifecycle, continuation, promotion, or
  rollback mechanisms.
- Side missions may run early only when they remove a blocker to the spine
  or are cheap independent research; they should not consume owner attention
  needed by M2-M8 gates.
- Every paradoc should name whether it is `spine`, `falsifier`, `promotion
  substrate`, `review surface`, or `side track`, so future agents do not
  confuse visible product work with architectural descent.

---

## M1a — Continuation synthesis deletion (pre-M1 side PR) — DONE 2026-06-12

**Real artifact:** the synthesis *decision* layer deleted ahead of M1, on the
principle that a deletion costing nothing proven and requiring no replacement
should not wait for its cutover step: retired `SynthesizeRunContinuation`,
retired `SelectSynthesizedRunContinuation`, the app-adoption→objective mapping with
its adoption-ID substring match, the hardcoded mission-doc fallback, the
synthesis legacy retired lease defaults, and the synonym folding inside the fingerprint
normalizer. retired `POST /api/continuations` now requires an explicit objective (the
caller decides; the record layer only records). The record layer —
retired `SelectRunContinuation`, `StartRunContinuation`, fingerprint dedup,
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
budgets, fingerprint dedup — no retired lease vocabulary in v1), in the runtime store. Additive; no control-flow change.

**Bridge conjecture (N2′):** one durable object — trajectory + work items —
replaces every parent/child control use with no loss of budget, cancellation,
or provenance function. *Falsifier:* a control use surfaced in the review-v2
inventory that trajectories cannot express. *Edge (frame_lock):* settlement
rules per trajectory kind may be wrong in ways the current vocabulary cannot
state; define each rule as data and review after first real cycle.

**Settlement:** records exist, are minted at conductor/processor/Texture spawn
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

## M2 — Messaging cutover (cutover step 3) — DONE 2026-06-13

**Kind:** spine.

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
restart; prompts updated (co-super.md, vsuper.md, vtext.md; old v-name prompt path). Settled by
`docs/archive/mission-messaging-cutover-v0.md` after post-review repairs and staging
landing at `794d28dd76ff00a2ae27c98a14dbce9e34834695`.

**Dependencies:** M1. **Size:** 1–2 overnight missions; the slot registry is
the riskiest single migration — its own control interval and test.

## M3 — Lifecycle cutover (cutover step 4)

**Kind:** spine.

**Heresies / evidence:** repairs H001-H005, H015-H016, and H025. Evidence class:
architectural-level locally plus staging proof for any vmctl/product-path claim.
Countable target: no new parent/child control reads, no `spawned_child_*` work
item semantics, and dead parent/child result-channel APIs deleted or explicitly
quarantined.

**Real artifact:** `executeRun` goroutine closures replaced by actor
activation loops; `recoverInterruptedRuns` blanket-fail deleted (boot = cold
actors + sweep); cancel-by-trajectory replaces `CancelRunGraph`;
retired `ParentRunID` → `spawned_by_run_id` provenance-only.

**Bridge conjecture (R1/R2):** activation/passivation/sweep semantics,
already proven at the protocol level (actor_protocol.tla) and package level
(internal/actor tests), survive contact with the real LLM loop. *Falsifier:*
kill -9 mid-activation under multi-agent load; on restart, sends reactivate
with correct memory and zero stranded messages. *Edge (resource):* the LLM
loop's streaming/tool machinery may resist the clean turn boundary; budget
for a shim layer rather than distorting the actor semantics.

**Settlement:** restart amnesia gone (the falsifier passes); ~50 retired ParentRunID
test sites migrated with their features; acceptance evidence re-pointed.

**Dependencies / blocked by:** M2; M3.1 has settled the immediate
Texture/prompt forcing regressions H009-H012/H024/H026; M3.2 must now land the
off-document Texture decision channel and reason-bearing prompt register so
lifecycle proof is not defined by role choreography or polluted canonical
documents. **Size:** 2 overnight missions; the big one.

## M4 — Continuation deletion (cutover step 5)

**Kind:** spine.

**Real artifact:** the residual retired RunContinuation record/API/event surface is
removed or explicitly shimmed to trajectory work items; app-adoption
progression is event-driven (adoption state change -> update to the owning
actor's mailbox); acceptance-evidence "continuation-level" is re-pointed at
work items; retired `/api/continuations` returns 410 or a compatibility response that
names the replacement. M1a already deleted the synthesis decision layer
(retired `SynthesizeRunContinuation`, retired `SelectSynthesizedRunContinuation`, hardcoded
mission fallback, adoption-ID substring policy, legacy retired lease defaults). M4 finishes
the ontology cut: no remaining product or verifier path depends on
retired RunContinuation as the way work continues.

**Heresies / evidence:** repairs H006-H008, H014, and the continuation/progress
portion of H022. Evidence class: architectural-level plus staging product proof
for any public API or run-acceptance claim. Countable target: continuation API
deleted or 410 shimmed, `continuation-level` retired or explicitly transitional
only during cutover,
and no acceptance record uses continuation events as architecture proof.

**Bridge conjecture (R3):** nothing of proven value is lost — every behavior
the synthesis layer provided is unproven (autonomous self-development) or
better expressed as events + work items. *Falsifier:* one app-adoption flow
end-to-end (propose → verify → approve → promote/rollback) with no
retired SynthesizeRunContinuation in the binary. *Edge (independence):* quiet
dependencies in trace evidence projections and acceptance records; sweep them
in the same mission or verifier discipline silently weakens.

**Dependencies:** M3 (work items must exist as the replacement first — M1).
**Size:** 1 overnight mission.

## M5 — Wire on settlement (cutover step 6; the route-switch evidence gate)

**Kind:** falsifier.

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
front page honest and full enough to expose accounting errors, settlement
queryable. This is an architecture falsifier, not a newsroom polish pass:
if Universal Wire is empty or ugly, record only the architectural predicate it
does or does not reveal. **This run is the evidence gate for calling the
durable-actor core claim supported.**

**Dependencies:** M4 for the production evidence gate. Substrate work can
consume M1 earlier, but the product-facing Universal Wire proof should wait
until M2-M4 have made durable actors operational and removed the old
coordination/continuation paths. **Size:** 1 overnight mission + 1 observed
production cycle.

**Heresies / evidence:** blocked by substrate ambiguity from H001-H008/H014.
Evidence class: falsifier/product-path evidence only after architectural
detectors have decreased. Product smoke or an attractive front page does not
settle M5 unless settlement accounting is the thing being proven.

## M6 — Route-flip consumer (promotion P1's load-bearing unknown)

**Kind:** promotion substrate.

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

**Heresies / evidence:** promotion/route semantics mission. Evidence class:
promotion-level only with owner approval, served behavior change, rollback, and
freshness evidence. Blocked by M4; may discover route-profile heresies without
claiming repair until the route consumer works.

## M7 — Changes app review loop

**Kind:** review surface.

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

**Dependencies:** M6 and M8 for settlement. Small preview/plan-view probes
can be sketched earlier, but the mission should not settle before activate,
rollback, and shared-state conflict semantics are real. **Size:** 1 overnight
mission (UI) after a half-day design pass.

**Heresies / evidence:** includes H027 cleanup in Features: replace `Open Trace`
and Trace UI copy with trace-evidence/run-acceptance/Super-Console-oriented
affordances. Evidence class: review-surface smoke can prove UI opens; it cannot
settle promotion architecture.

## M8 — Shared-state promotion: Dolt branching + rollback window (P3/P4)

**Kind:** promotion substrate.

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

**Heresies / evidence:** evidence class is promotion-level only when conflicting
foreground/candidate data is preserved, surfaced, or blocked with rollback
state. Product smoke does not settle merge semantics.

## M9 — Docs revision + heresy sweep (grand synthesis §6.1) — DONE 2026-06-11

**Kind:** side track / doctrine preflight.

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

**Kind:** side track / research.

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

**Kind:** side track / heresy sweep.

**Real artifact:** the codex ruins accounted for: an export-level sweep of
frontend JS/TS (dead exports inside live files — the `synthesizeContinuation`
class; tooling like `knip` or per-export grep), a Go API-route sweep
(endpoints with no remaining frontend or agent callers — candidate: parts of
`/api/trace/*` after the retired Trace app was unshipped in 95196069), and a verdict
on the 16 remaining pre-rewrite `.js` files (~2,670 lines: js→ts stragglers
from the TS migration unit — `stores/desktop.js` 688, `vtext.js` 555 (old v-name file path),
`auth.js` 384, …). File-level reachability already verified clean 2026-06-12
(zero orphaned files/components after `trace.js` deletion); this mission is
the export/endpoint level that scan cannot see.

**Bridge conjecture:** same as M9 — dead surface is a heresy vector
(stability, security, dev velocity), not housekeeping. *Falsifier (cheap):*
the sweep tooling reports zero dead exports/endpoints — then the conjecture
that ruins remain is refuted and the mission settles immediately.
**Dependencies:** none; M4/M7 retire some Trace/continuation surface anyway —
coordinate to avoid double deletion. **Size:** half a session, agent-heavy.

**Heresies / evidence:** includes code-bearing H027-H029 detector cleanup where
safe: Trace app residue, raw Terminal app residue, Browser-as-source-gathering
residue, plus dead continuation endpoints. Evidence class: detector/export-level
only unless runtime behavior changes, in which case use the relevant protected
surface proof.

## M11 — corpusd rename (side PR)

**Kind:** side track / naming.

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
- **Doc truth / context packet CI** —
  `docs/archive/mission-doc-truth-drift-ci-context-packet-v0.md` is the successor for
  executable heresy detectors, docs drift checks, and a generated
  `docs/choir-context-packet.md`. It is a process side mission, not
  architecture-spine descent, unless stale docs block M3-M5 execution.

## Portfolio Settlement Conjecture

*Claim:* this portfolio, executed with Parallax mission documents, converts
the rearchitecture from documents into a system whose causality, messaging,
lifecycle, continuation deletion, promotion, and rollback are each backed by
a machine-checked spec where applicable, scoped runtime conformance, deletion
of the old competing code paths, and product-path falsifiers only after the
architecture can carry their meaning.

*Bridge edge:* the portfolio can still succeed locally while failing the
deeper goal if missions ship isolated scaffolding, silently abandon partial
learning, leave old control paths alive beside the new model, or replace hard
conjectures with new mission names. Each mission therefore preserves lineage,
retained learning state, and successor links when it is blocked or
superseded. Each spine mission must also state the old-code deletion ledger:
which mechanisms are gone, which remain as temporary shims, and why.

*Resource edge:* this remains ~10–14 overnight missions. The bound is owner
attention at the gates (M5's evidence gate, M6's route-consumer decision,
M8's data-conflict/rollback policy, M7's dogfood), not agent capacity. Owner
attention should be reserved for architectural gates and falsifiers; product
polish waits until the bones stand up.

*Scope:* asserted only for the missions as defined; each mission re-scopes at
start inside its own Parallax State.
