# The four-stroke learning engine

The theory of how a Choir computer learns, and the one current home for it.
Written 2026-11-14 from the owner's statements of 2026-11-13 and 2026-11-14
(section 12) and a read of the code and docs at `origin/main` 5af04baad.

**Authority.** This page is the theory. The as-built reference is
[precommitment-records-practice-2026-11-08.md](precommitment-records-practice-2026-11-08.md).
Architecture stays normative in [choir-doctrine.md](choir-doctrine.md). This
page replaces `precommitment-records.md`, `psa-theory-2026-11-08.md`,
`design/bayesian-trust-2026-11-04.md` and
`design/tasks-precommit-icrl-2026-10-24.md` as the place to read what the
mechanism is for; those four are in [archive/learning/](archive/learning/) and
keep the detail this page points to. Where this page and the code disagree,
section 9 says which is the plan and which is the fact.

## 1. The engine

**Call it. Check it. Learn it. Earn it.**

| Stroke | What happens | Code name today |
|---|---|---|
| **Forecast** | Before acting, say what you expect, with a probability. | precommitment record; `Precommit`, `Cast` with `p_success` |
| **Outcome** | Someone else says what happened. | settlement, resolve, score |
| **Lesson** | Write down the most general thing learned, where the next actor and whoever it is about can read it. | not built (section 9) |
| **Trust** | Be given more, or less, ahead of the next outcome. | ceiling, lease, act gate, breaker |

The strokes run in that order and a skipped stroke means the engine does not
run. One stroke makes the power: the outcome meeting the forecast. The
surprise is the fuel, and a forecast that is never met by an outcome is intake
with no spark. Trust is also the flywheel: it carries what was earned into the
next cycle.

An engine cycles in order to move something. This is not a loop that returns
to where it started (owner, 2026-11-13: "You want to be making progress").

## 2. Two rules

**Rule 1. You do not mark your own work.** It has two parts, and both must
hold:

- **1a.** The one who made the forecast does not settle it.
- **1b.** The one whose act is being judged cannot write the outcome. An
  outcome the actor can write is not evidence.

A recipient who settles the sender's forecast about the recipient's own work
passes 1a and fails 1b.

**Rule 2. Nothing is held about you that you cannot read.** No hidden
dossiers. Owner, 2026-11-13: "everything that we learn about the listener is
going to be visible to the listener. No hidden dossiers. Fundamentally, I
think that that's borderline unethical." The same rule holds for agent desks
(owner, same day: "maybe ... no hidden dossiers also applies to agents. Maybe
that's a good symmetry. I do like that.").

Two things rule 2 does not say:

- **Readable is not writable.** A desk may read every record held about it
  because it can write none of them. Independence defends against gaming;
  secrecy does not have to.
- **A test may be unseen until it is marked.** A blind arm or a held-out
  sample may stay hidden while its window is open. When the window closes the
  actor gets the paper back. Nothing stays hidden for good.

## 3. The outcome is the teacher

Owner, 2026-11-14: "Yes, the outcome is the teacher."

The engine is pedagogy as much as machine learning. The student forecasts, the
outcome marks the paper, the student writes the lesson in their own notebook,
and trust is what the student has earned, including trust in themselves. Both
rules are what any decent classroom already does: the student does not mark
their own exam, and the student gets the marked paper back.

Outcomes differ in how well they teach. A good teacher is prompt, specific and
cannot be flattered.

| Outcome | Speed | Can it be flattered? |
|---|---|---|
| A listener's skip or listen | seconds | no |
| A test, a deploy, a rollback | minutes to days | only by gaming the test |
| The world settling a source's claim | days | no |
| A judge model with a rubric | fast | yes, by a desk that can read the rubric |

This is why the engine is built first where the teacher is best (section 8),
and why a judge model needs more care than the other three.

**The three-question test** for any place a forecast is made:

1. Is there a teacher? (Does anything settle it?)
2. Is the teacher someone other than the student? (Rule 1, both parts.)
3. Does the student get the paper back? (Rule 2, and the lesson stroke.)

A site that fails a question is fixed or stops asking for the forecast.

## 4. Two quantities, and what trust is

A settled forecast yields two different things with two different readers.

| Quantity | Question | Read by | Used for |
|---|---|---|---|
| **Calibration** | Was the forecast honest? | the actor | its next forecast |
| **Reliability** | Did the outcome meet the requester's criteria? | the delegator | trust |

A proper score makes honesty the best policy for calibration. Reliability is
judged against criteria the requester set, so forecasting failure and then
failing earns calibration and costs reliability. Timidity does not pay.

**Trust is what you are given ahead of the outcome.** It is consent to be
exposed before the evidence is in.

- **Everything gets an outcome, always.** That does not depend on trust; it is
  the cycle. Checking is also not distrust: a second perspective finds what
  the first cannot, whoever the first is.
- **Trust decides whether the check blocks the act.** A trusted desk acts and
  is settled after. An untrusted one waits.
- **Stakes set how much trust that takes.** Small and reversible needs little.
  Irreversible needs a great deal, or stays gated for good. The impact classes
  I0 to I4 are this scale and stay in the core.
- **Calibration is what makes an actor's own uncertainty usable.** A desk
  whose 0.6 means 0.6 tells the delegator when to check first. A desk can be
  unreliable at a task and still trustworthy about how unreliable it is.
- **Trust belongs to a relation:** who, whom, for what. Inside one computer
  there is one truster, the owner, acting through management. The relation
  matters across computers, where it is reputation.
- **Trust is also turned inward.** An actor's confidence is earned the same
  way: a record of having called it and been right. Owner, 2026-11-14:
  "Learning isn't just about learning, but about self-confidence."

Trust is the unexpected stroke and the most important one. Owner, 2026-11-13:
"the greatest significant bit of the meaning of language or ideas is the
relationship between the speaker and the audience. And that's a question of
trust." A lesson is a suggestion, and trust is upstream of suggestibility, so
a lesson is read according to where it came from. That is what joins the
fourth stroke back to the first.

## 5. Lessons

The lesson stroke is the one the system lacks, and the measured evidence says
it is where the gain is (section 9.2).

- **The student writes it.** The actor writes the lesson at the outcome, in
  its own words. The teacher marks; it does not dictate the takeaway. No text
  shown to an actor tells it which way to move a number ("be less confident").
- **A lesson cites its records.** A bare lesson loses the forecast, the
  failure and the revision that justified it. A lesson is an index over
  events, never a replacement for them.
- **A lesson is about the task, not the self.** "On schema migrations,
  estimates ran short when the table had triggers (4 of 5)" teaches. "You are
  over-confident" invites a uniform shift and loses discrimination.
- **A lesson attaches to the role and the task, and must survive a model
  swap.** Owner, 2026-11-13: "behavior is model specific but learnings are
  valuable insomuch as they generalize. Models have short lifetimes." Model
  churn is a free test: a lesson that stops holding after a swap was not a
  generalization.
- **Evidence about an actor stays keyed to the actor.** A model's calibration
  offset does not transfer to its successor. That is a record about an
  occupant, not a lesson about the work.
- **A lesson carries a forecast.** "Misses of this kind fall." It is settled
  by the records of the runs that read it, so a lesson is scored like anything
  else, and promoting one into shared context is an act under the same trust.
- **Most outcomes confirm.** The stroke always fires, but usually its result
  is that an existing lesson held again and its count goes up. New lines come
  from surprises. The notebook stays short enough to read.
- **The highest lesson is the generalization** that lets several lessons be
  deleted. Owner, 2026-11-13, on why: it restructures the whole ontology "into
  a lower energy state", so there is less updating afterwards.

When the owner promotes a lesson by hand into [CLAUDE.md](../CLAUDE.md), that
is this stroke at full verification. A desk promoting a lesson into a shared
stub is the same act under a lease.

## 6. Getting the paper back

In a system that learns in context, behaviour changes only when context
changes. What reaches which model, and when, is the learning system.

| What | When | Why then |
|---|---|---|
| Lessons for the role, each with its count | always, in the prompt stub | standing context shapes the whole run |
| The record that bears on this decision | returned by the verb, at the forecast or the delegation | the question arises at the decision |
| The raw records | on demand, through the REPL | the reason to dig is a surprise, which is also when a lesson gets written |

Records are stated plainly and without spin. Accurate, no optimistic bias, no
identity labels: "you said 0.8 on these and 5 of 9 held", not "over-confident".

## 7. One cycle at every scale

| Scale | Forecast by | Teacher | The lesson is written back to |
|---|---|---|---|
| One run | the run | the tool result | the run's own context |
| A desk | the desk | a rule, a blind judge, another desk | the verb-time record |
| A role | whoever holds it | the same | the lessons in the stub |
| The computer | a mission | CI, staging, the owner | the operating contract |
| Across computers | a publisher, a source | the world, other readers | published objects and their records |

## 8. Where it is built, in order

Fastest and best teacher first.

1. **Autoradio.** The DJ forecasts each segment. The listener's act settles
   it. Lessons go in a notebook the listener can read and edit. See
   [design/autoradio-learning-engine-2026-11-14.md](design/autoradio-learning-engine-2026-11-14.md).
2. **Autopaper.** A source's dated claim is a forecast and the world settles
   it. For one reader, a story's importance is a forecast and that reader's
   listening settles it. The listener's edition is the junction between the
   two. Not built: Autopaper reads the edition nowhere today.
3. **Self-development.** A mission forecasts, CI and staging settle, the
   lesson goes into the operating contract. The forecasts and outcomes exist;
   the lesson stroke is done by hand.

## 9. The state on 2026-11-14

### 9.1 The strokes as built

**Forecast.** Fourteen places write a forecast.

| State | Sites |
|---|---|
| Settled by an independent rule or a blind judge | engineering rule precommits (tests, freeze, paths, completes); desk-to-desk `Cast` `p_success`; research sub-calls; Texture write support |
| Settled by the party itself (fails rule 1) | `verify_pass`, which management states and management's own Verify settles; Cast hypotheses, settled by the recipient's own Report; `/api/precommit` claims, where the caller may settle as `self`; the Claude ledger, where the forecaster settles |
| Stated and never scored (no teacher) | `Ask` `p_success`; `Escalate`, `Feed`, `Decline`; lease-request `p_success`; the install-review prediction |
| Waiting on a script nothing schedules | Cast hypotheses by `resolves_by`; research `p_complete` |

The guard that should catch the second row reads a field with no writer.

**Outcome.** Trust reads accuracy only. The other two outcome questions
(section 10) never emit because nothing produces their inputs.

**Lesson.** Absent. `CommitmentRevision` and `RetrievedByIDs` exist in
`internal/types/commitment.go` with no writer and no reader. The engineer's
prompt says `choir.Pack` shows how each claim settled; the capsule request
carries no Pack, so it is always empty. Management states about half of each
round's claims and is never shown how they settled. The calibration digest
reaches engineering only and ends in an instruction.

**Trust.** Forecast evidence sets lease duration and feeds the act gate. The
gate enforces only on third-party app desks.

**Hidden today (the rule 2 surface).** The 10% held-out sample, the control
arm and its salt, management's round predictions (dropped from the engineer's
view), and the owner-only trust and calibration routes. The visibility
classes and hidden audits of the earlier design were never built.

### 9.2 What the evidence says

From 341 settled claims across five test arms
([design/calibration-wiring-permutations-2026-11-04.md](design/calibration-wiring-permutations-2026-11-04.md),
section 0):

- Brier 0.211 against a base rate of 0.238. Perfect recalibration of the
  stated numbers would gain at most about 0.016.
- The gap is discrimination. The numbers barely separate rounds that pass
  from rounds that fail; `tests_pass` shows no skill over the base rate
  (n=128).
- Claims within one round correlate at 0.69. The unit of sample is the round,
  and the fleet did about 95 rounds a day.

A number about the self can fix only the small term. Knowing which situations
fail is the large term, and that is what a lesson about the task carries.

## 10. The outcome stroke has three questions

| Question | Asks | State |
|---|---|---|
| Accuracy | Were the probabilities right? | live |
| Importance | Did you weigh the consequences right? | designed, no inputs |
| Coverage | What did you leave out? | designed, no inputs |

Importance and coverage are refinements inside the outcome stroke, not
strokes. Both are stated in the forecast and scored at the outcome. A coverage
miss is the natural trigger for a lesson: a wrong probability can be fixed by
changing a number, but an omission had no number, so the only repair is a
lesson. Their design is in
[archive/learning/bayesian-trust-2026-11-04.md](archive/learning/bayesian-trust-2026-11-04.md),
section 3.4. Do not build their inputs (reference predictors, the critic
panel, the materiality catalogue) until the four strokes run somewhere.

## 11. Words

The four words are used in everything a person reads: docs, prompts, the
notebook, the pitch.

| Say | Instead of |
|---|---|
| forecast | precommitment, precommit |
| outcome | settlement, resolve, score |
| lesson | pack, calibration digest, revision |
| trust | alignment; and ceiling, lease, gate, breaker when the mechanism is not the point |
| the learning engine | prospective self-alignment, PSA |
| the notebook | (the listener-facing home of lessons) |

"Perspective" is the word for what a lesson changes. It belongs to the theory
and stays out of product language.

**Not renamed on the wire.** The record kind `"precommit"` is in the desk
protocol lock (14.0.0), there is a `choir precommit` command, and `precommit`
appears in 51 Go files. Those names stay until a protocol bump is wanted for
another reason. Two words already have another meaning in the code and must
not be changed by search and replace: "forecast" is also a hypothesis kind for
a scheduled world event, and "settlement" is also how a mission or a work item
closes.

## 12. Why trust, and where it came from

The first three strokes each come from a technique that works. Forecast is the
lesson of pretraining: prediction is the universal verifier, and it turns
less verifiable work into more verifiable work. Outcome is the reinforcement
signal, made safe by rule 1. Lesson is in-context learning, the write-back.
The fourth has no ancestor in that lineage. It comes from the study of
persuasion and of institutions.

The owner's statements this page rests on, 2026-11-13 unless dated:

- "the point isn't primarily or merely to rename alignment to trust but to
  reframe through the lens of trust."
- "We will know we've succeeded when it's not a collection of rulings but a
  system which 'conducts' ... when it's coherent enough that it doesn't need
  explanation."
- On packs: "we currently document packs as per model but I've realized
  that's wrong."
- "I think we should call it a 4-stroke learning engine."
- "having a core that is both functional and marketable, that is, you can
  communicate it in a meme, is key for making it something which humans can
  hold in their mind."
- On whether a good record should mean less checking: "trust but verify,
  right? And obviously it depends on the stakes of the situation."
- 2026-11-14: "consider in light of not just machine learning but pedagogy."

The longer argument (perspective as the unit of intelligence, why plural
providers are more than horizontal scale, functional differentiation over
role names, trust as upstream of suggestibility) is the owner's and is not
restated here. [signal-is-sparse-not-the-learner-2026-08-01.md](signal-is-sparse-not-the-learner-2026-08-01.md)
makes the plurality argument in the repo.

## 13. Open questions

1. **A judge the student can read.** When the teacher is a model with a
   rubric, a desk that reads the rubric can write toward it. Autopaper
   importance has this problem today. Rule checks and human acts do not.
2. **Strategic omission.** Committing narrowly and acting broadly is caught by
   coverage, which has no inputs. Until it does, the stakes classes and the
   frame log are the only defence.
3. **Who is "someone else"?** Two judges from one model family share blind
   spots.
4. **The home of a lesson.** No kind, no custodian and no reader exists yet.
   Autoradio's notebook is the first.
5. **A forecast open at a crash.** Today it closes as interrupted and is
   excluded. The engine says nothing more.
6. **The actor's read route.** Rule 2 needs one. `GET /api/trust` is an owner
   route and no desk reads the claims API.
7. **Model-specific bias.** Evidence is thin both ways: two models, confounded
   with the computer they ran on.
8. **Does the lesson stroke work?** Nothing has shown it. Autoradio is the
   experiment.
