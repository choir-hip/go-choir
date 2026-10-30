# Store A (the platform Dolt) keeps every commit; a weekly squash with a backup first (2026-10-20)

Status: built, not deployed, timer disabled. Class: red (Store A is the platform's canonical
event and control database, the vmctl route database and the checkpoint
source; a squash cannot be undone, and the store is stopped for it).
Owner rulings: [notes 2026-10-14](../reports/owner-notes-2026-10-14.md) §23
("Store A history is squashed weekly with a backup first": the store keeps
every commit, 92k commits and 24 GB of old history, growing about 1.6 GB a
day, and nothing reads it; "after the Dolt liveness probe is live, back up
the Dolt data directory, then in a quiet window do one squash and a full GC.
After that, squash every week. Current rows and the event-tape rows are kept;
only Dolt's own version history of them is dropped. The squash can't be
undone, so the backup comes first") and §27 item 7 ("Store A's weekly squash
is built as a small tested command, on the paper compaction's method (squash,
identical root hash, memory cap)").

## Problem

Store A commits its working set every ~45 s (`internal/doltbatch`,
`CALL DOLT_COMMIT('-Am', ...)`), so `dolt_log` grows by ~1,900 commits a day.
No GC collects reachable commits, so history is retained forever: the
[clustering assessment](dolt-clustering-assessment-2026-10-15.md) measured
92,290 commits and 24 G of `noms/oldgen` (260 `.darc` archives) with a live
set far smaller, +1.6 G/day. The 2026-08-26 incident
([evidence](../evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md))
was the same mechanism at 6.9 M commits and 218 G; its squash was done by hand
(dump, fresh init, import). Nothing is scheduled to bound it: the daily
history audit only fails at 1 M commits or 50 GiB, a signal with no consumer.

The paper's guest store had the same disease and was fixed on 2026-10-19
([paper-disk-full-again](paper-disk-full-again-history-growth-2026-10-19.md)):
13,700 commits to 2, 51.95 GB to 8.06 GB, root hash identical, by a squash
plus a bounded full GC. That method is proven; Store A needs it as a tool and
a schedule.

## Evidence that history has no reader (read from code, not production)

- No Go, shell, nix or python file outside `internal/projectionbase/compact.go`
  and `internal/store` (guest stores) uses `dolt_log`, `dolt_diff`,
  `dolt_history_*`, `AS OF`, `DOLT_BRANCH`, `DOLT_TAG`, `DOLT_CHECKOUT`,
  `DOLT_MERGE`, `DOLT_REMOTE`, `DOLT_PUSH`, `DOLT_BACKUP` or `DOLT_STASH`
  (grep over `internal/ cmd/ scripts/ nix/`; the hits are the history audit's
  `COUNT(*) FROM dolt_log`, the Store B `dolt_ignore` projection tables in
  `internal/platform/capture_index.go`, and the ograph bench scripts).
  So Store A has one branch, no tags, no remotes, no stashes, by construction.
  The command still checks, because an operator could have created one by hand.
- `dolt_ignore` is used, but only for Store B's projection tables. Under the
  pre-split topology (empty `corpusDSN`) those tables would sit in Store A;
  ignored tables are in the WORKING root but never in a commit, so HEAD and
  WORKING roots differ and the root check below refuses (verified locally with
  the dolt CLI: `dolt_hashof_db('HEAD')` differs from `'WORKING'` with an
  ignored table present).
- The 2026-10-15 preflight
  ([dolt-upgrade-preflight](../reports/dolt-upgrade-preflight-2026-10-15.md))
  already took a reflink copy of the platform data dir (25 GB), ran
  `dolt fsck` on a scratch copy with 2.4.2 (no problems, 1,348 s, peak RSS
  ~4 GB), and showed 2.1.9 reads a store 2.4.2 opened. It says that backup
  "also serves as the pre-squash backup" for this ruling; it is a week old and
  not a substitute for the backup this tool takes.

## Method (the paper's, generalized)

1. Open the store with the embedded Dolt driver (`projectionbase`), refuse
   unless exactly one branch and no tags, remotes or stashes.
2. Record `dolt_hashof_db('WORKING')`.
3. `CALL DOLT_RESET('--soft', <root commit>)`, then `CALL DOLT_COMMIT('-A',
   '--allow-empty', ...)`: one commit holding the working set.
4. Check before the GC that the new HEAD root equals the recorded WORKING
   root and that the log is at most two commits (cheap, and it stops a GC that
   would free nothing).
5. `CALL DOLT_GC('--full','--archive-level','0','--incremental-file-size',
   '268435456')`, with temp files staged inside the store and a Go memory
   limit (`GOMEMLIMIT`-equivalent); the unit adds `MemoryMax` and
   `MemorySwapMax=0`. This bound took the paper's full GC from "13 GB plus
   15 GB of swap, never finished" to 6.1 GB and 2 min 19 s.
6. Reopen; refuse unless HEAD root and WORKING root both equal the recorded
   WORKING root. Prolly trees are history independent, so equal root hashes
   mean the same schemas and rows.

Differences from the guest tool: the root commit is found by topology
(`dolt_commit_ancestors` with a NULL parent, exactly one) instead of "oldest
by date", so a clock step cannot leave older history reachable; and the
ref/log checks above are stricter. Both live in one shared function in
`internal/projectionbase` that the guest compaction also calls, so the proven
path has one implementation.

## Decision: offline with the service stopped, not online through SQL

Online (`CALL DOLT_GC` and the squash through the running sql-server) is
rejected:

- **The invariant needs a quiet store.** The root-hash check compares the
  WORKING root before with the roots after. Writers (corpusd, the debounced
  committer every 45 s, vmctl route updates) change WORKING between the two
  reads, so a changed hash would be ambiguous: a bug or a write. Making it
  unambiguous requires stopping the writers, which is the same outage.
- **Memory.** The unit has `MemoryHigh=3G`, `MemorySwapMax=0`,
  `OOMScoreAdjust=-500` (the canonical log is never swapped or killed). The
  GC needs ~6 GB; running it inside the server either throttles the server
  or kills it. 2026-08-26 killed Store A with one large scan (27 GB peak).
- **Online GC invalidates open sessions** and, on 2026-10-15, an online
  server stalled for 40 minutes after a conjoin; a long online `--full` GC is
  the most likely thing to wedge it.
- **The backup must be consistent.** A reflink copy of a stopped store is
  crash-consistent without Dolt's help; a copy of a live one is not.

Offline: the Dolt unit and what it `Requires=` are stopped, a reflink copy
of the data dir is taken, the command runs against the closed store under
the cap, and the unit starts again. The window is a short planned platform
maintenance window (estimate below). The embedded driver is the same module
version as the guest tool; the server binary is `pkgs.dolt` (2.1.9, 2.4.2 on
the dolt lane), so version skew is a named risk (FM9).

## Failure modes, written before the code

| # | Failure | Guard | Pinned by |
|---|---|---|---|
| FM1 | Root hash changes (data lost or altered) | refuse unless HEAD and WORKING after equal WORKING before; script restores the backup | Go test: root identical; script test: refusal restores |
| FM2 | A branch, tag, remote or stash keeps history reachable | refuse before touching anything (exit 3); original byte-identical | Go tests for each ref class |
| FM3 | Ignored tables or other state HEAD cannot hold (HEAD root != WORKING root) | refuse after the squash commit and before the GC | Go test with a `dolt_ignore` table |
| FM4 | History not actually dropped (more than one root, merge shape, clock skew) | root commit by topology, exactly one; refuse before GC if log > 2 | Go test |
| FM5 | Unbounded GC memory | bounded GC flags, memory limit, unit `MemoryMax`, `MemorySwapMax=0` | flags are the proven ones; unit asserts in nix |
| FM6 | GC temp output on another filesystem (host `/tmp`) | tempfile provider at `<repo>/.dolt/temptf`, removed after | Go test checks the dir is empty after |
| FM7 | The server or a dependent opens the store during the squash | stop `RequiredBy` units and their timers first, verify the unit is inactive, `flock`; whether the embedded open itself refuses while a server holds the store is probed in the build and recorded below | script test; local lock probe |
| FM8 | Backup missing, not a reflink, or no space | `cp --reflink=always`, abort before the command and start services; free-space preflight (>= 50% of store size and >= 10 GiB) | script test |
| FM9 | Embedded module and server binary differ in version | plain table files (archive level 0) readable by all; supervised first run with `dolt fsck` afterwards | residual risk, first-run procedure |
| FM10 | The unit is OOM-killed or the host reboots mid-run, services stay down | persisted state file; `ExecStopPost` runs the same `--finish` which restores the backup if a squash was in flight, then starts services | script test: finish after a kill |
| FM11 | A failed run's evidence is destroyed by the next run | refuse to run while a `*.squash-failed-*` directory exists | script test |
| FM12 | Backups pile up (CoW extents pin old blocks) | keep the newest backup only, deleted after the next run's backup succeeds | script test |
| FM13 | A schedule enabled before the first run was watched | timer ships disabled (flag off, no `wantedBy`); first run by hand | nix |
| FM14 | The command mutates a store another run is using | `flock` on a lock file in the script | script test |

## Open hypothesis, to be checked on the first supervised run

vmctl reads Store A (`VMCTL_ROUTE_DSN`, the ComputerVersion route database)
and only `Wants=` the Dolt unit, so it is not stopped with it. At startup it
now retries (`cmd/vmctl/routedb_retry.go`); what a running vmctl does with a
route database that is down for ~15 minutes is not established by trace. This
is a hypothesis, not a finding. The first run is supervised for exactly this:
watch vmctl and the paper through the window (procedure in the commit that
adds the unit). The script never restarts vmctl (a restart detaches guests).

## Ceremony (red)

- **Mutation class:** red. Protected surfaces: Store A canonical writes
  (event append receipts and heads), vmctl route database, checkpoint source,
  VM lifecycle adjacency (dependents stop for the window).
- **Conjecture delta:** Store A history can be bounded weekly by the paper's
  method inside a window short enough that dependents tolerate it.
- **Admissible evidence:** equal root hash before and after (recorded in the
  command's JSON report and the journal); `dolt_log` at 1 or 2 commits;
  row counts spot-checked by the operator on first run; staging `/health`
  and the paper's runs continue after the window; a clean `dolt fsck`.
- **Rollback:** the reflink backup of the data dir, taken before the command
  and kept until the next run. Automatic on any failure after the first
  mutation (script restores, then starts services); manual after a bad
  outcome the checks missed: stop the Dolt unit, move the data dir aside,
  `cp -a --reflink=always` the backup back, start. The squash is irreversible
  without it; the backup is the only undo.
- **Heresy delta:** discovered: no defined behavior of a running vmctl when
  Store A is down for a planned window (hypothesis above); the history audit
  is a signal with no consumer (already named). Introduced: none while the
  timer is disabled. Repaired: none yet; the schedule repairs unbounded Store A
  history when enabled.
- **Standing questions:** provenance (owner §23, §27.7); no deletion of
  citers (nothing deleted); single authority (the root hash, not narration);
  artifact-verified success (JSON report with both roots); fate-sharing
  (named above); restart durability (state file plus `ExecStopPost`);
  no-SSH operability (everything is a unit and a script; the first run is by
  hand on Node B); registry hygiene (one backup kept, failed runs kept for
  diagnosis).

## Build plan

1. This document (green).
2. `internal/projectionbase` shared squash function; the guest compaction calls
   it; tests on a local temp Dolt repo; `cmd/platform-dolt-squash` (orange/red).
3. `scripts/platform-dolt-squash.sh` with a fake-systemctl behavioural test,
   and the nix oneshot, timer (disabled) and flake package (red).

## Design details added after the build

What was built (nothing is deployed or enabled):

- `internal/projectionbase/squash.go`: `SquashAndGC`, the shared method. The
  guest compaction (`CompactWorkspaceWith`, used by `projection-compact` and
  vmctl's sweep) now calls it. Two behaviour changes for the guest path, both
  stricter: the root commit is found by topology, and the squash is refused
  before the GC when the squashed HEAD root differs from the recorded root.
- `cmd/platform-dolt-squash`: the command. Exit 0 squashed, 3 refused before
  any change, 1 anything else. It prints one JSON report line (both roots,
  commit counts, seconds).
- `scripts/platform-dolt-squash.sh` (+ `scripts/test-platform-dolt-squash.sh`):
  stop, backup, run, restore on failure, start, query check, prune.
- `nix/node-b.nix`: `go-choir-platform-dolt-squash.service` (oneshot,
  `MemoryMax=10G`, `MemorySwapMax=0`, `ExecStopPost=... --finish`) and the
  `.timer` (`Sun 06:00 UTC`, `Persistent=false`), **disabled**:
  `platformDoltSquashTimerEnabled = false` leaves `wantedBy` empty
  (checked with `nix eval`: `wantedBy = []`). `flake.nix` packages
  `platformDoltSquash`; `.github/scripts/deploy-impact-classify` treats the
  command and script as host-OS-only.

Findings during the build (local probes, dolt CLI 2.1.9 and the pinned
embedded module):

- **A sql-server makes an embedded open read-only.** With `dolt sql-server`
  running on the repo, the embedded driver opens it and reads, but the first
  write fails with "cannot update manifest: database is read only". So a
  squash beside a live server fails closed even without the script. The
  server also writes `.dolt/sql-server.info` and removes it on a clean stop;
  `SquashAndGC` refuses on its presence (exit 3) with a message saying so.
  FM7 is therefore two layers (script stops units; Dolt refuses writes).
  Caveat: a server that died uncleanly leaves the file; the refusal then
  needs `rm` by hand after confirming no server runs.
- **The Node B layout works with the embedded driver.** `file://<parent>?
  database=platform` over a parent holding the sql-server's global
  `.dolt/config_global.json` and the repo in `<parent>/platform`, created by
  the CLI's `dolt init`, as `platform-dolt-init` does.
- **Refs and ignore:** `dolt_tags`, `dolt_remotes`, `dolt_stashes` exist and are
  checked. With a `dolt_ignore`d table `dolt_hashof_db('HEAD')` differs from
  `'WORKING'` (checked with the CLI), which the pre-GC root check catches.
- The squash's `-A` commit is the only write before the GC; everything else is
  a read.

### Scope and what the checks do not cover

The root hash proves schemas and rows are identical. It does not prove: that
`dolt gc --full` produced a store the server binary (2.1.9 or 2.4.2) reads
(archive level 0 writes plain table files, which every version reads; the
first run ends with the real server answering a query, then `dolt fsck`); that
Dolt's server-only state is intact (users and grants live in
`.doltcfg/privileges.db` beside the repo, untouched by the squash; the
post-start query check logs in as the app user, so a lost grant would show).

### Expected downtime and space

- Estimate, by analogy (unmeasured on Store A): the paper's 52 GB, 13.7k-commit
  store took 11 minutes end to end (squash, GC, verify) at a 6 to 10 GB peak.
  Store A is smaller in bytes (about 25 GB at 10-15, +1.6 GB/day, so
  roughly 30 to 35 GB now) but has about 100k commits, which the soft reset
  and commit handle in seconds. Expect **10 to 25 minutes of Store A down**:
  stop (<= 20 s), reflink backup (seconds), squash and GC (5 to 20 min), start
  and query check (< 1 min). Abort and investigate past 45 minutes; the hard
  bound is `TimeoutStartSec=2h`.
- What stops with it: Dolt, `go-choir-corpusd` (Requires=), the checkpointd and
  history-audit oneshots and their timers. corpusd down means the platform
  API behind it, source ingestion, publishing and the paper's corpusd calls
  pause; the front page keeps serving what is stored. vmctl and the guests
  keep running (hypothesis above).
- Space: the backup is a reflink, so free space is needed only for the GC's new
  files (about the live size, a few GB to 10 GB) while the old extents stay
  pinned by the backup until the next run replaces it. Preflight refuses below
  max(10 GiB, half the store).

### Supervised first run on Node B (operator commands)

The timer stays off until this has been watched once end to end.

1. Preconditions. CI has deployed this commit (`systemctl cat
   go-choir-platform-dolt-squash.service` shows the unit); no deploy is in
   flight; the paper is not mid-publish (check `/health` and recent runs);
   `df -h /var/lib/go-choir` shows well over 30 GiB free; `btrfs filesystem
   usage /var/lib/go-choir` shows unallocated space (a CoW GC allocates);
   `ls -d /var/lib/go-choir/platform-dolt.squash-failed-*` is empty; the
   liveness probe is running (`systemctl list-timers go-choir-dolt-liveness-probe`).
2. Baseline, read-only (record the output):
   `cd /var/lib/go-choir/platform-dolt/platform && HOME=/var/lib/go-choir/platform-dolt
   dolt sql -q "select count(*) from dolt_log; select count(*) from
   dolt_branches; select count(*) from dolt_tags; select count(*) from
   dolt_remotes; select count(*) from dolt_stashes"`, and `du -sh
   /var/lib/go-choir/platform-dolt`. Expect 1 branch and no tags, remotes or
   stashes; if not, the run will refuse (exit 3) and nothing changes.
3. In a second terminal start the watchers: `journalctl -fu
   go-choir-platform-dolt-squash`, `journalctl -fu go-choir-vmctl`, and
   `watch -n5 'systemctl show -p MemoryCurrent -p MemoryPeak
   go-choir-platform-dolt-squash.service; curl -s -m3 http://127.0.0.1:8086/health'`.
4. Run it, in the foreground: `sudo systemctl start
   go-choir-platform-dolt-squash.service` (blocks until the oneshot ends).
5. Abort, if needed: `sudo systemctl stop go-choir-platform-dolt-squash.service`.
   SIGTERM cancels the command; the script's exit trap and `ExecStopPost`
   restore the backup (if the squash had started), start Dolt, its dependents
   and the timers, and clear the state.
6. Verify success: unit result `success`; `cat
   /var/lib/go-choir/platform-dolt-squash-backups/last-run.json` shows
   `"squashed":true`, `root_before` equal to `root_after`, `commits_after`
   1 or 2; the journal ends "done; newest backup kept at ..."; `systemctl
   is-active go-choir-platform-dolt go-choir-corpusd`; the baseline query again
   (commits 1 or 2, same branch count); `du -sh` smaller; `curl -s
   https://choir.news/health`; the liveness probe journal clean; vmctl did not
   restart (`systemctl show -p NRestarts go-choir-vmctl`) and the paper
   publishes within 30 minutes. Record vmctl's journal lines during the
   window: they settle the open hypothesis.
7. Soak a day, then (owner call) delete `/var/lib/go-choir/dolt-backup-20261015-pre-2.4.2`
   and, after the next run, nothing else needs deleting: the run keeps only the
   newest backup. Run `dolt fsck` on a reflink copy of the new store under a
   `systemd-run -p MemoryMax=8G` cap (1,348 s last time) to settle the
   version-skew risk.
8. Enable the weekly timer: set `platformDoltSquashTimerEnabled = true` in
   `nix/node-b.nix`, push, deploy.

### Rollback

- During the run: step 5 above, or automatically on any failure after the
  squash began (the failed store is kept as
  `/var/lib/go-choir/platform-dolt.squash-failed-<ts>`; the next run refuses
  until it is removed by hand, so evidence is never overwritten).
- After a run that looked fine but is wrong: `systemctl stop
  go-choir-corpusd go-choir-platform-dolt`; `mv /var/lib/go-choir/platform-dolt
  /var/lib/go-choir/platform-dolt.bad`; `cp -a --reflink=always
  /var/lib/go-choir/platform-dolt-squash-backups/platform-dolt.<ts>
  /var/lib/go-choir/platform-dolt`; `systemctl start go-choir-platform-dolt
  go-choir-corpusd`. The backup is the pre-squash store as of the stop.
  Nothing writes Store A while it is stopped, so restoring within the same
  window loses nothing; once services have run on the squashed store, a
  restore drops every write made since.
- Code rollback: revert the commits; the guest compaction goes back to its
  previous implementation. The timer is disabled, so nothing runs by itself.

### Residual risks

- Embedded module and server binary versions differ (FM9); archive level 0 and
  a post-start query plus `dolt fsck` are the mitigations, not a proof.
- vmctl's behaviour with Store A down for 10 to 25 minutes is unverified.
- A stale `.dolt/sql-server.info` after an unclean Dolt death blocks the run
  (fail closed).
- The first GC on a store with years of archives is unmeasured; if memory
  exceeds the cap the run fails, the backup is restored, and the store is
  as before (cost: one outage).
- Power loss or a host reboot mid-run: Dolt may start at boot against a
  half-squashed store (Dolt's manifest swaps are atomic, so the store is
  consistent but unverified); the state file remains and the next run refuses
  until `platform-dolt-squash.sh --finish` has restored or cleared it.
- Backup pins pre-squash blocks until the next run: the host's btrfs
  headroom (see the 10-19 host-space notes) should be watched after the run.
- Deploying this commit restarts the services the classifier maps from
  `internal/projectionbase` (checkpointd, corpusd, gateway, proxy,
  autoputer, vmctl) and, with `nix/node-b.nix`, activates the host OS: an
  ordinary deploy, subject to the Landing Loop and the busy-computer rules.

## 2026-10-28 11:52 UTC: first supervised run, OOM-killed, restored (owner-approved GC)

Owner approved a GC of platform-dolt (2026-10-28). Preconditions held (90G
free, 16 GiB unallocated, no failed dirs, liveness timer live, no deploy in
flight; staging on f7e215e54). Baseline: 120,335 commits, 1 branch, 0 tags,
remotes, stashes; 50.89 GiB.

Run 11:52:07 -> 11:57:12: preflight ok (store 53,370,912 KiB), timers and
corpusd stopped, reflink backup `platform-dolt-squash-backups/platform-dolt.20261028T115258Z`,
then the `platform-dolt-squash` binary was **OOM-killed at MemoryMax 10G**
(peak 10,737,422,336; 16.4G read, 12.3G written, 2m20s CPU). `--finish`
restored the backup; platform-dolt and corpusd active; vmctl NRestarts 0
-> 0 (it rode out ~5 min of Store A down). The failed store is kept at
`/var/lib/go-choir/platform-dolt.squash-failed-20261028T115707Z`.

Findings:
1. The squash's memory exceeds 10G on this store (120k commits, ~51 GB);
   the 6-10 GB estimate came from the paper's 13.7k-commit store and does
   not transfer. Which phase holds it (soft reset vs full GC) is not yet
   known; the in-binary 5 GiB Go limit does not bound it (cgo or mmap).
2. Unit bug: `SQL_CHECK_CMD=... --sql 13306 platform` is unquoted in the
   nix Environment list, so systemd drops "--sql", "13306", "platform"
   ("Invalid environment assignment, ignoring"). The post-squash query
   check would run the probe with no arguments. Fix: quote the value.

Settled hypothesis: a running vmctl keeps its guests through ~5 minutes of
Store A down without a restart.

Next (not done): find the memory holder on a reflink copy of the store
under a high cap (systemd-run, not on the live unit), then size MemoryMax
or bound the phase; fix the quoting; rerun.
