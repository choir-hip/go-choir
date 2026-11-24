# BACKUP-DRIVE: the ChoirArchive external drive

Status: written 2026-11-24 (green; doc only). The drive is a **single copy**, not a
permanent backup (owner, 2026-11-24: "I'll have a better, more permanent backup
solution later"). It is not part of the repo and not mounted on any server. This
page is a copy of the text of the READMEs on the drive, so the repo knows the
drive exists and what is on it. The drive itself is the authority; where this
page and the drive differ, the drive's files (and their manifests) are right.
The prune proposal (`PRUNE-PROPOSAL.md`, in the dated backup folder on the
drive) is deliberately not in the repo; it awaits two review seats and the owner.

Related: [docs/system-map.md](../system-map.md),
[the storage definition](../definitions/choir-storage-efficiency-2026-11-22.md).

---

## Root README (`/Volumes/ChoirArchive/README.md`)

A 480 GB external drive (APFS, not encrypted) that holds copies of data taken
from the Choir staging host ("Node B", ssh alias `node-b`, at choir.news) on
2026-11-24 (UTC), so that data can be pruned from Node B's disk without being lost.

**This is a SINGLE copy. It is not a permanent backup.** There is no second
copy of this drive. If this drive fails, is lost or is erased, the data on it
exists nowhere else except on Node B, where some of it is about to be pruned.
The owner said (2026-11-24) a better permanent backup will come later. Until
then, do not delete the source of anything here unless this drive's check
(VERIFY.sh, below) passed recently and the owner has decided.

The drive is not encrypted. Several folders hold private text (the paper's
database, event logs, captured web pages, the accounts database). Keep the
drive somewhere safe. Do not copy it to a cloud service.

### Folders

- `paper-db/` : a copy of the paper computer's main database (`state.db`, 55.7 GB).
  **It is torn: it failed SQLite's integrity check.** Read
  `paper-db/README.md` before you open it.
- `backups/` : copies of files from Node B's disk. Today there is one dated
  folder, `backups/node-b-2026-11-24/`: the event logs ("tape segments") of
  27 computers, snapshot chunks and file roots of 4 computers, 637,497
  captured web page bodies, and a small staging copy of two databases. About
  186 GB. Read `backups/README.md`, then the dated folder's `README.md`.
- `evidence/` : small proof files from experiments (voice model samples).
  Read `evidence/README.md`.

### Checking the drive

`bash /Volumes/ChoirArchive/VERIFY.sh` re-reads every file and checks it
against the `MANIFEST.sha256` file that sits in each data folder. It prints
one PASS or FAIL line per manifest. It only reads. It takes a long time (it
reads about 240 GB; plan on an hour or more). Every file on the drive that is
data is listed in some manifest with its SHA-256.

Last run: 2026-11-24, finished 05:43Z (UTC): overall PASS. Eight manifests, eight PASS lines: capture-bodies 637,497 files; chunks 84,409; roots 2,462; tape-seg 9,313; the dated folder's own files 135; staging 3; paper-db 13; evidence 11. It took about 34 minutes.

### What is NOT on this drive

- Node B's two Postgres databases (`control`, 29 GB; `graph`, 13 GB).
- The key escrow and signing keys. **Private event payloads inside the tape
  segments are encrypted per computer; the keys that open them are not on
  this drive.** Without them those payloads cannot be read. The keys live
  in Node B's control database and under Node B's `platform-artifacts`.
- Other computers' `state.db` files and VM disks.
- Node B's own nightly restic backup repository (where it is stored was not
  checked).

### Where to read next

Everything needed to understand the drive is on it. The project's code
(github.com/yusefmosiah/go-choir) explains the formats; in the repo,
`docs/reference/BACKUP-DRIVE.md` is a copy of these READMEs, and
`docs/system-map.md` is the overview of the system.

---

## backups/README.md


Files copied from Node B's disk, one dated folder per copy. Node B is the
Choir staging host (ssh alias `node-b`). Only reads were done on Node B:
`rsync` over ssh with `sudo rsync` as the sending side. Nothing was written,
moved or deleted there.

- `node-b-2026-11-24/` : the first copy. See its `README.md`, and its
  `PRUNE-PROPOSAL.md` for what could be removed from Node B now that this
  copy exists (a proposal only; nothing has been pruned).

Each dated folder has its own `README.md` and `MANIFEST.sha256` files.
`bash /Volumes/ChoirArchive/VERIFY.sh /Volumes/ChoirArchive/backups` checks only this folder.

---

## backups/node-b-2026-11-24/README.md


Copied from Node B (Choir staging host, ssh alias `node-b`), under
`/var/lib/go-choir/platform-artifacts/` unless a row says otherwise. Copied on
2026-11-24 between 02:27Z and 04:42Z (UTC), by `rsync -a --partial` over ssh
with `--rsync-path="sudo rsync"` (read side only). Scripts and logs are in this
folder. Node B was read only: nothing was written, moved, deleted, pruned,
restarted or snapshotted there, and no computer was woken or paused. This is a
single copy; see `/Volumes/ChoirArchive/README.md`.

## What is here

| Folder here | Node B source | Files | Bytes | Copied (UTC, from the logs) |
| --- | --- | --- | --- | --- |
| `tape-seg/computer-1355b83b.../` (the paper) | `tape-seg/computer-1355b83bced1613eb048ee28d66688a8/` | 4,333 | 98,234,153,737 | pass 1 02:27:10 to 03:37:42; pass 2 03:37:45; final sync 04:38 |
| `tape-seg/` 3 other living computers (owner's `0333...`, test `59c9...`, test `6c50...`) | same, per computer | 1,311 / 977 / 291 | 8,165,677,128 / 6,572,622,841 / 1,390,207,489 | 03:37:52 to 03:48:09; final sync 04:38 |
| `tape-seg/` 23 computers that no longer exist (removed or never living) | same, per computer | 2,401 | 6,525,132,471 | 04:12 to 04:37 |
| `sha256/file-cas-chunks/<computer>/` (snapshot chunks, 4 living computers) | `sha256/file-cas-chunks/<computer>/` | 84,409 | 27,243,629,864 | 03:48:09 to 04:04:08 |
| `sha256/file-cas-roots/<computer>/` (file roots, same 4) | `sha256/file-cas-roots/<computer>/` | 2,462 | 4,034,851,853 | to 04:04:18 |
| `capture-bodies/<2 hex>/<sha256>` (captured web page bodies of the whole platform) | `capture-bodies/` | 637,497 | 43,439,825,171 | 04:04 to 04:37; catch-up pass 04:41:58 |
| `offhost-backup-staging/` | `/var/lib/go-choir/offhost-backup/staging/` | 3 | 4,227,305 | 04:04:18 |

The first four tape rows and the others add up to 113 GB on this drive.
The computer ids are the host's names for computers. "The paper" is the
computer that runs the Autopaper; its events and snapshots are by far the
largest.

## What a tape segment is

A computer's event log (its "tape") is stored on the host as numbered
segment files: `seg-00000001.seg` (records) with a matching `.idx` (index).
A segment is sealed at about 64 MB. The highest-numbered `.seg` of a living
computer is the live head: it was still being appended to while it was
copied, so the copy of that one file per computer is shorter than the source
(see "Known differences"). The format is in the Choir repo,
`internal/tapestore/format.go` (record bodies are zstd frames; every record
is checked against its SHA-256 digest). Read it with that Go package, or run
`taperepack census --root <dir containing tape-seg/>` (read-only), or
`ledger-witness --artifacts-root <dir>`. This folder works as such a root:
`tape-seg/<computer>/` is laid out as the tool expects.

**Private payloads are encrypted** with a per-computer key held in the host's
key escrow. The key escrow is NOT on this drive. The segments' structure,
digests and any non-private records can be read without it; private payload
bodies cannot.

## Known differences from the source

- Live heads: `seg-00002167.seg` (paper), `seg-00000656.seg` (owner's),
  `seg-00000146.seg` (test `6c50`) were shorter here than on Node B at the
  last check (by 401,072 / 7,256 / 7,310 bytes: they grew after the last
  sync). The `59c9` computer matched exactly. Every sealed file matched in
  name and size. New segments written after 04:38Z are not here.
- `capture-bodies` grows all day (about 90 new files in four minutes). At the
  last comparison (04:42Z) Node B held 637,587 files and this folder 637,497;
  the 90 are bodies that did not exist at copy time.
- `capture-bodies` is the whole platform's store, not only the paper's: the
  directory is not split by computer.
- Not copied at all: see "What is NOT on this drive" in the root README, and
  Node B's `sha256/computer-event`, `computer-event-payload`, `pin-receipts`
  (old loose tape files, covered by Node B's own restic job), `sha256/og`,
  `sha256/platform-update`, `capture-fts` (a rebuildable search index),
  `artifact-gc-log`, `tape-sweep-archive`, `vector-import`.
- `offhost-backup-staging/sqlite/` holds `auth.db` (the accounts database)
  and `mail.db`. These are sensitive. The files are as staged by Node B's
  backup job on 2026-11-23 21:44Z, not a fresh dump.

## How it was verified

All checks compare this drive with Node B at the time shown.

1. File lists, sizes: on Node B `sudo find <dir> -type f -printf '%P\t%s\n'`
   (read only); here `find ... stat -f '%N%t%z'`; both sorted and diffed.
   Result: every file name and size matched except the live heads and the new
   capture bodies listed above. Chunks, roots and staging matched exactly
   (84,409 / 2,462 / 3 files and bytes).
2. SHA-256 on both sides: a random sample of 230 paper segments and index files
   older than one hour (`sudo xargs sha256sum` on Node B, `shasum -a 256`
   here): 230 of 230 identical. A second sample of 150 (30 from each of the
   other three living computers, up to 4 from each of the 23 computers that no longer exist):
   150 of 150 identical.
3. Chunks and capture bodies are content-addressed (the file name is the
   SHA-256 of the contents). A check of 200 random paper chunks: name equals
   SHA-256 in 200 of 200. `VERIFY.sh` recomputes every hash and compares
   against the manifest.
4. Full content check from the manifests (05:08Z): in all 637,497 capture
   bodies and all 84,409 snapshot chunks, the file name equals the SHA-256 of
   the file's contents (0 exceptions). Roots are named by another digest and
   were not checked this way.
5. `VERIFY.sh` (at the drive root) checks every `MANIFEST.sha256`; the result
   of its last run is in the root README.

## What on Node B corresponds to it

The same paths under `/var/lib/go-choir/platform-artifacts/` (above). The
paper's `tape-seg` and `file-cas-chunks` there are the authoritative live
copies. This folder is a point-in-time copy.

## How to read or restore

- Look at files: they are ordinary files. `ls tape-seg/computer-1355b83b*/ | head`.
- Read segments: use the Choir repo's `internal/tapestore` (Go), or the
  `taperepack census` command above. Do not write into this folder.
- Restore to a host: `rsync -a <this folder>/tape-seg/<computer>/
  <host>:/var/lib/go-choir/platform-artifacts/tape-seg/<computer>/` while the
  computer is stopped. Not rehearsed. A restore also needs the matching
  Postgres rows (`control` database) and the key escrow, which are not here.
- Snapshot chunks and file roots go back under `sha256/file-cas-chunks/` and
  `sha256/file-cas-roots/` the same way.

## Other files in this folder

`PRUNE-PROPOSAL.md` (what could be pruned on Node B; proposal only),
`rsync-*.log` and `*.sh` (the exact commands run), `MANIFEST-paper-tape.txt`
(first manifest, sizes and sample hashes), `_verify/` (file lists and hash
samples used for the checks above).

---

## paper-db/README.md


Copies of the paper computer's main database. Today: one folder.

- `paper-20261124T0215Z/` : see its `README.md` (and the older `README.txt`
  written by the person who took the copy).

**Warning: this copy is torn. It failed SQLite's integrity check.**

Source on Node B: the guest disk image of the paper computer under
`/var/lib/go-choir/vm-state/vm-1669020adcece51c2f5ff9b6df399ef5/` (the computer
id is `computer-1355b83bced1613eb048ee28d66688a8`). The paper is the computer
that runs the Autopaper. Its `state.db` is a SQLite projection of its tape:
the tape (see `backups/`) is the record, the database can in principle be
rebuilt from it plus snapshots.

---

## paper-db/paper-20261124T0215Z/README.md


Files: `state.db` (55,698,690,048 bytes), `state.db-wal` (65,829,392),
`state.db-shm` (32,768), plus the check outputs `integrity.txt`, `sha256.src`,
`sha256.dst`, `move-verdict.txt`, `start.txt`, `db-done.txt`, `all-done.txt`,
`move-done.txt`, and the original note `README.txt` (kept; this file adds to it).

## What it is

A copy of the paper computer's `state.db` with its `-wal` and `-shm`. It was
taken starting 2026-11-24 02:08:56Z, by streaming `debugfs` (read only) out
of the **live** guest image `data.img` on Node B over ssh, for about 35 minutes
(state.db finished about 02:43Z; wal and shm after). Nothing was written on
Node B. The database was in use while it was read.

## Known defect: it is not consistent

Writes landed while it was being copied, so pages from different moments are
mixed. `PRAGMA integrity_check` (run with `sqlite3 -readonly`) **failed**: the
first 100 errors are in b-trees 49, 50, 93, 94 and 97 (the
`texture_agent_mutations` table and its indexes) and 58 (`og_objects`):
out-of-order rowids, bad pointer-map entries, page-init errors. See
`integrity.txt`. A full table scan of `og_objects` aborts. Most other tables
were not named in the first 100 errors, but that is not a proof they are sound.

### Still good for

- Looking at rows by rowid range in tables outside the named trees.
- Counting, sizing and sampling for analysis (what the store is made of).
- Salvaging rows with rowid-range queries from the damaged tables.
- Experiments that tolerate a damaged file.

### Must NOT be used for

- A restore of the paper computer, or any attempt to "repair" and boot it.
- Any claim that data is complete or that a row is absent (a missing row may
  be a torn page).
- Writing to it. Open only read only: `sqlite3 -readonly state.db` (mind that
  with the `-wal` present, SQLite may want to write; copy the three files to
  another disk first if you must run anything beyond read-only queries).

A consistent copy must come from the tape (rebuild by replay) or from a
snapshot plus tape tail; see `backups/node-b-2026-11-24/` (the tape and the
snapshot chunks are there; the key escrow is not).

## How it was verified

The three files were streamed to this Mac's disk, then moved here. SHA-256
(`sha256.src` taken before the move, `sha256.dst` after): identical, verdict
`MATCH`:

```
6be22db3995b495791ec16dca916c7ea34a65184dcb1c17710f4e276e8b6d4f8  state.db
95bf0e2d20aef0e8868630d17b5aa0038f3eae06556382d93e07fd7e947760d5  state.db-wal
2fd8d86120ad514dad82eff915146eb005ea409de0c02798743d460c8df56e81  state.db-shm
```

That proves the move was exact. It does not prove the data is sound (it is not).
`MANIFEST.sha256` here lists every file in the folder; `VERIFY.sh` checks it.

## What on Node B corresponds to it

The live `state.db` inside the paper's `data.img` (the file is still there and
changing). This copy is a moment-in-time (02:08Z to 02:43Z) smear of it.

## What is not in it

Nothing from other computers; no tape; no keys.

---

## evidence/README.md


Small proof files from experiments. Not a backup of anything.

- `omni-voice-2026-11-24/` : ten short audio files (`answer1..5.wav`,
  `transition1..5.wav`) made on 2026-11-24 on this Mac with the LFM3-Omni-2B
  voice model served by llama.cpp, with the timings of each. Its own table is
  in [omni-voice-2026-11-24/README.md](omni-voice-2026-11-24/README.md).
  Source: generated locally, no Node B data. About 3.3 MB. Check with
  `VERIFY.sh` (it has a `MANIFEST.sha256`).
