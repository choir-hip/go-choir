---
definition_version: 4

# Station FP9 of the launch first pass (batch B9, keys and recovery): SDC
# NET-06 (data at rest, review F12), NET-09 (keys behind policy signers;
# passkey-bound owner acts) and launch checklist L2-04 (backup ready for
# launch). Run as `/goal docs/archive/definitions/choir-launch-fp9-keys-recovery-2026-10-21.md`.
# Authority: the owner's statements quoted in the metamission
# (docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md) and its
# FP9 entry; the worker rules sent with the lane. This is a worker lane: it
# builds and proves locally, FP0 lands, deploys and runs live proofs.
readiness: drafted

review:
  reviewer: none yet
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T19:22:29Z
  source:
    canonical_ref: origin/main 0cb66b4cb (lane/b9-keys is based on it)
    deploy_identity: unknown (a lane worker does not call a live host; FP0 holds the staging identity)
  worktrees:
    - path: lanes/b9-keys of the session scratchpad (branch lane/b9-keys)
      status: clean at HEAD 0cb66b4cb before this file
      class: goal_candidate
      owner: FP9 worker
      touch: goal_owned
      recovery: branch lane/b9-keys
    - path: /Users/wiz/go-choir (the main checkout)
      status: dirty (untracked cmd/desktop/frontend/, ograph-bench, sourcecycled)
      class: user_wip
      owner: the owner or another session
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      Snapshot hibernate is on in production (nix/node-b.nix:1349, 36 hour
      age, 48 GiB budget). A snapshot's `mem` file is the guest's RAM and
      lives at <vm-state>/<vm>/snapshot on the main btrfs file system with no
      encryption layer (problems/data-at-rest-snapshots-images-escrow-key-
      plaintext-2026-10-21.md, evidence list).
    - >-
      data.img is created 0644 in a 0750 directory (manager.go:842, :2997);
      the escrow private key is a plaintext base64 file read by corpusd and
      checkpointd (keyescrow.go:68-111); corpusd generates a new key when
      the file is absent (cmd/corpusd/main.go:159); the daily backup holds
      the keys and the data in one repository (off-host-backup.sh:133-153).
    - >-
      The tape segment writer is deployed with PLATFORM_TAPE_SEGMENTS=write
      (docs/archive/definitions/choir-tape-segments-a2-repack-2026-10-21.md, "r1_and_flag");
      no repack or census tool exists in main (that file, "blocker"); the
      restore drill (scripts/off-host-restore-drill.sh) covers tape files,
      Dolt, one image and the Dolt snapshots, not segments.

finish:
  deliver: >-
    A computer's hibernate snapshot can be made to rest only on an
    ephemeral-key encrypted volume (fail closed, off by default, enabled for
    one test computer first); the escrow private key can rest sealed under a
    key that is not in the backup (off by default, with a tested way back);
    the data image and quarantine modes are tightened; a repack and census
    tool exists, proven on a fixture, with the restore drill written as
    steps; and the key-holder design for NET-09 is on file.
  artifact: >-
    Commits on lane/b9-keys, in this order: (1) this file and the problem
    record; (2) the mode change with its tests (orange); (3) the snapshot
    vault in internal/vmmanager behind VM_SNAPSHOT_VAULT_DIR, with tests
    first (red, dark); (4) nix/snapshot-vault.nix, scripts/snapshot-vault.sh
    (up, down, status, scan) and a stub-based shell test (red, dark, not imported); (5)
    sealed escrow keys in internal/keyescrow, the escrowkey tool, the
    corpusd and checkpointd wiring behind CHOIR_KEY_ESCROW_SEAL (red, dark);
    (6) internal/taperepack and cmd/taperepack: plan, copy and census, proven
    on a fixture (red, copy only, no delete); (7) the restore drill steps
    (green). Never a push, never a tape delete.
  acceptance:
    - action: >-
        go test ./internal/vmmanager -run 'Vault|Snapshot|DataImage' and the
        two shell tests, with each fix broken once on purpose.
      proves: >-
        The guard refuses every non-vault target, the default path is
        unchanged, and each failure mode written below has a test that fails
        without its code.
      evidence_class: local test
    - action: >-
        FP0 on a named disposable test computer, steps "Deployed proof"
        below: canary in guest RAM, hibernate, raw scan of the backing file
        and the state directory, wake, vault restart, unmounted vault.
      proves: >-
        The snapshot of that computer is on a dm-crypt plain device, no
        plaintext copy is in the state directory, and every failure path
        cold-boots instead of writing plaintext.
      evidence_class: deployed proof
    - action: taperepack plan, copy and census on a test computer's tape, then the restore drill piece.
      proves: >-
        Every eligible legacy pin of that computer has an equal-bytes
        segment copy read with the fallback off, and the restored segments
        pass the same census.
      evidence_class: deployed proof (counts only)
  rollback: >-
    Each switch is off by default and its rollback is unsetting it:
    VM_SNAPSHOT_VAULT_DIR (a snapshot is then written to the state
    directory as today; existing vault snapshots are read in place until
    used), CHOIR_KEY_ESCROW_SEAL=off with `escrowkey unseal` (the key file
    returns to plaintext, same public key), and not running taperepack (it
    only adds files under tape-seg). The mode change has no rollback need.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize what a read of Node B's disks, a copy of a directory, or a
    non-root process with file access yields (guest memory, data images, the
    escrow key, the keys and data together in one backup), while preserving
    boot, hibernate and wake behavior with every switch off, the crash rule
    (a lost snapshot cold-boots), and the tape's immutability.
  goodharting_would_be: >-
    A guard that checks a path string instead of the device behind it; a
    vault that is a plain directory because the mount did not happen; an
    encryption claim proved by a unit test that never looks at the disk; a
    sealed key whose key encryption key sits in the same backup; a census
    that reads the legacy file because the fallback was left on; a
    "data at rest solved" line for a data image that is still plaintext.

homotopy:
  realism_axis: >-
    Fixture and fake sysfs on a developer machine -> the same code with
    switches off on staging (byte-identical behavior) -> one disposable test
    computer on the vault with a canary scan of the raw backing file -> a
    host reboot (empty vault, cold wake) -> the paper's host path (owner
    step) -> a guest-image encryption design with a key delivered from
    outside the host (owner decision, not built here).

boundaries:
  mutation_class: red
  authority_sources:
    - docs/problems/data-at-rest-snapshots-images-escrow-key-plaintext-2026-10-21.md
    - docs/archive/reports/computer-isolation-security-review-2026-10-14.md section 3.8 and 4.8
    - docs/archive/reports/boundary-strength-ladder-2026-10-15.md rungs 5 and 6, plan items 7 and 8
    - docs/problems/host-compromise-blast-radius-2026-10-20.md (plan items 2 and 3, held privately)
    - docs/archive/definitions/choir-tape-segments-a2-repack-2026-10-21.md and docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md section 6.1
    - CLAUDE.md (no tape delete, no tape edit, no head rewind)
  protected_surfaces: [snapshot custody, the escrow private key, key-escrow loading in corpusd and checkpointd, VMM file view, off-host backup scope, the tape (read-only in the tool, copy-only writes)]
  must_preserve:
    - With every switch unset, vmmanager, corpusd, checkpointd and the backup behave as they do on main.
    - A snapshot that cannot be written to the vault is not written at all; the computer cold-boots (the crash rule).
    - The escrow key is never regenerated because a file was missing, in any sealed mode.
    - No tape file is deleted, edited or moved, and no head is rewound, by any code here.
    - No key, token or memory content is printed, committed or logged; tests generate their own keys.
  excluded:
    - Full-disk encryption of Node B, the EPYC bring-up, a TPM or network unlock, an HSM or KMS (owner decisions below).
    - Per-computer guest-side volume encryption (designed below, not built).
    - The jailer plus vault combination (refused, not built).
    - Any tape delete, balance or archive step; the repack of the paper's or the owner's computer.
    - A signer process (NET-09 is design only).
  ceremony: >-
    Red: this file carries the conjecture delta, protected surfaces, admissible
    evidence (local tests with the break-once check, then FP0's deployed
    proof), rollback and heresy delta; the problem record is the first
    commit. Failure modes are written below before any code.

heresy_delta:
  discovered: >-
    corpusd regenerates the escrow key when the file is missing; the host
    swap file is plaintext; a Firecracker core dump may carry guest memory
    (hypothesis); one backup secret opens keys and data together; the F12
    retention and 36 hour figures are stale (problem record).
  introduced: none
  repaired: >-
    Under CHOIR_KEY_ESCROW_SEAL=permit or require, a missing escrow key file
    is an error instead of a new key (still regenerated under off, as today);
    data images are created 0600 (commit 2). Neither is live until FP0 lands
    and sets the switch.

now:
  status: open_handoff
  slice: >-
    Repair round 2 done (bunch 31 findings E1 to E3 and five lows, below) on
    lane/fp9-on-g. Repair round 1 done (bunch 4b findings B1 to B4 fixed, below). Before it: commits 1 to 7 built on lane/b9-keys (the vault, its host script and
    nix module, the mode change, sealed escrow keys with the escrowkey
    tool, taperepack, the drill steps). Every remaining step is FP0's
    (landing, the flake entries, deployed proofs) or the owner's (open
    questions 1 to 4).
  source_ref: origin/main 0cb66b4cb
  deploy_identity: unknown
  candidate: {id: FP9-lane, state: built_locally, ref: lane/b9-keys, base: 0cb66b4cb, digest: none (FP0 freezes it at review), scope: [internal/vmmanager, internal/keyescrow, internal/taperepack, internal/tapestore (W10 test only), cmd/corpusd, cmd/checkpointd, cmd/escrowkey, cmd/taperepack, nix/snapshot-vault.nix, scripts, docs]}
  conjecture:
    id: FP9-C1
    claim: >-
      If a snapshot can be captured only into a mount the host probes as a
      dm-crypt plain device, whose key is read from /dev/urandom at host boot
      and never stored, then a read of the host's persistent disks, a copy of
      the state directory or a stolen backup no longer yields that computer's
      guest memory. The claim covers hibernate by snapshot with an unjailed
      VMM and nothing the guest wrote to its data image.
    test: >-
      A canary string held in a test computer's RAM is found by a scan of the
      decrypted vault view and not found by a scan of the raw backing file,
      the state directory, or the backup staging area.
    edge: missing_oracle
    delta_o: >-
      Plaintext written before the vault existed stays on free blocks of the
      main file system until overwritten; only a raw-device scan before and
      after (FP0) bounds it, and only full-disk encryption removes it.
    scope_if_supported: snapshot hibernate of computers selected for the vault, unjailed VMM, one host
    status: proposed
    evidence_refs: [local tests only; see receipts]
  conjectures_also:
    - id: FP9-C2
      claim: >-
        Once the escrow key is sealed and its KEK is kept out of the backup,
        a copy of platform-artifacts or of the backup does not yield the
        escrow key. Not claimed: a read of the root file system that also
        holds the KEK.
      status: proposed
    - id: FP9-C3
      claim: >-
        Every eligible legacy pin of a test computer has an equal-bytes
        segment copy read with the fallback off, and a restore of its
        tape-seg passes the same census. Edge: the manifest sees receipted
        pins only; FP0 compares its per-computer count with the computer's
        append receipts.
      status: proposed
  decision:
    what: >-
      Snapshot memory goes to an ephemeral-key dm-crypt plain volume (the
      review's section 4.8 recommendation), selected per computer, refusing
      when the probe fails; the escrow key may rest sealed under a KEK kept
      out of the backup; data volumes and how the host unlocks after a
      reboot are left as owner decisions with the design below.
    kind: architecture
    status: proposal
    evidence_ref: this file
    owner_ratification_ref: pending (FP0 relays the open questions)
  belief:
    believed_state: >-
      No snapshot, image or key at rest is encrypted on Node B today; the
      code to change that for snapshots and the escrow key exists on this
      branch, dark; data volumes need a key delivered from outside the host.
    main_uncertainty: >-
      Whether Node B's kernel reports CRYPT-PLAIN- in /sys/dev/block/*/dm/uuid
      for a plain mapping (code from cryptsetup's source, not observed), and
      whether a mount made before vmctl starts is visible in its private mount
      namespace.
    next_observation: FP0's `scripts/snapshot-vault.sh status` output on a host with the vault up (uuid prefix, findmnt, vmctl's view).
  blocker_or_risk: >-
    None for local work. The nix module cannot be evaluated here (no Nix in
    the lane VM; its evaluation test skips); it is not imported by
    node-b.nix so it cannot break a deploy until FP0 wires it. escrowkey and
    taperepack are in goServiceSubPackages and installed beside corpusd
    (repair round 2, E1); the flake was not evaluated here (no nix).
  next_action: FP0 reviews the frozen lane, lands it with the switches off, then runs "Landing steps for FP0" below.

receipts:
  - kind: local_tests
    at: 2026-10-22
    ref: lane/fp9-on-g (repair round 2)
    evidence: >-
      go test -race ./internal/vmmanager -run 'Vault|DataImage|Snapshot';
      go test -race ./internal/keyescrow ./internal/taperepack
      ./cmd/escrowkey ./cmd/taperepack; snapshot-vault-test.sh (H1-H17) and
      off-host-backup-kek-exclude-test.sh pass; bash -n on the changed
      scripts; go vet on the changed packages (and GOOS=darwin GOARCH=arm64
      for vmmanager, keyescrow, taperepack). Break-once: the mount-root
      check, the placement check and the live-writer refusal each removed
      made its test fail; the FIFO and script tests failed before their
      fixes. The flake change is reasoned, not evaluated (no nix here).
  - kind: local_tests
    at: 2026-10-21
    ref: lane/b9-keys (repair round 1)
    evidence: >-
      go vet and go test ./internal/vmmanager (whole package ok except
      TestLaunchThreadPolicyReachesOnlyItsChild: no /bin/cat in the lane VM,
      a file this lane does not touch), ./internal/keyescrow,
      ./cmd/escrowkey, ./cmd/checkpointd, ./internal/taperepack,
      ./cmd/taperepack, ./internal/tapestore; GOOS=darwin GOARCH=arm64 go vet
      ./internal/vmmanager; go build ./...; snapshot-vault-test.sh and the
      five off-host-backup shell tests pass. Break-once per finding: B1
      (capture paths reverted to the path string), B2 (discovery without
      the record), B3 (name check removed; unit sources not read), B4
      (grep error folded into no-hit; find status ignored); each made a
      test fail.
  - kind: local_tests
    at: 2026-10-21
    ref: lane/b9-keys
    evidence: >-
      go test ./internal/vmmanager -run 'Vault|Snapshot|DataImage',
      ./internal/keyescrow, ./cmd/escrowkey, ./cmd/checkpointd,
      ./internal/taperepack, ./cmd/taperepack, ./internal/tapestore: ok.
      scripts/snapshot-vault-test.sh, off-host-backup-kek-exclude-test.sh,
      off-host-backup-preflight-test.sh: pass. Break-once: the require
      refusal and the KEK mode check (keyescrow), the census fallback and
      the plan hash check (taperepack) each made a test fail.

---

# Station FP9: keys and recovery

## What is true now

See the problem record for the evidence list with file and line. In one
paragraph: guest RAM is written to the main file system in plaintext and kept
up to 36 hours; every data image is a plaintext ext4 file with a 0644 mode in
a 0750 directory; the escrow key that unwraps every computer's data key is a
plaintext file; the same restic password opens the keys and the data. Every
host secret sits on the same unencrypted root file system, so a key stored
there protects nothing against that disk. Only a key that never exists on a
disk (the snapshot vault) or arrives from outside the host (owner decision)
adds anything.

## NET-06: the smallest fail-closed change

| Asset | Change in this station | State | Switch |
|---|---|---|---|
| data.img, staged and quarantined image modes | create 0600; tighten existing images at boot and on quarantine | built (commit 2) | none (always on) |
| Snapshot `mem` and `vmstate` | capture into a verified dm-crypt plain mount, else refuse | built, dark (commit 3) | `VM_SNAPSHOT_VAULT_DIR`, `VM_SNAPSHOT_VAULT_ONLY` |
| The vault itself | nix module, `scripts/snapshot-vault.sh` (up, down, status, scan) | built, not imported (commit 4) | FP0 imports it |
| Escrow private key | sealed under a KEK from outside the artifacts volume | built, dark (commit 5) | `CHOIR_KEY_ESCROW_SEAL`, `CHOIR_KEY_ESCROW_KEK_PATH` |
| Data volumes (guest images) | design only | below | owner decision |
| Swap file and core dumps | recorded as findings, not changed | problem record | FP0 live check |

### The snapshot vault

Mechanism. A boot-time service creates a sparse file, attaches a loop
device, opens it with `cryptsetup --type plain` using a key read from
`/dev/urandom` (never written down), makes an ext4 file system, mounts it at
one path with mode 0700. vmmanager writes a computer's snapshot under
`<vault>/<vm>/snapshot` instead of `<state>/<vm>/snapshot`. Plain mode has
no header and no key slot, so a reboot or a `cryptsetup close` makes the
file unreadable even to root. A host reboot cold-boots every hibernated
computer, which loses no durable data and agrees with the crash rule.

The guard. Before a snapshot is attempted, and again at commit and at
resume, vmmanager reads `/proc/self/mountinfo` for the vault path and
requires: the path is clean and absolute and is itself a mount point (not a
subdirectory, not a symlink); the mount's device has a `dm/uuid` in sysfs
that starts with `CRYPT-PLAIN-` (any `CRYPT-` prefix only when
`VM_SNAPSHOT_VAULT_ACCEPT_ANY_CRYPT=on`, which is weaker because a LUKS key
slot is on disk); the directory is root-owned with no group or other access.
Anything else, including an unreadable probe or a non-Linux host, is a refusal
with a named reason and the computer takes the kill path. There is no
fallback to the state directory.

Selection. `VM_SNAPSHOT_VAULT_DIR` unset: nothing changes. Set with
`VM_SNAPSHOT_VAULT_ONLY=<vm ids or computer ids>`: only those computers
capture into the vault; the rest behave as today. Set with no ONLY list:
every computer. An existing snapshot is found where it exists (vault first),
so turning the vault on or off never strands a snapshot, and the boot sweep
removes any plaintext snapshot of a vault computer left in the state
directory.

Not supported, refused with `snapshot_vault_jailed_unsupported`: a jailed
VMM. The jailer needs the jail root on the state directory's mount to
hard-link `data.img` (confinement.go:110-112), and Firecracker writes its
capture inside the jail (snapshot_lifecycle.go:191-239), so the plaintext
would land on the main file system first. Options for the next slice, to be
settled with a probe on a host, not here: a submount inside the jail root
(needs the jailer to carry it into its mount namespace), or putting the jail
base on the vault with `data.img` bind-mounted. Until then, launch with the
jailer on and the vault on means no snapshot hibernate for those computers
(cold wake). That trade is the owner's (open question 3).

### The escrow key

`keyescrow.SealPrivateKey` writes a JSON file (version, purpose, public-key
fingerprint, nonce, ciphertext) sealed with XChaCha20-Poly1305 under a
32-byte KEK, with the purpose and the fingerprint as associated data.
`CHOIR_KEY_ESCROW_SEAL` is `off` (default, exactly today), `permit` (a sealed
or plaintext file is accepted, plaintext logged) or `require` (plaintext is
refused). In `permit` and `require` a missing file is an error, never a new
key. The KEK file must be 0400 or 0600 and 32 bytes (base64 text or raw);
under systemd it is a `LoadCredential=` file. The `escrowkey` tool has
`check` (state, fingerprint, whether it opens), `seal` (atomic, verifies
before replacing, same public key) and `unseal` (the way back).

What this does and does not buy. A copy of `platform-artifacts/` or of the
backup repository no longer carries the escrow key in the clear. It does not
help against a read of the root file system that also holds the KEK. To make
the first true for the backup, the KEK file is excluded from the daily backup
(one line in `off-host-backup.sh`, a no-op while the file does not exist) and
the owner keeps an offline copy, as for the restic password; a restore then
needs the owner's copy of the KEK. How the KEK reaches the unit after a
reboot is an owner decision (open question 1).

### Data volumes (design only)

Two honest routes, neither provable on a developer machine:

1. Full-disk encryption of the host (LUKS under the btrfs on Node B at its
   rebuild, at EPYC bring-up) with a key from outside the host (a network
   unlock such as Tang/Clevis, or a TPM). One change protects every file on
   the host, including the swap file and core dumps, and removes the plaintext
   remnants this station cannot. It is infrastructure; it is the review's
   first step.
2. Per-computer encryption of the guest data volume with a key delivered at
   boot: the guest opens a dm-crypt or LUKS volume on `/dev/vdb` in the
   initrd with a key that arrives on the credential disk (or the existing
   host-only resumed call) and is held in guest tmpfs, derived as
   HKDF(master, computer id) from a master that vmctl obtains at start from
   outside the disk. It needs the guest kernel's dm-crypt, the initrd step in
   `nix/autoputer-vm.nix` (FP2 owns the guest side of that file), a master
   key source, and a migration for existing images. It adds revocation (drop
   the master or a computer's derivation) which route 1 does not. The
   privacy key stays inside the image in both routes today; delivering it from
   escrow at every boot instead (the path `deliverPrivacyKey` already has for
   a restored image) is a separate, smaller change that only helps once the
   escrow key itself is sealed from outside.

Neither is started here. The mode change and the vault do not depend on them.

## Failure modes, written before code

Mode change (commits 2 and 3 of the branch: tests, then fix).
1. A new image is created 0644. 2. An existing 0644 image stays 0644 after a
boot. 3. A staged or quarantined image keeps a loose mode. 4. The tightening
follows a symlink and changes another file's mode (it opens O_NOFOLLOW and
refuses a non-regular file). 5. The tightening changes the snapshot disk
identity (it must not: identity is size, mtime, inode, device; chmod moves
ctime only). 6. A jailed VM loses access (the jail chowns the inode to the VMM
uid, so a 0600 root-owned file is only the unjailed case). 7. A chmod error
stops a boot (it must not; it logs, the parent directory remains the barrier).
The VM directory itself is already 0750 and is left alone.

Snapshot vault (commit 3).
V1 The vault path is a plain directory because the mount did not happen.
V2 The mount is on a non-crypt device (loop, ext4, tmpfs). V3 The device is
LUKS and only plain is accepted; and the reverse with the accept flag.
V4 The probe cannot read mountinfo or sysfs (non-Linux, hidden /sys).
V5 The path is a symlink, relative, not clean, or a subdirectory of a mount.
V6 The directory is group or world accessible, or not root-owned.
V7 The vault goes away between the start of a capture and its commit (the
probe runs again before the rename).
V8 A plaintext snapshot of a vault computer is found in the state directory
at resume or at the boot sweep. V9 A computer outside the ONLY list behaves
as on main even with a working vault. V10 vmctl restarts with a snapshot in
the vault (reservations and listing must include it). V11 The budget and the
free-space floor are measured on the file system the snapshot will occupy.
V12 A jailed VMM with the vault selected is refused. V13 The vault is empty
after a host reboot: resume reports no snapshot and the caller cold-boots.
V14 An invalid `VM_SNAPSHOT_VAULT_DIR` (relative, a symlink, a non-directory)
must not fall back to the state directory: every capture is refused.
V15 Unset: all existing tests pass unchanged.

Host module and script (commit 4).
H1 The unit runs twice and recreates a live vault (it must be idempotent when
the mount is up). H2 A stale device-mapper name or loop device from a failed
run blocks the next boot (clean up first). H3 The backing file is created
readable by others. H4 The key is read from anything but /dev/urandom or is
written to a file. H5 The mount point is left 0755. H6 `snapshot-vault.sh
status` passes on a vault that is not dm-crypt plain.

Sealed escrow key (commit 5).
E1 Wrong KEK. E2 A sealed file with no KEK configured. E3 A plaintext file in
`require`. E4 A missing file in `permit` or `require` (never generate).
E5 A truncated or edited sealed file. E6 A sealed file for another purpose
loaded as the escrow key. E7 A KEK file that is group or world readable.
E8 A seal that replaces the file without proving the same public key.
E9 The default `off` still accepts a plaintext file and still creates a key
when the file is absent exactly as today. E10 Key bytes left in memory or in
error text.

Repack and census (commit 6, detailed in the L2-04 section below).

## NET-09: keys behind policy signers; passkey-bound owner acts (design only)

Current holders (from nix/node-b.nix and the ladder, rungs 5 and 6):

| Key | Held by | Reaches |
|---|---|---|
| Auth JWT signing key | auth (`ed25519-key`), and also the gateway unit, which only needs to verify | mints any user's session, including the owner's |
| Platform signing key and platform-control key | corpusd and checkpointd (`platformSigningKey` kept visible) | signs mode receipts, checkpoints, head receipts |
| Escrow private key | corpusd and checkpointd | unwraps every computer's escrowed data key |
| Owner-act signing key | proxy (LoadCredential `choir-owner-act-key`) | signs owner acts the host verifies by public key |
| Escrow operator tokens | corpusd (`corpusd-escrow-operators.env`) | the two-approver reveal |
| Gateway provider credentials and the delegate token | gateway, vmctl's runtime copy | model and search spend |
| Per-computer privacy key and signer keys | the guest, on its data image | that computer's private payloads and statements |

Target (ladder plan items 7 and 8), in the order they pay off:
1. The JWT key used by auth only: the gateway verifies with the public key.
   Small, orange, and it removes one holder.
2. A host signer process holding the platform signing key, the platform-
   control key and the escrow key, with a statement policy: it signs only
   statement types a named caller may request (head receipt for corpusd,
   checkpoint for checkpointd), and opens an escrow wrap only for the two
   approver flow or the boot delivery path. The services call it over a unix
   socket with a peer check; the keys leave the general processes. This is
   where a sealed key (commit 5) becomes the signer's start-up input.
3. Guest signers behind per-desk policy (FP2's ARCH-02 manifest decides what
   a desk may ask for).
4. Owner acts bound to a fresh passkey assertion: user verification required
   and checked, a challenge that is the hash of a server-rendered summary of
   the act, covering self-development approval, restore, mode set and fleet
   push; recovery needs the old passkey or a waiting period with notice to
   the old devices. The ladder lists the one-request test of whether login
   accepts an assertion without the UV flag.

Order and dependence: item 1 is independent and can land first; item 2 needs
the policy table (a list of statement types and callers) written from the
code before it is built; item 4 is auth work (FP11-class, accounts and
session) and shares the passkey contract with B11. None is built here; this
section is the design the FP0 review can accept or redirect.

## L2-04: backup ready for launch

What exists: a restic backup to node-a (append-only, ciphertext only), a
restore drill for tape samples, Dolt, one image and the Dolt snapshots, and
the segment store deployed with the writer on. What is missing: the tool
that moves the legacy tape files into segments and proves them, and a drill
piece for segments. The rule stays: no tape delete, no edit, no head rewind;
a delete needs the owner's explicit yes (owner notes 34, evidence-gated).

The tool (`internal/taperepack`, `cmd/taperepack`, standard library plus
`internal/tapestore` and `internal/computerevent`, no Dolt):

- `plan`: reads the legacy pin receipts (`sha256/pin-receipts/*.json`),
  verifies each file's name equals the SHA-256 of its bytes, decodes it,
  and lists for each computer the eligible artifacts: events whose kind is
  `projection_batch_recorded` and payloads whose media type is the
  projection batch type (the same eligibility as the production pin path,
  event_artifacts.go:39 and :65), each with its receipt. The artifact file
  must exist and hash to the receipt's digest. It writes a manifest (JSON
  lines: computer, namespace, digest, length) and counts of dangling
  receipts, unreceipted artifacts and ineligible files. It writes nothing
  under the artifacts root.
- `copy`: for the named computers only, appends the manifest's artifacts and
  receipts through the production appender (`tapestore.Appender`, so the
  per-computer lock, fsync discipline and fail-stop apply), skipping digests
  already in a segment. It never writes, moves or deletes a legacy file. A
  digest that is already in a segment with different bytes is an error.
- `census`: for every manifest entry, reads the segment copy with
  `Reader.ReadSegment` (no legacy fallback), compares the bytes with the
  legacy file's, and counts equal, missing, different and corrupt. Exit
  status is non-zero unless every entry is equal. It is the evidence the
  tape-segments goal asks for before any delete, and the same command runs
  on a restored tree.

What the tool cannot see: a pin that is not receipted, an orphan, and
whether the receipts are complete against the chain. That is the manifest's
edge (FP9-C3); FP0 closes it by comparing the manifest's per-computer count
with the platform's append-receipt count for the test computer.

Restore drill on the segment snapshot (steps for FP0, on a disposable test
computer, never the paper or the owner's computer; the existing drill's
memory caps apply):

1. Pick a test computer with segments (the flag is on): note its id, the
   segment file count and total bytes under `tape-seg/<id>/` (counts only).
2. `taperepack plan --root <artifacts> --computer <id> --out <scratch>/manifest.jsonl`
   (the manifest must be outside the artifacts root; the tool refuses
   otherwise). Record the counts line. Compare `per_computer[<id>]` with
   the computer's append-receipt count (FP9-C3's edge). Then
   `taperepack census --root <artifacts> --manifest <manifest> --computer <id>`
   once before the copy: it must exit 3 with everything missing (the
   census reads no legacy file as a segment copy). Then
   `taperepack copy --root <artifacts> --manifest <manifest> --computer <id>`
   (idle IO: `nice -n 19 ionice -c3`), and the census again: exit 0, all
   equal. A second copy must report `records_written` 0.
3. Run the daily backup (or take the latest snapshot id after it).
4. `restic restore <snap>:/var/lib/go-choir/platform-artifacts/tape-seg/<id>`
   into a scratch directory outside any live path.
5. `taperepack census --root <scratch root> --legacy-root <artifacts>
   --manifest <manifest> --computer <id>` (segments from the restored tree,
   the legacy bytes from the live root): exit 0, `equal` equal to
   `records`, zero missing, corrupt, different, legacy_bad.
6. Delete the scratch restore and the scratch manifest. Record counts only.
7. Do not delete, move or edit any file under `platform-artifacts`.

## Deployed proof (FP0, on named disposable test computers only)

Snapshot vault. Pick one disposable test computer (not the paper, not the
owner's). 1. Import `nix/snapshot-vault.nix`, enable it with a size of at
least budget plus 4 GiB, deploy. 2. Run `scripts/snapshot-vault.sh status`
on the host: it must pass (plain crypt uuid prefix, mount point, mode, key
never on disk). 3. Set `VM_SNAPSHOT_VAULT_DIR=<mount>` and
`VM_SNAPSHOT_VAULT_ONLY=<the test computer's id>`; order vmctl after the
vault unit; deploy. 4. In the test computer hold a random canary string in
memory (a process that reads 32 random bytes and keeps them; record only the
canary's hash). 5. Hibernate it by snapshot. Expect: files under
`<vault>/<vm>/snapshot/`, none under `<state>/<vm>/snapshot*`. 6. Scan the
raw backing file for the canary with `scripts/snapshot-vault.sh scan
<canary file> <backing file> <state dir>`: expect zero hits, `read errors: 0`
and exit 0 (an incomplete scan exits 1 and is not evidence); scan the
mounted vault: expect at least one. 5b (finding B1). Check the create paths
in the hibernate log or timeline go through `/proc/<vmctl pid>/fd/`; if the
VMM was refused that path the capture fails closed (kill path, a cold wake)
and the vault is not usable until that is understood. 5c. While a second
selected computer is being hibernated, run `snapshot-vault.sh down`: it
must fail busy (the capture holds the vault), and the capture must land
in the vault or take the kill path, never the state directory. 7. Wake it: healthy, the canary process still holds it.
8. Hibernate again, then `systemctl restart` the vault unit only if no VMM
maps a vault file (otherwise skip; the unit is idempotent): wake cold. 9.
Unmount the vault (with the computer stopped) and hibernate another selected
computer: the timeline reason is `snapshot_vault_unavailable`, no files in
the state directory. 10. Compare one unselected computer's hibernate: still
in the state directory, as before. 11 (finding B2). With the vault up and
the test computer hibernated into it, unset `VM_SNAPSHOT_VAULT_DIR`, deploy,
and wake it: it resumes from the vault (the recorded root
`<state>/snapshot-vault-roots` names it); a later hibernate goes to the
state directory.

Escrow sealing (on staging, not the paper's host path first: use a copy of
the key on a scratch host or a dry run). 1. `escrowkey check` on the key
file: plaintext, record the public fingerprint. 2. Place the KEK
(`escrowkey` can generate it to a file with mode 0400) and an offline copy
for the owner; set `CHOIR_KEY_ESCROW_SEAL=permit` and the KEK path; deploy;
corpusd healthy. 3. `escrowkey seal`; fingerprint unchanged; restart
corpusd and the checkpoint worker; escrow public key endpoint unchanged and a
checkpoint job opens an escrowed key. 4. Flip to `require`. 5. Exclude the KEK
from the backup and take a backup; a restore of `platform-artifacts` without
the KEK fails `escrowkey check`. 5b (repair round 2, E3: restic omitted
the KEK, proven on the repository, not the plan text). With the restic
environment the restore drill uses (scripts/off-host-restore-drill.sh),
`restic ls latest | grep -c key-escrow-kek` must print 0, and for each
KEK source the plan listed under "excluded:" (the units' credential
source and any absolute `CHOIR_KEY_ESCROW_KEK_PATH`),
`restic ls latest <that path>` must list nothing. Counts and paths only;
never print a file. 6. Rollback drill: `escrowkey unseal`, set
`off`, restart: same fingerprint.

The tools on Node B (repair round 2, E1): the corpusd package carries them,
so they are `/var/lib/go-choir/services/corpusd/bin/escrowkey` and
`/var/lib/go-choir/services/corpusd/bin/taperepack` (the store copy is
`<corpusd package>/bin/...`). Every `escrowkey` and `taperepack` in these
steps means that path.

## Review round 1 (bunch 4b, frozen 3bd691a6b): findings and dispositions

Sol sent the candidate back; Argon accepted. Every blocking finding is
accepted; the evidence is the code as frozen. The fixes follow in later
commits, tests first.

B1 (high, accepted). The vault probe does not protect the writes after it.
`captureSnapshot` probes the path (snapshot_lifecycle.go:128) and then
creates `snapshot.partial` and hands Firecracker string paths under
`m.cfg.SnapshotVaultDir` (:192-223); the commit re-probes only after guest
RAM is written (:272). If the vault is unmounted in between (for example
`snapshot-vault.sh down` while the computer is being fenced, which succeeds
because nothing holds the mount), the path resolves to the plain directory
under the mount point on the main file system and guest RAM lands there in
plaintext; the commit probe then refuses, but removing the files does not
remove the blocks. A second point-in-time probe cannot close this. Fix:
open the vault root once, verify that open directory (its device is the
dm-crypt plain device the mount table names, and it is the probed
directory), and resolve every capture and commit path through that open
directory (`/proc/self/fd/N/...` for vmctl, `/proc/<vmctl pid>/fd/N/...` for
the unjailed VMM), holding it until the commit's rename is synced. The open
directory is also the coordinated teardown: a plain `umount` (what `down`
runs) fails with EBUSY while it is held, and a lazy unmount leaves the held
directory on the still-open encrypted device, so a write can never fall
through to the main file system. The files are checked at commit to be on
the vault's device.

B2 (medium, accepted). Unsetting `VM_SNAPSHOT_VAULT_DIR` strands vault
snapshots: `snapshotRoots`, `locateSnapshot`, `snapshotAnyDirs` and the sweep
consult only the configured path (snapshot_vault.go:250-274), contrary to
this file's rollback promise. Fix: separate write selection (the switch)
from discovery: when vmctl writes into a vault it records the vault root in
`<state>/snapshot-vault-roots`; discovery, listing, budget, discard and the
sweep use the configured root plus the recorded roots; a recorded root that
is no longer configured is never written, a snapshot in it resumes only if
it still probes as an encrypted mount, and the sweep drops the record once
that root holds no snapshot.

B3 (medium, accepted). The backup excludes the name `key-escrow-kek*`, but
`LoadKEK`, `GenerateKEKFile` and the CLI accept any file name, so a KEK at
`/var/lib/go-choir/secrets/custom-kek` is backed up beside the sealed key.
Fix: the name is enforced where the key is accepted (the KEK file's name
must start with `key-escrow-kek`, for loading and generating), a Go test
keeps the backup pattern and that prefix equal, and the backup also
excludes the actual persistent source of a `key-escrow-kek` credential (and
any `CHOIR_KEY_ESCROW_KEK_PATH`) it reads from the corpusd and checkpointd
units, so a systemd source file under another name is excluded too.

B4 (medium, accepted). `snapshot-vault.sh scan` suppresses `grep` read
errors (`2>/dev/null`, a non-zero status read as "no hit") and ignores
`find`'s traversal errors, so it can print zero hits and exit 0 having
inspected nothing. Fix: grep's status 2 and a failed find are counted as
read errors; a non-regular entry that is not inspected is counted; any
read error makes the scan exit 1 with "read errors: N"; a hit count of zero
is reported only when every requested byte was read.

Non-blocking (Argon), fixed here because they are cheap: the file named
`scripts/snapshot-vault-verify.sh` that does not exist (the script is
`scripts/snapshot-vault.sh status` and `scan`), and landing step 1 (FP0's
flake commit already adds the two commands). Sol's non-blocking notes need
no change (the census proves manifest equality, not chain completeness; the
station already requires the receipt-count comparison).

Repair commits (lane head after this file's update): 24f778b27 (this
section, green); a348e747f and 4ee539886 (B1, B2: tests, then the
vmmanager fix, red); a25482b57 and ec86efad2 (B3: tests, then the KEK name
rule and the unit source exclusion, red); 068a8fa02 and 5079aee6e (B4: tests, then
the scan, red). All four blocking findings: fixed. Residuals from this round:
the VMM's use of `/proc/<vmctl pid>/fd/N` is unproven on Node B (step 5b);
`kek_sources` parses `systemctl show` lines whose format is from systemd's
documentation, not observed on Node B (the `plan` check in landing step 3);
a KEK delivered some way other than a credential or that variable is not
discovered (the loader accepts no other way).

## Repair round 2 (bunch 31, DeepSeek v4.1 flash, accept with edits)

Reviewed on lane/fp9-on-g at ab67a159f (the lane rebased onto main
4fca1e01b). Tests first (1e5db9308, yellow), then the fixes; each finding:

- E1 (high, deliverability), fixed 6969a1522 (orange): the two commands
  were neither built nor installed. They join `goServiceSubPackages` and
  the corpusd package's `extraBins`; the drill names
  `/var/lib/go-choir/services/corpusd/bin/`. go.mod and go.sum are
  untouched and both commands import only the standard library and
  internal packages already in the source filter, so vendorHash does not
  change (reasoned from the flake text; no nix in the lane VM). Landing
  step 1 is corrected.
- E2 (medium), fixed 1e5db9308 (yellow): `newVaultFixture` skips off Linux
  (the vault resolves paths through /proc/<pid>/fd and is refused on any
  other host), so the 14 vault tests skip instead of failing on darwin.
- E3 (medium), fixed in this file: escrow sealing step 5b proves on the
  repository that restic omitted the KEK (`restic ls latest`), not only the
  plan text. `kek_sources` splitting `Environment=` on spaces (a KEK path
  with a space is missed) stays a residual beside the unobserved
  `systemctl show` format.
- Low, mount root, fixed f64062b54 (red): mountinfo field 4 must be "/".
- Low, taperepack live store, fixed 2c6aa397e (red): copy tries the
  appender's per-computer flock non-blocking before each batch and
  refuses if held; plan and census only read and take no lock.
- Low, non-blocking open, fixed 6058cb54d (red): readBounded opens with
  O_NOFOLLOW|O_NONBLOCK, fstats, refuses non-regular files.
- Low, script strict mode, fixed 0652617d4 (red): `set -euo pipefail` with
  every answer-bearing step guarded; `down` exits 1 if the backing file
  stays.
- Edge, capture selection, fixed f64062b54 (red): with the vault list set, a computer
  with no computer id whose VM id the list does not name is refused before
  the fence (`snapshot_vault_unknown_computer`, kill path), never captured
  in plaintext.

## Landing steps for FP0

All switches ship off; with them unset the lane is byte-identical in
behavior to main (must_preserve 1).

1. The flake is on the lane (commit 6969a1522, repair round 2, E1): the
   earlier statement here, that FP0's commit 3bd691a6b would be kept on
   rebase, was false (that commit was on a frozen candidate and the rebase
   onto 4fca1e01b dropped it). `cmd/escrowkey` and `cmd/taperepack` are in
   `goServiceSubPackages` and in the corpusd package's `extraBins`; check
   after the deploy that `/var/lib/go-choir/services/corpusd/bin/` holds
   both. Not evaluated in the lane VM (no nix): run `nix flake check` or
   the deploy build first.
2. Snapshot vault: the "Deployed proof" steps below.
3. Escrow sealing: the corpusd and checkpointd units read
   `CHOIR_KEY_ESCROW_SEAL` and `CHOIR_KEY_ESCROW_KEK_PATH`; give the KEK as
   `LoadCredential=key-escrow-kek:/var/lib/go-choir/secrets/key-escrow-kek`
   and set `CHOIR_KEY_ESCROW_KEK_PATH=%d/key-escrow-kek`. Generate it with
   `escrowkey kek-generate --kek /var/lib/go-choir/secrets/key-escrow-kek`
   (mode 0400, refuses to overwrite; the name must start with
   `key-escrow-kek`, finding B3). The daily backup excludes any
   `key-escrow-kek*` name and, read from the units, the source of a
   `key-escrow-kek*` credential and any absolute `CHOIR_KEY_ESCROW_KEK_PATH`:
   check `off-host-backup.sh plan` lists the configured source under
   "excluded:" before the first backup with the KEK in place. Then the escrow sealing steps below, using
   `escrowkey check|seal|unseal --key <key path> --kek <kek path>` with
   corpusd stopped and no checkpoint job running during seal and unseal.
4. Tape: the restore drill on the segment snapshot (L2-04 above).

## Open owner questions (defaults stand; nothing blocks)

1. What unlocks the host after a reboot: a TPM (is one exposed on Node B?), a
   network unlock from another host (Tang), an owner-typed secret, or none
   for now? Default: none for now. The escrow KEK is a file outside the
   backup; the data volumes stay as they are; full-disk encryption waits for
   the rebuild.
2. May the KEK for the escrow key live as a file on Node B (protects a copy of
   the artifacts or the backup, not the disk), with an offline copy in the
   owner's hands? Default: yes, for the test, not for the paper until asked.
3. Launch with the jailer on and the vault on means no snapshot hibernate
   until the jailed capture slice is settled. Which matters more for the first
   users, snapshot wake or snapshot confidentiality? Default: the vault is
   enabled only for computers the owner selects.
4. Should the swap file be replaced by an ephemeral encrypted swap (the same
   pattern as the vault) or removed? Default: record only.

## Residuals

- Plaintext that predates the vault stays on free blocks (btrfs, md RAID1)
  until overwritten; no code here can scrub it.
- The swap file and a possible Firecracker core dump are findings, not fixes.
- The vault's key lives in kernel memory; live root reads it.
- The vault is a sparse file that can grow to its size on the main file
  system; the state-directory pressure thresholds already watch that file
  system.
- The escrow KEK on Node B protects copies, not the disk.
- The platform-control and signing keys are not sealed here (the signer
  design covers them).
