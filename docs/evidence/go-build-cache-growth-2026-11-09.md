# Go build cache growth: root cause, fixes and runbook (2026-11-09)

Mutation class: green/yellow for the docs and the script; orange for the
test-support edits and `.envrc` (they change how local builds key their cache,
not product behaviour). Rollback: revert the commit; the cache is derived data.

Scope: the shared `GOCACHE` (`/Users/wiz/Dev/.caches/go-build`, set in
`~/Library/Application Support/go/env`, not in `.envrc` or the flake) on the
macOS host. Go 1.26.10 (Nix), no cgo. Every experiment below ran against a
temporary `GOCACHE` under `/private/tmp`, in two throwaway worktrees at the same
commit (6e0d016a3); all of it is deleted. The shared cache was only read.

"Observed" means measured here. "Inferred" and "speculation" are marked.

## 1. Root cause

The cache grew about 20 to 36 GB/day (observed: entries last used 11-07 18 GB,
11-08 30 GB; the pre-trim 180 GB over Go's five-day retention is 36 GB/day).
Go keeps an entry five days after its last use (`trimLimit = 5*24h`, verified
in `cmd/go/internal/cache/cache.go` of this toolchain), so steady state is
about 5 x 30 = 150 to 180 GB. That matches the 180 GB seen this morning.

Where the bytes are (shared cache at 09:12, 72.7 GB, 222,964 files):

| Slice | Files | Bytes |
|---|---|---|
| `-d` output files (compiled package archives) | 41,775 | 73.2 GB (99.4%) |
| `-a` index entries | 181,099 | 0.03 GB |
| files under 1 KB | 188,752 | 0.03 GB |
| files 1 to 10 MB | 6,210 | 19.1 GB |
| files 10 to 50 MB | 969 | 24.2 GB |
| files over 50 MB (none over 100 MB) | 332 | 25.2 GB |
| top 1,000 files | 1,000 | 45.5 GB (62%) |
| module packages (this repo) among files >=100 KB | 16,685 | 68.1 GB (94%) |

Observed: every giant is a compiled package archive (`ar` archive, one
`_go_.o` member), not a linked binary. `go test` does not cache its linked test
binary (the link ran on every repeat, the cache did not grow: D2, D4). By
dominant package: `agentcore` 716 of the top 1,500 files, 35.6 GB; `store` 7.1
GB; `platform` 3.6 GB. `agentcore` is 174k lines in 610 files; one test-variant
archive is 77 to 79 MB, the plain one 37 MB (race 91 MB, cover 82 MB).

Ranked causes:

1. **Every source edit rewrites huge archives. Confidence: high. About 79% of
   module-package bytes.** Observed: grouping the 16,685 module archives by
   (worktree, package, test or not) gives 5,533 groups; the first version of each
   is 14.2 GB, the 11,152 later versions are 53.9 GB. One builder worktree's
   timeline (agent-a93bb0, 11-08 02:00 to 02:21) shows 25 `agentcore` test
   archives (76.7 to 77.2 MB, growing with each edit) plus a dozen plain 37 MB
   ones in 21 minutes. Measured cost of one edit to `agentcore` followed by a
   targeted test run: +76 MB, 2 packages recompiled (F0). The load that drives
   this (observed from 170 session transcripts of the last three days, 4,948 go
   invocations): `go test` 2,658 (1,568 with `-run`), `go build` 1,205 (538 of
   them `./...`), `go vet` 944, most naming `./internal/agentcore` (1,259 times).
   Each of build, vet and test of `agentcore` after an edit stores its own
   archive variant.
2. **Retention far longer than the reuse window. Confidence: high.** Observed:
   for entries over 1 MB, 96% of the bytes (65.6 of 68.3 GB) were never used
   again more than one hour after they were created (mtime minus birth time).
   Entries reused after more than an hour total 2.7 GB. Go's five days keeps the
   other 65 GB around for nothing. A 12 hour window would have deleted 55.0 GB of
   the 71 GB now (`scripts/go-cache trim --dry-run`).
3. **Worktree path in the action ID (H1). Confidence: high that it is real,
   bounded to at most 21% of module bytes.** Source: `cmd/go/internal/work/exec.go`
   line 299 hashes `dir <package dir>` into every non-GOROOT package unless
   `-trimpath` is set; `GODEBUG=gocachehash=1` shows
   `"dir /Users/.../agent-.../internal/types\n"` in the hash input. Observed (B1):
   the same commit built from a second worktree recompiled 114 packages and
   stored +162 MB (+206 MB for the `agentcore` test build, D3), while the
   standard library and third-party packages were reused. With `-trimpath` the
   second worktree recompiled 0 packages and stored +1 MB (C2, E2). The 14.2 GB
   "first version of each group" above is an upper bound for what this costs in
   the 2-day window (67 worktrees contributed; the 171 on disk are mostly idle).
   Builders that start from the same base commit would share these entries after
   the fix.
4. **Package size (H5). Confidence: high as a multiplier, not a defect.** The 332
   files over 50 MB are 25 GB, almost all `agentcore` variants. Compiler DWARF is
   38% of the `agentcore` test archive (79.3 MB to 48.8 MB with
   `-gcflags=-dwarf=false`, E3).
5. **Inconsistent configs (H3). Confidence: low, minor.** Observed: one
   toolchain only, go1.26.10, in all 21,425 archives over 50 KB (the go1.26.8
   store path is not in the window). By target: darwin/arm64 66.0 GB, linux/arm64
   5.3 GB, linux/amd64 0.6 GB (`GOOS=linux` is 285 of 4,948 invocations).
   `-race` is 119 invocations (2.4%) and a separate 91 MB archive per version;
   `-cover` and `-race` partition the cache (E4, E5). Not a driver, but each one
   doubles the per-edit cost for that edit.
6. **Volatile inputs (H4). Ruled out for local growth.** The `-ldflags -X`
   version stamps are in `flake.nix` (Nix builds, which do not use this cache).
   `-buildvcs` affects only the link, and links are not cached. `agentcore`
   archives grow 76.7 to 77.3 MB over a day of edits, which is source growth, not
   noise. I did not audit embedded-file timestamps beyond that.
7. **Redundant full-repo runs (H2). Partly true.** 538 `go build ./...` and 944
   `go vet` invocations add archive variants per edit (see 1), but the dominant
   unit is a single edit of `agentcore`, not a full-suite sweep. No `-a` flag was
   seen in the invocation sample.
8. **Toolchain defect (H7). Not needed.** Every behaviour is explained by the
   action-ID rules.

## 2. Measurements (temporary caches, `nice -n 15`, one machine, other builders
active, so wall times are noisy; sizes are exact)

Packages for A to C: `./internal/agentcore ./internal/store ./internal/platform`
(`go build -x`). D to F: `go test -x -count=1 -run '^$' ./internal/agentcore/`
(compiles and links the test binary, runs nothing). "compile" counts `/compile`
lines in the `-x` log. No source changed except where F says so.

| Exp | Run | Wall | Cache before to after | Compiles |
|---|---|---|---|---|
| A1 | worktree A, cold | 22 s | 0 to 489 MB (3,754 files) | 580 |
| A2 | A repeated | 1 s | +0 | 0 |
| B1 | worktree B, same commit, same flags | 8 s | +162 MB (+683 files) | 114 |
| B2 | B repeated | 1 s | +0 | 0 |
| C1 | A with `-trimpath`, cold | 22 s | 0 to 488 MB | 580 |
| C2 | B with `-trimpath` | 1 s | +1 MB | 0 |
| C3 | A without `-trimpath`, in the trimpath cache | 25 s | +481 MB | 580 (flags partition the cache) |
| D1 | test A, cold | 39 s | 0 to 538 MB | 587 (+1 link) |
| D2 | test A repeated | 3 s | +0 | 0 (link repeats; test binary not cached) |
| D3 | test B, no `-trimpath` | 24 s | +206 MB | 119 |
| D4 | test A, different `-run` regex | 4 s | +0 | 0 |
| E1 | test A, `-trimpath`, cold | 38 s | 0 to 538 MB | 587 |
| E2 | test B, `-trimpath` | 3 s | +1 MB | 0 |
| E3 | E1 plus `-gcflags=-dwarf=false` (test pkg only) | 30 s | 0 to 509 MB; `agentcore` archive 79.3 to 48.8 MB | 587 |
| E4 | E1 plus `-race` | 48 s | 0 to 592 MB; archive 90.7 MB | 588 |
| E5 | E1 plus `-cover` | 42 s | 0 to 551 MB; archive 82.2 MB | 602 |
| E | `go test` of 150 packages (all but `agentcore`, `textureowner`) with `-trimpath -p 3` | 503 s | not sized | 2 packages failed (`extract`, `deskprotocol-lock`), both pass after the `extract.go` fix; receiptsigner, hostboot and splitfixture users had failed in an earlier subset run and passed after their fixes |
| F0 | baseline: cold, edit 1, edit 2 (appended a func to `agentcore/api.go`) | 37 s, 10 s, 25 s | 538, +76, +76 MB | 587, 2, 2 |
| F1 | `-trimpath`: cold, edit 1, edit 2 | 40 s, 10 s, 9 s | 537, +76, +76 MB | 587, 2, 2 |
| F2 | `-trimpath` plus module `-dwarf=false`: cold, edit 1, edit 2 | 29 s, 8 s, 7 s | 461, +47, +47 MB | 587, 2, 2 |

Reading: `-trimpath` removes the per-worktree cost (B1, D3) and does nothing for
the per-edit cost (F0 equals F1). `-dwarf=false` on the module's packages cuts
the per-edit cost by 38% (76 to 47 MB), the cold cost by 14%, and was not slower.
Everything not compared here (a `GOOS=linux` pair, a full-suite cache size) I did
not run; the E row is a pass or fail check of `-trimpath`, not a size
measurement.

Shared-cache growth sample: 09:12 72.7 GB / 222,964 files; 09:30 identical (no
builders active); 09:39 72.9 GB / 223,117 files. Activity is bursty: the mtime
histogram has 5 GB/hour peaks (11-08 18:00) and idle hours.

## 3. Recommended architecture for concurrent agents

1. **One shared `GOCACHE`, path-independent keys.** Go's cache is safe for
   concurrent writers (atomic writes, mtime touch), and the standard library and
   dependencies (about 400 MB cold, 25 to 40 s) are paid once. Do not give each
   agent its own cache: it loses that sharing and does not bound the cache. Set
   `-trimpath` so identical sources in different worktrees share entries.
2. **Bound the cache by use, not by age alone.** Run `scripts/go-cache trim`
   hourly (section 4). Reuse beyond one hour is rare, so a 12 hour window keeps
   the useful working set (about 3 GB) and drops the rest. At the observed growth
   this holds the cache near 20 to 30 GB instead of 180 GB.
3. **Cut per-edit bytes.** `-dwarf=false` for module packages (-38% per edit). For
   builders: run `go test <changed package> -run <test>`; do not follow each edit
   with `go build ./...` and `go vet ./...` (`go test` already runs a vet subset);
   leave `-race` to CI or the agents VM, where its 91 MB archives do not land on
   the laptop's cache. The broad runs belong to `scripts/go-test-runtime-shards`
   and `~/ci-mirror.sh` on `choir-agents`.
4. **The structural lever (inferred, not measured).** `agentcore` at 174k lines
   makes every edit store about 76 MB (test variant) plus 37 MB (plain) plus the
   dependents. Splitting it by package boundary would shrink the unit that
   changes; a design question, outside this task.
5. **Considered, not adopted.** A per-worktree cache that is cloned from a seed
   (`cp -c` on APFS) and deleted with the worktree would bound garbage by
   worktree lifetime; not measured here and it duplicates the trim job's goal
   with more moving parts.

## 4. Implemented fixes (uncommitted in this worktree)

| File | Change |
|---|---|
| `.envrc` | Appends `-trimpath` and `-gcflags=github.com/yusefmosiah/go-choir/...=-dwarf=false` to `GOFLAGS` (idempotent, keeps existing flags; checked for empty, `-mod=mod` and already-set `GOFLAGS`). |
| `scripts/go-cache` (new) | `report` (size, age and size profile, largest entries) and `trim [--dry-run]` (age window, then size-triggered halving, never below a 150 minute floor). Defaults: `MAX_GB=30`, `TARGET_GB=20`, `MAX_AGE_HOURS=12`, `MIN_AGE_MIN=150`. |
| `internal/receiptsigner/state_migration_test.go`, `state_projection_test.go`, `internal/hostboot/hostboot_test.go`, `internal/store/splitfixture/splitfixture.go` | `runtime.Caller(0)` replaced by `os.Getwd()` (or a walk up to `go.mod`). Under `-trimpath` `runtime.Caller` returns a module-relative path, which broke these tests (observed). |
| `internal/deskprotocol/extract/extract.go` | When `build.Default.GOROOT` is empty (a `-trimpath` binary has no compiled-in GOROOT), take it from `go env GOROOT`. Without it `cmd/deskprotocol-lock` and `extract` tests failed (observed). |

Not done: `flake.nix` (an edit forces a nix eval of both systems; `.envrc` does
the same job without it). Not run under `-trimpath`: `internal/agentcore` and
`internal/textureowner` tests (CI shards them; `agentcore` has no
`runtime.Caller` use). The `.envrc` change needs `direnv allow` in each worktree.
I could not confirm that a builder's shell loads `.envrc` (my session's
`GOFLAGS` was empty and `.envrc` did not yet set it); check with
`go env GOFLAGS` in a fresh builder session after integration, and fall back to
the approval item below if it is empty.

CI and the Nix build do not read `.envrc` and are unchanged.

## 5. Validation

| Fix | Before | After | Source |
|---|---|---|---|
| `-trimpath`, second worktree same commit, build | +162 MB, 114 compiles, 8 s | +1 MB, 0 compiles, 1 s | B1 vs C2 |
| same, `agentcore` test build | +206 MB, 119 compiles, 24 s | +1 MB, 0 compiles, 3 s | D3 vs E2 |
| `-dwarf=false` on module packages, per edit of `agentcore` | +76 MB, 10 s | +47 MB, 8 s | F0 vs F2 |
| same, cold test build | 538 MB, 37 s | 461 MB, 29 s | F0 vs F2 |
| `-trimpath` plus the source fixes, 150 packages | 4 packages failing in the subset runs (`receiptsigner`, `hostboot`, `extract`, `deskprotocol-lock`) | all pass (re-ran the 4 affected packages plus `store`, `vmmanager`, `computerversion`, `objectgraph`, `cmd/store-split-diff`) | E, `d6`, `d9` |
| `go build ./cmd/...`, `go vet`, 8 test packages with the full `GOFLAGS` | n/a | pass, build 28 s cold | `h.sh` |
| `scripts/go-cache trim` | 71.2 GB shared cache | dry run: a 12 hour window would delete 55.0 GB (145,756 files); not run on the shared cache | report |
| trim on a copy | 745 MB | 152 MB, then `go build` succeeded and recompiled 580 packages | G1 |
| `-d` files deleted while their `-a` entries stayed | n/a | `go build` succeeded, recompiled 580 | G3 |
| trim running every 0.3 s with a 1 minute floor against a warm cache, 3 test builds | n/a | all three builds passed, no missing-file errors | `stress.sh` |

Projection (arithmetic on observed shares, not a measurement): with `-trimpath`
(removes up to 14.2 of 68.1 GB) and `-dwarf=false` (x0.62 on the rest), module
bytes fall from 68 to about 33 GB over the same work, roughly -47%. The trim job
then caps the total regardless of workload.

## 6. Runbook

Inspect growth:

```sh
scripts/go-cache report                       # size, unused-for buckets, largest entries
du -sk "$(go env GOCACHE)"; find "$(go env GOCACHE)" -type f | wc -l   # log hourly to get GB/day
```

Diagnose repeated misses (same sources, still recompiling):

```sh
go build -x ./pkg 2>&1 | grep -c '/compile '   # 0 means all hits
# diff the action-ID inputs of the same package in two worktrees:
GODEBUG=gocachehash=1 go build ./pkg 2>&1 | grep 'HASH\[build <import path>\]' > /tmp/a.txt
#   (repeat in the other worktree, then diff). A `dir /...` line means no -trimpath;
#   `file x.go <hash>` lines show an edited or generated file; the flags line shows -race/-cover/-gcflags.
```

Reclaim space safely while builds run:

```sh
scripts/go-cache trim --dry-run               # what it would do
scripts/go-cache trim                         # only acts above MAX_GB; env overrides in the header
MAX_GB=0 MAX_AGE_HOURS=12 scripts/go-cache trim   # force a 12 hour window now
```

Why mtime trimming is safe (verified in this toolchain's source and by test):
Go touches an entry's mtime on use (at most hourly), looks an entry up by its
`-a` index file, and then stats the `-d` output; a missing `-a` is a miss
(`entryNotFoundError`), and `GetFile` also treats a missing or wrong-size `-d` as
a miss. Deleting `-a` or `-d` files independently therefore costs a rebuild and
nothing else (G1, G3). The only unsafe case is deleting an entry a running build
already resolved; entries used by a running build were touched within the last
hour, so the 150 minute floor never selects them. Do not use `go clean -cache`
while builders run (it removes entries a running build just resolved).

## 7. Approvals needed (not done)

1. **Hourly trim job (machine-wide).** Proposed launchd agent
   `~/Library/LaunchAgents/com.choir.go-cache-trim.plist`: `StartInterval` 3600,
   `Nice` 15, `LowPriorityIO` true, `ProgramArguments`
   `/bin/bash /Users/wiz/Dev/go-choir/scripts/go-cache trim /Users/wiz/Dev/.caches/go-build`
   (after this branch is merged to the main checkout). Thresholds come from
   section 1: a 12 hour window (96% of big entries are never reused after one
   hour), `MAX_GB=30`, `TARGET_GB=20`, floor 150 minutes. First run would free
   about 55 GB.
2. **One-time reclaim now.** `MAX_GB=0 scripts/go-cache trim` on the shared cache
   (about 55 GB) while the other session's builders run is safe per section 6, but
   is a deletion in the shared cache, so I did not do it.
3. **Fallback for `GOFLAGS`.** If builder shells do not load `.envrc`:
   `go env -w GOFLAGS='-trimpath -gcflags=github.com/yusefmosiah/go-choir/...=-dwarf=false'`
   (edits `~/Library/Application Support/go/env`; an exported `GOFLAGS` overrides it).
4. **Policy acceptance.** `-dwarf=false` trades debugger source info for 38% less
   per-edit cache; reverse it by deleting the second `case` block in `.envrc` if
   `dlv` is ever needed for module code.
