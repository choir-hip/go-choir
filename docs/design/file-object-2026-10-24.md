# The file object, first slice (2026-10-24)

Status: design for slice 1 of migration step 1 in
`docs/problems/data-has-more-than-one-home-2026-10-24.md` ("the file object").
Doctrine: `I27` (a file is a blob in the computer's content-addressed store,
referenced by digest, plus a path view; the files tree is a view, never a
home), non-conformance `H038`. Author: FP0 lane `fp0-file-object`.

Slice 1 makes the file a graph object, written on every Files API mutation
and by a reconcile pass, with a read path. It does not move the bytes, does
not change file CAS or file sync, and repoints none of the four path-string
references yet; it builds the id they will point at.

## 1. The kind

Two kinds, both `Store: dolt`, `IdentityMode: external_key`,
`Owner: internal/store (computer)`, `Writer: system:runtime`:

- **`choir.file`**: the file. Identity is a **stable file id** (`file_` plus
  128 random bits, hex), minted when a path first gets a file. The id never
  encodes the path, so a rename or move keeps it. Body (`store.FileObject`):
  `file_id`, `owner_id`, `computer_id`, `path` (files-root relative, slash
  separated, cleaned; **literal**: a backslash or a space is part of a name,
  as on the disk), `name`, `digest` (`hmac-sha256:<hex>`: HMAC-SHA-256 of the
  plaintext bytes under a key derived from the computer's DEK, section 3),
  `size`, `mode` (permission bits), `media_type` (by extension),
  `mod_time` (filesystem mtime, used only to skip re-hashing), `state`
  (`present` or `deleted`), `created_at`, `updated_at`, `deleted_at`,
  `recorded_by` (`files_api` or `reconcile`).
- **`choir.file_path`**: the path view, one object per live path, keyed by
  `computer_id + NUL + path`. Body: `path`, `file_id`, `computer_id`. A
  lookup by path is one primary-key get, not a JSON scan.

Directories are not objects. They are prefixes of paths in the view.

**Writer.** `system:runtime`, the VM store acting for the Files API (which
acts for the authenticated owner) and for the reconcile pass. `owner` cannot
be a kind's writer (`objectgraph/writer.go`: owner is declared by a write and
admitted only through `OwnerEdits`). The owner's act is the authenticated
Files request; the runtime records it. No desk writes either kind in this
slice. The writes declare `WithWriter(ctx, system:runtime)`.

**Versioned: no.** Each kind is a latest-state object (`Versioned: false`).
Every change is one projection batch on the tape, so the history of a file's
digest is the tape's history. Revision objects (`choir.file_revision`) come
with byte rewind (section 4), not before.

**Delete keeps the object.** Delete sets `state=deleted` and `deleted_at`,
and tombstones the path view. A reference to the file id then resolves to "a
file that was deleted at T", not to nothing. A new file later written at the
same path gets a new id. Garbage collection of deleted objects is later.

## 2. Keeping it true: the Files API and reconcile

All writes go through one in-process recorder (`autoputer.fileObjects`).
Every write first checks the switch (section 7), and the store's
`ApplyFileChanges` checks it again (`ErrFileObjectsOff`), so a pass in
flight stops when the switch goes off. The recorder reads the file from disk
after the handler's write, hashes it by streaming, and writes one batch.
A file with no digest key (no DEK reachable) is not recorded at all: the
plaintext digest is never written in its place.

- **Write (`PUT /api/files/{path}`, created or updated).** Look up the path
  view. If it names a present file: same id, new `digest/size/mode/mod_time`.
  If not: mint an id, write the file and the path view in one batch.
- **Delete (`DELETE /api/files/{path}`).** For a file: the object goes to
  `deleted`, the path view is tombstoned, one batch. An empty directory has
  no objects; nothing is written.
- **Mkdir (`POST`).** Nothing is written.
- **Rename and move (`PATCH /api/files/{path}` with `{"move_to": "<dest>"}`,
  new in this slice, regular files only, refused when the destination
  exists).** The bytes move with `renameat2(RENAME_NOREPLACE)` on Linux (a
  hard link and an unlink where that is missing), so a destination created
  between the check and the move is refused with 409, never replaced; then one
  batch: the file object
  with its new `path`/`name` (same id), the new path view, and the old path
  view tombstoned. A rename is a path-view change, never a new object. A
  directory move (rewrite every descendant's view) is a later slice; the
  route refuses it with 409. The route is behind the same switch: switch off,
  `PATCH` answers 405 as it does today.
- **Reconcile.** Writers outside the Files API exist (Texture import of
  `.texture` shortcuts, model-policy seeding, the guest Maildir, boot
  hydration). Reconcile walks the tree, skips symlinks, non-regular files and
  the top-level `Source/` directory (source and build state are exempt under
  `I27`; `Source/platform` is large), compares each file's `(size, mod_time)`
  with its object and hashes only when they differ. (A regular file named
  `Source` is exempt too.) A file it cannot read (permission) is counted
  `unreadable`, never deleted, and does not stop the pass. A file with no view
  gets a new object; an object whose path is gone and a new non-empty file
  with the same digest, when exactly one of each, is recorded as a move (same
  id; empty files never pair, since their digest is every empty file's).
  **A vanished object is deleted only when a fresh `lstat` at apply time finds
  nothing there in two consecutive passes** (a file only temporarily absent
  keeps its id); a tree showing no file at all while objects exist holds every
  deletion, however few. The walk and the hashing run **outside** the recorder
  lock (the Files API is not held still); the apply phase re-checks each record
  against the disk (`size`, `mod_time`, `mode`) and drops one that went stale
  (`raced`), re-reads the switch before every batch, and writes in batches of
  64. Steady state on an unchanged tree writes nothing. It runs in the
  background after the runtime starts and after hydration, then every 15
  minutes (`CHOIR_FILE_OBJECT_RECONCILE_INTERVAL`), and on
  `POST /api/file-objects/reconcile`; a pass refuses to run until boot
  hydration has returned without error (`fileSync.HydrationSettled`). With the switch off a tick reads one
  setting and stops.

**Read path.** `GET /api/file-objects?path=P` (the object at a path),
`GET /api/file-objects?id=F` (by id, deleted included), and
`GET /api/file-objects` (present objects, paged by `after`, `limit`). Each
response carries `enabled` (the switch). Reads are not gated: with the
switch off and nothing written they return 404 or an empty list.

## 3. The digest and file CAS

The object carries a **keyed digest** of the whole file:
`hmac-sha256:<hex of HMAC-SHA-256(key, plaintext bytes)>`, where `key` is
`filecas.FileDigestKey(DEK, computerID)`, an HKDF-SHA-256 derivation from the
computer's DEK with its own info string (`choir/filecas/v1/file-digest`),
separate from the chunk encryption key and the chunk nonce key. The DEK is
the one the file sync already reads in the guest
(`PrivateArtifactCipher.ExportKeyForEscrow`); no new key plumbing was added,
and the key is derived per use and zeroed. Equal bytes on one computer have
equal digests (reconcile's change detection and move pairing use it); nothing
else can compute it.

This replaces the plaintext `sha256` of the first draft (tier 2 review, bunch
208). Projection batches are pinned as owner payloads the platform host can
read; the file CAS manifest already gives the host every path and size. A
plaintext digest would add a confirmation oracle (does this computer hold a
file I already know?) and cross-computer correlation. A keyed digest gives the
host neither.

**Costs and residuals, named.**
- The digest is not the blob identity of the image blob store
  (`state.image-blobs/sha256/<hex>`), so the object and an image blob do not
  join on it. The unification that wants a join keys the blob address the same
  way later, or carries a plaintext digest only in guest memory.
- A DEK rotation changes every digest; the next reconcile re-hashes and
  records them as `diverged` (the same path a restore uses). The rotation path
  does not exist yet.
- The keyed digest still reveals, to the host, which of this computer's files
  have equal bytes (equal digests), as the sealed chunk addresses already do.
- A computer whose digest key cannot be derived records nothing and reports it.
- A reconcile that finds a directory it cannot read skips it and holds every
  deletion under it (counted `unreadable`); the pass is not wedged.

File CAS is unchanged. Its chunks are 4 MiB pieces sealed with a synthetic
nonce, addressed by the SHA-256 of the **ciphertext**, deterministic per
(DEK, computer, plaintext chunk) (`filecas.SealChunk`); the manifest lists
`path, mode, size, chunks[]` (`filecas.FileEntry`). In slice 1 the object and
a manifest entry join on `path` at a root, and agree on `size` and `mode`.
The manifest gets no new field: `ParseManifest` uses
`DisallowUnknownFields`, so a manifest with a digest field would fail to
parse on a rolled-back binary and break hydration. Carrying the chunk list
on the object, or a v2 manifest with a per-entry digest, is the later slice
that makes the bytes' home the CAS.

## 4. What restore means after this slice

The objects live in `og_objects` (`ReplayEventProjection`), so restoring a
computer to an earlier event head rewinds the **objects**. It does **not**
rewind a file's bytes: `HydrateIfNeeded` writes only into an empty tree or a
fresh volume (`file_sync.go:182`), and nothing reduces `file_root_committed`.
After a restore, objects may name digests the tree no longer holds. The next
reconcile records the tree's truth as a forward act on the tape: the object
follows the bytes, never the reverse. Rewinding bytes (restore materializes
each object's blob from CAS by digest) is a **later slice**, and needs the
object to carry its CAS chunk list and the CAS to retain chunks by digest.

## 5. Repointing the four path-string references

Order, each its own slice:

1. **`content_item.file_path`** (slice 2): `choir.content_item` is already a
   graph kind, so adding `file_id` to its body is additive, with no DDL;
   `ImportFile` and `CreateItem` set it from the path view; reconcile
   backfills. The path stays as a display field until every reader moves.
2. **`texture_document_aliases`**: becomes the registered `document_alias`
   edge from the file object to the Texture document, then the table goes
   (with migration step 8, Texture views).
3. **Window `app_context` paths**: an additive `file_id` key in the JSON map,
   written by the frontend, resolved by id first.
4. **`media_recents.file_path`** last: a bespoke table, which migration step 7
   re-homes to `choir.media_recent`; it carries `file_id` then, so no DDL is
   spent on a table about to be retired.

**Slice 1 repoints none of them.** It ships the id they point at, the
writes that keep it true, and the read path, with the switch off.

## 6. Failure modes (written before the code)

1. **Fresh volume or pending hydration.** The tree is the image seed, or is
   empty because the restore failed or is pending; a reconcile would mark
   every object deleted, and hydration would then mint new ids. Guard:
   reconcile refuses while the fresh-volume marker exists and until boot
   hydration has returned without error (a failed hydration leaves it
   refusing until a restart). The marker lives beside the root and can vanish
   with an unmounted mount, so the next two guards do not rely on it.
2. **Mass disappearance** (an unmounted or emptied volume). Guard: a tree
   showing no file while objects exist holds every deletion, however few;
   otherwise reconcile refuses to mark more than half of the present objects
   deleted when there are more than 8. It logs, holds every deletion and move,
   and still records new and changed files.
2a. **Temporarily absent.** A file renamed into a directory the walk had
   already read, or one that came back, looks deleted. Guard: a deletion is
   recorded only for a file a fresh `lstat` finds absent at apply time in two
   consecutive passes (the candidate set lives in memory; a restart only
   delays a deletion). A move pairs on the next pass.
3. **API write and reconcile race.** The recorder lock serializes the Files
   API's record with a pass's apply phase, not its walk. Each API write
   re-reads the disk at record time; each reconcile record is checked against
   the disk again at apply time and dropped if it changed (`raced`), so the
   object is never a stale hash.
4. **Recording fails after the bytes were written** (tape append error). The
   file write stands, the response is still success, the failure is logged
   and counted; the next reconcile repairs it. The object is never ahead of
   the bytes.
5. **Out-of-band writers** (Texture import, model policy, Maildir, a shell).
   Not seen by the API hook; caught by reconcile within one interval.
6. **Large trees and files.** Hashing streams (no whole-file read); `Source/`
   is skipped; unchanged files are skipped by `(size, mod_time)`; batches
   bound each tape event.
7. **Switch flipped off then on.** Objects go stale while off. Turning it on
   is followed by a reconcile within one interval (or the POST). An id
   minted before may be marked deleted and a new one minted if the file
   changed path while off without a unique digest match.
8. **Move to an existing destination, or of a directory.** Refused (409);
   nothing moves. A destination created concurrently is refused by the
   no-replace rename, not overwritten.
9. **Path traversal and symlinked parents.** Every verb's path (and the move
   destination) goes through `resolvePath`, which refuses (403) a path whose
   parent components include a symlink: the check is no longer lexical only.
   (A symlinked directory in the tree could carry a read, a write or a move
   out of the root; the recording walk never follows one.) The check and the
   use are separate syscalls; the files root is guest-owned, so this closes
   the path an API caller can reach, not a hostile guest process.
9b. **Leaf symlinks.** A symlink as the *last* component is judged too, for
   every verb but delete: it must resolve (`EvalSymlinks`) to a place under the
   root, or the verb answers 403 (a dangling or looping link is refused too).
   In-tree links (`Source/platform/CLAUDE.md -> AGENTS.md`) keep working.
   DELETE `lstat`s and removes the link itself, never its target. The same
   hole existed on main for GET and PUT (review bunch 217).
9a. **Names the filesystem keeps literal.** The recorder keeps a path as the
   walk and the API see it: a backslash or a leading or trailing space is part
   of the name, never rewritten, so two files cannot share one object path.
10. **Replay on a rolled-back binary.** The projector writes `og_objects`
    rows without consulting the registry (`store/project.go projectObject`),
    so the old binary replays these rows as inert data, and nothing reads
    them.
11. **Writer gate in enforce mode.** The writes declare `system:runtime`,
    the kinds' writer.
12. **Missing owner.** Reconcile needs `CHOIR_OWNER_ID`; without it, it
    refuses. API writes use the authenticated owner. `GET ?id=` is scoped to
    the computer, as `?path=` and the list are.
12a. **Switch turned off mid pass.** The pass re-reads the switch before every
    batch and the store checks it on every apply; the pass stops writing.
13. **Restore divergence.** See section 4; reconcile reports the files whose
    digest it changed.

## 7. The switch and its rollback

The switch is the computer setting **`files.object`**, default off, read live
on every write and every reconcile tick (`GET`/`POST /api/computer-settings`,
an owner act; no deploy, no restart). Off: no write records an object, the
move route answers 405, reconcile does nothing. Several switched-off slices
can share one deploy, and each is turned on for b@c alone.

Rollback, in order: (1) set `files.object` to false: writes stop at once, the
objects already written stay as inert rows; (2) revert the code commit: the
kinds leave the registry, the rows replay as inert data (failure mode 10).
No schema DDL is involved, and file bytes and file CAS are untouched
throughout.

## 8. Staging proof (b@c only)

On the deploy that carries the code, with `files.object` off on every
computer:

1. Health reports the SHA. `GET /api/file-objects` on b@c returns
   `enabled:false` and nothing; a `PUT` writes no object.
2. Set `files.object=true` on b@c. `POST /api/file-objects/reconcile`:
   counts of created objects equal the regular files outside `Source/`.
3. `PUT` a file: `GET ?path=` returns an id whose `digest` is an
   `hmac-sha256:` value that is **not** the SHA-256 of the bytes the test sent;
   a second `PUT` of the same bytes at another path has the same digest.
   `PUT` new bytes at the first path: same id, new digest.
4. `PATCH move_to`: `GET ?path=<new>` returns the same id; `?path=<old>` is
   404; `GET` of the bytes at the new path returns them.
5. `DELETE`: `?id=` returns `state:deleted`; `?path=` is 404.
6. A second reconcile on the unchanged tree writes zero objects.
7. Restart b@c (planned): the objects survive replay, and the reconcile
   after boot writes zero.
8. Set `files.object=false`: a `PUT` writes nothing new.

What only staging proves: the tape append and replay of the new kinds on a
real computer, the reconcile cost over a real tree, the interaction with
hydration at a real boot, and the setting read live through the proxy.

## 9. Class and review

**Red**: the tape (new projection batches when on) and a new Files API verb.
No store schema change (no DDL; new kinds in `og_objects`). **Tier 2**
(Devin and DeepSeek v4.1 flash), since a defect on the tape is silent. It is
additive, behind a live setting, with a one-act rollback. Switched off, the
deploy writes nothing to the tape, so it may share a deploy with other
switched-off slices (owner direction via FP0, 2026-10-24); turning the
setting on is the tape change, done on b@c alone, one slice at a time, and
never in the same window as a schema change. Heresy delta:
discovered none new; introduced none; repaired none until slice 2 (the four
references still hold paths).
