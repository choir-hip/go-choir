---
definition_version: 7
readiness: executable
authority: 'the owner, 2026-11-22: "we are still working on exact train arent we. we should of course continue that"; "whats the right goalstring for running both missions at once? maybe make a metamission which covers them both, and any 3rd or nth mission we add. so we can keep /goal running continuously".'
review:
  reviewer: none (the owner's statement is the authority; each station file carries its own)
  frozen_ref: none
  verdict: none
metamission:
  stations:
    - id: M1
      path: docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md
      readiness: executable
      status: working
      depends_on: []
      note: 'the phone: every page outside voice in shared Exact code, and the account boundary; and the search changes in flight (graph-first). Its ontology track (events and entities as separate kinds) is replaced by M2.'
    - id: M2
      path: docs/definitions/choir-first-design-2026-11-22.md
      readiness: executable
      status: working
      depends_on: []
      note: 'the first design: subjects and mentions, the hot path for captures, predictions that subscribe, the root Texture, the readers. Stations F0 to F6. No red station starts before the design panel has read the draft.'
    - id: M3
      path: docs/definitions/choir-storage-efficiency-2026-11-22.md
      readiness: executable
      status: working
      depends_on: []
      note: 'storage: no routine operation writes a second copy of a store, restore fits its budget, the store is measured by kind and cut. Stations S0 to S7. Added on the owner''s word after Node B''s disk refused deploys and wakes on 2026-11-22. Runs in the background before and during the other tracks (owner, 2026-11-23).'
    - id: M4
      path: docs/definitions/choir-autoradio-on-android-2026-11-22.md
      readiness: executable
      status: working
      depends_on: []
      note: 'Android: Autoradio running on an old Motorola from the shared codebase, and a measured table of which on-device models such a phone can run. Stations A0 to A5. Added on the owner''s word, 2026-11-22. Deferred on the owner''s word, 2026-11-23: no station starts until they say.'
    - id: M5
      path: docs/definitions/choir-records-and-measurement-2026-11-23.md
      readiness: intent
      status: working
      depends_on: []
      note: 'track T3, first: durable decision and turn records, the measure of the paper against the world with its cost, and the replay experiments (E1 to E5) that decide the engines design. Intent; the first three experiments read only captures and may start on the owners word.'
    - id: M6
      path: docs/definitions/choir-voice-pipeline-2026-11-23.md
      readiness: intent
      status: pending
      depends_on: []
      note: 'track T1: the voice pipeline as the owner specified it, on the phone: kept choices, model speech detection, the named recogniser, model end of turn, the same sound with voice on, the handover to the desk, spoken onboarding. Intent; written up after the design discussion.'
    - id: M7
      path: docs/definitions/choir-shipping-the-app-2026-11-23.md
      readiness: drafted
      status: working
      depends_on: []
      note: 'FIRST in order on the owner''s word (2026-11-23: "metamission wise, we can prioritize this, as we will have a couple days to wait on apples review"). Track T5: TestFlight and the App Store; the radio signed out, an account made and deleted in the app, consent and permissions, screenshots and previews, reviewer notes, a release an agent runs. Intent; the owners account steps come first.'
    - id: M8
      path: docs/definitions/choir-operations-podcasts-and-providers-2026-11-23.md
      readiness: intent
      status: pending
      depends_on: []
      note: 'part of track T4: podcast transcription on a durable host and model calls routed by each providers reported headroom. Intent; built after the plan is defined (owner, 2026-11-23).'
    - id: M9
      path: docs/definitions/choir-autoradio-in-the-desktop-2026-11-23.md
      readiness: intent
      status: pending
      depends_on: []
      note: 'Autoradio as an app in the choir.news desktop, listening only with a local model. Intent; deferred until after M4 (owner, 2026-11-23).'
start:
  captured_at: 2026-11-22T05:02Z
  source:
    canonical_ref: origin/main 9f10b7477 (docs); the last code commit is 92f068980
    deploy_identity: staging and the paper on 92f068980, read from the CI run's deploy job; the owner's iPhone on the build of 2026-11-20 at bfc1492c6
  what_is_true_now:
    - 'Two missions are live and each file holds its own state. This file holds only what they share: the order of landings, the shared resources and the list of missions.'
    - 'One landing and one deploy run at a time and a landing takes about two hours; one Mac native build runs at a time through the lock; Go builds on the build machines, several at once.'
  branches_in_flight:
    - 'see each station file; none belong to this file'
finish:
  deliver: 'Every mission listed here is complete or superseded, and the list is empty of working missions.'
  artifact: 'the station files, each settled by its own acceptance list'
  acceptance:
    - action: read each station file's now.status
      proves: every listed mission is complete or superseded with its own named evidence
      evidence_class: goal file state
  rollback: none of its own; each mission names its own
  landing:
    required: false
    environment: staging
    required_receipts: []
value:
  better_means: 'the landing slot is never idle while a reviewed candidate waits, and no mission stalls because another holds the session''s attention'
  goodharting_would_be: 'landings batched across missions to save a deploy; a mission marked working here that nobody has touched in a day; this file growing station detail'
homotopy:
  realism_axis: 'nine missions; another is one more station entry and its own file'
boundaries:
  mutation_class: green for this file; each mission states its own
  authority_sources:
    - 'the owner''s words above; CLAUDE.md; each station file''s authority'
  owner_rulings:
    - '"autoradio is already a compelling product imo. now we are sprinting towards production grade, scalable"; "and above all, software quality. craft matters" (2026-11-22). Reading (the lead''s): when speed and quality pull apart, quality wins; a station is done when its replacement is proved and the old mechanism is deleted, not when the new one runs beside it.'
    - '"we are still working on exact train arent we. we should of course continue that" (2026-11-22). Reading: the phone cutover is not displaced by the redesign.'
    - '"priority: A tool that answers show me this run, step by step, with its reasons"; "yes, we need to improve our systemic learning"; "we need shorter more honest and coherent documentation"; "we need to land our work in flight" (2026-11-22, evening, after the lead listed what is broken). Reading: these four come before anything new. The run viewer is first. Nothing new starts until the lanes in flight are landed or dropped.'
    - '"what we have lost sight of is supervision. we havent been using the self-development feature which has built-in texture supervision... i have put to the side much of my beliefs on supervision to just let models go wild. and it hasnt yielded great results actually. so, we should get more serious" (2026-11-22). Reading: fewer agents at once, each supervised; work on the product moves back toward the product''s own supervised self-development path, and the lead says so when a change goes around it. How and when that path is used again is the owner''s to direct; the lead proposes.'
    - '"we should focus on our core data systems, our core backend infra, our key algorithms and agentic systems, and let the ux stay semi-constant. well, thats something we can do once the ux lands. the exact cutover has been essential for that... i dont want to keep it as merely an ios app" (2026-11-22). Reading: the Exact cutover lands (M1), then the experience holds still while the core is rebuilt (M2, M3). M4 (Android, the model benchmark) and adding sources are written down and wait; adding sources comes after the redesign ("adding the sources comes later. for now we need to redesign").'
    - '"I guess I see 2 tracks... The first one is to the voice system and pipeline. The second one is to the underlying engine, the root texture, the auto radio desk, and the source capture and corpus ingestion pipeline, and subsequent processing with research into publication and post-publication including personalization"; on records and measurement as a third: "Yes, that matches as a third track"; "for the fourth pile, we can really defer the Android stuff... the operations with podcast transcription as something that we want to do earlier, and synthetic as well. I kind of want to do those things after we get our plan defined. So then the building incorporates them. Same for the exact cutover, app faults"; "let''s do these wins in the background. Basically, we should be including all these tracks in the metamission so you can dispatch work as we are planning and defining other work. The storage stuff can be done before and while we do other stuff"; "let''s go slow here, and work through the design iteratively in the chat. So we can then update our docs" (2026-11-23). Reading (the lead''s): four tracks. T1 voice: what runs on the phone (speech detection, recognition, turn-taking, echo, the local answer, the audio session). T2 the engine: intake, the root Texture, the Autoradio desk, research, publication, personalisation (M2 is its file and is rewritten after the design discussion). T3 records and measurement: durable decision records, turn records, the measure of the paper against the world; it goes first because the other two are settled by it. T4 the app and platform outside voice: the Exact cutover and app faults (M1), podcast transcription on a durable host, routing by Synthetic''s reported headroom, storage (M3), Android (M4, deferred). T1, T2 and T3 get goal files after the design is agreed in conversation; until then only storage builds, and investigation is read-only.'
    - '"i want the metamission to contain all of our plans going forward. my idea is that the metamission will be what we stay on with /goal for the next multiple weeks" (2026-11-23). Reading: every plan is a station here with its own file, the deferred ones too, so nothing lives only in a conversation. Added that day as intentions: M5 records and measurement, M6 the voice pipeline, M7 shipping the app, M8 operations, M9 Autoradio in the desktop. M2 is rewritten after M5''s experiments.'
    - '"we should prioritize making all the apple requirements like in-app account creation and account deletion before we apply. gotta put your best foot forward"; "metamission wise, we can prioritize this, as we will have a couple days to wait on apples review. we will be busy working during that time" (2026-11-23). Reading: M7 goes first and takes the landing slot when it has a reviewed candidate; the design discussion and M5''s experiments continue beside it and fill the days Apple takes.'
  must_preserve:
    - 'one landing and one deploy at a time; one Mac native build at a time through the lock; every must_preserve of each station file'
  excluded:
    - 'station detail: a station''s now card is its own authority'
operating_notes:
  - 'Landing order: the first reviewed and proved candidate takes the slot. When two are ready: shipping (M7) first, then the Exact train, then graph-first, then the account boundary, then first-design stations in their order. Storage stations S1 and S2 are small and go ahead of a large candidate that is not ready. A small candidate goes ahead of a large one that is not ready.'
  - 'Resources: the Mac build lock serves M1 (train proof, account drills, phone install); the build machines serve all Go work; read-only reads of the paper are free. At most about five builders at once, because the lead reads every report.'
  - 'Overlap: graph-first lands before any M2 station that changes how research or Texture opens a run.'
  - 'Adding a mission: write its goal file, add one station entry here, name it in docs/ACTIVE.md and docs/mission-graph.yaml. Run with /goal docs/definitions/choir-standing-metamission-2026-11-22.md.'
  - 'Each hourly tick reads both station files'' now cards and rewrites this one''s.'
now:
  status: working
  as_of: 2026-11-23T22:50Z
  slice: 'M7 first: the release build (lane store-build), the design of in-app account creation and deletion, the privacy policy and store texts in draft; M3 S3 awaiting its builder''s report; the design discussion with the owner continues'
  source_ref: origin/main 7866f4027
  deploy_identity: staging and the paper on a2ba96767 (CI run 45248602790)
  candidate: none
  conjecture:
    claim: 'see M1 (ledger C183, C191), M2 (C190, C192, C194) and M3 (C195); this file makes no claim of its own'
    test: the station files' tests
    edge: independence
    edge_note: 'M1 and M2 both change research and Texture prompts, and M3 changes the paper''s restarts and captures, so a result in one may be caused by another''s landing'
    observer_upgrade: one landing at a time with its own deployed proof window before the next
    scope_if_supported: none of its own
    ledger_id: C190
    status: untested
  believed_state: 'landed and deployed 11-23: the index lock, device approval return, the phone train (on the owner''s phone as Autoradio 1, untested by the owner), the voice ask''s search with its vector fix (deployed proof passes), the broker number fix, the guest-reach gate. Reverted: the research-frame prompts (C201 false on its first hour). Found: the day''s top story ranked low because importance holds no state (docs/problems/the-days-top-story-ranked-low-2026-11-23.md). M2''s offline slice failed its kill lines (C192) and no panel has read draft 4.'
  main_uncertainty: 'the measure of the paper against the world and the order of the intake stages; the owner''s rulings on the design draft''s open decisions'
  next_observation: 'the S3 builder''s report; the storage measurement; the owner''s reading of the ideal-form draft'
  blocker: none
  next_action: 'work the design through with the owner in conversation; on their word start M5''s first three experiments; review S3 (two seats) when its builder reports; write M5, M6 and M7 up to executable as each is agreed'
receipts:
  - 'M1: docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md'
  - 'M2: docs/definitions/choir-first-design-2026-11-22.md'
  - 'M3: docs/definitions/choir-storage-efficiency-2026-11-22.md'
  - 'M4: docs/definitions/choir-autoradio-on-android-2026-11-22.md'
---

# The standing metamission

One file to run with `/goal`. It lists the live missions and what they share:
the landing order and the machines. Each mission's state is in its own file.
A new mission is one more entry.
