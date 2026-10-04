# S1 remainder: guest runtime runs unconfined root; children share its UID and env

Date: 2026-10-04
Status: partially repaired; non-root runtime + child-UID boundary open.
Mutation class of this record: green (documentation only).
Mutation class of the remaining fix: red (guest runtime unit + credential
handoff + child spawn boundary).

## Problem

`go-choir-autoputer` runs as unconfined root
(`nix/autoputer-vm.nix` `systemd.services.go-choir-autoputer` has no
`User=`; only `ReadWritePaths`/`InaccessiblePaths` hardening). Two
consequences the S1 station names:

- Every terminal/zot child inherits the autoputer's UID and (until the
  scrub commits below) its environment, so a token file readable by the
  autoputer is readable by its children — DAC cannot separate them.
- A `readComputerCredentialEnvelopeOwned(path, 0)` check pins the
  bootstrap envelope to root ownership (`internal/autoputer/run.go`),
  so a non-root autoputer cannot consume it without a delivery change.

## Evidence

- `nix/autoputer-vm.nix:764-873` — unit without User=, broad
  `ReadWritePaths=["/mnt/persistent" ...]`.
- `internal/autoputer/terminal.go` — plain `exec.Command`, no
  `SysProcAttr`/`Credential`; children run with the autoputer's UID/GID.
- `internal/autoputer/run.go:763-802` — envelope read hard-checks
  `expectedUID=0`.
- `internal/capsule/executor.go:382-425` — capsule broker spawned with
  `CLONE_NEWUSER` mapping container uid 0 to host uid 65534 and ambient
  caps (CAP_SYS_ADMIN, SETUID, SETGID, ...). Capsule workloads already
  run under a distinct host uid and mount namespace (bootstrap credential
  disk invisible), so capsules are not the gap; terminal/zot children are.
- `nix/autoputer-vm.nix` sets no `kernel.yama.ptrace_scope`; at scope 0 a
  same-UID child could ptrace the runtime. Must be pinned (1 or 2).

## Repaired 2026-10-04

- `619d6458` — token off cmdline onto root-only credential disk;
  `RUNTIME_GATEWAY_TOKEN_FILE` file-read via `provideriface.GatewayToken()`.
- `a80d2146` — zot PATH-shadowing fallback removed.
- `98cf3875` — diag tcp-dial oracle restricted to host peer.
- `6b54522b` — OPENAI_API_KEY injection removed; `terminalChildEnv()`
  strips TOKEN/SECRET/KEY/CHOIR_* from terminal + shell children.

## Remaining shape (sketch from scout mapping)

- `User=choir-autoputer` on the unit with
  `CapabilityBoundingSet`/`AmbientCapabilities` scoped to what the capsule
  executor actually needs (SETUID/SETGID/CHOWN for namespace setup;
  CAP_SYS_ADMIN only if the overlay exec path still mounts). Systemd
  raises ambient caps across the User= setuid, so a non-root autoputer
  can still spawn the userns broker.
- Ownership migration for `/mnt/persistent/{state,files,go,go-build-cache,
  capsule-artifacts}` and `/run/choir{,-runtime-handoff}` via tmpfiles
  `z`/on-boot migration service; split autoputer-private
  `choir-updater` subtree from the root updater's release tree.
- Token + envelope delivery to the non-root unit: group-readable copies
  (root:choir-autoputer 0440) or the extract-cmdline oneshot projects
  them; `readComputerCredentialEnvelopeOwned` relaxes `expectedUID`.
- Terminal children: spawn under a distinct uid (nobody-class) via a
  setuid-capable boundary or reuse the capsule executor; `CHOIR_` env
  stays scrubbed (done).
- `kernel.yama.ptrace_scope=2` (or verified non-zero) pinned in the image.

## Rollback

Revert the unit/ownership change; the env scrub and credential-disk
delivery are independent of the runtime UID and can stay.
