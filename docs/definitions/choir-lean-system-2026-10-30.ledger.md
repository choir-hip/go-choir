# Ledger: choir-lean-system-2026-10-30

Append-only. One terse entry per pass. Never edit earlier entries.

- 2026-10-30 ~23:15 UTC. Goal authored; the data system build superseded (phase 1 landed). Already done tonight before authoring, on the owner's grants:
  - Node A was full (145 MB free) and the off-host backup failed with 507 after uploading 32 GB. Deleted the failed run's 472 orphan packs and 6 index files (all written 22:29-22:59, no snapshot referenced them; previous run 03:07): 32 GB.
  - Node A: deleted 144 workstation snapshots, keeping the newest per workstation (ws-control keeps 48 turn and 14 daily by its own rule; the owner asked for the latest generation only): 7 GB, the rest shared blocks with the live disks.
  - Node A: nix-collect-garbage --delete-older-than 2d: 4,893 paths, 389 GiB. Node A now 588 GB used, 359 GB free.
  - Node A restic repository: forget --keep-last 1 kept 5 snapshots (one per path group) and removed 7; the prune was killed for memory at 11 GB RSS (node A has 31 GB shared with the VMs), its stale lock removed. Not retried: L5 replaces the repository and deletes the old one after a verified snapshot.
  - Started: L3 triage (nine Haiku seats over 1,298 docs), L2 builder (crash restart with a loop bound).
