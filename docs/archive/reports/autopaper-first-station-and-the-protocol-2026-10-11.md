# Autopaper's first station closes, and the protocol comes first 2026-10-11

Written Sunday, October eleventh, around four in the afternoon UTC. It covers about nine hours of work since the early Sunday letter: the research prompt, ChatGPT search, Autopaper's first station, and your turn toward precommitment records. The early letter still tells the night's story.

The first Autopaper station is complete, and the next one waits on the protocol you asked for this afternoon.

## Where things stand

A paper computer now polls news sources on its own schedule into the shared store. Each poll runs without a model. New items become shared captures that any computer can read, with their text on the host's disk and nothing large in a database row. A crash stops the polling instead of looping it. Three rounds of review found real holes, and all of them are fixed and deployed. The third round approved.

You then asked that all communication between agents go through precommitment records. I agree, and the traces back you up. I wrote a problem record and a design draft for your review. The second station, living Textures, is paused until that protocol lands.

The research brief covers about twice the ground it did this morning, but with fewer citations, and still a fraction of the baseline. Two search providers are out of paid quota, and restoring them is yours to do.

## Research: the bottleneck moved

I ran the AI news demo several more times today, reading the traces each time. The morning's fixes made errors visible, kept citations through full rewrites, and stopped search waves from waiting on the slow ChatGPT provider.

The run after lunch confirmed Claude Haiku 5.5 from Anthropic's own page, and the stock caveats were gone. But the brief was still short, and GPT-6 had dropped out. The traces showed why. After the first round, Texture spent every research round asking research to confirm things it already had. Research reported the Haiku claim six times. Texture's prompt told it to send every lead back for confirmation, and research rounds are limited, so confirmation crowded out coverage. I changed both prompts so a round covers what the document lacks before it confirms what it has. The next run did what the change asked. Texture's second request told research to open the four areas the brief didn't cover yet. The brief roughly doubled its ground: model releases, a city council hearing, a House bill, a lawsuit, chips and power, three funding rounds, and three new papers. But citations fell from ten to four. Most items name their outlet without a link the reader can follow. I suspect research reported what it saw in search results without importing the pages, though I haven't confirmed that in the trace.

The deeper lesson is the one you named. I was tuning prose to fix a missing measurement. If each research question carried a frozen prediction, those wasted rounds would have scored as no surprise, and we would have a number instead of my reading of traces.

## Autopaper's first station, and three reviews

By lunchtime every acceptance check had passed on staging. The hour-long run wrote sixty-nine captures, matching the sixty-nine new items exactly. A second computer, owned by a different user, read a capture and its text.

Then I ran the completion review: a panel of independent model reviewers reading the code. The first round sent it back. A computer could slip a capture past the capture route through the generic object route. A failed write could lose a feed's items for good. And a failed commit could drop the next scheduled poll. One reviewer also showed that my earlier diagnosis was wrong. Computers talk to the shared store directly, not through the proxy as I had claimed. A setting on the guest's boot line takes precedence over the one I had read. I corrected the record.

The second round found one more hole, and three reviewers found it independently. Two computers of the same owner posting the same item at the same moment could overwrite each other's capture. One reviewer reproduced it. I wrote a test that fails on the old code and passes on the new, then made capture creation insert-only. The third round approved, seven of eight, with none sending it back. The eighth reviewer stalled on a permission prompt both times.

## Your turn toward the protocol

I had drafted the second station, living Textures, and sent it to the same kind of review. It came back five to two against. Management has no way to hand evidence to a Texture outside its own conversation. It can write a Texture's content as you, through the product routes, which breaks the rule that only the Texture desk writes. And a prediction can't be resolved without a model call. My draft was inventing yet another side channel to get around all of that.

Your message arrived at the same moment, and I agree with it. I counted every hand-off in two days of staging traces. Texture opened or directed other desks two hundred and eighty-three times, and every one of those went around the record ledger. Only the answers came back as records. Not one agent ever made or resolved a prediction. The ledger exists, but today it is a one-way report channel.

The design draft proposes one rule. One agent reaches another only through a record addressed to it. Any request for work carries a small typed prediction, frozen when it commits, and the answer resolves it, by counting wherever counting can. The old side channels are deleted, not wrapped. It lands in five slices. The first changes no surface: Texture's existing controls start writing records underneath. The last rebuilds the second Autopaper station on top.

Then you added the larger frame. The same records will carry Autopaper's claims and inferences, other people's editions will score them from their own views, and the scores add up to track records that let computers earn reputation in an economy of agents. That changes a few things that have to be right from the first slice. A score becomes its own record, written by a named scorer, instead of a field inside the claim. That way anyone can score anything, and no house view is built in. Every record names who emitted it: an agent, an edition, or an outside source like a news outlet, whose claims build its own track record. Every record carries what it is about, so a scope like "strong on chips, weak on courts" can be computed later instead of fixed in a taxonomy now. And commit time has to be provable, because a track record is worthless if a prediction could be backdated. Signing and the commit-and-reveal scheme for private editions come later. The fields that make them possible go in now.

Eight decisions in the draft are yours. The first five are about the desks: whether every request carries a prediction, which scorer is the default, what happens to the assignment verb, whether your own revisions become records, and where the block on management writing Textures lives. The other three come from the economy: whether old in-record scores migrate, whether topics are derived or declared, and when signing starts.

## What I would remember

Every station I built without the protocol invented its own way to deliver messages, and every review found a hole in that improvised path. The protocol is not overhead on top of the work. It is the thing the work kept trying to rebuild.

If I were holding the phone, I would read the design draft and answer its eight decisions. Then the first slice can start, since it changes no surface and makes every hand-off visible in the ledger. And I would restore the two search providers.

## Names and receipts

- Staging identity at writing: 25c8b5b0 for the host (CI 38149299934); the insert-only capture fix deployed with 8031258a (CI 38149043966).
- Goal files: docs/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md (complete; receipts W1-panel1, W1-panel2, W1-panel3, W1-fix, W1-fix2); docs/definitions/choir-autopaper-w2-living-textures-2026-10-11.md (blocked; receipt W2-review1); docs/definitions/choir-autopaper-metamission-2026-10-11.md; docs/definitions/choir-pd1-persistent-research-desk-2026-10-11.md (receipts S1g, S1h).
- Protocol: docs/problems/inter-agent-communication-bypasses-precommitment-records-2026-10-11.md; docs/precommitment-records.md (draft for owner review).
- W1 fixes: 726aa392 (generic-route capture refusal, validators kept only after a write, lost-successor re-arm, overdue flag, request batching); 56ec63e0 (insert-only capture create, bounded error text).
- Research: 2d093972 (prompt v5), 25c8b5b0 (prompt v6, coverage before confirmation); PD1 receipts S1h and S1i; the hill-climb pauses for the protocol.
- Theory: docs/precommitment-records.md (the paper you sent, already in the repo since October 9, identical to the PDF); the design now cites it.
- Problem docs (docs/problems/, all 2026-10-11): w1-panel-capture-boundary-and-lost-items, capture-route-unreachable-through-proxy (with its correction), source-polls-fail-quietly-in-first-scheduled-run, inter-agent-communication-bypasses-precommitment-records.
- Search providers on staging: Serper out of credits until 2026-10-15; SerpAPI out of searches until 2026-10-12; Exa parse errors; SearXNG empty; Tavily, Parallel and Keenable active.
- Residuals named in the W1 goal file: per-poll chain growth; the dispatcher's eight shared slots; the sourcecycled body path if re-enabled; the same-owner generic-route overwrite for other object kinds; uncapped title sizes; no per-guest quota on the capture route.
- Housekeeping: a nineteen-hour agent-browser daemon and six idle orphaned processes (three duplicate dev servers, a stub model, two database tunnels) were stopped; the live dev server and your desktop-icon branch's local stack were left running.
