# Supervision Is Agent Work

**Last updated:** 2026-08-05. **Scope:** current design boundary, corrected by
the owner after the Texture tape cleanup.

Supervision is not a separate Choir subsystem, service, actor class, causal log,
or workflow engine. It is what the existing agents do across authority
boundaries:

- the owner supervises the computer through owner-readable Texture state;
- Texture supervises meaning, intent, evidence incorporation, and the standing
  artifact;
- Super supervises execution, decomposition, verification, and return of
  results;
- CoSuper performs a capability-bounded execution or verification assignment;
- Researcher returns sourced evidence for Texture or Super to judge.

The durable-work kernel, typed updates, trajectories, Trace, work items,
settlement queries, and canonical computer events support that work. They are
substrate and evidence, not another supervisor.

## Current Code State

The role topology exists in current source:

- `texture` owns canonical document versions and may write, wait, ask
  Researcher, or record a blocker.
- `super` is a persistent per-owner orchestration actor. It consumes addressed
  `execution_request` packets and may delegate to Researcher or CoSuper.
- `co-super` is a durable child role. Implementation and verifier slots are
  separately bounded; effects use guest-local capsule broker tools rather than
  direct host mutation.
- typed `update_coagent` packets return evidence, actions, questions, and
  results through durable mailboxes.
- significant Texture mutations append compact private audit evidence through
  the canonical `ComputerEventAppender`. That audit is evidence, not a second
  semantic state authority.

The complete product loop is **not currently accepted live behavior**.
Self-development effects remain OFF, Texture's effects-OFF tool registry does
not expose `request_super_execution`, and no current executable Definition
authorizes capsule, updater, checkpoint, or route effects. Super and CoSuper
code, policies, tests, and durable controllers therefore exist ahead of a
deployed end-to-end Texture-to-Super acceptance proof.

## Authority Boundary

| Actor | Supervises | Owns | Must not do |
|---|---|---|---|
| Owner | intention and acceptance | final authority | be bypassed by hidden promotion |
| Texture | semantic state and owner-readable context | canonical document versions | execute privileged effects or treat delegation as mandatory |
| Super | execution and verification work | execution plan and addressed obligations | directly mutate canonical Texture or host state |
| CoSuper | one scoped implementation or verification assignment | returned bundle/evidence for that assignment | acquire authority from prompt text or mutate the host directly |
| Researcher | evidence gathering within a request | sourced evidence packet | write canonical Texture state |

Single-writer ownership is per semantic object. Supervision does not require a
generic `supervision_object`, findings database, observer hierarchy, or second
settlement authority.

## The Intended Loop

```text
owner input
  -> Conductor opens or routes to Texture
  -> Texture writes the standing understanding
  -> Texture decides whether execution is needed
  -> explicit Texture request wakes persistent Super
  -> Super decomposes and, when needed, assigns scoped CoSuper work
  -> CoSuper returns bundle/evidence; independent verification remains separate
  -> Super returns the grounded result and unresolved decisions to Texture
  -> Texture incorporates what matters into a new owner-readable revision
  -> owner accepts, redirects, or stops
```

Every arrow should reuse the existing durable-run, typed-update, artifact,
capsule, and canonical-event substrates. A missing arrow is a product-path gap,
not justification for a new supervision platform.

## Current Gaps

1. Effects-OFF deliberately breaks the explicit Texture-to-Super execution
   arrow in the deployed product.
2. Existing Super/CoSuper and capsule contracts have strong local coverage but
   no current owner-ratified, deployed end-to-end product acceptance.
3. Documentation previously described a separate trajectory-supervisor and
   meta-supervision layer. That was an ontology error and is superseded here.
4. The product still needs a concise owner-visible representation of pending
   execution, returned evidence, unresolved decisions, and settlement inside
   ordinary Texture state.

## Path Forward

The next executable Definition should prove one narrow, real supervised task,
not build a generic supervision protocol:

1. Start from a canonical Texture revision containing the owner's intent.
2. Expose one explicit, authenticated Texture-to-Super request in the
   authorized effects mode.
3. Reuse the persistent Super inbox and durable run.
4. If the task needs effects, use one implementation CoSuper and one independent
   verifier CoSuper in disposable guest-local capsules.
5. Return typed evidence and result handles to Super, then to Texture.
6. Make Texture revise the owner-readable artifact with result, evidence,
   blocker, or decision.
7. Prove restart durability, cancellation, idempotency, no direct host mutation,
   exact staging identity, and owner acceptance.

Only after that path works should obsolete branches or redundant state be
deleted. Do not add a generic observation schema, findings reducer, supervision
API/CLI, deployment write-mode controller, or parallel causal tape.

## Success Criterion

Supervision works when the owner can inspect Texture and understand:

- what the computer believes it is doing;
- what Super delegated and why;
- what evidence or artifact came back;
- what remains blocked or undecided;
- what changed after the owner's correction.

That is an agent-product outcome. More supervisory infrastructure is not proof
of more supervision.
