# Choir Shell Redesign — Hard Cutover Codex Brief

Repository: `github.com/choir-hip/go-choir`
Target frontend: `frontend/src`
Date: 2026-05-25
Status: revised hard-cutover brief

## 0. Correction and north star

This is not a compatibility pass. Do not keep the old bottom-only component name, bottom-only CSS variables, or bottom-only test selectors. A shell surface that can live at the top or bottom must not be named as a bottom bar. The old file should be renamed or replaced, and the tests should move with it.

The new primitive is **PromptSurface**. It contains the Desk/TetraMark button, window/app tray, command prompt, agent/audio affordance, and online indicator. It can sit at the bottom or at the top. Its Desk button opens a half-height-ish sheet upward when the surface is bottom-mounted, and downward when the surface is top-mounted.

The hard cutover has four connected changes:

1. Replace the old bottom-only shell control with `PromptSurface.svelte` and `DeskSheet.svelte`.
2. Replace the four-square Desk glyph with the asymmetric TetraMark.
3. Replace the existing schema-v1 theme list with exactly three schema-v2 themes: `futuristic-noir`, `carbon-fiber-kintsugi`, and `london-salmon`.
4. Redesign Desktop Overview, Compute Monitor, Trace, and Podcast so they all consume the same theme tokens and design language.

## 1. Current repo facts to account for

The current frontend is Svelte/Vite. The relevant files are:

```txt
frontend/src/App.svelte
frontend/src/lib/theme.js
frontend/src/lib/preferences.js
frontend/src/lib/Desktop.svelte
frontend/src/lib/BottomBar.svelte              # existing file to remove/rename, not preserve
frontend/src/lib/FloatingWindow.svelte
frontend/src/lib/DesktopOverview.svelte
frontend/src/lib/desktop-overview-preview.js
frontend/src/lib/ComputeMonitorApp.svelte
frontend/src/lib/compute-monitor.js
frontend/src/lib/TraceApp.svelte
frontend/src/lib/trace.js
frontend/src/lib/PodcastApp.svelte
frontend/src/lib/SettingsApp.svelte
frontend/src/lib/stores/desktop.js
frontend/tests/mobile-real-desktop-overview.spec.js
frontend/tests/desktop-overview-heavy-session.spec.js
```

Important observed implementation constraints:

`App.svelte` already owns root-level theme application and persistence. Preserve that authority, but move it to schema v2. It currently calls `applyThemeToElement(document.documentElement, normalized)` and listens for `choir-theme-change`; keep that event route, but normalize old stored themes to the new default.

`theme.js` is schema v1 and currently exposes a narrow token set plus several legacy aesthetic presets. Replace it rather than extending it.

`Desktop.svelte` imports and renders the old shell control component. Change the import to `PromptSurface` and pass through theme/layout placement. Do not leave a component or comment named around bottom-only placement.

`FloatingWindow.svelte`, `Desktop.svelte`, and `stores/desktop.js` currently reserve bottom-only geometry. Replace the reservation with top/bottom prompt-surface geometry.

`DesktopOverview.svelte` already has the event/action API needed by `Desktop.svelte`: close, focus, minimize, close window, suspend window, suspend background, open compute monitor, keep active only, clear saved windows. Keep those events, but redesign presentation.

`desktop-overview-preview.js` already has the right safety idea: live previews are limited to 3 on mobile and 6 on desktop. Keep the decision logic, but make mobile cards the primary interface instead of tiny screenshots.

`ComputeMonitorApp.svelte` currently overlaps with Overview by listing windows. Keep safe recovery actions, but move window management out of the primary UI. Compute Monitor should become temporal and numeric: charts, gauges, contributors, events.

`TraceApp.svelte` already has the data for trajectories, agents, edges, moments, acceptances, search stats, and inspector details. The redesign is mostly a view-model and CSS refactor.

`PodcastApp.svelte` already has search, import, library, feed parsing, episode selection, playback, seek, and progress persistence. The redesign should make it sparse rather than adding features.

## 2. File-level hard cutover plan

### 2.1 Shell files

Make these changes:

```txt
DELETE or RENAME frontend/src/lib/BottomBar.svelte
ADD frontend/src/lib/PromptSurface.svelte
ADD frontend/src/lib/DeskSheet.svelte
ADD frontend/src/lib/TetraMark.svelte
```

The safest implementation path is to rename the old file to `PromptSurface.svelte` and immediately replace bottom-only naming inside it. Then extract the sheet portion into `DeskSheet.svelte`.

Do not preserve these names in production code:

```txt
BottomBar
bottomBarEl
bottomBarHeight
bottom-bar
bar-left / bar-center / bar-right as public concepts
--choir-bottom-bar-height
data-bottom-bar
data-bottom-user
data-bottom-logout
data-connection-status
```

These can appear only in transitional comments in the PR description, not in final code.

New names:

```txt
PromptSurface
promptSurfaceEl
promptSurfaceSize
prompt-surface
surface-left / command-field / surface-right, or better semantic names
data-prompt-surface
--choir-prompt-surface-size
--choir-prompt-surface-top-offset
--choir-prompt-surface-bottom-offset
data-prompt-surface-user
data-prompt-surface-logout
data-online-indicator
```

### 2.2 `Desktop.svelte`

Change:

```svelte
import BottomBar from './BottomBar.svelte';
```

To:

```svelte
import PromptSurface from './PromptSurface.svelte';
```

Render:

```svelte
<PromptSurface
  {currentUser}
  {authenticated}
  placement={theme?.layout?.promptSurfacePlacement || 'bottom'}
  promptDisabled={!desktopReady}
  {promptPlaceholder}
  {promptStatus}
  on:logout={handleLogout}
  on:authrequest={() => requestAuth({ kind: 'sign_in' })}
  on:promptsubmit={handlePromptSubmit}
  on:launchapp={handleLaunchApp}
  on:showoverview={handleShowDesktopOverview}
  on:showdesktop={handleShowDesktop}
/>
```

Pass `theme={currentTheme}` from `App.svelte` into `Desktop.svelte`, or pass only `promptSurfacePlacement`. Passing the whole normalized theme is cleaner because the shell can react to layout as a first-class product state.

Update `.desktop-area` so it does not subtract a bottom-only bar. Use prompt surface offsets:

```css
.desktop-area {
  position: relative;
  height: 100dvh;
  padding-block-start: var(--choir-prompt-surface-top-offset, 0px);
  padding-block-end: var(--choir-prompt-surface-bottom-offset, 64px);
}
```

The window coordinate space may still be absolute over the viewport, but geometry helpers must respect both offsets.

### 2.3 `PromptSurface.svelte`

PromptSurface owns:

- TetraMark Desk button.
- Window tray / shelf.
- Command prompt textarea.
- Agent chyron.
- Optional voice/audio button.
- Online indicator.
- DeskSheet open/closed state.
- Publishing CSS geometry variables for top/bottom reservation.

It must set:

```txt
--choir-prompt-surface-size
--choir-prompt-surface-top-offset
--choir-prompt-surface-bottom-offset
document.documentElement.dataset.promptSurfacePlacement
```

When `placement === 'top'`, top offset equals measured surface height and bottom offset is zero. When `placement === 'bottom'`, bottom offset equals measured surface height and top offset is zero.

The root element should be:

```svelte
<div
  class="prompt-surface placement-{normalizedPlacement}"
  data-prompt-surface
  data-placement={normalizedPlacement}
  data-desk-sheet-open={sheetOpen ? 'true' : 'false'}
>
```

### 2.4 `DeskSheet.svelte`

DeskSheet is not a menu panel floating out of a bottom bar. It is a sheet attached to the PromptSurface.

Bottom mode:

```css
.desk-sheet.placement-bottom {
  bottom: calc(var(--choir-prompt-surface-size) + max(18px, env(safe-area-inset-bottom)));
}
```

Top mode:

```css
.desk-sheet.placement-top {
  top: calc(var(--choir-prompt-surface-size) + max(18px, env(safe-area-inset-top)));
}
```

Use approximately half-height, with mobile allowed to reach 62dvh if needed:

```css
height: min(var(--choir-desk-sheet-height, 56dvh), calc(100dvh - var(--choir-prompt-surface-size) - 28px));
```

DeskSheet should include:

- Desktop Overview hero row.
- App grid from `APP_REGISTRY`.
- Show desktop.
- Auth/sign-out section.

Keep it simpler than the current app grid. This is not a Windows Start menu.

### 2.5 `FloatingWindow.svelte`

Replace bottom-only measurement with prompt-surface reservation.

Old conceptual shape:

```js
const bottomBar = document.querySelector('[data-bottom-bar]');
const fromTheme = getComputedStyle(document.documentElement).getPropertyValue('--choir-bottom-bar-height');
```

New conceptual shape:

```js
function readPromptSurfaceMetrics() {
  const surface = document.querySelector('[data-prompt-surface]');
  const style = getComputedStyle(document.documentElement);
  const size = surface?.offsetHeight || Number.parseFloat(style.getPropertyValue('--choir-prompt-surface-size')) || 64;
  const placement = document.documentElement.dataset.promptSurfacePlacement || surface?.getAttribute('data-placement') || 'bottom';
  return {
    size,
    topOffset: placement === 'top' ? size : 0,
    bottomOffset: placement === 'bottom' ? size : 0,
  };
}
```

Then clamp normal windows against:

```js
maxNormalHeight = viewportHeight - topOffset - bottomOffset - viewportMargin * 2;
maxRenderedY = viewportHeight - bottomOffset - renderedHeight - viewportMargin;
renderedY = clamp(y, viewportMargin + topOffset, maxRenderedY);
```

Maximized windows should fill the workspace excluding whichever edge has PromptSurface:

```css
.window.maximized-or-style-equivalent {
  top: var(--choir-prompt-surface-top-offset, 0px);
  height: calc(100dvh - var(--choir-prompt-surface-top-offset, 0px) - var(--choir-prompt-surface-bottom-offset, 64px));
}
```

### 2.6 `stores/desktop.js`

Replace:

```js
const BOTTOM_BAR_HEIGHT = 56;
```

With:

```js
const PROMPT_SURFACE_FALLBACK_SIZE = 64;
```

Add a helper that reads prompt-surface placement and offsets when the DOM exists. Use it inside `getViewportMetrics()` and `constrainWindowGeometry()`.

This is a product-level behavior change, not just CSS. New windows should not spawn underneath a top-mounted prompt surface.

## 3. Selector hard cutover

Update tests. Do not keep old selector aliases just to make tests pass.

| Old | New |
| --- | --- |
| `BottomBar.svelte` | `PromptSurface.svelte` |
| `[data-bottom-bar]` | `[data-prompt-surface]` |
| `[data-show-desktop-btn]` | `[data-desk-menu-button]` |
| `[data-start-button]` | `[data-desk-menu-button]` |
| `[data-desk-button]` | `[data-desk-menu-button]` |
| `[data-start-menu]` | `[data-desk-sheet]` |
| `[data-desktop-menu]` | `[data-desk-sheet]` |
| `[data-start-app]` | `[data-desk-sheet-app]` |
| `[data-start-app-id]` | `[data-desk-app-id]` |
| `[data-window-indicator]` | `[data-window-tray-item]` |
| `[data-bottom-user]` | `[data-prompt-surface-user]` |
| `[data-bottom-logout]` | `[data-prompt-surface-logout]` |
| `[data-connection-status]` | `[data-online-indicator]` |

Preserve stable app/window selectors where they still describe reality:

```txt
data-window
data-window-id
data-window-app-id
data-window-active
data-desktop-overview
data-overview-card
data-overview-focus-window
data-compute-monitor-app
data-trace-app
data-podcast-app
```

### 3.1 Required new tests

Add or update tests for:

1. Bottom placement: PromptSurface at bottom, sheet opens upward.
2. Top placement: PromptSurface at top, sheet opens downward.
3. Mobile top placement: windows do not spawn under the top surface.
4. Mobile bottom placement: windows do not spawn under the bottom surface.
5. TetraMark is present inside `[data-desk-menu-button]`; four-square glyph is absent.
6. Theme hard cutover: old schema-v1 stored preferences normalize to `futuristic-noir`.
7. All three themes apply to shell, window frame, Overview, Compute Monitor, Trace, and Podcast.

Example top-placement assertion:

```js
await page.evaluate(() => {
  window.dispatchEvent(new CustomEvent('choir-theme-change', {
    detail: {
      theme: {
        schema_version: 2,
        id: 'futuristic-noir',
        name: 'Futuristic Noir',
        layout: { promptSurfacePlacement: 'top' },
      },
    },
  }));
});
await expect(page.locator('[data-prompt-surface][data-placement="top"]')).toBeVisible();
await page.locator('[data-desk-menu-button]').click();
await expect(page.locator('[data-desk-sheet].placement-top')).toBeVisible();
const boxes = await page.evaluate(() => {
  const surface = document.querySelector('[data-prompt-surface]').getBoundingClientRect();
  const sheet = document.querySelector('[data-desk-sheet]').getBoundingClientRect();
  return { surfaceBottom: surface.bottom, sheetTop: sheet.top };
});
expect(boxes.sheetTop).toBeGreaterThanOrEqual(boxes.surfaceBottom - 1);
```

## 4. TetraMark

Use the supplied `TetraMark.svelte` and SVG/PNG assets. The mark is intentionally asymmetric and only partly legible. Do not simplify it into a triangle, grid, launchpad, recycling symbol, or Penrose triangle.

Final asset set:

```txt
icons/tetra/raw/tetra-mark-simple.svg
icons/tetra/raw/tetra-mark-extracted.svg
icons/tetra/buttons/futuristic-noir.svg
icons/tetra/buttons/futuristic-noir.png
icons/tetra/buttons/carbon-fiber-kintsugi.svg
icons/tetra/buttons/carbon-fiber-kintsugi.png
icons/tetra/buttons/london-salmon.svg
icons/tetra/buttons/london-salmon.png
```

For production UI, prefer inline `TetraMark.svelte` using `currentColor`. Use the button SVG/PNG variants only for static mockups, favicons, or theme previews.

The final centered version preserves horizontal centroid centering, then shifts the mark down 20px for optical centering. Carbon Fiber Kintsugi and London Salmon variants have no extra faint curved or straight guide lines.

## 5. Theme hard cutover

### 5.1 Schema v2

Replace `frontend/src/lib/theme.js` with the schema-v2 structure in `design-tokens/theme-presets-v2.js`.

Exactly three presets:

```txt
futuristic-noir
carbon-fiber-kintsugi
london-salmon
```

No `system-noir`, `next-workstation`, `classic-mac`, `aqua-glass`, `frutiger-aero`, `gtk-slate`, or `y3k-console` in `THEME_PRESETS` after cutover.

### 5.2 Legacy stored preferences

If a stored theme has `schema_version !== 2`, or has an id outside the three new ids, normalize it to `DEFAULT_THEME`. Do not try to visually preserve legacy themes.

### 5.3 Theme tokens

Use these tokens across shell, windows, sheets, apps, charts, controls, and status chips:

```txt
--choir-bg
--choir-bg-2
--choir-panel
--choir-panel-strong
--choir-panel-soft
--choir-fg
--choir-muted
--choir-subtle
--choir-accent
--choir-accent-2
--choir-success
--choir-warning
--choir-danger
--choir-border
--choir-border-strong
--choir-input-bg
--choir-selected
--choir-on-accent
--choir-prompt-surface-bg
--choir-sheet-bg
--choir-control-bg
--choir-tetramark-color
--choir-chart-1 ... --choir-chart-5
--choir-radius-control-sm
--choir-radius-control
--choir-radius-panel
--choir-radius-sheet
--choir-radius-pill
--choir-blur
--choir-shadow-soft
--choir-shadow-floating
--choir-shadow-glow
--choir-control-shadow
--choir-prompt-surface-size
--choir-prompt-surface-top-offset
--choir-prompt-surface-bottom-offset
--choir-desk-sheet-height
--choir-font-ui
--choir-font-display
--choir-font-mono
```

### 5.4 Theme meanings

**Futuristic Noir** is the default. Dark navy/black glass, luminous blue and cyan accents, sparse panels, and soft depth. This is the base language of the new PromptSurface.

**Carbon Fiber Kintsugi** is dark grey industrial material repaired with glowing gold joinery. The metaphor is that broken web surfaces are repaired by value-creating agents. It should feel precise, mechanical, and expensive.

**London Salmon** replaces “Market Broadsheet Salmon.” It is salmon-colored paper, oxblood/ink accents, a Savile Row bespoke suit, Downing Street argument, Eton/Oxbridge debate, and wry humor. It should not become cute pastel. Use serif display type for headings where practical.

### 5.5 Settings app

Settings should show three theme choices, not a large arbitrary JSON editor as the primary UI. For this cutover, remove or hide the JSON editor behind a development-only flag. The product contract is three coherent themes applied everywhere.

## 6. Desktop Overview redesign

Desktop Overview should be a switcher, not a dashboard.

Keep the existing events and data attributes for Overview-specific actions where they still describe reality, but change the layout:

Desktop layout:

```txt
header: Open windows / summary / close
left rail: filters (All, Live, Paused, Hibernated), groups, sort
main: active window hero card
below: other window cards with large thumbnails only where useful
footer: manage groups / drag hint
```

Mobile layout:

```txt
sheet header
summary line
active window hero card
background window list
compact recovery actions behind Manage
```

Mobile must not depend on a dozen tiny screenshots. A card with icon, title, app, state, last active, and actions is primary. Live previews are accents.

State labels:

```txt
Live        running and interactive
Paused      memory-light but resumable
Hibernated  saved state; reload likely required
Redacted    preview intentionally hidden
Heavy       restore-cost annotation, not identity
Mounted     body currently mounted
```

Remove `layer 1`, `layer 2`, etc. from user-visible copy unless it is in a debug-only mode. Replace with Current / Background / Paused.

## 7. Compute Monitor redesign

Compute Monitor should be a diagnostic surface, not another window manager.

Primary hierarchy:

```txt
Health header
Pressure over time chart
Restore weight gauge
Windows over time stacked/area chart
Top contributors by restore cost
Recent recovery events
Intervention panel
```

Move full window management to Desktop Overview. Compute Monitor can still show top contributors, but it should not replicate Overview’s window cards.

Use chart components where possible. If no true historical API exists yet, use derived local samples while the app is mounted. Suggested local shape:

```js
samples = [...samples.slice(-60), {
  t: Date.now(),
  pressure: computePressureScore(status, windows),
  live: liveCount,
  paused: pausedCount,
  hibernated: hibernatedCount,
  restoreWeight: heavyMounted / Math.max(1, heavyTotal),
}];
```

Safe recovery actions remain, but do not dominate the page. Hide unavailable dangerous actions instead of showing disabled “Kill process” or “Force reset computer.”

## 8. Trace redesign

Trace should be visual-first. The current data model is enough; make scanning easier.

Top row:

```txt
trajectory title / failed|live|completed pill / Copy logs
metric strip: agents, moments, delegations, findings, searches, started, duration
```

Main area:

```txt
left: trajectory list
center top: visual run graph
center bottom: timeline lane chart
right: inspector
```

Run graph requirements:

- Agent nodes and tool nodes use different shapes or borders.
- Delegation edges and data-flow edges use different strokes.
- Node cards include tiny status dot sequences for calls/moments.
- Failed nodes use warning color and should be visible at a glance.
- The inspector should not dominate the first scan.

Timeline requirements:

- Use horizontal lanes per agent/tool.
- Use bars for duration and dots for moments.
- Failures are red/danger dots or ticks.
- Selection links graph node, timeline item, and inspector detail.

Mobile:

Use one visual panel at a time: Runs, Graph, Timeline, Inspector. Do not render every verbal detail at once.

## 9. Podcast redesign

Podcast should be sparse.

Keep the existing capabilities but reduce the surface:

Desktop layout:

```txt
left: minimal navigation and subscriptions
center: show card + episode list
right: now playing
bottom/global: PromptSurface remains outside the app
```

Do not show every feature at equal weight. Search and Import RSS can exist, but should be secondary. Recommended podcasts should only appear when library is empty.

Episode rows:

```txt
art mark
episode title
author / date / duration
progress bar if played
play button
overflow menu
```

Now Playing panel:

```txt
art mark
title
speaker/author
waveform or progress
-15 / play-pause / +30 / speed
small details
```

Mobile:

Use a single-column flow: show header, episode list, sticky mini-player. Hide secondary metadata behind details.

## 10. App-wide theming rules

All major app root components should start with the same base:

```css
.app-root-class {
  color: var(--choir-fg);
  background: var(--choir-panel);
  font-family: var(--choir-font-ui);
}
```

Use local gradients only through theme variables. Avoid hardcoded blues and hardcoded dark panels in app CSS.

Window frame should also be retokenized. `FloatingWindow.svelte` should not have hardcoded `#1e1e2e`, `#181825`, `#333`, or `#3b82f6` except as fallbacks.

## 11. Acceptance checklist

A PR is acceptable when:

- `frontend/src/lib/BottomBar.svelte` no longer exists, or exists only as a deleted file in the diff.
- `Desktop.svelte` imports `PromptSurface.svelte`, not the old component.
- No production code uses `[data-bottom-bar]`, `.bottom-bar`, `--choir-bottom-bar-height`, `bottomBarHeight`, or `BOTTOM_BAR_HEIGHT`.
- PromptSurface works at the bottom and top.
- DeskSheet opens upward from bottom placement and downward from top placement.
- TetraMark appears in the Desk button.
- The old four-square Desk icon is gone.
- `theme.js` is schema v2 and exposes exactly three presets.
- Legacy stored themes normalize to `futuristic-noir`.
- Desktop Overview is switcher-first and mobile-friendly.
- Compute Monitor has charts/gauges and no longer acts as a second Overview.
- Trace is scannable from graph/timeline before text details.
- Podcast is sparse and calm.
- Existing behavior still works: prompt submit, app launch, window focus, minimize, restore, overview actions, auth prompts, theme persistence.
- Playwright tests use new selectors and include top-placement coverage.

## 12. Suggested implementation order

1. Add `TetraMark.svelte` and assets.
2. Replace `theme.js` with schema v2, update Settings theme UI.
3. Rename/replace old shell control with `PromptSurface.svelte` and `DeskSheet.svelte`.
4. Update `Desktop.svelte`, `FloatingWindow.svelte`, and `stores/desktop.js` for top/bottom prompt-surface reservation.
5. Update Playwright selectors and add top-placement tests.
6. Retokenize `FloatingWindow.svelte` and shell surfaces.
7. Redesign Desktop Overview.
8. Redesign Compute Monitor.
9. Redesign Trace.
10. Redesign Podcast.
11. Run `cd frontend && pnpm run build && pnpm exec playwright test --workers=1`.

## 13. Bundle contents

```txt
choir-redesign-hard-cutover-brief.md
codex/codex-implementation-prompt.md
components/TetraMark.svelte
components/PromptSurface.svelte.snippet
components/DeskSheet.svelte.snippet
components/prompt-surface-and-desk-sheet.css
components/ChartSparkline.svelte.snippet
design-tokens/theme-presets-v2.js
design-tokens/choir-theme-vars.css
tests/selector-hard-cutover-map.md
icons/tetra/raw/*
icons/tetra/buttons/*
icons/tetra/preview/*
reference-mockups/*
manifest.json
```
