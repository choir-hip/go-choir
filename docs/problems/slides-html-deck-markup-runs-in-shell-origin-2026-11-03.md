# Slides renders HTML deck markup unsanitized in the shell origin (2026-11-03)

Status: open. Found by Devin's tier 1 review of the slides-next promotion (lane/slides-next, a18f22b3b), which inherited the pattern from the original.

## Evidence

- The original Slides app, frontend/src/lib/SlidesApp.svelte:410 on origin/main, renders an HTML deck slide with `{@html slides[currentSlide].html}`. Nothing sanitizes it or sandboxes it.
- `<script>` is inert under innerHTML. But `on*` attributes, `<iframe srcdoc>` (same origin, so it can reach `parent.document`), `<object>`, `<embed>`, `<link rel=stylesheet>`, `<style>` and `<base>` all run or apply in the shell origin, with the session's cookies and full /api access.
- Deck sources include arbitrary source URLs and /api/content items, which can be shared or published objects. That makes this a stored cross-site scripting path, not only self-XSS.
- slides-next made it worse: its filmstrip mounts every slide's markup at once.

## State

- slides-next: sent back to b@c with the fix specified: a sandboxed iframe with srcdoc, no allow-same-origin and no allow-scripts, plus a hostile-deck fixture (revision ea33e485, 2026-11-03). The lane is not landed.
- The original Slides: still live on staging with this exposure. Fix it in place, using the same sandboxed iframe, or retire it once slides-next replaces it. This is the owner's decision on timing; the exposure is live now.
