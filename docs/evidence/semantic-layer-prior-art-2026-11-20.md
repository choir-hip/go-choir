# Prior art for one semantic layer over stories, events and entities (2026-11-20)

Research record. Question: what has been built for event and entity
knowledge layers over news, what worked, what failed, and what that implies
for one layer built on an object graph and a vector index, queried by agents
from a code cell. Searches used generic terms only.

Verification key, used throughout:
- **[opened]** I fetched the source and read the relevant text this session.
- **[snippet]** I saw it only in a search-result summary; the primary page was
  not opened. Treat numbers as unchecked.
- **[recall]** From my own background knowledge, not checked this session.

Several arXiv ids in the 2026 range appeared only in search results. I could
not tell from snippets whether each is what it says; they are marked
[snippet] and should be opened before anyone cites them.

## Summary: what the prior art says a system like this should do

1. **Separate the happening, the storyline and the report as three kinds, and
   make the story-to-event link a typed many-to-many edge.** NewsReader keeps
   event instances apart from the mentions that ground them (SEM plus GAF)
   [snippet]; the BBC storyline ontology defines Event as "an un-disputable
   real world event" and Storyline as the editorial view, explicitly not the
   article, and lets storylines nest [opened,
   https://iptc.org/thirdparty/bbc-ontologies/storyline.html]; schema.org
   separates `about` from `mentions` [opened, https://schema.org/mentions].
   Design change: story->event edges carry a role (`reports`, `about`,
   `mentions`, `follows_up`), and an event is its own object kind, not a
   column on a story.

2. **Event identity is a tunable function, not a fact; record the grain you
   chose.** Vossen and Cybulska show identity over event components (action,
   participants, time, place) and that loose constraints give "extreme
   aggregation and lumping" while strict ones make "each mention unique"
   [opened, https://arxiv.org/pdf/1704.04259, 2017]. Design change: store a
   sub-event/part-of edge and a same-event edge as different edge types, and
   keep the matching threshold and method on the merge record so it can be
   re-run at a different grain.

3. **Ground every event and entity assertion in mentions (captures and
   spans), and keep the grounding when objects merge.** GAF/GRaSP link each
   instance URI to its mentions and group statements by provenance in named
   graphs so conflicting sources coexist [snippet,
   https://cris.fbk.eu/retrieve/ddb241a5-4052-ba8a-e053-3a05fe0afd55/1-s2.0-S0950705116302271-main.pdf].
   Wikidata statements carry qualifiers, references and a rank
   [snippet, https://www.mediawiki.org/wiki/Wikibase/DataModel/Primer].
   Design change: the unit that is grounded is the claim edge, with
   source-capture id, offset or quote, retrieval time and the extracting run.
   Never store a bare event-to-entity edge without a mention behind it.

4. **Make facts bi-temporal and invalidate, do not overwrite.** Graphiti/Zep
   keeps `t_valid`/`t_invalid` (world time) and `t_created`/`t_expired`
   (ingest time); a new edge that contradicts an old one sets the old one's
   `t_invalid` to the new one's `t_valid` [opened,
   https://arxiv.org/html/2501.13956, 2025]. Design change: every claim edge
   has both time pairs; "what is new since T" and "what did we believe at T"
   are then plain queries. Note Zep decides contradiction with an LLM, which
   is a cost and a failure point (see 12).

5. **Use an existing identifier space for entities and add a local NIL
   space, with merge as a recorded, reversible edge.** Entity linking to a
   KB leaves out-of-KB mentions; the standard fix is NIL clustering, where
   unmatched mentions share a NIL id [snippet,
   https://arxiv.org/pdf/2102.10962, 2021]. Wikidata itself has both
   conflations (two people as one) and duplications, concentrated in
   homonymous people [snippet,
   https://ricerca.sns.it/retrieve/28f9efbb-6647-48e4-923d-bc839a210f55/Conflations%20and%20duplications%20in%20Wikidata%20items.pdf].
   Design change: an entity object has a local id; `same_as` to a Wikidata QID
   is a claim with evidence, not the identity; merges and splits are tape
   records so they can be undone.

6. **Do not trust a code book or an extractor you did not measure.** A 2025
   manual review of 400 GDELT records found about 55% accuracy on key fields
   and heavy redundancy [snippet, https://www.mdpi.com/2306-5729/10/10/158];
   one article producing many event rows is a known GDELT behaviour
   [snippet]. Design change: ship a small audit set (hand-labelled story to
   event decisions) with the layer and report precision on it per release.

7. **Cluster stories into events online, with embeddings, a time window and a
   second look.** Event Registry clusters per language with a similarity
   threshold and drops clusters whose oldest article is over four days old
   [snippet, https://arxiv.org/pdf/1512.07046, 2016]; Miranda et al. cluster
   a multilingual stream online with a growing number of clusters [opened
   abstract, https://arxiv.org/abs/1809.00540, 2018]; a 2025 comparison of
   stream grouping found embedding-only grouping fast but fragmenting events,
   and LLM refinement cleanest but costlier [snippet,
   https://arxiv.org/pdf/2512.03389]. Design change: vector pre-select of
   candidate events, one LLM decision over all candidates in one context
   (matches the repository's one-call rule), and a periodic merge pass for
   fragments.

8. **Benchmarks for event coreference do not predict news performance.**
   ECB+ has low lexical diversity, and a same-lemma baseline reaches
   CoNLL F1 62 to 76 depending on the paper and setting [snippet,
   https://arxiv.org/pdf/2603.00621, https://arxiv.org/pdf/2602.17424, 2026].
   Design change: do not select a method by ECB+ score; evaluate on your own
   captures.

9. **A graph beats plain vector retrieval only for multi-hop and
   summary-style questions; for lookups it is equal or worse and costs
   more.** GraphRAG-Bench (GPT-4o-mini): fact retrieval 60.9 vanilla vs 60.1
   best graph; complex reasoning 42.9 vs 53.4; MS-GraphRAG global used about
   331k tokens per query vs about 879 for vanilla [opened,
   https://arxiv.org/html/2506.05690v3, 2025/2026]. The original GraphRAG
   paper makes the same split: vector RAG for local lookups, graph and
   community summaries for corpus-wide sensemaking [snippet,
   https://arxiv.org/pdf/2404.16130, 2024]. Design change: keep vectors as the
   default path and add typed graph hops only where the question is about
   relations, history or "everything about X"; precompute per-event and
   per-entity summaries as the community-summary analogue.

10. **Pick a small vocabulary from schema.org and Wikidata and keep a typed
    escape hatch; do not start with a deep ontology.** CAMEO is a fixed event
    code with documented coding trouble (6). BBC's storyline ontology stayed
    small (Storyline, StorylineSlot, Event, Topic, Attribution) [opened]. The
    NYT used a curated vocabulary of people, organisations, places and
    descriptors published as SKOS linked data [snippet,
    https://old.datahub.io/id/dataset/nytimes-linked-open-data]. Design
    change: a closed core of kinds and edge types (the registry already has
    this), `kind` open for event types with Wikidata class ids as optional
    labels, and a rule that a new type needs a use in a query.

11. **Give agents few primitives, typed results and bounded traversal, not a
    free query language.** Text-to-Cypher on full-scale graphs reaches about
    60% execution accuracy for the best model in CypherBench [snippet,
    https://arxiv.org/html/2412.18702v2, 2024]; LLM-KG-Bench found SPARQL
    errors dominated by hallucinated URIs [snippet]. Think-on-Graph does
    beam-search over neighbours with the LLM pruning each step [snippet,
    https://arxiv.org/abs/2307.07697, 2024]; GraphWalk uses 2 to 4 generic
    operations [snippet, https://arxiv.org/html/2604.01610]; Graphiti writes
    to the graph with predefined queries, not LLM-written ones [opened].
    Design change: in the REPL expose `find` (vector plus filters, one
    call), `neighbors(id, edge_types, since, limit)` returning edges with
    provenance, `path(a, b, max_hops)`, `get(id)`, `claims(id, as_of)`; all
    return typed structs with ids, so the agent composes in Go rather than
    writing a query language.

12. **LLM-built graphs need canonicalisation and a schema gate, or they
    drift.** Surveys of LLM knowledge-graph construction report duplication
    of surface forms, overly generic terms, hallucination and omission, and
    propose canonicalisation or schema-definition steps (EDC) [snippet,
    https://arxiv.org/pdf/2404.03868, https://arxiv.org/pdf/2502.05239]. I
    found no study that measures schema drift over time (6 of the
    unverified list). Design change: extraction returns only registered
    edge types and candidate entity ids from the index; free-text relations
    go to a review queue, not the graph.

13. **Treat people as the high-risk kind.** Same-name conflation is the
    documented failure for person items (5), and I found no news-outlet
    document describing privacy rules for automatically built entity pages;
    that gap is itself a finding (see "could not verify"). Design change:
    person entities that are not public figures get no entity page, no
    cross-story dossier edge and no query result beyond the story that names
    them, and a person merge needs stronger evidence than other kinds
    (conservative default, owner can lift it).

14. **Corrections and updates are versions of an event or a story, with
    `follows_up` and `supersedes` edges, not new objects.** BBC storylines
    use `hasUpdate`, `hasDevelopment` and `follows` slots [opened]; NewsML-G2
    news items are updated as the story progresses [snippet,
    https://iptc.org/std/NewsML-G2/2.21/specification/NewsML-G2-2.21-spec-CCL_1.pdf].
    Design change: "what is new since" is the diff of claims on an event
    since a cursor; a correction is a new claim that invalidates an old one
    (4), with the story that issued it as provenance.

15. **One identity space: every thing (capture, story, event, entity, claim)
    is an object with a stable id, and embeddings are an attribute of an
    object, not a separate store keyed differently.** No source states this
    as a rule; it follows from schema.org's single `Thing`/`CreativeWork`
    root and `@id`, from Wikidata's one QID space for items and statements,
    and from Graphiti keeping episodes, entities and facts in one graph with
    episodic edges back to sources [opened]. This is my inference, not a
    published finding.

## 1. Systems and standards

### NewsReader, SEM, GAF
- NewsReader reads news in four languages and builds event-centric RDF
  graphs: who, what, where, when [snippet; project materials at
  https://cordis.europa.eu/docs/projects/cnect/4/316404/080/deliverables/001-NewsReaderD21.pdf,
  approx. 2013 to 2016; link as returned by search].
- SEM (Simple Event Model, van Hage et al., about 2011 [recall]) represents
  an event instance with actors, place, time and type. NewsReader extended it
  with GAF so each instance links to its textual mentions, with identity
  across mentions within and across documents; statements are grouped in
  named graphs by provenance so conflicting claims coexist [snippet].
  A source-perspective layer (factuality, sentiment) is described under the
  name GRaSP; acronym use varies between papers [snippet].
- Vossen and Cybulska, "Identity and Granularity of Events in Text" (2017):
  event identity is a function of component similarity; the same action with
  different participants, roles or time is a different event; event
  instances are distinguished from mentions; the "bag of events" method
  builds instances per article from all its mentions and compares instances
  across articles; the parameters decide granularity [opened,
  https://arxiv.org/pdf/1704.04259]. It performs "close to state of the art"
  on cross-document coreference per its abstract.
- What it tells us: the mention/instance split is the workable core. The
  full NLP stack (SRL, coreference, temporal) was heavy; I found no
  evidence in these sources of a production news-organisation deployment
  [not verified].

### EventKG
- Multilingual event-centric temporal KG from Wikidata, DBpedia (five
  languages), YAGO, Wikipedia Current Events and event lists; over 690
  thousand events and 2.3 million temporal relations in V1.1; canonical
  representation based on SEM; SPARQL endpoint [snippet,
  https://arxiv.org/pdf/1804.04526, ESWC 2018]. I extracted the PDF text and
  saw: sub-event relations are first-class (it lists 2,816 sub-events of
  World War II) and event popularity and relation strength are stored as
  attributes [opened, partial read].
- Wikidata finding it reports: only 33% of Wikidata events have temporal
  information and 11.7% spatial information [snippet]. So even the largest
  open event vocabulary is sparse on time and place.

### GDELT
- Three tables in GDELT 2.0: Events, Mentions, GKG; GKG links people,
  organisations, locations, themes, counts [snippet,
  https://www.gdeltproject.org/data.html, https://blog.gdeltproject.org/?p=57].
- CAMEO coding is automated by dictionaries. Quality: a 2025 manual check of
  400 records gave about 55% key-field accuracy (56% English, 53% other)
  [snippet, https://www.mdpi.com/2306-5729/10/10/158]; a deduplication
  method was proposed because events are over-counted [snippet,
  https://ideas.repec.org/a/spr/jcsosc/v8y2025i3d10.1007_s42001-025-00409-4.html];
  the UK statistics office said it was unclear whether any quality
  assurance exists and called it experimental [snippet,
  https://www.ons.gov.uk/peoplepopulationandcommunity/birthsdeathsandmarriages/deaths/methodologies/globaldatabaseofeventslanguageandtonegdeltdataqualitynote].
  A Virginia Tech paper reports similar reliability problems for GDELT and
  ICEWS across event categories [snippet,
  https://people.cs.vt.edu/~naren/papers/1502.full.pdf].
- Lesson: one article produces many event rows, there is no story-level
  event identity, and a fixed code book caps what can be said. The 55% figure
  comes from one group; check the method before quoting it.

### Wikidata and Wikinews
- Statement model: main property-value, qualifiers, references, rank
  (preferred, normal, deprecated) [snippet,
  https://www.mediawiki.org/wiki/Wikibase/DataModel/Primer]. Qualifiers
  appear on about 20% of statements in one analysis [snippet].
- Events are items whose class is a subclass of event; there is no property
  that separates newsworthy events from others; Wikidata event classes are
  fine-grained and many share attribute sets [snippet,
  https://arxiv.org/pdf/1904.05557, 2019]. One benchmark used Wikinews links
  to collect event instances [snippet, https://arxiv.org/abs/2312.02334].
  I did not open Wikidata's own event modelling pages.

### Event Registry
- Online clustering per language: articles join a cluster above a similarity
  threshold; clusters expire when the oldest article is over four days old;
  cross-lingual similarity from Wikipedia-trained multilingual
  representations pre-selects candidate clusters to link across languages
  [snippet, https://arxiv.org/pdf/1512.07046 (JAIR 2016) and the WWW 2014
  demo paper, https://archives.iw3c2.org/www2014/proceedings/companion/p107.pdf].
- Lesson: the production definition of "event" is "a cluster of articles in a
  time window". Simple, cheap, and the four-day expiry is a stated choice.

### Europe Media Monitor / JRC
- Clusters about 30,000 daily reports into top stories in 13 languages and
  links across languages; later about 175,000 articles per day; event
  extraction for violent and natural-disaster events that exploits the
  clusters "at all stages", i.e. cluster first, extract second [snippet,
  https://publications.jrc.ec.europa.eu/repository/handle/JRC46850, JRC32872;
  2006 to 2013]. I did not open them.

### BBC storyline ontology and News Labs
- Storyline is "the editorial view on events, not the written article"; Event
  is "an un-disputable real world event"; Storyline contains slots
  (chapter, development, update) that contain Events or other Storylines;
  `follows`, `index`, `time` order them; Storyline has `topic` and
  `attributedTo` because storylines are subjective; the ontology has no
  article class [opened, https://iptc.org/thirdparty/bbc-ontologies/storyline.html].
  Published 2013, PURL purl.org/ontology/storyline [snippet,
  https://mattshearer.co.uk/2013/05/01/news-storyline-ontology-published-open-data-standards/].
  A 2014 talk and a local-news pilot with journalists tagging storylines are
  described [snippet]. I did not find current use; it may have lapsed.
- Lesson: the cleanest published statement of happening vs storyline vs
  report. Storylines were human-curated; no evidence of automatic
  construction.

### Reuters, PermID, NYT, AP
- PermID: open permanent identifiers for organisations, instruments, people,
  released 2015 with Open Calais tagging; identifiers do not change and sit
  alongside tickers, ISINs, LEIs [snippet,
  https://a-teaminsight.com/blog/thomson-reuters-details-permid-as-part-of-open-platform-strategy,
  https://www.lseg.com/en/data-analytics/products/permid-data-management/].
- NYT: indexers' vocabulary used since 1851, announced as TimesTags in 2008;
  linked open data released from 2009 (about 10,000 headings by 2010) as SKOS
  datasets for people, organisations, locations, descriptors [snippet,
  https://old.datahub.io/id/dataset/nytimes-linked-open-data]. Current API
  status unchecked.
- AP: taxonomy created 2006 and sold separately from tagging; upkeep is
  manual, so a concept nobody adds never appears in metadata; entity
  extraction with TF-IDF ranking [snippet, https://poynter.org/?p=222187
  (2013) and https://associatedpresscapstone.github.io (2018 student
  project; not an AP source)].
- Lesson: stable, curated ids, and the stated cost is human curation upkeep.
  I found no public detail on how any of these resolve merges.

### IPTC
- NewsML-G2 is the exchange XML; EventsML-G2 covers event data and coverage
  planning; Concept Items describe entities, concepts and events; Media
  Topics is about 1,200 terms in a hierarchy; rNews is a separate RDFa and
  Microdata vocabulary that fed schema.org [snippet,
  https://iptc.org/standards/newsml-g2, https://iptc.org/std/NewsML-G2/guidelines/].
  A newsItem "conveys news about a newsworthy event" and is updated as the
  story progresses [snippet, spec CCL_1 PDF above]. No dedicated "story"
  class found. Latest version number unverified (2.35/2.36 seen).

### schema.org
- `mentions`: "Indicates that the CreativeWork contains a reference to, but
  is not necessarily about a concept"; domain CreativeWork, range Thing
  [opened, https://schema.org/mentions]. `about` is the subject; `subjectOf`
  is its inverse (CreativeWork or Event) [snippet, https://schema.org/Thing].
  NewsArticle inherits both through Article and CreativeWork. Event can be
  the value of `about`. This is the cheapest standard vocabulary for the
  report-to-event edge types.

### Washington Post, Guardian, Bloomberg
- No Post or Guardian engineering write-up on entity or event systems
  surfaced [snippet]. Bloomberg published a 2021 WSDM paper on contextualising
  trending entities using Personalized PageRank and entity embeddings
  [snippet, https://www.bloomberg.com/company/wp-json/wp/v2/posts/20596].
  Not verified further.

## 2. Modelling questions

- **What is an event.** Four answers: a code row (GDELT), a cluster of
  articles in a window (Event Registry, EMM), a stable instance grounded by
  mentions (NewsReader), an editorially asserted undisputed happening inside
  a storyline (BBC). The first two are cheap and automatic; the last two keep
  more structure and need either heavy NLP or humans.
- **Granularity.** Vossen and Cybulska: granularity is the same parameter as
  identity [opened]. EventKG treats sub-events as a relation, with thousands
  under one war [opened, partial]. BBC nests storylines. Implication: store
  `part_of` and `same_as` separately and let the view pick the level.
- **Ongoing events.** Event Registry's four-day cluster expiry is a window
  hack [snippet]. BBC uses a storyline with chapters and updates [opened].
  Zep uses validity intervals on facts [opened]. For an ongoing event,
  represent the event as long-lived and its state as time-valid claims.
- **Story to event.** schema.org: `about` or `mentions` [opened]. NewsReader:
  mentions ground instances [snippet]. GDELT: a Mentions table with one row per
  article-event pair [snippet]. All are many-to-many with a typed edge.
- **Entity identity and merge.** Link to QIDs where possible, cluster the
  rest as NIL [snippet]; Zep resolves by embedding plus full-text candidates,
  then an LLM duplicate decision that also writes the fuller name and summary
  [opened]. Wikidata keeps duplicates and conflations, fixed by human
  merges [snippet].
- **Provenance and claims.** Wikidata qualifiers and references and rank
  [snippet]; RDF-star lets a triple be the subject of another statement but
  is a community-group draft, with RDF 1.2 in the W3C process as of March
  2025 [snippet, https://www.w3.org/2021/12/rdf-star.html,
  https://www.w3.org/TR/2025/WD-rdf12-concepts-20250327/]; PROV-O is a W3C
  Recommendation (2013) with Entity, Activity, Agent [snippet,
  https://www.w3.org/TR/prov-o/]. In a property graph, a claim is simply an
  edge with properties, which is the cheapest of these.
- **Contradiction.** Named graphs per source (GAF) [snippet]; rank
  (Wikidata) [snippet]; LLM-judged invalidation with time (Graphiti)
  [opened]. None solves disagreement about facts that are not time-ordered
  (two outlets, same hour); only the named-graph style keeps both.
- **Roles.** FrameNet, PropBank, ACE, ERE and RAMS are fixed role schemas;
  I did not open sources on them this session [recall]. News systems that
  survived used loose roles (actor, place, time) as in SEM. Open schemas
  trade query precision for coverage.
- **Time and place.** EventKG: only 33% of Wikidata events have time, 11.7%
  place [snippet]. Zep: two time axes [opened]. BBC: `time` interval on
  slots [opened].

## 3. Techniques, current state

- **Streaming clustering.** Miranda et al. 2018: online, growing cluster
  count, monolingual and cross-lingual, German, English, Spanish, code
  public (priberam/news-clustering) [opened abstract, snippet for code].
  Later: story identification as distinct from topic clustering, grouping by
  events, places, people with a retrieval encoder [snippet,
  https://arxiv.org/html/2508.08272v1]; LLM-embeddings plus clustering for
  GDELT events [snippet, https://arxiv.org/abs/2406.10552]; stream grouping
  comparison where embeddings fragment events and LLM refinement is cleanest
  [snippet, https://arxiv.org/pdf/2512.03389]; a 2026 benchmark of news
  streams (605 events, 15,354 documents) where current LLMs struggle
  [snippet, https://arxiv.org/pdf/2603.19250].
- **ECB+ and the wild.** Same-lemma baseline gets 62 to 76.5 CoNLL F1
  depending on setting; ECB+ has low lexical diversity; most models were
  built and tested only on it; dataset definitions differ [snippet,
  https://arxiv.org/pdf/2603.00621, https://arxiv.org/pdf/2602.17424, 2026].
  Inference: ECB+ rewards topic similarity, and news in the wild has many
  near-topics (same type of event at a different place or time), which is
  exactly Vossen and Cybulska's counter-example [opened].
- **Entity linking.** BLINK (bi-encoder plus cross-encoder), GENRE
  (generates entity names), ReFinED (about 6 times faster than the BLINK
  bi-encoder, 60 times faster than the cross-encoder, uses types and
  descriptions, works zero-shot) [snippet, https://arxiv.org/pdf/2207.04108,
  2022]. NIL: TAC-KBP NIL clustering since 2011, NASTyLinker (reports 87%
  linking and 90% NIL accuracy on a Wikipedia-list task), BLINKout [snippet,
  https://arxiv.org/pdf/2303.04426v2.pdf, https://arxiv.org/abs/2302.07189].
  I did not find a news-specific evaluation of LLM-era NIL clustering.
- **LLM-built KGs.** GraphRAG builds an entity graph then pre-generates
  community summaries; it wins on global sensemaking judged by an LLM
  [snippet, https://arxiv.org/pdf/2404.16130]. LightRAG: dual-level
  (entity and theme) retrieval and incremental update [snippet,
  https://arxiv.org/html/2410.05779v3]. HippoRAG: LLM-extracted KG,
  Personalized PageRank from query concepts, 10 to 30 times cheaper than
  iterative retrieval at similar quality [snippet,
  https://arxiv.org/abs/2405.14831]. Graphiti/Zep: episodes, entities with
  facts, communities by label propagation; its own limits include community
  drift under incremental update, single-session regressions, and that
  ontologies were left to future work [opened].
- **When a graph beats vectors.** GraphRAG-Bench [opened]: lookups parity,
  reasoning and summarisation favour graphs, token cost can be hundreds of
  times higher for global search; results are one model and the authors' own
  benchmark. Earlier work reported GraphRAG 13.4% below vanilla on Natural
  Questions and a 4.5% gain at 2.3 times the latency on HotpotQA [snippet,
  from the GraphRAG-Bench search summary; sources not opened]. Han et al.:
  the two are complementary, hybrid selection or integration helps [snippet,
  https://arxiv.org/html/2502.11371v2]. Microsoft's BenchmarkQED: each
  method wins on the query class it was designed for; DRIFT (both) strongest
  overall [snippet, https://www.microsoft.com/en-us/research/?p=1140429].

## 4. Ontology strategy

- Fixed code book (CAMEO via GDELT): cheap to apply, 55% key-field accuracy
  in one audit, bounded expressiveness, duplicates [snippet].
- Curated vocabularies (NYT, AP, PermID, IPTC Media Topics): reliable,
  human-maintained; AP's own coverage depends on manual upkeep [snippet].
- Pragmatic web vocabulary (schema.org): few relations, wide adoption;
  `about`/`mentions` is enough for story-to-thing edges [opened].
- Wikidata as vocabulary and id space: wide, with sparse event time and
  place (33% and 11.7%) and conflation problems for people [snippet].
- Emergent schema: EDC-style define-then-canonicalise reduces redundancy
  but I found no drift measurement [snippet].
- Property graph vs RDF: no head-to-head news study found. Practical point
  from the sources: Graphiti (Neo4j, property graph) puts times and
  provenance as edge properties and uses fixed write queries [opened]; RDF
  needs reification, named graphs or RDF-star for the same, and RDF-star
  was not a W3C standard when checked [snippet]. For a PostgreSQL object
  graph with typed edges, edge properties are the native fit.
- DBpedia vs Wikidata and "ontology first" failures: I did not open sources
  on these; the only evidence gathered is the CAMEO and AP-upkeep cost above.

## 5. Query surfaces for agents

- Text-to-SPARQL/Cypher: CypherBench, 11 property graphs, over 10,000
  questions: GPT-4o 60.2% execution accuracy, no model under 10B above 20%
  [snippet, https://arxiv.org/html/2412.18702v2]. LLM-KG-Bench: syntax is
  easy, wrong URIs from hallucination are the main error, semantically
  correct SELECTs are hard [snippet]. SM3-Text-to-Query: SPARQL gained least
  from the technique tested [snippet]. Several 2026 papers on Cypher
  recovery and multi-turn queries appeared only as snippets with unusual ids.
- Tool-style traversal: Think-on-Graph (ICLR 2024) alternates relation and
  entity exploration with LLM pruning under a beam; training-free; state of
  the art on 6 of 9 datasets at the time [snippet,
  https://arxiv.org/abs/2307.07697]. KG-Agent uses a toolbox over a KG
  executor [snippet, https://arxiv.org/pdf/2402.11163].
- Code as query: Code-on-Graph generates executable code over schema-level
  abstractions with execution feedback [snippet,
  https://arxiv.org/html/2606.03705]; a harness where agents write code over
  node and edge files reports gains from multi-turn program revision
  [snippet, https://arxiv.org/pdf/2610.18616]; GraphWalk argues for few
  generic operations [snippet, https://arxiv.org/html/2604.01610]. These
  are recent and I could not check their results; the direction fits an
  in-REPL Go cell but is weak evidence.
- What the sources agree on (all snippet-level except Graphiti): fewer,
  generic, typed operations; ids not strings; execution feedback; bounded
  hops; hybrid retrieval. Hybrid in one call is what Graphiti does
  (semantic, keyword, graph traversal) [snippet, https://github.com/getzep/graphiti].
- Datalog/Datomic/GraphQL for agents: no evidence gathered. Not covered.

## 6. What news organisations use the layer for

- Deduplication and consolidation: Event Registry and EMM exist for this,
  with clusters as the product [snippet]. Vossen and Cybulska cite
  deduplication and showing differing perspectives as the point of event
  coreference [opened].
- Storylines as versions: BBC slots for update and development [opened].
- Related coverage: shortest entity distance in a KG between articles as a
  relatedness measure [snippet, https://arxiv.org/pdf/1905.13132v1.pdf]; a
  Neo4j walk-through links entities in articles to Wikipedia and stores them
  [snippet].
- Entity pages and tagging: NYT linked data topics and AP taxonomy [snippet].
- Corrections: no source gathered describes how a news KG handles
  corrections beyond versioned items in NewsML-G2 [snippet].
- Privacy and dossiers: **no outlet policy found.** A search for newsroom
  rules on automated person pages returned only generic site privacy
  policies [snippet]. I did not search the right-to-be-forgotten case law or
  outlet unpublishing policies in time; those are known to exist [recall] but
  are not evidenced here.
- Wrong merges: documented for Wikidata persons (conflation of homonyms)
  [snippet]. Stale facts: addressed in design by validity intervals (Zep)
  [opened], not measured in news.

## Table: modelling questions and each system's answer

| Question | GDELT | NewsReader (SEM+GAF) | Event Registry / EMM | BBC storyline | Wikidata / EventKG | schema.org / IPTC | Graphiti / Zep |
|---|---|---|---|---|---|---|---|
| What is an event | Coded row per actor pair and article | Instance with mentions | Cluster of articles in a window | "Undisputed" real-world Event inside a Storyline | Item of an event class; EventKG canonicalises with SEM | Event (schema.org); Concept Item (NewsML-G2) | No event type; fact edges with times |
| Granularity / sub-events | Code hierarchy only | Parameter of identity | Cluster threshold | Nested storylines | sub-event relation, thousands under one war | Event `subEvent` [recall] | Not addressed |
| Story to event | Mentions table | Mention to instance | Cluster membership | Not defined (no article class) | Not defined | `about` / `mentions` | Episode to entity edges |
| Entity identity | Name strings | Linked to DBpedia [recall] | Wikipedia concepts | Topic via domain model | QIDs; duplicates and conflations occur | `sameAs` | Embedding plus LLM dedupe |
| Provenance | Source URL only | Named graphs, mention offsets | Article list | `attributedTo` | Statement references | `citation`, `isBasedOn` [recall] | Episodes retained, edge to source |
| Contradiction | None | Per-source graphs | None | Subjective storylines are attributed | Rank and qualifiers | None | LLM-judged invalidation |
| Time validity | Event date | Time on event | Article date, 4-day expiry | Interval on slot | Qualifiers; sparse (33%) | `startDate` / `endDate` | Bi-temporal on edges |
| Update / correction | New rows | Re-read | Cluster grows | `hasUpdate`, `follows` | Edit history | newsItem versions | Invalidate and add |

Cells marked [recall] were not checked this session.

## What failed and why

1. **Fixed code books on automated coding (CAMEO/GDELT):** about 55% key-field
   accuracy in one 400-record audit, duplicates from one article to many
   rows, no quality assurance, no event identity across stories [snippet].
2. **Benchmark-led event coreference:** lemma-match baseline close to trained
   models on ECB+ because of low lexical diversity; models tuned to it
   generalise poorly across datasets [snippet].
3. **Embedding-only stream grouping:** fragments one event into several
   clusters; LLM refinement fixes it at higher cost [snippet].
4. **Graph retrieval used by default:** parity or loss on lookups, very high
   token cost for global search [opened].
5. **LLM-extracted triples without canonicalisation:** duplicate surface
   forms, generic terms, hallucination and omission [snippet].
6. **LLM-written queries:** about 60% execution accuracy on full-scale graphs
   for the best model tested; hallucinated URIs [snippet].
7. **Curated taxonomies:** coverage bound by manual upkeep (AP) [snippet];
   BBC storylines were hand-tagged and I found no evidence of scale [snippet].
8. **Entity merging:** homonym conflation and duplicate items persist even in
   the largest curated open KG [snippet].
9. **Incremental community summaries:** drift from a full recompute, so
   periodic refresh is needed [opened, Zep paper].
10. **Zep's own evaluation:** on its own tests it regressed on single-session
    assistant questions (by 17.7% for one model) and could not run MemGPT on
    LongMemEval; DMR is small enough to fit in context [opened].

## Source list

Opened this session (full or relevant part read):
- Zep: a temporal knowledge graph architecture for agent memory, 2025,
  https://arxiv.org/html/2501.13956 (abstract page https://arxiv.org/abs/2501.13956)
- Multilingual clustering of streaming news, EMNLP 2018 (abstract only),
  https://arxiv.org/abs/1809.00540
- Vossen and Cybulska, Identity and granularity of events in text, 2017,
  https://arxiv.org/pdf/1704.04259
- When to use graphs in RAG (GraphRAG-Bench), 2025 to 2026 (first 100k
  characters), https://arxiv.org/html/2506.05690v3
- schema.org `mentions`, https://schema.org/mentions
- BBC Storyline Ontology (IPTC mirror), https://iptc.org/thirdparty/bbc-ontologies/storyline.html
- EventKG, ESWC 2018, https://arxiv.org/pdf/1804.04526 (text extracted,
  read only by keyword search)

Seen only in search summaries (primary page not opened):
- GraphRAG, 2024, https://arxiv.org/pdf/2404.16130
- RAG vs GraphRAG, 2025, https://arxiv.org/html/2502.11371v2
- BenchmarkQED, https://www.microsoft.com/en-us/research/?p=1140429
- HippoRAG, NeurIPS 2024, https://arxiv.org/abs/2405.14831
- LightRAG, 2024 (EMNLP Findings 2025), https://arxiv.org/html/2410.05779v3
- Think-on-Graph, ICLR 2024, https://arxiv.org/abs/2307.07697
- KG-Agent, 2024, https://arxiv.org/pdf/2402.11163
- GraphWalk, https://arxiv.org/html/2604.01610; Code-on-Graph,
  https://arxiv.org/html/2606.03705; DKGI, https://arxiv.org/pdf/2610.18616
- CypherBench, 2024, https://arxiv.org/html/2412.18702v2
- Extract-Define-Canonicalize, 2024, https://arxiv.org/pdf/2404.03868;
  KG construction hallucination study, 2025, https://arxiv.org/pdf/2502.05239
- ReFinED, NAACL 2022, https://arxiv.org/pdf/2207.04108
- NIL clustering: https://arxiv.org/pdf/2102.10962 (2021),
  https://arxiv.org/pdf/2303.04426v2.pdf (2023), https://arxiv.org/abs/2302.07189
- ECB+ and CDCR: https://arxiv.org/pdf/2603.00621, https://arxiv.org/pdf/2602.17424 (2026)
- News Across Languages (Event Registry line), JAIR 2016,
  https://arxiv.org/pdf/1512.07046; WWW 2014 demo,
  https://archives.iw3c2.org/www2014/proceedings/companion/p107.pdf
- Stream grouping and story identification: https://arxiv.org/pdf/2512.03389,
  https://arxiv.org/html/2508.08272v1, https://arxiv.org/abs/2406.10552,
  https://arxiv.org/pdf/2603.19250
- GDELT quality: https://www.mdpi.com/2306-5729/10/10/158 (2025),
  https://people.cs.vt.edu/~naren/papers/1502.full.pdf, UK ONS data quality
  note (link above); GDELT docs https://www.gdeltproject.org/data.html
- NewsReader SEM/GAF: https://cris.fbk.eu/retrieve/ddb241a5-4052-ba8a-e053-3a05fe0afd55/1-s2.0-S0950705116302271-main.pdf
- Wikibase data model primer, https://www.mediawiki.org/wiki/Wikibase/DataModel/Primer
- Wikidata conflations and duplications (SNS), link in finding 5
- Wikidata events and news, 2019, https://arxiv.org/pdf/1904.05557
- RDF-star https://www.w3.org/2021/12/rdf-star.html; RDF 1.2 draft
  https://www.w3.org/TR/2025/WD-rdf12-concepts-20250327/; PROV-O
  https://www.w3.org/TR/prov-o/ (2013)
- IPTC NewsML-G2 https://iptc.org/standards/newsml-g2; BBC storyline
  announcement 2013 (link above)
- PermID https://www.lseg.com/en/data-analytics/products/permid-data-management/
- NYT linked data https://old.datahub.io/id/dataset/nytimes-linked-open-data
- AP: https://poynter.org/?p=222187 (2013); student project (not AP)
- JRC EMM: https://publications.jrc.ec.europa.eu/repository/handle/JRC46850
- Bloomberg trending entities (WSDM 2021),
  https://www.bloomberg.com/company/wp-json/wp/v2/posts/20596
- Graphiti repository, https://github.com/getzep/graphiti

## What I could not verify

- Whether the 2026-range arXiv papers say what the snippets say (ids
  2603.00621, 2602.17424, 2603.19250, 2604.01610, 2606.03705, 2610.18616,
  and the CypherTurn and LAST-CQ ids). The ids looked unusual; open before
  citing.
- NewsReader's full architecture, scale and any production use; the
  acronym GAF vs GRaSP.
- Wikidata's own event modelling pages, Wikinews structure, and the
  `Event` property list in schema.org (sub-events, `startDate`) beyond
  recall.
- Event Registry's current system; EMM details beyond JRC titles.
- NYT, AP, Washington Post, Guardian and Bloomberg entity systems: no
  primary engineering documentation found for the Post and Guardian; NYT
  and AP details are from 2008 to 2013 posts.
- Role schemas (FrameNet, PropBank, ACE, ERE, RAMS) and the evidence on
  fixed vs open role schemas.
- DBpedia vs Wikidata comparisons, "ontology first" failure case studies,
  property graph vs RDF comparisons, Datalog/Datomic and GraphQL for
  agents.
- Newsroom privacy rules for people entities, corrections handling in
  knowledge graphs, and stale-fact measurement in news.
- Any measurement of schema drift in LLM extraction over time.
- Whether the GraphRAG-Bench results hold with models other than
  GPT-4o-mini, and the last 82k characters of that paper.
- Finding 15 (one identity space) is my inference from several systems,
  not a published result.
