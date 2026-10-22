# A related-stories test reads an over-budget answer as "no links" and failed deploy L's CI

Date: 2026-10-22. Status: **documented; cause read in code; test fix follows in
the next commit**. Found by FP0 while landing deploy L. Evidence class: CI
(GitHub Actions), not staging. Mutation class of this record: green.

## Evidence

- CI run 39761282573 on main `0cb344783` (deploy L: the decision-409 fix,
  RES-10 steps 1 and 2, the token-probe fix, docs). Job "Go Test (race,
  non-runtime shard 7)" failed at 10:20 UTC, and "Go Vet + Test + Build"
  failed with it. Staging kept deploy K (`5289387c9`).
- The only failing test:

      === RUN   TestRelatedHubCaptureGivesNoVote
          publication_related_test.go:136: six citers: 0 links, want the other 5
      --- FAIL: TestRelatedHubCaptureGivesNoVote (33.13s)

- Deploy L changes nothing in `internal/platform`. The test last changed on
  2026-10-19 (`0a541483c`) and has passed on every main run since.
- This is the third unrelated flaky test to fail a deploy today, after
  tapestore-appendfunc-test-races-its-own-done-flag-2026-10-22.md and
  release-patched-tree-fingerprint-flakes-under-ci-2026-10-22.md.

## Cause (read in code; matches the evidence)

`Service.RelatedStories` (`internal/platform/publication_related.go:521-531`)
bounds each request by `tuning.Budget`, which is 400 ms in production
(`:117`). When the walk overruns it, the answer is "empty, flagged partial,
never cached": `Links: []`, `Partial: true`, and no error.

The test lab's `related()` helper (`publication_related_lab_test.go:158`)
returns that answer without checking `Partial`. The test's second half
publishes six stories, over 30 fillers from `newRealisticLab`, and then
walks them. Each `list_edges` call in the job log takes 3 to 5 ms under
`-race`, and the whole test took 33 s on a loaded runner. So the walk
overran 400 ms, the answer was partial and empty, and the test reported
"0 links". No production code is wrong. The test cannot tell an overrun
from a wrong answer.

## Effect

A loaded CI runner can fail any lab test whose walk is long, and fail it as
a wrong result. The budget behaviour itself is tested on purpose in
`TestRelatedOverrunIsPartialAndNotCached` (`:436`, `:444`), which sets its
own budgets.

## Fix (next commit, test only)

- `newRelatedLab` gives the lab a 30 s budget, so a slow runner cannot
  overrun it. The budget test still sets its own 1 ns and 5 s values.
- The lab's `related()` helper fails with "over its budget (partial)" when
  an answer is partial. A future overrun then names itself rather than
  posing as missing links.
- The production budget (400 ms) does not change.
