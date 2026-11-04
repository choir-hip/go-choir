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

## Follow-up: question 1 failed a modal over its scrim (2026-11-04)

Pool Library r5 (op selfdev-41747edf…, shot 05): the judge failed the round
because "the page shows through". The larger-view panel was opaque and crisp;
the page text ("Precommitments", "1 recorded") sat outside the panel's edge,
dimmed under the modal's backdrop. The wording "one opaque app window in
focus, nothing behind showing through" reads a normal scrim as a defect.
Question 1 now asks that the focused surface (the window, or a modal or
dialog over it) be opaque: content dimmed behind a backdrop outside its
edges passes, and only text or imagery showing through inside its bounds
fails.
Questions 3 and 6 do not judge page content dimmed behind a backdrop, and
still judge everything else visible (review: a scrim-less dialog must not
hide the page from them); one luna run had failed the scrim modal on the
dimmed page's rows.

Check (OpenRouter, synthetic 1280x800 Library shots): an opaque modal over
a scrim with page text dimmed outside it, and the same modal at 60%
opacity. With the new wording, deepseek-v4.1-flash-1017 and gpt-6-luna
each ran three times: the scrim modal passed every question 6/6 and the
fading modal failed question 1 6/6. The four calendar shots above still
failed on the same planted defects.

## Follow-up: "Call with European office" was management's misquote, not the vision model's (2026-11-04)

Luna Calendar r4 (computer-2bf3cab1e3ebf3eb24a515c4603cf954, op
selfdev-e83ce99d9cb68bf85b08c309169d769c, release 1104c9890, before the
per-shot review): management's Verify failed the round saying an image
reading put the Tokyo label on "Call with European office". Shot 03 has no
such event.

Trace (management run 5086cb92-08ce-449b-8f22-61f993785f64, read-only
`GET /api/runs/{id}/events`):

- seq 29, 08:12:19Z: management's one vision call on shot 03
  (artifact:sha256:145809b8...), answered by deepseek/deepseek-v4.1-flash-1017
  (677 input tokens). The prompt itself was wrong: it asked for "a Tokyo zone
  label on the Rainfall review event". The answer was correct: "the
  “09:00 GMT+9” label is on the **Call with Tokyo office** event instead."
- seq 56, 08:13:05Z: management's Verify reason restates that answer as
  putting the label "on the \"Call with European office\" event rather than
  the Tokyo one". Management's own model is the same
  deepseek-v4.1-flash-1017 (run metadata), so the invention happened when the
  desk model restated a text answer, not in the vision reading.

Measured (OpenRouter, the real shots luna 03 and pool 05 from lane C):

- Transcribing every title and label, 3 runs per model per shot: deepseek-v4.1-flash-1017,
  stealth/crouton-alpha, gpt-6-luna and deepseek-v4-flash-vision-exp
  transcribed "Call with Tokyo office / 09:00 GMT+9", "Standup / 09:30 EST",
  "Rainfall review / 3:00 PM – 4:30 PM", the date and the modal's
  caption correctly in 24 of 24 runs. Nothing was invented. The only miss
  was the dimmed "Precommitments" under the pool 05 scrim, in 5 runs.
- A leading prompt naming a "Call with European office" event, and a neutral
  one, 3 runs each on deepseek, crouton and luna: all 18 answers denied
  the European event or quoted the Tokyo block correctly.

Decision: no model-routing change for the judge. No vision model
hallucinated on these shots, so pinning a verifier or requiring two
agreeing readings would not have prevented this failure, which happened in the
desk's restatement. The per-shot review already records the vision model's
answers verbatim, in the event's `shot_review` and in the fail reason, so
a fail it decides cannot be misquoted. The management overlay now tells management to
quote a vision answer's text_excerpt verbatim, never in its own words, and
to ask about what the request names. Residual: a fail that management decides
on its own still rests on management's prose. The per-shot review runs
only on a pass, by design (the Verify admission work is elsewhere).

## Follow-up: a candidate missing half its requested shots passed (2026-11-04)

Crouton Library r5 (op selfdev-8d4532481b4aab23820dad0aeb264035, release
1104c9890, before the per-shot review): management's Verify passed a
candidate that froze 6 of the 12 shots the request named. Shots 05 to 09
and 12 were missing, and shot 11 ("11 390 aero-chrome-heart the list on the
phone"), named `11-aero-chrome-heart.aero-chrome-heart.png`, was 1280x800.
The per-shot review judges only the shots present, so it would have passed
the six that exist.

Fix (FM9 in selfdev_shot_review.go): the review reads the owner's words the
round answers, from the tape through the assignment's Cast chain, the same
read that carries them to the engineer (`ownerWordsForAssignment`). It
parses their numbered shot list (the longest run 01, 02, ... in text
order; each item's text runs to the next number, ';' or line end).
- A shot the list names that the bundle lacks (matched by the screen name's
  leading two digits) fails the candidate: "missing shots the request names:
  05, 06, 07, 08, 09, 12", with no model call.
- A shot whose request item or own name says phone (or 390, or mobile) fails
  question 5 unless it is portrait and a whole multiple of 390 wide.
- Each image in the batch prompt carries the request's description of its
  number, so question 2 judges against the request and not only the file
  name.

A request with no numbered list (fewer than three in a run) or one not
reachable from the assignment checks nothing new. On all 44 Calendar,
Library and Slides requests and rounds in lane C the parser finds exactly
01 to 12 with 11 and 12 as phone shots; on the Files requests, which carry
no numbered list, it finds none. Residual: a later round whose newest owner
revision restates only the fixes, not the list, is not checked for missing
shots.
