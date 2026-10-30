# Cut 2 freeze receipt

**Status: FINAL, 2026-10-13.** Written before the designated paper
computer's first publish (computer-1355b83bced1613eb048ee28d66688a8, owner
e5962310-8767-4624-87e5-a9b93a4e1db1;
docs/archive/reports/reeng-autopaper-paper-computer-2026-10-13.md). Compatibility
begins at the paper's first Autopaper permalink: from then on the record
schema, normtext, the cid formula, the extractor versions, the Ref shape,
the /v<n> route shape and the manifest schema change only compatibly
(decisions §12.1). Tracer test computers published versions before this
receipt; those are pre-freeze and carry no compatibility promise.

Drafted on source b104f274 and finalized on main at the commit that adds
this file. The digests below were computed at b104f274 and verified
unchanged at finalization: `git diff b104f274 HEAD` over every file they
name (internal/normtext, internal/quotematch, internal/types/{commitment,
inbox,ref,story,support,texture}.go, internal/store/inbox.go,
internal/platform/{publication_versions,signing}.go,
internal/sourcefetch/blocks.go) is empty, so each declaration's text, and
its digest, is the same.

## How the digests are computed

Each digest is the sha256 of the exact Go source text of one declaration
(a type spec, a const or var spec, or a whole func declaration, from its
name through its end, comments above it excluded), as `go/parser` reports
its position in the file at the source commit. A change to a declaration,
including its field tags, changes its digest. Recompute with any Go
program that parses the file and hashes `src[decl.Pos():decl.End()]`; the
one used for this draft walked `f.Decls` and hashed the matching
`*ast.TypeSpec`, `*ast.ValueSpec` or `*ast.FuncDecl`.

The finalizer recomputes every digest at the commit that is live when the
paper first publishes, and records that commit here.

## 1. Record schema

Records are `types.CommitmentRecord` (schema id `CommitmentRecordSchemaV1`)
committed through the inbox store (`internal/store/inbox.go`). Fields the
records route uses: `schema_id`, `record_id`, `kind`, `discrepancy`,
`provenance` (`agent_id`, `committed_at`), `directive` (subtype, objective,
body), `emitter_kind`, `computer_id`, `subject_refs`, `inbox`,
`sender_inbox`, `inbox_seq` (store-assigned), `conversation_id`
(store-derived from the unordered inbox pair), `round` (store-assigned),
`final`, and the typed bodies `cast`, `report`, `support`, `captures`.

| Declaration | File | Digest |
|---|---|---|
| `CommitmentRecord` | internal/types/commitment.go | sha256:0ecb3f6e31dd178f71ccd95d87a5cf4d116d1316b29c2e449f8fc8cf998e430d |
| `CommitmentReport` | internal/types/commitment.go | sha256:323e2837838d240c358eb7bddd961277aafaeda57b5bd9c0f59ea078bfb25b75 |
| `CommitmentEmitterKind` | internal/types/commitment.go | sha256:39a1be899234301e75b540f3c7d6ebd42070bf41567c359a1550080ef442eb0c |
| `InboxRecordReceipt` | internal/types/inbox.go | sha256:97c0c6e361db7a73855c6ea6f5f2463daa4247acd2a5cd4dd47e14d959776c5f |

### Opening Cast (pre-flight review A6')

A story opens with a management Cast addressed to `texture:new`
(`types.StoryOpenInbox`). The store creates the story in the Cast's commit
and delivers the Cast to the new `texture:<doc>`. The opening Cast's body
is `CommitmentCast`: `hypotheses` (each `statement`, `probability`,
optional `stakes`, `crux`, `general`, `meaning`; the store assigns `id`
H1, H2, ... per conversation), `uncertainties`, `revisions`,
`withdrawals`, `captures` (capture ids Texture may cite at once) and
`publish` ("" or `"auto"`).

| Declaration | File | Digest |
|---|---|---|
| `CommitmentCast` | internal/types/commitment.go | sha256:1adea6070ef0b2319befcb897447e02868e09dc4075a8bd01ff35e587f6feb78 |
| `CastHypothesis` | internal/types/commitment.go | sha256:7758c7736068920db028973a567b80fbcd708322f45a1b40449d8044904e43de |
| `CastUncertainty` | internal/types/commitment.go | sha256:83b1d602625ae9592e03911d68e700470ff1dfe27d34ab2e648520a4d7368d20 |
| `CastRevision` | internal/types/commitment.go | sha256:a549c589454c60948cd7f2e557b6e5e18c7fde66887681ada40c923b9192d796 |
| `CastWithdrawal` | internal/types/commitment.go | sha256:ec46e33088e687e895b7ff9657be29ccdfc8260cd24f07fb6de032765c367f55 |
| `CastPublishAuto` | internal/types/commitment.go | sha256:8bc8404ce7aee1d7ca24aeb35b4daf79ec0a99a2fedbbfc46ad8b67403038cf0 |
| `StoryOpenInbox` | internal/types/story.go | sha256:e76874816dc3ae98ccb114cb58b0c69669f7d657d8a8b4c16c2b92e2aa273463 |

### Support records (2a (v))

A Texture commit's system list to `management:<owner>` (`support.pairs`,
record id `support-pairs:<revision_id>`, provenance `system:support`), and
management's failing verdicts to `texture:<doc>` (`support.failures`).
Pairs are keyed by `SupportPairKey` = `"sp-"` + the first 16 bytes (hex)
of sha256(citing block hash, NUL, cited block hash, NUL, hex sha256 of
the quote, NUL, matcher version).

| Declaration | File | Digest |
|---|---|---|
| `CommitmentSupport` | internal/types/support.go | sha256:cd27d4da141cd4b6cad7075f0be6dfe80d0e66866e8aaa71c960ee8d70e8c922 |
| `CitationPair` | internal/types/support.go | sha256:c991ec20b04f117c30a90d2ec6d602f61236e3492a214cef148ec4110530c5d1 |
| `SupportVerdict` | internal/types/support.go | sha256:c4229d2f80169a183d2360461b5b2ee81bbba02cd8518e3d3f713e22ed0889f6 |
| `SupportPairKey` | internal/types/support.go | sha256:79d32bb7741be21a5cdceaa3c77291b9794192a179c4e51b0fc85cf833aac3e6 |
| `SupportPairsRecordID` | internal/types/support.go | sha256:83f603eeacccf7f34ffddd6c759a6cfee08dc89d39fb0f53d8b9d01f914bbaf0 |

### Capture arrival (2b)

One system record per source poll that wrote new captures, to
`management:<owner>` from `source:<source_id>`, provenance
`system:captures`, kind report, body `captures` = {`source_id`, `items`:
[{`capture_id`, `title`, `url`, `published_at`}]}. Record id
`capture-arrival:<source_id>:<24 hex>` = the first 12 bytes of
sha256(source_id, NUL, the sorted capture ids joined by NUL). It wakes
management; a poll with nothing new sends nothing. Management declines
captures with a Note to `source:<source_id>`, which wakes no one.

| Declaration | File | Digest |
|---|---|---|
| `CommitmentCaptures` | internal/types/story.go | sha256:6b067ef08c4b77a851d34b14d088c8c8d8141e6d8ad167384327f791a2e0b87c |
| `CaptureArrival` | internal/types/story.go | sha256:c54b562d3bf188486f4bdd5abd69d0412afc70d95b85fead907fa6fc1b243dcd |
| `CaptureArrivalRecordID` | internal/types/story.go | sha256:68dd58ee8c520f4d5816f2bf5cc173d81cc43671813749370faa6635842f71ff |
| `CaptureArrivalSenderInbox` | internal/types/story.go | sha256:e647ce52d0f0416b637e68df7a295be96124daa5918d0c516183097f3a034301 |

## 2. normtext

`normtext.Version` = `"nt1"`. The NFKC tables come from
`golang.org/x/text v0.37.0` (go.mod); a module upgrade that changes them
for captured characters is a version bump.

| Declaration | File | Digest |
|---|---|---|
| `Version` | internal/normtext/normtext.go | sha256:59d28b66c83a9a89c68719ed652740fc5b9cf3935621258cc5f38f7472d03010 |
| `Map` | internal/normtext/normtext.go | sha256:53c244eb639025a949e7e976c0f4a886e838b2b50ca7d77cc5a968e455eb8b9b |
| `foldRune` | internal/normtext/normtext.go | sha256:54799cbf07627ac1e91175cbac5b24077d61aadf122fe58e5fda3ce09267cf13 |
| `isInvisible` | internal/normtext/normtext.go | sha256:7d50e490140df2d4b6daff826fb210eb8456f129f7fd589a761d0a128cf1cd63 |

The quote matcher's version embeds it: `quotematch.Version` =
`"qm1+" + normtext.Version` (sha256:23def70239e0d83cf19be5a441e1ec36743626438ffde688f133bc995ae43255,
internal/quotematch/quotematch.go).

## 3. The cid formula

`cid = hex(sha256(kind + "\x00" + normtext.Normalize(text)))[:16]`; in
document order the first block with a cid keeps it and the k-th later
duplicate is `cid.k` (the second occurrence is `cid.1`).

| Declaration | File | Digest |
|---|---|---|
| `CIDLength` | internal/normtext/normtext.go | sha256:dd9736d62785671176273ddb60c78dfeddf2a1aef714ee1f8f83afbeb5ed9055 |
| `Block` | internal/normtext/normtext.go | sha256:6f173f6349f28b667947df7289ecda6219dd91042f989c7104d062385a072e95 |
| `CID` | internal/normtext/normtext.go | sha256:64eafd74ab59c9a9ea6582ae1acd3f4d9b06f7f313b3cbf8f8a2eb865bbe45f3 |
| `AssignCIDs` | internal/normtext/normtext.go | sha256:ccd1d730e9c1a4827ffb8e4391548bab8855bd5aa9f38ba606aedc08c7fd4f06 |

## 4. Extractor versions

| Version | Value | File | Digest |
|---|---|---|---|
| `ExtractorVersion` (HTML DOM walk) | `html1+nt1` | internal/sourcefetch/blocks.go | sha256:7f3681abea9fa9e53d35026bfc8a08ab2ceb1c47597f9425e9c3321bc0a8032b |
| `TextExtractorVersion` (unmarked text) | `text1+nt1` | internal/sourcefetch/blocks.go | sha256:710bdf7552b5ef6806865981fc491b6f254317026bfe513c0fd5e9ffd24fb95d |
| `PDFExtractorVersion` (per-page PDF text) | `pdf1+nt1` | internal/sourcefetch/blocks.go | sha256:ed53b2666b8fcbef1112b81a0fab1c1d8893b26e65967239cf96c80acf5df756 |

These digests cover the version names only. Tuning an extractor without
bumping its version re-addresses frozen citations; the finalizer should
also record the digest of each extractor's body (ExtractBlocks,
TextBlocks, PageBlocks) or the package's tree hash.

## 5. The Ref shape

`types.Ref` = {`doc`, `version`, `cid`, `hash`, `quote`}: one block
(`cid`, or `cid.k`) of one version (`version`, a capture's doc_hash or a
revision id) of one document (`doc`), with the block's content hash
(`hash`, `"sha256:" + hex(sha256(block text))`) and the quoted text.

| Declaration | File | Digest |
|---|---|---|
| `Ref` | internal/types/ref.go | sha256:39b0c24ec2d8ebcf8fb94ab5baa2379a3938f33cc58773760f8a0877aa67b261 |

## 6. The /v<n> route shape

`/pub/texture/<slug>` is the bare link and serves the latest version in
place; `/pub/texture/<slug>/v<n>` is version n, n a positive decimal with
no leading zero (`v0`, `v01`, `/latest` are not routes). n is the Texture
revision number, `types.RevisionNumber(version_number)` = version_number
+ 1. A story's seed revision is revision 1 and never publishes, so a
story's first public version is v2; a failed publish is a gap nav skips,
and nothing renumbers.

| Declaration | File | Digest |
|---|---|---|
| `parsePublicationRoute` | internal/platform/publication_versions.go | sha256:b1a271b5f0385594cd6550671372143ee55a3b356ea726eb6516a907d8b2660f |
| `versionPermalink` | internal/platform/publication_versions.go | sha256:57cbc23902ed45a9d6794c5f08602fd2855bcd951a1324cbd8a1503b44d7fdfa |
| `RevisionNumber` | internal/types/texture.go | sha256:585085c0fad911de0d6780d38c0779619183026f356150232b2cdcd9c3660941 |

## 7. The manifest schema

A version's manifest is `publicationManifest` marshaled once with schema
`choir.platform.publication_version_manifest.v1`; its hash is the sha256
of those bytes; it chains to the previous version by
`previous_manifest_hash` and never embeds past bodies. The immutable
payload is `PublicationVersionPayload` (text, manifest and its hash,
signatures, resolved_blocks); resolved blocks are stored under
`choir.platform.resolved_blocks.v1` and hashed into the manifest. Nav and
the check overlay are live fields outside the manifest. Signatures use
`choir.platform.version_manifest_attestation.v1`.

| Declaration | File | Digest |
|---|---|---|
| `publicationManifestSchema` | internal/platform/publication_versions.go | sha256:d21d1edbc2966f6ecf0ccff97958d5f55ac008d12552c98b56a298139c571589 |
| `publicationManifest` | internal/platform/publication_versions.go | sha256:c0a8486abaa15cfcd8d73768be69a8df132e59332e9b59172b647dc0dbb960ce |
| `PublicationVersionPayload` | internal/platform/publication_versions.go | sha256:d8a8537fa49326e1037e54246d648912334d1d63c53c7da5e670dc4867211f30 |
| `ResolvedBlock` | internal/platform/publication_versions.go | sha256:26d13d9790044c7f4413ad53185cac3f02fbfcb1413fe92856c1370b67577afa |
| `PublicationAnchor` | internal/platform/publication_versions.go | sha256:990cda8e78389404969ae9bf478ce93d0d92ea79e7bc1409b1d8ea29fbc75916 |
| `resolvedBlocksSchema` | internal/platform/publication_versions.go | sha256:fef49bd26ebce8ae56b87f0370b8b0b6ef4cf3b641bcc9f29d156c59de675af6 |
| `manifestAttestationSchema` | internal/platform/signing.go | sha256:6b745c3c639eb3669bb93fee64afedbe6a806f341f5cf956b662ab9ab4ed57c2 |

## Outside the freeze

The check overlay (`choir.publication_check`, `choir.support_judgement`
in Store B, `PublicationCheckOverlay`/`CitationCheck`) is live and served
beside the payload; it may change after the freeze without touching any
version. The guest routes (`/internal/platform/texture/publications`,
`/internal/platform/support-verdicts`) are internal and may change.

## Extractor bodies (added at finalization)

Whole-file sha256 at finalization; a change to any of these files is an
extractor change and bumps its version constant (section 4), so frozen
citations are never re-addressed by tuning.

| file | sha256 |
|---|---|
| internal/sourcefetch/blocks.go | 2eeede97a44f08719d05a2546b97bbd9b429e0d4be3e8972bc1d85fe5c35254e |
| internal/sourcefetch/readability.go | aa49d420a6b4d09aa20b5c200a0636bf2481b10270764a5e751b6ab05b9ad4c1 |
| internal/content/pdf_blocks.go | b8148737dae316d63e02a0fb2353d5abbcad5b3dc21582f4420c14980983743b |

## Decisions at finalization

- Digests: verified unchanged since b104f274 (above).
- Extractor bodies: whole-file digests (above).
- Seed numbering: kept. A story's empty seed is revision 1 and never
  publishes, so a story's first public version is v2; version n is always
  Texture revision n, and nothing renumbers. Orchestrator decision under
  the owner's delegation (§12), recorded here.

## Compatible extensions after the freeze

Each entry is appended. The FINAL tables above are never rewritten.

### 2026-10-15: L3-1, hypothesis kind and clock

Three optional fields on `CastHypothesis` (and on the folded
`HypothesisState`):
- `kind`, either `assertion` or `forecast`;
- `criterion`;
- `resolves_by`, RFC 3339.

The station is
docs/archive/definitions/choir-ledger-l3-hypotheses-settled-2026-10-13.md (L3-1).

| Declaration | File | Digest at the freeze | Digest after |
|---|---|---|---|
| `CastHypothesis` | internal/types/commitment.go | sha256:7758c7736068920db028973a567b80fbcd708322f45a1b40449d8044904e43de | sha256:70e5e7cc7dde1a9bfea21f593c0105447d94975ba0c3f6dfe124c113bc90841a |

`CommitmentCast`, `CastRevision` and every other declaration above are
unchanged. Their digests were recomputed with the same method, which
reproduces the frozen `CastHypothesis` value from the parent commit.

Why this is compatible:
- The fields are `omitempty`. A record committed without them serializes,
  hashes and replays byte-identically, so existing command digests and
  record bodies are unchanged.
- No existing record is backfilled.
- Admission validates the fields on new Casts only. The fold, replay and
  `VerifyInboxHeads` never run the time check.
- Heads decode with unknown fields ignored, so a revert still reads heads
  written with the fields.
