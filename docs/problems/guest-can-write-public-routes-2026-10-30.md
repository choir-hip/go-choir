# A computer can write public routes and publications through the generic object route (2026-10-30)

Status: open. Found by reading code during the object graph redesign
(`docs/design/object-graph-redesign-2026-10-30.md`). Two investigators found
it independently, and I confirmed it by reading the code. It has not been
drilled; see "Why no drill" below.

## The problem

A guest computer can write host-owned publication kinds through the generic
object-graph route, and a public URL resolves to whichever route row the
database returns first.

1. **The proxy lets any bound guest PUT or POST any object under its own
   owner and computer** (`internal/proxy/guest_authority.go:133-163`). The
   only kinds it refuses are `choir.web_capture`, the software release, the
   package release, and the release listing. Everything else passes:
   `choir.public_route`, `choir.publication`, `choir.publication_version`,
   and the manifest and blob kinds.
2. **corpusd records every write as its own.** The store stamps
   `system:corpusd` as the writer and never takes it from the request
   (`internal/platform/objectgraph_store.go:938`). The writer gate defaults
   to `observe` because no Nix file sets `CHOIR_OG_WRITER_GATE`. So
   per-kind single writer cannot refuse the guest's write, even in enforce
   mode.
3. **A route is found globally by path, with no owner check and no
   deterministic winner.**
   - `resolvePublicationHead` calls
     `GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", bare)`
     (`internal/platform/service_publication_read.go:68`).
   - That function seeks `og_lookup_keys` with `LIMIT ?` and then reads
     `LIMIT 1`, with no `ORDER BY`
     (`internal/platform/objectgraph_store.go:433, 504-530`).
   - Nothing makes `route_path` unique.
   - Nothing checks that the route's owner equals the target publication's
     owner, or that the route was written by the publish path.

## What a computer could do (inferred)

- **Serve a page on choir.news without the publish path.** The page would
  skip consent, review, the story gate, route naming and the platform
  signature. It needs a route, a publication in state `published`, a
  version, and the `has_version` edges, all under the guest's own owner. A
  guest may write edges whose endpoints it owns.
- **Take over an existing public URL.** Write a second route with the same
  `route_path`. Which row wins is up to the planner. The `og_lookup_keys`
  primary key leads with `(object_kind, lookup_key)`, so an index-ordered
  seek likely returns the lowest `canonical_id`, which embeds the owner id.
  Several owners' ids sort before the paper's.

## Evidence that it has not been used (2026-10-30 14:25 UTC, `graph` database, metadata only)

- There are 3,488 `choir.public_route` rows across 18 owners. Every one has
  an empty `computer_id`. Only the real publish path writes routes with an
  empty `computer_id`. A route written through the proxy must carry the
  guest's computer id, because the proxy requires it.
- No `route_path` appears twice.
- No route targets a publication of another owner.

## Why no drill

The adversarial drill would run on d@e, but it would put a forged page on
the public site. The code path is unambiguous, so the fix is checked by a
test against the PostgreSQL store, and by a deployed refusal probe from
d@e that writes nothing when refused.

## The fix (separate commit)

A read-only check found that no guest code writes through the generic object
or edge routes. The only client, `objectgraph.HTTPStore`, has no caller.
Guests write through the typed routes: captures, support verdicts, story
relations, entities and entity candidates. So the fix allows zero guest
writes on the generic routes, rather than refusing particular kinds:

1. **corpusd** (`internal/platform/objectgraph_handlers.go`). A generic
   object or edge write (POST or PUT) needs the host-internal caller. A
   guest reaches corpusd directly at its platform URL, so this is the gate
   that matters. The per-kind release refusal, the guest checks on edge
   ownership, and the bound-guest checks inside create and put are deleted.
   They no longer guard anything.
2. **Proxy** (`internal/proxy/guest_authority.go`). The same rule applies
   to relayed requests, and replaces the denylist.
3. **Deferred to the redesign** (§5.5). Routes become a host-owned table
   with `path` as the primary key. Until then, nothing but the host
   publish path can write a route, so duplicate paths cannot arise from a
   guest.
