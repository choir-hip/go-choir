---
definition_version: 7
readiness: drafted
review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none
metamission:
  stations:
    - id: X
      path: docs/definitions/choir-autoradio-exact-cutover-2026-11-18.md
      readiness: executable
      status: working
      depends_on: []
    - id: V
      path: docs/definitions/choir-night-exact-vectors-learning-2026-11-19.md
      readiness: drafted
      status: working
      depends_on: []
    - id: L
      path: docs/definitions/choir-night-exact-vectors-learning-2026-11-19.md
      readiness: drafted
      status: working
      depends_on: []
start:
  captured_at: 2026-11-19T05:20Z
  source:
    canonical_ref: origin/main af0adc837
    deploy_identity: staging deployed_commit d2c8bfdd0 (deployed 2026-11-19T02:19Z); the corpus brief is pushed at bfa898f8d with CI run 44462773656 in progress
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: user_wip
      owner: the owner
      touch: forbidden
      recovery: one untracked file that holds secrets (.env.bak-bedrock-20261109124037); leave in place, never read, never commit here
    - path: .lane/trees/qa-mac
      status: clean
      class: goal_candidate
      owner: the lead
      touch: goal_owned
      recovery: the one Mac build tree; at lane/autoradio-integ23 a0671d74c, the build on the owner's iPhone since 2026-11-19T05:01Z
    - path: .lane/trees/exact-loop and .lane/trees/exact-paper
      status: unknown
      class: goal_candidate
      owner: builders of 2026-11-18
      touch: goal_owned
      recovery: branches lane/exact-loop 04626c0ee and lane/exact-paper 7f8617765 on origin; both predate the phone work landing on main and need a rebase; neither is measured (ledger C125, C126)
    - path: .lane/trees/csl-land
      status: clean
      class: goal_candidate
      owner: the lead
      touch: goal_owned
      recovery: branch lane/csl-land on origin; in the landing script at capture time
    - path: .lane/trees/search-ceramic
      status: unknown
      class: other_agent_wip
      owner: held for the owner
      touch: forbidden
      recovery: branch lane/search-ceramic on origin; kept by the owner's word of 2026-11-19
finish:
  deliver: 'By the morning of 2026-11-19 (13:00Z): the Autoradio app''s logic outside voice is shared Exact code as far as it can be proved without the owner awake; research on the paper searches its own corpus by meaning as well as by words, from pgvector, with stories linked to the pages they cite; and every recurring agent failure seen this week has a trace-backed cause and the calibration experiment is running.'
  artifact: 'three things on main and staging. (X) the stations of the cutover file landed in order, each with its Swift deleted, an iPhone build installed and an Android emulator tour. (V) pgvector in Node B''s Postgres holding an EmbeddingGemma 2 vector for every published story version and one for every captured page, an embedder on Node B that keeps them current, and a corpus search that fuses vectors with full-page BM25 and walks from a story to its cited pages. (L) a problem record per recurring failure with its cause classed as the model''s or the harness''s, a redesigned calibration experiment E1, and E1 started on the test computers.'
  acceptance:
    - action: 'scripts/check-autoradio-native-boundary on main (built by station XG)'
      proves: how many Swift sources and interface names remain outside the voice list; the count only falls
      evidence_class: gate output in CI
    - action: the Android emulator tour and the iPhone simulator tour for each landed Exact station, signed in as the drill account
      proves: the moved page shows real staging data on both, from one source
      evidence_class: deployed proof read by the lead (trees and screenshots)
    - action: 'on Node B: select count(*) of story vectors and capture vectors against the count of published story versions and public captures'
      proves: coverage of the first pass
      evidence_class: a count read from production, not written by the author
    - action: the 200-question retrieval set of the design (docs/design/corpus-first-retrieval-and-research-2026-11-18.md) run against the deployed fused search
      proves: recall at ten of 0.90 or more, and pages not in English in the top ten for 65% or more
      evidence_class: deployed proof
    - action: python3 -I scripts/status-corpus-reuse on the paper after each deploy
      proves: web searches a run, the share of reads from held pages, the brief's outcomes
      evidence_class: production counts (ledger C141, C142, C143 and the claims this file adds)
    - action: the run ledgers named in each trace record
      proves: the cause stated for each recurring failure
      evidence_class: trace evidence, read not inferred
  rollback: each landing reverts as one. Vectors are a derived index (drop the tables and stop the embedder unit; BM25 search is untouched). Each Exact station reverts as one landing and the phone rolls back by installing the build before it. E1 ends by clearing its arm settings on the test computers.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
value:
  better_means: 'three gaps close. The share of the app''s logic that exists once and runs on both phones rises to all of it outside voice. The share of research that stands on what the computer already holds rises, measured as web searches a run and reads from held pages. The share of recurring failures with a known cause rises to all of them.'
  goodharting_would_be: 'Swift deleted with the behaviour it carried gone untested, or a station called landed with no build on a device. Vectors stored and counted but never used by a search a desk makes, or a recall figure taken on questions written from the indexed text. A cause written for each failure from reading code and not the run''s ledger, or a prompt changed to make one bad run look better. An experiment started with arms nobody reads.'
homotopy:
  realism_axis: 'How much of the real system each track runs on, on how real a subject: from one page on the web host with a canned fetch, to every page on the owner''s locked iPhone; from 449 trial passages on the Mac, to every published story and every captured page on Node B answering the paper''s own research runs; from one loop read in one run''s ledger, to an experiment whose arms run on five computers for days.'
boundaries:
  mutation_class: red
  authority_sources:
    - the owner's words of 2026-11-19 (owner_rulings below)
    - CLAUDE.md
    - docs/choir-doctrine.md
    - docs/definitions/choir-autoradio-exact-cutover-2026-11-18.md (track X)
    - docs/design/corpus-first-retrieval-and-research-2026-11-18.md (track V, as amended by ruling R3)
    - docs/reference/EMBEDDINGGEMMA.md, docs/reference/EXACT.md, docs/reference/CAPTURE-FULLTEXT.md
  must_preserve:
    - the owner is asleep and nothing waits for the owner; a step only the owner can do is recorded and the work goes round it
    - the owner's iPhone is never launched tonight (the app speaks at launch); builds are installed, not run
    - the owner's iPhone stays signed in; a build that would sign it out is not installed
    - 'no switches: a station replaces and deletes; vectors and fusion ship on'
    - no limits on our own model calls; the embedder refuses and counts nothing
    - owner-private captures are never exported, embedded or indexed
    - captured text copied to the Mac is public captures only and is deleted after the import
    - nothing is done on the paper or the owner's computer except reading; drills run on d@e
    - one landing at a time; one Mac native build at a time through the lock; every simulator slim, one at a time
    - no prompt or harness change for a failure whose cause has not been read from a trace
  excluded:
    - TestFlight (next week, by the owner's word)
    - whole-page passage vectors for every capture (until a measurement asks for them)
    - any change to search providers or spending; the Ceramic branch is kept and not landed
    - anything sent upstream to Exact
  protected_surfaces:
    - 'nix and Node B host services (pgvector in Postgres, the embedder unit)'
    - corpusd's schema and its restore set
    - 'auth and session on the phone (cutover station X3)'
    - 'run acceptance and evidence: the ops the phone posts (cutover stations X4, X5)'
    - story export and the tape (station V6)
    - what a round records for calibration (track L)
stations:
  - id: V0
    name: The corpus brief and the faster corpus search, landed and measured
    scope: lane brief-land is on main at bfa898f8d; lane csl-land is in the landing script; read C141, C142 and C143 from the paper and corpusd after each deploy
    class: orange
    review: one seat (done, rounds 7 to 9 for the brief; rounds 1 and 2 for the search)
    depends_on: []
    carrier: main bfa898f8d; lane/csl-land
    status: working
  - id: V1
    name: pgvector and an EmbeddingGemma 2 embedder on Node B
    scope: 'the pgvector extension in Node B''s PostgreSQL 17 (nix/postgresql.nix); a host unit running llama.cpp''s server with the official EmbeddingGemma 2 Q8_0 file (pinned by SHA-256, fetched at activation, never in the repo), four threads, low CPU weight, loopback only, called only by corpusd; a health line. The reference page gains the server section.'
    class: red (nix, a host model service)
    review: two seats
    depends_on: []
    carrier: none yet
    status: pending
  - id: V2
    name: Vector tables and the embed worker in corpusd
    scope: 'two tables in corpusd''s database: one row per passage of every published story version, one row per public capture (title plus lead passage); 256 dimensions at half precision with the model''s hash; an HNSW index; a worker whose work list is derived (rows with no vector, newest first, stories before captures), which rests while searches are in flight and logs remaining=. No queue in memory, no limit, nothing dropped.'
    class: red (corpusd schema; a new writer)
    review: two seats
    depends_on: [V1]
    carrier: none yet
    status: pending
  - id: V3
    name: The first pass, on the Mac
    scope: 'an export of public text from Node B (every story version; each public capture''s title and lead passage), embedded on the Mac with Metal by the trial command (mobile/EmbedTrial), imported as vectors; a sample re-embedded on Node B must agree at cosine 0.999 or more; the text copy on the Mac is deleted after the import. Stories first (minutes), then captures (about six hours).'
    class: yellow (operations)
    review: none
    depends_on: [V2]
    carrier: none yet
    status: pending
  - id: V4
    name: 'Fused search: story vectors, capture vectors, full-page BM25, and the walk from a story to its pages'
    scope: 'corpusd answers one search with reciprocal-rank fusion over the three lists, keeps BM25''s top three, and for a story hit returns the captures that story cites (the object graph''s citation edges). The guest''s own merge is deleted. The corpus brief and choir.SearchWireCorpus read the fused answer.'
    class: orange
    review: one seat
    depends_on: [V2]
    carrier: none yet
    status: pending
  - id: V5
    name: The event layer reads research's pages, with a vector shortlist
    scope: station S7 of the design; a capture joins an existing event by vector shortlist and Jev's judgment, replacing the title-overlap slot
    class: orange
    review: one seat
    depends_on: [V4]
    carrier: none yet
    status: pending
  - id: V6
    name: Stories carry event and entity ids at publish
    scope: station S8 of the design; event ids on export; basis.events wired; the related-story setting and environment switches removed
    class: red (export, tape)
    review: two seats
    depends_on: [V5]
    carrier: none yet
    status: pending
  - id: V7
    name: Revise, do not repeat
    scope: 'stations S9 to S12 of the design: the neighbourhood read; the fold check at first publish (same primary event and revised within 72 hours, by ruling R7); entity merges; the front and the radio deduped by event'
    class: red (crosses Texture custody)
    review: two seats
    depends_on: [V6]
    carrier: none yet
    status: pending
  - id: L1
    name: Read the traces of every recurring failure
    scope: 'read-only. For each: the run''s ledger and console lines, what the model was shown, what it did, and whether the cause is the model''s behaviour or the harness''s. The list: run c0351184 reading one capture 3,090 times; runs that start 17 to 39 web searches; runs that read 6% of the pages a search returns; management opening a story with candidates_shown=0; the store_stalled holder lines; three bugs in corpus search in a week (a clustering assessment). One record each, or a section in the existing record.'
    class: green
    review: none
    depends_on: []
    carrier: none yet
    status: pending
  - id: L2
    name: The calibration experiment, redesigned
    scope: 'docs/design/calibration-experiment-e1-2026-11-04.md rewritten around what L1 finds: what is varied, on which computers, what a round records, which reading settles each arm, and when. One open-ended design pass, then two seats.'
    class: green (design), then red where it changes what a round records
    review: two seats
    depends_on: [L1]
    carrier: none yet
    status: pending
  - id: L3
    name: E1 runs
    scope: the arms set on the test computers by the owner's word of 2026-11-19; the paper and the owner's computer hold no arm; a reader that needs no live session; the switch registry entries become experiment records with an end date
    class: red
    review: two seats
    depends_on: [L2]
    carrier: none yet
    status: pending
  - id: L4
    name: The other dated switches
    scope: the eight registry entries besides E1 that reach 2026-11-25 are each shipped on, deleted, or given to the owner with a default, so main does not go red that day
    class: orange
    review: one seat
    depends_on: []
    carrier: none yet
    status: pending
decisions_for_the_owner:
  - id: D1
    question: Does the sign-in station carry the phone's existing key across, or sign the owner out once?
    why: the owner is asleep and said sign-in must not block; the cutover file's default (D3 there) was to sign in again once
    options: carry the key in place; sign in again once; hold the phone on the last build before that station
    default: carry the key in place; if that cannot be proved on the simulator, hold the phone on the last build before the station and go on with simulator and emulator
  - id: D2
    question: Which computers hold an E1 arm?
    why: 'the 4 November design names five test arms; the paper and the owner''s computer must hold none'
    options: the five test computers of the old design; the drill computer only; a fresh set the redesign names
    default: whatever the redesign names, never the paper, the owner's computer or a real user's
  - id: D3
    question: Do captures get a vector for every passage, or one per page?
    why: 'one per page is 215,000 vectors and six hours on the Mac; every passage is about 850,000 and one to three days'
    options: one per page now, every passage when a measurement asks; every passage now
    default: one per page now (ruling R3 leaves this to the lead's proposal)
owner_rulings:
  - 'on': '2026-11-19'
    decision: R1
    said: and we need to do the full exact rewrite. make a throughline goal doc to get this all done tonight after walking me through the questions/decisions
    read_as: this file; track X is the cutover file, run tonight as far as proof allows
  - 'on': '2026-11-19'
    decision: R2
    said: embeddinggemma2. has to be "2". its recent. [...] we need to get vectors in pgvector, using embeddinggemma2. yes, you can use the mac for embeddings.
    read_as: the model is EmbeddingGemma 2 and is always named so; vectors live in pgvector (the design's flat file is withdrawn); captured text may be copied to the Mac for the first pass
  - 'on': '2026-11-19'
    decision: R3
    said: 'maybe we should embed all of our published stories, with captured sources as metadata that they link to, so we can retrieve our own stories from pgvector then do graph traversal for the captures... could be better than embedding all the captures anyway. thoughts on this? then going forward we should def do embeddings on node b of everything we write and publish, and capture too? or should we continue to only embed our stories, and captures get retrieved based on the stories?'
    read_as: 'stories first, and a story hit walks to the pages it cites. The lead''s answer, not yet ruled on: also one vector per captured page, since a page no story cited cannot be reached through stories; going forward Node B embeds everything published and one vector per new capture.'
  - 'on': '2026-11-19'
    decision: R4
    said: exact first, but note that you can use the builder and lander vms on node-a as well. i think you can get it all done with concurrency
    read_as: track X has first call on the lead's attention and the Mac; tracks V and L run beside it
  - 'on': '2026-11-19'
    decision: R5
    said: not sure. i will be sleep so if signin is a blocker for you we need to work around that. you should be busy, not blocked, for the entire night
    read_as: no step waits for the owner; decision D1 above
  - 'on': '2026-11-19'
    decision: R6
    said: 'we need to review the traces and investigate all the problems eg the 3090 read loop. where we have repeated problems there may be solutions. but note that if the problems are at the model level, naively changing the harness or prompt may not be correct [...] the calibration experiment is something that we should do now, actually. you can redesign it as you see fit.'
    read_as: track L; the owner's word starts E1 once it is redesigned and reviewed
  - 'on': '2026-11-19'
    decision: R7
    said: 'we def need event and entity linking too. [...] the gemma license is fine. [...] dont delete the branch though [...] yes, testflight waits. next week. re stories becoming revisions, sure.'
    read_as: 'stations V5 to V7; the licence question is closed; lane/search-ceramic is kept and not landed; TestFlight is excluded; the revision rule is same primary event and revised within 72 hours'
  - 'on': '2026-11-19'
    decision: R8
    said: Wait a day (Recommended)
    read_as: nothing is bought or capped for web search tonight; the Keenable reading is taken again on 2026-11-20
  - 'on': '2026-11-19'
    decision: R9
    said: ok, get rid of lab pages. and we should draw the mark efficiently. its hand drawn for roughness. no need for pixel perfect. its imperfect by design.
    read_as: 'the lab pages (the voice lab and the motion gallery) are deleted, not moved to a second app; the animated mark is drawn whichever way is cheap, keeping its hand-drawn roughness; no pixel comparison is required of it'
now:
  status: working
  slice: 'three tracks at once. X: the two unmeasured Exact lanes are rebased and measured, the boundary gate is built, and the page stations fan out behind the paper page''s pattern. V: the faster corpus search lands; pgvector and the embedder are built; the Mac starts on stories. L: the traces are read.'
  source_ref: origin/main at the commit that carries this file
  deploy_identity: staging d2c8bfdd0; the owner's iPhone on lane/autoradio-integ23 a0671d74c, installed and not launched
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: N1
    calibration_claim: C144
    claim: If stories and one vector per captured page live in pgvector, fused with full-page BM25 and walked through citations, then a research run finds what the computer holds without asking the web, and web searches a run fall again.
    test: 'the 200-question set against the deployed search (recall at ten 0.90 or more); then on the paper, the first 24 research runs after the fused search deploys: median web searches started 3 or fewer'
    edge: independence
    delta_o: 'the brief''s first hours (C141, C142) say whether handing a model held pages changes what it searches for at all. If they do not, better retrieval will not either, and the lever is in what the desk is asked to do, which track L reads.'
    scope_if_supported: the paper's research desk at today's corpus
    status: active
    evidence_refs: []
  decision:
    what: 'pgvector with EmbeddingGemma 2 at 256 dimensions and half precision; stories in full and one vector per capture; the Mac for the first pass and Node B from then on; Exact stations in the cutover file''s order with pages in parallel; no prompt or harness change before its trace is read'
    kind: architecture
    status: proposal
    evidence_ref: docs/design/corpus-first-retrieval-and-research-2026-11-18.md as amended by R2 and R3
    owner_ratification_ref: 'R2 settles pgvector, the model and the Mac; the dimension, one vector per capture and the order are the lead''s proposal until the owner rules (D3)'
  belief:
    believed_state: 'full-page BM25 is live and every corpus search returns a page; searching for need cut web searches a run from 7 to 4; the corpus brief is on main and not yet deployed; a corpus search takes two seconds and the fix is landing; no vector exists on the server; the Exact move stands where it stood yesterday morning, with two unmerged lanes; nothing links stories by event'
    main_uncertainty: 'whether nixpkgs'' llama.cpp carries the EmbeddingGemma 2 architecture and whether pgvector fits Postgres''s 4 GiB cap with an HNSW index; and for Exact, how much of a red station can be proved with no ears and a phone that must not be launched'
    next_observation: 'the brief''s first thirty journal lines on the paper; the nix evaluation of the embedder unit; the rebased Exact lanes'' loop time'
  blocker_or_risk: 'the builder machine has 4 GB, so Go builders run two at a time; every server landing takes about 75 minutes of suites and they run one at a time, so the night holds five or six; the Mac carries the embedding pass, the Exact loop and the native builds at once; sound and the locked screen cannot be judged until the owner is awake'
  next_action: start the first wave of builders (Exact lanes rebased and measured, the boundary gate, the trace read, pgvector and the embedder) and land the faster corpus search
receipts: []
---

# The night of 19 November 2026: Exact, vectors, and learning

The owner came back after eighteen hours, asked what was done, and set the
night's work in a few sentences (the rulings above). This file orders it. It
is a set of three tracks, not a new design. Track X is the
[cutover from Swift to Exact](choir-autoradio-exact-cutover-2026-11-18.md),
which keeps its own stations. Track V is the
[corpus-first retrieval design](../design/corpus-first-retrieval-and-research-2026-11-18.md)
with two changes the owner made tonight. Track L is new.

## What changed in the retrieval design tonight

The design put vectors in a flat file beside the full-text index and gave
every passage of every page a vector. The owner ruled for pgvector (R2) and
proposed embedding stories, then walking to the pages they cite (R3). The
lead's answer keeps the owner's order and adds one thing: each captured page
also gets a single vector, for its title and opening passage. A page no
story cited cannot be reached through stories, and those pages are the ones
the computer holds without knowing. Full-page BM25 finds them by their words
but not by a paraphrase, and found a third of the pages not in English where
vectors found 72% (the design's measurement).

One vector a page is about 215,000 vectors and 120 MB, which removes the
design's objection to Postgres. Stories in full are a few thousand passages.

## What "tonight" can mean

Three bounds are physical, not choices.

- The Mac embeds about 3,000 tokens a second. Stories take minutes; one
  vector for each of 215,000 pages takes about six hours.
- A server landing takes about 75 minutes of suites, then CI and a deploy,
  and they run one at a time: five or six in the night.
- The lead cannot hear, and the owner's phone must not speak. Sound and the
  locked screen are proved when the owner wakes.

So the honest finish for each track is stated as what lands and is measured,
and what is built and waiting. The morning report says which is which.

## How the tracks share the machines

Exact work runs on the Mac: the web loop in lanes (megabytes), one native
build at a time by the lead. Go work compiles on the builder machine, two
builders at a time, and lands through the larger machine. The embedding pass
runs on the Mac's graphics processor beside both.

## Track L and the rule it carries

A failure that repeats is evidence about something, and the first question
is what. The owner's caution is the operating contract's rule 4 in other
words: where the cause is the model's behaviour, changing the harness or the
prompt may be wrong. Station L1 reads; nothing is changed until it has. The
calibration experiment is the instrument for the changes that follow, which
is why the owner asked for it now and why it is redesigned from what L1
finds and not from the 4 November plan.
