# The paper is not restorable by any path

Date: 2026-11-15. Found by the designer of
docs/archive/design/restore-bases-store-once-2026-11-15.md and both review seats of
its second round; read from code and from the paper's console log. Not
observed by attempting a restore (nothing acts on the paper).

## Problem

Until tonight the paper's restore problem was thought to be time: 315,000
tail events, about 68 minutes of replay, in a ten-minute request. It is
wider than that.

## Evidence

- The paper holds 150 `agent_evidence` rows. The eligibility check marks a
  computer replay-ineligible when `agent_evidence` or `co_super_slots` is
  non-empty (`internal/agentcore/replay_eligibility.go`, about lines 69, 90,
  187 to 198, 270). Those tables are written by direct SQL and are not on
  the tape.
- The checkpoint cadence compiles off and runs only for d@e and `class:test`
  (`internal/agentcore/checkpoint_cadence.go` about 27 to 32;
  `nix/node-b.nix` about 1704). The paper's console log from 01:29Z to
  06:38Z holds no cadence line.
- There is no fallback to replay from genesis: a tail over 720,000 events is
  refused (`internal/recoveryplan/recovery_plan.go` about 22 and 131) and the
  cold path answers `base_missing`. The paper's tape is at 11.8 million.
- The compaction copy carries those two tables from the later original
  (`internal/store/compact.go` about 720), so a base and a checkpoint can
  disagree on them.

## Belief

A restore of the paper can pass only to a checkpoint bound by hand, from a
base whose `agent_evidence` rows match it. In practice the paper has no
restore. The owner said on 2026-11-15 that Autopaper data is not precious
yet and restore is not urgent; this record exists so that statement is made
with the fact known.

## Remaining error

Whether a hand-bound checkpoint would pass is not tested. The repair is the
first landing of the design: both tables move into the graph through the
tape and are emptied everywhere, then checkpoints bind on every computer.

## Correction, 2026-11-15, after design review round 3

The belief above is too strong. Eligibility is read only by the
replay-completeness report; no restore is refused on it. The paper has no
bound checkpoint and a tail over an hour, and that is what stops a restore.
The 150 rows matter only if they changed after the base a restore starts
from. See
docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md.
