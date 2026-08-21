# Staging Evidence: Shell Job-Control Persists After sh-Preference Repair

- Date: 2026-08-21
- Mutation class: Red
- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Guest realization epoch: 355 (refresh receipt for idempotency key
  `boot-sweep-restore-refresh-dc266292-2026-08-21`; refresh POST returned 502
  during vmctl restart, epoch advance observed via product status)
- Guest build: `dc266292d05d2ad87930de8f97e06a14cd7a1613` (deployed
  `2026-08-21T08:19:22Z`, CI `32460378498`)
- CoSuper: `run:assignment-12b0d40e-0d1f-5e8f-a13e-06ab2eb78463`
- Provider: `chatgpt/gpt-5.6-luna`

## Observation

After the broker `sh`-preference repair (`967010b9`) was deployed and the
retained computer rebooted onto the new guest image, a fresh assigned CoSuper
still failed:

```text
capsule_exec failed before command launch with initialize_job_control: getpgrp failed
capsule_write_file returned permission denied for /workspace/platform
```

## Deployment identity verification

The fix is provably in the guest closure:

- Node B host store contains two distinct capsule-broker builds:
  - `/nix/store/cg29hwsgkl6dqgv1hshdnym46ygh3caq-capsule-broker-0.1.0`
    with embedded `build.json` commit `dc266292…` (contains the repair)
  - `/nix/store/12jh1dpykb0bj7w250i0prj66yp0zhqr-capsule-broker-0.1.0`
    with commit `6b13cef1…` (pre-repair)
- The guest store disk (`/var/lib/go-choir/guest/storedisk.erofs`, itself
  built from commit `dc266292…` per `/var/lib/go-choir/guest/build.json`)
  references exactly the `cg29hwsg…` (repaired) build.
- The guest autoputer reported build `dc266292…` before the failing run.

So the failure is not a stale-binary problem.

## Working diagnosis

On the NixOS guest, `/bin/sh` is a symlink to bash. `exec.LookPath("sh")`
therefore resolves to bash, and bash still runs `initialize_job_control` at
startup inside the PID namespace, failing on `getpgrp`. Preferring `sh` by
name does not select a non-bash shell when bash *is* `/bin/sh`.

The accompanying `permission denied` on writes is consistent with prior
recurrence behavior: once exec fails, the CoSuper's fallback write attempts
also fail (or the same broken-shell path poisons them); it should be rechecked
after the shell repair actually selects a non-bash interpreter.

## Required next repair direction

One of, in preference order:

1. Put a genuine non-bash POSIX shell (dash or busybox sh) into the capsule
   PATH ahead of bash, and keep the broker's `sh` preference.
2. Or resolve the broker's shell explicitly to that non-bash binary.
3. Or suppress job-control initialization at exec setup in a way proven not
   to run bash's startup path (earlier `Setpgid:false` and `+m` attempts did
   not).

Effects remain OFF; no candidate artifact, bundle, proposal, promotion, or
live state write exists. Rollback: revert through origin/main + CI/deploy;
checkpoint `99949fe2` remains untouched.
