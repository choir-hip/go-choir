# Research fan-out overruns the computer's model-call budget; 239 children, 4 results (2026-10-11)

Mission: [research fan-out](../definitions/choir-research-fanout-2026-10-10.md),
slice S1b. Problem first; no fix here.

## Evidence

S1b demo on staging ce03d075 (test computer demo-arxiv-2026-10-10c,
document 211dfcb6, prompt submitted 00:37:47 UTC). Texture opened two
research desks at 00:38:12 (runs d544b140 and 2b3320b8). Each called
`choir.Research` with four children (no model named, no `timeout_ms`).
The children called `choir.Research` in turn.

Guest log, 00:38:37 to about 00:43 (`research child:` lines):

- 239 child activations ended; 4 returned findings (101 s, 156 s, 170 s,
  176 s). The rest ended in `context deadline exceeded`, most at loop
  iteration 0 or 1 with zero tokens, after 1 to 100 seconds.

Host gateway journal for this computer, 00:38 to about 00:43:

- 297 inference requests admitted (296 gpt-5.6-luna, 1 deepseek-v4.1-flash,
  which is Texture's model);
- 2,546 `rate limit exceeded for autoputer` rejections, 40 to 70 per
  second at peak.

Texture made one model call in five minutes. The document stayed at its
framing revision.

## Cause

1. **The computer's model budget is 60 requests a minute.** The gateway's
   per-computer limiter (`internal/gateway/ratelimit.go`, default 60 per
   1-minute fixed window; staging sets no override) admits the first 60
   calls in each minute and rejects the rest with 429 and `Retry-After`.
   It is shared by every desk and child on the computer.
2. **Nothing in the guest schedules within it.** `researchChildConcurrency
   = 4` bounds children per `choir.Research` call, but each child can call
   it again, so live loops grow by four per level. Every live loop calls
   the model as fast as it can.
3. **A rejected call fails fast and retries blind.** The guest gateway
   client retries a 429 twice after 0.5 s and 1 s
   (`gatewayruntime.Provider.call`), then the tool loop retries three times
   after 15 s each (`toolloop.go`), ignoring `Retry-After`. Most children
   spend their parent cell's remaining deadline on rejected calls and die
   without a single answer, and their parents die the same way.
4. **Desks compete with children on equal terms.** Texture's turn waits
   behind hundreds of child calls for the same 60 slots.

## Effect

The fan-out the owner asked for spent its whole budget on rejections. One
desk's worth of real work (four children) finished; the owner-facing
document did not move for five minutes.

## Fix direction

- Research children take model calls through one computer-wide gate sized
  under the gateway budget, so a tree of any depth or width queues instead
  of failing; the gate holds calls, not child lifetimes, so a parent
  waiting on its children never holds a slot its children need.
- The guest gateway client waits out a 429 for its `Retry-After`, bounded
  by the caller's context, instead of failing after 1.5 s.
- Desk calls (Texture, management, research desks) are not starved by
  children.
- Whether 60 calls a minute is the right budget for a computer doing deep
  research is an owner decision (provider account limits and cost); the
  design should work within whatever budget is set.

## Amendment (00:52Z): the tree never converges; the desks never return

At 00:52, fourteen minutes after the prompt, both research desks were
still inside their first `choir.Research` call (cells at 00:38:19 and
00:38:28). The guest log shows children ending at 35 to 185 a minute
through 00:51 (about 780 in all, 49 with findings), and the gateway still
rejecting this computer (2,938 rejections and 300 admitted calls between
00:46 and 00:52). Children delegate again before doing work, so the tree
keeps widening. A desk's Research call has no deadline of its own when
its cell sets no `timeout_ms`, and no child is ever told how long it has,
so nothing forces the tree back to the desk. No report reached Texture;
the document stayed at its framing revision.

Cause, in the terms of the settled design: M6.5 gives a child "a
carve-out of the parent's budget". 470a734c shared the egress budget but
carved out nothing, and time was not budgeted at all. Pacing (above)
bounds the rate, not the size or the duration of a tree.

Fix direction: carve time. A desk's Research call returns within a fixed
window; each child gets a share of its parent's remaining time and is told
it; a child with too little time left to fan out does the work itself.
Depth and width stay the model's choice within that time.
