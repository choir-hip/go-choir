# A Jev outage leaves work that never catches up

Date: 2026-11-11. Class: green (record only; nothing changed). Related:
[paper-jev-credits-and-search-quotas-exhausted-2026-11-11](paper-jev-credits-and-search-quotas-exhausted-2026-11-11.md),
[SYSTEM1 reference](../reference/SYSTEM1.md) section 3.4.

## Problem

OpenRouter credits ran out at 16:05:21Z and every Jev call failed with a
402 until the owner topped up; the first success was 22:38:15Z. The paper
kept publishing (47 to 59 stories an hour), but several System 1 call sites
dropped their work with no path back once Jev returned. Nothing alerted
anyone during the six and a half hours.

## Evidence

Read-only trace from the Node B gateway log, the host front-importance
journal and the public paper API. The gateway logs the guest door as
`site=unknown`, so guest calls are assigned to sites by request shape
(inference, not log fact). The paper VM's own ledger was not reachable.

- 5,120 failed guest calls (5,115 the paper, 5 d@e) and 3,294 failed host
  calls; every failure in the window was a 402. Guest volume stayed about
  790 an hour against about 800 before: no retry storm.
- **Support judge** (`internal/agentcore/support_judge.go:369`): the public
  API showed 2,968 pending citation pairs on front stories at 22:42Z, 0
  before 16:00Z, unchanged at 22:44Z. Pending pairs are judged only when the
  same story is committed again citing them (`support_judge.go:132-155`); no
  sweep, no timer. Readers see "✓ exact" from the quote check; only the
  tooltip says "support: pending".
- **Intake labels** (`intake_labels.go:779`): about 3,020 failed calls
  covering about 5,100 captures, left unlabelled; "nothing retries a
  capture" (`intake_labels.go:269-275`).
- **Event layer** (`event_layer.go:460`): about 870 failed calls; captures
  not joined to events, released to management "never put to Jev again"
  (`event_wakes.go:401-416`).
- **Front importance** (`internal/frontimportance/scorer.go:686`, a
  10-minute timer over the newest 100 stories): by the 18:01Z run none of
  the 100 had a score, and the front page fell back to recency order once
  under half was scored. The 22:39Z run scored the newest 100. About 137
  stories published 16:00Z to 19:00Z had left the window and still carry no
  importance and no section, so they are missing from section pages.
- **System 1 verb**: 23 interactive desk questions failed (22 the paper).
  Sub-call and desk-cast judges closed their work as failed (count not
  visible).
- No traffic seen for the prompt router, classifier or disclosure gate; the
  op-ed gate did not fire.

## Belief

The fallbacks are safe in the moment (nothing false is published as
checked) but each one is a drop, not a defer: the work is not queued for
when System 1 returns. The weakest link is the support judge, the only
check that a cited passage supports the sentence. The second gap is that a
spent key is visible only in the gateway log.

## Remaining error

- The effect of unlabelled captures and unjoined events on story quality is
  unmeasured.
- The failed sub-call and desk-cast work items are not counted.
- The fix is not chosen: a catch-up sweep per site (pending support pairs,
  unscored stories past the window, unlabelled captures), an alert on the
  first 402, or a second System 1 vendor. The free
  `parallel/decisions-fast:free` is not a fit as an automatic fallback: 20
  requests a minute and at most 1,000 a day, free only until about
  2026-11-13, and it sends its state to live web research, so drafts would
  leave the system.
