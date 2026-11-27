---
definition_version: 7

readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none; the owner's statement of 2026-11-15T23:36Z is the authority to start ("lets shoot to get all this in the 6am build"). Each server candidate is reviewed by its tier before it lands.

start:
  captured_at: 2026-11-15T23:50Z
  source:
    canonical_ref: origin/main 24e17f77a
    deploy_identity: not read at capture
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: user_wip
      owner: owner
      touch: forbidden
      recovery: leave in place (one untracked env backup that holds secrets)
    - path: /Users/wiz/Dev/go-choir/.claude/worktrees/agent-a6cf48f517bc78dc1
      status: clean
      class: candidate
      owner: lead
      touch: lead only
      recovery: branch origin/lane/autoradio-qa (1ec3b7b1c), the app on the owner's phone since 23:34Z
  observed:
    - The app on the phone (lane/autoradio-qa, not on main) has the paper with views, the article stack, the embedded source page, the topic tree and edit page, the hands-free switch, Kokoro and Phonon voices, the speed slider, next and previous, autoplay, and a DJ stretch on Play with nothing chosen. None of the last five builds has been heard or seen by the lead; the owner has seen through the 23:23Z build.
    - Podcasts on the server are show search, subscriptions and feed refresh. No episode records, audio fetch, transcripts or clips. The Nov 10 research, Exa runs, hit rate (8 of 25 with six episodes) and transcription benchmark (Parakeet 110M, about 25x real time on the build VM) are recorded in docs/problems/podcast-clipping-work-was-never-written-down-2026-11-15.md.
    - The phone has no podcast screen, no library, no queue of mixed items, no remote audio streaming, no on-device first answer, no speaker check (the hook is off, no model loaded).
    - Asking by the bar or by voice "didn't seem to work right" (owner); not traced.
    - The developer profile on the phone expires 2026-11-17T21:49Z.
    - Unknown. Whether the clip pipeline can be reviewed, landed and deployed by 07:00Z. Whether remote audio, PDF hosting and the speaker check work on a device: nothing here has run on one.

finish:
  deliver: >
    At 6am local on 2026-11-16 (11:00Z) the owner's phone holds a build in
    which Autoradio feels like the whole product: a DJ mixes the paper's
    stories with podcast clips, a clip keeps playing into its episode, there
    is a podcast app and a library for EPUB and PDF that reads aloud, a URL
    typed in the bar opens and can be read aloud, one queue plays any of
    them while the reader goes elsewhere, the reader can ask and be answered
    quickly, and the pages look designed.
  artifact: >
    The installed build of origin/lane/autoradio-qa at its final tip, and on
    main and deployed: the podcast clips design
    (docs/design/podcast-clips-2026-11-15.md), episode intake, the public
    clips route, the podcast_clip programme segment, and the worker
    (cmd/podcast-worker) run by the lead on the build VM.
  acceptance:
    - action: the lead's device build of the final tip
      proves: it compiles, links and installs; the install time is read from the build log
      evidence_class: build log with exit code. It does not prove any feature works.
    - action: the lead's simulator run of the final tip with screenshots of every destination (paper views, article, player with Up next, Podcasts home and show and episode, Library and a document, edit edition, settings)
      proves: each page renders without an error state and matches the design pass
      evidence_class: screenshots read by the lead; weak for audio
    - action: GET /api/platform/podcast-clips on staging after the deploy
      proves: at least one clip for a story on today's front, with an audio URL that answers a byte-range request at the clip's start
      evidence_class: deployed proof, the response and the range request's status
    - action: the owner's use of the build on 2026-11-16
      proves: the feel; every claim about audio, voice and gesture
      evidence_class: owner's verdict. This is the only evidence for most of the deliverable.
  rollback: reinstall the 23:34Z build (lane/autoradio-qa at 1ec3b7b1c); revert each server commit.
  landing: server commits go through scripts/land, CI, the staging deploy and the deployed proof. The phone app stays on its branch tonight; its one-seat review before main is a successor station.
  non_goals:
    - TestFlight, Android, YouTube, offline downloads, OCR, echo cancellation for remote audio, a managed transcription service on a host, syncing phone subscriptions to the computer.
    - Landing the phone app on main.

value:
  better_means: fewer places where the owner, using the phone for a day, meets a dead end, an error, silence, or a screen that looks unmade, while every feature shown is real.
  goodharting_would_be: a build that lists every feature and plays canned audio, fake clips or stub pages; or pages that exist and fail on first touch because nothing ran on a device.

homotopy:
  realism_axis: from "each kind of item plays through one queue from real sources, rough" toward "the DJ writes one programme over all of them and learns from listening". A clip found by an operator-run worker and served by the real route is on the axis; a clip file bundled in the app is not.

boundaries:
  authority: owner's statements of 2026-11-15 (all recorded in docs/problems/autoradio-owner-phone-review-2026-11-15.md); standing install authorization for QA builds.
  mutation_class: "phone lanes orange (not on main). Server: design green; intake, clips route and programme segment orange; matching with a gateway host-caller entry red, two seats."
  invariants:
    - One Mac build at a time, run by the lead. Builders run package tests only, through the lock.
    - No switches, no limits on our own model calls; one Jev call per matching run.
    - Audio streams from the publisher's URL; no third-party audio, transcript or book text in the repo.
    - Nothing acts on the paper's or the owner's computers. No host config change on Node A or Node B.
    - The Exa key is the owner's, for podcast search only; no more than $3 without the lead's go.
  exclusions: the storage goal's lanes continue separately.

conjecture:
  bridge: if the build holds every piece working roughly, the owner can judge the whole product's feel in one day, and the next round's priorities come from use, not guesses.
  claims:
    - claim: clips exist often enough to mix
      test: the operator proof on today's paper (stories with a clip, of the front's stories)
      edge: a clip that matches by words and is not about the story (seen in October)
      delta_o: the support check in the one Jev call; the owner's ear
      scope: daily news feeds, today's paper
    - claim: seven lanes written without a build merge into one app that compiles
      test: the lead's builds through the night
      edge: it compiles and a page fails at run time
      delta_o: the simulator screenshot pass
      scope: tonight's lanes

stations:
  - id: Q
    name: "the play queue, remote audio, Up next, a URL in the bar"
    class: "orange"
    review: "by tier before landing on main; the phone branch is not landed tonight"
    depends_on: []
    carrier: "lane/autoradio-queue"
    status: working
  - id: P
    name: "the podcast app"
    class: "orange"
    review: "by tier before landing on main; the phone branch is not landed tonight"
    depends_on: []
    carrier: "lane/autoradio-podcasts"
    status: working
  - id: R
    name: "the Reader app, EPUB and PDF"
    class: "orange"
    review: "by tier before landing on main; the phone branch is not landed tonight"
    depends_on: []
    carrier: "lane/autoradio-reader"
    status: working
  - id: T
    name: "asking fixed, the on-device first answer, voice on and off, the reader's voice"
    class: "orange"
    review: "by tier before landing on main; the phone branch is not landed tonight"
    depends_on: []
    carrier: "lane/autoradio-talk"
    status: working
  - id: D
    name: "clips in the programme, Play hands the list to the DJ"
    class: "orange"
    review: "by tier before landing on main; the phone branch is not landed tonight"
    depends_on: []
    carrier: "lane/autoradio-dj-clips"
    status: working
  - id: V
    name: "the design pass, with screenshot rounds from the lead"
    class: "green"
    review: "by tier before landing on main; the phone branch is not landed tonight"
    depends_on: []
    carrier: "lane/autoradio-design"
    status: working
  - id: S
    name: "podcast clips on the server and the transcription worker"
    class: "orange, and red for the matching job"
    review: "by tier before landing on main; the phone branch is not landed tonight"
    depends_on: []
    carrier: "lane/podcast-clips"
    status: working
  - id: K
    name: "the desk sees the queue and returns ordered changes"
    class: "orange"
    review: "by tier before landing on main; the phone branch is not landed tonight"
    depends_on: []
    carrier: "lane/autoradio-queue-desk"
    status: working
  - id: M
    name: "merges, builds, installs and the report"
    class: "green"
    review: "by tier before landing on main; the phone branch is not landed tonight"
    depends_on: []
    carrier: "the lead"
    status: working

now:
  status: working
  slice: "K deployed and proven; S deployed with clips served, matched by hand; the phone lanes merged, built and installed through design sweep seven"
  believed_state: "the phone build of 07:57Z holds every lane; staging serves the queue desk (a99e07d3b) and podcast clips (53747dcca), 13 clips over 5 front stories; the worker runs on the lead's Mac, not as a durable service"
  main_uncertainty: "nothing has been heard on the device: audio, clips, voice, the DJ choosing a clip while signed in"
  next_observation: "the owner's first listen on the phone"
  next_action: "a second matching pass near 09:30Z, the morning report, then the owner's decisions: the Jev entry point for automatic matching, where the worker and its key live"
  blocker: none

receipts: []
---

# Autoradio, the full experience, overnight 2026-11-15

Standing: built | kind: happened | scope: in part | settled-by: K landed a99e07d3b and S landed 53747dcca (podcast clips) on origin/main; ledger 08:40Z final install | note: now.status is working (line 170); ACTIVE.md:215 heads it Superseded by the 2026-11-17 goal; automatic matching and the durable worker were not built

The owner's words, the lanes and the rules each builder works under are in
the common brief the lead gave them; the rulings are in
`docs/problems/autoradio-owner-phone-review-2026-11-15.md`. The ledger is
`choir-autoradio-full-experience-overnight-2026-11-15.ledger.md`.

If the 09:00Z merge window closes with a lane unmerged, it is left out and
named in the morning report. If a merged lane breaks the build and is not
repaired in two tries, it is backed out. The build installed at 6am is the
last one that compiled and installed, not the most ambitious one.
