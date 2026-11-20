# Story publishes fail with "failed to load private texture revision": the paper's revision read outlasts the proxy's 30 s

Date: 2026-11-20 (read at 22:42Z by `date -u`). Status: cause believed from the
code and the console, not proved by a timing of the read; no fix landed.
Class of the fix: orange (see Next).

## Problem

On the paper computer (`vm-1669020adcece51c2f5ff9b6df399ef5`, Node B), about
half of the story publish outcomes are 502s. The guest logs

```
story publish: <doc> v<n> (revision <rev>) failed and stays a gap:
/internal/platform/texture/publications: status 502: {"error":"failed to load private texture revision"}
```

Each such revision is never retried: its version stays a gap.

## Evidence

Where the message comes from.
- `internal/proxy/platform_publish.go:286-288` (the guest publish route,
  `HandleGuestTexturePublication`, publishing as `publish_mode auto`) and
  `internal/proxy/platform_public.go:282-284` (the owner's proposal route)
  write the 502 when `fetchAutoputerJSON` of
  `GET /api/texture/revisions/{id}` fails. The call is
  `internal/proxy/platform_publish.go:719-743`: an `http.Client` with a fixed
  `Timeout: 30 * time.Second`, to the guest at `10.210.217.2:8085`. The document
  read just before it (`:269`) does not fail in the journal.
- The real error is on the host, in `go-choir-proxy` (journal, `log.Printf` at
  `platform_publish.go:287`): `proxy: platform publish fetch revision: call
  autoputer: Get ".../api/texture/revisions/<id>": context deadline exceeded
  (Client.Timeout exceeded while awaiting headers)`. The guest console carries
  only the 502 body, no cause.

Counts.
- Proxy journal since 2026-11-18 16:53Z (its first line), 1273 `platform publish
  fetch` lines. On 2026-11-20, 679 lines, 678 of them `fetch revision` with
  `context deadline exceeded`; the one other is a model-provenance read that is
  logged and tolerated. No 404, no 401/403, no decode error, no other status,
  on any day. So not a missing revision, not custody or auth, not a size
  refusal: the guest does not answer within 30 s.
- Per hour (UTC), fetch-revision timeouts: 11-18 16h 3, 17h 3, 18h 5, 19h 6,
  20h 29, 21h 7, 22h 25, 23h 18; 11-19 00h-23h 23,18,12,10,12,17,22,10,5,6,17,
  13,24,41,24,41,44,23,20,23,20,28,26,20; 11-20 00h-22h 26,10,28,20,37,29,33,26,
  31,26,26,31,29,27,29,28,43,3,38,38,40,48,31. It is a steady 10-48 an hour for
  as far back as the journal goes (about 2 days), not a step. The one dip is
  17h on 11-20 (3): the paper booted at 16:39:57Z (vmctl, destructive kill and
  boot of the VM), and the proxy counts stay at 6-9 per 10 minutes until 16:5x,
  then 2-3 for 17:00-18:00, then 5-10 per 10 minutes from 18:00 (one-hour
  count in the guest console below). The earlier boots today (15:03Z, 16:00Z)
  show no step either. No landing today moves the rate: the rate is the same
  on 11-18 and 11-19, before every landing today.
- Guest console, `console.log` to `console.log.4` (rotated every ~20 min, so only
  21:11:28Z to 22:42:58Z survive; older consoles are gone): 147 story publish
  outcomes with a version: 70 failed, 77 published. By hour: 21h 39 failed /
  47 published; 22h 31 failed / 30 published (22h to 22:42). The 85-of-176 an
  earlier reader saw over 20:49-22:18Z cannot be rechecked: the console no longer
  holds 20:49-21:11; the proxy journal's 21h and 22h counts (48 and 31, to 22:42)
  agree with the console's 39 and 31 within the hour edges.
- Underlying slow call. All 70 console failures have, at the failure second, a
  guest line `texture api: list source graph objects for revision <that rev>:`
  either `objectgraph sql: scan object: context canceled` (55) or `query
  texture source refs for revisions: objectgraph sql: begin key index tx:
  context canceled` (15). 70 of 70 failed revisions have one; there are no such
  lines for other revisions. These come from
  `internal/textureowner/texture.go:1274-1296`
  (`attachTextureSourceGraphObjects`, called by `HandleTextureRevision`,
  `texture.go:1424-1450`, on every revision read) and are the handler's
  context cancelling when the proxy's 30 s client gives up. So the read hangs
  in the objectgraph SQL, not in the revision row read.
  `internal/store/texture_source_graph.go:125-151`
  (`listTextureSourceEntitiesForRevision`) reads every source entity of the
  owner and computer (`ListTextureSourceEntitiesByScope`) and filters to the
  revision in Go, so each revision read scans the whole scope's source
  entities, and the cost grows with the paper's corpus (read of the code; not
  timed).
- The guest was saturated while it happened: 1258 `dispatcher: activation
  wait` lines, 1238 with `slots=12/12` (due agents 47-49, waits of minutes);
  an `objectgraph sql: derived keys actor_wake_outbox.projected` re-check
  ran the whole window at about 10 rows/s (160800 of 462423 at 21:11:44Z,
  208400 at 22:42:52Z, 254023 left), which fits a start at the 16:39Z boot
  (4.5 h at 10 rows/s is 162k). That pass is a competing reader of the same
  tables. Existing records of the family:
  `docs/problems/derived-key-validation-reruns-every-boot-2026-10-27.md` and
  `docs/problems/derived-key-validation-rereads-og-objects-every-boot-2026-11-01.md`.
- Which stories (console window): 43 distinct stories had a failed version; 27
  failed once, 9 twice, 4 three times, 2 four, 1 five (each failure is a
  different revision: 70 distinct revisions, never the same one twice, so no
  retry succeeded or was tried). After the last failure in the window: 17 of
  the 43 published a later version (the gap closed by the next revision), 15
  had published an earlier version only (the failed one was the latest), and
  11 have no publish at all in the window (they may have published before
  21:11Z; not checkable). So the failure is spread, not the same few stories;
  26 of 43 (15 + 11) were left without a published version at or after their
  failed one when the window ended.
- Cost. A failed publish is off the run's path
  (`internal/textureowner/texture_cell_author.go:497-507`:
  `AutoPublishRevision` enqueues and returns; `story_publish.go:113-144`, one
  goroutine per document, `storyPublishTimeout` 2 min, no retry,
  `story_entities_source.go:178-180` logs and returns). The revision was
  already committed, the model calls and searches that wrote it are already
  spent, and nothing re-runs them: the failure line is followed by no further
  line naming the document for that purpose (the console has 58 lines naming
  the one document I checked, none a retry). The cost is a missing public
  version (and the entity export that follows a publish, `:181-184`, which
  does not run), plus a guest goroutine and a proxy request held for 30 s per
  failure.

## Belief

About 85 percent: the cause is the guest's revision read taking more than 30 s
inside the objectgraph source-entity query, because the paper's guest is
saturated (12/12 dispatcher slots, a backfill re-checking derived keys) and the
read scans the whole scope's source entities. The proxy's fixed 30 s then
turns it into the 502. It is load, not a race or a missing revision: the
revisions exist (the guest logs the revision it is reading by id), and no
refusal of any other kind appears in 1273 lines.

## Remaining error

- No timing of the read itself: the guest has no duration on
  `HandleTextureRevision`, and the proxy lifecycle row
  (`platform_publish.private_read` `revision_error`) was not read. The
  cancellation lines say the query was running at 30 s, not what it was
  waiting on (a lock, the derived-keys pass, the 12 busy runs, or the scan).
- Whether the derived-keys pass is the cause: failures predate the 16:39Z boot
  (they are in the journal from 11-18), so it is at most one contributor.
- The 85-of-176 count and the 20:49Z start could not be rechecked; consoles
  older than 21:11Z are rotated away. "Never published after" is bounded by
  that window.
- The 26 of 43 may have published later than 22:42Z or earlier than 21:11Z.
- Distinct stories failing before 21:11Z are unknown; only the proxy
  journal shows those, and it carries revision ids, not documents.
- I did not read the objectgraph query plans or `ListTextureSourceEntitiesByScope`.

## Next

- Orange fix, rollback revert. Two parts, in order: (1) the guest's revision
  read for the publish path should not scan the whole scope: read the entities
  by the revision's own refs (`listTextureSourceEntitiesForRevision`) and
  keep the entity attachment off the bare revision read the proxy needs, or
  time the read and log its duration on `HandleTextureRevision`; (2) the
  proxy's fixed 30 s (`platform_publish.go:729`) is a deadline, not a cure;
  a failed publish that a timeout caused should be retried by the publish
  queue (`story_publish.go`) with the same revision, as a failed publish
  stays a gap forever now. Protected surfaces: Texture canonical read path and
  the publication route to the platform (red if it changes what a publish
  commits; orange for a read path and a retry).
- First, read-only: put a duration on `HandleTextureRevision` and
  `attachTextureSourceGraphObjects`, or read the proxy's lifecycle
  `platform_publish.private_read` rows, to settle whether the scan or the
  contention is the time.
- Enter the claim in `docs/calibration/claude-ledger.md` before the fix lands:
  "after the fix the 30 s timeouts on fetch revision fall under 2 per hour";
  outcome is the proxy journal count per hour.
