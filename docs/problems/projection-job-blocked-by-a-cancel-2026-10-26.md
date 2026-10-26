# A cancelled projection job is recorded as a deterministic refusal and never retried

Found 2026-10-26 14:2x by FP0 while reading b@c's projection job before
booting it (X1 design, section 4.7 and residual R3). Documented before any fix.

## Evidence

checkpointd's journal on Node B, latest job per computer over 14 days:

- b@c (computer-6c50d6e5…): generation 14, status `blocked`, reason
  cadence, target 35,858, watermark 33,305, failures 1, updated
  2026-10-25 08:15:01Z, error "projection base refused: read-back witness:
  projection base: extract content witness: context canceled". No later
  generation in 30 hours, while b@c's tape grew to about 57k events.
- d@e (computer-59c902c5…): succeeded, generation 2, 2026-10-26 13:20.
- computer-71afbf14…: succeeded, generation 6, 2026-10-23 22:32.

Generations 1 to 13 for b@c all succeeded on cadence, about every 2,500
events.

## Mechanism

`cmd/checkpointd/worker.go:226-232`: when `rebuilder.Run` fails, the status
is `blocked` if the error is `projectionbase.ErrBaseRefused`, else `failed`.
The read-back witness step wraps a cancelled context inside
`ErrBaseRefused`, so a cancel (the unit stopping during a deploy, or the
job's deadline) is recorded as a deterministic refusal.

`internal/platform/projection_jobs.go:204`: admission returns the old job
unchanged while it is `blocked` with the same seed, key and repair inputs.
Head growth does not re-admit it. A `failed` job is re-admitted after a
backoff (`failedJobRetryBackoff`); a `blocked` one never is, until the seed
or key changes or an operator runs a genesis repair.

So one cancelled run froze b@c's projection base at event 33,305.

## Consequences

- The apply checkpoint replay that filled b@c's /tmp and crash-looped it
  (docs/problems/apply-checkpoint-replay-fills-guest-tmp-2026-10-26.md)
  replayed from this stale base, about 24k events instead of at most 2,500.
  This explains R3 in docs/design/apply-checkpoint-without-replay-2026-10-26.md.
- Any restore or rebuild of b@c starts from a base about 24k events behind
  its head.
- Nothing surfaces a blocked job; it was found by reading the journal.

## Belief and remaining error

- Believed: the cancel was transient (a deploy restarting checkpointd mid-run
  is the leading hypothesis; the deploy times on 10-25 near 08:15 were not
  checked).
- The fix should classify a cancelled or deadline-exceeded context as
  `failed` (retried after backoff), never `blocked`, wherever it is wrapped,
  and re-admit an existing `blocked` job whose recorded error is a cancel.
  Test first.
- Open: whether other wrapped transient errors (store I/O, lease loss) also
  land in `blocked`; whether the paper or owner computers log jobs to the same
  journal (only three computers appear in 14 days).

## Audit of `blocked` sites (2026-10-26, lane/projection-cancel)

The wrap that lost the cause: `internal/projectionbase/verify.go:97`
(`"%w: read-back witness: %v"`) formats the witness error with `%v`, so
`errors.Is(err, context.Canceled)` is false even though the text says
"context canceled". The lane's first test reproduces the production error
string exactly.

`fail("blocked", ...)` sites in `cmd/checkpointd/worker.go`:

| Site | Verdict |
| --- | --- |
| missing base beyond bootstrap bound | deterministic (needs genesis repair) |
| `GetKeyEscrow` error | **mixed**: `ErrKeyEscrowNotFound` is deterministic; any other error is store I/O or a cancelled context (transient) |
| escrow record JSON invalid | deterministic |
| escrow digest changed since admission | deterministic (admission re-admits on a key change) |
| `OpenDEK` unwrap refused | deterministic (local crypto, no I/O) |
| `NewRebuilder` config refused | deterministic |
| `rebuilder.Run` returns `ErrBaseRefused` | **mixed**, see below |
| result does not match frozen target | deterministic |

`ErrBaseRefused` wrap sites on the worker's `Run` / `VerifyPublishedBase` path:

- Carry a ctx- or I/O-bearing cause through `%v` (transient-capable):
  `verify.go:97` read-back witness, `seed.go:220` seed witness,
  `rebuilder.go:536` tail-start read, `resolve.go:47` tail read, and
  `rebuilder.go:498` (wraps the last rejected seed candidate's error).
- Local filesystem sites (`rebuilder.go:226/231/242/265/489`, `seed.go:204-248`,
  scratch open/head): can fail on ENOSPC or a transient I/O error but equally on
  a corrupt scratch; the cause is not distinguishable by type. Left `blocked`
  as a named residual; the worker's `ctx.Err()` backstop still demotes any of
  them when the job context itself was cancelled.
- Validation sites (descriptor, digest, witness mismatch, ancestry, reducer or
  schema version, bounds): deterministic.
- `source_http.go`, `install.go`, `advertise.go`, `rebase.go`: not on the
  checkpoint worker's path (restore and advertise callers); not changed.
