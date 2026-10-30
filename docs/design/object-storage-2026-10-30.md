# Object storage: one store for every blob, by tier and owner (2026-10-30)

Status: design, revision 2 (after the tier-2 review, `.agentic-consensus/object-storage-r1/`;
both seats returned send_back). Not ratified; nothing is built or deployed by
this note. Revision 1 is commit 3294bce10; the revision 2 log at the end
maps every finding to where it is addressed.
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

Owner decisions on revision 1 (2026-10-29): Q1-Q4 accepted, Q1 conditional
on export receipts and a restore drill of the keep set, Q3 with a per-owner
interim key-encryption key (section 9).

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
2. **Access is a policy over objects** (owner, selection §Owner answers).
   Four tiers: public, platform, private, internal. Everything is private
   except what is public; the Autopaper's captures are **platform**, not
   public (stated default, section 1).
3. **Two identity schemes.** Public and platform objects are named by the
   SHA-256 of their plaintext. Private and internal objects are named by
   HMAC-SHA-256 of the plaintext under a per-owner (or per-computer) ID key.
   Every non-public object is encrypted with a random-nonce AEAD under a
   random per-object DEK. No convergent encryption anywhere.
4. **Refs live with the holder; the catalog lives with the store.** Holders
   own refs. The store's catalog records existence, location and last
   touch. GC is mark-and-sweep from a closed list of root providers, with a
   touch on every ref commit and a recheck before every purge; never
   reference counts.
5. **Packs, not one file per object.** Small objects append to 64 MiB
   per-scope packs; write, fsync, rename, pread; no btrfs feature is assumed.
6. **A separate host process, `blobd`,** with its own system user and data
   directory, holding no owner keys and no database credentials. The GC
   driver that reads holders is a separate process.
7. **Keep set first; the rest is deleted, not migrated, and only after
   export receipts and a restore drill of the keep set.**

## 1. Blob classes: today and target

Tiers:

- **public**: readable by every computer of the deployment; exposure to the
  internet is a separate publication policy on the holding object. Plaintext
  at rest.
- **platform**: held by the Autopaper (the platform owner) and not
  published. Readable by host services and the Autopaper's own computers;
  never by another owner's computer, never served raw to the internet.
  Encrypted at rest under a platform key.
- **private**: one owner; encrypted under that owner's keys.
- **internal**: one computer; host restore path and that computer only;
  encrypted under the computer DEK.

**Stated default, owner may override: the Autopaper's captures are platform,
not public.** The owner ruled "everything is private except what is public
(the Autopaper's articles and what users explicitly publish) ... drafts and
owners' captures are private" (selection §Owner answers). The paper's raw
captured bodies are third-party pages, often under copyright; public readers
get the articles and what the citation and excerpt policy allows (the
resolved blocks a published version pins, which are og bodies and public),
not raw full bodies. Making captures platform rather than public also
removes the reading-list oracle (any computer probing whether the paper
captured given bytes) and keeps the 604 bodies shared with other owners
from becoming public copies of their captures.

| Class | Today [V] | Tier | Canonical? | Retention | Who reads |
|---|---|---|---|---|---|
| Capture bodies (raw, blocks, text) | 154,649 plaintext files, 9.37 GB, `capture-bodies/<2hex>/<sha256>`, no GC, any bound guest reads any hash (`internal/platform/captures.go:86,214,228`) | Autopaper's: platform. Any other owner's: private | canonical (keep set) | while a `web_capture` object refers to it; never condemned while referenced (section 4) | platform: host services, the Autopaper's computers. Private: the owner's computers |
| og blobs (published Texture manifests, resolved blocks, transclusions) | 21,848 referenced `sha256/og/<hex>.bin`, 0.47 GB; written without fsync (`internal/platform/objectgraph_store.go:44-74`); GC active on Node B (`nix/node-b.nix:1307`) | follows the holding object: public when published; otherwise the holder's tier (platform for the Autopaper, private for others) | canonical (Autopaper object graph) | refs from `og_objects.body_ref` | per the holding object's publication policy |
| Image blobs and self-development screenshots | guest `state.image-blobs/sha256/<hex>`, plaintext, not in file CAS; pins are guest files with no unpin and no sweep (`internal/imageblob/imageblob.go:20-60`, `pin.go:13-20`; problem doc violation 11) | private (owner) | canonical when held by a live Texture revision or evidence object; otherwise scratch | held: as long as a live holder; unheld: 14 days | the owner's computers. Publishing a revision that embeds one copies it into the public tier as an explicit act |
| File CAS chunks | 27.9 GB in 197 per-computer dirs; deterministic synthetic nonce under a per-computer DEK (`internal/filecas/filecas.go:143-160`); the seeded `files/Source` probably copied into each [H, inventory §1] | owner files: private. Shared open-source and seeded system files: public | canonical (owner files) | reachable from the computer's retained file roots | the owner's computers; public seed by all |
| Tape segments and payloads | `tape-seg/` 27.2 GB (26.4 the paper), 64 MiB seal (`internal/tapestore/format.go:106`); legacy one-file-per-pin 47.4 GB, 7.4 M inodes (`event_artifacts.go:118-125`); no segment ref table today (`artifact_gc.go:106-110` excludes the tape) | internal, per computer; private payloads stay envelope-encrypted inside | canonical for user computers and self-development; not for the Autopaper (owner, 21:10) | the retained receipt chain (chain retention, not a sweep) | the host restore path and that computer |
| Projection bases | 61.6 GB, 425 files, a plaintext copy of the guest store (no AEAD in `internal/projectionbase`, non-test files [V grep]); liveness: watermark, pins, latest two, queued jobs (`projection_jobs.go:570`) | internal, encrypted under the computer DEK | derived (accelerator) | latest two per computer plus pinned restore points, until the new restore path is proven; then revisit | the host restore path only |
| Mail attachments | outbound host files `mail/attachments/outbound/<owner>`, 1.2 MB; metadata in the per-owner `email_attachments` table; inbound bodies inline in the per-owner `mail.db` (keep-set §4) | private (owner) | canonical (keep set) | while the message exists | the owner's computers and maild |
| Capsule artifacts | guest `/mnt/persistent/capsule-artifacts`, 0.18 GiB, 4,848 files on the paper (paper-disk §A1); `nix/autoputer-vm.nix:621,1078` | private (owner) | evidence when cited, else scratch | cited: as the citing evidence object; else 14 days | the owner's computers |
| Platform update payloads | `sha256/platform-update` 1.4 GB, GC working; live set from the route tables (`artifact_gc.go:432-440,475-540`) | public (release artifacts) | derived from the build | the route tables' current and rollback refs | hosts and computers |

Three classes change tier on purpose. Captures were "shared content"
(`captures.go:213`); the owner ruled owners' captures private, and the
paper's captures move to the platform tier (above). Projection bases were
treated as derived bytes; they are a full copy of an owner's store, so they
become internal and encrypted.

Not covered by the store until mail moves into the computer: inbound mail
bodies stay plaintext inline in the host's per-owner `mail.db`. This is the
most private keep-set content and the tenancy story does not cover it yet.

## 2. The interface

The Choir layer, used by blobd's handlers, by guests through blobd, and
in-process by tests and the desktop single box:

```go
type Ref struct {
    Tier  Tier   // Public | Platform | Private | Internal
    Scope string // "" for public and platform; owner ID (private); computer ID (internal)
    ID    string // sha256:<hex> (public, platform) or hmac:<hex> (private, internal)
}

type Store interface {
    Put(ctx, Caller, PutRequest, io.Reader) (Ref, PutResult, error) // streaming; IfAbsent by default
    Get(ctx, Caller, Ref, Range) (io.ReadCloser, Info, error)        // verifies before or while returning bytes
    Stat(ctx, Caller, Ref) (Info, error)
    Touch(ctx, Caller, []Ref) ([]TouchResult, error)  // at every ref commit; un-condemns
    Release(ctx, Caller, []Ref) ([]Ref, error)        // holder's delete of its last ref; starts the hold
    Condemn(ctx, GCRun, []Ref) ([]Ref, error)         // GC phase 1
    Purge(ctx, GCRun, []Ref) ([]Ref, error)           // GC phase 2: rechecks, then marks pack space dead
}
```

`PutRequest` carries tier, scope, class (capture, og, image, file-chunk,
tape-segment, base, mail, capsule), media type, declared size and, for
private and internal puts, the **caller-computed ID**: blobd never holds an
ID key, so it cannot compute or confirm an HMAC (and is no confirmation
oracle). A reader that holds the key recomputes the HMAC after decrypting
and rejects a mismatch. The writer computes the ID while streaming the
plaintext to a local spool, then asks the catalog; dedup is decided before
the upload at the cost of one spool write for large objects.

**Conditional put is a catalog operation**: `IfAbsent` on a live ref
returns it and touches it; on a condemned or released ref it returns it to `live` in
the same catalog transaction and discards the upload; on an absent
or purged ref it writes a new record. Extent names are unique (a ULID per
pack) and the catalog decides identity, so the backend never needs
`If-None-Match`. Enumerating a holder's refs is the GC driver's job against
the holder's store (section 4), not a store operation.

**In the interface:** identity, tiers, the encryption format, verification,
caller authorization, the catalog and the GC protocol. **In the backend
(`Backend`):** `PutExtent(name, reader)` (a whole extent),
`GetRange(name, off, len)`, `StatExtent`, `DeleteExtent`,
`ListExtents(prefix)`. A backend knows nothing about owners, keys or refs.
Open packs are host-local; a remote backend receives a pack only when it
seals. One writer per scope at a time (a lease), so two hosts never append
to the same pack.

**Canonical ref grammar.** Inside the store and in every root-provider
stream a ref is `(tier, scope, sha256:<hex> | hmac:<hex>)`. Holders keep
their legacy encodings; one function, `objstore.CanonicalRef(class, s)`,
maps them, and both the root providers and the resolvers use it:
`sha256:<hex>` (capture metadata), `sha256/og/<hex>.bin` (`og_objects.body_ref`,
`objectgraph_store.go:49`), `artifact:sha256:<hex>` (image refs,
`imageblob.go:91`). So the 21,848 og rows are not rewritten. A string that
does not parse is a provider error, which aborts the sweep (section 4).

**Object format.** Header (version, tier, class, media type, plaintext
length, frame count, key version, wrapped DEK for encrypted objects, frame
size), then frames of 1 MiB. Encrypted frames are XChaCha20-Poly1305 with a
STREAM-style nonce (random prefix, counter, last-frame flag) and AAD
binding `tier || scope || id || class || plaintext length || frame count ||
frame index`, so a truncated or shortened object fails authentication.
HKDF `info` strings are `choir/objstore/v1/<purpose>` (vault design §2.3
convention). Public objects carry per-frame digests
`SHA-256(id || frame index || plaintext length || frame bytes)`, and the
catalog records their Merkle root; a range read checks its frames against
that root, never against the header alone. Every stored object also has a
**cipher digest**, SHA-256 of its stored bytes, in the catalog and the pack
index. A reader rejects a short read and a header whose length or frame
count differs from the catalog's.

**Identity rules.**

- Public and platform: `sha256:<hex of plaintext>`, the IDs the 154,613
  capture refs already carry (the og refs carry the same hex in path form,
  mapped by `CanonicalRef`). The paper's 63,837 capture rows are not
  rewritten. A platform ID is a hash of a public-web fetch; it discloses
  nothing a fetcher could not compute, and the access check runs before the
  lookup, so a caller without access gets the response it would get for an
  absent ref. SHA-256 stays rather than BLAKE3: every existing ref, the tape
  and file CAS are SHA-256, and one hash is simpler than two.
- Private: `hmac:<HMAC-SHA-256(owner ID key, plaintext)>`. Equal bytes from
  two owners get unrelated IDs. Within one owner, the host learns that two
  objects are equal; this is the cost file CAS already accepts
  (`filecas.go:154-157`) and buys within-owner dedup.
- Internal: `hmac:` under a key derived from the computer DEK, scope =
  computer ID.
- Encryption is never derived from content: a per-object DEK is random, and
  dedup comes from the ID lookup. New file CAS chunks move to this scheme;
  old chunks still open (the nonce is in their prefix).

## 3. Tenancy and security

**Keys (Q3, owner-accepted with a per-owner interim KEK).** Until the
vault's owner-KEK slice lands, each owner gets a random **interim owner
KEK**, generated at first use, wrapped under the host-sealed deployment key
(the vault's deployment-KEK v1) and escrowed like the existing key escrows.
Under it sit two keys: the owner **content key**, which wraps every
per-object DEK, and the owner **ID key**, which names private objects. The
ID key is stable: it is never rotated with the content key, because
rotating it would rename every private object (an explicit, separate
migration if ever needed). Content-key rotation rewraps DEKs. The move to
the vault's owner KEK rewraps the owner's content and ID keys once; object
headers and packs do not change, so sealed packs stay immutable. Content-key
rotation is different: it rewrites each wrapped-DEK header, so it rewrites
the owner's packs and is a compaction-class operation, not a free one. The
platform tier has the same structure under a platform KEK held by the
vault's deployment path. Internal objects use the computer DEK.

**Who encrypts.** The party that holds the plaintext and a key lease:

- guests for guest-originated private bytes (images, screenshots, files,
  capsule artifacts, tape payloads). A guest receives its owner's content
  key and ID key at boot, wrapped under its computer DEK. No such channel
  exists: the computer DEK is generated inside the guest
  (`computerevent/privacy.go:100-108`) and only exported upward for escrow
  (`:84-96`). Delivery is new work: the host unwraps the escrowed DEK to
  wrap the owner keys for that guest, or the guest pulls them over an
  authenticated call. File CAS uses the computer DEK only
  (`filecas.go:143-160`);
- host replay workers for projection bases (built host-side), under a
  computer-DEK lease;
- corpusd for the platform tier (the paper's captures) and for other
  owners' captures, and maild for mail attachments, under a vault lease for
  that owner. corpusd sees a private capture's plaintext on its request
  path; the tenancy gain is at rest and across tenants, not confidentiality
  from corpusd.

blobd stores ciphertext and never needs a key to store, verify (by cipher
digest), condemn or purge.

**Crypto-erase.** Destroying an owner's interim KEK and its escrow copies
makes that owner's private objects unreadable on every host, and only that
owner's. Backup copies of the wrapped KEK and escrow rows are erased by the
vault's deletion journal (vault design §2.8); until that journal has run
against a backup set, that backup can still open the owner's objects. Packs
are per owner scope, so a deleted owner's packs are also removed physically
in one pass, without compacting other owners' packs.

**Least-privilege read.** A guest reaches blobd only through the existing
bound-guest identity (`bindTapCaller`, `objectgraph_handlers.go:379-390`).
blobd allows a read when the ref is public; platform when the caller is a
host service or one of the Autopaper's computers; private when the ref's
scope equals the caller's owner; internal when it equals the caller's
computer. Absent and forbidden give the same status, body and timing class.
Browsers and media players get **capabilities**: a host-signed token over
`(ref, owner, range, expiry)` issued by the owner's authenticated session,
renewable, with expiry at most 10 min per token. Ciphertext is useless to a
browser, so private bytes reach the owner's browser through the owner's
computer, which holds the keys and already serves the owner's UI; a long
stream renews its token and re-resolves on a moved extent. S3 presigned
URLs never leave the host.

**Closing today's cross-owner capture read.** `HandleCaptureBody`
(`captures.go:214-240`) passes `allowAnyGuest=true` and serves any body by
hash, plaintext, to any computer that learns the hash, and its 404/200
confirms presence. After migration a body is fetched by capture object ID:
the host resolves the object, checks the caller against the object's
owner, then reads the ref. A raw-hash read exists only for the public tier.
Every in-process body reader moves to that path:
`captures.go:228,279,303`, `publication_versions.go:465` (`captureBlockDoc`,
which today trims `sha256:` with no caller), `researchtools/texture_captures.go:90`
and `capture_index.go:186`.

**Zero trust between hosts.** Every reader verifies: a full `Get` checks
the cipher digest before returning the last byte (and the plaintext hash for
public and platform objects, or the HMAC for private ones when the reader
holds the key); a range read checks frame tags or frame digests against the
catalog's root; a mismatch is an error, never a fallback to another copy
without verification (the `tapestore` rule, `format.go:13-17`). Replicas,
the S3 backend and a desktop cache are untrusted storage.

## 4. References and GC

**Refs.** Each holder records its refs in its own store, in the same
transaction as the holder. Guest refs reach the host as the **CAS closure**
of each semantic checkpoint (data rearchitecture §What it buys). Invariant:
every restorable checkpoint is retained, and its closure is a root.

**Root providers, closed list.** Each provider streams canonical refs
sorted by ref and declares an expected count per (class, scope).

| Provider | Source | Classes |
|---|---|---|
| corpus, og | `og_objects.body_ref` (through `CanonicalRef`) | og |
| corpus, captures | the three blob fields of `choir.web_capture` metadata JSON (not `body_ref`, which is empty for captures; reusing today's og query at `artifact_gc.go:447` would see none of them) | capture |
| maild | per-owner `email_attachments` and `email_drafts.attachments_json` (`maild/attachments.go:24-56,215-275`) | mail |
| control, releases | the route tables' current and rollback refs (`platformUpdateLiveDigests`, `artifact_gc.go:475`) | platform-update |
| control, computers | file roots; bases and their pins; tape segments by the retained receipt chain, with a segment ref table added before the tape moves into the store | file-chunk, base, tape-segment |
| guest closures | per retained checkpoint: image refs held by live Texture revisions and evidence objects (enumerated from the revision tables, not copied from pin files, which have no unpin), cited capsule evidence, file manifests | image, capsule, file-chunk |

A provider that fails, returns an unparseable ref, or is stale aborts the
sweep for the scopes it covers. A computer whose newest closure is older
than half the grace counts as stale, so its owner's private and internal
scopes are not condemned that run. The sweep report lists every skipped
scope and how long it has been skipped, so the exemption is visible.

**Catalog.** blobd keeps a SQLite (WAL, `synchronous=FULL`) catalog: ref,
cipher digest, frame root, size, class, pack ID (a ULID, no file extension)
and offset, `last_touch`, state (`live`, `released`, `condemned`),
`released_at`, `condemned_at`. `Release` is the only way a keep-set object
leaves: the holder (maild deleting a message, objectgraph replacing a
`body_ref`, a capture tombstone) calls it in the same transaction that drops
its last ref, which moves the object to `released`. A released object whose
ref no longer appears in any provider stream is purged 7 days after it was
last seen in a stream (not 7 days after `released_at`). A `Touch`, a fresh
ref, or the ref's appearance in any provider stream during a mark returns it
to `live` and voids the release, so a release never outlives the next mark
that still sees the ref. A shared object released by one holder therefore
stays live while another holder's ref is streamed, and if that ref later
vanishes without its own `Release`, the guard aborts and pages. The release
journal records (ref, `released_at`) and each revival, is fsynced before the
catalog commit, and fails toward retention: a journal entry with no catalog
state is ignored, a catalog `released` with no journal entry is treated as
live. A keep-set ref
that is absent from the streams without a prior `Release` stays an
abort-and-page condition. It is
rebuildable from packs, which are self-describing (each record header
carries tier, scope, object ID, class and plaintext length; `released`
state is replayed from the release journal, which restic carries); a rebuild sets
`last_touch = now`, which only delays GC. When a second host shares one
store, the catalog moves behind the same interface to the corpus
PostgreSQL instance.

**The GC driver** is a separate process with read access to the holders.
It produces the sorted ref streams and calls `Condemn` and `Purge`; blobd
holds no database credentials.

**Mark and sweep, not counts.** Reference counts drift when a crash lands
between a holder's commit and the store's increment, and there is no atomic
transaction across PostgreSQL, guest SQLite, maild and blobd. Protocol:

1. **Touch on ref commit.** A holder that commits a ref calls `Put` or
   `Touch` for it first and commits only within the **ref window** (1 h) of
   a successful result; a slower holder touches again. This covers a new
   ref to an old object: a Texture revision pinning a 13-day-old
   screenshot, a citation of an old capsule artifact, a manifest naming old
   chunks (batched). A `Touch` of a condemned ref un-condemns it. A `Touch`
   of a purged or absent ref returns not-found; the holder re-puts the bytes
   if it has them, or fails the commit loudly. This replaces the
   serialization `artifactGCMu` gives today ("a pin of an old blob must not
   commit after the sweep has classified it unreachable",
   `artifact_gc.go:197-200`).
2. **Mark.** A sweep records its start `S`, streams every provider and
   merge-joins with the catalog sorted by ref. Memory is the merge cursors
   plus one batch, not the live set.
3. **Condemn.** Refs absent from every stream with `last_touch < S - grace`
   (grace 24 h, more than the ref window) are condemned, subject to the
   guards below.
4. **Purge, after a hold and a recheck.** A condemned ref is purged no
   sooner than its hold: 7 days for keep-set and migrated classes, 24 h
   for scratch classes (unheld images and capsule artifacts additionally
   wait their 14-day retention before condemn). Before purging, the driver
   runs a fresh mark; `Purge` then re-reads each ref's state and
   `last_touch` under the catalog write lock and purges only refs still
   condemned (or released and past the hold), untouched since condemn or
   release, and absent from the fresh mark. This
   is today's `unlinkIfStillUnreachable` recheck (`artifact_gc.go:808,831-850`)
   carried over. Each purge is logged to the deletion-intent JSONL
   (`artifact_gc.go:760-834`) before it happens; pack space becomes dead
   for compaction.

**Guards.** Today's per-namespace live-set guard exists because a wrong
root provider once mass-deleted live og bodies (2026-10-05,
`artifact_gc.go:443-446`, commit 2fa30b179).

- Keep-set classes (capture, og, mail, and platform-tier objects): **zero
  tolerance**. Any condemnation of a keep-set object aborts the sweep and
  pages; a keep-set object leaves the store only through the holder's own
  delete (`Release`, §4 Catalog) followed by the 7-day hold.
- Other canonical classes (file chunks, tape segments, held images, cited
  capsule evidence): per (class, scope), a condemn set above the larger of
  1% and 100 objects aborts the sweep.
- Per-run caps carried over from today (`artifact_gc.go:38-50,283,323-328`;
  Node B: 50,000 deletes, 32 GiB).
- Each class goes active only after a reviewed dry run whose per-class live
  counts match the import receipts (ref coverage 100% for keep-set classes),
  as today's GC admits namespaces one by one.

**What it replaces.** The per-namespace walker over `sha256/<ns>/` with mtime
as liveness and `casReuseMu` with `Chtimes` to protect reuse
(`objectgraph_store.go:56-63`, `artifact_gc.go:106-125,381-430`), the
unswept image pins (`imageblob/pin.go:13-20`) and the absent capture GC.
mtime is not a liveness signal on packs or S3.

## 5. Physical layout

**Packs.** Objects under 16 MiB append to the open pack of their scope (one
open pack per public class, per platform class, per owner, per computer).
A pack seals at 64 MiB or after 1 h idle, matching the tape's 64 MiB seal
(`tapestore/format.go:106`). Objects of 16 MiB or more (bases, large media)
are one extent each, framed the same way. The 154,649 capture-body files
and 21,862 og files become a few hundred files (R0g-09); the 7.4 M-inode
legacy pin tree is the lesson that one file per object costs inodes,
directory scans and backup time far beyond its bytes (inventory §1).

**Write discipline.** Records append to the open pack with a header CRC and
the cipher digest. Puts are group-committed: up to 5 ms or 8 MiB, then one
`fdatasync` of the pack, then one catalog transaction, then the
acknowledgements. The catalog names a pack by ULID; the reader resolves
`<ulid>.pack`, then `<ulid>.open`. Sealing happens only after the pack's
last record is catalogued: write the index (sorted refs, offsets), fsync
it, fsync the pack, rename `*.open` to `*.pack`, fsync the directory.

**Recovery.** On start, blobd scans the open pack past the last catalogued
record (records whose CRC and digest verify are adopted; a torn tail is
truncated) and, after an unclean shutdown, compares every pack sealed since
the last clean checkpoint of the catalog against its index and records,
adopting any record the catalog lacks. Each crash point (after append,
after fsync, after catalog commit, after index write, after rename) gets a
kill test before step 1 ships.

**Compaction.** A sealed pack whose dead fraction exceeds 40% is rewritten:
live records copied to a new pack, catalog switched in one transaction, the
old pack deleted after a reader grace (10 min local; 24 h and a tombstone
on S3, where an in-flight range read of a deleted object fails rather than
finishing). Readers re-resolve through the catalog on not-found. Write
amplification is at most 1/(1 - 0.4) = 1.7x of what is deleted. Compaction
needs free space of one pack, not of the store (contrast the Dolt
compaction that grew 64.7 GB to 89.2 GB, paper-disk §A1).

**btrfs assumptions dropped.** The store uses write, fsync, rename, pread
and unlink. On Linux (ext4, xfs, btrfs) these are used as is; APFS and
NTFS on the desktop apps get a platform layer with its own crash tests,
since their fsync and rename semantics differ. No reflink, `FICLONE`,
subvolume snapshot, sparse-file or mtime semantics. Reflink stays a
compute-host-local optimization of vmctl (`projectionbase/capture.go:22`)
and is never a property a blob relies on.

## 6. Backend choice

**Now: the local pack backend**, on Node B and on a self-hosted single box,
and as the guest's local cache and the desktop's store. **Later: an
S3-compatible backend, SeaweedFS by default** (Apache-2.0, single-node mode,
small per-object index; 1 GB volumes because vacuum needs a volume's worth
of free space; we run repair ourselves). The research is one day old; MinIO
(archived), Garage (metadata limits, no conditional writes) and Ceph (size)
stay rejected without a re-check. Sealed packs map to S3 objects one to
one, so the bucket holds thousands of objects, not millions, and range GETs
serve records.

What S3-compatible buys: off-host and multi-host durability, extents
readable from a second host or a desktop, and erasure coding or replication
we do not write. What it does not buy: encryption, identity, refs, tenancy,
verification and GC, which stay in the Choir layer.

**Off-host copies before SeaweedFS (Q4, accepted):** a nightly restic run
uploads newly sealed packs, a catalog snapshot, the release journal, and the wrapped key
material (interim owner KEKs and escrow rows, platform KEK wrap) so the
private and platform tiers are recoverable from the copy. The run forces a
seal of idle open packs first; what is still open is the stated loss window
(at most a day of puts) until a second host.

## 7. Migration, keep set first

Downtime is acceptable now (owner, 2026-10-29). Every import step runs
under a **writer freeze** for its class: capture and og writes stop (the
capture endpoint returns 503 and the paper's publishing pauses; or corpusd
is down) before the import scan, and resume only after the handler switch.
The referenced-set comparison runs inside the freeze, so a body written
mid-step cannot land in the old tree unseen. The old directory stays,
read-only, until the comparison and a week of reads pass.

1. **Build blobd, the GC driver and `internal/objstore` with the local
   backend**, shipped on the proof ring first (d@e), then Node B's host.
   Red, tier 2. GC starts in dry run for every class.
2. **Capture bodies** (freeze). The Autopaper's 150,811 referenced bodies
   into the platform tier under unchanged `sha256:` IDs (no row rewrite).
   The other owners' ~3.8 K into the private tier under their interim keys;
   under the Q5 default, a body whose sha256 exactly matches a platform
   object is referenced, not copied (the 604 shared bodies): the row's
   blob field carries the platform `sha256:` ref, the read is host-mediated
   under the platform tier's policy, and no private ref aliases a platform
   extent. Their 1,840
   capture rows are rewritten through the normal objectgraph write path:
   new `hmac:` blob fields and a recomputed `content_hash`
   (`objectgraph/object.go:97`, which covers metadata), so derived keys see
   the change and re-derive (`objectgraph/derived_keys.go:490`). The
   canonical ID does not change (captures use an identity key,
   `sourcegraph/captures.go:212`), and anchors check `DocHash`, not blob
   IDs (`publication_versions.go:508`). Each rewritten row's prior metadata
   and `content_hash` go to a migration journal in the same transaction.
   Then verify every referenced ref resolves and hashes, switch `PutBody`,
   `Body` and every reader listed in section 3, and keep the 36 orphans in
   the old directory until the week passes.
3. **og blobs** (freeze). Import the 21,848 referenced bodies by the tier
   of their holding object; `body_ref` is not rewritten (`CanonicalRef`).
   The 14 orphans stay in the old directory with it. The 12,534
   unreferenced `sha256/*.txt` (24.7 MB) are **not** touched: open
   question 6. Fix the `nix/node-b.nix:1298` comment, which says og is
   outside the active GC namespaces while `:1307` includes it.
4. **Mail.** Outbound attachments (10 files) into the private tier; inbound
   bodies stay in `mail.db` until mail moves into the computer.
5. **Guest Textures.** The paper's Texture documents and revisions are
   rows, not blobs: they move with the guest engine (SQLite) migration, read
   offline from a reflink copy (keep-set §8.1). The owner's three
   computers' Textures are nice-to-keep (keep-set owner note): copied if the
   migration makes it cheap. Image blobs their live revisions hold go to the
   owner's private tier.
6. **New writes only, from then on:** guest image blobs and screenshots,
   capsule evidence, file CAS under keyed IDs, the public seed tier for
   shared system files (written once), bases encrypted under the computer
   DEK, tape segments of user computers as internal extents (the paper
   leaves the tape, owner 21:10).

**Rollback.** Each step is reversed by one act: the resolver config points
back at the old directory (never a silent fallback), and for step 2 the
migration journal is replayed backward, restoring each row's prior
metadata and `content_hash`. Step 2's proof includes this rollback on
staging. Writes made after the switch exist only in the new store, so a
rollback after resumed writes first exports those bodies to the old layout.

**Deleted, not migrated** (outside the keep set, pre-launch): legacy pin
files 47.4 GB and 7.4 M inodes, tape segments including the paper's 26.4
GB, the 61.6 GB of projection bases, per-computer file CAS 27.9 GB and
`file-cas-roots` 1.0 GB, capsule artifacts and `computer-inputs` once vmctl
no longer reads it. Blast radius [I]: every computer, not only the owner's,
loses its restore history (tape, bases) and its file CAS; running guest
disks are untouched. Order (Q1, owner-accepted with these conditions):

1. the replacing reader is switched for that class;
2. **export receipts with counts**: the paper's Texture documents and
   revisions, published set reconciled against Store B (3,443 documents,
   11,870 versions at least), and the owner's mail (254 messages, 32
   attachments); the owner's three computers' Textures and files are
   copied where cheap, with counts recorded either way;
3. the guest-engine migration of the paper's store has landed;
4. **a restore drill of the keep set** from the new store and the new
   engines, without the tape or the bases, compared by count and hash
   against the receipts in 2.

The paper's guest disk image on the host stays until the drill passes.

## 8. Precommitted expectations

Round 0g (R0g-01..07) was committed to
`docs/definitions/choir-data-architecture-selection-2026-10-28.precommit.md`
by FP0 on 2026-10-29 about 23:37 UTC (commit 3294bce10). Revision 1 said "not
committed by this note, so not yet records"; that stopped being true at
that commit. The ledger's records are never edited. The original text is
kept below; corrections are new records (round 0h), each naming the record
it corrects and why. Settler: the rule, reading the named artifact.

### Round 0g as committed (unchanged)

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

### Round 0h, corrections to round 0g (new records)

**R0g-08** (corrects R0g-01: its model double-counted the private copies,
since the 9.85 GB of referenced bodies already includes the other owners'
~3.8 K bodies, and the paper's captures are now platform-tier and
encrypted). Stored bytes of steps 2-4 in the local pack store (headers,
frames, pack indexes, encryption overhead included): est 9.95 GB [9.85,
10.3]. Assumes 9.845 GB referenced (9.369 capture, 0.475 og, 0.001 mail)
plus ≤ 1% overhead; the 604 shared bodies cost ~0.04 GB more if the owner
declines the Q5 default, inside the interval. Falsifier: over 10.6 GB.
Artifact: `du` of the store plus the catalog's byte sum.

**R0g-09** (corrects R0g-02: per-scope packs plus one index per pack cannot
fit [150, 260]). Files in the store after steps 2-3, packs plus pack
indexes plus catalog files: est 360 [300, 460], against 176,511 files
today. Assumes ~135 platform capture packs, ~8 og packs, ~30 private
per-owner packs, one index each. Falsifier: over 1,000.

**R0g-10** (corrects R0g-05: the provider set is now six providers, closed
in section 4, and the sweep runs in a separate driver process). Peak RSS
of the GC driver for one full mark, condemn and purge-time re-mark over
1 M catalog entries and the six providers: est 60 MiB [25, 150].
Falsifier: over 256 MiB.

**R0g-11** (corrects R0g-06: it named no class scope although step 6 adds
internal extents, and "95% public captures" encodes the revision-1 tier,
now platform). Growth of the keep-set classes (capture, og, mail) in the
store, 30-day mean after cutover at the then-current Autopaper rate: est
0.6 GB/day [0.35, 1.1], over 90% of it platform-tier captures. Falsifier:
over 1.5 GB/day. Artifact: daily catalog byte sums per class.

**R0g-12** (new scope split out of R0g-06). Growth of the whole store, all
classes, 30-day mean after step 6, with the paper off the tape: est 1.2
GB/day [0.6, 3.0]. Falsifier: over 5 GB/day. Hard to measure: base
rebuild bursts dominate single days; mitigation: report the 30-day mean
and the max day separately. If the paper's tape is still written into the
store, this record is void as stated and a new one is filed before cutover.

**R0g-13** (corrects R0g-07: platform-tier hashes are the realistic oracle,
and "identical responses" needs a measurable definition). A red-team pass
with a second owner's bound computer, given every hash in the capture and
og tables, reads zero platform, private or internal bytes, and over 1,000
probes each of absent, present-forbidden and malformed refs gets the same
status and body, with median latencies within 1 ms of each other: p = 0.85.
Falsifier: any byte read or any distinguishable response.

## 9. Open questions for the owner

1. **Discard pre-launch tapes, legacy pins, bases and file CAS?**
   Accepted 2026-10-29, conditional on the export receipts and the keep-set
   restore drill in section 7. Covers every computer, including the owner's
   three accounts' computers (`5bd6de97`, `5885aafc`, `c061de6f`; the
   hibernated `candidate-fleet-bf04bbc7` included).
2. **Keep the other 29 owners' 1,840 captures, as private objects?**
   Accepted.
3. **Interim keys until the vault's owner-KEK slice lands?** Accepted, as a
   per-owner interim KEK (section 3), not a deployment-wide wrap.
4. **Nightly restic of sealed packs before SeaweedFS?** Accepted, with the
   catalog snapshot and wrapped key material (section 6).
5. **Cross-owner dedup.** Default, owner confirmation pending: a private
   capture may reference an existing platform or public object only on an
   exact sha256 match of the bytes it fetched; the decision runs on the
   host and gives the caller the same response either way. A user's fetch
   is never published or promoted to a shared tier on inferred credential
   or paywall state. If declined, matched bodies get private copies
   (~0.04 GB today).
6. **The 12,534 unreferenced `sha256/*.txt` (24.7 MB, hand-kept
   pre-teardown corpus bodies per the inventory).** Import, keep in place,
   or delete? Kept untouched until answered.
7. **The Autopaper's captures are platform-tier, not public** (stated
   default, section 1). Override if raw captured bodies should be readable
   by every computer or published.

## Ceremony

- Conjecture delta: blob bytes leave per-namespace directories and become
  one store with four tiers and two identity schemes; the per-namespace GC,
  the any-guest capture read and plaintext bases are retired; the paper's
  captures become platform-tier.
- Protected surfaces: owner-content tenancy, Trace/evidence blobs, the GC
  that deletes them, checkpoint closure, the capture and og read paths,
  key custody (interim owner KEKs).
- Admissible evidence: full referenced-set comparisons (not samples) inside
  a writer freeze before each swap; dry-run per-class counts matching the
  import receipts before any class goes active; the export receipts and the
  keep-set restore drill before any deletion; the round 0g and 0h artifacts;
  a staging red-team receipt for R0g-13.
- Rollback: one act per step (resolver config, plus the migration journal
  for step 2), rehearsed on staging; the old directory stays until the
  comparison and a week of reads pass. Purges are irreversible and wait for
  the 7-day hold.
- Heresy delta: `discovered`: projection bases are plaintext copies of owner
  stores on the host; og bodies are installed without fsync; capture bodies
  have no GC namespace and an existence oracle by hash; `captureBlockDoc`
  reads bodies with no caller; the `nix/node-b.nix:1298` comment
  contradicts the active namespace list at `:1307`. `introduced`: a new
  host service, a GC driver, a catalog to keep consistent with packs, and
  interim owner key custody. `repaired`: none until built.

## Revision 2 log (2026-10-29)

Reviews: `.agentic-consensus/object-storage-r1/out/devin.out` (D) and
`omp-deepseek-v41-flash.out` (S). Each claim was checked against the code.

| Finding | Ruling | Addressed in |
|---|---|---|
| D-B1 resurrection: old object, new ref, purge without recheck | accepted | §4 steps 1 and 4 (Touch on ref commit, purge-time re-mark and locked recheck), stale-closure rule, retained-checkpoint invariant |
| D-B2 og `body_ref` is path form, not `sha256:` (`objectgraph_store.go:49` [V]) | accepted | §2 canonical ref grammar; §7 step 3 (no rewrite); dry-run coverage gate in §4 |
| D-B3 condemn guard covers only three classes; per-run caps dropped | accepted | §4 Guards (per-class thresholds, caps from `artifact_gc.go:38-50` carried over, per-class activation) |
| D-B4 R0g-06 incoherent with step 6 | accepted; the magnitude claim (paper tape ~2.5 GB/day into the store) partly rejected: the paper leaves the tape (owner 21:10), so step 6 writes only user computers' tape | §8 R0g-11, R0g-12 |
| D-B5 crypto-erase false under a deployment-wide wrap; custody unspecified | accepted (owner Q3) | §3 Keys, Crypto-erase; §6 backups |
| D-B6 12,534 `*.txt` deleted without an owner answer | accepted | §7 step 3; §9 Q6 |
| D-B7 no writer freeze during import | accepted | §7 preamble |
| S-B1 public capture tier contradicts "owners' captures are private" | accepted as a stated default (platform tier) | §1, §2 identity, §9 Q7 |
| S-B2 no owner key exists for guests; file CAS uses the computer DEK (`filecas.go:143-160` [V]) | accepted | §2 caller-computed ID; §3 Keys and Who encrypts (per-owner interim KEK, stable ID key, delivery under the computer DEK) |
| S-B3 purge has no recheck; resurrection undefined | accepted; the cited lines are off (recheck is `unlinkIfStillUnreachable` at `artifact_gc.go:831`, called at `:808`), substance correct | §2 IfAbsent semantics; §4 steps 1 and 4 |
| S-B4 1% keep-set guard permits silent loss; purge after 1 h | accepted; its reading that the ceremony's "week" governed GC purges was a misread (it governed the migration's old directory), but the 7-day hold is adopted | §4 Guards, step 4 |
| S-B5 root providers not closed (maild, platform-update, capture JSON, image pins, tape segments) | accepted; each sub-claim verified (`artifact_gc.go:447` reads only `body_ref`; `pin.go` has no unpin; no segment table) | §4 provider table |
| S-B6 rollback broken by the rewrite; `captureBlockDoc`; `content_hash`; og alias | accepted; added: the capture canonical ID is stable (identity key, `sourcegraph/captures.go:212`) and anchors check `DocHash`, so only `content_hash` and derived keys move | §3 reader list; §7 step 2 and Rollback |
| S-B7 seal/catalog naming; crash between fsync and catalog | accepted | §4 Catalog (pack ULID); §5 Write discipline, Recovery |
| S-B8 range reads not bound to object or length | accepted | §2 Object format |
| S-B9 deletion before engine migration and export | accepted in its conclusion; the premise that tape and bases are the only host-side copies of the paper's unpublished revisions is rejected: the paper's guest disk image is on the host (keep-set §7, 31 GiB `vm-state`) | §7 deletion order; §9 Q1 |

Non-blocking items taken: R0g-01 double counting and R0g-02 file count
(R0g-08, R0g-09); per-class retention (§4 step 4); private bytes to browsers
through the owner's computer (§3); bases built host-side (§3); corpusd sees
private capture plaintext and inbound mail stays plaintext (§1, §3); pack
index fsync (§5); measurable R0g-07 (R0g-13); spool cost of dedup-before-
upload (§2); `file-cas-roots` and the owner's three computers enumerated
(§7, §9); the nix comment contradiction (§7 step 3, heresy delta); S3
compaction grace (§5); `Touch` with a caller and `ListByRef` removed (§2);
open packs host-local and single-writer (§2); GC driver outside blobd (§4);
NTFS is not POSIX (§5); bases keep two until the new restore path proves
(§1); restic forces a seal and carries the catalog and keys (§6). Not taken:
none outright; the multi-host catalog and S3 operations stay later work.

## Revision 3 log (2026-10-30)

Re-review: `.agentic-consensus/object-storage-r2/out/devin.out`, send_back,
all sixteen round-1 findings closed. NB-1 (no legal exit for keep-set
objects): accepted; `Release`, the `released` state and the release journal
added (§2, §4 Catalog and Guards). Notes taken: key-delivery channel stated
as new work with the real direction (§3); Q5 shared bodies are row-level
references with host-mediated reads (§7 step 2); record headers carry
tier, scope and ID (§4 Catalog); content-key rotation rewrites packs (§3);
skipped scopes surface in the sweep report (§4).

## Revision 4 log (2026-10-30)

Round-3 re-review (`.agentic-consensus/object-storage-r3/out/devin.out`):
NB-1 closed. NB-2 (one holder's `Release` of a shared keep-set object
disarmed the zero-tolerance abort for every other holder): accepted; stream
presence during a mark now voids a release, and the hold runs from the
last stream sighting (§4 Catalog). Notes taken: `IfAbsent` and the purge
recheck name `released` (§2, §4 step 4); release journal contents, fsync
order and failure direction stated, and restic carries it (§4, §6). This is
the third review round on the GC deletion mechanism; per the operating
contract no fourth round is added. The build's own tier-2 code review is the
next check on this surface.
