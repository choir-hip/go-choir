# Owner revision blocked on engineering-bound documents: supervision subject counts as desk target and steals the revision wake

**Status:** FIXED in `c58ed60a` (2026-09-30); diagnosed on staging (deployed
`14faf3d5`); deployed verification pending in the mission Landing Loop.

## Symptom

Mission `choir-sub-rlm-document-channel-2026-09-22` (M2): every
`choir texture revise` on the engineering-bound document
`f939b0f9-44e9-5364-bcb2-3dbcace150fc` (trajectory
`84868c2d-a733-5d55-b333-17a137d8ad32`, computer
`computer-03335285269bdba4f94377e56879f9e6`) returns

```
http 409: {"error":"lifecycle has multiple open desk target work items"}
```

The M1 acceptance — an owner revision driving a cast-only sub-RLM cell on the
document channel — cannot run because the revision never lands.

## Root cause

`19e2c256` ("red(runtime): delete actuator=tools end to end; wire supervision
to desk progress") minted a durable supervision subject for every
engineering-bound document inside the assignment commit:
`textureSupervisionSubject` (`internal/store/engineering_assignments.go:1716`)
creates agent `texture:<docID>` plus work item
`work:texture-supervision:<docID>` — the return surface for the desk's
producer-report packets. That subject is a standing report target, never a
revision consumer.

Two sites written under the older invariant "at most one desk agent per
document" break under the resulting dual-agent topology:

1. **Revision guard** — `handleTextureAgentRevision`
   (`internal/textureowner/texture_agent_revision.go:136-151`,
   introduced in `98c6d96e`, predating supervision) iterates open work items
   matching `texture:<docID>` or `engineering:<docID>` and 409s on more than
   one match. It counts the supervision work item, so an engineering-bound
   document permanently shows two candidates: the engineering cast work item
   (`08ac7714-…`, objective "issue a typed precommit…") and
   `work:texture-supervision:…` ("Supervise the engineering desk's assignment
   reports"). Every owner revision 409s.

2. **Wake resolution** — `actorWakeOutboxFromObject`
   (`internal/store/lifecycle.go:712-747`) resolves the owner-revision wake
   target by scanning commit objects for the first agent on
   `ChannelID == docID` with a texture/engineering profile.
   `actorWakeResolverObjects` (`:786-791`) probes `texture:` before
   `engineering:`, so on an engineering-bound document the `owner_revision`
   outbox wake targets `texture:<docID>` — the supervision agent. The
   engineering occurrence consumer (`internal/actorruntime/handler.go:503-530`,
   which calls `ReconcileEngineeringRevisionCast`) never sees the revision;
   the supervision agent's texture-cell path
   (`ReconcileActorOccurrenceWake`) wakes on a cast revision it was never
   meant to execute.

## Impact

- `texture revise` on every engineering-bound document is dead (409).
- Even if a revision were force-committed, the cast wake would reach the
  supervision agent, not the engineering desk occurrence consumer.
- The M1/M2 document-channel cast mechanism is unprovable end-to-end until
  both sites route by executor desk rather than by "any desk agent on the
  channel".

## Evidence

- `choir trajectory 84868c2d-…`: work items `08ac7714-6686-5087-ad76-…`
  (engineering executor, open, objective = M1 task) and
  `work:texture-supervision:f939b0f9-…` (texture supervision, open), plus
  agents `engineering:f939b0f9…` and `texture:f939b0f9…` both with
  `channel_id = f939b0f9-…`.
- Reproduced 409 on `choir texture revise` (staging `14faf3d5`,
  2026-09-30 ~15:00Z).

## Direction (not yet implemented)

Exclude the supervision subject from revision-target selection and resolve
the owner-revision wake to the document's executor desk — engineering when an
`engineering:<docID>` binding exists, else texture — matching the occurrence
consumer contract at `actorruntime/handler.go:503`.
