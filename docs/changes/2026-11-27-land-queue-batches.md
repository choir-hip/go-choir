---
date: 2026-11-27
title: the landing queue lands its lanes as one batch
class: yellow
---
What: `scripts/land-queue a b c` calls `scripts/land` once with every lane: one verify, one push. A lane that conflicts is set aside and the rest retried; a failed check halves the lanes, in order, to find the lane that broke it, within a stated cap on attempts. The queue does not wait between pushes, refuses to push on top of a main whose newest code push has a red CI run, and at the end must find a successful CI run that covers the newest code it pushed (a cancelled run counts as superseded; nothing covering it is "CI unknown" and a non-zero exit). With no `--tree` it works in a lane of its own, the only tree it may reset, and removes it at exit; a tree it is given is never reset. `--serial` keeps one lane at a time with a wait for staging, for a lane that needs its own deployed proof; `--wait-staging` waits for staging after the last push. `scripts/land` reports its result through a file (`refused`, `conflict`, `failed step=`, `pushing`, `landed`).

Why: each queued lane waited for the one before it to pass CI and reach staging, 12.9 minutes at the median, although the deploy needs no such wait. Lanes approved together should land together.

Proof: `/bin/bash scripts/land-queue.test.sh` -> 225 passed, 0 failed; `/bin/bash scripts/land.test.sh` -> 58 passed, 0 failed (the builder's runs; the second reviewer saw 116 and 58 pass on an earlier candidate). Four review rounds: seat 1 (seven findings), a Sonnet reviewer (three blocking), seat 1 (one blocking), seat 1 on tree ownership alone (the reset and clean paths confirmed verified; two hardenings). All fixed; the last two hunks were read by the lead and not by a seat. Not proved: any run against the real `gh`, the real build machine or a real batch; ledger claim C257 is the first real batch.

Replaced: one landing per lane with a wait between.

Undo: revert this commit, or use `--serial`.
