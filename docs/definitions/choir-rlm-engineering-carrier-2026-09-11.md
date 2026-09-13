---
definition_version: 2
definition_id: choir-rlm-engineering-carrier-2026-09-11
execution_mode: mission_orchestrator
start:
  captured_at: '2026-09-11T12:10:00Z'
  predecessor_receipt:
    mission: choir-rlm-versioned-rename-2026-09-09
    definition_status: completed
    status_token_note: 'The predecessor records `now.status: completed`. That is not one of the Definition skill''s tokens (`working`, `blocked_incomplete`, `complete`, `superseded`). Read it as complete and do not key any gate on the literal token.'
    deployed_sha: e3396329
    settlement_sha: bddd4f00
    ci_run: '34571343061'
    staging_identity: staging https://choir.news serves the mission-two deployed commit e3396329 on the retained computer computer-03335285269bdba4f94377e56879f9e6; effects OFF; pre-A checkpoint 99949fe2.
    deployed_proof: docs/evidence/choir-rlm-versioned-rename-deployed-proof-2026-09-11.md
    entrypoint: false
    next_action: none
    inherited_residues:
      - R6
      - R7
  entry_gate:
    - gate: predecessor_terminal_receipt
      required: 'Read the deployed-proof artifact and confirm the deployed commit, the CI run, the staging deploy identity (https://choir.news/health returns that commit), the effects-OFF state, and the retained computer. Confirm mission two records `entrypoint: false` and `next_action: none` in docs/ACTIVE.md, docs/mission-graph.yaml, and docs/doc-authority-manifest.yaml. Every fact is already in this `start` block; the gate is re-reading them, not re-litigating them.'
      status: satisfied
    - gate: post_mission_two_work_disposition
      required: 'The disk/GC cleanup that followed mission two landed as tracked commits (`internal/store/dolt_maintenance.go`, `internal/vmctl/`, `internal/vmmanager/manager.go`, `nix/node-b.nix`, currently through a907f713). Record its deploy/monitoring receipt and classify it as out of this mission''s scope: this mission owns none of it and must not modify or revert it. At charter, re-run `git status --short` and classify every dirty path.'
      status: satisfied
    - gate: registry_promotion
      required: 'Promote this Definition atomically in docs/ACTIVE.md, docs/mission-graph.yaml, and docs/doc-authority-manifest.yaml as the sole row with `entrypoint: true`. Today the mission is absent from the graph and the manifest, and zero `entrypoint: true` rows exist. Promoted at charter: docs/ACTIVE.md carries a Working Definition section, docs/mission-graph.yaml carries a working spine node with entrypoint true, and docs/doc-authority-manifest.yaml carries a working definition entry.'
      status: satisfied
    - gate: owner_charter
      required: Owner dated the charter statement on 2026-09-11 and answered the four pre-charter questions (charter now; nix/deploy-provider-creds.sh; subscription caps with auto-reload off; model-policy overlay id).
      status: satisfied
  source:
    canonical_ref: main@0bdc1746 (charter base; re-pin if the charter commit differs)
    deploy_identity: staging https://choir.news serves e3396329 plus the post-mission-two GC commits (a907f713); retained computer computer-03335285269bdba4f94377e56879f9e6; effects OFF; OpenCode Go and Zen providers not wired (no OpenCode adapter string exists in Go source today).
  worktree_inventory:
    status: reconciled
    evidence_ref: '2026-09-11 read-only `git status --short` at a907f713: clean tree, three pre-existing untracked paths (`tmp/`, `scripts/__pycache__/`, `scripts/generate_restore_zero_completion_pdf_2026_09_09.py`) left in place. Review-panel reports of tracked GC/vmctl/store WIP described the mid-cleanup state; that work has since been committed as a907f713. Re-verify at charter.'
    preservation_rule: 'Preserve every non-primary worktree and all unrelated WIP. This Definition owns the engineering carrier surface: the eval envelope, the assigned-CoSuper registry and its RLM overlay, reducer intent reduction, run acceptance checkpoints, the engineering-desk prompt assembly, the in-cell session scope, and the OpenCode provider adapters. It owns no GC, disk-maintenance, vmctl ownership, or Node B runtime work.'
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: leave_in_place
      paths_or_digest: three preserved untracked paths (tmp/, scripts/__pycache__/, scripts/generate_restore_zero_completion_pdf_2026_09_09.py); tracked tree clean at 0bdc1746
  candidates:
    - id: none
      ref: none
      base: none
      scope: []
      disposition: unknown
  observed_artifact:
    - claim: 'The evaluation envelope is not minimal: `capsule_go_eval` declares two interchangeable string parameters (`source`, and `code` described as "Alias for source."), an empty required list, a silent source-wins fallback, and then calls `yaegikernel.CleanGoSource`.'
      claim_scope: current
      evidence_ref: internal/agentcore/tools_capsule.go:746-773
    - claim: A call with neither parameter executes an empty cell; a call with both silently resolves to `source`. Neither case fails at the schema boundary.
      claim_scope: current
      evidence_ref: internal/agentcore/tools_capsule.go:754,769-772
    - claim: '`CleanGoSource` strips markdown fences from model-authored Go at five call sites.'
      claim_scope: current
      evidence_ref: internal/yaegikernel/eval.go:301-317; callers tools_capsule.go:773, eval.go:122,148, session.go:86, cmd/capsule-broker/session_worker.go:392
    - claim: Run acceptance builds the `capsule_effect_frozen` and `capsule_verification_recorded` checkpoints by scanning for tool results NAMED `commit_transaction` and `record_self_development_verification`, each behind a `len(results) > 0` guard. The collector skips error results, so retiring a tool silently removes its checkpoint instead of failing the run.
      claim_scope: current
      evidence_ref: internal/agentcore/run_acceptance.go:627-645,281-313,790-791,843
    - claim: 'The two verifier-gated tools cannot pass their gate on the only live CoSuper activation path: both gates require `RunRecord.Metadata["co_super_slot"] == "verifier"`, the assigned CoSuper runtime writes `assignment_kind` but never `co_super_slot`, and the generic CoSuper activation path is refused outright.'
      claim_scope: current
      evidence_ref: internal/agentcore/tools_capsule.go:410-412,491-493; internal/agentcore/cosuper_assignment_runtime.go:342-355; internal/agentcore/runtime.go:987-988. No test pins either gate.
    - claim: 'The reducer is a message router, not a settlement authority: `ReduceCellIntents` commits Message, Spawn and Complete only as mailbox envelopes, while `record_assignment_result` runs the assignment-fate saga (proposition digest, slot gate, freeze/revoke ordering, lifecycle CAS, late-evidence and cancellation races).'
      claim_scope: current
      evidence_ref: internal/agentcore/rlm_reduce.go:150-198; internal/agentcore/cosuper_assignment_fate.go:18-45,525-810; the run loop's detached-terminal predicate calls it at internal/agentcore/runtime.go:3392
    - claim: 'The in-cell message path carries almost none of `update_coagent`''s authority validation: it checks only read-only mutation denial, a non-empty recipient, and tray quotas, and the reducer checks intent count, kind, destination and body. Missing: caller and target durable identity, owner/computer/run/trajectory/context binding, role message policy and CoSuper slot, target existence/profile/channel, typed packet schema validation, lifecycle and work binding, and durable update-id derivation.'
      claim_scope: current
      evidence_ref: internal/yaegikernel/choir.go:201-224; internal/yaegikernel/intent.go:100-121; internal/agentcore/rlm_reduce.go:97-133; validation inventory at internal/agentcore/tools_worker_update.go:179-230,329-753,1145-1415
    - claim: 'Live prompts still instruct retired names: the RLM engineering overlay names `update_coagent` in its tool catalogue and its reporting instruction, and the engineering prompt defaults instruct `record_assignment_result`.'
      claim_scope: current
      evidence_ref: internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml:9,56; internal/runtimeprompts/overlays/engineering_runtime.yaml:7-8
    - claim: 'The `actuator=tools` route is not dead code: the broker serves RLM only when RLM is requested AND the session worker is ready, and otherwise falls back to tools; an empty actuator value parses to tools; the tools branch of the assigned-CoSuper builder composes `RegisterCapsuleLocalTools`, which registers six tools including `capsule_go_eval` and `record_assignment_result`. The RLM branch composes a separate sealed list.'
      claim_scope: current
      evidence_ref: cmd/capsule-broker/main.go:63-79,251-255,674-689; internal/capsule/actuator.go:16-21,37-47; internal/vmmanager/manager.go:1519-1521; internal/agentcore/tool_profiles.go:325-358; internal/agentcore/tools_capsule.go:76-87
    - claim: The assigned-CoSuper registry is an exact closed set of ten JSON tools under the tools actuator, while the RLM overlay already excludes the four capsule file/exec operations. The remainder is hidden from the desk, not deleted.
      claim_scope: current
      evidence_ref: internal/agentcore/tool_profiles_authority_test.go:110-123 (TestAssignedCoSuperBuilderIsExactClosedSet), :271-291 (TestRLMAssignedCoSuperOverlayIsSealedGo)
    - claim: No model-id conditional exists anywhere in prompt assembly or REPL initialization, so the one-prompt invariant holds today by construction rather than by test.
      claim_scope: current
      evidence_ref: internal/agentcore/tool_profiles.go:195-302; internal/runtimeprompts/prompts.go:57-62; cmd/capsule-broker/session_worker.go:274-285; internal/yaegikernel/session.go:53-72; internal/yaegikernel/choir.go:113-132
    - claim: No replay harness, golden-receipt store, canonical-receipt projector, effect census, or forced-rewarm control exists. In-cell reads mint a new request id per call, so a caller-supplied semantic identity does not exist yet and must be built.
      claim_scope: current
      evidence_ref: internal/yaegikernel/choir.go:84-110; no fixture store found under internal/
    - claim: 'The verifier slot is not representable in the worker scope: the capsule request carries only source, cwd, allowed packages, timeout and inbox; the broker passes only the agent role to the session worker; `ChoirScope` carries read-only state but no role or slot and exports only computer and activation identity.'
      claim_scope: current
      evidence_ref: internal/capsule/types.go:101-113; cmd/capsule-broker/session_worker.go:26-33,274-289,336,386; internal/yaegikernel/sidecar.go:166-180; internal/yaegikernel/choir.go:20-61,265-272
    - claim: 'The assigned-CoSuper run cannot select a model: `StartAssignedCoSuperRequest` carries only the objective, kind, candidate, parent work item and tool call, and the run metadata is then enriched from the role policy. The owner-visible eval-arm mechanism (`model_policy_overlay_id`, `System/model-policy-overlays/<id>.toml`) exists on the coagent spawn tool and the Texture prompt-eval API, but not on the assignment path. The roster test therefore needs a model-selection mechanism that does not exist yet.'
      claim_scope: current
      evidence_ref: internal/agentcore/cosuper_assignment_runtime.go:51-57,342-355; internal/modelpolicy/model_policy.go:134; internal/coagentowner/spawn_tool.go:34,54,136; internal/textureowner/api_texture_prompt_eval.go:22-68
    - claim: The gateway and provider request structs carry no session identity and the gateway decoder ignores unknown JSON fields, so an additive optional field is version-skew safe.
      claim_scope: current
      evidence_ref: internal/provider/provider.go:69-105; internal/gateway/handlers.go:32-83,364-368
  unknowns:
    - Live provider facts (session-header enforcement, per-model wire shape, roster call results, image-input matrix) come from the 2026-09-10 research note; phase 1 re-pins them with its own live probes before any roster conclusion.
    - Whether self-development operations can be staged with effects OFF for golden-receipt capture, or whether that class must be proven by recorded fixture. Phase 4 resolves this and the Definition must not assume either answer.
  start_correction:
    - correction: The draft pinned `main@6f1a8014`, staging `cb571960`, retained-computer epoch 896, and described mission two as still holding the working-entrypoint position while blocking this mission on it.
      corrected: 'Mission two is `completed` at deployed commit e3396329 (CI 34571343061, deployed proof artifact published, settlement doc bddd4f00), records `entrypoint: false` and `next_action: none`, and its post-completion GC work landed through a907f713. The stale gate is replaced by `start.predecessor_receipt` and `start.entry_gate`; `now` no longer treats mission two as unfinished.'
      evidence_ref: docs/definitions/choir-rlm-versioned-rename-2026-09-09.md:220-290; docs/ACTIVE.md:248-251; docs/evidence/choir-rlm-versioned-rename-deployed-proof-2026-09-11.md
    - correction: The draft's worktree inventory said "untracked user artifacts only" and derived its claims from a mid-cleanup tree.
      corrected: 'Re-verified at a907f713: clean tree with three pre-existing untracked paths, preserved. The GC commits are in-tree and out of this mission''s scope.'
      evidence_ref: 2026-09-11 read-only git status at a907f713
now:
  status: working
  slice: 'P5 roster BLOCKED at three layers: the persistent Super drainer completed at 20:27:38Z and nothing restarted it, so the A4/A5 tells (cursors 1136, 1137) sit pending with no turn, run or inference call; arms resolve the unfunded base-policy deepseek because the structured model_policy_overlay_id never reaches the assignment (v4 wording written but never exercised); and the capsule-evidence route 413s by a global object bound. Needs an owner action or a product fix to resume.'
  question: 'OWNER DECISION PENDING (2026-09-12): P5-roster cannot run until the activation substrate is repaired or recovered - the deterministic Texture caller 24df8086-9919-51ba-9a98-3e85c1353490 sits in state running with no execution admission and self-seals because its Active() state makes the join no-op, stranding three owner instructions. Options, detailed in problems_discovered entry rewake-instruction-minted-without-caller-2026-09-12: (A) bounded out-of-band recovery then re-tell, no code; (B) authorize a general resident-activation admission repair in internal/agentcore/selfdev_texture_join.go as an explicit extension of this mission''s entrypoints, then measure the roster; (C) settle blocked_incomplete and schedule the substrate repair ahead of missions 4 and 6, which both build on this activation path. Owner is reviewing this mission and docs/reports/choir-rlm-missions-overview-2026-09-09.md before deciding; no technical action is being taken pending that review.'
  reconciliation:
    observed_at: '2026-09-12T20:35:00Z'
    source_ref: main@238ffa99 plus the P5 roster findings (deployed c7bbca4d; CI 34714402217 green; the management desk declined arm 1 as a duplicate of its own open work items at 20:16:43)
    deploy_identity: staging https://choir.news serves c7bbca4d (built 2026-09-12T19:31:09Z, deployed 19:57:30Z); effects OFF; the p5-deepseek-v41-flash overlay resolves to opencode-go/deepseek-v4.1-flash on the retained computer computer-03335285269bdba4f94377e56879f9e6
    authority_identities:
      - docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md (this file, sole entrypoint after charter)
      - docs/reports/choir-rlm-mission-three-review-2026-09-11.md
      - docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md
      - docs/reports/choir-opencode-provider-research-2026-09-10.md
      - docs/evidence/choir-rlm-versioned-rename-deployed-proof-2026-09-11.md
      - docs/mission-residues.md (R7 inherited, R8 deferred)
      - docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md
    policy_resolution_ref: not_applicable (no cell policy resolution observed yet; P5-roster establishes the eval-arm overlay)
    worktree_inventory_ref: 2026-09-11 read-only git status at 009e3c52
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
    selected: 'Owner route bundle 2026-09-11: the reducer authors assignment fate; OpenCode Go/Zen setup is phase 1 inside this mission, using nix/deploy-provider-creds.sh with existing subscription caps and auto-reload off; the actuator=tools deletion is deferred to residue R8 and is never a rollback target; the roster test is an experiment whose expected-pass ids are deepseek-v4.1-flash, muse-spark-1.3 (free while the quota lasts, then the paid id), glm-5.3-flash and gpt-5.6-luna, with every other id recorded and non-gating; a failing model means fixing the shared prompt for every model; the roster gains model selection through an owner-visible model-policy overlay id now, with runtime model policy set inside the Go RLM code as the direction so new model strings need no code change.'
    kind: authority
    status: settled
    source: owner
    evidence_ref: docs/reports/choir-rlm-mission-three-review-2026-09-11.md; receipt engineering-carrier-owner-decisions-2026-09-11
    owner_ratification_ref: owner dated charter 2026-09-11, recorded in receipt engineering-carrier-charter-2026-09-11
    recorded_at: '2026-09-11T13:40:00Z'
    consequence: P0-define is authorized as the first slice. No per-model prompt branch. No reduction of the update_coagent validation without a further owner decision. The four capsule operations stay functional until R8.
  evidence_refs:
    - docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md
    - docs/reports/choir-rlm-mission-three-review-2026-09-11.md
    - docs/reports/choir-opencode-provider-research-2026-09-10.md
    - docs/reports/choir-rlm-missions-overview-2026-09-09.md
    - docs/mission-residues.md
    - docs/evidence/choir-rlm-versioned-rename-deployed-proof-2026-09-11.md
    - docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md
    - docs/evidence/choir-rlm-engineering-carrier-completed-phase-audit-2026-09-12.md
    - docs/evidence/rlm-roster/p5-roster-evidence.md
    - docs/reports/choir-texture-in-cell-packet-reducer-design-2026-09-12.md
    - docs/reports/choir-model-policy-consensus-2026-09-12.md
  blocker_or_risk: 'P5-roster is blocked at three layers, all reproduced on the retained computer. (1) Activation: the deterministic self-development Texture caller 24df8086-9919-51ba-9a98-3e85c1353490 was projected to state running at 20:29:36Z with no execution admission and has produced no inference call, turn or state advance, and because it is Active() the join no-ops, so three owner instructions stay unconsumed (the rewake at seq 1132 and the roster tells at 1136, 1137); the stall is self-sealing. (2) Mechanism: the owner decision named the overlay-id mechanism, but roles.engineering resolves from the base policy to unfunded deepseek/deepseek-v4-flash, and both arms that opened carried an empty structured model_policy_overlay_id, so wrapper v4 is written but never exercised. (3) Evidence: /api/trajectories/{id}/capsule-evidence/{assignment} 413s by a global object bound (internal/store/cosuper_evidence.go:222-227), so the roster receipt and the P4-registry live-catalogue observation cannot be sourced from it; tool names are durable only in provider_call_started events, which no owner-facing route exposes. Carried risks: residue R8 still defers the tools-actuator branch, so the four capsule operations must stay functional; the shared-prompt fix rule can grow the prompt toward the weakest model; and the front-matter truncation class that corrupted this card for forty commits is still undetected by any check. Resolved since charter and no longer carried: the P0 hard gate is met (goldens plus the deployed replay proof are published), the replay harness and the assigned model-selection mechanism both exist, and the reducer authors the assignment fate.'
  problems_discovered:
    - id: landlock-devpts-regression-2026-09-12
      class: discovered
      surface: capsule execution (internal/capsule/landlock.go)
      evidence: 'TestRLMReplayGoldens on Node B: capsule admission failed at Landlock Apply; the path list still carried /dev/pts, which no longer exists in the capsule since the devpts bind-mount was removed - a nonexistent allow path fails closed. Pre-existing regression, not introduced by this mission'
      repair: 'remove /dev/pts from the broker and workload Landlock path sets; verified by the replay driver reaching the interpreter'
    - id: verify-replay-digest-binding-2026-09-12
      class: discovered
      surface: record_self_development_verification replay detection (internal/agentcore/tools_capsule.go)
      evidence: 'replayed verify intent names operation.BundleDigest (the finalized digest) but the early-return compared finalBundle.ContentDigest (the draft digest inside bundle.json); the two gates disagree so a replayed verify could never hit the idempotent path and fell through to a live re-verification attempt'
      repair: 'early return now binds operation.BundleDigest and requires the recorded verifier ref in the finalized bundle receipts; verified by the replay driver row for record_self_development_verification'
    - id: freeze-classifier-always-rejects-2026-09-12
      class: discovered
      surface: freezeCapsuleEffectBundle classifier (internal/agentcore/tools_capsule.go; shared by the retired commit_transaction tool and the in-cell choir.Freeze reducer path)
      evidence: 'capture driver on the pre-cutover build: every real diff rejects with "unknown paths rejected at commit time" because the classifier matches absolute ledger prefixes against RELATIVE upperdir paths (rlm_capture_linux_test.go:607-631). Consequence: the in-cell freeze surface cannot succeed on the live path today - a substrate defect, not a replay-harness artifact. The commit_transaction golden is therefore a rejection receipt, which the P4-review r2 panel ruled does not earn the deletion (P0 falsifier 4: constant-field comparison is vacuous).'
      repair: 'fix the classifier path normalization so real diffs classify; recapture a successful freeze golden on the pre-cutover tree with the fix applied; replay through the reducer path (choir.Freeze staging) against the full P0 row-5 field set'
    - id: replay-harness-vacuity-gaps-2026-09-12
      class: introduced
      surface: internal/agentcore/rlm_replay_linux_test.go
      evidence: 'P4-review r2 findings: (a) rows 5/7/8/9 call the shared bodies directly instead of staging intents through the real choir cell/tray/reducer path; (b) declared_fields are hand-narrowed - record_assignment_result omits proposition_digest/report_id, inspect omits runtime_files/groups, update_coagent omits packet digest/cursor; (c) rlmReplayEffectCensus and the successor adapter exist in rlm_replay_test.go but are never invoked - the "zero-effect census" log line overstates what ran; (d) conflict probes tolerate success-or-any-error instead of requiring a pre-effect conflict; (e) no fresh-operation-on-new-identity leg per row'
      repair: 'route each successor through the real in-cell carrier with the identity journal; generate declared fields from the frozen P0 projection table and assert golden field-set equality; wire the effect census per row; require pre-effect conflict; add the new-identity leg'
    - id: inspect-host-body-orphaned-2026-09-12
      class: discovered
      surface: inspectSelfDevelopmentBundle (internal/agentcore/tools_capsule.go:359-364)
      evidence: 'the in-cell inspector calls inspectMountedBundle (internal/yaegikernel/inspect_bundle.go), not the retained host body; the host body has zero non-test callers. The "shared bodies stay for the reducer" claim was wrong for inspect; two inspector implementations can now drift'
      repair: 'delete the orphaned host body or consolidate both inspectors on one implementation; correct the stale comments (tool_profiles.go:391, tools_capsule.go:75, runtime.go:3311)'
    - id: rlm-prompt-frozen-defects-2026-09-12
      class: discovered
      surface: internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml
      evidence: 'P0 froze two required P4 fixes that did not land: the worked example reads ctx["assignment_id"] but Choir.Context() exports only computer_id/activation_id/co_super_slot (choir.go:322-331); the overlay lists choir.Assign as staged but Assign is a synchronous broker call (choir.go:199-209)'
      repair: 'replace the example context key and move choir.Assign to the synchronous list; add a prompt test asserting the tools-actuator overlay omits retired names (r1 repair currently unguarded)'
    - id: broker-rlm-tools-readiness-fallback-2026-09-12
      class: discovered
      surface: cmd/capsule-broker/main.go:75-80 + capsule.HostSelectsRLM
      evidence: 'when sessionWorkerReady is false the broker degrades go_eval to the one-shot worker (no ChoirScope) while HostSelectsRLM still serves the RLM registry and prompt; post-cutover that desk has zero terminal authority. Latent only because sessionWorkerReady is the constant true'
      repair: 'make the readiness fallback fail the activation with a typed diagnostic instead of silently serving a desk with no terminal authority, or gate HostSelectsRLM on the same readiness signal'
    - id: replay-r4-adjudication-2026-09-12
      class: discovered
      surface: P4-review r4 (.agentic-consensus/p4-review-r4/run/, 5/5 panelists)
      evidence: 'repair (claude, sol, codex) vs accept (muse-spark, opencode). Adjudicated REPAIR: the fresh-Start junk row, failure-unsafe corrupt cleanup, stale next_action, fabricated row-5 state, golden-relative coverage guard, and fixed cell IDs are real defects with small fixes. Majority keeps restamp-with-qualifier over recapture (sol dissent recorded); no product-surface change for the replay marker (sol dissent recorded, exclusion+residue instead); fresh-Freeze independence deferred, not proven (sol/codex major).'
      repair: 'implement the r4 repair batch; verify same state dir three consecutive runs; commit docs-first then code; proceed to the deployed replay proofs'
    - id: replay-golden-provenance-false-2026-09-12
      class: introduced
      surface: docs/evidence/rlm-replay/goldens/ (build_sha on all six goldens + manifest)
      evidence: 'goldens stamped a907f713 (predates the 8f987d7f classifier fix; frozen success on relative upperdir paths is impossible on that tree) then 50fee6ed (committed 04:07Z, 59 min AFTER captured_at 03:08Z; deletes the capture tools). Capture ran 03:08Z on Node B on an 8f987d7f-equivalent tree: frozen success proves the classifier fix, the verify receipt decision field proves the 9f255cd3 receipt line. P4-review r3 + unstick panel (4 repair-first, 1 recapture-required) adjudicated restamp-with-qualifier over recapture.'
      repair: 'restamp build_sha to 8f987d7f with a note naming the equivalence evidence and both false predecessors; no hand-recapture (capture driver is deleted on HEAD, making recapture impossible without resurrection)'
    - id: replay-driver-rerun-poisoning-2026-09-12
      class: introduced
      surface: internal/agentcore/rlm_replay_linux_test.go (freeze fresh-Start leg, corrupt incoming dir, fixed capsule/handle names)
      evidence: 'fixed IdempotencyKey dedups on run 2 (projection-mode Start returns the existing row); corrupt-<digest> dir created inside shared RLM_CAPTURE_STATE with no cleanup, absorbed into later rows incoming baselines; fixed capsule/handle names collide on re-Spawn. Unstick panel unanimous; first Node B run also left corrupt-ea24... pollution in the shared state dir.'
      repair: 'per-run unique suffix on all minted identities; immediate RemoveAll of the corrupt copy after the leg; full-tree copy so rejection is content-caused'
    - id: replay-corrupt-leg-vacuous-2026-09-12
      class: introduced
      surface: internal/agentcore/rlm_replay_linux_test.go (inspect new-identity leg)
      evidence: 'binding "corrupt-"+digest fails SHA-256 validation before any file is read (inspect_bundle.go:85-89), so the corruption was causally inert; err != nil arm asserted nothing; partial copy (draft + one file) could reject on missing files instead.'
      repair: 'bind the valid digest under a fresh operation id, copy the whole bundle tree, require err == nil plus INSPECT_ERR containing frozen runtime file digest mismatch'
    - id: replay-report-null-convention-2026-09-12
      class: discovered
      surface: internal/agentcore/rlm_replay_linux_test.go row 9 vs record_assignment_result.golden.json
      evidence: 'capture receipts record absent scalars as null (verdict, candidate); the repaired view projected "" for both, which CanonicalJSON distinguishes from null. Silent until row 9 first executes (rows 1-6 run first).'
      repair: 'nil-when-empty projection in the replay view; no golden edit (golden nulls are the honest capture shape)'
    - id: replay-row5-state-excluded-2026-09-12
      class: discovered
      surface: commit_transaction golden declared_fields + rlmAssertReplayEquality exclusions
      evidence: 'row-5 state names the freeze-time state but the live row has since moved to awaiting_approval; the live receipt state is a real divergence, and a literal is a fabricated pass. P4-review r4 (claude-4, codex-2) requires exclusion over overwrite.'
      repair: 'drop state from the compared set via named exclusion (golden declared + both projections); carry as accepted claim-narrowing residue'
    - id: replay-fresh-freeze-deferred-2026-09-12
      class: discovered
      surface: internal/agentcore/rlm_replay_linux_test.go freeze new-identity leg
      evidence: 'the leg proves Start identity allocation (distinct object, no dedup, zero residue after row delete), not an independent choir.Freeze of the fresh operation. P4-review r4 (sol-2, codex-3) holds the P4 fresh-operation-through-successor contract unsatisfied for row 5.'
      repair: 'narrow the leg claim to allocation; defer fresh-Freeze execution; revisit if the acceptance review demands it'
    - id: roster-orphan-live-run-blocks-preflight-2026-09-12
      class: discovered
      surface: run:assignment-0f95f728-4381-52d5-93da-62515ce2951a (retained computer) + cmd/choir/roster.go rosterLiveEngineeringRun
      evidence: 'the 402-era chatgpt arm opened 19:06:04Z and stopped updating 19:06:06Z (state running, metadata model_policy_overlay_id null while the prompt named p5-chatgpt-g56luna in prose) and was still running an hour later: the unfunded 402 produced no terminal disposition. Because preflight refuses while any engineering CoSuper run is non-terminal, the orphan silently blocked the next arm and the re-tell could not open. Its parent management run 0d512e91-bcc3-465f-9a91-61ef64d7879e remains running (parked on the assignment).'
      repair: 'cancelled the orphan with choir run cancel (state cancelled, observed); the durable trajectory 24693e87 was NOT cancelled - it is the long-lived document work channel (created 2026-08-20, lifecycle_version 748) the tells target, and trajectory cancel CAS flags forced the read that revealed it. Open: whether a cancelled assignment releases its bound work item so a fresh assignment can open, and the parked parent run'
    - id: roster-arm1-paraphrased-task-body-2026-09-12
      class: discovered
      surface: run:assignment-0f95f728 prompt body vs docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md §7
      evidence: 'the served body is a paraphrase, not the frozen bytes: it reads "(3) run ... directly as one self-contained invocation and record its exit code; (4) complete with ... verdict=none. No paraphrase, no extra steps, no second assignment." where §7 reads the choir.Exec parenthetical, the verdict=none revision note, and the exact-receipt-refs clause. An arm served non-frozen task text is not roster-comparable, and this one would have been counted'
      repair: 'froze the task bytes in-repo at docs/evidence/rlm-roster/p5-frozen-desk-task.txt (1051 bytes, sha256 a0386c97...) and passed them through --expected-task-sha256, which refuses any mismatch; the wrapper is bumped to roster-v2 so v1 receipts remain distinguishable'
    - id: roster-open-work-items-block-next-arm-2026-09-12
      class: introduced
      surface: the trajectory work-item ledger (trajectory 24693e87) + cmd/choir/roster.go roster start
      evidence: 'six ROSTER-V1 execution work items are still open (4c202a6b 15:41, 72c68a04 16:30, 41a5f3d1 16:54, 3d1eb469 17:16, 85c8ab7d 19:04, plus the long-lived d7d7cf61 supervision item), one minted per tell and never dispositioned, while the assignment items (work:assignment-*) are correctly cancelled. On the tell at 20:15:56 the desk committed a turn at 20:16:43 whose reason is "Incorporate the latest management evidence and preserve the single-active-assignment invariant. Do not open a duplicate or claim pending execution results." and applied "The cancellation of the superseded assignment, sole-active-path status, and pending required evidence materially update the canonical execution state." The doc head (revision 16) records the operation as blocked and says a retry needs a new explicit execution request on a viable provider. So the arm was refused for a defensible reason: the harness itself had piled up six open arms, and opening a seventh reads as a duplicate. This is the harness poisoning its own trajectory, and it is the same residue class as the 402 orphan - a superseded arm that leaves non-terminal state behind'
      repair: 'wrapper v3 instructs the desk, before opening the arm, to disposition every earlier open ROSTER-V1 work item as cancelled ("superseded by a newer roster arm") so exactly one active arm remains and this arm is explicitly authorized; no owner-facing work-item disposition route exists, so the desk must do it through its own reporting path. Open: whether the desk holds that authority on this trajectory, and whether the earlier arms left the verdicts the roster needs (they did not - none produced an artifact)'
    - id: roster-base-policy-served-both-arms-2026-09-12
      class: discovered
      surface: internal/modelpolicy/model_policy.go defaultPolicyText roles.engineering + the assignment opener guard + cmd/choir/roster.go
      evidence: 'the base policy routes roles.engineering to provider deepseek / model deepseek-v4-flash, which is unfunded, and both live arms resolved from it: run metadata llm_policy_source "/mnt/persistent/files/System/model-policy.toml" serving deepseek/deepseek-v4-flash, while preflight resolves the arm overlay to opencode-go/deepseek-v4.1-flash. v2 arms carried the id as objective prose and the opener guard refused them loudly; the v3 arm opened with the structured field empty (metadata null) and named nothing in prose, so the guard had nothing to match and the arm silently served the base policy - the wrong-model spend the guard exists to prevent, with the failure moved from loud to silent. The parent management run prompt is 64 chars and carries neither the overlay id nor a marker, so the opener has no honest signal for a mandatory-field rule.'
      repair: 'wrapper v4 states the argument and its value as a JSON argument value ("never text inside the objective") while avoiding the refused literal; roster collect now reads llm_policy_source, records policy_source plus the served provider/model, and fails the receipt loudly the moment an arm was base-served instead of polling 30 minutes toward a pass. Open: whether the desk complies with v4, and if it does not, the owner decision named the overlay-id mechanism, so adopting the acceptance-sanctioned per-run engineering base-policy swap is an owner call rather than an orchestrator substitution'
    - id: capsule-evidence-global-bound-blocks-p5-2026-09-12
      class: discovered
      surface: GetCoSuperCapsuleEvidence object-graph bound (internal/store/cosuper_evidence.go:222-227) surfaced at /api/trajectories/{id}/capsule-evidence/{assignment}
      evidence: 'the route returned 413 "capsule evidence exceeds response bounds" for a two-second cancelled arm. The bound is not per assignment: the loader reads the whole owner+computer object snapshot and refuses when len(objects) > coSuperEvidenceMaxObjects, so on a computer that has accumulated months of objects the projection is permanently unavailable for every assignment, however small, and roster collect''s capsule-evidence read is dead on the retained computer. Fail-closed, but keyed on a global quantity rather than the assignment''s own closure'
      repair: 'not repaired: bound the projection to the assignment closure (assignment, fate steps, reports, capsule objects) instead of every object in the graph, or measure the closure before the snapshot. Recorded so the roster receipt and the P4-registry live-catalogue observation do not silently depend on a route that cannot answer on this computer'
    - id: persistent-super-drainer-completes-tells-strand-2026-09-12
      class: discovered
      surface: persistent Super drainer lifecycle (internal/agentcore/super_controller.go reconcilePersistentSuperActor and its entry paths)
      evidence: 'the drainer run 1ad1dc39-8fcc-48ff-8d30-61862ac9347f ("Process pending coagent update packets for privileged execution.") drained the A2 and A3 tells while resident and then COMPLETED at 20:27:38Z instead of passivating. Nothing replaced it, and the next two tells (cursors 1136 and 1137, status pending) produced no turn, no run and no inference call. Reconcile returns a resident run when one exists; the other entries are the owner self-development start/retry path (reconcilePersistentSuperActorForOwnerStart, reachable only from startSelfDevelopmentPersistentSuper, exposed in the CLI as self-dev mode get|set with effects OFF) and boot rewarm (ResumeInterruptedPersistentSuperControlRun, which reactivates only runs passivated by a process restart - the drainer completed, so a reboot is not its recovery either; super_controller.go states boot "is a recovery event, never a scheduler tick").'
      repair: 'not repaired by this mission: it is the product owner-instruction substrate, and the mission holds no surface for it. Two consequences recorded: the P5 roster cannot open another arm on the retained computer until an owner self-development start/retry restarts the drainer or the drainer is fixed to persist; and the harness must not read a pending tell as progress. Escalated in now.next_action as a separate owner decision rather than patched in-mission.'
    - id: rewake-instruction-minted-without-caller-2026-09-12
      class: discovered
      surface: self-development Texture rewake and caller activation (internal/agentcore/selfdev_texture_join.go; not in this Definition's entrypoints.implementation)
      evidence: 'the rewake path did fire. Cancelling arm A3 through the owner run route drove cancelBoundCoSuperRun -> revokeAssignedCapsule -> persistSystemCoSuperCancellation, and the reducer recorded queue:owner-instruction-selfdev-rewake-3133cda7-f299-5bb8-9e02-c6acf0fa30c5 (seq 1132). So the instruction is minted; what fails is execution. ensureSelfDevelopmentTextureCaller derives the deterministic caller run (uuid5 over the OID namespace of "choir:texture:self-development:<owner>:<computer>:<trajectory>:texture-run" = 24df8086-9919-51ba-9a98-3e85c1353490) and reactivateSelfDevelopmentTextureCaller then writes it to state running through ReplaceLifecycleActivation - a durable state projection with no execution admission. That run is live: state running, profile texture, prompt "Supervise self-development on this computer.", updated_at 2026-09-12T20:29:36Z, and no inference call, turn or state advance since. Because it is Active(), every later ensureSelfDevelopmentTextureCaller returns it as resident, so the join no-ops and the reducer keeps three unconsumed owner instructions (the rewake at 1132 and the roster tells at 1136, 1137). The stall is self-sealing. The rewake body also narrows itself hard for an operation with no bound Super runs, continuing unless the operation id is exactly selfdev-ccf0f1ec0e851750f253fe5f5ed97974, while the retained computer''s operation is selfdev-8dcdd2c5e7841addb24b0c7991f09a5c.'
      repair: 'not repaired, deliberately, and the phantom was left in place as live evidence rather than cancelled: the surface is the product self-development Texture-caller lifecycle (internal/agentcore/selfdev_texture_join.go:408-500, not among this Definition''s implementation entrypoints), cancelling would not help because the next join call would re-project another phantom, and it would destroy the diagnostic. The missing piece is execution admission for a projected run - the same class the code comment warns about in the opposite direction as the multiple-runs defect. Recovery needs an owner action or a product fix.'
    - id: texture-wake-one-shot-occurrence-fragility-2026-09-12
      class: discovered
      surface: texture owner-instruction wake chain (textureowner.HandleTextureOwnerInstruction scheduleTextureWorkerWake -> agentcore DispatchActor -> actorruntime handler reconcile)
      evidence: 'live re-derivation 23:47Z: texture watch on doc 040930e8 shows cursors 1136 (queued 20:32:48Z) and 1137 (20:42:26Z) still the last events, status pending, trajectory live, three hours stranded. The tell route queues durably then fires exactly one occurrence-keyed wake (texture_owner_instruction.go:152-156 comment: exactly one actor wake follows a new commit); if that wake does not execute there is no retry authority and no timer. Post-20:42 runtime logs show zero occurrence/turn/inference activity and zero retry-boot-dispatch lines, so the boot flush is not wedged; the wake was silently dropped or permanently deferred with no log. The 20:29:37-41Z window proves the chain can execute (revision runs minted, funded chatgpt inference) and then died at the A3 cancellation fate-join error, after which A4/A5 never woke.'
      repair: 'not repaired yet, owner-authorized as part of the completion route: pending-occurrence wake must gain durable retry or reconciliation authority (re-dispatch of the same occurrence identity is idempotent and safe), and the silent nil-nil drop paths in the handler texture branch need loud receipts. P5 repair proceeds on this surface, not only on the caller projection.'
    - id: assigned-fate-join-cancellation-race-2026-09-12
      class: discovered
      surface: assigned CoSuper fate join on execution error (internal/agentcore/runtime.go:4477-4486 handleExecutionError -> terminalizeRun)
      evidence: 'live log 20:29:41Z on the retained computer runtime: "assigned CoSuper execution error could not join assignment fate for run run:assignment-239f7309-78ca-5db7-9e4a-dc757559d013: context canceled (cannot cancel run in cancelled state)". An owner-cancelled run that also errors at execution tries to terminalize as failed and the fate join refuses the double-terminalization, so the error path logs a fate-join failure instead of recognizing the run is already terminal. The arm outcome still records cancelled, but the join failure aborts the fate chain mid-sequence.'
      repair: 'not repaired yet: handleExecutionError for assigned CoSuper runs should read the stored run state first and no-op the terminalize when the run is already terminal (cancelled/failed/completed), letting the existing fate receipt stand.'
    - id: pre-genesis-admission-refusal-loop-2026-09-12
      class: discovered
      surface: pre-genesis admission gate (internal/agentcore/runtime.go:750-761) on one autoputer runtime on Node B
      evidence: 'one autoputer runtime (container prefix safvdbs8l3yfwgrnm3wvfsrj32f9ryrw, vmctl-exec pid 1460048) refuses internal run admission continuously since at least 2026-09-06 at roughly eleven thousand refusals per day with "computer is pre-genesis: run admission refused (bootstrap-chain required; no canonical genesis on the tape)", still ongoing at 02:45Z Sep 13. Attributed via the vm-state ownerships registry: the refusing VM is candidate-fleet-d03dacaa7404b1e4412b2e6f = computer-4c20ff4a21a021c4306d8c783be0037d, active since 2026-07-17, which has NO head row anywhere (platform computer_event_heads has rows only for 033352 seq 151843 and aa7739a6 seq 225). The computer is genuinely pre-genesis, created before the chain-bootstrap contract. The refusal log line still carries no computer id.'
      repair: 'partially repaired: identified. The chartered owner bootstrap-chain route returned http 403 on this key ("api key computer ownership required") - computer-4c20ff4a belongs to a different user, so the genesis bootstrap needs that owner or the owner authority. Recorded for the owner decision; separate from the retained computer''s stall.'
    - id: staging-deploy-impact-classifier-diffs-previous-head-2026-09-13
      class: discovered
      surface: CI Deploy to Staging impact classifier (.github/scripts/deploy-impact-classify via ci.yml deploy-impact)
      evidence: 'forced deploy run 34732934273: the classifier fetched the PREVIOUS run head 47e8ff81 as its diff base instead of the deployed receipt target c7bbca4d, classified the accumulated runtime delta (modelpolicy, modelcatalog, gateway handlers, actor kernel, nix deploy script) as "only docs, workflow, or Playwright test artifacts changed" and skipped the deploy. The host deploy only landed via an explicit force_staging_deploy workflow_dispatch.'
      repair: 'not repaired: the classifier should diff against the deployed receipt target_commit (single state authority for deploy identity), not the previous run head. Recorded so the next platform deploy cannot silently skip accumulated runtime changes after intermediate failing runs.'
    - id: retained-computer-guest-pinned-to-constructed-code-ref-2026-09-13
      class: discovered
      surface: constructed-computer identity cutover (ci.yml active VM refresh preserving immutable realizations; AGENTS.md product-restore doctrine)
      evidence: 'the forced staging deploy to 425f2e45 preserved candidate-fleet-e15cb89f25d963c220319b7b (the retained computer) as an immutable constructed computer and left active_computers empty in the deploy receipt: the guest runtime deliberately stays pinned to its constructed CodeRef (c7bbca4d build). The substrate repairs (fate-join terminal-wins, caller-provenance rule, actor duplicate-wake activation) are live on the host services but NOT on the retained computer''s guest runtime until its constructed-computer identity is cut over through the product path.'
      repair: 'not repaired: the guest cutover is the mission''s own landing mechanism (the deployed staging cutover artifact item); the deterministic tells 1136/1137 stay stranded on the pinned guest until then. No platform-side fix may target it - product restore is a separate forward transaction.'
    - id: retained-computer-guest-cutover-2026-09-13
      class: repaired
      surface: owner-scoped lifecycle refresh on computer-03335285269bdba4f94377e56879f9e6 (the product path for constructed-computer identity advance)
      evidence: 'POST /api/computers/computer-03335285269bdba4f94377e56879f9e6/lifecycle/refresh (idempotency_key carrier-9ba782b8-guest-cutover-20260913) at 04:08:22Z: receipt 01a098f3-afe1-77f0-823a-a763baad04e8, epoch 913 -> 914, state active->active; the refreshed guest serves build 9ba782b8 (the deployed staging commit with every substrate repair) at http://10.200.8.2:8085/health status=ready; G4 preserve logic untouched - the refresh is a forward lifecycle transaction on the computer''s own event chain. The wake chain verified live on the new guest: boot sweeps ran, the desk''s texture turn at 04:13:53Z (cursor 1138, doc revision 18) consumed the rewake 1132 AND the stranded roster tells 1136+1137 in one commit, work item 2711e765 (the roster-v4 arm wrapper) opened at cursor 1140, and its control bound to a fresh drainer run 2454bdd4 at 04:14:32Z (cursor 1142).'
      repair: 'cutover landed; the guest now runs the substrate repairs.'
    - id: fresh-mint-drainer-dispatch-strand-2026-09-13
      class: discovered
      surface: persistent-Super drainer initial_dispatch (actorruntime/handler.go Super live-occurrence branch trusting the locked mint)
      evidence: 'drainer 2454bdd4 minted 04:14:01Z (state pending, bound to the roster control) never executed: no inference, no dispatch log, no retryable metadata, still pending 45 minutes later. The live-occurrence handler binds and returns nil on the codified assumption "Locked mint already dispatched initial_dispatch"; the resume watchdog (persistentSuperResumeDispatchDeadline, 10m) arms only runs with actor_reactivated_from_passivated - a fresh mint has no watchdog and no retry authority. The boot window is the loss window: dispatchActor queued the dispatch while boot phases were still running. Worked around through the designed owner path: run cancel (terminalizes the strand with a fate receipt) then computer restart (epoch 915); boot recovery re-drove a fresh drainer (db310c86) which executes with tool loops and funded-model inference.'
      repair: 'not repaired in source: the bind branch should arm the same dispatch deadline watchdog (or enqueue a recovery occurrence) for a pending resident past the deadline instead of silently trusting the mint. Same defect class the texture owner-instruction wake repair closed on the texture path; the management path still has it.'
    - id: guest-startup-fallback-names-deleted-provider-2026-09-13
      class: discovered
      surface: autoputer guest startup provider fallback (computer-owned config on the retained computer''s persistent store)
      evidence: 'the refreshed guest logs "autoputer: using gateway provider (url=http://10.200.9.1:8084 provider=deepseek model=deepseek-v4-flash reasoning=medium)" at every boot; the repo-side heresy deletion (f8db2ed1) removed deepseek from policy defaults, ladder, catalog, gateway, and nix, but this computer-owned startup configuration still names the deleted provider, and the gateway refuses those calls at request time (observed 04:11:09Z: "provider resolution failed ... unsupported provider: deepseek"). Fail-closed on every path that falls through to it.'
      repair: 'not repaired: computer-owned persistence, not repo source - the guest''s startup provider configuration needs an owner-scoped file correction (the same class as the live policy swap), and the heresy sweep for computer-owned configs is not a repo citer target.'
    - id: desk-prose-claim-vs-reducer-state-2026-09-13
      class: discovered
      surface: desk turn authoring on doc 040930e8 (model gpt-5.6-luna through the texture surface)
      evidence: 'the desk''s 04:13:53Z turn text asserts "exactly one fresh ROSTER-V1 implementation assignment is durably bound" while the trajectory reducer_seq (1142 at the time) shows no co_super assignment event - the claim is the desk''s prose belief, and its 04:36:59Z turn (cursor 1144, A6 arm) inherited it and queued no new control ("no second assignment is opened"), leaving the roster arm dependent on the pending control bound to the cancelled drainer. The drainer''s FIFO selection then served older pending controls first (run db310c86 bound work item 679641d3 on the OTHER supervision channel d8ccd11b/doc 78d0dc36), so the roster arm is queued behind the backlog drain.'
      repair: 'not repaired: the desk''s evidence surface still treats its own prior prose as durable evidence over reducer state; the roster receipt already fails loudly on base-served arms (policy_source_not_arm_overlay), so the remaining exposure is desk-belief divergence, recorded here for the P5 review panel.'
    - id: definition-front-matter-unparseable-2026-09-12
      class: discovered
      surface: this Definition's front matter (now.decision.selected and now.blocker_or_risk)
      evidence: 'both scalars were cut at 769 characters with an ellipsis appended and their closing single quote lost, so yaml.safe_load fails on the document - the first fatal is now.decision.selected - and any strict parser, including the skill dashboard generator named in view.generator, cannot read the Definition. The truncation predates this session and is present in every commit from 0b2f0fe6 (selected) and 8b6dd145 (blocker_or_risk) onward, i.e. through roughly forty commits including the whole P1-P5 run, so the mission card has been unreadable by tooling while being cited as authority.'
      repair: 'restored both scalars from history instead of inventing text: selected from 73815790 and blocker_or_risk from 0b2f0fe6, each first verified to be a pure truncation (the current 768-character prefix matches the ancestor exactly) and each ending with its quote; the front matter now parses under strict YAML. It went unnoticed because the skill''s own reader is parseYamlSubset (skills/definition/scripts/dashboard.mjs:326), a lenient subset parser that tolerates the unterminated quote, and no workflow validates the working entrypoint''s front matter. Not repaired: nothing in the repo detects this class, so a write path that clips a line at 768 characters and drops the closing quote can silently corrupt content in any Definition again - the content loss is independent of which parser reads it'
  next_action: 'P5-roster is BLOCKED at three layers, all recorded: (1) activation - the persistent Super drainer completed at 20:27:38Z and nothing restarted it, so the A4 and A5 tells (cursors 1136, 1137) sit pending with no turn, run or inference call; recovery needs an owner self-development start/retry or a product fix, and the mission owns no surface for it. (2) mechanism - the owner decision named the overlay-id mechanism, but the base policy routes roles.engineering to unfunded deepseek/deepseek-v4-flash, so any arm whose structured model_policy_overlay_id does not reach the assignment silently spends on an unfunded provider; v4 wording is written and committed but has never been exercised, because no tell has been driven since it landed, so whether the desk complies is still unmeasured. (3) evidence - the capsule-evidence route 413s by a global object bound, so the roster receipt and the P4-registry live-catalogue observation cannot be sourced from it. Requested decisions: restart the drainer (owner action) and, once a tell is drainable, either keep the overlay id or adopt the acceptance-sanctioned per-run engineering base-policy swap; the swap needs no model compliance and the loader re-reads the policy file on every resolve.'
  deliver: 'The engineering desk lives entirely on the in-cell carrier and nothing else: `capsule_go_eval` is the desk''s only JSON envelope, every other affordance is a typed in-cell function staging intents for the one reducer, the five overlay JSON tool names are deleted rather than hidden and each earned its deletion by replay proof, the assignment fate is authored by the reducer, run acceptance no longer keys on tool names and fails loudly when evidence is missing, one model-independent prompt with one REPL initialization serves the expected roster with zero output repair, and the proved replay harness plus its fixtures remain as durable evidence.'
  artifact: 'One deployed staging cutover on https://choir.news with effects OFF: the simplified envelope, the reducer-owned settlement path, the in-cell freeze/verify/inspect surface, the closed assigned registry, the frozen roster conformance evidence, the replay harness with golden receipts, and the closed R7 residue with R8 opened, and one adjudicated review receipt per frozen boundary (P0, P3, P4, P5).'
  non_gating_artifacts:
    - 'Texture packet/reducer design document: translates documents, patches, diffs, source graphs, controls and dispositions into the same in-cell discipline, stated as constraints against the canonical writer''s invariants and citing them from their own authority. No Texture runtime code lands in this mission. This artifact must not gate completion.'
  entrypoints:
    implementation:
      - internal/agentcore/tools_capsule.go
      - internal/agentcore/tool_profiles.go
      - internal/agentcore/rlm_reduce.go
      - internal/agentcore/cosuper_assignment_fate.go
      - internal/agentcore/cosuper_assignment_runtime.go
      - internal/agentcore/run_acceptance.go
      - internal/agentcore/tools_worker_update.go
      - internal/yaegikernel/eval.go
      - internal/yaegikernel/session.go
      - internal/yaegikernel/choir.go
      - internal/yaegikernel/sidecar.go
      - internal/capsule/types.go
      - cmd/capsule-broker/session_worker.go
      - internal/provider/provider.go
      - internal/gateway/handlers.go
      - internal/modelcatalog/catalog.go
      - internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml
      - internal/runtimeprompts/overlays/engineering_runtime.yaml
      - internal/promptstore/defaults/engineering.yaml
  acceptance:
    - action: 'P0-define (code-free; first mission commit; zero repair code): record the four discovered defects (envelope alias with empty required; tool-name-keyed acceptance checkpoints; reducer-is-not-a-settler; verifier-slot unreachable on the assigned path); publish the frozen nine-operation mapping table (old JSON name, in-cell successor or intent kind, receipt class, canonical identity fields, pre-declared exclusion list for nondeterministic fields, fixture path); publish the frozen prompt/REPL initialization manifest and the entropy-exclusion list (exact field paths) used by the digest comparison; publish the falsifiers; and publish the replay-harness specification (fixture schema, canonicalizer, effect census, durable receipt store, rewarm procedure). The four defects are new problems, so this boundary precedes any repair commit.'
      proves: Scope, identity and falsifiers are evidence-bounded before any carrier code moves.
      evidence_class: static_analysis
    - action: 'P0-review (agentic consensus, frozen-artifact review): bind the review to the P0-define artifact by base ref, scoped paths and content digest; run the bundled panel (skill://agentic-consensus, skills/agentic-consensus/agentic-consensus-runner.sh, convergent, --prompt-file, --out-dir under .agentic-consensus/) with one decision question: do the frozen nine-operation mapping table, its receipt classes and canonical/exclusion lists, the replay-harness specification and the prompt/REPL manifest actually support per-operation replay proof and deletion, and where would that proof be vacuous? Re-verify every load-bearing panel claim locally in source before acting; fold accepted findings into the artifact; adjudicate one outcome (accept, repair, reject, escalate) with the reason; record the panel''s cost, latency and failure modes. A panel is an evidence receipt bound to the frozen artifact, never a vote, and it may not change scope, authority or the evidence floor without an owner decision. Fold the outcome into the next implementation commit; do not create a standalone consensus commit.'
      proves: The artifact that authorizes every later deletion survived adversarial review before any code moved.
      evidence_class: static_analysis
    - action: 'P1-provider (red; its own Landing Loop): wire the OpenCode Go and Zen providers. Freeze, before implementation, the model-to-wire-shape map (chat completions, Responses, Anthropic Messages), the `conversation_id`/session identity bound to the durable `RunID`, the product User-Agent, the fail-closed rule for an empty identity, the credential variable names, a prior host-config digest and backup, and a spend cap. Deliver the keys through the authorized product/CLI path, or record an owner-executed break-glass step if no product path exists; never ad hoc SSH. Prove one live call per wire shape plus the empty-identity negative probe, with the receipt fields (request shape, model id, status, latency, identity present) named in the evidence artifact.'
      proves: Cross-model comparison is never gated on credential plumbing, and a provider outage can never be mistaken for a prompt or carrier failure.
      evidence_class: deployed_proof
    - action: 'P2-simplify: reduce `capsule_go_eval` to a single required `source` string, delete the `code` alias, cut the description to one sentence stating the affordance, and move the explanation into the desk''s static prompt body. A call with no source and a call with a fenced source both fail as parse or compile errors; neither executes an empty cell or gets repaired.'
      proves: The affordance the prompt describes is the only affordance the schema offers.
      evidence_class: local_test
    - action: 'P2-repair-deletion: delete `CleanGoSource` and its test and all five call sites, including the broker worker path. Prove zero references remain; prove a fenced cell now fails with an actionable error instead of being silently stripped; and state the non-regression for the shared broker session path that the `actuator=tools` fallback still uses.'
      proves: No code-level output repair exists on the carrier; a model that wraps code in fences is visible, not absorbed.
      evidence_class: local_test
    - action: 'P2-invariant: compute the desk prompt digest from the prompts ACTUALLY assembled during the roster runs, and assert equality across roster models for a fixed desk against the frozen entropy-exclusion list from P0-define. Ship the standing guard that fails when any model id or per-model branch appears in the prompt-assembly or session-initialization path, and re-pin the digest after the P4-registry catalogue rewrite changes the prompt text.'
      proves: The prompt varies by desk and never by model, and the property cannot regress silently.
      evidence_class: local_test
    - action: 'P3-settlement (red): make the reduction path the assignment-fate author. The in-cell Complete intent must produce the same durable fate as today''s tool: same assignment/attempt identity derivation, same proposition digest, same lifecycle compare-and-swap, same freeze and revoke ordering, same late-evidence and cancellation-race behaviour, and the partial (non-terminal) result path. Then delete the tool: registration, handler, the detached-terminal name predicate in the run loop, and the admission-grammar special cases that exist only for it. Prove single authorship: zero live references to the retired name; duplicate Complete intents are idempotent (replay returns the original fate receipt and writes no second fate step); a reused identity with changed input conflicts before any effect; a crash between message and fate writes is recoverable.'
      proves: Settlement has exactly one author under the new carrier, with the old semantics preserved.
      evidence_class: deployed_proof
    - action: 'P3-acceptance (red): rebuild the `capsule_effect_frozen` and `capsule_verification_recorded` checkpoints so they derive from canonical events and in-cell receipts, not from tool results named `commit_transaction` and `record_self_development_verification`. Record the checkpoint count before the change as a baseline and require it not to decrease. Make a run that lacks the evidence fail loudly and name the missing evidence. Prove the negative on staging: a run without the in-cell freeze/verify evidence must not report an accepted level.'
      proves: Retirement cannot silently weaken run acceptance, and the acceptance score stops depending on tool names.
      evidence_class: deployed_proof
    - action: 'P3-parity: close both parity gaps before any deletion, without an escape hatch. (a) Carry the verifier slot into the cell: capability role, the capsule eval request, the session worker config, and `ChoirScope` must all carry the durable assignment kind/slot, and an in-cell function must expose it. Add the tests that pin both verifier gates, which do not exist today, and make the assigned verification run populate the slot so the two verifier-gated capabilities are reachable. (b) Carry `update_coagent`''s authority checks into the staged message path: caller and target durable identity, owner/computer/run/ trajectory/context binding, role message policy and CoSuper slot, target existence and channel, typed packet schema validation, lifecycle/work binding, and durable update-id derivation. Desk-scope the retirement and prove Texture and research citers keep working. If any check cannot be carried, the substitution is an owner decision recorded in `now.decision` before deletion; an orchestrator may not self-approve a reduction.'
      proves: Retiring a tool does not retire the authority or the checks that tool performed.
      evidence_class: local_test
    - action: 'P3-in-cell-surface: provide the missing in-cell affordances: a staged freeze intent, a staged verify intent, and a synchronous read-only bundle inspection under the overview''s read-only exemption. Each is bound to the P0-define mapping table''s receipt semantics.'
      proves: Every affordance the desk needs exists on the carrier before the JSON remainder disappears.
      evidence_class: local_test
    - action: 'P3-review (agentic consensus, red-boundary review): freeze the candidate commit for the settlement, acceptance and parity work and review exactly those diffs, asking where two settlement authors, a silently weakened acceptance checkpoint, or an unreachable verifier gate could still survive. Re-verify each claim in source, adjudicate and fold. A finding that would change authority or the evidence floor escalates to the owner instead of being applied.'
      proves: The three red changes are adversarially checked before the deletions that depend on them.
      evidence_class: static_analysis
    - action: 'P4-harness: build the replay harness before any golden receipt is captured: versioned fixture driver, canonical-receipt projector with the pre-declared exclusion list, effect census reader, durable receipt store, legacy capture adapter, successor adapter, a caller-supplied semantic identity for in-cell operations (they mint a new request id per call today), a state-mutating read fixture, and a forced actor/host rewarm control that is reachable without SSH or is named as an owner-executed break-glass step. Capture goldens against the pre-cutover deployed build. Declare explicitly which operation classes are proven by recorded fixture instead of live capture, and why.'
      proves: The replay proof can exist, and its substrate is durable rather than improvised at deletion time.
      evidence_class: local_test
    - action: 'P4-replay: for each retired operation, invoke the successor under the same semantic identity and require canonical equality on the declared fields (never raw byte equality), zero additional effects by the effect census, a pre-effect conflict on reused identity with changed canonical input, and a fresh operation on a new identity. For read-only operations, mutate the underlying state between calls: the same identity returns the original observation and a new identity observes the change. One deployed proof per operation class, with the local harness run per operation.'
      proves: 'Each deletion is earned: replay through the new path returns the original receipt and no more.'
      evidence_class: deployed_proof
    - action: 'P4-registry: cut the assigned-CoSuper RLM registry to the eval envelope alone. Assert exactly: `TestAssignedCoSuperBuilderIsExactClosedSet` covers the tools branch, and the RLM overlay test asserts the sealed set contains only `capsule_go_eval` plus the reconciliation and report channels that stay, with the overlay catalogue sentence naming that one tool and the in-cell surface. Observe the live catalogue on staging to prove the singleton registry, since a unit test cannot prove the deployed catalogue. Unknown JSON tool names fail closed.'
      proves: Engineering has one envelope and the JSON remainder is deleted rather than hidden.
      evidence_class: deployed_proof
    - action: 'P4-delete: delete the five overlay JSON tool paths and their reducer aliases, including the admission-grammar special cases that exist only for them, with the citer sweep in the same change. Do NOT delete the four capsule file/exec operations, which the `actuator=tools` fallback composes until R8. Update every live prompt that names a retired tool, including the RLM engineering overlay (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`), the engineering overlay (`internal/runtimeprompts/overlays/engineering_runtime.yaml`) and the engineering prompt default (`internal/promptstore/defaults/engineering.yaml`), so no served prompt instructs a name that no longer exists.'
      proves: No dual path survives on the desk, and the deferred branch still compiles and serves.
      evidence_class: local_test
    - action: 'P4-review (agentic consensus, deletion-candidate review): bind to the frozen candidate that cuts the assigned registry and deletes the five overlay names, and ask where legacy behaviour, prompt citers, or the deferred tools fallback would break. Re-verify claims in source, adjudicate and fold. If the candidate changes materially after the review, the review is stale and reruns proportionately.'
      proves: The deletion set is bounded by evidence, and no citer or fallback breaks on it.
      evidence_class: static_analysis
    - action: 'P5-roster: run one frozen desk task (inspect, edit, run a test, complete with the exact receipt reference) under one prompt, with the task artifact path fixed in P0-define. All four expected-pass ids must complete it (`muse-spark-1.3-contributor-free` or the paid `muse-spark-1.3-contributor` counts as one id). Every other roster member is an experiment: run it, record pass or fail, tokens, latency and failure mode, and never let it gate. Record which id served each run so a quota switch is never read as a model failure, and exclude `hy3` from any image-bearing step by name. Resolve the model-selection mechanism before this item can run - an owner-visible model-policy overlay id on the assignment path, or a per-run engineering policy swap that is restored afterwards - and name the chosen mechanism in P0-define. The assignment path cannot select a model today.'
      proves: One prompt genuinely serves the expected roster, which is the owner's completion goal.
      evidence_class: deployed_proof
    - action: 'P5-prompt-fix: when a model fails, fix the shared prompt for every model, never a per-model branch, hint, retry ladder or schema fork. Record the digest and size of every revision and re-run the full expected roster against the new prompt. Bound the loop: after three shared revisions without all four expected ids passing, stop and record `blocked_incomplete` with the honest result rather than continuing to grow the prompt.'
      proves: A model failure is treated as a prompt defect or an honest blockage, never as an accommodation.
      evidence_class: local_test
    - action: 'P5b-tools-actuator: prove by test that the RLM profile never reaches the `actuator=tools` registry, and keep that branch functional and unused. Register residue R8 in `docs/mission-residues.md` at settle, naming the deferred deletion and its revisit trigger (management and research desks crossing to RLM). No mission proof may depend on the branch and no rollback path may target it.'
      proves: The deferred branch is provably out of the desk's reach while remaining a working fallback.
      evidence_class: local_test
    - action: 'P5-review (agentic consensus, roster interpretation): present the frozen roster results (per-id pass/fail, tokens, latency, failure mode, quota switches) and ask whether the one-prompt invariant holds or a per-model accommodation has crept in. Adjudicate the only responses the mission may take - exclude a model with a recorded reason, or fix the shared prompt for every model - and escalate to the owner if the answer would change the mission''s stopping condition.'
      proves: The one-prompt claim is judged from evidence rather than asserted from a green run.
      evidence_class: static_analysis
    - action: 'P6-landing: run the Landing Loop on behaviors changed here, with an intermediate deploy before the deployed replay and roster proofs (later phases cannot supply earlier deployed evidence). Run the exact deployed engineering-assignment scenario and record the accepted run and acceptance ids, the trace evidence that no legacy tool was used, and the traces that prove reducer settlement. Then move docs/ACTIVE.md, docs/mission-graph.yaml and docs/doc-authority-manifest.yaml atomically, close residue R7, leave R8 open, and settle this Definition. Already-accepted runs are not retroactively rescored.'
      proves: The cutover is proven on the deployed product path, and the mission record is closed with artifacts.
      evidence_class: deployed_proof
  rollback: 'Phase-specific, plus one source rollback. Source: revert the mission commits and redeploy; the retired JSON paths return with the revert. Phase 1: revert the adapter commits, remove the installed Node B credential, restart the gateway, and prove staging health returns to the pre-phase-1 identity; the adapter path is greenfield (no OpenCode adapter exists today), so the revert target is the added code, not a stub. Phase 3 settlement: revert to the previous fate author and re-verify that a run without in-cell evidence fails loudly; never leave both authors live. Phase 4: revert the acceptance rebuild and confirm the checkpoint baseline is restored. The `actuator=tools` branch is deliberately not a rollback target and is not deleted here: it is held until the other desks cross (residue R8). Product restore stays a separate forward transaction on the computer''s event chain, never a fix for a failed deploy.'
  landing:
    required: true
    environment: staging https://choir.news with effects OFF
    required_receipts:
      - pushed commit SHA and CI run for each landing
      - staging deploy and health/commit identity
      - the deployed engineering-assignment acceptance command and result with accepted ids
      - the frozen roster conformance evidence artifact naming the id that served each run
      - the replay harness golden-receipt artifact
  not_done_when:
    - 'The entry gate is unsatisfied: no predecessor receipt re-read, no post-mission-two work disposition, no atomic registry promotion, no owner charter.'
    - Any retired tool name is still reachable on the RLM path, or any retirement lacks its replay proof artifact.
    - Any run-acceptance checkpoint still keys on a tool name, or a run can report an accepted level with fewer checkpoints than the recorded baseline.
    - More than one code path can author the assignment fate, or the reducer-authored terminal does not reproduce the existing identity, digest and lifecycle semantics.
    - The verifier slot is still unreachable on the assigned path, or either verifier gate lacks its pinning test.
    - Any model identifier or per-model branch appears anywhere in prompt assembly or REPL initialization.
    - Any code path strips, repairs or tolerates malformed model output to make a cell work.
    - Fewer than all four expected-pass ids complete the frozen desk task, or a roster failure was answered with a per-model accommodation.
    - The four capsule file/exec operations were deleted or broken while residue R8 still defers their deletion.
boundaries:
  mutation_class: red
  red_subclass: Phase 1 (gateway/provider routing, Node B credentials, model routing) and phase 3 (assignment fate, run acceptance, canonical events/evidence) are red surfaces; the rest of the mission is orange inside a red-class mission.
  drafting_mutation_class: green
  authority_sources:
    - owner decisions 2026-09-11 recorded in now.decision
    - docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md
    - docs/reports/choir-rlm-mission-three-review-2026-09-11.md
    - docs/reports/choir-rlm-missions-overview-2026-09-09.md
    - docs/mission-residues.md (R7 inherited, R8 deferred)
    - docs/standing-questions.md
    - skills/agentic-consensus/SKILL.md (review panel contract)
  must_preserve:
    - Historic tape decodability under frozen versioned rules; no rewrite of prior bytes.
    - 'The mission-two live vocabulary: engineering-desk writes go through the version-selected live names.'
    - The read-only synchronous exemption for observations.
    - The `actuator=tools` fallback stays functional until residue R8 deletes it.
    - Provider credentials stay host-side; the conversation identity is request metadata, never a trust input.
    - Already-accepted runs are not retroactively rescored when the acceptance checkpoints are rebuilt.
  excluded:
    - Texture runtime code, canonical-writer mutation, and any deletion of Texture JSON writers.
    - Cache optimization, promotion weights, or any evaluation that promotes.
    - Management and Research desk cutovers; the four capsule operations stay for the tools fallback.
    - Any per-model prompt fork, schema hint, retry ladder or output repair.
    - Disk/GC maintenance, vmctl ownership, Node B runtime, and every commit landed around a907f713.
    - 'Dual settlement authors: the JSON fate path and the reducer path may not both exist after P3.'
    - '`actuator=tools` as a rollback target.'
  protected_surfaces:
    - gateway and provider calls, model routing, Node B credential store
    - assignment-fate saga and its lifecycle compare-and-swap
    - run acceptance
    - canonical events and evidence projection
    - capsule execution and the capability broker
    - Texture canonical writes (design constraint only)
  completion_evidence_floor:
    - deployed_proof for phase 1 provider calls, settlement, the acceptance rebuild negative, replay per operation class, the live registry observation, the roster, and the final landing
    - local_test for the envelope simplification, the repair deletion, the prompt invariant, the parity work, the in-cell surface, the replay harness, the prompt-fix rule, the tools-actuator reachability, and the path deletion
    - static_analysis for the P0-define boundary and the citer sweep
    - broad-package local runs use scripts/go-test-runtime-shards or scripts/go-test-non-runtime-shards, never a single whole-package run (internal/store is fsync-bound and internal/agentcore and internal/textureowner are sharded in CI)
  conjecture_delta:
    discovered:
      - 'Falsified: that the nine retirements are nine interchangeable deletions whose replacements exist. `record_assignment_result` is the only assignment-fate author, two acceptance checkpoints are keyed on the retiring tool names, and the two verifier-gated tools cannot pass their gate on the live assigned path at all.'
      - 'Falsified: that the `actuator=tools` branch is dead code. It is the live fallback whenever the session worker is not ready, and it composes four of the operations the retirement inventory names.'
      - 'Falsified: that a chat-completions-only provider adapter suffices for the roster. Muse Spark serves only the Responses API and Qwen serves the Anthropic Messages shape.'
    falsifiers:
      - If any retired operation cannot reproduce its original receipt through the new path under the same semantic identity, that operation is not deletable in this mission.
      - If the expected roster cannot pass the frozen desk task on one shared prompt after three revisions, the one-prompt invariant is false as stated and the mission reports that rather than tuning around it.
      - If only one code path cannot be shown to author the assignment fate, the settlement transfer is incomplete and P3 has failed regardless of test colour.
  heresy_delta:
    discovered:
      - 'Verifier-slot unreachability: both verifier-gated tools require a metadata key that the only live CoSuper activation path never writes, so bundle inspection and verification recording are unreachable today and their acceptance checkpoint can never be produced.'
      - 'Tool-name-keyed acceptance evidence: a retiring name silently removes a checkpoint instead of failing the run, because the collector skips error results and the checkpoint is only added when a result exists.'
      - 'Empty-envelope execution: with no required parameter, a call with neither field runs an empty cell and a call with both silently selects one.'
      - 'Hidden remainder: the RLM overlay already drops the four capsule operations while the shared builder still carries all ten tools, so the desk is dual-pathed by configuration.'
      - 'Settlement authority outside the reducer: the assignment fate is written by a JSON tool while the in-cell Complete intent only mails an envelope, and the run loop''s detached-terminal predicate keys on that tool''s name.'
      - 'Live prompts instruct retired names: the RLM engineering overlay and the engineering prompt defaults still name tools this mission deletes.'
    introduced: []
    repaired: none; mark repaired only after the deployed receipts exist
  review_policy: Agentic consensus runs only at frozen boundaries (after P0, after the P3 red work, on the P4 deletion candidate, and on the P5 roster results), bound to a frozen candidate identity with base ref, scoped paths and digest. Use the bundled panel (skill://agentic-consensus). Deterministic checks come first; a panel is added only where its answer can change a real decision. A reproducible minority blocker outranks an unsupported majority pass. Panel output is an evidence receipt with one adjudicated outcome, folded into the boundary commit that changed the artifact, never a standalone consensus commit and never a substitute for the required evidence class.
measures:
  - kind: gate
    name: code-level output repairs in the carrier path
    baseline: 5
    desired: 0
    decision_use: blocks P2 completion if any survives
    cannot_prove: whether a model wraps code in fences for reasons the prompt could remove
  - kind: gate
    name: acceptance checkpoints per accepted run
    baseline: 2
    desired: '>= 2, derived from in-cell evidence rather than tool names'
    decision_use: blocks P3-acceptance; counts silently dropping is the failure mode
    cannot_prove: whether the in-cell evidence is as trustworthy as the tool-result evidence it replaces
  - kind: gate
    name: retired operations with a green replay proof
    baseline: 0
    desired: 5
    decision_use: blocks each deletion
    cannot_prove: whether the four deferred capsule operations retain their semantics under R8
  - kind: weak_signal
    name: prompt digest and size per revision, per desk
    baseline: unmeasured; measured at the P0-define freeze
    desired: identical digest across roster ids for a fixed desk; size recorded per revision
    decision_use: detects per-model forks and prompt growth toward the weakest model
    cannot_prove: that a passing roster means a good prompt rather than a tolerant task
  - kind: gate
    name: expected-pass ids completing the frozen desk task
    baseline: 0
    desired: 4
    decision_use: blocks completion
    cannot_prove: behaviour on models outside the tested roster
  - kind: telemetry
    name: consensus review yield at frozen boundaries
    baseline: two 13-agent panels on 2026-09-11 for one mission (24 must-fix findings at the first, 8 review clusters at the second; one panelist lost to a provider quota)
    desired: each boundary panel yields a small number of decision-changing findings; a panel whose findings change no decision is dropped at the next boundary
    decision_use: decides whether the next boundary keeps a panel and how wide it is
    cannot_prove: that a defect would have been missed without a panel, or that a panel's absence would have failed the mission
receipts:
  - id: engineering-carrier-owner-decisions-2026-09-11
    boundary: define
    commit_or_artifact: owner decisions recorded 2026-09-11 in conversation; folded into now.decision and the consensus record
    proof_refs:
      - 'owner: the reducer, from inside the cell, marks an assignment finished'
      - 'owner: sweep the OpenCode configuration into this mission as its first phase'
      - 'owner: get rid of the actuator=tools path; no rollback logic is planned to be used; the bar is the other desks crossing, so deletion is deferred to residue R8'
      - 'owner: the roster is experimental with deepseek-v4.1-flash, muse-spark-1.3, glm-5.3-flash and gpt-5.6-luna expected to pass; other models are worth testing; success condition is the experiment itself'
      - 'owner: when a model fails, fix the shared prompt for every model'
      - 'owner: Muse Spark runs on the free Zen id while the free quota lasts, then continues the same testing on the paid muse-spark-1.3-contributor'
      - 'owner: gpt-5.6-luna already runs on the existing ChatGPT-authenticated path'
      - 'owner: phase-1 credentials use nix/deploy-provider-creds.sh, the existing operator path, recorded with a standing-question-9 exception note'
      - 'owner: existing subscription caps only, auto-reload off; no new spend beyond the plans already paid'
      - 'owner: the roster gets model selection through an owner-visible model-policy overlay id now'
      - 'owner direction: model policy should ultimately be set at runtime inside the Go RLM code so model strings are chosen dynamically and a new model needs no code update'
    rollback_ref: none; a decision record
    disposition: recorded and settled; binding on this mission; the runtime model-policy direction is recorded as the product direction, not as a substitute for the overlay id in this mission
    problem_ref: docs/reports/choir-opencode-provider-research-2026-09-10.md
    authorization_ref: owner statement 2026-09-11
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: engineering-carrier-mission-three-consensus-2026-09-11
    boundary: define
    commit_or_artifact: docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md
    proof_refs:
      - 'thirteen-agent convergent panel at .agentic-consensus/mission3/run1 (non-durable process diagnostics): 13 ok, 0 failed'
      - orchestrator re-verification of the eval alias with empty required, the five CleanGoSource call sites, the tool-name-keyed acceptance checkpoints, the reducer-versus-fate split, both closed-set tests, and the absence of model conditionals in the prompt path
    rollback_ref: docs-only draft; revert to remove
    disposition: adjudicated; owner answers folded into now.decision
    problem_ref: the four discovered defects recorded in start.observed_artifact
    authorization_ref: owner instruction 2026-09-11 to draft mission three with agentic consensus
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: engineering-carrier-executability-review-2026-09-11
    boundary: define
    commit_or_artifact: docs/reports/choir-rlm-mission-three-review-2026-09-11.md
    proof_refs:
      - 'thirteen-agent convergent review panel at .agentic-consensus/mission3-review/run1: 12 ok, 1 quota failure (omp-gemini38, Cloud Code Assist 429), 0 task failures'
      - orchestrator source verification of the verifier-slot gates, the assigned-CoSuper metadata, the acceptance collector's error filter, the actuator route resolution, and the tools-branch registry composition
    rollback_ref: docs-only review record; revert to remove
    disposition: adjudicated into this revision; every must-fix item is folded into finish.acceptance, now, or boundaries
    problem_ref: the four discovered defects and the stale draft refs recorded in start.start_correction
    authorization_ref: owner instruction 2026-09-11 to review the draft and make it executable
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: engineering-carrier-entry-reconciliation
    boundary: define
    commit_or_artifact: 'read-only reconciliation 2026-09-11 at a846bd21: `docs/reports/choir-rlm-mission-2-versioned-rename-report-2026-09-11.md` and `docs/reports/choir-testing-ci-speedup-report-2026-09-11.md` reviewed; git status clean apart from three preserved untracked paths'
    proof_refs:
      - 'predecessor deployed proof artifact read: e3396329, CI 34571343061, staged computer epoch 909, effects OFF'
      - 'post-mission-two work disposed: GC fix a907f713 with the offline GC executed (18.6 GiB journal -> 2.1 GiB store, head seq 148655, guest rebooted clean at 12.4% used); test/CI work e22b99d4, 58d4e2dc, 278263ab with green CI run 34622877021 and race run 34626445187'
      - 'registries re-checked: ACTIVE.md lists this mission as the next queued mission; mission-graph.yaml and doc-authority-manifest.yaml still carry no node for it; zero entrypoint: true rows exist'
    rollback_ref: none; read-only reconciliation
    disposition: 'partially satisfied: predecessor receipt read and post-mission-two work disposed; registry promotion and owner charter still pending'
    problem_ref: start.start_correction stale-ref correction; the assigned-run model-selection gap added to start.observed_artifact
    authorization_ref: owner instruction 2026-09-11 to review the completion and CI reports before starting
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: engineering-carrier-charter-2026-09-11
    boundary: define
    commit_or_artifact: owner charter statement 2026-09-11; this Definition promoted in docs/ACTIVE.md, docs/mission-graph.yaml and docs/doc-authority-manifest.yaml
    proof_refs:
      - 'predecessor receipt satisfied: mission two completed at e3396329 (CI 34571343061, deployed proof artifact published) with entrypoint false and next_action none'
      - 'post-mission-two work disposed: GC a907f713 with the offline GC executed (18.6 GiB journal to 2.1 GiB store, guest rebooted clean at 12.4% used) and test/CI work e22b99d4, 58d4e2dc, 278263ab green on CI 34622877021 and race 34626445187'
      - 'owner answered the four pre-charter questions: charter now; nix/deploy-provider-creds.sh; subscription caps with auto-reload off; model-policy overlay id'
      - 'owner: agentic consensus runs as mission steps at the frozen boundaries (after P0 and later), not as a separate assurance project'
    rollback_ref: revert this commit to return the mission to queued-draft status; no runtime effect
    disposition: 'chartered: this Definition is the sole working entrypoint and P0-define is the next action'
    problem_ref: docs/reports/choir-rlm-mission-three-review-2026-09-11.md (executability review and post-review addendum)
    authorization_ref: owner charter statement 2026-09-11
    candidate_or_evidence_refs:
      - docs/reports/choir-rlm-mission-2-versioned-rename-report-2026-09-11.md
      - docs/reports/choir-testing-ci-speedup-report-2026-09-11.md
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: docs/ACTIVE.md Working Definition section, docs/mission-graph.yaml working spine node with the only entrypoint:true row, docs/doc-authority-manifest.yaml working definition entry; doccheck warnings 7 -> 2 (both remaining are outside this mission)
  - id: engineering-carrier-p0-define-2026-09-11
    boundary: define
    commit_or_artifact: docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md (sha256 3eb10539c5bb596bc33eeaf8705fa69ccff0d67dc9fae7b24384af28e183693a at base main@73815790)
    proof_refs:
      - 'four defects recorded with file:line evidence: envelope alias with empty required (tools_capsule.go:746-773), tool-name-keyed acceptance checkpoints (run_acceptance.go:627-645,281-313), reducer-is-not-a-settler (rlm_reduce.go:150-198 vs cosuper_assignment_fate.go:525-810), verifier-slot unreachable (tools_capsule.go:410-412,491-493 vs cosuper_assignment_runtime.go:342-355)'
      - 'nine-operation mapping table frozen: tools-branch closed set minus capsule_go_eval; rows 1-4 deferred to R8, rows 5-9 deleted this mission; receipt classes, canonical identity fields, exclusion lists and fixture paths declared per row'
      - 'prompt/REPL manifest and entropy-exclusion list frozen: assembly order tool_profiles.go:195-302 plus sorted catalog toolregistry.go:169-194; exclusions limited to now_utc, per-run assignment/run-context identity fields, and the user message'
      - 'falsifiers published including the vacuous-proof conditions for the mapping table and the exclusion list'
      - 'replay-harness specification published: fixture schema v1, canonicalizer, effect census, durable receipt store docs/evidence/rlm-replay/, legacy and successor adapters, caller-supplied semantic identity, state-mutating read fixture, rewarm procedure'
      - 'model-selection mechanism chosen: owner-visible model-policy overlay id threaded through assign_co_super -> StartAssignedCoSuperRequest -> request digest -> run metadata before EnrichMetadata'
      - 'frozen desk task published at artifact section 7'
    rollback_ref: revert this commit; docs-only, no runtime effect
    disposition: 'published and frozen; P0-review panel is bound to this artifact identity'
    problem_ref: the four defects in start.observed_artifact, recorded in artifact section 1
    authorization_ref: owner charter 2026-09-11; P0-define acceptance item
    candidate_or_evidence_refs:
      - docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: docs/doc-authority-manifest.yaml frozen_boundary_artifact entry added in the same commit
  - id: engineering-carrier-p0-review-2026-09-11
    boundary: define
    commit_or_artifact: 'adjudicated P0-review of the frozen artifact; findings folded into artifact revision 2 (commits 98447096, e69de37a); panel diagnostics at .agentic-consensus/p0-review/ (non-durable)'
    proof_refs:
      - 'two convergent panel runs bound to the frozen artifact (base main@73815790, sha256 3eb10539c5bb596bc33eeaf8705fa69ccff0d67dc9fae7b24384af28e183693a): run1 4 ok of 5, run2 7 ok of 8; adjudicated outcome repair'
      - 'confirmed findings folded: receipt-class taxonomy (host-authored content-addressed, not signed), row-1 reclassification to transient_mutation, store-record canonical fields for rows 5-8, capture-entropy-vs-replay-identity split, per-operation conflict semantics, row-8 successor identity from assignment/attempt+proposition digest, Definitions() schema digest, skill-path entropy exclusion, filesystem-write witness, class-specific successor adapter contract, hard deletion prerequisite, desk-task execution_refs and go -C fix'
      - 'orchestrator source re-verification of panel claims before folding'
    rollback_ref: revert commits 98447096 and e69de37a; docs-only, no runtime effect
    disposition: 'adjudicated repair; artifact revision 2 is the frozen boundary for all later phases'
    problem_ref: the four defects in start.observed_artifact, recorded in artifact section 1
    authorization_ref: owner charter 2026-09-11; P0-review acceptance item
    candidate_or_evidence_refs:
      - docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
view:
  path: none
  generator: node skills/definition/scripts/dashboard.mjs docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md --serve 127.0.0.1:8787 --watch (skill-owned; served on demand, never committed)
---
# Mission 3 - Engineering moves onto the in-cell carrier

Chartered 2026-09-11 as the sole working entrypoint. The first slice is P0-define, a code-free freeze
artifact; the launch command is `/goal docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md`.

## What this mission does

Engineering becomes the first desk to run entirely on the in-cell carrier. Today the desk is split:
one JSON tool is the new way, five overlay JSON tools are the old way, and four capsule operations
are hidden from the desk but alive on the fallback route.

- `capsule_go_eval` becomes the desk's only JSON tool; the model hands it Go source.
- Everything else is a typed Go function inside the cell: files, commands, messages, spawning,
  finishing, verification, inspection.
- The five overlay JSON names are deleted, each earned by a replay proof.
- The reducer writes the assignment fate; the JSON fate tool is deleted.
- Run acceptance stops reading tool names and fails loudly when evidence is missing.
- One prompt serves every model: no per-model text, no per-model branching, no output repair.

## Four defects, written down before any fix

1. The envelope is not simple: `source` and `code` are interchangeable, nothing is required, a call
   with neither runs an empty cell, and a call with both silently picks one.
2. Acceptance reads tool names: two checkpoints are built by scanning for results named
   `commit_transaction` and `record_self_development_verification`, so deleting the tools removes
   evidence silently instead of failing the run.
3. The reducer cannot finish an assignment: a staged Complete is only a mailbox message while
   `record_assignment_result` writes the fate, and the run loop's terminal predicate keys on that
   tool's name.
4. The verifier slot is unreachable: both verifier-gated tools require a metadata key the live
   assigned path never writes, so bundle inspection and verification recording cannot be reached.

## Ordering

P0 code-free freeze, then an agentic-consensus review of that frozen artifact, then P1 provider
setup (independent of P2-P4 and may run in parallel), then the carrier repairs, the settlement and
acceptance work, the replay harness and deletions, the roster experiment, and finally the landing
with an intermediate deploy before the deployed replay and roster proofs. Consensus review gates sit
at the frozen boundaries: after P0, after the red P3 work, on the P4 deletion candidate, and on the
P5 roster results. Each is bound to a frozen candidate, locally verified, adjudicated to one
outcome, and folded into the boundary commit. No repair, settlement or deletion code precedes P0.
