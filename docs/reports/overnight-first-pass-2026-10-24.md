# The overnight first pass, October 24

I wrote this on the morning of Saturday, October twenty-fourth. It covers the night's launch work, from about two in the morning to half past three in the afternoon, Universal Time, and it picks up where last night's letter stopped. I last revised it at half past three.

The platform got sturdier overnight, but the app redesigns are not done yet.

Ten deploys landed, one at a time, and each was proved on staging. One proof is still owed tonight. Self-development ran a full cycle on a locked-down test computer for the first time. Engineering stopped burning tokens polling its own build job. Three outages surfaced and each now has a fix: vmctl's list of every computer stopped answering after a self-development apply; the paper was down for an hour after its nightly compaction; and the platform's main database froze three times. The app rewrites on the b@c test account applied one new Slides app at half past four. You looked at it and said it was a restyle, not a redesign. Five bigger redesign attempts followed. The first four were lost to host failures, to the desks misreading a staged plan, and to a refused finishing word. The sixth froze one candidate covering all five apps, and it looked worth approving. A product bug blocked the checkpoint every apply needs. I fixed that bug, then rejected the candidate itself, because its old runtime cannot read what the fix now writes. Its patch is saved. In the afternoon I tried one more rebuild of the five apps. It came out better, and it could not land for the same reason. That was my mistake, and I explain it below. Two research write-ups are ready for today's design session: one on how diverging computers take updates, one on where prospective self-alignment stands. Your questions are at the end.

## The early night: confinement and a full self-development cycle

Between two and half past three I widened the jailer and host-only DNS to every test computer. The jailer runs each computer's virtual machine as its own unprivileged user. All three test computers cold-booted jailed in about thirteen seconds each, and each resolved names only through the host. One lesson came early. The off-host backup ran from half past two to twenty past three, and while it ran, vmctl refused to start any computer for lack of memory. I recorded that. The real fix is deleting the legacy flat tape, which is most of the nine million files the backup walks. That needs your yes.

At ten to four, a self-development drill passed all nineteen of its checks on a freshly made, jailed test computer. Texture cast management, management cast engineering, engineering built and froze a change, and then the owner approved it. The host built the release, the computer restarted once and served the new route, and a restore put it back. That is the whole loop on a confined computer.

The drill also showed, for the third time, that engineering polls its own running build in a tight loop. In one round it polled thirty times in seventy-three seconds, then stopped a healthy build. I designed and landed a fix I call the job park. When engineering checks on its own running job, the turn simply waits until the job ends. The proof drill this morning parked four times, once for almost seven minutes on a Go build. Engineering used seventy-three percent fewer input tokens than in the earlier round.

## Four in the morning: two outages at once

At four, the paper began its nightly store compaction. The compaction itself went well, shrinking the store from fifty-five gigabytes to nineteen. But the computer's boot credential lasts ten minutes, vmctl mints it before the compaction starts, and the compaction took eleven and a half. The paper then looped on an expired credential and stayed down until five. The fix landed this morning: vmctl mints a fresh credential after the maintenance step, and an expired one now ends that boot at once instead of looping. Tonight's compaction is its real test.

At twenty-eight past four, the first self-development apply on the b@c computer made vmctl's whole list fail closed. The classifier did not recognize the new kind of ownership evidence the apply had written. Other computers kept routing, but b@c's could not be reached. Both review seats approved a narrow fix, and it went live at five past five.

I made one mistake in that hour. I read an empty watcher output as the list failure, when it was really the paper being down. I corrected the ledger once I saw it.

## Dawn: the database stalls

From six on, the platform's main database froze three times: at nine past six, at a quarter to seven, and at ten to eight. Each time, the liveness probe aborted it and systemd restarted it. Two of those restarts killed long engineering runs on b@c, about half an hour of work each time. One also kept a computer from starting.

A read-only investigation found the cause. The database is capped at three gigabytes of memory. That cap was sized in early October, when the database used about one gigabyte. It now sits pinned at the cap. Its own garbage collector pushes past it, the kernel throttles it, and the store's locks back up behind the collector. There was a second cost hiding there. With the heap filling the cap, the database had almost no page cache, so it read about twenty megabytes a second from disk.

The fix tells Go about the cap. It sets a three-gigabyte soft limit and raises the hard ceiling to four. In the first forty minutes after the fix, the database was throttled about twenty times less often, its page cache grew from eighty-seven megabytes to over a gigabyte, and it read about eighteen times less from disk. One garbage collection has passed since with no stall and no probe strike. I count the stall as fixed but want a second clean collection before I say it is closed.

## The app rewrites

The rewrites run on the b@c account's computer, under an Opus conductor. Texture writes a supervision document and revises it as the work moves, so you can follow it from the desktop.

Slides applied at twenty-seven past four, after two rejected candidates. You looked and said it was a restyle. You asked for real redesign: an upload inside Slides that lands in Files, a PDF that opens in a reader rather than Slides, press-and-hold menus, and the many other obvious features. The conductor took that as the bar. The desks now write a product spec into the Texture document first, and work across apps is in scope.

Then the night taught three lessons about the platform.

First, successive applies do not build on each other. Every self-development run starts from the computer's original source, not from what the last apply left. So a Files change would have quietly undone Slides. The conductor switched to one combined change for all five apps.

Second, a long engineering run loses everything when the host stumbles. Work is saved only when a run finishes, so the two runs the database stall killed left nothing behind.

Third, staging work in rounds fights the platform's vocabulary. The conductor tried five short rounds, with each round ending "blocked" so the next would carry its work. The rounds themselves were good: Files with a real context menu and open-with routing, one Reader for PDF and EPUB, and a new Slides. But "blocked" means failure in the code, and the desks read it correctly. They kept casting recovery rounds. One recovery round broke the carry chain, and the three finished stages were lost. They survive only as a local patch file.

A fifth attempt started at twenty to ten. It is one round for all five apps, with the lost stages as the minimum to match. The b@c computer now carries the job park. No redesign has been approved yet. The night's model cost on b@c was small: about forty-three million input tokens, ninety-six percent of them cached, for about two and a half dollars.

## Late morning: a candidate worth approving, and why I held it

I added this section at half past eleven. The fifth attempt froze four apps and was then lost to one wrong word. Engineering finished with "partial", the completion check refused it, and the frozen work went with the run. It also never saw the Podcasts brief. Engineering's view of its assignment is cut off at four kilobytes.

The sixth attempt worked. At thirteen past eleven it froze one candidate covering all five apps. Files is now the hub. It opens each file by type, so a PDF goes to the Reader. It has an open-with menu, a context menu on right-click and long-press, upload by picker and by drag, and a preview pane. One Reader handles both PDF and EPUB. Slides saves its decks into Files and can upload a deck inside the app. Calendar has day, week and month views, and imports and exports calendar files. Podcasts has a queue, playback speed and resume. The conductor rates it clearly better, with a few small regressions.

I have not approved it, for two reasons. First, the required checkpoint before an apply was refused. The cause is a real product bug. Changing the theme, or opening a PDF or a podcast, writes to three small tables the checkpoint requires to be empty. So any computer that has been used normally cannot take a checkpoint, and cannot self-develop. Approving anyway would leave no rollback except a checkpoint from before Slides. A worker is now building the fix: those three tables go onto the tape like the desktop's window positions. Second, the candidate's runtime was built from the computer's original source, eighty-one commits behind today's platform. That is the same "applies do not build on each other" problem, and it belongs to the design day.

At twenty past one the checkpoint fix went live. On b@c, a theme change recorded the old rows on the tape, the computer became eligible again, and the checkpoint that had been refused went through. That fix has a one-way side. The tape now holds records the candidate's old runtime cannot read, so if that candidate were applied, a later restore would leave the computer unable to boot. So I rejected the candidate rather than leave an unsafe approve button waiting for you. Its patch is saved. Once applies build on each other, it can be rebuilt on current source and offered again.

## The afternoon: one more fix, and a round I should not have run

At half past two the tenth deploy went live. Engineering now sees a long assignment whole, up to sixteen kilobytes instead of four. When a request is still too long, the cut-off note says plainly that the rest cannot be read from any desk, and asks engineering to report the cut so the sender can resend it shorter. The first version of that note pointed engineering at management, which cannot read the original either. The reviewer caught that.

Then I made a mistake. I believed the refreshed computer would let engineering build on today's source, so I cast the five apps once more, with the sixth attempt's review notes in the brief. It took forty-four minutes. The candidate is the best one yet: it builds cleanly with no warnings, and it keeps every contract the sixth attempt broke. But it was built from the same old source. The reason is simple, and it was already written down. Each computer's copy of the source is checked out once, when its disk is made, and nothing moves it afterwards. So every candidate on every computer is built from the commit the computer was born at, and ships a runtime from that commit. Since this morning's checkpoint fix, that runtime cannot read the computer's own tape. I rejected the candidate at twenty-three past three, kept its patch, and put engineering back on its original model. One look at that source copy before casting would have saved the round.

This sharpens the design question. Until a computer's source copy follows the platform it boots, no self-development candidate can be safely applied on any computer made before this morning. That is the first slice of making applies build on each other, and I have stopped casting app work until it lands.

## The research for today

You asked for real research on updating computers that diverge, and on prospective self-alignment, precommitment records and in-context learning in the REPL. Both write-ups are on main.

The update design splits each computer in two. There is a floor that the host owns and always patches: the guest kernel, the updater and the boot path. It never moves backward, not even on restore. Then there is a lock that the computer owns: its base, its ordered stack of applied changes, and its app packages with their pinned versions. A security fix rebuilds or replays the computer's own stack onto the fixed base, under a new evidence class that admits only deterministic transforms of fixes the host signed. Stock software and packages become subscriptions you choose. The proposed first slice is the same substrate the app work hit: make applies build on each other. The design also turned up three things worth checking on staging. A deploy that moves the base may silently drop a computer's own Go change. A kernel-only patch drops every layer. And a computer marked divergent gets no updates at all, security included.

The self-alignment write-up found more built than the October fourteenth status said, but no loop is closed. Engineering makes real precommitments that the runtime settles from receipts, but it never sees how they settled. The calibration digest for the other desks is built and switched off by your ruling. The smallest slice is giving engineering its own settled record, behind a switch that is off by default.

## What I would carry forward

The one thing worth remembering is this. The platform's weak spots last night were not the new features. They were old sizing and old vocabulary: a memory cap set when the database was a third its size, a credential life shorter than a maintenance window, and no word for "good, continue". Each failed loudly once it was looked at.

If I were holding the phone, I would start the design day on making applies build on each other. It blocks the app work, it is the first slice of the update design, and its precommitments are the cleanest place to start in-context learning.

## Questions for you

You can answer these in one message, by number.

1. Update design: when a critical fix conflicts with a computer's own changes and you are away, should the computer park its stack, contain itself, or stop? My lean is park when stock code can read its data, otherwise contain.
2. Update design: how many past platform bases should get backported security fixes before a computer must move?
3. Update design: once applies build on each other, should a published release carry the cumulative change against the platform base, or the whole stack?
4. Should making applies build on each other be the first slice? It is red, with two reviewers.
5. Self-alignment: may engineering see its own aggregate settled record, behind a switch that is off by default?
6. Self-alignment: is your approval a settler for engineering's claims?
7. Should engineering gain an outcome that means "good, continue", so staged work is not a failure claim? And should a run save its candidate before it finishes?
8. Should a planned restart count as activity for the idle sweep? Right now a computer can hibernate thirty-five seconds after its apply restart.
9. May I delete the legacy flat tape? It is about forty-six gigabytes, and it is why the backup blocks computer starts.
10. For b@c, keep Synthetic DeepSeek as engineering's model or go back to luna? The conductor has restored the original policy for now.
11. The five-app work: the best candidate's patch is saved, and it cannot be applied until a computer's source copy follows the platform it boots. Should that seed fix be the very first slice of the update work, ahead of the rest of making applies build on each other? Or would you rather land the five apps as an ordinary platform change on main, for every computer, and keep self-development for later?

## Names and receipts

- Deploys: D31 cc6479939 (VM slice round 2); D32 0cbc52edc (jailer and host-only DNS for class test); D33 824b84106 (self-development promotion classification); D34 911be16b1 (engineering job park); D35 92771df65 (event table deploy 2, inert); D36 0894e48fe (platform Dolt GOMEMLIMIT 3GiB under MemoryHigh 4G); D37 22a01746f (boot credential re-minted after offline maintenance; decisive proof at tonight's 04:00 compaction); D38 dc3f5ee3b (stream_seq seed by phase; the paper's seed 550 ms, append 391 ms).
- Ledger: docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md, through 29d8a0f65.
- Problem records: docs/problems/selfdev-apply-on-a-constructed-computer-breaks-vmctl-list-2026-10-24.md; offline-maintenance-outlives-the-boot-credential-2026-10-24.md; engineering-busy-polls-its-own-job-2026-10-24.md; platform-dolt-gc-throttled-at-memory-high-2026-10-24.md; idle-sweep-hibernates-a-computer-just-after-its-apply-restart-2026-10-24.md; successive-selfdev-applies-do-not-compound-2026-10-24.md; texture-supervision-doc-stops-at-the-decision-boundary-2026-10-24.md; on lane/fp0-apprewrite: staged-engineering-rounds-break-on-desk-reads-of-blocked-2026-10-24.md and long-engineering-run-dies-on-corpusd-cas-invalid-connection-2026-10-24.md.
- Design: docs/design/computer-update-architecture-rnd-2026-10-24.md; docs/design/psa-precommitment-icrl-status-2026-10-24.md.
- Evidence: docs/evidence/selfdev-drill-2026-10-24.md; drill traces under ~/.config/choir-qa/traces/fp0-selfdev-1024a and fp0-jobpark-1024b.
- Apps: b@c computer computer-6c50d6e566143ae32cdb660faa60e24d; Slides apply selfdev-41ea6ea6 (04:27); v5 trajectory 80db9c3f-f482-53c6-8604-be36bb0e6283, Texture doc 679b943b-e4ac-5abb-b03b-22f33ec0e1d8; lost v4 stages in /Users/wiz/Dev/lanes/_runs/apprewrite/v4/stage3.patch; conductor log /Users/wiz/Dev/lanes/_runs/apprewrite/log.md.
- D39 a3715cdc2 (media and preference tables as tape projections; b@c checkpoint ce22d325 at 13:23:28). The five-app candidate: op selfdev-79dc4bf4 (rejected 13:2x, patch kept), v6 trajectory 9da63b10-127f-53da-818b-92716d14f03a, Texture doc d4fe7dda-97b2-532b-b943-c5dd7272201c, patch /Users/wiz/Dev/lanes/_runs/apprewrite/v6/patch.diff. The checkpoint bug: docs/problems/stock-ui-writes-make-a-computer-checkpoint-ineligible-2026-10-24.md; fix lane lane/fp0-media-replay. Clustering assessment: docs/problems/engineering-work-product-durability-cluster-2026-10-24.md.
- D40 b2cebe931 (a Texture Cast's objective reaches engineering whole up to 16 KiB; an honest cut marker; CI run 40101276835; vmctl clean 14:32:16; the paper refreshed 14:32:41). Record: docs/problems/engineering-cast-objective-cut-at-4kib-2026-10-24.md.
- The seventh five-app attempt: trajectory 408ec51d-9cff-52b5-832e-5fd8a253337c, Texture doc a922c601-20e9-5adf-b330-b403f563b982, op selfdev-37b1b5a5 (frozen 15:20, rejected 15:23), patch /Users/wiz/Dev/lanes/_runs/apprewrite/v7/patch.diff (sha256 a70b862f…, applies to 0cbc52edc). Record: docs/problems/self-development-candidates-build-from-the-creation-commit-2026-10-24.md (7f5615e61); the seed is vmmanager sourceSeedStagingRoot.
- Held, not landed: lane/fp0-jobpark-wired 9d9fdc884 (a source-shape test; the staging events are the wired proof).
- Owed: D37's proof at the 04:00 compaction; a second clean platform Dolt GC. b@c's System/model-policy.toml is restored to its original (read back identical at 15:23).
