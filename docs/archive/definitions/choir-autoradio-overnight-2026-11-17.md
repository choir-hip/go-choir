---
definition_version: 7

readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none; the owner's words of 2026-11-17 (about 02:00Z to 03:30Z) are the authority to start ("let's make a goal file for our overnight work. We're doing a bunch of different things"). Each server candidate is reviewed at its tier before landing; phone and Mac builds are judged by the owner on the devices.

start:
  captured_at: 2026-11-17T03:35Z
  source:
    canonical_ref: origin/main c3a54a69d
    deploy_identity: staging deployed_commit 6ba3e1080 (2026-11-17T01:24Z)
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: user_wip
      owner: owner
      touch: forbidden
      recovery: leave in place (one untracked env backup that holds secrets)
    - path: /Users/wiz/Dev/go-choir/.claude/worktrees/agent-a6cf48f517bc78dc1
      status: dirty
      class: generated_temp
      owner: lead
      touch: goal_owned
      recovery: the QA build tree, detached at lane/autoradio-integ 102e874ee; two generated exact.json files are rewritten by every build and checked out again before a switch
    - path: lanes autoradio-integ (102e874ee), mac-app (867c05d4a), programme-opening (c6aeec9e4), phone-retrieval and voice-fullband (not yet pushed)
      status: unknown
      class: other_agent_wip
      owner: builders
      touch: read_only
      recovery: each lane is its own branch on origin
  known:
    - The phone runs lane/autoradio-integ 102e874ee, installed 2026-11-17 about 03:10Z. The owner confirmed voice works after the playback-session fix; the one-voice work after it is unheard.
    - The server run sheet is deployed and proven (ledger C114). The opening-first programme is reviewed and not landed.
    - Exact is pinned at 76cd50f9 with one local patch.

finish:
  deliver: In the morning the owner opens Autoradio on the phone and on the Mac, lands on a page that is the radio, hears it start talking at once, hears one voice at a time and more podcast clips, asks about a subject and gets an answer drawn from what the device holds, and is told plainly which embedding route works on which machine.
  artifact: A phone build and a Mac build from one branch on the latest Exact, each opening on the Autoradio page, with local retrieval in the on-device model's context; a measured verdict on EmbeddingGemma 2 (GGUF and Core AI) on the phone and the Mac in docs/reference/EMBEDDINGGEMMA.md.
  acceptance:
    - action: the owner's one QA pass on the phone (launch, play, mic on, talk over the radio, ask "tell me about the Senate", open a PDF, edit a Markdown file)
      proves: the phone build works as a whole, by the owner's ear and eye
      evidence_class: human inspection
    - action: the lead launches the Mac app, signs in, plays a programme aloud and runs one local-model turn; mobile/scripts/proof-mac.sh passes
      proves: the same app runs natively on macOS with its local models
      evidence_class: local run plus human inspection
    - action: a simulator tour photographs each state of the Autoradio page and asserts its test ids
      proves: the app opens on the radio page and the sheet view is gone
      evidence_class: local test
    - action: retrieval tests and a device turn whose context.json lists retrieved ids for a subject not in the queue
      proves: the on-device model's context comes from a local search, not only the queue
      evidence_class: local test plus device file
    - action: timing and recall figures for BM25 alone, vectors alone and both fused, on the Mac and on the phone, written into the reference page
      proves: which embedding route works where, and whether fusion beats BM25
      evidence_class: measurement
    - action: the launch console line on the phone (`AUTORADIO launch ... first_audio_ms= source=cached`) after a cold start with a fresh cached opening
      proves: the radio's voice starts from an opening written and synthesised before the launch
      evidence_class: device console
    - action: the worker's match run (dry run first, then written through the verified route) and the count of stories with a clip in the front window on staging, before and after
      proves: clips are matched without a hand and without a model call, at the precision the tests state
      evidence_class: local test plus deployed count
    - action: scripts/prove-autoradio-programme on the drill computer after the opening-first landing
      proves: an open is answered with one opening and its first speech time
      evidence_class: deployed proof
  rollback: the phone keeps the last build the owner accepted until a newer one passes its checks; every lane is a branch and nothing merges into lane/autoradio-integ unbuilt; server changes revert by commit.
  landing:
    required: true
    environment: staging, for the opening-first programme only; phone and Mac work is not deployed
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: fewer steps and fewer surprises between opening the app and hearing a programme that answers to the listener, on either machine, with every piece checked by running it (sound produced, a window opened, a search returned) rather than by reading it.
  goodharting_would_be: test counts and screenshots that pass while the device is silent or two voices overlap; a Mac window that opens in demo mode without models and is called the Mac app; retrieval that returns ids the model never uses; an embedding "works" claim from one load with no timing on the owner's phone.

homotopy:
  realism_axis: from Swift package tests with virtual audio, through the simulator and the Mac build with real engines, to the owner's phone in the owner's hand.

boundaries:
  mutation_class: orange for phone and Mac code; red for the opening-first programme (agentcore) which lands with its existing two-seat acceptance
  authority_sources: [the owner's messages of 2026-11-16 and 2026-11-17, CLAUDE.md, docs/choir-doctrine.md]
  must_preserve:
    - one Mac build at a time through the lock; every simulator slim
    - exactly one voice audible; the listener outranks the radio
    - one way to do each thing in the app; no switches or settings that gate behaviour
    - no model weights, secrets or addresses in the repo
    - the owner's installed build is replaced only by one that built from a pushed commit
    - no server-side embeddings in this mission
  excluded: [Mac-specific design, a server vector store, the master stream and its platform computer, spreadsheets, any new Jev caller]
  protected_surfaces: [internal/agentcore programme route (opening-first), gateway and provider calls]

stations:
  - id: S1
    name: One voice at a time
    scope: single-voice gate over every sound path; an answer keeps the story's place; full-band listening so the radio's tone does not change while voice is on (side lane until a device trial)
    class: orange
    review: one seat
    depends_on: []
    carrier: lane/autoradio-integ 102e874ee; lane/voice-fullband
    status: working
  - id: S2
    name: Local retrieval for the on-device model
    scope: BM25 over everything the phone holds, recent first among relevant hits, into the turn context with ids
    class: orange
    review: one seat
    depends_on: []
    carrier: lane/phone-retrieval 5a7813642 (first version 6e7551507 is on lane/autoradio-integ and installed)
    status: working
  - id: S3
    name: Latest Exact on both platforms
    scope: move the pin from 76cd50f9 to upstream head, re-fit or delete the editor patch, adapt to breaking changes, update the reference page; phone simulator build then Mac build before it reaches the phone branch
    class: orange
    review: one seat
    depends_on: []
    carrier: lane/mac-app
    status: working
  - id: S4
    name: The Autoradio Mac app
    scope: the same app as a native macOS app with the local models; platform guards, manifest, model libraries with macOS slices, audio without a session, proof script
    class: orange
    review: one seat
    depends_on: [S3]
    carrier: lane/mac-app
    status: working
  - id: S5
    name: EmbeddingGemma 2 on device
    scope: try the GGUF through the llama runtime (to support iOS 26 and the A16 phone) and the Core AI port (Mac, and the phone if it loads); embed what the device holds; flat-file cosine search; fuse with BM25; measure load, speed, memory and retrieval quality on both machines
    class: orange
    review: one seat
    depends_on: [S2]
    carrier: lane/embed-trial
    status: working
  - id: S6
    name: The Autoradio page
    scope: the app opens on a page that is the run sheet made readable (docs/design/autoradio-page-2026-11-17.md); the sheet behind the bar is deleted
    class: orange
    review: one seat
    depends_on: []
    carrier: lane/radio-page
    status: working
  - id: S7
    name: Opening-first programme
    scope: land lane/programme-opening (accepted in review round 3); the installed phone build already understands it. It stopped merging when the run sheet landed, so it is being rebased; a rebase that is more than mechanical is re-read by one seat. Deployed timing proof on the drill computer.
    class: red
    review: two seats (done); one seat on the rebase delta if it is not mechanical
    depends_on: []
    carrier: lane/programme-opening c6aeec9e4, rebasing to lane/programme-opening-rebased
    status: working
  - id: S8
    name: More podcast clips
    scope: automatic matching of stories to transcript passages inside the podcast worker with no model call (entity gate, score floor and margin, publication window, today's clip shape), precision at least 0.9 on a labelled set; long interviews split rather than dropped; the failed uploads explained. Measured opportunity 2026-11-17, about 30 to 40 stories in 100 against 6 in 100 by hand. Later slices, not tonight, more shows, a worker host that stays up, cold opens.
    class: orange
    review: one seat
    depends_on: []
    carrier: lane/clips-auto-match; plan in the lead's scratch (more-clips-plan.md)
    status: working
  - id: S9
    name: Talking at once
    scope: the phone keeps the next opening written ahead and synthesised to an audio file (asked after an opening airs, on going to the background, and from background refresh if Exact exposes it); a cold launch plays it through the one-voice gate within about a second; a stale opening is not played as new; one launch console line with the timing breakdown
    class: orange
    review: one seat
    depends_on: []
    carrier: lane/instant-start
    status: working

decisions_for_the_owner:
  - id: D1
    question: Which minimum iOS does the app support?
    why: it decides the embedding runtime and which Exact and Apple features are allowed
    options: iOS 27 only; iOS 26 and later
    default: iOS 26 and later (the owner's stated wish), so the GGUF route is tried first on the phone
  - id: D2
    question: If full-band listening makes the radio pause itself now and then, which is worse, that or the tone change while voice is on?
    why: it decides which audio mode ships
    options: keep voice-chat mode (tone changes, reliable pause); full-band (true tone, some risk of false pauses)
    default: keep the installed behaviour until the owner has tried the side build
  - id: D3
    question: May the station learn from anonymous listening in aggregate?
    why: the master stream's forecasts cannot be scored without it
    options: yes; no
    default: no; master forecasts stay unscored
  - id: D4
    question: May the Jev job entry point (jevJob) move to a shared package, with that one file admitted to the list scripts/check-jev-entry protects, so a service outside agentcore can ask Jev through the approved path?
    why: a judged clip match (or any Jev caller in corpusd) cannot obey the entry rule today, because corpusd cannot import agentcore
    options: yes; no, keep Jev callers inside agentcore
    default: no change; clip matching uses no model, so the question waits
  - id: D5
    question: Where does the podcast worker live?
    why: it runs on the owner's Mac, which sleeps, so new episodes are transcribed late
    options: stay on the Mac; a host that stays up
    default: stay on the Mac for now (the owner, 2026-11-17)
  - id: D6
    question: Clip length and how many per story; search spend for finding more shows
    why: rights exposure and cost
    options: keep or change
    default: 20 to 75 seconds, at most 3 a story, as the code has it; at most 1.50 of the 3 dollars of search budget, not spent tonight
owner_rulings:
  - on: 2026-11-17
    decision: S8, D5
    said: "I heard the first podcast clip, and I love it. So I want to add another thing to the list, which is how do we get more podcast clips? ... we can use embedding and BM25 on my Mac. Well, it's kind of a kludge, the thing we're doing on the Mac. I guess we can keep doing that for now."
  - on: 2026-11-17
    decision: S9
    said: "it takes about 5 to 10 seconds to start talking after you open the app. It'd be cool if it had already started talking, so that would mean it would have to already be scripted in advance ... We can save the master stream for later."
    read_as: solved on the phone with the existing per-owner programme route; no platform computer and no master stream in this mission
  - on: 2026-11-17
    decision: D1
    said: "it seems like my iPhone is too old for that Core AI version, and I don't really want to just support iOS 27. I'd love to support iOS 26 as well. So maybe we should target the GGUF version. Let's play around with it and see which one or both or neither ends up working"
  - on: 2026-11-17
    decision: S3
    said: "for overnight build we should also update to latest exact. then build mac off that; and later: i want new exact on phone too"
  - on: 2026-11-17
    decision: S6
    said: "we should definitely have a page for auto radio itself. That should be what it loads into... I let you, Fable, design that."

now:
  status: working
  slice: "the owner found the first candidate failing on the phone (podcasts broke the audio session, no restart after background, wrong context for the desk); the repaired candidate fbf7e84f8 is installed and not yet checked on the device; three server landings are deployed"
  source_ref: "origin/main at the commit that carries this edit; phone and Mac candidate origin/lane/autoradio-integ4 bfa5674f0"
  deploy_identity: "staging 0316d7cfc when last read (24cd67412 is worker-only code and docs)"
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: K1
    calibration_claim: C115
    claim: By the owner's morning check, one branch on the latest Exact builds both a phone app and a Mac app that open on the Autoradio page and speak, and the phone build carries local retrieval.
    test: the build logs of both targets from one pushed commit, the Mac proof script, and the owner's verdict on the phone
    edge: missing_oracle
    delta_o: a device check that needs no person, the console line per on-air entry with its sample count, read by the lead after each install when the phone is unlocked
    scope_if_supported: the two builds named, on the owner's two machines
    status: active
    evidence_refs: []
  decision:
    what: Mac first for embeddings and for the new Exact pin, the phone after each passes; the phone keeps its last accepted build meanwhile
    kind: operational
    status: settled
    evidence_ref: this file
    owner_ratification_ref: not_applicable
  belief:
    believed_state: "lane/autoradio-integ4 bfa5674f0 on Exact de212e671 is installed on the phone, unlaunched and unheard; the same code path spoke on the Mac (proof at e8f4454a4: three entries on air, 22 models registered) and passed four simulator tours of the Autoradio page; the opening-first programme and the clip matcher are deployed and proven; EmbeddingGemma 2 loads on the Mac by both routes"
    main_uncertainty: "whether the installed build makes sound on the device, and whether EmbeddingGemma 2 loads on the A16 phone by the GGUF route"
    next_observation: "the owner launches the phone build; then the lead launches it with -EmbedTrial 1 and reads Documents/proof/embed-trial.json"
  blocker_or_risk: "the installed build is unheard; a launch speaks at once, so the lead did not launch it at night; rollback is lane/autoradio-integ 5a7813642"
  next_action: "device check of fbf7e84f8 with the console attached (play, background, return; ledger C118), which needs the phone unlocked; then review and land the mobile lanes; the phone embedding model download and first vector query; the full-band listening trial (D2); the day-state page tour"

receipts:
  - "S3 the latest Exact on both platforms: pin de212e671 (mobile/exact.pin); simulator, device and Mac builds exit 0 at lane/mac-app e685d4a45 and later; builds refuse a wrong Exact (dbb466e69)"
  - "S4 native Mac app: proof-mac.sh MAC PROOF OK at e8f4454a4 (opening, two further entries on air with nonzero samples, 22 models registered); sound not judged by ear; the Mac build of integ4 itself has not been run"
  - "S2 retrieval and the local content store: lane/phone-retrieval 21a882649 in integ4; store renders in the simulator; sync and sign-out wipe not exercised on a device"
  - "S6 the Autoradio page: lane/radio-page bfa5674f0; tours cold, answer and empty pass in the simulator, day tour fails at a later step, listening is not photographed"
  - "S9 talking at once: lane/instant-start 2e773295a in integ4; launch timings unmeasured on a device"
  - "S1 one voice at a time is in the installed build; full-band listening is lane/voice-fullband cb6def5c9, not merged (D2)"
  - "S7 opening first: landed 0316d7cfc, CI 44019332807, scripts/prove-autoradio-programme PASS on d@e (open median 5.17 s); ledger C116 true"
  - "S8 clips: landed 24cd67412, CI 44019560901; staging went from 3 clips for 1 story to 30 for 17 after one match run; ledger C117 true; no clip judged by ear"
  - "S5 EmbeddingGemma 2: lane/embed-trial 1e24b6f7a; both routes load on the Mac; the phone rows of the verdict table are unknown; docs/reference/EMBEDDINGGEMMA.md"
---

# Autoradio overnight, 17 November

Standing: superseded | kind: wanted | settled-by: docs/ACTIVE.md line 206 heads it Superseded (2026-11-17 night); its own now.status is working (line 219) | replaced-by: docs/definitions/choir-autoradio-exact-cutover-2026-11-18.md

The owner asked for seven things in one evening and then for this file to hold
them together. They are the stations above. Three points are worth prose.

**Nothing is working until it has run.** On the 16th a phone build passed
nine hundred tests and a simulator tour and was silent on the device, because
the virtual audio engine returned before the real start. The acceptance lines
therefore name sound, windows and files, not test counts.

**The order protects the phone.** The new Exact pin, the Mac work, retrieval
and the page each live in a lane. A lane reaches `lane/autoradio-integ` only
after the lead has built it, and the phone gets a build only from there.

**The embedding question is an experiment, not a build.** Two routes exist
for one model. The owner's phone has an A16 chip and should run iOS 26 as
well as 27, so the GGUF through the runtime the app already carries is tried
first there; the Core AI port is tried on the Mac. The result that matters is
a table: route, machine, load time, speed, memory, and whether fused ranking
beats BM25 on questions the owner would ask.
