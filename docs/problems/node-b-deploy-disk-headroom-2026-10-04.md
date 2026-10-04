# Node B deploy disk headroom — deploys blocked below 90 GiB

**Status**: open — blocks Deploy-to-Staging for any commit on `main`.
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
