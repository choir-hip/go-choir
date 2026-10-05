# S0: artifact-GC `og` live set read Store A — active sweep mass-deleted live corpus bodies

**Status:** fix landed (`og` live set → `corpus()`); fail-closed on any live-set
load error. Loss quantification running on node-b.

## What happened — 2026-10-05 ~20:45Z

While closing the S2 station, the operator ran the new
`POST /internal/platform/artifact-gc` endpoint (dry-run, then active) on
Node B to clear the disk-headroom deploy gate. The sweep deleted
**45,829 files (1.4 GiB) from `sha256/og/`** that it reported as
unreachable. They were live: `og_objects.body_ref` rows in **Store B
(corpus-dolt :13307)** still name them.

## Root cause

`internal/platform/artifact_gc.go:202` computed the `og` live set with:

```go
s.store.db.QueryContext(ctx, `SELECT body_ref FROM og_objects WHERE body_ref <> ''`)
```

`s.store.db` is the Store A pool (platform-dolt :13306). `og_objects`
bodies are written by `ObjectGraphStore` to **`s.store.corpus()`**
(Store B). In the split topology the live set therefore contained only
Store A's residue (620 body_ref rows) and every corpus-referenced file
resolved unreachable.

Second amplifier: a live-set query error appended a warning and swept
anyway — an unloaded live set is not an empty live set.

## Loss estimate

- `corpus.og_objects`: 8,752,974 rows; **2,184,983 carry `body_ref`**.
- `sha256/og/` held ~616k distinct digests (deduped CAS) — almost every
  on-disk file was live under the corpus refs.
- Deleted: 45,829 files, all past the 1h grace (older bodies).
- `platform.og_objects` (the pool the code read): 6,090,210 rows, **620
  externalized** — the residual mirror from the pre-split era.
- Precise missing set: `SELECT body_ref` anti-join vs the post-sweep
  filesystem listing — running detached on node-b; receipt lands here.

## Restoration posture

- Externalized rows store `body = NULL` (`inlineBody = nil` when
  `body_ref` set) — **the CAS file was the sole copy; deleted bodies are
  not recoverable from the table.**
- Re-derivation path: bodies whose content equals an event payload or
  another store's artifact could be reconstructed by replay — open
  engineering, not scheduled.
- On-node backups cover dolt state, not `platform-artifacts`.

## Fix (same commit as this doc)

1. `artifactGCLiveSets` `og` branch → `s.store.corpus()`.
2. All live-set load errors (roots, watermarks, og) return early — the
   sweep aborts with the warnings instead of deleting with a partial set
   (dry-run still aborts; nothing sweeps blind).
3. `TestRunArtifactGCOGLiveSetReadsCorpus` — split-pool seam test:
   decoy `body_ref` on Store A pointing only at the dead file, live ref
   on Store B; active sweep must keep the live file, delete the dead.
4. `TestRunArtifactGCFailsClosedOnLiveSetError` — corpus pool without
   `og_objects` → sweep errors and deletes nothing.

## Prevention follow-ups (SO)

- Live-set cardinality floor per namespace? Rejected for now — a
  heuristic floor guesses at scale; the fail-closed invariant is the
  real guard.
- `platform.og_objects` (Store A residue, 6.09M rows) is dead weight and
  a future decoy — schedule its removal under the storage lifecycle.
- Artifact-GC cadence: daily timer in `nix/node-b.nix` (comment exists);
  wire it once the live-set audit is complete for **all** namespaces.
