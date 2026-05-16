# MissionGradient: Trace Candidate Promotion + Continuation v0

Status: ready for execution
Date: 2026-05-16
Operator: outer Codex supervising Choir through staging, git, CI, deploy, Playwright, Trace, VText, promotion, continuation, and worker-VM evidence

## One-Line Goal String

```text
/goal Run docs/mission-trace-candidate-promotion-continuation-v0.md as a Codex-operated MissionGradient mission: review candidate d7f70d7a-5776-4fac-85a0-26f0d5af93ce from trajectory d6a51c7d-af23-4de5-b00a-0bdd791e46fe, decide whether to promote or revise the exported Trace mobile_summary patch, eliminate duplicate child spawn/export behavior, add continuation-level compaction/run-memory evidence for worker/vsuper runs, land required fixes through git/CI/deploy, and rerun visible staging prompt-bar proof until Trace, VText, run acceptance, screenshots/DOM metrics, rollback refs, and promotion or precise blocker evidence are durable and readable on desktop and mobile.
```

## Mission Shift

The previous mission proved that the worker-vsuper substrate can now reach
`export-level` from the visible staging prompt bar. It also landed a direct
platform fix for mobile Trace acceptance wrapping. The next mission is to turn
that export-level capacity into owner-reviewable promotion and continuation
evidence.

The queued candidate is not automatically trusted. It must be reviewed as a
promotion artifact, revised or rejected if it is weaker than the deployed
platform patch, and promoted only with verifier and rollback evidence.

## Current Belief State

Deployed platform baseline:

- commit: `30456411a4dcc6b89ca34f44023fd2c4620b9c6a`
- staging: `https://draft.choir-ip.com`
- latest CI/deploy proof: GitHub Actions run `25974879206`, successful
- staging health: proxy and sandbox both reported commit
  `30456411a4dcc6b89ca34f44023fd2c4620b9c6a`

Latest product-path proof:

- trajectory: `d6a51c7d-af23-4de5-b00a-0bdd791e46fe`
- VText doc/channel: `164142d6-672b-4451-85a4-d9427367e4e6`
- run acceptance: `runacc-013800bf09e2aee18c02`, accepted, `export-level`
- worker: `worker-71b4fa13bd4f4d4e`
- worker VM: `vm-b516fd9c8868d59e1e99735c1d6a22df`
- root vsuper loop: `581d257d-e455-42cc-ae9e-73a22b27e47b`
- implementation co-super loop: `c8fcf787-d347-43d8-8dd5-eeda7730d3af`
- verifier co-super loop: `91ea61a6-92c2-4a62-bc32-92e527abf955`
- duplicate/cancelled child loop: `2770e9f5-4dd0-454f-afd0-6638b39405fc`
- exported worker head: `b68d638a5fedd2fb95fe26472ba18df2797a4ad0`
- queued candidate: `d7f70d7a-5776-4fac-85a0-26f0d5af93ce`
- patchset SHA: `01668080c581898fa70c088525c0fe80df283c14ec3ad2c2719e36ccaf60c291`
- promotion artifact manifest:
  `/mnt/persistent/promotion-artifacts/d7f70d7a-5776-4fac-85a0-26f0d5af93ce/manifest.json`
- promotion artifact patch:
  `/mnt/persistent/promotion-artifacts/d7f70d7a-5776-4fac-85a0-26f0d5af93ce/changes.patch`

Observed state:

- `delegate_worker_vm` now follows child runs after root vsuper completion.
- VText and run acceptance preserve worker state, child run IDs, channel
  messages, export patchsets, promotion queue refs, and rollback refs.
- Trace acceptance is readable on desktop and mobile after deployed commit
  `3045641`; mobile `390x844` proof recorded 8 checkpoints, 5 evidence refs,
  no overflow nodes, and no tiny text nodes.
- The exported candidate adds an API-level `mobile_summary` to Trace trajectory
  snapshots, but that candidate is not promoted or owner-reviewed.
- Continuation-level proof is absent.
- The worker path still duplicated implementation/export attempts before
  converging, which is acceptable evidence only if preserved as a residual risk,
  not if normalized as healthy behavior.

Highest-impact uncertainties:

- Does candidate `d7f70d7a...` improve over the already-landed mobile Trace UI
  wrapping, or is it redundant/too narrow?
- Can the promotion API expose enough artifact content and verifier evidence
  for owner review through the product path?
- Why did vsuper spawn/cancel duplicate implementation children and duplicate
  export the same patchset?
- What exact compaction/run-memory/continuation record is required before
  claiming `continuation-level`?

## Real Artifact

The artifact is a deployed, reviewable Choir self-development path:

```text
visible staging prompt bar
-> VText mission/report
-> conductor -> super
-> worker VM vsuper
-> implementation and verifier co-super channels
-> export patchset and promotion candidate
-> owner-readable Trace/VText/promotion review
-> optional platform or personal promotion
-> continuation/run-memory proof
-> rollback refs
```

The UX improvement is not a separate toy task. It is the proving workload for
the super -> worker VM -> vsuper -> co-super -> export/promotion path.

## Invariants

- Staging `https://draft.choir-ip.com` is the acceptance environment for
  platform behavior, worker VM, auth, gateway/model calls, Trace, VText,
  promotion, rollback, and run acceptance claims.
- Behavior-changing platform work completes:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

- Browser/public acceptance uses visible product surfaces and public
  authenticated product APIs only: `/api/prompt-bar`,
  `/api/prompt-bar/submissions/*`, `/api/vtext/*`, `/api/trace/*`,
  `/api/promotions/*`, `/api/continuations/*`, and
  `/api/run-acceptances/*`.
- Do not use browser-public internal/test success paths: `/api/agent/*`,
  `/api/prompts`, `/api/test/*`, `/internal/*`, raw event mutation endpoints,
  direct service ports, or manually seeded success records.
- Internal diagnostics may inspect local code, vmctl/worker state, worker logs,
  GitHub Actions, and artifact paths. Diagnostics guide fixes but do not count
  as product acceptance.
- Promotion changes reality. Do not claim `promotion-level` without review,
  verifier evidence, promotion or discard decision, and rollback refs.
- Do not claim `continuation-level` without run-memory/compaction and
  continuation evidence.
- Preserve logged-out read/explore usability and auth-on-mutation clarity.
- No fake-island placeholders, fake transclusion panels, fake candidate refs,
  fake verifier transcripts, or summaries that launder missing evidence into
  success.

## Value Criterion

Minimize:

```text
promotion ambiguity + continuation evidence gaps + duplicate worker churn
+ Trace/VText readability friction + hidden state + verifier Goodharting
```

while preserving authority boundaries, rollback, and product-path proof.

The mission moves uphill when:

- candidate artifact content is reviewable through durable refs;
- redundant or unsafe candidate deltas are rejected or revised rather than
  promoted mechanically;
- duplicate child spawn/export behavior is root-caused and reduced or precisely
  blocked;
- continuation evidence is synthesized from durable run memory rather than
  invented after the fact;
- Trace/VText/run acceptance make full provenance readable on desktop and
  mobile;
- every platform behavior change lands through git/CI/deploy and deployed
  proof.

The mission moves downhill when:

- export-level evidence is described as promotion-level;
- the candidate is promoted without owner/verifier confidence;
- a local-only script result is treated as staging proof;
- duplicate child/export behavior is ignored because the final export succeeded;
- continuation is claimed from a summary string without durable continuation
  state.

## Receding-Horizon Control

1. Inspect the current candidate, promotion API, Trace snapshot, VText report,
   run acceptance, and continuation API state.
2. Decide whether candidate `d7f70d7a...` should be promoted, revised, or
   rejected as redundant.
3. Root-cause duplicate spawn/export behavior using worker event evidence and
   prompt/tool contracts; patch prompt contracts, tool behavior, run acceptance,
   or diagnostics if implicated.
4. Add or repair continuation-level evidence paths for worker/vsuper runs.
5. Land platform changes through main, CI, deploy, and staging identity proof.
6. Rerun a visible staging prompt-bar workload through the same topology.
7. Stop only with promotion-level or continuation-level evidence, or with a
   hard blocker after named root-cause probes and at least one changed search
   strategy.

## Dense Feedback

- Local: targeted Go tests for runtime/Trace/promotion/continuation behavior;
  frontend build and DOM metric scripts for Trace readability.
- CI: GitHub Actions run for pushed SHA.
- Deploy: `/health` build identity for proxy and sandbox.
- Product proof: Playwright against `https://draft.choir-ip.com` using visible
  prompt bar and public product APIs.
- Evidence artifacts: VText export, Trace snapshot, run acceptance record,
  promotion candidate/review record, continuation/run-memory record, desktop
  and mobile screenshots/DOM metrics.

## Rollback Policy

- Platform rollback: revert pushed commits or deploy previous known-good SHA.
- Candidate rollback: discard/archive candidate
  `d7f70d7a-5776-4fac-85a0-26f0d5af93ce`.
- Promotion rollback: use recorded promotion rollback refs and prior route/base
  pointers.
- If continuation synthesis is wrong, do not delete evidence; append corrected
  continuation/run-memory records and mark the old acceptance as superseded or
  blocked.

## Stopping Condition

Stop when one of the following is true:

- `promotion-level`: candidate review and verifier evidence support promotion
  or explicit discard, promotion/rollback refs are durable, Trace/VText show the
  decision on desktop and mobile, and staging proof is attached.
- `continuation-level`: run-memory/compaction and continuation evidence is
  durable for the worker/vsuper trajectory, run acceptance records it honestly,
  and Trace/VText expose it readably on desktop and mobile.
- `hard blocker`: after root-cause probes and cognitive search-space transforms,
  an external/invariant blocker remains, with exact evidence, rollback refs,
  residual risks, and the next safe probe.
