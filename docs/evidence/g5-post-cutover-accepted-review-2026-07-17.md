# G5 Post-Cutover Closure — Accepted Review

**Gate:** `G5-frozen-post-cutover-closure`
**Adjudication:** **accept**
**Mutation class:** **yellow** — this receipt changes mission/registry optimization pressure and completion authority, not deployed product behavior.
**Frozen deployed source:** `2bc1799f72ce437b35d4606a23d14e62b7239ac5`
**Durable evidence commit:** `015ab0151598cfee9601dc423fd39ee1bba87abe`
**Packet SHA-256:** `fdec8720e5d786809c9605f1c8b28aa2a3b23037ad3bf74ed759b1722e21fa50`
**Reproducibility artifact SHA-256:** `23b8e439a512c3c10200798bf5fc8ef58caf450ffcd0a47c79ed3d0c1c0b4491`

## Decision

The final independent panel returned four `accept` verdicts with high confidence and no reproducible blockers: Cursor, OpenCode, Gemini 3.5, and GPT-5.5. Codex, Devin, and GLM 5.2 timed out without a verdict. The panel independently verified the source, tree, archive, packet, artifact, and content digests; recomputed all canonical section digests; matched all 149 execution receipts to 149 of 150 final joins; identified the remaining row as the intentionally unpublished control; and distinguished three `hibernated -> active` lifecycle changes from immutable-identity mismatches.

Two earlier panel rounds returned reproducible minority `repair` verdicts. The final candidate repaired every blocker before acceptance:

1. aggregate-only mutable evidence became one durable artifact containing all 149 execution receipt bodies, all 150 final join rows, canonical serialization rules, and supporting product/rollback/storage outputs;
2. the packet gained a proposed terminal receipt and exact registry deltas;
3. both evidence files were committed and pushed to `origin/main`;
4. orphan pre-durable digests were replaced by recomputable canonical durable digests;
5. the terminal Definition state, ACTIVE text, graph node, authority-manifest entry, and successor non-activation became explicit;
6. exact deployed CLI/API/browser product results were added; and
7. the incident record distinguished the revoked `ak_...` key identifier from the unrecorded `choir_sk_...` bearer secret.

## Independent Reproduction

The panel recomputed:

- execution receipts: `149`, unique route slots `149`, canonical SHA-256 `2a25c4946a3400c912e188b75739a2cd86d30ced89a0114b976424e6ffc5dde4`;
- post-restart joins: `150`, unique route slots `150`, published `149`, unpublished control `1`, errors `0`, canonical SHA-256 `1864529a4503a42d25a93578bdb7aa16f4843fc687ff367658f97131b8b81f07`;
- supporting outputs SHA-256: `97d4e8a942ab3f9202e79fc7dc6db949a63171e3c8c1d2a900673daca0912fbf`;
- receipt-to-join matches: `149/149` on route slot, `ComputerVersion`, disk receipt, route receipt, publication, and realization identity;
- immutable identity: exactly `CodeRef + ArtifactProgramRef`; disk receipt, backend, device, geometry, allocation, and recovery remain separate realization evidence.

## Incident Adjudication

- **Nix profile generations 555–558:** accepted as a bounded `introduced` heresy. The action was outside Definition authority and occurred after storage pressure was relieved. Current/rollback generations were preserved and no store GC ran. It does not falsify product completion and must remain visible.
- **CLI help credential leak:** accepted as repaired. The exposed admin credential was rotated and revoked; the source now keeps flag defaults empty and resolves environment fallback after parsing; regression tests and deployed no-SSH acceptance passed. The recorded `ak_257c3cae-877d-4d1a-ad21-155d2b7b56b5` is a revoked database identifier, not a bearer secret.

## Authorized Closure

The owner-authorized Definition makes an accepted G5 receipt the evidence gate for the terminal receipt, `now.status: complete`, and coherent registry closure; the panel supplies verification, not mutation authority. The audited Definition becomes historical evidence authority and is no longer a `/goal` entrypoint. The performance/capsule successor remains blocked and non-executable until separately owner-ratified and promoted through every registry.
