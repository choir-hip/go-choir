# Data architecture research, 2026-10-28

Status: evidence for docs/definitions/choir-data-architecture-selection-2026-10-28.md.
It follows the consensus record docs/evidence/data-architecture-consensus-2026-10-28.md.

Owner request: "do as much more research as we can, but delegate that to
DeepSeek and or Devin."

Two research seats ran in parallel, read-only, with web search:

- Devin on the hypervisor;
- DeepSeek on desktop sync, the SQLite witness, the object store and the
  tape acknowledgement.

Their raw memos, with full source lists, are in
`.agentic-consensus/data-arch/research/out-{devin,deepseek}/` (gitignored).

## FP0 spot-checks against primary sources

These were checked on 2026-10-28 with the GitHub API unless stated.

- Cloud Hypervisor's latest release is v54.0, from 2026-10-14. [V]
- Cloud Hypervisor issue #8645, "Feature Request: Differential snapshots
  (snapshot_type=diff)", is open and was filed 2026-07-28. [V]
- Firecracker issue #6255 is open. It is the memory hotplug failure that
  matches Node B's 39-bit CPU problem. [V]
- The `minio/minio` repository is archived (`archived: true`). Secondary
  sources date the "no longer maintained" notice to February 2026. The
  community fork is pgsty/minio. [V]
- DoltLite exists. It is a SQLite fork whose storage is a prolly tree, and
  it went to beta on 2026-08-31 (DoltHub blog). Neither of us confirmed
  that it exposes `dolt_hashof_table`; it does expose
  `dolt_hashof_catalog`. [V/H]

## Hypervisor (Devin)

- **Cloud Hypervisor has no incremental (diff) snapshots.** Its snapshots
  are full but sparse on disk, so a hibernate write is proportional to the
  pages touched. It restores lazily through UFFD (`ondemand`) or by
  copy-on-write (`copyonwrite`). Its default restore mode is an eager copy,
  so the seam must choose the mode explicitly; otherwise every wake slows
  down.
- **Moving from Firecracker to Cloud Hypervisor.**
  - We lose:
    - the density baseline we have measured;
    - Firecracker's enforced spec envelope (at most 125 ms to start, at
      most 5 MiB of VMM overhead);
    - lazy mmap restore as the default;
    - the jailer, which the planned choir-cage replaces;
    - one cold boot per computer at cutover.
  - We gain:
    - live migration, including same-host migration for VMM upgrades, of
      about 50 ms;
    - virtio-mem whose region sits inside guest RAM space, so it probably
      works on Node B's 39-bit CPU, though that is unproven at runtime;
    - VFIO and GPU passthrough;
    - copy-on-write fork fan-out;
    - core scheduling;
    - virtio-fs.
- **Live migration requires `shared=on` guest memory.** That excludes
  copy-on-write restore and changes how memory is accounted. It is a choice
  made per computer at boot.
- **Staged path:**
  1. Finish the seam on Firecracker.
  2. Run Cloud Hypervisor on the test ring.
  3. Keep Firecracker for Autoradio's session continuity.
  4. Bring in Cloud Hypervisor migration when a second host of the same CPU
     vendor, or a GPU, gives it a mission.
- **Desktops:**
  - macOS must use Virtualization.framework (arm64 only). It saves state in
    a monolithic, config-fragile file.
  - Linux desktops can run the same Firecracker or Cloud Hypervisor binary.
  - Windows is the weak spot: an HCS utility VM like WSL2's, or nested KVM
    inside WSL2.
- **What the seam must abstract:**
  - memory backing (private or shared);
  - the restore-mode enum;
  - the snapshot type as a capability;
  - networking by fd plus a user-mode option for desktops;
  - the boot envelope of each guest image;
  - reboot semantics;
  - a confinement declaration;
  - backend identity in the launch record.
- **Open questions:**
  - whether Cloud Hypervisor's virtio-mem plugs on Node B;
  - memory accounting under `shared=on`;
  - the overhead of a WSL2/HCS utility VM;
  - whether a Virtualization.framework save file survives an app update.

## Desktop sync and drive (DeepSeek)

- **One sync core with three thin adapters:**
  - Apple File Provider (`NSFileProviderReplicatedExtension`, macOS 11+
    and iOS 16+);
  - the Windows Cloud Files API (NTFS only; needs Desktop Bridge
    packaging);
  - Linux FUSE, since Linux has no native API.

  The extension stays thin. A launchd or Windows service daemon owns the
  connection and the cache.
- **The model fits:**
  - a stable ref ID is the item identity;
  - `contentVersion` is the content hash and `metadataVersion` is the ref
    revision;
  - Apple's sync anchor is a 24-byte value (projection epoch, tape sequence,
    change-log hash).

  A retained change log, derived per domain, answers enumeration. Scanning
  the tape on every enumeration would cost O(tape), which is not
  acceptable. Apple's working set must track materialized items, otherwise
  every change must be signalled.
- **Lease handover needs a fencing epoch**, after Kleppmann. Every tape
  append carries the epoch, and the append authority rejects stale epochs.
  The lease authority is host-side.
- **Offline fork-and-reconcile later.** Add to the tape schema now:
  `event_id` (hash of parents and payload), `parents[]`, `branch_id`, and
  per-object causal metadata. Retrofitting a linear tape into a DAG would be
  a migration.
- **Platform limits:**
  - many apps fully download a file rather than streaming it;
  - packages, Photos libraries and similar formats are unsupported;
  - Windows works on NTFS only.

## SQLite witness (DeepSeek)

- **No stock SQLite equivalent of `DOLT_HASHOF_TABLE` exists.** The
  construction DeepSeek proposes:
  - a per-table multiset hash (LtHash32 or MuHash3072) over canonical
    full-row images;
  - maintained by triggers in the same transaction;
  - each table's state is O(1) and each row update costs O(1);
  - the global root hashes the schema plus the sorted per-table roots.
- **Pitfalls reproduced locally on SQLite 3.53.4:**
  - `INSERT OR REPLACE` and `UPDATE OR REPLACE` diverge unless
    `recursive_triggers=ON`; forbid REPLACE on authority tables;
  - `VACUUM` renumbers rowids, so hash full rows, never `rowid`;
  - `ALTER TABLE ADD COLUMN ... DEFAULT` changes rows without firing
    triggers, so the table root needs a rebuild;
  - the update hook misses REPLACE deletes.
- **DoltLite is a third candidate** for the bake-off. It gives a per-table
  content root and branch/merge, which may also help the offline fork
  feature. It is beta, has no WAL and allows one writer.

## Object store (DeepSeek)

- **Now:** a local CAS directory with two-level fanout, 64 MB pack files,
  `O_EXCL` puts, BLAKE3 verification on read, chunk manifests for media,
  and mark-and-sweep GC from refs. This is also the self-hosted
  single-box backend.
- **Later:** SeaweedFS, which is Apache-2.0, keeps a 16-byte in-memory
  index per object and has a single-node mode. Use 1 GB volumes, because
  vacuum needs free space equal to the volume being compacted. The
  community edition has no auto-heal, so we would run repair ourselves.
- **Rejected:**
  - MinIO, because it is archived and unmaintained;
  - Garage, because its metadata collapses at about 10 million objects, it
    has no conditional writes, and LMDB can corrupt on power loss;
  - Ceph, which needs 4 to 8 GB per OSD and three or more nodes.
- **Encryption:** per-owner envelope encryption at the application layer,
  with AEAD. Private object IDs are keyed so that equality between owners
  does not leak. No convergent encryption for private content.

## Tape acknowledgement (DeepSeek)

- **The acknowledged-write boundary is a quorum of signed receipts from
  off-host relays**, two of three or three of five. Object storage is the
  asynchronous cold tier. This is the pattern Cloudflare SRS and Neon's
  safekeepers use; Litestream does not do this.
- **Latency:** a same-region quorum is estimated at p99 10 to 30 ms (an
  estimate), within the 100 ms gate.
- **Receipts carry the lease epoch**, so the relays are also the fencing
  check.
- **New gate question:** a self-hosted single box cannot provide an
  off-host receipt. Its options are a second machine or NAS, a remote
  bucket, or a documented weaker guarantee.

## What changes in the selection

1. The bake-off gets a third authority candidate, DoltLite, and a dedicated
   witness spike: triggers with LtHash32, a single writer with MuHash, and
   DoltLite, all tested against REPLACE, VACUUM, ALTER and UPSERT.
2. The tape ack is a relay quorum. Build and measure a relay prototype,
   covering kill -9, partition and stale epochs, before migration.
3. The tape schema should become fork-ready (event_id, parents, branch_id)
   before the per-computer migration, even though the offline feature is
   deferred.
4. The object store interface ships with the local backend first. MinIO is
   out.
5. The hypervisor stays on Firecracker. Cloud Hypervisor enters through
   the seam on the test ring. Its first real capability may be virtio-mem
   on Node B.
6. Owner decision needed later: what "off-host" means for a self-hosted
   single box.
