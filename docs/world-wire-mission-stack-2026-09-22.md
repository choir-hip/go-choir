# The Mission Stack: From Here to World Wire Live

Date: 2026-09-22 (revised same-day after owner review)
Status: **proposed roadmap, under owner review — not ratified.** Supersedes the
ordering rationale of
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
  What remains is only the roster-evidence floor: re-scope it to roster arms
  driven as in-cell sub-RLM casts on the document channel. The substrate half
  (the wake defect) is carried by 1d.
- **1b — Wake authority: minted continuations** *(folded into 1d)*. The
  transition-minted recovery-occurrence repair is subsumed by the ontology
  cutover's derivable-continuation model — pending deliveries are a
  projection, the dispatcher is the one consumer. Not a separate mission.
- **1c — Carrier landing.** Finish the roster on the real harness (in-cell
  sub-RLM casts, not `texture tell`), then P6: engineering desk fully on the
  in-cell carrier, five overlay tools + four legacy capsule ops deleted (R7),
  run acceptance on canonical evidence, R9 provider-heresy deletion. Proves:
  a desk lives entirely on the carrier — the desk the `precommit` module
  ships through.
- **1d — Ontology cutover** *(ratified 2026-09-22)*. Implement the minimal
  event-driven design: delivered = in state head, fenced atomic commit,
  serial-per-actor, cast-only sub-RLMs, migration inside a write fence;
  management and research cross; `actuator=tools` deleted (R8), Super
  substrate retired (R10), and the `tell`/`correct`/`LifecycleOwnerInstruction`
  out-of-band path plus `cmd/choir/roster.go` deleted — owner input becomes a
  document edit event. Proves: delivery and continuation are *derivable* —
  the wake family's structural death. Scope: deletes the wrong-path cluster
  enumerated in `docs/problems/root-cause-wrong-path-cluster-2026-09-22.md`
  (~44 instances across out-of-band input, process-local continuations, dual
  paths, sweep recovery, non-event mutations); five exceptions need real
  decisions (tell payload semantics, roster migration, actuator=tools/Super
  blocked on desk crossings, frontend pointer, vmctl/sourcecycled boundary).
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
