# A failed self-development freeze leaves the operation "executing" forever, and the computer never updates (2026-10-22)

Found 04:05 UTC. The owner reported that their computer (computer-ad1b46d4…,
vm-4a9afa23, the Claude delegate's computer) had deferred every deploy
refresh tonight and was "not actually doing any work".

## Evidence

- vmctl's retry sweep, every minute since at least 02:54: `deploy refresh
  … outcome=deferred planned_restart=false busy_allowed=false reason="busy:
  … running_runs 0, selfdev_active_operations 1, live_capsules 0"`.
  This was not a capsule: the console shows every capsule torn down by
  10-21 11:14.
- `GET /api/computers/<id>/self-development/operations` (host-internal,
  read-only) lists one active operation:
  - `selfdev-20d9baa5eb36704d52009b6876611f98`, state `executing`;
  - created 2026-10-21T10:23:37.961Z, updated 10:23:39.739Z;
  - `last_intent_error`: "freeze: capsule release must contain executable
    bin/autoputer: the running binary was built from 219535ef… but the
    capsule source…".
- The computer booted at 10:20:18 on 10-21 (crash boot; it closed the
  earlier `selfdev-f6171703…` from executing). Firecracker has run since
  then: 17 h 40 min, RSS 3.9 GiB of host memory.

## Cause (read in code, to be confirmed by the fix's test)

The freeze step failed and recorded `last_intent_error`, but nothing moved
the operation to a terminal state (rejected, failed or closed). The active
set `{executing, accepted, materializing, rollback_pending}` keeps
counting it (`activeSelfdevOperations`, internal/agentcore). So:
- `/health` reports `selfdev_active_operations: 1`;
- vmctl defers every refresh indefinitely ("would not survive a restart");
- an owner decision cannot close it either. Approve and reject both
  require a frozen operation (`decideSelfDevelopment`,
  internal/agentcore/api_self_development.go:310-330), and this one never
  froze.

The only exit is the crash-boot close ("interrupted by a restart"), which
needs a markerless restart that nothing schedules.

## Harm

- The computer never takes a deploy. It has run a build from before
  tonight's deploys A, B and C.
- It holds 3.9 GiB on a full host, where admission refused test
  computers for 20 minutes tonight.
- The owner sees nothing: no fate, no notice.

## Repair

1. Now (owner-authorized, 2026-10-22: "we should get it updated and not
   stuck"): a plain stop and start of this computer through vmctl's
   lifecycle, with no planned-restart marker. Boot then closes the
   operation as "interrupted by a restart" (the designed crash fate), and
   the computer boots on the current deploy. Expected afterwards: no
   active selfdev operation, and the current build.
2. Code (red: self-development lifecycle). A freeze or intent error
   closes the operation with a visible terminal fate (failed: the error),
   in the same transaction that records the error. Tests first: a freeze
   failure leaves no active operation; vmctl's busy check then answers
   not busy.
3. Detector: an active self-development operation with no transition for
   more than N minutes appears on the obligations surface as stalled.
