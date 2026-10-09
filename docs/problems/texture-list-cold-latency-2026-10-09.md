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

## Profile (owner computer on f8a968dd, 2026-10-09 08:51Z, 25 s CPU + mutex + block)

Taken through `/internal/debug/pprof/` (f8a968dd) while timing cold list
calls (5.6 s during the profile). **This record is late:** the fixes below
were committed before this section, against the problem-documentation-
first rule; the profile was reported in the session but not written here
first.

- **The list is not the cost; boot replay is.** 89% of CPU was under
  `textureowner.(*Handler).Start` (boot reconcile), which was still running
  minutes after boot. The list handler itself was 1.5%.
- `GetLifecycleSnapshot` was 68% of CPU, called **twice per Texture
  document** at boot: once by `textureLifecycleActivationEligible`, once
  for `PendingTextureOwnerRevision`.
- Inside it, `ReadObjectSnapshotFiltered`:
  - phase 1 scans the metadata of 9 kinds across the whole computer;
  - the keep filter kept **every** Texture document and revision;
  - phase 2 fetched all survivors in one `IN (...)` list.
  go-mysql-server's range overlap check on that list is quadratic:
  `StringType.Compare` alone was 38% of CPU.
- **Mutex profile:** 91% of `engineMutex` hold time was this snapshot read.
  Hypothesis 1 holds: list reads queue behind boot's per-document snapshots
  on the shared engine mutex. "Cold after idle" was really "behind boot
  reconcile or background sweeps".
- Other boot costs: `ListActionablePendingLifecycleUpdates` (12%, JSON
  extraction over update bodies per document);
  `sweepActorWakeOutbox` → `LatestActorRunMemoryEntries` (5%).

## Fixes (c01bc2f9, deployed to staging 09:32Z; owner computer refreshed 09:33Z)

| Commit | Change |
|---|---|
| e228711f | phase 2 fetches survivors in batches of 256 in one read transaction |
| 02807c7d | the snapshot keeps only the trajectory's own document and revisions; `GetLifecycleHeadView` (point reads) replaces the snapshot in the activation classifier |
| c01bc2f9 | boot reads the full snapshot for the owner-revision check only when the head view shows an owner-input head |

Still open: the phase-1 metadata scan is still computer-wide per snapshot
(the wake path and the API still use it), the list's own ~100 reads
(Next item 2), and `ListActionablePendingLifecycleUpdates` per document.

**Measured on the owner computer (c01bc2f9, 8 GiB, from Node B direct to
the guest):**

| Request | Before | After |
|---|---|---|
| `GET /api/texture/documents`, first call after boot (boot replay still running) | 2.67 s | **0.19 s** |
| same, repeated | 0.15–0.17 s | 0.15–0.18 s |
| same, after 90 s idle | 0.95 s | **0.17 s** |
| boot replay (`starting server` → `replay complete`) | 9.0–10.8 min | **4.7 min** |

Status: the owner-visible list latency is resolved. Boot replay is still
4.7 min, and the open items above remain.
