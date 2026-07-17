# G3 Frozen Bootstrap E Review Packet — 2026-07-17

## Gate and mutation boundary

- Mutation class: red.
- Governing gate: `G3-frozen-verifier-promotion-route-candidate` in `docs/definitions/choir-audited-autoputer-construction-2026-07-15.md`.
- Route scope: synthetic control route `computer:autoputer-control:control-20260716` only.
- Source/base ref: canonical `main` and deployed vmctl artifact `e3de55581a1cae3ecce1431f5f4440ab01f62fc8`.
- Protected surfaces: independent verification, signed owner approval, promotion certificate, signed G3 acceptance, immutable `ComputerVersion`, atomic route/evidence CAS, and durable transition receipt.
- Prohibition: this packet records a frozen, non-executed candidate. It is not G3 acceptance, a signature, or route-transition authority.

## Deployment and predecessor disposition

- GitHub Actions CI run `29550365185` succeeded, including normal and race shards, vet/build, SBOM generation, and Node B staging deployment.
- Node B activation receipt target and selected vmctl artifact both equal `e3de55581a1cae3ecce1431f5f4440ab01f62fc8`; vmctl is active.
- The predecessor `candidate-control-20260716-d` was disposed only through POST `/internal/vmctl/computer-version-realizations/dispose-unrouted` after exact route resolution returned HTTP 404.
- Disposal receipt: route `computer:autoputer-control:control-20260716`; realization `candidate-control-20260716-d`; ComputerVersion `(code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380, artifact-program:sha256:c106eb2c6dd72097e27754ba28ae9cb32bd962adca63fe973ebb906ac3ce824d)`; disk receipt `disk-instantiation:sha256:c96eded70f4c7cffe5d700744b18fc3c6ee96d6d59e43393b138716b51da2666`; `prior_state=stopped`; `route_absent=true`; disposed at `2026-07-17T03:08:32.175867123Z`.
- Disposal postconditions: no ownership record, VM state directory, Firecracker process, or route exists for `candidate-control-20260716-d`.

## Frozen realization and evidence identity

- Fresh realization: `candidate-control-20260717-e`; owner `autoputer-control`; desktop/candidate `control-20260716`; epoch `1`.
- Immutable ComputerVersion:
  - CodeRef `code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`.
  - ArtifactProgramRef `artifact-program:sha256:c106eb2c6dd72097e27754ba28ae9cb32bd962adca63fe973ebb906ac3ce824d`.
- Construction result canonical SHA-256: `dbf3a5e3c9b739a712daf4a6430884f049e762b3d9b050329442ed7e694da564`.
- Disk receipt: `disk-instantiation:sha256:b82e13473ef63f15ae574c5d034de703fb623d7cd98f294c02f61a29d1c90741`; 32 GiB logical ext4 device/filesystem; 4096-byte blocks; 8,388,608 blocks; 10,612,736 allocated host bytes at construction.
- Independent runtime geometry: filesystem bytes `33,501,757,440`; block size `4,096`; available bytes `33,341,689,856`.
- Observation SHA-256: `672fe5f30aa515bcc093d5052dd898d7b549ea545ebf5c3dee1e8ba3c579815c`.
- Independent verification receipt: `verification:sha256:36eb7d0e5623d3d55541a914d5a697b2e8949dee0d9f444d3321e86a233a4dfd`; verified at `2026-07-17T03:13:34.405968804Z`.
- Owner approval evidence: `approval:sha256:d10626065d73eef10e306640e25577455a572cd5310f7f48607e54b724db04ec`; approved at `2026-07-17T03:13:02.5618Z`; exact route, owner, ComputerVersion, and construction digest bound by Ed25519 signature under key ID `owner-control-route-20260716`.
- Promotion certificate: `certificate:sha256:d3694d74d1d6ec47b7f01843a4f700278cc20969cabdfbc80aa404873b2d5023`.
- Frozen bootstrap candidate: `route-bootstrap:sha256:eac7ab973a643c250e825700fd18348af6c4ba43b5912440a2d4172d0834d70d`; prepared at `2026-07-17T03:13:34.406101781Z`.
- Bootstrap plan: transition kind `bootstrap`; empty old ComputerVersion; exact new ComputerVersion above; expected generation `0`; certificate ref above; idempotency key `idempotency:bootstrap:36eb7d0e5623d3d55541a914d5a697b2e8949dee0d9f444d3321e86a233a4dfd`.

## Frozen file digests

These files remain non-authoritative transport copies; their hashes bind the exact bytes reviewed. No private signing key material is present in them.

- `/tmp/choir-control-construction-e.json`: SHA-256 `f549d853b7033c9457f59e7ce1235efac14b68b6850f64ff3eb6425582288842`.
- `/tmp/choir-control-bootstrap-prepare-request-e.json`: SHA-256 `bb3f2871e2db3b1f08b42828e98f6237c5ba947b72d55148c83bccc1879045a0`.
- `/tmp/choir-control-bootstrap-candidate-e.json`: SHA-256 `5f7c4e9ba41cc38ae8aea03abb85dc8d037198997942f7b43e008ef9d4c796cd`.

## Non-execution proof

- After candidate preparation, GET `/internal/vmctl/computer-version-routes/resolve?route_slot_id=computer%3Aautoputer-control%3Acontrol-20260716` returned HTTP `404` with `route ledger: slot not found`.
- No G3 acceptance exists for this candidate.
- No authorization evidence, route/evidence CAS, transition receipt, promotion, or rollback has been executed for bootstrap E.

## Initial G3 review and governing repair

- Review packet: `/tmp/choir-g3-bootstrap-e-consensus`; frozen prompt SHA-256 may be recomputed from `/tmp/choir-g3-bootstrap-e-review-prompt.md` while retained.
- Codex, Cursor, and OMP Gemini 3.5 returned `ACCEPT`, no blocker, high confidence.
- OMP GPT-5.5 returned `REPAIR`, high confidence: the named durable Phase D evidence did not bind bootstrap E, its current deployment/disposal/route-absence facts, or the exact frozen file hashes. The reproducible minority blocker governs.
- OpenCode verified the three file hashes but could not read the external `/tmp` files and emitted no adjudication. Devin and OMP GLM 5.2 timed out without adjudication.
- Repair: this code-free durable packet binds the exact frozen candidate and pre-CAS facts. Re-run G3 against the committed packet before creating any G3 signature or executing any route CAS.

## Ceremony and recovery

- Conjecture delta: a structurally and cryptographically valid frozen candidate is not mutation authority until its deployment identity, predecessor disposition, exact byte hashes, route absence, and non-execution state are durably bound for independent review.
- Heresy delta: discovered `1` (prompt-only/ephemeral evidence presented at a protected gate); introduced `0`; repaired `1` in this durable packet, pending independent re-review.
- Rollback before CAS: discard the frozen candidate or regenerate it if any bound file, deployed identity, route state, realization state, signature, or timestamp changes.
- Residual after an accepted bootstrap CAS: the synthetic slot cannot return to absent. Recovery must reconstruct the same immutable ComputerVersion and later use only separately frozen, reviewed, and signed promote/rollback transitions. Never mutate or clone a failed `data.img`.

## Repaired G3 terminal adjudication

- Reviewed base: committed durable packet and Definition at `main@1e82fe48`; deployed constructor/vmctl remained `e3de55581a1cae3ecce1431f5f4440ab01f62fc8`; the three transport hashes remained exact.
- Review packet: `/tmp/choir-g3-bootstrap-e-consensus-repair`.
- Codex, Cursor, OMP GPT-5.5, and OMP Gemini 3.5 each returned `ACCEPT`, `Blocking findings: none`, and `Confidence: high` after independently inspecting the committed packet, frozen JSON, typed joins, signatures, and mutation/refusal paths.
- OpenCode emitted no usable adjudication because its external-directory policy refused the `/tmp` JSON. Devin and OMP GLM 5.2 timed out without adjudication. No completed reviewer reported a blocker.
- Adjudication: `accept`. The prior durable-evidence blocker is repaired. G3 authorizes a separately signed acceptance over candidate `route-bootstrap:sha256:eac7ab973a643c250e825700fd18348af6c4ba43b5912440a2d4172d0834d70d` and one bounded bootstrap CAS only while the exact route remains absent, the same realization remains healthy, all frozen hashes/bindings remain unchanged, and the deployed signed writer remains `e3de5558`.
- Main residual risk: a successful bootstrap creates generation 1 and the route cannot return to absent. The accepted residue is the durable synthetic audit route; recovery is immutable reconstruction and later separately accepted promote/rollback, never route-history deletion or mutable-image repair.
- Heresy delta for the evidence repair: discovered `1`; introduced `0`; repaired `1`; no route/evidence CAS occurred during review.
