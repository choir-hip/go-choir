# Nothing durable on Node B exists anywhere else (2026-10-20)

Mutation class: green for this record. The work it leads to is red for
`nix/node-b.nix` (a backup client that reads protected surfaces: the event
tape, Store A, signing and escrow keys, auth state) and orange for the host
module handed to node-a. Status: problem recorded; design below; client
(`nix/node-b.nix`, `scripts/off-host-backup.sh`) and target
(`nix/node-a-choir-backup-target.nix`) written but **disabled and not applied
anywhere**.

> **Update 2026-10-30 (data system cutover, phase 2 item I7).** The Dolt steps
> in this record (Store A and Store B syncs and snapshots, the Dolt root
> credential, the dolt and snap restore-drill pieces) describe a retired
> system. The client now runs `pg_dump -Fc` of the `control` and `graph`
> PostgreSQL databases into `staging/pg` before restic, proves each dump with
> `pg_restore --list`, and ends partial (exit 3) if one fails; the units are
> enabled again. Restore proof: `scripts/off-host-restore-drill.sh <snapshot>
> pg`. Test: `scripts/off-host-backup-pg-test.sh`. Snapshots made before the
> cutover still hold Dolt data and restore with the old pieces.

## The problem

Node B is a single physical host. Everything Choir would have to rebuild a
computer, prove its history, or sign anything again lives on it, and no byte
of it exists on any other machine. The root filesystem is btrfs on an md
RAID1 pair (`nix/disks.nix`), which survives one disk failing. It does not
survive losing the host (hardware, provider account, fire, a bad
`nixos-rebuild`, an operator `rm`, ransomware) or a software error that
damages both mirrors at once (the 2026-10-05 artifact GC deleting live
bodies was exactly that class: `docs/archive/problems/s0-gc-og-wrong-store-deletion-2026-10-05.md`).

The only "backups" that have ever existed were on-node, temporary, and
partial:

- Dolt copies under `/var/tmp` taken before a risky step, kept until the
  step's soak ended, then deleted (the 48 GB pre-2.4.2 copy, the 2026-09-11
  and 2026-10-03 copies; `docs/archive/problems/guest-dolt-journal-and-host-image-leak-2026-10-03.md`).
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
excepted). **[V]** locally with rest-server 0.14.0 and restic 0.19.1: with
`--append-only`, `restic forget <snapshot>` is answered `403 Forbidden`, a raw
`DELETE` or overwriting `POST` of a pack is `403`, and a second backup of
unchanged data adds 0 bytes; node-a's agent repeats the check on the real
host with the commands in the handover note. Consequence: the daily job **cannot run
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
  holds and Node B never has); the owner runs `restic forget --prune` from a
  machine they trust (not an unattended Node B job), with the repository
  password from the offline copy, over that SSH key and a separate delete
  credential; the instance stops itself. The module defines this instance only
  if a prune key is configured, so by default Choir can never delete anything
  through it.
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
- **[V]** local restic 0.19.1 against rest-server 0.14.0 (loopback, htpasswd,
  `--append-only --private-repos --max-size`): `RESTIC_REST_USERNAME` and
  `RESTIC_REST_PASSWORD`, `init --repository-version 2`, a `--json` backup
  summary, `stats --mode raw-data --json`, `check --read-data-subset=3/10` all
  work; forget and raw DELETE/overwrite are `403`; another user's path and a
  wrong password are `401`. An **apr1/md5 htpasswd entry is rejected** by this
  rest-server ("Invalid htpasswd entry"); bcrypt and `{SHA}` entries work, so
  the client's `keygen` emits bcrypt (`htpasswd -niB`).
- **[V]** the Nix: `nix-instantiate --parse` on both files; the node-a module
  evaluates as a real NixOS module (no failed assertions; generated
  `authorized_keys` options, sshd `Match` blocks, rest-server command lines
  and the subvolume script inspected); the node-b `offHostBackup` units
  evaluate with stub arguments (disabled: `condition = false`). `bash -n` on
  the script, and `plan`/`freshness` run against a scratch tree.
- Not verified: anything on Node B or node-a; the SSH forward restrictions on
  a real sshd; btrfs subvolume and qgroup commands; the full `backup` run
  (needs restic, ssh and Dolt on Node B); restic memory at 9M files;
  shellcheck was not available.

### 13. Enable procedure, progress (2026-10-20)

The owner approved node-a as the stopgap target and authorized Choir to
change node-a's config (owner, 2026-10-20: "you can edit mike project work
with this. either edit node-a or the mike repo").

1. **Done, 18:23 UTC.** The target module is in the Mike project's node-a
   config (`/root/mike-infra/node-a/choir-backup-target.nix`, imported from
   `configuration.nix`), mirrored into the Mike repo on the owner's Mac
   (`infra/node-a/`, commit df9f05cc, only those two paths). Applied the
   way node-a's own rules ask: `nixos-rebuild build` (the closure diff was
   only the backup units and rest-server 0.14.0), `test`, a fresh root
   login, then `switch` (system generation 61; rollback is generation 60;
   the pre-change config is kept in `/root/configuration.nix.pre-choir-backup-*`).
   Two departures from the module defaults: `dataDir = /srv/choir-backup`
   (`/data` does not exist on node-a; `/srv` is a btrfs subvolume) and
   `enforceBtrfsQuota = false` (quotas are off on that filesystem and slow
   one with many snapshots, which node-a has; rest-server's `--max-size`,
   95% of 450 GiB, is the limit). rest-server listens on 127.0.0.1:8000
   only; the `choir-backup` user exists; no unit failed.
2. **Done.** The owner ran `keygen` on Node B (18:0x UTC). It stopped at
   the bcrypt line: Node B has no `htpasswd` (a script bug: fix is to fall
   back to `nix-shell -p apacheHttpd`, held until the S6-0 drill ends).
   The bcrypt line was generated on Node B from the rest password with
   that fallback and placed on node-a (`/var/lib/choir-backup/htpasswd`,
   0600). node-a's host key is pinned in `secrets/offhost-backup/known-hosts`,
   cross-checked three ways (node-a's key file, the operator session, Node
   B's existing builder entry).
3. **Done, 18:25 UTC.** `init` created the `daily` and `archive`
   repositories through the restricted tunnel (restic from `nix-shell`;
   Node B has no restic outside the unit). The owner confirmed the offline
   copy of the restic password opens the repository (owner, 2026-10-20:
   "restic password: MATCH").
4. **Running, started 18:39:15 UTC.** One supervised `backup`, timer off,
   as a transient unit with the daily unit's limits (Nice 19, idle I/O,
   CPUWeight 10, MemoryHigh 4G, MemoryMax 6G), tools from `nix-shell`.
   Readings at 19:43 UTC: both Dolt backup syncs done by 18:41:13
   (platform about 2 minutes, corpus 13 seconds); restic still uploading,
   66 GB on node-a, about 18 MB/s against the 25,000 KiB/s cap; memory at
   the 4G MemoryHigh line (page cache from reading the sources counts
   there; not yet told apart from restic's own heap); free disk on Node B
   121 GB at the start, 85 GB now.
5. **Problem found by the run: a hand-run backup is silently partial.**
   The `nix-shell` tool set I gave the run (restic, jq, openssh, dolt)
   left out `sqlite3`, so `auth.db` and `mail.db` were not staged
   (`sqlite3: command not found`, then `ERROR: sqlite backup of auth.db
   failed`, and the same for `mail.db`). The script carries on and the
   run ends `partial` (exit 3), an hour later. The systemd units do not
   have this problem: their `PATH` (`offHostBackupEnv` in
   `nix/node-b.nix`) already has restic, jq, openssh, sqlite, dolt and
   apacheHttpd. An earlier note of mine said the unit lacked restic, jq
   and openssh; that was the Dolt liveness probe's `path`, not this unit.
   Same cause as the keygen stop in item 2: a step run by hand on Node B
   finds tools missing that the unit has. Fix: each subcommand checks the
   tools it needs before it does anything, and names the `nix-shell`
   line that supplies them; keygen falls back to `nix-shell -p
   apacheHttpd` for `htpasswd`.
6. **Disk cost of a run, measured.** The Dolt backup directories are kept
   between runs (`platform-dolt/.choir-backup` 31 GB,
   `corpus-dolt/.choir-backup` 3.5 GB), so most of the 36 GB drop is a
   standing cost, not a per-run one. The image clones under `staging/`
   show 83 GB apparent but are reflinks on btrfs; they cost only the
   extents the running guests rewrite while the run is in flight, and
   the exit trap removes them. The 40 GiB disk guard is checked before
   staging; nothing rechecks it while restic runs.
7. **Run 1 result (18:39:15 to 20:19:44 UTC, 1 h 40 min).** Exit 3,
   `partial`, for the two sqlite stores only (item 5); every other path
   was present (`skipped: []`). Snapshot `31b057be…` in `daily`: restic
   scanned 9,555,586 files, 680 GB apparent (the guest image clones are
   sparse; the tape is millions of small files), and added 163.6 GB
   before compression; the repository on node-a is 101.2 GB raw
   (`repo_raw_bytes`), inside section 7's 110 to 180 GB estimate. The
   network counters (191 GB out, 98 GB in) count the loopback leg to the
   tunnel as well as the leg to node-a; node-a received about 101 GB.
   Free disk on Node B ended at 83 GB (the Dolt backup directories, 34.5
   GB, stay; the image clones were removed). CPU 2 h 11 min.
8. **Problem found by run 1: restic's working set is about three times
   its memory limit, and the overflow went to swap.** Memory peak 4.0 GB
   (the `MemoryHigh` line) and **swap peak 9.1 GB**, so restic needed
   about 13 GB; the unit read 1.3 TB from disk and wrote 1.0 TB. The
   writes are not the backup (the staging copies are reflinks and the
   Dolt syncs are about 35 GB); swap churn under the `MemoryHigh`
   throttle is the likely cause, not yet measured. Nothing else
   suffered that I could see: no OOM kill on the host, no vmctl
   timeouts, memory and I/O pressure back near zero after the run (I/O
   `full` averaged about 7% over the last five minutes of it). Why it
   matters: a terabyte written a night is real wear on Node B's NVMe,
   and the working set grows with the tape's file count (9.5 million
   now; restic holds the tree in memory). The unit's limits (4G high, 6G
   max) cap RAM, not swap: `MemorySwapMax` is unset. Not fixed yet.
   Options: give restic the RAM it needs (about 14 GB of Node B's 31
   GB), cap swap and let it fail loudly, or cut the file count (the
   tape-segment packing already on the roadmap; the pin receipts are
   most of the 9.5 million). Next measurement: run 2 (incremental),
   the same limits, with the host's swap-in and swap-out counters and
   the unit's I/O read before and after, so the writes are attributed.
9. **Memory, decided.** Owner, 2026-10-20: "give restic more memory".
   The unit now throttles at 10G, stops at 12G and may swap at most 4G
   (60ae1cb64, deployed); run 2 was raised to the same limits at 20:58
   UTC, after its Dolt stage.
10. **Question from the owner: do the guest images need cloning at all?**
    Owner, 2026-10-20: "Do we need to clone guest images? Can't those
    restore from tape/snapshot?" By doctrine, no: "if a field is needed
    after reboot of this computer, it is on the tape"
    (unified-event-tape design), and the self-development restore is
    tape-only by construction (`tape_reconstruct`, which must not read
    surviving local state). The tape and the key escrow are backed up in
    full. What is not yet shown: a computer rebuilt from a *restored*
    tape onto an empty disk on another host matches its live witness,
    and how long that takes for the paper computer (a full replay may be
    hours; an image restores in minutes). Known guest-local state exists
    (for example the guest Dolt GC disposition file, guest-dolt-storage-
    growth FM7), so "everything is on the tape" is a hypothesis to test,
    not a given. Today's images: 15 VM directories, most of them tracer
    test computers; the largest image is 40 GB on disk (128 GB apparent).
    The nightly cost of the images is the re-read (each clone is a new
    inode) and the copy-on-write extents the clones pin, not the upload
    (dedup).
    Plan: (a) after run 2, clone only the protected computers' images
    (the paper and the owner computer), not test computers; (b) the
    restore drill rebuilds one computer from the restored tape onto an
    empty disk and compares the witness; (c) if it matches, drop images
    from the nightly run, keeping at most a weekly paper image for
    recovery time.
11. **(a) done.** Owner, 2026-10-20: "Yes, we can stop backing up the
    test computers." 710ce53cb (deployed) clones only the paper and owner
    computers' images; on Node B's data the plan selects 2 of 15.
12. **Run 2 (20:21:56 to 21:42:04 UTC, 1 h 20 min): ok, the first clean
    run.** Script from main with the tool preflight, tools from the
    system packages, `auth.db` and `mail.db` staged. Snapshot
    `17cdae99…`: 9,557,485 files unmodified, 63,645 new, 5,052 changed;
    9.3 GB new before compression, 3.0 GB packed; the repository is
    104.2 GB. restic's stage took 37 min (20:58 to 21:35), then `restic
    check`. Memory peak 10.0 GB (the new 10G line, raised at 20:58),
    swap peak 7.5 GB (pushed out under the old 4G line, before the
    raise; 2 MB in swap at 21:40). Read 669 GB, wrote 118 GB, CPU 41
    min. This run still cloned all 15 images (it started before 710ce53cb).
    Open, measured but not explained:
    - **The Dolt platform sync took 36 minutes** (2 minutes in run 1),
      and the unit sat at its 4G memory line during it while the host
      swapped out about 27 GiB (`pswpout` 80.86M to 87.84M pages,
      20:21 to 20:58). `DOLT_BACKUP` runs in the Dolt server; what in
      the backup's own cgroup held 4 GB then is not known.
    - **669 GB read** is close to the whole source set (680 GB apparent),
      though restic re-read only changed files. Not yet attributed
      (image clones are new inodes and are re-read; the Dolt backup
      directory; swap-in).
    Run 3, with two images, is the comparison.
13. **Restore drill passed, 21:44 to 22:30 UTC**
    (docs/reports/off-host-restore-drill-2026-10-20.md): 200 of 200 tape
    files identical, both sqlite stores pass integrity checks, the corpus
    Dolt store restores at run 2's head, the platform Dolt backup
    directory is identical to live, and a test computer's image is
    identical and fsck-clean. Not shown, kept as residuals: a restore on
    another host, the platform store restored into a running database, a
    tape-only rebuild onto an empty disk (item 10 (b)), a full restore's
    time, a booted image. Restore needs about 8 GB of memory, so the
    drill runs capped.
14. **Enabled** (cac559cba, CI 39507571620, deploy receipt names
    cac559cba). Timers: backup 02:30 UTC plus up to 15 min, freshness
    every 6 h (first run 22:38:53: "fresh: last success 56 min ago, repo
    97 GiB of 450"), verify Sundays 06:00. The first unit run was started
    by hand at 22:39:44; the sandbox and `LoadCredential` worked.
15. **Incident: the backup's Dolt sync took Store A down, 22:51 to 22:53:35
    UTC.** Order of events, from the journals:
    - 22:39:46: the unit's `CALL DOLT_BACKUP('sync-url', …)` on the
      platform server (port 13306) starts.
    - Store A runs with `MemoryHigh = 3G` and `MemorySwapMax = 0` (the
      2026-10-09 host memory budget; about 1.1 GB RSS measured then). The
      sync pushed it to that line: the server's own cgroup memory
      pressure was `full avg10=46.71 avg60=47.98 avg300=45.75`, so it
      spent nearly half its time stalled on reclaim.
    - The Dolt liveness probe (every 30 s) found it unresponsive three
      times in a row and at 22:53:22 sent SIGABRT for a stack dump
      ("unresponsive for 3 consecutive probes"). The stack shows the
      sync in `dolt_backup.go:223` (`doltBackupSyncUrl`).
    - 22:53:30 the server exited; 22:53:35 systemd started it again
      (`NRestarts=1`). corpusd's store ping was refused twice at 22:53:35
      and then connected. The backup's sync failed with `invalid
      connection`; the corpus sync and both sqlite copies succeeded, and
      the run continues as `partial`.
    - Impact: Store A unresponsive for about 90 s and down for about 13 s.
      The site answered 200 afterwards; the paper's liveness probe
      reported nothing. No OOM kill. Run 2's 36-minute platform sync
      (item 12) was very likely the same throttling, without three missed
      probes in a row (no SIGABRT in the run 2 window).
    Why it was not seen before: runs 1 and 2 synced the same store and
    survived; run 1 was fast (2 min), run 2 slow (36 min), and the sync's
    effect on the server's memory was not watched. The restore drill
    tested the data, not the cost of taking it.
    Fix chosen (orange, the backup script): raise Store A's `MemoryHigh`
    at runtime only for the length of the platform sync (to 8G; the
    host has about 11 GB available then, before restic starts), then
    restore the exact value read before, in a trap, so a failed script
    still restores it. If the raise is refused, the platform sync is
    skipped and the run is `partial`, rather than syncing at 3G. To be
    proven by a supervised unit run before the 02:30 timer, watching
    Store A's memory pressure and the probe. Longer term: a filesystem
    snapshot of the Dolt data directory instead of a server-side sync
    (no server work at all).
16. **The long-term fix, now (owner, 2026-10-20: "lets do the longterm
    now").** Capture each Dolt store as a read-only btrfs snapshot of its
    server's data directory, and let restic read the snapshot.
    - Why it is safe: a btrfs snapshot is atomic across the subvolume, so
      the copy is what a power cut at that instant would leave, which
      Dolt's chunk journal is built to recover. A file-by-file copy (even
      reflinks) is not: the manifest and the table files could come from
      different moments, so it is never used.
    - What it removes: the server-side `DOLT_BACKUP` (the incident of
      item 15, and run 2's 36 minutes), the memory raise of 43b5bc034,
      and the standing `.choir-backup` copies (34.5 GB) once a restore
      drill has passed on the new path.
    - Precondition: the data directory is its own subvolume. Today
      neither is (both live in `@`). The servers' working directories:
      Store A `platform-dolt/platform`, Store B `corpus-dolt` (which also
      holds its `.choir-backup`; the snapshot's copy of it is excluded).
    - One-time migration per store (`scripts/dolt-data-to-subvolume.sh`):
      record the head, `systemctl stop` the server (which also stops the
      units that `Require` it: corpusd, checkpointd and the history audit
      for Store A; corpusd for Store B; vmctl only `Wants` Store A and is
      not stopped), `btrfs subvolume create`, `cp -a --reflink=always`
      (metadata only), swap the directories, start the server and its
      dependents, and compare the head. The old directory is kept as
      `<dir>.pre-subvolume-<time>` until the drill passes. Downtime:
      the stop, the copy (seconds) and Dolt's start.
    - The backup script takes the snapshot path when the directory is a
      subvolume and falls back to the sync (with the memory raise)
      otherwise, so it is safe before, during and after the migration.
      Snapshots live at fixed paths under `staging/dolt/` (so restic
      matches unchanged files with the previous run) and are deleted at
      exit, and before a new one is taken.
    - To prove: a supervised run on the snapshot path with Store A's
      memory pressure watched; a restore drill that starts `dolt
      sql-server` on the restored copy on a side port and reads the head
      the run recorded.
17. **Run 3 (22:39:44 to 23:58:44 UTC, 1 h 19 min): partial.** The
    platform Dolt sync failed (the item 15 incident); the corpus synced
    (head `bajdilhd…`), both SQLite copies were ok, and snapshot
    `127fc899` added 9.3 GB with the source down to 231 GB after the image
    change. The repository check passed. The run ended partial (exit 3)
    only for the platform sync. Two findings:
    - **The daily check runs at the memory line.** `restic check` (no
      data read) held the unit at its 10G MemoryHigh with its 4G swap
      full for 25 minutes. It finished, but slowly, and the unit read
      2.3 TB from disk for a 231 GB source. The likely cause is reclaim
      under MemoryHigh forcing restic to re-read its index and cache.
      This is a hypothesis; the per-phase I/O has not been measured. Run
      2's unexplained 669 GB read is probably the same effect. Next:
      measure the phases on run 4, and consider moving the daily check
      to the weekly verify unit.
    - **The deploy disk floor.** Free space fell from 97 GB (20:34) to
      75 GB during the run (the image reflink clones diverging, plus the
      restic cache), and deploys of 194fd31c9 and be786b801 failed the
      90 GiB preflight. The Nix collection freed nothing (four
      generations, no dead paths). `platform-dolt/.choir-backup` (31 GB,
      the old sync's target, outside the server's data directory) was
      deleted at 00:05 UTC on 2026-10-21, which brought free space to
      108 GB. Off-host, the copy from run 2 remains; tonight's restore
      drill has restored it. Consequence: until the snapshot path is
      deployed and the store is migrated, a fallback sync would rebuild
      the copy in full.
18. **Run 4, the first on the snapshot path (started 00:10 UTC on
    2026-10-21).** Both stores were snapshotted, not synced: no
    DOLT_BACKUP, no memory raise, and Store A's memory pressure stayed
    at 0. Snapshot `cbe1e08b` added 6.5 GB and was written 27 minutes in
    (run 3: 45). The repository check then ran for more than 73 minutes
    (run 3: about 34), again at the memory line: 10.3 GB resident, its
    4 GB of swap full, and about 290 MB/s of disk reads by page fault
    with no read calls (`rchar` flat). Two findings, measured on Node B
    at 01:40 to 01:50 UTC:
    - **The source is 9.7 million files in about 1,440 directories.**
      Almost all of them are three flat directories of the artifact
      store: `sha256/pin-receipts` (4.58 million),
      `sha256/computer-event` (2.29 million) and
      `sha256/computer-event-payload` (2.29 million). restic stores a
      directory as one tree blob, so each of these is a single tree of
      hundreds of megabytes. The repository's index is 407 MB and its
      tree packs 2.3 GB, for a 107 GB repository. `restic check` holds
      those trees in memory, which is the memory line. This replaces
      item 17's reclaim hypothesis with a measured shape; the per-tree
      memory is still inferred, not measured.
    - **The daily check downloads every tree again.** Without
      `--with-cache`, `restic check` builds a new temporary cache
      (`restic-check-cache-*`, 2.3 GB at 01:50) and so pulls all tree
      packs over the forward each day. That is most of the run's
      incoming traffic (6.9 GB in run 3).
    Options, none taken yet: (a) `check --with-cache`, which removes the
    download but not the memory; (b) move the structural check to the
    weekly verify unit, so the daily run ends after the snapshot; (c)
    shard the three flat directories by prefix in the artifact store,
    which shrinks every tree and also helps the file system and any
    restore. (c) is a substrate change in the artifact store and needs
    its own problem record; (a) with (b) is the small step.
    **Outcome and decision (02:17 UTC).** At 02:15 the check had run 98
    minutes with CPU time growing about a quarter of a core, so I stopped
    the unit (`Result=signal`; 2 h 7 min, 3.6 TB read). The snapshot
    `cbe1e08b` was already committed on node-a; the exit trap deleted
    both Dolt snapshots and restic left no process. More memory does not
    fit: Node B has 31 GB, about 14 GB free beside the guests, and the
    check's working set was above that. Decision: options (a) and (b).
    The daily run no longer runs `restic check`; the weekly verify runs
    `check --with-cache --read-data-subset`. Failure modes 7 and 8 in
    `scripts/off-host-backup-dolt-snapshot-test.sh`. The 02:38 run of
    2026-10-21 was skipped by stopping its timer at runtime (the deploy
    starts it again). Residuals: the weekly verify still walks every tree
    under the same cap and may run for hours; option (c), sharding the
    three flat directories, is the real fix and needs its own record; the
    snapshot restore drill (`snap`) against `cbe1e08b` has not run yet.
19. **Run 5's first start (02:37:31 UTC) was refused: a stale lock.** The
    script reported "repository is not initialised or unreachable"
    because `restic cat config` met the exclusive lock of run 4's check
    (created 00:46:42, never deleted; the rest-server log has no DELETE
    for it). Stopping the unit sends SIGTERM to the whole cgroup, so the
    ssh tunnel died with restic and restic could not reach the server to
    delete its lock. `restic unlock` from Node B at 02:50 removed it (the
    append-only server allows lock deletion), and run 5 was started by
    hand at 02:50:5x. Fix: the daily run calls `restic unlock` (stale
    locks only) before opening the repository; failure mode 9 in the
    snapshot test. The same fate-sharing would hit any killed run.
20. **Run 5 (02:50:55 to 03:27:29 UTC, 36 min): ok**, the first clean run
    since run 2, and the first without the daily check. Snapshot
    `0a731df5` added 10.4 GB in 29 minutes. The remaining memory line was
    `restic stats --mode raw-data`, which sized the repository by walking
    every snapshot's trees: about 7 minutes at a 10 GB peak with 2.9 GB of
    swap. Fix: the daily run sums the pack sizes the rest-server lists
    (REST API v2, `data/`). Measured on the live repository at 03:30:
    112,391,327,717 bytes in 1,627 packs in 0.27 s, against stats'
    111,985,828,263 (the pack sum also counts pack headers, so it errs high,
    the safe side for the 80% quota alert). Failure mode 10 in the snapshot
    test; `curl` joins the unit's PATH.
21. **Snapshot restore drill on the new path: passed (04:50 to 04:56 UTC
    on 2026-10-21).** `off-host-restore-drill.sh 0a731df5 snap` in the
    capped unit (MemoryHigh 8G, MemoryMax 10G, idle I/O). The first
    attempt exited at once: a `systemd-run` unit on NixOS has no
    coreutils on its PATH (`install: command not found`), so the unit
    needs `-p Environment=PATH=/run/current-system/sw/bin`. Then, from
    node-a, each store was restored into scratch and opened with Dolt,
    as after a power cut:
    - corpus: 3.7 GB in 36 s; head `s3517bp4…` found, 31 tables; it is
      the head run 5 recorded.
    - platform: 31 GB in 4 min 18 s; head `614mf7rf…` found, 70 tables;
      it is the head run 5 recorded.
    The unit peaked at 7 GB, with no swap. Each restore was deleted before
    the next. This is the drill the migration waited for, so the retired
    copies were removed at about 04:57 UTC: `corpus-dolt.pre-subvolume-*`
    (7.0 GB), `platform-dolt/platform.pre-subvolume-*` (31 GB),
    `corpus-dolt/.choir-backup` (3.5 GB, the old sync's target), the
    drill's restic cache, and the one-off `/root` scripts
    (`off-host-backup.sh`, `dolt-data-to-subvolume.sh`,
    `offhost-unlock.sh`). `dolt-backup-20261015-pre-2.4.2` is kept. All
    Dolt servers, corpusd and vmctl stayed active, and the Dolt liveness
    probe passed. Free space on `/` went from 139 GB to 143 GB at once;
    the reflinked extents the old copies shared with the live stores are
    not freed, by design. Still open: the tape and image pieces of the
    drill on this snapshot, and the weekly verify's cost, which waits for
    the segment store (`choir-tape-segments-a2-repack-2026-10-21.md`).
