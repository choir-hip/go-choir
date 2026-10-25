# CI prove once: a green pull-request run is the verdict for the identical commit (C3 design, 2026-10-24)

Station: CI0, slice C3 (docs/definitions/ci-test-cooptimization-2026-10-24.md).
Mutation class: red (it decides whether a deploy may proceed). Status: design
for FP0 review; the implementation lands behind a switch that is OFF.
Review tier: 1 (one seat, Devin), additive, one-act rollback.

## What it buys

Today a change is tested twice: once by the pre-land mirror (light, no
agentcore or store), once on main by the full run, where a failure costs a
fix-forward cycle (W2, 2026-10-24). With this change the pull-request run is
the pre-land gate, run in parallel with review, and the main push of the
exact commit that run passed does not run the Go test lanes again. Main push
to deploy becomes the Node B prebuild floor (about 8 min) instead of the
test lanes plus the floor.

## The rule (what "identical" means, and why it needs no new trust)

A main push of commit `AFTER` may reuse a pull-request run `R` only if all of:

1. `R` is a `pull_request` run of `.github/workflows/ci.yml` with
   `head_sha == AFTER` and `status == completed`, `conclusion == success`.
   FP0 lands the reviewed PR head commit as is (fast-forward), so the commit
   that reaches main is the commit the run tested.
2. The tree `R` tested equals `AFTER`'s tree. `R` checked out GitHub's merge
   of the PR head into the base at run time (`pull_requests[0].base.sha` in
   the run object). That merge has the head's tree when the base commit is an
   ancestor of the head, so the rule is: `compare/<R base sha>...AFTER`
   reports `ahead` (or `identical`). If main moved and the head was not
   rebased onto it, the merge tree differs from `AFTER`'s tree, the compare
   reports `diverged`, and the push runs everything.
3. `R` tested what main would test: its jobs include `success` for every Go
   test lane (non-runtime shards 0-7, agentcore/textureowner shards 0-7, the
   scale lane, Go Vet + Build) and `check`, matched by job name prefix and
   counted (8, 8, 1, 1, 1); `R`'s plan selected the Go lanes and the race
   mode (rule 4).
4. `R` ran under -race. On a pull request the plan selects the race matrices
   always (today only high-risk paths do, and main samples), so the gate is
   at least as strict as a main run. Reuse requires the run's plan job log
   line `race_selected=true`; the script reads it from the run's jobs
   (job names carry `race`).

Everything else about main stays: doccheck, heresy, v1-inventory, the repo
gates, build-frontend, deploy-impact, the prebuild and the deploy run on the
main push as today. Only these jobs are skipped on reuse: go-vet-build,
go-test, go-test-runtime, go-test-scale. `check` still runs and still
requires them: a skipped lane passes `check` only with the reuse record.

Why this adds no trust beyond today: a docs-only push already skips the Go
lanes on main because the classifier says no Go-relevant path changed. Reuse
extends that to "the Go-relevant tree is the one a green run tested", with the
run named and checkable.

## How it fails closed

The new step lives in `plan` (after classify) and runs only when the switch is
on and the event is a push to main. Its only output that can skip work is
`reuse_pr_run=<id>`; every other outcome sets it empty and the lanes run:

- any `gh api` error, timeout, rate limit, empty or unparseable answer;
- more than one candidate run (take none: ambiguity runs the tests);
- a rule above failing or not decidable;
- the switch off, the event not a push to main, `AFTER` not on main.

`check` re-derives nothing it cannot see: it requires, for each Go lane,
either `success`, or `skipped` together with `plan.outputs.reuse_pr_run`
non-empty and `plan.outputs.go == true`. A skipped lane with an empty reuse
output fails `check` with the lane named (today `require_success` already
fails a skipped selected lane; the exception is the only new branch). The
reuse record (run id, URL, head sha, base sha, job counts) is printed in
`plan` and in `check`'s log and written to the step summary.

The workflow contract test pins: the switch default is `false`; the lanes'
`if` carries the reuse condition and nothing else new; `check` keeps its
`needs` and fails on a skipped lane without the reuse output; the script
exits non-zero only on usage errors and prints an empty reuse id on every
unprovable case. The script has a fake-`gh` test over each failure above.

## The switch

`env.CI_REUSE_GREEN_PR_RUN` at the top of `ci.yml`, value `'false'`. Turning
it on is a one-line commit that FP0 makes after the draft pull-request runs
show the PR gate's timing and one landing shows the record. It is a constant in
the file (reviewed, visible in the run), not a repository variable.

## Rollback

Set the constant to `'false'` (or revert the commit): every main push runs the
Go lanes again, no state to clean. Nothing is written to GitHub.

## Residuals and risks

- The PR run must finish before landing, or the push finds none and runs the
  tests (safe, slow). FP0's landing procedure should wait on `gh pr checks`.
- A landing that is not a fast-forward of the reviewed head (a squash, a
  rebase) changes `head_sha`; it finds no run and tests. Land the head.
- The PR run costs runner minutes the mirror did not; the 20-slot account
  limit is shared with main runs. PR runs cancel superseded runs (their own
  concurrency), main runs queue.
- A PR that edits `ci.yml` to skip lanes is caught by rule 3 (job counts).
- GitHub's merge ref can be recomputed after the run when main moves; the rule
  uses the run's recorded base sha, not the live ref.
