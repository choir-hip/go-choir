# Texture Agentic Invariants - 2026-06-13

## Status

Doctrine and guardrail document. This records the working Texture semantics after
the M3 regression review, where an acceptance probe accidentally turned Texture
researcher delegation into a forced runtime workflow.

This document inherits [choir-doctrine.md](./choir-doctrine.md). Choir Doctrine
is the apex; this file specializes it for Texture. If this document is used to
justify hidden workflow forcing, first update the doctrine-level conjecture and
protected-surface evidence packet.

Texture is currently the most fragile core of Choir. It is also the key dependency
for almost every higher-level product path: owner-readable mission state,
canonical documents, appagent coordination, source-to-story work, publication,
promotion review, and long-running self-development all rely on Texture behaving
as the versioned artifact control plane rather than a workflow runner.

Read this before changing Texture tools, prompts, routing, revision creation,
coagent wake behavior, trace/Texture projection, run acceptance, or any mission
that uses Texture as its owner-readable narrative.

## Core Invariant

> Texture owns canonical document and artifact state inside a multi-agent system.
> It is not a workflow engine, a route script, or a role-sequence executor.

Texture may write, revise, wait, ask researcher, ask super, ask both, ask neither,
request clarification, or report a blocker. The correct choice is part of the
Texture agent's obligation and authority envelope. Runtime may expose tools,
durable evidence, pending work, and policy constraints; runtime must not force a
semantic delegation merely because text or metadata mentions another role.

Conductor routes exogenous input into Texture-owned artifact state. Prompt-bar
requests, sourcecycled/news ingestion, article creation, mission work, and most
user prompts should open or create Texture/context first. Texture then decides
whether to write/revise, attach or transclude sources, ask researcher, request
super execution, coordinate coding-agent trees through super, wait, or record an
off-document decision/blocker. Super is downstream execution authority, not the
ordinary ingress target for user or source prompts.

## Non-Negotiable Rules

1. **Canonical text is Texture-owned.** User revisions and Texture appagent
   revisions are canonical document versions. Researcher findings, super
   updates, trace moments, search results, and worker evidence are inputs to
   Texture, not canonical text until Texture incorporates them into a revision.

   Prompt-bar creation is not an exception. The owner's submitted prompt is the
   canonical `V0` Texture revision. It must not be moved into hidden metadata,
   a separate prompt band, or any other product chrome while `V0` remains blank.
   Metadata such as `seed_prompt` may preserve provenance, but it must not be
   the product display mechanism or the agent's substitute for the canonical
   starting version.

2. **Texture is the control plane for document and artifact work.** Conductor may
   classify exogenous input and create or open the target Texture/context, but it
   must not send ordinary prompt-bar, sourcecycled/news, article, mission, or
   document/artifact work directly to super based on prompt text. Product-path
   proof should show conductor entry, then Texture artifact materialization.
   `super` before Texture is a route invariant failure; `super` after Texture is
   valid only when Texture requested it.

3. **Delegation is agentic.** Texture decides whether to call `spawn_agent`,
   `request_super_execution`, both, neither, or a future coordination tool. A
   prompt saying "researcher" is evidence about owner intent; it is not a hard
   runtime command to spawn a researcher.

4. **No semantic forced continuations from `edit_texture`.** `edit_texture` stores a
   document revision. It must not require a subsequent researcher, super,
   verifier, or other semantic appagent call. Deterministic app protocol
   handoffs, such as persisting an email draft for owner approval, must be
   explicit, narrow, and documented separately.

   Texture write tools must also not become premature run terminators. A
   successful Texture write stores a canonical revision; the same logical
   `texture:<doc_id>` actor may store later canonical revisions in the same
   physical run as new evidence or owner direction arrives. The verifier and
   Trace evidence must therefore support N:1 loop-to-revision causality instead
   of assuming one run equals one write. A write should not prevent the same
   Texture run from making the next legitimate coagent decision, such as opening
   researcher work, requesting super execution, recording an off-document
   decision, requesting an email handoff, parking for later updates, or ending
   intentionally.

5. **Required tool choice is not policy.** Exact next-tool enforcement is
   allowed only for mechanical tool protocols whose second call is part of the
   same protocol state, for example a worker allocation followed by a
   worker-start handshake. It must not be used to steer Texture's semantic
   choices.

6. **Prompts and metadata do not replace agency.** Prompt flags, revision
   metadata, and route hints may inform Texture. They must not become hidden
   workflow edges. If the product needs explicit commands, create a visible
   command grammar and still record the resulting obligation as state Texture can
   settle honestly.

   Durable metadata forcing is an explicit violation: persisted flags such as
   `explicit_researcher_request`, base-revision content scans, or carried
   request-intent fields must not re-derive a required researcher/super
   delegation across turns. Prompt-pipeline forcing is also a violation:
   prompts and revision builders may describe obligations and affordances, but
   must not mandate "call spawn_agent now" or similar semantic role sequences.

7. **Trace and Texture have different jobs.** Trace is the causal ledger for tool
   calls, LLM content, events, and agent messages. Texture is the owner-readable
   narrative and canonical document surface. Do not turn Texture into a Trace-like
   topology/status dump, and do not use Trace role sequences as a substitute for
   Texture semantics.

8. **Acceptance verifies outcomes, not role choreography.** A test may require
   researcher participation only when the product behavior under test is
   researcher participation. Lifecycle missions must verify lifecycle evidence:
   open obligations, passivation, rewarm, delivered updates, settlement, and no
   stranded work. They must not force a particular Texture delegation sequence as a
   proxy.

9. **Harness minimalism protects Texture.** Do not add Texture-specific branches to
   the core tool loop, provider loop, continuation machinery, or run acceptance
   unless there is a documented invariant, a simpler prompt/policy/tool
   alternative has been rejected, focused regression tests exist, and a human has
   explicitly approved the divergence.

10. **Structured edits are the default for long documents.** Whole-document
   rewrites are exceptional and require rationale. Texture must preserve document
   structure, provenance, revision history, and source/citation semantics.

11. **Texture regressions are architecture regressions.** Treat unexplained Texture
    failures as mission-level blockers. Do not patch around them with one-off
    workflow enforcement unless the invariant being protected is explicitly
    named and reviewed.

## Allowed Runtime Help

Runtime may:

- expose `spawn_agent`, `request_super_execution`, source tools, edit tools, and
  other affordances to Texture;
- preserve owner intent, source refs, revision metadata, and trajectory/work
  evidence durably;
- wake Texture from pending coagent updates or assigned work items;
- debounce/coalesce updates before waking Texture;
- surface pending obligations and missing evidence in prompts;
- prevent duplicate revision writes and protect owner approval boundaries;
- reject invalid edits or unsafe operations.

Runtime may not:

- convert a role mention into a forced next tool;
- route ordinary prompt-bar, source/news, article, mission, or artifact work
  directly to super before Texture has created or opened the controlling
  artifact context;
- require Texture to ask researcher/super/verifier after storing a revision;
- terminate Texture merely because a `patch_texture`/`rewrite_texture` call
  succeeded when unresolved coagent, decision, or handoff obligations remain;
- silently satisfy Texture obligations through another agent's route;
- mark exact internal role sequence as acceptance unless that sequence is the
  product requirement;
- hide role-specific control policy in generic tool-loop continuation code.

Texture tool inventory should match Texture authority. Researcher-owned
evidence gathering and provider/model diagnostics should not be bundled into
Texture simply because they share an implementation registry. Split memory,
evidence, and diagnostic affordances when needed instead of giving Texture a
large generic tool bag.

## Coagent update delivery (2026-06-17)

`update_coagent` is the sole agent-to-agent wake primitive. Delivery semantics are
uniform across Texture, super, researcher, vsuper, and co-super activations.

### Typed packets, not inferred routing

- Every delivered update becomes a **typed user turn** in the target activation's
  context window: a `coagent_update` JSON packet with `packet_type`,
  `delivery_phase` (`cold_activation`, `mid_activation`, `final_checkpoint`),
  and structured update records.
- **Warm activations** inject pending updates between tool-loop iterations.
- **Cold activations** prepend pending updates at activation start when the run
  was opened from an `update_coagent` wake (`request_source=update_coagent` or
  seeded `worker_update_ids` metadata).
- **Parked resident activations** wait without provider calls until runtime
  injects a typed update turn or an idle/budget boundary fires. Park-and-wait is
  a role-uniform tool-loop primitive, not a Texture-only semantic branch.
- **Rewarmed activations** resume the same logical actor from durable run memory
  after process refresh/passivation. Provider-call and token spend carry forward
  across the replacement activation; elapsed wall-clock budget across sleeps is
  still an explicit open edge until separately proven.
- Runtime must **not** traverse spawned-by / parent-run edges to decide who
  receives an update. Provenance fields (`RequestedByRunID`, `requested_by_run_id`)
  are audit-only.
- Deleting a Texture document cancels the addressed `texture:<doc_id>` actor and
  any pending Texture revision trajectory before removing the canonical document
  rows. Deletion must not leave a parked actor or pending mutation able to write
  a deleted document.

### One Texture coagent per article

- Each Texture document/article has a durable Texture coagent id:
  `texture:<doc_id>`.
- Researchers spawned for that article must address **that exact id** on every
  `update_coagent` call via the required `agent_id` argument.
- Spawn metadata (`requested_by_agent_id`, run-context overlay) names the
  delivery target so the researcher can copy it; runtime does not infer the
  target when the caller is a researcher.
- Super and other roles may still use explicit `agent_id` or documented
  non-researcher resolution paths; researchers may not omit `agent_id`.

### Texture wake path

- `wakeUpdatedCoagent` uses the same `reconcileUpdatedCoagentActor` entry path
  for all addressed agents, including `texture:<doc_id>`.
- Texture integrate runs (`integrate_worker_findings`) start when pending
  updates exist and no conflicting pending mutation blocks; worker content
  arrives through injected packets, not a separate channel-only prompt embed.
- Failed Texture integrate runs must **not** advance the worker-update
  checkpoint or mark updates delivered without a canonical revision.

### Required tests for this contract

- researcher `update_coagent` rejects missing or non-texture `agent_id`;
- typed packet builder and Texture warm/cold injection paths;
- coagent rewarm and resident-activation injection behavior;
- Texture wake after researcher delivery produces a revision when the model
  patches.
- document deletion cancels the pending or parked Texture actor before deleting
  the document;
- workflow verification accepts many appagent revisions from one Texture loop
  when each revision has write-tool evidence and valid parent causality.

## Problem: Texture integrate wake is blind on turn 1 (2026-06-17)

Observed on staging `5e17138f` during the deployed live-search eval: the
researcher web search succeeded and `update_coagent` reached `texture:<doc_id>`,
but the Texture document stayed at v0 with "Texture run completed without storing
a Texture revision" / "Revision failed". The activity log showed the Texture
integrate run hitting repeated tool errors and ending without a canonical
revision.

Root cause (logic, model-independent):

1. **First-turn blindness.** The integrate wake run is started by
   `reconcileTextureAgentWake` via `submitTextureAgentRevisionRun` with intent
   `integrate_worker_findings`. That run does **not** set
   `request_source=update_coagent` or seed `worker_update_ids`, so
   `shouldPrependInitialCoagentUpdates` is false and the cold packet prepend does
   not fire. The integrate prompt itself no longer embeds worker messages
   (delivery moved to injection). The model's **first** inference turn therefore
   has the document and diff but none of the grounded findings; the injector only
   splices them after the first tool round or at the `end_turn` checkpoint. A
   model that ends the first turn with prose can complete the run before the
   findings ever enter context.

2. **No "must act" constraint on integrate.** `initialTextureToolChoice` returns
   `required` only when `scheduled_message_seq == 0`; integrate wakes
   (`scheduled_message_seq > 0`) get no initial tool-choice constraint, so a
   grounded integrate turn may legally end with prose and produce no durable
   artifact, which surfaces as "Revision failed".

This is distinct from delivery accounting, which is correct:
`markTextureWorkerUpdatesDelivered` runs only inside a successful write commit
(`commitTextureToolEdit`), so a no-write integrate leaves updates pending and the
doc is re-woken rather than silently dropping the findings.

### Intended invariant

- A Texture integrate wake must place the pending `update_coagent` findings in
  the model's context on its **first** inference turn (cold prepend), matching
  the warm-injection contract.
- A grounded integrate turn must take a **durable action**: write
  (`patch_texture`/`rewrite_texture`), delegate (`spawn_agent`/
  `request_super_execution`), or record an explicit Texture decision
  (`record_texture_decision`). It must not silently end with prose. This keeps
  Texture agentic (it chooses which durable action) while banning the silent
  no-op that presents as "Revision failed".

### Required tests for this fix

- integrate wake run carries cold-prepend eligibility so turn 1 sees findings;
- `initialTextureToolChoice` requires a durable action on grounded integrate
  wakes;
- a no-write integrate still leaves worker updates pending for re-wake.

## Regression From M3

During M3, the deployed restart proof required Trace to show conductor, Texture,
researcher, and super before vmctl refresh. When researcher did not appear, the
mission drifted from durable-actor lifecycle proof into trying to force Texture to
spawn researcher. The final shape returned `next_required_tool=spawn_agent` from
`edit_texture` and relied on the generic tool loop to enforce exact `spawn_agent`.

That was a regression. It made a probe precondition the runtime semantics.

Correct recovery:

- remove hard researcher continuation from Texture;
- document this invariant in worker-facing docs;
- test that Texture is not forced by role mentions;
- redesign M3 acceptance around lifecycle evidence;
- keep researcher participation as a possible Texture choice, not a runtime
  workflow step.

## Required Tests For Future Changes

Any behavior-changing Texture coordination change should include tests proving:

- prompt-bar and source/article ingestion enter Texture-owned artifact state
  before any super execution;
- `edit_texture` does not emit semantic `next_required_tool` values;
- prompts mentioning researcher or super do not force a delegation;
- Texture still has access to researcher/super affordances and can choose them;
- researcher findings remain non-canonical until Texture incorporates them;
- public/product acceptance observes outcomes and obligations, not hidden role
  sequence;
- long-document revisions preserve structured-edit defaults and operation
  evidence.

Tests to invert or delete when M3.1 repairs H010/H024/H026:

- tests that expect `edit_texture` to emit `next_required_tool=spawn_agent`;
- tests that preserve researcher intent through durable revision metadata as a
  forced follow-up;
- tests that treat base-revision content mentioning researcher as a required
  delegation oracle;
- tests that require Texture's first tool to be `request_super_execution` because
  a prompt matched super keywords;
- prompt-default assertions that encode a fixed Texture -> researcher -> super
  role sequence instead of obligations and evidence.

## Protected Surface Rule

Texture canonical writes, revision metadata, prompt routing, coagent wake
behavior, Trace/Texture projection, and acceptance involving Texture are protected
surfaces under Choir Doctrine. Before changing them, name the mutation class,
conjecture delta, evidence class, rollback path, protected surface touched, and
heresy delta (`discovered`, `introduced`, `repaired`).

## Short Rule For Agents

If a proposed Texture change makes the sentence "Texture must call X next" true for
a semantic agent role, stop. You are probably turning the multi-agent system
into a workflow. Document the problem, shift the observer, and protect Texture's
agency before writing code.
