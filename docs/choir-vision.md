# The Automatic Computer — Choir Vision

**Status:** product vision — the north star that mission sequencing flows
downstream from. Not an implementation plan, not doctrine. Where this document
and `docs/choir-doctrine.md` conflict, the doctrine is apex; where this
document and a Definition conflict, the Definition governs. Current sequencing
lives in `docs/ACTIVE.md`, not here.

## The One Idea

**Choir is the automatic computer: a persistent computer that works for you
continuously and develops itself under your supervision.**

The usual agent product gives an agent a computer and a chat thread to reach
you through. Choir inverts it: **the multi-agent system is the computer.** AI
is framed not as a worker that replaces a person but as the thing that replaces
the computer. Everything else — settlement, forks, publication, Autopaper,
Autoradio, reputation — is what the automatic computer does, not what it is.

The tagline (owner, 2026-10-14) is **multiplayer intelligence for the learning
economy**: people and agents learning from each other's settled records.

## The Order

1. **The automatic computer, working for supervised self-development.** A
   persistent per-user computer that runs continuously, corrects itself, and
   improves itself — its own code, state, and way of working — within rules the
   owner grants. This is the core; a computer that cannot develop itself cannot
   be trusted to report the world.
2. **Autopaper, the automatic newspaper — downstream, and now live.** Live since
   2026-10-13 as a platform tier: scheduled sources polled around the clock,
   stories written and revised by the computer's own desks, citations checked,
   versions published at public permalinks. It proves the substrate carries
   real continuous work; it is the first large application of the computer, not
   a separate ontology. Its synthesis quality is still maturing, and it runs
   ahead of the self-development gate by the owner's deliberate choice, which
   makes proving self-development on the new substrate the weak link and the
   next priority.
3. **Autoradio**, the interactive, two-way listening and voice surface over the
   same objects: a prototype only with free capacity and framed as one,
   production after the architecture work, with a no-slop bar.

The owner's standing correction (2026-10-14): **deepen foundations, do not
accelerate growth.** Growth, go-to-market and fundraising come after the
software reaches diminishing marginal returns for more human attention.

World Wire, Universal Wire and `sourcecycled` are retired names for Autopaper;
some code identifiers keep them until the rename
(`docs/autopaper-sw-design-2026-10-09.md` §9). The shared graph of published
objects that computers adopt onto their own tapes is **the Autopaper object
graph**.

## The Automatic Computer

The product object is a persistent computer — not a conversational session, not
a disposable autoputer, not a publishing surface:

- **Persistent** — stable identity (`ComputerID`) and a canonical event chain;
  its history is its state.
- **One tape** — every meaningful change is a typed transaction on the audit
  log. The tape is the authority; every surface is a projection of it.
- **Deterministic** — projections are reducers folding the tape; given the tape
  and the code version, the computer can be reconstructed anywhere.
- **Durable work** — trajectories run for hours and days. A crash ends work; a
  planned restart may resume it (`CLAUDE.md`).

Persistent root desks share the work. Three system desks: **management**
(coherence, attention, spend, admission), **engineering** (mutation only inside
disposable capsules) and **research** (read-only toward the world). App desks,
one per app agent: **Texture** first (the only agent that writes documents; the
owner-facing supervision surface), then Mail, Calendar, Slides and others as
apps become app agents. They talk through precommitment records.

A human cannot supervise this computer action by action. Supervision happens
where the work lives: at the level of ideas and standing state, read off the
artifact — not in the transcript.

## Supervised Self-Development — the first goal

The computer changes itself through the same typed transactions it runs on.
Each change is a conjecture tested by execution. Self-development is
**supervised** because the human is the constitutional root of intention, not
because the human approves every effect:

- The owner sets and can revise the policies under which the computer acts.
- Effect-specific multiagent consensus evaluates proposals under those
  policies; a human may be a required, optional, or absent participant.
- Irreversible effects stay inside the autonomy window when their stronger
  policy is satisfied (stronger evidence, narrower binding, durable receipts,
  compensation or forward action), because restore cannot erase them.
- Supervision of last resort is the honest claim: rarely exercised by design,
  never made to vanish.

The mechanism is **correction as an ordinary write**: a rival proposal is
forked, a policy-governed consensus selects one effective head, and a
correction supersedes a settlement when new admissible evidence falsifies it.

The proof target is a self-development program on staging: real changes to its
own working state, under granted rules, legible on the tape, durable across
restarts. One complete episode has been shown (2026-10-10); the gate must be
re-proven on the records substrate. Self-development is only real when built on
precommitment scoring — self-supervising and self-learning, not another app
builder.

## Precommitment Records: The Core Mechanism

Before a step with a verifiable outcome, an agent commits a typed prediction
(hypothesis, assumptions, alternatives, probabilities); the outcome resolves and
scores it; the accumulated log is the agent's evolving world model. This is
prospective self-alignment (PSA), the core of the product:

- **In-context reinforcement learning.** Records live in the desk's REPL and come
  back as context packs and on-demand retrieval, so feedback teaches a model
  without touching its weights.
- **Verifiability from unverifiable work.** Turning a task into a scoreable
  prediction elicits a verifiable signal where there was none.
- **Oversight.** The same records, viewed by role, are the supervision surface.
  Several scorers (self, peer desks, Jev, rules, humans) score each settlement,
  and each scorer is itself scored.

Documentation-driven development is the same pattern applied to the builders:
goal files with value criteria, typed conjectures and named evidence are
precommitments for coding agents. See
[`precommitment-records.md`](precommitment-records.md).

## Multiplayer By Design

Choir is multiplayer by design, which is why its interfaces are content-centric
— Texture documents, records, supervision surfaces — rather than chat logs. Chat
does not scale to long horizons, many agents or many users: after an eight-hour
run you do not read the message log, and an eight-day run with eighty agents and
eight teammates is unreadable as one. Multiplayer happens between computers
through publication and adoption, never direct merge.

The end state is an **organizational operating system**: the organization is a
multi-agent system in which humans are one type of agent and language-model
agents another. One Texture supervises one task, the root Texture supervises all
Textures in a computer, and a further level supervises many computers. Executives
and managers oversee every agent in their purview, now and historically,
through an interactive surface — not a dashboard. Chat agents remain interfaces
into Choir, which runs in the background around the clock. Each app becomes an
app agent: its own desk, with front ends as replaceable clients.

## What This Vision Is Not

- **Not chat-first** (above).
- **Not a settlement protocol.** Settlement, forks and publication are machinery
  that keeps self-development and Autopaper legible and honest.
- **Not supervised-by-default.** The human is the root of intention, not a
  universal per-effect approval gate; human participation is one policy option.
- **Not a publication product.** Autopaper is live and matters, but it is a tier
  the substrate carries, not what the substrate exists for.
- **Not growth-first.** Autoradio may drive growth; that is no reason to
  accelerate it before the foundations are deep.

## Why This Vision Is Right

[`signal-is-sparse-not-the-learner-2026-08-01.md`](signal-is-sparse-not-the-learner-2026-08-01.md)
is the foundational argument: sample inefficiency is undirected learning, not
architecture; signal density comes from a learner with standing questions and
from correction by genuinely independent others; both must be built into the
environment the intelligence runs in. Choir is that environment — the tape is
standing state, the harness is owned, plurality is architecturally available,
correction is an ordinary write, and precommitment records are how standing
questions become scoreable. The frontier's bottleneck is supervision, and this
vision is the architecture supervision lives in.

## Lineage and Authority

- `docs/choir-doctrine.md` — apex doctrine; this vision defers to it.
- `docs/computer-ontology.md` — persistent computer, candidate, promotion
  ontology.
- `docs/precommitment-records.md` — the core mechanism.
- `docs/signal-is-sparse-not-the-learner-2026-08-01.md` — the foundational
  argument.
- `docs/archive/reports/choir-state-and-vision-2026-10-14.md` — what is built, designed
  and vision today, and the business model.
- `docs/ACTIVE.md` — current sequencing (`docs/world-wire-mission-stack-2026-09-22.md`
  is a superseded roadmap kept for reference).
- `docs/archive/choir-coherent-computer-convergence-2026-07-21.md` — the
  ratified durable-work kernel this vision runs on.
