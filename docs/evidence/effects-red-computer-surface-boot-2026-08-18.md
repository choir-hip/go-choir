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
