# specs/ — current and historical formal models

Formal models are bounded evidence artifacts. They do not define product
authority or substitute for code conformance, deployed reconstruction, or
product-path acceptance.

Current product authority lives in:

- `docs/choir-doctrine.md`
- `docs/computer-ontology.md`
- `docs/agent-product-doctrine.md`
- the active Definition named by `docs/ACTIVE.md`

`texture_supervision.tla` is the current bounded safety model for
[`choir-texture-tape-supervision-2026-08-03.md`](../docs/definitions/choir-texture-tape-supervision-2026-08-03.md).
It maps the canonical appender's prepare/CAS/finalize recovery boundary, typed
assignment and attempt lineage, reconciliation-gated settlement, release-level
write disablement, and the future one-current-base composed-candidate seam.
`texture_supervision.cfg` exhaustively checks the declared finite model. Its
admissible evidence is limited to the named state-machine invariants.

`actor_protocol.tla` and `autoputer_lifecycle.tla` preserve earlier model
snapshots and their model-checking receipts. Their terminology and topology
must not be used as current implementation guidance without an explicit,
current conformance binding.

The former `promotion_protocol.tla` candidate-branch/route-flip model was
removed by the self-development clean cutover. Current acceptance is an
immutable per-computer event followed by verified guest materialization,
checkpoint publication, and vmctl-owned route projection; a speculative
candidate computer, branch merge/tag, or reset rollback is not an authority.

Future load-bearing specs must name their exact implementation mapping,
admissible evidence, and active Definition gate before this registry describes
them as current. Historical superseded models remain available in Git history.
