# Owner Rulings On The Ledger Roadmap

Date: 2026-10-14, about 00:20 UTC. The owner's answers to Part 2 of the
ledger roadmap report ("The Ledger Roadmap: Decisions, Stations and
Future Thoughts", 13 October 2026), given in session. Numbers match the
report's decisions 1 to 12. These rulings supersede the report's
defaults and recommendations wherever they differ.

## The owner's words (verbatim)

> Deepseek and luna are low cost. And if we are able to produce good
> product with them, I am willing to up my spend to scale. But we have
> other limits, namely ram, storage, and cpu. How well we use these is
> about our software architecture. We can control that.
>
> 1. We should optimize resource utilization, to scale agents per vm and
> ram/storage/cpu/token efficiency per task. track spending, but dont set
> budget caps. see the previously deferred missions from
> docs/archive/definitions/choir-appdev-sp-production-infrastructure-2026-10-09.md
> 2. Telegram should be read through mtproto. But source scaling (with
> other sources) should be deferred to after the software architecture
> improvements
> 3. Autopaper is commercial. leverage sources we can use as signals and
> profit from
> 4. Yes, Taiwan first. But, general news and tech news is zeroth. Taiwan
> is the first vertical
> 5. autoradio prototype…. We have a bunch of stuff to do, and auto radio
> quality is very execution dependent. i dont want slop. A prototype could
> be cute if its framed as a demo/prototype, not setting architecture that
> will be annoying to refactor. You can get creative with this. But
> definitely after our software architecture work.
> 6. Yes, we need paper publishing 24/7, even after crashes
> 7. My computer needs updates, yes
> 8. ok
> 9. Yes, good copyright citizen. Ill happy for threads access later
> 10. Rss hub for china content
> 11. We will iterate on discourse analytics… should think about
> architecture for future proof not any specific results rn
> 12. Sure, what do I need to do for cofacts? Ill do all application stuff
> in a couple days, not now
>
> The general point here is that we should be compounding. Not turning
> captures into copied articles, but building an object graph, and
> synthesizing articles from the convergence and conflicts between
> sources. Sense making and tension centering.
>
> So for example on language, we should be happy when the same story or
> topic is covered in multiple languages as that is inherently meant for
> multiple audiences, cultures, polities. We are uniquely positioned to
> synthesize. A disaster would be 3 separate articles from choir about 3
> articles published about the same story in 3 different languages.
> Copying is worthless. Synthesis, analysis, inference, and connections…
> how does this new story update our library of old stories?
>
> Later I see talk of "weekly aggregate observations per frame"… we
> shouldn't look to do weekly updates, but soft-real-time. We should think
> in differential terms.
>
> And the point is not to be a republisher of the bbc and Reuters, but to
> use copious long tail sources from the global south, social media, etc
> etc.

## Read as policy

**Order of work.** Software architecture first: resource efficiency (RAM,
storage, CPU, tokens per task, agents per computer) and the remaining
re-engineering. Source scaling, Telegram, more China content and
Autoradio come after it.

1. **Cost.** Spending is tracked and reported. There is no budget cap and
   no kill switch. The scarce resources are RAM, storage and CPU, and
   those are an architecture problem. The roadmap's cost amendment is
   recorded without the ceiling. Station LR carries this. LR also pulls
   in the deferred work the owner pointed to:
   - SP (production infrastructure, deferred 2026-10-09);
   - SA (agent density: more agents per computer; the guest store's
     serialized engine);
   - S3 (fast resume, so a hibernated computer wakes without a crash
     boot).
2. **Telegram.** Read it through MTProto, Telegram's client API, not the
   web preview. Deferred with all source scaling until after the
   architecture work. When that station opens, it reads Telegram's API
   terms first: the report's concern was the February 2026 prohibition
   on AI use and aggregation.
3. **Commercial.** Autopaper is commercial. Sources licensed only for
   non-commercial use (Central News Agency, United Daily News) stay out.
   Prefer sources the paper may use both as signals and commercially.
4. **Verticals.** General news and tech news are vertical zero. Taiwan
   is the first vertical.
5. **Autoradio.** After the architecture work. If a prototype is built,
   it is framed as a demo and must not set architecture that is costly
   to refactor. Quality bar: no slop.
6. **Crash and the paper.** The paper publishes 24/7, crashes included.
   This ratifies option B of decision 6:
   - a source schedule is a standing owner intent that survives a crash;
   - a crash closes only the poll that was in flight;
   - a schedule whose tick crashes the computer twice is closed, with
     that cause.

   It refines CLAUDE.md's crash rule for standing intents, and the
   operating contract must be updated to say so. It joins the tiers
   ruling: the paper computer is always on, and the always-on sweep
   wakes it.
7. **The owner's computer.** It gets updates. One operator refresh is
   approved, done when the computer is idle. Deploys then refresh it on
   their own when it is idle.
8. **The wedge bound.** Accepted: one automatic retry per external
   event, then a hold. This amends the 2026-10-10 ruling from "always" to
   "once per external event". The interim L1b-0 (one automatic warm per
   failure stop per 24 h) is consistent with it. The boot ledger (L1b)
   makes it exact.
9. **Taiwanese social platforms.** Be a good copyright citizen: no PTT
   or Dcard scraping. Threads access is applied for later.
10. **Mainland China.** Use RSSHub for China content, as a later source
    station. Every row stays labelled as platform-curated, because a hot
    list measures what the platform shows, not what people think. The
    report's PRC data-law concern is recorded for that station to read.
11. **Discourse analytics.** Iterate. The architecture should be future
    proof, with no commitment to specific results now. Replace "weekly
    aggregate observations" with soft-real-time differential updates:
    each capture updates the standing estimate, and what is published is
    the change.
12. **Cofacts.** The owner does the application work in a couple of days.
    What it takes:
    - a Hugging Face account;
    - accepting the Data User Agreement on the dataset
      `Cofacts/line-msg-fact-check-tw` (approval is automatic; the
      "who are you" field may say n/a);
    - a read token for that account, stored in the operator secret file,
      never in chat or the repo;
    - optionally, for the live API, an `app-id` from the Cofacts working
      group (their LEGAL.md §3).

    The licence is CC BY-SA 4.0 with attribution. Whether a derived
    aggregate counts as adapted material is a question for counsel.

## The product principle (owner, 2026-10-14)

Autopaper compounds. Captures become an object graph, not copied
articles. Articles are synthesized from where sources converge and where
they conflict: sense making and tension centering. A story covered in
several languages is one story with several audiences. Choir writes one
synthesis of it, never one article per source language. The question
each new capture answers is how it updates the library of earlier
stories. The paper is not a republisher of the BBC or Reuters. Its edge
is the long tail: sources from the Global South, social media, and other
sources nobody else synthesizes.

Consequences to check now, not later:
- Does the live paper open one story per capture, or one per real-world
  story? Three captures of one event in three languages must update one
  story. That is a check on the current paper's publications.
- **Observed 2026-10-14 00:25 UTC on the live paper computer** (its
  50 documents, read through the owner's API as counts only). For each
  document, I read the management Cast records in its `texture:<doc>`
  inbox.
  - 47 stories opened from exactly one capture, 2 from two, 1 from four.
  - No story received a second management Cast. Feeds go by Note: about
    11 Notes carried one new capture each into 8 existing stories, mostly
    follow-ups from the same source (the same NPR interview, Hacker News
    threads). *(Corrected 00:30 UTC: the first count read Casts only and
    missed the Note feeds.)*
  - The titles track single source articles (TechCrunch, Hacker News, BBC,
    NPR).

  So today intake turns one capture into one story.

  *Correction, 00:55 UTC.* The published stories are not copies of that
  one capture. Research widens each one from the open web: in 15:00-21:00
  UTC, 31 of 33 published versions cite two or more outlets, with a
  median of about 6 and one citing 36 (scripts/l2a_paper_window.py).
  What the principle rules out is still there:
  - captures of one event are not consolidated at intake;
  - research re-searches the web per story instead of building on the
    corpus;
  - stories do not update one another. This is not yet a measure of what
  research cites while it writes; the per-version source count is the
  next observation. It is the evidence for station L2.
- The intake redesign (station L2) already treats a capture as an
  observation and the open question as the paid unit. Cross-language
  grouping is part of that, and so is updating existing stories over
  opening new ones.
- The fourteen live sources are mostly large English-language wires. The
  long tail is the target, after the architecture work.

## What the report's phrases meant (owner asked)

- "One pinned story version opens with its PDF and media evidence" (4.21,
  station L5). It is a public evidence reader. A reader opens one fixed
  published version of a story, pinned to that version's id rather than
  to "latest", so it never changes under them. They see the captured
  evidence behind it: the original PDFs, images and media the sources
  provided. Each citation chip opens the exact captured passage. It
  works for a reader who is not logged in.
- "Weekly aggregate observations per frame" (2.37, 6.13). That was the
  research's proposal. Ruling 11 replaces it with soft-real-time
  differential updates.
