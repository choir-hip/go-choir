# S0-2: guest serial console + journal unreachable — no host-side capture

## Symptom

After any guest cold boot the guest's stderr and journal are host-unreadable.
The layered-apply marker `go-choir-autoputer: layering release` and the boot
sequence go nowhere the host can read; the only evidence was a one-off
`layering-diag.log` inside the guest's `/mnt/persistent` data image (durable
but not a stream). See docs/problems/guest-stderr-unreachable-after-cold-boot-2026-10-04.md.

## Root cause (source-traced)

The guest kernel is told `console=ttyS0` and the autoputer service uses
`StandardOutput/StandardError=journal+console`, but the host launch path never
captures the serial output:

- `internal/vmmanager/manager.go:1621-1641` `buildFirecrackerConfig` emits only
  boot-source/drives/machine-config/network-interfaces — no `logger`, no
  `log_fifo`, no `serial_out_path`, no metrics. Guest `console=ttyS0` output is
  written to Firecracker's own stdout.
- `internal/vmmanager/manager.go:1737` sets `cmd.Stdout = os.Stdout` — so all
  VMs' serial output mixes into vmctl's shared journald, unindexed per-VM.
- The guest module (`nix/autoputer-vm.nix`) has no journald persistence
  (`Storage=`, `/var/log/journal`) and no host-readable journal endpoint.
- The historical `console-b14-*.log` files under vm-state/ are external
  one-off captures (no repo writer), unbounded (one is 104 MiB) and 5 weeks
  stale — not a product stream.

Firecracker 1.15.1 accepts `serial_out_path` only through its API socket;
`--no-api` (the launch mode here) skips it. So the correct capture is the
child's stdout, retained per-VM rather than shared.

## Fix landed

`cmd.Stdout` is now a bounded per-VM rotating writer at
`<StateDir>/<vmID>/console.log` (1 MiB active + 4 rotated generations = 5 MiB
cap), opened in the single shared `launchFirecracker` path so cold boot,
resume, recover, and refresh all capture. `stderr` stays on the host journal.
Rotation is tested; unbounded growth is impossible by construction.

## Residual

- `ReattachVM` (vmctl restart) does not relaunch Firecracker, so its stdout
  fd is orphaned: serial capture stops until the next launch seam. Reading a
  reattached VM's console file shows only the last launch's tail — acceptable
  (the data is still there), but a live reattached VM emits nothing new until
  a stop/boot.
- The guest journald inside the guest is not independently host-readable;
  console.log is the serial stream, not a structured journald export. The
  layering-diag.log workaround remains for the guest's own structured marks.

## Mutation class

Orange (VM launch behavior). must_preserve held: boot args unchanged, no
guest-visible change, host-side sink only.
