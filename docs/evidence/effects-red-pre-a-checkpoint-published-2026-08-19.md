# Effects pre-A checkpoint bound and published — 2026-08-19

**Boundary:** deployed acceptance of the pre-A checkpoint baseline restoration fence.
Not freeze. Not promote. No live send. Super was not started. Effects remain OFF.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Mutation class:** red — pre-A checkpoint bind, guest/proxy write deadline extension,
and 40-table VM-local content witness.

## Checkpoint Identity

Retained computer `computer-03335285269bdba4f94377e56879f9e6` is active at realization epoch 324 on guest `0d8b8920487905b01abb74ca827ea3f858024f11`.

Product CLI invocation:
```bash
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=15m \
  go run ./cmd/choir computer checkpoint \
  --computer computer-03335285269bdba4f94377e56879f9e6
```

Observed Checkpoint Response:
- `computer_id`: `computer-03335285269bdba4f94377e56879f9e6`
- `checkpoint_eligible`: `true`
- `release_digest`: `7482d7f0a2c5b55d34ce6396f4e49eed48f08042913ca158a77a4be0f6cb20b2`
- `checkpoint_digest`: `99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7`
- `accepted_event_head`: `d83052ae618cba7966144c41c0d33db48d396f3215deafaf42eca7df87552a6a`
- `effective_event_head`: `a3cf16d0d1dbb46e4ebd5841af5007575fb74184d54c2e6fa26f856769b92b44`
- `effective_state_commitment`: `40df35913994fab47d2dd2c450a7f9d3958ea639ec9fb2002b8b8073534fe091`
- `dolt_head`: `ho5aeocad9fplm8q5eskfaqkj98cf8s5`
- `content_root`: `c302f6d9e570f8755936be9c178d9a8e16ccf417551a5181b5d8be5a0637c903`
- `frontend_identity.digest`: `00ec37261b45592ec84d1339f99d1b6c7bebf1d4fbafa131b3b0089c8e8bf643`
- `receipt.issuer`: `corpusd` (`signer_domain: "platform-control"`, `key_id: "868f96cca8726f99"`)
- `receipt.issued_at`: `2026-08-19T12:23:14.107068167Z`
- `receipt.signature`: `yMfipG9dn+T0KyjdlhP3Lgfisy2hM49ZgqEJC0YEIX+Z19McjmslNsJtPh3kQ0XCI8qYwRFlpd4U/ZtyW8k4DA==`

## Substrate Repairs Verified Live

1. **Family A Alias Schema Migration (997f25cb & 087cf290):**
   - Reducer-backed projection batch operation `ProjectOpTextureAlias` in `internal/computerevent/projection_batch.go`.
   - Idempotent `ensureTextureColumn`, `ensureTextureDocumentAliasesPrimaryKey`, and post-column index creation in `internal/store/texture.go`.
   - Legacy index name collision dropped and recreated with composite columns `(owner_id, computer_id, doc_id)`.
   - Desktop unscoped `computer_id=''` row twins deleted on scoped write in `internal/store/desktop_live.go`.
2. **Lifecycle Route Budgets and Deadline Extension (06ec8f8d & 0d8b8920):**
   - Checkpoint, restore, and rematerialize routed to `replayAutoputerHTTP` (10m client budget) in `internal/proxy/computer_lifecycle.go`.
   - Guest write deadline extended via `extendReplayCompletenessGuestWriteDeadline` in `internal/agentcore/api_self_development.go`.
   - Proxy write deadline extended via `http.NewResponseController(w).SetWriteDeadline(...)` in `internal/proxy/computer_lifecycle.go`.
3. **Replay Completeness & Verification:**
   - 40 tables matched between live Dolt and replay projection with 0 row differences.
   - `run_memory`: 1083/1083 matched (0 live-only, 0 replay-only, 0 differing).

**Heresy delta:** discovered — none. introduced — none. repaired — checkpoint timeout and guest server write deadline.

**Conjecture delta:** confirmed — extending guest and proxy write deadlines and routing checkpoint through `replayAutoputerHTTP` enables the 40-table Dolt state extraction (~120s) to complete reliably through the owner product path without connection severing.

**Rollback:** `git revert 0d8b8920` for platform source changes. The checkpoint and refresh events are forward records on the canonical event tape.
