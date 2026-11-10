# Lane G (G1 + G2): style guides as Textures, read at read time: how it can fail

Extends the G1 list (`docs/evidence/autopaper-g1-house-style-failures-2026-11-10.md`
on branch `lane-g1-style`, commit 3096cb967). G1 was held because the guide
lived in a frontend constant. This lane moves it into a Texture document,
read through a Texture surface, with a personal guide overriding the house one.
Pass 1: read time only. No desk prompt reads a style guide.

## Interpretation (spec silent)

- A style guide is a Texture document titled "Style Guide" on a computer,
  with a `## Design values` list of `- key: value` lines. Same format as G1.
- Texture (`internal/textureowner`) is the only reader of its revisions. It
  parses the current revision into a closed, typed design and serves only
  that: `GET /api/texture/style` (owner route). No guide text leaves Texture.
- The parser lives once, in Go (`internal/styleguide`). The frontend maps
  the closed names to fixed CSS strings and never parses a document.
- Personal beats house for that reader only: the reader's own computer
  answers the route from the reader's own document.

## Ways it can fail (written before any code)

1. Hostile CSS or values in a guide (a forked Texture someone else wrote).
   Defence: values are names from closed tables; the response carries names,
   the frontend maps names to fixed strings, re-checking against the table.
2. Unparseable, empty, binary or oversize guide. Defence: size cap, line
   cap, NUL refusal; the answer is the compiled default with source
   "default" and the reason; never an error that blanks the page.
3. No guide document. Defence: source "default"; the page is today's page.
4. Repeated keys, case variants, values in fences or other sections. As G1.
5. Another owner's guide leaks. Defence: the route resolves the document by
   the authenticated owner only; a test has two owners, one guide.
6. A forked or taken guide carries foreign values. Defence: same parse;
   the closed set is the only thing that survives.
7. Stale cache. Defence: `Cache-Control: no-store`; the frontend fetches on
   mount; the answer names the revision id so a proof can see which revision
   was read.
8. Two documents titled Style Guide. Defence: newest unarchived wins, and
   the answer names the doc id.
9. An in-process desk reads the guide (style reaching a prompt). Defence:
   the route is owner-only (the algorithm2 guard: an in-process desk call
   has no remote address); nothing in agentcore or the writer imports the
   package; a repo test greps for that.
10. The guide document is edited by a desk. Out of scope: the write path is
    untouched; owners edit it as any Texture.
11. Signed-out reader or a stopped computer: the route fails. Defence: the
    frontend uses plain fetch, ignores any failure and keeps the default.
12. The new shell-level request breaks package contracts. Defence: the stub
    answers `/api/texture/style`.

## Known design problem, not built (stop point)

The public front page deliberately never calls the paper computer (the front
list is corpusd alone, so the page stays up while the paper computer is
stopped). A house guide that lives only as a live document on the paper
computer therefore cannot reach a signed-out or other-computer reader without
breaking that. The house path needs the guide to be a published Texture in
corpusd (the paper publishes it like a story) and a corpusd read of its
latest version. That is more than this lane; this lane builds the Texture
read surface and the personal override, and leaves the public house channel
as a named residual.
