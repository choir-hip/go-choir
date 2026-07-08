# Problem: Universal Wire edition alias is never bootstrapped in production

**Date:** 2026-06-29
**Mission:** news-live-pr-merge-model-default-v0 (Track B)
**Status:** documented + fixed
**Mutation class of this doc:** green

## Evidence

The Universal Wire API at `/api/universal-wire/stories` on staging
(choir.news) returns zero stories. The empty-state diagnostics report
the `texture_edition` substrate state as `missing` ("No Universal Wire
Texture edition alias is present."). A local `curl` to
`https://choir.news/api/universal-wire/stories` returns `401`
(auth required), confirming the route is live but the feed is empty
behind auth.

## Root cause (the specific broken link)

**The Universal Wire edition document and its alias
`universal-wire/Wire.texture` are never created in production.** The
publication path that links a synthesized Texture article into the wire
feed — `Runtime.autonomousPublishWireArticleToEdition` in
`internal/runtime/wire_publication.go` — resolved the edition alias with
`Store.GetDocumentAlias(ownerID, "universal-wire/Wire.texture")`. When
that returned `store.ErrNotFound` the function returned `("", nil)`: a
silent no-op. The caller (`maybeAutonomousPublishWireArticle`) then left
the publication work item open and returned without transcluding the
story.

The Universal Wire API (`HandleUniversalWireStories` in
`internal/runtime/universal_wire.go`) reads stories exclusively from the
edition: it resolves the same alias, reads the edition revision, parses
`texture:<docID>` transclusion refs, and renders a `WireStory` per
transcluded document. With no edition alias, the API returns zero
stories every time, regardless of how many articles the upstream
pipeline (sourcecycled -> processor -> Texture agent -> platform
publish) actually produced.

A repo-wide search confirms `UpsertDocumentAlias` for
`universalWireEditionSourcePath` (`"universal-wire/Wire.texture"`) was
called **only in test fixtures** (`seedUniversalWireEditionFixture` in
`internal/runtime/universal_wire_test.go`). No production code, no
startup bootstrap, no migration, and no conductor route created the
edition document or its alias. The edition existed in tests only because
the tests seed it by hand.

## Pipeline trace (why the rest of the chain is not the blocker)

1. **sourcecycled** (`cmd/sourcecycled/main.go`): fetches sources,
   writes web captures to the object graph via the runtime API, builds
   ingestion handoffs, and dispatches processor runs. Wired; does not
   depend on the edition alias.
2. **processor -> Texture agent** (`internal/runtime/tools_coagent.go`,
   `wire_synthesis.go`): the processor spawns a Texture agent; the
   Texture agent writes a canonical revision via `patch_texture`. The
   coagent prompt (`buildCoagentTextureRevisionPrompt`) includes source
   body text. Does not depend on the edition alias.
3. **platform publish** (`internal/runtime/wire_platform_publish.go`):
   `publishWireArticleToPlatform` publishes the canonical revision to
   platformd/wire-publish and stamps `platformd_route_path` on the
   revision metadata. Runs **before** the edition step; does not depend
   on the edition alias.
4. **edition linkage** (`autonomousPublishWireArticleToEdition`): THIS
   was the broken link. It needed the alias to exist; it never did in
   production; it silently no-oped.
5. **API read** (`HandleUniversalWireStories`): reads the edition. With
   no edition, returned zero stories with a `missing` texture_edition
   diagnostic.

So even on a fully-healthy staging deployment where sourcecycled is
running, sources are fetched, the processor opens stories, the Texture
agent writes articles, and platformd publishes them, the Universal Wire
feed stayed empty because the edition that transcludes those articles
was never created.

## Fix

`internal/runtime/wire_publication.go`: added
`Runtime.ensureUniversalWireEdition(ctx, ownerID)`, called from
`autonomousPublishWireArticleToEdition`. When the edition alias is
missing (`store.ErrNotFound`), it bootstraps the platform-owned edition
document, an initial canonical revision (`# Wire` seed), and registers
the `universal-wire/Wire.texture` alias. The first eligible article
publication creates the edition; subsequent publications append
transclusions as before. The wire publication policy is already the
canonical writer for the edition, so this is a self-healing bootstrap
that needs no staging config change, no new service, and no operator
action.

**Mutation class:** orange (runtime behavior, wire publication path).
**Protected surfaces touched:** Texture canonical writes for the
platform-owned edition document (intentional — the wire publication
policy is the canonical writer for this document).
**Rollback path:** revert the commit; existing pre-seeded editions
(test fixtures, any staging edition created after deploy) are
unaffected because `ensureUniversalWireEdition` short-circuits when the
alias already exists.

## Evidence the fix works (local)

- `go test ./internal/runtime/... -run Wire` — all pass, including the
  new regression `TestWireAutonomousPublishBootstrapsEditionWhenAliasMissing`
  which deliberately does NOT pre-seed the edition fixture, publishes a
  story, and asserts (a) the edition alias is created, (b) the edition
  transcludes the story, and (c) `/api/universal-wire/stories` returns
  the story with `source = "universal-wire-edition-texture"`.
- `go test ./internal/wirepublish/...` and `go test ./cmd/sourcecycled/...`
  — pass.
- Existing tests that pre-seed the edition fixture still pass
  (`ensureUniversalWireEdition` reuses the existing alias when present).

## Open edges (need staging verification)

- **Platform publish configuration on staging:**
  `publishWireArticleToPlatform` returns `"wire publish is not
  configured"` if neither `WirePublishURL`/`PlatformdURL` nor the
  env-derived fallback base is set. If staging lacks that configuration,
  the publication path fails before reaching the edition step. This is a
  staging-config concern; verify on Node B.
- **sourcecycled deployment on Node B:** whether sourcecycled is
  actually running and fetching sources on Node B is a staging-ops
  concern. The code path is wired; this doc does not assert it is
  deployed. Check the sourcecycled service health on Node B.
- **platformd story verification:** when `platformdReadBaseURL()` is
  non-empty, `HandleUniversalWireStories` requires platformd to have
  the published texture document/revision. This is satisfied by the
  platform publish step; verify on staging.
- **Deploy + acceptance:** the fix is a behavior-changing commit; per
  the landing loop it must be pushed, CI monitored, Node B deploy
  confirmed, and `curl https://choir.news/api/universal-wire/stories`
  (with auth) re-run to confirm real articles appear. I could not run
  the deployed acceptance proof myself (no staging auth credentials in
  this environment).

## Next step

Push the fix to main, monitor CI, confirm Node B deploy, and run the
deployed acceptance proof with authenticated staging credentials.
