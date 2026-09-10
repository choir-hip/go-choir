---
definition_version: 2
definition_id: choir-rlm-versioned-rename-2026-09-09
execution_mode: draft_non_executable

start:
  captured_at: "2026-09-09T23:29:36Z"
  source:
    canonical_ref: "main@34629502"
    deploy_identity: "staging https://choir.news ok via proxy 0475ed84; mission-0 and mission-1 both completed with deployed proof (precondition for mission 2 satisfied)"
  worktree_inventory:
    status: reconciling
    evidence_ref: "2026-09-09 read-only git status; 4 untracked leftover paths preserved as unrelated WIP"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition will own only the versioned-rename surfaces named at charter."
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty
      class: user_wip
      owner: owner-and-session
      touch: read_only
      paths_or_digest: "docs/reports/choir-rlm-restore-zero-completion-report-2026-09-09.md, scripts/generate_restore_zero_completion_pdf_2026_09_09.py, scripts/__pycache__/, tmp/ — owner unknown, unrelated WIP, preserve read-only leave-in-place; fresh reconciliation required before charter."
      recovery: leave_in_place
  candidates:
    - id: none
      ref: none
      base: none
      scope: []
      disposition: none
  observed_artifact:
    - claim: "No executable V2 desk vocabulary exists; the desired vocabulary lives only as architecture text. The spawn path accepts engineering as a raw authorization synonym and ReduceCellIntents persists the raw in.Role, both mechanically visible; a mechanical persistence census beyond these two sites pins the exact live V1 write surface at charter."
      evidence_ref: "docs/designs/rlm-target-architecture-2026-09-04.md:79-85; internal/agentcore/rlm_reduce.go:76-89,180-187 (mechanically verified at current tree; re-pin at charter); persistence census beyond these sites re-pins at charter"
    - claim: "Live alias normalization runs in two general canonicalizers before any version-selected decode: agentprofile.Canonical and modelpolicy.NormalizeRole both accept legacy aliases on the live path."
      evidence_ref: "internal/agentprofile/agentprofile.go:115-139; internal/modelpolicy/model_policy.go:172-186; re-pin at charter SHA"
    - claim: "Mission-0's vocabulary_version seam gates ProjectionBase descriptors only; it does not select a V1/V2 tape or content role decoder."
      evidence_ref: "internal/projectionbase/types.go:22-33,63-113; internal/projectionbase/verify.go:17-48"
    - claim: "ProjectionBatch V1/V2 is an independent format discriminator, not a vocabulary version; it must not be confused with the absent vocabulary V2."
      evidence_ref: "internal/computerevent/projection_batch.go:11-21,157-215"
    - claim: "V1 role-bearing fields span twelve classes with scout-claimed file anchors: the eleven mission-state classes plus Yaegi session profiles as the twelfth executable vocabulary. All anchors are scout-sourced and unverified against any pinned SHA; charter re-pins every site."
      evidence_ref: "scout_unverified surface map 2026-09-09, re-pin at charter; docs/reports/choir-rlm-mission-state-2026-09-08.md rename section"
    - claim: "Mission 0 and mission 1 are both completed with deployed proof; the carried mission order names versioned rename next with Engineering as first executable carrier proof."
      evidence_ref: "docs/definitions/choir-rlm-restore-zero-2026-09-08.md now.status completed; docs/definitions/choir-rlm-settlement-gate-2026-09-09.md now.status completed; docs/reports/choir-rlm-mission-state-2026-09-08.md carried mission order"
  unknowns:
    - "V1 field sites beyond the scout-mapped anchors; the charter freezes the exhaustive inventory."
  start_correction:
    - date: "2026-09-09"
      note: "Single opening receipt main@34629502 retained; worktree_inventory.status reads reconciling until the four leftover paths are individually named at charter; claims 1-2 mechanically verified at review HEADs with line drift noted, re-pin at charter; claim 5 stays scout-unverified until the inventory census lands as the first charter Define."
finish:
  deliver: "Versioned rename is one durable truth per vocabulary version: V1 tape decodes under frozen V1 rules with original bytes preserved, V2 writers emit only the new desks vocabulary, unknown live values reject at activation, and the exhaustive V1 field inventory exists as a frozen artifact. Rename-first with owner-settled scope (revised 2026-09-10): every in-scope V1 desk-name occurrence renames (super, co-super/cosuper, researcher, live aliases, ID prefixes, agent addresses, prompt paths, digest inputs carrying desk tokens); version-tagged digest domains and the mission-1 v1 classification receipt stay immutable, with a successor v2 identity/mapping receipt recording token, prefix, and digest-input equivalence. No bundling, no alias period, no rename-last."
  artifact: "A frozen V1 field inventory artifact plus the version-selected decode/encode/activation contract across tape replay, writers, validators, policies, prompts, grants, and role-bearing persistence, with the Engineering-named desk as the first vocabulary proof vehicle and a deployed staging proof."
  entrypoints:
    implementation:
      - "internal/agentprofile/agentprofile.go (canonical profiles, Canonical normalizer, PolicyFor)"
      - "internal/capsule/roles.go (broker roles and verb sets)"
      - "internal/promptstore/store.go and defaults (role registry, strict validation, prompt files)"
      - "internal/modelpolicy/model_policy.go (role selection, NormalizeRole)"
      - "internal/agentcore/rlm_reduce.go (FromRole, desk routing, spawn synonyms)"
      - "internal/computerevent/event.go, http_client.go, appender.go, projection_batch.go (V1 envelope, tape decode, replay dispatcher)"
      - "internal/projectionbase/types.go, publisher.go, verify.go, rebuilder.go (vocabulary_version seam extension)"
      - "internal/types/task.go, cosuper_assignment.go, evidence.go (V1 role-bearing schema)"
      - "internal/computerevent/payload_resolver.go (payload fetch; charter records the negative sweep if payload bytes carry no role-bearing content)"
      - "frontend/src/lib/TextureEditor.svelte (model-policy role list, source-panel role selection)"
      - "internal/yaegikernel/profiles.go (Yaegi session profiles, GetProfile)"
      - "internal/store/cosuper_assignment_seed.go and internal/capsule/capability.go (seed roles, capability verbs)"
      - "internal/objectgraph/object.go (edge kinds and metadata)"
      - "internal/autoputer/run.go, internal/agentcore/rematerialize.go, restore_base.go (boot replay dispatch)"
      - "internal/agentcore/cosuper_assignment_runtime.go, internal/store/cosuper_assignments.go, internal/store/lifecycle.go, internal/store/store.go, internal/toolregistry/batch_executor.go, internal/coagentowner/spawn_tool.go, internal/runtimeprompts/overlays/, internal/agentcore/channel_store.go, internal/agentcore/runtime.go, tool_profiles.go, super_controller.go, api.go, tools_worker_update.go, tools_coagent.go, researcher_checkpoint_fallback.go (activation, admission, equality, and channel sites under the refusal item)"
      - "internal/platform/checkpoints.go, internal/agentcore/self_development_decision_binding.go (exact ActorProfile consumers); internal/autoputer/file_sync.go, internal/agentcore/self_development_materializer.go, internal/agentcore/texture_audit.go, internal/agentcore/tools_capsule.go (event ActorProfile writers); internal/platform/event_artifacts.go, internal/platform/event_replay.go, internal/store/computer_events.go, internal/projectionbase/source.go, internal/actorruntime/, internal/textureowner/value_helpers.go, internal/modelcatalog/catalog.go (checkpoint, replay, catalog, and texture value paths)"
      - "internal/textureowner/controller, handler, route, and coagent-route cluster (Canonical authorization sites); internal/store/lifecycle_control_delivery.go; internal/store/graph_store.go:913 join site; internal/agentcore/api_self_development.go and internal/agentcore/projection_tape.go raw ActorProfile writers"
  acceptance:
    - action: "Produce the exhaustive frozen V1 field inventory artifact in three phases: (a) the frozen artifact covering all twelve classes, enumerated here as the frozen partition — (1) agent profiles and canonicalizers, (2) capsule roles and verb sets, (3) Yaegi session profiles, (4) reducer and spawn aliases, (5) prompt roles and prompt file paths, (6) model policies and role selection, (7) persisted runs and lifecycle records, (8) assignments and work-item authority profiles, (9) mailbox destinations and agent addresses, (10) agent-ID prefixes and object-graph edge metadata, (11) grants, digest inputs, and evidence identifiers, (12) event envelope actor profiles and replay dispatch — with PayloadRef.Role payload-kinds classified payload-kind out of desk scope and TextureEditor SOURCE_PANEL_MODEL_ROLES classified live protocol under class 6, never display-only; (b) migrate and revert every inventoried row class with the inverse proven on staging; (c) a serving fence admitting no live spawn, admission, or persistence until (b) is green. Exhaustiveness proof is a standing CI gate frozen at charter consisting of four joined censuses over all non-test production Go, YAML, Svelte, SQL, embedded prompts, and generated and runtime prompt inputs: (1) typed and serialized fields capable of carrying a desk token, desk-derived agent identifier, or authority profile; (2) every writer into those fields; (3) every reader, equality check, authorization check, prefix parser, digest input, and projection decoder; (4) lexeme-anchored literal and prefix matching. Each inventory row records file:line, symbol, field path, semantic namespace, V1 token set, V2 token, version carrier, live-or-history classification, writers, consumers, digest consequence, forward migration, and inverse. Generic message roles stay outside the gate. Unknown or unanchored sites fail the inventory."
      proves: "The rename scope is bounded by evidence, not by assumption."
      evidence_class: local_test
    - action: "Add version-selected frozen V1 decode before the general canonicalizers, carried by explicitly named per-class version carriers extending the mission-0 vocabulary_version seam: charter names one carrier per class (tape event field, mailbox and channel row columns, run, lifecycle, assignment, and grant row columns, prompt and profile registry tags, payload envelope markers where role-bearing, computer-owned model-policy TOML overlays; classes with no carrier use unmarked-defaults-V1 only by charter-relative exception with negative sweep evidence), with unmarked tape, content, and persistence rows defaulting V1. Unknown role keys in computer-owned TOML refuse at parse instead of falling through to model defaults. ProjectionBase descriptors keep mission-0 refuse-missing/unknown and are never re-admitted as legacy; IsKnownVocabularyVersion widens to the explicit set {v1, v2} while CurrentVocabularyVersion advances to v2, so a v1-stamped base remains installable and replays through the frozen V1 decoder while a missing or out-of-set version still refuses; acceptance requires a pre-cutover v1 base descriptor installing and verifying on post-cutover source, and a post-cutover restore falling back to genesis replay is a mission-0 regression and a completion blocker. Commit ordering: the never-revert commit widens IsKnownVocabularyVersion to {v1, v2} only, and CurrentVocabularyVersion advances to v2 in a later revertible mission commit; rollback never reverts the known-set widening and always reverts Current if writers revert to V1. Base rebuild migrates inside the scratch database before publish, never stamping v2 over V1-named rows. Restore binds migration to the restore transaction: a successful post-cutover restore from a v1-stamped base runs forward-migration before the restored computer serves live authority, so no V1-stamped row ever holds live authority on a fenced computer. Frozen V1 decode runs at interpretation, never by rewriting envelope or batch bytes before digest: V1 tape bytes are identity-preserving, frozen decode interprets without rewriting, replay and v1-base install deposit V1 names into SQL, and a separate idempotent forward-migration stamps those rows v2. Adding the omitempty event vocabulary field must leave the digest of a fixture historic V1 event unchanged. Vocabulary stamp and digest domain are two independent stamps never read for each other: migrated rows keep their v1-computed digests and verify under the successor mapping receipt, and no verifier recomputes a migrated row under v2. Live projection rows forward-migrate to V2 names under the frozen mapping; V1 activation via decode serves replay verification only, never live authority."
      proves: "Tape bytes never change under replay; live rows migrate to V2 names only under the frozen mapping with a proven inverse, never silently."
      evidence_class: local_test
    - action: "Cut every live writer to the new desks vocabulary only, with the frozen V2 live set enumerated at charter: management, engineering, research (lowercase wire tokens) plus the canonical-stays-live profiles texture, conductor, processor, reconciler, email, and verifier roles, each separated from its retiring aliases. Freeze per-function V1 acceptor sets; never union them into one Canonical alias class. Canonical V1 maps {cosuper, co-super, coagent, co-agent} to co-super (engineering is not a Canonical alias; it passthroughs today). NormalizeRole extras {co_super, cosuper_coding, co-super-coding} plus overlapping Canonical keys map to co-super (engineering is not in this set). spawnRoleAllowed holds {co-super, cosuper, engineering} as spawn synonyms only. Yaegi registry keys are cosuper and researcher (V2 keys become engineering and research). V2 live canonical is engineering. Migration maps co-super, cosuper, and the Canonical plus NormalizeRole aliases to engineering under the frozen map. Persisted raw V1 engineering is census-classified before mapping; default classification refuses to elevate a passthrough or spawn-only token into Engineering-desk authority. Research aliases freeze per function identically (Canonical set plus NormalizeRole subset, pinned separately). Texture, processor, reconciler, email, verifier, and conductor alias sets enumerated identically (each canonical plus every alias branch) and classified per alias as decode-only with zero live acceptance. Freeze two sets, not one: the live-alias retirement set above, and the persisted V1 token set from the charter persistence census of tape, SQL, mailbox, grant, run, and agent rows. engineering is a V1 synonym and the V2 canonical string at once; live-authority eligibility is the row's version stamp after migration, never token shape, so a V1-stamped engineering row is unmigrated until stamped v2 and the identity-map engineering to engineering still requires the v2 stamp. Canonical EventKind wire values are frozen as-is — researcher_update keeps its V1 string in both V1 and V2 events because it is a digest input over the immutable chain, and any EventKind rename is explicitly out of scope. Frozen as implementation and protocol identifiers, never desk vocabulary: EventKind values; command-identity, admission-recovery, and idempotency prefixes; version-tagged digest domains; SQL table and index names; Go type, function, and constant identifiers, filenames, and package paths; object-graph kinds; tool names; PayloadRef payload-kinds; product app ids. Desk tokens inside identity strings follow the frozen-with-token tie-break: run-super-assignment and work-super-assignment style constructed identifiers rename with the desk token while bare command-identity prefixes stay byte-stable, adjudicated per site in the frozen inventory. All other rename migration covers ID prefixes, agent addresses, prompt paths, and digest inputs carrying desk tokens with per-site equivalence proof; a successor v2 identity/mapping receipt records the equivalence — enumerating at minimum the AssignedAgentID prefix, the AgentProfile carried in evidence, and the role in spawn requests — while the mission-1 v1 receipt remains immutable. Grant, policy, and prompt surfaces validate the new vocabulary. Model-authored cell source, report prose, and Texture bodies are out of scope."
      proves: "New truth is written once, in one vocabulary."
      evidence_class: local_test
    - action: "Reject unknown live values at activation: any role or desk value outside the version-selected vocabulary fails closed before spawn, admission, or persistence. Refusal covers the agentprofile.Canonical default passthrough (fail-closed Canonical returns empty string plus a typed unknown-profile error for unknown live values, never the input token: existing Canonical-not-empty sites then fail closed, while returning a non-empty unknown token would keep authorizing callers that ignore the error; every Canonical, PolicyFor, and CanSpawn caller checks the error, and a non-empty unknown token is a writer-purity failure), the PolicyFor default policy, the NormalizeRole default, grant/assignment/run equality sites, prompt normalizePromptRole, Yaegi GetProfile, toolCallSpawnProfile role/profile reads, the capsule RoleVerbSets constants rename, and the ReduceCellIntents raw in.Role and scope.FromRole persistence path. Named slips closed here, not by file-list coverage: HandleModelPolicyResolve empty-role to conductor default is retired so empty is typed missing and unknown and V1 names refuse; the production Yaegi and broker gate is capsule AgentRole into the session worker role into RoleVerbSets, not GetProfile; parsePolicy and parseOverlay unknown roles keys refuse at parse instead of NormalizeRole-and-store. Non-desk ActorProfile tokens owner and trusted-core are frozen protocol identifiers, never desk vocabulary: fail-closed Canonical and activation must not refuse them. Empty input is typed missing (preserving the toolCallSpawnProfile Profile-to-Role fallback), distinct from unknown. PayloadRef.Role and RunMemoryEntryProjection.Role stay outside this gate. Enforcement sits at central live persistence boundaries: every live ComputerEventAppender append entrypoint centrally requires an explicit V2 vocabulary version and validates ActorProfile against its classified actor namespace before digest, pin, or commit; CreateRun, UpsertAgent, channel and mailbox writes, assignment and grant writes, and prompt and policy writes likewise enforce version plus vocabulary at their narrowest shared persistence boundary. V1 replay and forward migration use explicit replay-only and migration-only entrypoints unreachable from live admission; caller checks remain defense in depth. The vocabulary gate lives at admission entry, never inside Event.Validate: Validate sits on the digest path (Digest to CanonicalBytes calls Validate, and Reduce validates every replayed event), so version-gating means callers apply V2 rules to new events while replay and digest verification apply exactly today's non-emptiness rule to V1-stamped events — a V1 whitelist inside Validate would fail digest recompute on existing tape. V1-stamped rows cannot spawn, admit, or persist on the live path even when the token string is a valid V2 name. Copy-forward of a V1 token into a V2-stamped row is a writer violation failing the writer-purity gate; V1 to V2 row migration occurs only through the frozen mapping with proven inverse, never by implicit canonicalizer pass-through. Per-record version covers the event envelope (interpretation only), assignment, run, agent, mailbox, grant, and prompt-selection rows. Fail-closed Canonical is a string-to-tuple signature change across roughly one hundred forty-seven non-test call sites in more than twenty packages; the charter sizes and stages that refactor as its own inverse-safe step."
      proves: "The unnamed can never execute or persist."
      evidence_class: local_test
    - action: "Retire the live alias path per rename-first: the general canonicalizers no longer accept legacy aliases on the live path, with no alias period and no rename-last fallback; the spawnRoleAllowed engineering/co-super/cosuper synonym set is retired as a named cutover entry. V1 aliases survive only inside the frozen V1 decoder."
      proves: "One vocabulary is live; the old one is read-only history."
      evidence_class: local_test
    - action: "Prove the engineering desk vocabulary end-to-end: V2 writers emit engineering (never co-super or cosuper aliases); activation accepts only the version-selected live name; V1 tape, mailboxes, and grants with canonical V1 names still replay byte-identical and verify via frozen V1 decode, with live rows forward-migrated per the decode item. No overlay JSON tool or legacy capsule operation is replaced or deleted under this item; the five-tool/four-operation retirement and replay-returns-original-receipts belong to the Engineering-proof successor mission (residue R7)."
      proves: "The first RLM desk speaks the new vocabulary without a carrier rewrite."
      evidence_class: local_test
    - action: "Extend the vocabulary_version seam beyond descriptors to tape/content role decoding, and prove the full matrix on staging with effects OFF: V1 replay byte-identical, V2 write/activate clean, unknown-value refusal, the Engineering carrier path, and the browser surface with /api/model-policy/resolve answering every V2 role name and refusing V1 names (a stale pre-cutover client bundle sending V1 tokens receives refusal until reload, recorded explicitly as the designed cutover window; the same designed refusal window applies to CLI, capsule, and other API clients still sending V1 desk tokens, not browser-only). All bound to one attempt/computer/capsule/deployment identity with CI green."
      proves: "The rename holds on the physical staging computer, not just in unit tests."
      evidence_class: deployed_proof
    - action: "Run the focused contracts with no regression: replay byte-identity, writer vocabulary, activation refusal, grant/policy attestation, prompt selection, and projection-base validation."
      proves: "Rename preserves adjacent behavior on the contracts it touches."
      evidence_class: local_test
  rollback: "Two-part rollback. (a) Source: revert the mission commits except the known-set widening commit, which rollback never reverts (reverting it would strand v2-stamped bases behind a v1-only gate); then redeploy the prior accepted source. (b) Live rows: the forward migration ships with a proven inverse — every V2-named projection row maps back under the frozen mapping, exercised as a migrate/revert/migrate cycle on staging before the writer cutover lands. Reverting source without the inverse is a known-broken state (V1 equality sites in store/cosuper_assignments.go and store/lifecycle.go would silently fail to match), so the inverse is a completion blocker, not a follow-up. A retained V1-live writer path is a completion blocker. Immutable events, reports, and recovery evidence remain untouched; product restore remains a forward event-chain transaction."
  landing:
    required: true
    environment: "staging https://choir.news"
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - "Any in-scope V1 desk-name string originates from a live writer, validator, policy, or prompt path outside the frozen V1 decoder."
    - "Any V1 tape replay mutates bytes or applies modernized semantics."
    - "Any legacy alias is accepted outside the frozen V1 decoder."
    - "Any V1 field class lacks anchored inventory coverage."
    - "Unmigrated V1 rows serve live authority, the migration inverse is unproven, a historic digest changes, a post-cutover restore falls back to genesis, or an EventKind, command prefix, tool name, or object kind is renamed."
    - "Only panel agreement, timing, local tests, or a deployment SHA exists in place of deployed proof."

boundaries:
  mutation_class: red
  drafting_mutation_class: green
  authority_sources:
    - "Owner-settled mission order; docs/reports/choir-rlm-mission-state-2026-09-08.md"
    - "docs/choir-doctrine.md; docs/computer-ontology.md"
    - "AGENTS.md; docs/standing-questions.md"
    - "Predecessors: docs/definitions/choir-rlm-restore-zero-2026-09-08.md; docs/definitions/choir-rlm-settlement-gate-2026-09-09.md"
  must_preserve:
    - "Rename-first: no bundling, no alias period, no rename-last."
    - "V1 tape decodes under frozen V1 rules with original bytes preserved."
    - "Problem-documentation-first: a code-free Define receipt precedes every repair-code commit."
    - "Frozen implementation and protocol identifiers are never desk vocabulary: EventKind values, command-identity and admission-recovery prefixes, idempotency and dedup keys, version-tagged digest domains, SQL schema/table/index names, object-graph kinds, tool names, PayloadRef payload-kinds, and product app ids."
    - "Mission-0 drill debt stays mission-0-owned (residue R1); cutover stays remainder holder (residue R6)."
    - "Pre-A checkpoint 99949fe2 remains untouched as the self-development fence."
    - "The charter census extends red surfaces only through a code-free Definition update before touching a newly found surface."
  excluded:
    - "Mission-0 restore and mission-1 settlement implementation (completed predecessors, consumed read-only)."
    - "Texture packet landing; Research and Management desk behavioral carrier work beyond their required vocabulary renames (the V2 wire mapping covers all three desks)."
    - "Prompt-bar diet, latency split, continuation census, shadow evaluations, native goals."
    - "Conductor-agentic behavior; styleguide control; provider/hill-climbing work."
    - "Candidate-A authoring, promotion, World Wire, tape deletion."
  protected_surfaces:
    - "internal/agentprofile/agentprofile.go"
    - "internal/capsule/roles.go"
    - "internal/promptstore/store.go and defaults"
    - "internal/modelpolicy/model_policy.go"
    - "internal/agentcore/rlm_reduce.go"
    - "internal/computerevent/event.go, appender.go, projection_batch.go, http_client.go"
    - "internal/projectionbase/types.go, publisher.go, verify.go, rebuilder.go"
    - "internal/store/store.go, graph_store.go, lifecycle.go, cosuper_assignments.go, project.go (role-bearing persistence and digests; run/lifecycle/assignment/grant equality sites and V1-token writers)"
    - "internal/objectgraph/object.go"
    - "internal/yaegikernel/profiles.go, internal/store/cosuper_assignment_seed.go, internal/capsule/capability.go, internal/computerevent/payload_resolver.go, frontend/src/lib/TextureEditor.svelte"
    - "internal/agentcore/cosuper_assignment_runtime.go, internal/store/cosuper_assignments.go, internal/store/lifecycle.go, internal/store/store.go, internal/toolregistry/batch_executor.go, internal/coagentowner/spawn_tool.go, internal/runtimeprompts/overlays/, internal/agentcore/channel_store.go, internal/agentcore/runtime.go and sibling admission sites (activation, admission, equality, and channel coverage)"
    - "internal/platform/checkpoints.go, internal/agentcore/self_development_decision_binding.go, internal/autoputer/file_sync.go, internal/agentcore/self_development_materializer.go, internal/agentcore/texture_audit.go, internal/agentcore/tools_capsule.go, internal/platform/event_artifacts.go, internal/platform/event_replay.go, internal/store/computer_events.go, internal/projectionbase/source.go, internal/actorruntime/, internal/textureowner/value_helpers.go, internal/modelcatalog/catalog.go (checkpoint, replay, catalog, and texture value coverage)"
    - "internal/textureowner/controller, handler, route, and coagent-route cluster (Canonical authorization sites); internal/store/lifecycle_control_delivery.go; internal/store/graph_store.go:913 join site; internal/agentcore/api_self_development.go and internal/agentcore/projection_tape.go raw ActorProfile writers"
    - "run acceptance and staging deployment routing"
  completion_evidence_floor: [local_test, deployed_proof]
  conjecture_delta:
    discovered:
      - "No V2 implementation waits unwired; the vocabulary must be built, not connected."
      - "Two general canonicalizers plus spawn synonyms form the live alias path that version-selected decode must precede."
    falsifiers:
      - "V1 tape contains role-bearing content outside every inventoried class; then the inventory is incomplete and the freeze waits."
  heresy_delta:
    discovered:
      - "Live alias normalizers accepting legacy values on the live write path."
      - "Vocabulary seam gating descriptors while tape decode stays unversioned."
      - "Event.Validate checking actor_profile non-emptiness only, so unknown actor profiles persist to tape today."
      - "Raw spawn synonyms authorizing without canonicalizing while ReduceCellIntents persists raw roles."
    introduced: []
    repaired: "none; mark repaired only after terminal deployed receipts"

measures:
  - name: v1_inventory_coverage
    kind: gate
    baseline: "Scout-mapped anchors across twelve classes; exhaustiveness unproven."
    desired: "Every V1 role-bearing site anchored; unknown sites fail the inventory."
    decision_use: "Blocks the V1 freeze while sites remain unanchored."
    cannot_prove: "Cannot prove absence of uninventoried content."
  - name: writer_vocabulary_purity
    kind: gate
    baseline: "Live writers emit V1 role strings."
    desired: "Zero V1 role strings originate from live writers; unknown values refuse at activation."
    decision_use: "Blocks completion while V1 remains writable."
    cannot_prove: "Cannot prove runtime activation reachability beyond the refusal tests."
  - name: live_row_migration_reversibility
    kind: gate
    baseline: "Forward migration specified; inverse unproven."
    desired: "Every migrated row class round-trips V1 to V2 to V1 with join predicates, assignment parents, and authorization keys intact, exercised as migrate/revert/migrate on staging."
    decision_use: "Blocks the writer cutover until the inverse is exercised."
    cannot_prove: "Cannot prove migration coverage of row classes outside the frozen inventory."
  - name: restore_migration_fence
    kind: gate
    baseline: "Migration unbound from restore."
    desired: "Every post-cutover restore from a v1-stamped base completes forward-migration before serving live authority; no restored V1-stamped row holds authority."
    decision_use: "Blocks cutover if any restore path serves unmigrated rows."
    cannot_prove: "Cannot prove restore-path coverage beyond the exercised matrix."
  - name: replay_byte_identity
    kind: gate
    baseline: "V1 replay unmeasured against original bytes."
    desired: "For frozen historic fixtures, the stored event artifact bytes, Event.CanonicalBytes result, event digest and head, projection-payload bytes, and payload digest are identical before and after the cutover. Version-selected decode creates a separate interpreted view and never mutates digest-bearing Event, DurableEvent, ProjectionBatch, or payload bytes."
    decision_use: "Blocks cutover if any historic artifact, canonical encoding, chain head, payload digest, or receipt verification changes, or if interpretation requires rewriting digest-bearing input."
    cannot_prove: "Cannot prove decoder totality beyond the inventoried classes."
  - name: panel_agreement
    kind: weak_signal
    baseline: "Draft consensus pending."
    desired: "No additional agreement threshold."
    decision_use: "Explains draft review scope only; does not authorize charter or completion."
    cannot_prove: "Cannot authorize promotion, prove code, or advance completion. Cannot prove rename correctness."

now:
  status: blocked_incomplete
  slice: "round-8 repaired draft under focused verification; missions 0 and 1 complete; charter ratification pending"
  question: "Is the round-8 repaired draft with per-function sets, restore fencing, and enumerated live set ready for owner ratification?"
  reconciliation:
    observed_at: "2026-09-09T23:29:36Z"
    source_ref: "main@364bbaf6 (round-7 repaired draft; re-observe HEAD at charter ratification)"
    deploy_identity: "staging https://choir.news ok via proxy 0475ed84 (last observed; re-observe at charter)"
    authority_identities:
      - "docs/designs/rlm-target-architecture-2026-09-04.md:79-85 (desks vocabulary)"
      - "docs/definitions/choir-rlm-restore-zero-2026-09-08.md (completed predecessor)"
      - "docs/definitions/choir-rlm-settlement-gate-2026-09-09.md (completed predecessor)"
      - "docs/mission-residues.md (R1, R6 open)"
      - "AGENTS.md; docs/standing-questions.md"
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "4 untracked leftover paths preserved as unrelated WIP; re-observe before charter"
    status: reconciling
  candidate:
    id: none
    state: none
    ref: none
    owner: none
    base: none
    digest: none
    scope: []
  decision:
    selected: "Mission 2 will charter versioned rename as drafted after review; closes no predecessor acceptance; retains cutover remainder-holder status and mission-0 drill debt ownership."
    kind: architecture
    status: proposal
    source: orchestrator
    evidence_ref: "Owner-settled mission order and rename-first direction; scout surface map 2026-09-09"
    owner_ratification_ref: "charter ratification still required before execution"
    recorded_at: "2026-09-09T23:29:36Z"
    consequence: "Draft and review only. No rename repair executes under this file before charter ratification with fresh reconciliation."
  evidence_refs:
    - "internal/agentprofile/agentprofile.go"
    - "internal/capsule/roles.go"
    - "internal/computerevent/event.go and appender.go"
    - "internal/projectionbase/types.go"
  blocker_or_risk: "Charter ratification pending. Per-function acceptor sets and v2 mapping receipt freeze at charter; census lands as the first charter Define."
  next_action: "Focused verification consensus on round-8 repairs; pin the reviewed draft digest; reconcile source, deployment, and individual WIP paths; record owner ratification and the code-free Define receipt; verify all three navigation registries before enabling execution."

