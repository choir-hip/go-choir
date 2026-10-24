# Staged engineering rounds break: the desks read a planned "blocked" as a failure, and a cancel strands the carried work (2026-10-24)

Date: 2026-10-24, 08:00 to 09:40 UTC. Found by: the FP0 app-rewrite
conductor on the test computer computer-6c50d6e5 (b@c.com), staging (guest
builds 911be16b1, then 22a01746f). Run log:
`/Users/wiz/Dev/lanes/_runs/apprewrite/log.md` (outside the repo). Class of
this record: green. Class of a fix: orange (desk prompts and the engineering
result vocabulary), possibly red (the carried-patch chain and trajectory
settlement). Status: problem recorded, not fixed. The conductor moved back to
one long round (v5).

## Why staged rounds were tried

A five-app frontend change was too long for one engineering round while the
platform Dolt kept stalling: two rounds died after 30 to 40 minutes
([long-engineering-run-dies-on-corpusd-cas-invalid-connection](long-engineering-run-dies-on-corpusd-cas-invalid-connection-2026-10-24.md)).
Applies on this computer do not compound
([successive-selfdev-applies-do-not-compound](successive-selfdev-applies-do-not-compound-2026-10-24.md)),
so all five apps had to land as one candidate. The plan used the carried
patch (`internal/agentcore/carried_patch.go`): five short rounds on one
request. Each of stages 1 to 4 ends with a deliberate
`choir.Complete("blocked", ...)` whose summary starts "Stage N of 5 done:".
The next round finds that work at `/selfdev/carried/source.patch`. Stage 5
freezes.

## What happened (trace evidence)

The rounds themselves worked. Every stage built cleanly, and the carry grew
from 4 files (stage 1) to 6 (stage 2) to 7 (stage 3). The chain broke between
the rounds:

| UTC | Event |
|---|---|
| v3, 08:10 | Stage 1 ends blocked. Management casts an extra round (ec97ec14, "Recover and finish Stage 1 ... No freeze") that Completes **completed**. `pickCarriedCandidate` ends the chain on a completed newest round, so stage 2 starts from the base without the Files work. |
| v3, 08:28 | Texture reads stage 2's blocked report as "a failed, unfrozen assignment". Management casts a Reader-only recovery round that is told to freeze. That candidate would have dropped Files and reverted the applied Slides. The conductor cancelled it. |
| v3, 08:38:53 | After the cancel, the trajectory casts engineering again by itself. The conductor ran `trajectory cancel`. |
| v4, 08:54:39 | Same request, plus rules: "always Complete blocked before stage 5", "one assignment per stage", "a 'Stage N of 5 done:' report is planned progress". Texture still reads stage 1 as "an assignment failure before freeze" and asks for recovery. The request's rules never reach engineering: management paraphrases the cast. |
| v4, 08:57:00 | An owner `texture revise` ("stage 1 succeeded; cast stage 2; carry these rules word for word") fixed it. Stage 2 then rolled into stage 3 with no intervention. |
| v4, 09:21:44 | Texture misreads stage 3 ("its assignment ended failed"). The recovery round, cast with "Do not Freeze", **froze anyway** (op selfdev-6da07a98, created 09:23:03). It then Completed blocked, and the op went to `failed`: "assignment failed; its candidate cannot be verified". |
| v4, 09:33:17 to 09:33:36 | A second stage 3 retry was cancelled by the conductor. Texture wrote "the staged chain now awaits the owner's decision". **trajectory_settled.** The owner revise is refused with `409 durable lifecycle state is unavailable or terminal`. |

## Why (code read)

- A blocked result is a claim of failure by design.
  `assignmentSubjectBinding` (`internal/agentcore/engineering_assignment_fate.go`)
  groups `blocked` with `failed` as "a report that claims failure". The
  platform therefore records the assignment as failed, and the desks read
  the platform truthfully. The staged plan used a failure path as a
  continue path. The desks' reading is not the defect: the defect is the
  missing result for "this round's work is good and the next round
  continues it".
- The carried chain is per trajectory (`CarriedCandidates(ctx, owner,
  computer, trajectoryID)`). Once the trajectory settles, the work can no
  longer be carried anywhere. The frozen patch was saved locally through
  `choir self-dev patch --operation selfdev-6da07a98 --out` (295,321 B,
  sha256 7e828539…). No product path puts it into a new trajectory's
  capsule.
- A completed newest round ends the chain (`pickCarriedCandidate`). Any
  verification-only or "recover" round a desk inserts, if it completes,
  throws away every earlier stage.

## What a fix has to do (not decided; failure modes first)

- Give engineering an honest non-terminal result, such as "progress" or
  "continue", that carries the round's tree and that the desks read as
  success. Alternatively, let "partial" carry and teach the desks that it
  is progress. The result must not certify the subject, because nothing
  froze.
- A request whose work lives in a carried chain should not settle while an
  unfrozen carried candidate exists. If it does settle, the owner should be
  able to continue the request into a new trajectory with the carry.
- Desks should pass the owner's operating rules to engineering verbatim,
  not paraphrased. The v4 rules reached engineering only after an owner
  revise asked for them word for word.
- Engineering should not Freeze when its cast says not to. Report it as a
  model-behaviour residual; Freeze itself was correct.
- Failure modes: a "progress" result that never ends (bound the chain
  length); a carry across a base move (the offer already records
  BaseMoved); a carry into a different request's trajectory (owner act
  only).

## Related findings from the next attempt (v5, one round, 09:40 to 10:38)

- **The request reaches engineering cut at 4096 bytes.** Engineering reported
  that the owner request it received was "TRUNCATED at 4096 of 6338 bytes"
  (its Reports at 09:44:26 and 10:24:26). Everything after Podcasts' second
  bullet was missing, including the allowed-files, package.json and
  completion paragraphs. Texture forwarded "the missing Podcasts scope", but
  engineering said at 10:24 that it never arrived. Podcasts was not built,
  and package.json was edited, a rule that was in the cut-off part. The v6
  request kept the whole brief to 3,342 B, and its record reads
  `truncated:false`.
- **A "partial" Complete after Freeze is refused.** Engineering froze
  (selfdev-f99b7ccd) and then sent `Complete("partial")`. The completion
  guard `engineering_overlay_terminal_fate_required` refused it twice, and
  the run was cancelled at 10:34:01. Because nothing reached Complete, no
  candidate was persisted, and the next round on that trajectory got no
  carried patch: 45 minutes of four-app work was lost from the product
  path. The honest result for "frozen, but one of five parts is not done"
  is refused, with no visible reason given to the model beyond the guard
  name.
