# The calibration seed: what exists, what to test, in what order (2026-11-03)

Status: green (design; the experiment plan for station S6 of
[`choir-selfdev-apps-feature-complete-2026-11-03.md`](../definitions/choir-selfdev-apps-feature-complete-2026-11-03.md)).

Owner, 2026-11-03: "a calibration seed, where we show the model in addition
to its system prompt and REPL ... Maybe it would be in a prompt injected in
on model initiation, or maybe it would just be data accessible in the REPL.
We have to empirically test this ... History of predictions and scores ...
there's the model level, there's the desk level, there's the system level
... make sure that we're not running too many experiments at once."

## What the repo already has

- **The idea is on the roadmap.** `roadmap-2026-10-31.md` stage 3: the desk
  seed goes (`problems/desk-runs-seeded-from-prior-run-2026-10-31.md`, owner:
  "Each desk should start with system prompt and repl only"), and "in its
  place a desk may get a small calibration seed, built from its history of
  precommitment records and their scores."
- **The mechanism is built and off.** Commit 978522bc0 added self-scores,
  independent settlement records and an in-context calibration digest
  (`internal/agentcore/calibration_feedback.go`,
  `internal/types/calibration_digest.go`, `internal/calibration/`), behind
  the computer setting `calibration.feedback`, default false. The digest is
  aggregate only (bins of stated p against the share that held, Brier and
  ECE by claim type and by model), never a per-item verdict. It is appended
  as a user turn after history, so the cached prefix holds, and is also in
  `choir.Pack()`. 6c47ab868 added the settlement writer and the measurement
  script.
- **The experiment is designed and was never run.**
  `archive/reports/icrl-calibration-experiment-2026-10-14.md`: arms by a
  deterministic hash of the desk thread, concurrent control, primary
  outcome ECE difference with a bootstrap interval, harm stops (Brier worse
  by 0.03, token use up 10%), no early stop for success, "inconclusive is
  reported as inconclusive". Baseline then: management stated mean p 0.63
  against 0.79 held over 559 resolved hypotheses (underconfident), Brier
  0.116.
- **Owner rulings that bound it.** `precommitment-records.md` 4.7: a desk
  sees its own settled record when it predicts again, but independent
  scorers' judgments stay out of the acting context (or are sampled and
  delayed); automatic pack injection stays off until held-out evidence
  supports it (H2). 2026-10-24: engineering sees aggregates of its own
  calibration with a 10% held-out set. 5.3: calibrate per question type;
  report calibration with accuracy and discrimination, since a desk can get
  calibrated by becoming vague.
- **The gaps.** Engineering is absent from the digest
  (`calibrationDeskOfThread` maps only management, Texture and research),
  and a capsule has no pack (`tasks-precommit-icrl-2026-10-24.md` 1.5, 1.6).
  The held-out hash uses a fixed public prefix, so a desk that reads the
  source can compute it; the 10-24 design moves it to a per-computer salt.
  The data cutover moved the tape, so the 10-14 baseline must be recomputed
  on the new store before it is trusted.

## How it fits

The seed is the read side of precommitment records. PSA and the records
write the predictions; settlement scores them; the seed is what a desk sees
of that history when it predicts again. Without the seed, records are an
audit trail. With it, they are the learning loop the owner called "the core
scientific innovation of Choir."

The three levels are three slices of one record set, not three mechanisms.
The model level is records filtered by pinned model id; the desk level by
desk thread; the system level is the computer's whole record. The digest
already slices by claim type and model inside a desk. Two uses differ in
kind and should not be mixed:

- **Self-calibration**: a desk reads its own record to adjust its stated
  probabilities. This is the seed.
- **Routing by record**: management, or the pool, reads other agents' and
  models' records to choose who gets the work. That is an allocation
  policy, measured by outcomes of the work, not by the reader's
  calibration. It belongs to the pool arm later, as its own experiment.

Pull against push is the owner's open question. Pull (data in the REPL,
read on demand) changes nothing unless the desk chooses to read it, and
whether it reads is itself a measurable behavior. Push (injected at run
start) is the 10-14 digest arm. They are two treatments against one
control.

## The sequence

One experiment at a time on any one desk, and no experiment on a computer
whose model or prompt is changing for another reason.

1. **Remove the prior-run seed first.** It is a defect the owner ruled on,
   not an experiment, and it confounds any seed test: a run that inherits
   the last run's conversation already carries a crude, unscored history.
   Land it ring by ring with the usual proof, then hold a baseline window
   with no other change.
2. **Recompute the baseline** on the new store with the existing
   measurement script: per desk, claim type and model, n, Brier, ECE,
   resolution, and the self-favor gap. If management's records are too
   few since the cutover, keep collecting; do not lower the sample size.
3. **Run the 10-14 experiment as designed, management first, push arm
   only.** Management has the volume (hundreds of settled hypotheses), and
   the design, harm stops and analysis are already written. Two arms
   within each computer by the hash, so every computer is its own control
   and the model comparison across computers is not confounded. Move the
   hash to the per-computer salt before it starts.
4. **Then the pull arm** (REPL-only), as a second experiment on the same
   desk after the first reads out, or as a third arm only if the sample
   allows it. Record whether and when the desk reads it.
5. **Engineering last.** Its claims need defining first: what an
   engineering run predicts (that verify passes, that the judge accepts,
   that the owner approves, rounds to approval) and how each settles. That
   is the PSA work in S6; the self-development arms supply the records.
   At a few rounds a day per arm, engineering takes weeks to reach the
   minimum n, so it starts collecting now and is tested later.
6. **Routing by record** in the pool arm, after the seed reads out.

## What would fool us

- Calibration improves because the desk turns vague: report resolution
  and accuracy beside ECE, per claim type.
- The scorer drifts or is gamed: keep the held-out set and scorer audits;
  self-scores never govern.
- Two changes in one window: the sequence above, and a note in each arm's
  ledger of every prompt or model change during a run.
- Too small a sample: inconclusive is the honest answer, not a lean.
