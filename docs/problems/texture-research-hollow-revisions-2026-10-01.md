# Texture hollow revisions: research→Texture return path dead
**Date:** 2026-10-01
**Status:** REPAIRED — fix commit lands the return path + hollow-revision prompt
contract + SSE stream hardening + list-latency stats query; divergent panel
agentic-consensus-20261001-205754 (5 ok: claude/codex/gemini38/glm53-flash/gpt6-sol).
**Mutation class:** orange (runtime behavior); lifecycle delivery path = red
**Deploy context:** staging deployed_commit=a3cfaa00; all findings below observed on the a3cfaa00 deployment

## Owner-reported symptoms

1. Texture app: ~10s to list recent textures (should be instant).
2. Opening the most recent doc takes several seconds.
3. Editor immediately shows "Revising…"; revisions accumulate (v0→v12) but every
   revision body is the same "research pending" boilerplate.
4. Persistent "Lifecycle stream disconnected" banner.

## Staging evidence (owner computer-03335285269bdba4f94377e56879f9e6)

Document `1e897c82-f45c-5c33-b896-894332aa9f0c` ("Most recent major climate/weather or
emissions-policy development…"), 13 revisions over ~19.4h, ALL hollow boilerplate:

- v1: "…not yet established here; a current, source-grounded check is needed…"
- v7: "…remains unresolved pending authoritative source verification. The next needed
  step is a research pass…"
- v12: "…remains unresolved pending date-anchored evidence. A research desk is being
  asked to identify one materially significant development…"

Runs for this doc (`/api/runs?doc_id=…`): 100 runs total.

- **Research desks DO spawn and several COMPLETE** (`research:*` agents, state
  `completed`, e.g. `10cd8749` finished 23:45:11, `8f34c0bb` 23:17:23,
  `0f71f827` 17:30:06). Their results never become revision content.
- Other research runs **fail** identically: `tool loop iteration 3: gateway call
  failed: gateway client: chatgpt: status 400 Bad Request` (b38c46fe, 93ea4bba,
  cfc529f8, fde24ead, 36f183ff). Iteration-3 400 = provider rejects the assembled
  request — likely a schema/context-shape defect in the research tool loop, not auth.
- Texture runs accumulate as `pending` (3 queued at 00:42–00:43) and `passivated`;
  a `cancelled` run exists. The revise cadence is ~hourly.

## Verified code defects (confirmed this session; independent of scout)

1. **Lifecycle research `choir.Message` return is rejected.**
   `internal/agentcore/rlm_reduce.go:1299 commitMessageIntent`: in the
   `authority.lifecycle` branch (~L1352) the request sets
   `ProducerUpdateID: update.ProducerUpdateID`, but `update.ProducerUpdateID` is
   never populated in that function (`update.UpdateID` is derived; ProducerUpdateID
   is not). `internal/store/lifecycle.go:3249 QueueLifecycleUpdate` hard-requires
   `producer_update_id` non-empty → every lifecycle-desk `choir.Message` to a
   lifecycle target fails → research findings are dropped at the queue boundary.

2. **Channel-mail fallback cannot wake Texture.**
   `internal/actorruntime/handler.go:306` `handleChannelMessage` returns early for
   `agentprofile.Texture+":"` targets, so `ReportPacket`/`coagent_result` channel
   mail delivers to the log but never reactivates the Texture actor.

3. **Research desks have no typed report surface.**
   `internal/agentcore/tool_profiles.go`: only Management exposes
   `report_to_texture`; Research profiles get `desk_go_eval` only. The only
   intended paths are (1) and (2), both dead.

4. **Hollow revisions are model-authored, prompt-permitted.**
   `textureprompts/overlays/run_system.yaml:16-23` and `revision_policy.yaml:45-58`
   allow interim uncertainty-only revisions. No code gate stops a Texture actor
   waiting on never-arriving research from committing "research pending" prose as
   a numbered revision. Each completed turn's non-revision writes would sleep the
   pending mutation (`store/texture.go:2057-2365`), but ApplyTexture commits a real
   revision every turn — so the "pending" indicator clears while content stays hollow.

## Latency picture (measured, complements texture-latency-layered-evidence-2026-10-01.md)

choir.news /health: `api.resolve` avg 63ms max 2.0s (cut-3 held); `api.upstream`
avg **8.0s, max 58.9s**; `api.total` avg 7.9s. The remaining ~8s is guest-side
(owner guest under desk load: 39 pending mutations, 8 selfdev ops, 3 researchers,
engineMu read/write sharing, ReadObjectSnapshot whole-computer scans).
`surface.resolve` avg 1.07s max 9.65s with 14/23 auth errors.

## Lifecycle stream disconnect — plausible causes (to narrow)

Server `streamLifecycleEvents` (internal/agentcore/api_trajectory.go:145-193) returns
silently on any `ListLifecycleEventPage` error (except cursor-expired → replay_required).
Guest-side page errors under store pressure → stream dies; frontend retries at the
durable cursor; if the error persists (e.g. engineMu-blocked reads timing out at the
proxy), every reconnect dies identically → persistent banner. Alternative: connection
dies at proxy/server WriteTimeout during long silence; heartbeat is 15s so unlikely
unless the guest itself is stalling on engineMu.

## Fix directions deferred to panel (divergent run in progress)

agentic-consensus-20261001-205754, 8 lenses. Expected outputs: choice among
(a) populate ProducerUpdateID in commitMessageIntent,
(b) stop no-op'ing Texture in handleChannelMessage,
(c) typed report_to_texture for research desks,
(d) route through the lifecycle "pending producer report" path texture_turn_runtime
already reads; plus a hollow-revision gate (prompt vs pending-mutation vs
ApplyTextureTurn validation) and the list-latency query.

## What this doc does NOT claim

- Does not claim the gateway 400 on iteration 3 is the same defect class as the
  return-path drop; it is a second failure mode, possibly unrelated.
- Does not claim a specific list-latency query; needs per-doc breakdown of the
  texture listDocuments path.

## Repair landed (same commit as this doc's second revision)

Convergent fix set, each item traceable to the panel's option space:

1. **Return path (red, `internal/agentcore/rlm_reduce.go` commitMessageIntent).**
   Mirrors the retired `update_coagent` lifecycle branch (tools_worker_update.go:217-226):
   derive `ProducerUpdateID` via `deriveLifecycleProducerUpdateID(execution, callerRun)`,
   re-key `UpdateID` via `deriveLifecycleWorkerUpdateID`, default `WorkDisposition=open`,
   and pass `WorkDisposition` into the queue request. Verified: durable pending
   producer update lands for `texture:<doc>` targets and `wakeUpdatedCoagent` dispatches
   `coagent_result` (redrive path, management_controller.go:2817).
   Regression: `internal/agentcore/rlm_reduce_lifecycle_message_test.go`
   `TestCommitMessageIntentLifecycleResearchQueuesProducerReport` — fails on the
   pre-fix code with `producer_update_id … required`, passes post-fix.

2. **Hollow-revision prompt contract (yellow, textureprompts overlays).**
   `run_system.yaml` line for the loop now permits ONE interim checkpoint, then
   requires a `decide`/`wait_for_evidence` turn or run end — never another apply
   turn rewording unresolved state. Research follow-ups ride continuation controls
   (target_work_item_id + packet) since direct Message/Note to research desks is
   rejected by `QueueLifecycleUpdate`'s target gate (lifecycle.go:3282). Same
   correction in `revision_policy.yaml`.

3. **Lifecycle SSE (orange).** `streamLifecycleEvents` +
   `handleLifecycleTextureDocumentStream`: `ResponseController.SetWriteDeadline(zero)`
   defeats the shared 120s WriteTimeout (`server.go:68`), headers flush immediately
   (fixes 15s `onopen` wait), and store page errors now log+backoff (≤10s) instead
   of silently returning (each silent return forced a snapshot-refetch reconnect;
   under engineMu pressure every reconnect died identically → persistent banner).

4. **Replay-from-zero loop (orange, frontend).** `observeLifecycle` gains
   `options.startAfter`; `TextureEditor.onReplayRequired` refetches the snapshot
   and restarts the stream at `snapshot_cursor` instead of `after=0` — closing the
   expiry-loop when 0 falls outside retained history.

5. **List latency (orange).** `textureDocumentResponse` no longer decodes up to
   100k full revision bodies per doc for `revision_count`/`current_version_number`.
   New `Store.TextureRevisionStatsByScope` → `ogCountObjectsByEdgeTo` →
   `DoltStore.CountObjectsByEdgeTo` runs `COUNT(*)+MAX(metadata.version_number)`
   on the indexed `og_edges.to_id` join (no body decode), falling back to the
   full list only when the doc has no revision edges (legacy rows).

Deferred to their own boundaries (not silently shrunk): the gateway
`chatgpt:400` at research-loop iteration 3 (fleet-side, different defect class);
Texture→research direct Message widening in `QueueLifecycleUpdate` (covered by
the prompt fix via continuation controls); the delivered-vs-incorporated gap
(consume-at-commit default can still mark a packet delivered without
incorporation — next repair boundary may require explicit dispositions).

## Residual defect discovered 2026-10-01 (dispatch layer, post-repair re-audit)

The ProducerUpdateID repair fixed the queue internals, but staging evidence
(research run 20cd8749 metadata: `work_item_ids` + `lifecycle_control_bindings`
present, `assignment_id` **absent**, `requested_by_*` **absent** on run
metadata — present only on `work.Details`) shows the defect is wider:

5. **Dispatch predicate too narrow.** `isAssignedDesk()` tests only
   `run.Metadata.assignment_id` (rlm_reduce.go:1239-1243). Lifecycle research
   runs lack it, so even a packet-bodied `choir.Message` falls through to
   `castStagedIntent` → channel row → texture channel_message no-op. The
   repaired queue path is unreachable for the exact runs it was repaired for.
6. **`IntentReport` never takes the queue path.** Only the `message` kind
   checks `isAssignedDesk`; `Report`/`ReportPacket` always go
   commitment-record + channel envelope. The research prompt instructs
   `ReportPacket` — guaranteed dead for texture targets regardless of
   assignment metadata.
7. **`requested_by_*` provenance lives on `work.Details`, not run metadata.**
   `loadLifecycleRequesterRun` reads `requested_by_run_id`/`requested_by_
   agent_id` from the caller run's metadata (tools_worker_update.go). Even if
   dispatch widened, authority resolution would fail until bind-time stamping
   (alongside `work_item_ids`, management_controller.go:~1745) carries it over.

Normalization options (a)/(b)/(c) are before the convergent panel
(.agentic-consensus/messaging-normalize-20261001); the map doc
docs/reports/agent-messaging-system-state-2026-10-01.md carries the full
mechanism census.

**Residual repair landed 2026-10-01 under S0m RN0** (this commit). Items
5-7 fixed: `commitTray` routes `IntentMessage` for lifecycle producers
(`isLifecycleProducer()`: work_item_ids / lifecycle_work_item_id /
lifecycle_control_bindings present, assignment_id absent) into
`commitMessageIntent`; packet-bodied `IntentReport` with an explicit
addressee commits through the same queue path (replacing only the
envelope — the commitment record still mints); and the
lifecycle-control activation path now stamps `requested_by_*` from bound
work items (`management_controller.go` ~1747 + the replay-refresh
branch). Dispatch-level regression tests:
`TestCommitTrayLifecycleProducer{Message,Report}RoutesToQueue` in
internal/agentcore/rlm_reduce_lifecycle_message_test.go.
