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
`scripts/selftest-gates-cover-ci`. It reads `.github/workflows/ci.yml` by hand
(narrowly; a shape it does not know fails closed) and compares it with
`scripts/gates` and `scripts/land` in both directions.

Forward: **every job's `run:` steps are scanned; no job is excluded**. (The
first version let a job be named "not a gate"; that hid
`go run ./cmd/lockscope`, a ratchet that refuses a push, inside
`go-vet-build`, and the line is gone.) Each repo program a step invokes
(`scripts/`, `frontend/scripts/`, `.github/scripts/`, `go run ./cmd/...`)
must be run by `scripts/gates` or `scripts/land`, or be named in
`scripts/gates-ci-exemptions.txt` with a reason. `go build`, `go vet` and
`go test` are not repo programs. `scripts/land` counts through the runner it
writes (the payload of its `echo` lines) and its
`for t in .github/scripts/*-test` glob, so the CI classifier tests in the
`plan` job are covered by the landing script on `.github/` changes.

Land coverage is conditional, and the file says so: `scripts/land` emits the
`.github/scripts/*-test` glob only on a `.github/` change (`GH=1`) and the shard
runners (`go-test-runtime-shards`, `go-test-with-timing`) only when agentcore
and textureowner both changed and not `--local` (`SHARD=1`). Each carries a
`land-conditional` line with its condition, which the check prints; a program
only land runs without such a line fails.

Interpreter forms are parsed, not skipped: `bash -e`, `-euo pipefail`,
`-o NAME`, `sh -c '...'` (the payload is scanned as a command line),
`env -i`; `bash -n` runs nothing. A form whose program cannot be determined
(`python3 -m`, `-c`, `node -e`, a `-c` with a variable) that names a repo
path, or a repo path in command position that is not a recognised form,
fails closed, in CI steps and in `scripts/gates` alike.

Converse: every executable `scripts/check-*`, `scripts/selftest-*`,
`scripts/test-*` and
`.github/scripts/*-test` must be run by CI, `scripts/gates` or `scripts/land`
or be named in the exemption file. A guard nothing runs can no longer sit
unseen. "Invoked" is one rule for all sides (the command word after
`run`, `bash`, `python3 -I`, `node`, `env`, `VAR=x`, `timeout`; `./x`, `$ROOT/x`
and `"$root"/x` count; a path in an assignment, an `echo` or a comment does not).
Fail closed: a command or script path built from `${{ }}`, a job with
`uses:` and no steps, a local action, an unbalanced quote, and a `run` line in
`scripts/gates` that is not a program or `go test|list|vet|build`.

Added to `scripts/gates` (Mac wall time): `scripts/doccheck` 26 s, the heresy
detector 5 s (report to a temp file), `check-base-packages` under 1 s, the
selftests of the kernel, desk-names, store-handles, owner-files,
switch-registry, guest-token and Jev-entry gates (1 to 5 s each),
`go run ./cmd/lockscope` about 1 s, `check-store-schema-floor.sh` and its
selftest under 1 s (the second incident's guard, now run here and in CI's
Vocabulary Gates), `check-package-export`,
`selftest-test-computer-create`, and the tests `test-check-guest-app-layer.sh`,
`test-guest-tool-exec-wrapper.sh` and `test-node-b-deploy-disk-preflight.sh`. `scripts/gates` went from 88 s to about two
minutes.

Still exempt, with reasons in the file: CI-only gates that need nix,
PostgreSQL, a merge base or a Linux userland, or take about a minute
(`selftest-guest-security-epoch` 59 s); CI planning and deploy programs whose
`-test` runs in `scripts/land`; the Go shard runners; two local actions.
Orphans the converse check found, exempted because they need the network or
nix and nothing runs them: `check-curated-go-modules`,
`check-guest-app-layer.sh`, `check-guest-data-fstrim.sh`,
`check-host-firecracker-pin.sh`; and four `scripts/test-*` that need a Linux
userland or are tools (`test-computer-create`, `test-node-b-growth-meter.sh`,
`-host-disk-alarm.sh`, `-nix-retention.sh`). After it, a new CI gate cannot be added, and
a new guard cannot be left unrun, without the exemption file saying why.
