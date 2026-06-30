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
- owner-visible Texture work-state revisions before delegated/pending work hides;
- durable per-document mailbox cursor and same-thread passivation/resume;
- event-driven delivery into the existing Texture thread;
- deletion of classifier/guard/exact-first-tool scaffolding that encoded semantic
  choreography;
- deployed proof that source-backed researcher evidence can deepen a Texture
  document, settle, and expose native source artifacts.

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
Historical implementation variant from the original 12 obligations is closed
for the source-evidence/source-panel axis by
`fce827ca2a43994d1d67312f33fe4fef1d97f4d3` proof:
runtime-owned update identity, work-state revision tests, durable mailbox
delivery, same-thread passivation/resume, deletion of initial-super/first-paint
model-prior scaffolds, source entity collation, saved-evidence delivery, and
passivation stream settlement all have receipts in the ledger.

Current exit V counts the newly reopened native-citation regression:
1. repair or refute the regression where source-backed Texture revisions may
   render ordinary web/markdown links and source-handle prose instead of using
   native Texture source entities and transclusion refs.

Current V=1. The independent-prover obligation is discharged for the scoped
support claim by Codex verifier thread `019eea0f-4872-7882-a1ea-f8e356ff40d9`,
which returned `accept_scoped_support`. It does not authorize broader settlement
than the scoped claim and residuals already named here. The current blocker is
not "source this" as a special feature; it is a regression against the
longstanding Texture source/citation contract: canonical source-backed prose
must preserve source object identity through `source_entities` and transclusion
refs, not reader-visible markdown URLs, "Source:" prose, or "Source handles"
sections as substitutes.

Last actual Delta V=-1 on 2026-06-21 from accepting the stale
Comet accessibility/live-region `Continuing...` node as a named polish edge.
The `_005` proof showed visible owner controls settled: no toolbar
`Revising...`, `Revise` enabled, `Sources` enabled and inspectable, and
`Publish v2` enabled. The lingering accessibility-tree text did not block the
durable-thread/source-panel mission. Discriminator: reopen as a new
accessibility/runtime problem only if screen-reader QA, live-region tests, or a
fresh Comet proof shows the stale node causes owner-visible confusion or blocks
assistive navigation after controls settle.

Automated widest-checker evidence is recorded for the latest test/code-touching
commit: CI run `27898525827` for
`4852cca612e817567f3ee57349d7a2a6504982da` passed runtime shards 0-3,
integration-tagged smoke, non-runtime Go tests, Go vet/build, TLA+ model checks,
Docs Truth Check, and deploy-impact detection; frontend build and staging deploy
were skipped because no deployed artifact changed. FlakeHub run `27898525830`
also passed. Later docs-only commits through
`2a402f18c783e4f7bacfde0e9e96e6a51839804c` passed Docs Truth Check
`27898703154`. Latest docs-only blocked checkpoint commit
`d550bee3ffaa56383a846de9d9cda4d76af0f68f` passed Docs Truth Check
`27898903344`.

Prior actual Delta V=-1 on 2026-06-21 from scope reconciliation:
the original witness clauses are now separated into supported scoped claims and
accepted residual edges. Supported at current evidence scope: runtime-owned
`update_id`; owner-visible work-state revisions before delegation; durable
mailbox cursor delivery; same-run passivation/resume for established Texture
documents; event-driven `update_coagent` delivery into the Texture thread;
deletion of the named prompt-bar first-paint, initial-super, and model-prior
guard scaffolds; and source-backed researcher evidence incorporation through
native inline chips plus the Sources panel. Accepted residual edges: proof is
not universal over every possible Texture document entry path; always-deep means
obligation/prompt-driven probe-and-incorporate loops with staged/source-panel
receipts, not exhaustive research for all prompts; and product acceptance remains
`staging-smoke-level` / `blocked`, not promotion or continuation acceptance.

Prior actual Delta V=-1 on 2026-06-21 from repairing the local same-thread
researcher verifier drift. `TestTextureCreatedResearcherEvidenceWakesTextureV2`
now opens researcher through the ordinary post-write `spawn_agent` path instead
of waiting for the deleted `model_prior_interim` completion-guard reminder, and
`TestTextureCurrentEventsPromptCanOpenProbePathWithoutCompletionGuard` no longer
asserts that an async researcher provider turn has appended a third observation
before the Texture path under test has settled. The focused same-thread,
mailbox, passivation, and stream verifier set now passes locally.

The latest durable product record is
RunAcceptanceRecord `runacc-21e9c87d45c3965bba1d` for the `_005` Comet proof.
The record was created on 2026-06-21 and is `staging-smoke-level` and
`blocked`, with
`authority_profile` `texture > conductor > researcher`, trajectory
`c3e06265-48a7-4f00-91d1-068c3706ff58`, document/Texture id
`33ac2a66-6c63-4bf6-8d8a-e0f965133a5b`, CI run `27897682907`, deploy job
`27897682907:82552403986`, and deployment commit
`fce827ca2a43994d1d67312f33fe4fef1d97f4d3`. It is blocked honestly because this
Texture/researcher source-panel proof did not exercise super/worker adoption or
continuation checkpoints.

Prior actual Delta V=-1 on 2026-06-21 from creating durable
RunAcceptanceRecord `runacc-21e9c87d45c3965bba1d` for the `_005` Comet proof.

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
  deployed commit, CI/deploy status, rollback ref, residual risks;
- run acceptance records at the honest level reached, without upgrading blocked
  source-panel smoke proof into promotion or continuation acceptance.

heresy delta: repairs H024a (trivial first patch as hidden work-state) and H024b
(model-invented coagent update ids); should also repair run-centric cold rewarm
and classifier/guard residues named in the v0 cautionary mission.

position / live conjectures / open edges:
- C1/C5/C6 supported: runtime-owned `update_id`, direct work-state revision,
  and durable mailbox cursor are implemented at focused/runtime scope.
- C7/C8/C9 supported: established Texture documents consume addressed
  `update_coagent` backlog through durable run-memory turns, idle as sleeping
  actors, and resume the same `loop_id`; staged semantic delegation with
  researcher return proved V2 on the same Texture loop. The local
  researcher-return verifier has been repaired to use the post-guard ordinary
  `spawn_agent` path.
- C10/C11/C12 supported: first-paint exact `patch_texture`, deterministic
  initial-super parsing, and Texture-specific model-prior completion guard /
  world-knowledge scanner are deleted; staged proofs show Texture acts first
  with normal tools and can choose researcher without prompt classifiers.
- C13/C14 partially supported but reopened: typed source refs and saved evidence
  can reach Texture source entities, and Texture idle passivation emits
  document-correlated `synth_completed`; the `_005` Comet proof showed
  source-backed v2, native source chip, idle toolbar settlement, enabled
  `Sources`, and one represented GPT-5.5 source artifact. However the same
  session also produced reader-visible markdown links and "Source handles" prose
  in a source-backed document. That is a regression against Texture source
  entities/transclusions, not a weaker form of citation success.
- C2/C3/C4 are reconciled as scoped support plus named residuals: mailbox and
  resume are supported for established Texture revision threads and prompt-bar
  source-evidence slices; always-deep is supported as prompt/obligation-driven
  repeated probe incorporation, not as a universal guarantee of exhaustive
  research depth. These residuals do not currently demand runtime construction.

Receipts for prior steps live in
`docs/mission-texture-durable-thread-v1.ledger.md`; this state intentionally no
longer mirrors the pass history.

next move: repair the native-citation regression without special-casing a
"source this" prompt. First prove the current write path can reject or normalize
source-backed revisions that substitute ordinary URL links / "Source handles"
sections for native source entities/transclusion refs; then run focused tests
and staging proof in Comet.

ledger file: docs/mission-texture-durable-thread-v1.ledger.md

version / lineage: v1 supersedes `docs/archive/mission-texture-long-running-agent-v0.md`
as the source program. v0 and its ledger remain cautionary evidence only.
Depends on `docs/mission-texture-versioned-artifact-v0.md` for provenance,
source validation, hash chain, and full-history publication substrate.

learning state: retain old failure history as a warning against matching the
wrong pattern. Promote only compact lessons: avoid run-centric reconstruction,
avoid role choreography, avoid model-invented delivery identity, and keep
owner-visible work state in Texture.

settlement: not settled. Independent prover review accepted the scoped support
claim, but the native-citation regression is open. The `_005` source-panel proof
is strong staging smoke evidence, not promotion-level or continuation-level
acceptance.

## Suggested Goal String

```text
Use Parallax on docs/mission-texture-durable-thread-v1.md. Treat it as the source program; do not use docs/mission-texture-long-running-agent-v0.ledger.md except as cautionary evidence for specific claims. Current status working, exit V=1: repair the reopened native-citation regression where source-backed Texture revisions can render ordinary markdown/URL links or "Source handles" prose instead of Texture `source_entities` and transclusion refs. Independent prover thread `019eea0f-4872-7882-a1ea-f8e356ff40d9` accepted the scoped support claim but not broader settlement. Automated checker evidence is recorded: CI run `27898525827` passed for `4852cca612e817567f3ee57349d7a2a6504982da`, FlakeHub run `27898525830` passed, Docs Truth Check `27898903344` passed for checkpoint `d550bee3ffaa56383a846de9d9cda4d76af0f68f`. Latest product proof is `CHOIR_NATIVE_SOURCE_ENTITY_PROOF_20260621_005` on deployed commit `fce827ca2a43994d1d67312f33fe4fef1d97f4d3`, trajectory `c3e06265-48a7-4f00-91d1-068c3706ff58`, Texture/doc id `33ac2a66-6c63-4bf6-8d8a-e0f965133a5b`, and RunAcceptanceRecord `runacc-21e9c87d45c3965bba1d` (`staging-smoke-level`, blocked honestly because no super/worker/adoption/continuation checkpoints were in scope). Mutation class red for runtime changes; protected surfaces remain Texture canonical writes, source/citation metadata, update_coagent persistence/delivery, run lifecycle, passivation, prompt defaults, Trace/evidence, and product acceptance.
```
