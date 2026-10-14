# Deployed Reader Spec: No Source Note When The Sources Panel Is Open

Date: 2026-10-14, 02:30 UTC. Found while proving the L2a deploy
(`66fe0917`) with `frontend/tests/autopaper-reader-deployed.spec.js`
against choir.news. Status: documented; not fixed.

## What fails

The test "Autopaper front list and stories as Texture published views,
logged out at phone width" fails at line 246. Its steps:
1. Open the Sources panel (the `Sources` toggle, which is pressed).
2. Check each citation in the panel.
3. Click the first citation number in the text.
4. Expect a `[data-texture-source-flow-note]` beside the text.

No note appears within 10 s. The page snapshot shows the Sources panel
still open, with its source button listed.

The front page part of the same spec passes, and so does the other test
("logged-out home opens Autopaper; a story opens as a published Texture
window").

## Why this is not caused by the L2a deploy

The deploy changed the Autopaper front page (`AutopaperApp.svelte`,
`AutopaperStory.svelte`, `AutopaperRoute.svelte`), desktop icon clicks
(PR 70) and the intake runtime. None of these touch Texture's published
reader. The code the step exercises changed in `7c81bbca` (2026-10-13
15:13 EDT): "published sources open above the text from a Sources
button".

## Hypothesis, not yet confirmed

With the Sources panel open, a click on a citation number goes to the
panel instead of opening a flow note. The spec expects the note either
way.

One of these needs changing: the spec (close the panel first, or expect
the focus to move in the panel) or the reader (a click on a number should
always open its note). Which is a product decision for the reader's
owner.
