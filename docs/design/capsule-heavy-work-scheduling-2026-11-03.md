# Heavy capsule work without a per-job grant (2026-11-03)

Status: design (an architectural design, reviewed like the change it
designs). Station S4 of
[`choir-selfdev-apps-feature-complete-2026-11-03.md`](../definitions/choir-selfdev-apps-feature-complete-2026-11-03.md).

Owner, 2026-11-03: "maybe we can figure out how to have many capsules
running as long as whatever heavy work, like front-end build, is scheduled
correctly"; then "i think we can do a better job of scheduling, perhaps
without roundtrip asking for a heavy usage grant ... whats the best
practices for this type of thing from systems and dist systems world?"

## What exists

- **Guest side.** Capsules are admitted against a memory request inside the
  guest's capsule budget; `memory.max` is twice the request
  (`internal/capsule/capsule_memory.go`). A job may take a memory boost if
  it fits the guest's live budget (`internal/capsule/job_memory_linux.go`).
- **Host side.** vmctl admits a computer's start against
  `MemAvailable >= floor + headroom + reserved + need`, asking idle guests
  to return cache and hibernating idle ones to make room
  (`internal/vmctl/host_admission*.go`). A running guest's growth inside its
  ceiling is not admitted by anyone.
- **The unwired seam.** `JobAdmission` (`internal/capsule/job_receipt.go:50`)
  was meant to lease host memory per job; `SetJobAdmission` has no caller.
- **Caches.** Go has `GOCACHE=/mnt/persistent/go-build-cache` per computer
  (`nix/autoputer-vm.nix:1130`); npm/Vite caches are per capsule; Chromium is
  a shared reflinked layer (selfdev-screens-as-files, e41f378b8).

Four concurrent arm builds can add about 12 GB of guest growth against 17 GB
available and a 4.7 GB vmctl floor (2026-11-03 00:46Z), and nothing refuses
it.

## Practice it follows

1. **CPU is compressible, memory is not** (Borg, Kubernetes). CPU contention
   slows everyone and breaks nothing: proportional weights, no scheduler.
   Memory contention kills something: it is reserved.
2. **Reserve at placement, not per job** (Kubernetes requests, Borg
   allocs). One admission when the capsule spawns, against a declared size
   class; the capsule's jobs spend that reservation locally.
3. **Backpressure from a shared pressure signal, not grants** (Linux PSI,
   as Meta's oomd and senpai and Android's lmkd use it; TCP congestion
   control). Each guest reads host pressure and defers new heavy work while
   it is high. No coordinator, graceful degradation.
4. **Do not repeat work** (Bazel remote cache, the Nix store, sccache).
   Outputs keyed by a digest of their inputs; most builds become a hit plus
   an increment.
5. **Priority classes and preemption** (Borg prod and batch). Protected
   computers are never squeezed; batch work waits or pauses.

## The design

1. **Size classes at spawn.** A capsule spawns `light` or `build`. The
   request travels with the spawn the guest already admits; `build` reserves
   the peak a frontend build plus Playwright measured, `light` the editing
   footprint. The guest's capsule budget is sized for one `build` capsule;
   vmctl already counts that ceiling when the computer starts. No job asks
   anyone.
2. **CPU by weight.** Capsule and job cgroups get `cpu.weight` (build jobs
   below interactive guest work); no CPU admission anywhere.
3. **Host pressure published, read locally.** vmctl writes the host's
   `/proc/pressure/memory` averages and MemAvailable-over-floor into each
   guest's read-only host-facts channel every few seconds. The capsule
   executor defers starting a `build`-class job while host pressure is over a
   threshold or headroom is under one build's peak, and starts it when it
   clears. A deferred job reports "waiting for host memory" in its status, so
   a stall is visible, never silent. The host floor stays the hard stop.
4. **A shared content-addressed build cache.** One host cache, read through
   by every capsule: npm and Vite dependency trees keyed by lockfile digest,
   built package outputs keyed by the digest of their sources and the
   toolchain, the Go build cache, the Playwright Chromium layer. Writes go
   through the host (a capsule never writes another computer's entries); a
   hit is verified by digest. This shares the capsule mirrors' host path
   (`choir_capsule_mirrors`), which the switch audit puts on with this work.
5. **Priority.** The paper and the owner's computer are `protected`;
   self-development arms are `batch`. Under host pressure, vmctl's existing
   make-room order takes batch computers first; a batch capsule's deferred
   job waits.
6. **Delete `JobAdmission`.** The per-job lease seam is superseded by 1
   and 3.

## How it scales

Every mechanism is local (weights, the pressure read) or content-addressed
(the cache): no coordinator and no round trip, so adding a computer adds no
scheduling load. Concurrency is bounded by idle footprint (about 0.8 GB an
arm) plus the build peaks the pressure signal admits; most of a round is
model calls, so many capsules overlap with two or three builds in flight.
The cache makes the second arm's build of the same tree a hit. Past one
host, the same facts channel and cache sit on whichever host runs the
computer.

## Proof

- Four arms start a frontend build at once on staging: host MemAvailable
  stays above the floor; at least one job reports waiting, then completes;
  no guest OOM; the paper's model-call rate unchanged.
- The second arm's build of an unchanged tree is a cache hit (time and
  bytes recorded).
- Pressure readings and deferrals in the growth meter beside the arms'
  round timings.

## Until it lands

At most two arms run a build at once, started by the orchestrator.

## Rollback

Revert. Size classes default to the current request; with no pressure facts
a guest starts jobs as today; the cache is read-through, so removing it only
loses the hits.
