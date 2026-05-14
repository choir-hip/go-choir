# Choir Repo State Continuum Report

**Date:** 2026-05-14  
**Repo:** `go-choir`  
**Branch observed:** `docs/audit-documentation-state`  
**Head observed:** `5fdf178 docs: apply documentation cleanup`  
**PR observed:** <https://github.com/yusefmosiah/go-choir/pull/4>

## Purpose

This report takes stock of the repository after the documentation audit and
ontology cleanup. It describes:

1. the current state of the repo;
2. the ideal state the repo is converging toward;
3. the proximate next steps that should move the current state toward the ideal
   without collapsing the architecture into a short-term task queue.

The intended shape is a continuum, not a ladder. Specific implementation order
can change as evidence improves, but the invariants should remain stable:
foreground stability, candidate mutation, typed deltas, verification,
promotion, rollback, and durable learning.

## Evidence Snapshot

Local and GitHub evidence gathered for this report:

- Local branch is `docs/audit-documentation-state`.
- Local branch is clean against `origin/docs/audit-documentation-state` before
  this report was created.
- Open PR #4 is mergeable.
- PR #4 currently contains three commits:
  - `f9af200 docs: audit documentation state`
  - `0cfb08d docs: add computer ontology`
  - `5fdf178 docs: apply documentation cleanup`
- PR #4 changes 24 files relative to `main`.
- GitGuardian passed on the PR branch.
- GitHub Actions did not run for the docs-only branch, which matches the repo
  invariant that docs-only changes should not trigger automatic CI.
- The docs directory currently contains 72 Markdown files.
- The implementation surface includes:
  - six service entrypoints under `cmd/`: `auth`, `proxy`, `vmctl`,
    `gateway`, `sandbox`, `shipper`;
  - 66 files under `internal/runtime`;
  - 14 files under `internal/store`;
  - 41 frontend Playwright test files;
  - Nix deployment/runtime configuration under `nix/`.

## Current State

### Repository Posture

The repository is no longer primarily a pile of mission artifacts. It now has a
cleaner canonical spine:

- `README.md`
- `AGENTS.md`
- `docs/README.md`
- `docs/mission-geometry.md`
- `docs/computer-ontology.md`
- `docs/project-goals.md`
- `docs/glossary.md`
- `docs/adr-dolt-as-canonical-state.md`
- `docs/current-architecture.md`
- `docs/runtime-invariants.md`
- `docs/implementation-scope.md`
- `docs/north-star.md`

The cleanup removed root-level stale project docs and old Mission 1/2/3/5/6/7
docs after extracting live signal. The old `docs/PROJECT-STATE.md` is now only a
historical pointer. Proof, dogfood, blocker, and next-frontier evidence files
remain in place as evidence artifacts.

This matters because long-running agents need current operating instructions.
The repo now says more clearly which docs are canonical, which docs are mission
context, and which docs are historical evidence.

### Product Ontology

The most important conceptual correction is the move from "sandbox" to
**computer** as the product noun.

Current ontology:

- A user has a persistent computer, not a disposable sandbox.
- A computer is a product object made from multiple ledgers:
  VM/runtime state, Dolt/app state, source/build state, blob/content state,
  artifact provenance, and route identity.
- `sandbox` remains only an implementation/service name.
- Candidate work should happen in background/candidate computers or candidate
  worlds.
- Personal computer promotion and platform/public promotion are different
  paths.

This puts user-local evolution back into the architecture. A user should be able
to fork their own computer, build a local runtime or UI change, install
packages, add apps, edit prompts or themes, verify the result, and promote that
candidate back into their active computer without waiting for global CI/deploy.

### Runtime And Product Surface

The current deployed product is a web desktop served from a user computer, with
apps such as VText, Files, Browser, Trace, Terminal, Podcast, and Settings. The
runtime/control path is currently framed as:

```text
prompt bar -> conductor -> VText/appagent -> super
-> vmctl worker/candidate computer
-> worker export -> promotion candidate
-> verification/owner decision -> promotion or rollback
```

The codebase has enough implemented substrate to make this real in slices:

- auth and passkey session service;
- proxy with user-context injection and VM routing;
- gateway for model/search provider access;
- vmctl and VM manager surfaces;
- runtime service with VText, Trace, browser/control APIs, tools, run memory,
  promotion queue, and run acceptance;
- Svelte desktop with app surfaces and Playwright tests;
- Nix deployment to the staging environment.

But the product path is not yet fully self-developing. The repo can express and
partially verify export-level work, but promotion-level and continuation-level
acceptance remain the core frontier.

### State Model

The repo now has an accepted state direction:

- Dolt is the default canonical store for durable product state.
- Per-user embedded Dolt should own private computer/appagent state.
- Platform Dolt should own platform-visible facts, publication records,
  routing/capacity records where durable, public artifacts, citation graph, and
  compute accounting.
- SQLite can remain for narrow hot runtime, auth/session, cache,
  compatibility, or transitional implementation roles.
- The filesystem, source/build ledgers, blob/content store, and artifact graph
  are separate ledgers with different merge laws.

The important rule is: promote typed artifacts, not opaque machine accidents.

### Verification Posture

The repo now treats staging as the acceptance environment for platform behavior:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

For docs-only work, CI is intentionally skipped. That is correct and should stay
true.

For personal computer evolution, the verifier target is different. A local user
computer can promote its own candidate without global deploy, but that path
still needs:

- lineage;
- typed deltas;
- verifier evidence;
- foreground-tail reconciliation;
- atomic route switch;
- rollback target;
- no lost active-computer updates.

The repository has a `RunAcceptanceRecord` concept with explicit levels:

- `docs-level`
- `staging-smoke-level`
- `export-level`
- `promotion-level`
- `continuation-level`

The hard remaining verification work is to make promotion-level and
continuation-level acceptance concrete enough that Choir can safely continue
after one agent stops.

## Ideal State

### Product Ideal

Choir should be a durable learning-control system over versioned artifacts.

The deepest product should not be chat, a desktop, a document editor, an app
queue, or a coding-agent wrapper. Those are projections. The underlying object
should be a learning artifact graph that receives signals, produces evidence,
updates semantic artifacts, mutates candidate worlds, verifies deltas, promotes
safe changes, and renders the graph as screen, text, radio, public memory, or
later capital allocation.

In ideal form:

- evidence enters through researchers;
- meaning is owned by appagents;
- computation is orchestrated by `super`;
- mutation happens in candidate computers/worlds;
- verification is contracted, not reified as a special caste;
- compaction carries long-range learning;
- promotion is the canonical bridge;
- `vtext` is the semantic substrate;
- radio is screenless traversal of promoted meaning.

### Computer Ideal

Each user has one or more persistent computers that can diverge from the
platform baseline.

Users should be able to:

- install packages;
- build local Go runtime changes;
- build local Svelte UI changes;
- add apps;
- alter prompts and agents;
- create and edit themes;
- index personal data such as podcasts or files;
- promote changes into their active computer after verification;
- publish useful local changes as typed packages or public proposals;
- receive platform updates without losing local divergence.

The algebraic operation is not "replace the active VM with the candidate VM."
The operation is a ledger-aware join:

```text
        C
      /   \
    B0     M
      \   /
        A
```

`B0 -> C` is the candidate delta. `B0 -> A` is the active foreground tail. `M` is
the merged/promoted computer state or an explicit conflict.

### Repository Ideal

The repository should become the platform baseline and shared package source,
not the only place where Choir can evolve.

Platform changes should still land through commit, CI, deploy, and staging
proof. User-local computer changes should not need platform deployment. The repo
should provide:

- stable service/runtime APIs;
- package formats for apps, agents, prompts, themes, verifiers, and tools;
- verifier contracts;
- promotion certificate schemas;
- platform update machinery for divergent user computers;
- staging-first proof for shared behavior;
- docs that keep agents aligned with the current invariants.

### Verification Ideal

The verification target should move from "did a local test pass?" to "did the
system produce a durable, recomputable acceptance record for the level being
claimed?"

For long-running self-development, the minimum ideal evidence is:

- run objective and constraints;
- candidate computer lineage;
- typed deltas produced;
- verifier contracts run;
- owner/appagent acceptance where needed;
- rollback reference;
- promotion certificate;
- run memory/compaction;
- continuation decision or next mission gradient;
- deployed staging proof when platform behavior changed.

## Gaps

The main gaps are not isolated feature gaps. They are substrate gaps.

1. **Promotion is not yet first-class enough.** The repo has promotion queue and
   acceptance concepts, but candidate computer lineage, typed delta joins,
   verifier contracts, route switching, and rollback certificates need to become
   durable product objects.

2. **Continuation is not yet first-class enough.** Codex goals stop when they
   finish. Choir should continue with the next objective when safe. That needs
   run memory, compaction, continuation-level acceptance, and bounded authority.

3. **Candidate computers are not yet product-grade.** The docs now name active,
   background, and candidate computers, but the implementation still has to
   make lineage, capacity, routing, credentials, gateway access, and rollback
   reliable in staging.

4. **The web surface is still unsettled.** Obscura/backend browser work exposed
   real complexity. Some use cases may be pixel/remote-browser based; others
   should use the same Choir web UI stack for candidate computer views. The
   architecture should avoid pretending DOM passthrough solves websites that
   intentionally block embedding.

5. **Product proof apps are incomplete.** Launcher/start-button, Files upload,
   theme creation/editing, Podcast/Radio depth, and app package installation
   are not just polish. They are good test cases for personal computer
   divergence and promotion.

6. **Docs are cleaner but not finished.** Proof/evidence files are still many
   and useful. They should be indexed and mined gradually, not deleted in bulk.

## Proximate Next Steps

### 1. Land The Documentation Cleanup

Merge PR #4 once reviewed. This gets the canonical docs, glossary, computer
ontology, project goals, and Dolt ADR onto `main`.

This is not cosmetic. It gives future long-running agents the right substrate
language before the next behavior-changing mission.

### 2. Define The Next Mission Around Personal Promotion

The next implementation mission should target one narrow, demonstrable personal
promotion path:

```text
active computer -> candidate computer -> typed delta
-> verifier contract -> promotion certificate
-> route switch -> rollback proof
```

The patch does not need to solve every ledger. It should choose one small
product slice that proves the algebra and leaves durable evidence.

Good candidate slices:

- a theme package promoted into one user's computer;
- a small local app package installed into one user's computer;
- a prompt/agent package promoted into one user's computer;
- a Files upload artifact promoted into VText/Files metadata;
- a Podcast/Radio data improvement promoted as a user-local artifact.

The key is not the feature. The key is proving the promotion shape.

### 3. Extend Run Acceptance To Promotion-Level

Build acceptance records that can prove:

- candidate lineage exists;
- candidate mutation did not touch canonical foreground state;
- active foreground tail was preserved or explicitly conflicted;
- typed deltas were verified;
- route switch happened atomically;
- rollback target exists;
- owner/appagent acceptance was recorded when semantic state changed.

Do this in staging for platform behavior. For personal computer behavior, do it
inside the user's computer/candidate-computer model without requiring global
deploy for every local change.

### 4. Make Continuation-Level Acceptance Concrete

Choir should not stop merely because one agent stops. The next substrate after
run memory is continuation:

- compaction as operational sufficient statistic;
- next objective selection from accumulated evidence;
- bounded authority for the next leap;
- explicit stop conditions;
- durable failure and recovery records;
- evidence that the next run picked up the right state.

This should become a verifier target, not just a prose aspiration.

### 5. Use Missing Product Features As Verification Pressure

After the promotion substrate is narrow but real, use concrete app work to
pressure-test it:

- app launcher/start button;
- Files upload UI and artifact handoff;
- user theme creation/editing and local promotion;
- Podcast/Radio search/index improvements;
- candidate computer view inside the active computer using the same Choir web
  UI flow where possible;
- browser surface rationalization where Obscura is used deliberately rather
  than as a vague iframe workaround.

These features should be built as evidence-producing personal-promotion
examples, not as unrelated frontend polish.

### 6. Keep The Staging Discipline

For platform behavior-changing work:

```text
commit -> push origin main -> monitor CI -> monitor deploy
-> verify staging identity -> run deployed acceptance proof
```

For docs-only work:

- do not trigger automatic CI;
- use direct markdown/link checks when useful;
- keep docs path filters intact.

For user-local computer changes:

- do not force global deploy;
- require lineage, typed deltas, local verifier evidence, route rollback, and no
  lost foreground updates.

## Suggested Next Mission String

```text
Use MissionGradient. Execute a staging-first personal-promotion substrate mission: after PR #4 lands, build one narrow user-computer promotion path from active computer to candidate computer to typed delta to verifier contract to promotion certificate to route switch with rollback, prove it with deployed evidence where platform behavior changes, and use one small product slice such as theme/app/prompt/File artifact promotion as the concrete Choir-in-Choir demonstration.
```

## Bottom Line

The repo is now conceptually cleaner than it was this morning. The documentation
names the real object: persistent computers that can diverge, mutate in
candidates, and promote typed artifacts. The next implementation step should not
scatter effort across app polish. It should make one narrow promotion path real,
because that path is the bridge from "Codex builds Choir" to "Choir develops
Choir."
