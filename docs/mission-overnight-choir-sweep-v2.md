# MissionGradient: Overnight Choir-In-Choir Sweep v2

Status: ready for overnight execution
Date: 2026-05-16
Operator: outer Codex supervising Choir through staging Playwright

## One-Line Goal String

```text
/goal Run docs/mission-overnight-choir-sweep-v2.md as a Codex-operated MissionGradient mission: first finish landing and staging verification for the super/vsuper prompt-boundary fix, then use the visible staging prompt bar to run a sweep-shaped Choir self-development workload that tests whether super delegates Choir/app/harness/repo/candidate/promotion work to worker-VM vsuper, whether vsuper coordinates worker and verifier co-super agents over channels, and whether VText, Trace, candidate/export/promotion or precise blocker evidence, run acceptance, rollback refs, residual risks, and next objective are durable; keep UX/onboarding work as the test workload after substrate proof, preserve logged-out read/explore usability, improve or precisely block Trace readability, and forbid fake-island placeholders.
```

## Real Artifact

The artifact is one deployed, evidence-backed Choir-in-Choir control interval
that improves the substrate while using a real product workload as pressure:

```text
outer Codex
-> git / CI / staging deploy for platform prompt or substrate fixes
-> staging Playwright at https://draft.choir-ip.com
-> visible natural-language prompt bar
-> VText mission report
-> Trace-visible conductor -> vtext -> super
-> super delegates durable/risky Choir work to worker VM -> vsuper
-> vsuper coordinates worker co-super + verifier co-super over channels
-> candidate/export/promotion or precise blocker evidence
-> run-acceptance synthesis
-> quality/code-review pass
-> residual risks and next objective
```

The visible UX/onboarding sweep is the workload used to test the substrate. It
is not a reason to skip substrate proof.

## Starting State

Previous v1 control interval submitted the mission prompt through the visible
staging prompt bar after auth-on-mutation.

Evidence:

- staging health before the run: proxy and sandbox deployed commit
  `d8185cc9b2f4c9eb8b765a891ef267f0160bd54c`;
- submission / trajectory id: `b50b9d07-0a03-4168-b392-dfa60eb7535a`;
- VText doc id: `b8a3561a-a406-4fb9-9934-602f258faa36`;
- framing revision id: `69e9fdcf-f61e-4767-8011-ac06f0f80dbe`;
- observed roles: `conductor`, `vtext`, `super`;
- observed VText revisions: 3;
- observed `submit_worker_update` results: 2;
- missing evidence: no `request_worker_vm`, no `delegate_worker_vm`, no
  `vsuper`, no worker/verifier `co-super`, no export/promotion evidence.

Learning:

- The runtime tool path already auto-completes `request_worker_vm` into
  `delegate_worker_vm(profile=vsuper)`.
- The failed transition is upstream of tooling: `super` did not classify the
  broad self-development workload as requiring the worker-VM/vsuper boundary.
- The correct boundary is not "super can never do local stateful work." Super
  may do bounded scratch/API/script work directly when it is read-only,
  ephemeral, or low-risk: API calls, `curl` fetches, small data-processing
  scripts, transcript fetches, and temporary inspection artifacts.
- Super should delegate work that changes Choir/app/harness behavior or crosses
  a durable/risky boundary: repo edits, package installs, builds meant as
  candidate changes, runtime/app state mutation, Choir-in-Choir development,
  candidate-world exploration, worker/verifier loops, export/promotion, and
  dangerous or privileged actions.

Behavior commit already prepared by the previous control interval:

- commit: `1310647` (`Clarify super candidate-world delegation prompts`);
- pushed to `origin/main`;
- at rewrite time, GitHub Actions had not yet surfaced a run for that SHA, and
  staging still reported `d8185cc9b2f4c9eb8b765a891ef267f0160bd54c`.

First action for this mission: monitor CI/deploy for `1310647`, verify staging
health reports it, then run deployed acceptance proof.

## Value Criterion

Maximize verified self-development capacity per unit of sleeping-human
attention while minimizing fake progress, hidden state, prompt theater,
local-only proof, unreviewed canonical mutation, disposable UI, verifier
Goodharting, and downstream cleanup.

The run moves uphill if it makes the next long sweep more autonomous and more
evidence-readable, even if it stops on a precise substrate blocker.

## Hard Invariants

- Choir prompt bar remains natural language. Do not add slash commands or goal
  modes to Choir.
- Staging is the acceptance environment for VM, worker, gateway, auth,
  publication, promotion, rollback, and Choir-in-Choir claims.
- Platform behavior changes follow:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

- Inner Choir candidate work must not deploy or mutate global staging
  infrastructure. Global platform changes are made by outer Codex through git,
  CI, and deploy.
- Product-path browser proof may use `/api/prompt-bar`,
  `/api/prompt-bar/submissions/*`, `/api/vtext/*`, `/api/trace/*`,
  `/api/promotions/*`, `/api/continuations/*`, and `/api/run-acceptances/*`.
- Do not use `/internal/*`, `/api/test/*`, `/api/agent/*`, `/api/prompts`, raw
  event mutation endpoints, direct service ports, or manually seeded success
  records for browser/public proof.
- Signed-out users should be able to use the public desktop for read/explore
  actions. Ask for login only at mutation boundaries: saving state, editing,
  publishing, creating proposals, launching owned/candidate computers,
  uploading files, or calling LLM/search/worker-backed actions.
- Foreground/canonical state stays stable until explicit promotion.
- Super is an orchestration and low-risk scratch-execution actor. Vsuper owns
  background candidate-world orchestration. Co-super workers and verifiers are
  subordinate to super/vsuper.
- Worker co-super does not verify its own work. Verifier co-super returns
  evidence or failure messages to the worker over agent-to-agent channels.
- Failed candidates leave diagnostics, evidence refs, rollback/export refs
  where possible, cognitive-transform blocker analysis, and next safe probe.

## Anti-Fake-Island Invariant

Every low-resolution implementation must be continuously deformable into the
real target.

Forbidden examples:

- fake transclusion panels;
- decorative citation affordances without typed citation/provenance records;
- UI that exists only to satisfy a test and will be thrown away;
- local-only worker/export simulations presented as staging candidate proof;
- static article rendering when the target is VText-native reading;
- manually seeded success artifacts or raw event mutation.

Allowed low-resolution projections:

- one product-path worker/verifier iteration before full automatic cycles;
- one candidate/export blocker with exact Trace and tool evidence;
- one selected onboarding VText publication before general editorial channels;
- one typed citation/proposal edge before ranking/economics.

## Control Priorities

### P0: Finish Landing The Prompt-Boundary Fix

Required:

- monitor GitHub Actions for commit `1310647`;
- monitor staging deploy;
- verify `/health` reports proxy and sandbox deployed commit `1310647`;
- if CI/deploy fails, inspect logs, fix through git, push, and repeat.

### P1: Focused Substrate Proof

Submit a natural-language product prompt that does not depend on the magic word
"sweep" as a trigger. The prompt should ask Choir to improve a small part of
Choir itself through a candidate computer, because that is the real boundary.

Acceptance evidence:

- visible prompt-bar submission id;
- VText doc/revision ids;
- Trace shows conductor -> vtext -> super;
- Trace shows `request_worker_vm` and chained `delegate_worker_vm`;
- worker run profile is `vsuper`;
- vsuper spawns or attempts worker and verifier `co-super` agents;
- at least one worker/verifier channel message or a precise blocker explaining
  why co-super iteration could not start;
- candidate/export/promotion/rollback evidence if reachable;
- browser request audit has forbidden-route count `0`;
- run acceptance record is synthesized when evidence is sufficient.

If the path still stops at direct super updates, classify it as a substrate
blocker and inspect exact VText -> super objective text plus Trace tool calls
before patching again.

### P2: Use UX/Onboarding As The Test Workload

Only after P1 is coherent, use the same machinery on visible product UX:

- logged-out desktop remains useful for read/explore actions;
- auth-on-mutation is clear and preserves intent;
- prompt bar remains usable with windows open;
- Trace UI is readable enough on desktop and mobile to inspect trajectories,
  agents, tool calls, messages, and evidence refs;
- initial Choir/VText explainer should be a VText-native artifact, not a
  marketing page or fake island.

### P3: Trace Readability Is Verifier Surface

Trace is part of acceptance, not a debug afterthought.

If Trace cannot expose agent/tool/evidence chains on desktop or mobile:

- fix the smallest coherent reading/navigation slice, or
- record a precise blocker with screenshots, DOM metrics, and the next safe
  probe.

The v1 run already exposed one issue: a VText window intercepted desktop icon
clicks during automated Trace inspection, and the Trace screenshot showed a
loading state instead of usable trajectory evidence.

### P4: Quality Pass

Before stopping:

- review touched prompts, system prompt assembly, authority boundaries, tests,
  and browser proof code;
- avoid unrelated refactors;
- strengthen tests/verifiers for any bug that became an invariant;
- record residual risks and next objective.

## Inner Choir Prompt

Submit through the visible staging prompt bar after `1310647` is deployed:

```text
Use Choir to safely improve one small part of Choir itself without changing the active computer directly. Create a VText mission report. This is app/harness/Choir-in-Choir development, so ask super for a background candidate computer instead of doing durable repo/runtime/app mutation in the foreground. In that candidate world, have vsuper coordinate separate worker and verifier co-super agents over agent-to-agent channels. The worker should make or precisely block the smallest safe improvement related to Trace readability or prompt-bar/window ergonomics. The verifier should independently inspect evidence and either pass it or message the worker with the concrete failing condition. Record exact VText revision ids, Trace trajectory/run ids, worker VM or candidate ids, worker/verifier messages, tests, export/promotion/rollback refs where available, blockers, residual risks, and the next objective. Super may do bounded scratch/API/script work directly if it is only ephemeral evidence gathering, but Choir/app/harness/repo/runtime/candidate/export/promotion work must go through the worker VM and vsuper boundary. Do not use slash commands, forbidden internal/test routes, manual success seeding, direct service ports, local-only proof, fake transclusion panels, decorative citations, or canonical mutation without verification/promotion. Before stopping on a blocker, apply route-changing cognitive transforms and record the next safe probe. Before stopping successfully, do a quality pass.
```

## Dense Feedback

- `git status`, `git log`, GitHub Actions, staging `/health`.
- Focused Go tests for prompt defaults, role permissions, worker VM request,
  delegate worker VM behavior, channel messages, export patchsets, and run
  acceptance synthesis.
- Deployed Playwright proof through visible auth and prompt-bar flow.
- Trace assertions for conductor, VText, super, vsuper, worker co-super,
  verifier co-super, channel messages, tool calls, exports, and VText revisions.
- Screenshots or DOM metrics for Trace desktop/mobile readability.
- Browser request audit for forbidden public routes.
- VText report and run acceptance records, not chat-only claims.

## Rollback

- Platform changes roll back by normal git revert/fix-forward through
  `origin/main`, CI, and staging deploy.
- Candidate/worker changes stay inside candidate/worktree/VM boundaries until
  exported and verified.
- If a prompt fix causes worse routing, revert the prompt-default commit or add
  a focused corrective commit, then redeploy and rerun the focused proof.
- Do not promote candidate output without verifier contract evidence plus owner
  review and rollback evidence.

## Stopping Conditions

Stop successfully when:

- latest required platform commit is deployed and staging health reports it;
- Playwright submitted the inner prompt through the real visible prompt bar;
- VText contains the mission report;
- Trace either shows the intended super -> worker VM -> vsuper ->
  worker/verifier co-super topology, or records a precise substrate blocker;
- worker/verifier channel iteration, candidate/export/promotion evidence, or
  blocker evidence is named;
- Trace readability is proven enough for inspection or precisely blocked;
- run acceptance is synthesized when enough evidence exists;
- code-review/quality pass, rollback refs, residual risks, and next objective
  are recorded.

Stop negatively only after:

- the failing authority boundary or verifier surface is named;
- cognitive transforms were tried;
- no safe smaller product-path probe remains;
- rollback refs and next safe probe are recorded.
