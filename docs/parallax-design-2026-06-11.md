# Parallax Design — Conjecture Circuit — 2026-06-11

## Status

Design record for `skills/parallax/SKILL.md`. Parallax is the mission
discipline succeeding MissionGradient for new work. Its literal shape is a
**conjecture circuit**: compile a mission document into a live conjecture,
then alternate construction, probing, observer shifts, and settlement.

MissionGradient remains a baseline/fallback for old mission docs. Parallax is
the active mission discipline for new broad work; its first durable-actor
missions are the live adoption evidence.

## 1. Mission as conjecture

The mission document is not just a plan. It is a conjecture:

```text
If witness A satisfies spec/objective S under invariants I and quality Q
over domain D, then deeper goal G is achieved or materially advanced.
```

This is the missing bridge in ordinary planning. A normal plan assumes that
finishing the stated objective means success. Parallax makes that assumption
testable. The mission can complete the spec and still fail if the bridge from
`A satisfies S` to `G advances` is false.

Terms:

- **G** — the deeper goal; why this mission matters.
- **A/S** — the witness and spec/objective; what the agent will build, prove,
  fix, decide, or document.
- **I/Q** — hard invariants and quality clauses.
- **D** — the scope/domain over which the claim may be asserted.
- **EDGE / DELTA_O** — what the current observer cannot see, and what
  observer upgrade would shrink that blind spot.

## 2. `/goal docs/<mission>.md`

The user interface should be:

```text
/goal docs/<mission>.md
```

The mission document is the source program and the handoff. It may begin as
research, architecture, a spec, an objective, a failure trace, a conjecture
set, or a mixture. Parallax compiles it just-in-time into a compact
`Parallax State` section in the same document:

```text
status
mission conjecture
deeper goal
witness/spec
invariants / qualities / domain ramp
authority / bounds
bridge conjecture + sub-conjectures / position
ledger / move log
version / lineage
learning state: retained here / promoted outward / successor links
settlement
```

Preserve the author's source text. Add only the missing executable state.
When the document already contains a field in prose, extract it rather than
duplicating it. Ask only when artifact identity, authority, or safety is
ambiguous.

For Choir platform behavior changes, the same document carries the landing
proof required by `AGENTS.md`: commit, push, CI, deploy identity, staging
acceptance, rollback refs, and residual risks. Parallax can choose the route
just-in-time; it cannot lower the evidence bar.

The mission document is mutable and versioned. A mission may change its
conjecture, narrow its domain, split its witness, or hand work to a successor.
That is learning, not failure, if lineage is preserved. A successor mission
must link the predecessor and migrate live conjectures, open edges, and
remaining obligations. The predecessor closes as `open_handoff`,
`superseded`, `blocked`, or `settled` — never as silent abandonment.

## 3. Circuit

Each control interval runs the same circuit:

```text
CLAIM     what conjecture currently decides the mission?
POSITION  what can this observer see, and what can it not see?
MOVE      probe | shift | construct | settle
BOUND     smallest safe substrate and authority envelope
UPDATE    what changed: route, scope, verifier, artifact, settlement?
EXIT      settled | open_handoff | blocked | superseded
```

The distinctive move is **shift**: move the observer instead of grinding from
one vantage. Shift forms: instrument, vantage, vocabulary, domain, prover,
inversion. The forcing rule remains: if the last two moves changed nothing,
or evidence agrees too easily, the next move is a shift.

The conjecture-learning rule is stronger: repeated obstacles are evidence
against the conjecture or its observer, not merely signs that the route is
hard. When the same obstacle class recurs, the agent must reconsider the
bridge from `A satisfies S` to `G advances`: perhaps the witness is wrong,
the spec is a proxy, the domain does not embed, or the observer lacks the
predicate that would expose the real goal. The next move should update,
weaken, split, or supersede the conjecture before continuing.

The representation rule is stricter still: if the witness changes the product
object being proved, the mission has not partially succeeded. It has tested a
different conjecture. For source-backed Texture artifacts, ordinary clickable
URLs, markdown web links, source lists, and "Source:" prose are not equivalent
to source entities or transclusions. A proof that substitutes them must update
the conjecture or record a falsifier before construction continues.

The retention rule is equally important: every mission must leave its mission
document as the durable learning artifact on success, failure, handoff, or
supersession. Promote learning outward only when it changes shared doctrine,
assertions, architecture, tests/specs, skills, or successor work. Partial
work is allowed; lost learning is not.

When the circuit needs extra perspective, the cognitive-transform portfolio is
a shift amplifier, not a parallel method. A transform is admitted only if it
changes the next probe, route, scope, verifier, or stopping condition.

## 4. Runtime mapping

Parallax uses the durable-actor ontology rather than inventing runtime nouns:

| Parallax | Runtime |
|---|---|
| mission conjecture | trajectory |
| obligations | work items |
| moves and evidence | updates |
| theory | assertion ledger |
| decided mission | settlement |
| one or more circuit passes | activation |

v1 is markdown discipline invoked by `/goal docs/<mission>.md`. v2 keys the
mission document to trajectory/work-item records. v3 makes the circuit the
native actor prompt core.

## 5. Adoption status

Keep Parallax as the active discipline while real missions show:

- the bridge conjecture changed the route, not just the wording;
- a shift narrowed a claim's scope or upgraded the observer;
- partial or superseded missions preserved lineage and retained learning;
- the mission document was easier to resume and retrospect than a
  MissionGradient doc;
- no canonical mutation was admitted without scoped evidence and rollback or
  owner acceptance.

Revise it if fields get filled while moves stay unchanged.

## 6. Name

**Conjecture Circuit** is the literal name. **Parallax** is the shorter
working name: observer displacement as the measurement method. A Parallax
mission document is a **paradoc**.
