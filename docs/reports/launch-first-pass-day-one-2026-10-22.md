# The launch first pass, from Wednesday evening to Thursday afternoon

Written Thursday, October twenty-second, a little after four in the afternoon UTC, which is just after noon your time. It covers about twenty-two hours of the launch first pass, from Wednesday evening at six UTC until now, and it is my own account as the orchestrator, FP0.

Staging has taken twenty-two deploys since Wednesday night, each proved on a test computer, and after an afternoon scare over Node B's disk, the landing queue is moving again.

## Where it stands

The first pass is the security and density checklist, with the self-development items included. Its job is to make your computer, and everyone's, safer to run and cheaper to keep on, before the launch work on apps begins.

Right now staging runs deploy R. That deploy put the gateway's token binding into enforce mode, so a model token used from the wrong computer is refused. It also landed a safer way for each computer's database to shut down. Both were proved on a test computer this afternoon. Deploy S is in continuous integration now. It checks that a capsule's network tunnel really goes where it says it does, in observe mode everywhere and enforced on one test computer.

Behind S, two more deploys are staged and checked, and the kernel and Firecracker update is reviewed and ready for after three this afternoon your time. That update needs your go for the Node B reboot. The checklist shows nine of sixty-five items fully closed. Many more are landed but still in observe mode, waiting for their enforce step or their last proof.

## Wednesday evening: the scope and the first clock

The pass started at about six on Wednesday evening, UTC. You narrowed it to the security and density checklist and asked for self-development to stay in scope. Five, then six, lanes started building at once. Each lane is a worker with its own branch and its own station.

I timed the first deploy end to end. Continuous integration started fifteen seconds after the push, the tests took eleven minutes, the deploy finished at fourteen, and the paper was serving at sixteen. The proof took about ten more. So one deploy slot costs about half an hour, and that number shaped the whole day. My working belief, which still holds, is that landing is the bottleneck, not building.

## Wednesday night: reviewers ran out, and the rules changed

Around midnight the reviewers I was using ran out. The GPT reviewer hit its usage limit, and the Gemini reviewer had no quota until next week. You changed the rules so the pass would not stall. Reviews now happen in bunches, not one station at a time. The GPT reviewer is no longer a gate. A red change, meaning one that touches a protected surface, lands on accepts from Gemini flash, DeepSeek, and a third seat that is either Devin or, since midday, a Sonnet subagent.

You also gave me the agents VM on node-a as a place to run Linux tests. I built a mirror of continuous integration there: it rebases a lane onto main and runs the race-detector tests on the packages it touches. That mirror caught real failures all day before they reached main. It also taught a lesson about memory. The VM has nine gigabytes, and the kernel's out-of-memory killer took several workers and mirror runs at once early on. Since then, heavy tests take a lock.

The night's first deploys fixed things that the proofs exposed. One test failure looked like a model problem until I read the gateway's log: thirty-six of thirty-nine calls had failed on an expired API key, not on anything the model did. Deploy A now parks a key for six hours when the provider rejects it, and moves the call to the next key. You decided to remove zot, the old container registry, and deploy D did that, with all four proof steps passing.

The most useful night find was the paper's boot window. By reading the trace, I found the paper was unreachable for about six and a half minutes after each deploy. The cause was in its startup, not the network. Deploy E brought that down to about thirty seconds from boot to healthy.

## Early Thursday: a security slip of my own

Between about four and nine in the morning, my own host-side probes sent the host's internal service token to guest computers. That token should only ever go to services on the host itself. Nothing was misused that I could see, but it was the exact kind of mistake the pass exists to prevent. I stopped it at nine eighteen and wrote it into the rules. This afternoon a scanner found the same mistake in a probe script, and that scanner now runs in continuous integration, so the next copy fails the build instead of reaching a guest.

The rest of the morning landed steadily. Deploys F through O covered:

- the dedicated desk worker, which keeps each desk's code in its own small process;
- the freeze step for self-development candidates;
- the first cut of the host's egress floor, which limits where guests can reach;
- the front-page scorer running with no provider keys of its own;
- a fix for a wake that raced a deploy and left a computer running outside the registry.

You also asked me to restart your own computer, which had been stuck on an old self-development operation since Wednesday. I did, and it closed that operation with a visible "interrupted by a restart" fate, as the restart rule says it should.

## Midday: two lanes met, and the rules changed again

Deploy P landed the second batch of the desk boundary work. Its first proof step failed, and the failure was useful. Two lanes that were each correct alone interacted badly on staging: a self-test was running the new dedicated desk worker instead of the one it meant to test. Production was unaffected. Deploy Q carried the fix and passed the seccomp proof, which checks the system-call filter on desk processes. Q also fixed the self-development admission queue. A queued engineering request used to wait about five minutes behind management. Now it is admitted seventeen seconds after the one before it finishes.

At one thirty-five you adopted two rule changes. A deploy may now carry two live red changes when they touch different services, each has its own proof on its own test computer, and each can be reverted alone. Observe-first is cheaper too: a change can observe everywhere and enforce on one chosen test computer in the same deploy. The paper and your computer are still never first.

You also asked whether to move to Linux seven. A Sonnet research agent read the current state, and the answer was to stay on the six eighteen long-term line. It also found an urgent point release, six eighteen fifty-seven. That release closes a host bug a guest can reach through its network checksum handling.

## Thursday afternoon: the disk

At about half past two, deploy R failed before it touched anything. The deploy checks for ninety gigabytes of free disk on Node B, and there were only about ninety-three before its own build. Free space was falling about a quarter of a gigabyte a minute.

I documented the problem before touching anything, then measured. The cause was the paper's disk image. Inside the paper, only about forty gigabytes were in use. On the host, its image held about eighty gigabytes and was growing about a hundred and forty megabytes a minute. Our Firecracker version cannot pass the guest's freed blocks back to the host, so the image only grows. A daily compaction at four in the morning resets it. This morning's run took the store from thirty-two gigabytes to twelve. But the image refills within the day, and the platform database on the host adds about two gigabytes an hour that nothing collects. With no action, the host would have run nearly full right around four in the morning, just when the compaction needs room.

You said yes to both options: run the compaction early, and delete the week-old database backup. The compaction took six minutes and took the paper's store from thirty-three gigabytes to fourteen and a half, with the same content root, which means nothing in it changed. Deleting the backup freed only seven gigabytes, because most of it shared storage with the live databases. Free space went to about a hundred and twenty-five gigabytes, and R deployed in two and a half minutes because its build was already cached.

R's proofs passed on test computer seventy-one A F. The gateway refused nothing it should not have, with fifty-one tokens pinned. A planned restart of the test computer took twelve seconds and left nothing open. The setting that moves database commits off the write path did what it says.

The disk work gave three lessons. First, problem documentation first paid off. Writing the record before acting turned "lower the floor" into a better question, and a worker later found that lowering only the deploy floor would have let Nix's own garbage collector fire in the middle of a build. Second, I slipped once: I pushed a docs-only record to main while R was undeployed, and that started a full rebuild of R that cost about seven gigabytes before I cancelled it. With undeployed code on main, any push is a deploy. Third, a string of mysterious database crashes on the VM finally has a cause. The store tests used the same temporary folder in every process, so two runs at once deleted each other's open database. That explains a failure from one this morning and another from nine. Every mirror and worker run now gets its own temporary folder, and the real fix is in review.

## What is coming

Next, deploy S gets its proofs: the tunnel check in observe on one test computer and enforced on the other. Then T, which is the second cut of the host floor, including the database's own login rules. Then U, which moves the deploy program itself out of the CI workflow and behind a gate on the host.

After three this afternoon your time comes the kernel and Firecracker deploy. Its reviewers asked for a canary, and the plan has one. I hold the paper on its current image, deploy, and boot a fresh test computer on the new kernel. Only when that computer shows the right kernel, opens its store and answers a model call do I let the paper refresh. Then I ask you for the go to reboot Node B. The reboot is what actually closes the host bug.

The same review found something older. Memory hotplug has never worked on Node B. The hotplug region sits above what this CPU can address, and that is true of every Firecracker version, not only the newer one. Nothing that works today depends on it. The conservative step is to turn hotplug off on Node B, and I will bring that to you as a choice.

After that, the fix rounds that are already with workers come back for review and land. They are:

- the fourth round of the self-development work;
- the seventh round of the store shutdown work;
- the first deploy of the event table, which goes alone because it changes the database schema;
- the capsule memory fix, which brought a cold release build under its two gigabyte ceiling, from a peak of about one and a quarter gigabytes to about eight hundred and fifty megabytes, and faster;
- the disk-pressure trigger, which compacts the paper when the disk runs low instead of only at four in the morning.

The second deploy of the event table is the structural fix for the paper's database growth, and it is being built now.

## The one thing to remember

The pass is limited by how fast proved deploys can land, and today Node B's disk briefly became that limit. The early compaction bought about a day. The lasting fix is the event table and the pressure trigger, so the disk does not catch us again tonight or tomorrow.

If I were holding the phone, I would do three things. Give the reboot a yes or a no when the kernel canary passes. Decide whether to turn hotplug off on Node B. And when you have a minute, look at the disk lane's proposal to lower Nix's free-space setting together with the deploy floor, because those two numbers have to move as a pair.

## Names and receipts

- Ledger: docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md; goal file docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md.
- Status page: https://claude.ai/artifact/WArejUzHzD42RnT7SMpP2F
- Deploys since Wednesday night (staging build commit): FP3 group 1 94b656fdb; FP1 ca21f1ce7; FP2 batch 1 7c61ce078; A 4dbdee107; B 3c46d5196; C 3c16f7c33; D fd58229aa; E d991907e6; F 8fabec36a; G 4fca1e01b; H 43e318712; I 58e705802; J a84b39bac; K 5289387c9; L 7448bf899; M 8cb7e1acb; N 4bc3aa3ec; O 358ce87c0; P 966d3f519; Q d206fdd40; R 0bf8a097d (live 15:45:36 UTC). S ddd20de0e in CI.
- Deploy R failure: CI run 39794075862 (disk preflight, root_available_kib_after_reclaim 93,855,960 against 94,371,840); re-run of the failed job succeeded.
- Disk records: docs/problems/node-b-disk-fills-from-the-paper-data-image-2026-10-22.md (71ec8e704); lane/fp0-disk-pressure 4515538eb with docs/problems/platform-dolt-oldgen-grows-every-auto-gc-2026-10-22.md.
- Early compaction: unit fp0-early-compaction-20261022b, 15:35:16 to 15:42 UTC; store 33,020,774,378 to 14,451,421,725 bytes; root 83jajjcq unchanged.
- Deleted: /var/lib/go-choir/dolt-backup-20261015-pre-2.4.2 (freed about 7.2 GiB).
- R proofs: _runs/deployR/proof.sh; FP7 step 4 pass (refused 0, pinned 51); ARCH-13 planned restart via compact-store 15:48:24 to 15:48:34, zero "close left the store open"; debounced setting on and off.
- Staged next: lane/fp0-deploy-t-land (FP3 cut 2), lane/fp0-deploy-u-land (SEC-14 deploy 1); kernel and Firecracker lane/fp0-deploy-kf-stage 0c7798c62 plus round 2 lane/fp0-kf-r2 115bf75e6.
- Kernel research: _runs/kernel7/report.md; record docs/problems/kernel-pin-behind-guest-reachable-checksum-fix-2026-10-22.md; hotplug record docs/problems/memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md (on lane/fp0-kf-r2).
- Test computers: 71afbf14 (computer-71afbf1461dffabcce8eeebe558d03bd) and the FP2 test computer (computer-9648031f92e131adc1fb65ec455c5903).
- Shared temp path cause: lane/fp10-arch13-on-main 776bfe5ac (per-process test paths; Close checks the workspace before closing the engine).
- Token slip: ledger 09:15 entry; scanner fix lane/fp0-token-scan-2 598216a4d; CI step in ddd20de0e.
