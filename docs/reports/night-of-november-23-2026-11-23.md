# The night of November twenty-third

I wrote this on Monday the twenty-third of November, a little after eight in the morning Eastern. It covers the ten hours from when you went to sleep, about ten forty at night, to now.

Five things landed and deployed overnight, the new phone build is on your phone as Autoradio 1, and one of the things I deployed made research run four times as often, so I am taking it back out.

Here is where things stand. Main is green and staging runs it. Sign-in can now return to the app, on the server side, and the stranger's proof of that passed. A real defect behind a red build was found and fixed. The voice ask that searches the paper is built, passed its review on the third round, and is going through its full tests now. The new phone train has every review fix in it. Its sixth simulator proof passed one hundred and sixty-seven checks and failed one, where the server did not answer the Files page. I installed that build as Autoradio 1 a few minutes after eight Eastern. You have four things to decide, and I list them near the end.

## What landed

The first thing was not planned. The build on main went red twice in the platform tests, and it looked like a flaky test. It was a real defect. When the index that links objects to their kinds is being repaired and a second repair starts, the second one gave up quietly and reported success, leaving the index broken. It now waits its turn and checks again. The follow-up made the tests themselves trustworthy, including one that waited on fixed times and would have flaked again.

The second was the server half of signing in from a sheet. The approval page can now send your phone back to the app, with nothing in the return address but a one-time marker. Both review seats accepted it. I ran the proof a stranger can run against staging and all six checks passed. What it cannot show is the approval itself coming back to the app. That needs your passkey and your phone.

The third was the research prompt change, which I now regret landing without a way to watch it. More on that below.

Before those, in the first hours, the fourth draft of the first design went in, with the corpus share as its measure and the store's real numbers. So did the system map, the phone model benchmark, and the run viewer.

## The research change went wrong

The research prompt change deployed just before three in the morning Eastern. Within fifteen minutes, research runs went from about thirteen an hour to about sixty. Captures were flat, management was flat, and the page writers were flat, so it was not the Monday news. Each run searched about as much as before. There were simply four times as many runs. After the deploy, forty-one of forty-eight new stories got a research request on their first round.

I believe the cause is one reworded rule that tells the page writer to ask research for whatever a page leaves open. I put that at a little better than even, and I could not prove it, because the log from before the deploy is gone.

The cost is real but not urgent. Model calls doubled, the provider refused about one call in twenty for quota, and web searches are running at more than twice the credit budget.

I did not tune the prompt on a guess. I prepared the plain reversal of the whole change and wrote the prediction down first: once it is deployed, runs fall back under twenty-five an hour. If they do not, I was wrong about the cause.

## The phone train

Five reviewers read the train in the night and found two faults that would have reached you: a skip that could leave the next entry silent, and a loop that asked the desk for more every second or two when it had nothing. Both are fixed, with the other findings. The radio now asks a dry desk once, then watches the front page and asks again when it changes.

The reviewers also found that an update computed for one account could land after sign-out. That is closed for the radio, the edition and the reader. Closing it exposed something worse: the developer menu, which is on in the app you carry, can reload the shared code, and after that every radio update would have been refused until you killed the app. That is fixed too. The remaining small windows are written down for the account boundary assessment.

The count of native lines was wrong. Six files that are not voice had been listed as voice. The true count was higher than reported, and after deleting dead code it is now lower: forty-nine files and about fifty-eight hundred lines.

The simulator proof ran six times. Most of its failures were the proof's own faults: it inherited state from the run before, and it read a log that had already overflowed. Those are fixed in the proof.

The fifth run led me to something you will hear. After the opening, the radio is silent for ten to twenty seconds, sometimes longer, at every launch. The opening lasts about thirty seconds, and the desk's first stretch takes forty to fifty to arrive. Between one desk entry and the next there is no gap. The old build hid this only because it usually ran dry after the opening and never reached a desk entry. I did not fix this overnight, because the fix is a choice: ask the desk before the opening plays, or fill the wait with a story from the paper.

Looking for that silence found a real fault. A second request to the desk could replace the first while it was still waiting, and throw away its answer. Now one request waits at a time.

## What I got wrong

I queued a proof under a name the build queue refuses, and lost twenty minutes. I switched the checkout of a tree while the landing script was using it, so the script pushed nothing; the tested commit was intact and I pushed it. I told you the new train launched seven times slower, and then that it left silence between entries, and both were misreadings that the builder corrected from the logs. And I landed a prompt change with a proof that was "owed after a day", when the damage showed in fifteen minutes.

## What you need to decide

First, the research reversal. It will be ready to land this morning. If you would rather keep the new prompts and pay for the runs, say so.

Second, the developer menu in the app on your phone. I would turn it off in the builds you carry.

Third, your current app's signing runs out at about seven in the evening Eastern on Tuesday the twenty-fourth. Autoradio 1 is its replacement once you are satisfied with it.

Fourth, the silence after the opening. I would fill it with the top story from the paper, because that works even when the desk is slow. Asking the desk earlier only shortens the wait.

Autoradio 1 is on your phone now. Please lock the phone with the radio playing and listen across three or four entries, delete and reinstall once to try the microphone on a first launch, and try signing in from the sheet.

The one thing to remember: the server side moved forward and is proven, the new phone build is in your hands and needs your ears, and the research change is the lesson of the night.

## Names and receipts

Main is at 2f5f4d7a3. Staging serves 33fc8ff10 for the computers, with 6229cd73a deployed after it.

Landed: 3b5f6fd34 and 6229cd73a, the index lock; 977442ff8, device approval return; 3dcddbe86 through 33fc8ff10, the research prompts, to be reverted from lane research-revert; 81de84c34, design draft four; 44d146c61, the system map; fa1ea5973, the model benchmark; 99338db60 and bd643bc55, the run viewer.

Every expedite: each of the commits above that is not docs carried a Ring-Expedite line. The index lock, for the red build. Device approval, for sign-in in a sheet. Research prompts, for miscounted sources. The run viewer, for a run that could not be read step by step.

Proof run: scripts/proofs/device-approve-return-deployed.sh, all checks passed at 08:12 UTC.

Phone: Autoradio 1 was installed at 01:33 and 05:04 UTC from lane exact-ar1-next, and at 13:08 UTC from lane exact-train2 at dcc56f2cd. Nothing else on the phone changed. The first app was not touched.

Node B: read only. No service was restarted. Disk is at 161 gigabytes free, after one dip to 106 during the daily snapshot's copy.

Train: lane train2 at dcc56f2cd. Simulator proofs train2p1 through train2p6. Voice search: lane ask-search at 7b6b53f33. Claim C201 in the calibration ledger is the research prediction. Problem records: the guest-reach file missing from the security surfaces, and native pushes without an account generation.
