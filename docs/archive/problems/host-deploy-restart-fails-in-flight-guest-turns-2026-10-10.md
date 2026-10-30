# A host deploy's corpusd restart fails a guest's in-flight turn, and the run is left "running" (2026-10-10)

Status: fixed in 880d3ffb (retry); the zombie-run residual below stays open. Mutation class of the fix: `red` (computer
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

## Fix and residual

880d3ffb makes `doWithResponseLimit` resend a request refused with
ECONNREFUSED, with backoff from 250 ms to 2 s, for up to 30 seconds and
within the request's context. A refused connection never reached corpusd,
so the resend cannot duplicate a write.

Residual, open: an outage longer than 30 seconds still fails the write
that records a run's own failure. That run stays `running` until the next
boot closes it as interrupted. Nothing retries a terminal run write later.

## Second instance: the gateway (model calls), 2026-10-10 23:32

Minesweeper demo on a114b46e (demo-ms-2026-10-10b, guest gateway address
10.200.62.1:8084). The 02e19146 deploy restarted `go-choir-gateway` (pid
93227 served calls through 23:31:54; pid 102507 listens afterwards).
Engineering's delegated run, three minutes into the first capsule spawn
that worked since the socket fix, was cancelled at 23:32:49. The parked
management turn woke on that report, and its first model call failed:
`gateway call failed: gateway client: http call: Post
"http://10.200.62.1:8084/provider/v1/inference": dial tcp
10.200.62.1:8084: connect: connection refused`. The run failed.

The residual named above ("whether proxy or vmctl restarts have the same
effect on other guest clients") is answered for the gateway: yes. The
guest gateway client sends once. Fix direction: the same bounded resend
on ECONNREFUSED as the event-chain client (a refused connection never
reached the server, so the resend cannot duplicate a model call).

## Third instance and clustering assessment (2026-10-10 23:33)

Research-only demo on a114b46e (demo-arxiv-2026-10-10c). The same
02e19146 deploy stopped the proxy, auth, platform, mail and gateway
services at 23:32:47 and started them at 23:34:35, a gap of 1 min 48 s,
then restarted them again at 23:34:40 (host journal). The guest could not
reach corpusd from at least 23:33:34. Event writes outlasted the 30 s
resend window, and guest credential renewal (`internal/selfdev/credentials.go`,
sends once) failed. Texture run 16e6bdbf could not record its own failure
and is a zombie `running` run. Three research results that arrived during
the gap were "consumed without a turn" (reconcile refused): research work
lost.

Clustering: four defects in one week share one cause. A deploy restarts
host services under busy guests (corpusd, gateway, auth/credential
renewal), and every guest-to-host client sent once or retried briefly.
The defects are the zombie run, the failed management turn, the cancelled
engineering run and the lost research results. The per-client resend
loops added in 880d3ffb (event chain) and 4fcf00ae (gateway) treat one
client each, and 30 s is shorter than the observed gap.

Substrate options:

1. Guest side: one shared transport for every guest-to-host HTTP client
   that resends when the dial was refused and the body can be replayed,
   for a window longer than a deploy gap (3 minutes), bounded by each
   call's context. It replaces the two per-client loops. Reversible; no
   deploy-shape change.
2. Host side: a deploy should not leave gateway, auth and platform stopped
   for nearly two minutes. That gap is the deploy's shape (what runs
   between stop and start). It is a deploy-shape decision for the owner,
   recorded here with the evidence. Not changed by this record.

The guest-side transport is the next commit. Lost occurrences ("consumed
without a turn") and zombie terminal writes remain residuals after it.

Where the gap comes from (from `.github/workflows/ci.yml`, Deploy to
Staging): when the host closure changes, `switch-to-configuration switch`
stops the changed `go-choir-*` units, runs activation, and starts them;
on 02e19146 that took 23:32:47 to 23:34:35. The script then restarts auth,
maild, corpusd, gateway and proxy again (23:34:40). Owner decision, not
changed here: whether host services should restart in place (no
stop-then-start across activation) and whether the second restart is
needed.
