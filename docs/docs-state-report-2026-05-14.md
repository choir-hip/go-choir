# go-choir Documentation State Report

Date: 2026-05-14

## Purpose

This report audits top-level Markdown files and `docs/*.md` after the mission-geometry README pass. It is intentionally conservative: old docs may contain useful signal, but stale docs should not be allowed to function as current operating instructions.

Choir is not a chat app. Chat may be an input/control affordance, but the product output is documents, artifacts, sources, versions, candidate worlds, promotions, and public memory. Documentation should preserve that ontology while making the implementation repo easier to navigate.

## Status labels

- `canonical-current`: current operating entrypoint or invariant-bearing doc.
- `current-mission`: active or near-active MissionGradient / implementation mission.
- `evidence-artifact`: proof, dogfood report, or run evidence. Preserve as evidence unless folded into canonical docs.
- `historical-signal`: old but useful context; not current instructions.
- `stale-dangerous`: stale enough to mislead agents or expose operational/security-sensitive directions.
- `delete-after-extraction`: useful facts should be folded elsewhere, then the file can go.
- `delete`: safe deletion candidate with no clear remaining signal beyond git history.

## Executive recommendations

1. Keep `README.md`, `AGENTS.md`, `docs/mission-geometry.md`, `docs/computer-ontology.md`, `docs/project-goals.md`, `docs/glossary.md`, `docs/adr-dolt-as-canonical-state.md`, `docs/current-architecture.md`, `docs/runtime-invariants.md`, `docs/implementation-scope.md`, `docs/north-star.md`, and `docs/README.md` as the canonical current spine.
2. Promote the `TODOS.md` SQLite/Dolt note into canonical state docs or an ADR, then delete `TODOS.md`. The desired direction is stronger than “evaluate”: Dolt should be canonical product state; SQLite should remain only for narrow hot/runtime/cache/compatibility roles when explicitly justified.
3. Move and update `PROJECT-GLOSSARY.md` into `docs/glossary.md`; it is useful but stale and misplaced at repo root.
4. Extract live content from `PROJECT-GOALS.md` into canonical docs or a refreshed `docs/project-goals.md`, then remove the top-level file.
5. Replace or delete `docs/PROJECT-STATE.md`. It is marked historical, but it contains stale operational/provider/credential material and old continuation instructions that are likely to mislead agents.
6. Do not mass-delete proof docs in the first cleanup pass. Most `*-proof-*`, `*-dogfood-*`, `*-blocker-*`, and `*-next-frontier-*` files are evidence artifacts. Index them, then fold durable lessons into canonical docs over time.
7. Old Mission 1-7 docs are mostly historical-signal or delete-after-extraction candidates. They should not remain visually equal to current MissionGradient docs.

## Cleanup Execution In This PR

The follow-up cleanup was applied in this same PR rather than a second PR:

- `PROJECT-GLOSSARY.md` moved into the updated `docs/glossary.md`.
- `TODOS.md` was promoted into `docs/adr-dolt-as-canonical-state.md` and deleted.
- `PROJECT-GOALS.md` was extracted into `docs/project-goals.md` and deleted.
- `docs/PROJECT-STATE.md` was replaced with a short historical pointer.
- Old Mission 1/2/3/5/6/7 docs were deleted after their live signal was folded into canonical docs.
- Proof, dogfood, blocker, and next-frontier evidence files were kept.

## Documentation matrix

| Path | Last touched | Current status | Signal value | Staleness risk | Recommendation | Extraction target | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `AGENTS.md` | 2026-05-13 `4db1144` | canonical-current | high | low | keep/edit as repo contract | n/a | Agent operating contract; staging-first language is current enough. |
| `README.md` | 2026-05-14 `4d964bf` | canonical-current | high | low | keep | n/a | Current repo entrypoint after mission-geometry PR. |
| `TODOS.md` | 2026-04-30 `1f5d151` | delete-after-extraction | medium | medium | promote Dolt/SQLite decision then delete | `docs/runtime-invariants.md` or `docs/adr-dolt-as-canonical-state.md` | Contains live state-boundary pressure; should not remain as top-level todo. |
| `PROJECT-GLOSSARY.md` | 2026-04-30 `1f5d151` | delete-after-extraction | high | medium | update and move to `docs/glossary.md` | `docs/glossary.md` | Useful canonical vocabulary but missing mission-geometry/run-control/radio terms. |
| `PROJECT-GOALS.md` | 2026-05-04 `9894da8` | delete-after-extraction | high | high | extract live goals, then remove root file | `docs/current-architecture.md`, `docs/runtime-invariants.md`, possible `docs/project-goals.md` | Valuable vtext/conductor/Trace/Dolt content, but many checklists and next-runs are stale. |
| `docs/README.md` | 2026-05-14 `a2c4430` | canonical-current | high | low | update taxonomy | n/a | Should explicitly explain canonical/current/evidence/historical/stale buckets. |
| `docs/mission-geometry.md` | 2026-05-14 `4d964bf` | canonical-current | high | low | keep | n/a | High-level mission geometry and product ontology. |
| `docs/computer-ontology.md` | 2026-05-14 new in this PR | canonical-current | high | low | keep | n/a | Names the persistent computer object, ledger split, personal promotion, platform promotion, and update algebra. |
| `docs/project-goals.md` | 2026-05-14 new in this PR | canonical-current | high | low | keep | n/a | Current goal continuum and extracted live signal from root project goals and old mission docs. |
| `docs/glossary.md` | 2026-05-14 new in this PR | canonical-current | high | low | keep | n/a | Updated canonical vocabulary replacing root `PROJECT-GLOSSARY.md`. |
| `docs/adr-dolt-as-canonical-state.md` | 2026-05-14 new in this PR | canonical-current | high | low | keep | n/a | Dolt/SQLite decision record replacing root `TODOS.md`. |
| `docs/current-architecture.md` | 2026-05-14 `a2c4430` | canonical-current | high | low | keep/edit incrementally | n/a | First architecture doc for current runtime changes. |
| `docs/runtime-invariants.md` | 2026-05-13 `4db1144` | canonical-current | high | low | keep/edit; add Dolt canonical-state direction | n/a | Right place for durable state boundary invariants. |
| `docs/implementation-scope.md` | 2026-05-04 `f4b65ea` | canonical-current | medium | medium | refresh dates/scope | `docs/current-architecture.md` | Near-term build order but older than latest mission geometry and controller work. |
| `docs/north-star.md` | 2026-05-14 `a2c4430` | canonical-current | high | low | keep | n/a | Long-range product direction now links mission geometry. |
| `docs/PROJECT-STATE.md` | 2026-05-13 `4db1144` | stale-dangerous | medium | high | replace with short historical pointer or delete | `docs/README.md`, git history | Contains stale Z.AI/provider/credential/Node B instructions and old continuation flow despite historical warning. |
| `docs/architecture.md` | 2026-04-30 `1f5d151` | historical-signal | high | high | mark historical; extract only if needed | `docs/current-architecture.md`, `docs/glossary.md` | Very large older design sketch; high signal but should not be read as current. |
| `docs/multiagent-architecture.md` | 2026-05-04 `9894da8` | historical-signal | medium | medium | mark historical or fold live pieces | `docs/current-architecture.md` | Useful older MAS framing; superseded by current architecture/runtime invariants. |
| `docs/research-dolt.md` | 2026-04-15 `55ad169` | historical-signal | high | medium | keep as research; link from future ADR | `docs/adr-dolt-as-canonical-state.md` | Dolt background research; not an implementation decision record. |
| `docs/api-surface-and-vtext-workflow-review-2026-05-01.md` | 2026-05-04 `f4b65ea` | historical-signal | medium | medium | keep as review artifact | `docs/current-architecture.md` if live gaps remain | Older API audit. Useful for archaeology, not current instruction. |
| `docs/api-vtext-hard-cutover-checklist-2026-05-01.md` | 2026-05-04 `f4b65ea` | historical-signal | medium | medium | keep as checklist artifact; extract unfinished items only | `docs/implementation-scope.md` | Old cutover checklist. Some goals completed. |
| `docs/desktop-vtext-ux-checklist-2026-05-08.md` | 2026-05-09 `5bd7da5` | historical-signal | medium | medium | keep as UX checkpoint; fold live UX invariants | `docs/current-architecture.md` | Specific deployed UX checklist. |
| `docs/vtext-next-planning-checklist-2026-05-09.md` | 2026-05-10 `270531d` | historical-signal | medium | medium | keep until next vtext planning pass | `docs/implementation-scope.md` | Planning checkpoint; may contain live vtext gaps. |
| `docs/publication-path-skeleton-2026-05-12.md` | 2026-05-12 `271adaf` | historical-signal | high | low | keep; promote publication invariants later | `docs/current-architecture.md`, `docs/mission-geometry.md` | Forward-compatible publication boundary; likely useful. |
| `docs/choir-origin-main-change-report-2026-05-10.md` | 2026-05-10 `270531d` | evidence-artifact | low | low | keep as historical change report | n/a | Repo cleanup report. |
| `docs/mission-1-deploy-pipeline.md` | 2026-04-10 `64a9933` | delete-after-extraction | low | high | delete after confirming deploy lessons absorbed | `AGENTS.md`, deployment docs | Early Factory-era deployment mission; likely superseded. |
| `docs/mission-2-build-system.md` | 2026-05-04 `f4b65ea` | delete-after-extraction | low | high | delete after extraction | `docs/README.md` history note | Old incremental service mission; completed/superseded. |
| `docs/mission-3-completion-summary.md` | 2026-05-04 `f4b65ea` | historical-signal | low | medium | keep only if historical summaries are desired; otherwise delete | git history | Mission summary from older phase. |
| `docs/mission-3-remaining-system-milestones.md` | 2026-05-04 `f4b65ea` | delete-after-extraction | low | high | delete after checking for live milestone residue | `docs/implementation-scope.md` | Old remaining-system plan; likely stale. |
| `docs/mission-4-core-functionality-and-choir-in-choir.md` | 2026-05-04 `f4b65ea` | historical-signal | medium | medium | keep as history or fold lessons | `docs/current-architecture.md` | Older core functionality mission. |
| `docs/mission-5-production-hardening-and-polish.md` | 2026-04-15 `55ad169` | stale-dangerous | low | high | delete after extraction | `AGENTS.md`, deployment docs | Old provider/production hardening mission; likely misleading now. |
| `docs/mission-6-desktop-ux-rewrite.md` | 2026-04-15 `55ad169` | historical-signal | medium | high | delete after UX lessons extracted | `docs/current-architecture.md` | Factory-era UX rewrite; contains useful design signal but stale implementation assumptions. |
| `docs/mission-7-cogent-integration.md` | 2026-04-16 `86a24b5` | stale-dangerous | medium | high | delete after extraction | `docs/current-architecture.md`, `docs/runtime-invariants.md` | Cogent is reference/bootstrap donor, not target control plane. This doc risks re-centering it. |
| `docs/mission-gradient-choir-in-choir-2026-05-11.md` | 2026-05-12 `5698736` | historical-signal | high | medium | keep but mark superseded by newer MissionGradient family | `docs/mission-choir-grand-deformation-v0.md` | Large MissionGradient seed; useful orientation but not active. |
| `docs/mission-gradient-choir-in-choir-final-report-2026-05-12.md` | 2026-05-12 `5698736` | evidence-artifact | medium | low | keep | n/a | Final report evidence for prior run. |
| `docs/mission-choir-grand-deformation-v0.md` | 2026-05-13 `efd6a2d` | current-mission | high | medium | keep; clarify active/proposed state | n/a | Grand mission geometry; broad 8-24h proposal. |
| `docs/mission-choir-in-choir-deformation-v0.md` | 2026-05-13 `efd6a2d` | current-mission | high | low | keep | n/a | Runnable mission geometry with completed slice. |
| `docs/mission-candidate-world-promotion-v0.md` | 2026-05-13 `efd6a2d` | current-mission | high | low | keep | n/a | Completed in repo; still important for promotion path. |
| `docs/mission-promotion-queue-v0.md` | 2026-05-13 `efd6a2d` | current-mission | high | low | keep | n/a | Next/product bridge mission. |
| `docs/mission-run-memory-v0.md` | 2026-05-13 `efd6a2d` | current-mission | high | low | keep | n/a | Completed in repo; important for compaction/run memory. |
| `docs/mission-run-acceptance-verification-v0.md` | 2026-05-13 `5eb232c` | current-mission | high | low | keep | n/a | Completed export-level acceptance mission and verifier docs. |
| `docs/mission-web-surface-rationalization-v0.md` | 2026-05-13 `efd6a2d` | current-mission | medium | medium | keep; revisit if web surface direction changes | n/a | Proposed mission; useful but not canonical. |
| `docs/mission-choir-in-choir-controller-v0.md` | 2026-05-14 `9ab4bc0` | current-mission | high | low | keep | n/a | Current controller mission; stopped on invariant blocker. |
| `docs/mission-choir-in-choir-controller-evidence-2026-05-14.md` | 2026-05-14 `9ab4bc0` | evidence-artifact | high | low | keep | n/a | Evidence companion for current mission. |
| `docs/*-proof-2026-05-13.md` | 2026-05-13 mostly `efd6a2d` | evidence-artifact | medium | low | keep for now; index by mission | `docs/README.md` | Includes backend browser, Obscura, podcast/radio, run memory, Trace, worker lease, promotion, and web-surface proofs. |
| `docs/*-dogfood-2026-05-13.md` | 2026-05-13 mostly `efd6a2d` | evidence-artifact | medium | low | keep for now | `docs/README.md` | Dogfood transcripts/reports prove mission slices. |
| `docs/*-blocker-2026-05-13.md` | 2026-05-13 mostly `efd6a2d` | evidence-artifact | medium | low | keep; blockers often contain high-value boundary findings | `docs/current-architecture.md` if invariant-level | Example: backend browser VM-local execution blocker. |
| `docs/*-next-frontier-2026-05-13.md` | 2026-05-13 mostly `efd6a2d` | historical-signal | medium | medium | keep but do not treat as active unless re-promoted | active mission docs | Next-frontier notes may duplicate newer missions. |

## Detailed grouped notes

### Top-level docs

Top-level Markdown should be sparse. Recommended top-level set:

- `README.md`
- `AGENTS.md`

Everything else should move into `docs/` or be deleted after extraction.

`TODOS.md` should become an ADR or invariant section, then disappear. Top-level TODO files decay quickly and make agents chase stale queues.

`PROJECT-GOALS.md` and `PROJECT-GLOSSARY.md` are useful but should not be root-level peers of the README. The glossary should be updated and moved. The goals file should either become a refreshed `docs/project-goals.md` or be mined into existing canonical docs.

### Dolt / SQLite decision

The old `TODOS.md` says to evaluate a SQLite -> Dolt hard cutover. The current design direction is clearer:

- Dolt should own canonical product state: vtext versions, appagent state, evidence metadata, publication staging, public artifact metadata, citation graph, VM lifecycle/capacity/routing records where appropriate, and compute/accounting records.
- SQLite may remain for narrow hot runtime, cache, local compatibility, or transitional implementation roles only when explicitly justified.
- The repo needs an ADR or runtime invariant that states this direction without forcing an unsafe all-at-once migration.

Recommended next doc:

```text
docs/adr-dolt-as-canonical-state.md
```

### Project state

`docs/PROJECT-STATE.md` should not survive as a long current-looking document. Even with a historical warning at the top, it contains stale credential, provider, and continuation instructions. If kept at all, it should be replaced by a short page saying the full historical content is available in git history.

### Mission docs

The docs directory mixes several layers:

1. current canonical docs;
2. active/current MissionGradient docs;
3. proof/evidence artifacts;
4. historical mission notes;
5. stale Factory-era mission notes.

That is acceptable only if `docs/README.md` makes the distinction explicit and agents are instructed not to treat every file equally.

## Applied Cleanup

This report originally recommended a second cleanup PR. That work is now part of
this PR:

1. `PROJECT-GLOSSARY.md` -> `docs/glossary.md`, including the computer ontology and the rule that `sandbox` is only a service/legacy name.
2. `TODOS.md` -> `docs/adr-dolt-as-canonical-state.md`, then deleted.
3. `PROJECT-GOALS.md` -> `docs/project-goals.md`, then deleted.
4. `docs/PROJECT-STATE.md` -> short historical pointer.
5. Old Mission 1/2/3/5/6/7 docs deleted after extraction.

## Full file inventory appendix

This inventory records the pre-cleanup audit state. The cleanup execution status
above names files moved, replaced, or deleted in this PR.

| Path | Last touched | Status | Recommendation | Extraction target |
| --- | --- | --- | --- | --- |
| `AGENTS.md` | 2026-05-13 4db1144 Refresh staging-first operational docs | canonical-current | keep | n/a |
| `PROJECT-GLOSSARY.md` | 2026-04-30 1f5d151 Document vtext-first architecture and remove factory assumptions | delete-after-extraction | move/update | docs/glossary.md |
| `PROJECT-GOALS.md` | 2026-05-04 9894da8 fix: restrict conductor delegation to vtext | delete-after-extraction | extract then delete or move | docs/current-architecture.md / docs/runtime-invariants.md / docs/project-goals.md |
| `README.md` | 2026-05-14 4d964bf docs: clarify Choir is not chat | canonical-current | keep | n/a |
| `TODOS.md` | 2026-04-30 1f5d151 Document vtext-first architecture and remove factory assumptions | delete-after-extraction | promote Dolt/SQLite invariant then delete | docs/runtime-invariants.md or docs/adr-dolt-as-canonical-state.md |
| `docs/PROJECT-STATE.md` | 2026-05-13 4db1144 Refresh staging-first operational docs | stale-dangerous | replace with pointer or delete | docs/README.md / git history |
| `docs/README.md` | 2026-05-14 a2c4430 docs: add Choir mission geometry | canonical-current | keep | n/a |
| `docs/computer-ontology.md` | new in this PR | canonical-current | keep | n/a |
| `docs/project-goals.md` | new in this PR | canonical-current | keep | n/a |
| `docs/glossary.md` | new in this PR | canonical-current | keep | n/a |
| `docs/adr-dolt-as-canonical-state.md` | new in this PR | canonical-current | keep | n/a |
| `docs/api-surface-and-vtext-workflow-review-2026-05-01.md` | 2026-05-04 f4b65ea feat: harden vtext workflow and runtime api | historical-signal | keep as dated artifact; extract live items | canonical docs if still live |
| `docs/api-vtext-hard-cutover-checklist-2026-05-01.md` | 2026-05-04 f4b65ea feat: harden vtext workflow and runtime api | historical-signal | keep as dated artifact; extract live items | canonical docs if still live |
| `docs/architecture.md` | 2026-04-30 1f5d151 Document vtext-first architecture and remove factory assumptions | historical-signal | keep/mark historical; extract durable lessons | canonical docs / ADRs |
| `docs/backend-browser-bounded-control-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/backend-browser-candidate-world-identity-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/backend-browser-capability-contract-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/backend-browser-cdp-screenshot-product-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/backend-browser-html-snapshot-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/backend-browser-link-snapshot-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/backend-browser-persistent-cdp-lifecycle-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/backend-browser-session-lifecycle-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/backend-browser-substrate-contract-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/backend-browser-trace-events-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/backend-browser-vm-local-execution-blocker-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/backend-obscura-browser-session-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/candidate-world-promotion-next-frontier-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | historical-signal | keep unless superseded by mission doc | active mission docs |
| `docs/candidate-world-promotion-v0-dogfood-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/choir-grand-deformation-product-slice-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | historical-signal | review manually in next pass | docs-state report follow-up |
| `docs/choir-in-choir-deformation-v0-dogfood-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/choir-in-choir-live-product-blocker-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/choir-in-choir-next-frontier-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | historical-signal | keep unless superseded by mission doc | active mission docs |
| `docs/choir-origin-main-change-report-2026-05-10.md` | 2026-05-10 270531d Preserve Choir auth setup and Obscura cleanup report | historical-signal | keep as dated artifact; extract live items | canonical docs if still live |
| `docs/context-limit-recovery-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/continuation-objective-fingerprint-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/current-architecture.md` | 2026-05-14 a2c4430 docs: add Choir mission geometry | canonical-current | keep | n/a |
| `docs/desktop-vtext-ux-checklist-2026-05-08.md` | 2026-05-09 5bd7da5 Record desktop vtext deployed verification | historical-signal | keep as dated artifact; extract live items | canonical docs if still live |
| `docs/docs-state-report-2026-05-14.md` | new in this PR | canonical-current | keep until superseded by next audit | n/a |
| `docs/implementation-scope.md` | 2026-05-04 f4b65ea feat: harden vtext workflow and runtime api | canonical-current | keep | n/a |
| `docs/inbox-delivery-idempotency-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/live-playwright-recurrence-control-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/live-playwright-worker-dogfood-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/local-vmctl-product-path-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/local-worktree-worker-fallback-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/mission-1-deploy-pipeline.md` | 2026-04-10 64a9933 Add mission briefs for deploy pipeline and system build | delete-after-extraction | extract live signal, then delete or historical-note | canonical docs / git history |
| `docs/mission-2-build-system.md` | 2026-05-04 f4b65ea feat: harden vtext workflow and runtime api | delete-after-extraction | extract live signal, then delete or historical-note | canonical docs / git history |
| `docs/mission-3-completion-summary.md` | 2026-05-04 f4b65ea feat: harden vtext workflow and runtime api | historical-signal | keep or delete after extraction | git history / current architecture |
| `docs/mission-3-remaining-system-milestones.md` | 2026-05-04 f4b65ea feat: harden vtext workflow and runtime api | delete-after-extraction | extract live signal, then delete or historical-note | canonical docs / git history |
| `docs/mission-4-core-functionality-and-choir-in-choir.md` | 2026-05-04 f4b65ea feat: harden vtext workflow and runtime api | historical-signal | keep or delete after extraction | git history / current architecture |
| `docs/mission-5-production-hardening-and-polish.md` | 2026-04-15 55ad169 Hard cut over etext to vtext with embedded Dolt | stale-dangerous | extract live signal, then delete or historical-note | canonical docs / git history |
| `docs/mission-6-desktop-ux-rewrite.md` | 2026-04-15 55ad169 Hard cut over etext to vtext with embedded Dolt | delete-after-extraction | extract live signal, then delete or historical-note | canonical docs / git history |
| `docs/mission-7-cogent-integration.md` | 2026-04-16 86a24b5 Document prompt flow and reset local mission priorities | stale-dangerous | extract live signal, then delete or historical-note | canonical docs / git history |
| `docs/mission-candidate-world-promotion-v0.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | current-mission | keep | n/a |
| `docs/mission-choir-grand-deformation-v0.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | current-mission | keep | n/a |
| `docs/mission-choir-in-choir-controller-evidence-2026-05-14.md` | 2026-05-14 9ab4bc0 Record Choir controller mission evidence | current-mission | keep | n/a |
| `docs/mission-choir-in-choir-controller-v0.md` | 2026-05-14 9ab4bc0 Record Choir controller mission evidence | current-mission | keep | n/a |
| `docs/mission-choir-in-choir-deformation-v0.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | current-mission | keep | n/a |
| `docs/mission-geometry.md` | 2026-05-14 4d964bf docs: clarify Choir is not chat | canonical-current | keep | n/a |
| `docs/mission-gradient-choir-in-choir-2026-05-11.md` | 2026-05-12 5698736 Record MissionGradient final proof | current-mission | keep | n/a |
| `docs/mission-gradient-choir-in-choir-final-report-2026-05-12.md` | 2026-05-12 5698736 Record MissionGradient final proof | current-mission | keep | n/a |
| `docs/mission-promotion-queue-v0.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | current-mission | keep | n/a |
| `docs/mission-run-acceptance-verification-v0.md` | 2026-05-13 5eb232c Record run acceptance proof | current-mission | keep | n/a |
| `docs/mission-run-memory-v0.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | current-mission | keep | n/a |
| `docs/mission-web-surface-rationalization-v0.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | current-mission | keep | n/a |
| `docs/multiagent-architecture.md` | 2026-05-04 9894da8 fix: restrict conductor delegation to vtext | historical-signal | keep/mark historical; extract durable lessons | canonical docs / ADRs |
| `docs/north-star.md` | 2026-05-14 a2c4430 docs: add Choir mission geometry | canonical-current | keep | n/a |
| `docs/objective-fingerprint-promotion-dedupe-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/obscura-browser-in-vm-frontier-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | historical-signal | review manually in next pass | docs-state report follow-up |
| `docs/obscura-cdp-screenshot-substrate-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/podcast-radio-brief-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/promotion-queue-owner-review-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | historical-signal | review manually in next pass | docs-state report follow-up |
| `docs/promotion-queue-product-bridge-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | historical-signal | review manually in next pass | docs-state report follow-up |
| `docs/prompt-product-path-worker-promotion-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/publication-path-skeleton-2026-05-12.md` | 2026-05-12 271adaf Document publication path skeleton | historical-signal | keep as dated artifact; extract live items | canonical docs if still live |
| `docs/research-dolt.md` | 2026-04-15 55ad169 Hard cut over etext to vtext with embedded Dolt | historical-signal | keep/mark historical; extract durable lessons | canonical docs / ADRs |
| `docs/run-control-memory-synthesis-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/run-memory-next-frontier-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | historical-signal | keep unless superseded by mission doc | active mission docs |
| `docs/run-memory-v0-dogfood-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/runtime-invariants.md` | 2026-05-13 4db1144 Refresh staging-first operational docs | canonical-current | keep | n/a |
| `docs/trace-control-artifact-links-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/trace-run-geometry-visibility-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/vtext-next-planning-checklist-2026-05-09.md` | 2026-05-10 270531d Preserve Choir auth setup and Obscura cleanup report | historical-signal | keep as dated artifact; extract live items | canonical docs if still live |
| `docs/web-surface-rationalization-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
| `docs/worker-lease-portfolio-control-proof-2026-05-13.md` | 2026-05-13 efd6a2d Document Choir-in-Choir mission evidence | evidence-artifact | keep for now; index by mission | canonical docs if invariant-level |
