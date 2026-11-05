# MA6.2 and MA6.3: worker and broker hardening (2026-11-05)

Class red, tier 2 (MA6.2); MA6.3 touches the same trust boundary. Rollback: revert.
Plan row: docs/design/modular-architecture-plan-2026-11-04.md, MA6.2 and MA6.3.

## Audit of the MA6.2 row against desk_cell_confinement_linux.go

| Item | Before | After |
| --- | --- | --- |
| Explicit seccomp denials | The filter is deny by default (allow list), so io_uring, keyctl, bpf, perf, userfaultfd and ptrace were denied implicitly; nothing named them, and a later addition to the allow list would have opened one silently | `deskCellDeniedSyscalls` names them (EPERM); the profile test fails if a denied name is also allowed; `clone` is admitted only with CLONE_THREAD and no CLONE_NEW* bit |
| Mount namespace without /proc, /sys, /dev | Absent (Landlock hid them by path only) | Spawner: CLONE_NEWNS and CAP_SYS_ADMIN as an ambient capability. Worker: `isolateCellMounts` builds a tmpfs root holding the allowed root (read-only), its tmp dir, /dev/null, /dev/urandom, the zone files and a reduced /proc (hidepid=2, subset=pid); pivot_root; unmount the old root; seal; then `dropAllCapabilities` on every thread. No /sys, no /dev tree, no /etc. **/proc is reduced, not removed** (residual below) |
| fd closure across spawn | Go's exec leaves only close-on-exec descriptors out; a descriptor inherited without FD_CLOEXEC from the daemon's parent would survive | `CloseInheritedFDs` at the top of the worker closes every non-CLOEXEC descriptor above fd 3 |
| rlimits | RLIMIT_AS only (and cgroup pids.max and memory.max) | plus CORE 0, NOFILE 256, FSIZE 256 MiB, MEMLOCK 0, MSGQUEUE 0 |
| Unique uid per activation | One uid per desk (siblings shared credentials) | A uid per live worker from a pool (200000 to 259999); the manifest uid still gates the spawn |
| Restart intensity | None: a crashing cell could be respawned without bound | 5 self-ended workers of a role in 60 s refuse the next spawn, naming the cause |

## Ways this could fail (written first)

1. The new deny group shadows an allowed call and the Go runtime dies (assembly test, helper cells).
2. A denied name sits in the allow list again (the profile test enumerates both).
3. The mount namespace step runs without the capability, or leaves capabilities on one thread (capabilities are per thread; the drop goes through psx and is read back).
4. A path the worker is handed no longer resolves after the pivot (the allowed root, the tmp dir and the device files are bound at their own paths).
5. A descriptor leaks across exec (a test leaks one and reads /proc/self/fd).
6. The pool hands one uid to two live workers (race test).
7. The crash ledger counts a host kill, or never ages out (fake clock test).
8. The self-test judge passes an unconfined worker (the mutation test stays).

## MA6.3 failure modes (written first)

1. A decode panics or allocates without bound on hostile bytes (fuzz BrokerRequest, the frame reader, the worker mux reader loop).
2. A frame handler panics on a hostile payload (fuzz each verb's payload through the real handler).
3. A verb acts with the daemon's authority on a cell-supplied reference (audit of FrameAllowed and the verb handlers).

## Residual: /proc is reduced, not removed

libpsx (used by go-landlock and by the capability drop to act on every OS thread in a cgo build, which the nix build is by default) lists `/proc/<pid>/task` after each call and fails the call when it cannot (go-landlock bug 39; the library adds a Landlock rule for that one directory for the same reason). A root without /proc therefore fails `Restrict`. The worker's /proc is mounted `hidepid=2,subset=pid`: no `/proc/sys`, `meminfo` or any other process's directory, and the cell's Landlock domain grants only `/proc/<pid>/task`. Removing it outright needs either a worker built with CGO_ENABLED=0 (psx then uses `syscall.AllThreadsSyscall`, no /proc) or a two-stage start (stage 1 holds the capability on one thread, applies Landlock and execs stage 2). Either is a build or spawn-shape change; named for MA6.4, where gVisor's filesystem view replaces Landlock and this question goes away.

Not testable here: the production shape (a root spawner, `AmbientCaps` through `Credential`) runs only as root; the unit tests build the same root in a user, mount and pid namespace and the root-only tests (`TestDeskCellWorkerRunsAsTheDeskUser`, `TestDeskCellSelfTestPassesTheProductionShape`) were updated but could not run on the dev host. The proof is the boot self-test read through `POST /api/runtime/isolation-selftest` (judged by `JudgeDeskCellSelfTest` and `scripts/desk_cell_confinement_probe.py`).
