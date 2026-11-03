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

## Problem 5 (hypothesis): management's Cast to engineering drops parts of the request

Claude arm round 3 (assignment-74d314c1d1429249750d8abb). The engineer asked
management for the twelve-entry screenshot list at 04:43:51Z and again at
05:12:03Z. Management replied at 04:51Z that "the Texture request packet
available to management contains only the actions and omits the
filename-to-preset-to-view/state list". The engineer's report says management
then read the list from the Texture document's revision 6. If the Cast body
carries only the actions, every detailed acceptance list in a request (shots,
features, measurements) reaches engineering secondhand or not at all. Status:
checked against the Cast record, 05:45Z: **partially confirmed, and the cut
is one level up**. Texture's Cast to management
(`cell:bd00c647-...:cast:tray-2`, 04:41:57Z, body 4,976 bytes:
actions, summary, owner_revision, owner_directions) named only the round-3
revision 27c25e19 in `owner_revision`. So `attachCastOwnerDirections`
(internal/agentcore/texture_owner_directions_cast.go:136-165) attached only
that revision's 2,264-character note, which points at "the twelve screenshots
the request lists" without the list. The request revision 91b253a0, which
holds the list, was never named, and `ownerDirectionFromRevision` (:238-261)
takes a document body only for the first, parentless revision. Nothing was
truncated (castOwnerTextMax 64 KiB). Engineering sees the parent Cast verbatim
(engineering_cast_request.go:101-146), so it lacked what Texture's Cast
lacked. internal/textureprompts/texture.yaml:32 asks Texture to list the
owner's request and each later direction; this Cast listed only the newest.
Fix direction: always attach the request-kind revision of the trajectory,
or have Texture name every owner revision. Management's own engineering Cast
was not in the trace records (hypothesis that it carried the same).

## Problem 6: a Library release cannot carry the app-sdk re-exports its package needs

`choir package publish` for pool selfdev-1d0527916b182fb513039514188a874d
(05:35Z) answered 409 package_change_not_confined: the operation also changed
frontend/src/lib/app-sdk.ts. All three candidates so far added re-exports
there (the request allowed it), because a package may import only app-sdk and
the file routes, fetchWithRenewal and the registry were not exported. Main's
fix: the re-exports on main (branch app-sdk-exports), reaching the arms
through the ring walk; then each arm's next round is package-only and publishes.

## Problem 7: the luna arm's capsule had no browser

Luna round 2 (assignment-43851d28d33d373927c8fb8a, 05:36-05:39Z): the
engineer's Playwright launch with `executablePath=$CHOIR_BROWSER` failed
because `CHOIR_BROWSER` was unset and no Playwright cache existed
("Executable doesn't exist at /tmp/.cache/ms-playwright/..."). The claude,
deepseek and pool capsules took screenshots in the same hour. Hypothesis: the
browser tool layer did not attach on luna's capsule (or its env did not export
it); check vmctl's "tool layers:" line for luna's last launch. The engineer
froze a 12-line stub and the judge failed it ("no browser behavior was
exercised because Chromium was unavailable").
