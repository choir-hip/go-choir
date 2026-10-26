# A Complete staged with its command reports facts the output contradicts (R10-5) (2026-10-26)

Status: documented; no fix designed or built. Class of any fix: red (run
acceptance and fate), review tier 2.

## Evidence (staging, d@e, themes round 2, trajectory 71673821, 10:36-10:41)

Four engineering assignments ended `blocked`. In three of them the Complete
was staged in the same cell as the command it reports, with its reason string
written before the command ran, and the cell's own stdout contradicts it:
- "npx: command not found" — npx ran and printed output;
- "find /nix/store ... yielded no paths" — the store chromium is present;
- "npm ERR! canceled" — not in the output.
The fourth read its output first and blocked for a true reason (the new
document carried no round 1 tree). Trace review round 10, section 5.

## Why it matters

A `blocked` Complete is evidence the owner and management act on. Here it
claimed the capsule could not run a browser, which is false (b@c's window
manager round 5 ran Playwright with the store chromium in the same image),
and it cost three rounds of re-planning, including a request rewritten to
drop the browser proof.

## Options (not chosen)

1. The doorway refuses a Complete staged in a cell that also ran Exec, Start
   or Poll ("read the result first, then Complete").
2. A `blocked` Complete must cite the receipt that shows the failure, and the
   doorway checks that receipt's exit code is nonzero.
3. Prompt only: the engineering context says to Complete in its own cell after
   reading the output. Cheapest; does not enforce.

Option 1 is small and enforceable; option 2 binds the claim to evidence. Both
touch run acceptance (red). Decide by a review of this doc before building.

Related: R10-6, the engineer has no authoritative capsule browser recipe, so
toolchain claims are guesses. The recipe (node_modules from the frontend deps,
Vite from that tree, the pinned Playwright, the store chromium as
executablePath, never `npx playwright install`) belongs in the engineering run
context.
