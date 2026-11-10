# Coding-agent desk over MCP: station (2026-11-10)

Lane X4 of the overnight mission. This is the named station a later session
continues from. Design: [coding-agent-desk-over-mcp-2026-11-09.md](coding-agent-desk-over-mcp-2026-11-09.md)
with its section 11 rulings. Failure list:
[coding-agent-desk-failure-list-2026-11-10.md](../problems/coding-agent-desk-failure-list-2026-11-10.md).
Probes: [coding-agent-desk-step0-probes-2026-11-10.md](../evidence/coding-agent-desk-step0-probes-2026-11-10.md).
Mutation class of this document: green. Branch `x4-coding-desk`, not pushed.

## Built (all additive, unwired: no runtime path calls any of it yet)

- `internal/deskjudge`: the two Jev questions.
  - `DecideRoute`: pass an interim update through to Texture, or wake the
    management model. Fails open to the pass-through and says "no judgment ran"
    (R1, R7). Decision id derives from the Report id (R3). The claim is data in
    Jev's context, never in the question (R6).
  - `Admit`: the `desk_complete` gate. A completed result needs the
    assignment's own checks, run by the runtime and green, and Jev's "suffice"
    on a whole read of the change; otherwise refuse (a check failed) or
    escalate (no checks, Jev cannot say, Jev says escalate, the read was cut).
    Fails closed (C1-C7, C9).
- `internal/deskmcp`: the eleven tools of design 2.3 (`catalogue.go`), typed and
  bounded, schema and validation from one table, `additionalProperties:false`;
  `Host.Handle` (identity from the `Binding`, verb check against the
  performer's registry row with no desk name, per-assignment write lock,
  observations unlocked, records suffix, Jev routing after commit, the
  `desk_complete` gate); the stdio MCP server (`server.go`: handshake,
  constant tool list, hostile-line survival, progress notifications, cancel);
  the per-run socket wire (`wire.go`: a frame has `tool` and `args` only).
- `cmd/choir-desk-mcp`: `serve --socket PATH` and `probe-echo`.
- `scripts/probes/x4_step0.py`: step 0 probes.

## Proved (local tests, no money, no staging)

Every item in the failure list marked F1-F14, R1-R3, R6, R7, C1-C7, C9 has a
test in `internal/deskjudge/deskjudge_test.go`,
`internal/deskmcp/host_test.go` or `server_test.go`; the server half of probe
(ii) is `cmd/choir-desk-mcp/main_test.go`. Suites run with `-race`.

## Not proved, and why

Nothing here has run against Claude Code, the reducer, a capsule, the store or
d@e. R4 (no setting selects routing), R5 (decisions on the tape), C8, F12 as a
crash drill, and M1-M2 are open.

## Owed, in order

1. **Probes on d@e** (evidence doc): (i) CLAUDE.md, (ii) 2.1.197 connects a
   stdio server, (iii) queued stdin lines. (ii) decides whether the pin moves.
2. **The host Backend** (red, two seats): implement `deskmcp.Backend` in
   `internal/agentcore` over the real reducer. `Reduce` sends a one-intent tray
   through the reduction a cell's intents take (so the verb gate, sender rules,
   the engineering-reaches-Texture-only-through-management refusal, the
   precommit rules and the Complete guards apply unchanged), serialized by
   `Host`. `NewRecords` reads the cursor shown records. `PrepareCompletion` runs
   the assignment's `checks` as one capsule job the host starts and builds the
   change (diff plus context) to `deskjudge.ReadBoundTokens`. `Escalate` writes
   a non-final record to the admitter with the gate's reason. Two review
   findings land here: the reducer must scope `desk_restate.target` and
   `desk_reply.target_ref` to the bound assignment (the tool only bounds their
   length, it cannot know whose record a ref names), and the Backend must pin
   the capsule diff between `PrepareCompletion` and the staged Complete (the
   checks and Jev's read must be of the bytes that Complete freezes, so a write
   after the gate cannot ride in). Add the
   `ActingWorker` field next to `ActingTask` (`types/commitment.go:198-202`)
   and stamp it from `Call.Acting`. Any new body must pass the vocabulary guard:
   no JSON keys named role, profile, agent_profile, agent_role,
   authority_profile, requested_by_profile, actor_profile, target_profile; add a
   store test committing it with the cutover on (see
   `TestCommitLifecycleHeadWithParentLinksAfterCutover`). `RecordRoute` writes
   `deskjudge.Decision` (its fields are already clear of those keys).
3. **The broker side**: the executor pulls desk calls from the broker per run
   and serves `ServeSocket` for that run's `Binding`; a socket per run;
   `--mcp-config` becomes `deskmcp.McpConfigJSON`, `--allowedTools` gains
   `deskmcp.AllowedToolNames()` (`internal/claudedelegate/spec.go`; its tests pin
   the argv). The stripped environment stays.
4. **Replace the setting with the routing** (R4): remove
   `engineering.progress_to_texture` (`store/computer_setting.go`,
   `agentcore/engineering_progress_relay.go`, `engineer_reuse.go`,
   `engineering_waits.go`); mint the Texture relay Note from the decision, in a
   second commit after the Report, not inside the reducer transaction (see
   "Design problems" 1). Delete the switch from the registry.
5. **The performer driver** (shape b): the executor starts the coding agent as
   the assignment's performer when a grant exists, with `--input-format
   stream-json` only if probe (iii) passes; the Go-driven engineer stays the
   performer when no grant exists, and the binding records which performer ran
   (M1). A grant that lapses mid-round closes the round `blocked` (M2).
6. **Grant as an owner act** (design step 6a; its own mission): proxy route,
   signed receipt (`internal/owneract`), host store, gateway read, tape mirror,
   revoke; delete the nix list. The owner's Claude login stays the credential
   (ruling 5); no spending cap, tracking only (ruling 6).
7. **d@e proof**: a full self-development operation with no DeepSeek engineer
   run, interim Reports routed by Jev, a forged socket call refused, the crash
   drill (kill the guest mid-round: closed "interrupted by a restart", nothing
   resumed or retried).
8. Second driver (opencode, Codex) and the deletions of design step 9 (the
   progress relay is not deleted: ruling 3 withdrew that).

## Decisions where the design was silent

- `desk_ask`'s `req` is not defined in the design; it is an optional short
  string ("the kind of answer you need").
- Shape (a) is not built (ruling 1), so all eleven tools are exposed, including
  `desk_precommit` and `desk_restate`.
- Route outcome names are `pass_through` and `wake_model`, not desk names, so
  the desk-names gate stays at its allowlist and a desk can be renamed.
- A completed result with no assignment checks escalates; it never passes.
  A truncated read never carries a "suffice".
- The two Jev failures run opposite ways on purpose: a routing failure passes
  the update through (never dropped), a completion failure escalates (a pass is
  never a default).
- An escalation at `desk_complete` returns a normal tool result ("Management
  has been asked; the assignment stays open"), not an error, and stages no
  Complete.
- A routing judgment in flight at a crash is lost and not replayed (restart
  rule); the Report itself already stands in management's inbox.
- The server rejects nothing on argument shape itself; the host validates, so
  the in-capsule server stays a translator. Only a tool name outside its list is
  a protocol error there.

## Design problems found

1. **Routing cannot live inside the reducer transaction.** The relay mints its
   Note "in the same transaction" as the Report (`engineering_progress_relay.go`),
   and the Jev judgment is a model call (up to two minutes). Wiring ruling 3
   means moving the relay to a second commit after the judgment. The Report and
   the Note then land at different times, and `CommandID`s must derive from the
   Report id so a replay is idempotent (`wait-relay:`/`review-relay:` already
   do).
2. **"Wake the management model" is not a lever the design gives.** The Report
   is addressed to management and a desk with `InboxHead > cursor` is woken
   (`inboxUnread`, `inbox_records.go:1096`, reached from
   `actorruntime/handler.go:591`); I did not trace the commit-time trigger, but
   as read, committing the Report wakes management whatever Jev says. So
   `pass_through` as "do not wake management" needs the wake rule to skip
   non-final engineering progress Reports until the decision says
   `wake_model`. That is a wake-rule change in a red path and wants its own
   failure list. The design's own section 10 listed this wake as untraced.
3. **The design's `execution_refs` at Complete.** Section 3.5 item 4 has the
   handler run the checks "and cite that receipt plus any the agent cited";
   `ResolveExecutionReceipts` resolves the agent's refs through the executor, so
   an agent ref that does not resolve refuses the Complete after the gate has
   admitted it. `Host` passes the agent's refs through as advisory; the Backend
   should drop an agent ref that does not resolve instead of failing the
   Complete, or the tool should say so before the gate runs.
4. **Step 0 probe (i) has no stored input in the tree.** It needs a real run
   directory from d@e or b@c.
