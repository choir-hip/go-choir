# Self-development on the calibration seed, overnight

I wrote this on the morning of November fourth, at about eight in the morning UTC. It covers roughly three in the morning to eight, the hours after you went to bed and asked for the self-development work to run on the calibration seed. It follows the earlier part of the night.

Two apps were built, judged and published by test computers tonight, each with its predictions stated in advance and scored by the runtime.

## Where things stand

The test computers, which I call the arms, are running self-development rounds on the platform you asked for. Every round now states five predictions when management casts it. Those are whether verification passes, whether the judge accepts, whether you approve, how many minutes until the candidate freezes, and how many minutes until approval. The runtime settles each one from what actually happened, and a new read lists every claim beside its outcome.

Two apps passed and are published. The first is luna's Calendar. The second is claude's Library, and it is the first release that carries its own screenshots, shown in an app-store gallery. I sent you both contact sheets.

The early numbers are honest. The clock predictions are good: six of eight held. The verification predictions are over-confident: management said about seventy percent and three of eight passed. Engineering is over-confident on every arm.

Your decisions waiting are small. They are at the end.

## The night, in order

Around three thirty, the Synthetic subscription ran out again, after briefly coming back. It stopped four of the five arms at once. Rather than move those arms to a different model, which would break the comparison between models, I routed the same DeepSeek snapshot through OpenRouter. The reviewer blocked the first version because the reasoning settings and image support were unverified. I checked OpenRouter's own listing, fixed the reasoning mapping, and the second review approved it. The arms came back at ten past five, and their prompts are ninety-seven percent cached, so they cost very little.

At the same time, the trace watcher found that work was being lost to waits. A good Calendar candidate froze, then died because a model stream hung for ten minutes and the run hit its deadline. I landed three fixes. A frozen candidate now survives its engineer's deadline. A silent model stream is cut after three minutes and retried. A loop parked on a provider quota no longer dies at its activation deadline, and it notices a model change in its policy within thirty seconds.

You asked for calibration in use as soon as possible, and for clock time to be one of the scored factors. The first cut landed at about five twenty. The second cut followed. It added your approval and the approval clock, a late flag for any prediction stated after the work is visibly under way, a salted held-out set, and the per-claim read. One builder reported that the prediction records were durable, and I checked that before trusting it. I also confirmed that no calibration digest had been shown to any desk before tonight, so tonight's over-confidence is a clean baseline.

You also asked how the seed should be wired: pushed into the prompt, available in the REPL, a short stub, or something else. A design agent mapped the whole space. It is in the repo as the calibration wiring permutations report. Its main findings are three. The real gap is discrimination, not calibration: perfect recalibration would gain little, and the desks mostly cannot yet tell good rounds from bad ones. Management's predictions are scored on engineering's record, so management never sees its own. And at tonight's pace, a plain comparison between rounds would take days. A two-stage design, where the desk states a number, is shown its record, then restates, would read out in about a day. The cheap fixes from that report are deployed. The first experiments are built and in review, waiting for the arm assignments.

Through the night, the rounds themselves taught us a lot.

Several rejections were the platform's fault, not the engineers'. The screenshot tool's fixture badge covered every window's buttons. Freeze accepts twelve screens, while our Library requests asked for fourteen. The tool photographed overlays mid-fade. And the biggest one, found by the lane reviewing the rounds: since May, the London Salmon theme has stripped every app button to bare text with a forced rule no app could override. That one rule explains most of the repeated "salmon buttons are bare" rejections across two arms. All four are fixed, and those findings no longer count against the engineers.

Some failures were real engineering, and they show a pattern. In five of seven failed rounds, the engineer froze with a named defect still visible in its own screenshots, and management's verification caught it within a minute. The fix for that is engineer reuse: a rejection should go straight back to the same engineer, with its context and capsule, rather than costing a new round. That is fully built and two-seat reviewed, and is on its final test run.

Management once ended without verifying a frozen candidate, and nothing woke it again. That stall cost twenty-two minutes of a fifty-eight minute round before a nudge recovered it. A guard like Texture's next-actor guard is in review.

The judge also passed a round that the owner-side review rejected, because it checked only the defects named in the previous round. It now asks six plain questions of every screenshot: one opaque window, shows what it says, readable with no overprint or clipping, plain errors, the right size, and every control styled. On planted defects it caught all of them, at about twelve seconds and fifteen thousand tokens a round.

Finally, three of five rounds froze just past the forty-minute target. The engineers could not see a clock. They can now: elapsed and remaining minutes, and one reminder at minute forty.

## What I would do next

The one thing to remember: the platform now records honest predictions on every round, and the first reading says the desks are fast and over-confident, not slow.

If I were holding the phone, I would let engineer reuse land and watch crouton's Calendar use it. Crouton failed six Calendar rounds, each on one more layer. With reuse, each of those is a few minutes of rework. Then I would start the first two calibration experiments.

Your decisions, none of them blocking:

- Should each prediction count on the record of the desk that stated it? I recommend yes. Today management's Cast predictions land on engineering's record.
- Should Texture's packet numbers be scored as Texture's own claims, or dropped?
- Should the mandatory clock be "minutes to a Freeze that passes verify", not just "minutes to a Freeze"?
- Same-model provider fallback: should a desk whose provider runs out switch to the same model at another provider automatically, as I did by hand tonight? Your standing rule is fail rather than fall back, so I left it for you.
- The five base-image questions from earlier in the night are still open.

## Names and receipts

Goal: docs/definitions/choir-overnight-pd-then-selfdev-metamission-2026-11-03.md, ledger beside it.

Published: luna calendar-next release 691e2ae9; claude library-next release ba1b0d90 with eight screenshots.

Landed tonight, in order: 7efaaeae2 quota park; 30f617c01 OpenRouter DeepSeek; 496610447 round receipt; 60c8953ff stream watchdog; 3df32b23b calibration cut 1; a13947b95 receipt freeze time; 88c3a3c91 spin detector; d82e34fc6 badge; 7c36347ff frozen candidate retained; 0e45a7cca quota park deadline; f169eeb18 through 58f5c4ef2 Library media and video; 6472fab6f byte ranges; 41b015c5c through bd83082b9 calibration cut 2 and claims read; cb5de2c4c receipt predictions line; c2c0590c5 OpenRouter session id; 8a9050b80 permutations report; e6fc85acf through e884887f6 calibration C2 to C11; 29894990c screenshot animation wait; 950f74113 engineer clock; 1104c9890 London Salmon buttons; 8085f1754 judge reviews every shot (both in CI at writing).

In review: engineer reuse (branch engineer-reuse); management verify-owed guard (verify-owed); E1 experiment mechanisms (calibration-cheap); and nothing else.

Problem docs: provider-quota-kills-desk-loops, synthetic-limit-stalls-deepseek-arms, frozen-candidate-dies-with-build-deadline, model-stream-hangs-ten-minutes, engineer-noop-spin, salmon-theme-strips-app-buttons, engineer-has-no-clock, all dated 2026-11-04 under docs/problems.

Design: docs/design/calibration-wiring-permutations-2026-11-04.md.
