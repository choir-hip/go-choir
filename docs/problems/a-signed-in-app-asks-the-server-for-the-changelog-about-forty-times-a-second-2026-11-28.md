# A signed-in app asks the server for the changelog about forty times a second

Standing: governing | kind: happened | settled-by: the console of a signed-in plain launch of lane proof-signin 4933144a6 (shared code as on origin/main bb10f2d21) in a slim simulator against staging, 2026-11-28 | note: not measured on a phone; whether the owner's phone carries a build with this defect is not known

## Problem

From launch, with the Activity page not open, the app asks its Activity source
again on every frame. Signed in, each ask is a request to the server:
`GET /api/changelog?limit=50&minor=1`. Signed out, no request is sent, but the
source is still asked every frame.

Two more things follow from it:

- Each new ask lets the last one go. Work left over from an answer that was
  let go reads the account key outside any answer, where Exact's secret store
  refuses the read. Shared code turns that refusal into "no key". So the app's
  own log flips between signed in and signed out about once a second.
- A page that reads the key at such a moment can show "Not signed in" for a
  moment while the listener is signed in.

## Evidence

- 6,643 changelog requests in 159 seconds, about 42 a second; between 7 and 65
  in any one second. Exact's trace also shows 8,492 Activity answers let go.
- `api: session found=false` and `found=true` alternate 313 times in the same
  log, 2 to 3 ms apart.
- The cause in the code: `members-settings.contract:28` passes
  `stime.epochAtZero + performanceNow()` after `with`. That value differs on
  every frame, and Exact asks a source again when the value after `with`
  changes. The line came in with `43c3db878` on 2026-11-27 (Activity's clock
  became Exact's date).
- Exact's secret store is readable only while an answer runs (`door.rs:45-53`
  in the Exact checkout); `api.ts` `storeSource` answers `found: false` for a
  session read that throws.

## Why nothing caught it

The real-data proof ran signed out from 2026-11-23 until today
([record](the-real-data-proof-cannot-sign-in-since-the-key-moved-to-shared-code-2026-11-28.md)),
so no run made a signed-in request. No test counts how often a source is
asked while only the clock moves. The review of `43c3db878` did not ask what
re-asks the source.

## Belief

The request storm is the `with` value and nothing else. The flipping key read
is its side effect and stops when the storm stops, but the weakness under it
stays: a key read that throws is reported as "no key" for a session read,
where the rule is "unreadable, never signed out".

## Remaining error

- Not known: the load this put on staging, and whether any phone ran such a
  build signed in. The test account's run today is the only one measured.
- Not known: whether a false "no key" can reach the account wipe. The reading
  so far says a refused read becomes "unreadable" on that path, which is safe.
  It has not been proved.
- Not checked: other sources fed a clock the same way.

## Fixed

`954102fc7` (2026-11-28): the Activity source no longer reads the clock and is
asked only while its page is on top; ages are made where the row is drawn from
a page clock that steps every 30 seconds. A signed-in proof run on that code
shows 0 changelog requests in the plain launch (was 6,643) and 3 key-read
flips (was 313). `be8b0dc6e` then made a refused key read "could not read"
and never "no key". Not fixed: the sign-in sheet still opens on a key that
could not be read
([record](the-sign-in-sheet-is-offered-while-the-key-cannot-be-read-2026-11-28.md)).
Not measured on a phone.
