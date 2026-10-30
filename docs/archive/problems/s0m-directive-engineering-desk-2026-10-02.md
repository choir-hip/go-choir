# S0m: directive packets cannot wake the engineering desk agent

**Found:** 2026-10-02, during RN3a (note directive cutover) implementation.

## Statement

`note` directives (record-native packets, `direction=directive`) now mint and
deliver through `CommitLifecycleAct`. They reach:

- **persistent Management** (`management:<owner>`): computer-scoped packet,
  live-occurrence resolution, unbound injection — armed.
- **research/texture desk agents** (`<profile>:<docID>`): doc-scoped packet,
  pending-list visibility, bind via `ReconcileUpdateDelivery`, consume at the
  desk's committed turn — armed.
- **engineering desk agent** (`engineering:<docID>`): NOT delivered. The desk
  agent is a durable parent authority and mailbox that never runs — engineering
  cells run as per-assignment actors spawned by the delegated-cast saga. A
  directive addressed to `engineering:<docID>` stays pending forever: nothing
  binds it to a run, and `reconcileUpdatedCoagentActor`'s lifecycle branch only
  treats `research` agents as lifecycle agents.

## Evidence

- `internal/agentcore/management_controller.go:1728` —
  `lifecycleAgent := profile == agentprofile.Research && agent.LifecycleVersion > 0`
  excludes engineering.
- `internal/agentcore/engineering_desk.go:17` — "The desk agent never runs."

## Consequence

`choir.Note(to_desk="engineering")` mints the commitment record and the
directive packet durably (record is preserved — no silent loss), but no
engineering activation consumes it until this gap is armed. The record remains
discoverable on the ledger (`ListCommitmentRecords` by addressee); delivery is
the missing leg.

## Deferred repair sketch (RN3b candidate)

Route engineering-addressed directives into the desk's activation fan-in:
either (a) treat `engineering:<docID>` as a lifecycle agent in
`reconcileUpdatedCoagentActor` and bind directives to the desk's next
activation run, or (b) have the engineering desk's own wake path
(delegated-cast saga / desk reconcile) claim pending directives for its
`agentID` before spawning assignment actors. Option (b) respects the current
"desk agent never runs" invariant.

## Status

Residual, non-blocking for the `note` slice: notes to `management`,
`texture:<doc>`, and `research:<doc>` deliver; `engineering` delivery is the
named gap this doc preserves.
