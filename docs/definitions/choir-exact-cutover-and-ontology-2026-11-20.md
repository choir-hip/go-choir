---
definition_version: 7
readiness: executable
supersedes: docs/definitions/choir-exact-search-entities-2026-11-20.md
authority: 'the owner, 2026-11-20 about 22:40Z: "we need to be aggressive with the cutover from Swift to exact, and aggressive with the ontology work for search events and entities, with the right design for the REPL to be queried so that we are more efficiently using our external search provider credits. And I think we should write a new throughline goal mission, and then I''ll set thread so that we''re using it."'
review:
  reviewer: none (the owner's statement is the authority; each station names its own review tier)
  frozen_ref: none
  verdict: none
start:
  captured_at: 2026-11-20T22:42Z
  source:
    canonical_ref: origin/main 79dbca58d (docs); the last code commit is bfc1492c6
    deploy_identity: 'staging runs bfc1492c6 (deployed 16:27Z; the owner''s screenshot of the phone''s About page showed phone and server both at bfc1492c6). The health route does not print the commit; unknown from it.'
  what_is_true_now:
    - 'Exact: non-voice native code on main is 95 files, 18,366 lines, 93 interface names (scripts/autoradio-native-ceiling.txt); it was 119 files, 28,093 lines, 145 names when the measure began on 2026-11-19. Landed in shared code: sign-in, sync, mail, files, podcasts, the Textures list, conversation rows, themes, About; demo mode deleted. Android has no sign-in, audio or voice; the Mac app is not started.'
    - 'Why the cutover stalled (the lead''s reading, 2026-11-20): each page was ported with a carry-over of the Swift build''s stored data, and review sent each back two to four times over the carry-over; and moves ran one or two at a time. Both are now changed by owner ruling (no carry-over) and by running moves in parallel.'
    - 'Search: 3,058 web searches sent by 22:31Z, about 3,260 for the day against the target of 1,000; all Keenable. The need-test holds refuse 30 to 40% of attempts. 90% of research runs are a story''s first run and carry 94% of searches (docs/evidence/research-runs-with-nothing-new-2026-11-20.md). About 800 stories and 5,000 captures a day (assumed from the design brief, not re-read).'
    - 'The semantic layer as built (docs/evidence/semantic-layer-as-built-2026-11-20.md): one registry of 109 object kinds and 86 edge kinds with custodians; facts about the world in at least ten homes and six shapes; events on the paper computer''s own store with no edge to stories, captures or entities in the Autopaper object graph (282 of the 300 newest events have one member; 84% of questions minted an event, 2.8% joined); entity identity is a hash of the normalised name (4 of 6,995 with a Wikidata id; 29% of 200 sampled attachments not entities; nothing merges); a story has four id forms that do not join; an outlet is a string; a research cell has no verb that reads an entity, an event by id, an outlet or an edge. The entity attach path is gated by the setting research.related_story, a switch; whether it is on for the paper is unknown.'
    - 'Design in flight: an Opus pass is writing docs/design/semantic-layer-2026-11-20.md from the ground up under the owner''s brief (stories with a semantic representation; identity as recorded, reversible judgments by vectors, System 1 and RLM agents; a typed Go library in the cell; the consolidation mechanism). Not returned at capture time; nothing red is built on it before the owner has read its opening page.'
    - 'Duplicate stories: a measurement is running (docs/evidence/duplicate-stories-on-the-paper-2026-11-20.md, not yet written). The owner''s hypothesis: overlapping stories on one topic lower the paper''s quality and spend searches.'
    - 'Publishing defects found today, not fixed: twelve stories have not published since 2026-11-08 because their disclosure labels only grow (docs/problems/disclosure-gate-refuses-story-publishes-on-the-paper-2026-11-20.md; the fix is at the label carry, red); 10 to 48 story publishes an hour have failed since at least 2026-11-18 with "502 failed to load private texture revision": the guests revision read passes the proxys 30-second timeout because it scans the whole scopes entities per revision, and a failed publish is not retried (docs/problems/story-publishes-fail-loading-the-private-revision-2026-11-20.md); management replies with no objective wake research desks (docs/problems/research-desks-woken-by-management-replies-loop-2026-11-20.md).'
    - 'Host: Node B 122 GiB free; each paper snapshot needs about 50 GiB for under two hours and took 73 minutes of a 90-minute budget at 18:07Z.'
  branches_in_flight:
    - 'lane/exact-edition-picks 80740057d: sent back twice; being rebuilt with no carry-over (picks start empty; the old file is left unread). One review follows.'
    - 'lane/exact-takes c534f89c0: sent back four times; being rebuilt with no carry-over at all. One review follows.'
    - 'lane/exact-about-build-row 9c8555f7e: the About page''s Build row shows the commit and time (the owner: "We should put the actual build in the phone rather than build one"; the lead''s reading, unconfirmed). Simulator proof running, seat 2 review running.'
    - 'lane/exact-reader, lane/exact-bar-runsheet, lane/exact-mail-files-rest: three builders started 22:35Z; no commit yet. Each lands in slices.'
    - 'lane/search-leads-l2 1e925aa37: the corpus says how strong and recent its best answer is, as a log line with no behaviour change; first review sent it back, repair pushed, the whole suite was running on the build machine; needs re-review and the security trailers.'
    - 'lane/reply-wake-fix 1a8f60f75: a management reply with no objective does not wake a research desk; sent back by the lead to keep waking a desk that is waiting on an Ask; unreviewed.'
    - 'lane/story-events-e5 (no pushed commit): a published story carries its event ids. Red. Not to land until the design confirms its shape.'
    - 'Stopped for the owner, kept: root-desk-r3 7a6d66dc4; snapshot-stream 0873aa1fc; exact-convo-edit 4b58dba83 (the Texture editor); owner-act-enforce ba32aa571; dispatch-deadlines 04a06df51 (failing suite). lane/search-ceramic ad404a072 is kept by the owner''s word.'
finish:
  deliver: 'Everything in Autoradio outside voice is shared Exact code, and the paper knows what it already holds: every story, capture, event, entity and outlet is an object in one identity space that an agent can query from its cell, so the paper builds one deep, growing narrative for each topic that takes in every development as it arrives, where a newspaper would print a new article for each beat, and the web is searched only for what is new.'
  artifact: 'on main and staging: (X) each remaining page''s logic in mobile/AutoradioLab with its Swift deleted and the native ceiling lowered; (O) the unified semantic layer of docs/design/semantic-layer-2026-11-20.md landed station by station, with the typed cell library; (C) story consolidation running on the paper; (P) the publishing defects fixed. On the owner''s phone: a build of main after each group of page moves.'
  acceptance:
    - action: scripts/check-autoradio-native-boundary on main, reported at every hourly tick
      proves: non-voice native files, lines and names; the count only falls from 95 files, 18,366 lines, 93 names; the finish is only what voice and the platform force, each with its reason beside the call
      evidence_class: gate output
    - action: mobile/scripts/proof-real.sh on the lead's simulator against staging as d@e, screenshots read by the lead, for each moved page
      proves: the page shows real data from shared code
      evidence_class: rerunnable script output plus the screenshots
    - action: the owner's own words after opening a phone build
      proves: the app works for its owner; nothing else proves it
      evidence_class: owner verdict
    - action: 'the ten agent questions of the design, each run as Go in a research cell on d@e against staging, output kept in docs/evidence'
      proves: an agent can read events, entities, claims, outlets and edges with vectors in the same call, with ids that join
      evidence_class: deployed proof, rerunnable
    - action: 'a hand read by someone other than the writer of 100 event joins and 100 entity attachments made after the layer lands, against the baselines (events 282 of 300 singletons; entities 29% not entities, 23.9% with a second id)'
      proves: identity judgments are better than a name hash and a mint per question, or they are not
      evidence_class: production sample recorded in docs/evidence
    - action: 'scripts/status-hourly search usage block and --history over a full UTC day, with distinct happenings published that day and stories with a second outlet'
      proves: web searches under 1,000 a day with coverage not lower; a count that falls because research stopped is a failure
      evidence_class: production journal counts
    - action: 'the duplicate-story measure rerun after consolidation lands'
      proves: the share of new stories that duplicate a happening already covered has fallen against the first measurement
      evidence_class: production counts and a hand read
  rollback: each landing reverts as one commit. Derived tables and vectors are dropped and rebuilt. Identity judgments are records, so a merge or a retirement is undone by a superseding record, not by a revert. The phone rolls back by installing the previous build of main.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance, device_install]
value:
  better_means: 'three gaps close: non-voice native lines fall to what voice and the platform force; a happening on the paper has one story, linked to its events, entities and sources in one graph an agent can query; web searches a day fall under 1,000 because the paper can tell what is new.'
  goodharting_would_be: 'Swift moved behind a thin native wrapper, or a page moved with its tests deleted; a typed library nobody''s cell calls; entities and events counted as linked while judgments are still a hash and a mint; consolidation that lowers the story count by dropping coverage, or one story that swallows unrelated happenings and reads as a list; a search count that falls because fewer runs start; a design document counted as the layer.'
homotopy:
  realism_axis: 'from one page slice proved on the simulator, to no page logic in Swift and the owner''s phone; from the library answering on d@e, to the paper''s research desks using it in their runs; from an hour''s search count after a deploy, to a full UTC day.'
boundaries:
  mutation_class: red (the semantic layer touches the tape, export, the registry and the desk protocol; page moves are orange, red where they post server ops)
  authority_sources:
    - CLAUDE.md (binding)
    - docs/choir-doctrine.md, docs/learning-engine.md, docs/computer-ontology.md
    - docs/evidence/semantic-layer-as-built-2026-11-20.md (ground truth for what exists)
    - docs/design/semantic-layer-2026-11-20.md (once written and read by the owner; until then no red station of track O or C starts)
    - docs/evidence/semantic-layer-prior-art-2026-11-20.md (failure modes only; its numbers are not facts)
    - docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md
  owner_rulings:
    - '"on search, we should be reading much more and searching less" (2026-11-21), said on seeing that runs read 9% of the pages their searches return and send 44% of later searches with no page read since the last. A run reads what it holds before it searches again.'
    - 'On what stays native: "should all this remain swift? research exact to see which of these could be exact. anything which can, should be. since that facilitates better cross platform. on-device models are more likely to be swift than the audio player or microphone." and "the cutover is essential" (2026-11-21). The lead''s reading: the voice list is not fixed; the audio player, microphone and audio session move to Exact wherever Exact carries them, after the research says which; fixing the audio bugs and setting up newer models are not this mission.'
    - '"whats going on with the exact cutover? its moving super slowly" (2026-11-21). The cutover moves by whole packages of files built in parallel, each deleting its Swift, and is measured in non-voice Swift lines deleted.'
    - '"raw duplicates may be only 17% but we can probably do more story clustering for where they overlap", and the lever chosen for the rest: fewer searches per run (2026-11-21): a research run sends one search and reads what it returned before another; clustering goes past exact duplicates to stories that overlap. Not chosen: waiting before a first search, researching fewer stories, relaxing the 1,000 target.'
    - 'When it is unclear whether a new article has the same main point as a held story: "Lean to revising" (2026-11-21). The lead''s reading: an unclear case is folded into the existing story; a wrong merge must therefore be visible and undoable, which the design has to provide.'
    - 'On staging the ontology over days: "no, it shouldnt take days or weeks to build. you can do this all overnight" (2026-11-21). The main-point judgment and the read before a search are built now, in parallel, on what exists; the wider query library follows them and does not gate them. Red changes still take their two reviews and their deployed proof.'
    - '"I asked for the whole app to be written in exact from the beginning. And then it got written in Swift. And then I keep asking for the cutover, and then it keeps not happening." (2026-11-20)'
    - '"I don''t need to save stuff from the last couple days of testing. The app is very new. We haven''t launched it yet. I don''t need to save anything." (2026-11-20): a moved page carries nothing over from the Swift build and never reads the old native file.'
    - '"Yes, I can pick my edition topics again. That''s fine." (2026-11-20)'
    - '"Yes, takes absolutely belongs in the product, but it''s not a separate feature that you start by clicking a button... the real interface for takes needs to be initiated from voice. But anyway, we can just leave it as it is... I never asked for muting." (2026-11-20): takes stays as it is; the voice-first design and the removal of muting are not in this mission.'
    - '"The reader app. Reads PDFs and ebooks, but they don''t need to be books." (2026-11-20): the words "books" and "shelf" are not the owner''s; the thing is imported documents.'
    - '"Are you saying we still haven''t done events and entities? Let''s do that. I''ve asked for that like 10 times now." (2026-11-20)'
    - '"we need to really architect the events and entities... That every story has a semantic representation, so that we''re building a whole ontology... re-architect it so that it''s all unified rather than ad hoc. make it so that it''s easy for agents in their REPL to query and use it." (2026-11-20)'
    - '"I don''t want to over-index on what the BBC did or what some single study with a 60% number says. We actually need to design this from the ground up, recognizing that we have more advanced agents than anyone has had in the past, right? We''re using RLM. We have a whole ontology built in." and "we do have the System 1 models as a tool as well." (2026-11-20)'
    - '"I think the stories we may be doing duplicate stories of the same topics. So that''s one way that we can improve the quality of the paper by consolidating overlapping stories, and also that''ll end up reducing searches." (2026-11-20)'
    - '"the deduplication stuff should be rewritten later to use the ontology. Since for stories with significant overlap we should be revising not making new stories. This way we will develop fewer deep stories on key topics rather than many shallow stories that cover parts of it. In this way choir.news will be very unlike a newspaper which has a new article for every beat of a story. Choir should instead build a singular narrative that incorporates all the article as they accumulate over time" (2026-11-20): a development is a revision of the story that holds the topic, not a new story; the design''s default of opening a follow-on story after 24 hours is overruled. C1b is a stopgap to be rewritten on the ontology.'
    - '"Ok let''s update to latest exact too"; "I do want to update Exact. We''re going to be writing the latest versions of Exact for some time. We definitely should be on the latest before we write all this exact code." (2026-11-20): the Exact pin moves from de212e671 to upstream FIRST, and is kept current after; X0 is the first station of track X.'
    - '"a story stops growing when the main point, the main beat changes. for example, an election story, before the election, the story could be about momentum or ads or debate or what someone said. These are all different stories. Then election night, the story is the election results as they''re coming in. Then the next day, the story is who won and who lost. Right? So the point is not to put everything in the same story, but that we can use natural reasoning to cluster things so that We aren''t making new stories every time a newspaper has a new story." (2026-11-20): a story is one main point. New material that serves the same main point revises that story; when the main point changes, a new story begins. The boundary is a judgment made by reasoning (an agent or System 1 reading both), not a time window, a vector threshold or a topic label. This answers Q6.'
    - '"For texture, we should be using exact wysiwyg markdown text editor (research it in exact repo) and their pretext/reflow/textflow text calculation stuff" (2026-11-20). What the source says (docs/reference/exact/markdown-editor-and-textflow.md): Pretext is not part of Exact; textflow and the reflow demo are Exact''s text layout for reading; the Markdown editor is the one we already use, WYSIWYG only on the web, with our patch 0001 hiding syntax on iPhone.'
    - '"if you upload a file that''s actually going to your computer... So uploaded files really shouldn''t be deleted because they''re saved in the cloud." (2026-11-20)'
    - '"The point is that we have two goals right now. To finish the cutover to exact... And to finish the search and ontology work, which involves entities and events. Takes and muting is beside the point." (2026-11-20)'
    - '"we should be able to get our searches underneath 1,000 per day... we only need to search for things which are new information for us" (2026-11-19).'
    - '"be sure to use the lander and builder vms on node a. The key constraint is our local cpu usage. Exact builds should be fast though, so the more we port to exact the faster it gets." (2026-11-20)'
  must_preserve:
    - no secrets, tokens or the owner's email in the repo or in output; .env, .env.bak-bedrock-20261109124037 and stash devin-provider-wip hold secrets and are left alone
    - the owner's iPhone credential is never changed, moved or re-keyed
    - nothing changes on the paper or other users' computers except by deploy; drills run on d@e only; Node B reads are read-only
    - 'no switches; no limits on our own model calls; one Jev call per flush with every question in it; a web search fix is the caller fixed, never a cap'
    - one custodian per data kind; desks talk through precommitment records; research has read-only world-evidence authority; a private computer's data never crosses owners
    - one landing and one deploy at a time; one Mac native build at a time through the lock; builders never run native builds or simulators
    - never pkill; never a bare git stash; never commit in the primary checkout; lanes only; lane/search-ceramic is kept
    - never a review trailer that no review backs
  excluded:
    - the voice-first takes interface and the removal of muting (their own mission, from the owner's description)
    - the Texture root desk, the snapshot streaming rewrite and owner-act enforcement (stopped for the owner in the previous mission; see decisions)
    - TestFlight; Android sign-in and voice in Kotlin; the Mac app
    - any change to search providers, their keys or spending
stations:
  - id: X0
    name: Exact moved to upstream
    scope: 'the pin moves from de212e671 (2026-11-16) to upstream HEAD (fb72ddc3a on 2026-11-20, 760 commits). The lead does it in the build tree, since it needs the native build: read the grammar and package-interface diff against our contracts and mobile/exact-package first; re-check patch 0001 (it applies to upstream by git apply --check; never compiled) and drop it if upstream hides syntax; contract build, test web, the native build, proof-real.sh with screenshots read, a phone install. The Apple build was restructured upstream: risk medium-high, unrun. Builders keep working against the old pin until this lands, then rebase. Rollback: the pin file.'
    class: orange
    review: one seat on our diff
    status: working
    note: 'first in track X by the owner''s word. Slices already built against the old pin (X1 to X5) are proved and landed only after they build on the new pin; their builders finish the commit in hand and then rebase. After it lands the pin is moved again whenever upstream has moved a week, as a routine landing.'
  - id: X1
    name: Edition picks and takes in shared code, no carry-over
    scope: lanes exact-edition-picks and exact-takes as rebuilt; one seat each; real proof with screenshots read; land; phone install
    class: orange
    review: one seat each
    status: working
  - id: X2
    name: The About page's Build row
    scope: lane exact-about-build-row 9c8555f7e; proof and review in flight; land
    class: orange
    review: one seat
    status: working
  - id: X3
    name: The reader
    scope: 'lane exact-reader, in slices: the read source and text blocks; citations and versions; related; tables and embeds. Each slice deletes its Swift and lowers the ceiling.'
    class: orange
    review: one seat a slice
    depends_on: [X0]
    status: working
  - id: X4
    name: The bar and the run sheet page models
    scope: 'lane exact-bar-runsheet, in slices; the player and audio session stay native as a state snapshot and commands; the slice that posts run sheet ops to the server is red.'
    class: orange; red for posted ops
    review: one seat; two for posted ops
    depends_on: [X0]
    status: working
  - id: X5
    name: Mail and files leftovers and the open-pages set
    scope: lane exact-mail-files-rest; dead Swift deleted first; the live mail store's server writes are red
    class: orange; red for the mail store
    review: one seat; two for the mail store
    depends_on: [X0]
    status: working
  - id: X6
    name: Imported documents (the code's "shelf"), onboarding and settings models
    scope: 'The owner, 2026-11-20: "if you upload a file that''s actually going to your computer, your VM on Choir.news. So uploaded files really shouldn''t be deleted because they''re saved in the cloud." That is true of the Files page (it uploads to the computer) and NOT of the reader''s imports today: an imported PDF or EPUB is kept only on the phone (Application Support/Autoradio/books, AutoradioModule+Books.swift:23). So this station makes an imported document a file on the owner''s computer, with the phone holding a cache; list, search, reading position and read-aloud hand-off in shared code, named for what they are (not "books" or "shelf"); PDF text extraction may need a native op, with its reason. Onboarding and settings page models. Start when two of X3 to X5 have landed a slice.'
    class: orange
    review: one seat a slice
    depends_on: [X1]
    status: pending
  - id: X7
    name: The Texture editor on what Exact's editor already holds
    scope: 'docs/reference/exact/markdown-editor-and-textflow.md: Exact''s editor holds the draft, the selection and undo. So the field is the draft, and shared code keeps one pair (the field''s text and the revision it descends from); a merge writes into the field''s value and Exact applies it as a minimal edit with the caret kept. Tests for each step before code. Patch 0001 (no syntax visible on iPhone) proved once on the simulator. Known costs: a remote merge clears undo; on Android stock Exact shows Markdown source (owner decision Q5). Textflow and the reflow measure are reader work: use them for the reader''s layout as a slice of X3.'
    class: red
    review: two seats
    depends_on: [X0]
    status: pending
  - id: X8
    name: What is left native, counted and justified
    scope: 'when X1 to X7 have landed: read every remaining non-voice native file; each is deleted, moved, or kept with its reason beside the call; the ceiling is set to what remains and the problem record of 2026-11-18 is closed or restated.'
    class: yellow
    depends_on: [X3, X4, X5, X6, X7]
    status: pending
  - id: O0
    name: The unified design, read by the owner
    scope: 'docs/design/semantic-layer-2026-11-20.md returns; the lead checks it against the as-built survey and the owner''s brief, has it challenged by a panel (skills/agentic-consensus) on identity grain and custody, and brings the owner its opening page and decisions. It replaces this file''s O and C stations with its own landing sequence in the same change. It rules on lane/story-events-e5 and the stopped E3.'
    class: green
    review: a panel on the design; no code review
    status: working
    result: 'the design is written (4a1aaeb4c): judgments as tape records with the graph as their projection; happenings, entities (outlets among them), stories and claims in the Autopaper object graph; choir.World with Get, Find, Neighbors, Walk, Ask, Judge; landings L1 (C1b), L2 (the flush writes recorded judgments, 50 vector neighbours, clustering within a flush), L3 (happenings into the graph; the story-events-e5 lane changed, not landed as is: its index is capped at 10,000 events). It rates under 1,000 searches a day at p 0.10 and 2,400 at p 0.50. Not yet read by the owner; not yet challenged by a panel. To amend for the owner''s rulings: a story is one main point; material on the same main point revises it and a change of main point starts a new story, judged by reasoning.'
  - id: O1
    name: One identity space
    scope: 'as the design sequences it: events, their members and the story-to-event and capture-to-event edges in the Autopaper object graph beside stories, captures and entities; one id form per thing with the others resolvable; outlets as objects; the custody tension over management and events resolved in doctrine''s terms; the og_edges id columns declared COLLATE "C". No rewrite of large tables in one step.'
    class: red
    review: two seats a landing
    depends_on: [O0]
    status: pending
  - id: O2
    name: Identity as recorded judgments
    scope: 'event joins and entity identity decided by vectors for candidates, System 1 typed questions with the distribution recorded, and an RLM agent for the unclear cases; merge, split and retire as superseding records, undoable; the flush stays one Jev call. The research.related_story switch is removed. Edges written and never read are deleted or given a reader.'
    class: red
    review: two seats a landing
    depends_on: [O1]
    status: pending
  - id: O3
    name: The typed library in the cell
    scope: 'the Go surface a research or management cell imports: read an event, an entity, a claim, an outlet, an edge; vector and graph in one call; typed results with ids and provenance; a read''s bounds in its type; custody and disclosure labels enforced by the surface. Proved by the design''s ten questions run on d@e. The first read-only verbs may land before O2 over what exists today.'
    class: red (desk protocol)
    review: two seats
    depends_on: [O0]
    status: pending
  - id: O4
    name: Quality read by someone other than the writer
    scope: 'a standing hand-read sample of joins and attachments scored against precommitted claims (the learning engine''s forecast and score), with a status script line; the baselines are in the as-built survey.'
    class: yellow
    depends_on: [O2]
    status: pending
  - id: C1
    name: How much the paper duplicates itself
    scope: 'docs/evidence/duplicate-stories-on-the-paper-2026-11-20.md: the share of a day''s new stories that report a happening an earlier story already covers, with examples and what each duplicate cost in searches and model calls. Sets C2''s threshold and its expected effect.'
    class: yellow (read-only measurement)
    status: proved
    result: 'docs/evidence/duplicate-stories-on-the-paper-2026-11-20.md: of 982 stories opened in 24 hours, 17.4% duplicate a story of the previous 72 hours and 20.1% are second-or-later on a covered happening; about one published story in five is a repeat; duplicates and follow-ups took 17.3% of web searches, so consolidation alone leaves about 2,800 to 3,200 a day. The management task that decides to open a story is shown only the newest 50 stories (a median 27 minutes), not the 8 hours its prompt promises; the earlier story was on that page for 6 of 171 duplicates.'
  - id: C1b
    name: The open decision sees what the paper holds (a stopgap)
    scope: 'to be rewritten on the ontology (owner, 2026-11-20). Before the ontology lands: management''s story page is the window its prompt promises (headlines only, inside the token bound), and the top held candidates of the last 72 hours are offered to the judge, so a held story is fed and not opened again. Lane dup-open-page.'
    class: orange
    review: one seat
    depends_on: [C1]
    status: working
  - id: C2
    name: One narrative per topic
    scope: 'built on the ontology, replacing C1b. A capture whose happening overlaps significantly with what a story already tells (the same happening, a development of it, or a part of the same situation) becomes material for a revision of that story, with what is new since its last version, and no new story opens; the story deepens over time into a single narrative with its history. The unit is the main point (owner, 2026-11-20): material that serves a story''s main point revises it, and a change of main point starts a new story (before an election: momentum, ads, a debate are different stories; election night is the results coming in; the next day is who won). The judgment is made by reasoning over the story and the new material, recorded with its reason, and scored by a hand read; a research run begins from the event''s state and searches the web only for what the corpus does not hold; duplicates already published are linked, not deleted. Search station L2 (a side measurement) was stopped after three send-backs (docs/problems/search-sufficiency-measurement-three-reviews-no-landing-2026-11-20.md); the read-before-send it was preparing, with its record, is built here as part of the one verb (L3).'
    class: red (run acceptance, desk protocol)
    review: two seats
    depends_on: [O1, O3, C1]
    status: pending
  - id: C3
    name: The day's number
    scope: a full UTC day under 1,000 web searches with coverage not lower, read from the search usage history
    class: yellow
    depends_on: [C2]
    status: pending
  - id: P1
    name: Stories that cannot publish
    scope: 'the label carry that keeps twelve stories unpublished since 2026-11-08 (fix at the carry, not the gate); the 502 on loading the private revision (time the read first; then the read stops scanning the whole scope and a timed-out publish is retried; orange); the reply wake (lane reply-wake-fix). Each its own landing.'
    class: red (labels, run acceptance)
    review: two seats
    status: working
  - id: R1
    name: Operations
    scope: 'the hourly tick, now also reporting the native ceiling; the Mac build queue at ~/.local/state/choir-buildq; settle open claims at their times (C165, C178, C183); an owner letter with the reporter skill at each real change in the story, naming every expedite; lanes removed when they land or are dropped.'
    class: yellow
    status: working
decisions_for_the_owner:
  - id: Q1
    question: Which part of the semantic layer's design do you want changed before red work starts on it?
    why: every later station builds on its kinds, identity grain and custody
    options: [accept the opening page as written, change named parts, stop and redesign]
    default: no red station of track O or C starts until the owner has answered; read-only verbs over what exists today (part of O3) and all of track X continue
  - id: Q2
    question: Should the paper's snapshot budget be raised from 90 to 150 minutes while the streaming rewrite waits?
    why: a capture took 73 of 90 minutes today and the store grows daily; past the budget no restore point is made
    options: [raise to 150 minutes, leave it]
    default: leave it; the lead watches each capture's time at the hourly tick and raises the question again at 85 minutes
  - id: Q3
    question: Do the root desk, owner-act enforcement and the snapshot rewrite stay parked?
    why: each was stopped after repeated send-backs and needs substrate work
    options: [parked until this mission ends, bring one back]
    default: parked
  - id: Q4
    question: Is the About page's Build row showing the commit and time what you meant by "the actual build"?
    why: the lead read it that way and built it
    options: [yes, an incrementing build number instead]
    default: the commit and time
  - id: Q5
    question: On Android, is Markdown source with a preview acceptable for the Texture editor until Exact offers more?
    why: stock Exact's editor hides syntax only on the web, dims it on iPhone and Mac (our patch hides it on iPhone), and is plain source on Android
    options: [accept on Android for now, build a syntax-hiding editor for Android]
    default: accept for now; Android is outside this mission
now:
  status: working
  as_of: 2026-11-21T03:00Z
  slice: 'X0 landed (ddadb1e6f: Exact at upstream f53347503; native build and proof-real passed first try), phone install running; X1 takes (5118bbe9e, review accepted), edition picks, X3 reader (slice 3 unreviewed), X4 bar (slice 2 and fixes unreviewed) rebasing onto the new pin; X5 two dead-sync deletions queued for proof then landing; C1b landed (d8367e2c8, red, repaired by 2b862a00e then e33533850) and deployed, the paper booted on it 02:46Z; L2 stopped after three send-backs; proof hardening landed (cd818de2a)'
  source_ref: origin/main ddadb1e6f
  deploy_identity: staging e33533850; the paper on e33533850 since 02:46Z; the owners iPhone deb9c9ef0 until the pin build installs
  candidate: none frozen
  conjecture:
    claim: 'most of the paper''s web searches are first runs of stories about happenings it already covers or already holds captures for; when a joining capture becomes a new version of the existing story and a run starts from the event''s state, first runs a day fall by at least 40% and web searches fall under 1,500 a day within two full UTC days of C2''s deploy, with distinct happenings published a day not lower than the week before'
    test: scripts/status-hourly --history for searches; the paper's run ledger for first runs; the duplicate measure for happenings
    edge: missing_oracle
    edge_note: nothing today says which stories report the same happening; C1's hand read is the first oracle, and the claim can survive falsely if "distinct happenings" is counted by the same judgment that merged them
    observer_upgrade: a hand-read sample of merged and unmerged pairs by a reader that did not make the judgment (O4)
    scope_if_supported: consolidation is the main lever on search volume; continue to C3
    ledger_id: C183
    status: untested
  believed_state: the app is two thirds Swift by the gate's measure and five moves are running under the no-carry-over rule; the semantic layer is built in pieces that do not join and its unified design is being written; the paper sends about 3,260 web searches a day
  main_uncertainty: whether the design's identity judgments are cheap and right enough at 5,000 captures a day inside one Jev call a flush; how much of the search volume duplicates really explain (C1 will say); whether the reader and run sheet move in days or weeks
  next_observation: scripts/status-story-choice on the paper from 02:46Z (window=8h on every line, opens with verdicts, feed share against 51%); the duplicate rate a day later; each page lane's proof on the new pin
  blocker: none
  next_action: 'read the paper''s first story-choice lines and record them; land X5''s two deletions after their proof; proof and land takes, then one review each for edition picks, reader slice 3 and bar slice 2; amend the semantic-layer design for the owner''s main-point ruling with real pairs and put it to a panel (Q1 before any red O or C work); P1: the label carry (red) and the publish 502 (orange); a landing check for tests tied to non-Go files by path belongs in scripts/land'
operating_notes:
  - 'Roles: the lead orchestrates; Sonnet subagents build in lanes (cd /Users/wiz/Dev/go-choir && git fetch && lane new <name> --base origin/main); Opus only for open design. Review seats are in CLAUDE.md: seat 1 Gemini 4 Hydrogen through the official agy CLI (a Sonnet subagent on a rate limit), seat 2 DeepSeek through the agentic-consensus runner on a frozen copy; orange one seat, red two. After two send-backs of one mechanism, change the mechanism, not the patch.'
  - 'Page moves: rewrite from what the page must do, then delete the Swift; nothing carried over; account ownership (accountGeneration, writeCurrent, the wipe list) from the first commit; tests red before code; slices that each lower the ceiling. The lead runs the one native build and the simulator proof through the queue and reads the screenshots.'
  - 'Go compiles and tests on the Node A build machines (ssh choir-runner; each builder its own directory), never the Mac; gate scripts on the Mac need GOFLAGS=-mod=mod. Before a landing run the tree-scanning checks too (the objectgraph registry test, the Nix vendor hash): docs/problems/train-landed-red-objectgraph-registry-and-vendor-hash-2026-11-20.md.'
  - 'Production differs from tests in collation (en_US.UTF-8): no text range predicate on ids. A builder''s report is a claim: see the suite''s ok line and the screenshots yourself, and enter the calibration claim first (next free id C186).'
  - 'Before any landing or review of a rebased lane: `git diff origin/main...HEAD --stat` (three dots) must list only the lane''s own files, and `git diff origin/main HEAD -- CLAUDE.md docs/definitions docs/calibration` must be empty apart from the lane''s own edits. A builder squashed with `git reset --soft origin/main` after a fetch and its commit reverted a day of main (caught by review, 2026-11-20). Builders squash by rebase or cherry-pick onto a fresh lane, never by soft reset.'
  - 'Before a landing that touches a non-Go file (a prompt overlay, YAML, JSON, a contract): find every test that depends on it by any route (grep the basename over test files; for a prompt, the size and byte-stability tests in internal/agentcore and the role tests in internal/yaegikernel) and run those packages. d8367e2c8 and its first repair each landed red this way (2026-11-21).'
  - 'Waits use a file''s content or age, never pgrep -f (it matches its own command line). State times only after date -u.'
receipts:
  - 'previous goal: docs/definitions/choir-exact-search-entities-2026-11-20.md (superseded by this file; its landed_this_thread list is the record of what landed on 2026-11-20)'
  - 'evidence of 2026-11-20: semantic-layer-as-built, semantic-layer-prior-art, research-runs-with-nothing-new, event-join-rate-hand-read, who-sends-the-web-searches (docs/evidence)'
  - 'letters: docs/reports/overnight-exact-search-entities-2026-11-20.md'
---

# The Exact cutover and the ontology: the mission of 2026-11-20

Two goals, in the owner's words: finish the cutover to Exact, and finish the
search and ontology work, which is events and entities with a surface agents
can query from their cells.

They are one mission because they share one failure. Each was asked for many
times and each was built in pieces that never added up: an app written in
Swift under Exact pages, and events, entities and vectors that do not join.
The previous goal treated both as lists of stations and landed many of them
without closing either gap.

What is different here.

The cutover runs wide, not deep. A moved page carries nothing over from the
Swift build, by the owner's ruling, so the carry-over that sent every move
back to review is gone. Pages are rewritten from what they must do, in
slices, by several builders at once. The count of non-voice native lines is
reported every hour.

The ontology is designed once, from the ground up, before more is built on
it. The system already has a registry of kinds with custodians, a tape,
vectors, System 1 and agents that write Go in a cell. The design decides
what a story's semantic representation is and makes every thing an object in
one identity space. Identity is a recorded judgment that can be undone, not
a hash. The owner reads the design's opening page before red work starts.

Search follows from the ontology. The paper spends its searches on first
runs of stories. If a happening has one story that grows, and a run starts
from what the paper already holds about that happening, the searches that
remain are for what is new. That is the conjecture, entered as C183.
