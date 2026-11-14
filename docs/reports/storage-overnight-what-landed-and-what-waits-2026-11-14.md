# Storage overnight: what landed, what is held, what needs you

Written Saturday, November fourteenth, at about six in the evening UTC. It covers the storage work you left running last night, which is part three of the storage solutions mission, called store once. The detail I can vouch for runs from about six in the morning UTC to now, roughly twelve hours. The landings from earlier in the night are listed at the end, not retold.

Three storage changes are deployed and running on the paper, a fourth is landing as I write, and two more are finished and waiting for a decision that is yours.

Here is where things stand. The paper now carries counters that say what its tape is made of. Old receipts are being rewritten into a smaller format, about three million of five million done, and the paper has not slowed. A lifecycle command can now be read when it names its revision, which is the safe half of removing a duplicate copy. The change that finally lets old run memory be deleted has been accepted by both reviewers and is in its landing run. Six flaky tests that were costing us landings are fixed. Two things wait for you. The second half of the duplicate-copy change cannot be proved on the test computer before the paper takes it, and I will not land it without that. The change that prunes old releases at boot is accepted by both reviewers and held because your ruling said so.

## What the paper told us

The first thing that landed on Saturday morning was a set of counters. Until then we were guessing where the bytes went.

The paper writes about a hundred and fifty megabytes of tape an hour. A little over half of the message bytes are run memory that is carried forward from one run to the next. The record of a run is rewritten almost five times per run, at nearly nineteen kilobytes each time, and that alone is more than a fifth of object bytes. Those two numbers decide what is worth building next. Carrying memory by reference and rewriting the run record less often are the big levers, and neither is built yet.

A second counter was a census of run memory. It found five million entries across a hundred and fifty thousand runs, and not one had ever been retired. The job that was supposed to delete old memory had been deleting nothing. There were two causes. It looked up each run's record under the wrong kind of id, so for forty-five thousand runs it found no record and kept everything. And its rule for protecting recent conversations protected nearly all the rest.

## The build machine ran out of memory

At about a quarter to seven in the morning, Node A killed the virtual machine that the builders compile on. Node A has thirty-two gigabytes and no swap, and its virtual machines are configured for about forty. A landing run and a builder's test run overlapped and the kernel chose a victim.

I moved everything to the one remaining build machine and made the rule one heavy test suite at a time. That is safe, and it is slow. A full landing takes between half an hour and an hour and a half, and everything queues behind it. This is the main reason the day produced five landings and no more. Node A's memory is not repaired. The two users Mike assigned to it should be protected first, and the proposal for that is not written.

## Three changes that reached the paper

The first train landed at half past eight in the morning. It carried the first step of the receipts work, a change to how writes are grouped, and the census. The grouping change was correct and did not help. I had predicted it would raise the number of writes that share a commit, and it stayed at one point one six. Writers are queuing one step earlier than the place I changed. That prediction is scored as false, and the next lever is known.

The receipts work was the hard one. Its second step came back from review with a third request for repair, and the rule says stop patching at three. I stopped and wrote an assessment. The mistake was in the frame. You had given permission to rebuild indexes on the live thirty-gigabyte table overnight, and I had been building a controller inside the service to do that by itself. A permission for a supervised act does not need a controller. I split the work into code that only writes the new format, and a script that a person runs and watches. That version was accepted in three smaller rounds, landed just after one in the afternoon, and deployed twenty minutes later.

The walk that rewrites old receipts started on its own at deploy. It did your computer first, then the paper. As of ten to six it had rewritten three point one million of the paper's five point one million rows. The paper's slowest appends in any ten-minute window have stayed between five and fourteen milliseconds, against a limit of one second that I set before the deploy. The table has grown about three gigabytes from rows waiting to be cleaned up, and the database's own cleanup ran for the first time in four days. The walk should finish around eight in the evening UTC. The index swap on the live table has not been started. It is a separate act with its own stop rule.

The first step of the stored-result change landed at a quarter to one and deployed half an hour later. It teaches every reader to follow a command that names its revision instead of carrying a copy. Nothing writes that form yet. An hour later the paper's log had no errors from it, and that prediction is scored as true.

## Retirement took four rounds, and one defect was mine

The repair to run-memory retirement deletes rows for good, so it got both reviewers and a hard look. Round one and round two each found real ways it could delete a conversation that a document still needed. Round three, both accepted.

Then I asked the builder for one more thing. Both reviewers had noted that each pass re-reads a great many records, so I asked for a memory of those reads that lasts for a whole wake. The next review showed that memory could go stale within the hour a wake lasts, and a stale entry could cause exactly the deletion the earlier rounds had closed. I had the memory deleted outright, not narrowed. A cache whose safety rests on an argument does not belong next to an irreversible delete. The read cost is now counted and logged, and the paper's first wake will tell us whether it matters.

One small window remains, and it is written into the record. Inside a single pass, about two minutes go by between reading the records and the last delete. If a protected conversation changes state in that window, the one that should take its place can be deleted. The document still has its other protected conversation. The landing run for this change started at a quarter past five and has not finished.

## Two changes that wait for you

The second step of the stored-result change makes a command stop carrying its duplicate. It is reviewed and accepted. Your ruling was that a change to what the tape contains is proved on the test computer before the paper takes it. I looked for a way to do that and there is not one in a normal landing. The deploy does visit the test computer first and the paper last, but the waiting times between them are set to zero, and any change that touches the agent core opens every ring at once. The paper would have it within ten minutes.

There are three ways through. The test computer's own engineering desk can re-author the change as a self-development apply, which is possible today, uncertain for a five-hundred-line change, and needs you for the revert and for the restore drill. We can restore a waiting time on the first ring, which is a small commit and cuts against expediting everything. Or we build the candidate pin that was designed on November tenth and never built. I recommend the first, on an evening you can be present.

The prune-at-boot change removes old releases from a computer when it starts. The first design drew a third repair and I stopped it. The redesign, a unit that runs at boot, was accepted by both reviewers. Your ruling for that case was no landing without you, so it is held. I recommend landing it.

## Things I got wrong

I told you a red self-test in the continuous integration scripts was an artifact of my machine. It was not. Main was red, and nothing runs that test unless the scripts folder changes. I recorded it, and the receipts landing repaired it. It passes on main now.

This afternoon I flagged Keenable as failing with intermittent auth errors. When you asked me to test the key, the log showed one slow refusal and a fifteen-minute bench, and the key answered eight of eight. You were right that we had fixed this, and the fix had a hole: it forgave errors and timeouts and did not forgive a refusal. That is now repaired, reviewed, and queued to land. The first review of it caught a second flaw, where a real rate limit after a run of failures would have been punished as a fifth strike.

Of the predictions I wrote down before the evidence came in, three were true and three were false. The grouping change is the false one that matters, because it told us where the queue really is.

## What I would do next

The thing to remember is that the paper's store was not shrinking because two jobs meant to shrink it were doing nothing, and both are now either running or about to be.

If I were holding the phone, I would do three things. Decide how the second stored-result step gets its proof, because that one is blocked on nothing else. Say yes or no to prune at boot. And look at the retirement numbers tomorrow morning, after the paper's first wake, before deciding anything about the store-once design. Tonight I will finish the retirement landing and watch that first wake, land the Keenable repair, let the receipts walk finish, and only then run the supervised index swap.

## Names and receipts

Goal file: `docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.md`, with its ledger beside it.

Deployed on November fourteenth, all times UTC:

- Counters: `a21558f9b`, landed 06:38, CI run 43567350353 (one flake rerun), deployed by 07:06.
- Train `79dd71b57` (receipts step 1 `6e7edb6e0` `04456bc5f`; late take `b1c912315` `134122de1` `7bec4376e` `5fc79dee3`; census `e5d00ba21` `79dd71b57`), landed 08:35, CI 43576567543, deployed 08:47.
- Stored-result step 1: `a1ec394fe` `b2494ce52` `d90131351`, landed 12:46, CI 43596488443 (rerun after two unrelated timing failures), deployed 13:12.
- Receipts step 2a and 2b: `7abf6494a` `4a8d80a2a` `a810c61cc` `40ce2fb07` `61c92229b`, landed 13:16, CI 43598955268, deployed 13:33. Staging is on `61c92229b`.
- Flake fixes: seven commits to `afb7ec449`, landed 16:20, CI 43615046206, deploy skipped by design (tests only).

Every non-docs commit above was expedited. Earlier expedites from the night, not retold here: `d6adf5c97`, `1accce528`, `eecafd3f1`, `30c9b1f34`, `d4a27b2a2`, `801f76b3e`, `a512e78d5`, `a5c175d3c`, `4de97d863`.

In flight or held:

- Run-memory retirement: `lane/retire-r1-land`, tip `bb7c72d4f`; landing run started 17:14. Prediction C85.
- Keenable repair: `lane/keenable-auth-land`, tip `560f6d8d1`; accepted, queued.
- Stored-result step 2: `lane/stored-result-step2`, tip `6299ddc61`; accepted, held for the d@e proof.
- Prune at boot: `lane/prune-at-boot`, tip `3ae395c89`; accepted by both seats, held for the owner.

Predictions settled: C74 true, C79 false, C80 false, C81 false, C82 true, C83 true. Open: C84 (receipts walk, settles 20:00), C85 (retirement), C68 (settles November fifteenth, 12:00).

Problem records written, under `docs/problems/`: the Node A out-of-memory kill; timing tests that fail under load; the receipts step 2 structural assessment; the classifier self-test red on main; the Keenable 403 and bench.

Rollback for every landing above is a revert, except stored-result step 2, which once deployed may be reverted to step 1 and never past it.

Not investigated: the paper's snapshot root is nineteen hours old; the Patch Watch workflow failed on `61c92229b`; the supply-chain accept job fails intermittently.
