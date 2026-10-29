# Keep-set accounting (2026-10-29)

Status: read-only investigation, mutation class green (docs). Nothing on Node B
was written, deleted, restarted or reconfigured. Input to the object-storage
design and the engine design ([selection](../definitions/choir-data-architecture-selection-2026-10-28.md)
owner direction 2026-10-29 ~22:30).

Labels: **[V]** verified by a command today (UTC ~23:25-23:30, host load 3);
**[I]** inferred, basis stated; **[U]** unmeasured, with why. Sizes are
logical (sum of field bytes) unless marked "du".

The owner's keep set: (1) everything captured, (2) the Autopaper object graph,
(3) the Textures produced (published stories and their documents/revisions),
(4) the owner's own email. Control state is accounted separately and is not
keep-set content.

## Method and caveats

- Store B queried read-only through `dolt sql --host 127.0.0.1 --port 13307`
  as root (password via `DOLT_CLI_PASSWORD`, never printed), each query under
  a 30 s `timeout`, aggregates over indexed or small tables only. Host
  artifacts by `find -printf` (metadata only) with `nice`/`ionice`; temp files
  were under `/tmp` on the host and were removed.
- **`information_schema.tables` is unreliable on Dolt** [V]: `avg_row_length`
  and `data_length` for `og_objects` read 8.59e9 and 5.69e15, for `og_edges`
  4.29e9 and 2.14e15 (overflow garbage), `proj_capture_index` reads 13.3 GB
  for 65,677 rows (202 KB/row, not credible against the 1.85 MB measured for
  5,000 rows). `table_rows` is right (it matched `COUNT(*)` for the og tables).
  Logical bytes below are `SUM(octet_length(...))` instead.
- Owner identity: `users` in `/var/lib/go-choir/auth/auth.db` has 1,063 rows.
  An exact case-insensitive match on `yusef@mosiah.org` returned **no row** [V].
  A prefix match on `yusef` returns exactly one user, `5bd6de97-3b58-408c-bf89-c42c81b083de`
  (created 2026-05-26; stored email is 21 characters, not the 16 of the given
  address, so it is a different address from the one in the brief). That user
  is also the one with a populated maild mailbox (254 messages) and the one
  with the always-active interactive computer below, so I treat it as the
  owner [I]. The owner should confirm; if there is a second account, its
  email, computer and mailbox are not covered here.

## 1. Captures (kept: metadata and bodies)

### Where it lives

| Place | What | Count | Logical bytes | Notes |
|---|---|---|---|---|
| Store B `og_objects` kind `choir.web_capture` | capture metadata (JSON in `metadata`; `body`/`body_ref` empty, `body_size` 0) | **65,677** [V] | 63.6 MB metadata [V] | 30 distinct owners, 57,940 distinct canonical URLs [V]; created 2026-10-11 11:33 to now [V] (the 10-09 corpus reset emptied the earlier set) |
| Store B `og_edges` kind `cites_capture` | publication -> capture citations | 45,424 [V] | in the og_edges total below | |
| Store B `proj_capture_index` / `proj_capture_terms` | **derived** search/lookup index of captures | 65,677 / 820,512 rows [V] | ~24 MB / ~118 MB (extrapolated from 5,000- and 20,000-row samples) [I] | rebuildable from the capture objects (`ReconcileCaptureIndex`, `internal/platform/capture_index.go:329`); not keep-set content |
| Host `/var/lib/go-choir/platform-artifacts/capture-bodies/<2 hex>/<sha256>` | the bodies | **154,649 files, 9,374,578,057 B (9.37 GB)** [V] | 9.37 GB | see below |

Owner split of the 65,677 capture objects [V]: the paper's user
`e5962310-8767-4624-87e5-a9b93a4e1db1` has 63,837 (97.2%); 29 other owners have
1,840 (the owner `5bd6de97` has 15).

### The blobs the captures reference (field names)

`objectgraph.WebCaptureMetadata` (`internal/objectgraph/web_capture.go:24-73`)
carries three blob refs, each `sha256:<hex>` of a body in `capture-bodies/`:

| Field | Meaning | Distinct referenced | Bytes on disk | Share |
|---|---|---|---|---|
| `content_blob_id` | raw bytes as fetched, truncated at 1 MiB | 64,383 | 8,351,253,670 | 89% |
| `blocks_blob_id` | extracted block JSON (65,511 captures have one) | 65,512 (incl. one blank) | 681,432,332 | 7% |
| `extracted_text_blob_id` | text projection | 63,352 | 360,233,566 | 4% |

Metadata also records `raw_bytes`; their sum over all captures is 8.476 GB [V],
consistent with the raw bodies on disk (the 8.35 GB is distinct, deduplicated
bodies).

### Referenced versus present [V, full set comparison, not a sample]

- Distinct blob hashes referenced by Store B web_capture metadata: **154,613**.
- Of those present in `capture-bodies/`: **154,613 (100%)**; missing: **0**.
- Files in `capture-bodies/` not referenced by any web_capture: **36**
  (5.70 MB), probably in-flight writes or captures whose object write failed
  [I]. Referenced bytes: 9,368,882,005 B (9.37 GB), i.e. **99.94% of the
  directory is referenced and therefore keep-set**; the earlier idea that the
  directory is a prunable cache is wrong while captures are in the keep set.
- 604 blobs referenced by non-paper owners are also referenced by the paper
  (cross-owner dedup, as the PutBody design intends).
- Paper-owned captures reference 150,811 distinct blobs, 8.95 GB; the other
  29 owners reference ~3.8k blobs [I: 154,613 - 150,811 plus the 604 shared].

### Writer, readers, growth

- Writer: `sourcegraph.WriteCaptures` (`internal/sourcegraph/captures.go:134`,
  `PutBody` calls at `:194,203,207`) via `POST /internal/platform/captures`
  (`internal/platform/captures.go:164`); body store `CaptureBodyStore.PutBody`
  (`captures.go:86`, plaintext, one file per body, no GC namespace).
- Readers: `HandleCaptureBody` (`captures.go:214`, any bound guest), capture
  read `captures.go:244,279,303`, publication anchor resolution
  `publication_versions.go:448-465` (`captureBodies.Body(BlocksBlobID)`),
  `researchtools/texture_captures.go:90`, capture index `capture_index.go:186`.
- Growth: Store B objects of kind web_capture per day (by `updated_at`) are
  2.2K (10-13), 4.9K, 2.9K, 3.3K, 0.8K, 0.6K, 1.1K, 1.3K, 6.5K (10-21), 9.0K,
  7.5K, 4.0K, 4.1K, 4.7K, 4.2K, 4.2K (10-28), 4.4K (10-29, partial) [V];
  last week about **4-5K captures/day**. Bodies: 18,979 files and 0.85 GB on
  10-28, 0.49 GB so far 10-29 (inventory correction) [V]: **~0.5-0.9 GB/day,
  ~19K files/day**, about 2.4 files per capture [I]. At that rate the keep set
  gains ~15-25 GB a month with no pruning.

## 2. The Autopaper object graph (kept)

### Store B tables [V counts; bytes by SUM(octet_length)]

Store B `corpus` on :13307: 15 GB du (+1.1 GB `.dolt`) on disk, so the
logical content is a small fraction (below); the rest is Dolt chunk and
history overhead.

| Table | Rows | Logical bytes | Class |
|---|---|---|---|
| `og_objects` | 662,422 | metadata 361.4 MB + inline `body` 133.4 MB + canonical_id/keys ~97 MB (ids alone) = ~0.6 GB in table; plus 1.02 GB of `body_size` declared (see blobs, of which 0.47 GB is distinct) | **keep (object graph)** |
| `og_edges` | 499,372 (16 tombstoned) | metadata 33.5 MB + ids/kinds 191.6 MB = ~225 MB | **keep (object graph)** |
| `og_lookup_keys` | 156,190 | ~30 MB (extrapolated from 20,000 rows) | derived index, rebuildable [I] |
| `proj_capture_index` | 65,677 | ~24 MB (+5 secondary indexes) | derived |
| `proj_capture_terms` | 820,512 | ~118 MB | derived |
| `publications`, `publication_versions`, `public_routes`, `artifact_*`, `citation_edges`, `consent_records`, `provenance_*`, `retrieval_*`, `review_records`, `rollback_refs`, `verifier_attestations`, `platform_texture_*`, etc. (26 tables) | **0** each | 0 | empty pre-split leftovers; the live data for these concepts is og kinds below, not these tables |

Logical Store B total, keep plus derived, is about **1.2 GB**, against 16 GB on
disk [V du], a ~13x overhead from Dolt chunking and history [I].

### `og_objects` by kind [V]

| Kind | Rows | Inline body + external `body_size` | Metadata bytes | Role |
|---|---|---|---|---|
| choir.publication_transclusion | 138,438 | 56.8 MB (5,099 externalized, 826 distinct) | 186.1 MB | story -> captured-block embed; graph |
| choir.publication_check | 136,744 | 25.1 MB | 20.3 MB | check results |
| choir.support_judgement | 85,362 | 14.8 MB | 11.1 MB | |
| choir.web_capture | 65,677 | 0 | 63.6 MB | captures (section 1) |
| choir.publication_source_entity | 61,593 | 52.3 MB | 18.7 MB | |
| choir.provenance_entity | 23,740 | 376.1 MB (11,870 externalized) | 4.4 MB | publication provenance |
| choir.artifact_manifest | 11,870 | 376.1 MB (11,870 externalized, **same blobs as provenance_entity**) | 2.4 MB | the published Texture text (section 3) |
| choir.publication_version | 11,870 | 0 | 16.2 MB | |
| choir.publication_resolved_blocks | 11,866 | 116.9 MB (11,227 externalized, 8,955 distinct) | 1.9 MB | captured blocks pinned into a version |
| choir.consent_record, provenance_activity, publication_proposal, retrieval_manifest, retrieval_source, retrieval_span, review_record, verifier_attestation (8 kinds x 11,870) | 94,960 | 3.8 MB | 28.8 MB | per-version provenance rows |
| choir.artifact_blob | 11,755 | 0 | 1.9 MB | |
| choir.publication / public_route | 3,443 / 3,443 | 0 | 1.8 / 1.1 MB | |
| choir.entity_review, entity, entity_stopword | 695 / 690 / 207 | 0.5 MB | 0.3 MB | |
| provenance_agent, publication_policy, subject (18 each), software_release 4 | 58 | 12 KB | small | |

`og_edges` by kind [V]: transcludes 138,445; references_entity 61,593;
cites_capture 45,424; contains_span 23,740; eleven per-version kinds at 11,870
(attested, attests_to, authored_review, consent_for, contains_blob,
derived_from_proposal, generated, granted_consent, has_manifest, has_policy,
has_retrieval_manifest, has_retrieval_source, has_version, is_version_of,
reviews, was_associated_with, was_derived_from); has_resolved_blocks 11,866;
supersedes 8,427; owns 3,443; routes_to 3,443; mentions_entity 1,183;
has_agent 18.

### The og blobs [V full set comparison]

Bodies of 1 KiB or more are externalized to
`platform-artifacts/sha256/og/<sha256>.bin` (`internal/platform/objectgraph_store.go:40-68`),
referenced by `body_ref = sha256/og/<hex>.bin`.

- Distinct `body_ref` values in Store B: **21,848**; present on disk: 21,848
  (100%), missing 0; the directory has 21,862 files, 14 orphans; referenced
  bytes **474,696,656 (0.47 GB)** of 475,187,499 total [V].
- The declared `body_size` sums (1.02 GB) overcount because `artifact_manifest`
  and `provenance_entity` point at the same 11,870 blobs, and resolved_blocks
  and transclusion bodies repeat [V].
- The `og` namespace is in the active GC set (`artifact_gc.go:118,443`), so
  body_refs written from Store B are live roots; an unreferenced `.bin` is
  collected after the 168 h grace [V from inventory].
- Separate, **not** referenced by Store B (all body_refs are `sha256/og/*.bin`
  [V]): 12,534 `*.txt` files at the top level of `platform-artifacts/sha256/`,
  24.7 MB, hand-kept pre-teardown corpus bodies per the inventory [I]. Not
  keep-set unless the owner says so.
- Growth of `og/*.bin` (file mtimes, which reuse refreshes, so approximate):
  862-2,862 files and 16-86 MB per day over 10-22..29 (10-22: 2,862 files,
  85.7 MB; 10-28: 862, 18.0 MB; 10-29 so far: 1,442, 39.2 MB) [V].
- Growth of Store B objects: 12K-112K objects/day (10-13..10-29, peak 111,923
  on 10-22; 28-51K a day over the last four days) [V].

### Writer and readers

- Writer: `ObjectGraphStore` (`internal/platform/objectgraph_store.go:196-270`,
  `externalizeBody` `:44`), on the Store B pool via `Store.corpus()`
  (`internal/platform/store.go:375`). Publication kinds are written in one
  batch by `PublicationGraphStore.PublishTextureToGraph`
  (`internal/platform/publication_graph.go:31`; version `:132`, resolved
  blocks `:163`, manifest `:184`, blob `:204`), called from
  `Service.PublishTexture` (`internal/platform/service.go:224`) under `writeMu`.
- Readers: `service_publication_read.go` (`resolvePublicationHead` `:52`,
  bundles `:134,147`, `graphResolvedBlocks` `:294`), `publication_front*.go`,
  `publication_related*.go`, `publication_opinion.go`.
- Kinds are declared in `internal/objectgraph/registry.go` (resolved blocks
  `:194`); the schema DDL runs on the Store B pool (`store.go:418`).

## 3. Textures produced (published stories and documents/revisions behind them)

### What is in Store B (published Textures) [V]

A published Texture is a `choir.publication` (3,443) whose
`choir.publication_version` objects (11,870; 8,427 `supersedes` edges) carry
`source_doc_id`, `source_revision_id`, `source_revision_hash`,
`revision_hash`, route path, signatures. The text itself is the
`choir.artifact_manifest` body (the manifest JSON, externalized to
`sha256/og/*.bin`, 376 MB for 11,870 versions, ~31.7 KB each), with the
captured blocks it embeds in `choir.publication_resolved_blocks` (117 MB).

- 3,443 distinct `source_doc_id`; 11,870 distinct `source_revision_id`
  (one per version) [V]; 18 distinct owners of publications.
- Owner split [V]: the paper user `e5962310-...` 3,330 publications and 11,614
  versions (97%); 112 publications across the other 17 owners. The owner
  `5bd6de97` has **no** publications in Store B (only 15 captures).
- Window: first publication created 2026-10-13 04:02:43, latest 2026-10-29
  23:24:50 (the 10-09 reset emptied earlier ones) [V]. Per day: 117 (10-13),
  367, 252, 149, 63, 57, 81, 95, 245, 420, 396, 238, 248, 176, 183, 126, 230
  (10-29) new/touched publications and 358-1,481 versions/day [V], so ~150-250
  new stories and ~500-1,000 versions a day lately.
- Logical bytes for the published set: manifest bodies 376 MB + resolved
  blocks (<= 117 MB) + version/publication metadata ~18 MB + provenance rows
  ~30 MB, i.e. **~0.55 GB**, plus the referenced captures (section 1).

### Where else Textures live

| Place | What | Measured? |
|---|---|---|
| Paper guest store (`computer-1355b83bced1613eb048ee28d66688a8`, `vm-1669020a...`, state `active`, ownerships epoch 13,353) | the canonical `texture_document` / `texture_revision` rows (working documents and all revisions, not only published ones) in the guest's own store; Texture is the sole writer (CLAUDE.md desk ontology) | **[U]** No host read API reaches the guest store without a computer session token, the guest disk (`data.img`, 31 GiB on host after the fstrim) is live-mounted by the running VM, and the brief forbids booting or waking. I did not call any in-guest API. The only count available is a lower bound: 11,870 published revisions of 3,443 documents. |
| Paper tape `tape-seg/computer-1355b83b...` | 26 GB of events and payload pins; contains the revision-writing events | du [V]; Texture content within it **[U]**. The owner direction (21:10) is that the paper does not need the tape. |
| Paper `projection-base` (14 base rows in `computer_projection_bases` for this computer [V], 26.5 GB largest) | the `runtime.db` replay, which contains the guest store | derived [I] |
| Owner computer `computer-03335285269bdba4f94377e56879f9e6` (`vm-state/candidate-fleet-e15cb89f25d963c220319b7b`, `active`, epoch 1,464, last active 2026-10-29 19:24) | the owner's Textures (drafts and documents) in the guest store | **[U]**, same reasons. Host-visible: `data.img` 32 GiB apparent (du 5.2 GB), tape-seg 8.4 MB, file CAS chunks 143 MB, 3 projection bases. |
| Store A `platform_texture_documents` 185 / `platform_texture_revisions` 545 | July leftovers | dropped from Store A today (not in the 35 remaining tables [V]); empty copies in Store B [V]. Not keep-set. |

### Duplicates [V/I]

- A published version duplicates a paper-guest revision, as an immutable
  snapshot: `publication_version.source_revision_id` /
  `source_revision_hash` point at the guest's `texture_revision` row, and the
  manifest body is a copy of its text plus pinned captured blocks [I from
  `PublishTexture`, `service.go:224`]. So the 3,443 documents and 11,870
  revisions exist in both the paper guest and Store B; Store B's copy is the
  public one. The guest additionally holds every unpublished revision and the
  working state. The extent of the overlap beyond the 11,870 is **[U]**.
- Inside Store B the same 11,870 manifest blobs are referenced twice
  (`artifact_manifest` and `provenance_entity`), one blob on disk [V].
- Captured blocks are not duplicated per story: `publication_resolved_blocks`
  and transclusions pin copies of block text (117 MB + 19.5 MB), which
  duplicate the capture blocks bodies (`blocks_blob_id`) logically [I].

## 4. The owner's email (kept)

Storage root `/var/lib/go-choir/mail` (`internal/maild/store.go:281-312`,
`mail.db` path `:297`). Total 13 MB du [V].

| Piece | Size | Count [V] |
|---|---|---|
| Routing DB `mail/mail.db` | 1.7 MB (+wal 164 KB) | shared tables: 5 aliases, 2 owner addresses, 151 provider-message-index rows, 209 webhook events, 2 receive policies |
| Per-owner DBs `mail/users/<owner>/mail.db` | 10 owners; 9.4 MB for the owner, 88-100 KB each for the rest | the rest are empty or near-empty: 8 of the 9 others have 0 messages, two have 2 each (50ef1965..., 70ff995f...) |
| **Owner `5bd6de97-...` mailbox** | `mail.db` 9.03 MB (+wal 0.72 MB), logical text 7.35 MB (text_body + html_body) | 254 messages (195 inbound 2026-05-28..2026-10-29, 59 outbound 2026-05-28..2026-10-23), 263 recipients, 32 attachments rows (1,705,138 B declared), 73 drafts, 195 source packets, 3 ingress events |
| Attachments `mail/attachments/outbound/<owner>` | 1.2 MB in 10 files, three owner dirs; `quarantine/` empty; `mail/raw` empty | the owner's outbound set is the bulk [I] |

No message content was read. Inbound mail is still arriving (latest inbound
2026-10-29 22:53:26). Writer: `internal/maild` (store.go; attachments
`attachments.go:215-275`). Total mail keep set: **~11-13 MB**, growing ~tens of
KB a day [I: 254 messages over five months].

## 5. Control state (needed by running computers; control, not keep-set content)

Store A `platform` on :13306, 55 GB du, 35 tables after today's drop [V]. Row
counts [V]: `computer_event_append_receipts` 6,054,889; lifecycle receipts
28,808; key escrow transparency 3,490; file roots 2,048; `verifier_attestations`
640; route authorization evidence 576; lifecycle operations 554;
**key escrows 279**; projection bases 254; event heads 251; checkpoints 213;
route transition receipts 213; route slots 181; tape tombstones 165;
self-development modes 103; replay watermarks 95; projection jobs 96;
owner_act_receipts 59; the rest under 80 each; key unwrap requests/approvals 2/4.
The live dump was 35.7 GB of SQL before the drop (selection doc), dominated by
the 6.05 M append receipts (so that one table is most of Store A, as the
dead 6.09 M-row og_objects were before it was dropped) [I]. Key escrow tables
must survive any rebuild (pre-split census). Other control state: platform
signing keys under `platform-artifacts/{signing-key,key-escrow-key}`,
`auth.db` (1,063 users, owner root-key wraps), `ownerships.json` (152
ownerships, 151 distinct users), `gateway-identities.json`, secrets.

## 6. Keep set, in one table

| Keep-set kind | Places | Logical bytes | Rows / files | Growth |
|---|---|---|---|---|
| Captures | Store B `web_capture` + `capture-bodies/` | 63.6 MB metadata + **9.37 GB** bodies (raw 8.35, blocks 0.68, text 0.36) | 65,677 objects; 154,613 blobs | ~4-5K objects, 0.5-0.9 GB, ~19K files /day |
| Autopaper object graph | Store B `og_objects`, `og_edges`; host `sha256/og/*.bin` | ~0.6 GB + ~0.23 GB tables (includes the capture and publication rows above) + 0.47 GB blobs | 662K objects, 499K edges, 21,848 blobs | 28-51K objects, 16-86 MB blobs /day |
| Textures (published) | Store B publication kinds + og blobs (+ pinned blocks) | ~0.55 GB, included in the object graph row | 3,443 docs, 11,870 versions | ~150-250 stories, 500-1,000 versions /day |
| Textures (working docs/revisions) | paper guest store; owner computer guest store | **[U]** | **[U]** | **[U]** |
| Email (owner) | maild per-owner `mail.db` + outbound attachments | ~11-13 MB | 254 messages, 32 attachments | tiny |

Hard numbers for what must survive a rebuild: **~9.9 GB of files plus ~1.2 GB
of logical Store B rows**, i.e. about **11 GB**, versus 16 GB on disk for Store
B alone and 9.37 GB in the capture-bodies directory. Derived and
rebuildable inside that: proj_capture_* and og_lookup_keys (~0.17 GB).

## 7. Everything else (disposable or derived), side by side

Sizes from [the inventory](node-b-storage-inventory-2026-10-29.md) (du, taken
earlier today) and the later receipts; the root fs is now 358 GB used, 110 GB
free (77%) [V `df`]. Not additive (hardlinks, reflinks).

| Category | Bytes | Disposition vs the keep set |
|---|---|---|
| Store A `platform` (control, mostly 6.05 M append receipts) | 55 GB du | control; the 35.7 GB dump is what a new engine must hold; needs the squash/export, not this keep set |
| `projection-base` (derived `runtime.db` replays) | 61.6 GB (425 files) | derived, disposable by retention rule |
| Paper tape `tape-seg` | 26.4 GB of 27.2 GB | owner: the paper does not need the tape; disposable for the paper |
| Legacy pins (`pin-receipts` 19.7, `computer-event-payload` 17.8, `computer-event` 9.9) | 47.4 GB, 7.4 M inodes | tape legacy; disposable unless user computers need it |
| Per-computer `file-cas-chunks` | 27.9 GB (197 computers) | disposable except per-owner isolation; ~150 MB seed copy each |
| Paper `vm-state` disk image | 31 GiB (host, after the guest fstrim) | the paper computer; its Texture and runtime content is [U] and potentially keep-set |
| Other `vm-state` (candidate fleets, `vm-48bc0981`, `vm-70070cf7`) | ~8 + 5.2 + 3.2 GB, plus an 8 GiB mem snapshot | owned interactive computers; the owner's is `candidate-fleet-e15cb89f...` (5.2 GB du) |
| `/nix/store`, guest-releases, builder releases, `/swap`, `/root`, caches, services | 49.3 + 11.8 + 6.0 + 16.8 + ~4.6 + ~3.3 + 2.5 GB | build and system; not data |
| `/data` (March-April per-user subvolumes + 62 snapshots) | 9 GB | awaits explicit owner yes to delete |
| Other `platform-artifacts` (`platform-update` 1.4, `file-cas-roots` 1.0, `computer-inputs` 0.7 live blobs, gc/sweep logs 0.8, checkpoints 0.1, `sha256/*.txt` 0.025) | ~4 GB | control or evidence |
| Keep set | ~11 GB (above) | **keep** |

Reading it: the keep set is about 11 GB of roughly 358 GB used, 3% of the disk.
Two items not yet resolved decide the real figure: the Textures in the paper
and owner guest stores (unmeasured), and whether the 29 non-paper owners'
captures (1,840 objects) belong to the keep set (they are "captured", so
counted here).

## 8. Residual unmeasured and how to close each

1. **Guest-store Texture counts** (paper and owner): need an authenticated
   read through the computer's existing API (a session on the owner's
   computer, which is active, and the paper's), or an offline read of the
   paper's data image on a reflink copy as `projection-compact` did today
   (`store-compact`). No wake or boot is needed for the offline route.
2. **Whether the owner has a second account**: the stated address did not
   match `auth.db` exactly; one `yusef*` user exists.
3. **Dolt-on-disk versus logical** (16 GB vs ~1.2 GB): an `INFORMATION_SCHEMA`
   figure is not trustworthy; a `dolt gc` size or a per-table export count
   would settle it.
4. **The 36 orphan capture bodies and 14 orphan og blobs**: likely in-flight;
   not examined further.
5. **Duplicate proof between Store B and the paper guest** (published revisions
   vs the guest's own revision rows): blocked by (1).

## Heresy delta

`discovered`: (a) the earlier "capture-bodies is a prunable cache" disposition
in the inventory is wrong, since 99.94% of the directory is referenced by
Store B captures and they are in the keep set; (b) `information_schema` row
size fields are overflow garbage on Dolt for large tables; (c) the stated
owner email matches no `users` row exactly. `introduced`: none. `repaired`:
none.
