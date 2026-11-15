# capture-bodies: the reachability sweep reports first, deletion is admitted later (2026-11-14)

Status: open. Class of this document: green. The code it describes is orange
to red (capture bodies are source evidence; a deleted file is not recoverable
except from the nightly restic set). Related records:
[platform-artifact-retention-2026-10-22.md](platform-artifact-retention-2026-10-22.md)
(F7, "no collector"),
[node-b-storage-clustering-assessment-2026-11-11.md](node-b-storage-clustering-assessment-2026-11-11.md)
(A7, B5), and D5 of
[the storage solutions part 2 goal](../definitions/choir-storage-solutions-2-2026-11-12.md)
("30 days local, then archive; no delete without the owner").

## Problem

`capture-bodies/<2 hex>/<sha256>` has no collector and grows about 1.7 GiB a
day (260k files on 2026-11-10). Nothing reports which of its files no capture
row names. The first branch (`lane/capture-bodies-gc`) both made the sweep
count them and switched deletion on in the same landing; both review seats
returned REPAIR for that reason (the collector's own rule, `artifact_gc.go`
header: a lifecycle is admitted only after its report is observed on the live
store), and one found a race in the writer.

## What this landing does and does not do

- `capture-bodies` joins the sweep's reachability pass, **report-only**:
  `PLATFORM_ARTIFACT_GC_ACTIVE_NAMESPACES` on Node B (`nix/node-b.nix`) is
  unchanged and does not list it, and `DefaultArtifactGCActiveNamespaces` does
  not list it, so no environment becomes active for it by omission. `nix/` is
  not touched. This is the collector's existing per-namespace admission of an
  irreversible deletion, not a feature switch.
- This is the **orphan** sweep: bodies no row names, older than the grace
  (168 h on Node B). It is not the F7/A7/B5 retention rule (referenced raw
  blobs older than N days), which stays an owner decision and is not built.
  If the orphans turn out to be a few thousand files, the sweep will not
  stop the 1.7 GiB a day; that growth is referenced bodies (about three per
  capture) and needs the F7 decision.
- `PutBody` now refreshes a reused body under the shared CAS reuse lock and
  returns a refresh error.

## Which stores the live set reads, and why no reader can name a body it calls unreachable

The live set is every `sha256:<64 hex>` string, at any depth of the metadata,
of every `choir.web_capture` row in `og_objects`, tombstoned and superseded
rows included, read from `s.store.corpus()` (Store B), the store the writer
writes and every reader reads (the same accessor the og sweep uses since the
2026-10-05 wrong-store deletion).

Writers of capture bodies: one. `sourcegraph.WriteCaptures`
(`internal/sourcegraph/captures.go:194-215`) calls `CaptureBodyStore.PutBody`
for the raw bytes, the block JSON and the text projection, then creates the
row with `CreateWebCaptureIfAbsent` through `ObjectGraphStore` (Store B). It
is reached from `HandleCaptures` (corpusd, host side; guests and the paper
computer post items over HTTP and never touch the directory). Bodies exist
before their row; the grace (mtime) covers that gap, and a reuse of an old
body refreshes its mtime under the reuse lock.

Readers, and where each gets the body id:

| Reader | Id comes from |
| --- | --- |
| `HandleCaptureRead` (`captures.go`) | the row's `extracted_text_blob_id`, `blocks_blob_id`, `content_blob_id` |
| publish pin (`publication_versions.go:471`) | the row's `blocks_blob_id`; the text is copied by value into the version, so a published object does not depend on the body afterward |
| `HandleCaptureBody` (by hash) | a hash the caller got from a row (`content_blob_id`) |
| `researchtools` `OpenCaptureBody`, agentcore engineering inputs (`engineering_inputs.go:117-137`), Texture captures (`texture_captures.go:90`) | the row's metadata, via corpusd |
| capture index (`capture_index.go`) | reads metadata only, no body |

Every reader derives its id from a Store B row, so a body no row names is a
body no reader can ask for, except by guessing a hash (`HandleCaptureBody`),
which after a hard delete of the row is a capture already unreadable. Other
stores: the paper's and other computers' stores hold no capture body ids
(captures live only in host Store B; `choir.artifact_blob.storage_ref` never
points into `capture-bodies`; publications pin by value). `scripts/cutover/
keepset-snapshot.sh` counts the same three fields and is the independent
read.

Not covered, named plainly: a restore of Store B from a backup older than a
body's grace, or a replay that rebuilds `og_objects` while a sweep runs, would
make live bodies look unreachable. Active admission must not happen during or
just after a Store B restore (a restored restic tree keeps old mtimes).

Guard beyond "empty live set": a query error or a row whose metadata does not
parse sets the namespace's live set to nil; nothing in it is classified or
made eligible, the report says `live_set_incomplete: true` and warns
`capture-bodies live set: ... (namespace not swept)`, and the other namespaces
still sweep. A directory listing error already keeps the namespace unswept.
The empty-live-set guard stays.

## Admission procedure for the later activation (an owner act)

Not before the owner says so on the record (D5: no delete without the owner).

1. Read the report. On Node B: `journalctl -u go-choir-corpusd --no-pager |
   grep 'artifact gc:'`, the last line, its `report=` JSON,
   `namespaces."capture-bodies"`. Read at least three consecutive hourly
   passes across a deploy and a quiet hour.
2. Required, in every pass read: `live_set_incomplete` absent; `errors` 0;
   `mode` `dry-run`; no warning containing `capture-bodies`.
3. The comparison of live against the store: `scanned` = `live` +
   `unproven` + `in_grace` + `eligible`, and `live` + `named_missing_on_disk`
   = `named_by_rows` (the distinct body ids the rows name; `doc_hash`, a
   `sha256:<hex>` that names no file, is excluded from the walk by key at any
   depth, so it does not inflate this; with the real writer
   `named_missing_on_disk` is 0, a test pins it). Cross-check once
   by hand: the distinct ids `scripts/cutover/keepset-snapshot.sh` counts
   equal `named_by_rows`, and `find $ART/capture-bodies -type f -name
   '[0-9a-f]*' | wc -l` equals `scanned`. `named_missing_on_disk` should be
   near zero (a large number means rows name bodies that are gone, a
   different problem: record it first).
4. Make it safe: `eligible` and `eligible_bytes` at the magnitude the
   measurements predict (the 2026-10-30 measurement: 3.8k of 154.6k files
   outside the referenced set; a count far above that is a reason to stop
   and find out why, not to delete); `unproven` 0; `eligible_sample` ages all
   above the grace (they are), the youngest not clustered just past it (a
   cluster there means a writer that lands its row late). Spot-check five
   sampled ids: no row names them (`SELECT 1 FROM og_objects WHERE
   object_kind='choir.web_capture' AND metadata LIKE '%<id>%'` is empty).
5. Confirm a restic set is newer than the oldest eligible id and that no Store B
   restore ran in the grace window.
6. Activation, a separate landing: add `capture-bodies` to
   `PLATFORM_ARTIFACT_GC_ACTIVE_NAMESPACES` in `nix/node-b.nix` (nix eval of
   both systems), citing the report read and the owner's word. The first
   active pass is bounded by the existing `MAX_DELETES` and `MAX_BYTES`; read
   its `deleted` against the dry-run `eligible`, then spot-read one capture
   (`HandleCaptureRead`), one raw body and one publish that cites a capture.
7. Rollback: none for deleted files. Removing the namespace from the list or
   reverting stops further deletion; deleted orphan bodies come back only from
   the nightly restic set of `platform-artifacts/capture-bodies`. The
   deletion log `artifact-gc-log/<day>.jsonl` names every path.

## Before activation: two traps in the admission itself

- `PLATFORM_ARTIFACT_GC_ACTIVE_NAMESPACES=all` parses to a nil list, and nil
  means every namespace participates (`artifact_gc.go`, the `participates`
  test in `RunArtifactGC`). Setting `all` would activate `capture-bodies`
  and `og` together. It is not changed in this landing; never set `all` on a
  host before the admission above is done, and read the parsed list, not the
  intent.
- The eligible count assumes a capture's row is inserted within the grace
  window of its `PutBody` (bodies are written before the row). A row that
  lands later than the grace after its bodies (a stalled writer, a restore)
  would leave live bodies counted eligible. Check `eligible_sample` ages and
  the writer's lag before admitting.
