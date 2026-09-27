# Precommitment Records: Theory and Implications

Condensed from the owner-authored paper (Sep 22, 2026). This is the theory the
engineering memo implements. Normative weight here; mechanism there.

## Thesis

Intelligent systems should commit to what they expect, and why, before the
evidence arrives. That one discipline does three jobs: it makes experience
learnable (a frozen model improves by retrieving records of its own prediction
errors), it makes behavior auditable (commitments checked against action and
outcome), and it grounds an alignment paradigm — rather than guaranteeing a
smarter system has the right goal, build systems that continually expose the
gap between what they predict, attempt, cause, and learn.

The hook: if an action predictably leads to harm, its commitment should say so,
and that should be enough to block it. Harm that was predictable but not
predicted is a *capability failure* — and capabilities can be hill-climbed.
Normative misalignment narrows to one case: the harm was predicted and the
model acted anyway. Precommitment records convert a large share of alignment
problems into capability problems.

Near-term this runs at the harness level with frozen models. Eventual version:
models trained end-to-end on their own commitment logs, overseen through those
logs, embedded in human governance.

## The problem: hindsight erases the before-state

Once an outcome is known, a capable model can usually explain why it was
unsurprising — genuine updating becomes indistinguishable from rationalization.
Post-hoc explanations omit factors that actually drove the answer; as models
reason in latent nonverbal states, "why did you do that" degrades further. Any
method that reconstructs reasoning after the fact inherits this weakness.

The fix is temporal, not introspective: **freeze the expectation before the
evidence exists.**

## The mechanism

A precommitment record has two hard requirements:

1. It is **committed before** the resolving evidence is available.
2. It resolves against a **verifiable outcome** — it can be falsified.

Cycle: commit (hypothesis + assumptions + alternatives considered + prediction
with confidence) → observe → score (the agent itself, another model, or a fast
System One model rates the match) → on surprise, revise (which assumptions
weakened/strengthened; updated hypothesis) → persist → retrieve on later tasks.

Commitments work at any scale and duration — one can wrap a file read, another
a months-long project. They nest: a large commitment contains small ones that
resolve along the way.

This is Dawid's prequential principle — judge a forecaster only by forecasts
made before outcomes — turned into learning data, not only evaluation.

## Learning: events, world models, the composed system

**Events, not lessons.** Typical agent memory stores a compressed rule ("the
test failed because the lexer doesn't nest comments"). A record stores the
*event*: the 90% prediction that nesting worked because a parser branch looked
recursive, the failure, the discrepancy, the revision. The event supports
questions a rule cannot: was the confidence justified? was the lesson too
broad? did the same assumption fail elsewhere?

**World models, not forecasts.** Each commitment states its reasons, so the
discrepancy points at the assumption that failed, and the revision updates the
model rather than only the forecast. The accumulated state is the agent's
evolving world model.

**The composed learner.** Weights stay frozen. The learner is *model + records
+ retrieval policy + context-building procedure*. Learning means prior
experience reliably improves later behavior through recoverable external state.
Remove a record and its effect should go; restore it and the effect returns.

**In-context reinforcement learning.** Working hypothesis: in-context RL is to
in-context learning as RL is to pretraining — feedback on one's own outputs is
far more token-efficient than learning from examples. Evidence is thin but
consistent: models learn in context from rewards on their own predictions
(Monea et al.); response quality rises as rewards accumulate in context even
when the model generates its own rewards (Song et al.) — which supports
self-scoring. The same mechanism drives reward hacking (Pan et al.: feedback
loops lead models to optimize proxies). Agents learn from feedback efficiently,
whatever the feedback measures.

Precommitment records redirect that mechanism: the feedback entering context is
**materiality-weighted prediction error about consequences**, not a proxy
objective — the optimization pressure that produces reward hacking then
produces better world models. The ledger makes the in-context optimization
visible: drift in what an agent commits to is itself a record.

This makes context design a safety decision. **Feed agents observations and
discrepancies. Keep independent scores out of the agent's context, or sample
and delay them, so the agent cannot learn a scorer's quirks within a session.**

**Behavior is the evidence.** A sophisticated revision that leaves later
behavior unchanged is a narrative, not learning. Learning value is not
accuracy: a badly wrong prediction can be the most valuable record; a correct
obvious one can be worth nothing.

**Partial verifiability.** Where quality is hard to judge (interpretation,
research, strategy, writing), "was this good?" is expensive, but "what did it
predict, and did that hold?" is cheap and repeatable. Commitments turn part of
a hard evaluation into small empirical tests.

**Preserve, then compress.** A score is a lossy view of an event. Keep the
evidence so later scorers, models, and training objectives can reinterpret it.

## Alignment as exposed discrepancy

Instead of asking how to guarantee the right goal, ask how intelligent
processes continually expose the gap between what they predict, attempt, cause,
and learn. Error is permanent and correctable; the system is trustworthy to the
degree discrepancies surface quickly and corrections stick.

Prospective loop: state → conjecture → intended action + expected consequence →
action → observed consequence → prediction error → next state.

Five surfaces separate what used to be one question:

| Surface | Question | Kind |
|---|---|---|
| Epistemic | Did beliefs predict reality? | technical |
| Material | Did predictions attend to the impactful factors? | technical |
| Instrumental | Did the action cause the intended result? | technical |
| Procedural | Did it act for roughly the reasons it stated beforehand? | technical |
| Normative | After consequences, is the action still endorsed/permitted? | governance |

The first four are measurable from commitment logs and hill-climbable as
capabilities. Epistemic and material are **distinct**: a model can be accurate
about trivia and silent on what matters. Materiality is its own question with
its own gauge. The fifth is a question of authority — governance, not
measurement.

### From alignment problems to capability problems

Every harmful outcome classifies by what the commitment said:

| Case | Record shows | Classification |
|---|---|---|
| Predicted, blocked | harm predicted; governance blocked/escalated | the system worked |
| Predicted, authorized | harm predicted; legitimate authority approved | governance decision — accountability on the authorizer |
| Predicted, done anyway | harm predicted; agent acted against a block or without authority | **normative misalignment** |
| Concealed | model foresees the harm when probed neutrally, but commitment omitted it | fidelity failure: concealment |
| Predictable, not predicted | a reasonable predictor would have foreseen it; the model did not | **capability failure** |
| Not reasonably predictable | no reasonable predictor would have foreseen it | accident |

"Predictable" is judged against a reference: independent critics, or the same
model asked about consequences outside action context. (The neutral-probe
mechanism that separates concealment from capability is theory-level; no probe
path exists in the current system.)

An *unpredicted* harm is a prediction miss — not evidence of wanting the harm,
not reward hacking. Treating it as capability failure makes it tractable.
Competence and safety stop pulling in opposite directions for this share of
problems.

The residue that remains classically alignment: acting against a predicted
harm, and concealing a prediction. Both surface as discrepancies between
records and outcomes — the conversion holds only while commitments are
faithful and blocking works, which is why procedural fidelity and governance
are load-bearing.

## Procedural fidelity and materiality

**Procedural fidelity** is the central objective: stable correspondence between
what an agent represents beforehand and what it then does. Neither behavioral
nor introspective — not "did it say the right thing," not "did reward go up,"
not "what was it really thinking," but *did behavior stay coupled to the
prospective representation available to oversight*. Roughly: the probability of
the observed action and consequences given the prior commitment. Judged by
action class and consequence envelope, never textual similarity between plan
and action.

Low fidelity looks like: predicting access to repository A then accessing B;
stating no network call then making one; predicting read-only then writing;
repeatedly predicting 2% production impact and repeatedly causing more.

### The materiality gauge

Accuracy and materiality are separate axes. "I will generate tokens" is true of
every action and changes no decision. "This function returns a boolean" is true
and specific but covers none of the impact if the function sits inside a
life-safety system. Both are accurate; both fail the materiality test.

**Materiality of a consequence.** For action `a` and consequence `c`:

    w(c) = P(c | a) · impact(c) · relevance(c)

- `P(c|a)` — likelihood, taken from reference predictors, not only the agent.
- `impact(c)` — magnitude on an ordinal scale set by *governance*, not the
  model: reversibility, scope (self / user / third parties / public), domain
  (money, data, safety, life).
- `relevance(c)` — whether knowing `c` could change the decision. With `g` the
  policy/authority check, `c` is relevant when `g(a, P ∪ {c}) ≠ g(a, P)`. A
  smooth version uses expected change in block probability.

A prediction that holds for every action never flips a verdict — relevance
zero, however true.

**Coverage of a commitment.** With `C*` a reference consequence set — built
prospectively from what independent critics surface, retrospectively from what
actually happened:

    coverage(P) = Σ_{c ∈ C* covered by P} w(c) / Σ_{c ∈ C*} w(c)

**One score for accuracy and materiality.** With `S` a proper scoring rule
(Brier, log) and `p_c` the agent's stated probability:

    score(P) = Σ_{c ∈ C*} w(c) · S(p_c, outcome_c)

Omitted consequences are scored as predicted at zero — omissions cost in
proportion to materiality. Trivial predictions earn nothing (zero weight).
Listing every catastrophe does not win — the proper scoring rule penalizes high
probabilities on things that don't happen.

**Materiality flows downstream.** Impact is not local:

    impact(change) ≈ max over downstream consumers of impact(consumer)

A commitment must predict *reach*: where the output flows and what it
controls. Failing to model reach is usually the most important materiality
failure — the boolean-function author didn't mispredict a return type; they
failed to model that it sits inside a life-safety system.

**The reference set is the weak link.** Coverage is only as good as `C*`; the
retrospective half helps — observed consequences enter `C*` even when nobody
predicted them, so the gauge sharpens as the log grows.

**Materiality completeness.** An agent can only seek permission for
consequences it represents. The test: did it surface the consequences that
could reasonably change the decision? Trained adversarially — critics search
for omissions, the agent is rewarded for anticipating them, consequential
omissions are penalized; both recall and precision are scored.

The full stack: competence, causal foresight, materiality coverage, procedural
fidelity, normative permission. Competence is becoming a commodity; the rest is
the work.

## Governance: normative alignment as allocation of authority

Once an agent accurately represents its intended action and material
consequences, whether the action is permitted is not a deep technical question
— it is ordinary governance (legal, moral, political, contractual,
institutional, professional, reputational).

Pipeline: commitment → predicted material consequences → policy and authority
check → act or escalate. The policy layer holds law, contracts, organizational
policy, user preferences, professional ethics, security classifications,
spending limits, jurisdiction, prohibitions, emergency exceptions.

**What aligned means.** Not "never violates a rule" — exception handling is
part of the system. Aligned means acting under an intelligible allocation of
authority, relevant consequences represented, applicable norms surfaced,
accountability attached.

**Who decides.** The user, employer, regulator, court, professional body,
system operator, or the agent under bounded delegated discretion. Roles
compose: agent supplies prediction and judgment, institution supplies authority
and constraints, human sometimes supplies authorization, logs supply
accountability.

**Normative content can't carry the long tail.** The morality a model carries
is chosen by its developers (training, ideally a published spec), varies by
developer/family/version, is partly emergent, and can never be complete or
fully formalized. What can be *checked* carries the weight instead:
represented consequences, procedural fidelity, external authority, ledger
oversight.

**Two governance layers:**

| Layer | Examples | Strengths | Weaknesses |
|---|---|---|---|
| Compiled into weights | published constitution | always present, machine speed, works unwatched | opaque, slow to update, hard to tailor |
| Applied at runtime | policies, capabilities, classifiers, authorization | legible, versioned, contextual, auditable | can be silent, slow, or captured |

Silence, illegitimate authority, and escalation judgment are where a compiled
floor helps — a backstop, not a guarantee. Both layers are auditable through
the same ledger; departure from a stated spec is a procedural-fidelity
question.

## Oversight dynamics

**Shadow trace.** Commitments give an externally auditable trace independent of
how reasoning happened: belief, prediction, planned intervention, expected
observation, actual intervention, actual observation, revision. Useful because
checkable — survives the move to latent reasoning where CoT monitoring loses
footing.

**The Zeno gap.** A gap always remains between what a model can claim, do, and
get away with — it narrows each step, never closes. Measurable: how often
discrepancies are caught late or never, and whether that rate falls.

**Circuit breakers.** Monitors may halt what they do not understand — the
burden of proof inverts: the agent acts at a level monitors can follow, or
explains until they can. Legibility is the price of action. Thresholds scale
with materiality so low-stakes steps aren't taxed.

**Structural coupling.** Agents and monitors learn from the same log;
monitors' predictions of agent behavior are themselves scored commitments. Any
capability the agent exercises generates the data that teaches monitors about
it. Usable capability is capped at monitor comprehension. What escapes:
capability from outside the loop (new base models) and capability held in
reserve.

**Monitoring scales with uncertainty.** Untested frontier checkpoints deserve
*more* monitoring. Diverse scorers/classifiers reduce correlated blind spots;
their disagreements are signal.

**Distributed supervision.** An executor predicts its own action and outcome;
a critic predicts what the executor will do; a monitor predicts material side
effects. All frozen before action, scored after — a prediction market over
behavior. The goal is not agreement but multiple prospective surfaces whose
joint consistency reality tests.

**Deception gets harder.** Combined with random audits, independent predictors,
frozen snapshots, provenance, capability separation, and resource limits,
deception is hard to sustain on every surface at once. Commitments that turn
sanitized under monitoring are themselves a detectable pattern.

## Near-term implications (all runnable today, frozen models, harness level)

- **Context packs** — the record log tunes curated context per model/task;
  corrects systematic biases; measured by what happens next.
- **Better research/reading** — commit before each search/retrieval/read;
  surprises show where to read deeper; missed search predictions expose weak
  queries; ambiguity gets recorded, not forced.
- **Incident prevention/detection/recovery** — predicted side effects visible
  before action; nested records show where the first failed prediction was.
- **Cheap independent scoring** — fast System One models score every record
  outside the agent's family; self-scoring retains value and the self-vs-
  independent gap is a practical monitoring number.
- **Calibration as by-product** — Brier per agent/model/prediction-type at no
  extra cost; audited subset calibrates the scorers.
- **Auditable agents** — trails, say-do checks, incident reconstruction.
- **Heavier monitoring where uncertainty is highest** — diverse scorers on
  untested checkpoints at small cost.
- **Adoption from outside** — open record format, open-weight scorers, working
  systems let outsiders build the evidence base.

## Eventual implications

- **One model end to end** — prediction, scoring, revision trained into the
  model on its own logs; extra models remain for speed and second opinions.
- **Alignment training grounded in outcomes** — rewards the fit between
  commitments and resolved reality: calibration under proper scoring rules,
  say-do consistency, consequence coverage (recall + precision), world-model
  revisions that improve later behavior, self/independent score agreement.
- **Oversight survives latent reasoning** — the shadow trace becomes primary.
- **The durable asset is the history** — models interchangeable; the structured
  record persists across them. Owning the log matters more than the
  checkpoint.
- **Training and deployment blur** — deployed systems keep learning through
  records; records flow into weights. The environment becomes a school that
  selects experiences, makes errors legible, revisits misconceptions.
- **Alignment becomes governance engineering** — policy languages, provenance,
  delegation, escalation, audit; trained content stays a backstop.
- **Legibility shapes capability expression** — capability deploys at levels
  monitors can follow.
- **Track records become trust** — scored histories act as credentials;
  delegated authority scales with demonstrated record.

## Limits and open questions

Scope: a practical improvement for the failure mode of heavily RL-trained
models — spiky dangerous capability without rounded consequence modeling. Not
protection against a hostile superintelligence; between extremes, the multiple
surfaces raise deception's cost without guaranteeing against it.

| Limit | Why it matters | Current answer |
|---|---|---|
| Fidelity training generalization | may produce honesty only on checked surfaces | the key empirical question; random audits + diverse monitors |
| Materiality completeness bounded by critics | critics score only imaginable omissions | diverse specialized critics; circuit breakers on what monitors can't follow |
| Self-fulfilling predictions | agent controlling outcomes chooses predictable actions | reward fidelity only with task outcomes, inside authorized actions |
| Fidelity ≠ goodness | a faithful agent can execute a harmful plan | governance decides permission; compiled floor is backstop |
| Capability outside the loop | base-model jumps, reserve capability | heaviest diverse monitoring on untested checkpoints |
| Scorer gaming | agents learn a scorer's quirks | no direct scorer access; rotated scorers; disagreement tracking |
| Legibility theater | looking legible rather than being faithful | reward verified correspondence with outcomes |
| Anchoring | stated prediction resists contrary evidence | accepted for caching; blind observation when work is delegated |
| Memory attack surface | poisoned records, adversarial retrieval, stale lessons | provenance, immutable commitments, audited packs, dated lessons |
| Hedging | calibration improves by saying less | score specificity; report calibration with discrimination |
| In-context scorer gaming | learns scorer's quirks within one session, faster than training | feed observations and discrepancies; keep independent scores out of context, or sample and delay them |

Open questions for first reps: how fast the Zeno gap narrows; which retrieval
features predict corrective value; context-pack yield per model; whether
commitment quality degrades under monitoring pressure.

## Relation to existing work

Each component has precedent; the combination and alignment reading are new.
Prequential statistics (Dawid 1984) supplies the timing rule. Reflection
methods (Reflexion, Self-Refine) store lessons after the fact — this stores the
frozen pre-evidence commitment alongside outcome and revision. Anticipatory
reflection (Devil's Advocate) moves reflection before action — this makes the
resolved episode the stored object. Persistent memory (Generative Agents,
MemGPT, Voyager) shows experience lives outside context — this specifies the
episode shape. CoT-faithfulness research motivates checkable commitments over
faithful reasoning — surviving latent reasoning. Deliberative alignment trains
over written specs — here the spec is one governance layer, fidelity auditable
through the ledger. Control/trusted-monitoring assumes partial trust and
designs oversight — this supplies the ledger monitors read.

---

*Source: "Precommitment Records: Theory and Implications" (owner-authored,
2026-09-22). The neutral-probe subsection is omitted at owner direction —
theory-level only, no probe path exists in the system. Companion:
`Precommitment Records — Engineering Memo.md` (mechanism contract).*
