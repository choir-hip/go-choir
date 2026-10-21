# Launch Checklist

Date: 2026-10-21. Status: **living checklist, first draft for owner review**;
tick items as they land. Mutation class of this document: green (docs only).
Each item's own class is that of the work it names.

Owner direction (2026-10-21): an outer checklist around the
[security, density and architecture checklist](security-density-architecture-checklist-2026-10-21.md)
(called **SDC** below). It names every feature and requirement for launch,
roughly in the order we intend to do them: from the Apple developer account
to the Autoradio app, by way of the voice pipeline (the owner is building it
in a separate project and will bring it in), the security and density work,
and self-development. Do all the security and density work before launch,
and order work by what speeds up the next piece, not only by importance:
"we can blitz out all this work at agentic pace."

Then the owner's launch condition and notes (2026-10-21,
[owner notes §37](reports/owner-notes-2026-10-14.md)): launch is a
condition, not a date; the foundations matter most (security, density, the
self-development loop); and work lands in bigger batches.

## What launch means

The stack is **autoputer, then Autopaper, then Autoradio**. An Autopaper
problem is read as an autoputer problem first, and a rushed Autoradio will
show where Autopaper needs work; the owner wants that pressure, and wants
the fixes to land in the layer that caused them. The self-development loop
is the measure of the foundations: good enough to build several app agents
at once.

## How to use it

- **Tick rule.** As in SDC: runtime items need the pushed SHA, the staging
  deploy that carried it, and an evidence path. Owner-only items need the
  date and a one-line note. Write them on the item:
  `Done: <sha or date>, <evidence>`.
- **Who.** `owner` means only the owner can do it (accounts, money, keys,
  counsel, rulings). `agent` means agents can land it. `both` means an owner
  ruling or act unblocks agent work.
- **Order.** The phases are the intended order. Owner items in phase 0 have
  lead times, so they start now and run in parallel with everything else.
  Within an agent phase, items run in parallel where they touch different
  surfaces. Red deploys still go one at a time (SDC).
- **Inner checklists are not copied.** Phase 2 points at SDC IDs. It does not
  restate them, and SDC stays the authority for those items.
- **IDs are stable.** Add new items at the end of a phase; never renumber.
- **Status** is what the docs said on 2026-10-21. Items marked *unverified*
  are claims I could not confirm from main.

## Launch condition

- [ ] **LD-01 (owner): the Autoradio launch condition** (owner, 2026-10-21,
  verbatim in notes §37). On the phone, the owner can:
  - watch embedded YouTube on their YouTube Premium account, through Sign in
    with Google (L4-04, L8-09);
  - sign in with Apple too, because offering Google requires it (L4-04);
  - play podcasts (L8-03);
  - have e-books, arbitrary PDFs and any file read aloud (L8-10);
  - interrupt any of those by voice at any time, and have agent events
    interrupt them (L8-11).
- **The date is the push, not the gate.** Submitting for App Review by
  2026-10-31 (notes §31) stays the target. The owner expects it may take
  longer: "so does everything worthwhile in life".
- [ ] **LD-02 (owner): production.** The Terms still call choir.news a
  staging environment, and every plan treats staging as the acceptance
  environment. Launch needs one answer: is choir.news production on launch
  day, and what is the acceptance environment after it?
- **The core experience, and how far launch goes toward it.** Today the
  owner plays a PDF in Speech Central and pauses it again and again to
  dictate notes into a notes app. Autoradio merges the two. The owner
  listens to a report, answers it line by line, goes off on tangents
  ("send an email to Sally about the reservation"), and each note lands in
  the right work stream. Launch needs the listen-and-answer loop
  (L8-12); the full routing is the longer vision (L3-06).

## Phase 0: owner setup with lead times (start now, in parallel)

- [ ] **L0-01 (owner): Apple Developer Program.** Renew the lapsed
  organization account (no D-U-N-S wait, notes §31). It blocks TestFlight,
  APNs and submission.
- [ ] **L0-02 (owner): App Store Connect app record.** Bundle ID, app name,
  the Sign in with Apple capability, an APNs auth key, and internal
  TestFlight testers.
- [ ] **L0-03 (owner): Node B backup key.** Run the one key command, and keep
  the password in two offline places (reports/start-here-2026-10-20). This
  gates durable off-host backup, and every tape delete after it.
- [ ] **L0-04 (owner): counsel engagement with a named scope.** The topics
  so far: the AI-written disclosure, GDPR profiling of public figures,
  translation and quoting lines, the EU press publishers' right, the podcast
  clip line, the trademark policy, and the refreshed Terms and Privacy Policy
  (L9-01). Today the docs say only "later".
- [ ] **L0-05 (owner): mail sending domain.** SPF, DKIM and DMARC aligned
  for the sending domain, plus a support address. This unblocks the daily
  digest and the email sign-in code (maild send-gate defects G1-G3).
- [ ] **L0-06 (owner): provider accounts and budgets for launch traffic.**
  Model, search and Keenable quotas sized for strangers. The Keenable free
  tier renews on 2026-11-05. This depends on SDC DEC-07 (agent-tree budgets
  under "track spending, never cap").
- [ ] **L0-07 (both): voice pipeline hand-off.** The owner brings the voice
  project in. Agents need its repo or package location, its licence, and the
  on-device models it uses, so phase 7 can plan the integration.

## Phase 1: accelerators (each makes the next phases faster)

- [ ] **L1-01 (agent): store throughput.** The paper's single-connection
  engine was 96 to 100% busy and desk wakes waited up to 19 minutes. This is
  in flight: fair dispatch order, a separate source-poll lane, indexed reads
  for the hot paths, and the announced-wake outbox. It went from 96% busy to
  70% at c038df7c6; the four remaining equalities are indexed in 51667a3ce.
  Next: boot warm-up contention, then engine concurrency, so desk slots can
  rise above 8 (docs/problems/research-more-slow-and-hidden-2026-10-21.md).
- [ ] **L1-02 (agent): memory measurement.** The owner judged that the
  Autopaper computer uses far more memory than it needs. Measure the daemon
  RSS, the per-desk workers and the guest page cache before cutting
  anything. This feeds SDC RES-07 and RES-19, and the density numbers.
- [ ] **L1-03 (both): bigger batches and a faster landing loop.** The owner's
  rule (notes §37): stop landing one small change per deploy. One deploy
  takes about 25 minutes and its staging check about 30 more, and an
  imperfect result starts the cycle again. Land larger changes per batch,
  accept that a batch may fail in more than one place, and debug forward.
  The Landing Loop still runs, once per batch. Then cut the loop itself:
  CI takes about 20 minutes, runs serialized, and cancels a superseded run.
  Shard or cache it, and script the staging proof, so a batch lands and is
  verified in minutes.
- [ ] **L1-04 (agent): desk manifest (SDC ARCH-02).** One declared table of
  desks, roles and capabilities. Confinement, budgets and app desks then
  read it instead of each re-deriving it.

## Phase 2: security and density (SDC, all of it before launch)

The owner's rule: every security and density item we can do goes in before
launch, ordered by what speeds up the next item. SDC's own "suggested P0
order" is the conservative baseline. **The acceleration-ordered sequence
goes into SDC after the owner's review of it** (owner, 2026-10-21). The draft
of that order:

1. SEC-01 with RES-01 and RES-02: desk-cell confinement, the in-guest cgroup
   tree, and capped desk workers. Landing them together frees resources.
   The Claude delegate leaves its test computer only after SEC-01.
2. SEC-02 and SEC-03: the frame-action role check and the host egress floor.
3. REC-02, RES-05 then SEC-04: quick containment, store sharing under the
   jailer, then the jailer on.
4. SEC-05, SEC-06, SEC-07 and SEC-11: the gateway and tunnel identity and
   fetch guards.
5. SEC-08 and SEC-09: delegate posture and KSM off.
6. The memory and density items (RES-*), shaped by the L1-02 numbers.
7. The NET-* items in parallel with all of the above, by a separate agent on
   its own worktree. Networking must enable, not starve (SDC).

- [ ] **L2-01 (both): SDC DEC-04 ruled.** Which P1 items block launch. The
  owner's direction suggests all of them where possible.
- [ ] **L2-02 (agent): the blast-radius set is landed** (notes §31 ruling).
  Keys out of reach of a host compromise, per-computer keys, encrypted
  backups, Dolt locked, the jailer on, desk-cell confinement, guest egress,
  and owner-act enforcement.
- [ ] **L2-03 (agent): host kernel patched and rebooted.** The kernel was 216
  days unpatched. The reboot was approved (notes §23); *unverified* whether
  it ran.
- [ ] **L2-04 (agent): off-host backup is durable**, not the node-a stopgap.
  It needs L0-03.
- [ ] **L2-05 (both): agentic red teaming before launch** (notes §8, §37).
  Agents attack a forked computer, never a live one. The secrets vault
  comes first.
- [ ] **L2-06 (agent): Obscura and Web Lens, decided.** A Sonnet research
  pass covers their history, what still runs, and what capture depends on
  them (owner, notes §37). Live viewing of arbitrary sites moves to the
  device's own browser (WKWebView on iPhone, the platform web view
  elsewhere). Server-side capture keeps only what it needs. SDC SEC-07
  covers the address guard on those fetches. The recommendation goes here
  when the research lands.

## Phase 3: self-development

- [ ] **L3-01 (agent): LP S6, the commit gate.** This is the active station
  (docs/ACTIVE.md).
- [ ] **L3-02 (agent): S7-0, every first-party app is a package**, with a
  frontend self-development drill in parallel. The branch
  `feat/s7-0-app-packages` is paused.
- [ ] **L3-03 (agent): app agents for launch: podcast and one reader**
  (notes §37). EPUB and PDF become a single reader app agent, replacing the
  two viewers in the §33 rewrite list. Each app agent ships as a package
  with its own desk (doctrine I26). Slides, calendar and mail app agents
  may land after launch (Post-launch, below).
- [ ] **L3-04 (both): the self-development update path** as far as launch
  needs. Published source only, the host is the only builder, and
  owner-only arming (P3) lands before P4
  (reports/selfdev-update-architecture).
- [ ] **L3-05 (agent): the dual-model tool-call evaluation.** DeepSeek and
  luna run in the pool. Fix the failures they share
  (docs/problems/tool-call-failures-dual-model-2026-10-21.md).
- [ ] **L3-06 (agent): Texture as a tree** (notes §37). Each Texture
  document gets its own sub-RLM, under a root Texture agent that routes
  input to the right work stream: a dictated note, a tangent, an errand.
  The work exists in part (Texture desks, Texture articles), but not this
  shape. Launch needs enough of it for L8-12.
- [ ] **L3-07 (agent): the loop builds app agents in parallel.** The
  measure of phase 3 is that self-development builds several app agents
  at once (L3-03 first) on test computers.

## Phase 4: accounts and onboarding for strangers

- [ ] **L4-01 (agent): account recovery protector, layers 0 to 3**
  (reports/account-recovery-protector-design-2026-10-20). Slice 1 has
  landed. The owner's own recovery drill goes first. Twelve owner questions
  in the design are open.
- [ ] **L4-02 (agent): in-app account deletion** (App Review 5.1.1(v)). This
  is black-class: it destroys a user's computer and data, so it needs its
  own design and panel.
- [ ] **L4-03 (agent): admission limits or a waitlist.** Every sign-up gets a
  computer on Node B; density is roughly 1,200 to 2,000 registered users per
  host (reports/vm-density-limits). The mechanism is unspecified.
- [ ] **L4-04 (agent): Sign in with Google and Sign in with Apple** (part of
  the launch condition, LD-01). Google is also how the owner's YouTube
  account connects (L8-09), and offering it makes Apple required (App
  Review 4.8). An emailed one-time code is optional. Every one of these is
  only as strong as the account behind it, and none has a passkey's PRF to
  derive a key from. A security design with a panel comes before code (red:
  auth and session).
- [ ] **L4-05 (agent): per-user budgets and rate limits.** These cover
  Research more (open to logged-out readers today), desk runs and search.
  They depend on L0-06 and SDC DEC-07.
- [ ] **L4-06 (agent): first-run onboarding.** What a new account sees, how
  its computer boots, and how long that takes.

## Phase 5: Autopaper ready for strangers

- [ ] **L5-01 (agent): related stories, phase 3** (System 1 on). In flight.
- [ ] **L5-02 (both): op-ed section** (reports/op-ed-section-design-2026-10-20).
  Phase 1 is house pieces, unlisted. A public listing waits for the
  disclosure line (deployed) and the forecast loop. Fifteen design
  questions are open. "Once we get op-eds working, then we'll be ready for
  the auto radio app" (notes §28).
- [ ] **L5-03 (both): forecast act and settlement loop**
  (reports/forecast-act-design). Thirteen questions are open.
- [ ] **L5-04 (agent): source expansion.** AT Protocol and Fediverse leads,
  a self-hosted RSSHub design, and Telegram public previews as tips only
  (notes §30).
- [ ] **L5-05 (agent): daily digest email on.** It is deployed but off, and
  needs L0-05 and the maild gate fixes.
- [ ] **L5-06 (agent): the Autopaper name rename in code**
  (autopaper-sw-design-2026-10-09 §9).

## Phase 6: server and Texture work Autoradio needs

From reports/autoradio-mobile-app-plan §8 and
reports/texture-media-and-autoradio-design.

- [ ] **L6-01 (agent): mobile redeem with device-bound tokens.** This blocks
  M0 sign-in. Red class (auth and session).
- [ ] **L6-02 (agent): capabilities endpoint and an OpenAPI contract** for
  the app.
- [ ] **L6-03 (agent): supervision feed.** What the owner's computer is
  doing, readable from the phone.
- [ ] **L6-04 (agent): APNs relay.** Red class. It needs L0-02.
- [ ] **L6-05 (agent): approval step-up from the phone.** Red class.
- [ ] **L6-06 (agent): a take capture kind** (a recorded voice take).
- [ ] **L6-07 (agent): Texture T1 embed block, T2 publication pins, and T4
  audio timeline.** A0 starts once T1 and T4 exist.

## Phase 7: voice pipeline

The owner is building this in a separate project (L0-07). The plan of
record is reports/autoradio-mobile-app-plan and
reports/autoradio-programming-engine.

- [ ] **L7-01 (both): bring the voice project in.** Agree on its package
  boundary, licence check (no GPL: Piper and espeak-ng were ruled out), and
  model files.
- [ ] **L7-02 (agent): speech to text on the phone.** Parakeet v3 through
  FluidAudio, with Apple SpeechAnalyzer as the fallback.
- [ ] **L7-03 (agent): text to speech.** Kokoro, with AVSpeechSynthesizer
  first. The voice is recognizably a little robotic and AI, mixed with human
  voices as much as possible (notes §31).
- [ ] **L7-04 (agent): barge-in and echo cancellation.** The user can speak
  over the radio and interrupt it.
- [ ] **L7-05 (agent): measured on a real iPhone.** Latency, battery and
  memory. None of the spikes is measured yet.

## Phase 8: Autoradio app

- [ ] **L8-01 (agent): spikes.** Parakeet on an iPhone, passkey sign-in
  through ASWebAuthenticationSession, TextKit 2 editing, and the
  programming-engine spikes.
- [ ] **L8-02 (agent): milestones M0 to M3** (autoradio-mobile-app-plan).
  The stack is native Swift: UIKit core, SwiftUI where cheaper, and one
  YouTube web view.
- [ ] **L8-03 (agent): podcasts, through the podcast app agent** (L3-03).
  Full episodes stream unmodified from the publisher's enclosure URL. Clips play a short time range from
  that URL, attributed, with "play the full episode". Never re-host and
  never cut ads (notes §31).
- [ ] **L8-04 (agent): drive the autoputer from the phone.**
- [ ] **L8-05 (agent): background audio and lock-screen controls.** The
  audio background mode, Now Playing, remote commands, and interruptions
  (calls, Siri, other audio).
- [ ] **L8-06 (agent): permission strings and consent.** Microphone and
  speech recognition, and AI-data consent (App Review 5.1.2(i)).
- [ ] **L8-07 (agent): accessibility.** VoiceOver and Dynamic Type for every
  screen.
- [ ] **L8-09 (agent): embedded YouTube on the owner's Premium account**
  (LD-01). Feasibility and terms are under research: whether a Premium
  session reaches an embedded player, and what YouTube's terms and App
  Review allow for background play and interruption. The result goes here.
- [ ] **L8-10 (agent): read anything aloud, through the reader app agent**
  (L3-03). E-books, arbitrary PDFs and any other file the owner opens.
  Speech keeps a sentence position, so it can stop, resume and be
  interrupted without losing its place.
- [ ] **L8-11 (agent): one interruption layer** for every source: YouTube,
  podcasts and read-aloud. A voice barge-in or an agent event pauses or
  ducks what is playing; the owner speaks or hears the event; playback
  resumes at the right place.
- [ ] **L8-12 (agent): answer as you listen.** A note dictated while a report
  plays is bound to the line it answers, and goes to that document's
  Texture agent; the root routes a tangent elsewhere (L3-06).

## Phase 9: App Store submission (by 2026-10-31)

- [ ] **L9-01 (both): Terms and Privacy Policy refreshed.** Today they say
  staging, list Telegram as a source, and lack the AI disclosure, account
  deletion and data-handling lines. Counsel reads them (L0-04).
- [ ] **L9-02 (both): privacy nutrition label and data-use answers.**
- [ ] **L9-03 (both): user-generated content controls** (App Review 1.2):
  report, block and filter, for any content strangers can see.
- [ ] **L9-04 (both): launch free.** No in-app purchase, and no paid compute
  that would need it (3.1.1). Address the thin-client risk (4.2.7(e)) in the
  review notes.
- [ ] **L9-05 (owner): listing assets.** Name, subtitle, description,
  screenshots, preview video, age rating, and the export-compliance
  (encryption) answer.
- [ ] **L9-06 (both): App Review demo account and review notes.** Reviewers
  need to sign in and get a working computer.
- [ ] **L9-07 (both): TestFlight round**, then submit.

## Phase 10: launch day and after

- [ ] **L10-01 (agent): monitoring and alerts** for the strangers' path.
  Sign-up, computer boot, desk wake latency, memory and disk.
- [ ] **L10-02 (owner): support and incident path.** Who answers, where,
  and how fast.
- [ ] **L10-03 (agent): crash and error reporting from the app** that keeps
  user data out.
- [ ] **L10-04 (agent): capacity runbook.** What happens when Node B is
  full: the waitlist, and when the second host is needed (SP is deferred).
- [ ] **L10-05 (owner): announcement.**
- [ ] **L10-06 (agent): self-hosting path** for the public repo. No install
  document exists today. This can come after launch.

## Post-launch

- [ ] **LP-01 (agent): Slides, calendar and mail app agents** (notes §37).
  Other app agents follow as apps get used.
- [ ] **LP-02 (both): residential computers.** Choir connects to an ordinary
  desktop computer on a residential IP for computer use and browsing, or
  is self-hosted on one (a Mac mini, an old laptop) (notes §37).
- [ ] **LP-03 (agent): Android and Linux clients** use their platform web
  views for live content, as the iPhone app uses WKWebView.

## Things that may be missing (for the owner's review)

- Android is after launch in the plan (M5 needs a 12-tester Play closed test
  for 14 days). Is that still right?
- The macOS desktop app needs Developer ID signing and notarization if it is
  part of launch. The README says it is not a focus.
- A trademark policy before launch, or after.
- Billing after the free launch: in-app purchase on iOS versus the web.
- The weekly nixpkgs bump and patch-watch failures, so security patches keep
  landing after launch.
- An independent security audit. The seed deck promises one for enterprise;
  none is planned for consumers.

## Dependencies that set the order

- L0-01 → L0-02 → L6-04 (APNs), L9-07 (TestFlight).
- L0-03 → L2-04 → any tape delete.
- L1-01 → slots above 8 → more agents per computer (density).
- L1-02 → the RES-* items in phase 2.
- SDC SEC-01 → delegate use beyond its test computer → faster agent work.
- L4-01 and L4-02 → sign-in in the app (App Review 5.1.1(v)).
- L6-01 → M0 sign-in (L8-02).
- L6-07 (T1, T4) → Autoradio A0.
- L5-01 → L5-02 → Autoradio (notes §28).
- L0-07 → phase 7 → the voice parts of phase 8.
- L9-01 → L9-07.
- L4-04 (Google sign-in) → L8-09 (YouTube on the owner's Premium account).
- L3-03 (podcast and reader app agents) → L8-03 and L8-10.
- L3-06 (Texture tree) → L8-12 (answer as you listen).
- Phase 7 (voice) → L8-11 (barge-in).
