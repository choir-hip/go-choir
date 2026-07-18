# Choir Design System

## 1. Atmosphere & Identity

Choir feels like a persistent personal computer for research work: quiet, dense, recoverable, and deliberate. The signature is a themed desktop shell where panels, windows, and public readers use the same variable-driven surfaces instead of separate marketing pages. Default Noir gives the desktop a soft aurora atmosphere (theme body background + light overlay drift) so the workspace reads as depth, not a flat void. Auth treats **Choir** as the brand hero above the sign-in headline.

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

Never add raw page-specific colors for app UI. Add theme tokens first when a new color role is necessary.

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

Use minimal motion on public legal/document routes. Interactive states should be immediate or use the theme `--choir-motion-fast` timing. Avoid layout animation.

## 7. Depth & Surface

Depth is mixed but tokenized: themed tonal surfaces plus light borders and existing shell shadows. Legal/public readers should use the same `--choir-surface-pane`, `--choir-surface-card`, `--choir-border`, and `--choir-window-shadow` vocabulary as existing public publication pages.
