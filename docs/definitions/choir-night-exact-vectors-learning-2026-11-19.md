---
definition_version: 7
readiness: drafted
review:
  reviewer: DeepSeek v4.1 flash (seat 2), rounds 1 to 3
  frozen_ref: e3944da74
  verdict: send_back
  evidence_ref: 'round 1, 2026-11-19T05:44Z: five blocking findings (the design not amended; fusion by reciprocal rank against the design''s measurement; a finish that needed the owner''s phone; a deadline the capture pass cannot meet; conjecture N1 and ledger C144 not the same claim). All answered in the revision that carries this line; re-review pending. The owner''s ruling R1 authorises the night; red stations V1, V2, V6, V7b, L2 and L3 each take their own two seats before landing. Round 2 (2026-11-19T06:10Z) sent it back again: the design still carried the old worker and file wording, the finish promised E1 running and a cause for every failure, and the cutover file''s sign-in defaults contradicted this file; answered in the revision that carries this line. Round 3 (2026-11-19T06:20Z, at e3944da74) sent it back a third time on one blocking finding: the acceptance named the demo smoke script as the signed-in staging tour, which it is not. That and its five smaller findings are answered in the revision that carries this line. Three rounds without acceptance is the contract''s signal to stop patching: there is no fourth round. The file runs on the owner''s own word of 2026-11-19T05:38Z, not on a review verdict; readiness stays drafted, and each red station takes its own review before landing.'
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
  deliver: 'By the owner''s check-in (2026-11-19T17:30Z; the owner, 05:38Z: "ill probably check in on this in 12 hours"): the Autoradio app''s logic outside voice is shared Exact code as far as it can be proved without the owner awake; research on the paper searches its own corpus by meaning as well as by words, from pgvector, with stories linked to the pages they cite; and every recurring agent failure seen this week has a trace-backed cause or a record that its cause is not established, and the calibration experiment is redesigned, reviewed and its pilot started or the reason it could not start reported.'
  artifact: 'three things on main and staging. (X) each area of the app moved to shared code and merged, its Swift deleted, and shown on the iPhone simulator and the Android emulator with the drill account where a tour ran (the report names the areas with no tour); the build on the owner''s iPhone is the newest one that keeps it signed in and is installed, never launched. The cutover file''s own proofs that need the owner (a locked hour on the phone, the owner''s ears, signing in again) are NOT part of tonight''s finish: they are listed in the morning report as waiting. (V) pgvector in Node B''s Postgres holding an EmbeddingGemma 2 vector for every public published story version and one for each public captured page as far as the first pass reaches (the count is read at the check-in), an embedder on Node B that keeps them current, and a corpus search in vector order with full-page BM25''s top three kept, which walks from a story to its cited pages. The capture pass takes seven to eight hours on the Mac and starts before the tables exist, so the count of capture vectors imported is read at the check-in, not promised whole. (L) a problem record per recurring failure with its cause classed as the model''s, the harness''s or not established, a redesigned and reviewed calibration experiment E1, and its pilot started on test computers or the reason it could not start.'
  acceptance:
    - action: 'scripts/check-autoradio-native-boundary on main (built by station XG)'
      proves: how many Swift sources and interface names remain outside the voice list; the count only falls
      evidence_class: gate output in CI
    - action: the Android emulator tour and the iPhone simulator tour for each merged Exact area, signed in as the drill account against staging, driven by the lead through Exact's agent on the simulator and the emulator, kept as the tool's tree output and screenshots. On the simulator the script is mobile/scripts/proof-real.sh (not demo mode; the drill computer's key placed for the simulator build to take at launch; reads only). mobile/scripts/proof-sim.sh is the demo smoke, with bundled fixtures and no network, and proves only that the pages build and draw. Android has no signed-in tour script yet; the lead commits one with the integration landing, under station XG
      proves: the moved page shows real staging data on both, from one source; only where a signed-in tour ran (an area with the demo smoke alone is reported as demo only, and an area with neither as merged and unproved)
      evidence_class: a rerunnable script's output against staging; the lead reads it, a second seat may rerun it
    - action: scripts/status-vectors (built by station V2), which reads corpusd's journal on Node B
      proves: 'have, want and remaining for story vectors and capture vectors: coverage of the first pass'
      evidence_class: a count from production's own journal line, not written by the author
    - action: the fixed 200-story retrieval set of the design (docs/design/corpus-first-retrieval-and-research-2026-11-18.md, section Fusion and windows) run against the deployed search by a committed script (station V4 commits the script and the set rebuilt with real page text; the 18 November files are in the lead's scratch folder, not the repo)
      proves: recall at ten of 0.92 or more (the design's bar for S6), and pages not in English in the top ten for 65% or more
      evidence_class: deployed proof
    - action: python3 -I scripts/status-corpus-reuse on the paper after each deploy
      proves: web searches a run, the share of reads from held pages, the brief's outcomes
      evidence_class: production counts (ledger C141, C142, C143 and the claims this file adds)
    - action: the run ids on the drill computer named in each trace record, exported again with choir trace export
      proves: the cause stated for each recurring failure, or that it is not established; the failure list was fixed in this file at 05:20Z, before the reading
      evidence_class: trace evidence anyone with the drill key can re-export
    - action: 'stations V5, V6 and V7a to V7d: each station''s own line in the design''s table (S7 to S12). L2: the protocol on main with both seats'' verdicts recorded. L3: the protocol''s own reading rules. L4: the problem record docs/problems/dated-switch-gate-turned-main-red-at-midnight-2026-11-19.md lists the eight entries and what each got, and scripts/check-switch-registry --today 2026-11-26 reports fewer failures than nine'
      proves: what those lines say, when each lands; none is expected by the check-in and the report says where each stands
      evidence_class: per station, as its line names
  rollback: each landing reverts as one. Vectors are a derived index (drop the tables and stop the embedder unit; BM25 search is untouched). Each Exact station reverts as one landing and the phone rolls back by installing the build before it. E1 ends by clearing its arm settings on the test computers.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance, device_install]
value:
  better_means: 'three gaps close. The share of the app''s logic that exists once and runs on both phones rises to all of it outside voice. The share of research that stands on what the computer already holds rises, measured as web searches a run and reads from held pages. The share of recurring failures with a trace-backed cause, or a record that the cause is not established and what would establish it, rises to all of them.'
  goodharting_would_be: 'One native call that decides everything behind a thin TypeScript wrapper. Swift deleted with the behaviour it carried gone untested, or a station called landed with no build on a device. Vectors stored and counted but never used by a search a desk makes; the vector half decorative because BM25 already ranked the same pages; pages reachable only through a story that cites them; or a recall figure taken on questions written from the indexed text. A cause written for each failure from reading code and not the run''s ledger; a failure list chosen after the reading; or a prompt changed to make one bad run look better. An experiment started with arms nobody reads.'
homotopy:
  realism_axis: 'How much of the real system each track runs on, on how real a subject: from one page on the web host with a canned fetch, to every page on the owner''s locked iPhone; from 449 trial passages on the Mac, to every published story and every captured page on Node B answering the paper''s own research runs; from one loop read in one run''s ledger, to an experiment whose arms run on five computers for days.'
boundaries:
  mutation_class: red
  authority_sources:
    - the owner's words of 2026-11-19 (owner_rulings below)
    - CLAUDE.md
    - docs/choir-doctrine.md
    - docs/definitions/choir-autoradio-exact-cutover-2026-11-18.md (track X)
    - 'docs/design/corpus-first-retrieval-and-research-2026-11-18.md (track V; its section "Amended 2026-11-19" records what rulings R2 and R3 changed)'
    - docs/reference/EMBEDDINGGEMMA.md, docs/reference/EXACT.md, docs/reference/CAPTURE-FULLTEXT.md
  must_preserve:
    - the owner is asleep and nothing waits for the owner; a step only the owner can do is recorded and the work goes round it
    - the owner's iPhone is never launched tonight (the app speaks at launch); builds are installed, not run
    - the owner's iPhone stays signed in; a build that would sign it out is not installed
    - 'no switches: a station replaces and deletes; vectors and fusion ship on'
    - no limits on our own model calls; the embedder refuses no request and counts none; the embed worker sends one batch at a time and has no pacing
    - vector rows have one writer, corpusd; the Mac's first-pass file is input to corpusd's import
    - a corpus search never fails because the embedder or the vector list is unavailable; it answers by BM25 alone and says so
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
    scope: 'two tables in corpusd''s database: one row per passage of every published story version, one row per public capture (title plus lead passage); 256 dimensions at half precision with the model''s hash; an HNSW index; a worker whose work list is derived (rows with no vector, newest first, stories before captures), which sends one batch at a time and logs have, want and remaining on a journal line that scripts/status-vectors reads. No queue in memory, no pacing, no limit, nothing dropped. corpusd is the only writer of vector rows; the import of a first-pass file runs the same checks as the worker. Window filtering with HNSW is post-filtering (the design''s objection): the windowed search asks the index for more than it returns and falls back to an exact scan of the window when too few survive; the test says so.'
    class: red (corpusd schema; a new writer)
    review: two seats
    depends_on: [V1]
    carrier: none yet
    status: pending
  - id: V3
    name: The first pass, on the Mac
    scope: 'an export of public text from Node B (every public story version; each public capture''s title and lead passage), embedded on the Mac with Metal by the trial command (mobile/EmbedTrial), imported as vectors; a sample re-embedded on Node B must agree at cosine 0.999 or more; the text copy on the Mac is deleted after the import. Stories first (about an hour for 61,000 passages), then captures (seven to eight hours).'
    class: yellow (operations); the import it feeds is V2's red code and takes V2's review
    review: 'none for the export and the Mac tool beyond the lead reading the export script (done; every read is confined under the artifacts folder). A partial pass is safe (the worker fills the rest). The text copy is deleted when the import is accepted or at the check-in, whichever is first.'
    depends_on: []
    carrier: lane/embed-pass f87b0e977; the pass has been running on the Mac since 05:39Z
    status: working
  - id: V4
    name: 'One ranked answer: vector order with full-page BM25''s top three kept, and the walk from a story to its pages'
    scope: 'corpusd answers one search in vector order (story passages and capture leads), with BM25''s top three pages guaranteed a place: the design''s measured ship rule (0.926 recall at ten; reciprocal-rank fusion measured 0.856 and is not used). A story hit returns the captures that story cites (the object graph''s citation edges), latest version of a publication only. When the embedder or the vector list is unavailable the answer is BM25 alone and says so. The journal line gains vector= and bm25_kept= so a reader can tell which half answered. The guest''s reciprocal-rank merge is deleted and docs/reference/CAPTURE-FULLTEXT.md section 5 corrected. The corpus brief and choir.SearchWireCorpus read this answer. Rolled back before V2 is.'
    class: orange
    review: one seat
    depends_on: [V2, V3]
    carrier: none yet
    status: pending
  - id: V5
    name: The event layer reads research's pages, with a vector shortlist
    scope: station S7 of the design; a capture joins an existing event by vector shortlist and Jev's judgment, replacing the title-overlap slot
    class: orange
    review: one seat
    depends_on: [V3, V4]
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
  - id: V7a
    name: The neighbourhood read
    scope: station S9 of the design; used by the brief, management tasks and intake labels
    class: orange
    review: one seat
    depends_on: [V4, V6]
    carrier: none yet
    status: pending
  - id: V7b
    name: The fold check at first publish
    scope: 'station S10 of the design; the rule is the design''s default, which the owner accepted with "sure" (R7): same primary event and revised within 72 hours, or Jev same/update at 0.8 or more'
    class: red (crosses Texture custody)
    review: two seats
    depends_on: [V6, V7a]
    carrier: none yet
    status: pending
  - id: V7c
    name: Entity merges
    scope: station S11 of the design
    class: orange
    review: one seat
    depends_on: [V6]
    carrier: none yet
    status: pending
  - id: V7d
    name: The front and the radio deduped by event
    scope: station S12 of the design
    class: orange
    review: one seat
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
    class: red (it decides what runs on live test computers and what is deleted)
    review: two seats
    depends_on: [L1]
    carrier: none yet
    status: pending
  - id: L3
    name: E1 runs
    scope: 'the redesigned protocol''s first experiment started on test computers made for it; ruling R6 is the owner''s act that starts it (the 4 November protocol''s separate start act and its twelve-hour quiet window belong to the old design and go with it); arms are paired on the same objective and run back to back, so a release change during the night falls on all arms alike and the reader records the release of every run; the paper, the owner''s computer and a real user''s hold no arm; a reader that needs no live session; an arm ends at its recorded expiry and persists across a restart only as that record'
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
    default: hold the phone on the last build before the sign-in station. The key is carried in place only if a test shows an item written by today's code is read by the new path and the station's two seats accept it; a build that would sign the phone out is never installed
  - id: D2
    question: Which computers hold an E1 arm?
    why: 'the 4 November design names five test arms; the paper and the owner''s computer must hold none'
    options: the five test computers of the old design; the drill computer only; fresh test computers made for the experiment
    default: fresh test computers made for the experiment (the redesign proposes one for the model arms and one for the prompt arm, with the drill computer as fallback), never the paper, the owner's computer or a real user's; the owner may rule otherwise
  - id: D3
    question: Do captures get a vector for every passage, or one per page?
    why: 'one per page is 215,000 vectors and seven to eight hours on the Mac; every passage is about 850,000 and one to three days'
    options: 'stories only, pages reached through the stories that cite them (the owner''s proposal in R3); stories plus one vector per page (the lead''s answer); every passage of every page (the 18 November design)'
    default: 'stories plus one vector per page. This is the lead''s proposal, not the cheapest option: the owner''s stories-only proposal costs one Mac-hour, the page vectors seven or eight more on a Mac that is also building. The pass therefore runs stories first and imports them alone if the page pass is not finished; the page vectors can be dropped without touching the story path'
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
    read_as: 'a question and a proposal from the owner, not a ruling. Taken from it as settled: stories are embedded, and a story hit walks to the pages it cites. The lead''s answer, not ruled on (decision D3): also one vector per captured page, since a page no story cited cannot be reached through stories; going forward Node B embeds everything published and one vector per new capture.'
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
    read_as: 'stations V5 to V7; the licence question is closed; lane/search-ceramic is kept and not landed; TestFlight is excluded; "sure" accepts the revision rule the lead had proposed, which is the design''s default (same primary event and revised within 72 hours, or Jev same/update at 0.8 or more)'
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
  slice: 'three tracks at once. X: the two Exact lanes and the boundary gate are joined in lane/exact-base (contract build 9 s); eight builders move the signed-in request seam, the deck, the run sheet, podcasts, files, mail, content and settings from it in parallel, each against an agreed signed-in request seam, and the lead merges and tours them; the cutover file''s order (sign-in, then the deck, then the run sheet) is the order they MERGE in, not the order they are written in. V: the faster corpus search lands; pgvector and the embedder are built; the Mac starts on stories. L: the traces are read.'
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
    claim: If stories and one vector per captured page live in pgvector, ranked with full-page BM25''s top three kept and walked through citations, then a research run on the paper is answered by meaning from what the computer holds.
    test: 'ledger C144 (its middle clause reads "answered from the fused lists"; the observable reading is the journal line''s vector= field): by 2026-11-19T18:00Z, vectors held for every published story version and 95% or more of public captures (scripts/status-vectors), a corpus search a research run makes on the paper answered with the vector half (the journal line''s vector= field, added by V4), and the 200-story set at 0.90 or more against the deployed search (C144''s bar, entered before the review; the acceptance bar above is the design''s 0.92). The effect on web searches a run is a second claim, entered when V4 deploys, with the brief''s hours (C141, C142) as its baseline.'
    edge: independence
    delta_o: 'a control for the effect claim: the same 24-run window read before and after V4 with the brief already live, so the brief''s effect is not counted as the vectors''; only runs after the faster search (V0) deployed are read, and each run''s release is recorded. If the brief did not change what the model searches for, better retrieval may not either, and the lever is in what the desk is asked to do, which track L reads.'
    scope_if_supported: the paper's research desk at today's corpus
    status: active
    evidence_refs: []
  decision:
    what: 'pgvector with EmbeddingGemma 2 at 256 dimensions and half precision; stories in full and one vector per capture; the Mac for the first pass and Node B from then on; Exact stations in the cutover file''s order with pages in parallel; no prompt or harness change before its trace is read'
    kind: architecture
    status: proposal
    evidence_ref: 'docs/design/corpus-first-retrieval-and-research-2026-11-18.md, section "Amended 2026-11-19"'
    owner_ratification_ref: 'R2 settles pgvector, the model and the Mac; the dimension and one vector per capture are the lead''s proposal until the owner rules (D3); the order of the Exact stations is the cutover file''s decision D1'
  belief:
    believed_state: 'full-page BM25 is live and every corpus search returns a page; searching for need cut web searches a run from 7 to 4; the corpus brief is on main and not yet deployed; a corpus search takes two seconds and the fix is landing; no vector exists on the server; the Exact move stands where it stood yesterday morning, with two unmerged lanes; nothing links stories by event'
    main_uncertainty: 'whether nixpkgs'' llama.cpp carries the EmbeddingGemma 2 architecture and whether pgvector fits Postgres''s 4 GiB cap with an HNSW index; and for Exact, how much of a red station can be proved with no ears and a phone that must not be launched'
    next_observation: 'the brief''s first thirty journal lines on the paper; the nix evaluation of the embedder unit; the rebased Exact lanes'' loop time'
  blocker_or_risk: 'the builder machine has 4 GB, so Go builders run two at a time; every server landing takes about 75 minutes of suites and they run one at a time, so the twelve hours hold eight or nine; the Mac carries the embedding pass, the Exact loop and the native builds at once; sound and the locked screen cannot be judged until the owner is awake'
  next_action: start the first wave of builders (Exact lanes rebased and measured, the boundary gate, the trace read, pgvector and the embedder) and land the faster corpus search
receipts: []
---

# The night of 19 November 2026: Exact, vectors, and learning

Standing: superseded | kind: wanted | settled-by: the successor's supersedes field (its line 4) and line 257 | replaced-by: docs/definitions/choir-exact-search-entities-2026-11-20.md | note: the successor is itself superseded by choir-exact-cutover-and-ontology

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
design's objection to Postgres. Stories in full are about 61,000 passages across
35,000 versions, about an hour on the Mac; with them the whole index is about
140 MB.

## What "tonight" can mean

Three bounds are physical, not choices.

- The Mac embeds about 3,000 tokens a second. Stories take about an hour;
  one vector for each of 215,000 pages takes seven to eight hours.
- A server landing takes about 75 minutes of suites, then CI and a deploy,
  and they run one at a time: eight or nine before the owner's check-in.
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
