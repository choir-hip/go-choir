# Handoff: the Autoradio mission (2026-11-09)

Mutation class: green (a design handoff; no runtime change). Rollback: revert.

Source: a design conversation with the owner on 2026-11-09, plus a read of
the repo at `86e72d71f`. This is input for a throughline `/goal` file, not a
goal file. The main thread authors the goal file with
`skills/throughline/SKILL.md` when the Autopaper mission
([handoff](autopaper-next-mission-handoff-2026-11-09.md)) has finished its
first pass.

The settled product decisions are in section 7 of that handoff and are not
repeated here. This document adds what exists, what must be built, the
lanes, and what done means.

## 1. Where this sits

- Roadmap (`docs/roadmap-2026-10-31.md:47-52`): the voice layer and
  Autoradio are the stage that "completes the functional requirements for
  launch". `docs/ACTIVE.md:23`: Autopaper first, then Autoradio.
- It starts after pass 1 of the Autopaper mission. Three lanes (section 4:
  N, V, S1) depend on nothing in that mission and can start earlier if
  builders are free.
- Run it the same way: one wide build pass, a QA pass, a second pass
  (Autopaper handoff, section 2).

## 2. The product in one paragraph

An iPhone app that is Texture and Autopaper driven by voice. It plays the
reader's edition aloud in an AI voice, plays podcast audio as the human
content, and lets the reader interrupt by voice to ask, comment or redirect.
Everything on screen is a Texture page rendered natively. Version 1 has no
over-the-air code updates, no desktop, no app building and no YouTube.

## 3. What exists today

Verified in code unless marked as a doc's claim.

| Area | State |
|---|---|
| Native app project | None. No Xcode, Gradle, nsh or `mobile/` tree in the repo |
| Voice | None in the repo. On-device dictation, read-aloud and a hands-free engine exist in the Mike repo (`mobile/plugins/speech/`; roadmap's claim, not read here). The prompt bar's voice button is a placeholder (`PromptBar.surface.svelte:664`) |
| Podcasts | Running: `internal/agentcore/podcast.go`, routes `/api/podcast/subscriptions`, `/refresh`, `/search` (`internal/apihandler/routes.go:34-36`), and a web surface |
| Embeds in Texture | Running: `internal/texturedoc/embed.go`, `embed_av.go` (image, file, video, audio URL). Publication refuses any document holding an embed |
| Audio bytes | `/api/assets/sha256/<hex>` serves images only; audio and video are refused (`internal/agentcore/api_assets.go`) |
| Audio timeline | Designed (`docs/reports/texture-media-and-autoradio-design-2026-10-20.md`, section 5: per-revision segments tied to block ids), not built |
| Autoradio data kinds | Reserved, unused: `choir.media_item`, `choir.audio_recording`, `choir.transcript`, `choir.autoradio_run_sheet` under an `autoradio` service (`internal/kernelservice/kernelservice.go:45-46,110-112`) |
| Sign-in for a non-browser client | Device authorization exists (`/auth/device/*`, `internal/auth/device_login.go`): passkey approval, then one full-access API key, which the proxy accepts as a bearer token. No named revocable device, no refresh, no Sign in with Apple |
| Autopaper read API | Running and public: `/api/platform/publications/{front,resolve,export,related,...}` |
| Texture API | Running: `/api/texture/documents`, revisions, stream, events, revise; `/api/prompt-bar` |
| Push notifications | None. No APNs or FCM code |
| Radio program engine | Designed (`docs/archive/reports/autoradio-programming-engine-2026-10-14.md`), not built. The October browser prototype is no longer in the tree |

What the earlier docs still contribute, now that a cross-platform runtime replaces the Swift
plan:
- web-view audio dies in the background, so playback is native
  (`autoradio-native-stack-comparison-2026-10-20.md`);
- golden documents keep two renderers in step (same doc);
- the audio layer is data, a timeline over block ids (media design,
  section 5);
- the mobile sign-in and push designs
  (`docs/archive/reports/autoradio-mobile-app-plan-2026-10-14.md`, §8.1,
  §8.2) and launch checklist phases 6 and 7
  (`docs/launch-checklist-2026-10-21.md:419-460`).

## 4. Lanes

The owner's rule from 2026-11-09 holds here: "we should look to fix existing
systems, not build a new system, and then we have two systems for the same
function." Each server lane extends the mechanism named in section 3.

**Step 0, before any lane (contracts, one agent, a day):**

- 0.1 **The page format.** One written spec of what the server sends for a
  Texture page, which both the Svelte and the native renderer draw. Golden
  documents are its test. It includes the audio timeline.
- 0.2 **The frozen API.** The app cannot change after review, so the
  endpoints it calls are listed, versioned and frozen, with a rule for what
  the app does when the server is newer.
- 0.3 **The runtime spike, on Exact.** Both codebases were read at source
  on 2026-11-09 (`expo/exact` at `94b8671`, `longbridge/nsh` at `506b539`).
  Exact is the lead; nsh is the fallback.

  What Exact already has in its tree:
  - one owner of the audio session, which composes playback, microphone
    capture, speech and calls (`host/apple/Sources/ExactKit/AudioSession.swift`);
  - an `audio` element with `MediaMetadata`, Now Playing and remote
    commands (`host/apple/videoarm/NowPlaying.swift`);
  - microphone capture with a voice-processing option
    (`packages/media-capture`), speech recognition on the device
    (`packages/speech-recognition`) and speech synthesis that can keep
    speaking in the background (`packages/speech-synthesis`);
  - store purchases, passkeys, sign-in, OAuth and Keychain packages;
  - a test driver that works on a physical iPhone and simulates heard and
    spoken words on a clock, so agents can test voice flows.

  What nsh says of itself (`docs/dev/audio-design.md`): background
  playback, Now Playing, remote commands, recording and text to speech are
  non-goals for now; players pause when the app leaves the foreground; the
  iOS audio path is "built, not run". We would write all of lanes V and P
  ourselves on it.

  Exact's costs:
  - it is pre-alpha and changes daily, with no releases and no packaged
    CLI. Pin a commit and move the pin deliberately;
  - remote notification registration is refused today
    (`docs/contract-grammar.md`). Push needs a Swift module of ours or an
    upstream change;
  - installed apps fetch signed UI and asset bundles by default. Version 1
    must ship with that off; the spike finds how;
  - TypeScript data logic ships as Hermes bytecode. A Rust data module
    carries no interpreter, which is the simpler App Review story;
  - Android is early. That is acceptable for an iOS-first launch.

  The spike, on a real iPhone, at a pinned commit:
  - play long audio with the screen locked, with Now Playing and remote
    commands;
  - run Mike's voice stack as a Swift module while that audio plays
    (owner, 2026-11-09: the voice stack is ours and is good enough). The
    risk is ownership, not quality: Exact's audio session owner is the
    only caller of `setCategory`, so Mike's stack must take the session
    through Exact's capture and speech claims, or through the capture
    package's engine hatch, and not configure the session itself;
  - keep listening while the app is not in front. Exact's own recogniser
    stops there; ours must not;
  - render a 20,000-word page with cites and images, scroll it, and read
    it with VoiceOver;
  - produce a release build that fetches no update bundle.

  If Exact fails a check that a Swift module of ours cannot repair, run the
  same checks on nsh. If both fail, stop and write the finding; do not fall
  back silently to a third stack.

**Wave 1 (parallel):**

| Lane | Scope | Depends on | Extends |
|---|---|---|---|
| N. Native shell | Exact app at a pinned commit; navigation; the build and TestFlight path; the Apple account, bundle id and signing | 0.3 | new tree `mobile/` |
| V. Voice | Port Mike's recognition, turn-taking, echo cancellation and AI voice as a Swift module that takes the audio session through Exact's claims; Exact's own speech packages are the fallback and the agent-test double; push-to-talk first, open mic after (launch checklist OD-05) | 0.3 | Mike's code, relicensed |
| P. Playback | Long-form background playback, Now Playing and lock-screen controls, clip start and end anchored to transcripts, the podcast as the echo reference | 0.3 | new, in `mobile/` |
| S1. Sign-in | A named, revocable device with a refreshable token, approved by passkey; account deletion from the app | 0.2 | the device authorization flow (`internal/auth/device_login.go`); no second sign-in system |
| S2. Audio in Texture | The audio timeline on a revision; audio bytes served by the asset endpoint; published stories may carry audio and images | 0.1 | `internal/texturedoc/embed*.go`, `api_assets.go`, the publish gate |

**Wave 2:**

| Lane | Scope | Depends on | Extends |
|---|---|---|---|
| R. Native Texture renderer | Draw the page format in TSX: text, cites, images, audio, the reader's fork and comments; no block editing on the phone | 0.1, N | the page format; golden documents shared with the web renderer |
| D. The Autoradio desk | Writes the run sheet: which stories and podcast clips play, in what order, with the AI voice between them; reacts to an interruption | S2, Autopaper lanes B and H1 | the reserved `autoradio` service and its four kinds; the podcast subscriptions that already run |
| U. Voice as input | A spoken request goes through the prompt bar's route: ask, comment, fork, add a source | V, R, Autopaper lanes C1, D, F | `/api/prompt-bar`; no second input path |
| X. Push | A relay that turns changelog events into APNs notifications which open native screens | S1, Autopaper lanes E and H1 | the changelog events; the relay itself is new, since none exists |
| M. Subscription | The Choir account's subscription, bought in the app or on the web at the same price | S1 | the account; store purchase wiring is new |

**Wave 3:**

| Lane | Scope |
|---|---|
| A. App Review | Review notes that describe the app plainly, the privacy labels, account deletion, a reviewer account, then submission |

Notes on the lanes:
- **D settles the open question** "who writes the radio program". The desk
  model and the already-reserved service both point to an Autoradio desk
  that is the custodian of the run sheet. Texture stays the custodian of
  documents, and the desk reads them through leases.
- **The programming engine design** is lane D's starting point, cut to what
  one listener on one phone needs: a run sheet, a buffer of the next few
  segments, and a response to an interruption. Scoring on stated
  aspiration and learned preferences waits for the second pass.
- **X is the only server piece with no existing mechanism.** Keep it
  small: a device registry and a sender. What to notify is already decided
  by the changelog.

## 5. What it needs from the Autopaper mission

| Autopaper item | Why Autoradio needs it |
|---|---|
| 0.1 revise with sources, lane C1 | the phone writes only through revise |
| 0.2 publish and adopt contract | a Choir source renders from the Autopaper object graph |
| C2 op-eds, forks | the reader's spoken response becomes their fork or op-ed |
| B sections | the reader's section list decides what plays |
| D prompt bar | the one input route that voice feeds |
| E changelog, H1 ledger records | the events that become notifications |
| F asking | "what about X" by voice |
| G style | themes are Textures, read as data by the native renderer |
| Q article quality | the paper is read aloud; a 60-word story or a French one is worse by ear |

Not delivered by that mission and needed by a store app: subscribing to
another user's edition, and account-level trust and safety (its section 5).
Version 1 plays the reader's own edition and the paper's.

## 6. Out of scope

- Android, CarPlay, the watch. iOS first; the order after it is open.
- YouTube, Sign in with Google, e-books and files read aloud. The launch
  checklist's LD-01 lists them; section 7 of the Autopaper handoff drops
  YouTube, and the rest wait for a later version.
- Block editing on the phone, Mail, any other module.
- Over-the-air updates of any kind.
- A server-side voice for all listening. The AI voice is on the device.

## 7. Done for pass 1

Proved on a real iPhone from a TestFlight build, signed in as a staging
identity that is not the owner's, against `choir.news`.

1. The reader signs in with a passkey, and the device appears by name in
   their sessions and can be revoked there.
2. The app plays the reader's edition aloud: a story in the AI voice, then
   a podcast clip, without a tap between them.
3. Playback continues with the screen locked for 30 minutes, with working
   lock-screen controls.
4. The reader interrupts by voice while audio plays, asks about the story,
   and hears an answer; the request is on the tape as a prompt bar
   submission.
5. The reader comments by voice and gets their own fork, shown natively.
6. A notification from a changelog event opens the right native screen.
7. The same golden documents render in the web and native renderers.
8. The reader deletes their account from the app.
9. No recognition or speech audio leaves the phone (shown by a network
   capture during a session).
10. The app is submitted for review with notes that describe what it does.

Approval is the benchmark for the mission as a whole, not for pass 1.

## 8. Open, with a default

| Question | Default if nobody rules |
|---|---|
| Open mic or push-to-talk at launch | push-to-talk; open mic when echo cancellation is proved against podcast audio |
| Which speech models ship, and the download size | whatever Mike runs today; measure the size in lane V |
| Where the page format spec lives | beside `internal/texturedoc`, with the golden documents |
| Sign in with Apple | not offered; App Review requires it only beside another third-party sign-in, and passkeys are first-party |
| Whether Exact needs our pieces merged upstream first | carry them as Swift modules of ours on the pinned commit and offer them upstream in parallel |
| The thin-client review risk (4.2.7(e)) | the app does voice, playback and rendering on the device; say so in the review notes |
| Where podcast and voice costs land | voice is on the device and free; podcast transcripts are a server cost, measured in lane D before pricing is fixed |
| Platform order after iOS, CarPlay | decide after approval |
| The app as the second factor for sensitive acts (owner, 2026-11-09; [note](second-factor-for-sensitive-acts-2026-11-09.md)) | not in pass 1; lane S1's named device is what it builds on. Authenticator-style codes can ship server-side first, and a signed approval from the app follows once push exists |
