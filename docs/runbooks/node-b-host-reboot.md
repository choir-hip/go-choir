# Runbook: Node B host reboot (kernel update)

Date: 2026-10-14. Mutation class: **red**. A host reboot stops every
computer, the paper included. It changes the host kernel, which every
computer's isolation rests on.

Problem: [host-kernel-216-days-unpatched-2026-10-15.md](../problems/host-kernel-216-days-unpatched-2026-10-15.md).
Script: `scripts/node_b_host_reboot.py`. Dry run is its default.
Restart rules: CLAUDE.md, "Restarts End Work (Crash) Or Resume It
(Planned Update)".

## The next reboot: 6.18.55 to 6.18.57 (added 2026-10-22)

The first reboot below happened on 2026-10-15 at 10:49 UTC: Node B booted
6.18.55 and the go-choir configuration
([handoff-2026-10-15.md](../reports/handoff-2026-10-15.md)). That 6.18.55
generation is now a kernel known to boot on this host. The KF deploy stages
**6.18.57** ([kernel-pin-behind-guest-reachable-checksum-fix-2026-10-22.md](../problems/kernel-pin-behind-guest-reachable-checksum-fix-2026-10-22.md)),
and the next reboot changes the kernel (6.18.55 to 6.18.57) and the
booted system (26.11.20260701 to the current one). Stage 1, hostname and
the kvm parameters are already booted.

Don't trust a version written in this runbook. Read the expected kernel
from the deployed system on Node B:

```bash
ssh node-b 'ls /run/current-system/kernel-modules/lib/modules/'   # what uname -r will print after the reboot
ssh node-b 'uname -r'                                            # what is booted now
```

The script reads the same thing: the pre-checks record the current
system's kernel, and the post-checks require `uname -r` to equal it. The
version numbers below are the first reboot's and the KF deploy's; the
read wins wherever they differ.

## What the first reboot changed (2026-10-15)

This is the first cold boot of the go-choir host configuration. The host
has run the March choiros-rs system since 2026-03-12. Every go-choir
generation since then was only switched into. So the reboot changes much
more than the kernel:

| | booted now | after the reboot |
|---|---|---|
| kernel | 6.18.16 | 6.18.55 (`nixpkgs-kernel` input; 6.18.57 after the KF deploy) |
| stage 1 | scripted initrd | systemd initrd (`root=fstab`) |
| NixOS | 26.05.20260306 (choiros-b) | 26.11.20260701 (go-choir-b) |
| hostname | choiros-b | go-choir-b |
| `kvm_intel nested` | Y | N |
| `kvm.halt_poll_ns` | 200000 | 0 |
| SMT | on | on (owner decision pending) |

Lineage: the base (main nixpkgs `ec84054`) is a NixOS 26.11 pre-release,
a nixos-unstable snapshot frozen on 2026-07-01. The kernel comes from the
`nixpkgs-kernel` input, which is current nixos-unstable: the same lineage,
newer. Only the kernel is newer. Userspace stays on the 2026-07-01 base
until the patch pipeline's weekly lane bumps it, and that lane, not this
reboot, owns the full base bump.

The root and `/boot` arrays (md RAID1), fstab and mdadm.conf are the
same. **No GRUB entry is known to boot.** Every entry on Node B today
uses the never-booted 6.18.37 kernel and go-choir userspace. The boot
path has not run since March.

## Main risk: the console

If the host doesn't come back over ssh, it can only be recovered from the
OVH IPMI/KVM console (KVM-over-IP or Serial over LAN) or from OVH rescue
netboot. The board has a BMC (`/dev/ipmi0`, ASRock Rack E3C242D4U2-2T),
but nobody has recorded opening its console through OVH. **Do not reboot
until step 0 has been done in the same window.** The script refuses
`--execute` without `--console-verified`.

## 0. Before the window

1. **Console.** In the OVH manager, open this server and go to IPMI. Open
   the KVM (browser) or Serial-over-LAN console and confirm it shows the
   login prompt. Confirm Netboot → Rescue is offered. Write down how long
   each took to open.
2. **Deploy the kernel branch** through the normal Landing Loop: push to
   `main`, then CI, then the deploy. The deploy switches the host
   (staging the new kernel, 6.18.57 for the KF deploy, for the next boot)
   and rebuilds the guest image. The deploy refresh then moves idle
   computers onto the new guest kernel (busy ones defer). Check it:
   - on Node B: `ls /run/current-system/kernel-modules/lib/modules/`
     shows the staged release (6.18.57), and
     `grep nested /run/current-system/etc/modprobe.d/nixos.conf`
     shows nested=0;
   - a refreshed guest's kernel capability receipt shows
     `kernel_release` equal to the guest image's kernel (6.18.57).
3. **Dry run:** `scripts/node_b_host_reboot.py`. Every pre-check must
   pass. It prints the computers it would mark.
4. **Optional, owner decision: a known-good fallback entry.** The proven
   6.18.16 kernel and initrd stay on disk only while `/run/booted-system`
   roots them, which ends at the reboot. To keep them in the GRUB menu as
   a recovery foothold, run this immediately before the reboot:

   ```bash
   ssh node-b '
     set -e
     systemctl stop go-choir-nix-gc.timer        # its "+4 generations" prune runs every 4 h
     B=$(readlink -f /run/booted-system); C=$(readlink -f /run/current-system)
     nix-env -p /nix/var/nix/profiles/system --set "$B"   # generation N+1: March system, 6.18.16
     nix-env -p /nix/var/nix/profiles/system --set "$C"   # generation N+2: current again (the default)
     "$C/bin/switch-to-configuration" boot                # rewrite GRUB with both
     grep -c "linux-6.18.16" /boot/grub/grub.cfg'
   ```

   That entry boots the old choiros-rs userspace. It gives ssh access, not
   Choir service. From it, boot the go-choir generation again once the
   fault is fixed. The timer restarts at boot, and the deploy after next
   prunes the extra generation.

## 1. Choosing the window

- Hold deploys: no push to `main` during the window. The script refuses
  while a NixOS switch is running.
- Quiet computers. A computer that is busy when marked (a run in flight or
  a self-development operation open) refuses its marker, except that a
  `critical_protected` or `public_platform` computer (the paper, the
  platform computer) is asked with `allow_busy` and takes the marker with
  runs running. The script retries for `--busy-wait` (600 s) and then
  aborts, clearing the marker on every computer that may hold one.
  `--allow-crash-close` reboots anyway through computers busy with runs,
  and their work ends `interrupted_by_restart`. A guest that cannot say
  whether an apply or a capsule is live (no answer, a timeout, a 503,
  another 5xx, a 404/405) aborts unless `--allow-crash-unverified`
  accepts that the reboot may cut one. A refusal for a self-development
  apply or a live capsule always aborts.
- The paper's source schedules are standing intents and survive any
  restart (`recoverSourceSchedulesAtBoot`). A crash closes only the poll
  in flight. The next tick is armed one interval after boot. A
  planned boot keeps the schedule's generation and its saved tick.

## 2. The reboot

With the IPMI console open in another window:

```bash
scripts/node_b_host_reboot.py --execute --console-verified
```

The script runs these steps in order:

1. **Pre-checks** (read-only). It reads the kernel, nested, GRUB and md
   state, units, vmctl, firewall chains, staging `/health`, a
   verify-host-hardening baseline, and the running computers with their
   run counts.
2. **Markers.** For every running, non-held computer it sends
   `POST <computer_url>/internal/runtime/planned-restart`
   `{"target":"host-reboot kernel <v>"}`, the same request
   deploy-refresh makes, with `?allow_busy=1` for `critical_protected`
   and `public_platform` computers (vmctl's `deployRefreshAllowsBusy`).
   The guest writes the marker durably, or answers 409 while busy. The
   script logs each computer's outcome (written, busy, unsafe,
   unsupported, unavailable) and keeps it in the receipt under
   `marks.outcomes`, with each computer's worst outcome across the
   rounds under `marks.worst` (the abort reads the worst). After the
   rounds it sends every marker again, immediately before the reboot, so
   each marker's 15-minute TTL starts then and the guest re-checks for an
   apply or capsule that started meanwhile; a computer that does not
   write again aborts the run. `marks.marked_at` is that last send. An
   abort, or any failure in this step (an ssh drop or timeout), clears
   the marker on every computer that may hold the host's: one that
   answered 200 in any round, gave no answer, answered 5xx, or was in a
   call that failed (`marks.may_hold_marker`). A computer that refused
   (409, 400, 403, 404, 405) is not cleared: the host wrote nothing
   there, and the marker file is shared with the guest's own
   self-development apply, whose marker a DELETE would remove. A clear
   that fails is logged and kept under `marks.unmark_failed`
   (clear those by hand: such a marker would make a crash in the next 15
   minutes boot as planned). Held computers, and every computer under
   `--no-markers`, are not asked; the script reads their
   self-development count from the obligations surface (`selfdev.busy`):
   a nonzero count aborts, an unreadable one needs
   `--allow-crash-unverified`. The computers are listed again
   immediately before the final pass: unasked computers are checked
   again, and a computer that started running during the wait is asked
   in the final pass. A 404/405 guest is not asked again in later
   rounds. A `WARNING` line before the reboot names every unasked
   computer whose live capsules were not checked. A final-pass abort
   (a computer that wrote earlier answers anything else, even `busy` or
   a passing 503) clears the markers: resolve the contention and rerun
   from the pre-checks.
3. **Reboot.** It runs `ssh node-b systemctl reboot` and waits up to
   `--boot-timeout` (1200 s) for ssh with a new `boot_id`. If ssh exits
   with a status other than 0 or its own 255 (systemctl refused: an
   inhibitor, a bus error), or the old `boot_id` still answers after
   `--down-wait` (300 s), the host did not go down: the script clears
   every marker and exits 3. If the host goes down later anyway, every
   computer boots as a crash (safe); run `--phase post` after it. Any
   failure after the markers are written and before this step ends (a
   failed receipt save, an interrupt) also clears them.
4. **Post-checks** (below). The receipt JSON goes to
   `$TMPDIR/node-b-host-reboot-<UTC>.json`.

It does **not** hibernate. A snapshot is bound to the host kernel release
(`host_kernel_mismatch`, `internal/vmmanager/snapshot.go`). After this
reboot every snapshot cold-boots as a crash boot, so hibernating first
only costs time and disk. The 147 hibernated computers at the time of
writing will cold-boot on their next wake whatever happens.

### Marker policy (the default, stated)

**Default: idle computers, and busy `critical_protected` and
`public_platform` computers, get markers written immediately before the
reboot, and the post-checks verify the boot happened inside the TTL.
Other busy computers' work is never carried across: the run waits for
them to go idle, or it aborts. No asked computer is rebooted through a
self-development apply or a live capsule its guest reports, and by
default none whose guest cannot say. Unasked running computers (held,
or all under `--no-markers`) are checked only for self-development
operations, read-only; their live capsules are not readable without
asking and are not checked. Computers that are not running are not
checked: the kernel change invalidates their snapshots, so a hibernated
computer's in-memory capsule is lost at its next wake.** Here is why,
and what the alternatives cost:

- The marker lasts 15 minutes (`plannedRestartMaxAge`). That is short for a
  host reboot, and this one's duration is unknown (estimate: 2 to 5
  minutes of POST, plus a first boot of a new stage 1). So the markers
  are the very last step before `systemctl reboot`. The post-checks report
  the marker age and each marked computer's actual boot kind
  (`restart.kind`). An expired marker makes that boot a crash boot, which
  closes work as `interrupted_by_restart`. That is the safe fallback, not
  a failure.
- The guest writes a marker only while it is idle (409 otherwise), unless
  asked with `allow_busy`. So for most computers a marker can resume
  parked work (scheduled wakes, the source schedules' generations and
  saved ticks) but never re-dispatches a run that was in flight. The
  always-busy tiers (`critical_protected`, `public_platform`; owner,
  2026-10-15, CLAUDE.md) are asked with `allow_busy`, the same rule
  vmctl's deploy refresh uses: their running runs park and resume once on
  the planned boot. The guest still refuses while a self-development
  operation is mid-transaction or a capsule is live, and the script then
  aborts, even under `--allow-crash-close`. `--no-allow-busy` restores
  the idle-only rule for every computer.
- `--allow-crash-close` accepts crash semantics for computers still busy
  with runs after `--busy-wait`. Their in-flight work closes as
  "interrupted by a restart" and is not resumed. It does not cover a
  guest that never answered or could not read its own state.
- `--allow-crash-unverified` also reboots through computers whose guest
  could not say whether an apply or a capsule is live. It is the risk
  the operating contract forbids on purpose: use it only for a guest
  known dead or wedged, from the console or the obligations surface.
- A guest built before the 409 body carried `live_capsules` is read as
  unsafe (fail closed), so it can never be marked while busy.
- `--no-markers` gives pure crash semantics for every computer. It is the
  most conservative choice, and parked work closes too. It still aborts
  on a computer whose obligations surface shows a self-development
  operation mid-transaction; live capsules are not checked. Use it if the
  console shows a slow POST, so the reboot will clearly take more than
  about 12 minutes.

A marker counts only if its computer boots within **15 minutes** of being
marked (`plannedRestartMaxAge`). Here is what that means for each kind of
computer:

- **Always-on and critical computers** (premium_always_on,
  critical_protected, the platform computer). vmctl's sweeper wakes them
  about a minute after vmctl starts. If the host is back within about 12
  minutes, they boot planned and resume their parked work once.
- **Other computers that were running** boot on their next request. By
  then their marker is stale, so the boot is a crash boot. They were idle
  when marked, so no run was in flight.
- **Any computer, if the host takes longer than 15 minutes:** every boot
  is a crash boot. That is safe by design: work closes as "interrupted by
  a restart" and is never re-dispatched.

### Without the script

```bash
ssh node-b 'curl -fsS -H "X-Internal-Caller: true" -H @/var/lib/go-choir/secrets/internal-service-token.header \
  http://127.0.0.1:8083/internal/vmctl/list | jq -r ".ownerships[] | select(.state==\"active\") | [.vm_id,.computer_url,.warmness_class] | @tsv"'
# per running computer (from Node B):
ssh node-b 'curl -sS -X POST -H "X-Internal-Caller: true" -H "Content-Type: application/json" \
  -d "{\"target\":\"host-reboot kernel 6.18.57\"}" http://10.200.X.2:8085/internal/runtime/planned-restart'
ssh node-b systemctl reboot
```

## 3. After the reboot

`scripts/node_b_host_reboot.py --phase post --state <receipt>` re-runs
these checks read-only at any time:

- `uname -r` is the staged kernel read before the reboot (6.18.57 for
  the KF deploy). `/run/booted-system` equals
  `/run/current-system`. The hostname is go-choir-b.
- `cat /sys/module/kvm_intel/parameters/nested` shows `N`.
- `systemctl is-system-running` shows `running`, there are no failed
  units, and every go-choir unit that was active before is active again.
- `/proc/mdstat` shows both arrays `[UU]`.
- vmctl `/health` returns 200.
- The firewall chains are re-created by vmctl at start
  (`reconcileAtStart`): FORWARD begins with `-o vm-+ … -j
  CHOIR-GUEST-RET` (replies only), then `-i vm-+ … -j CHOIR-GUEST-FWD`
  (the order `ensureBaseLocked` installs), `CHOIR-GUEST-FWD` ends in
  `-j CHOIR-GUEST-TAPS` then `-j DROP`, and `CHOIR-GUEST-TAPS` and
  `nixos-fw` exist.
- `scripts/verify-host-hardening.py` shows no failures beyond the
  pre-reboot baseline. Pass the current rollout stage with
  `--hardening-args '--expect-internal … --expect-dolt …'`.
- Deploy identity: `https://choir.news/health` shows `status ok`,
  `vmctl_status ok`, and the same `platform.deployed_commit` as before.
- Computers: always-on and critical computers are `active` with
  `/health` 200. Each marked one's boot kind
  (`GET <computer_url>/api/runtime/obligations` → `restart.kind`) is
  `planned` if it booted inside the 15-minute TTL. Past the TTL it is
  `crash_or_stop`, which is expected; the script reports the marker age.
- Guest kernel coverage: **a computer the kernel deploy deferred because
  it was busy stays on the old guest kernel (6.18.55 for the KF deploy)
  until vmctl's retry sweep catches it at an idle moment (every minute;
  `GET /internal/vmctl/deploy-refresh` lists it as pending). A held
  computer is skipped, never retried. After unhold, a running one needs
  the next deploy or an explicit deploy-refresh request; a stopped or
  hibernated one is booted by the warmer onto the current image; a failed
  one needs the explicit request.** The pre-checks flag every running
  computer that booted before the current guest image was installed. The
  reboot cold-boots every computer onto the new image, so after it all
  running computers are on the new guest kernel (6.18.57). A computer still hibernated or stopped
  gets the new image at its next wake, because its snapshot is invalid.
- The paper: its computer (critical_protected) is active, its source
  schedules are armed, and new paper ticks appear after one interval.
  **Paper healthy = health ok AND schedules not overdue AND inference in
  the last 30 minutes.** Health ok alone passed for 1h45m on 2026-10-15
  while 26 of 28 schedules were overdue and the paper made no call
  ([problem](../problems/paper-silent-since-10-56-after-host-reboot-2026-10-15.md)).
  Schedules: `GET /api/autopaper/sources/schedules` with the paper
  credential; none may have `overdue: true`. Inference: the host timer
  `go-choir-paper-liveness-probe` alerts at journal priority err
  (`journalctl -u go-choir-paper-liveness-probe -p err --since -1h`), and
  `scripts/verify-host-hardening.py` fails its `paper-probe` check on an
  alert in the last 15 minutes. The same state is the Landing Loop's
  "paper healthy" step after every deploy.

Then record the receipt in the problem doc: the kernel, `boot_id`, how
long the reboot took, and the boot kinds.

## Rollback

1. **ssh isn't back within 20 minutes.** Open the IPMI console and find
   where the boot stopped: firmware POST, GRUB, stage 1 (a systemd
   initrd emergency shell, e.g. md assembly), or stage 2.
2. **At the GRUB menu,** choose "NixOS - All configurations", then one of:
   - the 6.18.16 fallback entry, if step 0.4 added it. It is the only
     proven kernel and initrd: scripted stage 1, old userspace;
   - an earlier go-choir generation. For the KF reboot, the 6.18.55
     generation booted on 2026-10-15 is known good: pick it when 6.18.57
     or the newer system is at fault.
3. **Stage 1 fails on every go-choir entry and there is no fallback
   entry:** use OVH Netboot → Rescue, assemble the arrays
   (`mdadm --assemble --scan`), and mount root and `/boot`. Repairing a
   NixOS profile from the Debian rescue system is awkward. The cleaner
   path is a NixOS installer ISO through the IPMI virtual media, then
   `nixos-enter` and
   `/nix/var/nix/profiles/system-<N>-link/bin/switch-to-configuration boot`.
4. **Repo rollback:** `git revert` the kernel commit, push, and let CI
   redeploy (a switch). The previous kernel takes effect only at the next
   reboot.
5. **Nested virtualization only:** delete the `boot.extraModprobeConfig`
   line, deploy, and reboot.

## Known unknowns

- How long POST, the BMC and GRUB take on this board (estimate: 2 to 5
  minutes). It decides whether markers are still fresh.
- Whether systemd stage 1 assembles the md RAID1 root on this host. It is
  untested, and the scripted stage 1 that booted it in March is gone from
  the menu.
- Whether GRUB's one-shot `next_entry` (`grub-reboot`) clears itself with
  `/boot` on md RAID. Don't rely on it.

## OVH API: reboot and rescue (added 2026-10-15)

Node B is `ns106285.ip-147-135-70.us` on OVHcloud US
(`api.us.ovhcloud.com`), data centre vin1. The credentials are the
operator `.env` keys `OVH_APPLICATION_KEY`, `OVH_APPLICATION_SECRET` and
`OVH_CONSUMER_KEY`. That consumer key has full account access
(`GET/PUT/POST/DELETE /*`) and no expiry: scope it to this server later.
`ops-local/ovh_node_b.py (operator-local, gitignored)` wraps the calls. It is read-only unless given
`--execute`.

- **Status:** `python3 ops-local/ovh_node_b.py (operator-local, gitignored) status`. Boot options: harddisk
  (id 1), rescue `rescue12-customer` (id 218949), poweroff.
- **Planned reboot,** when SSH `reboot` is unavailable or hangs:
  `python3 ops-local/ovh_node_b.py (operator-local, gitignored) reboot --execute`. It returns a task id;
  poll it with `... task <id>`.
- **If the host does not come back** (no SSH within ~10 minutes of a
  reboot):
  1. `python3 ops-local/ovh_node_b.py (operator-local, gitignored) boot-rescue --execute` sets the next
     boot to OVH rescue and reboots. The rescue root password is emailed to
     the OVH account (rescueMail is unset).
  2. In rescue, assemble the RAID1 (`mdadm --assemble --scan`), mount the
     root and `/boot`, and point the boot default at the known-good entry
     (the fallback 6.18.16 entry, if it was added before the reboot).
  3. `python3 ops-local/ovh_node_b.py (operator-local, gitignored) boot-disk --execute`, then reboot. The
     next boot is from disk again.
- Always leave the boot setting on harddisk (bootId 1) when done; a server
  left on rescue boots rescue on every reboot.
