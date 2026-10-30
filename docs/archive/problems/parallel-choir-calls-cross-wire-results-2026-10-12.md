# Parallel `choir.*` calls from one cell get each other's results

Found 2026-10-12 during read-only design research for async search
(study A, owner direction "search should be async, not waiting for
results"). Status: documented, not fixed.

## What happens

A desk cell that calls `choir.*` network verbs (WebSearch, FetchURL and
the rest) from several goroutines at once gets **the wrong results back**,
and every call waits for the slowest.

`egressRoundTrip` (internal/yaegikernel/sidecar.go:325-350) writes a
request frame and then reads the next frame on the connection. It does not
match the response to the request id it just minted, and nothing serializes
the write and the read. The transport expects one reader
(transport.go:59). The host serves egress frames one at a time, inline
(desk_session_worker.go:331-334, 414-427).

## Evidence

A probe (a test file kept outside the repo, against a real socketpair and
the host `EvalCell`) ran four goroutines calling egress at once; the first
query took 300 ms, the rest were instant. Every run gave the same result:

- caller `a` got `c`'s answer, `b` got `a`'s, `c` got `b`'s;
- every caller waited about 300 ms (head-of-line serialization).

## Why it matters

The research prompt invites the pattern: "Use choir.WebSearch and
choir.FetchURL with parallelism appropriate to the task"
(internal/runtimeprompts/overlays/rlm_research_runtime.yaml:25).

- Any past research run that followed this advice could have attributed
  search results or page text to the wrong query, with nothing in the trace
  to show it. Research traces from before the fix carry less evidential
  weight on which source answered which query.
- Between cells it is likely worse (inference): a goroutine's frame sits
  unread until the next cell; the worker loop drops non-cell frames
  (session_loop.go:328-329) and the goroutine sees a cell frame as an
  unexpected stream, so a goroutine hangs or a cell frame is lost.
- A tray verb such as `ReportPacket` called from a goroutine after its cell
  returned fails "requires a bound cell" (choir.go:839-847).

## Related hypothesis (unconfirmed)

The broker handle a desk session uses is issued with a one-hour lifetime
(choir.go:119, `time.Hour`), but a desk session has no age cap and outlives
runs (tools_desk.go:42-53, 108-116). A session older than an hour may have
every `choir.*` verb refused. Next observation: a staging desk session past
one hour.

## Hypotheses for the fix

1. Correlate by request id in the transport (a reader loop that routes
   responses to waiters), or at minimum hold one lock across write and read
   (correct but serial).
2. Concurrency belongs on the host: async search (the design agreed
   2026-10-12) starts provider calls on the host and lets the model read
   arrivals from REPL state, so model-authored goroutines need not call
   network verbs at all.
3. Until fixed, the research prompt must stop recommending parallel
   `choir.*` calls (affordance invariant).

Mutation class of a fix: orange (desk cell transport, prompt text).
Full study: docs/archive/reports/re-engineering-studies-2026-10-12/A-async-search-yaegi.md.
