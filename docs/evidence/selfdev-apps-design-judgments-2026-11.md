# Self-dev apps: design judgments (2026-11)

## slides-next (b@c, operation selfdev-00b0b46014c4d5a26b48768330980e2e, applied 2026-11-02T22:17:27Z)

Judge: the b@c monitor, read-only. Bar: docs/design/app-design-bar-2026-10-26.md checks 1-12, revised stage-fill spec (frame at least 90% of the binding stage dimension; stage = content area minus the 168px filmstrip).

### Capture path (stated plainly)

- A real-shell capture against the deployed b@c is NOT possible from here: the b@c API key is not a browser session (no passkey session), and choir.news serves a shared SPA bundle, not b@c's applied source. The platform's visual-capture tooling only returns the frozen candidate's `/` and `/read.html` captures.
- Used instead: the bar's own documented harness ("How the screenshots were made"): a scratch copy of frontend/ (scratchpad, not the repo) with only the five slides-next files from the applied patch overlaid, Vite dev server, Playwright Chromium, mocked `/auth/session`, `/api/desktop/state` (one restored slides-next window), `/api/preferences/theme`, `/api/media/recents`, and fixture PDF (6 pages, 16:9), PPTX (4), HTML (4). It renders the real shell, window chrome and app.css overrides. Caveat: the local shell is the cutover/data-system tree, not b@c's exact booted tree; and my window (1100x794) overlaps the dock, so the bottom edge of the player bar in desktop shots is partly under the dock (harness artifact, not scored).
- Screenshots (45): /private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad/sn-bc/judge/shots/ (`<theme>-<d|p>-<state>.png`, states empty, pdf, pptx, html, error, unsupported, pdf-slide3, plus d-present). Metrics: sn-bc/judge/metrics.json. Harness: sn-bc/judge/run.mjs.
- Not testable: "edit" and "add a slide". The request defines slides-next as open and present only; the app has neither feature. Reported as absent by spec, not as a fail.

### Verdict: FAIL (4 defects; the request's central geometry now passes)

Passes (measured/seen, 3 themes x 2 widths unless noted):
- Check 2/3/4 tokens: no hex/rgb/hsl literals and no px radii in slides.css, App.svelte, Glyph.svelte; markup one element per line (no line over 200 chars).
- Check 7/9 chrome and states: no header, monogram, eyebrow or slogan; empty state "Open a presentation / PDF, PPTX or HTML. / Open from Files" with recents rows; error state "This presentation could not be opened", "The file was not found.", muted "Source fetch failed (404)", Try again; unsupported type handled.
- Check 8 icons: inline SVG icon buttons with aria-label; no Unicode glyphs; no button under 32px.
- Stage fill (revised spec): at 1100x732 content area the 16:9 frame is 932x524, 100% of the stage width (the binding dimension); filmstrip 168px; player bar 44px. Click right half advances (1/6 to 2/6), ArrowRight advances, click left half goes back, in all 3 themes. Fullscreen: `document.fullscreenElement` set in all 3 themes.
- Check 10 phone (390px): one column, no horizontal scroll, no clipped control (filmstrip thumbs beyond the 72px strip scroll sideways, by design), position, Prev, Next, Present visible, 72px strip, 44px bar. Note: the 16:9 frame is 358x201 (27% of window height) because 16:9 at 390px is inherently that tall; the "stage" row is large but mostly empty. Not scored as a defect under a strict 16:9 frame.

Defects (each specific enough for the next revision):
1. Manifest display title is "Slides" (choir-package.json `display.title`), so the launcher shows two apps both named "Slides" and the window titlebar reads "Slides". The request required the display title "Slides (new)". Fix: set `display.title` to "Slides (new)" (and description to say it is the rewrite).
2. PPTX and HTML decks have blank filmstrip thumbnails in all 3 themes at desktop (the thumbnail tile is an empty rounded box; only PDF thumbs show the slide). `thumbSrc()` returns '' for html and `imageUrls?.[0]` (usually undefined) for pptx. Fix: render a scaled-down text/title thumbnail (the slide title in 0.72rem on `--choir-surface-document`) for pptx/html slides, same 16:9 tile.
3. Present (fullscreen) in London Salmon: the position "2 / 6", the Prev/Next icons and the "Exit" button are dark-on-black and unreadable (contrast well below 4.5:1). `.slides-shell:fullscreen` sets `--choir-surface-media` (black) but the bar text keeps the theme's dark text colour. Fix: in fullscreen set bar text/icons to a token that is light on media surfaces (e.g. a `--choir-text-on-media` token added to theme.ts, or use `--choir-text-on-accent`-style light token) and give the Exit button a visible border; recheck in all 3 presets.
4. Desktop loaded layout: the slide frame is flush with the window's right edge and the filmstrip divider (0 gutter) and its rounded corners touch the window chrome; the bar says content gutter 16px. Revised spec says the frame fills the stage, so keep it at least 90% of the binding dimension but leave a 16px gutter (932-32=900 wide is 96.6% of the stage; still passes).

Minor notes (not rejecting): Salmon thumbnail text is illegible at 136px (cosmetic, PDF page text); in the desktop harness the player bar sits tight to the bottom.

## Applied operation checks (b@c)

- Apply: selfdev-00b0b46014c4d5a26b48768330980e2e `applied` at 22:17:27Z, five seconds after approval (22:17:22Z). PLANNED restart with marker: console "boot follows a planned restart (self_development_apply selfdev-00b0b460...); pre-boot work resumes" at 22:17:23; source projection "materialization_applied: projected base=562c638530cb head=0955cb9e4534 booted=562c638530cb ... changed=true".
- Package list: `package search` returns an empty catalog (nothing published), so there is no catalog entry; this is expected before `package publish`. In the shell, the launcher (reproduced in the harness with the applied package) lists "Slides" twice, original plus slides-next (defect 1). The live b@c launcher itself was not viewable from here (no browser session).
- Original Slides untouched: the applied patch's 14 files contain no frontend/src/lib/SlidesApp.svelte, no app.css, no registry file; only frontend/src/packages/slides-next/ (5 files) plus 9 Texture citation files.

## Is the patch cumulative with the applied ceb607ca stack? YES (settled)

The patch of 00b0b460 lists the 9 Texture citation files (TextureEditor.svelte, texture-citation-marker.ts, texture-citation-notes.css, texture-published-citations.ts, texture-source-renderer.ts, texture-structured-editor-doc.ts, and three specs). Compared against the patch of the already-applied rebase ceb607ca: after dropping the `index` lines and hunk headers, 8 of 9 files have identical added/removed lines; TextureEditor.svelte has the same function bodies in a different diff alignment (the same blocks reordered by `git diff`'s hunk matching; line multisets differ by one `refreshSourceJournalFlow` line moved between context and change). No new Texture content was introduced by the engineer or the delegate. Conclusion: the 9 files are the carried applied stack, not engineer edits, and the patch is cumulative against the booted base (562c638). Only the 5 slides-next files are new in this operation. Residual: I compared patch against patch, not against the applied tree byte-for-byte.
