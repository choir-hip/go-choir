# Texture reads go cold within seconds of idle; the list does ~100 store reads

Date: 2026-10-09. Status: open. Owner: "2.7 seconds to load a list of
textures is still wrong… Is it calling host? … Something else is wrong."
Mutation class of a fix: orange (Texture read path, store).

## Evidence (owner computer on 8fd243ba, 8 GiB, measured from Node B directly against the guest, bypassing the proxy)

| Request | Latency |
|---|---|
| `GET /api/texture/documents` (first call after boot) | 2.67 s |
| same, first call after ~5 min idle | 1.44 s |
| same, immediately repeated | 0.15–0.17 s |
| same, after 30 s idle | 0.45 s |
| same, after 90 s idle | 0.95 s |
| `GET /api/texture/documents/{id}` after idle | 0.43 s (8 ms warm) |
| `GET /health` | 4 ms |

The list does **not** call the host. Texture state is in the guest's
embedded Dolt store. The penalty applies to **every** store read after a
short idle period, not just the list.

## Code facts

**The list handler** (`internal/textureowner/handler.go:152`,
`texture.go:1487`) performs:
- two full scans of every Texture document object: the legacy owner list,
  then the scoped list;
- per document (50): one revision-stats query;
- per document (50): one full revision read, **body included**, used only
  for `last_editor`, `last_author_kind` and the version number. Revision
  metadata already carries `author_kind`, `author_label` and
  `version_number`.

**Store access is single-threaded:**
- the embedded Dolt handles use `SetMaxOpenConns(1)`
  (`internal/store/texture.go:399`);
- the read store shares the write store's engine mutex
  (`internal/store/store.go:918`).

So every read queues behind any background write on the computer
(dispatcher retries, wake outbox, idle sweeps).

## Hypotheses (unconfirmed; no profile yet)

1. Reads wait on the shared engine mutex behind periodic background
   writes. Latency would then depend on what else is running, not on
   idle time as such.
2. Dolt chunk or page cache eviction makes idle reads go to disk.
3. The list's ~100 reads multiply whichever per-read cost dominates.

## Next

1. Add runtime profiles (`/internal/debug/pprof/` on every service; mutex
   and block profiling enabled), deploy, and take a mutex, block and CPU
   profile while timing cold list calls.
2. Make the list one metadata pass:
   - one document scan;
   - revision stats and head author from revision metadata (no bodies).
