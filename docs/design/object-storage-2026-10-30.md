# Object storage: one store for every blob, by tier and owner (2026-10-30)

Status: design, revision 13 (after the round-12 reviews,
`.agentic-consensus/object-storage-r12/out/`: both send_back, and the
[restore and audit design](computer-history-restore-audit-2026-10-30.md),
revision 1). Revision 5 replaced the object-level release state of
revisions 3-4 with holder-scoped holds; revisions 7-10 made the holder
probe a by-key resolver and gave the batch guard an exit; revision 12 made
the cutover one act and retired the pre-cutover checkpoints; revision 13
keeps a tape for every computer, replaces host-built bases and host closure
rebuilds with guest-made sealed snapshots, takes the tape out of the hold
audit, and fixes the drain, the private-ref rewrites and the rollback
(sections 1-7). Not ratified; nothing is built or deployed by this note.
Revision 1 is commit 3294bce10; the revision logs at the end map every
finding to where it is addressed.
Mutation class of the work it proposes: **red** (tenancy and encryption of
owner content, Trace/evidence blobs and the tape, the GC that deletes them,
snapshot closure, legal holds). Review tier: 2 (Devin and DeepSeek v4.1 flash), because a defect
would silently disclose private bytes or silently delete keep-set content.
Labels: **[V]** verified by a cited command receipt or code read; **[H]**
hypothesis, to be settled by the predictions in section 8.

Owner direction (2026-10-29): "we do have to also consider the object storage
of when we're capturing these external files ... we make screenshots and
there they're all of these data blobs ... we don't yet have a S3 compatible
service running ... that too has to be multi-tenant and secure." Order of
priorities: security, stability, then scalability and simplicity. Self-hosting
is committed; a single local box must run with no hosted dependency.

**Owner ruling (2026-10-30): one cutover, no phased slice.** Captures, og,
mail, guest file CAS, tape segments, snapshots, images and capsule evidence
all move in one major cutover ("measure a dozen times, cut once"). There
is never a second data system: no class keeps a live legacy reader or
writer after the cut, and the legacy bytes that stay until the deletion
order (section 7) are frozen rollback material that nothing reads. The
scope is not narrowed.

Owner decisions on revision 1 (2026-10-29): Q1-Q4 accepted, Q1 conditional
on export receipts and a restore drill of the keep set, Q3 with a per-owner
interim key-encryption key (section 9).

**Owner directions (2026-10-30), carried by the restore design.** Every
computer keeps a tape. User computers keep their whole tape; the Autopaper
keeps a 7-day hot window and its older sealed segments are archived, never
purged, until an analytical database loads them; its precommitment records'
witnesses are kept forever. Any user computer can be restored to any head
inside a 7, 14 or 30 day window, even when its VM is broken, and every
operator act is audited. These supersede the 2026-10-29 reading that the
Autopaper leaves the tape. This note is the byte store under that design:
where it and the restore design differ, the difference is listed in
section 9, Conflicts with the restore design.

Evidence, not re-derived here:
[keep-set accounting](../evidence/keep-set-accounting-2026-10-29.md),
[storage inventory and its capture-body correction](../evidence/node-b-storage-inventory-2026-10-29.md),
[paper disk and base liveness](../evidence/paper-disk-and-base-liveness-2026-10-29.md),
[research, object-store section](../evidence/data-architecture-research-2026-10-28.md),
[data rearchitecture](data-rearchitecture-2026-10-28.md),
[selection, 2026-10-29 sections](../definitions/choir-data-architecture-selection-2026-10-28.md),
[more than one home](../problems/data-has-more-than-one-home-2026-10-24.md),
[secrets vault](secrets-vault-design-2026-10-28.md),
[restore and audit](computer-history-restore-audit-2026-10-30.md),
[tape window sizing](../evidence/tape-window-sizing-2026-10-30.md).

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
   HMAC-SHA-256 of the plaintext under a per-owner (or per-computer) ID key,
   except record witnesses, which the host assembles from bytes it already
   holds and names by the SHA-256 of what it stores. Every non-public
   object is encrypted with a random-nonce AEAD under a random per-object
   key, and its header carries a commitment to that key, or it is stored as
   its writer sealed it (snapshot manifests, record witnesses). No
   convergent encryption anywhere.
4. **Refs live with the holder; the catalog mirrors them as holds.**
   Holders own refs. The store's catalog records each object and one
   **hold** per (ref, provider, holder scope, holder key), set by `Touch`
   before the holder commits a ref and released by `Release` before it
   drops one. GC is an audit that reconciles holds against a closed list of
   root-provider streams, ends holds no stream confirms by looking the
   holder up through its own read path, then purges objects with no unreleased hold after a
   hold period and a locked recheck. Not reference counts: a hold set is
   reconciled member by member, a counter cannot be (section 4).
5. **Packs, not one file per object.** Small objects append to 64 MiB
   per-scope packs; write, fsync, rename, pread; no btrfs feature is assumed.
6. **A separate host process, `blobd`,** with its own system user and data
   directory, holding no owner keys and no database credentials. It also
   hosts the tape writer and reader (today `tapestore` inside corpusd). The
   GC driver that reads holders is a separate process.
7. **One cutover for every class, rehearsed many times, cut once.** The
   keep set is imported; the rest, the pre-cutover tapes, bases and
   checkpoints included, is deleted, not migrated, and only after export
   receipts, a restore drill of the keep set and sealed-restore drills.
   Every computer's tape continues from its drained head, and its first
   post-cutover restore point is a snapshot its guest makes.
8. **The tape is outside the GC.** Tape segments and record witnesses are
   never purged by the hold audit; they leave the store only by an audited
   deletion act from a closed list (section 4, Retained classes), refused
   under legal hold.

## 1. Blob classes: today and target

Tiers:

- **public**: readable by every computer of the deployment; exposure to the
  internet is a separate publication policy on the holding object. Plaintext
  at rest.
- **platform**: held by the Autopaper (the platform owner) and not
  published. Read only by host services, which hold the platform key
  lease; the Autopaper's own computers read platform objects through
  host-mediated endpoints and never receive the platform content key.
  Never readable by another owner's computer, never served raw to the
  internet. Encrypted at rest under a platform key.
- **private**: one owner; encrypted under that owner's keys.
- **internal**: one computer, encrypted under keys derived from its
  computer DEK (restore design §3.1); no host service holds a lease on
  that key (section 3, Keys, for the DEK's custody before release). Host
  services read the bytes of its tape, snapshot manifest headers and
  record witnesses, which are ciphertext or content-free control records.

**Stated default, owner may override: the Autopaper's captures are platform,
not public.** The owner ruled "everything is private except what is public
(the Autopaper's articles and what users explicitly publish) ... drafts and
owners' captures are private" (selection §Owner answers). The paper's raw
captured bodies are third-party pages, often under copyright; public readers
get the articles and what the citation and excerpt policy allows (the
resolved blocks a published version pins, which are og bodies and public),
not raw full bodies. Making captures platform rather than public also
removes the reading-list oracle (any computer probing whether the paper
captured given bytes). No private capture references a platform object:
there is no cross-owner dedup (section 9 Q5).

| Class | Today [V] | Tier | Canonical? | Retention | Who reads |
|---|---|---|---|---|---|
| Capture bodies (raw, blocks, text) | 154,649 plaintext files, 9.37 GB, `capture-bodies/<2hex>/<sha256>`, no GC, any bound guest reads any hash (`internal/platform/captures.go:86,214,228`) | Autopaper's: platform. Any other owner's: private | canonical (keep set) | while a `web_capture` object holds it; never purged while a hold is unreleased (section 4) | platform: host services; the Autopaper's computers through host-mediated reads. Private: the owner's computers |
| og blobs (published Texture manifests, resolved blocks, transclusions, publication and proposal text) | 21,848 referenced `sha256/og/<hex>.bin`, 0.47 GB; written without fsync (`internal/platform/objectgraph_store.go:44-74`); GC active on Node B (`nix/node-b.nix:1307`). Publication and proposal text is written to `sha256/<hex>.txt` and `sha256/proposals/<hex>.txt` (`service.go:411-412,790-791`) and named by the metadata `storage_ref` of a `choir.artifact_blob` row (`publication_graph.go:204-217,820-830`), which the public bundle reader follows (`service_publication_read.go:185-199`) [V]; the 12,534 top-level `*.txt` were counted as unreferenced because only `body_ref` was checked (keep-set §2, the og blobs) | the holding object's tier, passed by the writer to the `Put` and by the provider to `CanonicalRef`, never inferred from the ref's encoding: public for a published object, otherwise platform (the Autopaper) or private (other owners). A row's tier is fixed when the row is created. A body held at two tiers is two objects. Publish is copy-only: the published object is its own row with its own public body object and hold, the draft keeps its own row, object and hold, and no write changes the tier of an existing row | canonical (Autopaper object graph; the referenced text files are published-version content, keep set) | refs from `og_objects.body_ref`, a published row's `image_refs` and a `choir.artifact_blob` row's `storage_ref` | per the holding object's publication policy |
| Image blobs and self-development screenshots | guest `state.image-blobs/sha256/<hex>`, plaintext, not in file CAS; pins are guest files with no unpin and no sweep (`internal/imageblob/imageblob.go:20-60`, `pin.go:13-20`; problem doc violation 11) | private (owner) | canonical when held by a Texture revision or evidence object, live or in a retained snapshot; otherwise scratch | held: as long as any holder, including a retained snapshot's closure; unheld: 14 days | the owner's computers. Publishing a revision that embeds one copies it into the public tier as an explicit act: the copy is an og-class object (keep-set, public tier, the image's media type) held by the published og row, which names it in its `image_refs` metadata field (section 2, og class rule). That act needs a Texture change first: today no embed is publishable and publication refuses a document holding one (`internal/texturedoc/embed.go:21-22,72` [V]), so no published image exists to migrate |
| File CAS chunks | 27.9 GB in 197 per-computer dirs; deterministic synthetic nonce under a per-computer DEK (`internal/filecas/filecas.go:143-160`); the seeded `files/Source` probably copied into each [H, inventory §1] | owner files: private. Shared open-source and seeded system files: public | canonical (owner files) | reachable from the computer's retained file roots | the owner's computers; public seed by all |
| Tape segments and payloads | `tape-seg/` 27.2 GB (26.4 the paper), sealed at 64 MiB or 50,000 records (`internal/tapestore/format.go:106-107`), written by corpusd's pin path for projection batches only (`platform/tape_segments_write.go:17-26`); legacy one-file-per-pin 47.4 GB, 7.4 M inodes (`event_artifacts.go:118-125`); outside artifact GC (`artifact_gc.go:106-110`); payloads plaintext except Texture audit (restore design §1.1) | internal, per computer. From the cut every payload is sealed by the guest (envelope v2) except two content-free control kinds; envelopes and receipts are content-free control records (restore design §3.1) | canonical for every computer, the Autopaper included: the computer's source of truth (restore design §2) | user computers: the host holds a sealed copy covering at least the tenant's restore window, whole by default; the restore design decides whether the guest's own volume holds the whole tape and the host a replica or a window. The Autopaper: 7-day hot window; older sealed segments archived, unchanged, until the analytical database loads them. Outside the hold audit; trimmed or removed only by an audited deletion act (section 4, Retained classes) | that computer (plaintext); host services (control records and ciphertext) |
| Snapshot chunks and manifests (new) | none; today's 61.6 GB of projection bases, 425 files, are plaintext host-built copies of guest stores (no AEAD in `internal/projectionbase`, non-test files [V grep]) and are retired, not migrated | internal, sealed by the guest (restore design §3.1); a manifest has a plaintext header (head, sequence, witness digest, code identity, chunk refs, closure refs) | restore points: derived from the tape and checked against it, but held as a canonical class (section 4) | the restore design's window rule (§4), pins and legal holds; 24 h hold period | that computer; host services read manifest headers |
| Record witnesses (new, windowed computers only) | none; the paper's pre-cutover record batches are plaintext on its tape | internal (the paper's scope); a bundle of an event's envelope, guest-sealed payload, pin receipts, head receipt and anchor inclusion proof (restore design §4) | canonical (PSA evidence) | forever; outside the hold audit (section 4, Retained classes) | the paper; host services and reviewers given a disclosed key |
| Mail attachments | outbound host files `mail/attachments/outbound/<owner>`, 1.2 MB; metadata in the per-owner `email_attachments` table; inbound bodies inline in the per-owner `mail.db` (keep-set §4) | private (owner) | canonical (keep set) | while the message exists | the owner's computers and maild |
| Capsule artifacts | guest `/mnt/persistent/capsule-artifacts`, 0.18 GiB, 4,848 files on the paper (paper-disk §A1); `nix/autoputer-vm.nix:621,1078` | private (owner) | evidence when cited, else scratch | cited: as the citing evidence object; else 14 days | the owner's computers |
| Platform update payloads | `sha256/platform-update` 1.4 GB, GC working; live set from the route tables (`artifact_gc.go:432-440,475-540`) | public (release artifacts) | derived from the build | the route tables' current and rollback refs | hosts and computers |

Captures change tier on purpose. They were "shared content"
(`captures.go:213`); the owner ruled owners' captures private, and the
paper's captures move to the platform tier (above). Projection bases are
not migrated: they are a plaintext copy of an owner's store built by the
host, and the restore design replaces them with snapshots the guest makes
and seals, so the host builds and unwraps nothing (restore design §2, §3).

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
    ID    string // sha256:<hex> (public, platform, record witness) or hmac:<hex> (private, internal)
}

type Holder struct {
    Provider string // one of the closed provider list (section 4)
    Scope    string // owner or computer ID; "" for platform-wide holders
    Key      string // the holding row: capture object ID, og object ID,
                    // attachment ID, revision or manifest ID, snapshot ID, route slot
}

type Store interface {
    Put(ctx, Caller, PutRequest, io.Reader) (Ref, PutResult, error) // streaming; IfAbsent by default
    Get(ctx, Caller, Ref, Range) (io.ReadCloser, Info, error)        // verifies before or while returning bytes
    Stat(ctx, Caller, Ref) (Info, error)
    Touch(ctx, Caller, Holder, []Ref) ([]TouchResult, error) // before the holder commits refs: insert or refresh holds
    Release(ctx, Caller, Holder, []Ref) error                // before the holder drops refs: release those holds
    Sight(ctx, GCRun, []Sighting) ([]Repair, error)          // audit: record stream sightings, repair missing holds
    Purge(ctx, GCRun, []Ref) ([]Ref, error)                  // rechecks under the catalog lock, then marks pack space dead
    HoldScope(ctx, Caller, Act, Tier, scope string) error    // legal hold: Purge and DeleteByAct refuse the scope
    ReleaseScope(ctx, Caller, Act, holdID string) error
    DeleteByAct(ctx, Caller, Act, Selector) error            // the only removal of retained classes (section 4)
}

// One per provider, served by the holder's own service, called only by the
// GC driver (section 4, holder probe).
type HolderResolver interface {
    HolderRefs(ctx, provider, scope, key string) (exists bool, refs []Ref, err error)
}
```

`Act` is the ref of the platform audit entry that authorizes the call
(restore design §5); blobd records it and does not judge it. Only the
control service may call the three act methods. The tape has its own small
interface in blobd, the tapestore appender and reader moved out of corpusd
(section 5, Tape): append records for a computer, read a record by digest,
list a computer's segments, archive a segment.

A caller may `Touch` or `Release` only holders of its own provider and
scope (a guest only its own computer's holders, corpusd only the corpus
providers), and `Touch` needs read access to the ref, answering absent and
forbidden alike. The one exception is the GC driver, which may `Release`
any holder, and only with recorded holder-probe evidence (section 4).
`PutRequest` may carry a holder; the put then also sets its hold in the
same catalog transaction. Keep-set writers always put with a holder.

`PutRequest` carries tier, scope, class (capture, og, image, file-chunk,
snapshot-chunk, snapshot-manifest, record-witness, mail, capsule,
platform-update, audit), media type, declared size and, for
private and internal puts, the **caller-computed ID**: blobd never holds an
ID key, so it cannot compute or confirm an HMAC (and is no confirmation
oracle). A reader that holds the key recomputes the HMAC after decrypting
and rejects a mismatch. The writer computes the ID while streaming the
plaintext to a local spool, then asks the catalog; dedup is decided before
the upload at the cost of one spool write for large objects. The tier and
scope come from the writer, which takes them from the holding object (for
og, the holding object's publication state and owner); blobd never infers
a tier from an ID's encoding.

**Conditional put is a catalog operation**: `IfAbsent` on a present
(not purged) ref first verifies that the catalog's pack and offset name a
readable record whose header and cipher digest match; then it returns the
ref, sets the object's `last_touch`, sets or refreshes the request's hold
if it carries one, and discards the upload, all in one catalog transaction.
If the extent or record is missing or does not match, the upload is
written as a new record, the catalog is re-pointed at it, the request's
hold is set or refreshed as on a hit, and the repair is logged, all in one
catalog transaction. On an absent or purged ref it writes a new record. Every write of a
record (first put, re-put after a purge, repair) increments the object's
**generation**, and the record header carries it; a hit writes no record
and keeps the generation. Objects have no released or condemned state to
undo: release is a property of one hold, and purge eligibility is computed
(section 4).

**Class is per hold, not per object.** Identity excludes class and media
type, so the same private bytes can be put first as an image and later as
a mail attachment, and `IfAbsent` keeps the first record. The object's
`class` column records the first writing put and decides only packing; the
catalog's media type is the first put's and is checked against the header,
while each holder keeps and serves its own media type. Every hold records
the class of its holder (from the provider, set by `Touch` or the holding
`Put`). An object's protection is the strongest class among its hold rows,
released or not (keep-set > other canonical > derived > scratch): it is a
keep-set object for the audit branches, Guards and hold period while any
hold row is keep-set, whichever class inserted the bytes. A never-held
object is judged by its put's class. Extent names are unique (a ULID per
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
stream a ref is `(tier, scope, sha256:<hex> | hmac:<hex>)`. One function,
`objstore.CanonicalRef(class, holderTier, holderScope, s)`, maps a holder's
stored string to that tuple, and the root providers, the holder resolvers
and the readers all use it. The tier and scope are inputs, taken from the
holding object, never read from the string. A string that does not parse,
or that a class rule rejects, is a provider error, which suspends the
provider's scopes, and a resolver error, which makes the probe unable to
run (section 4).

**Designated ref fields.** Each class rule below names the fields a holder
may keep refs in. A ref in any other field is not a holder by contract:
writers must put refs only in designated fields, and no provider, resolver
or by-ref query reads elsewhere. Every cutover rehearsal's dry run
counts canonical-looking strings (`sha256:`, `hmac:`, `sha256/og/` and the
legacy forms below) in the values of the other structured fields of that
class's holders (metadata keys and columns), never in body or content
text, so a captured page or a revision that quotes a hash is not a hit;
the expected count is zero, and any hit pages and blocks the cut.

Legacy encodings it accepts, all from the code [V]: `sha256:<hex>`
(capture metadata); `sha256/og/<hex>.bin` (`og_objects.body_ref`,
`objectgraph_store.go:49`); `artifact:sha256:<hex>` (image and capsule
refs, `imageblob.go:91`); bare 64-hex (file-cas chunk digests inside
manifests, `filecas.go:31-45,259-265`, and platform-update digests);
`sha256/file-cas-chunks/<computer>/<hex>` and
`sha256/file-cas-roots/<computer>/<hex>.json` (`file_cas_blobs.go:20,27`);
`sha256/platform-update/<hex>` and
`artifact+sha256://<hex>/sha256/platform-update/<hex>`
(`platform_update.go:61,94`); `sha256/pin-receipts/<hex>.json`
(`event_artifacts.go:124`); `sha256/<hex>.txt` and
`sha256/proposals/<hex>.txt` (`choir.artifact_blob` `storage_ref`,
`service.go:411,790`).

Class rules, one per class; a legacy `sha256` form is accepted only where
its rule says so:

- capture: designated fields the three blob fields of the
  `choir.web_capture` metadata. In a private holder `hmac:` only (its own
  private object; a `sha256` form is rejected); in a platform holder
  `sha256:` only.
- og: designated fields `body_ref`, on a published row the `image_refs`
  metadata list, and on a `choir.artifact_blob` row the metadata
  `storage_ref` (publication and proposal text). Public and platform
  holders: `sha256/og/<hex>.bin`, `sha256/<hex>.txt`,
  `sha256/proposals/<hex>.txt` or `sha256:`; private holders: `hmac:` only
  (section 7 part 3). A `sha256` form in a private og row is rejected.
  Publishing a revision that embeds an image puts a public copy (class og,
  the image's media type) with the published row as holder and commits the
  row with the copy's `sha256:` ref in `image_refs`, in the same row write
  as its `body_ref`; the og provider streams both fields and the og
  resolver returns both. This waits on the Texture change that makes an
  embed publishable (section 1 image row); until then `image_refs` is
  always empty.
- mail: the attachment row's `blob_ref`, `hmac:` only; the owner-facing
  `sha256` column is never parsed as a ref (section 7 part 4).
- image and capsule: the holder's guest ref row, `hmac:` only; the
  `artifact:sha256:` string in revision or evidence content is a display
  and lookup key, not a ref (section 7 part 5).
- file-chunk: a version-2 manifest, built by the guest, names chunks as
  `hmac:<hex>`; a version-1 manifest's bare hex and the
  `sha256/file-cas-*` paths name legacy chunks outside the store, which
  `CanonicalRef` reports as legacy (counted separately, never streamed;
  frozen at the cutover, never read after it, and deleted with the
  per-computer file CAS, section 7). Version 2 is a guest/host protocol
  change [V]: `manifestVersion = 1` and `VerifyRoot` rejects any other
  version (`filecas.go:24,102`), the host parses manifests at
  `file_cas_http.go:119` and the guest at `autoputer/file_sync.go:219`, and
  the guest builds them (`file_sync.go:569`). There is no mixed period: the
  cutover release carries both sides, the host accepts only version 2 from
  the cut, every guest builds only version 2 after its cutover boot, and
  the chunk fetch path is store refs read through blobd.
- snapshot-chunk: internal, `hmac:` only, the HMAC of the chunk plaintext
  under the computer's history ID key (`choir/history/v1/id`, restore
  design §3.1), computed and sealed by the guest; unchanged chunks dedup
  through `IfAbsent`. Designated field: the chunk list of a snapshot
  manifest's plaintext header.
- snapshot-manifest: internal, `hmac:` only, guest-computed; stored as
  written (below), so host services can read its plaintext header.
  Designated field: the snapshot row's `manifest_ref` (section 4,
  snapshots provider).
- closure refs in a manifest header: image, capsule and file-chunk refs,
  `hmac:` only, read with the snapshot row's owner and computer as tier
  and scope (section 4, Snapshot closures).
- record-witness: internal, `sha256:` of the stored bundle, computed by
  the host that assembles it; stored as written. A retained class: no
  holder, never purged by the GC (section 4).
- audit: platform tier, `sha256:`, encrypted under the platform key like
  any platform object: the platform's audit-chain segments, deletion
  manifests and blobd's daily access logs (restore design §4-5 calls this
  the platform internal scope). A retained class.
- platform-update: public, any of its `sha256` forms.
- tape: not a class of `Put` and not a ref. Segments are extents of the
  computer's internal scope named by (computer, segment number), written
  by blobd's tape writer (section 5, Tape), outside the catalog and the
  hold audit.
- pin receipts: legacy, outside the store (deleted with the legacy pin
  tree, section 7); reported as legacy like version-1 chunks.

**Private holders store the canonical private ref.** A private or
internal ID is an HMAC the driver cannot compute (no ID key), so a holder
that kept only the plaintext sha256 could never be matched to its object.
Every private and internal holder therefore stores the canonical `hmac:`
ref in a field of its own row (or of a ref row written in the same
transaction), alongside any legacy value it keeps for its users. The
field, transaction and migration journal for each holder are in section 7
parts 2-6.

**Merge-key encoding.** Streams and the `holds` table sort and compare on
one byte encoding of the 4-tuple (ref, provider, holder scope, holder key),
where ref is (tier, scope, ID): each field is UTF-8 with no NUL byte
(validated on write; a NUL is a provider error), the fields are joined by
`0x00` in that order, and keys compare as bytes (`memcmp`). The `holds`
primary key stores this encoding (or the same columns under `BINARY`
collation in the same order), and the dry run asserts that each stream's
order and the table's order agree. "(ref, holder)" anywhere in this note
means this 4-tuple.

**Object format.** Header (version, tier, class, media type, plaintext
length, frame count, key version, key source, key commitment, frame size),
then frames of 1 MiB. The key source is a wrapped per-object DEK (owner
content key, platform key), or for a snapshot chunk the random salt from
which the guest derives its per-object key under the history snapshot key
(restore design §3.1); the salt is never derived from the content. The
**key commitment** is SHA-256(domain ‖ per-object key), with domain
`choir/objstore/v1/key-commit`, or `choir/history/v1/key-commit` for
snapshot chunks, on every encrypted object, platform tier included. A
reader checks it before decrypting, and a disclosed per-object key is
checked against it, so a key cannot be swapped for one that opens the same
ciphertext to other bytes (XChaCha20-Poly1305 is not key-committing).
Encrypted frames are XChaCha20-Poly1305 with a
STREAM-style nonce (random prefix, counter, last-frame flag) and AAD
binding `tier || scope || id || class || plaintext length || frame count ||
key commitment || frame index`, so a truncated or shortened object fails
authentication.
The AAD also binds the media type, and the catalog records it; a reader
takes the media type from the catalog and rejects a header that differs.
HKDF `info` strings are `choir/objstore/v1/<purpose>` (vault design §2.3
convention), and `choir/history/v1/<purpose>` for history keys.
**Stored as written:** snapshot manifests and record witnesses are formats
their writer already sealed where they carry content (restore design
§3.1); blobd adds no encryption and frames them like public objects, with
per-frame digests. Public objects carry per-frame digests
`SHA-256(id || frame index || plaintext length || frame bytes)`, and the
catalog stores the full list (32 bytes per MiB of object); a range read
checks each frame it returns against the catalog's digest for that frame,
never against the header or a backend-supplied digest. Every stored object also has a
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
  computer ID; except record witnesses, named `sha256:` of the stored
  bundle by the host, which has no ID key. The bundle holds only
  ciphertext and content-free records, so its hash discloses nothing the
  host does not already hold.
- Encryption is never derived from content: a per-object key is random or
  derived from a random salt, and dedup comes from the ID lookup. New file
  CAS chunks move to this scheme; old chunks still open (the nonce is in
  their prefix).

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
the owner's packs and is a compaction-class operation, not a free one. Its
order: deliver the new content-key version to the owner's running guests
(which then hold both versions), then rewrap; the old version is retired
only after every pack is rewritten. A guest that was down gets the current
versions at its next boot. The platform tier has the same structure under
a platform KEK held by the vault's deployment path. Internal objects use
keys derived from the computer DEK. No host service holds a lease on it
for this store. Until the restore design moves the DEK off the guest's
data image (a ring-rolled change after the cutover, host custody accepted
before release by the owner), a host that reads that image can read the
DEK; this store adds no other copy.

**Who encrypts.** The party that holds the plaintext and a key lease:

- guests for guest-originated private bytes (images, screenshots, files,
  capsule artifacts). A guest receives its owner's content key and ID key
  at boot. **Owner-key delivery is new work.** A host-to-guest channel
  exists for the computer DEK only [V]: realization key delivery
  (`HandleRealizationKeyDelivery`, `platform/key_delivery.go:45-144`) binds
  to the current issuance, writes a transparency entry, unwraps the
  escrowed DEK and seals it to the realization's key, and a guest uses it
  when its key file is missing (`autoputer/run.go:335-372`). It does not
  deliver owner keys. Owner-key delivery reuses that channel's shape:
  bound to the issuance, audited, corpusd opening the owner keys under its
  vault lease for that owner and sealing them to the realization's key. A
  delivery is ephemeral per boot: the guest keeps the owner keys in memory
  and never writes them, or their wrap, to guest storage, and the host
  keeps no copy of the delivered wrap. A sovereign owner's computer gets
  its keys from the owner's client instead (restore design §3.2). Guests
  also seal history (tape payloads as envelope v2, snapshot chunks and
  manifests) under keys derived from the computer DEK (restore design
  §3.1). File CAS uses the computer DEK only (`filecas.go:143-160`);
- nobody on the host for history: blobd's tape writer appends records the
  guest sealed or that carry no content, and record witnesses are bundles
  of such records. No host process builds a store or opens a computer DEK
  for routine work; checkpointd's rebuild and its escrow unwrap
  (`cmd/checkpointd/worker.go:181-205`) are deleted at the cut;
- corpusd for the platform tier (the paper's captures) and for other
  owners' captures, and maild for mail attachments, under a vault lease for
  that owner. The platform content key goes to host services only; every
  platform read by an Autopaper computer is host-mediated. corpusd sees a private capture's plaintext on its request
  path; the tenancy gain is at rest and across tenants, not confidentiality
  from corpusd.

blobd stores ciphertext and never needs a key to store, verify (by cipher
digest), record holds, audit or purge.

**Crypto-erase.** Destroying an owner's interim KEK and its escrow copies
makes that owner's private objects unreadable on every host, and only that
owner's. Backup copies of the wrapped KEK and escrow rows are erased by the
vault's deletion journal (vault design §2.8); until that journal has run
against a backup set, that backup can still open the owner's objects. The
guest delivery path adds no persistent copy: deliveries are per boot and
never stored (above), and the erase stops the owner's running computers,
so no wrap of the owner keys survives under a computer DEK. Packs
are per owner scope, so a deleted owner's packs are also removed physically
in one pass, without compacting other owners' packs. A computer is erased
the same way (restore design §3.3): its DEK protectors and its volume are destroyed, then
`DeleteByAct` removes its internal scope (tape, snapshots, record
witnesses) in one pass. Both are refused while a legal hold covers the
scope. The erase's tombstones and deletion manifest are written as for a
purge (section 4); head receipts, anchors and audit entries stay in the
control database.

**Least-privilege read.** A guest reaches blobd only through the existing
bound-guest identity (`bindTapCaller`, `objectgraph_handlers.go:379-390`).
blobd allows a read when the ref is public; platform when the caller is a
host service (an Autopaper computer reads through the host service, which
checks it); private when the ref's
scope equals the caller's owner; internal when it equals the caller's
computer, or when the caller is a host service and the object is tape, a
snapshot manifest or a record witness (bytes it cannot open). Absent and
forbidden give the same status, body and timing class. blobd logs every
read (caller, scope, ref, bytes) to a daily access log, an audit object
whose digest the restore design anchors (§5).
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
holds the key); a range read checks frame tags, or for public objects the
catalog's per-frame digests; a mismatch is an error, never a fallback to another copy
without verification (the `tapestore` rule, `format.go:13-17`). Replicas,
the S3 backend and a desktop cache are untrusted storage.

## 4. References and GC

**Refs.** Each holder records its refs in its own store, in the same
transaction as the holder. Guest refs reach the host two ways: the live
store's ref table (guest-closures provider) and the closure in each
retained snapshot's manifest header (snapshots provider). Invariant: every
head inside a computer's window has a retained snapshot at or before it
(restore design §4), and every retained snapshot's manifest, chunks and
closure are held. Only post-cutover snapshots exist as restore points: the
pre-cutover checkpoints are retired at the cutover (section 7). Version-1
file manifests and their chunks are deleted with the per-computer file CAS
(Deleted, not migrated). Version-2 roots are held like any other canonical
class.

**Snapshot closures.** A snapshot's closure is the guest ref table
(section 7 part 5) at the snapshot's head H: the image, capsule-evidence
and version-2 file-manifest refs the store names there, as `hmac:` refs.
The guest writes it into the manifest's plaintext header when it makes the
snapshot (restore design §2). No host process rebuilds a store any more, so
there are no closure rows, no host acceptance check and no verdict: every
retained snapshot holds its closure. Three rules bound what a header can
do. `CanonicalRef` takes tier and scope from the snapshot row (the
computer's owner and the computer), so a header can name only its own
owner's private objects, its own computer's internal objects and public
ones. Holds are per holder, so a guest that under-reports loses only its
own computer's holds; another computer's hold on the same object is a
separate row. A header that lacks the closure field or does not parse makes
the snapshots provider unable to run for that computer; an explicit empty
list is an empty closure. The check is in the guest: ref-table rows are
written in the same projection batch as the revision or evidence row they
belong to, so a replay of the tape reproduces them, and every restore and
weekly drill (restore design §7) compares the restored ref table with the
manifest's closure refs; a mismatch fails the drill and pages.

**Retained classes.** Tape segments, record witnesses and audit objects
are kept by rule, not by holders: no provider streams them, the audit
never judges them, `Purge` refuses them under the catalog lock, and they
leave the store only through `DeleteByAct`, which names its audit entry and
is refused in a scope under legal hold. Its uses are a closed list:
crypto-erase of an owner or a computer (section 3); a tenant's maximum
history (the segments wholly before the new first head, after a snapshot
at that head, restore design §4); the paper's archived segments, once the
analytical database has loaded them (restore design open question 14);
and, only if the restore design keeps just a window of a user computer's
tape on the host, the host segments wholly behind that window. No GC run
ever removes a segment.

**Legal holds.** `HoldScope` records a hold on a tier and scope (an owner
or a computer; a tenant is the set of its owners' and computers' scopes) in
the catalog's `scope_holds` table. While it stands, `Purge` and
`DeleteByAct` refuse every object in the scope, checked under the catalog
lock with the other eligibility conditions, and the snapshots provider
keeps every snapshot in the scope retained (and, for a head-range hold,
those in the range), so retention resumes normally when the hold is
released. Set and release are audited acts (restore design §4).

**Root providers, closed list.** Each provider streams canonical
(ref, provider, holder scope, holder key) tuples sorted by the merge-key
encoding (section 2) and stamps the stream with a host-assigned snapshot
time: the start of its read transaction on host databases (for snapshot
rows, the read that also applies the retention rule, so a retained
snapshot holds as of now, not as of its creation), and for live guest
closures the host's receipt time minus a skew margin, never a guest clock.
A snapshot time that is wrong in either direction fails loud (a page), not
silent. The driver also takes an expected count per (class, scope) from an
independent aggregate query over the holder's store (a `COUNT` written
separately from the extraction code, not a count of the emitted stream);
a stream whose length differs suspends that scope for the run. For guest
closures the count runs over the live store, one holder per holding object;
for snapshots, over the retained snapshot rows and the per-list counts each
manifest header declares.

| Provider | Source (holder key) | Classes |
|---|---|---|
| corpus, og | `og_objects.body_ref`, a published row's `image_refs` and a `choir.artifact_blob` row's metadata `storage_ref`, through `CanonicalRef`, with the row's own tier (published: public; else platform or private by owner) and scope as inputs; each `storage_ref` also gets one `og_lookup_keys` row for the by-ref query (og object ID) | og |
| corpus, captures | the three blob fields of `choir.web_capture` metadata JSON, not `body_ref`, which is empty for captures; reusing today's og query at `artifact_gc.go:447` would see none of them. For the by-ref query each blob field also gets a row in the `og_lookup_keys` side table (three rows per capture, one lookup kind per blob field, written in the row's transaction on every write of the row). Today's generic `writeLookupKey` deletes every lookup row for a canonical ID before inserting its one kind (`retrieval_schema.go:140-159`), which would drop these and the `image_refs` rows on any later write of a registered kind; its delete becomes scoped to the kind it writes, and the capture and og writers own their ref rows, not a generated column: on Dolt 2.1.9 a virtual generated column corrupts the upsert, a functional index fails it, and a stored column rewrites every `og_objects` row into history (`internal/platform/retrieval_schema.go:34-40` [V]) (capture object ID) | capture |
| maild | per-owner `email_attachments.blob_ref` (section 7 part 4), and `email_drafts.attachments_json` resolved through the attachment IDs it names to those rows' `blob_ref`, `maild/attachments.go:24-56,215-275` (attachment or draft ID) | mail |
| control, releases | the route tables' current and rollback refs, `platformUpdateLiveDigests`, `artifact_gc.go:475` (route slot) | platform-update |
| control, computers | file roots, through their manifests to the chunks each names (the root row holds only `manifest_ref`, `file_cas.go:13-21`; chunks are inside the manifest, `artifact_gc.go:713-735`) (root ID) | file-chunk |
| guest closures | the live store's closure: image refs held by its Texture revisions and evidence objects (enumerated from their guest ref rows, section 7 part 5, not copied from pin files, which have no unpin), cited capsule evidence, version-2 file manifests (revision, evidence or manifest ID; one holder per holding object) | image, capsule, file-chunk |
| snapshots | every snapshot row the restore design's retention rule keeps (§4: every snapshot under 48 h old, the last of each day in the window, the newest at or before the window start, pinned restore points and self-development rollback targets) plus every snapshot under a legal hold. The snapshot row is the checkpoint row, `computer_checkpoints` (`platform/computer_events.go:131`), naming the manifest in place of today's JSON artifact. From each row: its `manifest_ref`, and from the manifest's plaintext header its chunk refs and closure refs (Snapshot closures, above); a row whose head differs from its header's is a provider error. A ref held only by a retained snapshot stays held (snapshot ID) | snapshot-manifest, snapshot-chunk, image, capsule, file-chunk |

A provider that fails, returns an unparseable ref, or is stale makes its
scopes **suspended** for that run: no hold in a suspended scope is judged
absent, and no object with a hold (released or not) in a suspended
scope, or itself in one, is purged. A computer whose newest live closure is
older than 12 h (half the 24 h upload grace) is stale. The audit report lists every
suspended scope and how long it has been suspended, so the exemption is
visible.

**Catalog.** blobd keeps a SQLite (WAL, `synchronous=FULL`) catalog with
two main tables and two small ones (`pending_tombstones`, below, and
`scope_holds`, Legal holds above). `objects`: ref, generation, tier, class (first put's),
media type, plaintext size,
cipher digest, per-frame digests (public objects), pack ID (a ULID, no
file extension) and offset, `put_at`, `last_touch` (latest Put, IfAbsent
hit or Touch), `last_seen` (latest stream sighting of any holder),
`abandoned_at` and its probe evidence (keep-set objects that never had a
hold, below), `purged_at`. `holds`: ref, provider, holder scope, holder key,
class (the holder's), `touched_at`, `first_seen`, `last_seen`, `released_at`,
`pending_release_at` and `pending_run` (the first of two agreeing runs,
below), `flag_suppressed_at` (an operator override, below; cleared by `Touch`,
`Release`, a probe that no longer finds the tuple, or a stream naming the
tuple), and for a
release or confirmation made by the driver, an operator or a migration
rollback, `decided_by` (`driver`, `operator`, `rollback`), `decided_at`
and the evidence (resolver, result, holder state version, the guard page,
or the journal entry); primary key the merge-key encoding of (ref, provider, holder scope, holder
key). `Touch(holder, refs)` inserts each
hold or refreshes `touched_at` and clears `released_at`, the pending
fields and `flag_suppressed_at`.
`Release(holder, refs)` sets `released_at` and clears `flag_suppressed_at`
on those holds only; another
holder's hold on the same object is untouched, so one holder's delete never
covers another holder's absence. A hold with `first_seen` null is
**provisional**: the holder touched but no stream has yet shown it
committed.

**An object's row survives its purge.** The purge transaction sets
`purged_at`, clears the pack location and frame digests, and keeps the
row's generation, so a later re-put writes generation + 1 and outranks the
tombstone in any replay order; no step resets a generation. The same
transaction deletes the object's hold rows and writes its tombstone body
(ref, generation, the deleted hold rows) to `pending_tombstones`, so the
list exists before the rows are gone. The `holds` table stays bounded by
live objects plus those in their hold period; `objects` keeps one small
row per purged ref, which also lets a stream that names a purged object
page as a lost object.

**Deletion intent is not deletion.** Two records outside the catalog. A
run's **deletion manifest** lists every ref the run will purge; it is
written and fsynced before the run's first purge (as `artifact_gc.go:794-825`
writes `intent` before the unlink [V]), stored as an audit object, and
named by digest in the run's audit entry (restore design §4); it is for
audit only and nothing replays it. A **tombstone** (ref, generation, the
hold rows deleted) is appended to the tombstone log and fsynced after the
purge transaction commits, then its `pending_tombstones` row is deleted;
only a committed purge writes one. On start, blobd appends every pending
row's tombstone and deletes the row, so a crash between the purge commit
and the tombstone leaves the bytes in place and the hold list on record,
never a deletion without one. A record's space becomes dead for compaction
only once its tombstone is durable (section 5, Compaction).

Both calls come **before** the holder's own commit, and a disagreement
resolves toward retention: a holder that touches and then fails its commit
leaves a provisional hold that the holder probe ends (below); a
holder that releases and then fails its delete is still streamed, and the
audit restores the hold (below). The holder never needs an atomic
transaction across its store and blobd.

There is no separate release journal. Hold changes commit in the catalog's
WAL with `synchronous=FULL`, and the catalog is the only state that needs
them. Off host, restic carries a consistent catalog snapshot and the
tombstone log nightly (section 6). Rebuild: restore the latest snapshot;
replay its pending tombstones, then the committed tombstones written after
it, never a deletion manifest: a tombstone marks its (ref, generation) purged and deletes the
hold rows it lists only when the catalog's object for that ref is at that
generation; keep for every ref the highest generation seen in the
snapshot, the tombstones and the records; adopt records from packs written after the snapshot (record
headers carry tier, scope, object ID, generation, class and plaintext
length), where a record of a higher generation than a tombstone wins over
it whatever the replay order, so a re-put after a purge survives; reconcile
pack and offset for every object against the packs and their indexes (a
record that moved by compaction is re-pointed, one that is gone and not
purged pages as a lost object); set
every `last_touch = now`; then run one audit in report-only mode: missing
holds are repaired from the streams, and unreleased holds absent from the
streams page. Releases made after the snapshot surface as such pages; the
holder probe re-derives the ones it can decide, and the rest are reviewed,
not auto-released. A
rebuild with no snapshot gets holds from the audit alone, and no keep-set
object is purged after it without an operator acknowledgement of the
report. Every path fails toward retention or a page.

When a second host shares one store, the catalog moves behind the same
interface to the corpus PostgreSQL instance.

**The GC driver** is a separate process with read access to the holders.
It produces the sorted streams and calls `Sight` and `Purge`; blobd holds
no database credentials.

**Holds, not counts.** Reference counts drift when a crash lands between a
holder's commit and the store's increment, and there is no atomic
transaction across PostgreSQL, guest SQLite, maild and blobd. A count also
cannot be repaired except by recomputing it from scratch, and a wrong count
says nothing about which holder is wrong. A hold is a named member of an
explicit set: `Touch` and `Release` are idempotent (twice is once), and
every member can be checked against its holder's stream. Drift is bounded
by the audit: a missing hold is repaired from the stream that shows it; an
extra hold (unreleased, holder absent) is either a holder that never
committed (provisional, ended by the holder probe) or a holder that
vanished without a release (for keep-set classes it pages and the holder
probe decides it; for the others the audit releases it under a threshold). Neither direction can accumulate
silently.

Protocol:

1. **Touch before ref commit.** A holder that commits a ref calls `Touch`
   (or a `Put` carrying the holder) first and commits only within the
   **ref window** (1 h) of a successful result; a slower holder touches
   again. This covers a new ref to an old object: a Texture revision
   pinning a 13-day-old screenshot, a citation of an old capsule artifact,
   a manifest naming old chunks (batched). A `Touch` of a purged or absent
   ref returns not-found; the holder re-puts the bytes if it has them, or
   fails the commit loudly. This replaces the serialization `artifactGCMu`
   gives today ("a pin of an old blob must not commit after the sweep has
   classified it unreachable", `artifact_gc.go:197-200`).
2. **Release before ref drop.** A holder that drops a ref (maild deleting a
   message, objectgraph replacing a `body_ref`, a capture tombstone, a
   guest deleting a revision) calls `Release` for that holder first.
   Required for keep-set classes; optional elsewhere, where the audit
   releases holds whose holder is gone. Publish is copy-only: it puts the
   body at the public tier with the published object's row as holder, then
   commits that row; the draft's row, object and hold are not touched. No
   write changes the tier of an existing og row, so a row's tier, and the
   tier of every ref it holds, is fixed for the row's life.
3. **Audit.** A run records its start `S`, streams every provider and
   merge-joins the streams with the `holds` table, both in merge-key order
   (section 2); memory is the merge cursors plus one batch. In every branch,
   a sighting sets the hold's `first_seen` if null and refreshes the hold's
   and the object's `last_seen`, and a sighting of a purged or absent object
   names a lost object, which pages. A sighting, a probe confirmation and
   a `Touch` also clear `pending_release_at` and `pending_run`. Then, for
   each tuple:
   - streamed, hold unreleased: nothing more;
   - streamed, hold released: if the stream's snapshot time is after
     `released_at`, the holder still holds the ref (its delete failed, or
     it re-referenced without a Touch): clear `released_at` and log a
     repair (the repair log, not a page, is the operator signal); if the
     snapshot predates the release, the hold stays released;
   - streamed, no hold: insert a confirmed hold and log a repair;
   - hold unreleased and confirmed, holder absent from a fresh stream of a
     non-suspended scope, `touched_at < snapshot - ref window`: for
     keep-set classes, page and run the **holder probe** (below) on each
     such hold, never an audit release, so a crash between an og edit's
     upsert and its `Release` of the old body ends in a probe release
     rather than freezing the scope; for other classes, this is an
     **audit release** (stream absence, no per-holder evidence), subject
     to the two-run rule and the batch guard;
   - hold provisional, never streamed, in a non-suspended scope,
     `touched_at` older than the upload grace: for every canonical class
     (keep-set, file chunks, snapshot chunks and manifests, images and
     capsule evidence),
     the driver runs the **holder probe** (below); the audit never deletes
     the hold row and never ends the hold on "never streamed" alone, which
     cannot be told apart from a provider that omits the row. Only
     derived class (platform updates) keeps the plain expiry: the
     hold row is deleted after the upload grace (logged). A writer whose
     ref commit fails still releases its own provisional holds when it can.

   **Holder probe.** A point lookup of one holder: the driver calls the
   provider's `HolderRefs(provider, scope, key)` (section 2). Each holder
   service implements it by reading that one holder through the read path
   its product readers use, on decoded content, never by scanning stored
   bytes: maild's attachment read for an attachment or draft, objectgraph
   `GetObject` for an og or capture row, the manifest fetch and
   `filecas.ParseManifest` decode for a file root (so the chunks a manifest
   nests are returned), the guest's ref rows for an image or capsule
   holder, the snapshot row and its manifest header for a snapshot. It
   applies the provider's holding rule (tombstone, superseded, the snapshot
   retention rule) to the holder's current state and returns full
   canonical tuples through `CanonicalRef`, with the tier and scope taken
   from that state. It is independent of the bulk stream: a different query,
   by key, sharing only `CanonicalRef` and the holding rule. For a guest
   holder it reads the live guest store over the bound-guest channel; a
   ref held only by a retained snapshot is the snapshot holder's, read from
   its manifest header, and a header without a closure field or that does
   not parse cannot run. A read whose
   snapshot predates the hold's `touched_at` is stale and cannot run.
   Outcomes, comparing the returned tuples with the hold's ref:
   - the tuple is present: the hold is confirmed (`first_seen` set if null,
     `last_seen` refreshed, `released_at` cleared, evidence recorded); if
     the stream did not name it, the provider is flagged as omitting it,
     which pages and suspends that provider's scope, unless the hold
     carries `flag_suppressed_at` (operator override, below), in which
     case it is listed in the audit report and not flagged;
   - the holder is absent, or present without the tuple: the holder does
     not hold it (the crashed in-place rewrite: `PutObject` externalizes the
     body before the upsert, `objectgraph_store.go:107,115-131`;
     `WriteCaptures` puts three bodies before `CreateWebCaptureIfAbsent`
     creates the row, `sourcegraph/captures.go:194-229` [V]). The driver
     calls `Release(holder)` itself, recorded in the catalog (`decided_by`,
     `decided_at`, probe evidence), which sets `released_at`; the hold row
     stays and the hold period runs (purge path (a)). This is a **probe
     release**: for non-keep-set classes it waits for the two-run rule,
     and it counts in the batch guard's page but is not discarded by it
     (Guards). A later stream sighting newer than the release restores the
     hold as above;
   - a returned ref has the hold's ID but a different tier or scope: not a
     decision; it pages and the hold stays;
   - the probe cannot run (`err`, holder service unreachable, guest down,
     stale read, a manifest header without a closure field or that does
     not parse, a ref `CanonicalRef` rejects): it
     pages and the hold stays; this is never read as "absent". Of the
     probe outcomes, only here may an operator decide instead, after their
     own check of the holder;
     the acknowledgement acts as the driver's `Release` and is recorded
     with the same fields (operator, time, what was checked).

   **Two agreeing runs.** A release of a non-keep-set canonical hold (file
   chunk, snapshot chunk or manifest, held image, cited capsule evidence), audit or
   probe, is two-phase. The first run that would release it records
   `pending_release_at = now`, `pending_run` (its run ID) and the evidence,
   and releases nothing. A later run, with a different run ID, that reaches
   the same decision on the same hold applies the release. Any sighting,
   probe confirmation or `Touch` in between clears the pending fields, so
   the next decision starts over. A run in which the scope is suspended
   judges nothing in the audit branches and leaves the pending fields as
   they are; the flag resolution pass below is exempt and may write
   pending fields, apply a second agreeing decision and clear flags in
   that scope. The batch guard counts decisions, not applied releases: an
   above-threshold run still writes the pending fields of every hold it
   would release, so it is a first agreeing run, and the second agreeing
   run applies whatever the batch size (Guards).

   **Flag resolution pass.** A provider flag clears for each flagged ref
   when a later stream names it or a probe-verified release ends its hold;
   the scope resumes when no flagged ref is left. Because the suspension
   stops the audit branches above from probing that scope, every run ends
   with a pass that probes each flagged ref even inside the suspended
   scope; a probe that finds the holder does not hold the tuple clears its
   flag and releases it as above (for a non-keep-set class the pass writes
   the pending fields, or, if an earlier run wrote them, applies the
   release). A probe that keeps confirming a tuple
   the stream keeps omitting is a persistent resolver/provider
   disagreement: it pages each run, and an operator, after checking the
   holder, records an override (confirm the hold and clear the flag, the
   provider defect tracked as a bug; or release the hold), with the same
   recorded fields as an operator decision. A confirm also sets the hold's
   `flag_suppressed_at`, so later runs do not re-flag that tuple for the
   same omission; the suppression ends when a stream names the tuple or
   the hold changes (`Touch`, `Release`, or a probe that no longer finds
   the tuple), and every audit report lists the suppressed holds.

   **Never-held keep-set objects** (a `Put` without a holder, or a crash
   before the `Touch`): there is no holder key to look up, so after the
   upload grace each provider of the object's class runs a by-ref query
   over its designated ref fields (section 2), and each holder it returns
   is checked by `HolderRefs`. The query is an index lookup: `body_ref`
   for og, and for the `image_refs` metadata list and an artifact blob's
   `storage_ref` one `og_lookup_keys` row per ref; for captures, whose refs live in metadata JSON that
   today is matched only by a JSON predicate
   (`objectgraph_store.go:447-450,510-540` [V]), the three `og_lookup_keys`
   rows per capture (provider table); `blob_ref` for mail;
   the guest ref table in the live store and each retained snapshot's
   manifest header, for images and capsule evidence; for file chunks, a
   decode of the scope's retained manifests. A ref in a non-designated field is not found, by contract
   (section 2). Only when the class has at least one provider, all of them
   ran, and none found a holder is the object marked abandoned (`abandoned_at`,
   evidence per provider, logged) and purged after the keep-set hold
   period (path (c)). A provider that cannot run blocks `abandoned_at`
   and pages; nothing is abandoned on a failed lookup. A guest provider
   that cannot run because its computer is permanently gone blocks path
   (c) only until the owner deletes that computer: then an operator may
   record a decision for that (provider, computer), with the deletion
   record as evidence, after which that provider counts as ran and found
   none; never before the owner's deletion. Found: a confirmed
   hold is inserted for that holder, and its provider is flagged as
   above.

   **Pages aggregate** per (provider, scope) per run: one page carries the
   counts by cause (absent confirmed holder, provider omission, audit
   release over the batch threshold, probe release, probe tuple mismatch,
   probe failure, lost object) and a sample of refs; the full list is in the
   audit report.
4. **Purge, after a hold period and a recheck.** An object is
   purge-eligible when it is not of a retained class, is not in a scope
   under legal hold, has no unreleased hold, no hold in a suspended
   scope, is absent from a fresh audit, and one of: (a) it had holds and
   `now - max(latest released_at, last_seen) >= hold period`; (b) a
   non-keep-set object that never had a hold, and `now - last_touch >=` the
   upload grace; (c) a never-held keep-set object, re-probed with no holder
   found, and `now - abandoned_at >=` the keep-set hold period. Hold period:
   24 h for scratch classes and for snapshot chunks and manifests, which are
   accelerators checked against the tape and whose retention rule already
   keeps a restore point at the window start (resource use leads, owner
   2026-10-22); 7 days for every other class.
   Upload grace: 24 h (more than the ref window), 14 days for images and
   capsule artifacts, which is their unheld retention. A provisional hold
   on a canonical class never makes its object eligible except through a
   `Release` (the writer's, or the driver's on probe evidence) and then
   path (a). Before purging, the driver runs a fresh
   audit; `Purge` then re-reads, under the catalog write lock, each
   object's holds, `last_touch`, `last_seen`, class and scope holds, and
   purges only objects still eligible with `last_touch < S` and
   `last_seen < S` of that fresh audit. This is today's `unlinkIfStillUnreachable` recheck
   (`artifact_gc.go:808,831-850`) carried over. The run's deletion
   manifest is durable before its first purge (audit only); each purge
   keeps the object row, deletes its hold rows and writes its pending
   tombstone in one catalog transaction; then its tombstone is made
   durable, and only then is the record's pack space dead for compaction
   (Catalog, above).

A sighting refreshes `last_seen`, so an object a flapping provider missed
once is simply not eligible next run; there is no condemned state to undo.

**Guards.** Today's per-namespace live-set guard exists because a wrong
root provider once mass-deleted live og bodies (2026-10-05,
`artifact_gc.go:443-446`, commit 2fa30b179).

- Keep-set classes (capture, og, mail, and platform-tier objects): **zero
  tolerance**. A confirmed, unreleased hold whose holder is absent from a
  fresh stream is never audit-released: it pages and is probed. A keep-set object
  leaves the store only after every holder that ever held it, confirmed or
  provisional, has released it, followed by the 7-day hold period; a
  never-held one leaves only through a probe that finds no holder (path
  (c)). There is no silent per-run allowance: an unconfirmed keep-set hold
  past the upload grace, and a confirmed one absent from a fresh stream,
  is probed every run, and is released only on probe evidence that its
  holder does not hold the tuple, on a recorded operator decision when
  the probe cannot run or after a persistent disagreement, or by a
  migration rollback (section 7).
- **Batch guard**, every canonical class, per (class, scope) per run, at
  the larger of 1% and 100 holds. It counts release decisions, not
  applied releases, and above the threshold it pages with their count and
  a sample of the evidence. Audit releases (stream absence without
  per-holder evidence, non-keep-set classes only) above the threshold
  are not applied in that run when they are first decisions: the run
  writes `pending_release_at` and `pending_run` on each such hold, which
  makes it the first agreeing run (step 3). A hold whose pending fields an
  earlier run wrote is released by the second agreeing run whatever the
  batch size, and that run pages with the count. An operator, after
  reviewing a guard page's batch, may record an acknowledgement
  (`decided_by=operator`, the page and what was checked) that applies
  that batch's pending audit releases at once, without waiting for the
  second run. Probe releases are per-holder evidence and are never held
  back by the batch guard: above the threshold the run pages with their
  count and a sample of the evidence, and the releases apply (two-phase
  for non-keep-set classes). Every release still serves the hold period
  (7 days for keep-set classes), during which any sighting
  restores it. Keep-set classes are never audit-released (above).
- Other canonical classes (file chunks, snapshot chunks and manifests,
  held images, cited capsule evidence): an audit release or a probe release needs two
  agreeing runs, recorded in the hold's pending fields (step 3), so one
  bad provider or resolver run cannot empty a small scope.
- Per-run caps carried over from today (`artifact_gc.go:38-50,283,323-328`;
  Node B: 50,000 deletes, 32 GiB).
- **Activation.** Before the cut, every rehearsal of the cutover runs the
  GC in dry run over the rehearsal store, and its per-class counts must
  match the import receipts (ref coverage 100% for keep-set classes, zero
  repairs and zero pages on the first audit after import). At the cutover
  every class's GC runs from day one in the one store, except automatic
  deletion of keep-set objects: it starts report-only (a setting of the
  same store: audits, probes and releases run, and `Purge` of a keep-set
  object is reported, not executed), turned on ring by ring, proof ring
  first, after its reports match reality.

**What it replaces.** The per-namespace walker over `sha256/<ns>/` with mtime
as liveness and `casReuseMu` with `Chtimes` to protect reuse
(`objectgraph_store.go:56-63`, `artifact_gc.go:106-125,381-430`), the
unswept image pins (`imageblob/pin.go:13-20`) and the absent capture GC.
mtime is not a liveness signal on packs or S3.

## 5. Physical layout

**Packs.** Objects under 16 MiB append to the open pack of their scope (one
open pack per public class, per platform class, per owner, per computer;
retained classes pack apart from the other classes of their scope, so
compaction of churning snapshot packs never rewrites them). A pack seals
at 64 MiB (the tape's seal, `tapestore/format.go:106`) and at no other
time: there is no idle or age seal, because the nightly backup copies an
open pack as a verified prefix (section 6) and a sealed small pack would
need merging to keep the file count down. The S3 step (section 6) adds
whatever age rule a remote backend needs. Objects of 16 MiB or more
(large media) are one extent each, framed the same way. The 154,649 capture-body files
and 21,862 og files become a few hundred files (R0g-09); the 7.4 M-inode
legacy pin tree is the lesson that one file per object costs inodes,
directory scans and backup time far beyond its bytes (inventory §1).

**Write discipline.** A new pack or standalone extent is created
exclusively, its header written, the file fsynced, then its parent
directory fsynced, and every directory newly created up the tree, before
any acknowledgement that depends on it, as tapestore does
(`startSegment` and `syncTree`, `internal/tapestore/appender.go:508-540`
[V]: "nothing is acknowledged on top of a directory entry that may not be
durable"). Then records append to the open pack with a header CRC and the
cipher digest. Puts are group-committed: up to 5 ms or 8 MiB, then one
`fdatasync` of the pack, then one catalog transaction, then the
acknowledgements. A standalone extent is written, fsynced and its
directory entry made durable before its catalog commit. The catalog names a pack by ULID; the reader resolves
`<ulid>.pack`, then `<ulid>.open`. Sealing happens only after the pack's
last record is catalogued: write the index (sorted refs, offsets), fsync
it, fsync the pack, rename `*.open` to `*.pack`, fsync the directory.

**Tape.** The tapestore appender and reader move from corpusd
(`platform/tape_segments_write.go:87`) into blobd unchanged in format and
write discipline; corpusd's pin path calls blobd to append. This is the
host's copy of each tape; whether the guest's volume also keeps the whole
tape is the restore design's to settle (section 9 Q8). Each
computer's segments are extents of its internal scope, named by (computer,
segment number), sealed at 64 MiB or 50,000 records
(`tapestore/format.go:106-107`) and at no other time; an open segment is
backed up as a prefix like an open pack (section 6). From the cut every
pin of every kind goes to the segment, not only projection batches
(today's rule, `tape_segments_write.go:17-26`), so there is no legacy
one-file pin path; the segment also carries each event's head receipt
(restore design §4). Segments are not catalogued objects and not refs:
the reader resolves a digest through the segment indexes as tapestore does
today, and verifies every record it returns against its digest
(`format.go:13-17`). For the paper, a sealed segment whose events are all
older than the 7-day hot window and whose record witnesses are written is
renamed into the computer's archive prefix, bytes unchanged (restore
design §4); archived segments stay readable, so a replay from an older
retained snapshot still reads them.

**Record witnesses.** For a windowed computer (the paper), corpusd
assembles one bundle per record-carrying event (authority
`authority:vm-local-projection-record`, restore design §4): the envelope,
the guest-sealed payload, the pin receipts, the head receipt and the
inclusion proof of the first anchor covering the head. It is written after
that anchor and before the event's segment is archived, named `sha256:` of
the bundle, stored as written in the paper's internal scope, and indexed
by event in the control database. The paper's pre-cutover bundles are
re-sealed by its guest at the cut (section 7 part 6).

**Recovery.** On start, blobd scans the open pack past the last catalogued
record (records whose CRC and digest verify are adopted; a torn tail is
truncated) and, after an unclean shutdown, compares every pack sealed since
the last clean checkpoint of the catalog against its index and records,
adopting any record the catalog lacks. The tape writer recovers its
segments as tapestore does (`sealUnowned`, `appender.go:407-504`). Each
crash point (after create, after directory fsync, after append, after
fsync, after catalog commit, after the purge transaction, after
tombstone, after index write, after rename) gets a process-kill test
and a power-loss test (fault injection that drops unsynced file and
directory writes, e.g. a dm-flakey or `LD_PRELOAD` fsync-drop harness)
before the cutover's first rehearsal.

**Compaction.** A record is dead when its (ref, generation) has a
durable tombstone (none pending), or when the catalog's object for its ref
is at a higher generation (a repair or a re-put superseded it); nothing
else (no hold state, no age, no deletion manifest) makes it dead. A sealed
pack whose dead fraction exceeds 40% is rewritten: every record the
catalog names as current, and every purged record whose tombstone is
still pending, is copied to a new pack, and the catalog is switched in one
transaction. Compaction holds the catalog's GC lock, which `Purge` also takes, for the
whole rewrite, so the not-purged set of the pack cannot change during the
copy (a sealed pack gains no records, and a re-put of a purged ref writes
a new record to an open pack; puts and touches do not need the GC lock).
The switch runs under the catalog write lock, re-reads that set and aborts
if it differs. The old pack is deleted after a reader grace (10 min local; 24 h and a tombstone
on S3, where an in-flight range read of a deleted object fails rather than
finishing), and never while a backup run is between its catalog snapshot
and its verification: deletions of extents queue until it ends (section
6). Readers re-resolve through the catalog on not-found. Write
amplification is at most 1/(1 - 0.4) = 1.7x of what is deleted. Compaction
needs free space of one pack, not of the store (contrast the Dolt
compaction that grew 64.7 GB to 89.2 GB, paper-disk §A1).

**btrfs assumptions dropped.** The store uses write, fsync, rename, pread
and unlink. On Linux (ext4, xfs, btrfs) these are used as is; APFS and
NTFS on the desktop apps get a platform layer with its own crash tests,
since their fsync and rename semantics differ. No reflink, `FICLONE`,
subvolume snapshot, sparse-file or mtime semantics. Reflink stays a
compute-host-local optimization of vmctl and is never a property a blob
relies on.

## 6. Backend choice

**Now: the local pack backend**, on Node B and on a self-hosted single box,
and as the guest's local cache and the desktop's store. **Later: an
S3-compatible backend, SeaweedFS by default** (Apache-2.0, single-node mode,
small per-object index; 1 GB volumes because vacuum needs a volume's worth
of free space; we run repair ourselves). The research is one day old; MinIO
(archived), Garage (metadata limits, no conditional writes) and Ceph (size)
stay rejected without a re-check. Sealed packs map to S3 objects one to
one, so the bucket holds thousands of objects, not millions, and range GETs
serve records. Open packs and open tape segments stay host-local until
they seal, which for a quiet scope can take months; the S3 step must add
an age seal and a merge of small sealed packs, which this cut does not
need.

What S3-compatible buys: off-host and multi-host durability, extents
readable from a second host or a desktop, and erasure coding or replication
we do not write. What it does not buy: encryption, identity, refs, tenancy,
verification and GC, which stay in the Choir layer.

**Off-host copies before SeaweedFS (Q4, accepted):** a nightly restic run
(1) captures the holders' databases first: the control database, the
corpus database and maild's database (today's script already captures
Store A, the corpus store and `mail.db`, `scripts/off-host-backup.sh:195-245`
[V]). They come before the catalog because a holder touches a ref before
it commits it and a released object stays at least its hold period, so
every ref a captured holder names is in the later catalog snapshot. Guest
stores need no copy of their own: they are rebuilt from the tape and
snapshots in this store. (2) Takes a consistent catalog snapshot (objects,
holds, pending tombstones, scope holds; SQLite's backup API). (3) Writes an
**extent manifest** bound to the snapshot: every sealed pack, standalone
extent and sealed tape segment by ID with the SHA-256 of its file and
index; every open pack and open tape segment as a prefix, its length at
the snapshot (the end of its last catalogued record, or the segment's last
acknowledged append) and the SHA-256 of that prefix; and the snapshot's
own digest. Packs and segments are append-only, so a copy read later
contains the prefix. (4) Uploads the extents not yet uploaded or grown
since, the snapshot, the manifest, the tombstone log and the holders'
captures; the wrapped key material (interim owner KEKs and escrow rows,
platform KEK wrap) goes to a separate repository with its own secret
(restore design §9, keys and data split), so the private and platform
tiers are recoverable from the two copies. (5) Verifies from the
repositories that every manifest entry is present with its digest,
prefixes included, and only then reports success. From step 2 to step 5
no extent is deleted (compaction's old packs and `DeleteByAct` removals
queue). A restore from the copy truncates each open extent at its
manifest length, then rebuilds the catalog as in section 4. A run that cannot verify reports failure and pages;
it never reports partial success. Loss bound on host loss, stated
honestly: every put acknowledged after the last successful run's snapshot,
at most 24 h while runs succeed, plus one more day for each failed run
(each paged), until a second host.

## 7. Migration: one cutover, keep set first

**One cutover (owner ruling, 2026-10-30).** Every class moves in one
cutover under one writer drain: captures, og (publication and proposal
text included), mail, guest file CAS, the tape, snapshots, images, capsule
evidence and platform update payloads, so that no namespace walker or
legacy reader survives the cut. This is also the restore design's cut
(its §9): there is one drain, not two. The parts below are the order of work inside
that one drain, not separately shipped steps. Before the cut the whole
cutover, its rollback included, is rehearsed end to end many times on the
proof ring (d@e) and on a copy of Node B's data, each rehearsal producing
import receipts, dry-run GC counts (section 4, Activation) and timings;
the cut runs once those match. Red, tier 2.

Downtime is acceptable now (owner, 2026-10-29). The **writer drain** covers
every writer, not a rejection alone: a 503 stops new requests but not one
already inside `sourcegraph.WriteCaptures`, which writes the three bodies
and then the row synchronously (`internal/platform/captures.go:201`,
`internal/sourcegraph/captures.go:194-207` [V]). The drain: (1) a write
gate in corpusd covers every corpusd blob writer, HTTP handlers and
background jobs alike, each counted in flight: capture bodies (`PutBody`),
og bodies (`externalizeBody`, `objectgraph_store.go:44`), publication and
proposal text (`writeBlob`, `service.go:411-412,790-791`), platform-update
payloads (the offer mint, `platform_update.go:125`, and the blob PUT,
`platform_update.go:306`, routed at `handlers.go:616`), checkpoint
artifacts (`checkpoints.go:182-183`), tape pins and segments
(`event_artifacts.go:146,379`) and file CAS (`file_cas_blobs.go:31-83`)
[V]; the capture endpoint returns 503, the paper's publishing pauses, and
the release pipeline is paused so no platform update is minted or staged
inside the drain. maild's gate stops attachment staging. (2) Every
computer is stopped as for a planned update; checkpointd and the legacy GC
are stopped. (3) The import waits until each
gate's in-flight count, exposed on its service's health endpoint, is zero,
which quiesces body and row writes together, or the service is stopped
instead; (4) only then the import scan, the referenced-set comparisons and
the switch of every reader and writer; writes resume after the switch, and
computers boot onto the cutover release, which writes only the new store.
The import writes one confirmed hold per (ref, provider, holder scope,
holder key) tuple the providers stream inside the drain. The legacy
directories stay frozen (no writer, no reader, rollback material only)
until the comparison and a week of clean operation pass, and the legacy
tape, bases, file CAS and pin tree until the deletion order below; any
`*.txt` no row names stays frozen where it is until open question 6 is
answered, whatever the week.

**Migration writes.** Every rewrite of an existing holder row's ref fields
in parts 2 and 3 goes through one objectgraph migration write, not the
normal write path: before the switch, `PutObject` re-externalizes a body
of 1 KiB or more to `sha256/og/<hex>.bin` and overwrites `body_ref`
(`objectgraph_store.go:44-49,107`), and `WriteCaptures` puts its bodies
through the legacy `PutBody` before `CreateWebCaptureIfAbsent`, which only
creates (`sourcegraph/captures.go:194-229`) [V]. The migration write takes
an existing row and the canonical refs for its designated fields, and in
the row's own transaction (`execObjectWrite`, `objectgraph_store.go:268`,
which also writes its lookup rows) sets those fields, recomputes
`content_hash`, writes the row's `og_lookup_keys` ref rows, re-derives its
derived keys, and writes the migration journal entry. It never calls
`externalizeBody` or `PutBody`, and it refuses to run unless the drain's
write gate is closed.

1. **Build blobd (with the tape writer moved in), the GC driver and
   `internal/objstore` with the local backend**, with every crash and
   power-loss test of section 5 passing, and the restore design's
   before-the-cut items on d@e (its §9: envelope v2, the guest snapshot
   producer, the record-batch tag); rehearse the full cutover (above).
2. **Capture bodies.** The Autopaper's 150,811 referenced bodies into the
   platform tier under unchanged `sha256:` IDs (no row rewrite). The other
   owners' ~3.8 K into the private tier under their interim keys, each its
   own `hmac:` object, the 604 bodies that equal a platform body included:
   no private row references a platform object (section 9 Q5), so the
   owner-readable `content_blob_id` (`internal/platform/captures.go:289`
   [V]) never tells an owner whether the paper holds the same bytes. Their
   1,840 capture rows are rewritten by the migration write (above): new
   `hmac:` blob fields and a recomputed `content_hash`
   (`objectgraph/object.go:97`, which covers metadata), so derived keys see
   the change and re-derive (`objectgraph/derived_keys.go:490`). The
   canonical ID does not change (captures use an identity key,
   `sourcegraph/captures.go:212`), and anchors check `DocHash`, not blob
   IDs (`publication_versions.go:508`). Each rewritten row's prior metadata
   and `content_hash` go to a migration journal in the same transaction.
   The import also writes the three `og_lookup_keys` rows of every capture
   row, the paper's included (about 197 K rows, an insert into a side
   table, not a rewrite of `og_objects`), and the capture writer writes
   them in the row's transaction from then on.
   Then verify every referenced ref resolves and hashes, and switch
   `PutBody`, `Body` and every reader listed in section 3; the 36 orphans
   stay in the frozen old directory until the week passes.
3. **og blobs.** Import the 21,848 referenced bodies by the tier
   of their holding object, passed to the `Put` and to `CanonicalRef`; a
   body referenced by holders at two tiers becomes two objects, one per
   tier. `body_ref` is not rewritten for public and platform holders. A
   private og row needs an `hmac:` ID (the og class rule, section 2), so
   each private og row's `body_ref` is rewritten to `hmac:<hex>` by the
   migration write, and the row's prior `body_ref`, metadata and
   `content_hash` go to the migration journal in the same transaction. The
   dry run counts these rows first (none expected [H]); the rewrite runs
   only if the count matches the dry run, and the old resolver
   (`objectgraph_store.go:49,90`, path form only) is restored with the
   journal on rollback.
   The 14 orphans stay in the old directory with it. Publication and
   proposal text: every `sha256/<hex>.txt` and `sha256/proposals/<hex>.txt`
   named by a `choir.artifact_blob` row's `storage_ref` is imported the
   same way, at the row's tier (published text public; a proposal's,
   platform for the paper and private for other owners, whose rows get an
   `hmac:` `storage_ref` by the migration write), with one
   `og_lookup_keys` row per artifact blob. The dry run counts the rows,
   the files they name and the files no row names. Then `PublishTexture`
   and `SubmitPublicationProposal` write through the store, and the
   readers of `storage_ref` (`service_publication_read.go:199`,
   `publication_pieces.go:192`, `publication_front.go:476`) read from it.
   Files no row names are **not** touched: open question 6. Fix the
   `nix/node-b.nix:1298` comment, which says og is outside the active GC
   namespaces while `:1307` includes it.
4. **Mail.** Outbound attachments (10 files) into the private tier; inbound
   bodies are rows in `mail.db`, not blobs, and stay there until mail moves
   into the computer. Each
   per-owner `email_attachments` table gains a `blob_ref` column holding
   the canonical `hmac:` ref; the `sha256` column (`maild/attachments.go:133,236`,
   `maild/store.go:829`), an owner-facing API value, is unchanged.
   `StageOutboundAttachment` puts the bytes with the attachment row as
   holder, then inserts the row with `blob_ref` in the same insert. Drafts
   keep `attachments_json` and its `version_hash` unchanged; the draft
   holder resolves through the attachment IDs it names. The import fills
   `blob_ref` only for the existing rows with stored bytes (outbound),
   one transaction per owner database, writing each row's prior
   `blob_ref` (null) to a migration journal in that transaction. Inbound
   attachment rows are inserted with `storage_ref` NULL
   (`maild/ingest.go:102-112` [V]) and keep `blob_ref` NULL; a NULL
   `blob_ref` is no holder, and neither the provider nor its expected
   count includes it.
5. **Guest Textures, images and capsule evidence; retire the pre-cutover
   checkpoints.** The paper's Texture documents and revisions are rows,
   not blobs: they move with the guest engine (SQLite) migration, read
   offline from a reflink copy (keep-set §8.1). The owner's three
   computers' Textures are nice-to-keep (keep-set owner note): copied if the
   migration makes it cheap. Today's checkpoints are reconstruction
   records, not stored stores [V]: publication writes a JSON artifact and a
   row (`platform/checkpoints.go:182-201`), and the bind installs no base
   and replays no tape (`agentcore/checkpoint_restore_bindings.go:45`), so no
   pre-cutover checkpoint has a ref table to read or fill. The cutover
   **retires the pre-cutover checkpoint set and its restore history**
   (owner-accepted, section 9 Q1 and Deleted, not migrated): they are
   marked retired, are no restore points and no roots, and their bytes go
   by the deletion order below. There is no sidecar and no fill of an old
   checkpoint. The import puts into the owner's private tier every image
   and cited capsule-evidence object held by a revision or evidence row in
   the live store, read from the guest image directory. Revision and
   evidence content keeps its `artifact:sha256:<hex>` strings
   (`imageblob.go:91`): rewriting them would change canonical revision
   content and its hashes. Instead the guest store gains a ref table, one
   row per (holder kind, holder ID, content key) with the canonical
   `hmac:` ref; it is written in the same guest transaction and the same
   projection batch as the revision or evidence row that embeds the image,
   so a replay of the tape reproduces it, and the image reader
   maps a content key to its object through it. The import fills it in the
   live store; a key with no imported object fails that computer's import
   (no row is ever written for a missing object) and the cut does not
   proceed until the key is accounted for. Each inserted row (prior value:
   none) goes to a migration journal in the same transaction. The fill is
   part of the engine migration at the drained head H0, before the guest
   takes its first snapshot S0 (part 6), so S0's closure and S0's store
   agree; a fill after S0 would give the first restore point a closure its
   own store contradicts. The old image state directory is frozen with the
   others.
6. **Tape, snapshots, record witnesses, file CAS and platform updates.**
   Every computer's tape, the paper's included, continues from its drained
   head H0 in blobd's tape writer (section 5, Tape), sealed by the guest
   from the cut (envelope v2, restore design §3.1). The pre-cutover tape is
   not imported: its H0 receipts and the first anchor's leaves witness it
   (restore design §9), and its bytes go by the deletion order. The first
   post-cutover restore point of each computer is the snapshot S0 its guest
   makes at H0 at its cutover boot, before its first post-cutover append
   (restore design §9): its chunks and manifest are put with the snapshot
   as holder, and the snapshot row is recorded. No host base is captured;
   `projectionbase.PublishCapturedBase`, which needs the host tape head and
   a reflink of the guest image (`projectionbase/capture.go:22,44-46,67-69`), is
   deleted with projection bases and checkpointd. The paper's guest
   extracts its pre-cutover record batches from its plaintext tape,
   re-seals them and puts them as record witnesses (restore design §9 and
   open question 4), verified against their receipts before any deletion.
   A checkpoint is now its snapshot: the checkpoint row names the manifest,
   and today's separate JSON artifact is not written, since the row already
   carries `checkpoint_json` (`checkpoints.go:201`) and nothing reads the
   file back (grep for `computer-checkpoints` finds only the writer, `:182`
   [V]). File CAS restarts as version 2 under keyed IDs: each guest's first sync
   after its cutover boot uploads its file roots from its own disk
   (within-owner dedup), and the public seed tier holds shared system
   files, written once. Platform update payloads (1.4 GB) are imported into
   the public tier, held by the route slots that name them. From the cut, every new
   write of every class goes only to the store: guest image blobs,
   screenshots and capsule evidence each with its guest ref row (part 5).

**Rollback before writes resume: one act, host and guests together.** A
data-only rollback cannot run: the cutover release's host accepts only
version-2 manifests (as today's `VerifyRoot` accepts only version 1,
`filecas.go:24,102` [V]), its guests build only version 2 and envelope v2,
and the old release reads neither. So the act
is a release rollback under the same writer drain as the import (gates
closed, in-flight counts zero, computers stopped, until the resolvers
point back, so no writer can commit a new-form ref in between). Inside it:
every host service is redeployed at the pre-cutover release; the resolver
config points back at the frozen directories (never a silent fallback);
the migration journals are replayed backward, restoring each row's prior
values (capture metadata and `content_hash`; og `body_ref` and
`storage_ref`, metadata and `content_hash`; mail `blob_ref`; guest ref
rows removed) only where the row still matches its migrated value; and
each computer boots the pre-cutover release on its pre-cutover volume,
which the engine migration leaves untouched (it reads a reflink copy,
keep-set §8.1). For each ref the replay removes from a holder, it releases
the hold the import wrote for that (ref, holder), recorded with
`decided_by=rollback` and the journal entry as evidence, so the next audit
finds no confirmed hold whose holder no longer names it and no batch of
probe releases follows. Every rehearsal runs this rollback.

**After writes resume there is no one-act rollback.** Post-cut bodies,
refs, tape and snapshots exist only in new forms. The preferred path is a
forward fix. The emergency path runs under a drain and ends in the same
release rollback: an offline tool (not a live reader of any service)
exports post-switch bodies to the old layout and converts each post-switch
holder's ref fields back to the legacy form (capture blob fields, og
`body_ref` and `storage_ref` to their `sha256` forms, with `content_hash`
recomputed; mail `blob_ref` cleared; post-switch version-2 file roots
rewritten as version-1 manifests over chunks exported to the legacy
per-computer hex paths), journaling what it replaces and releasing those
holds with `decided_by=rollback`; each computer's post-cut tape and store
are converted by an in-guest decrypt into the old format, each an audited
`export` (restore design §9); the retired checkpoints are un-marked, which
is possible only while the deletion order below has not run. A row changed
since the switch is handled here, not by the journal replay. No holder is
left with an `hmac:`, `blob_ref` or ref-table ref after it. It is
rehearsed once on staging before the production cut.

**Deleted, not migrated** (outside the keep set, pre-launch): the
pre-cutover tape, legacy pin files 47.4 GB and 7.4 M inodes and segments
including the paper's 26.4 GB (their H0 receipts and anchor leaves stay,
and the paper's record witnesses are re-sealed first, part 6); the 61.6 GB
of projection bases, with `PublishCapturedBase` and checkpointd's
rebuilder, verdict pass and escrow unwrap (restore design §3); per-computer
file CAS 27.9 GB and `file-cas-roots` 1.0 GB; capsule artifacts; the
retired pre-cutover checkpoints (records and artifacts); the pre-cutover
guest volumes and their quarantines and the old restic repository (restore
design §9); and `computer-inputs` once vmctl no longer reads it. Blast
radius [I]: every computer, not only the owner's, loses its pre-cutover
restore history and its file CAS; restore points start at S0. Order (Q1,
owner-accepted with these conditions):

1. the cutover has switched every reader;
2. **export receipts with counts**: the paper's Texture documents and
   revisions, published set reconciled against Store B (3,443 documents,
   11,870 versions at least), and the owner's mail (254 messages, 32
   attachments); the owner's three computers' Textures and files are
   copied where cheap, with counts recorded either way;
3. the guest-engine migration of the paper's store has landed;
4. **a restore drill of the keep set** from the new store and the new
   engines, without the legacy tape or bases, compared by count and hash
   against the receipts in 2;
5. **sealed-restore drills pass with the old plaintext moved out of
   reach** (restore design §9): a user computer restored to an arbitrary
   head inside a fresh window, a broken-VM restore on d@e, and a paper
   restore from S0;
6. **the paper's record witnesses are verified** against their receipts.

The paper's pre-cutover guest volume on the host stays until steps 4-6
pass.

## 8. Precommitted expectations

Round 0g (R0g-01..07) was committed to
`docs/definitions/choir-data-architecture-selection-2026-10-28.precommit.md`
by FP0 on 2026-10-29 about 23:37 UTC (commit 3294bce10). Revision 1 said "not
committed by this note, so not yet records"; that stopped being true at
that commit. The ledger's records are never edited. The original text is
kept below; corrections are new records (rounds 0h, 0i, 0j and 0l), each naming
the record it corrects and why. Settler: the rule, reading the named artifact.

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

### Round 0i, after the hold model (new records)

Revision 5 replaces condemn with a hold audit, so R0g-10's "mark, condemn
and purge-time re-mark" no longer names the mechanism, and the hold table
is new state with its own checkable count. R0g-10 stays and is scored as
committed.

**R0g-14** (corrects R0g-10: the GC driver now merge-joins (ref, holder)
streams against the `holds` table, with no condemn phase). Peak RSS of the
GC driver for one full audit and one purge-time re-audit over 1 M objects,
their holds and the six providers: est 60 MiB [25, 150]. Falsifier: over
256 MiB, which would mean the merge is not streaming.

**R0g-15** (new). Hold rows after steps 2-3, written by the import from the
provider streams inside the drain: est 220 K [175 K, 320 K] (about three
blob fields per capture row over ~65.7 K capture rows, less shared
fields, plus one per og row). And the first audit after the import repairs
zero holds and pages zero: p = 0.85. Falsifier: hold rows under 150 K or
over 400 K, or any repair or page on that first audit. Artifact: the
catalog's `holds` count and the audit report.

Revision 7 note, no new record: R0g-14 still predicts what it says. The
merge key widens from (ref, holder) to the 4-tuple (ref, provider, holder
scope, holder key), but the merge's memory is the cursors plus one fixed
batch, independent of the object count; a wider key adds a few hundred
bytes per buffered row, a few MiB at most, and holder probes are point
queries run one at a time. The estimate and interval stand.

Revision 8 note, no new record. The `IfAbsent` extent verification reads a
hit's whole record to check its cipher digest. In the step 2-3 import the
sources are content-addressed directories, one file per ID, so almost every
put is a first put and hits only occur for bytes already written once in
the run; the extra read is at most the bytes of those duplicates, small
next to the 9.85 GB already hashed on read and on write. R0g-03 stands.
R0g-04's put latency is for capture-size bodies, whose hit reads are one
record. R0g-14 stands: resolver lookups are point queries, one at a time.
R0g-15 stands: hold rows of purged objects are deleted, which only lowers
later counts, and the import purges nothing.

Revision 9 note, no new record. The generated capture columns are on the
corpus table, not in the store, so R0g-01, R0g-08 and R0g-09 do not move;
by-ref queries and the flag pass are point queries run one at a time, so
R0g-14 stands; pending fields add two columns per hold row, not rows, and
`image_refs` holds come only with publishing after step 3, so R0g-15's
count and its zero-page first audit stand.

Revision 10 note, no new record. The capture lookup is now about 197 K
`og_lookup_keys` rows on the corpus database in place of the revision-9
generated columns; that is corpus state, not store bytes or files, so
R0g-01, R0g-08 and R0g-09 do not move, and no committed record predicts
corpus size. Checkpoint-held images and capsule evidence are step-5
imports, outside the step 2-4 scope of R0g-01 and R0g-08 and the step 2-3
scope of R0g-09 and R0g-15. The guard change alters when releases apply,
not how many holds the import writes, so R0g-15 stands, and R0g-14 stands
(the guard counts what the merge already visits).

### Round 0j, after revision 12 (new records)

Revision 12 drops cross-owner dedup (section 9 Q5): the 604 bodies that
equal a platform body get private copies. R0g-08 named that cost (~0.04
GB, inside its interval) but estimated under the Q5 default, so its
estimate moves; R0g-08 stays and is scored as committed.

**R0g-16** (corrects R0g-08: no cross-owner dedup; every non-paper owner's
capture body is its own private object). Stored bytes of the capture, og
and mail parts of the cutover in the local pack store (headers, frames,
pack indexes, encryption overhead included): est 9.99 GB [9.89, 10.35].
Assumes 9.845 GB referenced plus ~0.04 GB of private copies of the 604
formerly shared bodies, plus ≤ 1% overhead. Falsifier: over 10.6 GB.
Artifact: `du` of the store plus the catalog's byte sum.

Revision 12 note, no other new record. The parts of section 7 are now one
cutover; "steps 2-4" and "steps 2-3" in committed records name the same
class scopes (capture, og, mail; capture, og), and "after step 6" names the
30 days after the cut, so no scope moves. R0g-15 stands: hold rows are per
(ref, holder) tuple, and the 604 holders hold their own private objects
instead of the platform one, one hold each either way. The import fills
packs to 64 MiB in minutes, so the 1 h open-pack age does not move R0g-09;
one directory fsync per new pack (about one per thousand 60 KB puts) sits
beyond R0g-04's p99. Retiring the pre-cutover checkpoints removes
checkpoint-held imports, which were outside every record's scope (revision
10 note). R0g-13 stands.

### Round 0l, after revision 13 (new records)

R0g-11 and R0g-12 were written when step 6 carried only user computers'
tape ("the paper leaves the tape"); every computer now keeps a tape, the
paper's included (restore design §4). R0g-11 stays and is scored as
committed. R0g-12's own void clause is met (the paper's tape is written
into the store), so it is void as stated; R0g-18 is the new record that
clause requires.

**R0g-17** (corrects R0g-11: its premise that step 6 adds only user
computers' tape is false, and revision 13 adds classes, snapshots and
record witnesses, that "keep-set classes" could be read to include; the
scope is now named class by class, and the estimate does not move).
Growth of the capture, og and mail classes in the store, by class name
(tape, snapshot chunks and manifests, record witnesses and audit objects
excluded), mean over days 1-30 after the cut at the then-current
Autopaper rate: est 0.6 GB/day [0.35, 1.1], over 90% of it platform-tier
captures. Assumes publication text adds under 0.01 GB/day. Falsifier:
over 1.5 GB/day. Artifact: daily catalog byte sums per class. Hard to
measure: capture volume follows the news; report captures per day with
it.

**R0g-18** (replaces R0g-12, by its void clause). Growth of the whole
store, every class plus the tape extents, mean over days 1-30 after the
cut: est 2.2 GB/day [1.2, 4.5]. Assumes capture, og and mail 0.6
(R0g-17); the paper's tape 0.47 (0.45 to 0.49 GB/day compressed, hot plus
archive, nothing removed; R0k-13); paper record witnesses 0.05; paper
snapshots filling to about 20 GB (R0k-05) plus one day of released chunks
in their 24 h hold, 0.7 averaged over 30 days; user snapshots filling to
about 9 GB (R0k-04), 0.3; user tape 0.02; the file CAS re-upload after
the cut about 2 GB once, 0.07; images, capsule evidence and platform
updates under 0.03. Falsifier: over 7 GB/day. Artifact: daily catalog byte
sums per class plus the byte sum of the tape extents. Hard to measure:
snapshot fill dominates days 1-8 and the file CAS re-upload day 1;
mitigation: report the 30-day mean, the per-class sums and the days 15-30
rate separately.

**R0g-19** (new: closures from manifest headers). On every GC audit in
days 1-14 after the cut, the snapshots provider runs for every computer
that has a retained snapshot (no scope suspended for a missing or
unparseable manifest header or a row and header head mismatch), and the
first audit after each computer's S0 repairs zero holds of the snapshots
and guest-closures providers: p = 0.8. Falsifier: any such suspension or
repair. Artifact: the audit reports. Hard to measure: a computer that
does not boot after the cut has no S0; list it separately, not as a
pass.

**R0g-20** (new: the guest closure check). In the first 4 weeks after the
cut, every weekly drill that reconstructs a store at a snapshot's head
finds the restored ref table equal to that manifest's closure refs: p =
0.85. Falsifier: any mismatch not traced to the drill harness. Artifact:
the drill records. Hard to measure: drills skipped for free space are not
passes (as R0k-09); report them separately.

**R0g-21** (new: backup with open-extent prefixes). Each of the first 14
nightly backup runs after the cut verifies every extent manifest entry,
open-pack and open-segment prefixes included, and no run fails for a
missing or changed extent: p = 0.85. Falsifier: any run that fails
verification for a cause in the store or the run (not the remote
repository's availability). Artifact: the run reports and their manifests.
Hard to measure: a failed upload for network reasons; count it, and say
which cause it was.

Revision 13 note, no other new record. R0g-14 stands with seven providers:
the merge's memory is one cursor per provider plus one batch. The import
of publication and proposal text adds at most one hold per
`choir.artifact_blob` row (about 12.5 K) and about 25 MB, inside R0g-15's
and R0g-16's intervals, and a few packs, inside R0g-09's. With no idle or
age seal the import writes no more files than before, so R0g-09 stands.
Snapshot and record-witness puts happen after the cut, outside the import
records' scopes.

## 9. Open questions for the owner

1. **Discard pre-launch tapes, legacy pins, bases and file CAS?**
   Accepted 2026-10-29, conditional on the export receipts and the keep-set
   restore drill in section 7. Covers every computer, including the owner's
   three accounts' computers (`5bd6de97`, `5885aafc`, `c061de6f`; the
   hibernated `candidate-fleet-bf04bbc7` included). Revision 12 applies it
   to the pre-cutover checkpoint set too: retired at the cutover, deleted
   in the same order (section 7). Revision 13 applies it to every
   pre-cutover tape on the restore design's defaults (its open questions 4
   and 5): H0 receipts and anchor leaves stay, the paper's record witnesses
   are re-sealed and kept, and any owner computer the owner names is
   re-sealed instead of retired.
2. **Keep the other 29 owners' 1,840 captures, as private objects?**
   Accepted.
3. **Interim keys until the vault's owner-KEK slice lands?** Accepted, as a
   per-owner interim KEK (section 3), not a deployment-wide wrap.
4. **Nightly restic of sealed packs before SeaweedFS?** Accepted, with the
   catalog snapshot and wrapped key material (section 6); revision 13 adds
   the holders' databases, open packs and segments as verified prefixes,
   and keys in a separate repository.
5. **Cross-owner dedup.** Answered 2026-10-30: dropped. Every non-paper
   owner's capture body is its own private `hmac:` object; no private row
   references a platform or public object, so neither the upload response
   nor the stored blob ID (`content_blob_id`, `captures.go:289`) reveals a
   match. Cost: ~0.04 GB today (R0g-16). A user's fetch is never published
   or promoted to a shared tier.
6. **`sha256/*.txt` files that no row names.** Revision 13 found that
   publication and proposal text is written there and named by a
   `choir.artifact_blob` row's `storage_ref`, which the public readers
   follow (section 1 og row [V]); the keep-set count checked only
   `body_ref`. Files a row names are published-version or proposal text
   and are imported with og (section 7 part 3). The question is now only
   for the files no row names, which the dry run counts [H: few]: import,
   keep in place, or delete? Kept frozen and untouched until answered.
7. **The Autopaper's captures are platform-tier, not public** (stated
   default, section 1). Override if raw captured bodies should be readable
   by every computer or published. Platform reads by the Autopaper's
   computers are host-mediated; they never hold the platform key.
8. **How much of a user computer's tape the host keeps.** The 2026-10-30
   direction left open whether host-side segments for user computers stay
   or go. The restore design answers it, and its answer is in revision:
   revision 1 keeps the whole sealed tape in the computer's own scope on
   the host (its open question 1); the revision in progress keeps the
   whole tape on the guest's volume with a sealed replica or window on the
   host. This note does not decide it: it holds at least the restore
   window on the host either way, never collects tape through the GC, and
   trims only by an audited act (section 4, Retained classes).

### Conflicts with the restore design

Where this note departs from the restore design's §10 list or depends on
its revision in progress, the difference is kept here, not silently.

1. **Legal hold is not a root provider** (restore design §10 item 8 asks
   for "a legal-hold provider that refuses purge in a held scope"). Here
   it is a catalog scope hold checked by `Purge` and `DeleteByAct` under
   the catalog lock, plus an input to the snapshots provider's retention.
   A provider streams (ref, holder) tuples; a hold names a scope, so as a
   provider it would stream every ref in the scope every run and still
   need the locked check to be safe. Same effect, one mechanism fewer.
2. **The tape is outside the hold audit and the catalog** (§10 items 6 and
   8 keep `tape-segment` as a store class and list user and paper segments
   under providers). Here segments are tape extents named by (computer,
   segment number), with no catalog object, holder or provider, and leave
   only by `DeleteByAct`. Nothing the GC decides should ever remove
   history, so a provider could only ever answer "held"; a holder row
   written after the seal would open a crash window in which a probe
   could release a sealed segment. The restore design's outcome stands:
   user segments are never collected, the paper's are archived, not
   purged.
3. **The "platform internal scope"** (restore design §4-5: audit segments,
   deletion manifests, access logs) is the platform tier here, retained
   class `audit`. The internal tier is per computer.
4. **Host services read internal bytes.** The restore design (§2) says a
   computer's internal scope is one "only it can read". Here host services
   read its tape, snapshot manifest headers and record witnesses, to
   assemble record witnesses, archive, export and stream the snapshots
   provider. They read only ciphertext and content-free records; "only it
   can read" holds for plaintext, not for bytes.
5. **A snapshot chunk's key salt is random.** The restore design (§3.1)
   derives a per-chunk key from the snapshot key without naming the salt.
   This note requires a random salt, as for payloads; a salt derived from
   the chunk would be convergent encryption. Dedup comes from the HMAC ID.
6. **An empty closure.** The restore design (§10 item 5) makes a retained
   snapshot without closure refs unable to run. Here a missing or
   unparseable closure field is unable to run, and an explicit empty list
   is an empty closure, a statement by the guest.
7. **Dependencies on the restore design's revision in progress** (its next
   revision after its round-1 review, not yet committed):
   - a restore as an ordinary forward diff batch with no epoch root, and
     record tables kept out of any rewind: no dependency here. The closure
     check needs only that every restore and drill reconstructs a store at
     a snapshot's head, which holds either way;
   - the guest keeping its whole sealed tape on its own volume and the
     host a sealed replica or window: section 1 and section 4 hold for
     either. If the host keeps a window, the trim is a `DeleteByAct` use
     the restore design must name, and that design must say which copy is
     authoritative when the two differ (one state authority); the guest
     volume is not in this store's backup;
   - moving the DEK off the data image and encrypting volumes as a
     ring-rolled change after the cutover: nothing here waits on it.
     Snapshots, payload sealing and owner-key delivery work with the DEK on
     the image; until it moves, the internal tier is not confidential from
     a host that reads the image (section 3, Keys). Key delivery at every
     boot moves with it;
   - the rollback before writes resume needs the engine migration to leave
     each pre-cutover volume untouched (section 7, Rollback).

## Ceremony

- Conjecture delta: blob bytes leave per-namespace directories and become
  one store with four tiers and two identity schemes; the per-namespace GC,
  the any-guest capture read and plaintext bases are retired; the paper's
  captures become platform-tier; every class moves in one cutover, the
  pre-cutover checkpoints and tapes are retired, and every computer's tape
  continues in the store. Revision 13: the host builds and unwraps
  nothing for history. Guests make sealed snapshots, whose manifest
  headers carry their closures, and the tape is kept by rule outside the
  GC, so the GC can no longer delete history.
- Protected surfaces: owner-content tenancy, Trace/evidence blobs and the
  tape, the GC that deletes them, snapshot closure, the capture, og and
  publication-text read paths, key custody (interim owner KEKs, owner-key
  delivery), legal holds and audited deletion.
- Admissible evidence: full referenced-set comparisons (not samples) inside
  the writer drain before the switch; process-kill and power-loss tests of
  every crash point; dry-run per-class counts matching the import receipts
  in every rehearsal before the cut; a verified backup extent manifest with
  open-extent prefixes; the export receipts, the keep-set restore drill and
  the sealed-restore drills before any deletion; the round 0g, 0h, 0i, 0j
  and 0l artifacts, including a zero-repair first audit (R0g-15, R0g-19);
  a staging red-team receipt for R0g-13. Local tests are not admissible for
  the drain, the release rollback or the restore drills.
- Rollback: before writes resume, one act for the whole cutover under the
  same writer drain: a release rollback of host services and guests,
  resolver config, the migration journals, and the guests' untouched
  pre-cutover volumes; run in every rehearsal. After writes resume, a
  forward fix, with the emergency conversion path of section 7 rehearsed
  once on staging. The frozen legacy directories stay until the comparison
  and a week of clean operation pass. Purges are irreversible and wait for
  their hold period.
- Heresy delta: `discovered`: projection bases are plaintext copies of owner
  stores on the host; og bodies are installed without fsync; capture bodies
  have no GC namespace and an existence oracle by hash; `captureBlockDoc`
  reads bodies with no caller; the `nix/node-b.nix:1298` comment
  contradicts the active namespace list at `:1307`; checkpoints are
  reconstruction records with no stored store, so revisions 9-11's
  checkpoint sidecar had no substrate. Revision 13: publication and
  proposal text is a live class no revision covered (`storage_ref` on
  `choir.artifact_blob`, written and read by corpusd), so the 12,534
  `*.txt` were not unreferenced; corpusd has blob writers the drain did
  not gate (platform-update mint and PUT, checkpoint artifacts,
  publication text); the private-ref rewrites named a write path that
  re-externalizes; a data-only rollback cannot run against the cutover
  release; the checkpoint JSON artifact has no reader; this note carried
  a superseded owner reading of the paper's tape and a stale "no key
  channel" claim. `introduced`: a new
  host service, a GC driver, a catalog with a hold table to keep
  consistent with packs and with the holders, a GC driver that releases
  holds on its own probe evidence, a `HolderRefs` resolver in every holder
  service, new canonical ref fields in private holders (mail `blob_ref`,
  the guest ref table, version-2 file manifests, a guest/host protocol
  change), `og_lookup_keys` rows for the capture blob fields, `image_refs`
  and `storage_ref`, `image_refs` on published og rows, two-phase release
  state and flag suppression on holds, per-hold class, object generations,
  pending and committed tombstones, a backup extent manifest with prefixes,
  write gates in corpusd and maild, and interim owner key custody;
  revision 13 adds the tape writer inside blobd, a snapshots provider,
  snapshot and record-witness classes, key commitments, scope holds,
  `DeleteByAct`, a migration write, and host reads of internal bytes.
  Revision 13 deletes: host replay workers and bases, closure rows and
  their witness acceptance, the segment ref table, the checkpoint artifact
  import, the idle and age seals, the backup force-seal and the small-pack
  merge. `repaired`: none until built.

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

## Revision 5 log (2026-10-30)

Round-4 reviews: `.agentic-consensus/object-storage-r4/out/devin.out` (D,
accept_with_notes) and `omp-deepseek-v41-flash.out` (S, send_back). Three
rounds had patched one mechanism, an object-level `released` flag whose
meaning was reconstructed from provider streams; NBF-1 and D notes 1-3 are
symptoms of it. Per the operating contract this revision changes the
substrate instead of adding a fourth patch: holder-scoped holds in the
catalog, reconciled by an audit (§4). "Stream presence voids a release"
(revision 4) and the release journal are gone. The hold model is a new
mechanism and gets its own tier-2 review. Code claims checked:
`captures.go:201` calls `sourcegraph.WriteCaptures`, which calls `PutBody`
three times before creating the object (`sourcegraph/captures.go:194-207`);
the computer DEK is generated in the guest and exported only for escrow
(`computerevent/privacy.go:84-108`).

| Finding | Ruling | Addressed in |
|---|---|---|
| S NBF-1 one holder's release covers another holder's un-released absence | accepted; fixed by the substrate, not by a sighted-holder set: a release is per (ref, holder), so B's hold stays unreleased and its absence pages | §2 interface; §4 Catalog, Holds not counts, protocol, Guards |
| S NBF-2 a 503 does not drain in-flight `WriteCaptures` | accepted, verified | §7 preamble (write gate, in-flight count to zero or corpusd stopped, then scan, compare, switch) |
| S NBF-3 compaction could drop released records | accepted | §5 Compaction (dead = purged only; copy every non-purged record; GC lock plus re-read at switch) |
| S note 1 range reads need per-frame digests; media type unbound | accepted | §2 Object format (catalog stores per-frame digests; media type in AAD and catalog); §3 Zero trust |
| S note 2 hold timing from `last_seen` only | accepted | §4 step 4 (`max(latest released_at, last_seen)`) |
| S note 3 release-journal last-event-wins | moot: no journal; hold changes are catalog WAL commits, the nightly snapshot is the off-host copy, rebuild fails toward retention or a page | §4 Catalog; §6 |
| S note 4 mixed encodings; conditional rollback; old directory vs Q6 | accepted | §7 step 2, Rollback, preamble |
| S note 5 platform key delivery to Autopaper computers | accepted: all platform reads are host-mediated; those computers never get the platform key | §1 tiers; §3 Who encrypts, Least-privilege read; §9 Q7 |
| D note 1 (a) stale scopes and released purge, (b) un-condemn on sighting, (c) no last-seen field | accepted: (a) suspended scopes block both judgment and purge; (b) subsumed, there is no condemned state, a sighting refreshes `last_seen`; (c) `last_seen` on objects and holds | §4 providers, Catalog, step 4 |
| D note 2 false pages after releases | accepted: Release comes before the drop, and a sighting older than `released_at` is ignored (the snapshot-time narrowing D proposed); a sighting newer than it restores the hold quietly | §4 step 3 |
| D note 3 Put-orphaned keep-set objects page forever | accepted: provisional holds and holdless puts expire as abandoned uploads after the upload grace; not a keep-set loss; a per-run cap pages on a burst | §4 Catalog, step 4, Guards |
| D note 4 content-key rotation needs guest redelivery | accepted | §3 Keys |
| D note 5 crypto-erase and guest-delivered wraps | accepted: deliveries are per boot and never persisted; erase stops the owner's running computers | §3 Who encrypts, Crypto-erase |

Predictions: R0g-10 names the replaced condemn phase; corrected by R0g-14,
and R0g-15 is new (round 0i, appended to the ledger). No committed record
is edited.

## Revision 6 log (2026-10-30)

Round-5 reviews (`.agentic-consensus/object-storage-r5/out/`): both seats
send_back on the same finding, the provisional-hold exit. A keep-set object
whose holder committed but whose provider omitted the row was deletable after
24 h with no page, under a 100-per-run allowance. Accepted (§4 audit step 3,
purge (b), Guards): keep-set provisional holds page and stay until an explicit
`Release` or an operator acknowledgement after a direct holder probe; no
per-run allowance.

Notes, all accepted, as rules the build must carry (§4 unless stated):
- Expected counts come from an independent aggregate query, not from the
  emitted stream; a mismatch suspends the scope (Devin 1).
- A ref held only in a retained checkpoint closure stays held; closures of
  retained checkpoints stream as holders, not only live revisions (Devin 2).
- Audit releases of non-keep-set canonical holds need two agreeing fresh
  audits (Devin 3, Guards).
- Catalog rebuild reconciles pack and offset for every object, replays the
  deletion-intent log written after the snapshot, and `IfAbsent` verifies the
  extent exists before discarding an upload (Devin 4, DeepSeek 1).
- Snapshot times are host-assigned: the read-transaction start on host
  databases, and for guest closures the host receipt time minus a skew
  margin. Errors in either direction fail loud (a page), and the repair log,
  not the page, is the operator signal for a sighting-restored hold
  (Devin 5, DeepSeek 3).
- Every sighting sets `first_seen` if null and refreshes `last_seen`; the
  lost-object check (a stream naming a purged or absent object pages) runs
  in every branch, not only the no-hold branch (DeepSeek 2).
- The merge-join uses one defined byte encoding of (ref, provider, scope,
  key) for streams and the `holds` primary key, and the dry run asserts the
  orders match (DeepSeek 4).
- A body held at two tiers is two objects. Publishing copies an og body
  into the public tier and points the published object's holder at the
  public ref; the draft keeps its private object. `CanonicalRef` does not
  infer tier from the encoding (DeepSeek 5, §1 og row, §7 step 3).

## Revision 7 log (2026-10-30)

Round-6 reviews (`.agentic-consensus/object-storage-r6/out/`): Devin
accept_with_notes, DeepSeek send_back. Both close round-5 NBF-1.

| Finding | Ruling | Addressed in |
|---|---|---|
| S NBF-1 (r6) the keep-set provisional exit cannot end a crashed in-place rewrite (holder row present, ref not named), and an ack on a non-independent probe reopens r5 NBF-1 | accepted; code verified: `PutObject` externalizes before the upsert (`objectgraph_store.go:107,115-131`), `WriteCaptures` puts three bodies before `CreateWebCaptureIfAbsent` (`sourcegraph/captures.go:194-229`) | §4 step 3 holder probe (independent query; asks whether the holder names this ref; driver `Release` recorded in the catalog; omission confirms, pages and suspends; operator decision only when the probe cannot run); §2 driver exception to the caller rule; §4 Catalog fields |
| S note 1, D note 2: revision-6 rules live only in the log; body contradicts | accepted; every rule inlined and the contradicting text removed | merge-key encoding (§2, §4, §7 preamble); host-assigned snapshot times (§4 providers); independent expected counts (§4 providers); retained-checkpoint closures (§1 image row, §4 provider table); og tier source and two-tier objects (§1 og row, §2, provider table, §4 step 2, §7 step 3); "expires" removed for provisional holds (§4 Catalog, Holds not counts) |
| S note 2: `IfAbsent` extent check, rebuild replay, `first_seen` and lost-object check | accepted, inlined | §2 Conditional put; §4 Catalog rebuild; §4 step 3 preamble |
| S note 3, D note 3: ack durability; never-held keep-set objects; unbounded pages | accepted | §4 Catalog (`decided_by`, `decided_at`, evidence; `abandoned_at`); step 3 never-held probe and page aggregation; step 4 path (c); §2 keep-set writers put with a holder |
| D note 1: non-keep-set canonical provisional holds deleted on "never streamed" | accepted | §4 step 3 (probe for every canonical class; plain expiry only for derived classes) |
| S risk: publish in place | accepted | §4 step 2 (touch the public copy, release the old-tier hold) |

Found while inlining: a private og holder needs an `hmac:` ID, which an
unrewritten `body_ref` cannot carry; §7 step 3 now rewrites such rows with
the step-2 journal and counts them first. R0g-14 needs no correction (§8
note). No committed record is edited.

## Revision 8 log (2026-10-30)

Round-7 reviews (`.agentic-consensus/object-storage-r7/out/`): Devin (D)
accept_with_notes, DeepSeek (S) send_back. Both close r6 NBF-1. The
revision-7 probe searched holder text for hex; that is wrong for private
holders that store the plaintext sha256, blind to tier, and cannot see refs
nested in manifests. Replaced, not patched: the probe is a by-key holder
lookup that returns canonical tuples. Code claims checked [V]:
`maild/attachments.go:133,236` and `maild/store.go:829` (mail stores the
plaintext sha256); `imageblob.go:91` (`artifact:sha256:`); `file_cas.go:13-21`
(root row holds only `manifest_ref`); `filecas.go:31-45,259-265` and
`artifact_gc.go:713-735` (chunks are bare hex inside the manifest);
`objectgraph_store.go:49,90` (path-form `body_ref` only); the legacy paths
in `file_cas_blobs.go:20,27`, `platform_update.go:61,94`,
`event_artifacts.go:124`.

| Finding | Ruling | Addressed in |
|---|---|---|
| S NBF-1: a private holder storing the legacy sha256 makes the probe's negative false (mail, images); nested file chunks invisible to it | accepted | §2 `HolderResolver`, class rules, "Private holders store the canonical private ref"; §4 holder probe (product read path, manifest decode); §4 provider table; §7 steps 3-6 (field, transaction, journal per holder) |
| S NBF-2: hex-only, tier-blind positive; publish in place leaves an unreleasable confirmed hold | accepted; publish is copy-only and the in-place clause is deleted | §1 og row; §4 step 2; holder probe compares full tuples, a same-ID different-tuple result pages and retains |
| S note 1: confirmation does not clear `released_at`; probe releases bypass guards | accepted | §4 holder probe (confirm sets `first_seen`, refreshes `last_seen`, clears `released_at`); Guards (probe releases count; two agreeing runs for non-keep-set canonical classes); pages cause "probe release" |
| S note 2, D note 1: path (c) has no "cannot run" branch | accepted | §4 never-held objects (a provider that cannot run blocks `abandoned_at`; only all providers ran and none found abandons) |
| S note 3: og hmac rewrite cannot roll back | accepted | §7 step 3 (journal prior `body_ref`, gated on the dry-run count); §2 og class rule; §7 Rollback |
| D note 2(a): "stored text" must mean decoded content | answered: the resolver reads through the product read path, never stored bytes | §4 holder probe |
| D note 2(b): source of guest probes | answered: live guest store; a checkpoint-only holder is read from its checkpoint, else cannot run; a stale read cannot run | §4 holder probe |
| D note 2(c): legacy list omits bare hex and three paths | accepted, paths verified in code | §2 legacy encodings and class rules |
| D note 2(d): a stray match flags a scope forever | answered: no hex matching; a flag clears per ref on a later stream or a probe-verified release | §4 holder probe |
| D note 3: confirmed keep-set hold absent from the stream has no exit | accepted | §4 step 3 (the probe runs on it after the abort) |
| S risk: `IfAbsent` repair drops the request's hold | accepted | §2 Conditional put |
| S risk: legacy list omits `file-cas-roots`, `platform-update` | accepted | §2 legacy encodings |
| S risk: hold rows of purged objects never deleted | accepted | §4 Catalog, step 4 |
| S risk: `IfAbsent` verification read cost against R0g-03 | assessed: no committed prediction moves | §8 revision 8 note |

No committed record is edited and no new record is needed (§8 note).

## Revision 9 log (2026-10-30)

Round-8 reviews (`.agentic-consensus/object-storage-r8/out/`): Devin (D)
accept_with_notes, DeepSeek (S) send_back. Both close every round-7 item.
Code claims checked [V]: `filecas.go:24,102` (version 1 only),
`file_cas_http.go:119` and `autoputer/file_sync.go:219` (both sides parse),
`file_sync.go:569` (the guest builds the manifest),
`objectgraph_store.go:447-450,510-540` (capture metadata matched by a JSON
predicate or a one-field-per-kind lookup key, no column per blob field).

| Finding | Ruling | Addressed in |
|---|---|---|
| S B1: the batch guard discards the probe releases the keep-set branch depends on; the step 2-5 rollback is such a batch | accepted; extended to non-keep-set probe releases, which carry the same per-holder evidence | §4 Guards (batch guard: audit releases discarded, probe releases page and apply); step 3 branches; Rollback releases its own holds, `decided_by=rollback` |
| S note 1: two agreeing runs has no persisted first agreement | accepted | §4 Catalog (`pending_release_at`, `pending_run`); step 3 two agreeing runs |
| S note 2: the published public image copy has no holder or provider | accepted, copy kept: it is an og-class object held by the published row's `image_refs` | §1 image row; §2 og class rule; §4 provider table, path (c) |
| D note 1, S note 3: retained checkpoints and the guest ref table | accepted | §7 step 5 (every retained checkpoint store filled from its own rows; unfilled is cannot run); §4 provider table, expected counts, holder probe |
| D note 2: the flag's release valve is unreachable inside the suspended scope | accepted | §4 step 3 flag resolution pass and operator override |
| D note 3: path (c) query for captures, vacuous "every provider ran", stray refs | accepted: stored generated columns; at least one provider and all ran; designated fields by contract, dry run counts refs outside them | §2 designated ref fields; §4 provider table, path (c) |
| S risk: v2 manifests are a guest/host protocol change | accepted, verified | §2 file-chunk rule |
| S risk: catalog rebuild can resurrect purged objects' hold rows | accepted | §4 Catalog rebuild |
| S risk: a permanently gone computer blocks path (c) | accepted: a recorded operator decision after the owner deletes the computer | §4 path (c) |

No committed record is edited and no new record is needed (§8 note).

## Revision 10 log (2026-10-30)

Round-9 reviews (`.agentic-consensus/object-storage-r9/`): Devin (D,
`out/devin.out`) accept_with_notes, DeepSeek (S,
`out-ds2/omp-deepseek-v41-flash.out`) send_back. Code claims checked [V]:
`retrieval_schema.go:34-40` (side table, not generated columns, on Dolt
2.1.9); `texturedoc/embed.go:21-22,72` (no embed publishable);
`maild/ingest.go:102-112` (inbound attachments inserted with
`storage_ref` NULL).

| Finding | Ruling | Addressed in |
|---|---|---|
| S B1r, D note 1: an above-threshold audit batch is discarded with no pending state and no exit | accepted, one mechanism: the guard counts decisions; that run writes the pending fields, the second agreeing run applies whatever the batch size; a recorded operator ack can apply a reviewed batch early. Keep-set unchanged | §4 Guards (batch guard); step 3 two agreeing runs; Catalog; page causes |
| S B2: checkpoint-only images are never imported, so a checkpoint fill has nothing to map | accepted | §7 step 5 (import from live union every retained checkpoint; a key without an object fails that fill, no row written; old directory kept until every fill succeeds) |
| S risk: where a checkpoint's ref table lives | accepted: a host-side sidecar keyed by checkpoint digest; sealed checkpoints and witnesses unchanged | §4 provider table, holder probe; §7 step 5, Rollback |
| S note 1: the flag pass contradicts "a suspended run judges nothing" | accepted: the pass is exempt | §4 two agreeing runs, flag resolution pass |
| S note 2, D note 2 (first half): generated columns against the Dolt record | accepted: `og_lookup_keys` rows, three per capture and one per `image_refs` entry | §4 provider table, path (c); §7 step 2; §8 note |
| S note 3: the confirm override is a one-run reprieve | accepted: `flag_suppressed_at`, ended by a stream naming the tuple or a hold change | §4 Catalog, holder probe, flag pass |
| D note 2 (second half): published images at import | answered: none exist; publishing an embed needs a Texture change first | §1 image row; §2 og class rule |
| D note 3: rollback leaves post-switch holders in new ref forms | accepted | §7 Rollback |
| S risk: mail import for inbound rows | accepted | §7 step 4 |
| S risk: dry-run scan false positives on body text | accepted | §2 designated ref fields |

Not taken as written: file chunks named by a retained checkpoint's
version-1 manifest are not imported in step 5; they stay legacy under the
file-chunk rule (§2), which already keeps them outside the store. No
committed record is edited and no new record is needed (§8 note).

## Revision 11 log (2026-10-30)

Round-10 reviews (`.agentic-consensus/object-storage-r10/out/`): Devin and
DeepSeek both accept_with_notes; every round-9 item closed. Notes taken:

| Note | Ruling | Where |
|---|---|---|
| D1, S1: checkpoint v1 file chunks; closure invariant over-claimed | accepted: the invariant covers store-class refs; checkpoints restore from pinned bases (`projection_jobs_http.go:92-94`); v1 chunks were never restore material and are deleted pre-launch | §4 invariant, guest-closures provider |
| D2: restoring a pre-switch checkpoint leaves no ref table | accepted: a restore re-runs the fill before the computer resumes | §7 step 5 |
| D3: sidecar is unwitnessed state feeding probe evidence | accepted: sidecar carries checkpoint and content digests, verified on read, rebuilt on mismatch, unverifiable means cannot run | §7 step 5 |
| S2: rollback misses v2 file roots and the tape segment ref table | accepted: both converted to legacy layouts in the rollback act | §7 Rollback |
| S3: `writeLookupKey` deletes every lookup kind for an ID | accepted: its delete is scoped to its own kind; capture and og writers own their ref rows | §4 provider table |
| S minor: suppression clearing list omits a stream sighting | accepted | §4 Catalog |

## Revision 12 log (2026-10-30)

Review: `.agentic-consensus/object-storage-codex/out/codex.out` (Codex,
send_back on revision 11). Owner ruling the same day: one cutover for every
class, never two data systems, scope not narrowed; Codex's "first slice"
cut is therefore not taken. Code claims checked [V]:
`tapestore/appender.go:508-540` (`startSegment`, `syncTree`);
`platform/artifact_gc.go:794-825` (intent before unlink, result after);
`platform/checkpoints.go:182` (checkpoint JSON plus a row);
`agentcore/checkpoint_restore_bindings.go:45` (no base, no replay at bind);
`projectionbase/capture.go:80` (a captured base holds the store alone);
`platform/captures.go:289` (`content_blob_id` in the owner's response);
`selfdevprotocol/witness_v2.go`, `checkpoint_verdict.go` (per-table
hashes; `verified`, `unverified`, `not_derivable`).

| Item | Ruling | Where |
|---|---|---|
| Owner: one cutover, no phased slice | accepted | top of note; Decisions 7; §2 file-chunk rule; §4 Activation; §7 preamble, parts 1-6, Rollback, Deleted; Ceremony |
| B1 pack creation lacks a directory durability barrier | accepted: create, fsync file, fsync parent and new directories, then append, fdatasync, catalog, ack; power-loss tests | §5 Write discipline, Recovery |
| B2 backup skips non-idle open packs; no completeness check | accepted: 1 h maximum open-pack age; the run force-seals every open pack; snapshot bound to a verified extent manifest; loss bound stated | §5 Packs, Compaction (small-pack merge); §6 |
| B3 rebuild replays intent as deletion; no re-put ordering | accepted: committed, generation-qualified tombstones; intent is audit only; a higher generation wins; compaction drops only tombstoned (ref, generation) | §2 Conditional put; §4 Catalog, rebuild, step 4; §5 Compaction |
| B4 retained checkpoints are not stored snapshots | accepted: the pre-cutover set and its restore history are retired; sidecar machinery removed; post-cutover closures are closure rows reconstructed at H by the verification rebuild, accepted on the ref table's witness hash | §4 Refs, Checkpoint closures, providers, holder probe, path (c); §7 part 5, Deleted; §9 Q1 |
| Note 1 class collisions | accepted: class per hold; protection is the strongest class among an object's holds | §2 Class is per hold; §4 Catalog |
| Note 2 Q5 oracle via `content_blob_id` | accepted with owner direction: Q5 dropped, every non-paper capture body private; R0g-16 in round 0j | §1; §2 capture rule; §7 part 2; §8 round 0j; §9 Q5 |
| Note 3 rollback without a drain | accepted: rollback runs under the import's writer drain | §7 Rollback |
| Owner: keep-set automatic deletion | report-only at cutover in the same store, turned on ring by ring; every other GC from day one | §4 Activation |

Not taken: Codex's deferrals (guest file CAS, tape and bases, group commit,
compaction) conflict with the owner ruling. Earlier logs are unedited.

## Revision 13 log (2026-10-30)

Reviews: `.agentic-consensus/object-storage-r12/out/devin.out` (D,
send_back) and `omp-deepseek-v41-flash.out` (S, send_back); both close
every Codex item of round 11. Also the restore and audit design, revision 1,
§10, and the owner directions at the end of the selection definition. Code
claims checked [V]: `objectgraph_store.go:44-49,107,268` (`PutObject`
re-externalizes; `execObjectWrite` holds the row and lookup write);
`sourcegraph/captures.go:194-229`; `filecas.go:24,102`;
`handlers.go:616`, `platform_update.go:125,306`, `checkpoints.go:182-201`,
`event_artifacts.go:146,379`, `service.go:411-412,790-791,943` (every
corpusd blob writer); `publication_graph.go:204-217,820-830` and
`service_publication_read.go:185-199` (`storage_ref`); grep for
`computer-checkpoints` (writer only); `key_delivery.go:45-144`,
`run.go:335-372`; `tapestore/format.go:106-107`,
`tape_segments_write.go:17-26,87`; `projectionbase/capture.go:44-69`;
`cmd/checkpointd/worker.go:181-205`; `scripts/off-host-backup.sh:195-245`.

| Finding | Ruling | Where |
|---|---|---|
| D NB-1, S F1: "the paper leaves the tape" cites a superseded reading; R0g-12 voided without re-filing; the checkpoint and base paths depend on the deleted tape | accepted. Every computer keeps a tape; the paper keeps a 7-day hot window and archives older segments; record witnesses kept forever. The host rebuild and base capture that needed the tape are deleted, so nothing depends on host-side replay. R0g-12's void clause applied; R0g-17 and R0g-18 filed | owner directions at top; Decisions 7-8; §1 tape, snapshot and record-witness rows; §4 Retained classes; §5 Tape, Record witnesses; §7 part 6; §8 round 0l |
| D NB-2: checkpoint artifacts enter the store with no class, provider or resolver | accepted by deletion: a checkpoint is its snapshot; the row names the manifest (class snapshot-manifest, snapshots provider, control resolver); the JSON artifact has no reader and is not written | §2 class rules; §4 provider table; §7 part 6 |
| D note 1: repair-superseded records never die | accepted: a record superseded by a higher generation of its ref is dead; compaction copies only catalog-current records and purged ones whose tombstone is pending | §5 Compaction |
| D note 2(a): ref-table fill must precede the first restore point | accepted: the fill is part of the engine migration at H0, before S0 | §7 part 5 |
| D note 2(b), S F4: the drain misses corpusd writers (platform update, checkpoint artifacts) | accepted, and widened: the gate covers every corpusd blob writer, found by code read, including publication and proposal text; the release pipeline pauses | §7 preamble |
| D note 3: the user host-side tape answers an owner-open item silently | accepted: stated as open, answered by the restore design (in revision), and worded to hold for either answer | §9 Q8; Conflicts 7 |
| S F2: the private-ref rewrites cannot go through the legacy write path | accepted: one migration write, gate-checked, no externalization, journal in the row's transaction | §7 Migration writes, parts 2 and 3 |
| S F3: a data-only rollback cannot run against the cutover release | accepted: before writes resume, a host and guest release rollback under the drain; after, a forward fix with a rehearsed emergency path | §7 Rollback; Ceremony |
| S N1: small-pack merge amplification; backup manifest race | accepted by deletion: no idle or age seal, no force-seal, no merge; open extents are backed up as verified prefixes; no extent is deleted during a backup run | §5 Packs, Compaction; §6 |
| S N2: closure replay must reproduce the ref table; v1/v2/`not_derivable` wording | accepted: ref rows are in the same projection batch, so replay reproduces them, and drills compare; the host acceptance key and the `not_derivable` exemption are gone with the host rebuild | §4 Snapshot closures; §7 part 5 |
| S N3: holders' rows must be in backups | accepted: holders' databases captured in the same run, before the catalog snapshot; guest stores rebuild from the store | §6 |
| S B3 residual (a): object row and generation must survive purge | accepted: the row is kept with its generation and `purged_at`; generations never reset | §4 Catalog |
| S B3 residual (b): the purge must record the hold-row list before deleting it | accepted: `pending_tombstones` row in the purge transaction | §4 Catalog, Deletion intent; §5 Recovery |
| Restore §10.1: "no such channel exists" is wrong | accepted in part, after the restore design's own review: a channel exists for the computer DEK; owner-key delivery stays new work on that channel's shape | §3 Who encrypts |
| Restore §10.2: delete host replay workers for bases | accepted | §3 Who encrypts |
| Restore §10.3: the paper keeps a hot tape; correct R0g-11 and R0g-12 | accepted | as D NB-1 |
| Restore §10.4: S0, not `PublishCapturedBase` | accepted; the function is deleted | §7 part 6, Deleted |
| Restore §10.5: closure refs from the manifest header | accepted, with an explicit empty list as a valid closure | §4 Snapshot closures; Conflicts 6 |
| Restore §10.6: tape host-named; `base` retired; snapshot classes | accepted; the tape leaves the catalog | §2 class rules; Conflicts 2 |
| Restore §10.7: key commitment in headers | accepted, extended to platform objects (one header format) | §2 Object format |
| Restore §10.8: snapshots provider; user segments never collected; paper archive; legal-hold provider | accepted except the legal hold's form | §4 providers, Retained classes, Legal holds; Conflicts 1-2 |
| Restore §10.9: deletion-order steps 5 and 6 | accepted | §7 deletion order |
| Restore §4-5: deletion manifests, audit entries, access logs in the "platform internal scope" | accepted as the retained `audit` class; the deletion-intent log becomes the run's deletion manifest | §2 audit rule; §3 Least-privilege read; §4 Deletion intent; Conflicts 3 |
| Restore revision in progress: forward diff restore, records out of rewinds, guest-held whole tape, DEK move deferred | worded to hold under each; remaining dependencies listed | §1 internal tier; §3 Keys; §4 Retained classes; Conflicts 7 |
| Found while revising: publication and proposal text is a live og class (`storage_ref`), not unreferenced files | taken: designated field, import, readers and writers switched; Q6 narrowed | §1 og row; §2; §4 providers; §7 part 3; §9 Q6 |

Not taken: none. Earlier logs are unedited; the revision 2 log's ruling on
D-B4 ("the paper leaves the tape") is superseded by this revision and
stays as written.
