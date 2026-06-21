# Choir Agent Operating Contract

This file is the repo-level contract for coding agents working on Choir.

[docs/choir-doctrine.md](docs/choir-doctrine.md) is the apex architecture and
doctrine document. `AGENTS.md` is the operating contract for agents; it inherits
Choir Doctrine and must not become a competing doctrine source. When they
conflict, follow Choir Doctrine unless this file is carrying a newer explicitly
promoted operating update.

## Default Environment

Staging is the acceptance environment: `https://choir.news`.

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

For source-opening doctrine, default durable web-derived reading to Source
Viewer/reader artifacts and use Web Lens only for explicit live/original
inspection. Do not reintroduce Browser-as-source-gathering framing in docs,
prompts, or tests.

Read [docs/computer-ontology.md](docs/computer-ontology.md) before changing VM, sandbox, candidate-world, promotion, package, or persistent-state behavior. The product object is a persistent user computer. `sandbox` is an implementation/service name, not the product ontology.

Classify every mission/change by mutation class before editing:

- `green`: docs, comments, labels, and prompt/default text with no runtime
  behavior change;
- `yellow`: tests, detector manifests, or prompt framing that change future
  optimization pressure but not product behavior directly;
- `orange`: runtime behavior, product APIs, app state, database queries, or
  provider/model routing;
- `red`: protected surfaces such as Texture canonical writes, Trace/evidence,
  promotion/rollback, candidate computers, auth/session renewal, vmctl,
  gateway/provider calls, run acceptance, and deployment routing;
- `black`: irreversible or production-destructive work.

Before touching an orange or red protected surface, name the conjecture delta,
protected surfaces, admissible evidence class, rollback path, and heresy delta
(`discovered`, `introduced`, `repaired`). Do not count newly discovered heresies
as regressions, and do not count discovery alone as repair.

## Worktree Hygiene

Before handing off or stopping a mission, run `git status --short` and classify
every dirty path as intentional source, durable documentation/evidence,
temporary proof output, generated artifact, or unrelated WIP.

Do not leave untracked scratch files in the repo. Temporary Playwright probes
may use `*.tmp.spec.js` while investigating, but before stopping they must be
deleted, moved outside the repo, or promoted to a normal tracked `*.spec.js`
with a clear regression purpose. Do not let scratch tests become a parallel
test suite.

If unrelated WIP is already present, preserve it explicitly instead of mixing it
into the current mission commit. Use a named stash or a separate branch/commit,
and report the recovery handle.

## Independent Review Threads

For second-opinion review, independent prover, or handoff-tier verification,
prefer Codex thread tools over in-thread subagents when the user authorizes a
separate thread. Use `list_projects` and `create_thread` to start a fresh
project-scoped verifier thread with a narrow review prompt, and ask that thread
to return a verdict with evidence rather than implementation. Keep the verifier
thread read-only unless it discovers a problem that must be documented under
Problem Documentation First.

When thread inspection or wakeup tools are available, use them to reconnect the
review to the spawning thread: `read_thread`/`list_threads` for the verifier
result, `send_message_to_thread` or the app's wakeup/follow-up mechanism when
the spawned reviewer needs to notify or continue the spawning thread, and
`handoff_thread` only when ownership of a checkout/worktree should move. If the
thread tools are unavailable, record the fallback used and do not treat a
same-context reread as an independent prover.

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

Docs-only commits are different. The full CI/deploy workflow intentionally
ignores `docs/**` and top-level `*.md`; do not weaken those filters to force a
full CI or staging deploy path for docs-only changes. Docs-only pushes and pull
requests should run the report-only docs truth checker workflow instead. If
documentation needs validation beyond that checker, run the specific check
directly or use a manual workflow dispatch when one exists.

## Parallax

For multi-hour, overnight, staging, self-development, or broad architectural
work, use Parallax. A Parallax mission document is a **paradoc**: it states the
mission conjecture, deeper goal, witness/spec, invariants and qualities,
domain ramp, variant, budget, authority bounds, live conjectures/open edges,
next move, ledger, lineage, learning state, and settlement requirement.

Read [docs/parallax-design-2026-06-11.md](docs/parallax-design-2026-06-11.md)
and the available Parallax skill before authoring or executing broad missions.
When a legacy MissionGradient document is still the best source form, compile
it in place into a Parallax State section instead of starting a disconnected
control file. Preserve historical MissionGradient reports as evidence; do not
treat them as current operating doctrine unless a newer paradoc promotes the
claim.

Do not turn Parallax into a brittle checklist. Treat the bridge from artifact
completion to deeper-goal progress as suspect until evidence supports it.
Select moves by expected variant decrease per budget, force observer shifts
when probes stop changing decisions, and exit only as settled, open_handoff,
blocked, or superseded.

For long-running Choir-in-Choir missions, maintain an owner-readable Texture
narrative. Each substantive change in plan, evidence, blocker, or result should
produce a concise revision that explains the whole run state so far in plain
language: objective, past work, current work, what changed, evidence, learnings,
risks, and next step. Do not make Texture a Trace-like topology/status table, and
do not dump low-level events into Texture. Trace is the causal ledger for dense
tool calls, LLM content, and agent-to-agent messages; feature-specific live
surfaces such as Chyron may show granular activity streams; Texture is the human
supervision narrative.

Read [docs/texture-agentic-invariants-2026-06-13.md](docs/texture-agentic-invariants-2026-06-13.md)
before changing Texture tools, prompts, routing, revision creation, coagent wake
behavior, Trace/Texture projection, run acceptance involving Texture, or missions
that use Texture as their owner-readable narrative. Texture is the canonical
document/versioning core and must remain an agentic participant in a multi-agent
system, not a workflow runner. Runtime may expose affordances and durable
obligations, but it must not force Texture to call researcher, super, verifier, or
any semantic appagent merely because prompt text, revision metadata, or an
acceptance probe mentions that role.

Texture is also Choir's artifact control plane. Conductor routes exogenous
user/app/source input into Texture-owned artifact state: prompt-bar requests,
sourcecycled/news ingestion, article creation, mission work, and most user
prompts should open or create Texture/context first. Super is not the direct
ingress target for ordinary user or source prompts. Texture may later call
`request_super_execution` when the Texture-controlled artifact needs execution,
coding-agent trees, generated artifacts, verification, candidate work, or other
privileged action, and downstream researcher/super evidence must attach back to
the Texture/artifact context.

## Authority Boundaries

- `conductor` routes exogenous user/app/connector input into Texture/artifact state. It is not the semantic babysitter and not a direct-super router for ordinary prompts.
- Appagents own durable app artifacts. `texture` owns canonical document versions.
- `researcher` writes structured findings/evidence, not canonical text or code.
- `super` is the foreground orchestration root. It can request workers and candidate worlds.
- `vsuper` owns a background/candidate computer or candidate world.
- `cosuper` is subordinate to the super/vsuper that requested or assigned it.
- Verification is a contract over evidence, not a separate privileged caste.

Foreground/canonical state stays stable. Background/candidate computers mutate. Canonical state changes only by promotion.

Texture delegation is agentic. Texture may write, ask researcher, ask super, ask
both, ask neither, wait for more evidence, or report a blocker within its
authority envelope. `edit_texture` stores a canonical revision; it must not become
a semantic workflow gate that requires a subsequent researcher/super/verifier
tool call. Exact required-tool continuation is reserved for narrow mechanical
tool protocols, not appagent policy.

Prompt bar, source ingestion, and article/news creation should show conductor
entry followed by Texture artifact materialization. `super` before Texture is a
route invariant failure. `super` after Texture is valid only when Texture requested
execution through an explicit affordance such as `request_super_execution`.

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

## Harness Minimalism

Keep the agent harness small and programmatically uniform across roles by
default. The core tool loop, provider call semantics, run-memory plumbing,
event emission, cancellation, retry, compaction, and continuation mechanics
should behave identically for conductor, Texture, researcher, super, vsuper,
co-super, verifier, and future agent roles unless there is a proven invariant
that requires divergence.

Prefer prompts, tool descriptions, capability policy, and product-visible
state over role-specific harness branches. (Prompt content itself is moving
from persona framing toward obligation/authority-envelope framing — see
`docs/choir-role-free-actor-protocol-2026-06-11.md` — but the structural point
here, prompt/policy over code branches, holds either way.) If a proposed fix
requires
programmatic divergence in the core loop for one role, document the evidence,
the invariant being protected, the simpler alternatives rejected, and obtain
explicit human approval before landing it. Divergence is acceptable only when it
protects correctness, security, authority boundaries, or resource isolation in a
way that cannot be represented cleanly as policy or prompt contract.

## Runtime Configuration

Provider secrets and platform model catalogs are platform-owned. Per-computer
model policy is computer-owned durable state and should be editable through the
product path, including by `super` in response to an owner prompt. Do not patch
Node B environment variables or tracked server files as a substitute for a
runtime policy path unless the mission is explicitly a platform config deploy.

Role defaults are policy defaults, not architecture. Any configured model may
serve any agent role when its declared capabilities match the current turn:
conductor, Texture, researcher, super, vsuper, co-super, verifier, or future
roles. Text-only models are valid for orchestration, research, coding, writing,
and verification that does not need media input. Multimodal models are required
only when the turn needs screenshots, images, video frames, files, or other
media inputs. If a current policy maps a role to ChatGPT or Fireworks, treat
that as the active computer's effective policy, not a hard-coded role boundary.
Capability is evaluated for the next turn, not permanently for the role.
Do not add new role-specific provider assumptions such as "conductor must be
ChatGPT", "super must be ChatGPT", "Texture must be Fireworks", or "verifier must
be multimodal" unless the current turn's capability requirements actually imply
that. The long-term target is dynamic, agentically editable per-computer model
policy: an owner prompt may ask `super` to edit the computer's model policy,
and subsequent runs should use that policy without a platform deploy or Node B
environment edit. The platform catalog records model capabilities and provider
request semantics; per-computer policy selects among those capabilities.

Provider request schemas must preserve modality. If a task needs screenshots,
videos, files, or other media evidence, route through a model/provider path that
declares that modality and record the blocker precisely when the adapter cannot
resolve the artifact.

## Product-Path Verification

Browser or Playwright acceptance may use public authenticated product APIs such as:

- `/api/prompt-bar`
- `/api/prompt-bar/submissions/{id}`
- `/api/texture/*`
- `/api/trace/*`
- `/api/app-change-packages/*`
- `/api/computers/*/source-lineage`
- `/api/computers/*/adoptions`
- `/api/adoptions/*`
- `/api/continuations/*` (transitional H007/H008 residue; prefer
  trajectory/work-item product evidence when available and do not add new
  continuation-shaped acceptance)
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

Required evidence should include trajectory/run ids, authority profile,
build/deploy identity, worker/candidate handle evidence, AppChangePackage/
adoption evidence or a precise blocker, verifier contracts, rollback refs,
heresy delta, conjecture delta, and residual risks. Existing records may still
carry legacy lease vocabulary; treat that as transitional H019 residue, not the
target actor model. Use explicit levels: `docs-level`, `staging-smoke-level`,
`export-level`, `promotion-level`, `continuation-level`.

Do not claim `promotion-level` without AppChangePackage adoption verifier contract evidence plus owner review and promote/rollback evidence. Do not claim `continuation-level` without run-memory/compaction and continuation evidence.

`continuation-level` is transitional H008/H014 residue: the durable-actors rearchitecture
(`docs/choir-rearchitecture-durable-actors-2026-06-11.md`) re-points this
acceptance level at trajectory/work-item settlement evidence (portfolio M4).
Until that cutover lands, `continuation-level` keeps its current meaning and
evidence requirement above — do not weaken it and do not claim trajectory
settlement evidence in its place before the level is formally re-pointed.
Do not introduce new `continuation-level` claims or APIs as doctrine; M4 must
delete or explicitly shim the old surface.

## Git And Staging

GitHub `origin/main` is the source of truth for tracked deployed files. Do not edit tracked files directly on Node B as a source/config shortcut.

If a behavior-changing commit is pushed:

1. Monitor the GitHub Actions run for that SHA.
2. Confirm Node B deploy/health reports that SHA or deployed commit.
3. Run the relevant deployed Playwright/API acceptance proof against `choir.news`.
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
- mutation class and protected surfaces touched;
- heresy delta: `discovered`, `introduced`, `repaired`;
- conjecture delta and human-learning digest;
- residual risks and the next realism axis.
