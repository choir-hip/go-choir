# UI/UX Polish Evidence — 2026-07-17

Mutation class: orange (frontend visual behavior only; no protected surfaces).
Rollback path: revert the commit on `claude/choir-ui-ux-improvements-xkokac`.

Screenshots captured against the local Vite dev server with `GET /auth/session`
mocked to `{ authenticated: false }`, so the signed-out public desktop renders
without touching any protected route. Viewports: 1440×900 desktop and 390×844
mobile.

## Changes evidenced

1. **Preview window no longer covers the desktop icon rail** — the signed-out
   Choir Preview window previously spawned at ~6vw, truncating the icon labels
   ("Web Lens", "Compute Monitor", "Universal Wire"). It now clears the rail.
   Compare `before/01-desktop-signed-out.png` vs `after/01-desktop-signed-out.png`
   and the Files window scene in `04-app-window.png`.
2. **Layered desktop background for the default Futuristic Noir theme** —
   accent/accent2 radial gradients plus a vertical depth gradient replace the
   flat `#050912` fill (`themeBodyBackground` in `frontend/src/lib/theme.ts`).
3. **Prompt command field focus affordance** — the command field gains an
   accent focus ring via `:focus-within`, and placeholder text uses the subtle
   text token, making the prompt bar discoverable (visible in
   `after/01-desktop-signed-out.png`).
4. **Window and auth-overlay entrance motion** — floating windows and the
   sign-in overlay ease in (240–300 ms, `prefers-reduced-motion` disables all
   of it). Motion is not visible in stills; see the keyframes in
   `FloatingWindow.svelte` and `App.svelte`.

`npm run build` passes on this tree. Full Playwright acceptance runs in CI
(the local container has no Go service stack).
