# Changelog app (lane E): failure list

Written before the code (autopaper contracts section 5; handoff 4.E). Pass 1.

| # | Failure | Answer |
|---|---|---|
| E1 | The read is unbounded: a long changelog inbox is loaded whole | `GET /api/changelog` pages newest first by an inbox-sequence cursor (`before`), limit clamped to 200, and the scan per request is bounded |
| E2 | Minor events leak into the default view | the default is major only; `minor=1` is the zoom; level always from the kind |
| E3 | Unknown kind breaks the page or is shown as major | tolerated at read, shown as minor only on zoom (existing `ChangelogLevelOf`) |
| E4 | Seen cursor lives in the browser | stored on the owner's computer (user preference per computer); a second browser sees the same cursor |
| E5 | Seen cursor moves backwards or past the head (future events appear seen) | the write is monotonic and refuses a value above the inbox head |
| E6 | Unseen count is unbounded | counts major events past the cursor, capped at 100 with a `more` flag |
| E7 | Another owner's events are read | owner comes from the authenticated user only; no owner parameter |
| E8 | Verdict reason (Jev output) read as instruction or rendered as HTML | rendered as text, labelled untrusted; never given to a desk |
| E9 | An event with no document shows a dead link | link only when a doc id or a route path exists; otherwise plain text |
| E10 | The app wakes a desk or writes to the tape by reading | read-only route; the only write is the seen preference |
| E11 | Pop-up spams on every poll | pop-up shows once per new head above the cursor; dismissing or opening the app marks seen |
| E12 | Unreadable on a phone | single column, wrapping text, 44px tap targets |
