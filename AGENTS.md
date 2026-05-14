# Choir Agent Operating Contract

This file is the repo-level contract for coding agents working on Choir.

## Default Environment

Staging is the acceptance environment: `https://draft.choir-ip.com`.

Use local development only for fast frontend visual iteration, focused unit shaping, or reproducing a staging failure after staging evidence identifies the transition that failed. Do not claim local proof for vmctl, live worker/candidate computers, gateway credentials, model/search calls, auth/session renewal, platform promotion, rollback, or Choir-in-Choir behavior.

Read [docs/computer-ontology.md](docs/computer-ontology.md) before changing VM, sandbox, candidate-world, promotion, package, or persistent-state behavior. The product object is a persistent user computer. `sandbox` is an implementation/service name, not the product ontology.

## Landing Loop

Every platform behavior-changing mission includes:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

Do not stop at local tests or an unpushed commit when platform behavior changed.

Personal-computer changes are different. Choir should eventually allow a user to
fork their own computer, build or install local apps/runtime changes, and promote
that candidate into their own active computer without a global platform deploy.
That path still needs lineage, typed deltas, verifier evidence, route rollback,
and no lost foreground updates.

Docs-only commits are different. The CI workflow intentionally ignores `docs/**` and top-level `*.md`. Do not weaken those path filters to force docs-only CI. If documentation needs validation, run the specific check directly or use a manual workflow dispatch when one exists.

## MissionGradient

For multi-hour, overnight, staging, self-development, or broad architectural work, use MissionGradient: define the real artifact, invariants, value criterion, homotopy axes, dense feedback, forbidden shortcuts, rollback, learning side-channel, and stopping condition.

Do not turn MissionGradient into a brittle checklist. Preserve the invariant and increase realism as evidence arrives.

## Authority Boundaries

- `conductor` routes exogenous user/app/connector input. It is not the semantic babysitter.
- Appagents own durable app artifacts. `vtext` owns canonical document versions.
- `researcher` writes structured findings/evidence, not canonical text or code.
- `super` is the foreground orchestration root. It can request workers and candidate worlds.
- `vsuper` owns a background/candidate computer or candidate world.
- `cosuper` is subordinate to the super/vsuper that leased it.
- Verification is a contract over evidence, not a separate privileged caste.

Foreground/canonical state stays stable. Background/candidate computers mutate. Canonical state changes only by promotion.

## Product-Path Verification

Browser or Playwright acceptance may use public authenticated product APIs such as:

- `/api/prompt-bar`
- `/api/prompt-bar/submissions/{id}`
- `/api/vtext/*`
- `/api/trace/*`
- `/api/promotions/*`
- `/api/continuations/*`
- `/api/run-acceptances/*`

Do not use browser-public internal or test-only routes to bypass the product path:

- `/api/agent/*`
- `/api/prompts`
- `/api/test/*`
- `/internal/*`
- raw event mutation endpoints

The verifier must observe product/control evidence. It must not manually seed success records.

## Run Acceptance Records

For long-running self-development proof, synthesize a durable `RunAcceptanceRecord` from existing evidence:

```text
POST /api/run-acceptances/synthesize
```

Required evidence should include trajectory/run ids, authority profile, build/deploy identity, vmctl worker lease, worker export or promotion candidate, verifier contracts, rollback refs, and residual risks. Use explicit levels: `docs-level`, `staging-smoke-level`, `export-level`, `promotion-level`, `continuation-level`.

Do not claim `promotion-level` without verifier contract evidence plus owner review and promotion/rollback evidence. Do not claim `continuation-level` without run-memory/compaction and continuation evidence.

## Git And Staging

GitHub `origin/main` is the source of truth for tracked deployed files. Do not edit tracked files directly on Node B as a source/config shortcut.

If a behavior-changing commit is pushed:

1. Monitor the GitHub Actions run for that SHA.
2. Confirm Node B deploy/health reports that SHA or deployed commit.
3. Run the relevant deployed Playwright/API acceptance proof against `draft.choir-ip.com`.
4. Record evidence in the final report.

## Safety

Assume the worktree may contain user or other-agent changes. Do not revert unrelated changes. Avoid destructive commands unless the user explicitly asked for them.

Use background/candidate computers or candidate worlds for risky mutation. Failed candidates should leave diagnostics, rollback refs, and next safe probes.

## Final Evidence

A final report for behavior-changing missions should name:

- pushed commit SHA;
- CI run and deploy status;
- staging health/build identity;
- deployed acceptance command and result;
- accepted trajectory/run/acceptance ids;
- acceptance level reached;
- verifier contracts and evidence refs;
- rollback refs;
- residual risks and the next realism axis.
