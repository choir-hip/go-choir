# Continuous Texture Supervision Definition Consensus

- **Date:** 2026-08-07
- **Base:** `f64784e88b42abbf7d87fee058c989537b686d58`
- **Frozen candidate diff SHA-256:** `880c00cb8f7a6c546825689a7ba37c02a551f897a93080f5e1e2dbbda2cb89ed`
- **Candidate:** `docs/definitions/choir-continuous-texture-supervision-2026-08-07.md`; originally reviewed under its `-draft` path, then owner-ratified and promoted with its three registry entries
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
pre-cutover worker inbox. The generic resolver also self-targets a Texture channel before
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

**Superseded and owner-ratified (candidate v4 through final Define
promotion, corrected 2026-08-08T02:45:35Z):** the owner-directed model replaces
the narrow v2–v3 adoption topology with many capability-bound CoSupers. Super
may coordinate multiple writable assignments in separate networkless capsules,
normally one writable assignment per capsule. Verification is not a read-only
role: a verification CoSuper receives its own writable isolated capsule and may
edit files, create test support, and run builds, tests, or scripts there.
It verifies an immutable subject digest; changing subject bytes produces a new
candidate identity rather than a verdict on the original. At least one such
independently identified verification result is required before completion.
The earlier "at most one implementation and one verifier slot; one pre-existing
disposable capsule" and immutable-read-only-verifier sentences are retained
only as historical evidence of the superseded v2–v3 candidate and have no
executable authority. The surviving boundary remains load-bearing: assigned
networkless capsule isolation and no ComputerEventAppender, updater-root,
finalization, materialization, checkpoint, route, VM, host, or owner-decision
effect.

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

## Repaired-candidate acceptance review

- **Reviewed at:** 2026-08-07T20:49:40Z
- **Reviewed source commit:** `6681606d4e0f14e83dab89bb808862db82cdd21b`
- **Reviewed self-normalized digest:** `885cf4acb7f236b496df6adc41bd4a403266b08487488c9be261472d22188653`
- **Panel:** the same nine routes and lenses, including Claude
- **Result:** five `REPAIR`, three `ACCEPT`, one route failed before review

All eight completed reviewers found the six substantive architecture, authority,
atomicity, capsule, and deployed-proof blockers materially repaired. Claude,
Codex, Cursor, and OMP GPT-5.6 Sol independently reproduced one remaining
identity defect: the digest prose did not specify the path bytes and NUL
separators used by the recorded digest. Devin also returned `REPAIR`, but its
claim that the evidence file was absent from candidate scope is contradicted by
the reviewed Definition's explicit scope list and is dismissed.

**Adjudication:** accept the reviewed architecture and apply the minority
identity blocker. Candidate v3 specifies the exact byte-level self-normalized
algorithm and is re-frozen without changing runtime design or authority. This
receipt remains evidence, not owner ratification or execution authority.

### Repaired-candidate output identities

- Claude: `74a8ba8eb1987ea45548be170e162edbeddcc50152c14abb8264db73a62d1044`
- Codex: `a54ed67e414fca714bd8e58c6716cb74339809a604e4b6e2efb3eaa7cacc6d68`
- Cursor: `869cef99a3606c9a82e8f54e1e426aa44fcd68da34a03a618c30390b3bd46dde`
- Devin: `a5c9560da638660fc4a8817678e975cfcb91e489789a614f772bb2d047c83528`
- OpenCode: `440aa7e8eadedac22a123bb1a756db2a0decbf1a8ca39d543337bd13556f48a1`
- OMP Gemini 3.6 Flash: `43ddd0010aade48e42a77619c6dd9c523f3ff9804ecfc5bff76c327b3ab6a57a`
- OMP GPT-5.6 Sol: `3813f19c0d63811d85d63d219bc8c86671542a524df2904f3e0b20187ad96d04`
- OMP DeepSeek v4 Flash: `0a4070e9bc4c30994de21e906a2551f36407d5c4d78ef1a129c7063e48a3182c`
- failed OMP Cursor Grok route: `a56654b11f965aecaa3719dea6effed37ae9a8ea2fe2627224a4e582993af39d`
- runner manifest: `be448da9a2be8a8a26eba5415d0842d078c492011b2d6f57205a8e8ee472e3e8`
- frozen prompt: `c425e096dd59bf6b25a413a48c5ad226bbd7fe92c0ff6e75be2d9648cd25bb2f`


## Candidate v4 panel gate — 2026-08-07 (independent review of the full v4 candidate)

- **Reviewed at:** 2026-08-07T22:33:00Z
- **Reviewed head:** `6681606d`-lineage working tree at HEAD `b20bece30b408373d2844f5621fb9f91fc624d99`; the Definition file carries the v4 candidate and is the only dirty path.
- **Reviewed self-normalized digest:** `9db1c4397646142c28d1f85580ec91099a22c6340e20dd3740c36d4419373018`
- **Panel:** nine routes including Claude. Eight completed reviews; Devin returned no verdict (empty output) and the OMP Cursor Grok route failed before review on a missing provider API key, matching the prior panels' route-failure pattern.
- **Result:** six `REPAIR`, one `ACCEPT` (OMP Gemini 3.6 Flash), two routes did not deliver a review.

Every completed reviewer reproduced the exact frozen digest, confirmed the three
registries consistently list the candidate as blocked/draft_non_executable with
no dangling references, and found no dissent from the underlying single-authority
architecture (preserve producer-report `QueueLifecycleUpdate`, add an exact
direction-specific control command with `target_work_item_id`, one conditional
object-graph batch, persistent Super as an exact lifecycle target, Texture
spawn-Researcher-only, no immediate generic `update_coagent`, networkless
capability-bound capsules).

### Blocking findings (unanimous or near-unanimous)

1. **Read-only verification governed both as required and optional.** The
   accepted trajectory committed to a read-only verification result in
   `finish.artifact`, while deployed acceptance and the Land step called
   read-only CoSuper review optional, and the completion evidence floor did not
   list it at all. Six completed reviewers (Claude, Codex, Cursor, OMP GPT-5.6
   Sol, OMP DeepSeek v4 Flash, OpenCode) independently classified this as the
   stopping condition being readable two ways.
2. **The prior consensus receipt still freezes the superseded one-slot capsule
   cardinality.** The earlier adjudication here recorded "at most one
   implementation and one verifier slot; one pre-existing disposable capsule"
   as a binding repair. Candidate v4 inverts that to many capability-bound
   CoSupers, at least two parallel networkless writable assignments in separate
   capsules, and a verifier that is a capability shape rather than a singleton
   trajectory role. Both files are inside the frozen scope, so an implementer
   reads the opposite red-surface capsule contract from the one the Definition
   requires (Claude, reinforced by Codex and OMP GPT-5.6 Sol risk lists).
3. **Transclusion could be read as a second document-truth.** Implement C.4's
   "add one generic expanded source-transclusion block" wording can be read as
   introducing a second structured-document node type (Codex, OMP GPT-5.6 Sol),
   conflicting with the doctrine invariant (I15) that every source citation
   stays one `source_ref` node and expanded transclusion is a deterministic
   projection.
4. **Deletion-citer disposition is missing from scope.** The Definition removes
   the historical same-channel `request_super_execution` but does not itself
   disposition the canonical citers that still teach it: the conjecture /
   assertion ledger A1 (routes to the persistent Super) and the
   `super_requested` checkpoint in `internal/agentcore/run_acceptance.go:164`
   which keys on that retired tool name. This violates standing-question
   deletion-citer discipline (OMP GPT-5.6 Sol, reinforced by Codex).
5. **Restart-operation identity is unspecified.** Acceptance requires an
   "approved no-SSH same-build staging process restart" but never names the
   operation, so the deployed proof can become ad hoc (Claude, Codex,
   OMP GPT-5.6 Sol, Cursor).
6. **Stale `now.reconciliation` identity.** The candidate card referenced the
   older base `f64784e8…` inventory describing five goal-owned dirty docs;
   actual HEAD is `b20bece3…` with only the Definition dirty (OMP GPT-5.6 Sol).

### Adjudication into candidate v4.1

- Read-only verification becomes one required acceptance evidence item
  (acceptance action 3, Land step 4, completion evidence floor) and the
  artifact sentence is aligned.
- This evidence file's one-slot/one-capsule freeze is explicitly superseded in
  the paragraph below by the owner-directed v4 model; the older sentence remains
  only as a historical report.
- Implement C transclusion wording will state that expanded transclusion
  reuses the one `source_ref` node with deterministic projections and never a
  second independent truth.
- A deletion-citer slice is added: assertion ledger will be repaired to describe
  the removed tool, `run_acceptance.go`'s dead checkpoint is dispositioned, and
  fixtures are re-pointed at the new direction-specific lifecycle opener.
- The restart operation is pinned to the re-deploy of the same SHA through the
  Git repository's `workflow_dispatch` staging deploy (the existing `force
  _staging_deploy` input) and `/health` identity check.
- The registry/state card is updated to the real HEAD and dirty-path inventory,
  and the digest is re-freezed for candidate v4.1 (this file is part of the
  candidate scope, so this receipt participates in the re-freeze).

### Candidate v4.1 re-panel gate — 2026-08-07 (green-gate identity re-review)

- **Reviewed at:** 2026-08-07T22:33:00Z
- **Reviewed candidate:** `continuous-texture-supervision-definition-v4.1` (working tree), self-normalized digest `4a88d3637c657279370713308db7b7636835da05cb55d68ee1806cf0ef5c9727` as frozen; snapshot `.agentic-consensus/cts-v4.1-panel/snapshot/`
- **Panel:** nine routes including Claude; eight completed, Devin returned no verdict, OMP Cursor Grok-45 failed on missing API key (prior pattern)
- **Result:** six `REPAIR`, one `ACCEPT` (OpenCode), two routes non-delivering

### v4.1 blocking findings

1. **Definition front matter no longer parses under repo tooling.** `node skills/definition/scripts/dashboard.mjs` fails (`indentation must use two-space levels`, and unquoted colon scalars at line 75/108-110/258). Any pre-promotion candidate must load in `parseDefinition`/`parseYamlSubset` (Gemini, Codex, Cursor).
2. **B3 wording still not explicit enough.** Implement C.4 still said "add one generic expanded source-transclusion block"; reviewers flagged that the single `source_ref` + `display_mode: expanded_ref` contract promised in the adjudication is not stated in the Definition body at I15 granularity (Claude, Codex, Cursor, GPT-5.6-Sol).
3. **Identity block still mislabeled/duplicated.** The v4.1 re-panel receipts carried the v4 gate's output hashes mislabeled as v4.1, plus a second corrupted hash list (OpenCode, DeepSeek, Claude, GPT-5.6-Sol).
4. **`next_action` still named candidate v4** (stale identity in the ratification gate field) (Claude, Cursor, GPT-5.6-Sol, DeepSeek).
5. **Reconciliation inventory wording** ("only the goal-owned Definition dirty") contradicted the live four-file dirty state at freeze time (Claude, Codex).

Adjudication: all five repaired in candidate v4.2 below. The Definition now quotes every scalar the parser rejects, states the C.4/source_ref contract in the artifact (Implement C.4), names candidate v4.2 and its digest in `next_action`, carries a single correctly labeled identity block (this section, computed over the actual v4.1 panel outputs), and corrects the reconciliation inventory.

### Candidate v4.2 identity re-panel gate — 2026-08-07 (receipt mechanics)

- **Reviewed candidate:** `continuous-texture-supervision-definition-v4.2`, digest `9e6cd9222c7585cd1f64e44fd6ff2842413e4849b3446a95a3799b350c5c16a2`
- **Panel:** nine routes including Claude; seven completed, OMP DeepSeek timed out at 600 s, OMP Cursor Grok-45 failed on missing API key
- **Result:** REPAIR on receipt integrity only. All delivering reviewers confirmed the v4.1 blockers resolved (dashboard parser passes, C.4/source_ref contract explicit, pre-cutover mailbox wording used throughout, `next_action` names v4.2, reconciliation inventory correct) but found: (a) the v4.1 identity block listed six hashes that do not match the raw bytes of `/cts-v4.1-panel` outputs because they were recorded over ANSI-stripped text; (b) a v4 gate output-identity block was missing entirely; (c) Chronology of receipts and next_action digest-hex strictness flagged as non-blocking.
- **Adjudication into candidate v4.3:** identity blocks are rebuilt from raw bytes (v4 gate + v4.1 re-panel); the missing v4 gate block is now present; receipts remain chronological. Definition, evidence narrative, and registry wording unchanged in substance; the 5-file candidate digest is re-frozen after this identity edit.

### Candidate v4 gate output identities (raw bytes)

- Claude: `193c4968bf90e8df777da7537aec72ba19c349dc3b8d0752de625bd818f1d870`
- Codex: `096d9aa5b22dad6a5bada9295ffdfd2ba5f5f4f6b0f881b2b03969b2be84988f`
- Cursor: `917078c4992cce9bebf2778a379581916693129e292c1092195a99857d5c3625`
- Devin: `1fe863fce9f34273da1b5263056d837f13a5017a9ce58707a9087766b3fd6439` (no verdict; empty route output)
- OpenCode: `6d6c616ffe0da933babcecffdf93ec7aa36285f7222ce6cd2b176d7c61dba6af`
- OMP GPT-5.6 Sol: `83e3d01933f7bf59f32a251b7844c4c1b82a8f38c4ce9cb431f934d6ddfc591b`
- OMP Gemini 3.6 Flash: `0d5a05cdec4dd48a381fae2597b07cef060b13cb634c6ad2eb251e65de148e49`
- OMP DeepSeek v4 Flash: `665c7ccfbe97a32b8fd961fb1bd21483831c4eed600fa1b39fefc4ce005231b8`
- failed OMP Cursor Grok route: `1ef31e7ab11cf76adc29ad7d76facb04b1558dc1f8a297c5e0ddac496a53360c`
- runner manifest: `7a9ba3a3364e64c7a8e7aee8a245033e0c5337e6d762939125a7f22a853ae55f`
- frozen prompt: `90bedff1042d904eeb5e263620648db8412db63ce867f00e2d9d1c05e2a6d247`


### Candidate v4.1 re-panel output identities (raw bytes)

- Claude: `796265e629d97f7dee96500bf1b129f04622e73418f536af602de882a913ad2f`
- Codex: `477d23f4b258a644aa84991193c1938128598ceb6f4eef26f05128f503e07b17`
- Cursor: `19302b75ad4ab138d382a6b718c7be89394a9ad14134e9af44684f14fd889089`
- Devin: `320fd8fb3317f33a9dcd174f9be78f06f0898547edd3835c57ac52b12c41e1f4` (no verdict; route returned no review)
- OpenCode: `a2cab1fd9036bdb8da66c5d683ec6dde5d5c0139434ca31f6792323a3cc3befe`
- OMP GPT-5.6 Sol: `8a6941ad3e70566e60d92dafda771819c7e23165a8f4b039dd0009244670b7cf`
- OMP Gemini 3.6 Flash: `fe08d92c69f5916131d861e707bdf9d3b4b044dc3faf9f59fe159ffaadd9f5f8`
- OMP DeepSeek v4 Flash: `4dba2c717737221e05aa7b8e51a7074e9a6c70b0b4ec319f15d08090159211ab`
- failed OMP Cursor Grok route: `0b4c77a53b86eb4309fecd79b6b8f29408027a418f5ad1a9fb775fc98a59e18d`
- runner manifest: `cdd4f178145bc2f25b64f4ad88341f74cecf486c9dc85dec89045759c9d3108a`
- frozen prompt: `d2a7c1137d86e047332be62cf5ed4227964810363cfd9083746ccf824bfbd08e`

### Candidate v4.3 re-panel gate — 2026-08-08 (receipt-integrity chain)

- **Reviewed candidate:** Candidate `continuous-texture-supervision-definition-v4.3`
- **Panel:** nine routes including Claude; seven routes ok (Devin among them, no verdict), OpenCode timed out at 600 s, OMP Cursor Grok-45 failed (missing API key)
- **Result:** five `REPAIR` (Claude, Codex, Cursor, OMP GPT-5.6 Sol, OMP DeepSeek), one `ACCEPT` (OMP Gemini 3.6 Flash), Devin and OpenCode no verdict (OpenCode timed out at 600 s), and the OMP Cursor Grok-45 route failed on a missing API key
- **REPAIR findings:** (1) evidence ended with an orphaned digest restatement contradicting the frozen candidate.digest; (2) the Definition receipt chain lacked a v4.2 gate receipt entry and was non-chronological; (3) now.slice still named v4.2 while the candidate id is v4.3; (4) non-blocking: v4.2 panel outputs were not separately identity-recorded in the evidence.
- **Adjudication into candidate v4.4:** delete the evidence digest restatement (candidate.digest remains the single freeze authority); rebuild the receipt chain chronologically (v1, v2, v4, v4.1, v4.2, v4.3) and add the v4.2/v4.3 entries; now.slice updated to v4.4; v4.2 and v4.3 panel output identities recorded below.


### Candidate v4.2 re-panel output identities (raw bytes)

- Claude: `574843e7357af3225a421af5e2a59136e05f9ddc65ebeeb92b2445752c57f888`
- Codex: `3bd2f32c0a11f163c742ba40b0c44a16c83b7b8e37868271ffa327c14c3e72ea`
- Cursor: `8f1cee95b370b1ae433bca67835d08a90ce7050c0bc95f9b8cd55a83859a3bca`
- Devin: `29f5e155228d83789a463a5902b07c1bc780fc698b900210a883e8e04d11dc7b`
- OpenCode: `977f2146de8ea847d3024b09f79b3a69e8e3794746a2359732864b9f821c0879`
- OMP GPT-5.6 Sol: `f53ded5544f217f37eb1d4543013fbb92e97ce454eac292c360183f06e62babb`
- OMP Gemini 3.6 Flash: `bdc72cd888d340cb3910006da240218ec05cdebbbef6176febfde71c0430f023`
- OMP DeepSeek v4 Flash: `a93c68feb9c1e54c1d2fb0e03195ee1ed0e676245d4f72958a4edbbb1ba6ae34`
- failed OMP Cursor Grok route: `0b4c77a53b86eb4309fecd79b6b8f29408027a418f5ad1a9fb775fc98a59e18d`
- runner manifest: `931dfbf07d07618608d600c94ef63f653f0771118f5a8d7b27aedfadccf31b51`
- frozen prompt: `3a83d032569930a148ffcf4c14b4ed98e55f4f6d3a194bcf104c2f312e213558`

### Candidate v4.3 re-panel output identities (raw bytes)

- Claude: `dbd4d1df0a96d9d9e32a65a333de583dc24333dc01e845472ea4a5e485d64784`
- Codex: `b9e410fb9af8b28a396ab03e7080a38ddf1bc3a944a08ac773a81bf7eff0b348`
- Cursor: `ae5d0dfff8aa8fa4284b3ccf8a9c526109fb3dce460e18894833171e7c31a537`
- Devin: `1c76f917b15b1bb5aa98b0c27d770e038af3e690d59f2ea8aec5ee3f5e4a3d0d`
- OpenCode: `7241d512c1b85df7a464500caa3eaa63ecb978f79dd828e7eff91b0f4aab27b8`
- OMP GPT-5.6 Sol: `c8dd3636f3a72950df69c6ae2ef9ab53a57b91f5ae2de642bc33959f42245b3d`
- OMP Gemini 3.6 Flash: `d36d6b798b385b1dca1d58ef47a2fde9f6333a278f2db3daad77b0fede60cd8a`
- OMP DeepSeek v4 Flash: `3ff4385f37ea8974b76c2a4186844b8583a2a13710f1718cc756abbedb9b21b9`
- failed OMP Cursor Grok route: `0b4c77a53b86eb4309fecd79b6b8f29408027a418f5ad1a9fb775fc98a59e18d`
- runner manifest: `05232f386cca89c1ca8745d715dcab2716de794adcdf155924c53d0970766df9`
- frozen prompt: `834c808dc4eb849b42d3508d0c6c99fc6d8c1145b8f7a7e121d2fa384f345930`


### Candidate v4.4-1 re-freeze gate — 2026-08-08 (first v4.4 re-freeze; raw outputs lost)

- **Reviewed candidate:** Candidate `continuous-texture-supervision-definition-v4.4` (candidate bytes as frozen with digest at line 243 of that freeze)
- **Panel:** nine routes including Claude; REPAIR verdicts (Claude, Codex, Cursor, OMP GPT-5.6 Sol, OMP DeepSeek), one ACCEPT (OMP Gemini), Devin no verdict, OpenCode timed out at 600 s, OMP Cursor Grok-45 failed on missing API key.
- **REPAIR findings:** (1) v4.3 receipt digest was 65 hex chars (trailing `6`); (2) v4.3 proof path used wrong-case `docs/Evidence/` and dropped `.agentic-consensus/cts-v4.3-panel/`; (3) ACTIVE.md carried a corrupted word; (4) the v4.1 receipt lacked `registry_conformance_ref`. Adjudicated: repaired v4.3 digest, corrected evidence path, ACTIVE reworded, v4.1 registry ref added.
- **Raw-loss note:** this gate's raw `.out` outputs were overwritten in place by the v4.4-2 re-check in the same output directory, so no identity block can be rebuilt for v4.4-1; the loss itself is the receipt-integrity defect recorded in the v4.4-2 findings.

### Candidate v4.4-2 re-check gate — 2026-08-08 (green-gate re-verification of the fixed v4.4)

- **Reviewed candidate:** Candidate `continuous-texture-supervision-definition-v4.4` (fixed; digest `e89b73d007b4245b48a071eb227d4ac0cf063be0f602c475e260236523a66e16`)
- **Panel:** nine routes including Claude; eight ok (Devin among them returned no verdict), OMP Cursor Grok-45 failed on missing API key; no timeout.
- **Result:** four `REPAIR` (Claude, Codex, Cursor, OMP GPT-5.6 Sol), three `ACCEPT` (OMP Gemini 3.6 Flash, OMP DeepSeek v4 Flash, OpenCode), Devin no verdict, Grok-45 failed.
- **REPAIR findings:** (1) `observed_at` (2026-08-07T23:55Z) contradicted its own block label "2026-08-08 v4.4 gate" and the v4.3 candidate's later 00:12Z timestamp; (2) v4.3 gate section misstated panel composition (six REPAIR/one ACCEPT vs the actual five/one, plus an "eight completed" miscount), same miscount repeated for v4.2; (3) the first v4.4 gate had no receipt and its raw bytes were gone.
- **Adjudication into candidate v4.5:** `observed_at` set to current (2026-08-08T00:14Z); v4.3 tally corrected to five REPAIR/one ACCEPT in `blocker_or_risk`; evidence panel-composition lines corrected to "seven completed ..."; v4.4-1 and v4.4-2 receipts added to the Definition receipt chain; v4.1 receipt got its full 64-hex digest; candidate id bumped to v4.5 with `slice`/`worktree_inventory_ref`/`next_action` updated.

### Candidate v4.4-2 re-check output identities (raw bytes)

- Claude: `cb94ee2525d52706a46029a36d576557a930f4d2e4f1dc77eb0866de2b1c7f07`
- Codex: `100aac38ee1573f6175a0800d0be949f3ee836590aaf59fbe97564332479477c`
- Cursor: `05d85595099bda73d96b21a661dd28a5a76d9bbf0425bea1a160d7965817a919`
- Devin: `70e74ff543c344ab7ebe95000ea2d3aebdba948b93156766ed2bce8472e25cee`
- OpenCode: `fd57424621e501d1c731b7759fb224ca3a19c5df2ac122ba22fe2eaf90a5dafd`
- OMP GPT-5.6 Sol: `baca775b2eae6c1da21f6c4a2d630b217db4d924fc0d871c04762e6ac960a486`
- OMP Gemini 3.6 Flash: `11f159f8047621a047ab80e177ab3cff27832b9c115ffd64b259499908f8fdfb`
- OMP DeepSeek v4 Flash: `d7d03b9f4e3697e27ac1506d219ff2c77f90f98b019dfc8ab86ff390a0fd2e9c`
- failed OMP Cursor Grok route: `0b4c77a53b86eb4309fecd79b6b8f29408027a418f5ad1a9fb775fc98a59e18d`
- runner manifest: `e09fd1c2fb736c259b02b564d0c073b3d5e71528c944641fe17a09f3d329fb7d`
- frozen prompt: `5dde9713b7b8b9d9e17138d5de924d1f67ffc96119042f501232cef297bd7f8b`


### Candidate v4.5-1 gate — 2026-08-08 (stale-snapshot pointer; preserved as previous-round)

- **Reviewed candidate:** Candidate `continuous-texture-supervision-definition-v4.5` (digest `76aa485a596399e9dd509a686d49669c67b6d4b097780cc778928994b6b69df1`)
- **Panel:** nine routes; Claude returned REPAIR on four items (trailing-space scalar on Definition action line, v4.3 tally noun "seven completed verdicts", v4.4-2 "seven ok" undercount, reviewer-list comma), all of which were already fixed in the live v4.5 bytes before this gate launched.
- **Root cause:** the run launched with a prompt that still pointed at the prior v4.4 snapshot; the reviewed bytes were stale. Raw outputs preserved under `.agentic-consensus/cts-v4.5-panel/previous-round/` for identity; the real v4.5 gate is the v4.5-2 run below.

### Candidate v4.5-2 re-check gate — 2026-08-08 (green-gate re-verification of v4.5)

- **Reviewed candidate:** Candidate `continuous-texture-supervision-definition-v4.5` (digest `76aa485a596399e9dd509a686d49669c67b6d4b097780cc778928994b6b69df1`)
- **Panel:** nine routes including Claude; eight ok (Devin among them returned no verdict), OMP Cursor Grok-45 failed on missing API key; no timeout.
- **Result:** five `REPAIR` (Claude, Codex, Cursor, OMP GPT-5.6 Sol, OMP DeepSeek v4 Flash), two `ACCEPT` (OMP Gemini 3.6 Flash, OpenCode), Devin no verdict, Grok-45 failed.
- **REPAIR findings:** (1) two trailing-space scalars in the Definition (action line 108, candidate.ref line 240); (2) evidence v4.3 gate line said "seven completed verdicts" (noun error: seven routes ok including Devin, six verdicts); (3) evidence v4.4-2 line said "seven ok" but the v4.4 manifest has eight ok rows; (4) reviewer list "Claude Codex, Cursor" missing comma; non-blocking: ACTIVE.md adjudication label still named v4.4.
- **Adjudication into candidate v4.6:** trailing spaces stripped; v4.3 line reworded to "seven routes ok (Devin among them, no verdict), OpenCode timed out, Grok-45 failed"; v4.4-2 line corrected to "eight ok (Devin among them returned no verdict)"; reviewer comma added; ACTIVE.md label bumped to v4.6; v4.5-1 and v4.5-2 receipts added to the chain; candidate identity bumped to v4.6.

### Candidate v4.5-2 re-check output identities (raw bytes)

- Claude: `45c51ab2df27e63b58a8f3fab41c32981950fe3431198debf61f02a4710700b1`
- Codex: `d024d488c88c9330a5a8cb35e5a3dd5e65ca80e55ac9b4630b463818bd5a6653`
- Cursor: `a213f3ba0fcf77602f9d33ed622322a42085665267246968b75a684dc2ea66be`
- Devin: `71d322d5a57dceaa2bc630dcc4eb4ac2bb083540423534548e62b3407b254477`
- OpenCode: `ac609e19f2eac4b4c9d95def836ca8be9d9063ecc74c89337baa0b3ae32e0b85`
- OMP GPT-5.6 Sol: `cf8a8a4a85f6bfda6d7cc4dc94761696cddbc04f3e99a010faabccfdde9c06e3`
- OMP Gemini 3.6 Flash: `ee27e3307ff7365f2fb3c6754f746c7c78f240a8a60f4ac4736f45acabe41f49`
- OMP DeepSeek v4 Flash: `8cb514115fb4440a0b7ad34fa65998ec4d456e4c274e63ab48e1592de0cc59c4`
- failed OMP Cursor Grok route: `0b4c77a53b86eb4309fecd79b6b8f29408027a418f5ad1a9fb775fc98a59e18d`
- runner manifest: `85c576f4cad33cfe8d02ed82d8560286c4e56df5851170ccb0c25b571b93135b`
- frozen prompt: `0d16befdffe3e90e81d7cae5ced98610777ee3d7d7ff8230ccb9e861591fe193`


### Candidate v4.6-1 gate — 2026-08-08 (self-blocked; raw outputs overwritten)

- **Reviewed candidate:** Candidate `continuous-texture-supervision-definition-v4.6` (first freeze, digest `38fcbb5654c71f857db658220e92e8ab8284cd58c9be30a78bb9b42d9f6326de`)
- **Verdicts:** REPAIR across routes on three blockers (next_action hex literal contradicting candidate.digest; truncated 37-hex rollback_ref on the v4.5-2 receipt; "green snapshot" wording in a registry field). All three were adjudicated into the final v4.6 freeze; the run's raw outputs were overwritten in the same output directory by the same-candidate v4.6-2 run — an identity-block loss recorded in the Definition receipt chain (class: v4.4-1 repeat).

### Candidate v4.6-2 re-check gate — 2026-08-08 (green re-verification of frozen v4.6)

- **Reviewed candidate:** Candidate `continuous-texture-supervision-definition-v4.6` (digest `66392a6d827fde7754f4ba1a8f8e431f7cc8bb69bbd5367fce5c58d0ac4d3bea`)
- **Panel:** nine routes including Claude; eight ok (Devin among them returned no verdict), OMP Cursor Grok-45 failed on missing API key; no timeout.
- **Result:** four `REPAIR` (Claude, Codex, Cursor, OMP GPT-5.6 Sol), two `ACCEPT` (OMP Gemini 3.6 Flash, OMP DeepSeek v4 Flash), Devin and OpenCode no verdict, Grok-45 failed.
- **REPAIR findings:** (1) the v4.5-2 gate tally recorded Codex as ACCEPT while its raw output ended `# Verdict: REPAIR`; corrected to five REPAIR/two ACCEPT; (2) the v4.6-2 receipt originally misreported OpenCode as ACCEPT (raw opencode.out delivered no verdict); corrected to no-verdict. Process: v4.6-1 outputs overwritten (recorded, not repairable).
- **Adjudication into candidate v4.7:** v4.5-2 tally corrected to five `REPAIR`/two `ACCEPT` (Claude, Codex, Cursor, GPT-5.6 Sol, DeepSeek REPAIR; Gemini, OpenCode ACCEPT); v4.6-1/v4.6-2 receipts added; v4.6-2 disposition corrected to four `REPAIR`/two `ACCEPT` with Devin and OpenCode no-verdict; candidate id bumped to v4.7 with slice/next_action/worktree_inventory_ref/etc updated.

### Candidate v4.6-2 re-check output identities (raw bytes)

- Claude: `78521e5e79778b9fb7e052e8f70fdb5b38a0acf47a3df6748385e7ff630be1d4`
- Codex: `56fe97911f818de3b262b6feff55d0a1f5810f3c981bf04c418e67c2bd3f0f92`
- Cursor: `c1f1a6b13dd14b79d4fa5f4cb3c87480aa7433f1b484d42859cde81019d7655d`
- Devin: `20d0e95196532445f798c181c88923b43e9d1299d880cc70d93a790505e113f1`
- OpenCode: `be7f0af3aac5960aa8bc846306faeb5dab8becb4ea842cd840ebb2878869dce4`
- OMP GPT-5.6 Sol: `cb41b09f78efa2a955bd23b336683ba8a17cae9f630d30aab58103e700c3def3`
- OMP Gemini 3.6 Flash: `dd6c185fb7a8b10e9218e5cd5cf1aef745b53558ecb8f303c65430f10149a7cc`
- OMP DeepSeek v4 Flash: `7d0e9d42485a166059f54eea34bf2e6a80162c2e1faf796d10b9c7f42869b3e1`
- failed OMP Cursor Grok route: `0b4c77a53b86eb4309fecd79b6b8f29408027a418f5ad1a9fb775fc98a59e18d`
- runner manifest: `2e089bce0f552c13817e546b65aee90d03639f4f5ca571dc1ba9fe2ee197aca1`
- frozen prompt: `acb7e552653d2b0a7046d6c40aeae6320f0978a4732d302db2d69aef848ad5cf`

### Candidate v4.7-1 gate — 2026-08-08 (receipt-integrity re-check)

- **Reviewed candidate:** `continuous-texture-supervision-definition-v4.7`, historical self-normalized digest `2f6de3e16fe02e46962278aaf6c36251605f0db3fca153d3773ea8f41231333f`
- **Panel:** nine routes including Claude; Codex, Cursor, OMP Gemini 3.6 Flash, and OMP GPT-5.6 Sol returned `REPAIR`; Devin returned no verdict; Claude exhausted its monthly allowance before verdict; OpenCode and OMP DeepSeek timed out; OMP Cursor Grok-45 failed on missing API key.
- **Findings:** corrupt v4.6-2 receipt digest and rollback width, missing reviewer separators and required receipt fields, stale identity-count prose, and OpenCode mislabeled as `ACCEPT` despite no raw verdict.
- **Adjudication:** all identified receipt/schema/prose defects were repaired before the v4.7-2 freeze. No runtime or architecture authority changed.

### Candidate v4.7-1 output identities (raw bytes)

- Claude: `cdc95e84681612c8a5af6c99b6e06dc1292743bdd3ec9750b388bf7b1f584e3d`
- Codex: `fa850e7393e003cb561f61f53402f082f357018f064a62efa854073599e0690f`
- Cursor: `4e67427bb679cc122512d2dbe30a1f203d5b20bf37b165f953798590a8d42f36`
- Devin: `5cfb15a46892333acaae14c7cd5bb3e76e9c17cc79219c76de64fd0d511519ef`
- OMP Cursor Grok-45 failed route: `0b4c77a53b86eb4309fecd79b6b8f29408027a418f5ad1a9fb775fc98a59e18d`
- OMP DeepSeek v4 Flash timed-out route: `a93c68feb9c1e54c1d2fb0e03195ee1ed0e676245d4f72958a4edbbb1ba6ae34`
- OMP Gemini 3.6 Flash: `0e5f902ed5e1e519e0142206b473f1bb4c9deb38aa3fbfe8fc52357022a4e6a7`
- OMP GPT-5.6 Sol: `2750d1c7ce5e33d18199d90c83af0f1ab31724371adfc9857403d00e3524b483`
- OpenCode timed-out route: `bf74fd332fe4feb5420c049b479a30ba75c9866b890568d52cad99fcdd4955ed`
- runner manifest: `22489dd7454b2e87fe03faafc6aad89fc54db7dee6a019caf7bce3b45051168f`
- frozen prompt: `72b45170595fa9a32f5f290929fcf64c72c65cb2932145216b5ff1e7521dfcb2`

### Candidate v4.7-2 gate — 2026-08-08 (strict receipt re-verification)

- **Reviewed candidate:** `continuous-texture-supervision-definition-v4.7`, historical self-normalized digest `b189b070caaa25bfbd9b0aa12eb7c79a0f47508ec0d95616da36296408cdf94c`
- **Panel:** eight routes; six completed successfully, OpenCode timed out, and OMP Cursor Grok-45 failed. Claude was excluded from this run.
- **Result:** four `REPAIR` (Codex, Cursor, OMP GPT-5.6 Sol, OMP DeepSeek v4 Flash), one `ACCEPT` (OMP Gemini 3.6 Flash), Devin no verdict, OpenCode timed out, Grok-45 failed.
- **Findings:** missing v4.6-2 receipt fields and invalid boundary, a regressed v4.4-2 manifest count, residual corrupt prose, top-level rather than now-card `next_action`, and the absent v4.7-1 evidence section.
- **Adjudication:** the receipt linter was created; all mechanically enumerated receipt, width, prose, count, and placement defects were repaired. The evidence-ledger gap is closed in this append.

### Candidate v4.7-2 output identities (raw bytes)

- Codex: `9213c2e793e2e28ea28827842c2789b61ca8fb033eccc99315e7a582c011dbff`
- Cursor: `f707e6943194499d9e6703a836290d0d7ae308ceae43221701e07cdad977da29`
- Devin: `eafeb265fcfb9bea2442ea35fc227b65da4356039a0a41e5a673faa385bc5acd`
- OMP Cursor Grok-45 failed route: `0b4c77a53b86eb4309fecd79b6b8f29408027a418f5ad1a9fb775fc98a59e18d`
- OMP DeepSeek v4 Flash: `853943c92399d2133c0b19447234460806ab28ae0718475c90561f5f2f993c7d`
- OMP Gemini 3.6 Flash: `83c46373e320aad7b24d753d8f04878bedb78c8096a0de1785d7c4bebe9f1f89`
- OMP GPT-5.6 Sol: `442bc5266893ea7358bd2ec19c29ffb26c632081263e8da6710b8f46d9d666b3`
- OpenCode timed-out route: `46c6778601885c3220f3c9f2270f6fab055504978e68aec276cfb29a6f887ef4`
- runner manifest: `3a076dd277177115238f6988d9801f373faccdb002a35895440eb3789bdd7122`
- frozen prompt: `4ac63ae90ee63e91ad0c1479970fa2d5a338d23e2b3388ba33c2fd3aad8cecbc`

### Candidate v4.7-3 gate — 2026-08-08 (linter-clean promotion gate)

- **Reviewed candidate:** `continuous-texture-supervision-definition-v4.7`, historical self-normalized digest `5aeaa97c3d72bd20067ffc7ce6eb7b95df2c5f637adf48c8c2ee460d819b4d40`
- **Panel:** eight routes; seven completed successfully and OMP Cursor Grok-45 failed. Claude was excluded.
- **Result:** four `REPAIR` (Codex, Cursor, OMP GPT-5.6 Sol, OMP DeepSeek v4 Flash), one `ACCEPT` (OMP Gemini 3.6 Flash), Devin and OpenCode no verdict, Grok-45 failed.
- **Passing evidence:** dashboard parser, self-normalized digest, all 77 historical identity hashes, receipt schema, rollback widths, prose hygiene, registry blocking, and historical composition counts.
- **Remaining findings:** stale current-state wording incorrectly associated v4.7-2 with a successful gate; evidence did not yet append v4.7-1/v4.7-2; the new receipt linter was omitted from worktree inventory. All three are repaired in the owner-ratified final Define candidate.

### Candidate v4.7-3 output identities (raw bytes)

- Codex: `95014665895a11b5256cadb0e3ea80fa4b85c9e7b6bb943e65eedac3dd7705a5`
- Cursor: `5f06ac98d556ad6add9bdbd02a3fcadfcf8d3b08b6c8049fe7c4afde6ccfcc45`
- Devin: `88b57cadc40cdc65a12d9f813a67e4ea68935c3c46843bce5134ed6c3664f1d5`
- OMP Cursor Grok-45 failed route: `0b4c77a53b86eb4309fecd79b6b8f29408027a418f5ad1a9fb775fc98a59e18d`
- OMP DeepSeek v4 Flash: `841d6e8bce8263ce477fee3dd26fb7f04b8041471e2f214c085e60a499e83505`
- OMP Gemini 3.6 Flash: `965002871d2d153da89739f04d82df25bafae1b1f050e3d390d481a8eac00b06`
- OMP GPT-5.6 Sol: `c459930c3bf63f5545f9ef4772ecbc7d8db9bbdad79b4d8503c6b9a915eff0fe`
- OpenCode: `2943d561074ddb4b899c394147a80b65f9a10eadb432895b592453c5395d8873`
- runner manifest: `668c67e3151546536cf01b16a2628b878c0324811f45a999dc67cbbc3dc628bc`
- frozen prompt: `3a777e9c38b1a5be108c74af1e4f7d83e6143ccc80ce522a802cdb10aa58e0aa`

## Final owner settlement and promotion

The owner directed the orchestrator to proceed without another question or
panel, then corrected the verifier model explicitly: verifier must not be
read-only and may write files and run tests or other scripts. Those directions
ratify this architecture:

- Super may coordinate many capability-bound CoSupers; assignments use isolated
  writable networkless capsules and default to one writable assignment per
  capsule.
- A verification CoSuper has its own writable capsule and may edit files,
  create test support, and run builds, tests, and scripts. It is bound to the
  immutable subject identity being verified; changed subject bytes become a new
  candidate and cannot certify the original. Verification has no host or
  self-development effect authority.
- At least one independently identified verification result with test/script
  evidence is mandatory before completion.
- Expanded evidence is a deterministic rendering of the canonical `source_ref`
  node, never a second writable document truth.
- Texture revisions remain progressive and prose-first, with exact research and
  execution transclusions observable through the public API and CLI.

The v4.7-3 `REPAIR` dissent concerned current-state wording, missing ledger
entries, and linter inventory, not a new runtime architecture defect. The final
Define candidate repairs those items, inventories the durable linter, renames
the Definition without the draft suffix, and promotes the same executable
authority through `docs/ACTIVE.md`, `docs/mission-graph.yaml`, and
`docs/doc-authority-manifest.yaml`. Runtime behavior remains unchanged; the
Definition is executable but incomplete, and its red-mutation proof floor still
governs implementation.
