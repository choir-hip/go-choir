# Autoradio Mobile App: Research, Scope and Plan

Date: 2026-10-14 (evening). Status: research and planning only. No product
code was written. Mutation class of this note: **green** (docs only). Most
of what it proposes is orange (a new client, new product APIs), and some is
red (auth and session issuance for a mobile client, push delivery,
self-development approvals from a phone, the Texture schema for media).
Each item is classed where it appears (§9).

Evidence labels used throughout: **code** (read in this repository at
6415505e3, with a path), **doc** (a dated Choir document), **web** (a
vendor or third-party page, cited by URL, fetched 2026-10-14 by a research
sub-agent), **UNVERIFIED** (could not be confirmed from a primary source;
treat it as a hypothesis to test on the device), **inference** (reasoning,
not tested).

## The brief

The owner, 2026-10-14 at about 21:00 UTC:

> Think about accelerating building the Autoradio mobile app, which
> includes Autopaper. We can use local on-device voice models. Apple
> includes their own models, but I think we should use Parakeet instead.
> [...] The app should include Autopaper and therefore necessarily
> Texture, including not just viewing published Textures, but the whole
> Texture interface of writing and editing and using Texture as a control
> pane and supervision surface for the whole multi-agent system, in
> addition to the Autoradio prototype. And this is also a way to
> accelerate the development of all the integrations into Texture, that
> is embedding multimedia, as many mini apps inside it as possible, not
> all immediately: embedded PDF and ebook reader, video, audio, images,
> slideshows. Hold off on the spreadsheet; no calendar (phones have one);
> email later. Don't build anything. [...] It would be lovely if we can
> really pull forward the ability to use that as supervision via Texture
> of ongoing self-development. I don't have an Apple developer account
> right now, so we can only run it locally on my device; when the app is
> nice I'll get an account, set up notifications, and publish on the App
> Store. Then an Android app as well.

Owner corrections, about 21:10 UTC:

1. **Not a shell around the Svelte web frontend.** "We can't ship the web
   desktop as a mobile app — it's against App Store rules. And it's just
   not cool. It should be a purpose-built thing." Svelte may be used in
   limited ways (a specific embedded component); the app is native and
   purpose-built, not the Choir desktop rebuilt.
2. **Swift vs Objective-C, seriously.** "Swift was the first language I
   used and I built Swift apps before, and I think it kind of stinks. For
   a lot of stuff, Objective-C is still better — much less resource
   utilization. But we want it native and current with all of Apple's
   latest features — Liquid Glass, etc."
3. **The foldable iPhone** ("a new foldable iPhone with many different
   layout configurations"): verify it and plan for its configurations.

Owner scope ruling, about 21:20 UTC: **Autoradio is not a super app.**

- "I don't want to support every app that the Choir desktop will
  support."
- App agents such as an automatic video editor (and CAD later) ship as
  their own mobile apps. Each may be bundled with a related consumption
  experience, and each connects to the user's Choir computer.
- Autoradio includes Autopaper and everything that goes with Texture,
  extended to document types: PDF, ebooks, images, embedded audio and
  video, and slideshows. A slideshow is a kind of document.
- Plan for a family of focused apps.

Constraints carried in from the day's rulings: foundations first ("deepen
foundations, don't accelerate growth", `docs/reports/strategy-takeaways-2026-10-14.md`);
a prototype is welcome if framed as one and not "setting architecture that
will be annoying to refactor" (`docs/reports/owner-roadmap-rulings-2026-10-14.md`,
ruling 5); the voice principle: "I want my AI to be clearly sounding like
AI, and then we're going to be mixing in human voices"
(`docs/reports/pricing-and-economics-notes-2026-10-14.md`); and the
app-store constraint: no web desktop and no app store inside the app
(`docs/reports/choir-state-and-vision-2026-10-14.md` §2.10).

## Bottom line

1. **What the app is.** Autoradio is a native iPhone app with four jobs:
   **listen** (a programme from Autopaper, podcasts and, later, your own
   computer), **talk** (interrupt at any moment; a take is recorded,
   transcribed on the device, and becomes a draft, a team note or an
   Opinion), **read** (Autopaper and published Textures), and **supervise**
   (Texture as the control pane of your computer: what each desk is doing,
   what it committed to, what awaits your approval). It is not the web
   desktop, has no window manager, installs no apps and runs no
   downloaded code.
2. **The supervision surface is the thing to pull forward,** and it fits
   the radio better than expected: the computer's own ledger (open
   commitments, frozen capsules awaiting approval, settled scores) can be
   *read aloud* as a programme, and a take can answer it. "Asynchronous
   meetings that drive the multi-agent system" (state and vision §2.11)
   is that programme, with approvals as one tap plus Face ID.
3. **Architecture: fully native, UIKit-first in Swift, SwiftUI where Apple
   makes it the only or best door** (widgets, Live Activities, App
   Intents, Liquid Glass components), and no general WebView. The case is
   in §3.

   - **Swift, not Objective-C.** Objective-C cannot reach several APIs
     this plan uses (SpeechAnalyzer, App Intents, Live Activities,
     widgets), and no measured runtime advantage was found. The owner's
     real costs, build times and SwiftUI's unpredictability, are handled
     by a UIKit spine and by build-time budgets (§3.3).
   - **The Texture editor is rebuilt natively** on TextKit 2 over the
     existing `choir.texture_doc.v1` schema, which is small enough to make
     that tractable (§3.4).
   - **The iPhone Duo is real.** It was announced on 9 September and
     ships on 23 October with iOS 27.1. It has six poses and two
     displays, and resizes apps instead of letterboxing them. The plan
     therefore builds with the iOS 27.1 SDK, uses the scene life cycle,
     and designs every screen in compact and regular widths from M0
     (§3.6).
   - **Autoradio is one app in a family, not a super app.** Video, CAD
     and other app agents ship as sibling apps. All of them share a
     ChoirKit set of packages (auth, API client, TextureKit, viewers,
     speech, notifications, design), and they use the user's computer as
     their shared state (§3.8).
4. **Speech to text: NVIDIA Parakeet TDT 0.6B v3 through FluidAudio's Core
   ML build** (Apache-2.0 Swift package; weights CC-BY-4.0; about 480 MB,
   downloaded on first use; 25 European languages). Add the 120M Parakeet
   end-of-utterance model for live barge-in later. Apple's SpeechAnalyzer
   is the zero-download fallback while the model downloads (§4.1).
5. **Text to speech: start with `AVSpeechSynthesizer` and a deliberately
   machine-sounding voice; move to Kokoro-82M (Apache-2.0) with an
   espeak-free phonemizer** and a fixed, stylised voice with an audible
   "AI" treatment. Avoid Piper (its engine is now GPL-3.0) (§4.2).
6. **Media mini-apps are native viewers behind Texture embeds:** PDFKit,
   AVKit, a native image viewer with Live Text, Readium for EPUB, and
   QuickLook as the fallback. Order: audio and images, then PDF, video,
   EPUB, slides (§5). They need the Texture `media_ref` block (a red
   schema change) before they can be authored, but they can render
   existing `source_ref` targets (image, audio, video, pdf) on day one.
7. **No paid Apple account blocks less than feared.** Free provisioning
   allows background audio, the microphone, Core ML, App Groups and
   Keychain sharing. It blocks push notifications, associated domains (so
   native passkeys and universal links), iCloud and TestFlight, and every
   install expires after 7 days. Sign-in works by reusing the desktop
   app's existing hand-off (`/auth/desktop/exchange-redirect` →
   `choir://auth-complete` → `/auth/desktop/redeem`) through
   `ASWebAuthenticationSession`, where passkeys work without an
   entitlement (§7).
8. **The biggest App Store risk is guideline 4.2.7(e): "Thin clients for
   cloud-based apps are not appropriate."** A native app with its own
   audio, speech and reading, using the product API like any client, is
   not a thin client; a screen that mirrors the web desktop would be.
   That is a second, independent reason for the owner's "purpose-built"
   ruling (§7.4).
9. **The backend gaps are real and mostly foundations work:** a mobile
   session (a refresh token in the Keychain, not cookies), a host-side
   notification relay so a phone never keeps a computer awake, a
   supervision feed (one cursor over approvals, obligations, commitments
   and runs), a take upload and capture kind, a stable versioned product
   API, and a sync API with change anchors for the File Provider (§8).
10. **Plan:** M0 a local prototype on the owner's iPhone (listen, read,
    sign in, record; about 2 to 3 weeks of agent-assisted work), M1
    Texture supervision (read, approve, direct), M2 Parakeet takes into
    Texture, M3 media embeds, M4 the sync folder, then the paid account,
    push, TestFlight and the App Store, then Android (§10). The first
    thing to build is not the app: it is the two half-day spikes that
    decide the architecture's riskiest assumptions (Parakeet speed on the
    owner's phone; passkey sign-in in `ASWebAuthenticationSession` under
    a free team), and the supervision-feed API contract.

---

## 1. Product

### 1.1 What Autoradio is, and what it is not

| It is | It is not |
|---|---|
| A purpose-built native app for listening, talking, reading and supervising | The Choir web desktop in a wrapper |
| A client of the user's persistent computer through the product API, like the CLI | A remote-desktop or screen-streaming client |
| Autopaper (front page, stories, sections, Opinions) and published Textures | A general browser or an app store; no third-party mini apps, no downloaded code |
| Texture: reading, writing, editing, and the supervision surface | A spreadsheet, calendar or mail client (owner: hold, no, later) |
| A recorder of takes that become captures, drafts, team notes, Opinions | A social network of its own (takes route through the computer and the Autopaper object graph) |
| Native viewers for the document types Texture embeds: PDF, EPUB, images, audio and video, and slideshows (a slideshow is a kind of document) | A super app. App agents such as a video editor or CAD ship as **sibling apps** in a family, not inside Autoradio (owner, about 21:20 UTC; §3.8) |

The architecture claim it inherits: every surface is a projection of the
same tape (state and vision §2.1). A take on the phone, an edit in a
desktop window and an API call from an outside agent all land as typed
events on the user's computer. The phone holds caches and drafts, never
authority: **Texture stays the sole writer of canonical revisions**, and
the phone submits revisions and requests exactly as the desktop does.

### 1.2 The five core loops

```text
 (1) LISTEN                         (2) TALK (interrupt)
 programme = Autopaper front         hold Talk -> programme ducks/pauses
   + publisher podcast clips         -> record take (on device)
   + your computer's ledger (M1+)    -> Parakeet transcribes (on device)
   + team objects (later)            -> resume at the interrupted sentence
        |                                     |
        v                                     v
 (3) TAKE -> where does it go?  -------------------------------
     keep (on device) | share with team | publish as Opinion |
     direct my computer ("tell engineering to ...")
        |                    |                 |
        v                    v                 v
  capture on your      team object        Texture draft -> publish
  computer (voice      (adopted by team   (Opinion section, Autopaper
  provenance)          computers)         object graph, cites story@version)
        \___________________ | __________________/
                             v
 (5) SUPERVISE via Texture  <---  (4) READ
  root Texture: all work in       Autopaper front page, stories at
  the computer; per-task          pinned versions, published Textures,
  Textures; approvals; open       embedded media (PDF, EPUB, video ...)
  commitments; settled scores
```

- **Listen** exists as a web prototype (`prototype/autoradio-2026-10-14`,
  `docs/reports/autoradio-prototype-2026-10-14.md`): the front page in
  rank order, one sentence at a time in a machine voice, related clips
  from publishers' own enclosure URLs, and "new since you last listened"
  by story version. The native app ports its pure logic
  (`frontend/src/lib/autoradio.ts`, 524 lines on the branch) to Swift and
  replaces Web Speech, `<audio>` and MediaRecorder with native audio.
- **Talk** is push-to-talk first (hold, or tap-tap), with barge-in by
  voice later (§4.1 live model). A take carries the segment it
  interrupted (publication id and version, or the ledger record id).
- **Take** routing needs foundations that do not exist yet: contributions
  and accounts (LC, LA), the Opinion section (LC), and the precommitment
  loop (L3) for "direct my computer". M2 ships the parts that are local
  or already have an API (keep, transcribe, draft a Texture document);
  publish and share follow LC (§10).
- **Read** is Autopaper (public, works signed out, served from Store B
  even when the user's computer is stopped; code:
  `internal/proxy/handlers.go:832-847`) and published Textures.
- **Supervise** is §1.4.

### 1.3 Screens and navigation

Four tabs and two persistent controls. The tab bar adopts Liquid Glass
by building with the iOS 27.1 SDK (§3.3, §3.6); nothing below depends on custom
chrome.

| Tab | Primary screen | Key secondary screens |
|---|---|---|
| **Listen** | Now playing: current segment, AI/Human tag, rundown, Talk | Programme picker (Today's paper, My computer, a team), voice and clip settings, take review sheet |
| **Paper** | Autopaper front page, sections | Story (published Texture at a pinned version), Opinion composer (later), source and media viewers |
| **Texture** | Supervision home: the root Texture of the user's computer, with the "needs you" strip at the top | A Texture document (read and edit), approval sheet, run/trajectory timeline, commitment detail, prompt bar |
| **Library** | Takes (local and sent), downloads, the sync folder's recent files | Take detail (audio, transcript, destinations), media viewers |

- **Mini-player** above the tab bar on every tab (the programme keeps
  playing while you read or supervise).
- **Talk** is a large control on Listen, a button in the mini-player
  elsewhere, an App Intent ("Talk to Autoradio"), the Action button on
  phones that have one (via App Intents), and a Control Center control
  (iOS 18+ controls are App Intent based; web: Apple's controls
  documentation was not fetched, UNVERIFIED in detail).
- **Account** (sign-in, devices, sessions, account deletion, licences and
  model attributions) lives behind the avatar, not a tab.
- **Widgets and Live Activities** (SwiftUI-only, §3.3): "now playing",
  "needs you: 2 approvals", and a Live Activity while a self-development
  operation is running. Live Activities can be started locally without
  push on a free team (inference: Apple's capability table shows no gate
  for them); remote updates need APNs (paid).

On a wide or unfolded display (§3.6), Listen and Texture become two-pane:
rundown beside the player; supervision list beside the open document.

### 1.4 The supervision use case: Texture as the control pane

The owner's request: "pull forward the ability to use that as supervision
via Texture of ongoing self-development." The pieces that exist today:

| Need | What exists (code) | Gap |
|---|---|---|
| See what the computer is doing | Runs `/api/runs`, trajectories `/api/trajectories/{id}` with `/stream?after=` (SSE), runtime obligations `/api/runtime/obligations`, the desktop live socket `/api/ws?after_seq=` | No single "what needs me" view; the phone must not hold a socket open (wakes and keeps the computer awake, §8.3) |
| Approve or reject self-development | `/api/computers/{id}/self-development/operations?state=awaiting_approval`, `POST .../operations/{op}/decision` (CLI: `cmd/choir/selfdev.go`); API-key scopes `computer:self_development:approve|propose|rollback` (`SettingsApp.svelte`) | A mobile approval sheet showing what the capsule changes, its verifier evidence and the effect bundle id; a step-up (Face ID) before the decision |
| Direct the computer | Texture prompt bar `/api/prompt-bar`, Texture revise `/api/texture/documents/{id}/revise` | Voice directive → a typed directive record; today the prompt bar takes text |
| Open commitments and their scores | Commitment records (`internal/types/commitment.go`: precommit, report, resolve, disagreement, directive), inbox records `/api/inboxes/{inbox}/records`, derived views incl. "a supervision projection ordered by materiality" (`docs/precommitment-records.md`) | Nothing settles or scores yet (L3); box scores are design only. The phone should render the supervision projection the moment it exists, not invent its own |
| Read the work | Texture documents, revisions, the `/stream` SSE per document | A native Texture renderer (§3.4) |

**The supervision home** (Texture tab):

```text
+------------------------------------------------------+
| NEEDS YOU (2)                                         |
|  [capsule] engineering: "LU2 drain call" frozen,      |
|            verifiers 14/14, effect a9c3..  [Review]   |
|  [draft]   Texture: Opinion draft from your take       |
+------------------------------------------------------+
| RUNNING                                               |
|  research  Taiwan export story v4   12 min  (live)    |
|  engineering capsule build          3 min   (live)    |
+------------------------------------------------------+
| COMMITTED (open precommitments, by materiality)       |
|  engineering: "tests pass after LU2" p=0.80, due today|
|  research: "TSMC guidance holds" p=0.65, due Oct 21   |
+------------------------------------------------------+
| SETTLED THIS WEEK (box score, when L3 lands)          |
|  engineering 9/11 (Brier 0.14) | research 6/9 (0.21)  |
+------------------------------------------------------+
| ROOT TEXTURE  (the computer's own document) [Open]    |
+------------------------------------------------------+
```

The **approval sheet** shows: the request (who asked, from which
Texture), the conjecture (what the desk predicted, with probability),
what changed (files, a diff summary, the capsule's effect bundle id),
verifier results, the rollback ref, and two actions (Approve, Reject with
reason). Approve requires Face ID (LocalAuthentication) and sends the
decision with an idempotency key. The owner is "the constitutional root
of intention, not a per-action approver" (`docs/choir-vision.md`), so
the phone also exposes the policy (`self-dev mode set`, e.g.
`propose_only`) rather than encouraging approve-everything.

**Supervision as radio.** The novel part. A "My computer" programme is
built from the same feed: "Engineering froze the drain-call capsule; all
fourteen verifiers passed; it predicted the tests pass with probability
0.8; it needs your approval." The listener says "approve" (a take
interpreted as a directive, confirmed on screen with Face ID; never a
voice-only approval), or "hold it, ask why the drain took four rounds"
(a take routed as a directive record to management). This is the
asynchronous meeting of state and vision §2.11, with the precommitment
records as the agenda. It mixes private data into the programme, which
the prototype deliberately did not do; on the user's own phone, for the
user's own computer, it is the owner's own data, and it must never be
mixed into anything shared (state and vision §9 risk: aggregate box
scores never computed over private data).

**Notifications** (APNs needs the paid account, §7; until then, local
notifications fire only while the app runs or on a background refresh):

| Event | Notification | Payload rule |
|---|---|---|
| Self-development operation reaches `awaiting_approval` | "Engineering needs your approval" (time sensitive) | ids only; content fetched on open |
| A run or trajectory you started finishes or fails | "Taiwan story v4 published" | ids only |
| A story you heard got a new version | quiet, batched | publication id and version |
| A commitment settles (L3) | daily digest | none |
| A take finished transcribing / upload failed | local notification | local |

No private content in push payloads; a Notification Service Extension
can fetch and decrypt a summary later if the owner wants richer lock-screen
text.

### 1.5 Native or web, per surface

The owner's ruling makes the default native. The table records where a
web component is still allowed ("Svelte may be used in limited ways")
and why it is not chosen first.

| Surface | Choice | Why |
|---|---|---|
| Listen, Talk, takes | Native | Background audio, interruptions, recording, Core ML: native only |
| Autopaper front and story | Native renderer of the published `choir.texture_doc.v1` | Reading quality, offline cache, accessibility, 4.2 "app-like" |
| Texture read and edit | Native (TextKit 2) | §3.4; the web editor has iOS-specific gaps |
| Supervision home, approvals | Native | Face ID step-up, widgets, Live Activities |
| PDF, EPUB, video, audio, images, slides | Native viewers (§5) | Memory, gestures, PiP, Live Text |
| YouTube embeds | The official IFrame player in a small, single-purpose `WKWebView` | No native YouTube player exists; the terms require their embed (web, YouTube RMF) |
| Charts, maps in stories (later) | Native (Swift Charts, MapKit) or pre-rendered images from the server | Avoid a JS runtime |
| An exotic embed that has no native viewer yet | A sandboxed `WKWebView` for that one component, with bundled assets only | The "limited ways" exception; never remote code |

---

## 2. Where the code starts from

| Asset | Where | Reuse on mobile |
|---|---|---|
| Autoradio web prototype | branch `prototype/autoradio-2026-10-14`: `autoradio.ts` (pure logic), `AutoradioApp.svelte`, 9 Playwright tests | Port the logic and the tests' cases to Swift; the UI is redone natively |
| Autopaper reader | `AutopaperRoute.svelte`, `AutopaperStory.svelte`, `autopaper.ts` (`rankFrontStories`, `layoutFrontPage`) | Port ranking and layout rules; public API unchanged |
| Texture editor | `TextureEditor.svelte` (2612 lines), a custom `contenteditable` serialized to `choir.texture_doc.v1` by `texture-structured-editor-doc.ts` | Port the schema and the serializer's rules, not the DOM code (§3.4) |
| Texture API | `frontend/src/lib/texture.js`: documents, revisions, revise, manifest, `/stream` SSE | Same endpoints from Swift |
| Media desktop apps | `PdfApp.svelte` (pdfjs-dist), `EpubApp.svelte` (custom JSZip parser), `SlidesApp.svelte`, `VideoApp.svelte`, `ImageApp.svelte`, `AudioApp.svelte`, `PodcastApp.svelte` | Behaviour reference only; native viewers replace them |
| Media progress | `/api/media/progress`, `/api/media/recents` | Same API: resume a PDF or episode across phone and desktop |
| Source rendering | `texture-source-renderer.ts` renders `<audio>`, `<object type="application/pdf">`, image and video for source targets; allowed target kinds in `internal/texturedoc/schema.go:404` | The native renderer maps the same target kinds to native viewers |
| Desktop passkey bridge | `cmd/desktop/webauthn_session_darwin.go` (ASWebAuthenticationSession), `desktop_auth.go` (one-time code redeem, cookie jar kept from JS) | The same flow on iOS (§8.1) |
| CLI | `cmd/choir`: self-dev list/wait/approve/reject, files, trace, Texture events | The mobile client is "the CLI with a face": same API, same scopes |
| Prior File Provider | A macOS File Provider extension and desktop SyncEngine existed 2026-06-28 and were deleted 2026-07-10 as superseded (`7b59d33e8`, pre-delete rollback ref `a1073731`) | Read before designing the iOS one (§6); do not resurrect without the sync API |

**Findings from reading the editor (code; behaviour on iOS not tested).**
`TextureEditor.svelte` lets the browser do the editing and re-serializes
the whole surface on every `input` event (`handleEditorInput`, line 1538).
There is no `compositionstart`, `beforeinput` or `isComposing` handling
anywhere in `frontend/src/lib`. On iOS, autocorrect, predictive text,
dictation and CJK input all run through composition; serializing
mid-composition is a known source of lost or doubled characters in
contenteditable editors (web: ProseMirror forum, "isComposing gets out of
sync on iOS", https://discuss.prosemirror.net/t/iscomposing-gets-out-of-sync-on-ios/4067).
Also, `syncEditorSurface` skips re-rendering while the surface has focus,
so an agent revision arriving during a human edit is held until blur.
Both are hypotheses for the desktop editor's own backlog, and both are
reasons the phone gets a native editor.

---

## 3. Architecture

### 3.1 The decision after the owner's correction

The original question (how to wrap the Svelte UI) is closed by the
owner's ruling, and the research independently supports it:

- App Review guideline 4.2: "Your app should include features, content,
  and UI that elevate it beyond a repackaged website" (web:
  https://developer.apple.com/app-store/review/guidelines/, "Last
  Updated: June 8, 2026").
- 4.2.7(e): "Thin clients for cloud-based apps are not appropriate for
  the App Store" (same page).
- 2.5.2: apps may not "download, install, or execute code which
  introduces or changes features or functionality of the app". A WebView
  that loads the computer-surface frontend from the user's computer
  (which self-development can change, `docs/computer-ontology.md`) is
  exactly that.
- Practical: passkeys in a `WKWebView` work only for the app's associated
  domain (Apple DTS, web: https://developer.apple.com/forums/thread/723273),
  which a free team cannot have; WKWebView keyboard and viewport bugs on
  iOS 26 are open (WebKit bugs 191185, 292603; web:
  https://bugs.webkit.org/show_bug.cgi?id=292603); and the existing
  editor has no composition handling (§2).

So the question becomes: **which native stack**, and **how much shared
code with Android**.

### 3.2 Options compared

| Option | Native feel and latest Apple features | Reuse | Android path | Verdict |
|---|---|---|---|---|
| **UIKit + SwiftUI in Swift** (Apple's current stack) | Full, first on every new API | Logic ported, UI new | Separate Kotlin/Compose app, or Skip (below) | **Recommended** (§3.3) |
| **Objective-C + UIKit core, Swift islands** | Full for UIKit-era APIs; Swift-only APIs need Swift shims | Same | Separate | Viable; worse fit (§3.3) |
| Capacitor 8 (WKWebView + plugins) | Web UI; native only in plugins | Svelte UI | Same code | Ruled out by the owner; 4.2 / 4.2.7 risk (web: https://ionic.io/blog/announcing-capacitor-8) |
| React Native / Expo | Native widgets, JS runtime | Rewrite UI in React | Same code | Rewrite plus a JS runtime; no gain over native |
| Tauri 2 mobile | WebView UI, Rust core | Svelte UI | Same code | Ruled out; 47 open iOS-labelled issues at fetch time (web: https://github.com/tauri-apps/tauri/issues) |
| Flutter | Own renderer; Liquid Glass only by imitation | Rewrite UI in Dart | Same code | Not "current with Apple's latest features" by construction |
| Skip (SwiftUI → Compose) | SwiftUI on iOS | Swift logic shared | Generated Compose app | An Android option to evaluate at M5, not a reason to choose SwiftUI everywhere (web: https://skip.dev/docs/; free and open source since v1.7, https://alternativeto.net/news/2026/1/swift-cross-platform-framework-skip-is-now-free-and-open-source-with-licensing-removed) |

### 3.3 Swift or Objective-C

The owner's view, stated fairly: Swift "kind of stinks", Objective-C uses
fewer resources, and the app must still be current with Apple's latest
features. The evidence on each half:

**(a) Which current Apple APIs can Objective-C reach?** This was checked
against Apple's documentation data, which lists the language variants of
each symbol (web: `developer.apple.com/tutorials/data/documentation/...`).
"Swift only" means Apple lists no Objective-C variant.

| API Autoradio wants | From Objective-C? | Source |
|---|---|---|
| Liquid Glass in UIKit (`UIGlassEffect`, `UIGlassContainerEffect`, iOS 26+) | **Yes**, with Objective-C declarations | https://developer.apple.com/tutorials/data/documentation/uikit/uiglasseffect.json |
| Liquid Glass in SwiftUI (`.glassEffect`, `GlassEffectContainer`) | No (SwiftUI) | https://developer.apple.com/tutorials/data/documentation/swiftui/view/glasseffect(_:in:).json |
| Standard bars and controls adopt Liquid Glass on rebuild | Yes, in either language. Source: third-party guides. An Apple engineer says the `UIDesignRequiresCompatibility` opt-out "will no longer be honored" once an app is built against iOS 27 | https://developer.apple.com/forums/thread/832710 |
| **SpeechAnalyzer / SpeechTranscriber** (the STT fallback) | **No**. It is an actor whose results arrive as an `AsyncSequence` | https://developer.apple.com/tutorials/data/documentation/speech/speechanalyzer.json |
| **App Intents** (the Talk shortcut, the Action button, controls) | **No** | https://developer.apple.com/tutorials/data/documentation/appintents.json |
| **ActivityKit** (Live Activities for running operations) | **No** | https://developer.apple.com/tutorials/data/documentation/activitykit.json |
| WidgetKit | The UI is SwiftUI only; Objective-C sees three constants | https://developer.apple.com/tutorials/data/documentation/widgetkit.json |
| Foundation Models (the on-device LLM, e.g. for take summaries) | **No** (macros, async) | https://developer.apple.com/tutorials/data/documentation/foundationmodels.json |
| SwiftData, Observation | No | Apple documentation data |
| iPhone Duo APIs (§3.6) | The SwiftUI forms: no. Whether Objective-C can see the UIKit forms is UNVERIFIED | Apple Tech Talks 111463, 111464 |
| Core ML, AVAudioEngine, ASAuthorization passkeys, File Provider, PDFKit, CarPlay, TextKit 2 | Yes | Apple documentation data |
| FluidAudio (Parakeet), Readium, Kokoro ports | Swift packages. Objective-C can use them only through `@objc` wrappers you write | package sources |

Apple DTS says "Many new Apple frameworks are Swift only"
(https://developer.apple.com/forums/thread/95935). Apple has announced no
deprecation of Objective-C, and every framework the app needs for audio,
files, PDFs and passkeys still works from it.

But **four features this plan depends on are Swift-only:
SpeechAnalyzer, App Intents, Live Activities and widgets. Both chosen
speech libraries are Swift packages too.** Reaching them from an
Objective-C core means writing completion-handler shims, and those cannot
cleanly expose `AsyncSequence` streams, actors or macros. That is not a
small tax, because it falls exactly on the novel parts of the app.

**(b) Is Objective-C measurably lighter?** The research found no
independent, dated benchmark of the same app built in both languages.
The common claims ("Swift is 2.6x faster", "Objective-C is lighter")
trace back to Apple's 2014 marketing and to anecdote, with no method
given (web, for example: https://www.netguru.com/blog/objective-c-vs-swift).
What has been measured:

| Claim | What the evidence says |
|---|---|
| Swift binaries are bigger | **True at scale, and manageable.** In Uber's mostly-Swift Rider app, generics, closures and `try` inflated the code. Optimization took it from 145.7 MB to 120.1 MB across about 500 modules (web, 2021: https://www.uber.com/en-US/blog/how-uber-deals-with-large-ios-app-size). The claim that the Objective-C app was a third of the size is engineers' recollection, not Uber's measurement (https://blog.pragmaticengineer.com/uber-app-rewrite-yolo/) |
| Swift launches slower | Uber's 8 to 12 seconds before `main` came from 92 dynamic libraries, which is a linker problem, not the language. Apple measured Swift protocol-conformance lookup at up to half of launch time on real apps, and has precomputed it since iOS 16 (web: https://developer.apple.com/videos/play/wwdc2022-110363) |
| Swift carries a runtime | Not since ABI stability. The runtime ships in the OS "like the Objective-C runtime" (web: https://www.swift.org/blog/abi-stability-and-more/) |
| Swift builds slowly | **True, and the owner's strongest point.** Single expressions can stall the type checker: one 12-line expression took 42 s (https://danielchasehooper.com/posts/why-swift-is-slow/). Developers also report incremental builds getting slower after Xcode 16.1 (https://forums.swift.org/t/swift-6-xcode-16-1-slower-build-times/75707). These are anecdotes rather than a controlled study, but they agree |
| SwiftUI is slow | It was, for large lists. Apple claims large gains in iOS 26 (from third-party summaries of WWDC25 session 256; Michael Tsai questions the comparison: https://mjtsai.com/blog/?p=48138). For an app built from text and controls, UIKit remains the predictable choice |

My reading (an inference, not tested): the resource cost the owner felt
is real, but it shows up in **build times** and in **SwiftUI's
unpredictability**. It does not show up in the shipped app's speed or
memory. Both can be managed by how Swift is used.

**(c) The options.**

| Option | Latest Apple features | Control and resources | Verdict |
|---|---|---|---|
| A. Objective-C and UIKit core, with Swift islands (`-Swift.h`, `@objc`) | Every Swift-only API sits behind a hand-written shim | Fast builds for the core, but two languages, and shims on the novel parts | Viable, but it taxes exactly the features that make the app new |
| **B. UIKit written in Swift, with SwiftUI only where Apple requires it or it is clearly cheaper** | All, directly | UIKit gives explicit control over text, audio UI and lists, and the Swift-only APIs are used natively | **Recommended** |
| C. SwiftUI first | All | Least code, but least control over text editing and scrolling; TextKit 2 still needs `UIViewRepresentable` | Not for the editor. Fine for settings, widgets and sheets |

**Recommendation: B.** Write the whole app in Swift.

- **UIKit is the spine:** scenes, navigation, the TextKit 2 editor and
  the player.
- **SwiftUI is used where Apple requires it or it is clearly cheaper:**
  widgets and Live Activities (required), App Intents snippets,
  settings, and simple sheets hosted in `UIHostingController`.
- **Liquid Glass** comes from UIKit's own components when the app is
  built with the current SDK, plus `UIGlassEffect` wherever the app
  draws custom chrome.

These rules answer the owner's objections directly:

- **Use Swift like a systems language, not as a showcase.** Few generics
  in hot paths, final classes and structs, no deep stacks of protocols,
  and explicit types in long expressions, which keeps the type checker
  fast.
- **Keep modules few and static.** This avoids Uber's launch cost from
  dynamic libraries. Turn on Swift 6 strict concurrency, which catches
  the audio-thread races this app is prone to.
- **Give CI a build-time budget.** Set `-warn-long-expression-type-checking`
  and `-warn-long-function-bodies` to 100 ms, and fail the build when a
  clean build exceeds a set limit (yellow).
- **Measure, then decide.** M0 records launch time, memory during
  playback and during transcription, and binary size on the owner's
  phone. If a hot path proves heavy, it can drop to C or Objective-C
  behind a small interface; nothing in option B forbids that.

### 3.4 Texture on the phone: a native editor over the existing schema

The decisive fact (code): the structured Texture document is small.
`texture-structured-editor-doc.ts` serializes to schema
`choir.texture_doc.v1` with these node types: paragraph, heading, list
(ordered and unordered) and list item, blockquote, code_block,
horizontal_rule, hard_break, text (with marks), and `source_ref` (a
citation chip, or an `expanded_ref` embed block). The media plan adds one
block, `media_ref` (`docs/research/autopaper-media-and-rights-2026-10-14/README.md`
§5.2). That is a document model a native editor can own completely.

```text
            choir.texture_doc.v1 (JSON, from /api/texture/...)
                              |
               +--------------+---------------+
               |                              |
      TextureDocModel (Swift)          golden fixtures shared with
      nodes, marks, ids, revision      the web editor's tests
               |                       (same JSON in, same JSON out)
      NSTextContentStorage / TextKit 2
      paragraphs -> NSTextParagraph; source_ref -> attachment chip;
      expanded_ref / media_ref -> NSTextAttachmentViewProvider block
               |
      UITextView (TextKit 2) editing: native selection, autocorrect,
      dictation, IME composition, undo, accessibility
               |
      on edit: diff the model -> a revision submission
      (POST /api/texture/documents/{id}/revisions), never a canonical
      write; agent revisions merge in by node id while editing
```

- **Reading first.** The published reader is the same renderer in
  read-only mode, and it is needed for Autopaper anyway. M0 ships the
  reader; M1 ships editing.
- **Embeds as attachment views.** TextKit 2's
  `NSTextAttachmentViewProvider` hosts a live view (a PDF thumbnail, an
  audio scrubber, a video poster) inline in the text; tapping it presents
  the full native viewer (§5). Expand and collapse is the
  `display_mode` the media plan proposes.
- **Concurrent edits.** Agents revise documents while the owner reads or
  edits. The native model keeps node ids, so an agent revision can merge
  around the owner's caret instead of being held until blur (the
  desktop's current behaviour, §2).
- **Parity guard.** One set of JSON fixtures (document in, edits, document
  out) is run by both the web editor's tests and the Swift model's tests,
  so the two editors cannot drift on the schema. This is the yellow-class
  work that keeps two editors honest.
- **The fallback.** If TextKit 2 editing proves worse than expected, the
  Svelte editor can run as one sandboxed, bundled component inside a
  single `WKWebView` (the owner's "limited ways" exception), with a narrow
  bridge (load doc, emit revision). It is the fallback, not the plan,
  because it carries the composition gap and the WebKit keyboard bugs
  with it.

**What the research says about native editing on iOS** (web, linked in
the table):

| Option | Fit | Pitfalls |
|---|---|---|
| **`UITextView` with TextKit 2** (`init(usingTextLayoutManager:)` and `NSTextAttachmentViewProvider`, both with Objective-C and Swift declarations) | **Best fit.** Citation chips and media blocks become real views | TextKit 2 has rough edges. STTextView's author reports layout jitter from estimated heights and extra-line-fragment bugs. Keith Blount calls it "not yet ready for editing documents" because of multi-container layouts and tables. Touching `.layoutManager` silently falls back to TextKit 1 and loses view attachments; that is reported on macOS and UNVERIFIED on iOS. All collected at https://mjtsai.com/blog/2025/08/15/textkit-2-the-promised-land/ |
| SwiftUI `TextEditor` with `AttributedString` (WWDC25 session 280) | Not enough. It handles inline styles and custom attributes, but not block structure or arbitrary attachments | https://developer.apple.com/videos/play/wwdc2025/280/ |
| STTextView | GPL-3.0 or a paid licence, and built for code | https://github.com/krzyzanowskim/STTextView |
| lexical-ios (Meta) | "Pre-release with no guarantee of support" | https://github.com/facebook/lexical-ios |
| Notion's approach | Its apps are native "save for the editor" (December 2024: https://newsletter.pragmaticengineer.com/p/notion-going-native-on-ios-and-android). A secondary report says a native iOS editor has been in beta since | |

Texture's schema avoids TextKit 2's weakest areas: it has no tables, no
multi-column layout, and only one text container.

Rules for the editor:

- Own the document tree (the schema above) and render it to an
  attributed string with custom attributes.
- Never touch `layoutManager`.
- Confine every TextKit workaround to one `UITextView` subclass.
- Prototype the hard cases on the device first: nested lists, dictation,
  keyboard avoidance, Writing Tools, and undo across attachments. That
  prototype is spike (c) in §10.

### 3.5 Inside the app: modules

```text
AutoradioApp (iOS 27.1 SDK; deployment target iOS 26, see 10.2)
|
|-- AppShell (UIKit): scenes, tab bar, mini-player, Talk control,
|                     window-size adaptation (3.6)
|-- Features
|   |-- Listen      programme builder (port of autoradio.ts), rundown
|   |-- Paper       Autopaper front, story reader
|   |-- Texture     doc model, TextKit 2 editor/reader, supervision home,
|   |               approval sheet, run timeline
|   |-- Library     takes, downloads, sync-folder recents
|   `-- Viewers     PDFKit, AVKit, image+Live Text, Readium, QuickLook
|-- Audio engine    AVAudioSession policy, AVAudioEngine graph,
|                   TTS stage (AVSpeech -> Kokoro), clip player,
|                   recorder, Now Playing, remote commands
|-- Speech          FluidAudio: Parakeet TDT v3 (takes), EOU 120M (live),
|                   Silero VAD; SpeechAnalyzer fallback; model manager
|-- Client          product API client, auth session (Keychain),
|                   supervision feed cursor, offline cache, outbox
|-- Extensions      Widgets + Live Activity (SwiftUI), App Intents,
|                   File Provider (M4), Notification Service (paid)
`-- Shared          TextureDoc schema + fixtures, API types (generated
                    from an OpenAPI description, 8.6)
```

- **Cache and outbox.** A local SQLite store (GRDB or SQLite directly;
  SwiftData is Swift-only and young, and the cache wants explicit
  migrations and queries) holds published stories, the user's recent
  Texture documents, the supervision feed and its cursor. An **outbox**
  holds writes made offline (a revision, a take, a directive), each with
  an idempotency key, replayed in order when online. The phone never
  decides a conflict: the computer accepts or rejects a revision against
  its base revision, as for the desktop.
- **No background resumption of work.** The app restarts nothing on the
  computer. It shows state; the computer's own rules decide what a crash
  ended (CLAUDE.md, "Restarts end work").

### 3.6 Screen sizes, and the foldable iPhone

**The device is real.** Apple announced the **iPhone Duo** on
9 September 2026, alongside the iPhone 18 Pro. Pre-orders open on
16 October and sales start on 23 October, with iOS 27.1 preinstalled
(web: MacRumors roundup, https://www.macrumors.com/roundup/iphone-duo/;
NPR, https://www.wknofm.org/2026-09-10/apple-announces-foldable-iphone-what-to-know).

The hardware figures come from press reports, because Apple's newsroom
page did not load for the research agent:

- an outer display of 5.4 inches (1398 x 2034);
- an inner display of 7.6 inches (1878 x 2670);
- the same aspect ratio on both, and 120 Hz on both.

The developer guidance is Apple's own:

- the article "Preparing your app for iPhone Duo"
  (https://developer.apple.com/documentation/technologyoverviews/preparing-your-app-for-iphone-duo);
- the developer hub, https://developer.apple.com/iphone-duo/;
- three Tech Talks: 111461 ("Prepare your app"), 111463 ("Strike a pose
  with adaptive layouts") and 111464 ("Leverage multiple displays and
  scenes").

**Configurations.** Apple's article lists six poses: closed, tent, open
wide, partially folded, open tall, and laptop. Apps can also show content
on **both displays at once** through scene accessories. Apple's example
is a camera on the inner display with a teleprompter on the outer (Tech
Talk 111464).

| Display and pose | Size class (horizontal x vertical, Tech Talk 111461) | Autoradio layout |
|---|---|---|
| Outer display, portrait (closed) | compact x regular | The phone layout: tabs, mini-player, Talk |
| Outer display, landscape | compact x compact | A player-only "radio" view with a large Talk control |
| Inner display, open | regular x regular | Two panes: rundown and player (Listen), supervision list and document (Texture), front page and story (Paper) |
| Partially folded, laptop | regular, with a `.division` reserved region at the fold | Content above the fold, Talk and the transport controls below it (a natural "radio" pose) |
| Tent (both displays) | a scene accessory on the outer display | The outer display shows now playing and the AI/Human tag to someone across the table; the inner display keeps the owner's controls. Later, not in M0 |

**What supporting it takes**, from Apple's guidance:

- **Lay out by size class and scene geometry**, never by orientation or
  `userInterfaceIdiom`. The inner display ignores supported orientations.
- **Stop using `UIScreen.main`**, which is to be deprecated. Use the
  window scene's screen and `effectiveGeometry`.
- **Use the scene-based life cycle.** Duo is the first iPhone with
  multiple scenes, and Split View is available to all apps on it.
  `UIRequiresFullScreen` no longer opts out of resizing on iOS 27
  (TN3192:
  https://developer.apple.com/documentation/technotes/tn3192-migrating-your-app-from-the-deprecated-uirequiresfullscreen-key).
  Apps that are not updated are resized, not letterboxed.
- **Ship an iOS 27 launch screen.** It is required for App Store
  submission (same technote).
- **New APIs**, each in a SwiftUI and a UIKit form:
  - reserved regions: `reservedRegions(kind:)`, with `.division` for the
    fold and `.occlusion` for the cameras;
  - arrangement containers: `ArrangementView` and
    `UIArrangementViewController`, in split and overlay styles (iOS 27.1);
  - hinge state: `onHingeChange` and `UIHingeInteraction` report closed,
    partially open or fully open, plus the angle. Apple says to use them
    for effects, not layout;
  - vertical bars: `toolbarVerticalEdge` and the `verticalBarEdge` trait.

**Swift or Objective-C, for the Duo.** Every Duo API has a UIKit form, so
the UIKit spine of §3.3 loses nothing. All of Apple's samples are in
Swift. Whether `UIHingeInteraction` and `UIView.reservedRegions(kind:)`
are visible to Objective-C is UNVERIFIED; check the Xcode 27.1 headers.
That is one more small point for Swift.

**What this changes in the plan:**

- Build with the **iOS 27.1 SDK** (Xcode 27.1, listed as a release
  candidate on Apple's Duo page) from the first line of code.
- Use the scene-based life cycle from day one.
- Design every screen in compact and regular widths in M0. The
  inner-display two-pane layouts are cheap if planned and costly if
  retrofitted.
- Add the fold-aware "laptop radio" pose and the tent pose's
  outer-display accessory in M3.
- Test on Apple's Duo simulator until a device is available.
- Android foldables map onto the same compact and regular designs, through
  Jetpack WindowManager's `FoldingFeature` postures and window size
  classes
  (https://developer.android.com/develop/ui/compose/layouts/adaptive/window-size-classes).

### 3.7 Android

Owner: "Then an Android app as well." Recommendation: **a separate,
native Kotlin + Jetpack Compose app after the iOS app reaches TestFlight,
sharing contracts and fixtures, not code.**

| What is shared | How |
|---|---|
| Product API types | Generated from one OpenAPI description (§8.6) for Swift and Kotlin |
| Texture schema and editor behaviour | The same JSON golden fixtures (§3.4) |
| Programme logic (ranking, matching, script) | Specified by the shared fixture cases from the prototype's tests; small enough to port |
| Media viewers | Readium Kotlin Toolkit (BSD-3, 3.4.0, web: https://github.com/readium/kotlin-toolkit), AndroidX PDF (1.0.0-beta01, not yet stable, web: https://developer.android.com/jetpack/androidx/releases/pdf), Media3 ExoPlayer and `MediaLibraryService` (web: https://developer.android.com/media/media3/session/background-playback) |
| Speech | Parakeet via sherpa-onnx on Android (ONNX int8 export exists; Android packaging for this model UNVERIFIED) |
| Sync folder | `DocumentsProvider` (Storage Access Framework) |

Alternatives to revisit at M5: **Skip** (if the iOS UI is SwiftUI-heavy
enough to transpile; a UIKit-first app does not transpile), or **Kotlin
Multiplatform** for the client core (API, cache, outbox) consumed from
Swift. Neither is worth its tooling cost before the iOS app's shape has
settled.

### 3.8 A family of apps, not a super app

Owner ruling, about 21:20 UTC: Autoradio is not a super app. "I don't
want to support every app that the Choir desktop will support."

- Future app agents ship as their own mobile apps: an automatic video
  editor first, CAD later. Each may be bundled with a related consumption
  experience, such as a video editor with a video player.
- Each sibling app connects to the user's Choir computer and its other
  agents.
- Several focused apps that the user switches between are better on a
  phone than one super app.
- Building a complete native app end to end, with automated QA and
  security verification, in under a week is becoming feasible. So the
  plan is for a family of apps.

**What Autoradio owns, and what it leaves to siblings.**

| Autoradio owns | Sibling apps own (each its own desk and app) | Neither (stays on the web desktop or the phone's own apps) |
|---|---|---|
| Listening, talking and takes | The video editor and video player | Spreadsheets (on hold) |
| Autopaper: front page, stories, Opinions | CAD (later) | Calendar (the phone has one) |
| Texture: reading, writing, editing | Any future app agent, with "if there's demand" as the test (state and vision §2.10) | Mail (later, and possibly its own app) |
| Texture's document types: PDF, EPUB, images, embedded audio and video, slideshows | | The web desktop's window manager and app registry |
| Supervising the whole computer, including sibling apps' desks | Supervising their own desk's work in context | |

**What the family shares: a ChoirKit set of Swift packages.** The kit
lives in one repository with one Xcode workspace, and each app is a thin
target on top of it.

```text
          Autoradio        Video (editor + player)     CAD (later)
              \                    |                     /
   +--------------------------------------------------------------+
   | ChoirKit (Swift packages, versioned with the product API)    |
   |  ChoirAuth     sign-in hand-off, device key, Keychain group  |
   |  ChoirClient   generated API client, supervision feed,       |
   |                outbox, capabilities negotiation              |
   |  TextureKit    texture_doc model, TextKit 2 renderer/editor, |
   |                embed views -> viewer protocol                |
   |  ChoirViewers  PDFKit, AVKit, image + Live Text, Readium,    |
   |                QuickLook, slideshow                          |
   |  ChoirSpeech   Parakeet / VAD / TTS stage, model manager     |
   |  ChoirNotify   device registration, notification categories |
   |  ChoirDesign   tokens, AI/Human voice tags, Liquid Glass use |
   +--------------------------------------------------------------+
                               |
              the user's Choir computer (the shared state)
```

- **One sign-in for the family.** A shared Keychain access group holds
  the device-bound refresh token, so signing in once signs in every
  sibling. Keychain sharing is available to free teams (§7.1), but it
  needs one team ID across the apps.
- **Each app is a separate device** in `/auth/sessions`, with its own
  scopes. The video app does not need `self_development:approve`.
- **Texture is shared as a module, never re-implemented.** A video
  project's script, or a CAD part's notes, is a Texture document that the
  sibling renders with TextureKit. One editor, one schema and one fixture
  suite (§3.4) serve every app.
- **The sync folder has one owner.** Exactly one app (Autoradio) ships
  the File Provider extension, so "Choir" appears once in the Files app.
  Siblings open files through the system document picker or the Choir
  API, never through a second provider.
- **Speech models are shared on disk.** They are downloaded once into an
  App Group container that all family apps can read, so a 480 MB model
  is not downloaded three times. App Groups across apps need the same team
  ID; under free provisioning, one group per app was reported (UNVERIFIED).
- **Notifications are routed by owner.** Each app registers its own
  device token, and the host relay (§8.2) routes each event to the app
  that owns its kind. Approvals and supervision alerts go to Autoradio; a
  rendered video goes to the video app.

**How apps hand off to each other.**

| Mechanism | Use | Account needed |
|---|---|---|
| Choir object ids | Every hand-off carries an id (a Texture document, a capture, a publication at a version, a self-development operation). The receiving app fetches the object from the user's computer. **Apps never pass content to each other**, because the computer is the shared state | none |
| Custom URL schemes (`choir-autoradio://`, `choir-video://`) | Direct app-to-app open on the device | free |
| Universal links (`https://choir.news/...`) | The same link opens the right app when it is installed, and the web page when it is not | paid (associated domains) |
| App Intents | "Send this take to the video app", Shortcuts, Siri | free |
| Share sheet and Files | Ordinary iOS hand-off of files and links | free |
| A fallback chain | When a sibling is not installed, Autoradio shows the object read-only (a Texture document, a video in AVKit) with "Get the app". The desktop remains the full surface | none |

**Supervision across the family.** Each app agent is its own desk (owner
notes §5), and Autoradio's supervision home shows every desk on the
computer, siblings' desks included. A sibling shows only its own desk's
work, in the context of its own objects (for example, a video render's
open commitment beside the timeline). The approval itself always happens
in Autoradio, so there is one place where the owner says yes.

**Constraints:**

- **Free provisioning caps the family at about three apps.** It allows 3
  apps per device and 10 App IDs per 7 days, and each extension uses an
  App ID. A second sibling on the owner's phone is therefore another
  reason to buy the account (§7.2).
- **Each app is reviewed on its own.** Each needs its own answer to
  guideline 4.2 (minimum functionality) and 4.2.7(e) (thin clients). That
  answer is the bundled consumption experience: the player with the
  editor.
- **ChoirKit versions with the product API (§8.6).** A sibling that lags
  must keep working against the capabilities endpoint.
- **The "under a week per app" target rests on automated QA.** It
  depends on ChoirKit carrying the hard parts: auth, client, Texture and
  viewers. Each app also needs its own pipeline, built once in M0 for
  Autoradio and reused:
  - XCUITest flows on the simulator, including the iPhone Duo simulator;
  - snapshot tests in compact and regular widths;
  - contract tests against staging;
  - a static security pass: Keychain-only secrets, App Transport Security
    on, no secrets in the bundle. Play logistics: $25 one-time account; new personal accounts need
12 testers opted in for 14 days before production (web:
https://support.google.com/googleplay/android-developer/answer/14151465);
local testing by `adb` needs no verification (web:
https://developer.android.com/developer-verification/guides/faq).

---

## 4. On-device voice

### 4.1 Speech to text: Parakeet

**Which model.** All facts from the model cards and FluidAudio's docs
(web, fetched 2026-10-14).

| Model | Size / languages | Quality | Fit |
|---|---|---|---|
| **parakeet-tdt-0.6b-v3** (https://huggingface.co/nvidia/parakeet-tdt-0.6b-v3) | 0.6B, 25 European languages, auto language detect | Open ASR avg WER 6.34% | **Takes.** Punctuation, capitalization, word timestamps; up to 24 min per pass (3 h with local attention). CC-BY-4.0 |
| parakeet-tdt-0.6b-v2 (https://huggingface.co/nvidia/parakeet-tdt-0.6b-v2) | 0.6B, English | Open ASR avg WER 6.05% | English-only alternative; CC-BY-4.0 |
| parakeet-tdt_ctc-110m (https://huggingface.co/nvidia/parakeet-tdt_ctc-110m) | 114M, English | 7.49% | Small fallback for older phones; CC-BY-4.0 |
| **parakeet_realtime_eou_120m-v1** (https://huggingface.co/nvidia/parakeet_realtime_eou_120m-v1) | 120M, English, streaming | 9.30% at 160 ms chunks | **Live barge-in:** emits an end-of-utterance token (median 160 ms); no punctuation. NVIDIA Open Model License |
| nemotron-speech-streaming-en-0.6b (https://huggingface.co/nvidia/nemotron-speech-streaming-en-0.6b) | 0.6B, English, cache-aware streaming | 6.93% at 1.12 s chunks | Not on iPhone yet: FluidAudio reports ~130 s ANE load and ~1.4 GB residency |
| nemotron-3.5-asr-streaming-0.6b (https://huggingface.co/nvidia/nemotron-3.5-asr-streaming-0.6b) | 0.6B, 40 locales | Fleurs ~8.8-10.4% | Watch; OpenMDW-1.1 licence |

**Which runtime: FluidAudio** (https://github.com/FluidInference/FluidAudio):
an Apache-2.0 Swift package (minimum iOS 17), Core ML models on the
Neural Engine, downloaded from Hugging Face on first use, with an offline
mode and a configurable mirror (`ModelRegistry.baseURL`), Silero VAD and
on-device diarization in the same package. The default Core ML build,
`FluidInference/parakeet-tdt-0.6b-v3-coreml`, is about 480 MB
(https://huggingface.co/FluidInference/parakeet-tdt-0.6b-v3-coreml).
Measured: Core ML encoder compile on first load 3.4 s cold and 0.16 s
warm on an iPhone 16 Pro Max, 4.4 s cold on an iPhone 13; v3 at about
150x real time on an M5 Pro Mac (https://github.com/FluidInference/FluidAudio/blob/main/Documentation/Benchmarks.md).
**No iPhone real-time factor is published for Parakeet TDT
(UNVERIFIED);** it is the first number M0 must measure on the owner's
phone.

Alternatives considered:

| Runtime | Verdict |
|---|---|
| Argmax Pro SDK (Parakeet v2/v3, Core ML) | Commercial, not open (web: https://www.argmaxinc.com/blog/nvidia-frontier-speech-models-on-argmax-sdk); WhisperKit itself is Whisper-only |
| sherpa-onnx (k2-fsa) | Has a Parakeet TDT v3 export (int8 ~0.6-1.0 GB depending on export); official iOS packaging for it UNVERIFIED; better candidate for Android |
| Apple SpeechAnalyzer / SpeechTranscriber (iOS 26) | On device, no download, long-form; launch languages include English, French, German, Italian, Spanish, Portuguese, Japanese, Korean, Chinese, Cantonese (web: https://blog.addpipe.com/apple-speechanalyzer-api/). MacStories' single-file test: 34 min of video in 45 s vs 1:41 for Whisper large-v3 (web, via https://www.gigazine.net/gsc_news/en/20250619-apple-speech-analyzer); no WER measured. **Use as the fallback before Parakeet has downloaded** |
| WhisperKit large-v3-turbo (~632 MB 4-bit) | Slower and larger than Parakeet for the same job; no reason to prefer it |
| `parakeet-ultra-coreml` (FluidAudio's recommended newer build, ~630 MB, better FLEURS WER) | A third-party post-training of v3; check its licence chain before shipping |

**Why Parakeet over Apple's model** (the owner's preference, with
reasons): one model across iOS versions (SpeechAnalyzer needs iOS 26;
Parakeet runs from iOS 17), the same model family runs on Android and on
the server (one transcript format and one quality bar for takes and for
podcast transcripts, which the media plan wants as captures with timed
blocks), word timestamps for `timestamp_range` citations, and a model
version Choir can name in a transcript's provenance ("Choir speech-to-text
with model/version", media plan §4). Apple's model is the fallback, not
the default.

**Licences.** NVIDIA weights CC-BY-4.0: ship an attribution screen
(Settings → About → Models) naming the model and licence. FluidAudio's
Core ML card says Apache-2.0 in prose but cc-by-4.0 in metadata; treat
it as CC-BY-4.0. The EOU model is under the NVIDIA Open Model License:
read it before shipping (UNVERIFIED terms).

**Integration.**

```text
Talk pressed
  -> AVAudioSession: .playAndRecord (+ .defaultToSpeaker, .allowBluetooth)
  -> programme pauses at sentence boundary (TTS unit = sentence)
  -> AVAudioEngine input tap -> 16 kHz mono PCM -> take file (CAF/m4a)
     (+ Silero VAD trims silence; later: EOU model ends the take itself)
Talk released
  -> take saved locally (Library), linked to interrupted segment
  -> Parakeet v3 transcribes the file (foreground; seconds expected,
     UNVERIFIED until measured) -> transcript with word timestamps
  -> take review sheet: play, edit transcript, choose destination
  -> programme resumes at the interrupted sentence
```

- **Model download:** on first use of Talk, not at install; over Wi-Fi by
  default; a background `URLSession` download (web:
  https://developer.apple.com/documentation/foundation/downloading-files-in-the-background);
  stored in Application Support, excluded from iCloud backup; mirrored on
  Choir's own host later (`ModelRegistry.baseURL`) so the app does not
  depend on Hugging Face availability.
- **Prewarm:** compile the Core ML model when the Listen tab opens, so the
  3 to 4 s cold compile never lands on the first take.
- **Foreground only** for transcription until background behaviour is
  measured (UNVERIFIED for Neural Engine work in the background).
- **Bluetooth caveat** (inference, well known but not re-verified): when
  recording from AirPods, iOS switches to the hands-free profile and
  playback quality drops; prefer the phone's microphone while AirPods
  play, and say so in the UI.
- **Memory:** peak RAM for the Parakeet encoder on iPhone is UNVERIFIED;
  measure on the owner's phone and on a 6 GB model before choosing v3 over
  the 110M model as the default.

### 4.2 Text to speech: the clearly-AI voice

| Option | Verdict |
|---|---|
| **`AVSpeechSynthesizer`** | **M0.** Zero download, no licence risk for speaking. The web prototype picks Fred/Ralph/Albert on macOS; iOS ships Apple's "novelty" voices too (UNVERIFIED on the current iOS; check on device). Voice quality tiers depend on what the user downloaded in Settings; Siri voices are not listed for third parties (web: https://developer.apple.com/forums/thread/676726, no authoritative answer). Caching `write(_:toBufferCallback:)` output in a commercial app raises an unresolved SLA question (web: https://developer.apple.com/forums/thread/812285): speak live, do not cache |
| **Kokoro-82M** (Apache-2.0 weights, 54 voices, 8 languages, https://huggingface.co/hexgrad/Kokoro-82M) | **The product voice (M2 or M3).** kokoro-ios (MLX Swift, MIT) claims ~3.3x real time on an iPhone 13 Pro (https://github.com/mlalma/kokoro-ios); FluidAudio has a Kokoro build, iPhone inference "unvalidated" by its README. Pre-render sentence by sentence ahead of playback |
| Kyutai Pocket TTS (~100M, MIT code) | Streaming alternative if first-audio latency matters (https://github.com/kyutai-labs/pocket-tts); it clones voices, which we do not want; use a fixed voice only |
| Piper | **Avoid:** the engine is now GPL-3.0 (https://github.com/OHF-Voice/piper1-gpl) and embeds espeak-ng |
| KittenTTS 2, NeuTTS Air | Too large or cloning-oriented for v1 |
| Server TTS | Rendering a whole programme ahead of time on the server is the cheap fallback for older phones; adds latency and a network dependency to the talk-back path |

**espeak-ng is GPL-3.0-or-later** (https://github.com/espeak-ng/espeak-ng);
Kokoro's original pipeline uses it for phonemes. Use MisakiSwift
(Apache-2.0 English G2P, https://github.com/mlalma/MisakiSwift) or a
Core ML G2P, and check FluidAudio's current dependencies (its docs and
README disagree about eSpeak).

**"Clearly AI" is a design choice, not a model property.** Kokoro sounds
fairly natural. To keep the owner's contrast: one fixed voice, slightly
lowered pitch (the prototype's choice), a light, consistent audio
treatment on every AI segment (band-limiting, a faint "radio" filter),
the blue monospace "AI" tag on screen, and a short sonic ident before
each human clip. No voice cloning, ever.

### 4.3 Audio session and background behaviour

```text
            +-----------+   Talk down    +-----------+
  launch -> | PLAYING   | -------------> | RECORDING |
            | .playback |                | .playAnd- |
            | TTS+clips |  <-----------  | Record    |
            +-----------+   Talk up +    +-----------+
              |   ^         transcribe        |
  interrupt   |   | .shouldResume             | call / Siri interrupts:
  (call, Siri)v   |                           v save partial take, stop
            +-----------+               +-----------+
            | PAUSED    |               | REVIEW    | -> destinations
            +-----------+               +-----------+
  route change (headphones out) -> pause; Now Playing + remote commands throughout
```

- `UIBackgroundModes: audio`; Now Playing info and `MPRemoteCommandCenter`
  for lock-screen play, pause, skip; interruption and route-change
  observers. (Apple documentation pages did not render for the research
  agent; these are standard AVFoundation behaviour, UNVERIFIED this
  session.)
- **No silent keep-alive loops** (guideline 2.5.4 risk). Render TTS into
  an `AVAudioEngine` graph alongside clips so the programme is one
  continuous audio session; a report says `AVSpeechSynthesizer.speak`
  pauses at screen lock (web: https://developer.apple.com/forums/thread/849627),
  which is one more reason to move to an in-graph TTS (Kokoro) early.
- An audio session cannot be started from the background (web:
  https://developer.apple.com/forums/thread/759816): a programme starts
  in the foreground, by a widget tap, or by an App Intent.
- **CarPlay (later).** The audio entitlement
  (`com.apple.developer.carplay-audio`) is requested from Apple (web:
  https://developer.apple.com/carplay/). iOS 26.4 added a "voice-based
  conversational apps" CarPlay category (web:
  https://www.macrumors.com/2026/02/18/ios-26-4-carplay-support); whether
  one app can hold both is UNVERIFIED. Autoradio is an audio app first;
  apply for audio, and treat Talk in the car as a later question.

---

## 5. Texture multimedia mini-apps

### 5.1 The pattern

A mini-app is **a native viewer that a Texture embed opens**. It is
compiled into the binary, it shows content (never runs downloaded code),
and it is reachable only from a document, a story, the Library or the
sync folder. That keeps it outside guideline 4.7's "mini apps" (HTML5/JS
software not embedded in the binary) and 2.5.2.

```text
Texture document
  ... claim text [video 2:13] ...         <- collapsed chip (source_ref / media_ref)
        | tap
        v
  +----------------------------------+    <- expanded inline (attachment view):
  | poster / first page / waveform   |       caption, credit line, licence
  | caption - credit - licence       |
  +----------------------------------+
        | tap "open"
        v
  full-screen native viewer (PDFKit / AVPlayerViewController /
  Readium / image+Live Text / QuickLook), positioned at the
  cited selector (page, timestamp_range, image_region)
```

- **Day one** the renderer handles the `source_ref` target kinds the
  backend already allows (`internal/texturedoc/schema.go:404`: `image`,
  `video`, `audio`, `pdf`, `transcript`, `web_url`, `content_item`, and
  others), the same set the desktop's `texture-source-renderer.ts`
  renders today.
- **Authoring** media into a document needs the `media_ref` block
  (`source_entity_id`, `display_mode` collapsed or expanded, an optional
  `selector`, a Texture-authored caption and alt). That is a change to
  `internal/texturedoc/schema.go`: a **red** surface (Texture canonical
  writes) with full ceremony, as the media plan says (step 8). The phone
  should not lead that change; it should be the second client of it.
- **Rights travel with the media.** The viewer shows the credit line
  derived from licence fields; embeds (YouTube, podcasts) play from the
  publisher, never re-hosted (media plan §4, §5.4).

### 5.2 Per media type

| Media | Inline (in Texture) | Full screen | Notes and sources |
|---|---|---|---|
| **Audio / podcast** | Native mini scrubber at the cited timestamp | Hand off to the app's own player (Listen engine), Now Playing | Publisher enclosure URL, never proxied; `timestamp_range` selector exists in `internal/sourcecontract/selector.go` with no consumer yet |
| **Images** | Image with caption and credit | Native zoom/pan + VisionKit Live Text (`ImageAnalyzer`, iOS 16+, A12+; https://developer.apple.com/documentation/visionkit/imageanalyzer) | Media plan: "no image" is the default; images appear only when judged to add information |
| **PDF** | First-page thumbnail (PDFKit render) | `PDFView`: search, selection, `UIFindInteraction`, markup (https://developer.apple.com/documentation/pdfkit/pdfview) | pdf.js on iOS hits WebKit canvas limits (~16.7 MP; https://developer.apple.com/forums/thread/683573). PDFKit is free and native |
| **Video** | Poster frame and duration chip | `AVPlayerViewController`: PiP, AirPlay, HLS (https://developer.apple.com/documentation/avkit/avplayerviewcontroller) | Starts at the cited time |
| **YouTube** | Poster chip | Official IFrame player in a small WKWebView with a correct Referer; "Open in YouTube" fallback | YouTube requires client identity via Referer and forbids overlays (https://developers.google.com/youtube/terms/required-minimum-functionality); embed error 153 has hit iOS apps since late 2025 (https://til.simonwillison.net/youtube/fixing-153-embed) |
| **EPUB** | Cover card with "continue reading" | Readium Swift Toolkit navigator (BSD-3; 3.11.0, July 2026; EPUB 2/3 reflowable and fixed layout, TTS read-aloud; https://github.com/readium/swift-toolkit) | Stay on 3.x until 4.0 leaves alpha. Readium's read-aloud could feed Autoradio ("read me chapter 3") later |
| **Slideshows** | First-slide thumbnail | **A slideshow is a kind of document** (owner). An authored deck is a Texture document whose top-level sections are slides, and TextureKit presents it full-screen as a deck (paging, presenter notes, AirPlay to a display). Imported pptx or key files open in QuickLook (`QLPreviewController`, presented modally; confirm Office coverage on a device) | No web deck runtime. A slideshow document is a variant of the Texture schema, with the `media_ref` block for its images (red: schema) |
| **Anything else** | File card | QuickLook | Generic fallback |

### 5.3 Order

1. **Audio and images (M1-M2):** they serve Listen and the Autopaper
   story directly, and audio is half-built already.
2. **PDF (M3):** the most-requested document type; PDFKit makes it cheap.
3. **Video, including YouTube (M3):** primary-source media in stories.
4. **EPUB (M3-M4):** Readium is a dependency of real size; worth it for
   "Autoradio as a superset of audiobooks" (pricing notes) once the
   library has books.
5. **Slideshows (M4).** Imported decks open in QuickLook first. Authored
   slideshows come as a Texture document type once `media_ref` exists.
6. Held by the owner: spreadsheet (hold), calendar (no), email (later).

Each viewer also reports reading or playback position to
`/api/media/progress`, so a PDF opened on the desktop resumes on the
phone.

---

## 6. The sync folder on the phone

The owner wants a sync folder; a separate agent is designing the desktop
and server side. `docs/reports/desktop-sync-and-local-vms-2026-10-14.md`
had **not landed** when this note was written (checked in this worktree
and in the main checkout), so this section names the shared sync API as a
dependency and states what the phone needs from it.

**On iOS the sync folder is a File Provider extension**
(`NSFileProviderReplicatedExtension`, iOS 16+; web:
https://developer.apple.com/documentation/fileprovider/replicated-file-provider-extension):
"Choir" appears as a location in the Files app; the system owns local
copies, materializes on open and evicts under pressure; the extension
enumerates items, reports changes since an opaque sync anchor, and
uploads and downloads through background `URLSession` tasks.

What exists and what is missing (code):

| Need | Today | Gap |
|---|---|---|
| Read and write files | `/api/files/{path}` GET, PUT, POST (`cmd/choir/files.go`) | No item identity separate from path (renames break identity) |
| Enumerate a folder | Listing via the files API | Pagination for large folders |
| Changes since X | None | **A change feed with an opaque anchor** (a monotonic version or the tape position), tombstones for deletes, and "anchor expired" signalling |
| Versions and conflicts | None in the files API | Per-item content version (hash) and conditional writes (`If-Match`), so two writers produce a conflict copy, not a lost write |
| Large files | Single-request PUT | Resumable or chunked upload; background-transfer friendly (uploads must be from a file) |
| Wake the extension on remote change | None | APNs File Provider push (paid account) or polling on foreground |
| History | A macOS File Provider and SyncEngine were built and deleted (`7b59d33e8`, rollback ref `a1073731`) | Read them before the new design; the deletion note calls them superseded |

**Constraints to design for:**
- The extension process has a small memory ceiling (about 20 MB by
  developer reports; web: https://developer.apple.com/forums/thread/804378;
  not an Apple-documented figure). Stream everything; never load a file
  or a whole listing into memory.
- The extension needs an App Group to share the session and cache with
  the app. App Groups are listed as available to free teams (web:
  https://developer.apple.com/help/account/reference/supported-capabilities-ios);
  whether a File Provider extension runs under a free team is
  **UNVERIFIED** and is a spike in M4. The "FileProvider Testing Mode"
  entitlement is paid-only.
- **Single state authority:** the user's computer is the authority for
  file contents; the phone is a cache. The same API must serve the
  desktop sync client and the phone, so it is designed once, by the sync
  design, not by the mobile app.

Android equivalent: a `DocumentsProvider` (stable document ids,
`notifyChange` on child URIs; web:
https://developer.android.com/guide/topics/providers/document-provider).

---

## 7. Local-only development without a paid Apple account

### 7.1 What free provisioning allows and blocks

From Apple's capability table (web:
https://developer.apple.com/help/account/reference/supported-capabilities-ios,
parsed from the raw HTML) and membership comparison (web:
https://developer.apple.com/support/compare-memberships/): free teams can
register up to 10 App IDs and 3 devices, each expiring after 7 days, and
install up to 3 apps per device. Developer Mode must be on (web:
https://developer.apple.com/documentation/xcode/enabling-developer-mode-on-a-device).

| Autoradio needs | Free team | Effect on the plan |
|---|---|---|
| Background audio, microphone, Core ML / Neural Engine | **Yes** (background modes listed; mic is a usage string) | M0 to M2 are fully buildable |
| App Groups, Keychain sharing | **Yes** | Widgets and the File Provider can share the session |
| Widgets, Live Activities started locally | Yes (no capability gate; consumes App IDs) | Each extension is another App ID: app + widget + File Provider = 3 of 10 |
| Associated Domains (passkey `webcredentials`, universal links) | **No** | Native passkey API and passkeys inside a WebView are blocked; use `ASWebAuthenticationSession` (§8.1) |
| Push notifications (APNs) | **No** | Supervision alerts only while the app runs; local notifications only |
| iCloud, Sign in with Apple, IAP, App Attest | No | Not needed before the App Store |
| Increased Debugging Memory Limit | No | Matters for large Core ML models during debugging |
| TestFlight, App Store | No | Owner-only installs, re-signed every 7 days |
| File Provider extension | No capability row; **UNVERIFIED** in practice | Spike in M4 before relying on it |

### 7.2 When to buy the account

Buy the Apple Developer Program membership ($99 a year; web:
https://developer.apple.com/programs/enroll/) at the **first** of:

1. **M1 lands and supervision without push is the bottleneck** (an
   approval waiting hours because the phone could not alert). This is
   the likeliest trigger, and it is the owner's own condition ("set up
   notifications").
2. A second tester is wanted (TestFlight: 100 internal, 10,000 external
   testers, builds expire after 90 days; web:
   https://developer.apple.com/help/app-store-connect/test-a-beta-version/testflight-overview).
3. The 7-day re-sign becomes a nuisance.

Decide **individual or organization** before buying. An organization
account needs a D-U-N-S number and a legal entity (Choir is a Delaware C
corp, per the media research), shows the company as the seller, and takes
longer; D-U-N-S issuance can take up to 5 business days (web, via search
snippet: https://developer.apple.com/support/D-U-N-S/). Starting the
D-U-N-S lookup now costs nothing and removes the delay later.
**Recommendation:** the organization account, started in parallel with M1.

### 7.3 What changes with the paid account

APNs (supervision alerts, Live Activity updates, File Provider push),
Associated Domains (native passkeys via
`ASAuthorizationPlatformPublicKeyCredentialProvider` against
`choir.news`, and universal links so `choir.news/autopaper/...` opens in
the app), TestFlight, App Attest (device integrity for approvals), and
the CarPlay entitlement request. Server work it triggers:
`/.well-known/apple-app-site-association` on `choir.news` (with
`webcredentials` and `applinks`), and an APNs provider (§8.2).

### 7.4 App Store review risks

| Guideline (June 8, 2026 text) | Risk | Mitigation |
|---|---|---|
| **4.2.7(e)** "Thin clients for cloud-based apps are not appropriate" | **High if** the supervision tab looked like a remote desktop | Native UI over a documented API, with substantial local function (audio, speech, reading, offline). Present the computer as an account the app works with, as a developer-tools or cloud-console app does |
| 4.2 minimum functionality | Low for a native app | Listen and Paper work signed out |
| 2.5.2 downloaded code | Low | No WebView loading remote UI; Core ML model weights are data (inference: Apple's rule is about code; many apps download models) |
| 4.7 mini apps | Low | Viewers are compiled in; no third-party software catalog |
| **1.2 user-generated content** | **Medium once takes publish as Opinions** | Filtering, report and block, published contact details: these must exist on the server (LC) before publishing from the app ships |
| **5.1.2(i)** sharing personal data "with third-party AI" needs disclosure and explicit permission | Medium | A consent screen: takes and transcripts sent to the user's computer may be processed by third-party model providers (the gateway's routes) |
| 5.1.1(v) account deletion in the app | Certain | An account-deletion flow and endpoint (none exists today, §8) |
| 3.1.1 in-app purchase for digital subscriptions | Open | Billing does not exist yet; at launch either IAP, or US-only external links (3.1.1(a)), or sign-up outside the app with no purchase UI (common "account" pattern; UNVERIFIED how Apple treats it for Choir) |
| EU AI Act Art. 50 labelling (not Apple) | Already a live duty for AI-written public-interest text from 2 Aug 2026 (media plan) | The AI badge on narration; the story's AI disclosure carried into the app |

---

## 8. Backend dependencies and gaps

### 8.1 Auth for a mobile client

What exists (code): passkey WebAuthn (`/auth/register/*`, `/auth/login/*`),
cookie sessions (`choir_access` JWT 5 min, `choir_refresh` 720 h,
HttpOnly, SameSite=Lax; `AUTH_COOKIE_SECURE` defaults to false, worth
checking on staging), email magic-link recovery (`/auth/recovery/*`;
the handler logs token creation but **no mail send was found**, and the
frontend has no recovery caller, so recovery delivery is unconfirmed),
session listing and revocation (`/auth/sessions`), scoped API keys
(`choir_sk_...`), and the desktop hand-off: `GET
/auth/desktop/exchange-redirect` → `302 choir://auth-complete?code=` (60 s,
one-time) → `POST /auth/desktop/redeem` mints an independent session
(`internal/auth/handlers.go:1425-1515`).

**Proposal (red: auth and session issuance; full ceremony):**

```text
iPhone app                     choir.news (Safari-backed session)      auth
   | ASWebAuthenticationSession(url: /auth/... login page,
   |                            callbackURLScheme: "choir")
   |-------------------------------->| passkey ceremony (Face ID),
   |                                 | works without app entitlements
   |                                 | (Apple DTS; passkeys.dev)
   |                                 |-- GET /auth/desktop/exchange-redirect
   |<-- choir://auth-complete?code=<one-time, 60 s> ----------------------|
   | POST /auth/mobile/redeem {code, device_name, device_public_key}  --->|
   |<-- {refresh_token (device-bound), access_token, user}  -------------|
   | store refresh token in Keychain
   |   (kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly; shared with
   |    extensions via the App Group keychain)
   | API calls: Authorization: Bearer <access>; renew with refresh
```

- **Bearer tokens, not cookies.** The desktop's redeem returns cookies
  into a native cookie jar; a phone and its extensions are simpler with a
  device-bound refresh token. A device key in the Secure Enclave signs
  the refresh request (proof of possession), so a stolen token does not
  work elsewhere. This is new (a mobile redeem variant), and it must
  appear in `/auth/sessions` as a named device that the owner can revoke.
- **Step-up for approvals.** Approving a self-development operation
  requires a fresh local Face ID and a signature from the device key over
  the operation id, verified server side. Later (paid account, App
  Attest) the server can also check the app's integrity.
- **Scopes.** The mobile session gets the owner's scopes. The existing
  `computer:self_development:approve` scope exists for API keys; the same
  scope model should govern the mobile session so the owner can issue a
  "listen and read only" device.
- **Spike first:** the `ASWebAuthenticationSession` passkey ceremony with
  no associated-domains entitlement is supported by Apple's own guidance
  and passkeys.dev, but no first-hand report was found (UNVERIFIED). It is
  a half-day spike in M0 and the gate for the rest.
- **Account deletion** (App Store 5.1.1(v)): an endpoint that deletes the
  account and schedules the computer's teardown; it does not exist. It is
  black-class work on the server side (irreversible), with a grace period.

### 8.2 Push notifications

- **Paid account required.** Until then, local notifications only.
- **Host-side relay, not the computer.** The computer (a guest VM) should
  not hold APNs credentials. Desks emit typed events (awaiting approval,
  run finished); a host service (a new small `notifyd`, or a role inside
  the proxy or gateway) maps events to device tokens and sends minimal
  payloads (ids only). Credentials stay on the host like provider keys do
  (`docs/learn/03-gateway-and-providers.md`: credential custody). Red
  (new outbound provider and credentials).
- **Device registry:** `POST /api/devices` (token, platform, locale,
  notification preferences), tied to the mobile session; revoked with it.
- **Live Activity updates** for running self-development operations use
  APNs push-to-update tokens through the same relay.

### 8.3 Streaming and the supervision feed, without waking the computer

Fact (doc: `docs/learn/04-computer-lifecycle.md`): idle computers are
hibernated after `VMCTL_IDLE_TIMEOUT` (30 minutes), and a hibernate writes
the full 8 GiB guest. A phone that polls the computer, or holds
`/api/ws` open, keeps it awake and costs compute (pricing notes: compute
is where the margin is). So:

- **The phone talks to the host for "is there anything new?"** A
  host-side supervision summary (counts and ids, written by the computer
  when events happen, like the publication front is served from Store B
  while the computer sleeps) answers without waking the guest.
- **The phone wakes the computer only on intent:** the owner opens a
  document, reviews an approval, or talks.
- **One supervision feed with a cursor:** `GET /api/supervision/feed?after=`
  returning approvals awaiting decision, open obligations, running
  trajectories, open and recently settled commitments, ordered by
  materiality, as **one projection of the tape** (the "supervision
  projection" in `docs/precommitment-records.md`), not a client-side join
  of five endpoints. SSE while the screen is open; the cursor resumes on
  reconnect.
- **Per-document live updates** reuse `/api/texture/documents/{id}/stream`
  (SSE, exists) while a document is open.

### 8.4 Takes, captures and Opinions

- `POST /api/captures` exists (code: `internal/apihandler/routes.go`); a
  take needs a capture variant with voice provenance (device, model and
  version of the transcriber, the interrupted segment as a citation),
  audio bytes in the content-addressed store, and the transcript as
  timed blocks. Orange.
- Share with team and publish as Opinion depend on LC (contributions)
  and LA (accounts); the phone ships those actions when the server does.
- "Direct my computer" maps a take to a **directive** commitment record
  (`internal/types/commitment.go` has the `directive` kind) addressed to
  management, which is the existing delivery mechanism between Textures
  and desks (`docs/autopaper-sw-design-2026-10-09.md` §2).

### 8.5 Offline

| Data | Cache policy |
|---|---|
| Autopaper front and stories | Last N stories pinned by version; refresh on foreground; programme can play offline from cache (with on-device TTS) |
| Podcast clips | Streamed from the publisher; no offline copy (rights: never re-host; a personal offline download is an open question for the media lawyer list) |
| Texture documents | Recently opened documents and their last revision; edits queue in the outbox against a base revision |
| Supervision feed | Last snapshot, shown as "as of" with its time |
| Takes | Local until sent; never deleted before the server acknowledges |
| Speech and TTS models | Downloaded once; re-verified by hash |

### 8.6 A stable, versioned product API

The state and vision report already names the gap: "What is missing is a
published, stable API contract" (§2.10). A native app is a fixed binary
in users' hands for months, while the computer's own surface can change
by self-development. The app therefore needs:

- an **OpenAPI description** of the endpoints it uses, generated or
  checked in CI (types generated for Swift and, later, Kotlin);
- a **capabilities endpoint** (`GET /api/client/capabilities`: API
  version, feature flags), so the app degrades instead of breaking;
- a rule that self-development changes to these endpoints are
  **additive**, enforced by a contract test in CI (yellow).

This is foundations work in the owner's sense, and it benefits the CLI
and external agents equally.

### 8.7 Security

- Tokens in the Keychain only (never `UserDefaults`), with
  this-device-only accessibility; Secure Enclave device key for
  proof-of-possession and approval signatures.
- Takes and transcripts at rest under iOS Data Protection
  (`completeUntilFirstUserAuthentication` at least; `complete` for the
  take store if background upload is not needed).
- Certificate pinning: not at first (it complicates self-hosted
  computers, which the owner encourages); revisit for the managed cloud.
- **Self-hosting:** the app must accept a custom host (the owner wants
  self-hosting encouraged; pricing notes). The sign-in flow takes a host
  URL; Autopaper defaults to choir.news.
- **Secrets vault later** (owner notes §8): the phone is a natural second
  factor for releasing a secret to an agent ("engineering wants your
  GitHub token for 10 minutes: approve?"). Design it with the vault, just
  before agentic red teaming, not now.

---

## 9. Mutation classes of the proposed work

| Work | Class | Protected surfaces |
|---|---|---|
| The iOS app itself (a new client) | orange | none server-side; rollback = do not install |
| Mobile redeem, device-bound refresh tokens, device registry | **red** | auth/session renewal |
| Approval step-up (device signature verified server side) | **red** | self-development approvals, run acceptance |
| APNs relay and credentials | **red** | gateway/provider-style outbound credentials |
| Supervision feed projection and host-side summary | orange | reads the tape; no new authority |
| Take capture kind, upload | orange | object graph registry (additive kind) |
| `media_ref` Texture block | **red** | Texture canonical writes |
| Sync change feed, conditional writes | orange (red if it touches the tape's write path) | file authority |
| Account deletion | **black** on execution (irreversible) | user data; needs a grace window |
| OpenAPI contract tests, shared Texture fixtures | yellow | none |
| AASA file on choir.news | orange | deployment routing (static file) |

---

## 10. Phased plan

The owner's order today is foundations first (security, red teaming,
self-development on PSA), with Autopaper, Autoputer and Autoradio
production-grade in about a month. The mobile app fits that order if it
is **a client that pulls foundations forward** (the API contract, mobile
auth, the supervision projection, the sync API) and **never invents
authority** of its own. Estimates are in focused agent-days with owner
review, from the size of comparable work in this repo; they are
estimates, not commitments.

The owner expects that a complete native app with automated QA will soon
be buildable in under a week. These estimates are higher for three
reasons:

- **The first app pays for ChoirKit**, M0's QA pipeline and the server
  gaps (§8).
- **Device-in-the-loop checks on the owner's phone can't be fully
  automated.** They cover audio routes, lock-screen behaviour, the
  microphone and Neural Engine timings, and they run under 7-day free
  provisioning.
- **Red-class server work has full ceremony.** That covers auth, approval
  step-up, the APNs relay and the `media_ref` block.

Sibling apps should come much closer to the owner's week.

```text
week:     1    2    3    4    5    6    7    8    9   10   11   12
spikes  [##]
M0        [#########]                         local, free team
M1                [###########]               supervision (needs feed API)
M2                         [#########]        Parakeet takes -> Texture
paid acct              ^ buy (org, D-U-N-S started at M0)
M3                                 [########] media viewers, push, TestFlight
M4                                      [#######] sync folder (needs sync API)
App Store                                          [###] review
Android                                                 [... M5 ...]
```

| Milestone | Scope | Depends on (server) | Done when (artifact) | Effort |
|---|---|---|---|---|
| **Spikes** (first) | (a) Parakeet v3 via FluidAudio on the owner's iPhone: RTF, peak memory, cold compile, for a 30 s and a 5 min take; (b) `ASWebAuthenticationSession` passkey sign-in to staging under a free team, returning `choir://auth-complete`; (c) TextKit 2 editing of a real `choir.texture_doc.v1` document with dictation, autocorrect and a citation chip | none (staging as is) | A dated evidence note per spike with numbers and a screen recording | 2-3 days |
| **M0: local prototype on the owner's iPhone** | Listen (programme port, AVSpeech machine voice, publisher clips via AVPlayer, Now Playing), Paper (native front and story reader), sign-in, Talk records takes locally, Library | Mobile redeem variant (or reuse desktop redeem's cookie jar for M0 only) | Owner listens to today's paper on his phone, interrupts, plays back a take; a repeatable UI test run (XCUITest) recorded as an artifact | 8-12 days |
| **M1: Texture supervision** | Supervision home, approval sheet with Face ID, run timeline, Texture reader and editor (TextKit 2), prompt bar, "My computer" programme (supervision as radio), local notifications | Supervision feed + host summary (§8.3), approval step-up (red), OpenAPI contract for used endpoints | The owner approves a real self-development operation on staging from the phone, and the trace shows the decision with the device signature | 12-18 days app + 5-8 days server |
| **M2: Parakeet takes** | Transcription on device, take review, destinations: keep, draft a Texture document, directive to management; Silero VAD; model download manager; attribution screen | Take capture kind + upload; directive records from the API | A take becomes a Texture draft on the owner's computer with voice provenance and model version; transcript WER spot-checked on 20 takes | 8-12 days |
| **Paid account** | Apple Developer Program (organization), AASA on choir.news, APNs relay | APNs relay (red), AASA | Push for awaiting-approval on staging | 3-5 days server |
| **M3: media and polish** | Viewers: audio, images (Live Text), PDF, video and YouTube; native passkeys; universal links; widgets and Live Activity; TestFlight | `media_ref` (red, its own ceremony) for authoring; rendering works without it | TestFlight build used daily by the owner for a week | 10-15 days |
| **M4: sync folder** | File Provider extension | The shared sync API (§6) | Files app shows "Choir"; a file edited on the desktop appears on the phone and vice versa, with a conflict copy on a forced conflict | 8-12 days app, after the API |
| **App Store** | Account deletion, UGC controls if publishing ships, privacy labels, AI-data consent, review notes positioning (not a thin client) | Account deletion (black on execution), LC moderation | Approved build | 5-8 days + review time |
| **M5: Android** | Kotlin/Compose app to M2 parity | Same APIs | Play closed test with 12 testers for 14 days | 25-40 days |

**What to build first, given foundations-first:**

1. **The three spikes** (two to three days). They retire the riskiest
   assumptions: Parakeet speed and memory on the actual phone, sign-in
   without a paid account, and native editing of the real schema.
2. **The supervision-feed contract** (a design note and an OpenAPI
   fragment, no code): it is the API the owner most wants on the phone,
   it is a projection the desktop and the CLI also need, and it forces
   the materiality-ordered supervision projection of
   `docs/precommitment-records.md` out of design. This is foundations
   work that the app pulls forward.
3. **M0**, as a labelled prototype with no server changes beyond the
   mobile redeem variant.

**What not to do yet:** no take publishing before LC moderation exists;
no `media_ref` authoring before the media plan's steps 4-7 show what the
block must carry; no File Provider before the sync API; no CarPlay before
the App Store; no shared cross-platform code before the iOS app settles.

### 10.1 Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Parakeet too slow or too large on the owner's phone | Low-medium (Mac 150x RT; no iPhone figure) | Takes lag | Spike first; 110M model or SpeechAnalyzer fallback |
| Passkeys in `ASWebAuthenticationSession` fail under a free team | Low | No sign-in until paid | Spike first; buy the account early if it fails |
| App Review reads supervision as a thin client (4.2.7(e)) | Medium | Rejection | Native, substantial local function; Listen and Paper signed out; clear review notes |
| Native editor parity with the web editor drifts | Medium | Documents differ by client | Shared JSON fixtures in both test suites (§3.4) |
| The phone keeps computers awake | High if naive | Compute cost | Host-side summary; wake on intent only (§8.3) |
| Self-development changes an endpoint the app uses | Medium | App breaks in users' hands | OpenAPI contract test, capabilities endpoint, additive-only rule (§8.6) |
| Rights: clip excerpts and transcripts | Medium | Takedowns | Publisher-hosted playback only; media plan's lawyer questions before Opinion audio ships |
| Owner attention | High | Slows everything | The app is a supervision tool: it should reduce the owner's load, measured by approvals handled per day from the phone |
| Scope creep into the desktop | Medium | "Not cool", rejected | The four tabs are the scope; anything else must argue its way in |

### 10.2 Decisions for the owner

1. Apple account: organization (start D-U-N-S now) or individual.
2. Swift (UIKit-first, SwiftUI where required) as recommended in §3.3,
   or an Objective-C core.
3. Minimum iOS version. Recommendation: **iOS 26**, one release behind
   the current iOS 27. That makes the SpeechAnalyzer fallback and the
   Liquid Glass APIs unconditional and halves the test matrix. FluidAudio
   alone would allow iOS 17. Install share by version is UNVERIFIED;
   revisit before the App Store.
4. Parakeet v3 (multilingual, ~480 MB) or v2 (English) as the default
   download.
5. Whether the "My computer" programme (private ledger read aloud) is in
   M1 or waits for M2.
6. Whether takes may ever be stored on the server before LC.
   Recommendation: yes, as private captures on the owner's own computer.
7. Where the mobile code lives. Recommendation: a `mobile/` tree in this
   repository (ChoirKit plus app targets), so that API contract changes
   and client changes land in one commit and CI can run the contract
   tests. The alternative is a separate repository per app family.

---

## Sources

Choir (code and docs): `docs/reports/autoradio-prototype-2026-10-14.md`,
`docs/reports/choir-state-and-vision-2026-10-14.md` (§2.1, §2.10, §2.11),
`docs/reports/strategy-takeaways-2026-10-14.md`,
`docs/reports/owner-notes-2026-10-14.md`,
`docs/reports/pricing-and-economics-notes-2026-10-14.md`,
`docs/reports/self-hosting-and-desktop-2026-10-14.md` (main checkout),
`docs/research/autopaper-media-and-rights-2026-10-14/README.md`,
`docs/learn/04-computer-lifecycle.md`, `docs/precommitment-records.md`,
`frontend/src/lib/TextureEditor.svelte`,
`frontend/src/lib/texture-structured-editor-doc.ts`,
`frontend/src/lib/texture.js`, `frontend/src/lib/apps/registry.ts`,
`internal/auth/handlers.go`, `internal/auth/recovery.go`,
`internal/apihandler/routes.go`, `internal/texturedoc/schema.go`,
`cmd/choir/selfdev.go`, `cmd/choir/files.go`, branch
`prototype/autoradio-2026-10-14`, commits `7b59d33e8` and `02ff06a64`.

Web sources are cited inline. Research was done by sub-agents on
2026-10-14; Apple's JavaScript-rendered documentation pages often did not
load for them, and claims resting on general knowledge are marked
UNVERIFIED in the text.

## Owner rulings, 2026-10-14 ~21:40 UTC

Decisions (§10.2):
1. Apple account: the owner already has a business (organization) Apple
   Developer account — use it; push, passkeys, universal links and
   TestFlight are available from M0.
2-5, 7: agreed as recommended (Swift with a UIKit core; iOS 26 minimum;
   Parakeet v3 default; code under `mobile/` in this repo).
6. Takes are captured on the server (as private captures on the owner's
   own computer).
The "My computer" programme is replaced by the voice-driven model below.

Autoradio is entirely voice-driven; with every visual element removed it
should work on an Apple Watch. Want the state of your computer? Say so.
Turn it on without saying anything and it plays whatever is most
intelligent to play: it learns the mix of media you like and weighs the
importance and urgency of topics for you.

The core mechanism: Autoradio traverses the user's personal object graph
and the global Autopaper object graph to produce a continuous stream of
audio — segments and full episodes of media made by and for humans
(podcasts, videos), AI-voiced updates from written media (Autopaper),
updates from the user's agentic email, and updates on the state of the
user's work from the other desks. The user can interrupt at any time to
ask or redirect; it learns from usage and follows up on topics of
interest (e.g. a new Machine Learning Street Talk episode plays when it
appears for someone interested in AI and tech business strategy).

Interaction model: not conversational turn-taking. Long monologues in
(the owner may speak for ten minutes), long listening out (10, 30, 80
minutes), with interruption whenever wanted; passive listening or active
talking, including performing — a take that may be sent to other people,
optionally edited automatically before it goes out. Execution is
everything.

Email: Choir's email app becomes a full desk app agent with its own RLM;
every paid account gets its own agentic email address; by default it
sends nothing without approval; users may grant policies (e.g. a daily
digest to their personal address, or specific actions on their behalf).
Email goes hand in hand with Texture and media (attachments, documents)
and is included in Autoradio. Connecting external mailboxes (Gmail etc.)
is deferred to near the end: commodity features are not built until
users ask for them.
