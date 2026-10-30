# The Desk Boundary's Remaining Gaps: Taint, Frame Protocol, Memory, Capsule Seccomp, Guest Hardening

Date: 2026-10-21. Found while building batch B3 of the launch first pass
([FP2 goal file](../definitions/choir-launch-fp2-desk-boundary-2026-10-21.md)).
Read from code at `origin/main` f399a2e8c; nothing was run on a guest. Status:
documented; the fixes follow in the lane. Mutation class of the fixes: **red**
(guest desk confinement, the worker frame protocol, capsule seccomp, the guest
image). Items are those of the
[security, density and architecture checklist](../security-density-architecture-checklist-2026-10-21.md).
The two larger records for this batch already exist:
[desk-frame-actions-not-role-checked-by-host](desk-boundary-residuals-2026-10-21.md)
(SEC-02) and
[desk-cells-run-as-guest-root-and-egress-is-open](desk-boundary-residuals-2026-10-21.md)
(SEC-01).

## The one cause

A desk's authority is written in six files that nobody reads together:
`internal/agentprofile/agentprofile.go` (message targets),
`internal/capsule/roles.go` (broker verbs), `internal/yaegikernel/choir.go`
(handle actions in `NewChoirScope`, verbs in `deskModuleSets`),
`internal/agentcore/inbox_records.go` (`recordsRoute`),
`internal/agentcore/tool_profiles.go` (`deskCarrierLive`), and, on the
unlanded branch, `DeskCellUser`. Each gap below is a consequence: a check that
needs a desk's authority has no single place to read it.

## Gaps, with the code that shows each

**ARCH-07 · the frame protocol has no version and no sender binding.** The
header is 8 bytes: length, stream, three reserved bytes
(`internal/yaegikernel/transport.go:14-23`, `frameHeaderLen`). Reserved bytes
are written as zero and never checked, so a peer can put data in them and a
future change cannot tell an old frame from a new one. Nothing binds a frame to
the worker the daemon spawned: the daemon's only binding is that the frame
arrived on that worker's socket, which stops holding the day a worker's
transport is not an inherited descriptor (vsock, TLS).

**ARCH-05 · taint classes exist nowhere.** Research reads untrusted web
content; Mail's input is labelled `UNTRUSTED_EXTERNAL_EMAIL` in `maild` only
(`internal/maild/ingest.go:121,176`). The worker pool separates desks by
accident: `deskSessionKey` gives one session per agent id and
`sameDeskSessionConfig` compares role, computer and root
(`internal/agentcore/tools_desk.go:179,348`). Nothing says a research worker may
never share a process, a unix user or a pool with Texture or management, and
every worker runs as the same guest root. No document records how research
output reaches Texture and management (records and reports, never a function
call or shared memory).

**RES-02 / RES-01 · memory is a virtual address cap in a guest the size of
the cap.** `deskWorkerMemoryLimitBytes` is 8 GiB `RLIMIT_AS` in an 8 GiB guest
(`tools_desk.go:36-42`); `gomemlimit` gives a soft 512 MiB per worker and the
unit has `OOMPolicy=continue` and no memory settings
(`nix/autoputer-vm.nix:929-941`). On 2026-10-16 two runaway cells filled the
guest and the kernel killed the daemon
([record](../archive/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md));
the only mitigation since is `oom_score_adj` 1000 on the worker
(`desk_worker_memlimit_linux.go`), which orders victims and bounds nothing.
Capsules sit in a root-level `/sys/fs/cgroup/capsule/<id>`
(`internal/capsule/namespace.go:168`), beside systemd's tree and under no
aggregate.

**RES-03 · the capsule's `exec-go-session` worker has no memory bound.**
`cmd/capsule-broker/main.go:153-162` builds its `SessionWorkerConfig` without
`MemoryLimitBytes`, and the capsule broker applies no `GOMEMLIMIT`; only the
capsule's cgroup bounds it, so a runaway cell is a runaway for the whole
capsule's `memory.max`.

**RES-04 · capsule memory differs from doctrine.** Doctrine
(`docs/choir-doctrine.md:406-408`; `docs/runtime-invariants.md:139-141`):
`memory.high` is the request, `memory.max` twice that, admission counts the
request, `memory.events` is feedback. Code: `CreateCgroup` sets `Max` only
(`internal/capsule/namespace.go:36-40`); admission reserves the whole
`MemoryMax` against 3/4 of MemTotal
(`internal/capsule/job_memory_linux.go:37-43`); an engineering capsule
requests 2 GiB (`engineeringAssignmentMemoryMax`,
`internal/agentcore/engineering_assignment_runtime.go:33`), measured against the
`cmd/autoputer` link at 1.50 GiB stripped and 1.80 GiB with DWARF. No code
reads `memory.events`. A `memory.high` below the link's anonymous peak would
throttle the build with no swap to reclaim into (the 1 GiB budget thrashed the
drill for 14 minutes), so the values must come from measurement, not from the
formula alone.

**SEC-13 · capsule seccomp allows `clone3` and every `clone` flag.**
`internal/capsule/seccomp.go:28` lists `clone` and `clone3` unconditionally in
`capsuleAllowedSyscalls`, which every workload, broker and delegate filter
inherits. `clone3` takes its flags in a struct seccomp cannot inspect, so
allowing it makes any `CLONE_NEW*` flag unconditional; `clone` with
`CLONE_NEWUSER` lets a workload create a nested user namespace and regain
capabilities inside it, the usual first step of a kernel-bug exploit.

**SEC-15 · the guest runtime is root and runs document tools unsandboxed.**
The unit has no `User=` (`nix/autoputer-vm.nix:810-941`).
`runDocumentToolOnTempFile` runs `pdftotext`, `pdfinfo` and `pandoc` on
untrusted bytes as guest root, with the guest's network and no limits
(`internal/content/extract.go:215-251`; callers `extract.go:108,114,138,158`,
`pdf_blocks.go:20,33`). The guest image sets no sysctls (`nix/autoputer-vm.nix`
has no `boot.kernel.sysctl`; the only one in `nix/` is the host's
`vm.swappiness`). Obscura is FP6's to delete.

## Failure modes any fix must close

See the FP2 goal file, "Failure modes, written before code" (manifest M1-M5,
frames F1-F9, confinement C1-C3 plus the 2026-10-14 record's nine, memory
R1-R5).

## Fix direction (designed in the goal file)

One manifest (`internal/deskmanifest`) that every check reads; a versioned,
slot-bound frame; the recovered cell confinement with its profile and user
read from the manifest; a cgroup tree with a hard budget per worker; capsule
`memory.high`/`memory.max` mechanism with values from measurement; `clone3`
answering `ENOSYS` and `CLONE_NEWUSER` refused; guest sysctls and a sandboxed
document-tool child. Not in this batch: making the guest daemon non-root.

## Status by item (lane `lane/b3-desk-boundary`, not deployed)

- ARCH-07: wire version byte 1 and zero reserved bytes enforced in
  `FramedConn`; a per-spawn slot token in the ready handshake and on every
  host-bound request. Done on the lane.
- ARCH-05: taint class per manifest row; no shared uid, pool or process across
  classes (tests); the data-flow record is in the FP2 goal file. Done on the lane.
- RES-02 and the desks half of RES-01: a cgroup per worker with a hard
  memory.max from the manifest, an aggregate desks cgroup, `MemoryMin` for the
  daemon. Done on the lane; real values come from the staging probe.
- RES-03: the capsule session worker volunteers for the OOM kill and holds a
  soft and an address-space bound. Done on the lane.
- SEC-13: clone3 ENOSYS and no CLONE_NEWUSER in every capsule filter. Done on
  the lane.
- SEC-15: guest sysctls done; the non-root daemon and the document-tool sandbox
  are open (designs in the FP2 goal file).
- RES-04 and the capsules aggregate of RES-01: open; the mechanism needs
  `memory.events` from real builds to set values (a `memory.high` below the
  link's anonymous peak throttles the build).

## Second batch (lane `lane/b3-desk-boundary-2`), 2026-10-21

Designs and failure modes are in the FP2 goal file, "Second batch". Two
findings from reading the code again for it:

- **SEC-15's non-root daemon is not one slice.** The capsule executor (mounts,
  user and network namespaces, cgroups, overlay roots, the broker's ambient
  capabilities, `internal/capsule/executor.go:611-667`) is called in process
  from 55 sites in `internal/agentcore` and `internal/autoputer`; desk cell
  spawn switches to the cell uids (`desk_cell_confinement_linux.go:63`) and
  the desks cgroup is written by the daemon. A non-root daemon needs all of
  these behind a privileged helper first. This batch lands the capability
  floor under it (the daemon keeps 16 named capabilities, CAP_SYS_TIME among
  them since review round 2, and drops the rest from its bounding set) and the document-tool sandbox; the split stays a
  named residual.
- **The daemon's bounding set is full.** It runs with every capability
  (no `CapabilityBoundingSet` in the unit, `nix/autoputer-vm.nix`, unlike
  `go-choir-updater`'s empty set), so a compromised daemon or any child can
  load a kernel module, use raw I/O on the guest's block devices, or set the
  clock. Guest code needs none of these but the clock: the hibernate resume
  steps it (`internal/agentcore/hibernate_clock_linux.go`; review round 2,
  E1, docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md).

Status after the second batch (lane `lane/fp2-b2-on-main`, not deployed):

- RES-04 and the capsules aggregate of RES-01: the mechanism is built and
  ships in observe. Capsules keep their old shape. Each capsule's
  memory.peak and memory.events are recorded with what doctrine would have
  done, and the doctrine policy is one switch away once the receipts show
  builds fit (d36a1b78d).
- SEC-15, document tools: sandboxed as uid 61010 in an empty network
  namespace under Landlock V5 and seccomp, in observe (911da25ad; the
  observe shadow in a cgroup of its own since a40b6db09).
- SEC-15, the daemon: a capability floor (16 kept, CAP_SYS_TIME among
  them since b1cee0a10; the rest dropped from the bounding set), in
  observe (e2acf6025). Still root: the split is open.


## Merged from docs/problems/desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md (2026-10-14)

**Desk Cells Run as Guest Root, and Guest Egress Is Open.**
Date: 2026-10-14. Found by the computer isolation security review ([docs/archive/reports/computer-isolation-security-review-2026-10-14.md](../archive/reports/computer-isolation-security-review-2026-10-14.md), findings F4 and F6). This record re-reads the code on `origin/main` (980ffaf4e) and adds the guest egress inventory the fix needs. Status: documented; not fixed on main. The fix was local to the owner's iMac until 2026-10-21, w

(Full text in git history at the path above.)


## Merged from docs/problems/desk-frame-actions-not-role-checked-by-host-2026-10-21.md (2026-10-21)

**The Daemon Answers a Desk Worker's Frames Without Checking the Desk's Role.**
Date: 2026-10-21. Found while auditing desk isolation for the [security, density and architecture checklist](../security-density-architecture-checklist-2026-10-21.md) (item SEC-02). Read from code at `origin/main` 8f71f9c64; nothing was run. Status: documented; no fix yet. Mutation class of any fix: **red** (desk authority boundary, product API reachability from model code).
- Status: documented; no fix yet. Mutation class of any fix: **red** (desk

(Full text in git history at the path above.)
