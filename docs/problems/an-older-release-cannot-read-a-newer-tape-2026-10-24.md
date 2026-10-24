# An older release cannot read a newer tape (2026-10-24)

Status: problem documented; the invariant and its gate are owner-ratified
(2026-10-24, in session); no code yet. Station SR7 of
`docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.md`.

## The problem

Restore does not rewrite the tape. It moves a computer's effective head back
and runs the release that matches that earlier point; every event written
after it stays on the tape, because the tape is append-only. So after a
restore, an older binary boots on a tape that holds newer events. If one of
them is a kind or field that binary does not know, the computer may not boot,
or may project wrongly.

Receipt: b@c's five-app candidate selfdev-79dc4bf4 was rejected at 13:2x
(first-pass ledger). It was built from 0cbc52edc, and b@c's tape already held
the D39 projection op kinds (media_progress_recorded, media_recent_recorded,
user_preference_recorded), which that binary cannot read. Approving it would
have left the computer one restore away from unbootable. The restore point was
fine; the binary could not read the tape.

## The invariant (owner, 2026-10-24)

Any release can boot on any tape written by releases inside its support
window, in both directions:

- **Newer reads older.** Mostly holds today (decode roots, migrations).
- **Older reads newer.** Missing. A binary that meets an event kind or field it
  does not know keeps it intact and skips it in projection: it never fails
  boot and never drops the event. Restoring to an older release is then always
  safe; the newer events sit inert until a release that understands them runs.

Two rules follow:

1. A new event kind ships in two releases: first one that keeps and skips it,
   later one that writes it (the same shape as the update rulings' one-week
   support window and floor).
2. Store schema changes are additive only; a destructive change is two-phase
   (stop reading, then remove after the window).

## Mechanical enforcement

A cross-version replay gate in CI: boot the previous release's binary on a tape
the candidate wrote, and require it to boot and project, with the unknown
events skipped and preserved. It would have caught selfdev-79dc4bf4. Passing it
routinely is what makes a change class "proved reversible", the gate the staged
auto-approve path needs (SR8).

**Constraint (owner, 2026-10-24): the gate must not lengthen CI wall clock.**
It runs as its own parallel job, or inside an existing job's slack, and must
finish inside the current critical path. The previous release's binary comes
from a cache (the deployed build, or a pinned artifact), never a second full
build on the critical path. The CI timing baseline is read before the design.

## Open questions (hypotheses until read)

- What an old binary does today on an unknown kind. Hypothesis: the decode
  roots refuse it and boot fails or the projection stops; to be read from the
  decoder and replay code, not assumed.
- Which tapes the gate replays: a fixture tape written by the candidate's own
  tests, or a sample of a staging test computer's tape.
- Fields added to a known kind (a newer field on an old kind) versus whole new
  kinds; both directions need a rule.
- The Dolt projections: which tables an older binary reads that a newer one
  migrated.

## Next

1. Read the decoder and replay path for unknown kinds and fields (read-only).
2. Read the CI critical path timing (which jobs bound wall clock).
3. Design the keep-and-skip decoder and the gate; review it like the change it
   designs (tier 2: tape, boot path).
4. Build, prove on staging with a real restore across a kind boundary, then
   the doctrine entry.
