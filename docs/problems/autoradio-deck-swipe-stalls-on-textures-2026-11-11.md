# Autoradio: a sideways swipe on the Textures page never reaches the deck (2026-11-11)

## Problem
In the Autoradio app's page deck a sideways swipe sends the top page to the bottom. On the Textures page (`textures:`) the swipe does
nothing, in the simulator, in both directions, at every start height. The deck cycle step of `mobile/scripts/proof-app.sh` therefore fails
on the first Textures swipe (six swipes over a six-page deck end on Textures instead of on the starting story). Files, Mail, Settings, the
front page and a story cycle.

## Evidence (each simulator run, `proof-app.sh deckdiag` or the full tour; all drag from the page-deck node with `--touch platform`)
1. Full proof, head with the deck step: labels after each swipe: story, Front page, Settings, Textures, Textures, Textures, Textures.
2. Textures alone at y = 150, 300, 462, 600: none moved. Landed node is the scroll's content column, the same shape as Files.
3. Files, Mail, Textures, Settings in turn at y = 300: Files, Mail and Settings moved; Textures did not.
4. `state` right after the swipe: Files `pxAbs` 522 (flying); Textures `pxAbs` 0, `prog` 0, order unchanged: the deck's `pan` was never heard, it
   is not a release that came up short.
5. Ruled out by a run each: the looping "Being revised" progress control (replaced by a still glyph: still stalled); the search input (removed:
   still stalled); filtering the list to one row, to the revising row only, to the pinned glass edition card only (typed into the search, so the
   keyboard was up: inconclusive, not counted).
6. Cut-down lab pages (temporary, removed): heading only; rows as buttons around Cards (the page's own `TextureRowView`); plain buttons;
   Cards without a button; button around a plain shadowed box; the same with `backgroundMaterial="none"`; the pinned level 2 glass card;
   Recent label plus New document button: every one let the swipe through, as did the **whole TexturesPage body mounted under a lab
   address** (first time only; a second visit to it stalled once). The real Textures page stalls every time, also after a 4 s settle and
   when repeated, in both directions.

## Belief
Not the page's content (the same body under another address cycles on its first visit). Something about the page being `textures:`
(its kind, its Kit-side `texturesList` resource, its place in the Kit's focus handling) or about order and history in the drive, since the
one lab page that stalled once did so on a later visit. Confidence low. A fifth possibility: an artifact of the simulator's synthetic touch
on this page; the owner's finger on the phone will settle it.

## Remaining error
No cause in one sentence. Not tested: the Kit's `pageOpen("textures:")` side effects on focus; the `ScrollView id="textures"` against the
other ids; a real finger. The full state (not the derives) after a failed swipe, which the driver prints truncated.

## Next probe
Mount the real TexturesPage under a lab address and visit it twice and three times in a fresh app (to see whether the stall follows history
or the address); `state` with a path for `flying`/`pdrag`/`px`; the same swipe with `--touch agent` (not platform) to see whether the platform
touch is the difference; then, on the phone, swipe on Textures with a finger.

## Status
Known defect in the round 3 merge head. The launcher still brings any page to the top, so Textures can be left by the launcher. Not fixed.
