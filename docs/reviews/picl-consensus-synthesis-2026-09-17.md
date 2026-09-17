# PICL Consensus Synthesis

**Date:** 2026-09-17
**Panel:** 9 agents (claude, codex, cursor, devin, gemini-3.8, glm-5.3-flash,
gpt-5.6-luna, gpt-5.6-sol, grok-4.6) — independent reviews of the PICL draft,
the Choir mapping section, and the 2026-09-17 in-repo review. 4 agents failed
to launch; outputs in `.agentic-consensus/picl-review/run/`.
**Question:** is PICL sound, what does it change in the Choir design, and do we
adopt it for the beta sprint?

## Verdict

**Adopt at the schema level now; defer consumption and the experimental
program.** Unanimous across all nine panelists — including the three that
explicitly dissented from the in-repo review's framing. The disagreement is
about *why* adoption is right, and it matters.

## Where the panel corrected the in-repo review

The panel's strongest finding is a convergent attack on the claim my review
treated as airtight. Five corrections, ordered by how much they change the
design:

**1. The before-state claim survives as audit, not as information theory.**
Seven of nine panelists independently produced the same counterexample: if the
pre-observation context `s_t` is preserved — and Choir's tape preserves it —
then replaying `s_t` through the frozen model with `o_t` withheld recovers an
equivalent prediction. "Retrospective reflection cannot recover the
before-state" is true of *post-hoc verbal rationalization* and false of *a
system that retains prefixes*. What online commitment uniquely adds: the actual
stochastic sample at time `t` (which replay cannot recover across a model or
prompt swap — cursor's sharpest point), a staked audit object, and cost. My
review's "isomorphism is close to exact" overstated this; the tape supplies
provenance and ordering, not epistemic privilege.

**2. The tape's ordering is not the epistemic boundary.** Devin, codex, and
gpt-5.6-sol: tape order proves event P preceded event O; it does not prove the
predicting actor hadn't already seen O through another actor, a cached source,
or an unrecorded context. PICL's temporal requirement lives in context
construction (`G`), not in the tape. The design needs a stated invariant — a
prediction is PICL-valid only if committed before its resolution is delivered
*to the predicting actor's stream* — plus an evidence-boundary receipt binding
actor, model/version, activation head, and visible sources. My "the tape is
the PICL substrate by construction" claim needs this qualification.

**3. "Casts are free PICL episodes" is half true.** Unanimous. Async casts
provide free *resolution pairs* — the episode frame — because delayed
resolution via result events is genuinely elegant. But the prediction is not
free: `expected` is optional, and a task description is not a stated
expectation with confidence. Correct statement: casts provide free episode
frames; the prediction step is inference the design currently mandates
nowhere.

**4. A deterministic fold cannot mint learning records.** Gemini-3.8's best
point, echoed by cursor: projections are pure folds; they can pair
`expected`-carrying events with their resolutions into unresolved candidate
pairs on the object graph, but discrepancy and revision are model judgments.
The design needs an asynchronous **Curator desk** that consumes candidate
pairs off the critical dispatch path and appends `learning_record_minted`
events. My "folded from the stream — same mechanism as open-work" was wrong
about the mechanism; the fold produces *candidate pairs*, not records.

**5. "Only upside" was too casual.** Grok-4.6 and cursor dissent explicitly:
storage without consumption is the only schema change that is actually
ignorable. Retrieved records injected into default context are the risk —
poisoned-memory, anchoring, negative transfer — and must stay off until the
experiments justify them.

## The missing decisive experiment

Six of nine panelists independently named the same gap in the paper's program:
**the deferred/sealed pre-state replay control.** After `o_t` is observed,
replay preserved `s_t` in a fresh outcome-blind context to generate `p_t*`,
then run the identical discrepancy/revision/persist pipeline. Without it,
`E > B` cannot distinguish "temporal commitment helps" from "any structured
(p,o,r) record helps regardless of provenance." Cursor adds a second missing
arm: `B′`, a schema-matched retrospective record — if `D ≈ B′`, the ontology
is doing the work, not the clock. Gemini-3.8 flags that Condition B as written
lacks persistence/retrieval, so the headline comparison confounds prospectivity
with memory. This is the one paper change every panelist would make.

## Multi-actor: the break my review under-weighted

Devin: a cast's resolution is produced by a *different learner* (the child)
than the predictor (the parent). The discrepancy measures the parent's model
of the child's competence — **delegation calibration**, not world modeling.
That is a richer signal and a confound: `f_θ` in the formalism is one model;
in Choir `o_t` is another model's output. Grok-4.6 adds cross-actor leakage:
a shared tape lets actor B see A's prediction before B's own observation of
the same source. Records must be actor/model-scoped; cross-actor retrieval is
an experiment, not a default.

## Prior art the paper misses (convergent)

- **PreAct** (codex: "major omission" — explicitly learns from
  prediction/outcome disparity), **ExpeL** (3 panelists), **PreFlect**,
  **Hindsight**, **ECHO**.
- **Prioritized Experience Replay** (gemini-3.8): ECV is the natural-language
  analog of TD-error prioritization — grounds ECV instead of leaving it ad-hoc.
- **Conformal prediction** (devin): mature machinery for the calibration
  guarantees H4 needs.
- **Superforecasting/Tetlock** (devin, gpt-5.6-sol): the human existence proof
  for explicit forecasts + scoring rules improving calibration.
- **Predictive processing / active inference** (gemini-3.8), **hindsight
  bias / Fischhoff**, **pre-registration** (grok-4.6), **Quiet-STaR** (cursor,
  grok-4.6), **hypercorrection psychology** (codex).
- Cursor found citation-hygiene errors: Reflexion linked to Self-Refine's
  arXiv id, MemGPT to Generative Agents' id in the body text.

## The strongest theoretical argument — one the paper doesn't make

Devin: the base model was trained by a prequential objective; next-token
prediction *is* Dawid's framework. Explicit prospective prediction recruits
the computation mode the model was optimized on; retrospective rationalization
is a far rarer, RLHF-shaped genre. This gives H1 distributional plausibility
beyond "data intervention." Gemini-3.8 counters: prospective tokens search an
exponential fan-out of unexercised branches while retrospective tokens spend
100% of budget on the realized causal chain — a search-efficiency penalty the
paper never prices. Both are right; the experiment decides.

## What Choir gives PICL that no chat harness has

Convergent list, ranked by panel emphasis:

1. **Native hidden commitment** — a separate activation evaluates discrepancy
   without `p_t` in its context; no session-branching hacks (gemini-3.8).
2. **Cross-family decorrelated critics** — the gateway catalog lets model A
   predict and model B score, killing same-family blind spots the paper's
   multi-model fix can't reach (devin, gemini-3.8).
3. **Level-4 causality tests in production** — remove/restore ablation is
   just event filtering in context construction (devin).
4. **Replayable prefixes** — the tape already keeps `s_t`, enabling both
   backfill *and* the missing replay control (cursor).
5. **Desks as learner identity** — `L = (M_θ, E, R, G)` gets actor scope for
   free; the desk's calibration history is the persistence argument.
6. **Mutation-class ceremony** — the strongest fit in the paper: red/black
   already demands conjecture delta + evidence + rollback; making `expected`
   consequences a committed scoreable record converts prose ceremony into a
   foldable object. All nine flagged this as the natural placement site.

## Architecture decisions for the beta sprint

What the panel's verdict means concretely — the "set our architecture here"
list:

1. **`expected` field** — optional, on `rlm.spawn` and red/black action
   events. Permissive payload: text + optional confidence + optional
   expected-shape ref. Actor-scoped.
2. **`resolves` / `prediction_ref` linkage** — on result/observation events.
   Cast pairs get it free from `work_id`; non-cast predictions need it
   explicit.
3. **Epistemic boundary invariant** — stated, not assumed: a prediction is
   PICL-valid only if committed before its resolution is delivered to the
   predicting actor's stream; context construction for the prediction
   activation must not include the resolving event.
4. **Pairing projection** — deterministic fold materializes unresolved
   (prediction, resolution) candidate pairs onto the object graph with
   provenance edges. No model calls in the fold.
5. **Curator desk** — async, low-priority, consumes candidate pairs, invokes
   the model for discrepancy/revision, appends `learning_record_minted`
   events. Off the critical dispatch path.
6. **Result-wake policy** — on result delivery, context construction either
   reattaches `expected` or deliberately withholds it. Choir currently
   implements hidden-commitment-or-lost-commitment by accident; make it a
   policy (cursor's "revision activation").
7. **Retrieval injection off by default** — learning records queryable via
   the RLM repl, never auto-pulled into cell context. Consumption waits for
   the experimental program.
8. **Scope** — records carry `actor_id` + `model_id`; cross-actor/cross-model
   retrieval is an experiment, not a default.

Cost: two optional fields + one fold + one stated invariant + one async desk.
All additive, all ignorable, none load-bearing for correctness. Worst case if
H1 fails outright: a provenance-linked expectation ledger that is
independently useful for audit and debugging — and the beta generates the
naturalistic records the experimental program needs anyway.

## Residual disagreements in the panel

- Whether the replay attack *fully* deflates temporal commitment (cursor:
  replay dies across model swap, so the sampled commitment is still unique)
  or merely demotes it to one mechanism among several (gpt-5.6-sol, codex).
- Whether prospective elicitation is a measurement readout or itself a
  cognition intervention (claude's "forcing-function confound": requiring a
  commitment forces hypothesis formation — a generate-then-test intervention
  masquerading as a data intervention).
- Confidence that H1 survives the replay control: panel range 0.5–0.7.
- Whether to call schema adoption "adopting PICL" (cursor: don't — it's
  instrumentation) or the substrate PICL needs (my review's framing).

## Bottom line for the sprint

The paper's durable contribution is not before-state preservation — a skilled
retrospective system with retained prefixes approximates that — but the
**resolved prediction–observation record as a first-class, retrievable,
scoreable object**, plus the conversion of expensive evaluation into cheap
resolution. Choir is the best possible host for it: the tape, casts, object
graph, gateway, and mutation ceremony each supply a piece chat harnesses must
fake. Set the eight schema decisions above, ship the beta, let it generate
real episodes, and run the replay-controlled experiment on real data after
launch. The panel's unanimous verdict: the fields are nearly free; the claims
are not yet earned; collect the data that earns them.
