---
definition_version: 2
definition_id: choir-rlm-versioned-rename-2026-09-09
execution_mode: mission_orchestrator

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
      evidence_ref: "docs/designs/rlm-target-architecture-2026-09-04.md:79-85; internal/agentcore/rlm_reduce.go:76-89,180-187; unpinned source-trace, re-pin at charter SHA"
    - claim: "Live alias normalization runs in two general canonicalizers before any version-selected decode: agentprofile.Canonical and modelpolicy.NormalizeRole both accept legacy aliases on the live path."
      evidence_ref: "internal/agentprofile/agentprofile.go:115-139; internal/modelpolicy/model_policy.go:172-186; re-pin at charter SHA"
    - claim: "Mission-0's vocabulary_version seam gates ProjectionBase descriptors only; it does not select a V1/V2 tape or content role decoder."
      evidence_ref: "internal/projectionbase/types.go:22-33,63-113; internal/projectionbase/verify.go:17-48"
    - claim: "ProjectionBatch V1/V2 is an independent format discriminator, not a vocabulary version; it must not be confused with the absent vocabulary V2."
      evidence_ref: "internal/computerevent/projection_batch.go:11-21,157-215"
    - claim: "The mission-state report enumerates eleven role-bearing categories, and the current tree contains a separate executable Yaegi profile registry, suggesting a twelve-class coverage taxonomy. Exact membership, file anchors, and exhaustiveness remain unverified: the first charter Define generates and pins the mechanical census before implementation, and until then this is a scoped conjecture, not an exhaustive observed partition."
      evidence_ref: "scout_unverified surface map 2026-09-09, re-pin at charter; docs/reports/choir-rlm-mission-state-2026-09-08.md rename section"
    - claim: "Mission 0 and mission 1 are both completed with deployed proof; the carried mission order names versioned rename next with Engineering as first executable carrier proof."
      evidence_ref: "docs/definitions/choir-rlm-restore-zero-2026-09-08.md now.status completed; docs/definitions/choir-rlm-settlement-gate-2026-09-09.md now.status completed; docs/reports/choir-rlm-mission-state-2026-09-08.md carried mission order"
  unknowns:
    - "V1 field sites beyond the scout-mapped anchors; the charter freezes the exhaustive inventory."
  start_correction:
    - date: "2026-09-09"
      note: "The opening receipt main@34629502 remains immutable (creation commit bcfa97b7). Claims 1-4 are source-anchored to the cited symbols as file:line unpinned source-traces; no single mechanical-verification SHA is recorded for this draft, so charter reconciliation mechanically verifies and re-pins claims 1-5 to one exact source_ref before execution. Claim 5 remains scout_unverified. Claim 6 is documentary. Correction to the opening receipt: worktree_inventory.status was recorded as reconciled, but four dirty paths were not individually enumerated, so the opening inventory must be treated as reconciling. The worktree class goal_candidate was also incorrect for those paths; they remain unknown-owner unrelated WIP, read-only and leave-in-place, until fresh charter reconciliation names and classifies each path."
finish:
  deliver: "Versioned rename is one durable truth per vocabulary version: V1 tape decodes under frozen V1 rules with original bytes preserved, V2 writers emit only the new desks vocabulary, unknown live values reject at activation, and the exhaustive V1 field inventory exists as a frozen artifact. Rename-first with proposed in-scope surface, pending owner ratification at charter: every in-scope V1 desk-name occurrence renames (super, co-super/cosuper, researcher, live aliases, ID prefixes, agent addresses, prompt paths, digest inputs carrying desk tokens); version-tagged digest domains and the mission-1 v1 classification receipt stay immutable, with a successor v2 identity/mapping receipt recording token, prefix, and digest-input equivalence. No bundling, no alias period, no rename-last."
  artifact: "A frozen V1 field inventory artifact plus the version-selected decode/encode/activation contract across tape replay, writers, validators, policies, prompts, grants, and role-bearing persistence, with the Engineering-named desk as the first vocabulary proof vehicle and a deployed staging proof."
  entrypoints:
    implementation:
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
      - "internal/agentcore/cosuper_assignment_runtime.go, internal/store/cosuper_assignments.go, internal/store/lifecycle.go, internal/store/store.go, internal/toolregistry/batch_executor.go, internal/coagentowner/spawn_tool.go, internal/runtimeprompts/overlays/, internal/agentcore/channel_store.go, internal/agentcore/runtime.go, tool_profiles.go, super_controller.go, api.go, tools_worker_update.go, tools_coagent.go, researcher_checkpoint_fallback.go (activation, admission, equality, and channel sites under the refusal item)"
      - "internal/platform/checkpoints.go, internal/agentcore/self_development_decision_binding.go (exact ActorProfile consumers); internal/autoputer/file_sync.go, internal/agentcore/self_development_materializer.go, internal/agentcore/texture_audit.go, internal/agentcore/tools_capsule.go (event ActorProfile writers); internal/platform/event_artifacts.go, internal/platform/event_replay.go, internal/store/computer_events.go, internal/projectionbase/source.go, internal/actorruntime/, internal/textureowner/value_helpers.go, internal/modelcatalog/catalog.go (checkpoint, replay, catalog, and texture value paths)"
      - "internal/textureowner/controller, handler, route, and coagent-route cluster (Canonical authorization sites); internal/store/lifecycle_control_delivery.go; internal/store/graph_store.go:913 join site; internal/agentcore/api_self_development.go and internal/agentcore/projection_tape.go raw ActorProfile writers"
      - "cmd/capsule-broker/main.go (RoleVerbSets gate, exact RoleCoSuper compare); cmd/capsule-broker/session_worker.go (sessionFor role)"
  acceptance:
    - action: "Produce the exhaustive frozen V1 field inventory artifact covering twelve non-exclusive coverage lenses, using the twelve-class taxonomy below as the proposed partition — (1) agent profiles and canonicalizers, (2) static capsule vocabulary and authorization policy definitions (AgentRole, role constants, RoleVerbSets), (3) runtime capsule and broker role carriers and selection (Capability.AgentRole, spawn and session request fields, broker role admission, sessionFor; references to class-2 definitions are cross-links, not additional class-3 inventory rows; Yaegi ProfileRegistry and GetProfile have no production callers and are delete-first or test-only renamed, never version carriers), (4) reducer and spawn aliases, (5) prompt roles and prompt file paths, (6) model policies and role selection, (7) persisted runs and lifecycle records, (8) assignments and work-item authority profiles, (9) mailbox destinations and agent addresses, (10) agent-ID prefixes and object-graph edge metadata, (11) grants, digest inputs, and evidence identifiers, (12) event envelope actor profiles and replay dispatch — with PayloadRef.Role payload-kinds classified payload-kind out of desk scope and TextureEditor SOURCE_PANEL_MODEL_ROLES classified live protocol under class 6, never display-only; a remainder lens is a Define update and a freeze blocker, never a silent extra bucket. Each physical site has one unique row key (source_ref, file, symbol, field_path), one primary semantic namespace, and zero or more coverage-lens tags; exhaustiveness is measured over the union, never by pretending the lenses are mutually exclusive. Live-row migrate and revert plus the serving fence are the decode item, the deployed matrix, and rollback — not this item. Exhaustiveness proof is a standing CI gate frozen at charter over a versioned corpus manifest covering every tracked non-test production source, configuration, schema, generated-source input, and prompt asset: Go, TypeScript, JavaScript, Svelte, YAML and YML, JSON, TOML, SQL, HTML, shell, Nix, embedded prompts, and runtime and generated prompt inputs. CI fails when a new production path or extension is unclassified, as well as on any census set difference against the frozen artifact. The artifact header binds one exact source_ref and generator and query version; each inventory row records source_ref, file:line, symbol, field path, semantic namespace, V1 token set, V2 token, version carrier, live-or-history classification, writers, consumers, digest consequence, forward migration, and inverse; verification status is verified or unknown and scout-only rows never satisfy the freeze. Exhaustive means exhaustive over this named corpus; absence outside it is not claimed. Unknown or unanchored sites fail the inventory."
      proves: "The rename scope is bounded by evidence, not by assumption."
      evidence_class: local_test
    - action: "Add version-selected frozen V1 decode before the general canonicalizers, carried by explicitly named per-class version carriers extending the mission-0 vocabulary_version seam: charter names one carrier per class (tape event field, mailbox and channel row columns, run, lifecycle, assignment, and grant row columns, agent row columns, object-graph objects and edges as explicit carrier rows for class 10 with version markers or derived-only evidence, prompt and profile registry tags, payload envelope markers where role-bearing, computer-owned model-policy TOML overlays; classes with no carrier use unmarked-defaults-V1 only by charter-relative exception with negative sweep evidence), with unmarked tape, content, and persistence rows defaulting V1. All production decoding of Event, CASRequest, or DurableEvent — including json.Unmarshal, json.Decoder.Decode, transport-wrapper decoding, database scan helpers, custom unmarshalling, artifact inspection, and checkpoint decoding — routes through either the centralized raw-preserving historic decoder or the centralized V2 write-admission decoder; tests are the only exception, and the frozen decoder matrix enumerates every current call site with CI rejecting new decode roots outside those two APIs. Unknown role keys in computer-owned TOML refuse at parse instead of falling through to model defaults. ProjectionBase descriptors keep mission-0 refuse-missing/unknown and are never re-admitted as legacy; IsKnownVocabularyVersion widens to the explicit set {v1, v2} while CurrentVocabularyVersion advances to v2, so a v1-stamped base remains installable and replays through the frozen V1 decoder while a missing or out-of-set version still refuses; acceptance requires a pre-cutover v1 base descriptor installing and verifying on post-cutover source, and a post-cutover restore falling back to genesis replay is a mission-0 regression and a completion blocker. Commit ordering: the never-revert commit widens IsKnownVocabularyVersion to {v1, v2} only, and CurrentVocabularyVersion advances to v2 in a later revertible mission commit; rollback never reverts the known-set widening and always reverts Current if writers revert to V1. Base rebuild migrates inside the scratch database before publish, never stamping v2 over V1-named rows. The same serving fence binds every path that can deposit or serve rows, not restore-from-base only: appender replay deposits V1 names and never rewrites digest-bearing bytes; base rebuild migrates inside scratch before publish; rematerialize-from-tape completes forward-migration before the staged store flips to live; boot dispatch including first boot after deploy onto existing V1 SQL completes forward-migration before live spawn, admission, or persistence; RecoverPrepared recovery paths complete forward-migration before their projections commit; projection-batch decode remains a projector-format discriminator, not a vocabulary decoder; payload fetch keeps PayloadRef.Role payload-kind. Vocabulary stamp and digest domain are two independent stamps never read for each other: historic digest-bearing records and their bytes remain immutable V1 records, and where migration changes any field covered by a digest the live V2 representation receives a V2-domain successor digest or identity with the mapping receipt linking the immutable V1 predecessor to that successor while live references rewire atomically; a V1 digest remains only as explicitly named provenance and never authenticates renamed V2 content, and a row retains its digest only when the inventory proves the renamed field stood outside that digest's canonical input. The proven inverse is deterministic rollback to a canonical V1 representative proven semantically compatible with old source, or exact restoration backed by retained per-row source-token provenance where many-to-one collapse would otherwise lose the original token; immutable and digest-bearing history is never rewritten. Live projection rows forward-migrate to V2 names under the frozen mapping; V1 activation via decode serves replay verification only, never live authority."
      proves: "Tape bytes never change under replay; live rows migrate to V2 names only under the frozen mapping with a proven inverse, never silently."
      evidence_class: local_test
    - action: "Cut every live writer to the new desks vocabulary only, with the frozen V2 live set enumerated at charter: management, engineering, research (lowercase wire tokens) plus the canonical-stays-live profiles texture, conductor, processor, reconciler, email, and verifier roles, each separated from its retiring aliases. Freeze per-function V1 acceptor tables, never unioned into one Canonical alias class. (a) Management: super to management. Canonical V1 accepts exactly {super} mapping to Super with zero alias branches; V2 canonical is management; spawn matrix super-allows-any becomes management-allows-any; PolicyFor, CanSpawn, and CanMessage keep today's target sets with tokens only (Super AllowedSpawnTargets {researcher} becomes {research}, never any; Researcher spawn list stays empty even though spawnRoleAllowed allows research to research); unifying those matrices is out of scope. (b) Engineering: co-super to engineering. Canonical V1 maps {cosuper, co-super, coagent, co-agent} to CoSuper (engineering is not a Canonical alias; it passthroughs today). NormalizeRole extras {co_super, cosuper_coding, co-super-coding} plus overlapping Canonical keys map to CoSuper (engineering is not in this set). spawnRoleAllowed holds {co-super, cosuper, engineering} as spawn synonyms only, allowing children researcher, co-super, cosuper, engineering. Yaegi registry key is cosuper (V2 key becomes engineering). V2 live canonical is engineering. Migration maps co-super, cosuper, and the Canonical plus NormalizeRole aliases to engineering under the frozen map. Persisted raw engineering on the spawn and reduce path (spawnRoleAllowed synonym; ReduceCellIntents raw in.Role) is the named mapping-table row and forward-maps to engineering. Refuse-to-elevate applies only to census remainder tokens not in the frozen mapping table; unclassified remainder is a freeze blocker, never a silent grant and never a silent drop. (c) Research: researcher to research. Canonical V1 maps {researcher, researchers, research, research-agent, web-research, web-researcher} to Researcher; NormalizeRole carries no research branch (passthrough — charter pins this absence as a per-function fact). spawnRoleAllowed holds {researcher, research} allowing children researcher, research. V2 live canonical is research; the version collision is explicit — research is a V1 alias and the V2 canonical at once, so the row version stamp alone decides live authority. Migration maps the full Canonical research set to research under the frozen map. Texture, processor, reconciler, email, verifier, and conductor alias sets enumerated identically (each canonical plus every alias branch) and classified per alias as decode-only with zero live acceptance. Freeze two sets, not one: the live-alias retirement set above, and the persisted V1 token set from the charter persistence census of tape, SQL, mailbox, grant, run, and agent rows. Canonical EventKind wire values are frozen as-is — researcher_update keeps its V1 string in both V1 and V2 events because it is a digest input over the immutable chain, and any EventKind rename is explicitly out of scope. Frozen as implementation and protocol identifiers, never desk vocabulary: EventKind values; command-identity, admission-recovery, and idempotency prefixes; version-tagged digest domains; SQL table and index names; Go type, function, and constant identifier names (never their wire values — RoleVerbSets keys rename); filenames and package paths (never prompt file paths, which rename with owner-override migration); object-graph kinds; tool names; PayloadRef payload-kinds; product app ids. Compound protocol identifiers that contain a desk lexeme but are not a desk-role field stay frozen: EventKind values (researcher_update), admission-recovery prefixes, tool names, Texture source-entity status tokens (researcher_confirmed, researcher_refuted, researcher_qualified). Operator-visible UI role-name copy and live role lists rename (SOURCE_PANEL_MODEL_ROLES researcher to research, super to management; engineering joins only if the census shows it was already a live selector). Display labels for frozen status tokens may rename as copy without changing the wire token. Code filenames and package paths stay frozen; prompt file paths rename with owner-override migration (overlay map frozen at charter: super_runtime.yaml to management_runtime.yaml, co_super_runtime.yaml to engineering_runtime.yaml, rlm_co_super_runtime.yaml to rlm_engineering_runtime.yaml, researcher_runtime.yaml to research_runtime.yaml; YAML role ids follow the same lexeme map; desk-name prose inside overlays becomes Management, Engineering, Research; tool identifiers stay frozen until R7). Path lexemes join the census corpus so CI never certifies a filename blind spot. Desk tokens inside identity strings follow the frozen-with-token tie-break: run-super-assignment and work-super-assignment style constructed identifiers rename with the desk token while bare command-identity prefixes stay byte-stable, adjudicated per site in the frozen inventory. PolicyFor AllowedSpawnTargets and AllowedMessageTargets static V1 values cut to V2 desk names with the writer cutover. All other rename migration covers ID prefixes, agent addresses, prompt paths, and digest inputs carrying desk tokens with per-site equivalence proof; a successor v2 identity/mapping receipt records the equivalence while the mission-1 v1 receipt remains immutable. The receipt records each V1 digest-bearing field byte-identical pre-cutover digest (including TerminalPropositionV1, choir:terminal-proposition:v1, and the mission-1 v1 classification receipt); post-cutover verification of historic V1 records must equal those pinned bytes and never recompute a modernized v1 receipt. Grant, policy, and prompt surfaces validate the new vocabulary. Model-authored cell source, report prose, and Texture bodies are out of scope. Sequencing follows the normative landing order in the activation item below; migration and the serving fence precede refusal there, never as a prior production deploy."
      proves: "New truth is written once, in one vocabulary."
      evidence_class: local_test
    - action: "Freeze a per-function and per-carrier mapping table before implementation: every row records the V1 token, V2 token, forward map, inverse map, carrier, and V1 and V2 verifier rules. Migration maps all three desks under one frozen map, never engineering alone: (a) super, and every Canonical branch returning Super, map to management; (b) co-super, cosuper, coagent, co-agent, co_super, cosuper_coding, co-super-coding, and the V1 spawn synonym engineering map to engineering; (c) researcher, researchers, research, research-agent, web-research, web-researcher map to research. The V2 Canonical table is frozen at charter as the identity map over the live set {management, engineering, research, texture, conductor, processor, reconciler, email} plus the verifier roles, with zero alias branches and a fail-closed default. The V2 NormalizeRole table is frozen as the identity map over that same live set plus VerifierRole and MultimodalVerifierRole, with a fail-closed default. The V2 spawnRoleAllowed matrix is frozen as: spawner management allows any child; spawner engineering allows children engineering and research; spawner research allows child research; default false — and it accepts no V1 token in any position. The V1 alias research (which resolves to Researcher) and the V2 canonical research are the same string carrying different meanings, exactly as engineering is: live-authority eligibility is the row's version stamp after migration, never token shape, so a V1-stamped research row is unmigrated until stamped v2 and the identity map research to research still requires the v2 stamp. The successor v2 identity and mapping receipt enumerates every digest-bearing m1 predecessor field carrying desk vocabulary: the terminal proposition V1 domain, co-super command request JSON, AgentRecord Profile and Role, WorkItemRecord AuthorityProfile, grant Role, verb-set and policy digest inputs, command IDs, command digests, attestation references, schema and version strings, and all role-bearing identity prefixes. Historic V1 records verify with original bytes, tokens, and V1 digest rules. Charter classifies every role-shaped owner token (ActorProfile at internal/agentcore/rematerialize.go:351, mailbox ToDesk and FromDesk owner, and any other inventoried role field) as either a named V2 profile with frozen V1 decode cover or a frozen non-desk protocol value; PrivacyClass owner is a different namespace and stays frozen. Charter classifies the live restore writer value owner: either it maps to a named V2 profile with frozen V1 decode cover, or it is declared a frozen non-desk protocol value with negative-sweep evidence; the writer cutover never lands while owner is unclassified."
      proves: "No mapping is implied; every equivalence is written down and verified."
      evidence_class: local_test
    - action: "Reject unknown live values at activation: any role or desk value outside the version-selected vocabulary fails closed before spawn, admission, or persistence. Refusal covers the agentprofile.Canonical default passthrough (fail-closed Canonical returns empty string plus a typed unknown-profile error for unknown live values, never the input token: existing Canonical-not-empty sites then fail closed, while returning a non-empty unknown token would keep authorizing callers that ignore the error; every Canonical, PolicyFor, and CanSpawn caller checks the error, and a non-empty unknown token is a writer-purity failure), the PolicyFor default policy, the PolicyFor AllowedSpawnTargets and AllowedMessageTargets tables — static V1 profile values that CanSpawn and CanMessage compare through Canonical, making them authorization surfaces that migrate with the constant values and carry their own inventory rows rather than being incidental references to class 2 — the NormalizeRole default, grant/assignment/run equality sites, prompt normalizePromptRole, toolCallSpawnProfile role/profile reads, the capsule RoleVerbSets constants rename, and the ReduceCellIntents raw in.Role and scope.FromRole persistence path. Named slips closed here, not by file-list coverage: HandleModelPolicyResolve empty-role to conductor default is retired so empty is typed missing and unknown and V1 names refuse; the production role authority is capsule AgentRole into the session worker role into RoleVerbSets; parsePolicy and parseOverlay unknown roles keys refuse at parse instead of NormalizeRole-and-store. The tape-append gate is named: computerevent AppendNew entrypoints reject ActorProfile values outside the version-selected vocabulary, closing the disclosed non-emptiness-only heresy; Event.Validate itself remains digest-compatible and version-neutral for V1 replay. The non-desk ActorProfile token trusted-core is a frozen protocol identifier that activation must not refuse; the restore writer value owner is classified once, at charter, per the mapping-table item below — never frozen here. Empty input is typed missing (preserving the toolCallSpawnProfile Profile-to-Role fallback), distinct from unknown. PayloadRef.Role and RunMemoryEntryProjection.Role stay outside this gate, the latter only with census evidence that its writers are class-12 covered. Enforcement sits at every live write root identified by the frozen writer census, not a named subset: every ComputerEventAppender append variant; projection-batch dry-run, decode, and application; agent and run create, update, and object-graph projection paths including UpdateRun, lifecycle run projection and update commands, and direct object-graph run and agent writers; lifecycle activation replacement and projection; work-item, assignment, and grant create and update paths; channel and mailbox append paths; prompt and policy parse and persistence. Generic metadata patch paths reject classified desk-bearing keys unless they pass the same typed validator. Lower-level object-graph and SQL writers either enforce the same gate or become explicit replay-only or migration-only entrypoints unreachable from live callers. V1 replay and forward migration use explicit replay-only and migration-only entrypoints unreachable from live admission; caller checks remain defense in depth. The vocabulary gate lives at admission entry, never inside Event.Validate: Validate sits on the digest path (Digest to CanonicalBytes calls Validate, and Reduce validates every replayed event), so version-gating means callers apply V2 rules to new events while replay and digest verification apply exactly today's non-emptiness rule to V1-stamped events — a V1 whitelist inside Validate would fail digest recompute on existing tape. V1-stamped rows cannot spawn, admit, or persist on the live path even when the token string is a valid V2 name. Copy-forward of a V1 token into a V2-stamped row is a writer violation failing the writer-purity gate; V1 to V2 row migration occurs only through the frozen mapping with proven inverse, never by implicit canonicalizer pass-through. Per-record version covers the event envelope (interpretation only), assignment, run, agent, mailbox, grant, and prompt-selection rows. Fail-closed Canonical is a string-to-tuple signature change across roughly one hundred forty-seven non-test call sites in more than twenty packages; the charter sizes and stages that refactor as its own inverse-safe step, sequenced after the writer cutover lands. The landing order across acceptance items is normative and is not the order this list reads in: (1) inventory freeze; (2) the never-reverted widening of IsKnownVocabularyVersion to {v1, v2}; (3) the frozen V1 decoder and the two centralized decode roots, with V1 aliases still accepted live; (4) the string-to-tuple Canonical refactor landing behaviorally inert — it still accepts every frozen V1 alias and reports it known, so the signature change carries no vocabulary change; (5) migrate/revert/migrate drill on staging (or scratch), ending on V1-named serving rows; the drill never leaves staging serving mixed vocabulary. (6) Single never-partially-deployed cutover, intra-ordered: live-row forward-migration and serving fence, then CurrentVocabularyVersion=v2 plus V2-only writers plus alias retirement plus fail-closed refusal plus in-scope client bundles (TextureEditor.svelte and any other inventoried live role UI); frontend is not a later deploy. Landing refusal or alias retirement while live writers still emit V1, or serving V2 rows while writers still emit V1, is a defect. Landing refusal or alias retirement before step 5 completes is a defect, not an ordering preference."
      proves: "The unnamed can never execute or persist."
      evidence_class: local_test
    - action: "Retire the live alias path per rename-first: the general canonicalizers no longer accept legacy aliases on the live path, with no alias period and no rename-last fallback; the spawnRoleAllowed engineering/co-super/cosuper synonym set is retired as a named cutover entry. V1 aliases survive only inside the frozen V1 decoder."
      proves: "One vocabulary is live; the old one is read-only history."
      evidence_class: local_test
    - action: "Prove the engineering desk vocabulary end-to-end: V2 writers emit engineering (never co-super or cosuper aliases); activation accepts only the version-selected live name; V1 tape, mailboxes, and grants with canonical V1 names still replay byte-identical and verify via frozen V1 decode, with live rows forward-migrated per the decode item. No overlay JSON tool or legacy capsule operation is replaced or deleted under this item; the five-tool/four-operation retirement and replay-returns-original-receipts belong to the Engineering-proof successor mission (residue R7)."
      proves: "The first RLM desk speaks the new vocabulary without a carrier rewrite."
      evidence_class: local_test
    - action: "Extend the vocabulary_version seam beyond descriptors to tape/content role decoding, and prove the full matrix on staging with effects OFF. Publish a decoder matrix with one row per raw-entry path and version state: platform EventArtifactService EventsPage, projectionbase DiskEventSource EventsPage, computerevent HTTPClient EventsPage, ComputerEventAppender AppendNewPayload dry-run, ComputerEventAppender resolveProjectionBatch after ResolvePayloads, Reconstruct, ReconstructInto, and ReconstructThroughTarget, autoputer runReplayPhase, projectionbase Rebuilder Run, restore and rematerialization, and every checkpoint and event-artifact decoder reading raw event bytes. Each row names the raw bytes, carrier and selector, decoder, consumer, canonicalization boundary, and expected V1 and V2 behavior, with negative rows for receipts and generic message roles. Exercise on staging: V1 replay byte-identical, the census-derived live-row transform V1 to V2 to canonical V1 to V2 with joins and authorization verified after each transition, V2 write and activate clean, unknown-value refusal, the Engineering vocabulary path, and the browser surface with /api/model-policy/resolve answering every V2 role name and refusing V1 names (a stale pre-cutover client bundle sending V1 tokens receives refusal until reload, recorded explicitly as the designed cutover window; the same designed refusal window applies to CLI, capsule, and other API clients still sending V1 desk tokens, not browser-only). Prove the single computer-level serving fence stays closed until all row classes agree with the active vocabulary. All bound to one attempt/computer/capsule/deployment identity with CI green."
      proves: "The rename holds on the physical staging computer, not just in unit tests."
      evidence_class: deployed_proof
    - action: "Run the focused contracts with no regression: replay byte-identity, writer vocabulary, activation refusal, grant/policy attestation, prompt selection, and projection-base validation."
      proves: "Rename preserves adjacent behavior on the contracts it touches."
      evidence_class: local_test
  rollback: "Two-part rollback. (a) Source: revert the mission commits except the known-set widening commit, which rollback never reverts (reverting it would strand v2-stamped bases behind a v1-only gate); then redeploy the prior accepted source. (b) Live rows: proven inverse on staging before cutover (drill, then leave V1 serving). If cutover must be reverted after V2 rows or a v2-stamped base exist: inverse-migrate live SQL and projections under the frozen map, then install the last pre-cutover v1-stamped ProjectionBase as the serving base; never install a v2-stamped base onto rolled-back V1 source and never rewrite the v2 base blob. Known-set stays {v1, v2} so re-applying this mission can install v2 bases again. The forward migration ships with a proven inverse — every V2-named projection row maps back under the frozen mapping, exercised as a migrate/revert/migrate cycle on staging before the writer cutover lands. Reverting source without the inverse is a known-broken state (V1 equality sites in store/cosuper_assignments.go and store/lifecycle.go would silently fail to match), so the inverse is a completion blocker, not a follow-up. A retained V1-live writer path is a completion blocker. Immutable events, reports, and recovery evidence remain untouched; product restore remains a forward event-chain transaction."
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
    - "cmd/capsule-broker/main.go and session_worker.go (live broker role gate)"
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
  - name: recovery_migration_fence
    kind: gate
    baseline: "Migration unbound from recovery paths."
    desired: "Every post-cutover recovery path that can install a v1-stamped base or replay a V1 tail — boot reconstruction, rematerialize, cold recovery, owner-scoped recover_current, and RecoverPrepared recovery — completes forward-migration before the computer serves live authority; no restored V1-stamped row holds authority."
    decision_use: "Blocks cutover if any recovery path serves unmigrated rows."
    cannot_prove: "Cannot prove recovery-path coverage beyond the exercised matrix."
  - name: replay_byte_identity
    kind: gate
    baseline: "V1 replay unmeasured against original bytes."
    desired: "For frozen historic fixtures, the stored event artifact bytes, Event.CanonicalBytes result, event digest and head, projection-payload bytes, and payload digest are identical before and after the cutover. Version-selected decode creates a separate interpreted view and never mutates digest-bearing Event, DurableEvent, ProjectionBatch, or payload bytes."
    decision_use: "Blocks cutover if any historic artifact, canonical encoding, chain head, payload digest, or receipt verification changes, or if interpretation requires rewriting digest-bearing input."
    cannot_prove: "Cannot prove decoder totality beyond the inventoried classes."

now:
  status: working
  slice: "Owner ratified frozen non-desk protocol (2026-09-10T15:21:26Z); --freeze ACCEPTED with zero unknowns. Next: single writer cutover (step 6, intra-ordered) with v2 identity/mapping receipt."
  question: none
  reconciliation:
    observed_at: "2026-09-10T05:30:00Z"
    source_ref: "main@8bc16619 (charter promotion commit; four-scout mechanical census base)"
    deploy_identity: "staging https://choir.news ok via proxy 0475ed84; missions 0 and 1 completed with deployed proof"
    authority_identities:
      - "docs/reports/choir-rlm-mission-state-2026-09-08.md (carried mission order, rename-first)"
      - "docs/designs/rlm-target-architecture-2026-09-04.md:79-85 (desks vocabulary)"
      - "docs/definitions/choir-rlm-restore-zero-2026-09-08.md (completed predecessor)"
      - "docs/definitions/choir-rlm-settlement-gate-2026-09-09.md (completed predecessor)"
      - "docs/mission-residues.md (R1, R6, R7 open)"
      - "AGENTS.md; docs/standing-questions.md"
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "docs/reports/choir-rlm-restore-zero-completion-report-2026-09-09.md, scripts/generate_restore_zero_completion_pdf_2026_09_09.py, scripts/__pycache__/, tmp/ — unknown-owner unrelated WIP, preserve read-only leave-in-place"
    status: reconciled
  candidate:
    id: none
    state: none
    ref: none
    owner: none
    base: none
    digest: none
    scope: []
  decision:
    selected: "Owner tokens classify as frozen non-desk protocol (mapping §7 proposal accepted as written, including the named fallback)."
    kind: purpose
    status: settled
    source: owner
    evidence_ref: "Owner ask ratification 2026-09-10T15:21Z (mission-2 cutover gate question); mapping doc §7 rationale stands"
    owner_ratification_ref: "ratified 2026-09-10T15:21:26Z; --freeze unblocks on re-freeze flipping the 3 rows to verified/frozen"
    recorded_at: "2026-09-10T15:21:26Z"
    consequence: "Writer cutover unblocked. Charter architecture decision (owner ratification 2026-09-10T04:28:08Z) retained in receipts; this card now carries the live classification decision."
  evidence_refs:
    - "internal/agentprofile/agentprofile.go:6-15 constants; 32-113 PolicyFor; 116-139 Canonical; 147-168 CanSpawn/CanMessage"
    - "internal/capsule/roles.go:4-9 AgentRole constants; 42-57 RoleVerbSets; internal/capsule/capability.go:19 AgentRole carrier"
    - "internal/agentcore/rlm_reduce.go:76-89 spawnRoleAllowed; 166-195 ReduceCellIntents raw in.Role persistence"
    - "internal/modelpolicy/model_policy.go:172-186 NormalizeRole; 391-439 parsePolicy; 441-506 parseOverlay"
    - "internal/promptstore/store.go:35-49 promptRoles; 159-167 normalizePromptRole; defaults/*.yaml role ids and V1 prose"
    - "internal/runtimeprompts/overlays/ super/co_super/rlm_co_super/researcher_runtime.yaml role ids; prompts.go:49-67 selectors"
    - "frontend/src/lib/TextureEditor.svelte:166 SOURCE_PANEL_MODEL_ROLES live protocol list"
    - "internal/computerevent/event.go:20-62 EventKind frozen set; 140-201 Validate; appender.go:271-423 append gates; projection_batch.go:11-40 format discriminator"
    - "internal/agentcore/api_self_development.go, chain_bootstrap.go, self_development_materializer.go raw Event ActorProfile Super writers; actorcore adapter/handler researcher canonical consumers (late GrantsEvents rows)"
    - "internal/types/task.go, cosuper_assignment.go; internal/store/cosuper_assignments.go:182-218 terminal proposition V1 domain; internal/objectgraph/object.go:97-174 content/edge hashing"
    - "internal/yaegikernel/profiles.go: no production callers (delete-first, test-only)"
  blocker_or_risk: "Cutover is the remaining red work (forward-migration plus serving fence, then Current=v2 plus V2-only writers plus alias retirement plus fail-closed refusal plus client bundles, never partially deployed). R1 drill debt mission-0-owned; cutover remainder holder (R6); tool retirement R7 successor."
  next_action: "Land the single never-partially-deployed cutover intra-ordered: live-row forward-migration and serving fence, then CurrentVocabularyVersion=v2 plus V2-only writers plus alias retirement plus fail-closed refusal plus in-scope client bundles, with the v2 identity/mapping receipt."


receipts:
  - id: versioned-rename-charter-2026-09-10
    boundary: define
    commit_or_artifact: "933fa22f (owner-reviewed reconciled draft); executable promotion lands atomically in this commit"
    proof_refs:
      - "docs/definitions/choir-rlm-restore-zero-2026-09-08.md completed status consumed read-only"
      - "docs/definitions/choir-rlm-settlement-gate-2026-09-09.md completed status consumed read-only"
      - "docs/mission-residues.md R1, R6, R7"
      - "ten panel rounds plus opus and grok workability passes, zero HOLD verdicts"
    rollback_ref: "registry-only change; revert restores blocked-draft topology"
    disposition: "versioned-rename promoted to sole working entrypoint; predecessors remain completed non-entrypoints; cutover retained as remainder holder"
    problem_ref: "super/co-super terminology confusing relative to desk roles; researcher implying singularity for a transparently scaling RLM"
    authorization_ref: "Owner charter ratification 2026-09-10T04:28:08Z"
    candidate_or_evidence_refs: []
  - id: versioned-rename-census-2026-09-10
    boundary: define
    commit_or_artifact: "code-free Define; census base main@8bc16619; no source touched"
    proof_refs:
      - "CensusProfiles: agentprofile Canonical/PolicyFor/CanSpawn callers; capsule RoleVerbSets gates; Yaegi ProfileRegistry/GetProfile zero production callers (delete-first)"
      - "CensusReducerPrompt: spawnRoleAllowed V1 synonym set; promptRoles/normalizePromptRole strict-canonical; NormalizeRole no-research-branch; SOURCE_PANEL_MODEL_ROLES live protocol"
      - "CensusPersistMail: run/lifecycle/assignment/mailbox/ID-prefix/graph rows with writer-vs-equality split; rematerialize.go:351 owner token classified charter-pending"
      - "CensusGrantsEvents: terminal-proposition V1 digest domain; EventKind/command-prefix/tool-name frozen-protocol exclusions; full decode-root list for decoder matrix"
    rollback_ref: "registry-only change; revert restores pre-census now card"
    disposition: "census mapped; freeze (artifact plus corpus manifest plus CI gate) is the next Define before any repair-code commit"
    problem_ref: "live alias normalizers accept legacy values on the live write path; vocabulary seam gates descriptors while tape decode stays unversioned; Event.Validate actor_profile non-emptiness only at tape-append gate; raw spawn synonyms authorize while ReduceCellIntents persists raw roles"
    authorization_ref: "Owner charter ratification 2026-09-10T04:28:08Z; problem-documentation-first per AGENTS.md"
    candidate_or_evidence_refs: []
  - id: versioned-rename-freeze-2026-09-10
    boundary: define
    commit_or_artifact: "freeze commit (this commit): artifact plus manifest plus gate plus CI wiring; no runtime source touched"
    proof_refs:
      - "docs/evidence/choir-rlm-v1-inventory-2026-09-10.json: 308 rows, 894 pinned-query hits claimed, 306 verified, 3 unknown allowlisted"
      - "docs/evidence/choir-rlm-v1-inventory-corpus-2026-09-10.txt: 661-file manifest (self-covering fix 36c529d6)"
      - "scripts/check-v1-inventory.sh PASS (ci mode); --freeze FAILED on exactly the 3 allowlisted owner rows"
      - ".github/workflows/ci.yml v1-inventory job wired into check aggregation"
      - "CI run 34440565624: gate executed and failed on real manifest drift (adversarial proof); manifest fix 36c529d6"
      - "CI run 34440986161 (full-lane dispatch): V1 Inventory Gate completed/success"
    rollback_ref: "revert restores pre-freeze tree and unwires the v1-inventory job; gate proven green in CI run 34440986161"
    disposition: "acceptance item 1 landed with named remainder (owner classification at item 4); mapping-table freeze is next; no repair-code commits before it lands"
    problem_ref: "3 role-shaped owner tokens unclassifiable by orchestrator authority (rematerialize restore intent; revision Role/From); live alias path and unversioned tape decode unchanged"
    authorization_ref: "Owner charter ratification 2026-09-10T04:28:08Z; owner ratification still required for the owner-classification proposal at item 4"
    candidate_or_evidence_refs: []
  - id: versioned-rename-mapping-2026-09-10
    boundary: define
    commit_or_artifact: "docs/evidence/choir-rlm-v2-mapping-2026-09-10.md (this commit); no runtime source touched"
    proof_refs:
      - "§1 per-function V1 acceptor tables never unioned (Canonical 8-branch, NormalizeRole 4-branch plus pinned no-research-branch absence, spawnRoleAllowed, strict prompt registry)"
      - "§2 V2 identity maps with fail-closed defaults; §3 one three-desk forward map with INV-CANON/INV-PROV inverses"
      - "§4 research/research stamp-decides-authority rule; §5 owner-override overlay map; §6 v2 successor receipt field enumeration"
      - "§7 owner-as-frozen-protocol proposal with rationale and named fallback; 3 rows stay unknown-allowlisted pending ratification"
    rollback_ref: "registry-only change; revert restores pre-mapping now card"
    disposition: "acceptance item 4 frozen as Define; widening commit is next; writer cutover waits on owner ratification"
    problem_ref: "every equivalence was implied across four disjoint alias tables; owner token had no classification"
    authorization_ref: "Owner charter ratification 2026-09-10T04:28:08Z; §7 itself proposes, does not settle — owner ratification still required"
    candidate_or_evidence_refs: []
  - id: versioned-rename-widening-2026-09-10
    boundary: implement
    commit_or_artifact: "c19e3028 red(projectionbase): widen known vocabulary set to v1 and v2, never revert"
    proof_refs:
      - "go test ./internal/projectionbase ok (93s); v1-inventory gate PASS locally and in CI"
      - "CI run 34441610853 completed/success incl. staging deploy; staging /health ok serving c19e3028"
      - "verify_test unknown-vocabulary case now pins v3 refusal; TestKnownVocabularySetIsV1V2 pins {v1,v2} with whitespace handling"
    rollback_ref: "never revert this commit (would strand v2-stamped bases); cutover rollback reverts CurrentVocabularyVersion only, in a later revertible commit"
    disposition: "landing-order step 2 landed; frozen V1 decoder with two centralized decode roots is next"
    problem_ref: "v1-stamped bases would refuse install after the writer cutover without the widened known set"
    authorization_ref: "Owner charter ratification 2026-09-10T04:28:08Z; mapping freeze c908494e precedes per problem-documentation-first"
    candidate_or_evidence_refs: []
  - id: versioned-rename-decoder-2026-09-10
    boundary: implement
    commit_or_artifact: "dc413b56 red(decode): centralize Event/CASRequest/DurableEvent decode behind historic and admission roots"
    proof_refs:
      - "computerevent suite ok; projectionbase suite ok (100s); platform and store decode-adjacent subsets ok"
      - "check-v1-inventory.sh PASS (310 rows); check-decode-roots.sh PASS (new guard, wired into v1-inventory job)"
      - "CI run 34445356285 completed/success incl. staging deploy; staging /health ok serving dc413b56"
      - "decoder matrix docs/evidence/choir-rlm-decoder-matrix-2026-09-10.md freezes every raw-entry path"
      - "check-decode-roots caught uninventoried HTTPSource.TailPage mid-slice; migrated to historic root in the same commit"
    rollback_ref: "revert restores per-site json.Unmarshal decodes; roots are behaviorally inert so revert is safe"
    disposition: "landing-order step 3 landed; behaviorally-inert Canonical string-to-tuple refactor (step 4) is next"
    problem_ref: "tape/content decode ran through a dozen ad-hoc unmarshal sites with no version-selected seam"
    authorization_ref: "Owner charter ratification 2026-09-10T04:28:08Z; mapping freeze c908494e precedes per problem-documentation-first"
    candidate_or_evidence_refs: []
  - id: versioned-rename-tuple-2026-09-10
    boundary: implement
    commit_or_artifact: "19dd5116 red(agentprofile): Canonical returns typed unknown-profile tuple, behaviorally inert"
    proof_refs:
      - "build clean; vet clean; agentprofile/toolregistry/coagentowner/actorruntime suites ok; agentcore/store/textureowner focused subsets ok"
      - "CI completed/success with zero failures incl. sharded agentcore/textureowner race suites; staging /health ok serving 19dd5116"
      - "delegated edits reviewed pre-commit: restored a dropped refusal return, removed stray lines and a duplicated fragment, repaired two brace losses"
      - "full unsharded suites time out identically on the unmodified baseline locally (environmental); inventory re-frozen by hunk mapping (310 rows, keys stable)"
    rollback_ref: "revert restores string Canonical; inert so revert is safe at any point before the cutover"
    disposition: "landing-order step 4 landed; migrate/revert/migrate drill ending on V1 serving rows (step 5) is next"
    problem_ref: "stringly-typed Canonical hid unknown-profile passthrough from every enforcement point"
    authorization_ref: "Owner charter ratification 2026-09-10T04:28:08Z; mapping freeze c908494e precedes per problem-documentation-first"
    candidate_or_evidence_refs: []
  - id: versioned-rename-migrate-core-2026-09-10
    boundary: implement
    commit_or_artifact: "b2eccab3 red(migrate): frozen V1-to-V2 row migration core with proven inverses"
    proof_refs:
      - "vocabmigrate suite ok (forward fan-in, inverse, round-trips, provenance, prefixes, protocol-skip, metadata)"
      - "CI completed/success with zero failures; staging /health ok serving b2eccab3"
      - "inventory at 316 rows (+6 map/applier rows by Define update); corpus covers the new file; both gates PASS"
    rollback_ref: "revert removes the unused package; nothing calls it yet so revert is safe"
    disposition: "decode item part 1 landed; serving-fence wiring (part 2) is next, then the staging drill"
    problem_ref: "live rows had no frozen mapping embodiment; forward and inverse lived only as doc tables"
    authorization_ref: "Owner charter ratification 2026-09-10T04:28:08Z; mapping freeze c908494e precedes per problem-documentation-first"
    candidate_or_evidence_refs: []
  - id: versioned-rename-fence-core-2026-09-10
    boundary: implement
    commit_or_artifact: "70057ab7 red(migrate): serving-fence verification core with refusal contracts"
    proof_refs:
      - "vocabmigrate suite ok (fence contracts incl. V2-alias refusal and seam pin)"
      - "CI completed/success with zero failures; staging /health ok serving 70057ab7 (one transient empty response mid-rollout, healthy after)"
      - "inventory at 317 rows (+1 fence-table row by Define update); corpus covers the new file; both gates PASS"
      - "appender enforcement wiring written then fully reverted in-tree: landing refusal before migration/drill is a charter defect"
    rollback_ref: "revert removes uncalled fence code; nothing calls it yet so revert is safe"
    disposition: "decode item fence core landed; enforcement binds paths at the cutover after forward-migration runs; drill design is next"
    problem_ref: "no executable guarantee stood behind the serving fence; migration had no refusal counterpart"
    authorization_ref: "Owner charter ratification 2026-09-10T04:28:08Z; mapping freeze c908494e precedes per problem-documentation-first"
    candidate_or_evidence_refs: []
  - id: versioned-rename-drill-2026-09-10
    boundary: implement
    commit_or_artifact: "219814ca red(migrate): SQL row migration with provenance plus scratch drill proving inverse; 00af1397 red(migrate): break test-only import cycle on seam pin test"
    proof_refs:
      - "TestVocabDrillMigrateRevertMigrate PASS locally and in CI shard 3/6 (0.93s): migrate (fence-v2, joins, authz-equivalence, unknown untouched), revert byte-identical incl. aliases, re-migrate, final revert ending on V1 with fence-v1 clean"
      - "CI completed/success with zero failures; V1 inventory gate green; staging deploy correctly skipped (no reachable behavior change); staging healthy"
      - "import-cycle caught by CI vet (vocabmigrate test vs projectionbase-store edge); pin test moved projectionbase-side, both suites green"
      - "inventory at 321 rows (+4 drill rows by Define update); corpus covers new files; both gates PASS"
    rollback_ref: "revert both commits; migration code has zero production callers so removal is safe"
    disposition: "landing-order step 5 machinery landed on scratch; single writer cutover (step 6) is next with owner-ratified mapping"
    problem_ref: "proven inverse existed only as unit claims over structs, never over real SQL round-trips with joins and authorization"
    authorization_ref: "Owner charter ratification 2026-09-10T04:28:08Z; mapping freeze c908494e precedes per problem-documentation-first"
    candidate_or_evidence_refs: []
  - id: versioned-rename-owner-ratification-2026-09-10
    boundary: define
    commit_or_artifact: "owner ask decision 2026-09-10T15:21:26Z (this commit records it)"
    proof_refs:
      - "now.decision carries selected/kind/source/evidence/time/consequence for owner-as-frozen-protocol"
      - "3 owner rows flipped to verified/frozen with decode-cover positions; check-v1-inventory.sh --freeze: FREEZE ACCEPTED (zero unknown rows)"
    rollback_ref: "registry-only change; revert restores pending-ratification card (cutover re-blocks)"
    disposition: "writer cutover unblocked; single cutover (step 6) with v2 identity/mapping receipt is next"
    problem_ref: "role-shaped owner tokens had no classification; --freeze red and cutover blocked"
    authorization_ref: "Owner ask ratification 2026-09-10T15:21:26Z"
    candidate_or_evidence_refs: []