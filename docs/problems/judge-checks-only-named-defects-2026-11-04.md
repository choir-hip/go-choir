# The judge checks only the named defects, not every shot (2026-11-04)

Status: open. Mutation class of this doc: green. The fix is red (run
acceptance: it changes what passes).

## Problem

Management's Verify is the independent judgment on a frozen self-development
candidate (`commitVerifyIntent` → `recordSelfDevelopmentVerdict`,
`types/commitment_rules.go` R17). Its screen check is one optional vision
call through `POST /api/model-policy/verify` with one `image_ref`, and the
management overlay (`rlm_management_runtime.yaml`, "Screens") allows "at
most one vision call per candidate", asked "what the change should show;
does this screenshot show it?". So the judge looks at one shot, against the
engineer's claims and the previous round's named defects. Nothing reads
every shot, and nothing asks whether a shot is readable, opaque, the right
size, or styled.

The owner-side gate (lane C) asks six questions of every shot:

1. One opaque app window is in focus, nothing behind showing through.
2. The shot shows what its description says.
3. Everything is readable: no overprint, overlap, clipping, mid-word breaks,
   or literal escapes or placeholders.
4. Errors are stated plainly, if any.
5. The shot is the right size (desktop 1100x750 or 1280x800, phone 390x844
   at any DPR).
6. Every control is styled as a control in the active preset, including
   secondary ones.

A "no" on 1, 2, 3, 5 or 6 rejects.

## Evidence

2026-11-04, deepseek test arm computer-e80d229b0d78c35d47656b8c66426a43,
operation selfdev-4aef87444324b094102a58cf43e2733f:

- Management's Verify passed the candidate at 07:19:54.
- The owner-side gate rejected it 37 s later, on defects visible in the
  shots' own pixels:
  - agenda shots 04 and 11 print the time over the title ("2:45 PStandup");
  - month shots 01 and 10 run time and title together ("2:30 PMStandup"),
    and two events overprint in one line;
  - week shot 02 breaks "Standup" mid-word, and a list is cut off mid-line;
  - import picker shot 10 shows a literal escape (`\u2026` as text) where an ellipsis belongs;
  - under london-salmon (02, 09) the "Today" and "Import" buttons are bare
    text.
- Other rounds the same night show the pattern: shots captured with overlays
  half-transparent passed, and buttons were judged "fixed" loosely.

## Cause

The judge's screen check is scoped by its prompt to the claim delta (one
image, one question about the change). Its instruction and its tool both
bound it to one image, so even a careful model cannot cover a 12-shot
bundle, and no per-shot record exists to fail on.

## Fix direction

Extend the existing judge, not a parallel one:

- the verify route (`/api/model-policy/verify`) takes `image_refs`, a batch
  of shots in one call (the gateway admits 20 images per request);
- a management Verify "pass" on a candidate that carries screens runs the six
  questions over every shot of the bound bundle, in batches, through the
  same `verifier_multimodal` role; question 5 is decided from the bundle's
  recorded size, a blank shot fails 1 and 2 without a model call;
- the per-shot answers join the verification event, and any "no" on
  1/2/3/5/6 turns the verdict into a fail naming the shots and reasons;
- the management overlay states the six questions.

Rollback: revert the commit.

## Failure modes (written before the code)

Listed in `internal/agentcore/selfdev_shot_review.go` (FM1-FM8): a failed
call or an answer that is not the JSON asked for, or misses a shot, leaves
the candidate frozen with the error stamped, never a pass; a replayed pass
the review failed answers from the operation with no second call; shots are
answered by their "Image N:" label; size, blank and unresolvable shots need
no model; anything but "yes" on 1, 2, 3 or 6 is a "no", and question 4
never rejects; a fail verdict, or a candidate with no screens, runs no
review. `verify_pass` keeps reading management's own decision and
`judge_accepts` the operation's state, so the two part when the review fails
a candidate management passed (R17).

## Measurement (2026-11-04, OpenRouter, four synthetic 1280x800 shots)

Four calendar shots, three with a planted defect (time run into the title,
bare-text "Today"/"Import" under a salmon header, a literal `\u2026` escape; the fourth shot is also named an import picker but shows the agenda, which both models failed on question 2).

| | deepseek-v4.1-flash-1017 | gpt-6-luna |
|---|---|---|
| old: one shot, "does it show the change?" | 2.1 s, 711 in / 638 out; said yes (noted a collision as a caveat) | 3.5 s, 1248 in / 87 out; said yes ("cramped") |
| new: one batch of 4, six questions | 5.2 s, 3066 in / 1304 out; caught all three | 8.1-11.0 s, 5271 in / 704-899 out; caught all three |
| new: 12 shots, 3 parallel batches | 11.3 s wall, 9198 in / 5340 out | 13.3 s wall, 15813 in / 3033 out |

So a full 12-shot review costs about 10-16k input and 3-5k output tokens and
11-13 s at the Verify, against about 1k and 3 s for the one-shot check it
extends (that check stays optional for management). Both pool models failed
every planted defect the old prompt passed. The script and shots are not
kept; the numbers are from one run each.
