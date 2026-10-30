# S3 Snapshot Hibernation on Staging, First Proof

Date: 2026-10-14, 06:04-06:10 UTC. Deploy 5e4b6fa7 (CI green; 231 of 231
health probes 200; vmctl restarted 06:03:36 with
`VM_HIBERNATE_MODE=snapshot`, `VM_SNAPSHOT_BUDGET_GIB=48`,
`VM_BALLOON_FREE_PAGE_REPORTING=1`). Design:
docs/archive/reports/s3-snapshot-hibernate-design-2026-10-14.md. Probe:
`scripts/s3_snapshot_probe.py`; receipt
`~/.config/choir-qa/traces/s3/proof-1.json`.

Computer `computer-488fd5fce9e67c2a97be747eb6acfa0a`, VM
`vm-db4bb41c1822bc850b0fbc104efeba58`, 8 GiB, 4 vCPU, build 5e4b6fa7,
launched with its API socket (`api/fc.sock`, root only).

| step | result |
|---|---|
| cold boot (first launch) | healthy at 9,661 ms |
| host RSS before hibernate | 1,223,532 KiB (1.17 GiB) |
| hibernate (vmctl operator act) | 6,008 ms total: guest fence 9 ms, pause under 1 ms, full snapshot 5,542 ms, Firecracker stopped |
| snapshot files | `mem` 8 GiB, `vmstate` 88 KiB, `meta.json`; files 0600, directory 0700 |
| host process while hibernated | gone; guest RAM freed |
| wake (owner start act, 06:09) | lifecycle start answered in 237 ms |
| wake timeline | `boot_kind=snapshot_resume`, healthy: validated 1 ms, consumed 4 ms, Firecracker spawned 46 ms, snapshot loaded 72 ms, guest resumed 90 ms, **first healthy 103 ms** |
| clock | stepped forward 246,913 ms |
| state | the computer setting written before hibernating reads back unchanged |
| obligations | 0 closed as "interrupted by a restart" |
| host RSS after wake | 372,376 KiB (364 MiB: 52 MiB anon, 312 MiB file): guest memory pages in lazily from the snapshot |
| snapshot files after wake | deleted (one-shot) |

Wake to healthy is 103 ms against 9.7 s cold, about 95 times faster, with
the open work kept instead of closed.

## Found

- **An operator hibernate does not wake on a request.** The probe's first
  wake was an ordinary owner request, which got 502/503 for about 100 s.
  vmctl records `stopped_by: operator`, which, like a stop, waits for an
  explicit start; the owner start act woke it. Whether a reclaim
  hibernate (idle or pressure) wakes on a request through the snapshot
  path is still to be shown.
- **The probe's "process gone" check matched its own shell** (`pgrep -f`
  over ssh). vmctl's log shows the Firecracker process killed after the
  snapshot; the check needs to read `/proc/<pid>/comm`.
- **Disk:** the memory file is deleted after the load, but Firecracker
  keeps it mapped, so its blocks stay allocated until that process exits.
  The budget's accounting should count mapped, deleted snapshots
  (residual).
- **Resume window:** this snapshot's `resume_deadline` was 42 minutes
  after the hibernate (capability expiry). The credential-window follow-up
  (branch s3-credential-window) lifts it to the 12 h maximum age.

## Proof 2: past the credential window (deploy 9ace75e5)

Computer `computer-ddedef8828cf03daa058231f4cd7560f` (VM
`vm-e09dc6b3aeeb26e36672fc8a56f59bde`), build 9ace75e5, receipt
`~/.config/choir-qa/traces/s3/proof-2-window.json`, all checks pass.

| step | result |
|---|---|
| hibernate 06:38:05 | 6,139 ms snapshot; guest fence 9 ms; Firecracker gone (checked by process name) |
| held hibernated | 55 minutes; the old `resume_deadline` (07:20:05) passed while hibernated |
| wake 07:33 (owner start act) | `snapshot_resume`, healthy: validated 2 ms, **credential issued 14 ms**, consumed 17 ms, loaded 92 ms, guest resumed 376 ms (clock stepped 3,308 s), **first healthy 396 ms** |
| credential | `delivered`: a fresh envelope for the same realization, exchanged by the guest |
| state | the setting read back unchanged |
| obligations | 0 closed as "interrupted by a restart" |
| host RSS | 1.17 GiB before hibernate; 299 MiB just after the wake |

Wake to healthy past the old window: 396 ms. The resume limit is now the
12-hour maximum snapshot age (owner's call whether to lengthen it).
