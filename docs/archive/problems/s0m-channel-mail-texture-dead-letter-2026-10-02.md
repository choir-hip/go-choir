# S0m — channel mail addressed to texture:* persists then silently drops

**Status:** confirmed in code, fix in flight
**Found:** 2026-10-02, during S0m finish-acceptance item "channel mail
addressed to texture:* rejects at reduce time (not a durable dead letter)"
**Mutation class:** orange — runtime behavior on the desk-mail path

## Symptom

An addressed channel-message envelope whose `to_agent_id` is `texture:*`
commits durably (`AppendChannelMessage`) and then is silently discarded at
the actor boundary (`handleChannelMessage` returns the actor memory
unchanged). The sender's reduce commits successfully — the cell believes the
send landed — while the receiver never sees it. This is a durable dead
letter: persisted, never delivered, never rejected.

## Root cause

`internal/agentcore/channel_store.go:channelCast` (and its wrappers
`ChannelCast`/`CastEnvelope`) validates the cast *role* but never validates
that the *target* is a deliverable desk. `AppendChannelMessage` similarly
carries no target guard. The envelope reaches the recipient actor, where
`internal/actorruntime/handler.go:handleChannelMessage` short-circuits
`texture:*` mailboxes (`return memory, nil`) — texture desks are reached by
the lifecycle packet/queue path, not by channel mail — so the durable row is
a dead letter by construction.

The live callers that can produce a `texture:*`-addressed envelope today:

- `deskEmitSignal` (`internal/agentcore/tools_desk.go`) — `choir.Emit` on any
  desk emits an addressed envelope to an arbitrary `to`. `Emit("texture:…",…)`
  is callable and dead-letters.
- `castStagedIntent` (`internal/agentcore/rlm_reduce.go`) for a
  **non-lifecycle** caller — the `else` envelope arm mints a `channel_message`
  for an addressed `IntentMessage`/semantic act that reached neither the
  assigned-desk nor the lifecycle packet arm. Latent under current
  composition (all production desks are lifecycle-bound) but not impossible.

## Why this is the failure the acceptance forbids

S0m acceptance: "channel mail addressed to texture:* rejects at reduce time
(not a durable dead letter)". The durable dead letter is exactly the
silent-loss failure class the record-native cutover exists to eliminate — the
packet path already routes texture delivery record-natively; the envelope
surface must refuse rather than dead-drop.

## Fix

Reject at the reduce/commit seam: `channelCast` refuses an addressed cast
(`toAgentID` or `toRunID` non-empty) whose target is a `texture:` agent, with
an error pointing at the lifecycle packet path. Unaddressed broadcasts
(`ChannelPost`, empty target) are unaffected. The packet path is unaffected
— lifecycle `Message`/`Emit`→texture mints a `coagent_source_packet` and does
not traverse `channelCast` for delivery (the `ChannelMessage` it builds is
only an ephemeral wake event, never `AppendChannelMessage`'d).

## Evidence

- `internal/agentcore/channel_store.go:42-89` — `channelCast` validates role
  only; no texture-target guard.
- `internal/store/store.go:2438 AppendChannelMessage` — persists with no
  target-profile check.
- `internal/actorruntime/handler.go:306-321` — `handleChannelMessage`
  returns `memory, nil` for `texture:*` (the silent drop).
- `internal/actorruntime/adapter_test.go:1464
  TestHandlerChannelMessageIgnoresTextureMailbox` — proves the drop and its
  persistence (message stored, 0 runs minted, memory unchanged).
