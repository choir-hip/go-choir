# The kernel pin is behind a guest-reachable host fix (6.18.55; the virtio-net checksum fix is 6.18.57)

Date: 2026-10-22. Found by: FP0's kernel research (a Sonnet research agent asked by the owner whether to move to kernel 7; the full report, with citations, is in FP0's run folder `_runs/kernel7/report.md`). Mutation class of the fix: red (the host and guest kernel; the host change takes effect only at a host reboot).

## Problem

Host and guests run Linux 6.18.55, the `nixpkgs-kernel` lock at rev 7fd4004d… (its nixos-unstable commit is dated 2026-10-11, the lock's `lastModified`; an earlier line here said 2026-10-03). 6.18.56 (2026-10-11, 403 commits), 6.18.57 (2026-10-15, 622) and 6.18.58 (2026-10-21, 49) shipped since. 6.18.57 carries "ipv4: validate checksum_start before completing checksum" (commit 6785011f8b16): guest-supplied virtio-net checksum metadata (`csum_start`, `csum_offset`) was not checked against the IPv4 header, so a deferred checksum write could rewrite the header-length byte and leak or corrupt host kernel memory. The fix is in the official stable changelogs (ChangeLog-6.18.57, ChangeLog-7.2.11). Its public write-up ("PwnEscape") reports a repeatable host memory disclosure; the same researcher's "KVM 0day" was confirmed by a Firecracker-based sandbox vendor in early October. That they are the same bug is not confirmed.

## Why Choir is exposed (reasoned, not drilled)

Firecracker offers the guest `VIRTIO_NET_F_CSUM` and opens each tap with `IFF_VNET_HDR`, so a guest kernel controls that metadata on the host's tap device. Exploiting it needs control of the guest kernel's virtio driver (guest root), not a desk cell or a capsule: desk cells and capsules have no raw sockets. Choir's guests run model-driven code with root daemons, so guest root is the boundary that matters.

## Fix

1. Update `nixpkgs-kernel` alone (`nix flake update nixpkgs-kernel`, the kernel half of the patch cadence named in `flake.nix`) to a nixos-unstable revision whose `linuxPackages_6_18` is at least 6.18.57.
2. Raise the flake's security floor assertion from 6.18.45 to 6.18.57, naming this fix beside the older ones.
3. Deploy (canary first; see "Deploy plan" below): the guest kernel reaches each computer at its next refresh (a cold boot; snapshots taken under 6.18.55 are refused, as for any kernel change). The host kernel takes effect only at a host reboot. That is the owner's timing; it also clears the pending reboot (booted system 26.11.20260701, current 26.11.20261015).

## The lock bump also changes the kernel config (added 2026-10-22)

The new `nixpkgs-kernel` lock does not only move 6.18.55 to 6.18.57: its
nixpkgs builds the 6.18 kernel with a different config. Both reviewers
(DeepSeek F4, Sonnet F2) found it by evaluating the two configs; this
lane has no nix, so this list is theirs, not re-derived:

- `MODULE_SIG=y` with `MODULE_SIG_ALL`: every module is signed at build
  time with a throwaway key. Without `MODULE_SIG_FORCE`, unsigned modules
  still load (with a taint), so the guest's `overlay`, `virtio_mem` and
  other `=m` modules load as before.
- `SECURITY_LOCKDOWN_LSM=y` and `SECURITY_LOCKDOWN_LSM_EARLY=y`: built in,
  inactive. Lockdown stays `none` unless a `lockdown=` boot argument or a
  secure-boot path turns it on, and Choir passes neither.
- Integrity keyrings (the `INTEGRITY` / platform keyring options).
- `NR_CPUS=512` and `LOG_BUF_SHIFT=20` (a 1 MiB kernel log buffer, a
  little more guest memory).
- `kernel.dev/vmlinux` becomes a symlink. The guest image step
  `cp ${guestKernel}/vmlinux $out/vmlinux` (`flake.nix` `mkGuestImage`)
  follows it, because cp dereferences a symlink named on its command line.
  The image still holds a real ELF file.

None of these is expected to change what a guest does. That is a belief,
and the canary below is the test of it.

## Deploy plan: canary the guest image before the paper takes it

Every computer cold boots onto the new guest kernel at its deploy refresh,
and the paper (`critical_protected`) refreshes even while busy. Hold the
paper during the deploy, prove the new image on a test computer, then
release the paper.

What vmctl does (read 2026-10-22, `internal/vmctl`):

- `POST /internal/vmctl/hold` `{"computer_id","reason","held_by"}` sets a
  durable hold (`ownership.go` `SetHold`). While held, vmctl takes no
  automatic lifecycle action on the computer. The deploy's selection
  lists it under `skipped` with reason `held` (`deploy_refresh.go`
  `selectDeployRefresh`). CI refreshes only the candidates.
- **A held computer never enters the retry sweep.** The sweep retries only
  *deferred* computers (`deploy_refresh_retry.go` `settle`, `Sweep...`).
  A skipped computer is never recorded, and `POST /internal/vmctl/unhold`
  only clears the hold (`ClearHold`). So after unhold, the paper stays on
  the old image until something refreshes it: the next deploy, a host
  reboot, or the explicit request below.
- The catch-up is the deploy's own request, sent once by hand:
  `POST /internal/vmctl/deploy-refresh {"user_id","desktop_id","target"}`
  (the same body CI sends, `.github/workflows/ci.yml` deploy refresh). The
  paper is `critical_protected`, so `deployRefreshAllowsBusy` lets it take
  a planned, marker-backed refresh while busy and resume. If the guest
  defers (a self-development apply or an engineering capsule live), vmctl
  records it as pending, and the sweep retries every minute until it takes
  the refresh. No state is edited by hand.

FP0's procedure (all calls on Node B, with the internal headers the
reboot runbook uses, `-H "X-Internal-Caller: true" -H @/var/lib/go-choir/secrets/internal-service-token.header`,
against `http://127.0.0.1:8083`):

1. Before pushing the KF commits to main: hold the paper,
   `POST /internal/vmctl/hold {"computer_id":"<paper computer id>","reason":"KF canary: guest kernel 6.18.57","held_by":"fp0"}`.
   `GET /internal/vmctl/deploy-refresh` lists it under `skipped`, reason
   `held`.
2. Push; CI deploys. Idle computers refresh. A hold has no TTL. It
   persists across deploys and vmctl restarts, and while it is set vmctl
   never recovers, warms or refreshes the paper (`ownership.go`
   `MaintenanceHold`, `SetHold`). Unhold only clears the hold: a running
   paper stays on the image it booted, and only a stopped or hibernated
   paper is then booted, onto the current image, by the warmer. So an
   unhold is safe for a running paper only after the deploy's refresh
   selection has run (the deploy log lists the paper as skipped, reason
   `held`); before that, the selection would refresh the unheld paper
   onto the untested image. Every unhold below sends the body
   `{"computer_id":"<paper computer id>"}`; an empty body is refused
   with 400 `invalid request body` and leaves the paper held.
   Watch the paper's `/health` and its paper-healthy signals the whole
   time:
   - If the paper's `/health` fails while the canary has no verdict or
     has passed, unhold it at once and do not wait for step 5. vmctl
     recovers it onto the new image: an untested image is better than a
     paper that is down.
   - If the paper's `/health` fails after the canary failed, leave it
     held (step 4): recovery would boot it onto the failed image. Record
     the outage; the revert's deploy is the fix.
   - If the canary has no verdict 30 minutes after the deploy finished,
     record that it did not run in time and unhold (the selection has run
     by then, so the running paper stays on the old image).
   In the operator shell, keep two variables, `canary` (`pending`,
   `passed`, `failed`) and `selection_done` (`yes` once the deploy log
   shows the paper skipped as `held`), and set
   `trap '[ "$canary" != failed ] && [ "$selection_done" = yes ] && curl ... /internal/vmctl/unhold -d "{\"computer_id\":\"<paper computer id>\"}"' EXIT`,
   so that an aborted session leaves the paper unheld only when that is
   safe. Once the canary fails, run `trap - EXIT` before starting the
   revert.
3. Canary: mint a test computer (`node scripts/qa_test_computer.mjs --label kf-canary`).
   It cold boots on the new image. If the mint is refused (host memory
   admission, disk), use an existing test computer that the deploy
   refreshed instead (the deploy log's `Refreshed <vm>` line): the FP0
   test computers vm-c5c801f8 (computer-71afbf14) or vm-80b173b9
   (computer-9648031f). Never the owner's computer (vm-4a9afa23) or the
   paper. One that was `Deferred` is still on the old image and does not
   count. If neither exists, record that the canary did not run and
   unhold the paper under step 2's rule. Pass when all of these hold:
   - its console log (`/var/lib/go-choir/vm-state/<vm>/console.log`)
     shows `Linux version 6.18.57`, and its kernel capability receipt
     shows `kernel_release` 6.18.57 with every capability present;
   - no `module verification failed` that stops a module, and
     `/sys/kernel/security/lockdown` reads `[none]` (or the file is absent);
   - the store opens: `/health` ok and a store read through the API
     succeeds;
   - a model call passes: one prompt on the test computer gets a model
     answer (the qa script's run, or a Texture prompt);
   - hibernate by snapshot, then wake, works on 1.16.2: the boot kind is
     `snapshot_resume`, and the snapshot meta and timeline record
     `Firecracker v1.16.2` (step 2 of "FP0 verifies after the deploy" in
     [firecracker-1-16-1-virtio-mem-restore-bugs-2026-10-22.md](firecracker-1-16-1-virtio-mem-restore-bugs-2026-10-22.md);
     the `scripts/s3_snapshot_probe.py` pattern). A failure here is
     fail-closed (the refused snapshot cold boots), but it is a reason
     not to give the paper the image.
4. If the canary fails, set `canary=failed` and run `trap - EXIT`. Leave
   the paper held and running on the old image, revert the KF commits,
   and redeploy. Unhold once the revert is deployed (the paper then stays
   on, or recovers onto, the reverted image).
5. If the canary passes: `POST /internal/vmctl/unhold {"computer_id":"<paper computer id>"}`,
   then `POST /internal/vmctl/deploy-refresh {"user_id":"<paper user>","desktop_id":"<paper desktop>","target":"<deployed autoputer runtime commit>"}`.
   The target is the commit the deploy's refresh log expected; a
   refreshed computer's `/health` reports it as `build.commit`. Expect
   `outcome` `refreshed` with `planned_restart` true, or `deferred` (then
   watch `GET /internal/vmctl/deploy-refresh` `pending` until it clears).
   Then run the paper-healthy check (reboot runbook, section 3).
6. The host reboot comes after this. It cold boots every computer, the
   paper included, onto whatever guest image is current, so it must not
   come before the canary.

## As landed

`flake.lock` `nixpkgs-kernel` is now rev 02c087f3… (nixos-unstable,
`lastModified` 2026-10-20 13:28 UTC), with `linuxPackages_6_18` at
6.18.57. Commit 218a33a3b (2026-10-22) moved the lock and the floor. The
floor now lives in `nix/kernel-security-floor`, which the flake assertion
and `scripts/node_b_host_reboot.py` both read.

## What stays

Stay on the 6.18 LTS line (supported to about December 2028; Firecracker tests 6.18 as host and guest, and no 7.x). Revisit at the next 7.x LTS once Firecracker lists it. Cadence from the research: weekly `nixpkgs-kernel` bump for the guest, a monthly planned host reboot, an emergency host reboot within 72 hours for a guest-reachable host fix (this one), and an alarm when the booted kernel differs from the activated one.

## Related

- The Node B CPU has 39-bit physical addresses (`grep 'address sizes' /proc/cpuinfo`, read 2026-10-22). Firecracker places the virtio-mem hotplug region at 512 GiB, which a 39-bit CPU cannot address (firecracker issue #6255, open). **Corrected 2026-10-22:** this line first blamed 1.17. The placement is the same in 1.15.1, 1.16.1, 1.16.2 and 1.17.0, so hotplug cannot plug on Node B today either ([memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md](memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md)). Firecracker 1.16.2 fixes two virtio-mem snapshot-restore bugs and changes the snapshot format; it is the nearer upgrade.
- docs/problems/host-kernel-216-days-unpatched-2026-10-15.md (the floor's origin).
