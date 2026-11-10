# Lane F (asking and investigating): failure list, pass 1

Mutation class: green (docs). Rollback: revert. Written before the tests and the code.
Serves handoff section 6 item 7: the reader asks about an uncovered story and adds a source by asking.

## What exists

- An unfocused prompt-bar submit already creates a new Texture document on the asker's computer
  (v0 revision = the asker's words) and wakes the Texture desk, which researches and commits
  revisions. Nothing marks it as a story ask, and the desk is not told to investigate.
- A source schedule is a standing owner intent (`source_schedule.go`), keyed by a source id that
  must exist in the embedded registry. The owner route stamps `created_by_owner_act`. There is no
  way to schedule a source the registry does not list, so "add this feed" has no destination.

## Design (smallest change)

1. `POST /api/autopaper/ask` {text, command_id}: the existing prompt-bar submit path, unrouted,
   with run metadata `ask_intent=story`. The v0 revision records the question and `asked=story`;
   the desk's first objective is framed to investigate and to update the document as findings land.
   No new orchestration path.
2. `POST /api/autopaper/sources/schedules` takes `feed_url` (and optional `name`). The feed is
   validated with the real poller before anything is written; the validated definition rides on
   the schedule record (`feed_url`, `feed_kind`, `feed_name`), so boot recovery, the sweep and
   the poll tick need no new state. The id is `custom:<kind>:<hash of url>`: a replay is the same
   schedule.

## Ways it can fail (each has a test)

Ask:
- A1 empty or whitespace text: 400, no document.
- A2 text over the cap: 413, no document.
- A3 no session: 401.
- A4 replay of the same command id: one document, same ids.
- A5 the question is not recorded on the document: v0 content must equal the asker's words and its
  metadata must say `asked=story`.
- A6 the ask lands on another owner's computer or doc: the document belongs to the asker.

Feed add:
- S1 scheme other than http/https, userinfo, or a private/loopback/metadata address: refused with a
  named reason, nothing written (SSRF; `sourcefetch.ValidateURL` and the guarded client).
- S2 unreachable or non-2xx: refused, "feed not reachable".
- S3 reachable but not a feed (HTML, JSON): refused, "not a feed that parses".
- S4 a feed with zero items: refused, "feed has no items".
- S5 an oversize body: the read is bounded by `EffectiveMaxBodyBytes`; refused, never buffered whole.
- S6 a desk's in-process call (not the owner): refused, "adding a feed is an owner act". The feed
  URL can come from web-derived text; only the owner's own request adds one.
- S7 too many custom feeds per owner (cap 50): refused with the cap named.
- S8 a duplicate URL: the same schedule restarts, no second schedule.
- S9 a custom feed's poll must work with no registry entry (the poll resolves the definition from
  the schedule).
- S10 a crash boot keeps the custom schedule and its definition (standing intent); the in-flight
  poll closes as today.
- S11 a stopped custom schedule stays stopped across boot.
- S12 `feed_url` together with `source_id`: 400 (one or the other).

Not built (pass 2): non-RSS kinds beyond telegram by URL, per-owner source removal UI, a limit on
poll rate beyond the existing one-minute floor.
