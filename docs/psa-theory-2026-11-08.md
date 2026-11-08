# Prospective Self-Alignment: the theory (2026-11-08)

Status: written at MA8.0 of the modular architecture mission, from the plan
([modular-architecture-plan-2026-11-04.md](design/modular-architecture-plan-2026-11-04.md))
and the mission ledger
([choir-modular-architecture-2026-11-04.ledger.md](definitions/choir-modular-architecture-2026-11-04.ledger.md)).
It describes what main does at the date above. Where the plan said more than
main does, this document says so and marks the item open (section 9).

PSA (prospective self-alignment) names the theory and the science. Precommitment
records name the practice and the engineering; that half is in
[precommitment-records-practice-2026-11-08.md](precommitment-records-practice-2026-11-08.md).
The earlier, longer statement of the idea is
[precommitment-records.md](precommitment-records.md), sections 1 and 3. This
document extends it and replaces one claim of it (section 4). The owner writes
the paper at external launch from this document and from the artifacts of the
MA7.9 harness.

## 1. The claim

An agent that states, before it acts, what it expects to happen, how likely it
thinks that is, and how it will be settled, gives its principals something they
can score. The record of those scores is a track record. A track record is
evidence about how far the agent can be trusted with the next, larger action.
Alignment becomes a capability problem: does the agent predict the consequences
of its acts well enough to be given the act?

Three things follow, and the build tests each.

1. A message between agents is a prediction, not free text. It carries the
   sender's probability that the request achieves its outcome, the sender's
   proposed materiality, and the rule by which it settles.
2. Authority is earned in steps. A package declares a ceiling at install. Leases
   inside the ceiling are granted for a task or, on a strong record, for longer.
   The track record sets the length and scope of a lease. It never raises the
   ceiling.
3. Generated code is stranger code. A desk the owner's computer wrote for itself
   gets the same treatment as a package from a stranger: a ceiling, leases, a
   record. There is one trust model.

## 2. Custody and leases: authority as an earned grant

Each data kind has one custodian desk with sole read and write. Texture is the
custodian of document revisions; the Mail agent is the custodian of email. A
desk that is not the custodian reads or writes the kind only under a lease from
the custodian. Two layers decide what a desk may do:

- The **ceiling** is the capability list in the desk's manifest, reviewed at
  install and shown in the Library. Nothing the desk does can exceed it.
- A **lease** is a scoped grant issued by the custodian inside the ceiling. It
  names a kind (or a class of captures), a purpose and an end. A lease ends with
  the run that asked for it, at the next boot (a crash never resumes a lease),
  when the owner revokes it, or when the holder is retired.

The trust record decides how long a lease runs and how wide it is. The original
draft said trust never adds a capability. The revision (R20) says trust never
exceeds the ceiling, and governs leases within it.

Scores are statistics. A low score is a low score; nothing in the system is a
punishment. Decisions do read the statistics: lease duration and scope, whether
an act is autonomous or goes to a gate, the order in which a recipient admits
requests, and the depth of install review. That is selection on a public record.
The agent keeps its ceiling and may ask again.

## 3. Messages as predictions

Every request between desks (a cast, an ask, an escalation) is a precommitment
record. The sender states `p_success`, a proposed `weight` (materiality) and a
settlement rule from a closed vocabulary, or "recipient" when the recipient's
answer is the observation. Provenance (agent, model, desk, package and version,
run, origin class) is stamped by the reducer, never supplied by a cell.

Authority never travels with a request. The recipient decides on its own
grants. A cast to engineering from a desk that does not hold the admission grant
is forwarded to management's admission ledger, which decides.

A sub-call is a cast. A child agent that does work for a desk is reached by a
cast carrying the parent's precommitment, and no sub-call is exempt from scoring
(R28, R29). Settlement is asynchronous: a System 1 judge answers a closed
question with a probability distribution, and a judge failure leaves a record
unsettled, never a fan-out blocked.

An **inert** precommitment is one no plausible outcome could contradict. Main
detects inertness by rules and a sampled System 1 question, reports the rates per
desk, model and prompt version, and puts the count in the desk's calibration
digest. It does not refuse a cast for being inert.

## 4. Materiality, frozen at commit time (erratum to precommitment-records 3.1)

The earlier doc said the reference set `C*` is built partly from what actually
happened. That is superseded for scoring (R32, R38). If the yardstick is chosen
after the outcome, an agent that lists every disaster looks best whenever one
happens, and the score is no longer proper.

At commit time the reducer stores and hashes, with the record:

- the reference consequence set `C*`, and for each consequence its `P_ref`,
  impact class and relevance;
- the full output of the reference predictors and the critic panel, including
  what fell below the floor;
- the **realized-materiality procedure**: how and when realized materiality will
  be measured. It comes from a per-kind catalogue kept by the custodian of the
  record's kind, not from the committing desk. A kind with no procedure gets no
  importance score, and the record says so.

Nothing that happens later changes any of these. A measure fixed in advance and
read afterwards is hindsight, and it is fair. A changed procedure after commit
fails the record's hash.

`P_ref` comes from outside the agent: a base rate for the action class, a blind
System 1 question, the critics. The agent's own probabilities are what is scored.
**Relevance** is the decision flip rate: call the deciding desk or gate twice on
the same action, once with the consequence revealed and once without, and take
the share of verdicts that flip. A prediction true of every action never flips a
verdict, so its weight is zero. Impact stays an ordinal class, I0 to I4 (0, 1,
3, 10, 30 as starting numbers), combined late so another table can be replayed.

### The three scores

1. **Accuracy (foresight).** A proper score (Brier or log) of the agent's
   probabilities on `C*` and on every consequence it named, each weighted by the
   frozen weight. A consequence in `C*` the agent left out scores as predicted at
   zero. Negative skill is not clipped in the score; clipping is display only.
2. **Importance judgment (hindsight enters here).** The agent's materiality
   weight is a forecast of what the frozen procedure will measure. When its
   window closes the measured value is binned into I0 to I4 and the forecast is
   scored with a ranked probability score. It never rewrites a settled score.
3. **Coverage (a foreseeability test).** A consequence that turned out material
   and that the agent did not list counts against the agent only if the
   reference predictors, the critic panel or another desk's dated, scored record
   named it before the commitment. If nobody did, it is a **world surprise**. It
   counts against no desk, extends the watched-consequence ontology, and is
   evidence about the critics' coverage.

Trust reads scores 1 and 3. Score 2 is shown beside the posterior with its n and
interval and moves no posterior or lease. It may enter trust later, in
aggregate and by an owner decision, only if replay shows it predicts the next
material failure better than scores 1 and 3 alone. Learning and selection read
all three.

Four flaws in the first draft's arithmetic were fixed (R34): weights that
depended on outcomes, negative skill clipped at zero, large weights added as raw
pseudo-counts, and the agent's own probability sitting inside its weight.

## 5. Bayesian trust

The trust view is derived, never stored. For each subject (a desk, a desk on a
model, a package, a publisher, a scorer, a reviewer) and each action class, it
reads the evidence and returns a posterior and a lower credible bound.

Evidence classes: E1 outcome, E2 coverage omissions, E3 fidelity (declared
versus used, read from the frame log), E4 refusal by the frame check, E5 peer
acceptance or contradiction, E6 owner acts (approve, reject, revert,
uninstall), E7 platform events (yank, forced rollback), E8 cost estimate versus
metered spend. Negative evidence is stronger than positive: omissions weigh 5,
fidelity and platform events 20, refusals 5 to 20 by the refused grant's impact.
Evidence decays at 0.995 a day. A new subject starts from a hierarchical prior
that shrinks toward its desk, publisher or package family, and a change to the
desk's behaviour surface discounts its old record. Two weighting candidates, a
cap and a tempering power, are both computed; replay of settled records picks
one. These numbers are starting policy, not validated constants.

Excluded from the posterior: crash-closed obligations, self-fulfilling evidence
the emitter produces itself, and scoring by a scorer of the same publisher.

The impact classes carry thresholds tau (0.80 for I2, 0.95 for I3, 0.99 for
I4) on the lower bound. A non-first-party desk's act of class I2 or above is
autonomous only when its lower bound clears tau with no open breaker. I4 is
never autonomous. Otherwise the act goes to the owner. A forged frame trips the
breaker for every class of the desk, revokes its leases at once, and shows as E3.

## 6. Scoring the scorers: anchors

Scorers (the System 1 judge, peer desks, panels, reviewers) are scored. The
regress stops at anchors, which are not scored: mechanical rules, unannounced
random spot-checks settled slowly by the strongest judge available, and
world-resolved predictions. Two levels, no more. A question type with no anchor
reports its scorer's reliability as unknown.

A scorer's quality is a profile (discrimination, calibration per question type,
bias, test-retest consistency, exploitability), read against spot-checks and
never from agreement with other scorers. Cheap System 1 scores run on
everything and a small anchored sample corrects them; prediction-powered
inference combines the two into an estimate with an honest interval. Judges of
one family are not independent, and the error correlation between scorers is
measured, not assumed. Where a desk predicts the result of its own action, a
small share of gated choices is randomized and the propensity logged, so scoring
can reweight.

## 7. Goodhart: tiers, visibility, satisfice then diversify

No single score is maximized. Metrics sit in four tiers: hard invariants (rules,
not metrics: no restart mid-apply, the keep set intact, no secret leaked,
rollback available); floors (pass or fail over a rolling window); an exploration
budget above the floors (zero while any floor fails, about 10 percent of build
capacity when all pass, never below 5 percent once open); and never-optimized
ledgers (revenue, read only for a quarterly check).

Floors for self-development: F1 the share of changes rolled back or hotfixed
within 14 days; F2 defects that escape to the owner per active app-week; F3
median time to restore; F4 honest predictions (Brier skill at least zero against
the base rate, say-do fidelity, abstentions reported); F5 a sampled mergeability
audit.

What an agent may see matters as much as what is measured. Mechanical outcomes
and its own prediction error are visible to a desk. Floor status is visible as
pass or fail with no margin. Held-out audit results, scorer identity and rubric,
owner signals and the revenue ledger are not visible. A surprising success on a
visible measure with no matching movement in the hidden audit sends an auditor.

Above the floors, selection is a lottery among the candidates that pass,
weighted toward diversity (a quantilizer over the top fraction `q`). This is the
lead hypothesis for the paper. It is pre-registered and untested until
self-development resumes (Phase 8).

## 8. Hypotheses and what would falsify them

Pre-registered in MA0.6 and written to the mission's precommit file before the
evidence existed.

- **H4, the trust conjecture.** The lower bound of a materiality-weighted,
  calibrated track record predicts the next contradicted or fidelity event
  better than a fixed allowlist, a popularity signal (install counts), a
  success rate, or an uncalibrated rating. Falsified if, on replay of settled
  records, the lower-bound ordering has no better AUC than those baselines, or if
  the gate would ask the owner for more taps a day than the owner will answer.
- **Inertness.** Naming the material-precommitment standard in the prompt lowers
  the inert rate per desk, model and prompt version. Falsified if the rate does
  not move once the prompt changes.
- **Relevance by flip rate.** The flip rate is stable on repeated samples and
  separates consequences that matter from those that do not. No prior paper
  measures relevance this way for agents; this may be a contribution. Falsified
  if flip rates are noise on resampling.
- **Satisfice, then diversify.** A lottery among passing candidates beats
  always taking the top score. Untested.

The MA7.9 harness runs the replay tests, the baselines, and the red-team drills
that plant known-bad packages on d@e only, and writes a dated artifact that
reproduces byte for byte from its stored inputs. Its first runs (2026-11-06) read
a trust view of 18 rows and 26 replay pairs; P1, P2 and P5 stayed unresolved
below their minimum n because no material I2-or-above act had settled. Results
append to the pre-registration whatever they show.

## 9. What is built and what is open

Built on main: the request record and its closed act set; the one address
resolver; sub-call casts and the judge settler; inertness measurement; the
protocol lock (desk protocol 14.0.0); custodian-per-kind and the writer and
reader gates (package kinds enforce); leases; the disclosure gate with its
owner tap; the desk registry and grants; gVisor around every desk worker; the
trust view, lease policy, act gate and breaker, install review with reviewer
precommitments, publisher reputation as sentences, the MA7.9 harness; and the
library half of the anchor machinery (seeded sampler arithmetic, scorer
grading, prediction-powered intervals).

Open:

- **Built-in kinds are counted, not enforced.** The writer and reader gates
  enforce for package kinds and owner-private captures for package desks. For
  the built-in kinds (Texture, mail, the rest) they count violations and allow.
  The counters reached zero on a representative day, and the enforce flip is
  not taken.
- **MA4.11**, the deletion of the desk-name switches. Management's lifecycle code
  and the profile-name keys still exist, and the names gate keeps a Texture
  residue until Texture's desk moves with its app (plan section 18 item 6).
- **The anchor question as a desk act.** The live anchor sampler called an LLM
  from the computer runtime outside any desk. Under the owner's ruling of
  2026-11-08 (System 1 models outside desks are fine, LLMs are not) it was
  reverted. No live anchors are served, so trust-harness probes P6 and P7 have
  no anchors. The install review seats make the same kind of call and are the
  other site left to move onto a desk.
- **Phase 8**, resuming self-development under the floors and the satisfice-then-
  diversify selection, with its hypothesis test, has not started. Autopaper
  work waits for the end of this mission (R37).
- The trust parameters (impact numbers, tau, decay, the change-discount range)
  are starting policy. The replay of settled records decides the weighting mode
  and the tau values.
