# The paper tracer cannot see a test computer's publications

Status: open. Found 2026-10-13 on freemodels-paper-2 (build 18d01cc0).
Mutation class of the fix: orange (an owner-scoped read API) or yellow
(tracer only).

## Evidence

- `scripts/reeng_tracer.mjs --scenario paper` decides "published" by
  matching opened story titles against `GET /api/platform/publications/front`.
- Since 07af852a (2026-10-13 15:14Z, "designate the Autopaper paper
  computer"), `nix/node-b.nix` sets `CORPUSD_AUTOPAPER_OWNERS` to the paper
  owner, and `isAutopaperStory` lists only that owner's stories.
- freemodels-paper-2 on a minted computer: 11 stories opened, 33 Texture
  revisions committed, tracer exit 6 with `published: []`. The proxy's
  `/health` counters over the same window show 49 story publishes, all
  answered `published` (HTTP 201), and no failures. c2-paper-7 passed on
  761d94c5, before the allowlist.
- The match is also by title, which a revision may rewrite.

So the scenario's last claim has been unobservable on every minted computer
since 07af852a: a pass is impossible and a failure says nothing.

## Side finding

`platform_publish.corpusd` and `platform_publish.total` count every 201 as an
error (`errors: 49` with `by_status: {http_201: 49}`).

## Options

1. An owner-scoped read: `GET /api/platform/texture/publications?doc_id=`
   returns the caller's publication for that document (route, version
   count, latest revision). The tracer reads it per opened story, by doc id
   rather than title; the owner learns where a story is published.
2. Tracer only: read the public bundle by route, which needs the route the
   guest is told at publish time and does not keep.

Option 1 is the honest instrument and has owner value.

## Resolution (2026-10-13)

Option 1 landed at 29bbab97. `GET /api/platform/texture/publications?doc_id=`
takes the owner from the caller's credential, and corpusd answers at
`/internal/platform/publications/owned`. The response also says whether the
front list shows the publication (`on_front`). The paper scenario passes on
a publication with `publish_mode` auto whose public route resolves.

It passed on d2324a55 (docs/evidence/free-models-paper-3-2026-10-13.json):
8 stories published from a test computer, all with `on_front` false, as the
allowlist intends. The proxy counter that files 201s under errors is still
open.
