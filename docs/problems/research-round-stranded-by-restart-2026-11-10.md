# A research run parked by a planned restart is refused on its one resume, and its round stays open forever (2026-11-10)

Status: open; documented before any fix. Mutation class red (run
lifecycle and restart); the fix is a separate commit.

## Problem

A planned restart parks every pending or running run for "the one resume"
(CLAUDE.md, Restarts End Work Or Resume It). A work-bound research run
parked that way is reactivated by its durable `initial_dispatch`
occurrence, then refused at provider admission with `Research agent does
not own the exact live run`. The refusal passivates the run again and
returns without reaching `closeRoundsOfEndedRun`, so the Texture-to-research
round (round 1) stays open, `lifecycleSettlementReady` never passes, and the
trajectory stays `live` although its work item completed. Nothing later
re-wakes or closes the run, so the stranding is permanent.

## Evidence (trace bundle, staging test computer d@e, build 1380cec7f)

Trajectory `c11865a4-14b9-53e1-8e6e-35466ead6a62`, computer
`computer-59c902c5d46caecc0a180a8557164e72`, bundle
`~/.config/choir-qa/traces/c11865a4-14b9-53e1-8e6e-35466ead6a62.json`.

- 12:00:49Z research run `f7e7e79a` starts (`research:13604ed8-...`);
  eleven desk cells and five keenable searches to 12:01:34Z.
- 12:01:49.135Z Texture run `b3ebaeda` and 12:01:49.218Z research run
  `f7e7e79a` both emit `activation.passivated {"recovery":
  "passivated_on_restart"}`. That payload is written only by
  `passivateInterruptedActivations` (`runtime.go:3125`, the marker-boot
  branch), so a planned restart marker was present at that boot (shown by
  code plus event).
- 12:02:05.318Z lifecycle event `reactivate-run:f7e7e79a-...:2026-11-10T12:02:05.318250911Z:1`
  (kind `run_reactivated`): the research run is reactivated.
- 12:02:05.49Z run `f7e7e79a` is `passivated`, metadata
  `passivated_reason: lifecycle_researcher_provider_admission_refused`,
  `provider_admission_refusal: "Research agent does not own the exact live
  run"`, `request_source: inbox_record`. No event after 12:01:49 on that
  loop.
- The Texture run `b3ebaeda` has a second `loop.started` after its parking,
  writes revision `a1fd3c99`, and ends 12:02:35Z; a rewarmed Texture run
  `05e96808` completes 12:02:55Z. Texture resumed (via
  `resumeRestartParkedTexture`); research did not.
- Final state: trajectory `live` (`lifecycle_version` 5, `reducer_seq` 6,
  last update 12:02:54Z); inboxes `texture:13604ed8-...` and
  `research:13604ed8-...` both show open round 1, caster Texture, recipient
  research, `conv:research:13604ed8-...|texture:13604ed8-...`.

## Walk through the code (inferred from reading, not yet from a drill)

1. Boot passivation: `passivateInterruptedActivations` (`runtime.go:3081`)
   sets state `passivated`, `passivated_reason: runtime_restarted`, and
   writes through `store.UpdateRun`. For a lifecycle-bound run that goes
   through `projectLifecycleRun`, which holds the invariant
   `lifecycleRunOwnsActivation(state)` (pending or running) -> `agent.
   ActiveRunID = run`, else if the agent's active run is this one -> `""`
   (`internal/store/lifecycle.go:3367-3371`). A passivated run therefore
   clears the agent's `ActiveRunID`. This is correct for a parked run.
2. Resume: the durable `initial_dispatch` occurrence is redelivered;
   `handleInitialDispatch` sees `restartParked` and calls
   `ReactivateRunCanonical(..., RunRunning, ...)`
   (`internal/actorruntime/handler.go:686`), then
   `ExecuteActivationSyncChecked`.
3. `ReactivateRun` (`reactivateRunOnce`, `internal/store/lifecycle.go:5541`)
   rewrites the run and the trajectory and emits `run_reactivated`, but it
   never touches the agent record. The run is pending or running again while
   `agent.ActiveRunID` is still empty: the invariant of step 1 is broken.
   (`TerminalizeRun` clears the slot explicitly, `lifecycle.go:5510-5525`;
   `ReactivateRun` has no matching set.)
4. `executeActivation` first runs `admitLifecycleResearchProviderEntry`
   (`runtime.go:3924`, check at `~3707`), which requires
   `agent.ActiveRunID == run.RunID`. It does not, so the run is refused,
   passivated again by `passivateLifecycleResearchWithoutProviderAuthority`
   (`runtime.go:3870`), and `executeActivation` returns at `~3941`.
5. A run that ends this way never goes through `closeRoundsOfEndedRun`
   (`internal/agentcore/inbox_records.go:~1383`), and the boot-close of open
   obligations belongs to the markerless branch (`closeOpenObligationsAtBoot`),
   which did not run because the marker was present. The planned-restart
   resume is the only owner of this run and it refused.

So the existing mechanism that should have handled it is the planned-restart
resume itself (marker present: resume), and it is reached; the defect is that
its reactivation does not restore the activation slot the passivation
cleared, so a work-bound research run cannot pass the very admission that
`WakeRecordsInbox` and `projectLifecycleRun` satisfy by construction.

## Belief state

- Shown (events and stored run): the double `passivated_on_restart`; the
  reactivation at 12:02:05; the stored refusal reason and text; the open
  round; the live trajectory; Texture resumed.
- Shown by code: the passivated-run clear of `ActiveRunID`, the absence of a
  set in `ReactivateRun`, and the admission check.
- Inferred, not shown: that `agent.ActiveRunID` was empty at the refusal
  (the bundle has no agent record layer; the refusal text and the code path
  agree). The restart's own record (marker, boot log of 12:01Z) was not in the
  bundle: the log tail begins 12:39Z. The failing store test below reproduces
  the slot state without a restart.
- Decision from doctrine: the rule is marker present -> resume. The run was
  parked for that one resume, so the fix makes the resume succeed; closing it
  with "interrupted by a restart" would be the markerless rule applied to a
  marker boot. Note the same slot gap affects any lifecycle research run
  reactivated from `passivated` (channel_message, update_coagent); those are
  fixed by the same change.

## Residual (not fixed here)

A run that is refused at admission for any other reason is parked
`passivated` with its round open and no owner re-wakes it. That is the
sibling of
`research-ask-orphaned-by-planned-restart-2026-10-27.md`. It is left as a
named residual; closing the round on a permanent refusal needs a decision on
which refusals are permanent.

## Next

Failing store test (a reactivated run owns the agent's activation slot), then
the fix in `reactivateRunOnce`.
