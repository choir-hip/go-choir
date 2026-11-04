# gVisor in the arm64 Mac guest kernel, measured (MA0.8, 2026-11-04)

Status: evidence. Slice MA0.8 of the modular architecture mission (plan
`docs/design/modular-architecture-plan-2026-11-04.md` section 3; rulings R39 and
R40). Companion to `docs/evidence/gvisor-desk-capsule-x86-2026-11-04.md` (x86,
same method). Measured by a Sonnet investigation agent on the owner's Mac.
Nothing touched a Choir computer, Node B or the main checkout.

## Verdict

gVisor `runsc --rootless --platform=systrap` runs the real `cmd/desk-worker`
under the Choir aarch64 guest kernel on Apple Virtualization.framework. No
failure was met on the way that was not a harness mistake. R40's fallback
(refuse non-first-party desks on the Mac) is not needed on the evidence of this
slice. Costs differ from x86 in three ways the plan should carry:

- Syscall-bound work is slower in the sandbox than on x86 (getpid 14.6x against
  6x; open+read+close 15x against 9x). CPU-bound yaegi and allocation work is
  nearly free (fib(27) +11%, allocation loop +5%).
- Warm cell p50 is 0.48 ms against 0.10 ms native for a trivial cell (4.9x), and
  a verb round trip is 0.60 ms against 0.21 ms (2.9x). Absolute times are
  small.
- Memory: one idle sandbox costs about 78 MiB over native (sentry 61, gofer 33,
  launcher 4.5; x86 sentry about 37, gofer about 7). Eight sandboxes cost 49 MiB
  more each than native workers, which matches the x86 figure (49 MiB) only at
  fleet scale. After heavy cells one sandboxed worker reached 325 MiB PSS
  against 95 MiB native (see Memory; one run, cause not investigated).

Not proven here, and so still open for MA6.4: the full NixOS guest userspace,
the root-spawner uid switch inside the sandbox, `required` confinement mode,
cgroup budgets, and a hostile-cell probe.

## Environment identity

- Host: Apple M1, 16 GiB, macOS 27.2 (Darwin 27.2.0), vfkit 0.6.3 (Apple
  Virtualization.framework), 2 vCPU, 2048 MiB guest RAM, `virtio-rng`, one
  `virtio-serial` console to a file. No nested virtualization requested; the
  KVM platform was not tried (systrap is the ruled platform).
- Guest kernel: the repo's own aarch64 guest kernel,
  `nix build .#packages.aarch64-linux.guest-kernel` at origin/main `2bae9cbdd`
  (`/nix/store/gahfcs4n2npz3jdip40p0n85wmy0g44j-linux-6.18.57`, `Image`, 71,178,752
  bytes, sha256 prefix 853893d6af862a6a), booted by vfkit's Linux bootloader
  exactly as `nix/guest-aarch64.nix` assumes (uncompressed `Image`, `console=hvc0`,
  `lsm=landlock,yama,bpf` as the earlier L0 boot used). `uname`: `Linux 6.18.57
  #1-NixOS SMP PREEMPT aarch64`. Config facts read from the guest and the built
  config: `USER_NS`, `PID_NS`, `NET_NS`, `IPC_NS`, `UTS_NS`, `SECCOMP_FILTER`
  built in, `OVERLAY_FS=m` (not needed by runsc), LSMs
  `lockdown,capability,landlock,yama,bpf,ima`, `user.max_user_namespaces`
  7888, `yama.ptrace_scope` 1, no `unprivileged_userns_clone` knob, CPU
  features include `atomics asimd sha3 paca pacg`.
- Userland is NOT the Choir guest image. The earlier L0 boot of the full image
  (3.4 GiB base erofs, systemd) has no way in for a command without credentials or
  a network login, and rebuilding it was out of the hour. Instead the same
  kernel booted a minimal initramfs: static busybox (Alpine
  `busybox-static`), the gVisor tarball, a `linux/arm64` build of the repo's
  `cmd/desk-worker` and the harness. `/init` mounts a tmpfs and
  `switch_root`s into it (runsc's `pivot_root` fails from a bare initramfs
  with "make sure that the root mount has a parent"; the product guest's root
  is already a tmpfs, so this is a harness artifact, not a finding). The
  kernel, the hypervisor, the CPU and the Go binaries are the product's; systemd,
  the nix store layers and the services are absent. runsc and the worker are
  static binaries, so userland libc does not change their behaviour.
- gVisor: `release-20261102.0` aarch64, `gvisor.tar.bz2` from
  `storage.googleapis.com/gvisor/releases/release/20261102.0/aarch64/`, sha512
  `574b5775b4417618...f762fd19980`, verified with `shasum -a 512 -c` (OK). The release now
  ships a tarball (`runsc`, `gvisor-bin/gvisor_sentry`, `gofer` helpers), not a
  bare `runsc` at `.../latest/aarch64/runsc` (that URL, and its `.sha512`, return
  404 today). Both `runsc` and `gvisor-bin/` must sit together.
- Worker: `cmd/desk-worker` from origin/main `2bae9cbdd`, role `research`, spawned
  through the real `yaegikernel.SpawnDeskSessionWorker`, so the real fd 3
  session socket, slot token, frame protocol and `EvalCell` are exercised.
  Spawner runs as uid 1000 (non-root), confinement mode auto (no_new_privs,
  seccomp, Landlock where available). Native worker: Landlock available
  and applied by the kernel's LSM list; sandboxed worker: started and served
  cells (Landlock is ENOSYS in gVisor, as on x86).

## Method

A scratch harness (`ma08bench`, not committed; about 250 lines of Go) links
`internal/yaegikernel`. It has three modes. `bench` spawns the worker natively
(`Bin = desk-worker`) or sandboxed (`Bin = ma08`, `WorkerArg = wrap`). `wrap`
is the launcher: it rewrites the worker environment (`SOCK_FD=0`,
`ALLOWED_ROOT=/work`), writes a minimal OCI `config.json` (read-only rootfs
holding `/desk-worker`, tmpfs `/tmp` and `/work`, pid/ipc/uts/mount namespaces,
`noNewPrivileges`), `dup3`s the inherited fd 3 onto fd 0 and execs `runsc
--rootless --network=none --platform=systrap --ignore-cgroups run`. Only fds 0
to 2 cross into the sandbox, as on x86. `sysbench` runs inside the sandbox
and outside it.

Per mode: spawn one worker, time `SpawnDeskSessionWorker` and the first cell;
300 warm cells of each kind (wall time at the host around `EvalCell`: trivial
`fmt.Println(1+1)`, fib(27) as a closure, a 20,000 iteration allocation loop,
and a verb cell `choir.SearchWireCorpus` answered by an in-process `Egress`
handler); PSS from `/proc/<pid>/smaps_rollup` (native: the worker; runsc: every
process of the sandbox, classified by cmdline into sentry, gofer and
launcher); eight workers each given 100 mixed cells, one sandbox per worker;
five cold spawn plus first cell. Single run per number, 2 vCPU, so the
percentiles are noisy at p99.

## Numbers

Cell time, warm, 300 cells each (ms):

| Cell | Native p50 | runsc p50 | Ratio | Native p99 | runsc p99 |
|---|---|---|---|---|---|
| trivial | 0.098 | 0.477 | 4.9x | 0.87 | 1.92 |
| verb round trip (`SearchWireCorpus`) | 0.211 | 0.601 | 2.9x | 3.38 | 2.40 |
| fib(27) in yaegi | 538 | 596 | 1.11x | 998 | 708 |
| allocation loop | 5.47 | 5.74 | 1.05x | 17.3 | 21.4 |

Syscalls (us per op, Go loops; inside the sandbox it is the same binary):

| Syscall | Native | runsc | Ratio | x86 ratio |
|---|---|---|---|---|
| getpid | 0.12 | 1.75 | 14.6x | 6x |
| open+read+close | 0.56 | 8.41 | 15x | 9x |
| mmap 1 MiB + touch + munmap | 3.6 | 19.4 | 5.4x | 10x |

Spawn: first spawn native 10.5 ms, runsc 58.4 ms. Five cold spawns plus first
cell: native p50 6.0 ms, runsc p50 32.5 ms (30 to 39 ms; includes bundle
generation and the exec of the launcher). x86 recorded runsc 73 ms.

Memory (PSS, MiB, from `smaps_rollup`):

| Case | Native | One sandbox per worker |
|---|---|---|
| 1 worker, idle (after first cell) | 21.0 | 98.9 (sentry 61.4, gofer 33.0, launcher 4.5) |
| 1 worker, after 300 cells of each kind (fib heavy) | 94.8 | 325.3 (sentry 275.0, gofer 45.8, launcher 4.5) |
| 8 workers, after 100 mixed cells each | 139.4 | 532.2 (sentry 421.6, gofer 95.6, launcher 15.0) |
| Per worker, 8-worker case | 17.4 | 66.5 |
| Sandbox cost over native, per worker, 8-worker case | | 49.1 |
| Sandbox cost over native, idle single worker | | 77.9 |

The 8-worker figure of 49.1 MiB per sandbox equals the x86 fixed cost (about 49
MiB); the idle single sandbox is larger (78 MiB), mostly a larger gofer (33 MiB
against about 7 on x86) and sentry (61 against 37) at first boot. The 300-cell
single-worker case ended at 325 MiB PSS against 95 native. That run did 300
fib(27) cells, which grow and collect the Go heap; the sentry kept about 275 MiB
where the native process kept about 95 MiB. This was one run and not
investigated (hypothesis: the heap the worker freed is not returned to the
host promptly inside the sentry; not tested with `GOMEMLIMIT`, which the real
spawner sets from the cgroup budget, and the harness had no cgroup). The x86
doc's 100-cell figures (42 native, 78 to 87 sandboxed) do not show it. Treat
heavy-cell resident size as an open item for MA6.4's memory budget, not a
result.

## Failures met

1. `runsc` run fails with `urpc method "containerManager.SetNetworkArgs"
   failed: connection reset by peer: the sandbox exited before the container
   started`. The sandbox log named it: `FATAL ERROR: error setting up chroot:
   pivot_root failed, make sure that the root mount has a parent: invalid
   argument`. Cause: PID 1 was in a bare initramfs (ramfs has no parent mount).
   Fixed by `switch_root` onto a tmpfs. Fixable by configuration, harness
   only; the product guest's root is a tmpfs.
2. `crypto/rand: blocked for 60 seconds waiting to read random data` in the
   native worker: no entropy source on the first VM config. Fixed with
   `--device virtio-rng`. The guest image must have its rng seeded as it is
   today (not an arm64 finding).
3. `runsc` download: `.../latest/aarch64/runsc` and its `.sha512` are 404;
   release assets are now `gvisor.tar.bz2` and `.zstd` with `.sha512`.
   MA6.4's guest image build must fetch or build that tarball.
4. Harness mistakes, not findings: `syscall.Dup2` does not exist on arm64 (use
   `Dup3`); `AllowedRoot` must exist on the host; Go yaegi rejects top-level
   `func fib` in a cell (a closure works).

Nothing in the run is fundamental: systrap works on this kernel and hypervisor,
rootless user namespaces work as an unprivileged uid, and no kernel feature
was missing for runsc.

## Verdict for R40's fallback

- The fallback (refuse non-first-party desks on the Mac guest, run first-party
  desks under process confinement) is not triggered by this slice. gVisor is
  available under Apple Virtualization.framework with the guest kernel as
  built.
- The accepted overhead ratio for the Mac should be stated separately: warm
  cells 2.9x to 4.9x on tiny cells, near 1.1x on CPU-bound ones; syscall
  heavy work (file, mmap) 5x to 15x; memory per sandbox 49 MiB at scale, 78 MiB
  for a lone idle one. Both are small in absolute terms on this M1.
- Before the Mac result counts as settled for MA6.4, rerun in the real guest
  image once it is bootable with a command channel: root spawner with the uid
  switch, `required` mode, the cgroup memory budget with `GOMEMLIMIT`, and the
  heavy-cell resident size above. A capsule workload (go build, git) was not
  measured on arm64.

## Cleanup

The VM was a vfkit process per boot, each ended by the guest `poweroff -f` or
by `timeout`; none is running. Artifacts (initramfs, console logs, harness
source) are in the agent's scratchpad, not the repo. The kernel image came from
the Nix store (`guest-kernel`), already cached.
