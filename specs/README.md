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
It models a closed, role-authorized mutation vocabulary; pre-entropy logical
command-digest reservation (including same-key conflict refusal); and a typed
pending plan through prepare, CAS acknowledgement, normal finalization, and
crash recovery.  The plan carries complete rebase/cancel manifests, and no
semantic state becomes visible before the exact acknowledged plan finalizes.

The model includes three assignments, four attempts/results, retry,
cancellation, late and out-of-order result handling, dissent, rebase, and
disposition-gated settlement. Owner settlement acceptance and Super settlement
proposal are distinct and must both be fresh. Settlement is terminal: later
delivery is retained only as non-authorable evidence, while a successor
requires a new trajectory.

Candidates bind an explicit selected incorporated-result set and its digests
to a semantic composition base, separately from the sequencing head emitted by
the composition event. Verification is authored by an independent verifier and
carries immutable evidence identity; acceptance requires that fresh verified
head. A second/duplicate pending transition is refused. The model covers
materialization failure with compensating history and successful
materialization to the effective candidate. Rollback/effect/checkpoint/route
kinds are forbidden by the action vocabulary, not claimed as reachable
deployment actions.

`texture_supervision.cfg` is a tractable branching safety exploration: one
assignment/attempt/result/candidate and two canonical events.
`texture_supervision_witness.cfg` is a separate 39-event fair reachability
proof. Its explicit temporal property reaches three assignments through retry,
cancellation, a late result, dispositions, candidate composition, independent
verification, owner acceptance, second-pending refusal, materialization
failure/compensation, successful materialization, both settlement proposals,
and owner settlement. Every canonical digest differs from its predecessor.
Projection rebuild is separately constrained to a replay-derived fold of the
immutable event-digest/mutation tape, rather than a copy of live projection
state.
The witness is not an exhaustive state-space claim. Neither configuration
represents updater, capsule, checkpoint, or route application as deployment
authority. Their compatibility floor is an ordinary platform source/guest-image
release through main, CI, and staging, with effects off.

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
