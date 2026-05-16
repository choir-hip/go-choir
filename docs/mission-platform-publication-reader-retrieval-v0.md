# MissionGradient: Platform VText Reader Proposal Retrieval v0

Status: proposed
Date: 2026-05-16
Operator: Codex directly in the repo
Research input:
[publication-reader-retrieval-pretext-research-2026-05-16.md](publication-reader-retrieval-pretext-research-2026-05-16.md)

## Real Artifact

The artifact is a deployed staging Choir VText reader/proposal/retrieval path
for platform-published VText versions.

The V0 platform Dolt mission created real publication records and a thin
`platformd` HTML route. This mission replaces that proof renderer with the real
product boundary:

```text
/pub/vtext/... public Choir route
  -> VText app surface, guest/read-only when signed out
  -> proxy platform read API
  -> internal platformd JSON bundle API
  -> platform Dolt + platform artifact blobs
```

Signed-out visitors can read published pieces through the platform instance of
Choir in the VText app, not on a separate static page. Signed-out VText is
guest/read-only; attempted edits, forks, citations, transclusions, or proposals
are conversion moments that ask the reader to register or log in. Signed-in
users can read the same published pieces inside the VText app, embed source
spans inside their own host VText through a Pretext-powered transclusion
interface, edit their own private derivative, and submit that derivative as a
proposal back to the source publication author.

`platformd` remains the internal ledger and artifact API. It should not render
public HTML and should not be directly routed from the public internet.

The mission also adds the first owner-visible publish UX in VText: a selected
revision can be previewed, published, and opened in the VText reader without
implying that later private revisions are public.

Pretext is part of the target as the interface layer for embedded source text:
it should help a source VText span live inside a host VText as readable,
measurable, editable-around material. It is not the semantic source of
transclusion, citation, permission, proposal validity, or provenance.

## Invariants

- Staging is the acceptance environment: `https://draft.choir-ip.com`.
- Platform Dolt owns platform-visible publication, artifact, retrieval,
  citation, provenance, consent/review, verifier, and rollback facts.
- Per-user embedded Dolt remains the private mutable user-computer ledger.
- Publishing a VText revision copies a selected immutable public projection; it
  does not publish the mutable private document head.
- Later private revisions remain private unless explicitly published.
- `platformd` binds internally and exposes JSON/service semantics to trusted
  host callers. It does not serve browser-facing HTML.
- Public browser traffic reaches published pieces through Caddy, the Svelte app,
  and proxy read APIs. Browsers never talk to Dolt or internal `platformd`
  routes directly.
- Signed-out public reading still uses the VText app surface in guest/read-only
  mode. Do not build a separate static public article renderer.
- Edit/fork/cite/transclude/propose actions from guest VText are auth-boundary
  funnel moments: preserve the user's intent, ask for registration/login, then
  continue through an owned user computer.
- Public read APIs expose only platform-visible fields. They must not leak
  private computer ids, private file paths, unpublished revision content,
  prompts, worker traces, raw internal storage paths, or service-only rollback
  refs.
- Authenticated mutation remains behind auth and user-computer authority:
  publish, cite into a private VText, save a transclusion, retract, supersede,
  or create a new private document from a public piece.
- Editing a published VText creates a private derivative or proposal in the
  reader's computer. It must not mutate the source publication, source private
  VText, or author's canonical state.
- A proposal to an author is a typed platform relation over immutable refs. It
  becomes canonical author content only after owner or delegated author-side
  policy accepts it.
- Author-side delivery may wake or hydrate the author's computer through
  product computer infrastructure, but platform Dolt remains the durable ledger,
  not the live cross-computer message bus.
- Author-side VText agents may react to proposal/citation events within their
  authority by researching, preparing a candidate revision, or requesting worker
  help. Human/publication acceptance authority is not silently bypassed.
- Retrieval over published pieces returns exact source/version/span refs and
  snippets. Derived search/index state is rebuildable from platform Dolt
  publication/artifact/span records.
- Citation edges render from typed platform records, not from decorative links
  alone.
- Transclusion refs target immutable public publication versions/spans or
  explicitly authorized private refs. This mission should start with public
  published refs only.
- Do not implement CHIPS, paywalls, citation economy ranking, wallets, or broad
  governance. Typed citation/transclusion/proposal records are in scope as the
  substrate; pricing/ranking/economics are not.
- Node B tracked files are not edited directly as a deployment shortcut.
- Behavior-changing commits follow:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

## Value Criterion

Minimize divergence between a published VText's immutable platform facts and
the user's reading, editing, retrieval, citation, transclusion, and proposal
experience, while preserving private computer boundaries and keeping platform
service responsibilities narrow.

The loss function penalizes:

- public HTML generated by `platformd`;
- public routes that bypass Choir's app shell and proxy API boundary;
- reader UI that cannot be traced to exact publication version/content hashes;
- retrieval results that do not include exact span/source/version identity;
- citations that appear as text decoration but have no typed edge behind them;
- transclusions that copy source text without immutable source refs;
- proposals that mutate the source publication instead of creating a typed
  candidate relation;
- proposal delivery that depends on manual out-of-band notification;
- publish UI that hides which revision is becoming public;
- APIs that leak private computer state or internal service paths;
- Pretext integration that becomes a separate demo rather than a reader
  improvement;
- local-only proof for platform route, auth, publication, or retrieval behavior.

## Quality Gradient

Expected quality: `solid`.

A solid outcome:

- removes direct public `/pub/* -> platformd` HTML serving;
- adds internal platform JSON bundle/read endpoints with focused tests;
- adds proxy public read APIs with sanitization tests;
- adds a public VText guest/read-only route for `/pub/vtext/...`;
- opens the published VText inside the signed-in desktop VText app;
- supports a signed-in reader creating a private derivative/proposal from the
  published VText;
- records a proposal relation back to the source publication/author;
- attempts or explicitly records author-side proposal delivery state without
  mutating author canonical content;
- adds VText publish controls with preview and selected-revision clarity;
- adds stable rendered document/block/span data sufficient for Pretext-ready
  layout and retrieval;
- adds a minimal Pretext-powered or Pretext-ready transclusion node/display for
  embedding a published source span inside a host VText;
- renders citation/retrieval provenance from platform rows;
- proves signed-out and signed-in public read behavior on staging;
- proves private revisions after publication do not appear publicly;
- proves retrieval returns source/span refs for a published piece;
- documents residual risks and next realism axes.

Substandard work:

- making the existing `platformd` HTML template prettier;
- adding browser-public internal routes;
- publishing the current private document head by implicit side effect;
- inventing a second publication store outside platform Dolt;
- implementing search over private user computers;
- claiming transclusion because copied text appears in another document;
- treating a reader's edit as an accepted author revision;
- waking an author computer by an internal shortcut that bypasses product
  ownership/routing checks;
- adding CHIPS/ranking/paywall mechanics before source/span semantics are
  inspectable.

## Homotopy Parameters

Increase realism continuously along these axes:

- one published VText -> many published VTexts;
- full-document span -> block spans -> inline quote/range selectors;
- plain Markdown/block rendering -> Pretext-assisted inline/layout rendering;
- public signed-out VText guest reader -> edit intent/auth funnel -> signed-in
  VText reader -> private VText cite or transclude action -> proposal back to
  source author;
- simple deterministic retrieval -> indexed span retrieval -> vector/reranking
  as derived caches;
- accepted citation edge display -> proposal/citation lifecycle controls;
- proposal record only -> proposal delivery to active author computer -> wake
  hibernating author computer -> author-side VText agent response;
- one owner publish button -> preview, slug, supersession, retraction, reviewer
  evidence;
- platform API unit tests -> deployed public browser proof -> Node B platform
  Dolt inspection;
- source revision hash display -> full provenance reader affordance;
- public platform route -> custom handle/domain route later.

At low resolution, Pretext may be used for a single embedded source-span card or
measurement path. Do not block the mission on rich manual text layout if the
platform reader/proposal boundary is still wrong.

## Belief State

Current belief:

- platform Dolt/publication v0 is deployed and proved on staging;
- `platformd` currently has the right ledger responsibility but too much public
  rendering responsibility;
- the Svelte app already owns signed-in and signed-out desktop presentation and
  should own public publication reading too;
- Pretext is available as `@chenglou/pretext`, currently version `0.0.7` on npm,
  and is useful for layout but not sufficient for transclusion semantics;
- current VText rendering is local Markdown-ish Svelte code and can be factored
  into a shared reader model before deep Pretext work;
- retrieval rows exist but need public read/search APIs and more useful
  source/span materialization;
- owner publish UX is missing, so publication is currently API-only.
- VText currently behaves mostly as an authoring app; this mission should start
  turning it into the shared reading/editing/proposal app for published ideas.
- Cross-user proposal delivery and author-computer wake are conceptually aligned
  with the computer model, but the exact minimal product path needs inspection.

Evidence:

- [mission-platform-dolt-publication-retrieval-citation-v0.md](mission-platform-dolt-publication-retrieval-citation-v0.md)
- [publication-reader-retrieval-pretext-research-2026-05-16.md](publication-reader-retrieval-pretext-research-2026-05-16.md)
- [current-architecture.md](current-architecture.md)
- `cmd/platformd`, `internal/platform`, `internal/proxy/platform_publish.go`,
  `frontend/src/lib/VTextEditor.svelte`, `frontend/src/App.svelte`,
  `nix/node-b.nix`

Main uncertainties:

- the smallest clean route model for `/pub/vtext/...` inside the existing SPA;
- whether the first signed-out VText surface should appear as a focused VText
  route or a desktop window in the platform Choir instance;
- how much of the VText editor Markdown renderer should be shared with the
  reader before adding Pretext;
- whether retrieval v0 should scan artifact blobs on demand or add a derived
  public span text/index table;
- the minimal UI for citation/transclusion/proposal actions that does not imply
  broader governance or economics;
- the smallest safe author delivery path: durable platform proposal only,
  active-computer delivery, or hibernated-computer wake.

Highest-impact uncertainty:

Can the public reader move fully into the VText app/proxy boundary and support a
real edit/proposal loop while preserving the already-proven platform Dolt
publication records and without weakening the private/public trust split?

Next observation:

Inspect current Svelte routing/root behavior, Caddy `/pub/*` handling,
`platformd` bundle data available from existing rows, VText editor toolbar
structure, and vmctl/proxy capability for author-computer delivery. Choose the
smallest route/API seam that can be proven on staging.

## Receding-Horizon Control

Work in short Codex control intervals.

At each interval:

1. name the boundary being changed;
2. predict the observable evidence;
3. make the smallest coherent change;
4. run focused tests;
5. update belief state if observations surprise the mission;
6. continue, narrow, branch, rollback, or stop.

Initial mutation radius:

- add platform read/bundle APIs;
- add proxy public read/sanitization APIs;
- replace public Caddy `/pub/* -> platformd` with SPA route handling;
- add a frontend public VText guest-reader and signed-in desktop reader path;
- add VText publish UI using the existing publish API;
- add retrieval search/read over published spans only;
- add a minimal transclusion node/display for embedding a published source span
  in a host VText;
- add a proposal submission path from reader derivative to source publication;
- add durable proposal delivery state and attempt author-side delivery only
  through product ownership/routing boundaries;
- avoid private computer lifecycle, vmctl, auth ceremony, CHIPS, paywall, and
  ranking changes unless required by the reader/proposal boundary.

Widen scope only after `/pub/vtext/...` is rendered by VText/Choir rather than
`platformd` and the deployed product path proves the route plus one proposal
record.

## Dense Feedback Channels

Use feedback that reveals local error:

- platform store/service tests for publication bundle resolution and retrieval
  span search;
- proxy tests for public read sanitization and auth-required write/mutation;
- frontend unit/build checks for reader route and VText publish controls;
- Playwright proof for signed-out `/pub/vtext/...` VText guest reader;
- Playwright proof that signed-out edit intent opens auth and preserves the
  intended edit/fork action after registration/login;
- Playwright proof for signed-in desktop opening the published reader;
- Playwright proof for signed-in reader creating a private derivative from a
  published VText;
- API/Playwright proof for submitting that derivative as a proposal to the
  source publication author;
- author-side evidence: durable delivery record at minimum, and product-path
  active/wake delivery if safely reachable in the current system;
- Playwright/API proof that a later private revision remains absent from public
  reader and retrieval results;
- API proof that public retrieval returns source/version/span refs and snippets;
- browser request audit proving no `/internal/*`, `/api/test/*`, `/api/agent/*`,
  or raw `platformd` routes are used;
- Node B Caddy/service inspection proving no public reverse proxy route points
  `/pub/*` directly to `platformd`;
- platform Dolt inspection proving reader/retrieval data matches publication
  version/content hashes;
- staging health/build identity checks after deploy.

## Evidence Ledger

Every final claim must name evidence:

```text
claim
evidence source
command or observation
artifact path
result
uncertainty/caveat
promotion relevance
```

Required final evidence:

- pushed commit SHA;
- CI run and deploy status;
- staging health/build identity;
- deployed signed-out public VText guest-reader acceptance;
- deployed signed-out edit-intent auth funnel acceptance;
- deployed signed-in desktop reader acceptance;
- deployed publish-from-VText UI acceptance;
- deployed transclusion-in-host-VText acceptance;
- deployed reader-derivative proposal acceptance;
- author-side proposal delivery evidence or explicit invariant-level blocker;
- deployed retrieval-over-published-piece acceptance;
- browser request audit for forbidden paths;
- Node B route/service inspection for `platformd` public exposure;
- platform Dolt/publication row inspection for selected publication/version;
- rollback refs and residual risks.

## Forbidden Shortcuts

- Do not keep `platformd` as the public HTML renderer and call it done.
- Do not build a separate static public article renderer for signed-out users.
- Do not expose `platformd` internal routes to browsers.
- Do not make the public reader call `/internal/*`.
- Do not fetch private VText content for public reads.
- Do not retrieve over private user-computer state for public search.
- Do not mutate the source author's canonical VText or publication when a reader
  edits their own version.
- Do not call a proposal "delivered" unless product-path evidence shows durable
  delivery state and, where claimed, author-computer delivery.
- Do not use platform Dolt as a polling bus for live cross-computer agent work;
  use it as the durable ledger and recovery substrate.
- Do not manually seed platform Dolt rows for the acceptance proof.
- Do not use a test-only route to create publication, retrieval, or citation
  success.
- Do not hide missing typed citations behind rendered Markdown links.
- Do not treat Pretext integration as proof of transclusion.
- Do not add CHIPS, ranking, paywalls, or broad governance in this mission.
- Do not claim staging proof from local browser or local service checks.

## Rollback Policy

Git rollback:

- every behavior change lands in a commit on `main` only through the normal
  landing loop;
- if deployed reader routing fails, revert the commit or follow up with a route
  fix commit and redeploy.

Route rollback:

- preserve a simple path to restore `/pub/*` behavior if the SPA reader fails;
- do not delete the platform publication records during route rollback.

State rollback:

- publication rows are append-only or state-transitioned;
- proposal rows are append-only or state-transitioned;
- rollback/retraction should disable or supersede route pointers rather than
  mutate source private revisions;
- test publications/proposals created by acceptance should be clearly
  identifiable.

Service rollback:

- `platformd` should remain active for JSON/internal APIs even after HTML
  rendering is removed;
- if proxy public read APIs fail, Caddy should still serve the SPA and API errors
  should be visible without exposing internals.
- if proposal delivery fails, the durable proposal row and delivery error should
  remain inspectable and retryable without losing the submitter's private
  derivative.

Before stopping unsuccessfully, use a cognitive-transform pass to reframe the
blocker from at least three angles: service-boundary/security, product-reader
experience, and social/collaboration mechanism design. Before stopping
successfully, perform one quality pass over API shape, UI clarity, tests, docs,
and whether the proposal loop really feels like the beginning of VText-native
collaboration rather than a static comment form.

## Learning Side-Channel

Classify surprises:

- Tactical learning: apply directly and note in final evidence.
- Target-level learning: update this mission doc or create a follow-up mission
  if reader/retrieval needs a different parameterization.
- Invariant-level learning: stop and escalate before changing private/public
  boundaries, public API exposure, publication immutability, or auth/mutation
  semantics.

Project artifacts that should receive durable learnings:

- this mission document;
- [current-architecture.md](current-architecture.md) if the target boundary
  changes;
- platform/proxy/frontend tests that encode the boundary;
- final report evidence ledger.

## Stopping Condition

The mission completes only when staging proves:

- `/pub/vtext/...` is rendered by Choir's Svelte/product route, not by
  `platformd` HTML;
- signed-out visitors can read one published VText route;
- signed-out edit/fork/transclude/propose intent opens the auth flow instead of
  mutating public or private state anonymously;
- signed-in users can open the same publication inside the VText app;
- VText has an owner-visible publish flow for a selected revision;
- a signed-in reader can create a private derivative/host VText from the
  published source;
- at least one published source span can be embedded in the host VText as a
  typed transclusion ref with snapshot text and immutable source refs;
- the reader can submit their derivative as a proposal to the source
  publication/author;
- source publication and source private VText remain unchanged by the proposal;
- platform Dolt records proposal, transclusion/citation, provenance, and
  delivery state;
- author-side delivery is either proven through the product path or blocked by a
  named invariant-level gap with retryable durable state preserved;
- a later private revision after publish does not appear in the public reader or
  retrieval output;
- retrieval over at least one published piece returns exact source/version/span
  refs and snippets;
- at least one citation edge or citation candidate renders from platform rows;
- browser request audit shows no forbidden internal/test/product-bypass paths;
- Node B inspection confirms no public `/pub/*` reverse proxy to `platformd`;
- platform Dolt inspection confirms route/version/retrieval/citation facts;
- rollback target exists and residual risks are stated.

Do not claim completion from code existence, local tests, or a pretty page alone.
