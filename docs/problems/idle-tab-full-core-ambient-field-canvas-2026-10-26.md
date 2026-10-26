# An idle choir.news tab holds a full core: the ambient field canvas never stops (2026-10-26)

Recorded by FP0, 2026-10-26 02:4x, from a Sonnet investigation; documented before any fix. The owner, 01:5x: CPU pegged; the Comet helper at about 100 percent turned out to be the choir.news tab, with WindowServer at 63 percent. Mutation class of a fix: orange (desktop frontend). Diffs prototyped by the investigation: fixA (one rAF chain, 24 fps cap, pure render, time-based pulse), fixB (ambient canvas at dpr 1), fixC (freeze after 120 s without input, restart on input).


Scope: frontend at origin/main b0d0300d0. Read-only on the repo; own worktree in the scratchpad (removed afterwards); measurements are of the **production build** (`vite build` + `vite preview`) with the mocked backend from FP0's harness (signed in, mocked `/api/*`, `/api/ws` mocked through `page.routeWebSocket`). Real Chromium (Playwright chromium-headless-shell 1217) on the owner's Mac (the agents VM has no chromium and no nix CLI, and its libs for the downloaded browser are missing; its load average was also 8). Short single runs, `nice 10`.

## Cause (one sentence)
`ChoirField.svelte` (the ambient constellation canvas, mounted unconditionally in `Desktop.svelte:1629`, signed in or out) runs an uncapped `requestAnimationFrame` loop forever. That single canvas is ~100% of the idle cost (measured: with it disabled the idle page is ~0.1% CPU). Two bugs in it make it worse over time.

## Measured numbers (MEASURED, 1440x900 @2x unless noted, percent of ONE core, 20-30 s windows, after a 5 s settle)
CDP `SystemInfo.getProcessInfo` cpuTime deltas for renderer / GPU process; `Performance.getMetrics` for main-thread TaskDuration.

| scenario | main-thread task | renderer proc | GPU proc | rAF/s |
|---|---|---|---|---|
| idle, no windows (baseline, prod) | 8.4 | 10.8 | 42.7 | 60 |
| idle, 3 windows (files, settings, slides) | 7.2 | 10.0 | 51.4 | 60 |
| idle, WebSocket failing (closed on connect) | 7.7 | 10.1 | 39.6 | 60 |
| page hidden (visibilityState overridden + visibilitychange) | 0.15 | 0.3 | 0.4 | 0 |
| prefers-reduced-motion emulated (should be a still image) | 7.4 | 9.6 | 40.7 | 60 (BUG, see B2) |
| after 5 window resizes | 9.1 | 11.7 | 45.8 | 360 (6 loops) |
| after 20 window resizes | 12.8 | 14.4 | 56.0 | 1257 (21 loops) |
| bigger window 2056x1200 @2x | 17.2 | 19.2 | **87.8** | 55 (cannot keep 60) |
| ChoirField disabled (getContext returns null for `.choir-field`) | 0.04 | 0.14 | 0.29 | 0 |
| ChoirField disabled + 3 windows | 0.07 | 0.20 | 0.30 | 0 |
| ChoirField disabled, backdrop-filter and all CSS animations also off | 0.05 | 0.16 | 0.31 | 0 |
| field running, backdrop-filter off | 7.7 | 9.9 | 41.3 | 60 |
| field running, chyron animation off | 9.2 | 11.5 | 45.7 | 60 |
| field running, both off | 8.2 | 10.4 | 41.1 | 60 |

Reading: (1) disabling the field takes the idle page to ~0; the prompt bar's `backdrop-filter`, the 38 s chyron marquee, icon-breathe and every other CSS animation cost nothing measurable on their own, and removing them does not reduce the field's cost. (2) JS is not the cost: ScriptDuration is 0.3-0.4% of a core and the CPU profile (15 s, 0.5 ms sampling, idle desktop) is 93% `(idle)`, 6.5% `(program)`, `clearRect`/`fill`/`stroke`/`arc` ~10 ms each. The cost is producing and compositing a full-viewport canvas surface 60 times a second: the GPU process, then the renderer's paint/commit. (3) The cost scales with canvas area: 1440x900 -> 2056x1200 doubles it (GPU 43 -> 88%). (4) A hidden tab is free.

INFERRED (not measured; I have no 120 Hz or the owner's display): on a MacBook Pro ProMotion display rAF runs at up to 120 Hz, which doubles the frame count; add a larger canvas than 1440x900 and you are at or past one full core in the GPU helper ("renderer/helper" in the owner's words) and a heavily loaded WindowServer, which composites the page 60-120 times a second. Leaked loops (B1) add on top: each costs about +0.2% renderer and +0.7% GPU at the baseline size, so this is a secondary multiplier, not the main cause. I did not reproduce a 100% renderer in the mock; what the mock lacks (real event streams, real windows content) is covered by the inventory below, which found no other idle activity.

## The two bugs inside ChoirField (MEASURED)
- **B1 duplicate loops.** `draw()` re-arms itself on its first line (`ChoirField.svelte:192`). `onResize` calls `draw(performance.now())` directly (line 287) after the 160 ms debounce, which starts a second, independent rAF chain; `raf` only remembers the last id, so `stop()` can never cancel the others. Every window resize / zoom / display change / devtools toggle adds a permanent loop. 20 resizes -> 21 loops measured (1257 draws/s). Hide/show cycles do not leak (each stop/start pair cancels and starts one).
- **B2 reduced motion is not honoured.** For `prefers-reduced-motion` the code intends "draw one still frame" (lines 306, 310) but `draw()` always schedules the next frame, so the loop starts anyway. Measured: reduced-motion emulation still runs 60 rAF/s and 40% GPU.
- Minor: pulses advance by a fixed `p.sp * 16.6` per frame (line 226), so they move 2x faster on a 120 Hz display. Per-frame `nodes.map` allocates ~20-30 objects (heap stayed flat at ~3 MB over 30 s; no leak seen).

## Inventory of everything continuous or periodic in frontend/src (STATIC read, plus MEASURED zero where noted)
| what | file:line | stops when hidden? | stops when nothing changes? | idle cost |
|---|---|---|---|---|
| ChoirField rAF loop (ambient desktop field) | ChoirField.svelte:191-192, 271-273 | yes (visibilitychange, :290) | NO, runs forever; leaks on resize (:287); runs under reduced motion (:306,310) | the whole problem: 8% main / 11% renderer / 43% GPU |
| ChoirField resize debounce | ChoirField.svelte:283-288 | n/a | event-driven | adds a loop each time |
| `choir-chyron` marquee, 38 s linear infinite (prompt bar) | PromptSurface.svelte:570-580 | browser throttles | no | ~0 measured |
| `choir-icon-breathe` 4.2 s infinite (active desktop icon) | desktop-motion.css:138-149 | browser throttles | no | ~0 measured (3-window run) |
| `choir-prompt-routing` 0.9 s infinite | desktop-motion.css:181-195 | n/a | only while routing | 0 idle |
| `work-pulse` 1.1 s, `mail-shimmer`, spinners (`choir-loading-orbit`, `spin`, `app-spinner-spin`, `slides-spin`, `auth-spin`, `recover-spin`, `recovery-spin`), `boot-cursor-blink` | TextureEditor:2143, MailApp:3180/3192, App:623, FileBrowser:977, AppHost:148, SlidesApp:544, AuthEntry:461, RecoverRoute:281, RecoveryRequestForm:249, Desktop:1917 | browser throttles | only while the loading/working state shows | 0 idle |
| backdrop-filter on large layers | PromptSurface:334 (fixed, pill), DeskSheet:98, App.svelte:645, app.css:360/372; per-app toolbars (Epub/Audio/Video/Pdf/Image/Texture*) | n/a | n/a | measured: removing it changes nothing at idle; it is not the WindowServer cause by itself, but a blur sits over an animating canvas whenever the field runs, so the compositor must re-blur every frame (cannot isolate cheaply; with the field off there is nothing to re-blur) |
| WebSocket `/api/ws` reconnect | Desktop.svelte:975-1010, 1046-1078 | no (not tied to visibility) | n/a | measured with a failing socket: 1 + 5 attempts at 1,2,3,4,5 s then stops (:1053, `MAX_WS_RECONNECT_ATTEMPTS = 5`); NO tight loop. Not exponential and no recovery after the cap until reload (functional gap, not CPU). The "auth transient" path retries every 3 s indefinitely (:1066-1068) but costs one fetch |
| media progress flush timer | media-utils.js:154, 190 | flushes on hide | only while playing | 0 idle |
| TextureEditor published poll, 60 s | TextureEditor.svelte:1058-1059 | yes (checks visibilityState) | only with a Texture open | negligible |
| PodcastApp refresh, 30 min | PodcastApp.svelte:466 | no | only with Podcast open | negligible |
| AutopaperExpand poll | AutopaperExpand.svelte:51, 68 | no | stops when request is closed | negligible |
| lifecycle SSE per trajectory | lifecycle.js:88, 148-178 | resumes on visibility | only per active trajectory | event-driven |
| Texture document stream (EventSource) | texture.js:299 | n/a | event-driven | event-driven |
| ResizeObservers | FloatingDesktopIcons:274 (rAF-debounced :145), FloatingWindow:280, PromptSurface:181, TextureToolbar:155, PretextInlineDisclosure:141, PdfApp:309 | n/a | fire on size change only | measured layout = 0 and style = 0 at idle with 3 windows: no feedback loop |
| MutationObserver, setInterval in Desktop/stores | none found (setInterval only at TextureEditor:1058, PodcastApp:466) | | | |
| live events (`choir-live-event`) | live-events.js:42-52 | n/a | event-driven; no polling | 0 idle in mock (real event volume not tested) |
| other canvas | tetramark.ts/TetraMark.svelte are SVG/favicon (no loop found); Slides/Image/Pdf canvases are per-app, only with a window open | | | not measured |

Not tested: real server event streams, a real Texture/Mail window with live data (those are the unmeasured residual).

## Proposed fixes (smallest first; all in frontend/src/lib/ChoirField.svelte; prototyped and measured in the scratchpad worktree, repo untouched)
Diff: /private/tmp/claude-501/-Users-wiz-Dev-go-choir/99fb2dd5-a4f7-4cc5-a0f5-6ee541f24543/scratchpad/tabcpu/fixC.diff (cumulative A+B+C; fixA.diff and fixB.diff are the intermediate steps). Full file: tabcpu/ChoirField.fixC.svelte.

- **A (bugs + cap), mutation class orange (runtime behavior, frontend only), rollback = revert the commit.** Split `draw` into `frame` (the one rAF chain, with a 24 fps cap for the ambient density, i.e. 20 fps on a 60 Hz display) and a pure `render` (called from resize and the reduced-motion snapshot, so it never starts a second chain); make pulse speed time-based (`dtMs`). Measured on the same harness: idle 42.7% -> 14.3% GPU, 10.8% -> 4.1% renderer, 8.4% -> 2.9% main; 20 resizes now give the same as 1 (60 rAF callbacks/s, 20 draws/s, no leak); reduced motion now 0.3% / 0.4%; 3 windows 51% -> 22% GPU. (The 60 empty rAF callbacks per second remain; they are nearly free.)
- **B (ambient backing store at dpr 1, one line).** The field is drawn at opacity 0.22 behind the work and is blurry glow by design; `dpr = Math.min(devicePixelRatio, density === 'ambient' ? 1 : 1.5)`. Measured on top of A: GPU 14.3 -> 10.5%, 3 windows 22.2 -> 20.5%. Small extra gain; optional.
- **C (idle freeze, the one that makes the tab cost ~0 when left open).** After `IDLE_MS` (proposed 120 s) with no pointermove/pointerdown/keydown/wheel, `stop()` the loop (the canvas keeps its last frame); any input restarts it; visibility return calls the same path. Measured with a shortened idle window (8 s) on the production build: once frozen the tab sits at 1.8% main / 1.0% renderer / 1.2% GPU (the figure includes the first few seconds before the freeze), i.e. effectively the 0.2/0.3% floor. This is the fix for the owner's case (a tab left open for days): a 20 fps animation is still a permanent compositor client, a frozen canvas is not. Product trade: the "still moving after you look away" line in the component header would apply to the intro density only; ambient goes still when nobody is there. If the owner wants the desktop to stay alive for a person watching a second monitor, use a longer IDLE_MS or only gate on `document.hasFocus()`; FP0's call.
- Not needed on the evidence: removing backdrop-filter, removing the chyron or icon-breathe animations, changing the ws reconnect (it is bounded). Separate, non-CPU: the ws gives up for good after 5 attempts (Desktop.svelte:1053); worth a later fix (exponential backoff with a cap and a retry on `visibilitychange`/`online`), not part of this bug.

Suggested landing: A alone is a safe, tiny, behavior-preserving patch; A+C removes the symptom. Add one Playwright check to the existing harness (count rAF callbacks and `clearRect` calls per second on an idle desktop after 5 resizes: expect <= 25 draws/s, and ~0 after the idle window) as the verifiable artifact. Per CLAUDE.md, a behavior-changing frontend push needs the landing loop (CI, staging deploy, deployed acceptance).

## Artifacts (all under /private/tmp/claude-501/-Users-wiz-Dev-go-choir/99fb2dd5-a4f7-4cc5-a0f5-6ee541f24543/scratchpad/tabcpu/)
measure.mjs (harness; run: `node measure.mjs <base-url> <scenario,...> <secs> [profile]`), summ.py, run1.txt, run2.txt, run3-5197.txt (fix A), run3-5196.txt (fix A+B), fixA/fixB/fixC.diff, ChoirField.fixC.svelte, profile-idle-nowin.cpuprofile.
