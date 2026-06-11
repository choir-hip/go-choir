---
name: parallax
description: Run a mission as proof search over a living case — conjectures, observer positions, and displacement. Use for any nontrivial mission (10 minutes to overnight) where the route is uncertain, the evidence may mislead, or the work must hand off cleanly. The successor to mission-gradient for new work. The core skill is knowing when to probe and when to move the observer, because the observer's blind spot is where missions die.
version: 1.0.0
metadata:
  hermes:
    tags: [parallax, conjecture-learning, proof-search, long-running-agents]
    related_skills: [mission-gradient, cognitive-transform-portfolio]
---

# Parallax

Every mission is the constructive proof of a scoped claim. The artifact is
the witness. Work is proof search. Truth is measured by displacement of the
observer: a single vantage cannot distinguish its blind spot from the world.

Theory: `docs/conjecture-learning-proof-theory-2026-06-11.md`. Design and
post-mortem lineage: `docs/parallax-design-2026-06-11.md`.

## The Case

State the mission as a constructive claim:

```text
There exists artifact A such that I(A) and Q(A), over domain D.
```

- **I(A)** — hard invariants: the identity that survives every re-theorizing.
  Never optimized across, only inside.
- **Q(A)** — quality clauses: what "good" means, as checkable properties.
- **D** — the scope. Grow D continuously from small-but-real toward
  production; this is the homotopy. A claim whose domain does not embed in
  production's is a fake island, however green its checks.

The case is carried by **driving conjectures** — the claims whose truth or
falsity decides the mission:

```text
CONJECTURE = (CLAIM, TEST, EDGE, DELTA_O, SCOPE)
EDGE class:  independence | resource | missing_oracle | frame_lock
status:      proposed | active | testing | supported | weakened |
             falsified | superseded | promoted_to_assertion
```

An assertion is a supported conjecture with receipts and an explicit scope;
when a premise dies, it reverts — visibly.

## The Circuit

One pass per control interval. Same circuit at every scale; only budgets
differ.

```text
1. CASE      What do I currently believe decides this mission?
2. POSITION  From where am I looking? State it: "from here I can see X
             cheaply; I cannot see Y at all." Name the edge class.
3. MOVE      one of four:
             probe      test a conjecture under the current observer
             shift      move the observer (see catalog below)
             construct  build or extend the witness
             settle     decide the case, or accept-and-name the edge
4. BOUND     smallest substrate that can carry the move; stay inside the
             authority envelope; mutations reversible (candidate/capsule
             when risky; for canonical mutations, the S1–S5 decomposition
             in the proof-theory doc Part II).
5. UPDATE    Record what the move changed — conjecture status, route,
             verifier, scope, or stopping condition. A move that changed
             nothing is evidence about the OBSERVER, not the world.
6. EXIT?     case decided | edges accepted and named | obligation that
             only another authority can discharge → hand off.
```

**The forcing rule.** If the last two moves changed nothing, or the evidence
agrees with you too easily, the next move is a SHIFT. Confirmation is what a
stuck observer produces. Probing harder from a fixed position cannot escape
that position's blind spot.

**Move selection.** Weigh the value of information under the current
observer against the value of observer movement. Probes are cheap and
usually right; shifts are undervalued by default — which is why the circuit
prices them explicitly every pass instead of saving them for blockers.

## The Shift Catalog

Instruments of displacement, by what they change:

- **instrument** — a new test, trace, log, assertion, or measurement the
  current position lacks (the missing_oracle fix);
- **vantage** — read the artifact as a different party (user, attacker,
  maintainer, the next agent) or from a different layer of the stack;
- **vocabulary** — introduce the predicate the current language cannot
  state (the frame_lock fix; renames are shifts, not cosmetics);
- **domain** — shrink D until the claim is decidable, then grow it back
  (the resource fix; design-for-decidability);
- **prover** — hand the claim to an independent agent or checker
  (a proof checked by its own prover is not checked);
- **inversion** — stop seeking confirmation; try to refute. Default to
  refuted when uncertain.

## The Ledger

For every nontrivial claim: the claim **with its scope**, the evidence class
that produced it, the receipt (command, artifact path, trace ref), and the
edge it leaves open. Three rules bind everything:

1. **No claim outruns its evidence class.** Tests are existential; model
   checks are universal over the model; contracts cover the artifact
   checked. "Verified" never renders as "safe."
2. **Untrusted prover.** Your output is a candidate proof until an
   independent checker accepts it. Never verify your own work.
3. **Settlement is earned.** A case is decided when the witness exists with
   scoped receipts, or the claim is refuted, or it is superseded — never
   when effort ran out.

## Settlement

Exit statuses — say which, plainly:

- `settled` — the case is decided; driving conjectures supported (receipts,
  scopes), falsified, or superseded; remaining edges **accepted and named**
  with a next discriminator.
- `open_handoff` — useful ground gained, case undecided. The case file is
  the handoff: current conjecture states, position, last moves, next
  highest-information move. Never present this as settled.
- `blocked` — an obligation only another authority can discharge. Name the
  obligation, the authority, and the smallest discharge.

## Via Negativa

- **No new nouns.** The case lives on a trajectory; obligations are work
  items; moves and evidence are updates; the theory is the assertion
  ledger. (Lineage: Campaign Compiler died of schema.)
- **No cathedral.** Do not specify the mature system to take the first
  step. The circuit at 10 minutes is the circuit overnight.
- **No paperwork as progress.** A ledger entry that changed no move is
  commentary. If the circuit is not changing decisions, say so.
- **No fixed-position grinding.** Repeated confirming probes are a stuck
  observer, not mounting evidence.
- **No fake islands.** Every claim's domain embeds in production's.
- **No self-checked proofs.** Ever.
- **No identity.** You are not "a researcher"; you are currently performing
  search under a conjecture. Obligations, not personas
  (`docs/choir-role-free-actor-protocol-2026-06-11.md`).

## Case File Template

The written control object — keep it this small:

```text
# Case: <name>
claim: exists A such that I(A) and Q(A) over D
invariants (I):
qualities (Q):
domain ramp (D, small -> production):
driving conjectures:        # (CLAIM, TEST, EDGE+class, DELTA_O, SCOPE), falsifier each
position:                   # current observer; what it cannot see
authority envelope / bounds:
ledger:                     # claims with scope, class, receipt, open edge
move log:                   # pass N: position -> move -> what changed
settlement criteria:
status: working | settled | open_handoff | blocked
```

For overnight runs, keep an owner-readable report alongside (what was
proven, with scopes; what shifted the theory; residual edges; next move) —
the case file is for resumption, the report is for the human.

## Runtime Mapping

case ↔ trajectory · obligations ↔ work items · moves/evidence ↔ updates ·
theory ↔ assertion ledger (`docs/conjecture-assertion-ledger-2026-06.md`) ·
decided ↔ settlement · one activation = one or more circuit passes. The
skill is the cognitive layer of the durable-actor protocol; as the M1+
records land, the case file keys to them instead of standing alone.

## /goal Usage

```text
Use Parallax. Work docs/<case-file>.md as proof search: state the case,
state your position and its blind spot each pass, choose probe / shift /
construct / settle with the forcing rule (two null moves or
too-easy agreement forces a shift), bound every mutation, record what each
move changed, and exit only at settled, open_handoff, or blocked — stated
honestly. No claim outruns its evidence class; no self-checked proofs; no
fake islands.
```

## Adoption Gate

Parallax is candidate state. It is promoted only if its first real missions
show a shift that changed the route and claims whose scopes were narrowed by
named edges — and demoted, honestly, if its fields get filled while the
moves stay identical to what mission-gradient would have produced.
