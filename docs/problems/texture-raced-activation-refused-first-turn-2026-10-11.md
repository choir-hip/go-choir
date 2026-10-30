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
  [smg-management-open-invalid-transition-2026-10-07.md](../archive/problems/smg-management-open-invalid-transition-2026-10-07.md):
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

## Root cause (2026-10-15, from code)

Confirmed from code; the staging mutation rows have not been read, so the
trace-level confirmation is the next observation (expected: the recovered
run's row in `stale_activation`, `completed_at` a few ms after its
`created_at`, and a second `stale_activation` row for the wake's own run).

The two activators of a fresh Texture document do not share a lock, and the
wake's cleanup stales a pending mutation that belongs to the other one.

1. `ensureConductorTextureRoute` (texture_handoff.go) calls
   `StartLifecycle`, then `submitTextureAgentRevisionRun` → agentcore
   `createRunWithMetadata` → `persistLifecycleSubmittedRun`. That writes the
   handoff run's (H) `texture_agent_mutations` row as `pending` **first**
   and commits H's activation (`ReplaceLifecycleActivation`) **second**. It
   does not take `lockTextureWakeScope`.
2. `StartLifecycle` also assigns the initial work item, which wakes the
   Texture mailbox (`lifecycle_work_assigned`). The actor goroutine runs
   `ReconcileAgentWake`, which takes `lockTextureWakeScope` — but nothing
   else that activates this document holds it, so the lock excludes
   nothing here.
3. `reconcileAgentWakeLocked` (texture_controller.go) checks
   `TextureActiveRunByAgent` (store-backed). If H's activation has not
   committed yet, it finds nothing and goes on. A dozen store reads later
   (pending updates, snapshot, stranded deliveries, breaker streak, inbox,
   runs by channel, reactivation), it runs the "stale unbound pending
   Texture mutation" loop: `GetPendingAgentMutationByDoc` selects by
   **document**, not run, and `MarkAgentMutationStale` moves every pending
   row for the doc to `stale_activation`. If H's row was written by then,
   **H's mutation is staled**.
4. The wake then submits its own run W. W's activation fails with
   `ErrLifecycleInvalidTransition` because H is now active; W's rollback
   stales W's own row (keyed by run, correctly), and
   `recoverRacedTextureActivation` returns H ("recovered raced activation
   for doc … run H").
5. H is the doc's active run with a `stale_activation` mutation. Its
   `initial_dispatch` fails `ValidateActivationAuthority` ("mutation
   authority mismatch") eight times and is poisoned; any occurrence for it
   fails `ValidateOccurrenceActivationAuthority` ("Texture occurrence/run
   mutation mismatch"). Nothing ever moves H, and the dead letter has no
   handler.

The bad interleaving is: wake's active-run check < H's activation commit,
and H's mutation insert < wake's stale loop. When the wake wins instead (W
commits first), the handoff loses, stales only its own row, recovers W, and
W's row is pending — that is the 15 of 19 races that recovered cleanly. The
one-in-five rate is the fraction of races where the handoff's two writes
straddle the wake's read window.

What the code rules out:

- `agentMutationComputerID` is not the cause: both runs come from
  `submitTextureAgentRevisionRun`, which refuses to submit without
  `lifecycle_work_item_id` (unless records route), so both rows are scoped
  to the computer id the validators expect.
- The loser's rollback in `persistLifecycleSubmittedRun` is keyed by its own
  run id; it never stales the winner. Hypothesis 2 above is refuted.
- Hypothesis 1 is right in kind: the winner's row moved out of `pending`
  before the check — staled by the concurrent wake, not by a read-visibility
  lag.

Substrate: two activators for one Texture desk that are not serialized.
Fix shape: every caller that activates a document's Texture desk holds that
document's wake scope (`lockTextureWakeScope`), as the wake already does.
The handoff (`ensureConductorTextureRoute`) and the frozen coagent route
(`ensureCoagentTextureRevisionRoute`) are the two unlocked callers of
`submitTextureAgentRevisionRun`. Under the lock, whichever activator runs
second sees the first one's committed activation (the wake returns early on
a valid active run; the handoff loses the activation and recovers the
wake's run), and the stale loop only ever sees rows whose activation failed
or crashed.

Residual: `delivery_failed` (and `delivery_invalid`) reach the
`delivery-poison` sink with only the poisoned update id as content and fall
through to the "unknown update kind" default. Giving them a visible fate
(failing the run the poisoned `initial_dispatch` named, with a reason the
owner and desks can see) needs the poisoned update's kind and content on the
dead letter or a log lookup by update id; neither exists today, so it is not
a small change and is left as a named residual for the protocol's
observation records (P2 or later).

## Fix (2026-10-15, branch `fix/texture-raced-activation-mutation`, not yet deployed)

Commit 41cdcb94c: `ensureConductorTextureRoute` and
`ensureCoagentTextureRevisionRoute` hold `lockTextureWakeScope` around
`submitTextureAgentRevisionRun`. Regression
`internal/textureowner/texture_activation_wake_scope_test.go` reproduced the
staging refusal before the fix ("validate Texture activation: mutation
authority mismatch" on the recovered run) and passes after it.

Status: fixed locally, not deployed. Deployed proof still owed: after
deploy, guest consoles that log "recovered raced activation" show no
"mutation authority mismatch" or poisoned initial_dispatch for that doc.
The `delivery_failed` visible-fate handler remains a residual (see Root
cause).

## After the wake-scope fix (2026-10-15, 05:00Z): the race still poisons

Deployed `e4cdc2ba6` (contains 41cdcb94c, the handoff holding
`lockTextureWakeScope`). Tracer `selfdev-jobs-0458`, guest
vm-241980cd810414f23a06b6399adf121b (computer-d7c52c2bb443f461ad666f06309f090b):

```
05:00:20 texture api: recovered raced activation for doc 61e04b82-… run f80d052d-…
… validate Texture initial_dispatch: validate Texture activation:
  mutation authority mismatch   (×7)
05:00:20 dispatcher: poison 078dd8aa-…
05:00:20 actorruntime: handler: unknown update kind "delivery_failed"
05:02:25 texture api: recovered raced activation for doc 61e04b82-… run f80d052d-…
```

So the confirmed mechanism (the handoff not holding the wake scope) was real
but not the only path: with the handoff now serialized, a raced activation
still leaves the recovered run's mutation un-pending and the first dispatch
is still refused and poisoned. The guest kept making model calls (52 by
05:05), so a later activation may carry the story; the tracer's result will
say whether the first turn was lost.

Belief update: a second writer stales or never creates the winner's
mutation, outside the two submit paths now locked. Not yet identified.
Candidates to check from the trace and code: the `initial_dispatch`
occurrence's own reconcile (`reconcileAgentWakeLocked`'s stale-unbound loop)
running for a *different* wake kind than the one the lock covers; the
records-route activation; and whether `recoverRacedTextureActivation`
returns a run whose mutation was never created (no row at all, rather than
a stale one). The distinguishing observation is the mutation row state for
run f80d052d after the poison (read from the stopped guest's store copy, or
add a log line naming the row state on refusal).

This is the second fix iteration on this problem; per Convergence Before
Patching, the next step is a trace read and, if it does not converge, a
structural assessment instead of a third patch.

## Second path, from the trace and code (2026-10-15)

### What the guest console shows (vm-241980cd, runtime lines 04:58:16–05:02:25)

- 04:58:16 the computer boots on a fresh store.
- 04:58:35 Texture run **88df194f** writes revision f7335bc5 on doc 61e04b82
  (`texture cell authored turn`, `done:false`). **The first prompt was
  answered.** This incident is not a lost first turn.
- 04:58:48–05:00:31 management works the tracer (rounds, assignments,
  capsules).
- 05:00:19 Texture run **985d8cd7**, a second run, writes revision 16d146ba
  (base f7335bc5, `done:false`).
- 05:00:20, within one second: `recovered raced activation … run f80d052d`.
  f80d052d is a **third** run. Its `initial_dispatch` (update 9625af45) fails
  `validate Texture activation: mutation authority mismatch` eight times in
  about 110 ms and is poisoned. The validator got as far as the mutation
  check, so f80d052d's `current_revision_id` already equals the new head
  16d146ba. It was minted after 985d8cd7's write.
- 05:02:25 another wake: `recovered raced activation … run f80d052d` again.
  Two minutes later, f80d052d is still the agent's active run (pending) and
  its mutation is still not pending. Every later wake "recovers" it and does
  nothing. The desk is wedged until something passivates f80d052d.

No log line names who minted f80d052d (a successful submit does not log), and
the mutation rows have not been read (the guest is running, so its disk is
off limits).

### The path

The loser at 05:00:20 and 05:02:25 is a **locked** wake
(`reconcileAgentWakeLocked`). After the wake-scope fix, every caller of
`submitTextureAgentRevisionRun` holds the lock, so a locked wake that runs
after f80d052d's activation committed should have found it active and
returned. It did not, because its active-run check asks a different
authority than the activation gate:

- The activation gate (`projectLifecycleRun`, store/lifecycle.go) and
  `recoverRacedTextureActivation` use **`agent.ActiveRunID`**: a new
  activation is refused while that run is pending or running.
- `TextureActiveRunByAgent` (agentcore/texture_lifecycle_api.go) calls
  `GetLatestActiveLifecycleRunByAgent`, which lists the owner's run objects
  `ORDER BY updated_at DESC`, takes the **first run of that agent**, and
  returns `ErrNotFound` if *that* run is not active. Any later write to an
  **older, inactive run of the same agent** (985d8cd7 passivating or
  completing after its successor activated, or a metadata or run-memory
  update to it) hides the active run.

With the active run hidden, the locked wake does what the handoff race did:
it passes the "no active run" check, its stale-unbound loop
(`GetPendingAgentMutationByDoc`, picking by doc) stales **f80d052d's**
pending mutation, its own submit loses with `ErrLifecycleInvalidTransition`,
and recovery returns f80d052d with a `stale_activation` row. Both validators
refuse it, and the dispatch is poisoned. Every later wake repeats the hidden
check and recovers the same wedged run.

This also fits the 2026-10-11 and 2026-10-15 02:23 incidents: the first
desk run of a fresh doc races the handoff, and the previous run's late
update is the same hiding mechanism.

Candidates from the earlier section that the code rules out here:

- **Records-route activation:** it goes through the same locked
  `reconcileAgentWakeLocked`.
- **A run with no mutation row:** `persistLifecycleSubmittedRun` writes the
  row before the activation, and recovery only returns committed
  activations, so a missing row would need a crash between them. A missing
  row and a stale one fail with the same message; the path above produces a
  stale one.

Substrate: two state authorities for "the desk's active run" that disagree:
`agent.ActiveRunID`, which the store enforces, and "latest-updated run",
which the wake reads. Fix shape: `TextureActiveRunByAgent` reads
`agent.ActiveRunID` first (the authority the gate and recovery use) and
keeps the latest-run scan only as a fallback for agents without one. A run
the gate would refuse a successor for is then always visible to the wake,
so the stale loop never sees the active run's pending row.

The `delivery_failed` residual is unchanged.

### Fix for the second path (branch `fix/texture-raced-activation-2`, not deployed)

Commit 44944dc72: `TextureActiveRunByAgent` reads `agent.ActiveRunID` first.
Regression `internal/textureowner/texture_active_run_authority_test.go`
reproduces the staging signature before the fix (the active run is hidden;
after the wake, "mutation authority mismatch") and passes after it. Deployed
proof owed: a tracer whose desk takes more than one run shows no refusal
after a "recovered raced activation". The f80d052d computer stays wedged
until its active run is passivated or the computer restarts. The fix
prevents new wedges but does not repair existing ones.
