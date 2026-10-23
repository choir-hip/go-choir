# Firecracker 1.17 as the host's pinned build, after deploy D4 (2026-10-22)

Status: problem record and plan, before code. Mutation class of the fix:
**red** (VM lifecycle: the VMM binary and jailer every computer launches).
Lane: `lane/fp0-firecracker-117-fleet`, on `lane/fp0-deployD4` (2de635109).
Related:
[guest-data-image-never-trims-2026-10-20.md](guest-data-image-never-trims-2026-10-20.md)
(RES-16; "Proof on one test computer"; "Every computer, the paper
included"),
[node-b-disk-fills-from-the-paper-data-image-2026-10-22.md](node-b-disk-fills-from-the-paper-data-image-2026-10-22.md),
[firecracker-1-16-1-virtio-mem-restore-bugs-2026-10-22.md](firecracker-1-16-1-virtio-mem-restore-bugs-2026-10-22.md),
[memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md](memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md),
and the FP8 definition (P3, and "NET-08 and RES-16").

## Problem

The owner asked on 2026-10-22, about 22:30 UTC, for Firecracker 1.17 on
every computer as soon as reasonably possible. At 22:40 the owner ruled
that resource use comes ahead of short-term availability ("Discard for the
paper is important. Let's accelerate that."). Deploy D4 (stage
`lane/fp0-deployD4`, 2de635109) does that through the per-computer second
build: `VM_FIRECRACKER_NEXT_ONLY=*` and `VM_DATA_DISCARD_ONLY=*`. Every
computer, the paper included, launches 1.17.0 from `nix/firecracker-next.nix`
with discard on its data drive.

After D4 the host still **pins Firecracker and jailer 1.16.2**:
`flake.nix` `choirFirecrackerRelease`, the bound `choirFirecrackerBelow =
"1.17"`, and `VM_FIRECRACKER_BIN` and `VM_JAILER_BIN` in `nix/node-b.nix`
(commit 23a67ff0e, deploy KF). No computer runs that build any more. The
staging seam (`internal/vmmanager/vmm_next.go`) is carrying the whole
fleet, and the next version would have no seam left to be staged through.
The pinned 1.16.2 path is also a trap: if the next selector is narrowed or
its binary is refused at load (a missing file, or a different file name),
those computers silently fall back to 1.16.2 with no discard. Each of them
then cold boots on a version the fleet has left.

The follow-up: make 1.17.0 the pinned build, retire `firecracker-next.nix`
and the `VM_FIRECRACKER_NEXT_*` lines, and keep discard for every computer.
Owner and FP0 constraint: **a computer already on 1.17 with discard must
keep the same launch-config hash and the same Firecracker banner**, so its
snapshot is not refused a second time.

### Trial and D4 results (reported by FP0)

computer-9648031f, in D3: 1.17.0 booted, the store opened and Texture
answered. Only its data drive got `Sync` and `discard`. The first trim
punched 1.67 GiB, and `/health` answered all 93 of its probes at 10 s
intervals across that trim. The guest-data-image record holds D4's own
cost, monitoring and rollback. Neither this lane nor any record shows a 1.17
hibernate and wake (`snapshot_resume` under `Firecracker v1.17.0`, FP8
NET-08 step 4). That is part of this lane's proof, because the whole point
of this change is that such snapshots stay loadable.

## What decides the build, and what a version change does

- **Pin and bound.** `mkChoirFirecracker` overrides nixpkgs' firecracker
  with `choirFirecrackerRelease`'s version, source hash and vendor hash. It
  does this only while nixpkgs is older than that version, so a later
  nixpkgs at or above the pin is taken as is. The assertion keeps the
  result below `choirFirecrackerBelow`, which stops a weekly nixpkgs bump
  from moving the host up a minor version without review. Patch releases
  on the pinned line still go through, as they did on 1.16.x.
- **Jailer.** `VM_JAILER_BIN` is `${pkgs.firecracker}/bin/jailer`, the same
  derivation as the VMM, so it moves with the pin. `VMCTL_JAILER=off`
  today, so no launch uses it yet (FP8 SEC-04 turns it on per computer
  later). The jail directory is named after the binary, `firecracker` in
  both versions.
- **Per-VM build record.** Each launch writes `vmm-build.json` in the VM's
  state dir: the binary path, the jailer and whether it was the next build.
  The snapshot's `firecracker_version` is the recorded binary's
  `--version` banner. A restore under another banner is refused before
  Firecracker starts (`validateSnapshot`, exact string compare, reason
  `firecracker_version_mismatch`). The snapshot is discarded and the
  computer cold boots. A 1.16.2 snapshot is never loaded by 1.17, and a
  1.17 snapshot is never loaded by 1.16.2.
- **Launch-config hash.** The snapshot also records the sha256 of the
  Firecracker JSON config. The config holds no binary path or version, but
  it does hold the data drive's `discard` and `io_engine`. So turning
  discard on or off for a computer also refuses its snapshot
  (`launch_config_mismatch`) and cold boots it once.
- **Discard gate.** `VM_DATA_DISCARD=1` takes effect only when the binary
  reports 1.17.0 or newer (`dataDiscardEnabledFromEnv`), and then only for
  the computers `VM_DATA_DISCARD_ONLY` names. An empty selector means every
  computer. A 1.16.x binary never receives the field, because Firecracker
  rejects unknown fields and the boot would fail.
- **Deploy classes** (`.github/scripts/deploy-impact-classify`). A change
  to `flake.nix` or to `nix/*` other than `node-b.nix`, `hardware.nix` and `disks.nix` counts as a guest
  boot contract change. That means a host switch, a vmctl restart (running
  Firecracker children survive it), and a **deploy refresh of every active
  computer**: a planned restart backed by a marker. An idle computer
  restarts now. A busy one is retried at its next idle moment. The paper
  and other `critical_protected` and `public_platform` computers take it
  while busy and resume their work once, but never during a
  self-development apply or while a capsule is live. A change to
  `nix/node-b.nix` alone is a host OS change plus a vmctl restart, with no
  refresh: a running computer keeps its VMM and config until its next cold
  boot.
- **Memory hotplug.** `VM_MEMORY_HOTPLUG_ONLY=class:test` is unchanged.
  The hotplug region is at 512 GiB in 1.15.1 through 1.17.0, so no
  computer can plug memory on Node B under either version. 1.17.0 also
  bounds hotplug sizes to 32 bits (#6076), and 4096 MiB is well inside
  that. No step here may count a hotplug failure against 1.17.
- **Other places that name a version.** `.github/workflows/patch-watch.yml`
  compares the declared version with upstream's latest. After this change they
  match (1.17.0 is the latest tag, checked with `git ls-remote`). The
  `scripts/proofs/fp8-image-jailer.sh` step 7 checks for the "next
  Firecracker build" log line, which no longer appears once this change removes
  the next build. The proof below covers what step 7 was for. Comments
  in `data_discard.go` and `nix/node-b.nix` describe 1.16.x as the pinned
  line.

### 1.17.0 against 1.16.2

The CHANGELOG at the v1.17.0 tag (source
`sha256-1HL13XL8CvwGTcm8uIJxgW31hUXXLZ8F2+jV7FEgxEY=`) lists every 1.16.2
fix: #6174 and #6176 (virtio-mem restore), #6086 and #6143 (the logger
deadlock on SIGPIPE), #6120 (CPUID before MSRs on 6.18 hosts) and #6100.
Its Deprecated and Removed sections are empty. Its additions are opt-in,
except two that every block device now offers the guest:
`VIRTIO_BLK_F_BLK_SIZE` and `VIRTIO_BLK_F_TOPOLOGY` (#6098). The
computer-9648031f trial booted with them. Moving to 1.17 drops none of
1.16.2's fixes.


### The pinned build is the build D4 launches

Raising `choirFirecrackerRelease` to 1.17.0, with the source and vendor
hashes from `nix/firecracker-next.nix`, gives the same store path as D4's
next build. `nix eval .#nixosConfigurations.go-choir-b.pkgs.firecracker.outPath`
gives `/nix/store/jbzvr6c965zyby7ldj73yi0zvibgd65b-firecracker-1.17.0`
after the change. D4's `VM_FIRECRACKER_NEXT_BIN` is under that same path,
which FP0 built on Node B before D3 and is in use there. Three things
follow:

- the deploy builds no Firecracker;
- the binary and the jailer that every computer runs after this change are
  byte for byte the ones it ran under D4, and a computer's
  `vmm-build.json` (written under D4 as `next: true`) names the same path;
- the banner is the same (`Firecracker v1.17.0`), so `validateSnapshot`
  accepts every snapshot taken under D4.

The discard decision is the same too. Under D4 it reads the next build's
gate (`FirecrackerNext.DataDiscard`: the flag is on and the binary is
1.17), and `*` selects every computer. Under this change it reads the
pinned build's gate (`DataDiscard`: the flag is on and the binary is 1.17),
and `*` still selects every computer. Every data drive gets `Sync` and
`discard` in both, and nothing else in the Firecracker config depends on
which build was chosen. So every launch-config hash is unchanged.
`TestPinned117KeepsTheLaunchConfigOfTheNextBuild` checks this.

## Plan

One change, one deploy, and no cold boot beyond the usual deploy refresh for
any computer that has already launched under D4 (every running computer is
refreshed by the planned restart below; none is refused a snapshot):

- `choirFirecrackerRelease` becomes 1.17.0 with the
  `firecracker-next.nix` hashes, and `choirFirecrackerBelow` becomes 1.18.
- `nix/firecracker-next.nix`, its import in `nix/node-b.nix`, and the
  `VM_FIRECRACKER_NEXT_BIN` and `VM_FIRECRACKER_NEXT_ONLY` lines are
  removed. `VM_DATA_DISCARD=1` and `VM_DATA_DISCARD_ONLY=*` stay.
- The Go seam stays (`vmm_next.go`, inert while `VM_FIRECRACKER_NEXT_BIN`
  is unset), so 1.18 can be staged on one test computer later with a fresh
  `firecracker-next.nix`.

Why remove the file rather than keep it: its own header says to drop it
when the host moves to 1.17. Kept, it would be a second copy of the
pinned hashes. It would override a package that is already 1.17.0, and its
`*` selector would launch the same binary under a "next" label.

### What restarts

- The host switches and vmctl restarts. There is no host reboot (the host
  kernel is unchanged), and running Firecracker children survive the vmctl
  restart.
- `flake.nix` and `nix/*` changes are guest boot contract changes
  (`.github/scripts/deploy-impact-classify`). So **the deploy refreshes
  every running computer** with the planned, marker-backed restart, as
  every code deploy does. Idle computers restart now and busy ones at their
  next idle moment. The paper and other `critical_protected` or
  `public_platform` computers restart while busy and resume their work
  once, but never during a self-development apply or while a capsule is
  live. Each refresh is a cold boot on the same binary with the same
  config. Landing this together with the next code deploy costs nothing
  extra.
- **A hibernated computer with a D4 snapshot resumes it**: the banner and
  the launch-config hash both match.
- A hibernated computer still holding a 1.16.x snapshot from before D4
  cold boots once at its next wake (`firecracker_version_mismatch`). That
  is D4's cost, not this change's, and it is paid only once.

### Rollback

**Either rollback needs the 1.16.2 store path on Node B**
(`/nix/store/5nh9p1bhzhljbshbp8dibb5qd4qz1nxb-firecracker-1.16.2`, in no
public cache). Reverting the pin fix puts `VM_FIRECRACKER_BIN` back on that
path, `IsFirecrackerAvailable` stats it
(`internal/vmmanager/config.go:264-279`), and vmctl stops with "Firecracker
is mandatory" (`cmd/vmctl/main.go:145`, `:167`) if it is gone. After this
change no host generation needs the path, so the deploy preflight's reclaim
may collect it, and a missing path means a cargo build on Node B before
either rollback can deploy. So the GC root is a **deploy precondition**
(see the deploy note), not an option.

- Back to D4's shape (1.16.2 pinned, 1.17 next for `*`, discard `*`):
  revert the pin fix commit and redeploy. Every computer is back on the next
  build with the same binary, banner and discard, so no snapshot is
  refused. Running computers take the usual deploy refresh.
- All the way back to 1.16.2 with no discard: revert the pin fix commit,
  then delete all four lines in `nix/node-b.nix` (`VM_FIRECRACKER_NEXT_BIN`,
  `VM_FIRECRACKER_NEXT_ONLY`, `VM_DATA_DISCARD`, `VM_DATA_DISCARD_ONLY`), as
  D4's own rollback says, and redeploy. Reverting D4's selector commit
  (2de635109) is not a rollback: it only narrows both selectors back to
  `computer-9648031f…`, which leaves the four lines and so keeps 1.17 and
  discard on that computer (checked against `2de635109^:nix/node-b.nix`
  lines 1556 to 1559). With the four lines gone every computer cold boots
  once more on 1.16.2, because 1.16.2 refuses its 1.17 snapshot. The
  guest-data-image record has D4's own rollback.

### Proof (FP0, after the deploy)

0. Before: the GC root for the 1.16.2 path exists on Node B (deploy
   precondition above). `scripts/check-host-firecracker-pin.sh` passes on the lane head
   with `EXPECT_OUT=/nix/store/jbzvr6c965zyby7ldj73yi0zvibgd65b-firecracker-1.17.0`.
   Record `sha256sum fc-config.json` for the paper and one test computer
   that run under D4, and the snapshot meta of one hibernated D4 computer.
1. On Node B, `VM_FIRECRACKER_BIN` and `VM_JAILER_BIN` (from `systemctl show
   go-choir-vmctl -p Environment`) are under `jbzvr6c9…-firecracker-1.17.0`,
   and no `VM_FIRECRACKER_NEXT_BIN` is set. The vmctl journal has no "next
   Firecracker build" line and no "VM_DATA_DISCARD is on but" line.
2. The same launch config: the paper's and the test computer's
   `fc-config.json` hashes after their refresh equal the ones from step 0.
   Their data drives have `Sync` and `discard`.
3. No second refusal: the hibernated D4 computer from step 0 wakes with
   `snapshot_resume` under `Firecracker v1.17.0`. After the deploy time, no
   wake shows `fallback_cold` with `firecracker_version_mismatch` or
   `launch_config_mismatch` for a snapshot taken after D4.
4. One 1.17 snapshot cycle: hibernate a test computer by snapshot, then
   wake it, and expect `snapshot_resume`.
5. The paper: `/health` is ok after its refresh, its hourly
   `choir-data-fstrim` reports bytes trimmed, and its Texture work and
   Autopaper polls continue.

### Steps B and C of the first draft

The first draft of this record staged discard over three deploys: A pins
1.17 with discard on one computer, B adds `class:test`, C sets `*`. D4
replaced B and C on the owner's ruling, so this lane has only the pin.

## Review findings (bunch 144), before any fix

Three reviewers accepted the forward pin: Gemini ADOPT, Devin APPROVE,
DeepSeek ACCEPT, the last conditional on the first three findings below.
The runtime change itself drew no finding. The defects are in this record's
rollback text and in the proof tooling.

1. MEDIUM (DeepSeek): the "all the way back to 1.16.2" rollback is wrong.
   It says to revert D4's selector commit (2de635109) in addition to the
   pin fix. That commit only narrows the two selectors from `*` back to
   `computer-9648031f…`; `git show 2de635109^:nix/node-b.nix` has all four
   lines (`VM_FIRECRACKER_NEXT_BIN`, `_NEXT_ONLY`, `VM_DATA_DISCARD`,
   `VM_DATA_DISCARD_ONLY`) at lines 1556 to 1559. After that revert the test
   computer would still run 1.17 with discard, and its 1.17 snapshot would
   resume. D4's own commit message gives the right rollback: delete the four
   lines and redeploy.
2. MEDIUM (DeepSeek): the rollback to D4's shape also needs the 1.16.2
   path. Reverting the pin fix restores `VM_FIRECRACKER_BIN` to
   `…5nh9p1bh…-firecracker-1.16.2/bin/firecracker`
   (`2de635109^:nix/node-b.nix:1453` is `${pkgs.firecracker}` on the 1.16.2
   pin). `IsFirecrackerAvailable` stats that path
   (`internal/vmmanager/config.go:264-279`), and vmctl then stops with
   "Firecracker is mandatory" (`cmd/vmctl/main.go:145` and `:167`). So the
   record's "needs only the 1.17.0 path" is false: the 1.16.2 path is
   needed for either rollback, and the record makes the GC root optional.
3. LOW (DeepSeek, Devin): `scripts/proofs/fp8-image-jailer.sh` step 7
   (lines 32, 130 and 142) still tells the operator to fill
   `nix/firecracker-next.nix` (deleted here), to set
   `VM_FIRECRACKER_NEXT_ONLY` (removed here), and requires a "next
   Firecracker build" journal line that no longer appears. The deploy note
   says to read it as done, but the script is executable and unannotated,
   and run as written it fails.
4. LOW (DeepSeek): `scripts/check-host-firecracker-pin.sh` line 45 treats
   only the exact string `1` as discard on. The Go side (`envFlagOn`,
   `internal/vmmanager/config.go:244-249`) accepts `1`, `true`, `on` and
   `yes`, case-insensitively. A later `VM_DATA_DISCARD=on` with an empty
   `VM_DATA_DISCARD_ONLY` would enable discard for every computer and pass
   the guard.
5. NIT (Devin): `sort -V` in that script is GNU only, and `jq` is used
   without a preflight. Both fail closed, but the message would be unclear.
6. Risk note (DeepSeek): "no new cold boot" in the Plan and the deploy note
   can be read as "no cold boot at all". The deploy refresh still cold boots
   every running computer. The claim means no cold boot beyond that refresh.

Dispositions (fix round, bunch 144):

1. Fixed (docs). The Rollback section now says: revert the pin fix, then
   delete all four lines and redeploy. Reverting 2de635109 is named as not
   a rollback, with the reason.
2. Fixed (docs). The Rollback section and the deploy note state that the
   1.16.2 path is needed for either rollback, and the GC root on Node B is a
   deploy precondition that FP0 adds by hand before the deploy. One
   correction to the finding's citation: the "Firecracker is mandatory"
   `log.Fatal` is at `cmd/vmctl/main.go:167` (reached when
   `IsFirecrackerAvailable`, called at `:145`, is false); lines 145 to 146
   are the call and the config validation. The substance stands.
3. Fixed. Step 7 of `scripts/proofs/fp8-image-jailer.sh` is rewritten for
   the pinned build, rather than marked done: the rewrite was smaller than
   explaining why an executable step must not be run. It checks discard on
   the target's data drive and that no "next Firecracker build" line
   exists; the manual lines no longer name `firecracker-next.nix` or a
   selector. `--list` and `--step 7 --dry-run` ran with ssh and curl
   replaced by `false`.
4. Fixed. The guard in `scripts/check-host-firecracker-pin.sh` trims,
   lowercases and accepts `1`, `true`, `on`, `yes`. The new
   `scripts/check-host-firecracker-pin-test.sh` runs the check against a
   fake `nix` over 13 cases; six of them failed on 1016fc951 and all pass
   now. The real check still passes on the lane with `EXPECT_OUT`.
5. Fixed. The script notes that `sort -V` is GNU coreutils and fails with
   "jq is required" when jq is missing.
6. Fixed (docs). The Plan and the deploy note say "no cold boot beyond the
   usual deploy refresh", and that the refresh cold boots every running
   computer.

The other DeepSeek notes need no change: 1.17.x patch releases through the
weekly bump are the existing residual below, and "no host generation needs
the 1.16.2 path" is true of the new generation, though an older generation
in the profile may keep the path alive until it is pruned (the GC root
makes that irrelevant).

## Ceremony

- Mutation class: **red**. The protected surface is the VMM binary and
  jailer for every computer (VM lifecycle). Snapshot validation, vmctl
  lifecycle code, the guest image, Texture, auth and routing are not
  touched.
- Admissible evidence: local `nix eval` of the host (version, store path,
  environment, assertions), `nix-instantiate --parse` of the changed
  files, and focused `internal/vmmanager` tests. The deployed proof is the
  list above, on Node B.
- Heresy delta. Discovered: after D4, the pinned 1.16.2 build is a silent
  fallback that no computer should take. Introduced: none. Repaired: the
  host runs one Firecracker build again, and it is the pinned one.

## As built (lane/fp0-firecracker-117-fleet, on D4 2de635109)

- b42a29c10: this record (green).
- a92c7c380 (yellow), two tests:
  - `scripts/check-host-firecracker-pin.sh`, an evaluation of the host that
    builds nothing. It fails on 2de635109: "host Firecracker is 1.16.2,
    expected 1.17.0".
  - `TestPinned117KeepsTheLaunchConfigOfTheNextBuild` and
    `TestPinnedDiscardBuildNamesOnlyTheSelectedComputer`, in
    `internal/vmmanager/vmm_next_test.go`. Both pass on 2de635109, because
    the Go seam already handles both shapes. They pin the selection
    semantics this change relies on. Two mutations were checked by hand:
    turning off the pinned build's discard gate makes the first fail, and
    an empty discard selector makes the second fail.
- 70c82a1eb (red), the change itself:
  - `flake.nix`: `choirFirecrackerRelease` is 1.17.0 and
    `choirFirecrackerBelow` is 1.18.
  - `nix/firecracker-next.nix` is deleted, along with its import in
    `nix/node-b.nix` and the `VM_FIRECRACKER_NEXT_BIN` and `_ONLY` lines.
  - `VM_DATA_DISCARD=1` and `VM_DATA_DISCARD_ONLY=*` stay.
  - Comments in `flake.nix`, `nix/node-b.nix`, `data_discard.go` and
    `vmm_next.go` are updated. No Go code changed.

Review fix round (bunch 144), on 1016fc951:

- c67700a02 (green): the findings, before any fix.
- f67998525 (yellow): `scripts/check-host-firecracker-pin-test.sh`; six of
  its 13 cases fail on 1016fc951.
- a90c3d870 (yellow, proof tooling only): the guard's truthy set, the jq
  preflight, the `sort -V` note, FP8 step 7.
- This record's Rollback, deploy note and dispositions (green).

Local verification at 70c82a1eb:

- `nix-instantiate --parse` succeeds for `flake.nix` and `nix/node-b.nix`.
- `EXPECT_OUT=/nix/store/jbzvr6c965zyby7ldj73yi0zvibgd65b-firecracker-1.17.0
  scripts/check-host-firecracker-pin.sh` prints "ok: host Firecracker
  1.17.0 at /nix/store/jbzvr6c9…-firecracker-1.17.0 (VMM and jailer),
  next=<unset>, discard_only=*".
- `nix eval` of `config.system.build.toplevel.drvPath` evaluates, so every
  host assertion holds.
- The focused `go test ./internal/vmmanager -run 'TestPinned|…FirecrackerNext|TestDataDiscard|TestVMMFor|TestLaunchedVMM|…DataDiscard…|TestResumeRefusesSnapshotFromOtherFirecrackerPatch'`
  passes.

## Deploy note

- Land after D4 has deployed, or together with it. It changes `flake.nix`,
  so it is a host switch, a vmctl restart, and a planned deploy refresh of
  every running computer. It builds no Firecracker: Node B already has the
  1.17.0 path.
- Cold boot cost for computers that launched under D4: none beyond the
  usual deploy refresh (that refresh cold boots every running computer, as
  above). Their binary, banner and launch config are
  unchanged, so a hibernated D4 snapshot resumes. A computer still holding
  a 1.16.x snapshot from before D4 cold boots once, which is D4's cost.
- If the change lands without D4, every computer moves from 1.16.2 with no
  discard to 1.17.0 with discard at once. That costs the same single cold
  boot per computer that D4 costs.
- **Deploy precondition (FP0, by hand, before the deploy): add a GC root on
  Node B for
  `/nix/store/5nh9p1bhzhljbshbp8dibb5qd4qz1nxb-firecracker-1.16.2`**, and
  confirm the path exists. Both rollbacks restore a `VM_FIRECRACKER_BIN`
  under it (see Rollback). No host generation needs that path after this
  change, so the deploy preflight's reclaim may collect it, and vmctl
  would not start on a rollback deploy. Also confirm the 1.17.0 path
  (`…jbzvr6c9…-firecracker-1.17.0`) exists.
- `scripts/proofs/fp8-image-jailer.sh` step 7 is rewritten for the pinned
  build: it checks discard on the target's data drive and the absence of a
  "next Firecracker build" line, and no longer names `firecracker-next.nix`
  or a selector.
- Residual: after this change, the next minor version has to be staged with
  a new `nix/firecracker-next.nix`. Patch releases on 1.17.x from a weekly
  nixpkgs bump still reach the host with no review, because the bound
  stops only 1.18. Each one changes the banner and costs every computer one
  cold boot, as patch releases did on 1.16.x.
