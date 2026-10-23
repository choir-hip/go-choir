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

## Fix (lane/fp0-jailer-static-fc)

Direction 1, built from source. Mutation class red (vmctl, VM lifecycle).
Tests first: 0b72fb7d1 (red; stubs only, six tests fail), then the fix.

- **The jailed exec file.** `VM_JAILER_FIRECRACKER_BIN`
  (`JailerConfig.FirecrackerBinPath`; default `VM_FIRECRACKER_BIN`, which
  is the old behavior) is the jailer's `--exec-file`. `VM_FIRECRACKER_BIN`
  is unchanged, so direct launches keep the glibc build. Nix sets it to
  `pkgs.pkgsStatic.firecracker`. The `choirFirecrackerModule` overlay does
  reach `pkgsStatic`, so this is `mkChoirFirecracker` of the musl package
  set: the same 1.17.0 source and hashes. A new eval assertion requires the
  two versions to match. `scripts/check-host-firecracker-pin.sh` now checks
  the variable too, with three new cases in its fake-nix test.
- **Startup check.** `prepareJailerHost` now refuses the exec file before
  it touches the jail dirs, the cgroups or the no-IP rule. vmctl logs
  `JAILER UNAVAILABLE` and every launch goes direct. It refuses an exec
  file that is not ELF, or that has a `PT_INTERP` header or a `DT_NEEDED`
  entry. It also refuses one with a different file name from the direct
  binary, because the jail dir is named after it. Last, it refuses one
  whose `--version` banner differs from the direct binary's.
- **Snapshots.** `firecracker_version_mismatch` compares the
  `--version` banner, not a path or a hash. Snapshot metadata records the
  banner of the direct binary (`launchedVMM().Firecracker`). The startup
  version-equality check is what keeps a jailed snapshot's record true.
  Both builds print `Firecracker v1.17.0`, so no direct computer's snapshot
  is voided, and no launch-config hash moves.
- **NET-08 next build.** A computer on the next build jails with the next
  binary itself, but only when that binary is static
  (`FirecrackerNext.JailExec`, set at load). Otherwise it launches direct
  with the reason `next_build_not_static`. With `VM_FIRECRACKER_NEXT_BIN`
  unset, as it is today, nothing changes.
- **Built and verified.** The build ran on the agents VM with
  nix-portable, from the flake at 30acb645d. Its store path is
  `/nix/store/k1ipcp7mhmdydknpp205i9n5nf4avg7x-firecracker-static-x86_64-unknown-linux-musl-1.17.0`,
  the same path the Mac evaluates for this branch. Facts about the binary:
  - `bin/firecracker` is 5,381,784 bytes, sha256 `cdb21237…a597b`.
  - It is a static PIE. `readelf -h` gives Type DYN (PIE). `readelf -l`
    shows no INTERP. `readelf -d` shows zero NEEDED entries.
  - `--version` prints `Firecracker v1.17.0`.
  - Copied alone into an empty dir, it runs under
    `unshare -Ur chroot <dir> /firecracker --version` (exit 0). The glibc
    build fails at exactly this step in the jail.
  - The new `staticELFError` accepts it.
- **Real jailer test.** `TestRealJailerLaunch` now takes
  `CHOIR_JAILER_IT_JAILED_FIRECRACKER`, so the Node B run can use the
  binary Node B deploys (first residual above).

Not done here (FP0): the deploy, and the SEC-04 step 2 relaunch of
fp8-jailer-1023. Rollback: revert the fix commit, or `VMCTL_JAILER=off`.
