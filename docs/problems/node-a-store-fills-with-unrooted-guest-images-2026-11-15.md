# Node A's Nix store fills with unrooted guest images

Date: 2026-11-15

## Problem

Node A's root filesystem reached 99% and then 1 GiB free. At 1 GiB free
`nix-store --gc` itself crashed three times with SIGBUS (05:20Z to 05:21Z)
until one dead image was deleted by hand.

## Evidence

Read and collected on Node A, 2026-11-15, 04:40Z to 05:25Z, by an agent under
the owner's approval. `df -BG /` went from 13 GiB free to 488 GiB free (953G
size, 49% used). The collection deleted 8,114 paths, 533.8 GiB by Nix's count.
About 455 GiB of that was 276 unrooted `go-choir-guest-image` builds created
2026-11-09 to 2026-11-15 by Node B's remote builds. System generations 73 and
74 went with it; 75 to 78 are kept (77 is the one before the memory repair).
No VM shares the host store live: each boots a read-only erofs image that is
in the current system's closure, so a collection cannot remove a running VM's
store. Mike's three VMs and the runner kept their PIDs and start times.

The agents VM was restarted at 04:42Z and runs at 4 GiB (3916 MiB inside).

## Belief

The recurring cost is the guest image builds: about 75 GiB a day of outputs
nothing roots. The weekly collection is too slow for that rate.

## Remaining error

- Nothing is changed in the host config: either the remote build should not
  keep its outputs on Node A, or the collection should run daily. That is a
  change to Mike's infra flake and waits for the owner.
- About 12 GiB disappeared between 04:47Z and 05:17Z; `/srv/choir-backup`
  grew from 158 GiB to 167 GiB and may be part of it. Not established.
- The agents VM's home image holds about 20 GiB a single `fstrim /home` would
  return (its first timer run is 2026-11-16), and 30 to 60 GiB of builders'
  leftovers (`.tmp` 24 GiB, `mirror` 32 GiB, the Go build cache 10 GiB).
- The agent's wait for the runner was faulty (its `pgrep` counted its own
  shell), so the collection started 30 minutes later than it could have.
