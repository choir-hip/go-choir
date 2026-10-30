# In-Context RL for Calibration: Experiment Design and Precommitment (2026-10-14)

Owner, 2026-10-14: "now we get to run the experiment to prove or provide
evidence against the notion that you can improve calibration with
in-context reinforcement learning."

This document is written and committed **before** the code that runs the
experiment, and before any treatment data exists. §3 is the
pre-registered precommitment: once committed it is not edited; changes
are appended in §10 with a reason and a date.

Inputs: the first box score
([claims-e0-e6a-results](claims-e0-e6a-results-2026-10-14.md) §4–§5),
owner notes 1 and 3 ([owner-notes](owner-notes-2026-10-14.md)), the
`model_id` problem
([desk-records-lack-model-id](../problems/desk-records-lack-model-id-2026-10-14.md)),
the prompt-cache work
([prompt-cache-breaks-across-turns](../problems/prompt-cache-breaks-across-turns-2026-10-14.md),
branch `worktree-agent-a2974b69ac404317d`, not yet on main).

Mutation class: **orange** (runtime context: a user turn and a pack
field; prompt lines; three optional self-score fields; one owner API).
No provider routing, VM lifecycle or Texture canonical-write semantics
change. §8 has the ceremony.

## 1. The question, and why management is the primary arm

In-context reinforcement learning here means: no weight update; the desk
sees a summary of how its own past probabilities fared against
independent settlement, and we measure whether its next probabilities are
better calibrated than those of the same desk, same model pool, same news,
without the summary.

E6a gives one desk with a large, measured, directional miscalibration:

| Desk | Claim | Baseline (E6a, paper, 2026-10-13/14) |
|---|---|---|
| management | Cast hypothesis `probability` | n = 559 resolved; mean p **0.63** vs **0.79** held (qualified = 0.5); Brier 0.116 resolved-only, 0.141 strict; reliability 0.031, resolution 0.013; 45% of p in [0.4, 0.6]; bins 0.37→0.62, 0.57→0.75, 0.74→0.84, 0.88→0.90, 0.96→0.83 |
| research | "completed" (implied p = 1) | coverage 0.65 of asked questions (n = 400 questions, 120 rounds); Brier 0.25 |
| texture | citation asserts support (implied p = 1) | supported 0.984 of judged (n = 1,830); 47% never judged |

Management is the primary arm: it already states probabilities, it is
the largest n (≈ 650 hypotheses a day), and its error has a direction a
digest can name. Research and Texture never stated a probability; their
arms start at zero self-scores and are exploratory.

Expected-calibration error (ECE) of the baseline, from E6a's five bins
(n-weighted |mean p − observed|): **0.144** for management.

## 2. What is built (summary; the code follows this commit)

1. **`model_id` on every desk record** (the problem doc's fix):
   `<provider>/<model>` of the model that answered, so every number
   below can be split by model.
2. **Self-scores, written before any independent settlement**, always
   on (both arms):
   - management: the existing hypothesis `probability`;
   - research: a final Report's packet carries `p_complete`, the desk's
     probability that the Report answers every question the Cast asked
     (stored as a `self` entry in the record's `scores`);
   - Texture: `choir.Write(edits, why, pSupported)`, the probability that
     each new citation's passage will be judged to support its sentence
     (stored on the revision's metadata and on each new citation's check
     memo).
   - One prompt sentence per desk asks for calibrated probabilities.
3. **Independent settlement on the tape.**
   - Texture citations: Jev's existing support verdicts (in the guest).
   - Management hypotheses and research completeness: no in-guest
     settler exists (§6, F-new-1). The E6a scorers settle them off the
     computer (the frozen M1 and R2 judge prompts, `gemini-3.8-flash`,
     blind to the desk's probability) and write one **calibration score
     record** per target through a new owner route,
     `POST /api/calibration/settlements`: an ordinary `resolve` record
     with the scorer's `scores` and `model_id`, a `calibration:` id, and
     discrepancy `unresolved`, so it never reads as a verdict in any
     desk's pack.
4. **The calibration digest**, computed in the guest from those
   settlements: per desk, the most recent 400 settled self-scores
   (excluding a 10% held-out set), Brier, ECE, mean p against observed,
   the five E6a bins, the claim type with the worst calibration, and the
   current model's own line when it has 20 or more.
   - Placement: a **user turn** headed `Calibration digest:` appended
     after the run's history (at activation start, and again when the
     digest changes, at most once per 30-minute refresh). It is never in
     the system prompt or the tool catalog, so the cached prefix stays
     byte-identical (§5, P1).
   - On demand: `choir.Pack().Calibration` in the desk's REPL (the same
     digest, same arm rule).
   - Read-only owner view: `GET /api/calibration` (digests, and with
     `detail=1` the observations and the pending self-scores).
5. **The switch**: the computer setting `calibration.feedback`, default
   **off**, set with `POST /api/computer-settings` (no deploy). Off: no
   digest is computed or shown. On: the digest goes to the treatment arm.

## 3. Precommitment (pre-registered; not edited after this commit)

**Hypothesis H-ICRL.** Showing a desk the aggregate calibration of its
own settled predictions in context makes its subsequent probabilities
better calibrated than those of a concurrently randomized control arm
that has the same prompt and the same self-score instruments but no
digest.

**Units and arms.** The arm is a deterministic function of the desk
thread (the run's agent id): `sha256("choir-icrl-v1:" + thread)`, first
byte even → `digest`, odd → `control`. Threads: each management task
(`management-task:<id>`), each story's Texture (`texture:<doc>`) and
research (`research:<doc>`) desk; the persistent management desk
(`management:<owner>`) is one unit and is reported separately, not
pooled. Randomization is concurrent, so news mix, time of day, model pool
and provider health are balanced across arms by design.

**Phases.**
- **Phase 0, instruments only** (setting off, after deploy): 48 hours or
  300 settled management hypotheses, whichever is later. Measures the
  prompt sentence's effect against E6a and seeds the digest. Checks that
  `model_id` is on 100% of new desk records and that settlement records
  land.
- **Phase 1, A/B** (setting on): until **500 settled management
  hypotheses per arm**, or 7 days, whichever is first. Settlement runs at
  least every 6 hours on hypotheses whose story has had ≥ 2 hours since
  the Cast; the same frozen judge throughout.

**Primary metric.** Management hypotheses committed during phase 1,
resolved outcomes only (confirmed 1, qualified 0.5, contradicted 0;
unresolved and no-evidence excluded, counted and reported), settled by
the frozen M1 judge (`HYPOTHESIS_SETTLE_V0`, sha256[:16]
`86a018c0353b05ac`, `google-antigravity/gemini-3.8-flash`):
**ECE over the five E6a bins** (<0.5, 0.5–0.7, 0.7–0.85, 0.85–0.95,
≥0.95), per arm. Difference Δ = ECE(control) − ECE(digest), 95% interval
by a story-clustered bootstrap (2,000 resamples of stories, seed
20261014).

**Secondary metrics** (reported, not decisive): Brier resolved-only and
strict; mean p − observed (signed gap); Murphy reliability and
resolution; hedge share (p in [0.4, 0.6]); the same per `model_id`;
the held-out 10% separately; research `p_complete` against the frozen R2
coverage judge (`COVERAGE_V0`, sha256[:16] `d26ef0b73cad0d6a`) on up to
120 sampled rounds per arm; Texture `pSupported` against Jev verdicts.

**Evidence for H-ICRL** (all three):
1. Δ ≥ 0.03 with the 95% interval excluding 0;
2. Brier (resolved-only) of the digest arm not worse than control by
   more than 0.005;
3. resolution of the digest arm not below control by more than 0.01 (a
   desk that collapses every p to the base rate gains calibration and
   loses discrimination; that is reported as "calibration by hedging",
   not as evidence for H-ICRL).

**Evidence against H-ICRL**: at the full sample, the upper end of Δ's
95% interval is below 0.03 (no effect of the size that matters), or
condition 2 fails with its interval excluding 0.

**Inconclusive**: anything else, including a stop before the full sample
for a reason other than harm. Inconclusive is reported as inconclusive.

**Stop rules.**
- Harm (checked once, at 50% of the sample, and daily for operational
  harm): digest-arm Brier worse than control by > 0.03 with interval
  excluding 0 → stop, setting off, report against. Run failure rate in
  the digest arm above control by > 5 percentage points, or desk input
  tokens up > 10% in the digest arm → stop, setting off, report as an
  operational failure, not as evidence about calibration.
- No early stop for success: the interim look is for harm only.
- Model pool change mid-window (a model added or removed): the primary
  analysis is within `model_id` strata, weighted by the pooled sample.
- Settlement outage > 24 h: pause the window clock; the digest freezes
  on its last value.

**What this cannot show.** A null result does not rule out ICRL with a
different digest wording, with per-item examples, or over longer
horizons; it rules out this digest at this size of effect. A positive
result is about calibration against this judge; the score-the-scorer
check (§4) bounds how much of it is learning the judge.

## 4. Measurement

Script: [`scripts/icrl_calibration.py`](../../scripts/icrl_calibration.py).
- `baseline`: recomputes the E6a management numbers (ECE, Brier, bins)
  from `e6a-raw.json`, the reproducibility check of the metric code.
- `pending` / `settle`: reads the computer's pending self-scores
  (`GET /api/calibration?detail=1`), settles hypotheses and completed
  reports with the frozen judges from a ledger pull, and (only with
  `--write`) posts the settlements. Default is a dry run.
- `measure`: per desk, arm, phase and model: n, Brier, ECE, gap, bins,
  Murphy decomposition, story-clustered bootstrap for Δ, the stop-rule
  checks and the verdict per §3. Output JSON under
  `docs/evidence/icrl-calibration/`.

Score the scorer: on a seeded 10% of settled hypotheses, a second blind
scorer (`gpt-6-luna`, as in E6a) settles again; the desk's Brier under
each scorer is reported per arm. If the digest arm improves under the
primary judge and not under the second, that is learning the judge.

## 5. Failure modes, written before the code

- **P1. The digest breaks the prompt cache.** Guard: it is a user turn
  appended after history, never the system prompt or tools. Test: the
  system prompt rendered for a run is byte-identical with the setting
  off, on/control and on/digest, and with or without a digest.
- **P2. The digest changes every call and floods context.** Guard: a
  30-minute cache per computer; a new turn only when the rendered text
  differs from the last one this activation sent or replayed.
- **P3. Control arm contamination.** Paths by which a control desk could
  see its outcomes: the pack (`choir.Pack().Calibration` follows the
  same arm rule), per-item verdicts in pack items (score records carry
  discrepancy `unresolved` and no observation, so a pack item stays
  unresolved), raw score records (they are ledger-only, addressed to no
  inbox, so they never appear in `choir.Records()`). Tests on each.
- **P4. Pending verdicts leak** (E6a guard). The digest reads only
  settled self-scores; a pending item contributes nothing.
- **P5. Small n noise.** No digest under 20 settled observations; bins
  under 5 are omitted.
- **P6. Wrong desk attribution.** Management tasks send as
  `management:<owner>`; the thread is the task (`acting_task`). Test.
- **P7. The digest query loads the store.** One computation per
  computer per refresh, only while the setting is on; off costs nothing.
- **P8. Self-score fields refused or malformed.** `p_complete` outside
  [0, 1] is refused at the call with the packet shape; `pSupported`
  outside [0, 1] is refused by Write. A missing self-score is allowed
  (no observation). Tests.
- **P9. Replay identity.** Self-scores are part of the act (they are in
  the intent) and are deterministic; `model_id` is excluded from replay
  digests (problem doc M1). Test.
- **P10. Score records read as verdicts.** Guard: `resolve` with
  discrepancy `unresolved`, no observation excerpt, no `resolved_at`.
  Test: an acting pack item targeted by a score record stays
  unresolved with no observation.
- **P11. Owner route misuse.** Only the owner; the target must exist on
  this computer and be the claimed kind (a Cast with those hypothesis
  ids, or a final completed research Report); outcomes from a closed
  set; one record per (scorer, target), idempotent; at most 500 per
  request.
- **P12. Gaming.** A desk could lower every `p_complete` to look
  calibrated. That is calibration by hedging; resolution is reported
  beside ECE and condition 3 of §3 guards the conclusion.

## 6. Findings made while designing (not repaired here)

- **F-new-1. Hypotheses cannot be settled in protocol.** Only the
  counterpart may settle a hypothesis (`store/inbox.go` stepConversation).
  Management's hypotheses are settled by the Texture that received them,
  and Texture's verb set has no Report (`yaegikernel/choir.go`
  deskModuleSets). Texture's hypotheses are settled by research, but
  `choir.ReportPacket` decodes with `DisallowUnknownFields` into
  `SourcePacket`, which has no `settled` field, so a packet that settles
  is refused (`yaegikernel/report_packet.go`). The fold, the store
  validation and `reportBodyFromPacket` all handle `settled`; no desk can
  send it. This explains E6a F4 (0 `Report.Settled`). The experiment uses
  off-computer scorers instead; repairing the protocol path is a
  separate decision (is a peer desk an independent settler?).
- **F-new-2.** The acting pack already carries aggregate tallies
  (`IssuerTallies`), an owner-directed relaxation of the score-free pack
  (`docs/desk-system-harness-architecture-2026-10-01.md`). The digest
  follows that precedent: aggregate signal only, never a per-item score.

## 7. Operating the experiment (after review and deploy)

1. Deploy; leave `calibration.feedback` off. Start phase 0.
2. Every 6 h: `python3 scripts/icrl_calibration.py settle --cred
   ~/.config/choir-qa/test-computers/autopaper-paper.json --write`
   (a write to the computer: requires the lead's go-ahead).
3. After phase 0's bar: `POST /api/computer-settings
   {"key":"calibration.feedback","value":true}`. Start phase 1.
4. Daily: `measure --phase 1 --harm-only`; at 50%: the harm look.
5. At the bar: `measure --phase 1`, write the result as a report, settle
   row 19 of `docs/calibration/claude-ledger.md`.
6. Rollback at any point: set `calibration.feedback` false (the digest
   stops on the next activation; nothing else changes).

## 8. Ceremony

- Mutation class: orange. Protected surfaces touched: none of red's list
  as a semantic change; Texture's turn path gains an optional
  self-score that is stored, not interpreted (no change to what Texture
  writes or to how a revision commits).
- Rollback: the setting (live, no deploy) for the digest; revert the
  commits for everything else. Records written meanwhile keep their extra
  optional fields; old readers ignore them.
- Heresy delta: discovered F-new-1; introduced none known; repaired E6a
  F2 (`model_id`) on the branch.

## 9. Claude's prediction

Recorded in [`docs/calibration/claude-ledger.md`](../calibration/claude-ledger.md)
open claims, row 19.

## 10. Amendments

- **2026-10-14, before any treatment data (clarification, no threshold
  changed).** Resolution in condition 3 is the Murphy resolution over the
  same five bins as ECE, as `icrl_calibration.py measure` computes it. On
  the E6a baseline that is 0.0060 (resolved only, n = 559); E6a's 0.013
  came from `claimslab`'s finer decomposition. ECE 0.144, Brier 0.116 and
  mean p 0.65 against 0.79 observed (resolved only) reproduce
  (`docs/evidence/icrl-calibration/baseline.json`).
- **2026-10-14 (clarification).** The script automates the calibration
  harm rule. The operational stop rules (run failure rate and input
  tokens per arm) are read at the daily check from `/api/runs` and
  `/api/costs`, splitting runs by `calibration.Arm(agent_id)`.
- **2026-10-14 (implementation).** Settlement scorer names are
  `e6a-m1-gemini-v0` (hypotheses) and `e6a-r2-gemini-v0` (completed
  Reports); one record per (scorer, target), so a target is settled once
  per scorer. The bootstrap clusters by the story the claim is about (the
  observation's `story`, the Cast's or Report's `texture:<doc>` inbox).
