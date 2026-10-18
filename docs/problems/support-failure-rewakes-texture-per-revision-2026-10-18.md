# A support failure re-wakes Texture once per revision, so an unsupportable story takes 7 to 9 turns (2026-10-18)

Status: documented, not fixed. This is a cost problem, not a correctness
one: the gate holds every such story, and the cycle ends on its own.

Found at the 15:30 hourly check on 2026-10-18. The ledger confirms it:
inbox records, run metadata and console lines, all listed below.

## Evidence (ledger)

Story `861f0ff9` cites only an HN capture of theatlantic.com
("Theranos-Style Fraud Has Become the Norm"), whose body is links only.
Research reported that the Atlantic page returns 403 to fetch. Its
Texture inbox (`/api/inboxes/texture:861f0ff9-…/records`) holds 14
records: 1 management cast, 4 research reports, and **9 from
`system:support`**, committed at 15:23:55, 15:25:33, 15:26:04, 15:26:29,
15:26:56, 15:27:29, 15:28:01, 15:28:31 and 15:29:16. Each record's id
names the revision it judged (`support-judge:<revision>:…`), and each of
those revisions is the previous Texture turn's commit. Every Texture run
on the story has `request_intent=integrate_execution_findings` and
`request_source=inbox_record`. Research ran only twice for it (15:23:55,
15:24:54).

The same pattern, counted from the inboxes:

| story | system:support records | span |
|---|---|---|
| 861f0ff9 (theatlantic.com, link-only) | 9 | 15:23–15:29 Oct 18 |
| 854a11be (wordpress.com, one outlet) | 7 | 15:25–15:29 Oct 18 |
| 4f1a228b (github.com, one outlet) | 7 | 18:07–18:11 Oct 17 |
| 5af5ef1e (scottstevenson.net, one outlet) | 7 | 19:21–19:23 Oct 17 |
| 3ee21d44 (thing.net) | 1 | 15:25 Oct 18 |

**Correction.** The handoff rows for the 18:40 and 19:40 checks on Oct 17
blamed the 4f1a228b and 5af5ef1e bursts on research verdicts arriving one
at a time. The inboxes show they were support-judge records. The
"story verdicts: published N verdicts" console line after each turn is
these support verdicts reaching the published overlay, not research.

## Mechanism (code)

- `internal/agentcore/support_judge.go:40`: a Texture revision commit
  hands its new citation pairs to Jev. A failure is "one record per
  document to texture:<doc>, which wakes the Texture to correct it with
  Write."
- `internal/textureowner/texture_controller.go:602`: `InboxUnread` makes
  `recordsPending`. At `:655` the update breaker refuses dispatch only
  when `!recordsPending`, so record-driven wakes bypass the breaker.
- So, when a story's sources cannot support a sentence (a link-only
  capture, or a 403 page), each correcting revision writes a new
  sentence. That is a new pair, judged again, failing again, and waking
  Texture again. The cycle stops after 7 to 9 turns, presumably when
  Texture removes the claim or reuses already-judged pairs. I have not
  confirmed what ends it.

## Classification

This is a substrate cost in the wake rule: a system judgement record has
no per-document budget. The design doc
(`docs/reports/support-judgement-to-jev-design-2026-10-14.md`) lists 12
failure modes. None is "the correction cannot succeed, so the loop of
corrections repeats." Its falsifier ("watch the failure-to-correction
ratio") is exactly what this measures: 7 to 9 failures, no successful
correction.

## Cost

About 7 to 9 Texture turns per unsupportable story, on gpt-6-luna,
inside 3 to 6 minutes. On the evening of Oct 17 and the afternoon of
Oct 18, the hourly checks show such a burst roughly once or twice an
hour, which is perhaps 10 to 15% of Texture runs. That is an estimate,
not measured.

**First measurement (15:30 to 16:17 Oct 18).** 45 Texture runs on 21
stories. Their inboxes hold 11 `system:support` records committed in the
window, on 7 stories: d24f1104 got 3 records over 6 runs, e79b0b46 2 over
4, 1f4b845c 2 over 3, and four others 1 each. So about 24% of Texture
runs that hour were support-failure wakes, if each record caused one
turn (records can coalesce into one turn, so this is an upper bound). No
story went past 3 this hour. The long 7 to 9 cycles need a story whose
only source can't support it.

**Hourly series (same method, upper bounds).** 16:17–17:17: 3 records over
33 Texture runs (9%). 17:17–18:17: 7 over 34 (21%). 18:17–19:17: 21 over
56 (38%), on 11 stories, none past 3. The share swings with how many fresh
stories open on thin sources, and in the busiest hour it was the
largest single cause of Texture turns.

## Fix directions (not chosen; red surface, wake rule)

1. **A per-document support budget.** After N consecutive support-failure
   wakes with no passing verdict (N = 2 or 3), the next failure record
   still commits but does not wake. Texture sees it at its next real wake,
   such as new research, a cast, or a capture. This is the smallest change.
2. **Batch the support judgement with the publish hold.** When the
   publish gate already holds the revision for outlet count, a support
   failure adds nothing Texture can act on without new sources. So it
   should not wake until new evidence arrives.
3. **Tell Texture the cause.** If the failing pair cites a capture with
   no outlet words (the `captureHoldsOnlyLinks` test), say so in the
   record. Then Texture removes the claim in one turn, rather than
   rephrasing it seven times.

Ways a fix could fail, to test first:
- a real correctable failure (a misquote on a full capture) no longer wakes
  Texture;
- the budget never resets, so a later real failure is silenced;
- a pass starts waking Texture (design failure mode 6);
- a crash restart resumes a parked support wake (owner restart rule).

Measure after any fix: the count of `system:support` records per story,
and the Texture turns per held story, against the five stories above.
