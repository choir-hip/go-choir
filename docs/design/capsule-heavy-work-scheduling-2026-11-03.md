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
   footprint. No job asks anyone. vmctl's deployed admission is measured
   (`VMCTL_ADMISSION_MODE=measured`, `NEED=wake`, nix/node-b.nix): a running
   computer is charged at its settled RSS, not its ceiling, so a capsule's
   growth is not already counted. The guest therefore reports its live
   `build` reservations to vmctl over the guest-facts channel, and measured
   admission charges each computer the larger of its settled RSS and its
   idle footprint plus live build reservations. A `build` spawn the host
   cannot cover is refused at spawn, visibly, rather than admitted and
   killed later: one report and answer per `build` spawn, never per job.
   For a running computer the live reservations join vmctl's shared
   `reserved` term, so the next admission anywhere sees them. A reported
   reservation is clamped to the guest's remaining ceiling, released when
   its capsule ends, and expires on guest death by the reservation
   ledger's existing in-flight TTL.
2. **CPU by weight.** Capsule and job cgroups get `cpu.weight` (build jobs
   below interactive guest work); no CPU admission anywhere.
3. **Host pressure published, read locally.** vmctl writes the host's
   `/proc/pressure/memory` averages and MemAvailable-over-floor into each
   guest's read-only host-facts channel every few seconds. The capsule
   executor defers starting a `build`-class job while host pressure is over a
   threshold or headroom is under one build's peak, and starts it when it
   clears. A deferred job reports "waiting for host memory" in its status, so
   a stall is visible, never silent. The host floor stays the hard stop.
4. **A build cache that cannot be poisoned across computers.** Capsule
   jobs run arbitrary code and input digests are computable, so no capsule
   output is ever served to another computer. Shared entries are only
   downloads the consumer verifies against a digest it computes from
   content it already trusts: npm tarballs against the lockfile's
   `integrity` hashes, Go modules against `go.sum` and the checksum
   database, the Playwright Chromium layer from the Nix store. The host
   fetches them (the capsule mirrors path, `choir_capsule_mirrors`, on at
   d@e since 0336c8bfe and widened with this work); a
   capsule never writes a shared entry. Built outputs (the Go build cache,
   Vite and package builds) stay per computer: hits across one computer's
   capsules are fine, and a poisoned entry harms only the computer that
   wrote it.
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
The shared download cache makes the second arm's install of the same lockfile a hit; build outputs hit within one computer. Past one
host, the same facts channel and cache sit on whichever host runs the
computer.

## Proof

- Four arms start a frontend build at once on staging: host MemAvailable
  stays above the floor; at least one job reports waiting, then completes;
  no guest OOM; the paper's model-call rate unchanged.
- The second arm's install of an unchanged lockfile is a download-cache
  hit (time and bytes recorded); a capsule on d@e that tries to write a
  shared entry is refused.
- With build reservations live, measured admission charges them and
  refuses a build spawn the host cannot cover.
- Pressure readings and deferrals in the growth meter beside the arms'
  round timings.

## Until it lands

At most two arms run a build at once, started by the orchestrator.

## Review

Devin, round 1 (2026-11-03): send_back with two blocking-real findings,
answered above. Deployed admission is measured, so vmctl does not count the
ceiling (point 1 now reports build reservations into measured admission).
A cross-computer cache written by capsules can be poisoned (point 4 now
shares only consumer-verified, host-fetched downloads and keeps build
outputs per computer). Round 2: accept (reservation clamp, lifecycle and
the running-computer charge folded in above).

## As built (2026-11-03, branch selfdev-s4-scheduling)

- **Size classes.** `capsule.SpawnSpec.SizeClass` (`internal/capsule/capsule_size.go`);
  engineering assignment capsules spawn `build`. `build` reserves 3 GiB
  (`BuildPeakBytes`) on the host; `light` reserves nothing, since its editing
  footprint is already in the computer's settled RSS. The guest-local
  admission and the cgroup limits are the same for both classes.
- **Reservations and facts share one route.** The guest posts its whole live
  build set to `POST /internal/vmctl/computers/capsule-reservations`, which is
  tap-bound like the effective-release report. It posts at a build spawn
  (with `admit`) and every 5 s while any build capsule is live. vmctl
  replies with the host facts. So "published to guests" is a reply to the
  guest's own report, not a separate host-written channel. With no live
  build the guest asks nothing, and no job then needs the facts. Code:
  `internal/vmctl/capsule_reservations.go`, `internal/autoputer/capsule_host_memory.go`.
- **The charge.** Per computer it is `min(ceiling - rss, max(0, settled + builds - rss))`.
  It goes into the same `reserved` term that starts and build spawns are
  admitted against. A set expires after the in-flight TTL without a
  report, and is dropped when the VM stops.
- **Refusal.** A build spawn the host cannot cover returns
  `capsule.HostMemoryRefusal`. The assignment waits instead of being
  cancelled (problem 24,
  docs/problems/build-refused-for-host-memory-cancels-assignment-2026-11-03.md):
  it stays open and unbound in the engineering admission queue, a ledger
  record addressed to management says "waiting for host memory: <the
  host's reason>", and its spawn wake retries every 30 to 60 s. The wait
  is bounded at 30 minutes from the first refusal (the record's time),
  matching the job deferral; a build still
  refused at the bound is cancelled with the host's reason, visible in the
  cast's fate. A crash boot closes a waiting request as interrupted; a
  planned boot resumes it once, like any queued request. Protected builds
  are always admitted, so they never wait. Any other spawn error still
  cancels the open. A refused build does not make room (make-room stays a
  start's). A vmctl that cannot be asked, or one that predates the route,
  leaves the spawn as before.
- **Deferral.** A build-class job reads the facts before it starts. It
  waits in the new job state `waiting` ("waiting for host memory: ...") in
  two cases: PSI memory some avg10 above 10, or avg60 above the reclaim
  threshold; or headroom (MemAvailable less the floor and what is promised
  to other computers) under 3 GiB. It starts when that clears. It ends
  stopped on its stop, at its capsule's end, or after 30 minutes of
  waiting, and a restart closes it as interrupted. Protected computers,
  light capsules and facts older than 30 s never wait.
- **Priority.** Protected (`premium_always_on`, `critical_protected`,
  `public_platform`) build spawns are always admitted and still counted.
  Every other class is batch. The existing make-room order already takes
  test computers (the arms) first and never takes protected ones.
- **CPU.** `cpu.weight` is 50 for a build capsule and 100 for a light one.
  The capsules aggregate is 50.
- **`JobAdmission`** is deleted, with the lease-share release it threaded
  through job boosts.
- **Not built here:** the shared download cache (point 4). It rides the
  capsule mirrors path that lane B turned on at d@e. Widening the mirrors
  and the "capsule cannot write a shared entry" check are that lane's.

## Rollback

Revert. Size classes default to the current request; with no pressure facts
a guest starts jobs as today; the cache is read-through, so removing it only
loses the hits.
