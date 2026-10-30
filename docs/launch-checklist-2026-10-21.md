# Launch Checklist

Date: 2026-10-21. Status: **living checklist, revised after a panel review**;
tick items as they land. Mutation class of this document: green (docs only).
Each item's own class is that of the work it names.

Owner direction (2026-10-21): an outer checklist around the
[security, density and architecture checklist](security-density-architecture-checklist-2026-10-21.md)
(called **SDC** below). It names every feature and requirement for launch,
from the Apple developer account to the Autoradio app, by way of the voice
pipeline (the owner is building it in a separate project and will bring it
in), the security and density work, and self-development. Do all the
security and density work before launch, and order work by what speeds up
the next piece, not only by importance: "we can blitz out all this work at
agentic pace."

Then the owner's launch condition and notes (2026-10-21,
[owner notes §37](reports/owner-notes-2026-10-14.md)): launch is a
condition, not a date; the foundations matter most (security, density, the
self-development loop); and work lands in bigger batches.

**Revision of 2026-10-21, afternoon.** At the owner's request, a
three-model panel reviewed both checklists to improve, streamline, reorder
and correct them: GPT-6.1-sol, Gemini 4 Argon and Devin. Every factual
correction below was checked against the repo before it was applied. Two
Sonnet research passes also landed: YouTube and the rest of the launch
condition (its findings are in "Owner decisions"), and Obscura and Web Lens
(L2-06). The batch plan replaces the phase order. The phases now list the
work; the batches order it.

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
  ruling, account or device act is part of the item.
- **Order.** The [batch plan](#batch-plan) is the one ordered list for both
  checklists. SDC keeps each of its items' facts and ticks, and points here
  for order.
- **Inner checklists are not copied.** Phase 2 points at SDC IDs.
- **IDs are stable.** Add new items at the end of a phase; never renumber.
  An item moved to post-launch keeps its ID and says where it went.
- **Status** is what the repo said on 2026-10-21.

## Launch condition

- [ ] **LD-01 (owner): the Autoradio launch condition** (owner, 2026-10-21,
  verbatim in notes §37). On the phone, the owner can:
  - watch embedded YouTube on their YouTube Premium account, through Sign in
    with Google (L4-04, L8-09; but see OD-01: Premium in an embed cannot be
    promised);
  - sign in with Apple too, because offering Google requires it (L4-04);
  - play podcasts (L8-03);
  - have e-books, arbitrary PDFs and any file read aloud (L8-08, L8-10;
    OD-07 names "any file");
  - interrupt any of those by voice at any time, and have agent events
    interrupt them (L8-11, L8-14; OD-05).
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
- [ ] **LD-03 (both): production made true.** The work LD-02's answer
  implies: the domain and deploy routing, the Terms (L9-01), and the
  acceptance environment after launch.

## Owner decisions the launch condition needs

From the YouTube and launch-condition research (2026-10-21). Each has a
default, so work does not wait (CLAUDE.md, No Blocking Asks); the owner
can overrule any of them.

- **OD-01 YouTube Premium.** It cannot be promised inside an app's
  embedded player. Sign in with Google gives the app a token, not a
  YouTube session in its web view. Google refuses sign-in inside an app's
  own web view (`disallowed_useragent`). Google documents Premium's
  benefits for its own apps. YouTube's developer policies forbid a
  background player, separating audio from video, and overlays on the
  player. Options: (a) a foreground embed controlled through the IFrame
  API (pause, seek, volume), with an "Open in YouTube" hand-off for
  Premium and background play; (b) SFSafariViewController, where sign-in
  works but the app cannot control playback; (c) a spike with a throwaway
  Google account to measure what a signed-in embed actually does.
  **Default:** (a), plus the (c) spike in batch 0. LD-01's YouTube clause
  would read "a foreground embed the app can interrupt; Premium where
  YouTube honors it".
- **OD-02 Google as a login, or only as "Connect YouTube".** A login brings
  App Review 4.8 (an equivalent login such as Sign in with Apple).
  **Default:** a login, with Apple, as the owner asked.
- **OD-03 Google tokens on the phone only.** App Review 5.1.1(v) forbids
  storing social-network tokens off the device, so desks cannot call the
  YouTube API with them. **Default:** accept.
- **OD-04 YouTube data scopes at launch.** Identity scopes need no review.
  `youtube.readonly` needs Google's review (days to weeks) plus a verified
  domain, a privacy policy and a demo video. **Default:** identity scopes
  for launch; `youtube.readonly` in Google's testing mode for the owner
  only, since testing mode allows up to 100 users.
- **OD-05 Hands-free voice.** "Interrupt by voice at any time" needs an
  open-mic listening session: the orange microphone dot, battery cost, and
  echo cancellation against YouTube audio that plays in another process
  (unverified). The engine plan defaults to push-to-talk.
  **Default:** push-to-talk and the headset button first; the open-mic
  session after the echo-cancellation spike (L8-01).
- **OD-06 YouTube pauses when the screen locks or the app goes to the
  background.** YouTube's terms require it. **Default:** accept.
- **OD-07 "Any file".** A named list: PDF, EPUB, text, HTML, Markdown,
  DOCX, and images through OCR. The long tail is converted by the user's
  own computer. DRM'd Kindle and Apple Books purchases are out of reach of
  any third-party app. **Default:** that list.
- **OD-08 Podcasts.** Streaming only at launch; downloads after counsel
  (L0-04). **Default:** streaming only.
- **OD-09 Text to speech.** AVSpeechSynthesizer first, or Kokoro on
  AVAudioEngine (better voice, and its audio runs where echo cancellation
  can hear it). **Default:** AVSpeechSynthesizer first, then Kokoro.

## Batch plan

Rules, reconciled from the three reviews:

- **A batch is one protected surface and one rollback unit.** It gets one
  deploy and one acceptance proof, not one per item.
- **Batches on different surfaces are built in parallel**, on separate
  worktrees by separate agents. Node B activation stays serialized: red
  batches land one after another, and each is ready the moment the last is
  verified. Two red batches on unrelated surfaces never share a deploy.
- **Order is by what a batch unlocks.** Every security and density item
  lands before launch (owner notes §37; SDC DEC-04 answered).

The batches:

- **B0, now, no platform deploy.** Owner lead times: L0-01, L0-02, L0-04,
  L0-06, L0-07, L0-08, LD-02. Device spikes on a physical iPhone: L8-01.
  Measurements: L1-02 (SDC RES-07, RES-19). Decisions: OD-01 to OD-09,
  L2-06. Contract: L6-02's API frozen so the app can build against it.
  Proof: dated owner receipts, device measurements, and a written YouTube
  verdict.
- **B1, landing machinery.** L1-03 and SDC SEC-14. Proof: one timed
  CI-to-deploy-to-proof run.
- **B2, store and daemon throughput.** L1-01, SDC RES-18, L3-05. Proof: the
  paper boots in under two minutes, and the engine's busy share and desk
  wake waits are measured under load.
- **B3, the guest desk boundary.** SDC ARCH-02 (this is L1-04), ARCH-05,
  ARCH-07, SEC-01, SEC-02, SEC-13, SEC-15, RES-01 to RES-04. The manifest
  is the one table that confinement, role checks and budgets read. Proof:
  forged-frame, escape and out-of-memory probes fail safely, and real desk
  turns still work.
- **B4, the host network floor.** SDC SEC-03, SEC-09, SEC-10, NET-07.
  Proof: cross-tenant, private and metadata destinations are refused, and
  SDC's networking capabilities still pass.
- **B5, host-service authority.** SDC REC-02, SEC-11, SEC-12. Proof:
  unauthorized callers and passwordless database access are refused;
  owner acts still succeed.
- **B6, VM image, jailer and density.** SDC RES-05 then SEC-04, NET-08 then
  RES-16, RES-06, RES-09, RES-14; L4-03. Proof: cold boot, snapshot wake,
  reattach and burst reclaim work; admission refuses safely when full.
- **B7, gateway identity and fairness.** SDC SEC-05, SEC-08 (Option K),
  SEC-16, RES-11, RES-12; L4-05. Proof: a bearer from another computer is
  refused, and computers share the gateway fairly.
- **B8, host-mediated network.** SDC SEC-06, SEC-07 (with the Obscura
  deletion, L2-06), NET-01, NET-02, NET-05, NET-10. Proof: the full network
  capability matrix passes; general guest egress closes only after every
  replacement passes.
- **B9, keys and recovery.** SDC NET-06, NET-09; L2-04. Proof: an encrypted
  off-host backup restores, and keys revoke. L2-02 is satisfied when the
  security items of B3 to B9 are ticked.
- **B10, the self-development spine** (its own lane from today). L3-01,
  L3-04, L3-08; then L3-02 with SDC ARCH-03 and ARCH-04; then L3-03 and
  L3-07. Proof: the podcast and reader app agents are built at the same
  time on test computers and installed as packages.
- **B11, accounts and session.** L4-01, L4-02, L4-04, L6-01, L6-05. One
  coherent contract for mobile sign-in, recovery, step-up and deletion.
  Proof: Google, Apple and passkey flows on a real device; recovery and
  deletion on a disposable account.
- **B12, event delivery and Texture interaction.** L6-03, L6-04, L8-14;
  L3-06 (minimum routing), L6-06, L6-07. Proof: dictate against a
  sentence, route a tangent, restart, and recover the right document and
  anchor; an agent event interrupts listening.
- **B13, the native listening release.** L7-01 to L7-06, L8-02 to L8-11,
  L8-13, L8-15. Proof: the LD-01 matrix on a physical iPhone through
  TestFlight, against the deployed APIs.
- **B14, stranger readiness.** L10-01, L10-03 and L10-04 before admission
  opens; L2-07, then L2-05; L4-06; L9-01 to L9-07; RES-19's
  after-measurement; the LD-01 and LD-02 receipts. Proof: a
  release-candidate soak, a bounded sign-up surge, the red-team report,
  and an alert-and-recovery drill.

**Lanes that run in parallel from today:** the store (B2); the guest
boundary (B3); the host (B4 to B9, built in parallel and landed in order);
self-development (B10); accounts and the Autoradio server (B11, B12); the
iPhone (B0's spikes, then B13; it needs a Mac with Xcode); and the
remaining Autopaper items (phase 5).

**Critical path and estimates.** The YouTube verdict, then sign-in, then
the native integration of the sources and the interruption layer. The
external clocks no batch size speeds up: the Apple renewal, Google's
verification, and App Review. The panel's planning estimates (not
measurements) run from about 7 to 9 days (Gemini 4 Argon) to 10 to 18
engineering days (GPT-6.1-sol), plus external review. The top risks, in
the panel's agreed order:
1. the YouTube embed path (OD-01);
2. hands-free audio across YouTube and iOS audio routes (OD-05);
3. capacity for strangers on Node B (L4-03), and the review clocks.

## Phase 0: owner setup with lead times (start now, in parallel)

- [ ] **L0-01 (owner): Apple Developer Program.** Renew the lapsed
  organization account (no D-U-N-S wait, notes §31). It blocks Sign in with
  Apple, TestFlight, APNs and submission.
- [ ] **L0-02 (owner): App Store Connect app record.** Bundle ID, app name,
  the Sign in with Apple capability, an APNs auth key, and internal
  TestFlight testers.
- [x] **L0-03 (owner): Node B backup key.** Done: 2026-10-20. The key on
  Node B decrypts the node-a repository, and the owner's offline copy
  matched (reports/off-host-restore-drill-2026-10-20, "MATCH").
- [ ] **L0-04 (owner): counsel engagement with a named scope.** The topics
  so far: the AI-written disclosure, GDPR profiling of public figures,
  translation and quoting lines, the EU press publishers' right, the podcast
  clip line and downloads (OD-08), the trademark policy, and the refreshed
  Terms and Privacy Policy (L9-01). Today the docs say only "later".
- [ ] **L0-05 (owner): mail sending domain, the rest.** Corrected
  2026-10-21: DKIM, SPF and DMARC (`p=none`) are already live for
  choir.news (reports/autopaper-daily-digest-design-2026-10-20). What
  remains: a DMARC report address and a firmer policy, bounce and complaint
  handling, and a support mailbox.
- [ ] **L0-06 (owner): provider accounts and budgets for launch traffic.**
  Model, search and Keenable quotas sized for strangers. The Keenable free
  tier renews on 2026-11-05. Quota planning does not need to wait for SDC
  DEC-07.
- [ ] **L0-07 (both): voice pipeline hand-off.** The owner brings the voice
  project in. Agents need its repo or package location (`~/mike`, the
  owner's separate project, noted 2026-10-21), its licence, and the
  on-device models it uses. Not in the first pass (owner notes §38). L7-01 is the integration side of this item.
- [ ] **L0-08 (owner): Google Cloud project and OAuth clients.** The
  consent screen, brand verification (days), the iOS and web clients, and
  the scope decision (OD-04). Submit early: it is an external clock.

## Phase 1: accelerators (each makes the next phases faster)

- [ ] **L1-01 (agent): store throughput.** The paper's single-connection
  engine was 96 to 100% busy and desk wakes waited up to 19 minutes. It
  went to 70% busy at c038df7c6. The next step, four stored generated
  columns (51667a3ce), blocked the paper's boot: adding them rewrites every
  row of `og_objects` on the boot path, holding the store's one connection
  (docs/archive/problems/stored-column-add-blocks-paper-boot-2026-10-21.md). It was
  reverted in f2c4a3ec3. Measured locally on 20,000 rows: a plain nullable
  column adds in 8 ms and indexes in 94 ms, while a stored generated column
  takes 2.3 to 3.6 s. Next: plain columns that the four writers of
  `og_objects` fill, an online backfill in bounded batches with progress
  logs, and reads on the JSON path until a column is complete. Then boot
  warm-up contention, then engine concurrency, so desk slots can rise
  above 8 (docs/problems/research-more-slow-and-hidden-2026-10-21.md).
- [ ] **L1-02 (agent): memory measurement**, which is SDC RES-07 and
  RES-19. The owner judged that the Autopaper computer uses far more memory
  than it needs. Measure the daemon RSS, the per-desk workers and the guest
  page cache before cutting anything.
- [ ] **L1-03 (both): bigger batches and a faster landing loop.** The owner's
  rule (notes §37): stop landing one small change per deploy. One deploy
  takes about 25 minutes and its staging check about 30 more, and an
  imperfect result starts the cycle again. Land a batch (see the batch
  plan), accept that it may fail in more than one place, and debug forward.
  The Landing Loop still runs, once per batch. Corrected 2026-10-21: CI on
  main never cancels a run in flight, and deploys queue newest-first
  (`.github/workflows/ci.yml:21-28`); tests already run in eight
  non-runtime and eight runtime shards. So measure where the time goes
  (queue, build, image, refresh, proof) before changing CI, and script the
  staging proof.
- [ ] **L1-04 (agent): desk manifest.** This is SDC ARCH-02, landed in
  batch B3.

## Phase 2: security and density (SDC, all of it before launch)

Every SDC item lands before launch (owner notes §37; SDC DEC-04 answered).
The batch plan orders them.

- [x] **L2-01 (both): SDC DEC-04 ruled.** Done: 2026-10-21, owner notes §37.
  Every security and density item that can land before launch does.
- [ ] **L2-02 (agent): the blast-radius set is landed** (notes §31 ruling).
  This is an acceptance gate, not separate work: it is satisfied when the
  security items of batches B3 to B9 are ticked. It reduces what a host
  compromise exposes; it cannot protect against live host root
  (problems/host-compromise-blast-radius-2026-10-20).
- [x] **L2-03 (agent): host kernel patched and rebooted.** Done: 2026-10-15
  (reports/node-b-host-reboot-2026-10-15): kernel 6.18.55, and
  verify-host-hardening went from 69 to 77 passing. Keeping patches current
  is SDC NET-08 and RES-16.
- [ ] **L2-04 (agent): backup ready for launch.** Node-a is the accepted
  launch target (owner notes §27 item 17); durable offsite storage comes
  later. What remains before any tape delete: the tape-segment repack and
  census, and a restore drill on the segment snapshot.
- [ ] **L2-05 (both): agentic red teaming before launch** (notes §8, §37).
  Agents attack a forked computer, never a live one. Fork re-keying and the
  secrets vault (L2-07) come first.
- [ ] **L2-06 (agent): Obscura and Web Lens, decided.** Research done
  2026-10-21. No automated capture path uses Obscura: research Read,
  content import and the source pollers all use `sourcefetch`. Its only
  caller is the Web Lens "Snapshot" button. It runs untrusted JavaScript as
  guest root on an old V8, with no address guard (SDC SEC-07, SEC-15), and
  costs about 508 MiB of image. **Recommendation (default):** delete it,
  and make Web Lens open the device's own browser, since a top-level page
  is not subject to frame blocking. The steps:
  1. a problem record;
  2. unset `CHOIR_OBSCURA_BIN` (reversible);
  3. Web Lens becomes a launcher: Source Viewer snapshot plus "Open
     original", with the `web_lens` value still accepted in stored data;
  4. delete `internal/browsercontrol` and its routes;
  5. remove the package from the image (red), in batch B8.

  Residential-computer browsing stays post-launch (LP-02).
- [ ] **L2-07 (both): the secrets vault.** It comes before red teaming
  (notes §8). Neither checklist had it as an item.

## Phase 3: self-development

- [ ] **L3-01 (agent): LP S6, the commit gate.** This is the active station
  (docs/ACTIVE.md; its goal file is reviewed).
- [ ] **L3-02 (agent): S7-0, every first-party app is a package**, with a
  frontend self-development drill in parallel. The branch
  `feat/s7-0-app-packages` is paused. For launch, scope it to the package
  and desk loader, plus Texture, podcast and reader.
- [ ] **L3-03 (agent): app agents for launch: podcast and one reader**
  (notes §37). EPUB and PDF become a single reader app agent, replacing the
  two viewers in the §33 rewrite list. Each app agent ships as a package
  with its own desk (doctrine I26). The Slides, Calendar and FileBrowser
  rewrites move to post-launch (LP-01).
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
  Split: the minimum root routing that L8-12 needs lands before launch;
  the full per-document tree may finish after.
- [ ] **L3-07 (agent): the loop builds app agents in parallel.** The
  measure of phase 3 is that self-development builds several app agents
  at once (L3-03 first) on test computers. Say which is proven: several
  computers in parallel, or several capsules on one.
- [ ] **L3-08 (agent): engineering admission does not cancel live work.**
  Today opening an assignment cancels the live one
  (`internal/agentcore/engineering_assignment_runtime.go:651-660`);
  doctrine says FIFO with a retryable refusal. SDC ARCH-10's launch half.

## Phase 4: accounts and onboarding for strangers

- [ ] **L4-01 (agent): account recovery protector, layers 0 to 3**
  (reports/account-recovery-protector-design-2026-10-20). Slice 1 has
  landed. The owner's own recovery drill goes first. Twelve owner questions
  in the design are open. It gates public sign-up, not the development of
  mobile sign-in.
- [ ] **L4-02 (both): in-app account deletion** (App Review 5.1.1(v)). The
  implementation is red. Running a deletion is black: it destroys a user's
  computer and data. The owner ratifies the retention and grace window.
  Revoke Apple and Google tokens, and delete YouTube-derived data within
  seven days of a revoke (YouTube policy III.D.2).
- [ ] **L4-03 (agent): admission limits or a waitlist.** Every sign-up gets a
  computer on Node B. Corrected 2026-10-21: Node B serves about 100 more
  registered users today, with only 2 or 3 busy computers beside the paper
  and the owner, and about 400 after the medium tier of density work
  (reports/vm-density-limits-2026-10-14). The 1,200 to 2,000 figure was for
  a larger EPYC host. Nothing was load-tested. Decide the first cap and the
  mechanism now.
- [ ] **L4-04 (both): Sign in with Google and Sign in with Apple** (part of
  the launch condition, LD-01). Offering Google makes an equivalent login
  required (App Review 4.8), and the owner wants Apple. Google sign-in
  does not carry a YouTube session into an embedded player (OD-01).
  Add an identities table keyed by provider and subject. Verify ID tokens
  and nonces on the server. Never merge accounts on email. Google tokens
  stay on the phone (OD-03). An emailed one-time code is optional. None of
  these has a passkey's PRF to derive a key from. A security design with a
  panel comes before code (red: auth and session).
- [ ] **L4-05 (agent): per-user budgets and rate limits.** These cover
  Research more (open to logged-out readers today), desk runs and search.
  They depend on L0-06 and SDC DEC-07.
- [ ] **L4-06 (agent): first-run onboarding.** What a new account sees, how
  its computer boots, and how long that takes, shown as progress.

## Phase 5: Autopaper ready for strangers

- [ ] **L5-01 (agent): related stories, phase 3** (System 1 on). In flight.
- [ ] **L5-02 (both): op-eds, the rest.** Corrected 2026-10-21: signed-in
  users publishing to an Opinion section is deployed and proven
  (docs/ACTIVE.md). Before launch: moderation and unpublish (with L9-03).
  House op-eds with forecasts move to post-launch (LP-04).
- [ ] **L5-03 (both): forecast act and settlement loop.** Moved to
  post-launch (LP-04): the launch condition does not need it.
- [ ] **L5-04 (agent): source expansion.** AT Protocol and Fediverse leads,
  and Telegram public previews as tips only (notes §30). The self-hosted
  RSSHub moves to post-launch (LP-05).
- [ ] **L5-05 (agent): daily digest email on.** It is deployed but off. It
  needs the rest of L0-05 and the maild gate fixes.
- [ ] **L5-06 (agent): the Autopaper name rename in code.** Moved to
  post-launch (LP-06): a rename across many files collides with every
  active branch.

## Phase 6: server and Texture work Autoradio needs

From reports/autoradio-mobile-app-plan §8 and
reports/texture-media-and-autoradio-design.

- [ ] **L6-01 (agent): mobile redeem with device-bound tokens.** This blocks
  M0 sign-in. Red class (auth and session).
- [ ] **L6-02 (agent): capabilities endpoint and an OpenAPI contract** for
  the app. Freeze it in batch B0 so the app builds against it.
- [ ] **L6-03 (agent): supervision feed.** What the owner's computer is
  doing, readable from the phone, with a durable cursor.
- [ ] **L6-04 (agent): APNs relay.** Red class. It needs L0-02.
- [ ] **L6-05 (agent): approval step-up from the phone.** Red class.
- [ ] **L6-06 (agent): a take capture kind** (a recorded voice take).
- [ ] **L6-07 (agent): Texture T1 embed block and T4 audio timeline.** A0
  starts once both exist. T2 publication pins is split out: it lands only
  if a launch item needs it.

## Phase 7: voice pipeline

The owner is building this in a separate project (L0-07). The plan of
record is reports/autoradio-mobile-app-plan and
reports/autoradio-programming-engine.

- [ ] **L7-01 (both): bring the voice project in** (the integration side of
  L0-07). Agree on its package boundary, licence check (no GPL: Piper and
  espeak-ng were ruled out), and model files.
- [ ] **L7-02 (agent): speech to text on the phone.** Parakeet v3 through
  FluidAudio, with Apple SpeechAnalyzer as the fallback.
- [ ] **L7-03 (agent): text to speech.** AVSpeechSynthesizer first, then
  Kokoro (OD-09). The voice is recognizably a little robotic and AI, mixed
  with human voices as much as possible (notes §31).
- [ ] **L7-04 (agent): barge-in and echo cancellation.** The user can speak
  over the radio and interrupt it (OD-05).
- [ ] **L7-05 (agent): measured on a real iPhone.** Latency, battery and
  memory. None of the spikes is measured yet.
- [ ] **L7-06 (both): the integration bar.** "Voice integrated" means a
  device-verified end-to-end run: speak, be heard, interrupt each source,
  and resume, with measured latency.

## Phase 8: Autoradio app

- [ ] **L8-01 (both): device spikes, in batch B0.** On a physical iPhone:
  the YouTube embed (IFrame API pause and resume, screen lock, and the
  OD-01 spike with a throwaway Google account); the audio session with a
  playing web view and the microphone together (echo cancellation);
  Parakeet; and passkey sign-in through ASWebAuthenticationSession. The
  TextKit 2 editing spike moves to post-launch (LP-07): version 1 has no
  block editing on the phone.
- [ ] **L8-02 (agent): the native app** (autoradio-mobile-app-plan). The
  stack is native Swift: UIKit core, SwiftUI where cheaper, and one
  YouTube web view. Acceptance is the LD-01 matrix, not the inherited M0
  to M3 scope.
- [ ] **L8-03 (agent): podcasts, through the podcast app agent** (L3-03).
  Full episodes stream unmodified from the publisher's enclosure URL, with
  Now Playing and chapters. Clips play a short time range from that URL,
  attributed, with "play the full episode". Never re-host and never cut
  ads (notes §31).
- [ ] **L8-04 (agent): drive the autoputer from the phone.** Named verbs
  for launch: see the supervision feed, approve or deny, send a note to a
  desk, and start a run. Anything more is post-launch.
- [ ] **L8-05 (agent): background audio and lock-screen controls.** The
  audio background mode, Now Playing, remote commands, and interruptions
  (calls, Siri, other audio, Bluetooth route changes). YouTube pauses on
  lock (OD-06).
- [ ] **L8-06 (agent): permission strings and consent.** Microphone and
  speech recognition, and AI-data consent before voice or notes reach a
  model provider (App Review 5.1.2(i)).
- [ ] **L8-07 (agent): accessibility.** VoiceOver and Dynamic Type for every
  screen.
- [ ] **L8-08 (agent): file import.** The document picker, the share sheet
  and Files: files reach the reader from anywhere. The OD-07 list is read
  on the phone (PDFKit, EPUB text, Vision OCR for images and scans), and
  the long tail is converted by the user's computer. (Added 2026-10-21;
  the ID was unused.)
- [ ] **L8-09 (both): embedded YouTube** (LD-01, OD-01). A foreground IFrame
  embed of at least 200 by 200 points, with nothing over the player. It is
  controlled through the IFrame API, sends the app's Referer, and offers an
  "Open in YouTube" hand-off. Premium only where YouTube honors it. Never
  background play or audio extraction.
- [ ] **L8-10 (agent): read anything aloud, through the reader app agent**
  (L3-03). Text is split into sentences, one utterance each, so speech
  can stop, resume and be interrupted without losing its place; position
  is saved as a locator plus a sentence index.
- [ ] **L8-11 (agent): one interruption layer** for every source: YouTube,
  podcasts and read-aloud. A voice barge-in or an agent event pauses what
  is playing; the owner speaks or hears the event; playback resumes at
  the right place (TTS at the sentence, a podcast a few seconds back,
  YouTube a few seconds back while visible).
- [ ] **L8-12 (agent): answer as you listen.** A note dictated while a report
  plays is bound to the line it answers, and goes to that document's
  Texture agent; the root routes a tangent elsewhere (L3-06).
- [ ] **L8-13 (agent): App Store technicals.** The privacy manifest and
  required-reason APIs, third-party SDK manifests (FluidAudio, Kokoro),
  IPv6-only networking, and the on-device model download size and storage
  policy.
- [ ] **L8-14 (agent): the agent-event channel.** A foreground socket while
  listening, and APNs when the app is in the background (L6-04). Event
  priority and duplicate suppression.
- [ ] **L8-15 (agent): podcast discovery.** Search and subscribe through a
  public directory (the iTunes Search API or Podcast Index). It belongs in
  SDC NET-01's network inventory.

## Phase 9: App Store submission (target 2026-10-31)

- [ ] **L9-01 (both): Terms and Privacy Policy refreshed.** Today they say
  staging, list Telegram as a source, and lack the AI disclosure, account
  deletion and data-handling lines. Counsel reads them (L0-04).
- [ ] **L9-02 (both): privacy nutrition label and data-use answers.**
- [ ] **L9-03 (both): user-generated content controls** (App Review 1.2):
  report, block and unpublish, for any content strangers can see,
  starting with the live op-eds.
- [ ] **L9-04 (both): launch free.** No in-app purchase, and no paid compute
  that would need it (3.1.1). Address the thin-client risk (4.2.7(e)) in the
  review notes, and say that app packages run on the user's server
  computer, not on the phone.
- [ ] **L9-05 (owner): listing assets.** Name, subtitle, description,
  screenshots, preview video, age rating, and the export-compliance
  (encryption) answer.
- [ ] **L9-06 (both): App Review demo account and review notes.** Reviewers
  need to sign in and get a working computer, inside the admission cap.
- [ ] **L9-07 (both): TestFlight round**, then submit.

## Phase 10: launch day and after

- [ ] **L10-01 (agent): monitoring and alerts** for the strangers' path:
  sign-up, computer boot, desk wake latency, memory and disk. Before
  admission opens (B14), not on launch day.
- [ ] **L10-02 (owner): support and incident path.** Who answers, where,
  and how fast.
- [ ] **L10-03 (agent): crash and error reporting from the app** that keeps
  user data out. Before TestFlight.
- [ ] **L10-04 (agent): capacity runbook.** What happens when Node B is
  full: the waitlist, and when the second host is needed (SP is deferred).
- [ ] **L10-05 (owner): announcement.**
- [ ] **L10-06 (agent): self-hosting path** for the public repo. No install
  document exists today. This can come after launch.
- [ ] **L10-07 (both): backup objectives stated.** How much data a restore
  may lose, and how long it may take.

## Post-launch

- [ ] **LP-01 (agent): Slides, calendar and mail app agents** (notes §37),
  and the Slides, Calendar and FileBrowser rewrites (from L3-03). Other
  app agents follow as apps get used.
- [ ] **LP-02 (both): residential computers.** Choir connects to an ordinary
  desktop computer on a residential IP for computer use and browsing, or
  is self-hosted on one (a Mac mini, an old laptop) (notes §37). A cheaper
  bridge first: the owner's own client captures a page in its web view and
  uploads the readable text as a content item.
- [ ] **LP-03 (agent): Android and Linux clients** use their platform web
  views for live content, as the iPhone app does.
- [ ] **LP-04 (both): house op-eds with forecasts, and the forecast act and
  settlement loop** (from L5-02 and L5-03; reports/forecast-act-design has
  thirteen open questions).
- [ ] **LP-05 (agent): the self-hosted RSSHub** (from L5-04).
- [ ] **LP-06 (agent): the Autopaper name rename in code** (from L5-06),
  in a quiet window.
- [ ] **LP-07 (agent): block editing on the phone** (TextKit 2, from
  L8-01).
- [ ] **LP-08 (both): Android** (M5 needs a 12-tester Play closed test for
  14 days).

## Things that may be missing (for the owner's review)

- The macOS desktop app needs Developer ID signing and notarization if it is
  part of launch. The README says it is not a focus.
- A trademark policy before launch, or after.
- Billing after the free launch: in-app purchase on iOS versus the web.
- The weekly nixpkgs bump and patch-watch failures, so security patches keep
  landing after launch.
- An independent security audit. The seed deck promises one for enterprise;
  none is planned for consumers.
- App Review's view of public-figure profiling in Autopaper.

## Dependencies that set the order

- L0-01 → L0-02 → L6-04 (APNs), L9-07 (TestFlight).
- L0-08 → L4-04 (Google sign-in) and any YouTube data scope.
- L8-01 (YouTube spike) → OD-01 → L8-09.
- L2-04 → any tape delete.
- L1-01 → slots above 8 → more agents per computer (density).
- L1-02 → the RES-* items.
- SDC SEC-01 and SEC-08's Option K → delegate use beyond its test
  computer → faster agent work.
- SDC ARCH-02 → SEC-02's role table (or SEC-02 ships an interim table).
- L3-08 → L3-07 (app agents built at once).
- L4-01 and L4-02 → public sign-up (App Review 5.1.1(v)).
- L6-01 → M0 sign-in (L8-02).
- L6-07 (T1, T4) → Autoradio A0.
- L5-01 → L5-02 → Autoradio (notes §28).
- L0-07 → phase 7 → the voice parts of phase 8.
- L6-04 → L8-14 (agent events while in the background).
- L9-01 → L9-07.
- L3-03 (podcast and reader app agents) → L8-03 and L8-10.
- L3-06 (minimum routing) → L8-12 (answer as you listen).
- Phase 7 (voice) → L8-11 (barge-in).
- L2-07 and fork re-keying → L2-05.
