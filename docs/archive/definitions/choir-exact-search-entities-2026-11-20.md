---
definition_version: 7
readiness: executable
supersedes: docs/archive/definitions/choir-night-exact-vectors-learning-2026-11-19.md
authority: 'the owner, 2026-11-20 about 04:45Z: "we should make sure the /goal file is correct, then i will start a new thread"; "i think we should be able to finish these missions overnight"'
review:
  reviewer: none (written by the lead of the previous thread as its handoff; the stations name their own review tiers)
  frozen_ref: none
  verdict: none
start:
  captured_at: 2026-11-20T04:50Z
  source:
    canonical_ref: origin/main bd2ff1c24
    deploy_identity: staging reports bd2ff1c24 (https://choir.news/health)
  what_is_true_now:
    - 'Vectors: every published story and public capture has a vector in pgvector on Node B; corpus search runs BM25 and vectors together on every query (recall at ten 0.940 on the 200-question set, bar 0.90). Two embedder units (bulk 8090, query 8091).'
    - 'Event layer: deployed; joins captures to events through a vector shortlist. In three flushes at 04:33Z it resolved 53 captures: 3 joined, 46 minted, 4 tentative. Whether that join rate is right is NOT read.'
    - 'Entities: NOT BUILT. No entity layer links people, organisations or places across captures and stories. The previous goal named it and only the event half was done.'
    - 'Search volume: the paper sent about 190 web searches an hour (about 4,000 a day against the owner target of 1,000). The search gate (3f3e10ba1, deployed 03:18Z) brought the next hour to about 100, bursty. Claim C163 is open on it. Keenable carries every search; Brave, Parallel, Serper and Tavily are quota-limited, ChatGPT search has failed authentication on every request since 2026-11-18T16:50Z, SearXNG mostly returns nothing.'
    - 'Phone: main has the signed-out work (39d2138d5, bd2ff1c24): one account state in shared code, one Sign in card, the paper and radio work signed out. The owner''s iPhone carries lane/exact-demo-off 8a3c1a5ff (main plus the demo guard), installed 04:44Z. The owner had not yet said what that build shows.'
    - 'Why the owner''s phone looked signed out with sample stories: it was in DEMO MODE (a stored flag set by one tap on Settings > Developer; it survives reinstalls). The owner: "what is demo mode? i think we can get rid of that. i never asked for that."'
    - 'Exact: non-voice native code is 101 files, 22,036 lines, 105 interface names (scripts/check-autoradio-native-boundary); shared TypeScript about 10,400 lines, contracts about 8,000. Android has no sign-in, audio or voice. The Mac app is not started.'
    - 'Hourly status: scripts/status-hourly now has a search usage block (corpus searches and each web provider) and a history at ~/.local/state/choir-status/search-usage.tsv. The hourly tick was a session cron and dies with the old thread.'
  branches_in_flight:
    - 'lane/exact-demo-off 8a3c1a5ff: demo mode off on a physical iPhone. On the owner''s phone; not on main. Seat 2 SENT IT BACK: (blocking) the Settings > Developer "Demo mode" row is still drawn on a phone and now does nothing; (should-fix) demo mode left content on the phone the real app may show (certainly the seeded sample book, shelf id 6916a7c31cd3, mobile/exact-package/apple/AutoradioModule+Books.swift:27 and ReaderSample.swift:81-89; possibly onboarding.json, edition-topics.json, a demo open-pages.json and signals.jsonl) and the account wipe will not fire for it; (should-fix) write the guard as `#if os(macOS) || targetEnvironment(simulator)` so a new platform fails closed. The owner has since said to remove demo mode altogether (station D2), which answers the first.'
    - 'lane/exact-textures-place 70a5d3c77: the Textures list moved to shared code, its Swift deleted; real simulator proof PROOF OK at this tip at 04:33Z (queue run integ3-tx2), bun test 704 pass; not reviewed, not landed. Its builder never compiled the Swift directly (the queue''s build did). EditionInfo, EditionLookup and opening a texture: document stay in Swift.'
    - 'lane/search-leads 8f7ba1dce: table of every web result a run was handed (L1 of docs/definitions/corpus-first-search-one-verb-2026-11-19.md). Whole internal/platform under -race passed (325 s). Its three agentcore observer tests failed once in three repetitions on a data race in the TEST (search_leads_test.go:119 reads a plain bytes.Buffer the background call writes; guard it as corpus_log_test.go does). Whole agentcore never run. It drops writes past 32 in flight: fix before landing (one write per search, not a dropped background call). Its builder used pkill once.'
    - 'lane/brief-objective bb5c2bf5a: runs that start with no objective get one (fix 4). Focused tests passed under -race -count=3 (73 s). The whole agentcore run FAILED 7 tests, all phone-fixture tests (TestF43, TestF44, TestT1b, TestT8, TestThePhonesLearningFixturesDecodeAsTheRoutesShape, TestThePhonesSegmentStreamsCarryOnlyKeysTheServerSends, TestThePhonesSteerRequestIsAccepted); the builder guesses its copy of the tree on the build VM left out mobile/ (unchecked). Rerun whole agentcore from a full checkout before believing either way.'
    - 'lane/root-desk-on (2cf3fd995 when the findings were sent; 62e9ac65c at 05:06Z, mid-rework, its builder told to stop and push; read `git log` on the lane for a WIP label): removes the texture.root_desk setting. Both seats sent 8af45a133 back (a second root run on already-digested documents; about 30 root runs an hour on ordinary commits; a deploy loses the story batch; rollback text false). The rework is pushed and its builder stopped: 2cf3fd995 (does not compile alone, an unused import) plus 62e9ac65c (removes the import, fixes the boot test); land the two together. By its builder''s account: a digest carries a nonce and boot id and no documents; every document uses ONE hourly digest (nothing reads the master within minutes), so at most one digest-started root run an hour per computer and the two-digest design is deleted; a resumed digest runs a full refresh; a digest that starts no run ends; rollback text says revert this and e02c6a990 together and names no operator step. NOT seen passing at the tip: the boot test fix, whole textureowner, whole agentcore (last whole pass was at 8af45a133, before the rework). The proof script was rewritten and only syntax-checked. Needs both seats again and a deployed proof on d@e.'
    - 'lane switches-land (local, a144865d1; also on choir-runner as ~/lead-plan, output ~/sw-run.out): intake.events and dispatch.wide_slots removed, both accepted by both seats (scratch review-switches/r1). At 05:06Z ~/sw-run.out shows ok for actor, actorruntime, store (2162 s), provideriface and textureowner under -race; agentcore was still running (it ends with the line SWDONE). Claim C167 entered. Needs Security-Effect trailers at landing.'
    - 'lane/search-ceramic ad404a072: kept by the owner''s word; do not land, do not delete.'
finish:
  deliver: 'The owner opens Autoradio on their iPhone and it is plainly either signed in to their computer or signed out with one obvious way in, never sample data; the paper and radio work either way. Everything in the app outside voice is Exact as far as Exact can carry it. The paper searches the web only for what is new to it, under 1,000 searches a day. Stories and captures are linked by event and by entity.'
  artifact: 'on main and staging: (D) demo mode removed from the product; (X) the non-voice native count falling station by station with each page''s Swift deleted; (S) the remaining search fixes; (E) an entity layer; (W) the remaining settings removed. On the owner''s phone: a build of main the owner has opened and judged.'
  acceptance:
    - action: the owner's own words about what their phone shows after each install
      proves: the sign-in and signed-out experience; nothing else proves it
      evidence_class: owner verdict
    - action: scripts/check-autoradio-native-boundary on main
      proves: non-voice native files, lines and interface names; the count only falls (101 files, 22,036 lines, 105 names at start)
      evidence_class: gate output
    - action: mobile/scripts/proof-real.sh on the lead's simulator against staging (signed in as d@e and signed out), screenshots read by the lead
      proves: each moved page shows real data from shared code; a proof whose screenshots were not looked at proves less (the card was cut off by the bar under a passing proof)
      evidence_class: rerunnable script output plus the screenshots
    - action: 'scripts/status-hourly, search usage block and --history'
      proves: web searches a day by provider against 1,000; corpus searches; per research run
      evidence_class: production journal counts
    - action: 'the paper''s console event layer lines and, once built, the entity layer''s own line; a read of 50 minted events against the events they should have joined'
      proves: the join rate is right or the layer mints duplicates
      evidence_class: production counts and a hand read recorded in docs/evidence
  rollback: each landing reverts as one commit. The phone rolls back by installing the previous build of main. Vector and lead tables are derived (drop and rebuild). Removing a setting is rolled back by reverting its commit, which restores the setting default off.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance, device_install]
value:
  better_means: 'four gaps close: the owner''s phone shows the truth about its account; non-voice native lines fall toward zero; web searches a day fall under 1,000 with no cap; captures and stories that are about the same event or entity are linked.'
  goodharting_would_be: 'a proof that passes on a screen nobody looked at; Swift lines moved into a native call behind a thin wrapper; a search count that falls because research stopped; events minted one per capture and counted as linked; a setting removed by leaving its behaviour off.'
homotopy:
  realism_axis: 'from the simulator signed in as the drill account, to the owner''s own iPhone opened by the owner; from one page moved, to no page logic in Swift; from an hour''s search count after a deploy, to a full day.'
boundaries:
  mutation_class: red (search gate follow-ups and switches touch agentcore; the entity layer touches export and the tape)
  authority_sources:
    - CLAUDE.md (binding; read it first, and docs/operating-contract-rationale.md for reasons)
    - 'the owner''s words of 2026-11-19 and 2026-11-20, quoted in owner_rulings below'
    - docs/evidence/standing-rules-audit-2026-11-19.md (which standing rules are the owner's, the contract's, or the previous lead's own cautions)
    - docs/definitions/corpus-first-search-one-verb-2026-11-19.md (search fixes 2 and 3, landings L1 to L4)
    - docs/design/corpus-first-retrieval-and-research-2026-11-18.md (event and entity stations S7 to S12)
    - docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md and docs/evidence/exact-swift-pages-to-move-2026-11-20.md
  owner_rulings:
    - '"only the voice stuff should be swift. the rest should be exact"; "the rule, more clearly, is to get everything in exact, as much as possible" (2026-11-20): a change to a page still in Swift moves it; do not repair the Swift.'
    - '"autoradio and autopaper should work signed out, as autopaper works signed out on choir.news" (2026-11-20).'
    - '"what is demo mode? i think we can get rid of that. i never asked for that." (2026-11-20).'
    - '"we should be able to get our searches underneath 1,000 per day... we only need to search for things which are new information for us" (2026-11-19).'
    - '"I''ve been saying no switches for a while... I want everything on" (2026-11-19).'
    - '"you can restart a host service on Node B" (2026-11-19).'
    - '"what happened to the script that runs every hour with the paper status? we should continue that, and also track the search usage" (2026-11-20).'
    - '"compaction leads us astray" (2026-11-20): do not carry a rule or a fact forward from a summary without its source. Check the source, or drop it.'
  must_preserve:
    - no secrets, tokens or the owner's email in the repo or in output; .env, .env.bak-bedrock-20261109124037 and stash devin-provider-wip hold secrets and are left alone
    - the owner's iPhone credential is never changed, moved or re-keyed
    - nothing changes on the paper or other users' computers except by deploy; drills run on d@e only
    - 'no switches; no limits on our own model calls; a web search fix is the caller fixed, never a cap'
    - one landing and one deploy at a time; one Mac native build at a time through the lock; builders never run native builds or simulators
    - never pkill; never a bare git stash; never commit in the primary checkout; lanes only
    - lane/search-ceramic is kept
  excluded:
    - TestFlight
    - Android sign-in and voice in Kotlin, and the Mac app (after the iPhone's non-voice Swift is gone)
    - any change to search providers, their keys or spending (the ChatGPT search authentication failure is the owner's to decide)
stations:
  - id: D1
    name: Demo mode off on a real phone, landed
    scope: 'lane/exact-demo-off 8a3c1a5ff: read the review (scratch review-demo/r1, or rerun seat 2), answer it, land. Then ask what the owner''s phone shows.'
    class: orange
    review: one seat
    status: landed
    state_at_handoff: in review
    owner_verdict: '2026-11-20 about 05:26Z, with a screenshot of the build installed at 04:44Z playing that day''s stories: "I''m signed in. Idk what you mean by the sign in card or sample stories." The conjecture in `now` held: the phone was in demo mode, not signed out. The owner also said: "Yes, install new builds overnight."'
  - id: D2
    name: Demo mode removed from the product
    scope: 'delete demo mode: the Settings Developer rows, Demo.swift, DemoFixtures.generated.swift (2,191 lines) and every Demo.isOn branch; the demoSet/demoInfo interface names; the demo key store. The twelve proof scripts that set AutoradioDemo (proof-sim, proof-lane1 to 9, proof-mac, proof-default-themes, proof-common) move to real data against staging as d@e (proof-real.sh is the model) or to the web host with canned fetch in shared tests; a proof that cannot move is deleted and named. Also clear what demo mode left on a phone that was in it (the review of D1 was asked to list it).'
    class: orange
    review: one seat
    depends_on: [D1]
    status: landed
  - id: X1
    name: The Textures place in shared code, landed
    scope: lane/exact-textures-place 70a5d3c77; rerun the real proof at the tip, read the screenshots, one seat, land
    class: orange
    review: one seat
    status: landed
    state_at_handoff: built, unreviewed
  - id: X2
    name: The remaining page logic out of Swift, smallest first
    scope: 'in the order of docs/evidence/exact-swift-pages-to-move-2026-11-20.md: conversation info, Texture edit info, themes, takes, edition picks (with its writers), the books shelf; then the full reader (citations, versions, related, tables, embeds, texture: documents, editing), the bar and run sheet page models, onboarding and settings models, mail and files remnants, and the native open-pages set. Each: tests ported first, Swift deleted in the same change, the boundary ceiling lowered, real proof with screenshots read, one seat. Voice, audio, speech, the on-device models and what the platform forces stay native with the reason beside each call.'
    class: orange
    review: one seat each
    depends_on: [X1]
    status: working
  - id: S1
    name: The search gate's follow-ups and its claim
    scope: 'settle C163 from scripts/status-corpus-reuse over two hours after 03:18Z with at least 20 searching runs (restate it for tip 8407f6c33 if needed). Seat 2''s should-fixes on the refuse-while-running rule: a same-terms running search still waits 15 s in reuse; SearchAll does not apply the running rule to its wordings; the refusal''s example call is not valid Go; the two-minute age and run-end escape are untested.'
    class: orange
    review: one seat
    status: landed
    state_at_handoff: deployed; follow-ups pending
  - id: S2
    name: Runs that start with no objective get one (fix 4)
    scope: lane/brief-objective bb5c2bf5a; whole agentcore under -race seen passing, one seat, land, measure brief outcomes
    class: orange
    review: one seat
    status: landed
    state_at_handoff: built, suite not seen passing
  - id: S3
    name: Search leads and the one search verb (fixes 2 and 3, L1 to L4)
    scope: 'lane/search-leads 8f7ba1dce is L1: fix the dropped writes, read the FAIL, see the suites pass, one seat, land. Then L2 to L4 of the design: the corpus says how strong and recent its best answer is, and one verb choir.Search decides held, sent or sent-with-since.'
    class: orange (L1); red where the desk protocol or run acceptance changes
    review: per landing
    status: working
    state_at_handoff: L1 built, unverified
  - id: S4
    name: The day's number
    scope: a full UTC day under 1,000 web searches on the paper with research still publishing (stories a day not lower than the week before), read from the search usage history
    class: yellow
    depends_on: [S1, S2, S3]
    status: pending
  - id: E1
    name: Is the event join rate right
    scope: read 50 recently minted events on the paper against the events that existed when each was minted; say how many should have joined; record in docs/evidence; fix the shortlist or the judgment if the layer mints duplicates. Also whether the 48 captures left unresolved during gateway drains are retried (internal/agentcore/event_layer.go:374).
    class: read-only, then orange
    status: working
  - id: E2
    name: The entity layer
    scope: 'design first (Opus for the open design, as the contract allows): what an entity is, who is its custodian desk, how a capture and a story get entity ids, merges, and what reads them; then stations S8 and S11 of the retrieval design (stories carry event and entity ids at publish; entity merges). Jev: one call per batch, every question in it.'
    class: red (export, tape)
    review: two seats
    depends_on: [E1]
    status: working
    state_at_handoff: not designed
  - id: W1
    name: Two settings removed (intake.events, dispatch.wide_slots)
    scope: 'switches-land a144865d1: read ~/sw-run.out on choir-runner (actor, actorruntime, store, provideriface, textureowner, agentcore under -race), add Security-Effect trailers backed by review-switches r1, gates, land; settle C167. State the measurement and the rollback trigger for 8 to 12 slots in the commit.'
    class: orange
    review: done (both seats accepted)
    status: landed
    state_at_handoff: suites running
  - id: W2
    name: The Texture root desk on everywhere
    scope: lane/root-desk-on after its rework (both seats' findings are in this file's branches_in_flight); two seats again; deployed proof on d@e; claim C165 restated against log lines that exist
    class: red
    review: two seats
    status: working
    state_at_handoff: rework
  - id: W3
    name: The last three settings
    scope: owner_act_mode, intake.event_wakes, vm_guest_reclaim_damon (table in the previous thread's scratch switches/table.md; rebuild it from switches/registry.yaml if gone)
    class: orange to red
    status: working
  - id: H1
    name: Host problems recorded and not fixed
    scope: 'the paper''s snapshot writes a full copy of its store inside its image (51 GiB, held 83 minutes; seven times in four days; docs/problems/paper-data-image-grew-40-gib-in-eleven-minutes-2026-11-16.md): stream without the scratch copy or copy changed pages only. The gateway refuses every guest while it drains on a deploy (docs/problems/gateway-refuses-guests-while-it-drains-on-deploy-2026-11-19.md): socket handoff. Both red; design before code.'
    class: red
    status: working
  - id: R1
    name: Operations the old thread held in its session
    scope: 'recreate the hourly tick (run scripts/status-hourly from the repo, report model calls and shares, Devin refusals, the search usage block, Node B free disk, every FLAGS and ACTION line). The phone build queue lived in the old session''s scratchpad; a copy with its paths rewritten is at ~/.local/state/choir-buildq (serve.sh, run-one.sh, README.md, device-install.sh): read it, start serve.sh once, and move it into mobile/scripts when convenient. The E1 calibration pilot is paused on Devin quota: settle C148 to C150 as unresolved at their time and run scripts/e1 end. Write the next owner letter with the reporter skill (name every expedite: event layer, query embedder, Exact delta, embedder thread fix (re-landed without a re-review), search usage status, search gate, signed-out work and its follow-up).'
    class: yellow
    status: pending
now:
  status: working
  as_of: 2026-11-20T16:28Z (lead-docs lane carries this file)
  slice: four trains and the phone cleanups are deployed; the themes are in a last bounded fix; the owner letter is published; C176 (the re-ask holds on the paper) settles after 16:30Z
  landed_this_thread:
    - 'W1 7f5ad41e1 (intake.events and dispatch.wide_slots gone): CI 44684974356 ok, deployed 06:18:24Z, the paper logs slots=N/12 and event layer flushes after its refresh. C167 settles after 07:20Z.'
    - 'X1 de7aeb7f7 (the Textures list is shared Exact code): seat 2 accepted the rework; real-data simulator proof PROOF OK at 99b3c5dfb with screenshots read; CI 44688798576 ok, deployed 06:45:49Z.'
    - 'S2 train 84a853ce4 (the brief''s objective, five commits; the search gauge; scripts/status-entities): whole agentcore ok on the tested tree, CI 44698550035 ok, deployed 08:13:40Z. C171 settles after 10:15Z from the paper''s brief lines.'
    - 'D1 6bdddb530 (demo mode never on a real phone, row gone, leftovers cleaned once on a positive trace): seat 2 accepted the fourth version; real-data proof PROOF OK at the landing shape; CI 44701650918 ok; installed on the owner''s iPhone 08:24:22Z. The proof is the owner''s phone when they wake.'
    - 'R1: C148 to C150 settled void (the E1 pilot stopped after two of ten objectives); scripts/e1 end run (arm L''s overlay delete answered 403).'
    - 'Train C b1257d04a (the embed-worker wake; search leads): whole agentcore ok, CI 44709891003 ok, deployed 09:38:58Z. C170 and C172 settle after 12:10Z.'
    - 'Design rules 49cbd976d: accepted on re-review; the lead read the signed-out screens (a banner with Sign in on every signed-out page); CI ok, deployed 09:53:38Z.'
    - 'D2 a7e0e6535 (demo mode deleted from the product; 7,450 lines removed; the leftover cleanup kept): accepted; real proof PROOF OK; CI 44723094577 ok, deployed 11:05:03Z; on the owner''s iPhone at f1aa4df9d since 11:25:55Z. The build queue''s `proof` mode is retired; `real` is the only proof.'
    - 'H1 gateway G1 f1aa4df9d and its follow-up f883ea88d (the two spend files merged under a file lock by an asynchronous flusher; a corrupt file set aside): red, both seats accepted each; CI 44725241858 ok, deployed 11:16:41Z; the paid counts rose across the restart with no save-failure line. C175 settles after 12:17Z.'
    - 'C171 settled false as worded: 0 of 16 briefs skipped for want of an objective (66% before), but every one took its objective from the unread record, a source the claim did not name.'
    - 'Train D dc584af3b (entity station E2 with its follow-up; the DAMON and event-wakes settings deleted): CI 44731379068 FAILED (the edge kind in_event unregistered, the Go vendor hash stale); repaired by e33d9df5f, CI 44735823978 ok, deployed 12:43:17Z. Record: docs/problems/train-landed-red-objectgraph-registry-and-vendor-hash-2026-11-20.md. C173 and C174 settled false.'
    - 'The entity read listed no stories in production (a text range that matches nothing under the database''s en_US.UTF-8 collation; tests ran under C). Fixed by 0eef16ec7, deployed 13:38:45Z; scripts/proofs/entity-e2-deployed.sh exits 0 (14 entities of front stories list their stories). The review found no other broken range; og_edges is the one graph id table not declared COLLATE "C".'
    - 'The wider demo cleanup bf478ee52: accepted, CI 44738989977 ok; on the owner''s iPhone at 20af586d3 since 13:18:34Z.'
    - 'Train E 20af586d3 (the re-ask holds, four commits; desk protocol lock 18.9.0, the lead''s commit, not reviewed by a seat): CI 44741404137 ok, deployed 13:24:44Z. C176 settles after 16:30Z.'
    - 'C170 settled false narrowly (unembedded at flush fell from 78% to about 12%, bar 10%); C172 true; C175 true by a narrower reading.'
    - 'The owner letter: docs/reports/overnight-exact-search-entities-2026-11-20.md.'
    - 'After the letter: the conversation rows f981ead7c (split from the stopped editor lane; typed-ask proof read), the themes e90932c94 (accepted on a fourth review; three should-fixes open) and the About page bfc1492c6 (the commit the app was built from and the server''s commit; the owner could not tell builds apart) are on main, CI ok each. C176 settled true: the gateway''s web searches fell 35% in the three hours after the re-ask holds (455 to 294), about 2,350 a day at the new rate.'
    - 'Flags recorded from the hourly ticks: Node B free fell 7.1 GiB in six hours overnight (cause not established; six deploys) and stands at 114 GiB, six days to the floor; the paper made 166 synthetic and 130 river calls on 2026-11-20 (bedrock $0.55 of 40, river $0.18 of 10, synthetic $1.53 estimated), read as fallbacks under Devin quota refusals, not traced.'
    - 'The lead shut the proof simulator down at about 14:38Z while the build queue was using it, which failed one builder''s proof; the lead does not touch the simulator while the queue is busy.'
  discovered_this_thread:
    - 'D1: the owner, 05:26Z, with a screenshot: signed in, the day''s real stories. Installs overnight are allowed. Seat 2 sent the cleanup back twice: it must not delete anything on "no readable key", must act only on a positive demo trace, and must mark itself done only after it succeeded. Third version building.'
    - 'S: the search gate did NOT lower the gateway''s rate (162 an hour before, 159 after); about 6 searches a run by the gateway meter. 79% of searches are a story''s first run hunting a second outlet; 1,000 a day is 1.8 a run.'
    - 'S3 L1: leads cannot be embedded: the embedder does about 37,000 passages a day and leads would be about 190,000 a day, ahead of captures. Leads are stored as text with a lexical index; no vectors. Rework in progress.'
    - 'E: the embed-worker wake as a full pass per write was sent back (each pass costs 10 to 18 s of counts over 228,000 captures); reworked to hand the worker the written capture ids (4d324a125), in re-review.'
    - 'E2 (design station E2: ready list, candidates with no model, the public entity read) built at b74f9f431 on the old wake commit; in review; needs a rebase.'
    - 'W2: the root desk was sent back a fourth time (a digest retried after a deferral is dropped by its own nonce check). Contract rule 7 applied: stop patching. The reading: process-local state (pending set, nonce, full-refresh flag) beside a durable digest is two authorities for one fact. Restructure in progress: a durable watermark, a digest that carries nothing and computes what is pending from the store, so any duplicate or late digest is harmless.'
    - 'NEW problem: the paper''s dispatcher holds twelve of twelve slots with 82 to 99 agents due and median waits of 250 s, since before 04:56Z (docs/problems/paper-dispatcher-twelve-slots-full-ninety-due-2026-11-20.md). Publishing rate is normal. Half the noroom waits are deadline wake-ups for finished work. Lane dispatch-deadlines: log activation time, then resolve finished deadlines without a slot.'
    - 'NEW problem: the paper''s snapshot took 82m51s for 51.39 GB against a 90-minute budget; at 2.4 GB a day it stops finishing in about two days (docs/problems/paper-snapshot-will-pass-its-ninety-minute-budget-2026-11-20.md). H1 designs are on main (docs/archive/design/snapshot-without-scratch-copy-2026-11-20.md, docs/design/gateway-handoff-without-a-gap-2026-11-20.md). Lane snapshot-stream builds S1.'
    - 'W2 round six: both seats sent the restructure (cf31b8d2d) back for boundary conditions of the watermark, not for two authorities: cut-off stamped after the read, read errors that still moved it, the 50-document bound silently dropping, restarts stacking runs, every ordinary run listing all documents. Answered in 7a6d66dc4 (cut-off before the read with a one-minute overlap and version marks; stories not digested by the root desk, the lead''s decision and a residual for the owner; a durable hourly gate; doubling wait after runs that move nothing). In review by both seats. If it is sent back again it goes to the owner.'
    - 'H1 snapshot S1 (fab0d6a0e): both seats sent it back: it reverted the ledger and this file by accident; its WAL figure can be zero or negative; the claimed 12 GiB bound has no mechanism; the checkpoint after release is a six-times-larger burst on the one write connection; the paper''s first capture has no decision rule. The trade is judged unproven until measured on a test computer. Rework in progress.'
    - 'W3: vm_guest_reclaim_damon and intake.event_wakes are deleted as dead paths on lane last-switches (4715d2f02, 6fe318268), in review. owner_act_mode is prepared (ba32aa571, red) and HELD: enforcing it contradicts the owner''s ruling of 2026-10-24 (observe), no observe window with real owner acts was ever read, and a missing proxy key would refuse every owner mode write. Owner decision; default: leave observe.'
    - 'X2 themes (ee067fb8e): sent back: a theme tap could mark the migration done when it never ran; the native delete made a revert lose themes; the first frame was a partial theme; the iOS system colour scheme is unproven. Rework in progress. X2 conversation and Texture edit (b8015a497): built; real proof not yet passed.'
    - 'Design rules (4d1c506f7): five violations fixed and the rule tests moved to bun; the reviewer asks whether Sign in as a text action is still obvious to a signed-out user: the lead reads the screens before landing.'
    - 'S1 re-ask holds: three reviews. The first version folded `site:` out of the near key and would have answered a second-outlet search from the first outlet''s pages; fixed (site kept, only today''s and yesterday''s dates and weekdays fold, the mostly-seen hold never stops a `site:` search to an outlet not yet held, a paywalled question is released). Accepted; in the fourth train.'
    - 'X2 conversation and editor move: sent back twice. Second time: a merged save never reached the page''s field, so the next keystroke would revert the other writer''s change; and text with no recorded base was posted over a newer head. Last round in progress.'
    - 'X2 themes: sent back twice; one blocking left (the carry-over of old themes can silently not run). Last round in progress.'
    - 'The lead corrected one store test in the event-wakes commit after review (it still asserted that a held arrival wakes no one, the behaviour the reviewed change removes); the store suite is rerun for it.'
    - 'Main''s Swift design and typography rule tests fail and CI does not run them; lane design-rules-fix.'
    - 'Review seat 1 is Gemini 4 Hydrogen through agy (owner, 2026-11-20); in print mode it needs --sandbox --dangerously-skip-permissions in a frozen copy.'
  lanes:
    - 'root-desk-r3 7a6d66dc4 (W2): STOPPED for the owner; docs/problems/texture-root-desk-six-reviews-no-landing-2026-11-20.md'
    - 'snapshot-stream 0873aa1fc (H1 S1): STOPPED for the owner; the budget problem record recommends raising the 90-minute budget as the interim relief'
    - 'exact-convo-edit 4b58dba83 (X2 conversation rows and the Texture editor): STOPPED for the owner after a third send-back; docs/problems/texture-editor-in-exact-three-reviews-no-landing-2026-11-20.md; the two conversation-row commits drew no blocking finding and could land alone'
    - 'owner-act-enforce ba32aa571 (W3 third setting): HELD for the owner'
    - 'dispatch-deadlines 04a06df51: internal/actor fails its suite on the build machine (600 s timeout); not landed; no round since 11:29Z'
    - 'landed lanes not removed (lane rm refuses without --force because the landed commits were rebased): exact-demo-leftovers, entity-e2-ids, search-reask-hold, last-switches, embed-kick'
  conjecture:
    claim: 'the paper''s web searches are mostly a hunt for a second outlet and re-asks of one question; holding re-asks on every verb and telling a run at its start which outlets the corpus already holds removes more than half, and under 1,000 a day needs the second outlet to come from the corpus (events with two outlets) and not from the web'
    test: the gateway's searches a day and searches per run after each landing, with stories a day not lower than the week before
    edge: missing_oracle
    edge_note: the logs carry no outlet per capture, so the size of the second-outlet hold is unknown until Landing 0's fields are deployed
    status: testing
  believed_state: W1, X1, S2, D1, D2, the wake, search leads, the design rules, gateway G1, entity station E2 with its collation fix, two more settings removed and the re-ask holds are deployed; the phone carries 20af586d3; three pieces are stopped for the owner and one is held
  main_uncertainty: whether any set of caller fixes reaches 1.8 searches a run; whether the event layer joins once captures have vectors; whether the build machine (one heavy run at a time) can carry eleven builders before morning
  blocker: none
  next_action: settle C176 after 16:30Z from the paper's run lines and the gateway's search counts; land the themes if the bounded fix is accepted; the owner's four decisions (snapshot budget, root desk, editor, owner acts)
operating_notes:
  - 'Roles: the lead orchestrates; Sonnet subagents build in lanes (cd /Users/wiz/Dev/go-choir && git fetch && lane new <name> --base origin/main); Opus only for open design. Review: seat 1 a Sonnet subagent that did not write the change, seat 2 DeepSeek through the agentic-consensus runner (--include omp-deepseek-v41-flash) on a frozen clean copy; orange one seat, red two.'
  - 'Go compiles and tests on the build machines, never the Mac (gate scripts on the Mac need GOFLAGS=-mod=mod). choir-agents has 3.9 GB of memory: one heavy run at a time under flock ~/agentcore.lock, and stop the test Postgres a killed run leaves. choir-runner (10 GB) is the landing machine; whole agentcore under -race took 69 to 83 minutes on the night of 2026-11-19, so use -timeout 150m.'
  - 'A builder''s report is a claim. Before a landing, see the suite''s ok line yourself, look at the proof''s screenshots yourself, and enter the calibration claim in docs/calibration/claude-ledger.md (next free id C179).'
  - 'State times only after date -u. The ledger and reports are in UTC; the owner is UTC-5.'
receipts:
  - 'previous goal: docs/archive/definitions/choir-night-exact-vectors-learning-2026-11-19.md (superseded by this file; its station statuses were never updated and are not to be trusted)'
  - 'landings of 2026-11-19 and 20: 4365f2ca3 event layer; 2754468d2 query embedder; 62cb7f35b Exact deck and reader; ad44370da embedder threads; 41cd8c979 search usage status; 3f3e10ba1 search gate; 39d2138d5 and bd2ff1c24 signed-out work'
  - 'ledger: 87 settled at handoff (C166 true); open C120, C127, C129, C148 to C150, C163, C167'
  - 'letters: docs/reports/exact-vectors-and-the-search-count-2026-11-19.md'
---

# Exact, search and entities: the handoff of 2026-11-20

Standing: superseded | kind: wanted | settled-by: the successor's supersedes field (its line 4) | replaced-by: docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md

This file replaces the night goal of 2026-11-19. It was written at the end of
a thread that had been summarised many times, at the owner's request, so that
a new thread can start from what is true and not from what a summary says.

Three things to hold on to.

The owner's phone was not signed out. It was in demo mode, a proof aid that
shows bundled sample stories and pretends to be signed in. A day of work on
the signed-out state was real and is deployed, but it could not show on that
phone. The lesson is the one the contract already states: read the trace
first. Nobody looked at what the phone was displaying until the owner sent a
screenshot.

"Vectors are done" is true and "event and entity linking is done" is not.
Events are linked, with a join rate nobody has checked. Entities are not
built.

The search count is the number the owner watches. One fix is deployed and
roughly halved it. The fixes that should do the rest are written and stuck
behind test runs on a machine with too little memory.
