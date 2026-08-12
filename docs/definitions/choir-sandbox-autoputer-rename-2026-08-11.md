---
definition_version: 2
definition_id: choir-sandbox-autoputer-rename-2026-08-11
execution_mode: subordinate_only

start:
  captured_at: 2026-08-11T22:10:00Z
  source:
    canonical_ref: main@f1fdaf7c
    deploy_identity: "staging https://choir.news frontend and proxy 914f7a5d976a, proxy status ok, deploy time 2026-08-11T18:11:01Z"
  worktree_inventory:
    status: reconciled
    evidence_ref: 2026-08-11 read-only git status after f1fdaf7c; clean single worktree /Users/wiz/go-choir
    preservation_rule: Preserve every non-primary worktree and all unrelated WIP. This Definition owns the rename manifest and the paths it names.
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      paths_or_digest: [docs/definitions/choir-sandbox-autoputer-rename-2026-08-11.md, cmd/sandbox/, internal/sandbox/, flake.nix, nix/, .github/workflows/, frontend/src/, docs/]
      recovery: revert the mission commits
  candidates:
    - id: none
  observed_artifact:
    - claim: "The generated manifest covers 3,728 case-insensitive sandbox token matches across 294 tracked files outside docs/archive: 2,284 service-surface matches to autoputer, 1,431 identity-surface matches to computer, and 13 explicit exceptions across 6 files. The frozen docs/archive inventory is separately bounded at 85 files and 959 matches. This is not a spot-edit change."
      evidence_ref: docs/evidence/choir-sandbox-autoputer-rename-manifest-2026-08-11.yaml
    - claim: "The identity field is already semantically the computer, not the sandbox. Go code compares stored.SandboxID against computerID directly, and 24 SandboxID / 85 sandbox_id occurrences carry ComputerID values."
      evidence_ref: internal/agentcore/runtime.go:2606; internal/agentcore/tools_worker_update.go:406,656
    - claim: "Doctrine already fixed the product noun and it is 'computer', not 'autoputer'. computer-ontology.md instructs using sandbox only for existing service/process names, and an archived note states 'The product noun is persistent user computer' with the target rename 'sandbox to computer'."
      evidence_ref: docs/computer-ontology.md:8-14,352; docs/archive/deferred-reliability-migrations-2026-05-14.md:11-13; docs/archive/choir-agentic-depth-canonical.md:385
    - claim: "Archived intent for the service name is autoputer: 'Service rename sandbox->autoputer' and 'rename product/runtime terminology from sandbox -> autoputer'. The two archived targets are different surfaces, not a contradiction."
      evidence_ref: docs/archive/mission-og-dolt-heresy-hard-cutover-v0.md:188; docs/archive/mission-report-universal-wire-production-recovery-2026-06-10.md:273-280; docs/archive/universal-wire-production-recovery-missiongradient-2026-06-10.md:463
    - claim: "sandbox_id is a persisted column in at least 6 tables including runs and agents, with index idx_runs_sandbox_id, and there is no migration framework — stores create tables with CREATE TABLE IF NOT EXISTS at startup."
      evidence_ref: internal/store/store.go:144,161,419; internal/platform/computer_events.go:18,39,63
    - claim: "sandbox_id crosses the public API boundary into the deployed frontend and acceptance specs, but all of them deploy from this repo alongside the server, so the rename is an in-repo coordinated change rather than a client break."
      evidence_ref: internal/agentcore/api.go:171,239 (json:\"sandbox_id\"); frontend/src/lib/Desktop.svelte:930; frontend/tests/adaptive-lifecycle-control-deployed.spec.js:26,108,131
    - claim: "Environment, service, and package names form a coordinated deploy surface: SANDBOX_ID, SANDBOX_PORT, SANDBOX_FILES_ROOT, SANDBOX_URL, PROXY_SANDBOX_URL, VMCTL_SANDBOX_URL_BASE, VMCTL_SANDBOX_PACKAGE_DIR, SANDBOX_TOKEN_TTL; systemd units go-choir-sandbox.service, -recovery, -restart, -vm and go-choir-sandbox.env; flake package 'sandbox' from cmd/sandbox."
      evidence_ref: "env/systemd/nix grep 2026-08-11; flake.nix:265-267; nix/sandbox-vm.nix"
    - claim: "Genuine non-product uses of the word exist and must not be rewritten: Nix's own build sandbox in flake.nix comments and options, and docs/archive which is frozen historical text."
      evidence_ref: flake.nix:117,151; docs/archive/ (177 files, excluded from doccheck findings)
    - claim: "A prior mission already scoped this work and set its reliability shape: inventory, exceptions manifest, git mv, case-aware rewrite, schema decision, build/test, staging deploy, deployed product-path proof — with an explicit instruction that blind replacement is unsafe and this is a platform behavior-changing mission rather than cleanup. Its compatibility and dual-write guidance is superseded by the pre-launch no-compat decision."
      evidence_ref: docs/archive/deferred-reliability-migrations-2026-05-14.md:5-45
    - claim: "The manifest is generated before source edits and records every matching line, token count, category, replacement target or exception reason, plus the frozen archive inventory."
      evidence_ref: docs/evidence/choir-sandbox-autoputer-rename-manifest-2026-08-11.yaml
  problems_documented:
    - id: two-targets-not-one-2026-08-11
      problem: "'Rename sandbox to autoputer' names one rename, but the surface holds two. The service, process, package, and environment surface should become autoputer — it is the machine that runs the product. The identity surface (SandboxID, sandbox_id, sandbox_url) should become computer, because doctrine already fixed the product noun as the persistent user computer and the field already carries ComputerID values. Renaming identity fields to autoputer_id would encode the wrong ontology at a cost this large."
      evidence_ref: "see observed_artifact identity and doctrine claims"
      consequence: "The mission renames service surfaces to autoputer and identity surfaces to computer, with an exceptions manifest for genuine non-product uses."
    - id: no-compat-pre-launch-2026-08-11
      problem: "An earlier draft of this Definition deferred persisted columns and public JSON to a successor phase, on the reasoning that renaming them means a data migration and a client-visible break inside a deploy window. That reasoning assumes users and external clients. Choir is pre-launch: there are none. The staging database holds test data, the frontend and acceptance specs deploy from this repo alongside the server, and no third party reads sandbox_id."
      evidence_ref: "owner direction 2026-08-11: pre-launch, no backwards compatibility, cleanest possible codebase; frontend/src/lib/Desktop.svelte and frontend/tests/ deploy from this repo"
      consequence: "One cutover, no phases. Persisted columns, indexes, and public JSON rename with everything else. No compatibility shim, alias, dual-read, dual-write, or legacy field is introduced — those would be the dual-path shape doctrine I5 rejects, adopted to protect consumers that do not exist."
    - id: probe-before-state-drop-2026-08-11
      problem: "Because tables are created with CREATE TABLE IF NOT EXISTS at startup, an existing database keeps its old columns and would not gain renamed ones; the clean cutover therefore drops and recreates staging state. But the effects Definition's first step is a replay completeness probe that measures whether accumulated VM-local rows are derivable from the event chain. Dropping state first makes that probe run against a clean-room database, where it would come back clean for the wrong reason and teach nothing."
      evidence_ref: "internal/platform/computer_events.go:18,39,63 (CREATE TABLE IF NOT EXISTS); docs/definitions/choir-supervised-self-development-effects-2026-08-11.md route map step 1"
      consequence: "Run the effects Definition's replay completeness probe against current accumulated staging state before this mission drops it, and keep the diff as durable evidence. The probe is read-only and diagnostic, so it costs little and preserves a finding that cannot be recovered afterward."
    - id: replay-probe-no-product-path-2026-08-11
      problem: "The effects Definition requires a deployed replay completeness probe that rematerializes VM-local state through the current event head and diffs it against a live DoltStateExtractor reading, but the current product CLI exposes no replay/rematerialization command and the inspected runtime API exposes only self-development operation/checkpoint projections, not the required state diff. SSH or internal Node B access is not an admissible substitute."
      evidence_ref: "go run ./cmd/choir help (2026-08-11); grep of cmd/internal routes for replay/rematerial/observation; docs/definitions/choir-supervised-self-development-effects-2026-08-11.md:75-77; standing question 9"
      consequence: "The owner authorized adding one read-only owner/computer-scoped product API and CLI probe. It must materialize the event chain into a disposable Dolt workspace, compare its content-addressed observations with the live workspace, emit the exact diff and digest, and never append events or mutate current state. The rename remains blocked on running that deployed probe before state drop."
    - id: ci-autoputer-package-order-2026-08-12
      problem: "The first pushed landing commit failed the CI plan contract because the generated SBOM verifier's expected package array was not sorted after the service package rename from sandbox to autoputer. Local focused Go and shell checks did not exercise this CI-only package-order assertion."
      evidence_ref: "CI run 31551473129, Plan CI Lanes job 93974880121, failed Test CI classifiers and differential SBOM reuse step; local reproduction .github/scripts/verify-sbom-candidate-test before repair"
      consequence: "The landing remains incomplete until the failure is documented first, the expected package list is corrected, the contract suite passes locally, and a new pushed CI run is green."
    - id: ci-go-module-vendor-hash-2026-08-12
      problem: "After the package-order repair, the pushed SBOM candidate reached the changed-package build and failed every required Go package whose module closure uses the shared flake vendorHash. Nix expected sha256-NQ3VEnZ8q5Lo1uat8z9lV7YCM4auEkQu6uiI1TcIEvs= but downloaded sha256-9dsR+XGLTVDZ49SYVzNBIEPOxPZNlNlpPplNVeAocSk=; the same fixed-output mismatch appeared for maild, maildctl, corpusd, autoputer, sourcecycled, and vmctl."
      evidence_ref: "CI run 31551955304, Build Differential SBOM Candidate job 93976443314, Reuse semantic inventories or build changed packages logs at 2026-08-12T01:00:21Z-01:02:52Z; flake.nix:103-104"
      consequence: "The landing remains incomplete until this independently documented build failure is repaired, the focused SBOM contract and package build path pass, and a new pushed CI run is green. No staging deployment or state drop is authorized."
    - id: deploy-staging-schema-drift-2026-08-12
      problem: "The first deployment of the renamed runtime reached the existing staging VM but could not start its runtime store: the retained database still had the pre-cutover sandbox_id schema while the new bootstrap attempted an index whose key column is computer_id. vmctl therefore stayed unavailable and the deploy health gate failed."
      evidence_ref: "CI run 31552694600, Deploy to Staging job 93978717729, diagnostics at 2026-08-12T01:25:52Z-01:27:20Z: runtime Error 1072 key column computer_id does not exist in table; proxy health vmctl unavailable; incomplete receipt /var/lib/go-choir/deploy-failures/31552694600-1.json"
      consequence: "The deploy failure is documented before repair. The replay probe must still run against the pre-drop state through the product path; only then may staging state be dropped and recreated so the renamed schema can boot. No direct Node B mutation is an admissible substitute."
    - id: ci-sbom-artifact-attempt-mismatch-2026-08-12
      problem: "On CI rerun attempt 2, Build Differential SBOM Candidate completed and uploaded an artifact named with run attempt 1, while Accept Differential SBOMs looked for the attempt-2 name and failed before verification. The workflow's producer and consumer do not share the same attempt identity under rerun."
      evidence_ref: "CI run 31554455290 attempt 2, Build Differential SBOM Candidate job 93985446583, artifact list sbom-candidate-31554455290-1-e09d822499ce8533bdc8e18d1c6c48e3d4c2fe61, Accept Differential SBOMs download failure at 2026-08-12T01:56:30Z"
      consequence: "The repair CI rerun deployed successfully but the overall run is red on SBOM acceptance. The rename remains blocked until the producer/consumer artifact identity is corrected, the focused workflow contract is verified, and a new pushed CI run is green."
    - id: replay-start-vmctl-unavailable-2026-08-12
      problem: "The deployed read-only replay probe reached the product route but could not resolve the named computer because staging reported the computer stopped. An owner-scoped lifecycle start then timed out with lifecycle actuation failed; the following status call returned computer ownership authority unavailable and proxy health degraded with vmctl unavailable."
      evidence_ref: "deployed e09d822499ce8533bdc8e18d1c6c48e3d4c2fe61; scoped replay command at 2026-08-12T02:29Z returned 502 failed to resolve user autoputer; status returned stopped; start command at 2026-08-12T02:30Z returned 502 lifecycle actuation failed; subsequent status returned 503; https://choir.news/health at 2026-08-12T02:31Z reports vmctl_status unavailable"
      consequence: "The replay diff has not been captured and no state drop is authorized. The staging vmctl/ownership substrate must recover through the authorized deployment path before the probe can be rerun; direct Node B mutation remains inadmissible."
    - id: replay-start-after-vmctl-recovery-2026-08-12
      problem: "The authorized recovery deployment restored staging proxy and vmctl health at commit 4a747934, but the named computer still reported stopped and an owner-scoped lifecycle start again returned HTTP 502 lifecycle actuation failed after 63 seconds. Host health is no longer the immediate blocker; the computer lifecycle actuation path remains unavailable."
      evidence_ref: "CI run 31557302447, Deploy to Staging job 93994767707 completed 2026-08-12T03:03:34Z; https://choir.news/health at 2026-08-12T03:03Z reports vmctl_status ok and commit 4a747934; computer status at 2026-08-12T03:03Z and 2026-08-12T03:07Z reports stopped; owner-scoped computer start at 2026-08-12T03:05Z returned 502 lifecycle actuation failed after 63 seconds"
      consequence: "This second lifecycle failure is documented before repair. The replay diff, state recreation, and renamed acceptance remain unauthorized; repair must use the authorized deployment path and must not mutate Node B directly."
    - id: replay-probe-guest-startup-refused-2026-08-12
      problem: "The authorized recovery deploy restored vmctl health and schema bootstrap, but the retained named computer still cannot reach the replay probe: its existing guest repeatedly refuses runtime startup while reconciling a durable Texture lifecycle subject with an invalid transition, and the vmctl resolve path times out waiting for guest health."
      evidence_ref: "one-time staging recovery diagnostics CI run 31562873202, Inspect first-boot guest signer job 94008710612 at 2026-08-12T04:18Z-04:19Z: candidate-fleet-e15cb89f25d963c220319b7b logged runtime startup refused with reconcile Texture owner ... lifecycle invalid transition; vmctl logged failed to start existing VM and guest did not become healthy at http://10.200.2.2:8085 within 3m; vmctl service itself was active with NRestarts=0"
      consequence: "The replay diff, state recreation, and renamed acceptance remain unauthorized. The next repair must expose the read-only replay route without running the unrelated durable actor/Texture reconciliation, then rerun the probe against the unchanged retained state; the temporary probe-only path must be removed before normal renamed acceptance."
    - id: replay-start-retry-after-vmctl-recovery-2026-08-12
      problem: "A second owner-authorized lifecycle start retry after vmctl health recovery again returned HTTP 502 lifecycle actuation failed after 62.37 seconds; the named computer remains stopped. The failure is repeatable through the deployed product path and remains opaque at the public boundary."
      evidence_ref: "owner-scoped CLI retry at 2026-08-12T03:15Z with idempotency key rename-replay-start-20260812-0315; https://choir.news/health reported vmctl_status ok immediately before the retry; command exited 1 after 62.37 seconds with HTTP 502"
      consequence: "This retry confirms the prior lifecycle failure is not a transient vmctl health outage. No replay diff, state drop, or renamed acceptance is authorized. Repair must expose or resolve the underlying VM launch/readiness failure through the repository/deployment path, then rerun the replay probe before any state drop."
    - id: replay-probe-product-route-binding-2026-08-12
      problem: "The deployed read-only replay probe is present, but the public product route cannot complete its authority join: the CLI names the computer only in the path, while the generic proxy target resolver requires an explicit computer_id selector for owner-wide API keys, and after that selector is supplied the proxy forwards no X-Authenticated-Computer binding required by the guest route."
      evidence_ref: "deployed 6efee77fafffb1493af162bdfdbaffcffd5b88ac; choir computer replay-completeness --computer computer-03335285269bdba4f94377e56879f9e6 --timeout 10m at 2026-08-12T04:47Z returned HTTP 400 owner-wide api key requires a named computer (computer_id); direct deployed request with computer_id query at 2026-08-12T04:48Z returned HTTP 403 authenticated computer binding required; internal/proxy/handlers.go:653-659 and internal/agentcore/api_self_development.go:88-90"
      consequence: "The replay diff remains uncaptured and state drop remains unauthorized. Repair the proxy/CLI route through the authorized repository and deployment path, then rerun the probe against unchanged retained state; do not weaken the guest binding check or substitute a direct host request."
    - id: replay-probe-runtime-identity-mismatch-2026-08-12
      problem: "After the proxy route binding repair, the deployed replay probe reaches the active guest but the guest runtime rejects the request because its configured ComputerID is candidate-fleet-e15cb89f25d963c220319b7b, while vmctl ownership and the public route identify the retained computer as computer-03335285269bdba4f94377e56879f9e6."
      evidence_ref: "staging commit 0b3a592a01745e093709f7a67d547d69cfd43244; choir computer replay-completeness --computer computer-03335285269bdba4f94377e56879f9e6 --timeout 10m at 2026-08-12T05:03Z returned HTTP 503 replay completeness probe unavailable: computer binding does not match runtime; the same deployed computer path's /api/shell/bootstrap at 2026-08-12T05:04Z returned computer_id candidate-fleet-e15cb89f25d963c220319b7b while computer status reported computer-03335285269bdba4f94377e56879f9e6 active."
      consequence: "The replay diff remains uncaptured and state drop remains unauthorized. Repair the authorized VM realization identity/configuration so the guest runtime and vmctl stable ComputerID join, then rerun the read-only probe against unchanged retained state; do not relax the runtime binding check or substitute a direct host request."
    - id: replay-probe-retained-guest-not-refreshed-2026-08-12
      problem: "The stable-identity repair is deployed and proxy/vmctl health is green, but the retained active guest realization still serves its pre-repair runtime: deployment does not refresh an already-running VM, so the guest continues to report the VM realization ID as its ComputerID and the replay probe continues to reject the stable computer binding."
      evidence_ref: "staging commit ba27a3e8ed1815dff9853bf96741b4333cf7c1f4 deployed 2026-08-12T05:35:07Z; https://choir.news/health reports commit ba27a3e8 and vmctl_status ok; computer status reports computer-03335285269bdba4f94377e56879f9e6 active; bootstrap at 2026-08-12T05:36Z still returns computer_id candidate-fleet-e15cb89f25d963c220319b7b; replay probe at 2026-08-12T05:36Z still returns HTTP 503 computer binding does not match runtime."
      consequence: "The replay diff remains uncaptured and state drop remains unauthorized. Use the owner-authorized lifecycle restart/refresh path to boot the retained computer on the deployed guest closure, then verify its identity and rerun the read-only probe; do not mutate Node B directly or drop state."
    - id: ci-runtime-stop-waitgroup-race-2026-08-12
      problem: "The landing CI race shard deterministically reports a data race when a test cleanup calls Runtime.Stop while ExecuteActivationSync is between registering its cancellation and incrementing the runtime WaitGroup. Stop can reach Wait before the activation calls Add, so the runtime lifecycle substrate has an unsafe shutdown boundary."
      evidence_ref: "CI run 31567927970, Go Test (race, agentcore/textureowner shard 3) job 94023605265 and failed-job rerun 94026227436; internal/agentcore/runtime.go:214-243,595"
      consequence: "The temporary rename-mode removal is not releasable until the runtime shutdown ordering is repaired and the focused race test plus a green landing CI run prove the fix."
    - id: ci-sqlite-injection-recovery-busy-2026-08-12
      problem: "Landing CI non-runtime shard 1 intermittently failed TestAdapterSQLiteInjectionAppendRecoveryExecutesWithoutSnapshot because the SQLite recovery assertion observed database is locked (SQLITE_BUSY) instead of completing the no-snapshot injection recovery."
      evidence_ref: "CI run 31569560429, Go Test (race, non-runtime shard 1) job 94028457219; adapter_test.go:2224"
      consequence: "Classify as an independent CI flake or substrate regression only after the same job is rerun against the unchanged repair commit; no code change is authorized from this single failure."

finish:
  deliver: "One clean cutover with no compatibility surface. The service that runs a Choir computer is named autoputer everywhere it is a service, package, process, unit, environment variable, or vocabulary term. The persistent user computer's identity is named computer everywhere it is a field, column, index, or public JSON name. The word sandbox survives only where it genuinely means something else. No shim, alias, dual-read, or legacy field is left behind."
  artifact: "A pushed commit series carrying: the effects Definition's replay completeness probe run against pre-drop staging state with its diff retained as evidence; a generated rename manifest with per-category counts and an exceptions list; git mv of cmd/sandbox and internal/sandbox to their autoputer names; case-aware rewrite of Go identifiers, nix packages, systemd units, environment variables, workflows, scripts, frontend, and current docs; renamed persisted columns and indexes with staging state dropped and recreated at the new schema; renamed public JSON with frontend and acceptance specs updated in the same series; green CI; a staging deploy reporting the new unit and package identity; and a deployed product-path proof that a computer boots, serves, and passes acceptance under the new names."
  acceptance:
    - action: "Produce the rename manifest before any edit: every occurrence classified as service-surface (to autoputer), identity-surface (to computer), or exception (unchanged), with counts per category and the exceptions enumerated by file and reason."
      proves: "The rename is scripted and reviewable by category rather than blind. Blind replacement is the named failure mode of this work."
      evidence_class: repo artifact
    - action: "Apply the rename by generated script, not manual edits. Show the diff reviewed by category and gofmt/build/vet clean."
      proves: "Mechanical consistency across 231 files; manual spot edits are how this work goes wrong."
      evidence_class: repo artifact
    - action: "Prove the cutover is total: no sandbox_id column, index, struct field, or JSON tag remains; the frontend and acceptance specs read the new field; and a grep for compatibility aliases returns nothing."
      proves: "One name, everywhere. A rename that leaves both names working has failed."
      evidence_class: repo artifact
    - action: "Drop and recreate staging state at the new schema in the same landing as the code change, and show startup created the renamed tables rather than preserving old ones."
      proves: "CREATE TABLE IF NOT EXISTS cannot silently keep the old schema alive under new code."
      evidence_class: deployed proof
    - action: "Exceptions hold: Nix's own build sandbox terminology in flake.nix and docs/archive are unchanged, and no HTML or iframe sandbox attribute was rewritten."
      proves: "The rewrite distinguished product ontology from unrelated uses of an ordinary English word."
      evidence_class: repo artifact
    - action: "Full CI green, staging deploy completes, and staging reports the renamed service and package identity."
      proves: "The coordinated deploy surface — units, env, package pointer — moved together rather than partially."
      evidence_class: deployed proof
    - action: "Deployed product-path proof on staging after the rename: a computer boots, serves, and passes the existing acceptance specs unchanged."
      proves: "A vocabulary change did not become a behavior change."
    - action: "Run the owner-authorized replay completeness probe through the deployed product API/CLI against current staging state before the drop, and retain its exact diff and digest as durable evidence."
      proves: "The one finding that the state drop would destroy is captured first. The product path performs a disposable event-chain rematerialization and live DoltStateExtractor comparison without mutating the current computer."
      evidence_class: deployed proof
  rollback: "Revert the mission commits through origin/main and CI, then redeploy and drop staging state again so it recreates at the reverted schema. Keep the rename in a single reviewable commit series so revert does not have to disentangle it from unrelated work. Pre-launch state is test data and is not itself a rollback obligation."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - A compatibility shim, field alias, dual-read, dual-write, or legacy name survived the cutover.
    - Persisted columns and indexes were renamed in code while staging state was left at the old schema, so startup silently kept old tables through CREATE TABLE IF NOT EXISTS.
    - Staging state was dropped before the replay completeness probe ran against it, destroying evidence the effects mission needs.
    - The rename was applied by manual spot edits rather than a reviewable generated script with a manifest.
    - Nix's own sandbox terminology, docs/archive, or an unrelated English use of the word was rewritten.
    - Environment variables, systemd units, and the nix package pointer moved partially, leaving a deploy that boots under mixed names.
    - Identity surfaces were renamed to autoputer rather than computer, encoding the wrong product noun.
    - The mission landed without a deployed product-path proof, on the theory that a rename cannot change behavior.

boundaries:
  mutation_class: red
  authority_sources: [owner-ratified decisions, docs/computer-ontology.md, docs/choir-doctrine.md, docs/agent-product-doctrine.md, AGENTS.md, owner authorization 2026-08-12]
  must_preserve:
    - No compatibility surface. Pre-launch means no shim, alias, dual-read, dual-write, or legacy field is introduced or retained; a rename that leaves both names working has failed.
    - The product noun is computer. Service and process names become autoputer; identity never does.
    - Code schema and deployed state move together. If persisted columns rename, staging state is dropped and recreated in the same landing, because CREATE TABLE IF NOT EXISTS will otherwise preserve old tables silently.
    - docs/archive is frozen; historical documents keep their original vocabulary.
    - Genuine non-product uses of the word sandbox survive, with each exception justified in the manifest.
    - The effects Definition stays executable and is not blocked or edited beyond mechanical service-name updates.
    - Deploy surfaces move atomically: units, environment, package pointer, and workflows land together.
    - Compatibility machinery of any kind.
    - SQLite-to-Dolt cleanup, which is a separate concern from naming.
    - Deleting code that is unused today but scheduled for wiring by the effects Definition (for example the host self-development freeze/propose tools).
    - No unrelated product behavior change. The owner-authorized read-only replay completeness probe is the sole added behavior; it must not append events, alter current state, or become a general replay API.
  protected_surfaces: [persisted schema, public API field names, owner/computer API authority, replay event chain and projection, DoltStateExtractor content witness, systemd units and environment for the guest runtime, nix package pointer and VM configuration, deployment routing, CI workflows]
  completion_evidence_floor: [deployed proof, reviewed rename manifest]

measures:
  - name: manifest coverage
    kind: gate
    baseline: 0 of 2653 non-doc occurrences classified
    desired: every occurrence classified service / identity-deferred / exception before any edit
    decision_use: unlocks the scripted rewrite
    cannot_prove: that the classification is correct, only that it is complete
  - name: compatibility surface
    kind: gate
    baseline: 0
    desired: 0 — no shim, alias, dual-read, dual-write, or legacy field introduced
    decision_use: blocks the mission; pre-launch is the reason to have none
    cannot_prove: that the new names are the right ones
  - name: deployed acceptance after rename
    kind: gate
    baseline: current staging acceptance passing under sandbox names
    desired: same specs passing unchanged under autoputer names
    decision_use: confirms a vocabulary change stayed a vocabulary change
    cannot_prove: absence of latent naming coupling outside the tested paths
  - name: residual sandbox occurrences
    kind: weak_signal
    baseline: 2653 non-doc
    desired: only manifest-listed exceptions and deferred identity surfaces remain
    decision_use: inspect what is left and why; never advances complete alone
    cannot_prove: that the remaining uses are correct

now:
  status: blocked_incomplete
  slice: "The deployed read-only replay probe now completed against the retained pre-drop staging state after the owner-authorized restart refreshed the guest identity. It captured an exact 26-difference report with probe digest 67ec50ed1526659eb04e7d1be6cabc02d33e6b1f16559d1e2e0036f4f3785af1 and result not_equivalent. State recreation and renamed product acceptance remain outstanding."
  question: "Whether the authorized staging state-drop/recreation path can replace the retained pre-cutover state without mutating the durable replay evidence or leaving the temporary probe-only runtime mode enabled."
  reconciliation:
    observed_at: 2026-08-12T05:36:44Z
    source_ref: mission/choir-sandbox-autoputer-rename-2026-08-11@ba51f5ba
    deploy_identity: "staging ba27a3e8, CI run 31565423783, deploy completed 2026-08-12T05:35:07Z; https://choir.news/health reports vmctl_status ok; owner restart receipt 019ff478-7818-7cf4-9061-9643ebda07d1 advanced realization epoch 200 to 201"
    authority_identities: [docs/computer-ontology.md, docs/choir-doctrine.md, docs/agent-product-doctrine.md, AGENTS.md, owner answer 2026-08-12]
    policy_resolution_ref: "owner authorization 2026-08-12: add read-only product API/CLI replay probe"
    worktree_inventory_ref: "2026-08-12T05:36:44Z git status to be reconciled after evidence receipt"
    operator_boundary_ref: ".github/workflows/ci.yml:848-867,1061-1063; .github/scripts/deploy-impact-classify; scripts/node-b-sync-service-pointers"
    operator_boundary: "The staging deploy fetches the immutable tested commit, hard-resets /opt/go-choir to that commit, cleans it, and installs service pointers from the checked-out repository. No external deploy script or runbook is a source/config authority for the rename; inaccessible Node B runtime files are outputs, not additional source surfaces."
    status: reconciled
  candidate:
    id: sandbox-autoputer-candidate-2026-08-11
    state: deployed_probe_evidence_pending_state_drop
    ref: "origin/main@ba51f5ba; deployed runtime source ba27a3e8"
    owner: owner-and-session
    base: fdf7ceb1fe61a847acdd912bd6c0dcd330a5534d
    digest: f04e68284c0a241460c44677e2000412f589554665ffd8bea27713cf41cfd621
    scope: [cmd/autoputer, cmd/choir, internal/autoputer, internal/agentcore, internal/computerevent, internal/computerversion, internal/store, flake.nix, nix/, frontend/, .github/, scripts/, docs/, AGENTS.md, README.md, .gitignore]
    digest_method: "sha256(git diff fdf7ceb1fe61a847acdd912bd6c0dcd330a5534d..HEAD --binary -- . excluding this Definition and the pre-edit manifest receipt)"
    selected: "Split targets, one cutover, before supervised self-development effects: service/process/package/unit/environment/workflow/vocabulary surfaces become autoputer; every persistent identity surface, including Go fields, persisted columns, indexes, public JSON, frontend, and specs, becomes computer. No compatibility surface. Run the effects replay probe before dropping staging state."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: "owner answer recorded 2026-08-11T23:30:01Z in this Definition"
    owner_ratification_ref: "owner answer recorded 2026-08-11T23:30:01Z: split targets; land before effects"
    recorded_at: 2026-08-11T23:30:01Z
    consequence: "The rename is the pre-effects mission boundary. The effects Definition resumes only after the renamed service and computer identity surfaces pass deployed acceptance."
  evidence_refs: [docs/computer-ontology.md, docs/definitions/choir-supervised-self-development-effects-2026-08-11.md, owner answer 2026-08-11T23:30:01Z]
  probe_authorization:
    selected: "Add one read-only owner/computer-scoped product API and CLI replay completeness probe before the rename state drop."
    source: owner
    status: settled
    recorded_at: 2026-08-12T00:00:22Z
    constraints: [disposable replay workspace, live DoltStateExtractor comparison, exact diff and digest, no event append, no current-state mutation, no general replay API]
    consequence: "The probe is the only behavior addition allowed in this mission; it is a red protected-surface change and must be deployed and exercised before staging state is dropped."
  blocker_or_risk: "The pre-drop replay evidence is captured and committed next; the remaining gates are authorized staging state drop/recreation at the renamed schema, removal of temporary RUNTIME_REPLAY_PROBE_ONLY mode, renamed product acceptance, and final Definition receipts."
  next_action: "Commit the exact replay evidence and updated receipt, then use the authorized deployment path to drop and recreate staging state at the renamed schema; verify the temporary probe-only mode is removed before normal acceptance."

receipts:
  - id: rename-manifest-2026-08-11
    boundary: define
    commit_or_artifact: docs/evidence/choir-sandbox-autoputer-rename-manifest-2026-08-11.yaml
    proof_refs: [docs/evidence/choir-sandbox-autoputer-rename-manifest-2026-08-11.yaml, "manifest generator SHA-256 fd4c0a19ed9d5beeb684eb4e9c1b8606b9844bdb544078105743e68959eec71c", "go test ./... -run '^$' (76 packages ok, 21 no tests)", "go vet ./... (pass)", "go run ./cmd/doccheck -mode full (315 docs, 75 warnings)"]
    rollback_ref: "Discard uncommitted candidate; retain the manifest and problem receipt."
    disposition: "accepted as the reviewable pre-edit classification; no deployment or state mutation authorized"
    problem_ref: replay-probe-no-product-path-2026-08-11
    authorization_ref: "owner answer 2026-08-11T23:30:01Z"
    candidate_or_evidence_refs: [sandbox-autoputer-candidate-2026-08-11, docs/evidence/choir-sandbox-autoputer-rename-manifest-2026-08-11.yaml]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml updated in the same working tree"
  - id: replay-probe-source-2026-08-12
    boundary: observe
    commit_or_artifact: [internal/agentcore/replay_completeness.go, internal/agentcore/api_self_development.go, internal/computerevent/appender.go, cmd/choir/main.go]
    proof_refs: ["go test ./internal/agentcore -run '^TestReplayCompletenessUsesDisposableProjectionWithoutMutatingLiveStore$' -count=1 (pass)", "go test ./internal/computerevent ./internal/agentcore ./cmd/choir -run '^$' (compile pass)", "go test ./internal/computerevent ./internal/agentcore ./cmd/choir -count=1 (pass)", "go run ./cmd/choir help (replay command present)", "pnpm build in frontend (source-contract check and Vite production build pass)"]
    rollback_ref: "Revert the probe source and CLI route; no staging state or event chain was mutated."
    disposition: "source path exists; deployed evidence and exact staging diff remain pending"
    problem_ref: replay-probe-no-product-path-2026-08-11
    authorization_ref: "owner answer 2026-08-12: add product-path probe"
    candidate_or_evidence_refs: [sandbox-autoputer-candidate-2026-08-11]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml remain the active registries"
  - id: replay-probe-deployed-pre-drop-2026-08-12
    boundary: acceptance
    commit_or_artifact: docs/evidence/choir-sandbox-autoputer-replay-completeness-2026-08-12.json
    proof_refs: ["choir computer replay-completeness --computer computer-03335285269bdba4f94377e56879f9e6 --timeout 10m (exit 0)", "captured_at 2026-08-12T05:36:44.159743453Z", "result not_equivalent with 26 deterministic DoltStateExtractor differences", "probe_digest 67ec50ed1526659eb04e7d1be6cabc02d33e6b1f16559d1e2e0036f4f3785af1", "live_head null and replay_head null", "staging health commit ba27a3e8; bootstrap computer_id matched requested stable ComputerID after lifecycle receipt 019ff478-7818-7cf4-9061-9643ebda07d1"]
    rollback_ref: "Delete the evidence projection only if the mission is abandoned; the probe itself appended no event and mutated no live state."
    disposition: "accepted as the required pre-drop observation; the exact non-equivalence is retained for the effects mission and does not authorize completion or state recreation by itself"
    problem_ref: replay-probe-no-product-path-2026-08-11
    authorization_ref: "owner answer 2026-08-12: add product-path probe"
    candidate_or_evidence_refs: [sandbox-autoputer-candidate-2026-08-11, docs/evidence/choir-sandbox-autoputer-replay-completeness-2026-08-12.json]
    landing:
      source_commit: ba27a3e8ed1815dff9853bf96741b4333cf7c1f4
      ci_ref: "31565423783 (success)"
      deploy_ref: "Deploy to Staging (Node B) job 94018717399 (success)"
      environment_identity: "https://choir.news/health deployed_commit ba27a3e8ed1815dff9853bf96741b4333cf7c1f4"
      deployed_acceptance: "pre-drop replay route and stable identity proof passed; rename acceptance remains pending state recreation"
    registry_conformance_ref: "docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml remain the active registries"

view:
  path: none
  generator: none
---

# Sandbox to Autoputer Rename

Mini mission preceding
[choir-supervised-self-development-effects-2026-08-11.md](choir-supervised-self-development-effects-2026-08-11.md).
One cutover, no phases, no compatibility surface.

## What the research changed

The instruction was one rename. The surface holds two.

**Service surfaces become autoputer.** `cmd/sandbox`, `internal/sandbox`, the
flake package, `go-choir-sandbox.service` and its recovery/restart/vm siblings,
`SANDBOX_*` and `VMCTL_SANDBOX_*` environment variables, workflow and script
references. These name the machine that runs a Choir computer, and autoputer is
the archived intent for exactly this surface.

**Identity surfaces become computer, not autoputer.** `SandboxID`, `sandbox_id`,
`sandbox_url` carry the persistent user computer's identity — the Go code
already compares `stored.SandboxID` against `computerID`. Doctrine fixed this
noun some time ago: `computer-ontology.md` says to use *sandbox* only for
existing service and process names, and the archived migration note states the
product noun is the persistent user **computer**. Renaming these to
`autoputer_id` would spend a large mechanical cost encoding the wrong ontology.

## No compatibility surface

Pre-launch means there is nobody to be compatible with. The staging database
holds test data, and the frontend and acceptance specs deploy from this repo
alongside the server, so `sandbox_id` crossing the API is an in-repo coordinated
change rather than a client break.

So the rename is total. Persisted columns, indexes, struct fields, JSON tags,
frontend reads, and specs all move together, and nothing is left behind: no
shim, no field alias, no dual-read, no legacy name kept working for a season. A
rename that leaves both names working has failed, and the dual path it creates
is the shape doctrine I5 already rejects.

The one consequence worth stating plainly: because tables are created with
`CREATE TABLE IF NOT EXISTS` at startup, an existing database keeps its old
columns and would not gain renamed ones. Code and state therefore move in the
same landing — staging state is dropped and recreated at the new schema. Test
data is not a rollback obligation.

## Run the probe before dropping state

The effects Definition's first step is a replay completeness probe: rematerialize
VM-local state from the event chain and diff it against a live extractor reading,
to learn whether accumulated rows are derivable from the tape. Dropping state
first makes that probe run against a clean-room database, where it comes back
clean for the wrong reason and teaches nothing.

The probe is read-only and diagnostic, so running it against current accumulated
staging state before the drop costs little and preserves a finding that cannot be
recovered afterward. Its diff is retained as durable evidence.

## Shape

The prior mission already set the reliability shape, and it is adopted here
minus its compatibility guidance:

```text
replay probe on current state -> inventory -> exceptions manifest
-> git mv paths -> case-aware rewrite -> build/vet/test
-> staging deploy with state drop -> deployed product-path proof
```

Blind replacement is the named failure mode. The word *sandbox* has legitimate
non-product uses — Nix's own build sandbox in `flake.nix`, and 177 frozen
archive documents — and each exception is justified in the manifest rather than
discovered in review.

The deploy surface moves atomically. Environment variables, systemd units, the
nix package pointer, and the state drop are one coordinated change; a partial
landing boots staging under mixed names or an old schema.

## Stopping condition

Staging runs the computer under autoputer service names with computer identity
names, passes its existing acceptance specs, and a grep for the old names returns
only manifest-listed exceptions. The effects mission then builds checkpoint and
restore against final names rather than names scheduled to change under it.
