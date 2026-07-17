# G3 Distinct-Version Promotion B Frozen Review Packet — 2026-07-17

## Gate and mutation boundary

- Mutation class: **red**.
- Governing gate: `G3-frozen-verifier-promotion-route-candidate`.
- Authorized scope: synthetic route `computer:autoputer-control:control-20260716` only.
- Frozen packet state: **not executed**. No promote or rollback CAS has run.
- Builder base: `main@ccff94ddb1aceacd6e4cc2877e5859a87fe15dc4`, equal to `origin/main` before this packet.
- Protected surfaces: immutable ComputerVersion inputs, independent realization verification, signed owner approval, promotion certificate, SQL route CAS, rollback receipt lineage.
- Rollback: apply only the frozen rollback transition after a successful generation-2 promotion; if promotion is refused, leave generation 1 unchanged and dispose the unpublished B realization through exact routed disposal after review.
- Heresy delta: `discovered: 0`, `introduced: 0`, `repaired: 0`.
- Conjecture delta: this packet tests whether the accepted route contract changes between two actually distinct immutable versions and reverses by prior receipt, rather than merely replaying one version.

## Current route A

The read-only resolution `/tmp/choir-control-route-before-promotion-i.json` (SHA-256 `da48b2a16366faf4b8e945ccd88b40a774bc7c0da229d20babbffb9facce58f7`) proves:

- generation: `1`;
- latest receipt: `c3490ed2-287f-4b9f-a3c3-85c5055a50a0`;
- current CodeRef: `code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`;
- current ArtifactProgramRef: `artifact-program:sha256:c106eb2c6dd72097e27754ba28ae9cb32bd962adca63fe973ebb906ac3ce824d`.

The prior A realization `candidate-control-20260717-h` was exactly disposed before constructing B because ownership is keyed by owner/desktop and only one unpublished candidate may hold that identity. The disposal receipt is `/tmp/choir-control-dispose-routed-h-receipt.json`, SHA-256 `59774c51a7b56e0b257d4df00370b753fc11634b99d774e506b9e0318574b2bd`; it preserved the route and immutable A state.

## Distinct immutable version B

- CodeRef: `code:sha256:f47a6a0f295f186157f008f0ccb5999de254b274042fa876f47f00daa36579ff`.
- ArtifactProgramRef: `artifact-program:sha256:c349329330a83abfdd22f6726b0e0dc7e3c7f30fe8dffa9c43d70faa04c15d5a`.
- Semantic file: `audit-b.txt`, 33 bytes, blob `sha256:6d3a97017872e7abf9a7456b320f018578f8ef0ee43360de998e7e63668f22c2`.
- Input records were pinned through vmctl's immutable catalog; the blob was installed under the catalog's two-level content-addressed layout and hash-checked.

Construction `candidate-control-20260717-i` is frozen in `/tmp/choir-control-construction-i.json`, SHA-256 `ee63449c7facbc813ad0d3d6fd61c4191498899cffc6f5ef59d827fe5bdf4a9e`:

- exact B identity joined across construction, disk, boot, and observations;
- disk receipt `disk-instantiation:sha256:d47e5ce54d2c3520be1526652a2250be00cad0e942d1d87fe2de89e75037c0a6`;
- fresh 32 GiB ext4 realization, 10,596,352 allocated bytes;
- healthy epoch 1 at `http://10.200.3.2:8085`;
- materializer equivalence `equivalent`.

Direct authenticated product readback lists `audit-b.txt` at 33 bytes (`/tmp/choir-control-files-i.json`, SHA-256 `1047fe302a717b677c28e49241aad2a5c97e1416f5038237a9c0551df8610d7b`). Health is `ready` (`/tmp/choir-control-health-i.json`, SHA-256 `1f9369e025d491f9390f84bbcdded4c5871b8a49279ffc0743f11e007e55abbe`).

## Independent verification and frozen transition

The signed prepare request is `/tmp/choir-control-prepare-promotion-i.json`, SHA-256 `d9dd6c8c779fcc8188774254c6e9dd012601fb3646479ddc9a241d368019006b`. The owner approval is limited to this route, owner, B version, and construction SHA-256 `895ff42d66f5201f7d560f8baaf5dc8961d5a6ecfae1ec36bc53c9a68e96d78f`.

The vmctl-prepared frozen candidate is `/tmp/choir-control-promotion-candidate-i.json`, SHA-256 `9e3fd4d6c5a6277d661f80a0fe8a18ae3250f0474919d192af93ed2b8db5a67e`:

- candidate: `route-promotion:sha256:193c793ea0a4ba3f42bd7e70f8946401560682d81d0e83c2bfbb6e973bb63f3b`;
- verifier: `independent-production-realization-verifier`;
- verification: `verification:sha256:0587e697cb0bdd27234a84c2eb1235238d6e332f2f0cd6f762d2436da1fc7c21`;
- independent observation SHA-256: `6fabf39d6bba182ff8f113dade70a1febe2b30fb0cdc0551459b1b1ed2b58809`;
- approval evidence: `approval:sha256:d29d163ca507cb90fefb38460533d0c04dc526f8666d9960c7b5b0cd0fdeedf9`;
- promotion certificate: `promotion:sha256:0587e697cb0bdd27234a84c2eb1235238d6e332f2f0cd6f762d2436da1fc7c21`;
- pinned certificate evidence: `certificate:sha256:397f1a34edc655a71dac4092742403c9b7e8259f440ff522f56f80809b80dfa2`;
- promote precondition: exact generation `1`, latest receipt `c3490ed2-287f-4b9f-a3c3-85c5055a50a0`, A → B;
- rollback precondition: exact generation `2`, B → A, rollback target receipt `c3490ed2-287f-4b9f-a3c3-85c5055a50a0`.

## Reviewer obligation

Recompute the durable JSON hashes; validate exact route/version/identity/disk/verification/approval/certificate joins; verify stale generation, stale receipt, wrong route, wrong owner, forged signature, and replay refusal from source/tests; confirm current route remains generation 1; and adjudicate `ACCEPT`, `REPAIR`, `REJECT`, or `ESCALATE`. A reproducible minority blocker governs. Review must not sign, publish, pin new evidence, call apply-promotion, or execute any route transition.


## Independent adjudication — accepted

Panel output: `/tmp/choir-g3-promotion-b-consensus`.

- Codex: `ACCEPT`, no blockers, high confidence.
- Cursor: `ACCEPT`, no blockers, high confidence.
- OMP GPT-5.5: `ACCEPT`, no blockers, high confidence.
- OMP Gemini 3.5: `ACCEPT`, no blockers, high confidence.
- OpenCode exited successfully but could not access `/tmp` and emitted no adjudication; it is not counted.
- Devin and OMP GLM 5.2 timed out without verdicts.

All four usable adjudications independently recomputed the seven frozen hashes and accepted the typed A/B, construction, verification, approval, certificate, generation, prior-receipt, refusal, and rollback joins. No minority blocker exists. G3 therefore authorizes applying only the exact frozen promotion candidate. Route state remained generation 1 A throughout review; no CAS was executed by the panel.
