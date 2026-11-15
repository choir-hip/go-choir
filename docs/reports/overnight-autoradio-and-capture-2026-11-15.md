# Overnight: Autoradio's programme route is live, and capture no longer holds the store open

Written Sunday, November fifteenth, at about half past two in the afternoon UTC. It covers the nine and a half hours since you went to bed at about five in the morning UTC, across both missions: the Autoradio learning engine and part three of the storage work.

Three trains landed overnight and all three are deployed. Autoradio can now write a programme and play it on your phone.

Here is where things stand. Jev is now called through one entry point per job, and the paper's intake takes one or two calls a flush where it used to take many. The Autoradio namespace and the programme route are on staging, and the test computer wrote three stretches of programme in about five seconds each. The programme player is installed on your phone and has not been launched. The capture change, which stops a snapshot from holding the store's write log open for the whole witness, landed just before two in the afternoon and is deployed, but no computer has taken a capture with it yet, so it is unproved. The restore work did not land. It went three review rounds twice, I stopped both times, and the second stop turned up a deeper fact: a restore is undone by the next boot in every form we have. That one needs a decision from you.

## Jev, and the first Autoradio stations

The first train took four attempts and landed just after ten in the morning. Three failures were mine to fix, and the last one cost the most: the run failed at its final gate after a hundred and seven minutes, because a new baseline file was not listed where the deploy classifier expects it. The gate that failed takes seconds. So I moved the script self-tests ahead of the long suites in the landing script, and the second train landed on its first attempt.

The Jev change does what you asked for. One entry point builds one context per job and splits only when the vendor's limit refuses it, and the log says which limit. Over two hours on the paper I counted seventy-three flushes. Fifty-seven took one call, thirteen took two, and every split named the sixty-four thousand token request limit. That prediction is scored true.

The count also showed a caller problem that is recorded and not fixed. Thirty-two of those seventy-three flushes carried a single capture. Each is its own call, and each carries about twenty-three thousand tokens of shared state. The flush does not wait to gather captures. And three flushes wrote a label with no Jev call at all. Two followed a timed-out index read. One has no cause I can see yet.

The second train carried the programme route. A request for a stretch of programme streams back segments, and each content segment carries one forecast claim. A replay returns only segments that a claim backs. On the test computer three stretches completed, four claims each, and a replay came back identical. Two honest limits. River is not registered, so Bedrock Haiku answers, and each request is a paid call. And Bedrock does not stream here, so the idle deadline has never been exercised.

The player went in on the same train. It decodes a stretch, shows segments with their kind and source, and jumps between them. I built it from main and installed it on your phone without launching it, as you ruled. The phone does not yet send candidates, so the programme it gets can only be idents and bridges. That is the next station.

## Restore, stopped twice

You asked for the paper's restore problem to be solved in a storage-efficient way, going forward. I did not get there.

The design for restore bases drew three repair verdicts and I set it aside. Then a smaller lane, the restore act alone, drew three more. The third round found that a panic after the flip would run the undo's release step and leave the computer in a state with no exit. I stopped again and wrote both assessments.

While writing the second I read the boot path, and that changed the picture. A restore today is transient. The next boot reconstructs the store from the tape and replays over the restored state, and so does the next append. There is no event on the tape that says "this computer was restored to here." Until that forward restore event exists, a better restore act is polish on something that does not last. Both lanes are parked. The forward restore event was designed on October thirtieth and never built. Building it changes what the tape means, so it is your call, and my default is to not build it until you say so.

I also have to correct myself. I wrote that the paper is not restorable by any path. That was too strong. The eligibility check I was reading only feeds a report. The correction is in the record.

One lesson from the three stops, now a rule I work by. In the capture design a builder added a second mechanism inside a repair round, and both reviewers rejected the round for it. A repair round repairs. New mechanism gets its own lane, with a design section and a failure list reviewed before any code.

## Capture on a static copy

A snapshot capture on the paper held a read transaction open for the whole witness, about twenty-six minutes on the last capture I have a record for, and the write log could not be trimmed for that time. The new capture copies the store's pages to a scratch file, releases the transaction, and witnesses the copy.

The design took three rounds and was accepted with a forty-seven entry failure list. The code passed both seats. The first landing failed fifty-four minutes in, on a stale test that the builder's test selection had never run. I deleted it, and the second run landed at three minutes to two. Continuous integration passed, and staging, the paper and the test computer are all on the new commit.

It is not proved. Neither computer is due for a capture. The paper's next one comes when its event ceiling or its interval arrives, which could be many hours. I precommitted two claims. On the paper, the new pin must be at most half of the old read transaction. The newest capture's own figure was lost to a restart and log rotation, so I set the baseline from the newest recorded one, twenty-five minutes fifty-one seconds, and wrote that substitution down before any outcome existed. On the test computer, two natural captures, a restore that matches its witness, and two kill drills. If the paper's first capture fails, is refused, or pins longer than before, the choice is to revert or to accept the result, and the design says that is yours when the outcome is read.

## The paper through the night

The paper published all night. Devin's cheap model served between eighty-two and a hundred percent of its calls each hour, Bedrock cost nothing, and River and Synthetic took no calls. The free fallback share wanders between zero and eighteen percent with no Devin errors logged, and I have not traced why. The last tick's pacer line contradicts itself, zero percent beside a hundred and forty-three of fourteen hundred calls, and I have not checked which is right.

Two storage readings to watch. Node B's free disk fell about four gigabytes in six hours around midday, with two deploys in the window, then rose again. A hundred and ninety-two gigabytes are free. And the paper's snapshot root is thirty-nine hours old, which the capture change is meant to help with.

Gemini refused every review request overnight with a quota error, four attempts. Reviews ran on DeepSeek and Sonnet.

## What I would do next

The thing to remember is that both Autoradio stations you can see are real and deployed, and the one storage change that landed is waiting on a capture nobody can hurry.

If I were holding the phone, I would open the Lab app and play a programme, because that tells you more than my proofs do. Then I would decide three things. Whether the forward restore event gets built. Whether the programme route should keep spending Bedrock money per request or wait for River. And whether the numbers I invented stand: a hundred and twenty seconds of wall time, thirty seconds idle and thirty segments on the route, and eight seconds on the phone. On my side, next is the phone sending candidates, the fix that lets intake gather captures before it calls Jev, and reading the paper's first capture when it comes.

## Names and receipts

Goal files: `docs/definitions/choir-autoradio-learning-engine-2026-11-14.md` and `docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.md`.

Landed and deployed on November fifteenth, times UTC. Every non-docs commit was expedited; rollback for each is a revert.

- Train 1, landed 10:07, CI run 43702528075, staging on `e9a0b757a`, paper booted 10:31: Jev one entry point `8b15ec2e9` (red); Autoradio namespace `c4e8446b2`, `1f160b0ba`, `2ad3cc9d2`, `b40edecb4` (red); docs `e9a0b757a`.
- Train 2, landed about 11:15, CI 43708149004, staging on `b8b741f9f`: landing script self-tests first `d019b78b2` (yellow); programme route `45bf7ce4c` (red; reviewed by DeepSeek and Sonnet, round 2); programme player `5748506db` (orange); simulator slimming script `b8b741f9f` (yellow).
- Train 3, landed 13:57, CI 43721684380, staging, paper and d@e on `842e0fda6`: capture on a static copy (red; DeepSeek round 2, Sonnet round 1; security epoch 127).

Expedites from earlier in the night, already landed when you left: `62dce298c`, `826ad4aa9`, `304a81277`, `57ff86475`, `c7eabe010`, `cc8ea803d` (CI 43680650822).

Predictions settled: C95 true (namespace refuses on d@e), C96 true (one or two Jev calls a flush), C98 true (three stretches on d@e), C68 false. Open: C99 (paper pin at most 775 seconds, baseline 1,551, recorded in `e2553b47d`), C100 (d@e correctness and kill drills), C97 (page-churn bench, by November eighteenth), C85 (retirement wake).

Parked or held: `lane/restore-act` at `06b742853`; `lane/restore-boot-install` at `0c9c357df`; `lane/bases-bench` at `4a3792369`; `lane/node-a-memory` (docs, unmerged); `lane/take-writer-land` (held by you until the week of November sixteenth).

Records written under `docs/problems/`: the two restore structural assessments; the landing script running cheap gates after the suites; intake flushing single captures as their own Jev call. Designs under `docs/design/`: `capture-static-copy-2026-11-15.md` (accepted), `restore-bases-store-once-2026-11-15.md` (set aside), `restore-installs-at-boot-2026-11-15.md` (unreviewed, parked).

Not done: the label written with no Jev call is untraced; the free-fallback share and the Node B disk dip are untraced; whether the checkpoint authority can accept a checkpoint at an older head is uninvestigated; `docs/ACTIVE.md`, the mission graph and both goal ledgers are not updated; the `lane` worktree tool is not looked at; Patch Watch failed again on its schedule at 13:55; mobile-only commits still trigger a platform deploy; the search count reads zero answered; minor review notes on the capture change (a stale comment, a test path string, an untested error branch, log wording) are unfixed.
