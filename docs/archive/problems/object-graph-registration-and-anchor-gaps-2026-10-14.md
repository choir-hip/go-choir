# Object graph: kind registration is not a write gate, a dead kind, and quote anchors drop their span, 2026-10-14

Status: repaired on branch, not yet deployed: H-OG2 in 001aaf4d8, H-OG1 in
74a2456e6, H-OG3 in 7287ca3a7 (see "Repairs" at the end).
Source: the claims, events and entities design survey
(`docs/reports/claims-events-entities-ontology-2026-10-14.md` §2, heresies
H-OG1, H-OG2 and H-OG3). Owner, 2026-10-14 ~15:00 UTC: "we definitely need to
fix those gaps pursuant to really improving it with this intermediate layer,
which is really the core of the system."

Mutation class of the fixes: **red**. They touch object-graph writes on both
durable stores (the VM's Dolt store, including Texture's workspace, and
corpusd's Store B, including the publication path) and the citation evidence
a Texture revision and a published version carry.

## H-OG1. The registry is consulted on one write path only

### Evidence (read from code at da9de233f)

`objectgraph.Registry` (`internal/objectgraph/registry.go`) is looked up only
by `objectgraph.Service` (`internal/objectgraph/service.go`: `newObject`,
`GetObject`, `ListObjects` with a kind, `PutEdge`). Every other write reaches
SQL without asking it:

| Physical writer | Write methods | Registry check |
|---|---|---|
| `objectgraph.DoltStore` (VM runtime store and Texture workspace) | `PutObject`, `PutBatch`, `PutBatchConditional`, `PutEdge` (`dolt_store.go:214,533,540,459`) | none; only the optional vocabulary `WriteValidator` and the projection interceptor |
| `platform.ObjectGraphStore` (corpusd, Store B) | `PutObject`, `InsertObjectIfAbsent`, `PutBatch`, `PutEdge` (`internal/platform/objectgraph_store.go:99,142,428,247`) | none |
| HTTP `PUT /internal/platform/objects` and `/edges` (`objectgraph_handlers.go:200-225,290-310`) | forwards to `platform.ObjectGraphStore` | none |
| Event projector (`internal/store/project.go:210,250`) | replays `object`/`object_edge` operations into SQL | none (correctly: replay must not refuse history) |
| Vocabulary migrations (`internal/store/vocab_migrate_og.go:768,1008`) | rewrite existing rows in place | none (correctly: they rename identity, never kind) |

The VM store writes almost everything through `ogStore.PutBatch` /
`PutBatchConditional` / `PutObject` directly (`internal/store/*.go`, 30 call
sites), so most of the graph never meets the registry.

### Kinds written today that the registry does not know

A source scan of every non-test Go file under `internal/` and `cmd/` for kinds
named in a kind-typed position (const or var typed `ObjectKind`/`EdgeKind`,
`ObjectKind("…")` conversions, `Object{ObjectKind: "…"}`,
`Edge{Kind: "…"}`, `BuildCanonicalID("…", …)`, and literal arguments to
functions whose parameter is a kind, such as `makeEdge`) finds **26 object
kinds** and **12 edge kinds** with live writers and no registration. The
report counted "at least 22"; the scan found four more.

Object kinds (writer):

| Kind | Writer |
|---|---|
| `choir.actor_wake_outbox` | `internal/store/lifecycle.go:851` (VM) |
| `choir.capture_disposition` | `internal/store/management_tasks.go:542` |
| `choir.citation_check` | `internal/store/support.go:119` |
| `choir.co_super_assignment` | `internal/store/engineering_assignments.go:1138,1467` |
| `choir.co_super_assignment_report` | `internal/store/engineering_assignments.go:1928,2070` |
| `choir.co_super_subject_candidate` | `internal/store/engineering_assignments.go:1885` |
| `choir.co_super_capability_claim` | `internal/store/engineering_assignments.go:1481` |
| `choir.co_super_capsule_claim` | `internal/store/engineering_assignments.go:1488` |
| `choir.co_super_run_claim` | `internal/store/engineering_assignments.go:1474` |
| `choir.commitment_record` | `internal/store/lifecycle_commit_act.go:307,562` |
| `choir.conversation_head` | `internal/store/inbox.go:530` |
| `choir.inbox_command` | `internal/store/inbox.go:650` |
| `choir.inbox_cursor` | `internal/store/inbox.go:514` |
| `choir.inbox_head` | `internal/store/inbox.go:523` |
| `choir.lifecycle_cancel_intent` | `internal/store/lifecycle.go:5625` |
| `choir.lifecycle_command` | `internal/store/lifecycle.go:1494` |
| `choir.lifecycle_event` | `internal/store/lifecycle_commit_act.go:404`, `lifecycle_control_issue.go:479`, and others |
| `choir.lifecycle_sequence` | `internal/store/lifecycle.go:139` |
| `choir.management_task` | `internal/store/management_tasks.go:532` |
| `choir.migration_marker` | `internal/store/lifecycle.go:1161` |
| `choir.scheduler_sequence` | `internal/store/lifecycle.go:60` |
| `choir.story_library_head` | `internal/store/management_tasks.go:629` |
| `choir.support_verdict` | `internal/store/support.go:180` |
| `choir.publication_check` | `internal/platform/publication_checks.go:97` (corpusd) |
| `choir.support_judgement` | `internal/platform/publication_checks.go:127` (corpusd) |
| `choir.publication_resolved_blocks` | `internal/platform/publication_graph.go:162-168` (corpusd) |

Edge kinds (writer):

| Kind | Writer |
|---|---|
| `co_super_assignment_trajectory`, `co_super_assignment_parent`, `co_super_assignment_parent_run`, `co_super_assignment_parent_work`, `co_super_assignment_agent`, `co_super_assignment_work`, `co_super_assignment_run`, `co_super_report_assignment`, `co_super_report_candidate` | `internal/store/engineering_assignments.go:33-41` (VM) |
| `commitment_record_packet` | `internal/store/lifecycle_commit_act.go:23,388,718` (VM) |
| `has_resolved_blocks` | `internal/platform/publication_graph.go:177` (version → resolved blocks, corpusd) |
| `is_version_of` | `internal/platform/service.go:552` (citation relation type, becomes an edge kind at `publication_graph.go:594`) |

Two of these are dynamic and invisible to a literal scan:
`publication_graph.go:594` makes a citation edge of kind
`EdgeKind(cite.RelationType)`. Today the only relation types set are
`references` (registered) and `is_version_of` (not), both in
`internal/platform/service.go:540,552`.

### Registered and never written

| Kind | State |
|---|---|
| `choir.universal_wire_story_cluster` | H-OG2 below |
| `choir.media_item`, `choir.audio_recording`, `choir.transcript`, `choir.autoradio_run_sheet` | no writer in any commit's live code; named by current Autoradio and Autopaper media research (`docs/research/autopaper-media-and-rights-2026-10-14/repo-media-survey.md`, `docs/archive/research/puter-everything-2026-10-14/media.md`) as the kinds to reuse |

### Staging evidence

No existing API returns a count of every kind. `GET /api/runtime/census`
(`internal/agentcore/census_surface.go`) lists only
`vocabmigrate.RetiredWorkStateKinds`; `GET /internal/platform/objects` is
internal-only; querying the platform DB directly is out of bounds. The
inventory above is therefore from code only. Historic rows of kinds no
writer names any more stay readable: the gate the fix adds refuses new
writes, not reads, replay or migrations.

### Impact

- A typo or a new kind reaches durable storage with no owner, no identity
  mode and no store routing on record. Nothing tells a reader of
  `og_objects` which kinds are legitimate.
- The claims layer (`docs/reports/claims-events-entities-ontology-2026-10-14.md`
  §12.3, E1 entry condition) needs "the new kinds registered and written
  only through typed corpusd routes with validation". Without a gate,
  registration is a convention, not a check.

## H-OG2. `choir.universal_wire_story_cluster` is registered and never written

### Evidence

- Citers: `internal/objectgraph/registry.go:54` (registration),
  `internal/objectgraph/web_capture.go:16-17`
  (`UniversalWireStoryClusterObjectKind`, `UniversalWireStoryClusterSchemaVersion`,
  no other Go reference), `internal/objectgraph/objectgraph_test.go:46`
  (the registry list test), `internal/vocabmigrate/families.go:192` (frozen
  decode table), and one doc,
  `docs/archive/definitions/choir-wire-kb-graph-schema-w0-2026-10-01.md:15`.
- History: the kind was written by the deterministic clustering slice
  (5efdcd453, 338632879, 8121b4d41); its writer was deleted in 4b5c665bb
  ("delete Universal Wire deterministic-scaffold heresies"). Since then no
  code writes or reads it.
- "Universal Wire" is a retired name for Autopaper (CLAUDE.md).

### Impact

A dead registration that names a retired product. Old computers may still
hold rows written before 4b5c665bb, so the frozen vocabulary decode table
must keep the string; only the write registration and the constants go.

## H-OG3. A quote anchor drops the span the matcher found

### Evidence

- `quotematch.Match` (`internal/quotematch/quotematch.go:80-137`) returns
  `Result{BlockID, Start, End, Excerpt, Spans}`: the block and the byte
  span of every ellipsis segment in the block's original text.
- Write keeps only part of it (`internal/texturewrite/apply.go:600-609`):
  `ref.CID = match.BlockID`, `ref.Quote = match.Excerpt`,
  `ref.Hash = blockHash(...)`, and the tier. `Start`, `End` and `Spans` are
  discarded. `types.Ref` (`internal/types/ref.go`) has no field for them,
  and `refAttrValue` (`apply.go:735`) writes only doc, version, cid, hash
  and quote into the `source_ref` node.
- The Ref flows unchanged into the published version's anchors
  (`internal/platform/publication_versions.go` `PublicationAnchor.Ref`,
  manifest `anchors[].ref`) and into the check overlay (`CitationCheck.Ref`).
  The resolved block the reader shows is the same text the matcher matched
  (`resolveAnchors` refuses a Ref whose `Hash` differs from the pinned
  block's text hash).
- The reader re-matches (`frontend/src/lib/texture-published-citations.ts`
  `splitAtQuote`): `text.indexOf(quote)`, then a lowercase `indexOf`.

### Impact

- A quote whose words occur twice in a block is highlighted at the first
  occurrence, not necessarily the one the matcher verified.
- An ellipsis quote (`Excerpt` is the segments joined by " ... ") is never a
  substring of the block, so the reader shows it unmarked, as a standalone
  quote above the block.
- Every consumer that wants the exact words (the claims layer's mention
  and claim spans, §5 of the survey) must run the matcher again.

## Fix plan (each in its own commit, after this one)

1. **H-OG1, single authority.** `objectgraph.DefaultRegistry` stays the one
   list of kinds. One function, `objectgraph.CheckWrite(objects, edges)`,
   refuses a non-tombstone object or edge whose kind is not registered. It
   runs at the top of every mutation method of both durable stores
   (`DoltStore` before the validator and the interceptor, so nothing
   unregistered reaches the event tape; `platform.ObjectGraphStore` before
   SQL), which is the boundary every write path above passes, including the
   HTTP PUT routes and `Service`. Replay, migrations and deletes stay
   ungated. Register the 26 object kinds and 12 edge kinds with an owner
   (the writing package). Mark the four Autoradio kinds `Reserved`. Add a
   standing test that scans production source for kind literals and fails
   on any unregistered one, and on any registered kind that has no writer
   and is not reserved.
2. **H-OG2.** Delete the registration and the two constants; keep the
   frozen decode-table entry with a comment, because historic rows may
   exist.
3. **H-OG3.** Add an optional `span` to `types.Ref`: start and end in Unicode
   code points of the cited block's text (the unit the existing
   `TextPositionSelector` uses, `internal/platform/service.go:401`), one
   range per ellipsis segment, and `text_hash`, the sha256 of the covered
   text. Write fills it from `quotematch.Result`; `refAttrValue` persists it
   on the `source_ref` node, so it reaches published anchors and the
   overlay unchanged. Old Refs have no span and read as before. The reader
   marks the span when it is present and its text agrees with the block,
   and falls back to `splitAtQuote` otherwise.

### Failure modes to test before code

H-OG1:
- F1. A live writer's kind is missed and its writes start failing (boot
  migrations, lifecycle commits, publication). Guard: the source scan test,
  plus an explicit check of the two dynamic citation relation types.
- F2. The gate sits after the interceptor, so an unregistered object still
  lands on the event tape. Guard: test that a refused write leaves the
  interceptor uncalled.
- F3. Replay of an old tape that carries a now-unregistered kind fails at
  boot. Guard: the projector stays ungated; a test projects such an object.
- F4. Retiring (tombstoning) a historic object of an unregistered kind is
  refused. Guard: tombstones pass the gate.
- F5. One durable store is gated and the other is not. Guard: refusal tests
  on `DoltStore` and on `platform.ObjectGraphStore`, each write method.
- F6. The scan silently finds nothing (a path or parser change) and passes.
  Guard: the test fails when it finds fewer than a floor of kinds.

H-OG2:
- F7. A computer holding historic cluster rows fails to decode them. Guard:
  the frozen decode table keeps the string.

H-OG3:
- F8. Offsets in bytes are read as characters (or the reverse), and the
  highlight lands on the wrong words for non-ASCII text. Guard: a test with
  multi-byte text before the quote, checked by slicing code points.
- F9. A tolerant match's span is computed from the normalized text rather
  than the original. Guard: quotematch spans index the original block
  text; the test slices the original and compares with the excerpt.
- F10. Old Refs without a span break decoding or rendering. Guard: span is
  optional (`omitempty`); a test decodes an old Ref.
- F11. An ellipsis quote's segments are lost. Guard: one range per segment;
  test with an ellipsis quote.
- F12. A carried citation loses its span on a later Write. Guard: carried
  citations copy the node's attrs whole (`writer.citation` existing path);
  test that a second Write keeps the span.
- F13. The reader trusts a stale span over a changed block. Guard: the
  reader uses the span only when the code-point slice equals the quote
  segment; otherwise it falls back.

## Belief state

- Confident: the write gate gap, the unregistered kind list (from code),
  the dead cluster kind, and the dropped span.
- Not known: the full set of kinds present on staging (no API). Rows of
  unregistered historic kinds would only matter if code re-wrote them,
  and no code reads or writes them.

## Remaining error field

- Dynamic kinds a source scan cannot see: citation relation types. The
  gate refuses them at the store, which is fail-closed by design, so an
  unseen relation type would fail a publish rather than land unregistered.
- The four reserved Autoradio kinds stay registered without a writer until
  Autoradio or Autopaper media writes them or a later pass deletes them.

## Repairs (branch, 2026-10-14)

- **H-OG2** (001aaf4d8): registration and the two constants deleted; the
  frozen decode table keeps the string, marked retired.
- **H-OG1** (74a2456e6): `objectgraph.CheckWrite` against
  `DefaultRegistry` runs first in every write method of `DoltStore` (before
  the validator and the interceptor) and of `platform.ObjectGraphStore`.
  26 object kinds and 12 edge kinds registered; every kind names an Owner;
  the four Autoradio kinds are `Reserved`. Standing tests:
  `TestEveryWrittenKindIsRegistered`, `TestEveryRegisteredKindHasAWriter`,
  `TestCitationRelationEdgeKindsAreRegistered`, the Dolt and Store B gate
  tests, and the replay test. Full `internal/agentcore` and
  `internal/textureowner` suites pass at this commit.
- **H-OG3** (7287ca3a7): `types.Ref.Span` (code points, segments,
  `text_hash`), filled by Write from `quotematch.Result`, carried into
  published anchors; the reader marks it through `splitAtSpan` and falls
  back to `splitAtQuote`.

Not done here: deploy and staging proof (the branch is not pushed); a
staging count of kinds (no API); the deployed reader check of a span
highlight, which needs a story published after the deploy.
