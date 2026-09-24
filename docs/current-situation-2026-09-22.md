# Current Situation — Where Choir Actually Is

Date: 2026-09-22
Purpose: the owner's missing context. One page on what is true, what is
decided, what is open, and what the docs now say. Read this before the
mission stack.
> **Supersession — 2026-09-23:** The
> [desk-RLM rectification plan](desk-rlm-rectification-plan-2026-09-23.md)
> supersedes this snapshot's M2-tail mission sequencing. The roster re-run
> wording below and Phase 1's “roster path” are stale: the current path is a
> document-channel delegated sub-RLM `choir.Cast`. The R8/R10 deferral note is
> superseded by R2/R3.


## The product direction (decided)

Choir is the automatic computer — a persistent computer for supervised
self-development first, the World Wire downstream. The mechanism that makes
supervision real is **precommitment records** (renamed from PICL): agents
commit typed predictions before acting, outcomes resolve and score them, and
the accumulated log is the computer's world model. It is the learning loop,
the alignment surface, and the audit trail — and it is what humans supervise.
It is **in product** (owner direction 2026-09-22), not a side track.

## The mission format (decided)

**Throughline** (`skills/throughline/SKILL.md`) is the current `/goal` file
format. The lineage: mission-gradient → parallax → throughline. All three
earlier skills — `mission-gradient`, `parallax`, and `definition` — are
**deprecated, kept for reference only**; do not author new goal files with
them. Existing goal files written in the definition format remain valid and
executable.

## Where the work actually stands

- **Substrate landed.** Dolt storage normalized (commit leak fixed, platform/
  corpus split, CAS externalization, recovery verified). The event log is
  trustworthy.
- **Carrier (engineering desk on RLM) is blocked** on an owner decision —
  see open questions.
- **Product on staging:** Texture + Mail live on choir.news; Newspaper in
  refactor; Radio in dev.
- **Precommitment records: designed, not built.** Two Sep-22 docs are the
  canonical statement (theory + engineering memo). No runtime record type
  exists yet — the current "ledger" is Markdown + a doccheck regex.
- **Docs overhauled 2026-09-22:** zombie authority killed, ~14 superseded
  definitions retired to Git history, PICL renamed on customer-facing docs,
  manifest/graph/ACTIVE repaired. A second consistency sweep (5-scout fanout)
  then re-aligned the authority layer (doctrine, product-doctrine, ontology,
  standing-questions now say throughline + precommitment records, not the
  superseded effects Definition), tombstoned remaining zombie `next_action`s
  and deleted-file refs across definitions, and bannered ~30 dated
  reports/reviews/designs as stale evidence. 604→~590 live docs.

## Open questions (what needs your call)

These are the decisions that gate the sequence. Each is named with where it
lives and what it changes.

### 1. The carrier decision (the immediate one)

The engineering-desk carrier proved its mechanism (A9 first overlay-served
arm; A14 executed task cells and staged `choir.Complete`) but the roster
receipt is quarantined. **Two corrections landed 2026-09-22 that re-scope
this:**

- **The roster was run on a makeshift harness, not the RLM carrier.**
  `cmd/choir/roster.go` drives arms via `texture tell` — an out-of-band
  owner-instruction message — not the desk's native sub-RLM spawn path. The
  1-of-4 tally measured a tell→desk-translation→drainer-residency chain, not
  the carrier. The roster must be re-run on the real harness.
- **`texture tell` is a bug, not a feature.** Texture is document-driven:
  owner input is an edit to the document (an event on the trajectory the
  desk reads), not an out-of-band message. `tell`/`correct` and the whole
  `LifecycleOwnerInstruction` path are a second input channel that bypasses
  the tape — to be deleted, and the roster driver with it.

What remains of the original decision is only the evidence floor — and owner
correction 2026-09-22 sharpens it: **roster isn't a concept at all; it's just
a sub-RLM call.** Not "re-run the roster on the real harness" but "a sub-RLM
cast on the document channel drives a task; the roster driver is deleted, not
migrated." The substrate half (the wake defect) is carried by the ontology
cutover below.

### 2. The ontology cutover — **ratified 2026-09-22**

`docs/archive/choir-event-driven-rlm-ontology-minimal-2026-09-15.md` is
**ratified**: collapse delivery/continuation into derivable state (delivered =
in state head, fenced atomic commit, serial-per-actor, cast-only sub-RLMs).
It subsumes the remaining desk crossings and retires the Super substrate
(R10), `actuator=tools` (R8), and the `tell`/`roster` out-of-band path in one
move — the structural death of the wake family. The standalone wake repair
is folded into it; the desk crossings become migration targets. The full
wrong-path cluster it deletes is enumerated in
`docs/problems/root-cause-wrong-path-cluster-2026-09-22.md` (~44 instances:
out-of-band input, process-local continuations, dual paths, sweep recovery,
non-event mutations).

### 3. Self-development — the acceleration axis (owner direction 2026-09-22)

Self-development is the priority because it compounds: the more Choir drives
its own development, the faster everything lands, and it cuts the CI pipeline
out of the UX-improvement loop. Current state: email works, Texture is broken
(the `tell`/roster input path — the wrong-path cluster). Three capability
phases, then the proof:

- **Phase 1 — `choir` CLI from a harness** (ASAP): an external harness drives
  Choir through the CLI — the roster path on the real input channel. This is
  the carrier landing (stack 1c) viewed as the first self-dev capability.
- **Phase 2 — skip the harness** (soon): Choir's own actors drive development
  on derivable continuations — needs the ontology cutover (1d).
- **Phase 3 — choir → microVMs via yaegi** (eventually): needs
  computer→computer code publishing.
- **The updating system** is a named dependency and half-developed:
  platform→computer security-update push has no path (the updater is
  guest-local), and computer→computer code publishing has no surface
  (`wirepublish` is articles, not code). This is what makes self-dev deploy
  without CI.

The wire gate stays: the self-development *proof* on records (stack 3e) is
still what "genuinely develops itself" requires before the wire.

### 4. Box score timing (resolved: out of the stack)

The box score (track records on public claims) is **not** a pre-wire mission.
It comes after the World Wire has run in production for a few weeks and
accumulated resolutions. Removed from the stack.

### 5. Deck vs bones

`docs/deck/pitch-bones-vision.md` (09-18) is the vision-deck source — I
renamed PICL→precommitment in it. The seed deck (09-22, `choir-seed-deck-…`)
is a **separate, newer artifact** — different framing (supervision workbench,
"the work itself is the interface"), and it doesn't mention the learning
loop. The bones are stale relative to the current pitch; they need updating
to match, or the seed deck becomes the canonical pitch and the bones retire.
The two 09-18 HTML renders still say PICL — re-render from the renamed source
or retire them.

### 6. Engineering-memo open decisions

`Precommitment Records — Engineering Memo.md` lists 8 open decisions (final
name, default question set, material action classes + spending threshold,
default scorer per event type, retention/compression, surprise threshold,
circuit-breaker thresholds, record spec to publish). None block the schema;
all block the first implementation's defaults.

### 7. Standing residues (R1–R11)

- **R1** live failure-injection drill — blocked, needs a scoped product control.
- **R2** watermark cadence — periodic near-head W publication.
- **R3** proxy boot-screen during resolve lock — cosmetic.
- **R6** cutover final disposition — remainder holder, awaits named successor.
- **R7** tool/operation retirement — owned by the carrier.
- **R8** `actuator=tools` deletion — when every desk crosses.
- **R9** deepseek/xiaomi provider heresy — carrier deletion completes it.
- **R10** Super substrate retirement — the ontology cutover (M3) carries it.
- **R11** `items.body` CAS externalization — world-wire search redesign.

### 8. Follow-ups not yet done

- **Legal docs** (`docs/legal/terms-of-service.md`, `privacy-policy.md`):
  product/storage descriptions predate Texture, Mail/attachments, and
  precommitment records — refresh before any external use.
- **HTML decks** (above) — re-render or retire.
- **~20 dated problem receipts** (`docs/problems/`, mostly 2026-08-28 boot
  cluster): several read as open but were superseded by later repairs —
  closure banners still needed.

## The docs map (post-overhaul)

- **Authority:** `choir-doctrine.md` (apex) → `AGENTS.md` (contract) →
  promoted goal file (sole executable) → `ACTIVE.md` (view) →
  `mission-graph.yaml` (discovery) → evidence.
- **Roadmap:** `world-wire-mission-stack-2026-09-22.md` (proposed; under
  revision — see its open-questions section).
- **Mechanism:** the two Sep-22 precommitment-records docs.
- **This doc:** the situation brief. `reorientation-docs-overhaul-2026-09-22.md`
  is the overhaul record.
