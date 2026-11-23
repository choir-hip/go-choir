---
definition_version: 7
readiness: executable
authority: 'the owner, 2026-11-22 about 04:50Z, on draft 2 of docs/design/first-design-2026-11-22.md read back from the top: "great, this is the plan for the next throughline". Earlier the same day: "we are approaching a fundamental redesign. not a refactor but a redesign... in some ways, this is the first design"; "we need to make this efficient and fast".'
review:
  reviewer: none (the owner's statement is the authority for the plan; each station names its own review tier, and the design gets a panel before any red station starts)
  frozen_ref: none
  verdict: none
start:
  captured_at: 2026-11-22T05:00Z
  source:
    canonical_ref: origin/main 080d40510 (docs); the last code commit is 92f068980
    deploy_identity: 'staging and the paper run 92f068980 (the main-point judgment, deployed 2026-11-21T23:58Z, and read-first, live from about 2026-11-22T01:30Z). The health route does not print the commit; identity is read from the CI run''s deploy job and the paper''s console lines.'
  what_is_true_now:
    - 'Three mechanisms ask which thing a capture belongs to: the intake label (intake_labels.go), the event layer''s call a minute later (event_layer.go), and a management session for up to 12 captures (management_tasks.go). The third ignores the second. Event memberships are read only by the owner''s event view and research''s corpus search.'
    - 'About 1,000 model calls an hour for about 100 captures judged on the paper (gateway totals, 2026-11-22 02:09Z to 03:09Z). The split by desk is not logged.'
    - 'Subjects do not exist. What exists: 6,690 story entity rows with mentions_entity edges and a learned alias lexicon, of which a hand read found 29% are not entities and 24% duplicate another (docs/evidence/entity-layer-baseline-hand-read-2026-11-20.md); captures carry none.'
    - 'Predictions are stored (CastHypothesis, CalibrationClaim) and scored for some desks; nothing matches an arriving object to an open prediction, nothing settles a prediction when its time passes, and Texture is never shown its score.'
    - 'Sub-calls exist for research only and no run on the paper used one in the 2.5 hours read; the research prompt does not mention them. A child has no tape of its own.'
    - 'The old root desk sits on main behind texture.root_desk (off by default), with an hourly cap, after six reviews without a landing (docs/problems/texture-root-desk-six-reviews-no-landing-2026-11-20.md).'
    - 'A personal edition does not exist as a paper; Autoradio is two server routes and the phone sends the public front page as candidates.'
    - 'After the two search landings the paper sent 47 web searches in the first 2.1 hours of 2026-11-22 (a rate of about 544 a day against 1,865 the day before), from fewer research runs; searches per run did not fall (4.33 against 3.9).'
  branches_in_flight:
    - 'lane/graph-first (a4ac4995f at capture): the research digest from the object graph, the graph-first prompt, the refusal gate removed. Both seats sent the first candidate back: research cannot read a Texture document, so the story section was empty in production. Being reworked so the custodian sends the text in its Cast. Belongs to the previous mission and is this mission''s first use of the retrieval shape.'
    - 'lane/exact-train1 and lane/account-partition-s1: the phone cutover and the account boundary; the previous mission''s, untouched by this one.'
finish:
  deliver: 'The paper runs on the first design: a capture is attached to subjects with no agent session; stories and the root predict, are woken by what answers or surprises them, and are scored; a person''s computer has a root whose Texture is their edition; Autoradio is a desk that runs cells and composes from the graph.'
  artifact: 'on main and staging: the subject and mention tables with their query verbs; the hot path replacing the intake label''s same-event question, the event layer and per-capture management tasks; predictions that name a subject with matching, settlement and the score shown; stories woken by mentions; the root with stories as sub-calls; the personal root; the Autoradio cell desk; and the deletions listed in the design''s section 9.'
  acceptance:
    - action: a status script reading the gateway journal by desk on the paper, over a whole day
      proves: model calls per capture, per woken story and per published story, by desk; the hot path adds no agent session per capture
      evidence_class: deployed count
    - action: a hand read of 100 capture-to-subject attachments and 50 story pairs the rarity-weighted overlap calls alike, by a reader that did not make the judgment
      proves: attachments are right and likeness finds real duplicates, at bars set in the station before the read
      evidence_class: hand-read sample
    - action: the surprise rate and the prediction scorecard printed by a status script from the paper's records over a week
      proves: predictions are matched, settled on arrival and on expiry, and scored; the share of arriving claims no prediction expected is measured
      evidence_class: deployed count
    - action: the owner reads their own edition and listens to a programme composed from the graph
      proves: the readers work for a person
      evidence_class: owner verdict
  rollback: each station lands as commits that revert; subject and mention tables are derived and are dropped and rebuilt; a retired mechanism is deleted only after its replacement has a deployed proof, in its own commit.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
value:
  better_means: 'three gaps close: the cost of one more capture falls to an embedding and a share of one System 1 call; what the paper holds is found and used before the web (stories cite stories); the paper states what it expects and is measured on what surprises it.'
  goodharting_would_be: 'a subject table nobody traverses; a surprise rate lowered by predicting vaguely or by subscribing to everything; calls per capture lowered by moving the same sessions to another desk''s name; an edition that is the front page with a name on it.'
homotopy:
  realism_axis: 'from stories only (subjects from the rows that exist, predictions that name a story''s subject) to captures, then the root, then a person''s computer; the same tables, verbs and records at each step.'
boundaries:
  mutation_class: red (the tape, the desk protocol, Texture wakes, run acceptance and the registry of kinds are touched); stations 0 and 1 are yellow and orange
  authority_sources:
    - 'the owner''s words of 2026-11-22, quoted in docs/design/first-design-path-2026-11-22.md and in the owner_rulings of docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md'
    - 'CLAUDE.md (the operating contract) and docs/choir-doctrine.md'
  owner_rulings:
    - '"one bucket from which any symbol can be associated with a story... no need to model events and entities as separate things. the real question is about usability, query patterns, efficient traversal" (2026-11-22).'
    - '"we shouldnt need rules. we should be able with prompts to encourage models to search the object graph first, and then the web" (2026-11-22).'
    - '"adding the sources comes later. for now we need to redesign"; "yes, we should codesign the capture path with the sources. that makes sense and gives us a scaling law for our resource consumption and ontology growth" (2026-11-22, evening). Reading: the capture path and the source set are one design. Its output includes a scaling law: for a source of a given kind, language and rate, what it costs per day in disk, model calls and new subjects, measured on the offline slice before it is built, so the number of sources is chosen from the law and not found by running out of disk. Sources of new kinds (a model hub''s listing API, podcasts with transcripts, non-English feeds, forums) are test cases for the design, not additions to the current path. No source is added to the paper before the redesign except by the owner''s own act.'
    - '"we need to redesign the paper at both micro and macro levels, from data architecture to prompt engineering"; "the self-dev part is reminding us not to overfit for text research and writing... autoradio will be a conduit for you to do general computer use. eg you get an important email. autoradio interrupts to read that email aloud, and you respond to it in your voice... or instead of email, its a question from an agent, and the answer influences their next moves which are executed with code" (2026-11-23). Reading: the design builds one general shape (something arrives; what is it about; does it belong to something under way; does it matter now; act; learn) with the paper as its first user, so mail and agent questions use it and do not imitate a news pipeline. "Matters now" must serve interruption as well as story choice.'
    - '"the paper should become, at scale, more economical than exposing raw web search for any agent looking for current events. indeed, ive already been asked to sell it as a data product" (2026-11-23). Reading: this is the design''s measure. For a current-events question, the cost and quality of answering from the paper against answering by raw web search. Our own research desk is the first customer and today it fails the test: 65% of a week''s captures were pages research fetched from the web itself, and giving it the graph first did not cut its web searches (C191 false). Selling it is the owner''s act.'
    - '"the point isnt to cut search to zero, but to cut it down 50, 75, 80, 90 %. and that doesnt even need to cut search volume... it could mean keeping it flat or scaling it while we scale corpus usage even more. weve already cut searches quite a bit, now we need to scale corpus utility" (2026-11-23). Reading: the measure is a share, not a count: of what a run relies on, how much came from the paper and how much from the web, with rungs at 50, 75, 80 and 90%. Web search volume may stay flat or grow. No further work aims at lowering searches per run. Baseline, one window on 2026-11-22: of the pages research read, 27% were held before the run and 35% were new from the web; 238 corpus searches against 188 web searches.'
    - '"we can do learning over retrieval. predictions then retrieval then scoring predictions. and next prediction gets an accumulated sentiment of the past predictions scoring" (2026-11-23). Reading: the learning engine''s four strokes applied to each retrieval: the run forecasts what the paper holds before it asks, the retrieval answers, the forecast is scored by an outcome the run cannot write, and the next forecast is shown the accumulated score. The design panel works out what is forecast, what scores it and how the score is accumulated and shown.'
    - '"the goal of any run shouldnt just be to write the best story, but to be building on and using our corpus, and then the story adds to our corpus" (2026-11-22).'
    - '"we can run captures through vector search, then for each match we can do graph traversal. this is then sampled and provided with metadata and full data accessible in repl" (2026-11-22).'
    - '"stories, textures, should become much more recursive, more frequently citing and transcluding other stories... textures reference textures is the essence of it" (2026-11-22).'
    - 'The root Texture: "giving a global perspective on the paper... each texture agent would be a sub-rlm of the root texture. enabling messaging... it would do much of what management is doing now. and for individual users, it would be similarly tracking a computer-owner model"; "root is one long session. but its repl makes it easily resettable... it writes its own texture, which is a text vector, a list of revisions" (2026-11-22).'
    - '"concurrency is to be achieved by desk subcalls" (2026-11-22).'
    - 'Desks: "yes, the middle form" (one interface a desk, a story addressable inside it, the root observing); the root "should get events. debounced. it can subscribe to events because it makes predictions about events and entities and those predictions register subscriptions"; "stories predict next events and get woken with them. surprises are key" (2026-11-22).'
    - 'Subjects for a capture: "maybe we can predict subjects from keyword extraction and/or vector embedding? and then any prediction can be scored with system1, which can also select subjects from a list, and also suggest more/other/idk which can escalate to llm desk" (2026-11-22).'
    - 'Autoradio''s desk "needs to become a go rlm anyway" (2026-11-22).'
    - 'Earlier, in force: "a story stops growing when the main point, the main beat changes"; "revising not making new stories"; no two stories with the same subjects and claims; a subject''s importance is the number of stories about it.'
    - 'Q5, model spend for the offline slice: "yes, i definitely want to spend to make it work... obv" (2026-11-22). Read as: rungs 1 and 2 may spend; no amount was named, so each rung states its estimate before it runs and its measured cost after.'
    - '"unpublished drafts in research are fine" (2026-11-22). Read as: a research run may be given the text of stories not yet published, and its web queries may draw on it.'
    - '"we should design for user computers being always accessible. todays storage issues wont remain" (2026-11-22). Read as: the app gets no separate mode for an unreachable computer; the host''s storage is what is fixed.'
    - 'On Node B''s disk (2026-11-22): "we will offload the tape soon. for now, lets do the other efficiencies"; the only computers to keep are the paper, the owner''s own, and the two test accounts; "we should delete the extra computers. we can make more later"; "Retire it and remove old paper snapshots".'
    - 'On the phone''s sign-in key (2026-11-22), after the lead found Exact keeps secrets in the Keychain itself: "yes, i want to move everything to exact that can be. signing in again is fine." Read as: the key moves to Exact''s secret store; the old native item is deleted, not migrated; this lifts the earlier rule that the owner''s iPhone credential is never moved, for this change only.'
  must_preserve:
    - 'the paper publishes throughout; a mechanism is retired only after its replacement is proved on staging'
    - 'no switches; no limits on our own model calls; one System 1 call a flush with every question in it'
    - 'one custodian a data kind; desks talk through precommitment records; research has read-only world-evidence authority; a private computer''s data never crosses owners'
    - 'a crash restart never resumes work; the root''s memory is its Texture, not its session'
    - 'no secrets, tokens or the owner''s email in the repo or in output; drills on d@e only; Node B reads are read-only'
  excluded:
    - 'the phone cutover to Exact and the account boundary (the previous mission)'
    - 'any change to search providers, their keys or spending'
    - 'Android'
stations:
  - id: F0
    name: The desk on every model call
    scope: 'the gateway''s model-call line carries the desk and run; a status script prints calls per capture judged, per story woken and per story published, by desk, for an hour and a day'
    class: orange
    review: one seat
    status: pending
  - id: F1
    name: Subjects and mentions for stories, and the query patterns tested
    scope: 'read-only first: from the story entity rows and alias lexicon that exist, compute hub sizes and the rarity-weighted overlap of recent story pairs, and hand-read 50 pairs it calls alike. Then the tables (derived, in corpusd''s database), the alias map with a merge as a row, and the five verbs. The design names the custodian before any write lands.'
    class: orange for the read; red for the tables and verbs
    review: one seat; two for the tables and verbs
    depends_on: []
    status: pending
  - id: F2
    name: The hot path for captures
    scope: 'candidates by rule (labels and aliases, names and rare phrases, neighbours'' subjects); one System 1 call a poll selecting per capture, with other; the escalation desk, many captures a session, and a random share of the unescalated checked; mentions written. Then the intake label''s same-event question, the event layer and its store are retired, each after a deployed proof.'
    class: red
    review: two seats
    depends_on: [F1]
    status: pending
  - id: F3
    name: Predictions that name a subject
    scope: 'a prediction names a subject and what would answer it; matching at mention write; settlement on arrival (System 1 judges) and on expiry (false); the score shown to the predictor; the surprise flag and rate. Stories only.'
    class: red
    review: two seats
    depends_on: [F1]
    status: pending
  - id: F4
    name: Stories woken by mentions
    scope: 'a story''s Texture is woken, debounced, by answers and surprises on its subjects, opens with the retrieval shape, cites and transcludes stories; per-capture management tasks and their dispatcher are removed after the deployed proof'
    class: red
    review: two seats
    depends_on: [F2, F3]
    status: pending
  - id: F5
    name: The root
    scope: 'one long resettable session whose memory is its Texture; woken by its predictions and by surprises; stories as its sub-calls with their own tape; the old root desk, its setting and cap, and the research child spacing are deleted'
    class: red
    review: two seats
    depends_on: [F4]
    status: pending
  - id: F6
    name: The readers
    scope: 'a person''s root and its Texture as their edition, reading the public graph with nothing flowing back; Autoradio as a desk that runs cells and composes by traversal'
    class: red
    review: two seats
    depends_on: [F5]
    status: pending
decisions_for_the_owner:
  - id: Q1
    question: Is the paper's root Texture the front page, and a person's root Texture their edition?
    why: it decides whether the edition is a thing to build or what a root writes
    options: [yes for both, the root's Texture is internal and the front page stays a read of stories, decide after F5]
    default: decide after F5; nothing before it depends on the answer
  - id: Q2
    question: Which desk is custodian of subjects and mentions?
    why: one desk writes a kind and the others ask; the hot path, a story and the root all want to change a subject
    options: [a graph desk that corpusd's host code acts for, the Texture desk through the root, the computer as today for events]
    default: host code acting as a named graph custodian writes them, and a story or the root asks by a record; no red write lands until the design panel has read this
  - id: Q3
    question: Which model runs the escalation desk that names new subjects?
    why: it sees only the hard cases, so quality matters more than speed
    options: [the paper's default model, a stronger model for this desk only]
    default: the paper's default model, with the escalated share and a hand read of its answers reported before changing it
  - id: Q4
    question: May anything go from a person's computer to the public graph without an explicit act of publishing?
    why: a personal root's queries and its model of the owner are private
    options: [nothing, counts only]
    default: nothing; the personal root reads a copy or a read it cannot be identified by, settled in F6's design
  - id: Q5
    question: May the offline slice spend on model calls, and up to how much?
    why: 'rungs 1 and 2 replay captures through System 1 and a language-model step; the plan (docs/design/first-design-offline-slice-plan-2026-11-22.md) estimates under 2 dollars of System 1 and under 6 of escalation for a full week, through the d@e identity; no spending cap for System 1 exists in code and its credit balance is not known to the lead'
    options: ['yes: 5 dollars for System 1 and 10 for the language-model step', 'yes with other ceilings', 'no: rung 0 only']
    default: 'no spend. Rung 0 (rules only, no model, a hand read for kill line (c)) runs now; rungs 1 and 2 wait for the owner''s word'
    ruling: 'granted 2026-11-22, see owner_rulings'
now:
  status: working
  as_of: 2026-11-22T05:00Z
  slice: 'F0 and the read-only half of F1, in parallel; the design panel on draft 2'
  source_ref: origin/main 080d40510
  deploy_identity: staging and the paper on 92f068980
  candidate: none frozen
  conjecture:
    claim: 'the subjects the paper already names for its stories are enough to find its duplicate stories: of the 50 recent story pairs with the highest rarity-weighted shared-subject score, at least 30 are judged the same story or one a part of the other by a reader that did not compute the score'
    test: the F1 read on the paper's data and a hand read of the 50 pairs
    edge: missing_oracle
    edge_note: 'the entity rows are 29% not entities and 24% duplicated, so overlap may be carried by junk subjects; a high score could also only restate vector nearness'
    observer_upgrade: the same 50 pairs scored by vector distance alone, so the read shows what subjects add
    scope_if_supported: build the tables and verbs on this likeness measure (F1's second half)
    ledger_id: C190
    status: supported
  believed_state: 'the design is agreed in outline and nothing of it is built; the paper runs the main-point judgment and read-first; graph-first is in rework'
  main_uncertainty: 'whether cheap candidate rules propose the right subjects often enough that the escalation desk stays small'
  next_observation: 'F0''s first by-desk count; the same 50 pairs against vector nearness; a score that ranks (it saturates at 1.0 for 35,177 pairs). F1''s read (docs/evidence/subjects-find-duplicate-stories-read-2026-11-22.md): 32 of 50 same or part, 18 same actors and a different happening, none unrelated; one thing sits under several rows and 28% of stories carry no subject.'
  blocker: none
  next_action: 'land F0; the next no-spend rung: group the week''s unmatched arrivals by vector and count groups by distinct origin at thresholds 2, 3 and 5, with a hand read of 50 groups. Rungs 0 and 0b (docs/evidence/offline-slice-rung-0-rules-find-names-not-happenings-2026-11-22.md): rules find actors, not happenings; the ten nearest earlier stories by vector hold the story the paper used for 72.6% of 2,282 arrivals (control 19.5%); 46 of 75 news arrivals report something no story covers, so an unmatched capture must not escalate by itself.'
operating_notes:
  - 'Roles and review seats are in CLAUDE.md; builders work in lanes (lane new <name> --base origin/main) and compile Go on the build VM.'
  - 'A test of anything a desk reads runs with the reader gate in enforce, as production ships (the graph-first digest passed its tests in observe and was empty in production).'
  - 'No test touches the Mac''s real Keychain.'
  - 'A retired mechanism is deleted in its own commit after the replacement''s deployed proof; before a deletion, grep docs, specs, skills and code comments for citers (standing question 3).'
receipts:
  - 'the design: docs/design/first-design-2026-11-22.md; the path to it: docs/design/first-design-path-2026-11-22.md'
  - 'the previous goal, still live for the phone cutover and the search landings: docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md'
---

# The first design: the mission of 2026-11-22

The owner called the day's design "the plan for the next throughline". The
design is [docs/design/first-design-2026-11-22.md](../design/first-design-2026-11-22.md);
this file is its mission: the start, the finish, the stations and what the
owner still has to decide. The previous mission continues beside it for the
phone and for the search changes already in flight.
