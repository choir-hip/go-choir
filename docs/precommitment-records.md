# Precommitment Records and Prospective Self-Alignment

Canonical mechanism and theory document, synthesized 2026-10-14.

**Status and authority.** The one current home for the precommitment record
mechanism, its theory (prospective self-alignment, PSA), the inter-agent
protocol built on it, scoring, and signing. It inherits
[Choir Doctrine](choir-doctrine.md), the apex; doctrine wins any conflict. It
restates once what was spread over seven overlapping documents, now archived
under `docs/archive/precommitment/` and in git history (section 9 maps them).
Where a source disagreed with a newer decision this document follows the
newest, and section 7 lists what was withdrawn. Deciding documents, by
authority: [owner notes 2026-10-14](reports/owner-notes-2026-10-14.md),
[re-engineering decisions 2026-10-12](re-engineering-decisions-2026-10-12.md),
the owner's 2026-10-11 protocol answers. Two live companions stay separate and
are summarized in section 5: the
[claims, events and entities design](reports/claims-events-entities-ontology-2026-10-14.md)
and the
[historical back-testing research](research/historical-precommitment-backtesting-2026-10-14.md).

Labels: *built* means in the code; *decided* means an owner decision the code
does not yet fully implement; *hypothesis* means untested. PSA is a research
program, not a demonstrated guarantee of alignment, honesty or safety.

---

## 1. Thesis, and why this is the core

### 1.0 Why: reading, and the scientific method (owner, 2026-10-24)

The owner's own account, kept here to be revised as the thinking develops.

**Engineering becomes science.** "Pre-commitment records turn engineering
into science. ... We're applying, quote-unquote, the scientific method to
everything, that you have to register your hypothesis before you do the
experiment, where an experiment is something related to coding, or it'll
also be as simple as research of, you know, before you do a search, you
should set your expectations clear. So that way you actually are basically
forced to learn as the reality meets your hypothesis, either confirming or
updating."

**The origin: reading, not writing.** One origin was disappointment with
the quality of language models' writing, until it became clear that "it
wasn't writing quality that was the weakness, but reading quality." Reading
is among the most general cognitive functions, and it has many levels:
close reading, reading between the lines, context and co-text, allusion,
critical reading, and the traditions of literary and legal study. The
mainstream habit of taking people at face value, and not imputing motives,
is not deep reading. Deep reading is what poker players do in hand
reading, or what anyone does reading a face, body language or a room: "being
able to impute from the little bits of leaked information that people are
trying to conceal is the essence of reading at a deep level. And the only
way that you can calibrate that finally is by having more of a scientific
approach where you have limited information, you make a hypothesis, and
then you have to run an experiment or you have to observe how that
hypothesis meets reality and update from that and keep calibrating. That's
how you can work with imperfect information."

**Feedback, not examples.** The owner says in-context *reinforcement*
learning on purpose: "in-context learning evokes in most people's minds
few-shot learning from examples," while today's models, trained with so
much reinforcement learning, are far more sample-efficient with feedback
than with examples. "We want to provide as much rich feedback to models as
we can. We're really gonna be bootstrapping and improving the calibration
of models ... This is the core scientific innovation of Choir in my mind."
The 2026-10-24 rulings that follow from it (mandatory precommits on
background tasks, fixed and free-form claims, System1 scoring, engineering
sees aggregates of its own calibration with a 10% held-out set, calibration
feedback on) are in the launch first pass ledger, 18:2x; the context-design
text in 1.2 and 4.7 is being revised to match
(docs/design/tasks-precommit-icrl-2026-10-24.md, in progress).

### 1.1 The mechanism

Intelligent systems should commit to what they expect, and why, before the
evidence arrives. A precommitment record is a typed, time-stamped record,
immutable once committed, of a hypothesis, its assumptions, the alternatives
considered, and a prediction with probabilities, bound to an actor, task and
time and falsifiable against a later observation. Two requirements are hard
and everything else is a default: the prediction is committed before the
evidence that resolves it is available to the agent, and it resolves against
a verifiable outcome. The loop is commit, authorize, act, observe, resolve,
score, revise, retrieve. Authorization is its own step: an accurate promise
to do an unauthorized thing does not make it acceptable. Records make
governance inspectable; they do not replace it.

**The hindsight problem.** Once an outcome is known a capable model can
explain why it was unsurprising, so real updating cannot be told from
rationalization. Post hoc explanations also omit factors that drove the
answer, and asking "why did you do that?" gets less reliable as models reason
in latent states. The fix is temporal, not introspective: freeze the
expectation before the evidence exists. This is Dawid's prequential
principle (judge a forecaster only by forecasts made before outcomes), with
one change: here the results are learning data, not only evaluation.

### 1.2 It is in-context reinforcement learning (the core claim)

Owner, 2026-10-14: the key value of PSA is storing records in the desk's Yaegi
REPL, with context packs and on-demand retrieval; it serves as in-context
reinforcement learning, where feedback to models enables self-learning without
modifying weights. This is what makes Choir useful as an organizational
multi-agent operating system. Working hypothesis: in-context RL is to
in-context learning as RL is to self-supervised pretraining, because learning
from feedback on one's own outputs is far more token-efficient than learning
from examples. The evidence is thin but consistent: Monea et al. (2025) show
models learning in context from rewards on their own predictions; Song et al.
(2025) report quality rising as rewards accumulate in context even when the
model generates them, which supports self-scoring.

The same mechanism drives long-horizon reward hacking (Pan et al. 2024:
test-time feedback loops make models optimize a proxy with negative side
effects). Agents learn efficiently from feedback whatever it measures. Records
redirect the mechanism: the feedback in context becomes materiality-weighted
prediction error about consequences, not a proxy, so the pressure that
produces reward hacking produces better world models, and drift in what an
agent commits to over a long session is itself visible in the ledger.
**Context design is therefore a safety decision**: feed an agent observations
and discrepancies; keep independent scores out of its context, or sample and
delay them, so it cannot learn a scorer's quirks inside one session (section
4.7).

**Verifiability from less-verifiable tasks.** Where quality is hard to judge
(interpretation, research, strategy, writing), "was this good?" is expensive;
"what did it predict, and did that hold?" is cheap and repeatable. PSA turns
everything into a scoreable prediction (owner, 2026-10-14). It does not
answer the hard question; it yields evidence about it repeatedly and cheaply,
and learning counts only if later behavior improves on independently
assessed work.

### 1.3 One mechanism, four uses

- **Context packs (core).** The log tunes curated context per model and task,
  correcting systematic biases and leaning on reliable strengths (4.7).
- **Capability.** A commitment before each search, retrieval and read, and an
  update after, makes reading sequential model revision: surprises show where
  to read deeper, ambiguity is recorded rather than forced, and the agent
  cannot claim it always knew.
- **Alignment and oversight.** Prevention (predicted side effects visible in
  time), detection (departures show as discrepancies), recovery (the first
  failed prediction in the nested chain marks the divergence). Section 3.
- **Reputation and the learning economy.** Scored histories are conditional,
  domain-specific competence records; tagline (owner, 2026-10-14) "multiplayer
  intelligence for the learning economy". Section 5. Organizational oversight
  is the same data viewed by role: an executive sees the box scores and open
  commitments of every agent in scope, across vendors. A product direction,
  not a built surface.

### 1.4 Lineage: goal files were the earlier recursion

Owner framing, 2026-10-14: the throughline goal-file format
(`skills/throughline/SKILL.md`) and its predecessors (mission-gradient,
parallax, definition; deprecated 2026-09-22, kept in `docs/archive/skills/`)
are a precursor of precommitment records: the same pattern one level up. A
mission states before acting its value criterion (`better_means`, and
`goodharting_would_be`, the cheap proxy that would satisfy the letter and
miss the goal), its realism axis and typed conjectures (claim, test, edge,
smallest observer upgrade, scope), then updates a current-state card as
evidence arrives. The mission-gradient thesis ("homotopy, not ladder") was
explicitly a reward-hacking mitigation: fake stages and disposable mocks
become local proxy rewards and in-context precedent that later agents copy. A
goal file is feedback engineering for the agent that reads it: what sits in
context steers what is optimized. Records add the frozen before-state: a goal
file's `now` card is rewritten in place and judged as prose; a record is
append-once, typed, per act and scored later by someone else.

### 1.5 Names

"Precommitment record" is the primitive (code: *commitment record*, kind
`choir.commitment_record`). PSA is the research program: whether conduct and
consequences stay coupled to the prospective representation under which a
system was allowed to act. *Self-alignment* means an agent's own prior
commitments become objects of assessment, not that it decides what is
permitted. PICL and "predictive ICRL" are retired names.

---

## 2. The record

### 2.1 What it holds, and why

- **Events, not lessons.** A stored lesson ("the test failed because the lexer
  doesn't nest comments") loses what a record keeps: a 90% prediction that
  nesting worked, the failure, the discrepancy, the revision. The event shows
  whether confidence was justified, the lesson too broad, or the same
  assumption failed elsewhere.
- **Predictions carry their reasons.** The unit of learning is the belief
  state behind a prediction; a discrepancy points at the failed assumption and
  the revision updates the model, not just the forecast. The accumulated
  records are the agent's evolving world model.
- **Behavior change is the evidence.** An eloquent revision that leaves later
  behavior unchanged is a narrative. Learning value is not accuracy: a badly
  wrong prediction can be the most valuable record, and a surprise can be noise.
- **The composed learner** is the frozen model plus records, retrieval and
  context construction: remove a record and its effect should go; restore it
  and the effect should return. Keep rich evidence (scores are views) so later
  scorers and objectives can reinterpret it.

### 2.2 Schema

Five fields are required. Records are structured Go types, queryable from the
REPL and passable between agents as data (`internal/types/commitment.go`).

| Field | Req. | Contents |
|---|---|---|
| Prediction | yes | Hypothesis, assumptions, competing alternatives, typed questions with the agent's probabilities and a materiality weight, frozen at commit |
| Observation | yes | Compact excerpt plus a reference to the full source |
| Score | yes | Per scorer: answers, probabilities, pinned model id; a disagreement flag |
| Discrepancy | yes | confirmed, qualified, contradicted, unresolved (code adds `answered`, an arrival-class resolution, not a material verdict) |
| Provenance | yes | Agent, model id, context reference, commit and resolve times |
| Revision, Action, Consequences, Links | no | Assumptions weakened or strengthened and the updated hypothesis, written on real surprise; intended action and declared capabilities and predicted versus observed side effects for material actions; parent, child, related and retrieved-by records |

As built the record also carries the act kind (precommit, report, resolve,
disagreement, directive), addressee, inbox sequence, conversation id, round, a
final flag, emitter kind, computer id and subject references. It still has an
in-record `Scores` slice; the decided shape is scores as separate records (4.6,
8.3). Records are minted once under a deterministic identity with a not-exists
condition, so a replay mints nothing, and live on the computer's object graph
and tape, never a separate store.

### 2.3 Scale and when to commit

A commitment can wrap one file read or a months-long project; open and close
can be separated by any gap; commitments nest, the parent capturing purpose
and reach, children exposing decisions where correction is possible. Default
commit points, chosen for cost rather than as limits: **before sub-agent
dispatch** (the most frequent, cheap) and **before material actions**
(intended action, expected result and side effects, and when to escalate
instead). Initial material classes: destructive or irreversible writes;
permission or credential changes; external sends and network access; spending
above a threshold; effects on other users or services. The final list and
threshold are unset (8.3).

### 2.4 Accounting, not mind-reading

A prediction is useful because it can be checked later, not because it
reports internal computation. Commitments leave a **shadow trace** independent
of how reasoning happened: belief, prediction, planned intervention, expected
observation, actual intervention, actual observation, revision. It survives
latent reasoning, where chain-of-thought monitoring loses footing (CoT can be
unfaithful: Turpin 2023, Lanham 2023; optimizing against a CoT monitor can
induce obfuscation: Baker 2025). The claim is a complementary behavioral
channel, not proof it matches CoT monitoring in detection power.

A *neutral probe* (ask the model about consequences outside the action context)
could separate capability from concealment. It is theory only: no probe path
exists in the system, and the condensed copy of the owner paper dropped it at
owner direction. Do not build one on this paragraph.

---

## 3. What the records measure, and why that is alignment

The classical question is how to guarantee a smarter optimizer has the right
goal. This paradigm asks how intelligent processes can continually expose the
gap between what they predict, attempt, cause and learn: error is permanent
and correctable, and trust follows how fast discrepancies surface and
corrections stick, not values certified in advance.

### 3.1 Five surfaces

Kept separate on purpose: one aggregate "alignment score" would erase the
difference between correct prediction, faithful execution, foresight and
legitimate authority.

| Surface | Question | Measured by |
|---|---|---|
| Epistemic | Did beliefs predict reality? | Calibration, world-model revision |
| Material | Did the commitment cover the consequences that could change a decision? | Materiality coverage |
| Instrumental | Did the authorized action cause the intended result? | Outcome scoring |
| Procedural | Did it act within, and for roughly, the reasons and envelope it stated? | Procedural fidelity |
| Normative | Is the action permitted and accountable to legitimate authority? | Policy and authority checks (governance, not measurement) |

The first four are measurable from the log and can be hill-climbed as
capabilities. Competence is becoming a commodity; foresight, materiality,
fidelity and permission are where the work is.

**Procedural fidelity** is the central objective: stable correspondence
between what an agent represents beforehand and what it does. It is neither
behavioral nor introspective. Roughly it is the probability of the observed
action and consequences given the prior commitment, judged by action class
and consequence envelope, never by textual similarity. Low fidelity: predicting
access to repository A then accessing B; saying no network call is needed then
making one; declaring read-only then calling a write-capable module (checkable
mechanically, with no model judgment). Fidelity is not goodness: a kept
promise to do harm is aligned on this surface.

**Materiality** is its own axis: "I will generate tokens" is true of every
action and changes no decision; "this function returns a boolean" is true but
may cover none of the impact if the function controls something that affects
life or property. For action *a* and consequence *c*:

```text
w(c)        = P(c | a) * impact(c) * relevance(c)
coverage(P) = sum of w(c) over c in C* covered by P / sum of w(c) over C*
score(P)    = sum over c in C* of  w(c) * S(p_c, outcome_c)     S: Brier or log
```

`P(c | a)` comes from reference predictors, not only the agent; `impact` is
set on a governance scale (reversibility, scope, domain) and flows downstream,
so a commitment must predict *reach*; `relevance` is whether adding *c* would
flip the policy verdict (a prediction true of every action never does, so its
weight is zero). `C*` is built from what independent critics surface and from
what actually happened. Omissions score as predicted at zero, trivial
predictions earn nothing, and listing every catastrophe does not win because a
proper rule punishes high probability on things that do not happen. The
reference set is the weak link (critics find only what they can imagine;
observed consequences sharpen it), and this is a proposed design, not a
validated formula. **Materiality completeness** trains adversarially: critics
hunt for omissions and consequential ones are penalized heavily.

### 3.2 From alignment problems to capability problems

If an action predictably leads to harm, its commitment should say so, and that
should be enough to block it. Harmful outcomes then classify by what the
commitment said:

| Case | Record shows | Classification |
|---|---|---|
| Predicted, blocked | Harm predicted; blocked or escalated | The system worked |
| Predicted, authorized | Authority approved with consequences represented | Governance decision; accountability on the authorizer |
| Predicted, done anyway | Agent acted against a block or without authority | Normative misalignment |
| Concealed | Foreseen under a neutral probe, omitted from the commitment | Fidelity failure |
| Predictable, not predicted | A reasonable predictor foresaw it; the model did not | Capability failure (prediction power) |
| Not reasonably predictable | No reasonable predictor would | Accident |

An unpredicted harm is a prediction miss, not evidence of wanting harm and not
reward hacking; treating it as capability makes it tractable (better world
models, richer materiality, stronger critics), so capability progress works
for safety for this share of problems. The owner paper says this converts "a
large share" of alignment problems; the PSA paper rightly adds that not all
reduce to capability, because hidden objectives, adversarial action,
measurement gaps and illegitimate authorization stay live. The residue (acting
against a predicted harm; concealing a prediction) is narrow and measurable.

### 3.3 Governance and oversight

**Normative alignment is governance.** Once intended action and consequences
are represented, permission is legal, moral, political, contractual or
institutional. The gate: declare action and consequences, assess coverage,
check policy and authority, then authorize, escalate or block, execute under
control, resolve and audit. Aligned does not mean never breaking a rule; it
means acting under an intelligible allocation of authority with consequences
represented and accountability attached. The core question is who decides:
user, employer, regulator, court, operator, or the agent under bounded
delegated discretion. Developer-chosen morality varies, is partly emergent and
never complete, so it cannot make the long tail safe; checkable things carry
that weight. Governance sits in two auditable layers, *compiled into weights*
(always present, hard to override; opaque, slow to update) and *applied at
runtime* (legible, versioned; can be silent, slow or captured); silence,
illegitimate authority and escalation judgment justify a compiled floor, which
is a backstop and no guarantee.

**Oversight works on the ledger, not the mind.** A gap always remains between
what a model can claim, do and get away with (the *Zeno gap*); the claim is a
measurable narrowing rate, how often discrepancies are caught late or never,
not closure. *Circuit breakers* let monitors halt what they do not understand,
so the agent acts at a level its monitors can follow or explains until they
can, with thresholds scaled to materiality. *Structural coupling*: agents and
monitors learn from the same log and monitor predictions are scored
commitments, so exercised capability cannot outrun monitoring; what escapes is
capability from outside the loop (a new base model's jump) and capability held
in reserve. Untested checkpoints get more and more diverse scorers. In
*distributed supervision* the executor predicts its action, a critic predicts
what the executor will do and a monitor predicts side effects, all frozen
before action. Deception gets harder, not impossible, and sanitized
commitments under monitoring are a detectable pattern; none of this protects
against a hostile superintelligence far beyond its monitors.

**Training (hypothesis).** The eventual version trains one model end to end to
predict, score and revise on its own logs, rewarding calibration, say-do
consistency, consequence coverage and honest self-scores; the durable asset is
the history, not the checkpoint. Cautions: self-fulfilling predictions,
legibility theater (reward verified correspondence, not how a commitment
reads), and the crux, whether fidelity training generalizes beyond the
surfaces that get checked.

---

## 4. The protocol: how desks use records

### 4.1 The invariant

One agent affects another only through a commitment record addressed to it.
The record is the only thing that wakes the addressee, and the ledger is the
complete account of who asked whom for what, what they expected, and what
happened (owner, 2026-10-11: lock down the system so all inter-agent
communication goes through the protocol). A committed prediction does not
authorize an act: casts to engineering and material actions still pass
existing gates (management's admission, capsule boundaries, owner approval).
The owner's input and runtime machinery are not agent speech; where they
inform a desk they appear as observation records with system provenance (a
poll's new captures become an observation to `management:<owner>`), and an
owner revision commits an owner-emitted record pointing at it, so every inbox
sequence belongs to a record. Desks: Texture is the sole writer of canonical
document revisions; management owns computer-wide coherence and admits
engineering through delegated casts; engineering performs capsule-bound
mutation; research has read-only world-evidence authority.

### 4.2 Acts and routing

| Act | Asks for work | Resolves | Note |
|---|---|---|---|
| Cast | yes | no | Carries hypotheses and uncertainties (4.4) |
| Ask / Reply | yes / no | the Ask | An Ask and Reply inside a round do not close it |
| Report | no | the Cast or Ask it answers | A final Report closes the recipient's round |
| Note | no | no | Unscored; read at the addressee's next wake |
| Escalate | yes | no | To management or the owner |
| Precommit | no | no | A standalone prediction |
| Resolve, Disagreement | no | the target | Disagreement keeps a scorer's differing verdict apart |

Retired from the desk surface (deleted): `choir.Message`, `choir.Emit`, the `ApplyTexture`
controls that opened work, `choir.Assign`, and (per Re-eng 13, though the type
still lists the subtype; verify) Retract. Management sits between Texture and
engineering: engineering never addresses Texture; management sends records and
scores to a Texture and never modifies its document, and with Jev decides
whether to pass an engineering report through or restate it. Jev is part of
management and never holds.

### 4.3 Addressing, conversations, rounds, cursors

- An address is a desk plus a subject: `research:<doc>`, `management:<owner>`,
  `engineering:<assignment>`, `texture:<doc>`. A Cast opens work on a subject
  or continues open work. Any desk may address `texture:<doc>` for a
  same-owner document; Texture decides what to do with the delivery.
- **One conversation** per (sender desk, recipient desk, subject).
  Parallelism is the recipient's business (research spreads over sub-RLMs;
  engineering over capsules); sub-RLM work reaches the sender only through the
  recipient's Reports.
- **Rounds.** The recipient closes the open round with a final Report saying
  what it settled, dropped and why it stopped; the next Cast opens a new
  round. **Status is a fold** over append-only records, never a mutable
  column; work items and delivery packets are deleted.
- **Delivery by cursor, as in Kafka.** Each record addressed to an inbox takes
  the next sequence number; each reader keeps one cursor, advanced in the same
  transaction as the work that read the records (for Texture, the revision
  commit), so nothing is read twice, skipped or stranded by a crash.
- **Crash closes, planned restart may resume** (CLAUDE.md): a crash closes the
  open round with a system record, "interrupted by a restart", that wakes no
  one; its hypotheses stay on record for a deliberate next round.

### 4.4 Casts carry hypotheses and uncertainties

A request for work is not "find X; I expect a primary source, p=0.8". It
states what the caster believes and where it is unsure, which leaves room for
learning and tells the worker how hard to work. The unit of progress is the
hypothesis (idea-level, not action-level, supervision).

- **Hypotheses** have stable ids in a conversation (H1, H2, ...), each with a
  probability, a weight for how much it matters, and what would change the
  caster's mind. A later Cast may add, revise or withdraw. Each is open,
  settled (with an outcome) or dropped (with a reason). Reports name the
  hypotheses they bear on or mark findings outside every one. Scoring and
  calibration are per hypothesis. (Built: `internal/store/inbox.go` is the only
  writer of round and hypothesis state.)
- **Open uncertainties** with no hypothesis yet, weighted by stakes.
- **General hypotheses** are open-ended but falsifiable claims that fit almost
  any inquiry ("there are important things I haven't mentioned"; "a primary
  record exists that would settle this"; "sources that look independent share
  one origin"), offered as a space to sample from, not as rules, each needing
  an operational meaning of its key word before it can resolve; the seed list
  is Re-eng 3.1. How often one comes true against its stated probability
  measures how well a desk knows what it does not know.
- Effort follows uncertainty times stakes; no round caps, budgets or
  runtime-owned research job. **Support judgment** (does a source support a
  claim) is done by management or Jev, never by research (which found the
  evidence) nor by Texture (which wrote the claim).

### 4.5 What runs where

The ledger is on each computer's object graph and tape. Desks reach it
through the in-cell `choir.*` module (Cast, Ask, Note, Reply, Escalate,
Precommit, Report, ReportPacket, Resolve, Disagreement, `choir.Pack()`), and a
reducer appends the records. Agents can commit and resolve; they cannot edit a
committed record or query a scorer. The prediction stays visible in the
agent's transcript by default (append-only context, intact prefix cache);
blind reading is an option when delegating, and anchoring is an accepted cost.
Desk-to-desk records are working state on the computer; published edition
claims, inferences and scores go to the shared Store B, where other computers
read and score them and overlays decide whom a reader trusts.

### 4.6 One schema across desks, editions and sources

Owner, 2026-10-11: the same records serve desks and Autopaper; claims and
inferences from news are precommitments scored over time; others make their own
editions and score them, "so it's not our hegemonic house view only"; the basis
of a track-record reputation system, scoped by scorer and topic.

- **Three emitter kinds, one record**: a desk's act; an edition's claim or
  inference (emitter: the computer); a source's claim. Sources never emit
  records; per the claims design Choir records their claims as attributed
  precommitments on their behalf (amends the old "sources never precommit";
  needs ratification, 8.3).
- **Scores are separate records, never fields on the claim.** A score
  references its target and carries the scorer's identity (computer, desk,
  agent, pinned model, method), its answers and the evidence it used; any
  number of scorers can score one record and disagreement is kept. Brier,
  calibration and skill are views computed at query time.
- **Resolution evidence is shared; judgment is not.** What happened is
  evidence anyone can cite; how it bears on a claim is each scorer's call.
- **Reputation is a query, not a number**: scores on an emitter's records
  filtered by scorer, topic and time; readers choose whose scores to trust, and
  scorers are scored the same way, the defense against a crowd of fake ones.
  Topic is derived from subject references at query time, never a taxonomy.
- **Reserve now**, because records written now become track record and cannot
  be re-keyed: emitter identity (computer, agent or desk, model, kind), subject
  references, a content hash and store-assigned commit time, scores as
  separate records.

### 4.7 Records in the desk REPL, packs, and what a desk may see

Decided (owner, 2026-10-14): records and their settled scores are first-class
in each desk's REPL, readable as records and retrievable on demand into
context packs, not only dashboard rows. The feedback loop is the product: a
desk sees its own settled track record when it predicts again. Built:
`choir.Pack()` returns an acting pack; derived views include per-agent
tallies, a supervision projection ordered by materiality, a score-free pack
for the acting desk and a score-carrying pack for supervision, and a gate
refusing a learning claim that cites no scored records. Not built: record
query verbs in the REPL as such, and automatic pack injection, which stays off
until held-out evidence supports it (H2).

The tension (8.3): the 2026-09-29 owner-ratified boundary says the acting pack
never carries commitment scores (read as: no scorer's judgment of the desk's
own predictions), while the 2026-10-14 intent is that a desk sees its settled
record. The memo's principle reconciles them: feed observations and
discrepancies (what happened, what the desk predicted, how settlement came
out); keep independent scorers' judgments out of the acting context, or
sample and delay them. Where exactly the line falls is open.

**Packs** aggregate records by model and task type, find systematic biases and
strengths, and select the records with the most corrective value (the why, not
only the events); each is measured by calibration, error recurrence and
disagreement in later records, and built per model because biases differ.
**Retrieval is by corrective value, not text similarity**: the most useful
record may share the mistake rather than the vocabulary (authentication versus
authorization recurs across APIs); features include error type, shared
assumptions, causal structure, surprise size and whether the revision later
helped, and causally different look-alikes are the adversarial case. Packs are
the nonparametric step; the same log can later feed weight training.

---

## 5. Scoring, track records and who is tracked

### 5.1 Principles

- **Scoring needs intelligence, not a runtime counter.** A model, Jev or an
  RLM scores, often from several perspectives (re-engineering decisions
  section 9). The runtime keeps substrate: delivery, idempotency, schema
  well-formedness, crash safety. How deterministic rule checks fit is open
  (8.3).
- **Settlement is separate from scoring.** Settlement is the outcome plus
  evidence refs written by a designated settler; a committing desk never
  settles its own hypotheses. A score is a separate record with scorer
  identity. A score uses the committed version and original probability,
  never a later edit, and the commitment's sequence number precedes the
  earliest capture its settlement cites (station L3 intent).
- **Answers are evidence, scores are arithmetic.** Brier, calibration, skill
  and box scores are views over answers with their sources; the governing
  answer for a question is a query parameter, not a field.

### 5.2 Who scores: several scorers, each scored

Owner, 2026-10-14: agents and desks score themselves; they are not the only
scorers. Every settlement has several scorers, each scorer has its own track
record, and calibration of self-scores is itself scored.

| Scorer | Role and known bias |
|---|---|
| Self (the committing agent's model) | Default learning signal at no extra cost; flatters its own forecast; never an oversight signal. Calibrated self-scoring is a key capability. |
| Peer desks (answering desk, management) | Can answer typed questions blind to the asker's probabilities; flatters its own work, so checkable claims are checked by rule. |
| Jev (TypeSafe System One; typed `noul`, `choice`, `score` questions) | Fast, cheap, a different model family; blind to who asserted and to the asker's probability; literal reading, per-type miscalibration. Also judges citation support, intake labels and front-page importance today. |
| Rules (deterministic checks) | Ground truth where a fact is checkable: declared versus used capabilities, a figure against a dated primary record, an event of a named class. |
| Panels, other LLMs (agentic consensus) | Offline audits of disagreements and gold labels, for uncertainty, not ceremony. |
| Humans (owner, editors) | Audit samples and gold labels; a quality audit, not a gate. |

**Self-scores are not governance**: they improve the agent's calibration and
retrieval; anything that gates or oversees action needs an independent
scorer. **Disagreement is a first-class signal**, logged with direction. Self
says confirmed, independent says contradicted: possible self-serving scoring;
the **self-favor gap** per agent and model is the most useful oversight number
self-scoring yields. The reverse: the agent may know what the scorer's state
lacks, or the scorer read literally. Either way the prediction may be vague.
Material disagreements go to a third opinion and are audited first.
**Score the scorer**: compare its answers with later ground truth (rule
outcomes, panel or owner labels); this is the defense against captured, lazy
or fake scorers.

### 5.3 Typed questions, Jev, calibration

Typed yes/no, choice or scale questions make scoring simple for any model and
make Jev a drop-in: confirmed, degree of match, discrepancy class, and
specificity (to catch predictions that score well by saying little), plus one
yes/no per stated assumption so a miss points at the assumption that failed.
Quantities and dates are compared in code. The scorer is a configuration choice
behind an interface with a pinned model id.

Treat scorer probabilities as a monotone score until calibrated locally. An
independent test of Jev (900 unseen synthetic tickets, 2026-09-19) found that
miscalibration sign depends on question type (choice and score overconfident,
noul underconfident), so calibrate per type; probabilities are rounded to 0.01
and often exactly 0 or 1, so clamp before logging; the separate confidence
field never beat the max probability. Two calibration loops: **agent
calibration** (resolved records label the agent's predictions; Brier per agent,
model and type come free) and **scorer calibration** (audit a sample weighted
toward disagreements, fit a temperature per type and pinned version). Report
calibration with accuracy and discrimination, since calibration can improve
simply by becoming vaguer. Jev's benchmarks are self-run, not ground truth, and
its price and rate limit (about $0.042 per million input tokens, 60 calls a
minute per computer as of 2026-10-11) need rechecking.

### 5.4 Guards: how scoring gets gamed

Design-level guards; the gaming and generalization risks need more
experimental reps before any mitigation is trusted.

| Failure | Guard |
|---|---|
| Trivial, vague, hedged or easy predictions | Skill against a base rate (copying it earns zero); sharpness and resolution (Murphy decomposition) beside reliability; specificity per agent; judged questions need a criterion; for agents writing their own questions "unresolvable" costs as much as wrong (public actors get an unscorable bucket, 5.5) |
| Avoiding predictions | A desk that stops committing has not stopped failing: track dispatch volume and the Note-to-Cast ratio |
| Self-serving self-scores or answerers; self-fulfilling predictions; padded counts | Self never governs; self-favor gap; checkable answers checked by rule; questions whose outcome is the emitter's own later act are flagged and excluded from calibration; count distinct origins, not carriers |
| Scorer gaming, including in-context RL on the scorer | No desk access to a scorer; independent scores out of the acting context, or sampled and delayed; rotate second-opinion scorers; score the scorers |
| Anchoring and desk collusion | Accepted for caching; blind reading when delegating; keep the asker's probabilities out of the addressee's packet (a 2026-10-11 audit found the Precommit packet copied them; not re-verified) |
| Poisoned records or retrieval; privacy leaks; stale lessons; store growth; observation text that argues for its own classification | Provenance on every record, committed records not editable, packs from audited records; scope to owner and strip private data from shared packs; date and re-test lessons; retrieve by corrective value; scores on untrusted content are advisory, never a security gate |
| Platform failure scored as forecasting error; scorer drift | Questions on work a crash closed are `interrupted` and excluded; pin and log the model id, calibrate per type, keep a fallback scorer |

### 5.5 Track records, box scores and claims

A **box score** is a view over settlements with a scope (entity, outlet,
agent, team), window, derived topics and a governing policy; never a stored
number. A published story pins a hashed snapshot. It never shows one trust
number or a ranking of people, and a figure below its sample bar reads "n too
small". Choir publishes its own box score (support rate of cited sentences,
corrections, forecasts) like anyone's (claims design D10, recommended).

The claims design (2026-10-14, design only) extends records to the world:

- **The claim is the precommitment.** "S said Y at T" is a precommitment Choir
  records on the speaker's behalf, with a provable commit time (the capture's
  `fetched_at`, distinct from the speaker's `uttered_at` and the outlet's
  `published_at`). Only claims from scheduled, timely captures enter a track
  record; claims research dug up later are recorded but never scored, so
  nobody is cherry-picked. A reporting claim ("Reuters says the ministry said
  P") and the embedded claim are scored separately.
- **Typed by how it settles**: assertions (by later evidence; independence
  counted by origin, so relays confirm attribution, never truth), forecasts
  (Brier or log, skill against a base rate), commitments (by observed action),
  intents and values (consistency evidence only), Choir's own agents'
  precommitments (by the trace), opinions (never scored). Five uncertainties
  (attribution, interpretation, the speaker's expressed confidence, extraction,
  settlement) ride on every claim and are never folded into one number; what
  cannot be scored is shown as **unscorable**, never as pass or fail.
- **Say-do alignment** answers "can I take you at your word?": the share of an
  actor's checkable commitments that observed actions kept, with n and an
  interval, separate from whether the actions are good. Choir's own desks are
  the first population (complete action log, independent scorer); public
  actors second. Declines are claims too, sampled and run anyway.
- **Settlement order, cheapest and most independent first**: rules, blind Jev,
  research (read-only toward the world), offline panel, human sample; new
  evidence supersedes, never edits.

### 5.6 Who is tracked

Owner, 2026-10-14: individuals who are public figures or who publish for their
content to be consumed by AI (later growth idea: Substack writers with good
track records, only once the platform is ready for users), plus outlets,
organizations, institutions, APIs, and Choir's own agents and models. Not
private individuals; `actor_scope` defaults to `private` and must be set
positively before any box score exists. The owner accepted the legal risk of
scoring public actors (2026-10-14) and kept an excellence bar: attribution
evidence, several readings shown, neutrality, a right of reply that reopens
settlement, visible uncertainty. GDPR profiling of natural persons is
separately open.

### 5.7 Historical back-testing without hindsight (research, not built)

Owner, 2026-10-14: score historical statements from a later vantage point (a
1941 statement from 1942); an evaluation and a product, most valuable for the
recent past. Hindsight leaks in three
places: **selection** (famous hits and misses get extracted),
**operationalization** (criteria bent toward a known outcome) and **verdict**
(the judge answers from memory). Guards: a pre-registered census (a hashed
corpus manifest committed before extraction, `selection: archival_census`)
instead of targeted digging; criteria frozen blind by a time-locked or masked
model; settlement by rule over dated primary records (SEC timestamps, Fed
releases, Hansard sittings; vintage data for revised figures; never live web
search); tests that measure leakage rather than assume it away. Every
historical score carries a **leakage class** L0 to L3 (L3, an unmasked frontier
judge on a famous actor, is illustrative only), a `witnessed_at` replaces
`fetched_at` as commit time, and historical and live rows are never summed. The
first corpora are Federal Reserve 2012 to 2025 and S&P 100 earnings guidance
FY2018 to FY2025 (primary, dated, mostly settled by rule, continuing as live
polls), with a registered **bridge test**: does historical say-do predict live
say-do? It also gives the claims layer its missing evaluation now, not after
30-day settlements. Far history (Churchill) is an editorial showcase, not a
track record.

---

## 6. Signing and computer keys

A track record is worth something only if a prediction demonstrably came
before its evidence. Three attacks: **forge** (make a record look like this
computer said it), **backdate**, **hide** (keep losing predictions out).
Nothing here is built for records; the underlying audit dates from 2026-10-11,
so re-verify code facts before building.

**What exists.** One host-held platform key signs every computer's event-head
receipts and checkpoints. Guest keys belong to one realization, not the
computer (a fresh disk silently mints new ones), only the verifier key is
pinned, and rotation has no caller. The host operator can read every guest
key; no software-only design removes that.

**The finding that reframes the question.** Every commitment record already
passes through the computer event chain as a projection batch, and the platform
signs that batch's receipt under host time: "computer C submitted content D as
event N after head H, no later than T." A commit time is already witnessed by a
party the computer does not control. A computer key would not improve that: a
signature proves who signed, not when. (Medium confidence: the path is
code-cited, but no specific record was traced to its receipt on staging.)

| Scheme | Forge | Backdate | Hide losers |
|---|---|---|---|
| A. Unsigned, platform-witnessed (today) | Host operator | Host operator (the computer can lie in `committed_at`, not in the receipt) | The computer by selective publishing; the operator by deleting |
| B or C. Host-held or guest-held computer key, plus witness | Operator, or the computer itself, or anyone with a disk copy | Operator only; the key adds nothing on time | As A |
| C plus external anchoring of platform heads, plus public commitments (commit and reveal) | As C | Nobody before the last anchor | Nobody; unrevealed commitments count as unrevealed |

B is A with a different name on the key and plants a mislabeled authority; C
protects authorship, not time. The levers that matter most for a track record
are not keys: external anchoring stops backdating, including by Choir, and
public commitments stop selective publication.

**Plan.**

1. *Now, no new key.* Define a canonical frozen **commit body** (schema, kind,
   prediction, directive, addressee, parent, emitter kind, computer id, subject
   refs, agent, model) and store `commit_hash`, the sha256 of its canonical
   JSON; later mutation never changes it. Record the first event-head receipt
   that carried the record; treat the guest `committed_at` as advisory and the
   receipt time as trusted. Label records `platform-witnessed/v0`, unsigned;
   mint no host-held per-computer key; declare retention for receipts and
   batches behind records that may go public. The record type has no hash
   field today.
2. *Before any public track record.* Publish each commitment hash when made,
   with a platform-signed publication receipt (private editions publish only
   the hash and reveal later). Anchor platform heads on a schedule to a
   third-party timestamp (transparency log, public git, RFC 3161 or a chain;
   daily bounds backdating to a day). Give the platform key real custody:
   provision, back up, and write its rotation-overlap policy.
3. *Real computer keys (own station, after runtime confinement).* One computer
   identity key, generated at genesis, pinned with a platform-signed receipt,
   escrowed and delivered to new realizations like the privacy key, never
   re-minted; a public key history; the reducer signs `commit_hash` at mint
   (`computer-signed+platform-witnessed/v1`); rotation by old key plus platform
   countersign, revocation by the owner's passkey plus the platform; fork
   re-keying and one-shot snapshot rules before any fork or memory snapshot of a
   keyed computer exists.
4. *Verification.* Recompute `commit_hash`, find it in the pinned batch, check
   the batch against the event's payload commitment and the receipt against the
   platform key in force then. Never re-sign v0 records: that would be a
   backdated signature by construction.

Owner position (2026-10-11): wait for real per-computer keys; until then a
store-assigned commit time and content hash are the floor and records are
marked unsigned. Re-engineering (2026-10-12) defers per-computer, per-desk and
per-sub-RLM signing as a non-functional requirement. Open: platform key custody
and backup, key scope (recommended: one computer identity), whether the
operator's ability to forge or backdate until anchoring exists is acceptable,
the anchor service and cadence, who may revoke, and receipt retention against
the open Store A history decision.

---

## 7. Withdrawn or superseded

"Re-eng" is the [re-engineering decisions](re-engineering-decisions-2026-10-12.md).

| Withdrawn or superseded | Date, deciding source | Replacement and reason |
|---|---|---|
| The `Expectation` shape (1 to 4 count, date or choice questions, a `By` time, a resolver) | 2026-10-12, Re-eng 3 | Casts carry hypotheses, open uncertainties and general hypotheses; effort follows uncertainty times stakes. A count question with a deadline leaves no room for learning and says nothing about how hard to work. |
| Runtime count resolution: a closed predicate vocabulary (`answered_by`, `new_sources`, `domain_match`, `incorporated`, `cited_next_revision`, ...), a runtime resolver, "count governs every countable question" | 2026-10-12, Re-eng 1.2, 9, 11 | Intelligence lives in models, not the runtime: no runtime scoring. A model, Jev or an RLM resolves and scores, often from several perspectives. Rule checks survive only as one scorer type (8.3). |
| Stations P3 (predictions mandatory on every Cast, Ask, Escalate) and P4 (runtime scoring, desk-visible calibration block from count-governed questions) | 2026-10-12, Re-eng 11; protocol metamission | P3 replaced by hypothesis-shaped Casts in Cut 1; P4 pending, to be re-authored. The owner's "predictions are mandatory" survives as: a request for work states its hypotheses and uncertainties. What a desk sees of its own record is open (4.7). |
| Harness-run Jev scoring of every record with a 25% blind shadow, answerer and self answers as a default pipeline, accuracy bars (95% answerer, 85% Jev) that move the governing scorer, weekly panel audits | 2026-10-12, Re-eng 11 | Multi-scoring with scored scorers (owner, 2026-10-14). Kept: measure scorers against ground truth, audit disagreements. Dropped: the fixed pipeline and thresholds. |
| `choir.Assign`, `Message`, `Emit`, Texture controls that opened work, management routes writing Texture content; "Notes may not open work" as a P3 guard | Re-eng 9; P2 landed 2026-10-11 | One surface (Cast and Ask); only Texture writes a Texture. Note-as-evasion stays a monitored failure (5.4). |
| Mutable work items, delivery packets and status machine; the directive inbox (shape D); the staged P1 to P5 cutover, drain-on-old-kinds, version-1 tray replay | Re-eng 2, 10, 12 | Append-only records, a cursor per inbox, status as a fold; three radical cuts, no backwards compatibility until the first public Autopaper permalink. |
| "Sources never precommit" (original wording) | 2026-10-14, claims design D1, awaiting ratification | Sources never emit records; Choir records their claims as attributed precommitments; a source's record is a view over settled claims with denominators, never one number. |
| Self-development as a code-level concept; Jev holding reports | Re-eng 9 | Integral and diffuse (2026-10-14: built on PSA, not another app builder); Jev passes reports through or management restates; Jev never holds. |

---

## 8. Status, open questions, research program, next steps

### 8.1 Status (2026-10-14)

**Built** (code and `docs/ACTIVE.md`): the typed record and five-kind act
taxonomy; append-once persistence on the object graph; derived views (tallies,
supervision projection, score-free and score-carrying packs, a gate refusing a
learning claim that cites no scored records); the desk verbs; one Cast and Ask
surface (P1 and P2, 2026-10-11); and from re-engineering Cut 1 (core landed
2026-10-13) records as the work state: inbox sequences and cursors,
conversations, rounds, hypothesis ids with open, settled and dropped states,
crash closure as a system record. Engineering runs on records (C1e). Autopaper
has been live on this substrate since 2026-10-13.

**Not built**: settlement and scoring of hypotheses on the live paper; routine
independent scoring; automatic pack injection; record query verbs in the REPL;
`commit_hash`, signing and publication of commitments; the claims layer, box
scores and say-do for public actors; historical back-tests. **Pre-outcome
ordering is not proven**: a record carries its commit time and precedes any
resolution that references it, but the system does not show that the
committing agent lacked access to the resolving evidence, the hard requirement
of 1.1. The 2026-10-11 problem record found desks wrote no `Precommit` or
`Resolve` at all; the protocol made records the only path, but "records exist"
is not "records settle". None of H1 to H5 has been run.

### 8.2 Next steps, in the owner's order (2026-10-14)

1. PSA scoring first: the desk feedback loop and the claims plan's E0 (offline
   study; it can stop the layer) and E6a (scoring our own desks' say-do from
   the trace; moved to the front), alongside the capsule build unblock. Make
   records and settled scores first-class in each desk's REPL.
2. Station L3, hypotheses settled and scored (intent; entry criterion a
   recoverable off-host ledger copy): Cast hypotheses carry `resolves_by` of
   seven days or less and a criterion; settlement and scoring are separate
   records by different actors; Brier per desk as a view. Acceptance: ten or
   more hypotheses committed before their evidence, at least half settled with
   evidence refs.
3. The self-development gate, built on PSA (self-supervising, self-learning,
   not another app builder).
4. Signing step 1 inside the protocol work; anchoring and public commitments
   before any public track record. Back-test H0 beside E0 (green, offline). The
   owner expects about a month of live operation before track records are worth
   anything.
5. Adoption (memo): ship it in Choir so real logs and calibration curves are the
   evidence; publish the record format as an open spec; offer audit trails and
   say-do checks to enterprises.

### 8.3 Open questions (conflicts and unmade decisions)

1. **Rule checks versus "no runtime scoring".** The ledger metamission keeps "no
   runtime reasoning budgets, round caps or runtime scoring"; the owner's
   2026-10-14 scorer list includes "rules"; the claims design settles
   assertions "by count" with no model call over a closed set of action
   classes. Reading used here: rules write attributed score records as one
   scorer among several, never the runtime's authority over work. Needs owner
   confirmation.
2. **What a desk may see of its settled record** (4.7): the 2026-09-29 boundary
   versus the 2026-10-14 intent.
3. **Are hypotheses mandatory on a Cast**, and does the reducer refuse one
   without? Re-eng 3 gives the shape, not a refusal rule.
4. **Ratifications**: claims D1 (amend "sources never precommit"), D6 (whose
   records are shown; GDPR profiling of public figures) and D13 (scoring
   defaults) gate public-actor scores.
5. **Scores as separate records** is decided, but the type keeps an in-record
   `Scores` slice and the deployed score-record kind is not confirmed here; old
   records are not to be migrated.
6. **Memo decisions never closed**: free-form question wording; material action
   classes and spending threshold; default scorer per event type and which
   events get a second opinion; retention of observation excerpts; the surprise
   threshold for a revision; circuit-breaker thresholds per materiality class;
   which part of the record spec to publish and when.
7. **Signing**: the open items in section 6. **Retract** is deleted per Re-eng
   13 but still in the type; verify.
8. **Conjectures** HB-1 (back-tests reach L0 to L1 for at least 70% of
   checkable post-2012 claims; historical say-do predicts live say-do) and CE-1
   (a fixed claim schema beats labels-only on merge recall), falsifiable by H0
   and E0.

### 8.4 Research hypotheses and design constraints

PSA's hypotheses, tested separately, none run: **H1** prospective records
capture diagnostic information that retrospective reflection, final grading and
traces omit; **H2** retrieval and harness optimization from resolved
commitments improve out-of-sample research at controlled cost against
memoryless, reflection-only and random-episode controls; **H3**
commitment-versus-action discrepancies find consequential violations where
internal reasoning is unavailable, against tool-trace audits and CoT
monitoring; **H4** a prior, domain-specific history of resolved records
predicts future performance better than success rates, benchmarks, popularity
or uncalibrated ratings; **H5** scores stay useful under strategic omission,
vague forecasts, self-fulfilling predictions, evaluator exposure and collusion.
Success is a discriminating result on fresh tasks with stable calibration and
bounded cost. Failure is informative: scores optimizable without better
outcomes, records restating the obvious, commitments turning uninformative
under monitoring, reputations that do not transfer. An observer must not infer
a failure mode vanished because an agent stopped committing about it.

Constraints from the September predictive-ICRL review that still stand:
objectively resolvable classes (code and tests, retrieval facts, dated events)
carry the headline claim, with semantic reading a weaker secondary track;
prediction is also an anchoring device, so keep maintained parallel hypotheses
(realized by hypothesis ids); score the *choice* of prediction (expected
information gain) or the system converges on safe trivia; burn in with simple
retrieval before the meta-layers (scoring the scorer) switch on; verbalized
confidence must be informative for the frozen checkpoint (Kadavath 2022; Lin
2022); claim only the delta over forecasting prior art (Halawi 2024 is the
baseline). The cheapest first experiment is predict-before-act in RLM cells
scored by tests and the compiler. Support: the pretesting effect (Richland
2009); failure mode: outcome bias (Baron and Hershey 1988). Its ontology
questions are resolved: episodes live on the object graph and tape, local by
default and published to Store B by choice.

---

## 9. Lineage: where each archived source now lives

| Archived source (`docs/archive/precommitment/`) | Where its content lives here |
|---|---|
| `Precommitment Records — Engineering Memo.md` (owner, 2026-09-22; the base) | 1.1 to 1.3, 2.1 to 2.3; 3.1; 4.5, 4.7; 5.2 to 5.4; 8.2, 8.3 |
| `Precommitment Records Theory and Implications.md` (owner paper, 2026-09-22) | 1.1, 1.2; 2.1, 2.4; 3.1 to 3.3; 5.4; 8.4 |
| `precommitment-records-theory.md` (condensed duplicate) | No unique content; it dropped the neutral-probe subsection at owner direction (2.4) |
| `prospective-self-alignment-2026-10-09.md` (PSA paper) | 1.5, 3.2 (framing, caveats); 1.3, 5.5 (reputation); 8.4 (H1 to H5); 8.1 (status, updated) |
| `precommitment-protocol-design-2026-10-11.md` | 4.1 to 4.3, 4.6; 8.1. Its sections 4 and 5 are withdrawn (7) |
| `precommitment-predictions-and-scoring-design-2026-10-11.md` | 4.6, 5.1, 5.2, 5.4. Its count vocabulary, resolver, Jev pipeline and decision rules are withdrawn (7) |
| `precommitment-signing-and-computer-keys-2026-10-11.md` | Section 6, without the dated code-audit citations at main@4b114e61 |
| `memo-predictive-icrl-delta-2026-09-10.md` | 8.4 |

Live documents pointed to, not absorbed:
[claims, events and entities](reports/claims-events-entities-ontology-2026-10-14.md),
[historical back-testing](research/historical-precommitment-backtesting-2026-10-14.md),
[re-engineering decisions](re-engineering-decisions-2026-10-12.md),
[owner notes](reports/owner-notes-2026-10-14.md),
[ledger metamission](definitions/choir-ledger-metamission-2026-10-13.md),
[computer ontology](computer-ontology.md), `skills/throughline/SKILL.md`. Code:
`internal/types/commitment.go`, `internal/store/inbox.go`.

## References

Load-bearing citations (full bibliography in the archived PSA paper): Dawid
(1984), prequential approach, JRSS A 147(2); Gneiting and Raftery (2007);
Monea et al. (2025, arXiv:2410.05362); Song et al. (2025, arXiv:2506.06303);
Pan et al. (2024, arXiv:2402.06627); Turpin et al. (2023, arXiv:2305.04388);
Lanham et al. (2023, arXiv:2307.13702); Korbak et al. (2025, arXiv:2507.11473);
Baker et al. (2025, arXiv:2503.11926); Greenblatt et al. (2024, ICML);
Halawi et al. (2024, arXiv:2402.18563); Karger et al. (2024,
arXiv:2409.19839); Kadavath et al. (2022); Lin et al. (2022); Richland et al.
(2009); Baron and Hershey (1988); Dellarocas (2003). Jev: TypeSafe (2026-09-15),
the Valyu guide (2026-09-17), the independent calibration test (2026-09-19).
