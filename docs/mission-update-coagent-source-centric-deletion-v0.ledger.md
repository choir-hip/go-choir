# Ledger: update_coagent Source-Centric Deletion v0

## 2026-06-22 - Pass 0 - Paradoc Creation

Claim: a dedicated Parallax mission is needed for the source-centric
`update_coagent` deletion because the parent mission
(`mission-texture-structured-document-transclusion-cutover-v0`) raised the D9
target in Pass 43 but the actual deletion work is a distinct product: delete
legacy surfaces and migrate/quarantine data so the source-centric contract is
the only live path. The parent's domain ramp (D0-D7) targeted the structured
document substrate; this mission targets the coagent update envelope and the
legacy paths that still run alongside it.

Move: construct. Applied the cognitive-transform portfolio before authoring.
Inversion reclassified the work from "what to delete" to "what must survive"
(small survivor set → mechanical deletion once pinned). Root-cause 5-whys on
the v3 stall showed the stall is upstream of the render failures; render
fixes against a stuck v3 are product-irrelevant. Steelman of Codex's
deletion-reticence split it into a correct data instinct (don't strand
accounts) and a wrong code-path instinct (keep divergence alive); the
synthesis is unconditional code-path deletion with one-time data
migration/quarantine as the legitimate outlet. Pre-mortem produced five
sequencing constraints (deploy D9 first; migrate before removing
reconstruction; accept on existing account not synthetic; diagnose stall
before deleting render code; contract test against shim reintroduction).
Bottleneck named the v3 stall as the binding constraint. Second-order paired
each deletion with a write-time guard or caller audit. Each transform changed
the route, not the wording.

Expected delta V: initialize the mission at 12 open obligations (E0 stall
diagnosis; E1 survivor contract test; E2 data migration/quarantine; E3.1-E3.5
five deletion families; E4 P1/P2 validation gates counted as three; E5 staging
acceptance).

Actual delta V: initialized. No execution obligations discharged in this pass.

Receipt: `docs/mission-update-coagent-source-centric-deletion-v0.md`.

Open edges:
- E0 must name the actual v3 stall cause among three candidates
  (silently-dropped old-shape packet; non-`execution_request` packet stuck in
  persistent Super mailbox; plain-text fallback marking a revision as
  already-structured). Sequencing of E3 is provisional until E0 reports.
- E2 audit may reveal non-deterministically-convertible rows that must be
  quarantined, leaving some historical documents without native sources. This
  is acceptable but must be documented.
- Local D9 commits (`be52b194`, `c35502b2`) are not deployed; Node B is at
  `63f44e07`. Deletion must either deploy D9 first or fold the deploy into E5.

## 2026-06-22 - Pass 1 - E0 Stall Diagnosis

Claim: name the actual root cause of the v3 stall on `yusefnathanson@me.com`
doc `08fa6a2f-...`, and distinguish the owner's two hypotheses: H_deploy (VM
running pre-D9 code that accepts legacy `findings`) vs H_code (deployed code
is current but the source-centric contract is itself broken).

Move: probe + observer shift. Three parallel read-only code traces of the
revision-loop wiring first produced a code-only hypothesis (mutation-lifecycle
asymmetry). The owner then offered two further instruments that shifted the
observer: (a) ssh access to node-b to spelunk the owner's Dolt, and (b) the
explicit H_deploy vs H_code fork. The shift was decisive: code-only reasoning
could not distinguish deploy state from code state, but live evidence could.

Probe path on node-b (`choiros-b`, root):
- Platform Dolt (`/var/lib/go-choir/platform-dolt/platform`): zero rows for
  this owner — it mirrors only published artifacts, not live runtime state.
- Host runtime Dolt workspaces (`runtime.texture/texture`, `runtime.vtext`):
  empty — per-user runtime lives inside each owner's VM.
- Host platformd/sandbox service logs: zero texture-agent lines — confirms the
  runtime executes inside the VM, not on the host.
- VM mapping: owner `5bd6de97-...` → firecracker VM
  `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19` (running, epoch 659, data.img 17GB,
  runtime API at `10.200.233.2:8085`).
- VM runtime API queried read-only with `X-Authenticated-User` header (the
  header the gateway normally injects post-passkey; host can set it directly).
  Endpoints: `/api/texture/documents`,
  `/api/texture/documents/08fa6a2f-.../diagnosis`,
  `/api/texture/documents/08fa6a2f-.../revisions`.

Expected delta V: -1 for naming the root cause.

Actual delta V: -1. Current V=11. Root cause is H_deploy.

Verdict: **H_deploy.** Four converging proofs that the VM is running pre-D9
code (`63f44e07`), not the local D9 HEAD (`c35502b2`):

1. Message string divergence. Live consumed worker_update `content_preview`
   begins `"Coagent update ready.\nRole: researcher.\nKind: findings."`.
   `git show 63f44e07:internal/runtime/tools_worker_update.go:256` emits
   exactly that from `update.Kind`. Local D9 (`be52b194` rewrite, line 280)
   emits `"Coagent source packet ready."` from `packet.SchemaVersion`/`Kind`.
   The live Dolt carries the deployed string, not the local string.
2. Packet Kind vocabulary. Live packets have `Kind: findings`. D9
   `validCoagentPacketKind` (`tools_worker_update.go:568-570`) allows only
   `evidence_update`, `execution_request`, `execution_result`, `blocker`,
   `question`, `proposal`, `decision_request` — `findings` would be rejected.
   Deployed `63f44e07` accepts arbitrary `in.Kind`.
3. Runtime type. Deployed `buildWorkerUpdateMessage` takes
   `types.WorkerUpdateRecord`; D9 takes `types.CoagentSourcePacket`.
4. Direct run state. Texture run for trajectory `9424f974-...`:
   `state=passivated`, `reason=idle_deadline`,
   `payload={"attempt":3,"continue":false,"reason":"idle_deadline"}`,
   `actor_sleep_state=idle`. Researcher/Super/conductor runs all
   `state=completed`. Zero occurrences of `"sources"` in the entire 210KB
   diagnosis JSON. `worker_updates_consumed` shows seq 1 and 2, both
   `Kind: findings` from researcher `846311c1-...`.

Live causal chain (all four owner symptoms explained under one cause):
researcher emits legacy `Kind: findings` with no typed `packet.sources` →
Texture consumes the findings prose, writes v3 (the brief), has nothing to
transclude, parks waiting for a further deliverable packet, idle-deadline-
passivates at the 120s mark → v3 head stuck, "Revising..." flag still truthy.
The visible-markdown and process-metadata-as-first-paragraph symptoms are
downstream of v3 having been written under pre-D9 Texture prompt + the
plain-text fallback (`texture_structured_revision.go:130`); they are real
defects but not the stall.

Code-only hypothesis corrected. The initial code-only trace named the
mutation-lifecycle asymmetry (`runtime.go:2818` deferral +
`store/texture.go:2007` non-reactivatable + `texture_controller.go:116`
debounce-instead-of-start) as the likely mechanism. The live run did not
defer — it wrote v3 and passivated on idle_deadline — so that hypothesis is
demoted to a latent defect worth a later pass, not the present stall. Lesson
retained: code-only reasoning cannot distinguish H_deploy from H_code; live
runtime evidence is required to settle deploy-state questions.

Ruled out as the stall cause (still real defects):
- Silent validation drop: validation failures return to the agent
  (`tools_worker_update.go:147`); loop does not hang.
- Non-execution Super mailbox residue: `request_super_execution` is
  fire-and-forget; Texture gates only on `texture:<docID>`; pinned left-pending
  by `update_coagent_source_packet_test.go:169-175` but does not gate Texture.
- Plain-text markdown fallback: downstream render defect, not the stall.
- Mutation-lifecycle asymmetry: latent, not the live mechanism.

D9 code/prompts confirmed coherent. The H_deploy verdict raised a secondary
question: after D9 deploys, will the loop advance with sources? Yes. D9
researcher defaults (`prompt_defaults/researcher.yaml:36-37,42`) and runtime
overlay (`runtimeprompts/overlays/researcher_runtime.yaml:19`) instruct
`coagent_source_packet.v1` with `packet.sources` and forbid legacy fields.
D9 Texture prompts (`textureprompts/overlays/revision_worker_findings.yaml:9`,
`run_system.yaml:30`, `revision_policy.yaml:34`) instruct metabolizing
`packet.sources` and keeping process metadata out of body. D9 validation
rejects legacy fields (`tools_worker_update.go:746-755`) and unknown packet
kinds (`tools_worker_update.go:529-531`). The D9 path has simply never run on
staging.

Notable side-discoveries:
- The silent source-materialization skip at `texture_evidence_sources.go:163-170`
  must become loud under E1/E3 (a source that fails to materialize is silently
  `continue`d). Under D9 this becomes more important, not less.
- `scanWorkerUpdate` reconstruction (`store.go:2688-2697`) only affects
  pre-cutover historical rows: the only `INSERT INTO worker_updates`
  (`store.go:2389`) always writes `packet_json`. Legacy-shape rows cannot
  reach live delivery for new writes. The reconstruction is data-migration
  debt, not a live-path bug.
- `research_findings` is a dead live path: written only by tests, read only by
  the trace UI. Deletion is cleanup of the dead schema/type/trace exposure.

Receipt: `docs/mission-update-coagent-source-centric-deletion-v0.md` §"E0 Stall
Diagnosis - 2026-06-22". Code citations cross-referenced against both deployed
`63f44e07` and local HEAD `c35502b2` via `git show`. Live evidence from the
VM runtime API on node-b.

Open edges:
- The D9 deploy itself must land before any deletion proof is meaningful.
  Deploying `be52b194` + `c35502b2` is the cheapest probe of the source-centric
  contract on real data and should, under H_deploy, make the researcher emit
  `packet.sources` and the loop advance past v3. E5's deploy therefore moves
  to the front of *observable product proof*, while E1-E4 harden the cutover
  so a future stale-VM/stale-build scenario cannot silently fall back.
- E1 survivor contract must still include a loop-advances assertion and a
  rejected-sources-are-loud assertion; these do not depend on H_deploy.
- E2 data migration still required for existing accounts (legacy findings-
  shaped rows, raw-markdown revisions, queued non-execution Super packets).

Receipt: `docs/mission-update-coagent-source-centric-deletion-v0.md`
§"E0 Stall Diagnosis - 2026-06-22". Code citations verified against the files.

Open edges:
- H1 vs H2 distinction pending the in-VM Dolt read at E5. Does not block the
  deletion plan: both hypotheses share the same root cause (partial cutover)
  and the same deletion imperative, and the survivor contract (E1) must include
  a loop-advances assertion under either.
- E1 must include: loop advances past v3 on a researcher-bearing prompt-bar
  submission on the existing account; rejected sources are reported, not
  swallowed.
- E3.2 (Super backlog settlement) must land so non-execution packets stop
  accumulating as reconciliation noise.
