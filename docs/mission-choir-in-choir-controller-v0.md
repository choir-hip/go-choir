# MissionGradient: Choir Self-Development Controller v0

Status: proposed next 8-24h mission
Date: 2026-05-13

## Real Artifact

Choir self-development controller: the durable control layer that turns run memory, mission docs, promotion queue state, verifier results, product gaps, and user constraints into the next bounded run, then drives candidate-world work through verification and promotion without Codex manually selecting the next goal.

## Invariants

- Foreground/canonical state changes only by verified promotion.
- Candidate work mutates only in background VM or integration-branch boundaries.
- Every continuation has objective, reason, authority profile, lease, source run, and stop condition.
- Every promoted product change has candidate identity, verifier contract, rollback point, and traceable source run.
- `vsuper` owns candidate-world mutation, not promotion.
- Researchers/appagents may influence objectives through durable records, not terminal mutation.
- Automatic continuation must stop with a review packet if no bounded safe next objective exists.

## Value Criterion

Maximize verified Choir self-improvement per human review minute while minimizing hidden state, verifier bypass, authority leakage, canonical corruption risk, and context loss.

The controller is better when it selects the same next high-value objective a careful operator would select, writes why, runs it in the right authority boundary, and leaves enough evidence to promote or stop.

## Homotopy

Low resolution:

- deterministic selector over a mission doc, promotion queue, and failed verifier records;
- metadata-driven continuation remains available as an escape hatch;
- one product slice is chosen from launcher/uploads/themes/files.

Medium resolution:

- super creates continuation proposals from run memory and queue state;
- `vsuper` executes a candidate in a worker VM;
- verifier contracts are attached before promotion;
- Playwright proves the product path.

High resolution:

- Choir uses the desktop prompt/product path to request the change;
- background VM candidate work exports patchsets;
- promotion queue is visible in the product;
- after promotion, the controller selects and starts the next bounded candidate run.

## Dense Feedback

- Store tests for continuation proposal selection and promotion queue state transitions.
- Runtime tests for automatic continuation after verified promotion.
- `delegate_worker_vm` test proving worker export queues a candidate.
- Playwright prompt-bar/bootstrap test for the first Choir-driven loop.
- Frontend tests for app launcher, file upload UI/API, and theme editor validation.
- Git assertions for base SHA, integration branch, promoted commit, and divergence block.

## Forbidden Shortcuts

- Do not hard-code a single next goal and call it synthesis.
- Do not bypass the prompt/product path when claiming Choir-in-Choir proof.
- Do not promote candidate patches without verifier contracts.
- Do not make theme presets a closed list instead of examples of user-editable themes.
- Do not let automatic continuation start unbounded super runs.

## Rollback Policy

- Every candidate records base SHA and worker head.
- Every integration branch is disposable until verified.
- Every continuation is bounded by profile and lease.
- Failed candidates remain as queue/report evidence, not silent deletions.
- UI/API migrations are additive and reversible by git rollback.

## Learning Side-Channel

Write the run result to a dogfood report and update this mission with:

- selected objective and why;
- verifier contracts used;
- failed candidates and recovery route;
- next objective the controller would select after promotion.

## Stopping Condition

Stop when Choir can select, start, verify, promote, and continue one real launcher/uploads/themes/files product slice through the product path and candidate-world promotion queue, or when the blocked invariant and next smallest safe probe are documented.

## One-Line Goal

`/goal Use MissionGradient to execute docs/mission-choir-in-choir-controller-v0.md, turning run memory plus promotion queue into a self-development controller that selects, verifies, promotes, and continues one real launcher/uploads/themes/files product slice through the Choir product path.`
