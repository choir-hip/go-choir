# Self-development two-arm drill: ImageApp, Sonnet delegate against GPT-6 Luna (2026-10-21)

Owner order: docs/reports/owner-notes-2026-10-14.md sections 32 and 33.
Both arms ran on test computers only: a frontend-only rewrite of
`frontend/src/lib/ImageApp.svelte`, starting from the same deployed commit
(8d027370b) with the same tracer scenario
(`scripts/reeng_tracer.mjs --scenario engineering --change rewrite --app ImageApp`).
Trace bundles stay outside the repo. Section 33 then stopped further Luna
arms and asked for Sonnet rewrites of six more apps; those are appended
below as they finish.

## Side by side

| | Arm B: GPT-6 Luna (engineering writes) | Arm A: Sonnet through the Claude delegate |
|---|---|---|
| Computer | freshly minted test computer | the allowlisted delegate test computer |
| Rounds | 1 | 2 (round 1 failed by design; round 2 with the carried patch) |
| Mint to engineering cast | 45 s | about 10 min (Texture and management on a computer carrying old history) |
| Cast to frozen | 3 min 34 s | not reached |
| Frozen to applied | 43 s, with owner approval at 03:42:08 | not reached |
| Restore | 03:43:11, `tape_reconstruct`, witness matched | not reached |
| Wall clock | about 6 minutes, mint to restore | 51 minutes, then stopped by the tracer's deadline |
| Where it stopped | done | round 2's `npm ci && npm run build` job was running when the tracer stopped the computer at 04:27:30 |
| Cost visible | not measured | delegate round 1: 13 turns, $0.13 |

## What each arm shows

**Arm B (Luna) went straight through.** Engineering wrote the rewrite in
its own capsule, built the frontend, froze at 03:41:40 and reported. The
owner approval, apply, route receipt and restore all answered on the first
try, and the restored checkpoint's witness matched. The capsule has no git
metadata, so the tracer could not record the size of the diff; the frozen
bundle digest is the receipt.

**Arm A (Sonnet) proved the carried patch and nothing after it.** In round
1 the delegate rewrote the file in 13 turns. Its sandbox refused the npm
build, which is the designed limit of the delegate's network mode, so
engineering reported a terminal failure and asked for a second round. In
round 2 the failed round's diff reached the new capsule as
`/selfdev/carried/source.patch` (6479 bytes), and the first `git apply`
succeeded. That is the first staging proof of failure 15's option A. The
build then ran as a job. The tracer's 50-minute deadline passed while it
ran, and the tracer stopped the computer, so the arm has no outcome.

## What the comparison is and is not

- It is one run each, on different computers. Arm A's computer carries
  weeks of earlier drills, and its Texture and management took about ten
  minutes to cast; a fresh computer took 45 seconds. That gap is not about
  the model.
- The delegate path costs a second round by design: the delegate cannot
  build, so a delegate rewrite always needs engineering to build it. On
  this evidence, the delegate arm costs at least one extra round and one
  cold `npm ci`, about 15 to 20 minutes, before the same freeze the Luna arm
  reaches in under four.
- Quality of the rewrite was not compared. Neither arm's diff was read
  side by side, and no reviewer scored them.

## Problems found (recorded in the engineering cluster doc)

Failures 16 to 18 and the tracer defects, in
docs/problems/engineering-completion-path-cluster-2026-10-20.md, section
"First carried-patch drill on staging":

- secret-named and sandbox-mask files rode in the carried patch (fixed
  test-first in 9c31920e6, not yet deployed);
- `choir.Exec` runs with working directory `/`;
- a long synchronous `choir.Exec` dies on the broker timeout;
- the tracer's deadline path threw "phase is not defined" (fixed in
  73922d044), cannot boot a stopped delegate computer, and 50 minutes is
  too short for two delegate rounds.

## Sonnet rewrites after section 33

Run one at a time on the delegate test computer, smallest app first, with
`--timeout-min 110 --keep-computer`.

| App | Lines | Label | Result |
|---|---|---|---|
| SlidesApp | 810 | sonnet-slides-1021 | running (round 1 failed by design at 04:47; round 2 cast 04:48) |
