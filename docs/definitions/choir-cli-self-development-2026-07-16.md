---
title: "Make Choir Self-Developing — CLI-Controlled Computer Changes"
definition_version: 2

start:
  captured_at: 2026-07-18T06:04:32Z
  source:
    canonical_ref: refs/heads/main@a36ebb08b024d74c06c3124c49c46e5acc4d2b63
    origin_ref: refs/remotes/origin/main@a36ebb08b024d74c06c3124c49c46e5acc4d2b63
    relation: canonical_ref_equals_origin_ref
    deploy_identity: "choir.news /health reported 9d9945e65f5b54069e1a86a530cb0960d96b3474 at authoring; source/deploy mismatch must be reconciled before deployed acceptance"
  worktree_inventory:
    path: /Users/wiz/go-choir
    status: clean
    branch: main
    preservation_rule: "Preserve unrelated worktrees, owner computers, accepted ComputerVersions, rollback realizations, and production recovery images. Build all candidates from immutable inputs; never import an unreviewed worker branch or host-local VM state."
  observed_artifact:
    - claim: "Audited construction is complete: every served staging computer routes through a vmctl-owned ComputerVersion CAS and can be constructed, verified, rolled back, reconstructed, and inspected without SSH."
      evidence_ref: docs/evidence/audited-construction-terminal-receipt-2026-07-17.md
    - claim: "The supported choir CLI currently exposes run, computer lifecycle/status, and API-key operations, but no source-change proposal, candidate build/verification, approval, promotion, rollback, or change-receipt workflow."
      evidence_ref: cmd/choir/main.go
    - claim: "AppChangePackage/AppAdoption records and ComputerSourceLineage can describe source change, but the existing promotion service can advance lineage when its adapter is unavailable; they do not own D-ROUTE and cannot prove a served ComputerVersion changed."
      evidence_ref: internal/promotion/service.go; docs/platform-os-app-state.md
    - claim: "vmctl already owns immutable construction, independent realization verification, frozen promotion/rollback plans, signed authorization evidence, and the sole D-ROUTE compare-and-swap writer."
      evidence_ref: internal/vmctl/construction_handler.go; internal/vmctl/promotion_candidate.go; internal/vmctl/promotion_execution.go; internal/vmctl/promotion_authority.go
  problem:
    classification: substrate
    statement: "Choir can construct and route immutable computers, but no supported product path turns an external agent's source change into a new CodeClosure, an unpublished candidate computer, independently verified promotion evidence, and a served route transition. Development therefore still escapes the product through GitHub, local branches, SSH, or paper-only package/lineage state."
    existing_fix_connection: "Connect the audited vmctl constructor/verifier/frozen-route path to one durable product-owned ComputerChange lifecycle and a narrow choir CLI/API. Do not build another constructor, route writer, package activation tower, or third store."
  unknowns:
    - "The exact production build worker boundary that can import a content-addressed SourceChangeBundle and emit the sandbox runtime artifact without trusting a mutable branch."
    - "The smallest existing runtime-store object-graph pattern that can persist ComputerChange transitions, operation handles, failures, and receipt refs without becoming route authority."
    - "Measured staging distribution for source-bundle validation, runtime build, construction, verification, frozen promotion, and rollback; initial bounds below are acceptance ceilings, not claimed baselines."
  corrections:
    - corrected_at: 2026-07-18T06:04:32Z
      preserves_original_observation: true
      clarification: "The Definition filename and owner decision retain the 2026-07-16 mission date. The executable start receipt was captured on 2026-07-18 after audited construction closed on 2026-07-17; the earlier authoring timestamp was not promoted as execution authority."
      evidence_ref: "Canonical repository history and machine UTC at frozen-review repair."
  owner_ratification:
    status: settled
    settled_by: owner
    recorded_at: 2026-07-16
    statement: "The next realism axis is the self-developing computer, first operated through the supported choir CLI by an external agent with a scoped key and no SSH. Proceed agentically from draft to final and then run it autonomously."
    evidence_ref: "Owner statement in this 2026-07-16 conversation."

finish:
  deliver: "Make one Choir computer safely develop Choir itself through the supported choir CLI. An external agent submits a bounded, content-addressed source change against the exact active ComputerVersion, observes an asynchronous durable candidate lifecycle, receives independent verification, promotes through the existing vmctl-only frozen D-ROUTE CAS, observes the changed served computer, and rolls back or re-promotes—without SSH, host files, mutable remote branches, direct database access, or raw vmctl authority."
  artifact: "A production ComputerChange product path with one durable lifecycle authority in the existing runtime Dolt object graph; a reproducible SourceChangeBundle-to-CodeClosure build boundary; owner-scoped public APIs and choir CLI commands for propose/status/approve/promote/rollback; joined receipts binding source input, built CodeClosure, candidate realization, verifier decision, delegated approval, frozen route plans, route CAS, served ComputerVersion, rollback, and restart durability."
  acceptance:
    - action: "Using a mutation-scoped Choir key from an external client, target an explicitly disposable non-production staging computer with a proven prior-version rollback route and submit a content-addressed SourceChangeBundle containing a harmless verifier-known served marker against the exact active ComputerVersion and route generation. Receive a durable change ID within 10 seconds; let build, construction, and verification continue asynchronously."
      proves: "The product accepts a bounded development intent without SSH, a mutable branch, a long-held HTTP request, or an in-memory-only operation."
      evidence_class: deployed_external_change_intake
    - action: "Recompute the bundle digest, owner/computer binding, active CodeRef-to-CodeClosure.SourceCommit base, single candidate commit/ref, full path/mode/delete/rename manifest, candidate source-tree digest, runtime artifact digest, CodeClosure, and reused ArtifactProgramRef. Enforce the named input ceilings and reject path traversal, secret/private material, extra refs/tags, non-descendant history, gitlinks/submodules, LFS/filter dependencies, changed symlinks or special modes, undeclared changes, stale active identity, and any non-code state delta. Import and build quarantined bytes without network or inherited secrets; pin only verified outputs through the existing corpusd ComputerVersion input catalog."
      proves: "The new computer is derived reproducibly from the agent-supplied change and the active immutable base rather than a fixture, GitHub branch, worker checkout, or hidden state."
      evidence_class: independent_source_build_join
    - action: "Construct an unpublished candidate from the resulting ComputerVersion through the production materializer, independently verify it, and freeze promotion/rollback plans against the observed current route slot, generation, and ComputerVersion. Restart the product orchestrator before approval and recover the same status and receipt refs."
      proves: "ComputerChange is durable and joins to the audited constructor/verifier instead of reimplementing them or losing state on restart."
      evidence_class: durable_candidate_lifecycle
    - action: "Attempt status and all mutation verbs with no key, a revoked key, a read-only key, a different-owner key, and a write key lacking approval authority. Require scoped reads and deterministic 401/403/refusal results; only the explicitly delegated approval/promotion key may authorize the frozen transition."
      proves: "An external agent has least-privilege product authority, not ambient host access, cross-owner access, raw signing keys, or direct vmctl control."
      evidence_class: deployed_scoped_authority_refusal
    - action: "Approve and promote the frozen candidate through the public product API. Verify the server mints and pins delegated authorization evidence only after independent verification, vmctl performs the sole D-ROUTE CAS, and the fetched joined identity changes from the exact base ComputerVersion to the candidate at generation + 1."
      proves: "Approval and package metadata cannot impersonate route activation; the served computer changes through the audited authority boundary."
      evidence_class: deployed_computer_change_promotion
    - action: "Fetch the harmless marker and joined immutable identity through supported authenticated product paths; stop/start the computer through choir CLI and verify the candidate ComputerVersion and marker survive. Then roll back through the frozen vmctl plan, prove the prior identity and bytes return, and re-promote or roll forward to the candidate once more."
      proves: "The changed computer is genuinely served, restart-durable, reversibly promoted, and externally observable—not a package row, lineage projection, fixture, or local process."
      evidence_class: deployed_served_change_rollback_restart
    - action: "Advance the active route, ArtifactProgramRef, resolved artifact-program digest/app-state head, or submit a competing change after freeze, then attempt the stale promotion. Require refusal without modifying D-ROUTE; rebase/rebuild/reverify against the new active ComputerVersion before any later promotion."
      proves: "Foreground code or state changes are not lost: promotion proves no tail, or refuses and rebuilds/reverifies with a replayed/merged new ArtifactProgramRef; stale agents cannot overwrite a newer computer."
      evidence_class: deployed_stale_base_refusal
    - action: "Attempt the legacy AppAdoption/ComputerSourceLineage promotion route with the same candidate evidence and verify that it cannot claim or produce a served ComputerVersion transition for this flow. Inventory D-ROUTE writers and independently prove vmctl remains the sole writer."
      proves: "The existing paper/source-lineage path is bypassed or hard-refused rather than widened into a competing activation authority."
      evidence_class: route_authority_negative_acceptance
    - action: "For the pushed origin/main SHA, verify CI, Node B deployment identity, public API/CLI operation, candidate construction, served marker, route generation, restart, rollback, re-promotion, and no-SSH refusal evidence on choir.news."
      proves: "The mission changes the deployed product rather than only local code, contracts, or documentation."
      evidence_class: complete_deployed_self_development_acceptance
  rollback: "Before route CAS, mark the ComputerChange refused or failed and dispose the unpublished candidate while retaining immutable input and failure receipts. After CAS, invoke only the frozen vmctl rollback plan, observe the exact prior ComputerVersion and served marker state, and retain both accepted versions for the configured TTL. On stale base, verifier failure, auth ambiguity, deploy mismatch, or joined-identity mismatch, refuse promotion and stop; never repair route truth through AppAdoption, lineage, SQL, SSH, or a mutable branch."
  landing:
    required: true
    environment: staging_node_b_and_choir_news
    required_receipts: [pushed_origin_main_commit, ci, deploy, staging_build_identity, external_change_intake, source_build_join, construction, independent_verification, scoped_authority_refusal, delegated_approval, frozen_promotion, vmctl_route_cas, served_marker, restart_durability, rollback, repromotion, stale_base_refusal, no_ssh_acceptance]
    registry_hygiene:
      required: true
      must_update: [docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      acceptance: "This Definition is the sole executable /goal while working. The completed audited-construction Definition remains historical evidence; the performance draft remains blocked and non-executable. Terminal closure removes this Definition from executable authority without automatically activating a successor."
  not_done_when:
    - "A CLI command only creates AppChangePackage, AppAdoption, ComputerSourceLineage, source-ledger, Git, or candidate-package records without changing the fetched served ComputerVersion."
    - "A worker branch, origin/main, an uploaded source archive without a pinned base relation, or a pre-baked CodeRef substitutes for the submitted SourceChangeBundle."
    - "The candidate lifecycle exists only in memory, logs, a polling goroutine, or process-local files."
    - "An API key can call /internal/vmctl/*, mint arbitrary route evidence, cross owners, or promote without independent verification and a frozen route plan."
    - "Foreground route or ComputerVersion drift can be overwritten rather than refused and rebased."
    - "Promotion or rollback uses the non-conformant Dolt tag adapter, AppAdoption graceful-degradation path, direct SQL, SSH, or a second route writer."
    - "Only unit tests, package rows, source lineage, CI, a constructed fixture, dashboard output, or local macOS evidence passes."
    - "The marker is not fetched from the actually served computer before and after restart, rollback, and re-promotion."

scope:
  mutation_class: red
  protected_surfaces: [ComputerVersion, CodeClosure, ArtifactProgramRef, candidate_computers, runtime_Dolt_object_graph, routeledger_evidence, vmctl, D_ROUTE, promotion, rollback, API_keys, credentials, product_API, deployment_routing, run_acceptance]
  allowed:
    - "Add the minimal ComputerChange record, runtime-store methods, state-transition service, authenticated public handlers, choir CLI verbs, and focused verifier contracts required by the finish."
    - "Add a bounded isolated SourceChangeBundle importer and reproducible runtime build that emits content-addressed CodeClosure inputs and receipt joins."
    - "Connect existing production construction, independent verification, signed authorization, frozen promotion/rollback, and vmctl route-client APIs."
    - "Delete or hard-refuse superseded computer-level activation paths where leaving them reachable would create split authority."
    - "Update focused tests, deployment wiring, docs, registries, and durable evidence needed for deployed acceptance."
  forbidden:
    - "No third store, second D-ROUTE writer, direct SQL route mutation, raw vmctl access for external keys, SSH operator step, host-file authority, or mutable branch as build input."
    - "No extension of candidate_package_activation_contract.go as a substitute for an executor."
    - "No performance/disk optimization, full capsule runtime, Features UI activation, marketplace/public package distribution, fleet-wide source rollout, provider routing, unrelated auth redesign, or broad source-system rewrite."
    - "No production route CAS before a frozen candidate has deterministic evidence and G2 accepts it."
    - "No completion claim before deployed marker/identity/restart/rollback/re-promotion evidence and G3 acceptance."
  rollback_path: "Use the accepted prior ComputerVersion and vmctl-owned frozen rollback plan. Feature/API exposure can be disabled and the new handler routes removed only after any active candidate is rolled back and lifecycle receipts are preserved."
  conjecture_delta:
    - discovered: "Audited computer construction exists, but the SourceChangeBundle-to-CodeClosure build and durable product lifecycle wire are absent."
    - introduced: "A narrow code-only change can safely reuse the exact active ArtifactProgramRef while changing CodeRef if route generation and full ComputerVersion base are frozen and stale changes are refused."
    - repaired_target: "Replace off-product development and paper lineage activation with an externally controlled, content-addressed, audited ComputerVersion change loop."
  heresy_delta:
    discovered: ["AppAdoption promotion may advance source lineage while its adapter is unavailable", "candidate activation contracts are not connected to served D-ROUTE", "current development requires off-product Git/host workflows"]
    introduced: []
    repaired: []
  input_policy:
    target: "One explicitly disposable non-production staging computer with an accepted prior ComputerVersion, healthy rollback realization, and bounded rollback TTL; no owner-primary or fleet route is eligible in this slice."
    identity: "Bundle owner_id and computer_id must match the authenticated route; base ComputerVersion and route generation must equal the active route; base commit must equal CodeClosure.SourceCommit resolved from the active CodeRef."
    git_shape: "Exactly one advertised refs/heads/candidate ref; no tags or extra refs; candidate is exactly one direct-child commit of base. Full manifest records every added, modified, deleted, and renamed path plus old/new modes and blob digests."
    ceilings: "Compressed bundle <=64 MiB; <=100000 Git objects; <=25000 files in candidate tree; <=500 changed paths; candidate tree expanded regular-file bytes <=1 GiB; individual blob <=32 MiB; validation/build timeout <=30 minutes."
    filesystem_refusals: "Reject absolute/dotdot/NUL paths, case-fold or Unicode-normalization collisions, gitlinks/submodules, LFS pointers or required clean/smudge filters, changed symlinks, special files/modes, undeclared changes, and private/secret material. Changed file modes are only 100644 or 100755."
    isolation: "Authenticate and quota before upload; at most one nonterminal change per computer, four proposals per key per hour, and two builds per computer per hour. Quarantine bundle bytes before validation. Import/build without network, inherited credentials, writable host source, or mutable remote refs; use bounded CPU, memory, disk, process count, and temporary lifetime."
    artifact_program: "Code-only reuse requires allowlisted code/build paths, exact unchanged ArtifactProgramRef, independently equal resolved ArtifactProgram digest/app-state head at propose/freeze/CAS, cold boot from the pin, and the audited pin-before-ack state rule. If foreground state advances, replay/merge into a newly pinned ArtifactProgramRef then rebuild/reverify, or refuse."
    authority: "Only verified CodeClosure/runtime outputs enter the existing corpusd ComputerVersion input catalog. Quarantine, worker checkout, bundle storage, and build logs are not durable computer or route authority."
  non_purpose:
    - "Not a general autonomous software-development agent, planner, or capsule executor. The external agent already supplies the source change."
    - "Not ComputerVersion performance optimization or disk/storage policy work."
    - "Not app marketplace publication, AppAdoption activation, Features UI work, or public multi-owner distribution."
    - "Not fleet-wide deployment of arbitrary agent changes; the acceptance target is one explicitly disposable non-production staging computer with a proven rollback route and one harmless code-only change."

execution:
  - id: A-authority-and-input
    purpose: "Make one durable ComputerChange authority and one reproducible source/build boundary real before exposing mutation."
    entry: "Canonical main is clean and equals origin/main; registries promote this owner-ratified Definition; staging deploy mismatch is recorded rather than mistaken for acceptance."
    work:
      - "Document the source-build and lifecycle gaps before repair code; inventory every existing package, lineage, candidate, build, verifier, signer, and D-ROUTE writer path."
      - "Define ComputerChange identity, owner/computer scope, allowed transitions, immutable request, operation handle, receipt refs, refusal reasons, idempotency, and restart reconstruction in the existing runtime Dolt object graph."
      - "Define SourceChangeBundle as a content-addressed Git bundle bound to owner/computer with exact active CodeRef-resolved CodeClosure.SourceCommit, one advertised candidate ref containing one direct-child commit, bundle SHA-256, full changed path/mode/delete/rename manifest, and requested base ComputerVersion/route generation. Enforce the input policy, quarantine bytes, validate ancestry and tree digest, and pin verified outputs only through the existing corpusd ComputerVersion input catalog."
      - "Build the candidate runtime in an isolated bounded worker from immutable bundle bytes; pin resulting source tree/runtime artifacts as a new CodeClosure. Reuse ArtifactProgramRef only under the explicit code-only policy: changed paths are allowlisted code/build paths; no program/state/tape delta exists; the resolved ArtifactProgram digest and app-state head equal the frozen base at propose, freeze, and CAS; the candidate cold-boots from that pin without live process/data.img continuity. Any tail requires replay/merge into a new ArtifactProgramRef, rebuild, and reverify, or refusal."
    exit: "Focused persistence, restart, adversarial bundle, deterministic build, and typed CodeClosure/ArtifactProgramRef join checks pass; G1 accepts the executable substrate candidate."
  - id: B-candidate-and-product-path
    purpose: "Connect the durable lifecycle to audited candidate construction and least-privilege external operation."
    entry: "A accepted; no public mutation endpoint exists before the source/build authority is deterministic."
    work:
      - "Implement async propose/status so request acknowledgement is at most 10 seconds, one operation survives restart, duplicate idempotency keys return the same change, and terminal failures are durable."
      - "Invoke only the production ComputerVersion materializer for an unpublished candidate; independently verify and persist receipt refs; never call AppAdoption promotion."
      - "Add scoped public API and choir CLI commands: change propose; candidate status; candidate approve; candidate promote; candidate rollback. Status carries receipts, so no separate mutation-like receipts authority is created."
      - "Separate read, change-write, and delegated approve/promote scopes. Product auth authorizes a server-side request; only the existing signing boundary mints/pins routeledger evidence."
      - "Freeze the current route slot, generation, base ComputerVersion, resolved ArtifactProgram digest/app-state head, candidate ComputerVersion, verification receipt, promotion certificate, promote plan, and rollback plan. Allow one nonterminal change per computer; competing proposals refuse until disposition."
    exit: "Focused API/CLI/auth/restart tests and a disposable staging rehearsal construct and verify one unpublished candidate; no route CAS has occurred."
  - id: C-frozen-promotion
    purpose: "Prove the exact candidate can safely become the served computer and return."
    entry: "B accepted; G2 reviews a frozen pre-route candidate and may still prevent all route mutation."
    work:
      - "Run independent bundle/build/ComputerVersion/realization/authorization joins and inventory route writers."
      - "Exercise missing/revoked/read-only/cross-owner/insufficient-scope refusal and stale route/base refusal."
      - "Promote through the vmctl-only frozen D-ROUTE CAS; fetch joined identity and marker; restart through product lifecycle; rollback; fetch prior state; re-promote or roll forward and fetch candidate state."
      - "Prove AppAdoption/lineage paths cannot claim this transition and preserve a bounded prior-version rollback TTL."
    exit: "Deployed evidence records exact source, build, candidate, route generations, product bytes, restart, rollback, and re-promotion; all no-SSH."
  - id: D-land-and-close
    purpose: "Land one coherent product path and close only on deployed reality."
    entry: "C works end to end on staging from a disposable external client."
    work:
      - "Commit and push origin/main; monitor CI and Node B deployment; verify choir.news build identity equals the pushed SHA."
      - "Rerun the complete deployed acceptance from a clean external client with no SSH and preserve typed receipts and command outputs."
      - "Adjudicate G3 on a frozen closure packet; repair reproducible blockers before terminal registry changes."
      - "Update terminal receipt and registries, classify all dirty paths/candidates, and leave no untracked proof output or live unauthorized candidate."
    exit: "G3 accepts, registries are coherent, the Definition is complete/historical, and rollback/evidence refs remain durable."

orchestration:
  implementation_order: "A then B then C then D. Parallel read-only investigation and disjoint implementation are allowed inside a phase; ComputerChange state transitions, route CAS, landing, Definition state, and terminal registry authority remain serialized."
  state_authority:
    lifecycle: "One ComputerChange object in the existing runtime Dolt object graph owns request, state, operation handles, idempotency, errors, and receipt refs."
    immutable_inputs: "Existing ComputerVersion input catalog owns CodeClosure and ArtifactProgramRef pins."
    candidate_realization: "vmctl owns candidate construction and realization lifecycle."
    verification_and_authorization: "Existing independent verifier and routeledger evidence own their typed receipts."
    served_route: "Existing corpusd D-ROUTE tables with vmctl as sole CAS writer own served ComputerVersion truth."
    projections: "CLI/API responses, AppChangePackage, AppAdoption, source lineage, dashboards, logs, and evidence packets do not own activation."
  transforms:
    - name: state_machine
      consequence: "Define legal transitions and impossible/stuck states before handlers; every async status is reconstructible from one durable record."
    - name: single_writer
      consequence: "ComputerChange owns lifecycle, vmctl owns D-ROUTE, and neither can silently impersonate the other."
    - name: inversion
      consequence: "Acceptance actively attempts the green-looking failures: package-only promotion, stale-base overwrite, fixture CodeRef, leaked signer, in-memory success, and wrong deployed SHA."
    - name: artifact_gradient
      consequence: "Optimize for a fetched changed computer plus reversible route evidence, not command count, contract volume, test count, or package rows."
    - name: weakest_link
      consequence: "The mission is incomplete if any join from supplied bundle to served bytes is missing, even when every other subsystem passes."
  budgets:
    request_ack: "<=10 seconds for propose/approve/promote/rollback acknowledgement or terminal validation refusal; long work returns a durable operation handle"
    build_construct_verify: "<=30 minutes end to end for the harmless code-only staging candidate; record phase timings and tighten only from measured evidence"
    frozen_promote: "<=120 seconds from accepted promote request to observable joined route, else fail/refuse without split route truth"
    frozen_rollback: "<=120 seconds from accepted rollback request to observable prior joined route, else stop further mutation and escalate"
    stale_refusal: "before any D-ROUTE write"
  decision_gates:
    - id: G1-executable-source-and-lifecycle
      review_kind: agentic-consensus
      changes_decision: "Whether ComputerChange persistence and SourceChangeBundle-to-CodeClosure implementation are real, deterministic, bounded, and safe enough to expose candidate construction."
      after: A-authority-and-input
      before: B-candidate-and-product-path
      frozen_input_required: [base_ref, candidate_ref, path_scope, content_digest, deterministic_test_evidence, source_build_receipts, state_machine]
      deterministic_first: "Focused state transition/restart/idempotency tests; adversarial bundle/path/secret/ancestry tests; repeated source-tree/runtime-artifact/CodeClosure digest reproduction; exact ArtifactProgramRef reuse refusal checks."
      builder_obligation: "Present the executable importer, builder, persistent lifecycle, typed receipts, failures, measured resource bounds, and deletion/bypass map."
      falsifier_obligation: "Seek mutable refs, worker checkout authority, secret ingress, non-reproducible archives, unbounded builds, hidden state, split lifecycle authorities, process-local recovery, implicit ArtifactProgramRef reuse, and fixture-only CodeRefs."
      verifier_obligation: "Independently rebuild from frozen bytes and recompute every digest and transition without using builder outputs as authority."
      adjudication: [accept, repair, reject, escalate]
      minority_rule: "One reproducible blocker overrides an unsupported majority pass."
      durable_evidence_ref: "required:G1 frozen packet, selection/independence telemetry, verdicts, and adjudication"
    - id: G2-frozen-pre-route-candidate
      review_kind: agentic-consensus
      changes_decision: "Whether the public API/CLI, scoped delegation, constructed candidate, independent verification, and frozen promote/rollback plans authorize the first D-ROUTE CAS."
      after: B-candidate-and-product-path
      before: C-frozen-promotion
      frozen_input_required: [base_ref, candidate_ref, path_scope, content_digest, deployed_candidate_receipts, auth_refusals, route_writer_inventory, frozen_plans]
      deterministic_first: "Product API/CLI contract checks; missing/revoked/read-only/cross-owner scope refusals; restart recovery; exact input/build/realization joins; route-writer inventory; stale generation refusal; rollback rehearsal without route mutation."
      builder_obligation: "Present the exact unpublished candidate, joined receipts, delegated authorization boundary, promotion certificate, frozen plans, current route identity, and bounded rollback TTL without executing CAS."
      falsifier_obligation: "Seek direct vmctl exposure, signer leakage, cross-owner access, package/lineage activation, stale base, split brain, unverified candidate, mutable inputs, missing rollback, and any pre-review route mutation."
      verifier_obligation: "Independently recompute bundle-to-candidate joins, challenge scopes, verify signatures/certificates/plans, and prove vmctl is the only possible route writer."
      adjudication: [accept, repair, reject, escalate]
      minority_rule: "One reproducible blocker overrides an unsupported majority pass."
      durable_evidence_ref: "required:G2 frozen packet, selection/independence telemetry, verdicts, and adjudication"
    - id: G3-deployed-self-development-closure
      review_kind: agentic-consensus
      changes_decision: "Whether the landed product path and deployed promote/restart/rollback/re-promotion evidence satisfy self-development and authorize terminal closure."
      after: D-land-and-close
      before: [registry_terminal_closure, status_complete]
      frozen_input_required: [source_commit, deployed_build_identity, candidate_ref, content_digest, complete_acceptance_refs, rollback_ref, registry_diff]
      deterministic_first: "Recompute source bundle/build/ComputerVersion/realization/route joins; rerun no-key and scoped-key refusals; fetch marker and identity before/after restart/rollback/re-promotion; stale-base refusal; CI/deploy identity; doccheck and registry checks."
      builder_obligation: "Present a frozen closure packet with exact external commands, typed receipt refs, deployed identities, timings, route generations, served bytes, candidate disposition, rollback, residual risk, and proposed terminal registry diff."
      falsifier_obligation: "Seek fixture substitution, package-only success, wrong SHA, unsupported endpoint, SSH/manual host dependency, stale overwrite, lost state after restart, rollback mismatch, unauthorized mutation, missing receipts, dirty worktree, or registry competition."
      verifier_obligation: "Independently replay the supported external product path and recompute all joins without trusting dashboard prose or builder summaries."
      adjudication: [accept, repair, reject, escalate]
      minority_rule: "One reproducible blocker overrides an unsupported majority pass."
      durable_evidence_ref: "required:G3 frozen packet, selection/independence telemetry, verdicts, and adjudication"
  prohibitions:
    - "No model vote, dashboard, local test, package row, lineage projection, or agent assertion authorizes route mutation or completion."
    - "Parallel workers cannot publish routes, promote, roll back, land, mutate this Definition's now state, or close registries."
    - "No repair-code commit may precede the durable problem record required by problem-documentation-first."

now:
  status: working
  slice: "A-authority-and-input"
  question: "Which existing build boundary can import the frozen SourceChangeBundle and emit the production runtime artifact with the least new code while preserving content-addressed authority?"
  reconciliation:
    observed_at: 2026-07-18T06:20:00Z
    source_ref: refs/heads/main@f06b0941_before_goal_start_reconciliation_equals_refs/remotes/origin/main@f06b0941
    deploy_identity: "choir.news /health reports 9d9945e65f5b54069e1a86a530cb0960d96b3474; acceptance blocked on a later pushed source and matching deployment"
    authority_identities:
      - owner_decision:2026-07-16-self-developing-computer-cli-first
      - predecessor:docs/definitions/choir-audited-autoputer-construction-2026-07-15.md@complete
      - mission_graph:docs/mission-graph.yaml@working_sole_entrypoint
      - authority_manifest:docs/doc-authority-manifest.yaml@active_authority_and_entry_root
    policy_resolution_ref: "Owner-selected CLI-first external-agent ComputerVersion development; performance optimization remains separate and blocked."
    worktree_inventory_ref: "Canonical main clean and equal origin/main at f06b0941 before this compact goal-start reconciliation; no unrelated work is present."
    status: reconciled_for_execution_with_known_deploy_lag
  candidate:
    id: cli-self-development-definition-2026-07-16
    state: registry_promoted_and_executable
    ref: docs/definitions/choir-cli-self-development-2026-07-16.md
    owner: integration-authority
    base: refs/heads/main@f06b0941
    scope: [definition, registries, problem_record]
  decision:
    selected: "Use a narrow code-only SourceChangeBundle against the exact active ComputerVersion; persist one ComputerChange lifecycle in the existing runtime store; build and pin a new CodeClosure; reuse the exact ArtifactProgramRef only when state is unchanged; construct and verify an unpublished candidate; let a least-privilege delegated key invoke server-side approval and the existing vmctl-only frozen route CAS; prove served bytes, restart, rollback, and re-promotion."
    kind: architectural
    status: settled
    source: owner
    settled_by: owner
    evidence_ref: "Owner selected the self-developing computer and CLI-first external-agent interface in this 2026-07-16 conversation; architecture refines that outcome under settled two-store, vmctl-only D-ROUTE, no-SSH, and audited-construction constraints."
    recorded_at: 2026-07-16T05:00:00Z
    consequence: "AppAdoption/lineage remains evidence, not activation. The performance draft stays blocked. This mission builds one honest end-to-end computer change rather than a general capsule or package system."
  evidence_refs:
    - docs/evidence/audited-construction-terminal-receipt-2026-07-17.md
    - docs/platform-os-app-state.md
    - internal/computerversion/input_resolver.go
    - internal/computerversion/production_materializer.go
    - internal/computerversion/realization_verifier.go
    - internal/vmctl/promotion_candidate.go
    - internal/vmctl/promotion_execution.go
    - internal/vmctl/promotion_authority.go
    - internal/vmctl/route_client.go
  blocker_or_risk: "No execution-authority blocker. The SourceChangeBundle build executor remains the highest-risk unknown and G1 must reject any mutable-ref, non-hermetic, unbounded, or third-store implementation. Staging deploy identity remains intentionally behind source until a behavior-changing landing reaches Node B."
  next_action: "Execute A-authority-and-input: document the concrete source-build/lifecycle gap, map existing build and store patterns, implement the smallest deterministic SourceChangeBundle importer and durable ComputerChange authority, then freeze G1 evidence before public mutation."

successor:
  status: none_selected
  candidate_goal: none
  note: "Performance optimization remains a separate blocked draft. Capsule execution, UI activation, multi-owner distribution, and broader autonomous development require later owner-ratified Definitions."

view:
  path: none
  endpoint: "http://127.0.0.1:8788"
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-cli-self-development-2026-07-16.md --serve 127.0.0.1:8788 --watch"
  generator_version: "definition-dashboard-js/v1"
  authority: "The dashboard is a read-only projection. This Markdown/YAML Definition and its coherent registries are the sole mission authority."
---

# Make Choir Self-Developing — CLI-Controlled Computer Changes

Completion is one real reversible development loop: an external agent submits a content-addressed Choir source change through the supported CLI, Choir constructs and verifies a new immutable computer, the delegated product authority promotes it through vmctl's sole route CAS, and supported product reads prove the changed computer survives restart, rolls back, and can be served again. Package rows, source lineage, local builds, fixtures, and SSH do not count.
