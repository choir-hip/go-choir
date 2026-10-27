# An idle choir.news tab still holds a core after the field fix: two infinite CSS animations keep the compositor drawing (2026-10-27)

Recorded by a Sonnet investigation for FP0, 2026-10-27 about 02:30 UTC; documented before any fix, read-only on product code. Mutation class of the fix: orange (desktop frontend CSS and one class toggle). Follows `idle-tab-full-core-ambient-field-canvas-2026-10-26.md` (on branch `doc`, commit e9cb2b5cc).

Owner report (01:40 to 01:5x): the Comet (Chromium) helper sits at 98%+ on choir.news whatever the theme; only while the tab is visible (another macOS window focused: still 100%; another tab selected: calm). The deployed bundle has the freeze (`12e4` in app-*.js).

## Cause (measured, ranked)

**The idle freeze stops ChoirField but not the CSS animations. With any window open, the desktop keeps at least one infinite compositor animation, and a running compositor animation makes the GPU process redraw the whole page every frame (about 60 DrawFrame/s with zero main-thread work, zero rAF).** That matches "visible only, focus irrelevant, not script or network".

1. `choir-icon-breathe` (`desktop-motion.css:131-149`, from 2026-10-01): `[data-desktop-icon].icon-active .icon-emoji`, `scale` 1 to 1.05, 4.2 s infinite. Exists whenever a window is in front, i.e. whenever any app window is open. Alone: **33% of a core GPU** (london-salmon, 3 windows, field frozen); pausing only this one: 0.7%.
2. `choir-titlebar-sheen` (`app.css:~619`, themes of rounds 6/7, so the default theme aero-chrome-heart and the three siblings): `::after` on every window title bar, `translate` -46% to 240%, 7 s or 9 s infinite. One per open window. Alone: about **46 to 53%** (3 windows). Additive with (1).
3. backdrop-filter on windows roughly doubles (2) (aero, sheen on: 99.8 to 53.6 when it is turned off) and is irrelevant to (1) (london-salmon 33.5 to 32.9).
4. Nothing else: idle desktop with no window has no infinite animation at all and sits at 0.75 to 2.6% GPU after the freeze in every one of the 7 presets.

No regression in the freeze: ChoirField.svelte is byte-identical since 4785746f1; after about 120 s idle rAF/s and draws/s go to 0 in every run. No new setInterval or rAF loop in `frontend/src` since 4785746f1 (the only new rAF is the Reader's `renderRaf`, not on the desktop). The 10-26 doc's measurement hid this: it was taken with ChoirField running (dominant) and its "removing CSS animations changes nothing" was true only while the field drew; the 10-26 "field disabled + 3 windows = 0.3% GPU" was a build before the sheen, and with the field off the breathe row is also near 0 there, which says the breathe cost needs the (then) layer arrangement; today it is 33% with the field frozen (see Limits).

## Measured (mocked backend, production build, real Chromium on the owner's Mac, load average about 20 so absolute values are noisy; percent of one core; 15 s windows)

Default theme aero-chrome-heart, 1440x900 @2x:

| scenario | point | main task | renderer | GPU | rAF/s | field draws/s | infinite anims |
|---|---|---|---|---|---|---|---|
| idle, no windows | 30 s | 2.0 | 3.3 | 24.9 | 60 | 20 | 0 |
| idle, no windows | 150 s | 0.2 | 0.45 | 2.55 | 0 | 0 | 0 |
| 3 windows (Texture, Files, Settings) | 30 s | 1.7 | 3.1 | 99.4 | 29 (GPU saturated) | 18.7 | 4 (3 sheens + breathe) |
| 3 windows | 150 s | 0.16 | 1.2 | **99.2** | 0 | 0 | 4 |
| 3 windows, 2056x1200 @2x | 30 s | 2.9 | 4.8 | 94.7 | 38 | 17 | 4 |
| 3 windows, 2056x1200 @2x | 150 s | 0.3 | 2.6 | **64.9** | 0 | 0 | 4 |
| idle, no windows, 2056x1200 | 30 / 150 s | 2.7 / 0.3 | 3.9 / 0.5 | 29.1 / 2.65 | 60 / 0 | 20 / 0 | 0 |

All seven presets, idle, no windows, 1440x900 (30 s then 150 s, GPU %): aero-chrome-heart 24.9 / 2.55, futuristic-noir 24.9 / 2.41, carbon-fiber-kintsugi 19.4 / 1.82, london-salmon 14.8 / 0.75, frutiger-aero 25.4 / 1.59, vector-heart 15.8 / 2.09, liquid-metal 25.0 / 2.23. Renderer 0.3 to 0.5 at 150 s in all. Theme does not matter without windows, and every preset freezes.

3 windows, london-salmon (no sheen), 150 s: GPU **36.4**, renderer 2.1, 1 infinite animation (breathe), 600 DrawFrame per 10 s (about 60/s). The default theme with 3 windows: 295 per 10 s (GPU-bound).

A/B on the frozen, 3-window desktop (each row is one change, 8 to 10 s, GPU % of a core):

| default theme | GPU | | london-salmon | GPU |
|---|---|---|---|---|
| baseline | 99.8 | | baseline | 33.5 |
| sheen paused only | 99.5 | | sheen paused only | 33.0 |
| breathe paused only | 53.1 | | breathe paused only | **0.7** |
| backdrop-filter off only | 53.6 | | backdrop-filter off only | 32.9 |
| backdrop off + sheen paused | 45.5 | | both anims paused | 0.6 |
| both animations paused | **0.7** | | | |

The ranked attribution: breathe about 33 to 46 points, sheen about 46 to 53 points, backdrop-filter about 46 points of the default theme's total (it multiplies the sheen), everything else under 3.

## Real staging, signed in: NOT obtained (limits)

The b@c browser session does not exist (b@c is a CLI key, `~/.config/choir-qa/keys/b-c.key`; the saved Playwright states are the `*.storage.json` test computers). Five test accounts (kf-canary, fp8-jailer-1023b, batching-e2e-1022, res13-r5-1022, fp0-selfdev-1024a) and fp0-jobpark-1024b all load `/auth/session` 200 but `/api/shell/bootstrap` answers 503 or 502 for 25 to 180 s, so the desktop never became ready (`data-desktop-ready` absent): their computers are down. I did not use the owner's or the paper's session. So no real-site desktop was measured; the mocked desktop is the evidence, and the owner's real window set is the unmeasured residual. The 100% is reproduced in the mock as GPU-process time (page main thread idle), which fits the report that it appears in the helper.

Side findings from the stuck boot screen on fp0-jobpark-1024b (180 s, real staging): `GET /api/shell/bootstrap` once per second without backoff (278 requests in 3 min), the boot console grows the DOM (355 to 877 nodes in 2 min, then trimmed), `boot-cursor-blink` is an infinite animation that costs 1.2% GPU (0.3% paused), ChoirField freezes there as well (rAF 60 to 0 at about 85 s). Low cost, but a tab left on the "Computer is not answering" screen polls forever (`Desktop.svelte:~930-960`).

Not captured: paint regions (the devtools.timeline Paint events were empty in the compositor-only case, consistent with no repaint), a real Texture document with citations, a headed/ProMotion display (inferred worse at 120 Hz: DrawFrame would double).

## Smallest fixes and expected gains

1. **Pause CSS animations at the same idle point as the field** (about 6 lines): when ChoirField's idle timer fires set `document.documentElement.dataset.idle = '1'` and clear it in `onInput`/`onVisibility`; one rule in `app.css`: `:root[data-idle] *, :root[data-idle] *::before, :root[data-idle] *::after { animation-play-state: paused !important; }`. Measured stand-in (both animations paused on the frozen desktop): 99.8 to 0.7% (default), 33.5 to 0.7% (london-salmon), i.e. the whole remaining idle cost. Trade: the sheen and the breathing icon stand still after 2 idle minutes. Rollback: revert.
2. Lower cost, no trade: drop `choir-icon-breathe` and give the sheen a finite count (e.g. `animation-iteration-count: 3`) so nothing is infinite. Expected: same near-0 when idle, but the dock/titlebar lose the ambient life permanently.
3. Do not bother with removing backdrop-filter: it only changes the sheen's multiplier.
4. Separate: bootstrap poll backoff on the boot screen (one request per second forever).

Hypothesis to verify, not asserted: why one small scaling icon costs 33% of a core in the GPU process (a full-page composite per frame with the translucent field canvas, backdrop layers and a 22 px box-shadow glow under it); the A/B only shows the animations are the trigger, not the per-frame mechanism. The fix does not depend on it.

Landing: orange, frontend only; needs the landing loop (CI, staging deploy) and a deployed check that a signed-in desktop with windows reads under 5% GPU past 150 s (harness: `real.mjs` and `measure2.mjs`/`measure3.mjs`, kept outside the repo in `/Users/wiz/Dev/lanes/tabcpu2-harness/`).

Heresy delta: discovered: the 10-26 fix froze one animation source and left two infinite CSS animations; introduced: the sheen animation (round 6, 670f9ea50) added a second source on every window; repaired: none.
