# Nothing durable on Node B exists anywhere else (2026-10-20)

Mutation class: green for this record. The work it leads to is red for
`nix/node-b.nix` (a backup client that reads protected surfaces: the event
tape, Store A, signing and escrow keys, auth state) and orange for the host
module handed to node-a. Status: problem recorded; design below; client
(`nix/node-b.nix`, `scripts/off-host-backup.sh`) and target
(`nix/node-a-choir-backup-target.nix`) written but **disabled and not applied
anywhere**.

## The problem

Node B is a single physical host. Everything Choir would have to rebuild a
computer, prove its history, or sign anything again lives on it, and no byte
of it exists on any other machine. The root filesystem is btrfs on an md
RAID1 pair (`nix/disks.nix`), which survives one disk failing. It does not
survive losing the host (hardware, provider account, fire, a bad
`nixos-rebuild`, an operator `rm`, ransomware) or a software error that
damages both mirrors at once (the 2026-10-05 artifact GC deleting live
bodies was exactly that class: `docs/problems/s0-gc-og-wrong-store-deletion-2026-10-05.md`).

The only "backups" that have ever existed were on-node, temporary, and
partial:

- Dolt copies under `/var/tmp` taken before a risky step, kept until the
  step's soak ended, then deleted (the 48 GB pre-2.4.2 copy, the 2026-09-11
  and 2026-10-03 copies; `docs/problems/guest-dolt-journal-and-host-image-leak-2026-10-03.md`).
  Same disk, same failure domain.
- `s0-gc-og-wrong-store-deletion-2026-10-05.md`: "On-node backups cover
  dolt state, not `platform-artifacts`." The file-CAS bodies it lost were
  the sole copy; nothing could restore them.

## What is at stake (all of it on Node B only)

Sizes are from the cited records, not re-measured here.

| Durable thing | Where | Why it matters | Size (record) |
|---|---|---|---|
| Event tape (computer events, payloads, pin receipts) | `platform-artifacts/sha256/{computer-event,computer-event-payload,pin-receipts}` | The canonical, signed, immutable history of every computer. Owner rule: never edit it, never rewind heads. No replica anywhere. | 19.9 GB in 9.07M files (tape-volume report) |
| Store A, platform Dolt | `/var/lib/go-choir/platform-dolt` | Control store: ownership, heads, append receipts, event index. | 1.26 GiB logical; 23 GB on disk 2026-10-14 |
| Store B, corpus Dolt | `/var/lib/go-choir/corpus-dolt` | Autopaper corpus, shared published objects. | 22 GB on disk 2026-10-14 |
| File CAS chunks and roots | `platform-artifacts/sha256/file-cas-*` | Owner files; "the CAS file was the sole copy" (S0 record). | about 48 GB |
| `og` bodies, capture bodies, proposals, platform updates | `platform-artifacts/sha256/{og,platform-update,proposals}`, `capture-bodies/` | Externalised rows hold `body = NULL`; the file is the only copy. | about 6 GB |
| Platform signing key and key-escrow key | `platform-artifacts/signing-key`, `key-escrow-key` | Every head receipt and pin receipt is signed by the first; the second unwraps escrowed owner material. Without them the tape is unverifiable going forward and escrow is lost. | bytes |
| Auth state | `/var/lib/go-choir/auth/auth.db`, `auth-signing/` | Passkey credentials and sessions. Owner ruling #13 already says recovery is a launch blocker. | MB |
| Guest data disks | `/var/lib/go-choir/vm-state/*/data.img` | Each computer's persistent state and guest Dolt. Rebuildable in part from the tape, not entirely. | 110 GB with snapshots (2026-10-14) |
| Host secrets | `/var/lib/go-choir/secrets/*`, provider and mail env files | Rotation is possible but slow; the owner-act key and Dolt passwords are not. | KB |

Not at stake, because regenerable: `projection-base` (47 GB, rebuilt from the
tape), `checkpoint-scratch`, `host-build`, the Nix store, guest images.

## Why now

- **L3 is gated on it.** The ledger metamission's entry criterion is "a
  recoverable off-host copy of the ledger" (`choir-ledger-metamission-2026-10-13.md:137-138`);
  the cost-track report says SP0 therefore gates L3.
- **Tape surgery needs an archive before any delete.** The tape-volume
  report (section 6.1 step 5, 6.3, 6.4) makes the delete steps black unless a
  verified off-node archive exists, and asks the owner to "name the archive
  target". There is no target.
- **Growth.** The tape adds about 1.5M small files a day; the host is at
  about 80% of 476 GB. Disk pressure is the likeliest thing to force a
  hurried delete, and today a hurried delete has no undo.
- The 10-17 report puts it plainly: "Until then, a total loss of Node B
  would lose history that exists nowhere else."

## The ruling

Owner, 2026-10-14, `docs/reports/owner-notes-2026-10-14.md` section 27
item 17:

> **Off-host backup: node-a now, as a stopgap.** Encrypted (key only on
> Node B and the owner's offline copy), quota'd subvolume, node-a holds
> only ciphertext. "True durable offsite backup requires more infra which
> we will get later." Node-a is also the tape cleanup's archive target.

Constraints that follow, from `docs/node-a-shared-host.md` and the
roadmap's "Node-a stopgap" section:

- node-a runs another project and its administrators have root. Only
  ciphertext may reach it; the key never does.
- Choir does not touch node-a. Its own agent applies a NixOS module that we
  hand over.
- It is a stopgap: one extra host, shared, not a true offsite copy. A
  compromise or loss of Node B and node-a together, or of the owner's
  offline key copy plus Node B, still loses everything.

## Evidence this is worth doing the careful way

- Restic, rest-server and the Dolt backup procedure were checked as far as
  possible without touching any host (see the design's "What was verified").
- The app Dolt user cannot run `DOLT_BACKUP` (`command denied`); root can.
  Checked locally against Dolt 2.1.9. This shapes the client's credentials.

## Heresy delta

- discovered: backups exist only in the same failure domain as the data; the
  S0 record named the gap on 2026-10-05 and nothing closed it for 15 days.
- introduced: none (nothing is enabled).
- repaired: none yet. The repair is the enabled, drilled client and an
  applied target; this record closes only when a restore drill from node-a
  passes.

## Design

Labels: **[V]** verified here (read in the repo, or run locally);
**[M]** measured in a cited record; **[H]** hypothesis, with the probe that
settles it. Nothing below ran on Node B or node-a.

### 1. Tool: restic, with a repository password

Chosen: **restic** (repository format v2: compression, password-derived master
key), not age-encrypted tar streams.

- **Deduplication and incremental by construction.** Most of what we back up
  is large, mostly unchanged files (guest data images, Dolt table files,
  file-CAS chunks). A tar stream either re-sends everything nightly or needs a
  hand-built full/incremental chain whose restore depends on every link.
  Restic's content-defined chunking sends only new chunks, and every snapshot
  restores standalone. The tape is immutable, so its increments are natural.
- **Prune and retention are built in** (`forget --prune`); a tar stream would
  have to reinvent them.
- **Self-checking.** `restic check --read-data-subset` re-reads and re-hashes
  packs; a tar stream has no equivalent short of downloading all of it.
- **Encryption is part of the format**, authenticated (AES-256-CTR +
  Poly1305-AES), and the password is used only on Node B. node-a holds packs,
  an index and a scrypt-wrapped `keys/` file; it learns sizes and timing,
  nothing else.
- **Cost, stated.** restic walks the whole source tree each run. The tape is
  9.07M files **[M]**, so a run costs minutes of metadata IO and some GB of
  RAM for the tree **[H]** (measured on the first supervised run, before the
  timer is enabled; `MemoryHigh=4G` and the idle IO class bound the damage).
  Segment packing (tape-volume report A1/A2) cuts the file count by about
  four orders of magnitude and removes the cost; the backup does not wait for
  it.
- **Rejected:** `age` tar streams (no dedup, no prune, restore chains);
  `borg` (needs a borg binary and a shell-restricted account on node-a, and
  its append-only mode is an `authorized_keys` command restriction that we
  would be trusting the other project's sshd config to keep); btrfs
  send/receive (node-a would run `btrfs receive` as root on plaintext, which
  is exactly what we cannot allow).

### 2. Transport: rest-server in append-only mode, over an SSH port forward

Chosen: **`rest-server --append-only --private-repos` bound to loopback on
node-a, reached from Node B through a restricted SSH port forward.**

Why append-only is the point: the threats this backup exists for include
"Node B does something destructive" (a GC bug, an operator `rm`, ransomware,
a compromised service). With restic over SFTP a chrooted user can delete its
own repository, so a compromised or buggy Node B deletes the backups together
with the data. With rest-server `--append-only` the client can add packs,
indexes and snapshots, and cannot delete or overwrite them (lock files
excepted). Per rest-server's documentation; node-a's agent confirms it with
the commands in the handover note. Consequence: the daily job **cannot run
`forget --prune`**. Retention is a monthly, owner-supervised act through a
second, normally stopped, non-append-only instance (section 5). That is
deliberate: deleting backups is the black operation, and it should need a
human.

Why a port forward and not an exposed port: node-a is a shared host. The
forward means no new listening address, no firewall change (22 is open
already), no TLS certificate to run, and the SSH host key (pinned in
`known_hosts` on Node B) authenticates node-a to us. The SSH key is
restricted to forwarding to one loopback port; it gets no shell and runs no
command. rest-server still requires HTTP basic auth (`.htpasswd`, with
`--private-repos` so the Node B user reaches only `/nodeb/...`), because any
local user of node-a can reach loopback. None of this protects
confidentiality; restic's encryption does. These controls protect integrity,
quota and availability.

What node-a's admins can do regardless (root): read ciphertext, delete or
corrupt it, fill the disk, stop the service. Append-only protects against
Node B, not against them. Corruption is detected by `restic check`; deletion
is the stopgap's known limit (section 12).

### 3. What is backed up, and how consistently

| Item | How it is captured | Consistency | Notes |
|---|---|---|---|
| Store A (platform Dolt) | `CALL DOLT_BACKUP('sync-url','file://<dir>')` on the running server into `platform-dolt/.choir-backup/platform`; restic reads that directory | Dolt-native, no downtime: a point-in-time copy of every branch **and** the working set. **[V]** locally (Dolt 2.1.9): sync works against a running server, repeats incrementally, and `dolt backup restore` returns committed data plus uncommitted working-set rows. Node B runs 2.4.2: **[H]** until the first run. | **Needs root.** The app user `choir` is refused (`command denied to user 'choir'`, **[V]**), so the job loads the existing `dolt-root-password` credential. The server's sandbox lets it write only under its own data dir, hence the `.choir-backup` dot-directory inside it. Costs one extra on-disk copy of each store. |
| Store B (corpus Dolt) | Same, port 13307, `corpus-dolt/.choir-backup/corpus` | Same | Torn down 2026-10-09 by owner decision but the data remains (22 GB). Uploaded once; deduplicates to near zero afterwards. |
| Event tape and the rest of `platform-artifacts` | restic reads `platform-artifacts` directly | Files are immutable and content-named, so a file that is visible is complete (write-then-rename, **[H]**: the drill compares sampled files to live). A file landing mid-run is caught by the next run. Store A's snapshot may be slightly ahead of or behind the tape copy; every event on the tape is self-verifying by digest and signature, so restore replays from the Store A copy and ignores tape files past it (**[H]**). | Excludes `sha256/projection-base` (47 GB, rebuilt from the tape) and `checkpoint-scratch`. **Includes** `signing-key` and `key-escrow-key`. No btrfs snapshot of `/@`: the tape report shows a snapshot pins the metadata of 9M files and makes the planned deletes worse. |
| `auth.db`, `mail.db` | `sqlite3 .backup` into the staging dir | SQLite's online backup: consistent while the service runs | `-wal` and `-shm` are not copied. |
| Guest `data.img` (one per computer) | `cp --reflink=always` into `offhost-backup/staging/vm-images/<id>/`; restic reads the clones; an exit trap deletes them | A clone is atomic (one FICLONE per file), so the image is crash-consistent: what the guest sees after a power cut. Its Dolt journal recovers on boot. **[H]** until the drill boots one. | Skips `data.img.*` (manual snapshots, quarantine, staging: derived). Clones pin COW extents while they exist, hence the trap. If a clone fails the run uploads everything else and **fails loudly**. |
| Secrets and signing material | restic reads the files | Small, rarely written | `secrets/` (except `secrets/offhost-backup/`, which holds the key and is never in its own backup), `auth-signing/`, `gateway-*.env`, `codex-auth.json`, `maild.env`, `searxng.env`, `corpus-dsn.env`, `deploy.env`, `sandbox-gateway-token.env`, `mail/`, `files/`, `vm-state/ownerships.json`. |
| Run manifest | written to staging, then included | | Dolt head hashes, counts, tool versions, start and end times. |

Skipped as regenerable: `projection-base`, `checkpoint-scratch`, `host-build`,
the Nix store, `guest/` images, `front-importance`, Firecracker memory
snapshots, and `qdrant` (a vector index, rebuilt from the corpus; **[H]**,
owner to confirm at the drill).

Everything is encrypted by restic on Node B before it leaves.

### 4. Schedule

- **Backup:** daily 02:30 UTC, `RandomizedDelaySec=15m`, `Persistent=false`.
  `Persistent=false` matches the repo's rule that nothing replays work at boot
  without a planned-restart marker: a missed run is picked up by the next
  tick. The slot is after the evening load and before the 04:00 store
  compaction.
- **Verify:** weekly (Sunday 06:00 UTC), `restic check
  --read-data-subset=n/10`, with `n` rotating by ISO week, so all data is
  re-read every ten weeks (about 20 GB of download a week).
- **Freshness probe:** every 6 hours (section 10).
- The service sets `restartIfChanged = false`, `Nice=19`, the idle IO class,
  `MemoryHigh=4G`, and an upload cap, so a deploy does not kill a run and a
  run does not starve the paper. A run cut off leaves only unreferenced
  packs; the next day's run continues.

### 5. Retention

- Policy: keep 14 daily, 8 weekly, 12 monthly snapshots. Between prunes every
  snapshot is kept (append-only).
- **Prune is a monthly, owner-supervised window.** node-a's agent starts the
  second rest-server instance (no `--append-only`, bounded by
  `RuntimeMaxSec`, reachable only through a different SSH key that the owner
  holds and Node B never has); the owner runs `restic forget --prune` from
  Node B with that key and the delete credential supplied interactively; the
  instance is stopped. The module defines this instance only if a prune key is
  configured.
- The `archive` repository (tape-cleanup archives) is **never pruned**; an
  archive is retained until the owner rules otherwise (tape report 6.4).
- Because the sources are mostly additive, prune reclaims little. The
  governing constraint is the quota (section 7), not retention.

### 6. Recovery targets (proposed; the SP table says "yours")

- **RPO:** 24 hours for everything: up to a day of events (tape, Store A,
  guest state) can be lost with the host. A stricter RPO needs a second daily
  run or tape tailing; not designed here.
- **RTO:** the same day, 8 hours, to restore Node B's durable state onto a
  freshly provisioned host that has the Nix config from git. About 110 to 180
  GB must come back (section 7): 1.5 to 3 hours at 150 to 250 Mbit/s, plus the
  Dolt restore, plus guests rebuilding projection bases from the tape. **[H]**,
  set by the first drill.

### 7. Size and bandwidth

First full upload, before compression and dedup:

| Part | GB | Source |
|---|---:|---|
| Tape (`computer-event*`, `pin-receipts`) | 19.9 | **[M]** tape-volume report |
| File CAS chunks and roots | 48 | **[M]** 12gb-a-day record; sealed with random nonces, so incompressible |
| `og`, `platform-update`, `capture-bodies`, `proposals` | 6 | **[M]** |
| Store A + Store B Dolt backup dirs | 5 to 45 | on-disk stores were 23 + 22 GB **[M]**; Store A logical size 1.26 GiB. **[H]** |
| Guest `data.img` (allocated) | 30 to 60 | `vm-state` was 110 GB including snapshots **[M]**. **[H]** |
| Secrets, auth, mail | under 1 | |
| **Total** | **about 110 to 180** | |

At a 200 Mbit/s cap that is roughly 1.5 to 2 hours; at 100 Mbit/s, 3 to 4.

Steady state: the tape grows about 2.3 GB/day (1.42 + 0.48 + 0.41 **[M]**),
guest images and Dolt another 2 to 5 GB/day **[H]**, and file-CAS chunks up to
6.2 GB/day while "every boot re-uploads the whole Files tree" is unfixed
**[M]**. Compression (about 2x on the tape's JSON and receipts, **[H]**) and
dedup bring that to roughly **4 to 12 GB/day** of new ciphertext. Each night
restic reads the whole source set but sends only changed chunks.

**The quota is the binding constraint.** node-a had 907 GB free; the roadmap
suggested "say 300 GB"; the owner offered half to two thirds. The target
module defaults to 450 GiB. At about 150 GB base and 4 to 12 GB/day, 450 GiB
lasts roughly 25 to 75 days with no prune relief. So this stopgap is sized in
weeks to a couple of months unless the growth fixes land (CAS re-upload, tape
segment packing, retention). The freshness probe alerts at 80% of the quota so
the decision arrives with warning.

### 8. Key procedure

- The restic **repository password** is the only key. It is generated **on
  Node B**, by the owner in a supervised session, into
  `/var/lib/go-choir/secrets/offhost-backup/restic-password` (0600, root). To
  be clear: this record describes the procedure and does not generate
  anything. The value is 32 random bytes from the system CSPRNG, base64
  encoded, written under `umask 077`, never on a command line, never logged.
- It is **printed once**, on the owner's terminal, for the offline copy: two
  physical copies (paper or an offline encrypted medium) in different places.
  The owner confirms by typing it back into `restic key list` against the
  initialised repository; only then is the timer enabled.
- It is **not** in any snapshot (excluded), not in the repo, not in a Nix store
  path, not on node-a, not in CI.
- node-a sees only an SSH public key and a bcrypt hash of the rest-server
  password (the password itself is in `secrets/offhost-backup/rest-password`
  on Node B).
- **If Node B and the offline copy are both lost, every backup is
  unrecoverable noise.** That is the design. A second custodian copy is the
  owner's choice.
- Rotation: `restic key add`, then `key remove` for the old one. Removing a key
  deletes a `keys/` file, so it needs the supervised non-append-only window.
  Rotation is required if Node B is ever considered compromised, and it does
  not make old snapshots safe from someone who already copied them.

### 9. Restore drill

Before the timer is first enabled, then quarterly. This record does not close
without one passing.

1. On a scratch host (never Node B's live paths), fetch the password from the
   owner's offline copy and restore the latest snapshot into an empty
   directory over the tunnel.
2. Compare 200 random tape files byte for byte with live (`cmp`).
3. `dolt backup restore file://<restored>/.../platform-dolt/.choir-backup/platform`
   into an empty directory, start `dolt sql-server` on a side port, run the
   platform health queries, and compare the head with the manifest.
4. `sqlite3 auth.db 'PRAGMA integrity_check'`.
5. Boot one restored `data.img` in a throwaway Firecracker VM if the scratch
   host has KVM; otherwise `e2fsck -n`, mount read-only and read the guest
   Dolt.
6. Record wall-clock time (the real RTO), bytes transferred and any failure in
   `docs/reports/`.

### 10. Monitoring and alerting

There is no alert channel on Node B yet (the cost-track report: "Not started:
... log shipping, metrics, alerts"). What exists and what is added:

- The backup, verify and freshness units are `Type=oneshot`: a failure leaves
  the unit `failed`, visible in `systemctl --failed` and the journal, where the
  existing probes already report. A partial run (an item skipped) exits
  non-zero too.
- The script writes `/var/lib/go-choir/offhost-backup/status.json`: last
  success, last attempt, snapshot id, bytes added, repository raw size,
  skipped items.
- A **freshness probe** (every 6 h) fails if the last success is older than 36
  hours, if the last run was partial, if no verify has passed in 10 days, or if
  the repository is over 80% of the quota.
- **Gap, named:** nobody is paged by a failed unit. The probe makes failure
  loud and machine-readable; wiring it to mail (`maildctl`) or a health
  endpoint is a follow-up and stays a residual risk until done.

### 11. Enable procedure on Node B

1. node-a's agent applies the target module and hands back the SSH host key
   fingerprints (handover note).
2. Owner, on Node B: create `secrets/offhost-backup/` (0700); generate the
   restic password and the rest password; generate an ed25519 tunnel key;
   place a pinned `known_hosts`; give node-a's agent the public key and the
   bcrypt hash of the rest password. Copy the restic password offline.
3. `scripts/off-host-backup.sh init` once (creates the repositories).
4. A supervised `systemctl start go-choir-offhost-backup` with the timer off:
   measure wall time, RSS, bytes added, disk guard.
5. Run the restore drill (section 9).
6. Only then set `offHostBackup.enable = true` in `nix/node-b.nix` and deploy
   through the Landing Loop (red: commit, push, CI, staging, identity, proof).
   Rollback: set it back to `false`; nothing on node-a is touched.

### 12. Residual risks

- node-a admins can delete or corrupt the repository, and nothing here
  prevents it. This is one extra copy on a shared host, not an offsite. A
  rented second copy is the real fix.
- The quota runs out in weeks to a couple of months (section 7).
- restic memory and IO on 9M files is unmeasured **[H]**.
- Crash-consistent guest images, the Dolt 2.4.2 backup path and the tape's
  write-then-rename assumption are unproven until the drill.
- The repository password protects against node-a, not against Node B's own
  compromise: the signing and escrow keys are in the backup.
- Failure is loud but nobody is paged (section 10).
- The owner's offline key copy is a single point of unrecoverability.

### What was verified

- **[V]** local Dolt 2.1.9: `CALL DOLT_BACKUP('sync-url','file://...')` against
  a running `sql-server` as root works and repeats; restore returns committed
  data and uncommitted working-set rows; a user with `ALL` on its database is
  denied.
- **[V]** syntax: `nix-instantiate --parse` on the Nix files and `bash -n` on
  the script. restic, rest-server and shellcheck are not installed where this
  was written, so their flags come from documentation, and the handover note's
  verification steps are what settle them on the real hosts.
- Not verified: anything on Node B or node-a; rest-server append-only
  behaviour; the SSH forward restrictions; restic memory at 9M files.
