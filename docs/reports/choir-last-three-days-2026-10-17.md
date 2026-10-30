# Choir, the last three days

I wrote this on Saturday afternoon, October seventeenth. It covers about two and a half days of work on Choir, from Wednesday night, October fourteenth, to Saturday afternoon. All times are in UTC.

The paper published around the clock for three days, on one model. Most of what broke along the way is now fixed and measured, and the large decisions that are left are yours.

## Where things stand

Autopaper is live at choir dot news and is healthy right now. Every desk runs on gpt-6-luna, as you asked. The paper runs between twenty-five and ninety model runs an hour and publishes a few stories an hour. Opening an article takes about a fifth of a second, down from more than a second. Pages and story bundles now travel compressed, so a phone downloads between an eighth and a third of the bytes it used to.

This morning I found that Hacker News point counts were waking stories every fifteen minutes. That waste was more than half of all Texture work, and it is fixed. This afternoon a two-day soak test of the new database version passed. I then landed the long-parked work that lets the paper search all of its history instead of only the newest few hours.

Several decisions wait for you. The biggest is what counts as one "event" in the paper's world model. That one choice gates the next stage of the paper. The others are about storage growth, a signing key, backups and money. They are in the second-to-last section, each with options and consequences. After that come two chapters gathered by five Sonnet researchers while you rested. One holds ideas for doing more and better journalism with fewer tokens. The other zooms out on precommitment records and prospective self-alignment, the foundation you want under Autopaper, Autoputer and, later, Autoradio.

## Wednesday night: the paper's disk, and a night of self-development

The story starts late on Wednesday. At about 23:22, the paper's computer filled its thirty-two gigabyte disk and wedged. The cause was the guest's database journal, which had grown to twenty-seven gigabytes because its periodic cleanup never ran. I grew the disk, made the cleanup run online every five minutes, and recorded the incident. The lesson was simple. A cleanup job that never ticks fails silently, so the cleanup now runs on the open store and bounds the journal by the free space.

The same night, a dozen streams of work landed in parallel. The largest was self-development, Choir's ability to change its own code safely. A computer can now freeze a proposed change in a capsule. The host builds it from the frozen patch, and the change applies as a layered release on top of the running system. A standing consensus check votes on the host build before anything is accepted. Owner acts, such as accepting a change, now carry receipts that the proxy signs and corpusd verifies. The release record, publishing and adoption also landed, so one computer can publish a change and another can adopt it.

By Thursday these pieces had passed real drills on staging. The first full self-development loop passed seventeen of eighteen checks. The first passing adopt loop followed in the afternoon. By evening the Go-change drill passed eighteen of eighteen, twice.

Smaller things landed too. Account recovery now sends a magic link through Choir's own mail service. Memory hotplug landed behind a flag, for test computers only, so a computer can grow its memory for a big job and shrink back. The Go toolchain moved to 1.26.9. A malicious-PDF vulnerability in the frontend's PDF viewer was patched. Every third-party GitHub action is now pinned by commit. Keenable search is paced to its free monthly allowance, and Exa's searches stopped failing on oversized pages.

## Thursday: a host reboot, a quiet paper, and the cost of a token

On Thursday morning, Node B rebooted onto a new kernel. All the checks passed. But the paper then fell silent, because twenty-six of its twenty-eight source schedules had lost their wake-ups during the database outage around the reboot. I fixed the lost-wake path in three ways. A planned boot re-arms overdue schedules. A periodic sweep catches any that slip. And a failed delivery now leaves a visible line instead of vanishing. The paper also gained a liveness probe that alerts when it makes no model call for half an hour.

The platform database also stalled that morning. That stall hung checkpointd, crash-looped vmctl, and failed a deploy. I added guardrails: bounded timeouts on every store open, a retry instead of a crash in vmctl, and a liveness probe that queries both database servers every thirty seconds. I could not confirm the stall's root cause. The probe now captures a stack trace if it ever recurs.

In the afternoon, both database servers moved to Dolt version 2.4.2, after a backup and a compatibility check. That began the two-day soak that finished today.

Thursday was also about money. You asked me to halve the paper's compute while keeping the most important stories. The single biggest lever was the ChatGPT prompt cache. Sending the right session headers took the paper's cached share from about thirty-six percent to about eighty-eight percent within hours. Then I found that most remaining misses came from failover. The OpenCode account was out of funds and rate-limited, so about half of all calls failed over, and the failover cache keys split one conversation's cache into pieces. I keyed the cache by the serving model and removed a free model whose rate limits were parking the shared keys. The cache settled around seventy-eight percent, and the realistic ceiling is in the mid-eighties.

You then set the default pool to three parts gpt-6-luna to one part DeepSeek. Measured over hours, DeepSeek held about a quarter of conversations, as intended. But its conversations ran longer and made more calls, so it took about seven-eighths of the cost. That question became moot on Friday night, when you moved everything to luna.

On Thursday evening I drafted and reviewed the next station of the ledger work. This station records hypotheses and scores them later. A hypothesis can now say whether it is an assertion or a forecast, with a criterion, and a forecast with a resolution date. It deployed that night. I also built a read-only tool that walks a computer's event chain from the host. On the paper it walked fourteen thousand events in two seconds, and it showed that a capture arrived before the story that cites it. The second witness design is decided too, but it needs a signing key that only you can choose.

## Thursday night: your rulings, and the event layer

Late on Thursday you set the rules for the night. Capture is separate from production. A story needs two outlets to publish. Jev may run at ingestion and is pinned to its latest version. Telegram waits. Panels are Devin and Gemini only. Then you went to sleep and gave me the night.

The first catch was a dangerous one. The retention prune would have destroyed the paper's disk after a twenty-four-hour stop, because the paper's account is a test address that the prune treats as disposable. The prune now never touches a computer marked as protected.

The publish gate now counts outlets properly. A story publishes only when its citations span two distinct outlets. Aggregators, search pages and syndicators no longer count as outlets, and reader proxies unwrap to the real publisher.

Most of the night went into the event layer. This is a world model at ingestion. Jev reads every capture from every source and decides which real-world event it belongs to. Captures then accumulate on events, and an event with several independent outlets counts as corroborated. The offline study cost about a hundredth of a cent per capture. Version one went live on the paper in shadow at about one in the morning. Version two made corpus search return events first, with their outlets and corroboration. Version three, where corroborated events wake management to write stories, was built, reviewed and proven on a test computer, where four corroborated events became four stories in four minutes. On that test computer I also found that one event's records could open two stories, and fixed it.

The same night I fixed a Texture stall. Texture had been writing one confidence number per citation, the write was refused, and the next "done" closed the story at its seed for good. Done is now refused after a refused write, and the surface says plainly that confidence is one number per write. In the next hour the paper had no such refusals at all.

## Friday: a hard day for the paper

Friday was the roughest day in this window, and I want to be honest about it.

At seven in the morning, event layer version one failed its acceptance test. Three of forty-three sampled merges were false, against a bar of five percent. I raised the join threshold and reran on a fresh window that afternoon. It failed again, four of forty-two. Reading the failures showed why no threshold can fix it. Jev merges the developments of one ongoing story, such as several separate ship attacks near Hormuz, with near certainty. The judges, reading the same definition, treat an event as one occurrence. That is a definition question, not a tuning question, and it is yours. Version three stays off on the paper until it is settled.

At about half past ten I found that Texture opened stories blind. It was given a capture's identifier but had no way to read the capture. More than half of its opening runs went hunting for a reader, and one story stalled for good. I gave Texture the capture reader that its design always called for. In the next half hour, eleven of eleven opening runs read their capture, and none went hunting.

At eleven, Node B ran out of filesystem metadata space while it still had a hundred and thirty-five gigabytes free. About seven and a half million small artifact files had filled the metadata area. For twenty-five minutes, writes failed across the host. I recovered space online and added a daily guard that rebalances whenever unallocated space falls below thirty gigabytes. The guard works. But the files keep growing, and that growth is one of your decisions.

That one-minute storage failure had a long shadow. A management run's final state write failed, so the run looked "running" forever, and management swallowed every wake for about fifty minutes. I made the final write retry until it lands. That night I also made a wake replace any run that is stored as running but has no live loop behind it.

At ten to one, the paper went down for eighteen minutes. A deploy's planned restart ran a database migration that misread a headline containing the text "super" as an internal identity field, and refused to boot. I fixed that class twice that afternoon, first for capture labels, then for all prose and URLs. Then a deploy failed only because vmctl's reply timed out on a slow but successful boot, so I lengthened that timeout to ten minutes.

At five to seven in the evening, the guest kernel killed the paper's main process for running out of memory. The cause was a bug in yaegi, the Go interpreter that runs desk cells. A common Go loop shape never ends in yaegi, so two string-scanning cells grew without bound at the same time. I rewrite that loop shape before running it. Desk workers now volunteer to be killed before the main process. A worker that dies tells the model why. Since then, no worker has died and no cell has run away. Later I also found that a cell's timeout started before the host had prepared its inputs, so one cell was killed after three milliseconds. That is fixed too.

## Friday night: your three requests

On Friday night you sent three requests from your phone.

First, the source sections. You said they were bad, ugly and senseless, because they republished other outlets' text instead of citing it. You were right, and you never asked for them. I retired the embed feature completely. A story now cites passages and never reprints a source's text as a block. Old stories show a numbered marker where an embed used to be.

Second, OpenCode use. You asked for one hundred percent gpt-6-luna. Every desk now runs on luna, and DeepSeek remains only as an emergency failover. I verified it within half an hour: every run was on luna. It has stayed that way for every hourly check since.

Third, slow article opens, and resource use in general. Node B was not under memory pressure. The cost was in the database. Every article open scanned every check row the paper had ever written, and the reader re-downloaded the whole story every minute. After the fix, a story resolves in about a fifth of a second instead of more than a second, and the minute poll asks only for the navigation. While measuring, I found that the edge compressed nothing. A four-hundred-sixty-seven kilobyte story bundle now travels as fifty-five kilobytes, and the main script as ninety-one kilobytes instead of two hundred seventy. That change briefly stalled live event streams on Node B's web server version. I caught it within ten minutes, reproduced it on the host, and fixed it so that streams are never compressed.

The same evening I fixed a restart bug. Every planned restart for a deploy dropped the Texture turns that were in flight. A test reproduced it six times out of six, and a restart-parked Texture turn now resumes through the Texture controller. I also measured storage growth. The host gains about seven hundred thirty thousand small files every twelve hours, half of them signed pin receipts, one receipt per pinned artifact.

## Saturday: a quiet night, one large waste found, and a database milestone

Overnight I checked the paper every hour. Every check was clean: all runs completed, all on luna, stories every fifteen to thirty minutes, and no errors in the logs.

At ten to nine this morning, one story caught my eye. Its title was about its own empty source, and its revision count kept climbing, from four to nine to fifteen. The trace showed why. Hacker News feeds include each item's live point and comment counts. Every fifteen-minute poll saw new numbers, saved a brand-new capture, and woke the story about it, and Texture dutifully updated the count. Five stories were doing this. Together they took fifty-eight percent of all Texture runs over two hours. One of them, "Nine-Inch Snails", had twenty-five published versions. The fix strips those two counters when the feed is read, and only for Hacker News text. Since then, no story has been woken on that fifteen-minute beat.

I also measured the stories opened on link-only captures, the ones titled about their own empty source. They are about two percent of openings, and the two-outlet gate keeps every one of them off the front page. A similar but milder case showed up in the afternoon. Deutsche Welle kept lightly editing its own summary of the Pope's discharge, so the same item was captured three times. Because the publisher's text really changed, I recorded it and built nothing.

At a quarter to four this afternoon, the Dolt soak ended cleanly. There were fifty-six hundred probes over two days and not one failure. That cleared the way for the Store B retrieval work, which had waited three days for it. Until now, the paper's corpus search read only its newest thousand captures, about five hours of history. So a story could not cite a capture from yesterday, even though it was saved. I rebased the work onto current code and tested its migration against the exact database version Node B runs. Then I landed it. On Node B the migration took fifty-one seconds. Search now covers all twelve thousand of the paper's captures. A capture from yesterday is found by its address, and another owner's search finds nothing. If anything goes wrong, a switch file on the host turns the new search off without a deploy.

## Ideas for more and better journalism with fewer tokens

Four Sonnet researchers each took one area: tokens and caching, article quality and sourcing, storage and the object graph, and the read path. Each worked from the code and documents only, with no access to live data. I checked the most important claim in each area against the code, and the ones I mention as facts held up. Their savings estimates are educated guesses, partly built on measurements from the DeepSeek period, so the first step in every area is to measure again on luna.

### Where the tokens go

Before anything else, the token researcher suggests one hour of homework: find out what the ChatGPT weekly limit actually counts, and in particular whether cached tokens count against it. The answer decides how much several of the ideas below are worth.

The largest single consumer of input has been the research desk. In the last full measurement it took about half of all input tokens. The cause is structure, not verbosity. A research run makes many calls, and every call replays the results of all the calls before it, so a long run costs roughly the square of its length. The research prompts tell research to work one step per cell, to work in rounds and to keep going, and they give it no budget. The proposed fix is mostly prompt text. Search and read in one cell. Print only titles, addresses and short snippets. Stop after about six cells unless a central claim is still unsupported. Late in a long run, add one gentle line saying to send the final packet soon. The estimated saving is a tenth to a sixth of all input.

The second lever is waking. Research is told to send Texture everything after each round and to send promising leads at once, and every one of those packets wakes Texture. A ninety-second hold on interim packets, with an immediate wake for a final packet, would remove whole model calls. The estimate is another five to nine percent.

The third lever is something I confirmed in the code. When Texture's frame is built, the system fetches every source capture's text, notes that it exists, and throws the text away. Then the prompt tells Texture to read the capture, which costs a call. Putting the first few hundred characters of each source into the frame is nearly free and saves about one call in three for each new story.

There are smaller levers too. A Texture run that wakes after a long idle could be compacted, since its old cache has expired anyway. Texture and management could be told plainly that a record adding no new fact, outlet or correction owes no revision. Refusal messages could show the nearest matching passage, so the model fixes a bad quote in one try. You may also want to revisit Thursday's ruling against trimming search results and eliding old tool results. That ruling protected the cache ratio for the DeepSeek pool. Now that everything runs on luna's weekly quota, the trade may have changed. The switches already exist.

### Better-sourced stories

The quality researcher found a real hole in your two-outlet rule, and I confirmed it. The publish gate counts outlets by each cited capture's web address. For a Hacker News item, that address is the linked site itself, even when the capture holds nothing but the link. So a link-only Hacker News capture can supply the second outlet. The event layer counts it the same way. The fix is to carry a "thin" mark on every capture from ingestion onward, stop counting link-only captures as outlets, and stop announcing them to management at all. That also removes the stories titled about their own empty source.

The same researcher found the root of thin stories in general. One hundred and four of the paper's hundred and forty-three RSS sources are marked excerpt-only, including the BBC, Al Jazeera, the Guardian, Deutsche Welle and NPR. The paper is not allowed to fetch the full article for them, so Texture starts from a one-sentence summary and must send research to find depth. For two Hacker News feeds, simply allowing the linked article to be fetched would give real text instead of a link. For the big outlets, it is a policy question about terms of service, and it belongs in your sources work.

Other ideas in this area cost few tokens and protect readers. Score a new event's importance before spending research on it, and spend more on what matters. Tell Texture that a story opened with two outlets already attached should be written as a real story in its first revision, not as a plan. Run a stricter publish check in shadow first, holding headlines that talk about their own sources and bodies under about eighty words. Then enforce it once the shadow results look right. Treat a recapture that differs only by a light copy edit as the same capture. And once the event definition is settled, let corroborated events open stories directly, with sources attached from the start.

### Storage and the object graph

The storage researcher found that the small-file growth is not mainly captures or published stories. It comes from the event tape. Every write the paper makes to its own state becomes one tape event. Each event costs four files on the host: the event, its payload, and two signed receipts. Published stories are well under one percent of the growth.

So the main levers are on the tape. The first is batching. Writers already queue for the tape one at a time, so merging the writes that are waiting into one event would cut events two to four times. The second is packing. Store receipts, and later all small artifacts, in large sealed segment files with an index, instead of one file each. That would turn one and a half million files a day into about a hundred. It changes the evidence store, so it needs your approval, and it raises its own questions about the signing key and off-host backup. The third is that the guest's database commits every hundred events and on some reads, and much of its history is never read.

Two cheaper items stand out. The platform database commits every forty-five seconds, and nothing reads that history. A longer interval plus the squash you already ruled for would stop most of its growth. Separately, nothing prevents publishing a version identical to the last one, and nothing spaces versions out. A small guard there would have turned twenty-five versions of one story into a handful.

### Opening and reading articles

The read-path researcher counted thirty to fifty database queries to open one story. Most of them fetch parts of a published version that can never change. An in-memory cache in corpusd, keyed by version, would cut that to about six, and probably take the open from about a fifth of a second to a few hundredths. The front page is worse. A first visit asks for sixty stories at about four queries each, around two hundred and forty queries. A per-story cache plus a few seconds of page cache would make a warm front page one query.

Neither the story bundle nor the front list sends caching headers today, so a reader who goes back and forth downloads everything again. The proxy also reads the story bundle through a one-megabyte limit. I confirmed that a story bundle larger than that would fail with an error. Today the largest is under half a megabyte, but the bundle repeats its content several times, so a long story could reach it. Passing the bytes straight through fixes both the waste and the hidden failure. Smaller ideas include starting the story fetch before the editor code loads, skipping the signed-in desktop's start-up work for public readers, and sending a slimmer bundle to the reader.

### What I would do before the sources work

If you want something done in advance of the sources work, here is a short batch that is low in risk and needs no big decision from you. First, measure: what the weekly limit counts, and where luna's tokens go today. Second, close the link-only outlet hole and mark thin captures. Third, take the research prompt changes, the "no revision owed" rule and the source excerpts in Texture's frame, in one prompt edit so the cache cold-starts only once. Fourth, add the duplicate-version guard and the longer platform-database commit interval. Fifth, add the story and front-page caches with proper caching headers and the proxy passthrough.

Some things should wait for the sources work: letting events open stories directly, the importance thresholds, and each new source's body policy. All three depend on how many outlets overlap once the new sources arrive.

## Zooming out: precommitment records and prospective self-alignment

You asked me to zoom out on precommitment records and prospective self-alignment. A precommitment record is a prediction written down before the evidence arrives, then settled against what actually happened and scored. Prospective self-alignment is the research program built on those records. On October fourteenth you said its key value is in-context reinforcement learning. The records live in each desk's interpreter, with context packs and retrieval, so a model learns from feedback on its own predictions without any change to its weights. The working hypothesis is that learning from feedback on one's own outputs is far more token-efficient than learning from examples. So this is the same question as the rest of this letter, asked one level deeper: how does the paper get better per token over time?

One Sonnet researcher read the design document and the code, and I checked its main claims against the code myself.

Here is the honest picture, measured against that core claim. The record and its ledger are real and well built. Every story the paper opens carries hypotheses, and since Thursday a hypothesis can say whether it is an assertion or a forecast. Engineering's rule-settled promises close a true loop: the runtime settles them from receipts, with no model's opinion involved. Texture's citations close a second loop, because Jev judges whether each source supports its claim.

But the paper's main loop stops early. Management's prompt defines a hypothesis's probability as the chance that our own research desk will confirm it. So almost every hypothesis is a verification claim. Thirty-two of the thirty-eight sampled were assertions of that kind, one was a dated forecast, and all eight opened since Thursday were assertions. Those predictions measure our research desk more than the world. Settlement is done by the counterpart desk, the one that supplied the evidence, so it is not independent. The calibration feedback that would show a desk its own record exists in code, but it is switched off by default. Nothing settled feeds back into any live decision yet. And nothing retrieves past records as memory. In other words, the in-context learning loop that prospective self-alignment is about is not yet closed anywhere on the paper. The records are written, but no desk ever learns from them.

The research found that this matters more than any scoring detail. Whatever the claims are about is what everything downstream learns. That includes source reliability, memory, and later Autoradio. If the claims are about our own desks, the whole structure learns about our own desks.

On your ideas for deepening it, the research came back like this. Stories as resolvable claims and forecasts: yes, starting with scheduled, checkable events such as data releases, earnings dates and votes, inside the seven-day horizon. Source and outlet reliability: yes, but only after a claims layer exists, and as a view with counts, never one number. Event-layer corroboration as evidence: yes, with one hard limit. We must prove the corroborating capture arrived after the prediction, which needs the second witness and its signing key. Precommitting the value of a turn before spending tokens: the idea is right, because tokens are the real constraint. But a free-form "value" has no ground truth. The better start is a small rule-settled claim, for example that a research cast will answer its question within a stated number of tokens. Settled records as memory: the weakest piece today, and best started plainly, with "your last few confident misses on this topic".

Why this is the foundation for Autoradio: the repo's design for Autoradio's programming engine already plans to commit a prediction for every segment it plays. Will the listener finish it, skip it, or act on it? Each is settled by rule from the listening log. That is the same loop. It needs the same distinction between an interrupted prediction and a wrong one, and the same discipline about what a desk sees of its own record. If the loop is shallow, Autoradio inherits a recommender with a ledger-shaped label. If the loop is deep, Autoradio inherits something that corrects itself. So your instinct holds: deepen this first, then Autoradio is mostly a new surface on a proven loop.

The research proposed a calm path of five small stations, none of them rushed. First, read the ledger honestly for a few days, with no code: how many hypotheses carry a kind, how many are forecasts, and who settles them. Second, teach management to write real forecasts on scheduled, checkable events, kept separate from verification claims. Third, close one independent loop on the live paper. A rule or Jev settles forecasts, the settlement is its own record with evidence, and it is never written by the desk that opened the prediction. Fourth, join each record to the cost of the turn that made it, so the paper can learn whether its expectations about cost and coverage are calibrated. Fifth, give desks a way to retrieve their own past misses. That fifth station is where the core claim of prospective self-alignment gets its first real test: does memory built from settled records beat no memory, at a controlled cost? Autoradio fits after the third station at the earliest, which matches your sense of about a week, with no rush.

## Open questions, possible answers, and what follows

These are the decisions I could not take for you, in rough order of how much they matter.

### What counts as one event

The event layer decides which captures describe the same real-world event. Jev groups the developments of one ongoing story together. Your judges, and most readers, think of an event as one occurrence. This choice gates version three, where corroborated events start stories on their own.

The first option is to tighten Jev's definition to one incident, place and time, with later developments as new events linked to the first. The paper would then get crisper corroboration and fewer false merges. It would also get more, smaller events, and possibly more stories about one ongoing situation. The second option is to accept story-level events, but require a stricter second check before two captures count as corroborating the same occurrence. That keeps the natural story unit, at the cost of an extra model call for each corroboration. The third option is to count corroboration only among captures that arrive close together in time, since later developments arrive later. That is cheap, but blunt.

If I were choosing, I would take the first option, because it matches how a newsroom thinks and how the judges grade. I would then rerun the acceptance test on a fresh window. Until you choose, the paper keeps working as it does today, with the event layer in shadow.

### Storage growth on Node B

The host gains about one and a half million small files a day. Half are signed receipts, one per pinned artifact. The daily guard keeps the filesystem healthy, but growth without bound is a design choice, not a plan. The options are to keep everything and rely on the guard, to pack many receipts into one file per published version, to publish fewer versions by waiting for a story to settle, or to set a retention policy for old artifacts. Packing and fewer versions keep the full audit trail. Retention is cheaper, but it deletes evidence. This touches the evidence store, so it needs your ruling. The storage researcher found that the files come from the event tape, four files per event, which makes batching and packing the real options. Two smaller decisions sit beside it. About five hundred seventy-five thousand old body files have been unreferenced since October ninth and could be deleted. And you could decide whether run telemetry must stay replayable event by event, or may be sealed in batches.

### The signing key for the second witness

The ledger work needs a host-signed record of when a capture first entered the shared store. The choice is between reusing the existing platform-control key under a separate statement domain, or minting a new key just for capture witnesses. Reusing the key is less to manage. A new key limits the damage if either key leaks. I lean toward a new key, but it is your security boundary.

### An off-host backup target

The next ledger stage waits for an off-host copy of each computer's ledger. That needs a place outside Node B that you trust, and a decision about cost. Until then, a total loss of Node B would lose history that exists nowhere else.

### The weekly squash of the platform database

You ruled that the platform database's history is squashed weekly, with a backup first. The backup exists and the soak has passed. But a squash rewrites production history, and our standing rule is no hand-written SQL against production databases. I have not run it. I would rather build it as a small, tested maintenance command, or have you approve a one-time supervised run. Either is fine. I need to know which you prefer.

### Two rollback disk images

The paper's directory still holds two rollback images from Wednesday night's disk work. Deleting them frees about twenty gigabytes. The paper has run on its new disk for more than two days without trouble. I have not deleted them, because deletion is irreversible.

### Money and accounts

The local OpenCode account that some review-panel models use is out of funds, so those panelists fail. Paid search providers are also out of credit, and the paper runs on Keenable's free tier, as you ruled. Whether to top up is your call. Neither blocks the paper today.

### The yaegi bug report

The loop bug that killed the paper on Friday belongs upstream in yaegi. Filing it under your name is your choice. Our workaround holds either way.

### Prospective self-alignment

Four questions from the zoom-out are yours. First, what should a story's hypothesis be? Keeping verification claims protects throughput, but it teaches the paper about its own desks. Requiring at least one forecast per story gives a real signal about the world, but it risks padding, or refusals on a paper that runs all day. Making forecasts a separate act is cleaner, at the cost of one more surface. Second, who settles a prediction first: rules only, which is independent but limited to scheduled facts; Jev, which is cheap but literal; or a second model. Third, may a desk see its own settled record, and in what form? That gates the in-context learning experiment at the heart of the program. Fourth, is a small token-budget learner in scope now, or after the first independent loop is closed? I would close one independent loop first.

### Search trimming, now that everything runs on luna

On Thursday you ruled against trimming search results and eliding old tool results, to protect the cache ratio. Since Friday every desk runs on luna, whose weekly quota is the limit. The token researcher estimates the switches, which already exist, would cut a few percent to a sixth of input, depending on what the quota counts. Whether to revisit is your call, and the measurement of what the quota counts should come first.

### Things that wait for evidence, not for you

One item needs no decision, because it enforces your own rule: the link-only outlet hole in the two-outlet gate. I will document it and fix it next, unless you say otherwise.

The restart fix has not met a live case yet. Every planned restart since it landed happened with no Texture turn in flight. The narrower database fence has not met a replaying boot. The forecast kind for hypotheses is not yet required, by design, until most new hypotheses carry it. A per-worker memory budget stays defensive, because nothing has failed since the loop fix. And the embedded database library still needs its version bump, canaried on one computer before the paper.

## The one thing to remember

Most of the paper's cost is not writing; it is waking up and re-reading. Fifty-eight percent of Texture's work on Saturday morning came from one feed's changing numbers. Research replays its own history on every call. The cheapest tokens are the ones the paper never spends. And the deepest saving is the one prospective self-alignment promises: a paper that learns, from its own settled predictions, when a turn is worth taking.

You said you want to zoom out before zooming back in, and I think that is right. Autoradio can wait without losing anything. If I were holding the phone, I would do four things next, none of them in a hurry. Settle the event definition. Take the small, safe batch from the optimization chapter. Run the honest ledger reading, the first station of the zoom-out, which costs nothing but attention. And do a coverage-gap analysis of the paper's sources before the sources work begins, so new sources fill real gaps. Rest well. The paper is fine.

## Names and receipts

Goal file: docs/archive/definitions/choir-ledger-metamission-2026-10-13.md. Running handoff: docs/archive/reports/handoff-2026-10-15.md (also in iCloud Choir Reports). Owner rulings: docs/reports/owner-notes-2026-10-14.md sections 20 to 25.

Staging at writing: 4ed852b67 (deployed 2026-10-17 16:04 UTC). Paper computer: vm-1669020adcece51c2f5ff9b6df399ef5.

Wednesday night and Thursday: paper disk full, docs/archive/problems/guest-dolt-storage-growth-2026-10-15.md, online guest GC 4bf25d82a. Self-development: P5 host build f20ba7f5d, P4 standing consensus 97e023100, S1 owner-act receipts 9c62b209f, P6 to P8 release and adopt ff01e0a2f, first passing adopt loop adopt-1523, go-served-1825 and go-served-1920 at 18 of 18. Account recovery 13380efc7. Memory hotplug merge ce959b2db. Patch lanes 01d8c5981. Lost-wake fix 0b1571de2. Dolt guardrails 7b8944f03. Dolt 2.4.2 servers 5c83b91f8. Cache routing headers 73b14c327, cache-key logging 8f2f0dc5d, failover cache keys d2b4f0939, cache report docs/archive/reports/chatgpt-cache-ratio-2026-10-15.md. Pool 7b4585cfc, management pin 77633ecdc. L3-1 0fe4cead0, ledger witness 4592c7b63, witness (b) design docs/problems/capture-first-creation-witness-missing-2026-10-15.md (deleted 2026-10-30, in git history).

Thursday night: retention prune fix 7b9679082, Jev pin da8123d0b, two-outlet gate d54242fc9, outlet key afd160449, event layer V1 84344ffa8 and 96814cded, V2 9eb2c037f, V3a ea6a90dd1, V3b 5d1a4192d, one-event-one-task f81e729c5, Texture stall 1e13ab348 and 7869cdef3. Design: docs/reports/event-layer-at-ingestion-design-2026-10-16.md.

Friday: V1 failures docs/problems/event-layer-false-merges-at-marginal-p-2026-10-16.md, threshold 78ae91887. Texture capture reader bc67aee10. btrfs metadata docs/problems/node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md, guard 14308b42e and a2276e78c. Run end retry c8e1dc259. Boot refusal 3d669e89b, prose leaves b8feaa147, vmctl timeout 173d02b68, vocabulary fence b8ff3481f. OOM loop fix 3046fbb9a, worker containment f93bbfd84, cell timeout 57c4d0b9b.

Friday night: embeds retired 3143ade5b, luna-only 269c1e11a, article open 575cb3ef5, 3b345f87c and e8379a0ca, edge compression 869420ad3 and 1a0eef8de, restart resume e3ffabbca, dead-loop wake eb0247c83.

Saturday: HN counters 67201a94c, docs/problems/hn-point-counts-mint-a-capture-every-poll-2026-10-17.md. Link-only measurement docs/problems/texture-opens-stories-blind-to-their-capture-2026-10-16.md. Store B retrieval 4ed852b67 (CI 39047149193), docs/archive/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md. Rollback switch: CORPUSD_CAPTURE_SEARCH=off in /var/lib/go-choir/corpusd-retrieval.env, then restart go-choir-corpusd.

Research findings checked in code (Saturday afternoon). Link-only outlet hole: internal/agentcore/story_publish.go line 261 and internal/agentcore/event_layer.go lines 548 and 668 key outlets by the capture URL. Excerpt-only sources: 104 entries in internal/sources/registry.json. Frame fetches capture blocks and discards them: internal/agentcore/texture_frame.go lines 148 to 154. Research prompt rounds: internal/promptstore/defaults/research.yaml lines 27 to 30. Proxy 1 MiB read limit: internal/proxy/platform_public.go lines 315 and 378. No Cache-Control on resolve or front; reader front fetch uses no-store: frontend/src/lib/autopaper.ts line 134. Store B pool of five: internal/platform/store.go line 296. Platform commit debounce 45 s: internal/doltbatch/committer.go line 27, unset in nix/node-b.nix. Calibration feedback off by default: internal/store/computer_setting.go line 44. Management hypothesis probability defined as research confirmation: internal/runtimeprompts/overlays/rlm_management_runtime.yaml line 23. PSA core claim and hypotheses: docs/precommitment-records.md sections 1.2, 8.2 and 8.4. Compute baseline: docs/archive/reports/autopaper-compute-halving-2026-10-15.md.
