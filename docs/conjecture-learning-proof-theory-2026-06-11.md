# Conjecture Learning as Proof Theory — 2026-06-11

## Status

Comprehensive treatment of conjecture learning in the proof-theoretic
framing, then the promotion system reviewed through that lens. This is the
document where the conjecture system (handoff of 2026-06-10) and the formal
verification program (specs/, June 2027 target) become one story.

The framing is a metaphor that is engineered to stop being one: every
correspondence below is either already mechanical (TLC proofs, verifier
contracts) or has a named path to becoming mechanical.

---

# Part I — The theory

## 1. The dictionary, properly grounded

A **proof system** is: a language (what can be said), axioms (what is
assumed), inference rules (what steps are valid), and — for our purposes —
a budget and a set of oracles (tools, data access). Choir's word for a proof
system is an **observer**. This is the load-bearing identification:

> **An observer is a proof system. Its reach is what it can derive.
> Its hyperthesis is its incompleteness.**

| Choir object | Proof-theoretic reading | Already mechanical? |
|---|---|---|
| observation | ground fact / evidence term | yes (Trace, artifacts) |
| hypothesis | open proposition φ | prose today |
| test | proof search for φ or ¬φ under observer O | partially (verifiers, TLC) |
| conjecture | the sequent-under-scrutiny: O ⊢? φ, with declared edge and scope | record schema exists |
| hyperthesis edge | the region where O cannot decide φ | named per-conjecture |
| ΔO (observer upgrade) | proof-system extension: new axiom, rule, oracle, budget, or vocabulary | concrete (new verifier, new instrumentation) |
| assertion | theorem: O ⊢ φ on domain D, with proof object attached | receipts = proof objects |
| verifier | proof **checker** (not prover) | yes |
| invariant | theorem admitted into the ambient theory | yes (specs, contracts) |
| promotion gate | the admission rule: nothing enters the theory unproven | partially (this is Part II) |
| heresy | an inconsistency in circulation: a usable statement not derivable from (or contradicting) the theory | doc sweeps |
| compaction | lemma caching: replace a derivation with its conclusion + reference | yes (run-memory compaction) |
| trajectory settlement | goal closure: proved, refuted, or withdrawn | spec'd (wire_pipeline) |
| open obligations | open subgoals in the proof tree | spec'd (settlement soundness) |
| cross-level invalidation | truth maintenance: retract an axiom, and every theorem whose proof used it reverts to a conjecture | designed, not built |

Four of these deserve full sections, because they are where the framing does
real work rather than decoration.

## 2. Hyperthesis is incompleteness, and that changes what it demands

Popper's falsifiability asks: *what observation would refute this?* It
silently assumes the observer is adequate to make that observation.
Hyperthesis asks the prior question: **is this observer capable of being
wrong here?** — which is precisely the incompleteness question. A
proposition can be undecidable *for a given proof system* in four distinct
ways, and naming which one is what makes an edge actionable:

1. **Independence** — the claim is outside what the axioms settle (the
   observer's assumptions simply don't bear on it). Fix: new axiom = a new
   trusted evidence source, adopted explicitly.
2. **Resource bound** — a proof exists but exceeds the budget (the
   exhaustive check is too expensive; the candidate world too costly to
   build). Fix: more budget, or a smaller claim.
3. **Missing oracle** — the proof needs a fact the observer has no access
   to (no instrumentation, no permission, no tool). Fix: ΔO adds the
   oracle — this is most "observer upgrades" in practice.
4. **Frame lock** — the refutation cannot even be *stated* in the
   observer's language; the vocabulary lacks the predicate. This is the
   most dangerous class because evidence gets reinterpreted to fit the
   expressible. Fix: extend the signature — new vocabulary is a formal
   act, not a writing-style choice. (The parent/child→trajectory rename
   was exactly this: the old language could not express "coagent on a
   channel," so the system literally could not represent its own bug.)

Most systems run with **null hyperthesis** — they assert confidence without
naming their incompleteness class. The discipline "every conjecture names
its edge" is, formally: every sequent carries a declared bound on the proof
system's competence over it.

## 3. Scope is bounded quantification, and overclaiming has a formal name

An assertion is never φ; it is **∀x ∈ D. φ(x)** for an explicit domain D —
the domain the proof actually covers. The reach of the evidence *is* D.

This gives precise names to the two cardinal sins:

- **Overclaiming** = asserting ∀x∈D′ with D′ ⊃ D — quantifying beyond the
  proof. "Tests pass, therefore the code is correct" is the canonical case:
  tests are **existential** evidence (∃ executions that behave), silently
  promoted to a **universal** claim (∀ executions behave).
- **Heresy** = keeping an assertion in circulation after its proof died —
  using a theorem whose axioms were retracted. By *ex falso quodlibet*, one
  tolerated contradiction licenses anything downstream; this is why heresy
  sweeps are consistency maintenance, not housekeeping.

Artifact systems add a third failure mode: **representation substitution**. A
proof of an artifact invariant must preserve the object whose invariant is being
proved. A lookalike representation is not the same theorem. For source-backed
Texture work, a clickable URL, markdown web link, footnote, or visible source
inventory does not prove source/citation behavior. The theorem is about durable
source entities and transclusion points. Replacing those with link-shaped prose
changes the witness and must be counted as a falsifier, not partial support.

Each evidence class is a proof system with a characteristic reach, and the
whole verification program is just refusing to confuse them:

| Evidence class | Quantifier shape | Reach |
|---|---|---|
| model checking (TLC) | ∀ — exhaustive | all states **of the model**; transfers to code only via conformance |
| code-level proof | ∀ — over the code itself | the few hundred lines you can afford it for |
| property-based tests | probabilistic ∀ | sampled domain, stated distribution |
| example tests | ∃ | exactly the executions run |
| verifier contract | decidable predicate | exactly the artifact checked |
| human review | social | the reviewer's attention and competence |

## 4. Untrusted provers, trusted checkers (the agentic keystone)

Proof theory's deepest gift to agentic systems is the **De Bruijn
criterion**: the trusted kernel that *checks* proofs must be small; the
machinery that *finds* proofs may be arbitrarily large, wild, and untrusted
— because a found proof is checked, not believed.

Map it directly: **LLMs are untrusted provers.** An agent's output — code,
a claim, a plan, a "done" — is a *candidate proof*, never a theorem. The
harness's verifiers are the kernel. Nothing an agent says is load-bearing
until a checker (type system, test, contract, TLC run, owner) has accepted
the certificate. This single principle is:

- why workers produce candidates and verifiers produce evidence;
- why a worker must never verify itself (a proof checked by its own prover
  is not checked);
- why the June 2027 boundary is "verified harness, gated unverified
  cognition" — we verify the kernel, not the prover;
- why capsules matter: proof *search* must be sandboxed so its side effects
  cannot leak into the theory while the search runs (a prover that can
  rewrite the axioms mid-search proves anything).

## 5. The fixed point is Gödelian, and stratification is not optional

A sufficiently strong system cannot prove its own consistency. The
conjecture handoff's anti-pattern "self-rewrite without gates" is this
theorem wearing work clothes: a system that certifies improvements to its
own proof discipline *using only that discipline* has proved nothing.

So the self-improvement recursion must be **stratified** — and the three
ledgers of the grand synthesis are exactly the strata:

- **Object level** (Level 1): prove claims about artifacts, inside the
  current proof system.
- **Meta level** (Level 2): prove claims about the *system* — in a candidate
  world, checked by verifiers that the change being evaluated cannot touch,
  admitted by a promotion gate outside the change's reach.
- **Meta-meta level** (Level 3): prove claims about the *method* — rarest,
  maximally gated, and always adjudicated by evidence from levels below
  (did action/verifier/scope/stopping-condition actually change?).

Promotion gates on self-changes are not process conservatism; they are the
structural answer to Gödel: the admission rule for level-N changes lives at
level N+1. "The ecology proposes; the gates dispose" is a reflection
principle.

## 6. Truth maintenance: assertions die when their axioms do

An assertion's proof depends on premises: verifier versions, base states,
axioms-in-force. **Cross-level invalidation** is dependency-tracked belief
revision (a truth-maintenance system): retracting a premise reverts every
dependent theorem to conjecture status — visibly, queryably, not by manual
audit. `invalidation_triggers` on AssertionRecords are TMS justifications.
The freshness rule in Part II is the same machinery at the single-promotion
scale: a proof constructed under premise "base = S" dies when the base
moves.

## 7. The loop, restated

Conjecture learning is **proof search over a living theory**:

```
conjecture     pose the sequent, declare its edge and scope
search         agents (untrusted provers) attempt proof or refutation
check          verifiers (trusted kernel) accept or reject certificates
admit          promotion adds the theorem, scoped, with its proof object
maintain       invalidation retracts theorems whose premises die
extend         ΔO grows the proof system where edges blocked progress
```

A run is learning when conjectures sharpen, edges shrink *or become
explicitly accepted*, theorems gain proof objects, and the theory stays
consistent. The system is mature (the fixed point) when this loop can be
applied to the loop itself — under stratified gates, never reflexively.

---

# Part II — The promotion system under this lens

## 8. The naive goal is undecidable, by theorem

What we want to prove: *"this mutation is safe."* What Rice's theorem says:
every nontrivial semantic property of programs is undecidable. "Safe," "does
what the user intended," "won't misbehave in production" are nontrivial
semantic properties. **There is no general proof, and there never will be.**

The unsophisticated responses are both wrong:
- *Prove nothing, ship on vibes* — null hyperthesis, the current
  `PromoteAppAdoption` (fires from `verified`, no approval, no freshness).
- *Demand the impossible proof* — paralysis, or worse, decorative
  ceremony that pretends to be the impossible proof.

The proof-theoretic response is the **third move**: *change the theorem.*
Don't prove "the mutation is safe." Define a conjunction of claims that are
each decidable by construction, prove those, and carry the difference —
between proven-conjunction and true-safety — as a **named hyperthesis that
the owner explicitly accepts**. The constraint on the proof is not a
limitation we suffer; it is the design act that makes proof possible.

## 9. The safety theorem, decomposed into its provable strata

**"Safe to promote" is hereby defined as the conjunction:**

```
S1  ProtocolSafe        ∧    (universal, machine-checked)
S2  InstanceCompliant   ∧    (decidable record check)
S3  ContractsPassed     ∧    (bounded semantic checks, scoped)
S4  Revertible          ∧    (the substitute theorem, windowed)
S5  ResidualAccepted         (the owner discharges the intent obligation)
```

Each stratum with its proof system, reach, and edge:

### S1 — ProtocolSafe: the mechanics preserve invariants

*Claim:* the promotion machinery itself can never produce torn state, stale
commits, unapproved visibility, or unsafe reverts.
*Proof system:* TLC over `specs/promotion_protocol.tla` (∀ over the model),
plus conformance tests binding the Go to the spec.
*Why decidable:* **we made it finite-state on purpose.** The protocol is
small (one commit point, four ledger states, seven commit states) because
designing for decidability is the first constraint on the proof. A protocol
too rich to model-check is too rich to trust.
*Edge:* the model/code gap — bounded by conformance tests now, trace
validation later. Scope: the harness, all promotions, forever (until the
spec changes, which re-proves).

### S2 — InstanceCompliant: this promotion followed the protocol

*Claim:* prepared against the current base (freshness CAS), all ledgers
prepared, verified, approved, rollback refs recorded.
*Proof system:* record inspection — a decidable predicate over durable
state. This is type-checking the transaction.
*Edge:* none worth naming — this stratum is exactly as strong as the
records are honest, which is S1's job to guarantee.

### S3 — ContractsPassed: bounded semantic checks

*Claim:* build succeeds; tests pass; no cross-computer binary copying; no
secrets in payloads; N-1 schema compatibility; migration dry-run clean.
*Proof system:* verifier contracts — each a decidable check over the
candidate artifact, run by checkers independent of the authoring agent
(untrusted-prover discipline, §4).
*The critical scoping:* these are **existential and syntactic** evidence.
"Tests pass" is ∃, not ∀. The assertion admitted is *"the candidate passed
contracts C1..Cn"* — never *"the candidate is correct."* Writing the
stronger sentence anywhere (UI, docs, prompts) is overclaiming in the §3
sense, and it is how users get betrayed politely.
*Edge:* everything the contracts don't check — which is most of semantics.
This edge is not shrunk at this stratum; it is *handed* to S4 and S5.

### S4 — Revertible: the substitute theorem

This is the keystone of the whole design. **When you cannot prove a change
is good, prove instead that observing it to be bad is recoverable:**

```
Cannot prove:   Good(change)                      (Rice)
Can prove:      Observed(¬Good) → Restore(pre-state)   — within the window
```

*Proof system:* the protocol spec again — consistent-cut rollback
(NoTornOutcome), restore-point existence (rollback refs as a precondition
of commit, already enforced in code today), RevertSafety.
*The window is the validity domain of the substitute.* The theorem holds
exactly until the first N-1-incompatible write (the poisoned write) —
after which `Restore(pre-state)` is itself unprovable and the substitute
dies. Hence, formally derived design rules:
- **Reversible and irreversible changes are different theorem classes**
  and must not share a promotion. Contract-phase (destructive) changes go
  in a separate, later promotion — not as policy preference, but because
  bundling them voids S4 for the whole bundle.
- The rollback window is **explicit durable state**, because it is the
  domain of quantification of a live theorem — and the UI must show it,
  because the owner is accepting a different residual when it's closed.
- External side effects (emails sent, payments made, API calls) are
  **outside the restorable state space entirely** — no window ever covers
  them. They must be flagged at plan time as S5 material, never silently
  classed as revertible.

### S5 — ResidualAccepted: the obligation only the owner can discharge

After S1–S4, the residual hyperthesis is precisely characterizable:

```
residual = semantic intent       (does it do what I actually wanted?)
         + emergent behavior      (what happens under real use?)
         + adversarial semantics  (passes contracts, means harm)
         + the irreversible set   (effects outside every window)
```

For the first item there is a unique sound oracle: **the owner.** "Does
this match my intent" is not approximable by any verifier, because intent
lives with the owner. Therefore the approval gate is not bureaucracy — it
is **the discharge of a proof obligation that no other component can
discharge.** The spec's `ApprovalGate` invariant, read proof-theoretically:
*no theorem touching user intent is admitted without the owner's signature
on that obligation.*

Three corollaries with teeth:

1. **Approval must be informed to be a discharge.** Signing a sequent you
   cannot read discharges nothing. This is the formal argument for the
   changes-app review loop (headline, try-it preview, plan with destructive
   items flagged, window status): those are not UX niceties, they are the
   *legibility conditions of the proof step*. A rubber-stamp approve is
   decorative consent — S5 silently null.
2. **Approval is of a specific sequent, not a schema.** The owner approved
   *this* candidate against *this* base with *this* residual. When the
   base moves, the premise dies (§6), the proof dies, and the approval is
   void — which is exactly why `Restage` clears `approved` in the spec.
   Re-review after restage is not friction; it is re-proof.
3. **The second and third residual items get empirical, post-admission
   treatment:** the health window is an experiment run *after* commit,
   with auto-revert as the retraction rule — defeasible admission, kept
   honest by S4's still-valid substitute theorem. Adversarial semantics is
   bounded (not proven absent) by capability confinement: capsules and the
   authority lattice cap what a malicious-but-contract-passing change can
   reach. Bounding an edge is legitimate edge work; pretending it's closed
   is not.

## 10. Answering the question directly: what are the constraints on the proof?

**The constraints are the engineering.** Enumerated:

1. **Finite-state by construction.** The protocol is provable because it
   was designed small. Decidability is a design budget, spent before
   implementation. (S1)
2. **Bounded quantifiers everywhere.** Every claim names its domain; ∃ is
   never promoted to ∀; "passed contracts C1..Cn" is the strongest
   admissible sentence about candidate quality. (S3, §3)
3. **Reversibility substitutes for correctness — inside a window.** The
   central trade: we prove undo-ability where goodness is unprovable, and
   we treat the window as the explicit validity domain of that substitute.
   Irreversibility ejects a change from this regime into S5. (S4)
4. **Untrusted provers, trusted checkers.** No agent output is
   load-bearing unchecked; no prover verifies itself; the kernel
   (verifiers, spec, gates) stays small enough to trust. (§4)
5. **Intent obligations route to the only sound oracle.** The owner's
   approval is a proof step, with legibility as its precondition and
   premise-freshness as its validity condition. (S5)
6. **The residual is named, bounded, and accepted — never dropped.** The
   hyperthesis of a promotion is a first-class object the owner sees.
   Identity is accepted residual risk; an approved promotion is the owner
   choosing an edge, knowingly. (S5, §2)
7. **Self-changes are stratified.** A promotion that changes the promotion
   system is a Level-2/3 event: proved in a candidate world, checked by
   verifiers it cannot touch, admitted by a gate outside its own reach.
   (§5)

## 11. What this changes in practice (delta to the promotion program)

Mostly the promotion conjecture doc survives intact — this lens *explains*
it rather than amends it. The genuine deltas:

1. **The plan view gets a proof structure.** The changes app's plan should
   render the S1–S5 conjunction explicitly: what is machine-proven, what
   was check-passed (named contracts), what is revertible and until when,
   and what the owner is being asked to accept. The approve button is the
   S5 signature; it should look like one.
2. **Claim language is audited as overclaiming.** "Verified" in any surface
   means S3 — and must never render as "correct" or "safe." A wording
   sweep is a §3 consistency obligation, same class as the heresy sweep.
3. **Irreversibility classification is a plan-time type, not a rollback-time
   discovery.** Every effect in a change package is classed
   revertible-in-window / revertible-never / external — because S4's
   theorem needs its domain stated to be a theorem at all.
4. **ConjectureRecord v0 gains the quantifier fields.** `scope` should
   carry the domain D explicitly; `edge.boundary_type` maps to the four
   incompleteness classes of §2 (independence / resource / missing-oracle
   / frame-lock). Cheap change, sharper records.
5. **The 2027 verification target gets its honest decomposition.**
   "Formally verify everything" means: S1-class proofs for the harness
   protocols (actors, promotion, wire, capabilities), S3-class contract
   coverage as broad as affordable, S4 substitutes everywhere reversible,
   S5 legibility for everything else — and *no claim anywhere that
   quantifies past its proof.*

---

## 12. Compressed form

```
An observer is a proof system; its hyperthesis is its incompleteness.
A claim's scope is its quantifier domain; overclaiming is ∃ sold as ∀.
Agents are untrusted provers; the harness is the trusted checker.
Self-improvement is stratified or it is nothing (Gödel).

Safety of a mutation is undecidable — so change the theorem:
  prove the protocol        (finite by design),
  prove the compliance      (a record check),
  prove the contracts       (bounded, ∃, scoped),
  prove the undo            (while the window holds),
  and hand the named residual to the only oracle for intent.

We don't prove the change is good.
We prove that betting on it is bounded —
and we show the owner exactly what the bet is.
```
