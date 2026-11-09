# The ordinary prompt path, a checkpoint

I wrote this on Monday, November ninth, in the afternoon. It covers about twelve hours of work on the ordinary prompt path mission, from your report a little after three in the morning, Greenwich time, to about half past three in the afternoon.

An ordinary research prompt now reaches a cited answer in about a minute and a quarter, where this morning it took well over three minutes.

That was the mission's central bet, and the test on d at e supported it. The question "What's latest in ai?" got its first cited version in seventy-eight seconds, against two hundred and two this morning. Its final version, with twenty-four citations, came at about two minutes. Every planned station has landed and deployed. So have the five things you added along the way: the themes on b at c, the appearance flow, the account switcher, the restart-to-update notice, and sandbox pause. One proof is still open. I have not yet watched a live engineering sandbox pause across a real update on d at e and come back. That drill runs at the next code release. Then this mission closes, and the Autopaper work starts.

## The morning: why prompts were slow

Your report described three things: few sources, a runaway on a note, and a slow second version. I read the traces before changing anything. The slowness had one main cause. The model pool kept trying members that were already known to be dead, and every dead try cost time. Research also reported late and read sources the desk could not see. Two stations went at those causes directly. Routing now skips dead members. Research now shows every read, checks its packet against what it actually captured, and sends an early report as soon as it has its first source. A third station stops a tool loop that repeats the same call with the same result. The loop is told once, then stopped, with a visible reason.

## The middle of the day: the rest of the stations

The layered image went to every computer, with every tool layer attached, and tools now leave memory when nothing uses them. On d at e, at rest, the tool layers hold no memory at all. During an engineering sandbox the toolchain holds about forty to sixty megabytes, and it drops back to zero afterwards. Texture gained images, charts, video, audio and file embeds. The engineering path produced a real artifact: research fetched two government data series, and engineering turned them into a PDF report with a chart, delivered to Files and linked from the Texture page. Along the way I fixed three defects the traces exposed. First, a model choice now follows the whole trajectory, not only the Texture desk. Second, Python bytecode files no longer leak into a sandbox's candidate change. Third, a candidate left behind by a settled trajectory no longer blocks every deploy to its computer for two hours. That third one is a good lesson. It looked like d at e refusing updates. Underneath, it was one orphaned candidate holding the gate.

## Your additions

The theme rounds on b at c taught me something real. Rounds were finishing their installs but never closing, because the system never wrote the record that says the install happened. After that fix, Aero Heart installed and closed within thirty seconds. Metal Heart and Utopian Scholastic had started before the fix, so they stayed open, as expected. Liquid Scholar is the round running now.

The appearance flow is live. Files has a picker that opens as a window over the desktop, and images in Files can be set as the wallpaper. The desktop's right-click menu changes the background and theme. Settings is in tabs, so that menu opens straight onto Appearance.

The account switcher is live and passed its test on choir dot news. One browser holds several signed-in accounts. You switch between them from the launcher, sign one out, or sign out of all. Its first push broke two tests elsewhere in the code, which I fixed within the hour. The repo's pre-push checks did not catch either break. The lesson is to run the full set of front-end checks that CI runs, not just the ones nearest the change.

The restart-to-update notice is live. It answers your point about b at c. A computer that is always busy never looks idle, so the rollout kept skipping it. Now the desktop shows "Updates available" with a button to restart, and running work resumes after the restart. On staging I confirmed that b at c, already current, answers "already up to date" and restarts nothing. I also confirmed that a key without your owner permission is refused. I have not yet seen it restart a computer that was actually behind.

Sandbox pause took the most design work. Three rounds of review kept finding protections that the "re-attach the old sandbox" approach dropped. On the third round I changed the approach rather than patch it again. The resume now starts a fresh sandbox the normal way, seeded with the old one's saved changes. Every protection then comes from the ordinary start. Review still found two real problems in the build. One could leave a paused sandbox frozen forever. The other could let a sandbox plant a link that redirects root writes on resume. Both are fixed. A crash still never resumes work, and a planned restart resumes at most once.

## What remains, and what comes next

The one open item is the live pause drill. My first attempt missed its window, because the engineering task finished before the release arrived. A builder is now writing the last review fix for sandbox pause, which makes cleanup unmount only mounts the system itself lists. I will start a long engineering task on d at e just before that push, then press restart to update and watch the sandbox pause and come back. After that, this mission closes. The Autopaper mission is next. It starts with the paper's slow health checks under store contention, and the sixteen hundred management tasks queued on the paper. Autoradio follows it.

The thing worth remembering is that the slow prompt was mostly one cause, routing into dead pool members, and fixing that cause did more than any tuning would have. If I were holding the phone, I would open the desktop on b at c, try the account switcher, and watch for the update notice after the next deploy.

## Names and receipts

- Goal: docs/definitions/choir-ordinary-prompt-path-2026-11-09.md (status checkpoint_incomplete), ledger beside it.
- Conjecture test: d@e trajectory b28c14d8, doc 3addfd44, at eaf66a1aa; v2 at 78 s, v3 at 126 s.
- Account switcher: 9f3faa7c0 through 60dbdd310, CI fix 943b6e2dd; deployed proof frontend/tests/account-switcher-deployed.spec.js.
- Restart to update: b180565b8 through 417472207, CI fix 6e0d016a3.
- Sandbox pause: c6e1af593, 23b3dac7a, eaf66a1aa; security epoch 127.
- Appearance flow: 3b2aa7eef. Wake fix: 5b5c07a80. Orphan candidate: 9f8b1a47d, c0af6603a. Bytecode: bc689ad91. Overlay: 867c5c5c5.
- Designs: docs/design/account-switcher-2026-11-09.md, update-now-2026-11-09.md, sandbox-pause-resume-2026-11-09.md.
- Every landing commit carried Ring-Expedite (owner tempo: expedite until Autoradio is solid).
