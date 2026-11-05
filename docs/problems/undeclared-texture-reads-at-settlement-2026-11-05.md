# Undeclared Texture reads after a settled trajectory (2026-11-05)

## Evidence

Staging d@e, build c4d733c89: one research-brief trajectory (Texture desk
turns, a research child, a final turn that passivated and then settled) left
the object-graph reader gate (observe mode) with
`choir.texture_document|undeclared|undeclared_reader: 1` and
`choir.texture_revision|undeclared|undeclared_reader: 2`. Before f58ba6707
(a passivated final turn settles the trajectory) the counts were empty.

## Investigation

Scratch instrumentation of `recordUndeclaredRead` (stack to a file, removed)
under `TrackUndeclaredReadsFromProduct`, run over the agentcore and
textureowner suites. textureowner product code declares every read
(`asTextureReader`); the undeclared product call sites in agentcore that a
Texture trajectory reaches are:

- `internal/agentcore/tools_desk.go:1142` (`answerIdleTextureTrigger`):
  `GetLifecycleDocument` at the end of a Texture activation, from
  `ExecuteActivationSyncChecked`, which runs on the actor handler's context
  (no desk, no reader). texture_document x1 per activation end.
- `internal/agentcore/texture_frame.go:294` (`textureRunOwnerRevision`):
  `GetLifecycleRevision` while the host builds a Texture frame, on a context
  `textureRunRevision` had wrapped for its own reads only. texture_revision x1
  per frame whose run names an owner revision other than the framed one.

Together they give document x1, revision x2 for a two-frame trajectory.
The trace was not available for the d@e run, so this attribution is a
hypothesis matched on counts and timing, not a confirmed finding; the
settlement path itself (`ReconcileLifecycleSettlementForTerminalRun`, the
passivation persist, the inbox rechecks) was driven in a scratch test and
declared every read.

Other undeclared sites seen in the sweep but not on this path, left as
residuals: `story_analysis.go:208/218/233` (a research frame reading the
story; a research read of a Texture kind is a cross read, to be resolved with
a lease, not declared as the custodian), `api_trajectory.go:425` (owner
route; production routes declare the owner), the rebase and stack-base wake
paths (deploy-time), and `rlm_reduce.go:2341` (reducer; desk context in
production).

## Fix

Both sites read on Texture's behalf and declare the custodian with
`withKindReader` (declares only where the context names no actor), the
pattern `textureRunRevision` already uses. Regression tests use
`TrackUndeclaredReadsFromProduct`.

## Site observer

The reader gate (and the writer gate) now records the call site of each
violation: the first stack frame outside `internal/objectgraph` and
`internal/store`. Read it on the runtime observability response, next to the
counts: `objectgraph_reader_gate.sites`, keyed
`kind|outcome|function (internal/pkg/file.go:line)`, capped at 64 distinct
keys with the rest counted under `overflow`. After a research-brief
trajectory on d@e, the `choir.texture_document|undeclared_reader|...` and
`choir.texture_revision|undeclared_reader|...` keys name the real callers.
