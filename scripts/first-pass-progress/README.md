# First-pass progress

The launch first pass's status page shows stage-weighted progress built from
`docs/definitions/choir-launch-first-pass-progress.json`.

- Each checklist item has slices; each slice has a stage and a dated history.
- Stages and weights: 0 queued (0), 1 building (.2), 2 in review (.35),
  3 accepted (.5), 4 landed (.65), 5 deployed (.8), 6 proved (1.0).
- An item is the mean of its slices; superseded and owner-deferred items leave
  the denominator. The page's history line is rebuilt from the slices' dated
  histories, from the pass start (2026-10-21T18:00Z).

`bump.py ITEM SLICE_SUBSTR STAGE "evidence" [--at ISO]` moves a slice and
appends its history (the data file is `$PROGRESS_JSON`, else the repo's). `render.py ITEMS_JSON PAGE_HTML` rewrites the
progress sections of the status page in place. FP0 runs both at every landing,
deploy and proof.
