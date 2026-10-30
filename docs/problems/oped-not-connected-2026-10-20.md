# Op-eds are not connected: research cannot find published pieces, the paper cannot list them

Status 2026-10-30: Store B publication references predate the PostgreSQL graph database; read Store B as the graph database. The design contract's status was not re-verified in this pass. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

**Status:** documented before any code. Design and failure modes below are the
contract the tests are written against.
**Date:** 2026-10-20
**Decision provenance:** owner, 2026-10-20, `docs/reports/owner-notes-2026-10-14.md`
section 32: "any logged in user should be able to write anything in a texture,
research should pull relevant published texture articles from autopaper amongst
other sources, and user should be able to publish their texture to autopaper.
thats an op-ed.... this may already be implicitly built actually. just needs
better connection." Section 30 ties op-eds to reader corrections; corrections
are out of scope here. Station LC intent (Opinion section, labelled, separate
from synthesized news): `docs/archive/reports/autopaper-ranking-and-contributions-intent-2026-10-14.md`.
**Mutation class:** step 2 (research search) orange; step 3 (publish mode and
public listing) red: publication mode chosen by route, public listing.
Protected surfaces: Texture canonical publish path, the public front list,
proxy auth boundary for guests.

## What exists (read from main, 2026-10-20)

- Any signed-in user can write in Texture and publish: Texture toolbar Publish
  -> `publishTexture` -> `POST /api/platform/texture/publications` ->
  `HandleTexturePublication` -> `publishTextureRevision` -> `Service.PublishTexture`
  with `PublishModeOwner` (mode chosen by the route, never the body,
  `internal/platform/types.go:32-43`). The result is an unlisted public link
  `/pub/texture/<slug>`.
- The paper front lists only auto-mode publications of `CORPUSD_AUTOPAPER_OWNERS`
  (`isAutopaperStory`, `publication_front.go:148-156`). `FrontStory` has no
  author and no kind.
- `texture-ai-disclosure.ts` shows the AI disclosure only for `autopaperStory`.

## The gaps (what "just needs better connection" means in code)

1. **Research cannot find a published piece.** `choir.SearchWireCorpus`
   (`researchtools.go:39-125`) searches the computer's own stories and captures
   only. `Service.SearchPublished` (`service_publication_read.go:625`) searches
   Store B but no research verb calls it, it reads only the first 100
   publication rows (`Limit: 100`, no order, so the 101st publication and every
   later one is invisible forever) and it returns every published Texture,
   including hand-published owner-mode pieces that are neither stories nor
   op-eds.
2. **A guest cannot reach the search.** `requirePlatformObjectGraphCaller`
   (`guest_authority.go`) allows a bound guest only `GET` on
   `/internal/platform/captures/` and own-scoped `/internal/platform/objects`.
   Every other corpusd route is refused to guests. A new read route needs an
   explicit, narrow carve-out.
3. **A published piece is not a capture, so it cannot be cited.** Research
   cites capture ids. The public export is a JSON envelope
   (`/api/platform/publications/export`), which `choir.Read` refuses (Read takes
   HTML, PDF and plain text).
4. **No Opinion mode.** Publish modes are `auto` and `owner`. There is no
   signed-in, per-user, labelled, listable mode.
5. **No byline.** The auth store has no display name (`auth.User` is id, email,
   created_at). The only identity text available is the email, which must never
   be published.
6. **No Opinion bucket on the front, no label on the story page.**

## Smallest design

Step 2, research finds and cites published pieces:

- `Service.SearchPublishedPieces(ctx, query, limit)`: walks publications newest
  first in windows (the front list's `frontPublicationBatch`), not the first
  100. It returns only publications whose state is `published`, with a public
  access policy and an active route, in their latest *published* version, and
  only of kind `story` (an Autopaper story) or `opinion`. It is bounded: at most
  `limit` results (default 10, max 25) and at most a fixed scan cap of
  publications per call; hitting the cap sets `capped: true`, so a miss is never
  silently read as "nothing exists". `SearchPublished` (the existing unfiltered
  route) is fixed the same way, bounded and no longer limited to 100.
- corpusd route `GET /internal/platform/published/search?q=&limit=`; the proxy
  registers the same path through the bound-guest pass-through with a
  read-only `GET` carve-out for that one path.
- `choir.SearchWireCorpus` gains `published: [{route_path, title, snippet, kind,
  author, read_url}]` plus `published_count` and `published_note` on failure.
  `read_url` is a plain-text export URL (`export?route=...&format=txt&raw=1`,
  new `raw=1` returns the text body with `text/plain`), so `choir.Read(read_url)`
  captures the piece and research cites the capture id. Research never cites a
  published piece by search snippet alone. The verb summary says so.

Step 3, Opinion mode:

- `PublishModeOpinion = "opinion"`. A new proxy route
  `POST /api/platform/texture/opinions` runs `publishTextureRevision` with
  that mode. It requires a **cookie session** (`AuthMethod == "cookie"`); an
  API key or a guest is refused. The mode is still set by the route, never by
  the body.
- Author label is computed on the proxy from the session and stored on the
  publication's metadata (`author_label`), fixed by the first version like the
  mode and the owner. No display name exists, so the label is a short handle,
  `writer-<6 hex of sha256(user id)>`; never the email. When a display name
  exists later it replaces the handle without touching the contract.
- Per-user daily limit in `PublishTexture`: a *new* opinion publication is
  refused (`ErrOpinionDailyLimit`, HTTP 429) when the owner already opened
  `CORPUSD_OPINION_DAILY_LIMIT` (default 5) opinion publications in the last 24
  hours. New versions of an existing piece do not count: they do not add listing
  rows.
- Kill switch `CORPUSD_OPINION_LISTING=on|off`, default off in code. Off: an
  opinion publish is refused (409), the front's Opinion bucket is empty, and
  research search returns no opinion kind. On in `nix/node-b.nix` beside
  `CORPUSD_AUTOPAPER_OWNERS`.
- Front: `FrontStory` gains `kind` and `author`. The default list (the news
  front) never contains opinion pieces; `section=opinion` lists them, regardless
  of `CORPUSD_AUTOPAPER_OWNERS`. Story page: the label "Opinion" and the
  human-author line replace the AI disclosure. Toolbar: "Publish as opinion".

## Failure modes, written before code, each with a guarding test

| # | Failure mode | Guard |
|---|---|---|
| F1 | An op-ed counted as a news story: it enters the news front, the related-stories walk, the two-outlet/corroboration inputs, or gets the AI disclosure. `isAutopaperStory` must stay false for opinion mode, the default front list must not contain it, and the story page must show the human-author line, not the AI disclosure. | `TestOpinionPublicationIsNotAnAutopaperStory`, `TestResolvedPublicationNamesOpinionKindAndAuthor` (platform); `frontend/tests/oped-opinion-label.spec.js` (O1-O5, mocked) |
| F2 | An anonymous or API-key caller publishes to Opinion; a body field flips the mode. | `TestOpinionPublishRequiresCookieSessionAndRefusesBodyPolicy` (proxy: no auth 401, API key 403, `publish_mode`, `owner_id`, `access_policy`, `author_label` in the body rejected as unknown fields) |
| F3 | One user floods Opinion. | `TestOpinionDailyLimit`, `TestOpinionHTTPStatuses` (platform: the piece past the limit is refused with 429, a new version of an existing piece is not, another user is not, a day later it resets) |
| F4 | Opinion listed with the kill switch off, or publishable with it off. | `TestOpinionKillSwitchOff`, `TestOpinionListingSwitchParsesOnlyOn` (platform: publish refused with 409, front bucket empty, search has no opinion hits) |
| F5 | Published search returns pieces that were never published: owner-mode drafts, a publication whose state is not `published`, a withdrawn route, or a version that failed to publish. Only published versions; only Autopaper stories and opinion pieces on the research route. | `TestSearchPublishedPiecesOnlyPublishedStoriesAndOpinion` |
| F6 | Search scans only 100 items: the 101st-newest publication is never found. Results must stay bounded. | `TestSearchPublishedPiecesPastTheFirstWindow`, `TestSearchPublishedPiecesBoundedResults` |
| F7 | Research cites a published piece without a capture (cites a snippet). The result must carry a `read_url` that `choir.Read` can capture (plain text), and the verb summary must tell research to Read it and cite the capture id. | `TestPublicationExportRawIsPlainText` (proxy), `TestSearchWireCorpusIncludesPublished`, `TestSearchWireCorpusPublishedFailureIsNotEmpty` (researchtools), `TestSearchWireCorpusSummaryTellsResearchToReadAPublishedPiece` (yaegikernel) |
| F8 | A guest calls a corpusd route outside the carve-out through the new path: other methods, the unfiltered `/retrieval/search`, `/publications/owned`, publish routes. | `TestGuestPublishedSearchCarveOutIsNarrow` (proxy) |
| F9 | The email leaks into the byline. | `TestOpinionAuthorLabel` (platform: empty, email-shaped, markup and over-long labels refused; the label is fixed by the first version), the byline assertions in `TestOpinionPublishRequiresCookieSessionAndRefusesBodyPolicy` (proxy: a handle derived from the user id, no email part) |

## Standing questions (docs/standing-questions.md), short answers

- Single state authority: the publication metadata (`publish_mode`,
  `author_label`, `owner_id`) in Store B is the one record; the front, search
  and story page all read it.
- Deletion citers: none deleted. `SearchPublished` stays; its row-100 cap is
  replaced.
- Restart durability: publication rows are durable; the daily-limit count is
  derived from publication `created_at`, so it survives a restart.
- No-SSH operability: the kill switch is an env var in `nix/node-b.nix`; the
  daily limit has a defaulted env override.
- Artifact-verified success: the deployed spec `frontend/tests/oped-deployed.spec.js`
  writes, publishes as opinion, and checks the logged-out listing, byline and
  label.
- Fate-sharing: opinion listing reads Store B alone, as the front already does.

## Open items

- No display name exists; the byline is a handle until the account model has
  one.
- The toolbar shows "Publish as opinion" to every signed-in user; with the kill
  switch off the server refuses with a plain message. A status endpoint to hide
  the button is not built.
- Reader corrections (section 30) and moderation or unpublish of opinion
  pieces are not in this change. The kill switch hides all opinion pieces at
  once; it is the only takedown lever until moderation is built.

## Landed (2026-10-20)

Commits on `feat/oped-connect`: this doc; research search (orange); opinion
mode (red); frontend; the deployed spec `frontend/tests/oped-deployed.spec.js`
(written, not run: it needs a deploy with `CORPUSD_OPINION_LISTING=on`).
Extra guard beyond the table: publishing as opinion a Texture first published as
a plain link is refused (409, `ErrOpinionModeConflict`), because a publication
keeps its first mode for good and would otherwise stay silently unlisted.
Off-switch without a deploy: write `CORPUSD_OPINION_LISTING=off` to
`/var/lib/go-choir/corpusd-retrieval.env` and restart corpusd.

## Red ceremony at landing (2026-10-21)

Conjecture delta: before, a signed-in user's Texture piece could only be
an unlisted link and research could not find published Autopaper
stories; after, research's corpus search returns published stories and
can capture one, and a signed-in user's piece can be listed under
Opinion, labelled and bylined, while the news front is unchanged.
Admissible evidence: `frontend/tests/oped-deployed.spec.js` on
choir.news with a fresh test account (unit tests are shaping only).
Rollback: `CORPUSD_OPINION_LISTING=off` hides every opinion piece and
refuses new ones; revert for the code. Heresy delta: discovered, the
published-search cap of 100 rows and a published article that could not
be cited; repaired, both, by this change; introduced, an opinion piece
has no unpublish or moderation path beyond the kill switch (named in
Open items).
