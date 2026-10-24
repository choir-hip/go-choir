# Engineering sees a Texture Cast's objective cut at 4 KiB, with no way to read the rest (2026-10-24)

Mutation class of the fix: orange (engineering's Records() view of a Cast;
no tape or canonical-write change). Status: documented first, fix follows.

## Evidence

Recorded in the v5 attempt of
`docs/problems/staged-engineering-rounds-break-on-desk-reads-of-blocked-2026-10-24.md`
(branch `lane/fp0-apprewrite`, "Related findings from the next attempt").

- `engineeringRequestObjectiveMax = 4 << 10`
  (`internal/agentcore/engineering_cast_request.go`). Engineering's
  `Records()` view joins the parent Texture Cast in, but cuts its objective
  at 4 KiB (the body at 16 KiB, `engineeringRequestBodyMax`). The cut ends
  with "the full text is on the tape in record cell:...".
- Texture put the whole request and spec into the Cast's objective: 6,338 B.
  Engineering saw the marker at 09:40:27, spent about ten cells trying to
  read past it, found no way to read that record, and reported it
  (09:44:26, 10:24:26). The lost third held the allowed-files,
  package.json and completion paragraphs; Podcasts was not built and
  package.json was edited against a rule that was in the cut part.
- The v6 request kept the brief to 3,342 B and its view read
  `truncated:false`.

## Findings from the code (read before any change)

- No in-cell verb reads a commitment record by ref. `choir.Records()` returns
  only unread inbox records as of cell start; `choir.Pack()` returns acts and
  resolutions, not directive text; `choir.ReadFile` reads the capsule
  workspace, not the tape. Engineering cannot reach the Texture Cast record
  the marker names, so the marker promises a read that does not exist.
- A Texture Cast's objective is not bounded by 4 KiB or 16 KiB anywhere
  upstream: `Tray.Cast` stages it under only the 256 KiB aggregate tray
  quota (`MaxTrayBytes`). The 16 KiB `MaxIntentBody` applies to Note bodies,
  not to the Cast objective. So a single objective can exceed 16 KiB, but
  the objective cap of 4 KiB was a third of what the sibling body cap
  allows, for no stated reason.
- The view is derived from the tape at every read and the binding's digest
  covers the full parent text, so the cut is display-only.

## Options

1. Add a read-by-ref verb (`choir.ReadRecord`) to engineering's surface.
   Complete, but a new verb on a protected surface (kernel allowlist,
   surface docs, parity tests, authority question: which records may a desk
   read). Larger than the defect.
2. Raise the objective cap to the body's 16 KiB, and make the marker
   honest for anything beyond: it must say the rest is not readable from
   the desk and name the working route (choir.Ask to management, which
   holds the Cast in its inbox, for the omitted part). Small, no new
   authority.
3. Have Texture put long text in the body. Rejected as the fix: it relies on
   the model's choice, and the body is cut at 16 KiB as well.

Chosen: option 2. A new read verb remains a possible later step if requests
over 16 KiB prove common.

## Belief state and remaining error field

- Fixed by option 2: any request whose objective or body is at most 16 KiB
  reaches engineering whole (the observed 6,338 B case).
- Not fixed: a request over 16 KiB is still cut; the marker will say so and
  name the route, but engineering depends on management resending it.
- Not investigated here: whether Texture should cap its Cast objective.

Heresy delta: discovered 1 (a marker that promises an unreadable record),
introduced 0, repaired 1 (objective bound raised to 16 KiB; the marker now says the rest is not readable and names choir.Ask to management). Rollback: revert the fix commit (display-only change; the tape and digests are untouched).
