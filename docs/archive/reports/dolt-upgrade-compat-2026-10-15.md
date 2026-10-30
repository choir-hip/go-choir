# Dolt upgrade compatibility: server 2.1.9 to 2.4.2, embedded module to the v2.4.2 commit (2026-10-15)

Status: research, written before any code (Problem Documentation First).
Branch `ops/dolt-upgrade`, not pushed. Mutation class of this document: green.
Mutation class of the upgrade it prepares: **red** (the persistent store
substrate: Store A, Store B, and every computer's embedded store).

Owner ruling (2026-10-15): "update Dolt to the latest version; research that
everything is compatible first." Background:
[dolt-clustering-assessment-2026-10-15](../problems/dolt-clustering-assessment-2026-10-15.md),
[platform-dolt-deadlock-hangs-checkpointd-and-crashloops-vmctl-2026-10-15](../problems/platform-dolt-deadlock-hangs-checkpointd-and-crashloops-vmctl-2026-10-15.md),
embedded-dolt-single-connection-is-a-fixed-upstream-race-2026-10-11.

Tags: **[E]** evidence (command output, upstream source or release text, file
and line). **[H]** hypothesis. Sources: `gh api repos/dolthub/dolt/releases`
(all notes from v2.1.10 to v2.4.2, read), the `dolthub/dolt` commit history for
the format-defining paths (`gh api .../commits?path=`), the upstream
compatibility test suite at v2.4.2, read-only `ssh node-b`, and this repo.
Nothing was run against a real Dolt server; no credential was read.

## Verdict

1. **No blocker found.** The server binary and the embedded module can both
   move to 2.4.2. Compile surface is tiny (see section 5).
2. **Storage format: no version change found.** The manifest (`5:__DOLT__`),
   the journal record format and the archive (`.darc`) format have no commits
   that change them between 2.1.9 and 2.4.2 [E, section 3]. The rollback is
   *probably* clean for the server binary, **with one loud exception: descending
   indexes** (2.3.3). A table that carries a descending index written by 2.3.3
   or later cannot be read at all by an older binary ("table has unknown
   fields"). Our guest schema declares four `... created_at DESC` indexes
   (`internal/store/texture.go`), so a computer created fresh on the new
   embedded module carries unreadable-by-old tables. That is a one-way door for
   those computers unless the DDL is made ASC first (the new engine would
   otherwise start honoring a `DESC` that Dolt silently dropped before). The
   branch includes that change as its own commit.
3. **Relevant fixes are real but none is a confirmed match for the 10-15
   stall.** There is a cluster of garbage-collection concurrency fixes in 2.4.0
   (a lost wakeup in `waitForGC`, a read barrier before a GC installs its
   keeper, generation ordering on reads) and one data-corruption fix for
   non-archive GC (2.3.3) that applies directly to our guests. Section 4.
4. **Rollback reality:** the file-level backup is the only real rollback for
   data. The binary rollback is a nix generation revert; it is data-safe only
   while no descending (or extended vector) index and no other new-format
   object has been written. Section 7.

## 1. Every form of Dolt in this repo [E]

| # | Form | Where | Current | Target |
|---|---|---|---|---|
| 1 | `dolt sql-server` for Store A (platform, :13306) | `nix/node-b.nix` unit `go-choir-platform-dolt`, ExecStart `${pkgs.dolt}/bin/dolt sql-server` | **2.1.9** (`dolt version` on Node B; `/nix/store/psvajwk2...-dolt-2.1.9`) | **2.4.2** |
| 2 | `dolt sql-server` for Store B (corpus, :13307) | same file, unit `go-choir-corpus-dolt` | 2.1.9 (same package) | 2.4.2 |
| 3 | `dolt` CLI on the host: init scripts (`platformDoltInit`, `corpusDoltInit`), `doltEnsureUsers` (`dolt sql` as root/app user), the history audit (`dolt sql -q "SELECT COUNT(*) FROM dolt_log"`), `path = [ pkgs.dolt ]` on a client unit, and `environment.systemPackages` | `nix/node-b.nix` lines 38-52, 115-150, 225-247, 855, 1792 | 2.1.9 | 2.4.2 (all use `pkgs.dolt`, so one override moves them together) |
| 4 | dev shell `dolt` | `flake.nix` `mkDevShell` (`devPkgs.dolt`) | nixpkgs-locked 2.1.9 | 2.4.2 |
| 5 | **Embedded engine**: `github.com/dolthub/dolt/go` linked into guest computers (each computer's embedded Store, `internal/store/texture.go`, `internal/objectgraph/dolt_store.go`), `checkpointd`, `projection-compact`, `internal/computerversion`, `internal/platform`, `internal/doltbatch` | `go.mod` | `v0.40.5-0.20260716231223-6bea93f2f665` = upstream commit 6bea93f2f ("Fix some data races surfaced by running go-sql-server-driver tests under -race", 2026-07-16). It sits after v2.2.0 and before v2.2.2 (`gh api compare`: ahead of v2.2.0, 49 commits behind v2.2.2) | v2.4.2 = commit `9c835f4dc797` (tag object equals the commit) |
| 6 | go-mysql-server, vitess (transitive) | `go.mod` indirect | gms `v0.20.1-0.20260715210958-36c4fed9afa6`, vitess `v0.0.0-20260624214226-81d034e0fde8` | gms `...20261003020513-77662b650e46`, vitess `...20260930232419-a6b7b0c65d17` (v2.4.2's own go.mod) |
| 7 | `github.com/dolthub/driver/v2` (the `database/sql` embedded driver) | `go.mod` | v2.2.0 | v2.4.1 (newest tag; its diff against v2.2.0 is `go.mod` only, `gh api compare`; requires `go 1.26.5`) |

Guests do **not** carry a `dolt` binary: `nix/autoputer-vm.nix` has no Dolt
reference [E, grep]. The guest image therefore changes only through form 5.
There is **no `replace` and no fork** in `go.mod`; the pseudo-version is a plain
upstream commit pin, not a patched copy [E]. The release tag `v2.4.2` names the whole repo while the Go module is the
`go/` subdirectory, so the module is pinned by commit (the driver pins the
same way) [H on the reason, E on the practice]. Latest upstream release:
**v2.4.2, published 2026-10-06** (`gh api repos/dolthub/dolt/releases`); the
server's own warning on Node B says the same ("The newest version is 2.4.2").
nixos-unstable already packages `dolt 2.4.2` (`pkgs/by-name/do/dolt/package.nix`),
so the override can reuse its two hashes.

The incident docs describe the embedded pin as `a6690826d767`. That was the
driver's pin at v2.2.0; `go.mod` was later bumped to `6bea93f2f`. Practical
consequence: **the single-connection race fix (6bea93f2f) is already in our
embedded module**; this upgrade does not newly remove that race.

## 2. What changed between 2.1.9 and 2.4.2

Sixteen releases (2.1.10 06-26, 2.1.11 07-15, 2.2.0 07-15, 2.2.1 07-17,
2.2.2 07-22, 2.2.3 07-30, 2.2.4 08-12, 2.3.0 08-13, 2.3.1 08-19, 2.3.2 09-02,
2.3.3 09-09, 2.3.4 09-14, 2.3.5 09-16, 2.4.0 09-30, 2.4.1 10-02, 2.4.2 10-06).
The two minor bumps that matter for compatibility: **2.2.0** (a JWT-auth
change, section 5) and **2.3.3** (descending indexes, section 3).

### 2.1 Breaking or behavior changes

| Change | Release | Touches us? |
|---|---|---|
| `authentication_dolt_jwt` now validates the Subject claim (backward incompatible for JWT users) | 2.1.11 (#11296), minor bump 2.2.0 | **No.** We authenticate with password users (`doltAuthStage = "users"`, `nix/node-b.nix`); no JWT plugin is configured. |
| Descending indexes and NULL ordering; "feature version bump because old databases trying to read a DESC index would read it incorrectly" | 2.3.3 (#11746) | **Yes**, see section 3. |
| Default `SQL_MODE` now matches MySQL's; upstream says "does not change any query behavior associated with the new default modes", plus warnings for invalid strict combinations | 2.3.3 (gms #3768, #3723) | Low. We run default modes; worth the store test pass. |
| Many correctness fixes that change results: integer division of strings as decimals (#3777), `GROUP_CONCAT` keeps empty strings (#3774), `JSON_LENGTH`/member-access (#3598), JSON numbers too large for float64 (#3699) and `JSON_EXTRACT` large integers (#11416), JSON control-character escaping (#3589), `LIKE` backslash escapes (2.4.1), `GREATEST`/`LEAST` decimals, arithmetic on `BIGINT UNSIGNED` | 2.1.10 to 2.4.1 | Low to medium. We use `JSON_EXTRACT`/`JSON_UNQUOTE` heavily and index **STORED generated columns over JSON_EXTRACT** (`internal/objectgraph/dolt_store.go` lines 232-285). See risk R3. |
| `Sum`/`Avg` report float64 truthfully (#3614; the analyzer already forced float64); decimal handling in `CONVERT`, `GREATEST`/`LEAST` (#3946, #3979) | 2.1.11, 2.4.0, 2.4.1 | Only if code type-asserts engine types; we reach the engine through `database/sql` only. Test the `SUM`/`AVG`/`COUNT` users in `internal/store`. |
| CLI: boolean results print as 1/0, `dolt log` rewritten (much faster), `dolt version` records a failed check in `version_check.txt` and times out (fixes #12021, a 30 s block when api.github.com is unreachable) | 2.4.0 to 2.4.2 | Low. The history audit and `doltEnsureUsers` run `dolt sql` on Node B; #12021 is a plus there. |
| `sql_mode` set with `SET PERSIST` is now persisted as a string and reloaded at startup | 2.3.4 (#11788) | None: we do not `SET PERSIST` (grep: no match in `internal/`, `cmd/`, `nix/`). |
| `config.yaml` schema | none found | `go/libraries/doltcore/servercfg` has one commit in the window (test certificates). The two live `config.yaml` files contain only the `listener` block (host, port) [E, ssh]. Nothing to migrate. |

### 2.2 Toolchain

- `dolt/go` v2.4.2 `go.mod`: `go 1.26.2`. `driver/v2` v2.4.1 `go.mod`:
  `go 1.26.5`, so `go.mod`'s `go` line rises from 1.26.2 to 1.26.5.
- Our compiler is 1.26.9 (`flake.nix` `choirGoRelease`, `toolchain go1.26.9`),
  so the floor is met with room. No Go bump is needed.
- Dolt 2.2.2 also bumped `golang.org/x/net` to v0.55.0 "for Go 1.27
  compatibility"; we are already on v0.55.0.

## 3. Storage format and the rollback question [E unless marked]

What was checked: the commit history of every path that defines on-disk bytes
in `dolthub/dolt` for 2026-06-22 to 2026-10-07.

| Path | Commits in window | Meaning |
|---|---|---|
| `go/store/nbs/manifest.go` | 2 (backup-sync prune locking only) | manifest format (`5:__DOLT__:...`) unchanged. Both live manifests start `5:__DOLT__` [E, ssh]. |
| `go/store/nbs/journal_record.go` | 2 (the fslock to file-locks import rename) | journal record format unchanged. |
| `go/store/nbs/archive_*` | writer cleanup on error; reader batching; **footer and index validation on open** (e1c71e081, 09-16) | `.darc` byte format unchanged. Stricter checks only. |
| `go/store/constants`, `go/store/types/format.go` | 0 | the format version constant did not move. |
| `go/serial/*.fbs` | prolly.fbs: a new field for key addresses (Doltgres only; Dolt "always requires prefixes for long keys"); schema.fbs: **descending index columns** (#11746); vector index extension (#11594) | additive flatbuffers fields. |

Compatibility facts from upstream's own suite (v2.4.2
`integration-tests/compatibility/`):

- The forward-compatibility list for the 2.x line is `v2.0.0` only
  (`2_0_forward_compatible_versions.txt`): CI checks that v2.0.0 reads and
  writes repos made by the current build. 2.1.9 was not individually tested
  by upstream but sits inside the tested span.
- The one deliberate break: `desc_index_breaking.bats`. "Descending columns are
  written as new fields of the index schema message, and older clients reject
  schemas with fields they don't know with 'table has unknown fields'.
  Indexes without a descending column don't write the fields." The test also
  asserts that an old client still reads a table whose index uses the default
  order, and still does after the new client alters a column type.
  Consequence: **the break is per table, and only for tables that declare a
  DESC index column** (the old client fails on `SELECT`, `schema show`, and
  `diff` for that table).

What this means for us:

- **Store A and Store B (server binary).** Their DDL has no `DESC` index
  [E, `grep -rnE "INDEX[^;]*DESC"` over `internal/`, `cmd/`, `scripts/`: only
  `internal/store/texture.go`]. A 2.4.2 server running our existing platform
  and corpus DDL writes the same bytes an old one does. Downgrade to 2.1.9 is
  expected to work. **Not verified against the 2.1.9 server itself** (upstream
  tests start at v2.0.0; the local round trip in section 9 covers the embedded
  module pair, not the server binary).
- **Guest embedded stores.** `internal/store/texture.go` creates
  `idx_texture_revs_doc_created`, `idx_agent_evidence_owner_agent`,
  `idx_agent_evidence_owner_created` and `idx_texture_revs_doc_version` with
  `created_at DESC` / `version_number DESC`. Before 2.3.3 Dolt accepted and
  ignored `DESC`. After 2.3.3 a *new* `CREATE INDEX` is a true descending
  index and the table becomes unreadable to an older module.
  - Existing stores: the statements are `CREATE INDEX IF NOT EXISTS`, the
    indexes already exist, so they are no-ops and stay ASC. [H, confirmed in
    the local test, section 9.]
  - New stores (a fresh computer, a restore into an empty workspace): would be
    created DESC and become one-way. **Fix before the guest rollout: write the
    four indexes without `DESC`.** That reproduces exactly what the pinned
    engine stored, and the planner still scans an ASC index backwards for
    `ORDER BY ... DESC`.
- **Other one-way operations to avoid after the upgrade**: `VECTOR` /
  extended vector indexes, `dolt_squash_history` is fine (it only writes
  commits), a `sql_mode` persist is fine, partial indexes (`CREATE INDEX ...
  WHERE`, gms #3553) and multi-expression functional indexes (#11327,
  gms #3634) are new schema features whose old-reader behavior was **not**
  checked: do not use them until the soak is over.
- **Stricter open-time checking is a new failure mode.** 2.4.0 (#11773) checks
  sorted prefixes, ordinal ranges and offsets on every index parse, and
  checks hash and uniqueness when a table file is added; e1c71e081 validates
  archive footers. A table file that was already damaged will now fail
  loudly at open instead of later. The known producer of such files is
  #11757 (below), fixed in 2.3.3. `dolt fsck` itself still panics instead of
  reporting a broken table-file index (#11775, open).

## 4. Upstream items relevant to our incidents

"In pin" = already inside our embedded commit 6bea93f2f (2026-07-16). "In
2.1.9" = in the Node B server. Release attribution comes from the release
notes; the commit dates are in the commit history.

| Our incident | Upstream item | Fixed in | In pin? | In 2.1.9? | Relevance |
|---|---|---|---|---|---|
| 10-15 platform stall at a conjoin plus auto GC; hung `DOLT_COMMIT` | **#11888** "Fix a lost cancellation wakeup in `waitForGC`" (take `nbs.mu` when broadcasting `gcCond`, so the waiter is parked when the wakeup arrives) | 2.4.0 | no | no | **Closest to the observed shape**: an operation waits for an in-progress GC and a wakeup is lost. A committer that waits behind `waitForGC` with a context that cancels would sit forever. [H] that this is our stall; we have no stack. |
| same | #11891, #11889 "Drain outstanding reads before a GC installs its keeper"; "sample the keeper in `beginRead`, register every unlocked read" | 2.4.0 | no | no | GC/read interleaving; a barrier is added. Correctness (GC could collect a chunk a read had taken) more than liveness. |
| same | #11892 read new generation before old (a chunk moving new to old during GC could be missed); #11887 NodeStore/ValueStore caches no longer repopulate across a Purge | 2.4.0 | no | no | Reads missing retained chunks during GC (errors such as "chunk required but not found"). |
| same | #11312 race between the journal persister and `PruneTableFiles`: "Racing GC and/or crashing at an inopportune time could result in 'root hash not found' or a panic" | 2.2.2 | **no** (commit a287668d4 is not an ancestor of 6bea93f2f) | no | Applies to Store A/B (auto GC) and to every guest (online GC, boot GC). |
| same | #11796 `journal_writer.go`: a rare race when the caller's context is cancelled during bootstrap leaves the writer in a "weird state" | 2.3.4 | no | no | Boot-time journal open under a cancelled context (our boot has deadlines). |
| 10-09/10-15 conjoin of 257 old-generation files | #11742 conjoin clones table-file sources already open instead of reopening them; #11592 a failed conjoin is logged as deferred, with a `deferred` field | 2.3.3, 2.3.2 | no | no | Cheaper conjoin; better log. The conjoin log line at 09:23 is also the only machine-readable marker we have. |
| Guest GC (`DOLT_GC()`, `DOLT_GC('--full')`, boot GC, online GC, `internal/projectionbase/compact.go`) | **#11757** "Fix an error in non-archive push and GC paths which could result in table files with missing indexes ... the file itself was unusable without manual recovery" (BufferedFileByteSink boundary). Reported as #11747: an auto-GC published a manifest whose root chunk was absent; a 208,869-chunk output | 2.3.3 | **no** | no | **Highest-value fix for guests.** Our guest GC is the non-archive default and writes large outputs (journals of 13 to 27 GiB). Possible silent producer of damaged table files today. [E for the bug; H for whether any of our stores is already affected.] |
| Auto GC races a branch/working-set write | #11343 retry branch creation on `ErrOptimisticLockFailed` "e.g. from a background auto-GC cycle" | 2.2.3 | no | no | Same interaction family. |
| Journal repair | #11185 (journal.idx handling), `dolt fsck --revive-journal-with-data-loss` | 2.1.5 | yes | yes | Already present; no new repair tooling since. |
| Journal growth, history growth (27 GiB journal, 22 GB oldgen) | none | n/a | n/a | n/a | **No upstream change addresses retention.** Journal size is policy (ours): commit cadence and GC cadence. The upgrade does not shrink it. |
| `dolt_squash_history` | #11365 new procedure: "take the content of HEAD, and squash history before it ... much faster than rebasing" | 2.2.3 | no | no | A supported replacement for the hand-rolled squash in `internal/projectionbase/compact.go` and the Store A squash runbook. Not part of this upgrade; an opportunity. |
| Embedded single-connection / shared cache race (10-11) | 6bea93f2f copy-before-write in `unescapeHTMLCodepoints`; plus #11313 further `-race` fixes | 2.2.2 | **yes** for 6bea93f2f | yes | Already in our pin. Remaining -race fixes (auto-increment tracker init race #11585, SequenceTracker race #11577, #11313) are newer. Raising the connection count is a separate decision with its own race test. |
| Query deadlock class | gms #3630 listener-shutdown race (Accept goroutine could send on a closed channel) | 2.3.3 | no | no | Shutdown path only. (gms #3587, the strftime lock leak that can deadlock queries, merged 2026-06-12 and is already in 2.1.9.) |
| Memory | gms #3696 cap initial heap size, #3952 account for COUNT DISTINCT memory (the `CachedResults` rewrite #3572 is already in 2.1.9) | 2.3.1, 2.4.1 | no | no | Small. **No upstream item addresses a 24 GB oldgen or the 3.5 GB peak RSS.** Treat memory as unchanged until measured. |
| Reaper for stuck sessions | #11194 `GET_LOCK` held by a crashed client retained indefinitely (fixed); 8 h listener timeouts remain defaults | 2.1.11 | no | no | Not our hang (no `GET_LOCK` use). The 8 h timeouts are config we still own. |

Reading of the stall: the best-supported hypothesis in the clustering
assessment was a maintenance-path stall. 2.4.0's GC changes are the only
upstream work that touches that path and its liveness, and they were landed
after, and apparently because of, races found by running Dolt's server tests
under `-race`. That is an argument for upgrading before replacing Dolt, **not
evidence that the stall is fixed**. The probe-with-ABRT proposal in the
assessment (4.3 #1) stays valuable: ship it before or with the bump so a
recurrence is attributable.

## 5. API and compile surface in this repo [E]

Imports of `github.com/dolthub/*` in Go source:

- `github.com/dolthub/driver/v2` in 13 files (store, doltbatch, platform,
  objectgraph, computerversion, projectionbase, agentcore tests, proxy tests):
  `embedded.ParseDSN`, `embedded.NewConnector`, `cfg.BackOff`, the
  `database/sql` driver. Upstream diff v2.2.0 to v2.4.1: `go.mod` only.
- `github.com/dolthub/dolt/go/libraries/doltcore/memlimit` in
  `internal/gomemlimit/gomemlimit.go` (and its test): `memlimit.Init` and
  `memlimit.NodeCacheSize`; both still exist in v2.4.2.
- **No import of go-mysql-server or vitess anywhere**; they are transitive
  only. So go-mysql-server and vitess API changes cannot break our compile; they
  can only change SQL behavior (section 2.1).
- No `replace`, no vendored copy, no local patches.
- Go module vendor hash: `flake.nix` `commonGoArgs.vendorHash` (line 138)
  covers the whole module graph, so changing `go.mod`/`go.sum` changes it.
  The CI/deploy build prints the new value from its first run; the branch
  computes it locally (see the commit body).

## 6. Risks

| # | Risk | Likelihood | Mitigation |
|---|---|---|---|
| R1 | Descending-index one-way door for new guest stores | certain if DDL unchanged | Remove `DESC` from the four index statements before the module bump ships (own commit). |
| R2 | Open-time validation rejects an already-damaged table file (from #11757 on 2.1.9 or an older embedded pin) and the computer's store does not open | low, unknown | Pre-flight on a *copy*: new-binary `dolt fsck` and a throwaway server on the Store A backup; for guests, a copy of the paper computer's store. A failure is a finding, not a regression: the file was already bad. |
| R3 | JSON function semantics changed (large integers, control-character escaping, member access): STORED generated columns over `JSON_EXTRACT` computed before and after the bump could differ on edge inputs | low | Run `internal/objectgraph` and `internal/store` suites on the new module; the generated columns are recomputed only for new rows. |
| R4 | First open of a 48 GB platform store under new open-time checks is slow or memory-heavy | medium, unmeasured | Time it on the backup copy before the switch; unit has `MemoryHigh=3G`. Keep the `go-choir-platform-dolt` start limit (10 in 600 s). |
| R5 | Both Dolt units restart together with dependents (`requires = go-choir-platform-dolt`) | certain | Normal deploy path; schedule outside active work. A Dolt restart is a crash-shaped restart for clients: no resume (see "Restarts End Work" in AGENTS.md). |
| R6 | Memory behavior differs under the 3G / 5G cgroup limits | unknown | Watch `memory.events` and RSS for 48 h after the bump. |
| R7 | The upgrade does not fix the stall | likely | Probe-with-ABRT first or with it; decide on replacing Dolt for Store A only after a stack. |
| R8 | Guest fleet change is immediate on next boot, for every computer | certain | Canary one non-paper computer; soak the server binary first. |

## 7. Rollback story

| What | How | Data caveat |
|---|---|---|
| Server binary | revert the flake commit and deploy, or boot the previous NixOS generation; `/nix/store/psvajwk2...-dolt-2.1.9` stays in the store until garbage-collected | Safe while no descending / vector / partial / multi-expression-functional index exists in a database the server touched. Our Store A and Store B DDL has none. Not tested against 2.1.9 itself by upstream. |
| Embedded module (guest image) | revert the `go.mod`/`go.sum` commit and ship the previous guest image | Safe for stores created before the bump (indexes stay ASC). **One-way for stores created on the new module with a `DESC` index**: hence R1's DDL commit must ship first or with the module. |
| Data | **Pre-bump backup of both Dolt data dirs while the units are stopped.** Node B is btrfs (`/dev/md126`, zstd): `cp -a --reflink=always` of `platform-dolt/platform` (48 G) and `corpus-dolt/corpus` (934 M) is near-instant and costs little space until the stores diverge; 116 G free of 476 G. (`corpus-dolt/rebuild-20261009`, 22 G, is a leftover from the 10-09 rebuild, not live.) | A reflink copy shares extents with the live files. It protects against a bad bump, not against disk loss. Do not delete the live originals to "free space" while the copy exists. |
| Per-computer state | restore is a forward transaction on the computer's event chain (AGENTS.md "Product restore vs platform deploy") | Not touched by this upgrade. |

**State plainly: no on-disk format version bump was found, so downgrading the
server is expected to work. The only known irreversible write is a descending
index (and the vector / partial / multi-expression features). If any of those
are created on the new version, the older version cannot read that table
until the index is dropped on the new version.** Dropping the index on the new
version and then downgrading is the repair path.

## 8. Recommended sequence

0. **Land this doc and the DDL commit** (no `DESC` in the four guest indexes;
   behavior-preserving; orange). This alone removes the one-way door and ships
   with the normal landing loop (guest image).
1. **Probe-with-ABRT and bounded clients** from the assessment (4.3 #1, #2),
   so a recurrence of the stall is attributable. Independent of the bump.
2. **Pre-flight on Node B, no production change**: build `dolt 2.4.2` (nix
   store only); stop both Dolt units, take the reflink backups, start them
   again (short outage); on the backup copy run the new binary's `dolt fsck`
   and a throwaway `sql-server` on another port to time the open and record
   RSS. Record the numbers in this document.
3. **Server binary** (this branch's first commit): deploy through the landing
   loop; verify `dolt version`, `SELECT 1` through the app user, journald free
   of open-time validation errors; observe one auto-GC and one conjoin cycle;
   soak 24 to 48 h.
4. **Embedded module** (this branch's second commit): `go.mod`, `go.sum`,
   `vendorHash`; guest image build; canary one non-paper computer and watch
   boot, GC disposition (`dolt_gc` on guest `/health`) and journal size; then
   the fleet at next boot. The paper computer last (it carries the largest
   journal; its existing journal was written by the older, #11757-affected
   GC).
5. Later, separately: `dolt_squash_history` in place of the hand-rolled
   squash, more than one embedded connection, a timeout in `config.yaml`.

## 9. Local verification (Phase 2, branch `ops/dolt-upgrade`) [E]

Commits on the branch, in order: the research doc; the Nix lane for the
server binary; guest indexes made ascending; the embedded module bump.

**Nix (evaluation only, nothing built).** `nix eval` on
`nixosConfigurations.go-choir-b`: `pkgs.dolt.version = 2.4.2`,
`src.rev = refs/tags/v2.4.2`, `go-choir-platform-dolt` ExecStart is
`/nix/store/h6amjbl3...-dolt-2.4.2/bin/dolt sql-server ...`. The source hash
was re-derived independently (`nix flake prefetch github:dolthub/dolt/v2.4.2`,
narHash equal). The vendor hash is confirmed by the fixed-output path
`dolt-2.4.2-go-modules`, which exists in cache.nixos.org (Hydra built it from
the same hash). The override retires itself when the base nixpkgs reaches
2.4.2 (`versionAtLeast` guard, same pattern as `choirGo`).

**Go build.** `GOMAXPROCS=2 GOFLAGS=-p=2 go build ./...` is clean after
`go get ...@9c835f4dc797 driver/v2@v2.4.1` and `go mod tidy`; no source change
was needed. The bump also moves non-Dolt direct dependencies because Dolt's own
`go.mod` requires them: `x/crypto` 0.52 to 0.55, `x/net` 0.55 to 0.58, `x/sys`
0.45 to 0.47, `x/text` 0.37 to 0.41 (auth, proxy and others compile against the
new ones), plus grpc 1.79 to 1.83 and the OpenTelemetry/Google Cloud
indirects. `go.mod`'s `go` line is now 1.26.5.

**Vendor hash.** `commonGoArgs.vendorHash` had to change. Method: reproduce the
filtered source tree of `goServicesSrc` (cmd services plus `internal`, no
`*_test.go`), run `go mod vendor`, and `nix hash path --type sha256 --sri` the
vendor directory. On the pre-bump `go.mod`/`go.sum` this printed
`sha256-aC9g7xO+TmLKCOmmS6TkrtAhMBCYtEY3tgqWgZkv6WI=`, exactly the committed
value, so the method is sound; on the new files it prints
`sha256-Odhx6ccFIO3H63WsXaX/WU2PBkZrf1S76RAMJhwpIgM=` (committed). Not built
under Nix here; the first CI run is the confirmation (the "fix the hash from
the first Nix build error" fallback applies).

**Focused tests (new module, Mac, one package at a time), all passing:**
`internal/store` (`Texture|ListEvidence`, then
`DoltGC|PeriodicDoltGC|GraphStore|Fresh|Migration|Fingerprint|ScanCost`),
`internal/objectgraph` (whole package, covers the STORED generated columns over
`JSON_EXTRACT`), `internal/doltbatch`, `internal/gomemlimit`,
`internal/computerversion`, `internal/projectionbase` (whole package, includes
the `DOLT_GC('--full')` compaction), `internal/platform`
(`TestRunArtifactGCDryRunAndActive`, `TestRunArtifactGCOGLiveSetReadsCorpus`),
`internal/agentcore`
(`TestStandingConsensusEndToEndFreezeVerifyAcceptApplyWithoutPerCandidateOwnerAct`).
Not run: the full `internal/agentcore` and `internal/textureowner` packages
(per the repo rule), and anything against Node B or a real Dolt server.

**Round trip between the pinned and the new embedded module** (a throwaway
program, deleted after use, built twice with `-modfile` and run on copies of
the data directories; the same DDL shapes as `texture.go`):

| Test | Result |
|---|---|
| store created by OLD (DESC DDL), read, written and re-`CREATE INDEX IF NOT EXISTS ... DESC` by NEW | works; `SHOW CREATE TABLE` still shows ascending keys, the statements are no-ops |
| the same store then read by OLD after NEW wrote to it | works (the written row is visible) |
| OLD printing `SHOW CREATE TABLE` for its own DESC DDL | prints the keys **without** `DESC`: confirms older Dolt silently dropped `DESC` |
| store created by NEW with the old DESC DDL | `SHOW CREATE TABLE` shows ``(`doc_id`,`created_at` DESC)``; NEW reads and writes it |
| that NEW-created DESC store read by OLD | **fails: `Error 1105: table has unknown fields`** (the one-way door, confirmed) |
| store created by NEW with the ascending DDL (this branch), read and written by OLD, then read by NEW | works both ways |
| OLD-created store: `DOLT_COMMIT`, `DOLT_GC()`, `DOLT_GC('--full')` by NEW, then read by OLD | works |
| NEW-created store: the same maintenance by OLD, then read by NEW | works |

The server binary was not run locally (no 2.1.9 CLI on the Mac); the embedded
module pair exercises the same storage code (`go/store/nbs`, `go/store/prolly`,
schema serialization). The remaining unchecked path is the actual 2.1.9 server
reading a 48 GB store written by 2.4.2; the backup is the answer to that.

## Residual unknowns

- Whether 2.4.0's `waitForGC` fix is the 10-15 stall. No stack exists.
- Whether any of our stores already contain a table file damaged by #11757.
- Open-time cost of the 48 GB Store A on 2.4.2.
- Whether upstream's #11775 (`dolt fsck` panic on a broken index) is fixed in
  a release after 2.4.2; it was still open at the time of writing.
- 2.1.9 to 2.4.2 round trip with the actual 2.1.9 server binary on real data
  (the local check uses the embedded module pair; see section 9).

## Heresy delta

- discovered: the incident docs name `a6690826d767` as our embedded pin; it is
  `6bea93f2f665`, which already carries the single-connection race fix. Our
  guest DDL declares four `DESC` indexes that were silently ASC and become real
  on the new engine. Guest GC runs the non-archive path whose large-output bug
  (#11757) is not in our pin.
- introduced: none (documentation only).
- repaired: none.
