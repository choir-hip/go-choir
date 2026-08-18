# Effects computer-surface boot failure after image refresh — 2026-08-18

**Boundary:** execute. Problem documentation first. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Mutation class:** red — updater root and per-computer computer-surface serving.
**Protected surfaces:** updater root, computer-surface serving join, vmctl route resolution, deployment routing.

## Live observation

After the gateway timeout repair landed as `eb67848c740fbf3e3e8ef21bf2d78de7dedd9010`, staging reported the retained computer `computer-03335285269bdba4f94377e56879e6` active at realization epoch 302. Guest observability reported the same deployed guest build:

```text
commit eb67848c740fbf3e3e8ef21bf2d78de7dedd9010
started 2026-08-18T06:08:43Z
```

The named computer surface request then returned HTTP 503:

```text
self-development checkpoint: served SPA is underivable
```

The preceding product-path refresh evidence records that `choir.refresh_runtime=1` removes the persistent updater `current` pointer so the guest executes the freshly deployed immutable image. The same boot path does not restage an updater release or its `frontend/index.html` after removing that pointer.

## Source convergence

The serving path is explicit:

- `internal/autoputer/computer_surface.go` serves only `CHOIR_UPDATER_ROOT/current/frontend/index.html` and fails closed when it is absent.
- `nix/autoputer-vm.nix` sets `CHOIR_UPDATER_ROOT=/mnt/persistent/choir-updater`, and the refresh wrapper removes `current` when `choir.refresh_runtime=1`.
- The updater service starts before autoputer but only exposes baseline import through its permissioned Unix API; it does not import the immutable image at boot.
- `internal/agentcore/rematerialize.go` can derive and import the trusted `/nix/store/` baseline, but only when checkpoint binding or another self-development caller invokes `ensureCheckpointReleaseManifest`.
- `nix/node-b.nix` and `nix/node-a.nix` route the computer surface through the authenticated proxy/guest serving hop rather than the host-global `frontend-current` tree.

This leaves a fresh image boot with a healthy runtime API but no joined computer-surface release. A digest or route identity without served bytes is intentionally not green; the observed 503 is the correct fail-closed response, but boot has no product-path recovery to establish the baseline serving join.

## Belief delta

The refresh-runtime repair correctly prevents a stale persistent updater binary from masking the deployed guest image. It introduced or exposed a second boot invariant: after dropping `current`, the immutable image must be imported into the updater root before the computer surface can serve. The existing checkpoint-only fallback is too late for account boot and cannot be treated as a boot repair because checkpoint binding also performs replay eligibility and may publish owner-recovery evidence.

Leading root cause: refresh removes the only serving join and no boot-time, idempotent baseline-import path recreates it. This is source-confirmed; the product observation confirms the resulting fail-closed surface, while direct guest filesystem inspection is intentionally unavailable on the no-SSH product path.

## Safe repair boundary

Implement one narrow, idempotent boot/surface bootstrap that uses the existing trusted-baseline manifest and updater `ImportBaseline` contract, derives the exact current `ComputerVersion` route, and never appends an event, publishes a checkpoint, changes self-development mode, starts Super, or arms an outbox. Keep the computer surface fail-closed while route or baseline evidence is unavailable. Ordinary restart/recovery must continue to preserve a promoted `current` release.

The repair must have focused tests for: refresh with no current eventually serving the immutable baseline; existing current remaining untouched; route/baseline failure remaining 503; and no checkpoint/effect side effect. Rollback is a git revert of the repair commit; the retained computer remains propose-only with effects off.

## Remaining error and next safe probe

Do not call checkpoint, genesis, rematerialize, restore, or an operations POST to make the account surface appear. First land the documentation-only receipt, then implement and locally verify the boot/surface bootstrap. After the normal commit → CI → Node B deploy loop, owner-refresh the retained computer and verify the named surface returns the staged baseline with the expected build/serving identity before any self-development retry.

**Heresy delta:** discovered — refresh can leave a healthy guest with no served computer surface; introduced — none claimed until repair landing; repaired — none at this receipt.
**Conjecture delta:** the refresh/image identity repair remains supported; the new boot serving-join invariant is currently unproven.
**Rollback:** this receipt makes no runtime mutation; supersede or correct it in the Definition if a later product-path receipt disproves the source convergence.

## Post-landing observation

The boot bootstrap repair landed in workflow `32108952503` at `13a0ae7cebc7081753d0a93b92310b00ff41a6d0`. Staging host health and the activation receipt report commit `13a0ae7cebc7081753d0a93b92310b00ff41a6d0`; the autoputer package is installed at that commit. The deploy job intentionally did not refresh the retained computer because vmctl classified it as an immutable `constructed-computer-version` realization:

```text
active_computers: status=empty
computer-03335285269bdba4f94377e56879f9e6: epoch 302, immutable constructed realization
```

The retained guest therefore still reports the pre-repair autoputer build `eb67848c740fbf3e3e8ef21bf2d78de7dedd9010`, and the authenticated named surface still returns HTTP 503 with `self-development checkpoint: served SPA is underivable`. This is not deployed-identity proof for the repair; it is evidence that the normal landing loop preserved the immutable computer rather than exercising the new boot path.

The next safe probe is the Definition-authorized owner refresh of this same retained computer, not a second computer, checkpoint, rematerialization, restore, or self-development retry. The refresh must preserve the VM-local persistent state and then prove guest commit `13a0ae7cebc7081753d0a93b92310b00ff41a6d0` plus the named surface baseline before any further operation.

**Post-landing heresy delta:** discovered — successful host deployment can leave the retained immutable computer on the prior guest commit and therefore cannot prove a guest boot repair; introduced — none; repaired — none at this receipt.
**Post-landing conjecture delta:** the boot bootstrap implementation remains locally supported and deployed, but its behavior on the retained computer is unproven until the owner refresh crosses the immutable-realization boundary.

## Post-refresh replay observation

Owner refresh was executed through the authorized computer-scoped lifecycle key with idempotency key `boot-surface-refresh-20260818`. The lifecycle receipt `01a013c5-c111-7c6f-b0c4-6e6973a71bb4` advanced the same computer from epoch 302 to epoch 303 and left it active. Guest observability now reports autoputer commit `13a0ae7cebc7081753d0a93b92310b00ff41a6d0`, built at `20260818065410`, and the authenticated `X-Choir-Computer`/`X-Choir-Desktop: primary` root request returns HTTP 200 HTML from the guest. The response is not the platform shell, does not contain the underivable-SPA error, and serves asset `/assets/index-yFRxc-PI.js`; the 523-byte root body SHA-256 is `9edd6b0319798d5a2f9eb7bf5c26dcfa29d95dc0efb9466da24ac065e801c0de`.

A separate post-refresh read probe of `/api/computers/computer-03335285269bdba4f94377e56879f9e6/self-development/replay-completeness` repeatedly returns HTTP 500:

```text
replay completeness: reconstruct event chain: computer event appender: fetch durable chain: computer event client: decode response: unexpected EOF
```

This receipt does not claim replay eligibility, checkpointability, restore completeness, or self-development readiness. It records a new restore-substrate observation after the successful serving-join proof. Do not blind-retry replay, bind a checkpoint, rematerialize, restore, or retry self-development until the EOF is documented and repaired through its own problem-first landing loop.

**Replay EOF heresy delta:** discovered — post-refresh durable-chain reconstruction can fail with an unexpected EOF even while lifecycle status and the authenticated guest surface are healthy; introduced — none; repaired — none at this receipt.
**Replay EOF conjecture delta:** the boot serving-join repair is accepted for the named surface, while replay completeness remains unproven and is now a separate blocking substrate problem.


## Replay EOF diagnosis

The EOF is source-reproducible as a bounded transport failure, not evidence of a malformed event chain. `internal/computerevent/HTTPClient.do` wraps every corpusd response in `io.LimitReader(result.Body, 1<<20)` before decoding JSON. A valid `[]DurableEvent` response whose encoded body exceeds one mebibyte is therefore cut at the limit and `json.Decoder.Decode` returns `unexpected EOF`; a local `httptest` probe reproduced that exact error with a single valid durable-event record carrying a 1 MiB field. The staging error is the same decode site on a 200 response, after corpusd's replay handler has constructed the chain.

The current interface fetches all events after a sequence in one response, so merely raising the cap would defer the failure as the durable chain grows. The safe repair boundary is a bounded, sequence-progressing replay read: corpusd must honor an explicit page size, the guest client must reject a non-progressing or overlong page rather than silently truncate it, and reconstruction must consume pages until the chain is exhausted. Preserve the existing fail-closed behavior for malformed, truncated, or incomplete pages. No checkpoint, restore, event append, or effect is authorized by this diagnosis.

**Replay EOF diagnosis delta:** discovered — the guest replay client imposes a 1 MiB response cap on an unpaged durable-chain endpoint, and the cap converts a valid oversized response into `unexpected EOF`; introduced — none; repaired — none at this docs-first receipt.
**Replay EOF conjecture delta:** the staging failure is consistent with the bounded transport defect; the chain's durable records remain unverified until a paginated repair is deployed and replay eligibility succeeds.
**Evidence:** `internal/computerevent/http_client.go:178-225`, `internal/platform/event_replay.go:13-86`, `internal/platform/event_handlers.go:283-307`, and the local large-response probe run on 2026-08-18.


## Replay EOF repair implementation

The source repair is committed as `c38324f4` (`fix: page computer event replay responses`) after the diagnosis receipt above. Corpusd now accepts a bounded `limit` query (default 32, maximum 128) and applies it to the durable-chain query. The guest `HTTPClient.Events` requests pages, requires exactly contiguous sequence progress, and stops only on an empty or short page. Replay pages use an explicit 8 MiB read bound; a page over that bound is rejected with an explicit size error before JSON decoding rather than being truncated into `unexpected EOF`. Non-progressing pages, gaps, oversized pages, malformed JSON, and corpusd failures remain fail-closed.

Local regression evidence passed:

```text
go test ./internal/computerevent ./internal/platform -run 'TestHTTPClientEvents|TestEventArtifactServiceEventsPage|TestHandleComputerEventReplay' -count=1
go test ./internal/agentcore -run '^TestSelfDevelopmentTextureJoinRewakesTerminalPersistentSuper$' -count=1 -v
```

The first command passed the paged oversized-response, non-progress, response-bound, SQL-page, and handler-limit tests. The second command re-passed the unrelated full-package failure identified during a broad run; no replay code is exercised by that test. The repair is not yet staging evidence: no deploy, retained-computer refresh, replay-eligibility result, checkpoint, restore, or self-development retry is claimed here. Effects remain OFF.

**Replay repair heresy delta:** discovered — none beyond the documented transport defect; introduced — none claimed pending landing; repaired — source-level unpaged replay truncation path.
**Replay repair conjecture delta:** the bounded page contract is locally supported; staging replay eligibility remains unproven until the normal red landing loop and a fresh owner-authorized replay read complete.
**Rollback:** revert `c38324f4`; retain the prior staging guest untouched until deployment identity and replay acceptance are independently verified.

## Post-repair replay observation

The replay transport repair landed in commit `c38324f4` and was carried through the normal landing loop in CI run `32114070641`, attempt 2. The run completed successfully, including the Node B deployment. Staging `/health` reports proxy/deployed commit `bdbf7b7eea30bc425e95145040cd9ca55d0a473e`, which contains the repair. The retained computer was not refreshed by this follow-on landing: it remains active at epoch `303` with guest/autoputer `13a0ae7c`.

A fresh owner-authorized replay-completeness read at `2026-08-18T08:32Z` and a second read at `2026-08-18T08:34Z` both returned HTTP 500:

```text
replay completeness: reconstruct event chain: computer event appender: projection repair required
```

This is a changed failure surface: the prior `unexpected EOF` transport failure is no longer reported after the paged replay repair. The deployed code now reaches the appender's fail-closed projection-repair sentinel, which is returned when reconstruction finishes without a local projection head matching the canonical platform head. The receipt does not yet establish whether the durable replay page is empty, incomplete, or otherwise unable to advance; that distinction requires an owner-authorized page-level observation or source-side staging evidence.

This receipt accepts the bounded transport repair as landed, but does not accept replay completeness, restore eligibility, checkpointability, rematerialization, or self-development readiness. Do not bind a checkpoint, rematerialize, restore, retry self-development, or send mail. The next safe probe is to identify the canonical-head/replay-page mismatch under the authorized event-read path, then document that problem before any repair.

**Post-repair replay heresy delta:** discovered — the unpaged EOF path was replaced on the deployed host, exposing a guest/server pagination contract skew; introduced — none claimed beyond the intentional contract change; repaired — host-side bounded replay transport. **Post-repair replay conjecture delta:** server pagination is staging-verified, while the retained guest client and end-to-end event-chain reconstruction remain unverified and blocked by projection repair required.
**Evidence:** CI `https://github.com/choir-hip/go-choir/actions/runs/32114070641` (attempt 2), staging `/health` at `bdbf7b7e`, retained epoch `303`, and the two owner-authorized replay-completeness reads above.
**Rollback:** revert `c38324f4`; no product-state rollback was performed.


## Replay client/server contract-skew diagnosis

Source inspection after the post-repair probe identifies a deployment-version boundary in the changed failure. The deployed corpusd replay handler in the `bdbf7b7e` tree defaults an omitted `limit` to `EventReplayPageSize` (`32`) and reads only that page. The retained guest still reports autoputer `13a0ae7c`; the guest-era `internal/computerevent/http_client.go` at that commit sends only `computer_id` and `after_sequence`, then treats one response as the complete chain. The paged client in `c38324f4` is the code that sends `limit`, follows sequence progress, and reads to the explicit replay bound.

The retained guest therefore has an old client talking to a newly paged server. If the current durable chain is longer than the server default page, the old guest reconstructs only the first page and then returns the appender's final `projection repair required` sentinel when its local replay head does not equal the canonical platform head. This explains the changed error without claiming event-data corruption; the exact current chain cardinality and page sequence remain unobserved through the owner product surface.

The safe next probe is an owner-authorized refresh of the same retained computer so the guest crosses to the deployed replay client, followed by guest build identity and replay-completeness verification. Do not create a computer, rematerialize, bind a checkpoint, restore, retry self-development, or send mail.

**Contract-skew heresy delta:** discovered — host replay pagination can be deployed while a retained immutable guest still runs the pre-pagination client; introduced — the c38324f4 source repair intentionally changes the replay contract and requires guest refresh; repaired — none until the retained guest is refreshed. **Contract-skew conjecture delta:** client/server version skew is the leading explanation for the projection-repair sentinel; exact chain/page evidence remains required.
**Evidence:** `git show 13a0ae7c:internal/computerevent/http_client.go`, deployed `internal/platform/event_handlers.go`, deployed `internal/platform/event_replay.go`, staging `/health` bdbf7b7e, and guest observability 13a0ae7c.
**Rollback:** no product-state mutation; if the refresh probe fails, leave the retained computer untouched and record the failure before any further action.


## Post-refresh replay authority timeout

The same-computer owner refresh was executed with idempotency key `replay-client-refresh-20260818`. Lifecycle receipt `01a01408-586e-7252-b90e-7987653d9899` advanced realization epoch `303` to `304` and left the computer active. Guest observability now reports autoputer commit `bdbf7b7eea30bc425e95145040cd9ca55d0a473e`, so the pagination-aware guest client is installed. The authenticated named surface remains healthy: HTTP 200, guest HTML, 523 bytes, body SHA-256 `f78218ff55222a3e0b781188ad57e46cec6477fb6b9ab8c90ba133ce86232`, asset reference present, no underivable-SPA error.

Two owner-authorized replay-completeness reads after the refresh each took about 31 seconds and returned HTTP 502:

```text
replay completeness authority unavailable
```

Source inspection identifies the new boundary: `internal/proxy/handlers.go` constructs `autoputerHTTP` with a 30-second client timeout, while `internal/proxy/self_development.go` forwards replay completeness through that client and converts any upstream request error to the generic 502. The guest replay probe now gets past the old client/server contract skew, but its full disposable reconstruction and paged event read exceed the proxy's fixed 30-second budget. The exact guest/server phase that crosses the deadline is not exposed by this owner surface.

This is a new gateway/runtime timeout problem, not replay acceptance. Do not blind-retry, bind a checkpoint, rematerialize, restore, retry self-development, or send mail. The next safe work is a docs-first timing diagnosis and a narrowly scoped replay-route timeout repair that preserves fail-closed behavior; do not globally weaken proxy timeouts or mask an unavailable guest.

**Replay timeout heresy delta:** discovered — the pagination-aware guest reaches replay reconstruction but the proxy cuts the owner read at 30 seconds; introduced — none claimed; repaired — none. **Replay timeout conjecture delta:** client/server contract skew is repaired on the refreshed computer, while replay eligibility remains blocked by the fixed proxy budget and the backend duration is unmeasured.
**Evidence:** refresh receipt `01a01408-586e-7252-b90e-7987653d9899`, guest observability bdbf7b7e at epoch `304`, named surface probe above, two replay-completeness 502 reads, `internal/proxy/handlers.go`, and `internal/proxy/self_development.go`.
**Rollback:** no additional product-state mutation; retain epoch `304` and effects OFF while the timeout is diagnosed.

## Replay proxy timeout repair implementation

The docs-first timeout diagnosis is now implemented in source commit `cf950de7` (`fix: give replay completeness a bounded route budget`). The proxy keeps the ordinary autoputer client at its existing 30-second timeout and adds a separate `replayAutoputerHTTP` client for the owner-only replay-completeness route. `PROXY_REPLAY_COMPLETENESS_TIMEOUT` configures that client; the default and Node B value are 110 seconds, below the proxy server's default 120-second write deadline, so a slow or unavailable guest remains a legible 502 rather than a connection-level EOF. The route still returns the upstream status/body on a completed response and remains fail-closed on client timeout.

Focused proof passed:

```text
go test ./internal/proxy -run 'TestLoadConfig_ReplayCompletenessTimeout|TestReplayCompletenessUsesDedicatedUpstreamTimeout|TestReplayCompletenessPathUsesOwnedComputerAndTrustedBinding' -count=1 -v
go test ./internal/proxy -count=1
```

The timing regression holds the guest response for 80ms, proves the 5ms ordinary client is not used, proves a 5ms replay budget returns HTTP 502, and proves a 500ms replay budget reaches HTTP 200. Nix service configuration now supplies `PROXY_REPLAY_COMPLETENESS_TIMEOUT=110s`. This is local source/test evidence only; the retained computer has not been refreshed and replay eligibility, checkpointability, restore completeness, and self-development retry remain unverified and forbidden until the normal landing loop and owner-authorized staging read complete.

**Replay timeout repair heresy delta:** discovered — none beyond the documented proxy budget; introduced — a dedicated bounded replay-route client and config surface; repaired — the fixed 30-second budget no longer governs replay completeness in source.
**Replay timeout repair conjecture delta:** the route-level budget repair is locally supported and preserves fail-closed timeout behavior; staging replay eligibility remains unproven until deployment identity, retained-guest state, and a fresh owner-authorized read are verified.
**Rollback:** revert `cf950de7`; no product-state rollback has been performed.

## Post-repair replay non-equivalence observation

After CI `32118922263` and the successful Node B deployment, staging `/health` and the refreshed retained guest both reported commit `7976ec15dcba462be95feac95670aa4aaeabca77`. An owner-authorized refresh using idempotency key `replay-proxy-timeout-refresh-20260818` produced lifecycle receipt `01a01424-8553-74a2-8547-2373871f4e78` and advanced the same computer from epoch 304 to 305. Guest observability reported the deployed autoputer build at the same commit.

One owner-authorized replay-completeness read completed at `2026-08-18T09:14:04.567717037Z` instead of timing out. It returned HTTP 200 with sequence `3217` on both live and replay heads and matching canonical/desired/effective head fields, but the semantic projection comparison was not equivalent:

```text
result.status: not_equivalent
differences:
- dolt:texture:content_root
- dolt:texture:table:run_memory_entries
- dolt:texture:table:self_development_operations
- dolt:texture:table:self_development_start_intents
- dolt:texture:table:texture_agent_mutations
eligibility.eligible: false
eligibility.reason: behavior-bearing direct-write tables are non-empty without reducers
eligibility.unsupported_tables: run_memory_entries, self_development_operations, self_development_start_intents, texture_agent_mutations
probe_digest: 196f723cff6a0ca1a3edc125669050cd4b4fd09816de53ca5c5fb3a1449b93f8
```

This accepts the route-budget repair as deployed and proves the timeout boundary is crossed, but it does not accept replay eligibility, checkpointability, restore completeness, or self-development readiness. The mismatch is a new projection-authority problem. Do not SQL-empty the listed tables, bind a checkpoint, rematerialize, restore, retry self-development, or send mail. The first next action is source diagnosis of the unsupported direct-write tables and the reducer-backed replacement, with a new problem-first receipt before any runtime repair.

**Replay non-equivalence heresy delta:** discovered — the deployed paged replay now completes but the replay projection is not equivalent because four behavior-bearing direct-write tables lack reducers; introduced — none claimed; repaired — the proxy timeout boundary only.
**Replay non-equivalence conjecture delta:** transport and route-budget defects are repaired on the refreshed computer; replay eligibility remains fail-closed until those table authorities are represented by the event/reducer path.
**Evidence:** CI `32118922263`, staging `/health` commit `7976ec15`, refresh receipt `01a01424-8553-74a2-8547-2373871f4e78`, epoch 305 guest observability, replay result captured at `2026-08-18T09:14:04.567717037Z`.
**Rollback:** no product-state rollback; retain the same computer at epoch 305 with effects OFF while the unsupported-table authority is diagnosed.

## Replay non-equivalence source diagnosis

The four residue tables are not merely stale schema names; each has a live behavior-bearing writer outside the existing event projector:

- `internal/store/run_memory.go:AppendRunMemoryEntry` allocates `seq` and inserts `run_memory_entries` directly. An object-graph adapter (`AppendRunMemoryEntryOG`) exists, but serving reads and writes still use the SQL table, and `BindProjectionTape` does not intercept this method.
- `internal/selfdev/operations.go:BindStartIntent`, `Start`, `RecordAppliedBaseline`, `StartRollback`, and `Transition` write `self_development_start_intents` and `self_development_operations` directly through `DBProvider.DB()`. The operation store has no projection-tape dependency.
- `internal/store/texture.go:CreateAgentMutation` and the state-transition methods (`RecordAgentMutationRevision`, `CompleteAgentMutation`, `DeferAgentMutation`, `FailAgentMutation`, `CancelAgentMutation`, `MarkAgentMutationStale`, `SleepAgentMutation`, `SleepAgentMutationAfterTextureTurn`, and `ReactivateAgentMutation`) update `texture_agent_mutations` directly through the unified embedded Dolt handle.
- `internal/store/projection_tape.go` currently intercepts only object-graph mutations and desktop snapshots. `internal/computerevent/projection_batch.go` and `internal/store/project.go` therefore have no operation that can reconstruct any of those four tables.

The existing graph-backed run-memory adapter is an unwired replacement, not a serving authority: production `ListRunMemoryEntries` still queries `run_memory_entries`, while self-development operations and Texture mutation rows have no corresponding graph kind or read path. Merely changing the replay manifest from `empty_until_supported` to `event_projection` would therefore weaken the gate; it would authorize a nonempty live table that a fresh event replay cannot produce.

### Safe reducer-backed replacement

Use the existing `computerevent.EventProjectionBatchRecorded` → `FinalizeBatch` projector seam as the single replacement. Add typed, versioned projection operations for complete row snapshots of the four tables, and route every production writer through the already-bound `projectionTape` before any direct SQL fallback. The operation store receives an optional batch-builder interface so idempotency/state checks and the append occur under the same tape serialization boundary; unbound unit-test stores retain the current SQL path only as a test seam. `FinalizeBatch` performs the row upsert in the same transaction as the event head, so no save-then-emit path remains.

The retained computer requires one co-moving, owner-authorized residue import batch for all four table families (alongside the already implemented desktop/OG import). The import is a state-as-of-now event, not fabricated history for earlier heads. Only after source repair, deployed refresh, import receipt, a fresh replay-completeness result, and matching table hashes may the manifest reclassify these tables as `event_projection`. Do not SQL-empty rows or copy live SQL into the disposable replay workspace outside the event projector.

**Source diagnosis heresy delta:** discovered — an existing graph adapter for run memory is not wired and the other three tables have no reducer-backed replacement; introduced — none; repaired — none at this docs-first diagnosis.
**Source diagnosis conjecture delta:** the non-equivalence is explained by four direct-write authorities absent from the projection batch; a typed row-snapshot projector plus co-moving residue import is the smallest existing-seam repair. The live authority and replay equivalence remain unverified.
**Evidence:** `internal/store/run_memory.go`, `internal/selfdev/operations.go`, `internal/store/texture.go`, `internal/store/projection_tape.go`, `internal/computerevent/projection_batch.go`, `internal/store/project.go`, and `docs/choir-unified-event-tape-design-2026-08-16.md`.
**Rollback:** this diagnosis is docs-only; revert the documentation commit. No product-state mutation, eligibility change, or effect authorization occurred.


## Reducer residue import proxy timeout observation

After source commit `f551d835866c2fc361d3a14b8ca5307eadb53151` passed CI run
`32130418644`, Node B deployed it and staging `/health` reported proxy commit
`f551d835866c2fc361d3a14b8ca5307eadb53151` (`deployed_at`
`2026-08-18T11:36:43Z`). The retained computer remained the same active
computer at realization epoch 305. The owner-authorized command
`CHOIR_TIMEOUT=600s go run ./cmd/choir computer import-residue-snapshot
--computer computer-03335285269bdba4f94377e56879f9e6` returned after about 32
seconds with HTTP 502:

```text
{"error":"workspace replace authority unavailable"}
```

The command did not return a residue-import receipt, so no appended event or
imported-row claim is made. The computer remains active and effects remain OFF.
The source path currently forwards residue import through the ordinary
30-second `autoputerHTTP` client, while the reducer snapshot can include the
four runtime tables and must serialize a potentially large projection batch.
This is a new bounded-route-budget problem, not permission to retry the import.

**Residue-import timeout heresy delta:** discovered — the deployed reducer
implementation is unreachable through the owner product path when its snapshot
request exceeds the ordinary 30-second workspace-replace proxy budget;
introduced — none claimed; repaired — none.
**Residue-import timeout conjecture delta:** the reducer and owner scoping are
locally tested and deployed, but the retained computer has no import receipt;
the next safe action is a docs-first, route-specific timeout diagnosis and
repair. Do not retry import, replay completeness, checkpoint, rematerialize,
restore, self-development, self-promote, qualified-consensus, or send mail.
**Evidence:** CI `32130418644`, staging `/health` at `2026-08-18T12:21:19Z`,
owner command above, retained computer status epoch 305, and the HTTP 502 body.
**Rollback:** no product-state rollback; retain epoch 305 and effects OFF while
the route budget is diagnosed.


## Reducer residue import deployed and payload EOF observation

The route-budget repair was committed as `c6301b07f162e0688c860c8fc57722d3a0d09516`, passed CI run `32137064213` (attempt 2; the first attempt's unrelated `internal/actorruntime` shard failure was a transient `SQLITE_BUSY` test failure and the failed shard passed on rerun), and deployed to Node B. Staging `/health` at `2026-08-18T12:54:01Z` reported proxy commit `c6301b07f162e0688c860c8fc57722d3a0d09516`, deployed at `2026-08-18T12:49:13Z`, with status `ok`, upstream `vmctl`, and `vmctl_status` `ok`.

With the owner-authorized, computer-bound lifecycle key, the retained computer import was then exercised once:

```text
CHOIR_TIMEOUT=600s go run ./cmd/choir computer import-residue-snapshot --computer computer-03335285269bdba4f94377e56879f9e6
{
  "computer_id": "computer-03335285269bdba4f94377e56879f9e6",
  "desktops": 1,
  "sessions": 0,
  "objects": 3068,
  "edges": 132,
  "appended": true
}
```

The dedicated route budget repaired the prior HTTP 502: the import completed in about 40 seconds and appended the co-moving residue event. A subsequent owner-authorized `computer replay-completeness` read failed HTTP 500 while reconstructing the event chain:

```text
replay finalize sequence 3218: computer event payload: fetch 39c8f6192dbd5d932ddb3e13634505961c517771d2df0ff8ad3e10ba4c25e5d3: computer event client: decode response: unexpected EOF
```

Source inspection shows `internal/computerevent.HTTPClient.FetchPayload` still uses the ordinary `do` response reader capped at 1 MiB, while the payload endpoint returns a base64 JSON envelope for the newly appended large residue payload. The replay-page repair's explicit 8 MiB limit does not cover payload fetches. This is a new transport-bound payload-size problem; replay eligibility remains unproven and effects remain OFF. No checkpoint, restore, self-development retry, self-promote, qualified-consensus, or mail send is authorized. The temporary computer-bound key used for the probe was revoked after the read.

**Residue-import route repair heresy delta:** discovered — the ordinary 30-second proxy budget blocked the deployed reducer path; introduced — a dedicated bounded 110-second residue-import client and `PROXY_RESIDUE_IMPORT_TIMEOUT` Node B setting; repaired — the 502 route-budget failure, verified by the successful 40-second import and appended residue event.
**Payload EOF heresy delta:** discovered — payload fetches remain on the ordinary 1 MiB response cap and truncate the large residue payload envelope; introduced — none; repaired — none at this docs-first observation.
**Payload EOF conjecture delta:** the new EOF is caused by an unwidened payload response reader, not by a new replay projection mismatch; the smallest repair is to use the existing explicit bounded replay response limit for payload fetches and add an oversized-payload regression test.
**Evidence:** commit `c6301b07f162e0688c860c8fc57722d3a0d09516`; CI `https://github.com/choir-hip/go-choir/actions/runs/32137064213`; staging `/health` above; import response above; replay HTTP 500 above; `internal/computerevent/http_client.go:FetchPayload`.
**Rollback:** revert `c6301b07f162e0688c860c8fc57722d3a0d09516` for the route-budget source repair; do not delete or rewrite the appended residue event. Keep effects OFF and retain the computer event head while the payload transport repair is evaluated.

## Post-refresh payload response bound observation

The payload-response repair was committed as `c6d0b34a79f63e4ca4350b8a8b9aa9fe9363e66f` (`fix: bound large computer event payload reads`), passed CI run `32139497055`, and deployed to Node B. Staging `/health` at `2026-08-18T13:59:16Z` reports proxy commit `c6d0b34a79f63e4ca4350b8a8b9aa9fe9363e66f` and deployed time `2026-08-18T13:34:38Z`.

The retained computer was owner-refreshed once with idempotency key `replay-payload-refresh-20260818`. Lifecycle receipt `01a01529-baa3-7b42-89df-5b53632ffa0e` advanced the same active computer from realization epoch `305` to `306`. The first replay read before refresh still reached the old guest's `unexpected EOF`; after refresh installed the payload-bound client, the owner-authorized replay read failed closed with the more specific response-size error:

```text
replay finalize sequence 3218: computer event payload: fetch 39c8f6192dbd5d932ddb3e13634505961c517771d2df0ff8ad3e10ba4c25e5d3: computer event client: response exceeds 8388608 bytes
```

This confirms the prior one-mebibyte truncation path is no longer active on the refreshed guest, but the shared `EventReplayMaxResponseBytes` bound is smaller than the base64 JSON envelope for the retained residue projection payload. The explicit rejection is fail-closed; it does not prove payload corruption or replay equivalence. No checkpoint, restore, self-development retry, self-promote, qualified-consensus, or mail send is authorized.

**Payload-bound heresy delta:** discovered — the first bounded payload repair is below the size of an admitted residue projection envelope; introduced — the explicit bound correctly rejects oversized input instead of decoding truncated JSON; repaired — the prior one-mebibyte payload EOF path on the refreshed guest. **Payload-bound conjecture delta:** the remaining blocker is a payload transport-size contract, not the earlier stale-guest client or a newly observed projection mismatch.

**Evidence:** commit `c6d0b34a79f63e4ca4350b8a8b9aa9fe9363e66f`, CI `https://github.com/choir-hip/go-choir/actions/runs/32139497055`, staging `/health`, lifecycle receipt `01a01529-baa3-7b42-89df-5b53632ffa0e`, retained epoch `306`, and the post-refresh replay read above.

**Rollback:** no product-state rollback; retain epoch `306` and effects OFF while the payload-size contract is repaired through a separate problem-first landing loop.

## Payload response bound repair implementation

The docs-first payload-bound problem receipt above is followed by source commit `8754884d` (`fix: give replay payloads a dedicated response bound`). `FetchPayload` now uses a payload-specific `EventPayloadMaxResponseBytes` bound of 64 MiB instead of sharing the 8 MiB replay-page bound. Replay pages remain bounded at 8 MiB; payload reads remain finite and reject an overlong response before JSON decoding. This preserves fail-closed behavior while admitting the large residue projection envelope observed at sequence 3218.

Local proof passed:

```text
go test ./internal/computerevent -run 'TestHTTPClientFetchPayload|TestHTTPClientEvents' -count=1
go test ./internal/computerevent -count=1
```

The new regression covers explicit rejection above the payload-specific limit; the existing payload test covers an envelope beyond the legacy one-mebibyte reader. This is source-only evidence until the normal landing loop deploys `8754884d`, the retained computer is owner-refreshed onto that guest, and an owner-authorized replay-completeness read completes. Effects remain OFF; no checkpoint, restore, self-development retry, self-promote, qualified-consensus, or mail send is authorized.

**Payload-bound repair heresy delta:** discovered — the first 8 MiB shared bound was too small for the admitted residue envelope; introduced — a separate finite 64 MiB payload budget; repaired — the 8 MiB payload/page coupling that blocked the refreshed guest before JSON decode. **Payload-bound repair conjecture delta:** the payload transport is locally bounded and tested; staging replay eligibility remains unproven until deployment identity, guest refresh, and a fresh replay read complete.

**Rollback:** revert `8754884d`; no product-state rollback has been performed, and the retained computer remains at epoch 306 with effects OFF.

## Post-refresh payload repair timeout observation

The payload-bound source repair landed in workflow `32146221851`; all required CI gates and the Node B deploy completed successfully. Staging `/health` at `2026-08-18T14:25:29Z` reports commit `7ae01fa151e9a6a34c3ef1395026724762f4c8f4`, which contains source repair `8754884d`, deployed at `2026-08-18T14:21:08Z`.

The retained computer was owner-refreshed once with idempotency key `replay-payload-budget-refresh-20260818`. Lifecycle receipt `01a01541-586e-71f3-b4f8-050ad5899f65` advanced the same active computer from realization epoch `306` to `307` and left it active. The owner-authorized replay-completeness read then ran for approximately 112 seconds and returned HTTP 502:

```text
replay completeness authority unavailable
```

The prior `response exceeds 8388608 bytes` failure did not recur, so the refreshed guest passed the 8 MiB payload/page coupling and entered the replay path. The read instead crossed the dedicated `PROXY_REPLAY_COMPLETENESS_TIMEOUT=110s` route budget. This is a new route-budget observation, not replay acceptance; the exact backend phase remains unmeasured at the owner surface. Do not blind-retry, bind a checkpoint, rematerialize, restore, retry self-development, self-promote, qualified-consensus, or send mail.

**Replay payload-route timeout heresy delta:** discovered — after the payload-specific bound repair, the owner replay route exceeds its 110-second proxy budget; introduced — none claimed; repaired — the prior 8 MiB payload rejection. **Replay payload-route timeout conjecture delta:** payload transport now passes the previous bound, while replay eligibility remains blocked by a route budget whose backend phase is not yet observed.

**Evidence:** workflow `https://github.com/choir-hip/go-choir/actions/runs/32146221851`, staging `/health`, refresh receipt `01a01541-586e-71f3-b4f8-050ad5899f65`, retained epoch `307`, and the replay HTTP 502 above.

**Rollback:** no product-state rollback; retain epoch `307` and effects OFF while the route budget is diagnosed through a separate problem-first landing loop.

## Replay route timeout repair implementation

The docs-first timeout receipt above is followed by source commit `f86d6ed7` (`fix: extend replay completeness route budget`). Node B now assigns the owner-only replay-completeness route a bounded `PROXY_REPLAY_COMPLETENESS_TIMEOUT=119s`, up from 110s, while ordinary interactive proxy routes remain at 30s. The value remains one second below the proxy service's 120-second server write deadline, preserving a legible 502 on timeout rather than a connection-level EOF. The default/local route budget remains 110s.

Local proof passed:

```text
go test ./internal/proxy -count=1
nix-instantiate --parse nix/node-b.nix
```

This is source/config evidence only until the normal landing loop deploys `f86d6ed7`, staging health reports the new identity, the same retained computer is refreshed if needed, and a fresh owner-authorized replay-completeness read completes. Effects remain OFF; no checkpoint, restore, self-development retry, self-promote, qualified-consensus, or mail send is authorized.

**Replay route-timeout repair heresy delta:** discovered — none beyond the documented 110-second route budget; introduced — a Node B-only 119-second bounded budget below the existing server write deadline; repaired — the 110-second staging route budget that expired after the payload-bound repair. **Replay route-timeout repair conjecture delta:** the staging route now has measured headroom without globally widening ordinary proxy routes; deployed replay eligibility remains unproven.

**Rollback:** revert `f86d6ed7`; no product-state rollback has been performed, and the retained computer remains at epoch 307 with effects OFF.

## Replay route outer-deadline failure

After CI `32148760924` completed successfully and staging `/health` reported `ae8060035195be3d862cac325244491e85befb7f`, the same retained computer was owner-refreshed with receipt `01a01551-9aa3-7ef6-aaee-bde3cfa10a46` from epoch `307` to `308`. A fresh owner-authorized replay-completeness read still returned HTTP 502 `replay completeness authority unavailable` after `120.54s`. The deployed Node B proxy had the route-specific `PROXY_REPLAY_COMPLETENESS_TIMEOUT=119s`; the failure therefore shows that the replay authority needs more than the 119-second client budget and the outer proxy's 120-second server write deadline is still the limiting boundary.

This is a new substrate problem, not evidence that the payload repair or the bounded 119-second source repair was incorrect: the route requires a longer owner-only deadline, but broadening `SERVER_WRITE_TIMEOUT` for every proxy route would weaken the ordinary-route fail-fast boundary. The next source repair must extend the response deadline for this route only and give its upstream client a longer bounded budget. No state mutation, candidate, bundle, replay-equivalence result, eligibility, checkpoint, restore, retry, promotion, or effect is authorized.

**Problem-first receipt:** discovered `2026-08-18T14:44:18Z`; environment `staging ae8060035195be3d862cac325244491e85befb7f`, retained computer `computer-03335285269bdba4f94377e56879f9e6`, epoch `308`; evidence `CI 32148760924`, staging `/health`, refresh receipt `01a01551-9aa3-7ef6-aaee-bde3cfa10a46`, owner replay command result `HTTP 502 after 120.54s`; remaining error `replay route cannot complete through the public proxy before the 119s upstream/120s server deadline`; rollback `revert f86d6ed7` only if the prior route configuration must be restored, with no product-state rollback.

## Replay route-local deadline repair implementation

The outer-deadline problem receipt above is followed by source commit `34c68283` (`fix: extend replay completeness route deadline`). The proxy now uses `http.NewResponseController(w).SetWriteDeadline(...)` for the owner-only replay-completeness route, deriving the deadline from its dedicated upstream client budget plus a one-second write grace. This extends only that response writer; ordinary proxy routes retain the global 120-second server write deadline. Node B raises only the replay route budget to `10m`; local/fallback deployments retain the 110-second default.

Local proof passed:

```text
go test ./internal/proxy -run 'TestReplayCompleteness' -count=1
go test ./internal/server -run 'Test(NewServerUsesWriteTimeoutFromEnv|WriteTimeoutFromEnv)' -count=1
nix-instantiate --parse nix/node-b.nix
```

This source/config repair is not deployed proof. Effects remain OFF; the same retained computer remains at epoch `308` until the normal landing loop deploys `34c68283`, staging health reports it, the guest is refreshed if required, and a fresh owner-authorized replay-completeness read yields a result.

**Replay outer-deadline repair heresy delta:** discovered — the deployed 119-second route budget was still bounded by the 120-second global proxy write deadline; introduced — a route-local write-deadline extension and a Node B-only 10-minute upstream budget; repaired — the outer deadline without widening ordinary proxy routes. **Conjecture delta:** replay can now run within a bounded owner-only route budget while ordinary proxy fail-fast behavior remains unchanged; staging replay eligibility is unproven.

**Rollback:** revert `34c68283`; no product-state rollback has been performed, and epoch `308` remains retained with effects OFF.

## Guest replay outer-deadline failure

After CI `32150927162` completed successfully and staging `/health` reported `af34ac68c6c6e8f882b315cc17397359b6cc70d4`, the same retained computer was owner-refreshed with receipt `01a0156a-56f6-7ea1-aca7-28094b033545` from epoch `308` to `309`. The fresh owner-authorized replay-completeness read returned HTTP 502 `replay completeness authority unavailable` after `138.77s`. The proxy route-local deadline repair therefore passed the former 120-second proxy boundary, but the guest autoputer still terminated the upstream request before the replay authority completed.

Source inspection identifies the remaining boundary: `nix/autoputer-vm.nix` does not set `SERVER_WRITE_TIMEOUT` for `go-choir-autoputer.service`, so the guest uses `internal/server`'s 120-second default. The next repair must extend the response deadline only when the guest handles the owner-only replay-completeness route; globally widening the guest server write deadline would weaken ordinary guest-route fail-fast behavior. No state mutation, candidate, bundle, replay-equivalence result, eligibility, checkpoint, restore, retry, promotion, or effect is authorized.

**Problem-first receipt:** discovered `2026-08-18T15:11:33Z`; environment `staging af34ac68c6c6e8f882b315cc17397359b6cc70d4`, retained computer `computer-03335285269bdba4f94377e56879f9e6`, epoch `309`; evidence `CI 32150927162`, staging `/health`, refresh receipt `01a0156a-56f6-7ea1-aca7-28094b033545`, owner replay command result `HTTP 502 after 138.77s`, source `internal/agentcore/api_self_development.go`, `internal/server/server.go`, `nix/autoputer-vm.nix`; remaining error `guest replay route cannot complete before the guest's 120s default write deadline`; rollback `revert 34c68283` only if the prior proxy route configuration must be restored, with no product-state rollback.

## Guest replay route-local deadline repair implementation

The guest outer-deadline problem receipt above is followed by source commit `44c02a07` (`fix: extend guest replay route deadline`). The guest API now uses `http.NewResponseController(w).SetWriteDeadline(...)` only for the owner-only replay-completeness handler, with a bounded 10-minute route budget plus one second of response-write grace. Ordinary guest routes retain the global 120-second `internal/server` write deadline; no guest-wide `SERVER_WRITE_TIMEOUT` override was added.

Local proof passed:

```text
go test ./internal/agentcore -run 'TestReplayCompletenessExtendsGuestWriteDeadline|TestReplayCompletenessUsesDisposableProjectionWithoutMutatingLiveStore' -count=1
```

This source repair is not deployed proof. Effects remain OFF; the same retained computer remains at epoch `309` until the normal landing loop deploys `44c02a07`, staging and the guest report it, the guest is refreshed if required, and a fresh owner-authorized replay-completeness read yields a result.

**Guest replay outer-deadline repair heresy delta:** discovered — the guest's global 120-second write deadline cut the owner-only replay route after the proxy route was extended; introduced — a guest route-local response deadline with a bounded 10-minute budget; repaired — the guest outer deadline without widening ordinary guest routes. **Conjecture delta:** both proxy and guest replay hops now have aligned bounded owner-only budgets; staging replay eligibility remains unproven.

**Rollback:** revert `44c02a07`; no product-state rollback has been performed, and epoch `309` remains retained with effects OFF.
