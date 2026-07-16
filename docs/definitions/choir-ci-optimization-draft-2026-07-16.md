---
title: "DRAFT — CI Re-Enablement and Pareto Optimization"
definition_version: 2
draft: true
executable: false

start:
  captured_at: 2026-07-16T03:17:13Z
  source:
    canonical_ref: refs/heads/main@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
    origin_ref: refs/remotes/origin/main@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
    relation: canonical_ref_equals_origin_ref
    deploy_identity: unknown
  worktree_inventory:
    status: reconciled
    evidence_ref: "git status --short observed clean 2026-07-16T03:17:13Z on branch claude/ci-optimization-mission-ttur4k"
    preservation_rule: "This draft may touch only itself and the three definition registries (docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml). It authorizes no edit to .github/workflows/**, .github/scripts/**, or scripts/go-test-*."
  worktrees:
    - path: /home/user/go-choir
      status: clean
      class: goal_candidate
      owner: current-session
      touch: goal_owned
      paths_or_digest: "docs/definitions/choir-ci-optimization-draft-2026-07-16.md and the three registries only"
      recovery: "New draft file plus registry entries; revert by deleting the file and its registry rows."
  candidates:
    - id: none
      ref: none
      base: d87bdc446ecc28585c3bc08d4d469b9f94d3c246
      scope: []
      disposition: paused
      evidence_ref: none
  observed_artifact:
    - claim: "The Race Detector lane is fully disabled. ci.yml job `race` carries `if: ${{ false }}` and a 2026-07-14 owner-pause comment; the `check` gate keeps it in the dependency graph via `require_success \"false\" race`."
      evidence_ref: ".github/workflows/ci.yml:197-202,303-305"
    - claim: "The differential SBOM lane is fully disabled. ci.yml job `sbom` carries `if: ${{ false }}` and the same 2026-07-14 owner-pause comment; the differential builder and cache wiring remain intact."
      evidence_ref: ".github/workflows/ci.yml:309-363"
    - claim: "The path classifier already emits `high_risk_race` (always-on race-sensitive substrate paths) and `sampled_race` (deterministic 5% SHA-bucket sample) outputs, but both are dead because the `race` job is unconditionally off. The intended selective re-enable is described as a two-line condition change."
      evidence_ref: ".github/scripts/ci-impact-classify:138-142,151-158; .github/workflows/ci.yml:197-202"
    - claim: "The reusable Race Detector workflow is complete and independent: 4 agentcore/textureowner race shards plus one non-runtime race job, a weekly `cron: '17 8 * * 1'` schedule, and manual cache restore keyed `go-build-v2-...`."
      evidence_ref: ".github/workflows/race.yml"
    - claim: "The SBOM disablement sits on top of a documented substrate failure: nested `nix build` inside the outer Nix sandbox fails all package SBOMs. A viable host-side sbomnix replacement topology is already identified but not reconnected."
      evidence_ref: docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md
    - claim: "Eight Go jobs run in parallel per non-docs push (1 go-vet-build + 3 non-runtime shards + 4 runtime shards), each performing independent compilation. ci.yml uses `actions/setup-go` `cache: true` keyed on go.mod/go.sum; race.yml uses a divergent manual `go-build-v2` cache key, so the race lane cannot reuse the main lanes' warm build cache."
      evidence_ref: ".github/workflows/ci.yml:104-108,154-158,179-183; .github/workflows/race.yml:34-42"
    - claim: "The runtime shard runner runs `go test <pkg> -list` and then `go test -run <regex>`, compiling the test binary twice per shard."
      evidence_ref: "scripts/go-test-runtime-shards:20-45"
    - claim: "`doccheck` runs unconditionally on every event with full Go setup — it has `needs: plan` but no `if:` gate, so a comment-only or workflow-only push still spins up Go and runs the docs truth checker."
      evidence_ref: ".github/workflows/ci.yml:204-231"
    - claim: "`tla-model-check` downloads tla2tools.jar via curl on every run with no cache; `heresy-detector` apt-installs ripgrep on every run; `deploy-impact` re-runs `deploy-impact-classify-test` and `deploy-workflow-contract-test` already executed by the `plan` job's `ci` lane."
      evidence_ref: ".github/workflows/ci.yml:130-131,241-248,411-415,85-94"
  unknowns:
    - "The observed wall-clock and billed-minute distribution of each lane on real pushes and PRs; no baseline timing has been captured yet."
    - "Whether the `race` and `sbom` pauses were provoked by cost/noise, by the documented SBOM substrate failure, by flakiness, or by unrelated queue pressure — the pause comments state owner direction on 2026-07-14 but not the underlying receipt."
    - "Whether the owner wants race re-enabled selectively (substrate + 5% sample + weekly full) or on every non-docs change, and whether SBOM should block PRs or only run post-merge on main."
    - "The current true-positive yield of the race lane and heresy detector — how often each has actually caught a defect versus consumed minutes."

finish:
  deliver: "CI regains its full safety signal — race detection and differential SBOMs are running again — while median PR feedback time and billed CI minutes drop, achieved as Pareto moves that trade nothing away on correctness or supply-chain assurance."
  artifact: "A revised .github/workflows/ci.yml (+ race.yml, ci-impact-classify, and scripts/go-test-* as needed) where: the race lane is selectively re-enabled through the existing high_risk_race/sampled_race classifier plus the weekly full schedule; the SBOM lane is re-enabled on a fixed, non-nested topology gated to post-merge main; Go compilation is shared across lanes through a unified cache key; and per-run redundant setup (uncached tool downloads, duplicate classifier tests, unconditional doccheck) is removed. Each change is backed by before/after timing evidence."
  acceptance:
    - action: "Capture a pinned baseline: for a representative non-docs push and a representative PR at a fixed SHA, record per-job queue time, run time, and billed minutes for every current lane, plus the total critical-path wall clock, from GitHub Actions run receipts."
      proves: "Optimization is measured against the real observed CI, not against an assumed cost model. Every later 'faster' claim is attributable to a named change."
      evidence_class: ci_run_baseline
    - action: "Re-enable the race lane by restoring the classifier-based condition (run the full race matrix on `high_risk_race` changes, run the sampled shard on `sampled_race`, retain the weekly cron for the exhaustive sweep) and the matching `check`-gate selection, then prove on a substrate-touching PR that race runs and on a non-substrate PR that only the sampled fraction does."
      proves: "Race signal returns for the changes that need it without paying the race tax on every commit; the two-line re-enable claim in the pause comment is realized and verified end to end."
      evidence_class: deployed_ci_lane_proof
    - action: "Re-enable the SBOM lane on the host-side sbomnix topology named in docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md (build the package with Nix on the runner, run pinned sbomnix outside the Nix sandbox), gated to post-merge main, and demonstrate one full baseline followed by a differential run reporting `0 built`, `N reused`, checksum-valid complete artifacts."
      proves: "The disabled lane returns on a topology that actually executes, reconnecting the existing replacement rather than patching the superseded nested-sandbox path, and pays its cost only where a supply-chain artifact is needed (main), not on every PR."
      evidence_class: deployed_ci_lane_proof
    - action: "Unify Go build caching across the main test lanes and the race lane on one cache key derived from go.mod/go.sum and toolchain, and show a measured reduction in aggregate compile time on a warm-cache run versus the pinned baseline."
      proves: "The redundant per-shard and per-lane recompilation is a real, removable cost; the race lane inherits the warm cache instead of cold-building."
      evidence_class: ci_paired_benchmark
    - action: "Admit each streamlining change (gate doccheck to relevant changes, cache tla2tools.jar and ripgrep, drop the duplicate deploy classifier tests, collapse the double runtime-shard compile) only when its GitHub-run before/after shows a wall-clock or billed-minute reduction AND the full `check` gate still passes on a matched change set that exercises the gated lane."
      proves: "Each streamlining move is a Pareto improvement: it removes cost without dropping a gate that would have caught a regression. No lane's coverage silently narrows."
      evidence_class: pareto_admission_receipt
    - action: "Run the revised workflow end to end on a real merge to main: confirm the `check` gate, the re-enabled race and SBOM lanes, and the untouched deploy-impact / deploy-staging path all reach their correct terminal states, and that a docs-only push still takes only the report-only docs path."
      proves: "The optimized CI preserves the landing loop and the docs-only fast path exactly, and the deploy/acceptance gates it feeds are unchanged."
      evidence_class: deployed_ci_acceptance
  rollback: "Every change is a workflow/script edit revertable by a single commit. Re-enablement is guarded by the classifier condition and the `if:` expressions, so any lane can be re-paused by reverting to `if: ${{ false }}` without touching product code. The deploy-staging and flakehub publishing edges are not modified; if a cache-key or topology change regresses, revert that commit and CI returns to the pinned baseline behavior. Keep every baseline and paired-run receipt."
  landing:
    required: true
    environment: github-actions-and-staging-node-b
    required_receipts: [pushed_commit, ci, ci_run_baseline, ci_paired_benchmark, pareto_admission_receipt, deployed_ci_lane_proof, deployed_ci_acceptance]
  not_done_when:
    - "A lane is called re-enabled while its `if:` still resolves to false, or the SBOM lane is re-enabled on the same nested-Nix topology that the problem doc already falsified."
    - "A 'faster' claim rests on intuition, a local run, or a single best-case sample rather than pinned before/after GitHub-run receipts."
    - "A streamlining change removes or narrows a gate's coverage (e.g. doccheck stops running where it would have caught a docs/code drift) in order to save time."
    - "The deploy-impact, deploy-staging, flakehub, or `check` acceptance wiring is altered without full red/black ceremony and a passing landing-loop proof."
    - "Only PR/warm numbers are reported while cold-cache, main-push, and scheduled-sweep costs are left unmeasured."

activation_requirements:
  executable_only_after:
    - "The owner confirms the intended re-enablement policy for race (selective substrate + 5% sample + weekly full, versus every non-docs change) and for SBOM (post-merge main only, versus PR-blocking)."
    - "The underlying reason for the 2026-07-14 race/sbom pause is reconciled from its receipt so re-enablement does not reintroduce the exact condition that motivated the pause."
    - "A pinned CI baseline (per-lane timing and billed minutes) has been captured so Pareto claims have a real comparison point."
    - "The SBOM re-enable is scoped to the documented host-side replacement topology, not the nested-sandbox path."
    - "All three registries promote exactly one revised, owner-ratified successor to executable entrypoint status; this draft is never invoked directly."

boundaries:
  mutation_class: orange
  class_rationale: "The load-bearing work is CI orchestration (.github/workflows/**, .github/scripts/**, scripts/go-test-*), which is orange. It becomes red at three seams that this mission must NOT casually alter: the `check` acceptance gate, the deploy-impact/deploy-staging routing, and the SBOM/flakehub publishing surfaces. Any change to those requires full red ceremony (conjecture delta, protected surfaces, admissible evidence, rollback, heresy delta)."
  conjecture_delta: "Replace 'the safest way to reduce CI cost and noise is to switch race and SBOM fully off' with 'race and SBOM signal can run continuously at low marginal cost by selecting where they run and fixing the SBOM topology, so full disablement was over-broad and each remaining lane cost is independently measured and removable only when it dominates.'"
  protected_surfaces: [ci_check_gate, deploy_impact_classifier, deploy_staging, flakehub_publish, sbom_publishing, race_lane, path_classifier, landing_loop, docs_only_fast_path]
  completion_evidence_floor: [ci_run_baseline, ci_paired_benchmark, deployed_ci_lane_proof, pareto_admission_receipt, deployed_ci_acceptance]
  authority_sources:
    - "owner direction recorded 2026-07-16: review the CI system and its docs; hypothesize a better CI; re-enable disabled lanes while speeding up and streamlining; Pareto optimizations; output is a PR containing this CI-optimization mission draft"
    - AGENTS.md
    - docs/standing-questions.md
    - docs/choir-doctrine.md
    - docs/agent-product-doctrine.md
    - docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md
  must_preserve:
    - "The `check` gate remains the single acceptance authority: every selected lane must still be required, and a selected-but-failed lane must still fail the run. Re-enabling a lane means adding its `require_success` back, not loosening the gate."
    - "The docs-only fast path stays intact: docs/** and top-level *.md changes must not be forced onto the full CI or staging path (per AGENTS.md Landing Loop)."
    - "The deploy-impact → deploy-staging → deployed-acceptance landing loop and the flakehub publishing edge are not weakened, reordered, or made to depend on optimization state."
    - "SBOM re-enablement uses the documented host-side sbomnix topology and pins sbomnix to flake.lock; it does not resurrect the nested-Nix path or fetch an unpinned sbomnix."
    - "Every performance change is proved by GitHub-run before/after receipts at pinned SHAs; local timing and single samples are inadmissible for a Pareto-admission claim."
    - "Correctness and coverage gate first: a faster configuration that drops or narrows any lane's true-positive coverage is rejected regardless of the time it saves."
  excluded:
    - "Executing this draft or editing any workflow/script before owner policy confirmation, pause-reason reconciliation, and a pinned baseline exist."
    - "Changing product runtime, provider/model routing, or application behavior — this mission is CI-only."
    - "Weakening the check gate, deploy routing, staging acceptance, or supply-chain publishing to win a time or minutes number."
    - "Migrating CI to a different runner provider or self-hosted fleet, or introducing new external CI services, as part of this optimization."

measures:
  - name: pr_feedback_wall_clock
    kind: telemetry
    baseline: "unknown until the pinned GitHub-run baseline is captured"
    desired: "lower median PR critical-path time to the `check` gate without dropping a required lane"
    decision_use: "Rank streamlining candidates by critical-path reduction; stop optimizing lanes that are not on the critical path."
    cannot_prove: "That coverage is preserved or that a lane still catches its target defect class."
  - name: billed_ci_minutes_per_push
    kind: telemetry
    baseline: "unknown until baseline capture"
    desired: "lower total billed minutes per non-docs push at equal or greater signal"
    decision_use: "Compare full-race-on-everything versus selective-race, and PR-SBOM versus main-only SBOM."
    cannot_prove: "That a cheaper configuration is as safe; safety is proved by the coverage gate, not the minute count."
  - name: go_compile_reuse
    kind: telemetry
    baseline: "unknown: 8 parallel Go jobs compile independently; race lane uses a divergent cache key"
    desired: "raise warm-cache hit rate and cut aggregate compile seconds via a unified cache key"
    decision_use: "Choose between per-shard caching, a shared prime-the-cache job, and unified keys."
    cannot_prove: "End-to-end correctness or that shard partitioning is balanced."
  - name: race_lane_true_positive_yield
    kind: weak_signal
    baseline: "unknown; the lane is currently off"
    desired: "keep the coverage that catches real data races while paying for it only where races are plausible"
    decision_use: "Justify the substrate-path list and the 5% sample rate; revisit if the sample never catches anything or the substrate list misses a real race."
    cannot_prove: "Absence of races in unsampled commits."
  - name: sbom_freshness_cost
    kind: weak_signal
    baseline: "no successful complete SBOM baseline exists (per problem doc)"
    desired: "produce checksum-valid differential SBOMs on main at low marginal cost via reuse"
    decision_use: "Decide PR-blocking versus post-merge-only placement and the reuse cache retention."
    cannot_prove: "That the SBOM contents are complete or that supply-chain risk is eliminated."

experiment_portfolio:
  rule: "These are conjectures, not commitments. Capture the pinned baseline first; test cheap/high-leverage candidates first; retain, revise, or reject each from GitHub-run evidence. Newly discovered candidates may replace them when they dominate on the Pareto frontier of feedback time, billed minutes, and preserved coverage."
  candidates:
    - id: selective-race-reenable
      idea: "Restore the classifier-driven race condition: full race matrix when `high_risk_race` (substrate paths) is true, the single sampled shard when `sampled_race` (5% SHA bucket) is true, plus the existing weekly cron for the exhaustive sweep; restore the matching `check`-gate `require_success`."
      experiment: "On a substrate-touching PR confirm the full matrix runs; on a non-substrate PR confirm only the sampled fraction runs; measure added minutes versus race-on-everything."
      principal_risk: "The substrate path list under-covers a package that later races, or the sample rate is too low to catch intermittent races in time."
    - id: host-side-sbom-reconnect
      idea: "Re-enable SBOM on the pre-consolidation host-side topology (Nix-build the package on the runner, run pinned sbomnix outside the sandbox, write CycloneDX into the artifact dir) while keeping the derivation-identity manifest, checksum verification, complete-bundle cache, and diff."
      experiment: "Run one full baseline (all packages built) then a no-op-change differential run; require `0 built`, `N reused`, checksum-valid artifacts, and pin sbomnix to flake.lock."
      principal_risk: "sbomnix pinning drift, or the differential cache key not surviving across runs."
    - id: sbom-main-only-placement
      idea: "Gate SBOM to post-merge main (and workflow_dispatch), not PRs, since it is a supply-chain artifact rather than a per-PR correctness gate."
      experiment: "Compare PR feedback time and billed minutes with SBOM on-PR versus main-only; confirm main still produces the artifact for every deployed SHA."
      principal_risk: "A dependency-changing PR merges without its SBOM being previewed; mitigate by classifier flagging flake/go.mod changes."
    - id: unified-go-build-cache
      idea: "Use one Go build/module cache key (go.mod/go.sum + toolchain + OS/arch) across go-vet-build, both test shard families, and the race lane so all lanes and shards share warm compilation."
      experiment: "Measure aggregate compile seconds and per-shard cold-start on a warm-cache run versus the divergent-key baseline; verify the race lane restores from the same key."
      principal_risk: "Cache contention or eviction under GitHub's cache size limits; a stale cache masking a build change."
    - id: prime-then-fan-out-compile
      idea: "Optionally add a single prime-the-cache build job that compiles the module once, so the fan-out test shards restore rather than each compiling from cold."
      experiment: "Compare total critical-path time with prime-job serialization overhead versus independent per-shard compile."
      principal_risk: "The added serial dependency lengthens the critical path if the prime job is slower than parallel cold builds."
    - id: single-compile-runtime-shards
      idea: "Eliminate the double compile in go-test-runtime-shards (list then run) by discovering the test set without a separate compiling `-list` pass, or by caching the compiled test binary between list and run."
      experiment: "Measure per-shard time with one compile versus the current list+run two-compile path."
      principal_risk: "Test discovery correctness — the shard partition must remain identical and deterministic."
    - id: gate-doccheck-to-relevant-changes
      idea: "Give doccheck an `if:` so it runs when docs OR its witnessed code/manifests change, instead of unconditionally spinning up Go on every push including workflow-only or comment-only changes."
      experiment: "Confirm doccheck still runs on every docs and witnessed-code change (no coverage loss) and is skipped on unrelated-only changes; measure minutes saved."
      principal_risk: "Under-gating so a doc/code drift ships unchecked; the witness set must match the checker's real inputs."
    - id: cache-external-tool-downloads
      idea: "Cache tla2tools.jar (pinned v1.7.4) and preinstall/cache ripgrep instead of curl-downloading the jar and apt-installing ripgrep on every run."
      experiment: "Measure setup time for tla-model-check and heresy-detector before/after caching."
      principal_risk: "Cache-key staleness on a tool version bump; low absolute savings if these lanes are off the critical path."
    - id: drop-duplicate-classifier-tests
      idea: "Run the deploy classifier/contract tests once (in the `plan` job's `ci` lane) rather than re-running them inside deploy-impact."
      experiment: "Confirm the tests still run for any .github/** change and remove the duplicate step from deploy-impact; measure the minutes saved on main pushes."
      principal_risk: "A path that reaches deploy-impact without having triggered the `ci` lane loses the check; verify the classifier triggers overlap."
    - id: merge-vet-into-a-shard
      idea: "Evaluate folding `go vet ./...` into an existing shard (which already compiles everything) to remove one job spin-up, versus keeping vet as an independent fast-fail gate."
      experiment: "Compare a dedicated go-vet-build job's spin-up cost against vet-inside-shard, weighing fast-fail value."
      principal_risk: "Losing the early, cheap vet fast-fail signal that currently short-circuits a broken build."
    - id: fail-fast-critical-path-tuning
      idea: "Review `fail-fast` and concurrency settings per matrix so a genuinely broken change cancels its siblings quickly while flaky-independent shards still all report."
      experiment: "Compare time-to-red on an intentionally broken change under current versus tuned fail-fast."
      principal_risk: "Over-aggressive cancellation hides a second, independent failure that a full run would have surfaced."

now:
  status: blocked_incomplete
  slice: "draft-only-ci-optimization-successor-definition"
  question: "What is the owner's intended re-enablement policy (selective versus universal race; main-only versus PR-blocking SBOM), and what receipt explains the 2026-07-14 pause so re-enablement does not reintroduce its cause?"
  reconciliation:
    observed_at: 2026-07-16T03:17:13Z
    source_ref: main/origin@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
    deploy_identity: unknown
    authority_identities:
      - doctrine:AGENTS.md@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
      - standing_questions:docs/standing-questions.md@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
      - doctrine:docs/agent-product-doctrine.md@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
      - problem:docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
      - mission_graph:docs/mission-graph.yaml@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
      - authority_manifest:docs/doc-authority-manifest.yaml@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: start.worktree_inventory
    status: reconciled
  candidate:
    id: none
    state: paused
    ref: none
    owner: none
    base: none
    digest: none
    scope: []
  decision:
    selected: "Retain this document as a non-executable draft successor. Re-enable race and SBOM and pursue CI speed only as Pareto moves proved by GitHub-run before/after receipts; capture a pinned baseline before any change; never weaken the check gate, deploy routing, staging acceptance, or supply-chain publishing to win a time number."
    kind: purpose
    status: proposal
    source: owner
    evidence_ref: "Owner direction in this 2026-07-16 conversation requesting a CI-optimization mission draft as PR output."
    owner_ratification_ref: not_applicable_until_execution
    recorded_at: 2026-07-16T03:17:13Z
    consequence: "No workflow or script edit, timing claim, or /goal invocation is authorized. The next action is deferred until owner policy confirmation and pause-reason reconciliation, after which this draft is revised, baselined, and promoted through all registries."
  evidence_refs:
    - .github/workflows/ci.yml
    - .github/workflows/race.yml
    - .github/scripts/ci-impact-classify
    - scripts/go-test-runtime-shards
    - docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md
  blocker_or_risk: "The 2026-07-14 pause receipt and the owner's re-enablement policy are unknown, and no pinned CI timing baseline exists, so every optimization ranking and every 'safe to re-enable' claim remains conjectural."
  next_action: "Obtain the owner's re-enablement policy and the pause reason, capture a pinned per-lane GitHub-run baseline (read-only), then reconcile and revise this draft, set the first bounded experiment set, and promote a revised executable successor through registry hygiene before invoking /goal."

receipts: []

view:
  path: none
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-ci-optimization-draft-2026-07-16.md --serve 127.0.0.1:8789 --watch"
  authority: "Any rendered view is a non-authoritative projection. This file is a non-executable draft until revised and promoted."
---

# DRAFT — CI Re-Enablement and Pareto Optimization

This is a successor design surface, not a command. Do not run `/goal` on it.

The intent is to get the full CI safety signal back — the **Race Detector** and
**differential SBOM** lanes that were paused on 2026-07-14 — while making CI
*faster* and *leaner*, as strict Pareto moves that give up nothing on correctness
or supply-chain assurance.

Two facts make this tractable rather than a rewrite:

1. **The race re-enable is already designed.** `ci-impact-classify` still emits
   `high_risk_race` (always run race on substrate-touching changes) and
   `sampled_race` (a deterministic 5% SHA-bucket sample), and `race.yml` still
   holds the weekly full sweep. The lane is off only because `race`'s `if:` is
   hard-`false`. Re-enabling it selectively is the "two-line condition change"
   the pause comment itself describes — the safety signal returns without paying
   the race tax on every commit.

2. **The SBOM re-enable already has an identified fix.** The lane was paused on
   top of a documented substrate failure —
   [`docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md`](../problems/ci-sbom-nested-nix-sandbox-2026-07-09.md)
   — where nested `nix build` inside the outer Nix sandbox fails every package
   SBOM. Per the repository's *Check for Existing Fixes* and *Deletion-First*
   heuristics, the mission reconnects the pre-consolidation host-side sbomnix
   topology (already the named replacement) instead of patching the superseded
   nested path, and pays for it only post-merge on main.

The remaining speed work is redundant-compute removal: eight Go jobs compile the
same module independently while the race lane uses a divergent cache key; the
runtime shards compile twice (list then run); `doccheck` spins up Go on every
push regardless of relevance; and external tools (tla2tools.jar, ripgrep) are
re-fetched every run. Each is a candidate in the portfolio above, and each is
admissible **only** with a GitHub-run before/after receipt showing it removed
cost without narrowing a gate's coverage.

Per the *Problem Documentation First* invariant, the SBOM substrate problem is
already recorded; this draft references it rather than re-deriving it. Before
this draft becomes executable, capture a pinned CI baseline, confirm the owner's
re-enablement policy and the pause reason, and promote exactly one revised
successor through all three registries.
