# A long engineering run dies when corpusd's event CAS fails with "invalid connection" (2026-10-24)

Date: 2026-10-24, 07:5x UTC. Found by: the FP0 app-rewrite conductor on the
test computer computer-6c50d6e5 (b@c.com), staging, guest build 911be16b1
(booted 06:13:33 by a planned platform refresh). Run log:
`/Users/wiz/Dev/lanes/_runs/apprewrite/log.md` (outside the repo). Class of
this record: green. Class of a fix: red (the event tape append path, corpusd).
Status: cause found by FP0 (08:4x): the platform Dolt's liveness probe
aborted it mid auto-GC (strikes 3/3 at 06:43:09 and 07:51:50), so corpusd's
CAS commit died on its connection. That cause and its fix (D36,
GOMEMLIMIT under MemoryHigh, live 08:00:49) are recorded in
[platform-dolt-gc-throttled-at-memory-high](platform-dolt-gc-throttled-at-memory-high-2026-10-24.md).
Hypothesis 1 below (oversized payload) is refuted. What stays open here is
the second finding: a host blip loses the whole run.

## What happened, twice

Both runs were the same large frontend change: five apps in one engineering
assignment, on Synthetic DeepSeek V4.1 Flash. The model wrote big Go cells
(up to about 12 KB of source) and its context grew to 230 to 350 k input
tokens per call.

| Run | Started | Died | Iterations | Last guest log lines |
|---|---|---|---|---|
| run:assignment-85c82f45984254dabe028759 | 06:14:56 | 06:43:13 | 203 | `persist event f8dff403…: … computer event appender: head CAS: computer event client: corpusd returned 400: computer event CAS: commit: invalid connection`, then `assigned Engineering execution error could not join assignment fate …: tool loop persist assistant message: append projected run memory entry: … resolve head for new event: computer event client: corpusd returned 403: computer event capability refused` |
| run:assignment-f84bdbb75477b92189943b5d | 07:12:16 | 07:52:07 | 262 | `tool loop: slow tool call tool=capsule_go_eval … dur=54.036s err=reduce: record cell fate success: … append projected run memory entry: … head CAS: … corpusd returned 400: computer event CAS: commit: invalid connection`, then the same 403 `computer event capability refused` and "could not join assignment fate" |

(Source: `GET /api/runtime/observability` `recent_log`, read through the
product route.)

After each failure:

- The run stays `running` in `GET /api/runs` and never moves again. This is
  the [engineering-run-death-leaves-bound-assignment](engineering-run-death-leaves-bound-assignment-2026-09-29.md)
  shape. The capsule's work is lost.
- The computer's event appends worked again later. In the first case,
  `choir run cancel` of the zombie run at 06:49:41 appended its cancellation,
  and the head sequence moved 5853 to 5936.

## What is known and not known

- Known: the first error is corpusd's CAS commit failing (`400 … commit:
  invalid connection`). The `403 capability refused` on the next append
  follows within the same second. The run's fate append then fails, so the
  runtime cannot settle the run.
- Not known at the time (answered since, see Status): why the commit fails. Hypotheses:
  1. An oversized append body. A run memory entry is one message; DeepSeek's
     cells and printed outputs were large. "invalid connection" is what the
     MySQL protocol driver returns when a packet exceeds the server's
     max_allowed_packet.
  2. corpusd's Dolt connection was dropped by the host: load, a restart, the
     paper's work, or a deploy step. No platform deploy was visible at either
     time: the proxy stayed on 911be16b1, deployed 05:54:33.
  3. The 403 is a consequence: a failed CAS invalidates the guest's
     capability or its head binding.
- The luna runs earlier tonight were shorter, with smaller cells, and never
  hit this.

## What a fix has to do (not decided)

- Cause: found and fixed elsewhere (see Status).
- Open: a candidate is persisted only at Complete, so a run killed by a host
  blip after 30 to 40 minutes leaves nothing to carry; the next round starts
  from the base. A run's work should survive its run (a periodic or
  per-write candidate the carried-patch path can offer).
- Whatever the cause, a failed append must not leave a zombie run. The run
  should end with a visible fate ("interrupted: event store unavailable"),
  so management can re-cast it and the owner sees it in Texture.
- If the cause is size: bound or chunk run memory entries and cell
  outputs before they reach the tape, and refuse visibly at the cell.
