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
- Host corpusd/sandbox service logs: zero texture-agent lines — confirms the
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
  `packet.sources` and the loop settle cleanly without reproducing the old v3
  idle-deadline/revising stall. E5's deploy therefore moves to the front of
  *observable product proof*, while E1-E4 harden the cutover so a future
  stale-VM/stale-build scenario cannot silently fall back.
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

## 2026-06-22 - Pass 2 - E1 Survivor Contract Test

Claim: the survivor contract can be pinned as a test before any deletion, so
every later deletion commit has a green gate to keep green.

Move: construct. Wrote internal/runtime/update_coagent_survivor_contract_test.go
with six focused sub-tests covering the full survivor surface: canonical
coagent_source_packet.v1 accepted; all 9 legacy top-level fields rejected;
unknown top-level field rejected (closed surface); Texture collates ONLY
packet.sources (prose scraping blocked from notes/summary/claims.text); Super
privilege gate rejects non-execution packets from both the update filter and
the run deliverable filter; and a t.Skip marker for the E3.3 "rejected sources
are reported" obligation with the unblock condition named in the skip text.

Expected delta V: -1 for the survivor contract obligation.

Actual delta V: -1. Current V=10. 5 tests green, 1 skipped (E3.3 marker).
Full coagent test set green (TestUpdateCoagent*, TestPersistentSuper*,
TestRequestSuperExecution*, TestSurvivorContract_*). go vet, gofmt,
git diff --check clean.

Notable design decisions:
- The "Texture collates ONLY packet.sources" test deliberately embeds
  source-shaped text in notes (http URL), summary ([Source: foo] label), and
  claims.text (bare command_output: URI), then asserts exactly ONE entity is
  produced (the typed packet.source) and none of the prose-only references
  leak. This is the core invariant that makes the H_deploy failure mode
  impossible once the D9 code runs: even if a future stale prompt tells the
  researcher to write source prose, Texture will not scrape it.
- The E3.3 marker is t.Skip, not a stub failing test, because the loud-
  rejection surface is a real behavior change (E3.3) that must land with its
  own implementation. A failing test would block CI unrelated to the
  behavior change. The skip text names the exact unblock condition so E3.3
  cannot silently leave it skipped.
- The Super gate test constructs a persistent-Super-shaped RunRecord
  directly (rather than relying on reconcilePersistentSuperActor) so it
  exercises the coagentUpdateDeliverableForRun filter in isolation,
  independent of the mailbox reconciliation path.

Receipt: commit 1a603e4d. Evidence:
nix develop -c go test ./internal/runtime -run 'TestSurvivorContract_' -count=1 -v
nix develop -c go test ./internal/runtime -run 'TestUpdateCoagent|TestPersistentSuper|TestRequestSuperExecution|TestSurvivorContract_' -count=1

Open edges:
- The "loop advances past v3" survivor obligation is not unit-testable in
  isolation; it requires the deployed D9 code + a real prompt-bar submission
  on the existing account (E5a). The unit-level survivor contract here pins
  the preconditions (sources are typed, Super gating is intact, prose is not
  scraped); E5a pins the outcome.
- E3.3 must remove the t.Skip on TestSurvivorContract_RejectedSourcesAreReported
  and make the assertion green, OR D9 validation must reject unsupported
  source kinds at the tool boundary (the test records that as an acceptable
  alternative).

## 2026-06-22 - Pass 3 - E2 Data Audit (Read-Only)

Claim: the existing-account data audit can be completed read-only via the VM
runtime API, and its results determine the migration/quarantine policy for
each legacy data family.

Move: probe. Enumerated all 25 docs owned by 5bd6de97-... via
/api/texture/documents, pulled /api/texture/documents/{id}/diagnosis for each
(5.6MB total), and aggregated counts across four legacy data families.

Expected delta V: -1 for the data audit obligation; the migration/quarantine
implementation remains open.

Actual delta V: -1. Current V=9. Audit complete; migration policy recorded.

Audit results:
- Legacy Kind: findings worker_updates: 34 occurrences across 11 of 25 docs.
  Account is ~3.8x legacy-shaped vs source-centric-shaped (9 "sources":[
  occurrences across 6 docs, concentrated in D8 probe docs). Expected under
  H_deploy.
- research_findings: ZERO references in 5.6MB of diagnosis data. Confirms
  the live write path has already cut over; table is dead schema for this
  account. E3.1 deletion is pure code/schema cleanup.
- Raw-markdown texture_revisions: all 25 docs have markdown in revision
  content; 515 heading patterns total. The visible-markdown symptom at scale.
- Queued non-execution Super packets: not directly countable via product API
  (mailbox table not exposed), but the stall is systemic — all 25 docs have
  exactly 2 idle_deadline passivations. 08fa6a2f-... is representative, not
  exceptional. Account run totals: 388 completed, 161 passivated, 50
  idle_deadline.

Migration/quarantine policy (per family):
- Family A (legacy findings worker_updates): quarantine as audit-only
  historical rows. Do NOT fabricate typed sources from prose — that would
  violate E1's TextureCollatesOnlyPacketSources. E3.1's scanWorkerUpdate
  removal makes legacy rows fail live delivery reads, enforcing quarantine.
- Family B (research_findings): no migration needed (zero rows). E3.1 deletes
  write path + type.
- Family C (raw-markdown revisions): historical revisions stay historical
  (audit facts). After E5a deploys D9, next prompt-bar revision produces a
  structured head. plainTextStructuredTextureDoc fallback (E3.3) is the
  mechanism that currently fakes-structured from markdown; deleting it
  prevents new acquisitions of the defect.
- Family D (queued non-execution Super packets): E3.2 settles them, does not
  leave pending. The survivor contract test already encodes the target.

Key finding for E5a: the systemic 25/25 stall means the acceptance probe can
run on any doc, but E5a is the first time this account sees the source-centric
contract in production. Synthetic-fixture D9 tests did not exercise
existing-account data; watch for H_code bugs pre-mortem failure mode C did
not cover.

Scratch files (/tmp/audit, /tmp/diag.json, /tmp/docs.json, /tmp/traj.json on
node-b) cleaned up; no scratch data left on the host.

Receipt: docs/mission-update-coagent-source-centric-deletion-v0.md §"E2 Data
Audit - 2026-06-22". Evidence: 25 diagnosis JSON pulls from the VM runtime
API (transient, aggregated into the counts above).

Open edges:
- The migration/quarantine is not a separate data-write pass; it is encoded
  as policy enforced by E3.1 (scanWorkerUpdate removal) and E3.2 (Super
  settlement). No standalone data-migration script is needed for this
  account under H_deploy, because deploying D9 + deleting the
  reconstruction/settlement paths enforces the quarantine structurally.
- E5a must run on the existing account and watch for H_code bugs the
  synthetic D9 tests missed.

## 2026-06-22 - Pass 4 - E3.2 Super Backlog Settlement

Claim: non-`execution_request` packets addressed to persistent Super can be
settled as explicit non-executable receipts without weakening the privilege
gate, and the settlement must also handle historical/migration mailboxes where
a non-execution row precedes a valid execution request.

Move: construct. Repaired `reconcilePersistentSuperActor` to list the
persistent Super mailbox through a bounded settlement loop before filtering for
executable work. Each pass marks non-executable packets delivered to
`settled_non_executable`, refreshes the mailbox cursor, and re-lists. After the
backlog converges, only validated `kind=execution_request` packets can start a
persistent Super run.

Expected delta V: -1 for E3.2 Super backlog settlement.

Actual delta V: -1. Current V=8. E3.2 is locally repaired. Non-execution
Super packets neither start privileged execution nor remain live pending
backlog. Mixed historical mailboxes no longer let an earlier non-execution row
block a later executable row.

Evidence:
- `nix develop -c go test ./internal/runtime -run 'TestPersistentSuperIgnoresNonExecutionRequestUpdatePackets|TestSurvivorContract_Super(SettlesNonExecutionRequestPackets|SettlesNonExecutionBeforeExecutionBacklog|ExecutesOnlyExecutionRequestPackets)' -count=1 -v`
- `nix develop -c go test ./internal/runtime -run 'TestUpdateCoagent|TestPersistentSuper|TestRequestSuperExecution|TestSurvivorContract_' -count=1`
- `git diff --check`

Notable implementation detail: new `update_coagent` writes addressed to
persistent Super already wake reconciliation synchronously, so the new-write
test must assert the durable `settled_non_executable` receipt after tool
execution rather than expect an intermediate visible backlog row. A separate
store-seeded mixed-backlog test covers the existing-account/migration case.

Receipt: `internal/runtime/super_controller.go`,
`internal/runtime/update_coagent_source_packet_test.go`, and
`internal/runtime/update_coagent_survivor_contract_test.go`.

Open edges:
- E3.1 research_findings/test-endpoint deletion remains open. The interrupted
  Agy WIP attempted to rename a legacy test endpoint to
  `/api/test/texture/worker-update`; that was not kept in this pass because it
  mixes endpoint deletion with a live test route rename.
- E3.3 rejected-source reporting remains skipped in the survivor contract.
- No staging deploy or existing-account product proof was attempted in this
  pass.

## 2026-06-22 - Pass 5 - E3.2 Reviewer P1 Checkpoint

Claim: the Pass 4 E3.2 local repair still has a convergence defect when an
executable packet precedes a non-execution packet in the persistent Super
mailbox.

Move: document before repair. The reviewer observed that `ListCoagentMailboxBacklog`
is cursor-based, not `delivered_at`-based. If seq 1 is an undelivered
`execution_request` and seq 2 is a later `evidence_update`, settling seq 2
does not advance the cursor past seq 1. The next backlog read can still return
the delivered seq 2 row. Because delivered rows fail
`persistentSuperExecutableUpdate`, the settlement helper can classify the
already-settled row as non-executable again; `MarkWorkerUpdatesDelivered`
then no-ops due to `delivered_at IS NULL`, and the bounded settlement loop can
return `mailbox did not converge` before starting the seq 1 executable run.

Expected delta V: none yet. This is the Problem Documentation First checkpoint
for the reviewer P1; the repair follows in the next commit.

Actual delta V: 0. Current V remains 8.

Required repair:
- collect only currently undelivered/unsettled rows before classifying
  non-executable packets for settlement;
- add a survivor test where `execution_request` precedes `evidence_update` in
  the persistent Super mailbox;
- prove the executable Super run starts and the later non-execution packet is
  settled.

Heresy delta: `discovered` for the executable-before-non-execution convergence
case. No code repair is included in this checkpoint.

## 2026-06-22 - Pass 6 - E3.2 Reviewer P1 Repair

Claim: the E3.2 convergence defect is repaired if the settlement helper ignores
already-settled rows returned by the cursor backlog query before classifying
non-executable packets.

Move: construct. Updated `settlePersistentSuperNonExecutionUpdates` to collect
only rows with `DeliveredAt == nil` and empty `DeliveredToRunID`; added
`TestSurvivorContract_SuperExecutesBeforeSettledNonExecutionBacklog` for the
reviewer case where seq 1 is executable and seq 2 is non-execution.

Expected delta V: restores the E3.2 repair claim from Pass 4 after the P1.

Actual delta V: E3.2 remains repaired; current V=8.

Evidence:
- `git diff --check`
- `nix develop -c go test ./internal/runtime -run 'TestPersistentSuperIgnoresNonExecutionRequestUpdatePackets|TestSurvivorContract_Super(SettlesNonExecutionRequestPackets|SettlesNonExecutionBeforeExecutionBacklog|ExecutesOnlyExecutionRequestPackets|ExecutesBeforeSettledNonExecutionBacklog)' -count=1 -v`
- `nix develop -c go test ./internal/runtime -run 'TestUpdateCoagent|TestPersistentSuper|TestRequestSuperExecution|TestSurvivorContract_' -count=1`

Heresy delta: `repaired` for the executable-before-non-execution convergence
case. No staging deploy or existing-account product proof was attempted.

## 2026-06-22 - Pass 7 - E5 Acceptance Semantics Adjudication

Claim: the literal "advance past v3" phrase in the E5 text was a proxy for the
old v3 idle-deadline/revising stall, not a requirement to force additional
Texture revisions after a source-backed structured head has already settled.

Move: adjudicate + green checkpoint. Re-read the paradoc's owner direction,
E0 diagnosis, E5 acceptance text, and the hard-cutover deletion inventory. E0
shows why `> v3` was chosen: the old deployed path produced legacy
`Kind: findings` updates with no typed `packet.sources`, wrote a v3 markdown
head, showed "Revising...", and idle-deadline-passivated without a further
deliverable packet. The version threshold was a symptom discriminator for that
specific failure. It should not override the deeper source-centric settlement
condition once a fresh deployed proof cleanly settles earlier.

Expected delta V: clarify E5a acceptance so the mission can accept a clean
earlier completion/passivation without weakening canonical-source gates.

Actual delta V: E5a source/stall proof accepted under corrected semantics. The
full deletion mission still carries any remaining E3/E4 hard-cutover receipts
and post-deletion re-confirmation obligations not settled by this adjudication.

Evidence reviewed:
- deployed commit `fc620961e0be07a7dbab41aca3843ef396b2a512`;
- prompt token `E5_RETRY_SOURCE_TEXTURE_PROOF_20260622`;
- owner `5bd6de97-3b58-408c-bf89-c42c81b083de`
  (`yusefnathanson@me.com`);
- doc `6f7114c7-72ab-4b68-8e73-b5b687a2bc09`, current revision
  `2b854664-9081-4d22-ab34-acbcb061fb32`, current version number `2`,
  revision count `3`;
- trajectory `32e2169d-d7f2-4e59-91dd-c7fb4e3494b7`, Texture loop
  `ae1d6f94-0f5f-42aa-b546-631db76eae26`, researcher loop
  `27320029-8d3c-44b9-b571-f36912b44cfe`;
- final trajectory state `passivated`, `live=false`;
- final Texture loop state `passivated`;
- head `body_doc.schema=choir.texture_doc.v1`;
- three structured `source_ref` nodes and three top-level `source_entities`;
- no `metadata.source_entities` legacy sidecar;
- reader-facing first paragraph;
- consumed worker update seq 1 with `Schema: coagent_source_packet.v1` /
  `Kind: evidence_update`;
- trace `update_coagent` args with `schema_version=coagent_source_packet.v1`,
  `kind=evidence_update`, and canonical sources `s1`, `s2`, `s3`;
- `worker_updates_pending` empty; and
- no Super agent or `execution_request` path for the trajectory.

Decision: accept the deployed proof for the E5a stall/source criterion. The
corrected criterion is "must not reproduce the old v3 idle-deadline stall":
clean completion/passivation before v3 is acceptable when canonical
`coagent_source_packet.v1` researcher updates, native source nodes, structured
head, empty pending updates, and product/runtime read evidence all show clean
settlement. Version number greater than 3 is not independently required and
should not be forced.

Evidence boundary: the callback did not record a passivation/sleep reason for
the new run, `agent_revision_pending=false`, or signed-in UI toolbar/no-revising
proof. Final Comet extraction saw a signed-out preview, so this adjudication
does not claim independent visible UI proof; its clean-settlement claim rests on
the product/runtime read evidence above.

Heresy delta: `repaired` for the over-specific E5 version-number proxy.
`introduced`: none. `discovered`: the earlier acceptance wording could drive
counterproductive prompt design that manufactures revisions rather than proving
source-centric settlement.

Residual risks:
- This adjudication accepts the provided deployed read-only product/runtime
  evidence; it did not create a new prompt-bar mutation.
- Source-centric acceptance remains strict: future proofs must still show
  canonical source packets, native source nodes/entities, no legacy metadata
  source sidecar, no pending worker updates, and no privileged Super path for
  non-execution evidence updates.
- The broader hard-cutover mission may still need remaining deletion receipts
  and post-deletion re-confirmation; this checkpoint only removes the
  counterproductive `current_version_number > 3` requirement.

Receipt: `docs/mission-update-coagent-source-centric-deletion-v0.md`
§"E5 Acceptance Semantics Clarification - 2026-06-22".

## 2026-06-22 - Pass 8 - E5 Adjudication Evidence Boundary Repair

Claim: the E5 adjudication is ready for second review only if its evidence list
distinguishes captured runtime settlement from uncaptured UI no-revising proof
and if live Parallax wording no longer teaches future agents to optimize for a
literal `current_version_number > 3` threshold.

Move: green docs repair. Added the captured final state fields from the proof
callback (`trajectory state=passivated`, `live=false`; Texture loop
`state=passivated`) to the mission doc and this ledger. Explicitly recorded the
unknowns: no new-run passivation/sleep reason, no `agent_revision_pending=false`
field, and no signed-in toolbar/no-revising UI proof were captured. The final
Comet extraction saw a signed-out preview, so the clean-settlement claim rests
on product/runtime read evidence only.

Also updated the live Domain Ramp/Parallax witness language so "advance past
v3" remains an old-failure description, not the binding E5 acceptance target.
The source-centric gates are unchanged: canonical
`coagent_source_packet.v1` researcher updates, typed sources, native source
nodes/entities, no legacy metadata source sidecar, no pending worker updates,
and no privileged Super path for non-execution evidence updates.

Expected delta V: remove the docs evidence gap that caused
`revise_before_continue` without changing source-centric acceptance.

Actual delta V: docs-only checkpoint repaired; ready for second review.

Evidence:
- proof callback values already recorded in Pass 7 plus added final states:
  trajectory `passivated`/`live=false`; Texture loop `passivated`;
- explicit unknowns: passivation/sleep reason, `agent_revision_pending=false`,
  and visible signed-in no-revising UI proof.

Heresy delta: `repaired` for the over-specific version-number proxy and for
the evidence-boundary gap in the adjudication record. `introduced`: none.
`discovered`: empty pending worker updates alone is insufficient to distinguish
clean settlement from the old E0 stuck state because E0 also had
`worker_updates_pending=[]` while `agent_revision_pending` remained truthy.

## 2026-06-22 - Pass 9 - Manual QA Rendering And Source Blocker Checkpoint

Claim: the source-centric `update_coagent` cutover is not user-ready while a
fresh manual QA pass can produce a structured-source Texture that still renders
markdown syntax as prose, opens empty source-reader windows, and places
numbered source refs inside words.

Move: document first. Owner manual QA on staging around 2026-06-22 18:30-18:34
captured a fresh document with handle prefix `a98b2384-7...b3aec9`. The visible
revision flow advanced from an evidence-plan v1 into a source-bearing v2, but
the user-facing article body still showed literal `#`, `##`, `###`, and
markdown-list tokens instead of heading/list structure. In a later state, the
content collapsed into a single long paragraph with heading markers inline.
The same screenshots showed `Sources 14` and source titles such as
`Newsroom - Anthropic`, but opening the source displayed only source chrome and
`content item not found`. Inline source refs were inserted into words, including
examples equivalent to `s[1]hipping`, `enterpris[2]e`, `rele[3]ase-note`,
`clea[4]n`, and `O[5]penAI`.

Mutation class: green for this checkpoint. The intended repair is orange
because it may change Texture canonical BodyDoc construction, Texture patch
operations, frontend source rendering, or source entity fallback behavior.
Protected surfaces: Texture canonical writes/body_doc projection,
`patch_texture` source-ref insertion semantics, top-level `source_entities`,
source reader/viewer rendering, and source-centric coagent evidence projection.

Conjecture delta: a clean source-centric packet and native source identity are
necessary but not sufficient. The rendered artifact must also preserve document
structure, display source content or an honest source fallback, and place source
refs at readable boundaries. A source node that resolves to a missing content
item, or a source ref inserted mid-word, is not acceptable owner-visible proof.

Admissible evidence class for repair: focused Go or frontend tests that prove
markdown headings/lists no longer render as literal raw markdown in generated
Texture content; source-ref insertion normalizes unsafe offsets to readable
boundaries; source viewer/source panels display non-empty source snapshot,
summary, excerpt, or URL-backed fallback when no content item exists; and no
legacy source identity path is reintroduced. Product/staging proof is still the
acceptance environment after landing, but this local repair thread must not
push or deploy.

Rollback path: revert the follow-up code/test repair commit(s) while preserving
this problem checkpoint. Do not roll back to clickable markdown source links,
metadata `source_entities`, legacy `research_findings`, or generic markdown
source-link parsing as canonical source identity.

Heresy delta: `discovered` for the rendering/source-reader/ref-placement defects
in the source-centric path. `introduced`: none in this checkpoint. `repaired`:
none until a later code/test commit.

## 2026-06-22 - Pass 10 - Source Text Preservation Correction Checkpoint

Claim: the Pass 9 repair is incomplete if URL-backed sources degrade to
title/URL-only fallback even when the researcher already read source content.
`web_url` is the correct target identity for URL-backed sources, but it is not a
license to drop bounded source text from the Texture source/transclusion
substrate.

Move: document first. Owner correction after the manual QA landing review:
the expected behavior is that URLs can back native source entities while still
showing source content in the small transcluded inline/body stub and fuller
source viewer. The source viewer may honestly show title/URL fallback only when
no bounded source text or reader snapshot is present. When `update_coagent`
delivers researcher-read text, the runtime must preserve it as native source
entity content rather than treating the source as link-only chrome.

Mutation class: green for this checkpoint. The intended repair is red because
it changes the canonical source packet/source entity contract and Texture
source display semantics. Protected surfaces: `update_coagent` packet schema,
source entity materialization, structured `source_entities`, source viewer
reader snapshots, inline source/transclusion excerpts, and publication export
of source metadata.

Conjecture delta: a source-centric Texture system must preserve source content
as a source artifact, not merely source identity. Rightsholder-positive display
means users see bounded source material in context and can open the source
viewer for more/full available text, while original URLs remain available as an
escape hatch. A title/URL-only fallback is acceptable only as an explicit
absence-of-content state.

Admissible evidence class for repair: focused tests that prove canonical
`update_coagent` packet sources can carry bounded source text; URL-backed
source entities retain `web_url` identity while gaining `transclusion`/
`reader_snapshot` content in stored structured source entities; frontend inline
transclusions and Source Viewer prefer that content over title/URL fallback; and
legacy markdown links, metadata `source_entities`, or synthetic `content_item`
IDs are not reintroduced.

Rollback path: revert the follow-up code/test repair commit(s) while preserving
this checkpoint. Do not roll back to clickable markdown links or synthetic
content-item IDs for URL-backed sources.

Heresy delta: `discovered` for the link-only interpretation of URL-backed
sources. `introduced`: none in this checkpoint. `repaired`: none until a later
code/test commit.

## 2026-06-22 - Pass 11 - Imported Source Text Preservation Gap

Claim: the Pass 10 source-text repair is incomplete while imported source
content can still re-enter Texture through `content_id:*` refs as title-only
or URL-only source chrome. Researcher-read text preserved directly in
`update_coagent` packets now works, but imported content items must also carry
their stored text into native source transclusions and the Source Viewer.

Move: document first. A fresh deployed proof on commit
`3fc60e1b37e5170dcd7c532ce21d64bcfd1b0735` used prompt token
`SOURCE_TEXT_PRESERVATION_PROOF_20260622_2334` and created Texture document
`f8c29bde-0fa6-45cf-8430-3ec0bbade95d`. The proof showed the intended
positive path: head v5 had structured `choir.texture_doc.v1`, no
`metadata.source_entities`, seven top-level source entities, four URL-backed
entities with preserved `reader_snapshot.text_content`, and visible embedded
source excerpt blocks in the Texture body. It also exposed the remaining gap:
three imported/fallback entities showed `content_id:*` or `file_artifact:*`
labels and no reader snapshot even though imported source text is available in
the content substrate. The run also showed repeated researcher
`update_coagent` tool errors for string-valued `sources` entries before runtime
fallback salvaged a canonical packet.

Mutation class: green for this checkpoint. The intended repair is red because
it changes Texture source entity materialization and source text display for
imported content items. Protected surfaces: Texture structured
`source_entities`, content-item source projection, coagent source fallback
normalization, Source Viewer/inline transclusion content, and the
source-centric `update_coagent` contract.

Conjecture delta: preserving source content cannot depend on whether the
researcher delivered text directly in `sources[].reader_snapshot` or indirectly
through an imported `ContentItem`. `content_id:*` is an internal pointer and
must not become a user-visible source title; it should hydrate the native source
entity from the stored content item title, URL, and text snapshot.

Admissible evidence class for repair: tests proving `content_id:*` source refs
materialize as native content-item sources with the content item title,
canonical URL, and `reader_snapshot.text_content`; packet/fallback titles that
look like internal typed refs do not overwrite hydrated source titles; Texture
source embeds prefer stored source text over title/URL fallback; and no
metadata source sidecars, clickable markdown links, or synthetic content-item
IDs are reintroduced.

Rollback path: revert the follow-up code/test repair commit(s) while preserving
this checkpoint. Do not roll back to title/URL-only source display as the
target behavior.

Heresy delta: `discovered` for the imported content-item source text gap and
internal typed-ref title leak. `introduced`: none in this checkpoint.
`repaired`: none until a later code/test commit.

## 2026-06-23 - Pass 12 - Structured Patch Misuse: Markdown Collapse And Source Ref Bunching

Claim: the source-centric pipeline is still not owner-readable while
`patch_texture` permits whole-document markdown to be written through a single
`update_block_text` paragraph and then permits multiple native `source_ref`
operations at offset `0`. This creates a formally structured revision whose
visible document is still a collapsed markdown blob and whose citations are
bunched at the beginning rather than distributed next to the claims they
support.

Move: document first. The owner created a new staging account `d@a.com`, mapped
on Node B to owner id `45ea050f-9824-4614-8bf0-02e595136a69` and active VM
`vm-1f73393c7a5b70a315cac61be0e79a1e` at `http://10.200.235.2:8085`.
The prompt `whats new in music this week?` created Texture document
`eb3ac586-ad4a-48a4-af9c-da5d50bb5e69`, current head
`342a4f9b-1d6e-42c0-a2ea-ced4a498d170`, version `v3`. Product APIs showed the
trajectory `cb5d740a-5f29-4227-b6b6-5b0f5df9a370` was `passivated` and
`live=false`; the Texture run `7ae64428-de7b-4f2c-b370-b76b7dca93b9`
passivated at `2026-06-23T00:00:45Z` with `passivated_reason=idle_deadline`
and `actor_sleep_state=idle`. The observed "Revising..." stall is therefore a
presentation/lifecycle boundary problem around actor park/passivation, not a
live worker still running.

The same v3 head showed the structural content defect. The stored `body_doc`
had one paragraph, zero heading/list blocks, nine `source_ref` nodes, and zero
`source_embed` nodes. The Trace event for `patch_texture` shows Texture called
`update_block_text` with a complete markdown article beginning
`# What’s new in music this week` and then called four `insert_source_ref`
operations against the same block with `offset: 0`. V2 had the same shape with
five `source_ref` nodes at the start of a single paragraph. This explains both
literal markdown tokens in the reader surface and citation points bunched at
the beginning of the Texture body.

Mutation class: green for this checkpoint. The intended repair is red because
it changes Texture canonical write-tool validation and visible revision
structure. Protected surfaces: `patch_texture`, `rewrite_texture`, structured
`body_doc`, native `source_ref` placement, Texture stream/passivation status,
and source-centric source display.

Conjecture delta: having native source entities is not enough. The write API
must make the correct shape the easiest path and reject or normalize tool calls
that would collapse document structure or detach citations from the claims they
support. A sleeping durable Texture actor is valid, but the owner-facing editor
must not present a parked/passivated actor as actively revising.

Admissible evidence class for repair: focused tests proving whole-document
markdown is parsed into structured heading/list/paragraph blocks when using a
whole-document rewrite path; `update_block_text` rejects markdown-document
payloads instead of storing them as one paragraph; `insert_source_ref` rejects
offset-zero placement into non-empty text; duplicate source refs cannot be
stacked at the same block/offset; passivated Texture stream events clear the
frontend revising state; and no markdown source links, metadata
`source_entities`, or legacy citation sidecars are reintroduced.

Rollback path: revert the follow-up code/test repair commit(s) while preserving
this checkpoint. Do not roll back to clickable markdown links, source sidecars,
or title/URL-only source display.

Heresy delta: `discovered` for collapsed markdown document writes, offset-zero
source-ref bunching, and parked Texture actor status being experienced as a
stall. `introduced`: none in this checkpoint. `repaired`: none until a later
code/test commit.
