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

## Artifact GC dry-run does not finish (2026-10-09 12:30Z)

The dry-run started at ~11:15Z and had written nothing after more than an
hour. Its live set includes `SELECT body_ref FROM og_objects` against Store B
(the 104 G corpus Dolt), so the sweep's cost is bound to the store being torn
down. That is one more reason to finish the corpus teardown before turning
artifact GC on. The 12:24Z platform deploy restarts the service and ends the
request. Hypothesis, not yet measured: the og live-set query dominates.

## Store B reset runbook (owner-approved teardown; executing 2026-10-09)

Store B holds only World Wire data: `og_objects`/`og_edges` (object graph),
`platform_texture_revisions`, publication routes and ingestion tables. Code
reaches it only via `Store.corpus()`, which **falls back to Store A when the
DSN is unset**. So unwiring `CORPUSD_CORPUS_DOLT_DSN` would not tear Store B
down. It would send World Wire reads and writes to Store A's stale
pre-split tables (the 2026-10-05 og wrong-store class). The reset keeps the
wiring and swaps the repo:

1. Survey: the only connection to :13307 is corpusd; sourcecycled is
   inactive and out of the boot set.
2. `systemctl stop go-choir-corpus-dolt`. corpusd stays up: credential
   issuance and the event tape are Store A.
3. `mv corpus-dolt/corpus corpus-dolt/corpus.reset-20261009` (same
   filesystem, instant, reversible by moving it back).
4. `systemctl start go-choir-corpus-dolt`. `corpus-dolt-init` creates an
   empty `corpus` repo.
5. `systemctl restart go-choir-corpusd`. Bootstrap applies
   `corpusSchemaDDL` to the empty Store B.
6. Verify: corpusd health, the owner computer stays ready, a credential
   envelope issues (signup path), and World Wire reads return empty rather
   than errors.
7. Archive `corpus.reset-20261009` to node-a (584 G free) with rsync and a
   size and file-count check. Delete the Node B copy only after that check:
   ~104 G.

Rollback: stop corpus-dolt, move the empty repo aside, move
`corpus.reset-20261009` back, start corpus-dolt, restart corpusd.

Known consequences:
- Old source citations in owner documents that point at og objects will
  not resolve until World Wire is rebuilt (gate 3). The data stays in the
  archive.
- Artifact GC's og live set becomes empty. If GC is ever set to active
  before the og blob namespace is reviewed, it will collect the og bodies.
  That is the intended teardown, but only behind an explicit mode change.

## Done 2026-10-09 (owner-approved)

- 12:33Z: Store B swapped for an empty repo (runbook above). corpusd is
  healthy on the empty schema.
- 12:57Z–13:15Z: history-free dump of the old repo's current state,
  `/var/lib/go-choir/corpus-dolt/rebuild-20261009/corpus.sql` (23 GB).
- ~12:35Z: deleted the pre-compact backup (17 G) and the August console
  log. Both quarantine images kept.
- ~12:40Z: `nix-store --gc` freed 16.7 GiB (287 paths).
- 14:04Z: owner decided to start Autopaper from zero content ("yes, delete
  it"). Stopped corpus-dolt (its server had the old repo open as a second
  database), deleted `corpus.reset-20261009` (104 G), started corpus-dolt
  and restarted corpusd. Health ok.

Free space on `/`: 89 G this morning, **226 G** now. Remaining:
- the 23 GB dump (compress and copy to node-a, or delete, owner's call);
- the two quarantine images (32 G each), pending the 484-path check;
- the bounds that are still missing (O10, above).
