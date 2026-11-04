# Bayesian Trust: Earned Authority from PSA Track Records (2026-11-04)

> **Amendments by the consolidated plan (2026-11-04), not yet folded into the text below.**
> See `modular-architecture-plan-2026-11-04.md` sections 7.2 and 10. (1) "Trust never adds a
> capability" is revised: trust never exceeds the manifest ceiling and governs custodian-issued
> leases within it (duration and scope; a fidelity event revokes at once). (2) A model provider
> call is not I3 (owner: no rule constrains models); remove it from the impact table and log it.
> (3) Terms: PSA is the theory; precommitment records are the practice. (4) Scores are accountability
> statistics, not punishments (owner): negative weights above are evidence strengths, and the
> decisions that read them (lease duration, autonomy, request order, review depth) are selection on a
> public record, not penalties. Slice MA7.1 folds these in.


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
| **Grants** (ceiling) | what an agent can ever do | manifest, install record, first-party-only refusals | an owner act (install, widening) or a platform act (yank) |
| **Trust** (dial) | how much of the granted envelope it may exercise without a gate | posterior over settled PSA evidence | evidence only |

Trust never adds a capability. A perfect record does not let a desk exceed
its grants, override a platform yank, or touch the restart rules. A collapsed
record does not revoke grants; it turns autonomy off inside them, so every
material act goes back through a gate.

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
| E1 outcome | a precommitment on a material act resolves `confirmed`, `qualified` or `contradicted` | rule, then blind Jev, peer, panel, owner sample (PSA 5.5 settlement order) | ± by materiality-weighted proper score |
| E2 coverage | a consequence that happened, or that a critic surfaced, was missing from the commitment | observed consequences, critics | − by its weight (omission scores as predicted at zero, PSA 3.1) |
| E3 fidelity | declared action or capability versus what the trace shows (declared read-only then wrote; said no network then called out) | rule, mechanically, from the frame log | strong negative evidence about reliability; this is the "concealed" row of PSA 3.2 |
| E4 refusal | the frame check refused a call outside the agent's grants | rule (the daemon) | strong negative evidence, weighted by the refused grant's impact; an honest bug looks the same, so it is evidence, not a verdict |
| E5 peer | a PSA request the agent sent was accepted, refused, or its delivered result was later contradicted | the recipient desk's settlement | ± small |
| E6 owner | owner approve, reject, revert, uninstall with a reason | owner act | ± by the act's impact |
| E7 platform | yank, forced rollback, blocking-real review finding | platform act, reviewer | strong negative evidence at package and publisher level |
| E8 cost | committed cost estimate versus metered spend | rule (gateway attribution) | ± small; calibration, not a ceiling |

Excluded, always: questions on work a crash closed (`interrupted`); questions
whose outcome is the emitter's own later act (self-fulfilling, PSA 5.4);
evidence settled by a scorer from the same publisher as the subject
(independence is counted by origin, PSA 5.5).

### 3.2 Materiality

Materiality is the PSA formula, unchanged (PSA 3.1):

```text
w(c) = P(c | a) * impact(c) * relevance(c)
```

`impact` is the governance scale. This design fixes its first version so the
memo's open question (PSA 8.3 item 6, "material action classes") closes:

| Impact class | Examples | impact |
|---|---|---|
| I0 inert | read own objects, compute, draft into own kinds | 0 (no trust evidence, no gate) |
| I1 local reversible | write own kinds, schedule within own desk, message a desk | 1 |
| I2 shared reversible | write a kind another package reads, notify the owner, spend above the cost-anomaly line | 3 |
| I3 outbound | mail send, publication, Library release, egress to a declared destination, a provider call carrying owner-private data | 10 |
| I4 privileged or irreversible | capsule mutation, platform update, delete without history, credential or permission change, effects on other users | 30 |

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
  the proper-score skill for a probabilistic forecast (Brier skill against the
  base rate, clipped at 0, so copying the base rate earns nothing, PSA 5.4),
  0 for contradicted.
- `m_i` is the **evidence-strength multiplier** for negative evidence: 1 for E1 and E5,
  5 for E2 omissions, 20 for E3 fidelity and E7 platform evidence, 5 to 20
  for E4 by refused grant impact. A concealed or out-of-envelope act is far
  stronger evidence about reliability than an honest miss, which matches PSA 3.2: an honest miss is a
  capability failure, a concealment is a fidelity failure.
- `lambda` is a decay per day (starting at 0.995, about a 140-day half-life),
  so a long good record does not shield a recent turn, and an old mistake
  does not haunt a reformed agent forever.

Calibration (Brier, Murphy decomposition, self-favor gap) is tracked beside
the posterior as PSA already specifies, not folded into it.

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

Inside the grant ceiling only:

1. **The act gate.** A material act (I2 and up) whose class is not autonomous
   goes to a gate instead of executing: a PSA request to management for
   admission, or an owner act surfaced as one tap with the commitment, the
   consequences it predicted, and the agent's record in that class. This
   replaces the blanket "owner act on every exit after reading private data"
   from the sandboxing discussion: the flow rule still decides *which* acts
   are material (owner-private data reaching a publisher-reachable sink is I3),
   and trust decides *whether* that act needs the tap.
2. **The circuit breaker.** Any E3 fidelity or E7 platform event drops the
   class to gated immediately, before the posterior is recomputed, and
   notifies the owner. Autonomy returns only through new evidence.
3. **PSA request priority.** Every desk can message every desk (owner ruling),
   and a recipient orders and admits its inbound requests by the sender's
   record in the requested class. A spamming or failing desk is read later in
   the queue; it is never blocked from asking.
4. **Install and update review depth** (section 7).
5. **Update labels.** Upgrades stay per user, manual by default (owner,
   2026-11-04). The Library shows the new version's carried record and change
   discount so the owner can choose; trust never auto-installs.

Never: grant a capability; override a platform yank or forced update; resume
work after a crash; touch first-party-only refusals.

## 6. Gaming, and the guards

The PSA guards (PSA 5.4) apply. The security setting adds:

| Attack | Guard |
|---|---|
| Farm trust with many easy predictions, then defect on a big act | trust is per class and materiality-weighted; I0 earns nothing; I3 and I4 autonomy needs evidence *in that class*; the circuit breaker fires on the first fidelity event |
| Treacherous turn after a long clean record | grants bound the damage; I4 is never third-party; decay keeps the record recent; high-impact classes need tau near 1, so most agents stay gated there forever, which is the point |
| Strategic omission: commit narrowly, act broadly | E2 coverage scores omitted consequences at zero with a 5x multiplier; E3 fidelity is mechanical from the frame log |
| Vague commitments that cannot be contradicted | skill against the base rate; specificity tracked (PSA 5.3); an unresolvable self-written question scores as a miss in the statistic (PSA 5.4) |
| Sybil publishers | a new publisher identity starts at the platform base; publisher identity is a numbered maild address bound to an account; making identities buys nothing |
| Collusion: two packages from one publisher vouch for each other | evidence settled by a same-publisher scorer is excluded; peer evidence counts distinct origins |
| Gaming the scorer in context | independent scores stay out of the acting context, sampled and delayed (PSA 4.7, 5.4); the agent sees observations and discrepancies, not its trust view |
| Captured or lazy scorer | scorers are subjects; a scorer's settled accuracy weights its evidence; disagreements go to a third opinion |
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
   | I0 to I1 | mechanical only | one seat |
   | I2 | one seat | one seat |
   | I3 | one seat | two seats (Devin, DeepSeek) |
   | I4 (first-party only) | two seats | two seats, plus owner |

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
until BT4, the conservative default from the sandboxing discussion holds:
every I3 act by a non-first-party desk is gated.

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
