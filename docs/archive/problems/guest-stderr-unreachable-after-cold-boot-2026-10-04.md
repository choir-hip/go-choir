# Guest service stderr is unreachable after cold boot — no serial sink, no journal surface

**Date:** 2026-10-04
**Status:** open. Confirmed on staging during S2 layering acceptance.
**Mutation class of this record:** green (problem documentation only).
**Station receiving it:** S2 (in-flight), then a substrate repair before any
later station that must debug in-guest behavior — S0b probes, S3 resume, S4
capsule exec, S6 rollback — because each one will need the same visibility and
currently cannot have it.

## Evidence

The `autoputer.service` unit already declares
`StandardOutput=journal+console` / `StandardError=journal+console`
(`nix/autoputer-vm.nix` serviceConfig), so guest runtime + wrapper stderr
*should* be observable end-to-end. It is not:

- **No serial sink on disposable/interactive VMs.** `fc-config.json` has
  `logger: None`, `metrics: None`, `vsock: None` — console output has nowhere
  to land, so `journal+console` writes are dropped.
- **The host-relayed `journal_events` covers only the cold-boot window.**
  `vmmanager`'s `boot_tap`/`journal_events` buffer fills during the first
  `systemd` bring-up (~first 30 unit lines) and is empty for every later
  event — including the `choir-updater` apply-driven `go-choir-autoputer`
  restart. After the boot timeline closes, no guest journal is reachable from
  the host at all.
- **No guest-side query surface.** The guest runs no SSH and no journal
  endpoint; `/internal/boot/timeline` reports `journal_events` from that same
  cold-boot buffer only, and the autoputer's own stderr is not in it.

**Cost already paid:** the S2 layering apply materialized a release, set
`layering-entrypoint`, and restarted `go-choir-autoputer` — and the service
came up on the *base* binary with `exec_resolved` pointing at the immutable
EROFS path, not the applied release. The layering wrapper's own `echo`
diagnostics never reached any host-visible surface. Diagnosing why the
`unshare -m`/`mount -t overlay`/`exec` chain fell back required adding a
`layering-diag.log` file write inside the wrapper (`e605cdde` → `3c1cbaf6`)
because the journal path was a dead end — a full image rebuild + redeploy +
re-probe cycle just to read one stderr line.

## What it breaks downstream

- **S0b probes** run disposable experiments whose whole value is guest-side
  evidence; a guest that cannot report its own stderr makes every negative
  result ambiguous (did the mechanism fail, or did the probe wedge?).
- **S3 resume** needs to distinguish "resume succeeded but runtime is slow"
  from "the resumed process image is dead"; today those look identical.
- **S4 capsule exec** needs capsule-side output; same visibility hole.
- **S6 rollback / S11 security push** need to prove a release actually
  started (or failed) — `exec_resolved`/`exec_id` already exist but only
  reach the host when the process lives.

## Candidate fix shapes (no decision yet)

1. Attach a Firecracker **serial log file** (`logger`/`serial` to a
   per-VM `vm-state/serial.log`) at boot for every VM class, not just
   debug builds — journal+console then lands on a host-readable file.
   Cheapest; the plumbing (fc-config `logger`) already exists in the
   Firecracker schema, vmmanager just does not populate it.
2. Extend `boot_tap`/journal relay to keep streaming past first-healthy
   (or add a guest `GET /internal/journal` endpoint) so the timeline's
   `journal_events` is not a cold-boot-only artifact.
3. A `vmctl` guest-exec / journal pull for interactive computers — heavier,
   but gives shell-level inspection when the service itself is down.

Option 1 is the substrate fix and unblocks every later station; 2 and 3 are
additive conveniences.
