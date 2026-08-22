# Fix Evidence: Trusted-Guest Privacy-Key Copy Wired for recover_current

- Date: 2026-08-22
- Mutation class: Red (repair)
- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Checkpoint fence: `99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7` (untouched)
- Effects: OFF

## Problem

`docs/evidence/effects-red-recovery-trusted-guest-copy-authority-2026-08-22.md` observed that `cmd/vmctl/main.go` wired storage, headReader and verifier but not `TrustedGuestKeyCopier`, so `HandleColdRecover` returned `503 cold recovery dependencies are unavailable`.

## Fix

- `internal/vmctl/trusted_guest_copier.go`: New `TrustedGuestCopier{StateRoot}` implements `TrustedGuestKeyCopier`. It validates the fencing token (audience/operation/ComputerID/OwnerID/VMID/generation/head/nonce/expiry), validates quarantine/staging paths are under `StateRoot/VMID` with deterministic `data.img.quarantine-*` / `data.img.staging-*` names, rejects symlinks, checks regular files, detects plain-JSON vs ext4 by magic, extracts the single privacy-key via `debugfs cat` or plain read, validates JSON binding (`version==1`, `computer_id` matches token, `key` base64, canonical), writes to staging via `debugfs write` or plain atomic replace with `0400`, verifies by re-read, and durably syncs file and parent. It never mounts the guest filesystem on the host and copies only the single file.

- `internal/vmmanager/manager.go`: Added `StateDir() string` getter for wiring.

- `cmd/vmctl/main.go`: After `SetColdRecoveryStorage(mgr)`, wires `SetTrustedGuestKeyCopier(vmctl.TrustedGuestCopier{StateRoot: mgr.StateDir()})` and logs `recover_current trusted guest copier configured`. With `VMCTL_CORPUSD_URL` also wires `HTTPRecoveryHeadReader` and `HTTPRecoveryVerifier`.

- `internal/vmctl/cold_recover.go`: Extended journal to carry `IdempotencyKey`, `CanonicalHead`, `RouteGeneration`, `FinalHead`, `Status`; added `coldRecoveryStorage` interface, `StateDir` handling, `readExistingColdRecoveryJournal` for crash-resume, fixed `RecoverVMForDesktop` boot path, and added durable `verified`/`route_published`/`done` phases.

## Verification

- `go vet ./internal/vmctl ./internal/vmmanager ./internal/proxy ./internal/platform ./cmd/vmctl` passes.
- `go test ./internal/vmctl -run TestTrustedGuestCopier -count=1` passes: plain-file copy preserves `computer_id`/`key`, mode `0400`, rejects cross-VM.
- `go test ./internal/vmctl ./internal/vmmanager ./internal/proxy ./internal/platform -count=1` passes.
- Frontend `npm run build` passes (BIOS one-shot `cold-recover`).
- Manual `debugfs` probe: quarantine ext4 `cat` and staging `write` succeed on Nix `e2fsprogs` (`/nix/store/qmx82l4z674298hczh0klqb0rygnabsg-e2fsprogs-1.47.4-bin/bin/debugfs`), fallback to plain JSON for test fixtures.

## Remaining

Staging deploy and owner product-path recovery proof on `0333528` still required to close `docs/definitions/choir-host-orchestrated-recovery-2026-08-22.md`. The new authority is local-only until CI/deploy verifies it.

## Rollback

Revert this commit and the wiring commit `45eb523e`; handler reverts to `503` and no data image is mutated.
