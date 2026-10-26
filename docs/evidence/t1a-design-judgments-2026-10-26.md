# T1a design judgments

FP0's pass or fail, per app candidate, on the twelve checks of
[the design bar](../design/app-design-bar-2026-10-26.md). Screenshots are
rendered locally in the real shell (mocked backend, signed in) from the
candidate's patch, at London Salmon, Futuristic Noir and Carbon Fiber Kintsugi,
1100x750 desktop and 390x844 phone, with the slow capture (15 s wait) while the
laptop is loaded. One fail rejects. Goal file:
[choir-t1a-app-self-development-2026-10-26.md](../definitions/choir-t1a-app-self-development-2026-10-26.md).

## The Reader, rounds 3 and 4 (selfdev-dc62cd52, then selfdev-02626b17), 2026-10-26

Round 3 passed the bar at 02:05 and was approved and applied on b@c. Review 258
(Devin, on the promotion bb070d0f7) then found search snippets read `hit.text`,
which search results do not carry, so every snippet was empty; two regexes with
doubled backslashes that never matched; dead code; a dead Open file on the
signed-out preview; and a stray `.tmp_check.js`. Round 4 was cast by texture
revise at 02:18 on the same document and frozen at 02:50.

Round 4's patch is cumulative from a03d5266a (r3 plus r4). Interdiff over the
round 3 promotion: four files, +51 -23. Rendered 17 scenes at 02:5x.

| # | Check | Round 4 | Evidence |
|---|---|---|---|
| 1 | Screenshot set | pass | 17 scenes, three presets, two widths, plus TOC-search and sepia |
| 2 | Colors from tokens | pass | no hex, rgb() or hsl() literal in reader.css |
| 3 | Shape and motion tokens | pass | radii and transitions read --choir-*; reduced motion drops transitions |
| 4 | Fonts from tokens | pass | chrome in the UI font, the page in the reading font |
| 5 | Type scale | pass | five sizes in reader.css (0.72, 0.78, 0.85, 1, 1.4rem) |
| 6 | Spacing scale | pass | 4/8/12/16/24/32px; one 2px row gap on phone recents |
| 7 | Hierarchy and chrome budget | pass | one toolbar; title only in the window title bar |
| 8 | Controls | pass | plain button, select and input |
| 9 | States | pass | empty state is one sentence, two actions and Recent |
| 10 | Phone | pass | one column; recents now put the name on its own line and the folder below |
| 11 | Contrast in every preset | pass | Noir and Kintsugi toolbars and page read clearly |
| 12 | Robust layout, reviewable markup | pass | snippets now split the cut snippet around the match; regexes fixed; dead code gone; no stray script |

Verdict: approve. FP0 approved selfdev-02626b17 at 02:58:44. Residual (cosmetic):
a snippet can begin mid-word ("...r nobody"), because the cut is by characters.
