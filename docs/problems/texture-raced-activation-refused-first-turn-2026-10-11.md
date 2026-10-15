# A raced Texture activation can leave the first prompt unanswered

Found 2026-10-11 during the P1 deployed demo (staging 0f1c86c7, reader test
computer demo-arxiv-2026-10-10c, guest vm-82cf0bb76f586b9cbaf492e272d94fa2).
Status: documented, not fixed. The cause below is a hypothesis until a trace of
the mutation row confirms it.

## What happened

The owner prompt went in at 17:07:20Z. Texture's first run (1fbdbe01) recorded
`loop.submitted` and `texture.agent_revision.started` at 17:07:21 and then
nothing for eight minutes. The document showed only the user's revision until
the demo's follow-up at 17:15:34 woke a new Texture run (4a5fb1fe), which wrote
a revision and opened research. A real owner would have seen no answer to
their first prompt.

Guest console, 17:07:21 (all from the same second):

```
texture api: recovered raced activation for doc 00f36207-… run 1fbdbe01-…
actorruntime: Texture occurrence texture:00f36207-…/a861a6f8-… (coagent_result)
  consumed without a turn: reconcile refused: invalid Texture actor occurrence:
  Texture occurrence/run mutation mismatch
dispatcher: handle be4649cf-… (×7)
dispatcher: poison be4649cf-…
actorruntime: handler: unknown update kind "delivery_failed" for agent computer-eb87caee…
```

The repeated `lifecycle stream: page error … context canceled` lines in the
same window are UI stream pages cancelled by the browser, not this failure.

## What is known

- The activation race is the one in
  [smg-management-open-invalid-transition-2026-10-07.md](smg-management-open-invalid-transition-2026-10-07.md):
  the `initial_dispatch` wake activates Texture before `EnsureTextureHandoff`
  submits, and the loser recovers the winner's run
  (`recoverRacedTextureActivation`, internal/textureowner/texture_agent_revision.go).
- That recovery normally works. Node B guest consoles show 19 recovered races
  across four guests and one refusal (this one).
- The refusal comes from `ValidateOccurrenceActivationAuthority`
  (internal/textureowner/texture_controller.go): the run's agent mutation was
  missing, scoped to another document or run, or not `pending`.
- The dispatcher poisoned the handle after seven tries, and the resulting
  `delivery_failed` update reached a computer agent with no handler for it, so
  the failure was invisible to the owner and to every desk.
- P1 is not implicated: no control was issued in that window. The P1 audit
  covers the control issued later, at 17:16:00.

## Hypotheses (unconfirmed)

1. The occurrence was checked against the recovered run before the winner's
   mutation row was visible to the reader, or after it had moved out of
   `pending`.
2. The occurrence was bound to the loser's run, whose mutation the failed
   activation marks `stale_activation` (`persistLifecycleSubmittedRun`'s
   rollback), while the console names the recovered winner.

The next observation that distinguishes them: the mutation rows for doc
00f36207 (`ListRecentAgentMutationsByDoc`) and the occurrence's bound run id.

## Why it matters

A refused first turn is the worst failure for the owner: nothing happens and
nothing says so. The poison path ends in an unhandled update kind, so no
supervision surface records a fate. Under the protocol (design §3), a refused
delivery would be an observation record addressed to the sender; today it is
a log line.

## Disposition

Not fixed in P1. Named as a residual on the P1 goal file. Candidate fix shapes,
for whoever takes it: make the recovered run's first occurrence re-resolve the
current active run and mutation instead of refusing; and give
`delivery_failed` a handler that records a visible fate. The second belongs to
the protocol's observation records (P2 or later).

## Later observation (2026-10-11, 19:20Z)

The P2a deployed demo on a fresh test computer (demo-p2a-2026-10-11,
staging a2bcf724) took its first Texture turn about twenty seconds after the
prompt, with no refusal. One clean run does not refute either hypothesis;
the race may need an existing computer with prior desks.

## Recurrence (2026-10-15, 02:23Z): a self-development tracer lost its story

Tracer `selfdev-jobs-0223` (staging 133de45f, test computer
computer-2a4710004ee4d5cf474ceb68596e4492, guest
vm-a767721d2f30d552cca32ad6b99a83f8) submitted its story at 02:23:29 and then
reached nothing for 45 minutes: no Texture cast, and the gateway saw **zero**
inference calls from that guest. The 01:01 tracer on the previous build
(e83735f2) reached `applied` with the same story; its guest also logged a
recovered race, but no refusal.

Guest console, all at 02:23:29:

```
texture api: recovered raced activation for doc df75020c-… run 12086fec-…
actorruntime: Texture occurrence texture:df75020c-…/e9098fe7-… (coagent_result)
  consumed without a turn: reconcile refused: invalid Texture actor occurrence:
  Texture occurrence/run mutation mismatch
dispatcher: handle … texture:df75020c-…/ae8042ea-…: actorruntime: validate
  Texture initial_dispatch: validate Texture activation: mutation authority
  mismatch   (×7)
dispatcher: poison … after 8 attempts
actorruntime: handler: unknown update kind "delivery_failed"
```

So the refusal hits **both** checks on the recovered run: the occurrence check
(`ValidateOccurrenceActivationAuthority`) and the initial-dispatch check
(`validateTextureActivation…`, texture_controller.go:360-372). Each requires a
`pending` `texture_agent_mutations` row for the run, scoped to the doc,
owner and computer.

Frequency across the 23 guest consoles on Node B now: 19 recovered races,
4 poisoned first dispatches (vm-0f24d3b8, vm-1669020a (the paper's computer),
vm-6a6b4c9e, vm-a767721d). Roughly one race in five loses the first turn.

A lead to check first: `agentMutationComputerID` (coagent_injection.go)
returns "" unless the run carries `lifecycle_work_item_id` or the records-route
flag, and both validators compare `mutation.ComputerID` with the computer id.
A winner run started by the `initial_dispatch` wake may carry different
metadata from the handoff's run, so its mutation could be written with an
empty computer id, or not at all. Unconfirmed until the mutation row is read.

Status: still documented, not fixed. Fix work moves to branch
`fix/texture-raced-activation-mutation`.
