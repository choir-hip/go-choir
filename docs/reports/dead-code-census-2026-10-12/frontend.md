# Dead-code census: frontend/ (read-only; nothing was edited)

Date: 2026-10-12. Repo HEAD c30147a6. Area: `/Users/wiz/go-choir/frontend` (src 85 files / 38,121 lines; tests 79 files / 22,033 lines; 11 root `*.mjs` scripts; public/; configs).

## Method

1. Import graph from the real entry (`index.html` -> `src/main.js` -> `App.svelte`) with a small node script (static `import`/`from`/`@import`, plus the `() => import('../X.svelte')` lazy imports in `src/lib/apps/registry.ts`). The registry is the only place components are resolved by id; every `component:` is a string-literal dynamic import, so Vite resolves them statically and there is no string-built component lookup anywhere. Result: 85 src files, **10 unreachable** (2 components + 6 primitives + README + 0 others). The first pass wrongly flagged CalendarApp; that was a regex artifact (it is imported from registry.ts line 225 and is live).
2. Export-usage scan: every `export` in src `.js/.ts` counted against all other src files, then against `tests/` and `docs/`.
3. Registry audit: each of the 18 registered apps checked for a launcher path (DeskSheet via `launcher.desk`, desktop icons via `launcher.desktopIcon`, `content` opened by `sourceEntityLaunchPayload`) and for the backend routes it calls. Every `/api/...` and `/auth/...` string in src was grepped against Go route registrations (`internal/apihandler/routes.go`, `internal/proxy/handlers.go`, `internal/textureowner/router.go`, `internal/maild`, `internal/autoputer`, `internal/platform`). **No registered app is backed by a dead Go route.** All of them have live routes today (including `/api/universal-wire/stories`, which exists until Cut 2 deletes it).
4. Store audit (`stores/desktop.js`, `stores/terminal.js`): every exported store/derived checked for writer and reader.
5. Playwright: `data-*` selectors, app ids, and `/api/*` strings in every spec checked against src and Go. `playwright.config.js` has only `testDir: './tests'` (no testMatch/testIgnore, no webServer). **No GitHub workflow runs Playwright** (`grep -i playwright .github/workflows/*` is empty; the CI frontend job is only `pnpm build`, which also runs `generate-source-contract.mjs --check`). Specs run by hand (README line 145: `pnpm exec playwright test --workers=1`) against localhost:4173 or deployed `choir.news`. That is why they rotted without any signal. No `*.tmp.spec.js` files exist.
6. CSS: every `.class` in `app.css`, `desktop-motion.css`, `texture-source-flow.css` searched for in non-CSS src.

## Totals

| Kind | Count / lines |
|---|---|
| Unreachable src files | 8 .svelte (DesktopIcons 240, Window 383, 6 apps/* primitives 33) + `apps/README.md` (16) = ~672 lines |
| Reachable but retired-by-Cut-2 | UniversalWireApp 666 + App.svelte legacy reader ~225 + TextureCompareMergePanel 281 + merge wiring ~290 |
| Dead exports (zero importers anywhere) | 15 symbols (~170 lines), see item 8 |
| Dead CSS selectors in `app.css` | ~38 class names (theme override lists for removed apps) |
| Playwright specs testing removed UI | ~7,500 lines across 17 specs (stale positively-asserted selectors), plus 758 lines of Universal Wire specs |
| Orphan root scripts | 11 `.mjs`, 760 lines |
| Unused assets | none found (`public/` = desktop-bridge.html (live: `cmd/desktop` loads it), legal md (live: LegalDocument)) |

Items are sorted by size (lines removable).

---

## 1. Stale Playwright specs (selectors for UI removed on 2026-05-29)

- **Paths** (`frontend/tests/`): `desktop-shell-core.spec.js` 934, `file-browser.spec.js` 726, `desktop-state-persistence.spec.js` 817, `deploy-readiness-browser-contract.spec.js` 746, `logout-user-switch.spec.js` 471, `register-login-shell.spec.js` 408, `session-rehydration-renewal.spec.js` 405, `floating-window.spec.js` 518, `desktop-window-manager.spec.js` 373, `texture-markdown-lineage.spec.js` 801, `mobile-real-desktop-overview.spec.js` 300, `desktop-overview-heavy-session.spec.js` 266, `mounted-shell-renewal.spec.js` 226, `shell-ui.spec.js` 183, `compute-monitor-app.spec.js` 165, `trace-settings-registry.spec.js` 137, `save-cookies.spec.js` 48. About 7,500 lines in total. Whole files are not always dead; the stale tests inside them are.
- **What they did**: Product-path acceptance for the desktop shell: Start menu launch, window switcher, shell logout/user area, live-status dot, desk button, left rail.
- **Why dead**: Commit `80823eba` (2026-05-29, "frontend: prepare mainline redesign cutover") removed these DOM hooks from src: `data-start-button`, `data-start-menu`, `data-start-app-id`, `data-window-switcher`, `data-window-indicator(-active)`, `data-shell-user`, `data-shell-logout`, `data-shell-live-status`, `data-connection-status`, `data-desk-button`, `data-desktop-logout`, `data-features-window`. They are asserted **positively** (click/toBeVisible) in the specs above, e.g. `desktop-shell-core.spec.js:156` (`[data-start-button]`), `:743` (`[data-shell-logout]`), `:254/:663` (window indicators/switcher), `file-browser.spec.js:385`, `desktop-overview-heavy-session.spec.js:53`, `mobile-real-desktop-overview.spec.js:25` (`[data-desk-button]`; src now has `data-desk-menu-button`), `desktop-window-manager.spec.js:52` (`[data-desktop-logout]`), `trace-settings-registry.spec.js:26` (`openStartApp`), `texture-markdown-lineage.spec.js:45` (`[data-rail-item]`; the rail was removed 2026-04 and `desktop-shell-core` itself asserts it is absent). Other drift: app ids `terminal` (`floating-window.spec.js:406,474`) and `features` (`desktop-state-persistence.spec.js:224-278`, `compute-monitor-app.spec.js:121,163`) are not in the registry any more (now `super-console`, `compute-monitor`); `desktop-shell-core.spec.js:148` has a rename-damage typo `.titltexture`; `save-cookies.spec.js` waits for text "Register with Passkey" (0 hits in src) and writes `/tmp/cookies_*.json`. Many other specs keep `toHaveCount(0)` assertions for removed things (`data-desktop-icon-id="trace"`, `data-hamburger-btn`, `data-shell-bootstrap`, etc.). Those are valid negative regression checks and are not dead.
- **Last touched**: several were edited 2026-09-24 by the bulk rename `66981cef` ("R1: desk-RLM vocabulary cutover") without being run, so mtime does not mean healthy.
- **Citers**: README.md:145 (command only); docs corpus mentions. `reports/choir-whole-system-architecture-and-invention-review-2026-08-18.md:331` already flags "stale test scripts". No CI.
- **Already scheduled?**: no. (Cut 2 only says the Universal Wire specs are "replaced".)
- **Proposal**: owner question, then remove-or-rewrite per test, not per file. The stale tests are the ones that touch the selectors above. Healthy ones that exercise live selectors (`data-desk-menu-button`, `data-desk-sheet`, `data-desktop-icon-id`, `data-prompt-surface`) should stay. Before the re-engineering, decide whether Playwright is the acceptance mechanism at all (CLAUDE.md says "highly prefer E2E"), because if so it should run somewhere (CI has no job for it).
- **Risk**: `helpers/auth.js`, `helpers/fixtures.js`, `helpers/webauthn.js`, `helpers/auth-state.js` are shared by live deployed specs (`texture-deployed-*`, `texture-acceptance-deployed`, `mail-attachments-deployed`, `genesis-settings-staging`); keep them. `frontend/scripts/setup-auth-state.mjs` (package.json `auth:setup`) imports them too.

## 2. Universal Wire app and its client-side tail  (Cut 2: YES)

- **Paths and size**:
  - `src/lib/UniversalWireApp.svelte` 666
  - `src/lib/apps/registry.ts` entry `universal-wire` (lines 132-143, ~12)
  - `src/App.svelte` legacy public reader `/universal-wire/publications/<token>`: script ~25 lines (59-62, 82, 184, 293-304, 378, 495-498), markup 565-642 (~80), CSS 727-848 (~120); ~225 in total
  - `src/lib/texture.js` `WIRE_PLATFORM_READ_OWNER`, `pushTextureReadOwner`, `popTextureReadOwner`, `withReadOwnerQuery` + `readOwner` option on 5 fetchers (~35)
  - `src/lib/TextureEditor.svelte` `wantsPlatformReadScope` / `applyPlatformReadScope` / `clearPlatformReadScope` / `textureReadOwner` (17 refs, lines 561-1275, 876-885, 1863 `appHint === 'universal-wire'`) (~30)
  - `tests/universal-wire-app.spec.js` 651, `tests/universal-wire-staging-acceptance.spec.js` 107 (both opt-in/skipped by env flag)
- **What it did**: A signed-in "living newspaper": lists stories from `GET /api/universal-wire/stories`, opens each as a Texture read through the `read_owner=universal-wire-platform` platform computer. Created 2026-06-06 as "Global Wire storygraph news app" (`f3e6a59d`), renamed 2026-06-10 (`2a8afddf`), last touched 2026-07-10 (`60249573`).
- **Why dead / retired**: The producer (wire processor / sourcecycled) is what Cut 1 deletes; the app shows "fabricated claims" and uses the retired `WireStory` schema (autopaper-app-design-2026-10-11.md §3 table). `App.svelte`'s `loadUniversalWirePublicLink` already just prints "Legacy publication links were removed" (git: this was already gutted in `fd675252` 2026-06-09); the design doc §3 says **"Delete: Dead code: it only prints an error."** No Go route serves `/universal-wire/publications/`.
- **Citers**: docs: autopaper-app-design-2026-10-11.md §3/§9, definitions/choir-autopaper-w3a-reader-2026-10-11.md:31,81, choir-doctrine.md, standing-questions.md, computer-ontology.md. Test: `texture-source-entities.spec.js:54` uses a `choir://universal-wire/source/...` string only as input to a pure function (keep). Go: `proxy/universal_wire.go`, `platform/universal_wire.go`, `vmctl` platform-computer constants (other agents' area).
- **Already scheduled?**: **Cut 2** (cut2 definition lines 129-140 and 281: "The Universal Wire app, its route, types.WireStory and the fixed claims are deleted, with their specs replaced"). The read_owner client path and the App.svelte legacy reader are **not named** in the Cut 2 file even though autopaper-app-design §3 says to retire them. Add them to the unit.
- **Proposal**: remove in Cut 2 together with the backend route. Add to the Cut 2 list: App.svelte reader block (can be removed now, since it is already a stub), `read_owner` plumbing in texture.js/TextureEditor. Keep `sourceEntityLaunchPayload` (`texture-source-launcher.ts`), still used by TextureEditor. Masthead/column CSS in UniversalWireApp is intended to be hand-carried to the Autopaper app before deletion.
- **Risk**: Registry removal changes `APP_REGISTRY`, desk order, and the `desktop-state` restore path: persisted window state with `appId: 'universal-wire'` must be dropped gracefully (check `desktop.js` / Desktop.svelte restore for unknown appIds). `getAppDefinition` returns null for unknown ids, so verify AppHost handles null. `tests/desktop-state-persistence.spec.js` and `prompt-surface-registry.spec.js` enumerate apps; check before deleting.

## 3. Texture compare/merge  (Cut 2: YES)

- **Paths and size**: `src/lib/TextureCompareMergePanel.svelte` 281; `TextureEditor.svelte` merge wiring ~190 (state 139-145, `resetCompareMergeState` 460-470, `toggleMergeSuggestion` 484, `handleCompareToDraft` 1259, `handlePreviewMerge` 1291, `handleAcceptMerge` 1323 (95 lines together), `handleDiscardMerge` 1485 (14), toolbar props/events 1979-2009, panel mount 2052-2063, read-only/state-label reactive statements 1865-1888); `TextureToolbar.svelte` Compare / Merge into draft / Accept / Discard buttons and props ~80 (lines 15-29, 129-192, 683); `texture.js` `semanticCompareTexture`, `previewTextureMerge`, `acceptTextureMerge` ~45; `tests/texture-document-stream.spec.js:415-470` one test (~55).
- **What it did**: Pick a historical version, ask a model to compare it with the draft ("Model compare"), choose suggestions, preview a model-written merge, and accept it as the next owner revision. Panel extracted 2026-06-06 (`82a823e7`, then named VtextCompareMergePanel), renamed 2026-06-15/16.
- **Why dead / retired**: Cut 2 deletes the backend (`textureowner/texture_merge.go`, routes `/compare`, `/merge-preview`, `/accept-merge` in `textureowner/router.go:58-62`) because it is "a one-shot model call outside any desk that writes owner revisions". The frontend is the only caller.
- **Citers**: cut2 definition lines 232-238; `tests/texture-document-stream.spec.js` (1 test).
- **Already scheduled?**: **Cut 2**. The definition names the panel by name; it does **not** name the TextureEditor/TextureToolbar wiring or the 3 client fetchers, which must go in the same commit or the build imports a deleted file.
- **Proposal**: remove in Cut 2 with the Go routes. "The version chevrons replace compare" (definition): confirm TextureVersionHistory/toolbar chevrons exist first.
- **Risk**: `publishDisabled` and `isEditorReadOnly` in the toolbar/editor include `hasMergePreview`/`mergePreview` terms; delete these terms with the feature. `restoreTextureRevision` (route `/restore`) is a separate live feature; do not remove with it.

## 4. Orphaned components: `DesktopIcons.svelte`, `Window.svelte`, `apps/App*` primitives

- **Paths and size**: `src/lib/DesktopIcons.svelte` 240; `src/lib/Window.svelte` 383; `src/lib/apps/AppButton.svelte` 8, `AppCard.svelte` 3, `AppList.svelte` 3, `AppTabs.svelte` 3, `AppToolbar.svelte` 7, `AppEmptyState.svelte` 10; `src/lib/apps/README.md` 16 (describes using them).
- **What they did**: `DesktopIcons` = the left rail of app icons (added 2026-04-12 `67b63b60`, "desktop shell left rail and bottom bar"), with `data-desktop-rail`/`data-rail-item`. `Window` = the first window component (2026-04-11 `7f4941ca`), replaced by `FloatingWindow.svelte` on 2026-04-12 (`730a05f4`). The six `App*` primitives were added 2026-05-29 (`80823eba`, "prepare mainline redesign cutover") as shared UI atoms for apps but no app ever adopted them.
- **Why dead**: No importer of any of the eight (graph + grep). Desktop.svelte imports `FloatingDesktopIcons` and `FloatingWindow`, not these. `DesktopIcons` still comments-only referenced from `desktop-motion.css:106`. The rail is asserted absent by `desktop-shell-core.spec.js:91` and `responsive-layout.spec.js:305,407`. CSS selectors `.app-card/.app-list/.app-tabs/.app-empty-state` appear only in `app.css` theme override lists; `.app-toolbar`/`.app-button` have no CSS at all.
- **Citers**: docs only (`apps/README.md`, the 2026-09-10 inventory corpus). `data-window-titlebar/-close/-minimize` selectors in specs are served by `FloatingWindow.svelte`, not `Window.svelte` (verified: FloatingWindow carries the same attrs).
- **Already scheduled?**: no.
- **Proposal**: remove all eight plus the `apps/README.md` step 3 line referencing the primitives (or the whole README if the registry doc moves). Owner question only if the six primitives are intended for the Autopaper app; if so keep them and wire them in, otherwise delete.
- **Risk**: none dynamic: none is in the registry, none string-resolved. `Window.svelte` has "appId" prop comment but no consumers.

## 5. Dead `export`s with zero importers (src, tests, scripts, docs)

| Symbol | File | ~Lines | Notes |
|---|---|---|---|
| `getHistory`, `getDiff`, `getBlame`, `updateDocument`, `deleteDocument`, `importContentURL`, `importContentFile`, `searchPublishedTexture` | `src/lib/texture.js` | ~110 | Thin clients for routes that **still exist** in Go (`/history`, `/diff`, `/revisions/{id}/blame`, `PUT/DELETE /documents/{id}`, `/api/content/import-*`, `/api/platform/retrieval/search`), so the backend routes are caller-less from the UI (CLI `cmd/choir` may call some; other agents' area). |
| `pushTextureReadOwner`, `popTextureReadOwner` | `texture.js` | 8 | Stubs returning `''`; part of item 2. |
| `createAgentRevision` | `texture.js` | 12 | Only wrapped by `submitAgentRevision`; fold the two. |
| `getLifecycleEvents` | `lifecycle.js` | ~15 | Route `/api/trajectories/{id}/events` exists. |
| `previewEmailMessages` | `public-preview-data.ts:162` | ~33 | MailApp has its own preview data. |
| `minimizedWindows` (derived) | `stores/desktop.js:268` | 5 | Never subscribed. |
| `MOBILE_SWITCHER_APPS` | `apps/registry.ts:274` | 4 | See item 7. |
| `layoutSourceJournalFlow` | `texture-source-flow.ts:183` | ~40 | Only `layoutSourceJournalFlowBlocks` is used; archived mission doc is the only citer. |
| `sourceEntityDisplayPolicy` | `texture-source-renderer.ts:170` | ~10 | |
| `themeStyleString` | `theme.ts:422` | ~14 | |
| `textureRelatedMarkdownTarget` | `texture-source-renderer.ts:422` | ~12 | **Keep**: imported by `tests/texture-source-entities.spec.js:23`. Not dead. |

Also 40+ exports are used only inside their own file (e.g. `SURFACE_HANDOFF_STORAGE_KEY`, `assetGraphFromHTML`, the `OVERVIEW_*` constants, `sourceEntityRecord`, types in `source-contract.ts` / `texture-source-flow.ts`). Those are over-exported rather than dead: drop the `export` keyword, not the code. (`assetGraphFromHTML`/`shouldReplaceDocument` are imported by `tests/computer-surface-handoff.spec.js`; `lifecycleCurrentDocumentRevisionID` by `tests/lifecycle-dual-head.spec.js`: keep those.)
- **Already scheduled?**: no (`semanticCompare/preview/accept` merge fetchers are Cut 2, see item 3).
- **Proposal**: remove the zero-importer ones when the owning feature is touched; the texture.js clients wait on a decision about whether the backend routes stay.
- **Risk**: low; pure functions. For the texture.js clients, check `cmd/choir` and `scripts/*.mjs` first (grep found no JS caller).

## 6. Orphan root-level debug scripts

- **Paths and size** (`frontend/`): `check-network.mjs` 96, `check-runtime.mjs` 54, `check-runtime2.mjs` 55, `check-shell-content.mjs` 63, `check-task-status.mjs` 50, `final-screenshots.mjs` 74, `test-shell-access.mjs` 58, `transfer-cookies.mjs` 64, `webauthn-register.mjs` 84, `webauthn-register-capture.mjs` 60, `webauthn-register-round5.mjs` 102. 760 lines.
- **What they did**: One-off Playwright debugging of the early gateway-vm milestone (April 2026, added in `afbc5789`/`b644a988`), hitting `https://choir.news` and endpoints such as `/shell`, `/api/vm/status`, `/api/autoputer/health`, `/api/health`.
- **Why dead**: `/shell`, `/api/vm/status`, `/api/autoputer/health` and `/api/health` have no Go route. Not referenced by package.json, any workflow, or any script. Only citers are two docs reports (one of which, the 2026-08-18 architecture review, already calls them "orphaned diagnostic scripts ... not part of any CI workflow").
- **Already scheduled?**: no. (They also violate CLAUDE.md's "no untracked scratch in the repo" spirit.)
- **Proposal**: remove (git history keeps them).
- **Risk**: none. Keep `scripts/setup-auth-state.mjs` and `scripts/generate-source-contract.mjs` (live: package.json).

## 7. Registry metadata written but never read

- **Paths**: `src/lib/apps/registry.ts`: field `launcher.mobileSwitcher` (set on 17 entries, only consumed by the dead `MOBILE_SWITCHER_APPS`); field `auth.requiresAuthFor` (set on all 18 entries, read by nothing; `AppSurface` reads only `auth.preview`); type `AppPreviewPolicy` values `public-readonly`/`private` are live.
- **What it does or did**: Declared the logged-out preview contract (README: "registry entry owns ... auth-required actions"). Actual enforcement is by each component dispatching `authrequired` at action time.
- **Why dead**: grep across src: `requiresAuthFor` only in registry.ts; `mobileSwitcher` only registry.ts (PromptSurface has an unrelated local variable `mobileSwitcherOpen`). Added in `80823eba`.
- **Citers**: `apps/README.md`.
- **Already scheduled?**: no.
- **Proposal**: owner question. It is documentation-as-data that could feed a future auth-policy test; if not, remove both fields and `MOBILE_SWITCHER_APPS` (~40 lines of repetitive entries).
- **Risk**: type `satisfies ChoirAppDefinition[]` will fail until the type is updated in the same edit.

## 8. Dead selectors in `src/app.css`

- **Paths**: `src/app.css` (504 lines), theme-override selector lists at lines ~100-135, ~470-485, and scattered rules.
- **What they did**: Shared theme overrides for per-app classes of apps that no longer exist (trace app, features app, acceptance/swimlane views, old mail rail).
- **Why dead**: ~38 class names have zero occurrences in any `.svelte/.js/.ts`: `trace-app trace-frame trace-main trace-sidebar features-app feature-card acceptance-card acceptance-row evidence-detail event-card lane-card swimlane trajectory-item state-pill library-card media-card document-card message-card message-detail message-list mail-rail mobile-mailbar reader-panel reader-toolbar browser-sidebar browser-toolbar detail-card detail-meta folder-item control-button toolbar-button settings-panel section-title desktop-icons desktop-title app-label floating-desktop-icons mobile-app-button is-active`. Verification caveat: classes built by string concatenation would not show up in the grep; none found, but check `class:` directives in Svelte `<style>` blocks before deleting (svelte-scoped styles do not use these).
- **Citers**: none (`desktop-motion.css` and `texture-source-flow.css` have zero unused classes).
- **Already scheduled?**: no.
- **Proposal**: remove the selectors; low value but trivially safe once confirmed.
- **Risk**: `!important` override lists can change specificity; verify visually with `theme-contract.spec.js` / `theme-hydration.spec.js` (which exist and use live selectors).

## 9. `tests/s0a-reality-probe.mjs` (360)

- **What it does**: One-shot station-S0 probe (boot timeline receipts, tap reachability, gateway token visibility) run against staging on 2026-10-01. Not matched by Playwright's default testMatch (it is `.mjs`, not `.spec.`), not in package.json or CI.
- **Citers**: `docs/reports/s0a-boot-timeline-landing-2026-10-01.md`, `docs/choir-appdev-s0-reality-boot-timeline-ledger-2026-10-01.md` (landed receipts).
- **Already scheduled?**: no.
- **Proposal**: keep until the re-engineering decides where scripts/probes live (the repo has ~20 similar `scripts/*_probe.mjs`); belongs in `scripts/` or `docs/evidence`, not `tests/`. Owner question.
- **Risk**: none.

## 10. Smaller stale test fragments

- Mock/route filters in specs that blacklist routes with no Go implementation: `/api/agent/spawn`, `/api/agent/topology`, `/api/events` in `texture-deployed-live-search.spec.js:189-192`, `trace-settings-registry.spec.js:45-49`, `content-substrate-routing.spec.js:68-72`, `content-apps-routing.spec.js:122-126`, `texture-real-workflow-demo.spec.js:184-187`, `deployed-origin-auth-shell.spec.js:148-150`. Harmless dead branches (~25 lines in total). Part of item 1 clean-up.
- `/api/protected` in `auth-renewal-transient.spec.js` is a Playwright `page.route` mock, not a real route (fine).
- Duplicate lockfiles `frontend/package-lock.json` and `frontend/pnpm-lock.yaml`: **both live** (flake.nix:92-121 builds with npm + package-lock; CI/local dev use pnpm). Not dead, but a drift hazard; flag for the re-engineering.

---

## Verified live (so nobody deletes them by mistake)

- All 18 registry apps have launcher paths and live backends: files, browser (Web Lens), email, compute-monitor, pulse (`/api/pulse/summary`), texture, universal-wire (until Cut 2), podcast, image/audio/video/pdf/epub/slides (via Desk sheet and Files), calendar, content (opened from source launcher), super-console (`/api/super-console/ws`), settings.
- `public/desktop-bridge.html`: loaded by `cmd/desktop/desktop_auth.go:137`.
- `source-contract.generated.ts` (481): generated by `scripts/generate-source-contract.mjs` in `pnpm build`.
- All of `stores/desktop.js` stores except `minimizedWindows` have writers and readers; `stores/terminal.js` is used by SuperConsoleApp.
- `PretextInlineDisclosure` (AuthEntry), `ChoirField`/`TetraMark`/`LandingIntro` (App/Desktop), `TextureVersionHistory`, `TexturePublicationResult`, `TextureSourcePanel`, `texture-structured-editor-doc.ts` etc.: all reachable from TextureEditor.
- npm deps `@chenglou/pretext`, `ghostty-web`, `jszip`, `pdfjs-dist` each have an importer; `typescript` is needed for svelte-preprocess TS.

## Suggested ordering for removal

1. Now, no dependency: items 4, 6, 8, the zero-importer exports in item 5, `minimizedWindows`, and the App.svelte reader block in item 2.
2. With Cut 2: items 2 (rest) and 3, including the TextureEditor/Toolbar/texture.js wiring and the read_owner plumbing which the Cut 2 text does not list.
3. Owner decisions: item 1 (what Playwright is for and where it runs), item 7, item 9.
