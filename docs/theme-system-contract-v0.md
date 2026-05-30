# Theme System Contract v0

## Problem

Choir's current frontend theme system is not comprehensive enough for durable
multi-theme product work. The schema-v2 theme presets exist, but app and shell
CSS still contain many hard-coded design colors. The result is visible theme
fragmentation: Carbon Fiber Kintsugi can be selected while Email, Trace,
VText, desktop chrome, and app-specific controls still expose blue/cyan Future
Noir residue.

The transparency bug exposed the deeper design failure. A global rescue layer
treated structural panes as soft decorative panels, which made foreground
windows visually blend with windows behind them. The first repair then exposed
another gap: opaque app surfaces must be theme-native and must not fall back to
Future Noir colors.

## System Review

The current system has four layers:

- `frontend/src/lib/theme.ts` owns preset theme records and emits CSS
  variables.
- `frontend/src/app.css` applies global shell/app rescue styling with broad
  selectors.
- `AppSurface` wraps each registered app and provides the app host.
- Individual app components still define many local colors in their `<style>`
  blocks.

The registry already has a good app protocol shape: app identity, window
policy, preview policy, and surface kind are centralized. The theme contract is
weaker: `surface` is only a broad shell kind (`standard`, `document`, `media`,
`terminal`) and the available CSS variables are mostly palette nouns such as
`panel`, `panelSoft`, `accent`, and `selected`.

That forces app authors to make palette decisions locally. Over time, those
local choices become hidden theme policy.

## Cognitive Transforms

### 1. Name The Real Object

The real object is not a palette. It is a product-wide visual protocol between
theme authors, shell authors, and app authors.

If the contract only says "here are colors," every app must become a mini theme
engine. If the contract says "here are semantic surfaces and states," apps can
express structure without owning the theme.

Changed action: build semantic tokens first, then migrate app CSS to those
tokens.

### 2. Depth Extraction

The shallow version of theming is "make the colors match." The deeper version
is "make visual roles explicit enough that rendering, accessibility, and app
composition stay correct across arbitrary themes."

Load-bearing variable: whether a style represents a role (`surface-pane`) or a
literal color (`rgba(15, 23, 42, 0.72)`).

Changed action: define opaque vs soft surfaces as separate roles, and forbid
raw design colors outside the theme module.

### 3. Via Negativa

Do not add more per-app rescue selectors for Carbon Fiber Kintsugi. More rescue
selectors would make the current bug class harder to see and would keep theme
logic scattered.

Changed action: add a scanner/test that makes hard-coded app design colors
visible as a regression.

### 4. Product-Path Translation

From the user's perspective, selecting a theme should be a durable state change
that changes the whole computer. It should not look like a theme painted over a
different product.

Changed action: theme persistence and app-state persistence are separate, but
theme application must be universal across all currently open apps, restored
apps, and logged-out preview apps.

## Contract

Theme authors own these semantic variables:

- `--choir-surface-app`: full app backings.
- `--choir-surface-pane`: structural panes, sidebars, readers, and toolbars
  that must occlude windows behind them.
- `--choir-surface-card`: decorative cards and list rows.
- `--choir-surface-control`: buttons, inputs, segmented controls, and command
  controls.
- `--choir-surface-inset`: nested low-emphasis wells and code/preformatted
  panels.
- `--choir-state-selected`: selected rows, active tabs, and active window
  controls.
- `--choir-state-hover`: hover affordances.
- `--choir-state-focus`: focus rings and active glows.
- `--choir-text-primary`, `--choir-text-muted`, `--choir-text-subtle`,
  `--choir-text-accent`, `--choir-text-on-accent`: text roles.
- `--choir-status-success`, `--choir-status-warning`,
  `--choir-status-danger`: semantic status roles.
- `--choir-status-success-soft`, `--choir-status-warning-soft`,
  `--choir-status-danger-soft`: status backgrounds.

Legacy variables such as `--choir-panel`, `--choir-panel-soft`,
`--choir-accent`, `--choir-selected`, and `--choir-success` remain aliases for
compatibility, but new app CSS should not reach for them first.

## Migration Strategy

1. Extend `theme.ts` to emit the semantic token protocol while preserving
   existing palette variables.
2. Update global shell/app CSS to use semantic tokens.
3. Migrate component style blocks from raw design colors to semantic tokens.
4. Keep true non-theme values only where the value is not a design color:
   `transparent`, `currentColor`, shadows whose color is derived from theme
   tokens, and media-intrinsic black for video/canvas wells.
5. Add a frontend test that fails if hard-coded design colors reappear outside
   `theme.ts` and explicitly documented exceptions.

## Acceptance

- Carbon Fiber Kintsugi has no visible Future Noir blue/cyan residue in common
  shell and app surfaces.
- London Salmon and Future Noir still pass existing theme tests.
- Restored overlapping windows remain opaque before focus.
- `rg`/test coverage shows app and shell CSS no longer contain hard-coded
  design colors outside the theme module and documented exceptions.
- The app-building docs tell future app authors which semantic tokens to use.

