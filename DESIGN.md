# Choir Design System

## 1. Atmosphere & Identity

Choir feels like a persistent personal computer for research work: quiet, dense, recoverable, and deliberate. The signature is a themed desktop shell where panels, windows, and public readers use the same variable-driven surfaces instead of separate marketing pages.

## 2. Color

Choir colors are theme variables generated from `frontend/src/lib/theme.ts`.

| Role | Token | Usage |
| --- | --- | --- |
| Background | `--choir-bg`, `--choir-body-background` | App and public-route canvas |
| Surface | `--choir-surface-app`, `--choir-surface-pane`, `--choir-surface-card` | Windows, panels, legal reader sections |
| Control | `--choir-surface-control`, `--choir-state-hover`, `--choir-state-focus` | Buttons, tabs, links with button affordance |
| Text | `--choir-text-primary`, `--choir-text-muted`, `--choir-text-subtle` | Body, secondary metadata, captions |
| Accent | `--choir-accent`, `--choir-accent-2`, `--choir-text-accent` | Links, focus, selected states |
| Border | `--choir-border`, `--choir-border-strong` | Reader sections, panel dividers |
| Status | `--choir-status-success`, `--choir-status-warning`, `--choir-status-danger` | State and error messaging |
| Field | `--choir-field-edge`, `--choir-field-node-far/mid/near`, `--choir-field-halo` | The ChoirField constellation (landing intro + desktop ambience) |
| Desk | `--choir-desk-texture`, `--choir-desk-management`, `--choir-desk-engineering`, `--choir-desk-research` | Desk orb identities in the ensemble act |
| Grain | `--choir-intro-grain` | Film grain over the intro |

Never add raw page-specific colors for app UI. Add theme tokens first when a new color role is necessary. `ChoirField` resolves its entire palette from CSS custom properties at draw time, so the canvas holds no literals either.

## 3. Typography

Font stacks come from theme variables: `--choir-font-ui`, `--choir-font-display`, and `--choir-font-mono`. Public pages use the UI font for body text and reserve display weight for document titles. Body text stays at or above `1rem`; captions and metadata stay near `0.78rem` only when secondary.

## 4. Spacing & Layout

Spacing follows the existing shell rhythm: compact controls, 8px panel radii for document/public cards, and constrained content widths. Public document routes use a maximum readable line length, sticky header, and responsive single-column mobile layout. Use `clamp()` for page padding where public routes need to fit both 375px and desktop widths.

## 5. Components

### Public Reader Shell

- **Structure**: sticky header, brand link, small navigation links, one constrained content panel.
- **Spacing**: header padding mirrors `universal-wire-public-reader`; content panel uses `clamp(1rem, 3vw, 2rem)`.
- **States**: links and buttons expose hover and focus-visible states through existing theme variables.
- **Accessibility**: route content is in `main`; document title is one `h1`; legal navigation uses real links.

## 6. Motion & Interaction

**Tokens come in two shapes, and picking the wrong one fails silently.**
A *composite* value (`--choir-motion-sheet`) carries duration and timing
together and is only safe as a whole value: `transition: box-shadow
var(--choir-motion-sheet)`. An *separated* pair (`--choir-duration-sheet`
plus `--choir-ease`) is required the moment a shorthand also names a
duration, because `opacity 0.8s var(--choir-motion-sheet)` expands to two
time values and voids the entire declaration. Nothing renders, no error is
logged, and the surface simply never moves. Any component that needs a
per-property duration composes the separated pair.

**Animate the individual transform properties, not `transform`.** Windows
and icons already own `transform` for drag, preview scaling and geometry.
`translate` / `scale` / `rotate` are separate properties and compose on top
of it, so motion added in `desktop-motion.css` can never fight an active
drag or a placement animation.

**Composed properties only** in shared motion: transform, opacity, filter,
plus colour and box-shadow. A window that reflows on open moves the user's
click target under their cursor.

**Reduced motion is a state, not a checkbox.** Every block collapses
durations to ~1ms rather than deleting the rule, so nothing is left frozen
mid-`from` keyframe. `prefers-reduced-motion` also *retires* the landing
film entirely rather than slowing it down — the argument the film makes is
made in full, in prose, by the signed-out preview document, so a visitor
who asked for stillness gets a readable page instead.

**Legal and document routes stay still.** They are reading surfaces.

### Landing intro

`LandingIntro.svelte` is the one surface with authored motion. Its rules:

- **Non-blocking by construction.** The scrim is `pointer-events: none`, so
  the public desktop underneath stays live, and any interaction with it
  retires the film. It can never trap a visitor or a test.
- **`overflow: clip`, never `overflow: hidden`.** `hidden` still creates a
  scroll container, and because every entrance animates with a transform
  the overlay ends up with hidden scroll range — at which point any
  `scrollIntoView` (a click, a focus, a test locator) drags the whole film
  upward and breaks every layout in it.
- **One rAF clock, and motion is a pure function of (act, localTime).**
  Entrances are declarative: an element declares a release time, the loop
  flips a flag once, CSS does the transition. No `setTimeout` chains, so
  scrubbing, pausing and replaying are frame-exact and idempotent.
- **Stacking band.** The film sits above the desktop plane (prompt surface
  10000) and below real dialogs (DeskSheet 9998/9999, auth overlay 20000).
  Every route to a dialog passes through an interaction that retires the
  film first, so nothing can be trapped beneath it.
- **Cost is a design constraint.** The first build spent ~90% of the frame
  budget on a full-viewport `backdrop-filter`, a `blur(110px)` on gradients
  that were already soft, and a masked animated lattice. Measured at 6fps
  under software rasterisation; all three removed, the same frame rate
  locks. Motion that costs a third of the budget is a tax, not a design.

## 7. Depth & Surface

Depth is mixed but tokenized: themed tonal surfaces plus light borders and existing shell shadows. Legal/public readers should use the same `--choir-surface-pane`, `--choir-surface-card`, `--choir-border`, and `--choir-window-shadow` vocabulary as existing public publication pages.
