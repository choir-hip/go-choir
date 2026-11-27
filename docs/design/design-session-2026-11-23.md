# Design session with the owner, 2026-11-23 (afternoon and evening): what was said and what was measured

Standing: governing | kind: happened | settled-by: the owner's words of 2026-11-23, quoted exactly in this file (line 7); a record, not a design | note: its design half is carried by design/desks-and-engine-2026-11-24.md

This is the record of one conversation, not a design. The owner's words are quoted exactly (some were dictated by voice, so a few words are mis-transcribed: "Quire" is "Choir").

## The owner's statements today

On what is wrong:
- "I think the big problem is the source ingestion pipeline. Management is not supposed to be the top of the funnel and bottleneck. Every capture should first run through keywords, vectors, graph traversal, and then get clustered before groups of captures go to jev…well, the exact order and process of keywords, embeddings, graph traversal and jev is something we need to learn and theorize. But those are all cheaper than llm. The point is to do work before we send captures to rlm to turn into stories. But also, to minimize the time between capture and publication. And maximize the semantic coverage. We are building our object graph corpus ontology, not just writing stories."
- "The root texture rlm is for keeping the high level understanding. It keeps and updates the importance of different topics, based on statistical aggregates and delegation. It replaces much of management."
- "management is doing way too much work right now, and the work that management's doing should be done by the root texture and should be done by keyword extraction, vector similarity, graph traversal, and system 1 models. And we may also want to use language model calls as like a tool call in that pipeline, but it should not be something that management is responsible for."
- "It may turn out actually that just passing captures through to an llm to summarize them as they come in is more efficient than doing keyword extraction, vector similarity, system1 classification, and graph traversal first… especially if we can do like 10-50 captures per llm call. we do need to experiment with this."
- "I realize this is sounding quite vague, there are still some unknowns, so how do we find the right constraints to reduce our design search space? methinks that the 4 stroke learning engine should be integrated more deeply. That will help us with the design. Wherever we have design questions or where there seem like too many unknowns, remember that we need to do more learning and we have a general learning loop for that."
- "Novelty and importance learning is key. We have to learn the right way to learn them."
- "We should do more to review traces so we have more understanding of how the system currently works as well. That, alongside reinforcing focus on embedding the 4-stroke learning engine, will be the grounding and vision that keep us on track."
- "My general impression is that we've already built something phenomenal with autopaper and autoradio... But it feels more like a prototype than production grade."

On the measure:
- "we want to have some measure of perplexity of our corpus versus the world."
- "the way to lower perplexity is to have maximal coverage, so that basically like anything that happened more than 1 minute ago that is consequential should be in Choir, and the importance of that story in the world should be roughly the importance of it in Choir. Now, what's materially consequential, what's important in the world, these are kind of subjective judgments, so different outlets will vary on that, but I think we can be intelligent about it, and if we're able to do a pretty good job as a baseline, then it'll be personalized, and what's important to Sally is going to be different than what's important to Jonathan. And we can serve both of them subjectively based off of how they use autoradio and what they say and ask for."
- On a held-out set of sources: "Not sure I really want a held out set. Like, we should be incorporating as many sources as we can now. Maybe the natural held out set is there may be some sources that we can't use in choir, so we can kind of test against those, but the point is not that we should be not covering something so then we can test it."
- "ideally we are reading less because we are aggregating the consequential and long tail sources that matter. And so over time, we'll be able to basically skip what's redundant and only be carrying signals that in a linear combination comprise the news."
- "you are right we need better measurements of the paper vs the current events / news. We should be tracking this over time. It's important that root texture desk for paper computer knows its own metrics. It can / should be able to call research for this metareporting."
- "our goal is not just trying to write stories, but reduce the perplexity of our corpus."

Rulings on the root and the readers:
- "Yes, root texture is autopaper front page (for paper computer) and edition for user computers. Meaning root texture/edition for users can include stuff from beyond the paper, such as emails, textures that track engineering work, and more. This is the intended design actually."
- "And autoradio connects with root texture. As it turns text into voice, scripting transitions and linearizing/reranking the story queue."
- "To be clear it's not that an email interrupts the program, but that it can be included in the queue alongside stories, podcast clips, etc."
- "In no circumstances can we tolerate waits or delays or pauses in our radio. The design is meant so there's always something to play even if the desks are down. We should be able to use our hybrid rag system, which we're designing, so that we get the front page. We also get the user's edition; there's two different front pages; and then we always have nearly unlimited stories because we have non-agentic ways of ranking in general and for every user. Furthermore, we have the queue of stories since the last time the user listened."
- A desk is an RLM (a recursive language model agent with a REPL running cells). The thing answering voice turns today is "the phony desk which is not a desk". The intended turn: "the local model should respond while the autoradio desk (real rlm desk) writes the new radio queue as a function call, then once it's done it smoothly transitions to take over voice again."
- "We can't lose logs."
- On resources: another month of the Synthetic provider (DeepSeek) to use at maximum sustained throughput beside Devin flash; a new Keenable search key; "I do want to think about optimal resource usage, about how we can scale up captures to cover more of the world (breadth and depth) and optimally use our limited llm and system1 resources. Since we are doing vectors and keywords and graph traversal locally, we should also be mindful about our throughput there as well."

## Proposals the lead made in this conversation (the owner has not ruled on these)

- Perplexity read three ways: capture level (did we ingest it), story level (did we publish it), attention level (is it as prominent with us as in the world). Today's failure showed only at the third.
- No held-out sources. Three outcomes the paper cannot write for itself: time (each arrival tested against what the corpus held the moment before), a new source's first contact (how surprised we are by its first items estimates what is still unseen), and listeners' asks (an ask the corpus could not answer is surprise from the reader's side).
- Importance as a forecast of what will follow (how many independent origins pick it up, how long it persists, how many later stories cite it, how often listeners ask), settled by what did follow; personal importance is the same forecast judged by one listener's behaviour.
- Each source carries learned state: originality, lead time, niche; a forecast per source ("its next items will be this much new") settled by arrivals; redundant sources polled rarely, with a random share still read in full as a check.
- An echo (a repeat of what we hold) should cost a vector match and no model, and still count as one more independent origin on the subject: content from the few originals, attention from the many echoes.

## Measured today (2026-11-23), read-only

The failure that started this: the day's largest story (Jensen Huang reinstated at Nvidia) was published with 63 sources and ranked near the bottom. Its importance was 3 of 10, scored fresh at its current revision: a host timer asks System 1 for a 1 to 10 judgment from the title and first paragraph only, with no memory. The reinstatement was fed into a week-old story about a petition; that story took 38 of 276 feed decisions (14%) in a two-hour window. Stories opened before 2026-11-21 have no fixed main point (the judge is shown their current first paragraph, which is rewritten each revision), so they can never look like they have moved on.

The current funnel on the paper, two-hour window: about 200 to 240 captures judged an hour; management decisions: 67% feed an existing story, 15% open a story, 18% decline; successors 3 of 60 opens. Feeding requires management to write nothing; opening requires a main point and a judgment on every candidate; the prompt says "when unclear, feed". Earlier counts: about 9 model calls per capture and 58 per published story; 65% of a week's captures were research's own web reads; a management session for every 12 captures; the event layer mints its own event for 84% of captures.

Model calls, three hours: Devin swe-2-flash 4,238; Bedrock Haiku 789; Synthetic DeepSeek 265; River DeepSeek 235; others about 180. Devin sheds 40 to 160 calls an hour to the next provider for quota. Synthetic's weekly bucket refills 2% every 202 minutes (about 0.6% of capacity an hour sustained; roughly 47 calls or 2.3M input tokens an hour at today's mix, an estimate); it sat at 97% full. Research runs that made a model call: 12 to 44 an hour today after a prompt revert, 3 to 34 overnight.

Web search: Keenable answered all 1,568 searches today; the three other paid providers were out of quota.

Vector search: the exact read over 72,565 story passages takes 67 to 97 ms for up to 80 neighbours and 2.3 to 2.7 s from 100 (a planner change). Captures hold about 252,000 indexed pages. The query embedder is a local model on the host. Corpus search for research: median about 425 ms.

System 1 (Jev): one call can hold many questions (the practical cap in the scorer is 12 questions a call for sections; the design's rule is one call per flush with every question in it, split only where the vendor's limits refuse). The offline trial of the first design found hundreds of questions per call are 10 to 100 times beyond measured accuracy.

Logs: the paper computer's own decision lines live only in a console file that keeps about two hours. The run ledger is durable but does not hold the prompt a model saw, what woke the run, or its search queries. No record joins a capture's id to the choice made for it, so a density measurement (does corpus density around an arrival separate fed from opened captures) could not be run. A partial reading on 39 research runs: where the corpus was denser around the question, the run leaned more on held pages (AUC 0.72, interval 0.55 to 0.86, definition chosen after seeing the data: a lead only).

The first design's offline trial (2026-11-22) failed all four kill lines: rules found names and not what happened; candidate lists were mostly page furniture (median 28 entries); 46 of 75 news arrivals reported something no story covers; when a story exists the ten nearest by vector hold it three times in four.

The radio: nothing puts front-page stories after its opening; the saved queue is discarded after 30 minutes; the server "desk" for turns and for stretches is one streamed model call with no tools; the first stretch takes 40 to 50 s.

## Later the same evening: the App Store, and a list of things before we can ship

The owner: "i just renewed my apple developer account. so, another new mission is doing our app store/testflight application... we need to do this and also make screenshots and video previews... and improve the onboarding (voice onboarding as mentioned above) and explanatory materials for the apple reviewers." And: "you should be able to make an account (which also applies to choir.news) first inside autoradio app. so, we may have a list of things to do like this before we can ship."

Reading (the lead's): a fifth track, T5, shipping the app. Its first product requirement is the owner's: an account is created inside the Autoradio app, and it is the same account as on choir.news. Today the app only signs in to an existing account (the device flow, with the approval page in the system sign-in sheet); it creates nothing.

The list before we can ship, as it stands (the owner's items first; the rest are faults and gaps found today, not yet ruled on as blockers):

1. Create an account inside the app; the same account works on choir.news (owner).
2. Onboarding by voice: after the voice model downloads, the listener says what they are interested in (owner).
3. Screenshots, preview videos and explanatory materials for Apple's reviewers (owner).
4. The radio never waits and never opens blank (owner's rule; today the saved queue is discarded after 30 minutes and nothing plays front-page stories after the opening).
5. The voice pipeline as specified: a model for speech detection, the recogniser the owner named, a model for end of turn, voice-on sounding the same as voice-off (today the first-commit baseline runs, and setup's picks are not kept across a launch).
6. The developer menu out of any build a tester or reviewer carries.
7. A way in for a reviewer who has no account and no second device.
8. Deleting an account inside the app, which Apple requires of an app that creates one (to be confirmed against the current guideline).
9. The account boundary: frozen at one account per install, three approaches tried.
10. Faults the owner reported today: page switches that show a title for ten seconds or more, playback stopping on a switch to another app with voice on, an answer cut off by a stretch arriving, podcasts skipped while transcription has no durable host.

The owner, asked whether the radio should play before any account exists: "and yes, autopaper and autoradio should work logged out." Reading (the lead's): a new person opens the app and hears the public paper at once; an account is asked for only when they want something personal (their edition, voice turns that need their computer, mail, files). This is item 0 on the list above and it also answers item 7 (a reviewer's way in) and most of item 4 (never blank, never waiting). The simulator rule also changed today: several slim simulators may run at once (CLAUDE.md).

The owner, on voice when logged out: "logged out should be listening only. if you ask it should prompt you to log in/ sign up". Reading (the lead's): signed out, the app reads the paper and plays the radio; the voice control, or an attempt to ask, opens the prompt to sign in or to create an account in the app. No public ask route and no phone-only answers for a signed-out listener.

The owner, on the web: "we should also have an autoradio app inside choir.news... but that should be deferred til after android i think. not urgent. we need to figure out our web voice stack. the obv answer is cloud based voice but i dont like that for cost reasons... but maybe we can do listening only in autoradio on web choir.news, using a local model". Reading (the lead's): deferred until after Android, which is itself deferred. The first form to consider is listening only, with speech made in the browser by a local model and no cloud voice. Not designed and not scheduled.

The owner corrected the lead's reading: "exact web target, yes, but i mean an app inside our desktop". So: Autoradio as an app in the choir.news desktop, beside the desktop's other apps, not a separate web page. Whether that app hosts the shared Exact code's web build in a desktop window or is written in the desktop's own frontend is not decided.
