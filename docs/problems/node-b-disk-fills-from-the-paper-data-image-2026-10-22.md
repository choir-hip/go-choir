# Node B's root disk fills from the paper's data image; deploys are blocked by the disk floor

Date: 2026-10-22, 14:40 UTC. Found by: FP0, when deploy R (main 0bf8a097d, CI 39794075862) failed at "Deploy to Staging (Node B)". Status: open. Mutation class of the fixes: red (deploy routing; the paper's store and data image; host disk).

## Evidence (read-only, from Node B)

- Deploy R's log: `Deploy disk preflight failed: root filesystem still below required headroom`, with `root_available_kib_after_reclaim=93855960` against `min_free_kib=94371840` (90 GiB; `scripts/node-b-deploy-disk-preflight`). The preflight's own reclaim had just freed 5.7 GiB (149 store paths). Failure evidence: `/var/lib/go-choir/deploy-failures/39794075862-1.json`.
- Root filesystem `/dev/md126` (btrfs) is 476G. Available: 90 GiB at 14:22, 85,744,784 KiB at 14:32:42, 85,494,248 KiB at 14:33:42, 84,493,092 KiB at 14:37:03. It falls about 250 to 290 MiB a minute, so at that rate it reaches zero in about five hours.
- Who grows, per `du` over 90 s at 14:35:
  - The paper's state dir (vm-1669020a…) grew +260 MiB (78,570 → 78,830 MiB).
  - The candidate fleet, 71afbf14, the owner's computer and the FP2 test computer did not grow.
- The paper's data image: `data.img` holds 78,881 MiB on the host. vmctl's lookup reads file_bytes 82,721,660,928 and cap_bytes 137,438,953,472 (128 GiB). About 54 GB of headroom is left before the cap, and at about 130 to 170 MiB a minute the image reaches it in about 5 to 7 hours.
- Totals: VM state 180 to 185 GiB, the nix store about 84 GiB, the journal 222 MiB. There are only 4 system generations, so old generations offer little to collect.

## Cause (read in docs and code; the guest-side half is a hypothesis)

The paper's Dolt journal grows about 1.6 GiB per 10 minutes (docs/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md). The online GC frees that inside the guest every few minutes, but the host image never gets the space back:

- Firecracker 1.16.1 has no virtio-blk discard. nix/node-b.nix says so where it deliberately leaves VM_DATA_DISCARD unset. So the guest's frees never reach the host.
- **Hypothesis (unconfirmed):** the guest filesystem keeps writing to blocks it never used before, so the host's sparse image grows until the whole 128 GiB is allocated. No guest-side reading of its free space was taken.
- The real fix is RES-16 (data images that shrink: discard), which needs Firecracker 1.17. Node B's CPU has 39-bit physical addresses, and Firecracker 1.17 breaks virtio-mem hotplug on such CPUs (issue #6255; docs/problems/firecracker-1-16-1-virtio-mem-restore-bugs-2026-10-22.md).

## Consequences

- Every deploy is refused while free space stays under 90 GiB, so the whole landing queue is blocked. Deploy R failed this way.
- With no action, the root disk fills in about 5 hours, or a little later if the paper's image reaches its cap first. A full root disk stops every service and computer on the host, the paper included.

## Options (FP0's recommendation first)

1. **Lower the deploy floor** (DEPLOY_MIN_FREE_KIB) from 90 GiB to 30 GiB, so landings resume. A deploy's observed build footprint is about 3 GiB, so 30 GiB still leaves 10× headroom. This does nothing about the fill.
2. **Reclaim host space that nothing needs.** Candidates: stale VM state of long-stopped test computers (vmctl's retention prune: VMCTL_STALE_STATE_MIN_AGE=6h, VMCTL_STALE_STATE_MAX_DELETES=25 per pass), and nix store paths that only old CI deploy artifacts keep alive. Never the paper's or the owner's state.
3. **Stop the paper's image from growing.** Fast levers inside the paper: less journal growth (F-a batching is off for its drain bug; F-c is being built; GC cadence). Or a one-time compaction of the paper's data image: stop it, copy the live filesystem to a fresh sparse image, start it. That is red, touches the paper's store, and is the owner's call.
4. **Discard (RES-16)** for the paper, which needs a Firecracker that supports it on this CPU. That is the durable fix and is not quick.
