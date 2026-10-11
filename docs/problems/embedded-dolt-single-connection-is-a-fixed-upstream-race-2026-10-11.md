# The single embedded Dolt connection works around a race fixed upstream

Found 2026-10-11 while starting Autopaper W1
(definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md).
Evidence class: code reading of our pinned module and of upstream Dolt,
plus a local race-detector experiment.

## What we have

The guest's embedded Dolt runs every pool at one connection
(`configureEmbeddedDoltDB`, internal/store/texture.go), and every object
graph access takes a shared engine mutex (internal/objectgraph/dolt_store.go,
`engineMu`). That mutex was added in 488e8cd0 after the race detector
caught two pools on one connector racing. The cause it records:
`unescapeHTMLCodepoints` mutates a JSON value's buffer in place, and that
buffer aliases a shared, cached prolly node. This is July's cause C1 in the
Autopaper design (§0): health, Texture, graph reads and background work
all queue on one connection. The design's W1 requires that high-volume
captures never share it.

## What we found

- Our pinned Dolt is `github.com/dolthub/dolt/go`
  v0.40.5-0.20260715172757-a6690826d767 (July 15). Its
  `unescapeHTMLCodepoints` (store/prolly/tree/blob_builder.go) still
  writes into its input. The caller is `JSONDoc.ToLazyJSONDocument`.
- Upstream commit 6bea93f2f665 (July 16, "Fix some data races surfaced by
  running go-sql-server-driver tests under -race") makes it copy before
  writing. The new comment: "|path| may be a view into shared, cached
  chunk storage that other goroutines can be reading concurrently, so we
  must not write to it."
- The prolly node cache is process-wide (`sharedCache` in
  store/prolly/tree/node_store.go), keyed by chunk hash. So a second
  embedded workspace in the same process would not fully isolate the bug:
  identical chunks in two databases share one cached buffer.
- A local race-detector experiment (eight sessions on one connector;
  two connectors on two workspaces; concurrent reads of shared rows) did
  not reproduce the race. The tables used LONGTEXT and plain JSON reads,
  which never took the lazy-JSON path. That is not evidence of safety.

## Why it matters

C1 is the July failure Autopaper must not repeat. Every workaround on the
table (a second workspace, a separate store process, a different engine)
treats a symptom of a bug the library has already fixed. Convergence Before
Patching applies: fix the substrate.

## Belief and test

I believe upgrading Dolt (and its go-mysql-server pairing) past 6bea93f2
removes the documented race. If so, more than one connection per engine
becomes possible, and captures can get their own pool instead of their own
engine. Tests:
1. A race test that reaches `ToLazyJSONDocument` with HTML-escaped JSON
   from several sessions at once. It should fail on the pinned version and
   pass after the upgrade.
2. The store, objectgraph and agentcore packages under the upgrade.
3. A staging computer with existing data boots and serves on the upgraded
   engine (storage format compatibility).
Only after those pass: raise the connection count, and remove the engine
mutex in measured steps.
