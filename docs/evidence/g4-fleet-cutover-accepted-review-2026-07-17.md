# G4 Frozen Fleet Cutover — Accepted Independent Review

**Adjudicated:** 2026-07-17. **Verdict:** `accept` for entry to strictly serialized fleet cutover. This is not a route signature and does not itself authorize any CAS.

## Frozen candidate

- Runtime/deployed base: `42e50b6b1fa3ae7461bb789ec173521a768b548d`.
- Documentation identity: `6db85e456e2ec2817555c5efd9e1167ec5c2be45`.
- Candidate: [`g4-fleet-cutover-candidate-2026-07-17.json`](g4-fleet-cutover-candidate-2026-07-17.json).
- Candidate SHA-256: `b7f4529535990596b54d2f97e230470cc7d6ba4ec4e17f1c00d67beeed345b1f`.
- Scope: candidate JSON, active Definition, and candidate-named evidence refs.
- Authority: `fleet_mutation_authorized: false`; each route still requires exact live preconditions, owner approval, accepted G3 envelope, and vmctl's signed transition path.

## Panel health and adjudication

The first review wave found one governing repair blocker: closing owner reconstruction and active-disposal receipts were only ephemeral hashes, while the frozen Definition still said deployment was pending. Three reviewers accepted; Cursor returned `repair`. The repair was applied as the durable adjacent Markdown/JSON zero-realization receipt and reconciled Definition at `6db85e45`. No behavior or frozen runtime changed.

The repaired frozen packet was reviewed by the default seven-runner panel. Four returned usable verdicts before the 300-second deadline:

- Cursor: `accept`, no blocking finding, high confidence;
- OMP Gemini 3.5: `accept`, no blocking finding, high confidence;
- OMP GPT-5.5: `accept`, no blocking finding, high confidence;
- OpenCode: `accept`, no blocking finding, high confidence.

Devin, Codex CLI, and OMP GLM 5.2 timed out without a verdict. Timeouts are reviewer-health telemetry, not votes. No minority blocker remained.

## Deterministic findings

Reviewers independently recomputed the packet digest and all 150 `plan_sha256` values using compact sorted-key JSON with `plan_sha256` omitted: 150/150 matched. They confirmed:

- 150 unique route slots and owner/desktop pairs; serial sequence `0..149`;
- 149 mutation-required legacy plans and one retained accepted control route;
- classification counts match the frozen inventory;
- each mutation route binds exact VM ID, epoch, state, persisted-row digest, route absence, construction absence, owner signature, sparse allocation policy, and old-image read prohibition;
- every bootstrap generation-0 transition has a signed generation-1 `bootstrap_rollback` companion before CAS;
- the recovered ArtifactProgramRef appears in exactly one plan, owner `5bd6de97-3b58-408c-bf89-c42c81b083de`; all other plans use the baseline with no legacy migration claim;
- legacy detach/restore, route CAS, and unrouted disposal share vmctl's mutation lock and exact route checks;
- the three discovered G4 substrate blockers—legacy detach/restore, first-bootstrap rollback-to-absence, and active unrouted disposal—are repaired and deployed;
- CI run `29565482629` is successful for the deployed runtime base.

## Residual risks and execution rules

- Freeze-to-execution drift must return refusal. Re-freeze the affected route; never override its epoch/state/row digest.
- Run one route at a time. A failure stops the fleet until that route is rolled back to absence, its candidate disposed, its exact legacy ownership restored, and restart readback passes.
- Reverify the retained control route before the first mutation.
- Every route must collect its own construction, independent verification, disk geometry/allocation, signed authorization, transition, product-path, and restart receipts.
- The packet's plan digest scheme is reproducible but not self-describing. Do not change the reviewed JSON to add metadata; preserve the accepted digest and name the scheme in execution tooling/receipts.
- The 134 missing-auth rows and other baseline-reset rows intentionally make no legacy-data migration claim. Their signed authority remains mandatory.

**G4 adjudication:** accepted. Entry to `F-cutover-fleet-and-close` is authorized subject to the packet's strict serialization and per-route signed gates. G5 remains mandatory before terminal closure.
