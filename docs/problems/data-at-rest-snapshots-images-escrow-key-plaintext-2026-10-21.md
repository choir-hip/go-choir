# Guest memory, data images and the escrow key rest in plaintext on the host disks (SDC NET-06, review F12)

Status 2026-10-30: SEC-06 stays an open red item; references to the guest Dolt data image are stale (guest state is now SQLite `state.db` on the guest disk, and the escrowed key lives in the control database). Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Date: 2026-10-21. Station FP9 of the launch first pass
(`docs/archive/definitions/choir-launch-fp9-keys-recovery-2026-10-21.md`). Mutation
class of the fixes: red (snapshot custody, the escrow key, the VMM's file
view), with one orange item (file modes). Status: documented; no fix yet.

This record re-reads F12 of
[the isolation security review](../archive/reports/computer-isolation-security-review-2026-10-14.md)
(section 3.8) against main at 0cb66b4cb, because the review's line numbers
and one number (the 36 hour age limit) have moved, and the private design
for the blast-radius record (`host-compromise-blast-radius-2026-10-20.md`,
plan item 3) is not in this repository. Everything below is a code reading
or a configuration reading. Nothing here was observed on a live host, and
the live checks are listed as FP0 steps at the end. Live root on the host
stays total in a design that runs Firecracker without confidential-computing
hardware; this record is about what a disk read, a drive that leaves the
building, a copy of a directory, or a non-root process with file access
obtains, not about live root.

## What is true on main

### A snapshot is the guest's RAM, in plaintext, on the main file system

- A snapshot hibernate is on in production: `nix/node-b.nix:1349`
  (`VM_HIBERNATE_MODE=snapshot`), kept at most 36 hours
  (`nix/node-b.nix:1355`; the code default is 12 hours,
  `internal/vmmanager/snapshot.go:67`), within a 48 GiB budget
  (`nix/node-b.nix:1350`).
- A snapshot lives under the VM's own state directory, which is on the same
  file system as everything else:
  `internal/vmmanager/snapshot.go:241-251` (`<state>/<vm>/snapshot` and
  `snapshot.partial`; the state directory is `VM_STATE_DIR`,
  `/var/lib/go-choir/vm-state`, `nix/node-b.nix:1250-1253` and `:1297`).
- Firecracker writes a Full snapshot, the guest's entire memory, to the
  `mem` file: `internal/vmmanager/fcapi.go:124-130`
  (`"snapshot_type": "Full"`). The host never encrypts it: after the VMM
  stops, `commitSnapshot` only fixes the mode and fsyncs
  (`internal/vmmanager/snapshot_lifecycle.go:254-264`, `0o600` at `:258`).
  The permissions are right (the capture directory is `0700`, `:181`), and
  permissions are all there is.
- A jailed capture writes the plaintext inside the jail directory first and
  moves it afterwards (`internal/vmmanager/snapshot_lifecycle.go:191-239`),
  so a jail whose base is on the same file system gives the same result,
  with one more copy of the bytes on disk. The jail base must be on the state
  directory's mount so that `data.img` can be hard-linked
  (`internal/vmmanager/confinement.go:110-112`).
- A resumed VM maps the memory file privately for its whole life and the
  name is deleted after a healthy resume
  (`internal/vmmanager/snapshot_lifecycle.go:594-597`): the blocks stay
  allocated until the VMM exits.
- What a guest's RAM holds is the review's inference (the gateway bearer the
  runtime re-reads, the privacy data key once loaded, signer keys, open
  documents): `reports/computer-isolation-security-review-2026-10-14.md`
  section 3.8. The lifetime is code (36 hours, plus the deleted-name tail).

### A data image is a plaintext ext4 file; the modes are looser than the review says is safe

- Creation: `os.OpenFile(path, O_CREATE|O_EXCL|O_RDWR, 0o644)` then
  `mkfs.ext4`: `internal/vmmanager/manager.go:2997` and `:3020`. The
  per-computer directory is created `0750` (`manager.go:842`; the later
  `0o755` `MkdirAll` at `:883` is a no-op on an existing directory); an
  existing image is never re-moded. Correction of the first draft of this
  record, which said the directory was `0755`.
- The state directory above is `0750` (`nix/node-b.nix:1253`;
  `scripts/verify-host-hardening.py:375` checks the same), so a non-root
  user cannot traverse to the image today. The review's point stands: the
  mode on the file should not be the only thing that depends on the parent.
- The guest's privacy key (the key for the computer's private payloads and
  file sync) is a file on the same image,
  `/mnt/persistent/choir-credentials/privacy-key`
  (`internal/autoputer/boot_timeline.go:452`, `internal/autoputer/run.go:358`).
  A copy of the image carries the key beside the data.
- Quarantined images keep the original mode (the move is a rename,
  `manager.go:3091`). The review says retention is unstated; it is stated:
  two post-swap quarantines per VM
  (`internal/vmctl/cold_recover.go:145-155`, swept by
  `internal/vmctl/handlers.go:795`). That part of F12 is stale.

### The escrow private key is a plaintext file on the artifacts volume

- `keyescrow.LoadOrGeneratePrivateKey` reads or creates
  `<artifacts>/key-escrow-key` as base64 text, mode `0600`
  (`internal/keyescrow/keyescrow.go:68-111`); the path is
  `nix/node-b.nix:465` and `internal/platform/config.go:71`.
- corpusd calls it at startup and checkpointd calls it for the
  checkpoint worker (`cmd/corpusd/main.go:159`, `cmd/checkpointd/main.go:41`).
  The file's only protection is its mode and the units' `InaccessiblePaths`
  (`nix/node-b.nix:983`, `:1109`).
- The same host reads that key to unwrap every computer's escrowed data
  key (`keyescrow.OpenDEK`, `cmd/checkpointd/worker.go:146`). Whoever holds
  the file holds every escrowed computer key; the review says so
  (`keyescrow.go:1-12` states "HSM/KMS deferred").

### One backup secret opens the keys and the data together

- The daily backup's required path set holds the artifacts root (which
  contains `key-escrow-key` and `signing-key`), the whole `secrets/`
  directory, `auth-signing` (the JWT key), and the provider and mail
  environment files, in the same restic repository as the Dolt stores, the
  tape and the data images: `scripts/off-host-backup.sh:133-153`. Only the
  backup's own key directory is excluded (`:123-132`). The repository
  password is on Node B and in the owner's offline copy
  (`docs/reports/node-a-backup-handover-2026-10-20.md`, "What this is").
  This is plan item 3 of the blast-radius record ("split backups by class so
  no single backup secret opens everything"); nothing in main does it.

## Findings this reading added (heresy delta: discovered)

1. **corpusd silently generates a new escrow key when the file is absent.**
   `LoadOrGeneratePrivateKey` creates one on `ErrNotExist`
   (`keyescrow.go:86-110`) and corpusd does not check first
   (`cmd/corpusd/main.go:159`); checkpointd does check
   (`cmd/checkpointd/main.go:32-34`). A missing or wrongly mounted artifacts
   root would give corpusd a fresh key under which every existing wrap fails
   to open, with no error at start. Fail closed means a sealed or plaintext
   key that is absent is an error, and creating one is an explicit act.
2. **The host swap file is plaintext.** `nix/disks.nix:35-37` declares
   `/swap/swapfile` on btrfs with no encryption. Several units set
   `MemorySwapMax=0` (the vmctl unit at `nix/node-b.nix:1225` keeps guests
   out of swap), but the units that hold the escrow key and the JWT key in
   memory are not all among them (a live check is a step below). A key held
   in memory can reach the swap file.
3. **Firecracker core dumps are not blocked in the vmctl unit.** `LimitCORE=0`
   appears only on the Dolt units (`nix/node-b.nix:894`, `:941`), and no
   `systemd.coredump` setting exists in `nix/`. A VMM crash may write guest
   memory under `/var/lib/systemd/coredump`. This is a hypothesis from the
   configuration; `coredumpctl list` on the host settles it.
4. **No key on the same disk protects against that disk.** Every host secret
   is a file on the root file system (`nix/node-b.nix:76-114`), and the
   disks are btrfs on md RAID1 with no LUKS (review section 3.8, live
   `lsblk`). A key that sits beside the data adds nothing against a drive
   read; the snapshot case is the exception, because its key can be made
   never to exist on any disk.

## What the fix must and must not claim

- A **snapshot vault** (a volume whose key is generated from
  `/dev/urandom` at each host boot and never written down) removes guest RAM
  from every persistent disk, copy and backup the vault is kept out of. The
  review's own recommendation (section 4.8). A host reboot cold-boots every
  hibernated computer, which loses no durable data (the image persists) and
  agrees with the crash rule in `CLAUDE.md`.
- **Sealing the escrow key** under a key encryption key that is *not* in the
  backup and not on the artifacts volume protects a copy of the artifacts
  directory and the backup repository; it does not protect against a read of
  the root file system that also holds the key encryption key. How the key
  encryption key reaches the unit (a credential, a TPM, a network unlock, an
  owner-typed secret after a reboot) is an owner decision (open question 1 in
  the goal file).
- **Data volumes** (the guest images) are protected at rest only by full-disk
  encryption with a key delivered from outside the host, or by per-computer
  encryption with keys delivered at boot. Neither is a change that can be
  proved on a developer machine, and neither is built in this station; the
  goal file records the design and what blocks it.
- None of this changes live-root exposure.

## Fix direction

Specified and sequenced in the station goal file. In short: file modes first
(orange); then the snapshot vault, dark behind `VM_SNAPSHOT_VAULT_DIR`, with
a fail-closed probe that refuses a snapshot unless the target is a mount on a
`dm-crypt` plain device; then a sealed escrow key behind
`CHOIR_KEY_ESCROW_SEAL=require`, with a sealing tool whose rollback is
unsealing; then host provisioning and the verification script.

## Live checks for FP0 (counts and states only; no key or memory contents)

- `findmnt -no SOURCE,FSTYPE /var/lib/go-choir/vm-state` and `lsblk -f`:
  confirm no `crypto_LUKS` or `crypt` layer under the state directory.
- `stat -c '%a %U' /var/lib/go-choir/vm-state/*/data.img | sort | uniq -c`:
  the mode histogram of live images.
- `coredumpctl list --no-pager | tail`, and `cat /proc/sys/kernel/core_pattern`
  (finding 3).
- `swapon --show` and, per key-holding unit, `systemctl show -p MemorySwapMax
  <unit>` (finding 2).
- A snapshot present for a hibernated test computer:
  `ls -l /var/lib/go-choir/vm-state/<vm>/snapshot/` (size and mode only).
