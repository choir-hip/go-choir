# The operator cannot restart a computer that is already current (2026-11-10)

Class: orange/red (vmctl owner update, planned restart). Rollback: revert.

## Problem

The platform operator's restart verb (`POST /api/platform/operator/computers/<id>/restart`,
`choir paper restart`) asks vmctl for the owner update. The owner update first
reads the computer's served commit; when it equals the release the computer
would boot, vmctl answers `current` and does nothing (`ownerUpdate`,
`internal/vmctl/owner_update.go`). That is right for the owner's "Restart to
update" click. It leaves the operator with no act for a computer that is on the
right version but wedged, and the paper is that case: an always-busy
`public_platform` computer. Today that needs host access or a new deploy.

## Evidence

O5 proof on d@e (computer-59c902c5d46caecc0a180a8557164e72), 2026-11-10: the
operator restart returned `{"outcome":"current"}` with its receipt, and vmctl
marked and restarted nothing.

## Fix

The operator route asks with `restart_in_place` (new field on the deploy-refresh
request, with `owner_busy`; client `OperatorRestartContext`). vmctl then skips the
`current` shortcut and runs the same `trackedDeployRefresh` an update takes: the
ring gate, held and divergent rules, one refresh in flight, the guest's
planned-restart marker (`/internal/runtime/planned-restart?allow_busy=1`, written
before the restart, consumed once at boot, cleared when the restart fails) and the
guest's refusals for a self-development apply, a live capsule and a pending
decision. No marker code and no boot-time resumer is added. The answer is
`restarted` when the computer was on its release, `refreshed` when an update was
pending and this applied it.

Rate bound: a second operator restart of one computer within five minutes
(`restartInPlaceMinGap`) is refused `restart_rate_bound` (409, nothing asked of
the guest). A restart that failed counts; a refusal or deferral does not. In
memory in the deploy-refresh tracker, like the failed-target map beside it; a
vmctl restart forgets it. A correctness refusal, not a switch.

The owner's click does not send the field and is unchanged. The roster the gate
allows (`platform_computers`, `operator_proof_computers`) is unchanged.

## Not proven

The guest's behavior on a boot of the same build after the marker (resume once,
markerless crash boot closes work) is the existing planned-refresh path and is
not re-proved here. Proof on d@e after deploy: `choir paper restart --computer
computer-59c902c5d46caecc0a180a8557164e72 --reason "O5 same-version proof"` must answer
`restarted`; a second call within five minutes must answer `restart_rate_bound`.
