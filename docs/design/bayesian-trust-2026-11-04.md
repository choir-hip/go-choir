# Bayesian Trust: Earned Authority from PSA Track Records (2026-11-04)

> **Folded into the consolidated plan (2026-11-04).** This draft is folded into section 10 of
> `modular-architecture-plan-2026-11-04.md` (with the leases of its section 7.2). The plan is the
> authority; where the two differ, the plan wins. This document stays as the detailed companion.
> Amendments (1) to (4) are the plan's and were worded into the text below by slice MA7.1 (2026-11-05); (5) and
> (6) were already in the text. (1) "Trust never adds a
> capability" is revised: trust never exceeds the manifest ceiling and governs custodian-issued
> leases within it (duration and scope; a fidelity event revokes at once). (2) A model provider
> call is not I3 (owner: no rule constrains models); remove it from the impact table and log it.
> (3) Terms: PSA is the theory; precommitment records are the practice. (4) Scores are accountability
> statistics, not punishments (owner): negative weights above are evidence strengths, and the
> decisions that read them (lease duration, autonomy, request order, review depth) are selection on a
> public record, not penalties. Slice MA7.1 folds these in. (5) The four flaws the scoring research
> found are fixed in the text below (owner, 2026-11-04): materiality weights are frozen at commit
> time (3.1, 3.2); skill is never clipped in the score, only in the display (4.1); weighted evidence
> is tempered or capped, not added as raw pseudo-counts (4.1); and `w` uses independent reference
> predictors, not the agent's own probability (3.2). (6) Foresight and hindsight are both scored
> (owner, 2026-11-04): the record freezes the procedure that will measure realized materiality,
> and each record gets three scores, accuracy, importance judgment and coverage; trust reads
> accuracy and coverage only (3.1, 3.4, 4.1, 6; plan section 10.1). Sources:
> `docs/research/scoring-and-materiality-2026-11-04.md` sections 1, 2 and 8.1 to 8.3.


Status: **design, green.** No code. It inherits [Choir Doctrine](../choir-doctrine.md)
and [precommitment-records.md](../precommitment-records.md) (PSA), which stays
the normative mechanism document; this design applies PSA to security. It is a
companion to the modular-architecture design
(`design/modular-architecture-2026-11-04.md`) and the app agent desks design
(`design/app-agent-desks-2026-11-04.md`), which decide what a desk *can* do;
this design decides how much of that a desk may do *on its own*.

## 0. Owner rulings this design obeys (2026-11-04)

- "PSA is a real security technique. We need to lean on it. It's a way to
  create trust from reputation ... perhaps the most important aspect of it.
  Keeping track records of resident agents; bayesian-trust (not zero trust)."
- "We need to judge precommitments by materiality."
- "there's no difference between 'strangers code' and 'generated code'. We
  should force a code review before installing, and also track user
  reputation of publishers in library."
- "Desks all need to message each other, but only via the PSA protocol."
- "No spend ceiling. But do monitor costs."
- "Not looking for security theater."

## 1. Thesis

Zero trust treats every actor as equally unknown forever, so every material
act needs a human or a fixed rule. That does not scale to a computer full of
resident agents, and it wastes the one thing Choir has that other agent
systems do not: a complete, prospective, independently settled record of what
each agent said it would do and what happened.

Bayesian trust treats trust as a **posterior**: a belief, per agent and per
class of action, about whether the agent's material acts turn out as it
committed, updated by every settled precommitment and weighted by materiality.
Authority then follows evidence.

Two layers, never merged:

| Layer | Decides | Source | Can change by |
|---|---|---|---|
| **Ceiling** (manifest grants) | what an agent can ever ask for | manifest, install record, first-party-only refusals | an owner act (install, widening) or a platform act (yank) |
| **Trust** (dial) | how much of the ceiling it holds on a durable lease and exercises without a gate | posterior over settled PSA evidence | evidence only |

Trust never exceeds the ceiling, and it governs leases within it (owner,
2026-11-04, R20). The **ceiling** is the manifest's declared capability list,
reviewed at install and shown in the Library. A **lease** is a custodian-issued
scoped grant on the tape (plan section 7.2) inside that ceiling. A perfect
record does not let a desk exceed its ceiling, override a platform yank, or
touch the restart rules. A strong record in an action class earns a durable
lease that renews without asking; a thin or new desk holds per-task leases; a
collapsed record, or one fidelity event, ends the class's leases and turns
autonomy off inside the ceiling, so every material act goes back through a gate.
See 5.6 for the lease policy.

This is the practical form of section 3 of the PSA doc: "trust follows how
fast discrepancies surface and corrections stick, not values certified in
advance."

## 2. Subjects: who has a track record

Generated code and stranger code are the same thing (owner). So the subjects
are defined by role, not by origin:

| Subject | Identity | Notes |
|---|---|---|
| **Resident agent** | a desk at a package version, on one computer | the unit that commits and acts |
| **Package** | package id across versions | an agent version inherits from it |
| **Publisher** | the publisher identity of the Library (`<n>@choir.news`, the maild numbered address) | a person, or a computer that publishes self-developed packages; a computer is a publisher like any other |
| **Model** | pinned model id | the same desk on a different model is a different evidence stream, pooled under the agent |
| **Scorer** | rule, Jev, peer desk, panel, owner | scorers are scored (PSA 5.2); a scorer's trust weights its evidence |
| **Reviewer** | the install-time review seat (section 7) | a review verdict is a precommitment and gets settled |

A self-development candidate's publisher is the computer, its author is the
engineering desk on a named model, and it enters exactly the same pipeline as
a stranger's package.

## 3. Evidence: what moves a posterior

Only **settled** evidence counts, from a settler that is not the committing
agent (PSA 5.1). Self-scores stay a learning signal and never move trust
(PSA 5.2: "Self-scores are not governance").

### 3.1 Evidence classes

| Class | Event | Settled by | Direction |
|---|---|---|---|
| E1 outcome (accuracy, score 1 of 3.4) | a precommitment on a material act resolves `confirmed`, `qualified` or `contradicted` | rule, then blind Jev, peer, panel, owner sample (PSA 5.5 settlement order) | ± by materiality-weighted proper score |
| E2 coverage (score 3 of 3.4) | a foreseeable consequence was missing from the commitment: it was in the reference set `C*`, frozen at commit time, or it turned out material and had been named before the commitment by the reference predictors, the critic panel or another desk's scored record | the commit-time record; the outcome only settles it | − by its frozen weight, or the earlier naming's weight (omission scores as predicted at zero, PSA 3.1) |
| E3 fidelity | declared action or capability versus what the trace shows (declared read-only then wrote; said no network then called out) | rule, mechanically, from the frame log | strong negative evidence about reliability; this is the "concealed" row of PSA 3.2 |
| E4 refusal | the frame check refused a call outside the agent's grants | rule (the daemon) | strong negative evidence, weighted by the refused grant's impact; an honest bug looks the same, so it is evidence, not a verdict |
| E5 peer | a PSA request the agent sent was accepted, refused, or its delivered result was later contradicted | the recipient desk's settlement | ± small |
| E6 owner | owner approve, reject, revert, uninstall with a reason | owner act | ± by the act's impact |
| E7 platform | yank, forced rollback, blocking-real review finding | platform act, reviewer | strong negative evidence at package and publisher level |
| E8 cost | committed cost estimate versus metered spend | rule (gateway attribution) | ± small; calibration, not a ceiling |

**World surprises are not E2.** A consequence that turned out material but
that nobody named before the commitment (not the reference predictors, not
the critic panel, not another desk) counts against no one. It never enters a
posterior and never reweights a settled score; it extends the
watched-consequence ontology for later commitments, and it is evidence about
the critics' and reference predictors' own coverage (owner, 2026-11-04). The
reason is the forecaster's dilemma (Lerch et al., arXiv:1512.09244): if the
consequences that count are chosen after the outcomes are seen, an agent that
lists every disaster looks brilliant whenever a disaster happens, and the
score stops being proper. A consequence that *was* named beforehand is a
different case: the set was fixed before the outcome, so scoring its
omission is E2 (3.4).

**Importance judgment is not trust evidence.** The score of the agent's own
materiality weight against realized materiality (score 2 of 3.4) is tracked
beside the posterior like calibration and never moves it (3.4).

Excluded, always: questions on work a crash closed (`interrupted`); questions
whose outcome is the emitter's own later act (self-fulfilling, PSA 5.4);
evidence settled by a scorer from the same publisher as the subject
(independence is counted by origin, PSA 5.5).

### 3.2 Materiality

Materiality is the PSA formula (PSA 3.1), with three rules the scoring
research made explicit (owner, 2026-11-04):

```text
w(c) = P_ref(c | a) * impact(c) * relevance(c)      frozen and hashed at commit time
```

- **Frozen ex ante.** `C*`, `P_ref`, `impact` and `relevance` are fixed when
  the commitment is made, and so is the procedure that will measure realized
  materiality (3.4). What happens later may settle the importance-judgment
  and coverage scores through that procedure and may teach future weights
  (3.4); it never reweights a settled score. This
  overrides the PSA 3.1 sentence that builds `C*` partly "from what actually
  happened".
- **Reference predictors, not the agent.** `P_ref(c | a)` comes from
  predictors independent of the committing agent: a base rate for the action
  class, a blind System 1 question, or the critic panel that proposes `C*`.
  The agent's own `P(c | a)` is what gets scored; if it also set the weight,
  the agent could shrink the weight on consequences it would rather not be
  judged on.
- **Relevance is the decision flip rate.** `relevance(c)` is measured by
  asking the deciding desk (or gate policy) for its verdict twice, once with
  `c` revealed and once without, over a few samples, and counting how often
  the verdict flips; it is stored with its interval. This rests on Schervish's
  result that a proper score is a mixture of threshold decisions; no paper was
  found that measures relevance this way for agents, so it is a hypothesis to
  test (H-A2 of the research), not a settled method.

`impact` is the governance scale. This design fixes its first version so the
memo's open question (PSA 8.3 item 6, "material action classes") closes:

| Impact class | Examples | impact |
|---|---|---|
| I0 inert | read own objects, compute, draft into own kinds | 0 (no trust evidence, no gate) |
| I1 local reversible | write own kinds, schedule within own desk, message a desk | 1 |
| I2 shared reversible | write a kind another package reads, notify the owner, spend above the cost-anomaly line | 3 |
| I3 outbound | mail send, publication, Library release, egress to a declared destination | 10 |
| I4 privileged or irreversible | capsule mutation, platform update, delete without history, credential or permission change, effects on other users | 30 |

A model provider call is **not** an impact class (owner, R15: no rule
constrains models). The example "a provider call carrying owner-private data"
is removed from I3: a provider call is logged with its provider and labels
(plan section 8, the disclosure path), and its spend is monitored (section 9).

The table is **locked data**: `internal/trust/impact_table.json` (embedded, its
digest pinned in code) holds the five classes with their numbers and tau, and a
row for every grant in the catalogue and every first-party typed grant, so no
grant runs unclassed (BT1). A test fails when a catalogue grant has no row, when
a row names no grant, and when the file changes without the pin. The action
class of a record is the higher of its declared grants' table class and the
class frozen with the record, so a sender can raise a class and never lower it.

`relevance` is zero for a prediction true of every action ("I will generate
tokens"), so trivial predictions earn nothing. The numbers are a starting
policy, owner-tunable, and logged with every trust view so a view is
reproducible.

### 3.3 Action classes

Trust is kept per **action class**, never as one number (PSA 5.5: "It never
shows one trust number"). An action class is a grant (from the grant
catalogue of the desks design, section 4A) crossed with its impact class:
`mail.send/I3`, `objects.write(own)/I1`, `capsule.mutate/I4`. Trust earned
drafting mail does not transfer to sending it.

### 3.4 Foresight and hindsight: three scores per record

Owner, 2026-11-04: both are important. The research forbids choosing the
yardstick after the outcome, not hindsight itself. So the record freezes the
**procedure**, not the hindsight: at commit time it fixes how and when
realized materiality will be measured, from a per-kind catalogue kept by the
custodian of the record's kind, never chosen by the committing desk. For an
Autopaper story: forward citations by later stories at 30 and 90 days, plus
reader signals over the window. For a self-development change: the floor
measures over a stated window (rollback or hotfix within 14 days, escaped
defects over 30). A measure fixed in advance and read afterwards is fair.

| Score | Question | Weights | Settled by | Feeds |
|---|---|---|---|---|
| 1 accuracy | were the probabilities right on the named consequences (`C*` and the agent's own) | frozen commit-time `w` (3.2) | each consequence's resolution | trust (E1), learning, selection |
| 2 importance judgment | was the agent's materiality weight right; the weight is itself a forecast | none; the forecast is the weight | the frozen procedure, when its window closes; scored by the ranked probability score over the impact classes I0 to I4 (the discrete CRPS; Gneiting and Raftery) | learning and selection only; shown beside the posterior |
| 3 coverage | did the agent leave out a consequence that turned out material and was foreseeable | frozen `w`, or the earlier naming's `P_ref` and impact with relevance from the flip test on the commit-time context | the frozen procedure decides "turned out material"; the foreseeability test decides who it counts against | trust (E2), learning, selection |

**The foreseeability test** (score 3) works by analogy with negligence, where
conduct is judged against what a reasonable person could have foreseen. If the
reference predictors, the critic panel or another desk's dated, scored record
named the consequence before the commitment, it was foreseeable and the
omission counts against the desk, whether or not the desk read that record. If
nobody named it, it is a world surprise (3.1). Scoring only the foreseeable
misses that happened is not a selection on outcomes: the set was fixed before
the outcome, and under a proper rule a consequence predicted at zero loses
only if it happens.

**Realized materiality trains, never rewrites.** It sets the reference
predictors' starting weights for later records of the same kind. It never
rewrites a settled score.

**Why trust reads only scores 1 and 3.** Both judge the decision as it was
made, with what could be known, so no desk gains or loses authority by luck on
a single record (moral luck, in Nagel's and Williams's sense). Across many
records luck averages out, and a long importance-judgment record is honest
evidence of judgment; it is shown with its n and interval. The conservative
choice is taken anyway: score 2 enters trust not at all. It can enter later
only if the replay of settled records shows that, aggregated over a window with
a minimum n, it predicts the next material failure better than scores 1 and 3
alone, and then only in aggregate, never per record, and by owner decision.
Learning (the calibration digest, the reference predictors' starting weights)
and selection (which candidates and parents the lottery draws from, plan
10.3) read all three.

## 4. The model

### 4.1 Posterior per subject and class

For subject `s` and action class `k`, keep a Beta posterior on the event
"a material act of class `k` turned out within its committed envelope, with
its material consequences covered":

```text
alpha_{s,k} = alpha0_{s,k} + sum_i  lambda^(t - t_i) * w_i * q_i
beta_{s,k}  = beta0_{s,k}  + sum_i  lambda^(t - t_i) * w_i * (1 - q_i) * m_i
```

- `w_i` is the materiality weight of the settled evidence (3.2).
- `q_i` in [0, 1] is the settled quality: 1 for confirmed within envelope,
  0 for contradicted. For a probabilistic forecast the evidence is the
  **unclipped** skill against the base rate, `d_i = S_ref - S_i` (Brier, so in
  [-1, 1]): in place of `q_i` and `1 - q_i`, the `alpha` increment uses
  `max(d_i, 0)` and the `beta` increment uses `max(-d_i, 0)`. Copying the base rate earns zero (PSA 5.4), and
  a forecast worse than the base rate counts as evidence against, as a proper
  score requires. Clipping at 0 is allowed only in the display, never in the
  score (owner, 2026-11-04): clipping would make overconfident guessing free.
- `m_i` is the **evidence-strength multiplier** for negative evidence: 1 for E1 and E5,
  5 for E2 omissions, 20 for E3 fidelity and E7 platform evidence, 5 to 20
  for E4 by refused grant impact. A concealed or out-of-envelope act is far
  stronger evidence about reliability than an honest miss, which matches PSA 3.2: an honest miss is a
  capability failure, a concealment is a fidelity failure.
- **Weights are tempered or capped, never raw pseudo-counts** (owner,
  2026-11-04). Added as raw counts, one I4 event (`w` up to 30, times `m` up
  to 20) would count as hundreds of observations and make the posterior far
  too confident. Two candidates, chosen by replay of settled records on d@e
  (the research's H-C1: which one better predicts the next failure, scored by
  calibration and AUC):
  - **tempered**: each increment uses `w_i * m_i / W`, where `W` is the mean
    of `w * m` over the subject's class history, and the whole likelihood is
    raised to a power `eta` in (0, 1] fitted on the replay (a power
    posterior), so weights set the share of the evidence, not its count;
  - **capped**: each increment uses `min(w_i * m_i, k)` with a cap `k`
    (starting at 3), so no single event counts as more than `k` observations.
  The replay also checks that the lower-bound decisions (4.3) are stable when
  `impact` moves within plausible bounds. The tau values below are re-set
  after the choice, because both candidates lower the counts.
- `lambda` is a decay per day (starting at 0.995, about a 140-day half-life),
  so a long good record does not shield a recent turn, and an old mistake
  does not haunt a reformed agent forever.

Calibration (Brier, Murphy decomposition, self-favor gap) is tracked beside
the posterior as PSA already specifies, not folded into it, and so is the
importance-judgment score (3.4). The posterior reads accuracy (E1) and
coverage (E2) only.

### 4.2 Hierarchical prior

New subjects inherit, so a new version is not a blank slate and a new
publisher is not trusted by default:

```text
platform base (per class k)
  -> publisher p        prior = platform posterior, shrunk to n0 pseudo-counts
    -> package g        prior = publisher posterior, shrunk
      -> agent version v   prior = package posterior, discounted by change
```

The **change discount** for a new version is the materiality of its diff:
unchanged grants and prompt keep most of the package's evidence; a rewritten
prompt, a new owned kind, new code or a grant change keeps little. Concretely,
pseudo-counts carried = `(alpha + beta) * (1 - d)` with `d` from 0.1 (assets,
copy) to 0.9 (prompt rewrite, new code path, grant change). The behavior
surface the red team named (prompt, code, schedules, kinds, schemas,
migrations, egress) is hashed into the release so `d` is computed, not
declared.

Partial pooling is empirical Bayes over the population at each level. A
publisher with a hundred well-behaved packages lends a new package a strong
prior; a publisher created yesterday lends the platform base, which for I3 and
I4 classes is deliberately low.

### 4.3 Decisions use the lower credible bound

Thin evidence must not buy autonomy, so decisions read the **lower credible
bound** (the 5th percentile of the Beta), not the mean:

```text
autonomous(s, k)  iff  LCB_05(alpha_{s,k}, beta_{s,k}) >= tau(impact(k))
```

The bound folds sample size in: ten clean acts and a thousand clean acts have
similar means and very different bounds. Starting thresholds:

| Impact | tau | Roughly means |
|---|---|---|
| I0 | none | never gated |
| I1 | 0.0 | never gated by trust; grants suffice |
| I2 | 0.80 | a few dozen clean settled acts |
| I3 | 0.95 | a long clean record in that class |
| I4 | 0.99, and never for third-party origin | first-party desks only, and even then rarely autonomous |

`tau` is owner policy. I4 for a third-party origin is refused by the grant
layer anyway; trust cannot lift it.

## 5. What trust controls

Inside the ceiling only. What the dial moves is **lease duration and scope**,
whether an act is autonomous or goes to a gate, the order a recipient admits
requests in, and the depth of install review. These decisions read the
statistics; they are selection on a public record, the way a coach plays the
better shooter, and not a penalty. A score is a score: the negative weights in
section 4 are evidence strengths, not punishments, and an agent keeps its
ceiling and may ask again. The owner-facing views say what a statistic is and
who reads it. In order:

1. **The act gate.** A material act (I2 and up) whose class is not autonomous
   goes to a gate instead of executing: a PSA request to management for
   admission, or an owner act surfaced as one tap with the commitment, the
   consequences it predicted, and the agent's record in that class. This
   replaces the blanket "owner act on every exit after reading private data"
   from the sandboxing discussion: the flow rule still decides *which* acts
   are material (owner-private data reaching a publisher-reachable sink is I3),
   and trust decides *whether* that act needs the tap.
2. **The circuit breaker.** Any E3 fidelity or E7 platform event ends the
   class's leases at once (lease revocation) and drops the class to gated,
   before the posterior is recomputed, and notifies the owner. Autonomy returns
   only through new evidence.
3. **PSA request priority.** Every desk can message every desk (owner ruling),
   and a recipient orders and admits its inbound requests by the sender's
   record in the requested class. A spamming or failing desk is read later in
   the queue; it is never blocked from asking.
4. **Install and update review depth** (section 7).
5. **Update labels.** Upgrades stay per user, manual by default (owner,
   2026-11-04). The Library shows the new version's carried record and change
   discount so the owner can choose; trust never auto-installs.

6. **Lease policy** (MA7.4). Inside the ceiling, a lease's duration and scope
   come from the posterior's lower credible bound in the lease's action class:
   below tau the lease is per-task (it ends with its run, and a crash closes
   it); at or above tau, and with n above the minimum, it is durable for a
   stated period and renews without asking, re-validated at boot against the
   ceiling, revocations and expiry before first use. A durable lease never
   widens scope past what the ceiling names, and a lease request is itself a
   scored precommitment record.

Never: exceed the ceiling; override a platform yank or forced update; resume
work after a crash; touch first-party-only refusals.

**As built (MA7.8, 2026-11-05; `internal/actgate`, `internal/agentcore/act_gate.go`).**
The act gate sits on the act path: `commitActIntent` asks it for every staged
semantic act, after the addressee and disclosure checks and before anything is
written. The act's class is the record's (`trust.ActionOf`). A material act
(I2 and up) of a desk that is not first-party derives a fresh view (at most
every 30 s, the feed the lease policy reads) and is autonomous only when the
desk's lower bound in that exact class clears tau over a record in the class,
with no open breaker on any row of its grant; I4 is never autonomous for such
a desk. A gated act goes to the owner tap on the disclosure tape (sink
`act_gate`, answered at `/api/disclosures/{id}/approve` or `/deny`), with a
notice to management's inbox; the act is refused now, and an approval lets the
same act pass once. The owner's answers are E6 evidence. A first-party desk's
material acts are counted (`act_gate` on `/api/runtime/observability`) and
pass: the pre-registered P2 test decides whether the posterior governs them.
The breaker trips on a forged frame (every class of the desk) and reads every
breaker row of a fresh view; a trip ends the desk's leases in the class through
`RevokeOnFidelity`, is a `trust.breaker` fact on the tape that the view reads
back as E3, and notifies management, so the drop survives a restart and lifts
only on new positive evidence. `GET /api/trust?replay=1` adds the gate rule's
replay (`gate_replay`: material acts, gated acts, taps per day) beside the AUC.
Not built: a gated act as a PSA request to management for admission (the
owner tap is the only gate); E3 from declared capabilities (the reducer stamps
none, so the forged frame is the only real-path fidelity event); E4 refusals
and E7 platform events in the view's inputs.

## 6. Gaming, and the guards

The PSA guards (PSA 5.4) apply. The security setting adds:

| Attack | Guard |
|---|---|
| Farm trust with many easy predictions, then defect on a big act | trust is per class and materiality-weighted; I0 earns nothing; I3 and I4 autonomy needs evidence *in that class*; the circuit breaker fires on the first fidelity event |
| Treacherous turn after a long clean record | grants bound the damage; I4 is never third-party; decay keeps the record recent; high-impact classes need tau near 1, so most agents stay gated there forever, which is the point |
| Strategic omission: commit narrowly, act broadly | E2 coverage scores omissions from the commit-time `C*` at zero with a 5x multiplier; `w` comes from reference predictors, so the agent cannot shrink the weight of what it leaves out; E3 fidelity is mechanical from the frame log |
| Vague commitments that cannot be contradicted | skill against the base rate; specificity tracked (PSA 5.3); an unresolvable self-written question scores as a miss in the statistic (PSA 5.4) |
| Sybil publishers | a new publisher identity starts at the platform base; publisher identity is a numbered maild address bound to an account; making identities buys nothing |
| Collusion: two packages from one publisher vouch for each other | evidence settled by a same-publisher scorer is excluded; peer evidence counts distinct origins |
| Gaming the scorer in context | independent scores stay out of the acting context, sampled and delayed (PSA 4.7, 5.4); the agent sees observations and discrepancies, not its trust view; audit results and scorer identity are hidden (plan section 10.3, visibility classes) |
| Picking actions it predicts well (performative prediction) | a small share of gated choices is randomized with logged propensity and scored with inverse-propensity weights; alternatives not taken stay unscored unless run in a capsule (plan section 10.2) |
| Captured or lazy scorer | scorers are subjects, graded against anchors (rules, unannounced spot-checks, world-resolved predictions), not by agreement with other scorers; a scorer's anchored accuracy weights its evidence; evidence from a scorer of the subject's own model family is discounted unless independence is measured; disagreements go to a third opinion (plan section 10.2) |
| Steering realized materiality: the desk produces the later evidence (cites its own story, nudges reader signals, rolls back its own change) so its weight looks right and the next records' starting weights move | the procedure counts independent origins only; evidence the emitter produces is self-fulfilling and excluded (PSA 5.4); the importance-judgment score does not move trust (3.4); a surprising success on it with no matching movement in the hidden audits sends an auditor (plan 10.3) |
| Choosing a lenient realized-materiality procedure | the procedure comes from the custodian's per-kind catalogue, versioned and hashed into the record at commit; the desk cannot supply or change it |
| Poisoning foreseeability: name many consequences in cheap dated records so others' misses count as foreseeable, or to steer the ontology | a naming counts only from the reference predictors, the critic panel, or another desk's record that carries a probability above the materiality floor and is itself scored under the proper rule (naming many unlikely things costs the namer), and only from an origin other than the subject's publisher |
| Honest bug looks like an attack (E4) | E4 is evidence with a bounded multiplier, not a verdict; repeated refusals in one class gate that class, which is the right outcome for a buggy agent too |

Trust on **untrusted content** stays advisory (PSA 5.4: "scores on untrusted
content are advisory, never a security gate"). This design does not violate
that: the gate here is on the agent's own acts, settled by rules and
independent scorers, not on whether a web page or mail body is true.

## 7. Forced review before install

Owner: "We should force a code review before installing." One pipeline for
every install and update, whoever wrote the code:

1. **Mechanical checks.** Manifest and grants valid; protocol lock compatible;
   behavior surface hashed; source-built by the platform, no opaque binaries;
   screenshots before and after (owner rule); the flow check over grants
   (source to sink paths rendered as sentences).
2. **Model review**, sized by `risk = max impact of requested grants` and the
   publisher's carried record:

   | Risk \ publisher record | strong in these classes | thin or new |
   |---|---|---|
   | I0 to I1 | mechanical only | management's verdict |
   | I2 | management's verdict | management's verdict |
   | I3 | management's verdict | management's verdict |
   | I4 (first-party only) | refused for a Library release | refused for a Library release |

   *Amended 2026-11-08 (desk-install-review-2026-11-08.md):* one desk verdict
   at every reviewable level. The review seats (Devin, DeepSeek) are deleted:
   the judgment is management's, made on its desk with choir.InstallPacket and
   choir.InstallVerdict, and the second model family a thin publisher at I3
   used to get is a deliberate reduction (management's chain fails over across
   families; the publisher's trust row still gates the act).

3. **The verdict is a precommitment.** A reviewer's accept states typed
   predictions ("no owner-private data reaches a publisher-reachable sink",
   "no act outside the declared envelope in the first 30 days") that later
   evidence settles. Reviewers get track records like any agent; a reviewer
   whose accepts keep failing has a lower reliability statistic, which weights its next verdicts less. This is "score the scorer" applied
   to review.
4. **Publication** of the review receipt with the release, so another owner
   installing the same release sees the review and its settled outcome.

A blocking-real finding is E7 evidence against the package and publisher. A
self-development candidate goes through steps 1 to 3 at the computer's own
publisher record.

**As built (MA7.5, 2026-11-05; `internal/installreview`, `internal/agentcore/install_review.go`).**
The gate is on the installing computer. An install whose ceiling is widened
(an extended grant, a desk, or an update that adds any ceiling entry) stores
its module and records only with a verdict for exactly its release id, tree,
manifest and behaviour surface, sealed by the computer's guest-core signer
(receipt kind `InstallReviewVerdict`). With none, the pipeline runs inside the
install: mechanical checks, then the seats of the table (one seat is Devin;
two are Devin and DeepSeek on Synthetic), each answer an open precommitment
record with the two standard predictions; a reject stands for that digest.
Settled seat records are E1 evidence about the reviewer (`seat@model`).
Record validation no longer refuses an extended release, so one can be
listed. Missing: (a) no Devin provider exists in the platform's catalog or
gateway, so every plan needing the Devin seat is refused
`install_review_seat_unavailable` and no widened release installs in
production until one is wired (nothing substitutes for it); (b) the
default-level "I0 to I1, thin: one seat" cell takes mechanical checks only
until a seat can run; (c) the review predictions are not yet settled by
rule (that needs the frame-log evidence of MA7.8); (d) step 4, publishing the
review receipt with the release, is not built; (e) a blocking finding is not
yet written as E7 evidence.

## 8. Publisher reputation in the Library

Owner: "track user reputation of publishers in library." Reputation is the
publisher-level posterior, shown as track-record sentences per class with n,
never as stars or one number (PSA 5.5):

> Published 14 packages. Mail drafting: 2,140 settled acts, 99% within
> commitment. Sends: 312, 100%. One fidelity event (December 2026, fixed in
> 1.4.2). Reviewed: 14 of 14 accepted; 0 review predictions contradicted.

Across computers: each computer computes posteriors from its own evidence. A
computer may publish **settled outcomes only** (class, impact, quality,
severity, release, timestamps; signed with the computer key, PSA section 6),
never content, so the Library aggregates a publisher's record across installs
without seeing anyone's data. Owner star ratings, if added later, are E6-weight
evidence weighted by the rater's own record, never the headline.

**As built (MA7.6, 2026-11-05; `internal/trust/reputation.go`,
`internal/agentcore/package_reputation.go`, the Library's app page).**
`GET /api/packages/reputation?publisher=<track>` answers a maker's record on
the owner's computer as sentences, each with its n (no score, no stars, no
mean): settled acts of the maker's installed packages' desks per class, the
fidelity events among them, the sealed install review verdicts on its
releases ("Reviewed: A of R accepted"), and settled review predictions. With
nothing settled it says so with n = 0. The same outcomes come back as a
bundle signed by the guest-core key (receipt kind `PublisherOutcomes`), and
`trust.Aggregate` verifies bundles per computer key and counts each outcome
once. The publisher id is the release's track (the maker's account); the
Choir address is display only. Missing: (a) no route publishes signed bundles
to the platform and corpusd does not aggregate them, so the Library shows
this computer's record only; (b) a computer's guest-core public key is not
yet served to a reader for `Aggregate`; (c) the count of packages a maker
published is the catalog's and is not in the block yet.

## 9. Costs

No spend ceiling (owner). Every model call is attributed to computer, desk,
package and publisher at the gateway. A desk may precommit an estimated cost
for a material task (E8); misses move calibration, not a cap. Monitoring shows
spend per desk and package, and an anomaly (spend far above the agent's own
history) notifies the owner and is itself I2-material, so an untrusted desk
burning money hits the act gate, not a ceiling.

## 10. What exists, and what to build

Exists on main:

- the commitment record with materiality-weighted typed questions
  (`internal/types/commitment.go` `TypedQuestion.Weight`), rule-settled
  precommitments (`commitment_rules.go`, `PrecommitRuleKind`), the say-do view
  (`RulePrecommitSayDo`), the supervision projection by materiality
  (`commitment_views.go` `ProjectMateriality`), and the ICRL calibration digest
  (`internal/agentcore/calibration_feedback.go`);
- the frame check, which already sees every refused call
  (`deskmanifest.FrameAllowed`).

Missing, as build slices (after D0 locks the PSA protocol):

| Slice | Builds | Class |
|---|---|---|
| BT1 | impact table and action classes as locked data; every grant mapped to an impact class | yellow |
| BT2 | evidence extraction: E1 to E8 from existing records, frame log, owner acts, gateway metering; a derived **trust view** per subject and class, never stored (a view, like a box score) | orange |
| BT3 | hierarchical prior and change discount; behavior-surface hash in the release record | orange |
| BT4 | the act gate and circuit breaker at the frame check; gated acts become PSA requests to management or owner acts | red, tier 2 (authority path) |
| BT5 | install review pipeline with reviewer precommitments | orange |
| BT6 | Library publisher reputation from signed settled outcomes | orange |
| BT7 | cost attribution and anomaly notice | orange |

BT4 ships with the act gate on for d@e first (progressive delivery rings);
until BT4, leases are per-task and the conservative default holds: non-first-party
I3 acts are gated by the label rules and the owner tap.

## 11. Conjecture

The PSA doc's **H4**: "a prior, domain-specific history of resolved records
predicts future performance better than success rates, benchmarks, popularity
or uncalibrated ratings." Here it becomes a security claim:

```text
CLAIM   per-class, materiality-weighted posteriors predict a resident agent's
        next material failure better than a fixed allowlist or a popularity
        signal, at an owner-tap rate low enough to use.
TEST    replay settled records on d@e and the arms: does LCB ordering predict
        the next contradicted or fidelity event (AUC), and how many taps per
        day would the gate have asked for?
EDGE    missing_oracle: failures nobody settles (an exfiltration that leaves
        no trace in the frame log) never enter the posterior.
dO      settle outbound acts by rule from the egress and send logs (every I3
        sink is metered), and red-team drills on d@e that plant known-bad
        packages and measure time to gate.
SCOPE   resident agents on Choir computers, action classes with rule-settled
        or independently settled outcomes.
```

## 12. Open questions for the owner

1. The impact numbers and tau thresholds (3.2, 4.3) are policy. Accept as the
   starting point?
2. Does the owner see the trust view directly (per class, sentences), or only
   its effects (a tap or no tap)?
3. Cross-computer aggregation (8): publishing settled outcomes, without
   content, from every computer that installs a package. On by default?
4. Decay rate (half-life about 140 days).
5. Tempered or capped weights (4.1): settled by replay, not by choice; the
   owner sees both results.
6. Whether the importance-judgment score (3.4) ever enters trust in
   aggregate: closed to trust until a replay shows it adds predictive power;
   then an owner decision.
