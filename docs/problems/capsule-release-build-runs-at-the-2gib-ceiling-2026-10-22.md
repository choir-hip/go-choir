# A release build in an engineering capsule runs at the 2 GiB ceiling and is OOM-killed (2026-10-22)

Found in the FP4 round 3 re-run on the test computer
`computer-71afbf1461dffabcce8eeebe558d03bd` (deploy Q, d206fdd40,
2026-10-22 13:38-14:01 UTC; evidence in
`docs/evidence/fp4-admission-queue-deployed-proof-2026-10-22.md`). This
commit only records the problem; there is no fix in it. Each cause is
marked confirmed in code, confirmed by measurement, or hypothesis.

## Evidence

- Capsule A, `capsule-3b63529a-9bc6-55e8-a589-e8decdcfe8cb`, ran "build the
  release as usual (bin/autoputer and the frontend)". Its teardown receipt
  (policy observe), guest log 14:01:27:
  `request=2048MiB admitted=2048MiB high=- max=2048MiB peak=2048MiB events
  high=0 max=385311 oom=73 oom_kill=2 doctrine_would_throttle=true`
  (peak 2,147,594,240 bytes).
- Three of A's `capsule_go_eval` cells hit the 30 s broker read timeout
  (13:44:16, 13:44:57, 13:58:54). Hypothesis: memory pressure (reclaim
  stalls) delayed the broker and the session worker, which live in the
  same cgroup. Not confirmed.
- Capsule B (cheap receipts, no build) peaked at 158 MB with no events. An
  earlier step 3 reading had peaks of about 650 MB and no events.
- A's build ran for about 20 minutes. Which argv the desk used (one
  `go build` or several, `-p`, whether the frontend build overlapped the Go
  build) is in the monitor's `trace-A-1402.json`, which this lane cannot
  read. Hypothesis until that trace is read: one `go build -o
  bin/autoputer ./cmd/autoputer` job and one `npm run build`, with no flags.

## What the build runs (read from code at 71ec8e704)

- **The command is the model's.** No code fixes the capsule's build argv:
  the engineering desk starts it as a capsule job or a cell `choir.Exec`
  (`internal/deskmanifest/manifest.go:258-259`), and the release-adoption
  prompt only says "build the release tree"
  (`internal/agentcore/release_adoption.go:354`). The host build
  (`flake.nix:196-205`, `buildGoModule`) is a different path and does not
  run in the capsule.
- **Environment (confirmed in code).** Jobs and direct exec get
  `PATH, HOME=/tmp, TMPDIR=/tmp, LANG` plus the toolchain carry
  (`cmd/capsule-broker/main.go:538-562`, `cmd/capsule-broker/jobs.go:48-61`);
  cells get the same from `cellExecEnv`
  (`internal/yaegikernel/broker.go:467-487`). The toolchain adds
  `CGO_ENABLED=1` and the ICU `CGO_*FLAGS`/`PKG_CONFIG_PATH`
  (`internal/yaegikernel/build_toolchain.go:59-87`). An egress capsule adds
  `GOFLAGS=-mod=mod` and `GOPROXY` (`internal/capsule/egress.go:44`).
  **Nothing sets `GOMAXPROCS`, `GOMEMLIMIT`, `GOGC` or `-p`**, and a job
  takes no caller variables (`jobs.go:44-47`), so a job's `go build` runs
  with Go's defaults unless its argv wraps it in `env`.
- **The capsule's limits (confirmed in code).** `memory.max` 2 GiB,
  `memory.swap.max` 0, `cpu.max 100000 100000` (one CPU), `pids.max` 256
  (`internal/agentcore/engineering_assignment_runtime.go:33-40,464-473`,
  `internal/capsule/namespace.go:31-60`).
- **Go cannot see the one-CPU limit, so the build runs four-wide (confirmed
  in code).** Go 1.25+ sets `GOMAXPROCS` from the cgroup's `cpu.max` only
  when `/proc/self/mountinfo` shows a `cgroup2` mount
  (`$GOROOT/src/internal/runtime/cgroup/cgroup.go:360`, go1.26.9). The
  capsule masks `/sys` with a tmpfs (`internal/capsule/executor.go:2271-2283`),
  so no cgroup2 mount is visible and `GOMAXPROCS` is the guest's CPU count:
  4 on Node B (`nix/node-b.nix:1395`, `VM_INTERACTIVE_CPU_COUNT=4`). `go
  build -p` defaults to `GOMAXPROCS` (`cmd/go/internal/cfg/cfg.go:87`), so
  up to four compiles (or cgo `cc1`/`cc1plus` runs) are resident at once,
  inside one CPU of quota. The extra width buys no throughput (the quota is
  one CPU) and multiplies resident memory.
- **The build is big (measured, 71ec8e704).** `go list -deps
  ./cmd/autoputer`: 1,420 packages, 1,124 outside the standard library;
  cgo in `runtime/cgo`, `net`, `os/user`, libcap `psx`,
  `dolthub/gozstd` (zstd C sources) and `dolthub/go-icu-regex/internal/icu`
  (ICU, C++). The link is external (cgo), so `g++`/`ld` run after the Go
  linker.
- **Every capsule compiles from a cold cache (confirmed in code).** With
  `HOME=/tmp` the build cache is `/tmp/.cache/go-build`, a fresh
  per-capsule directory on the state volume, removed with the capsule
  (`executor.go:2284-2301`). A one-line change rebuilds all 1,420 packages.
  The cache is disk-backed, but its pages are charged to the capsule's
  cgroup as page cache while they are written.
- **The link peak is close to the cap (measured earlier).** 1.50 GiB
  anonymous stripped, 1.80 GiB with DWARF
  (`engineering_assignment_runtime.go:27-33`;
  `docs/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md`).
  The broker, the session worker and the `go` command itself share the
  same 2 GiB.

## Local measurement (this lane, node-a agents VM, 71ec8e704)

A cold `go build -o … ./cmd/autoputer` in a systemd scope shaped like the
capsule (`MemoryMax=2G`, `MemorySwapMax=0`, `CPUQuota=100%`), fresh
`GOCACHE`, warm module cache, `GOMAXPROCS=4` (what the capsule's Go sees):

| run | wall | cgroup peak | events max / oom / oom_kill | largest process RSS | GOCACHE |
|---|---|---|---|---|---|
| GOMAXPROCS=4 | 8 m 45 s | 2,147,487,744 (at max) | 7,955 / 0 / 0 | 1.55 GiB | 1.4 GB |
| GOMAXPROCS=4, anon sampled every 0.5 s | 9 m 32 s | 2,147,487,744 (at max); **anon peak 1,359,892,480 (1.27 GiB)** at 362 s, in the compile phase | 1,957 / 0 / 0 | 1.44 GiB | 1.4 GB |
| GOMAXPROCS=1 GOMEMLIMIT=1280MiB (added after this record, for the fix) | **7 m 53 s** | 2,147,487,744 (at max); **anon peak 887,144,448 (846 MiB)** at 469 s, in the link | 3,570 / 0 / 0 | 1.50 GiB | 1.4 GB |

The cgroup totals sit at `memory.max` in every run because the build's
written cache is page cache charged to the cgroup; the anonymous peak is the
part that cannot be reclaimed without swap. The capsule's state volume is
disk (`CHOIR_CAPSULE_STATE_DIR=/mnt/persistent/capsule-state`,
`nix/autoputer-vm.nix:1025`), so `/tmp` and the cache are not tmpfs. The
runs shared the VM with other workers' tests, so wall times are noisy;
the anonymous peaks are not.

The local build reaches `memory.max` too, with no OOM kill: the build alone,
with nothing else in the cgroup, already lives at the ceiling. The guest
adds what this run lacks: the broker and the session worker in the same
cgroup, the frontend build if it overlaps, and model cells (`go vet`, a
second `go build`, `go_eval`) started while the build runs (the
2026-10-14 drill saw two builds at once after a kill hit the wrong pid).

## Causes

- Confirmed in code: the build runs `-p 4` inside a one-CPU, 2 GiB, no-swap
  cgroup, because the `/sys` mask hides `cpu.max` from Go.
- Confirmed in code: no `GOMEMLIMIT` reaches capsule workloads, so each Go
  tool grows its heap to `GOGC=100` (about twice its live heap) with no
  regard for the cgroup.
- Confirmed in code: the build cache is cold in every capsule.
- Confirmed by measurement: a capsule-shaped cold build alone sits at the
  2 GiB `memory.max`; the largest single process is 1.55 GiB.
- Hypothesis: A's two OOM kills hit a compiler or the linker when a second
  heavy process (a parallel compile, the frontend build, or a model cell)
  was resident with it. Read `trace-A-1402.json` for the job argv and start
  times to confirm.
- Hypothesis: the three 30 s broker read timeouts are reclaim stalls in the
  shared cgroup.

## Harm

- An OOM kill inside the capsule can hit the build (lost minutes), a model
  cell, or the broker or session worker, which ends the capsule's tools.
- Twenty minutes of one CPU per Go change, from a cold cache.
- Under the doctrine policy (`memory.high` = request = 2 GiB) the same build
  would throttle at `memory.high` instead (`doctrine_would_throttle=true`).

## Options (chosen in the fix commit, not here)

1. Bound the build inside the cap: give capsule workloads `GOMAXPROCS`
   equal to the capsule's CPU quota (so `-p` follows) and a `GOMEMLIMIT`
   below the cap, as defaults a cell may override.
2. Raise the cap for capsules that run release builds (3-4 GiB). The guest
   has 8 GiB; the capsule budget is 3/4 of MemTotal (6 GiB), shared with
   job boosts and, under doctrine, ceilings at twice the request. A 4 GiB
   capsule is two thirds of the capsule budget and half the guest, beside
   the daemon, Dolt and the desks; guest page cache does not return to the
   host.
3. Reuse a build cache across capsules (a seeded `GOCACHE` for the
   deployed commit). A shared writable cache lets one capsule poison
   another's builds, so it needs its own design (red).

## Fix (fd8337941, orange; tests 55e9ac1ae)

Option 1, keeping the 2 GiB cap. The executor gives each capsule broker
`CHOIR_CAPSULE_GOMAXPROCS` (the CPU quota in CPUs, rounded up: 1 for
engineering, 2 for a Claude capsule) and `CHOIR_CAPSULE_GOMEMLIMIT` (5/8 of
`memory.high` when doctrine set one, else of `memory.max`: 1280 MiB of
2 GiB). The broker carries them to the session worker; cells, jobs and
direct exec get them as `GOMAXPROCS` and `GOMEMLIMIT`, which a cell's
payload or `env` in a job's argv overrides. Option 2 is not needed by this
evidence; option 3 (a shared cache) stays a residual with its own design.

## Next probes

1. Done: the bounded shape measured (table above).
2. FP0: read A's job argv and start times from `trace-A-1402.json`.
3. Hypothesis to watch on the guest: dirty-page writeback to the guest's
   virtio-blk volume slows reclaim of the cache's pages, which a host
   without that disk does not show.
4. The deployed proof is an FP4 drill re-run on 71afbf14 whose
   `capsule_memory` receipt for the build capsule shows `oom_kill=0`, with
   the build job's wall time from its status.
