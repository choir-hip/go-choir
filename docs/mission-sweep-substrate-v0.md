# MissionGradient: Sweep Substrate v0

Status: active
Date: 2026-05-15

## Codex Goal String

Use MissionGradient. Execute `docs/mission-sweep-substrate-v0.md`: as outer Codex, harden Choir's staging sweep substrate, land and deploy required platform fixes, then use Playwright to prompt Choir through `https://draft.choir-ip.com` and prove one bounded inner Choir sweep with VText, Trace, vsuper, worker/verifier cosupers, channel iteration, candidate/export evidence, rollback refs, cognitive-transform blocker analysis, a post-correctness quality pass, and residual risks.

## Real Artifact

The real artifact is not a slash-command mode, a local patch, or a one-off Playwright script. It is a staging-proven training-wheels loop:

```text
Codex outer mission
-> repo/platform fixes when substrate is missing
-> commit/push/CI/deploy/staging identity
-> Playwright drives Choir through the visible product prompt bar
-> VText owns the durable sweep artifact
-> super leases a separate vsuper/candidate computer
-> vsuper holds MissionGradient, MissionBag, selection policy, and belief state
-> worker cosuper produces candidate work
-> verifier cosuper independently verifies and messages failures back
-> worker/verifier iterate over agent channels
-> vsuper meta-verifies and reports evidence/export/rollback refs
-> VText records the sweep report
```

Codex is currently the bootstrap operator. The mission should reduce Codex's role over time, but not pretend Choir is self-sufficient before the staging evidence proves it.

## Invariants

- Choir prompt bar remains natural language. Do not add slash commands, `/goal`, or goal-mode semantics to Choir.
- Staging is the acceptance environment for VM, worker, gateway, auth, promotion, and Choir-in-Choir claims.
- Platform behavior-changing work follows: outer Codex commits -> push origin/main -> monitor CI -> monitor staging deploy -> verify staging commit identity -> run deployed acceptance proof.
- Inner Choir/user-computer candidate work may build or install candidate Go/Svelte/runtime changes inside that user's candidate computer, but it must not deploy or mutate global staging infrastructure.
- Browser automation may use only the visible product surface and authenticated product APIs such as `/api/prompt-bar`, `/api/vtext/*`, `/api/trace/*`, `/api/promotions/*`, `/api/continuations/*`, and `/api/run-acceptances/*`.
- Do not use `/internal/*`, `/api/agent/*`, `/api/prompts`, `/api/test/*`, raw event mutation endpoints, or manually seeded success records.
- Foreground/canonical state stays stable. Candidate/worker computers mutate. Canonical state changes only through verified promotion and owner review.
- `super` delegates candidate-world orchestration to `vsuper`; `vsuper` delegates local work and verification to `cosuper` roles.
- Worker and verifier cosupers communicate over agent-to-agent channels until verification passes or a real blocker is recorded.
- Workers do not judge their own work as final. Verifiers produce evidence. Vsuper meta-verifies the verification process.
- Failed candidates leave diagnostics, trace refs, rollback/export refs where available, and the next safe probe.

## Value Criterion

Maximize verified sweep capability per unit of human attention while minimizing:

- fake progress through local-only proof or internal routes;
- unclear authority boundaries between super, vsuper, worker cosuper, and verifier cosuper;
- unverified canonical mutation;
- hidden worker state or missing repo checkout access;
- duplicate worker runs that obscure evidence;
- long-running pending states without progress or timeout evidence;
- prompt-bar command syntax that turns Choir into a command runner instead of a natural-language multiagent system.

The system is better when a natural-language prompt can start a bounded sweep, Choir can decompose and verify work through its own agent topology, and Codex only steps in to repair substrate gaps.

## Starting Belief State

Current evidence:

- Staging `https://draft.choir-ip.com` was deployed at commit `a0e8c4e4d0f4953db5edc6f4d53bfa50b4e83b2d` during the first sweep probes.
- Prompt-bar product path can create VText documents and Trace trajectories.
- A bounded sweep probe produced VText doc `54bab4cf-dedd-4511-a0d7-8326b6632db2`, appagent revision `6dad37b4-e656-4bb8-9512-9725db7273df`, and trajectory `21bd8510-c451-41f4-8df9-5d32d874f1e4`.
- The probe showed conductor, VText, researchers, super, and co-super worker activity with no forbidden browser-public routes.

Main uncertainties and known blockers:

- Worker/candidate VM environments did not have a reachable `go-choir` checkout or `.git` directory, so they could not inspect repo state or export a meaningful patchset.
- `request_worker_vm` attempted unsupported `machine_class="standard"` before falling back to supported worker classes.
- The observed delegation topology was too direct: super delegated worker work, but the desired control topology is super -> vsuper -> worker/verifier cosupers.
- A corrected-host probe against `draft.choir-ip.com` could remain pending long enough that the outer observer had to stop it, exposing weak progress/timeout evidence.
- Choir skill support for repo skills such as `mission-gradient` and `cognitive-transform-portfolio` is not yet proven as first-class runtime context.

Highest-impact uncertainty:

Can a staging Choir run create a separate candidate-world vsuper that has repo-aware worker substrate, can coordinate worker and verifier cosupers by channel messages, can iterate until verifier pass/fail, and can export evidence without canonical mutation?

Next observation that reduces uncertainty:

A deployed product-path proof where a prompt-bar request produces Trace evidence of super -> vsuper -> worker cosuper plus verifier cosuper, at least one verifier-to-worker failure or pass message, and either an exported candidate patchset or a precise repo-access blocker.

## Homotopy

Increase realism continuously, preserving topology:

1. **Docs-level framing:** keep this mission and `docs/choir-agentic-depth-canonical.md` aligned with actual runtime behavior.
2. **Gated runtime proof:** add or adjust tests for role permissions, vsuper delegation, worker/verifier channel iteration, repo checkout availability, and timeout/progress reporting.
3. **Local engineering proof only where appropriate:** use focused local tests for code correctness, not as proof of staging VM behavior.
4. **Deployed substrate proof:** land required platform fixes, verify staging commit identity, and run product-path Playwright against `draft.choir-ip.com`.
5. **Inner Choir sweep proof:** use Playwright to type the natural-language inner prompt into Choir, then inspect VText, Trace, worker/verifier evidence, candidate/export refs, and residual risks.
6. **First product sweep:** once substrate works, sweep first-use UX and onboarding. If safe, include exploratory items such as podcast search/index behavior and native skill use.

## MissionBag Seed

The outer Codex mission should sweep these items by expected value and dependency, not as a rigid checklist:

| Item | Local objective | Done evidence |
|---|---|---|
| MB-1 | Make worker/candidate environments repo-aware | Worker/vsuper can see a `go-choir` checkout, base SHA, branch/worktree, and export target without mutating foreground canonical state |
| MB-2 | Correct worker machine-class guidance | No live probe attempts unsupported `standard`; prompts/tools/tests steer to supported worker classes |
| MB-3 | Make vsuper the candidate-world orchestrator | Trace shows super delegates to vsuper, and vsuper owns worker/verifier cosuper routing inside the candidate boundary |
| MB-4 | Add worker/verifier cosuper iteration | Trace/channel evidence shows verifier failure or pass messages sent to worker cosuper, with at least one bounded repair loop or explicit blocker |
| MB-5 | Make progress and timeout states observable | Long-running sweep attempts expose actionable progress, pending reason, timeout, or blocker evidence in VText/Trace |
| MB-6 | Add native skill-context support | Choir agents can use repo-owned `skills/mission-gradient` and `skills/cognitive-transform-portfolio` as runtime guidance/artifacts without slash commands or prompt-bar command syntax |
| MB-7 | Run first UX/onboarding sweep | Inner Choir VText report covers login/register/onboarding, initial explanatory VText, prompt bar ergonomics, window crowding, desktop polish, acceptance evidence, and residual risks |
| MB-8 | Add exploratory/speculative sweep items | MissionBag includes podcast app search/index improvement, skill-native behavior, and other high-information probes without derailing substrate proof |

## Inner Choir Prompt

Codex should submit this through Playwright to the visible staging prompt bar only after the substrate fixes are deployed:

```text
Run a bounded first sweep control interval using MissionGradient, Cognitive Transform Portfolio where useful, MissionBag, and Sweep geometry. Focus on first-use UX for https://draft.choir-ip.com: login/register/onboarding, initial VText explanation of Choir and VText, prompt bar ergonomics, window crowding, desktop aesthetics, and exploratory/speculative improvements such as podcast search/index behavior and native skill use. VText owns the durable artifact. Super should lease a separate vsuper/candidate computer. Vsuper should orchestrate worker and verifier cosupers over agent-to-agent channels until verification passes or a real blocker is recorded. Record a MissionBag, belief state, selection policy, evidence ledger, worker/verifier messages, candidate/export/rollback refs where applicable, residual risks, and the next sweep item. Do not use slash commands, local-only proof, forbidden internal/test routes, manual success seeding, or canonical mutation without verification/promotion.
```

## Dense Feedback

- `git status`, `git log`, GitHub Actions run status, and staging `/health` commit identity.
- Focused Go tests for role permissions, worker VM requests, delegate worker VM behavior, run events, channel messages, export patchsets, and run acceptance synthesis.
- Frontend build and focused Playwright only for touched product surfaces.
- Deployed Playwright proof against `https://draft.choir-ip.com` through visible auth and prompt-bar flow.
- Trace assertions for conductor, VText, super, vsuper, worker cosuper, verifier cosuper, channel messages, verifier failures/passes, export patchset, and VText revision creation.
- VText evidence that the inner sweep report was created by `edit_vtext`, not merely final model text.
- Acceptance evidence naming submission id, doc id, trajectory id, worker id, VM id, candidate/export refs, rollback refs, CI run, deploy identity, and residual risks.

## Rollback

- Global staging changes come only from outer Codex landing tracked repo changes through GitHub `origin/main`, CI, and the existing deploy path. Do not create a Choir-internal route that pushes, deploys, or changes platform staging directly.
- Keep platform repo work on normal git history with clear commits and no unrelated reversions.
- Before platform behavior-changing commits, record base SHA and touched surfaces.
- Candidate/worker changes must stay inside candidate/worktree/VM boundaries until exported and verified.
- A user computer may build or install a candidate Go binary, frontend, skill, app, prompt, or runtime package inside that user's own candidate computer. That path needs lineage, typed deltas, verifier evidence, route rollback, and owner review, but it must not affect global staging for other users.
- Promotion remains serialized and evidence-backed. This mission may produce export-level proof before promotion-level proof.
- If an outer-Codex platform commit passes CI but deployed staging evidence reveals an integration/config/runtime regression that CI missed, recover by a normal follow-up fix or git revert through `origin/main`, then redeploy and record the evidence. CI is required, but it is not treated as complete proof of staging behavior.

## Forbidden Shortcuts

- Do not add slash commands or `/goal` parsing to Choir prompt bar.
- Do not ask the user to manually prompt Choir for the inner sweep. Codex must do that through Playwright.
- Do not claim the inner sweep worked from a local app or from Codex-only implementation.
- Do not collapse vsuper into super or cosuper when the candidate boundary matters.
- Do not accept a worker self-report as verification.
- Do not ignore duplicate worker/delegation behavior if it affects evidence clarity.
- Do not call a selected continuation, queued candidate, or VText report a promoted result.
- Do not broaden into podcast/radio or speculative work before substrate proof is stable enough to keep evidence coherent.

## Learning Side-Channel

Durable learnings should be written back into:

- this mission doc when the mission route changes;
- `docs/choir-agentic-depth-canonical.md` when sweep/fly/cycle vocabulary changes;
- runtime or architecture docs when authority boundaries change;
- focused tests when a bug becomes an invariant;
- final evidence docs when staging proof or blockers are found.

Classify surprises:

- Tactical: adjust implementation or next probe inside this mission.
- Target-level: update the MissionBag or inner prompt before rerunning.
- Invariant-level: stop and ask before changing prompt-bar semantics, authority boundaries, canonical mutation rules, or verification meaning.

Before stopping on a negative result, apply the Cognitive Transform Portfolio. Select two to five transforms that could change the route, verifier, scope, or next probe. At minimum, use one depth transform to identify the load-bearing hidden truth and one lateral/audience transform to find a different route through the blocker. Record whether the transforms changed the next safe probe.

## Stopping Condition

Stop successfully when:

- required platform fixes are committed, pushed, green in CI, deployed to staging, and staging health reports the expected commit;
- Playwright submits the inner Choir prompt through the real prompt bar;
- Trace shows the intended super -> vsuper -> worker/verifier cosuper topology or a documented invariant-level blocker;
- worker/verifier channel iteration is observed or the missing substrate is precisely recorded;
- VText stores a final sweep report revision via `edit_vtext`;
- evidence names submission id, doc id, trajectory id, agents, worker/candidate ids, export/rollback refs if any, and residual risks.
- a quality pass has been completed after first correctness: simplify the route, remove duplicate or confusing paths, strengthen verifiers, improve Trace/VText evidence clarity, update docs/tests, and state remaining quality debt.

Stop on blocker only after cognitive transforms fail to produce a safe route around the negative result. The blocker report must name the failing boundary, transforms attempted, evidence refs, and the next safe probe.
