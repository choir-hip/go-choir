# The night Autoradio started keeping score

Written before dawn on Wednesday, November eighteenth, 2026. It covers one night of work, about eight hours, from Tuesday evening until your check-in at a quarter to six. I led the work and Sonnet builders wrote most of the code.

Autoradio now keeps score of its own guesses about you, and the build on your phone is the one that reports them.

Here is where things stand. The first two strokes of the learning engine are live on staging and proven on the drill computer. The disc jockey forecasts whether you will hear each segment through. Your phone reports what you actually did. Only your phone's report can settle a forecast. The third stroke, the notebook the disc jockey reads and writes lessons into, is built and in review, and it is not deployed. Takes are built on both sides, and the two sides have never met with a real recording. The server side landed on main just before five, after five rounds of review. It was deployed at a quarter past five, and its proof passed on the first run. Your phone has a new build, installed at twenty-six past four and not launched. It is the fifth install since midnight, and every repair in it was read by a reviewer before it went on. What I need from you this morning is small: listen, skip what bores you, and tell me what breaks.

## What you can do on the phone this morning

Open Autoradio and listen the way you normally would. Every segment the disc jockey places now carries a forecast. When you hear one through, that forecast settles as true. When you skip one or cut it off, it settles as false. When you close the app, sign out, lose the connection or the audio fails, it settles as nothing at all. That last rule was your open question from Tuesday. I took the conservative answer, and you can overrule it.

The notebook page will say it is not available yet. That is honest. The scorecard underneath it is live, and I can read it back to you at any time.

To give a take, tap Take on the radio page while something is on air, speak, and publish. A take runs from three seconds to a minute, and the phone stops the recording by itself just short of the minute. The server is live, so publishing should work. If it does not, the phone keeps your recording and you can publish it again later. No real recording has ever gone from a phone to the server, so yours will be the first, and I expect it to teach us something.

Podcast clips play from fingerprinted audio now. Thirty-six of the forty-nine clips in the store carry a fingerprint after the repair pass. Twelve were skipped and one could not be found.

## The night on the server

The settlement work was the centre of the night. It was deployed just before half past two and its proof passed on the first run. I had given that only forty percent, so I was too cautious, and I wrote that down.

Two reviewers read the settlement code before it landed, and they found three things that mattered. A lost report from the phone could leave one entry stuck on air forever. The rule that you do not mark your own work was enforced only by the shape of a message, so the server's own code could have settled a forecast. And a bare report that a segment was heard, with no outcome stated, counted as heard through. All three were repaired and reviewed again before landing. A follow-up landed at a quarter past three and its proof also passed first time.

The takes server went through five rounds of review before it landed, and then it deployed and proved first time. I had given that forty-five percent. That is the third landing in a row I priced at a coin flip or worse that came true first time. A change that two reviewers have accepted after repairs is more reliable than I have been giving it credit for. The count of blocking findings went three, one, none, then one more after I rebased it onto the settlement code. That last one is worth telling. The server was writing a take onto the run sheet as an ordinary story, with no pointer to the take's audio. The phone builds the same entry as a take. Each side's tests passed. Nothing would have failed until a real take was played to a real listener.

The notebook came back from its first review with three real findings, and I have ruled on each. The disc jockey could retire a lesson that the evidence contradicted and write it again in new words, and the count would restart at zero with no trace of the old one. Now the old numbers are kept where you can read them, and a lesson can only be retired by citing evidence about that lesson. The second finding is an edge I chose to show rather than close. The disc jockey decides which of its forecasts name a lesson, so a lesson that reads nine heard of nine may have been cited only where a hit was likely. The page will show how many forecasts were made while the lesson was live, beside the count. The check the disc jockey cannot steer is the overall scorecard. The third finding was plain waste: every settled forecast rewrote your document. Counts will now ride along with real changes, or be written at most once an hour.

## The night on the phone

This is the part I am least proud of and most glad I did. Three days of phone code had gone onto your phone without any reviewer reading it. Review is tied to landing on main, and none of the phone work has landed. It was installed from side branches. So I ran three reviewers over the installed build, and all three sent it back.

They found real things. When a message or an imported file was read aloud, its title went to your computer with the run sheet, and so did the rest of a passage when an answer cut in. Your rule is that mail and local files never leave the phone, and this broke it. Sign-out left behind the last mail read aloud, your listening signals, the conversation and your voice profile. The first-run microphone check could never pass. Sending the app to the background with voice on could stop a podcast. A clip could not be replayed. And the words you spoke were being written to the log.

All of that is repaired, reviewed again and in the build on your phone. The fix for the mail and file titles has one visible edge. A book or a message shows its real title on the radio page and in the bar, because the phone keeps the title locally. Once it has finished, the heard list shows a neutral title, Read aloud or A message. One leak is still open. An audio file played from Files still posts its file name inside the entry's address.

I made one decision for you that you may want to reverse. Sign-out now deletes imported books, the Files cache and your enrolled voice from the phone. It seemed right that signing out leaves nothing of the account behind. If you would rather keep books across a sign-out, say so.

## What only you can check

I cannot hear the phone. Nothing about sound has been verified by a person: playback through the speaker and over Bluetooth, the return from the background, a real phone call arriving mid-story, a real recording, and whether a file made by the phone's recorder passes the server's check. The simulator shows that pages draw. It does not show that the radio sounds right.

There is one older claim still open. On Tuesday I gave forty percent to the background audio fix holding on your device. Its deadline has passed without a device check, so your first session this morning settles it.

## What went wrong in how I worked

Six times in one night, two sides of a connection were built apart and disagreed while both sides' tests passed. I have written that up as one problem with one cause. A contract between two programs lives in prose and in each side's private test data, not in one checked file both must satisfy. Where a shared file exists, as it does for the run sheet, the disagreement showed up as a failing test, which is what we want. The fix is one fixture directory per connection, written by the server side and decoded by both.

I also started one landing without entering a forecast first, which the contract asks for. I entered one for the part still open and noted the miss.

## Autopaper overnight

The paper kept publishing all night. Two flags stand. Devin refused thirty-six calls on quota in the hour before each of my last two checks, and about a tenth of calls went to free fallbacks. And free disk on the server dipped by about fifty gigabytes just after midnight and then recovered. I looked into it without changing anything. It is the paper's store making its compact copy, which we recorded on the fifteenth. The new part is the pattern. It now happens once or twice a day, it finishes, and the server gets the space back. But each copy needs free space equal to the whole store, and the store has doubled this month. I made one mistake here. I wrote a second problem record before reading the first, then deleted it and folded the evidence into the original.

## What I would do next

The one thing to remember is that the loop is now closed far enough to score. Your listening this morning produces the first real settlements. Everything after this, the notebook, the lessons, trust between listeners, stands on those numbers being honest.

If I were holding the phone, I would listen for twenty minutes, skip freely, and record one take. Then I would ask for the scorecard.

While you are out I will land the notebook once its review accepts, land the phone work on main so it can be reviewed like everything else, and start the Android experiment you asked for.

Decisions waiting for you, each with the default I am using. Closing the app counts as no verdict. The document is called My notebook. Testers are invited only. A take that its author pulls keeps the counts it earned. For TestFlight I need three things from you: the paid developer team, the App Store Connect record, and a distribution profile.

## Names and receipts

The goal file is docs/definitions/choir-autoradio-learning-loop-2026-11-18.md. The live station is in docs/ACTIVE.md, and what is built is in docs/current-architecture.md.

On the phone: branch lane/autoradio-integ19, commit 014d71f7f, installed at 04:26 local time. It passed 1,235 package tests with none failing, a simulator build, and four simulator tours. Earlier installs tonight: integ13 at 00:36, integ15 at 01:20, integ17 at 03:36, integ18 at 04:03.

Settlement on main: f710dda6c, 53537c7ca, f54e92f48, then the follow-up de53cb778 and 843ee5d80. CI runs 44257019990 and 44265751276. Staging reported f54e92f48 at 07:26 universal time and 843ee5d80 at 08:35. Proof: scripts/prove-autoradio-learning on the drill computer, passed on its first run both times. The second run printed: settled plus seven, no verdict plus three, scorecard seventy-seven forecasts, fourteen settled, eight heard through.

Takes server on main: 3f3cfdb7b, 9edf286b8, a1c3ac93a, ce719fe1a. CI run 44280975897. Proof script: scripts/prove-autoradio-takes. Staging reported ce719fe1a at 10:13 universal time. The proof passed all eight checks on its first run on the drill computer: publish, list, audio served back byte for byte, own take not offered, a stand-in take played as a take segment with a forecast, own outcome refused, delete.

Notebook: branch lane/learn-notebook, not landed. Both reviews are in the session scratchpad and their findings are with the builder.

Clip fingerprints: landed as cd2301994, CI run 44240158109.

Calibration ledger, docs/calibration/claude-ledger.md: C121 true, C122 true at forty percent, C123 true at seventy percent, C124 true at forty-five percent, C118 open and waiting on your device, C120 open until November twenty-fifth.

Problem records written tonight, all under docs/problems: autoradio-two-sides-of-a-wire-built-apart-2026-11-18, autoradio-phone-code-unreviewed-for-three-days-2026-11-18, and an update to compact-copy-grows-the-host-image-and-restarts-discard-it-2026-11-15.

Every landing tonight was expedited, as the contract requires until you say Autoradio is solid. There were nine expedited commits. The five settlement commits named these defects: forecasts could not settle because the disc jockey's entries were not on the server run sheet; a lost end report wedged the on-air slot; the forecaster's own reports could settle its forecasts; a contradictory end could settle true; and the reference described a rule the code did not implement. The four takes commits named these: takes cannot be published or offered until the server exists, twice; every honest full-length take would have failed every listener's programme request; and a take placed as a story would never settle as a take. Each expedite kept the planned refresh, the health guards and automatic rollback.

Rollback for any of the server changes is a revert of its commits. The phone rolls back by installing an earlier branch.
