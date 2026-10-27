# The backfill bound landed without its security trailers, so CI refuses main (2026-10-27)

Status: open; documented before the fix. Class of the fix: red (the guest
security epoch).

## Evidence

- FP0 landed 4a995bb44 and 192759f04 (the related-review act's 90 s
  bound, review 356 approve) and c114ec827 on main at about 16:50. CI run
  40544992228 failed: `guest-security-epoch: FAIL 4a995bb449e3 … touches
  internal/agentcore/story_entities_backfill.go`, missing both
  `Security-Effect` trailers; the Go job failed on the same gate
  (`v1-inventory was selected but result was failure`). Staging stays on
  c0274ecdd, and every later push fails the same way until the range is
  covered, since CI's base is the live staging commit.
- `internal/agentcore/` is a surface (`nix/guest-security-surfaces.txt:32`).
  FP0 knew `internal/sourceproj` and `internal/platform` were not surfaces
  and did not check agentcore.

## Why

- The CI mirror on the agents VM runs three repo gates but not
  `scripts/check-guest-security-epoch`, so the cheap checks passed.
- The gate's rule: trailers cannot be added to a pushed commit; after
  landing, a follow-up commit that raises `security_epoch` covers the
  range.

## Fix

1. Raise `security_epoch` 1 → 2 (`nix/guest-release-policy.json`), the
   gate's own remedy. Effect: every hibernated computer's release-bound
   snapshot cold boots onto the current release at its next wake instead
   of resuming (design D2 §3, refusal R4); nothing runs in a fenced
   snapshot, so no open work is lost. The epoch never falls. Not chosen:
   rewriting main's history to add the trailers.
2. The CI mirror runs `check-guest-security-epoch --base <merge-base>`.
