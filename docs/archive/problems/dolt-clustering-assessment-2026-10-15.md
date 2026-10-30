# Dolt clustering assessment: the platform deadlock, the storage leaks, and what to do (2026-10-15)

Status: assessment and recommendation. Nothing was implemented or changed on
Node B. Mutation class of this document: green. Mutation class of the
repairs it proposes: red (Store A is the canonical event/control store).

Written under AGENTS.md "Convergence Before Patching" and "Apply Root Cause
Clustering". Trigger: the platform Dolt deadlock
([platform-dolt-deadlock-hangs-checkpointd-and-crashloops-vmctl-2026-10-15](platform-dolt-deadlock-hangs-checkpointd-and-crashloops-vmctl-2026-10-15.md))
is at least the seventh Dolt problem in three weeks. Owner question: "What are
we going to do about Dolt? Do we need more monitoring? We've been battling
different kinds of storage leaks and such for a while, so it may be related."

Tags: **[E]** evidence (journal line, path, file:line, measurement taken
2026-10-15 ~10:50-11:00 UTC on Node B, read-only: journald, `du`/`ls`,
`/proc`, cgroup files; no SQL was run and no credential was read).
**[H]** hypothesis.

## Verdict in four lines

1. **Root cause of the hang: not confirmed.** No goroutine stack exists. The
   best-supported hypothesis is a stall in Dolt's online-maintenance path
   (auto GC / conjoin / commit) against 24 GB of old-generation history. [H]
2. **The earlier timeline was wrong.** The server did not die at 09:49. It
   degraded from 09:23 and stopped answering between 10:05 and 10:06. [E]
3. **Related to the storage leaks: yes, by shared input, not proven by
   mechanism.** The same unbounded-history, Dolt-as-event-tape workload feeds
   the leaks and keeps auto GC busy about every 15 minutes. [E for the
   workload, H for the causal link]
4. **Recommendation:** make Dolt boring first (probe, bounded clients, stop
   committing history on Store A), capture a stack on the next stall, and
   decide on replacing Dolt for Store A only if it stalls again after that.

## 1. What happened, from the Node B journal

### 1.1 Corrected timeline [E]

| Time (UTC) | Event | Source |
|---|---|---|
| to 09:15 | `checkpointd` runs finish in under 1 s (all of the 07:30-09:20 buckets have max 1 s) | `journalctl -u go-choir-checkpointd`, wall-clock from Starting to Failed lines |
| 09:02:35 | auto GC completes in 2.4 s (30 completions between 21:35 and 09:30, one every 3-120 min) | `sqle/auto_gc: Successfully completed auto GC of database platform in 2.356351445s` |
| **09:23:05** | **"beginning conjoin of database ... generation=old upstream_len=257"** and, the same second, auto GC completes in 3.49 s. This is the only conjoin line in the unit's whole 13 h lifetime and in the journal since 10-01 | `dolt[1404751]` 09:23:05 |
| **09:23:20** | the next `checkpointd` run starts and **takes 4 min 33 s** (normally under 1 s). It ends at 09:27:53 | `go-choir-checkpointd.service: Consumed 117ms CPU time over 4min 33.716s wall clock time` |
| 09:27:53 | six new Dolt connections and seven closes in one second: the queue flushes. The same second shows a vmctl `/internal/vmctl/stop` lifecycle request | Dolt log 09:27:53; vmctl log |
| 09:28-10:05 | every `checkpointd` run is slow and getting slower. Max wall time per 10-minute bucket: 9 s, 35 s, 91 s, 189 s | computed from the Starting/Failed journal lines per 10-minute bucket: 09:30 max 9 s, 09:40 max 35 s, 09:50 max 91 s, 10:00 max 189 s |
| 09:49:02 | the last `error running query ... nothing to commit` (connection 843). This is the line the first write-up treated as the last sign of life. It is only the last *warning*; successful queries and closes keep appearing after it | Dolt log |
| 09:49:14-09:51 | P5 deploy's first switch. `dbus-broker` reload times out at 09:50:45 and the switch fails (status 4); a second switch succeeds at 09:50:51. Platform service, vmctl, gateway restart and their Dolt connections close normally at 09:50:54 | `nixos[1633864]`, `nixos[1634188]` |
| 09:50-10:05 | Dolt still answers: `ConnectionClosed` lines continue (10:00:48, 10:01:29, 10:03:27, 10:05:06) | Dolt log |
| 10:04:57 | **first hung `DOLT_COMMIT`**. The first `doltbatch platform: snapshot commit failed: dolt commit: context deadline exceeded` is logged at 10:06:57. The committer uses a 2-minute timeout (`internal/doltbatch/committer.go`, `fire`), so it started at 10:04:57 | service log 10:06:57 |
| **10:05:06** | **last `ConnectionClosed` of the server's life** (connection 868, a panicking `checkpointd`) | Dolt log |
| 10:06:06 | `checkpointd` connection 869 opens and is never closed. Nothing is ever closed again; 24 connections are open by 10:45 (normal is 3-5) | Dolt log; connection reconstruction |
| 10:06:57 to 10:39:57 | a commit-timeout line every 165 s (120 s timeout plus the 45 s retry), 13 lines. This is the earliest machine-readable alarm | service log |
| 10:09-10:45 | switch for P4 (97e023100) blocks on a `checkpointd` start stuck on Bootstrap DDL; vmctl crash-loops on its route-DB ping | incident doc |
| 10:45:56 | restart; SIGTERM; at 10:46:04 the operator's SIGQUIT prints exactly three lines (`SIGQUIT: quit`, `PC=0x49d6a1 m=0`, `goroutine 0 ... [idle]:`) and the dump stops | `dolt[1404751]` 10:46:04 |
| 10:47:26 | SIGKILL after the 90 s stop timeout. Lifetime: 13h11m wall, 1h08m CPU, 3.5 G memory peak, 142.6 G read, 62.9 G written | systemd `Consumed` line |

Corrections to the incident doc: (a) hang onset is about 10:05, with
degradation from 09:23, not 09:49; (b) SIGQUIT was *received* (the Go runtime
started its dump) but the dump stalled after the header. "Ignored SIGQUIT" is
not what the journal shows. A stalled traceback is itself evidence about the
process state, but I cannot say which state. [E]

### 1.2 What the clients were doing [E]

- Connections are few and short. 889 connections were opened in 13 h. Open
  count at 22:00, 03:00, 09:00, 09:23, 09:49 and 10:05 was 3, 3, 3, 4, 4, 5.
  There was **no connection or transaction leak before the stall**; the 24
  open connections at 10:45 are a consequence (clients retrying behind a dead
  server), not a cause.
- The long-lived clients are the platform service's pool (max 5 open, 30 min
  lifetime, `internal/platform/store.go:267-269`) and one committer.
- Dolt's listener timeouts are the 8 h defaults (`Config ... T="28800000"`
  in the start line; `read_timeout_millis` and `write_timeout_millis` are
  commented out in `/var/lib/go-choir/platform-dolt/platform/config.yaml`),
  `max_connections` is the 1000 default, and `behavior.auto_gc_behavior`
  is the commented default (`enable: true`, `archive_level: 1`). The server
  never reaps a stuck session for 8 hours.
- Platform Dolt is **Dolt 2.1.9** (`dolt version`, unit ExecStart path;
  upstream newest is 2.4.2 per the CLI's own warning).
- A confounder, not a cause as far as I can tell: **`checkpointd` has been
  panicking once a minute since 2026-10-14 23:07** (357 `panic:` lines on
  10-15; `mkdir /root/.dolt: read-only file system` then a nil dereference in
  `env.(*DoltCliConfig).createLocalConfigAt`). Each run connects to Dolt (one new connection a minute in the Dolt log) and,
  per the incident doc's stack, re-applies the 5.7 KB schema DDL, then dies.
  That is about 1,440 DDL statements a day against Store A from a crash loop.
  Before that the unit ran about 1,270 times a day without panics. [E] Whether the
  DDL contends with a GC or conjoin is [H].
- Another unexamined confounder: the paper VM's health check failed at
  09:32:51-09:33:21 (`vmmanager: VM vm-1669... is unhealthy`) and WS relays
  to two guests timed out at 09:53-09:54. Those guests have their own
  embedded Dolt; I did not trace them. [E that it happened, H on relevance]

### 1.3 The Dolt server's shape [E]

| Item | Value | Source |
|---|---|---|
| Data dir | 25 G (`/var/lib/go-choir/platform-dolt`); corpus Dolt 23 G | `du -sh` |
| `noms/oldgen` | 24 G in 260 `.darc` archives | `du`, `ls` |
| Journal (`vvvv...`) | 230 MB six minutes after the restart; `journal.idx` 3.5 MB | `ls -laS` |
| Commits (`dolt_log`) | 90,385 on 10-14 00:08, **92,290 on 10-15 00:00** (+1,905/day); the 08-26 incident store had 6.9 M | history-audit journal lines; `docs/archive/evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md` |
| oldgen growth | 22.77 G -> 24.38 G in 24 h (+1.6 G/day) while the live set is much smaller; the 09-18/19 import is 19.6 G of it | history audit; `ls --time-style` |
| Unit limits | `MemoryHigh=3G`, `MemorySwapMax=0`, `OOMScoreAdjust=-500`, no `MemoryMax`, no watchdog, default 90 s stop timeout | `systemctl cat`; `nix/node-b.nix:687-715` |
| Memory | cgroup at 3.11 G four minutes after boot (anon 2.0 G, page cache 1.0 G); the last two lifetimes peaked at 3.0 G and 3.5 G, i.e. at or over `MemoryHigh` | `memory.stat`; systemd `Consumed` lines |
| GC | auto GC enabled (default); 30 completions in the 12 h before the stall; read 142.6 G and wrote 62.9 G in 13 h (the previous 25 h life wrote 152 G) | journal |
| Host | not under memory pressure at stall (31 GiB, ~19 GiB available now); no kernel `err`/hung-task lines in the previous boot; host rebooted 10:49 | `free`, `journalctl -k -b -1 -p err` |

## 2. Is it related to the storage leaks?

**Shared input: yes. Shared mechanism: unproven.**

- The platform store is not growing without bound in the 08-26 sense. It was
  squashed after the 218 G incident and is at 25 G, +1.6 G/day, 92 k commits.
  That is still an unbounded series: there is no committed bound and no
  squash scheduled. The history audit's thresholds are 1,000,000 commits or
  50 GiB oldgen (`nix/node-b.nix`, `GO_CHOIR_DOLT_MAX_COMMITS`,
  `GO_CHOIR_DOLT_MAX_OLDGEN_KIB`), so it will not fire until the store has
  doubled. [E]
- Store A history has **no reader**. Outside `internal/store` and the guest
  compaction in `internal/projectionbase/compact.go`, no Go code uses
  `AS OF`, `dolt_log`, `dolt_diff` or `dolt_history`. The platform comment
  already says the snapshot is "bookkeeping" and SQL COMMIT is the durability
  (`internal/doltbatch/committer.go`, package comment). Every ~45 s the
  committer runs `CALL DOLT_COMMIT('-Am', ...)` over the whole working set.
  [E] The first-day finding of `store-a-dead-wire-data-and-unbounded-history`
  (declare Store A history disposable) was never decided.
- GC is on a short cycle on Store A: roughly one auto GC every 15 min by day,
  each 1.5-7.5 s, plus a conjoin of 257 old-generation files at 09:23. The
  stall began 15 s after the only conjoin the server ever logged. [E for the
  timing, H for causation.] Upstream material I could check says:
  online GC "blocks on any in-progress work until those sessions have
  registered their roots", a long-running statement "can keep a dolt_gc from
  completing", and no timeout is described
  ([session-aware GC](http://www.dolthub.com/blog/2025-03-21-session-aware-gc-technical-details/));
  auto GC is documented as "experimental"
  ([announcement](https://www.dolthub.com/blog/2025-02-28-announcing-automatic-gc-in-sql-server/));
  v1.84.1 fixed a conjoin-during-GC hazard and 2.1.11 fixed an unrelated
  clone deadlock ([2.1.11 notes](https://newreleases.io/project/github/dolthub/dolt/release/v2.1.11)).
  I found **no upstream issue that matches** this hang. Version 2.1.9 predates
  the 2.1.11 fix. Web search was limited; the dolthub/dolt issue tracker
  itself was not browsed.
- The competing hypothesis is cgroup throttling at `MemoryHigh=3G` with swap
  disabled. It fits "progressive slowdown", "S state, 0% CPU" and "peak above
  the high mark". It does not fit one hour of zero progress, zero I/O and a
  stalled traceback: a throttled task still sleeps in bounded slices (the
  kernel's over-high delay is capped at about 2 s, from kernel knowledge, not checked here) and makes progress. The
  `memory.events` counters died with the cgroup, so it cannot be checked
  retroactively. [H, low weight]
- The honest reading: the platform store is large, history-heavy and being
  garbage-collected online all day while serving a 45 s commit cadence and a
  per-minute DDL crash loop. A stall in that interaction is plausible and
  consistent with the trace. The data do not isolate it.

## 3. Clustering assessment (the contract's required write-up)

### 3.1 The cluster

| Problem (date) | Store | Surface | Substrate input |
|---|---|---|---|
| Per-mutation `DOLT_COMMIT`: 218 G / 6.9 M commits (08-26) | platform | disk | **unbounded history** |
| Platform Dolt OOM, realization flap (10-01) | platform | memory | in-process engine at 23.7 GiB RSS |
| Guest embedded Dolt memory-starved (10-01) | guest | memory | in-process engine, working set |
| Guest journal leak, 27 GiB (10-03) | guest | disk | **journal, GC guard ordering** |
| GC sweep holds the service write lock (10-08) | platform service | locks | **maintenance holds a foreground lock** |
| Guest history bloat, 5x live data (10-09) | guest | disk | **unbounded history** |
| Store A 22 GB of dead data and 86 k commits (10-09) | platform | disk | **unbounded history** |
| Embedded Dolt single connection / shared cache race (10-11) | guest | concurrency | one engine, one connection |
| Guest Dolt GC ran only at boot; paper disk full (10-15) | guest | disk | **journal + history**, GC cadence |
| **Platform Dolt stall (10-15)** | platform | liveness | **maintenance vs. foreground, unobserved** |

### 3.2 Do they share a substrate cause?

**Yes, one primary and one secondary.**

*Primary:* **Dolt is being used as a high-rate append-only event tape, and
every written batch becomes retained, versioned history.** Evidence: guest
stores take ~3 tape events/s and commit every 100 events (3,595 commits and
27 GB of journal in 32 h on the paper computer, `guest-dolt-storage-growth`);
Store A commits every 45 s (~1,900/day) with no reader of history; the 08-26
incident had 6.9 M commits. Dolt keeps everything reachable from any commit,
so GC cannot reclaim it (the platform incident panel and the guest
compaction experiment, 8.9 GiB -> 1.7 GiB only after a squash plus
`gc --full`, agree). Four of the ten rows above are this cause, and every
"fix" so far has been a bound added per surface (guest GC guard, boot GC,
online GC on a 5 minute timer, the pending paper-only compaction) rather than
removing the retained history.

*Secondary:* **the maintenance that bounds history (GC, conjoin, squash,
commit) runs inside the same process and, in places, behind the same lock as
the foreground path, and nothing watches it.** Rows: the 10-08 write-lock
hold, the 10-03 GC guard that suppressed GC, the 10-15 boot-only GC, and the
10-15 stall, whose timing sits on a conjoin and GC. This is [E] for the
first three and [H] for the stall.

*Not shared:* the 10-01 memory rows and the 10-11 single-connection row are
different mechanisms (the first partly mitigated by cgroup limits, the second
by upstream's fix). They share the *deployment* (embedded engine in the
product's critical path) but not the cause.

### 3.3 Existing replacement and deletion check (contract steps)

- **Replacement that exists:** `internal/doltbatch` already replaced
  per-mutation commits with a debounced committer (2026-09-19). It cut the
  rate but kept the series unbounded. A *no-commit* mode is a deletion of a
  behavior, not new code. [E]
- **Replacement not wired:** the pending branch
  `fix/bound-guest-dolt-history` (`git log main..fix/bound-guest-dolt-history`:
  `898b1a58c`, `84cb0c2a2`, plus two docs commits) squashes history online and
  runs `DOLT_GC('--full')` past a 2 GiB budget, **paper computer only**,
  fleet-off by default. It is the right mechanism for guests and does nothing
  for Store A. Note it adds a *new* online maintenance operation on the guest
  path; given the Store A stall, ship it behind the same liveness probe, and
  keep its boot-tick skip and jitter (they are in the branch). [E]
- **Prefer deletion:** the history audit squash runbook, the debounced
  committer, the guest 5-minute online GC, the paper compaction and the
  per-surface size guards are all additions that exist because history is
  retained. Removing the retained history on Store A removes the need for
  most of them there.

### 3.4 Bound on this assessment

Three or more symptoms in one subsystem within a week is the contract's
trigger; this is ten in three weeks. The next action is substrate-level
(section 5), not another per-symptom patch. Dissent recorded: the stall
could be a plain upstream bug in 2.1.9 that history size only makes more
likely; in that case a version bump is the fix and the clustering above
explains only the leaks. Both are served by the same first steps.

## 4. Monitoring gap

### 4.1 What exists [E]

- `go-choir-platform-dolt-history-audit` (daily, `nix/node-b.nix:225-262` and
  `:1252`): counts commits and oldgen size and exits nonzero past thresholds
  (1 M commits, 50 GiB, +10 k/day). Comment says "failed unit -> monitoring",
  but `nix/node-b.nix` has **no `OnFailure=` and no alert sink**; only
  `nix/autoputer-vm.nix:548` has one, for a guest signer. The audit is a log
  line nobody is paged by. It last wrote `92290`.
- `scripts/verify-host-hardening.py` checks Dolt **authentication** (empty-root
  handshake, app user, file modes). It is not a liveness check.
- Dolt unit: `Restart=on-failure`, start limit 10 in 600 s, no `WatchdogSec`
  (the file records why: the Go servers do not send `sd_notify`;
  `nix/node-b.nix:534-536`). Dolt itself is not a notify service either.
- `vmctl` pings the route DB once at boot with 30 s and then
  `log.Fatalf` (`cmd/vmctl/main.go:196-197`), which with the start limit
  produced the crash loop.
- `doltbatch.Committer.LastError()` exists "for health surfaces" and the
  timeout logs a line, but no probe reads either.

### 4.2 What would have caught it at 09:50 instead of 10:44 [E on signal availability]

Three independent, already-present signals were firing and unread:

1. **`checkpointd` wall time** rose from under 1 s to 273 s at 09:23-09:27
   and 35-189 s after 09:40.
2. **Commit timeouts** from 10:06:57, every 165 s.
3. **Zero `ConnectionClosed` lines** after 10:05:06 with a rising open count.

A `SELECT 1` with a 5 s deadline against the app user every 30 s would have
failed at ~10:05:30 (it would have flagged the 09:23 stall too if its
deadline were 2 s; the 4.5 min checkpointd run is exactly such a query
latency). Dolt's own handshake succeeds when queries hang, so a TCP or
`mysqladmin ping` check would not have caught this.

### 4.3 Proposals ranked by value for cost

| # | Action | Cost | Value | Notes |
|---|---|---|---|---|
| 1 | **Query-level liveness probe** (timer, every 30 s): `SELECT 1` with 5 s deadline as the app user via `LoadCredential`, against 13306 and 13307. After 3 failures: record `/proc/<pid>/{status,stack,wchan}` and `cgroup memory.events/pressure` to the journal, `systemctl kill -s ABRT` (Go dumps all goroutines on SIGABRT, which the SIGQUIT path failed to do), then let `Restart=on-failure` start it. Set `TimeoutStopSec=20s` on the Dolt units (cost 90 s on this restart) | small (one script, one unit, one timer) | detects at ~10:06, restarts within ~3 min, and **captures the missing stack** | the only item that both repairs and explains. Needs a credential-handling review (red surface) |
| 2 | **Bound the clients** (the incident doc's candidate repairs 1-3): deadline on `platform.OpenStore`/`Bootstrap`, `TimeoutStartSec` of minutes on the `checkpointd` oneshot, stop bootstrapping the schema from `checkpointd`, and make `vmctl` retry its route-DB ping with backoff instead of `log.Fatalf` | small to medium, three files | stops fate-sharing: a dead Dolt no longer blocks a deploy for 30 min or crash-loops vmctl into start-limit-hit | does not detect or fix the stall. Also stop the `checkpointd` panic loop (the 357 panics) or it keeps generating DDL load |
| 3 | **Alert on signals that already exist**: `OnFailure=` for the audit and probe units into one sink, plus a threshold on the history audit well below the current 1 M / 50 GiB (e.g. 150 k commits, 40 GiB) and a daily line for journal size, Dolt RSS and `memory.events high`. Export `doltbatch.LastError()` on the platform `/health` | small once a sink is chosen; the sink does not exist | turns the existing audit into a signal and gives early warning on size | per-connection transaction age was proposed in the task; I found no evidence of leaked transactions in this incident, so it is **lower value here** than the above |
| 4 | systemd watchdog | medium (needs sd_notify) | covers hangs of Go services we own, not Dolt | skip for Dolt; revisit for vmctl |

## 5. Recommendation

**Do three things, in this order, and decide on replacement only after the
second stall (if any) has a stack.**

1. **Ship probe-with-ABRT (4.3 #1) and bound the clients (#2).** Cheapest, and
   the only way to turn "hypothesis" into "confirmed". Include a Dolt bump
   from 2.1.9 toward the current release in the same window as a *candidate*
   fix, but only after the probe is in so a recurrence is attributable; the
   2.1.11 release notes do not list this bug, so a bump is not a known fix.
2. **Remove history from Store A.** Declare Store A Dolt history disposable
   (nothing reads it; section 2), make the platform committer a no-op or a
   weekly squash, and run the squash plus `gc --full` runbook once to take 24
   GB to the live set. This deletes the main input to both the leaks and the
   GC/conjoin load, and aligns Store A with the doctrine that the tape, not
   Dolt history, is the authority. Mutation class red: pre-ceremony needs a
   conjecture delta, rollback (restore from the pre-squash copy; disk has
   130 GB free), and heresy delta. Same treatment for the guest via the
   pending branch, gated by the probe.
3. **Keep Dolt for Store A for now.** Judged by evidence: the leaks are
   explained by how we use it (history retention), not by Dolt failing at its
   job, and the one liveness failure in 13 hours of observation is not yet
   attributed. Dolt gives Store A nothing it uses (no branches, no history
   reads), so it is replaceable by a plain MySQL-wire or SQLite/Postgres
   store with moderate work (the clients speak the MySQL wire; most of the
   SQL is plain), but a migration of the canonical log is a red,
   weeks-scale mission and not justified by one unexplained stall. **Trigger
   to start it:** a second stall after steps 1 and 2 with a captured stack
   pointing inside Dolt, or a stack that shows GC/conjoin holding a lock.

## 6. What to capture next time (no fix here)

Before restart: `cat /proc/<pid>/status /proc/<pid>/task/*/wchan`,
`/proc/<pid>/stack`, `cat .../memory.events .../memory.pressure
.../memory.stat`; then `kill -ABRT` and read the journal (the SIGQUIT dump
stalled after three lines this time, so also try `gdb -p ... thread apply
all bt` if installed, or `eu-stack`). Record the `checkpointd` and
`doltbatch` timing lines as the onset.

## Residual unknowns

- Whether the dump stalled because the process was wedged in the kernel,
  blocked on stderr to journald, or inside the runtime. Not determined.
- What released the 4.5 min stall at 09:27:53. The coincidence with a vmctl
  VM stop is not evidence of a link.
- Whether the corpus Dolt (13307, 23 GB) is exposed to the same stall; its
  journal shows only connection lines from 09:00 to 10:46 and it carries 1,246 commits, so history size is a
  difference worth testing rather than assuming.
- Whether the P5 switch (and its two `dbus-broker` reload timeouts at 09:50:45
  and 10:10:46) is a host-level effect; both switches ran after the 09:23
  degradation began, so they are not the cause of it.

## Heresy delta

- discovered: the incident timeline in the deadlock doc was off by 40 minutes
  and mischaracterized SIGQUIT; no host alert sink exists although the
  history audit assumes one; `checkpointd` has been in a panic loop since
  2026-10-14 23:07; the history-audit thresholds (1 M / 50 GiB) are far above
  the current store.
- introduced: none (documentation only).
- repaired: none.

## 7. Guardrails implementation record (branch fix/dolt-guardrails)

Written before any guardrail code, per the Testing rule ("write down all the
ways it could fail, then write the code") and Problem Documentation First.
Mutation classes: checkpointd HOME and deadlines orange; vmctl red; the probe
red-adjacent (host ops). Nothing here was run against the real Dolt servers;
the Node B evidence below is read-only `journalctl`, `ls` and `systemctl show`.

### 7.1 New finding: the checkpointd panic loop has a one-line cause [E]

The loop in section 1.2 is not a Dolt bug and not a crash-loop of the DDL
path. It is the F3 hardening commit `fda386b47` (2026-10-14 15:35 -0400),
which added `ProtectHome = "tmpfs"` to `commonServiceHardening`
(`nix/node-b.nix:318`). That mounts `/root` as an empty read-only tmpfs.
`go-choir-checkpointd` does need an embedded Dolt workspace: when a
projection job is claimed, `projectionbase.Rebuilder` opens a scratch store
(`internal/projectionbase/rebuilder.go:224,273`) through `store.Open`
(`internal/store/store.go:630`, `openTextureWorkspaceDB` at
`internal/store/texture.go:211`), and the embedded Dolt engine creates
`$HOME/.dolt` (global config) on `CREATE DATABASE`. With `HOME` unset the
unit's home is `/root`, so `mkdir /root/.dolt: read-only file system`, and
Dolt then dereferences the nil config in `createLocalConfigAt`
(`dolt/go/libraries/doltcore/env/config.go:143`).

Evidence (Node B, read-only):

- Before 23:07:19 on 10-14 every run was a 0.15-0.2 s no-op. The first
  `store: open phase=workspace-open` line of the loop is 23:07:20, and the
  panic follows in the same second. 568 `workspace-open` lines and 65
  `checkpoint published` lines exist since 10-01; the published runs took
  43-56 s (maximum 55.8 s). So the embedded workspace is legitimate and used,
  and it worked until the hardening deployed.
- `/root/.dolt` exists on the host (written by earlier, unsandboxed runs);
  under `ProtectHome=tmpfs` the unit cannot see it.
- Of the host units, only `checkpointd` runs the projection rebuilder.
  `corpusd` sets `HOME` explicitly (`nix/node-b.nix:842`); the Dolt servers
  set `HOME` to their data dirs.

Consequence: a job is claimed every minute and the process dies before
`FinishProjectionJob`, so that computer's checkpoint has not advanced since
23:07 (the claim repeats; [H] that exactly one computer is affected, the
journal does not name the computer). Correction to section 1.2: the
per-minute connection and DDL come from `OpenStore`/`Bootstrap` on every run,
as stated, but the panic is after the job claim, not in the DDL path. Heresy
discovered: a security-hardening change broke a maintenance unit for 12 h and
nothing alerted (no `OnFailure=`, section 4.1).

### 7.2 Failure modes to design against

**Commit 1: give checkpointd a writable home (orange).**

- The new HOME directory is missing or not writable (StateDirectory creates
  it root-owned 0750; the sandbox is `ProtectSystem=strict`). Design: use
  `StateDirectory=go-choir/checkpointd` (writable by construction) and add a
  preflight in `main` that creates `$HOME/.dolt` before the job claim, so a
  future sandbox change fails fast with a clear line and without claiming a
  job, instead of panicking after the claim.
- Dolt needs more than `.dolt/` (a global user config). The scratch store
  passes `commitname`/`commitemail` in its DSN (`texture.go:233`), so no
  global identity is required. Only a staging run can prove the rest.
- The fix un-wedges a 12 h backlog: the first run may rebuild immediately.
  Resource limits are unchanged (`MemoryMax=6G`, `CPUQuota=100%`,
  `OOMScoreAdjust=1000`) and the observed maximum duration is 56 s, which the
  deadline in commit 2 must exceed with a wide margin.
- Persistent `HOME` across runs accumulates state: it holds only a tiny
  config file; the rebuild scratch stays under `platform-artifacts`.
- Another unit hits the same wall: none found (see the evidence above).

**Commit 2: client deadlines (orange).**

- A 60 s open deadline is too short for a legitimately slow DDL on a loaded
  Dolt, so `OpenStore` fails where it used to wait. `corpusd` already retries
  20 times (now each attempt bounded), so its worst case becomes 20 x 60 s
  plus backoff instead of unbounded; `checkpointd` fails and the timer
  retries in a minute.
- A cancelled multi-statement DDL leaves partial schema. The DDL is
  `CREATE ... IF NOT EXISTS` and ensure-column, re-applied on the next run.
- The client's cancel does not stop a hung Dolt session. Connections can pile
  up server-side on a deadlocked server (24 open at 10:45). The deadline
  bounds the client and the deploy; the probe (commit 4) addresses the server.
- The open context must not become the worker's lifetime context. Only
  `OpenStore` takes the 60 s context; the worker keeps the signal context.
- `TimeoutStartSec` too small kills a legitimate rebuild every minute
  forever. Observed maximum 56 s over 65 publishes; choose 10 minutes (about
  10x), down from 8 h. A deploy switch can then be held at most 10 minutes by
  a `checkpointd` start.
- Dropping the DDL from `checkpointd` opens a skew window: a new
  `checkpointd` binary may run against the previous schema until `corpusd`
  restarts and bootstraps (the deploy restarts it right after the pointer
  update; on a fresh Dolt, until `corpusd` first runs). Effect: a few failed
  one-minute runs with an SQL error, healed by the next `corpusd` start.
  Accepted, and recorded as the price of making `corpusd` the single schema
  owner.

**Commit 3: vmctl must not crash-loop on a slow ping (red).**

- Retrying forever masks a permanent fault (bad DSN, bad password). Design:
  log every attempt with the error and the next delay; the unit stays
  `active` but vmctl does not serve, so deploy health checks fail visibly
  instead of the unit hitting `start-limit-hit`.
- vmctl serves nothing while it waits (no listener until the authority is
  set). The route authority is a plain field read by reattach, the idle
  sweeper and the handlers, and `RequireRouteAuthority` makes a nil authority
  fail closed; attaching it later from a goroutine would be an unsynchronised
  write and is not attempted. Not worse than today: a crashed vmctl serves
  nothing either.
- The wait must honour SIGTERM so a switch stop does not hang for the 90 s
  stop timeout.
- Backoff too tight hammers a sick Dolt: capped exponential, 1 s to 30 s,
  each attempt bounded at 30 s.
- A partial `EnsureSchema` followed by a retry: both DDLs are idempotent.
- The unit's `StartLimit` (10 in 600 s, restart-subsystem L1a) stays: it is a
  deliberate no-crash-loops policy, and in-process retry removes the exit
  that fed it.

**Commit 4: platform and corpus Dolt liveness probe (red-adjacent).**

- False positive during a deploy switch or boot: skip unless the unit is
  `active`, and skip for 120 s after `ActiveEnterTimestamp`. Any skip resets
  the strike count.
- False positive on a long GC or conjoin: the 09:23 stall lasted 4.5 minutes
  and cleared by itself, so three strikes (about 90 s) would have restarted a
  server that recovered. Accepted under the owner's 3-strike rule: a restart
  costs seconds, a missed stall cost 40 minutes. Every strike is logged so a
  GC correlation stays visible. Strikes and deadline are environment-tunable.
- A probe fault must not kill a healthy server. Only a timeout (exit 124) or
  a refused connection on an active, past-grace unit is a strike. "Access
  denied", a missing client binary, an unreadable credential, or an
  unrecognised error is logged as `probe-error`, is not a strike, and makes
  the probe unit fail so the hardening check shows it.
- SIGABRT storm: cooldown of at least 10 minutes per target after a signal,
  kept in `/run` with `RuntimeDirectoryPreserve`. At most 6 signals an hour,
  inside the Dolt unit's start limit (10 in 600 s). The evidence is journaled
  before the signal.
- Wrong pid: take `MainPID` from systemd and check `/proc/<pid>/comm` is
  `dolt` before signalling, and signal with `systemctl kill --kill-whom=main`.
- The dump may not reach the journal (the 10:46 SIGQUIT dump stalled after
  three lines). `/proc` status, wchan, io and per-thread wchan are journaled
  first.
- A core dump of a 3.5 GB process delays the restart. SIGABRT's default
  action is core; set `GOTRACEBACK=all` (prints all goroutines and exits 2,
  no core) and `LimitCORE=0` on both Dolt units.
- A process stuck in D state never takes the signal: if the pid is unchanged
  20 s after the signal, `systemctl restart` (stop is bounded by
  `TimeoutStopSec=20s`).
- The restart loses in-flight work. A committed SQL transaction is durable
  (the platform's own comment: SQL COMMIT is the durability, the snapshot
  commit is bookkeeping); in-flight transactions roll back and clients retry.
  The 10-15 SIGKILL recovery was clean, and SIGABRT is the same class of exit.
- Changing the Dolt units (`TimeoutStopSec`, `LimitCORE`, `GOTRACEBACK`)
  makes the next deploy switch restart both Dolt servers. That is a planned
  restart inside a deploy, the same as any change to these units, noted for
  the operator.
- Overlapping runs: `flock` on a lock file in the runtime directory; the probe
  bounds its own wall time (2 targets x 5 s plus kill grace).
- Credentials: read from `$CREDENTIALS_DIRECTORY` (LoadCredential of the
  existing `dolt-app-password`, the same source as the `dolt-users` unit),
  exported only as `DOLT_CLI_PASSWORD` to the one child, never in `argv`,
  never logged, no `set -x`, and stderr is classified, not echoed.
- The probe cannot be tested against real Dolt here. It gets a dry-run mode
  (log the action, send no signal) and a fake-command test harness covering
  the strike, reset, cooldown, skip-while-activating and credential-error
  cases, written before the script.

### 7.3 What was built, and what only a staging deploy can prove

Built (branch fix/dolt-guardrails): checkpointd HOME and preflight;
`OpenStore(ctx, ...)` and `OpenStoreWithoutBootstrap`, 10 minute
`TimeoutStartSec`; vmctl `retryWithBackoff`; `scripts/dolt-liveness-probe.sh`
with its timer and service in `nix/node-b.nix`, `TimeoutStopSec=20s`,
`LimitCORE=0` and `GOTRACEBACK=all` on both Dolt units, and
`check_dolt_probe` in `scripts/verify-host-hardening.py`.

Tests that ran locally (no real Dolt, no credentials): the checkpointd preflight
(Go), the three context-deadline tests against a TCP listener that never
answers (Go), the vmctl retry tests (Go), the probe harness
`scripts/test-dolt-liveness-probe.sh` (30 cases: strikes, reset, cooldown,
skip while activating, grace, credential error, dry run, wrong pid, restart
escalation, canary secret never echoed; a mutation that removes the cooldown
fails 3 of them), and `scripts/test_verify_dolt_probe.py`.

Only staging (Node B) can prove: that Dolt is satisfied by a `HOME` under
`/var/lib/go-choir/checkpointd` and the projection backlog drains; that the
probe's sandbox (`commonServiceHardening` plus `CAP_SYS_PTRACE`) lets
`systemctl show/kill` and the `/proc` reads work and that `dolt sql` as `choir`
with the credential succeeds; that SIGABRT with `GOTRACEBACK=all` prints a
goroutine dump into the Dolt unit's journal and Dolt restarts within the start
limit; that the deploy switch restarts the Dolt units cleanly when their unit
files change; and the vmctl wait-and-recover path against a real stalled Dolt.
Suggested staging proof: after the deploy, `verify-host-hardening.py` shows the
`dolt-probe` section passing; a dry run of the probe by hand
(`PROBE_DRY_RUN=1`) logs two `unit=... ` lines and no strikes; do not stall the
production Dolt to test the ABRT path.
