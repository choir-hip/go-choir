# Ordinary use of the stock apps makes a computer checkpoint-ineligible, so it cannot self-develop

Status: open. Recorded 2026-10-24 by FP0. Mutation class of this record:
green. Class of a fix: red (checkpoint and replay projection, new tape event
kinds).

## What happened

At 11:1x UTC the app-rewrite conductor froze a five-app candidate on b@c's
computer (computer-6c50d6e566143ae32cdb660faa60e24d, op selfdev-79dc4bf4).
The required pre-apply checkpoint (`choir computer checkpoint`) was refused
with 409 "replay is ineligible: behavior-bearing direct-write tables are
non-empty without reducers". The CLI does not name the table.

`GET /api/computers/<id>/self-development/replay-completeness` (b@c's key,
11:2x) names it: `eligibility.unsupported_tables = ["media_recents"]`.

## Why, from the code

- The replay manifest (`internal/agentcore/replay_eligibility.go`) classes
  `media_recents`, `media_progress` and `user_preferences` as
  `ReplayEmptyUntilSupported`: allowed only while empty, because no reducer
  rebuilds them from the tape.
- The stock frontend writes all three in ordinary use:
  - `frontend/src/lib/media-utils.js` POSTs `/api/media/recents` when a media
    source is opened and `/api/media/progress` as playback or reading moves
    (handlers in `internal/mediastate/handler.go`, store in
    `internal/store/media.go`);
  - `frontend/src/lib/preferences.js` PUTs `/api/preferences/theme` when the
    owner changes the theme (`user_preferences`).
- So one opened PDF, one played podcast or one theme change makes the
  computer ineligible for every checkpoint, restore and effects transition
  that needs a replay claim, until the tables are emptied. The pre-apply
  checkpoint is required before an owner approval, so self-development on
  that computer is blocked.
- The self-development drills passed because their computers are minted
  fresh and never touch these routes.

## Who it reaches

Every computer whose owner has used Settings' theme or any media surface.
That is likely most real computers, the owner's own included (not probed:
the replay-completeness report is not run on the paper or the owner's
computers). b@c is the first observed case.

## Options (not taken here)

1. Make the three tables event projections: the handlers append computer
   events and a reducer projects them, as the desktop_* tables already do
   (`ReplayEventProjection`). The replay claim then covers them.
2. Reclass them as non-behavior-bearing: excluded from the equivalence
   check, left as they are on restore. Smaller, but it weakens the
   reversibility claim (a restore would not rewind recents, progress or the
   theme) and needs a new manifest class.
3. Name the table in the CLI's 409 (cheap, any time): the refusal should say
   which tables block it.

FP0's lean: 1 for all three tables (they are small, per-owner, last-write-wins
records, the same shape as window placements), with 3 alongside.

## The b@c candidate

The candidate stays at awaiting_approval. Without a fresh checkpoint its only
rollback point is checkpoint 0a310d91 (04:26), from before the Slides apply,
and the rollback route requires a prior checkpoint. The conservative choice
is to hold it until the checkpoint can be taken.

## Evidence

- /tmp/fp0-bc-replay.json (local, FP0's Mac): the replay-completeness report.
- The conductor's log /Users/wiz/Dev/lanes/_runs/apprewrite/log.md, v6 section.
