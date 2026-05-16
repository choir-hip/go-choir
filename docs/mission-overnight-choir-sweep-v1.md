# MissionGradient: Overnight Choir-In-Choir Sweep v1

Status: proposed for overnight execution
Date: 2026-05-16
Operator: outer Codex supervising Choir through staging Playwright

## Real Artifact

The artifact is one deployed, evidence-backed overnight control interval that
uses Choir through Choir as much as the current substrate permits:

```text
outer Codex
-> staging Playwright at https://draft.choir-ip.com
-> natural-language prompt bar
-> VText mission artifact
-> Trace-visible super -> vsuper -> worker/verifier cosuper topology
-> worker/verifier channel iteration
-> candidate/export/promotion evidence or precise blocker
-> deployed fixes through normal GitHub/CI/staging when substrate gaps require it
-> first-use UX/onboarding sweep only after substrate evidence is coherent
```

The goal is not to produce many disconnected patches. The goal is to increase
Choir's ability to run the next long sweep with less Codex supervision while
also using a visible product slice as pressure.

## Starting State

- Staging is deployed at `d8185cc9b2f4c9eb8b765a891ef267f0160bd54c`.
- Platform VText publication exists through platform Dolt, internal
  `platformd`, proxy read APIs, and Svelte/VText public routes.
- Public published VText reader regressions fixed on staging:
  - exact and trailing-slash public permalinks resolve;
  - the visible fake metadata/transclusion panel is gone;
  - VText content is selectable DOM text;
  - signed-in reloads do not spawn duplicate published VText windows.
- The fake transclusion panel was a MissionGradient failure mode: a fake island
  that would be thrown away by the real Pretext inline citation/transclusion
  system.
- Embedded per-user Dolt runtime migration and platform Dolt service are in
  place.
- Choir-in-Choir substrate exists in partial form: worker delegation, run memory,
  candidate worlds, promotion queue records, and run acceptance records exist,
  but live product-path sweep proof remains weak.

## Control Update: Sweep As Substrate Training Ground

The overnight sweep is not a UX bug bash with agent theater around it. The
visible product sweep is the workload used to test and improve the actual
Choir-in-Choir substrate.

First control interval on 2026-05-16 submitted the inner prompt through the
visible staging prompt bar after auth-on-mutation:

- staging health: proxy and sandbox deployed commit
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
- The failed transition is therefore upstream of tooling: the broad sweep
  prompt reached `super`, but the role prompts did not make the authority
  boundary clear enough. `super` may do bounded local scratch work such as API
  calls, `curl` fetches, and small scripts, but app/harness/Choir-in-Choir
  development, repo-aware changes, candidate-world work, worker/verifier loops,
  package/runtime changes, export/promotion, and dangerous or durable mutation
  should route through worker VM -> `vsuper`.
- Trace UI itself exposed a verifier-surface issue: a VText window can intercept
  desktop icon clicks during automated inspection, and the Trace window showed
  a loading state rather than usable trajectory evidence in the screenshot.

Route correction:

- Treat prompt/substrate learning and prompt-default changes as first-class
  mission output.
- Keep the UX/onboarding sweep as the test workload, but do not spend effort on
  UX fixes until the sweep workload reliably exercises
  `super -> vsuper -> worker/verifier co-super`.
- Strengthen VText and Super role prompts so app/harness/Choir-in-Choir
  development, candidate-world, worker/verifier, vsuper/co-super, repo-aware
  changes, export, promotion, package/runtime changes, and dangerous/durable
  mutation preserve the intended worker-VM -> `vsuper` topology, while bounded
  local scratch/API/script work remains available to `super`.
- Rerun the smallest deployed product-path proof after the prompt fix; only
  then continue into broader UX/onboarding sweep work.

## Value Criterion

Maximize verified self-development capacity per unit of sleeping-human attention
while minimizing fake progress, hidden state, local-only proof, disposable
placeholder UI, verifier Goodharting, unreviewed canonical mutation, and
downstream cleanup.

The overnight run moves uphill if tomorrow morning there is a durable VText,
Trace, run-acceptance, candidate/export, or blocker record that makes the next
sweep more reliable.

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
- Product-path browser proof may use `/api/prompt-bar`, `/api/vtext/*`,
  `/api/trace/*`, `/api/promotions/*`, `/api/continuations/*`, and
  `/api/run-acceptances/*`.
- Do not use `/internal/*`, `/api/test/*`, `/api/agent/*`, `/api/prompts`, raw
  event mutation endpoints, direct service ports, or manually seeded success
  records.
- Signed-out users should be able to use the public desktop for read/explore
  actions. Ask for login only at mutation boundaries: saving state, editing,
  publishing, creating proposals, launching owned/candidate computers, uploading
  files, or calling LLM/search/worker-backed actions.
- Foreground/canonical state stays stable until explicit promotion.
- `super` delegates candidate-world orchestration to `vsuper`; `vsuper`
  delegates work and verification to bounded cosupers.
- Worker cosuper does not verify its own work. Verifier cosuper returns evidence
  or failure messages to the worker over agent-to-agent channels.
- Failed candidates leave diagnostics, evidence refs, rollback/export refs where
  possible, and next safe probe.

## Anti-Fake-Island Invariant

Every low-resolution implementation must be continuously deformable into the
real target.

Forbidden examples:

- visible fake transclusion panels;
- decorative citation affordances without typed citation/provenance records;
- product UI that exists only to satisfy a test and will be thrown away;
- local-only worker/export simulations presented as staging candidate proof;
- static article rendering when the target is VText-native reading;
- one-off onboarding pages that do not become a published/readable VText or
  real desktop startup behavior.

Allowed low-resolution projections:

- clean VText reader with provenance as machine-readable data while real
  Pretext inline transclusion is not built;
- one typed citation/proposal edge before ranking/economics;
- one product-path worker/verifier iteration before full automatic cycles;
- one selected onboarding VText publication before general editorial channels.

## Priority Order

### P0: Overnight Run Safety And Evidence

Before broad product work, prove that the overnight loop can see what it is
doing.

Required evidence:

- staging health identity;
- visible prompt-bar submission id;
- VText doc/revision id for the mission report;
- Trace trajectory/run ids;
- enough Trace UI usability evidence to inspect the run on desktop and mobile,
  or a precise Trace-UI blocker that gets promoted above product polish;
- browser request audit with forbidden-route count;
- run acceptance record when enough evidence exists.

### P1: Sweep Substrate

Use Choir through the product path to run one bounded self-development sweep.

Targets:

- repo-aware worker/candidate environment or precise blocker;
- Trace evidence of super -> vsuper -> worker/verifier cosupers;
- Trace is treated as an evidence instrument, not just a debug page: if its
  current UI blocks inspection, especially on mobile, improve the smallest
  reading/navigation slice needed to verify the sweep;
- at least one worker/verifier channel message loop, pass/fail, or precise
  missing-substrate blocker;
- candidate/export/promotion queue evidence if reachable;
- cognitive-transform blocker analysis before stopping negatively;
- quality pass before stopping successfully.

### P2: First-Use UX/Onboarding Sweep

Only after P1 evidence is coherent, use the sweep machinery on visible product
UX.

Targets:

- signed-out first view explains Choir through a VText-native artifact, not a
  marketing page;
- logged-out desktop is useful without identity for read/explore/open-public
  actions, and login/register appears only when the user tries to mutate state
  or call an LLM/search/worker-backed action;
- login/register/auth-on-mutation flow is understandable and preserves the
  user's intent after authentication;
- prompt bar remains usable when windows are open;
- desktop/window chrome is less crowded and more legible;
- Trace UI has a usable mobile layout for reading trajectory summaries, agents,
  tool calls, messages, and evidence refs;
- onboarding VText can be authored by one user, published, and selected as the
  platform guest startup explainer if the current publication system supports it.

### P3: Publication Reader Hardening

Do not reopen fake transclusion. Harden the real publication loop.

Targets:

- owner-visible publish UX clarity for selected revisions;
- proposal inbox/acceptance state for authors;
- retraction/supersession route state;
- retrieval over published spans with exact refs;
- citation/proposal display from typed platform rows.

### P4: Pretext Inline Transclusion Design Slice

Only start implementation if P1-P3 do not reveal a higher-priority substrate
blocker.

Target shape:

- citation superscripts in host VText;
- click/tap expands source text inline with a small margin in the host flow;
- immutable source version/span refs and snapshot text;
- typed proposal/citation/transclusion records;
- no standalone transclusion panel.

### P5: Ingestion And Radio/Podcast

Keep as exploratory overnight stretch only.

Targets:

- URL/YouTube/text upload ingestion as VText source material;
- podcast index search as content artifact input;
- radio/podcast traversal as a projection of VText, not a separate media toy.

### P6: General Code Review And Quality Pass

Run throughout the sweep and explicitly before stopping. This is not permission
for unrelated refactoring. Review the changed surfaces and the highest-risk
adjacent boundaries:

- auth-on-mutation/public-read split;
- Trace data/UI contracts;
- VText/publication proposal contracts;
- worker/vsuper/cosuper authority and evidence paths;
- mobile/responsive layout of touched surfaces;
- tests and acceptance commands that could be Goodharted.

Fix small, high-confidence defects discovered during review. Record larger
quality issues as next objectives instead of churning the overnight run.

## Inner Choir Prompt

Submit through the visible staging prompt bar:

```text
Run one bounded overnight-style sweep control interval using MissionGradient,
Cognitive Transform Portfolio when stuck, MissionBag, and Sweep geometry. The
primary objective is to prove and improve Choir's own sweep substrate before
product polish: create a VText mission report, use Trace-visible super ->
vsuper -> worker/verifier cosuper topology, have worker and verifier cosupers
communicate over agent-to-agent channels until pass/fail or a real blocker, and
record candidate/export/promotion/rollback evidence where available. If the
substrate is coherent, spend the remaining effort on first-use UX/onboarding:
signed-out platform desktop that remains usable for read/explore actions until
mutation or LLM/search/worker actions require auth, login/register
auth-on-mutation clarity, an initial VText explainer for Choir/VText, prompt
bar ergonomics with windows open, Trace UI readability including mobile, and
desktop polish. Include a general code-review/quality pass over touched and
high-risk adjacent surfaces. Do not use slash commands, forbidden internal/test
routes, manual success seeding, direct service ports, local-only proof, fake
transclusion panels, decorative citations, or canonical mutation without
verification/promotion. Before stopping on a blocker, apply cognitive
transforms and record the next safe probe. Before stopping successfully, do a
quality pass and record the next objective.
```

## Outer Codex Duties

- Register/login to staging through product auth only.
- Drive the prompt through Playwright, not by private runtime mutation.
- Inspect Trace, VText, public product APIs, and staging health.
- Treat Trace readability as part of the verifier surface: if it is unusable on
  mobile or cannot reveal agent/tool/evidence chains, fix the smallest coherent
  slice or record it as a blocker.
- If substrate fixes are needed, make repo changes locally, commit, push,
  monitor CI/deploy, verify staging commit identity, and rerun acceptance.
- Do not let Choir-internal workers push or deploy platform changes.
- Preserve a clean git worktree before final sleep report.

## Dense Feedback

- `git status`, `git log`, GitHub Actions, staging `/health`.
- Focused Go tests for touched runtime/store/proxy/platform behavior.
- Frontend build and focused Playwright for touched UI.
- Deployed Playwright audit for prompt-bar, VText, Trace, published routes,
  reload behavior, and forbidden browser requests.
- Desktop and mobile screenshots or DOM assertions for Trace when Trace UI is
  changed or blocks verification.
- Product/API evidence for run acceptance synthesis.
- VText report and Trace evidence, not chat-only claims.

## Stopping Conditions

Stop successfully when:

- staging proves the latest platform changes if any;
- inner Choir prompt was submitted through the visible prompt bar;
- VText contains the mission report;
- Trace shows intended topology or a precise invariant-level blocker;
- worker/verifier iteration, candidate/export/promotion evidence, or blocker
  evidence is named;
- logged-out desktop/auth-on-mutation behavior and Trace mobile readability are
  either improved/proven or recorded as named blockers;
- a code-review/quality pass and next objective record exist.

Stop unsuccessfully only after:

- the failing invariant is named;
- cognitive transforms were tried;
- no safe smaller probe remains for the current authority/budget;
- rollback refs and residual risks are recorded.

## One-Line Goal String

```text
/goal Run docs/mission-overnight-choir-sweep-v1.md as a Codex-operated MissionGradient mission: supervise a staging Choir-in-Choir overnight sweep through Playwright and the visible prompt bar, prioritize sweep substrate proof before first-use UX/onboarding, keep the logged-out desktop usable until mutation or LLM/search/worker actions require auth, improve or precisely block Trace UI readability especially on mobile, forbid fake-island placeholders such as fake transclusion panels, land any required platform fixes through git/CI/deploy, and finish with VText/Trace/run-acceptance evidence, code-review quality pass, rollback refs, residual risks, and next objective.
```
