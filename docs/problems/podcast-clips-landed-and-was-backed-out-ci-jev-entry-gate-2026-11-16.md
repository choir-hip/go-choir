# Podcast clips landed, CI refused it, and it was backed out

2026-11-16, 05:10Z.

## Problem

The podcast clips series (18 commits, tip 93004c45f) passed four review
rounds and `scripts/land` (every suite green) and was pushed to main at
04:30Z. CI run 43804540972 failed. Nothing deployed; staging stayed on
a99e07d3b. Main was red for about 35 minutes until the revert af31e4fd0.

## Evidence

CI job `Vocabulary Gates`, step "Check the Jev entry point":
`check-jev-entry: internal/platform/podcast_match.go calls Jev directly and
is not on the list.` The gate (`scripts/check-jev-entry`, owner's rule of
2026-11-15) allows a Jev call only from the files on
`scripts/jev-entry-baseline.txt`, and with `--base` that list may only
shrink. The match job in corpusd builds and sends its own judgment request
through a gateway host caller.

## Two defects

1. **The match job is a new direct Jev site.** It makes one call a run, but
   it packs its own request with its own bounds rather than through the one
   job entry point (`jevJob`, in agentcore). That is the shape the owner's
   rule forbids. Four rounds of review did not see it: no review prompt named
   the Jev entry rule, and the lead did not read section 10 of the Jev design
   before ordering a Jev caller (the contract's "read the reference page
   before touching a new dependency").
2. **`scripts/land` does not run a gate CI runs.** `check-jev-entry` is in
   `.github/workflows/ci.yml` only, not in `scripts/gates`, so a landing can
   pass locally and fail on main.

## Belief

corpusd cannot import agentcore, so a conforming match job needs the job
entry point in a shared package and the gate taught to admit a new ENTRY
file. That changes the owner's gate and is the owner's decision.

## What was done

- Backed out as af31e4fd0 (one seat verified the tree equals a99e07d3b
  outside `docs/`).
- The series is being re-cut without the automatic match job: kinds, reads,
  operator writes, the worker, the programme's clip segment, and an operator
  tool that lists ranked candidates (no model) and writes judged clips
  through the existing verified route. Matching tonight is an operator act.
- `check-jev-entry` is being added to `scripts/gates`.

## Remaining error

Other CI-only gates may exist that the landing script does not run; the
workflow has not been compared with `scripts/gates` step by step. The
automatic match job needs the owner's decision on the entry point.

## The same class a third time, and the clustering assessment (2026-11-18)

The remaining error above came true. The search-reuse landing
(`3f52dd46a`) passed `scripts/land` and was refused by CI's Vocabulary
Gates: `guest-floor-paths: FAIL: floor package internal/searchquery is
imported by a floor command but is not covered by nix/guest-floor-paths.txt`.
`scripts/gates` did not run `scripts/check-guest-floor-paths`. The deploy
was skipped; the fix (`c29c58798`: the list entry, and the check added to
`scripts/gates`) cost about 25 minutes of a change meant to cut search
volume while the credits ran down.

Three in a week (contract rule 3): the Jev entry gate (2026-11-16), the
store schema floor guard (2026-11-15,
[record](store-schema-floor-guard-is-not-run-2026-11-15.md)), the guest
floor list (2026-11-18). Each time one line was added to `scripts/gates`.
That is the symptom. The substrate: **`scripts/gates` is a hand-kept copy
of the gate steps in `.github/workflows/ci.yml`, and nothing compares the
two.**

The comparison, made by the lead on 2026-11-18 (scripts a CI gate job runs
that `scripts/gates` does not name):

| CI job | Script | Matters to a landing |
|---|---|---|
| Docs Truth Check | `scripts/doccheck` | yes |
| Heresy Detector | `scripts/check-heresies.sh --fail-on-regression` | yes |
| Base Packages Are Listed | `scripts/check-base-packages` | yes, when a package directory is added |
| Guest Floor Digest Is Commit-Free | `scripts/test-guest-floor-commit-free` | when `nix/` or the floor changes (needs nix) |
| Vocabulary Gates | `scripts/test-node-b-guest-release-retention.sh` | when that script changes |
| Vocabulary Gates | the selftests of the kernel, desk-names, store-handles, owner-files, switch-registry, guest-token, security-epoch and Jev-entry gates | when a gate script changes |
| Receipts Index Swap Script | `scripts/receipts-index-swap-test` | needs a throwaway PostgreSQL |
| Package Modularity | the SDK surface scripts | when the frontend SDK changes |
| Desk Protocol Lock | `scripts/fuzz-broker-protocol.sh` | fixed-time fuzz |

Fix being built (yellow): a check, run by `scripts/gates` and by CI, that
reads the workflow's gate jobs and fails when a script a gate job runs is
neither run by `scripts/gates` nor named in a short exemption list that
gives the reason (needs PostgreSQL, needs nix, selftest of an unchanged
gate). The gates CI has and the landing lacks are added to `scripts/gates`
in the same change. After it, a new CI gate cannot be added without the
landing script learning of it.

## A fourth time, before the fix landed (2026-11-18, 21:24Z)

The train of 18 November (heard memory, full-page search, the deploy-race
fix, the batched closure) passed `scripts/land` and pushed `7119f6ab5`; CI
run 44400808469 then failed in "Lock scope ratchet (O12)", a gate
`scripts/gates` does not run (it is one of the lines the fix above adds):

    NEW internal/platform/capture_fulltext.go:630 Store.CatchUpCaptureFullText holds ft.catchMu across chan select

Nothing deployed; staging stayed on `38d86121e`. The finding is true and the
hold is intended: `catchMu` lets one catch-up pass run at a time. It is
taken only by `CatchUpCaptureFullText`, whose one production caller is the
single background goroutine started once per store; a search or an inline
page write never takes it. What contends is a second catch-up pass, which
must wait for the first. The rest it is held across is the pass's own pause
between batches, and the pass returns when its context ends. The key is
added to `cmd/lockscope/baseline.txt` with this as its reason.

Cost this time: one failed CI run and about forty minutes before the train
could deploy. The fix for the cluster (`lane/gates-cover-ci`) is in its
last review.
Built (yellow): `scripts/check-gates-cover-ci`, run by `scripts/gates`
and by CI's Vocabulary Gates job, with its selftest
`scripts/selftest-gates-cover-ci`. It reads `.github/workflows/ci.yml` (by
hand, narrowly; an unparsable workflow fails closed), takes every repo
program a gate job's `run:` step names (`scripts/`, `frontend/scripts/`,
`.github/scripts/`, `go run ./cmd/...`) and fails, naming job, step and
program, when `scripts/gates` neither runs it nor
`scripts/gates-ci-exemptions.txt` lists it with a reason. A stale
exemption, one for a program `scripts/gates` runs, and a reasonless one each
fail, so the list cannot rot.

The rule for a gate job: every job is one, except a job the exemption file
names on a `not-a-gate` line with a reason. A new job is covered by default.
Not gates (14): plan, go-vet-build, go-test, go-test-runtime, go-test-scale,
govulncheck, check, sbom-candidate, sbom, build-frontend, deploy-impact,
flakehub-publish, prebuild-staging, deploy-staging (they plan, build, test the
Go suites, publish or deploy; `scripts/land` owns the Go suites). Gate jobs
(10): doccheck, heresy-detector, v1-inventory (Vocabulary Gates),
base-packages-listed, guest-floor-commit-free, boot-prune-unit,
receipts-index-swap, package-modularity, desk-protocol, npm-audit. The `plan`
job also runs the tests of the CI classifiers; it is listed as not a gate
because the classifier, not the landing, decides when they matter.

Added to `scripts/gates` (wall time on the Mac): `scripts/doccheck` 26 s,
`scripts/check-heresies.sh --fail-on-regression` 5 s (report to a temp file),
`scripts/check-base-packages` under 1 s, and the selftests of the kernel,
desk-names, store-handles, owner-files, switch-registry, guest-token and
Jev-entry gates (1 to 5 s each), plus the new check. `scripts/gates` went from
88 s to about 2 minutes.

Still exempt, with reasons in the file: `selftest-guest-security-epoch` (59 s,
over the budget), `test-node-b-guest-release-retention.sh` (fails on macOS:
BSD `touch -d`; 7 of 43 cases), `test-guest-floor-commit-free` (needs nix),
`receipts-index-swap-test` (needs PostgreSQL), `fuzz-broker-protocol.sh`
(fixed-time fuzz) and `frontend/scripts/sdk-surface.mjs` (frontend install and
a merge base). After it, a new CI gate cannot be added without the landing
script learning of it or the exemption file saying why not.
