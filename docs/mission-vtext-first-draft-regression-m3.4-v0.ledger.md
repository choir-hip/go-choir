# Mission M3.4 Ledger - Prompt-Bar VText First-Draft Regression

## 2026-06-15 - Problem Checkpoint

Mutation class: red. Protected surfaces include prompt-bar API, conductor
route materialization, VText document/revision/mutation state, provider
tool-loop handling, Trace/VText projection, vmctl restart/passivation recovery,
and staging deploy routing.

Initial owner evidence: prompt "What's new with Iran war" opened a VText titled
the same, showed `V0` and `Writing first draft...`, but stayed empty/pending for
minutes. Screenshot exposed run prefix `386f6c28-5...7be3ad`.

Node B evidence:

- Exact VText activation:
  `386f6c28-5594-4605-ba02-5c90387be3ad`.
- Prompt-bar/conductor run on the same trajectory:
  `7855146d-59f0-419a-ab99-3ebb0e28481f`.
- Owner:
  `5bd6de97-3b58-408c-bf89-c42c81b083de`.
- Start time:
  `2026-06-15T12:15:16Z`.
- Gateway sequence: xiaomi `402`, deepseek `402`, then repeated ChatGPT
  successes with `tool_choice=function:edit_vtext`, `text_len=0`, and tool-use
  stops.
- VM interruption: `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19` Firecracker exited
  with `signal: killed` at `2026-06-15T12:17:22Z`.
- Restart recovery: guest runtime passivated the same VText run as `was
  running` at `2026-06-15T12:18:00Z`.
- Later direct active guest `/health` was ready at `10.200.64.2:8085`, but
  authenticated data routes for prompt-bar status, Trace trajectory, and VText
  document listing timed out during probing.

Conjecture delta:

- The prompt seed may be present in metadata and initial VText prompt; visible
  empty V0 is expected for prompt-bar instruction revisions.
- The first real product failure is non-creation of a non-empty V1 and/or stale
  pending-state recovery after VM interruption.
- Repeated `edit_vtext` tool-use responses need tool-result inspection before
  choosing a repair.

Expected next move: extract the failed run's tool result errors, document id,
revision list, mutation row, and trace moments. If product routes remain timed
out, use a read-only, snapshot-safe store inspection route rather than mutating
live VM state.

## 2026-06-15 - Root Cause And First Repair Move

Code review found the mismatch behind the repeated tool-use loop. VText
agent-revision runs selected exact initial `edit_vtext`, but configured terminal
tool successes only for `spawn_agent`, `request_super_execution`, and
`request_email_draft`. `edit_vtext` stores the canonical revision and completes
the pending mutation, but the enclosing tool loop still asked the provider for
another turn. The existing prompt-bar test hid this because its fake provider
returned `end_turn` after seeing the edit.

Repair conjecture: a successful `edit_vtext` must terminate the current VText
agent-revision tool loop. VText may still choose researcher/super/email tools in
the same or a later VText-owned turn, but a canonical document write is not a
prompt for another forced edit cycle. The first code move adds `edit_vtext` to
the VText agent-revision terminal tool set and tightens the prompt-bar VText
test to require a single terminal edit turn.

## 2026-06-15 - H001 Parent/Child Residue Flagged

Review of the investigation language surfaced a live doctrine mismatch: the
runtime logs and code still use parent/child vocabulary (`StartChildRun`,
`ParentRunID`, child-run helpers, child-result channels, and prompt/test
language). This is H001 from Choir Doctrine, not a new discovery.

For this M3.4 repair, use neutral evidence wording such as "VText activation"
or "prompt-bar trajectory" instead of teaching child-run ontology. Do not claim
M3 lifecycle readiness until H001 is resolved or explicitly bounded in the M3
paradoc. Current judgment: H001 is a blocking M3/M4 lifecycle-cutover heresy,
but not the direct cause of the observed first-draft loop unless deployed proof
shows parent/child control is still needed for prompt-bar VText completion.

## 2026-06-15 - Deployed First Repair Failed, Second Repair Scoped

Deployed proof after `60bd2f47c380432a3e55db5f766db6b6f9209bb9` did not settle
M3.4. A browser-driven staging probe submitted a fresh prompt and created:

- Prompt-bar/conductor run:
  `e8bb34ab-8f47-4848-840f-f1b505487f0b`.
- VText activation:
  `793f1e07-27e9-4c96-a33e-96c23ed0ea2d`.
- Owner VM:
  `vm-2915a448148dd7e897e0e7dfa368a424` at `10.200.65.2:8085`.

Gateway logs again showed repeated ChatGPT responses with exact
`tool_choice=function:edit_vtext`, `stop=tool_use`, and two tool calls until the
owner VM restarted and runtime passivated the VText activation. During the loop,
direct product API probes to the guest timed out, so the loop can starve normal
product observability.

Conjecture delta:

- First repair was necessary but insufficient. `edit_vtext` can terminate the
  VText turn only after the tool executes successfully.
- The repeated exact call pattern after the terminal-tool repair points to
  failed structured edit execution, likely from an underspecified payload such
  as content-only `edit_vtext` without `doc_id`, `base_revision_id`, or
  `operation`.
- A VText activation has one pending agent mutation, one current document head,
  and run metadata/channel authority. Runtime can safely default omitted
  `doc_id`, `base_revision_id`, and operation from that context while still
  rejecting explicit target mismatches and stale base revisions.

Second repair: `commitVTextToolEdit` now derives omitted document/base context
from run metadata, VText channel, pending mutation, and current document head;
it defaults content-only edits to `replace_all` and edit-list payloads to
`apply_edits`. Regression coverage adds a prompt-bar product-path test where
VText emits only `{"content": ...}` and must still create a visible appagent
revision, complete the mutation, and stop after one exact `edit_vtext` provider
turn.

Local receipt:

```text
nix develop -c go test ./internal/runtime -run 'TestInitialVTextRunWritesFirstAppagentRevisionThroughEdit|TestInitialVTextRunDefaultsMinimalEditContextFromActivation|TestVTextAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestRunToolLoopTerminalToolSuccessStopsWithoutExtraProviderTurn' -count=1
ok  	github.com/yusefmosiah/go-choir/internal/runtime	1.327s
```

Wider runtime receipt:

```text
nix develop -c scripts/go-test-runtime-shards
```

The full first pass hit one timing-sensitive failure outside the VText edit
surface: `TestVSuperCoSuperSlotReusedByTrajectorySlot` reported a passivated
co-super slot still active. The same test then passed in isolation, and shard 0
passed when rerun directly:

```text
nix develop -c go test ./internal/runtime -run '^TestVSuperCoSuperSlotReusedByTrajectorySlot$' -count=1 -v
ok  	github.com/yusefmosiah/go-choir/internal/runtime	2.744s

nix develop -c env SHARD_INDEX=0 TOTAL_SHARDS=4 scripts/go-test-runtime-shards
ok  	github.com/yusefmosiah/go-choir/internal/runtime	30.551s
```

Residual risk: this reinforces existing H001/coagent-passivation cleanup debt,
but it does not falsify the VText edit-context repair.
