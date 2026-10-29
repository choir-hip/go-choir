# Object storage: one store for every blob, public and per-owner (2026-10-30)

Status: design, not ratified; nothing is built or deployed by this note.
Mutation class of the work it proposes: **red** (tenancy and encryption of
owner content, Trace/evidence blobs, the GC that deletes them, checkpoint
closure). Review tier: 2 (Devin and DeepSeek v4.1 flash), because a defect
would silently disclose private bytes or silently delete keep-set content.
Labels: **[V]** verified by a cited command receipt or code read; **[H]**
hypothesis, to be settled by the predictions in section 8.

Owner direction (2026-10-29): "we do have to also consider the object storage
of when we're capturing these external files ... we make screenshots and
there they're all of these data blobs ... we don't yet have a S3 compatible
service running ... that too has to be multi-tenant and secure." Order of
priorities: security, stability, then scalability and simplicity. Self-hosting
is committed; a single local box must run with no hosted dependency.

Evidence, not re-derived here:
[keep-set accounting](../evidence/keep-set-accounting-2026-10-29.md),
[storage inventory and its capture-body correction](../evidence/node-b-storage-inventory-2026-10-29.md),
[paper disk and base liveness](../evidence/paper-disk-and-base-liveness-2026-10-29.md),
[research, object-store section](../evidence/data-architecture-research-2026-10-28.md),
[data rearchitecture](data-rearchitecture-2026-10-28.md),
[selection, 2026-10-29 sections](../definitions/choir-data-architecture-selection-2026-10-28.md),
[more than one home](../problems/data-has-more-than-one-home-2026-10-24.md),
[secrets vault](secrets-vault-design-2026-10-28.md).

## Decisions in one place

1. **One object store, two layers.** A Choir layer (`internal/objstore`) owns
   identity, tier, encryption, verification, access and the catalog. A
   backend stores immutable named extents (packs and large objects) and
   nothing else. Local pack backend now; S3-compatible (SeaweedFS) later.
2. **Two identity schemes.** Public objects are named by the SHA-256 of their
   plaintext and dedup across owners. Private objects are named by
   HMAC-SHA-256 of the plaintext under a per-owner ID key, encrypted with a
   random-nonce AEAD under a per-object DEK wrapped by the owner's content
   key. No convergent encryption anywhere.
3. **Refs live with the holder; the catalog lives with the store.** Holders
   (PostgreSQL corpus and control, guest SQLite via checkpoints) own refs.
   The store's own catalog records existence, location and last touch. GC is
   mark-and-sweep from refs with a touch-time grace, never reference counts.
4. **Packs, not one file per object.** Small objects append to 64 MiB
   per-owner-scope packs; plain POSIX (write, fsync, rename, pread); no
   btrfs feature is assumed.
5. **A separate host process, `blobd`,** with its own system user and data
   directory, so a corpus or control compromise is not a raw-disk read of
   every owner's ciphertext, and so the store can move to its own machine
   like corpus can.
6. **Keep set first; most of the rest is deleted, not migrated.**

## 1. Blob classes: today and target

Tiers: **public** (readable by every computer of the deployment; exposure
to the internet is a separate publication policy on the object, per the
owner's "access is a policy over objects", selection §Owner answers),
**private** (one owner; encrypted), **platform-internal** (host services
only; never served to a guest except the one computer it belongs to).

| Class | Today [V] | Tier | Canonical? | Retention | Who reads |
|---|---|---|---|---|---|
| Capture bodies (raw, blocks, text) | 154,649 plaintext files, 9.37 GB, `capture-bodies/<2hex>/<sha256>`, no GC, any bound guest reads any hash (`internal/platform/captures.go:86,214,219`) | Autopaper's captures: public. Any other owner's: private | canonical (keep set) | while a `web_capture` object refers to it; keep-set content is never swept | public: any computer; internet only the blocks a published story pins. Private: the owner's computers |
| og blobs (published Texture manifests, resolved blocks, transclusions) | 21,848 referenced `sha256/og/<hex>.bin`, 0.47 GB; written without fsync (`internal/platform/objectgraph_store.go:44-74`) | public when the holding og object is published; private otherwise | canonical (Autopaper object graph) | refs from `og_objects.body_ref` | public: anyone the publication policy allows |
| Image blobs and self-development screenshots | guest `state.image-blobs/sha256/<hex>`, plaintext, not in file CAS, pins without a sweep (`internal/imageblob/imageblob.go:20-60`, `pin.go:13-19`; problem doc violation 11) | private (owner) | canonical when pinned by a Texture revision or evidence object; otherwise scratch | pinned: as long as the holder; unpinned: 14 days | the owner's computers. Publishing a revision that embeds one copies it into the public tier as an explicit act |
| File CAS chunks | 27.9 GB in 197 per-computer dirs; deterministic synthetic nonce under a per-computer DEK (`internal/filecas/filecas.go:143-160`); the seeded `files/Source` probably copied into each [H, inventory §1] | owner files: private. Shared open-source and seeded system files: public | canonical (owner files) | reachable from the computer's retained file roots | the owner's computers; public seed by all |
| Tape segments and payloads | `tape-seg/` 27.2 GB (26.4 the paper), 64 MiB seal (`internal/tapestore/format.go:106`); legacy one-file-per-pin 47.4 GB, 7.4 M inodes (`event_artifacts.go:118-125`) | platform-internal, per computer; private payloads stay envelope-encrypted inside | canonical for user computers and self-development; not for the Autopaper (owner, 21:10) | semantic events forever; telemetry class by rule (tape schema v2 payload class) | the host restore path and that computer |
| Projection bases | 61.6 GB, 425 files, a plaintext copy of the guest store (no AEAD in `internal/projectionbase`, non-test files [V grep]); liveness: watermark, pins, latest two, queued jobs (`projection_jobs.go:570`) | platform-internal, encrypted under the computer DEK | derived (accelerator) | latest one per computer plus pinned restore points; superseded after 24 h | the host restore path only |
| Mail attachments | outbound host files `mail/attachments/outbound/<owner>`, 1.2 MB; inbound bodies inline in the per-owner `mail.db` (keep-set §4) | private (owner) | canonical (keep set) | while the message exists | the owner's computers and maild |
| Capsule artifacts | guest `/mnt/persistent/capsule-artifacts`, 0.18 GiB, 4,848 files on the paper (paper-disk §A1); `nix/autoputer-vm.nix:621,1078` | private (owner) | evidence when cited, else scratch | cited: as the citing evidence object; else 14 days | the owner's computers |
| Platform update payloads | `sha256/platform-update` 1.4 GB, GC working | public (release artifacts) | derived from the build | release keep-count | hosts and computers |

Two classes change tier on purpose. Captures were "shared content"
(`captures.go:213`); the owner ruled owners' captures private (selection
§Owner answers), so only the Autopaper's captures stay public. Projection
bases were treated as derived bytes; they are a full copy of an owner's
store, so they are private material and become encrypted.

## 2. The interface

The Choir layer, used by blobd's handlers, by guests through blobd, and
in-process by tests and the desktop single box:

```go
type Ref struct {
    Tier  Tier   // Public | Private | Internal
    Scope string // "" for public; owner ID for private; computer ID for internal
    ID    string // sha256:<hex> (public) or hmac:<hex> (private/internal)
}

type Store interface {
    Put(ctx, PutRequest, io.Reader) (Ref, PutResult, error) // streaming; IfAbsent by default
    Get(ctx, Caller, Ref, Range) (io.ReadCloser, Info, error) // verifies before or while returning bytes
    Stat(ctx, Caller, Ref) (Info, error)
    Touch(ctx, Ref) error            // a dedup hit or a re-commit refreshes last touch
    Condemn(ctx, []Ref, sweepStart time.Time) ([]Ref, error) // GC phase 1
    Purge(ctx, []Ref) error          // GC phase 2, physical
    ListByRef(ctx, Holder) ([]Ref, error) // answered by the holder's metadata store, not the backend
}
```

`PutRequest` carries tier, scope, class (capture, og, image, file-chunk,
tape-segment, base, mail, capsule), media type, declared size and, for
private puts, the caller's ID key handle. **Conditional put is a catalog
operation**: `IfAbsent` returns the existing ref and touches it; the backend
never needs `If-None-Match` because extent names are unique (a ULID per
pack) and the catalog decides identity. `Range` reads exist for media and
large bases.

**In the interface:** identity, tiers, the encryption format, verification,
caller authorization, the catalog and the GC protocol. **In the backend
(`Backend`):** `PutExtent(name, reader)`, `GetRange(name, off, len)`,
`StatExtent`, `DeleteExtent`, `ListExtents(prefix)`. A backend knows nothing
about owners, keys or refs; swapping local for S3 changes only where extents
live.

**Object format.** Header (version, tier, class, media type, plaintext
length, key version, wrapped DEK for private objects, frame size, frame
digests for multi-frame public objects), then frames of 1 MiB. Private
frames are XChaCha20-Poly1305 with a STREAM-style nonce (random prefix,
counter, last-frame flag) and AAD binding `tier || scope || id || class ||
frame index`, with the HKDF `info` strings `choir/objstore/v1/<purpose>`
(vault design §2.3 convention). A range read decrypts or hashes only the
frames it touches. Every stored object also has a **cipher digest**,
SHA-256 of its stored bytes, recorded in the catalog and the pack index.

**Identity rules.**

- Public: `sha256:<hex of plaintext>`, the same IDs the 154,613 capture refs
  and 21,848 og refs already carry, so their metadata is not rewritten.
  SHA-256 stays rather than BLAKE3 (the research memo's choice): every
  existing ref, the tape and file CAS are SHA-256, hardware SHA extensions
  make it fast enough for 1 GB/day, and one hash is simpler than two.
- Private: `hmac:<HMAC-SHA-256(owner ID key, plaintext)>`. The ID key is
  derived from the owner content key (vault design §2, "per-owner CAS object
  keys"). Equal bytes from two owners get unrelated IDs, so a hash learned
  from one owner neither fetches nor confirms another's object. Within one
  owner, the host learns that two objects are equal; this is the cost file
  CAS already accepts (`filecas.go:154-157`) and buys within-owner dedup.
- Encryption is never derived from content: a per-object DEK is random, and
  dedup comes from the ID lookup before the upload, not from deterministic
  ciphertext. New file CAS chunks move to this scheme; old chunks still open
  (the nonce is in their prefix).
- Platform-internal: `hmac:` under the computer's key, scope = computer ID.

## 3. Tenancy and security

**Who encrypts.** The party that holds the plaintext and a key lease:
guests for guest-originated private bytes (images, screenshots, files,
capsule artifacts, the computer's own bases and tape payloads, as file CAS
does today); maild and corpusd for host-originated private bytes (mail
attachments, a non-Autopaper owner's captures) under a vault lease. blobd
stores ciphertext and never needs a private key to store, verify (by cipher
digest), condemn or purge.

**Key source.** The vault owns every key (vault design §2): owner ROOT, owner
KEK, and under it the per-owner content key and ID key; the computer DEK
for platform-internal objects. Until the vault's owner-KEK slice lands,
private objects are written with their per-object DEK wrapped under an
**interim deployment wrap** and a `key_version` naming it. Per-object DEKs
make the later move a rewrap of 72-byte headers, not a re-encryption of
data (open question 3).

**Crypto-erase.** Destroying an owner's content-key versions makes every
private object of that owner unreadable on every host and in every backup
copy (vault design §2.8; backup erase follows the vault's deletion
journal). Packs are per owner scope, so a deleted owner's packs are also
removed physically in one pass, without compacting other owners' packs.

**Least-privilege read.** A guest reaches blobd only through the existing
bound-guest identity (`bindTapCaller`, `objectgraph_handlers.go:379-390`).
blobd allows a read when the ref is public, or the ref's scope equals the
caller's owner (private), or the caller's computer (internal). There is no
"any bound guest" read of a private or internal object. Browsers and media
players (Autoradio streaming) get **capabilities**: a host-signed token over
`(ref, owner, range, expiry ≤ 10 min)` issued by the owner's authenticated
session; S3 presigned URLs never leave the host, because private bytes are
ciphertext and the backend has no notion of the caller.

**Closing today's cross-owner capture read.** `HandleCaptureBody`
(`captures.go:214-240`) passes `allowAnyGuest=true` and serves any body by
hash, plaintext, to any computer that learns the hash, and its 404/200
confirms presence (inventory, final correction). After migration: a body is
fetched by capture object ID, the host resolves the object's ref and checks
tier and owner; a raw-hash read exists only for the public tier; a private
ID is an HMAC that no other owner can compute. `HandleCaptureRead`
(`captures.go:244-249`) gets the same rule. No evidence yet that a private
body is in the store [V inventory]; the migration classifies by owner, not
by content, so a private body that is there is covered either way.

**Zero trust between hosts.** Every reader verifies: a full `Get` checks
the cipher digest before returning the last byte (and the plaintext hash for
public objects); a range read checks frame tags or frame digests; a mismatch
is an error, never a fallback to another copy without verification (the
`tapestore` rule, `format.go:13-17`). Replicas, the S3 backend and a desktop
cache are untrusted storage.

## 4. References and GC

**Refs.** Each holder records its refs in its own store, in the same
transaction as the holder: `og_objects.body_ref` and capture metadata in
the corpus PostgreSQL; file roots, tape segments, bases and their pins in
the control PostgreSQL; guest refs (image pins, capsule evidence, file
manifests) in guest SQLite, exported to the host as the **CAS closure** of
each semantic checkpoint (tape tail + CAS closure + authority snapshot,
data rearchitecture §What it buys). The host never reads a guest store to
find refs; it reads the closures of retained checkpoints.

**Catalog.** blobd keeps a SQLite (WAL, `synchronous=FULL`) catalog: ref,
cipher digest, size, class, extent and offset, `last_touch`, state (`live`,
`condemned`). It is rebuildable from packs, which are self-describing; a
rebuild sets `last_touch = now`, which only delays GC. SQLite rather than a
PostgreSQL schema keeps blobd one process on the host, the desktop and in
the guest cache; when a second host shares one store, the catalog moves
behind the same interface to the corpus PostgreSQL instance.

**Mark and sweep, not counts.** Reference counts drift when a crash lands
between the holder's commit and the store's increment, and there is no
atomic transaction across PostgreSQL, guest SQLite and blobd. Mark and sweep
is idempotent and recomputes truth each run. Protocol:

1. A holder commits a ref only within the **ref window** (1 h) of its `Put`
   or `Touch` of that ref; a slower holder touches again.
2. A sweep records its start `S`, then streams live refs from every root
   provider (corpus, control, retained checkpoint closures), each sorted by
   ref, and merge-joins them with the catalog sorted by ref. Memory is the
   merge cursors plus one batch, not the live set.
3. Phase 1 condemns refs absent from every stream with `last_touch < S -
   grace` (grace 24 h > ref window). A `Touch` of a condemned ref fails, so
   the putter uploads again under a new extent.
4. Phase 2 purges condemned refs after a further hour, logs each purge (the
   existing deletion-intent JSONL, `artifact_gc.go:743-834`), and marks pack
   space dead for compaction.

A root provider that fails aborts the sweep (today's "live set is empty"
guard, `artifact_gc.go:270`, generalized). Keep-set classes (captures, og
bodies, mail) are never condemned while their holder rows exist, and a sweep
whose condemn set exceeds 1% of a keep-set class aborts for review.

**What it replaces.** The per-namespace walker over `sha256/<ns>/` with mtime
as liveness and `casReuseMu` with `Chtimes` to protect reuse
(`objectgraph_store.go:56-63`, `artifact_gc.go:106-125,381-430`), the
hardcoded "latest two" base rule (`projection_jobs.go:570`), the unswept
image pins (`imageblob/pin.go:13-19`) and the absent capture GC. mtime is
not a liveness signal on packs or S3.

## 5. Physical layout

**Packs.** Objects under 16 MiB append to the open pack of their scope (one
open pack per public class, per owner, per computer-internal scope). A pack
seals at 64 MiB or after 1 h idle, matching the tape's 64 MiB seal
(`tapestore/format.go:106`). Objects of 16 MiB or more (bases, large media)
are one extent each, framed the same way. The 154,649 capture-body files
and 21,862 og files become about 160 packs [H, R0g-02]; the 7.4 M-inode
legacy pin tree is the lesson that one file per object costs inodes,
directory scans and backup time far beyond its bytes (inventory §1).

**Write discipline.** Records append to the open pack with a header CRC and
the cipher digest. Puts are group-committed: up to 5 ms or 8 MiB, then one
`fdatasync` of the pack, then one catalog transaction, then the
acknowledgements. Sealing fsyncs the pack, renames `*.open` to `*.pack`,
fsyncs the directory, and writes the pack index (sorted refs, offsets). On
open, blobd scans the open pack past the last catalogued record: records
whose CRC and digest verify are adopted; a torn tail is truncated.

**Compaction.** A sealed pack whose dead fraction exceeds 40% is rewritten:
live records copied to a new pack, catalog switched in one transaction, the
old pack deleted after a 10-minute reader grace. Write amplification is at
most 1/(1 - 0.4) = 1.7x of what is deleted, and keep-set packs (append-only
content) almost never qualify. Compaction needs free space of one pack, not
of the store (contrast the Dolt compaction that grew 64.7 GB to 89.2 GB,
paper-disk §A1).

**btrfs assumptions dropped.** The store uses write, fsync, rename, pread
and unlink on any POSIX filesystem (ext4, APFS, NTFS through the desktop
app, a bucket). It uses no reflink, `FICLONE`, subvolume snapshot,
sparse-file or mtime semantics. Reflink stays where it is today, a
compute-host-local optimization of vmctl (data image capture,
`projectionbase/capture.go:22`, store compaction), and is never a property a
blob relies on.

## 6. Backend choice

**Now: the local pack backend**, on Node B and on a self-hosted single box,
and as the guest's local cache and the desktop's store. **Later: an
S3-compatible backend, SeaweedFS by default** (Apache-2.0, single-node mode,
small per-object index; 1 GB volumes because vacuum needs a volume's worth
of free space; we run repair ourselves). The research is one day old; MinIO
(archived), Garage (metadata limits, no conditional writes) and Ceph (size)
stay rejected without a re-check. Packs map to S3 objects one to one, so the
bucket holds thousands of objects, not millions, and range GETs serve
records.

What S3-compatible buys: off-host and multi-host durability, extents
readable from a second host or a desktop, and erasure coding or replication
we do not write. What it does not buy: encryption, identity, refs, tenancy,
verification and GC, which stay in the Choir layer. Off-host copies before
SeaweedFS: sealed packs are immutable, so the nightly restic run uploads
only new packs (open question 4).

## 7. Migration, keep set first

Downtime is acceptable now (owner, 2026-10-29); each step still swaps only
after a full referenced-set comparison, and the old directory is kept until
that comparison passes.

1. **Build blobd and `internal/objstore` with the local backend**, shipped
   on the proof ring first (d@e), then Node B's host. Red, tier 2.
2. **Capture bodies.** Import the 154,613 referenced bodies (keep-set §1):
   the Autopaper user's 150,811 into the public tier under unchanged
   `sha256:` IDs (no metadata rewrite); the other owners' ~3.8 K into the
   private tier under the interim wrap, rewriting those captures' three blob
   fields to `hmac:` refs. The 604 bodies both share are stored once public
   and once per other owner. Verify every referenced ref resolves and
   hashes; switch `PutBody`/`Body` and the two read handlers; drop the 36
   orphans.
3. **og blobs.** Import the 21,848 referenced bodies by the tier of their
   holding object; `body_ref` resolves through the store; delete the 14
   orphans and the 12,534 unreferenced `sha256/*.txt` (outside the keep
   set, keep-set §2).
4. **Mail.** Outbound attachments (10 files) into the private tier; inbound
   bodies stay in `mail.db` until mail moves into the computer.
5. **Guest Textures.** The paper's and the owner's Texture documents and
   revisions are rows, not blobs: they move with the guest engine
   (SQLite) migration, read offline from a reflink copy (keep-set §8.1).
   Image blobs their revisions pin go to the owner's private tier.
6. **New writes only, from then on:** guest image blobs and screenshots,
   capsule evidence, file CAS under keyed IDs, the public seed tier for
   shared system files (written once), bases encrypted under the computer
   DEK, tape segments as internal extents.

**Deleted, not migrated** (outside the keep set, pre-launch): legacy pin
files 47.4 GB and 7.4 M inodes, the paper's 26.4 GB of segments, the 61.6
GB of projection bases (derived from tapes being dropped), per-computer
file CAS 27.9 GB, capsule artifacts and `computer-inputs` once vmctl no
longer reads it. Each deletion waits for the step that replaces its reader
and for open question 1.

## 8. Precommitted expectations (proposed round 0g; to be committed by the orchestrator)

Proposed for `docs/definitions/choir-data-architecture-selection-2026-10-28.precommit.md`
under its rules; not committed by this note, so not yet records. Settler:
the rule, reading the named artifact. They extend R0f-05 (blobs 12 GB [9.5,
30]) and R0f-06 (blobs 0.65 GB/day).

**R0g-01.** Stored bytes of steps 2-4 in the local pack store (headers,
frames, pack indexes included): est 9.95 GB [9.75, 10.6]. Assumes ≤ 2%
format overhead on 9.85 GB of referenced bodies plus ~0.25 GB for the
private copies. Falsifier: over 11 GB. Artifact: `du` of the store plus the
catalog's byte sum.

**R0g-02.** Files in the store after steps 2-3: est 165 [150, 260] packs and
indexes, against 176,511 files today. Falsifier: over 1,000.

**R0g-03.** Wall time of the step 2-3 import on Node B under `nice`/`ionice`,
one writer, every body hashed on read and on write: est 15 min [5, 45].
Hard to measure: page-cache state; mitigation: drop caches before the run
and record it.

**R0g-04.** blobd on Node B, 60 KB capture-body reads by a bound guest,
24 h of production traffic: p99 4 ms [1.5, 15]; group-committed puts p99
12 ms [4, 40]. Falsifier: read p99 over 25 ms or put p99 over 80 ms.

**R0g-05.** Peak RSS of one full mark-and-sweep over 1 M catalog entries
and three root providers: est 60 MiB [25, 150]. Falsifier: over 256 MiB,
which would mean the merge is not streaming.

**R0g-06.** Object-store growth, 30-day mean after cutover at the
then-current Autopaper rate: est 0.6 GB/day [0.35, 1.1], 95% of it public
captures. Falsifier: over 1.5 GB/day. Consistent with R0f-06.

**R0g-07.** A red-team pass with a second owner's bound computer, given
every hash in the capture and og tables, reads zero private or internal
bytes and gets identical responses for absent and present private refs:
p = 0.90. Falsifier: any byte read or any distinguishable response.

## 9. Open questions for the owner

1. **Discard all pre-launch tapes, legacy pins, bases and file CAS,
   including your own computer's, after exporting your Textures, files and
   mail?** Recommended: yes. Your computer's file CAS is 143 MB and is
   exported as files; the tape restarts on the new engines.
2. **Keep the other 29 owners' 1,840 captures, as private objects?**
   Recommended: yes; they are "captured" (keep-set §8) and cost under 0.4 GB.
3. **Accept an interim deployment wrap for private objects until the
   vault's owner-KEK slice lands?** Recommended: yes; per-object DEKs make
   the later move a header rewrap, and blocking on the vault would hold the
   capture tenancy fix.
4. **Off-host copies of the store before SeaweedFS: restic of sealed packs
   nightly?** Recommended: yes; immutable packs upload once, unlike the
   52.7 GB-a-night Dolt copy (inventory §2). It is asynchronous and can lose
   the last day on host loss, a stated weaker guarantee until a second host.
5. **May a user's credential-free fetch reference the public object with
   the same hash instead of storing a private copy?** Recommended: not at
   launch; store private and measure the overlap (604 shared bodies today),
   because a misclassified logged-in fetch would publish private bytes.

## Ceremony

- Conjecture delta: blob bytes leave per-namespace directories and become
  one store with two identity schemes; the per-namespace GC, the
  any-guest capture read and plaintext bases are retired.
- Protected surfaces: owner-content tenancy, Trace/evidence blobs, the GC
  that deletes them, checkpoint closure, the capture and og read paths.
- Admissible evidence: full referenced-set comparisons (not samples) before
  each swap; the round 0g artifacts; a staging red-team receipt for R0g-07.
- Rollback: every swap keeps the old directory and the old resolver until
  the comparison and a week of reads pass; the resolver falls back by
  config, never silently. Purges are irreversible and wait for that week.
- Heresy delta: `discovered`: projection bases are plaintext copies of owner
  stores on the host; og bodies are installed without fsync; capture bodies
  have no GC namespace and an existence oracle by hash. `introduced`: a new
  host service and a catalog to keep consistent with packs. `repaired`:
  none until built.
