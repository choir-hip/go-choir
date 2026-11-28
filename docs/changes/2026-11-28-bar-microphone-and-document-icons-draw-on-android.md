---
date: 2026-11-28
title: the bar's microphone and the viewer's document icon draw on Android, and a build check keeps the rest in view
class: orange
---
What: `contract build app.contract` warned `lower-symbol-platform` for seven symbols, each `symbol:sf/...`, which draws on Apple only and is blank on Android and web. Three have a role every host draws, so they now use it: `sf/mic.fill` in the bar's glyph (`bar.contract`) and the radio's mic button (`radio.contract`) is `symbol:microphone`, and `sf/doc` in the viewers (`viewers.contract`) is `symbol:document`. New `AutoradioLab/symbol-platform.test.ts` runs the real contract build (about 15 s) and fails on any `lower-symbol-platform` warning not in its list of symbols still waiting for art; the list may only shrink.

Why: the bar's microphone was invisible on Android. The fix the warning names is an icon token in `app.forma` with an SVG; this app has no `app.forma` and no icon art, and art is not invented, so the symbols with a role take the role (Exact docs/recipes/symbols.md: "A role draws everywhere").

Still blank on Android and web, each needing an SVG drawn as filled paths and an `icon.<name>` token: `sf/mic.slash` (`bar.contract:55`, the bar's microphone when off, so Android shows no icon there, only the caption and colour), `sf/chevron.down` (`notebook.contract:51`), `sf/text.badge.plus` (`paper.contract:31`), `sf/hourglass` (`viewers.contract:193`). No role stands for them.

Differences: on iPhone the microphone is the role's glyph (the outline `mic`), no longer the filled `mic.fill`, and the document is the role's `doc`. Decided: a visible icon on both phones is worth the filled look. The bar's comment "solid when on" still describes the colour, not the fill.

Proof: `symbol-platform.test.ts` failed before (`radio.contract:217`, `viewers.contract:193`, `bar.contract:53` unlisted) and passes after; the build then warns of exactly the four symbols above. Not proved on a device.

Replaced: three `symbol:sf/...` literals.

Undo: revert the commit.
