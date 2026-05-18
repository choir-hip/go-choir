# UI/UX Bug Sweep Review - 2026-05-18

Status: report-only review. No product code was changed in this pass.

Staging reviewed: `https://draft.choir-ip.com`

Playwright evidence:

- `/private/tmp/choir-ux-audit-1779118051219/audit.json`
- `/private/tmp/choir-ux-audit-1779118051219/logged-out-desktop-mobile.png`
- `/private/tmp/choir-ux-audit-1779118051219/podcast-library-mobile.png`
- `/private/tmp/choir-ux-audit-1779118051219/podcast-feed-mobile.png`
- `/private/tmp/choir-ux-trace-shell-audit-1779118177768/audit.json`
- `/private/tmp/choir-ux-trace-shell-audit-1779118177768/trace-mobile-selected.png`
- `/private/tmp/choir-ux-trace-shell-audit-1779118177768/trace-vtext-overlap-mobile.png`
- `/private/tmp/choir-ux-trace-shell-audit-1779118177768/candidate-desktop-mobile.png`
- `/private/tmp/choir-ux-trace-shell-audit-1779118177768/settings-mobile.png`

User evidence:

- Mobile Safari screenshots from `draft.choir-ip.com` showing Podcast with hash-like XML titles, Open source, Open in VText, provenance, missing player controls, broken landing hierarchy, and Trace/VText overlap.

## Executive Summary

Choir has reached the point where UX debt is a platform blocker, not a polish backlog. The core product promise depends on users being able to open apps, inspect traces, read and revise VText, and trust candidate/promotion flows. On mobile, these surfaces currently fail basic navigation and readability expectations.

The highest-value next sweep should not try to fix every visual issue. It should establish a usable app shell and two representative apps:

1. Podcast as the "ordinary app" proof: library, search/import, feed navigation, playback controls, episode scrolling, playback position, and logged-out read/explore.
2. Trace as the "evidence app" proof: readable run acceptance/provenance and reachable inspector inside the same mobile web desktop, with overlapping windows remaining powerful rather than chaotic.

The podcast app is not meaningfully different per user computer right now in the way the new source-lineage architecture intends. Users have different content/library state, but they are still seeing the same deployed platform frontend/runtime unless they are explicitly inside candidate/scoped routes. The app code is shared; the data differs by user.

## Current Architecture Reading

### Podcast

`frontend/src/lib/ContentViewer.svelte` is serving as the podcast app. That is the root product problem. It is a generic content viewer with podcast-specific branches, not a real podcast app.

Relevant anchors:

- `frontend/src/lib/ContentViewer.svelte:13-22` stores only library/search/import/loading state.
- `frontend/src/lib/ContentViewer.svelte:68-98` loads podcast library by filtering `/api/content/items?limit=100`.
- `frontend/src/lib/ContentViewer.svelte:100-132` imports an arbitrary RSS URL through `/api/content/import-url`.
- `frontend/src/lib/ContentViewer.svelte:134-165` searches `/api/podcast/search`.
- `frontend/src/lib/ContentViewer.svelte:174-178` opens a podcast by setting `item = content`; there is no navigation stack.
- `frontend/src/lib/ContentViewer.svelte:410-418` shows generic content header and `Open source`.
- `frontend/src/lib/ContentViewer.svelte:427-500` makes RSS import first-class on the landing screen.
- `frontend/src/lib/ContentViewer.svelte:501-530` renders feed description, Open in VText, all episodes, and native `<audio controls>`.
- `frontend/src/lib/ContentViewer.svelte:557-561` exposes provenance as default surface chrome.

Server search exists and is reasonable for v0:

- `internal/runtime/podcast.go:37` implements `/api/podcast/search`.
- `internal/runtime/podcast.go:86-104` defaults to Apple iTunes search.
- `internal/runtime/podcast.go:167` falls back to the user's local podcast library.

The server path is not the main issue. The UI has no app-grade playback, route model, library model, or per-user playback state.

### Logged-Out App Access

`frontend/src/lib/Desktop.svelte:509-519` blocks all app launches when unauthenticated and opens auth instead. This is too coarse. It treats opening any app as mutation.

That explains why logged-out users cannot open Podcast or other read/explore surfaces. Mutation should require auth; exploration should not.

### Bottom Bar And Window Shell

`frontend/src/lib/BottomBar.svelte` is a fixed bottom bar. It combines app launcher, minimized windows, prompt input, and connection status.

Relevant anchors:

- `frontend/src/lib/BottomBar.svelte:47-49` restores minimized windows but does not explicitly clear show-desktop mode.
- `frontend/src/lib/BottomBar.svelte:87-94` resizes the prompt input and can shrink in a clean Playwright run.
- `frontend/src/lib/stores/desktop.js:367-401` restores a single window and sets it active.
- `frontend/src/lib/stores/desktop.js:457-484` show-desktop mode stores `_showDesktopMinimized` and restores all show-desktop windows only through `toggleShowDesktop`.

Playwright did not reproduce "prompt bar grows and never shrinks" on a fresh test account: height went `56 -> 141 -> 56`. The user-observed bug is still credible, likely account/session/window-composition dependent. The more important architectural issue is that the bar is doing too much and is hard-coded as a bottom bar.

On mobile, the shell should still be the desktop. That is the product thesis: a powerful web desktop should come to the phone, not collapse into a gimped mobile app. The defect is not that windows overlap; the defect is that overlap is currently unmanaged on a touch viewport. Mobile needs desktop-grade window controls adapted for fingers: reliable focus/raise, minimize/restore, fit-to-screen, snap, overview, panel positioning, and possibly viewport pan/zoom.

### Trace

Trace has useful evidence data, but the mobile layout is not usable.

Relevant anchors:

- `frontend/src/lib/TraceApp.svelte:416-455` renders a sidebar trajectory list.
- `frontend/src/lib/TraceApp.svelte:457-1025` renders a very long main evidence/detail surface.
- `frontend/src/lib/TraceApp.svelte:1757-1841` and `1869-1895` responsive CSS stacks the sidebar over the main content and wraps dense sections.

This is not enough for mobile. Trace needs a mobile state machine: Runs, Summary, Timeline, Inspector. Evidence inspection should be a deliberate drill-in, not a 2,800px scroll below a trajectory list.

### Candidate Desktop

`frontend/src/lib/CandidateDesktopViewer.svelte:38-55` asks for a raw desktop ID. This is a diagnostic tool, not a product path. Candidate desktops should appear automatically when the current Trace, promotion, or run acceptance context has candidate evidence.

### VText

VText is structurally richer, but the flicker complaint is plausible from the code path:

- `frontend/src/lib/VTextEditor.svelte:222-226` syncs the contenteditable surface from rendered Markdown when not focused.
- `frontend/src/lib/VTextEditor.svelte:663-681` auto-advances to head changes when possible.
- `frontend/src/lib/VTextEditor.svelte:690-716` stream events update revision/head state.
- `frontend/src/lib/VTextEditor.svelte:1065-1073` serializes editor input and re-renders on blur.
- `frontend/src/lib/VTextEditor.svelte:1093-1094` reactively renders Markdown and calls `syncEditorSurface`.

That does not prove the flicker, but it gives the next sweep a concrete test target: streaming/head-change plus Trace open plus mobile shell switching.

## Confirmed Bugs And Risks

Severity definitions:

- P0: blocks ordinary use or acceptance evidence.
- P1: seriously degrades task completion or trust.
- P2: polish, consistency, or medium-term architecture.

### P0 - Podcast Has No App Navigation

Evidence: Playwright `POD-003`.

The feed/player view has no back button and no route stack. Opening a podcast replaces the library state with an item. On mobile, the user can get stuck in feed view with no clear path back to library or search.

Code: `frontend/src/lib/ContentViewer.svelte:174-178`, `427-530`.

Recommendation: split Podcast into an app-level state machine:

- Library
- Search
- Podcast detail
- Episode detail
- Now playing

Every non-root view needs a stable Back control.

### P0 - Podcast Player Lacks Standard Controls

Evidence: Playwright `POD-005`; user screenshots.

The app uses native `<audio controls>` per episode. It lacks:

- prominent play/pause for the selected episode;
- skip back/forward;
- speed control;
- scrubber with elapsed/remaining time;
- persistent now-playing bar;
- queue/current episode identity;
- progress persistence.

Code: `frontend/src/lib/ContentViewer.svelte:525-526`.

Recommendation: implement one app-owned player state and one sticky now-playing control. Episode rows should select/play into that state instead of embedding independent native audio players.

### P0 - Podcast Episode Scrolling Is Not Reliable On Mobile

Evidence: user report and Playwright `podcast_feed_mobile` metrics: `episodeCount: 18`, `audioCount: 18`, `firstAudioTop: 648`, but selected shell metrics showed `canScroll: false`.

Even if another ancestor scrolls in some cases, the practical result is bad: the player and episode list are hard to discover on mobile, and the user had to shrink font before the latest episode player appeared.

Recommendation: make the podcast content viewport explicit:

- app header fixed;
- body scrolls;
- now-playing bar sticky;
- episode list virtualized or simply scrollable in the app body;
- no provenance/source controls in the primary scroll path.

### P0 - Trace Mobile Is Not Inspectable

Evidence: Playwright `TRACE-001`.

Trace window rect on a 390px mobile viewport was only `268px` wide in the tested overlapping state. The Trace main content had `mainScroll: 2847`, and the inspector was at `inspectorTop: 2335`. The mobile summary exists, but critical evidence is still buried.

Recommendation: make Trace mobile a drill-in app:

- Runs tab: searchable/filterable trajectory list.
- Summary tab: selected trajectory state, run acceptance, evidence counts, rollback refs.
- Timeline tab: moments/events/messages.
- Inspector tab: selected moment detail, payloads, channel messages, artifacts.

Do not rely on one long page.

### P0 - Mobile Desktop Window Management Is Not Touch-Usable

Evidence: Playwright `SHELL-003`.

Trace and VText were both visible as overlapping windows:

- Trace: `left: 108`, `width: 270`, `height: 680`
- VText: `left: 12`, `width: 366`, `height: 764`

This is not a reason to remove overlapping windows on mobile. It is evidence that the desktop needs better touch window management. The user should be able to run Trace and VText together on a phone, raise either one, resize or fit either one, inspect the stack, and move between them without relying on accidental z-order or "hide desktop" rituals.

Recommendation: preserve desktop semantics and add mobile-capable controls:

- tap task/dock item raises and focuses the window;
- long-press or menu exposes fit, snap left/right/top/bottom, minimize, close;
- overview shows all open windows as live cards;
- active window gets enough width by default but can coexist with others;
- drag handles and title bars have touch-sized hit targets;
- optional desktop viewport zoom/pan lets users work with true desktop layouts when they want density.

### P0 - Logged-Out Read/Explore Is Over-Blocked

Evidence: Playwright `AUTH-001`.

Opening Podcast while signed out showed auth instead of the public/read-only app surface.

Code: `frontend/src/lib/Desktop.svelte:509-519`.

Recommendation: classify app actions by authority:

- Guest allowed: public landing, public search, public recommendations, public published VText, read-only Trace examples if exposed.
- Auth required: subscribe, import, persist playback state, mutate VText, launch worker/candidate actions, publish/promote.

The current "launch app means auth" rule is too blunt.

### P1 - Podcast Landing Prioritizes Advanced Import

Evidence: Playwright `POD-001`; user screenshot.

The first screen shows an RSS URL field and "Loading podcast artifacts..." instead of subscriptions, recommended podcasts, continue listening, and search.

Code: `frontend/src/lib/ContentViewer.svelte:427-500`.

Recommendation: default landing should be:

- Continue listening;
- Subscriptions;
- Search podcasts;
- Recommended or starter podcasts;
- Add by URL tucked behind Advanced or More.

### P1 - Podcast Debug/Provenance Chrome Displaces The Product

Evidence: Playwright `POD-004`; user screenshot.

`Open source`, `Open in VText`, and `Provenance` are not primary podcast controls. They are inspect/developer affordances. On mobile they take space before basic listening controls.

Recommendation: keep provenance and source accessible, but behind an inspect menu. "Open in VText" can be a secondary action for generating a radio brief, not default feed chrome.

### P1 - No Podcast Subscription Or Playback State Model

Evidence: code review and Playwright `POD-006`.

The current library is a filtered list of content items. There is no visible distinction between:

- subscribed feed;
- imported artifact;
- search result;
- played/unplayed episode;
- in-progress episode;
- completed episode;
- latest episode.

Recommendation: introduce a minimal per-user podcast state record:

- feed subscription/import identity;
- episode GUID/audio URL identity;
- playback position seconds;
- duration if known;
- completed/played flag;
- last played timestamp;
- playback speed preference.

### P1 - Prompt Bar Does Not Drive App Actions

The user expectation "play latest Lenny's podcast" is correct for Choir. The current frontend conductor path can open generic apps, but Podcast does not expose a command protocol that can search, import, choose latest episode, and start playback.

Recommendation: add app action contracts:

```text
podcast.search(query)
podcast.openFeed(feedId | feedUrl)
podcast.playEpisode(feedId, episodeId, autoplay)
podcast.playLatest(query, confidenceThreshold)
```

Conductor should route intent to app actions, not just open windows.

### P1 - Minimized Restore And Show Desktop Are Confusing

Evidence: user report; code review.

The user observed that tapping minimized apps does not always bring them back until "hide desktop" is toggled. The likely risk is interaction between `restoreWindow` and `showDesktopMode`. `restoreWindow` restores the individual window, but `showDesktopMode` can remain active with other windows still marked `_showDesktopMinimized`.

Recommendation: when restoring a minimized app from the bar, clear show-desktop mode or make show-desktop a derived state rather than a sticky global flag. On mobile, keep the desktop model but make restore/focus deterministic: tapping an app indicator should raise that desktop window immediately, even if show-desktop mode was active.

### P1 - Candidate Desktop Is A Raw Diagnostic Surface

Evidence: Playwright `CAND-001`.

The app asks for a raw candidate desktop ID. That is not acceptable as the normal candidate/promotion UX.

Recommendation: replace the blank/manual surface with contextual candidate cards:

- current Trace candidate;
- current promotion candidate;
- recent worker exports;
- failed candidate with blocker reason;
- open, inspect Trace, rollback/export actions.

Manual ID entry can remain in an advanced developer drawer.

### P1 - VText Flicker Needs A Focused Stress Test

Evidence: user report; code review.

VText may re-render the contenteditable surface on stream/head changes, blur, and revision refreshes. This can cause visible flicker, selection loss, or text jump, especially when Trace and VText are open together.

Recommendation: add a Playwright test that opens VText and Trace together on mobile, triggers streaming/head-change events, types into VText, and checks:

- no selection loss while focused;
- no full-surface re-render while focused;
- no repeated text flash on head-change when not focused;
- app switching preserves scroll and focus.

### P1 - Settings Themes Are Not Product-Grade

Evidence: code and staging screenshot review.

Settings is not the worst blocker, but theme editing is still too raw/config-oriented. The user sees "themes are mostly bad" because the shell does not have a coherent design system with tasteful presets and constraints.

Recommendation: defer deep theming until after shell mechanics are fixed, then build theme presets as first-class products:

- System;
- Classic;
- GNOME-like;
- Longhorn-inspired;
- High contrast;
- Paper/read mode.

Avoid direct JSON as the primary user path.

### P2 - Visual System Is Too One-Note And Too Windows-11-Like

The current shell leans heavily into dark purple/blue glass, rounded floating windows, and a bottom taskbar. The user wants a broader, more modular shell that can feel more Mac, GNOME, XFCE, Windows 2000/XP/Aero/Longhorn-inspired without being a knockoff.

Recommendation: do not start by reskinning everything. First introduce shell layout tokens:

- panel position: top, bottom, side;
- app switcher mode: dock, taskbar, overview;
- window mode: floating desktop everywhere, with fit/snap/overview/touch affordances on small screens;
- density: compact, standard, reading;
- debug chrome: off, inspect, developer.

Then themes can safely change aesthetics without breaking layout.

## Root Causes

### 1. Generic Content Viewer Masquerading As Apps

Podcast is not a podcast app yet. It is content import plus generic display plus a few RSS parsing branches. This is why hash strings, source links, VText buttons, and provenance appear before player controls.

### 2. Desktop Window Model Is Under-Instrumented For Touch

The floating-window concept is correct for Choir. The missing layer is mobile desktop ergonomics: touch-sized controls, predictable raise/focus, overview, snapping, fit-to-screen, and possibly zoom/pan. Without those controls, Trace and VText overlap in ways that feel broken even though the desktop model itself is the right product direction.

### 3. Auth Boundary Is Too Coarse

The product currently treats app launch as an authenticated action. This blocks public read/explore UX and makes logged-out users see the system as broken even when data could be public or ephemeral.

### 4. No App Command Protocol

The prompt bar can submit intent, but apps do not expose enough typed commands for conductor to execute user-level actions like "play latest Lenny's podcast".

### 5. Evidence UX Is Mixed With Ordinary UX

Trace, provenance, VText, and source controls are essential for trust, but they should not crowd primary app surfaces. The system needs inspect mode.

### 6. Candidate/Promotion Surfaces Are Still Operator Tools

Candidate Desktop and promotion evidence exist, but the UI still makes users input hashes/IDs or inspect internal-looking surfaces. That undermines trust and slows development loops.

## Recommendations For The Next UX Bug Sweep

### Sweep Shape

Do one focused MissionGradient UX sweep with these invariants:

- Do not mutate active computers directly for risky work.
- Use staging product-path Playwright evidence.
- Preserve logged-out read/explore wherever possible.
- Make mobile usable before polishing desktop.
- Keep debug/provenance available but out of primary app flows.
- Do not turn the sweep into a full podcast product mission.

### Highest-Value Work

1. Mobile web desktop: preserve floating windows, add reliable touch raise/focus, overview, snap/fit controls, restore behavior, and sane panel height.
2. Podcast: real app navigation, usable landing, scrollable episodes, sticky player controls, speed/seek/scrub, basic playback state.
3. Trace: compact desktop-pane controls with reachable inspector and run acceptance summary inside a normal window.
4. Auth: public/read-only app launch for guest mode, auth only at mutation/persistence boundaries.
5. Candidate: contextual candidate cards instead of raw ID input.
6. VText: flicker-focused Playwright reproduction and fix.

### Podcast Feature Boundary

The podcast app should become real enough to be a good automatic-computer app, but not so broad that it consumes the whole sweep.

Minimum standard features:

- library/subscriptions list;
- search and import from search results;
- Add by RSS hidden under Advanced;
- podcast detail with Back;
- scrollable episode list;
- one sticky now-playing bar;
- play/pause;
- seek back/forward;
- speed control;
- scrubber;
- played/unplayed and progress;
- resume last position;
- "Open in VText" moved to secondary action.

Defer:

- downloads/offline;
- chapters/transcripts unless already available;
- ratings/reviews;
- marketplace/discovery governance;
- notifications;
- complex queue management.

### Trace Feature Boundary

Minimum mobile Trace:

- selected run summary visible without scrolling past the trajectory list;
- run acceptance visible in first screen or first tab;
- evidence refs and rollback refs readable;
- agent/channel timeline accessible;
- selected moment inspector reachable;
- JSON/payloads wrap and can be expanded;
- Trace and VText can both be open as real desktop windows on phone, and the user can reliably raise, fit, snap, minimize, restore, and inspect either one.

### Fleet And Promotion Path

For urgent UX platform bugs today, "publish/promote to the fleet" still means platform code changes through git, CI, deploy, staging identity verification, and deployed Playwright acceptance. The per-user source-lineage/AppChangePackage path is not yet a complete fleet rollout path for frontend/runtime shell changes.

Near-term emergency bug fix path:

```text
platform branch/commit
-> CI
-> staging deploy
-> staging identity check
-> mobile Playwright acceptance
-> rollback ref
```

Medium-term fleet path:

```text
user/candidate app improvement
-> AppChangePackage
-> recipient computer rebuild/adoption proof
-> platform computer candidate/adoption
-> default-base/public-route proof
-> per-user adoption/migration policy
```

Do not claim that user-computer promotion updates the whole fleet until route/default-base and per-user adoption records prove it.

## Acceptance Checks For The Next Mission

Run these on staging with a 390x844 mobile viewport and at least one desktop viewport.

Podcast:

- Logged-out user can open Podcast public/explore landing without auth.
- Subscribe/import/persist actions request auth at the moment of mutation.
- Landing shows subscriptions/continue/search/recommendations before RSS import.
- RSS import is in Advanced/Add by URL.
- Search can find a known podcast and import/open it.
- Podcast detail has Back.
- Episode list scrolls inside the app body.
- Latest episode can be played from the first screen of the feed.
- Player has play/pause, seek back, seek forward, speed, and scrubber.
- Playback position persists after closing/reopening the app.
- Played/unplayed state is visible.

Trace:

- Mobile Trace keeps the desktop app model while using compact pane controls so the trajectory list, summary, timeline, and inspector are all reachable.
- Run acceptance/evidence/rollback summary is readable without hunting through a long page.
- Inspector is reachable and readable.
- Long JSON/payload text wraps or scrolls inside its own block without page overflow.
- Trace and VText can overlap on phone without becoming unusable; task/dock/overview controls foreground each reliably.

Shell:

- Bottom/prompt panel grows for multiline input and shrinks after clearing.
- Minimized app restore foregrounds the app without needing show-desktop toggles.
- Panel position is not hard-coded into app layout assumptions.
- App switcher works with Podcast, Trace, VText, Settings, and Candidate Desktop.

Candidate/Promotion:

- Candidate Desktop shows contextual candidate cards when candidate evidence exists.
- Manual ID input is advanced-only.
- Trace links to candidate/export/promotion evidence without requiring copied IDs.

VText:

- Typing while stream/head events arrive does not flicker, lose selection, or overwrite focused text.
- VText and Trace can be used together through task/dock/overview focus controls.

## Recommended Mission Frame

Use this review as the bug inventory, then write a MissionGradient mission with one migrating objective:

Make Choir's mobile web desktop trustworthy enough that a user can open, listen, inspect, overlap, raise, resize, and switch apps without fighting the shell.

Suggested next mission:

```text
/goal Run docs/mission-mobile-ux-bug-sweep-v0.md as a Codex-operated MissionGradient mission:
repair the mobile web desktop substrate and two representative apps without boiling the ocean.
Use staging Playwright and product-path evidence to make Podcast usable as an ordinary app
and Trace usable as an evidence app: logged-out read/explore, floating-window mobile
desktop controls, reliable raise/focus/fit/snap/restoration, Podcast library/search/feed/player/progress,
Trace mobile drill-in/run-acceptance/inspector readability, and VText flicker regression
coverage. Land through git/CI/deploy for platform UX fixes, verify staging identity, and
finish with screenshots, DOM metrics, acceptance results, rollback refs, residual risks,
and the next realism axis. Do not hide product failures behind debug controls, manual IDs,
local-only proof, or platform deploy claims that are not verified on staging.
```

## Priority Verdict

The automatic newspaper should remain the strategic product direction, but the immediate blocker is the automatic computer's UX substrate. Users cannot focus on newspaper intelligence while app launch, mobile windows, playback, Trace, and VText are fighting them.

The next best hill is not "make podcasts great" and not "redesign everything." It is:

```text
mobile web desktop works
-> Podcast feels like a real app
-> Trace is inspectable
-> VText remains stable beside Trace
-> candidate/promotion evidence appears in product context
```

That sequence creates a stable foundation for the next automatic-newspaper work.
