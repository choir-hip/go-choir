# Parallax: Design — 2026-06-11

## Status

Design record for the Parallax skill (`skills/parallax/SKILL.md`) — the
proof-theoretic conjecture-learning circuit that supersedes MissionGradient
for new missions. Named, designed, and constrained by the post-mortem of two
prior attempts. Parallax is itself candidate state: its promotion follows the
adoption gate in §6.

## 1. Why a new skill instead of rewriting MissionGradient

MissionGradient (now v2.0.0, ~750 lines) is a plan-control artifact that
accumulated anti-failure armor over months, with the conjecture material
added on top — producing redundancy (belief state vs conjecture ledger,
evidence ledger vs assertions, blocker taxonomy vs hyperthesis edges) rather
than clarity. It was the precursor: the thing we needed before the conjecture
frame existed. Rewriting it in place would churn every prompt and mission
that references it. Instead: **MissionGradient is frozen** (no further
development; existing references keep working), and Parallax is the pure
successor for new work.

What makes a mission succeed is not only whether the next step is +EV. It is
**how you shift your theory of the case, your positionality, and therefore
your blind spot**. In proof terms: the critical moves in proof search are not
inference steps but changes of proof system (ΔO). That is the organizing
insight Parallax is built around, and the reason for its name: parallax is
the method of measuring what no single vantage can — truth revealed by
displacement of the observer.

## 2. The Campaign Compiler harvest (failure modes → design constraints)

`docs/mission-campaign-compiler-selfdev-v0.md` (2026-05-29) was the prior
attempt to transcend MissionGradient. It died at `ready_for_execution`,
having shipped nothing. The harvest:

| Failure mode | What happened | Parallax constraint |
|---|---|---|
| Cathedral before chapel | ~900 lines specifying the mature system; the first executable step was too big to take | the circuit is identical at 10-minute scale and overnight scale; only budgets differ |
| Noun proliferation | 8 new record types as the deliverable; a schema instead of a loop | **zero new nouns** — the runtime objects are trajectory, work item, update, assertion (they exist) |
| Paperwork as product | violated its own via-negativa ("mission paperwork but no evidence") predictably | the anti-decoration gate is the adoption criterion: first use must change a move |
| Layer conflation | tried to be cognition + control plane + product surface at once | Parallax is ONLY the cognitive circuit; the control plane is the actor runtime; UI comes later |
| Transforms as bolt-on | cognitive transforms decorated a plan-control frame with no epistemic core to operate on | the observer shift is a first-class move type weighed against probing on every pass |

Meta-lesson from both attempts: **a cognitive circuit must fit in working
memory** — the model's attention and the human's review. Parallax's size
budget is a hard constraint: under 200 lines.

## 3. The two unifications that make it small

**The artifact is the witness.** Every mission is the constructive proof of a
scoped claim: *there exists — and here it is — an artifact A satisfying
invariants I with qualities Q over domain D*. Building is proving (proof by
exhibition). This absorbs MissionGradient's "real artifact," value criterion,
and quality gradient into one clause structure: I(A) are the hard invariants,
Q(A) the quality clauses, D the scope.

**Homotopy is scope.** MissionGradient's λ coordinate IS the domain D of the
goal claim, grown continuously: λ 0→1 = D small→production. A "fake island"
is precisely a claim whose domain does not embed in production's. One concept
replaces a section, and the anti-detritus rule becomes checkable: *does this
simplified domain embed?*

## 4. The circuit

```text
1. CASE      what do I believe decides this mission? (active conjectures)
2. POSITION  from where am I looking — and what can this position not see?
3. MOVE      probe | shift | construct | settle
             forcing rule: two null moves, or evidence that agrees too
             easily, forces a SHIFT — confirmation is what a stuck
             observer produces
4. BOUND     smallest substrate, authority envelope, reversible
5. UPDATE    what did this move change? (nothing → evidence about the observer)
6. EXIT?     case decided | edge accepted | obligation handed off
```

The distinctive beat vs MissionGradient: POSITION is a per-pass statement,
and SHIFT competes with PROBE on every pass — not a remedy applied before
giving up. MissionGradient's "apply 2–5 cognitive transforms before stopping"
made observer movement a blocker ritual; Parallax makes it the move most
likely to be undervalued and therefore explicitly priced each pass.

The shift catalog (instruments of displacement): change instrument (new
test/trace/log), change vantage (read as user/attacker/maintainer; different
layer of the stack), change vocabulary (new predicate — the frame_lock fix),
change domain (shrink D until the claim is decidable), change prover
(independent agent or checker), invert (seek refutation instead of
confirmation).

## 5. Runtime mapping (the transfer path into Choir)

Campaign Compiler invented its runtime nouns; Parallax inherits ones that
already exist from the rearchitecture:

| Parallax object | Runtime object |
|---|---|
| the case | trajectory (+ its conjecture state, M1 records) |
| obligations | work items |
| moves and evidence | updates |
| the theory of the case | assertion ledger |
| case decided | settlement |
| one circuit pass (or several) | one activation |

The role-free actor protocol already prompts actors with obligation +
conjecture + edge + authority — **an activation natively is a circuit pass**:
wake → read case → state position → move → update → settle or passivate.

Staging: v1 — markdown discipline invoked via `/goal` and in-session (now).
v2 — the case file keys to M1's trajectory/work-item records (the ledger
becomes durable runtime state). v3 — native: the circuit is what actors do;
the skill text becomes the actor prompt's cognitive core. No stage adds
nouns.

## 6. Adoption gate (Parallax applied to Parallax)

Parallax is candidate state. Promotion criteria (the §15 shape):

- first real mission (M1, which now runs under Parallax rather than
  MissionGradient v2) shows at least one SHIFT that changed the route, a
  probe-vs-shift decision made explicitly, and a claim whose scope was
  narrowed by an edge;
- the case file was cheaper to resume than a MissionGradient doc;
- failure criterion: the circuit fields were filled but no move differed
  from what MissionGradient would have produced — then Parallax is
  decoration and gets demoted, honestly.

MissionGradient v2 remains the comparison baseline and the fallback.

## 7. Name

**Parallax**: displacement of the observer as the measurement method.
One word, speakable, STT-safe (no homophone collision — the
hyperthesis/hypothesis lesson), verb-able ("we've probed three times — time
to parallax"). Considered and declined: Casework (generic), Tack (cute),
Conjecture Circuit (boring). Recorded per the codesign rule: vocabulary is
candidate state; the name is promoted with the skill.
