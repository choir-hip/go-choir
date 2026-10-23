# The Jailed Firecracker Cannot Exec: The Nix Build Is Dynamically Linked

Date: 2026-10-23, 23:07 UTC. Found by FP0 on staging during SEC-04 step 2,
the first jailed launch on Node B (D25, e30c8b51a). Mutation class of any
fix: red (vmctl, VM lifecycle, tenant isolation). Parent records:
[firecracker-runs-as-root-without-jailer-2026-10-14.md](firecracker-runs-as-root-without-jailer-2026-10-14.md),
[jailer-copies-store-image-per-vm-2026-10-21.md](jailer-copies-store-image-per-vm-2026-10-21.md).

## Evidence (Node B, read-only after the failed launch)

- vmctl started at 23:06:36 with `jailer=true` and logged `jailer ready:
  bin=/nix/store/jbzvr6c9…-firecracker-1.17.0/bin/jailer`. The selector
  names one computer only: fp8-jailer-1023, computer-476cd794…, minted at
  22:56 for this step.
- 23:07:42: an operator stop of that computer (caller
  `operator.fp0-sec04-step2`), then a resolve. vmctl admitted the launch
  (class test), gave it VMM uid 900757, built the shared reflink copies of
  vmlinux, initrd and storedisk.erofs (RES-05 works), and created the tap.
- The jailer then failed inside the jail:
  `Failed to exec into Firecracker: No such file or directory (os error 2)`.
  vmctl killed the jailed pid (`cause=jailer-exec-unverified`) and the
  resolve failed: `jailed pid 758978 did not become Firecracker`.
- The jail root holds the copied `firecracker` (5,223,032 bytes, owner
  900757, 0555), `fc-config.json`, `firecracker.pid`, the hard-linked
  `data.img` and `credential.img`, and `dev/`.
- `readelf -l` on both pinned binaries:
  `Requesting program interpreter: /nix/store/h4wfwic1…-glibc-2.44-25/lib/ld-linux-x86-64.so.2`,
  NEEDED `libgcc_s.so.1` and `libc.so.6`. Neither the loader nor the
  libraries exist inside the chroot, so `execve` of the copied binary
  returns ENOENT for the interpreter.

## Why it was not caught

- The only real jailer run, `TestRealJailerLaunch` (10-14), used the
  official Firecracker and jailer v1.15.1 release binaries. Those are
  statically linked musl builds. Node B has always used the nixpkgs build:
  1.16 before D8, and 1.17.0 from source since D8. The nixpkgs build links
  dynamically against the store's glibc.
- `prepareJailerHost` checks that the jailer binary is executable, and it
  checks the uid range, the jail dirs, the cgroups and the no-IP rule. It
  does not check that the exec file can run inside a chroot. So vmctl
  logged `jailer ready` for a configuration that can never launch.
- The readiness plan named this risk in general ("the 1.17 jailer has not
  been run jailed"), but nobody checked the linkage.

## Scope and state now

- Only the selected test computer is affected. Every other computer,
  including the paper, launches direct as before (`Only` is non-empty).
- The test computer stays down with its jail in place, as evidence. It is a
  disposable test computer and nobody uses it.
- Rollback, if wanted: `VMCTL_JAILER=off` and a deploy. The computer cold
  boots direct at its next launch.

## Fix directions

1. **A static Firecracker for jailed launches** (preferred). Build the same
   pinned 1.17.0 source as a static musl binary (`pkgsStatic`, from the
   same `mkChoirFirecracker` pin). Upstream's supported release target is
   `x86_64-unknown-linux-musl`. Point the jailer's `--exec-file` at it. The
   direct launch binary stays unchanged, so no snapshot is voided. Open
   question: whether a jailed computer's snapshot taken with the musl build
   is accepted by a direct wake with the glibc build, and the reverse.
   Today a jailed snapshot is refused on a direct launch anyway (the
   computer cold boots), so this changes nothing for rollback.
2. **The upstream static release binary**, pinned by hash. It is faster to
   ship, but it is an external artifact rather than a source build.
   Fallback if 1 does not build.
3. **Put the glibc closure in the jail** (bind or hard-link the loader and
   libraries). Rejected: upstream requires a static binary, and this widens
   what the jail exposes.

In any direction, add a startup check: `prepareJailerHost` refuses (`JAILER
UNAVAILABLE`) when the exec file has a `PT_INTERP` program header. Then a
dynamic binary is caught at vmctl start, not at the first launch. Under
that check a selected computer falls back to a direct launch, as for any
other unavailability. The test for the check is written first.

## Residuals

- The real jailer integration test should run against the binary Node B
  deploys, not the upstream release.
- SEC-04 steps 2 to 4 wait on this fix.
