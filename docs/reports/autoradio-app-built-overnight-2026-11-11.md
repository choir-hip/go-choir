# The Autoradio app, built overnight

I wrote this in the early hours of Wednesday, November eleventh. It covers the night's work on the Autoradio phone app, from Tuesday evening to about half past two in the morning, roughly nine hours. Nothing was installed on your phone.

## The news

The whole app now exists as one build, and it runs against your real paper in the simulator.

Here is where it stands. Nine builders each made one part of the app on top of a shared spine: the bar and the player, the paper, the reader, the pile and launcher, files, mail, settings, voice setup with the interview, and the motion work. A tenth agent merged them. Two rounds of fixes followed, each started from my own reading of the screenshots. Then a last builder signed the simulator app in to the test computer on staging and toured every screen with real stories, real documents and real mail. That tour passes. So does the demo tour, in a true light mode and a true dark mode, with two hundred and sixteen Swift tests behind it.

What it is not yet: it has never run on a phone. No real microphone, no real voice, no real model download, no real sign-in through the browser. Those are the next step, and they are yours to schedule, because they need one install on your phone.

The app is on a branch, not on main. I held it there on purpose. It is thirty-seven thousand lines, and I would rather you see it on the phone before it goes through review and lands.

## How the night went

The first stretch was your rulings. You said the interview is voice only, open microphone, interruptible from the first word, and that it runs on the engine we already have. You said the body font belongs to the theme. You said to do as much in Exact as possible, with Contract first so the app can move to Android. I rewrote the plan and the Exact reference to say those things before any builder started.

Then the server. The app needed four routes that did not exist: who am I, entities on the front page, the edition, and a theme read from a Texture. Those were built, reviewed, landed and deployed around half past eleven on Tuesday night. I made a mistake there that cost an hour. I reported a landing as passed when I had only seen the exit code of the command that printed its last lines. The landing had failed. Then it failed again, because the builder's repairs had never been pushed and I had trusted the word "pushed". I now check the branch head and the real exit code myself.

Then the lanes. Nine builders worked at once, but the Mac can only stand one Exact build and one simulator at a time, so they queued for a single lock. Early on I had the proof script shut the simulator down after each run. Every boot put the machine at a load of two hundred for minutes, and nothing could attach. Keeping one simulator booted fixed it. By about one in the morning all nine lanes passed their own proofs.

A pattern showed up in the lane screenshots. Five lanes' light mode pictures were dark. The cause was one thing: a setting the app has written itself beats a setting written from outside. The merged proof now deletes the app's saved settings and taps the real control. That lesson, and about twenty-five others, are now in the Exact reference pages on main.

The merge went better than I expected. Five conflicts, all small. The merged app passed its tour on the fifth run.

## What I saw, and what the second round fixed

I read the merged screens as a designer would. Light mode was already good. The paper, the reader, the Textures list, mail and settings looked like one app. But several things were wrong. Three buttons in voice setup were system blue and ignored the theme. The pile of open pages was washed out, with the page behind showing through as ghost text. The player had a hole under its title and a second prompt field. The bar said "Make it yours" in the middle of the interview. And "Start the radio", at the end of the interview, did not start the radio.

The second round fixed all of those. The radio bug was simple once found: the button toggled play and pause, so after the interview it paused. The bar now says where you are: setting up the voice, checking the microphone, interview paused, writing your edition.

In the same round an engine builder did three things the screens had asked for. The speed buttons now really change the speed and keep the pitch. A turn you cut into now records that it was cut, and the words you had actually heard, so the interview can reframe from there rather than guess from timing. And mail's "Read it to me" now reaches the engine.

## The real-data round

This was the part I was least sure of, because the lanes had guessed the shape of several server answers. I expected it to find broken screens. It found several, and fixed them.

"More on this" in the reader was always empty with real data, because the client read the wrong field. The app never asked your computer whether an edition exists. The account page showed a dash for the computer. Long ids ran off the edge of the screen. And there was a real crash: a set was being changed from a background thread. All of those are fixed, each with a test built from the real shape.

The largest find was the prompt bar. Typing a question and pressing send did nothing but record a signal. It never reached the computer. It now runs a turn the same way a spoken one does. Two real questions reached the test computer. I did not see a useful answer come back within forty seconds, so that path is proven to send and not yet proven to answer.

One thing is designed wrong and not fixed. A typed question aims at the story the radio has loaded, not at the page in front of you. The plan says the page in front. That is an engine and desk change, and I left it for daylight.

## What is rough

Mail read aloud shows the wrong title in the bar, and I have not confirmed the message itself plays. The bar's "speaking" during the interview is a guess. Section names on the real paper show as raw ids, such as "tech underscore a i" in lower case. The Textures list stops at a hundred documents. Opening a document fetches every revision of it. PDF is drawn in a web frame, and EPUB falls back to plain Markdown. Saving a mail draft twice makes two drafts, because the server has no route to update one, and sending from the phone is refused, so for now it emails you an approval link. The empty mail screen, offline states, and the Wake and Update buttons were not exercised with real data.

## What I would do next

The one thing to remember: the app is whole and reads your real paper, and every remaining unknown is on the far side of a phone install.

If I were holding the phone, I would agree one install this morning and walk the first run: sign in, download the voice, the microphone check, the interview, the edition, then the radio. That single walk tests everything the simulator cannot. While you do that, I would send the branch to Gemini for review in pieces and fix the typed question's aim.

## Names and receipts

The app branch is lane/ar-app at 3d1b1934c. The same commit is lane/ar-app-real. It is 248 files and 36,986 added lines under mobile, 116 commits ahead of main.

Server routes on main, deployed to staging at 04:31 UTC on November eleventh: 3df4e78c7 is the last of ten commits. Every one carried the expedite trailer "Autoradio app blocked on" the missing route.

Docs on main: 6f0848e2c folds the Exact lessons into docs/reference. Earlier in the night: 4e2c6a3b8 (the plan), 2f9af0f5c (Gemini as review seat), b99548d50 (Exact capabilities).

Proofs, all exit zero. Demo tour: mobile/scripts/proof-app.sh, 216 tests, 35 light and 23 dark screenshots. Real tour: mobile/scripts/proof-real.sh, 24 screenshots, signed in as the d@e test account.

Created on the test computer: one Texture named "Autoradio theme test", two forked copies of a story from the two typed questions, and edits by the desk to its Master document. Two model-call prompts were used.

Not done from the list I carried into the night: the Jev before-and-after comparison, the fifteen front stories without entities, the ledger entries for the two failed landings, and the goal file's current-state card.

