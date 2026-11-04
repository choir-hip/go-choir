# gVisor for desk workers and capsules, measured on x86 (2026-11-04)

Status: evidence. Measured by a Sonnet investigation agent on `choir-agents`
(Intel Xeon E-2236, 8 vCPU, NixOS kernel 6.18, nested KVM guest), unprivileged,
`runsc release-20261102.0 --rootless --network=none --platform=systrap`. The
desk worker is the real `cmd/desk-worker` from origin/main spawned through
`SpawnDeskSessionWorker`. Nothing touched Node B or any user computer.
Settles records R0-01 to R0-07 of
`docs/definitions/choir-modular-architecture-2026-11-04.precommit.md`.

## Earlier pass (same host, same day)

Warm desk cell 0.52 ms native, about 0.9 ms systrap. yaegi fib(27) +20%.
Allocation loop 2.1x. getpid 6x, open+read+close 9x, mmap 1 MB 10x. Verb IPC
22 to 26 us native, 60 to 64 us systrap. Sandbox start about 30 ms. KVM
platform unavailable in Choir guests (Node B sets `kvm_intel nested=0`).
WebAssembly (yaegi compiled to wasip1 under wazero, macOS arm64) measured 45x
slower warm cells and was dropped by the owner.

## Memory (PSS, MiB)

| Case | Native | One sandbox per worker |
|---|---|---|
| 1 worker, idle | 25.7 | 70 |
| 1 worker, after 100 cells | 42 | 78 to 87 |
| 8 workers, after 100 cells | 206 | 342 |
| Marginal per extra worker, after 100 cells | 23 | 37 |

Fixed cost of a sandbox: about 49 MiB (sentry about 37, launcher about 8,
gofer about 7). Shared sandbox (trust-class option), 8 workers after 100
cells: 299 MiB, about 27.6 MiB per extra worker.

## Layering

- runsc outer, existing confinement inside: uid switch, `CLONE_NEWNET`/
  `CLONE_NEWIPC`, no_new_privs and the cell seccomp filter all work and pass
  the self-test probes. Landlock is not implemented by gVisor (ENOSYS), so
  required mode would refuse to start; the sandbox's read-only rootfs, tmpfs
  `/tmp` and `--network=none` cover what Landlock granted.
- runsc inside Landlock: impossible (Landlock denies the `mount` runsc needs).
  Inside the cell seccomp filter: only with about 180 added syscalls, which
  protects almost nothing.

## Broker

The session socket inherited as fd 0 works with no `--host-uds`; only fds 0
to 2 pass, so a shared sandbox needs an in-sandbox supervisor to fan out one
socket. Raw round trip p50: native socketpair 9.7 us, runsc inherited fd 28.7
us. A real `read_capture` from a cell: 72.5 us native, 203.6 us sandboxed.
Warm cell p50: 1.0 ms native, 1.7 ms sandboxed.

## Cold start

Native spawn to first cell 10 to 27 ms; runsc about 73 ms (about 35 to 40 ms of
it runsc boot, the rest the test's bundle generator). Further workers inside a
running sandbox spawn in 20 to 25 ms.

## Capsule workload (traefik/yaegi, pure Go, `--overlay2=root:memory`)

| Operation | Native | runsc systrap | Ratio |
|---|---|---|---|
| `go build` cold | 11.1 s | 16.3 to 17.1 s | about 1.5 |
| `go build` warm | 0.1 s | 0.45 s | about 4.5 |
| `go test` cold | 2.8 s | 5.1 s | about 1.8 |
| `go test` warm | 0.44 s | 0.92 to 1.05 s | about 2.2 |
| `git status` warm, 4,995 files | 18 ms | about 187 ms | about 10 |
| `git commit`, small change | 12 ms | about 120 ms | about 10 |

Go and git ran unmodified (cgo, node and ptrace tools not tested). Peak PSS
during the build was the same as native. gVisor's own overlay and namespaces
replace the capsule's overlay and namespace setup, so that part needs no
CAP_SYS_ADMIN in the daemon (host side needs only an unprivileged user
namespace); open: cgroup limits (rootless runsc skips cgroups), CAP_NET_ADMIN
for the egress forwarder, and extracting results from the memory overlay.

## Agent's recommendation

Desk workers: one sandbox per trust class (about 49 MiB fixed plus about 28 MiB
per worker), runsc outer with the uid switch, seccomp and no_new_privs inside;
required mode is redefined as gVisor-confined. Capsules: a pilot.
