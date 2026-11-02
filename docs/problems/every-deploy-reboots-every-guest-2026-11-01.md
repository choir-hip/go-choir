# Every deploy that touches a vmctl or guest-adjacent path reboots every guest

Date: 2026-11-01. Class of the fix: red (vmctl / deployment routing). Rollback:
revert the commit; the classifier returns to path rules only.

## Problem

A staging deploy that changes vmctl, vmmanager or almost any Go package gives
every running guest a planned refresh (a reboot), even when nothing the guest
boots changed. On 2026-11-01 seven deploys meant seven paper reboots in 4.5
hours; each paper boot does a multi-GB read burst (the store open, the index
checks), which is the cost the guest isolation mission was measuring.

Owner rule that holds: a computer stays on while it is working
(CLAUDE.md, "Restarts End Work (Crash) Or Resume It (Planned Update)"). A
planned refresh is the right mechanism when the guest changed; it is wrong
when it did not.

## Evidence

Node B vmctl journal (read-only), 2026-10-31 18:01Z to 20:32Z: after each
vmctl restart, `POST /internal/vmctl/deploy-refresh caller="ci.deploy-refresh"`
followed by `vmmanager: destructive kill ... cause=refresh` and `vmctl:
refreshed VM` for each of the fleet candidate, the paper
(vm-1669020a...) and d@e: 18:01, 19:26, 20:06, 20:21, 20:31, and so on. The
deploy receipt at 61772f741 shows `autoputer` and `active_computers` both at
the deploy commit.

Guest content digest (below) walked over the non-docs commits of 2026-11-01:
these commits did not change what a guest boots, and the classifier refreshed
for the ones that touched `cmd/vmctl/*` or `internal/vmmanager/*`:
3d250b6f7, a30787be5, 6a9941f3e, d32e6c2c3, 5d6fd3100 (vmmanager and node-b),
and the node-b-only commits did not refresh on their own. Commits that did
move the digest (real guest changes) include the store/objectgraph read fixes
(8092c9dbf ... f7a560fb3), guestmem (8ab7b2da5), the guest memory ceiling
(54a12aa40), the drill route (ea13c6a67) and the epoch 29/30 raises.
internal/vmctl edits (a22d1a273, e8307302f) also move it, see Residual.

## Cause

Three layers, each over-approximate, none comparing what a guest runs:

1. `.github/scripts/deploy-impact-classify` decides `deploy_active_vm_refresh`
   from changed paths. `cmd/vmctl/*` and `internal/vmmanager/*` call
   `mark_guest_boot_contract`; so do `internal/vmctl/*`, the catch-all
   `cmd/*|internal/*` ("conservative Go impact"), `flake.nix`, any `nix/*` and
   the unknown-path default. `mark_guest_boot_contract` sets
   `deploy_host_os`, `deploy_vmctl_restart` and `deploy_active_vm_refresh`, and
   any selected `autoputer` service forces it too (the deploy script refuses an
   autoputer pointer deploy without a refresh and the reverse).
2. `.github/scripts/node-b-ci-deploy` then posts every active interactive
   computer to `/internal/vmctl/deploy-refresh`. Its only per-guest shortcut is
   "guest /health `build.commit` equals the autoputer commit just built". The
   commit is the repo SHA, stamped into every binary
   (`-X buildinfo.Commit=${buildCommit}` in `commonGoArgs`, `build.json`, the
   guest image), so it differs on every deploy and the shortcut never fires.
3. vmctl's `deployRefreshWith` (internal/vmctl/deploy_refresh.go) refreshes any
   active, unheld, non-divergent computer once the guest accepts the planned
   restart marker. It compares nothing: there is no guest release or content
   identity in the request. (`floor_digest` in `release.json` exists, but it
   covers only the floor commands and is used for snapshot wake validity, not
   for deploy refresh; the autoputer binary is outside the floor.)

Why a vmctl-only change reaches a guest: `cmd/vmctl` and `internal/vmmanager`
are not imported by any guest binary (`GOOS=linux go list -deps` over the guest
commands has no vmmanager, and cmd/vmctl is a main package), so they are not
in a guest closure at all. The path rule routed them to the guest anyway so
that host launch settings (jailer, cgroup limits) would reach running guests
immediately. Those settings apply at a guest's next boot; the cost of waiting
is that a running guest keeps its old launch shape until it next boots.

vmctl restart itself leaves guests running: the unit has `KillMode=process`
(nix/node-b.nix) and the new process reattaches running VMs (journal:
"managed VMs left running for reattach", "reattached VM ...").

## Fix (design)

A guest is refreshed when the content it boots changed.

- `scripts/guest-content-digest REV` hashes git objects at REV: the Choir
  packages the guest binaries import (`go list -deps`, linux, non-test, no
  testdata), go.mod/go.sum, flake.nix/flake.lock, frontend/ (not its tests),
  skills/, nix/ without the host-only files, the guest signer scripts. The
  security epoch, guest_abi and guest_abi_min are inside it
  (nix/guest-release-policy.json), so an epoch raise moves it. The commit id,
  build date and host-only files are not in it.
- The classifier, given the deploy base and head (as the CI job already does),
  compares the two digests. Same: `mark_guest_boot_contract` and
  `mark_active_vm_refresh` do nothing beyond marking the deploy needed, and the
  autoputer package pointer is dropped from the selected services, so the
  deploy script sees neither an autoputer pointer nor a refresh and its two
  consistency checks stay satisfied. Moved: the path rules decide as before.
  Digest unavailable (no base, no Go, a missing object): the path rules decide,
  which is the old conservative answer.
- `cmd/vmctl/*` and `internal/vmmanager/*` now ask for a vmctl restart
  explicitly and leave the guest refresh to the digest.
- Kept as they were: the guest-side busy and self-development refusals, the
  live-capsule refusal, the planned-restart marker and its single resume, the
  security epoch gate (CI) and its wake-time check, and the manual
  `force_staging_deploy` / `refresh_active_computers` dispatch inputs, which
  still force a refresh and are the way to apply a host launch-shape change to
  running guests on demand.

Not done: the commit stamp stays in the guest closure and image. With an
unchanged digest the autoputer pointer does not move, so guests keep running
the build they booted from; only its commit stamp is older than main, exactly
as after an auth-only deploy today. A stamp-free guest closure (version via a
host-passed sidecar) would remove the stamp but touches the build identity that
deploy verification and `/health` read; it is a larger change and not needed
for this residual.

## Residual

- `internal/vmctl` is linked into the guest binary (the route client and shared
  types: `vmctl.Client`, `ReportedRelease`, `GuestOwnership`), so an edit to any
  file in that package still moves the digest and still refreshes guests. Real
  fix: split the guest-visible types and client into their own package, after
  which the host controller can change freely.
- flake.nix and non-host-only nix files stay in the digest (they can change the
  guest image), so a host-only edit to flake.nix refreshes guests.
- The digest is a source-set approximation of the guest closure, over-counting
  on purpose; the tests pin both directions. Nothing here proves the built
  closure is identical; deploys that skip the refresh keep the old pointer.
