# Host-Orchestrated Tape Recovery — Design

- Date: 2026-08-22
- Status: reviewed — panel `approve with repairs` (codex, omp-gpt56-sol 2026-08-22); blocking repairs applied below
- Problem: `docs/evidence/effects-red-guest-dependent-restore-2026-08-22.md` (red, staging computer 0333528 looping `COMPUTER BOOT IS STILL PENDING`)
- Related: `docs/definitions/choir-tape-recovery-2026-08-13.md` (tape completeness), `docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md` (E2 restore fence), `docs/computer-ontology.md` Ledger Split, `internal/agentcore/rematerialize.go`, `internal/computerevent/appender.go`, `internal/proxy/computer_lifecycle.go`

## Goal

Make tape restore work when the guest is down, without giving the host arbitrary event-write authority or a filesystem backdoor.

The durable tape is `corpusd` (`platform Dolt`, `EventSource` + `HeadCAS` per `appender.go:108`). `data.img` (`/var/lib/go-choir/vm-state/<vm>/data.img`, virtio-blk `drive_id=data`) is a disposable projection rebuilt on every boot via `ReconstructThroughTarget` (`autoputer/run.go:199`). Host should orchestrate quarantine + re-bootstrap; guest remains the single semantic appender and the verifier.

## Non-goals (structural, not policy)

First slice compiles out `authorized_checkpoint` entirely — no `checkpoint_digest`/`authorization_ref` fields, no `append(EventRestoreRequested)` capability, no `.../lifecycle/restore` cold fallback. Tests reject `99949fe2` and any checkpoint-bearing payload with 400/409 before any host state change. History restore is a separate E2 Definition change with quorum verification.
No general host event append, no platform-store rewind, no World Wire, no Cloud Hypervisor / parallel assignment work, no effects ON.
No host loop-mount of ext4, no third Dolt, no new minimal initramfs for the first slice.

## Two operations, different fencing (first slice ships only the first)

| Operation | Target | Event | When | Who | Shipped |
|---|---|---|---|---|---|
| `recover_current` | current canonical head from corpusd (resolved by host, not caller) | none — forward recovery, no semantic event | guest down / `:8085` refused, any time | host `vmctl` orchestrator + guest self-reconstruct on boot | **this Definition** |
| `authorized_checkpoint` | `checkpointDigest` (e.g. `99949fe2`) | `EventRestoreRequested` first, then `ReconstructThroughTarget` halt at `AcceptedEventHead` | E2 only, after CoSuper→consensus→promotion→play→falsify | same host orchestrator + guest, gated on canonical authorization | **not in this Definition — future gate** |

`recover_current` structurally cannot rewind because it takes no checkpoint param and its journal/code has no checkpoint fields. `authorized_checkpoint` is not compiled in this slice.

## Architecture — recover_current only

```
owner (cookie or API key with ComputerVersion route guard)
  → proxy
    → active :8085 ? guest RematerializeFromTape (existing path, unchanged)
    → inactive/failed && owner-authorized && mode==recover_current ? vmctl cold path (new)
vmctl (internal/vmctl/cold_recover.go)
  → per-ComputerID fencing token + corpusd recovery lease (allowlist: only boot lifecycle appends from the new realization)
  → fence route, stop VM, quarantine data.img AS FILE (no host mount)
  → copy privacy-key via trusted-guest attachment (no host ext4 parse)
  → stage fresh sibling data.img (sparse, via existing disk-instantiation)
  → boot fresh, let guest replay current head, verify equivalence + ComputerVersion + frontend before route CAS
  → retain quarantine as evidence, idempotent journal for crash recovery
```

### Shared engine — not used by recover_current

For `recover_current`, normal boot replay is canonical (`autoputer/run.go:199-224` `ReconstructThroughTarget` / `Reconstruct` + `replay_completeness`). Do not invoke or extract `selfdevprotocol.RematerializeFromRequest` / `WitnessContentMatches` (they require a checkpoint, `rematerialize.go:30-45`). Those belong to the future `authorized_checkpoint` slice and its `internal/computerrestore` extraction. First slice uses only existing guest replay and a new host orchestrator; the shared engine extraction is deferred until E2.

### vmctl cold path — recover_current request

Internal endpoint (recover_current only):

```
POST /internal/vmctl/computers/{computerID}/cold-recover
{ computer_id, expected_canonical_head, expected_route_generation, idempotency_key }
// NO owner_id, NO checkpoint_digest, NO authorization_ref, NO mode field
// vmctl re-derives owner/VMID/route slot from OwnershipRegistry independently; unknown fields rejected 400
```

- Rejoin owner/computer/VMID/route slot from `OwnershipRegistry` + `route_authority` independently; caller-supplied `owner_id` is not authority (prevents cross-tenant disk transition via network-reachable header). Derive VM paths exclusively from registry; reject symlinks/non-regular images; enforce containment under configured state root.
- Fencing token: on `fenced`, atomically join {ComputerID, owner, VMID, route slot, routeGeneration, canonicalHead, recoveryGeneration (monotonic)} in corpusd. Every subsequent stop, quarantine rename, staging create, boot accept, and route CAS carries the token; stale generation is refused. `isInternalCaller` alone is not authentication; the token is minted only after owner+route verification and is bound to {audience=vmctl, op=recover_current, ComputerID, owner, VMID, routeGen, expectedHead, recoveryGen, nonce, expiry} and single-use.
- Journal (outside the swapped directory, with file+parent fsync): `fenced → stopped → key_copied → staging → verified → swapped → booted → route_published → done`. Deterministic names include operation ID + recoveryGeneration (not wall-clock alone). Restart resumes the recorded phase or safely rolls back; repeated identical `idempotency_key` returns the existing receipt; conflicting target returns `409`.
- Fence: mark realization `recovering`, refuse/coalesce `resolve`/`resume`/`recover`/`refresh` and `pressure-reclaim` for that ComputerID while token is live.
- Stop Firecracker, verify dead (`vmmanager` wait), re-read canonical head for head-movement check; on mismatch re-verify or retry bounded, do not publish stale head.
- Secrets: attach quarantine image read-only to a trusted-guest recovery unit for the single-key copy (no host ext4 parse, see Secrets section).
- Quarantine `data.img → data.img.quarantine-<recoveryGen>-<opID>` (same filesystem, fsync file+parent) and stage sibling `data.img.staging-<recoveryGen>-<opID>` via `vmmanager` sparse allocation; quarantine retention is evidence until explicit `done` and capacity-gated (never prune the only rollback copy).
- Boot fresh with new epoch + recovery handoff under the token; guest replays canonical chain to its *final* head (may include boot lifecycle appends); host verifies live/replay equivalence, head == final corpusd head (not necessarily the captured head), effective `ComputerVersion` + frontend `serving_join` before route CAS. Keep prior realization/UI on mismatch — partial never greens.

### Proxy / BIOS

- `internal/proxy/computer_lifecycle.go` keeps `POST /api/computers/{id}/lifecycle/{rematerialize-from-tape,restore}` → guest when `active`; adds owner-authenticated fallback → `vmctl cold` only when inactive (first slice: `recover_current` only).
- `Desktop.svelte` `wake_current_computer`: after `resume`/`refresh` still gets `connection refused` on `:8085`, do **one** `cold-recover` `recover_current` then boot; surface `recovery.status=rematerializing` instead of looping `Bootstrap probe is still waiting`.

## Secrets continuity — the blocking correctness fix

Fresh `data.img` is blank-formatted (`vmmanager/manager.go:696`), but the privacy key lives inside it at `/mnt/persistent/choir-credentials/privacy-key` (`nix/autoputer-vm.nix:678`) and is deliberately not recreated when a canonical chain exists (`autoputer/run.go:204-213`, `computerevent/privacy.go:85-89`). Without it, private payload replay fails before `:8085` is healthy.

First-slice fix: a narrowly scoped **trusted-guest recovery unit** (same reviewed guest rootfs, no network/models/gateway/candidate execution) attaches the quarantined image read-only and the staging image writable, validates the source path is a regular file with expected mode/owner/ComputerID/key-version digest, copies **only** `{privacy-key}` (and, if inventory proves required before Definition execution, an enumerated `signer-state` — no general filesystem copy), fsyncs, then detaches before the main boot. This is not host ext4 parsing and violates no `rematerialize.go:16-20` `Staged cannot read Original` invariant because the check belongs to checkpoint-rematerialize, not `recover_current`. Full lost-image (unreadable quarantine) remains explicitly not supported — this is guest-down recovery, not destroyed-disk recovery — and is documented as not_done_when. Future key escrow (checkpoint-bound) is the durable fix.

## Multitenant security (hardened)

- Capability: per-computer, short-lived, audience-bound `read-events(computer_id)` only — no `append(EventRestoreRequested)` in this slice. Minted by proxy after ownership + `ComputerVersion` route guard; vmctl re-verifies independently. Never trust `X-Internal-Caller: true` alone (`vmctl/handlers.go:1203` is not auth).
- Per-ComputerID fencing token + lease (see vmctl section) prevents cross-tenant races and stale-resumption overwrite; `ComputerID + recoveryGeneration + canonicalHead + routeGen` is the idempotency identity, not wall-clock.
- Paths derived from registry only; symlink traversal rejected; containment enforced.
- Operation capability binds {audience, op, ComputerID, owner, VMID, routeGeneration, expectedHead, recoveryGeneration, nonce, expiry} and is single-use.
- Audit: every transition logs computer, head, final head, effective ComputerVersion, frontend identity, quarantine/staging paths, token; on mismatch keep prior realization/UI.

## Lease / boot-append deadlock fix

Normal boot replays `Reconstruct` (full canonical chain, `run.go:223`) then appends pending lifecycle/credential-revocation receipts before `:8085` (`run.go:226-271`); append failure is fatal. A blanket “refuse appends until route publication” therefore deadlocks the same liveness it tries to fix.

Fix: corpusd recovery lease allowlists **only** boot lifecycle appends from the realization bearing the current fencing token, captures the *final* head after those appends, and requires re-verification (live/replay equivalence + effective ComputerVersion + frontend join) against that final head before route CAS. A head movement by an unrelated writer is a concurrent conflict (fail/retry), not a success.

## Sequencing (fenced)

1. This design → consensus (approve with repairs) → Definition `choir-host-orchestrated-recovery-2026-08-22` — `recover_current` only.
2. Follow-on E2 Definition: add `authorized_checkpoint` mode, quorum verification, `internal/computerrestore` checkpoint engine, `append(EventRestoreRequested)` authority, and `.../lifecycle/restore` cold fallback. `99949fe2` stays untouched until scheduling Definition reaches E2.

## Rollback

Git revert of Definition commits. Quarantined `data.img.quarantine-*` retained as evidence (capacity-gated). Never rewind `corpusd` canonical events; a failed fresh boot rolls back via a supported fenced transition (stop new realization, reattach prior accepted realization's route or leave route safely unavailable with honest `recovery.status`), not by mere file existence.

## Open questions closed by these repairs

- Restore set: `data.img` + `privacy-key` (via attachment) are the only stateful members for `recover_current`; updater pins and `persist/host` are regenerated or evidence-only. Full inventory is now enumerated; any additional member discovered during rehearsal becomes a Definition amendment, not silent implementation drift.
- Shared engine: deferred to E2 checkpoint slice; first slice needs no `RematerializeFromRequest` extraction.
- Pre-boot witness: not applicable to `recover_current`; verification is post-boot `ReplayCompleteness` + live/replay equivalence + `ComputerVersion`/frontend before route publish.
