# Arm rounds build for 50 minutes and then fail at Freeze; capsules cannot show real content (2026-11-03)

Status: open. Mutation class of this doc: green. Found by lane C (FP0) on the
first Files (reinvented) round on the four test arms. Evidence is metadata
from the CLI and the arms' ledgers, read by the Sonnet monitors.

## Problem 1: a computer whose self-development mode is "off" still admits engineering

All four stock arms (claude a875023a, deepseek e80d229b, luna 2bf3cab1, pool
4d38420c) had `self-dev mode get` = `off`, generation 0. Management Cast the
Files request on claude and deepseek at 03:15Z; each engineer read, wrote,
built and screenshotted for about 50 minutes; then `choir.Freeze` refused:
"self-development candidate: the computer's mode does not authorize a
proposal: mode is "off"" (claude 04:01:31Z and 04:02:04Z; deepseek
04:05:14Z, `internal/agentcore/freeze_candidate.go:40`). The capsules were
revoked, so the work was lost (engineering.carry_unfinished is off on the arms).

- The mode is consulted only at Freeze. Nothing at Cast or engineering
  admission (`internal/agentcore/engineering_admission.go`) checks it, so a
  round that cannot end in a proposal spends a full build first. Round 1
  cost about $1.0 (claude, plus $2.81 of delegate) and $1.4 (deepseek).
- `scripts/test-computer-create` sets neither the mode nor `files.object`;
  a fresh test computer cannot complete a self-development round.
- The refusal did reach Texture in prose (deepseek's turn at 04:07:03Z says to
  enable the mode and retry), which is right; the gap is the order of checks.

Recovery used: `choir self-dev mode set --mode propose_only
--expected-generation 0` on all four arms (04:07Z, generation 1), then an
owner revision on each document.

Fix direction (orange): check the mode at admission and refuse the Cast with
the reason on the commitment ledger, or let test-computer-create set
`propose_only` for test computers. Either one removes the class.

## Problem 2: the capsule cannot read the computer's files, so screenshots are not of real content

The spec ("Proof is real content") and the screens-as-files design ask the
engineer to screenshot the app showing the files on the computer. Both
engineers report that the capsule has no live `/api/files` and no view of
the owner's file tree (16 seeded files on each arm were invisible). They
served stand-ins: claude generated files with the same names; deepseek used
repository samples. So every Files screenshot is of stand-in content, which
is the fixture pattern the owner rejected, now produced by the engineer
instead of the platform.

Fix direction (red: capsule isolation, owner data): a read-only, per-round
grant of named owner paths (or a snapshot copy) mounted into the capsule,
recorded on the tape; or the screenshots move after apply, taken on the live
computer by the engineering desk outside the capsule.

Interim answer (main, 2026-11-03): stand-ins are acceptable inside the
capsule this round, but they are not the acceptance evidence. After a
candidate is applied on its arm, FP0 takes the real-content screenshots on
the live arm with a Playwright spec run from the Mac, signed in as the arm
with its key, showing the arm's 16 seeded files in the bar's presets and
sizes. FP0 decides on those, and they are the morning evidence. The red
design (a read-only grant of named owner paths into the capsule) waits for a
later decision. Problems 1 and 4: a builder is on branch
selfdev-mode-at-admission; the arms keep their manual settings.

## Problem 3: the computer's source tree predates docs the request cites

The capsule's tree is the computer's own `Source/platform`, written at its
creation (00:34Z). `docs/design/apps-feature-complete-spec-2026-11-03.md`
landed after it, so both engineers searched for it and worked from the
request text. Workaround: the round-2 revisions quote the spec sections
verbatim. This is by design (a computer's source is its own), but a request
that cites repo docs has no way to know which the arm holds.

## Problem 4: the Claude delegate cannot compile inside the capsule

On the claude arm the engineer started one delegate run at 03:24:21Z
(`choir-delegate`, 58 turns, about $2.81, 156,770 output tokens by its
report) and polled it 25 times. It reports the delegate's sandbox has no
bwrap, so the delegate could not run builds; the engineer built and fixed
everything itself. A delegate that cannot run `npm run build` writes code
blind. Cause is a hypothesis until the delegate's own log is read
(`internal/claudedelegate` notes a bubblewrap sandbox).

## Remaining error field

- Problem 1 class fix not landed; arms recovered by hand.
- Problem 2 open; FP0 judges round 2 on stand-in content and names it.
- Problem 4 hypothesis; check the delegate log on the next claude round.

## Owner ruling (2026-11-03 ~04:25Z)

On problems 2 and 3: "capsules should be an overlay over the exact computer
state"; and "computer source tree should auto update." A capsule's lower
layer is the computer's exact current state (its files, its source tree,
its data), read-only; the capsule's writes live in its own upper layer and
reach the computer only through Freeze, verify and apply. This supersedes
the interim answer above (live-arm screenshots after apply) once built, and
it supersedes "a read-only grant of named owner paths". The source tree
stops being a creation-time copy: the overlay's lower is the computer's
current source, kept current by source projection after every deploy. An
investigation of the gap (today's capsule root and mounts, the isolation
boundary that keeps owner data out, consistency of a live store under the
overlay, secrets that must stay out) is running; the design follows from it.
