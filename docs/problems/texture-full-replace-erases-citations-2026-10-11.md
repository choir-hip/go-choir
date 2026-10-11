# A Texture full replace erases every citation

Found 2026-10-11 in S1e (staging 78c9173e, document b2352fec). Evidence
class: revision JSON for all eight revisions (scratchpad
demo13/ainews/revisions) plus a unit test reproducing the drop.

## What happened

The owner's follow-up asked for "a date and a source link for each item".
The final brief has nine items and three citations. Research packets
carried about a dozen sources across rounds 7629c97f, 348535e2 and
ddb8a7c7.

Source entities and source_ref nodes per revision:

| revision | entities | source_ref nodes in body |
|---|---|---|
| 4 (Texture) | 2 (mistral.ai, council.nyc.gov) | 2 |
| 5 (owner follow-up) | 2 | 2 |
| 6 (Texture, full replace in turn ea5e62fa) | 0 | 0 |
| 7 (Texture) | 1 | 1 |
| 8 (Texture) | 3 | 3 |

Revision 6 is a full-replace apply (`"content"`). In
`materializeTextureToolEdit` (internal/textureowner/tools_texture.go),
`replace_all` sets `entities = nil`. The markdown parser refuses citation
markers. Every source the document held is gone, and nothing in the cell
result says so. Later revisions re-cited only the sources in the packets
that woke them.

Turn ea5e62fa chose the full replace after a cell lost to a compile error
(a function declaration) and several rereads. The prompt reserves full
replace for recovery rewrites, but nothing warns that recovery costs the
citations.

A smaller projection gap: the owner revision (5) copies the body and the
entities but commits an empty source-graph write set, so its source_ref
rows are missing. The body still holds the nodes, and Texture's next edit
reads the body, so nothing is lost. Left as a named residual.

## Cause (belief)

A full replace treats the document's sources as part of the old body.
Sources are evidence the document holds; a rewrite of the prose should
not discard them.

## Test

After a full replace, every source the document held stays in its source
list, marked unused when the new body does not cite it. The next edit can
cite it again with insert_source_ref. An explicit delete of a source_ref
node still removes its source; that is a deliberate act.
