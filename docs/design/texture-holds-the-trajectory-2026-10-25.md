# Texture holds the trajectory (2026-10-25)

Owner ruling, 06:3x: "i dont want to build more code, but rather fix what we
have. texture, for example, should hold onto the trajectory, and always
remember to prioritize the user intention." Problem record:
docs/evidence/selfdev-trace-reviews-2026-10-25.md (rounds 2 to 4, the
clustering note) and docs/design/texture-spec-follows-owner-2026-10-25.md.
Mutation class: red (Texture's run context, the input to its canonical
writes). Review tier 1: additive, behind a computer setting, one-act rollback.

**Finding** means a trace line or the code shows it; **hypothesis** means
inference.

## How a Texture desk's conversation persists today

There are two paths, both keyed by the agent `texture:<doc>`:

1. **Same-run reactivation** (`reactivatePassivatedTextureRun`,
   texture_controller.go). A Texture run that passivated without its
   mutation completing, on the same head, is reactivated with
   `actor_reactivate_existing_memory`. It replays its own run memory byte for
   byte. This is the management precedent: persistent management reactivates
   its passivated run the same way (management_controller.go) and is bounded
   by in-run compaction. Finding (code; reader-4 runs a2e92f87, 43629206,
   5046343e, bad15657, f0cb56e2 carry `actor_reactivated_from_passivated`).
2. **A new run per committed wake, seeded with an actor rewarm snapshot.**
   Once a Texture turn commits a revision its mutation completes, so the next
   wake starts a new run (`submitTextureAgentRevisionRun`). The run's memory
   is seeded by `seedActorMemorySnapshot` (run_memory.go) from the newest
   completed or passivated run of the same agent
   (`LatestActorRunMemoryEntries`, keyed by `agent_id`). Finding (code, and
   the unit test `run_memory_test.go`). On staging the snapshot is present:
   every rewarmed run's first call has one more message than a first run of
   the same intent (e3e37430: 9173d70d, no prior, 4 messages; dc8b0e5d, same
   intent with a prior, 5). Finding (trace counts); that the extra message is
   the snapshot is a hypothesis from the code, the only append on that path.

`actor_rewarm_source_loop_id` is the same query run earlier by textureowner
and written on the run. Nothing reads it because the runtime repeats the
query itself. Finding (code). It is a label, not a dead mechanism.

## Why it does not hold the trajectory

The snapshot is the deterministic `summarizeRunMemoryMessages` digest, built
for a last-resort checkpoint, not for continuing a desk (finding, code):

- each message's text is cut to 350 characters, each tool result to 250;
- a `tool_use` is printed as its name and id only. Texture acts in Go cells,
  so every Cast, Write and Note it made is a `tool_use` input and is absent
  from the snapshot: Texture cannot see what it cast or wrote;
- the previous checkpoint is cut to 2000 characters and then nested in the
  next snapshot, so after one or two wakes the earlier history is gone.

So the owner's 02:59 direction on e3e37430 (in dc8b0e5d's request turn, after
the 7075-byte seed) is cut at 350 characters in 230c5a4e's snapshot (finding,
code plus prompt lengths). On reader-4 the owner's original request was the
first revision's body (62d2e3e6, 8484 characters, no `seed_prompt`), and
Texture's first revision replaced the body with a summary (19440c75, 762
characters). After that no turn carried the request: not the request turn
(no seed), not the snapshot (truncated frame), not the frame (the new body).
Texture asked management three times to find it (06:17:04, 06:18:02,
06:18:35). Finding (trace ac0cb480, revisions and records).

The instructions add to it (finding, texture.yaml): nothing tells Texture
that the owner's newest direction governs, that a cast carries the owner's
words rather than a summary, or that a superseded spec is never re-cast. The
two-sentence first cast on reader-4 and the verbatim re-casts on reader-3 are
what the prompt allowed. Round 5 (06:40:17): Texture woke on engineering's
interim progress report and cast "Continue the authorized repair" while
management's bound assignment was doing exactly that, so a duplicate round
opened (finding, trace review round 5). A digest that drops Texture's own
casts cannot tell it that its cast is in flight (hypothesis on cause).

## The fix

Connect what exists; no new package, no projection layer.

1. **The rewarm carries the conversation, not a digest.** Behind the new
   computer setting `texture.hold_trajectory` (off by default), textureowner
   marks a Texture run `actor_rewarm_carry_conversation`. For such a run,
   `seedActorMemorySnapshot` reads the run named by
   `actor_rewarm_source_loop_id` (the key is now read; the newest same-agent
   run if it has no memory) and appends that run's latest checkpoint and the
   messages after it, verbatim, before the new wake's turns. The bound is
   the one the run memory already has: threshold compaction before each
   provider call, with the LLM checkpoint (objective, user hard constraints,
   decisions, open obligations), and the existing tool-result elision. The
   carried prefix is the earlier run's exact bytes on the same prompt-cache
   thread (`texture:<doc>`), so most of it is a cache hit.
2. **The original request is visible without hunting.** With the same
   setting on, when the head carries no `seed_prompt`, the existing
   owner-direction walk continues to the document's first revision and,
   when the owner wrote it, the request turn prints its text as the owner's
   request (capped). The existing labels and the owner-directions ranking
   apply unchanged.
3. **Texture's instructions put the owner first** (texture.yaml, a short
   block): the newest owner direction governs; a cast to management carries
   the owner's request and directions in the owner's words; a superseded spec
   or in-flight request is never re-cast (a progress report updates the
   document and casts nothing); a correction while work is in flight is
   cast as a correction and asks management to pass it to the bound
   engineer in full.
   This is prompt text, unconditional; its rollback is the revert.

Off, the run and the turn are byte-identical to today.

## PD2 and PD3

The continuity is keyed by the agent `texture:<doc>`, never by a run: the
source run is chosen by agent and must hold that agent's memory. That is
PD2's rule ("desk workers keyed by agent ID, not run ID"; "the next Texture
turn sees what its earlier runs defined") applied to the desk's conversation,
where PD2's landed slice applied it to the desk's Go session. PD2's realism
axis is in-memory session per agent, then retention, then checkpointed
sessions across a planned restart. The conversation half lives in run memory,
which is already durable, so it is the checkpointed rung for conversation; it
does not resume work after a crash, since only a new wake starts a run.

PD3 keeps `texture:<doc>` as each document's sub-RLM, the sole writer of its
document. This fix is that sub-RLM's own continuity. It adds no root, no
routing and no new identity, so PD3's axis (a root that routes wakes, then
the master document from the ledger, then sub-RLMs as children with PD2
sessions) builds on it with nothing to undo.

## Failure modes, written before the code

1. Off changes the run (seed or turn). Test: switch off, the new run's memory
   is the digest snapshot, as today.
2. On, the earlier conversation is still lossy. Test: a cast in an earlier
   cell's `tool_use` input appears whole in the next run's context.
3. A carried runtime injection counts as delivered in the new run (injection
   dedupe, the frame mark and report authority read only
   `runtime_injection` entries). Test: carried entries are stored with their
   message role, never `runtime_injection`.
4. The carry crosses actors: a source run of another agent, or a run id of
   another owner. Test: a source whose entries belong to another agent is
   refused and the newest same-agent run is used.
5. The carried tail ends on a `tool_use` with no result (a run passivated
   mid-cell), which providers refuse. Test: the dangling call is dropped.
6. Old run-context turns (old times, old facts) are carried. Test: they are
   dropped; the new run appends its own.
7. Unbounded growth. The carried run compacts at the threshold like any long
   run; a carried checkpoint is carried as a checkpoint, not re-expanded.
   Test: the earlier run's checkpoint is carried as the new run's checkpoint
   with only the messages after it.
8. A non-Texture run with the flag changes. Test: the flag is honoured for
   Texture runs only.
9. The original request is missing when the body was the request and Texture
   rewrote it (reader-4). Test: a document created with the request as its
   body, a Texture rewrite, an owner revise; the next wake's turn contains
   the request text.
10. The original request prints twice when a seed exists, or when the head is
    the first revision. Test: with a seed, the root body is not printed.

## Is texture.owner_directions redundant once this is on?

Partly, not yet wholly. While the conversation is below the compaction
threshold, every earlier direction is in it verbatim, so the walk's list
repeats what Texture already holds. After a compaction the directions survive
only as the checkpoint's summary, and the walk is the verbatim copy from the
revision chain. Keep it until staging shows a compacted Texture run still
obeying a direction given before the compaction; then it can be deleted, or
kept as the post-compaction refresh.

## Rollback

Live: set `texture.hold_trajectory` off on the computer (owner act, no
deploy); the next wake seeds the digest snapshot again. Runs already carried
keep their memory; nothing else is written. Code: revert the fix commit (the
prompt text included).

## Heresy delta

- discovered: the actor rewarm snapshot drops every `tool_use` input and cuts
  text to 350 characters, so a committed Texture wake does not see its own
  casts, writes or the owner's directions from earlier runs;
  `actor_rewarm_source_loop_id` is written and then recomputed rather than
  read; a document whose request is its first body loses the request after
  Texture's first revision (no `seed_prompt`).
- introduced: none intended. A carried Texture run has a longer context
  (bounded by the existing compaction), so input tokens per wake rise until
  the first compaction.
- repaired (switch on): Texture continues its own conversation per document
  across wakes; the owner's original request reaches every turn; the prompt
  ranks the owner's newest direction first and forbids summary casts and
  superseded re-casts.

## Residuals

- Nothing refuses a duplicate cast mechanically (round 5's re-cast on a
  progress report). This slice gives Texture its earlier cast in view and
  the rule; a store-side refusal of a second execution request while one is
  bound is management's or the store's, a later step if staging shows the
  model still re-casts.

- Management not forwarding an owner correction to a bound engineer (round 4
  P1) is management's text (lane fp0-round4); Texture's cast now asks for it
  in words.
- Whether gpt-6-luna follows the newest direction with the whole
  conversation in view is a staging question. Next probe: on a test document,
  a request, a Texture revision, an owner revise, a report wake; read the
  next cast for the owner's words and the absence of the old spec.
