# Node B deploy disk headroom — deploys blocked below 90 GiB

## Recurrence 2 — 2026-10-04T04:43Z, deploy run 37177170737 (a3f0d48e)

Third hit of the same gate within ~10 days. Headroom eroded back below 90
GiB: 88 GiB free after the script's bounded reclaim. Cleared by deleting
~16 GiB of stale forensic material the bounded reclaim cannot see:

- `/var/lib/go-choir/vm-state/*/​*.pre-upgrade-20260824T074931Z*` (incl. an
  11 GiB `data.img` backup on a stopped UWP VM) — 8 files;
- `/tmp/{texture-live,texture-ro,texture-snap,texture-dolt-ro,
  guest-data-ro,vm-live-e15cb,vminspect,sa.db*}` — 10–24 day-old forensic
  clones the owner had already mined; none referenced by any live service.

After reclaim: 96 GiB free, deploy rerun passed the gate. Live computers
untouched (owner VM 65G + stopped UWP disk preserved).
**Root-cause observation:** the bounded reclaim only reclaims *unprotected
active* VMs and nix generations; it never inspects `/tmp` or `*.pre-*`
stale artifacts inside vm-state. A third lever for the next recurrence:
`find /var/lib/go-choir/vm-state -name '*.pre-*' -mtime +7` +
`find /tmp -maxdepth 1 -mtime +7 -name 'texture-*' -o -name 'vm-*' -o
-name 'guest-*'` in the preflight script itself.

**Status**: RESOLVED 2026-10-04 — owner authorized deleting what's not needed.
Deleted `dump-20260918/platform-dump.sql` (20G rollback ref; split live ~2
weeks) + `nix-env --delete-generations old` + `nix store gc` (23.5 GiB). Free
space 84G → 103G (≥90G floor cleared). `a4fcdb8d` deployed to node-b at
2026-10-04T02:01:45Z. Recurring risk remains — see Residual.
**Date observed**: 2026-10-04 (deploy run 37165170516, commit `a4fcdb8d`).
**Computer**: Node B staging host (`/var/lib/go-choir`, root `/dev/md127`).
**Mutation class**: black to repair (candidate/rollback-ref deletion) — needs
owner direction.

## Symptom

`Deploy to Staging (Node B)` fails in the disk-preflight step:

```
Deploy disk preflight failed: root filesystem still below required headroom
```

`scripts/node-b-deploy-disk-preflight` requires `DEPLOY_MIN_FREE_KIB =
94371840` (~90 GiB) free on `/`. Observed free after the script's own bounded
reclaim: **84 GiB** (`/dev/md127` 476G, 386G used, 83%).

The bounded reclaim already ran and returned ~0: `vmctl/reclaim` →
`vms_reclaimed:0` (`no unprotected active ownerships are eligible`); the fleet
VMs are `hibernated interactive` (protected), `journalctl` vacuum and
`nix-env --delete-generations +4` / `nix store gc` did not reach the
threshold.

## Largest consumers (`/var/lib/go-choir`, `du -sh`)

| consumer | size | notes |
|---|---|---|
| `corpus-dolt` | 93G | live corpus store |
| `vm-state` | 91G | hibernated fleet disks: `candidate-fleet-e15cb89f` 65G, `candidate-fleet-d03dacaa` 22G |
| `platform-artifacts/sha256` | 65G | content-addressed build store |
| `platform-dolt` | 21G | live platform store |
| `dump-20260918` | 20G | `platform-dump.sql` — deliberate rollback ref from the 2026-09-18 authority split |
| `/nix` | 88G | nix store, 4 system generations kept (1068–1071) |

## Blocked deliverable

`d20483e8` (`consumeIdleTextureTrigger` honest decision kind, CI green run
37164284057 before supersede) and `a4fcdb8d` (probe correction) cannot reach
staging. Neither is an acceptance prerequisite — S0m's chain proved on
`d61c9b1b` — but the deploy gate means *every* subsequent commit on `main` is
also blocked until headroom is restored. This is a substrate constraint, not a
code defect.

## Repair options (owner decision — all ≥ orange/black)

1. **Delete `dump-20260918/platform-dump.sql` (20G)** — the named rollback ref.
   `docs/archive/platform-dolt-storage-normalization-2026-09-18.md` says keep
   it "until the split is verified." The split has been live ~2 weeks; the ref
   is the single safest ≥6G lever. Black: irreversible.
2. **Prune hibernated candidate-fleet VM disks** (65G + 22G available) —
   `candidate-fleet-*` are disposable by ontology, but they're `hibernated
   interactive` / protected, so reclaim won't touch them; deleting their
   `vm-state` destroys those computers' state. Red/black.
3. **GC `platform-artifacts/sha256`** — content-addressed; re-derivable, but
   65G of it may be cold build cache. Needs a store-aware prune, not `rm`.
4. **Raise the headroom floor or shrink `corpus-dolt`** — structural, slower.

## Residual

Until one of (1)-(4) frees ≥ ~6 GiB, deploy is blocked. `d20483e8`/`a4fcdb8d`
are parked as `pushed_commit + ci` receipts without `deploy`/`environment_
identity`; they land when headroom is restored and `Deploy to Staging` re-runs.
