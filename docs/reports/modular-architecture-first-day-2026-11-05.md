# The modular architecture mission, the first day

Written Thursday, November fifth, in the early afternoon Eastern time. It covers about twenty-two hours of work, from Wednesday afternoon, when the plan became a mission, to now. It replaces the evening check-in, the morning check-in and the Node A letter as the thing to read.

The mission is about half done: the first three of its eight phases are on main and running on staging, and the third is one step from its finish line.

## Where things stand

Wednesday afternoon you signed off on a plan to make every desk and every app a package, with privilege only from grants. Since then about a hundred and ten commits have landed on main. Desks now talk to each other through one channel only: scored request records. Two small kernels hold the core, one in Go and one in the browser, each guarded by structural checks. The package manifest has a second version. And Phase 3, custody, is nearly done. Every kind of data now has one custodian desk. Every write and every read names who is doing it. The system counts every crossing of a custody line, and nothing is refused yet for built-in desks.

The next step is to flip built-in desks from counting to refusing. That needs three things: the counters to read zero through a real day, a fix that is deploying now, and one decision from you about research that has read your stories.

Along the way I found three problems outside the mission that touch the paper. Web search has been answering nothing for most of every day. The search provider you have the most access to, Keenable, was switched off for good by a single error. And a leftover migration rule rewrites one of the new free model names. The first two have a fix deploying. The third is written up and not yet fixed.

## Wednesday afternoon: the plan becomes a mission

The day began as design work. The plan pulled together several earlier designs: the system modularity design, the app desk design, and the forced modularity design. It became one document with nine phases, numbered zero to eight. Devin reviewed it and accepted it with no blocking findings. You walked through the open questions with me. You settled the trust scores as three separate numbers. You also decided where the gVisor sandbox applies, set the trust classes, chose a pilot for capsules, and gated Go modules by capability. You removed out-of-process tool servers: a package's backend is its desk.

At about five in the afternoon Eastern time you signed off, and the goal file became executable. You set the tempo yourself: aggressive, review running alongside landing rather than in front of it, and most changes expedited. Security-critical changes merge only after a review.

Before any code, I wrote down predictions about how the work would go, with probabilities, so the mission could be scored against them later. I come back to how those predictions fared near the end.

## Wednesday evening: one channel, small kernels

Phases 1 and 2 ran in parallel, with Sonnet builders each in its own worktree and me reviewing, merging and deploying.

Phase 1 made precommitment records the only channel between desks. The old emit and message channels are gone, about sixteen hundred lines of Go. Their producer had been unreachable since October anyway. One data-driven address resolver replaced the hand-written allowlists, and a differential over almost ten thousand inputs showed it routes everything the old code routed. Every request between desks is now a record that carries a probability of success, a weight and a frozen materiality procedure. The first one read back on the test computer, d@e, carried all of it with no fix. Sub-calls inside a desk are now scored requests too, settled by an asynchronous judge. Desks can also see how often their own requests turn out to be inert. Finally, the desk protocol is locked: a check in CI refuses any change to the protocol that does not bump its version.

Phase 2 built the two kernels and the second package manifest. The adopt mechanism became install. The desk registry is now driven by grants, so the closed lists of desks in the code became lookups.

Phase 1 needed three small hotfixes, all expedited. An expired CI switch failed every commit when the date rolled over, so I retired it. One test only failed on Linux. A fake judge in a test was timed by a clock and flaked under the race detector. The lesson I took: builders were testing on the Mac only, so a Linux vet now runs before every push.

## The night: a round that stranded

During the late-night test of the sub-call work on d@e, a trajectory stranded. Texture sent research a second request while the first round was still open. Research's final report closed the round over that unread request, and Texture then waited forever. This flaw is old, from mid-October, but the faster desk exchange made it more likely. I wrote it up first, then fixed it. A final report is now refused while a request in its round is unread, and a run's end wakes the sender of a request that reached a closed round. Both Devin and DeepSeek accepted the fix, and a three-round exchange on d@e afterwards settled cleanly.

The same test showed something you should know. The sub-call path the judge scores was never exercised on d@e. No prompt teaches the research call that creates sub-calls, so Texture instead sent research seventeen separate requests. Those desk-to-desk requests carry scores, but nothing settles them. That is an open question for you, below.

Phases 1 and 2 closed at about eleven thirty Wednesday night, six and a half hours after your sign-off. I had predicted half a day to three days. I was wrong because my base rate assumed a human waiting on each review. With parallel builders the critical path is CI and the deploy, about twelve to fifteen minutes a slice.

## Thursday: custody and labels

Phase 3 is the security core of the mission. It makes custody real. Every kind of data has a custodian, and only the custodian writes it or reads it, except through a lease.

It went in seven steps.

First, a custodian for every kind, derived from the manifests. The first load found twenty-six kinds with no custodian, which the step resolved.

Second, every write names its writer, and package kinds are refused at once. The writer counters found a real crossing: management's story opening wrote Texture's story document in the same batch.

Third, the same for reads. The reader counters on d@e found Texture reading computer settings, which belong to the shell. The plan's inventory had missed it, and I had predicted something like it.

Fourth, leases: a custodian can grant another desk a time-bound read of its data, and a capsule's copy of the store carries only what the lease names.

Fifth, provenance labels. A run that reads private data carries that label on everything it does afterwards, durably, across restarts.

Sixth, a disclosure gate. Where data can leave its custody, by publication, web egress or a message to another desk, a rules table decides. If the table is unsure, the System 1 judge is asked, and then you. For built-in desks it only counts for now.

Seventh, in three parts, the resolutions of every crossing the counters found. Management now asks Texture to open a story instead of writing it. Desks read a story index that Texture publishes, not Texture's documents. Research can search your stories only under a lease, and gets short snippets, never whole documents. Boot and host paths name their reader.

Every one of these steps went to Devin and most went to DeepSeek as well. Most came back with a blocking finding the first time, and those findings were real. The capsule copy shipped tables the lease did not cover. One capture label opened every private capture. A search path skipped the egress gate. The story open could leave a story half made after a crash. Each was fixed and re-reviewed before it merged.

Two outside problems slowed Thursday morning.

From about two in the morning, every d@e test failed on model quota. I checked that it was real and not a gateway artifact: ChatGPT and Synthetic were both refusing upstream, and the paper was the largest caller. That is part of why the free model pool mattered later.

Then, late morning, every deploy stopped because Node A's disk was completely full. You told me to prune it. I deleted all fifty-nine workstation snapshots and ran a garbage collection, which freed about six hundred gigabytes. I kept all six virtual machines, not four. Which four you meant was not clear, and deleting a machine cannot be undone.

## Thursday afternoon: your requests

You asked me to put the free models into a pool. The default pool now has iceman and beam from OpenCode, plus crouton, flint and nemotron through OpenRouter, alongside GPT luna. Synthetic's DeepSeek is fallback only. On your word, management's task now runs on the pool like any role; its pinned model and its free-model filter are deleted.

You asked me to move the paper onto the pool. Its owner-edited model policy now names the pool for every role. I saved the old file for rollback. The paper's API key had expired, so I renewed it; the new one expires November twelfth. Over twenty-five minutes the paper's calls spread across luna, flint, beam, crouton and iceman. Nemotron got none, for a reason I come to below.

You also noted that the paper's account hangs on a saved browser session. I recorded that as a task for after the mission: a platform operator identity, or the paper's passkey kept in the vault, so no secret sits on a developer machine.

## What today's checks found

The last three hours were checks, and they found more than I expected.

After the custody fixes deployed, I ran a research trajectory on d@e. Writes counted zero. Reads counted three Texture documents and three revisions with no reader named. I traced it: when a run finishes, the store checks whether the trajectory can settle. That check read the story without naming itself, and the test oracle skipped it because the read happens entirely inside the store. The store now reads as itself. The fix is pushed and expedited, and I will re-run the trajectory once it deploys.

The same trajectory raised the decision I mentioned. Once research reads your stories, the run carries your private label. Under refusal, every web search that run makes afterwards would wait for your approval. Sixteen sends were flagged in one short trajectory, seven of them web searches.

Then the paper's traffic sample showed web search answering nothing. I looked back thirty hours. The paid providers' daily budgets run out about two hours after midnight UTC. The self-hosted searxng gets rate limits and captchas from every engine behind it. So for about twenty-two hours of every day, research on every computer, the paper included, writes without fresh sources. Nothing fails loudly; the runs complete.

You told me Keenable should carry a hundred thousand searches a month. The reason it carried none: one authentication error had set it to disabled, and a disabled provider was never tried again, whatever its timer said. You said a single error should not block forever and that it should back off exponentially. It now does: fifteen minutes, doubling, at most six hours, and the old record clears when the gateway starts. I tested your local Keenable key and it works. Whether the gateway's copy works, I will know minutes after the deploy.

Last, nemotron. Twenty-seven calls asked for a model named "nemotron, management, VL", which does not exist. A rule left over from the desk rename rewrites "super" to "management" inside any stored string without spaces, so the model's stored name was rewritten. This is the second time that rule has misfired; in October it hit headlines. It is written up and not yet fixed. The fix touches replay, so it needs care.

## Model provenance on articles

You asked that every article say which models wrote it, including Claude delegate use and switches in the middle of a trajectory. The old version was unwired. A builder rebuilt it: every model activation is recorded, and the article's source panel and its published version list the models by role.

Devin's first review sent it back for three good reasons. The public version would have published a file path on the computer and internal ids. Research children were invisible. And the lookups would have grown with the computer's whole history. All three are fixed, with tests. Devin's second review is running now. It ships after that review, because it puts new fields into what the public sees.

## How my predictions fared

I made fifteen-odd scored predictions before acting. The most useful misses were about time and units. Phase 1 took a quarter of a day where I said half to three days, because I priced in human review latency. Twice my unit was loose enough to make a record ambiguous: "routes" when I meant "routing rules". Each miss changed how the next round is written. The predictions that held were mostly about d@e behaving as built, and about review finding real defects.

## What is waiting on you

Here are the decisions I cannot make alone, most urgent first.

- **Search budget.** If Keenable comes back, much of the gap may close. If it does not, you either replace the key on the gateway, raise the paid caps, or cut searches per story.
- **Research that has read your stories.** Should its later web searches ask you, or should research be trusted to search after reading your stories? The answer sets the last cells of the disclosure table before refusal goes on.
- **Desk-to-desk requests.** Should the judge also settle desk-to-desk requests, or should Texture's parallel research go through the scored sub-call path?
- **The Library.** Nineteen releases in the old manifest format stopped being served when the new manifest landed. Should they be republished, or should the loss be accepted?
- **Node A's machines.** Which two of the six should go, if any?
- **Packages from other publishers.** Should they install before the iframe sandbox exists? Two reviewers raised it, and current behaviour is kept.

## What I would do next

If I were holding the phone, I would watch three readings in the next hour: Keenable's first answers, the custody counters on d@e after the settlement fix, and Devin's verdict on provenance. Then I would fix the migration rule that renames models. Then I would turn on refusal for built-in desks, once you have answered the question about research and your stories.

The one thing worth remembering: the counters found real crossings the plan's inventory missed. And today's checks found three paper problems nobody was looking for. Running the real thing and reading what it counts has been worth more than any review.

## Names and receipts

Goal file: docs/definitions/choir-modular-architecture-2026-11-04.md. Plan: docs/design/modular-architecture-plan-2026-11-04.md. Ledger: docs/definitions/choir-modular-architecture-2026-11-04.ledger.md. Predictions: docs/definitions/choir-modular-architecture-2026-11-04.precommit.md.

Sign-off and goal executable: a209cb739. Phase 1 and 2 landings: e13c082a0 (MA1.1), 2bae9cbdd (MA1.2), 7826565c8 and 658a0c324 (MA1.3), 3ff42209e and 4b8cf26ad (MA1.4), 90a26080e and d3d6f9ee2 (MA1.5), 23c381ac8 and 69ff34c30 (MA1.6), f23c16f84 (inert count in calibration), d76608665 (MA1.7 doctrine), 1a5ca5437 (MA2.1), 2cd55c973 (MA2.2), de4f93b10 and e434f8bf1 (MA2.3), 24e3437e2 (MA2.4), cf2e7ed0f and 0359fff5e (MA2.5). gVisor evidence: 0d0936ad7.

Round strand: docs/problems/final-report-strands-unread-cast-2026-11-05.md; fix 020276dde and afeca9f2c; d@e trajectory 74335147.

Phase 3: 14ee014bd (MA3.1), 31d8bdc79 (MA3.2), 89db770e9 and 24a3ad5c2 (MA3.3), cd6ea493a and c9f63914d (MA3.4), 5a47ba515 (MA3.5), 947fd2078 and 26f2ed6df (MA3.6), 0d8014abc and 5e836949f (MA3.7a), 6ac5a8b27 and dbacd9d34 (MA3.7b), 25ecce33c and 8dda5426f (MA3.7c), fadd941ca (settlement reads as the store; d@e trajectory 2585dfb7). Guest security epoch 52.

Expedites, by name: 0987b876a, 97018316f, e385fd349, 897662321, afeca9f2c, 24a3ad5c2, c9f63914d, 1a5b8ef61, d290c08d7, 6855e5eca, ec8b4e90d, fadd941ca, d613108ed. The deploy script also auto-expedites any release touching desks or packages.

Models: d290c08d7 and 6855e5eca (pool), ec8b4e90d (management task on the pool). Paper policy rollback: ~/.config/choir-qa/traces/paper-model-policy-before-20261105.toml; paper key expires 2026-11-12.

Node A: 59 snapshots deleted, about 599 GB freed by garbage collection, six VMs kept (ws-owner, ws-01 to ws-04, agents).

Problems documented today: docs/problems/web-search-dead-most-of-the-day-2026-11-05.md (ca2db6776, Keenable section in d613108ed); docs/problems/vocab-infix-rewrite-mangles-model-ids-2026-11-05.md (7d279adba); docs/problems/model-provenance-unwired-2026-11-05.md (on branch model-provenance, commits cf6493a8d and c1d53514b, Devin round 2 pending).

In flight at writing: CI and deploy for fadd941ca and d613108ed; Keenable probe after the gateway restart; Devin round 2 on provenance.
