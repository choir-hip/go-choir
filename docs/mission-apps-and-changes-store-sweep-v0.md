# MissionGradient: Apps & Changes Store Sweep v0

**Status:** draft
**Date:** 2026-05-20
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)
**Prior portfolio:** [mission-alternate-computer-ux-experiment-portfolio-v0.md](mission-alternate-computer-ux-experiment-portfolio-v0.md)
**Portfolio certificate:** [alternate-computer-ux-experiment-portfolio-certificate-2026-05-20.md](alternate-computer-ux-experiment-portfolio-certificate-2026-05-20.md)
**Portfolio epoch review:** [alternate-computer-ux-experiment-portfolio-epoch-review-2026-05-20.md](alternate-computer-ux-experiment-portfolio-epoch-review-2026-05-20.md)

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
- update [platform-os-app-state.md](platform-os-app-state.md) to replace
  Candidate Desktop with Apps & Changes.

No "maybe later" code. If it is not wired into the new flow, remove it.

## Breadth Task Ledger

Keep this ledger current during the mission. Do not go depth-first on Chiron
until the breadth substrate exists.

| Area | Required Outcome | Status |
| --- | --- | --- |
| Product model | User-facing terms and lifecycle are encoded in docs/UI/API names. | checkpoint: UI and platform-state docs use Change/Apps & Changes; deeper API names remain AppChangePackage/adoption. |
| Catalog | Four seed Changes appear without package ids in ordinary UI. | local-verified: four cards seeded; package/source refs are in collapsed technical details. |
| Apps & Changes UI | Replaces Candidate Desktop in launcher/Desk. | local-verified: `apps-changes` replaces `candidate-desktop`; launcher absence is tested. |
| Change detail | Shows summary, screenshots/video, VText, Trace, verification, risks, compatibility, and collapsed technical refs. | partial: summary, proof text, action state, candidate/build refs, rollback status, and collapsed technical refs exist; media/VText/report links still pending. |
| Try flow | Creates candidate/review adoption without mutating active computer. | local-verified with mocked product endpoints; staging Chiron proof pending. |
| Preview | Opens candidate/review desktop from the selected Change. | local-verified through internal `ChangePreviewFrame`; staging real candidate proof pending. |
| Install | Promotes verified candidate into active computer with rollback refs. | pending |
| Uninstall | Honestly supports inverse/remove flow when safe or marks rollback-only/disable-only. | partial: rollback is exposed; separate uninstall is intentionally not faked. |
| Disable | Represents feature-flag/capability-disable only when supported. | pending: no disable support claimed. |
| Installed ledger | Shows installed Changes and action availability. | local-verified structurally; real installed records pending staging proof. |
| VText dashboard | Live mission VText updated on substantive changes. | pending |
| Per-change VTexts | Chiron, Motion, Liquid, Python each get owner-readable reports. | pending |
| Screenshots/video | All four Changes have review media linked from detail/VText. | pending |
| Liquid benchmarks | WebGL/WebKit/mobile/desktop resource/frame evidence. | pending |
| Python benchmarks | Matched bash-vs-Python task-set token/time/tool-loop evidence. | pending |
| Chiron proof | First end-to-end inspect -> Try -> preview -> install -> uninstall/rollback proof. | pending |
| Dead-code cleanup | Candidate Desktop removed or refactored into used internal component; no dead island remains. | local-verified: `CandidateDesktopViewer.svelte` deleted; remaining `candidate-desktop` matches are mission docs or absence assertions. |
| Product proof | Staging desktop and 390x844 mobile proof, Trace/VText/run-acceptance evidence, rollback refs. | pending |

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
status: checkpoint_incomplete
last checkpoint: local breadth substrate implemented and verified before
  staging landing.
current artifact state: Apps & Changes exists as the launcher-facing Change
  catalog; Candidate Desktop app code is deleted; candidate preview survives
  only as internal ChangePreviewFrame used by Apps & Changes.
what shipped: nothing from this mission yet; local implementation is ready for
  commit/push/CI/deploy once the next proof slice is selected.
what was proven: frontend build passed; `git diff --check` passed; targeted
  Playwright passed for no browser storage/manual refresh regressions,
  Apps & Changes launcher replacement, no manual candidate-id UI, mocked Try
  candidate preview, Settings/Trace product-safety, and no browser-internal
  promotion route use.
unproven or partial claims: real staging Chiron pull/Try/verify/install/
  rollback; VText mission dashboard and per-change reports; Liquid/Python
  benchmarks; screenshots/video links in the Change detail; mobile 390x844
  deployed proof.
belief-state changes: package ids are implementation details; user-facing
  object is Change; Candidate Desktop should be removed, not preserved as a
  public island; Settings is low-level evidence, not the ordinary install
  surface.
remaining error field: store/review substrate now exists locally, but the
  mission has not yet proven real Chiron adoption on staging or made VText the
  live dashboard.
highest-impact remaining uncertainty: can the deployed store run Chiron
  through pull -> candidate adoption -> recipient build verification ->
  install/promote -> rollback without package-id UI or active mutation during
  Try?
next executable probe: commit/push this breadth substrate, monitor CI/deploy,
  verify staging identity, then run Chiron through real product-path Try,
  recipient build verification, install, and rollback on desktop and mobile.
suggested resume goal string: use the One-Line Goal String in this document.
evidence artifact refs:
  - docs/alternate-computer-ux-experiment-portfolio-certificate-2026-05-20.md
  - docs/alternate-computer-ux-experiment-portfolio-epoch-review-2026-05-20.md
  - frontend tests:
    `cd frontend && npx playwright test tests/computer-live-sync-hard-cutover.spec.js tests/web-surface-rationalization.spec.js tests/trace-settings-registry.spec.js --project=chromium`
  - build: `npm --prefix frontend run build`
rollback refs: none yet for live install; code rollback is the pre-commit diff
  if not landed, or the eventual commit SHA after landing.
```
