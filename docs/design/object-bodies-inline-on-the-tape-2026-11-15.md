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
bytes the encoder would emit). A batch that holds at least one such op is
written as **batch version 3**, which every binary that predates the reader
refuses at `Validate`. If the assembled batch bytes hold a secret-shaped match
or do not decode, the whole batch is written as version 2 with base64 bodies,
as today. The reader is `objectgraph.Object.UnmarshalJSON`, which accepts
either field forever. The reader ships first with store epoch 7; the writer
ships second and asserts floor 7 at every `Open` and `Reopen`. Old tape is
never touched.

## 1. What the code does today (anchors)

- The type. `objectgraph.Object` has `Body []byte` tagged `json:"body,omitempty"`
  and `Metadata json.RawMessage` (`internal/objectgraph/object.go:23-36`).
  `encoding/json` writes `[]byte` as base64. `Edge` has no `[]byte`
  (`object.go:38-46`). `HeaderRow` embeds `Object` (`internal/objectgraph/sql_store.go:1526`)
  and would inherit the promoted `UnmarshalJSON`; it is never JSON-decoded
  today, and a guard test says so.
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
  (`tape_group_commit.go:150-155`). None reads `body`.
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
  `Apply` and `restorePrior` to a release below the floor or declaring no epoch
  (`internal/updater/updater.go:552-560`, `579-600`, `restorePrior` at `1233`).
  `RestagePinnedRelease` and the baseline stage and import pass
  `allowUndeclared=true` (`updater.go:689`, `908`, `1010`): they refuse a
  release declaring an epoch below the floor but let one that declares **no**
  epoch (`StoreSchemaVersion == 0`) through (`updater.go:590-593`). The current
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

At most one of `body`, `body_json` may be present and non-empty (`body` has
`omitempty`, so an empty body is absent). Both non-empty is a decode error (the
reader refuses; it never prefers one). `"body_json":null` decodes into a
`RawMessage` as the literal `null`; the reader treats it as an error, never as
a body (the writer never emits it, eligibility test 3).

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
op has it and the batch passes the two batch-level checks of section 3. `Validate` accepts (3,2) in the reader release. (The projector did
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
the encoder guarantees. The same pass applied twice is the identity, so the
outer batch marshal does not change bytes the op marshal already produced (a
reviewer measured the two-level round trip on go1.26.10; test 2 below makes it
a repo test).

### The per-body eligibility test (invented)

A body `B` (non-empty) is written as `body_json` iff all of:

1. `utf8.Valid(B)` and `json.Valid(B)`;
2. `bytes.Equal(json.Marshal(json.RawMessage(B)), B)`: the encoder would emit
   exactly `B`, so nothing is changed by compaction or escaping;
3. `B` is not the literal `null` (a `null` value is ambiguous with an absent
   field to a reader).

Otherwise the op keeps `body` (base64) exactly as today, in the same batch.
The decision is a pure function of `B`, so two writers given the same object
choose the same form. Test 1 keeps `utf8.Valid` for two real reasons: an
unsealed payload is a raw JSON file on the tape that jq, Python and archive
tooling read, and the SQL text columns want valid UTF-8. It is **not** because
the encoder rewrites invalid UTF-8: a reviewer measured that
`json.Marshal(RawMessage)` leaves it unchanged.

### The two batch-level checks (replace the per-op secret test and the per-op self-check)

After the writer has marshalled the batch as version 3 (any op inline), it
checks the exact bytes it is about to hand to the appender:

- **A. Secret rule.** If `findSecretMatches(payload)`
  (`internal/computerevent/redaction.go:328`) returns any match on those
  bytes, the whole batch is re-marshalled as version 2 with every body
  base64, which is today's behaviour exactly.
- **B. Decode rule.** `DecodeProjectionBatch(payload)`
  (`projection_batch.go:414-423`) must succeed, and for each inline op the
  decoded `Object.Body` must equal the original body bytes. If either fails,
  the whole batch is re-marshalled as version 2 with base64 bodies. The
  sealed path already decodes the same `payload` once per batch
  (`projection_tape.go:240`, a plain `json.Unmarshal`, whose failure today
  fails the write); rule B is that decode plus `Validate` and the body
  comparison, moved before it so a failure is a fallback, not a failed write.

Both fall back to the same thing, so there is one fallback, not two. Each is a
correctness refusal of the new form for that batch, not a switch.

Fallback mechanics (round 2):

- **The checks run once.** Rules A and B run on the version 3 bytes only. The
  version 2 fallback is written **unchecked**: it is today's encoding, and
  re-running rule A on it would not terminate in the `AIza` case below (a
  base64 body token can itself match `google_api_key`, so "fall back again"
  has nowhere to go). "One fallback, not two" means one-shot.
- **Rule B compares against a carrier of the pre-encode bytes, never against
  `body_json`.** The writer's per-op value carries the original body bytes in a
  Go-only field (`json:"-"`, **invented**, replacing the `InlineBody bool` of
  section 2). Comparing the decoded `Object.Body` with the `body_json` it was
  decoded from would be a tautology that never exercises the outer batch
  marshal. Test 2 makes a tautological check harmless for eligible bodies, but
  the design does not rely on that.
- **The version is derived from the final bytes.** The constructor sets
  version 3 iff some marshalled object op carries `body_json`, and rule B
  asserts "version 3 if and only if some op is inline" on the same decode, so
  a copy that drops the Go-only field cannot produce a version 2 batch holding
  `body_json`.
- **How the fallback rebuilds base64 ops.** The ops hold already-marshalled
  bytes. The fallback decodes each object op through the reader and
  re-marshals it with `Object`'s default encoder, and a test asserts the
  version 2 output contains no `body_json`.

**Which bytes `Encrypt` receives.** The payload that `projection_tape.go:212`
marshals is passed unchanged to `AppendNewPrivatePayload`
(`projection_tape.go:256`), which hands it as `plaintext`
(`appender.go:719`) to `PreparePrivatePayload` (`appender.go:732`), which calls
`cipher.Encrypt(..., plaintext)` (`http_client.go:170`), whose first act is
`redactPrivatePayload(material.key[:], plaintext)` (`privacy.go:322`). No byte
is added, removed or re-marshalled between the writer's marshal and the
redactor, so rule A runs on the redactor's own input with the redactor's own
matcher.

### 3.1 Bodies that are not JSON or not UTF-8

Plain text and any binary (the three platform kinds above, any media body,
anything `[]byte(string)`) fail test 1 and keep base64. Whether the guest
holds a kind whose body is not JSON, I did not establish; the design does not
depend on the answer. A per-kind count of `inline` against `base64` (section 7)
says which kinds fall back; it does not break the fallback down by reason (the
reason `secret` would publish to the host that a body held a secret-shaped
string).

### 3.2 Why the secret rule is batch-level

The sealed path redacts the whole batch plaintext before compressing
(`privacy.go:310-350`). `redactJSONPayload` (`redaction.go:183-280`) runs
`findSecretMatches` over the entire batch and replaces whole string tokens
that any match overlaps. One pattern, `private_key`, is a `(?s)` PEM form
(`redaction.go:27`; `private_key_header` at `:28` is not `(?s)` and matches only
the header line) whose `.` matches `"` and `,`, so one match **can
cross ops**: a `-----BEGIN ... PRIVATE KEY-----` in a run-memory text (shell
output, no END) and the matching END in an inline body in the same batch. A
per-op test passes the body; the batch-level match then covers the body's
string token and the redactor replaces it by a handle. The result is silent:
the live finalize and the replay both resolve through the artifact reader
(`appender.go:1755`; `livePayloads` is populated only on the unsealed path,
`appender.go:689`), so they agree, the replay-completeness probe passes, and
the row keeps the pre-redaction `content_hash` against a redacted `body`
(`project.go:243-279` writes `obj.Body` and the op's `content_hash` verbatim).
Deciding from the assembled batch bytes needs no boundary argument: the
redactor returns the payload untouched when `findSecretMatches` finds nothing
(the decoded-string pass runs only after a raw match), so "no match on the
bytes `Encrypt` receives" is sufficient for the redactor to leave every body
alone. It leaks nothing: the only difference observable is version 2 against 3
inside sealed plaintext. Cost: one secret-shaped string anywhere in a batch
sends the whole batch to version 2, which only shrinks the saving.

The base64 form is not immune either: a Std-encoded body token can begin with
`AIza` and satisfy `google_api_key` (`\bAIza[0-9A-Za-z_-]{20,}\b`, `\b`
satisfied by the opening quote), so the same corruption shape is reachable
today. That is the pre-existing gap of O7; this design neither fixes nor
worsens it. The unsealed path (`AppendNewPayload`) never redacts; rule A there
is stricter than needed and harmless. Production always seals
(`SetProjectionSealer` refuses a nil cipher).

### Proof (what the tests must show; section 9)

For every fixture body `B` and both forms: `decode(encode_inline(B)).Body == B`
bytewise, `decode(encode_base64(B)).Body == B`, hence
`ContentHash(kind, decode(...).Body, meta)` is equal in the two forms. A
property test over generated bodies (whitespace, `<>&`, U+2028, lone
surrogate escapes, deep nesting, big numbers, duplicate keys, BOM, invalid
UTF-8, empty, `null`, bare scalars) must show that every ineligible body falls
to base64 and every eligible one round-trips.

Depth: `json.Valid` and `compact` allow nesting up to 10,000 but the batch
adds about four levels, so a body nested near 9,997 passes the per-body test
and then fails the batch `Unmarshal` with "exceeded max depth", a
`SyntaxError`, which `appender.go:1805` turns into "applied as no change" on a
sealed replay. Rule B is the fence; it is why the check is on the final batch
bytes.

## 4. Old binary on new tape: refuse loudly, never misread

Two fences, independent of each other:

- **F1, in the tape (survives every workspace swap).** A version 3 batch fails
  `Validate` on every binary without the reader (`projection_batch.go:366-370`),
  on replay, catch-up, receipts-driven reads and the dry-run probes alike. The
  error is returned, not downgraded (`appender.go:1805-1807`). A binary that
  never learned `body_json` cannot apply a batch that has one without first
  failing this check. This is the fence that does not depend on a file, and
  the only fence against a silent misread: a binary with no reader sees an
  empty body, so the version is not optional.
- **F2, the store floor (pre-mutation, updater-visible).** Floor 7 refuses
  `Open` of the store by a build whose epoch is below 7 (`CheckFloor`) and
  refuses the updater's `Apply`/`restorePrior`/restage to such a release.

### Epochs (the two releases)

- Release 1 (reader): `storeschema.Version = 7`, `min_store_schema_version`
  6 (the field is set only by the platform update request,
  `internal/platform/platform_update.go:48,236`; nothing else in the repo
  produces it, so confirm the release pipeline sets it before treating it as a
  fence), `Validate` accepts (3,2), `Object.UnmarshalJSON` accepts both forms.
  **It writes no floor and no version 3 batch.**
- Release 2 (writer): `MarshalTapeObject` on the three writers; version 3
  constructor; floor 7 asserted at `Open` and `Reopen`. Epoch stays 7.

### Why the floor is 7 and not 6

A floor equal to the current epoch (6) would also be satisfied by every
pre-reader binary: it would fence nothing. The reader must carry a higher
epoch, so that "a binary that reads this" is exactly "epoch at least 7".

### The floor is asserted unconditionally, from the hook that already exists

`Store.Reopen` already re-asserts the event-table floor on every reopen
(`open.go:391-415`, via `writeEventTableFloorAt`, `event_table.go:550-555`),
and `WriteFloor` never lowers (`floor.go:70-107`), so asserting it again at or
above the target is a no-op. The writer release writes floor 7 from the same
place on every `Open` and every `Reopen`, with no condition, no memo and no
"first inline write" test. That closes the round-3 hole (a restore that
renames a staged workspace over the live one and drops the floor file; the
`projectionBatchTrimFloorWritten` memo at `event_index_trim.go:85-98` never
re-writes it) for this floor without a new mechanism: there is nothing to
remember, only an idempotent write on the path every adoption passes through.
The existing hook is conditional on the event-table writer switch
(`open.go:409`); the inline floor line is not.

**A floor-write failure fails `Open` and `Reopen`** in the writer release. The
existing hook logs and turns the event-table writer off when its floor write
fails (`open.go:409-414`); the inline floor line must not be treated that way,
because a floor that is absent while version 3 writes proceed removes F2's
pre-mutation fence (F1 would still refuse old binaries, loudly). So the
failure is returned, the store does not open, and the computer never writes
version 3 without floor 7. Test: a floor directory that refuses the write makes
`Open` and `Reopen` return the error.

What this costs, stated: automatic rollback below the reader release is refused
from the writer release's first boot, not from its first inline write. The
rollback target of the writer release is the reader release (epoch 7), which
stays legal (`updater.go:552-612`). The only thing refused earlier is rollback
below the reader, which the first version 3 batch would refuse anyway (F1).

### What restore, automatic rollback and capsule copies do when the floor is above their target

- **Automatic rollback (restorePrior after a failed health probe).** The
  updater checks the floor before restoring (`updater.go:552-576`). If the
  prior release is epoch 6 and the floor is 7, the rollback is refused with
  `ErrStoreFloorRefused` and the computer stays on the failing release. That
  is the loud outcome the floor exists for. Rollback from the writer lands on
  the reader release, which is allowed.
- **Product restore.** A restore restages the checkpoint's pinned release
  (`RestagePinnedRelease`). A checkpoint whose release **declares** an epoch below 7 is
  refused on a floored computer (`checkManifestAgainstFloor`,
  `updater.go:595-597`). A pinned release that declares **no** epoch is not:
  `RestagePinnedRelease` passes `allowUndeclared=true` (`updater.go:1010`) and
  `StoreSchemaVersion == 0` returns nil (`updater.go:590-593`). The outcome is
  safe: a pre-release-1 tape holds no version 3 batch, and a tape that does hits
  F1 loudly on the restored binary. But the stated refusal is not universal, and
  F1 is its backstop. Either keep the qualification or add a restage check that
  refuses an undeclared epoch on a floored computer (a choice for release 2's
  author; the reader release does not touch the updater). So restore to a
  checkpoint older than release 1 that declares an epoch is refused, loudly. A restore to a newer
  checkpoint works. The refusal must reach the owner as a named refusal
  (failure 20).
- **Capsule copies.** A capsule whose release epoch is below 7 fails
  `CheckFloor` at `Open` (`open.go:223`) and fails to start with the floor
  message. No capsule can hold a binary that cannot read the tape it is a copy
  of.

## 5. Two releases, order and rollback

### Release 1: the reader (lands first)

Contents: `Object.UnmarshalJSON` (both forms), `Validate` accepts (3,2),
`storeschema.Version = 7`, the telemetry of section 7 (zero until release 2).
No behavior change on tape. Class: orange (the decode path of every object op
and the epoch). Red surfaces touched: Texture canonical writes only through
the decode path, tape and evidence (the batch validator), checkpoint
projection (the version check). That is red; the two seats apply (CLAUDE.md
"Review").

**Landing gate for both releases: the builder that builds release 1 and
release 2 is itself at epoch 7.** `builder/request.go:275` stamps
`storeschema.Version` of the builder **process**, and `builder/builder.go:56-62`
says that is exact only on the same-commit path. A release built by an epoch-6
builder declares `store_schema_version` 6 in its manifest even when its source
has the reader. Then release 2 is refused on a store whose receipt reads 7
(`checkStateCompatibility`, `updater.go:524-527`), and after release 2 writes
floor 7 a failed probe cannot `restorePrior` to release 1 (`updater.go:596-597`,
call at `1248`). The failure is loud, not silent, but the landing report must
show that the release manifests of both releases declare
`store_schema_version` 7. This is not only a statement about self-development
layers.

Rollback from release 1 lands on the pre-reader release. Safe for tape (no
version 3 batch exists). `Apply` of that epoch-6 release is refused once the
receipt reads 7 (below); the revert path is restorePrior or a manual re-stage.
`scripts/check-store-schema-floor.sh` also blocks a release-1 revert by
construction (it refuses a lowered `Version`). State this in the landing
report.

### What the epoch 7 receipt refuses (the cost of every epoch bump)

`storeschema.Write` never lowers (`schema.go:104-121`), so once release 1 has
opened a store its receipt reads 7. From then on
`checkStateCompatibility` refuses any release whose `StoreSchemaVersion` is
below 7 (`updater.go:524-527`, "refusing stale binary"). Concretely:

- **Self-development layers built from a commit below epoch 7.** A host build
  takes its epoch from the build result, not from the computer:
  `layeredSelfDevelopmentManifest` sets `base.StoreSchemaVersion =
  r.StoreSchemaVersion` (`internal/agentcore/self_development_host_build.go:361`),
  and the builder stamps `storeschema.Version` of the **builder process**
  (`internal/builder/request.go:275`), not of the candidate source. Any apply
  or revert whose bundle was built at epoch 6 declares 6 and is refused.
- **In-flight frozen layers.** A layer frozen at epoch 6 and not yet applied
  becomes unappliable once the receipt reads 7.
- **Checkpoint restore to a pre-release-1 release** (section 4).

This is the same trade the earlier epochs made (3, 4, 5, 6), not a new cost of
this design. The operator action: before release 1 reaches a computer, list
the computers that carry a layered release or an in-flight frozen layer, and
rebuild and re-apply each from a commit at or after release 1 with a builder
at epoch 7 or later. For d@e's layered release (`code_commit` `4e816d720` on a
base equal to the deployed commit): its manifest epoch was stamped by
whichever builder built it, which I did not read. If that builder predates
release 1, the manifest says 6 and the layer is refused once the receipt reads
7, so it **needs a rebuild after release 1 is on d@e**; if the manifest
already says 7 it does not. Read the manifest before deciding. Do not "fix"
this by leaving release 1 at epoch 6: floor 7 would then refuse release 1
itself through `checkManifestAgainstFloor` (`updater.go:580-600`), and the
reader would not be a legal rollback target at all.

### Gate before release 2

**Every computer, every capsule image, every pinned checkpoint release that
may restore, and every layered or frozen self-development release is on a
binary that has the reader.** "Has the reader" is checked from the running
binary, not from a declared epoch: a builder at epoch 7 building a pre-reader
candidate source stamps 7 (`builder/request.go:275`) and passes the floor, yet
carries no reader. The check is that the release's code commit contains the
release-1 commit, or that the running binary decodes a probe batch with
`body_json`; a manifest field alone does not prove it. The capsule `Freeze`
path (`capsule/release_binary.go:70`) reuses only equal binary inputs and is
safer, but is held to the same check. A binary without the reader meets the
first version 3 batch as a loud wedge (F1), not a misread.

`ledgerwitness` reads batches with plain `json.Unmarshal` and no `Validate`
and reads only kind and metadata, so it is indifferent. The platform side is
**not** claimed to need no change: it is asserted unaffected only after the
two greps of failures 21 and 22, to be run before release 2.

### Release 2: the writer

Contents: the three writer sites call `MarshalTapeObject`, the batch
constructor sets version 3, the two batch-level checks of section 3, the
floor assertion at `Open` and `Reopen`, the telemetry. Class: red (tape and
evidence, checkpoint and route projection, replay).

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
  1. **Not available.** The operating contract (CLAUDE.md, "Delivery")
     requires `Ring-Expedite: <the defect it unblocks>` on every landing that is
     not docs-only, so there is no untrailered option for release 2. (The
     deploy also auto-expedites the paths in `.github/scripts/node-b-ci-deploy`;
     `internal/store`, `internal/computerevent` and `internal/objectgraph` are
     not among them, but that does not make the trailer optional.) A ring walk
     without an expedite would need an owner waiver of that rule. Release 1 and
     release 2 are both expedited and both name it in the report.
  2. **Reader release 1 first, then a d@e-only drill** (CLAUDE.md allows
     adversarial drills on d@e): run the whole-tape replay equivalence
     (section 9) on d@e **before** landing release 2, using a repo test that
     writes version 3 tape into a test computer's *copy*.
  3. **The batch checks at write time** (section 3). A failure falls to
     version 2 for that batch. This bounds the damage of a writer bug to a
     fallback, not to a misread.
  4. **The owner restores a soak floor for this release**: rejected as a
     general rule by the owner's 2026-11-10 instruction; named so it is not
     forgotten.
  5. **A per-computer canary by tape position or by kind** is an allowlist
     that gates a feature (CLAUDE.md "No switches"): not allowed.
- Conservative recommendation: 2 and 3 together (option 1 needs an owner waiver). If the owner wants more,
  it is an owner decision about the soak constants. (Open question O3.)

## 6. Everything else that reads the op length or the body

`len(op.Body)` has more consumers than the two the first draft named. Each cap
below bounds one append's size or latency; it is not a quota on model calls.
Inline ops are smaller, so more ops fit under the same byte cap, and no cap
needs to change.

| Consumer | Anchor | What changes | Action |
| --- | --- | --- | --- |
| `tape_op_bytes_by_object_kind`, `object_rewrites` bytes | `tape_write_counters.go:151-157`, `228-236`; `object_rewrite_counters.go:128-148` | `len(op.Body)` drops by about 19% for inline ops (base64 is a quarter of the body, the body about three quarters of the op); the series breaks at the writer release. | Note the break in the scope text and in `ACTIVE`. |
| `run_write_counters.go` scope text | `run_write_counters.go:280, 318` ("four thirds", "the tape carries the body in base64") | Becomes false for eligible bodies. Counts are of `Object.Body`, which is unchanged. | Edit the text in release 2: "four thirds for the base64 form, one for inline". Green text. |
| Group commit `opsBytes`, cap `groupMaxBytes` 1 MiB | `tape_group_commit.go:150-155`, `47-50`, `288` | Cap on the members' bytes in one group (one append). Inline members are smaller, so a group holds more ops. | None: it bounds one append's size. |
| Telemetry sealer `size`, cap `telemetryMaxBatchBytes` 256 KiB | `telemetry_sealer.go:138`, `42`, `509`; `tape_group_commit.go:461` | Cap on one sealed telemetry batch. Inline bodies fit more events per batch at the same cap. | None: it bounds one batch's size and latency. |
| Media chunking budget | `media_projection.go:168, 242` | `chunkMediaStateOps` and `batchBodyBytes` size by `len(op.Body)`; media state ops that stay base64 are unchanged. | None. |
| Receipts and digests over batches | `computerevent.DigestBytes(raw)` (e.g. `ledgerwitness/walk.go:139`), `payload_resolver.go`, `receipt*.go` | A digest names the exact plaintext bytes of a batch. Old batches keep their digest; new batches have new digests. No digest is recomputed over an old batch. | None. |
| Canonical witness (commit 304a81277) | `304a81277` message; `CheckpointFromRequest` | Covers state rows (decoded bytes, identical in either form by section 3) and the event index (new digests for new batches). Nothing hashes op bytes. | Test: old-form and mixed-form tapes give equal witnesses. |
| Replay-completeness probe | `ReconstructInto`, `ReconstructThroughTarget` (`appender.go:1818-`), reached via `replay_completeness.go:393` | Calls `DecodeProjectionBatch`; needs the reader. | Covered by F1 on old binaries. The deployed proof names the trigger (section 9). |
| Snapshot chunks | snapshots are state projections, not op copies; I did not read the chunk writer | Nothing to change if a chunk holds rows. | **Not verified.** Grep for any producer that copies `ProjectionOp.Body` into a chunk before release 1. |
| Residue import | `residue_import.go:126-130`, `588` | The writer uses `MarshalTapeObject`; `scanResidueObject` scans SQL, not a tape op. | Included in the three writer sites. |
| Compression survival | `privacy.go:75-84, 334-336` | `compressPrivatePayload` keeps the compressed form only if it saves 5% and the plaintext is over 256 bytes. | Read from `PlaintextLength` (section 7). |
| Ledger witness walker | `ledgerwitness/walk.go:147-165` | Reads `object_kind` and `metadata` only. | Test with a version 3 batch. |
| Platform decode | `ledgerwitness` uses `json.Unmarshal` with no `Validate`; platform HTTP decoders decode objects, not batches | Unaffected. | **Not verified**: grep for any other batch decoder outside `internal/computerevent` and `internal/store`. |

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
also two thirds, the compressed tape is `1/3 + 2/3 x (0.52 to 0.56)`, that is
**0.68 to 0.71 of today's, a saving of 29% to 32% (estimate)**, which is an
upper bound. It falls if: (a) a share of bodies or batches falls back to
base64 (non-JSON, a secret anywhere in the batch, not compact-exact; unknown
share, and tool-result text often says "token" or "password"), (b) the share of
object ops in the compressed tape is below two thirds (the two-thirds figure
is pre-compression), (c) other kinds compress differently from run records
(the bench was Texture-heavy run records that still carried the
source-entities list, gone since 8d81b7fe5). Plausible range **10% to 32%**,
and I will not narrow it by argument. A before/after counter would compare
different workloads, so it could not settle the number alone.

What settles it, with no new code: **the envelope already records it.**
`PlaintextLength` is in every private envelope's metadata (`privacy.go:34`);
it is the length **after redaction** (`len(redacted)`, `privacy.go:341`), not
the raw batch length, which differs for a batch the redactor touched.
`len(envelope)` is the stored size, so stored bytes per plaintext byte, by
batch version, is readable from existing tape with no counter and without
shipping release-1 code. Two readings:

1. On d@e's tape copy (the same drill as section 5 option 2): re-encode a
   sample of existing version 2 batches both ways (base64 and inline), seal
   each through the real `Encrypt`, and compare stored sizes. Exact, same data.
2. After release 2: read `PlaintextLength` and envelope length of version 2 and
   version 3 batches from the tape and compare by family.

The only host-visible telemetry added is `ops_inline` and `ops_base64` by
object kind (no reason breakdown), so the host learns which kinds fall back and
nothing about why.

## 8. Failure list

Recounted after the round-1 deletions: 28 items (the first draft had 35).

1. A base64 `body` on an old binary decoded with `body_json` absent: an old
   binary misreads an inline op as a bodyless object. Fence: version 3
   (F1), floor (F2). Test: old-binary refusal.
2. A version 3 batch on an old binary refused by `Validate` during **catch-up
   after restore** wedges the computer rather than skipping the batch.
   Intended (loud), but a production outage on any computer that restores to
   an older release past the floor. Fence: the updater refuses that release
   first (F2).
3. Floor absent after a workspace swap (round 3). Closed by the unconditional
   write at `Open` and `Reopen` (section 4). Test: write the floor, replace
   the workspace as `rematerialize` does, `Reopen`, observe the floor.
4. `json.Marshal(RawMessage)` compacts and escapes: a body with raw `<`, `>`,
   `&`, U+2028, U+2029 or whitespace changes bytes if carried naively. Fence:
   eligibility test 2.
5. A body with invalid UTF-8: kept as base64 by test 1 (reasons in section 3,
   not an encoder rewrite).
6. A body that is `null`, an empty string, a bare scalar, or an empty
   `[]byte{}` vs `nil`: `omitempty` drops both empties today. `null` is
   refused (test 3). The reader refuses `"body_json":null`. Round-trip test
   covers them, including an empty object and a zero-length body.
7. Duplicate keys and key order in a JSON body: carried verbatim; test with an
   object that repeats a key.
8. Huge numbers and exponents: carried as text; test `1e400` and
   `12345678901234567890`.
9. Redaction rewrites an inline body, so replay differs from live; the match
   can cross ops. Fence: rule A on the batch bytes `Encrypt` receives. Test: a
   batch whose op A holds a PEM BEGIN and whose op B's body holds the END is
   written as version 2; a body with a bearer token, an `api_key=` and an
   `sk-` token each lands the batch as version 2 and replays equal. (There is
   no exported wrapper and no per-op test.)
10. Depth: a body near 9,997 deep passes `json.Valid` and the marshal and
    fails the batch decode, a `SyntaxError` that a sealed replay downgrades to
    "applied as no change". Fence: rule B. Test: a body nested 9,990 deep is
    written as version 2.
11. Mixed-form batches: inline and base64 ops in one version 3 batch. The
    reader handles each op by field; test it.
12. A version 3 batch holding **no** inline op (writer bug): harmless but
    raises the fence for nothing. Test: version 3 iff any inline.
13. `ProjectionOp.InlineBody` is `json:"-"`: lost on a batch decode; only the
    writer uses it. Assert the constructor, not the decoder, sets the version.
14. `ProjectorVersion` pairing: `Validate` requires an exact pair
    (`projection_batch.go:366-367`); (3,2) must be added without loosening the
    others. Test every other pair stays refused.
15. A **second** producer of `body_json` that does not go through the version 3
    constructor (a future writer, or the unreadable stub at
    `appender.go:1801`). Fence: a guard test, as `event_table_guard_test.go`
    does for `Table`, that the only constructor of a batch that holds
    `body_json` is the version 3 path. No `Validate`-time refusal of a version
    1 or 2 batch containing `body_json`. This is not because the check is
    impossible: `Validate` already parses every op body into a map
    (`projection_batch.go:425-432`, `rejectProjectionPresence`), so a
    parse-based check scoped to object ops would not false-positive on old
    tape. It is dropped on impact: no producer can emit it without bypassing the
    constructor (the guard test fences that), the reader reads `body_json` in
    any batch version correctly anyway, and after the release gate no
    pre-reader binary can read such a batch in the first place, so a refusal
    would protect no reader that exists.
16. The upcaster (`vocab_upcast_deposit.go:560-675`) re-encodes with
    `json.Marshal`; after decoding an inline op it re-emits base64. Fine in
    memory. Test the upcaster over an inline op and over a mixed batch.
17. `HTTPStore.PutObject` (`http_store.go:45`) keeps `body`: an object copied
    from tape to the platform is re-encoded base64 because `Object`'s default
    marshal is unchanged. Test: a marshal of a decoded inline object yields
    `body`, not `body_json`. Also a guard test that `HeaderRow`, which embeds
    `Object`, is never JSON-decoded.
18. `Object.UnmarshalJSON` slows every object decode (alias struct plus
    `RawMessage` copy), and each write adds `json.Valid`, a marshal, a batch
    decode and a batch regex scan (rule A's `findSecretMatches` duplicates the
    redactor's own scan, so two regex passes run per batch). Estimate a few percent of decode cost and
    some write CPU; measure with the existing bench. Not a correctness risk.
19. A decoder that disallows unknown fields would refuse `body_json` on an old
    type. Grep for `DisallowUnknownFields` on object decodes before release 1.
    It is used in many places (for example `internal/selfdevprotocol/control.go`,
    `internal/deskprotocol/surface.go`), not only the floor file; none was found
    that decodes `objectgraph.Object` directly, and a custom `UnmarshalJSON`
    bypasses strict unknown-field checking on the type anyway.
20. Product restore to a checkpoint pinned to a release that declares an epoch
    below 7 is refused after the floor exists (section 4); a release declaring
    no epoch produces no refusal (`updater.go:590-593`, `1010`). Verify the
    restore path shows the owner a named refusal, not an opaque failure, for
    the refused case.
21. Snapshot chunks or any other consumer that copies `ProjectionOp.Body` into
    a place an older decoder reads (section 6, not verified).
22. A **platform** path that reads the guest tape (a purge/archive tool)
    and parses object ops for their body would see `body_json`. The anchor
    `tape_purge.go:367` is a SQL `row_to_json` row dump (`:348`) written to
    `rows/*.jsonl`, not a tape-op parser, so it shows no such reader; the grep
    for a platform op-body reader is still open. Grep before release 2.
23. Telemetry sealer's `recheckTelemetryOp` (`telemetry_sealer.go:540-560`)
    runs `json.Unmarshal(obj.Body, &ev)`: with the reader, `obj.Body` is the
    raw bytes; unchanged. Test the sealer over inline.
24. Counter series break (section 6) misread as a regression by a later
    investigator. Note it in the scope text and in `ACTIVE`.
25. Automatic rollback is refused from the writer release's first boot
    (section 4): a failing writer release stays running. The mitigation is
    rollback to release 1, which is legal, plus section 5 option 2.
26. Fork semantics: a restore to a checkpoint ahead of an abandoned tape
    branch; I did not read whether the tape past the restore target is kept.
    If it is kept and an old release continues from the target, it meets a
    version 3 batch past the target and refuses. Loud, but verify. (O5.)
27. Reader and writer both in one release by mistake: a rollback then lands on
    a binary that cannot read version 3. Fence: the owner's landing procedure
    checks that the previously deployed release already carries the reader
    (checked from the binary, section 5), not code.
28. Old tape stays readable forever: a fixture batch from each historical
    version replays equal, and a test fails if `UnmarshalJSON` drops the
    `body` branch.

## 9. Proof

Repo tests first (yellow, written before the code, per CLAUDE.md):

- **Round-trip byte equality**, for every registered object kind, using a real
  fixture body per kind taken from existing test fixtures, plus the property
  test of section 3: `decode(encode(B)).Body == B`, and `ContentHash` equal
  across forms. Add one assertion on the **final batch bytes**, not only on the
  op.
- **Mixed-form batches**: inline, base64-forced (non-JSON, escape-needing)
  and edge ops in one batch; replay equal to the live store.
- **Batch-level fallbacks**: a cross-op PEM pair, a secret-shaped string in
  any op, and a 9,990-deep body each produce a version 2 batch with base64
  bodies that replays equal.
- **Old-form tape replay equality by canonical witness**: a fixture tape of
  version 2 batches written by the pre-change code replays under the new
  reader and produces the same version 4 canonical witness (`304a81277`) as
  today; the same tape re-written inline produces the same witness.
- **Old-binary refusal**: a test, using the `Validate` of the pre-change code
  vendored as a fixture, or the pattern in `event_table_old_binary_test.go:15-50`,
  that a version 3 batch is refused with `ErrProjectionBatchInvalid`, not
  applied as a bodyless object. (This simulates an old binary; it is not one.)
- **Floor**: the failure 3 test; and `WriteFloor` at an existing floor of 7 is
  a no-op.
- **Guard tests**: the version 3 constructor is the only producer of
  `body_json`; `PutObject` emits `body`; `HeaderRow` is never JSON-decoded.

Deployed:

- **Whole-tape replay equivalence** on the test computer: replay the tape
  (the `ReconstructInto` dry run, reached through the replay-completeness probe
  at `replay_completeness.go:393`; the proof names the route or schedule that
  invokes it on that computer) and compare the witness to the live store,
  before and after the first version 3 batches.
- **Restore drill on d@e only**: restore to a checkpoint **after** release 2,
  observe the floor file; try a restore to a pre-release-1 checkpoint, observe
  the named refusal, not a misread.
- **Saving**: the `PlaintextLength` reading and the offline re-encode on d@e's
  tape copy (section 7).
- A clean `git status`, the pushed SHA and CI/deploy status as CLAUDE.md
  "Final report" requires, per release.

## 10. Rollback

- Release 2 (writer): revert the commit. The reader (release 1) stays, so
  version 3 tape written meanwhile stays readable. The floor stays at 7; it is
  never lowered. The previous release (release 1) is a legal target.
- Release 1 (reader): revert is safe while no version 3 batch exists. After
  release 2 has written one it is not: the revert must not be taken, and the
  floor refuses it (the epoch). Even before then `Apply` of an epoch-6 release
  is refused once the receipt reads 7, and `check-store-schema-floor.sh`
  blocks a lowered `Version` (section 5).
- Nothing is rewritten. Old tape is never touched.

## 11. Open questions, with conservative defaults

- **O1. Version pair (3,2) vs (3,3).** Default (3,2): the projector did not
  change.
- **O3. Proof on the test computer before the largest.** Default: section 5
  options 2 and 3 together, no new mechanism. Owner decides whether to
  restore a soak floor for this release.
- **O5. Restore past an abandoned branch.** Default: treat as loud-refusal
  and verify in the restore drill.
- **O7. Redaction gap on base64 bodies** (a secret in a body is not redacted
  on the tape today, and a base64 token can start with `AIza`). Default:
  record as its own problem, do not fix here.
- **O8. Epoch bump in the reader release makes `Apply` of epoch-6 releases
  refused, and blocks a release-1 revert via
  `scripts/check-store-schema-floor.sh`.** Default: accept, as the earlier
  epochs did (section 5).

(O2 and O4 of the first draft were the floor memo and are deleted; O6 was not
applicable.)

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

## 13. Review round 1

Two seats (Sonnet, DeepSeek v4.1 flash), both REPAIR. Every finding was
checked against the code. Round 1 repaired by subtraction: no mechanism was
added.

| Finding | Seat | Disposition |
| --- | --- | --- |
| Redaction fence tests the wrong bytes; a match can cross ops (PEM `(?s)` patterns) | both, blocking | Accepted. Replaced by rule A on the assembled batch bytes; section 3.2 rewritten; the exported wrapper and per-op tests deleted. `Encrypt` receives exactly those bytes (section 3). |
| The first draft's stated consequence ("the probe fails") was wrong; live and replay agree and the hash is stale | DeepSeek | Accepted, verified at `appender.go:689` vs `:1755`; rationale rewritten. |
| "Base64 is opaque to the patterns" is false (`AIza`) | DeepSeek | Accepted; stated in 3.2 and O7, not fixed here. |
| Floor memo, stat identity, `inlineSeen`, rules 1 to 3, O2, O4 | both | Accepted, all deleted; unconditional write from the `Reopen` hook (`open.go:391-415`). F1 stays as the independent backstop; F3 was a restatement of F1 and is folded into it. |
| Refuse `body_json` in a v2 batch (Sonnet: drop; DeepSeek: extend to every version below 3) | disagree | Dropped (failure 15). Round 1's reason ("a text scan that can refuse valid old tape") was wrong: `Validate` already parses op bodies, so a parse-based check is possible. The drop stands on impact, stated in failure 15 (round 2). The guard test remains. |
| Epoch gate incomplete; "platform needs no change" false | DeepSeek B2, Sonnet 2 | Accepted; section 5 states what is refused, the operator action and the d@e layered release; "has the reader" checked from the binary. DeepSeek's dissent (a known cost of every bump) is correct and stated as such. |
| Depth: batch decode fails on a body near 10,000 deep | Sonnet | Accepted; rule B decodes the final batch bytes and compares bodies; the depth cap and the per-op self-check are deleted. |
| Self-check decodes the op, not the batch | DeepSeek | Accepted; subsumed by rule B. |
| `"body_json":null` | Sonnet | Accepted (section 2, failure 6). |
| Both-present refused, `omitempty` | DeepSeek | Accepted (section 2). |
| Two `len(op.Body)` consumers | DeepSeek | Accepted and extended: the code has more (group commit, telemetry sealer, media chunking, counters); section 6. |
| Invalid UTF-8 reason | DeepSeek | Accepted (section 3). |
| About a quarter vs 19% | DeepSeek | Accepted (section 6). |
| Drop the reason breakdown from host-visible telemetry | DeepSeek | Accepted (sections 3.1 and 7). |
| New `sealed_*` counter vs `PlaintextLength`; offline re-encode | both | Chose the two readings that need no code: `PlaintextLength` and the re-encode on d@e's tape copy (section 7). The counter is deleted. |
| `HeaderRow` embeds `Object` | Sonnet | Accepted as a guard test (failure 17). |
| CPU cost per write | Sonnet | Accepted (failure 18). |
| Re-marshal sites, `PutObject` pin test | Sonnet | Already failure 17; kept. |
| Name the deployed replay-proof trigger | DeepSeek | Accepted (section 9). |
| Old-binary test is a simulation | DeepSeek | Accepted, said in section 9. |
| O8 omits `check-store-schema-floor.sh` | DeepSeek | Accepted (O8, section 10). |
| Reduce to a core of four or five pieces | both | Accepted: reader, eligibility test, batch version 3 with the two batch checks, unconditional floor 7. |
| Sonnet: a depth failure is a `SyntaxError` silently downgraded on sealed replay | Sonnet | Checked: `appender.go:1805` does downgrade it, as stated. But on a sealed live write the existing decode at `projection_tape.go:240` already fails the write loudly, so the reviewer's silent case needs a batch that decodes live and not at replay, which I could not construct. Rule B is kept because it turns that loud failure into a fallback and covers the unsealed path. |

## 14. Review round 2

Two seats (Sonnet, DeepSeek v4.1 flash). Both **ACCEPT**; neither found a
blocking defect, and all three round-1 blockers (the redaction fence, the epoch
gate statement, the dropped version 2 check) were found closed. Every
should-fix was checked against the code at the cited line and is adopted above
as a correction; none adds a mechanism to the reader release.

| Should-fix | Seat | Disposition |
| --- | --- | --- |
| 1. The gate must say the builder used for releases 1 and 2 is at epoch 7; `builder/request.go:275` stamps the builder process, so an epoch-6 builder declares 6 and release 2 is refused on a receipt-7 store | DeepSeek | Accepted, verified (`request.go:275`, `builder.go:56-62`, `updater.go:524-527`, `596-597`). Landing gate in section 5: both release manifests declare `store_schema_version` 7. |
| 2. Restage allows an undeclared epoch (`updater.go:590-593`, `1010`), so "below epoch 7 is refused" is not universal | DeepSeek | Accepted, verified. Sections 1 and 4 and failure 20 qualified; F1 is the backstop; a restage check is a release 2 choice. |
| 3. The rationale for dropping the version 2 `body_json` check was wrong (`Validate` parses op bodies, `projection_batch.go:425-432`) | DeepSeek | Accepted. Failure 15 and the round 1 table rewritten to the true reason. |
| 4. State the fallback mechanics: checks run once on the version 3 bytes, the version 2 fallback is written unchecked, the self-decode compares against a carrier of the pre-encode bytes | DeepSeek | Accepted. Section 3, "Fallback mechanics". |
| 5. A floor-write failure must fail `Open`/`Reopen`, not log and continue as the existing hook does (`open.go:409-414`) | DeepSeek | Accepted. Section 4. |
| 6. Only one pattern, `private_key` (`redaction.go:27`), is `(?s)` | DeepSeek | Accepted, verified. Section 3.2; the cross-op scenario stands. |
| Note: `PlaintextLength` is post-redaction (`privacy.go:341`) | DeepSeek | Accepted. Section 7. |
| Note: failure 19's `floor.go:54` parenthetical is wrong | DeepSeek | Accepted; `DisallowUnknownFields` is used widely. Failure 19. |
| Note: failure 22's anchor is a SQL row dump, not an op parser | DeepSeek | Accepted, verified (`tape_purge.go:348`, `367`). Failure 22. |
| Note: `min_store_schema_version` has no producer outside the platform update request | DeepSeek | Accepted, verified (`platform_update.go:48,236`). Section 5. |
| Note: section 5 option 1 (no `Ring-Expedite`) conflicts with the operating contract | DeepSeek | Accepted. Every non-docs landing carries the trailer; there is no untrailered option. Section 5 option 1 and the recommendation rewritten. |
| 1. Derive the batch version from the bytes, not from a side flag | Sonnet | Accepted, in section 3 "Fallback mechanics": rule B asserts version 3 iff some op is inline. |
| 2. Specify how the fallback rebuilds base64 ops | Sonnet | Accepted, section 3, with a test that the version 2 output holds no `body_json`. |
| 3. Name rule A's duplicate scan cost in failure 18 | Sonnet | Accepted. Failure 18. |
| 4. Writing floor 7 at every `Open` also floors tool opens, capsule copies and test directories; check for tests that assert a floor file's absence | Sonnet | Accepted as a release 2 task: grep the tests before the writer lands. Not a reader release item (the reader writes no floor). |

Not verified in round 2 (as in round 1): snapshot chunk writers (failure 21)
and any platform reader of tape op bodies (failure 22).
