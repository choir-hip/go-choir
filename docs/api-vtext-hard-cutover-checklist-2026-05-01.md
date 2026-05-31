# API and VText Hard Cutover Checklist

Date: 2026-05-01

Purpose: ordered implementation checklist for hard-cutting Choir to a secure, event-log-verifiable prompt-bar -> conductor -> VText workflow.

Goal instruction: complete this checklist in order. Do not skip ahead to VText behavior before the public API surface, internal agent boundary, Trace hardening, and event-log verification are addressed.

Read first:

- `docs/api-surface-and-vtext-workflow-review-2026-05-01.md`
- `docs/current-architecture.md`
- `docs/implementation-scope.md`
- `docs/runtime-invariants.md`
- `README.md`

Treat those as the source of truth. Other mission docs, tests, comments, and prompt defaults may be stale. If docs conflict, prefer current architecture.

2026-05-31 update: the conductor-authored initial VText seed described in this
completed checklist is superseded. Current architecture requires conductor to
route/open only; VText must write the first canonical appagent document version
through the same edit path used for later VText revisions. This checklist
remains historical evidence for API hard-cutover work, not the current VText
version contract.

## Operating Instructions

- Work through this checklist sequentially.
- Mark checklist items complete only when implemented and verified.
- Public API hard cutover comes first.
- Internal agent boundary comes second.
- Trace hardening comes third.
- Event-log verification comes fourth.
- VText product behavior comes after the boundaries and verifier.
- Docs/tests and final verification come last.
- Do not preserve old browser-public runtime APIs for compatibility.
- Do not deploy, push, or open a PR unless explicitly asked.
- Final report must list changed files, removed/gated endpoints, new endpoints, verifier guarantees, remaining risks, and exact commands run.

## Invariants

- Browser-public APIs express normal user/app operations only.
- Browser callers cannot name runtime roles, spawn agents, mutate prompts, claim appagent authorship, or allocate desktops.
- Trace is read-only and owner-scoped.
- VText verification is over durable event-log causality, not UI appearances.
- VText owns document synthesis.
- Workers provide structured updates; workers do not patch VText.
- Persistent `super` is the privileged execution root.
- VText does not spawn `super`.
- Only `super` spawns `cosuper`.
- Dry-run/stub paths are engineering scaffolding only, never product proof.

## Checklist

### 1. Public API Hard Cutover

- [x] Add/standardize a product prompt-bar endpoint such as `POST /api/prompt-bar`.
- [x] Make prompt-bar requests accept user intent only.
- [x] Remove browser-supplied `agent_profile`, `agent_role`, model, trajectory, channel, desktop execution semantics, and runtime metadata.
- [x] Server creates conductor runs internally from prompt-bar submissions.
- [x] Remove or gate browser access to `/api/agent/loop`.
- [x] Remove or gate browser access to `/api/agent/spawn`.
- [x] Remove or gate browser access to `/api/agent/status`.
- [x] Remove or gate browser access to `/api/agent/loops`.
- [x] Remove or gate browser access to `/api/agent/events`.
- [x] Remove or gate browser access to `/api/agent/channel-messages`.
- [x] Remove or gate browser access to `/api/agent/topology`.
- [x] Remove or gate browser access to `/api/prompts`.
- [x] Register `/api/test/vtext/*` only when test APIs are enabled.
- [x] Remove or test-gate `/api/shell/error`.
- [x] Make public VText revision POSTs create user-authored revisions only.
- [x] Rename or replace `/api/vtext/documents/{id}/agent-revision` with product language such as `/api/vtext/documents/{id}/revise`.
- [x] Split vmctl desktop lookup from provisioning.
- [x] Ensure browser-selected `desktop_id` cannot allocate, fork, promote, mint, or switch to unknown VMs.

### 2. Internal Agent Boundary

- [x] Ensure conductor is started server-side from prompt-bar submissions, not by browser metadata.
- [x] Ensure VText owns canonical document synthesis.
- [x] Ensure researchers provide structured findings/evidence updates.
- [x] Ensure workers never patch VText directly.
- [x] Ensure appagent-authored VText revisions are created only through the VText edit tool path.
- [x] Ensure provider final text cannot create VText revisions.
- [x] Introduce or enforce persistent per-user/session `super` as privileged execution root.
- [x] Remove VText permission to spawn `super`.
- [x] Ensure VText requests privileged execution through persistent `super`.
- [x] Ensure only `super` can spawn `cosuper`.

### 3. Trace Hardening

- [x] Make Trace use only read-only owner-scoped `/api/trace/*` projections.
- [x] Remove all Trace frontend dependencies on `/api/agent/*`.
- [x] Remove all Trace test dependencies on `/api/agent/*`.
- [x] Ensure Trace cannot start runs, spawn agents, mutate prompts, expose arbitrary raw mailboxes, or change desktop/VM state.
- [x] Keep Trace as a UI over event-log projections, not the verifier itself.

### 4. Event-Log Verification

- [x] Build deterministic event-log verifier tests for VText workflows.
- [x] Verify browser used only the public prompt-bar endpoint.
- [x] Verify conductor run was created by the server.
- [x] Verify conductor routed/opened the VText workflow.
- [x] Verify VText document revisions have valid causal parents.
- [x] Verify appagent-authored revisions were created only by `edit_vtext`.
- [x] Verify VText requested only allowed work.
- [x] Verify privileged execution flowed through persistent `super`.
- [x] Verify `super` spawned `cosuper` when execution delegation was needed.
- [x] Verify researchers emitted structured findings/evidence.
- [x] Verify execution workers emitted structured artifacts/tests/results.
- [x] Verify real search tool events when live search is required.
- [x] Verify real artifact write events.
- [x] Verify real verification command and result events.
- [x] Verify VText consumed worker update event IDs/message sequences in later revisions.
- [x] Verify no browser request called removed/internal runtime APIs.
- [x] Add stochastic workflow tests with varied timing/revision ordering.
- [x] Make dry-run/stub tests clearly named and gated as plumbing only.
- [x] Make opt-in live workflow acceptance pass only through the product prompt-bar path and event-log verifier.

### 5. VText Product Behavior

- [x] Ensure V0 is the initial user input.
- [x] Historical May 1 target: ensure V1 is a useful first document
  seed/abstract. Superseded on 2026-05-31 by the current contract: VText writes
  `v1`; conductor does not create appagent document text.
- [x] Ensure V1 is not "Conductor framing", transcript text, or control-plane instructions.
- [x] Ensure later appagent revisions are complete document versions created by VText editing prior versions.
- [x] Ensure VText revises toward a coherent current-state document, not chat/status updates.
- [x] Ensure user edits can happen at any time and become part of the version graph.
- [x] Ensure worker updates are inputs to synthesis, not document patches.

### 6. Docs And Tests

- [x] Update `README.md` to the new public API, Trace, and VText contracts.
- [x] Update architecture docs so future agents do not preserve old `/api/agent/*` behavior.
- [x] Update stale prompt defaults that still instruct wrong spawning behavior.
- [x] Add regression tests proving forbidden browser APIs are unavailable.
- [x] Add regression tests proving browser cannot set runtime roles or appagent authorship.
- [x] Add regression tests proving browser-selected `desktop_id` cannot mint VMs.
- [x] Add/keep one opt-in live workflow demo, but treat the event-log verifier as proof and video as demonstration.

## Do Not

- [x] Do not preserve compatibility with old browser-public runtime APIs.
- [x] Do not keep `/api/agent/*` as "debug-only" browser APIs.
- [x] Do not manually spawn workers from Playwright and call it product proof.
- [x] Do not use marker strings, screenshots, Trace role summaries, or stub output as acceptance proof.
- [x] Do not add regex/status classifiers as the primary safety mechanism.
- [x] Do not deploy, push, or open a PR unless explicitly asked.

## Completion Criteria

- [x] `go test ./...` passes.
- [x] Frontend build passes.
- [x] Browser/product tests pass against the new prompt-bar endpoint.
- [x] Removed/gated APIs are covered by tests.
- [x] Trace works without `/api/agent/*`.
- [x] Event-log VText verifier passes deterministic tests.
- [x] Event-log VText verifier passes stochastic tests.
- [x] Opt-in live workflow acceptance passes only through the real product path.
- [x] Final report lists changed files.
- [x] Final report lists removed/gated endpoints.
- [x] Final report lists new endpoints.
- [x] Final report lists verifier guarantees.
- [x] Final report lists remaining risks.
- [x] Final report lists exact commands run.
