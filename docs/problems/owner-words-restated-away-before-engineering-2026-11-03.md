# The owner's words are restated away before they reach the engineer (2026-11-03)

Status: open. Mutation class of the fix: orange (what engineering is told;
the Cast authority chain is unchanged).

Cluster 1 of docs/evidence/selfdev-trace-review-synthesis-2026-11-03.md.

## Evidence

- Pool r2 (docs/evidence/selfdev-trace-review-pool-arm-2026-11-03.md, finding
  3): management's Cast to engineering was one sentence and a parent id. The
  engineer froze after 9 minutes with twelve identical `body` shots.
- Pool r4 (same finding): management indexed `OwnerDirections[4]`, which held
  round 3's text, pasted it as "round 4 verbatim" (objective 6,184 chars) and
  Noted Texture that round 4 was carried. Round 4's step order, its 15 and 25
  minute budgets and the contrast report pointer never reached the engineer.
  The packet held six revisions, oldest first, so the newest was at index 5.
- Deepseek restore round (docs/evidence/selfdev-trace-review-deepseek-arm-2026-11-03.md,
  finding 5): Texture's Cast named no `owner_revision` and reframed the
  owner's restore as "retry the publication"; management's Cast said "publish
  the combined stack". The owner's scope, verify commands and 20-minute limit
  were in neither.
- Claude arm (docs/evidence/selfdev-trace-review-claude-arm-2026-11-03.md,
  finding 3): "start the Claude delegate for the edits and shots" became
  "report the delegate's call count".

## Cause

- The engineer's prompt is management's objective; its run context names the
  assignment and the capsule recipe and nothing of the owner's. The owner's
  words reach engineering only inside the Texture request in `choir.Records()`
  (`engineeringRequestView`), and only when Texture named the revision.
- `owner_directions` is a JSON list on the Texture Cast's packet that the
  model reads; the runtime builds it oldest first
  (`attachCastOwnerHistory`), with the named revisions appended after the
  earlier ones in the order Texture named them, so the list is chronological
  only when Texture names one id. No entry says which is current. The
  pool r4 index (4 for "round 4") is a model reading a round number as a list
  index (hypothesis from the packet shape; the code wrote the six entries
  oldest first, which put round 4 at index 5).

## Chosen fix

1. Every `owner_directions` entry keeps its revision id and time, the list
   is sorted oldest first by time whatever order Texture named the ids in,
   and the newest entry carries `"current": true`.
2. The platform appends the latest owner revision the Cast chain answers
   (the newest owner revision on the Texture Cast's packet, or on the
   document's trajectory at or before that Cast) to the engineering run
   context, verbatim, under "Owner's words (verbatim; the operative
   request)", cut at 12 KiB with a visible marker. It is derived from the tape
   at every read, so no desk can edit it. Management's objective stays.
