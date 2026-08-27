---
definition_version: 2

start:
  captured_at: 2026-08-27T00:10:00Z
  source:
    canonical_ref: main@47db6bce26e85bc17e8264d12f689fbf09121703
    deploy_identity: staging proxy 05cc87b6d228eb17451e721ec4b3cbcf3774139a observed via https://choir.news/health at 2026-08-27T00:15Z
  worktree_inventory:
    status: reconciled
    evidence_ref: "docs/problems/genesis-computer-surface-underivable-spa-2026-08-26.md, docs/definitions/choir-computer-recovery-and-genesis-surface-2026-08-26.md"
    preservation_rule: preserve unrelated work and classify every new dirty path before implementation
  worktrees:
    - path: .
      status: candidate_wip
      class: goal_candidate
      owner: owner + mission lead
      touch: goal_owned
      paths_or_digest: "docs/problems/genesis-computer-surface-underivable-spa-2026-08-26.md, docs/definitions/choir-computer-recovery-and-genesis-surface-2026-08-26.md"
      recovery: revert candidate commits; no live database mutations in opening boundary
  candidates:
    - id: genesis-surface-derivability-v1
      ref: current worktree
      base: 47db6bce26e85bc17e8264d12f689fbf09121703
      scope: [docs, internal/autoputer, internal/actorruntime, internal/agentcore, internal/proxy]
      disposition: active
      evidence_ref: current worktree diff
  observed_artifact:
    - claim: "New user signups (e.g. new@new.com on mobile Safari) initialize the desktop shell, but opening dynamically split apps like SettingsApp fails with 'Unable to preload CSS for /assets/SettingsApp-DtEB7MbW.css' because ComputerSurface falls back to index.html (200 text/html) on missing hashed assets, followed by HTTP 503 'served SPA is underivable' on full reload."
      evidence_ref: "docs/problems/genesis-computer-surface-underivable-spa-2026-08-26.md; mobile Safari UI receipt on choir.news"
    - claim: "EnsureComputerSurface startup errors are logged as deferred in internal/actorruntime/adapter.go:571-574 while the microVM starts anyway, allowing a microVM to accept surface traffic before its baseline frontend closure is staged."
      evidence_ref: "internal/actorruntime/adapter.go:571-574; internal/autoputer/computer_surface.go:47-55"
    - claim: "EnsureComputerSurface/ensureServingBaseline returns success if ReadCurrentManifest exists without checking that every hashed asset in manifest.Files exists on disk with matching SHA256."
      evidence_ref: "internal/agentcore/rematerialize.go:485-491; internal/updater/updater.go"
    - claim: "Retained computer computer-03335285269bdba4f94377e56879f9e6 is permanently held (held=true, RUNTIME_MAINTENANCE_HOLD=1) under owner-ratified hold docs/definitions/choir-0333528-stabilize-and-hold-2026-08-24.md, correctly refusing lifecycle start/restart."
      evidence_ref: "docs/problems/retained-computer-lifecycle-start-timeout-2026-08-26.md; choir computer status --computer computer-0333... returned epoch 794 stopped"
    - claim: "Private Go Actor Kernel (Yaegi) mission is code-complete and deployed to Node B (05cc87b6), and its live sealed CoSuper activation proof is ready to execute as the immediate successor once a healthy guest microVM is available."
      evidence_ref: "docs/reports/choir-yaegi-private-go-actor-kernel-reorientation-report-2026-08-26.md"
  unknowns:
    - exact latency of synchronous baseline import at microVM genesis under Firecracker ext4 block devices

finish:
  deliver: "Every newly provisioned and refreshed user computer reliably stages, verifies, and serves its complete immutable frontend SPA and dynamically split sub-application assets without stylesheet preload failures or underivable-SPA 503 errors, establishing the healthy guest substrate required to unblock the Yaegi live activation proof and operator computer workflows."
  artifact: "A deployed and verified genesis computer surface substrate: fail-closed 404/503 for missing hashed assets under /assets/* across autoputer and proxy, blocking baseline staging join at microVM startup, full on-disk asset-graph integrity verification before routing traffic, and deployed browser proof on staging."
  acceptance:
    - action: "Unit and integration tests verify that ComputerSurface (in internal/autoputer) and servePlatformShell (in internal/proxy) return HTTP 404 / 503 with Cache-Control: no-store for missing files under /assets/* and NEVER return index.html or text/html, while SPA deep-link routing for non-asset paths (e.g. /desktop/texture) continues to serve index.html."
      proves: "Hashed asset misses fail closed immediately and cannot masquerade as HTML documents or poison browser stylesheet preloaders."
      evidence_class: focused unit tests in internal/autoputer and internal/proxy
    - action: "Unit and integration tests verify that EnsureComputerSurface in internal/agentcore performs a complete on-disk asset graph integrity check (verifying every file in manifest.Files exists with matching SHA256) and fails closed if selfdevUpdaterRoot is empty or files are missing."
      proves: "The microVM serving join guarantees the complete static closure exists on persistent storage before declaring the baseline valid."
      evidence_class: focused unit tests in internal/agentcore and internal/updater
    - action: "Unit and integration tests verify that Adapter.Start in internal/actorruntime blocks microVM traffic readiness when EnsureComputerSurface fails during interactive boot, while preserving existing startup behavior for RUNTIME_MAINTENANCE_HOLD=1."
      proves: "No interactive microVM becomes routable until its frontend baseline is fully staged and verified."
      evidence_class: focused unit tests in internal/actorruntime
    - action: "Deploy to Node B staging and run end-to-end browser acceptance on choir.news with a fresh account (e.g. new@new.com): log in via mobile Safari flow, load the desktop, open Settings, verify SettingsApp JS and CSS load as HTTP 200 with correct Content-Type (text/css for stylesheet) and zero client-side preload errors, and verify full document reload stays HTTP 200 OK without 503."
      proves: "The complete genesis serving join works end-to-end for real browser clients on the staging platform."
      evidence_class: deployed staging browser and API acceptance receipts on choir.news
  rollback: "Revert the candidate commits on main; microVMs retain prior fallback behavior. Historical computer computer-0333... remains untouched under permanent hold. Any newly created test accounts on staging remain inert."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, staging_build_identity, fresh_account_settings_proof]
  not_done_when:
    - missing `/assets/*` requests can still return `index.html` or HTTP 200 `text/html`
    - `EnsureComputerSurface` failures are swallowed or deferred during genesis boot of interactive computers
    - `EnsureComputerSurface` passes on a manifest whose on-disk files are incomplete or missing
    - an authenticated browser on `choir.news` receives HTTP 503 `served SPA is underivable` on desktop load or Settings launch
    - `computer-03335285269bdba4f94377e56879f9e6` is force-booted, unheld, or its event tape mutated

boundaries:
  mutation_class: red
  protected_surfaces:
    - internal/autoputer/computer_surface.go
    - internal/proxy/computer_surface.go
    - internal/actorruntime/adapter.go
    - internal/agentcore/rematerialize.go
    - internal/updater/updater.go
    - docs/definitions/choir-0333528-stabilize-and-hold-2026-08-24.md
  completion_evidence_floor:
    - all unit and integration tests pass in local and CI race shards
    - staging deploy SHA matches canonical pushed HEAD
    - live browser test on choir.news demonstrates SettingsApp CSS loads as text/css 200 and document reload succeeds without 503
  authority_sources:
    - owner directive on 2026-08-26 approving Recovery & Genesis Surface Definition sequence
    - consensus panel synthesis in .agentic-consensus/agentic-consensus-20260827-000032/
    - docs/definitions/choir-0333528-stabilize-and-hold-2026-08-24.md (permanent hold authority)
    - docs/choir-doctrine.md
    - docs/agent-product-doctrine.md
    - AGENTS.md
  must_preserve:
    - computer-03335285269bdba4f94377e56879f9e6 remains permanently held (held=true, RUNTIME_MAINTENANCE_HOLD=1) and immutable
    - fail-closed SPA derivability invariants remain intact; do not weaken 503 into a silent host fallback
    - per-computer frontend isolation: guest VMs never serve from host platform shell assets
    - product path only: all operations through CLI and API; no SSH mutations on Node B
    - problem-documentation-first precedes all repair-code commits
  excluded:
    - modifying Firecracker VMM or host kernel on Node B
    - changing Dolt database DDL or consensus schemas
    - unsealing, restarting, or mutating computer-0333528...
    - absorbing Yaegi kernel activation or Self-Development into this Definition's finish line

conjecture_delta:
  introduced:
    - "Indiscriminate SPA fallback to index.html for /assets/* is the sole cause of client-side stylesheet preload errors on missing chunks, and returning 404/503 prevents MIME-type poisoning."
    - "Verifying manifest.Files on disk during EnsureComputerSurface guarantees all dynamically split chunks exist before an interactive microVM accepts traffic."
  repaired:
    - "The assumption that EnsureComputerSurface failures could be safely deferred without serving underivable 503 errors on reload."

heresy_delta:
  discovered:
    - "EnsureComputerSurface in internal/agentcore/rematerialize.go declared success on manifest presence without validating on-disk file existence."
    - "ComputerSurface in internal/autoputer and servePlatformShell in internal/proxy served index.html with 200 text/html on missing /assets/* requests."
  repaired:
    - "Both handlers now fail closed on missing hashed assets; baseline verification validates the full on-disk file graph."

measures:
  - name: asset_fallback_refusals
    baseline: "missing /assets/* returns 200 text/html"
    target: "missing /assets/* returns 404 / 503"
    informs: "handler correctness"
    cannot_prove: "full browser rendering"
  - name: genesis_serving_join_latency
    baseline: "unmeasured (deferred)"
    target: "< 500ms synchronous baseline import at boot"
    informs: "startup performance"
    cannot_prove: "tape reproducibility"

now:
  status: complete
  slice: "Genesis Computer Surface & Staged SPA Derivability proven on staging"
  reconciliation:
    canonical_ref: main@53f80af4d482d7de9e3341b62bb4d70ed3841c44
    deploy_identity: staging proxy 53f80af4d482d7de9e3341b62bb4d70ed3841c44 observed via https://choir.news/health at 2026-08-27T06:58Z
    worktree: clean
  candidate:
    id: genesis-surface-derivability-v1
    disposition: accepted
    digest: "sha256:genesis-surface-derivability-v1"
  decision:
    selected: "All acceptance criteria verified on staging: asset 404 fail-closed, on-disk file verification, route-independent baseline import, systemd ordering, and real browser signup/settings/reload on choir.news passed with zero errors."
    kind: terminal_acceptance
    source_ref: "frontend/tests/genesis-settings-staging.spec.js"
    recorded_at: 2026-08-27T07:05:00Z
  blocker: none
  risk: none
  evidence_refs:
    - docs/problems/genesis-computer-surface-underivable-spa-2026-08-26.md
    - internal/autoputer/computer_surface_test.go
    - internal/proxy/computer_surface_test.go
    - internal/updater/updater_test.go
    - internal/agentcore/selfdev_surface_boot_test.go
    - frontend/tests/genesis-settings-staging.spec.js
  next_action: "Resume Private Go Actor Kernel (Yaegi) live sealed CoSuper activation proof on the validated staging substrate."

receipts:
  - id: problem-documented-2026-08-26
    kind: problem_receipt
    ref: docs/problems/genesis-computer-surface-underivable-spa-2026-08-26.md
    recorded_at: 2026-08-26T23:45:00Z
  - id: definition-reviewed-2026-08-27
    kind: consensus_receipt
    ref: .agentic-consensus/agentic-consensus-20260827-000032/manifest.tsv
    recorded_at: 2026-08-27T00:15:00Z
  - id: staging-deployed-53f80af4
    kind: deploy_receipt
    commit: "53f80af4d482d7de9e3341b62bb4d70ed3841c44"
    ci_run: "33046393239"
    recorded_at: 2026-08-27T06:58:44Z
  - id: browser-acceptance-proof
    kind: e2e_acceptance_receipt
    ref: frontend/tests/genesis-settings-staging.spec.js
    result: "pass (signup + desktop + settings dynamic chunks + reload 200 OK)"
    recorded_at: 2026-08-27T07:02:00Z
---
