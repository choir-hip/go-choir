# platform-dolt's oldgen: the rate, its cause, and the squash window (2026-10-22)

Date: 2026-10-22, 17:00 UTC. Lane: FP0 platform Dolt oldgen
(`lane/fp0-platform-dolt-oldgen`). Status: open; problem record before any
code. Companion to FP0's record
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

## Open questions

- H1 or H2 (the readings above).
- The 6.2 GB file at 14:54.
- What a running vmctl does when Store A is down for a planned window
  (open since the squash record; the first supervised run settles it).
- What a guest does when its event append to corpusd fails during the
  window (`internal/computerevent/http_client.go` does not retry; the
  callers' behaviour is not read here).
