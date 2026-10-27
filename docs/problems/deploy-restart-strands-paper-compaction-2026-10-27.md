# A deploy restart strands the paper's nightly compaction (2026-10-27)

Status: open; documented before any fix. Found by FP0 while sizing S2
(docs/problems/paper-never-checkpointed-2026-10-27.md).

## Evidence

- The 04:00 sweep (`POST /internal/vmctl/compact-sweep`) began the paper's
  compaction. vmctl restarted at 04:00:30 and again at 04:19:05, both deploys.
  The reflink copy `data.img.compacting` was last written at 04:19:30 and then
  left: 128 GiB apparent, 65.5 GiB allocated, 24.07 GiB exclusive (btrfs).
- No compaction landed for the paper today. Its data image went from 79%
  (04:05) to 93% of its 128 GiB cap. On 10-26 the same compaction took the
  store from 37.0 GB to 22.4 GB (`outcome=refreshed`).
- Cleanup of a stale copy runs only at the start of the next attempt
  (`hostStoreCompactor.CleanupStale`, `internal/vmctl/store_compaction.go:137`),
  which for the timer means 04:00 the next day. Until then the host carries
  the exclusive bytes.

## Action taken

20:5x UTC: after checking that no `vmctl-store-compact-*` unit, mount or open
handle referred to it, FP0 removed the stale copy. The host's root went from
76 GiB free to 100 GiB free.

## Fix direction (not chosen)

- A deploy restart should not land inside the compaction window, or vmctl
  should clean stale copies at start.
- This is the same class as the 10-27 paper boot killed by a deploy restart:
  deploys restart vmctl regardless of in-flight host work for a protected
  computer.
