# platform-dolt's oldgen: the rate, its cause, and the squash window (2026-10-22)

Date: 2026-10-22, 17:00 UTC. Lane: FP0 platform Dolt oldgen
(`lane/fp0-platform-dolt-oldgen`). Status: open; problem record first (381b7ee5f), then the switch
(`90ea1e378`, inert) and the runbook. Companion to FP0's record
[platform-dolt-oldgen-grows-every-auto-gc-2026-10-22.md](platform-dolt-oldgen-grows-every-auto-gc-2026-10-22.md)
(on `lane/fp0-disk-pressure`), which reads the GC mechanism in Dolt's code;
this record takes the next questions: why the rate, what the supervised squash
costs, and what bounds it for good. Mutation class of this record: green. Of
the fixes it names: red (Store A writes, the platform Dolt unit).

## Evidence (FP0, read-only on Node B, 2026-10-22)

- btrfs root 475 GiB. Free 139 GiB at 16:50, after an owner-approved early
  compaction of the paper (store 33.0 to 14.5 GB) and deleting a week-old
  Dolt backup (it freed only 7.2 GiB: its extents are shared).
- The deploy disk preflight floor is 90 GiB (`DEPLOY_MIN_FREE_KIB` in
  `scripts/node-b-deploy-disk-preflight`); deploy R was refused by it at
  14:28.
- Holders: VM state about 150 GiB (the paper 51 GiB, regrowing about 85 to
  140 MiB a minute until its 125.7 GiB cap or the 04:00 compaction; the
  candidate fleet dir 40 GiB, of which 32.4 GiB are two quarantined data
  images from 2026-10-07; the owner computer 15 GiB; test computers 9.6, 12,
  8.6, 8.7 and 1.4 GiB; another user 3.1 GiB); `/nix/store` 65 GiB
  (min-free 80 GiB, max-free 100 GiB, auto-optimise-store false,
  keep-derivations true; 51 auto gcroots including 7 prebuild-results and
  builder releases under `/var/lib/go-choir/builder/releases/*/out`; 4 system
  generations; releases about 3.9 GiB each, 11 in the store with little
  sharing); **platform-dolt 36 GiB, almost all `noms/oldgen/*.darc`**;
  corpus-dolt 6.3 GB; a 16 GiB swap file (its removal waits on tonight's
  02:37 backup proof); builder 5.8 GB; offhost-backup 4 GB.
- btrfs metadata DUP at 84% of 18.6 GiB; 38 GiB unallocated at 14:55.
- platform-dolt: auto GC about every 30 minutes; oldgen files of 25.7 GB
  (11:12) and 6.2 GB (14:54). Nothing collects oldgen.
- `/var/lib/go-choir/platform-dolt/history-audit-last-count` exists: the
  daily history audit's last `dolt_log` count (not read yet).

## 1. Why oldgen grows, read from code

### Confirmed in code

- **Only commits feed oldgen.** Dolt's Default GC (FP0's record, read in
  `doltdb.go` and `value_store.go`) copies into oldgen the chunks reachable
  from branch refs, and keeps in newgen only what the working sets need.
  A chunk written by SQL but superseded before the next `DOLT_COMMIT` never
  reaches oldgen. So oldgen growth is the sum, over commits, of the chunks
  each commit references that no earlier commit did: commits per hour times
  the prolly-tree leaves and internal nodes each commit changes.
- **Store A commits often, and some writes force a commit at once.**
  `internal/doltbatch` snapshots the whole working set with
  `CALL DOLT_COMMIT('-Am', …)` 45 s after the first unsnapshotted mutation
  (`DefaultDebounce`; `DOLT_SNAPSHOT_DEBOUNCE` is read but not set in
  `nix/node-b.nix`). `commitBoundary` commits synchronously on: every
  projection job enqueue, lease, finish, base pin, maintenance key use and
  verified-checkpoint publish (`internal/platform/projection_jobs.go:199,
  267, 301, 312, 345, 391`); every checkpoint and route projection
  certificate publish (`checkpoints.go:208, 371`); every file root and
  replay watermark record (`file_cas.go:47, 107`); tape tombstones and
  projection base pin releases. `-Am` commits every table, so one boundary
  commit also snapshots whatever vmctl and checkpointd wrote.
- **Nothing reads Store A's commit graph.** No code outside the guest
  stores (`internal/store`, `internal/projectionbase`, the
  `computerversion.DoltStateExtractor` used only on guest workspaces) reads
  `dolt_log`, `dolt_diff`, `dolt_history_*`, `AS OF` or `HASHOF('HEAD')`
  (grep over `internal/` and `cmd/`; the squash record's wider grep agrees).
  The history audit's `COUNT(*) FROM dolt_log` and the off-host backup's
  `DOLT_BACKUP('sync-url', …)` copy it; neither needs it. The doltbatch
  package comment already calls the snapshot "bookkeeping": durability is
  the SQL `COMMIT` (the 2026-09-19 kill -9 drill on 2.1.9).
- **Conjoin makes one large new file out of old history.** In Dolt's
  `store/nbs` (module `6bea93f2f665`, Dolt 2.2.0), when a generation holds
  more than 256 table files (`defaultMaxTables`) the store conjoins: it
  sorts the files by chunk count, copies the smallest ones (until the copy
  would be smaller than the next file and fewer than 256 remain) byte for
  byte into one new file, swaps the manifest, and deletes the inputs
  (`conjoiner.go` `chooseConjoinees`; `file_table_persister.go`
  `ConjoinAll` cleanup). The new file's mtime is the conjoin's time and its
  size is the sum of its inputs. With one oldgen file per auto GC every
  ~30 min, a conjoin falls due about every five days.

### The rate: probably not 25 to 35 times 10-15's (hypothesis, one reading settles it)

- 10-15 measured oldgen at 24.38 G, +1.6 G a day
  ([clustering assessment](dolt-clustering-assessment-2026-10-15.md) §1.3).
  Seven and a quarter days at that rate gives **36.0 G on 10-22**, which is
  what FP0 measured (36 GiB). So the total is consistent with the 10-15 rate.
- The "1.5 to 2.5 GiB an hour" came from the two files' sizes and times. If
  the 25.7 GB file at 11:12 is a conjoin (its size is about the whole
  pre-existing oldgen less the 6 to 10 GB of other files), it is old data
  re-packed, not 25.7 GB written in one GC. **Hypothesis H1 (conjoin
  artifact, rate unchanged at about 1.6 to 2 GB a day)** fits the total;
  **H2 (a real rate rise)** fits the two file sizes read as single-GC
  outputs but then the total would be far above 36 GiB unless much older
  history had been dropped, which nothing does.
- The 6.2 GB file at 14:54 is not explained by H1 alone (a second conjoin
  3.7 h after the first needs 256 new files). Candidates: a partial conjoin
  at 11:12 that left more than 256 files; or one GC promoting a large
  commit (a table rewrite, for example a column add or backfill on a large
  Store A table, or many boundary commits in a burst). Not read from any
  trace.
- What is read in code about **new writers since 10-15**: see §1b (writer
  map); no new writer raises the steady commit cadence by an order of
  magnitude.

The settling reading is read-only and needs no SQL (FP0):

```sh
# 1. the daily series since 10-14: commits and oldgen KiB per audit run
journalctl -u go-choir-platform-dolt-history-audit --since 2026-10-14 --no-pager \
  | grep -E 'history audit'
# 2. conjoins and GCs today, with sizes on disk
journalctl -u go-choir-platform-dolt --since '2026-10-22 00:00' --no-pager \
  | grep -E 'conjoin|auto GC|auto_gc' | tail -n 80
ls -l --time-style=full-iso /var/lib/go-choir/platform-dolt/platform/.dolt/noms/oldgen \
  | sort -k6,7 | tail -n 20
ls /var/lib/go-choir/platform-dolt/platform/.dolt/noms/oldgen | wc -l
# 3. the oldgen slope over one hour
du -sk /var/lib/go-choir/platform-dolt/platform/.dolt/noms/oldgen; sleep 3600; \
du -sk /var/lib/go-choir/platform-dolt/platform/.dolt/noms/oldgen
cat /var/lib/go-choir/platform-dolt/history-audit-last-count
```

Reading: a `beginning conjoin … generation=old` line at about 11:12 and
daily oldgen deltas near 1.6 to 2.5 GB confirm H1; daily deltas of tens of
GB confirm H2, and then the commit delta per day says whether the cadence
(commits) or the size per commit rose.

### 1b. Who writes Store A (writer map, read in code)

The split is live (since 2026-09-19): `og_*`, `proj_capture_*`,
`publication_*`, `platform_texture_*` and `proposal_delivery_records` go
through `Store.corpus()` to Store B (`store.go:332`) when
`/var/lib/go-choir/corpus-dsn.env` sets `CORPUSD_CORPUS_DOLT_DSN` (FP0 can
confirm it is non-empty with `test -s`; if it were empty every corpus table
would write Store A). Store A's writers, highest churn first:

| Table | Writer | Cadence | Shape |
|---|---|---|---|
| `computer_event_append_receipts` | `computer_events.go:284` via corpusd `/internal/computers/events/append`; `markDirty` at `:304` | every event every computer appends | append; two JSON columns (the signed head receipt, pin digests) and about ten 64-char hashes; PK `(computer_id, idempotency_key)` (random within a computer) plus unique `(computer_id, sequence)`, unique `(computer_id, event_digest)` and `(computer_id, event_kind, sequence)` |
| `computer_event_heads` | `computer_events.go:267/270`, same transaction | every event | the computer's one row updated in place |
| `projection_worker_status` | checkpointd `worker.go:72, 103, 126` | heartbeat every 45 s while a job runs, no `Mark` | one row in place; rides the next commit |
| `computer_projection_jobs` | `projection_jobs.go:192, 261, 271, 290` | each job: enqueue, claim, lease renewal every 45 s, finish | in place; enqueue, claim, finish each `commitBoundary` |
| `computer_key_escrow_transparency` | `projection_jobs.go:337` (key-use audit) | each checkpoint job, `commitBoundary` | append, `payload_json` |
| `computer_replay_watermarks`, `computer_projection_bases`, `computer_file_roots`, `computer_checkpoints`, `computer_route_projection_certificates`, `computer_projection_base_pins` | `projection_jobs.go:308, 378-391`, `file_cas.go:44-107`, `checkpoints.go:201-371` | per checkpoint publish and per guest watermark or file-root call; each `commitBoundary` | small rows, receipts as JSON |
| `computer_version_route_*`, `computer_version_code_closures`, `computer_version_artifact_programs` | vmctl, `internal/routeledger/sql_ledger.go`, `internal/computerversion/input_resolver.go` | route transitions and new version inputs | vmctl never commits; its rows ride corpusd's next commit |
| keys, escrow, lifecycle, owner acts, consensus, self-development modes, tape tombstones | `control_keys.go`, `key_escrow.go`, `lifecycle_control.go`, `owner_acts.go`, `standing_consensus.go`, `self_development_modes.go`, `tape_tombstone.go` | per operation, rare | small |

Event payloads, pins and tape segments are files under the artifact root,
not Dolt rows. **No new Store A writer, no faster cadence and no
whole-table rewrite landed in Go code since 2026-10-15** (`git log` over
`internal/platform`, `internal/doltbatch`, `cmd/checkpointd`,
`cmd/corpusd`, `internal/routeledger`): the only new Store A table is
`computer_tape_tombstones` (`685dffc68`, rare), and the tape segment writer
(`c528c8ad3`, `d9649b191`) writes files. So if H2 holds, its cause is
volume (more events: the paper publishing around the clock with 66
schedules, more test computers), not a code change; the commit delta in
the audit series says which.

**Cause, confirmed in code:** Store A's oldgen is the accumulated history of
a snapshot commit (`DOLT_COMMIT -Am`) that runs at least every 45 s while
events arrive and synchronously at every checkpoint boundary, each commit
retaining the receipt-tree leaves, the per-event head row and the
heartbeat rows it changed. Nothing reads that history, and no GC can drop
it while a commit reaches it.

**What this means for a fix, read in Dolt's code:** auto GC is driven by a
commit hook that also runs on working-set updates
(`sqle/auto_gc.go` `ExecuteForWorkingSets() == true`;
`doltdb/hooksdatabase.go:112`), so it keeps running if Store A stops making
Dolt commits. With no new commits, a Default GC adds nothing to oldgen and
keeps in newgen only the chunks the working set needs that HEAD does not
have. `DOLT_BACKUP('sync-url', …)` (the off-host backup) commits the
session's working set to the chunk store and copies the whole noms root,
working sets included (`dprocedures/dolt_backup.go` `syncRemote`;
`env/actions/remotes.go` `SyncRoots`), so a store without snapshot commits
backs up whole.

## 2. What either reading means for the disk

Under H1 oldgen grows about 1.6 to 2.5 GB a day, under H2 up to 60 GB a
day. Either way it grows without bound until a squash and a full GC, and
the only alarm (the daily audit at 50 GiB oldgen) has no consumer. At 36 GiB
it is the largest single reclaimable holder on the host after the VM
images.

## 3. Runbook: one supervised squash and full GC, server stopped (green; every step on Node B is the owner's call)

The tool is on main since `aab89bfe0` (2026-10-19; FP0 confirms the unit with `systemctl cat`):
`go-choir-platform-dolt-squash.service` runs
`scripts/platform-dolt-squash.sh` (stop, reflink backup, squash to one
commit, bounded full GC `--archive-level 0 --incremental-file-size
268435456` under `MemoryMax=10G` and a 5 GiB Go limit, root-hash check,
restore on any failure, restart, query check, keep the newest backup).
Its record ([store-a-weekly-squash-2026-10-20.md](store-a-weekly-squash-2026-10-20.md),
"Supervised first run") has the operator commands; what follows is
what changed or was not there.

**Window.** After the paper's 04:00 compaction has finished (the paper is
then freshly compacted and appends less), not between 02:30 and the end of
the off-host backup (it calls `DOLT_BACKUP` on Store A and is not stopped
by the script: it only `After=`s Dolt), no deploy in flight, no computer
creation or refresh planned. Store A down 10 to 30 minutes, estimated:
stops under 1 min (each unit's `TimeoutStopSec` is 20 s), the reflink
backup about a minute (36 GiB of large archive files), the squash seconds,
the full GC proportional to the **live** set, not the history (the paper's
52 GB to 8 GB took 2 min 19 s; Store A's live set is unknown, at most the
36 GiB, likely a few GB since the corpus moved to Store B), restart and
query check under 2 min. Abort past 45 min (`systemctl stop
go-choir-platform-dolt-squash.service`; the backup is restored).

**Free space.** The backup is a reflink, so the run needs free space for
the new oldgen only: the live set L, at most 36 GiB. Two binding
conditions, both read just before the start:

- `df` free at least **90 GiB + L + 10 GiB**, so a deploy is not refused
  afterwards (take L = 36 GiB unless the reading below gives less):
  about 136 GiB. The paper regrows 85 to 140 MiB a minute; start soon
  after its compaction.
- `btrfs filesystem usage /`: metadata under 90% used and at least 20 GiB
  unallocated (metadata was DUP at 84% of 18.6 GiB, 38 GiB unallocated at
  14:55). A reflink copy adds extent references to metadata; a CoW GC
  allocates data chunks. If either fails, do not start; a balance is a
  separate owner call.

**The space comes back only when the backup goes.** The script keeps the
newest backup, which pins all of the old 36 GiB. After the run, free space
is *lower* by L until the owner deletes
`/var/lib/go-choir/platform-dolt-squash-backups/platform-dolt.<ts>` after a
day's soak; then about 36 GiB minus L returns.

**What is down in the window** (read in `nix/node-b.nix`):

- Down: `go-choir-platform-dolt` (Store A) and what `Requires=` it:
  `go-choir-corpusd`, `go-choir-checkpointd` (+ timer),
  `go-choir-platform-dolt-history-audit` (+ timer);
  `go-choir-platform-dolt-users` is `PartOf=` Dolt and restarts with it.
  With corpusd down: every computer's event append
  (`/internal/computers/events/append`), checkpoints and file roots, the
  Autopaper front list and reader and publishing, capture search, the
  object graph routes (Store B itself stays up), key escrow, owner acts,
  corpusd's credential issuance (a computer created or refreshed in the
  window fails to get its credential), maild's digest reads.
- Up: auth and sessions (`auth.db`, SQLite; no Dolt), the proxy (static
  site and login; its corpusd routes return errors), the gateway (model
  calls), vmctl and running guests (vmctl only `Wants=` Dolt; its route
  database reads fail in the window: open hypothesis), Store B
  (`go-choir-corpus-dolt`).
- Hypotheses the supervised run settles: what a guest does when an event
  append fails (`internal/computerevent/http_client.go` does not retry; a
  desk run may fail and be closed); what vmctl does with its route
  database down. Watch `journalctl -fu go-choir-vmctl` and the paper's
  health through the window.

**Steps** (in addition to the squash record's 1 to 8):

0. FP0's readings of §1 (read-only) and `test -s
   /var/lib/go-choir/corpus-dsn.env` (the split is live). If oldgen is
   still growing past 45 GiB before the window, the 50 GiB audit will
   fail: that is a signal only.
1. Preconditions as in the record, plus the free-space and btrfs checks
   above, plus `ls /var/lib/go-choir/platform-dolt/platform/.dolt/sql-server.info`
   absent after the stop (the tool refuses on it).
2. `sudo systemctl start go-choir-platform-dolt-squash.service` in the
   foreground, the watchers in a second terminal.
3. Verify: `last-run.json` with `"squashed":true`, `root_before ==
   root_after`, `commits_after` 1 or 2; the units active; `du -sh
   /var/lib/go-choir/platform-dolt` (new oldgen about L; the total drops
   only in this directory, not on the disk, while the backup lives);
   `https://choir.news/health`; a paper source poll succeeding within
   30 minutes; vmctl `NRestarts` unchanged; the Dolt liveness probe clean.
4. Soak a day, then the owner deletes the backup (above), and runs
   `dolt fsck` on a reflink copy under `systemd-run -p MemoryMax=8G` if a
   check of the version skew (embedded module 2.2.0, server 2.4.2) is
   wanted.

**Rollback.** In the window: stop the squash unit; the script restores the
backup and restarts everything. After services have run on the squashed
store: stop corpusd and Dolt, `mv` the data dir aside, `cp -a
--reflink=always` the backup back, start; every write since the window is
lost, so only for a store that is wrong.

## 4. A lasting cadence: stop the snapshots at the source, squash rarely

**The fix (built, red, inert: `90ea1e378`).** `PLATFORM_DOLT_SNAPSHOTS=off`
stops Store A's `DOLT_COMMIT` snapshots: `doltbatch` gains a mode in which
`Mark`, `CommitNow`, `Flush` and `Close` are no-ops returning nil, and
`platform.openStore` applies it to the Store A committer only. corpusd and
checkpointd load the optional
`/var/lib/go-choir/platform-dolt-snapshots.env`; absent (as deployed),
nothing changes. With it, oldgen stops growing between squashes: a Default
auto GC with no new commit adds nothing to oldgen and rewrites in newgen
only what the working set has that HEAD lacks (the divergence D). Store A's
size is then bounded by its live set plus D, whatever the event rate, so
H1 or H2 stops mattering.

Why it is safe, read in code: nothing reads Store A's commits (§1); SQL
`COMMIT` is the durability (the doltbatch drill on 2.1.9; the new test
`TestModeOffWriteSurvivesReopen` on the embedded 2.2.0); auto GC still runs
(its hook fires on working-set updates); the off-host backup copies the
working set; the squash tool commits the working set (`-A`) and checks
HEAD root equals the WORKING root it recorded.

What it costs: each auto GC copies D, which grows by Store A's net live
change (mostly new event receipts) until the next squash folds it into
oldgen. Auto GC then runs less often (it waits until the store has grown
by more than newgen) and each run takes longer. Watch the Dolt journal's
`Successfully completed auto GC of database platform in …` durations.

**Order (each step the owner's or FP0's):**

1. Land `90ea1e378` (inert; the deploy restarts corpusd and checkpointd
   with the same behaviour). No-effect proof: the paper polls, corpusd
   healthy, `dolt_log` keeps growing as before (audit line).
2. Flip, observe first on the readings: `echo PLATFORM_DOLT_SNAPSHOTS=off
   | sudo tee /var/lib/go-choir/platform-dolt-snapshots.env && sudo chmod
   0600 … && sudo systemctl restart go-choir-corpusd`. corpusd logs
   `Store A snapshot commits off`. Over the next day: the oldgen `du` is
   flat across auto GCs (no new oldgen file), the audit's commit count is
   flat, newgen (`du -sk …/.dolt/noms` minus oldgen) grows slowly, auto GC
   durations stay under a minute, the liveness probe never aborts, the
   02:30 off-host backup succeeds.
   Stop rule: an auto GC over 2 minutes, a liveness abort, or corpusd
   errors: `sudo rm` the env file and restart corpusd (the next snapshot
   commits the whole working set; nothing is lost).
3. The supervised squash (§3), once, to drop the history already there.
   It can run before or after step 2; after is better (the oldgen it
   rewrites stops moving).
4. Cadence afterwards: the squash only folds D into oldgen, so it is
   needed rarely: monthly, or when newgen passes a few GiB, or when auto GC
   durations grow. Keep `platformDoltSquashTimerEnabled = false` while
   snapshots are off; each run is a 10 to 30 minute Store A outage.
   If the owner keeps snapshots on instead, the weekly timer is the
   bound: under H1 that retains about 11 to 18 GB a week; under H2 weekly
   is too slow and the timer needs a daily calendar, at a daily outage.

Not changed: the history audit's thresholds (the alarm stays at 50 GiB);
Store B's committer (`platform-corpus`, corpus-dolt 6.3 GB) has the same
mechanism and is out of this lane's scope (named residual).

## Open questions

- H1 or H2 (the readings above).
- The 6.2 GB file at 14:54.
- What a running vmctl does when Store A is down for a planned window
  (open since the squash record; the first supervised run settles it).
- What a guest does when its event append to corpusd fails during the
  window (`internal/computerevent/http_client.go` does not retry; the
  callers' behaviour is not read here).

## Review round 1 (bunch 75: Sonnet, DeepSeek, Gemini flash; all accept with edits; land inert, do not flip until fixed), 2026-10-22

Recorded before the fixes. Reviews: `~/runs/reviews/bunch75-{sonnet,deepseek,flash}.out`
(FP0's runs; patch digest `6901fc8e…f1f74`, head `752856812`).

- **R1 (Sonnet F1, DeepSeek 1, flash 1; medium): a second squash fails
  with snapshots off.** `internal/projectionbase/squash.go:214` makes the
  squash commit only when `dolt_log` holds more than 2 commits. After the
  first squash it holds 2, and with snapshots off HEAD never moves, so
  WORKING differs from HEAD; the next run skips the commit and fails at
  `:239` ("root changed by the squash commit", not a refusal before
  mutation). On Node B: `platform-dolt-squash` exits 1, the script restores
  the backup after a full Store A outage and leaves a
  `*.squash-failed-<ts>` directory that blocks every later run. Reproduced
  by Sonnet and DeepSeek. §4's "the squash tool commits the working set"
  and "monthly" were true only while the log holds more than 2 commits.
  The first squash (about 92k commits) is unaffected.
- **R2 (Sonnet F2, DeepSeek 2; low-medium): the restore drill's head
  check becomes vacuous.** `scripts/off-host-backup.sh` records the newest
  `dolt_log` hash in `manifest.json` `dolt_heads`;
  `scripts/off-host-restore-drill.sh` (`snap`) requires it in the restored
  log. With snapshots off the hash is frozen, so any restore of any age
  passes, while the working set is then the only copy of every write
  since the flip.
- **R3 (Sonnet F3; low-medium): the env wiring in `openStore` is
  untested.** Mutants P1 (the apply call dropped), P2 (a wrong env name)
  and P3 (an unknown value turns snapshots off at the seam) survive every
  test; P3 fails open against the record's own rule.
- **R4 (Sonnet F4, DeepSeek 3 and 5; low to medium): nothing alarms on
  newgen or the journal, and D is not bounded "whatever the event rate".**
  With snapshots off growth moves to newgen and the journal; the audit
  only checks oldgen (50 GiB) and an upper bound on the commit delta, so it
  can never fail. D is the post-flip live delta (mostly the append-only
  receipts and their four indexes) and grows with the event rate: the
  switch removes history overhead, not live growth, and "oldgen flat" can
  pass while total `noms` still grows. Auto GC under 2.4.2 was read in the
  2.2.0 module only.
- **R5 (Sonnet F5; info): the durability wording.**
  `TestModeOffWriteSurvivesReopen` closes the store gracefully; it proves
  persistence across a reopen, not crash durability. Crash durability is
  the 2026-09-19 kill -9 drill on 2.1.9 and Sonnet's probes on real Dolt
  2.4.2: a `sql-server` killed with -9 while a client inserted with no
  `DOLT_COMMIT` came back with all 93 acknowledged rows, `dolt_log`
  unchanged and the working set intact; the same with two `DOLT_GC` calls
  during the writes (108 rows). Sonnet also restored a `DOLT_BACKUP
  ('sync-url')` of an uncommitted working set on 2.4.2 (93 rows) and
  measured oldgen in a never-commit arm staying at 8 KiB while a
  commit-every-tick arm grew 1.4 to 8.0 MiB.
- **R6 (Sonnet §1): four operational readers of Store A's commit history
  the record missed** ("nothing reads it" holds for the product and every
  authority path, not for operations):
  1. the squash tool (`squash.go:211, 214, 239`): R1;
  2. the off-host backup manifest (`off-host-backup.sh`, `dolt_sync` and
     `snapshot_store` record `dolt_log`'s newest hash);
  3. the restore drill (`off-host-restore-drill.sh` `snap`): R2;
  4. the subvolume migration's check (`scripts/dolt-data-to-subvolume.sh`
     `head_of`, head equality before and after a one-shot move): stays
     equal with a frozen head; harmless, but it no longer proves anything.
  The history audit's `COUNT(*) FROM dolt_log` is a fifth, already named
  (R4).
- **R7 (DeepSeek 4): "Store B's committer is untouched" is conditional.**
  True with a separate corpus DSN; in the shared-pool fallback (empty
  `corpus-dsn.env`) the one committer serves both and the switch stops
  corpus-table snapshots too, by design.
- **R8 (flash 2): §4's sentence about the squash tool did not match the
  code** (the same defect as R1, in the record).
- Observed by Sonnet, harmless: with the file present checkpointd logs the
  "snapshot commits off" line at every run (1,440 a day).
