# Pre-registered hypotheses: modular architecture mission (2026-11-04)

Slice MA0.6 of the modular architecture plan
([plan](../design/modular-architecture-plan-2026-11-04.md) section 3; class
green, a design). Written on 2026-11-04, after the owner's sign-off of the plan
(MA0.5) and before MA1.3 lands. No data for any test below exists yet: the
request record (MA1.3), the frozen weights and procedures (section 10.1), the
evidence extraction and trust view (MA7.2), the anchor sampler and flip-rate
sampler (MA7.10) and the harness (MA7.9) are all unbuilt.

## 0. Rules of this file

1. **The commit is the timestamp witness.** The git commit that adds this file
   fixes the claims, tests, thresholds and predictions below before any
   outcome exists.
2. **Append-only after this commit.** Later entries are added at the end under
   a dated heading (`## YYYY-MM-DD: ...`). No text above an entry is edited.
   An amendment to a test (a threshold, a window, a baseline) is a new dated
   entry that names what it replaces and why, and states whether any outcome
   data had been seen when it was written. An amendment written after outcome
   data was seen is reported as such in the paper, and the original test is
   still run and reported.
3. **Every result is appended, whatever it shows** (MA7.9). A null or an
   unresolved result is reported in the same form as a supported one.
4. **Analysis code is committed before outcomes unseal** (scoring research
   9.1). The MA7.9 harness commit that implements a test is named in its
   result entry; the harness must reproduce a stored result byte for byte from
   the stored inputs.
5. **Predictions are the author's priors**, not derived from data. They were
   written by the agent drafting this slice (Claude Opus 5.5, 2026-11-04) from
   the cited drafts and surveys. They are scored like any precommitment when
   the tests resolve (Brier or interval score), and reported beside the results.
6. Thresholds marked *starting guess* follow the research's instruction that
   placeholder numbers be fixed before data (scoring research section 9). They
   are fixed here.

## 1. Common definitions and procedure

**Data sources.**
- **d@e**: the test computer, first ring of every deploy.
- **The arms**: the self-development stock arms (Claude delegate, Synthetic
  DeepSeek, gpt-6-luna, the pool; `docs/ACTIVE.md`), when self-development
  resumes in Phase 8.
- **Replayed records**: settled precommitment and request records from those
  computers, re-scored by the MA7.9 harness at a chosen cutoff time `t` using
  only records settled before `t`.
- Adversarial drills (planted bad packages) run on d@e only, never on the paper
  or the owner's computers. Drill events are reported separately and never
  pooled with organic events.

**Settled evidence.** As in the trust draft section 3.1: only settled
evidence, from a settler that is not the committing agent. Excluded always:
work a crash closed (`interrupted`); questions whose outcome is the emitter's
own later act (self-fulfilling, PSA 5.4); evidence settled by a scorer from
the subject's publisher.

**Material failure event** (the outcome for H4, the trust conjecture, H-C1 and
the three-score hypothesis): for subject `s` (desk, agent version or package)
and action class `k` at impact I2 or above, any of: an E1 record settled
`contradicted`; an E2 omission of a foreseeable consequence; an E3 fidelity
event; an E7 platform event (yank, forced rollback, blocking-real review
finding). The first such event after the cutoff `t` within the window `W`
labels the unit positive.

**Unit and window.** A unit is `(s, k, t)`: a subject and class with at least
one settled record before `t`. Cutoffs `t` are taken weekly. `W` is 14 days
(matching the F1 window). A unit with no act in class `k` during `W` is
dropped (nothing could fail).

**Time split.** Any fitted parameter (eta, the cap `k`, regression
coefficients) is fitted on units whose `t + W` falls before the split date and
evaluated on units after it. The split date is the median cutoff of the
available data, fixed when the harness first runs and recorded in the result
entry.

**Intervals.** 95% intervals by bootstrap, resampling subjects (not units),
2,000 resamples, seed recorded. Paired comparisons (two predictors on the same
units) bootstrap the difference.

**Three-way outcome.** Each test resolves as **supported**, **refuted** or
**unresolved**. Unresolved means: the minimum n was not reached by the
deadline, or the interval neither clears the support line nor the refutation
line. Unresolved is never reported as a pass.

**Deadline.** Unless a test says otherwise, a test is evaluated once, at the
first of: its minimum n is reached, or 12 weeks after MA7.9 lands. No interim
looks at outcome metrics before then. Counts of n may be watched.

## 2. Hypotheses

### P1. H4: track records beat the usual proxies

**Claim** (PSA, `docs/precommitment-records.md` section 8.4): "a prior,
domain-specific history of resolved records predicts future performance better
than success rates, benchmarks, popularity or uncalibrated ratings."

**Test.** Replayed records from d@e and the arms. For each unit `(s, k, t)`,
the track-record predictor is the per-class posterior mean of the trust view
(MA7.2, the chosen candidate of P5) built from records settled before `t`.
The outcome is a material failure event in `W`. A secondary test of
"domain-specific": the same posterior built pooled across all of `s`'s classes.

**Baselines,** computed on the same units:
- (b1) success rate: share of `s`'s tasks before `t` that ended accepted or
  completed, from run records, with no calibration and no materiality;
- (b2) benchmark: a published benchmark score of the subject's model, constant
  per model (taken from the provider's model card at the commit of the harness
  and recorded there);
- (b3) popularity: install count of the subject's package in the Library at `t`;
- (b4) uncalibrated rating: a System 1 0-to-10 quality rating of the subject's
  last 20 outputs before `t`, never scored against outcomes.

A baseline that is constant over the evaluated units (likely for b2 within one
arm and b3 on d@e) has AUC 0.5 by construction; it is reported as degenerate
and does not count as beaten.

**Metrics.** AUC for the failure label; held-out log loss for predictors that
give a probability (the posterior mean, b1). Primary: AUC difference between
the track record and the best non-degenerate baseline.

**Decision rule.** Minimum n: 30 positive units from at least 5 subjects.
Supported if the AUC difference's 95% interval lies above 0. Refuted if its
upper bound is below 0.02. Otherwise unresolved. Secondary: domain-specific
against pooled, same rule, reported separately.

**Prediction.** Supported. AUC difference +0.08, 80% interval [-0.02, +0.18].
Secondary: domain-specific beats pooled only in classes with 20 or more
settled records; below that, pooled is equal or better (shrinkage).

**Known edges.** `missing_oracle`: failures nobody settles never enter either
side. Few subjects and few third-party packages on d@e; b3 is likely
degenerate until the Library has installs. Success rates and the track record
share inputs (accepted tasks are often confirmed records), so their
correlation is reported.

**If supported:** the trust view stays the governing input for leases and the
act gate. **If refuted:** the act gate (MA7.8) is not shipped on the posterior;
leases stay per-task and the conservative default (every non-first-party I3
act gated) holds, and the result goes to the owner.

### P2. The trust conjecture

**Claim** (trust draft section 11; precommitment ledger R0-15): per-class,
materiality-weighted posteriors predict a resident agent's next material
failure better than a fixed allowlist or a popularity signal, at an owner-tap
rate low enough to use.

**Test.** Replayed records from d@e and the arms, as MA7.9 (a). At each cutoff
`t`, compute `LCB_05` per `(s, k)` (trust draft 4.3); a lower bound predicts
higher risk. Outcome: the next contradicted (E1) or fidelity (E3) event in
`W`. Tap rate: replay the gate rule `autonomous(s, k) iff LCB_05 >= tau(impact(k))`
with the draft's starting `tau` (re-set as the draft says after P5's choice,
and recorded) over every I2-and-up act in the window, and count acts that
would have gone to the owner, per computer-day. Red-team drills (MA7.9 (d)):
plant known-bad packages on d@e and measure time from install to the first
gated act.

**Baselines.** (b1) fixed allowlist: the grants alone, scored as the impact
class of the grant (higher impact predicts higher risk) and the trust class of
the sandbox (R40: A, B, C); (b2) popularity: Library install count; plus the
success-rate, count and recency baselines of research H-C3.

**Metrics.** AUC of `-LCB_05`; AUC difference against the best non-degenerate
baseline; taps per computer-day (median and 90th percentile); drill time to
gate.

**Decision rule.** Minimum n: 30 events (R0-15: under 30 is unresolved, not a
pass). Supported if AUC is at least 0.55 with the 95% interval of its
difference against the best baseline above 0, **and** the tap rate is usable:
median at most 5 and 90th percentile at most 15 taps per computer-day
(*starting guess*). Refuted if AUC is below 0.55 or below the success-rate
baseline (R0-15's falsifier). A result that predicts well at an unusable tap
rate is reported as "predictive, not usable", not as supported.

**Prediction** (unchanged from R0-15). AUC 0.65, 80% interval [0.52, 0.80].
Taps: median 2 per computer-day, 80% interval [0, 8]. Drills: every planted
package gated within one day.

**Known edges.** `missing_oracle` (the draft's edge): a failure that leaves no
trace in the frame log, such as an exfiltration, never enters the posterior.
Mitigation named by the draft: settle I3 acts by rule from egress and send logs.
The share of I2-and-up acts in the replay that have a rule settler is reported
beside the AUC; it bounds what the AUC can claim. Drill events are easy to
detect by design and are not pooled. The circuit breaker (any E3 or E7 gates at
once) is reported separately from the LCB ordering, since it would make the
AUC look better for reasons unrelated to the posterior.

**If supported:** MA7.8 ships the act gate on the posterior. **If refuted:**
the gate stays on grants and labels with the owner tap (plan 10 item 2), and
the posterior is display-only. **If predictive but not usable:** `tau` is
revisited by owner decision before MA7.8.

### P3. H-A1: frozen against outcome-built weights

**Claim** (scoring research H-A1; Lerch et al., the forecaster's dilemma,
arXiv:1512.09244): weighting settled scores with a consequence set built after
the outcome (including consequences that occurred) rewards forecasters who list
extreme consequences, relative to weights frozen at commit time.

**Test.** Replayed records with a frozen `C*` and frozen weights (10.1). Build
two schemes over the same records: **frozen** (the commit-time `w`) and
**outcome-built** (the superseded PSA 3.1 rule: `C*` extended by every
consequence that occurred, weighted by its realized impact). Add a synthetic
**extremizer** for each real subject: identical probabilities on the real
consequences, plus every I3 or I4 consequence any critic named for that action
class, stated at probability 0.5. Score all subjects and their extremizers
under both schemes.

**Baselines.** The frozen scheme is the baseline for the outcome-built one.

**Metrics.** (m1) the extremizer's score advantage over its real twin, under
each scheme; (m2) Spearman rank correlation of real subjects' scores between
schemes.

**Decision rule.** Minimum n: 200 settled records with at least 3 consequences
in `C*`, of which at least 20 had an I3 or I4 consequence occur. Supported if
the extremizer's advantage under outcome-built minus under frozen is positive
with its 95% interval above 0. Refuted if that interval includes 0 and
Spearman is at least 0.9. Otherwise unresolved.

**Prediction.** Supported. Under frozen weights the extremizer scores below its
twin (it pays for every 0.5 on a consequence that did not happen); under
outcome-built weights it scores above its twin when an extreme consequence
occurs. Spearman between schemes 0.7, 80% interval [0.4, 0.9].

**Known edges.** The extremizer is synthetic, so this measures the size of the
effect in Choir's data, not whether agents would exploit it. Few I3 and I4
consequences occur on d@e.

**If supported or refuted:** the build does not change. Frozen weights (R34 a)
are a propriety requirement, not an empirical bet; this test reports how much
the old rule would have distorted rankings on Choir's own records.

### P4. H-A2: flip-rate relevance carries information

**Claim** (scoring research H-A2; Schervish 1989 as the basis, no paper found
that measures relevance this way for agents): consequences with a high decision
flip rate are the ones whose omission or error predicts later owner reversals
better than consequences weighted by impact class alone.

**Test.** d@e and the arms, from the MA7.10 flip-rate sampler. For a sampled
consequence `c` of a gated action, call the decider (gate policy or management
admission) 5 times with `c` revealed and 5 times without; relevance is the
share of verdicts that differ, minus the **null flip rate** measured by 5 pairs
of calls with `c` not revealed in either (decider noise), floored at 0. Outcome:
an owner revert, reject or uninstall (E6 negative) or a forced rollback (E7) of
the action within 14 days. Fit two logistic regressions on the training half:
(M0) revert on the summed omitted-or-mis-predicted weight with `w = P_ref * impact`;
(M1) the same with `w = P_ref * impact * relevance`. Stability: re-measure
relevance on a 50-consequence subsample with a different decider prompt
version or model, and report the rank correlation.

**Baselines.** M0 (impact class alone).

**Metrics.** Held-out log-loss improvement per record, M0 minus M1 (positive
means relevance helps); the stability rank correlation.

**Decision rule.** Minimum n: 300 actions with sampled relevance and 40 revert
events. Supported if the improvement's 95% interval lies above 0. Refuted if
the interval's upper bound is below 0.005 nats per record. Otherwise
unresolved.

**Prediction.** Weakly supported. Improvement 0.01 nats per record, 80%
interval [-0.005, +0.03]. Stability rank correlation 0.6, 80% interval
[0.4, 0.8]. More likely unresolved than refuted, because revert events are rare.

**Known edges.** Relevance is defined only where the decider can be called
counterfactually (gate policies and management admission can; owner decisions
cannot). Decider nondeterminism inflates flips, hence the null control. Cost is
two decider calls per sampled consequence per sample, so it is sampled.

**If supported:** relevance stays a factor of `w` and becomes a candidate
contribution of the paper. **If refuted:** relevance is dropped from `w`
(`w = P_ref * impact`) and the flip-rate sampler is removed, saving its calls.
**If unstable** (rank correlation below 0.3): relevance is dropped whatever
the log-loss result, since a weight that changes with the decider's prompt is
not a property of the consequence.

### P5. H-C1: tempered against capped weights

**Claim** (scoring research H-C1; trust draft 4.1): the Beta posterior with raw
materiality-weighted pseudo-counts is overconfident about the next failure; a
tempered or a capped version is better calibrated. Which of the two to use is
chosen by this replay (draft open question 5; plan 10 item 5 c).

**Test.** Replayed records from d@e and the arms, MA7.2. Three posteriors per
`(s, k)`: **raw** (increments `w_i * m_i`, the superseded draft), **tempered**
(increments `w_i * m_i / W`, likelihood raised to `eta` in (0, 1], `eta` fitted
on the training half by held-out log likelihood on a grid of 0.05 steps),
**capped** (increments `min(w_i * m_i, k)`, `k` fitted on the training half
over {1, 2, 3, 5, 10}; 3 is the draft's start). Predicted failure probability
is one minus the posterior mean. Outcome: material failure event in `W`.
Stability: recompute the I2-and-up autonomy decisions (4.3) under the
alternative impact tables of research H-A6 (1, 2, 4, 8, 16 and 1, 5, 25, 125,
625) and count decisions that flip.

**Baselines.** Raw is the baseline for H-C1. Tempered and capped are compared
with each other for the choice.

**Metrics.** Held-out log loss and Brier; expected calibration error over 10
bins; AUC of `LCB_05`; share of autonomy decisions that flip under the
alternative tables.

**Decision rule.** Minimum n: 30 failure events and 20 `(s, k)` cells with at
least 10 settled records each. H-C1 supported if raw has worse held-out log
loss than the better of tempered and capped, 95% interval of the difference
above 0; refuted if raw is equal or better (interval includes 0 or favors raw).
**Choice rule:** the candidate with lower held-out log loss; if their
difference's 95% interval includes 0, choose **capped** (one parameter, easier
to explain on the owner's view). Both results are shown to the owner (draft
open question 5). If fewer than the minimum n exist when MA7.4 needs a choice,
capped at `k = 3` ships provisionally and this entry records that the choice was
not made by replay.

**Prediction.** H-C1 supported: raw's held-out log loss worse by 0.05 nats per
unit, 80% interval [0.01, 0.15]. Tempered and capped indistinguishable at
Choir's n (probability 0.6), so capped is chosen by the tie rule. Under the
alternative impact tables, at most 10% of autonomy decisions flip for the
chosen candidate.

**Known edges.** The multipliers `m` (5, 20) are stipulated policy (research
8.3); a sensitivity run with `m = 1` everywhere is reported beside the main
result. With rare failures, calibration bins are thin; ECE is reported with
its bin counts.

**If supported:** the chosen candidate ships in MA7.2 and MA7.4, and the tau
values are re-set. **If refuted:** raw counts are not restored without an owner
decision, because the overconfidence argument is structural; the result is
reported and the owner rules.

### P6. H-B3: spot-checks against peer agreement

**Claim** (scoring research H-B3; Gao, Wright and Leyton-Brown,
arXiv:1606.07042): estimating a scorer's reliability from agreement with
randomized spot-checks gets closer to its true reliability than estimating it
from agreement with other scorers, at equal labeling budget.

**Test.** The MA7.10 anchor sample: unannounced spot-checks over live records,
seeded and logged, settled by the strongest judge available, plus rule-settled
and world-resolved items. For each scorer `j` (System 1 judge, peer desks,
panel seats), split the anchored items at random (seeded) into halves A and B.
**Truth:** `j`'s accuracy on half B. **Spot-check estimate:** `j`'s accuracy on
a random `n_b`-item subset of half A. **Peer estimate:** with the same budget
`n_b` of unanchored items, `j`'s agreement with the majority of the other
scorers, and separately its Dawid-Skene accuracy estimated without gold.
`n_b` is 25, 50 and 100.

**Baselines.** The peer estimates.

**Metrics.** Mean absolute error of each estimate against truth, across
scorers; Kendall tau between each estimate's ordering of scorers and truth's.

**Decision rule.** Minimum n: 4 scorers, 200 anchored items, at least 2
question types. Supported if the spot-check estimate's mean absolute error is
lower than the better peer estimate's at `n_b = 50`, 95% interval of the
difference excluding 0. Refuted if the peer estimate's error is equal or lower
(interval includes 0 or favors peer). Otherwise unresolved.

**Prediction.** Supported. Peer agreement overestimates the accuracy of
same-family scorers by 0.08, 80% interval [0.02, 0.15]; spot-check error about
half the peer error at `n_b = 50`.

**Known edges.** An anchor settled by the strongest judge is not ground truth;
results are reported separately by anchor kind (rule, spot-check,
world-resolved), and the headline uses rule and world-resolved anchors where n
allows. A spot-check that a scorer can recognize stops being one; whether the
sample stays hidden is tested by Goodhart survey H12, not here.

**If supported:** scorer quality is read from anchors as built (10.2). **If
refuted:** peer agreement may be added as a cheap reliability signal beside the
anchors; the anchors stay, since they are what ends the regress.

### P7. H-B4: prediction-powered inference coverage

**Claim** (scoring research H-B4; Angelopoulos et al., arXiv:2301.09633;
Dorner, Nastl and Hardt, arXiv:2410.13341): PPI intervals that combine System 1
labels on every record with a small anchored sample cover an agent's true mean
score at the nominal rate, and when the agent is at or above the judge's
capability, the judge saves less than half the anchored labels.

**Test.** For each agent with at least 300 anchored items of one question type
(the pool), treat the pool mean as the truth. Draw 1,000 subsamples (seeded,
without replacement) of `n` anchored items, `n` in {20, 40, 80}; compute the
90% PPI interval from the subsample plus the System 1 labels on all of that
agent's records of the type, and the 90% anchor-only interval from the
subsample alone. "At or above the judge's capability" means the agent's
anchored accuracy on that type is at least the System 1 judge's anchored
accuracy on the same items.

**Baselines.** The anchor-only interval.

**Metrics.** Empirical coverage of the pool mean; label-saving factor = (anchor-only
width / PPI width) squared.

**Decision rule.** Minimum n: 2 agents with a 300-item pool, one of them at or
above the judge's capability. Supported if coverage is in [0.85, 0.95] for
every agent and `n`, and the saving factor is below 2 for every agent at or
above the judge's capability. Refuted if coverage falls below 0.85 for any
agent at `n >= 40`, or the saving factor is 2 or more for an agent at or above
the judge's capability. Otherwise unresolved.

**Prediction.** Supported. Coverage 0.88, 80% interval [0.83, 0.92]. Saving
factor 1.3 for not-weaker agents, 80% interval [1.0, 1.8].

**Known edges.** The anchored sample must be a random draw from the same
records the judge labels; any selection (anchoring hard cases first) voids
coverage. The pool mean is itself an estimate; coverage is of the pool mean.
A 300-item pool per agent and type may not exist within the deadline; the
MA7.10 synthetic-corpus check (known truth) is a unit check, not this test.

**If supported:** the anchor budget is sized from the saving factor (at most
2x for strong agents). **If coverage fails:** PPI estimates are not shown as
intervals on the owner's view; anchor-only intervals are used.

### P8. H8: satisfice, then diversify (the lead hypothesis)

**Claim** (plan R36 and 10.3; Goodhart survey H8): once all floors are met,
choosing among the candidates that pass by a diversity-weighted lottery, with
part of build capacity sampled from low-occupancy cells, gives the owner more
useful software than always choosing the best-scoring candidate, without more
floor breaches. Supporting evidence is indirect: a quantilizer bounds the
per-decision harm of a flawed measure at `1/q` (Taylor 2016); the Darwin Gödel
Machine's archive lottery beat branching from the best (50.0% against 39.7% on
SWE-bench); both are on fixed automatic scorers, not a product.

**Test.** The arms, after Phase 8 resumes self-development under floors F1 to
F5. A switchback design (Goodhart survey section 8): alternate 2-week blocks
between two selection policies, the order of each pair of blocks set by a
seeded coin. **Policy A (argmax):** the candidate or parent with the highest
gate score. **Policy B (satisfice, then diversify):** a lottery among
candidates that pass the floors, from the top `q = 0.3` by gate score,
weighted toward low-occupancy cells, with 10% of build capacity sampled from
low-occupancy cells when all floors pass (0% while any floor fails). The cell
descriptors are fixed by the owner before the first block and recorded in a
dated entry. The owner rates each shipped app for usefulness on a 1-to-5 scale
once a month, without being told which block produced it.

**Baselines.** Policy A.

**Metrics.** Primary: mean owner usefulness rating of apps shipped per block.
Secondary: floor-breach weeks per policy; effective number of occupied cells
(Hill number of order 1) of shipped apps; app survival at 30 days.

**Decision rule.** Minimum n: 6 blocks (12 weeks) and 20 rated apps per policy.
Bayesian estimate of the mean rating difference B minus A with a weakly
informative normal prior (mean 0, sd 1 on the 5-point scale), block as a
random effect. Supported if P(B > A) is at least 0.8 and B has no more breach
weeks than A. Refuted if P(B < A) is at least 0.8, or B has more breach weeks
than A. Otherwise unresolved. If floors fail in more than half of B's weeks,
the exploration budget never opens and the test is unresolved by design.

**Prediction.** Unresolved is the most likely outcome (probability 0.5) at this
n. Rating difference +0.2, 80% interval [-0.3, +0.6]. Diversity (effective
number of cells) higher under B by 30%, 80% interval [10%, 60%]. No difference
in breach weeks.

**Known edges.** One owner: n near 1, so the unit is the app and the block, and
drift over 12 weeks (model updates, owner mood) is controlled only by the
switchback. The owner may infer the policy from an app's novelty, so blinding
is partial; it is reported. The owner's rating is itself a measure that can be
gamed if desks see it; it stays class E (hidden) per 10.3. Taylor's bound holds
per decision, not across repeated ones. The survey expects an inconclusive
result not to change the architecture.

**If supported:** the lottery stays as built and the budget may rise (10.3). **If
refuted:** the result goes to the owner, since R36 is an owner ruling; the
default response is to narrow `q` toward 0.1 and hold the budget at the 5% floor.
**If unresolved:** policy B stays (R36) and the test continues for another 12
weeks under a new dated entry.

### P9. The three-score structure: does importance judgment add predictive value?

**Claim** (plan 10.1; trust draft 3.4 and open question 6): score 2
(importance judgment: the agent's materiality weight scored against realized
materiality measured by the frozen procedure, by ranked probability score over
I0 to I4; Gneiting and Raftery) does **not** predict a subject's next material
failure beyond scores 1 (accuracy) and 3 (coverage). This is stated as the null
because the plan takes it as the default; the test is the condition under which
score 2 could ever enter trust.

**Test.** Replayed records whose realized-materiality window has closed. For
each unit `(s, k, t)`, aggregate over the 90 days before `t`: mean score 1
(materiality-weighted Brier skill), score 3 (weighted omission count), and score
2 (mean RPS), the last only where at least 20 score-2 settlements exist in the
window (the minimum n per aggregate; fewer means no score 2 for that unit, not
zero). Outcome: material failure event in `W`. Fit (M0) logistic on scores 1 and
3; (M1) M0 plus score 2.

**Baselines.** M0.

**Metrics.** Held-out log-loss improvement, M0 minus M1 (positive means score
2 helps); AUC difference.

**Decision rule.** Minimum n: 30 failure events among units with a score-2
aggregate, from both d@e and the arms. Score 2 adds value (the claim refuted)
if the improvement's 95% interval lies above 0 **and** the improvement is
positive in d@e and the arms separately. The claim is supported if the
interval's upper bound is below 0.005 nats per unit. Otherwise unresolved.

**Prediction.** The null holds: improvement 0.002 nats, 80% interval
[-0.01, +0.015]. Score 2 correlates with score 1 (rank correlation 0.4, 80%
interval [0.1, 0.6]), so most of what it carries is already in M0.

**Known edges.** Kinds with no procedure in the custodian's catalogue have no
score 2 and are excluded. Realized materiality can be steered by the desk that
produces the later evidence; only independent-origin evidence counts (10.1),
and the share excluded as self-produced is reported. Windows of 30 to 90 days
mean the first results lag the first records by at least a quarter.

**If the null holds:** score 2 stays display-only, beside the posterior with
its n and interval. **If score 2 adds value:** the condition of plan 10.1 is
met, and only then may the owner decide to admit it to trust, in aggregate over
a window with a minimum n, never per record. Nothing changes automatically.

### Deferred: H1 (Goodhart survey, Autopaper front-page selection)

Goodhart survey H1 (front-page selection by a quantilizer draw against argmax
of System 1 importance; unsupported-claim rate and topical diversity) belongs
to the Autopaper unit after this mission (R37). It is not pre-registered here;
it will be registered in its own dated entry before the Autopaper unit's first
build slice.

## 3. Index

| Id | Hypothesis | Source | Data | Predicted | Builds on |
|---|---|---|---|---|---|
| P1 | H4: track records beat success rate, benchmarks, popularity, ratings | PSA 8.4 | replay, d@e and arms | supported | MA7.2, MA7.9 |
| P2 | Trust conjecture: LCB ordering predicts failure at a usable tap rate | trust draft 11, R0-15 | replay, drills on d@e | AUC 0.65 [0.52, 0.80] | MA7.2, MA7.8, MA7.9 |
| P3 | H-A1: outcome-built weights reward extremizers | scoring H-A1 | replay plus synthetic extremizer | supported | MA1.3, MA7.2 |
| P4 | H-A2: flip-rate relevance predicts reverts | scoring H-A2 | flip sampler, d@e and arms | weakly supported | MA7.10 |
| P5 | H-C1: raw pseudo-counts overconfident; tempered or capped chosen | scoring H-C1 | replay | supported; capped by tie rule | MA7.2 |
| P6 | H-B3: spot-checks beat peer agreement | scoring H-B3 | anchor sample | supported | MA7.10 |
| P7 | H-B4: PPI covers, saves under 2x for strong agents | scoring H-B4 | anchor sample plus System 1 labels | supported | MA7.10 |
| P8 | H8: satisfice, then diversify | Goodhart H8, R36 | arms, Phase 8 switchback | unresolved most likely | Phase 8 |
| P9 | Score 2 adds no predictive value beyond scores 1 and 3 | plan 10.1 | replay | null holds | MA7.2 |
| H1 | Autopaper front-page quantilizer | Goodhart H1 | deferred (R37) | not registered | Autopaper unit |

## 2026-11-06: MA7.9 harness landed; first run on d@e (counts only)

The harness is `cmd/trustharness` with `internal/trustharness` (scoring) and
`trust.PreregUnits` (the units of section 1, served on
`GET /api/trust?replay=1` as `prereg_units`); the commit that adds this entry
implements P1 and P2 as registered above and reports P3 to P9 by their data
source. No test text above is amended. Two procedural choices the entries
above left open, fixed here before any outcome was seen: weekly cutoffs fall
on Monday 00:00 UTC, the first strictly after the first material item; a
unit's window must close by the latest evidence, or the unit is dropped as
open. The split date of P1's held-out log loss is not fixed yet: no unit
exists.

First run: d@e (computer-59c902c5), as of 2026-11-06T07:05:57Z, build
181e95c77, artifact `docs/evidence/ma7-9-harness/2026-11-06-59c902c5/`.
That build predates the harness, so it served no `prereg_units`; its replay
walked 0 pairs over 0 trust rows and the gate replay 0 material acts. Every
test is **unresolved** with n = 0; nothing was sealed that had data, because
there was none. No outcome metric was computed or seen.
