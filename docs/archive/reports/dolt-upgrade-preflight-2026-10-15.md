# Dolt 2.4.2 pre-flight on Node B (2026-10-15)

Owner ruling: update Dolt to the latest version, after checking
compatibility. Research is on branch `ops/dolt-upgrade`
(`docs/reports/dolt-upgrade-compat-2026-10-15.md`). This is rollout step 2:
a backup, then 2.4.2 run against scratch copies. Production was not changed.

## Backup (14:50:31–14:50:34 UTC, 3 s outage)

- Stopped go-choir-corpusd, the checkpointd timer, and platform and corpus
  Dolt. Took `cp -a --reflink=always` copies of `/var/lib/go-choir/platform-dolt`
  (25 GB) and `/var/lib/go-choir/corpus-dolt` (23 GB) into
  `/var/lib/go-choir/dolt-backup-20261015-pre-2.4.2/` (btrfs, extents
  shared), then restarted everything. Staging `/health` was ok afterwards.
- Corpus is 23 GB, not under 1 GB as the compat report said.
- This backup also serves as the pre-squash backup for the owner's weekly
  Store A squash ruling.

## 2.4.2 against scratch copies (`/var/lib/go-choir/dolt-preflight-2.4.2/`)

The binary was `/nix/store/nzgdqcx…-dolt-2.4.2`, fetched from
cache.nixos.org (nixos-unstable), not built on Node B. It ran under
systemd-run with MemoryMax=8G and low CPU and IO weight.

| store | open + `SELECT 1` | `dolt fsck` |
|---|---|---|
| platform | 1 s | No problems found; 1,348 s; peak RSS about 4 GB |
| corpus | <1 s | No problems found; 34 s |

## Rollback check

After 2.4.2 had opened both scratch copies, the current 2.1.9 binary read
them: `SELECT COUNT(*) FROM dolt_log` returned 94,009 (platform) and 1,616
(corpus) commits, and `dolt status` was clean. Opening with 2.4.2 does not
lock 2.1.9 out of a sql-server store.

## Next

Step 3: deploy only the server binary bump (`ops/dolt-upgrade` ca5f0f5e2,
the flake Dolt lane), together with the ascending-index commit d893bc998,
which is harmless on its own. Do it when no tracer is running. Watch the
liveness probe, one auto-GC and one conjoin cycle, then soak 24–48 h before
the Store A squash and the embedded module bump. The scratch copies can be
deleted; the backup stays until the squash and module bump have soaked.
