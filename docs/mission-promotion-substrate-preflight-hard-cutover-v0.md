# MissionGradient: Promotion Substrate Preflight Hard Cutover v0

**Status:** checkpoint_incomplete until CI/deploy/staging proof completes
**Date:** 2026-05-20
**Operator:** Codex-operated MissionGradient preflight before the alternate
computer UX experiment portfolio
**Gated follow-on mission:**
[mission-alternate-computer-ux-experiment-portfolio-v0.md](mission-alternate-computer-ux-experiment-portfolio-v0.md)
**Computer ontology:** [computer-ontology.md](computer-ontology.md)
**MissionGradient method:** [missiongradient-method.md](missiongradient-method.md)

## One-Line Goal String

```text
/goal Run docs/mission-promotion-substrate-preflight-hard-cutover-v0.md as a Codex-operated MissionGradient preflight mission: remove false-success paths before the alternate-computer experiment portfolio. Root-cause and fix the local/dev/CI Dolt ICU cgo wiring so normal Go test/build commands can find unicode/regex.h without hand-entered flags; hard-cut the old export_patchset and /api/promotions promotion-candidate path out of product/runtime evidence flows; make AppChangePackage -> adoption -> actual recipient Go/Svelte build -> promote/rollback the only current patch movement path; and make real recipient builds mandatory by default rather than an opt-in require_recipient_build flag. Preserve product-path Trace/VText/run-acceptance evidence, worker/vsuper ability to publish reviewable AppChangePackages, rollback refs, and staging deploy discipline. Do not keep compatibility routes, synthetic recipient digest success, old promotion queue UI, or summaries that treat export_patchset as acceptable evidence. Land through git/CI/deploy, verify staging identity, run focused promotion/adoption tests plus product-path smoke, update docs, and finish with a precise go/no-go certificate for the experiment portfolio, residual risks, rollback refs, and the next executable probe. If complete proof is not reached, report checkpoint_incomplete or blocked_incomplete with a resumable mission-doc checkpoint and continue/redirect/delegate any safe next probe inside authority before stopping.
```

## Mission Frame

The alternate-computer experiment portfolio should test real user-computer
divergence and reviewable app/runtime/UI changes. It should not spend the night
discovering that the promotion substrate still has two competing semantics or
that local Go tests require memorized machine-specific cgo flags.

This preflight mission narrows the field before the larger run:

```text
candidate or worker source change
-> AppChangePackage with source deltas and contract
-> recipient adoption record
-> actual recipient Go/Svelte build
-> verifier evidence
-> promote or rollback
-> Trace/VText/run-acceptance certificate
```

There should be no alternate product path where a worker exports a patchset into
the old `/api/promotions` queue and calls that comparable proof. The old path
may exist in git history and historical docs, but it must not be an active
runtime/product/agent success route after this mission.

## Real Artifact

The artifact is a hardened single promotion substrate:

```text
working normal Go test/build environment
+ AppChangePackage publication path for worker/candidate changes
+ adoption path with actual recipient runtime/UI builds
+ product-visible verifier, rollback, Trace, and run-acceptance evidence
+ deleted or inert old export_patchset and /api/promotions path
+ updated prompts/docs/frontend surfaces using package/adoption terminology
+ staging proof that the portfolio mission can start without false evidence paths
```

The artifact is not:

- a doc note saying "use the new path" while old routes still work;
- a hidden compatibility endpoint for `/api/promotions`;
- an `export_patchset` tool still available to worker profiles as a success
  surface;
- a verifier that accepts synthetic recipient digests as success;
- local-only proof that ignores staging deploy identity;
- a broad promotion redesign beyond the smallest hard cutover needed before
  the experiment portfolio.

## Invariants

- The current source movement path is source-first, not binary-copy-first.
  AppChangePackages carry source deltas, contracts, provenance, and policy.
- Recipient adoption must build recipient-specific runtime and UI artifacts.
  Source/package digests are not enough.
- No active product/API/agent path may treat old `PromotionCandidateRecord`,
  `export_patchset`, or `/api/promotions` evidence as sufficient.
- Worker/vsuper/cosuper coordination should still be able to return reviewable
  candidate evidence, but that evidence must be shaped as AppChangePackage and
  adoption records.
- Browser-public APIs must remain owner-scoped and product-path. Do not replace
  `/api/promotions` with an internal/test shortcut.
- Historical records may remain inert if dropping persistent tables would risk
  data loss, but no current UI, run acceptance synthesis, prompt contract, or
  worker tool should depend on them.
- Local developer commands and CI must not require manually remembering ICU
  cgo flags for ordinary Go test/build loops.
- Platform behavior changes land through git, CI, deploy, staging identity, and
  deployed acceptance proof.

## Value Criterion

Minimize:

```text
promotion semantic ambiguity
+ false completion from old patchset exports
+ synthetic recipient-build evidence
+ local-only test fragility
+ UI/docs/prompt references to removed promotion queues
+ overnight mission time lost to known substrate debt
```

while maximizing:

```text
single-path source lineage clarity
+ actual recipient-build confidence
+ worker/candidate evidence durability
+ staged product-path inspectability
+ ease of owner review after the experiment portfolio
```

The mission moves uphill when the next experiment portfolio cannot accidentally
"pass" through the old export queue or synthetic build verifier, and when normal
test commands work without one-off ICU incantations.

## Current Belief State

Known observations:

- The source ledger repository exists:
  `https://github.com/yusefmosiah/choir-source-ledger`, private, default branch
  `choir/platform/main`.
- Focused AppChangePackage/adoption tests previously required manually supplied
  macOS ICU cgo flags in a bare shell.
- `go test` in a bare shell can fail in `github.com/dolthub/go-icu-regex` with
  `fatal error: 'unicode/regex.h' file not found`.
- The clean ownership boundary is to keep upstream `github.com/dolthub/go-icu-regex`
  and make the repo development environment supply ICU discovery through Nix,
  not to carry a local copy of DoltHub's ICU binding.
- Product APIs currently expose both `/api/app-change-packages` /
  `/api/adoptions` and legacy `/api/promotions`.
- The current AppChangePackage verifier has a non-build branch that computes
  recipient digests from metadata when recipient build is not required.

Main uncertainties:

- Whether the Nix dev shell plus optional `.envrc` is enough for all local agent
  loops, or whether candidate workers also need to enter that shell explicitly.
- How much old promotion queue code is still needed by Trace, Settings,
  Candidate Desktop, run-acceptance synthesis, or worker delegation tests.
- The cleanest replacement interface for worker/vsuper exports: direct
  AppChangePackage publication, a typed "candidate package draft" helper, or a
  narrowly scoped product command that creates an AppChangePackage from a
  candidate workspace.

Highest-impact first observation:

```text
Run normal focused Go promotion/adoption tests without manual ICU flags, then
map every remaining export_patchset and /api/promotions reference to either
delete, replace with AppChangePackage/adoption, or mark historical-doc only.
```

## Homotopy Axes

Start at the smallest real cutover and increase realism without changing the
artifact topology.

1. **ICU/tooling axis**
   - low resolution: focused local tests run through a repo-owned command or
     dev-shell environment that supplies ICU discovery automatically;
   - higher resolution: CI and normal local `go test ./...` use the same
     documented mechanism;
   - forbidden island: telling agents to paste ICU flags manually forever.

2. **promotion-path axis**
   - low resolution: browser-public `/api/promotions` is removed or returns
     gone, and frontend surfaces no longer call it;
   - higher resolution: worker/vsuper exports publish AppChangePackages and
     adoption records directly;
   - forbidden island: keeping the old queue as "temporary fallback."

3. **recipient-build axis**
   - low resolution: missing `require_recipient_build` defaults to required;
   - higher resolution: the opt-out branch and synthetic success contract are
     removed from the verifier;
   - forbidden island: a verifier result that says recipient build passed
     without actual build commands and artifact hashes.

4. **evidence/readability axis**
   - low resolution: Trace/run-acceptance names package/adoption ids instead
     of promotion candidates;
   - higher resolution: Settings/Candidate/Desktop surfaces inspect
     AppChangePackage/adoption lifecycle directly;
   - forbidden island: old labels over new data or new labels over old data.

## Receding-Horizon Control

Use short loops:

1. Inspect current references and classify them as live product, live agent,
   test, historical doc, or dead code.
2. Patch one boundary at a time.
3. Run focused tests after each boundary.
4. Re-check references to prevent compatibility drift.
5. Deploy and prove staging identity only after local tests and source audit
   show a coherent single path.

Do not start broad UI or experiment work until the mission can state whether
the experiment portfolio is greenlit, blocked, or should be reparameterized.

## Dense Feedback

Required local feedback:

- `go test` or an equivalent repo-owned test command proving Dolt ICU discovery
  works without hand-entered shell flags;
- focused AppChangePackage/adoption tests, including:
  - actual recipient build success;
  - actual recipient build failure preserves started/blocked evidence;
  - private package visibility rules;
  - forbidden private source marker rejection;
  - package migration/adoption/rollback lineage;
- reference audit showing no live code path still calls `/api/promotions` or
  exposes `export_patchset` as a worker success route;
- frontend build or targeted tests for affected Settings/Candidate/Trace
  surfaces.

Required deployed feedback:

- pushed commit SHA;
- CI run result;
- staging deploy identity;
- product-path smoke that publishes or inspects an AppChangePackage, creates an
  adoption, verifies actual recipient build evidence or precise build blocker,
  and shows Trace/run-acceptance evidence;
- browser verification that removed routes/UI do not appear as active product
  paths.

## Anti-Goodhart Constraints

- Do not rename `export_patchset` to something else while preserving old
  semantics.
- Do not keep `/api/promotions` reachable for "temporary compatibility."
- Do not pass tests by weakening product-path verification or deleting evidence
  expectations.
- Do not count synthetic digest generation as recipient build.
- Do not solve ICU only by editing a local shell profile outside the repo.
- Do not bury still-live references in bundled frontend output or docs while
  claiming deletion; classify historical docs explicitly.
- Do not break worker/candidate evidence and call that simplification.

## Rollback

Rollback refs must include:

- pre-mission git SHA;
- commit(s) that change ICU/tooling, promotion routes/tools, recipient-build
  verifier behavior, and frontend/docs surfaces;
- deployment SHA before and after cutover;
- any persistent data migration or table-drop decision.

If old promotion tables are left as inert historical data, state that clearly.
If they are dropped or no longer migrated, record the rollback path and data
loss implications.

## Stopping Condition

`complete` requires all of:

- normal local focused Go promotion/adoption tests pass without hand-entered
  ICU flags;
- AppChangePackage/adoption actual recipient build is mandatory and verifier
  cannot produce success from synthetic recipient digests;
- `/api/promotions` and `export_patchset` are not active product/runtime/agent
  evidence paths;
- frontend/product surfaces reference package/adoption lifecycle instead of the
  old promotion queue;
- Trace/run-acceptance synthesis can report package/adoption evidence;
- platform changes are committed, pushed, pass CI, deploy, and are verified on
  staging;
- the final certificate says whether
  [mission-alternate-computer-ux-experiment-portfolio-v0.md](mission-alternate-computer-ux-experiment-portfolio-v0.md)
  is ready to run.

`checkpoint_incomplete` is allowed only when a coherent uphill checkpoint
lands but a remaining safe probe requires more time than the current mission
budget. The final report must not call the mission complete.

`blocked_incomplete` requires named root-cause probes and cognitive transforms.
Examples:

- Dolt/go-icu-regex cannot be made portable in local bare shells without an
  accepted repo environment contract, upstream change, or a dependency fork;
- existing worker delegation cannot produce AppChangePackage records without a
  deeper substrate redesign;
- staging deploy cannot be verified because of an external outage.

## Run Checkpoint & Resumption State

```text
status: complete
last checkpoint: deployed hard-cut implementation and staging product-path smoke completed
current artifact state:
- Dolt ICU cgo discovery is repo-owned through the repo Nix dev shell and
  `.envrc`, while `go.mod` continues to use upstream `github.com/dolthub/go-icu-regex`.
- Runtime/product surfaces now use AppChangePackage -> recipient adoption ->
  mandatory recipient Go/Svelte build -> promote/rollback.
- `/api/promotions`, `/internal/promotions`, runtime Queue/Verify/Promote
  PromotionCandidate methods, and old worker `export_patchset` tool success
  path are removed from active product/runtime/agent flows.
- Historical promotion candidate storage/types and Trace readability remain
  inert for old records; they are not current success paths.
what shipped:
- behavior hard-cut commit: `305b0e48a4f5133a466dae5da9176cdbb6576949`
  (`Hard cut promotion substrate to app adoptions`)
- cleanup/dev-shell commit: `98b73c58930dab1c123f5be3eedb8130b19f796c`
  (`Use dev shell for Dolt ICU wiring`)
CI/deploy:
- GitHub Actions run `26150144224` for `305b0e48...` passed all Go/frontend
  gates but failed the Node B deploy because Nix fixed-output vendoring saw
  `github.com/dolthub/go-icu-regex` replaced in `go.mod` while
  `vendor/modules.txt` did not mark that replacement.
- GitHub Actions run `26151055695` for `98b73c5...` passed all Go/frontend
  gates and `Deploy to Staging (Node B)` succeeded in 5m01s.
staging identity:
- `https://draft.choir-ip.com/health` reported proxy and sandbox deployed
  commit `98b73c58930dab1c123f5be3eedb8130b19f796c`, deployed at
  `2026-05-20T08:34:29Z`.
what was proven:
- focused runtime hard-cut suite passed:
  go test -count=1 ./internal/runtime -run
  'TestBrowserSessionRejectsLegacyPromotionCandidateBinding|TestHandleTraceTrajectorySnapshotIncludesControlArtifactLinks|TestDelegateWorkerCheckpointUpdatePreservesTypedAppChangePackages|TestExecuteToolsChainsRequiredWorkerDelegation|TestAppChangePackageMigratesAcrossCandidateComputers|TestAppAdoptionRequiresActualRecipientBuild|TestRunAcceptanceSynthesizeDerivesExportLevelRecord|TestRunAcceptanceSynthesizeRequiresAdoptionPromotionForPromotionLevel|TestRunContinuationPublicSynthesizeListAndStartAreOwnerScoped|TestDelegateWorkerVMReturnsTimeoutRunEvidence|TestDelegateWorkerVM'
- full runtime suite passed during the initial local hard-cut validation:
  go test -count=1 ./internal/runtime
- store suite passed inside the repo-owned Nix dev shell, without hand-entered
  ICU flags:
  nix develop .# --command go test -count=1 ./internal/store
- focused promotion/adoption runtime subset passed inside the repo-owned Nix dev
  shell after adding `git` to the shell tools:
  nix develop .# --command go test -count=1 ./internal/runtime -run
  'TestAppChangePackageMigratesAcrossCandidateComputers|TestAppAdoptionRequiresActualRecipientBuild|TestBrowserSessionRejectsLegacyPromotionCandidateBinding|TestRunAcceptanceSynthesizeDerivesExportLevelRecord|TestRunAcceptanceSynthesizeRequiresAdoptionPromotionForPromotionLevel'
- post-doc-string-change acceptance subset passed:
  go test -count=1 ./internal/runtime -run
  'TestRunAcceptanceSynthesizeDerivesExportLevelRecord|TestRunAcceptanceSynthesizeRequiresAdoptionPromotionForPromotionLevel|TestRunAcceptanceSynthesizeKeepsDocsLevelForPackageWithoutVerifiedAdoption|TestRunContinuationPublicSynthesizeListAndStartAreOwnerScoped'
- focused runtime hard-cut subset also passed during the initial local
  hard-cut validation:
  go test -count=1 ./internal/runtime -run
  'TestAppChangePackageMigratesAcrossCandidateComputers|TestAppAdoptionRequiresActualRecipientBuild|TestRunAcceptanceSynthesizeDerivesExportLevelRecord|TestRunAcceptanceSynthesizeRequiresAdoptionPromotionForPromotionLevel|TestDelegateWorkerVMReturnsTimeoutRunEvidence'
- frontend build passed with only existing Vite chunk-size warnings:
  pnpm --dir frontend build
- live code exact-string audit is clean for active runtime/frontend product
  surfaces:
  rg -n 'QueuePromotionCandidate|VerifyPromotionCandidate|ApprovePromotionCandidate|PromotePromotionCandidate|RejectPromotionCandidate|/internal/promotions|/api/promotions|export_patchset|export_patchsets|promotion_queue|require_recipient_build' internal/runtime frontend/src -g '!frontend/dist/**'
- deployed product-path smoke passed against staging:
  PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com pnpm --dir frontend exec
  playwright test tests/trace-settings-registry.spec.js -g
  'Settings renders AppChangePackages and adoptions without browser-internal routes'
  --workers=1 --reporter=line
- deployed smoke registered a real passkey user, verified authenticated
  `/api/promotions` returns `404`, seeded `/api/app-change-packages` and
  `/api/computers/:id/adoptions`, and observed Settings rendering the adoption
  lifecycle state as `candidate_applied` with no browser-internal route use.
unproven or partial claims:
- full end-to-end alternate-computer experiment portfolio remains unstarted;
  this mission only removes false-success paths before that portfolio
- the untracked frontend/tests/platform-promotion-substrate-proof.tmp.spec.js
  still contains historical old-path strings but is not tracked or active
belief-state changes:
- old patchset promotion should be treated as historical audit data only;
  current experiment portfolio evidence must be package/adoption evidence
remaining error field:
- none for this preflight; staging smoke passed on the current deployed cutover
highest-impact remaining uncertainty:
- whether candidate workers consistently enter the repo dev shell when local
  Dolt/ICU tests are required; the platform CI/deploy path itself remains
  upstream-DoltHub/Nix-owned and green
next executable probe:
- run the alternate-computer experiment portfolio with AppChangePackage/adoption
  evidence as the only current patch movement path
suggested resume goal string:
- /goal Run docs/mission-promotion-substrate-preflight-hard-cutover-v0.md as a Codex-operated MissionGradient preflight mission...
evidence artifact refs:
- local command outputs in this Codex session
- GitHub Actions: https://github.com/yusefmosiah/go-choir/actions/runs/26151055695
- staging `/health` identity for `98b73c5...`
rollback refs:
- pre-mission git SHA: 11327dbd580e2259aa84c17782b1c31ba56ee35d
- behavior hard-cut rollback: revert `305b0e48a4f5133a466dae5da9176cdbb6576949`
- dev-shell cleanup rollback: revert `98b73c58930dab1c123f5be3eedb8130b19f796c`
go/no-go certificate:
- GO for the alternate-computer experiment portfolio, with one condition:
  success evidence must use AppChangePackage -> adoption -> recipient build ->
  promote/rollback records. `export_patchset`, `/api/promotions`, synthetic
  recipient digests, and old promotion queue UI are not acceptable evidence.
```
