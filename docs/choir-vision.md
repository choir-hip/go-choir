# The Automatic Computer — Choir Vision

**Status:** product vision — the north star that mission sequencing flows
downstream from. Not an implementation plan, not doctrine. Where this document
and `docs/choir-doctrine.md` conflict, the doctrine is apex; where this
document and a Definition conflict, the Definition governs.

## The One Idea

**Choir is the automatic computer: a persistent computer that works for you
continuously and develops itself under your supervision.**

That is the whole product. Everything else — settlement, forks, publication,
multiplayer authoring, the World Wire — is what the automatic computer does,
not what it is.

## The Order

Two mountains, climbed in exactly one order:

1. **The automatic computer working for supervised self-development.** A
   persistent, per-user computer that runs continuously, corrects itself, and
   improves itself — its own code, its own state, its own way of working —
   within rules the owner grants. This is the core. Until it works, nothing
   else is real.
2. **The World Wire, the automatic newspaper.** Once the computer genuinely
   develops itself, it can run the news: sourcing, writing, verification,
   correction, and publication operating around the clock. The automatic
   newspaper is the payoff the automatic computer enables — the first big
   downstream application, not a separate product.

The order is not negotiable. A computer that cannot develop itself cannot be
trusted to report the world. Self-development comes first because it is the
hardest thing and the thing everything else depends on.

## The Automatic Computer

The product object is a persistent computer, not a conversational session, not
a disposable autoputer, not a publishing surface:

- **Persistent** — it has a stable identity and a canonical event chain. It is
  still there tomorrow, and its history is its state.
- **One tape** — every meaningful change is a typed transaction on the audit
  log. The tape is the authority; every surface is a projection of it.
- **Deterministic** — projections are computed by reducers folding the tape.
  Given the tape and the code version, the same computer can be reconstructed
  anywhere. State is intentional, not an accident of execution: every byte has
  a reason.
- **Durable work** — long-running trajectories run for hours and days, and
  survive restarts, because the database remembers and the tape is replayable.

A human cannot supervise this computer action by action. It runs too long and
too continuously for that. So supervision happens where the work lives: at the
level of ideas and standing state, read off the artifact — not in the
transcript.

## Supervised Self-Development — the first goal

The automatic computer is a self-authoring program: it changes itself through
the same typed transactions it runs on. Each change is a conjecture tested by
execution; the computer learns what it should be by acting on what it
believes, observing results, and revising.

Self-development is **supervised** because the human is the constitutional root
of intention, not because the human approves every consequential action:

- The owner establishes and can revise the policies under which the computer
  acts.
- Effect-specific multiagent consensus evaluates proposals under those
  policies. A human may be a required, optional, or absent participant.
- Irreversible effects remain inside the autonomy window when their stronger
  policy is satisfied. Their consequences cannot be erased by restore, so they
  require stronger evidence, narrower subject binding, durable receipts, and
  compensation or new forward action when correction is needed.
- The owner does not need to be the continuous operator. Supervision of last
  resort is the honest claim: rarely exercised by design, never made to vanish.
- The computer corrects itself within policy—visible, legible, and on the tape;
  reversible state may additionally return to a prior checkpoint.

The mechanism by which self-development becomes real and legible is
**correction as an ordinary write**: a rival proposal is forked, a
policy-governed consensus selects one effective head, and a correction
supersedes a settlement when new admissible evidence falsifies it. The owner
holds constitutional authority over policy; trusted reducers and actuators
enforce each consensus decision. This is the load-bearing spine of the vision —
the thing to make real first — and it is deliberately stated as a consequence
of the vision, not as the vision itself.

The proof target is a self-development candidate accepted on staging: the
computer makes one real change to its own working state, under granted rules,
with the whole story legible on the tape and durable across a restart.

## The World Wire — the downstream payoff

Once the automatic computer works, the automatic newspaper follows: a
continuous publication that reports the world *as reported* — contested and
plural, not a god's-eye index.

- Sources are gathered, synthesized, and written by the computer's own
  durable trajectories.
- Verification and correction are the same settlement machinery that made
  self-development legible, pointed at claims about the world.
- Publication runs 24/7 because the computer itself runs 24/7.

The World Wire is not a separate ontology. It is the automatic computer's
first large application: the same tape, the same artifacts, the same settlement
spine, one more projection. Its reach and honesty are bounded by the
computer's; there is no wire before the computer.

## What This Vision Is Not

- **Not a multiplayer app.** Many people and agents participate, but the unit
  of the product is the computer and its trajectories, not a crowd.
- **Not a settlement protocol.** Settlement, forks, and publication are
  machinery. They are how supervised self-development and the World Wire stay
  legible and honest. Confusing the machinery for the vision is how work
  drifts into implementation detail and loses the big picture.
- **Not a supervised-by-default platform.** The human is the constitutional
  authority and root of intention, but not a universal per-effect approval gate.
  The machine works continuously under effect-specific consensus policies;
  human participation is one policy option.
- **Not a publication product.** The newspaper is a downstream proof of the
  automatic computer, not the product the substrate is built for.
## How Missions Flow From Here

Implementation flows downstream from this vision, in this order:

1. **Supervised self-development, proven.** Restore the automatic computer's
   resident liveness on the existing lifecycle authority, then prove one
   bounded correction loop: a candidate change proposed, settled under a
   granted rule, and legibly corrected — durable across restart, on staging.
2. **Deletion of displaced machinery.** As the proven slice displaces
   duplicated publication and liveness paths, delete them. Cleanup follows
   proof; it does not lead it.
3. **The World Wire.** Only after the computer demonstrably develops itself,
   point the same settlement spine at reporting, verification, and continuous
   publication — the automatic newspaper.

Each step is a Definition with its own pre-flight, evidence class, and landing
loop. The vision does not sequence itself; it sets the order in which
candidate missions are worth writing.

## Why This Vision Is Right

[`signal-is-sparse-not-the-learner-2026-08-01.md`](signal-is-sparse-not-the-learner-2026-08-01.md)
is the foundational argument for this vision: sample inefficiency is undirected
learning, not architecture; signal density comes from a learner with standing
questions and from correction by genuinely independent others; and both have to
be built into the environment the intelligence runs in. Choir is that
environment — the tape is standing state, the harness is owned, plurality is
architecturally available, and correction is an ordinary write. The frontier's
bottleneck is supervision, and this vision is the architecture supervision
lives in.

## Lineage and Authority

- `docs/choir-doctrine.md` — apex doctrine. This vision defers to it.
- `docs/computer-ontology.md` — persistent computer, candidate, promotion
  ontology.
- `docs/definitions/choir-coherent-computer-convergence-2026-07-21.md` — the
  ratified durable-work kernel this vision runs on.
- `docs/archive/vision-choir-category-texture-transclusion-v0.md` — the
  earlier texture/transclusion vision; its re-visioning banner and this
  document supersede its center. The audited-computer core it contains is the
  ancestor of this vision.
