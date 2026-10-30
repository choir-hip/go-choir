# The Go Module Proxy Counts Its Cache as 24 Bytes; Eviction Would Delete the State Directory Link

Date: 2026-10-14, 18:05 UTC. Found while preparing the self-development
drill rerun on staging 865dc152 (the drill itself is recorded separately).
Mutation class of the fix: orange (host service behavior; no egress or
capsule change). Not on the drill's failure path.

## Evidence (Node B, read-only)

- `curl 127.0.0.1:8088/healthz` -> `{"cache_bytes":24,"max_bytes":8589934592,"ok":true}`.
- Every boot logs `cache /var/lib/go-choir-goproxy (24 bytes)` and
  `warm_done ... "cache_bytes":24`, after a warm that verified all 1,617
  entries.
- `du -sh /var/lib/private/go-choir-goproxy` -> 439M;
  `du -sh /var/lib/go-choir-goproxy` -> 4.0K.

## Cause

With `DynamicUser = true` and `StateDirectory = "go-choir-goproxy"`
(`nix/node-b.nix:1056-1080`, the fix in 3439462a), systemd keeps the real
directory at `/var/lib/private/go-choir-goproxy` and makes
`/var/lib/go-choir-goproxy` a symlink to it (link text
`private/go-choir-goproxy`, 24 bytes). The binary opens
`-cache-dir /var/lib/go-choir-goproxy`.

`openDiskCache` (`internal/goproxy/cache.go`) sizes the cache with
`filepath.WalkDir(root, ...)`. WalkDir does not follow a symlink root: it
visits the root once as a non-directory entry, so the only "file" counted
is the link itself, 24 bytes. Lookups and writes go through the kernel and
follow the link, so serving works; only the accounting and eviction walk
are wrong.

## Consequences

1. The size counter starts at 24 instead of about 439 MiB, so the 8 GiB cap
   is effectively 8 GiB of *new* fetches past the deployed set. Minor.
2. When eviction does trigger, `evict` walks the same root, finds one
   entry (the symlink), sorts it oldest and calls `os.Remove` on it: it
   deletes the state directory's link. Every later lookup misses, every
   `create` tries `MkdirAll` under root-owned `/var/lib` as the dynamic
   user and fails, and module fetches from capsules fail until the unit
   restarts (systemd recreates the link). Latent: needs 8 GiB of new
   modules.
3. `/healthz` and the boot line misreport the cache size to operators.

## Ways a fix could fail (written before the test)

- F-a: the walk still does not descend when the root is a link.
- F-b: eviction still removes the link (or anything outside the tree).
- F-c: a root that is a plain directory changes behavior (the existing
  eviction test must still pass).
- F-d: a dangling or missing root is not created (first boot).

## Fix (narrow)

Resolve the root once in `openDiskCache` with `filepath.EvalSymlinks`
after `MkdirAll`, and keep the resolved path as `c.root`, so the size walk,
lookups, writes and eviction all operate on the real directory. Test: a
cache opened through a symlink counts its files and evicts files, never
the link.
