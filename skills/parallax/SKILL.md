---
name: parallax
description: "Run a mission as a conjecture circuit: the mission document claims that completing an artifact/spec/objective will actually advance a deeper goal, then tests and constructs that claim through observer shifts, descending a declared variant under an explicit budget. Use for any nontrivial /goal mission where the route is uncertain, the evidence may mislead, or the work must hand off cleanly."
version: 1.3.2
metadata:
  hermes:
    tags: [parallax, conjecture-learning, proof-search, long-running-agents]
    related_skills: [cognitive-transform-portfolio]
---

# Parallax

Parallax is the **conjecture circuit** for missions. A mission document claims
that completing some artifact, following some spec, or achieving some
objective will actually advance a deeper goal. Work is proof search over that
claim. The artifact is the witness; observer shifts keep the mission from
confusing a local proxy with real success; the variant keeps the search from
mistaking motion for descent.

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
- **V** — the variant (ranking function): see below. Never decrease V by
  weakening I or faking D.

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

## The Variant (Ranking Function)

Invariants prove a loop safe; a **variant** proves it terminates. Every
mission document must declare one: a concrete, well-founded measure of
remaining distance to settlement that each productive pass strictly
decreases. Counts beat adjectives — count the things that must reach zero:

```text
V = open obligations without a typed record
  + control reads the route-switch must delete
  + domain rungs remaining to the acceptance target
  + driving conjectures still undecided
```

The variant is stated per mission, in the mission document, in the mission's
own vocabulary. The skill requires only that it exists, that move selection
names the expected decrease (ΔV), and that the update step records the
actual decrease against prediction.

The variant is the move-selection criterion: among admissible moves, prefer
the largest expected ΔV per unit budget — not the smallest honest step.
Smallest-step descent is how a mission spends thirty passes approaching a
gate it never reaches. "The gap is narrower" is not a measurement; ΔV is.

A pass that decreases no variant and buys no observer evidence changed
nothing — that is the forcing rule's trigger, now typed.

## Budget

Declare the budget at mission start in the mission document: passes, tokens,
wall-clock, or whatever the authority granted. Every pass performs the
solvency check: **estimated remaining descent of V versus remaining budget.**
If the descent does not fit the budget, the next move is not another
construct — it is a re-plan: bigger steps, a narrower claim, a domain
shrink, or an immediate handoff while the document is still resumable.

Running out of budget mid-pass is the one exit this skill forbids. Settlement
is earned; so is handoff. The rate limiter must never be the terminator.

## Parallax Mission Documents

The canonical short name for documents created by or compiled for this skill
is **paradoc**: a Parallax mission document. Use paths like:

```text
docs/mission-<short-name>-vN.md
docs/mission-<short-name>-vN.ledger.md
```

When a document pre-exists Parallax, do not rename it just to satisfy the
format. Compile it in place and call it a paradoc once it contains a
`Parallax State` section. The companion ledger is the Parallax mission
ledger.

Every new paradoc, and every materially re-scoped paradoc, must include a
copy-pasteable **Suggested Goal String** section. Put it outside `Parallax
State` so state stays compact. The goal string should be enough for a fresh
agent to resume the mission: path, source-program instruction, current status,
variant/budget, authority bounds, protected invariants, first next move, ledger
path, and settlement rule. When the user asks for a paradoc, return the same
goal string in the final response as a fenced text block in addition to writing
it into the document.

## The Mission Document

A mission document may begin as research, architecture, a spec, an objective,
or an initial conjecture set. Compile those source forms just-in-time into
the mission conjecture. Preserve the author's text; add or update a compact
**Parallax State** section rather than rewriting the document into a template.

**State, not log.** The Parallax State section is **rewritten in place**
every pass — it holds the current position, live conjectures, open edges,
variant value, and next move, and nothing else. It must answer "where is
this mission now?" in one read, and it is the only section a resuming pass
must re-read. Hard cap: ~1,500 words; when it exceeds the cap, compact it
before the next move. Narrowing is expressed by rewriting the position, not
by appending a narration of the rewrite. Write each fact once: position,
blind spots, and open questions are one current picture, not three parallel
histories.

Move history goes to a companion ledger file:

```text
docs/<mission>.ledger.md     append-only; written every pass, never re-read
                             in full — consult it only when auditing or
                             when the state section has lost a thread
```

At mission start: read the document and required references; extract
objective, artifact, invariants, qualities, domain/acceptance target,
variant, budget, authority, initial conjectures, blind edges, and
obligations; classify mutation class; name protected surfaces touched; define
the evidence packet; and record expected heresy delta (`discovered`,
`introduced`, `repaired`). Infer conservatively when safe; ask only when
artifact identity, authority, or safety is ambiguous. Then execute from the
compiled state and update it after moves that change conjecture status,
position, scope, verifier, artifact state, or settlement.

```text
## Parallax State
status: working | settled | open_handoff | blocked | superseded
mission conjecture: if A satisfies S under I/Q over D, then G advances
deeper goal (G):
witness/spec (A/S):
invariants / qualities / domain ramp (I/Q/D):
variant (ranking function) V: definition; current value; last ΔV
budget: granted / spent / remaining; solvency verdict
authority / bounds:        (standing bounds stated once, not per pass)
mutation class / protected surfaces:
evidence packet:
heresy delta:
position / live conjectures / open edges:
next move:
ledger file: docs/<mission>.ledger.md
version / lineage:
learning state: retained here / promoted outward / successor links
settlement:
```

Suggested goal strings are handoff instructions, not proof. Keep them current
when the paradoc is split, superseded, narrowed, or converted from planning to
execution. Do not let an old goal string route a future agent around the current
Parallax State.

**Pointers, not mirrors.** When a mission splits, links to its successor or
predecessor are one line each: the successor's path, the resume condition,
nothing more. Never transcribe another mission's passes into this document.
Double-entry bookkeeping across a split doubles the cost of every pass and
proves nothing.

For behavior-changing Choir platform missions, this same mission document
must also carry the landing proof: commit, push, CI, deploy identity, staging
acceptance, rollback refs, and residual risks. Local proof is not settlement
for vmctl, candidate computers, gateway/model calls, promotion, rollback, or
Choir-in-Choir behavior.

The mission document is mutable and versioned. If a later mission completes
the outcome, do not abandon this one: mark it `superseded` or
`open_handoff`, link the successor, migrate live conjectures and open edges,
and leave this document in a learning-bearing state.

## The Circuit

One pass per control interval. Same circuit at every scale; only budgets
differ — and the budget is declared, checked, and spent visibly.

```text
1. CLAIM     What conjecture currently decides this mission?
2. POSITION  From where am I looking? State it: "from here I can see X
             cheaply; I cannot see Y at all." Name the edge class.
3. MOVE      one of four — and name the expected ΔV, or the observer
             evidence the move buys:
             probe      test a conjecture under the current observer
             shift      move the observer (see catalog below)
             construct  build or extend the witness
             settle     decide the conjecture, or accept-and-name the edge
4. BOUND     smallest substrate that can carry the move; stay inside the
             authority envelope; mutations reversible (candidate/capsule
             when risky; for canonical mutations, the S1–S5 decomposition
             in the proof-theory doc Part II). Batch when the route is
             unambiguous (see Batching).
5. UPDATE    Rewrite Parallax State in place; append one terse entry to the
             ledger file; record actual ΔV against expected. A move that
             changed nothing is evidence about the OBSERVER, not the world.
6. EXIT?     conjecture decided | superseded | edges accepted and named |
             obligation only another authority can discharge | budget
             insolvent → re-plan or hand off.
```

**Batching.** When the route ahead is unambiguous — a planned sequence of
bounded constructs whose shape is already decided — one pass may plan and
execute the whole batch: name the k constructs, the predicted total ΔV, and
the per-construct check, then run them back-to-back with focused
verification only. The tripwire ends the batch early: any surprise, any
deviation of actual evidence from predicted ΔV, returns to a full circuit
pass. Thirty deliberation cycles for ten foreseeable constructs is overhead,
not rigor.

**The forcing rule.** If the last two moves changed nothing — no ΔV, no new
observer evidence — the next move is a SHIFT. If shifts also produce
nothing, re-plan against the budget. Confirmation is what a stuck observer
produces. Probing harder from a fixed position cannot escape that position's
blind spot.

**The learning rule.** Repeated obstacles are evidence about the conjecture,
not just the route. When the same class of obstacle recurs, reconsider the
bridge `A satisfies S => G`: the witness may be wrong, the spec may be a
proxy, the domain may not embed, or the observer may lack the predicate that
would reveal the real goal. Update, weaken, split, or supersede the
conjecture before grinding further.

**The architectural-mode rule.** A move that changes Choir from agentic to
workflow, trajectory/work-item to run-tree, evidence contract to smoke proxy,
or promotion protocol to shortcut behavior requires an explicit conjecture
delta before construction. Do not let a probe precondition silently become
architecture.

**The evidence-packet rule.** Missions must leave a packet containing mutation
class, protected surfaces touched, claims and evidence class, tests/probes,
rollback refs or blocker, heresy delta (`discovered`, `introduced`,
`repaired`), conjecture delta, residual risks, and a short human-learning
digest. Discovery of a heresy is epistemic progress but not repair progress.

**The doctrine-touch rule.** When touching doctrine, operating contracts,
mission portfolio, prompts, or high-read architecture docs, reconcile framing
and sentiment as well as facts. Choir Doctrine is the apex: self-improving
mainframe, persistent computers, truth from facts, conjecture learning,
evidence-bounded claims, protected invariants, and deletion pressure.

Apply the H027-H029 guardrail while in doctrine-touch mode:

- `Trace` language in docs is evidence/topology only unless the section is
  explicitly historical and marked as such. `Trace app`, `Trace UI`, and
  `Open Trace` are not normal user surfaces and should not be framed as product
  surfaces.
- `raw Terminal`, `Terminal app`, and terminal-shim surface framing are invalid
  in user-facing doctrine except for explicit historical or implementation
  residue labels. Super Console/zot is the replacement repair surface.
- `Browser`/`Browser app`/`source-gathering` mentions are valid only when they
  resolve to Source Viewer/reader artifacts plus explicit Web Lens live/original
  inspection.

**The retention rule.** Every mission leaves its mission document and ledger
file as the durable learning artifacts, even when it fails or is superseded.
Promote learning outward only when it changes shared doctrine, assertions,
architecture, tests/specs, skills, or successor work. Do not let partial
missions vanish as chat memory.

**Move selection.** Among admissible moves, prefer the largest expected ΔV
per unit budget. Probes are cheap and usually right; shifts are undervalued
by default — which is why the circuit prices them explicitly every pass
instead of saving them for blockers.

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
  (a proof checked by its own prover is not checked). Run prover shifts,
  probes, and repo archaeology in **fresh disposable contexts** (subagents)
  that return conclusions, not transcripts: the mission context keeps the
  verdict, never the file dumps;
- **inversion** — stop seeking confirmation; try to refute. Treat uncertain
  claims as unsupported for promotion or settlement until checked.

When extra perspective is required, use
`cognitive-transform-portfolio/SKILL.md` as a shift amplifier. Select only
transforms that change the next probe, route, scope, verifier, or stopping
condition; otherwise they are commentary.

## Verification Tiers

Checks are tiered by scope, and each boundary pays for the tier it crosses:

- **in-construct** — focused tests on the touched surface; fast, narrow,
  existential.
- **batch boundary** — the full default suite of every touched package, plus
  the consolidation pass (below). Focused filters prove the branch; only the
  full suite proves the package.
- **handoff / settlement** — the widest checker the repo has: all build
  tags, vet, every touched package's full suite — and an **independent
  prover**: a fresh-context agent reviews the accumulated diff for bugs and
  accretion. The authoring context never grades its own work; a test written
  by the same hand that wrote the bug will bless the bug.

A claim's evidence class is capped by the widest tier actually run. "Focused
tests passed" is not "the suite is green" — never let the ledger imply
otherwise.

**Consolidation.** At every batch boundary, one quality pass over the code
landed since the last one: simplify, merge duplicate pathways, delete dead
code, fix names. Incremental constructs accrete — twice-evaluated
predicates, copy-pasted fixtures, three same-shaped branches that should be
one rule. A construct is not complete until consolidated; consolidation debt
is variant, not optional polish.

## The Ledger

Each pass appends one entry to the ledger file. Terse schema — claim with
its scope, move, expected vs actual ΔV, receipt (command, artifact path,
trace ref), the edge it leaves open. Standing bounds live once in Parallax
State; do not restate them per entry. Three rules bind everything:

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
  carries conjecture states, position, variant value, budget state, and next
  move. Never call this settled.
- `blocked` — an obligation only another authority can discharge. Name the
  obligation, the authority, and the smallest discharge.
- `superseded` — a better conjecture or successor mission now carries the
  work. Link it, migrate live obligations, and retain the learning state
  before stopping.

Every exit requires the handoff tier of verification: widest checker plus
independent prover over the accumulated diff. No handoff on focused tests
alone.

## Via Negativa

- **No new nouns:** mission conjecture ↔ trajectory; obligations ↔ work
  items; moves and evidence ↔ updates; theory ↔ assertion ledger.
- **No cathedral:** the 10-minute circuit is the overnight circuit.
- **No log-shaped state:** Parallax State is rewritten, never appended;
  history lives in the ledger file, which is written, not re-read.
- **No double-entry:** cross-mission links are one-line pointers, never
  transcriptions.
- **No paperwork as progress:** if the circuit changes no decision, say so.
- **No descent-free passes:** a pass with no ΔV and no observer evidence is
  a stuck observer; force the shift.
- **No smallest-step default:** the bound limits blast radius, not
  ambition; pick moves by expected ΔV per budget, batch the foreseeable.
- **No fixed-position grinding:** repeated confirmation is a stuck observer.
- **No obstacle grinding:** repeated obstacles force conjecture revision.
- **No abandoned missions:** close with settlement, handoff, blocker, or
  supersession; retain learning state first.
- **No fake islands:** every domain embeds in production's.
- **No self-checked proofs.**
- **No unconsolidated handoffs:** accreted duplication is open variant.
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
update a compact Parallax State section in place (state, not log; move
history appends to docs/<mission>.ledger.md), declare the variant
(ranking function) and budget, then run the circuit. Each pass states
position/blind spot, chooses probe / shift / construct / settle by expected
ΔV per budget, bounds mutation, batches unambiguous construct sequences with
a deviation tripwire, records actual ΔV, and checks budget solvency. Full
suite + consolidation at batch boundaries; widest checker + independent
prover before any exit. Exit only at settled, open_handoff, blocked, or
superseded. Platform behavior settlement requires repo landing proof in the
same document. No claim outruns its evidence class; no self-checked proofs;
no fake islands; no descent-free passes.
```

When authoring a new paradoc, include a mission-specific version of that goal
string in the paradoc's `Suggested Goal String` section and repeat it in the
final response for the owner to copy and paste.

Parallax is candidate state. Promote it only if real missions show the
conjecture bridge changing the route, observer shifts narrowing scope, the
variant shortening runs, and handoffs becoming easier to resume.
