# A correct Go loop never ends in a desk cell, and the runaway worker takes down the paper (2026-10-16)

Status: fixed in 3046fbb9a (the loop) and f93bbfd84 (containment),
deployed 20:02 and 20:19; outcome below. **The paper crashed at 18:55:52
and closed 16 runs; a worker died the same way three times at 12:30–12:46.**

## Evidence

The paper's guest kernel OOM-killed autoputer processes twice today
(guest: 7942 MiB, no swap; the only guest OOMs in Node B's retained
consoles):

| Time (UTC) | Killed | anon-rss | Effect |
|---|---|---|---|
| 12:46:39 | Texture desk worker, pid 3702 (run 512f3f66) | 4.5 GB | the cell failed with "frame: read header: EOF" |
| 18:55:52 | **the daemon**, pid 689 | 3.0 GB | crash boot at 18:56:20; 6 running and 2 passivated runs closed; the RSS polls in flight were dead-lettered |

Desk workers died the same way four more times, from the Go runtime at
the worker's 8 GiB RLIMIT_AS (exit status 2, a goroutine dump in stderr):
run 512f3f66 at 12:34:49 (260 s) and 12:39:44 (284 s); run 7349d774 at
18:51:09 (359 s).

**Every one of these cells has the same loop**, read from the runs'
`tool.invoked` events:

- 512f3f66 (12:30, 12:35, 12:40): `for i := 0; ; { j := strings.Index(s[i:], kw); if j < 0 { break }; …; out = append(out, …); i = pos + len(kw) }`
- 7349d774 (18:45 and its retry at 18:51, still open at the kill):
  `for i := 0; ; { j := strings.Index(s[i:], "{{cite"); …; out = append(out, …); i = j + k + 2 }`
- c81bb26b (open at the kill): `for i := 0; ; { j := strings.Index(out[i:], "Super Pac"); …; idxs = append(idxs, i+j); i = i + j + 9 }`

Each loop ends in real Go. At 18:55 two workers ran one at once; together
they outgrew the guest, and the kernel killed the largest single process,
the daemon.

## Mechanism (code-confirmed, reproduced locally)

Yaegi v0.16.1 (the latest release) wires a loop with an init statement,
no condition and no post statement (`forStmt1`) back to the init on every
iteration (`interp/cfg.go:1424-1428`: `n.start = init.start; body.tnext =
n.start`). The body's `i = …` is overwritten by `i := 0` before the next
pass, so `strings.Index(s[0:], kw)` finds the same hit forever and the
slice grows until memory runs out.

A local probe in `internal/yaegikernel` (session, default allowlist):

| Shape | Iterations (guard at 6) |
|---|---|
| `for i := 0; ; { …; i = i + j + 1 }` | runaway, `i` stays 0 |
| `var i int; for i = 0; ; { i++ … }` | runaway: the init re-runs |
| `for i, k := 0, 0; ; { … }` | runaway |
| `for i := 0; true; { … }` | correct |
| `for i := 0; i < 3; { i++ }` | correct |
| `for i := 0; ; i++ { … }` | correct |
| `for ; ; { … }`, `i := 0; for { … }` | correct |

The model gets no signal it can act on: the cell returns "frame: read
header: EOF" after four to six minutes with empty stdout, so it re-runs
the same shape (512f3f66 three times, 7349d774 twice).

## Why nothing bounded it

- **The worker's hard cap does not fit the guest.** `deskWorkerMemoryLimitBytes`
  is 8 GiB of address space (5de819d58, raised from 1 GiB because the
  re-executed autoputer binary maps more than that at start). The guest
  has 7.9 GB, so one worker can fill it and two certainly can. Its doc
  comment says the cap stops a cell that would "OOM the daemon"; on the
  paper it does not.
- **The soft limit does not bind.** The worker's GOMEMLIMIT (496 MiB)
  makes the GC work harder; it never stops a heap that is all live.
- **No Texture cell deadline fired.** The kills came at 260–360 s.

## Class

The substrate is the interpreter, not the model: a valid Go program
behaves differently in a desk cell than in Go. Related ergonomics fixes
live in `cell_normalize.go` (K4, K9, the leading-func guard), which
already rewrites cell source so yaegi runs it as Go would.

## Fix direction

1. **Interpreter (orange):** in `normalizeCellSource`, rewrite every
   `for init; ; {` to `for init; true; {` at the token level, before any
   other normalization. The two are the same Go program, and the
   constant-true condition takes yaegi's correctly wired `forStmt3` path.
   It runs on every cell shape (file, declaration fragment, statement
   fragment, import mix), because a token scan needs no parse. A
   regression test runs the three production loops above. Not chosen:
   a forked yaegi (a whole-module vendor for a one-line fix); a newer
   yaegi (none exists).
2. **Containment (red, next):** a cell that runs away for any other
   reason still fills the guest. Bound a worker by resident memory, not
   address space (a daemon-side RSS watchdog or a cgroup per worker), and
   tell the model "the cell exceeded its memory budget" instead of
   "frame: read header: EOF". Residual until then.
3. **Upstream:** report the `forStmt1` wiring to traefik/yaegi (needs the
   owner's go-ahead: outward-facing).

## Outcome (20:20)

- **3046fbb9a** rewrites `for init; ; {` to `for init; true; {` in
  `normalizeCellSource`. The three production loops end in a cell (they
  print the runaway guard without it). CI green; deployed 20:02, and the
  paper's planned refresh booted with tail=0 in 3 s.
- **f93bbfd84** (containment, the part of fix direction 2 that needs no
  new subsystem): each desk worker sets its own `oom_score_adj` to 1000
  (best effort), so a guest OOM takes a worker before the daemon; a
  worker that dies without a deadline now tells the model how it ended
  and the usual cause, instead of "frame: read header: EOF". On Linux CI
  `TestDeskWorkerVolunteersForTheOOMKiller` passed (not skipped): a real
  re-executed worker reads 1000, the test process does not. Deployed
  20:19 (tail=0, 3 s).
- **Not yet seen live:** a loop-shape cell on the paper after 20:02, and
  a worker death message in a run's tool result.
- **Still open:** an RSS budget per worker (a runaway cell still grows
  until the guest is short, and a guest OOM still kills, now a worker);
  reporting the `forStmt1` wiring upstream (outward-facing, owner's call).
