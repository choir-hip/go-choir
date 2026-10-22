# The host's planned restart writes the marker without draining batched telemetry

Date: 2026-10-22. Status: **fixed in code (first candidate), proven
locally; staging proof owed** (lane `lane/fp1-planned-restart-drain`, see
"The fix" below). Originally documented, not fixed (found by FP0's phase 1
staging gate of run-telemetry batching, lane `lane/fp0-batching-e2e`). Class of
the eventual fix: red (tape write path, restart handling). Evidence class: code
reading on `840e1a701` (unchanged in `d991907e6`), plus vmctl's journal line
from this gate's kill drill on Node B, `05:25:46 vmmanager: destructive kill
vm=vm-c5c801f8… cause=refresh … caller=vmctl.(*OwnershipRegistry).refreshVMForDesktop`
then `signal: killed`: the same `RefreshVMForDesktop` the deploy refresh
calls is a SIGKILL. Not reproduced with a marker: see "Why the staging drill
cannot show it" below.

Design: [telemetry-batching-design-2026-10-20.md](../reports/telemetry-batching-design-2026-10-20.md)
section 4.5 and phase 1 failure mode P1-F14.

## What the design promised

Design 4.5, "Planned restart": the sealer goes into synchronous mode, the queue
is drained under the shutdown deadline, and only then is the marker written,
"so the one resume a planned restart allows resumes from a tape with no
telemetry gap". If the drain fails, the marker is still written and the resume
record says "the last window of this run's trace may be missing".

## What is built

The drain exists in one place only: `Runtime.markPlannedRestart`
(`internal/agentcore/planned_restart.go:311`), which calls
`drainTelemetryForRestart` before `WritePlannedRestartMarker`. Its callers are
the guest's own self-development apply (`self_development_materializer.go:970`)
and the guest-driven platform update (`platform_update.go:242`).

The planned restart every deploy uses does not go through it:

1. vmctl `deployRefreshWith` (`internal/vmctl/deploy_refresh.go:408`) asks the
   guest for a marker: `POST /internal/runtime/planned-restart`.
2. The guest's `HandleInternalPlannedRestart`
   (`internal/agentcore/planned_restart.go:450`) checks for busy work and calls
   `WritePlannedRestartMarker(storePath, marker)` at `:504` directly. It does
   not put the sealer in synchronous mode and does not drain.
3. vmctl then calls `RefreshVMForDesktop` (`deploy_refresh.go:432`), which
   reaches `vmmanager.RefreshVMWithConfig` and `killFirecrackerProcess`
   (`internal/vmmanager/manager.go:1609`). That is a SIGKILL of the VM, not a
   graceful stop, so the guest's shutdown hook (`internal/autoputer/run.go:543`,
   `DrainTelemetry`) never runs.

So on a deploy refresh the queued window (at most `W` plus one in-flight batch)
is lost exactly as on a crash, but the boot finds the marker and treats it as
planned: runs are passivated for their one resume
(`passivateInterruptedActivations`), and `passivatedOnRestartPayload`
(`telemetry_batching.go:120`) adds the "trace may be missing" sentence only
when `marker.TelemetryDrainFailed` is set, which this path never sets. The
gap is silent.

The self-development restore restart (`writeRestoreRestartMarker`,
`self_development_restore_restart.go:75`) also writes the marker without the
drain, but it then restarts the guest service, whose SIGTERM reaches the
shutdown hook, so the queue is normally drained there. It does not record a
failed drain on the marker either.

## Who is exposed

Only computers that take a planned refresh **while busy**:
`deployRefreshAllowsBusy` (`deploy_refresh.go:215`) is true for
`critical_protected` and `public_platform`, which includes the paper computer.
Every other class gets the marker only when no run is running
(`planned_restart.go:490`), and an idle computer's queue is empty within `W`
of its last telemetry record (the deadline seal, and every terminal run event
is an acknowledged write that barriers). So today, with the flag off
everywhere, nobody is exposed. Turning `telemetry.batch` on for the paper would
expose it on every deploy.

What is not at risk: sequence numbers. `configureTelemetryBatching` runs at
`Runtime.Start` before the first event, so the boot after the kill seeds
`stream_seq` and per-run `seq` with the 512 gap and writes the boot watermark;
no number is reissued. The loss also stays inside the ruled bound (owner notes
§27 item 18). What fails is the design's stronger claim that a planned restart
has no telemetry gap, and its rule that a gap is never silent.

## Why the staging drill cannot show it

The test computer of this gate is warmness class `test`. Its guest refuses the
marker whenever a run is running, so a marker with records queued is not
reachable on it; the only way to put a busy protected computer through a
deploy refresh is a deploy or a host action on a protected computer, both
outside this lane. Section 9 step 5 is therefore recorded as not run, and
this record stands on the code path above.

## Failure modes a fix must cover (write the tests first)

1. Host marker request with records queued: the queue is sealed before the
   marker is durable, and new telemetry takes the acknowledged path until the
   kill.
2. The drain times out or loses records: the marker is still written (the
   planned-restart contract is about work) and carries
   `telemetry_drain_failed`, so the passivation record names the gap.
3. The host's request is refused as busy after synchronous mode was entered:
   synchronous mode is undone, or the next deploy retry finds it on and the
   computer runs acknowledged until then (state which).
4. vmctl clears the marker after a failed refresh (`marker.clear`): synchronous
   mode must be undone too, or batching stays off until the next boot.

## Candidate fixes (the first is applied)

- In `HandleInternalPlannedRestart`, call the same `drainTelemetryForRestart`
  before `WritePlannedRestartMarker` and set `TelemetryDrainFailed` from it;
  undo synchronous mode on the `DELETE` (marker clear) path. Smallest change,
  one seam shared with `markPlannedRestart`.
- Or make vmctl's deploy refresh a graceful stop (SIGTERM, bounded wait) before
  the kill. Broader, touches vmctl and every refresh.

Until the fix below is on staging and proven there, do not turn
`telemetry.batch` on for a computer of class `critical_protected` or
`public_platform`.

## The fix (lane `lane/fp1-planned-restart-drain`)

The smallest candidate. `HandleInternalPlannedRestart` calls
`holdHostRestartTelemetry` (`internal/agentcore/telemetry_batching.go`), which
is `drainTelemetryForRestart` with its undo kept on the runtime, after the busy
check and before `WritePlannedRestartMarker`; the marker's
`TelemetryDrainFailed` comes from that drain and the 200 response echoes it.
`releaseHostRestartTelemetry` undoes synchronous mode on the `DELETE` path, on
a busy refusal, and when the marker write fails. The failure modes resolve as:

1. Queue sealed before the marker is durable; synchronous mode stays on until
   the kill, so later telemetry is acknowledged.
2. Drain failure or timeout: marker still written with
   `telemetry_drain_failed`, so `passivatedOnRestartPayload` names the gap.
3. A busy refusal never enters synchronous mode (the busy check comes first),
   and it **undoes** one an earlier host request left on: a refused computer
   is not restarting, so it goes back to batching. The retry enters it again.
4. `marker.clear` undoes it.

With batching off (every computer today) the hold returns before any call:
no sealer, no drain, no latency, marker and response bytes unchanged.

Proven locally (`go test -race ./internal/agentcore -run
'PlannedRestart|Telemetry|Marker'`, 23 pass), in
`internal/agentcore/planned_restart_telemetry_test.go`: one test per failure
mode 1-4 (each failed on `origin/main`) and the flag-off pin (passes either
side, by design).

Needs a staging proof (FP0): a `critical_protected` or `public_platform`
computer with `telemetry.batch` on, busy, taken through a deploy refresh; the
planned boot's passivation records must carry no gap (or the gap sentence if
the drain failed), and stream_seq must show no hole beyond the boot reseed.
The staging test computer cannot show it (see above).

Residuals: synchronous mode is one flag in the store, not a count. If a
guest self-development apply (`markPlannedRestart`) and a host request both
hold it, the first undo turns it off for both. The host refuses while an apply
is mid-transaction, so the overlap needs an apply to start after the host's
200 and before the kill. The cost is a window of queued, not lost-by-design,
telemetry. The restore restart (`writeRestoreRestartMarker`) still does not
record a failed drain (unchanged; its SIGTERM reaches the shutdown drain).
