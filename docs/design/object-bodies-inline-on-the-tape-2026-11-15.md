# Object bodies inline on the tape

Date: 2026-11-15. Class: green (a design; no code, no tests, nothing compiled
or run). Station: the store-once goal, lane `inline-body-design`.
Problem record:
[tape-object-ops-carry-more-than-twice-their-payload-2026-11-15.md](../problems/tape-object-ops-carry-more-than-twice-their-payload-2026-11-15.md).
Related: [carry-by-reference round 3](../problems/carry-by-reference-design-three-review-rounds-structural-assessment-2026-11-15.md).

Every `file:line` below was read on the commit this branch was cut from
(`origin/main` at 718ea5ed4). Nothing was measured on a live computer. Items
marked **(invented)** are proposals with no code behind them; items marked
**(estimate)** are arithmetic on stated inputs.

## 0. The one mechanism, in one paragraph

A new object op carries the object's body as raw JSON in a new field
`body_json` instead of base64 in `body`, when and only when the body passes a
deterministic eligibility test (valid JSON, valid UTF-8, already in the exact
bytes the encoder would emit, no secret the redactor would touch). A batch
that holds at least one such op is written as **batch version 3**, which every
binary that predates the reader refuses at `Validate`. The reader is
`objectgraph.Object.UnmarshalJSON`, which accepts either field forever. The
reader ships first with store epoch 7; the writer ships second and raises the
store floor to 7 before its first inline write and whenever any binary applies
a version 3 batch. Old tape is never touched.

## 1. What the code does today (anchors)

- The type. `objectgraph.Object` has `Body []byte` tagged `json:"body,omitempty"`
  and `Metadata json.RawMessage` (`internal/objectgraph/object.go:23-36`).
  `encoding/json` writes `[]byte` as base64. `Edge` has no `[]byte`
  (`object.go:38-46`).
- The content hash. `ContentHash(kind, body, metadata)` marshals a struct whose
  `Body` is `[]byte` and hashes the result (`object.go:97-110`). So the hash
  input is the base64 of the body **bytes**, plus the kind and the normalised
  metadata. It is a function of the bytes, not of any tape encoding.
  `NormalizeMetadata` (`object.go:117-`) re-marshals metadata through
  `any`, which sorts keys; that concerns metadata, which this design does not
  touch.
- Every writer that marshals an object into a tape op (all use plain
  `json.Marshal(obj)`):
  1. `interceptOG`, `internal/store/projection_tape.go:105-116` (all ordinary
     object writes, including the lifecycle fold at `lifecycle.go:3513-3519`);
  2. `eventObjectOp`, `internal/store/event_table.go:645-656` (stamped
     acknowledged events and the telemetry sealer, `telemetry_sealer.go:126-139`);
  3. residue import, `internal/store/residue_import.go:126-130`;
  4. the deposit upcaster re-encodes an object **in memory at replay time**
     (`internal/store/vocab_upcast_deposit.go:663`); it rewrites no tape.
  Two other marshals of an `Object` are not tape ops: the guest-to-platform
  `HTTPStore.PutObject` (`internal/objectgraph/http_store.go:45`) and nothing
  else I found in `internal/agentcore` (`runtime.go:4063` marshals a payload
  map, not an object).
- Every reader that decodes an object op (all `json.Unmarshal` into
  `objectgraph.Object`, so one type-level reader serves them all):
  `projectObject` (`internal/store/project.go:242`), `decodeChoirEventRow`
  (`event_table.go:90`, and the body is then unmarshalled into an
  `EventRecord` at `event_table.go:101`), the telemetry partition projector
  (same decode path), `recheckTelemetryOp` (`telemetry_sealer.go:548`, then
  `json.Unmarshal(obj.Body, &ev)` at 554), the upcaster
  (`vocab_upcast_deposit.go:560`), and residue scanning
  (`residue_import.go:588`, a SQL scan, not a tape decode). Platform-side:
  `internal/platform/objectgraph_handlers.go:235`, `objectgraph_store.go:918`,
  `publication_oped.go:124` decode HTTP objects, not tape batches.
- Readers that look at an op and do not need the body: the ledger witness
  walker (`internal/ledgerwitness/walk.go:147-165`) reads only `object_kind`
  and `metadata`; the replay fence (`internal/store/vocab_replay_fence.go:121-137`)
  reads only `canonical_id`; the group commit sizes ops by `len(op.Body)`
  (`tape_group_commit.go:140-146`). None reads `body`.
- Batches. `ProjectionOp.Body` is `json.RawMessage`
  (`internal/computerevent/projection_batch.go:30-35`), so the object JSON sits
  inline in the batch already; only the object's own `Body` is base64.
  The only constructor of a production batch is
  `internal/store/projection_tape.go:212-221` (version 2, projector 2); the
  other is the "unreadable" stub at `internal/computerevent/appender.go:1801`.
  `Validate` accepts exactly (1,1) and (2,2) and refuses everything else with
  `ErrProjectionBatchInvalid` "unsupported version"
  (`projection_batch.go:366-370`). `DecodeProjectionBatch` is plain
  `json.Unmarshal`, no unknown-field refusal (`projection_batch.go:414-423`).
- What a refusal does on replay. The replay batch decode (`appender.go:1788-1815`, function name not read)
  turns a `json.SyntaxError` on a sealed batch into "applied as no change"
  (`ErrProjectionBatchUnreadable`), but any other error, including a version
  refusal, is returned (`appender.go:1805-1807`). A version refusal is
  therefore loud. It is not a `SyntaxError`, so it cannot take the silent path.
- The floor. `WriteFloor` never lowers, writes durably
  (`internal/storeschema/floor.go:70-107`); `CheckFloor` refuses a store whose
  floor is above the build epoch, at `Open` before any DDL
  (`floor.go:109-120`, `internal/store/open.go:219-225`); the updater refuses
  `Apply`, `restorePrior`, `RestagePinnedRelease` to a release below the floor
  or declaring no epoch (`internal/updater/updater.go:552-612`). The current
  epoch is 6 (`internal/storeschema/schema.go`, `const Version uint64 = 6`).
- The bodies. Kinds I could find with non-JSON bodies: platform-side
  `choir.review_record` (`internal/platform/publication_graph.go:255-260`, the
  text `v0 owner consent publication path`), transcript snapshot text
  (`publication_graph.go:534`), publication content
  (`publication_graph.go:786`). These are platform objects, not guest tape ops,
  but the code path does not know the difference. Guest bodies are mostly
  `json.Marshal` output (`buildEventObject`, `graph_store.go:1274`;
  `capture_label.go:217`). I did **not** enumerate every guest kind's body
  encoding; the design therefore must not assume it (section 3).

## 2. The wire form

### Choice: `body_json` beside `body`, plus batch version 3

Two candidates:

- **A. Additive field `body_json` only, no version.** Smallest, but an old
  binary ignores the unknown field, decodes an object with an **empty body**,
  and projects it into `og_objects` (this is exactly the failure the `table`
  stamp had, `internal/store/event_table_old_binary_test.go:15-50`, and
  `choir_events` rejects `body is not an event record`). That is a silent
  misread for every kind except events. Refused.
- **B. Batch version 3 in which every object body is `body_json`.** Refuses
  loudly on old binaries, but then non-JSON bodies have no home, and "old
  field absent" would make a mixed-form batch impossible.
- **C (chosen). Both.** The object carries `body_json` when eligible and
  `body` otherwise; the batch is version 3 exactly when it holds at least one
  `body_json`. Version 3 is the loud fence on old binaries; the per-object
  field is the carrier; a non-JSON body keeps `body` in the same op.

Why C: A is unsafe alone; B cannot carry non-JSON; C is B's safety with A's
fallback. A batch with no inline body stays version 2, so ops that cannot or
need not change do not raise the fence.

Wire shape of an inline object (key order is Go's struct order; the only
change is the one field):

```
{"canonical_id":"...","object_kind":"...","owner_id":"...","content_hash":"sha256:...",
 "body_json":{"event_id":"...", ...},"metadata":{...},"created_at":"...","updated_at":"..."}
```

Exactly one of `body`, `body_json` may be present and non-empty. Both present
is a decode error (the reader refuses; it never prefers one).

### The reader, accepted forever (new)

`objectgraph.Object` gets an `UnmarshalJSON` that decodes through an alias
struct with an extra `BodyJSON json.RawMessage `json:"body_json,omitempty"``
(**invented**). If `body_json` is present, `Object.Body` is set to a copy of its
raw bytes; if `body` is present it is the base64 as today. Because every
decoder listed in section 1 unmarshals into `objectgraph.Object`, no per-site
change is needed, and the platform HTTP decoders also accept the form
harmlessly. There is no setting. The reader never goes away: tape is
read forever.

### The writer (new)

`objectgraph.MarshalTapeObject(obj) (encoded []byte, inline bool, err error)`
(**invented**) is the only change at the three writers (`projection_tape.go:111`,
`event_table.go:646`, `residue_import.go:126`). `Object`'s own default
marshalling is **not** changed: `HTTPStore.PutObject` (`http_store.go:45`) and
anything else keep emitting `body`, because an older platform binary behind
the HTTP path would misread `body_json`. The upcaster keeps `json.Marshal`
(in memory, replay only).

`ProjectionOp` gets a Go-only field `InlineBody bool `json:"-"``
(**invented**) set by the writer. The batch constructor at
`projection_tape.go:212-221` sets `Version: 3, ProjectorVersion: 2` iff any
op has it. `Validate` accepts (3,2) in the reader release. (The projector did
not change, so its version stays 2; a (3,3) pair would work equally, the pair
is a one-line choice for the author of the code.)

No switch: the writer release always uses the new form for every eligible
body.

## 3. Byte-exactness

### What must round-trip

Objects are identified and judged by `ContentHash`, which hashes the base64 of
`Body` bytes (section 1). Everything downstream (`og_bodies`, the canonical
witness, the upcaster that recomputes hashes) works on the decoded bytes.
Therefore the invariant is: **the bytes of `Object.Body` after decoding an op
are identical to the bytes before encoding.** If that holds, every hash, row
and witness is identical in either form, with no change to any hash code.

### Carry the body verbatim, do not re-encode

Re-encoding (decode the body into `any`, marshal it back) would sort object
keys and change number text and so change the bytes: refused. The body is
carried as `json.RawMessage`, which the decoder copies verbatim
(`json.Unmarshal` into a `RawMessage` stores the exact bytes of the value).

### The trap: `json.Marshal` compacts and escapes a `RawMessage`

`encoding/json` runs `compact(..., escapeHTML=true)` on the output of a
`RawMessage`. That removes insignificant whitespace and rewrites `<`, `>`,
`&`, U+2028, U+2029 to `\u00XX` escapes. A body that contains any of those in
raw form would come back with different bytes, and a different hash. Today's
bodies mostly come from `json.Marshal`, whose output is already compact and
escaped, so they survive; that is a fact about the producers, not a property
the encoder guarantees.

### The deterministic eligibility test (invented)

A body `B` (non-empty) is written as `body_json` iff all of:

1. `utf8.Valid(B)` and `json.Valid(B)`;
2. `bytes.Equal(json.Marshal(json.RawMessage(B)), B)`: the encoder would emit
   exactly `B`, so nothing is changed by compaction or escaping;
3. `B` is not the literal `null` (a `null` value is ambiguous with an absent
   field to a reader);
4. no match of the redactor's secret patterns in the candidate op bytes
   (section 3.2).

Otherwise the op keeps `body` (base64) exactly as today, in the same batch.
The decision is a pure function of `B` and of the pattern set, so two
writers given the same object choose the same form, and a re-run of the same
write is deterministic. The writer also decodes its own encoded op
(`UnmarshalJSON`) and compares `Object.Body` to `B` before it appends; a
mismatch writes the base64 form and counts a `inline_self_check_failed`
(a correctness refusal, not a switch).

### 3.1 Bodies that are not JSON or not UTF-8

Plain text and any binary (the three platform kinds above, any media body,
anything `[]byte(string)`) fail test 1 and keep base64. Whether the guest
holds a kind whose body is not JSON, I did not establish; the design does not
depend on the answer. A measured per-kind count of `inline` against
`base64_fallback` (section 6) will say which kinds fall back and why.

### 3.2 Redaction sees inline bodies; base64 hides them

The sealed path redacts the batch plaintext before compressing
(`internal/computerevent/privacy.go:310-350`; `redactJSONPayload`,
`internal/computerevent/redaction.go:183-280`). A base64 body is opaque to
the secret patterns (`redaction.go:30-45`: bearer tokens, `api_key=`, `sk-`,
`ghp_`, PEM headers). An inline body is not. Without a guard, a body
containing `token: abcdefgh12345` would be rewritten on the tape while the
live store keeps the original: replay would then produce a different body and
a different hash than the live store, and the replay-completeness probe would
fail. Test 4 above sends any body whose candidate op would be touched by the
redactor to base64, which is today's behavior exactly. (Side finding, not
fixed here: today a secret inside a body is **not** redacted on the tape,
because base64 hides it. This design does not change that for bodies it
refuses; it is a pre-existing gap that wants its own record.)

The test needs the redactor's matcher exported from `computerevent`
(`findSecretMatches`, `redaction.go:328`; **invented** wrapper).

### Proof (what the tests must show; section 8)

For every fixture body `B` and both forms: `decode(encode_inline(B)).Body == B`
bytewise, `decode(encode_base64(B)).Body == B`, hence
`ContentHash(kind, decode(...).Body, meta)` is equal in the two forms. A
property test over generated bodies (whitespace, `<>&`, U+2028, lone
surrogate escapes, deep nesting, big numbers, duplicate keys, BOM, invalid
UTF-8, empty, `null`, bare scalars) must show that every ineligible body falls
to base64 and every eligible one round-trips.

## 4. Old binary on new tape: refuse loudly, never misread

Three fences, each independent of the others:

- **F1, in the tape (survives every workspace swap).** A version 3 batch fails
  `Validate` on every binary without the reader (`projection_batch.go:366-370`),
  on replay, catch-up, receipts-driven reads and the dry-run probes alike. The
  error is returned, not downgraded (`appender.go:1805-1807`). A binary that
  never learned `body_json` cannot apply a batch that has one without first
  failing this check. This is the fence that does not depend on a file.
- **F2, the store floor (pre-mutation, updater-visible).** Floor 7 refuses
  `Open` of the store by a build whose epoch is below 7 (`CheckFloor`) and
  refuses the updater's `Apply`/`restorePrior`/restage to such a release.
- **F3, content.** A binary that reads an `Object` with `body_json` and has no
  reader sees no body field: only F1 prevents this from being silent, which is
  why the version is not optional.

### Epochs (the two releases)

- Release 1 (reader): `storeschema.Version = 7`, `min_store_schema_version`
  6, `Validate` accepts (3,2), `Object.UnmarshalJSON` accepts both forms.
  **It writes no floor and no version 3 batch.**
- Release 2 (writer): `MarshalTapeObject` on the three writers; version 3
  constructor; floor 7 assertion. Epoch stays 7.

### Why the floor is 7 and not 6

A floor equal to the current epoch (6) would also be satisfied by every
pre-reader binary: it would fence nothing. The reader must carry a higher
epoch, so that "a binary that reads this" is exactly "epoch at least 7".
Consequence (stated, not hidden): once release 1 has opened a store, its
receipt reads 7, and the updater refuses `Apply` of an epoch-6 release to
that store ("refusing stale binary", `updater.go:527`). A rollback from the
reader release to the pre-reader release is therefore not available through
`Apply`; it remains available through restorePrior (the floor is unset until
release 2). This is the same trade the earlier epochs made (`schema.go`
history for 3, 4, 5, 6).

### What restore, automatic rollback and capsule copies do when the floor is above their target

- **Automatic rollback (restorePrior after a failed health probe).** The
  updater checks the floor before restoring (`updater.go:552-576`). If the
  prior release is epoch 6 and the floor is 7, the rollback is refused with
  `ErrStoreFloorRefused` and the computer stays on the failing release. That
  is the loud outcome the floor exists for. It means: after the first inline
  write, **the writer release cannot automatically roll back below the
  reader release**. Rollback from the writer lands on the reader release (the
  prior release, if release 1 was the previous deploy), which is allowed.
- **Product restore.** A restore restages the checkpoint's pinned release
  (`RestagePinnedRelease`). A checkpoint whose release is below epoch 7 is
  refused on a floored computer (`updater.go:552-576`; the baseline paths
  allow an undeclared epoch, a declared one below the floor is refused). So
  restore to a checkpoint older than release 1 is refused, loudly, as soon as
  the floor exists. A restore to a newer checkpoint works.
- **Capsule copies.** A capsule is a copy of the workspace opened by a binary
  of the capsule's own release. A capsule whose release epoch is below 7
  fails `CheckFloor` at `Open` (`open.go:223`) and the capsule fails to
  start with the floor message. No capsule can hold a binary that cannot read
  the tape it is a copy of. Capsule images must therefore carry release 1 or
  later before the writer release ships (section 5).

### The floor-after-restore hole (round 3) and how this design does not have it

The finding (`carry-by-reference-design-three-review-rounds-structural-assessment`,
round 3): `ensureProjectionBatchTrimFloor` writes the floor once per process
and then skips it (`internal/store/event_index_trim.go:85-98`, memo
`projectionBatchTrimFloorWritten`, a `sync.Map` keyed by path). A checkpoint
restore (`internal/agentcore/rematerialize.go:243-261`) renames a staged
workspace over the live one, which drops the live floor file, then
`rt.store.Reopen`. The memo still says "written", so the floor is never
re-written, and the store runs unfloored with the trimmed or, here, inline
data on the tape.

This design closes it with three rules, none of them a memo of "I wrote it":

1. **No path memo.** The writer's floor check is `ensureInlineFloor()`, run
   before every batch that holds an inline body. It does one `os.Stat` of
   `store-floor.json`; it skips the read only if the stat's (inode, size,
   mtime) equal the values recorded when this process last **read** the file
   at floor >= 7. Anything else (file replaced by a rename, file missing,
   file changed) falls through to `ReadFloor` and `WriteFloor`. A restore that
   swaps the workspace changes the inode, so the next write re-establishes the
   floor. Cost: one `stat` per tape batch (**estimate**: microseconds;
   batches are millisecond-scale writes). **(invented)**
2. **Applying a version 3 batch asserts the floor too, tape-derived.** Any
   binary that projects a version 3 batch (live, catch-up or replay) calls
   `ensureInlineFloor()` first. A restored older workspace has no floor, but
   the tape it will catch up from has version 3 batches beyond its head; the
   first one it applies re-asserts the floor, before applying. The trigger is
   the tape, which a workspace swap cannot remove. **(invented)**
3. **Reopen re-asserts when this process has written or applied a version 3
   batch**: `Store.Reopen` (the call at `rematerialize.go:259`) runs
   `ensureInlineFloor()` if a process flag `inlineSeen` is set, so the window
   between adoption and the next batch is closed for the common restore case.
   A fresh process that adopts a workspace and has seen nothing relies on rule
   2 and on F1.

Residual window, stated: between a swap that drops the floor and the first
version 3 batch applied or written, the floor file is absent. In that window
F1 still refuses an old binary at the first version 3 batch it meets, and the
updater (which reads only the floor file) could permit an `Apply` of an
epoch-6 release that then refuses at boot on the tape. That ends in a loud
failure on the older release, not a misread, and the next version 3 batch the
newer release applies fixes the floor. If the owner wants the window to be
zero, the conservative extension is to floor 7 unconditionally at every `Open`
of a writer-release binary; its cost is that automatic rollback is refused
from the first boot, not from the first inline write. (Open question O2.)

## 5. Two releases, order and rollback

### Release 1: the reader (lands first)

Contents: `Object.UnmarshalJSON` (both forms), `Validate` accepts (3,2),
`storeschema.Version = 7`, counters for what the writer will count (section
7, zero until release 2). No behavior change on tape. Class: orange (the
decode path of every object op and the epoch). Red surfaces touched: Texture
canonical writes only through the decode path, tape and evidence (the batch
validator), checkpoint projection (the version check). That is red; the two
seats apply (CLAUDE.md "Review").

Rollback from release 1 lands on the pre-reader release. Safe for tape (no
version 3 batch exists). `Apply` of that epoch-6 release is refused once the
receipt reads 7 (section 4); the revert path is restorePrior or a manual
re-stage. State this in the landing report.

Gate before release 2: **every computer, every capsule image and every
pinned checkpoint release that may restore is on release 1 or later.** The
platform side must also be: if the platform hosts a binary that decodes guest
batches (`ledgerwitness` reads batches with plain `json.Unmarshal` and no
`Validate`, so it is indifferent), it needs no change.

### Release 2: the writer

Contents: the three writer sites call `MarshalTapeObject`, the batch
constructor sets version 3, `ensureInlineFloor`, the counters, the self
check. Class: red (tape and evidence, checkpoint and route projection,
replay).

Rollback from release 2: the prior release is release 1 (it reads version 3),
so a rollback by `restorePrior` or `Apply` lands on a binary that replays every
batch. The floor stays 7 forever. Reverting the writer commit keeps the
reader, so tape already written stays readable.

### How the writer release is proved on the test computer before the largest takes it

I found no existing way, and I will not invent a switch.

- A normal deploy reaches every computer within minutes. `ringSoaks` are
  zero for every ring (`internal/vmctl/ring_walk.go:86-104`; exposure floors
  R0 1 member, R1 3, R2 1, R3 2, R4 1, R5 1), a ring closes when the required
  members were seen healthy, and an expedite opens every ring at once
  (`docs/design/ring-ordered-deploys-2026-11-03.md`, "Expedite"). Any
  landing touching `internal/agentcore/` is expedited by the deploy without a
  trailer (`.github/scripts/node-b-ci-deploy`). The wedge, boot and crash
  guards run inside a walk, but they test whether a computer boots and
  appends, not whether tape is equivalent on replay.
- Options, none of which is a switch:
  1. **Land release 2 without `Ring-Expedite` and with no change in the
     paths the deploy auto-expedites.** The walk then goes R0 (d@e), R1
     (test, three healthy members), and only then R2..R5. With zero soak
     this is minutes, but each ring closes only after members are healthy on
     the release. It proves boot and append, not replay.
  2. **Reader release 1 first, then a d@e-only drill** (CLAUDE.md allows
     adversarial drills on d@e): run the whole-tape replay equivalence
     (section 8) on d@e **before** landing release 2, using a repo test that
     writes version 3 tape into a test computer's *copy*. This proves the
     reader on real tape without exposing production to the writer.
  3. **The self-check at write time** (section 3). Every op's own decode is
     compared at write; a failure falls to base64 and is counted. This
     bounds the damage of a writer bug to a counter, not to a misread.
  4. **The owner restores a soak floor for this release** (an edit of the
     constants, a commit): rejected as a general rule by the owner's
     2026-11-10 instruction; named so it is not forgotten.
  5. **A per-computer canary by tape position or by kind** is an allowlist
     that gates a feature (CLAUDE.md "No switches"): not allowed.
- Conservative recommendation: 1, 2 and 3 together. The largest computer is
  then exposed to a writer that has been replayed on d@e's tape, the test
  computers' tape, and that checks itself on every op. If the owner wants
  more than that, it is an owner decision about the soak constants, not a
  mechanism this design supplies. (Open question O3.)

## 6. Everything else that reads the op length or the body

| Consumer | Anchor | What changes | Action |
| --- | --- | --- | --- |
| `tape_op_bytes_by_object_kind`, `object_rewrites` bytes | `tape_write_counters.go:151-154`, `228-236`; `object_rewrite_counters.go:128-148` | `len(op.Body)` drops by about a quarter for inline ops; the series breaks at the writer release. | Add `ops_inline`, `ops_base64` (by reason) and `body_bytes_inline` counts by kind; note the break in the scope text. |
| `run_write_counters.go` scope text | `run_write_counters.go:280, 318` ("four thirds", "the tape carries the body in base64") | Becomes false for eligible bodies. Counts are of `Object.Body`, which is unchanged. | Edit the text in release 2: "four thirds for the base64 form, one for inline". Green text. |
| `tape_group_commit.go opsBytes` | `tape_group_commit.go:140-146` | Sizing of a group uses `len(op.Body)`; inline ops are smaller, so groups hold more ops at the same cap. | Check the cap is a bound on strangers or latency, not a quota; I did not read the cap. |
| Receipts and digests over batches | `computerevent.DigestBytes(raw)` (e.g. `ledgerwitness/walk.go:139`), `payload_resolver.go`, `receipt*.go` | A digest names the exact plaintext bytes of a batch. Old batches keep their digest; new batches have new bytes and new digests. No digest is recomputed over an old batch. | None. Test: replay verifies each stored digest against its bytes (existing). |
| Canonical witness (commit 304a81277) | `304a81277` message; `CheckpointFromRequest` | The witness covers state rows, which are decoded bytes; identical in either form by section 3. It also covers the event index, whose rows are keyed by batch payload digest (new digests for new batches, as for any new batch). | Test: old-form and mixed-form tapes give equal witnesses (section 8). |
| Replay-completeness probe | `ReconstructInto`, `ReconstructThroughTarget` (`appender.go:1818-`) | Calls `DecodeProjectionBatch`; needs the reader. | Covered by F1 on old binaries. |
| Snapshot chunks | snapshots are state projections, not op copies; I did not read the chunk writer | Nothing to change if a chunk holds rows; if a chunk holds op bodies, they pass through the same decoder. | **Not verified.** Grep for any producer that copies `ProjectionOp.Body` into a chunk before release 1. |
| Residue import | `residue_import.go:126-130`, `588` | The writer uses `MarshalTapeObject`; `scanResidueObject` scans SQL, not a tape op. | Included in the three writer sites. |
| Compression survival | `privacy.go:75-84, 334-336` | `compressPrivatePayload` keeps the compressed form only if it saves 5% and the plaintext is over 256 bytes; inline bodies are smaller before compression, and compress better. | Count `compression` empty vs zstd per family. |
| Ledger witness walker | `ledgerwitness/walk.go:147-165` | Reads `object_kind` and `metadata` only; unaffected. | Test with a version 3 batch. |
| Platform decode | `ledgerwitness` uses `json.Unmarshal` with no `Validate`; platform HTTP decoders decode objects, not batches | Unaffected. If any platform path decodes batches with `Validate`, it needs release 1. | **Not verified**: grep found no other batch decoder outside `internal/computerevent` and `internal/store`. |

## 7. Expected saving, and what settles it

Inputs (all from the problem record, nothing re-measured):

- Bench on 20 run records, zstd -3: base64 form is **1.77 to 1.92 times** the
  inline form (195,463 vs 110,158 bytes alone; 138,111 vs 72,038 in batches of
  five). The sealed envelope's own base64 multiplies both sides equally.
- Object ops are about two thirds of tape bytes (120.3 of 182.7 MB,
  pre-compression).
- Metadata, envelope, run-memory entries and edges do not change.

Estimate: object-op bytes after compression fall to `1/1.77 to 1/1.92` of
today's, that is 0.52 to 0.56. If the object-op share of compressed tape is
also two thirds, the compressed tape is `1/3 + 2/3 × (0.52 to 0.56)`, that is
**0.68 to 0.71 of today's, a saving of 29% to 32% (estimate)**. It falls if:
(a) a share of bodies falls back to base64 (non-JSON, secrets, not
compact-exact; unknown share), (b) the share of object ops in the compressed
tape is below two thirds (the two-thirds figure is pre-compression), (c)
other kinds compress differently from run records (the bench was Texture-heavy
run records that still carried the source-entities list, which is gone since
8d81b7fe5). Plausible range **10% to 32%**, and I will not narrow it by
argument.

What will settle it: a compressed-bytes-per-sealed-batch counter. The
problem record says none exists (`PlaintextLength` is in the envelope
metadata, `privacy.go:28-40`, but nothing sums it or the stored size). The
smallest counter **(invented)**: in `Encrypt` (`privacy.go:300-340`), after
`compressPrivatePayload`, bump `sealed_plaintext_bytes` (`len(redacted)`) and
`sealed_stored_bytes` (`len(sealed)`) by the batch's first-op family (the same
family `tape_bytes_by_family` uses, `tape_write_counters.go:62-68`) and by
version (2 or 3). Land it in release 1, so release 1 gives the "before" and
release 2 the "after" on the same computer. The settling number is stored
bytes per object-op byte of plaintext, compared before and after, by family
and by version.

## 8. Failure list

1. A base64 `body` on an old binary decoded with `body_json` absent: an old
   binary misreads an inline op as a bodyless object. Fence: version 3
   (F1), floor (F2). Test: old-binary refusal.
2. A version 3 batch on an old binary refused by `Validate` during **catch-up
   after restore** wedges the computer rather than skipping the batch.
   Intended (loud), but it is a production outage on any computer that
   restores to an older release past the floor. Fence: the updater refuses
   that release first (F2).
3. Floor absent after a workspace swap (round 3). Rules 1 to 3 in section 4.
   Residual window named.
4. `os.Stat` identity (inode, size, mtime) reused by an adopted file that
   happens to match: a restore renames a staged workspace in; a staged file
   with identical inode/size/mtime to the old one is not plausible but not
   impossible on a copy-on-write filesystem. Fall back: also re-read the
   floor every N batches or on any `Reopen`. (Open question O4.)
5. `json.Marshal(RawMessage)` compacts and escapes: a body with raw `<`, `>`,
   `&`, U+2028, U+2029 or whitespace changes bytes if carried naively. Fence:
   eligibility test 2.
6. A body with invalid UTF-8: `json.Valid` accepts bytes inside strings that
   the encoder would replace; test 1 refuses via `utf8.Valid`.
7. A body that is `null`, an empty string, a bare scalar, or an empty
   `[]byte{}` vs `nil`: `omitempty` drops both empties today. `null` is
   refused (test 3). Round-trip test covers them.
8. Duplicate keys and key order in a JSON body: carried verbatim, not
   re-parsed; test with an object that repeats a key.
9. Huge numbers and exponents: carried as text; test a body with `1e400` and
   `12345678901234567890`.
10. Redaction rewrites an inline body (section 3.2), so replay differs from
    live. Fence: test 4. Test: a body with a bearer token, a PEM header, an
    `api_key=` and an `sk-` token each stays base64 and replays equal.
11. The redactor's matcher sees context outside the body (the field name
    before, the next field after). Test 4 runs over the **candidate op**
    bytes, not the body alone, but the batch-level context (`"body_json":`
    preceded by other ops) is not identical to the op alone. A regex that
    spans the op boundary is not possible (ops are separate JSON values in an
    array, comma separated), but I did not prove the unquoted `credential_assignment`
    pattern cannot bridge the comma. Test it.
12. A body that is JSON today but contains a secret only in its **escaped**
    form (`token`): the redactor works on raw bytes first, then decodes a
    string if it matched wholly inside (`redaction.go:214-250`); the
    eligibility test must use the same function, not a re-implementation.
13. Mixed-form batches: one batch with inline and base64 ops, version 3. The
    reader handles each op by field; test it.
14. A batch of version 3 holding **no** inline op (writer bug): harmless but
    raises the fence for nothing. Test: version 3 iff any inline.
15. `ProjectionOp` marshalled with `InlineBody` as `json:"-"`: lost on a
    round trip through a batch decode; only the writer uses it. Assert the
    constructor, not the decoder, sets the version.
16. `ProjectorVersion` pairing: `Validate` requires an exact pair
    (`projection_batch.go:366-367`); (3,2) must be added without loosening
    the others. Test every other pair stays refused.
17. A **second** writer of a batch not through `projection_tape.go:212`
    (a future one, or the unreadable stub at `appender.go:1801`) emits version
    2 with inline ops: an old binary then misreads. Fence: a guard test (as
    `event_table_guard_test.go` does for `Table`) that the only constructor
    of a batch that holds `body_json` is the version 3 path; and the reader
    refuses a **version 2 batch containing `body_json`** in release 2's
    `Validate` (loud). Invented.
18. The upcaster (`vocab_upcast_deposit.go:560-675`) re-encodes with
    `json.Marshal`; after decoding an inline op it re-emits base64. Fine in
    memory; but if the upcast ever feeds a re-written batch into a version 2
    path, the base64 re-encode is the safe direction. Test the upcaster over an
    inline op and over a mixed batch.
19. `HTTPStore.PutObject` (`http_store.go:45`) keeps `body`: an object copied
    from tape to the platform must be re-encoded base64. Because `Object`'s
    default marshal is unchanged, this is automatic; a test pins it (a
    marshal of a decoded inline object yields `body`, not `body_json`).
20. `Object.UnmarshalJSON` slows every object decode: alias struct plus
    `RawMessage` copy. Estimate a few percent of decode cost; measure with
    the existing bench. Not a correctness risk.
21. `Object.UnmarshalJSON` and `DisallowUnknownFields` anywhere: a decoder that
    disallows unknown fields would refuse `body_json` on an old type. Grep for
    `DisallowUnknownFields` on object decodes before release 1 (`floor.go:54`
    uses it for the floor file only).
22. Snapshot chunks or any other consumer that copies `ProjectionOp.Body` into
    a place an older decoder reads (section 6, not verified).
23. Telemetry sealer's `recheckTelemetryOp` (`telemetry_sealer.go:540-560`)
    runs `json.Unmarshal(obj.Body, &ev)`: with the reader, `obj.Body` is the
    raw bytes; unchanged. Test the sealer over inline.
24. The unsealed path (`AppendNewPayload`, no compression found, `privacy.go`
    only) writes plaintext batches: inline bodies there are larger relative
    to base64 only if there is no compression (they are smaller than base64
    either way) and the redactor does not run, so test 4 is stricter than
    needed. Harmless.
25. Group commit sizing (`tape_group_commit.go:140-146`) shrinks: more ops per
    group; check the cap's meaning (section 6).
26. Counter series break (section 6) misread as a regression by a later
    investigator. Note it in the scope text and in `ACTIVE`.
27. Automatic rollback refused after the first inline write (section 4): a
    failing writer release stays running. Mitigation: the ring guards run
    before the floor exists only if the first inline write is later than the
    guard window; I did not check the timing. The self-check (section 3) and
    option 2 of section 5 are the real mitigation.
28. Product restore to a checkpoint pinned to a release below epoch 7 is
    refused after the floor exists (section 4). Verify what the restore path
    shows the owner: a named refusal, not an opaque failure.
29. Capsule images pinned to older releases fail `CheckFloor` at open
    (section 4). Gate release 2 on every capsule image being release 1 or
    later.
30. Fork semantics: a restore to a checkpoint ahead of an abandoned tape
    branch; I did not read whether the tape past the restore target is kept
    or abandoned (`ReconstructThroughTarget` halts at the target). If it is
    kept and an old release continues from the target, it meets a version 3
    batch past the target and refuses. Loud, but verify. (O5.)
31. The floor file itself written by a release that is not release 2: any
    binary that applies a version 3 batch writes it. Release 1 does too
    (rule 2), but release 1 should never meet one before release 2 exists; if
    it does (a manual tape import), the floor is raised, which is correct.
32. Reader and writer both in one release by mistake (collapsing the two
    steps): then a rollback lands on a binary that cannot read version 3.
    Fence: a landing check that the reader's epoch (7) is already the
    deployed floor candidate in `origin/main`'s previous deployed release
    receipt. Invented, and best done by the owner's landing procedure rather
    than code.
33. Platform-side binary skew: a platform that decodes a guest object from
    the guest HTTP path keeps getting `body` (item 19), so it is unaffected.
    A **platform** path that reads the guest tape (a purge/archive tool,
    `tape_purge.go:367` writes `rows/*.jsonl`) and parses object ops for
    their body would see `body_json`. I did not read it. Grep before release 2.
34. A document of the wire form that says "exactly one of body, body_json":
    a decoder that tolerates both silently (preferring one) hides a writer
    bug. The reader refuses both being non-empty.
35. Old tape stays readable forever: the reader is kept by test (a fixture
    batch from each historical version replays equal). The reader must never
    be removed; a test that fails if `UnmarshalJSON` drops the `body` branch.

## 9. Proof

Repo tests first (yellow, written before the code, per CLAUDE.md):

- **Round-trip byte equality**, for every registered object kind
  (`ogKind*` and the `ObjectKind` constants listed by `grep`), using a real
  fixture body per kind taken from existing test fixtures, plus the property
  test of section 3: `decode(encode(B)).Body == B`, and
  `ContentHash` equal across forms.
- **Mixed-form batches**: one batch with inline, base64-forced (non-JSON,
  secret, escape-needing) and edge ops; replay equal to the live store.
- **Old-form tape replay equality by canonical witness**: a fixture tape of
  version 2 batches written by the pre-change code replays under the new
  reader and produces the same version 4 canonical witness
  (`304a81277`) as today; the same tape re-written inline produces the same
  witness.
- **Old-binary refusal**: a test, using the `Validate` of the pre-change code
  vendored as a fixture, or the existing pattern in
  `event_table_old_binary_test.go:15-50`, that a version 3 batch is refused
  with `ErrProjectionBatchInvalid`, not applied as a bodyless object, and that
  a version 2 batch containing `body_json` is refused by release 2.
- **Floor**: write the floor, replace the workspace (as `rematerialize` does),
  write an inline op, observe the floor reappear; apply a version 3 batch to
  a workspace with no floor, observe the floor; the stat memo test with a
  replaced file of equal size.
- **Redaction fence** (item 10).
- **Guard test** (item 17): the version 3 constructor is the only producer of
  `body_json`.

Deployed:

- **Whole-tape replay equivalence** on the test computer: replay the tape
  (the existing `ReconstructInto` dry run) and compare the witness to the
  live store, before and after the first version 3 batches.
- **Restore drill on d@e only**: restore to a checkpoint **after** release 2,
  observe the floor file re-established; try a restore to a pre-release-1
  checkpoint, observe the named refusal, not a misread.
- **Counter reading**: the sealed bytes counters of section 7, version 2
  against version 3, plus `ops_inline`, `ops_base64` by reason, and
  `inline_self_check_failed` at zero.
- A clean `git status`, the pushed SHA and CI/deploy status as CLAUDE.md
  "Final report" requires, per release.

## 10. Rollback

- Release 2 (writer): revert the commit. The reader (release 1) stays, so
  version 3 tape written meanwhile stays readable. The floor stays at 7; it is
  never lowered. The previous release (release 1) is a legal target.
- Release 1 (reader): revert is safe while no version 3 batch exists. After
  release 2 has written one it is not: the revert must not be taken, and the
  floor refuses it (the epoch).
- Nothing is rewritten. Old tape is never touched.

## 11. Open questions, with conservative defaults

- **O1. Version pair (3,2) vs (3,3).** Default (3,2): the projector did not
  change.
- **O2. Floor at first inline write vs at every `Open` of a writer binary.**
  Default: at the first inline write plus rules 2 and 3 of section 4 (keeps
  automatic rollback available during the first health window). The stricter
  alternative closes the residual window and costs that rollback.
- **O3. Proof on the test computer before the largest.** Default: section 5
  options 1, 2 and 3 together, no new mechanism. Owner decides whether to
  restore a soak floor for this release.
- **O4. Is the stat identity enough**, or add a periodic re-read?
  Default: stat identity plus a re-read on `Reopen` and every 1,024th batch.
- **O5. Restore past an abandoned branch.** Default: treat as loud-refusal
  and verify in the restore drill.
- **O6. Should `body_json` also apply to `metadata`'s already raw form?**
  Not applicable: metadata is already inline.
- **O7. Redaction gap on base64 bodies** (a secret in a body is not redacted
  on the tape today). Default: record as its own problem, do not fix here;
  this design keeps the base64 form for any body the redactor would touch so
  it does not change that behavior.
- **O8. Epoch bump in the reader release makes rollback of release 1 via
  `Apply` refused.** Default: accept, as the earlier epochs did.

## 12. Other envelope costs, listed only (no design here)

From the problem record; each is a candidate later slice with a one-line size
estimate (all **estimates**, taken from the record's reconstruction, not
measured):

- **Metadata block copies ten fields of the event record** (`graph_store.go:1258-1269`):
  about 380 bytes of the mean 3,198 byte `choir.event` op (about 12%
  before compression; likely less after, as the keys repeat).
- **The record wrapper around the payload** (event id, seq, ts, ids, keys of
  the event record): about 400 bytes per event (about 12% before compression).
- **Tool results written twice** (the `tool.result` event's `output`, and the
  run-memory entry's message, plus `full_output` when a projection exists;
  `batch_executor.go:103-148`, `toolloop.go:2328-2345`): `tool.result` is 9.44
  MB of the 13.65 MB event payload; the second copy is between zero and that
  size, the share not measured.
- **Run-memory messages escape their JSON twice** (`MessageJSON` is a string
  field, `projection_batch.go:49`): the cost is the backslashes, not
  estimated.
- **Object envelope and op wrapper** (canonical id twice, hash, two
  timestamps): about 330 bytes per op (about 10% before compression).
