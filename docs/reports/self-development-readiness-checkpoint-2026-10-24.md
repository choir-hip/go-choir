# Self-development readiness, first checkpoint

I wrote this on the evening of October twenty-fourth, about two hours and ten minutes after the self-development readiness metamission opened. It covers only that stretch, from about eight in the evening to twenty past ten, UTC.

The first wave is live on staging. The second is held at CI, by a frontend build bug that I have already fixed on main, and it carries the first change a computer made for itself on its way into main.

Here is where things stand. The new goal replaced the launch first pass. The first wave of platform work is deployed, and I turned on the source projection for the test computer b at c. The source projection is the switch that makes a computer's own source tree follow what it actually runs. The moment it went on, it showed something real. b at c still carries the Slides app it built for itself this morning, and that change never reached main, so the computer refuses every new self-development request. The fix is the promote path, so I built it and used it on Slides. The second wave carries that promote, the first app-packages slice, three loop gaps, and two host switches. Its CI run failed on a frontend bug, which I fixed and pushed. CI is running again now. Nothing here needs a decision from you.

## The new goal and the first wave

At about eight in the evening the new metamission opened. Its finish line is the loop you described. A coding agent asks b at c for the Reader app through the choir command line, engineering builds it on current main, you approve it in Texture, and it is promoted to main for every computer. You also handed me the switches. I now turn proved switches on myself, test computers first and the paper last.

The first wave went live at twenty-six past nine. It carries two pieces: the source projection, and the step that fetches the platform's git history into a computer when it boots on a new commit. It also fixed two loop gaps. A reject now stays possible until a change is applied. And engineering can now write Svelte files that contain backticks. On the host, the gateway now refuses oversized model requests instead of only logging them, and guest name resolution through the host widened to the primary computers. Memory hotplug went off, since nothing uses it. vmctl, the host's computer manager, restarted cleanly, with zero restarts since.

## What the test computer showed

Five seconds after I turned the projection on, b at c reported that its stack's base had moved. The stack is the Slides app, applied at half past four this morning on an older base. The design says a computer in that state refuses new casts until its change is upstreamed or rebased, and rebasing is not built yet. So the Reader request could not have run there.

That made the promote path the next thing to build, not a later one. I wrote a script that fetches a computer's frozen patch through the command line, checks its digest, applies it to main on a new lane, and predicts whether the computer's stack will retire once the change ships. On b at c the digest matched exactly. The Slides file had not changed on main since, so the patch applied as it was, and the prediction says the stack retires as upstreamed. Devin reviewed the Slides change and approved it. That is the first time a computer's own change has gone through review on its way into main.

## The second wave, and what the reviews found

The second wave holds sixteen commits:

- the first stage of the trace log;
- the first app-packages slice, which turns the image viewer into a package;
- the fix that keeps a computer awake while a command line drives it;
- engineering's progress reports reaching your Texture;
- the promote script and the Slides promote;
- two host switches.

The first host switch turns on retention for idle test computers. I read its shadow plan before turning it on. It lists five test computers, about fifty gigabytes, all kept, and nothing would be deleted today. The second switch turns on balloon statistics for b at c alone.

The reviews were worth their cost tonight. Three lanes came back with real blockers. The evidence and slot work had a race where two claims could both win. The app-package kinds accepted reserved desk names. The file object could drift from the files on disk. Fixing the slot race, the worker found a worse bug that is already on main: with every setting off, the engineering slot claim never stops a second claim, so two engineering runs could each believe they hold the slot. Both reviewers approved that fix. Then the file-object re-review found a security hole that is also on main. A symlink as the last part of a path lets a read, or a write, reach outside the files folder. I recorded both bugs in the ledger before any fix, as the rule asks, and a worker is fixing the symlink now.

## The CI failure

The second wave's CI run failed for two reasons. The real one was mine to catch. The new package boundary decided which package a file came from by reading the first folder after node modules. Under pnpm, the package manager CI uses, that first folder is a hidden store, so it refused even Svelte itself and the build stopped. My pre-CI mirror never runs the frontend build, so nothing caught it. The second failure was an old flaky test unrelated to this wave. I reproduced the build failure locally, fixed both, and pushed. I also changed the mirror so any lane that touches the frontend now gets a real pnpm build.

## Lessons, including mine

Three lessons from tonight. First, a frontend lane is not proved until a production build passes under the same package manager CI uses. The mirror now does that. Second, pushing a docs commit while a code run is queued cancels the queued run. That cost about twenty minutes tonight, so I will hold docs commits until the code run starts. Third, an honest one. Three times tonight I wrote a ledger time before reading the clock, and each time I had to correct it. The fix is mechanical: read the clock as its own step, then write.

## What comes next

When the second wave deploys, I will check b at c. Its Slides stack should retire as upstreamed, and a new cast should be admitted. If it does, I submit the Reader brief through the command line. The third wave is already approved and waiting: the per-computer switches for run-memory elision and desk-search caps, the app-package kinds with their owner route, the evidence and slot homes with the double-claim fix, and the command that pulls a candidate's screenshots. The file object follows once the symlink fix passes. After the third wave deploys, I turn on run-memory elision for b at c first. Simulation says it saves about seventeen percent of all compute.

The one thing to remember is this. Turning the projection on for a single test computer found a real blocker in the loop within seconds, and the promote path that fixes it is the same path the Reader app will take. If I were holding the phone, I would let the second wave land and watch b at c's stack retire, before anything else.

## Names and receipts

Goal file: docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.md; ledger: the same name with .ledger.md.

Wave 1: 1c707d8d5, CI 40132893674, deployed 21:26 UTC. source.projection on for b@c (computer-6c50d6e566143ae32cdb660faa60e24d) at 21:26:22; state stack_base_moved at 21:26:27, base 0cbc52edc, booted base 1c707d8d5, operation selfdev-41ea6ea6668ad096b24b28fdc626ed75, patch sha256 b26af68e37a63a0ccc154c83dc0204b8fd768654f5bdba44fde4808c100dabdd.

Promote: scripts/selfdev-promote (lane fp0-promote, da04ee81b); Slides lane fp0-promote-slides ed1468d5c, bunch 212 APPROVE, mirror slides-1 PASS.

Wave 2: lane fp0-w2, landed as 5a58facae; CI 40136125633 failed (Build Frontend: package-boundary-plugin.mjs read the first node_modules segment; race shard 3: TestHandlerDeadLettersDeliveryFailedOnceAndNamesTheSourcePoll). Fix: e270ef42a ledger, 52afc0e0b plugin, 6844e118d test; current head 9cab8e56f in CI. Mirror w2-1 failed on a test race (owner_act_receipts_test.go, fixed a34498386); w2-2 PASS. Switch review bunch 213 APPROVE.

Reviews: 206 evidence/slots SEND BACK then 218 APPROVE (Devin and DeepSeek); 207 P2 SEND BACK then 215 APPROVE; 208 file object SEND BACK then 217 SEND BACK (leaf symlink); 209 K APPROVE; 210 C APPROVE; 214 E2/E3 settings APPROVE; 216 choir self-dev visual APPROVE.

Problems recorded before their fixes: slot double claim, ledger 34b720872 (time corrected 4866c143d); leaf symlink, ledger cd2cbcb81 (time corrected 9cab8e56f); W2 CI failure, e270ef42a.

Retention shadow plan read 21:3x: 5 test-class computers, 49.8 GB, 0 projected deletes. Mirror change: ~/ci-mirror.sh on choir-agents, backup ci-mirror.sh.bak-2026-10-24.
