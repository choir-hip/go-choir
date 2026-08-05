# specs/ — historical model snapshots

The checked-in TLA+ modules are historical verification artifacts. They do not
define current Choir product authority or gate the active self-development
mission.

Current product authority lives in:

- `docs/choir-doctrine.md`
- `docs/computer-ontology.md`
- `docs/agent-product-doctrine.md`
- the active Definition named by `docs/ACTIVE.md`

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
