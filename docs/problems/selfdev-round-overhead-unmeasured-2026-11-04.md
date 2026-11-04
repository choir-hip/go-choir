# Self-development round overhead is unmeasured (2026-11-04)

Status: open. Mutation class of the fix: orange (a new ledger record at the
end of a round, a line in Texture's owner-act record, one wait-summary record
extended). This note is green.

Owner goal, 2026-11-04: "minimal overhead over the raw speed of writing,
building, testing code".

## Problem

How much of a self-development round is platform and how much is the engineer
writing, building and testing is estimated by reading logs. The estimates have
been wrong in both directions: the night of 2026-11-03 showed build jobs
waiting 6 to 22 minutes for host memory and a refresh hold of up to 20
minutes, and nobody could say what fraction of a round those were. The wait
kinds are now typed and tallied per assignment (751eb988a and the waits work),
but the tally covers only waiting. Spawn, cast and bind latency, the engineer's
exploration before the first build, build and choir-shot job time, and
Freeze-to-decision time are not recorded as phases anywhere, and no one reads
a round's total.

## Evidence that the pieces already exist

- Job end records in the engineer's inbox (`job:<id>:ended`) carry argv,
  `started_at` and `finished_at`.
- The assignment carries `CreatedAt`, the bind attestation's `RecordedAt` and
  `TerminalAt`.
- The self-development operation carries `created_at` (the Freeze) and the
  owner-act records carry the decision and apply times.
- The Texture Cast and management's Cast to engineering are commitment
  records with `committed_at`.
- The wait tally is in memory and is put on the ledger when the assignment
  ends (`engineering-waits:<assignment>:<attempt>`).
- Only the capsule spawn is timed nowhere but the guest log
  (`logAssignmentSagaPhase`).

## Fix design

Derive every phase from those records. Add one in-memory mark (the spawn
saga's length) and carry it on the existing wait-summary record, which is now
written for every ended assignment. At the round's end (apply, reject, fail,
abandon) compose one receipt record to management with the phases, the waits
by kind and the platform share, and add one line to Texture's owner-act
record.

Not adding: new event streams, new tables, a new timer.

Heresy delta: discovered (an unmeasured claim about overhead); none
introduced.
