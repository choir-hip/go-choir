# Turn-Outcome vs Revision-Outcome Census — Before/After Authoring Repair

Mission: choir-scheduling-and-candidate-proof-2026-08-21, Phase 2 census gate.
Tool: `scripts/selfdev-turn-census.sh` (read-only, owner product path).

## Definitions

- **Turn outcomes** (durable in `TextureTurnRecord`, one per
  `texture_turn_committed` lifecycle event): `revision`,
  `no_semantic_change`, `wait`, `block`. Only `revision` advances the
  canonical document head.
- **Revision outcomes**: `TextureRevision` rows authored by `AuthorAppAgent`
  (the Texture agent). Owner edits are `AuthorUser` and are not Texture-agent
  revisions.
- **The invariant under test** (ground truth item 10): semantic-changing
  authoring turns produce exactly one revision each; wait/block/no-change
  turns produce none; scheduler/assignment lifecycle never calls
  `ApplyTextureTurn`.

## Before repair (staging, 2026-08-21 evidence)

Observed on staging computer `computer-03335285269bdba4f94377e56879f9e6`
epoch 361 before this mission's Phase 2 landed:

| Metric | Value | Source |
|---|---|---|
| selfdev supervision documents | 9 ("Self-development supervision") | GET /api/texture/documents |
| texture_turn_committed events | ~250 across trajectories | docs/reviews panel finding |
| canonical head revisions per doc | v0-v2 only | GET /api/texture/documents/{id}/revisions |
| synthetic join wait-turns | every Super rewake minted one via deterministic caller run (`selfdev_texture_join.go` ApplyTextureTurn with Outcome=TextureTurnWait) | code inspection pre-repair |

The defect: turn events vastly outnumbered revisions because every runtime
rewake committed a synthetic wait-turn. Panel finding (2026-08-21): the ~250:2
ratio was *correct behavior* for genuine wait turns — but the synthetic join
projection manufactured wait-turns without any Texture agent decision at all.

## After repair (this commit 9de9683c)

Code-level changes that alter the census profile:

1. Rewake path queues an owner instruction; the genuine Texture agent commits
   its own turn outcome. Synthetic wait-turns from `ensureSelfDevelopmentTextureJoin`
   no longer occur after a terminal Super.
2. First-time opener retains exactly one direct commit (subject creation).
3. Scheduler/assignment lifecycle never calls `ApplyTextureTurn` (verified:
   `cosuper_assignment_runtime.go` has no ApplyTextureTurn callsite).

Expected post-deploy census signature:

- New supervision trajectories: exactly 1 `texture_turn_committed` per
  semantic change, each paired with exactly 1 appagent-authored revision.
- Rewake occurrences: zero new `texture_turn_committed` until the Texture
  agent itself runs; its turn then appears with outcome attributable to an
  agent-authored run (not the deterministic caller run).
- No revision ever authored by a deterministic run ID
  (`uuid.NewSHA1(..."texture-run")` pattern).

## Collection status

- Staging computer unresolved ("failed to resolve user autoputer", 502) since
  ~01:15Z 2026-08-22; product-path census blocked on VM availability.
- Local verification: all three rewake-path tests pass 5x consecutive;
  `TestApplyTextureTurnDoesNotSpendArrivalOrdinalOnResearcherControls` and the
  ordinal tests confirm lifecycle paths do not touch document state.

## Re-collection command

Once staging resolves:

```bash
CHOIR_API_KEY=... scripts/selfdev-turn-census.sh
```

The gate passes when: semantic changes → exactly one version each; wait/block
→ none; no milestone-forced revisions; identity metadata present on all
appagent revisions.
