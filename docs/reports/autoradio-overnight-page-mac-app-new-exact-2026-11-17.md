# The night Autoradio got a page, a Mac app and a newer Exact

I wrote this early on Tuesday the seventeenth of November, Universal Time. It covers about ten hours of work, from Monday evening, when you went to bed, to just after six on Tuesday morning. It replaces the short notes I sent through the night, and the closing letter I had drafted on Monday and never sent.

Your phone has a new build that opens on an Autoradio page, and nobody has heard it yet.

Here is where things stand. The phone build sits on the latest Exact. It opens on the Autoradio page I designed. It keeps your content on the phone, and it is meant to start talking sooner. The same code runs as a native Mac app, and that app spoke under an automated check. Two server changes are deployed and proven: the programme now writes its opening first, and podcast clips are matched to stories automatically. Clips went from one story to seventeen. EmbeddingGemma runs on the Mac by both routes. It has not run on your phone. Three things wait on you. Launch the phone app and tell me whether it speaks. Listen to a few clips and tell me which are wrong. And try full-band listening, which I built but did not merge.

## Monday evening: the silent phone, and what it taught me

You came back after five hours and found that the new build made no sound at all. I had checked that build in the simulator, by looking at screens. The simulator has pretend audio, so it could not fail the way your phone did. The cause was small. The app asked for the microphone on a session that was set up for playback only, and the audio engine refused to start. A builder fixed it, and you confirmed that the voice was back.

Then you found two voices talking over each other when the microphone was on. That is fixed too. The rule now is one voice at a time. When you speak, the programme pauses at once. When the radio answers, the story waits and then picks up from the sentence where it stopped.

You also asked for retrieval, so the small model on the phone knows more than what is in the queue. That is built. When you ask about the Senate, it searches everything on the phone and gives the model the best passages, newest first.

The lesson of the evening was plain. A check that looks at a screen says nothing about sound. From then on I asked every build for a line in its log that counts the audio samples it rendered.

## Late Monday: nine jobs, one lock

You gave me a long list before you slept, and I wrote it into one goal file with nine stations. I ran eight builders in separate lanes. They all shared one build lock on your Mac, because the rule is one Mac build at a time. That lock was the bottleneck all night. I kept the rule.

Moving to the latest Exact meant crossing about three thousand upstream commits. It broke in six small ways, and each one showed up only by building. Two of them needed something installed on your Mac. I added the iPhone targets to the newer Rust toolchain that Exact now pins. I also ran Exact's own installer for its JavaScript engine, which is pinned by checksum in Exact's source. Both are build dependencies in your user folders. I am telling you because I installed them without asking.

The new Exact also checks the finished app against Apple's privacy rules. It refused our build because the app looks at free disk space without declaring why. We now declare the four such things the app really does, with reason codes taken from Apple's own page.

## The small hours: three bugs that only running the app could find

The Mac app built, launched and showed the front page on the first real try. Then I pressed Play and it went quiet. The cause was a bug in code the phone shares. With nothing on air, pressing Play started the radio, and then the same press saw that it was playing and paused it. You had not hit this, because the phone usually starts itself two seconds after launch. A fresh install would have.

The second bug came from the Autoradio page's simulator tour. When you start the radio by pressing Play, the opening ended and nothing followed. I had guessed that the test data was at fault. I was wrong. It was a real bug, and the build that was on your phone until tonight very likely had it. If the radio ever went quiet after its greeting, that was probably why.

The third was mine. I built the phone candidate in a folder whose copy of Exact was still the old one. The setup step printed one error line and carried on, and the build passed against the wrong Exact. I caught it because I checked the version after seeing that line. The setup now fails loudly, and every build refuses to start if Exact is not the pinned version.

I also made one design call. The phone speaks its own opening at launch, and the server now sends an opening first as well. You would have heard two greetings in a row. The rule is now one opening per start.

## Before dawn: two landings, and the clips

The opening-first programme landed on its first attempt. On the drill computer, the server's first spoken segment arrived in a median of about five seconds across five tries. I had given that landing even odds, and it came true.

The clip matcher went through review three times. The reviewer first found that its limits held within one run but not against what the server already had. My own dry run showed the same thing, with one broadcast matched twice. The repair then introduced two smaller faults, and the reviewer caught those as well. It landed first time after that. I replaced the worker on your Mac, ran it once, and staging went from three clips on one story to thirty clips on seventeen stories. No story has more than three. On the clips I had judged by hand, the matcher is right about eleven times in twelve. So expect two or three of the thirty to be wrong. I have not listened to any of them.

## What the hourly watch saw

The paper published all night and spent nothing on paid models. Devin's quota refusals stopped for a few hours and then came back at a low level. Search has a second provider answering again. At six in the morning, Node B's free disk fell by thirty-eight gigabytes in one hour, and the paper's data image doubled. That matches the pattern from Monday, where a snapshot writes a whole copy before uploading and the space comes back. I have not confirmed it from the snapshot's own log. If the space is still gone when you read this, it is something else.

## What I would remember, and what I would do next

The thing to remember is that every real defect this night was found by running the real app, and none by reading or by the test suite. The suite grew past a thousand tests and passed every time. It passed while Play paused itself, and while the radio stopped after its greeting.

If I were holding the phone, I would open the app and wait. It should greet you and keep going into the first story. Then I would turn voice on and interrupt it. If anything is wrong, I can put the previous build back in about ten minutes.

After that I would run the embedding trial on the phone, which needs one launch with a flag, and then connect the embedder to the content store. Full-band listening is ready for you to try. It keeps the normal sound while you are silent, and switches to echo cancelling only when it hears you talk.

## Names and receipts

- Phone build installed: `lane/autoradio-integ4` at `bfa5674f0`, on Exact `de212e671`. Kit suite 1021 tests, 0 failures. Not launched, not heard. Rollback: `lane/autoradio-integ` at `5a7813642`.
- Lanes inside it: `lane/mac-app` (`dbb466e69`), `lane/phone-retrieval` (`21a882649`), `lane/instant-start` (`2e773295a`), `lane/radio-page` (`bfa5674f0`). Not merged: `lane/voice-fullband` (`cb6def5c9`), `lane/embed-trial` (`1e24b6f7a`). None of the phone work is on main.
- Mac app: `mobile/scripts/proof-mac.sh` printed MAC PROOF OK at `e8f4454a4`: three entries on air with nonzero samples, 22 models registered. The Mac build of the final candidate has not been run.
- Opening-first programme: `0316d7cfc`, CI run 44019332807, `scripts/prove-autoradio-programme` PASS on the drill computer, open median 5.17 seconds. Expedited by trailer and by path. Ledger C116 true.
- Clip matcher: `24cd67412`, CI run 44019560901. Staging clips: 3 for 1 story before, 30 for 17 after. Expedited by trailer. Ledger C117 true. Previous worker binary kept beside the new one.
- Server run sheet, from Monday night: `6ba3e1080`, deployed, ledger C114 true.
- EmbeddingGemma on the Mac: GGUF loads in 0.5 seconds, 23.5 passages a second, 161 megabytes; Core AI loads in about 8 seconds, 26.8 passages a second, 1,110 megabytes. Recall at five on thirty questions: vectors 1.00, keyword search 0.77. Numbers and commands in `docs/reference/EMBEDDINGGEMMA.md`.
- The trial lane vendors a third party's Swift runner under Apache-2.0. It stays off main until you say.
- Goal file: `docs/archive/definitions/choir-autoradio-overnight-2026-11-17.md`. Its claim, ledger C115, is still open and needs your verdict on the phone by 14:00 Universal Time.
- Installed on your Mac by me: Rust targets `aarch64-apple-ios` and `aarch64-apple-ios-sim` for toolchain 1.99.1, and three Hermes engine bundles under the cargo folder.
- Known and not fixed: one "extend" answer from the server began with an opening; the page's mid-day tour fails at a later step; the listening state is not photographed; the greeting said "Good evening" at one in the morning; mail size on the phone is an assumption; the scorecard in the calibration ledger has not been recounted.
- My wrong statements this night: that all five lanes merged cleanly; that the radio stopping after its opening was probably the test data.
- Rollback for every landed change: revert the commit.
