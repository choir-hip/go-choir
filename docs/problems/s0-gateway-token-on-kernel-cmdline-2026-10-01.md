# S0 finding — gateway token passes through the guest kernel command line

**Status:** recorded 2026-10-01 (source-observed; staging confirmation rides the
`s0a-gateway-token-visibility-2026-10-01.json` receipt). Owner: S1 security
floor. This record precedes any repair per problem-documentation-first.

## Observation

`internal/vmmanager/manager.go` `buildFirecrackerConfig` appends
`choir.gateway_token=<token>` to Firecracker `boot_args` (both the store-disk
and legacy branches, ~lines 1438-1440 and 1474-1476). The guest
`go-choir-extract-cmdline` unit reads it from `/proc/cmdline` into
`/run/go-choir-autoputer.env` (nix/autoputer-vm.nix:344-346).

`/proc/cmdline` is world-readable inside the guest: any process — including a
capsule workload that escapes its env scrub — can read the gateway token for
the lifetime of the boot. The token is also visible on the host in the
Firecracker process's `/proc/<pid>/cmdline` (root-only) and in the persisted
`fc-config.json` written 0644 under the VM state dir
(`launchFirecracker`, manager.go:1576).

The in-code comment (manager.go:1396-1399) claims the data disk cannot be
pre-seeded before first boot; that premise predates the per-VM
`PersistentDir` write at manager.go:689-694 which already writes
`gateway-token` to `persist/gateway-token` mode 0600 before launch.

## Why it matters

The metamission's `must_preserve` includes "provider credentials never enter a
capsule" and S1 names "gateway token scrubbed from child environments" as part
of the security floor. A world-readable kernel cmdline is a wider surface than
an env var: it survives env scrubbing, is readable by every UID in the guest,
and is captured in host-side artifacts (fc-config.json, ps output).

## Evidence class

Source inspection (exact lines above). The S0a receipt names
`cmdline_has_gateway_token` from the live guest's `/proc/cmdline` (presence
flag only — the token value is never emitted into evidence) plus
`/proc/self/environ` presence and file modes of `persist/gateway-token` and
`/run/go-choir-autoputer.env`.

## Repair direction (S1 scope, not S0)

Move the token out of `boot_args` entirely: the persist-volume
`gateway-token` file path already exists and is written before launch; the
extract unit can read it instead of `/proc/cmdline`. S1 owns the cutover plus
the child-env scrub; do not fix inside S0.
