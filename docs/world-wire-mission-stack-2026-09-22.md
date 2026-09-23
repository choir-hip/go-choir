# The Mission Stack: From Here to World Wire Live

Date: 2026-09-22
Status: **proposed roadmap, awaiting owner ratification.** Supersedes the
ordering rationale of
[`reports/choir-rlm-missions-overview-2026-09-09.md`](reports/choir-rlm-missions-overview-2026-09-09.md)
(the 11-mission stack), which ended at "native goals" and never reached a
product. Mission state still lives in `ACTIVE.md` + `mission-graph.yaml`;
this document supplies the corrected scope and ordering.
Format: every mission below is authored as a **throughline** `/goal` file
(`skills/throughline/SKILL.md`) — the current mission format.
Provenance: synthesized from a 13-agent divergent consensus panel
(`.agentic-consensus/agentic-consensus-20260922-202933/`, 8 substantive
returns) plus the Sep-22 precommitment-records docs and the repo survey in
[`reorientation-docs-overhaul-2026-09-22.md`](reorientation-docs-overhaul-2026-09-22.md).

## The correction in one paragraph

The 11-mission stack was a **harness program**: it built a trustworthy event
log and stopped at a goal selector, with no users, no wire, no record. The
corrected stack treats the commitment ledger as what the substrate is *for* —
**precommitment records are the product mechanism**, and the World Wire is
that ledger made public. The stack is ordered substrate → mechanism →
product → wire, ending at the running record with real users.

## The load-bearing insight (panel consensus)

**An unresolved precommitment is a durable wake obligation — the same object
as the carrier's stranded-continuation bug.** A record that commits a
prediction must be resolved later; that resolution is a continuation. The
carrier's "activation-wake family" defect (a lifecycle transition happens but
no durable wake/retry authority is minted) is the same substrate gap the
records mechanism needs closed. This collapses two missions into one
substrate repair and reframes the carrier's pending owner decision: the wake
repair is not a detour for a roster tally, it is the prototype for open
commitments and for the wire's impact-propagation (a new source version
waking the standing beliefs it bears on).

## What the old stack got wrong (panel consensus)

- **It ended at machinery.** "Native goals" was the endpoint; production
  publication, API, box score, radio, real users, repeat use were absent.
- **Shadow evals (old M9) were mislocated.** A late measurement program that
  can't promote anything is a cathedral beside the deployment logs.
  Precommitment records make every production run a scored episode —
  deployment logs *are* the evaluation.
- **Four serialized desk crossings (old M3–M6) over-weighted ceremony.** The
  minimal ontology subsumes research/management crossings in one collapse.
- **Prompt-bar diet (old M7) and continuation census (old M8) were mid-stack
  gates.** The census is the ontology migration's input; the diet is a
  product-latency repair at the workbench/beta boundary.
- **It treated precommitment records as a side track.** The mechanism lands
  immediately after the substrate can hold it, and the self-development proof
  runs *on* it.
- **No docs-hygiene track.** 604 files, three ledger-skill generations,
  zombie executables — retrieval pollution corrupts every mission's `start`
  receipt.

## The stack

```text
M0  Docs overhaul (parallel green track — in flight, not a dependency)
M1  Wake authority: minted continuations          [red substrate]
M2  Carrier landing                                [red substrate]
M3  Ontology cutover                               [red substrate]
M4  Precommitment records: the ledger in product   [orange mechanism]
M5  Self-development proof on records              [red proof]
M6  Context packs + the learning-claims gate       [orange mechanism]
M7  Supervision workbench                          [orange product]
M8  Beta hardening + first users                   [orange product]
M9  Wire observation plane                         [orange wire]
M10 Editorial pipeline + publication transaction   [red wire]
M11 Box score                                      [orange wire]
M12 World Wire live                                [the north star]
```

### M0 — Docs overhaul (parallel green track)

Phase A–D of `reorientation-docs-overhaul-2026-09-22.md`: kill zombie
executables, PICL→precommitment rename, K3/K4/K6 prune, reorient standing
docs to this stack. Proves: retrieval no longer steers agents to superseded
topology. Gates nothing, but every mission below is cheaper with honest docs.
Residue: ~20 definitions deleted into git history; throughline settled as the
mission format in AGENTS.md.

### M1 — Wake authority: minted continuations

Scope: transition-minted recovery occurrences at every pending lifecycle
state, one consumer (boot reconcile + selection sweep), run-terminal ≠
fate-terminal, idempotent revocation resume — per the 09-13 memo, killing the
six-strand defect *class*. Proves: a pending state mints its own continuation
authority; sweeps consume occurrences instead of enumerating signatures.
Gates: carrier roster unblocks; **also the prototype for open commitments** —
an unresolved precommitment is a pending state that must mint its resolver.
Residue: the general "pending → minted continuation" primitive the records
mechanism (M4) and wire impact-propagation (M9) both reuse.
`better_means`: minimize stranded execution strands per 1,000 transitions
while preserving zero-reentrancy invariants. `goodharting_would_be`: sweeping
dead strands via cron and calling them "resumed."

### M2 — Carrier landing

Scope: finish the roster on the repaired substrate (re-scoped tally —
repaired channels plus live arms, not resurrection of channels the ontology
cutover deletes), then P6: engineering desk fully on the in-cell carrier,
five overlay tools + four legacy capsule ops deleted (R7), run acceptance on
canonical evidence. Proves: a desk lives entirely on the carrier — the desk
the `precommit` module ships through. Gates: `choir.*` in-cell symbol surface
(where `Commit`/`Resolve` land), the replay-harness evidence pattern every
later mission reuses. Residue: R9 provider-heresy deletion.

### M3 — Ontology cutover

Scope: implement the minimal event-driven design — delivered = in state head,
fenced atomic commit {events + head}, serial-per-actor, `not_before` +
dispatcher due-index, `work_id`/`attempt_id`/`status` fold, capability
admission on one arbiter actor, cast-only sub-RLMs, migration script inside a
write fence; management and research cross; `actuator=tools` deleted (R8),
Super substrate retired (R10). Proves: delivery and continuation are
*derivable*, not maintained — the wake family's structural death, which M1
approximated in the old ontology. Gates: everything downstream; the wire
needs cast-only actors at scale, and the event schema is being rewritten
anyway — `expected` on `rlm.spawn` and red/black actions rides the rewrite
(the last cheap moment to add a field before another vocabulary migration).

### M4 — Precommitment records: the ledger in product

Scope: the eight schema decisions — `expected` linkage +
`resolves`/`prediction_ref`, epistemic-boundary invariant, deterministic
pairing projection onto the OG, async Curator desk minting
`learning_record_minted`, result-wake policy, REPL-queryable records with
auto-inject OFF, actor/model scoping — plus the `precommit` Yaegi module
(`Commit`/`Resolve` via `ChoirExports`), the `Scorer` interface (self-score
default; second-LLM and Jev as config), typed question sets, specificity
scoring. Proves: commit → observe → score → revise → persist → retrieve runs
end-to-end on the real tape; replay reproduces the pairing fold
deterministically; agents cannot edit committed records. Gates: context
packs, procedural fidelity, the box score's schema. Residue: the Markdown
conjecture ledger + doccheck regex retire — the record type is an OG object
with provenance edges, never a third store.

### M5 — Self-development proof on records

Scope: the queued self-development Definition rewritten as throughline —
candidate A authored via RLM cells, every material action carrying
`expected`, qualified consensus under `reversible-selfdev-v1`, promotion,
live-play verification, falsification with B, restore to `99949fe2`. Proves:
the computer makes one real change to itself, legibly and durably — *and the
proof's receipts are the first scored commitment records of a real
self-development episode*: say-do gaps, consequence coverage, procedural
fidelity measured on an actual autonomous change. Gates: effects ON
(bounded); the vision's gate for the wire. Residue: first real corpus for
context packs; procedural-fidelity baseline per desk.

### M6 — Context packs + the learning-claims gate

Scope: aggregate M4–M5 records per model+task; build first packs by
corrective value; run the sealed pre-state replay control the 09-17 panel
demanded (does temporal commitment beat outcome-blind replay of the same
prefix?); flip retrieval ON for pack consumers only if the evidence supports
it; Brier/calibration/disagreement curves per desk. Proves: **behavior
change** — the only admissible evidence of learning (remove a record → its
effect goes; restore → it returns) — or falsifies the learning conjecture
cheaply while records still pay as audit. Gates: any learning claim in
pitch/product; model-selection routing that learns from `choir.Call`
expectations. Residue: calibrated scorer table.

### M7 — Supervision workbench

Scope: Texture + Mail + Newspaper finished as one surface where **the
commitment ledger is what humans supervise** — open commitments, say-do gaps,
materiality coverage, incident reconstruction from nested records; the
internal ancestor of the box score. Proves: a human can supervise continuous
work at the level of commitments, not transcripts. Gates: beta users; the
wire's editorial supervision reuses these seats. Residue: prompt-bar diet
lands here, where its latency is a product bug.

### M8 — Beta hardening + first users

Scope: R2 watermark cadence, R1 scoped fault-injection control,
continuation-census residue, problem-doc burndown, heresy-detector CI wiring,
no-SSH operability contract, auth/session renewal under load. Proves:
production users on the computer, repeat use measured. Gates: wire-live
claims need a computer that survives real usage; distribution evidence
starts here. Residue: ops runbooks; the honest incident log that is itself a
records corpus.

### M9 — Wire observation plane

Scope: source observation modules (sourcecycled narrowed to capture-only),
immutable source versions, `ReportedClaim`/corroboration/contradiction
objects on the OG, and **impact propagation: a new source version wakes the
standing beliefs it bears on** — the M1 minted-continuation primitive
generalized from lifecycle to epistemics. Proves: the computer observes the
world continuously on the same substrate — no second scheduler, no
news-specific loop. Gates: anything the wire publishes; the box score's raw
material. Residue: backpressure policy; tainted-source handling.

### M10 — Editorial pipeline + publication transaction

Scope: attention policy, investigation trajectories, editorial
multisupervision seats (cognitive *and* evidentiary independence), the typed
publication/correction transaction to corpusd binding one exact Texture head
+ provenance manifest + decision receipt, and a public projection that serves
without fate-sharing with the live computer (standing question 7 — the
failure the old wire already shipped once). Proves: one bounded edition
publishes continuously; correction is an ordinary forward write with visible
lineage. Gates: public exposure — effectively irreversible, the
highest-policy mission in the stack. Residue: edition = immutable attention
snapshot.

### M11 — Box score

Scope: public statements → forecast objects with resolution contracts; the
track-record ledger; resolution sweeps; per-figure/per-institution scores —
**precommitment records applied to the world's commitments**, consumer-facing.
Proves: the moat claim — a provenance-linked resolvable record a wrapper
cannot build. Gates: the "sell the record" pitch becomes a product; airtime/
attention allocated by track record. Residue: the cold-start clock started at
M9 (shadow accumulation from first observation); disagreement sampling for
audit.

### M12 — World Wire live

Scope: API/MCP access for orgs, autoradio as the second surface (DJ desk +
open-source audio), repeat-use evidence on the running record. Proves: the
north star — the record running in production with real users, the
commitment ledger underneath, the box score accumulating. Gates: the platform
phase (white-label newspapers on private+public data). Residue: whether the
wire's usage feeds records that improve the computer — the flywheel the whole
stack bet on.

## The carrier decision, folded

The pending owner decision resolves as **(a) charter the wake-authority
repair + (b) amend the roster scope**, combined:

- The repair is correct regardless of roster outcome: it kills the defect
  class, and the minted-continuation primitive is what M4 (open commitments)
  and M9 (impact propagation) both need. Not "another sweep for the roster" —
  the obligation mechanism the rest of the stack stands on.
- The roster tally is re-scoped, not honored literally: resurrecting channels
  the ontology cutover deletes is ceremony. Evidence floor: repaired channels
  + served arms + the stranded-saga receipt closed.
- If the owner instead picks (c) `blocked_incomplete`: the stack degrades to
  the ontology-first inversion — M3 becomes the wake fix and `precommit`
  ships via `ChoirExports` on the partial carrier. The plan survives all
  three outcomes; only the evidence floor changes.

## Divergent alternatives the panel raised (kept visible, not chosen)

- **Wire-as-gym inversion** (grok, sol): the newspaper's public oracle is the
  best training domain; invert the vision order. Rejected: violates
  owner-doctrine "no wire before the computer"; a public garbage record with
  a box score of confident wrongness is the failure mode.
- **Box-score-first wedge** (sol, grok): sell the track record before the
  newsroom. Rejected as the *first* mission — it becomes a forecasting
  platform beside Choir — but absorbed: M11's box score is the same mechanism
  pointed at public claims.
- **Dogfood/gym-first** (claude, cursor): throughline's own ledger becomes
  the first record stream. Absorbed: M0+M4 make the mission format emit
  records; the self-development proof (M5) is the dogfood episode.
- **Parent+children homotopy** (grok, cursor): one "World Wire live" goal
  with parallel children instead of a serial stack. Rejected as the registry
  topology (three spines violates the zero-or-one-entrypoint rule) but
  absorbed as the realism axis: each mission's `realism_axis` runs
  private→public, unscored→scored.
- **Ontology-first** (devin option 2): skip M1; the minimal ontology *is* the
  wake repair done right. Kept as the fallback if the owner picks (c) — see
  the carrier fold above.

## Risks that could make this plan wrong

- **Context packs may not improve behavior** — the ablation (M6) is the gate;
  records still pay as audit if it fails.
- **Resolution scarcity** — newspaper outcomes are slow/ambiguous; the loop
  may not close at a useful rate. Mitigation: M4–M6 use fast engineering
  outcomes first.
- **Score leakage** — feeding scores into context recreates reward hacking;
  the epistemic-boundary invariant (M4) is the control.
- **Ontology cutover is the single largest deletion** — if M3 stalls, the
  mechanism and product layers queue behind it. The M1 repair is the
  de-risking hedge.
- **Owner bandwidth** — the carrier decision plus the docs overhaul are both
  open; if both stay open, the stack stalls at M1.
