# Fourteen hours on Choir, November 9

I wrote this on the afternoon of Monday, November ninth. It covers about fourteen hours of work, from just before midnight on Sunday to about two in the afternoon, by me and the builder and reviewer agents I ran.

The ordinary prompt path mission is finished. The paper now has spending caps and remembers which providers are down. And we learned that about half of the paper's tokens were going to one stuck loop.

## Where things stand

The ordinary prompt path is complete. A prompt now reaches a cited second draft in about seventy-eight seconds, down from about two hundred. The last open item, pausing a live sandbox across a restart, passed its drill this morning.

The live station is now Autopaper improvement. Today's release on staging changes the paper in three ways. It routes to free models first. It caps paid spending at twenty dollars a day on Bedrock and one dollar a day on River. And it stops retrying providers that are down, with that memory surviving restarts.

Two problems remain that you should know about.

First, Haiku 5.5 is blocked on our AWS account. The agreement is accepted, but Bedrock refuses the model even with the account's own root credentials, and points to AWS Sales. So the paper has no working paid fallback right now.

Second, the paper's management desk has been wedged since about November third. It burns roughly half the paper's tokens and commits nothing. Two fixes for it are built and in their second round of review.

## Overnight: the desks learn new tricks

Just before midnight and into the small hours, I landed a run of features for the ordinary prompt path.

The Texture desk can now embed an image, a file card that opens in Reader, and YouTube videos and podcast episodes that play inside the document. Engineering assignments now put their deliverables into Files, under Reports. Devin's review of that took three rounds, because a capsule could have written text that looked like a real line of the summary. The final version sends every line through one escaping step.

Two fixes went in for capsule start times. A capsule no longer copies the computer's whole store when it starts. Instead it shares one content-addressed copy of the source. This came out of a finding that three capsules had timed out on d@e, and the cause was the store snapshot under memory pressure, not the launcher.

Theme packs that contain only data, with no code, now install and show up in Settings. That is what let the theme rounds on b@c work at all.

A few smaller changes came with these. A run that repeats the same call with the same result is told once, then stopped with a visible reason. Research now shows the desk every page it reads, and its first capture triggers an interim report. Idle tool layers now give their memory back.

## Early morning: self-development closes its loops

Between about three and six, I fixed several places where self-development stalled.

A package install now records the owner's apply act, so Texture can close a theme round whose install actually landed. Research's data now reaches engineering as read-only input files, instead of being pasted into the prompt. A model policy override now follows its trajectory, so research, management and engineering all resolve through it. And a settled trajectory now closes its undecided candidates. Before, one orphaned frozen candidate had been holding up every deploy refresh.

## Morning: three features you asked for

From about six to ten, I designed and built the three features you added to the mission. Each design went through three review rounds before any code.

The first is an account switcher. You can now be signed into several accounts in one browser and move between them. Its deployed acceptance test passed on choir.news.

The second is "Restart to update". When a new release is ready, your computer shows a chip, and the restart goes through the same planned path a deploy uses.

The third is sandbox pause. A live engineering sandbox can be paused across a planned restart and resumed afterwards, without losing its work. This took the most review, mostly over races at the moment of resume.

At ten I ran the mission's conjecture test on d@e. The prompt reached a cited second draft in seventy-eight seconds, against two hundred and two at the start of the mission. I wrote the checkpoint letter then.

## Late morning: the mission closes and the paper work begins

Just after eleven the sandbox pause drill passed on d@e, and I marked the ordinary prompt path complete. I set Autopaper improvement as the live station, and Devin reviewed its goal file twice before you made it executable.

The first two Autopaper problems were the queue and the quotas. The paper's management queue held about eighteen hundred tasks, the oldest seventy hours old. Now a queued task older than a day closes as expired, with a visible reason, and the backlog clears in batches. Separately, when every model was out of quota, the paper's runs failed together. Now a quota wait is bounded by time and waits for the earliest provider reset.

## Midday: models and money

Around noon you noticed the Synthetic DeepSeek allowance had run out. I traced it. The paper had made more than eight thousand Synthetic calls today, about fifty-two of the fifty-seven dollars at list price. That came from the routing change on November fourth, which put Synthetic in the paper's pool. I took Synthetic out of the paper's pool.

Then, following your rules, I rebuilt the routing. Everything tries free models first. The paper's paid fallback is Haiku 5.5. Self-development falls back to Synthetic and then River. Everything runs at low reasoning, except the Sonnet engineering delegate at medium. I added River as a provider and put its key on Node B. I minted a new Bedrock key with the AWS command line tool and accepted the Haiku 5.5 agreement. I also found that Bedrock had never worked on Node B at all, because it had no region set.

You asked that the system stop hammering providers that are down. I wrote that rule into the doctrine. A builder then audited every retry path and gave them memory that survives restarts. Devin caught a real risk in that work: one computer's bad requests could have shut a shared model for everyone for six hours. I narrowed the rule before it landed.

You set budgets: twenty dollars a day for the paper and one dollar a day for self-development. The gateway now counts every provider's spend per day and refuses Bedrock or River past its cap, so the paper moves on to the next model. You also get a spend notification every hour.

All of this deployed at about two o'clock, after a GitHub runner failure that a rerun cleared.

## Early afternoon: where the tokens go

You asked whether the paper could do the same work with a fraction of the tokens. A Sonnet investigator measured it.

The paper reads about fifty million input tokens an hour, and about half of that does nothing. The management desk's sandbox data had grown past its four megabyte limit, so every attempt to read the inbox failed. Each time the model ended its turn, the loop handed it the same update again. The model acknowledged it, and the cycle repeated until every provider ran out of quota. Research takes about thirty percent, Texture writing thirteen and intake seven. Output tokens are only about one percent of the total.

Two builders made fixes, and both came back from review with real defects, which is what review is for. The loop fix had a case where the desk could think work was waiting, find nothing, and fail instead of finishing cleanly. The data fix windowed the desk's backlog, but two paths would still have marked updates as handled even though the desk never saw them. On the paper's backlog of about a thousand updates, that would have quietly lost most of them. Both builders are working on the second round.

## What I would remember

The lesson from today is that the paper's cost tracked a defect, not its output. It published about thirty-two stories an hour whether it spent sixteen million tokens or fifty. Fixing the stuck desk should halve the spend with no change in quality. Trimming research and turning on prompt caching could take that to about a tenth. Getting to two percent would need a redesign, to a few model calls per published story.

If I were holding the phone, I would do two things before your demos. Decide what to do about Haiku 5.5: ask AWS to enable it, accept Haiku 4.5 at ten times the price, or stay free only for now. And watch the hourly spend notes, because crouton's free week ends around tomorrow.

## Names and receipts

Goal files: docs/definitions/choir-ordinary-prompt-path-2026-11-09.md (complete) and docs/definitions/choir-autopaper-improvement-2026-11-09.md (live), with its ledger alongside.

Evidence: docs/problems/paper-quota-incidents-2026-11-09.md, docs/evidence/provider-retry-audit-2026-11-09.md, docs/evidence/paper-token-budget-2026-11-09.md.

Overnight: e3dab98a9 and e8bd4e9df (deliverables to Files), 342928d6e and 2032d2c4f (Texture embeds), 8719bc698 (capsule spawn without a store copy), 0b78940cf (data-only theme packs), 259c83696 (repeated-call stop), 8d53b58b5 (research reads), 970f7d975 (tool layer eviction).

Early morning: 5b5c07a80, 5b6450a1d, 867c5c5c5, c0af6603a, 3b2aa7eef.

Morning: 9f3faa7c0 and c471ae5dd (account switcher), b180565b8 and 38a829aa0 (restart to update), c6e1af593, 23b3dac7a, eaf66a1aa, b5191c789 (sandbox pause), 5426cdf4e (conjecture test), 6d4c2b06a (checkpoint letter).

Late morning: 043db0977 (mission complete), 0ea07ed44 and f6f313d51 (queue expiry), 36534be53 (bounded quota incident), a0b6fb621 (pause test flake).

Midday: 217a78a52, 52d93ddb3, 5ca86505a, afbf2c8f9, 45844274b, 7334daf86. Deployed build 7334daf86 on staging.

Afternoon: 46e8783e3 (token evidence). In review: the final-checkpoint loop fix and the frame windowing fix, local only.

Computers: d@e is computer-59c902c5d46caecc0a180a8557164e72; the paper is computer-1355b83bced1613eb048ee28d66688a8; b@c is computer-6c50d6e566143ae32cdb660faa60e24d.
