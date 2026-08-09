---
definition_version: 2
definition_id: choir-continuous-texture-supervision-2026-08-07
execution_mode: mission_orchestrator

start:
  captured_at: 2026-08-07T20:10:30Z
  source:
    canonical_ref: main@f64784e88b42abbf7d87fee058c989537b686d58
    deploy_identity: "Host exact fd83ce64209beae56f2a515d4408a1d88a2fd6e3 via CI/deploy 31306818891 attempt 2; target stable guest identity unproved because ordinary hibernated-computer wake returns persistent VM-route 502"
  worktree_inventory:
    status: reconciled
    evidence_ref: 2026-08-07 read-only git status and git worktree inventory
    preservation_rule: Preserve every non-primary worktree and all unrelated WIP; this draft owns only its Definition, review evidence, and three registry entries.
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-current-session
      touch: goal_owned
      paths_or_digest: [docs/definitions/choir-continuous-texture-supervision-draft-2026-08-07.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      recovery: revert the docs-only draft commit
    - path: /Users/wiz/.codex/worktrees/eb3c6a2a-cb9f-4067-8cd8-e8ec6224cb6f/go-choir
      status: dirty
      class: other_agent_wip
      owner: unknown
      touch: forbidden
      paths_or_digest: [.context/]
      recovery: leave in place
    - path: /Users/wiz/go-choir-terminal-outcome-closure
      status: dirty
      class: user_wip
      owner: unknown
      touch: forbidden
      paths_or_digest: [internal/objectgraph/dolt_store.go, internal/objectgraph/registry.go, internal/objectgraph/store.go, internal/store/graph_store.go, internal/store/store.go]
      recovery: leave branch autoputer-terminal-outcome-closure and worktree in place
    - path: clean secondary worktrees listed by the 2026-08-07 inventory
      status: clean
      class: other_agent_wip
      owner: unknown
      touch: forbidden
      paths_or_digest: [/private/tmp/choir-head-review, /Users/wiz/.codex/worktrees/5fbb817f-e1ac-4f37-8976-9ec4e94afb99/go-choir, /Users/wiz/go-choir-autoputer-v2, /Users/wiz/go-choir-s0-ratchet, /Users/wiz/go-choir-s3i17, /Users/wiz/go-choir-worktrees/chatgpt-search-provider]
      recovery: leave in place
    - path: prunable worktree registrations listed by the 2026-08-07 inventory
      status: unknown
      class: generated_temp
      owner: unknown
      touch: forbidden
      paths_or_digest: [choir-g1-judge, choir-g1-review, choir-g1-review-22a2, go-choir-architecture-recovery, go-choir-ci-doccheck-pr1, go-choir-ci-goal, go-choir-ci-integration-owner, choir-g1-review-yheq91cz]
      recovery: leave registration metadata untouched
  candidates:
    - id: continuous-texture-supervision-definition-v1
      ref: main working tree
      base: f64784e88b42abbf7d87fee058c989537b686d58
      scope: [docs/definitions/choir-continuous-texture-supervision-draft-2026-08-07.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      disposition: active
      evidence_ref: frozen review digest 880c00cb8f7a6c546825689a7ba37c02a551f897a93080f5e1e2dbbda2cb89ed
  observed_artifact:
    - claim: Texture's production registry omits update_coagent although its profile allows coagent tools and its prompt describes Researcher follow-up.
      evidence_ref: internal/agentcore/tool_profiles.go InstallDefaultAgentTools; internal/agentprofile/agentprofile.go PolicyFor
    - claim: Texture may spawn only Researcher; adding Super to AllowedDelegateTargets would wrongly allow arbitrary Super creation rather than address the one persistent Super.
      evidence_ref: internal/coagentowner/spawn_tool.go RegisterSpawnTool; internal/agentcore/runtime.go persistentSuperAgentID
    - claim: The typed coagent_source_packet.v1 path carries producer UUID, trajectory, work-item, payload-digest, disposition, and wake identity, but target authorization is not Texture-safe.
      evidence_ref: internal/agentcore/tools_worker_update.go; internal/agentcore/tools_researcher.go; internal/types/evidence.go
    - claim: Target lookup can fail open when AgentRecord lookup errors, and the generic resolver self-targets a Texture channel before honoring explicit agent_id.
      evidence_ref: internal/agentcore/tools_worker_update.go target lookup; internal/agentcore/tools_researcher.go resolveCoagentFindingsTarget
    - claim: Researcher target resolution accepts an arbitrary owner-scoped texture:<doc_id> shape without proving requester, document, or trajectory binding.
      evidence_ref: internal/agentcore/tools_researcher.go resolveResearcherFindingsTarget
    - claim: Lifecycle QueueLifecycleUpdate accepts only Texture targets, while persistent Super drains the pre-cutover worker inbox; a lifecycle Texture can therefore reach neither a bound Researcher nor persistent Super through one accepted durable path.
      evidence_ref: internal/store/lifecycle.go QueueLifecycleUpdate; internal/store/store.go DispatchWorkerUpdate; internal/agentcore/super_controller.go reconcilePersistentSuperActor
    - claim: Researcher, Super, and CoSuper updates can already wake Texture and be incorporated into immutable canonical revisions.
      evidence_ref: internal/textureowner/texture_controller.go; internal/textureowner/tools_texture.go; internal/agentcore/coagent_update_packet.go
    - claim: A successful Texture patch or rewrite terminates the activation, and inbound update activations mechanically require a canonical write; this prevents post-revision redirection and can create bogus no-op revisions for redundant evidence.
      evidence_ref: internal/agentcore/runtime.go executeWithToolLoop; internal/toolregistry/toolloop.go; internal/textureowner/tool_loop_policy.go; internal/textureowner/tools_texture.go
    - claim: 'Owner correction is split: /revise dispatches a pre-cutover worker-update packet, while a direct canonical revision changes the head without reliably waking the lifecycle Texture actor.'
      evidence_ref: internal/textureowner/texture_agent_revision.go deliverOwnerRevisionToTextureActor; internal/textureowner/texture.go handleTextureCreateRevision
    - claim: Persistent Super and capsule-scoped implementation/verifier CoSuper authority already exist, but lifecycle-requested effects-capable activation is refused while effects are OFF.
      evidence_ref: internal/agentcore/super_controller.go; internal/agentcore/runtime.go StartCoagentRun; internal/agentcore/tools_capsule.go; internal/capsule/roles.go
    - claim: The deleted request_super_execution was a same-channel ChannelCast with no durable trajectory accounting; restoring that implementation would restore the defect.
      evidence_ref: git commit 7b7bba73 historical Texture tool source; removal 0df1412312de; docs/archive/choir-architecture-review-next-moves-2026-06-11.md
  unknowns: []

start_corrections:
  - corrected_at: 2026-08-07T20:34:47Z
    evidence_ref: docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md
    correction: The primary worktree was clean at start capture and became dirty-but-goal-owned when the four-file candidate was frozen. Review reproduced digest 880c00cb8f7a6c546825689a7ba37c02a551f897a93080f5e1e2dbbda2cb89ed; the repaired candidate adds this evidence file and awaits a new self-normalized digest.
    consequence: "Exact source, independent review, full CI/SBOM, deployment, and product identity gates passed for b5d907a3. The deployed product proves Researcher control binding and repeated provider evidence, but producer reports and a queued owner correction did not wake/advance the resident Texture actor, while open Researcher work amplified provider activations. Effects cannot turn on and registries cannot close until this newly documented mailbox/reconciliation substrate is repaired, a fresh repeated authenticated trajectory reaches persistent Super and parallel capsules, all no-effect/restart/cancellation/source-open/run-acceptance gates pass, temporary credentials/backups receive terminal disposition, and the final audit closes."
  - corrected_at: 2026-08-08T02:37:12Z
    evidence_ref: docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md
    correction: The owner ratified the repaired architecture and directed promotion. The executable Definition is renamed without the draft suffix; its final candidate adds the consensus evidence, all three registry entries, and scripts/lint/cts-receipt-lint.py to the identity-bound scope.
    consequence: The immutable start paths remain historical. The now card and promoted registries name the executable path and complete current worktree inventory.


finish:
  deliver: Texture continuously supervises one durable trajectory by revising a canonical, prose-first document and issuing mechanically authorized controls to its own Researchers and its owner's one persistent Super. Texture publishes grounded intermediate versions while work remains open; Super may coordinate many assigned capsule-only CoSupers and returns intermediate evidence until Texture settles or redirects the work. Research, code excerpts, diffs, terminal commands and output, tests, and later multimedia all use one generic transcluded-source contract rather than workflow-specific document syntax.
  artifact: An authenticated staging trajectory with at least three causally distinct Texture revisions and two downward control cycles. At least one informative appagent revision is owner-visible while assigned work remains open, a later revision incorporates an owner correction or new result, and the versions include exact openable research and execution-source transclusions. The trajectory includes a Texture-originated Researcher follow-up, a Texture-originated persistent-Super execution request, parallel isolated CoSuper work, a verification CoSuper result from its own writable isolated capsule incorporated by Texture, durable work/update/request identities, and an owner-inspectable latest Texture version. The same facts are observable through the Texture API and the `choir texture` CLI without browser prose scraping or internal database access.
  acceptance:
    - action: Run focused role-registry, fail-closed target authorization, direction-specific lifecycle reducer, atomic Texture transition, structured-document projection, generic source-transclusion, race, replay, owner-correction, cancellation, capsule capability, and Texture API/CLI contract tests.
      proves: Allowed calls work; prose and structured evidence remain one revision; lookup errors and cross-owner, cross-computer, cross-document, cross-trajectory, arbitrary-Super, direct-CoSuper, non-lifecycle Texture, verifier host/effect mutation, duplicate, stale-head, and late-result calls fail without weakening existing authority. Verification CoSupers may write and run tests or scripts only inside their assigned capsules.
      evidence_class: focused local contract and race tests
    - action: Reconstruct pending Researcher and Super controls and resumable Texture version observation after process restart; replay equal and conflicting command, packet, request, and cursor identities; replay pre-cutover fixtures.
      proves: One lifecycle reducer/projection is authoritative, ordered outbound digests are stable, equal retry is idempotent, conflicting retry fails, old revisions and their exact transcluded sources remain readable, and no new Texture traffic enters the pre-cutover worker inbox.
      evidence_class: local restart, reducer, API/CLI, and compatibility proof
    - action: Drive prompt bar -> Texture v1 -> Researcher partial result -> owner-visible Texture v2 while work remains open -> focused Researcher redirect -> persistent Super intermediate result -> Texture v3 and changed Super direction -> at least two parallel networkless writable CoSuper assignments in separate capsules -> at least one verification CoSuper that writes test support and runs tests/scripts in its own isolated capsule -> later Texture version incorporating its typed result on staging through authenticated product APIs.
      proves: Repeated bidirectional supervision produces progressive prose revisions with grounded evidence and uses real actors, lifecycle work/control, and isolated capsules rather than manually started workers, a fixed one-implementer/one-verifier topology, an immediate generic update tool, or one terminal round trip.
      evidence_class: deployed authenticated product-path proof
    - action: Through public APIs and the `choir texture` CLI, create a Texture, tell it what to do, watch successive versions, show an exact current or historical version, and open the exact source behind a research quotation and an execution transclusion. Disconnect and resume watching from the last durable cursor.
      proves: Automation can observe the same progressive document the owner sees; human prose is not the test protocol; version lineage, ongoing-work state, request/update causality, and source identity/content are machine-readable and survive reconnect.
      evidence_class: local contract plus deployed CLI/API acceptance
    - action: Run an owner-directed continuous-prose case with no requested headings or lists and a differently styled report case through the same document and transclusion APIs.
      proves: The schema supports generic writing and does not force a supervision dashboard or coding template; document form follows owner intent while source grounding remains mechanically inspectable.
      evidence_class: deployed product behavior and structured-document inspection
    - action: "Leave Researcher and Super controls pending, passivate both actors, perform an approved no-SSH same-build staging process restart by re-running the existing workflow_dispatch force_staging_deploy deploy with the identical SHA and confirming the same container identity through /health; continue the same owner/computer/document/trajectory/work/actor/update identities."
      proves: Pending state, cold reconstruction, and exact causality survive process and residency boundaries.
      evidence_class: deployed no-SSH restart and durability proof
    - action: Commit a direct owner revision through the deployed editor while Texture is parked; separately submit a natural-language revise request; then inspect the next revision and downstream controls.
      proves: Canonical owner edits remain immediate, both correction semantics wake through lifecycle authority, and subsequent supervision binds the corrected head.
      evidence_class: deployed owner-control proof
    - action: Cancel in-flight Super/CoSuper work, deliver a real delayed result and exact retry after cancellation, and fetch trajectory/update/run evidence.
      proves: The result is late and evidence-only; no obligation reopens and no actor wake, semantic revision, or accepted state follows.
      evidence_class: deployed failure-semantics proof
    - action: Compare canonical computer event heads, self-development operation state, materialization, checkpoint, route generation, and relevant host projections before and after capsule work; inspect each accepted Texture version against its typed packets, source objects, and Trace.
      proves: The owner sees evolving semantic state with exact embedded evidence rather than command-log or status-template leakage, while verifier and capsule work cause no computer-event, finalization, host, or promotion effect.
      evidence_class: deployed human inspection and no-effect proof
  rollback: Keep all schemas additive and replay-compatible; leave self-development materialization, acceptance, checkpoint, and route effects OFF; revert the behavior commit through origin/main and CI to the last accepted runtime, retaining new packets as pending/late rather than falsely delivered. If lifecycle-native addressing cannot be made single-authority, discard the candidate and revise this Definition instead of adding a bridge or second queue.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - Owner ratification or executable authority in any of the three registries is absent.
    - Only prompt text, registry presence, mocks, local tests, or model narration says the loop works.
    - Acceptance manually starts Researcher, Super, or CoSuper instead of Texture addressing them through product behavior.
    - Only one worker round trip or one post-worker Texture revision exists.
    - New Texture controls write the pre-cutover worker inbox, create a second causal log, or permit two stores to disagree about pending/delivered state.
    - Texture can address CoSuper, a foreign Researcher, or an arbitrary Super; Super or verifier CoSuper gains host mutation.
    - A redundant or unusable packet forces a duplicate semantic revision or disappears without a durable disposition.
    - Effects-capable work can materialize, accept, checkpoint, route, or mutate the host while those effects remain OFF.
    - Target lookup errors, a non-lifecycle Texture, or a direct generic update call can enqueue privileged work.
    - One field or reducer rule ambiguously treats producer-owned report work as target-owned control work.
    - A Texture control can commit or wake independently of its revision/no-change/wait/block transition.
    - Direct owner revisions wait for a provider, or owner head commit and actor wake are separate failure domains.
    - Verifier CoSuper can call record_self_development_verification, append a computer event, mutate updater state, or propose an effect.
    - The disposable capsule has host networking or any capability not bound to the exact run/capsule/slot.
    - Texture exposes only a final report after assigned work settles, or the proof cannot identify an informative revision committed while work was still open.
    - A displayed quotation, code excerpt, diff, command, test result, or media item is copied prose rather than a transclusion bound to an exact openable source version and content hash.
    - Automated acceptance must scrape generated prose, poll with sleeps, or inspect an internal database because version, request/update causality, ongoing-work state, source identity, or resumable observation is absent from the public Texture API and CLI.
    - The document schema or default prompt forces headings, lists, status fields, work-item inventories, or a coding-specific report shape when the owner requested continuous prose.
    - Super is limited to one implementation and one verifier CoSuper, or two writable CoSupers can race inside the same capsule without an explicit exceptional coordination contract.

boundaries:
  mutation_class: red
  authority_sources: [AGENTS.md, docs/choir-doctrine.md, docs/agent-product-doctrine.md, docs/computer-ontology.md, docs/standing-questions.md, docs/supervision-protocol.md, docs/texture-agentic-invariants-2026-06-13.md, "owner directive in this run: proceed without asking; promote the repaired Definition as executable"]
  must_preserve:
    - Texture is the sole canonical document writer among agents and the delegated semantic controller; direct owner revisions remain canonical, while operational directives remain typed packets, work items, and Trace.
    - The lifecycle trajectory, work-item, worker-update object, event sequence, and snapshot reducers are the single authority for new supervision state; no new event log, service, workflow engine, or dual mailbox write.
    - "Producer reports and downward controls are distinct reducer commands; producer_work_item_id belongs to the reporting caller; target_work_item_id belongs to the addressed actor; no sender settles target work."
    - Texture may spawn Researcher only, address only its bound Researchers and exact super:<owner> on the current computer, and never address CoSuper directly.
    - Target lookup is fail-closed; channel shape, prompt text, caller-supplied role, and model-authored direction are never authority. Non-lifecycle Texture cannot send this control.
    - The Super identity is one persistent non-lifecycle actor per owner/computer; Super may be an exact lifecycle control target/assignee without becoming a generic lifecycle actor class.
    - A Texture revision/no-change/wait/block outcome, inbound dispositions, target-work changes, and ordered outbound controls commit in one conditional object-graph batch; target wake happens only afterward.
    - Direct owner revisions remain immediately canonical and atomically record a lifecycle correction cursor/obligation; natural-language revise requests remain Texture-authored decisions.
    - Every implementation or verification CoSuper is run-bound, assigned-capsule-only, writable within that capsule, and networkless. A verification assignment may edit files and run tests/scripts in its own capsule, but it verifies an immutable subject identity; changing subject bytes produces a new candidate identity rather than a verdict on the original. Neither role receives self-development finalization or host-effect authority.
    - Producer and control identity is runtime-derived; owner, computer, document, trajectory, producer work, target work, target, action class, ordered payload digest, and idempotency are validated before durable enqueue.
    - Canonical event/private payload, encryption, Trace/evidence, cancellation, late-result, restart replay, run acceptance, and exact staging identity contracts do not weaken.
    - Semantic delegation remains agentic; no keyword oracle, forced first tool, required role choreography, polling loop, or trivial revision hides background work.
    - Texture's visible artifact is generic writing, normally coherent prose. Structured document nodes carry editing and evidence semantics but never mandate a supervision dashboard, workflow inventory, coding report, heading sequence, or list-heavy style.
    - One generic source-reference/transclusion model covers research quotations, files and code excerpts, diffs, patches, commands and terminal output, test runs, images, and later multimedia. Domain-specific renderers do not create domain-specific supervision authority.
    - Every expanded transclusion binds its body placement to an immutable source identity, version, selector, and content hash. The structured document is canonical; rendered prose and the machine-readable transclusion manifest are deterministic projections, never independently writable truths.
    - Public Texture API and CLI surfaces expose create, tell/correct, watch, show exact version, and open-source behavior with stable machine-readable output and resumable durable observation; they do not expose raw actor chatter as document state.
  excluded:
    - Restoring the historical same-channel request_super_execution implementation, its required-first-tool behavior, or the current immediate generic update_coagent implementation for Texture.
    - Adding a generic supervision subsystem, separate public supervision API/CLI, generic actor-mailbox migration, second tape, new TLA control plane, VM/route controller, or broad write-mode flag. Extending the existing Texture API and `choir texture` CLI to expose the accepted document/version/source contract is required, not excluded.
    - Promoting persistent Super to a generic lifecycle actor class; new Texture traffic through DispatchWorkerUpdate; dual delivery cursors.
    - External coding-agent integration, fleet promotion, ComputerVersion materialization, checkpoint publication, route CAS, self-development verification/finalization/acceptance, or canonical event append from capsule proof.
    - UI redesign, publication/Wire work, and unrelated runtime cleanup.
  protected_surfaces: [Texture canonical writes and source graph, lifecycle trajectory/work/update reducers and conditional object-graph batch, actor mailbox/passivation/restart, private payload encryption and audit, role tool registries and fail-closed target authorization, persistent Super identity and inbox, capsule capability/slot/network namespace enforcement, canonical computer event chain, self-development operation and updater root, cancellation and late results, run acceptance, provider/model calls, deployment routing]
  completion_evidence_floor: [problem-documenting docs-only commit before repair code, frozen implementation candidate, independent security and architecture review, focused race and replay tests, deterministic Texture document/projection/transclusion and API/CLI contract tests, full applicable CI, exact staging commit identity, authenticated repeated-cycle and same-build restart proof, owner-correction proof, progressive revision while work remains open, exact research and execution transclusion proof, resumable CLI/API observation, independent writable-capsule verification result with test/script evidence, actual late-result proof, before/after no-effect receipts, owner-visible Texture inspection, rollback receipt]

measures:
  - name: target_authority
    kind: gate
    baseline: Texture has no registered update_coagent; generic resolver is unsafe if merely enabled.
    desired: Every accepted outbound packet proves exact caller and target owner/computer/document/trajectory/work binding; all skip-level and foreign targets fail.
    decision_use: Blocks implementation or landing on any authority bypass.
    cannot_prove: Semantic usefulness or deployed continuity.
  - name: single_delivery_authority
    kind: gate
    baseline: Lifecycle updates target Texture while persistent Super consumes pre-cutover worker-inbox rows.
    desired: New Texture-originated control uses one lifecycle-authoritative enqueue/replay/disposition path and no pre-cutover dual-write.
    decision_use: Rejects compatibility bridges that create two pending/delivered truths.
    cannot_prove: Human comprehension of Texture.
  - name: revision_control_causality
    kind: gate
    baseline: Upward packet-to-revision exists; downward post-revision control does not.
    desired: At least two distinct downward controls and at least two later revisions each cite consumed packet/update cursors on one trajectory.
    decision_use: Distinguishes a loop from one-shot generation.
    cannot_prove: Long-horizon quality beyond the accepted trajectory.
  - name: authority_negative_matrix
    kind: gate
    baseline: Registry tests cover Super/CoSuper names but not Texture addressing or slot-complete invocation behavior.
    desired: Role and slot matrix proves forbidden host, spawn, target, capsule, and verifier operations fail at call time.
    decision_use: Blocks removal of effects-capable guard until equivalent narrower enforcement exists.
    cannot_prove: Provider quality.
  - name: owner_read_amplification
    kind: telemetry
    baseline: unknown
    desired: Multiple grounded revisions and control turns may occur between owner reads while the latest Texture remains sufficient to resume supervision.
    decision_use: Guides later cadence and compaction tuning only.
    cannot_prove: Mission completion or correctness.
  - name: progressive_owner_visibility
    kind: gate
    baseline: Upward evidence can eventually produce a revision, but the accepted contract does not require an informative version before work settles.
    desired: At least one grounded appagent revision becomes owner-visible while assigned work remains open, and a causally later revision incorporates new evidence or an owner correction on the same trajectory.
    decision_use: Rejects final-report-only behavior and fake progress events that do not change the owner-readable artifact.
    cannot_prove: Long-horizon editorial quality.
  - name: generic_grounded_writing
    kind: gate
    baseline: Texture supports structured source references and execution source kinds, but expanded in-document transclusion and prose-first genericity are not accepted end to end.
    desired: Continuous prose and differently structured writing both use the same canonical document schema; exact research and execution sources are visibly transcluded and openable by immutable identity/version/hash.
    decision_use: Rejects workflow-template writing, copied evidence without provenance, and domain-specific document schemas.
    cannot_prove: That every generated style choice is tasteful.
  - name: automatable_texture_surface
    kind: gate
    baseline: Public Texture APIs expose documents, revisions, and a non-resumable stream; the CLI exposes only read, history, and revisions.
    desired: Public API and CLI can create, tell/correct, durably watch and resume, show an exact version, and open a transcluded source with stable JSON or JSONL identities.
    decision_use: Makes progressive supervision verifiable without browser text scraping, internal storage access, or timing sleeps.
    cannot_prove: Human comprehension without owner inspection.

now:
  status: active
  slice: "API-key stable-computer repair fbc7ff5a is committed, deployed, and source/CI identity-proved. CI/deploy 31326948312 and registry/docs follow-ups through ce8ac099 are green. Public /health and selected-service activation receipts exact-join fbc7ff5a; the host closure also updated canonical guest-image and sandbox pointers without refreshing the failed retained computer. Independent security/route reviews ACCEPTed the guard. Secure clipboard ingestion, exact sequential pre-recovery bearer proof, terminal narrow-key revocation, the sole retained-computer recovery/nonce-identity ceremony, and a bounded post-recovery no-effect baseline are source-prepared and independently ACCEPTed. No usable bearer exists and none of these ceremonies has run. The bounded two-role capsule policy apply/rollback is also independently source-ACCEPTed under a specific orange provider-routing ceremony and remains unexecuted. The exact first acceptance write is now independently source-ACCEPTed at helper SHA-256 677cc8a0bff9ffa58465d01db72774ef6080d17e18d6ec12997baf2c01d6b54c with an unchanged 1,109-byte outcome prompt at SHA-256 3726cc5037f15cc6da4bd6db8a9f2eefe07503020b2a140382c53e2b005d4cc1; it atomically retains the exact start/recovery/cancel/rollback obligations and can PASS only on joined V0 plus continuous-prose v1 with exact open Texture and Researcher work. It too remains unexecuted. The exact owner-correction/causal-v2 ceremony is now independently source-ACCEPTed at helper SHA-256 c068c1c87aa9534248c5970aa96881de0ee0c2e1a6f0f6aeb8a98a9e73052dd5; it catches up and freezes the accepted cursor before one semantic `/correct`, joins the exact pending/consumed receipt through public Texture and raw lifecycle events to a live/open v2, requires the frozen qualification exactly once plus a canonical source identity new from v1, and proves an identical consumed replay did not duplicate the occurrence. It remains unexecuted. The exact focused second-control ceremony is independently source-ACCEPTed at helper SHA-256 0f09449a67c03160aa004ce481374184b330224518cd9a07df48bd8f7628491f; after complete public/raw cursor and first-control joins it makes one outcome-only tell and can PASS only on the unique queue → non-version consuming turn → same-command second control, with the full set changing exactly one-to-two while reusing the same open Researcher/work binding. It stops before delivery, Super, capsules, or v3 and remains unexecuted. A new source-only product-contract audit proves fbc7ff5a can drive and observe the public supervision loop but mounts no public raw Trace handler, so action 9 remains source-blocked even after the owner gate; this problem is now documented before any repair and no internal substitute will count. The frozen direct payload may execute once only after canonical /auth/session proves exact retained owner c72404bb-3c43-4a53-8671-b5cbc48b24a7 with physical presence. Effects remain OFF."
  question: "Will exact retained-owner authentication mint one narrow stable-computer-bound key that proves deployed correct-target selection and wrong-target no-effect denial, then recover only the retained computer for nonce-bound exact guest/service acceptance?"
  reconciliation:
    observed_at: 2026-08-09T20:38:00Z
    source_ref: fbc7ff5a048ed58d0f6dd02ae8462ae211eca328
    deploy_identity: "CI/deploy 31326948312 succeeded for exact fbc7ff5a; public proxy /health reports build.commit and deployed_commit fbc7ff5a with deployed_at 2026-08-09T18:09:46Z. Deployment activation receipts exact-join auth, proxy, vmctl, gateway, corpusd, maild, and sourcecycled to fbc7ff5a. This proves host/source activation only, not bearer ownership, correct-target routing, wrong-target no-effect, retained guest identity, or acceptance. Both accidental keys are revoked/post-401; no usable bearer exists. Retained computer/VM remains unproved pending exact owner/key and nonce identity; last exact guest proof is b5d907a3 epoch8252."
    authority_identities: [docs/choir-doctrine.md, docs/agent-product-doctrine.md, docs/computer-ontology.md, docs/supervision-protocol.md, docs/texture-agentic-invariants-2026-06-13.md, docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, docs/evidence/continuous-texture-supervision-joined-runtime-review-2026-08-08.md, docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md, "owner directive in this run: proceed without asking", "owner directive in this run: use the Definition dashboard to supervise /goal until Texture works correctly", "owner directive 2026-08-08T23:24Z: update the ChatGPT token using the token held locally", "owner directive 2026-08-08T23:24Z: SSH Node B is authorized for this restoration"]
    policy_resolution_ref: "Authenticated model-policy file and resolve APIs; original SHA-256 7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a restored exactly after both the alternate-route probes and the post-escalation ChatGPT/Z.AI recurrence; sanitized local provider-route/availability diagnostics found HTTP 402 DeepSeek/Xiaomi balance failures, HTTP 412 Fireworks precondition failure, HTTP 429 Z.AI balance/rate failure, and direct local Bedrock bearer invokes in us-east-1 returned HTTP 403 for both gateway seed us.anthropic.claude-haiku-4-5-20251001-v1:0 and exact owner-policy-selected us.anthropic.claude-sonnet-4-6 without exposing credentials or response bodies; the Bedrock results prove only those bounded tuples were forbidden, not account or host state"
    worktree_inventory_ref: "2026-08-09T20:38Z: local/origin main exact 0a8c1bb0 over host-deployed runtime fbc7ff5a. Runtime source/tests remain clean. This Definition, adjacent problem inventory, and requirement audit are the only intended tracked edits for the accepted second-control ceremony checkpoint. The secure ingestion/binding/recovery/baseline/revocation helpers, bounded policy apply/rollback helpers, exact start/v1, correction/v2, and focused second-control helpers, and reproducible no-redirect verifier remain unexecuted outside the repo. The frozen direct payload remains byte-exact. Three unrelated untracked memo files remain untouched. Root-only provider-auth backups remain pending terminal disposition."
    status: reconciled
  candidate:
    id: continuous-texture-supervision-runtime-v1
    state: api_key_computer_binding_repair_deployed_owner_presence_pending
    ref: git:fbc7ff5a048ed58d0f6dd02ae8462ae211eca328
    owner: current goal session
    base: 4a112be6
    digest: fbc7ff5a048ed58d0f6dd02ae8462ae211eca328
    scope: [internal/auth, internal/proxy, internal/agentprofile, internal/actor, internal/agentcore, internal/actorruntime, internal/capsule, internal/store, internal/textureowner, internal/toolregistry, internal/types, cmd/choir, scripts/go-test-non-runtime-shards, docs/supervision-protocol.md, docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md, docs/evidence/continuous-texture-supervision-joined-runtime-review-2026-08-08.md, this Definition and adjacent focused tests]
  decision:
    selected: "Preserve the settled lifecycle/Texture/Super/capsule architecture and add one shared API-key use-time authority: before any private computer selection, route, resolve, wake, dial, self-development call, recovery, lifecycle intent, or execution attestation, require non-empty stable ComputerID metadata and exact vmctl LookupComputerContext(UserID, ComputerID) plus canonical path/DesktopID equality. Creation metadata attenuates but never proves ownership; unbound legacy/admin keys are denied computer routes."
    kind: security_architecture
    status: settled
    source: owner
    evidence_ref: docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md
    owner_ratification_ref: "Owner directives in this run: proceed without asking; promote the repaired Definition; verifier may write and run tests/scripts only inside its capsule; supervise through the Definition dashboard until Texture works correctly."
    recorded_at: 2026-08-09T16:26:00Z
    consequence: "All prior lifecycle/Texture/runtime source, CI, deployment, rollback, provider, and partial product evidence remains retained. Exact-owner reconciliation falsified registry binding as ownership; fbc7ff5a now deploys the shared vmctl ownership guard across private computer routes. Source/host identity is proved, but live bearer behavior is not. Effects cannot turn on or registries close until exact-owner key creation, sequential target/no-effect proof, the sole accepted retained-computer recovery, nonce-bound guest identity, and the full supervision loop/restart/cancellation/no-effect/run-acceptance/cleanup audit pass."
  evidence_refs: [git:ac6dd16b1980a1a3faedd7d1d83fefa79395a1ee, "CI 31261269488 success", "Node B exact deployment ac6dd16b", "nonce-bound execution identity at VM epoch 8247", "Texture restart run 5ee276b3-d25c-41ac-afaa-5879a6ea5ecf", "public lifecycle cancellation command public-cancel:cts-failed-acceptance-cancel-ac6dd16b-v7", "post-escalation cancelled ChatGPT trajectory 41cec88f-510f-53cc-a5e7-84c372b5421b and Z.AI trajectory aca3504c-2ae0-5a4e-bab5-b22541e90585", "sanitized local provider-account diagnostic: DeepSeek 402 balance-related, Xiaomi 402 insufficient_balance, Fireworks 412 PRECONDITION_FAILED, Z.AI 429 code 1113 balance/rate-related", "deployed CLI partial acceptance on document 11902866-d32e-55c4-9483-d9bd47c91a6c: read/history/revisions/exact-show; durable watch pages 1-2,3-4,5-6,7-8,9 and empty after 9; cancelled tell/correct HTTP 409; cross-document show rejected", "local source-only rollback rehearsal: 99 non-doc paths exact cdaa787b; path digest 0b7eb4241e3dc5a70705ce596f436a372b5213593457c0c9b831c8c9296b22f3; reverse patch sha256 64a61e5db159cf7d839532bad9a2a9d320e11a430e100a0ad6de2998b77530a8; sequential runtime shards/focused/vet/diff passed", "authenticated partial no-effect readback: self-development off generation 0 before/after; model-policy sha256 7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a restored; route digest/epoch 8247/host+guest ac6dd16b unchanged between post-deploy identities; key revoked/post401", "deployment rollback red-team: historical rerun-all rejected; one active interactive primary at epoch8247, 12 terminal/0 active runs, no deploy triggered, preflight key revoked/post401", "representative cross-version rollback compatibility: ac6 terminal graph with owner instruction/Texture turn+2 revisions/exact-run control/cancel intent/cancelled runs+work/events+receipts; old 460 Store+Runtime.Start on same Dolt HEAD lvtb74ss94q6u8jpmtd32707oefj2pu5; dispatch0, clean before/after, ac6 reopen preserved new kinds; not production DB/deploy proof", "independent canonical rollback red-team 2026-08-08: PROCEED iff binding current-main R/F gates; active owner-ratified Definition supplies authority; historical rerun remains forbidden", "canonical R 10d4865958b7d8deaab5665f74b37dd1b5005070: CI/rolling/deploy 31267448310 success; exact joined identity epoch8248; midpoint old response omitted six stored owner-instruction request_id fields and triggered F", "canonical F 67a61358ceda55c30e9853907f85648bb8531bb8: whole tree exact pre-R 2f8d912e; CI/rolling/deploy 31268477380 success; exact joined identity epoch8249; all captured frozen comparator response/state digests equal baseline; 1909s exposure; key revoked/post401", "sanitized post-R/F local availability recheck: DeepSeek 402, Xiaomi 402 insufficient_balance, Fireworks 412 PRECONDITION_FAILED, Z.AI 429 code1113, direct Bedrock bearer invokes in us-east-1 returned 403 for gateway seed haiku and exact selected sonnet; diagnostic only, no host proof", "exact-F default ChatGPT recurrence: first key omitted write:texture and was refused 403/no-mutation then revoked/post401; replacement key created doc 39eafb8c-11c6-5ecc-a8c9-aec323eaa67d, v0 79dc0bed-d71b-5a31-97d5-371c3c06d916, trajectory e5f85464-560b-5383-b199-cf4c62c12145, work 8d62ca55-cae2-5f9a-95e5-83c0245b3fb1, run 74a5a20f-24a3-4b25-b11c-1072f881f8a9; ChatGPT 401 iteration0; public cancel v3; 13 terminal/0 active; selfdev/policy exact; two keys revoked/post401; empty generic doc 457320df-e047-405c-b2a1-a0263b4cb5dc retained with version/revision count 0 as operator setup mistake", docs/evidence/continuous-texture-supervision-joined-runtime-review-2026-08-08.md, docs/evidence/continuous-texture-supervision-requirement-audit-2026-08-08.md, docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md, "independent wake-scope red-team ACCEPT at ac6dd16b", /tmp/choir-capsule-final-linux.test, "git:fbc7ff5a048ed58d0f6dd02ae8462ae211eca328 API-key stable-computer binding repair", "independent binding-security-review ACCEPT", "independent binding-route-audit ACCEPT", "CI/deploy 31326948312 success including selected Race, aggregate Go, differential SBOM, rolling publication, and Node B deploy", "public proxy /health exact fbc7ff5a build/deployed commit at 2026-08-09T18:09:46Z", "activation receipts exact fbc7ff5a for auth/proxy/vmctl/gateway/corpusd/maild/sourcecycled", "git:fc73805e registry reconciliation with docs CI 31329713521", "independent narrow-key-probe-review ACCEPT for mode-0600 ingestion, sequential pre-recovery proof, and DELETE-204/post-401 cleanup", "independent retained-recovery-ceremony-review ACCEPT for exact one-POST retained-computer recovery", "reproducible no-redirect fbc CLI verifier binary sha256:3511c9f66f80367e4c31db735cafc84cfb0f6ec31fe295b94d4cf0a5c1d1887c from two byte-equal builds; sole patch sha256:afaa760da2a15d30df0fd804aa4e335719f0f9b2bcf0aa07311bd82b29682041", "independent post-recovery-baseline-review ACCEPT for helper sha256:8835b245be4d4706faa156c0ee03ff5b3cb73b254b5362dcfef48a9962079970", "source-only fbc7ff5a product-contract audit: public lifecycle/Texture/source/run acceptance mapped; no mounted raw Trace product route or CLI", "independent acceptance-policy-ceremony-review ACCEPT: apply sha256:7520e02b294c445bfe3c5ea5d18db3f5e80549c015367006a719d01f8b7d0e4b; rollback sha256:fe752b17cde9ee2fdac7a2c47d86ac4b419db2367381303f5913bc0905d9c640"]
  blocker_or_risk: "Repair fbc7ff5a is deployed and source/CI identity-proved, and every immediate key/recovery helper has independent source ACCEPT, but staging bearer-path proof remains blocked because no usable key exists and exact retained-owner authentication requires ordinary headed persistent Chrome plus native Touch ID/security-key presence within the five-minute challenge TTL. The frozen one-POST payload remains NOT EXECUTED. Never inspect the clipboard before the owner explicitly replies copied. Separately, exact source audit proves fbc7ff5a lacks a mounted owner-scoped raw Trace product route, so final action 9 will require a problem-led source repair, new deployment identity, and public/CLI proof; derived summaries and internal reads are rejected. The prior aggregate load-only actorruntime SQLite failure remains documented and did not recur in selected CI. Effects remain OFF; historical key row and root-only backups remain terminal obligations."
  next_action: "Wait for exact retained-owner physical presence in ordinary headed persistent Chrome. The owner runs the frozen payload only after canonical /auth/session returns c72404bb-3c43-4a53-8671-b5cbc48b24a7, clicks Copy exact key, and replies only copied. Then run the accepted non-logging ingestion to mode 0600 and prove clipboard empty; run only the accepted exact fbc health -> retained lifecycle failed epoch8253 -> self-development off -> wrong-target 403 -> pre-recovery identity 503 -> unchanged lifecycle sequence. If it passes, run the independently accepted single-POST recovery with durable request marker and exact signed fbc host/guest/service/VM identity, then run the accepted GET-only baseline before any acceptance write. Continue only through independently reviewed provider-policy and public supervision ceremonies. The documented Trace gap requires a later minimal product-route/CLI repair and new exact deploy gate; never substitute internal evidence. Any STOP follows accepted cleanup; recovery is never retried. Effects remain OFF."
  policy_ceremony:
    mutation_class: orange_provider_routing_full_ceremony
    status: prepared_not_executed_independently_accepted
    conjecture_delta: "Exact base policy maps CoSuper and text verifier to DeepSeek deepseek-v4-flash, for which retained availability evidence is HTTP 402. Temporarily selecting the already product-proved ChatGPT gpt-5.5 for only those two roles should permit real assignment-bound capsule turns; only authenticated runs and capsule receipts can confirm it."
    protected_surfaces: [computer-owned model-policy bytes, provider/model selection, model-call metadata, capsule and run acceptance]
    admissible_evidence: "Fresh accepted post-recovery baseline; exact stable epoch and immutable-route digest before and after each write; raw byte GET/PUT/GET hashes; strict resolve contracts; real run/capsule receipts; exact final restoration."
    apply_artifact: "/tmp/cts-acceptance-model-policy-apply.py sha256:7520e02b294c445bfe3c5ea5d18db3f5e80549c015367006a719d01f8b7d0e4b"
    rollback_artifact: "/tmp/cts-acceptance-model-policy-rollback.py sha256:fe752b17cde9ee2fdac7a2c47d86ac4b419db2367381303f5913bc0905d9c640"
    temporary_policy_sha256: 6624bb112e805ff95974052a5fb808247bd81e5b9582ca81377b7b6297f40202
    rollback: "Persist exact original 1124 bytes at sha256 7192b8b1600561a331fda32f27628296c3f5b9bd1ba30dd5fb82681985c45e2a before the apply marker. On any post-marker STOP, rollback precedes key revocation/expiry and accepts marker-plus-backup recovery even without an apply PASS; exact temporary bytes restore, exact original bytes close without a second write, and foreign bytes stop."
    heresy_delta:
      discovered: "Base capsule roles select a provider with retained 402 evidence; raw Trace product route is absent."
      introduced: "At most one bounded temporary two-role policy difference; no credential, route, self-development, host, or second authority change."
      repaired: "None until original bytes and original role resolutions are re-proved."

receipts:
  - id: api-key-stable-computer-binding-deploy-v1
    boundary: implement
    commit_or_artifact: git:fbc7ff5a048ed58d0f6dd02ae8462ae211eca328
    proof_refs: ["independent binding-security-review ACCEPT", "independent binding-route-audit ACCEPT", "GitHub Actions run 31326948312 success", "public proxy /health exact fbc7ff5a", "selected-service activation receipts exact fbc7ff5a for auth/proxy/vmctl/gateway/corpusd/maild/sourcecycled"]
    rollback_ref: git:be8b32b7dfa5dd1656016deda75f4ff3aec0b80a
    disposition: "Source, focused/Race tests, selected aggregate CI, differential SBOM, rolling publication, Node B deployment, proxy public identity, and selected-service activation all pass. This closes implementation/deployment identity only. No usable bearer remains, so correct-target/wrong-target/no-effect and retained-guest acceptance are still open; direct payload authorized only with exact-owner physical presence and not executed; effects OFF."
    problem_ref: docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md#api-key-stable-computer-binding-authorization-gap--red-repair-authorized
    authorization_ref: docs checkpoint be8b32b7dfa5dd1656016deda75f4ff3aec0b80a and owner-ratified active Definition
    candidate_or_evidence_refs: [git:fbc7ff5a048ed58d0f6dd02ae8462ae211eca328, "CI/deploy 31326948312", "public health deployed_at 2026-08-09T18:09:46Z"]
    landing:
      source_commit: fbc7ff5a048ed58d0f6dd02ae8462ae211eca328
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31326948312
      deploy_ref: https://github.com/choir-hip/go-choir/actions/runs/31326948312
      environment_identity: "host selected-service identity exact fbc7ff5a; guest identity not yet proved"
      deployed_acceptance: "partial: source/CI/deploy identity only; bearer route selection, wrong-target no-effect, and retained guest acceptance pending exact-owner narrow key"
    registry_conformance_ref: "goal registries remain open; effects OFF; no acceptance entries closed"
  - id: continuous-supervision-runtime-landing-v1
    boundary: implement
    commit_or_artifact: git:ac6dd16b1980a1a3faedd7d1d83fefa79395a1ee
    proof_refs: [docs/evidence/continuous-texture-supervision-joined-runtime-review-2026-08-08.md, docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md, "GitHub Actions run 31261269488", "nonce-bound execution identity at VM epoch 8247", "public cancellation snapshot at lifecycle version 9", "local source-only rollback rehearsal: 99 paths exact cdaa787b, reverse patch sha256 64a61e5db159cf7d839532bad9a2a9d320e11a430e100a0ad6de2998b77530a8; deployed rollback still open at this historical receipt checkpoint"]
    rollback_ref: git:cdaa787bf2d006a1d4e59c1650a232f2083d8f9d
    disposition: "Independent exact wake-scope review ACCEPT; full selected CI and exact host/guest deployment succeeded. Boot recovery projected the stranded initial work, and public create/tell/cancel/model-policy surfaces executed. Deployed repeated supervision is blocked by protected provider credential/quota availability. The failed trajectory is durably cancelled, owner policy restored exactly, temporary key revoked, effects OFF, registries open."
    problem_ref: docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md#exact-candidate-provider-credential-failure--2026-08-08
    authorization_ref: owner-ratified active Definition landing loop
    candidate_or_evidence_refs: [git:ac6dd16b1980a1a3faedd7d1d83fefa79395a1ee, "independent lifecycle wake-scope ACCEPT at exact ac6dd16b", "execution identity guest computer-42850e9734d9442386c5dd8bf3afbf19 epoch 8247", "cancelled trajectory 8f3b6ac6-dbdf-5bfe-99f0-661961c64f3d"]
    landing:
      source_commit: ac6dd16b1980a1a3faedd7d1d83fefa79395a1ee
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31261269488
      deploy_ref: https://github.com/choir-hip/go-choir/actions/runs/31261269488
      environment_identity: "nonce-bound joined host/guest/platform identity exact ac6dd16b at VM epoch 8247"
      deployed_acceptance: "partial: create/initial-wake/owner-instruction/policy-rollback/public-cancellation and CLI read/history/revisions/exact-show/watch-resume/negative authority passed; provider-dependent supervision and positive source-open blocked"
    registry_conformance_ref: "all three continuous-supervision entries remain active/open; effects remain OFF pending provider restoration and full deployed acceptance"
  - id: continuous-supervision-canonical-rollback-rehearsal-v1
    boundary: implement
    commit_or_artifact: "R git:10d4865958b7d8deaab5665f74b37dd1b5005070; F git:67a61358ceda55c30e9853907f85648bb8531bb8"
    proof_refs: [docs/evidence/continuous-texture-supervision-joined-runtime-review-2026-08-08.md, docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md, "GitHub Actions R 31267448310", "GitHub Actions F 31268477380", "/tmp/cts-rf-before.json", "/tmp/cts-rf-midpoint.json", "/tmp/cts-rf-final.json"]
    rollback_ref: git:10d4865958b7d8deaab5665f74b37dd1b5005070
    disposition: "Canonical current-main R/F passed both normal landing loops. R exposed old-response identity ambiguity by omitting six stored owner-instruction request_id fields; fail-closed gate initiated F immediately. F restored the exact pre-R tree and all final captured frozen comparator response/state digests. Exposure was 1909 seconds; effects remained OFF; key revoked/post401. Provider-dependent acceptance remains open."
    problem_ref: docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md#canonical-current-main-rollback-rehearsal-and-old-response-identity-ambiguity--2026-08-08
    authorization_ref: "owner-ratified active Definition canonical R/F red ceremony; independent mechanical PROCEED"
    candidate_or_evidence_refs: [git:10d4865958b7d8deaab5665f74b37dd1b5005070, git:67a61358ceda55c30e9853907f85648bb8531bb8, "99-path digest 0b7eb4241e3dc5a70705ce596f436a372b5213593457c0c9b831c8c9296b22f3", "patch sha256 64a61e5db159cf7d839532bad9a2a9d320e11a430e100a0ad6de2998b77530a8"]
    landing:
      source_commit: 67a61358ceda55c30e9853907f85648bb8531bb8
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31268477380
      deploy_ref: https://github.com/choir-hip/go-choir/actions/runs/31268477380
      environment_identity: "nonce-bound joined F host/guest/platform identity on vm-bbdbbd01c4390b7036067aaa12afeb68, guest computer-42850e9734d9442386c5dd8bf3afbf19, epoch8249; R identity exact at epoch8248"
      deployed_acceptance: "canonical rollback/forward and final equality of all captured frozen comparator response/state digests passed; midpoint response identity ambiguity triggered immediate F; repeated provider-dependent supervision remains blocked"
    registry_conformance_ref: "Definition remains active; rollback evidence gap closed; effects OFF and provider/capsule/checkpoint/run-acceptance/terminal registry gates remain open"

  - id: continuous-supervision-definition-review-v1
    boundary: define
    commit_or_artifact: sha256:880c00cb8f7a6c546825689a7ba37c02a551f897a93080f5e1e2dbbda2cb89ed
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    rollback_ref: main@f64784e88b42abbf7d87fee058c989537b686d58
    disposition: Six REPAIR, two ACCEPT, one route failed before review; blockers adjudicated into candidate v2.
    problem_ref: this Definition observed_artifact
    authorization_ref: owner request to draft and review only
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: 460c142394e12b6e307949d0180da08d1b058745 observation only
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries

  - id: continuous-supervision-definition-review-v2
    boundary: define
    commit_or_artifact: git:6681606d4e0f14e83dab89bb808862db82cdd21b
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    rollback_ref: main@f64784e88b42abbf7d87fee058c989537b686d58
    disposition: Eight completed reviewers accepted the substantive repair; five REPAIR verdicts isolated an underspecified digest construction, three returned ACCEPT, and one route failed before review. Candidate v3 applies the exact byte-level identity repair.
    problem_ref: this Definition observed_artifact
    authorization_ref: owner request to draft and review only
    candidate_or_evidence_refs: [reviewed source commit 6681606d4e0f14e83dab89bb808862db82cdd21b, docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    landing:
      source_commit: 6681606d4e0f14e83dab89bb808862db82cdd21b
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31216848206 success
      deploy_ref: skipped_docs_only
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries


  - id: continuous-supervision-definition-review-v4
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4 (self-normalized digest 9db1c4397646142c28d1f85580ec91099a22c6340e20dd3740c36d4419373018) -> adjudicated into v4.1
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    rollback_ref: main@f64784e88b42abbf7d87fee058c989537b686d58
    disposition: Six completed reviewers REPAIR, one ACCEPT, one route returned no verdict, one route failed before review on a missing API key; blockers (read-only verification required vs optional, in-scope superseded one-slot capsule freeze, single source_ref guarantee, deletion-citer disposition, unnamed restart operation, stale reconciliation) adjudicated into candidate v4.1.
    problem_ref: this Definition observed_artifact-2 (the v4 delta had never been panel-reviewed by definition gate)
    authorization_ref: owner request to run consensus and promote on green
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@v4.1]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: b20bece30b408373d2844f5621fb9f91fc624d99
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries until green panel and owner ratification

  - id: continuous-supervision-definition-review-v4.1
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.1 (self-normalized digest 4a88d3637c657279370713308db7b7636835da05cb55d68ee1806cf0ef5c9727 verified by panel)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, .agentic-consensus/cts-v4.1-panel/]
    rollback_ref: main@f64784e88b42abbf7d87fee058c989537b686d58
    disposition: v4.1 re-panel (nine routes, Claude included) returned six REPAIR, one ACCEPT (OpenCode), Devin no verdict, Grok-45 failed on missing API key; blocking findings were Definition front-matter YAML parse failures, B3 still unresolved in Implement C.4, duplicated/mislabeled identity records in the evidence receipt, stale next_action candidate identity, and reconciliation inventory wording; all adjudicated and fixed into candidate v4.2 below.
    problem_ref: v4.1 candidate did not clear the green gate; repairs were applied under the parse/test-independence regime
    authorization_ref: definition review protocol; owner request to run consensus including Claude and promote on green
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@v4.2]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable; no runtime deploy occurred during this define-gate review
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries until green panel and owner ratification

  - id: continuous-supervision-definition-review-v4.2
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.2 (self-normalized digest 9e6cd9222c7585cd1f64e44fd6ff2842413e4849b3446a95a3799b350c5c16a2)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, .agentic-consensus/cts-v4.2-panel/]
    rollback_ref: main@f64784e88b42abbf7d87fee058c989537b686d58
    disposition: "v4.2 identity re-panel (nine routes, Claude included) returned REPAIR on receipt integrity: six recorded v4.1 block hashes computed over ANSI-stripped text did not match raw bytes, and the v4 gate output identity block was absent. All deliberative architecture, parser, and registry checks passed."
    problem_ref: v4.2 gate identity receipt integrity (raw-bytes requirement)
    authorization_ref: v4.2 re-panel authorization; definition review protocol
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@v4.3]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries

  - id: continuous-supervision-definition-review-v4.3
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.3 (self-normalized digest 58db42d21ae631557e4f55797c2f3646f5560dc34cf6b1c21aa650c0d3204344)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, .agentic-consensus/cts-v4.3-panel/]
    rollback_ref: main@f64784e88b42abbf7d87fee058c989537b686d58
    disposition: "v4.3 identity re-re-panel (nine routes, Claude included) confirmed all 22 identity hashes and the digest fixed point, and flagged four repair items detailed in the evidence."
    problem_ref: v4.3 gate receipt-integrity chain (orphaned digest restatement in evidence, missing v4.2/v4.3 receipts, non-chronological ordering, stale now.slice)
    authorization_ref: v4.3 re-review authorization; definition review protocol
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@v4.4]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries until green panel and owner ratification

  - id: continuous-supervision-definition-review-v4.4-1
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.4 (first re-freeze; raw .out bytes of this gate were overwritten in place by the v4.4-2 run, so no identity block can be built retroactively — recorded as a receipt-integrity data loss)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    rollback_ref: main@b20bece30b408373d2844f5621fb9f91fc624d99
    disposition: "v4.4-1 gate (nine routes, Claude included) returned REPAIR: 65-hex v4.3 receipt digest, wrong-case docs/Evidence path in the v4.3 receipt, corrupted-word typo in ACTIVE.md, missing registry_conformance_ref on the v4.1 receipt; all four were adjudicated and repaired (the ACTIVE.md edit was itself defect-ridden and had truncated the file, which the v4.4-2 gate caught)."
    problem_ref: v4.4 first re-freeze receipt integrity
    authorization_ref: v4.4 re-freeze authorization; definition review protocol
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@v4.5]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries until green panel and owner ratification

  - id: continuous-supervision-definition-review-v4.4-2
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.4 fixed (digest e89b73d007b4245b48a071eb227d4ac0cf063be0f602c475e260236523a66e16; see evidence identity block)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, .agentic-consensus/cts-v4.4-panel/]
    rollback_ref: main@b20bece30b408373d2844f5621fb9f91fc624d99
    disposition: "v4.4-2 re-check gate (nine routes, Claude included) returned REPAIR (Claude, Cursor, GPT-5.6-Sol, Codex) against ACCEPT (Gemini, DeepSeek, OpenCode): observed_at not current, v4.3 panel tally misstated (five REPAIR/one ACCEPT not six), and the v4.4-1 gate had no receipt; all adjudicated into candidate v4.5 (this Definition)."
    problem_ref: v4.4-2 gate receipt integrity (observed_at currency, v4.3 tally, unreceipted v4.4-1)
    authorization_ref: v4.4-2 re-check authorization; definition review protocol
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@v4.5]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries until green panel and owner ratification

  - id: continuous-supervision-definition-review-v4.5-1
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.5 (self-normalized digest 76aa485a596399e9dd509a686d49669c67b6d4b097780cc778928994b6b69df1)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, .agentic-consensus/cts-v4.5-panel/previous-round]
    rollback_ref: main@b20bece30b408373d2844f5621fb9f91fc624d99
    disposition: "v4.5-1 gate (nine routes, Claude included) reviewed a stale snapshot (the prompt text then pointed at the prior v4.4 snapshot in error); its four REPAIR findings were all satisfied by the live v4.5 bytes: observed_at, v4.3 tally, v4.4-2 tally, comma fix. The run is preserved in 'cts-v4.5-panel/previous-round' for identity review; its outputs were regenerated by the v4.5-2 gate."
    problem_ref: stale-snapshot pointer in the v4.5-1 prompt
    authorization_ref: v4.5 re-freeze authorization; definition review protocol
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@v4.6]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries until green panel and owner ratification

  - id: continuous-supervision-definition-review-v4.5-2
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.5 (self-normalized digest 76aa485a596399e9dd509a686d49669c67b6d4b097780cc778928994b6b69df1)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, .agentic-consensus/cts-v4.5-panel/]
    rollback_ref: main@b20bece30b408373d2844f5621fb9f91fc624d99
    disposition: "v4.5-2 gate (eight ok including Devin, one failed: Claude, Codex, Cursor, GPT-5.6-Sol, DeepSeek REPAIR; Gemini, OpenCode ACCEPT; Devin no verdict; Grok-45 failed) returned four hygiene findings (trailing-space scalars in the Definition, v4.3 'seven completed verdicts' noun error, v4.4-2 'seven ok' undercount vs its manifest with eight ok rows, and a reviewer-list comma corruption) plus one non-blocking ACTIVE adjudication label still naming v4.4; adjudicated into candidate v4.6 (see evidence identities)."
    problem_ref: "v4.5 gate hygiene: trailing-space, tally nouns, list comma, ACTIVE label"
    authorization_ref: v4.5 re-review authorization; definition review protocol
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@v4.6]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries until green panel and owner ratification

  - id: continuous-supervision-definition-review-v4.6-1
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.6 (self-normalized digest 38fcbb5654c71f857db658220e92e8ab8284cd58c9be30a78bb9b42d9f6326de)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    rollback_ref: main@b20bece30b408373d2844f5621fb9f91fc624d99
    disposition: "v4.6-1 gate (nine routes, Claude included) reviewed the v4.6 candidate; three REPAIR blockers (next_action hex literal contradicting candidate.digest, 37-hex rollback on v4.5-2 receipt, 'green snapshot' typo). All three fixed immediately; the run's raw outputs were overwritten in place by the same-directory v4.6-2 run, so the identity block cannot be built (recorded process defect, same class as v4.4-1)."
    problem_ref: "v4.6-1 output overwrite: no identity block recordable"
    authorization_ref: v4.6 re-check authorization; definition review protocol
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@v4.7]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries until green panel and owner ratification

  - id: continuous-supervision-definition-review-v4.6-2
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.6 (self-normalized digest 66392a6d827fde7754f4ba1a8f8e431f7cc8bb69bbd5367fce5c58d0ac4d3bea)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, .agentic-consensus/cts-v4.6-panel/]
    rollback_ref: main@b20bece30b408373d2844f5621fb9f91fc624d99
    disposition: "v4.6-2 gate (nine routes, Claude included) returned REPAIR (Claude, Codex, Cursor, GPT-5.6-Sol) / ACCEPT (DeepSeek, Gemini) with Devin and OpenCode delivering no verdict, Grok-45 failed on missing API key: confirmed the v4.5-2 gate Codex verdict REPAIR not ACCEPT, corrected the OpenCode misreport (no verdict), and left no byte/identity defects beyond the receipt prose. Adjudicated into candidate v4.7."
    problem_ref: "v4.5-2 gate miscount (Codex) + v4.6-2 OpenCode misreport"
    authorization_ref: v4.6-2 re-review authorization; definition review protocol
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@v4.7]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries until green panel and owner ratification


  - id: continuous-supervision-definition-review-v4.7-1
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.7 (self-normalized digest 2f6de3e16fe02e46962278aaf6c36251605f0db3fca153d3773ea8f41231333f)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, .agentic-consensus/cts-v4.7-panel/]
    rollback_ref: main@b20bece30b408373d2844f5621fb9f91fc624d99
    disposition: "v4.7-1 gate (nine routes, Claude included; Claude spent monthly limit so delivered no verdict, Devin ok no verdict, DeepSeek/OpenCode timed out, Grok-45 failed; REPAIR from Codex, Cursor, Gemini, GPT-5.6-Sol) found receipt-integrity defects: v4.6-2 receipt corrupt digest + 41-hex rollback, missing reviewer commas, evidence corruption and stale 66/66 identity count, OpenCode miscoded ACCEPT (no verdict in raw output). All adjudicated into the v4.7 final bytes; re-panel scheduled as v4.7-2."
    problem_ref: "v4.6-2 receipt corruption + evidence stale count + OpenCode miscode"
    authorization_ref: v4.7 re-review authorization; definition review protocol
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@v4.7]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries during the v4.7-1 gate; no promotion occurred

  - id: continuous-supervision-definition-review-v4.7-2
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.7 (self-normalized digest b189b070caaa25bfbd9b0aa12eb7c79a0f47508ec0d95616da36296408cdf94c)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, .agentic-consensus/cts-v4.7-2-panel/]
    rollback_ref: main@b20bece30b408373d2844f5621fb9f91fc624d99
    disposition: "v4.7-2 gate returned four REPAIR (Codex, Cursor, GPT-5.6-Sol, DeepSeek) and one ACCEPT (Gemini); Devin returned no verdict, OpenCode timed out, Grok-45 failed, and Claude was excluded. It found missing receipt fields, boundary/prose corruption, the regressed v4.4-2 composition count, a missing v4.7-1 evidence section, and an unreceipted current freeze. All mechanical findings are adjudicated in the final Define boundary."
    problem_ref: "v4.7-2 receipt integrity, evidence-ledger gap, and composition-count regression"
    authorization_ref: "definition review protocol; owner directive to clean ledger gaps and proceed"
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@continuous-texture-supervision-definition-v5]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries during the v4.7-2 gate; no promotion occurred

  - id: continuous-supervision-definition-review-v4.7-3
    boundary: define
    commit_or_artifact: not_applicable working-tree candidate v4.7 (self-normalized digest 5aeaa97c3d72bd20067ffc7ce6eb7b95df2c5f637adf48c8c2ee460d819b4d40)
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, .agentic-consensus/cts-v4.7-3-panel/, scripts/lint/cts-receipt-lint.py]
    rollback_ref: main@b20bece30b408373d2844f5621fb9f91fc624d99
    disposition: "v4.7-3 gate returned four REPAIR (Codex, Cursor, GPT-5.6-Sol, DeepSeek) and one ACCEPT (Gemini); Devin and OpenCode returned no verdict, Grok-45 failed, and Claude was excluded. All enumerated parser, digest, identity, receipt-schema, hygiene, registry, and composition checks passed. REPAIR findings were stale green-silence wording, missing v4.7-1/v4.7-2 ledger entries, and omitted inventory/disposition of the new receipt linter; the final Define boundary repairs them without changing the ratified architecture."
    problem_ref: "v4.7-3 stale current-state claim, evidence-ledger gap, and untracked linter inventory"
    authorization_ref: "definition review protocol; owner directive to clean ledger gaps and proceed"
    candidate_or_evidence_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, this Definition@continuous-texture-supervision-definition-v5, scripts/lint/cts-receipt-lint.py]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: draft blocked/non-executable in all three registries during the v4.7-3 gate; no promotion occurred

  - id: continuous-supervision-owner-ratification-and-promotion
    boundary: define
    commit_or_artifact: this executable Definition at candidate.digest
    proof_refs: [docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md, scripts/lint/cts-receipt-lint.py, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
    rollback_ref: main@b20bece30b408373d2844f5621fb9f91fc624d99
    disposition: "Owner ratified the direction-specific lifecycle control, atomic progressive Texture turn, many capability-bound CoSupers, mandatory independently identified verification result from a writable isolated capsule, single canonical source_ref transclusion projection, and automatable Texture API/CLI contract; corrected the verifier model so it may edit and run tests/scripts inside its capsule; accepted the mechanically repaired ledger; and directed promotion without another panel. The Definition is executable, not complete: runtime behavior remains to be implemented and accepted on staging."
    problem_ref: docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md
    authorization_ref: "Owner directive in this run: proceed without asking and promote the repaired Definition as executable."
    candidate_or_evidence_refs: [this Definition@continuous-texture-supervision-definition-v5, docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md]
    landing:
      source_commit: docs-only/yellow promotion commit containing candidate.digest
      ci_ref: docs truth and repository contract checks required
      deploy_ref: not_applicable; no runtime behavior changed
      environment_identity: not_applicable; staging identity remains observational baseline only
      deployed_acceptance: not_applicable until the red implementation boundary lands
    registry_conformance_ref: "docs/ACTIVE.md Active Definition; docs/mission-graph.yaml active mission_orchestrator entrypoint; docs/doc-authority-manifest.yaml active executable definition authority"

  - id: continuous-supervision-implementation-inventory
    boundary: define
    commit_or_artifact: git:7467b678cfb2a92906434158fb2a18124f3ee0c3
    proof_refs: [docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md]
    rollback_ref: main@d40a711f67144b4e1c41e3155e4d79975682dec1
    disposition: "Read-only production-caller inventory found the exact A–C substrate boundary plus new structured-normalization, owner-correction, durable-stream, production-capsule-wiring, verifier-effect, and registry-root problems. Commits a5308e87 and 7467b678 landed the problem-first receipts without runtime repair; the current docs candidate repairs the deleted citer and registry root before red mutation."
    problem_ref: docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md
    authorization_ref: this active owner-ratified Definition and AGENTS.md problem-documentation-first invariant
    candidate_or_evidence_refs: [docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md, main@7467b678cfb2a92906434158fb2a18124f3ee0c3]
    landing:
      source_commit: 7467b678cfb2a92906434158fb2a18124f3ee0c3
      ci_ref: docs truth pending; live pre-flight failure is recorded in the problem receipt
      deploy_ref: skipped_docs_only
      environment_identity: 6965f7f71f764f91737b21804bc376281cbdbe8f observation only
      deployed_acceptance: not_applicable
    registry_conformance_ref: "live doccheck failure recorded before repair: one product mission-graph entrypoint versus zero authority-root product Definitions"

  - id: continuous-supervision-preparatory-substrate
    boundary: implement
    commit_or_artifact: "local unpublished commits 7ce06244, e3342476, 020ff64d"
    proof_refs: [internal/agentprofile/agentprofile_test.go, internal/agentcore/prompts_policy_test.go, internal/agentcore/tool_profiles_authority_test.go, internal/store/lifecycle_test.go, docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md]
    rollback_ref: main@cdaa787bf2d006a1d4e59c1650a232f2083d8f9d
    disposition: "Separated spawn from message policy without widening Texture spawning; made delegated CoSuper host-effect installers unreachable while leaving capsule-local mutation unwired; and connected Start/apply lifecycle revisions to the canonical structured normalization authority with derived readable content and mismatch refusal. Focused, full-store, and race evidence passed. These are necessary substrate repairs, not an A–E or product-loop completion claim; the commits remain local until the joined red candidate is ready."
    problem_ref: docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md
    authorization_ref: "owner-ratified active Definition; frozen Implement A-C/D safety boundaries"
    candidate_or_evidence_refs: [git:7ce06244c5c05ba20ac189a71489103aba22d78b, git:e33424764e8785445a8558aa73cf8909eba0e083, git:020ff64dfcb9212ebd2c894cd5ba317909d2ef7d, docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md]
    landing:
      source_commit: pending joined runtime candidate
      ci_ref: pending
      deploy_ref: pending
      environment_identity: staging baseline 6965f7f71f764f91737b21804bc376281cbdbe8f only
      deployed_acceptance: pending
    registry_conformance_ref: "not a Definition status transition; active registries remain current and live doccheck passed at cdaa787b"

  - id: continuous-supervision-joined-runtime-candidate
    boundary: implement
    commit_or_artifact: git:363bf39398128fa0e1a85a19ae9a7762f92ba3dc
    proof_refs: [docs/evidence/continuous-texture-supervision-joined-runtime-review-2026-08-08.md, docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md, internal/store/lifecycle_control_delivery_test.go, internal/store/lifecycle_cancellation_intent_test.go, internal/agentcore/lifecycle_control_injection_test.go, internal/agentcore/cosuper_assignment_late_receipt_test.go, internal/capsule/revocation_receipt_linux_test.go, internal/textureowner/texture_observation_test.go]
    rollback_ref: main@cdaa787bf2d006a1d4e59c1650a232f2083d8f9d
    disposition: "Joined direction/address authority, atomic Texture turns, progressive observation/API/CLI, lossless paginated exact-run delivery, authenticated-memory consumption and report settlement, same-run append recovery, late Super evidence, complete owner-head rebase, one public version, assignment-only capsule tools, immutable subject/candidate and exact execution receipts, durable cancellation intent/effect/ack/restart, cancellation-wins late evidence, Linux orphan cleanup authority, resident provider drain, and bounded terminal-report closure. Independent lifecycle and capsule/security red teams returned ACCEPT with no critical/high source finding. Full Store, runtime shards, focused race suites, supporting packages, receipt lint, and Linux capsule cross-compile passed. This is reviewed local source, not deployed acceptance."
    problem_ref: docs/problems/continuous-texture-supervision-implementation-inventory-2026-08-08.md
    authorization_ref: "owner-ratified active Definition; red-mutation ceremony; problem-documentation-first receipts"
    candidate_or_evidence_refs: [git:363bf39398128fa0e1a85a19ae9a7762f92ba3dc, docs/evidence/continuous-texture-supervision-joined-runtime-review-2026-08-08.md, /tmp/choir-capsule-final-linux.test]
    landing:
      source_commit: pending documentation-bearing candidate commit
      ci_ref: pending
      deploy_ref: pending
      environment_identity: staging baseline 6965f7f71f764f91737b21804bc376281cbdbe8f only
      deployed_acceptance: pending
    registry_conformance_ref: "Definition remains active in all three registries; effects OFF; no completion or promotion claim before deployed proof"

view:
  path: http://127.0.0.1:8787/
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-continuous-texture-supervision-2026-08-07.md --serve 127.0.0.1:8787 --watch"
  generator_version: definition-dashboard-js/v1
  authority: "This local dashboard is a non-editable, non-authoritative projection for owner supervision while Texture itself is being repaired. The Markdown/YAML Definition remains the sole mission authority; dashboard health is not acceptance or completion evidence."
  lifecycle: "The /goal executor must launch this generator as a supervised long-running process at startup, keep it live through terminal closure, and stop it on completion, blocked_incomplete, or supersession. It is session-local and must not become a service, repository artifact, second mission state, or product supervision API."
---

# Continuous Texture Supervision

## Problem and cause

The durable loop is only half connected. Researcher, Super, and CoSuper packets
can reach Texture and cause immutable semantic revisions. Texture cannot use the
same accepted lifecycle authority to redirect a bound Researcher or assign work
to the one persistent Super. Merely registering `update_coagent` would expose an
unsafe generic resolver, not repair the loop.

The missing edge came from a correct deletion with an incomplete replacement.
The old Super request was a same-channel cast that bypassed trajectories and was
removed during the durable-lifecycle cutover. The typed packet, work-item,
actor-wake, persistent-Super, and capsule pieces now exist, but their authority
boundaries do not join. This mission connects those pieces; it does not restore
the deleted channel cast or build another supervision platform.

## Target capability contract

| Caller | May create | May address | Must never do |
|---|---|---|---|
| Texture | Researcher work; one persistent-Super work obligation through a typed opener | Researchers already bound to its document/trajectory; exact `super:<owner>` | Spawn Super/CoSuper, address CoSuper, mutate host/capsule, or smuggle commands into canonical prose |
| Researcher | Nothing | Requesting `texture:<doc_id>` | Execute, write, spawn, or address arbitrary Texture/Super identities |
| Super | Researcher and assigned implementation/verifier CoSuper work | Requesting Texture and its own children through bound work | Shell/write host state, execute capsule commands, accept/materialize/route |
| Implementation CoSuper | Nothing | Owning Super/Texture result path through bound work | Host execution, another capsule, acceptance/route, verifier verdict |
| Verification CoSuper | Nothing | Owning Super/Texture result path through bound work | Mutate host or another capsule, alter the frozen subject under the same verification identity, self-development effect, acceptance/route |

Spawn authority and message-address authority remain separate. Texture gaining a
persistent-Super address must not add Super to `AllowedDelegateTargets`.

## Owner-facing document and automation contract

Texture's visible artifact is ordinary writing, normally coherent prose. The
document may be a research report, essay, investigation, proposal,
correspondence, publication draft, or technical explanation. Its storage schema
must not force headings, lists, status blocks, work-item inventories, or a
coding-specific report shape. Structure serves editing and evidence placement;
it does not dictate rhetoric.

Texture publishes new immutable versions when owner-relevant understanding
changes, including while delegated work remains open. A long trajectory may
produce many useful versions before its final version. Texture may coalesce a
burst of redundant inputs, but it may not hide substantive interim learning
until settlement.

One generic source reference/transclusion binds exact research quotations, file
or code excerpts, diffs, patches, commands and terminal output, test runs,
images, and later multimedia into the prose. Source kind selects presentation;
it does not select a different workflow or authority path. An expanded item is
not copied model prose: its body placement resolves to an immutable source
identity, version, selector, and content hash, and the full permitted source can
be opened through the product.

The existing Texture API and `choir texture` CLI must let an authenticated owner
or automated verifier perform the same five product actions: create a Texture;
tell or correct it; watch successive versions; show an exact current or
historical version; and open the exact source behind a transclusion. Machine
mode returns stable JSON or JSONL identities and watch resumes from a durable
cursor. Human prose remains the artifact, never the test protocol.

## Proposed substrate cutover

The new path remains in the existing lifecycle worker-update object, event, and
snapshot authority, but it does not overload one work relationship. Existing
`QueueLifecycleUpdate` retains upward producer-report semantics:
`producer_work_item_id` is open and assigned to the authenticated caller, and
only that producer may report its disposition. A direction-specific lifecycle
control command carries a runtime-derived `target_work_item_id`; the exact target
owns that open obligation, and the sender cannot settle it. Direction is derived
from caller, target, packet kind, and work bindings, never model-authored.

The persistent Super remains the existing non-lifecycle `super:<owner>` actor,
scoped by owner and computer. It may be admitted only as that exact lifecycle
control target/assignee; it is not promoted into a generic lifecycle actor
class. Its reconciler reads pending lifecycle controls by target identity.
Pre-cutover worker-inbox rows remain compatibility input for old non-lifecycle work, but
new Texture traffic never writes or marks delivery there.

A Texture-only typed Super-work opener creates or reuses target work for the
exact persistent Super and queues `kind=execution_request` with validated
actions. A continuation can address only existing target work for a bound
Researcher or that Super. These are operation variants over the atomic Texture
transition below, not independently committing calls and not additions to
`AllowedDelegateTargets`. The current immediate generic `update_coagent` is not
exposed to Texture.

One Texture-specific conditional lifecycle apply commits expected lifecycle
version and document head plus exactly one semantic outcome:

1. a semantic revision;
2. an explicit no-semantic-change disposition;
3. a deliberate wait; or
4. a durable block.

A semantic revision contains one canonical structured document. Its readable
prose and machine-readable transclusion manifest are deterministic projections,
not independently writable copies. Generic source references and expanded
transclusions resolve and pin before the revision becomes canonical.

The same object-graph batch commits inbound dispositions, target-work open/reuse,
zero or more validated ordered outbound controls, source objects and references,
events, and command receipt. The ordered outbound set and target-work bindings
participate in its replay digest. `patch_texture`, `rewrite_texture`, the Texture
form of `update_coagent`, the typed Super opener, and public owner correction are
affordance/validation views over this apply; none queues, wakes, or reports
durable success independently. Remove the synthetic self-queue-before-apply
convention. Wake targets only after commit; restart sweep recovers committed
packets if wake delivery fails. The activation may remain terminal afterward,
preserving stale-base safety without fake revisions or escaped directives.

## Execution sequence

### Define — freeze authority before code

1. Preserve the frozen v1 digest and durable panel receipt; adjudicate every
   blocking finding into this repaired candidate.
2. Freeze the repaired self-normalized digest and obtain owner ratification for
   its direction-specific work semantics, atomic apply, and exact capsule-local
   evidence boundary.
3. Only then promote the Definition through all three registries. The docs-only
   Define commit is the required problem-documentation-first checkpoint.
4. If the direction-specific lifecycle command or writable verification-capsule effect boundary
   is infeasible, stop and revise this Definition; do not improvise a dual-write,
   generic lifecycle actor migration, broad refusal deletion, or smaller artifact.

### Observe — keep the local Definition dashboard live

1. At `/goal` startup, launch `view.generator` as a supervised long-running
   process and confirm `view.path` reports a current projection before Implement
   A begins.
2. Keep it live and watching this Definition through terminal closure so the
   owner can supervise revisions, current action, evidence, dirty paths, and
   blockers while Texture itself remains the artifact under repair.
3. The dashboard is read-only and non-authoritative. Never infer acceptance,
   completion, product health, or mutation authority from dashboard health, and
   never copy its ephemeral session log into mission state.

### Implement A — make addressing mechanically safe

1. Separate spawn policy from message-address policy in `internal/agentprofile`;
   Texture remains spawn-Researcher-only.
2. Make every target lookup error a hard refusal. Delete channel/self inference
   from Texture authority; reject non-lifecycle Texture, cross-document/
   trajectory/owner/computer targets, arbitrary Super, and every CoSuper target.
3. Freeze the role/slot/target negative matrix and direction-specific packet
   identities before registering any Texture affordance.
4. Land target validation, atomic lifecycle control, and readers in the same
   candidate. Texture must not temporarily receive the immediate generic tool or
   a pre-cutover persistent-Super fallback.

### Implement B — join the lifecycle inboxes

1. Preserve producer-report `QueueLifecycleUpdate`; add a distinct control
   command in the same reducer/object/event/snapshot authority with exact target
   work and no sender-owned target disposition.
2. Admit bound Researcher and exact owner/computer persistent Super as control
   targets/assignees without making Super a generic lifecycle actor.
3. Make Researcher, Texture, and persistent Super warm/cold reconciliation read
   pending lifecycle state and reconstruct it after restart.
4. The Texture Super opener atomically opens/reuses target work and queues one
   execution request. Equal retry produces one packet/work/activation; conflicting
   payload or ordered-control digest fails.
5. Retain pre-cutover-inbox readers only for identified pre-cutover rows; add replay fixtures
   and a detector proving new lifecycle traffic cannot enter them.

### Implement C — close the semantic control transaction

1. Extend the existing source-aware canonical Texture write with one narrow
   Texture-turn transition; the current helper name is not architecture, and
   atomicity must not be simulated with independently acknowledged tool batches.
2. Commit revision/no-change/wait/block, inbound dispositions, target-work
   changes, ordered controls, source objects/references, events, and receipt
   under one expected-head CAS.
3. Make the structured document canonical. Derive readable content and the
   normalized transclusion manifest from it; reject any supplied projection that
   disagrees rather than retaining two document truths.
4. Expanded transclusion reuses the one `source_ref` node; `display_mode:
   expanded_ref` selects quotation, code, diff, terminal, test, image, or later
   multimedia rendering on the same node. Preserve the exact source version,
   selector, hash, and open path; no second node type or independently writable
   source-embed truth exists.
5. Replace required-write behavior with required durable transition. Silent
   no-op remains forbidden; redundant evidence receives a disposition without a
   fake version. Substantive interim learning may publish before work settles.
6. Preserve owner semantics: direct revisions commit canonical `AuthorUser` head
   and correction cursor/obligation atomically; `/revise` queues a lifecycle
   decision request for a Texture-authored revision. Coalesce wakes, not edits.
7. Update prompts only after the capability exists. Prefer coherent prose and
   follow owner-requested form without role choreography, forced headings,
   first-tool forcing, or unavailable-effects claims.

### Implement D — connect Super and capsule-only CoSuper work

1. Make persistent Super consume target-bound lifecycle execution requests,
   emit intermediate typed updates, and preserve one owner/computer identity.
2. Do not broadly remove the lifecycle-profile refusal. Add only the exact
   persistent-Super -> assigned CoSuper path after the authority matrix passes.
3. Permit many concurrent CoSuper assignments subject to computer resource and
   capability policy. The safe default is one writable assignment per isolated,
   run-bound disposable capsule; two writable CoSupers share a capsule only
   under an explicit coordination contract that owns file-race behavior.
4. Bind every assignment and retry to exact owner, computer, trajectory, parent
   Super decision, work item, attempt, scope digest, capability digest, and
   capsule when writable. Cancellation and late-result dispositions remain
   assignment-specific.
5. Every implementation or verification CoSuper receives only its assigned
   capsule mutation capability. A verification assignment may edit files,
   generate test fixtures or harnesses, and run builds, tests, and scripts inside
   its own writable networkless capsule. It is not a read-only role.
6. Bind a verification assignment to the immutable subject digest it evaluates.
   Its typed result records commands, outputs, relevant capsule mutations, and
   verdict. If it changes subject bytes, that output is a newly identified
   candidate requiring its own verification; it cannot certify the original.
7. No CoSuper may call `record_self_development_verification` or gain a
   ComputerEventAppender/updater-root/effect-proposal/finalization, acceptance,
   materialization, checkpoint, route, VM, host-path, or owner-decision effect.
   If writable capsule isolation from those effects is impossible, stop and move
   that slice to a separately ratified successor without weakening the artifact.
8. Dispose the historical same-channel request citers in the same landing
   boundary: repair the conjecture/assertion-ledger entry (A1 no longer routes
   through `request_super_execution`), disposition the dead `super_requested`
   checkpoint in `internal/agentcore/run_acceptance.go` to the new direction
   opener, and re-point fixtures at the typed lifecycle receipt. This satisfies
   the deletion-citer standing question without restoring the removed tool.

### Implement E — expose the progressive Texture through API and CLI

1. Extend the existing public Texture API rather than creating a supervision
   service. Give each owner instruction/correction a durable identity even when
   one resident Texture actor handles many requests.
2. Make document observation durable and resumable. Version events expose cursor,
   revision, parent, version, working/terminal state, causal request/update
   identities, and transclusion identities without exposing raw actor chatter.
3. Extend `choir texture` with create, tell/correct, watch, show exact version,
   and open-source behavior. Default automation output is stable JSON; watch uses
   JSONL and resumes after its last durable cursor.
4. Keep deterministic verification on the real product contract: direct
   owner-authored fixture revisions may supply a canonical structured document
   and source objects, while agentic acceptance uses the real Texture actor. Add
   no test-only route and require no browser prose scraping or fixed sleeps.

### Land — prove the product loop

1. Freeze and independently review the complete runtime candidate, including
   fail-closed authority, conditional batch, generic prose/transclusion
   projection, resumable API/CLI observation, replay, private payload, owner
   correction, many-CoSuper capsule isolation, cancellation, and no-effect
   checks.
2. Run focused/race tests and the applicable full CI shards once.
3. Commit, push `origin/main`, monitor CI and staging deployment, and verify the
   exact deployed commit through `/health`.
4. Run the full authenticated repeated-cycle acceptance. Prove an informative
   transclusion-grounded Texture version while work remains open; a later
   correction and changed direction; at least two parallel writable CoSuper
   capsules; at least one independently identified verification CoSuper result from a writable networkless capsule, including test/script evidence; pending packets across passivation and
   no-SSH same-build process restart; in-flight cancellation plus actual late
   result/retry; and before/after protected-state comparison.
5. Run the public API/CLI acceptance: create, tell, watch, disconnect/resume,
   show current and historical versions, and open exact research and execution
   transclusions. Run continuous-prose and differently structured writing cases
   through the same schema.
6. Fetch exact document/revision/source, owner/computer/trajectory,
   producer/target work, actor, request/command/update/digest, run, capsule,
   cancellation/late, and acceptance identities; inspect every acceptance
   Texture version against its machine-readable receipts.
7. Close the Definition and registries only after the artifact is fetched,
   effects remain unchanged, and rollback is rehearsed.

## Red-mutation ceremony

**Conjecture delta:** C6/C7 gain a deployed test of durable actors supervising
through trajectory/work authority; they remain active conjectures rather than
being promoted by one success. The proposal removes one H011/H015/H018 instance
without claiming those heresy classes globally repaired.

**Protected surfaces:** canonical Texture writes and owner heads; lifecycle
reducers, conditional object-graph batch, replay, and pre-cutover compatibility;
actor delivery/restart; encrypted private payload and audit; fail-closed
role/target/work authority; persistent Super identity; capsule slot, handle, and
network namespace; canonical computer events, updater state, cancellation/late
results, run acceptance, provider calls, and deployment routing.

**Admissible evidence:** deterministic direction/authority negatives,
conditional batch/race/replay and same-build restart proofs, deterministic
document projection and generic transclusion contracts, frozen-candidate
independent review, exact CI/deploy identity, authenticated staging trajectory
with an informative owner-visible revision while work remains open, later owner
correction, parallel isolated CoSuper work, exact research and execution source
opening, resumable API/CLI observation, actual late result, protected-state
before/after receipts, fetched artifacts/IDs, owner-visible Texture inspection,
and rollback receipt.

**Rollback:** additive compatibility plus source revert. New packets remain
pending/late and inspectable; they are never marked delivered merely because an
older runtime cannot execute them. No speculative capsule effect can cross into
materialization, checkpoint, route, or host state.

**Heresy delta:** discovered — lifecycle-addressable versus pre-cutover worker-inbox split, absent Texture tool,
fail-open/self-target/cross-document resolution, producer/target work ambiguity,
two-commit Texture mutation, terminal redirection gap, forced no-op revisions,
split owner correction, and verifier self-development effects. Introduced — none
by this Definition. Repaired — none until deployed acceptance proves the cutover.


### 2026-08-09 deployed Researcher bind-loop repair ceremony

**Mutation class:** red. This change touches lifecycle activation/control
delivery and the durable actor acknowledgement boundary. Effects remain OFF.

**Structural/root-cause clustering assessment:** the opaque packet rejection,
pre-commit mutation terminalization, generic Texture cancellation exposure,
recursive assignment-field exposure, and this post-commit bind loop are five
symptoms in the same Texture-to-lifecycle control substrate. The first four are
already repaired at shared schema/retry/policy boundaries. The new symptom is
not another schema patch: atomic Texture commit and wake are correct, but the
Researcher cold-wake hydrator crosses back into the deliberately legacy-only
`ListWorkItemsByTrajectory` projection. That projection must continue excluding
`LifecycleVersion > 0` rows. The exact existing replacement is the already
validated control's unique `TargetWorkItemID` read through scoped
`GetLifecycleWorkItem` (or its exact trajectory snapshot); the computer-wide
`ListOpenAssignedLifecycleWorkItems` boot scan is explicitly not request-path
authority. Separately, the actor substrate intentionally retries handler errors
at least once. A random activation plus a best-effort terminal write violates
the handler's idempotence contract and turns one canonical control into
unbounded new runs.

**Conjecture delta:** the exact `lifecycle invalid transition` occurs at
`lifecycleRunBindsWork`, not at run-state validation: a v1 open assigned work is
present in canonical state, while the legacy list intentionally returns no
lifecycle work, so the pending run omits `work_item_ids`. Hydrating only the
ordered/deduplicated work IDs named by the already selected controls, with exact
owner/computer/trajectory/agent/open checks, should let the existing activation
and delivery reducers commit exactly once. The second conjecture is that two distinct runtime-derived keys can make
pre-dispatch reconciliation idempotent across actor retry, `MarkProcessed`
failure, deployment, and process restart: a build-independent logical
activation key names the exact owner/computer/trajectory/agent and ordered
control/work join, while a failed-attempt key additionally includes immutable
build commit plus canonical control/work lifecycle versions. Replay always
reuses/rebinds an active logical activation across builds; only an exact
same-build/same-state durably failed attempt is suppressed, so a changed build
or changed canonical lifecycle state can recover the still-pending control. If
either conjecture fails, no provider/capsule effect may run.

**Protected surfaces:** lifecycle-scoped agent/run/work/control object graph,
activation replacement, atomic control delivery/event cursor, actor durable
mailbox acknowledgement/retry, restart recovery, provider activation boundary,
and public cancellation evidence. The legacy work projection, broad boot scan,
generic cancellation, persistent-Super authority, capsules, host/VM,
checkpoint, route, materialization, acceptance, and provider credentials must
not be widened.

**Authorized repair boundary:**

1. Keep the legacy non-lifecycle branch unchanged. For lifecycle Researcher
   controls, hydrate only unique `TargetWorkItemID`s from the already selected
   controls via the exact scoped getter and revalidate lifecycle version,
   owner, computer, trajectory, assigned agent, and open status. Mixed scope,
   closed/reassigned work, or any ambiguity refuses before run/provider creation.
2. Stamp two runtime-derived keys. The logical activation key contains the
   exact owner/computer/trajectory/agent and ordered/deduplicated control/work
   joins but no build identity. Before minting a run, reuse that active logical
   key across any deployment: an unbound pending run is rebound, and a bound
   pending run is deterministically redispatched. The failed-attempt key adds
   immutable build commit and the exact canonical control/work lifecycle
   versions (or content hashes). Only a matching durably failed attempt is a
   typed handled outcome; changed build or canonical version remains eligible
   to recover without authoring a second run while the old logical activation
   is active. `coagent_result` actor update IDs must also be deterministic from
   their canonical update identity so repeated dispatch does not create a
   second mailbox fact.
3. Construct that typed outcome only for classified
   `ErrLifecycleInvalidTransition` after `UpdateRun` successfully persists the
   failed run and its fingerprint. A failed terminal write, CAS/context/store
   error, or other unclassified bind error remains unprocessed and retryable
   against the same active fingerprint; it must not mint a fresh run. If atomic
   bind committed but the post-commit reload failed, the nonempty lifecycle
   receipt forbids stale terminal overwrite; replay reloads the canonical bound
   run and deterministically dispatches it.
4. Actor acknowledgement remains a separate write, so correctness comes from
   durable fingerprint replay rather than a Go error type: a crash or
   `MarkProcessed` failure after terminal persistence re-observes the same
   failed fingerprint and creates zero additional runs. Pending control/work
   remain canonical evidence and may be retried by a changed build or settled by
   public cancellation; a failed RunRecord never settles them by itself.

**Admissible evidence:** (1) retain the authority test proving the legacy
projection excludes lifecycle rows; (2) real atomic Texture open-Researcher
control through cold `ReconcileCoagentWake`, proving exact scoped work metadata,
one delivery receipt, one run, and no bind-failure marker before provider; (3)
negatives for wrong owner/computer/trajectory/agent, lifecycle version,
closed/reassigned work, mixed trajectory/target, ordering/dedup, and legacy
compatibility, with zero delivery/provider/capsule effects; (4) deterministic
bind rejection plus successful terminal persistence, injected terminal-persist
failure, transient bind error, cancellation race, bind-commit/reload-failure
classification, and terminal-write-to-actor-ack failure, proving bounded run
identity and no stale overwrite; (5) same-build restart/replay with no additional
failed run; old-build active pending/unbound followed by a new build reusing and
rebinding the same run; hydration-to-bind reassignment/version race followed by
valid canonical recovery; changed-build/changed-state failed-attempt eligibility;
and successful-bind restart with no second delivery; (6) focused Race tests,
sequential runtime shards, vet,
doc/receipt checks, and independent lifecycle/security review; then exact
CI/SBOM/deploy identity and authenticated staging recovery on a fresh trajectory.

**Rollback:** source revert of exact hydration, fingerprint reconciliation, and
deterministic actor dispatch, followed by normal exact deployment rollback.
Pending controls/open work and failed runs remain canonical and inspectable.
Rollback must never revive cancelled `14c99be0…` or revert restored provider auth.

**Heresy delta:** discovered — a stale legacy projection was used as lifecycle
activation authority; random pre-dispatch activation violated actor idempotence;
and a best-effort run terminal write was treated as indefinitely retryable.
Introduced — none intended; acknowledging a transient/unpersisted error,
suppressing changed-build recovery, or hydrating unrelated work would each be a
new loss-of-delivery/authority heresy and is explicitly forbidden. Repaired —
none until focused fault/restart tests, independent review, exact deployed
identity, and authenticated product proof pass.

### 2026-08-09 exact-deployed Texture mailbox-starvation checkpoint

**Mutation class:** red for any eventual runtime repair; this checkpoint itself
is green documentation. Effects remain OFF.

**Exact landing evidence:** `b5d907a34d0250d966e026d13f64f6d575a4423d`
passed every job in CI/deploy run `31298004583`, including the fresh
attempt-bound Differential SBOM acceptance. Nonce-bound product identity joined
that exact commit across host, guest, every service, deployment receipt, and
platform attestation on stable VM `vm-bbdbbd01c4390b7036067aaa12afeb68`,
guest `computer-42850e9734d9442386c5dd8bf3afbf19`, epoch 8252, route digest
`sha256:648d6071215206b190376ff6c24f3c93c08483b09bfb2ffc4790c00f3dd66489`.

**Conjecture delta:** the prior activation conjecture is promoted for the exact
product boundary exercised: lifecycle Researcher work hydrated exactly,
control `ddc49c43…` bound/delivered before provider entry, and thirteen clean
Researcher provider runs produced real source packets without any
`lifecycle_control_bind_failed` marker. Full C6/C7 remains unpromoted. New
conjectures are that producer-report and owner-correction occurrences are not
being delivered into the resident Texture actor, and that open Researcher work
reactivates without target-side acknowledgement/backpressure. The two may share
one actor-mailbox/reconciliation substrate but are not yet claimed to share a
root cause.

**Protected surfaces:** resident Texture actor memory and wake, canonical
producer-report and owner-instruction delivery/disposition, lifecycle work
settlement and Researcher reactivation, provider execution, cancellation,
canonical Texture revisions, persistent-Super opening, and no-effect/run
acceptance.

**Admissible evidence for diagnosis/repair:** exact actor occurrence and memory
identities for Texture run `f6eed29e…`; the fifteen retained producer reports
and queued correction `owner-instruction-99bd6f13…`; deterministic proof of
whether each occurrence was enqueued, processed, or omitted; same-build
passivation/restart reproduction; a bounded open-Researcher backpressure test;
no acknowledgement before canonical Texture incorporation/disposition; focused
Race and fault tests; independent lifecycle/security review; full CI/SBOM;
exact deployment identity; then a fresh authenticated trajectory proving v2,
focused follow-up, persistent Super, parallel capsules, source opening, and all
remaining acceptance gates.

**Rollback:** no repair exists yet. Public cancellation
`cts-b5-final-mailbox-stall-cancel-*` is the current safe rollback and retained
trajectory `8f7a926f…` at lifecycle v21/seq23 proves it: exact v1 terminal head,
two cancelled work items, sixteen cancelled stored updates, no active agent
runs, and no capsule/protected effects. A future source change uses normal
origin/main revert, exact redeployment, and public trajectory cancellation;
never fabricate update acknowledgement or auto-completion.

**Heresy delta:** repaired — the deployed pre-provider Researcher bind loop;
discovered — resident Texture mailbox starvation and open-worker retry
amplification after successful provider evidence; introduced attribution is
unknown pending read-only convergence. No code edit may precede the committed
problem-first receipt.

### 2026-08-09 Texture resume and bounded-continuation repair ceremony

Read-only source convergence after problem receipt `4095b17d` proves two joined
actor-substrate failures and identifies existing replacements.

The target starvation occurs after the first valid report wake. A passivated
Texture run is correctly reactivated in canonical Store state, but
`reactivatePassivatedTextureRun` calls exported `ActivateRun`, which emits the
same deterministic `initial_dispatch` occurrence keyed by the same run ID as the
already processed first activation. The SQLite actor log correctly deduplicates
that one-shot occurrence, leaving the run projected `pending` with no executable
mailbox fact. The Texture-specific `coagent_result` handler then returns nil and
acknowledges each distinct report/correction wake after owner reconciliation;
later reconciliation sees the apparently valid pending mutation and returns
early. The runtime injectors for exact producer packets and owner instructions
therefore never execute. The deployed shape — f6 pending, v1 unchanged, fifteen
pending reports, queued correction — is the exact signature of this seam.

The replacement is already present immediately below the Texture-specific fork:
the role-uniform `coagent_result` resume path executes an exact reconciled run
synchronously in the current actor goroutine and returns durable memory state.
Texture already has stronger canonical document/head/mutation validation than
generic memory. Rewarm must connect that validation to the current occurrence;
it must not redispatch the one-shot initial occurrence. Cold/new Texture run
creation retains its one unique initial dispatch. Boot recovery must dispatch
canonical pending report/instruction occurrences before projecting any
passivated Texture run pending.

A separate old continuation synthesizer amplifies the starvation.
`continueOpenLifecycleWorkAfterTerminal`, added to preserve durable obligations,
interprets one open work item as standing authority to mint an unlimited series
of fresh `terminal_activation_work_recovery` runs after each provider
completion. Pending open producer reports provide no backpressure, and
`StepBudget`/`TokenBudget` are prompt-only. This behavior is explicitly
normalized by an existing test, but it conflicts with the newer exact control,
passivated-memory, and actor-occurrence substrate: durable open work records
responsibility, not continuously runnable demand. A later exact Texture control
is the authority to resume more Researcher work; Texture can atomically
incorporate/reject a report, settle the producer work, or keep it open while
issuing a focused follow-up.

Mutation class is **red**. Conjecture delta: executing a canonically reactivated
Texture run inside the current unprocessed report/correction actor occurrence,
plus deleting no-control Researcher continuation synthesis, should make every
committed wake either produce durable authenticated run memory and an atomic
Texture disposition or remain unprocessed/retryable. One control then authorizes
one bounded Researcher activation chain until a target-visible report, not
unlimited sequential provider calls. Protected surfaces are actor log identity
and acknowledgement, Texture document/head/mutation authority, run memory,
producer-report/owner-instruction injection and disposition, lifecycle work and
control delivery, restart recovery, provider calls, cancellation, and
no-effect/run acceptance.

Authorized repair boundary:

1. Preserve deterministic one-shot `initial_dispatch` identity for cold/new
   runs. For a revision-bound Texture run reactivated from passivated memory,
   remove the duplicate `ActivateRun` bridge. Return an explicit reactivation
   decision to the current Texture `coagent_result` handler, revalidate exact
   owner/computer/document/head/mutation/run authority, execute synchronously in
   that actor handler, and persist/return only the resulting actor memory state.
   Never trust a generic/stale Texture run ID or execute before canonical
   reactivation CAS.
2. Boot recovery enumerates exact pending Texture producer reports and owner
   instructions and dispatches their canonical actor occurrences; duplicates
   deduplicate in the actor log. It does not first turn a passivated Texture run
   into pending through an already-used initial occurrence. Initial owner work
   with no prior run may still create one cold run and one initial dispatch.
3. Delete the `ActivateRun` wrapper if no production caller remains. Delete
   `continueOpenLifecycleWorkAfterTerminal`, both completion call sites, and the
   test contract that demands generic successor runs. In the Researcher branch
   of the broad boot work scan, pending exact controls may use the scoped
   control/fingerprint reconciler; open work with no pending control remains
   durably open and idle. Do not mint a no-control run from recency, generic
   work scan, terminal completion, or provider result.
4. Preserve all reducer authority: producer reports remain pending until a
   Texture turn explicitly incorporates/rejects them; owner instructions remain
   pending until bound to an exact head and applied; open work is neither
   acknowledged nor auto-completed. A later exact control may resume the same
   parked Researcher or create only the canonically authorized activation.
   Cancellation and late evidence remain Store-owned.

Admissible evidence is a full Adapter plus real SQLite-log test in which an
initial Texture run writes v1/passivates and the same run consumes a producer
report into durable runtime memory and v2; the equivalent owner-correction test;
crash/restart cuts before actor append, after append, and after reactivation CAS;
coalesced ordered reports plus correction; negatives for stale generic memory,
wrong head/scope/run/mutation, failed injection append, and no-write turns; and a
bounded Researcher contract proving one open report yields zero generic
successor/provider call across repeated reconcile and restart until a new exact
control arrives. Existing control-bind/restart/cancellation/Race matrices,
independent lifecycle/security review, full CI/SBOM, exact deployment identity,
and fresh end-to-end acceptance remain required.

Rollback is a normal origin/main revert and exact redeployment followed by public
cancellation of any affected trajectory. Pending reports, instructions, work,
and controls remain canonical and inspectable. Never randomize initial IDs, add
polling, mark evidence delivered before run-memory append/Texture turn, restore
generic Researcher continuation, widen cancellation deadlines, or auto-complete
work.

Heresy delta at ceremony: discovered — a duplicate one-shot dispatch was treated
as reactivation authority, distinct committed wakes were acknowledged against a
non-executing pending projection, and open work was treated as unlimited
provider authority. Introduced — none intended. Repaired — none until exact
fault/restart tests, independent review, full landing loop, and authenticated
product proof pass.

### Independent-red-review rejection and tightened authority boundary

The first ceremony review is **REJECT**, so implementation remains unauthorized
until these gaps are incorporated. The following constraints supersede any
looser reading above.

**Occurrence authority and acknowledgement fate-sharing.** A received actor
wake is not authority merely because the target has some pending backlog. Define
two injective, versioned, length-prefixed canonical occurrence identities:

- producer report: exact owner, computer, trajectory, target Texture agent,
  producer agent, update id, producer-update id, producer work id, lifecycle
  version, and reducer/message sequence from the pending Store row;
- owner instruction: exact owner, computer, trajectory, document, target Texture
  agent/work, instruction id, request id, head revision, kind, lifecycle version,
  and reducer sequence from the pending Store row.

Live queue and boot recovery use the same identity and complete actor envelope,
including trajectory and an explicit authenticated owner source for owner
instructions rather than empty/random fields. Before reactivation and again
before provider entry, the handler reloads the exact canonical trigger row and
joins owner, computer, live trajectory/no cancellation intent, document/current
head, Texture agent, open Texture work, run, mutation/revision, occurrence
version, and pending fate. Wrong, stale, foreign, terminal, cancelled, late, or
already disposed occurrences are typed zero-provider outcomes; they never select
global backlog or authorize a write.

A handler may report success to the actor log only after the exact triggering
occurrence is durably appended to authenticated run memory or is atomically
incorporated, rejected, applied, cancelled, or classified late in Store. An
injection, run-state, mutation, provider, no-write, or persistence error while the
trigger remains pending returns an error or durably passivates plus appends a
new explicit retry occurrence before acknowledgement; it may not terminalize the
only resume authority. Expose a checked cold-start dispatch result: projecting a
new run is insufficient until its unique initial occurrence append succeeds, or
the current trigger remains unprocessed. `ExecuteActivationSync` needs an
explicit outcome/postcondition rather than a void success assumption.

Because pre-fix deployments may already contain `processed actor row + canonical
pending row + no authenticated run-memory receipt`, add one deterministic,
versioned recovery occurrence derived from the exact pending Store row and its
canonical target run/memory state. It is not a random retry and cannot collide
with the processed legacy occurrence. Recovery must first prove the absence of
an admissible run-memory/disposition receipt, and repeated boot/reconcile must
deduplicate it. This is a migration for retained stranded facts, not a second
mailbox authority.

**Central Researcher provider admission.** Deleting the immediate completion
hook is necessary but insufficient. Inventory every production caller of
`reconcileAssignedWorkItemActor*`, `sweepPassivatedSpawnedCoagentWork`, broad
open-work sweep, interrupted activation rewarm, start/replay, and provider
entry. Enforce once at the provider boundary: a lifecycle Researcher may execute
only when its exact run is joined to a current pending-or-canonically-delivered
Texture control fingerprint and exact owner/computer/live trajectory/Researcher
agent/open-work versions. Open work alone, generic recency, a terminal provider
result, a passivated spawned-work scan, or a stale pending/running projection is
never provider authority. Preserve separately identified legacy non-lifecycle
behavior. Existing lifecycle Researcher work without a control remains explicitly
idle and inspectable, eligible only for cancellation or a later exact Texture
control; it is not silently settled.

**Expanded cross-ledger fault proof.** Add cuts at canonical trigger commit before
actor append; actor append before handler; reactivation CAS/run update before
execute; authenticated memory append; provider response and any tool result;
atomic Texture-turn commit; handler return; SQLite `MarkProcessed` failure; and
snapshot persistence. At every cut, restart must prove at most one canonical
head/turn/disposition, exact pending or typed terminal trigger fate, no stale
memory authority, and a measured bound on unavoidable provider replay. Counting
provider tests must show one exact Researcher control cannot become two
authorized provider chains across handler retry/restart/cancellation; open work
alone produces zero calls, while a later exact control resumes safely.

The negative matrix includes wrong owner, computer, trajectory, document, head,
Texture/producer agent, target/producer work, run, mutation, update/instruction
id, lifecycle/reducer version, cancellation intent, terminal trajectory, and
late result, each with zero provider, Texture write, control, capsule, or
protected effect. Coalesced reports plus correction must each have one ordered
run-memory/disposition/actor-ack fate. No code edit begins until independent
review accepts this tightened ceremony and the docs-only authority commit is
pushed.

### Final actor-ack and no-write clarification

A second lifecycle review remains **REJECT** until the cross-ledger snapshot gap
is explicit. The repair therefore adopts canonical replay rather than assuming
SQLite processed state and actor snapshot update atomically.

Actor snapshot is never occurrence, run, or lifecycle authority. After handler
return, a crash may persist `MarkProcessed` before `SaveSnapshot`. Boot and live
reconciliation must join the canonical pending trigger/disposition with exact
Store-owned `RunMemory` entries and the canonical Texture run/mutation/head. A
versioned recovery occurrence incorporates the exact trigger id/version, run id,
latest authenticated run-memory tail identity, and current canonical head/turn
identity. If the trigger is still pending, it resumes the exact canonically
validated run even when the old actor row is processed and the actor snapshot is
missing or stale; if the trigger was atomically disposed/applied/cancelled/late,
it produces a typed zero-provider terminal outcome. Repeated recovery of the
same joined state deduplicates, while an advanced run-memory/head/disposition
state yields the next exact identity. The handler must also return an explicit
resume-now decision for a run already projected pending by a crash after
mutation CAS or `UpdateRun`; it may not early-return merely because that pending
projection validates.

Authenticated run memory proves model visibility, but it does not by itself
settle or acknowledge canonical evidence. A successful Texture activation
triggered by producer report or owner instruction must atomically name the exact
trigger in its Texture-turn update/instruction disposition, or commit an explicit
Store-owned waiting/blocked transition that includes a deterministic successor
wake identity. A no-write result, a Texture turn omitting the current trigger,
or a failed injection/persistence cannot return actor success; the triggering
row remains unprocessed/retryable and no lifecycle update/instruction is marked
consumed. Required-write/decision tooling must enforce this postcondition rather
than rely only on prompt compliance.

The required fault matrix now explicitly includes handler return,
`MarkProcessed` success/failure, crash before `SaveSnapshot`, stale/missing
snapshot with present run memory, and crash after mutation CAS/`UpdateRun` with
the run already pending. Each must recover the same exact run/trigger without a
second canonical Texture turn or an unauthorized provider chain.
