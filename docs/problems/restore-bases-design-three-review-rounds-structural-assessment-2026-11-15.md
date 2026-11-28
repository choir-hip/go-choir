# Restore bases design: three review rounds without convergence

Date: 2026-11-15T06:52Z. Subject:
docs/archive/design/restore-bases-store-once-2026-11-15.md. Rule applied: three
rounds without convergence, stop patching and change the approach.

## What happened

Round 1 (two seats, REPAIR): page deltas were ranked first on numbers nobody
had measured. Round 2 (REPAIR): the revision introduced the claim that the
paper is replay-ineligible and a five-landing plan. Round 3 (both seats
REPAIR, independently the same findings):

- A checkpoint cannot be bound at a capture's head. The platform accepts a
  checkpoint only at the computer's current head
  (internal/platform/checkpoints.go about 59). Publishing at the copy's head
  means holding appends through copy, witness and publish: 25 to 50 minutes
  on the paper, longer than today. Landing 3's premise is false.
- The import that moves agent_evidence and co_super_slots into the graph
  deletes the table rows by direct SQL (internal/store/graph_home.go about
  208 and 493). The delete is not on the tape, so a replay from any base
  that holds the rows fails the witness against a checkpoint bound after the
  import. Landing 1 as written makes every existing base unusable.
- The import does not run at boot today, refuses rows it cannot represent
  and stops there; the design said nothing is lost.
- A fixed scratch path left by a crash wedges the snapshot producer, because
  the free-space check runs before any cleanup.
- Deleting the cadence allowlist leaves the cadence off (unset means off)
  and cold-boots every computer once.

Landing 2 (copy the pages inside the read transaction, release it, witness
the copy) was found correct by both seats in rounds 2 and 3, with the
scratch cleanup and free-space rule to settle.

## Why three rounds did not converge

Each revision answered findings by adding mechanisms, and each new mechanism
was written from a summary of the code, not from the code. The reviewers
read the code and found the new mechanism wrong. This is the same fault as
the P1 player rounds earlier tonight: scope added inside a repair round is
unreviewed design. One document carrying five red mechanisms cannot settle,
because any one of them reopens it.

## Correction to an earlier record

docs/problems/the-paper-is-not-restorable-by-any-path-2026-11-15.md says the
paper's 150 agent_evidence rows make it unrestorable. That is too strong.
The eligibility result has one reader, the replay-completeness report
(internal/agentcore/replay_completeness.go about 459); nothing refuses a
restore on it. What stops a restore of the paper is that no checkpoint is
bound for it (the cadence is off) and the tail takes over an hour. The rows
matter only if they changed after the base a restore starts from, because
then the witness cannot match. Whether they did is not measured.

## Change of approach

The package design is set aside. It stays in the tree as the record of the
options and their costs; nothing is built from its landing list.

1. Each mechanism is its own lane. The lane's builder reads the code and
   writes a short design section and failure list first; two seats review
   that before any code. No mechanism is added in a repair round.
2. Built now: capture copies pages inside the read transaction, releases it,
   and witnesses the static copy. It shortens the WAL pin from about 37
   minutes to the copy time. It is the one mechanism three reviews agree is
   correct. It does not need the others.
3. Investigated next, read only: whether the checkpoint authority can accept
   a checkpoint at a head behind the current one, with proof that it is on
   the chain. Without that, a checkpoint on a busy computer costs either
   repeated failed scans or a held append for the whole witness. This is the
   substrate question under "checkpoints on every computer"; the cadence
   stays as it is until it is answered.
4. After that: retiring the two direct-SQL tables as an act on the tape (one
   op meaning "these tables are empty from here"), so a replay from any base
   reaches the same state.
5. The base cadence by restore budget waits for the measurement (C97) and
   for item 3.

The owner said on 2026-11-15 that restore of the paper is not urgent and
its data is not precious yet. That is why item 2 goes first: it saves disk
and WAL now, and the rest is ordered by what it unblocks, not by urgency.

## Remaining error

Item 3 may have no acceptable answer; then the choice between a long hold
and no checkpoints on busy computers is the owner's. The paper's row
question above is open.
