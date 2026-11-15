# Calibration wiring: the permutations, and a self-development experiment programme (2026-11-04)

Status: green (research and design; no runtime change). Mutation class of
this document: green. Every change it proposes carries its own class.

Prompted by the owner, 2026-11-04 about 06:10Z: "The specifics of how to
wire up the predictions and calibration seed, who predicts what and who
scores and when and how that info is shown…it's still in flux. Investigate
and report on the permutations and we should do some experimenting and
optimization with self dev on this." And, minutes later: "For example, seed
as an initial prompt or just in the repl as optional data to fetch? Or some
hybrid (a stub in prompt)?" Delivery is therefore the first axis below.

Binding inputs, read in full:
[calibration-seed-2026-11-03.md](calibration-seed-2026-11-03.md),
[choir-calibration-seed-2026-11-01.md](../archive/definitions/choir-calibration-seed-2026-11-01.md),
[precommitment-records.md](../archive/learning/precommitment-records.md). Code read at
origin/main `ed2e54c6b` (cut 2, `bd83082b9`, is in that tree; staging still
served cut 1 at 06:2xZ: `GET /api/calibration/claims` returned 404 on all
five arms). The engineer-reuse design read at the `engineer-reuse` branch
(`1f7b9c178`, `docs/design/engineer-reuse-2026-11-04.md`).

Evidence: `GET /api/calibration?detail=1` on the five test arms at about
06:20Z (claude, deepseek, luna, pool, crouton; 341 settled observations,
25 pending), the live8 trace review (04:31Z to 05:45Z), and the overlay
prompts. Numbers below are from that pull unless marked otherwise. No
agentic-consensus panel was run; the permutations below come from the code
and the records.

---

## 0. Five findings that shape everything else

1. **The feedback goes to a desk that did not state most of the claims.**
   Cut 1 and 2 make management state five of a round's claims in its Cast
   (`verify_pass`, `judge_accepts`, `owner_approves`, `freeze_minutes`,
   `approval_minutes`). `calibrationThreadOfRecord` counts every rule
   precommit with an assignment on `engineering:<assignment>`, so they enter
   **engineering's** digest, and the digest tells the engineer "your track
   record ... your self-scored predictions". The push desk is engineering
   only (`calibrationPushDesks`). Management, which stated those numbers,
   never sees how they settled. The A/B arm on those records is the
   engineer's arm, although management stated them before the engineer
   existed and saw no treatment. For about half of each round's claims the
   cut-2 A/B therefore measures nothing, and the engineer is shown someone
   else's errors labelled as its own.
2. **Management cannot be split within a computer by thread.** The arm is
   `HMAC(salt, thread)`. An engineer's thread is per assignment, so its
   rounds split 50/50. Management's thread is one long-lived inbox per
   computer, so every management run on a computer lands in the same arm.
   A within-computer A/B for management needs a per-act unit (the Cast, or
   the trajectory it serves).
3. **The ceiling on calibration feedback is small; discrimination is the
   gap.** Over all 341 settled claims: Brier 0.211, base-rate Brier 0.238
   (skill 0.11). Murphy decomposition at 0.1 bins: reliability 0.016,
   resolution 0.043, uncertainty 0.238. Perfect recalibration of the stated
   numbers would cut Brier by at most about 0.016. By claim type, Brier
   against the base rate's: `tests_pass` 0.241 vs 0.243 (no skill, n=128),
   `freeze_within` 0.271 vs 0.248 (worse than the base rate, n=46),
   `completes` 0.223 vs 0.239, `verify_pass` 0.209 vs 0.219,
   `paths_confined` 0.051 vs 0.040 (near-certain, n=48, 96% held). The
   desks are over-confident (mean p 0.71, 0.61 held) but, more
   importantly, their numbers barely separate rounds that will pass from
   rounds that will not. A seed judged only by ECE would chase the small
   term.
4. **Claims within a round are nearly one observation.** The intraclass
   correlation of (p − y) within an assignment is 0.69 (108 assignments,
   3.2 settled claims each), a design effect of 2.5 now and about 7 at the
   ten claims a cut-2 round states. The unit of sample size is the round,
   not the claim.
5. **The round rate is the binding constraint.** From 2026-11-03 18:00Z to
   2026-11-04 05:50Z (11.8 h, including a 1.5 h Synthetic quota outage)
   the five arms opened 48 engineering assignments: 4.1 per hour for the
   fleet, 0.7 to 0.9 per arm per hour (crouton 1.5 per hour since it began
   at 22:18Z). Call it **0.8 rounds per arm per hour, 4 per hour for the
   fleet, about 95 a day**. A between-round A/B on outcome calibration needs
   about 125 rounds per arm (section 9), about 2.6 days of the whole fleet
   for one two-arm comparison. Designs that measure within an act read out
   several times faster.

Two smaller findings recur below. The prompt examples anchor the numbers:
`tests_pass` p = 0.8 is the most common stated value (21 of 138), the
engineering overlay's example is `"p": 0.8`, and management's literal Cast
example states 0.8 / 0.7 / 0.6 / 25 [15, 45] / 90 [40, 240]. And Texture's
packet states predictions management then copies (crouton r4: identical) or
restates (luna r3: 0.78 / 0.6 / 30 [18, 40] against 0.8 / 0.65 / 32
[20, 48]); Texture's numbers are scored nowhere.

---

## 1. Axis: how and when it is shown (delivery first)

The owner's question. Four arms plus the null, costed in tokens, model
turns and the prompt cache. Measured sizes: the rendered engineering digest
is 600 to 640 characters, about 100 words, about 150 tokens (crouton n=30,
luna n=114). An engineer run is long: crouton r3 made about 200 cells in
its last 8 minutes alone, at about 130k input tokens a call with
`cached_input_tokens=0` on the OpenRouter route. A management activation
is shorter. Every arm below appends after history, so none of them breaks
the cached prefix; the one design that would (the digest in the system
prompt or a tool description) is ruled out, because each digest change
would invalidate the cache for every later call.

| Arm | What it is | Built now | Cost | Risk | What chooses it |
|---|---|---|---|---|---|
| **0. No seed** | Nothing shown; records still written and settled. | Control half of the cut-2 A/B; every desk but engineering. | None. | None; it is the comparator. | — |
| **(a) Push** | The full digest as a user turn at activation start, re-appended after a tool batch when the record changed; also in `choir.Pack().Calibration`. | **Baseline**: cut 2, engineering digest arm (50% of assignments by salted hash). | ~150 tokens once, then carried in every later call of the run: ~150 × calls, about 30k tokens on a 200-call engineer run (0.1 to 0.2% of its input). No extra turn. | Read at a moment far from the prediction (the engineer's precommits come in its first cell, so this is close; management's Cast may come much later in a long activation). Shows management's errors to the engineer (finding 1). Mixes claim types in its bins. | Reduces mean bias on the next claims against no-seed, at no resolution loss. Between-round A/B: slow (section 9). |
| **(b) Pull** | Nothing in the prompt. The record is data in the REPL (`choir.Pack().Calibration`, or a dedicated `choir.Calibration()`); the surface line names it. | Not as an arm: `Pack().Calibration` is nil for a control run (`calibrationDigestForRun`), so no run can pull without being pushed. | 0 tokens until read. A read costs one extra model turn at full context (50k to 130k input tokens uncached) unless folded into a cell the desk was running, plus ~150 to 400 tokens of result. | Most likely outcome: never read, so no effect (the 10-14 design's own expectation). A read after the prediction teaches nothing. Read rate becomes a behaviour the prompt wording drives. | The **read metrics** below. If fewer than about 1 in 4 treated runs read before predicting, pull is dead as a delivery and needs no outcome test. |
| **(c) Hybrid stub** | One or two lines pushed, built from the same record, naming the fetch for detail. For example: "Engineering record (n=30): you said 0.78, 0.67 held; `tests_pass` least calibrated; your Freeze estimates ran 2.8× the actual minutes. `choir.Calibration()` for the bins and recent misses." | No. | ~30 to 40 tokens carried per call (~7k on a 200-call run), plus pull's cost when followed. | The headline does most of the work, so (c) mostly tests "short push" against "long push". A headline line chosen badly is an instruction in disguise ("you are over-confident" invites a uniform shift down, which can lose resolution). | Same outcome metric as (a), plus the read-through rate. Wins if it matches (a) at a fifth of the tokens, or beats it because it is read. |
| **(d) Event-timed** | Shown at the predicting act, per claim kind, rather than at run start. Three variants: (d1) in the refusal text only (exists today for malformed Casts, without the record); (d2) in the result of the act, which informs only the *next* act; (d3) **two-stage**: the act's first statement p0 is recorded blind, the result shows that kind's record ("your `freeze_minutes` estimates: median 2.8× actual, 1 of 4 intervals covered"), and the desk may restate once (p1) before the capsule starts. | No. | (d1) 0; (d2) ~60 to 100 tokens per act; (d3) one extra model turn per predicting act (one call at full context; management's Cast once per round, the engineer's first cell once per run). | (d3) is itself a treatment: a second look may move p toward 0.5 without the record (hence its control is "restate without the record"). Late restatement must stay before the first build (R20). | (d3) gives a **paired** measurement on the same claim (p0 and p1 share y), so it reads out in tens of rounds, not hundreds. It is also the only arm that randomises management per act (finding 2). The owner's note is right: predictions happen at specific acts, and this arm puts the record exactly there. |

Combinations (a)+(d), (c)+(d) are later questions; each is a second
variable.

**Read metrics for (b) and (c)** (needed before either arm runs; see cheap
change C6):
- `read_any`: the run evaluated the calibration accessor at least once.
  `choir.Pack()` is called for other reasons (Waits, ProviderWaits), so the
  accessor must log its own read mark (a dedicated `choir.Calibration()`
  that writes a ledger mark with run id, time and the digest's hash).
- `read_before_predict`: the first read precedes the run's first
  Precommit (engineer) or its predictions-bearing Cast (management). This
  is the primary read metric; a read after the prediction is curiosity, not
  feedback.
- `read_latency_s`: run start to first read; `reads_per_run`.
- For (c), `read_through` = `read_before_predict` among stub runs; the gap
  between (b) and (c) is what the stub's headline buys.

**Delivery to whom.** Its own record only, sliced by the stater (cheap
change C1). Showing a desk another desk's record is routing by record
(axis 6), a different objective. Independent scorers' judgments (Jev, a
panel, the owner's labels) stay out, or are sampled and delayed (4.7).
Rule settlements of events are observations, not a scorer's judgment, so
they may be shown; the judge's verdict is a judgment, and `judge_accepts`
outcomes should be shown in aggregate only and with a lag of at least one
round, never the verdict text of the current round.

**Display to the owner.** The round receipt line (built, one line in
Texture's owner-act record), the morning letter, `GET /api/calibration`
and `/claims`. Owner display does not touch the acting context, with one
exception: a Texture document is a desk's acting context. Until Texture's
own seed experiment, calibration for the owner goes to the morning letter
and the API, not into a Texture document body.

---

## 2. Axis: who predicts

| Who | What it is | Built now | Cost | Risk | Evidence that chooses |
|---|---|---|---|---|---|
| Texture | States predictions in the packet it Casts to management. | Texture does it unasked (crouton r4, luna r3); unscored; management reads it. | ~0 (already in its packet). | Anchors management; a forecaster no one scores. | Score it as a third forecaster for a week, or strip it. Owner call (C8). |
| **Management at Cast** | The round's five mandatory claims. | **Baseline** (cut 1 and 2): mandatory, strict decode, refused with a literal example. | One spec block per Cast; the 6-tries failure is fixed by the literal example. | `judge_accepts` predicts its own Verify (self-fulfilling until the judge leaves Verify). Its record goes to engineering (finding 1). | Its own digest; its Brier against the engineer's on the same claim. |
| **The engineer at start** | Rule precommits in its first cell (`tests_pass`, `freeze_within`, `paths_confined`, `completes`, and optionally `verify_pass`, `judge_accepts`, `freeze_minutes`). | **Baseline**, optional but prompted; clock claims after the first build or shot are late and unscored (cut 2c). | One cell. | Self-fulfillable clocks: the engineer controls its own Freeze time, so `freeze_within` can be hit by freezing early (the crouton pattern: a 13.4-minute Freeze the judge rejected 25 s later). `paths_confined` is near-certain filler. | Joint or conditional clock claims (C10); exclusion of trivial claims from the digest (C4). |
| The engineer per loop or re-Freeze | A fresh claim at each revision under engineer reuse ("minutes to re-Freeze", `verify_pass` for revision n). | No (named as not built in the seed doc; reuse R2 keeps precommits per assignment, settled at the first Freeze). | One cell per revision (at most three). | More claims per round raise the ICC, not n. | Needed once reuse lands, because the round-level clock stops meaning one Freeze. The natural moment is the resume that carries the rejection reason: an event-timed (d) moment. |
| The judge, predicting the owner | Management's Verify (today's judge) states p(owner approves) beside its verdict. | No. | A field in Verify. | On the test arms the "owner" is FP0, an agent (settlements carry `delegated`). | Whether the judge's p beats management's Cast-time `owner_approves`; needs real owner decisions, so it waits for the owner's own rounds. |
| The Claude delegate | The delegate states claims on its write. | No. | Delegate turn. | A fourth forecaster per round, same ICC. | Later; only if the delegate's write is where rounds fail. |
| The pool or router | The router predicts a model's success before choosing it. | No. | Router step. | This is routing by record, a different objective (axis 6). | Phase 2, after the seed reads out. |

Engineer and management already state `verify_pass` (and sometimes
`judge_accepts`) on the same round (luna 05:29: management 0.8, engineer
0.68). That is a free paired comparison of two forecasters on one outcome
(experiment E5), with no code change.

---

## 3. Axis: what is predicted, and in what form

| Family | Claims | Built now | Settled by | Notes |
|---|---|---|---|---|
| Outcomes | `tests_pass`, `verify_pass`, `judge_accepts`, `owner_approves`, `completes`, `paths_confined`; apply success, `adopted_elsewhere`, rollback | **Baseline**: the first six (owner_approves in cut 2). Apply, adoption, rollback: no. | Runtime events (rule scorer). | `completes` and `paths_confined` are mostly about the stater's own later act (flagged in 5.4 as self-fulfilling). Apply and rollback are the outcomes that matter to the owner and are cheap to settle from existing events. |
| Clock | `freeze_within` (threshold), `freeze_minutes`, `approval_minutes` (estimate, interval, p) | **Baseline** (cut 1 and 2). Not built: to first build, per phase. | Cast `committed_at` to Freeze or approve record. | The round receipt already records every phase (Texture Cast, management Cast, bind, spawn, waits, first build, shots, Freeze, decision), so per-phase claims cost nothing to settle. `approval_minutes` on the arms is mostly FP0's latency (34 minutes awaiting approval on crouton's op 6dfb9260): it scores a forecast of the operator, not of engineering. |
| Cost | tokens, build or shot jobs, memory waits | No. | Run usage, job end records. | Cheap to settle; owner's 11-01 list names cost/token estimates. A second family, after clock. |
| Quality | judge per-shot scores; review findings | No (no per-shot score is emitted yet). | The judge. | A judgment, so a scorer's output; keep it out of the acting context or delay it. |
| Counts | `approved_within_rounds` (N Freezes); re-Freezes | Cut 2, optional. | Freeze count on the trajectory. | Settles late; low n per day. |
| Release level | `Predicts:` trailers walked through the rings | No (designed in the seed doc). | The ring history settler. | A different population (platform authors, not desks); can run concurrently with any desk experiment. |

**Form.**
- Binary p: built. Scored by Brier or log. Cheapest; vague p near 0.5 is
  caught by resolution and sharpness, not by ECE.
- Interval plus p(inside): built for clock. The current digest scores only
  the binary "inside", so it never shows the log ratio the owner asked for
  ("you estimated 3 weeks ... you did in 3 hours"), though the readout
  computes it.
- Point plus p: what is stated today, if the estimate is scored too.
- Quantiles (p10, p50, p90): sharper and proper under the quantile or
  interval score, and they fit CRPS. They cost one more number and a
  change of the strict decode.

Recommendation: keep the built shape and score all three parts of it
(the inside answer by Brier, the estimate by the absolute log ratio, the
interval by the interval score at the stated level). Do not change the
form during an experiment.

---

## 4. Axis: when it is stated

| When | Built now | Risk | Note |
|---|---|---|---|
| **At Cast** | **Baseline** for management's five claims; never late. | Stated before the work is understood; anchors on the example numbers. | The Cast record's `committed_at` is the clock anchor. |
| **Before the first build** | **Baseline** for engineer clock claims (cut 2c: late if after the first build or shot job). Other engineer claims may come any time before the Freeze. | The "Freeze within 40" claim staged 21 s before the Freeze (claude r2, 05:22:00 vs 05:22:21) is the failure this rule marks late. | Keep. |
| At each Freeze | No. | One more claim with the same outcome. | Needed for revisions under reuse. |
| Continuously, with revisions | No. | A revision near the outcome scores well by peeking. | Score each stated version at its own time (prequential); the digest shows the first statement's score. The (d3) two-stage design is the bounded version of this: exactly one restatement, before the capsule starts. |

Late or retroactive claims: cut 2c keeps them, marks them late and never
scores them; the late share per desk is its own metric. That is right.
Add one thing: show the late share in the desk's own digest, since a desk
that states late has learned nothing about prediction.

---

## 5. Axis: who scores, and how

**Who.**
- **Runtime event settlement** (rule scorer): **baseline** for every
  engineering claim. Not a judgment; an observation of an event. Settles
  in seconds.
- **The judge** (management's Verify today): settles `judge_accepts` by its
  own verdict; management predicts its own act. Until the judge leaves
  Verify, slice `judge_accepts` stated by management out of management's
  record or flag it.
- **The owner** (approve, reject): settles `owner_approves`; on the arms the
  actor is FP0 and the record says `delegated`. Report delegated and owner
  settlements apart.
- **A delayed independent scorer**: the pending list behind
  `GET /api/calibration` exists for hypotheses; no engineering claim needs
  it today. It is the path for quality claims.

**Scoring rules, all views over the same answers.**
- Brier: built; bounded, proper. The headline.
- Log: proper and harsher at the extremes; the 0.02 to 0.98 clamp makes it
  safe. Report beside Brier.
- Murphy decomposition: reliability (calibration), resolution, uncertainty.
  Owner rule: calibration is reported with accuracy and resolution, so
  vagueness cannot win. The digest today shows Brier and ECE only; add
  resolution (or sharpness, mean |p − 0.5|) per claim type.
- Brier skill against the base rate per claim type: "copying the base rate
  earns zero" (5.4). Finding 3 is this number.
- Clock: coverage of the stated interval against the stated p; the
  absolute log ratio of estimate to actual (and its signed median, the
  bias the owner wants shown); the interval score at the stated level;
  CRPS if quantiles are adopted.
- Censoring: today a Freeze that never happens settles "no, censored". For
  the binary inside claim that is right only when the round ended after
  the interval's high bound. A round that ended (crash, abandon) before the
  low bound tells nothing about the clock: exclude it, and report the
  censored share apart (C7). A crash-closed round is `interrupted` and
  excluded from all calibration (5.4).
- The A/B readout: never ECE alone. Primary per experiment below.

---

## 6. Axis: what it is for

| Objective | Measured by | Shares an experiment with |
|---|---|---|
| Self-calibration of stated p | Brier, reliability, resolution, log ratio on held-out claims | Owner display (it does not touch the acting context). Not with behaviour change: if behaviour changes, base rates move and calibration metrics shift for a reason that is not calibration. |
| Changing behaviour (the crouton fast Freeze that does not carry the work) | Round outcomes: judge accepts, owner approves, rounds and minutes to an accepted candidate | Not with self-calibration as primary; may share as a secondary readout. Separate experiment with its own primary. |
| Routing by record (pool, model choice) | Outcomes of the routed work, cost | Nothing: it changes who predicts and who works, which confounds every other readout. Phase 2. |
| Ring-walk speed (the bridge) | Release Predicts settled by the walk; soak time saved; breaches missed | Can run concurrently with any desk experiment: different population (platform authors), different settler. Weighting the walk by record waits for n. |
| Owner trust | The owner reads the morning letter and the round line; does the owner approve faster or reject better | Shares with anything, as a display; is measured only by the owner's own rounds. |

---

## 7. Axis: levels

Settled observations at about 06:20Z, engineering desk only (management,
Texture and research digests have n = 0 on all five arms since the data
cutover; the 10-14 management baseline of 559 hypotheses is gone):

| Slice | n | Supported now? |
|---|---|---|
| Fleet, engineering, all claims | 341 | Yes: mean bias, Brier, decomposition. |
| Fleet, by claim type | tests_pass 128, completes 86, paths_confined 48, freeze_within 46, verify_pass 31, judge_accepts 1, freeze_minutes 1 | The top four yes; verify_pass marginal; the cut-1 and cut-2 kinds not yet. |
| Model | DeepSeek V4.1 flash ~180 (claude, deepseek, most of pool), gpt-6-luna ~145 (luna, part of pool), crouton-alpha 31 | Two models on aggregate bias; not model × claim type. |
| Computer | 31 to 127 | Each computer is mostly one model, so computer and model are confounded. Pool, the only mixed one, is the only computer where model can be separated within a computer. |
| Desk | engineering only | Management needs C1 before it has a record. |
| Model × type × arm | — | No. Do not show it in a digest (MinN = 20 already blocks it). |

The record's arm labels give an A/A check. On the cut-1 tree the
`calibration.feedback` setting reads false on all five arms, so (if no
digest turn was injected, which a trace check should confirm) the
"digest" and "control" labels are a placebo split: Brier 0.206 (n=191)
against 0.217 (n=150). A difference of 0.011 is the noise floor of a
between-arm comparison at this n.

---

## 8. The current baseline, stated precisely

On origin/main `ed2e54c6b` (cut 2), deploying; staging was on cut 1 when
read:
- **Who predicts:** management, in every engineering Cast, five mandatory
  claims (`verify_pass`, `judge_accepts`, `owner_approves` as p;
  `freeze_minutes`, `approval_minutes` as estimate, low, high, p; optional
  `approved_within_rounds`), refused with a literal example when missing.
  The engineer, optionally and prompted, in its first cell: `tests_pass`,
  `freeze_within`, `paths_confined`, `completes`, and any rule kind.
  Texture states numbers in its packet, unscored.
- **When:** management's at Cast (`stated_by` cast, never late); the
  engineer's any time, with clock claims after the first build or shot job,
  and any claim after the Freeze, kept but marked late and unscored.
- **Who scores:** the rule scorer from runtime events (Freeze, Verify,
  terminal, approve act). `judge_accepts` is management's own Verify.
  `owner_approves` is FP0's act on the arms, marked delegated.
- **Scoring:** Brier, ECE, mean p against observed, bins; the readout adds
  coverage, log ratio and log width for clock claims. The digest shows the
  binary part only.
- **Counted on:** engineering's record, whoever stated it.
- **Shown:** to engineering runs only, in the digest arm of a salted
  50/50 hash of the engineer's thread (the assignment), as a ~150-token
  user turn at activation start and after a changed refresh, and in
  `choir.Pack().Calibration`. Control runs get nothing, pushed or pulled.
  Aggregate only: overall Brier and ECE, direction, three to four bins
  over all claim types pooled, the least calibrated type, a model line.
  10% held out by salted hash. MinN 20, window 400.
- **To the owner:** `GET /api/calibration`, `/api/calibration/claims` (cut
  2), the round receipt line.
- **Not built:** pull arm, stub, event-timed, per-revision claims under
  reuse, `adopted_elsewhere`, `Predicts:` trailers, a clock line in the
  digest, any management, Texture or research push.

---

## 9. Sample-size arithmetic

Two-sided α = 0.05, power 0.8, so (z₀.₉₇₅ + z₀.₈)² = 7.85. Measured: sd of
(p − y) per claim 0.448, sd of the Brier term 0.239, within-round ICC 0.69.
With m settled claims per round, the sd of a round's mean is
σ·sqrt(ICC + (1 − ICC)/m); at m = 3.2 that is 0.887σ (0.40 for bias, 0.21
for Brier); at m = 8 it is 0.86σ, so more claims per round barely help.

- **Between-round A/B, mean bias** (the cut-2 design), δ = 0.10 (removing
  most of today's 0.10 to 0.16 over-confidence): n per arm = 2 × 7.85 ×
  0.40² / 0.10² ≈ **126 rounds per arm**, 252 in all. At 4 per hour for the
  fleet that is 63 h with every arm in one comparison; on one computer at
  0.8 per hour, 13 days.
- **Between-round A/B, Brier**, δ = 0.03 (the 10-14 harm-stop size): 2 ×
  7.85 × 0.21² / 0.03² ≈ **770 rounds per arm**. Not feasible; Brier
  differences are a harm stop, not a primary, at this rate.
- **Paired two-stage (d3)**, primary Δ = Brier(p1) − Brier(p0) on the same
  claim, with sd of the round-mean Δ assumed 0.04 (p moves by 0.05 to 0.15
  on a restatement; to be measured in the first 20 rounds), δ = 0.015,
  against a restate-without-record control: 2 × 7.85 × 0.04² / 0.015² ≈
  **112 rounds** in all, 56 per arm. At 4 per hour fleet-wide, about
  **28 h**; on crouton alone at 1.5 per hour, about 75 h.
- **Read rate (b, c)**: with all treated runs on that computer in the arm
  (no control is needed for a rate defined only under treatment), 20 runs
  give a 95% interval of about ±0.2 around 0.5; a go/no-go at 0.25 is
  readable after 20 runs. Engineer runs: ~25 h per computer at 0.8 per
  hour. Management activations are several per round, so its read rate
  reads out faster.
- **Mean stated p shift** (no outcome needed), sd of a round's mean stated p
  about 0.10 (to be measured), δ = 0.05: 2 × 7.85 × 0.01 / 0.0025 ≈ 63 rounds
  per arm. Useful as an early signal that a treatment does anything at all;
  not as the success criterion (a shift is not an improvement).

The programme below is designed around these numbers: within-act and read
measurements first, the between-round outcome A/B only for one finalist,
fleet-wide.

**Raise the rate.** The arms idle between requests and wait on FP0's
approval (34 minutes on one op). A standing queue of small requests per arm
and a faster approval loop would roughly double the rate; it is the
cheapest lever on every experiment's duration. It changes the request mix,
so it lands before the experiments start, not during one.

---

## 10. The open questions that decide the most

1. **Attribution and target.** Whose record does a Cast prediction count on,
   and who is shown it? Today it is stated by management and shown to the
   engineer as the engineer's own. Every experiment's subject depends on
   this; it should be settled before any treatment runs (C1).
2. **Delivery.** Does any desk read pulled data before it predicts, and does
   a record shown at the moment of predicting (d3) improve proper scores
   without losing resolution? These two answers prune the four arms to one
   or two cheaply.
3. **Calibration or discrimination.** Reliability is 0.016 of a 0.211 Brier;
   resolution is where the skill is missing. Does any content (per-type
   records, recent misses with their settled reason, clock bias) raise
   resolution, or does feedback only shift numbers down? If only the
   latter, the seed's value is a 0.01 Brier and the effort belongs to
   behaviour and claim design.
4. **Which clock.** Freeze minutes reward a fast Freeze the judge then
   rejects (crouton r4). Is the scored clock "to Freeze", "to a Freeze that
   passes Verify", or "to approval"; and with engineer reuse, is a
   revision's clock a fresh claim at the resume?
5. **The rate.** Can the arms reach ~125 rounds per arm for one confirmatory
   between-round A/B within a week? If not, the within-act designs are the
   evidence class, and the owner should know the between-round result will
   be "inconclusive" for a while.

---

## 11. The experiment programme

Rules for every experiment: one variable; one experiment per desk per
computer at a time; no experiment on a computer whose model or prompt is
changing for another reason (note every such change in that arm's
ledger); the 10% held-out set is the evaluation set for every outcome
metric; inconclusive is reported as inconclusive; no early stop for
success. Harm stops apply to every arm: Brier on the treated arm worse than
its control by 0.03 over at least 40 rounds; tokens per round up more than
10% (5% for push); late share up 10 points; read turns that crowd out work
(time to first build up 20%).

**Precondition (before wave 1):** cheap changes C1 to C7 land, ring by
ring, then a 12-hour no-change baseline window on all five arms.

### Wave 1 (concurrent; one experiment per desk)

**E1-M. Event-timed two-stage at management's Cast (delivery d3), all five
arms.**
- Variable: whether the result of the predicting Cast shows management's
  own per-kind record before its one restatement. Every Cast restates once
  (so the "second look" is in both arms); the arm decides only whether the
  record is shown. Randomised per Cast by salted hash of the Cast record id
  (finding 2).
- Primary: difference in differences of the paired Brier, (Brier(p1) −
  Brier(p0)) treated minus control, over the three binary claims, on
  held-out-eligible claims; co-primary for clock, the change in absolute
  log ratio of estimate to actual. Report resolution and sharpness of p1
  against p0 per type, so a uniform shrink toward 0.5 is visible.
- n: 112 Casts in all (section 9); at 4 per hour fleet-wide, about 28 h.
- Harm stops: as above, plus restatement turn tokens above 5% of the
  round's management tokens.
- If it wins: the record at the predicting act becomes management's
  delivery, and the same shape is offered to the engineer's first cell and
  to every revision under reuse.

**E1-E. Engineering delivery screen, one mode per computer, each with its
own no-seed control.**
- Assignment (concurrent, different computers): claude (a) push, the
  running cut-2 A/B, re-scoped to the engineer's own claims by C1;
  deepseek (b) pull; pool (c) stub; luna (d3) two-stage at the engineer's
  first cell, restate-without-record control; crouton no seed (the A/A
  baseline and the home of E3 later).
- Variable on each computer: its mode against none.
- Primary per mode: (b) and (c): `read_before_predict` (go/no-go at 0.25
  after 20 treated runs; for (b) and (c) every engineer run on the computer
  is treated, the no-seed comparison comes from crouton and the claude
  control half); (a): mean bias of the engineer's own claims treated vs
  control, accumulating toward wave 2; (d3): the paired Brier DiD as in
  E1-M. Cost per run for all four.
- n and time: reads, ~25 h; (d3) on one computer, about 140 h at 0.8 per
  hour. (d3)'s full readout therefore comes from E2.
- Mode is confounded with model across computers. The screen only prunes:
  it does not rank (a) against (d3).
- If (b) or (c) is read before predicting in under a quarter of runs, it
  is dropped (and with it the owner's REPL-only option for this desk).

Why concurrent is safe here: different desks; independent hashes;
management's restatement never reaches the engineer (P3, the engineer's
view of its Cast drops the predictions block). Why only this much: each
desk on each computer carries exactly one experiment.

### Wave 2 (sequential, after wave 1 reads out)

**E2. Confirmatory delivery, engineering, fleet-wide.** The best of the
surviving modes from E1-E against no seed, per-assignment 50/50, all five
arms (removes the model confound). Primary: mean bias on the engineer's own
held-out claims, with resolution reported; n 126 rounds per arm, about 63 h
fleet-wide. If (d3) survived and E1-M won, run (d3) here. If it wins: it
becomes engineering's delivery, and management, Texture and research get
the same shape one desk at a time (the seed doc's sequence).

### Wave 3 (sequential, one at a time, on the winning delivery)

**E3. Self-development optimises the presentation (crouton authors, others
evaluate).**
- An engineering round on crouton is Cast with the objective "change the
  calibration presentation to improve the stating desks' proper scores",
  with the round's own predictions about its effect (it eats its own
  record: "this presentation cuts mean log ratio by 0.1 on the evaluating
  arm within 60 rounds, p = ...").
- The editable surface is narrow: one pure renderer, record set in, text
  out (`internal/calibration` Render and its per-kind sibling for d3), at
  most 300 tokens, `paths_confined` to that package. The candidate is
  judged and approved as any round, published to the Library, and adopted
  by **a different arm** for the evaluation, randomised per assignment
  against the incumbent renderer there.
- Anti-Goodhart guards: (1) the authoring computer is never the evaluating
  one, so the author cannot tune to its own scorer; (2) the metrics are
  computed by the platform readout from raw records, outside the editable
  package, on the held-out 10% the renderer never receives; (3) the score is
  proper (Brier and log, interval score) with resolution and accuracy shown,
  never ECE alone; (4) a lint refuses directional instructions in the
  rendered text (any imperative about the direction of p or time, e.g.
  "estimate shorter", "be less confident"; the 11-01 definition's own
  Goodhart case), so the text stays feedback, not instruction; (5) one
  candidate evaluated at a time, at most three candidates per week, and the
  success bar fixed before the round (a Brier skill gain of 0.01 on held-out
  claims with resolution not lower); (6) the evaluating desk's model and
  prompt are frozen for the window.
- n: 126 rounds per arm at mean bias, or 56 per arm for (d3)'s paired
  metric; days per candidate. If it wins: the renderer ships, ring by ring,
  as any platform change; the authored text is the evidence that
  self-development can improve its own feedback.

**E4. Content: the clock line and per-type records** (if E3 has not
already found it). One variable: the digest with the clock bias line and
per-type records (with recent misses and their settled reason, no judge
text) against the wave-2 digest. Primary: resolution per claim type, which
finding 3 says is the real gap; secondary: absolute log ratio on clock.

**E5. Who predicts (observational, can run any time, no treatment).**
Management and the engineer already both state `verify_pass` on many
rounds. Compare their Brier and resolution on the same outcomes, per model.
If one is consistently better, drop the duplicate claim from the other
(fewer turns, same information). Needs ~60 paired rounds; accrues from
existing data during waves 1 and 2.

**E6. Behaviour: the clock that carries the work.** After the seed
experiments, change what is predicted: the headline clock becomes minutes
to a Freeze that passes Verify (C10). Primary: share of rounds whose first
Freeze is accepted, and minutes to an accepted candidate, against the
current freeze-minutes headline, randomised per assignment. This one
changes behaviour on purpose, so it never shares a window with E1 to E4.

**Later.** Routing by record on the pool arm; `Predicts:` trailers and the
ring settler (can start any time; different population); `adopted_elsewhere`
once the Library has adopters.

### Concurrency at a glance

| Window | management desk | engineering desk | other |
|---|---|---|---|
| Precondition | — | — | C1 to C7, 12 h baseline |
| Wave 1 (~1 to 2 days) | E1-M, all arms | E1-E, one mode per computer | E5 accrues; Predicts trailers may start |
| Wave 2 (~3 days) | E1-M's winner rides as default, no new test | E2, all arms | E5 reads |
| Wave 3 (a week or more) | — | E3, then E4 | — |
| Wave 4 | — | E6 | routing by record |

---

## 12. Cheap wiring changes worth making now

All are small, in code already read for this report. Marked for owner
decision where they change a ruled design or a desk's role.

| # | Change | Why | Owner decision? |
|---|---|---|---|
| C0 | A literal Precommit and Cast example | Management needed six tries | Done |
| C1 | Count each claim on the **stater's** desk (`stated_by`, already recorded): Cast predictions on management's record (subject: the assignment), the engineer's own on engineering's; the engineer's digest shows only its own claims; management becomes a push or event desk for its own Cast claims | Finding 1: feedback goes to a desk that did not state the claims, labelled "your" | **Yes**: the lane F spec says Cast predictions "count on engineering's record". Light: it is the reading of "engineering predicts" |
| C2 | Management's A/B unit is the Cast (salted hash of the Cast record id), not the thread | Finding 2: one thread per computer puts all management in one arm | No |
| C3 | Replace the anchoring numbers in the literal examples with numbers no one would state (0.5 everywhere, minutes 1 [1, 1]) and say "replace every number with yours"; keep the shape literal | `tests_pass` 0.8 most common; Cast example values recur | No |
| C4 | Digest bins per claim type, not pooled; exclude near-certain kinds (`paths_confined`, 96% held) from the bins and Brier, and show them as a coverage line | Pooled bins mix easy and hard claims (Simpson); trivia pads n | No |
| C5 | Add the clock line the owner described: median signed log ratio of estimate to actual ("your Freeze estimates ran 2.8× the actual minutes"), coverage against stated p, censored share; and a resolution or sharpness figure per type beside Brier | Owner 11-01 example and 11-03 "resolution so vagueness can't game it"; readout already computes the clock numbers | No |
| C6 | A dedicated `choir.Calibration()` read verb that writes a read mark (run, time, digest hash); `Pack().Calibration` for every run of a pull or stub arm | Needed for the read metrics; without it pull cannot be measured | No |
| C7 | Clock censoring: a round that ends before the interval's low bound is excluded (and counted as censored), after the high bound is a miss | Today every no-Freeze is "no", which scores crashes and abandons as forecasting error | No |
| C8 | Texture's packet predictions: either strip them from what management reads, or record them as Texture's own scored claims | They anchor management and are scored nowhere | **Yes**: Texture's role as a forecaster |
| C9 | The live8 trace export should carry desk precommits and settlement records (today only the guest `recent_log` lines, lost on reboot) | Trace review cannot audit calibration otherwise | No |
| C10 | Add "minutes to a Freeze that passes Verify" as a claim kind (settled at Verify), kept beside `freeze_minutes`; the headline clock in E6 | Fast Freezes that do not carry the work score as clock hits | No for the kind; **yes** before it replaces `freeze_minutes` as the mandatory clock |
| C11 | Show the late share in the desk's own digest | A desk that states late has learned nothing; the metric exists (R20) | No |
| C12 | Before reading cut 2's A/B: check one digest-arm engineer trace for the digest turn, and confirm `arm_rule` in `GET /api/calibration` no longer names the public `choir-icrl-v1` prefix once cut 2 is live | The A/A reading in section 7 assumes no digest was injected on cut 1 | No |

---

## 13. What would fool us

- A treatment that moves stated p down uniformly improves ECE on an
  over-confident desk and loses resolution: always report resolution and
  sharpness per type.
- A seed shown to the wrong desk: C1 before any readout.
- Model and mode confounded across computers: wave 1 prunes, wave 2
  confirms fleet-wide.
- The clock gamed by the agent that controls the event: clock claims stated
  by the freezer are self-fulfillable; prefer management's clock and the
  accepted-Freeze clock.
- Operator latency scored as the desk's forecast: report `approval_minutes`
  delegated and owner settlements apart.
- Too few rounds: the arithmetic in section 9 is the honest bound; an
  under-powered A/B is "inconclusive", not a lean.
