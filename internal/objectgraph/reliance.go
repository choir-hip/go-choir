package objectgraph

// EdgeReliesOn is the reliance link of the Autopaper object graph
// (docs/design/autopaper-learning-materiality-2026-11-04.md sections 6 and 8.6;
// pre-registered in docs/precommitments/autopaper-materiality-hypotheses-2026-11-10.md).
// It runs from a published story to an earlier published story whose capture
// the first story cites: the later story relied on the earlier one.
//
// It is data only. Nothing ranks, lists, searches or prompts by it, and no
// desk that writes or chooses stories sees it; the settlement of the
// materiality hypotheses reads it offline.
const EdgeReliesOn EdgeKind = "relies_on"

// ReliesOnMetadata is the edge's whole metadata. It is fixed, so the edge id
// (derived from the metadata) is the same on every computation and a
// republish writes the same edge, not another. It carries no key the
// vocabulary write guard reads as a desk name.
const ReliesOnMetadata = `{"relation":"relies_on"}`
