# The play queue and the Autoradio desk (2026-11-15)

Standing: built | kind: happened | scope: in part | settled-by: own status line 3 and section 4 line 191 (first slice built); internal/autoradioqueue; c00312841; docs/current-architecture.md:116 row built | note: section 6 line 211: the cell module and queue ops on the programme route not built

Status: design, with the first slice built on `lane/autoradio-queue-desk`.
Mutation class of this note: green. Rollback: revert.

## The ruling this serves

Owner, 2026-11-15, about midnight UTC: "the play queue should be a real data
structure but also controlled by the ai dj. you should be able to queue
something to it in the app, but also change the playback order by talking, and
the ai dj (autoradio desk) changes the queue struct in its go repl cell." And:
"as soon as you open autoradio, it should start playing. eventually we will have
multiple stations available but for now one station per user."

Evidence marks: **[code]** read in this repository at `59c083641`; **[assumed]**
reasoned and not verified.

## 1. What the queue is

One station per user. The queue is that station's state: one `current` entry
(what is playing, with its position) and an ordered list of `upcoming` entries.
Later stations are more instances of the same structure, each with a
`station_id`; tonight there is one, so the wire carries no station field and
adding one later is additive.

An entry is `{id, item, added_by}`. `id` is minted by the phone when the entry
enters the queue, is unique within the queue, and never reused; every op the
desk sends names an entry by `id`, never by position alone. `added_by` is
`listener` (tapped in the app), `dj` (the desk's op), or `programme` (the
programme stretch put it there). An item is one of:

| kind | fields |
|---|---|
| `story` | `route`, `title` |
| `page` | `url`, `title` |
| `document` | `id`, `title`, `at` |
| `audio` | `url`, `title`, `show`, `start`, `end`, `episode_id` |

On the wire an item is a flat object with a `kind` discriminator
(`{"kind":"story","route":"...","title":"..."}`). `version` is a whole number
the phone increments on every change to the queue, including the current entry
finishing and the next one starting.

## 2. Custodian and the two copies

One custodian per data kind (CLAUDE.md, Desks): **the Autoradio desk is the
custodian of the queue as an ordering**. The desk alone decides what order the
DJ proposes; nothing else on the computer writes it. But the queue also has a
fact the desk cannot know: what is playing this second, and where. The phone
plays offline and the reader taps things. So the copy is split by field, not
by device:

- The phone is authoritative for **what is playing now and its position**, and
  for the effect of any tap. Its queue always plays, with or without the
  computer.
- The desk is authoritative for the **proposed order**: it proposes ordered ops
  against a version. The phone applies them. The phone's applied state, sent
  with the next request, is the desk's only view of the queue.

**The desk keeps no copy between requests tonight.** The phone sends the
snapshot with each request; the desk answers with ops. Where a desk must
remember (the learning engine, section 7) the record is the turn's Report on the
tape, which already exists (`autoradio-turn-rep:<turn_id>`). No new persistent
store is built.

## 3. The wire (frozen shapes)

### Request: the optional `queue` field on `POST /api/autoradio/turns`

```json
"queue": {
  "version": 7,
  "current": {"id": "q12", "item": {"kind": "story", "route": "front", "title": "A strike ended in Nancy"}, "position_seconds": 41.2},
  "upcoming": [
    {"id": "q13", "item": {"kind": "audio", "url": "https://example.test/ep.mp3", "title": "Harbour talks", "show": "Up First", "start": 120, "end": 176.5, "episode_id": "ep-9"}, "added_by": "dj"},
    {"id": "q14", "item": {"kind": "page", "url": "https://example.test/a", "title": "A long read"}, "added_by": "listener"}
  ]
}
```

`current` may be absent or null (nothing playing). `upcoming` may be empty.
Unknown fields inside `queue` are ignored (a newer phone does not break an older
server; the desk simply never sees what it does not know). Optional too:
`candidates`, `[{"route": "...", "title": "..."}]`, at most 40: the paper's
stories the desk may put in the queue (the phone sends its front page's). The DJ
can add only a story named there; it never invents an item. Bounds (bounds on
what a stranger can cause, not on model calls): at most 50 upcoming entries;
`id` is `[A-Za-z0-9][A-Za-z0-9_.-]{0,63}` and unique; a title at most 300 bytes,
a `url` 1024, other strings 128 to 256. A malformed snapshot is `400` naming
the field; a well formed one over a size bound is answered without the queue and
the desk says so (a listener's words are not refused because their queue is
long). The body bound rises from 80 KB to 256 KB, the sum of the part maxima, so
a queue of ordinary text within its bounds fits; `page.text` stays at 48 KB. The
phone sends at most the first 50 upcoming entries. The reference page has the
arithmetic.

### Response: the `queue_ops` event

Sent once, after the last `speech` event and before `done`, only when the
desk produced at least one op that survived validation:

```
event: queue_ops
data: {"turn_id":"cmd-7f3a9c","base_version":7,"replayed":false,"ops":[
  {"op":"move","id":"q14","to_index":0,"base_version":7,"reason":"you asked for the long read next"},
  {"op":"play_next","item":{"kind":"story","route":"sport","title":"Cup final"},"base_version":7,"reason":"you asked for the sport story"}
]}
```

Every op carries `op`, `base_version` (the same as the event's), and a one-line
`reason` in the DJ's words (at most 160 bytes), which the phone shows when it
applies the op ("moved: you asked for the long read next"). The ops are an
ordered list: each is meant against the queue as the previous ones left it.
The vocabulary is the phone's: `play_now`, `play_next`, `enqueue` (these three
carry an `item`), `remove`, `move` (`id`, `to_index`, an index into
`upcoming`), `skip`, `previous`, `clear`. The server's validator is stricter
than the vocabulary (section 4). `reason` is required: an op with none is dropped. `turn_id` is the dedupe key: the same turn
replayed sends the same event with `replayed: true`, and the phone remembers
the turn ids it has applied and applies each once.

### Stale versions: the phone's rebase rule

The queue may change during a turn (an entry finishes, the reader taps). The
rule is deliberately one sentence per kind of op:

1. An op that names an entry by `id` (`move`, `remove`, `skip`) applies if that
   entry still exists, whatever the version. `skip` names the current entry and
   applies only if it is still the current one. An op whose entry is gone is
   dropped silently. `to_index` is clamped to the list.
2. An op that adds an item (`play_now`, `play_next`, `enqueue`) always applies.
   It does not depend on the order around it.
3. An op about the whole queue (`clear`, `previous`) applies only if the
   phone's `version` still equals `base_version`; otherwise it is dropped.

A dropped op is not an error and is not retried. The next request's snapshot
shows the desk what is true. (`skip` and `move`/`remove` carry `id`; the server
fills `id` for `skip` with the current entry's id.)

## 4. How a spoken request reaches the desk, and how it changes the queue

The words go to `POST /api/autoradio/turns` with the snapshot. **The route is
the desk's fast turn: one streamed model call, no cell** [code:
`internal/agentcore/api_autoradio.go`]. The doctrine says desks act in a Go cell
with granted modules; the fast turn has no cell, and wiring a cell into it (an
activation, a worker, a module grant, the deskprotocol locks) is more than one
night. Tonight's slice therefore does the conservative thing and says so:

- The same single model call that writes the spoken reply also writes the ops.
  The queue block of the request tells the model to write its reply first and,
  only when the listener asked to change the order, to end with the request's
  own fence line (`<<<AR... queue>>>` with the request's token, given to it
  verbatim) and a JSON array of ops. The stream cut starts the ops only at that
  exact line, at the start of a line, with a `[` (or `{`) after it; any other `<<<` the
  model writes, and the line merely echoed in a sentence, is speech. The cut and the rules
  exist only on turns that carry a queue: the desk's system prompt is unchanged.
- The server validates every op through the **queue module** (below), which is
  the single place the rules live: unknown ids dropped, indexes clamped, an
  `enqueue` of anything but a named candidate dropped, at most 8 ops per turn
  (a bound on what a model steered by hostile page text can cause, not a
  model-call limit), malformed JSON dropped whole, and each op simulated
  against the queue as the earlier ops left it. If the model asked for ops and
  none survive, the desk speaks one more sentence ("Sorry, I couldn't change the
  queue.") so the voice does not claim what did not happen.
- A question ("what's coming up?") yields speech from the snapshot and no ops.

**The queue module.** `internal/autoradioqueue` holds the structure and its
reducer: `Snapshot`, `Item`, and a `Desk` holding a snapshot and the ops
proposed so far, with the methods the doctrine's cell would call:

```go
q := autoradioqueue.New(snapshot, candidates)
q.Snapshot()                         // the state after the ops so far
q.PlayNext(item, reason)  q.PlayNow(item, reason)  q.Enqueue(item, reason)
q.Move(id, toIndex, reason)  q.Remove(id, reason)
q.Skip(reason)  q.Previous(reason)  q.Clear(reason)
q.Ops()                              // the recorded ops, base-versioned
```

Each method validates, applies to its own copy, and records the op, or returns
an error. The fast turn calls it from the model's JSON; **the next step is the
same type bound into the desk's cell as the `queue` module** (a grant, say
`autoradio.queue`, whose verbs `QueueSnapshot`, `QueuePlayNext`, `QueueMove`,
`QueueRemove`, `QueueEnqueue`, `QueueSkip`, `QueueClear` resolve through
`deskmanifest` into `choir` exports, as `story.relate` does), where the desk
writes `choir.QueueMove("q14", 0, "you asked for the long read next")` in its
REPL cell and the route returns `q.Ops()` as the same `queue_ops` event. The
module records ops; it does not reach the phone. Only the route, which holds
the request, returns them. The wire above does not change when the cell
arrives; that is why the wire is frozen now.

**What was built (2026-11-15).** The conservative slice: `internal/autoradioqueue`
(the structure, bounds, reducer and the stream cut), the turn route's `queue`
and `candidates` request fields and `queue_ops` event, and the rules the model
reads in the request's queue block. The route is the contract:
[AUTORADIO-TURNS.md](../reference/AUTORADIO-TURNS.md). Review 1 repaired: the cut
ran on every turn (now only with a queue, and only at the exact fence), a
fence-only reply walked to a second pool member (now an answer), a replay re-sent
ops to a queue it was not written for (now only to the same version), and the
bounds could not hold a realistic queue.
`play_now` forgets the entry it replaces for the rest of the turn (the phone
decides what becomes of it); an entry added this turn has no id and cannot be
named by a later op.

## 5. How the reader sees why the order changed

Each op carries its own `reason`, written by the DJ in one line and bounded to
160 bytes. The phone shows it when it applies the op. The same ops and reasons
are recorded on the turn's Report (`queue_ops` and `queue_base_version` in its
subject refs), so the tape shows what the DJ proposed, in what order, and why.

## 6. Not built tonight

- The cell module (section 4): the `autoradio.queue` grant, the choir verbs, an
  activation for a voice turn. The fast turn has no cell.
- `queue` and `queue_ops` on `POST /api/autoradio/programme`. The clips lane is
  changing that route tonight (`podcast_clip`); keeping the edit out of its files
  is worth more than the field. The phone sends `queue` to the turns route only
  and must not send it to the programme route (unknown fields there are `400`).
- A server copy of the queue; multiple stations; the DJ adding anything but a
  named paper story (it cannot search for a podcast, a page or a document it was
  not told of; it can reorder or remove them, or play them by moving them to
  the front and skipping).
- The learning engine's forecasts and settlements for reorders (section 7).
- Autoplay on open: the DJ's first stretch starting the moment the app opens is
  the phone's behaviour and the programme route's `open` cause; nothing here
  changes it.

## 7. How this meets the learning engine

A reorder is a forecast: "you will want this next" ([the learning
engine](autoradio-learning-engine-2026-11-14.md), section 3). Each op that moves
or adds an entry toward the front, with its `reason`, would become one claim
(agent `autoradio-dj`, `caused_by: instruction:turn/<turn_id>` since the
listener asked, or `importance` / `explore` when the DJ moved it unasked, and a
`p_heard`). Settlement is listening: the phone's air event for that entry id.
The entry heard past the heard rule's threshold settles the claim true; skipped
within the first seconds, or removed by the reader afterwards, false. An op the
phone dropped as stale has no outcome and is void, not false. Tonight the route
writes no claim for an op; the Report carries the ops so a later station can
write the claims from the tape without a migration. Not built.
