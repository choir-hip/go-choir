# Autopaper learning: materiality learned from readers and from the future (2026-11-04)

Status: design note, not decided. Mutation class green (docs only). Nothing
here is built, and nothing here should be built before the owner walks
through it.

Scope and order. This is the Autopaper work that follows the modular
architecture mission
(`docs/design/modular-architecture-plan-2026-11-04.md`, which lives on the
plan branch). It does not change that plan's phases. It names the places
where the plan gives this work something to stand on: the prompt router in
Phase 4 (MA4.8, plan section 9.2), the prompt-bar and app packages in
Phase 5 (plan section 11), the custody rules in Phase 3, and Bayesian trust in
Phase 7. Section 8 is the one part that could move sooner, because it is a
small change to how stories are produced today.

Evidence marks. Every code claim is marked V (verified by reading origin/main
at 7eabda58e) or H (hypothesis). Owner words are quoted with their date.
Research claims point at the two research files and are only as strong as
those files say they are.

## 1. The question

The owner asked how the system should learn materiality weights. The first
answer was readers: clicks, related-story chains, "irrelevant to me" and
"unimportant" buttons, and freeform text in the prompt bar with an article
selected. The second answer, later the same day, was that readers are not
enough: "materiality can't just be reader focused but also future world
focused. There's a lag time but that's the ground truth." This note takes
both. Readers give a fast, noisy, partly gameable signal. The future world
gives a slow signal that the paper does not control.

## 2. Materiality is learned, not stipulated

Today the materiality formula is a stipulation. The precommitment doc writes
`w(c) = P(c|a) * impact(c) * relevance(c)` and calls it a proposed design
(`docs/precommitment-records.md` section 3.1). The scoring research found the
first flaw in it: if the set of consequences that count is chosen after the
outcomes are seen, the score is dishonest (the forecaster's dilemma). The
fix, which the owner accepted on 2026-11-04, is that weights are frozen at
commit and later evidence settles the predictions without reweighting them
(`docs/research/scoring-and-materiality-2026-11-04.md` section 8.1, point 1).

For Autopaper the same rule gives a mechanism, not a formula.

1. At publication the paper precommits a relevance prediction for the story.
   It says, with a probability and an interval, who the story will matter to
   and how: which kinds of reader will act on it, whether later stories will
   rely on it, whether a correction will be needed. The weights that combine
   these are frozen in the record.
2. Later evidence settles each prediction: reader behaviour, later stories
   that rely on it, corrections, world events. The score uses the committed
   probability and the committed weights, never an edited one.
3. What the settlements teach goes into the next commitment's prior, not into
   the score already given. A consequence nobody predicted but that mattered
   goes to the separate foresight ledger (the research file's E2 events), and
   does not reweight the proper score.

The same mechanism applies to self-development. A change precommits an effect
on named metrics, and later outcomes settle it
(`docs/research/goodhart-and-metric-design-2026-11-04.md` section 9, point 1).
Autopaper is where the learning is fastest, because it produces about two
hundred published stories a day (H: `docs/evidence/keep-set-accounting-2026-10-29.md:285`
says "~150-250 stories, 500-1,000 versions /day"). That is a large sample
for a one-owner system, which makes the paper the natural place to find out
whether the mechanism works. The result then transfers to self-development
as evidence about the method, not as a shared model.

## 3. Reader signals, micro and macro

### 3.1 What counts as a signal

Revealed behaviour after reading counts. In rough order of strength:

- opening a source the story cites;
- following a related-story chain from the story;
- asking a follow-up in the prompt bar with the story focused;
- coming back to the story or its topic later;
- transcluding the story, or a passage of it, into the reader's own writing.

Clicks and dwell time are a diagnostic only. The Goodhart research is direct
on this: engagement metrics are the standard case of a proxy that decouples
from value, and the paper's own plan already lists readership and dwell as
"diagnostic only, paired with audit-found error rate"
(`docs/research/goodhart-and-metric-design-2026-11-04.md` section 7.3,
counter-metrics). A diagnostic is reported and never optimised, and never
enters a desk's context.

Stated preference beats revealed preference where they disagree, and the
reader's own words outrank the reader's clicks. A reader who says "this
mattered to me" is more informative than the same reader's dwell time.

### 3.2 Micro and macro are different claims

The owner's two buttons mark the split. "Irrelevant to me" is personal and
micro. It says this story should rank lower for this reader, and it says
nothing about the story. "Unimportant" is general and macro. It says the
story should not have run, or should have run lower, for anyone. The
second is a claim about the world and is settled like one: other readers,
later stories and the audit sample can contradict it.

Two other signals do not go to relevance at all. "I already knew" and "this is
wrong" route to the correction path, which exists today for support verdicts
and corrections. A reader who already knew is evidence about novelty. A reader
who says the story is wrong is a claim to be checked against sources, and it
inherits the paper's correction latency floor (P2 in the research file's
Autopaper section).

### 3.3 Freeform text is a ladder

Freeform response runs from one adjective, through a comment, a post and a
letter to the editor, up to an op-ed that transcludes articles. The owner's
2026-11-04 direction is that all of it is written in the prompt bar with the
article focused. System 1 extracts typed features from the text: what the
reader reacts to, whether the reaction is about the reader or the world,
whether it asserts an error, whether it asks for more. With the reader's
consent the text can become content. A letter can be published, an op-ed is
already a published kind (`internal/platform/publication_pieces.go:112-122`,
V: the published search treats "opinion" as a piece kind beside "story").

## 4. Response kinds are not the reader's concern

Owner, 2026-11-04: "response kinds" are irrelevant to the user, the prompt
bar and the conductor. The reader writes. The article is focused. The desks
decide what kind of response it is.

This removes a design the earlier prompt-bar audit had sketched, with router
choices named "comment", "letter" and "op-ed". Those names stay as internal
labels the desks may use in their own records. They are not choices offered to
the reader, not categories the router predicts, and not fields in the request
the prompt bar sends. The only things the prompt bar adds are the focus and
the text.

## 5. One hierarchical model for micro and macro

The owner asked for one system covering personalization (micro) and the
paper's evolution (macro). The natural shape is one hierarchical model. Each
reader has a posterior over what matters to them. Those posteriors sit under
a population distribution, and partial pooling lets a new reader borrow from
the population while a reader with history departs from it. Micro is the
reader level, used to personalise ranking and related-story chains. Macro is
the population level, used to decide what the paper covers and how it ranks.

Two things keep this honest. First, reader-level posteriors never leave the
reader's custody (section 7); only the population-level summary is built from
aggregated outcomes. Second, the model is a predictor to be scored like any
other desk's predictions: it commits a probability that a given reader will
act on a story, and later behaviour settles it. A hierarchical model that
cannot beat a popularity baseline at that is not earning its keep.

H: partial pooling is standard practice and needs no new theory. What is
unknown is whether Choir ever has enough readers for the population level to
be anything but the owner and a few others. The research file says plainly
that the experiment sizes of large platforms do not apply, and that "n too
small" is the honest report
(`docs/research/goodhart-and-metric-design-2026-11-04.md` section 8).

## 6. Future-world materiality: the lagged ground truth

Readers can be wrong, bored, or gamed. The future world is what actually
happened. A story's materiality has three parts, in order of how late they
arrive.

(a) Whether later stories build on it. This is measured with typed reliance
relations between stories, not with mentions. A later story that relies on an
earlier one for a fact, a context or a premise is evidence that the earlier
story mattered to the paper's own understanding of events.

(b) Whether those later stories are themselves downstream material. A story
relied on by ten stories nobody builds on has less claim than a story relied
on by two stories that others build on. So the measure is recursive and runs
forward in time, in the manner of PageRank over the paper's own citation
graph, with the difference that edges are typed and carry only reliance.

(c) Whether readers ultimately care. This anchors the graph in people. A
citation graph can validate itself. A graph whose important nodes are also
the ones readers return to, ask follow-ups about and transclude has not.

Later world events are a further settlement. If a story predicted or framed
something that then happened, or was overtaken by something that then
happened, that settles part of the relevance prediction too. These events are
the delayed natural resolution that the scoring research recommends as the
third kind of anchor (`docs/research/scoring-and-materiality-2026-11-04.md`
section 8.2, point 3). They are ground truth that no Choir scorer controlled.

This is the part of the design that depends most on what the code can do
today, so section 8 states what exists. In short: a typed relation between
two stories exists, but it is symmetric and has three kinds, none of them a
reliance direction. The forward measure in (a) and (b) needs a directed
relation that the producing desk records at the moment it relies on a prior
story. That is why self-citation is part of this design and not only an
editorial improvement.

## 7. Goodhart guards

These come from the metric research and are listed here as requirements on
the design, not as options.

1. Count typed reliance, not mentions. A story that merely names an earlier
   story, or shares an entity with it, is not reliance. The existing code
   already draws this line for entities: a `mentions_entity` edge has a role
   of central or incidental and only central counts
   (`internal/platform/publication_related_entities.go:7`, V). Reliance needs
   the same discipline.
2. Count a citation only when the citing story brings new independent
   sources. A story that restates an earlier story and cites it adds nothing
   and would otherwise raise the earlier story's materiality for free. This
   also prevents the paper from manufacturing materiality by citing itself.
3. Keep the materiality score out of the context of any desk that writes
   stories. This is class C of the research file's visibility table
   (`docs/research/goodhart-and-metric-design-2026-11-04.md` section 7.1), and
   the principle "no metric is both visible to agents and used for selection
   above its floor" (section 7.0, point 2). A Texture that knows which
   stories score as material will write toward the score.
4. Anchor on readers so the graph cannot validate itself. Part (c) of
   section 6 is the anchor. A material story whose only support is other
   stories is flagged, not trusted.
5. Stated preference beats revealed preference where the two disagree.
6. Satisficing floors and an exploration budget. The research's Autopaper
   floors are support (P1), correction latency (P2), source diversity (P3),
   calibration (P4) and owner non-regret (P5), with a fixed share of front
   page slots, starting at 15 percent, reserved for forms or beats never
   published before, each with a precommit and a kill date
   (`docs/research/goodhart-and-metric-design-2026-11-04.md` section 7.3).
   Materiality ranks among stories that pass the floors. It does not
   override them, and above the floors the draw is a diversity-weighted
   lottery, not an argmax. The owner's own words for this are "satisfice,
   then diversify".
7. Metrics expire. Each materiality signal carries a validation date and a
   check against an audit sample, because a proxy that tracked value last
   quarter may not this quarter.

## 8. Self-citation now: what the code does today

Owner, 2026-11-04: "we should do a better job of citing our own articles in
autopaper article production now that we have some corpus." This section is
the read-only check the owner asked for. All of it is from origin/main at
7eabda58e; nothing was run against production.

### 8.1 The pipeline

A story begins as a source poll and ends at a public route.

1. Source schedules. Each scheduled source is a chain of durable wakes. A
   wake polls the source with no model call and arms the next wake; the
   default interval is fifteen minutes
   (`internal/agentcore/source_schedule.go:20-24, 36`). V.
2. Intake. A poll's captures reach management as a captures record. Jev's
   label on each capture is advice, including a "Story as texture:<doc>"
   field that ties the capture to an existing story, and management is told
   to feed a capture into the story Jev names unless it reads otherwise
   (`internal/runtimeprompts/overlays/rlm_management_runtime.yaml:26`). V.
   Event records add a reason for each arrival: corroborated, grew,
   official_single_outlet or unresolved (`:32`). V.
3. Opening or feeding a story. Management either feeds an existing story or
   casts to `texture:new` to open one. Opening is done by the store in the
   cast's own commit: it creates the document, the trajectory and an empty
   seed revision (`internal/store/story_open.go:18-33`), and only a
   management cast opens a story (`internal/store/inbox.go:188-189`). V.
   Before it opens one, management must have read `choir.Stories`, which lists
   every story newest first; a cast from a turn that has not seen the latest
   stories is refused (`rlm_management_runtime.yaml:26`). V.
4. Research. Texture casts research. The research desk is the only desk with
   the corpus search verb (`internal/deskmanifest/manifest.go:243`; the
   Texture row at `:228` has `ReadDoc`, `ReadCapture` and `Write` and no
   search, and its comment says it "never fetches the world" at `:227`;
   management's row at `:210-212` has `Stories` and `ReadCapture`). V.
5. Texture. Texture is the sole writer of the revision. It writes with
   `choir.Write`, cites with `{{cite handle "exact words"}}` tokens against a
   frame source, a capture id or a captured URL, and reports "the news, not
   the coverage" (`internal/textureprompts/texture.yaml:23, 44`). V.
6. Publish. A story opened with `publish: auto` publishes every committed
   revision as its next version, off the commit's path
   (`internal/agentcore/story_publish.go:27-33, 159-165`). A revision that
   cites no source, or does not lead with a headline, is held
   (`:167-181`). V. The route is `/pub/texture/<slug>` and
   `/pub/texture/<slug>/v<n>` (`internal/maild/digest_render.go:15`,
   `internal/maild/digest_select.go:16`; the reader links are in
   `frontend/src/lib/AutopaperStory.svelte`). V.
7. After publish, a story's research desk reads the story and records what it
   is about, and may record relations to other published stories. That is
   section 8.3.

### 8.2 Does production retrieve the paper's own earlier stories?

Partly, and not as a requirement.

What exists:

- `SearchWireCorpus` returns three things: events the computer has resolved,
  its stories and captures, and the published pieces from any computer,
  which are Autopaper stories and opinion pieces with a `read_url`
  (`internal/researchtools/researchtools.go:49, 112-116`). The description
  tells the caller to Read the `read_url` and cite the capture that returns,
  "never the snippet". V.
- The research overlay tells research to search the corpus first and
  start from an event's captures before the web
  (`internal/runtimeprompts/overlays/rlm_research_runtime.yaml:30`). V. The
  verb is listed for the research desk and the research child role
  (`internal/deskmanifest/manifest.go:243, 291`). V.
- System 1 can read a story as evidence. A `story:<revision_id>` context ref
  resolves to that revision's text, bounded
  (`internal/agentcore/system1_verb.go:72, 461-476`). V. But it reads a
  revision of the computer's own store through `GetLifecycleRevision`, so it
  reaches a story the asking computer holds, not any published story. H, from
  the call at `:467` being scoped by owner and computer.
- On a story's research desk, the story analysis gives each existing entity
  candidate up to a bounded list of prior published stories that mention it
  (`internal/types/story_entity_candidates.go:116-118`, filled at
  `internal/platform/publication_entity_candidates.go:545-556`) and asks
  System 1 about the story's novelty
  (`internal/agentcore/story_analysis.go:24-37`). V. This is built after the
  story has a head revision (`story_analysis.go:200-253`). V. The facts can
  be trimmed to fit the bound (`:399-433`). V. This hint exists to record
  what the story is about, not to tell Texture what the paper has already said.
  H: no code path hands these prior stories to Texture.

What is missing or weak:

- Nothing makes research search for prior coverage of the story's subject.
  The overlay says to search the corpus first, but it does not say to look
  for the paper's own earlier stories on the same subject, and the finish rule
  in force on a story's research desk is about entities, not prior coverage
  (`internal/agentcore/inbox_records.go:873-920`). V.
- Texture's prompt has no instruction about earlier coverage, and its
  "one report is enough" rule points the other way
  (`texture.yaml:44`). V. Texture also has no verb to look for it.
- The published search is a plain keyword scan. It walks publications newest
  first and reads each one's text, and a piece must contain every query word
  (`internal/platform/publication_pieces.go:141-166, 198-215`, where a term
  with zero hits sets the score to zero and breaks). The query is bounded to
  eight words (`:22`). V.
- That scan stops at 2,000 publications (`:33`) and then reports itself
  capped (`internal/researchtools/published_search.go:75-77`). V. The corpus
  was already 3,443 publications on 2026-10-29
  (`docs/evidence/keep-set-accounting-2026-10-29.md:204`). So the search
  cannot see the oldest part of the corpus today, and the gap grows every
  day. V for the cap and the count, H for the reach (it depends on how
  `scanPublicationRows` orders and counts rows).
- The search is lexical. A story about the same subject in different words, or
  in another language, is not found. V that it is lexical; H that this
  matters in practice.

Management does see the list of stories (`choir.Stories`) but only to avoid
opening a duplicate. That is the paper's main use of its own corpus
at production time today: one story per event, an update feeds the existing
story (`rlm_management_runtime.yaml:26`). V. So the paper handles "the same
event again" well, as one living story with versions. It has no
mechanism for "a new event that follows from an older, different story".

### 8.3 Do published stories cite or link earlier stories?

Between stories, rarely and by accident. Within a story, to the world, yes.

- Citation tokens resolve to captures. A published version gets one
  `cites_capture` edge to each capture its anchors cite
  (`internal/platform/publication_related_edges.go:3-19`), and the kind is
  registered at `internal/objectgraph/registry.go:301`. V. Transclusion
  anchors are `GraphTransclusion` records with a `Ref`, a relation type and a
  snapshot (`internal/platform/publication_graph.go:700-713`). External
  citations become `GraphCitation` edges with the relation type
  `"references"` (`internal/platform/service.go:587-606`). V.
- There is no edge whose target is another story. A story that cites an
  earlier story does so only if the earlier story's export was Read into a
  capture and cited as a capture. That is the route the op-ed problem doc
  designed (`internal/researchtools/published_search.go:12-16`, citing
  `docs/problems/oped-not-connected-2026-10-20.md`). V that the route exists;
  H that Texture or research uses it for ordinary stories. The reader would
  see an ordinary numbered citation to a capture of choir.news, with no
  record that the target is a story of the paper.
- Reader-facing related stories are computed, not authored. The walk finds
  other current stories that cite the same captures or captures of the same
  page, adds judged pairs and central entities, and caches the answer in
  memory (`internal/platform/publication_related.go:3-36`). V. It is a read
  over Store B and stores nothing.

### 8.4 Typed relations

They exist, in one place, and not in the form the forward measure needs.

- The research desk's `choir.RelateStories` records a judged pair of
  published stories with one of three relations: `same_incident`,
  `later_development` and `same_subject`
  (`internal/types/story.go:164-170`). V. The pair rests on a System 1
  answer of the same run, the probability is read from that answer and not
  supplied, and a relation System 1 did not rank first needs a written reason
  (`internal/agentcore/story_relation_verb.go:21-47`). V. The record goes on
  the tape first and is exported afterwards as a `related_story` edge
  (`:37-40`; edge kind at `internal/platform/publication_related_story.go:43`).
  V.
- The edge is symmetric. It is stored once with the smaller id first, and its
  metadata is only the relation name
  (`internal/platform/publication_related_story.go:12-16`). V. So
  `later_development` does not say which story came later, from the edge
  alone. H: a time order can be recovered from the publication times, but
  the edge does not state that the later story relied on the earlier one.
- There is no `builds_on`, `updates`, `contradicts`, `follows_up` or
  `mentions` relation anywhere in the code I searched
  (`git grep` for those names under `internal` and `cmd` found no relation
  vocabulary). V for absence in that search.
- The `mentions_entity` edge is typed by role, central or incidental
  (`internal/objectgraph/registry.go:307-310`). V.
- Both write paths are gated. The research desk's verb requires the
  `research.related_story` setting, which is off by default at the setting's
  definition (`internal/store/computer_setting.go:49-59`). The reader-side
  read switch `CORPUSD_RELATED_STORY=on` is set in the deployment
  (`nix/node-b.nix:1088-1094`) at the owner's direction of 2026-10-26. V.
  Whether the setting is on for the paper's computer is H; I did not query it.
- The related-stories feature was built for the reader's "more like this"
  list. Its design documents say so, and its evidence docs measure link
  coverage, not production use
  (`docs/evidence/related-stories-why-empty-2026-10-26.md`). V from titles
  and first lines; H that nothing in production reads these edges.

### 8.5 How big is the corpus

On 2026-10-29 the platform store held 3,443 distinct published source
documents and 11,870 versions, with 3,330 of the documents and 11,614 of the
versions belonging to the paper's computer
(`docs/evidence/keep-set-accounting-2026-10-29.md:204-208`). The earliest
publication was dated 2026-10-13 (`:209`), because the 2026-10-09 reset
emptied the earlier ones. The same file expects "~150-250 stories, 500-1,000
versions /day" (`:285`).

So the corpus is about three weeks old at that date. An estimate for today,
2026-11-04, six days later at 150 to 250 stories a day, is roughly 4,300 to
5,000 documents. This is H, built from a measured count and a rate
estimate; I did not query production. It is large enough to need an index,
and small enough that a few thousand title embeddings or a plain full-text
index would hold all of it comfortably.

### 8.6 The smallest change

Principle: the paper should find its own earlier coverage of the story's
subject, read it, rely on it where it is true and relevant, and say so in a
form that is both a good citation for the reader and a record the machines can
use.

The smallest change has three parts and touches production prompts and one
search path. It adds no desk, no schema and no setting.

1. Retrieval. Give the story's research desk an instruction, in the research
   overlay, to search for the paper's own earlier stories on the subject
   before the web, and to Read the ones that bear on it, using the verb it
   already has (`SearchWireCorpus`, published part). The instruction goes next
   to the corpus-first sentence at `rlm_research_runtime.yaml:30`. Research
   then reports what it found in its packet like any other source, and each
   story it read has a capture id it can cite.
2. Make the retrieval work. Remove the two defects that stop it reaching the
   corpus: the scan cap of 2,000 (`publication_pieces.go:33`) and the
   all-words-must-match rule (`:198-215`). The second can be softened to
   ranked partial matches without a new index; the first needs either a
   raised cap, since the scan reads blob text for each publication, or a
   small index. H: raising the cap costs more per search, and a search that
   reads thousands of blobs is slow. This is the part with real risk (below).
3. Citation. Add one sentence to the Texture prompt, next to the cite rule
   (`texture.yaml:23`), saying that when the frame holds a source that is the
   paper's own earlier story, Texture may cite it for what it established and
   should say "as reported earlier" only where the sentence needs it, and
   that a citation to the paper's own story supports only what that story
   itself established from its own cited sources. The existing rule "never
   describe the sources" at `texture.yaml:46` has to be read beside it, and
   the conflict is a hypothesis to test: an instruction to cite earlier
   stories may drive the model to narrate coverage, which the prompt forbids.

What this does not do. It does not record that the citation is a reliance on
a prior story. A cite to a captured export of a story is still an ordinary
capture citation (section 8.3). For the forward measure in section 6 the
publish step has to notice that a cited capture is a capture of one of the
paper's own stories and write a directed `relies_on` edge. That is a small
platform change beside `citesCaptureEdges`
(`internal/platform/publication_related_edges.go:53-62`), where each cited
capture is already visited. It is a second slice, not part of the smallest
change, and it is the first real use of the typed reliance relation in
section 6. Its edge needs the guard in section 7 point 2 before it counts,
which has to wait until the reliance edges can be compared with the new
sources a story brings.

Risks.

- Self-reinforcement. Citing own stories can launder an earlier error: a
  claim wrong in story A is cited by story B as established, and then by
  story C. The mitigation is the rule in step 3 (a citation supports only
  what the cited story itself established from its own sources), the support
  judge running on every new cite, and the correction path: when a story is
  corrected, the reliance edges mark every dependent as needing a look. This
  last part does not exist and would be a later slice. The first two are
  already in place for external citations, which is the reason to believe the
  risk is bounded.
- Cost and latency. More searching and reading per story. The search scan is
  the expensive part. The owner has said there is no spend ceiling and that
  costs are monitored per desk (2026-11-04).
- Prompt drift. The Texture prompt is a protected surface of the writer. A
  change to it is a behaviour change to every story and has to be proved on a
  staging story, not reviewed.
- Self-congratulation. Stories that cite the paper's own work as a signal of
  authority. The guard in section 7 point 2 applies to the later measure; for
  the editorial voice the existing rule "report the news, not the coverage"
  is the right brake.
- Landing it counts as platform behaviour change, so it follows the Landing
  Loop and is red-adjacent because it touches the writer's prompt and the
  publish path. Whether it may land during the architecture mission is the
  owner's call; I have not checked the plan for a freeze on unrelated changes (H).

### 8.7 Recommendation

Make parts 1 and 2 now, as a small separate change that does not wait for the
architecture mission, and defer part 3 and the `relies_on` edge until
Autopaper becomes an app agent.

Reasoning. Parts 1 and 2 are on the research desk, which already has the
verb and the instruction to search the corpus. They change what research
finds and reports and do not touch the writer or the publish path. They also
fix a defect that grows with the corpus: the scan cap is already below the
corpus size, so the search is already blind to the oldest stories, and every
day makes it blinder. That is a bug in a feature the research overlay already
promises, not a new feature. Part 3 changes what Texture writes and the
`relies_on` edge changes the object graph. Both are better done once, in the
Autopaper app agent's own package, where the app desk is "the sole agent
writer of its app's object kinds" (`CLAUDE.md`, Desk Ontology) and the edge
kind can be registered by the package instead of added to the host's
registry. Doing them in Texture and in the platform first and moving them
later is the kind of "excess building only to make more changes" the owner
warned against on 2026-11-04.

If the owner prefers one change and not two, then do all of it with the
Autopaper move and accept that the search keeps losing the oldest stories in
the meantime. I recommend against that because the cost of the first change
is low and its benefit starts at once.

## 9. Privacy and custody

Reader signals are private to the reader. The custodian is the Autopaper app
agent, and a reader's signals go to it and nowhere else. This follows the
owner's custody ruling of 2026-11-04 (state of an app goes only to that app's
agent) and the plan's custody rule that no desk reads another desk's kind
(plan section 8). A reader's opened sources, follow-up prompts, return
visits and freeform text are the reader's state.

Only aggregated outcomes flow to the macro level, never content. The macro
level sees counts and rates, with a floor on group size below which nothing
is reported, and it sees the typed features System 1 extracted, not the text.
A reader's text becomes content only with the reader's consent, as an
explicit act, and then it is a published piece like any other with the
reader as author.

Two consequences. The materiality score of a story is built from aggregates,
so no desk that writes stories can read it (section 7 point 3), and none can
read a reader's signals. And the hierarchical model of section 5 lives with
the custodian, which holds the reader-level posteriors and publishes only the
population level.

An open question for the owner in the plan's own terms: whether the Autopaper
app agent is the custodian of reader state and a separate small desk does the
macro aggregation, or one agent does both. The separation is safer, since the
desk that holds the aggregates can then be given a narrower grant than the one
that holds the readers' content. That is a design choice to decide with the
owner, not to settle here.

## 10. The prompt bar: sequencing

This section follows the prompt-bar audit of 2026-11-04 (V where it cites
code; the file is the session scratchpad and is not part of the repo).

Today. The prompt bar sends three fields, `text`, `command_id` and an
optional `continue_doc_id`, and the backend refuses any other field
(`internal/textureowner/prompt_bar.go:82-137`, the refusal at `:94`;
client at `frontend/src/lib/conductor.js:7-31`). V. Everything but a single
bare URL goes to Texture (`prompt_bar.go:206-246`). Focus changes routing in
one case: a focused owner Texture document makes "continue" the default
(`frontend/src/lib/prompt-target.js:9-19`). V. With a published article
focused, nothing is sent about it: the target function rejects published
and guest windows (`prompt-target.js:16`), and the server revises only a
document the caller owns, so a reader cannot respond to a published article
through the prompt bar today. V. A typed prompt creates an unrelated new
Texture. The package manifest has no `accepts` field; the `opens` field routes
files, not prompts (`internal/apppackage/manifest.go:55-90`).

Sequence.

1. Phase 4, the router (MA4.8, plan section 9.2). The request carries the
   focused app id, the focused object ref and any selection, which makes it
   a focus-aware request. The plan's section 9.2 lists the candidates and
   the System 1 typed choice but not the focus context; the audit found that
   gap, and the plan should add it. The `accepts` field is added to the
   package manifest so that candidates come from installed packages. The
   System 1 routing learns through precommitment records: each route is a
   precommitted prediction, settled by the owner's next acts. Changing the
   request body from three fields to a focus context touches the strict
   decoder, so the frontend and the backend change together.
2. Phase 5, the prompt-bar package (slice H3, plan section 11). The visible
   focus chip for every app agent, showing "Conductor" when nothing is
   focused. This pairs with the router service and can be done earlier if
   the owner wants the label before the router.
3. After both: reader responses. With the article focused and the focus
   carried, the reader writes, the Autopaper app agent receives the request
   as a precommitment-record request, and the desks decide what it is. No
   response-kind choice is offered to the reader (section 4).

Reader signals other than prompt-bar text, such as source opens and
related-chain follows, do not depend on this sequence. They are events the
Autopaper app already sees and can go to the custodian at once. Only
the freeform ladder of section 3.3 waits for the prompt bar.

## 11. Open research questions and hypotheses to pre-register

Questions. Each has no known answer.

1. Can reliance be extracted reliably from a story's text and its cited
   sources, as a typed, directed edge? What is the agreement between System 1
   and a human on "this story relies on that one"?
2. Is the recursive forward measure stable? On a graph of a few thousand
   stories over weeks, does it converge, and does it concentrate on a few
   hub stories as PageRank-style measures tend to?
3. How long is the lag? If materiality only shows after weeks, what is the
   shortest horizon at which it predicts the longest?
4. Can a reader's "irrelevant to me" and "unimportant" be told apart from
   free text without the buttons, and how often do readers mean both?
5. Does partial pooling help at all at Choir's reader counts, or is the
   population level the owner plus a handful?
6. Does the self-citation change of section 8.6 raise or lower the
   audit-found unsupported-claim rate? Citing own work could launder errors
   or catch them.
7. How should settlement treat a story that was relied on only by stories
   from the same desk and day? This is the self-validating case in a small
   form.

Hypotheses. Each states a refuting observation and needs a stated sample size
and window before it is registered, as the research files require
(`docs/research/scoring-and-materiality-2026-11-04.md` section 9.1).

- AL-H1, forward-citation materiality. Precommitted relevance predicts the
  lagged citation-graph materiality better than a reader-click baseline
  does. At a fixed horizon (start at 28 days), rank stories by the
  precommitted relevance probability and by click and dwell counts, and
  compare each ranking's rank correlation with the story's recursive reliance
  score from section 6. Refuted if the click ranking correlates as well or
  better, within an interval. This is the owner's question put as a test.
- AL-H2, the anchor matters. The reliance score with reader anchoring
  (section 6 part c) agrees better with an audit sample's judgment of
  importance than the unanchored citation graph does. Refuted if the two
  agree equally well, in which case the anchor is not earning its cost.
- AL-H3, self-citation does not launder errors. After self-citation, the
  audit-found unsupported-claim rate on stories that cite a prior story is
  not higher than on matched stories that do not, by more than one
  percentage point. Refuted if it is higher by two or more. This is the
  Goodhart-side test of section 8.6.
- AL-H4, search recall. After the scan cap is lifted, the share of stories
  for which research finds at least one relevant earlier story rises on a
  labelled sample of old and new subjects. Refuted if recall on subjects
  older than the old cap does not improve, which would mean the cap was not
  the limit.
- AL-H5, preference over behaviour. Where a reader's stated "irrelevant to
  me" and the same reader's later behaviour on similar stories disagree, the
  stated preference predicts the reader's later explicit ratings better.
  Refuted if behaviour predicts better.
- AL-H6, the quantilizer on materiality. Ranking above the floors by a
  diversity-weighted draw from the top of the materiality ranking does not
  lower the later reliance score of the front page compared with argmax, and
  raises topical diversity. This is the research file's H1 applied to this
  score, and is underpowered at Choir's scale, so an inconclusive result will
  not change the design.

Each of these is a precommitment in the sense of the project's own
practice: the paper writes down the prediction and the refuting observation
before the data comes, and the ledger settles it. That is also the best test
of the design as a whole. If the paper cannot predict which of its own
stories will matter, better than a click counter can, the learned
materiality is not working, and it is better to know that early.

## 12. What this note does not decide

It does not decide the form of the reliance edge, the horizon of settlement,
the hierarchical model's family, the custodian split, or any threshold. It
does not authorise any change to the Texture prompt, the research overlay,
the published search or the object graph. Section 8.7 is a recommendation to
the owner, not a plan to execute.
