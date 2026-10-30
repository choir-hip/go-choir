---
definition_version: 4

# W3a — the Autopaper reader on published Textures. Station of
# definitions/choir-autopaper-metamission-2026-10-11.md; design
# docs/autopaper-app-design-2026-10-11.md §4.6, §5 slice 1, §6 R2 and R3.
# Proposed ahead of W2 because it needs nothing from the held protocol
# station P2d2 (design §5, open question 3).
readiness: drafted

review:
  reviewer: "none yet (owner review first; owner rule 2026-10-11: panels for uncertainty, not ceremony)"
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-11T22:00:00Z
  source:
    canonical_ref: main f291d5ac
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      The Universal Wire app (UniversalWireApp.svelte) works only signed in,
      renders the retired processor's WireStory schema, and its server side
      stamps the same two fixed "claims" on every story
      (internal/platform/universal_wire.go, Claims field). On staging it
      returns an empty list (design §2.1).
    - >-
      Every publish mints a new publication, version and route
      /pub/texture/<slug>-<shortid> (internal/platform/service.go,
      PublishTexture); no call adds a version to an existing route. A living
      story would multiply on any front page built from publications.
    - >-
      GET /api/platform/publications/resolve?route= is public and returns a
      PublicationBundle with citations, source entities, transclusions and
      the full version history (internal/platform/types.go). A logged-out
      visit to /pub/texture/... opens a read-only Texture window.
    - >-
      The Texture renderers are pure functions that already draw numbered
      source refs with quote popovers and pinned links between Textures with
      a newer-version marker (texture-source-renderer.ts).
    - >-
      Today's AI news demo sources are bare URLs with no quoted text (10 of
      10 on test computer demo-p2c-2026-10-11), so quoting captures is a
      separate slice (design §5 slice 2, C).

finish:
  deliver: >-
    A logged-out reader at choir.news sees a front page of published story
    Textures, newest first, and opens one as a read-only story page with
    its numbered sources. Each published version of a story has its own
    immutable permalink (/v<n>; v7's link always shows v7), linked to its
    neighbours: chevrons move between versions, a "new version available"
    control jumps to the latest, and the bare story link shows the latest
    in place. A revised story stays one item on the front page with an
    "updated" marker. The old Universal Wire app, its route and its fixed
    claims are gone (owner 2026-10-12, docs/re-engineering-decisions-2026-10-12.md §8).
  artifact: >-
    Step 0, Prototype A (no deploy): a standalone page that takes a real
    PublicationBundle from staging's public resolve route and renders it
    with the existing renderers, judged at phone width (design §6 R3).
    Step 1, a version chain per document: publishing a document that
    already has a publication appends version n+1 with its own immutable
    route /v<n+1>, sets latest_version_id and supersedes_version_id (the
    columns exist; publishing hard-codes "v1" today), and stops copying
    every past revision into each publish. The bundle carries prev, next
    and latest for the chevrons and the "new version available" control
    (study docs/reports/re-engineering-studies-2026-10-12/E-unified-document-model.md §5). Step 2, the public front list: a GET route listing the
    paper's published routes with title, dek, updated time and version
    count, served from the shared store, never from a paper computer.
    Step 3, the autopaper app: the front page and the story page, on the
    Desktop and as a public page, reading only those GET routes. Step 4,
    delete UniversalWireApp, its route, the WireStory schema and the fixed
    claims, with their Playwright specs replaced by specs for the new
    pages.
  acceptance:
    - Prototype A renders a real staging bundle at phone width without editor chrome (screenshot artifact)
    - publishing the same document twice yields two immutable version permalinks linked by prev/next, the bare link shows v2, v1's link still shows v1, and the front list shows the story once, marked updated
    - a logged-out Playwright spec on staging opens the front page, opens a story, and sees its numbered sources and version history
    - no served route or app still renders the fixed claims (grep and a request against staging)
  rollback: >-
    git revert per step; versions appended to an existing route stay
    readable as versions after a revert (the old code ignores the link and
    mints new routes again)
  landing:
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: grounded, current Textures readable at permalinks (the metamission's value criterion), visible before the protocol stations finish
  goodharting_would_be: a front page that looks like news while its sources are bare links, or a reader that flattens version history
  realism_axis: Prototype A against staging bundles -> per-version linked permalinks on staging -> logged-out front page and story on staging -> owner-opened AI news stories as the first content
  mutation_class: red (publication and route projection) for steps 1 and 2; orange (app) for steps 3 and 4
  authority_sources:
    - docs/autopaper-sw-design-2026-10-09.md
    - docs/autopaper-app-design-2026-10-11.md
    - owner direction 2026-10-11 evening (stories as Textures that transclude their sources, built on the protocol, designed first)
  must_preserve:
    - only the Texture desk writes a Texture (P2b)
    - public reads never reach a paper computer
    - the published version history and its hashes
  excluded: [quoted capture citations (slice C), auto-publish (W3b), records and scores in the reader (W4), management feeding stories (P2d2, W2)]
  protected_surfaces: [internal/platform/service.go, internal/platform/universal_wire.go, internal/proxy/platform_publish.go, frontend Universal Wire app and routes]
  read_only_dependencies: [internal/textureowner, internal/agentcore]

now:
  status: drafted
  slice: written from the app design for the owner's review; nothing built (owner direction 2026-10-11 evening)
  source_ref: f291d5ac
  deploy_identity: staging b172895b
  candidate: none
  conjecture:
    id: W3a-1
    claim: >-
      The existing publication bundle and Texture renderers are enough for
      a readable public news page once a document's versions are linked
      permalinks, so the reader needs no new content model.
    test: Prototype A, then the logged-out Playwright spec on staging
    edge: missing_oracle
    delta_o: whether the renderers read well outside the editor at phone width
    scope_if_supported: owner-opened story Textures on one paper
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      The reader lands before living stories (W2), reading published
      Textures only. Open for the owner: design open question 3 (reader
      before W2) and question 4 (how long public quotes may be, for the
      later citation slice).
    kind: sequencing
    status: proposed
    evidence_ref: docs/autopaper-app-design-2026-10-11.md §5, §8
  belief:
    believed_state: the public resolve route and renderers exist; per-version linked permalinks and the front list do not
    main_uncertainty: how many per-publish objects a version-append needs (design §6 R2)
    next_observation: the owner's review, then Prototype A
  blocker_or_risk: owner review of the sequencing; publishing changes are red
  next_action: owner reviews this file with the master report; then Prototype A (no deploy needed)

receipts: []
---

# W3a — the Autopaper reader

The AI news demo already produces story Textures, and staging can already
serve a published Texture to a logged-out reader. What is missing is the
paper around them: a front page, a story page outside the editor, and a
linked permalink per version so a living story stays one story while
every version stays citable. This station
builds that reader and deletes the old Universal Wire app, without
touching the protocol work that is held for review.

Ways it can fail, written before the code:

- the renderers depend on editor state and break on a public page
  (Prototype A finds this before any deploy);
- appending a version breaks the version history's hash chain or its
  signatures (the version-append reuses the existing chain);
- a version's permalink changes after a later publish (each /v<n> is
  immutable; only the bare link moves);
- the front list reads from a paper computer and fails when it sleeps
  (served from the shared store only);
- a story published before this change and again after it gets two
  unlinked publications (the first republish after the change links to
  the newest existing publication for that document);
- deleting the old app breaks a Desktop entry or a deep link (the old
  route redirects to the new front page for one release);
- logged-out reads leak an owner-only document (only published routes are
  listed; the resolve route's policy check is unchanged);
- the front page floods with near-duplicate stories (owner-opened stories
  only in this station; clustering belongs to management later).
