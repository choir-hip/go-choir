# The contract rule tests are Swift, and CI runs neither them nor bun for the phone

Date: 2026-11-20. Status: repaired in lane design-rules-fix, with one gap left open (below).

## Problem

DesignRulesTests and TypographyRulesTests read the `.contract` text under `mobile/AutoradioLab` and refuse a design-rule violation (no filled blocks
off the flow pages, a tappable headline carries a chevron or an Action word). They are Swift tests, and no CI job runs Swift for the phone or `bun test`
for `mobile/AutoradioLab`. So the signed-out work landed on main with violations nobody saw.

## Evidence

- The lead's run on main, `swift test --filter 'DesignRulesTests|TypographyRulesTests|StreamedDeskEngineTests'`: 24 tests, 5 failures.
  `testFilledBlocksAreOnlyOnFlowPages`: radio.contract:133 (Play the public paper), settings.contract:376 (Sign in), signedout.contract:20 (Sign in).
  `testATappableHeadlineCarriesAnActionAffordance`: radio.contract:137 and :140 (the public paper's story rows).
- `grep -rn "bun\|mobile" .github/workflows scripts/land .github/scripts` finds no step that runs bun or Swift tests for `mobile/`.
- After the port, the bun versions run against the old contracts report exactly those five lines (checked 2026-11-20).

## Belief

The rules lint text; nothing about them is native, so Swift only hid them from the only gate that runs everywhere. The five violations were new
code written without the rules in view, and the rules are right about all five (a flat block is for a flow page; a bare serif title does not say it can be
tapped). Fixed in the contracts, not by exemption.

## Remaining error

CI still does not run `bun test` in `mobile/AutoradioLab` (709 tests, 3 seconds); this change does not add it because `.github/workflows/ci.yml` is a
guarded gate file. Until a job runs it, the rules are only as good as whoever runs `bun test`. Two theme checks (R1, T8) stay in Swift until the themes
port (lane exact-themes) and are likewise unrun by CI. Seen only by text: nobody has looked at the changed screens.
