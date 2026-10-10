# A host deploy's corpusd restart fails a guest's in-flight turn, and the run is left "running" (2026-10-10)

Status: documented, not fixed. Mutation class of the fix: `red` (computer
event chain client).

## Evidence

arXiv demo rerun on staging (test computer demo-arxiv-2026-10-10c,
vm-82cf0bb7…, guest on a74437902; traces in the session scratchpad under
traces/arxiv7). The owner prompt was submitted at 22:13:01. Texture run
00f8b67a started at 22:13:02. Meanwhile the deploy of 7ca33196 was
restarting host services on Node B.

Guest log, 22:13:27:

- `runtime: persist event …: store: create object: store: append
  projection batch: computer event appender: resolve head for new event:
  Get "http://10.200.59.1:8086/internal/computers/events/head?…": dial tcp
  10.200.59.1:8086: connect: connection refused` (four events)
- `runtime: update run 00f8b67a… to failed: …same error`
- `dispatcher: durable-invalid be4649cf…`
- `runtime: update run 00f8b67a… to running: …same error`, then `to
  failed` again.

Node B: `go-choir-corpusd` restarted around 22:13-22:15 (journal: stopped
22:15:09, started 22:15:10, serving on :8086). The run list still shows
00f8b67a as `running` at 22:16 with four events. The document never got
a second revision, and the demo stalled.

## Cause (from source)

`computerevent.HTTPClient.doWithResponseLimit`
(`internal/computerevent/http_client.go`) sends each request once. Every
durable write on the guest resolves the event head through it first
(`bindCurrentHeadLocked`). A host restart of corpusd, a two-second window,
therefore fails every write in flight. That includes the write that would
record the run's own failure, so the run stays `running` with no
executor behind it.

Deploys already avoid restarting busy guests ("deploys never restart a
busy computer"). The host services a busy guest depends on are restarted
anyway. A refused connection means the request never reached the server,
so retrying it is safe for any method.

## Belief state

Before: a deploy is invisible to busy computers. After: a deploy that
restarts corpusd fails whatever a busy computer was writing, and can
leave a zombie `running` run. Remaining error field: whether proxy or
vmctl restarts have the same effect on other guest clients. Those clients
are out of scope here; the event chain is the substrate every turn writes
through.
