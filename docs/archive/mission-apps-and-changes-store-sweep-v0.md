# MissionGradient: Apps & Changes Store Sweep v0

**Status:** complete
**Date:** 2026-05-21
**State ledger:** [platform-os-app-state.md](../platform-os-app-state.md)
**Prior portfolio:** historical alternate-computer portfolio docs were pruned
during Campaign Compiler cleanup. The retained current path is
AppChangePackage -> adoption -> recipient build -> verify -> promote/rollback.

Doctrine note (2026-06-13): any continuation- or Trace-shaped language in this
older mission is historical evidence of the reviewed product state, not target
doctrine.

## One-Line Goal String

```text
/goal Run docs/mission-apps-and-changes-store-sweep-v0.md as a Codex-operated MissionGradient mission: build the breadth-first Apps & Changes product surface for discovering, inspecting, trying, installing, uninstalling, disabling, and rolling back source-level changes without exposing package IDs as ordinary UI. Replace the launcher-facing Candidate Desktop app with Apps & Changes; delete dead Candidate Desktop code unless it is immediately refactored into a used internal ChangePreviewFrame/ChangePreviewPane exercised by the new flow. Seed the store from the four alternate-computer experiment packages, but use Chiron only as the first end-to-end proof payload after the store/review substrate exists: inspect -> Try in candidate preview -> verify recipient build -> install/promote -> uninstall/disable/rollback honestly. Make VText the live mission dashboard and require regular substantive updates plus per-change VText reports with screenshots/video/benchmark links. Capture screenshots and Playwright video or trace-derived clips for all four experiments, run real Liquid and Python benchmarks, land required platform changes through git/CI/deploy, verify staging identity, and prove the product path on desktop and 390x844 mobile. Do not ask users to paste package IDs, keep a dead Candidate Desktop island, use export_patchset or /api/promotions, copy binaries, fake preview thumbnails, mutate active computers during Try, hide technical refs entirely, or claim completion without product-visible catalog/change/adoption/rollback evidence, VText reports, rollback refs, residual risks, and the next realism axis.
```

## Mission Frame

The prior alternate-computer portfolio proved that four experiments can become
owner-pullable AppChangePackages. That is not enough for a product. A user
should not paste package ids or understand candidate VM internals. The product
object is a **Change**: a reviewable source-level modification that can be
discovered, inspected, tried in a candidate computer, installed into the user's
active computer, disabled or uninstalled when possible, and rolled back safely
when removal is not clean.

The user-facing app is **Apps & Changes**. It replaces the launcher-facing
**Candidate Desktop** app. The useful candidate preview machinery may survive
only as an internal component used by Apps & Changes in the same mission. Do
not keep speculative dead code for future reuse.

## Product Vocabulary

- **Apps & Changes:** the launcher app for discovering, reviewing, trying,
  installing, uninstalling, and rolling back changes.
- **Change:** user-facing installable/reviewable unit. It may be an app, app
  update, shell improvement, runtime/profile change, theme, workflow, or media
  reader improvement.
- **AppChangePackage:** hidden technical object backing a Change. It appears in
  technical details only.
- **Catalog:** product-visible list of available Changes, seeded in v0 from
  known packages and later backed by publication/discovery records.
- **Try:** apply/build/verify the Change in a disposable candidate/review
  computer without mutating the active computer.
- **Preview:** open the candidate/review computer desktop after Try succeeds or
  reaches a useful blocker.
- **Install:** promote a verified candidate/adoption into the active computer.
- **Uninstall:** remove one installed Change while preserving unrelated later
  changes, when verifier contracts can prove that removal is safe.
- **Disable:** turn off a Change through a supported feature/capability flag
  without removing source, when available.
- **Rollback:** return the computer to a prior source ref/profile. This is
  sometimes the only honest removal path.
- **Installed ledger:** product state naming installed Changes, version/source,
  adoption ids, artifact digests, verifier results, capability grants,
  dependencies, rollback refs, and uninstall/disable availability.

## Real Artifact

The artifact is:

```text
Apps & Changes app
  -> catalog of user-facing Changes
  -> change detail with evidence, screenshots/video, VText, Trace, verifier status
  -> Try flow that creates/applies/builds/verifies candidate preview computer
  -> preview surface for candidate computer
  -> Install/promote action
  -> Installed ledger
  -> Uninstall/Disable/Rollback actions with honest capability labels
  -> VText mission dashboard and per-change reports
  -> product-path tests/screenshots/video/Trace/run-acceptance evidence
```

The artifact is not:

- a renamed package-id form;
- a raw candidate VM launcher;
- a marketplace with ratings/payments/social discovery;
- a local-only installer script;
- a platform-default merge of Chiron;
- a hidden dev/debug panel;
- an AppChangePackage admin console dressed as product UI.

## Seed Changes

Seed the v0 catalog with the four existing experiment packages. Package ids and
source owner ids are seed data and technical-detail refs; ordinary catalog
cards must use human names/descriptions.

| Change | Package | Source Owner | First Product Action |
| --- | --- | --- | --- |
| Chiron Shelf Observability | `28433c19-5d02-416f-9368-de56390e1927` | `80e6da5b-9394-4ebd-8aee-a531927221c7` | first end-to-end Try/Install/Uninstall proof |
| Process & Window Motion | `98b98c73-eef0-4a88-a6f5-b7dfe695be09` | `80e6da5b-9394-4ebd-8aee-a531927221c7` | inspect + evidence report |
| Liquid Material Shell | `1dad3dfc-7f83-4b22-bfb5-7f1714159f66` | `e1842324-90e5-4dfa-b9f1-64db95a46744` | benchmark + inspect |
| Python Code Mode A/B | `f31edbc8-1b43-44f5-82a1-834dce4833ca` | `e1842324-90e5-4dfa-b9f1-64db95a46744` | benchmark + inspect |

## Architecture Shape

```text
Change catalog record
  -> AppChangePackage refs
      -> Try/adoption candidate
          -> recipient-specific Go/Svelte build
              -> verifier result
                  -> preview desktop
                      -> install/promote
                          -> installed-change ledger
                              -> disable/uninstall/rollback
```

Core product APIs may use existing routes where they are correct:

- `/api/app-change-packages/*`
- `/api/app-change-packages/pull`
- `/api/computers/*/source-lineage`
- `/api/computers/*/adoptions`
- `/api/adoptions/*`
- `/api/trace/*`
- `/api/vtext/*`
- `/api/run-acceptances/*`

But the UI should present Changes, not package records. If new product APIs are
needed, prefer a small `changes`/catalog facade over teaching the frontend to
assemble product concepts from raw package internals.

## Preview Without Install

Preview is still useful, but only under a selected Change.

Valid flow:

```text
Change detail -> Try
  -> pull package if needed
  -> create candidate/review adoption
  -> build recipient runtime/UI artifacts
  -> run verifier contracts
  -> open preview desktop for that candidate/review computer
  -> user chooses Install, Discard, Disable, Uninstall, or Rollback where valid
```

Invalid flow:

```text
launcher -> Candidate Desktop -> manual candidate id -> raw iframe
```

Candidate preview code may remain only if it is actively imported by Apps &
Changes and exercised in deployed product proof. Otherwise delete it.

## Dead-Code Rule

Hard cutover. No compatibility island.

Required cleanup:

- remove `candidate-desktop` from the Desk/launcher registry;
- delete `CandidateDesktopViewer.svelte` unless its useful code is immediately
  refactored into a smaller component such as `ChangePreviewFrame.svelte`;
- if a preview component is kept, prove it is imported and exercised by
  `AppsChangesApp.svelte`;
- run `rg "CandidateDesktopViewer|candidate-desktop"` and ensure remaining
  matches are only docs, migration notes, or intentional internal test fixtures;
- update [platform-os-app-state.md](../platform-os-app-state.md) to replace
  Candidate Desktop with Apps & Changes.

No "maybe later" code. If it is not wired into the new flow, remove it.

## Breadth Task Ledger

Keep this ledger current during the mission. Do not go depth-first on Chiron
until the breadth substrate exists.

| Area | Required Outcome | Status |
| --- | --- | --- |
| Product model | User-facing terms and lifecycle are encoded in docs/UI/API names. | checkpoint: UI and platform-state docs use Change/Apps & Changes; deeper API names remain AppChangePackage/adoption. |
| Catalog | Four seed Changes appear without package ids in ordinary UI. | deployed-verified: four cards render on desktop and 390x844 mobile; package/source refs are hidden in collapsed technical details. |
| Apps & Changes UI | Replaces Candidate Desktop in launcher/Desk. | deployed-verified: `apps-changes` replaces `candidate-desktop`; launcher absence is tested and proven on staging. |
| Change detail | Shows summary, screenshots/video, VText, Trace, verification, risks, compatibility, and collapsed technical refs. | deployed-verified: summary, proof text, action state, candidate/build refs, rollback status, VText report creation/opening, collapsed technical refs, all-four artifact/benchmark links, all-four package-scoped accepted summaries, and selected Chiron Trace/run-acceptance surfacing exist; media links remain path text. |
| Try flow | Creates candidate/review adoption without mutating active computer. | deployed-verified for Chiron: product UI created adoption `adoption-chiron-shelf-62e544c3-4d3c-484a-9638-317fe964f554` and candidate `candidate-chiron-shelf-c0ec9010-bf57-45a0-bfb9-954050dd6638`. |
| Preview | Opens candidate/review desktop from the selected Change. | deployed-verified structurally through internal `ChangePreviewFrame`; preview iframe is created from the selected Change, not from a launcher-facing manual candidate id. |
| Install | Promotes verified candidate into active computer with rollback refs. | deployed-verified for Chiron: verify -> adopted -> rolled_back through product APIs with recipient runtime/UI artifact digests and rollback profile. |
| Uninstall | Honestly supports inverse/remove flow when safe or marks rollback-only/disable-only. | deployed-verified honesty: Apps & Changes now marks Chiron as rollback-only, keeps Uninstall disabled with "no verified inverse source patch", and no longer treats empty rollback JSON as evidence. Real inverse uninstall remains pending. |
| Disable | Represents feature-flag/capability-disable only when supported. | deployed-verified honesty: Apps & Changes keeps Disable disabled with "no declared feature flag"; no disable support is claimed. |
| Installed ledger | Shows installed Changes and action availability. | deployed-verified for the Chiron flow: install and rollback states are product-visible, and the selected Change exposes a removal/recovery panel; richer installed history polish remains a follow-up. |
| VText dashboard | Live mission VText updated on substantive changes. | deployed-verified: Apps & Changes opens/creates `Apps & Changes Store Sweep v0` through product VText APIs on desktop and 390x844 mobile. |
| Per-change VTexts | Chiron, Motion, Liquid, Python each get owner-readable reports. | deployed-verified: all four reports were generated/opened through product VText APIs on desktop and 390x844 mobile with package refs, acceptance refs, manifest hashes, benchmark status, and artifact links. |
| Screenshots/video | All four Changes have review media linked from detail/VText. | deployed-verified as linked artifacts: all four reports link screenshots/video/benchmark paths. VText still cannot embed image/video inline. |
| Liquid benchmarks | WebGL/WebKit/mobile/desktop resource/frame evidence. | local benchmark complete and deployed-report linked: isolated package worktree rendered WebGL in Chromium and WebKit at desktop and 390x844, avg 16.66-16.67ms and p95 <= 18.1ms. Manual real mobile Safari plus heavy-session battery/thermal review remain residual risks. |
| Python benchmarks | Matched bash-vs-Python task-set token/time/tool-loop evidence. | local benchmark complete and deployed-report linked: 5 matched repo tasks showed bash 807.19ms avg wall time vs Python 129.28ms, estimated payload tokens bash 128 vs Python 221. Live LLM loop benchmark remains residual risk. |
| Chiron proof | First end-to-end inspect -> Try -> preview -> install -> uninstall/rollback proof. | deployed-verified through Try -> Verify -> Install -> Rollback on staging commit `75c80cd4b17e5403bf5f20ef835b4d42a0aea859`. |
| Dead-code cleanup | Candidate Desktop removed or refactored into used internal component; no dead island remains. | deployed-verified: `CandidateDesktopViewer.svelte` deleted; remaining `candidate-desktop` matches are mission docs or absence assertions. |
| Product proof | Staging desktop and 390x844 mobile proof, Trace/VText/run-acceptance evidence, rollback refs. | deployed-verified: screenshots/video/DOM metrics, rollback refs, mission VText dashboard, Chiron adoption proof, all-four VText reports, all-four portfolio report aggregation, all-four package-scoped accepted summaries, Liquid/Python benchmark links, rollback-only removal honesty, Chiron promotion-level run acceptance with rollback refs, and Apps & Changes -> Trace handoff. |

## Invariants

- Users do not paste package ids in ordinary UI.
- Package ids, source owner ids, refs, manifests, and digests remain available
  under technical details and evidence surfaces.
- Try never mutates the active computer.
- Install/promote mutates only after candidate build and verifier evidence.
- Uninstall must be honest. If source-level inverse removal cannot be verified,
  label the Change as `rollback-only` or `disable-only`; do not fake uninstall.
- Disable exists only when the installed Change explicitly supports a capability
  toggle or feature flag.
- Rollback refs must be named before any install claim.
- Candidate/Desktop preview machinery is internal to Apps & Changes or deleted.
- No old `export_patchset` or `/api/promotions` path is acceptable evidence.
- No binary copying between computers.
- VText is the mission dashboard and owner-readable report layer. Trace is the
  event ledger, not the primary human summary.
- Do not expose host/global telemetry in browser UI.
- Maintain logged-out read/explore and auth-on-mutation boundaries.
- Mobile remains the same floating-window desktop, not a phone-mode rewrite.

## Value Criterion

Maximize:

```text
owner-understandable change management
+ safe preview before install
+ real install/uninstall/rollback evidence
+ visible VText progress
+ technical traceability under details
+ zero dead candidate-desktop leftovers
```

while minimizing:

```text
raw developer concepts in ordinary UI
+ false uninstall confidence
+ platform-default mutation before review
+ local-only proof
+ duplicate app/package concepts
+ breadth loss from Chiron depth-first work
```

## Homotopy Axes

Increase realism along these axes while preserving topology:

1. **Catalog:** static seed list -> product-backed Change catalog -> published
   ecosystem.
2. **Preview:** package inspection -> candidate adoption preview -> live
   candidate desktop with verifier status.
3. **Install:** recipient build -> verified adoption -> active promotion with
   installed ledger.
4. **Uninstall:** rollback-only label -> disable where supported -> verified
   source-level uninstall preserving later changes.
5. **Evidence:** JSON refs -> VText report -> screenshots/video -> Trace detail
   -> run-acceptance.
6. **Review:** Chiron only -> four seeded experiments -> broader user-published
   changes.

Do not introduce a lower-resolution fake object. Every simplification must be a
projection of the real change-management path.

## VText Dashboard Requirement

At mission start, create or open a VText dashboard titled:

```text
Apps & Changes Store Sweep v0
```

Update it after every substantive transition:

- mission start and current hypothesis;
- current breadth ledger status;
- created/changed files;
- catalog and UI state;
- Chiron Try/Install/Uninstall state;
- Liquid/Python benchmark status;
- screenshots/video links;
- blockers and root-cause hypotheses;
- CI/deploy/staging identity;
- final certificate.

Each seeded Change also needs an owner-readable VText report before completion:

- what the Change does;
- package technical refs under a details section;
- source and recipient acceptance ids;
- screenshots/video;
- benchmark evidence where relevant;
- verifier results;
- install/uninstall/rollback status;
- recommendation and residual risks.

If VText cannot embed images/video yet, link the artifact paths and record that
embedding is a product gap.

## Implementation Notes

Potential code shape:

- `frontend/src/lib/AppsChangesApp.svelte`
- optional internal `frontend/src/lib/ChangePreviewFrame.svelte`
- small API/client helpers for Changes/catalog if existing package/adoption
  routes are too raw for product UI;
- tests under `frontend/tests/` proving catalog, Try, preview, install,
  rollback/uninstall labeling, and no Candidate Desktop launcher entry.

Naming preference:

- launcher app: **Apps & Changes**
- product object: **Change**
- hidden technical object: **AppChangePackage**

Avoid names that expose implementation:

- Candidate Desktop
- Package Installer
- AppChangePackage Store
- Promotion Queue

## Dense Feedback And Verification

Use product-path proof:

- Playwright desktop viewport;
- Playwright `390x844` mobile viewport;
- screenshots and video where possible;
- DOM metrics proving the catalog/detail/preview/install surfaces are visible
  and usable;
- VText dashboard and per-change VText reports;
- Trace events for package pull/adoption/verify/promote/rollback;
- run-acceptance where synthesis is meaningful;
- `rg "CandidateDesktopViewer|candidate-desktop"` cleanup proof;
- focused Go/frontend tests;
- staging `/health` commit identity after deploy.

For platform behavior changes, follow the repo landing loop:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

Docs-only updates do not require CI/CD.

## Rollback Policy

- Every install must name a rollback source ref before success is claimed.
- Apps & Changes must expose rollback for installed Changes.
- If uninstall is not verified, label the Change `rollback-only` or
  `disable-only` instead of showing a working uninstall button.
- Any platform code change can be rolled back by reverting the pushed commit and
  redeploying through the normal staging loop.
- Candidate/review computers created by Try can be discarded without mutating
  active computer state.

## Forbidden Shortcuts

- No ordinary UI asking users to paste package ids.
- No launcher-facing Candidate Desktop app.
- No dead preview component kept for hypothetical future use.
- No generic package admin console masquerading as a store.
- No `export_patchset`, `/api/promotions`, old promotion queue, or synthetic
  recipient digest evidence.
- No platform deploy used as proof that a user-computer install worked.
- No active-computer mutation during Try.
- No fake uninstall, fake disable, fake screenshots, fake thumbnails, or
  summary-only VText.
- No local-only proof for product claims.
- No hiding technical refs entirely; they belong in details/evidence.
- No Chiron depth-first polish before the breadth store substrate exists.

## Stopping Condition

Complete only when all of the following are true on deployed staging:

1. Apps & Changes replaces Candidate Desktop in the launcher/Desk.
2. The four seeded Changes are visible as human-readable catalog entries without
   package ids in ordinary UI.
3. Chiron can be inspected, tried in a candidate preview, verified, installed,
   and either uninstalled or honestly marked rollback-only/disable-only with a
   working rollback path.
4. The active computer is not mutated during Try.
5. Installed ledger shows Chiron with adoption/build/verifier/rollback evidence.
6. Candidate Desktop dead-code cleanup is proven by code search and tests.
7. VText dashboard and per-change VText reports exist and link screenshots/video
   and evidence.
8. Liquid and Python benchmark evidence is recorded or explicitly blocked by a
   named benchmark substrate issue after root-cause probes.
9. Desktop and mobile Playwright screenshots/DOM metrics prove the product path.
10. CI/deploy/staging identity and rollback refs are recorded.

If the stopping condition is not reached, report `checkpoint_incomplete` or
`blocked_incomplete`, update this mission doc with a resumable checkpoint, and
continue/redirect/delegate any safe executable next probe inside current
authority before stopping.

## Run Checkpoint And Resumption State

```text
status: complete
last checkpoint: deployed Apps & Changes plus first Chiron product-path
  adoption proof, all-four VText report surface, Liquid/Python benchmark links,
  a compact/mobile catalog overlap fix, an honest rollback-only removal
  model, a Chiron promotion-level run-acceptance record, selected Chiron
  Trace/run-acceptance surfacing from the owner-facing Change detail,
  all-four portfolio review aggregation, package-scoped source-computer
  accepted summaries for all four experiments, and rollback refs.
current artifact state: Apps & Changes exists as the launcher-facing Change
  catalog; Candidate Desktop app code is deleted; candidate preview survives
  only as internal ChangePreviewFrame used by Apps & Changes. Four seeded
  experiment Changes are visible as ordinary catalog cards, with package/source
  refs hidden under Technical refs. Apps & Changes can open/create the mission
  VText dashboard and per-change VText reports through product VText APIs. The
  selected Chiron Change now shows a Trace & acceptance panel and can open
  Trace focused to the relevant trajectory/run acceptance. The portfolio
  review panel aggregates all four experiment Changes with report/benchmark
  coverage and package-referenced accepted evidence from source computers,
  without exposing package ids in ordinary UI.
what shipped:
  - `e0a8f76954cb01a983c6d980b3e558fae45e06a0` Add Apps and Changes store surface.
  - `75c80cd4b17e5403bf5f20ef835b4d42a0aea859` Preserve Apps and Changes adoption state.
  - `a73affbc5c58121ceead49b8a8580b4247627fe6` Add Apps and Changes VText reports.
  - `efeb5d8fc926099ddbebf731d916f6dd83b54245` Link Apps and Changes benchmark evidence.
  - `2ea3deefa0108b9cc7307f2c7e64dbe58c3c295e` Fix Apps and Changes compact layout.
  - `a6767cf1436d18d2f144faad4ccb300ec8707b21` Expose Apps and Changes removal model.
  - `9bb9446b55588beabb63a750f8d25e93a692e074` Surface Apps and Changes trace acceptance evidence.
  - `22410dafff91cdc4edcddfa65ffa609c2973e928` Aggregate Apps and Changes portfolio evidence.
  - `737ea54ca439b3af838496807ce40e610acf2231` Surface package review evidence in Apps and Changes.
  - `2a5c297eab08f1816f105ea04e7b79bc7a32fdf6` Resolve package review evidence from source computers.
  - `38d143a7828be91290db5b83f024fbfdaa031ae0` Retry package review evidence loading.
what was proven:
  - GitHub Actions run `26197219323` passed and deployed to staging.
  - staging `/health` showed proxy and sandbox commit/deployed_commit
    `75c80cd4b17e5403bf5f20ef835b4d42a0aea859`.
  - deployed Playwright product proof opened Apps & Changes from the Desk
    launcher on a fresh account, found zero Candidate Desktop launcher entries,
    found one Apps & Changes entry, rendered four Changes on desktop and
    390x844 mobile, found no manual candidate id input, and kept ordinary UI
    free of package UUIDs before Technical refs.
  - Chiron Try created product adoption
    `adoption-chiron-shelf-62e544c3-4d3c-484a-9638-317fe964f554` and candidate
    `candidate-chiron-shelf-c0ec9010-bf57-45a0-bfb9-954050dd6638`.
  - Chiron Verify returned `verified` with verifier contracts including
    source refs, source ledger provenance, manifest hash, no cross-computer
    binary copying, foreground-tail accounting, and actual recipient runtime/UI
    build.
  - recipient artifact digests were:
    runtime `sha256:8de3d34c476d819e872312193b360e76b3dc086a8df99ad4606a0ad52d20dd3d`
    and UI `sha256:b2367c43c9e0b2d31eb51894237b3bdfef3fe9bfae040bb8e6f2e27972209024`.
  - Chiron Install returned `adopted`; Rollback returned `rolled_back` with a
    rollback profile naming previous active source ref `refs/computers/primary/active`
    and previous route profile `route:primary`.
  - GitHub Actions run `26198364649` passed and deployed commit
    `a73affbc5c58121ceead49b8a8580b4247627fe6`.
  - staging `/health` showed proxy and sandbox commit/deployed_commit
    `a73affbc5c58121ceead49b8a8580b4247627fe6`.
  - focused local checks passed:
    `npm --prefix frontend run build` and
    `cd frontend && npx playwright test tests/web-surface-rationalization.spec.js tests/trace-settings-registry.spec.js --project=chromium`.
  - deployed Playwright product proof opened Apps & Changes on desktop and
    390x844 mobile, found four Change cards, found the mission VText and
    per-change report actions, confirmed Technical refs stayed collapsed, and
    confirmed ordinary UI did not expose the Chiron package id.
  - the same proof created/opened mission VText document
    `65e11994-79fa-4813-b8d9-f505013e800d` on desktop, with the mission
    checkpoint and seeded Change status text.
  - the same proof created/opened Chiron report VText documents on desktop
    (`8c8a64b3-de27-4076-8245-e526c67a9cd5`) and mobile
    (`6932829f-d017-492b-be5a-69307a4cbee2`), containing Chiron summary,
    recommendation, source/recipient acceptance ids, benchmark status,
    package technical ref, and product-gap note for embedded media.
  - isolated package benchmark evidence was created under
    `test-results/apps-changes-benchmarks-2026-05-21T01-00-45-3NZ/`.
    Liquid rendered through WebGL in Chromium/WebKit at desktop and 390x844
    with avg frame time 16.66-16.67ms and p95 <= 18.1ms. Python code-mode
    primitive A/B across 5 matched repo tasks measured bash 807.19ms average
    wall time vs Python 129.28ms, with estimated input payload tokens bash 128
    vs Python 221; focused runtime tests passed in the repo dev shell.
  - GitHub Actions run `26199378174` passed and deployed commit
    `efeb5d8fc926099ddbebf731d916f6dd83b54245`.
  - the first deployed all-four report proof at `efeb5d8` found a real mobile
    Apps & Changes bug: in the compact layout, the selected detail pane
    intercepted clicks on visible catalog cards.
  - commit `2ea3deefa0108b9cc7307f2c7e64dbe58c3c295e` fixed the compact layout
    by making it an explicit vertical flow and added a focused 390x844
    regression test.
  - local checks passed after that fix:
    `npm --prefix frontend run build` and
    `cd frontend && npx playwright test tests/web-surface-rationalization.spec.js tests/trace-settings-registry.spec.js --project=chromium`
    with 9 passing tests.
  - GitHub Actions run `26199796372` passed and deployed commit
    `2ea3deefa0108b9cc7307f2c7e64dbe58c3c295e`.
  - deployed proof
    `test-results/apps-changes-benchmark-reports-staging-2026-05-21T01-33-57-228Z/apps-changes-benchmark-reports-proof.json`
    passed on desktop and 390x844 mobile: four catalog cards, no visible
    package ids in ordinary UI, all four reports opened through product VText,
    package refs and manifest hashes present inside reports, and Liquid/Python
    benchmark artifact links present in the owner-readable reports.
  - commit `a6767cf1436d18d2f144faad4ccb300ec8707b21` fixed a false-evidence
    edge: empty rollback-profile JSON no longer counts as rollback evidence,
    and Apps & Changes now exposes a selected Change's removal model instead
    of implying uninstall/disable support.
  - local checks passed after that fix:
    `npm --prefix frontend run build` and
    `cd frontend && npx playwright test tests/web-surface-rationalization.spec.js tests/trace-settings-registry.spec.js --project=chromium`
    with 10 passing tests.
  - GitHub Actions run `26200571636` passed and deployed commit
    `a6767cf1436d18d2f144faad4ccb300ec8707b21`.
  - staging `/health` showed proxy and sandbox commit/deployed_commit
    `a6767cf1436d18d2f144faad4ccb300ec8707b21`.
  - deployed product proof
    `test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/apps-changes-removal-model-proof.json`
    passed on desktop and 390x844 mobile after a real Chiron pull -> Try ->
    recipient build Verify -> Install flow. It recorded adoption
    `adoption-chiron-shelf-b8d4c4d9-c787-4fe0-9f2b-c26bacb57efb`,
    candidate `candidate-chiron-shelf-c732c878-35fc-4026-acde-a379bf6f4794`,
    recipient runtime digest
    `sha256:194a0b412998a1a373e9c489717181578012ed3edd7a9a1f71cd9f4e68a8879f`,
    recipient UI digest
    `sha256:b2367c43c9e0b2d31eb51894237b3bdfef3fe9bfae040bb8e6f2e27972209024`,
    rollback profile previous active source ref `refs/computers/primary/active`,
    and previous route profile `route:primary`.
  - the same proof confirmed the ordinary removal UI says `Rollback-only`,
    keeps Rollback enabled, keeps Uninstall disabled because there is no
    verified inverse source patch, keeps Disable disabled because there is no
    declared feature flag/capability toggle, and still hides package IDs from
    ordinary UI before Technical refs.
  - product run-acceptance synthesis
    `test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/apps-changes-removal-run-acceptance-proof.json`
    returned HTTP 202 with accepted `promotion-level` record
    `runacc-e89094a0f29869807b09` for trajectory
    `apps-changes-chiron-shelf`. Invariant checks passed for product-path
    observation, bounded worker mutation, non-overclaiming promotion, and
    checkpoint causal order.
  - commit `9bb9446b55588beabb63a750f8d25e93a692e074` added owner-facing
    Trace/run-acceptance surfacing to Apps & Changes and an Apps & Changes
    -> Trace handoff that opens Trace focused to the selected Change's
    trajectory and acceptance.
  - local checks passed after that fix:
    `npm --prefix frontend run build`,
    `cd frontend && npx playwright test tests/web-surface-rationalization.spec.js --project=chromium -g "rollback-only removal"`,
    `cd frontend && npx playwright test tests/web-surface-rationalization.spec.js tests/trace-settings-registry.spec.js --project=chromium`,
    and `git diff --check`.
  - GitHub Actions run `26202379885` passed and deployed commit
    `9bb9446b55588beabb63a750f8d25e93a692e074`.
  - staging `/health` showed proxy and sandbox commit/deployed_commit
    `9bb9446b55588beabb63a750f8d25e93a692e074`, built at
    `20260521024542`, deployed at `2026-05-21T02:47:25Z`.
  - deployed product proof
    `test-results/apps-changes-trace-surfacing-staging-2026-05-21T02-58-41-000Z/apps-changes-trace-surfacing-proof.json`
    passed on desktop and `390x844` mobile. It recorded account
    `apps-changes-trace-trace-surfacing-mpexedqq@example.com`, package
    `28433c19-5d02-416f-9368-de56390e1927`, adoption
    `adoption-chiron-trace-trace-surfacing-mpexedqq`, candidate
    `candidate-chiron-trace-trace-surfacing-mpexedqq`, trajectory
    `apps-changes-chiron-shelf-trace-surfacing-mpexedqq`, and accepted
    `promotion-level` run acceptance `runacc-2ec3b0a57b8ac4f0bc05`.
  - the same proof recorded a real recipient build with runtime digest
    `sha256:d764e5a1f56f1f781d0d453619d55370228e9ecc2463241f242dc7072fca0c84`,
    UI digest
    `sha256:b2367c43c9e0b2d31eb51894237b3bdfef3fe9bfae040bb8e6f2e27972209024`,
    base source SHA `575ff3014a85524da4233e60ce44345804d46807`,
    head source SHA `5f46838346e861a2e3f0265f380f5f8a60ff8437`,
    runtime build duration `5m21.378980548s`, and UI build duration
    `7.999625224s`.
  - the same proof captured desktop and mobile screenshots of Apps & Changes
    trace surfacing and the focused Trace handoff:
    `desktop-apps-changes-trace-panel.png`,
    `desktop-trace-opened-from-change.png`,
    `mobile-390x844-apps-changes-trace-panel.png`, and
    `mobile-390x844-trace-opened-from-change.png`.
  - proof-harness learning: the first mobile follow-up reused storage captured
    before a long recipient build; Install later renewed and rotated the
    refresh cookie, making that saved storage stale. The passing proof captures
    storage after Install and clears only saved window state before the mobile
    check.
  - commit `22410dafff91cdc4edcddfa65ffa609c2973e928` added the portfolio
    review panel and made run-acceptance synthesis carry adoption rollback
    refs from promotion/rollback events.
  - local checks passed after that fix:
    `npm --prefix frontend run build`,
    `cd frontend && npx playwright test tests/web-surface-rationalization.spec.js --project=chromium -g "portfolio reports"`,
    `cd frontend && npx playwright test tests/web-surface-rationalization.spec.js tests/trace-settings-registry.spec.js --project=chromium`,
    `nix develop -c go test ./internal/runtime -run 'TestRunAcceptanceSynthesize(RequiresAdoptionPromotionForPromotionLevel|AcceptsDirectProductAdoptionEvidence)$'`,
    and `git diff --check`.
  - GitHub Actions run `26204257440` passed and deployed commit
    `22410dafff91cdc4edcddfa65ffa609c2973e928`.
  - staging `/health` showed proxy and sandbox commit/deployed_commit
    `22410dafff91cdc4edcddfa65ffa609c2973e928`, built at
    `20260521034811`, deployed at `2026-05-21T03:49:58Z`.
  - deployed product proof
    `test-results/apps-changes-portfolio-aggregation-staging-2026-05-21T03-55-47-000Z/apps-changes-portfolio-aggregation-proof.json`
    passed on desktop and `390x844` mobile. It recorded account
    `apps-changes-portfolio-portfolio-aggregation-mpezvpo8@example.com`,
    adoption `adoption-chiron-portfolio-portfolio-aggregation-mpezvpo8`,
    candidate `candidate-chiron-portfolio-portfolio-aggregation-mpezvpo8`,
    trajectory `apps-changes-chiron-shelf-portfolio-aggregation-mpezvpo8`,
    and accepted `promotion-level` run acceptance
    `runacc-fa74b7932d330ba7f04d`.
  - the same proof recorded a real recipient build with runtime digest
    `sha256:28353f90b25e4c9092180f24e080aa8d55dc87beae8740811a5ecf944284300d`,
    UI digest
    `sha256:b2367c43c9e0b2d31eb51894237b3bdfef3fe9bfae040bb8e6f2e27972209024`,
    base source SHA `575ff3014a85524da4233e60ce44345804d46807`,
    head source SHA `90db78e095f9487c4ebb1efe73580a3e8b4c5edc`,
    runtime build duration `5m22.71091975s`, and UI build duration
    `7.927733702s`.
  - the same proof confirmed the portfolio review panel showed four Changes,
    four reports, four benchmark/media links, and one accepted record; ordinary
    portfolio UI did not expose the Chiron package id.
  - the same proof captured desktop and mobile screenshots:
    `desktop-apps-changes-portfolio.png`, `desktop-portfolio-vtext.png`,
    `desktop-trace-from-portfolio.png`, and
    `mobile-390x844-apps-changes-portfolio.png`.
  - proof-harness learning: long recipient builds can outlive the short browser
    auth session; product proof must renew through `/auth/session` before the
    promote step. The passing proof used that normal product renewal path.
  - commit `737ea54ca439b3af838496807ce40e610acf2231` added the package-scoped
    review-evidence API, source/recipient acceptance summarization, redacted
    cross-owner summaries, and Apps & Changes frontend loading for package
    review evidence. Focused local checks passed:
    `nix develop .# --command go test -count=1 ./internal/store ./internal/runtime -run 'TestAppChangePackageReviewEvidenceReturnsRedactedPackageScopedAcceptances|TestPrivateAppChangePackageIsNotVisibleAcrossOwners|TestRunAcceptance'`,
    `npm --prefix frontend run build`, and
    `npm --prefix frontend run e2e -- web-surface-rationalization.spec.js`.
  - GitHub Actions run `26206251744` passed and deployed commit
    `737ea54ca439b3af838496807ce40e610acf2231`.
  - commit `2a5c297eab08f1816f105ea04e7b79bc7a32fdf6` made the proxy resolve
    `/api/app-change-packages/{package_id}/review-evidence` from the package's
    source computer using `source_owner_id` and `source_desktop_id`, while the
    source runtime still returns only redacted package-scoped summaries.
    Focused local checks passed:
    `npm --prefix frontend run build`,
    `nix develop .# --command go test -count=1 ./internal/proxy ./internal/runtime ./internal/store -run 'TestAppChangePackageReviewEvidenceFetchesSourceComputerSummary|TestAppChangePackagePullImportsPackageIntoTargetComputer|TestAppChangePackageReviewEvidenceReturnsRedactedPackageScopedAcceptances|TestRunAcceptance'`,
    and `npm --prefix frontend run e2e -- web-surface-rationalization.spec.js`.
  - GitHub Actions run `26206864112` passed and deployed commit
    `2a5c297eab08f1816f105ea04e7b79bc7a32fdf6`.
  - deployed proof against `2a5c297e` found a real frontend resilience gap:
    direct product API calls could load package-scoped accepted summaries for
    all four experiments, but a fresh Apps & Changes portfolio sometimes
    rendered only 3/4 accepted rows when one source-computer request lost a
    cold-start/race. This was not solved by a manual refresh; it required
    product-side bounded retry.
  - commit `38d143a7828be91290db5b83f024fbfdaa031ae0` added bounded automatic
    review-evidence retries in Apps & Changes and incremental row updates.
    Local `npm --prefix frontend run build` passed. The first focused browser
    suite run had one desktop fixture setup timeout before app interaction; the
    immediate rerun of
    `npm --prefix frontend run e2e -- web-surface-rationalization.spec.js`
    passed with 9/9 tests.
  - GitHub Actions run `26207519231` passed and deployed commit
    `38d143a7828be91290db5b83f024fbfdaa031ae0`; the deploy job completed in
    5m27s.
  - staging `/health` showed proxy and sandbox commit/deployed_commit
    `38d143a7828be91290db5b83f024fbfdaa031ae0`, built at
    `20260521053321`, deployed at `2026-05-21T05:35:08Z`.
  - deployed product proof
    `test-results/apps-changes-review-evidence-staging-2026-05-21T05-02-34-000Z/apps-changes-review-evidence-proof.json`
    passed on desktop and `390x844` mobile. It recorded `result: pass`, proxy
    and sandbox commit `38d143a7828be91290db5b83f024fbfdaa031ae0`, and
    package-scoped review evidence API status 200 for all four experiments:
    Chiron `runacc-a352091712fdd96aa00d`, Motion
    `runacc-5784f0028b01753ad0ca`, Liquid
    `runacc-0194bfce2cdecffea784`, and Python
    `runacc-a7e993d7c4f56d4420d9`.
  - the same proof showed Apps & Changes DOM metrics with four Changes, four
    reports, four accepted records, no visible package IDs, redacted
    `package-referenced` summaries for cross-owner evidence, disabled
    non-shared Trace buttons labeled `Summary`, a portfolio VText screenshot,
    and desktop/mobile screenshots plus Playwright video.
unproven or residual claims: polished installed history; true source-level
  uninstall/disable beyond rollback-only labeling; inline media embedding in
  VText; owner hands-on QA for the four experiments; durable product-backed
  publication/catalog records instead of hard-coded seed metadata;
  continuation-level acceptance.
belief-state changes: package ids are implementation details; user-facing
  object is Change; Candidate Desktop should be removed, not preserved as a
  public island; Settings is low-level evidence, not the ordinary install
  surface; product proof can now cross the AppChangePackage -> recipient build
  -> adoption -> rollback boundary without a package-id paste UI.
remaining error field: the Apps & Changes mission stopping condition is now
  satisfied on staging, but the next realism frontier is turning the seeded
  catalog into durable publication/discovery records, adding true inverse
  uninstall or feature-disable contracts for Changes that support them,
  embedding media in VText, and raising acceptance from product/staging proof
  toward continuation-level proof.
highest-impact remaining uncertainty: can Apps & Changes become a durable
  owner review surface backed by real publication records, not hard-coded
  package seed metadata, while preserving source-computer privacy and recipient
  build/adoption semantics?
next executable probe: move the four experiment Change records out of frontend
  seed constants into durable product-backed catalog/publication records, then
  add a tiny reversible Change with a verified inverse uninstall or feature
  flag disable contract.
suggested resume goal string: define a new mission around durable Change
  publication records plus one verified inverse uninstall/disable contract.
evidence artifact refs:
  - historical alternate-computer portfolio docs pruned during Campaign
    Compiler cleanup
  - test-results/apps-changes-store-staging-2026-05-21T00-16-29-145Z/apps-changes-staging-proof.json
  - test-results/apps-changes-store-staging-2026-05-21T00-16-29-145Z/desktop-apps-changes-open.png
  - test-results/apps-changes-store-staging-2026-05-21T00-16-29-145Z/mobile-apps-changes-open-390x844.png
  - test-results/apps-changes-store-staging-2026-05-21T00-16-29-145Z/desktop-chiron-after-try-ready.png
  - test-results/apps-changes-store-staging-2026-05-21T00-16-29-145Z/desktop-chiron-after-verify.png
  - test-results/apps-changes-store-staging-2026-05-21T00-16-29-145Z/desktop-chiron-after-install.png
  - test-results/apps-changes-store-staging-2026-05-21T00-16-29-145Z/desktop-chiron-after-rollback.png
  - test-results/apps-changes-store-staging-2026-05-21T00-16-29-145Z/mobile-chiron-final-390x844.png
  - test-results/apps-changes-store-staging-2026-05-21T00-16-29-145Z/page@77114092c565b67b41926d6d58479761.webm
  - test-results/apps-changes-vtext-report-staging-2026-05-21T00-50-49-966Z/apps-changes-vtext-report-proof.json
  - test-results/apps-changes-vtext-report-staging-2026-05-21T00-50-49-966Z/desktop-apps-changes-vtext-actions.png
  - test-results/apps-changes-vtext-report-staging-2026-05-21T00-50-49-966Z/desktop-mission-vtext-dashboard.png
  - test-results/apps-changes-vtext-report-staging-2026-05-21T00-50-49-966Z/desktop-chiron-vtext-report.png
  - test-results/apps-changes-vtext-report-staging-2026-05-21T00-50-49-966Z/mobile-apps-changes-vtext-actions-390x844.png
  - test-results/apps-changes-vtext-report-staging-2026-05-21T00-50-49-966Z/mobile-chiron-vtext-report-390x844.png
  - test-results/apps-changes-vtext-report-staging-2026-05-21T00-50-49-966Z/page@e01984cda35c79689a657542692805ba.webm
  - test-results/apps-changes-vtext-report-staging-2026-05-21T00-50-49-966Z/page@c8b0812f9cb8b4edb0d08d3d96384cfb.webm
  - test-results/apps-changes-all-reports-staging-2026-05-21T00-58-41-312Z/apps-changes-all-reports-proof.json
  - test-results/apps-changes-benchmarks-2026-05-21T01-00-45-3NZ/liquid-material-benchmark.json
  - test-results/apps-changes-benchmarks-2026-05-21T01-00-45-3NZ/python-code-mode-ab-benchmark.json
  - test-results/apps-changes-benchmark-reports-staging-2026-05-21T01-33-57-228Z/apps-changes-benchmark-reports-proof.json
  - test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/apps-changes-removal-model-proof.json
  - test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/desktop-chiron-removal-model.png
  - test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/mobile-390x844-chiron-removal-model.png
  - test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/page@a557298aa1e00a37ee63b7880179a5fd.webm
  - test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/page@fc3398eae4f24d97dc856c05a69cfef0.webm
  - test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/apps-changes-removal-run-acceptance-proof.json
  - test-results/apps-changes-trace-surfacing-staging-2026-05-21T02-58-41-000Z/apps-changes-trace-surfacing-proof.json
  - test-results/apps-changes-trace-surfacing-staging-2026-05-21T02-58-41-000Z/desktop-apps-changes-trace-panel.png
  - test-results/apps-changes-trace-surfacing-staging-2026-05-21T02-58-41-000Z/desktop-trace-opened-from-change.png
  - test-results/apps-changes-trace-surfacing-staging-2026-05-21T02-58-41-000Z/mobile-390x844-apps-changes-trace-panel.png
  - test-results/apps-changes-trace-surfacing-staging-2026-05-21T02-58-41-000Z/mobile-390x844-trace-opened-from-change.png
  - test-results/apps-changes-portfolio-aggregation-staging-2026-05-21T03-55-47-000Z/apps-changes-portfolio-aggregation-proof.json
  - test-results/apps-changes-portfolio-aggregation-staging-2026-05-21T03-55-47-000Z/desktop-apps-changes-portfolio.png
  - test-results/apps-changes-portfolio-aggregation-staging-2026-05-21T03-55-47-000Z/desktop-portfolio-vtext.png
  - test-results/apps-changes-portfolio-aggregation-staging-2026-05-21T03-55-47-000Z/desktop-trace-from-portfolio.png
  - test-results/apps-changes-portfolio-aggregation-staging-2026-05-21T03-55-47-000Z/mobile-390x844-apps-changes-portfolio.png
  - test-results/apps-changes-review-evidence-staging-2026-05-21T05-02-34-000Z/apps-changes-review-evidence-proof.json
  - test-results/apps-changes-review-evidence-staging-2026-05-21T05-02-34-000Z/desktop-apps-changes-review-evidence.png
  - test-results/apps-changes-review-evidence-staging-2026-05-21T05-02-34-000Z/desktop-liquid-review-summary.png
  - test-results/apps-changes-review-evidence-staging-2026-05-21T05-02-34-000Z/desktop-portfolio-vtext-4-of-4.png
  - test-results/apps-changes-review-evidence-staging-2026-05-21T05-02-34-000Z/mobile-apps-changes-review-evidence-390x844.png
  - test-results/apps-changes-review-evidence-staging-2026-05-21T05-02-34-000Z/page@a89aecc42a9c044afd573ffc41bdba23.webm
  - frontend tests:
    `cd frontend && npx playwright test tests/computer-live-sync-hard-cutover.spec.js tests/web-surface-rationalization.spec.js tests/trace-settings-registry.spec.js --project=chromium`
  - build: `npm --prefix frontend run build`
rollback refs: code rollback by reverting `38d143a7828be91290db5b83f024fbfdaa031ae0`,
  `2a5c297eab08f1816f105ea04e7b79bc7a32fdf6`,
  `737ea54ca439b3af838496807ce40e610acf2231`,
  `22410dafff91cdc4edcddfa65ffa609c2973e928`,
  `9bb9446b55588beabb63a750f8d25e93a692e074`,
  `2ea3deefa0108b9cc7307f2c7e64dbe58c3c295e`,
  `a6767cf1436d18d2f144faad4ccb300ec8707b21`,
  `efeb5d8fc926099ddbebf731d916f6dd83b54245`,
  `a73affbc5c58121ceead49b8a8580b4247627fe6`,
  `75c80cd4b17e5403bf5f20ef835b4d42a0aea859`, and
  `e0a8f76954cb01a983c6d980b3e558fae45e06a0`; Chiron adoption rollback profile
  recorded previous active source ref `refs/computers/primary/active` and
  previous route profile `route:primary`.
```
