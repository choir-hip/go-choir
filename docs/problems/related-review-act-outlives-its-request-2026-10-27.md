# The related-review act outlives its request, so the backfill loses every receipt (2026-10-27)

Status: open; documented before any fix. Mutation class of a fix: orange
(an owner act's API and store reads). Investigation: read-only, Sonnet,
16:0x; claims marked verified (code or log line) or hypothesis.

## Evidence

- `POST /api/texture/related-review` returned receipts for batches b13 to
  b16 (limit 12) until 11:18. From 11:23 to 16:02 the proxy logged 29
  `upstream transport error` lines on the route: 17 `context canceled`
  (the client gave up) and 12 `EOF` (11:46:50 … 16:02:15). Batch b40
  (limit 10, waits up to 15 min) never returned in 20 tries; the loop
  stopped at 16:03. Entities still grew 147 → 190, so asks were committed.
- The paper's console over the same window: `store_stalled` lines with
  `write_pool{max_open=1 … wait_count=137137 wait_ms=3778072}` and
  `read_pool{max_open=1 in_use=1}`; holders named
  `selfdev.(*Store).ListByStates`, `objectgraph.(*DoltStore).ListObjectsPage`,
  `GetObject`; a repeated `run_memory_agent_list slow … rows_returned=15248
  elapsed_ms=1108`; online dolt gc at 14:08 and 15:38; `desk_go_eval` calls
  of 30 to 97 s. No log line names the act itself (hypothesis gap).

## Why (code, verified unless marked)

- Every act first reads the whole System1 channel
  (`system1ChannelTape`, `system1_verb.go:118` →
  `ListAcknowledgedEventsByChannel`, `store.go:2867`: up to 10,000 JSON
  bodies decoded), then **every texture document of the store**
  (`ListStoryDocuments`, `stories.go:43`, calls
  `ListTextureDocumentsByScopeOG(…, 1<<30)`, paged 256 at a time, decoded
  and sorted before the cursor and `scanMax` apply).
- Per story: a commitment read, the full head revision (`storyHeadEmpty`),
  three reads for an active research run, and `CommitStoryResearchRequest`,
  which holds `trajectoryMu`, the lifecycle write lock the busy desks share.
  Recorded, empty and busy stories count toward the 2,000-story scan, not
  the limit, so one act can walk far (hypothesis).
- All of it goes through one-connection read and write pools on a paper
  running 12 dispatch slots.
- The guest server's write timeout is most likely the 120 s default
  (`internal/server/server.go:107`; `nix/autoputer-vm.nix` sets none; the
  kernel-cmdline env file not checked): an act longer than that cannot
  write its receipt, hence the `EOF`s (hypothesis). The act runs on
  `r.Context()`, so a client that gives up cancels it midway; asks already
  committed stay.
- A retry with the same batch id repays both full scans before replaying.

## Fix candidates (not chosen)

1. **Bounded start:** the handler commits the batch and returns 202 at
   once; the work runs detached and records its receipt (cursor, counts)
   on the tape for the client to read.
2. **Wall-time bound:** stop at about 90 s and return the partial receipt
   with `next` set; the cursor already supports it.
3. **Cheaper reads:** a bounded, ordered story read instead of the
   full-store scan; read the System1 channel once per act or keep the
   recorded set incrementally.

2 is the smallest and keeps the API; 3 removes the cost that grows with
the store. Pins: `internal/agentcore/story_entities_backfill_test.go`,
`internal/textureowner/texture_related_review_test.go`.

Since train 34 every story shows 3+ related stories (the floor), so the
backfill now improves link quality only; nothing waits on it.
