# Catching up on the launch first pass

Written early on Saturday, October twenty-fourth, at about two in the morning UTC. It covers about twelve hours of the launch first pass, from mid-morning Eastern time on Friday the twenty-third until about ten at night Eastern, and it ends with the course for overnight and the questions I would like you to answer in one prompt.

Twelve deploys reached staging since mid-morning, and the paper is healthy at the end of the day. The one serious slip, an eighteen-minute outage of the VM manager, is understood, fixed and proved.

Here is where things stand. The paper runs on ChatGPT luna for every role, with Synthetic only as a fallback, as you asked. Four security switches are on and proved: the object graph's internal routes now require the host credential, the Dolt root account is locked, guest DNS on the test computer uses only the host resolver, and the Firecracker jailer runs on two fresh test computers. The paper and T no longer count every event at boot; they read a small lease instead. The paper's database journal now grows at roughly half yesterday's rate. Self-development on T works again after a repair of old records. Event table deploy 2, the change that moves trace lines out of the busiest table, has passed three review rounds and goes out after its waiting period ends at six fifty-two UTC this morning. Nothing is waiting on you to keep going. A few decisions would let me go further, and they are at the end.

## The morning: models and sections

The day started with two things you raised. The paper had lost its sections, such as world news and politics. The cause was not the paper itself. The scorer that ranks front-page stories calls a model through OpenRouter, and the account had run out of credit, so every score failed without a visible error. After your top-up, all sixty of the newest front stories carried a section again. I wrote that up as a silent provider outage, because a failure that looks like an empty result is the dangerous kind.

You also noticed Synthetic was draining fast, about eight points of its weekly allowance an hour. You asked for luna everywhere with Synthetic as a fallback. I moved the paper to luna alone at about a quarter to three in the afternoon UTC, then built a small pool that tries luna first and falls back to Synthetic only when luna fails. That went live in the evening. You also told me that an owner-decided switch like this does not need code review. I took that as a standing rule: a change you already decided, with a one-step rollback, gets the automatic checks only.

## The afternoon: making the checks faster

You pointed out that the build machine on node A should be removing bottlenecks, not adding them. You were right, and the bottleneck was my own process. I had been running every landing's full race test serially on that small VM, about an hour each. I changed the protocol: each landing now gets a draft pull request, which runs the full race suite on GitHub without deploying, and the VM runs only a fast check. After that, landings that used to wait an hour cleared in minutes.

The first deploy of the afternoon failed its tests on an old flaky test, not on the new code. I fixed the test so it waits for the background job it was racing, and landed again. That deploy added a label to the paper's stall log, so whenever the store stalls, the log now says whether the database's online cleanup was running at the time. It caught a twenty-eight second cleanup pass on a journal of one point four gigabytes. That made the case for writing less, which became the evening's theme.

## The evening: a run of deploys

Between six and eleven in the evening UTC, six deploys went out, one at a time, each with its own proof on staging.

Guest DNS on T now uses only the host's resolver, with no public fallback. A probe I built found that the fallback was still quietly in use, and I fixed it behind the per-computer switch so the paper keeps its fallback until its turn.

The self-development repair went out next. Some old records on T carried a stamp that blocked replay, so T could not take self-development checkpoints. The repair re-stamps only rows whose state still matches, and refuses the rest. On T it healed two rows, the replay checked out, and a second run found nothing left to do. It never ran on the paper. The same deploy carried a fix for a real bug that the tests uncovered: every one of our services could miss its own stop signal if the signal arrived before it started waiting.

Then the run-memory index diet. Three database indexes were never read but were written on every run-memory append. Dropping them took about half a second on the paper, with health checks at two hundred throughout. Since then the paper's journal has grown at between fifty-four and a hundred and twenty-five megabytes a minute, against about a hundred and sixty-four the day before. Part of that gap is the earlier trace cuts, so I do not credit it all to the diet.

Two security switches followed, each alone. The object graph's internal routes now refuse a caller without the host credential, and the paper still publishes its verdicts. Then the Dolt root account was locked. The host verifier passes two hundred and seven checks, and a passwordless root login is refused on both database servers.

The last deploy of that run was the stream sequence lease. Every event the computer records gets a number, and until now each boot found the next number by scanning the whole event history. On the paper that scan took two minutes and forty-three seconds on its first boot under the new code. From then on, each boot reads a small lease row instead. T read its lease in a fifth of a second, and the paper read its lease in about one point three seconds at its next restart. Both are slower than the fifty milliseconds I aimed for, which I recorded but did not chase. The lease is also the precondition for event table deploy 2.

## Late night: the jailer, and the outage

The jailer is Firecracker's standard way to run each VM's process as its own unprivileged user, sealed in its own directory, so a VM that escaped its guest would find very little. I turned it on for one freshly made test computer, and its first launch failed at once. Our Firecracker build from nix is dynamically linked, and the libraries it needs do not exist inside the sealed directory. The only earlier real test had used upstream's statically linked binaries, which is why nobody saw it. I recorded the problem first. A worker then built the same pinned version as a static binary, and vmctl now refuses at startup to jail with a binary that cannot run sealed. After that deploy the test computer launched jailed, hibernated and woke jailed in under half a second, and an assumed-breach probe, run as that VM's own user, was refused every secret file and every port I tried.

Then I made the night's mistake. I added a second test computer to the jailer, and that deploy restarted vmctl, the VM manager, while the first jailed VM was still running. systemd could not start vmctl again, ten times in a row, and gave up. From two minutes to midnight until sixteen minutes past, nothing could launch, wake or refresh. Running computers kept running, the paper included. I stopped the jailed test VM, cleared the leftover settings by hand, and started vmctl, which reattached everything. Then I turned the jailer off and sent you a notification.

A worker confirmed the cause from the systemd and kernel source and reproduced it on the agents VM. The jailer writes resource controls into every parent cgroup of the VM, including vmctl's own, and the kernel then refuses to start vmctl's first step there. The lesson is that I treated the restart as routine because restarts are routine. The rollout plan listed restart and reattach as a check for later, and the next deploy ran that check for me on staging.

The fix moves jailed VMs into their own systemd slice, outside vmctl. It deployed cleanly. Then I turned the jailer back on for both test computers, launched them jailed, confirmed they share one read-only store image, and restarted vmctl on purpose with both running. It came straight back and reattached all four VMs. The fix is proved.

Two smaller findings came out of the night. A CI test pinned vmctl's exact unit text, which blocked the slice's first deploy; I updated it. And the slice worker found that vmctl's memory protection does nothing, because its parent slice has none to pass down. I recorded that and did not act on it.

## What I will do overnight

Under your four goals, secure, scalable, stable and self-development, the night looks like this.

For scalable, event table deploy 2 goes out after six fifty-two UTC, on T first and then the paper, with its proof script. Trace lines are about a third of the journal, so this is the largest remaining cut in writes.

For stable, the slice's follow-ups land first. They make the jailer fail safe if vmctl cannot read its own cgroup, and they add a deploy check that refuses to restart vmctl if anything is left in the old place. I will also watch the paper's health and the two thirty-five backup, which is the first nightly job to log in to Dolt as root with a password since the lock.

For secure, I will widen the jailer from the two named test computers to every test computer, and widen host-only DNS the same way. Every real computer, the paper and yours included, stays as it is until you choose.

For self-development, T can take checkpoints again. Next I want a complete self-development cycle on T, from a change in a capsule through apply, checkpoint and restore, proved end to end, as the step before your own computer.

The one thing worth remembering: the risky part of a rollout is often the routine step next to it, so the proof plan should run that routine step on purpose, early, on a test computer.

## Questions for one reply

You can answer these in any order, in one prompt.

First, the jailer for real computers. Moving every computer onto it means each hibernated computer cold boots once, and the paper and your computer restart at their next launch. Should I do that overnight, wait for a window you pick, or wait for you?

Second, memory protection. Should I give the system slice a memory reserve, so that vmctl's fourteen gigabytes of protection actually works? I would read memory pressure first and propose a number.

Third, trace-line batching on the paper. This is the telemetry batch setting that has waited a few days. Yes or no?

Fourth, the roadmap re-cut on its branch still waits for your review. Should I keep it parked, or fold it into tomorrow's thinking?

Fifth, owner-act enforcement needs one browser session from you to prove. Any time works.

Sixth, the legacy tape delete of about forty-six gigabytes. Your yes is recorded. Should I spend the night on its preconditions, the keep set, a restore drill and an off-host snapshot?

If I do not hear back, I will do the overnight plan above, keep real computers off the jailer, leave memory policy alone, and leave batching off.

## Names and receipts

Staging serves d9336696b (D30); main is at 880ca9638 or later. The ledger is docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md and the status page is https://claude.ai/artifact/WArejUzHzD42RnT7SMpP2F (version 46).

Deploys in this letter: D17 c53237618, D19 883a38c21, D20 e0bd976a3, D21 06979ca71, D22 2b7d130c3, D23 46e4e0a8e, D24 a01d2e1f0, D25 e30c8b51a, D26 372cd205b, D27 f84c8fe34 (its deploy failed), D28 1fec78a1b, D29 0d943bee0, D30 d9336696b. D18 shipped inside D20.

Problem records written today: docs/problems/front-importance-silent-provider-outage-2026-10-23.md, docs/problems/server-shutdown-lost-wakeup-2026-10-23.md, docs/problems/jailed-firecracker-cannot-exec-dynamic-binary-2026-10-23.md, docs/problems/vmctl-restart-fails-while-a-jailed-vm-runs-2026-10-24.md, docs/problems/vmctl-memory-low-protects-nothing-2026-10-24.md. The lease design is docs/problems/stream-seq-high-water-mark-design-2026-10-23.md.

Open pull requests: #94 (event table deploy 2 r3, head f1b76e0fa, approved in bunch 181) and #96 (the slice follow-ups, lane/fp0-vm-slice-r2, in review as bunch 182).

Test computers: T is computer-71afbf14; the jailer test computers are fp8-jailer-1023 (computer-476cd794) and fp8-jailer-1023b (computer-51443441). The paper is computer-1355b83b.

The outage: vmctl down 23:57:58 to 00:16:20 UTC on October 24; failed deploy run 40041867841; recovery and cause in the vmctl restart record above.


## Merged from docs/reports/launch-first-pass-catch-up-2026-10-24.md (2026-10-22)

**The launch first pass, from Wednesday evening to Thursday afternoon.**
Written Thursday, October twenty-second, a little after four in the afternoon UTC, which is just after noon your time. It covers about twenty-two hours of the launch first pass, from Wednesday evening at six UTC until now, and it is my own account as the orchestrator, FP0.
- Status page: https://claude.ai/artifact/WArejUzHzD42RnT7SMpP2F

(Full text in git history at the path above.)
