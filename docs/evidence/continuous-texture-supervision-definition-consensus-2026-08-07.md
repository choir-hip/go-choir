# Continuous Texture Supervision Definition Consensus

- **Date:** 2026-08-07
- **Base:** `f64784e88b42abbf7d87fee058c989537b686d58`
- **Frozen candidate diff SHA-256:** `880c00cb8f7a6c546825689a7ba37c02a551f897a93080f5e1e2dbbda2cb89ed`
- **Candidate:** `docs/definitions/choir-continuous-texture-supervision-draft-2026-08-07.md` plus its three registry entries
- **Mode:** convergent; deletion-first, architecture, security, product, operability, history, and maintainability lenses
- **Disposition:** `REPAIR`

## Panel receipt

Nine routes were attempted through the repository `agentic-consensus` runner.
Eight completed: Codex, Devin, Claude, Cursor, OpenCode, OMP GPT-5.6 Sol, OMP
Gemini 3.6 Flash, and OMP DeepSeek v4 Flash. OMP Cursor Grok 4.5 failed before
review because its provider API key was absent. Six completed routes returned
`REPAIR`; two returned `ACCEPT`. The repair majority was accepted because its
blocking findings reproduced current code behavior and changed authority,
transaction, effects, or acceptance semantics. The two accept votes verified
the same substrate direction but treated non-executable status as sufficient to
defer those decisions.

Claude was included explicitly and returned `REPAIR` with high confidence.

## Consensus that survived

1. The existing lifecycle trajectory/work/update substrate is the correct single
   authority. Do not add a mailbox, second tape, workflow engine, or dual write.
2. Spawn and address authority remain separate. Texture may spawn Researcher
   only; it may never spawn Super/CoSuper or address CoSuper directly.
3. Opening work for the exact persistent `super:<owner>` is authority creation;
   continuing already-bound Researcher/Super work is a different operation.
4. The old same-channel Super request stays deleted. The new path uses typed,
   replayable lifecycle state and exact owner/computer/document/trajectory/work
   identity.
5. Persistent Super keeps its one owner/computer identity and controller. It is
   an addressed target/assignee in a lifecycle trajectory, not promoted into a
   generic lifecycle actor class.
6. A Texture revision-or-disposition and its outbound controls need one
   conditional object-graph commit. Tool sequencing is not atomic.
7. Capsule-local CoSuper work may remain in this mission only behind a frozen,
   guest-local, evidence-only boundary that cannot advance self-development,
   event, materialization, checkpoint, route, or host state.
8. Red mutation ceremony, non-executable registry status, problem-documentation-
   first ordering, source rollback, and exact staging identity are correct.

## Blocking findings and adjudication

### Direction-specific work authority

Current `QueueLifecycleUpdate` interprets `work_item_id` as work assigned to the
producer. A downward control instead needs an open work item assigned to the
target. One ambiguous field or a producer-or-target rule would become a generic
mailbox and could let one actor settle another actor's work.

**Adjudication:** preserve the existing producer-report command. Add a distinct
lifecycle control command in the same reducer/object/event/snapshot authority.
The reducer derives direction from caller, target, packet kind, and bindings;
the model does not author a direction flag.

- Upward report: runtime-derived `producer_work_item_id`; producer owns any work
  disposition; target is its requesting Texture/Super.
- Downward continuation: runtime-derived `target_work_item_id`; exact target owns
  the open obligation; sender cannot settle it.
- Persistent-Super opener: atomically creates/reuses target work only for exact
  `super:<owner>` on the current computer and queues only validated
  `execution_request`.

### Atomic Texture transition

The existing generic `update_coagent` enqueues immediately. Texture patch and
rewrite are activation-terminal. The current Texture lifecycle write also queues
then applies in two transactions. Calling tools in either order can produce a
directive without its committed semantic state or a revision without its
redirection.

**Adjudication:** do not expose the current immediate generic implementation to
Texture. Delete the synthetic self-queue-before-apply convention. Extend the
existing conditional lifecycle apply boundary, or add one narrow Texture apply
command, with:

- expected lifecycle version and document head;
- exactly one semantic outcome: revision, explicit no-change disposition,
  wait, or block;
- inbound dispositions;
- zero or more bound Researcher continuations;
- optionally one persistent-Super opener/control;
- deterministic command, ordered-control, target-work, and packet identities.

`patch_texture`, `rewrite_texture`, the Texture form of `update_coagent`, and the
typed Super opener are affordance/validation views over this one commit. None
reports durable success or wakes a target independently. Wake occurs only after
the batch commits; restart sweep recovers a committed-but-not-woken packet.

### Fail-closed and staged enablement

The current target lookup can fail open because authority checks run only when
lookup succeeds, and a non-lifecycle Texture could otherwise fall back to the
legacy mailbox. The generic resolver also self-targets a Texture channel before
honoring explicit `agent_id`. Researcher-to-arbitrary-Texture cross-document
injection is not mechanically excluded.

**Adjudication:** every target lookup error is refusal; no channel/self fallback
is authority. Texture control refuses non-lifecycle activation. The Texture tool
is absent until the reducer, exact target/work validation, and single-authority
reader are present in the same candidate. Persistent-Super addressing remains
disabled until its lifecycle control path lands. Cross-document, foreign owner,
foreign computer, foreign trajectory, arbitrary Super, and CoSuper targets are
explicit negative contracts.

### Owner correction semantics

A direct owner revision is already canonical; a natural-language `/revise`
request asks Texture to author a revision. Treating both as mailbox requests
would subordinate owner state to provider availability. Committing a direct head
and separately waking can lose the correction edge.

**Adjudication:** preserve two semantics under one lifecycle wake contract.
Direct owner revisions remain immediate `AuthorUser` head transitions and
atomically record a head-advanced correction cursor/obligation. `/revise`
queues a lifecycle decision request for Texture. Editor burst coalescing may
coalesce wakes against the latest committed head, never canonical owner edits.

### Capsule and verifier boundary

The broad lifecycle-profile refusal is not an effects flag. The current verifier
`record_self_development_verification` tool appends canonical computer events,
mutates updater-root files, proposes an effect, and advances an operation toward
approval. It is not admissible evidence-only verification for this mission.
Network isolation is also load-bearing, not a model-declared action property.

**Adjudication:** keep scoped CoSuper work because it is part of the requested
Super decomposition proof, but freeze this narrower path:

- exact persistent Super caller and same owner/computer/trajectory/open target
  work;
- at most one implementation and one verifier slot;
- one pre-existing disposable capsule with `CLONE_NEWNET`/networkless isolation
  and a run-bound opaque capability;
- implementation may exec/read/write only there and freeze an inert bundle;
- verifier receives immutable read-only bundle/evidence access and returns a
  typed evidence packet; it has no mutation handle and no
  `record_self_development_verification`;
- no ComputerEventAppender, updater-root mutation, effect proposal, acceptance,
  materialization, checkpoint, route, VM, host path, or owner decision.

If that boundary cannot be isolated from current self-development finalization,
the implementation stops and the capsule slice moves to a separately ratified
successor; the Definition may not weaken its artifact silently.

### Deployed acceptance

The first candidate did not explicitly perform deployed process restart, owner
correction, an actual post-cancel late result, or before/after no-effect checks.

**Adjudication:** one authenticated staging trajectory must leave Researcher and
Super packets pending, passivate actors, restart the same build through a
no-SSH product/deployment operation, and cold-reconstruct the same identities.
It must apply a real owner correction and prove changed downstream direction,
cancel in-flight Super/CoSuper work, deliver a real late result and exact retry,
and prove no reopen/wake/state mutation. It records exact owner/computer/document/
trajectory/work/actor/update/digest identities and compares canonical event
heads, self-development operation, materialization, checkpoint, route, and host
projections before/after.

### Candidate integrity

The frozen candidate recorded the primary worktree as clean and left its digest
pending even though the review substrate was dirty-but-owned and externally
hashed.

**Adjudication:** the repaired Definition carries a dated start correction, the
reviewed digest, this durable receipt, and a new repaired-candidate digest. It
remains non-executable pending owner ratification against that repaired digest.

## Panel output identities

- Claude: `7196b380295724204f4d69b32497fe4a58a00e975c186713f2e2ecd44a8d8199`
- Codex: `62629a33c91b2599e4abb0125ee7bad4714bcd239aa7ee8cce2773490f8a9d67`
- Cursor: `4ffb50d26707f5fa841c260314b56b993424b61ced13a74616305f884b63f708`
- Devin: `342dd19ed44ca4196f7304cc137db676f1f5876ec02e8eed5a79656e8e91c0af`
- OpenCode: `28705b2a6f81046b42ac93a0891f417cc963ffb14c53f380716e06bede387bae`
- OMP GPT-5.6 Sol: `40e1e52ff5326ad167eb7e5c9f5a31b70addc489850f7e46d54c2fd3b06ebc53`
- OMP Gemini 3.6 Flash: `175adead20e39f10b0ea244e48b12bdcb5f20b3ec418cf21e93d973627d9aede`
- OMP DeepSeek v4 Flash: `9e6664c224bb2fcd4b4304319b7385a0ee6ea6339ce4661e5d127f60ea1f196b`
- failed OMP Cursor Grok route: `a56654b11f965aecaa3719dea6effed37ae9a8ea2fe2627224a4e582993af39d`
- runner manifest: `72a27f306590daee7b507afb95a8906e5be2def3b85a71b300424ce29ddc3633`
- frozen prompt: `ce14a12959640222378d47598881fac5b8802a62df77480bca5419a9a269e9ab`
