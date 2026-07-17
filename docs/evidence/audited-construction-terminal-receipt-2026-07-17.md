# Audited Computer Construction — Terminal Receipt

**Status:** **complete**
**Accepted gate:** `G5-frozen-post-cutover-closure`
**Acceptance level:** promotion-level for audited `ComputerVersion` construction, fleet D-ROUTE cutover, rollback, reconstruction, and no-SSH product inspection. External-agent capsule development is excluded successor work.
**Mutation class:** **red mission**, **yellow terminal registry mutation**. The completed mission touched protected runtime surfaces; this receipt and registry closure change authority state but do not change deployed behavior.

## Frozen Identity and Deployment

- deployed source commit: `2bc1799f72ce437b35d4606a23d14e62b7239ac5`;
- source tree: `22e07c8a86bf547669170b3ae548a83771857b5a`;
- source archive SHA-256: `aab03be55c0baf1fd96ce9d60726be78b7ed016df981cd2f6497f9bd1e16235f`;
- durable G5 evidence commit: `015ab0151598cfee9601dc423fd39ee1bba87abe`;
- push CI: `29609807147`, success;
- forced deployed acceptance: `29610242297`, success;
- Node B activation: `2026-07-17T20:28:47Z`, exact deployed source `2bc1799f72ce437b35d4606a23d14e62b7239ac5`;
- final vmctl health: `ok`, `150` ownerships after restart.

## Accepted Product Result

The fleet contains 150 persisted computers and 150 constructed ownerships. There are 149 published served routes and one intentionally unpublished audited control. All 150 final `ComputerVersion`/route/ownership/disk joins are durable and independently recomputable; all 149 serialized fleet execution receipts match their final joins with zero immutable-identity mismatch.

Owner `5bd6de97-3b58-408c-bf89-c42c81b083de`, desktop `primary`, is active, published, and ready after final vmctl restart:

- `ComputerVersion.CodeRef`: `code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`;
- `ComputerVersion.ArtifactProgramRef`: `artifact-program:sha256:9d90c8666a1d9a69f46daca644bb9470505831bb9926e21d2a577d0bd9aa5a6f`;
- disk receipt: `disk-instantiation:sha256:af1ac24e778ce9abfdbb5ceb0c9f26f628d26fc86ab80c7763a11cdfc4c97e2e`;
- D-ROUTE generation: `1`;
- route receipt: `d9dcef02-7cf1-51f7-972d-84921914adad`;
- approval: `approval:sha256:c2b96c781bbce6a7d4c00656462af6a0317f46a42dcd8b79f40c9be00e5fa89d`;
- promotion certificate: `certificate:sha256:ad5deb98712158a24a3019f02b2ebc71b5038b81a9d38a9fe2a123ff363eae75`.

Deployed product proof used supported authenticated surfaces, not SSH substitution:

- `CHOIR_API_KEY=<temporary-read-only-secret> go run ./cmd/choir computer status` exited `0`, returned status `ok`, `immutable_identity.joined=true`, active/published current computer, and ready runtime;
- `GET /api/shell/bootstrap` returned HTTP 200 and selected `candidate-fleet-e15cb89f25d963c220319b7b`;
- `GET /api/files` and `GET /api/files/.choir/source-lineage.json` returned HTTP 200 and exact owner/computer lineage;
- owner-authenticated `choir.news` browser proof returned online desktop, bootstrap/files/audit marker after restart;
- the temporary read-only key received HTTP 403 for `choir computer stop`, was revoked, and reuse was refused.

## Accepted Trajectory, Run, and Acceptance Identifiers

This fleet-construction mission did not use Choir trajectory IDs or run-acceptance IDs; those fields are **not applicable** rather than omitted. Its accepted execution identities are the 149 row-level fleet receipts and 150 route joins in the reproducibility artifact, owner route receipt `d9dcef02-7cf1-51f7-972d-84921914adad`, gate `G5-frozen-post-cutover-closure`, push CI run `29609807147`, and deployed acceptance run `29610242297`.

## Verifier Contracts and Evidence

- G5 packet: `docs/evidence/g5-post-cutover-closure-candidate-2026-07-17.json`, SHA-256 `fdec8720e5d786809c9605f1c8b28aa2a3b23037ad3bf74ed759b1722e21fa50`;
- row-level reproduction: `docs/evidence/g5-post-cutover-reproducibility-2026-07-17.json`, SHA-256 `23b8e439a512c3c10200798bf5fc8ef58caf450ffcd0a47c79ed3d0c1c0b4491`;
- accepted independent review: `docs/evidence/g5-post-cutover-accepted-review-2026-07-17.md`;
- execution receipt canonical digest: `2a25c4946a3400c912e188b75739a2cd86d30ced89a0114b976424e6ffc5dde4`;
- post-restart join canonical digest: `1864529a4503a42d25a93578bdb7aa16f4843fc687ff367658f97131b8b81f07`;
- receipt-to-join result: `149/149`, zero mismatch;
- semantic boundary: durable `ComputerVersion` is exactly `(CodeRef, ArtifactProgramRef)`; backend, device, filesystem geometry, allocation, disk receipt, corruption, deletion, pressure recovery, and reclaim are separate realization evidence;
- recovery contracts: deletion, corrupt-disk replacement without damaged-image reuse, pressure replacement/reclaim, owner zero-realization reconstruction, route rollback, disposal, legacy restore, and restart durability are indexed by the Definition.

## Rollback and Disposition

- 149 validated detached legacy rollback receipts are restart-safe and protected from pressure reclaim;
- system generations 559 and 560 remain retained rollback roots;
- two manual owner recovery snapshots remain preserved; deletion requires separate authority;
- failed candidates were disposed or rolled back as recorded;
- current fleet realizations remain accepted active/hibernated materializations;
- unrelated reclaim removed 100 orphan directories while projecting zero deletion of retained rollback state;
- post-reclaim vm-state allocation was 121.65 GiB and root availability was 237.49 GiB.

## Protected Surfaces

Texture canonical state, D-ROUTE authority, immutable `ComputerVersion` realizations, Trace/evidence, promotion/rollback, candidate computers, vmctl, owner authentication credentials, provider/product APIs, run acceptance, and deployment routing.

## Heresy Delta

**Discovered:** product status originally omitted immutable route/construction evidence; CLI help rendered an environment-backed API credential default.

**Introduced:** integration authority deleted stale Nix profile generations 555–558 outside Definition authority after capacity was already sufficient. Current and rollback generations were preserved; no store GC ran. This remains visible and is not relabeled as repaired.

**Repaired:** all 150 computer identities route over immutable `ComputerVersion`; served realizations join construction and D-ROUTE receipts; detached rollback state fate-shares with reclaim; authenticated product status exposes redacted immutable joins; CLI help no longer renders environment secrets; the exposed admin credential was rotated and revoked. The recorded `ak_...` value is a revoked key identifier, not the bearer secret.

## Conjecture Delta and Human Learning

**Confirmed:** `ComputerVersion` semantics are independent of disk representation; production can construct, verify, route, restart, roll back, destroy, and reconstruct sparse realizations fleet-wide; legacy rollback can fate-share with pressure reclaim; joined immutable identity is inspectable without SSH.

**Falsified:** a surviving mutable `data.img` is the computer; fleet closure can rely on aggregate-only mutable receipts; environment-backed secret flag defaults are display-safe.

The durable computer is the immutable `ComputerVersion` plus authoritative route and accepted materializer receipts. Disk images are replaceable realizations. Closure evidence must retain row-level receipts, not only aggregate counts and digests. CLI help metadata is an output surface and must never contain environment secrets.

## Residual Risks and Next Realism Axis

Non-blocking residuals: 149 detached legacy rollback directories need separately authorized disposition after retention TTL; two manual owner snapshots remain preserved; secondary exposure of the revoked admin credential is unknown; one control route remains intentionally unpublished; test-account cleanup and stale-root cleanup remain outside this authority.

The next realism axis is a separately owner-ratified Definition for external-agent capsule development through the supported Choir CLI. It remains blocked and non-executable until its deployed baseline, numerical SLOs, authority, rollback, and registry promotion are independently settled.
