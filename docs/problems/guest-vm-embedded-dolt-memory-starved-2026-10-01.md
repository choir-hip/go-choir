# Guest VM embedded objectgraph-dolt starved inside 8-GiB interactive shape

**Status:** OBSERVED + fix staged 2026-10-01 (staging, build `4a718af4`, guest
`computer-03335285269bdba4f94377e56879f9e6`). The second level of the
memory substrate — the *guest VM's* memory budget, not host dolt.

## Symptom

`run start` hangs at `/api/prompt-bar` (`context deadline exceeded`), and the
guest log shows the run-persist path dying mid-write:

- `persist run ... resolve head for new event: Get http://10.200.88.1:8086/.../events/head: context canceled`
- `texture prompt bar: submit ... objectgraph dolt: scan object: context canceled`

The guest's **embedded objectgraph-dolt** cannot satisfy `resolve head` /
`scan object` — same surface as the owner's iOS Texture bug (list loads slow,
revision body fails → blank v0). Confirmed: the embedded store is the shared
dependency behind both the Texture read flap and the run-start hang.

## Root cause

The guest's interactive shape is **`VM_INTERACTIVE_MEM_MIB=8192`** (8 GiB),
set in `/etc/systemd/system/go-choir-vmctl.service:30`. The repo already
documents this defect class at
`internal/vmctl/ownership.go:1220-1224`:

> "The retained staging computer outgrew the 2 CPU / 2 GiB default (11 GiB
> texture/object-graph store OOMs the 2 GiB guest on boot replay and snapshot
> scans, 2026-09-03)."

The embedded objectgraph store was 11 GiB on 2026-09-03 and has grown since
(all the QA-research docs + texture revisions). An 8-GiB guest cannot host a
>11 GiB embedded store plus the runtime. vmctl's own pressure reclaimer fires
`pressure=true` every ~2min with "no unprotected active ownerships eligible" —
the guest is at its ceiling with nothing to reclaim. So queries thrash and
`context canceled` surfaces.

## Two-level substrate picture

1. **Host corpus-dolt** (`go-choir-corpus-dolt`, :13307): was the OOM driver —
   capped `MemoryHigh=18G`/`MemoryMax=20G` (durable drop-in). OOMKills=0,
   connection errors dropped. FIXED.
2. **Guest embedded objectgraph-dolt**: starved inside the 8-GiB interactive
   guest — `run start` + texture reads fail. THE REMAINING DEFECT.

## Staged fix (non-destructive)

`VM_INTERACTIVE_MEM_MIB=16384` appended to `/var/lib/go-choir/vmctl-priority.env`
(the ops-managed EnvironmentFile override — not a tracked-file edit). **Takes
effect on the next interactive-guest reboot** (the firecracker shape is fixed
at boot). The running guest keeps 8 GiB until then.

Host headroom supports it: 31Gi total, ~13Gi available, corpus-dolt bounded at
20G, platform-dolt 1.5G, vmctl ~8G→~16G. A 16-GiB interactive guest fits.

## What remains

- **Reboot the interactive guest** so it boots at 16 GiB — this is the
  disruptive step (kills in-flight Management work); staged and ready, do on
  the next natural recycle or an authorized restart.
- After reboot: confirm `run start` returns non-timeout and texture revision
  reads succeed — that discharges both this defect and the Texture read flap.
- Consider whether the embedded store should GC/snapshot-prune so the guest's
  budget doesn't have to grow unboundedly — a longer-horizon question for the
  capacity mission.
