# Autopaper section pages go empty when a section is quiet for a few hours

Found 2026-10-15 ~12:00 UTC by the owner: the U.S. Politics section showed
"No stories in this section yet." although U.S. politics stories were
published yesterday and overnight. Nothing was deleted.

## Evidence

`GET https://choir.news/api/platform/publications/front?limit=200` (served
100 rows, more pages exist), counted by `section`:

| newest N | world | tech_ai | business | us_politics | none |
|---|---|---|---|---|---|
| 20 | 6 | 6 | 3 | 0 | 5 |
| 40 | 19 | 10 | 6 | 0 | 5 |
| 100 | 43 | 27 | 14 | 11 | 5 |

The newest U.S. politics story sits at position 65 (04:01 UTC), the others at
67–79 (02:58–03:35 UTC): overnight the paper published mostly World and Tech.

## Two faults

1. **Section pages filter only the first page.** `AutopaperApp.svelte` loads
   `fetchFrontPage('', FRONT_FIRST_PAGE_SIZE)` (60) and a section page is
   `storiesInSection(frontStories, section)`, so a section is empty whenever
   its newest story is older than the newest 60 stories overall (~3.5 h at
   ~410 stories/day). The "Older stories" pages are filtered too, but the
   section page shows the empty state before the reader can reach them.
2. **Sections are erased after ~100 stories.** A story's `section` (and
   `importance`, `promo`) comes only from the front-importance scores file
   (`attachFrontImportance`). The scorer (`internal/frontimportance/scorer.go`
   `run`) reads the newest 100 (`defaultFrontURL ...?limit=100`) and writes a
   new file containing only those 100; rows for older stories are dropped.
   Past position 100 (~6 h) a story has no section anywhere, so no fix to
   paging alone could list it on its section page.

## Owner ruling

"We should always show the stories we have, even if they are a few hours
old." A section page lists that section's stories, newest first, back as far
as the paper goes, with "Older stories" paging.

## Fix shape

- Scorer: carry forward previous rows for stories not in the current window
  (bounded by age, e.g. 14 days by `scored_at`), so a story keeps its
  section once scored.
- corpusd: `section` filter on `/internal/platform/publications/front`
  (pages through publications until the page is full, bounded scan), proxy
  passes `section` through.
- Frontend: a section page fetches `?section=<id>` itself with cursor paging,
  instead of filtering the front page's rows.

## Fix

Branch `fix/autopaper-section-pages` (orange; rollback: git revert each commit).

- `b6490f95b` scorer: `run` carries previous score rows for stories outside
  the fetched window, dropping rows older than 14 days by `scored_at`
  (`-retention` flag / `FRONT_IMPORTANCE_RETENTION`, default 14d) or with no
  readable `scored_at`.
- `44226829e` corpusd: `?section=` on the internal front list; invalid section
  is 400; scans publication batches (100 at a time, at most 2000 per request),
  `next_cursor` resumes after the last publication scanned, so a short or
  empty page can carry a cursor. Unfiltered path unchanged.
- `763e05ea9` proxy: passes `section` through.
- `e15d7b3a8` frontend: a section page fetches `?section=<id>&limit=60` and
  pages its own cursor; empty pages with a cursor are followed; the empty
  state shows only when the server returns zero.

Deploy: the scorer change ships in the corpusd binary (the timer runs
`corpusd front-importance`), so restart/redeploy corpusd, proxy and the
frontend. Sections of stories already past the newest 100 are not recovered
(their rows are already gone); they refill only as stories are rescored.

### One-off backfill after deploy (stories past the newest 100)

The scorer now reads `-pages N` front-list pages (env
`FRONT_IMPORTANCE_PAGES`, default 1, the old behaviour), following
`next_cursor` from `FRONT_IMPORTANCE_FRONT_URL` (`...?limit=100`). Stories
that already lost their rows have no score, so one run over deeper pages
scores them. The unit (`nix/node-b.nix`, `go-choir-front-importance`) is a
hardened oneshot with credentials, `TimeoutStartSec=5min`, `CPUQuota=50%`
and a fixed `ExecStart`, so run the backfill as a temporary runtime drop-in
that changes only the command line and timeout, not as a hand-built
`systemd-run`. On Node B, after the deploy has put the new corpusd binary in
place:

```sh
sudo systemctl stop go-choir-front-importance.timer
sudo mkdir -p /run/systemd/system/go-choir-front-importance.service.d
BIN=$(systemctl show -p ExecStart --value go-choir-front-importance.service \
  | sed -n 's/.*path=\([^ ;]*\).*/\1/p')
sudo tee /run/systemd/system/go-choir-front-importance.service.d/backfill.conf >/dev/null <<EOF2
[Service]
ExecStart=
ExecStart=$BIN front-importance -pages 5 -max-calls 500 -budget 30m
TimeoutStartSec=45min
EOF2
sudo systemctl daemon-reload
sudo systemctl start --wait go-choir-front-importance.service
journalctl -u go-choir-front-importance.service -n 20 --no-pager
# cleanup: back to the normal 10-minute run
sudo rm -r /run/systemd/system/go-choir-front-importance.service.d
sudo systemctl daemon-reload
sudo systemctl start go-choir-front-importance.timer
```

Expect `front-importance: ... N scored` in the journal; check `scores.json`
section counts afterwards. `-pages 5` covers the newest 500 stories (about
a day); the 14-day carry-forward keeps those rows from then on. If the run
hits the budget, run it again: scored rows are kept and only unscored
stories cost calls.
