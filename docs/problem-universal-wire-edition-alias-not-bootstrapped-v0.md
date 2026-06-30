# Problem: Universal Wire edition alias is never bootstrapped in production

**Date:** 2026-06-29
**Mission:** news-live-pr-merge-model-default-v0 (Track B)
**Status:** documented (fix pending)
**Mutation class of this doc:** green

## Evidence

The Universal Wire API at `/api/universal-wire/stories` on staging
(choir.news) returns zero stories. The empty-state diagnostics report
`texture_edition` substrate state as `missing` ("No Universal Wire Texture
edition alias is present.").

## Root cause (the specific broken link)

**The Universal Wire edition document and its alias
`universal-wire/Wire.texture` are never created in production.** The
publication path that links a synthesized Texture article into the wire
feed — `Runtime.autonomousPublishWireArticleToEdition` in
`internal/runtime/wire_publication.go` — resolves the edition alias with
`Store.GetDocumentAlias(ownerID, "universal-wire/Wire.texture")`. When that
returns `store.ErrNotFound` the function returns `("", nil)`: a silent
no-op. The caller (`maybeAutonomousPublishWireArticle`) then leaves the
publication work item open and returns without transcluding the story.

The Universal Wire API (`HandleUniversalWireStories` in
`internal/runtime/universal_wire.go`) reads stories exclusively from the
edition: it resolves the same alias, reads the edition revision, parses
`texture:<docID>` transclusion refs, and renders a `WireStory` per
transcluded document. With no edition alias, the API returns zero stories
every time, regardless of how many articles the upstream pipeline
(sourcecycled → processor → Texture agent → platform publish) actually
produced.

A repo-wide search confirms `UpsertDocumentAlias` for
`universalWireEditionSourcePath` (`"universal-wire/Wire.texture"`) is
called **only in test fixtures** (`seedUniversalWireEditionFixture` in
`internal/runtime/universal_wire_test.go`). No production code, no
startup bootstrap, no migration, and no conductor route creates the
edition document or its alias. The edition exists in tests only because
the tests seed it by hand.

## Pipeline trace (why the rest of the chain is not the blocker)

1. **sourcecycled** (`cmd/sourcecycled/main.go`): fetches sources, writes
   web captures to the object graph via the runtime API, builds ingestion
   handoffs, and dispatches processor runs. This path is wired and does
   not depend on the edition alias.
2. **processor → Texture agent** (`internal/runtime/tools_coagent.go`,
   `wire_synthesis.go`): the processor spawns a Texture agent; the Texture
   agent writes a canonical revision via `patch_texture`. The coagent
   prompt (`buildCoagentTextureRevisionPrompt`) now includes source body
   text. This path does not depend on the edition alias.
3. **platform publish** (`internal/runtime/wire_platform_publish.go`):
   `publishWireArticleToPlatform` publishes the canonical revision to
   platformd/wire-publish and stamps `platformd_route_path` on the
   revision metadata. This runs **before** the edition step and does not
   depend on the edition alias.
4. **edition linkage** (`autonomousPublishWireArticleToEdition`): THIS is
   the broken link. It needs the alias to exist; it never does in
   production; it silently no-ops.
5. **API read** (`HandleUniversalWireStories`): reads the edition. With no
   edition, returns zero stories with a `missing` texture_edition
   diagnostic.

So even on a fully-healthy staging deployment where sourcecycled is
running, sources are fetched, the processor opens stories, the Texture
agent writes articles, and platformd publishes them, the Universal Wire
feed stays empty because the edition that is supposed to transclude those
articles is never created.

## Why this is fixable locally

The edition is a platform-owned document
(`ownerID = universal-wire-platform`). The wire publication policy is
already the canonical writer for the edition (it creates edition
revisions with `source: "universal_wire_edition"`). Making
`autonomousPublishWireArticleToEdition` bootstrap the edition document +
alias + initial revision on first use (when `GetDocumentAlias` returns
`ErrNotFound`) is a self-healing fix that needs no staging config change,
no new service, and no operator action. The first eligible article
publication creates the edition; subsequent publications append
transclusions as they do today.

## Open edges (after the fix)

- **Platform publish configuration:** `publishWireArticleToPlatform`
  returns `"wire publish is not configured"` if neither
  `WirePublishURL`/`PlatformdURL` nor the env-derived fallback base is
  set. If staging lacks that configuration, the publication path fails
  before reaching the edition step. This is a staging-config concern, not
  a code gap, and must be verified on staging.
- **sourcecycled deployment on Node B:** whether sourcecycled is actually
  running and fetching sources on Node B is a staging-ops concern. The
  code path is wired; this doc does not assert it is deployed.
- **platformd story verification:** when `platformdReadBaseURL()` is
  non-empty, `HandleUniversalWireStories` requires platformd to have the
  published texture document/revision. This is satisfied by the platform
  publish step above; verify on staging.

## Next step

Fix `autonomousPublishWireArticleToEdition` to bootstrap the edition on
first use, add a regression test that proves the edition is created and
the API returns stories without a pre-seeded edition alias, and run the
wire publication + universal wire test suites.
