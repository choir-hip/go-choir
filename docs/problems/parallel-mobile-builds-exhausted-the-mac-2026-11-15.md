# Parallel mobile builds exhausted the Mac and cost the first QA build (2026-11-15)

Recorded 2026-11-15, about 21:45Z.

## Problem

Between about 19:45Z and 21:39Z the lead ran five mobile workers at once on
the owner's Mac (16 GB, 8 cores). At least two ran cold Exact builds (the
Rust toolchain and the Swift host) at the same time. The Mac ran out of
memory. The build that was meant to prove the phone pipeline failed after
6,740 s, the owner killed every worker and background job by hand, and no QA
build was made in the two hours the owner had set aside for the first round.

## Evidence

- Load average 880 to 975 at 21:39Z; swap 10.7 to 11.8 GB of 12 to 15 GB
  used; about 24 `rustc` and `swift-frontend` jobs, each at 10 to 25% of a
  core, some running for over 100 minutes.
- The build engineer's `build-lab.sh ios` exited 1 with "the bake tool did
  not build" after three Rust crates were killed with SIGKILL.
- An earlier worker's contract build "spun" for ten minutes and was killed:
  the same cause, a cold compile under contention, read at the time as a
  possible hang.
- The Exact reference page already says one build at a time, one simulator
  at a time.
- The owner's kill also stopped the landing run of the capture-trigger fix
  (not pushed) and a code review.

## Belief

The cause is the lead's fan-out, not Exact and not the workers: the Go rule
(builders compile on the build VM) has no counterpart for mobile work, which
can only build on the Mac, so five workers each did what a Go builder does
and compiled.

## Rule from now

1. **One Mac build at a time, and the lead runs it.** A mobile worker writes
   code and may run the Swift package tests, one worker at a time; it never
   runs an Exact build, a device build or a simulator.
2. The QA build is assembled by the lead from the workers' branches and
   built once.
3. A worker's brief for mobile work says this in its first paragraph.

## Remaining error

`mobile/scripts/sim-slim.sh` (landed 2026-11-15) is not verified and nothing
makes a worker use it. The `lane` worktree tool is still not investigated.
Whether a single cold Exact build fits in 16 GB beside the owner's own apps
is not measured: the first serialized build will say.
