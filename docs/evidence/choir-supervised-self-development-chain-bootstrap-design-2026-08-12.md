# Canonical chain bootstrap — design receipt 2026-08-12

Define receipt for the plain-computer canonical-chain bootstrap that makes a
VM-local computer replay-eligible. This is a red-class design touching the
canonical computer event chain (the mission's single semantic authority).
Documentation first; the implementation slice follows in a separate commit.

- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Precondition: post-cutover replay probe `equivalent` (82 live == 82 replay, zero differences), but `live_head` and `replay_head` null.
- Owner direction: pre-launch, no backwards compatibility; effects stay OFF; event chain remains sole authority.

## Problem

`replayEligibility` (`internal/agentcore/replay_eligibility.go:234-253`) requires
**both event heads non-nil and equal** plus zero residue and equivalence. The
retained computer satisfies everything except non-nil heads: its workspace was
replaced onto current DDL, so `computer_event_projection_heads` is empty and
`Store.Head` returns nil. The only production genesis path
(`POST /self-development/genesis`, `api_self_development.go:356-508`) is
checkpoint-coupled (recordGenesisBaseline -> selfdev Operation + CheckpointResponse)
and proxy-disabled (`handlers.go:722-723`), and the Definition forbids reusing it.

The product already models a pre-genesis computer: `credential_envelope.go`
uses `issued_pre_genesis` when `head == nil` and `issued` when non-nil. A null-head
computer is a recognized state that is expected to gain a head.

## Design (convergent panel 2026-08-12, 6/6 APPROVE WITH CONDITIONS)

New owner-scoped product verb, guest-core handler, distinct from the VM lifecycle
parser and from selfdev genesis.

- **Surface:** `POST /api/computers/{id}/lifecycle/bootstrap-chain`, scope
  `computer:lifecycle`, CLI `choir computer bootstrap-chain --computer <id>`.
- **Binding:** `TargetStateCommitment = StateCommitment(EffectiveStateRefs{`
  `ReducerVersion: ReducerVersionV1`, `CodeRef: "git:"+buildinfo.Commit`,
  `ArtifactProgramRef: "guest-image:"+digestIdentityArtifact("guest-image-manifest", CHOIR_GUEST_IMAGE_MANIFEST).SHA256`,
  `EmbeddedDoltRefs: nil` `})`. Both refs are derived inside the guest, never
  accepted from the request. Refuse if `Commit` empty/`local` or
  `DeployedCommit != Commit` (execution-identity invariant).
- **Event:** `EventGenesisImported` (sequence 1, `PreviousHead=ZeroHead`),
  `ResultingEffectiveCommitment == TargetStateCommitment`, `PayloadCommitment == commitment`,
  `AuthorityRef: "external-owner-genesis:"+ownerID`, deterministic idempotency key.
  Appended via `eventAppender.AppendNew` (writes platform CAS + local projection;
  the internal Dolt commit is not a product checkpoint).
- **Idempotency:** if `Store.Head` is already non-nil, return the existing head
  without a write. Concurrent losing call re-reads and returns the converged head.
- **Prohibitions:** no `EventCheckpointPublished`, no `CheckpointResponse`, no
  selfdev `Operation`/baseline/route/updater/verifier, no arm/propose/approve/
  materialize, no effect, no live-content binding (`EmbeddedDoltRefs` empty).

## Eligibility consequence

Genesis writes only event-projection rows (`computer_event_index`,
`computer_event_projection_heads`) — the only non-empty class the manifest admits.
Replay reconstructs the same genesis via `Reduce` deterministically, so
`liveHead == replayHead` and `eligible=true` on the retained computer.

Eligibility here certifies **projection-reconstruction equivalence**, not causal
provenance of the 82 live rows. It is not restore license; checkpoint design is
the next axis.

## Blocking risks (resolved by design)

- Binding live Dolt contents into `EmbeddedDoltRefs` (blesses arbitrary state) — rejected.
- Binding agent/worktree HEAD instead of the guest's `buildinfo` — rejected; bind `Commit` and gate `DeployedCommit == Commit`.
- Reusing selfdev genesis — rejected (checkpoint + Operation side effects).
- `replace-workspace` after a successful bootstrap — forbidden (would split platform CAS head from a wiped local projection).

## Rollback

Git revert of the implementation commit. The quarantined pre-cutover workspace
remains in-guest as inert evidence. A bootstrap append is a forward event, not
an in-place mutation; deleting the event in place is not an admissible rollback.

## Consensus

`.agentic-consensus/replay-chain-bootstrap-20260812/` — devin, omp-gemini36,
omp-cursor-grok45, omp-gpt56-sol, opencode, cursor all `APPROVE WITH CONDITIONS`
(high confidence); omp-deepseek-v4-flash-free timed out; codex excluded.
