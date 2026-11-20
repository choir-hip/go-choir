# The off-host backup has not pruned for twelve days, and its freshness check fails unseen

Recorded 2026-11-20T23:25Z by the lead, found while clearing disk on Node A at the owner's
request. Read-only; nothing about the backup was changed.

## Problem

Node B backs up to a restic server on Node A (`/srv/choir-backup/nodeb/daily`,
211 GiB). New backups are being saved, but the weekly prune has failed, so
the repository only grows. Node B's freshness check has been failing every
six hours and nothing surfaced it.

## Evidence

- Node B, `go-choir-offhost-backup-freshness.service`, 2026-11-20T22:32Z:
  "ERROR: last prune was 'failed' (prune: restic exit 1)" and "no successful
  prune in 240 h (last: 305 h ago)". The unit exits 1 at each run (16:32Z and
  22:32Z seen).
- Node A, `choir-backup-rest-prune.service` (the weekly window in which the
  server allows deletes): failed with result `timeout` on 2026-11-15T07:00Z
  after its two-hour limit. Its log at 05:11:50Z shows the server answering
  **507** to a `POST /nodeb/daily/data/...` during the prune, then lock
  deletes. It read 56.6 GiB and wrote 28.5 GiB in the window.
- Backups themselves succeed: the append-only server logged a snapshot saved
  at 2026-11-20T02:47:12Z with status 200, and no 507 since 2026-11-19.
- Btrfs quotas are not enabled on the backup subvolume, so the 507 is not a
  filesystem quota. The server's own size cap is the likely source (the unit
  line is below if it could be read).
- Node A's disk was 81% full (185 GiB free) at the time; after the nix
  garbage collection of 2026-11-20 it has 409 GiB free.

```
ExecStart=/nix/store/rhws8kfki3dzqkxx6l9g2ixd89s6b79b-restic-rest-server-0.14.0/bin/rest-server --path /srv/choir-backup --private-repos --log - --listen 127.0.0.1:8001 --htpasswd-file %d/htpasswd
RuntimeMaxSec=2h
```

## Belief

A prune repacks data before it deletes, so it needs room above the current
size. The server refused a write for lack of room under its cap, the prune
could not finish, and the next weekly window (2026-11-22T05:00Z) will fail
the same way unless the cap or the retention changes. Meanwhile the
repository grows by each day's backup.

## Remaining error

- The cap's value and whether it is the server's `--max-size` are not
  confirmed.
- Whether the failed prune left the repository consistent has not been
  checked (`go-choir-offhost-backup-verify` was not read).
- Why a failing freshness unit reaches no one: the hourly status script does
  not read it.

## Next

1. Read the verify unit's last result and the cap.
2. Raise the cap or shorten retention so one prune can complete (red: backup;
   the owner's call on retention), then watch the 2026-11-22T05:00Z window.
3. Add the freshness unit's state to `scripts/status-hourly` as a FLAGS line
   (yellow).
