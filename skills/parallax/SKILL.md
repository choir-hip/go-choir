---
name: parallax
description: Run a mission as a conjecture circuit: the mission document claims that completing an artifact/spec/objective will actually advance a deeper goal, then tests and constructs that claim through observer shifts. Use for any nontrivial /goal mission where the route is uncertain, the evidence may mislead, or the work must hand off cleanly.
version: 1.2.0
metadata:
  hermes:
    tags: [parallax, conjecture-learning, proof-search, long-running-agents]
    related_skills: [mission-gradient, cognitive-transform-portfolio]
---

# Parallax

Parallax is the **conjecture circuit** for missions. A mission document claims
that completing some artifact, following some spec, or achieving some
objective will actually advance a deeper goal. Work is proof search over that
claim. The artifact is the witness; observer shifts keep the mission from
confusing a local proxy with real success.

Theory: `docs/conjecture-learning-proof-theory-2026-06-11.md`; design:
`docs/parallax-design-2026-06-11.md`.

## The Mission Conjecture

The mission document is the source program. `/goal docs/<mission>.md` should
be sufficient: read that document, follow its references, compile any missing
conjecture fields into the same document, then execute and update it. Do not
create a separate control file unless the mission document explicitly asks
for one.

State the mission as a conjecture:

```text
If witness A satisfies spec/objective S under invariants I and quality Q
over domain D, then deeper goal G is achieved or materially advanced.
```

- **G** — the deeper goal: why the mission matters beyond task completion.
- **A/S** — the witness and spec/objective: what will be built, changed,
  proven, or decided.
- **I/Q** — hard invariants and quality clauses; never optimize across I.
- **D** — the scope. Grow D continuously from small-but-real toward production.
  A claim whose domain does not embed in production's is a fake island.

The load-bearing conjecture is often the bridge `A satisfies S => G`. Treat
that bridge as suspect until evidence supports it. Many missions fail by
achieving the stated objective while missing the deeper goal.

```text
CONJECTURE = (CLAIM, TEST, EDGE, DELTA_O, SCOPE)
EDGE class:  independence | resource | missing_oracle | frame_lock
status:      proposed | active | testing | supported | weakened | falsified | superseded | promoted_to_assertion
```

An assertion is a supported conjecture with receipts and an explicit scope;
when a premise dies, it reverts — visibly.

## The Mission Document

A mission document may begin as research, architecture, a spec, an objective,
or an initial conjecture set. Compile those source forms just-in-time into
the mission conjecture. Preserve the author's text; add or update a compact
**Parallax State** section rather than rewriting the document into a template.

At mission start: read the document and required references; extract
objective, artifact, invariants, qualities, domain/acceptance target,
authority, initial conjectures, blind edges, and obligations; infer
conservatively when safe; ask only when artifact identity, authority, or
safety is ambiguous. Then execute from the compiled state and update it after
moves that change conjecture status, position, scope, verifier, artifact
state, or settlement.

```text
## Parallax State
status: working | settled | open_handoff | blocked | superseded
mission conjecture: if A satisfies S under I/Q over D, then G advances
deeper goal (G):
witness/spec (A/S):
invariants / qualities / domain ramp (I/Q/D):
authority / bounds:
bridge conjecture + sub-conjectures / position:
ledger / move log:
version / lineage:
learning state: retained here / promoted outward / successor links
settlement:
```

For behavior-changing Choir platform missions, this same mission document
must also carry the landing proof: commit, push, CI, deploy identity, staging
acceptance, rollback refs, and residual risks. Local proof is not settlement
for vmctl, candidate computers, gateway/model calls, promotion, rollback, or
Choir-in-Choir behavior.

The mission document is mutable and versioned. Append concise revision notes
when the conjecture, witness/spec, domain, observer, route, or settlement
changes. If a later mission completes the outcome, do not abandon this one:
mark it `superseded` or `open_handoff`, link the successor, migrate live
conjectures and open edges, and leave this document in a learning-bearing
state.

## The Circuit

One pass per control interval. Same circuit at every scale; only budgets
differ.

```text
1. CLAIM     What conjecture currently decides this mission?
2. POSITION  From where am I looking? State it: "from here I can see X
             cheaply; I cannot see Y at all." Name the edge class.
3. MOVE      one of four:
             probe      test a conjecture under the current observer
             shift      move the observer (see catalog below)
             construct  build or extend the witness
             settle     decide the conjecture, or accept-and-name the edge
4. BOUND     smallest substrate that can carry the move; stay inside the
             authority envelope; mutations reversible (candidate/capsule
             when risky; for canonical mutations, the S1–S5 decomposition
             in the proof-theory doc Part II).
5. UPDATE    Record what the move changed — conjecture status, route,
             verifier, scope, codebase learning, or stopping condition. A
             move that changed nothing is evidence about the OBSERVER, not
             the world.
6. EXIT?     conjecture decided | superseded | edges accepted and named |
             obligation only another authority can discharge → hand off.
```

**The forcing rule.** If the last two moves changed nothing, or the evidence
agrees with you too easily, the next move is a SHIFT. Confirmation is what a
stuck observer produces. Probing harder from a fixed position cannot escape
that position's blind spot.

**The learning rule.** Repeated obstacles are evidence about the conjecture,
not just the route. When the same class of obstacle recurs, reconsider the
bridge `A satisfies S => G`: the witness may be wrong, the spec may be a
proxy, the domain may not embed, or the observer may lack the predicate that
would reveal the real goal. Update, weaken, split, or supersede the
conjecture before grinding further.

**The retention rule.** Every mission leaves its mission document as the
durable learning artifact, even when it fails or is superseded. Promote
learning outward only when it changes shared doctrine, assertions,
architecture, tests/specs, skills, or successor work. Do not let partial
missions vanish as chat memory.

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
- **inversion** — stop seeking confirmation; try to refute. Treat uncertain
  claims as unsupported for promotion or settlement until checked.

When extra perspective is required, use
`cognitive-transform-portfolio/SKILL.md` as a shift amplifier. Select only
transforms that change the next probe, route, scope, verifier, or stopping
condition; otherwise they are commentary.

## The Ledger

For every nontrivial claim: the claim **with its scope**, the evidence class
that produced it, the receipt (command, artifact path, trace ref), and the
edge it leaves open. Three rules bind everything:

1. **No claim outruns its evidence class.** Tests are existential; model
   checks are universal over the model; contracts cover the artifact
   checked. "Verified" never renders as "safe."
2. **Untrusted prover.** Your output is a candidate proof until an
   independent checker accepts it. Never verify your own work.
3. **Settlement is earned.** A mission is settled when the witness exists with
   scoped receipts, or the claim is refuted, or it is superseded — never
   when effort ran out.

## Settlement

Exit statuses — say which, plainly:

- `settled` — the mission conjecture is decided; driving conjectures
  supported (receipts, scopes), falsified, or superseded; remaining edges
  **accepted and named** with a next discriminator.
- `open_handoff` — useful ground gained, conjecture undecided; the mission doc
  carries conjecture states, position, last moves, and next move. Never call
  this settled.
- `blocked` — an obligation only another authority can discharge. Name the
  obligation, the authority, and the smallest discharge.
- `superseded` — a better conjecture or successor mission now carries the
  work. Link it, migrate live obligations, and retain the learning state
  before stopping.

## Via Negativa

- **No new nouns:** mission conjecture ↔ trajectory; obligations ↔ work
  items; moves and evidence ↔ updates; theory ↔ assertion ledger.
- **No cathedral:** the 10-minute circuit is the overnight circuit.
- **No paperwork as progress:** if the circuit changes no decision, say so.
- **No fixed-position grinding:** repeated confirmation is a stuck observer.
- **No obstacle grinding:** repeated obstacles force conjecture revision.
- **No abandoned missions:** close with settlement, handoff, blocker, or
  supersession; retain learning state first.
- **No fake islands:** every domain embeds in production's.
- **No self-checked proofs.**
- **No identity:** obligations, not personas
  (`docs/choir-role-free-actor-protocol-2026-06-11.md`).

## Runtime Mapping

mission conjecture ↔ trajectory · obligations ↔ work items · moves/evidence
↔ updates · theory ↔ assertion ledger
(`docs/conjecture-assertion-ledger-2026-06.md`) · decided ↔ settlement · one
activation = one or more circuit passes. The skill is the cognitive layer of
the durable-actor protocol; as M1+ records land, the mission document keys to
them instead of standing alone.

## /goal Usage

```text
Use Parallax on docs/<mission>.md. Treat the mission document as the single
source program and handoff: read it and its required references, compile or
update a compact Parallax State section in place, then run the circuit. Each
pass states position/blind spot, chooses probe / shift / construct / settle,
bounds mutation, records what changed, and exits only at settled,
open_handoff, blocked, or superseded. Platform behavior settlement requires
repo landing proof in the same document. No claim outruns its evidence class;
no self-checked proofs; no fake islands.
```

Parallax is candidate state. Promote it only if real missions show the
conjecture bridge changing the route, observer shifts narrowing scope, and
handoffs becoming easier to resume.
