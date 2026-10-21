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
  test-first in 85250d067, not yet deployed);
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
| SlidesApp | 810 | sonnet-slides-1021 (cbef56155) | stopped 05:08 after five rounds: management never cast a round with npm access (failure 19) |
| SlidesApp | 810 | sonnet-slides-1021b (d31403fd5) | stopped 05:28: after the prompt fix, management cast round 1 as the build round; the brief demanded `git diff` in a capsule without `.git` |
| SlidesApp | 810 | sonnet-slides-1021c (d31403fd5, brief fixed) | stopped 05:50: round 1 delegate rewrite (25 turns, $0.28); round 2 applied the carried patch, `npm ci` and `vite build` ran; the release copy failed on `fchmodat2` (failure 20) |
| SlidesApp | 810 | sonnet-slides-1021d (4d45601d0, fchmodat2 allowed) | stopped: round 1 delegate rewrite; round 2 built, but `cp` into the release tree failed with EPERM and freeze refused. Cause: the capsule lower is the guest root, whose applied release is owned by an unmapped uid (failure 21, which corrects failure 20) |
| SlidesApp | 810 | sonnet-slides-1021e (871c3c145, own release dir) | stalled 06:54: round 1 delegate rewrite (15 turns, $0.20); round 2 applied the patch and `npm ci && npm run build` passed, then completed as blocked on the `diff -u` exit code and the missing `.git`, with no release tree and no freeze; round 3 had no carried patch (failure 22, a brief defect) |
| SlidesApp | 810 | sonnet-slides-1021f (f9ad90709, brief fixed) | stopped 07:29 after four rounds: round 1 delegate rewrite (19 turns, $0.20); round 2 completed while its build job ran (failure 23); rounds 3 and 4 hit `cp` EPERM on new files from refused xattr calls; round 4's retry built and copied, but Freeze refused it because the failed first job had left build outputs in the workspace (failure 24) |
| SlidesApp | 810 | sonnet-slides-1021g (16a5aa0e0, xattr EOPNOTSUPP, Complete guard) | stalled 07:54 after round 1 (21 turns, $0.21): management read "awaiting the carried patch" as an input it had to supply and refused to cast the build round (failure 25) |

Each stop is a Choir or drill defect found and fixed, in order; none is
about the delegate's work. The rewrites of Pdf, Epub, Podcast, Calendar and
FileBrowser wait until SlidesApp reaches a freeze.
