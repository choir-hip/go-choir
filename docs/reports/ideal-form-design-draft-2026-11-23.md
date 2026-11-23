# The ideal form, a design draft

I wrote this on Monday the twenty-third of November, in the evening. It is the listening version of a design draft that a second model wrote this afternoon, at your request, from your words today, the fourth draft of the first design, the learning engine, and today's measurements. Nothing in it is built. Where a number is a guess, I say so.

The draft proposes one measure for the paper, a new order for the work that happens when something arrives, a way to learn importance without ever asking a model how important something is, and a path that begins with experiments.

Here is the whole of it in a minute. The measure is the gap between how the world spreads its attention and how our front page spreads its own. Importance belongs to a subject, and it is counted from independent sources, so a new story on a hot subject starts hot. When something arrives, vectors go first and keywords second, because the useful keyword question is which words are new to the neighbourhood. The root steers a ranking that plain code renders, so the page and the radio exist when every desk is down. Your idea of batching many captures into one model call is strong on the arithmetic, and an experiment decides it. Since the draft came back you have answered several of its questions, and the last chapter says where that leaves us.

## The measure

Start with a happening. A happening is a group of arrivals, from sources, that report the same thing. Research's own reading never counts, and neither do our stories.

The world's attention on a happening is its share of the independent sources reporting it right now. One source counts once. Forty outlets running one wire report count once.

Our attention on a happening is the share of the front page held by the story that covers it, where the top of the page counts for more than the bottom. A happening with no story gets a tiny share, so that missing it is expensive.

The measure is how far our shares are from the world's, in bits, averaged over time. It is zero when we give each happening the weight the world gives it, at the moment the world gives it. That is your perplexity of the corpus against the world, said as a number.

It splits into five parts. Missing is what the world is reporting and we have no story for. Late is the same thing charged by the minute, with the clock starting at the source's own timestamp. Misranked is what we cover at the wrong weight. Unseen is what none of our sources reported. Personal is the same sum for one listener, with their attention in place of the world's.

Today's failure was all Misranked. We had the Nvidia story, with sixty-three sources, and gave it a tenth-place share. A measure that only counted coverage would have called today perfect.

The Unseen part needs no source held back. There is an old statistical trick for it. Count the happenings that exactly one source reported, and the ones exactly two reported. The more singletons you see, the more you have not seen at all. That gives a running estimate from what we already hold. Beside it, once a day, research can read a handful of front pages we do not ingest and ask whether we cover each headline. That is the root's reporting about itself, and it costs perhaps thirty searches a day.

The draft is plain about the biases. Our sources are not the world: four fifths of last week's arrivals were social posts. How much a wire counts against a social account is an editorial choice, and it should be yours, stated openly. And the grouping is ours, so the grouping needs its own audit by hand.

## Importance

The draft says do not ask a model how important something is. That is what failed today.

Instead, each subject carries a counted rate: independent sources per hour, remembered at two speeds. The fast one forgets in a couple of hours. The slow one forgets over a week. Importance is the fast rate. A burst is the fast rate standing far above what the slow one expects.

That rate is also a forecast. It says how many sources will report on this subject in the next day. The next day's arrivals settle it, and nobody in the system can write that outcome.

Because the rate sits on the subject, a successor story inherits it in its first minute. The reinstatement would have opened at the top of the page.

The draft names three ways this fails. Some things matter and are barely talked about. Attention can be manufactured. And prominence can feed itself, when outlets and listeners follow what we lead with. For the quiet things it offers a slow outcome at a week and a month, listeners' asks, the record of sources that are often first, and subjects you declare yourself. For manufactured attention, count origins and not copies. For the loop, a small random share of front page and queue slots filled against the ranking, so that what readers do can be corrected for what we showed them.

The root may raise or lower a subject. But its edit is itself a forecast, bounded, and it fades in a day unless arrivals prove it right. How far the root may move things grows and shrinks with its record.

## The work when something arrives

Yesterday's offline trial failed every line we set for it, and the draft treats those failures as instructions.

Names cannot lead. Rules that looked for names found the actor and missed what happened. Vectors found the right story three times in four with no model at all. So vectors go first.

Keywords come second, and are read differently. The question is not what this item is about. It is which words here are rare among its neighbours. The word "reinstated" was not in the petition story. That is the cheap detector for a turn inside a running story, which closeness alone cannot see.

More than half of arrivals match no story. They cannot each go to a model. They pool, and group with each other, and most never find a second source and cost nothing more. They stay searchable and playable.

So the order is: clean the page down to the article, check whether it is a copy, embed it, find its neighbours, read its new words against them, walk the graph for candidate stories, and then let plain rules settle the easy cases. An echo adds one source to its subject and stops. Only the ambiguous remainder goes to System 1, and a language model is called once for a group, never for a single item.

There is a guard on the echo rule, and it corrects me. I had said an echo should cost one vector match. On that rule alone, today's turn would have been swallowed as an echo, because a turn in a running story is close to everything before it. So an item is an echo only if it is close, and carries no new words, and its subject is not bursting.

The local stages take under a second. With a short wait to gather a batch, an item is held, counted and rankable about two minutes after we fetch it.

On scale. Today the limit is one we designed in: a management session for every twelve captures. At ten times today's arrivals the limits become System 1's accuracy when a call holds many questions, and the disk. At a hundred times it is the single host. The order of the stages does not change.

## Your batching idea

The draft takes it seriously, and the arithmetic favours it more than I had allowed.

All of today's source arrivals, at twenty-five to a call, is about six model calls an hour. The system makes about nineteen hundred an hour now. Six an hour fits inside the Synthetic allowance that sits nearly full.

One caution. A batch call with no retrieval cannot know what the corpus already holds. So the honest form still does the cheap local work first, and replaces the rules, System 1 and the per-group call with one batch call.

The experiment has four arms on one replayed day: the staged funnel, batches with no retrieval, batches with each item's nearest stories attached, and what management actually did. Three outcomes judge them. Did the world follow, meaning two more independent sources within a day. What a blind reader says on two hundred items. And twenty hand-collected cases where a story's point turned, today's among them. Cost is priced at the paid rate, because a free quota is not a price. The lines that decide are written before the run.

The draft's guess, at six in ten, is that a hybrid wins: local work for copies, echoes and counts, a batch call for routing and naming, and System 1 as the auditor.

## The root, the edition and the radio

The root's Texture holds four things: the index of subjects with their importance, the plan of the page, a notebook of lessons, and the paper's own numbers.

The front page is that Texture rendered by plain code. The root edits the ranking's settings and pins. It does not type the list. So if the root is down, the last settings and the live counts still give a fresh page.

The root wakes on events, not a clock: a group with sources and no story, a burst on a subject that has one, a forecast settled, a part of the measure moving, a listener's question we could not answer. It is given totals, never raw captures. It sends stories out as sub-calls, most uncovered attention first. It sends research for a named gap and for the daily look at outside front pages.

Management keeps the coherence of the computer, admits engineering, and holds the bound on how far the root may move things. It makes no decision about any capture.

For a story's identity: one main point, fixed when it opens. A turn is decided on a group of arrivals, never on one. A successor inherits its subject's weight. If a successor draws no arrivals of its own in a day, it folds back into its predecessor as a section.

A person's edition is the same render on their own computer, over the paper plus their private things, ranked there so their interests never leave.

The radio has four layers, each a fallback for the one above. The phone's saved queue, which is never thrown away. The plain render of the edition. The Autoradio desk, a real one, which reorders and writes the transitions and replaces the tail of the queue, never the item on air. And the root's edits. The player always has the lowest layer, so it never waits.

## The path

Instrument first. Today no record joins a capture to the decision made about it, and the paper's decision log keeps two hours. So the first thing is a durable row for every capture at every stage.

Then measure, with nothing a reader can see. Compute the measure over a replayed week, and check one claim before trusting it: that at seven in the evening, universal time, today, it shows the Nvidia happening in the world's top three while our page held it tenth. If the measure cannot see the failure that started this, it is the wrong measure. In the same replay, drop sources at random and count happenings, to learn whether happenings grow more slowly than sources. If they do not, there is no scaling law.

Then run the new stages in shadow beside today's machinery, with the batching experiment.

The first change a reader would see is the front page ordered by the counted importance, once the shadow shows it beats today's order.

## Where it stands after your answers

You have said five things since the draft came back, and they move it.

You agreed the page is a render the root steers. You agreed vectors go before keywords. You want the batching experiment run. And you said experiments come before the design is decided, which is the draft's own order.

You questioned three things. First, you said reading less at the same coverage should improve the measure. I think that is right, and it means the measure is two numbers and not one: the gap, and what it cost to get there. Second, you said fixed polling should give way to learning each source's own rhythm, so that many sources on staggered rhythms make the paper nearly live. Third, you said a rule like "three or more sources" is an antipattern, and the turn needs a better criterion than a count. I agree, and I answer all three in our conversation.

The one thing to remember: the draft's centre is that importance is counted from the world and attached to subjects, and that everything else is measured against the gap between the world's attention and ours. Whether that holds is an experiment on data we already have, with no spending.

## Names and receipts

The full draft, with its tables and update rules, is in the repository at docs/design/ideal-form-draft-2026-11-23.md. The record of today's conversation that it was written from is beside it at docs/design/design-session-2026-11-23.md.

The draft lists thirteen decisions for you. Answered so far: the front page as a host render the root steers, yes; vectors before keywords, yes; the batching experiment, run it; experiments before design, yes. Open: the two clocks for held and published; splitting System 1 calls by neighbourhood; the turn criterion; counted arrivals as the wake with predictions riding along; who is custodian of groups and counts; source class weights as your prior; the measurement panel as append-only; the measure's conventions; random exploration slots; and what follows the fourth of December, when the system map says the paper's main model stops being free.

The calibration claim still open on density is C203. The metamission with the four tracks is at commit 548696b5b.
