# The Mission Stack: From Here to World Wire Live

Date: 2026-09-22 (revised same-day after owner review)
Status: **proposed roadmap — the R-series spine ratified 2026-09-25**
(desk-rlm-rectification-plan §11 owner-ratified); the M9+/world-wire tail
stays under owner review. Supersedes the
[`reports/choir-rlm-missions-overview-2026-09-09.md`](reports/choir-rlm-missions-overview-2026-09-09.md).
Mission state lives in `ACTIVE.md` + `mission-graph.yaml`; this document
supplies the corrected scope and ordering. The owner's situation brief is
[`current-situation-2026-09-22.md`](current-situation-2026-09-22.md).
Format: every mission is authored as a **throughline** `/goal` file
(`skills/throughline/SKILL.md`).
Provenance: synthesized from a 13-agent divergent consensus panel
(`.agentic-consensus/agentic-consensus-20260922-202933/`), then revised against
owner feedback: the box score is out of scope (it follows weeks of production
wire), and the pre-wire phase was under-scoped — more RLM and self-development
work remains than the first draft showed.

## The correction in one paragraph

The 11-mission stack was a **harness program**: it built a trustworthy event
log and stopped at a goal selector, with no users, no wire, no record. The
corrected stack treats the commitment ledger as what the substrate is *for* —
**precommitment records are the product mechanism**, and the World Wire is
that ledger made public. But the path there is longer than a substrate→
mechanism→product compression suggests: the RLM program is unfinished, the
self-development gate is a program not a single proof, and the wire does not
start until the computer genuinely develops itself.

## The load-bearing insight (panel consensus)

**An unresolved precommitment is a durable wake obligation — the same object
as the carrier's stranded-continuation bug.** A record that commits a
prediction must be resolved later; that resolution is a continuation. The
carrier's "activation-wake family" defect is the same substrate gap the
records mechanism needs closed. This reframes the carrier's owner decision:
the wake repair is the prototype for open commitments and for the wire's
impact-propagation.

## What the old stack got wrong (panel consensus)

- **It ended at machinery.** "Native goals" was the endpoint; production
  publication, API, real users, repeat use were absent.
- **Shadow evals (old M9) were mislocated.** Precommitment records make every
  production run a scored episode — deployment logs *are* the evaluation.
- **It treated precommitment records as a side track.** The mechanism lands
  before the self-development proof, so the proof generates scored records.
- **No docs-hygiene track.** Retrieval pollution corrupts every mission's
  `start` receipt.

## What the first draft of THIS stack got wrong (owner review)

- **Box score was in scope.** It is not a pre-wire mission; it follows weeks
  of production wire accumulating resolutions. Removed.
- **The pre-wire phase was compressed.** M1–M3 implied the substrate was
  nearly done. It is not: the carrier is blocked, the ontology cutover is
  unratified, the remaining desk crossings are real work, and the
  self-development gate is a program, not one proof.
- **Uncertain steps were presented as settled.** The ontology cutover, the
  desk-crossing strategy, and the depth of the self-development program are
  open questions, not decided sequence.

## The stack (revised)

Phases, not a flat list — the phases are ordered, the missions inside a phase
may overlap. Every mission is a throughline goal file.

```text
Phase 0  Docs overhaul (landed, green, not a dependency)
Phase 1  Finish the RLM substrate
Phase 2  Precommitment records: the mechanism
Phase 3  Self-development, proven for real
Phase 4  Production hardening + first users
Phase 5  The World Wire
(box score: after the wire runs in production — out of this stack)
```

### Phase 0 — Docs overhaul (landed)

Phase A–D of `reorientation-docs-overhaul-2026-09-22.md`, plus a second
consistency sweep. Landed 2026-09-22: zombie authority killed, ~14 superseded
definitions retired, PICL renamed, manifest/graph/ACTIVE repaired, throughline
settled as the format, the authority layer re-aligned (doctrine, product-
doctrine, ontology, standing-questions), remaining zombie `next_action`s and
deleted-file refs across definitions tombstoned, and ~30 dated
reports/reviews/designs bannered as stale evidence. Remaining: deeper
reorientation of `computer-ontology.md` internals and the 2026-07-24
architecture memos (banner-marked stale), HTML-deck re-render, legal-doc
refresh, and ~20 dated problem receipts needing closure banners. Not a gate,
but every mission is cheaper with honest docs.

### Phase 1 — Finish the RLM substrate

The carrier program is mid-flight and blocked. This phase completes it.

- **1a — The carrier decision (owner).** Re-scoped 2026-09-22: the roster was
  run on a makeshift `texture tell` driver (`cmd/choir/roster.go`), not the
  in-cell carrier, and `tell` itself is a bug — Texture is document-driven.
  What remains is only the evidence floor — and owner correction 2026-09-22
  sharpens it: roster isn't a concept, it's a sub-RLM call. The capability is
  a sub-RLM cast on the document channel driving a task; the roster driver is
  deleted, not migrated. The substrate half (the wake defect) is carried by 1d.
- **1b — Wake authority: minted continuations** *(folded into 1d)*. The
  transition-minted recovery-occurrence repair is subsumed by the ontology
  cutover's derivable-continuation model — pending deliveries are a
  projection, the dispatcher is the one consumer. Not a separate mission.
- **1c — Carrier landing.** Prove the sub-RLM call on the real harness (a
  cast on the document channel, not `texture tell`), then P6: engineering desk
  fully on the in-cell carrier, five overlay tools + four legacy capsule ops deleted (R7),
  run acceptance on canonical evidence, R9 provider-heresy deletion. Proves:
  a desk lives entirely on the carrier — the desk the `precommit` module
  ships through.
- **1d — Ontology cutover** *(ratified 2026-09-22)*. Implement the minimal
  event-driven design: delivered = in state head, fenced atomic commit,
  serial-per-actor, cast-only sub-RLMs, migration inside a write fence;
  management and research cross; `actuator=tools` deleted (R8), the legacy
  management substrate retired (R10), and the `tell`/`correct`/`LifecycleOwnerInstruction`
  out-of-band path plus `cmd/choir/roster.go` deleted — owner input becomes a
  document edit event. Proves: delivery and continuation are *derivable* —
  the wake family's structural death. Scope: deletes the wrong-path cluster
  enumerated in `docs/problems/root-cause-wrong-path-cluster-2026-09-22.md`
  (~44 instances across out-of-band input, process-local continuations, dual
  paths, sweep recovery, non-event mutations). Owner-corrected 2026-09-22:
  `tell` is a hallucination (a revision is just a diff — no payload to
  preserve) and roster isn't a concept (it's a sub-RLM call — delete, not
  migrate); `actuator=tools`/management stay blocked on desk crossings;
  `install_frontend_pointer` is the platform-shell deploy contract (separate
  migration); vmctl/sourcecycled deferred (post-RLM reengineer / unknown).
- **1e — Remaining desk crossings** *(migration targets of 1d)*. Texture,
  research, and management desks cross to the carrier as the cutover's
  migration targets — the work the old stack serialized as M4–M6.


### Phase 2 — Precommitment records: the mechanism

The product capability. Lands once the substrate can hold it (a carrier that
doesn't strand pending states).

- **2a — The record type + ledger.** The eight schema decisions: `expected`
  linkage + `resolves`/`prediction_ref`, epistemic-boundary invariant,
  deterministic pairing projection onto the OG, async Curator desk minting
  `learning_record_minted`, result-wake policy, REPL-queryable records with
  auto-inject OFF, actor/model scoping. The record is an OG object with
  provenance edges — never a third store. The Markdown conjecture ledger +
  doccheck regex retire.
- **2b — The agent-facing surface.** The `precommit` Yaegi module
  (`Commit`/`Resolve` via `ChoirExports`), the `Scorer` interface (self-score
  default; second-LLM and Jev as config), typed question sets, specificity
  scoring. Proves: commit → observe → score → revise → persist → retrieve
  runs end-to-end on the real tape; agents cannot edit committed records.
- **2c — Context packs + the learning-claims gate.** Aggregate records per
  model+task; build packs by corrective value; run the sealed pre-state
  replay control (does temporal commitment beat outcome-blind replay?); flip
  retrieval ON only if the evidence supports it. Proves: **behavior change** —
  the only admissible evidence of learning — or falsifies the conjecture
  cheaply while records still pay as audit.

### Phase 3 — Self-development: the acceleration axis

Owner direction 2026-09-22: self-development is the priority because it
compounds — the more Choir drives its own development, the faster everything
else lands, and it cuts the CI pipeline out of the UX-improvement loop.
Current state: email works, Texture is broken (the `tell`/roster input path —
see `docs/problems/root-cause-wrong-path-cluster-2026-09-22.md`). Three
capability phases, then the proof:

- **3a — Capability phase 1: `choir` CLI from a harness.** An external harness
  (this OMP session) drives Choir through the `choir` CLI — the roster path on
  the real input channel. This **is** the carrier landing (1c) viewed as the
  first self-dev capability: same work, near-term payoff. Blocked on the
  input-path fix (tell → document event). Proves: a harness drives a real
  Choir development task end-to-end.
- **3b — Capability phase 2: skip the harness.** Choir's own actors drive
  development with no external harness process — the self-dev operation
  substrate (`api_self_development.go`, `selfdev/operations.go`, the
  materializer) running on derivable continuations. Needs the ontology
  cutover (1d) so continuations survive restart. Proves: a self-dev operation
  runs to a materialized change with no external driver.
- **3c — Capability phase 3: choir → microVMs via yaegi.** Choir calls out to
  other microVMs through the yaegi capsule substrate. Needs computer→computer
  code publishing (3d). Proves: one computer delegates a bounded task to
  another.
- **3d — The updating system** *(dependency; half-developed)*. Two surfaces:
  (i) **platform→computer security-update push** — `internal/updater` is
  guest-local today (apply/baseline/pinned/journal/restore-prior); no platform
  push path exists; (ii) **computer→computer code publishing** — no surface
  exists (`wirepublish` is article publishing, not code). This is what makes
  self-dev actually deploy: a self-developed change has to reach computers
  without CI.
- **3e — The self-development proof on records** *(the wire gate)*. The queued
  Definition as throughline: candidate A authored via RLM cells, every
  material action carrying `expected`, qualified consensus under
  `reversible-selfdev-v1`, promotion, live-play verification, falsification
  with B, restore to `99949fe2`. The capability phases make this cheap to run;
  the proof is what the vision's gate requires — one real self-change, legibly
  and durably, receipts as the first scored commitment records of a real
  self-development episode.

### Phase 4 — Production hardening + first users

- **4a — Supervision workbench.** Texture + Mail + Newspaper finished as one
  surface where the commitment ledger is what humans supervise — open
  commitments, say-do gaps, materiality coverage, incident reconstruction.
  Proves: a human supervises continuous work at the level of commitments.
- **4b — Beta hardening.** R2 watermark cadence, R1 scoped fault-injection,
  continuation-census residue, problem-doc burndown, heresy-detector CI,
  no-SSH operability, auth/session renewal under load. Proves: production
  users on the computer, repeat use measured.

### Phase 5 — The World Wire

Only after the computer demonstrably develops itself (Phase 3 gate).

- **5a — Wire observation plane.** Source observation modules, immutable
  source versions, `ReportedClaim`/corroboration/contradiction objects on the
  OG, impact propagation (a new source version wakes the standing beliefs it
  bears on — the Phase-1 minted-continuation primitive generalized to
  epistemics). Proves: the computer observes the world continuously on the
  same substrate.
- **5b — Editorial pipeline + publication transaction.** Attention policy,
  investigation trajectories, editorial multisupervision, the typed
  publication/correction transaction binding one exact Texture head +
  provenance manifest + decision receipt, and a public projection that serves
  without fate-sharing with the live computer. Proves: one bounded edition
  publishes continuously; correction is an ordinary forward write.
- **5c — World Wire live.** API/MCP access for orgs, autoradio as the second
  surface, repeat-use evidence on the running record. Proves: the north star.

### After the wire (out of this stack)

- **Box score** — public statements → forecast objects with resolution
  contracts; track records. Starts only after the wire has run in production
  for a few weeks and accumulated resolutions. It is precommitment records
  pointed at the world's commitments — the moat — but it is downstream of a
  live wire, not a pre-wire mission.
- **Platform phase** — white-label newspapers on private+public data.

## The ordered mission list (consensus 2026-09-22)

> **Superseded in part 2026-09-23, revised 2026-09-25.** The M2-tail…M8
> region is replaced by the desk-RLM rectification sequence
> ([`desk-rlm-rectification-plan-2026-09-23.md`](desk-rlm-rectification-plan-2026-09-23.md),
> proposed under owner review). R0 and R2 landed; R3/R4/R5's 2026-09-23
> drafts were deleted 2026-09-25 and the spine restructured by consensus —
> see **§11** of that plan for the current mission list: R2x substrate
> repair → R3a texture ledger consumer → R3b host desk-cell carrier → R3c
> management live + cast → R3d texture authoring → R4 scores/packs →
> M7 → M9a → M11 (the gate). M9b/M10 move after M11; M8 stays subsumed by
> R4; R5 splits into R5a (decoders, gates M11) + R5b (deferred).

Synthesized from a 9-agent convergent panel:

```text
R0  Strand-2 patch
R1  Live vocabulary and documentation cutover
K   Ontology kernel
R2  Commitment ledger, carrier, and semantic-act verbs
R3  Live desks and supervision loop
R4  Scores, surfacing, context packs, and learning-claims gate
R5  Durable vocabulary migration
M7  Skip the harness
M8  Subsumed by R4; no standalone mission
M9a Platform→computer update push
M9b Computer→computer code publish
M10 Choir → microVMs via yaegi
M11 Self-development proof on records
M12 Supervision workbench
M13 Beta hardening
M14 Wire observation plane
M15 Editorial and publication transaction
M16 World Wire live
```

**The spine (restructured 2026-09-25, plan §11):**
`R2x ∥ R3a → R3b → R3c → R3d → R4 → M7 → M9a → M11 → M14 → M15 → M16`.
R3r (research cell) ∥ R3c; R5a anywhere before M11; M9b/M10 after M11.
Owner rulings 2026-09-25: the live Texture doc gates M11 (R3d + R4 are
upstream of the gate), and M7 runs after R3c with the management cell as
driver. M12–M16 remain the World Wire sequence.

**Not missions:** 1a (closed — the evidence floor is M2), 1b (folded into M3),
Phase 0 remainder, box score, `install_frontend_pointer` (platform-shell
deploy contract, separate migration), vmctl/sourcecycled (deferred), a "goal
selector" (dissolved into corrective-value retrieval).

### M1 — the first mission (cleanup)

"Cleanup" is **not** delete-only and **not** the full ~44-instance cutover.
It is the subclass-(a) vertical slice: owner input becomes a document
revision event, and the wrong-path ingress is deleted *in the same move*.

**Delete:** `cmd/choir/roster.go` + test; `choir roster` + `texture
tell|correct` CLI verbs (`main.go`); `/tell` + `/correct` endpoints
(`texture_owner_instruction.go`); `LifecycleOwnerInstruction` type/store
(`types/owner_instruction.go`, `store/lifecycle_owner_instruction.go`); the
`/revise`→`/tell` forwarder (`texture_agent_revision.go:117-121`); the
self-dev synthetic tell (`selfdev_texture_join.go`); the unbound `/revise`
legacy mailbox; dead overlay names after a caller census.

**Replace with:** owner (and harness) writes a document revision — the
revision *is* the event. No owner-intent payload, no new wake table.
Constraint: do not invent a tell-shaped "revision-wake" object M3 would
delete.

**First probe inside M1:** whether owner keystrokes already persist as
document revisions and `tell` is only an extra wake. If yes, M1 is "stop the
extra wake, delete the channel." If no, persist the diff as the revision
event first, then delete.

**Not in M1:** overlay tools (M2/R7), `actuator=tools`/management (M4),
process-local wakes/sweeps (M3), `install_frontend_pointer`, vmctl,
sourcecycled.

**Acceptance:** on staging, a bound Texture doc — owner edit → new revision
on the tape → desk observes the head; `choir roster`/`choir texture tell`
absent; no new `lifecycle_owner_instruction` rows. Mutation class **red**
(canonical input/event authority).

**Dissent recorded:** claude/codex/devin argued for a leaf-only first mission
(delete roster + CLI verbs, keep LOI for M2). Rejected: roster-only deletion
produces no product capability and leaves `/tell` as the live heresy; LOI is
safe to delete *because* the replacement lands in the same mission. The
roster-only deletion is a valid first *commit* inside M1, not a standalone
`/goal`.

### Forced vs judgment

**Forced:** M1 before M2 (no document channel → M2 is another tell-shaped
fake); replacement before deleting live ingress; M3 before M4 (R8/R10 stay
until desks cross); M3 before M7 (derivable continuations); M5 → M6 → M8;
M9b before M10; M11 before M14–M16 (wire gate); M14 → M15 → M16.

**Judgment:** M2 before M3 (carrier proof is live-process only; M3 owns
restart durability — do not claim it in M2's acceptance); M3/M4 split (one
goal file with desk-crossing slices is the alternative); M7 ∥ M6/M8 (3b
doesn't need records; 3e does); M9 after M7 (don't build publish before
there's a self-developed change to publish); M10 not required for M11 (proof
depth is open); M13 after M12 (don't steal the compounding path).

### Risks the panel named

- **M1 dual-path:** document event + leftover `scheduleTextureWorkerWake`
  recreates tell. M1 acceptance forbids the side table.
- **M2-before-M3 is a fake island:** a live-process sub-RLM call that dies on
  restart. M2's proof is live-process only; M3 owns restart.
- **M1 is secretly 1d:** if "the revision wakes the desk" needs the fenced
  dispatcher, stop and re-scope — M1 only admits the revision as an event the
  existing Texture actor already reads.
- **M3 stalls, everything queues.** The hedge is M2 already compounding on
  the live process — only 3b+ is blocked.
- **M5-before-M11 may be wrong:** if the proof should exist before
  instrumentation, invert M5/M6/M8 past M7. Cost: M11 doesn't generate scored
  records; you retrofit.
- **Unbound `/revise` mailbox** (`texture_agent_revision.go:234-309`) is
  leftover subclass (a) — include in M1 or explicitly defer to M3 with a
  named exception.
- **Existing carrier Definition** (`choir-rlm-engineering-carrier-2026-09-11.md`)
  still marked `entrypoint: true` — supersede it when M1 is authored, or
  authority splits (standing Q5).

1. **The carrier decision** (1a) — re-scoped 2026-09-22: roster re-run on the
   real harness; only the evidence floor remains open.
2. **The ontology cutover** (1d) — **ratified 2026-09-22.** It subsumes the
   desk crossings and retires R8/R10 plus the tell/roster path in one move.
3. **Self-development proof depth** (3e) — the capability phases are decided
   (CLI-from-harness → no-harness → microVMs); the open question is what
   evidence satisfies "genuinely develops itself" for the wire gate. If more
   than one episode, Phase 3 is longer and the wire is later.
4. **Does the mechanism precede the self-development proof?** This stack
   says yes (Phase 2 before 3e) so the proof generates scored records. The
   alternative — prove self-development first, then instrument — inverts 2
   and 3.
5. **Where does goal selection live?** The old stack ended at "native goals."
   This stack dissolves the selector into the mechanism (corrective-value
   retrieval over the commitment ledger). If that's wrong, a selection
   mission belongs in Phase 3–4.
6. **Desk-crossing strategy** — resolved by the ratified cutover: the desk
   crossings are its migration targets, not separate missions.

## Risks that could make this plan wrong

- **Context packs may not improve behavior** — the Phase-2 ablation is the
  gate; records still pay as audit if it fails.
- **Resolution scarcity** — newspaper outcomes are slow/ambiguous; the loop
  may not close at a useful rate. Mitigation: Phase 2 uses fast engineering
  outcomes first.
- **Score leakage** — feeding scores into context recreates reward hacking;
  the epistemic-boundary invariant is the control.
- **Ontology cutover is the single largest deletion** — if it stalls, the
  mechanism and product layers queue behind it. The Phase-1 wake repair is
  the hedge.
- **The self-development gate is underspecified** — "genuinely develops
  itself" has no acceptance test yet; Phase 3 could be one mission or five.
- **Owner bandwidth** — the carrier decision plus the remaining open
  questions are the bottleneck; if they stay open, the stack stalls at 1a.
