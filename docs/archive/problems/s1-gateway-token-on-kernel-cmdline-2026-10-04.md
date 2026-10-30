# S1 remainder: autoputer gateway token rides the guest kernel cmdline

Date: 2026-10-04
Status: fix in flight (problem documented before code, per AGENTS.md)
Mutation class of this record: green (documentation only).
Mutation class of the fix it gates: red (VM credential handoff + guest boot
path + runtime env).

## Problem

`vmmanager` passes the per-VM autoputer gateway token as a kernel cmdline
parameter (`choir.gateway_token=…`) in `buildFirecrackerConfig`
(`internal/vmmanager/manager.go`, both the store-disk and legacy-rootfs
branches). Inside the guest, `/proc/cmdline` is world-readable, so every
process — including untrusted terminal/zot children and capsule workloads —
can recover the autoputer identity token that authenticates to the host-side
gateway.

The metamission records this as a confirmed staging finding
(`choir-supervised-app-development-metamission-2026-10-01.md` security
detail: "The gateway token is on the guest kernel cmdline and readable
inside the guest") and lists its removal under the S1 remainder ("gateway
token off the kernel cmdline and out of child environments").

## Evidence

- `internal/vmmanager/manager.go`: `choir.gateway_token=` appended to
  `bootArgs` in the store-disk branch (~line 1573) and legacy branch
  (~line 1609).
- `nix/autoputer-vm.nix`: `go-choir-extract-cmdline` maps
  `choir.gateway_token=*` to `RUNTIME_GATEWAY_TOKEN` in
  `/run/go-choir-autoputer.env` (~line 450).
- Consumers read only the env var: `internal/autoputer/run.go:184`,
  `internal/autoputer/terminal.go:251`, `internal/search/search.go:74`.
- `/proc/cmdline` is mode 0444; no mount option removes it for guest
  children.

## Substrate check (per AGENTS.md)

An existing replacement channel exists and is already used for a stronger
credential: the per-realization `credential.img` ext4 disk that carries the
mode-0400 `computer-event-envelope` to `/run/choir-bootstrap`
(`createCredentialDisk`, `internal/vmmanager/manager.go`). The token can
ride the same disk as a second mode-0400 file instead of a new mechanism —
deletion of the cmdline channel, not a new delivery system.

## Fix shape

1. `createCredentialDisk` gains a gateway-token parameter and writes
   `gateway-token` mode-0400 next to the envelope.
2. Both `bootArgs` branches stop emitting `choir.gateway_token=`.
3. The autoputer reads the token through
   `RUNTIME_GATEWAY_TOKEN_FILE=/run/choir-bootstrap/gateway-token`
   (set in `nix/autoputer-vm.nix`); a `provideriface` helper resolves
   file-first, then `RUNTIME_GATEWAY_TOKEN` for non-guest/dev runs.
4. `go-choir-extract-cmdline` drops the `choir.gateway_token` case so a
   legacy vmmanager cannot reintroduce it through a stale cmdline.

## Out of scope (named residual)

Child-environment inheritance: `internal/autoputer/terminal.go` injects the
same token as `OPENAI_API_KEY` into zot sessions, and non-capsule children
inherit the unit's environment. The acceptance action ("token absent and
unrecoverable from children") needs the non-root-runtime + child-env slice
that follows; this slice only removes the cmdline channel and stops putting
the token in the unit environment (file-based read keeps it out of
`os.Environ()` inheritance for children that spawn after env-file changes).

## Rollback

Git revert of the fix commit; restore the prior guest base image through
the pinned-head path. New and legacy guest boots both keep working with the
env-var path retained as a fallback.
