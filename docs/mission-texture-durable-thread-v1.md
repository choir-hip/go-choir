# Mission: Texture Durable Thread v1

Clean successor to [mission-texture-long-running-agent-v0.md](mission-texture-long-running-agent-v0.md).
Do **not** use the v0 ledger as the source program. The v0 ledger is historical
failure evidence: it shows why run-centric park/rewarm, semantic role
choreography, prompt classifiers, and exact-first-tool scaffolding are the wrong
shape. Read it only when auditing a specific cautionary claim.

## Mission Claim

If each Texture document becomes one durable, owner-visible agent thread with
runtime-owned mailbox identity, honest work-state revisions, same-thread
sleep/resume, and always-deep evidence gathering, then Texture becomes Choir's
reliable artifact control plane instead of a sequence of reconstructed runs.

## Current Problem

Current code still behaves like a run controller with helpful patches:

- A quiesced/parked Texture is reconstituted by new runs from document head,
  recent channel messages, and run-memory summaries rather than resumed as the
  same literal thread.
- Work delivery is discovered through pending-update queries and cold/warm
  injection machinery instead of being appended to one durable thread mailbox.
- Texture revision runs can be forced into a first `patch_texture`, producing
  tiny edits that hide background work instead of making owner-visible work state.
- `update_coagent` asks models to invent `update_id`, even though the store uses
  it as an owner-scoped idempotency and delivery key.
- Classifiers and guards still encode semantic role choreography in runtime
  (`texturePromptNeeds*`, explicit prompt scanners, model-prior completion guard,
  no-op/prompt-copy guards, exact-first-tool retry machinery).

## Target Shape

- One durable Texture thread per document.
- `V0` is owner input; `V1+` are Texture responses or owner edits in the same
  artifact history.
- Each canonical revision is an assistant turn that writes Texture.
- Owner follow-ups, researcher findings, super results, blockers, and source
  evidence append into the same thread mailbox.
- Runtime mints or derives `update_id`; the model describes payload, target,
  evidence, blockers, and questions, not delivery identity.
- Texture updates promptly. If work is pending, delegated, or incomplete, the
  next revision is an honest work-state / acknowledgement revision, not a fake
  final answer or instruction cleanup.
- The thread deepens by default. Research is not a mode and not keyword-gated.
- The thread quiesces on idle/await/budget, with zero provider calls while
  asleep; it resumes the same thread when input arrives.
- The thread ends only on document delete/cancel.

## Non-Goals

- Do not patch the old run-centric spine until it looks acceptable.
- Do not recreate a fixed Texture -> researcher -> super workflow.
- Do not make Trace or Chyron the owner-visible source of work state.
- Do not use model-authored labels such as `checkpoint-1` as durable delivery
  identity.
- Do not preserve compatibility shims for non-users. Hard cutover is preferred.

## Parallax State

status: working

mission conjecture: if Texture document work is represented as one durable
thread with runtime-owned mailbox identity and owner-visible work-state
revisions, then owner supervision, deep research, source evidence, and
foreground edits become one coherent artifact trajectory rather than
run-reconstruction side effects.

deeper goal (G): make Texture the artifact control plane for Choir's
multi-agent work, where canonical versions, active work state, and source
evidence remain legible to the owner and durable across process refresh.

witness/spec (A/S):
- runtime-owned `update_id` for model-facing `update_coagent`;
- direct Texture owner requests write honest work-state revisions when work is
  delegated or pending;
- durable per-document thread identity and mailbox cursor;
- same-thread passivation/resume with literal thread memory or structured
  compaction, not doc-head reconstruction;
- event-driven update delivery into the thread;
- deletion of classifiers/guards/exact-first-tool scaffolding that encode
  semantic choreography;
- staging proof that direct Texture revise, researcher delivery, and multi-round
  grounded deepening happen through one durable thread.

invariants / qualities / domain ramp (I/Q/D):
- I: Texture owns canonical document versions. Researcher/super evidence is
  non-canonical until Texture incorporates it.
- I: Work-state revisions are canonical artifact state, not Trace dumps.
- I: `update_id` is internal delivery/idempotency machinery. Runtime owns it.
- I: Prompt-bar `V0` is owner prompt; direct Texture `V0/current` may be a full
  owner-authored document.
- I: Runtime may enforce mechanical protocols but not semantic role sequences.
- I: Hard cutover; prefer deletion over shims.
- Q: keep the v1 mission readable enough for a fresh agent to start without the
  v0 ledger.
- D0: focused unit tests for update-id minting/idempotency and work-state
  revision policy.
- D1: integration tests for researcher delivery into Texture and source/evidence
  consumption.
- D2: same-thread passivation/resume and event-driven mailbox tests.
- D3: deployed product proof on `https://choir.news`.

variant (ranking function) V:
1. runtime-owned update-id scheme designed and implemented;
2. model-facing `update_coagent` no longer requires model-invented `update_id`;
3. retry/collision tests prove idempotency without human-label collision;
4. direct owner-triggered Texture work-state revision test passes;
5. Texture first-tool policy stops forcing trivial patches as hidden work-state;
6. durable thread/mailbox data model implemented;
7. update delivery appends into the thread instead of cold reconstruction;
8. same-thread passivation/resume implemented;
9. classifiers/guards/exact-first-tool scaffolding deleted;
10. always-deep research prompt/loop behavior proven beyond the current V2 cap;
11. focused runtime tests and runtime shards pass;
12. landed to main, CI/deploy identity verified, staging acceptance recorded.
Current V=5; last ΔV=0 on 2026-06-21 from moving Texture update wakes off
cold prompt-prefix packet delivery and onto durable activation mailbox turns. A
parked Texture actor keeps the same `loop_id` and run memory across idle, can
resume that same loop for later `update_coagent` input, and advances the
Texture controller checkpoint even when a same-loop mailbox packet is consumed
without a canonical write. Fresh Texture update wakes now append pending packets
as the first run-memory mailbox turn instead of cold-prepending prompt context.
The remaining durable thread risk is replacement wake scaffolding for documents
without a resident/sleeping Texture actor and the classifier/exact-first-tool
residues.

budget: one broad red-surface mission, but execute in reviewable slices. R0a
(`update_id` + work-state revisions) may land first if it reduces risk, but it
does not settle the mission by itself.

authority / bounds: repo changes only until landing. No Node B tracked-file or
environment shortcuts. Behavior settlement requires commit, push to `origin/main`,
CI, deploy identity, and deployed product acceptance.

mutation class / protected surfaces: red. Protected surfaces are Texture
canonical writes, revision metadata, `update_coagent` schema and persistence,
worker-update delivery/marking, run lifecycle, passivation/rewarm, prompt
defaults, Trace/evidence semantics, and product acceptance.

evidence packet:
- focused tests for update-id minting, retry idempotency, and collision
  resistance;
- focused tests for owner-visible work-state revisions;
- integration tests for researcher delivery and Texture consumption;
- tests for durable thread mailbox, passivation/resume, and delete/cancel;
- runtime shard suite or justified scoped substitute before commit;
- staging proof with doc id, trajectory/thread id, revision count, source count,
  deployed commit, CI/deploy status, rollback ref, residual risks.

heresy delta: repairs H024a (trivial first patch as hidden work-state) and H024b
(model-invented coagent update ids); should also repair run-centric cold rewarm
and classifier/guard residues named in the v0 cautionary mission.

position / live conjectures / open edges:
- C1 supported for the update-identity half of R0a: runtime-owned
  `update_id` can land before the full thread rewrite without adding run-centric
  routing. It remains a ramp repair, not mission settlement.
- C2 active: the durable mailbox cursor can replace pending-update rediscovery
  without losing crash recovery.
- C3 active: same-thread resume needs a run-lifecycle model change, not only
  prompt or controller edits.
- C4 active: always-deep research should be prompt/obligation driven, not
  keyword-classifier driven.
- C5 supported at focused-test scope: direct user-authored revise requests no
  longer force exact `patch_texture`, and an end-to-end direct revise test proves
  a canonical Texture work-state revision is written before researcher
  delegation. This repairs the local R0a H024a shape; staging still must prove
  the product path with a real provider before mission settlement.
- C6 supported at runtime scope: `coagent_mailboxes` records a durable
  contiguous processed cursor per addressed actor, and actor backlog reads no
  longer filter on `delivered_at`; delivered markers are audit compatibility,
  not the actor-facing cursor authority.
- C7 supported for fresh, resident, restart-reactivated, and idle-resumed
  Texture update delivery: new `update_coagent` packets are appended as durable
  run-memory user turns, including first-turn `activation_mailbox_turn` delivery
  for fresh wakes. Texture no longer uses cold initial packet prep. This still
  does not settle the full event-driven cutover because replacement wake
  scaffolding remains when no resident or sleeping Texture actor exists.
- C8 supported at focused-test scope: normal Texture idle quiescence now
  passivates the run as a sleeping actor instead of completing it, keeps the run
  memory intact, and resumes the same `loop_id` when new mailbox input arrives.

next move: remove the remaining replacement wake scaffolding for Texture
delivery: require ordinary addressed `update_coagent` input to land in a
resident or sleeping Texture thread when a document already has a Texture
thread, define the explicit creation path for documents without one, and then
begin deleting the classifier/exact-first-tool residues that still encode
semantic choreography.

ledger file: docs/mission-texture-durable-thread-v1.ledger.md

version / lineage: v1 supersedes `docs/mission-texture-long-running-agent-v0.md`
as the source program. v0 and its ledger remain cautionary evidence only.
Depends on `docs/mission-texture-versioned-artifact-v0.md` for provenance,
source validation, hash chain, and full-history publication substrate.

learning state: retain old failure history as a warning against matching the
wrong pattern. Promote only compact lessons: avoid run-centric reconstruction,
avoid role choreography, avoid model-invented delivery identity, and keep
owner-visible work state in Texture.

settlement: settled only when landed and deployed proof shows runtime-owned
coagent update identity, direct Texture work-state visibility, durable same-thread
resume, event-driven delivery, deepening beyond V2, deletion of old scaffolding,
and no stranded source/evidence updates.

## Suggested Goal String

```text
Use Parallax on docs/mission-texture-durable-thread-v1.md. Treat it as the source program; do not use docs/mission-texture-long-running-agent-v0.ledger.md except as cautionary evidence for specific claims. Current status planned, V=12. First move: implement R0a if it does not entrench the run-centric spine: model-facing update_coagent without model-invented update_id, runtime-owned idempotency, and honest owner-visible Texture work-state revisions. Then continue toward one durable Texture thread per document: same-thread mailbox, event-driven delivery, passivation/resume, always-deep research, and deletion of cold rewarm/classifier/guard/exact-first-tool scaffolding. Mutation class red; protected surfaces are Texture canonical writes, update_coagent persistence/delivery, run lifecycle, passivation, prompt defaults, Trace/evidence, and product acceptance. Settlement requires commit -> push origin main -> CI -> deploy identity -> staging proof on choir.news with direct Texture work-state, researcher delivery without model-authored update_id, durable same-thread resume, deepening beyond V2, and rollback refs.
```
