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


## Accepted execution, product readback, and rollback

Only after the accepted adjudication was committed at `main@808121a2`, vmctl applied the exact signed candidate.

### A → B promotion

- Apply envelope: `/tmp/choir-control-promotion-apply-i.json`, SHA-256 `88a22d9cdb410d2df6037687ab617b79b48ea00e14f83b4b7f6303424718d6fb`.
- Resolution: `/tmp/choir-control-promotion-resolution-i.json`, SHA-256 `1673ce619fc453b42662437793d0a943220f2ac6725950d9a1c40a2f82704b55`.
- Receipt: `f1df7f6f-31df-46da-83b8-ffd5d9a78e40`, transition `promote`, expected generation 1, committed generation 2, exact A → B, certificate `certificate:sha256:397f1a34edc655a71dac4092742403c9b7e8259f440ff522f56f80809b80dfa2`.
- Independent readback: `/tmp/choir-control-route-promoted-i.json`, SHA-256 `d5d7542521de152a04ed3622a15024d2511f06eaf4349c67e5ce07dc9a6a199e`, showed generation 2 B.
- Supported vmctl lookup selected `candidate-control-20260717-i` (`/tmp/choir-control-lookup-promoted-i.json`, SHA-256 `758082a584c06c633c2d8193ee7966d87235fbc8c53ab3c4ecf1a914310a928a`). Authenticated guest product readback returned `audit-b.txt` at 33 bytes (`/tmp/choir-control-routed-files-promoted-i.json`, SHA-256 `1047fe302a717b677c28e49241aad2a5c97e1416f5038237a9c0551df8610d7b`) and health `ready`.

### B disposal and B → A rollback

B was hibernated and exactly disposed while generation 2 still selected B. Receipt `/tmp/choir-control-dispose-routed-i-receipt.json`, SHA-256 `c9390d9d4cf1e7f8ad604cee0d1e646dee3cee7f18bcff22f18b76c273cb40b9`, binds route, generation 2, promotion receipt, B version, realization, and disk receipt and proves route preservation.

- Frozen rollback envelope: `/tmp/choir-control-rollback-apply-i.json`, SHA-256 `9479795cbe61663851d7b7c39ddb01dd9a3175d4ec626129a22aa863d7a5c08c`.
- Resolution: `/tmp/choir-control-rollback-resolution-i.json`, SHA-256 `f370108e708fb17716520425395efca414510e0d8b86e8b90039f8f9cffc0391`.
- Receipt: `8ae4f21a-14fd-4800-b044-76edef9604e7`, transition `rollback`, expected generation 2, committed generation 3, exact B → A, rollback target receipt `c3490ed2-287f-4b9f-a3c3-85c5055a50a0`.
- Fresh A reconstruction `candidate-control-20260717-j`: `/tmp/choir-control-construction-j.json`, SHA-256 `666cd64c42739b4c8eb02b546cb6141cd0725400564458569551e2458110707e`; fresh disk receipt `disk-instantiation:sha256:b3dfc5d43fb314b990e3645ded7a1804a51c0dba3906349ee95e2835e2300e8e`; healthy epoch 1; equivalence `equivalent`.
- Independent route readback `/tmp/choir-control-route-rolled-back-j.json`, SHA-256 `8ae7196f1dc421d82e25fcf1d2a5965332e94301bda5f27a14fd31145db8b677`, showed generation 3 A and exact prior-receipt lineage.
- Supported lookup selected fresh J (`/tmp/choir-control-lookup-rolled-back-j.json`, SHA-256 `78167646a817c95de85bc0e327806f6765d3a1b02506749dc8f11dce6bed5275`). Authenticated product readback contained A's `audit.txt` and not B's `audit-b.txt` (`/tmp/choir-control-routed-files-rolled-back-j.json`, SHA-256 `f5fb45b99f2767d44f542d8ee3e381f2a0ee6ca24a07ce09ab6b5b9ca65ad9be`) and health was `ready`.
- Replaying the stale generation-1 promotion envelope after generation 3 returned HTTP 409; route bytes remained SHA-256 `8ae7196f1dc421d82e25fcf1d2a5965332e94301bda5f27a14fd31145db8b677`. Refusal body: `/tmp/choir-control-stale-promotion-replay-i.json`, SHA-256 `2a8e0621ac3239e817f65ff73e72fe256ab1edfa9156ff34c9f74b6855ad5126`.

Result: the distinct immutable route transition and rollback contract is accepted and executed end to end on the authorized synthetic route. No real-user, platform, or fleet route changed.
