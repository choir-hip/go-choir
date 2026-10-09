# Node B disk: 90 GB free of 476, with no retention on the big state

Date: 2026-10-09. Owner: "can we clear up free space on node b?"
Operational invariant O10 (declared bound per growth surface). Related:
[`guest-store-history-bloat-and-memory-shape`](guest-store-history-bloat-and-memory-shape-2026-10-09.md),
[`ci-deploy-latency-per-service-go-builds`](ci-deploy-latency-per-service-go-builds-2026-10-09.md).

## Evidence (read-only survey, ~12:00Z)

`/` (md127, holds `/nix/store` and `/var/lib`): 476 G, 382 G used,
**90–96 G free**. Nix `min-free` is 80 G, so most builds start with a store
GC that deletes cached build inputs (97 deletions in run 37918703736).

| Path | Size | What it is | Recovery role |
|---|---|---|---|
| `/var/lib/go-choir/corpus-dolt/corpus/.dolt/noms/oldgen` | **104 G** | World Wire corpus (Store B), Dolt old generations; untouched since 10-05 | Corpus teardown is owner-approved (ACTIVE.md). corpusd still holds a connection via `CORPUSD_CORPUS_DOLT_DSN` from an EnvironmentFile. |
| `/nix/store` | 115 G | build closures; 10 system generations | Current and rollback generations |
| `platform-artifacts/sha256/projection-base` | 40 G | projection bases (owner: 8.7 G × 3 plus 15.9 G from 09-09) | Checkpoint restore points; artifact GC is dry-run only |
| `platform-artifacts/sha256/file-cas-chunks` | 27 G | file content chunks | Live file data |
| `vm-state/candidate-fleet-e15cb89f…/data.img.quarantine-1-39ddc7f2…` | **32 G** | owner pre-restore image (10-07) | Holds 484 paths absent today: possibly unique |
| `vm-state/candidate-fleet-e15cb89f…/data.img.pre-compact-20261009T053125Z` | **17 G** | rollback copy for this morning's compaction | Compacted store verified over ~10 boots; tape can rebuild |
| `vm-state/candidate-fleet-e15cb89f…/console-b14-serve3.log` | 104 M | August console log | none |
| `/nix/var/nix/gcroots/deploy-deps` (1,263 roots, made by hand 09-18) | **12.3 G** pinned only by these | a 09-18 system closure plus libguestfs, rustc, linux | none; rebuildable |
| 11 loop devices on deleted guest images | unknown | agent probe mounts (09-28, 10-04) | none |

## Done this session

- Unmounted the 10 stale probe mounts (`/mnt/ed554data` × 9,
  `/mnt/m11-probe`). The loops stay attached: copies of those mounts live
  in the private mount namespaces of services (gateway, proxy, corpusd,
  auth, maild, vmctl, two dolt servers) and of two firecracker guests
  that started while the probes were mounted. They are released when
  those processes restart (deploy, guest refresh).
- Moved the 09-18 GC roots aside to
  `/root/gcroots-deploy-deps-20260918.moved-20261009` (reversible until a
  store GC runs). The next Nix GC, at `min-free` or manual, frees about
  12 G. A manual `nix-store --gc` was refused by the session permission
  classifier.

## Needs owner approval (irreversible deletions)

1. `data.img.pre-compact-20261009T053125Z`: **17 G**. Recommended.
2. `console-b14-serve3.log`: 104 M. Recommended.
3. `data.img.quarantine-1-39ddc7f2…`: **32 G**. Not recommended until the
   484 absent paths are checked against the file-root chain.
4. Corpus Store B data: **104 G**. Belongs to the teardown, in order:
   1. unwire `CORPUSD_CORPUS_DOLT_DSN` and `SOURCECYCLED_DOLT_DSN`;
   2. stop and remove `go-choir-corpus-dolt`;
   3. optionally copy the data to node-a (586 G free) as an archive;
   4. delete it.

## Durable bounds still missing (O10)

- Artifact GC runs dry-run only. A dry-run report for the 86 G of artifacts
  is running (slow; result pending).
- Nothing removes ad-hoc GC roots, probe mounts or backup images. Every
  operator copy needs an expiry, and mount helpers must detach their
  loops.
- Nix `min-free` of 80 G against ~90 G free means routine GC churn.
  Freeing the items above takes free space to ~140–250 G.
