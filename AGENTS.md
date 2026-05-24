# Choir Agent Operating Contract

This file is the repo-level contract for coding agents working on Choir.

## Default Environment

Staging is the acceptance environment: `https://draft.choir-ip.com`.

Use local development only for fast frontend visual iteration, focused unit shaping, or reproducing a staging failure after staging evidence identifies the transition that failed. Do not claim local proof for vmctl, live worker/candidate computers, gateway credentials, model/search calls, auth/session renewal, platform promotion, rollback, or Choir-in-Choir behavior.

Use the repo dev shell for Go, Dolt, and other native-dependency work:

```text
nix develop -c go test ...
nix develop -c go test ./internal/runtime ...
nix develop -c go build ./cmd/sandbox
```

The runtime package is intentionally broad and CI shards it. For local runtime
coverage, prefer the same sharded path instead of a full serial package run:

```text
nix develop -c scripts/go-test-runtime-shards
nix develop -c env SHARD_INDEX=0 TOTAL_SHARDS=4 scripts/go-test-runtime-shards
nix develop -c scripts/go-test-local
```

Use focused `go test ./internal/runtime -run TestName` while shaping one
transition. Avoid unbounded full serial `go test ./internal/runtime` runs unless
you are deliberately profiling the whole package. Local all-shard runs are
sequential by default because concurrent embedded-Dolt runtime shards contend on
developer machines; use `PARALLEL_SHARDS=1` only when you are deliberately
checking local shard concurrency.

Do not hand-enter local `CGO_CFLAGS`, `CGO_CXXFLAGS`, or `CGO_LDFLAGS` for the
Dolt ICU dependency except as a short diagnostic to confirm a missing-dev-shell
failure. The durable fix is that Codex, workers, candidate computers, and CI
enter the repo dev shell or an equivalent declared Nix environment before
running Go/Dolt tests and builds. If a worker/candidate environment cannot run
the dev shell, treat that as harness/runtime configuration debt to fix or
document precisely; do not normalize ad hoc host-specific include paths.

Browser proof is a specialized capability. Do not assume every worker or
candidate VM should carry Playwright/Chromium; that is a resource-heavy worker
class. Use it when product-path proof requires interactive screenshots, video,
or DOM metrics. Obscura-style extraction can be used for lightweight browsing
and scraping only after its auth, action, screenshot, video, and extraction
capabilities are explicitly verified for the task at hand.

Read [docs/computer-ontology.md](docs/computer-ontology.md) before changing VM, sandbox, candidate-world, promotion, package, or persistent-state behavior. The product object is a persistent user computer. `sandbox` is an implementation/service name, not the product ontology.

## Problem Documentation First

Every platform behavior-changing mission must observe the following invariant:

> **Documenting a problem is the first priority. Fixing it is second.**

When staging evidence (or any reliable evidence) reveals a new problem, the
first commit that follows must be a checkpoint or mission doc update that names
the problem, records the evidence, and updates the belief state and remaining
error field — without any code fix. The fix commit(s) come second, referencing
the prior documentation.

This ensures that:

- Problems can be reviewed independently of any particular solution.
- Alternative solutions can be considered before committing to an approach.
- Refactoring and re-evaluation can happen after mission pressure passes.
- Other agents and humans can examine the problem record to form their own
  judgment about the right fix.

Context-dependent fixes authored during a mission ("get past this blocker") are
especially susceptible to narrowing the solution space prematurely. A separate
documentation step creates a natural review gate.

Exceptions require explicit justification in the commit message, naming why the
problem could not be documented before being fixed.

See [docs/memo-problem-documentation-first.md](docs/memo-problem-documentation-first.md)
for the finding that motivated this invariant.

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

Read [docs/missiongradient-method.md](docs/missiongradient-method.md) before authoring or executing broad MissionGradient runs.

Do not turn MissionGradient into a brittle checklist. Preserve the invariant and increase realism as evidence arrives.

For long-running Choir-in-Choir missions, maintain an owner-readable VText
narrative. Each substantive change in plan, evidence, blocker, or result should
produce a concise revision that explains the whole run state so far in plain
language: objective, past work, current work, what changed, evidence, learnings,
risks, and next step. Do not make VText a Trace-like topology/status table, and
do not dump low-level events into VText. Trace is the causal ledger for dense
tool calls, LLM content, and agent-to-agent messages; feature-specific live
surfaces such as Chyron may show granular activity streams; VText is the human
supervision narrative.

## Authority Boundaries

- `conductor` routes exogenous user/app/connector input. It is not the semantic babysitter.
- Appagents own durable app artifacts. `vtext` owns canonical document versions.
- `researcher` writes structured findings/evidence, not canonical text or code.
- `super` is the foreground orchestration root. It can request workers and candidate worlds.
- `vsuper` owns a background/candidate computer or candidate world.
- `cosuper` is subordinate to the super/vsuper that leased it.
- Verification is a contract over evidence, not a separate privileged caste.

Foreground/canonical state stays stable. Background/candidate computers mutate. Canonical state changes only by promotion.

Prefer asynchronous supervision. A delegation, worker VM run, candidate preview,
or verification job should leave durable status/evidence and return a handle
rather than blocking the foreground supervisor until completion. If a required
tool is blocking, treat that as runtime debt to repair or document precisely.

Avoid skip-level authority confusion. If `super` needs to address a `cosuper`,
the owning `vsuper` must receive the same instruction or remain the forwarding
authority. A subordinate should not have to reconcile competing directives from
two supervisors.

Verifier agents may be read-only with respect to product/canonical state, but
they are not necessarily computation-only observers. They may run commands,
write temporary scripts, or create tests inside an authorized scratch or
candidate environment when that is required to verify behavior.

## Runtime Configuration

Provider secrets and platform model catalogs are platform-owned. Per-computer
model policy is computer-owned durable state and should be editable through the
product path, including by `super` in response to an owner prompt. Do not patch
Node B environment variables or tracked server files as a substitute for a
runtime policy path unless the mission is explicitly a platform config deploy.

Role defaults are policy defaults, not architecture. Any configured model may
serve any agent role when its declared capabilities match the current turn:
conductor, VText, researcher, super, vsuper, co-super, verifier, or future
roles. Text-only models are valid for orchestration, research, coding, writing,
and verification that does not need media input. Multimodal models are required
only when the turn needs screenshots, images, video frames, files, or other
media inputs. If a current policy maps a role to ChatGPT or Fireworks, treat
that as the active computer's effective policy, not a hard-coded role boundary.
Do not add new role-specific provider assumptions such as "super must be
ChatGPT" or "verifier must be multimodal" unless the current turn's capability
requirements actually imply that. The long-term target is dynamic, agentically
editable per-computer model policy: an owner prompt may ask `super` to edit the
computer's model policy, and subsequent runs should use that policy without a
platform deploy or Node B environment edit.

Provider request schemas must preserve modality. If a task needs screenshots,
videos, files, or other media evidence, route through a model/provider path that
declares that modality and record the blocker precisely when the adapter cannot
resolve the artifact.

## Product-Path Verification

Browser or Playwright acceptance may use public authenticated product APIs such as:

- `/api/prompt-bar`
- `/api/prompt-bar/submissions/{id}`
- `/api/vtext/*`
- `/api/trace/*`
- `/api/app-change-packages/*`
- `/api/computers/*/source-lineage`
- `/api/computers/*/adoptions`
- `/api/adoptions/*`
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

Required evidence should include trajectory/run ids, authority profile, build/deploy identity, vmctl worker lease, AppChangePackage/adoption evidence or a precise blocker, verifier contracts, rollback refs, and residual risks. Use explicit levels: `docs-level`, `staging-smoke-level`, `export-level`, `promotion-level`, `continuation-level`.

Do not claim `promotion-level` without AppChangePackage adoption verifier contract evidence plus owner review and promote/rollback evidence. Do not claim `continuation-level` without run-memory/compaction and continuation evidence.

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
