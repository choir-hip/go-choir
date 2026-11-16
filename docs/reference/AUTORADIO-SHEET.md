# The server run sheet: `/api/autoradio/sheet`

Status: built in slice D1 (lane `runsheet-server`, 2026-11-16, revised after
the two-seat review); not deployed, not proven. Types, reducer and fixtures:
`internal/runsheet` and `mobile/fixtures/runsheet/`. Code:
`internal/agentcore/api_autoradio_sheet.go`. Proof script:
`scripts/prove-autoradio-sheet` (run by the lead on d@e).

The custodian's copy of one listener's run sheet. The phone keeps its own
copy, plays from it, and posts its ops here; the server sequences them,
assigns versions and echoes every change. Authentication is the turn route's:
a signed-in client, or an owner-wide key with `X-Choir-Computer`. One sheet
per owner per computer. `by` and `placed_by` are claims of the client of that owner; the
route only refuses `by: desk` (the desk acts in process).

## Routes

| Route | Answer |
|---|---|
| `GET /api/autoradio/sheet` | `200`, a `RunSheet` (`entries` is `[]`, never null; the reducer's `applied_ops` memory is not sent). `sheet_epoch` is present after a repair (see Replay). `503` when a tape query fails (a store read error; the answer is repeated for a few seconds without reading the tape again) or when the sheet is being rebuilt and is not yet settled after three tries (try again) |
| `POST /api/autoradio/sheet/ops` body `{ops:[Op]}` | `200 {version, results:[{op_id, applied, dropped?, version}], sheet_epoch?}`, one result per op in order; `sheet_epoch` appears only on a repaired sheet and when the sender's does not match (the body may carry `sheet_epoch`: the epoch the sender holds), and means the sender must take `GET /api/autoradio/sheet`. `400` names the field (`ops[2].op`, `ops[0].op_id`, `body`, `ops`); an unknown op, kind, state or enum value is `400` (nothing in the batch applies). More than 64 ops is `400`; a body over 256 KB is `413`; over the hourly bound `429` with `Retry-After`; a tape failure `500` (the ops before it are durable, the failing op and the rest are not acknowledged; the sheet is dropped from memory and rebuilt from the tape on the next call) |
| `GET /api/autoradio/sheet/stream?after=<version>&epoch=<sheet_epoch>` | SSE. Begins with the comment `: autoradio`. Then `sheet` (a full `RunSheet`) when `after` is absent, older than the server keeps (the last 256 applied ops, which must be every version after `after`: a stepped-over version makes a hole and so the full sheet), ahead of the server, **or when the sheet has an epoch the reader does not hold** (`epoch` absent or different), even at the current version; `sheet_ops` `{from_version, to_version, ops}` when `after` is kept (none when `after` equals the version). After that one `sheet_ops` per batch that changed the sheet, including the echo of the sender's own ops; `to_version - from_version` equals the number of ops. A dropped op changes nothing and is not sent. A `: ping` comment every 20 s. `429` when 4 streams are open, `503` when the sheet is being rebuilt and not yet settled after three tries (reconnect). Every write has a deadline of 15 s plus a second per 32 KB (a full sheet can be 5 MB), so a half-open client is released; a reader that falls 64 events behind, or whose sheet was rebuilt, is closed: reconnect with its last version **and the `sheet_epoch` it holds** (GET, POST answers and the first `sheet` event tell it the epoch; a repaired sheet sends the full sheet to a reader without the right epoch) |

## What a phone must do with the answers

An applied op's answer is exact for ever: the tape record is its memory, so a
replay of the same `op_id` returns `applied` at its version, also after a
restart. A repeat of an `op_id` inside one batch is `duplicate` (reducer rule
6).

**A drop is not durable.** The server keeps the answer for the last 2048 op
ids of a process, so a quick retry gets the same answer, never with a version
below the sheet's. After a restart (or 2048 later ops) the same op is judged
afresh against today's sheet and may apply: `ahead_full` once a remove frees
room, `duplicate_id` once the entry is gone, `not_ahead` after a `previous`.
So the phone must apply the `sheet_ops` it receives (and replace its copy
when it receives `sheet`), even for an op it was told was dropped, and must
not set its version from a `results[].version` that is below its own.

## Dropped reasons

The reducer's, returned as they are: `duplicate`, `not_ahead`, `not_on_air`,
`not_phone`, `bad_transition`, `other_on_air`, `unknown_id`, `stale_version`,
`no_previous`, `duplicate_id`, `reason_required`, `invalid`, `ahead_full`
(from `previous`, see below). The route adds:

| Reason | When |
|---|---|
| `by_desk_refused` | an op with `by: desk` over HTTP. The desk acts in process only (a later slice); a client cannot speak as the desk |
| `placed_by_mismatch` | a `place` whose entry's `placed_by` is not the op's `by` |
| `replay_skipped` | an op whose tape record a replay could not fold (it did not decode, today's reducer drops it, or another record took its version): the op is not on the sheet and never was; the sender takes the sheet |
| `record_too_large` | an op whose record would exceed 64 KB (see Bounds); nothing was applied or written |
| `ahead_full` | a `place` (other than `at: "now"`, which adds nothing ahead) while 100 entries are ahead; the entry is not placed and not trimmed in; a `remove` frees room |

## Bounds

Per request: 64 ops, 256 KB. Per op (reducer): `op_id`, `id`, `after` 64
bytes, `reason` 160, `origin` 160, `ended.at` 40, `position_seconds` 0 to
86400, `text` 8 KB and only on `rewrite`, an `entry` only on `place` (an op
carries nothing it does not use: it is stored whole). Per entry (reducer,
shared with the Swift twin by fixtures `23`, `24`, `32`): `id` 64; `title`
300; `text` 8 KB; `source.ref` and `source.url` 2048 (real feed and enclosure
URLs run long), `source.label` and `clip_id` 512; at most 12 `cites`, each
`ref` 2048 and `label` 512; `between` ids 64; `forecast.p_heard` in 0 to 1,
`caused_by` and `claim_id` 160; `ended.at` and `air_at` 40; `origin` 160;
`est_seconds` 0 to 86400; and the entry at most 32 KB **as a record carries
it**: the entry's compact JSON (HTML escaping off) marshalled as a JSON string,
as a record body is, minus the two quotes. So a plain ASCII byte costs 1, `<`
`>` `&` 6, a double quote or backslash 4, a control character 7 (U+2028 and
U+2029 too), other characters their UTF-8 bytes (fixture `33` pins the rule
for the Swift twin). It is checked at `place` and after every `rewrite`. An
entry over any bound makes its `place` `invalid`; an unknown enum value is a
`400` for the batch.

Measured maxima, in commitment-record bytes
(`TestSheetTheLargestValidOpRecordIsUnderTheStatedBound`: the real marshalled
record of a maximal entry, a maximal `rewrite` and a 151-entry snapshot with a
full `applied_ops`, for each of a control character, `<`, `>`, `&`, a quote, a
backslash, a multi-byte character, U+2028 and plain text): the largest op
record is **58,414 bytes**, the largest snapshot record **4,982,465 bytes**, the
largest sheet on the wire 4,921,601 bytes.

Bounds by construction, in **commitment-record bytes** (the record's real
marshal): an op whose record would exceed **64 KB (65,536)** is dropped
`record_too_large` before anything is applied or appended; a snapshot record
that would exceed **6 MB (6,291,456)** is not written and one line is logged
(the sheet stays correct from the previous snapshot plus the ops). The
reducer's bounds keep every valid op and sheet well inside both (the maxima
above); the guard is what the figures rest on.

Per sheet: 100 entries ahead; at most 4 open streams (`429`); 1200 ops not
already answered per clock hour, a whole batch refused when it would pass the
count (the count survives eviction of an idle sheet but not a restart).

**On the tape** a record is the base64 body of an objectgraph object
(`internal/objectgraph/object.go`, `internal/store/projection_tape.go`; see
`docs/problems/tape-object-ops-carry-more-than-twice-their-payload-2026-11-15.md`),
so it costs 4/3 of its bytes plus the object's envelope and metadata. Measured
by the same test with the real object marshalled: the maximal op record is
**78,571 tape bytes**, the maximal snapshot **6,643,978**; at the guard bounds
an op record is **88,057** and a snapshot **8,389,283**.

Worst case an owner can add to its own computer's tape in one clock hour: 1200
op records at the bound (one op per request) plus a snapshot every 128
versions (at most 10) at the bound: 1200 x 88,057 + 10 x 8,389,283 =
**189.6 MB** (at the measured maxima, 1200 x 78,571 + 10 x 6,643,978 = 160.7
MB); across an hour boundary the counter resets, so about **379 MB** in two
adjacent windows. In commitment-record bytes the same figures are 141.6 MB and
283 MB. Ordinary use is thousands of times smaller (a snapshot of a normal
sheet is tens of KB).

## Records

Each applied op is one commitment record: id `autoradio-sheet-op:<op_id>`,
directive subtype `notice` (no inbox, wakes nothing), `desk=app:autoradio`,
`record_kind=autoradio.sheet_op`, the owner-private label
`autoradio_sheet_op`, `computer_id` of the computer, subject refs
`sheet_ops=owner`, `op_id`, `version`, `by`; the body is `{version, op}`.
Every 128 versions, and after any repairing replay (id
`autoradio-sheet-snap:<version>-e<epoch>`, so it cannot collide with an earlier
snapshot of the same version), a snapshot record
`autoradio-sheet-snap:<version>` (`autoradio.sheet_snapshot`, ref
`sheet_snap=owner`, body the reducer's whole sheet) bounds replay. Dropped ops
write nothing. A `previous` that trimmed desk entries to stay within 100 ahead
carries the subject ref `trimmed=<ids>` (the entries whose forecasts have no
air evidence). A snapshot carries `skipped_ops` (the newest 256 op ids any
replay skipped).

## Replay (first use after a start, after an idle eviction, after a tape error)

A run sheet is a playlist: availability with a loud record beats refusal.

1. The base is the newest snapshot that decodes (paging deeper when the
   newest do not); none means an empty sheet.
2. Op records with a version at or below the base are not decoded at all.
3. The rest are read newest first, in pages that grow fourfold until a page
   reaches the base version or the tape is exhausted; the computer filter is
   in the query, so another computer's rows cannot shorten a page. They are
   folded in version order (ties by record time). Past 200000 records the
   replay stops reading and rebuilds from the newest ones alone (`truncated`),
   loudly: snapshots have then been failing for days.
4. A record that does not decode, or that today's reducer drops, is skipped
   and counted, **and consumes its version** (the sheet takes the version the
   record names, entries unchanged), so no later op reuses it. A version gap
   is stepped over the same way; the version is never recomputed. For a
   duplicate version the first candidate that decodes and applies wins (by
   record time); the rest are skipped. The op id of every skipped or
   deduplicated record is answered `replay_skipped`, never `applied`
   (remembered in the snapshot, so also once a snapshot has passed it).
5. A row whose envelope does not decode is counted (`unreadable`): it names no
   version, so it cannot be placed or paid for once, and it does not by itself
   repair.
6. A replay that skipped, stepped or deduplicated a version-bearing record
   gives the sheet a new `sheet_epoch` (the repair's time in milliseconds,
   always above the last), writes a repairing snapshot at the resulting version
   at once (damage is paid once), and logs one line: `autoradio sheet replay
   repaired owner=<id> skipped=<n> gaps=<n> dup=<n> unreadable=<n>
   truncated=<bool> epoch=<n> first=[v<version>:<op id>:<why> ...]` (the first
   eight). Every reader that does not hold the epoch is sent the full sheet.
7. Only a failed tape query fails the replay (`503`); it is remembered for 3 seconds so a
   retrying phone does not rescan the tape.

Replay only reads (and writes the repairing snapshot): a restart resumes
nothing. Any error from the tape append (it may or may not have landed) drops
the sheet from memory and closes its streams, so memory never runs behind the
tape; a tape record for an `op_id` is answered from only after the live sheet
holds that record's version (a record above a freshly replayed sheet is
answered `replay_skipped`). A sheet nobody has used for 15 minutes and that
holds no stream leaves memory and returns by replay.

Residuals:

- Two writers on one tape are not expected: a deploy refresh kills the old
  process and then boots the new one (`internal/vmctl/deploy_refresh.go`
  lines 19 to 28 and 444 to 445), so one process serves a tape at a time. If it
  ever happened, two records could take one version; the later is lost, and
  the loss is announced (`dup` count in the log, a new `sheet_epoch`, a full
  sheet to every reader). A writer lease is the fix if it is ever observed.
- Skipped op ids older than the newest 256 are no longer remembered and are
  then answered `applied` if retried.
- A future reducer change can reshape a repaired sheet (records the new
  reducer drops are skipped and the sheet is announced). Stamping a reducer
  version on op records is the planned guard.

## Amendments to the frozen interface

- **An answer already said.** A `place` by the phone of an entry of kind
  `answer`, `placed_by: phone`, `state: heard` is valid. It records something
  the phone has already said: it lands after the last heard or skipped entry,
  never goes on air, and `at` is ignored. (Fixture `26`.)
- **`previous` and the ahead bound.** `previous` returns two entries to ahead.
  When that would leave more than 100 ahead, the desk-placed ahead entries
  farthest from the air (not the two just returned) are dropped to make room,
  never a listener's or the phone's; with none to drop the op is dropped as
  `ahead_full`. (Fixtures `27` and `28`.)
- **Air reports (from the phone lane, `origin/lane/autoradio-integ`
  1651b6647).** `ended.outcome` has five values: `finished` (a natural end,
  with its final `last_block`), `skipped`, `cut` (the entry on air replaced by
  a `place` at `now`), `left` (sign-out or session end: `aired` by the phone,
  state `heard`, with the true `position_seconds`) and `failed` (the phone
  could not make the entry play: `aired` by the phone, state `skipped`; it is
  not the listener's judgement and settles no forecast). The reducer applies
  them like any `aired`; the tape keeps `by`, `state`, `ended` (`outcome`, `at`,
  `last_block`) and `position_seconds` exactly as sent, because these ops are
  the evidence that settles the desk's forecasts. (Fixtures `29` to `31`,
  `07` for `cut`.)
- **The 2026-11-16 additions** (`air_at`, `origin`, `revision` on an entry;
  `station`, `master_version`, `clock_at` on a sheet; `origin` on an op) are
  recorded in the types and on the tape, not interpreted: the reducer neither
  sets nor reads them (fixture `25`). Rule 8 and the `revision` increment are
  not built.
- Reducer bounds, listed above, are part of the interface (fixtures `23`,
  `24`, `32`), as are two op-shape rules (an `entry` on any op but `place`,
  and `text` on any op but `rewrite`, are `invalid`) and the sheet field
  `sheet_epoch` (server-assigned; absent until a repair; the reducer neither
  sets nor reads it); the Swift twin (`origin/lane/autoradio-integ`,
  `mobile/AutoradioKit/Sources/AutoradioKit/RunSheet.swift`) must read them.

## For the phone

- `previous` and `clear` take the **server's** version as `base_version`, not
  the phone's local one.
- A `429` (hourly bound) and a `413` are not "never retry": a `429` carries
  `Retry-After`; a `401` (an expired token) must not drop pending ops.
- An answer with `sheet_epoch`, or a `sheet` stream event, replaces the
  phone's copy.

## Master sheets later

Every record carries a sheet id (`sheet_ops` and `sheet_snap` refs; `owner`
today). A public master sheet is another sheet id under another owner
principal; its records take the id prefix `<sheet id>:` (never inside an
`op_id`), and the same store, reducer, replay and stream serve it. Only the
authorisation (a read without an owner session) is new.
