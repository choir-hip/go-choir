# Mission: update_coagent Source-Centric Deletion v0

## Summary

The source-centric `update_coagent` cutover was added *alongside* the legacy
`worker_updates` / `research_findings` / markdown-source surfaces rather than
*instead of* them. That partial cutover is the root cause of the live staging
failure: a packet, revision, or render call can still take the old shape and
hit an incompatibility the new code did not expect. This mission's product is
deletion and streamlining, not addition. We delete the legacy surfaces and
migrate/quarantine the data so the source-centric contract is the only live
path, and we prove it on staging against the existing
`yusefnathanson@me.com` account.

This is a successor/specialization of
[mission-texture-structured-document-transclusion-cutover-v0.md](./mission-texture-structured-document-transclusion-cutover-v0.md)
(Pass 43 / D9 raised the source-centric `update_coagent` target) and consumes
the deletion inventory in
[report-update-coagent-hard-cutover-legacy-deletion-2026-06-22.md](./report-update-coagent-hard-cutover-legacy-deletion-2026-06-22.md).

## Source Documents And Prior Art

- [report-update-coagent-hard-cutover-legacy-deletion-2026-06-22.md](./report-update-coagent-hard-cutover-legacy-deletion-2026-06-22.md)
  — the deletion inventory, P1/P2 review findings, existing-account cutover
  requirements, acceptance criteria, and manual QA re-confirm.
- [mission-texture-structured-document-transclusion-cutover-v0.md](./mission-texture-structured-document-transclusion-cutover-v0.md)
  — parent paradoc; Pass 43 raised the D9 source-centric target.
- [texture-agentic-invariants-2026-06-13.md](./texture-agentic-invariants-2026-06-13.md)
  — Texture invariants that must hold after deletion.
- [memo-problem-documentation-first.md](./memo-problem-documentation-first.md)
  — the discipline this mission follows: document, then delete.

## Owner Direction

- This is before launch; do a hard cutover. No live runtime compatibility for
  old update shapes.
- Deletion is the fix, not cleanup. The staging failures are symptoms of a
  partial cutover; removing the divergent paths is the repair.
- Codex authored the D9 cutover work and is deletion-reticent: it adds the
  canonical path and repairs validation gates but leaves the old surfaces live.
  Every retained old surface is a divergence point. Treat any proposal to keep
  a code path alive "for compatibility" as a conjecture to weaken, not a safe
  default.
- Remove old ways first. Bring back only what is proven required, and only as a
  one-time data migration/quarantine, never as a live runtime path.
- Sources must appear on every researcher update, not only on some revisions.
- The revision loop must advance. The observed v3 stall ("Revising..." for ~a
  minute, then stop with no v4) is a wiring defect, not a render bug, and it is
  the highest-signal acceptance target.

## Problem

The owner's manual QA on 2026-06-22 against the authenticated
`yusefnathanson@me.com` session on `choir.news` re-confirmed four failures on a
fresh prompt-bar submission:

1. Markdown rendering is broken in the document body.
2. No sources appear, even though every researcher update should carry
   `packet.sources`.
3. The first paragraph is still irrelevant process/status metadata rather than
   reader-facing content.
4. **The document held at v3, showed "Revising..." for roughly a minute, then
   the revision loop stopped without producing a v4.**

The first three are downstream of the fourth. A revision loop that does not
advance cannot produce a corrected head, so render fixes against a stuck v3 are
product-irrelevant. The root cause is a partial cutover where the new
source-centric path and the old `worker_updates` / `research_findings` /
markdown-source paths coexist: a packet can be silently dropped by new
validation, stuck in persistent Super's mailbox, or turned into a
"already-structured" dead end by the plain-text fallback. Staging itself is
behind the local D9 work (Node B deployed `63f44e07`, predating the local D9
commits `be52b194` and `c35502b2`), so the visible failure is pre-D9 behavior;
but the local D9 work is not a hard cutover either, only a partially repaired
intermediate state.

## Replacement Architecture Target

The live contract after deletion (verbatim from the deletion report):

- `update_coagent` accepts only: `schema_version`, `kind`, `summary`,
  `claims`, `sources`, `actions`, `questions`, `notes`, plus addressing fields
  `agent_id` and `channel_id`.
- The runtime validates nested packet objects in Go, not only via JSON Schema
  metadata.
- Texture source collation reads only `packet.sources`.
- Texture canonical writes persist structured `body_doc` plus validated
  `source_entities`; source references are document nodes, not markdown links.
- Super execution starts only from validated `kind=execution_request` packets
  with executable actions.
- Command outputs, diffs, tests, screenshots, videos, artifacts, and app change
  packages are represented as `packet.sources`.
- User-facing Texture body text never includes checkpoint/status metadata
  unless the user explicitly asks for a process report document.

## Cognitive Transform Portfolio (admitted because each changes the next move)

These were applied before authoring. Each is recorded because it changed the
route, not the wording.

**Inversion — "what must survive?" instead of "what do we delete?"** The
survivor set is exactly the Replacement Architecture Target above. Everything
outside it is deletable without per-case judgment *once a contract test pins
the survivors*. This converts deletion from a deliberative, reticence-prone
activity into a mechanical one gated by a contract test. Route change: write
the survivor contract test first; deletion becomes "make the contract test the
only green path."

**Root-cause (5-whys on the v3 stall)** — markdown visible ← plain-text
fallback fired ← revision had no `body_doc` ← writer never produced structured
ops ← *the revision loop stalled at v3* ← the packet that should have driven
v4 was silently dropped or stuck in Super's mailbox. The root is the stall.
Route change: diagnose and delete the silent-drop path *before* any
render/fallback deletion. Render fixes against a stuck v3 are product-irrelevant.

**Steelman of Codex's deletion-reticence** — the reticence is *correct as a
data instinct* (strand existing accounts, lose renderability of old revisions)
and *wrong as a code-path instinct* (keep divergence alive). Synthesis: delete
code paths unconditionally; migrate/quarantine data once; never keep a code
path alive because data exists. Route change: separate "code-path deletion"
(always) from "data migration" (one-time, allowed), and give the reticence its
legitimate outlet in the data phase so it does not block the code phase.

**Pre-mortem** — five ways this fails, each reshaping sequencing:
- (A) D9 is not deployed; deleting against an un-deployed baseline strands the
  cutover. → Deploy D9 before, or as part of, deletion.
- (B) Deleting `scanWorkerUpdate` reconstruction breaks every account with
  empty `packet_json`. → Data migration/quarantine must precede the
  reconstruction removal.
- (C) Tests pass on synthetic accounts but `yusefnathanson@me.com` still fails
  because its data is old-shape. → Acceptance must run on the existing account,
  not synthetic users.
- (D) Render code is deleted but the stall persists because the dropped-packet
  path is untouched. → Stall diagnosis is the first move, not render deletion.
- (E) Codex re-introduces a "compat shim" and recreates the divergence. → A
  contract test must fail on any reintroduction; review for shims explicitly.

**Bottleneck** — the binding constraint is the v3 stall. No deletion of
render/fallback code matters until the loop advances with sources. Critical
path: diagnose stall → delete the silent-drop path → *then* delete
render/fallback/sidecar paths.

**Second-order** — deleting a live path strands its callers:
- Deleting `research_findings` as a live path strands any researcher flow still
  writing it → need a write-time guard that rejects new writes, not a table
  drop on day one.
- Deleting the plain-text fallback strands any `CreateRevision` caller still
  writing plain `content` → audit all callers before removing the fallback.
- Deleting Super backlog residue strands queued packets → existing-account scan
  + quarantine must precede the settlement cut.

Route change: each deletion commit must be paired with a write-time guard or a
caller audit, not performed as a bare removal.

## E0 Stall Diagnosis - 2026-06-22

Mutation class: `green` documentation checkpoint only. No runtime behavior,
schema, API, prompt, editor, publication, or source resolver code changes in
this checkpoint.

### What E0 set out to determine

Name the actual root cause of the observed v3 stall on the live
`yusefnathanson@me.com` account (doc `08fa6a2f-d886-412d-b2ac-83fe548a9ac4`,
current revision `dadcc214-de23-4404-b8ac-e17e436e383c`, v3), and distinguish
two hypotheses the owner raised:

- **H_deploy**: the VM is running pre-D9 code that still accepts the legacy
  `findings` shape, so the source-centric cutover has never actually run in
  production. Fix is "deploy D9," not "delete more."
- **H_code**: the deployed code is current but the source-centric contract is
  itself broken in a way that prevents the loop from advancing. Fix is code
  deletion/repair.

The pre-mortem's failure mode (A) ("D9 is not deployed; deleting against an
un-deployed baseline strands the cutover") is exactly H_deploy.

### Method

1. Three parallel read-only code traces of the revision-loop wiring
   (`internal/runtime/texture.go`, `texture_agent_revision.go`,
   `texture_controller.go`, `super_controller.go`, `runtime.go`, `toolloop.go`,
   `internal/store/texture.go`).
2. Read-only probe of node-b (`choiros-b`): the platform Dolt, the host runtime
   Dolt workspaces, platformd/sandbox service logs, and the running firecracker
   VM state for the owner's computer
   `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19`.
3. Read-only queries of the live VM runtime API at `10.200.233.2:8085` using
   the `X-Authenticated-User` header (owner-authorized; header is normally
   injected by the gateway after passkey auth, but the host can set it
   directly). Endpoints: `/api/texture/documents`, `/api/texture/documents/{id}`,
   `/api/texture/documents/{id}/diagnosis`, `/api/texture/documents/{id}/revisions`.
4. Cross-reference of the live packet/message strings against both deployed
   commit `63f44e07` and local HEAD `c35502b2` via `git show`.

### Finding: H_deploy is correct. The VM is running pre-D9 code.

Three independent lines of evidence converge on this conclusion.

**1. The stored worker-update message text matches deployed `63f44e07`, not
local D9.** The live diagnosis JSON for doc `08fa6a2f-...` shows the consumed
researcher worker_updates carrying `content_preview` beginning:

```text
Coagent update ready.
Role: researcher.
Kind: findings.
```

The current local `buildWorkerUpdateMessage`
(`internal/runtime/tools_worker_update.go:277-300`, rewritten in commit
`be52b194`) emits:

```text
Coagent source packet ready.
...
Schema: <packet.SchemaVersion>
Kind: <packet.Kind>
```

`git show 63f44e07:internal/runtime/tools_worker_update.go` shows the deployed
version's `buildWorkerUpdateMessage` (line 254) emitting exactly
`"Coagent update ready."` from `update.Kind`. The live Dolt string is the
deployed string, not the local string. The message-formatting code was
rewritten in D9 and has never run on staging.

**2. The packet `Kind` value is legacy.** The live consumed packets have
`Kind: findings`. The D9 `validCoagentPacketKind`
(`tools_worker_update.go:568-570`) allows only `evidence_update`,
`execution_request`, `execution_result`, `blocker`, `question`, `proposal`,
`decision_request`. It does not allow `findings`. Deployed `63f44e07` accepts
arbitrary `in.Kind` strings (its line 105 passes `strings.TrimSpace(in.Kind)`
straight through with no enum). So a packet with `Kind: findings` can only
have been produced and accepted by pre-D9 code.

**3. The runtime type is legacy.** Deployed `63f44e07`'s
`buildWorkerUpdateMessage` takes `types.WorkerUpdateRecord`; local D9's takes
`types.CoagentSourcePacket`. The live packets are `WorkerUpdateRecord`-shaped.

### Finding: the live causal chain (all four owner-reported symptoms explained)

From the diagnosis JSON for trajectory `9424f974-e38d-4e83-87d7-88b55a4276c3`
on doc `08fa6a2f-...`:

1. Conductor run: `state=completed`.
2. Researcher run: `state=completed`. It emitted two worker_updates
   (`worker_updates_consumed` seq 1 and 2, both `from_agent_id` researcher
   `846311c1-...`, both `Kind: findings`) with `content_preview` carrying
   legacy findings prose. **Zero occurrences of `"sources"` in the entire
   210KB diagnosis JSON** — no typed `packet.sources` was ever produced.
3. Super run: `state=completed`.
4. Texture run (`agent_id=texture:08fa6a2f-...`, `loop_id=0d41a6d0-...`):
   `state=passivated` with `reason=idle_deadline`,
   `payload={"attempt":3,"continue":false,"reason":"idle_deadline"}`.
   `actor_sleep_state=idle`, `actor_sleep_at=2026-06-22T16:08:17.98Z`.
   The 120s park-idle deadline (`cfg.TextureActorParkIdle` default) fired with
   no further deliverable packet.
5. Document head: `current_version_number=3`, `actor_sleep_state=idle`,
   `agent_revision_pending` still truthy from the sleep-state row.

This explains all four owner-reported symptoms under a single cause:

- **v3 stall / no v4**: the Texture run consumed the two findings-shaped
  packets, had no typed sources to metabolize, wrote what it could (v3, the
  reader-facing brief from findings prose), then parked waiting for a further
  deliverable packet that never came, and idle-deadline-passivated at the
  120s mark.
- **markdown rendered as visible text**: v3's body begins
  `# What's new in AI now?\n\nSnapshot as of 2026-06-22 16:03 UTC: ...` and
  contains `## The short version` and `**...**`. Under pre-D9 code, the
  `plainTextStructuredTextureDoc` fallback
  (`internal/store/texture_structured_revision.go:130`) wraps that raw
  markdown as paragraph text. (This is a real downstream defect, but it is
  not the stall.)
- **no sources / citations**: the researcher never produced typed
  `packet.sources`, only legacy findings prose; Texture had nothing to
  collate into source entities. `worker_updates_pending=[]`.
- **process metadata as first paragraph**: the "Snapshot as of ... UTC"
  preamble is exactly the process-status prose the D9 Texture prompts
  (`revision_policy.yaml`, `run_system.yaml`) now forbid in canonical body;
  the deployed pre-D9 Texture prompt did not forbid it.
- **"Revising..." stuck on**: `agent_revision_pending` is truthy as long as
  a non-terminal mutation/sleep row exists (`internal/runtime/texture.go:1020`
  → `GetPendingAgentMutationByDoc`), and the passivated run leaves it set.

### Finding: the local D9 code and prompts are coherent

The H_deploy conclusion raises a secondary question: even after D9 deploys,
will the loop advance with sources? Cross-reference says yes:

- The D9 researcher defaults (`internal/runtime/prompt_defaults/researcher.yaml`
  line 36-37, 42) instruct: "The canonical payload is
  `schema_version='coagent_source_packet.v1'`, kind, summary, claims, sources,
  actions..." and "do not send legacy update_id, findings, evidence_ids,
  evidence, artifacts, refs, tests..."
- The D9 researcher runtime overlay
  (`internal/runtime/runtimeprompts/overlays/researcher_runtime.yaml:19`)
  instructs: "include typed source substrate in update_coagent packet.sources
  and cite those source_id values from claims[]."
- The D9 Texture prompts
  (`internal/runtime/textureprompts/overlays/revision_worker_findings.yaml:9`,
  `run_system.yaml:30`, `revision_policy.yaml:34`) instruct Texture to
  metabolize `packet.sources` and to keep process metadata out of the body.
- The D9 validation rejects both legacy fields (`tools_worker_update.go:746-755`)
  and unknown packet kinds (`tools_worker_update.go:529-531`).

So the researcher, under D9, would emit `coagent_source_packet.v1` with typed
`sources[]`; Texture would metabolize them; and the loop would advance. The
D9 code path has simply never executed on staging.

### Findings ruled out (still real defects, not the stall)

For completeness, the three pre-mortem candidate wiring defects were checked
and are not the stall cause on this doc:

- **Silently-dropped packet at validation**: ruled out. Validation failures
  return to the agent as failed tool results (`tools_worker_update.go:147`),
  so the loop does not hang on them. (And under H_deploy the deployed code
  has no D9 validation to fail anyway.)
- **Non-`execution_request` packet stuck in persistent Super's mailbox**:
  ruled out as the stall cause. `request_super_execution` is fire-and-forget
  (`tools_texture.go:488-549`); Texture gates only on `texture:<docID>`
  (`texture_controller.go:109`), never on Super's backlog. The
  non-execution-packet backlog is real P2 queue residue (pinned by
  `update_coagent_source_packet_test.go:169-175`), but it does not gate any
  Texture decision.
- **Plain-text fallback**: real downstream render defect, not the stall. It
  explains the visible-markdown symptom but produces a valid structured doc
  and does not stop the loop.
- **Mutation-lifecycle asymmetry** (`runtime.go:2818` deferral +
  `store/texture.go:2007` non-reactivatable + `texture_controller.go:116`
  debounce-instead-of-start): a real latent defect worth a later pass, but
  not the mechanism on this doc. The live run did not defer; it wrote v3 and
  passivated on idle_deadline.

### Conjecture delta

E0 confirms the paradoc's root-cause transform in a sharper form than the
pre-mortem allowed: the v3 stall is upstream of the render failures, and the
root is **the partial cutover expressed as a deploy gap** — the source-centric
contract exists locally but has never run in production, so the production
researcher still speaks the legacy `findings` dialect that carries no typed
sources for Texture to transclude. The pre-mortem's failure mode (A) is not a
hypothetical risk; it is the actual present state.

This reframes the deletion plan. The deletion is still required (the legacy
surfaces in the deletion report are real and must be removed), but the
**first observable product fix is deploying the already-written D9 work**,
not new deletion. Deletion then hardens the cutover so H_deploy can never
recur by accident (no legacy path remains for a stale VM to fall back to).

### Heresy delta

- discovered: the live staging failure is H_deploy — the VM is running
  pre-D9 code (`63f44e07`), producing legacy `Kind: findings` packets with no
  typed `packet.sources`, which Texture cannot metabolize, so the loop
  idle-deadline-passivates at v3. Four converging proofs: message-string
  divergence, packet-Kind vocabulary divergence, runtime type divergence,
  and direct run-state evidence (`state=passivated`, `reason=idle_deadline`).
- discovered (latent, not the live stall): mutation-lifecycle asymmetry,
  silent source-materialization skip at `texture_evidence_sources.go:163-170`,
  Super non-execution backlog residue, plain-text markdown fallback.
- introduced: none by this documentation checkpoint.
- repaired: not yet. Discovery is not repair.

### Rollback path

This checkpoint is documentation only; no rollback needed. The follow-up code
fixes (E1-E5) carry their own rollback paths as each lands.

### What this changes about the domain ramp

The original ramp had E0 (diagnose) → E1 (contract) → E2 (data migration) →
E3-E4 (deletion) → E5 (deploy + accept). The diagnosis reshuffles the
*observable-fix* order without removing any step:

- **E5's deploy moves to the front of observable product proof.** Deploying
  the already-landed D9 commits (`be52b194`, `c35502b2`) is the first
  observable fix: under H_deploy, it alone should make the researcher emit
  `packet.sources` and the loop advance past v3. This is the cheapest probe
  of the source-centric contract on real data.
- **E1-E4 then harden the cutover.** The contract test (E1) pins what the
  deploy just proved; the data migration (E2) and code deletion (E3-E4) remove
  the legacy surfaces so a future stale-VM or stale-build scenario cannot
  silently fall back to the `findings` dialect.
- **The full landing loop (E5) is still required for settlement** — deploy
  alone, without deletion receipts and existing-account acceptance, does not
  close the mission.

## Domain Ramp

- **E0 Stall diagnosis (probe).** Reproduce the v3 stall locally or against
  staged trace evidence on the existing account. Identify which of the three
  candidate wiring defects (silently-dropped old-shape packet; non-
  `execution_request` packet stuck in Super mailbox; plain-text fallback
  marking a revision as already-structured) is the actual cause. No deletion in
  this pass. Output: a named cause with evidence.
- **E1 Survivor contract test (construct).** Pin the Replacement Architecture
  Target as a failing-then-green contract test: `update_coagent` rejects legacy
  top-level fields and invalid nested objects; Texture reads only
  `packet.sources`; Super executes only `kind=execution_request`; revisions
  carry `packet.sources` on every researcher update; the loop advances past v3.
  This test is the gate every later deletion commit must keep green.
- **E2 Data migration/quarantine (construct, one-time).** Per the deletion
  report's existing-account audit: count invalid/empty `packet_json`, old-shape
  `worker_updates`, live `research_findings`, raw-markdown `texture_revisions`,
  and queued non-execution Super packets. Convert deterministically or
  quarantine as audit-only. This is the legitimate outlet for deletion
  reticence.
- **E3 Code-path deletion (construct).** Delete in this order, each commit
  keeping E1 green:
  1. Storage compatibility shims: `scanWorkerUpdate` reconstruction
     (`store.go:2693`), live `research_findings` write path.
  2. Super queue settlement: non-`execution_request` packets addressed to
     persistent Super are rejected/quarantined, not left pending.
  3. Runtime prompt/tool legacy: `textureInlineSourceRefRE` (`texture.go:47`),
     `plainTextStructuredTextureDoc` fallback
     (`texture_structured_revision.go:130`), `coagentSourcesFromRefs` generic
     parsing.
  4. Frontend/publication legacy: clickable-link upgrading
     (`texture-source-renderer.ts:489`), markdown-as-canonical-body rendering.
  5. Test-only endpoints: `/api/test/texture/research-findings`.
- **E4 P1/P2 validation gates (construct).** Vocabulary-validate
  `packet.sources[].kind` and `selectors[].kind` against the source contract
  (P1); reconcile `target.uri` schema vs Go validation (P2); settle non-
  execution Super packets (P2).
- **E5 Deploy and staging acceptance (construct, red).** Deploy to Node B,
  confirm commit identity, run acceptance on `choir.news` with
  `yusefnathanson@me.com`. Verify loop advances past v3 with sources on each
  revision.

## Parallax State

status: working

mission conjecture: If the legacy `worker_updates`, `research_findings`,
markdown-source, and Super-backlog surfaces are deleted (with one-time data
migration/quarantine) so that the source-centric `update_coagent` contract is
the only live path, then a prompt-bar submission on the existing
`yusefnathanson@me.com` account will advance past v3 and each revision will
carry native `packet.sources` rendered as transclusions rather than stalling,
showing raw markdown, or omitting sources.

deeper goal (G): Texture is Choir's canonical artifact control plane, and the
source-centric `update_coagent` is the single envelope by which researcher and
execution coagents hand evidence into Texture. A partial cutover makes that
envelope untrustworthy; a hard cutover makes the whole product surface —
writing, research, source identity, publication — coherent.

witness/spec (A/S): A deployed staging state on Node B in which (a) the
survivor contract test is green; (b) `rg` for legacy markers returns only
migration/rejection code; (c) live mailbox reads fail on empty/invalid
`packet_json` rather than reconstructing; (d) a fresh prompt-bar submission on
`yusefnathanson@me.com` advances past v3 with `packet.sources` on each
researcher update; (e) markdown control tokens do not appear as visible prose;
(f) non-`execution_request` Super packets neither execute nor linger as
backlog.

invariants / qualities / domain ramp (I/Q/D):
- I1: no live runtime compatibility for old update shapes; one-time data
  migration/quarantine only.
- I2: code-path deletion is unconditional; data migration is the only
  legitimate outlet for deletion reticence.
- I3: every deletion commit keeps the E1 survivor contract test green.
- I4: no reintroduction of compat shims; a contract test fails on any
  reintroduction.
- I5: acceptance runs on the existing `yusefnathanson@me.com` account, not
  synthetic users.
- Q1: deletion is mechanical once the survivor contract is pinned; no
  per-case judgment to be re-litigated.
- Q2: the revision loop advancing past v3 with sources is the binding
  acceptance signal, not render correctness against a stuck revision.
- Domain ramp E0-E5 above.

variant (ranking function) V: open obligations:
1. v3 stall root cause unidentified (E0).
2. survivor contract test absent (E1).
3. existing-account data not migrated/quarantined (E2).
4. storage shims live (`scanWorkerUpdate` reconstruction, `research_findings`
   write path) (E3.1).
5. Super backlog settlement absent (E3.2).
6. runtime prompt/tool legacy live (`textureInlineSourceRefRE`,
   `plainTextStructuredTextureDoc`, `coagentSourcesFromRefs`) (E3.3).
7. frontend/publication legacy live (clickable-link upgrading,
   markdown-as-canonical-body) (E3.4).
8. test-only `/api/test/texture/research-findings` endpoint live (E3.5).
9. P1 source/selector vocabulary validation absent (E4).
10. P2 `target.uri` schema/Go disagreement (E4).
11. P2 Super non-execution packet settlement absent (E4).
12. staging deploy + existing-account acceptance not run (E5).

Current value: 11 open. E0 stall diagnosis discharged (Pass 1): root cause is
H_deploy — the VM is running pre-D9 code (`63f44e07`), confirmed by four
converging proofs (message-string divergence, packet-Kind vocabulary
divergence, runtime type divergence, and direct run-state evidence showing
`state=passivated`/`reason=idle_deadline` with zero `packet.sources` in the
live diagnosis). The researcher emits legacy `Kind: findings` with no typed
sources; Texture cannot metabolize it; the loop idle-deadline-passivates at
v3. The D9 code and prompts are coherent and would fix this if deployed.

budget: Planning budget is this paradoc pass. E0 is complete (read-only
diagnosis discharged; H_deploy established). E1 is a bounded construct (one
contract test file). E2 is a bounded one-time data pass. E3-E4 are bounded red
constructs, each one deletion family per commit, each keeping E1 green. E5 is
the landing loop. Solvency verdict after E0: the first *observable* product
fix is deploying the already-written D9 commits (`be52b194`, `c35502b2`) —
under H_deploy that alone should make the researcher emit `packet.sources` and
the loop advance past v3. E1-E4 then harden the cutover so a future
stale-VM/stale-build scenario cannot silently fall back to the legacy
`findings` dialect. Full landing loop (E5) still required for settlement.

authority / bounds: This paradoc authorizes documentation and planning only.
E0 is read-only. E1 is additive test code. E2-E5 are red (storage schema,
coagent delivery, Texture canonical writes, Super execution gating,
publication/export, frontend rendering, staging deploy) and require the full
landing loop per `AGENTS.md`. Protected surfaces: Texture canonical writes,
coagent update delivery, researcher/super update propagation, source
transclusion, publication export, Super execution gating, existing account
runtime data, and staging deploy routing.

mutation class / protected surfaces:
- E0: green (read-only diagnosis).
- E1: green/yellow (additive contract test).
- E2: red (existing-account runtime data migration/quarantine).
- E3.1: red (store schema, `scanWorkerUpdate`, `research_findings`).
- E3.2: red (Super mailbox/backlog settlement).
- E3.3: orange/red (runtime prompts/tools, `texture.go`,
  `texture_structured_revision.go`, `researcher_checkpoint_fallback.go`,
  `delegate_worker_update_fallback.go`).
- E3.4: orange (frontend renderer, publication/export).
- E3.5: orange (test-only API surface).
- E4: red (validation gates on `update_coagent`, Super settlement).
- E5: red (staging deploy + acceptance).

evidence packet:
- Deletion inventory and P1/P2 findings:
  `docs/report-update-coagent-hard-cutover-legacy-deletion-2026-06-22.md`.
- Parent mission Pass 42-43 (D9 source-centric target raised):
  `docs/mission-texture-structured-document-transclusion-cutover-v0.ledger.md`.
- Local D9 commits `be52b194` (source-centric shape) and `c35502b2` (validation
  gate repairs); focused tests passed locally but not deployed.
- Staging deployed at `63f44e07` (pre-D9); owner manual QA 2026-06-22
  re-confirms four failures including the v3 stall on
  `yusefnathanson@me.com` / owner id `5bd6de97-3b58-408c-bf89-c42c81b083de`;
  live doc `08fa6a2f-d886-412d-b2ac-83fe548a9ac4`, current revision
  `dadcc214-de23-4404-b8ac-e17e436e383c`, v3.
- Node B read-only inspection: deployed checkout `63f44e07`; schema still has
  both `research_findings` and `worker_updates`.

heresy delta:
- discovered: the partial cutover leaves multiple source-shaped code paths
  alive (`worker_updates` reconstruction, `research_findings` live path,
  markdown source links, plain-text fallback, Super backlog residue).
- introduced: none by this paradoc.
- repaired: not yet. Discovery (this paradoc, the deletion report) is not
  repair. Repair is claimed only after E3-E5 land and staging acceptance on the
  existing account passes.

position / live conjectures / open edges:
- C1 (inversion): the survivor set is small and already named; deletion is
  mechanical once pinned. Supported by the Replacement Architecture Target.
- C2 (root-cause): the v3 stall is the root; render fixes are downstream.
  Supported by the owner QA ordering (stall observed alongside render
  failures). To be confirmed by E0.
- C3 (reticence steelman): data migration is the legitimate outlet for
  deletion reticence; code-path deletion is unconditional. This dissolves the
  observed Codex failure mode.
- E1: the three candidate stall causes (silently-dropped packet; stuck Super
  mailbox; plain-text fallback dead-end) are hypotheses; E0 must name the
  actual cause before E3 sequencing is finalized.
- E2: existing-account data shape is partly unknown; the E2 audit may reveal
  rows that cannot be deterministically converted and must be quarantined,
  which could leave some historical documents without native sources. That is
  acceptable per the deletion report (historical revisions may remain
  historical) but must be documented.

next move: two parallel tracks.
(A) Observable product proof: deploy the already-written D9 commits
(`be52b194`, `c35502b2`) to Node B so the VM runs current code. Under H_deploy
this alone should make the researcher emit `packet.sources` and the loop
advance past v3 on a fresh prompt-bar submission on `yusefnathanson@me.com`.
This is the cheapest real-data probe of the source-centric contract and the
first half of E5.
(B) Hardening: E1 — pin the survivor contract as a contract test
(`update_coagent` rejects legacy top-level fields and invalid nested objects;
Texture reads only `packet.sources`; Super executes only
`kind=execution_request`; revisions carry `packet.sources` on every researcher
update; the loop advances past v3; rejected sources are reported, not silently
swallowed at `texture_evidence_sources.go:163-170`). Then E2 (one-time data
migration/quarantine), then E3-E4 deletion in families. Then the second half
of E5 (deletion-proof staging acceptance on the existing account).

ledger file: docs/mission-update-coagent-source-centric-deletion-v0.ledger.md

version / lineage: v0. Created 2026-06-22 as a successor/specialization of
`mission-texture-structured-document-transclusion-cutover-v0` (Pass 43 / D9
raised the source-centric target). Consumes the deletion inventory in
`report-update-coagent-hard-cutover-legacy-deletion-2026-06-22.md`. Sibling to
`mission-texture-hard-cutover-v0` (legacy name/ontology residue) — this mission
is scoped to the `update_coagent` source-centric code/data deletion only.

learning state: retained here. Promote outward only when the survivor contract
test, the deletion receipts, or the staging acceptance on the existing account
changes shared doctrine, assertions, or architecture.

settlement: Not met. Settlement requires:
- E0 names the v3 stall cause with evidence.
- E1 survivor contract test is green.
- E2 existing-account data is migrated or quarantined; the audit counts are
  recorded.
- E3-E4 deletion receipts land, each keeping E1 green, with `rg` for legacy
  markers returning only migration/rejection code.
- E5 staging on Node B deploys the post-deletion commit; a fresh prompt-bar
  submission on `yusefnathanson@me.com` advances past v3 with `packet.sources`
  on each researcher update; markdown control tokens are absent from visible
  prose; non-`execution_request` Super packets neither execute nor linger.
- No compat shim is reintroduced (C4 contract test green).

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-update-coagent-source-centric-deletion-v0.md.

Read the paradoc first. E0 is COMPLETE and committed (commit 36b0f591); do not
redo E0 or re-diagnose the v3 stall. The E0 conclusion is authoritative: the
v3 stall on yusefnathanson@me.com doc 08fa6a2f is H_deploy — the owner's VM
(vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19, runtime API 10.200.233.2:8085, reachable
from node-b with X-Authenticated-User header) is running pre-D9 code
(63f44e07). The researcher emits legacy Kind: findings with no typed
packet.sources; Texture cannot metabolize it; the loop idle-deadline-
passivates at v3. The D9 code and prompts (be52b194, c35502b2) are coherent
and would fix this if deployed. Current V=11.

Resume at E1, then proceed E2 -> E3 -> E4 -> E5. Keep the original ordering
discipline: NO render/fallback code deletion before E1 (the survivor contract)
is pinned green; NO compat shims reintroduced.

E1 — pin the survivor contract as a failing-then-green contract test. The
survivor set is exactly: update_coagent accepts ONLY the
coagent_source_packet.v1 surface (schema_version, kind, summary, claims,
sources, actions, questions, notes, agent_id, channel_id) and rejects legacy
top-level fields (findings, evidence_ids, evidence, artifacts, refs, tests,
proposals, capability_requests) and invalid nested objects. Texture reads
ONLY packet.sources. Super executes ONLY kind=execution_request packets.
Revisions carry packet.sources on every researcher update. The loop advances
past v3 on a researcher-bearing prompt-bar submission. Rejected sources are
REPORTED (not silently swallowed at internal/runtime/texture_evidence_sources.go:163-170).
Every later deletion commit must keep this test green.

E2 — one-time data migration/quarantine for the existing account
(5bd6de97-3b58-408c-bf89-c42c81b083de). Read-only audit first (count
invalid/empty packet_json worker_updates; old-shape research_findings;
raw-markdown texture_revisions; queued non-execution Super packets). Then
convert deterministically or quarantine as audit-only. Code-path deletion is
unconditional; data migration is the only legitimate outlet for deletion
reticence.

E3-E4 — delete the legacy code paths in this order, each commit keeping E1 green:
  E3.1 storage shims: scanWorkerUpdate reconstruction (store.go:2688-2697),
      live research_findings write path;
  E3.2 Super backlog settlement: non-execution_request packets addressed to
      persistent Super are rejected/quarantined, not left pending
      (super_controller.go:256-265; the pinned regression at
      update_coagent_source_packet_test.go:169 must move from "asserts
      pending" to "asserts settled");
  E3.3 runtime prompt/tool legacy: textureInlineSourceRefRE (texture.go:47),
      plainTextStructuredTextureDoc fallback
      (texture_structured_revision.go:130), coagentSourcesFromRefs generic
      parsing, silent source-materialization skip
      (texture_evidence_sources.go:163-170);
  E3.4 frontend/publication legacy: clickable-link upgrading
      (frontend/src/lib/texture-source-renderer.ts:489),
      markdown-as-canonical-body rendering;
  E3.5 test-only endpoints: /api/test/texture/research-findings;
  E4 P1/P2 validation gates: vocabulary-validate packet.sources[].kind and
      selectors[].kind (P1, tools_worker_update.go:604); reconcile target.uri
      schema vs Go validation (P2, tools_worker_update.go:58 vs :608);
      settle non-execution Super packets (P2).

E5 — deploy and run staging acceptance on yusefnathanson@me.com. Two halves:
  E5a push be52b194 + c35502b2 (and any E1-E4 commits) to origin/main, run
      the full landing loop (CI -> Node B deploy identity -> health), then on
      choir.news create/revise a Texture from the prompt bar as the owner and
      confirm: the loop advances past v3, each researcher update carries
      packet.sources, Texture renders native source nodes (not clickable
      links, not markdown prose), the first paragraph is reader-facing (not
      process metadata), markdown control tokens are absent from visible
      prose, non-execution Super packets neither execute nor linger.
  E5b after deletion: re-confirm all of the above plus the E1 contract test
      green on the deployed build, and rg for legacy markers returns only
      migration/rejection code.

Do not claim settlement without deployed staging proof on the existing
yusefnathanson@me.com account (not a synthetic user). Do not reintroduce
compat shims.
```
