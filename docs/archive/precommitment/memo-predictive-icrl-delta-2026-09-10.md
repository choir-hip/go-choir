# Delta Note — RLMs are RLMs (predictive ICRL memo)

Status: working delta note, 2026-09-10. Green-class document: no runtime,
product, or registry behavior. It is input to a revision of the *RLMs are
RLMs* working memo (draft dated 2026-09-10, held outside this repo) and is not
authority, doctrine, or a mission Definition.

Purpose: record what to add, what to change, and what must be decided before
the memo is rewritten. Written after a first read of the draft; the follow-on
research pass should confirm every citation and identifier below rather than
trust them.

## 1. Verdict

The mechanism is sound, narrow enough to test, and the memo is unusually honest
about its precedents. Two structural problems should be fixed in the research
design, not in the prose.

**P1 — the weakest link sits where the motivation is strongest.** Semantic
prediction scoring has no ground truth, and the motivating domain is reading.
Either the headline claim rests on objectively resolvable prediction classes
(code, tests, retrieval facts, dated world events) with semantic reading
carried as a separate weaker track, or the paper is quietly about forecasting.
The memo should make that split on purpose rather than let a reviewer find it.
Sections 11 and 23.3 identify the scoring problem correctly but Experiments C
and 23.12 still lean on the weak track.

**P2 — prospective prediction is an anchoring device, not only a learning
device.** Committing to a prediction before observing, then retrieving one's own
stored predictions, compounds confirmation bias. The memo files this as open
question 23.14. It is closer to the most likely failure mode, and it needs a
structural answer (maintained parallel hypotheses), not a countermeasure list
(occasional cold readers).

Neither is fatal. Both change which experiments come first.

## 2. Literature delta

### 2.1 Forecasting — the missing strand, and the most important one

Section 18 lists memory, agent-harness, and ICL/RL priors and contains no
forecasting literature at all, even though forecasting is the domain where
predict-before-observe is most testable and most precedented. Add:

- Halawi, Zhang, Chen, Novikov, et al. (2024), *Approaching Human-Level
  Forecasting with Language Models*, arXiv:2402.18563. Closest prior system:
  retrieval-augmented LLM forecasting scored by Brier. This, not a generic
  stateless model, is the baseline Experiment B must beat.
- Karger, Bastani, Chen, et al. (2024), *ForecastBench*, arXiv:2409.19839.
  Continuously resolved live benchmark; the natural external baseline for the
  World Wire environment claim in section 20.
- Zou, Xiao, Jia, et al. (2022), *Forecasting Future World Events with Neural
  Networks* (Autocast). Earlier retrieval-plus-forecast architecture; useful for
  showing how old that shape is.
- Paleka et al. (2024), *Consistency Checks for Language Model Forecasters*.
  Directly relevant to scoring integrity and to rescoring discipline.
- Schoenegger, Tuminauskaite, Park, Tetlock (2024), *Wisdom of the silicon
  crowd*. Ensemble LLM forecasting; relevant to sections 13.3–13.5.
- Tetlock (2005), *Expert Political Judgment*; Tetlock & Gardner (2015),
  *Superforecasting*; Mellers et al. (2014), Good Judgment Project results. The
  forecasting-tournament tradition already established that calibration
  improves with scored feedback, that frequent small updates beat rare large
  ones, and that aggregation beats individuals. Sections 8, 11, and 13.5
  restate findings this literature owns.

Why it matters: the memo currently claims less precedent than exists for the
forecasting half, which obscures where the real delta is. The delta is
prediction as a general epistemic primitive applied across reading and code,
persistence beyond the active context, and cross-model trace sharing. It is not
"predicting outcomes and scoring them with Brier is new."

### 2.2 Calibration of the frozen model — an unstated precondition

The mechanism assumes expressed confidence carries information. Add:

- Kadavath et al. (2022), *Language Models (Mostly) Know What They Know*.
- Lin, Hilton, Evans (2022), *Teaching Models to Express Their Uncertainty in
  Words*.
- Whatever post-training calibration evidence exists for the specific model
  families used.

If verbalized confidence is uninformative for a given frozen checkpoint, Brier
and log scores measure noise, and the remedy is elicitation (multiple samples,
consistency checks, structured intervals) rather than a larger trace corpus.
This belongs in the memo as a precondition on the experiment, not an
assumption.

### 2.3 Cognitive science — the support cited is not the support wanted

The memo grounds the mechanism in prediction-during-comprehension work, which
the cited reviews describe as mechanistically unsettled. The stronger and more
direct support is the pretesting effect:

- Richland, Kornell, Kao (2009), *The pretesting effect: do unsuccessful
  retrieval attempts enhance learning?*, Journal of Experimental Psychology:
  Applied. Attempting a before-state changes what is learned from the material
  even when the attempt fails. That is the engineering claim, already
  established experimentally in humans.

And name the failure mode section 5 is built on:

- Baron & Hershey (1988), *Outcome bias in decision evaluation*. Hindsight
  reconstruction under outcome knowledge is a named, measured phenomenon.

## 3. Research-design deltas

**D1 — Declare the evidence split in the thesis, not in a caveat.** Two tracks:
objectively resolvable classes carry the headline claim; semantic classes form
a secondary track with their own metrics, its own falsifiers, and explicit
lower evidential weight. Name the boundary and who maintains it.

**D2 — Score the choice of prediction, not only the prediction.** Proper scoring
rules already penalize hedging at scoring time, so the "conservative model"
worry in 23.4 is partly a choice-of-rule problem. What proper rules do not fix
is which prediction the model elects to make; unaddressed, the system converges
on safe topic-adjacent trivia that scores well and teaches nothing. Make
prediction selection its own episode class with its own score: expected
information gain over the live hypothesis set, with the counterfactual baseline
section 11.6 already names. Section 11.6 lists this as a countermeasure; it
should be a first-class scored object.

**D3 — Make parallel hypotheses structural.** Require the working state to
carry a live hypothesis set with weights, and require predictions to
discriminate among members. This answers P2, and it supplies the input D2
needs to compute information gain. Occasional cold readers can stay, but as a
check on the mechanism rather than the mechanism's only guard.

**D4 — Type the prediction; treat prose as a rendering.** Store
`{claim, class, confidence, horizon, falsifier, discriminating hypotheses}`
with a machine-checkable falsifier wherever the class permits, and keep raw
prose as an additional field for human and model readability. The memo's
Episode struct is close to this but `Prediction` is a string, which means the
learning signal is over prose and inherits prose ambiguity.

**D5 — Design the bootstrap.** Retrieval quality gates learning and learning
gates retrieval quality; the recursive layers ("predict whether predicting is
useful", scoring the scorer, learning whom to learn from) have no data to learn
from at t=0. Specify a burn-in: fixed predict-everything policy, simple
retrieval (recency or round-robin over hypothesis-relevant classes), and a
budget, with the meta-layers enabled only after a threshold episode count. The
memo names the recursion but not the cold start.

**D6 — Provenance edges are load-bearing, not an optimization.** Delayed and
multi-horizon scoring (11.5) cannot propagate credit without explicit
dependency edges between retrievals, hypotheses, predictions, and later
outcomes. The MemQ TD(lambda) reference is right; the consequence is that the
Episode schema needs parent references from the first implementation, because
retrofitting edges onto an append-only log means rewriting history the whole
design exists to preserve.

## 4. Ablation order

Ordered by how much each result would change the program, and by how cheap it
is to run:

1. **Experiment B, equal compute, objective classes only.** predict → retrieve →
   score versus retrieve → reflect → store. Run it on classes with mechanical
   resolution so that weak semantic scoring does not confound the mechanism
   question. This decides whether the thesis is alive.
2. **Regime split inside B.** Immediate-resolution versus delayed-resolution
   predictions. Prediction should be most valuable where the before-state
   cannot be reconstructed after the fact, and least valuable where evidence
   arrives inside the same context window. The plausible outcome is a win on
   delayed resolution and a tie or loss on immediate reading; that result would
   move the memo's center of gravity from comprehension to long-horizon
   assimilation and forecasting, which is much cheaper to learn now than after
   the full harness exists.
3. **Experiment D.** Does calibration improve beyond the active context, and
   does removing the external history remove the capability. This is the
   publishable empirical decomposition; it depends on 1 and 2 pointing the
   right way.
4. **Experiment F, trace representation.** Raw event versus distilled lesson.
   Cheap, high information, and it tests the section 15 claim directly.
5. **Experiment G, scoring architecture.** Only informative once the objective
   track has a reference result; otherwise it tunes a scorer against no anchor.
6. **Experiment H, scale.** Expensive and meaningful only after the above. Run
   last; it answers the retrieval-collapse question rather than the mechanism
   question.
7. **Experiment C, sequential reading.** Run after the evidence split in D1 is
   explicit, because it is the experiment most exposed to scoring weakness.

## 5. Choir integration deltas

**C1 — Decide episode state authority before writing any schema.** Is a
learning episode part of the computer's canonical event chain, or a derived
artifact rebuilt from other state? This repo has a scar exactly there
(`docs/standing-questions.md`, the phantom route ledger that nearly became a
third store). The safe option consistent with single-state-authority: append
episodes to the existing event chain as a new kind with content-addressed
payload refs, rather than introducing a new store. If instead they are derived,
they need a rebuild path and an explicit not-authority marker so no consumer
mistakes them for canonical history. Read `docs/computer-ontology.md` before
touching this; persistent-state behavior is in its scope.

**C2 — Cross-model trace sharing is a product-ontology question, not a schema
question.** The memo's shared empirical substrate assumes many agents and model
families reading one another's traces. In Choir terms that is shared state
across computers, and the durable product object is the individual computer.
Whether traces are per-computer, shared across a fleet, or shared with
per-field visibility rules is an owner-level decision that should be taken
before the multi-model experiments, not discovered during them. Section 13's
transfer matrix presumes an answer.

**C3 — The cheapest first experiment is not news.** Predict-before-act inside
RLM cells, scored by the test suite and the compiler, needs no new
instrumentation, has objective resolution, and sits on exactly the surface the
RLM cutover mission is already landing. World Wire is the better long-run
environment because reality supplies delayed labels for free, but it needs
resolution infrastructure that does not exist yet: a horizon registry, a
resolver, and dispute handling with the same append-only discipline as the
scores themselves. Section 20 should say that the environment has a build cost.

## 6. Open questions the revision should answer

1. Where exactly is the evidence-class line, and what maintains it as classes
   are added?
2. What is the estimator for expected information gain with a frozen model, and
   what is the counterfactual baseline in practice?
3. How is the live hypothesis set represented, and how does it relate to the
   compact gestalt — same object, or two objects with a defined reconciliation?
4. What is the resolution infrastructure for World Wire: horizons, resolvers,
   and what happens when an outcome is disputed or never resolves?
5. Episode storage ontology in Choir: canonical versus derived, and
   per-computer versus shared (C1, C2).
6. How is equal-compute enforced across arms in Experiments A, B, and D, given
   that prediction adds calls and retrieval adds tokens? The memo asserts
   comparability; the protocol should name the accounting.

## 7. What to cut or demote

- Section 19 (procedure learning, interpreted-to-compiled) stays peripheral, as
  the memo already says. Do not let it into the abstract.
- Section 16 (gradient to parameter RL) is good discussion and should stay out
  of the claim. The memo is right to avoid "recursive self-improvement";
  keep avoiding it.
- Section 3 is well-hedged but long. The engineering claim — that sequential
  predictive updating can be made explicit and measured — does not need the
  cognitive-science framing to stand, and the pretesting citation carries more
  weight than the comprehension-prediction citations.
- The title works as an essay. For a paper version, add a subtitle naming the
  contribution, because the acronym collision is memorable but not
  searchable, and it collides with the already-published meaning of RLM.
- 23.14 (prediction distorts reading) should be promoted out of open questions
  into a design constraint, per P2 and D3.
