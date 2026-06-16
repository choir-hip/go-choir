# Mission: Texture Hard Cutover v0

## Summary

Texture is the promoted ontology and product language for Choir's versioned,
transclusive artifact control plane. The old V-name is now migration residue,
not current doctrine.

This mission is not a cosmetic rename. It is an ontology cutover. The codebase,
prompts, UI, APIs, tests, docs checker, high-read docs, and product-path proofs
must teach the same object: Texture as the artifact layer that turns autonomous
activity into directed results and compounding learning.

The protocol spec is deliberately not written first. The mission must make the
product path work, delete accidental complexity, prove the minimal surface, and
only then canonize a Texture Protocol v0.

## Source Documents

- [why-texture-2026-06-15.md](./why-texture-2026-06-15.md)
- [why-texture-background-2026-06-15.md](./why-texture-background-2026-06-15.md)
- [choir-doctrine.md](./choir-doctrine.md)
- the M3.4 first-draft regression paradoc linked through
  [mission-graph.yaml](./mission-graph.yaml)
- [mission-portfolio-2026-06-11.md](./mission-portfolio-2026-06-11.md)

## Problem

The system currently carries a split ontology. The product object has outgrown
its old internal name, but code, prompts, docs, tests, route names, tool names,
and acceptance language still teach the old object. That split invites shallow
patches: route fixes that preserve wrong concepts, prompt fixes that encode
workflow decisions, and docs that describe a control plane while the runtime
still names it like an internal text widget.

The current urgent regression is also a warning. A prompt can open the artifact
surface but fail to create the first useful revision. That failure is easier to
miss when acceptance overweights route topology and underweights browser-driven
proof of the actual artifact loop.

## Problem Checkpoint: Retired-Name Inventory

Mutation class: `green` documentation and evidence only. No runtime behavior,
schema, API, prompt default, UI, or test surface changed in this checkpoint.

Read-only search on 2026-06-15 confirms that the old V-name is not isolated
implementation residue. It is still the dominant artifact-control-plane name
across current docs, runtime, frontend, tests, API routes, data attributes,
tool names, prompt defaults, and storage identifiers.

Receipts:

- `rg -l -i 'vtext|\.vtext|VText|VTEXT'` over the worktree found retired-name
  content in 172 docs files, 82 runtime Go files, 35 frontend source files,
  33 frontend tests, 9 store files, 9 runtime prompt files, 6 type files,
  4 command files, 2 spec files, and both root contracts.
- The same inventory found retired-name path components in 44 docs paths,
  22 runtime Go paths, 18 frontend source paths, 16 frontend test paths,
  2 store paths, 1 type path, 1 runtime prompt path, and 1 command path.
- Selected affordance line counts: `/api/vtext` 505, `data-vtext` 604,
  `edit_vtext` 390, `request_super_execution` 122, V-name profile references
  417, `.vtext` 942, and `vtext_` 658.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json` completed in report-only mode with 212 docs and
  803 warnings before any Texture-specific checker rule was added.

Checker warning design:

- Add a report-only Texture retired-name warning family to `cmd/doccheck`
  rather than failing docs-only CI in the same pass.
- Scan current and mixed non-evidence docs plus code/prompt/frontend/test
  surfaces for retired-name terms: `VText`, `vtext`, `.vtext`, `/api/vtext`,
  `data-vtext`, `edit_vtext`, and `vtext_`.
- Treat `docs/why-texture-background-2026-06-15.md` as the standing historical
  background allowlist entry.
- Allow historical mission/evidence occurrences only when the manifest marks
  the file `claim_scope: historical` or `is_evidence: true`, or when a mixed
  mission line explicitly labels the occurrence as historical evidence,
  retired-name evidence, migration residue, or a deletion target.
- Current docs, prompts, UI labels, tests, API affordances, storage-facing names,
  and tool names should warn until renamed or explicitly classified as temporary
  cutover residue with a deletion receipt. Warning silence is not settlement;
  final settlement still requires the retired-name search to show only allowed
  historical/background occurrences.

## Problem Checkpoint: Platform Publication Route Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
schema, API, prompt default, UI, or test surface changed in this checkpoint.

Read-only search on 2026-06-16 confirms a specific remaining route split after
the main Texture API cutover: publication and platform-document routing still
teach the old artifact name even though `/api/texture` is the active canonical
document API.

Receipts:

- `frontend/src/lib/vtext.js` still documents and calls
  `/api/platform/vtext/publications` for publishing a Texture revision.
- `internal/proxy/handlers.go` still dispatches the public platform publish
  route at `/api/platform/vtext/publications` and the internal wire publish
  route at `/internal/wire/platform/publications/vtext`.
- `internal/proxy/platform_publish.go`,
  `internal/proxy/wire_platform_publish.go`, `internal/wirepublish/client.go`,
  and `internal/runtime/wire_platform_publish.go` still call platformd or proxy
  publication endpoints ending in `/vtext`.
- `internal/platform/handlers.go` still registers platformd internal publish,
  sync, document-read, and revision-read routes under
  `/internal/platform/publications/vtext` and `/internal/platform/vtext/...`.
- `/pub/vtext/...` public publication routes remain the live published URL
  shape and require a separate route migration/redirect policy; do not silently
  rename existing public article URLs in the same slice.

Next behavior slice design:

- hard-cut the platform/proxy/internal publication control routes to
  `/texture` naming without preserving a browser-public or platform-internal
  `/vtext` compatibility route;
- preserve `/pub/vtext/...` published reader URLs until a route identity
  migration plan exists, because existing public links are route state rather
  than merely handler names;
- prove the cutover with focused proxy/platform/runtime tests, CI, staging
  deploy identity, and a deployed route probe that shows the old control route
  absent while the new Texture route reaches its expected auth/method gate.

## Problem Checkpoint: App Identity And Storage Symbol Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
schema, API, prompt default, UI, test, or persistent state changed in this
checkpoint.

Read-only search on 2026-06-16 confirms that, after public route and visible UI
label cutovers, the retired name still carries several different kinds of state
with different migration risk. They must not be collapsed into one rename.

Receipts:

- Path inventory excluding `frontend/dist` found 103 current source/doc/test
  paths whose filenames still contain the retired name or `.vtext`.
- App identity search found 38 current frontend/runtime/store/test hits for
  `appId: 'vtext'`, `id: 'vtext'`, `AppID: "vtext"`, URL `app=vtext`, or
  preview/Trace agent ids. The canonical app registry still uses `id: 'vtext'`
  while the visible app name is already `Texture`.
- Storage symbol search found 1,009 hits for `vtext_documents`,
  `vtext_revisions`, `vtext_document_aliases`, `vtext_agent_mutations`,
  `vtext_controller_checkpoints`, `vtext_decisions`, `platform_vtext_*`,
  `database=vtext`, `.vtext`, and `go-choir-vtext`.
- Metadata/tool search found 791 hits for symbols such as `edit_vtext`,
  `vtext_ref`, `vtext_doc`, `vtext_revision`, `source_vtext`,
  `platformd_route_path`, `related_vtext`, `transcluded_vtext`, and `vtext_`.
- `frontend/src/lib/apps/registry.ts` exposes the current visible Texture app
  under the old app id; `frontend/src/App.svelte`,
  `frontend/src/lib/Desktop.svelte`, `frontend/src/lib/UniversalWireApp.svelte`,
  `frontend/src/lib/source-contract.ts`, and `frontend/src/lib/VTextEditor.svelte`
  still launch or auth-gate that app with `appId: 'vtext'`.
- `internal/store/desktop_test.go`, `internal/runtime/desktop_test.go`, and
  `internal/store/store_test.go` show persisted desktop/app state can contain
  `app_id='vtext'`.

Next behavior slice design:

- cut the canonical frontend app id from `vtext` to `texture` so new launches,
  desktop icons, app switchers, auth intents, source-open plans, and public
  preview windows teach Texture at the app identity layer;
- normalize the legacy `vtext` app id at the desktop-state boundary so existing
  persisted windows reopen as Texture instead of disappearing after deploy;
- keep auth intent kinds such as `save_vtext` and deeper storage/table/file
  symbols out of this slice unless tests prove they must move together;
- prove the slice with focused frontend build/tests, Go desktop-state tests if
  backend normalization is touched, CI, staging deploy identity, and a staging
  browser/DOM proof that the Texture app renders under `data-app-id="texture"`
  while legacy `app=vtext` URL or saved state still opens the same app.

## Repair: Texture App Identity

Mutation class: `orange`, because this changes frontend app identity, app
launch/replay behavior, desktop persistence/restore normalization, source-open
app selection, and runtime desktop-state API sanitization.

Conjecture delta: new app launches and restored windows can use canonical
`texture` app identity while deletion-receipted legacy `vtext` app ids still
resolve at launch, URL-intent, frontend desktop-store, and runtime desktop API
boundaries.

Protected surfaces: app registry, desktop window persistence/restore,
source-open app selection, auth intent replay, public preview windows, and
runtime desktop-state get/save.

Local evidence on 2026-06-16:

- `npm --prefix frontend run build` passed. Vite reported pre-existing
  Universal Wire warnings for unused `currentUser` export and `.wire-state`
  selectors.
- `nix develop -c go test -tags comprehensive -v ./internal/runtime -run '^TestDesktopState'`
  passed, including `TestDesktopStateSanitizesLegacyTextureAppID`.
- `nix develop -c scripts/go-test-runtime-shards` passed all four runtime
  shards.
- App-id residue search for `appId: 'vtext'`, `id: 'vtext'`, legacy open calls,
  `getAppIcon('vtext')`, `public-preview-vtext`, and `data-app-id="vtext"`
  found only public preview Trace fixture agent ids after excluding
  `frontend/dist`.

Rollback path: revert the behavior commit to restore canonical `vtext` app ids
and remove the frontend/runtime normalization shims.

Deployed evidence on 2026-06-16:

- Commit `f27c00154f4eb1025075cc6eb6b76383324dd5f1` passed CI run
  `27588733421`.
- Deploy job `81564942700` succeeded.
- Staging health at `https://choir.news/health` reported proxy and sandbox
  commit `f27c00154f4eb1025075cc6eb6b76383324dd5f1`, deployed at
  `2026-06-16T01:55:03Z`.
- Staging Playwright DOM proof on `https://choir.news/` found one
  `data-app-id="texture"` window, zero `data-app-id="vtext"` windows, one
  `data-desktop-icon-id="texture"` icon, zero legacy `vtext` desktop icons, and
  restored public preview window id `public-preview-texture`.
- Staging Playwright DOM proof on
  `https://choir.news/?app=vtext&doc=legacy-proof-doc&title=Legacy%20Texture`
  found one Texture window, zero legacy `vtext` windows, visible Texture text,
  and no visible `VText` text.

Heresy delta: repaired for deployed app identity; no storage/table/file/metadata
symbol repair claimed.

Remaining scope: storage schema/workspace/file suffixes, metadata keys,
`/pub/vtext/...` route identity, and protocol v0.

## Problem Checkpoint: Public Preview Trace Fixture Residue

Mutation class: `green` documentation and evidence only. No frontend source,
runtime behavior, schema, API, prompt default, UI, test, or persistent state
changed in this checkpoint.

Read-only search on 2026-06-16 shows that the next small residue class is a
public-preview Trace fixture in `frontend/src/lib/public-preview-data.ts`. It
still names the Texture actor as `agent_id: 'vtext'`, routes preview edges
through `vtext`, and records preview moments against `agent_id: 'vtext'`.
This is distinct from durable runtime agent ids such as `vtext:<doc_id>` and
from storage symbols such as `vtext_revisions`; it is local signed-out fixture
data.

Receipts:

- `rg -n "agent_id: 'vtext'|to_agent_id: 'vtext'|from_agent_id: 'vtext'"`
  on `frontend/src/lib/public-preview-data.ts` found seven fixture hits.
- `rg -n "previewTraceSnapshot|previewTraceTrajectories" . -g '!frontend/dist' -g '!node_modules'`
  found only the fixture definitions themselves, with no consumers.
- The fixture's acceptance text says "Trace layout renders without private
  trajectories", which conflicts with the current doctrine guardrail that Trace
  is evidence/topology, not a normal public product surface.

Next behavior/source slice design:

- delete the unused `previewTraceTrajectories` and `previewTraceSnapshot`
  fixture exports instead of renaming their actor ids, so the mission does not
  preserve a dead Trace product preview;
- keep the live `previewVTextDocument` export for the signed-out Texture app
  preview, leaving its exported symbol name for a later broader frontend file
  and API-name migration;
- prove with frontend build and residue searches that no public-preview Trace
  fixture actor id remains.

## Repair: Public Preview Trace Fixture Deletion

Mutation class: `yellow`, because this deletes unused frontend fixture exports
and changes future optimization/documentation pressure without changing a live
product path.

Conjecture delta: deleting the unused fixture is a cleaner Texture cutover move
than renaming it, because it removes a dead Trace-as-product preview and avoids
creating a new public Trace surface.

Protected surfaces: signed-out preview data module and frontend build.

Local evidence on 2026-06-16:

- `npm --prefix frontend run build` passed. Vite reported the existing
  Universal Wire warnings for unused `currentUser` and `.wire-state` selectors.
- `rg -n "previewTraceSnapshot|previewTraceTrajectories|preview-trace|Trace layout|agent_id: 'vtext'|to_agent_id: 'vtext'|from_agent_id: 'vtext'" frontend/src/lib/public-preview-data.ts frontend/src -g '!frontend/dist'`
  returned no hits.

Deployed evidence on 2026-06-16:

- Commit `3037e1f92971e7324a8bb8c3e356474e4eee2cc6` passed CI run
  `27589138319`; deploy job `81566163866` succeeded.
- Separate `Docs Truth Check` run `27589138321` and FlakeHub publish run
  `27589138328` completed successfully for the same commit.
- Staging health at `https://choir.news/health` reported proxy and sandbox
  commit `3037e1f92971e7324a8bb8c3e356474e4eee2cc6`, deployed at
  `2026-06-16T02:06:07Z`.
- Staging Playwright DOM proof on `https://choir.news/` found one
  `data-app-id="texture"` window, zero `data-app-id="vtext"` windows, one
  `data-desktop-icon-id="texture"` icon, zero legacy `vtext` desktop icons,
  and no visible "Trace layout", `preview-trace`, or public-preview `vtext`
  actor text.

Rollback path: restore the deleted fixture exports if a real consumer is found.

Heresy delta: repaired for deployed unused public-preview Trace fixture residue;
no durable runtime agent-id or storage-symbol repair claimed.

## Problem Checkpoint: Public Publication Route Identity

Mutation class: `green` documentation and evidence only. No route generation,
database state, frontend routing, API behavior, prompt default, test, or
deployment surface changed in this checkpoint.

Read-only search on 2026-06-16 shows that publication control routes are now
Texture-named, but newly published public reader URLs still mint under
`/pub/vtext/...`. This is live public route identity, not the same surface as
the browser-public `/api/texture` document API or platform/internal publication
control routes already cut over.

Receipts:

- `internal/platform/service.go` still defines `publicVTextPrefix =
  "/pub/vtext/"` and constructs new `routePath` values from that prefix in
  `PublishVText`.
- The same file stores the slug by trimming `publicVTextPrefix` and only
  normalizes trailing slashes for routes with that prefix in
  `normalizePublicationRoutePath`.
- `frontend/src/App.svelte` only recognizes direct public reader entry when
  `window.location.pathname.startsWith('/pub/vtext/')`.
- `frontend/src/lib/Desktop.svelte` only normalizes public reader paths with
  the `/pub/vtext/` prefix before opening a published Texture window or
  deduplicating already-open published windows.
- Product tests still assert newly published route paths match
  `^/pub/vtext/` in `frontend/tests/file-browser.spec.js` and
  `frontend/tests/vtext-source-service-publication.spec.js`; platform and proxy
  tests still fixture public routes under `/pub/vtext/...`.

Next behavior slice design:

- mint new publication reader routes under `/pub/texture/...`;
- continue resolving and exporting existing stored `/pub/vtext/...` rows
  through `/api/platform/publications/resolve` and
  `/api/platform/publications/export`, because those rows are public link state;
- make the frontend public reader recognize both `/pub/texture/...` and
  deletion-receipted legacy `/pub/vtext/...` route paths so existing public
  links keep opening Texture;
- avoid database rewrites or silent external-link redirects in this slice; a
  redirect/migration policy can be a later settlement move after new
  Texture-named URLs are proven;
- prove locally with platform route generation/read tests, proxy public
  resolve/export tests, and frontend publication tests that new routes are
  Texture-named while legacy reader paths are still accepted.

## Local Repair: Public Publication Route Identity

Mutation class: `orange`, because this changes public publication route
generation, frontend public-reader route recognition, proxy/platform route
tests, and publication-product expectations.

Conjecture delta: new public publication links can teach Texture by minting
`/pub/texture/...` while existing `/pub/vtext/...` link state remains readable
through explicit legacy route acceptance.

Protected surfaces: platform publication route generation, public route
lookup/export, frontend direct public reader entry, published Texture window
deduplication, proxy publication public URL projection, and product
publication tests.

Local evidence on 2026-06-16:

- `nix develop -c go test ./internal/platform -run 'TestPublishVTextCreatesImmutablePublicRecords|TestInternalPublishRequiresInternalCallerAndBundleResolve'`
  passed.
- `nix develop -c go test ./internal/proxy -run 'TestPlatformPublicationResolveIsPublicAndInternalOnly|TestPlatformPublicationResolveAndExportPropagateNotFound|TestHandleVTextPublication'`
  passed.
- `nix develop -c go test ./internal/platform ./internal/proxy` passed.
- `npm --prefix frontend run build` passed. Vite reported pre-existing
  Universal Wire warnings for unused `currentUser` and `.wire-state`
  selectors.
- Route residue search
  `rg -n "publicVTextPrefix|/pub/vtext/|\^\\/pub\\/vtext|startsWith\('/pub/vtext/'\)|startsWith\(\"/pub/vtext/\"\)" internal/platform internal/proxy frontend/src frontend/tests --glob '!frontend/dist/**'`
  now finds only the explicit legacy route prefix/helper, legacy route tests or
  fixtures, and frontend dual-prefix acceptance.
- Local Playwright was attempted, but the local service harness could not reach
  platformd because the existing `/tmp/go-choir-m2/platform-dolt` state reported
  missing `.dolt/repo_state.json`. The controlled foreground service session
  was stopped and health checks for local service ports returned down.

Rollback path: restore `/pub/vtext/...` route minting, remove
`/pub/texture/...` public-reader prefix recognition, and revert route
expectations if staging publication/read/export proof fails.

Heresy delta: repaired locally for new public route minting; legacy
`/pub/vtext/...` public links remain explicit compatibility state pending
deployed proof and any later redirect/migration policy.

## Deployed Repair: Public Publication Route Identity

Mutation class: `orange`, deployed behavior evidence for the public route
identity repair.

Conjecture delta: deployed Choir can mint new public publication URLs under
`/pub/texture/...` while preserving existing `/pub/vtext/...` public link state
for resolve, export, and direct public reader entry.

Deployed evidence on 2026-06-16:

- Commit `65502a706ef1adba7fc2d1ed5428e3f709f9d2d0` passed CI run
  `27590698503`; the deploy job `81570766605` succeeded.
- Docs Truth Check run `27590698536` passed, and FlakeHub publish run
  `27590698504` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `65502a706ef1adba7fc2d1ed5428e3f709f9d2d0`, deployed at
  `2026-06-16T02:50:42Z`.
- Deployed Playwright product proof registered fresh user
  `texture-public-route-proof-1781578657650-ce9lel@example.com`, created
  Texture document `79579ae6-f620-4194-9a0a-afabee56a1fd`, created revision
  `e673f6f3-3c80-4577-9699-be146f996283`, and published publication
  `pub-19a8e51e-732d-498e-814c-fe18aa37568a` /
  version `pubver-4f361ae5-30e0-4ed6-b9a8-6dd1edb9c2ef`.
- The new route was
  `/pub/texture/texture-public-route-proof-1781578657650-pub19a8e51e7`.
  Public resolve normalized the route with a trailing slash back to that exact
  path, public Markdown export returned the same route and proof content, and
  retrieval search for `1781578657650` returned the new `/pub/texture/...`
  route.
- Direct browser navigation to the new route opened one Texture window and one
  published reader, displaying proof stamp `1781578657650`.
- Legacy route
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6` resolved
  with trailing slash normalization, exported Markdown, and opened in the
  published Texture reader.
- The browser proof observed zero forbidden product-path requests to
  `/internal/*`, `/api/agent/*`, `/api/test/*`, `/api/prompts`, or
  `/api/events`.

Evidence artifact: `/tmp/choir-texture-route-proof-1781578657650.json`.
Screenshots: `/tmp/choir-texture-route-proof/new-texture-route-1781578657650.png`
and `/tmp/choir-texture-route-proof/legacy-vtext-route-1781578657650.png`.

Rollback path remains: restore `/pub/vtext/...` route minting and remove
`/pub/texture/...` public-reader prefix recognition if later deployed public
reader/export regressions appear.

Heresy delta: repaired for deployed new public route minting. Existing
`/pub/vtext/...` public routes remain deliberate legacy compatibility state,
not a current new-publication minting path.

## Problem Checkpoint: Texture Auth Intent Label Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
frontend source, schema, API, prompt default, UI, test, or persistent state
changed in this checkpoint.

Read-only search on 2026-06-16 shows a narrow product-facing residue class:
frontend auth-required intent kinds and replay labels still use old-name
tokens even though the canonical app id and visible app label are now Texture.
This is distinct from durable runtime actor ids such as `vtext:<doc_id>`,
source metadata keys such as `vtext_source_artifact_attachment`, and storage
symbols such as `vtext_documents`.

Receipts:

- `frontend/src/lib/apps/registry.ts` still declares Texture auth requirements
  as `save_vtext`, `revise_vtext`, and `publish_vtext`.
- `frontend/src/lib/VTextEditor.svelte` still dispatches auth intents
  `save_vtext`, `publish_vtext`, `vtext_diagnosis`,
  `vtext_source_repair`, `vtext_source_artifact`, and
  `published_vtext_edit` while passing `appId: 'texture'` and
  `appName: 'Texture'`.
- `frontend/src/App.svelte` still renders/replays `save_vtext`,
  `publish_vtext`, `published_vtext_edit`, and `private_vtext_document`
  intent kinds.
- Legacy route compatibility remains in `frontend/src/App.svelte` for
  `?app=vtext&doc=...`, and tests intentionally cover that compatibility.
  That is app-route compatibility, not a current app identity target.
- Nearby hits such as `created_from: 'vtext_source_artifact_ui'`,
  `source: vtext_source_artifact_attachment`, `publish_vtext_revision`,
  `choir.platform.publish_vtext.v0`, and `vtext:<doc_id>` are metadata,
  provenance, verifier, or runtime actor-route residue. They need separate
  migration design and must not be renamed as part of this small frontend
  auth-intent slice.

Next behavior slice design:

- introduce Texture-named frontend auth intent kinds:
  `save_texture`, `revise_texture`, `publish_texture`,
  `texture_diagnosis`, `texture_source_repair`,
  `texture_source_artifact`, `published_texture_edit`, and
  `private_texture_document`;
- keep legacy intent-kind handling in the auth overlay/replay boundary during
  the cutover so already-created in-memory or URL-derived intents do not drop;
- update Texture app registry auth requirements and Texture editor dispatches
  to emit only the new intent kinds;
- keep `?app=vtext&doc=...` legacy URL compatibility and durable
  `vtext:<doc_id>` actor ids out of this slice;
- prove locally with frontend build and targeted frontend tests, then push,
  monitor CI/deploy, verify staging identity, and run a deployed browser proof
  that a signed-out Texture action opens an auth overlay whose pending intent
  is Texture-named while legacy `app=vtext` still opens Texture.

## Local Repair: Texture Auth Intent Labels

Mutation class: `orange`, because this changes frontend auth-required intent
kinds, Texture app registry auth requirements, auth overlay test affordances,
and post-auth replay normalization.

Conjecture delta: new frontend Texture actions can emit Texture-named auth
intent kinds while the auth overlay and replay boundary still accepts
deletion-receipted legacy intent names and legacy `?app=vtext&doc=...` URL
compatibility.

Protected surfaces: Texture app registry auth requirements, Texture editor
auth-required dispatches, auth overlay copy, post-auth app replay, legacy
intent replay, legacy `?app=vtext&doc=...` URL compatibility, and signed-out
public preview Texture actions.

Local evidence on 2026-06-16:

- `npm --prefix frontend run build` passed. Vite reported the existing
  Universal Wire warnings for unused `currentUser` and `.wire-state`
  selectors.
- `npm --prefix frontend run e2e -- --project=chromium
  tests/auth-entry-ui.spec.js --grep "signed-out Texture publish"` passed
  against an explicit local Vite preview server.
- A broader
  `npm --prefix frontend run e2e -- --project=chromium
  tests/auth-entry-ui.spec.js` attempt failed before app execution because no
  local server was listening on `localhost:4173`; this was harness setup, not a
  product assertion.
- Producer residue search
  `rg -n "save_vtext|revise_vtext|publish_vtext|vtext_diagnosis|vtext_source_repair|vtext_source_artifact|published_vtext_edit|private_vtext_document" frontend/src frontend/tests -g '!frontend/dist'`
  now finds only the explicit legacy normalization map in `frontend/src/App.svelte`
  and the out-of-scope provenance marker
  `created_from: 'vtext_source_artifact_ui'`.

Rollback path: restore old intent strings in Texture editor dispatches,
registry auth requirements, and App replay/message handling if auth overlay
replay or legacy app URL compatibility regresses.

Heresy delta: repaired locally for new frontend auth intent labels; durable
actor ids, storage symbols, and source/provenance metadata remain separate
discovered residue.

Open edge: push the repair, monitor CI/deploy, verify staging identity, and run
a deployed browser proof that a signed-out Texture action exposes a
Texture-named auth intent while legacy `?app=vtext&doc=...` still opens
Texture.

## Deployed Repair: Texture Auth Intent Labels

Mutation class: `orange`, deployed behavior evidence for the frontend
auth-intent label repair.

Conjecture delta: deployed Choir can present Texture-named auth-required intent
state for signed-out Texture actions while preserving deletion-receipted legacy
`?app=vtext&doc=...` compatibility for authenticated document deep links.

Deployed evidence on 2026-06-16:

- Commit `2f13598d37be2807f8cefe9258300a1a798a081c` passed CI run
  `27591417530`; the deploy job `81572916777` succeeded.
- Docs Truth Check run `27591417528` passed, and FlakeHub publish run
  `27591417545` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `2f13598d37be2807f8cefe9258300a1a798a081c`, deployed at
  `2026-06-16T03:10:59Z`.
- Deployed Playwright proof
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-auth-intent-deployed.tmp.spec.js`
  passed both browser assertions before the temporary spec was deleted.
- The signed-out proof opened the public Texture preview, used the Texture
  publish action, observed `[data-auth-overlay]` with
  `data-auth-intent-kind="publish_texture"`, observed auth copy containing
  "Publish this Texture", observed zero `[data-app-id="vtext"]` windows, and
  observed zero forbidden browser-public requests to `/internal/*`,
  `/api/agent/*`, `/api/test/*`, `/api/prompts`, or `/api/events`.
- The authenticated legacy URL proof registered a fresh staging user with a
  virtual passkey, created a Texture document through `/api/texture/documents`,
  created a revision through `/api/texture/documents/{doc}/revisions`,
  navigated to `?app=vtext&doc=...`, and observed exactly one canonical
  `[data-app-id="texture"]` window, zero `[data-app-id="vtext"]` windows,
  rendered proof content, and a consumed URL with no `app=vtext` query.

Screenshots: `/tmp/choir-texture-auth-intent-1781579569646.png` and
`/tmp/choir-texture-auth-legacy-url-1781579569646.png`.

Rollback path remains: restore old intent strings in editor dispatches,
registry requirements, and App replay/message handling if later auth replay or
legacy app URL compatibility regresses.

Heresy delta: repaired for deployed frontend auth intent labels. Durable actor
ids, storage symbols, and source/provenance metadata remain separate discovered
residue.

## Problem Checkpoint: Source Repair Metadata Label Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
frontend source, schema, API, prompt default, UI, test, or persistent state
changed in this checkpoint.

Read-only search on 2026-06-16 shows a narrow source/provenance metadata
residue class: new source repair and source artifact paths can still emit
old-name `vtext_source_*` provenance strings even though the user-visible
surface, product API routes, and auth intents now teach Texture. This is
separate from storage table names, durable actor ids, publication verifier
predicates, and app-package review contract fields.

Receipts:

- `internal/runtime/vtext_source_repairs.go` still writes revision metadata
  `source="vtext_source_gap_repair"` for source gap repairs and
  `source="vtext_source_artifact_attachment"` for source artifact attachment
  revisions.
- `frontend/src/lib/vtext-source-actions.ts` still creates source content item
  metadata with `created_from: 'vtext_source_artifact_ui'`.
- `internal/runtime/vtext_test.go` still asserts the old emitted metadata
  values in source repair and source artifact attachment tests.
- `frontend/tests/vtext-markdown-lineage.spec.js` still asserts
  `repaired.metadata?.source === 'vtext_source_gap_repair'`.
- Adjacent metadata hits such as `canonical_vtext_source_path`,
  `related_vtexts`, `story_vtext_doc_id`, `vtext_doc_id`,
  `vtext_revision_id`, `private_vtext_revision`,
  `publish_vtext_revision`, and `choir.platform.publish_vtext.v0` are broader
  storage, transclusion, app-package review, or platform publication
  provenance surfaces and require separate migration design.

Next behavior slice design:

- emit `texture_source_gap_repair`,
  `texture_source_artifact_attachment`, and
  `texture_source_artifact_ui` for new source repair/artifact paths;
- keep source entity structs, source routes, and `.vtext`/alias/storage fields
  out of this slice;
- preserve no reader compatibility unless investigation finds a live consumer
  of these exact old emitted values; if compatibility is required, make it an
  explicit legacy read predicate with a deletion receipt rather than continuing
  to emit old values;
- prove locally with focused runtime source repair tests, the focused frontend
  markdown-lineage/source repair test, frontend build, and residue searches,
  then push, monitor CI/deploy, verify staging identity, and run a deployed
  proof through the source repair or source artifact product path if the
  behavior reaches staging.

## Local Repair: Source Repair Metadata Labels

Mutation class: `orange`, because this changes new revision metadata emitted by
the source repair and source artifact attachment product paths, plus frontend
source content item provenance metadata.

Conjecture delta: new source repair/artifact metadata can emit Texture-named
provenance values without changing source entity structures, source routes,
storage tables, `.vtext` alias behavior, durable actor ids, or platform
publication attestations.

Protected surfaces: source gap repair revision metadata, source artifact
attachment revision metadata, frontend source content item creation provenance,
source repair tests, source artifact attachment tests, and markdown-lineage
browser tests.

Local evidence on 2026-06-16:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextSourceGapRepairCreatesRevision|TestVTextSourceArtifactAttachmentCreatesMetadataOnlyRevision'
  -count=1` passed.
- `npm --prefix frontend run build` passed. Vite reported the existing
  Universal Wire warnings for unused `currentUser` and `.wire-state`
  selectors.
- Live residue search
  `rg -n "vtext_source_gap_repair|vtext_source_artifact_attachment|vtext_source_artifact_ui" internal frontend/src frontend/tests -g '!frontend/dist/**'`
  returned no hits.
- Texture-name search now finds only the intended emitters and focused
  assertions for `texture_source_gap_repair`,
  `texture_source_artifact_attachment`, and
  `texture_source_artifact_ui`.
- Local Playwright attempt
  `npm --prefix frontend run e2e -- --project=chromium
  tests/vtext-markdown-lineage.spec.js --grep "Migrated source gaps"`
  failed before app execution because no local server was listening on
  `localhost:4173`; this is local harness availability, not product behavior
  evidence.

Rollback path: restore the old emitted `vtext_source_*` metadata values and
test expectations if source repair, source artifact attachment, or downstream
metadata readers regress.

Heresy delta: repaired locally for new source repair/artifact metadata labels;
broader metadata, storage, actor-id, app-package, and platform publication
provenance residue remains discovered and out of scope.

Open edge: push the repair, monitor CI/deploy, verify staging identity, and run
a deployed product proof for source gap repair metadata through the
browser/API path.

## Deployed Repair: Source Repair Metadata Labels

Mutation class: `orange`, because this changed new revision metadata emitted
by source repair and source artifact attachment paths, plus frontend source
content item provenance metadata.

Conjecture delta: deployed source repair metadata can teach Texture at the
new-emission boundary while preserving source entity structures, source routes,
storage tables, `.vtext` alias behavior, durable actor ids, and platform
publication attestations for later migration slices.

Protected surfaces: deployed source gap repair revision metadata, deployed
Texture document/revision APIs, Texture desktop document opening, browser-public
route hygiene, staging deployment identity, and focused runtime/frontend tests.

Admissible evidence class: focused local tests, residue search, full CI,
Node B staging deploy identity, and deployed browser/product proof that creates
a source repair through public Texture APIs and observes
`metadata.source="texture_source_gap_repair"`.

Deployed evidence on 2026-06-16:

- Pushed behavior commit:
  `39d0c2ba125c81d59b34002685a9ce19ec98eda0`
  (`runtime: rename texture source metadata labels`), after docs checkpoint
  `9498bae2`.
- CI run `27591835245` passed. Runtime shards 0-3, non-runtime tests,
  integration-tagged smoke, Go vet/build, frontend build, Docs Truth Check job,
  TLA+ model check, final Go gate, and Node B staging deploy job all passed.
- Deploy job `81574215697` succeeded.
- Docs Truth Check run `27591835237` passed; FlakeHub publish run
  `27591835231` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `39d0c2ba125c81d59b34002685a9ce19ec98eda0`, deployed at
  `2026-06-16T03:22:47Z`.
- Deployed Playwright proof
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-source-metadata-deployed.tmp.spec.js`
  passed before the temporary spec was deleted.
- The proof used public product APIs
  `/api/texture/markdown-lineage/import`,
  `/api/texture/documents/{doc}/source-repairs`, and
  `/api/texture/documents/{doc}/revisions`; no browser-public internal or
  test-only routes were used.
- Evidence artifact:
  `/tmp/choir-texture-source-metadata-1781580461671.json`; screenshot:
  `/tmp/choir-texture-source-metadata-1781580461671.png`.
- Product evidence ids: staging user
  `playwright-state-1781580336142-whrv71@example.com`; Texture document
  `8161aac2-4710-46a9-a3a3-2e2f7193b797`; base revision
  `f5ae5dd5-7455-4cfd-8e88-009d923fd4bd`; repaired revision
  `4e0ec188-10a3-4b1a-b4fd-dbcaaf71f0ea`.
- Product observations: repaired revision metadata source was
  `texture_source_gap_repair`, not the retired
  `vtext_source_gap_repair`; repaired content linked the citation to the
  source entity; the Texture desktop app opened the proof document under
  canonical `texture` app identity; the rendered citation transclusion showed
  the source label and excerpt; forbidden browser-public request count was
  zero for `/internal/*`, `/api/agent/*`, `/api/test/*`, `/api/prompts`, and
  `/api/events`.

Rollback path: restore the old emitted `vtext_source_*` metadata values and
test expectations if later source repair, source artifact attachment, or
downstream metadata readers regress.

Heresy delta: repaired for deployed new source repair/artifact metadata labels.
Adjacent metadata keys such as `canonical_vtext_source_path`,
`related_vtexts`, app-package `vtext_doc_id` and `vtext_revision_id`,
platform publication provenance, storage symbols, and durable actor ids remain
discovered residue outside this slice.

## Problem Checkpoint: App Package And Platform Provenance Label Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
frontend source, platform provenance writes, tool schema, prompt default, API,
test, or persistent state changed in this checkpoint.

Read-only search on 2026-06-16 shows a protected evidence/provenance residue
class: AppChangePackage human-proof refs and platform publication provenance
still teach the old artifact ontology even though the app, routes, public
publication URLs, auth intents, and source repair metadata now teach Texture.
This is separate from Universal Wire story projection metadata, general
Texture document metadata keys, storage tables, file suffixes, and durable
actor ids.

Receipts:

- `internal/runtime/tools_shipper.go` still exposes
  `publish_app_change_package` args and schema fields `vtext_doc_id` and
  `vtext_revision_id`, writes the same keys into package provenance refs, and
  describes the human proof narrative as VText.
- `internal/runtime/api_app_promotion.go` still classifies human proof
  narrative refs by keys or values containing `vtext`, and missing-proof copy
  says `narrative VText`.
- `internal/runtime/prompt_defaults/vsuper.md` still instructs candidate
  publishers to produce a causal VText narrative and pass `vtext_doc_id` /
  `vtext_revision_id`.
- `internal/runtime/agent_tools_test.go`,
  `internal/runtime/app_promotion_test.go`, and
  `frontend/tests/web-surface-rationalization.spec.js` still use those old
  app-package evidence field names in current fixtures.
- `internal/platform/service.go` still writes publication provenance and
  verifier records using `private_vtext_revision`,
  `publish_vtext_revision`, `choir-private:vtext/...`, and
  `choir.platform.publish_vtext.v0`.
- `internal/platform/service_publication_read.go` still rewrites
  `private_vtext_revision` citation edges so private revision ids do not leak
  into public bundles.
- Adjacent hits such as `story_vtext_doc_id`, `projection_vtext_docs`,
  `vtext_content`, `source-network-vtext-index`,
  `canonical_vtext_source_path`, `related_vtexts`, durable `vtext:<doc_id>`
  actor ids, storage tables, and `.vtext` file aliases are broader surfaces
  kept out of this slice.

Next behavior slice design:

- emit and document `texture_doc_id` and `texture_revision_id` for new
  AppChangePackage human-proof refs;
- update the human-proof detector and review evidence copy so current
  Texture narrative refs are first-class;
- keep deletion-receipted legacy read compatibility for existing package
  provenance refs only if review-evidence tests prove it is needed;
- emit platform publication provenance as `private_texture_revision`,
  `publish_texture_revision`, `choir-private:texture/...`, and
  `choir.platform.publish_texture.v0`;
- keep public bundle reads from leaking either legacy or current private
  revision ids;
- prove locally with focused runtime app-promotion/shipper tests, platform
  publication tests, frontend fixture tests if touched, residue search for new
  emitters, then push, monitor CI/deploy, verify staging identity, and run a
  deployed product/API proof for AppChangePackage review evidence or platform
  publication provenance if the behavior is reachable on staging without
  manually seeding success records.

## Local Repair: App Package And Platform Provenance Labels

Mutation class: `red`, because this changes protected AppChangePackage
human-proof evidence fields, vsuper package-publishing prompt defaults,
platform publication provenance entities/activities/verifier predicates, and
public bundle private-revision redaction behavior.

Conjecture delta: new package review evidence and platform publication
provenance can teach Texture at the evidence contract boundary while existing
legacy package provenance and legacy platform rows remain readable only behind
deletion-receipted compatibility.

Protected surfaces: AppChangePackage tool schema and provenance refs,
review-evidence human-proof classification, vsuper prompt defaults, platform
publication provenance/citation/verifier rows, public bundle citation
redaction, focused runtime/platform tests, and frontend review-evidence
fixtures.

Local evidence on 2026-06-16:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestPublishAppChangePackageToolPublishesWithoutGitHubPush|TestAppChangePackageReviewEvidenceRequiresNarrativeAndMediaForHumanReview'
  -count=1` passed.
- `nix develop -c go test ./internal/platform -run
  'TestPublishVTextCreatesImmutablePublicRecords|TestInternalPublishRequiresInternalCallerAndBundleResolve'
  -count=1` passed. The focused platform test now asserts stored
  `private_texture_revision`, `choir-private:texture/...`,
  `publish_texture_revision`, and `choir.platform.publish_texture.v0`
  values and still verifies public bundle reads do not leak private revision
  ids.
- `npm --prefix frontend run build` passed. Vite reported the existing
  Universal Wire warnings for unused `currentUser` and `.wire-state`
  selectors.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json` completed report-only with 212 docs and 1,129
  warnings.
- `git diff --check` passed.
- Current-emitter residue search for old package/platform labels across the
  touched runtime/platform/frontend-test files now finds only explicit legacy
  compatibility/read assertions:
  `private_vtext_revision` redaction support in
  `internal/platform/service_publication_read.go`, a no-leak assertion in
  `internal/platform/service_test.go`, and a legacy package-provenance fixture
  in `internal/runtime/app_promotion_test.go`.
- Texture-name search finds the new emitted/proven values:
  `texture_doc_id`, `texture_revision_id`, `private_texture_revision`,
  `choir-private:texture/...`, `publish_texture_revision`, and
  `choir.platform.publish_texture.v0`.
- Focused frontend Playwright attempt
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/web-surface-rationalization.spec.js --grep "package-scoped machine receipts"`
  failed before exercising the package evidence assertions because the test
  still opens the retired `apps-changes` launcher path while the current app
  registry exposes the surface as `features`. This is a stale frontend test
  launcher residue, not AppChangePackage provenance behavior evidence.

Rollback path: restore the old emitted package provenance field names and
platform provenance predicates if AppChangePackage review evidence, platform
publication, public bundle reads, or downstream adoption proof regresses.

Heresy delta: repaired locally for new AppChangePackage and platform
publication provenance labels. Legacy package provenance refs and legacy
platform rows remain deletion-receipted read compatibility until a migration
or deletion receipt removes them. Universal Wire story projection fields,
general Texture metadata keys, durable actor ids, storage symbols, and file
suffixes remain separate discovered residue.

Open edge: push the repair, monitor CI/deploy, verify staging identity, then
run deployed product/API proof for AppChangePackage review evidence or platform
publication provenance without manually seeding success records.

## Deployed Repair: App Package And Platform Provenance Labels

Mutation class: `red`, because this changed protected AppChangePackage
human-proof evidence fields, vsuper package-publishing prompt defaults,
platform publication provenance entities/activities/verifier predicates, and
public bundle private-revision redaction behavior.

Conjecture delta: deployed package review evidence can teach Texture at the
evidence contract boundary while existing legacy package provenance and legacy
platform rows remain readable only behind deletion-receipted compatibility.

Protected surfaces: deployed AppChangePackage create/detail/review-evidence
APIs, package provenance refs, review-evidence human-proof classification,
platform publication provenance/citation/verifier rows, public bundle citation
redaction, staging deploy identity, and browser-public route hygiene.

Deployed evidence on 2026-06-16:

- Pushed behavior commit:
  `24bff527b56e8f76e1ba3066dd5c71d52543120e`
  (`runtime: rename texture package provenance labels`), after docs
  checkpoint `5a7e8a40`.
- CI run `27592592351` passed. Runtime shards 0-3, non-runtime tests,
  integration-tagged smoke, Go vet/build, Docs Truth Check job, TLA+ model
  check, final Go gate, and Node B staging deploy job all passed. The frontend
  build job was skipped by deploy-impact classification because this slice did
  not change deployed frontend source.
- Deploy job `81576474144` succeeded.
- Docs Truth Check run `27592592337` passed; FlakeHub publish run
  `27592592343` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `24bff527b56e8f76e1ba3066dd5c71d52543120e`, deployed at
  `2026-06-16T03:44:38Z`.
- Deployed Playwright proof
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/texture-package-provenance-deployed.tmp.spec.js`
  passed before the temporary spec was deleted.
- The proof used public authenticated product APIs
  `POST /api/app-change-packages`,
  `GET /api/app-change-packages/{id}`, and
  `GET /api/app-change-packages/{id}/review-evidence`; it did not call
  browser-public internal or test-only routes.
- Evidence artifact:
  `/tmp/choir-texture-package-provenance-1781581617265.json`; screenshot:
  `/tmp/choir-texture-package-provenance-1781581617265.png`.
- Product evidence ids: staging user
  `playwright-state-1781581607161-v10dlq@example.com`; package
  `pkg-texture-provenance-1781581617265`; Texture proof document ref
  `doc-texture-package-proof-1781581617265`; Texture proof revision ref
  `rev-texture-package-proof-1781581617265`.
- Product observations: the created package and package detail carried
  `provenance_refs_json.texture_doc_id` and
  `provenance_refs_json.texture_revision_id` with no emitted
  `vtext_doc_id` or `vtext_revision_id`; review evidence returned
  `human_proof.state="human_reviewable"` with narrative refs containing the
  Texture doc and revision ids; review evidence contained no `VText` copy;
  forbidden browser-public request count was zero for `/internal/*`,
  `/api/agent/*`, `/api/test/*`, `/api/prompts`, and `/api/events`.

Rollback path: restore the old emitted package provenance field names and
platform publication provenance predicates if AppChangePackage review evidence,
platform publication, public bundle reads, or downstream adoption proof
regresses.

Heresy delta: repaired for deployed new AppChangePackage and platform
publication provenance labels. Legacy package provenance refs and legacy
platform rows remain deletion-receipted read compatibility until a migration
or deletion receipt removes them. Universal Wire story projection fields,
general Texture metadata keys, durable actor ids, storage symbols, and file
suffixes remain separate discovered residue.

## Problem Checkpoint: `edit_texture` Compatibility Alias

Mutation class: `green` documentation and evidence only. No runtime behavior,
tool registration, prompt default, revision metadata, publication eligibility,
or test surface changed in this checkpoint.

Read-only search on 2026-06-16 shows that `edit_texture` is no longer the
common-path Texture write affordance, but it is still wired into several
separable layers. Removing it as a compatibility alias must not accidentally
remove legacy revision metadata needed for publication reads or turn the tool
loop into a semantic workflow gate.

Receipts:

- `rg -n "edit_texture" internal/runtime internal/wirepublish internal/proxy cmd frontend/tests frontend/src -g '!frontend/dist/**'`
  found current non-doc hits only in `internal/runtime` and
  `internal/wirepublish`: 118 runtime hits and 7 wire-publish hits across 15
  code/test files.
- `internal/runtime/tools_vtext.go` still registers
  `newEditTextureCompatibilityTool(rt)` for Texture and classifies
  `edit_texture` as a Texture write tool in `isTextureWriteToolName`.
- `internal/runtime/tools.go` still treats `edit_texture` as sequential and as
  a duplicate-protected Texture write tool.
- `internal/runtime/runtime.go` still treats `edit_texture` as a terminal
  Texture tool success even though `initialVTextToolChoice` now chooses
  `patch_texture` or `record_texture_decision`.
- `materializeVTextToolEdit` and `addVTextEditRevisionMetadata` still default
  a missing `SourceTool` to `edit_texture`; new `patch_texture` and
  `rewrite_texture` calls set `SourceTool` explicitly, so this is a fallback
  residue rather than the intended new-write path.
- `internal/wirepublish/eligibility.go` and
  `internal/runtime/universal_wire.go` still accept revision metadata
  `source=edit_texture` and legacy `source=edit_vtext` for autonomous wire
  publication eligibility and private publication reads. That is a persisted
  revision metadata compatibility concern, not the same surface as the
  model-visible compatibility tool.
- Test residue is broad: `rg -n "edit_texture" internal/runtime/*_test.go internal/wirepublish/*_test.go internal/proxy/*_test.go frontend/tests -g '!frontend/dist/**'`
  found 112 test hits, including tool-profile exposure tests, duplicate
  Texture write tests, email appagent tests, workflow verifier checks, and
  publication eligibility tests.

Next behavior slice design:

- remove the model-visible `edit_texture` registered tool from the Texture tool
  registry, agent profile expectations, terminal-tool success list, sequential
  side-effect list, and duplicate-write test fixtures;
- change new-write fallback metadata from `edit_texture` to `patch_texture` so
  untagged internal edit paths do not mint new alias metadata;
- keep explicit `source=edit_texture` and `source=edit_vtext` read/eligibility
  compatibility in wire publication and Universal Wire for this slice, with
  tests labeling it as persisted metadata migration residue rather than a live
  tool affordance;
- prove with focused runtime tests that Texture exposes `patch_texture`,
  `rewrite_texture`, and `record_texture_decision` but not `edit_texture`, that
  duplicate write protection still covers `patch_texture`/`rewrite_texture`,
  that no new `edit_texture` tool result is available, and that legacy metadata
  reads remain explicitly supported until a separate migration plan removes
  them.

## Local Repair: `edit_texture` Compatibility Alias Deletion

Mutation class: `red`, because this changes protected Texture tool exposure,
canonical write metadata fallback, tool-loop terminal handling, duplicate write
protection, and Texture writer tests.

Conjecture delta: removing the model-visible `edit_texture` compatibility alias
while preserving explicit legacy revision metadata compatibility should advance
the Texture tool ontology without breaking stored Universal Wire publication
history.

Protected surfaces: Texture tool registry, canonical Texture write metadata,
tool-loop terminal successes, duplicate Texture write protection, Universal
Wire publication eligibility/read compatibility, and Texture appagent tests.

Local evidence on 2026-06-16:

- `nix develop -c go test ./internal/runtime -run 'TestInstallDefaultAgentToolsProfiles|TestExecuteToolsSkipsDuplicateVTextEditsInSameTurn|TestVTextAppagentEditCanonicalizesAliasedMarkdownTitle|TestVTextAgentRevisionMutationCompletedOnlyOnce|TestEditVTextInitialWorkingRevisionDoesNotSmuggleRequiredContinuation|TestEditVTextExplicitResearcherDoesNotForceSpawnContinuation|TestEditVTextExplicitResearcherDoesNotForceSpawnAfterSuperBase|TestEditVTextExplicitResearcherFromBaseRevisionContentSurvivesWorkerPrompt|TestEditVTextExplicitResearcherFromSeedPromptSurvivesRequestIntent|TestEditVTextExplicitResearcherDoesNotDuplicateExistingResearcher|TestVTextTool|TestEmailAppagent'`
  passed.
- `nix develop -c go test ./internal/wirepublish` passed.
- `nix develop -c scripts/go-test-runtime-shards` passed all four runtime
  shards.
- Live-alias residue search
  `rg -n "newEditTextureCompatibilityTool|Name:\s+\"edit_texture\"|decode edit_texture args|executeTextureEditTool\(ctx, \"edit_texture\"|WithTerminalToolSuccesses\([^)]*edit_texture|case \"patch_texture\", \"rewrite_texture\", \"edit_texture\"|sourceTool = \"edit_texture\"" internal/runtime internal/wirepublish --glob '!frontend/dist/**'`
  returned no hits.
- Broad current-code search
  `rg -n "edit_texture" internal/runtime internal/wirepublish --glob '!frontend/dist/**'`
  now finds only explicit forbidden-tool assertions and legacy
  `source=edit_texture` metadata compatibility tests/read predicates.

Deployed evidence on 2026-06-16:

- Commit `c6db0df57bd06a22e392fd89eb0f4ee1f4c1bcc1` passed CI run
  `27589732107`; deploy job `81567905099` succeeded.
- Staging health at `https://choir.news/health` reported proxy and sandbox
  commit `c6db0df57bd06a22e392fd89eb0f4ee1f4c1bcc1`, deployed at
  `2026-06-16T02:22:51Z`.
- Deployed Playwright product proof registered a fresh staging user, submitted
  prompt-bar request `d2a0ccf4-276f-43f2-be6b-f6da43fdaf15`, and received
  conductor -> Texture decision for document
  `d4e62340-bd4c-4644-9fd6-fb28a2b85d30`.
- The Texture head revision `f5fee46f-4178-4dc2-aee3-fe127525cd9b` had
  `metadata.source=patch_texture` and content
  "Current write tool: patch_texture. Do not call any retired compatibility
  alias."
- Trace for trajectory `d2a0ccf4-276f-43f2-be6b-f6da43fdaf15` contained
  conductor and Texture agents only, 28 moments, two `patch_texture returned`
  tool-result moments, four non-error `patch_texture` tool events, zero
  `rewrite_texture` hits, zero `edit_texture` hits, and zero `super` hits.
- The deployed UI proof found one Texture window, zero legacy `vtext` windows,
  visible `patch_texture` content, no visible `edit_texture`, no
  "Writing first draft" placeholder, and no forbidden browser requests to
  `/internal/*`, `/api/agent/*`, `/api/test/*`, `/api/prompts`, or
  `/api/events`.

Rollback path: restore the `edit_texture` registered tool, write-tool
classification, terminal success entry, duplicate-write handling entry, and
`edit_texture` metadata fallback if deployed Texture writers cannot use
`patch_texture` or `rewrite_texture`.

Heresy delta: repaired for the deployed model-visible `edit_texture`
compatibility alias; legacy `source=edit_texture` and `source=edit_vtext`
metadata compatibility remains discovered migration residue.

## Problem Checkpoint: Universal Wire Story Projection Label Residue

Mutation class: `green` documentation and evidence only. No runtime behavior,
frontend source, API contract, test fixture, storage, file alias, platform
publication, or persistent state changed in this checkpoint.

Read-only search on 2026-06-16 shows that Universal Wire's live story
projection contract still teaches the retired artifact ontology at the product
API and frontend-open boundary. The residue is bounded to the Universal Wire
story projection fields and story/source-state labels; broader `.vtext`
shortcut files, storage table names, durable `vtext:` actor labels, source
metadata keys, and Style.vtext prompt/style-source language remain separate
migration surfaces.

Conjecture delta: new Universal Wire story projections can emit Texture-named
document/content fields and source-state labels while the frontend consumes the
current Texture fields first and, only if needed, carries deletion-receipted
legacy read fallback for existing staged or persisted story payloads.

Protected surfaces: `/api/universal-wire/stories` response JSON, runtime story
publication verification checks, Universal Wire frontend story-open and related
Texture launch context, source-state labels, focused runtime/frontend tests,
and deployed browser product proof for opening a Universal Wire story as
Texture.

Admissible evidence class: focused Universal Wire runtime tests, frontend build
or focused Universal Wire Playwright tests, residue search for the old emitted
story projection fields in current code, CI/deploy identity if behavior changes
land, and deployed staging product proof through public authenticated product
paths.

Rollback path: restore the prior Universal Wire story field emitters and
frontend consumers if story indexing, platform publication verification,
related Texture launches, or Universal Wire frontend rendering regresses.

Heresy delta: discovered: Universal Wire still emits and consumes old-name
story projection labels after route/tool/app/provenance cutovers. Introduced:
none in this checkpoint. Repaired target: new Universal Wire story projection
payloads and frontend story launches should teach Texture without pretending
that storage, file suffix, actor id, or style-source residue is fixed.

Receipts:

- `internal/types/wire.go` still defines `ProjectionVTextDocs` with JSON
  `projection_vtext_docs`, `StoryVTextDoc` with JSON
  `story_vtext_doc_id`, and `VTextContent` with JSON `vtext_content`.
- `internal/runtime/universal_wire.go` still emits story ids and source state
  labels such as `source-network-vtext-*` and
  `source-network-vtext-index`, uses `StoryVTextDoc` for platform publication
  verification, and returns `universal-wire-vtext-index` /
  `universal-wire-edition-vtext` source labels.
- `frontend/src/lib/UniversalWireApp.svelte` still reads
  `story_vtext_doc_id`, creates related entities as `gw-vtext-*`, uses
  `target_kind: 'vtext_document'`, and opens story source paths ending in
  `.story.vtext`.
- `frontend/tests/universal-wire-app.spec.js` still stubs
  `source-network-vtext-*`, `universal-wire-vtext-index`, and visible copy
  saying `VText article`.
- Adjacent residue kept out of this slice includes `internal/store/vtext.go`
  storage tables, `platform_vtext_documents`, `.vtext` shortcut/alias files,
  durable `vtext:<doc_id>` author labels, `related_vtexts` metadata,
  source-renderer `vtext_document` compatibility beyond Universal Wire launch
  context, and Style.vtext selection prompt language.

Next behavior slice design:

- emit `projection_texture_docs`, `story_texture_doc_id`, and
  `texture_content` from Universal Wire stories;
- rename new source labels toward `source-network-texture-*`,
  `source-network-texture-index`, and `universal-wire-*-texture` where they
  are current payload/state labels rather than persisted storage keys;
- update Universal Wire frontend story-open and related launch code to consume
  Texture fields first, with legacy fallback only if tests prove current
  payload compatibility needs it;
- keep `.vtext` shortcut file names, durable `vtext:` actor labels, storage
  tables, and general `related_vtexts` metadata out of scope for this slice;
- prove locally with focused runtime Universal Wire tests, frontend build or
  focused Universal Wire Playwright tests, residue searches for the old story
  projection emitters, then push, monitor CI/deploy, verify staging identity,
  and run deployed Universal Wire product proof if the behavior is reachable
  without manually seeding success records.

## Local Repair: Universal Wire Story Projection Labels

Mutation class: `orange` with red-adjacent evidence boundaries, because this
changes the browser-public `/api/universal-wire/stories` story projection JSON,
runtime story publication verification references, Universal Wire frontend
launch context, deployed staging acceptance expectations, and current
Universal Wire tests. It does not change canonical Texture writes, storage
tables, `.vtext` shortcut files, durable actor ids, or platform publication
route registration.

Conjecture delta: Universal Wire can publish current story projection payloads
with Texture-named document/content fields and source labels while retaining
only frontend legacy-read fallback for old `story_vtext_doc_id` payloads until
staging proves the new payload shape.

Protected surfaces: `/api/universal-wire/stories`, `types.WireStory` JSON,
Universal Wire story indexing, platform publication verification, Texture API
read owner resolution for platform-owned Universal Wire docs, Universal Wire
frontend story launch and related Texture entity construction, and the deployed
Universal Wire staging acceptance spec.

Local evidence on 2026-06-16:

- `types.WireStory` now emits `projection_texture_docs`,
  `story_texture_doc_id`, and `texture_content`; the focused runtime test
  marshals a story and asserts those JSON keys exist while
  `projection_vtext_docs`, `story_vtext_doc_id`, and `vtext_content` do not.
- Universal Wire source labels now emit `universal-wire-texture-index`,
  `universal-wire-edition-texture`, `source-network-texture-*`, and
  `source-network-texture-index`.
- `frontend/src/lib/UniversalWireApp.svelte` now defaults to
  `universal-wire-texture-index`, creates related entities as `gw-texture-*`
  with `target_kind: 'texture_document'`, opens stories through
  `story_texture_doc_id` first, and keeps `story_vtext_doc_id` only as an
  explicit legacy payload fallback. The `.story.vtext` source path and
  `relatedVTexts` app-context property remain broader metadata/file-suffix
  residue outside this slice.
- `frontend/tests/universal-wire-staging-acceptance.spec.js` now treats
  `universal-wire-edition-texture` as the edition payload and asserts
  `story_texture_doc_id` is present while `story_vtext_doc_id` is absent.
- Focused runtime test:
  `nix develop -c go test ./internal/runtime -run 'TestHandleUniversalWireStories|TestResolveUniversalWireTextureReadOwner|TestNormalizeWireArticleSourceServiceProse|TestWireAutonomousPublishTranscludesEditionAndDebounces|TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails' -count=1`
  passed.
- Runtime shard coverage:
  `nix develop -c scripts/go-test-runtime-shards` passed.
- Frontend build:
  `npm --prefix frontend run build` passed with the existing Universal Wire
  warnings for unused `currentUser` and `.wire-state` selectors.
- Current-code residue search
  `rg -n "ProjectionVTextDocs|StoryVTextDoc|VTextContent|projection_vtext_docs|story_vtext_doc_id|vtext_content|source-network-vtext|universal-wire-vtext-index|universal-wire-edition-vtext" internal frontend/src frontend/tests -g '!frontend/dist/**'`
  now finds only explicit legacy fallback/negative assertions:
  two fallback reads in `UniversalWireApp.svelte`, one staging negative
  assertion, and three runtime JSON absence assertions.

Rollback path: restore the old `WireStory` JSON fields, Universal Wire source
labels, and frontend consumers if staging shows story indexing, platform
publication verification, signed-in Universal Wire rendering, or Texture story
launches regress.

Heresy delta: repaired locally for current Universal Wire story projection
emitters, source-state labels, and frontend launch context. Discovered residue
remaining outside this slice includes `.vtext` aliases/source paths,
`vtext:` edition transclusion syntax, durable `vtext:<doc_id>` author labels,
`vtext_agent_revision` metadata types, Style.vtext style-source language,
general `related_vtexts` metadata, storage tables, and platform table names.

## Staging Evidence Checkpoint: Universal Wire Empty Edition Acceptance Gap

Mutation class: `green` documentation and evidence only. No test, runtime,
frontend, product API, staging state, or persistent data changed in this
checkpoint.

Staging evidence on 2026-06-16 after deploying commit
`9f332529d209e82df86056176ffac2d31d2c5df1` exposed an acceptance-oracle gap:
the deployed Universal Wire stories API returned the new
`universal-wire-edition-texture` source label with an empty `stories` array.
The staging acceptance spec assumed that an edition source always implies at
least one story and failed before it could observe a deployed story payload's
`story_texture_doc_id` field.

Receipts:

- Pushed behavior commit:
  `9f332529d209e82df86056176ffac2d31d2c5df1`
  (`runtime: rename texture wire projection labels`), after docs checkpoint
  `e7a61b9e`.
- CI run `27593330137` passed, including all runtime shards, non-runtime tests,
  integration-tagged smoke, Go vet/build, Docs Truth Check job, TLA+ model
  check, frontend build, final Go gate, and staging deploy.
- Deploy job `81578635355` succeeded.
- Docs Truth Check run `27593330130` passed; FlakeHub publish run
  `27593330160` passed.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `9f332529d209e82df86056176ffac2d31d2c5df1`, deployed at
  `2026-06-16T04:05:57Z`.
- Deployed Playwright proof attempt
  `GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/universal-wire-staging-acceptance.spec.js`
  failed because `stories.source === "universal-wire-edition-texture"` and
  `stories.stories.length === 0`; the assertion expected length greater than
  zero.

Conjecture delta: the deployed proof must distinguish "Universal Wire source
labels and app surface are cut over" from "staging currently has at least one
edition story payload available to inspect." Empty editions can still prove the
source-label and app-empty-state parts of C22, but they cannot prove deployed
story payload field names until staging has an edition story or a product path
creates one without manually seeding success records.

Rollback path: no runtime rollback is indicated by this evidence alone. If the
empty edition reflects a story-indexing regression rather than normal staging
data shape, restore the prior Universal Wire story projection behavior while
investigating platform publication verification.

Heresy delta: discovered: the Universal Wire deployed acceptance oracle
conflated edition existence with story availability. Introduced: none in this
checkpoint. Repaired target: update the acceptance spec to pass on empty
edition state while still asserting Texture labels and app surface, and leave
deployed story-field proof open until an actual story payload is reachable.

## Deployed Evidence: Universal Wire Texture Source Labels

Mutation class: `yellow` for the acceptance-spec repair and `green` for this
evidence update. The deployed runtime/frontend behavior under proof remains
commit `9f332529d209e82df86056176ffac2d31d2c5df1`.

Conjecture delta: after the acceptance oracle distinguishes empty edition state
from story payload availability, staging can prove the Universal Wire source
label and app-surface parts of C22 without overstating deployed story-field
coverage.

Deployed evidence on 2026-06-16:

- Refreshed staging Playwright auth state with
  `node scripts/setup-auth-state.mjs --baseUrl https://choir.news` from
  `frontend/`; generated user
  `qa-1781583037734-7tuzeq@example.com`.
- Deployed Playwright proof
  `GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/universal-wire-staging-acceptance.spec.js`
  passed after the spec was corrected to accept empty editions.
- Product observations: `/api/universal-wire/stories` returned a
  `universal-wire-*` source label that was not
  `universal-wire-vtext-index` or `universal-wire-edition-vtext`; the response
  included the Universal Wire edition at `universal-wire/Wire.vtext`; the
  signed-in Universal Wire app rendered without SourceMaxx or Global Wire
  preview copy; because `stories.length === 0`, the app rendered the
  Universal Wire empty state and no story cards.
- Deployed story payload fields remain unproven on staging because no
  Universal Wire edition story payload was available. Local focused tests and
  runtime shards prove the emitted JSON shape for current story payloads.

Rollback path: if a future staging edition story exposes old story payload
fields or the app cannot open a story through `story_texture_doc_id`, restore
the previous Universal Wire story consumer/producer while investigating the
projection contract.

Heresy delta: repaired for deployed Universal Wire source-label and empty-state
app proof. Discovered but unrepaired for deployed scope: staging currently lacks
a Universal Wire story payload to prove `story_texture_doc_id`,
`projection_texture_docs`, and `texture_content` end to end.

## Problem Checkpoint: Canonical Texture Source Path Metadata

Mutation class: `green` documentation and evidence only. No runtime behavior,
frontend source, API contract, test fixture, storage, file alias, platform
publication, or persistent state changed in this checkpoint.

Read-only search on 2026-06-16 shows that current Texture revision writers
still emit the durable source-path metadata key
`canonical_vtext_source_path`. This key is a narrower surface than `.vtext`
shortcut files, `vtext_documents` storage tables, durable `vtext:<doc_id>`
actor ids, Style.vtext style-source language, and `/pub/vtext/...` public
route compatibility. It is nevertheless a current writer/carry-forward contract
on ordinary user and appagent Texture revisions, so changing it is a runtime
metadata repair rather than docs cleanup.

Conjecture delta: current Texture revisions can write
`canonical_texture_source_path` while legacy revisions carrying
`canonical_vtext_source_path` remain readable and can be carried forward into
the canonical Texture-named key on the next revision. The repair should not
rename `.vtext` file suffixes, document titles, storage table names, or actor
ids in the same slice.

Protected surfaces: user revision creation, appagent `patch_texture` revision
creation, durable metadata carry-forward, file-open projection alias creation,
Markdown/source-lineage import metadata, focused runtime tests, and frontend
tests that inspect markdown-lineage metadata.

Admissible evidence class: focused runtime tests covering file-open user
revision carry-forward, structure stabilization, and appagent patch revisions;
focused frontend markdown-lineage tests that assert the new metadata key; a
residue search proving current writers/tests no longer require
`canonical_vtext_source_path` except explicit legacy compatibility; CI/deploy
identity if the metadata writer change is pushed.

Rollback path: restore `canonical_vtext_source_path` as the emitted durable
metadata key and remove the Texture-name promotion if file-open, revision
history, import lineage, or appagent patch revisions lose source-path lineage.

Heresy delta: discovered: a current durable metadata key still teaches the old
artifact ontology after the source repair, package provenance, related
transclusion, and source-contract cutovers. Introduced: none in this
checkpoint. Repaired target: current revisions should emit
`canonical_texture_source_path`; legacy `canonical_vtext_source_path` should
remain read-compatible only.

Receipts:

- `internal/runtime/vtext.go` writes `canonical_vtext_source_path` on user
  revision creation after `ensureCanonicalVTextProjectionPath`.
- `internal/runtime/tools_vtext.go` writes `canonical_vtext_source_path` on
  appagent-authored `patch_texture` revisions.
- `internal/runtime/runtime.go` lists `canonical_vtext_source_path` in
  `durableMetadataKeys`, so generic carry-forward preserves the retired key.
- `internal/runtime/vtext_structure.go` carries durable keys from parent
  revisions without alias promotion.
- `internal/runtime/vtext_test.go` and
  `frontend/tests/vtext-markdown-lineage.spec.js` assert the retired key as
  the current expected metadata.

Next behavior slice design:

- introduce a Texture-named metadata key for the current writer path:
  `canonical_texture_source_path`;
- preserve read compatibility by promoting legacy
  `canonical_vtext_source_path` from parent/run metadata into
  `canonical_texture_source_path` when creating a new revision;
- keep `.vtext` filename/title/alias suffixes, storage table names,
  `vtext:` actor ids, `/pub/vtext` public routes, and Style.vtext language out
  of scope;
- update focused runtime/frontend tests and residue searches, then push,
  monitor CI/deploy, and use staging identity as the deployed evidence class
  unless a product-path metadata proof is needed.

## Problem Checkpoint: Publication Fallback Texture Labels

Mutation class: `green` documentation and evidence only. No runtime behavior,
frontend source, API contract, route minting, export bytes, test fixture, or
persistent state changed in this checkpoint.

Read-only search on 2026-06-16 shows that current publication fallback/default
writers still emit the retired artifact name in user-visible or exported
publication surfaces:

- `internal/platform/publication_document.go` falls back to `Published VText`
  when building a publication document without an explicit publication title;
- `internal/platform/export_docx.go` writes `Published VText` into DOCX core
  properties when the publication title is empty;
- `internal/platform/service.go` defaults untitled platform publication writes
  to `Untitled VText` and publication proposals to `VText proposal`;
- `internal/platform/service_publication_read.go` defaults export filenames to
  `published-vtext.<format>` when neither slug nor title supplies a basename;
- `frontend/tests/vtext-source-service-publication.spec.js` still expects the
  published reader accessibility label to be `Published VText document`, while
  the current frontend source already renders `Published Texture document`.

This is narrower than public route identity (`/pub/vtext/...`), storage table
names, `PublishVText` Go type/function names, and exported HTML/CSS class
names. It is nevertheless a current writer/default surface: new untitled
publications, proposals, generated export metadata, export filenames, and
frontend acceptance expectations should teach Texture rather than the retired
ontology.

Conjecture delta: current publication fallback/default values can switch to
Texture without changing live public-route compatibility or broad platform API
symbol names. The repair should preserve explicit user-provided titles and
slugs, keep `/pub/vtext/...` legacy public reads out of scope, and avoid
renaming `PublishVText` APIs in the same slice.

Protected surfaces: platform publication default titles, proposal default
titles, publication document construction, DOCX core metadata, export filename
basenames, published-reader accessibility assertions, focused platform tests,
frontend build/tests, and staging publication/read/export proof after push.

Admissible evidence class: focused platform tests covering publication
creation/read/export defaults, frontend build and focused publication reader
test coverage, residue search proving the scoped fallback/default strings no
longer appear except explicit legacy compatibility or historical evidence,
CI/deploy identity, and deployed product-path proof that a new publication
mints Texture-named default reader/export surfaces.

Rollback path: restore the V-name fallback/default strings and test
expectations if route minting, publication reads, proposals, export filenames,
DOCX metadata, or published-reader accessibility regress.

Heresy delta: discovered: after public routes, app identity, source metadata,
and source-path metadata cutovers, publication fallback/default writers still
mint owner-visible old ontology. Introduced: none in this checkpoint. Repaired
target: current publication fallback/default writers should mint
Texture-named labels while broad Go API names, storage names, public legacy
routes, and exported CSS class names remain separately classified residue.

Next behavior slice design:

- change publication fallback document/export title values to
  `Published Texture`;
- change untitled publication and proposal default titles to `Untitled Texture`
  and `Texture proposal`;
- change default export filename basenames to `published-texture`;
- update the published reader acceptance expectation to
  `Published Texture document`;
- add or update focused tests for default publication titles/export filenames
  where existing coverage is missing, then push, monitor CI/deploy, and prove
  the behavior on staging through product publication/read/export surfaces.

## Problem Checkpoint: Exported HTML Texture Class Names

Mutation class: `green` documentation and evidence only. No runtime behavior,
export bytes, frontend source, test fixture, public route, API contract, or
persistent state changed in this checkpoint.

Read-only search on 2026-06-16 shows that the platform HTML publication export
still emits retired-name CSS classes in generated artifacts:

- `internal/platform/export_html.go` writes
  `class="vtext-publication"` on the exported `<article>`;
- table exports use `class="vtext-table"` and profile CSS selectors
  `.vtext-table`;
- source citation links use `class="vtext-source-ref"` and profile CSS
  selectors `.vtext-source-ref`;
- the source list uses `.vtext-sources` plus `vtext-sources-heading`;
- `internal/platform/service_test.go` asserts those old classes as the
  expected HTML export contract.

This is not the same surface as the live editor's internal `.vtext-source-ref`
classes, storage tables, durable `vtext:` actor ids, `.vtext` file aliases, or
`/pub/vtext/...` legacy route identity. It is a current exported artifact
contract, however: new downloaded/published HTML should teach Texture in its
semantic CSS hooks and accessibility ids.

Conjecture delta: current HTML exports can switch their generated artifact
classes and ids to Texture names without changing source manifests,
publication routes, JSON-LD, profile metadata, or the live editor renderer.
The repair should not attempt broad frontend CSS class migration in the same
slice.

Protected surfaces: platform HTML export rendering, embedded export profile
CSS, source citation anchors, source-list accessibility ids, focused platform
tests, deployed publication HTML export proof, and any downstream consumers of
new exported HTML class names.

Admissible evidence class: focused platform tests asserting Texture-named HTML
export classes/ids and old-class absence, current-code residue search proving
the scoped export classes no longer appear outside negative assertions or
separate live-editor residue, CI/deploy identity, and deployed product-path
proof that a new HTML publication export from staging contains Texture-named
classes and no retired export classes.

Rollback path: restore the previous V-name HTML classes/ids and test
expectations if generated HTML layout, source anchors, source lists, or export
profile styling regresses.

Heresy delta: discovered: after route, source-contract, fallback-label, and
metadata repairs, exported HTML artifacts still carry old ontology in CSS
hooks. Introduced: none in this checkpoint. Repaired target: new platform HTML
exports should emit Texture-named artifact classes while live editor CSS,
storage names, actor ids, file suffixes, and public legacy routes remain
separate residue classes.

Next behavior slice design:

- rename generated HTML export classes/ids from `vtext-publication`,
  `vtext-table`, `vtext-source-ref`, and `vtext-sources*` to
  `texture-publication`, `texture-table`, `texture-source-ref`, and
  `texture-sources*`;
- update embedded export profile CSS selectors to match the generated classes;
- update focused platform tests for the HTML export contract and old-class
  absence;
- keep live editor `.vtext-source-ref` classes, frontend renderer classes,
  storage, actor ids, `.vtext` suffixes, and `/pub/vtext` compatibility out of
  scope;
- push, monitor CI/deploy, and prove the new exported HTML artifact surface on
  staging through product publication/export APIs.

## Problem Checkpoint: Live Editor Texture Source Classes

Mutation class: `green` documentation and evidence only. No frontend source,
runtime behavior, rendered DOM, CSS, tests, storage, API contract, public route,
or persistent state changed in this checkpoint.

Read-only search on 2026-06-16 shows that the live Texture renderer still emits
and styles retired-name CSS classes in current editor and published-reader DOM:

- `frontend/src/lib/vtext-source-renderer.ts` emits
  `vtext-source-ref`, `vtext-source-ref--missing`,
  `vtext-source-ref-label`, `vtext-source-ref-popover`,
  `vtext-transclusion-body`, `vtext-transclusion-quote`,
  `vtext-source-facts`, and `vtext-source-open`;
- `frontend/src/lib/VTextEditor.svelte` styles the live source-ref DOM through
  `.vtext-source-ref*` selectors;
- `frontend/src/lib/vtext-source-flow.ts` creates source journal flow DOM with
  `vtext-source-journal-*`, `vtext-source-flow-close`, and
  `vtext-source-open` classes, and uses `--vtext-source-flow-*` CSS variables;
- `frontend/src/lib/vtext-source-flow.css` styles the source journal flow
  through `.vtext-source-journal-*`, `.vtext-source-ref*`, and
  `--vtext-source-flow-*`;
- focused frontend tests still inspect some of those old class names for source
  flow geometry and old-card absence.

This is narrower than renaming frontend module/file names such as
`vtext-source-flow.ts`, app/editor component names, storage schema, `.vtext`
file suffixes, durable `vtext:` actor ids, `PublishVText` Go symbols, and
`/pub/vtext/...` public route compatibility. It is broader than a pure selector
cleanup because the old classes are emitted into live product DOM and govern
source transclusion interaction styling.

Conjecture delta: live Texture source-ref and source-flow DOM classes can move
to Texture names while preserving stable `data-texture-*` behavioral selectors,
Markdown serialization, source popovers, journal flow layout, source-open
buttons, and published-reader behavior. The repair should not rename frontend
file/module names or unrelated `vtext-related-ref` and transclusion body classes
in the same slice unless a focused test proves they are part of the same source
class contract.

Protected surfaces: frontend source-ref rendering, Markdown serialization,
source journal flow layout, source popover styling, source-open controls,
published-reader source interaction, focused Playwright tests, frontend build,
and staging browser proof for live source refs and journal source flow.

Admissible evidence class: focused frontend tests covering source-ref rendering
and source journal flow geometry, frontend build, residue search proving the
scoped emitted/styled source classes no longer use the retired name except
explicit negative assertions or out-of-scope file/module names, CI/deploy
identity, and deployed browser proof that a new Texture with source refs renders
Texture-named source classes and no old source-ref/source-flow classes.

Rollback path: restore the previous V-name live source classes/selectors and
test expectations if source refs lose styling, popovers, journal-flow layout,
source-open behavior, Markdown serialization, or published-reader source
interaction.

Heresy delta: discovered: after exported HTML artifacts moved to Texture
classes, the live editor/published-reader renderer still exposes old ontology
through product DOM classes. Introduced: none in this checkpoint. Repaired
target: current live source-ref/source-flow DOM should emit Texture-named
classes while module filenames, storage, actor ids, file suffixes, Go API
symbols, and public legacy routes remain separately classified residue.

Next behavior slice design:

- rename live source-ref classes from `vtext-source-ref*` to
  `texture-source-ref*`;
- rename source journal flow classes and CSS variables from
  `vtext-source-journal-*`, `vtext-source-flow-close`,
  `vtext-source-open`, and `--vtext-source-flow-*` to Texture names;
- update CSS, TypeScript DOM construction/querying, Markdown serialization, and
  focused frontend tests to use the new class names while keeping
  `data-texture-*` selectors stable;
- keep frontend file/module names, storage schema, `.vtext` suffixes, durable
  actor ids, `PublishVText` API symbols, `/pub/vtext` routes, and unrelated
  related-ref/transclusion-body classes out of scope;
- push, monitor CI/deploy, and prove the live DOM source class surface on
  staging through product browser evidence.

## Problem Checkpoint: Public Legacy Publication Routes

Mutation class: `green` documentation and evidence only. No frontend source,
runtime behavior, public route resolution, platform storage, API contract, test,
or persistent state changed in this checkpoint.

Read-only search on 2026-06-16 shows that the old public route ontology remains
partly active outside historical evidence:

- `frontend/src/App.svelte` treats both `/pub/texture/...` and `/pub/vtext/...`
  as public Texture route paths during first-page load;
- `frontend/src/lib/Desktop.svelte` normalizes both `/pub/texture/...` and
  `/pub/vtext/...` when opening public publication routes inside the desktop;
- `frontend/tests/vtext-source-entities.spec.js` uses `/pub/vtext/...` fixture
  paths in publication source-reader tests, so local frontend evidence still
  trains current tests on old public route spelling;
- `internal/platform/service.go` defines `legacyPublicVTextPrefix =
  "/pub/vtext/"` and `normalizePublicationRoutePath` trims trailing slashes for
  stored legacy public route rows;
- `internal/platform/service_test.go` manually inserts a legacy `/pub/vtext/...`
  route row and asserts backend bundle resolution still works;
- `internal/proxy/platform_public_test.go` verifies public resolve/export
  return 404 for an unresolved `/pub/vtext/private` route, which proves the
  proxy forwards the route to platformd instead of rejecting old public route
  spelling at the proxy boundary.

This residue is narrower than storage table names, durable `vtext:` actor ids,
`.vtext` file suffixes, and public API route shims. It is broader than a
frontend string cleanup because `/pub/...` paths are user-visible publication
URLs and route compatibility can affect existing stored publication rows.

Conjecture delta: current user-facing browser/UI route recognition can stop
canonizing arbitrary `/pub/vtext/...` paths while backend platformd keeps a
small, explicitly documented legacy-row read shim until a later storage/public
route migration decides whether to rewrite or delete those rows. New
publication minting already uses `/pub/texture/...`; this slice should move
frontend/public-reader fixtures to `/pub/texture/...` and document the backend
legacy prefix as remaining compatibility residue instead of silently treating it
as current product vocabulary.

Protected surfaces: public publication URLs, first-load public route detection,
desktop public route normalization, published-reader/source-reader fixtures,
platform route normalization, proxy publication resolve/export behavior, and
staging publication proof for current `/pub/texture/...` routes.

Admissible evidence class: frontend build and focused tests/search proving
current browser/UI/source-reader fixtures use `/pub/texture/...`; focused
platform/proxy tests proving generated routes stay `/pub/texture/...` and the
legacy backend shim remains explicit; CI/deploy identity if runtime/frontend
behavior changes; deployed product proof that a newly published Texture uses and
loads through `/pub/texture/...`. This slice does not claim storage migration
or deletion of existing legacy public route rows.

Rollback path: restore frontend recognition of `/pub/vtext/...` as a public
route if deployed publication readers or source windows regress, and retain the
backend legacy read shim until a separately documented storage migration
provides stronger evidence.

Heresy delta: discovered: after current publication minting moved to
`/pub/texture/...`, browser/UI route recognition and source-reader fixtures
still normalize old public route spelling as if it were current. Introduced:
none in this checkpoint. Repaired target: current public route product and
frontend proof surfaces should speak `/pub/texture/...`; backend `/pub/vtext/...`
support remains an explicitly named compatibility shim with a deletion/migration
edge.

Next behavior slice design:

- remove `/pub/vtext/...` from frontend first-load public route recognition and
  desktop route normalization;
- update frontend source-reader/publication fixtures from `/pub/vtext/...` to
  `/pub/texture/...`;
- add or update focused tests/search so current frontend surfaces no longer
  use `/pub/vtext/...`;
- leave `internal/platform` legacy route normalization and proxy forwarding
  behavior in place with explicit comments/receipts, because rewriting stored
  public route rows is a separate storage migration;
- push, monitor CI/deploy for behavior changes, and prove a newly published
  Texture opens through `/pub/texture/...` on staging.

## Problem Checkpoint: Universal Wire Style Texture Suffixes

Mutation class: `green` documentation and evidence only. No prompt text,
runtime behavior, style source metadata, tests, API contract, import/export
logic, storage schema, file-browser behavior, or persistent state changed in
this checkpoint.

Read-only search on 2026-06-16 shows a bounded style-source residue class
inside Universal Wire and coagent prompt construction:

- `internal/runtime/tools_coagent.go` emits `## Style.vtext Source`, `Selected
  Style.vtext source context`, reader-facing exclusion rules mentioning
  `Style.vtext`, default style source titles such as `Style.vtext: Universal
  Wire`, default source paths such as `styles/universal-wire.style.vtext`, and
  style-selection rationales ending in `Style.vtext`;
- `internal/runtime/universal_wire.go` still supplies the default title
  `Style.vtext: Universal Wire`, trims `.vtext` from story-derived headlines,
  and filters generated content headings named `Style.vtext Source`;
- `internal/runtime/tool_profiles.go` and
  `internal/runtime/prompt_defaults/processor.md` still instruct agents to pass
  `Style.vtext` needs;
- runtime tests in `internal/runtime/{runtime,universal_wire,agent_tools}_test.go`
  assert `Style.vtext` prompt content and metadata.

This is narrower than canonical file suffix migration. It does not change
`.vtext` import/open behavior, file-browser recognition, alias ordering,
workspace paths, migration adapter names, metadata compatibility keys, durable
`vtext:` actor ids, or stored document titles. It is broader than test wording:
the old label appears in runtime prompt contracts that shape Universal Wire
article drafting and source/style metadata.

Conjecture delta: Universal Wire style-source labels, style source paths, and
prompt instructions can move from `Style.vtext` / `.style.vtext` to
`Style.texture` / `.style.texture` while preserving the same selected style
source semantics, article-head completion contract, and content filters. The
legacy content filter should continue removing old `Style.vtext Source`
headings as historical/generated cleanup, but current prompts and defaults
should no longer introduce those headings.

Protected surfaces: coagent processor/reconciler prompts, default Wire style
source metadata, Universal Wire article projection cleanup, tests that assert
style-source prompt contracts, and downstream Wire publication eligibility that
depends on selected style metadata.

Admissible evidence class: focused runtime tests covering Universal Wire prompt
construction and processor handoff, residue search proving current style-source
defaults/prompts no longer introduce `Style.vtext` outside explicit legacy
cleanup or negative assertions, CI/deploy identity if behavior changes land,
and staging/product evidence only if a deployed Wire story-field proof becomes
available through product paths. This slice does not claim canonical `.texture`
file import/storage migration.

Rollback path: restore previous `Style.vtext` prompt labels, style source
paths, and expectations if Universal Wire style selection, article prompt
contracts, or generated-content cleanup regress.

Heresy delta: discovered: after public route and source-class repairs, current
Universal Wire style prompts still teach agents to think in `Style.vtext`
documents. Introduced: none in this checkpoint. Repaired target: current
style-source prompt/default surfaces should speak `Style.texture` while the
legacy `.vtext` file/import/storage migration remains separate and explicit.

Next behavior slice design:

- rename current style-source labels from `Style.vtext` to `Style.texture`;
- rename default style source paths from `styles/*.style.vtext` to
  `styles/*.style.texture`;
- update processor/reconciler prompt instructions and focused runtime tests to
  expect `Style.texture`;
- keep cleanup filters that recognize old `Style.vtext Source` headings as
  legacy generated-content sanitizers, but add/confirm current `Style.texture`
  cleanup paths as well;
- keep canonical `.vtext` file import/open behavior, file-browser shortcuts,
  storage aliases, metadata compatibility keys, durable actor ids, and protocol
  v0 out of C30.

## Repair: Universal Wire Style Texture Suffixes

Mutation class: `orange`, because this changes runtime prompt/default text,
Wire style-source metadata, Universal Wire article cleanup behavior, and tests
that encode agent handoff contracts.

Conjecture delta: current Universal Wire style-source prompt/default surfaces
can speak `Style.texture` and use `.style.texture` source paths while preserving
the same style selection semantics, article-head completion contract, and
generated-content cleanup. Legacy `Style.vtext Source` cleanup remains a
scoped generated-content sanitizer, not a current prompt/default source.

Protected surfaces: coagent processor/reconciler prompts, default Wire style
source metadata, Universal Wire story projection cleanup, processor prompt
defaults, and focused tests that assert style-source handoff contracts.

Local evidence on 2026-06-16:

- Problem checkpoint commit
  `a59b86f2acffb669a851c44c75b03a5db7b6c514` landed the documentation-first
  record; Docs Truth Check run `27597206898` passed.
- Current style-source labels and paths in
  `internal/runtime/tools_coagent.go` now use `Style.texture` and
  `styles/*.style.texture`.
- Coagent revision prompts, runtime tool profiles, processor prompt defaults,
  and focused tests now expect `Style.texture` handoff language.
- Universal Wire article cleanup strips both current `Style.texture Source`
  and legacy `Style.vtext Source` headings.
- `nix develop -c go test ./internal/runtime -run
  'TestHandleUniversalWireStories|TestWireArticle|TestCoagent|TestProcessor|Test.*Style|TestVTextPrompt|TestAgentTools|TestSystemPromptForUniversalWireVTextRunsRequiresArticleHead'
  -count=1` passed.
- `nix develop -c scripts/go-test-runtime-shards` passed all runtime shards.
- `npm --prefix frontend run build` passed; Vite reported pre-existing
  Universal Wire warnings for the unused `currentUser` export and unused
  `.wire-state` selectors.
- `PLAYWRIGHT_BASE_URL=http://127.0.0.1:5173 npm --prefix frontend run e2e --
  --project=chromium tests/universal-wire-app.spec.js -g 'deletes detritus
  source chronology and bespoke style controls'` passed against local Vite,
  asserting both retired `Style.vtext` and current internal `Style.texture`
  labels stay out of the visible Universal Wire UI.
- Scoped residue search for `Style.vtext` / `style.vtext` in the touched
  runtime and Universal Wire test surfaces found only legacy cleanup code and
  its negative fixture/assertion.

Deployed evidence on 2026-06-16:

- Behavior commit `9b77112902eaa3f7ab308e7ff976c5f3fcb5f13a` and follow-up
  test/evidence commit `d05cbc5556227ec9c3b5826a101128725532e882` were pushed
  to `origin/main`.
- Push CI run `27597833570` for `d05cbc5556227ec9c3b5826a101128725532e882`
  passed. The preceding behavior push CI run `27597769875` was cancelled by
  the follow-up push before deploy, so a manual deploy run was required.
- Manual CI run `27597934917` was dispatched with
  `force_staging_deploy=true`; CI and deploy job `81592293236` passed.
- Staging health at `https://choir.news/health` reported proxy and sandbox
  commit `d05cbc5556227ec9c3b5826a101128725532e882`, deployed at
  `2026-06-16T06:12:17Z`.
- Deployed Universal Wire UI proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  --project=chromium tests/universal-wire-app.spec.js -g 'deletes detritus
  source chronology and bespoke style controls'`. The proof guards that
  retired `Style.vtext` and internal `Style.texture` labels are absent from the
  visible Universal Wire app.

Rollback path: revert the behavior commit to restore previous `Style.vtext`
prompt labels, style source paths, and expectations.

Heresy delta: repaired for current Universal Wire style-source prompt/default
surfaces. No canonical `.vtext` file import/open behavior, storage schema,
workspace path, metadata compatibility key, durable `vtext:` actor id, stored
document title, or protocol v0 repair is claimed.

## 2026-06-16 - C31 Problem Checkpoint: Publication Helper Symbols

Problem: publication/export behavior is already routed through Texture paths,
but current helper and API symbols still teach the old ontology at the exact
publication boundary. Read-only inventory found `PublishVTextRequest`,
`PublishVTextResponse`, `Service.PublishVText`,
`HandleInternalPublishVText`, frontend `publishVText`, proxy
`publishVTextRequest`, and sandbox helper structs named
`sandboxVTextDocument` / `sandboxVTextRevision` on the active Texture
publication path. This is distinct from stored table names, durable
`vtext:<doc_id>` actor ids, `.vtext` file shortcuts, and stored legacy
`/pub/vtext/...` public route rows.

Conjecture delta: C31 tests whether the publication/export residue can be
repaired as code-symbol vocabulary while keeping the deployed product contract
unchanged. If successful, new/current publication code will be named Texture
without changing JSON payload fields, HTTP routes, database schema, public
route compatibility, or publication bytes.

Protected surfaces: platform publication creation and export, proxy
publication POST, Wire autonomous publication, runtime publication refs,
frontend publish action, and publication tests. Mutation class is `orange`
because runtime code and product publication paths are touched, with red
storage/actor/public-route surfaces explicitly excluded.

Admissible evidence class:

- focused Go tests for `internal/platform`, `internal/proxy`,
  `internal/runtime`, and `internal/wirepublish` publication surfaces;
- frontend build or focused frontend checks for the Texture editor publish
  callsite;
- scoped retired-name search showing the targeted helper/API symbols are gone
  or reduced to explicit compatibility residue;
- after behavior lands, normal CI/deploy identity and staging publication proof
  if the pushed diff changes platform behavior.

Rollback path: revert the C31 behavior commit to restore previous helper and
API names while leaving already-deployed Texture routes and publication data
unchanged.

Heresy delta: discovered publication-helper vocabulary residue; repair target
is the current code-symbol boundary only. C31 does not claim storage migration,
durable actor-id migration, `.vtext` suffix migration, stored public-route-row
migration, or protocol v0.

## 2026-06-16 - C31 Local Evidence: Publication Helper Symbols

C31 local repair changed current publication/export code vocabulary while
preserving deployed contracts:

- `internal/platform` now exposes `PublishTextureRequest`,
  `PublishTextureResponse`, `Service.PublishTexture`, and
  `HandleInternalPublishTexture` on the existing
  `/internal/platform/publications/texture` route.
- `internal/wirepublish`, `internal/runtime`, and `internal/proxy` now use
  `PublishTexture*` response/request types for Wire publication flow.
- `internal/proxy` now uses `HandleTexturePublication`,
  `publishTextureRequest`, and sandbox Texture helper structs while keeping
  JSON fields and `/api/platform/texture/publications` behavior unchanged.
- `frontend/src/lib/vtext.js` now exports `publishTexture`, and
  `VTextEditor.svelte` calls that Texture-named helper.

Local evidence on 2026-06-16:

- Problem checkpoint commit
  `268db43c234f57fdea6e65870b11568805706e7c` landed first; Docs Truth Check
  run `27598505265` passed.
- `nix develop -c go test ./internal/platform ./internal/proxy
  ./internal/wirepublish ./internal/runtime -run
  'TestInternalPublishRequiresInternalCallerAndBundleResolve|TestRegisteredTextureRoutesExcludeLegacyVTextPlatformPrefix|TestPublishTextureCreatesImmutablePublicRecords|TestPublicationFallbackDefaultsUseTextureLabels|TestPublicationPersistedDefaultTitlesUseTextureLabels|TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes|TestHandleTexturePublication|TestHandleInternalWirePlatformPublishPostsToPlatformd|TestWirePlatform|TestWirePublication|TestPostPlatformPublication|TestBuildAutonomousPublishRequest'
  -count=1` passed.
- `npm --prefix frontend run build` passed with only the pre-existing
  Universal Wire warnings for the unused `currentUser` export and unused
  `.wire-state` selectors.
- Scoped C31 residue search found no targeted helper/API hits for
  `PublishVText`, `publishVText`, `publishVTextRequest`,
  `HandleInternalPublishVText`, `HandleVTextPublication`,
  `HandlePublicVText`, `sandboxVTextDocument`, `sandboxVTextRevision`,
  `failed to publish vtext`, or `publish vtext` in the touched publication
  surfaces.

Heresy delta: repaired locally for publication/export helper/API symbol
vocabulary. Storage tables, `.vtext` suffixes, durable `vtext:` actor ids,
stored `/pub/vtext/...` route rows, and protocol v0 remain explicit residue.

Deployed evidence on 2026-06-16:

- Behavior commit `90746bccead98b839c1c8cc3fa5c537a80ce66fe` was pushed to
  `origin/main`.
- CI run `27598740366` passed, including Docs Truth Check, frontend build, Go
  vet/build, non-runtime tests, runtime shards, integration smoke, TLA+, and
  deploy job `81594789846`.
- Staging health reported proxy and sandbox commit
  `90746bccead98b839c1c8cc3fa5c537a80ce66fe`, deployed at
  `2026-06-16T06:31:08Z`.
- Deployed publication proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  --project=chromium tests/vtext-source-service-publication.spec.js -g
  'publishes source-service source entities as expandable transclusions and
  canonical exports'`.

## 2026-06-16 - C32 Problem Checkpoint: Texture File Suffix Defaults

Problem: current file manifestation and shortcut behavior still creates and
prioritizes `.vtext` as the canonical document-file suffix. New Texture
manifests are allocated as `*.vtext`, document titles derived from file imports
end in `.vtext`, alias selection prefers `.vtext`, File Browser treats only
`.vtext` as the special shortcut extension, and Universal Wire story-open
source paths still use `.story.vtext`. This leaves the product-visible Files
surface teaching the retired ontology even after the app, routes, tools,
publication helpers, and source surfaces have moved to Texture.

Observer probe: the deployed Universal Wire staging acceptance passed again on
2026-06-16, but direct product API inspection of
`/api/universal-wire/stories` returned `source:
universal-wire-edition-texture`, an edition at `universal-wire/Wire.vtext`,
and `story_count: 0`. Therefore the Universal Wire story-field proof remains
open: there was no deployed story payload to prove `story_texture_doc_id`
against.

Conjecture delta: C32 tests whether new/current file manifestations can move
to `.texture` defaults while legacy `.vtext` shortcuts and aliases remain
readable. If supported, current writes and UI affordances will teach Texture at
the file boundary without performing a database/table rename or deleting
historical `.vtext` rows.

Protected surfaces: Texture file open/import, document manifest creation,
canonical source-path metadata, file-browser shortcut recognition, Universal
Wire story source open paths, filesystem writes under the sandbox files root,
and document alias selection. Mutation class is `red` because persistent file
manifestation and alias behavior are touched. Explicit exclusions: Dolt table
names (`vtext_documents`, `vtext_revisions`, `vtext_document_aliases`),
`database=vtext`, durable `vtext:<doc_id>` actor ids, stored
`/pub/vtext/...` route rows, historical `.vtext` compatibility reads, and
protocol v0.

Admissible evidence class:

- focused Go tests around file open, manifest creation, alias selection,
  canonical source-path carry-forward, and Universal Wire story payload shape;
- focused frontend tests for markdown/file lineage and File Browser shortcut
  affordances;
- local frontend build for File Browser and Universal Wire callsites;
- scoped retired-name search proving new/current file-manifest defaults no
  longer introduce `.vtext` except explicit legacy compatibility and historical
  tests;
- if behavior lands, CI/deploy identity and deployed product proof against
  staging for a new Texture file manifestation.

Rollback path: revert the C32 behavior commit to restore `.vtext` as the new
manifest suffix and shortcut recognizer while leaving legacy `.vtext` aliases
and existing files untouched.

Heresy delta: discovered file-manifest default residue; repair target is new
Texture file manifestations and user-facing shortcut recognition. C32 does not
claim storage table, database, durable actor-id, stored public-route-row, or
Universal Wire story-field repair.

## 2026-06-16 - C34 Problem Checkpoint: Storage And Durable Identity Residue

Problem: after C32/C33, the remaining Texture hard-cutover residue is no longer
primarily user-facing file manifestation. It is persistent identity. The Dolt
document store still uses `vtext_*` table/index names, a `.vtext` workspace
directory suffix, and `database=vtext`; durable agent/channel identity still
uses `AgentProfileVText`, `role=vtext`, `vtext_agent_revision`, and
`vtext:<doc_id>` addressing; public publication storage still keeps legacy
`/pub/vtext/...` route rows readable; and Universal Wire still has edition and
wire-reference residue such as `universal-wire/Wire.vtext` and
`vtext_edition:<doc>/<rev>`.

Observer probe:

- `internal/store/vtext.go` lines 41-122 define `vtext_documents`,
  `vtext_revisions`, `vtext_document_aliases`, `vtext_agent_mutations`,
  `vtext_controller_checkpoints`, and `vtext_decisions`.
- `internal/store/vtext.go` lines 193-239 derive `.vtext` workspace paths,
  create `database=vtext`, and open Dolt with `database=vtext`; maintenance
  also opens `database=vtext` in `internal/store/dolt_maintenance.go`.
- `internal/runtime/tool_profiles.go` lines 15-25 define
  `AgentProfileVText = "vtext"` and lines 288-300 canonicalize `vtext` /
  `vtext-agent` / `document-agent` to that profile. Runtime code, tests, and
  run metadata still address durable document actors as `vtext:<doc_id>`.
- `internal/platform/service.go` lines 21-24 define current
  `/pub/texture/` minting plus `legacyPublicVTextPrefix = "/pub/vtext/"`;
  lines 204 and 259-261 mint only `/pub/texture/...` for new publications.
- `internal/platform/service_publication_read.go` lines 13-41 resolves whatever
  active row is present in `public_routes`, and
  `internal/platform/service_test.go` lines 1298-1309 explicitly inserts an
  active `/pub/vtext/...` row and proves it remains readable.
- Staging probe
  `curl -fsS 'https://choir.news/api/platform/publications/resolve?route=%2Fpub%2Fvtext%2Fprivate'`
  returned HTTP 404, so this pass discovered code/test support for stored
  legacy routes but did not prove a live staging row.

Conjecture delta: C34 asks whether the next repair should be a storage/identity
migration rather than another surface rename. The safe answer is not yet a
blind rename: storage tables, Dolt database names, run actor ids, update_coagent
targets, and public route rows are durable ledger keys. A correct repair must
introduce typed aliases, migration/backfill, verifier queries, and rollback
refs before deleting legacy reads.

Protected surfaces: Dolt workspace path and database name, document table/index
names, appagent mutation/checkpoint/decision tables, run `agent_id` /
`agent_profile` / `agent_role` metadata, update_coagent addressing, conductor
spawn contracts, workflow verifier expectations, public_routes rows, publication
resolution/export, Universal Wire edition refs, and run-acceptance evidence
labels. Mutation class for this checkpoint is `green/yellow` documentation and
inventory only; any behavior commit that changes these surfaces is `red`.

Admissible evidence class for a future behavior slice:

- Problem Documentation First checkpoint (this section) before any migration.
- Local migration tests proving old `vtext_*` storage remains readable and new
  writes use `texture_*` or a declared compatibility view/alias.
- Store-level round trip: create a document, revision, alias, agent mutation,
  checkpoint, decision, and source/publication metadata before migration; open
  all of them after migration.
- Runtime actor round trip: old `vtext:<doc_id>` addressed worker updates still
  wake the same Texture document, while new/current calls emit the chosen
  Texture identity.
- Public route proof: existing `/pub/vtext/...` rows either redirect/resolve
  through an explicit compatibility contract or are migrated to `/pub/texture`
  with rollback refs; new publications keep minting only `/pub/texture/...`.
- CI, staging identity, and deployed product proof after any behavior change.

Rollback path: for documentation-only C34, revert this checkpoint. For a future
storage/identity migration, rollback must be typed before implementation:
snapshot or Dolt commit before migration, reversible table/view/alias changes,
public route rollback refs, and a verifier that old actor/update/public-route
lookups still resolve after rollback.

Heresy delta: discovered persistent identity residue as the next hard cutover
edge. No runtime repair is claimed. The repair target is a typed migration plan
that preserves existing computers and public routes while making Texture the
current write identity.

## Local Repair: C34a Texture Workspace Identity

Mutation class: `red`, because this changes persistent embedded-Dolt workspace
path selection for runtime/document storage. It deliberately avoids table,
database, actor-id, or public-route migration in this slice.

Conjecture delta: new/current store workspaces can use the Texture filesystem
identity (`.texture` and `go-choir-texture`) while existing `.vtext` /
`go-choir-vtext` workspaces remain readable and writable without migration.

Protected surfaces: embedded Dolt workspace path derivation, store open,
document-only workspace open, runtime test-store template cloning, Dolt GC
workspace discovery, repeated test cleanup, and existing computer data under
legacy workspace directories.

Local behavior:

- `deriveVTextWorkspacePath` now resolves to the current Texture workspace
  suffix for new/current stores, while `deriveLegacyVTextWorkspacePath` records
  the deletion-receipted legacy path.
- `openVTextWorkspaceDB` chooses `.texture` when no workspace exists or when a
  current workspace already exists; if only the legacy `.vtext` workspace is
  present, it opens that legacy workspace instead of creating a parallel empty
  `.texture` workspace.
- Dolt GC uses the same resolver so maintenance continues to see legacy
  workspaces during the compatibility interval.
- The runtime store test template mirror now clones `.texture` workspaces.

Local evidence on 2026-06-16:

- `nix develop -c go test ./internal/store -run 'TestOpen(UsesTextureWorkspacePathForNewStores|FallsBackToLegacyVTextWorkspace|CreatesDatabase)|TestVTextInitWorkspace' -count=1`
  passed.
- `nix develop -c go test ./internal/runtime -run 'Test.*Store|TestDesktopState' -count=1`
  passed.
- `nix develop -c go test ./internal/store -count=1` passed.
- `nix develop -c scripts/go-test-runtime-shards` passed.
- `scripts/doccheck --report /tmp/choir-doccheck-c34a-workspace.md --json
  /tmp/choir-doccheck-c34a-workspace.json` passed report-only with 212 docs and
  1117 warnings.

Rollback path: revert this behavior commit. No data migration is performed, so
existing `.vtext` workspaces remain intact; new `.texture` workspaces created
during the interval can be preserved as rollback inputs or explicitly copied
before reverting if a local computer has advanced there.

Heresy delta: repaired for filesystem workspace identity only. Still
unrepaired: `database=vtext`, `vtext_*` tables/indexes, durable `vtext:<doc_id>`
actor ids, `AgentProfileVText`, `vtext_agent_revision`, stored legacy
`/pub/vtext/...` rows, `universal-wire/Wire.vtext`, and protocol v0.

## Deployed Repair: C34a Texture Workspace Identity

Mutation class: `red`, deployed behavior evidence for the embedded-Dolt
workspace identity repair.

Conjecture delta: deployed Choir can continue opening its existing persistent
workspace while new/current stores now use Texture workspace identity in the
source-controlled runtime path.

Deployed evidence on 2026-06-16:

- Commit `8e68553e23330e110eacf7f298f7471e101c7c15` passed CI run
  `27602041868`.
- Docs Truth Check run `27602041894` and FlakeHub publish run `27602041885`
  also passed for the same commit.
- Deploy job `81605380928` succeeded.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `8e68553e23330e110eacf7f298f7471e101c7c15`, deployed at
  `2026-06-16T07:41:44Z`.
- Deployed Playwright product proof
  `CHOIR_AUTH_STATE=/tmp/choir-c34a-workspace-auth.json PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/vtext-markdown-lineage.spec.js -g 'Imported Markdown advances|Imported plain text advances'`
  passed with 2 tests. This re-proved Markdown and plain-text import through
  canonical `.texture` source metadata, Markdown export, and recent Texture
  open on the deployed commit.

Rollback path remains: revert the behavior commit. Existing legacy `.vtext`
workspaces remain intact because this slice performs no migration.

Heresy delta: deployed repair for filesystem workspace identity only. No
database/table, actor-id, stored route-row, Universal Wire edition, or protocol
repair claimed.

## 2026-06-16 - C35 Problem Checkpoint: Durable Actor/Profile Identity Residue

Problem: after C34a, new/current store workspaces can carry Texture filesystem
identity, but the runtime actor/profile layer still teaches the old V-name as
the durable document owner. This is not only prompt copy. It is run metadata,
agent records, tool schemas, Trace acceptance, workflow verifier predicates,
model policy role keys, and coagent addressing.

Mutation class for this checkpoint: `green` documentation and evidence only. No
runtime behavior, schema, prompt default, tool schema, API response, route,
frontend test, or persistent state changed in this checkpoint.

Read-only evidence on 2026-06-16:

- The former invariant path named by the operating contract,
  `docs/vtext-agentic-invariants-2026-06-13.md`, now resolves to
  `docs/texture-agentic-invariants-2026-06-13.md`; that current document says
  Texture owns canonical document/artifact state and must not be turned into a
  workflow engine or role-sequence executor.
- `rg -n "AgentProfileVText|role=vtext|profile=vtext|requested_app\".*vtext|requested_app.*AgentProfileVText|vtext_agent_revision|vtext:<|agent_id\":\"vtext|agent_id.*vtext:" internal/runtime internal/store internal/types frontend/tests internal/runtime/prompt_defaults -g '!frontend/dist/**' | wc -l`
  found 431 current actor/profile residue hits.
- The same search touched 54 current files, including runtime tool/profile
  code, model policy, prompt defaults, workflow verifier, agent revision
  submission, coagent routing, persistence tests, API tests, and deployed
  frontend Trace assertions.
- `internal/runtime/tool_profiles.go` defines `AgentProfileVText = "vtext"`,
  canonicalizes `vtext`, `vtext-agent`, and `document-agent` to that profile,
  gives conductor/processor/reconciler delegate targets of `vtext`, and tells
  conductor to prefer `spawn_agent` with `role=vtext`.
- `internal/runtime/vtext_agent_revision.go` still writes
  `type="vtext_agent_revision"`, `agent_profile="vtext"`,
  `agent_role="vtext"`, and `agent_id="vtext:<doc_id>"` for document revision
  runs.
- `internal/runtime/tools_coagent.go` still exposes `role=vtext` tool
  descriptions, persists `AgentRecord{Profile:"vtext", Role:"vtext"}`, and
  returns `agent_id:"vtext:<doc_id>"` from handoff paths.
- `internal/runtime/vtext_workflow_verifier.go` verifies prompt-bar/conductor
  routes and worker deliveries by matching `AgentProfileVText`,
  `vtext_agent_revision`, and `vtext:<doc_id>`.
- Frontend deployed acceptance tests still assert Trace agents with
  `profile === 'vtext'` and `agent_id === vtext:<doc_id>`.

Conjecture delta: C35 asks whether current actor/profile writes can move to
Texture identity while old runs, pending worker deliveries, stored agent rows,
model-policy role keys, and Trace acceptance over legacy evidence remain
readable. The repair must be an alias/backfill boundary, not a blind rename.

Protected surfaces: run `agent_profile` / `agent_role` / `agent_id` metadata,
`agents` table rows, channel/update target IDs, tool-profile registries, model
policy role selection, prompt defaults, workflow verifier contracts,
prompt-bar acceptance, Trace agent projections, pending coagent deliveries, and
run-memory/persistence that infers Texture authority from legacy
`vtext_agent_revision` records.

First behavior slice design:

- introduce current `texture` actor/profile identity and legacy `vtext`
  compatibility helpers in one place;
- accept legacy `role=vtext`, `profile=vtext`, `agent_profile=vtext`, and
  `agent_id=vtext:<doc_id>` at read/match boundaries;
- make new/current spawned Texture document runs and tool affordances emit
  `texture` profile/role and `texture:<doc_id>` agent ids where the compatibility
  layer proves legacy lookups still resolve;
- keep `vtext_agent_revision` task type and model-policy TOML key out of the
  first slice unless tests prove they must move together, because task type and
  policy keys are separate durable compatibility surfaces;
- update prompt defaults and acceptance tests only after runtime can read both
  old and new identities;
- avoid semantic workflow forcing: do not add any rule that Texture must call a
  particular downstream role next.

Admissible evidence class for a future behavior slice:

- focused unit tests proving `texture` and legacy `vtext` profile/role inputs
  resolve to the same Texture affordances;
- old-read/new-write tests for run records and coagent handoff records:
  existing `vtext:<doc_id>` deliveries still reach the Texture document while
  new handoffs emit `texture:<doc_id>`;
- workflow verifier tests that accept legacy evidence and require new current
  prompt-bar runs to show Texture identity;
- model policy tests proving legacy `[roles.vtext]` continues to load until a
  policy migration is explicitly designed;
- prompt-bar/local runtime tests proving conductor -> Texture first revision
  still has no super-before-Texture route and does not force researcher/super;
- CI, staging identity, and deployed acceptance proof after any behavior change.

Rollback path: revert the behavior commit for the first C35 slice. The slice
must not rewrite existing run or agent rows without a separate migration and
rollback ref. If any compatibility alias is removed later, that deletion must
have a retired-name search receipt plus a verifier proving no pending legacy
delivery or stored Trace evidence depends on it.

Heresy delta: discovered durable actor/profile identity residue as the next
runtime cutover edge. No runtime repair is claimed. Repair target is current
Texture actor/profile write identity with explicit legacy-read compatibility
and no new semantic decision tree.

## Non-Goals

- Do not write a full protocol cold.
- Do not preserve compatibility aliases as indefinite dual paths.
- Do not implement semantic phrase matching in runtime to make the cutover pass.
- Do not weaken docs-only CI filters.
- Do not resume M3 or source/news work until the core prompt-bar artifact loop
  has product-path proof under the Texture ontology.

## Parallax State

status: open_handoff

mission conjecture: if Choir hard-cuts the artifact control-plane ontology to
Texture across docs, code, prompts, UI, tests, tool names, acceptance, and
checker warnings while preserving the deployed prompt-bar -> conductor ->
Texture revision loop, then M3 can resume with less route confusion and fewer
hidden workflow gates.

deeper goal (G): make Texture the stable semantic substrate for directing
autonomous results and compounding learnings across source/news articles,
style, research, super evidence, and future media projections.

witness/spec (A/S): retire the old V-name except historical/background
evidence; preserve one Texture writer and human canonical edits; keep super
downstream of Texture for privileged execution; avoid runtime semantic decision
trees; keep transclusions pinned by default with newer-version indicators; leave
`docs/texture-protocol-v0.md` until the working surface is proven.

invariants / qualities / domain ramp (I/Q/D): Texture owns canonical artifact
meaning and learning; other agents produce evidence/proposals/receipts/faults;
every version is immutable, addressable, comparable, restorable, and forkable;
compatibility shims need deletion receipts; proof moves from docs/checker ->
focused local tests -> CI/deploy identity -> staging browser/product proof ->
protocol v0.

variant (ranking function) V: current V=2; last ΔV: C35 documented durable
actor/profile identity residue as the next typed problem; no repair decrease
yet. Database/table names, durable actor ids, stored legacy routes, Universal
Wire edition refs, deployed Universal Wire story-field proof, and protocol v0
remain.
Discharged:
retired-name inventory,
report-only H5 docs checker, high-read docs reconciliation, browser-public
`/api/texture` route and old `/api/vtext` refusal, registered-router
normalization, platform publication control routes, app identity, visible UI
labels/import affordances, `patch_texture`/`rewrite_texture` affordances,
`edit_texture` alias deletion, prompt-bar -> conductor -> Texture first
revision proof, pinned transclusions/newer-version proof, source metadata,
package/provenance, Universal Wire local story projection plus deployed empty
state/source-label proof, related Texture refs, source-contract open surface,
canonical source-path metadata, public route minting, publication fallback
labels, and C26 deployed evidence. Remaining coarse obligations: storage
symbols plus durable actor/stored-route residue, deployed Universal Wire
story-field proof, and protocol v0 after proof.

budget: one broad red-surface cutover mission before M3 resumes; split only if
a distinct product regression appears after documenting it here.

authority / bounds: target mutation class remains `red`; each slice names its
actual class. Protected surfaces include canonical artifact writes, prompt-bar
routing, conductor materialization, Texture prompts/tools, Trace/acceptance
projection, UI labels, docs checker, deployment routing, publication exports,
and database/storage migrations. Apply Problem Documentation First before
behavior changes.

evidence packet: mission checkpoints and ledger receipts; docs checker report;
focused tests for each touched surface; local runtime shards when runtime
changes land; pushed commits; CI run ids; Node B deploy identity; staging
browser/product proof; retired-name searches; final protocol v0 distilled from
proof.

heresy delta: discovered: the old ontology is a system-wide drift source.
Introduced: none accepted. Repaired target: delete dual-path naming,
direct-super ingress ambiguity, workflow-forcing prompts, and overloaded edit
affordances where this mission proves the repair.

position / live conjectures / open edges: C1 vocabulary shift remains active;
C2-C3 and C6-C27 are supported at the scopes recorded in the ledger, with C22's
deployed Universal Wire story-field proof still open until staging has a story
payload or product path creates one without manual success seeding. C4 remains
active for old mission docs that may be clearer to leave historical. C5 remains
active: protocol v0 is last. C27 is supported for deployed HTML export scope:
generated platform HTML exports now emit `texture-publication`,
`texture-table`, `texture-source-ref`, and `texture-sources*` classes/ids; local
tests assert old-class absence, and staging product proof exported an HTML
publication with Texture classes and no retired export classes. C28 is deployed
supported for live editor source-ref/source-flow class names: the renderer,
serializer, editor CSS, source-flow CSS/DOM builder, and focused frontend tests
now use `texture-source-ref*`, `texture-source-journal-*`,
`texture-source-flow-close`, `texture-source-open`, and
`--texture-source-flow-*`; CI/deploy passed; staging health reports the pushed
SHA; and deployed browser proof created a Texture document, opened it in the
Texture app, clicked a source ref, and observed Texture live/source-flow classes
with no scoped retired classes. This slice excludes frontend file/module names,
storage tables, `.vtext` file suffixes, durable `vtext:` actor ids,
`PublishVText` Go symbols. C29 is deployed supported for public legacy
publication routes: frontend first-load public route recognition, desktop public
route normalization, and current source-reader fixtures now use only
`/pub/texture/...`; scoped frontend search is clean; CI/deploy passed; staging
health reports the pushed SHA; and deployed product proof created and published
a Texture through `/pub/texture/...` while same-slug `/pub/vtext/...` was not
treated as a public reader. The backend `/pub/vtext/...` stored-route row shim
remains explicitly tagged as compatibility residue until storage migration. C30
is deployed-supported for Universal Wire style-source suffixes: current
prompts/defaults now introduce `Style.texture` labels and `.style.texture`
source paths; focused runtime tests, runtime shards, frontend build, focused
local Playwright, and scoped residue search passed. Legacy `Style.vtext Source`
cleanup recognition remains only as generated-content cleanup. CI/deploy passed
after a manual forced staging deploy, staging reports the pushed head SHA, and
deployed Universal Wire UI proof shows both `Style.vtext` and `Style.texture`
style labels absent from the visible app. Canonical `.vtext` import/storage
behavior and Universal Wire deployed story-field proof stay out of scope. C31
is deployed-supported: publication/export helper and API symbols now use
Texture names while preserving JSON fields, current Texture routes, stored
public route compatibility, storage tables, and durable actor ids. CI/deploy
passed, staging reports the pushed SHA, and deployed publication proof passed.
C32 is deployed-supported: new/current import titles, manifest allocation,
manifest shortcut kind, alias priority, File Browser shortcut recognition,
VText editor shortcut recognition, desktop-shell manifest expectations, and
Universal Wire story-open source paths now default to `.texture`; legacy
`.vtext` shortcuts remain readable. Focused runtime/store tests, full runtime
shards, full store package, frontend build, CI run `27600056369`, deploy job
`81598902993`, and staging health for commit
`ae2ada4a4b51f9c2671113e9c07dc7c3e5417050` passed. Deployed proof against
`https://choir.news` created a Markdown-backed Texture, observed `.texture`
title/source metadata/manifest, verified Markdown export remains `.md`, and
opened the recent Texture through Desk -> Texture at `v1`. The reusable
Playwright selector drift discovered during this proof is now repaired by C33.
Storage workspace paths/tables, durable `vtext:` actor ids, stored
`/pub/vtext/...` rows, Universal Wire edition `Wire.vtext`, Universal Wire
deployed story-field proof, and protocol v0 remain outside C32.
C33 is supported for the reusable staging acceptance harness repair:
`frontend/tests/vtext-markdown-lineage.spec.js` now launches Texture through
floating icon, rail, or Desk surfaces and recognizes canonical `texture` plus
legacy `vtext` window ids. The previously failing deployed command
`PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- --project=chromium tests/vtext-markdown-lineage.spec.js -g 'Imported Markdown advances|Imported plain text advances'`
passed with a fresh auth state, proving the reusable acceptance path no longer
depends on retired desktop/window selectors. This is a yellow proof-surface
repair, not product runtime behavior. Commit
`376ac6d9c5439fd7c08c52fa628dc5f341820b97` landed the harness repair; CI run
`27601085720`, Docs Truth Check `27601085740`, and FlakeHub publish
`27601085759` passed. Deploy to staging was skipped because no deployed
artifact changed.
C34a is deployed-supported for Texture filesystem workspace identity:
new/current stores now derive `.texture` / `go-choir-texture`, existing
`.vtext` / `go-choir-vtext` workspaces are reopened when no current workspace
exists, Dolt GC uses the same resolver, and the runtime store test harness
clones `.texture` workspaces. Focused store tests, focused runtime store tests,
the full store package, runtime shards, CI run `27602041868`, deploy job
`81605380928`, and staging health for commit
`8e68553e23330e110eacf7f298f7471e101c7c15` passed. Deployed Playwright proof
re-ran the Markdown/plain-text Texture import, `.texture` metadata/export, and
recent Texture open acceptance against `https://choir.news`. This does not
claim `database=vtext`, `vtext_*` table/index, durable `vtext:<doc_id>` actor,
`AgentProfileVText`, `vtext_agent_revision`, stored `/pub/vtext/...` route row,
or `universal-wire/Wire.vtext` repair.
C35 is problem-documented only: durable actor/profile identity remains old-name
state across `AgentProfileVText`, `role=vtext`, prompt/tool affordances,
`agent_profile` / `agent_role`, `agent_id=vtext:<doc_id>`, workflow verifier
checks, model-policy role keys, and Trace/front-end acceptance assertions. The
current invariant doc is `docs/texture-agentic-invariants-2026-06-13.md`, which
forbids turning Texture identity repair into a forced workflow or role sequence.
No C35 runtime repair is claimed yet.

next move: implement the first C35 behavior slice only after preserving old
lookups: introduce centralized Texture actor/profile compatibility helpers,
accept legacy `vtext` reads, and make a focused new-write path emit current
`texture` profile/role/agent identity with old-read/new-write tests. Keep
`vtext_agent_revision` task type and model-policy key migration separate unless
the tests prove they must move together.

ledger file: `docs/mission-texture-hard-cutover-v0.ledger.md`

version / lineage: spawned from M3.4 readiness review and the 2026-06-15
Texture rename discussion. Blocks M3 until settled or explicitly narrowed.

learning state: Texture exists to direct results with autonomy and facilitate
learnings; the rename must preserve that reason, not collapse into branding.

settlement: settled only when non-allowed retired-name occurrences are gone or
explicitly scoped as remaining residue, Texture docs/doctrine agree, checker
coverage and report receipts exist, deployed core Texture loop and transclusion
proofs are recorded, and minimal Texture Protocol v0 is canonized from the
working surface.

## Suggested Goal String

```text
Use Parallax on docs/mission-texture-hard-cutover-v0.md. Treat it as the source
program for the Texture hard cutover before M3 resumes. Texture is the promoted
ontology for Choir's versioned, transclusive artifact control plane; the old
V-name is migration residue allowed only in the historical background doc and
explicit historical mission evidence. Current status is open_handoff with V=2.
The inventory, report-only H5 docs checker, high-read docs reconciliation,
Texture route/tool/prompt slices, deployed prompt-bar -> conductor -> Texture
first-revision proof, deployed pinned-transclusion proof, visible UI proof,
source-contract open-surface proof, canonical source-path metadata repair,
publication fallback label repair, C27 deployed exported HTML class-name proof,
C28 deployed live editor source class proof, and C29 deployed public route proof
are landed. C30 is deployed-supported for Universal Wire style-source suffixes.
C31 is deployed-supported for publication/export helper and API symbols. C32 is
deployed-supported: new/current Texture file manifestation defaults moved from
`.vtext` to `.texture` while legacy `.vtext` shortcuts remain readable; CI,
deploy identity, and staging product proof are recorded. C33 repairs the
reusable staging acceptance harness so it follows current Desk/Texture identity
while preserving legacy selector compatibility. Next move is the remaining
storage/durable actor/stored-route residue, now documented by C34 as requiring
a typed migration/alias plan before behavior edits. Universal Wire story-field
staging proof and protocol v0 remain open. Keep storage schema, durable
`vtext:` actor ids, backend stored-route migration, Universal Wire story-field
proof, and protocol v0 out of C32.
Preserve one Texture writer among agents, keep human direct edits canonical,
keep super downstream of Texture for privileged execution, and avoid runtime
semantic decision trees. Append moves to
`docs/mission-texture-hard-cutover-v0.ledger.md`; settle only with CI, staging
identity, deployed acceptance, retired-name search receipts, checker report,
and a minimal protocol distilled from proof.
```
