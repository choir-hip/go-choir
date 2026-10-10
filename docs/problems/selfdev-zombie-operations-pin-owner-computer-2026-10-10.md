# Zombie self-development operations pin the owner's computer busy (2026-10-10)

**Status:** open. Documented before any fix (problem documentation first).
**Mutation class of this record:** green. The fixes are `orange` with a
`red` neighbor (the self-development operation state machine); see Fix shape.
**Owner input (2026-10-10):** "owners computer shouldnt show 8 active
self-dev operations. we havent even started working with the new desk
prompts yet, so all of them are definitely gonna fail. and we shouldnt be
trying that much concurrency yet, we dont yet have proof that we can do 1
correctly."

## Evidence

The owner's computer (`computer-03335285269bdba4f94377e56879f9e6`) reports
`selfdev_active_operations: 8`, `running_runs: 0`,
`desk_pending_mutations: 1` on its guest `/health`. Its self-development
mode is `propose_only`, generation 1. Only states and timings were read.
No operation content was read.

Five of the eight are identified by id from earlier receipts. Each was
read with the guest's GET-by-id route. All five are `executing`. Each was
touched once, within a second of its creation, and never again:

| operation | created | last updated |
| --- | --- | --- |
| `selfdev-b090bcd72d300fed17cb3f5a142f8595` | 2026-08-16 20:07:13Z | 20:07:13Z |
| `selfdev-3f842968bd296fcc85400d53ce1023a6` | 2026-08-19 12:28:22Z | 12:28:23Z |
| `selfdev-ccf0f1ec0e851750f253fe5f5ed97974` | 2026-08-19 17:45:03Z | 17:45:05Z |
| `selfdev-d21a7fcb5ad2c4826f6397cd860a8eac` | 2026-08-20 18:41:32Z | 18:41:32Z |
| `selfdev-8dcdd2c5e7841addb24b0c7991f09a5c` | 2026-08-20 21:23:01Z | 21:23:02Z |

The other three are not identified. The guest has no list route for
operations. Its database sits on a running guest's block device, which is
off limits.

The owner chain's last thousand events contain no self-development event,
so today's work (19834f68, rerun 11) did not create these.

## Cause

1. **Nothing closes an orphaned pre-decision operation at boot.** Desk
   tools drive every pre-decision transition: requested to executing
   (`freeze_candidate.go`, the start API), executing to frozen and frozen
   to verified (`tools_capsule.go`), and verified to awaiting_approval
   (`tools_capsule.go`, `engineering_desk.go`). A crash boot closes the
   desk run that drives those transitions as `interrupted_by_restart`
   (`actorruntime/handler.go`), but leaves the operation alone. The only
   closer, `failBoundSelfdevOperation`, runs when a bound run is cancelled
   or when reconstruction revisits that exact trajectory. Trajectories from
   August are never revisited. This breaks the owner rule "Restarts End
   Work (Crash) Or Resume It": every open obligation closes at a crash
   boot.
2. **The busy probe counts them.** `activeSelfdevOperations`
   (`runtime.go`) counts executing, accepted, materializing and
   rollback_pending. vmctl's `guestBusy` (`vmctl/ownership.go`) treats any
   count above zero as busy. The code comments say parked states are
   excluded "so a wedged op cannot pin the guest busy forever". An executing
   operation orphaned by a crash pins it anyway.
3. **Busy computers never update.** Deploys leave active computers alone
   (`ci.yml`, docs/problems/deploy-restarts-busy-computers-2026-10-09.md).
   A computer picks up new code at its next idle stop or wake. A computer
   that is never idle never boots onto code that could close its zombies.
   This deadlock is the "known weak spot" in that doc, reached through
   self-development operations rather than runs.
4. **No admission limit.** `selfdev.Store.Start` admits a new operation
   while others are open. Nothing stopped five attempts in five days from
   piling up.

Related: docs/problems/s0-selfdev-executing-wedge-2026-10-04.md. That doc
covers an operation that stays in executing after its model turn ends,
while the guest is still up. This doc covers the restart case. The fixes
below close both cases at the next crash boot. They do not close the live
wedge on a computer that never restarts. That case still needs the desk
run's terminal fate to close its bound operation.

## Fix shape

1. **Crash-boot closer.** A crash boot (no planned-restart marker)
   transitions every operation in requested, executing, frozen or verified
   to `failed`, with the terminal error "interrupted by a restart". It
   leaves alone:
   - awaiting_approval, because it waits on the owner's decision and is
     durable;
   - accepted, materializing, rollback_pending and degraded, because the
     materializer recovers them from the updater journal.
   A planned boot leaves every state alone.
2. **One open operation per computer.** `selfdev.Store.Start` refuses a
   new operation while any operation on the same computer is in a
   non-terminal state. Applied counts as settled. The idempotent retry of
   the same request still returns its existing operation. The start API
   answers 409, naming the open operation.
3. **One boot of the owner's computer** after (1) lands, so the closer
   runs there. The owner's statement that these operations "are definitely
   gonna fail" is the direction to close them. The boot is a refresh with a
   named lifecycle caller, taken while `running_runs` is 0.

Rollback: revert the fix commit. Already-failed rows stay failed. That is
correct, because their driving runs are gone.

## Separate open question: where the mode lives

The self-development mode is stored host-side in corpusd
(`computer_self_development_modes`, CAS, corpusd-signed ModeReceipt), not
on the computer's own chain. No doctrine records that choice. The owner
asks whether it belongs in the computer. The probable answer is an
owner-signed fact on the computer's chain. Protection against self-arming
then lives in the signer (only the owner's key arms), not in host
custody. This stays open until it is designed. The fixes above do not
depend on it.
