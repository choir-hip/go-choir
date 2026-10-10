# The first live L2 capsule cannot run a shell, rename across directories, or hold a package install (2026-10-10)

Status: documented, not fixed. Mutation class of the fix: `red` (capsule
isolation: seccomp allowlist, Landlock rights, capsule filesystem layout).

## Evidence

arXiv demo on staging at e1bcb393 (test computer demo-arxiv-2026-10-10c,
vm-82cf0bb7…; all-agents traces in the session scratchpad under
traces/arxiv5). This is the first time a management cast reached a bound
engineering run on staging (`run:delegated-sha256:b66e551d…`, bound at
21:14:33, capsule `capsule-f22e6c74…`, slot `implementation`). L2 works:
the guest console log shows `capsule-egress:` records for CONNECT to
pypi.org, files.pythonhosted.org and arxiv.org, with megabytes down.

Then every route the engineering desk tried for "install numpy and scipy"
failed for a reason the desk cannot fix from inside the capsule:

1. **No shell.** `bash -lc …` and `sh -c …` exit 1 with
   `bash: initialize_job_control: getpgrp failed`. So does `pip3`, which is a
   bash wrapper script in the Nix closure. Reproduced in Docker with
   bash 5.3 and a seccomp profile that refuses only `getpgid`/`getpgrp`:
   the same message, exit 1, for both `bash -c` and `bash -lc`.
2. **No cross-directory rename.** `uv pip install` downloads, then fails
   renaming a temp directory into its cache: `Invalid cross-device link (os
   error 18)`, the same under `/tmp`, `/var/tmp`, with `--no-cache` and with
   `--link-mode copy`. `python3 -m venv` fails in `ensurepip` (it unpacks the
   same way).
3. **No room.** `python3 -m pip install --target /var/tmp/pylibs numpy scipy`
   fails with `[Errno 28] No space left on device`.
4. **No disk usage query.** `os.statvfs` on every path fails `[Errno 1]
   Operation not permitted`, so the desk could not even measure (3).
5. **Go's telemetry child cannot start.** `go version` prints `can't start
   telemetry child process: fork/exec …/go: operation not permitted`.

## Causes (from source)

1. `capsuleAllowedSyscalls` (`internal/capsule/seccomp.go`) is default-deny
   with EPERM and does not list `getpgid`, `getpgrp`, `getsid` or `setsid`.
   Bash 5.3 exits when it cannot read its process group, even
   non-interactively.
2. `LandlockRestrictor.Apply` (`internal/capsule/landlock.go`) uses the V5
   ABI but grants no `AccessFSRefer` on any path. Landlock always handles
   REFER from ABI 2 on, and denies every rename or link that changes a
   file's parent directory with EXDEV unless both sides grant it. Package
   managers rename temp dirs into place, so all of them fail. The
   directories also lack `MakeFifo` and `MakeSock` (named pipes and Unix
   sockets in `/tmp`).
3. The capsule's writable layers are RAM: the overlay upper lives under
   `CHOIR_CAPSULE_STATE_DIR=/run/choir/capsules` (guest tmpfs), and
   `prepareCapsuleRoot` masks the capsule's `/tmp` with a 256 MiB tmpfs. Both
   are charged to the guest's 8 GiB RAM, and tmpfs writes count against the
   capsule's 1 GiB memory limit. A pip install builds in `/tmp` before moving
   into place, so a 50 MB download plus the earlier failed attempts fills
   the 256 MiB `/tmp`. The data disk (32 GiB) holds none of it.
4. `statfs`/`fstatfs` are not in the allowlist.
5. `setsid` is not in the allowlist; Go's telemetry starts its child in a
   new session.

## Also seen in the same trace (desk behavior, not capsule)

- Management cast engineering at 21:14:18 with an objective that depends on
  a paper research had not yet chosen ("once Texture/research fixes a
  candidate paper…"). Engineering spent its first three minutes grepping the
  platform source tree for the paper. Hypothesis, not yet confirmed by later
  traces: the management prompt does not say that a cast should carry the
  inputs it needs, or wait for them.
- Kernel cell friction: a cell that starts with an `import (…)` block of not
  yet installed packages and continues with statements fails with
  `expected declaration`, so the desk paid an extra cell to install imports
  alone. A top-level `sh := func(…)` that redefines `sh` with a different
  type fails `cannot use type … in assignment`, because the normalizer
  turns a fully bound `:=` into `=` (my change earlier today, K4).

## Belief state

Before: a bound L2 capsule could install packages from the allowlisted
ecosystems. After: L2 egress works, but the capsule's process and
filesystem contract refuses ordinary build tools. The ledger and traces
show the desk tried six routes and was refused by the substrate each
time. This was not a model failure.

Remaining error field: whether a disk-backed writable layer can fill the
guest data disk and starve the computer (fate-sharing), and how large the
`/tmp` mask should be once it is not RAM.
