# Current Situation — Where Choir Actually Is

Date: 2026-09-22
Purpose: the owner's missing context. One page on what is true, what is
decided, what is open, and what the docs now say. Read this before the
mission stack.

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

The engineering-desk carrier proved its mechanism but the roster receipt is
quarantined: six strands in the "activation-wake family" share one substrate
cause — a lifecycle transition happens but no durable wake/retry authority is
minted, so work strands until an unrelated sweep/restart/owner action.
Options (from `docs/memo-activation-wake-authority-substrate-2026-09-13.md`):

- **(a)** Charter the wake-authority substrate repair — transition-minted
  recovery occurrences, one consumer, run-terminal = fate-terminal.
- **(b)** Amend the roster scope/evidence floor to match reachable state.
- **(c)** Settle `blocked_incomplete`, charter the repair as a successor.

Why it matters beyond the roster: an unresolved precommitment is *also* a
pending state that must mint its resolver. The wake repair is the prototype
for open commitments (records) and for the wire's impact-propagation. The
panel leaned (a)+(b): repair the defect class, re-scope the roster tally.

### 2. The ontology cutover (unratified, largest single step)

`docs/designs/choir-event-driven-rlm-ontology-minimal-2026-09-15.md` proposes
collapsing delivery/continuation into derivable state (delivered = in state
head, fenced atomic commit, serial-per-actor, cast-only sub-RLMs). It would
subsume the remaining desk crossings and retire the Super substrate (R10) and
`actuator=tools` (R8) in one move. **It is a design, not a ratified mission.**
If ratified, it's the structural death of the wake family. If not, the desk
crossings stay as separate missions and the wake repair is a patch on the old
ontology. This is the biggest sequencing fork.

### 3. How much self-development before the wire

The vision's gate: "a computer that cannot develop itself cannot be trusted
to report the world." One proof (candidate A, falsify B, restore) is
necessary but likely not sufficient — "genuinely develops itself" is a
program, not a single receipt. Open: how many episodes, and does the
precommitment mechanism need to be live first so the episodes generate
scored records?

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
