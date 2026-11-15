# Restore as an accepted act: three review rounds without convergence

Date: 2026-11-15T08:29Z. Subject: lane/restore-act (tips 50aaf8152, d9abf7a74,
06b742853). Rule applied: three rounds without convergence, stop patching
and change the approach.

## What happened

Each round both seats said REPAIR, closed every earlier finding, and found
new ones in the same place.

- Round 1: the install record was not durable; a completed flip could be
  undone; a corrupt record was skipped.
- Round 2: renames were not synced before the completion fact; the undo
  guessed from file existence; the in-process undo deleted staging before
  the outcome was recorded; a failed undo was closed as terminal.
- Round 3, both seats, reproduced by both: a panic after the flip completed
  runs the undo's release step over the restored store and records a clean
  failure on the pre-restore state; nothing repairs it. A failed undo is
  followed by a reopen that can create an empty store at the original path.

Not in dispute after three rounds: the act itself (202 with an act id, the
status route, the record, one act at a time, the CLI), the durable record
writer, the refusal beside live work, and that boot's undo and
bookkeeping-only roll forward are within the restart rule.

## Why it does not converge

Every finding is in one mechanism: swapping a running process's store and
release in place. That is ten renames, a release pointer, a close and a
reopen, inside a process that can fail or panic between any two of them,
with a second actor (boot) that must reach the same verdict from the record
alone. Each repair narrowed one path and the next review found the next
path. The state machine is too wide to settle by review, and a wrong end
state here is silent.

## Change of approach

Do not swap in a running process. The act replays into a staged store, as
now, then records a durable intent and asks for a planned restart. The swap
happens at boot, before the store is opened, in the step that already
installs a staged compact copy under a planned-restart marker
(internal/autoputer/run.go, the rebase before the store open). One swap
mechanism then does both jobs.

What this deletes: the in-process flip, the in-process undo, the reopen and
rebind after a flip, the deferred failure path, the panic cases, and the
"restored; restart required" state as a special case (every restore ends
that way). What a crash can then leave: an intent not yet installed (boot
installs it or, with no marker, drops it and closes the act as interrupted)
or an installed store. The contract's restart rule already describes this:
a planned restart may resume once, with a marker.

What it costs: a restore always includes a restart of the computer. Today a
successful restore already replaces the store under the running process, so
no session survives it in a meaningful way; this is stated, not measured.

## Next

The lane's builder writes a design section and failure list for the
boot-time install, from the code, keeping the accepted act surface. Two
seats review that before any code. The round-3 findings are not patched.
The existing synchronous restore stays as it is on main until then; the
owner said on 2026-11-15 that restore is not urgent.

## Remaining error

Whether the boot rebase step can carry a restore's release change and
quarantine without growing its own wide state machine is not yet read from
the code. If the design review says it cannot, this goes to the owner.
The quarantine directory each restore leaves (store-sized, unpruned,
outside the disk floor) must be settled in that design.
