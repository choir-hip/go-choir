# Staging Evidence: recover_current Has No Trusted-Guest Privacy-Key Copy Authority

- Date: 2026-08-22
- Mutation class: Red
- Computer: `computer-03335285269bdba4f94377e56879e9f6`
- Existing checkpoint fence: `99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7` (untouched)
- Effects: OFF

## Observation

The local `recover_current` implementation reaches a deliberate
`TrustedGuestKeyCopier` boundary after whole-file quarantine and sparse staging.
`cmd/vmctl/main.go` wires the VM-manager storage operations and, when
`VMCTL_CORPUSD_URL` is set, the corpusd head reader and post-boot verifier. It
does not wire a `TrustedGuestKeyCopier`; therefore the handler returns
`503 cold recovery dependencies are unavailable` instead of attempting a
filesystem copy.

Repository inspection found no existing trusted copy authority:

- `internal/vmctl/cold_recover.go` has only the interface and setter.
- `internal/vmmanager/manager.go` supports normal `data.img`/credential-drive
  boot, but no drive hotplug, vsock command, or recovery-unit copy operation.
- `nix/autoputer-vm.nix` mounts `data.img` as `/mnt/persistent` and reads
  `/mnt/persistent/choir-credentials/privacy-key`; it has no recovery copy
  service or guest command.
- Existing guest endpoints expose replay completeness, lifecycle, and
  observability, not a privacy-key export/import operation.

The absence was found before any staging deploy or host filesystem workaround.
No `data.img` was mounted or parsed by the host. No event was appended or
rewound.

## Belief and remaining error

Belief: whole-file quarantine plus a narrowly scoped trusted guest unit remains
sufficient, but the trusted unit is not yet an implemented product authority.
Remaining error: the recovery unit must attach quarantine read-only and staging
read-write, validate the computer/key binding, copy only
`/mnt/persistent/choir-credentials/privacy-key`, fsync, detach, and expose a
fenced vmctl completion result without adding arbitrary host event-write
authority.

## Required next probe

Design and implement the trusted recovery-unit contract (guest command/vsock
or equivalent reviewed product path), wire it into vmctl, add fault and
multitenant tests, then run a staging refusal proof before attempting the
stopped → active recovery. Until that authority is configured, no recovery
endpoint is claimed deployed and no staging acceptance is claimed.

## Rollback / non-goals

Rollback is the git revert of the documentation and any later recovery-unit
commit. The existing canonical event chain and `99949fe2` remain untouched.
Do not solve this observation with host ext4 parsing, `debugfs`, loop mounts,
SQL replacement, SSH mutation, arbitrary filesystem copying, a platform-only
recovery actor, or effects activation.
