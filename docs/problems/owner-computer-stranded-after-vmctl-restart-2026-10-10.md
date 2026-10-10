# The owner's computer is stranded: alive, unanswering, protected, and impossible to restart (2026-10-10)

**Status:** open. Documented before the fix.
**Mutation class of this record:** green. The fix is `red` (owner lifecycle
restart, desktop recovery).
**Evidence class:** host-side process, network and vmctl log facts only.
Nothing inside the owner's guest was read.

## Evidence

The owner's computer is `computer-03335285269bdba4f94377e56879f9e6`, VM
`candidate-fleet-e15cb89f25d963c220319b7b`, epoch 1187.

- At about 11:10Z the guest answered self-development reads. At 11:22:54Z,
  the first vmctl restart of the d41c2caf deploy, reattach failed its
  health check. Every reattach since then has failed the same way. vmctl's
  load demoted the ownership to `stopped`, `stopped_by: vmctl-restart`.
- The Firecracker process (started 07:05:37Z) is alive and was never
  killed. The destruction receipts have no entry for this VM. Its tap
  device `vm-lackej5pmie2` is up with 10.200.34.1/30. Firecracker's tun
  descriptor names that tap.
- The guest does not answer ARP: the neighbor entry is FAILED. Tap TX
  counters rise with each ping; RX counters do not move.
- Two of the four vCPU threads have run at 100% since about 11:22–11:30Z.
  vcpu0 has 1650 s of CPU, vcpu1 1513 s, and vcpu2 and vcpu3 about 70 s
  each over the whole boot. The host RSS is 3.3 GB of the 8 GB guest.
- Every two minutes the warmness policy tries to resume the always-on
  desktop. The resolve guard (cfc17e05) refuses each attempt with
  `guest_reattach_pending`, "retry shortly". Reconcile logs "protected VM
  (premium_always_on) runs outside the registry; reattach pending, never
  reaped".

## What this means

The computer is wedged, and no path can end it:

1. **Automatic paths refuse, by design.** Phase 0 makes uncertainty never
   destroy a computer, and the reattach guard and reconcile protection
   carry that out. Nothing bounds the protection, so it holds forever.
2. **The owner's restart cannot end it either.** The proxy's `restart`
   action, given an ownership that reads `stopped`, only resolves, and
   resolve hits the same refusal. Stop is the path that ends a process
   vmctl lost track of (`reapUnmanagedLocked`, with a receipt), and restart
   skips it.
3. **The one path that would end it is not a restart.** The desktop
   called cold-recover automatically after three failed bootstrap probes
   (removed in the Phase 0 step 2 commit, 4eb1ec55, which made it a click).
   Cold-recover stops the computer, quarantines its data volume and
   rematerializes it from the tape. That is a rebuild, offered under the
   label "Restart computer".

## Cause of the wedge: two hypotheses, neither confirmed

- **Console pipe without a reader.** vmmanager gives Firecracker an
  `io.Writer` as stdout, so Go creates a pipe and copies it into the
  console log from inside vmctl. Firecracker's fd 1 is that pipe, and no
  other process holds it. Its reader died with the vmctl that launched the
  guest at 07:05. A guest kernel writing to ttyS0 into a pipe nobody
  reads can spin in the serial console path. Against this: the pipe has
  had no reader since at least the 09:26 restart, while the guest stayed
  healthy until about 11:10–11:22.
- **Load inside the guest.** Two pegged vCPUs and no ARP answer also fit
  guest memory or CPU thrash.

Telling these apart needs the guest's console or kernel log, which is the
owner's data. Both stay hypotheses. The console pipe is a fate-sharing
defect either way: Phase 1's systemd template units give each guest its
own stdout sink.

## Fix shape

1. The owner's restart always stops before it starts. Stop ends a process
   vmctl lost track of and writes a destruction receipt, cause `stop` or
   `reap-unmanaged`. The owner's explicit restart is the positive,
   attributable, authorized reason the doctrine requires. The stop names
   its caller, `proxy.lifecycle.restart`.
2. The boot console's "Restart computer" calls that plain restart, not
   cold-recover.
3. The owner's computer is not touched by an operator. Once the fix
   deploys, the owner's "Restart computer" in the boot console ends the
   wedged guest and boots epoch 1188. The crash-boot closer (9399f3ed)
   then closes the August self-development zombies.

**Correction, 12:10Z (owner report).** The owner opened choir.news and saw
"Your computer cannot start right now. This page will keep checking." The
page load resolves the computer, gets the same `guest_reattach_pending`
refusal, and the proxy serves its small self-reloading page
(`computer_surface_page.go`). The desktop never loads, so its boot console
and its Restart button never appear. Step 2 of this fix shape could not
reach the owner. That page must offer the restart itself. An operator
restart from this session was refused by the session's permission policy
(remote state change), so the restart stays with the owner.

Bounded protection, step 4 of Phase 0, remains open. Until it lands, the
owner's restart is the only bound on a stranded computer.
