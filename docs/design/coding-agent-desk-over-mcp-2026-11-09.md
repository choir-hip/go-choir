# A coding agent as a desk over MCP (2026-11-09)

Status: design, not built. Mutation class of this document: green.
Author: a Sonnet design pass for the owner. Nothing here is settled until the
owner rules on the questions in section 9.

Owner direction, 2026-11-09:

> "Engineering should be delegate first if the delegate exists for it. But
> ... to really make it work, we should follow up with our investigation about
> using MCP as a way to have a coding agent like Claude Code function as a
> desk. So we can do the pre-commitment records protocol via MCP between our
> Go harness and Claude Code, as an example. That way we could just totally
> replace the engineering desk with Claude or another coding agent. Or we
> could also still maintain the delegate, but that way the delegate can send
> intermediate results back to management and texture. I mean, the way the
> delegate currently works, it's not doing that, and that's one of the
> problems with it."

Ruled the same day: nothing may send material straight to Texture around the
desk boundaries. Desks talk only through precommitment records, and each data
kind has one custodian desk.

Labels used below: **[C]** read in code at this tree (file:line given),
**[D]** documented by Anthropic, OpenAI or opencode (URL given, fetched
2026-11-09), **[M]** measured in the earlier investigation, **[I]** inferred,
**[U]** unverified. The earlier investigation is
`docs/evidence/token-efficiency-2026-11-09/followup/claude-delegate-desk.md`
(on the investigation worktree; it is not on main at this tree). Its
measured numbers are quoted, not re-measured.

## 0. Short answer

1. A desk, at the protocol level, is a set of verbs checked against a grant
   set, whose acts become records that a reducer appends to a ledger. A coding
   agent can be a desk if every one of its acts reaches the reducer as the same
   kind of staged intent a cell produces. MCP is a good transport for that.
2. Run a stdio MCP server in the capsule. It is a translator with no
   authority and no credentials. It forwards each tool call to the host, where
   the existing reducer checks and commits it exactly as it checks a cell's
   staged intent. The boundary is the host reducer, not the in-capsule server.
3. Build shape (a) first: the delegate stays under the engineering desk and
   gains four tools, so it can report progress to management through records
   while it runs. Then build shape (b): the coding agent is the engineering
   desk, and the Go-driven DeepSeek loop is not started when a grant and a
   driver exist.
4. Interim results reach management as non-final engineering Reports.
   Management restates to Texture if it wants. Texture renders the ledger. The
   existing engineering-to-Texture progress relay breaks the owner's ruling and
   the no-switch rule; this design does not use it and recommends deleting it
   (section 5).
5. Acceptance stays artifact-verified: Complete cites capsule job receipts the
   executor minted, Freeze freezes the capsule diff on the host, management
   verifies a named bundle digest. The agent's words are a claim.
6. The grant is today a nix list in the gateway, not an owner act on the tape.
   That follow-up is open. It must land before shape (b), because (b) puts the
   owner's money behind every engineering round.

## 1. What a desk is at the protocol level today

### 1.1 The registration row

A desk is a row in `internal/deskmanifest`. Its authority is the union of the
grants it holds; its verbs are computed from them (`grants.go:1-12`, the
catalogue at `grants.go:80-160`). The engineering row is at
`manifest.go:410-428`:

- carrier `CarrierCapsule`, instance per assignment;
- grants `capsule.mutate`, `selfdev.complete`, `selfdev.freeze`,
  `act.precommit` with the rule kinds, `act.report`, `act.ask`, `act.reply`,
  `calibration.read`, `calibration.restate`, and `perform.assignment`.

The grant-to-verb map [C] (`grants.go`):

| Grant | Verbs it adds |
|---|---|
| `act.precommit` | `Precommit` (`grants.go:86`) |
| `act.report` | `Report` (`grants.go:87`) |
| `act.ask`, `act.reply` | `Ask`, `Reply` (`grants.go:82-83`) |
| `calibration.read`, `calibration.restate` | `Calibration`, `Restate` (`grants.go:100-101`) |
| `capsule.mutate` | `WriteFile`, `Exec`, `Start`, `Poll`, `Stop` plus capsule verbs (`grants.go:155`) |
| `selfdev.freeze`, `selfdev.complete` | `Freeze`, `Complete` (`grants.go:156-157`) |
| `perform.assignment` | no verb; it names the one holder of assignments (`grants.go:132`) |

Engineering holds no `act.cast`, `act.escalate`, `act.report_packet` or
`Verify`. It cannot open work, escalate or judge a candidate.
`capsule.mutate`, `selfdev.*` and `perform.assignment` are `Privileged`: no
package may request them (`grants.go:132, 155-157`; `manifest.go:892-896`
refuses capsule verbs to an app desk). This matters for shape (b): a coding
agent cannot be an app desk. It is the first-party engineering desk with a
different driver (section 4.2).

### 1.2 Receiving a cast

Management's Cast to `engineering:<assignment>` opens an assignment and a
conversation. A Cast opens an assignment only on management's records route
(`rlm_reduce.go:1341-1346`; `inbox_records.go:31-33`). The engineering desk
reads it with `choir.Records()`, which returns the cell-start snapshot of
records addressed to this desk in (cursor, head]: Casts, Asks, Notes and
Reports, each an `InboxRecord` with `Seq`, `RecordID`, `Kind`, `From`,
`ConversationID`, `Round`, `Final`, `Objective`, `Body`, `Cast`, `Report`,
`Job`, `ParentID`, `RelatedIDs`, `EvidenceRefs`
(`choir.go:1404-1425`; `intent.go:220-262`). The cast body is
`types.CommitmentCast`: hypotheses, uncertainties, revisions, withdrawals,
captures (`types/commitment.go:468-475`). `choir.Context()` gives the
activation identity (`choir.go:1450`).

The cursor advances only when the cell's acts commit, and only over the
records a `Records()` call actually returned (`choir.go:1404-1408`;
`recordsShown`, `choir.go:91-94`). Delivery is "by cursor, as in Kafka"
(`docs/precommitment-records.md:431-434`).

A capsule job's end is also a record in the same inbox, from `job:<id>`, with
state, exit code, receipt and output tail (`intent.go:255-257`;
`capsule_jobs.go:198-221`). It wakes the run.

### 1.3 Reporting interim results

`choir.Report(toDesk, claim, evidenceRefs, resolverID)` stages an
`IntentReport` (`choir.go:1874-1883`; `intent.go:817-823`). The reducer
validates it needs a target and a body (`rlm_reduce.go:456-461`). For an
engineering sender the reducer forces it to be progress:

> "Engineering's staged Reports are progress (C1e D3): only its Complete, or
> its cancellation, mints the final Report" (`inbox_records.go:405-410`)

So an interim result is `Report("management", claim, evidenceRefs, "")`.
A Report closes the recipient's round only when it is final
(`precommitment-records.md:407-415`). `Ask` and `Reply` are the question
verbs (`choir.go:1766, 1800`); an Ask and Reply inside a round do not close it
(`precommitment-records.md:411-420`). Engineering also states a rule-settled
`Precommit` before its evidence (`choir.go:1844`;
`engineering_precommit_rules.go`), and may `Restate` once when the runtime asks
(`choir.go:1861`; `rlm_reduce.go:195-206`).

Engineering reaches Texture only through management: a staged act from the
performer to the Texture desk is refused at the reducer
(`rlm_reduce.go:133-136`).

### 1.4 Reporting a final result with evidence

Three verbs, in this order for a source-changing implementation assignment.

- `choir.Start` / `choir.Poll` run a command as a capsule job (`jobs.go:224,
  271` in yaegikernel). The executor watches the job (poll interval 2 s,
  `capsule/jobs.go:35-37, 603`), mints its receipt from the broker's status
  and its own worktree digests (`capsule/jobs.go:26-31`), and commits the job
  end record (`capsule_jobs.go:200-221`).
- `choir.Freeze(buildRecipeRef, testReceipts, dependencyToolchainRefs)` stages
  `IntentFreeze` (`choir.go:1157-1171`; `intent.go:476-486`). The reducer
  requires all three non-empty (`rlm_reduce.go:157-160`) and freezes the
  capsule diff on the host using the cell's bound handle, never model input
  (`rlm_reduce.go:1206-1216`).
- `choir.Complete(result, verdict, summary, evidenceRefs, executionRefs)`
  stages `IntentComplete` (`choir.go:1142-1152`; `intent.go:442-474`). Result
  is one of completed, failed, blocked, partial. It must be the last intent of
  its cell (`rlm_reduce.go:145-151`), cannot share a cell with a job start
  (`intent.go:466`), is refused while a job runs
  (`rlm_reduce.go:941-958`), and is refused for a platform-touching
  implementation with no freeze (`rlm_reduce.go:966-984`). A completed pass
  needs at least one execution ref (`rlm_reduce.go:2113-2115`), and the refs
  are resolved by the executor, not by the model: `ResolveExecutionReceipts`
  opens each receipt (`capsule/executor.go:1701-1710`; call at
  `rlm_reduce.go:2116`). Complete mints the final Report to management in the
  same transaction as the assignment fate (`rlm_reduce.go:1183-1204`).

Then management judges. `choir.Verify(assignmentID, decision, verifierRefs,
bundleDigest)` records a verdict against the exact frozen bundle digest
(`choir.go:1176-1190`; `rlm_reduce.go:165-175`). Management starts and judges
before/after shots (`ShotStart`, `ShotVerdict`, `choir.go:1192, 1261`).

### 1.5 How a cell becomes an intent

The harness is host-side. The DeepSeek engineer's model loop runs in
`internal/agentcore`; each cell's Go source goes to the capsule's session
worker (`cmd/capsule-broker/session_worker.go:223, 456`), the cell runs under
`ChoirScope`, and the staged intents come back to the host as the cell
result. The host reducer (`rlm_reduce.go`) validates and commits them. Verb
availability is gated twice: the export gate by desk row
(`choir.go:476-483`, `deskModule` at `choir.go:371`) and the reducer by
sender (`rlm_reduce.go:100-215`). The desk identity and slot come from the
verified outer capability and "the model can never set it" (`choir.go:38-47`).

This is the property to preserve. Every act arrives as `StagedIntent`; the
reducer decides.

## 2. The MCP server

### 2.1 What it is

`choir-desk-mcp`: a new small command, a sibling of `cmd/choir-delegate`.
It speaks MCP over stdio. Claude Code starts it from `--mcp-config` in place
of today's empty `{"mcpServers":{}}` (`internal/claudedelegate/spec.go:307-308`
[C]), with `--strict-mcp-config` kept so no other server loads.

It has no state, no credentials and no model. It does four things: advertise
tools, validate argument shape, forward the call over a per-run Unix socket to
the capsule broker, and return the host's answer as the tool result.

### 2.2 Where authority lives

The honest statement is narrower than "the server holds the authority".

The server runs in the capsule, beside an agent with a Bash tool, so the agent
can reach the server's socket without MCP. Anything in the capsule is
reachable by the agent. Therefore:

- The in-capsule server is a translator and must be treated as untrusted. It
  is a convenience for the agent, not a boundary.
- The boundary is the host. The executor pulls pending desk calls from the
  broker (the same pull pattern as jobs: the executor already polls the broker
  and commits records on the host, `capsule/jobs.go:603`;
  `capsule_jobs.go:198-221`). It identifies the call by the capsule and run
  the broker fixed at spawn, never by anything in the call. A call carries a
  verb and arguments only.
- The host handler builds a one-intent tray and sends it through the same
  reduction a cell's staged intents take. It runs the same checks: the desk
  row's verb set (`choir.go:371-380`), the reducer's sender rules
  (`rlm_reduce.go:100-215`), the engineering-reaches-Texture-only-through-
  management refusal (`rlm_reduce.go:133-136`), the precommit rule validation,
  the Complete guards (`rlm_reduce.go:941-984`), and the assignment binding.
  A refusal returns as the tool result text, as a cell sees it
  (`verbRefusal`, `choir.go:115-120, 286-306`).
- The agent holds no record authority of its own. Whatever it could do by
  calling the socket directly it can do by calling the tool; both land in the
  same reducer. Holding the desk's authority is therefore not a secret the
  agent could steal. It is a grant the host applies per call.
- Credentials stay where they are. The relay secret lives only on the claude
  process as `ANTHROPIC_AUTH_TOKEN` (`spec.go:393` `Env`; investigation 1.1).
  The MCP server is started with the same stripped environment. Freeze keeps
  scanning changed files for the secret.

What the agent can and cannot reach:

- It can read and write the capsule filesystem under its tools (Read, Edit,
  Write, Glob, Grep, Bash), work in `/workspace/platform`, call loopback and
  the model relay and Go proxy (`spec.go` `SystemContext`; capsule egress in
  claude mode, `engineering_assignment_runtime.go:777-790`), and call the
  desk tools.
- It cannot reach any other computer, the gateway credential, other desks'
  data kinds, Texture, the owner, Cast, Escalate, Verify, ShotStart, the apply
  path or vmctl. None of those verbs is in the engineering row or in the tool
  list.
- It cannot choose its identity, its target desk or its conversation. The
  `desk_report` tool takes no `to` argument; the host targets management.

### 2.3 The tools

Names are the MCP tool names. In Claude Code they appear as
`mcp__choirdesk__<name>` [D: tool naming `mcp__<server>__<tool>`,
https://code.claude.com/docs/en/mcp]. Each row says what the host does and
which record it writes. All arguments are typed JSON; free text is bounded.

| Tool | Arguments | Result | Host effect and record |
|---|---|---|---|
| `desk_records` | `since?` (a seq) | the `InboxRecord` list (section 1.2), plus the backlog count | `Records()`. No record written. Marks records shown; the cursor moves with the next committing call, as in a cell (`choir.go:1404-1425`). |
| `desk_context` | none | the activation identity map | `Context()`. No record. |
| `desk_precommit` | `precommit` (typed, with a rule kind) | the record ref | `Precommit`. A rule-settled ledger-only record from `engineering:<assignment>` (`rlm_reduce.go:1243-1246`). |
| `desk_restate` | `target`, `statement` | ref | `Restate`; refused in the cell that stages Freeze or Complete (`rlm_reduce.go:202-212`). |
| `desk_report` | `claim`, `evidence_refs[]` | the record ref and the seq | `Report` to management, always non-final (`inbox_records.go:405-410`). A progress record in the management inbox with provenance naming the acting worker (section 3.1). |
| `desk_ask` | `question`, `req` | ref | `Ask` to management; opens a question the answer to which arrives as a Reply record. |
| `desk_reply` | `target_ref`, `answer` | ref | `Reply` to the asker. |
| `desk_run_checked` | `argv[]`, `timeout_s` | `job_id`, state, exit code, output tail, and the `execution_ref` | `Start` then wait for job end, as one tool call. The executor mints the receipt (`capsule/jobs.go:26-31`). The job end record lands in the inbox. |
| `desk_wait` | `timeout_s` (at most 1500) | the new records since the last read, or "none" | Blocks with MCP progress notifications. No model call while blocked. No record written. |
| `desk_freeze` | `build_recipe_ref`, `test_receipts[]`, `dependency_toolchain_refs[]` | the bundle digest and the saved screens note | `Freeze` (shape (b) only). |
| `desk_complete` | `result`, `summary`, `evidence_refs[]`, `execution_refs[]` | the fate | `Complete` with verdict `none` (shape (b) only). Mints the final Report. |

Shape (a) exposes the first nine minus `desk_freeze`, `desk_complete`, and
minus `desk_precommit` and `desk_restate` (the engineer holds the
assignment's precommit). Shape (b) exposes all eleven.

Notes on specific tools:

- `desk_run_checked` exists because the delegate's own Bash runs are "not
  capsule jobs" and so are not evidence (overlay
  `rlm_engineering_runtime.yaml:74`). Through this tool the delegate's checks
  become receipts. This is the one place the investigation found MCP buys
  something real for the dev loop (investigation 2.5 layer 3). A job cannot
  share a cell with Complete (`intent.go:466`), so the host runs
  `desk_run_checked` as its own reduction and `desk_complete` as another.
- `desk_wait` and long calls must outlast Claude Code's idle abort. A stdio
  tool call with no response and no progress notification for 30 minutes
  aborts; the per-call wall clock defaults to about 28 hours; progress
  notifications reset the idle window but do not extend a per-server
  `timeout` [D, https://code.claude.com/docs/en/mcp "Timeouts"]. The server
  sends a progress notification every 60 seconds. Non-interactive runs are
  never auto-backgrounded unless `CLAUDE_AUTO_BACKGROUND_TASKS=1` [D, same
  page]; the bridge leaves that unset.
- Every result may carry a short suffix: "N new records since your last read:
  <seq, kind, one line each>". Records ride tool results. This is the
  agent-neutral wake (section 3.3) and it appends to the transcript, which is
  cache-safe.
- Tool descriptions are fixed by code. An MCP server that "changes the tools
  it offers" can send `list_changed`, which `-p` honours for the tool list
  only [D]. The server never sends it: the tool set is constant for a run.

### 2.4 Claude Code flags this design needs [D]

Fetched 2026-11-09 from https://code.claude.com/docs/en/cli-reference and
/headless:

- `--mcp-config <json-or-file>`: loads servers. With `-p` Claude Code waits
  for pending servers before the first turn, up to `MCP_TIMEOUT` (30 s
  default). **That wait requires Claude Code v2.1.221 or later.** The tree
  pins 2.1.197 (`spec.go:293`). So step 1 includes a version decision or a
  probe that 2.1.197 connects a stdio server (check `mcp_servers` in the
  `system/init` event; a skipped entry shows in `mcp_server_errors`).
- `--strict-mcp-config`: only servers from `--mcp-config`.
- `--tools` restricts built-ins and "doesn't affect MCP tools; to deny those
  too, use `--disallowedTools "mcp__*"`". The allowlist is therefore
  `--allowedTools` naming each `mcp__choirdesk__*` tool (and the existing six
  built-ins).
- `--permission-mode bypassPermissions` is what the delegate runs under
  today (`spec.go:319`). It auto-approves MCP calls. The host reducer, not the
  Claude permission prompt, is the check.
- `--input-format stream-json` makes the process long-lived and accepts
  queued user messages; "with `--input-format stream-json`, a message still
  queued when the limit ends a turn stays queued and starts a new turn with
  its own limit" [D, cli-reference `--max-turns`].
- `--max-budget-usd` is a client-side estimate that "can pass the cap"
  [D]. `--bare` skips CLAUDE.md, hooks and MCP discovery and loads only
  `--mcp-config` servers [D, headless]. It needs an API key and ignores OAuth,
  so it is unusable while the credential is the owner's Claude login
  (section 6).
- `--permission-prompts none` needs v2.1.259 [D]. Not needed under
  `bypassPermissions`.
- `claude -p` loads a project's `.mcp.json` without a prompt [D]. The
  capsule's tree has none, and `--strict-mcp-config` excludes it anyway.

Unverified here [U]: the exact line shape the CLI expects on stdin under
`--input-format stream-json`. The SDK documents
`{"type":"user","message":{"role":"user","content":"..."}}`
(https://code.claude.com/docs/en/agent-sdk/streaming-vs-single-mode). Whether
several queued lines become several turns is not stated; the investigation
listed it as test E2.

## 3. Two shapes, compared

### 3.1 Shape (a): the delegate stays under the engineering desk and gains tools

What changes:

- `--mcp-config` names `choir-desk-mcp`. Tools: `desk_records`, `desk_context`,
  `desk_report`, `desk_ask`, `desk_reply`, `desk_run_checked`, `desk_wait`.
- The DeepSeek engineer still owns the assignment: it holds Freeze, Complete,
  Precommit, the grant to spend, and the cell loop.
- A record the delegate writes is the engineering desk's record. The delegate
  has no inbox of its own. It goes out from `engineering:<assignment>` with
  provenance naming the acting worker. There is precedent: a management task
  acts as management with `ActingTask` and `ActingRunID` stamped by the host
  (`types/commitment.go:198-202`; `inbox_records.go:385-389`). Add
  `ActingWorker` (the delegate run id, the session id) the same way. The
  claim text is delegate-authored and untrusted at the reader, labelled so.
- An interim Report reaches management during the run. This fixes the stated
  defect: today the delegate runs "in its own process", the engineer is parked
  on `choir-delegate wait --park` (overlay lines 74-76), and no record leaves
  the capsule until the engineer's next cell.
- `desk_ask` writes a record to the engineer's own inbox and signals the
  run's job wake (`signalJobWake`, `capsule_jobs.go:236-246`), so a parked
  engineer wakes for a delegate question. This costs one DeepSeek turn per
  question.

What is deleted: nothing of substance. A small amount of overlay text changes
(item 4 asks the engineer to name the run, `rlm_engineering_runtime.yaml:78`).
`choir-delegate` stays.

What stays: everything else. The engineer's cell loop, the completion guard
(`runtime.go:4805-4821`), the delegate server, `cmd/choir-delegate`.

Cost per round [M] and [I], from the investigation: today the DeepSeek
engineer is 100-143 calls, 10.4-14.8M input tokens, **$0.64-1.32 list, mean
$0.94**, and the delegate adds **$0.10-0.28** on the owner's Claude plan.
With the delegate-first prompt and one acceptance job (investigation Part 2,
not yet built), the engineer is 8-14 calls, **$0.03-0.06** and the delegate
about **$0.30-0.70** [I]. Shape (a) adds the tool schemas to the delegate's
prefix (about seven short schemas; with MCP tool search the definitions are
deferred and only names enter context [D, investigation 3.1]) and
a few extra turns. Estimate: **no measurable change in money**; unmeasured.

Failure modes:

- The engineer is the single point of review and its cost floor remains
  (8-14 DeepSeek calls per round).
- The delegate's interim claim and the engineer's final claim can disagree.
  Management sees both. That is correct, not a defect; the ledger keeps both.
- A delegate that spams `desk_report` wakes management repeatedly. Bound it:
  at most one progress Report per five minutes per run at the host, the rest
  refused with the reason. This is a rate rule in the handler, not a switch.
- The investigation's open risk stands: another in-flight dependency on a
  Claude Code feature set that moves fast.

Acceptance: unchanged. The engineer cites receipts it or `desk_run_checked`
produced, Freezes, Completes. `desk_run_checked` receipts are executor-minted
and so count.

Crash restart: the capsule dies; the broker's delegate session dies with it
("a session never crosses assignments", `claudedelegate/server.go:307-311`);
the executor closes open jobs "interrupted by a restart"
(`capsule/jobs.go:827-854`); boot closes the assignment and the round
(CLAUDE.md "Restarts End Work"; `precommitment-records.md:441-443`). Records
already committed stand. No resume.

Another agent: the tools are plain MCP. Section 3.4.

### 3.2 Shape (b): the coding agent is the engineering desk

What changes:

- On an assignment with a grant and a driver, the executor starts the coding
  agent in the capsule as the assignment's performer. No Go-driven model loop
  runs for it. The cast text and records reach the agent through
  `desk_records` and the first user message.
- All eleven tools are exposed. The agent precommits, works with its own
  Read/Edit/Bash, runs checks through `desk_run_checked`, freezes, completes.
- The driver is a package-level choice of the engineering desk (plan MA4.10,
  "Engineering as a package, last", `modular-architecture-plan-2026-11-04.md`
  section 9.1): a capsule driver registered with a name, `claude-code` first.
  The desk row does not change; it is still `engineering`, with the same
  grants. This respects I26: one engineering desk, same registration path,
  different implementation (`choir-doctrine.md:397-418`).
- The host runs the Cast's acceptance checks itself at `desk_complete` and
  cites that receipt (section 3.5).

What code is deleted when (b) ships and the DeepSeek loop is retired for
delegate computers [I, to be confirmed by grep at step 8]:

- the engineer prompt overlay for the delegate, about 30 KB of YAML
  (`rlm_engineering_runtime.yaml`, the "Claude Delegate" block lines 70-78
  and the delegate conditionals) [C line cites];
- the engineering completion guard and job-wait resume
  (`runtime.go:4805-4821`; `capsule_jobs.go:301-380`) for the delegate path;
- `cmd/choir-delegate` and its park/poll protocol (`cmd/choir-delegate`), since
  the host drives the process directly;
- the parked-wait pattern and its Poll-only turns.

What stays: the capsule and executor; jobs and receipts; Freeze; management
Verify; shots; the reducer and all its guards; the assignment ledger; the
broker; the Claude relay and egress route. The DeepSeek engineer stays as the
performer when no grant exists (section 6).

Cost per round [I]: no engineer calls. Delegate-only about **$0.30-0.80**
[I from the investigation's $0.0107-0.0129 per delegate turn, [M]; the
exploration turn count is a guess]. River spend for engineering goes to about
zero; the Claude plan spend carries the round. Against shape (a) the saving
is the engineer's $0.03-0.06 and 8-14 DeepSeek calls, a small amount of
money and a large amount of latency and moving parts. Against today it is the
investigation's ~15-20x River cut. The money moves from a metered, capped
budget to the owner's Claude grant, which is why section 6 is a prerequisite.

Failure modes, honest list:

- **No second reader.** Today the engineer reads the delegate's work (and
  re-proves it, wastefully). In (b) nobody on the Go side reads it. The
  independent checks are the host acceptance job, scope checks against the
  Cast's `paths` (`written_files` already exists, `claudedelegate/server.go`
  status), Freeze's platform-path and blank-screen refusals, and management's
  Verify and shot judgement. A wrong-but-green change is caught only if the
  acceptance job and management's judgement can see it. This is the real risk
  of (b).
- **Bypass permissions plus a long-lived agent.** The agent can run any
  command in the capsule for up to the wall clock. The capsule is the sandbox
  (`spec.go:312-318`). The risk is unchanged from the delegate today, but the
  agent now also holds the assignment's lifecycle verbs.
- **Compaction rewrites earlier bytes.** Claude Code compacts and clears tool
  results [D, costs page, per the investigation 3.4]. Harmless for a
  30-120 minute capsule run; wrong for Texture. Fine for engineering.
- **Version drift.** The pinned 2.1.197 predates the MCP startup wait
  (v2.1.221). Upgrading the pin is a separate risk.
- **Cost is not capped per assignment.** The gateway caps tokens per computer
  per day (50M, 2 in flight, `anthropic_relay.go:53-73, 340-357`
  `grantState`) and `--max-budget-usd` can overshoot [D]. One runaway round
  can spend a computer's day. Add a per-capsule counter (investigation 3.3).
- **Single-writer assumptions in the store.** The assignment ledger assumes
  the performer's cells commit sequentially. Tool calls from a coding agent
  can be concurrent (parallel tool calls). The host handler serializes per
  assignment. Unverified [U] against the store's `engineering_assignments`
  checks; step 6 proves it.

Acceptance and crash restart: sections 3.5 and 3.6.

### 3.3 How wakes reach the agent

Two channels. Neither is a resumer.

1. **Records ride tool results.** Every desk tool result ends with the new
   records since the last read. This works with any MCP-capable agent. It does
   not wake an agent that is thinking and calls no tool, but such an agent is
   working, and the next call carries the news.
2. **Claude Code only: `--input-format stream-json`.** The host writes one
   user message per wake to the live process. The message appends to the
   conversation, so the request keeps its cached prefix. The process stays
   alive between wakes; the owner's login gets the one-hour main-conversation
   cache TTL, API and cloud keys get five minutes unless
   `ENABLE_PROMPT_CACHING_1H=1` [D per the investigation 3.2]. If the process
   dies, `--resume <session>` re-enters inside the same capsule. Do not
   enable channels: they are a research preview, ignored under `-p`, and
   unavailable on cloud providers [D, investigation 3.1].

A cancellation is not a message the agent may ignore. Management's
`CancelAssignment` (`choir.go:1822`) kills the process group and closes the
assignment on the host. A stop is a correctness act, not a record to read.

### 3.4 Behaviour with Codex and opencode

Nothing in sections 2.2-2.3 is Claude-specific, because the tools are plain
MCP over stdio and every inbound path has the tool-result form.

- **Codex CLI [D, https://learn.chatgpt.com/docs/extend/mcp?surface=cli,
  fetched 2026-11-09].** MCP servers go in `[mcp_servers.<name>]` in
  `~/.codex/config.toml` or a project `.codex/config.toml`, with `command`,
  `args`, `env`, `cwd`, `startup_timeout_sec` (default 10) and
  `tool_timeout_sec` (default **60**). `enabled_tools` and `disabled_tools`
  filter tools; `default_tools_approval_mode` takes `auto`, `prompt`,
  `writes`, `approve`. Consequences: the host must set `tool_timeout_sec`
  above `desk_wait`'s cap (or cap `desk_wait` at 50 s for Codex), and
  `desk_run_checked` must return a job id rather than block for minutes, then
  be polled. **Unverified [U]:** whether non-interactive `codex exec` loads
  `mcp_servers`; the page does not say.
- **opencode [D, https://opencode.ai/docs/mcp-servers/, fetched
  2026-11-09].** `mcp.servers.<name>` with `"type": "local"`, `command` as an
  array, `environment`, and `timeout` as an object with `startup`, `catalog`
  and `execution`. Tools can be denied by a permission rule on the normalized
  `<server>_<tool>` name. "Code Mode" is on by default and exposes server
  tools through code; `"codemode": false` exposes them natively. A tool call
  interface through code is close to how our own cells work, which may suit
  the protocol well. **Unverified [U]:** non-interactive `opencode run` MCP
  loading and its wake behaviour.
- **Wake.** Neither has a verified stdin-push equivalent. Both therefore use
  channel 1 (tool results) and `desk_wait`. Per-tool-call timeouts differ, so
  the host's tool set takes a per-driver `max_blocking_seconds`.
- **Driver per agent.** The driver supplies the argv, the config file, the
  environment, the credential route (the model relay or the agent's own),
  how to read cost, and how to stop it. The tools and the reducer do not
  change. The driver is a package of the engineering desk (MA4.10).
- **Credential.** The Claude relay swaps a per-capsule secret at the egress
  proxy and the gateway holds the credential (`capsule/claude_delegate.go:19-67`
  [C]). A Codex or opencode driver needs its own relay route and its own
  grant; they are not covered by today's `/provider/anthropic` route.
  Model-call money must be metered at the gateway for any driver.

### 3.5 Acceptance stays artifact-verified

The rule (standing question 6): a round is complete when its artifact exists
and is fetched, not when the agent narrates success. In both shapes:

1. Receipts are minted by the executor from the broker's status and its own
   worktree digests (`capsule/jobs.go:26-31`). The agent cannot write one.
   `Complete` resolves each cited ref through the executor and fails if one
   does not resolve (`rlm_reduce.go:2116`).
2. Freeze runs on the host over the capsule diff (`rlm_reduce.go:1206-1216`).
   It refuses a diff with no screens unless docs-only and refuses platform
   paths the Cast did not name (investigation 2.5; `SystemContext` in
   `spec.go`). It scans changed files for the relay secret
   (`capsule/claude_delegate_linux.go`).
3. Management verifies a specific bundle digest (`choir.go:1176-1190`) and
   judges shots (`choir.go:1192, 1261`). The owner approves apply.
4. **New in (b): host-run acceptance.** `desk_complete` does not trust the
   agent's `execution_refs`. The handler first runs the Cast's `checks` list
   as one capsule job it starts itself (the investigation's "one combined
   acceptance job", Part 2.5 layer 1), then stages Complete citing that
   receipt plus any the agent cited. A failing acceptance job turns a
   `completed` result into a refusal the agent reads in the tool result.
   The agent's own `desk_run_checked` runs are advisory evidence.
5. Scope: `written_files` against the Cast's `paths`. A write outside is a
   refusal at `desk_freeze`.

In (a) the engineer plays that role as it does today.

### 3.6 Crash restart

Owner rule: a crash never resumes work (CLAUDE.md "Restarts End Work";
`manifest.go:100-104` `RestartContract`).

- The agent process, the MCP server and the host's pending-call queue die
  with the capsule. Nothing is persisted that could resume them.
- Jobs open at boot are closed "interrupted by a restart"
  (`capsule/jobs.go:827-854`). Boot closes open obligations including the
  engineering assignment and its round with a system record that wakes no one
  (`precommitment-records.md:441-443`; `appdesk/required.go:25`).
- A planned restart never happens while a capsule is live (CLAUDE.md,
  "Even these are never restarted ... while an engineering capsule is live"),
  so there is no marker-backed resume to design. The driver registers no boot
  hook and no resumer.
- A tool call in flight at the crash is lost. If the host had committed the
  record before the crash, it stands; if not, it is not retried, because the
  agent that would have retried is gone.
- The agent-side session (`--resume`) lives under the capsule's `/tmp`. It
  does not outlive the capsule. A new round after a crash is a new Cast with a
  fresh brief.

## 4. The two shapes compared, in one table

| | (a) delegate gains tools | (b) the agent is the desk |
|---|---|---|
| Fixes "delegate sends no interim results" | yes | yes |
| New code | `choir-desk-mcp`, host call queue, provenance field, rate rule | same, plus driver, host acceptance job, freeze/complete tools, per-capsule budget |
| Code deleted | none | engineer overlay for delegate computers, job-wait guard path, `choir-delegate` park protocol (after step 8) |
| DeepSeek calls per round | 8-14 [I] (100-143 today [M]) | 0 |
| Money per round | engineer $0.03-0.06 + delegate $0.30-0.70 [I] | delegate $0.30-0.80 [I] |
| Second reader of the work | the engineer | none; host acceptance, scope, Freeze, management Verify |
| Risk | low | medium; money is owner's grant; no second reader |
| Works with Codex, opencode | the delegate only; a second delegate driver | yes, as a driver, subject to [U] items |
| Reversible | revert | revert; DeepSeek engineer stays the fallback performer |

Recommendation: ship (a) first as steps 1-5, measure, then (b) as steps 6-7
once the grant is an owner act (step 0 of money, section 6). Do not skip (a):
it proves the bridge, the provenance and management's use of interim reports
with the engineer still in the loop.

## 5. Interim results to management and Texture without crossing boundaries

Path:

1. The agent calls `desk_report`. The host reducer commits a non-final Report
   record from `engineering:<assignment>` to management's inbox, with acting
   worker provenance. Same transaction as the cursor advance
   (`inbox_records.go:405-410, 690-735`).
2. Management reads it with `Records()` at its next wake. Management decides:
   pass it through, restate it, or ignore it. Doctrine already says
   management "with Jev decides whether to pass an engineering report through
   or restate it" (`precommitment-records.md:424-426`).
3. If management wants Texture to know, management addresses Texture with its
   own record. Management sends records and scores to a Texture and never
   modifies its document (`precommitment-records.md:422-426`). Texture, the
   custodian of document revisions, decides what to do with the delivery.
4. Texture also renders the commitment ledger as the owner-facing
   supervision surface (CLAUDE.md "Desk Ontology"). That is a read of the
   ledger through the custodian's surface, not a delivery of engineering
   material into Texture's inbox.

**A conflict in the current code, found while reading.**
`engineeringProgressRelays` mints a platform-authored Note into the Texture
inbox for every non-final engineering Report, from `progress:<assignment>`,
in the same transaction (`engineering_progress_relay.go:29-61`). Its header
says "Engineering still addresses management only", and the relay is "the
ledger projected into the inbox of the one desk that renders it"
(`engineering_progress_relay.go:17-23`). It is also gated by a setting,
`engineering.progress_to_texture` (`:25-26, :42`), which is a switch in the
sense of CLAUDE.md "No switches".

Against today's ruling this is material sent to Texture around the
management boundary, and it is a switch. This design does not use it and does
not depend on it. Because shape (a) would multiply the Reports flowing
through it (a delegate may report several times a run), turning the relay on
with an MCP-capable delegate would send delegate-authored, untrusted text into
Texture's inbox with only a system label. Recommendation: delete the relay,
and let Texture's supervision surface read the ledger. Open question Q3.

Taint note: a delegate-authored claim is produced by a model that read the
repository, which is untrusted input. Management reads it as untrusted. The
`ActingWorker` provenance lets the reader apply that label.

## 6. The money boundary

Today [C]:

- The Claude delegate grant is a nix list of computer ids,
  `claudeDelegateComputers` (`nix/node-b.nix:94`), with an expiry window
  (`nix/node-b.nix:95`, `claudeDelegateExpires`), fed to the gateway as
  `GATEWAY_ANTHROPIC_RELAY_IDENTITIES` (`nix/node-b.nix:2592`) and to the tool
  profile. The gateway answers "this computer has no Claude delegate grant" or
  "the Claude delegate window is closed" or "this computer spent its Claude
  delegate budget for the day" (`gateway/anthropic_relay.go:340-357`).
- The comment on that list names the follow-up: "an owner act per computer
  (owner, switch audit 2026-11-03, group 7) replaces this list"
  (`nix/node-b.nix:80-82`). The problem doc records the intended shape:
  proxy route, signed receipt, host grant store read by the gateway and vmctl,
  tape record, revoke (`docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md:156-166`).
  **Not built at this tree.** The owner-act receipt machinery exists
  (`internal/owneract/owneract.go:1-12`: an ed25519-signed receipt from the
  proxy's host-owner-act key) and other acts use it
  (`internal/platform/owner_acts.go`), but no delegate-grant act does.
- The relay holds the credential on the host, per computer. A cap of
  50M tokens a day applies per computer, two calls in flight
  (`anthropic_relay.go:53-73`).
- Whether the delegate exists for a given Cast is read at admission:
  `engineeringCastNetwork(in.Statement, rt.capsuleExecutor.ClaudeDelegateStatus().Available)`
  (`engineering_assignment_runtime.go:383, 784-790`). The engineer prompt is
  rendered with or without the delegate block (`rlm_engineering_runtime.yaml:9, 70`).

Design:

1. **The grant is an owner act on the tape, never a switch.** The act is: the
   owner, authenticated by the proxy, grants computer C the Claude delegate
   with a daily token cap and an expiry. The proxy signs an owner-act receipt
   over (computer, cap, expiry, driver kinds). The host grant store keeps it;
   the gateway reads it; the computer's tape carries a record mirroring it.
   Revoke is a second act. This is the already-named follow-up, and shape (b)
   depends on it. A computer's own tape cannot be the authority for a
   host-held credential ("guest root writes that tape", problem doc `:156`),
   so the host store is the authority and the tape is the mirror.
2. **Every driver is a grant kind.** The grant names which driver kinds it
   covers (`claude-code`, later `codex`, `opencode`). A Codex round is not
   covered by a Claude grant.
3. **The cap is per capsule as well as per computer.** One assignment is one
   capsule. A runaway cannot spend the computer's day.
4. **When no grant exists for a round**, there is no coding-agent performer
   and no delegate tools. The engineering desk performs the round with its
   Go-driven cell loop, as it does for every computer off the list today.
   The assignment's binding records which performer ran, so the ledger can
   say so. This is the existing availability read
   (`engineeringCastNetwork`), not a new flag: a grant is an owner act, and
   the absence of one is an absence of authority. It is a correctness
   refusal of the coding-agent path, not a runtime switch. Management is told
   in the assignment's first record which performer was admitted.
5. **A grant that lapses mid-round** (expiry, revoke, daily cap) makes the
   relay refuse model calls at once (`anthropic_relay.go:340-357`). The agent
   process then fails its next turn. The driver reports `blocked` with the
   relay's reason, the assignment closes with that reason, and nothing
   retries. The reason reaches management through the Report, in plain words,
   as the delegate-not-attested reason does today
   (`capsule/claude_delegate.go:60-62`).
6. **SEC-08 residual.** Anthropic's terms are recorded as forbidding a
   service that intermediates a subscription token; the owner carries that
   residual (`claude_delegate_token...md:176-188`). A standing engineering
   desk makes more calls a day. The relay takes a Console key with no code
   change. Recommend moving to a Console key before shape (b) (Q6).

## 7. Build order

Each step is small and provable on d@e, the test computer, with its own
rollback. Classes follow CLAUDE.md "Mutation Classes". Tiers follow the
risk-tier rule; if the data-system tempo rule is still in force for a step,
runtime checks are the verifier. Every step lands through the Landing Loop
(commit, push, CI, deploy, staging identity, deployed proof). A step that
changes platform behaviour first lands a problem or ledger entry.

| # | Step | Class | Proof on d@e | Rollback |
|---|---|---|---|---|
| 0 | Free probes, no code: (i) read one stored `events.jsonl` and settle whether `--setting-sources user` loads CLAUDE.md; (ii) run pinned `claude` 2.1.197 by hand in a d@e capsule with a one-tool stdio MCP server and read `mcp_servers` and `mcp_server_errors` in `system/init`; (iii) test whether two queued stdin lines become two turns | green | the three answers recorded in this doc's ledger | none needed |
| 1 | Delegate-first prompt and one host acceptance job (the investigation's Part 2). Not new here; a prerequisite | orange | a d@e round with engineer under 15 iterations, same gates (investigation E1) | revert the overlay |
| 2 | `choir-desk-mcp` and the host call queue, read tools only: `desk_records`, `desk_context`, `desk_wait`. Pull via the executor. Bump the pin only if probe (ii) fails | orange | a d@e delegate run calls `desk_records` and returns the Cast; a hand-forged socket call with another assignment's id is refused; a forged verb is refused; trace shows each call | revert; `--mcp-config` returns to `{"mcpServers":{}}` |
| 3 | `desk_report` with `ActingWorker` provenance and the 5-minute rate rule | red, additive, one-act rollback: tier 1 | a d@e round where the delegate's first Report reaches management's `Records()` within 60 s and before the delegate ends; the ledger shows a non-final record from `engineering:<id>` with acting worker; the Texture inbox gets nothing from engineering or `progress:` (assert zero) | revert; the tool disappears; records already written stay |
| 4 | `desk_ask`, `desk_reply`, wake via `signalJobWake`, tool-result record suffix | orange | delegate asks a question mid-run; the parked engineer wakes, answers; both are records; cost of the wake recorded | revert |
| 5 | `desk_run_checked`: the delegate's checks as receipts. Serialize per assignment | red (receipts are evidence): tier 2 | Freeze cites a receipt the delegate produced and it resolves; a forged ref is refused by `ResolveExecutionReceipts`; the engineer's re-run tail is gone from the trace | revert; the engineer re-runs as today |
| 6a | Grant as an owner act (proxy route, signed receipt, host store, gateway read, tape mirror, revoke) and delete the nix list. Its own mission | red/black-adjacent: money; two seats | grant, use, revoke on d@e; the gateway refuses within the cache window; the tape record exists | restore the nix list |
| 6b | Driver `claude-code` for the engineering desk, `desk_freeze`, `desk_complete`, host acceptance at complete, per-capsule budget. The performer is chosen at admission from the grant read | red, tier 2 (new concurrency, boot path, evidence) | a full self-development operation on d@e with no DeepSeek engineer run: assignment, capsule, interim Reports, freeze, management Verify, shots, owner approval; with the grant revoked the same Cast runs on the Go-driven engineer | revert; the Go-driven engineer is the performer |
| 7 | Crash drill for (b): kill the guest mid-round | red | the assignment and round close "interrupted by a restart", no process resumes, no record is retried, the open job is closed | revert 6b |
| 8 | Second driver, `opencode` or Codex, same tools. Settle the [U] items first | orange | one d@e round; same ledger shape; per-driver blocking limits honoured | remove the driver |
| 9 | Delete: the delegate-conditioned engineer overlay, job-wait path for delegate rounds, `choir-delegate`, and the progress relay (Q3) | red, delete: tier 2 | grep for citers is empty (standing question 3); a d@e round runs; the gates pass | revert |

Parallelism: steps 2-5 are one package boundary (`cmd/choir-desk-mcp`,
`internal/claudedelegate`, the executor pull, one reducer entry). Step 6a
touches proxy, gateway, nix and vmctl; build it in parallel with 2-5.

## 8. Pre-flight questions (docs/standing-questions.md)

1. **Who settled this, owner or orchestrator?** The direction (delegate
   first; MCP desk; replace or keep the delegate; interim results) is the
   owner's, 2026-11-09, quoted above. The tool set, the host-pull design, the
   shape ordering and the acceptance job are this document's proposals,
   unratified. The no-Texture-bypass ruling is the owner's. The
   "owner act" for the grant is the owner's (switch audit 2026-11-03, group 7,
   per `nix/node-b.nix:80-82`).
2. **Does the topology conform to every settled decision?** Checked:
   I26 (one registration path, desks talk only through records, custodian per
   kind; `choir-doctrine.md:397-418`); the Desk Ontology in CLAUDE.md;
   engineering reaches Texture only through management
   (`rlm_reduce.go:133-136`); no switches (the performer is read from the
   grant, not a setting); capsules for self-development candidates; restart
   rule. One conflict found and named: the existing progress relay
   (section 5). One tension named: modular plan MA4.10 puts engineering as a
   package "last"; this design makes the driver a package concern and lands
   after MA4.9 for step 6b. If MA4.10 is not yet reached, 6b lands against the
   built-in row, which MA4.10 later migrates.
3. **What still references what is deleted?** Step 9 deletes the overlay
   block, the job-wait path for delegate rounds, `cmd/choir-delegate` and the
   progress relay. Citers to grep and repair in that commit: the overlay
   (`rlm_engineering_runtime.yaml`), `runtime.go:4805`, `capsule_jobs.go`,
   `docs/design/claude-delegate-slice1-2026-10-14.md`,
   `delegate-first-for-source-changes-2026-11-02.md`,
   `delegate-one-round-build-2026-10-21.md`, `internal/types/round_receipt.go`
   (matches `choir-delegate` by string, `:39`), `engineering_delegate_use.go`,
   `types/engineering_assignment.go:488-533` (`DelegateUse`), and the setting
   `store.SettingEngineeringProgressToTexture`. Not yet grepped across
   `docs/`, `specs/` and `skills/` [U]; step 9 does it.
4. **Who consumes this in production?** The Claude delegate is used by the
   computers on the nix list today; the earlier investigation measured two
   live theme rounds. The new tools have no consumer until step 2. The
   progress relay's consumers: none known; it is off unless its setting is on
   [U: which computers have the setting on is not read here].
5. **What is the single authority for this state?** Records and assignment
   fate: the ledger via the host reducer. The agent's transcript is not
   state. The grant: the host grant store once step 6a lands; the nix list is
   the authority today. Performer choice: recorded once on the assignment
   binding at admission. The in-capsule server holds no state, so nothing
   there can disagree.
6. **What artifact proves this success claim?** Executor-minted receipts,
   the host-run acceptance job, the frozen bundle digest, management's
   Verify and shots, and for the design itself the d@e proofs in section 7.
   The agent's `result` and summary are claims.
7. **What does this read path fate-share with?** `desk_records` and
   `desk_report` need the capsule, the broker socket, the executor's pull and
   the host reducer, all healthy. That is acceptable: engineering is
   capsule-bound by definition. The durable product state (the records, the
   assignment fate) is on the tape and servable without the capsule. Texture's
   ledger render reads the tape, not the capsule. The model call additionally
   depends on the gateway and the relay.
8. **What happens on restart?** Section 3.6. The call queue, the MCP server,
   the agent process and its session are erased and not resumed. Committed
   records and receipts stand. Acceptable by the owner rule.
9. **Could an agent do this without SSH?** The tools are the product path.
   Diagnosis: the run directory's `events.jsonl` and `status.json` through
   the existing delegate status surface, `/api/runtime/observability` for
   `daily_tokens_used` and the cap, and a new per-driver cost line in
   `/api/costs`. The grant, once an owner act, is granted and revoked through
   the product API / `choir` CLI under a scoped key, not by editing nix.
   Registry hygiene: this change adds a design doc; the registry contract
   (ACTIVE.md, mission-graph.yaml, doc-authority-manifest.yaml) applies when a
   mission authors steps 2-6, not to this design.

## 9. Open questions for the owner, each with a recommendation

1. **Order: (a) before (b), or straight to (b)?** Recommend (a) first
   (steps 2-5), (b) after step 5 shows interim Reports used by management and
   receipts accepted. (a) is cheap and proves the bridge.
2. **When no grant exists, does the Go-driven engineer remain the
   performer?** Recommend yes, indefinitely. It is the fallback, the reference
   implementation and the performer on every computer the owner has not
   granted. The coding agent is a driver, not a replacement of the desk.
3. **Delete the engineering-to-Texture progress relay?** Recommend yes. It
   sends engineering material toward Texture around management and sits behind
   a setting, against both rulings. Replace it with Texture's supervision
   surface reading the ledger. Until deleted, do not enable the setting on any
   computer that runs shape (a).
4. **Does `desk_complete` run the Cast's checks itself?** Recommend yes
   (section 3.5 item 4). The agent's own runs are advisory. This removes the
   32-iteration re-verify tail without trusting the agent.
5. **Who is the author of a delegate-origin record?** Recommend: the
   engineering inbox, with `ActingWorker` provenance and an untrusted label,
   following the management-task precedent. No new inbox for the delegate.
6. **Console key instead of the owner's Claude login before (b)?** Recommend
   yes. A standing desk raises call volume; SEC-08 is already the owner's
   named residual. The relay supports it without a code change. A Console key
   also means a 5-minute cache TTL by default unless
   `ENABLE_PROMPT_CACHING_1H=1` is set [D].
7. **Bump the pinned Claude Code (2.1.197)?** Recommend: probe first (step 0
   ii). Bump only if 2.1.197 does not connect a stdio server under `-p`; the
   documented startup wait needs v2.1.221.
8. **Wake channel for Claude: tool-result suffix only, or also stdin push?**
   Recommend suffix first (agent-neutral, no new process mode), stdin push
   as a measured optimisation after test E2.
9. **Per-assignment spend cap and rate rule values?** Recommend: a per-capsule
   cap of one tenth of the computer's day, and one progress Report per five
   minutes. Both are the host's, both are numbers to tune from d@e traces.
10. **Is a coding agent allowed to be a desk beyond engineering?** Recommend
    no for now (investigation 3.5): research wants search, read and send in
    one cell; Texture and management want an append-only prefix the harness
    controls. This design is for engineering only.

## 10. What I could not verify

- Whether the pinned Claude Code 2.1.197 connects a stdio server under
  `-p` without the v2.1.221 wait. Step 0 (ii).
- The exact stdin line shape for `--input-format stream-json` in the CLI
  (the SDK form is documented; section 2.4).
- Whether queued stdin lines become separate turns (test E2).
- Whether `codex exec` and `opencode run` load MCP servers non-interactively,
  and how either is woken.
- Whether the store's assignment ledger tolerates concurrent tool-call
  reductions. The design serializes per assignment; step 5-6 proves it.
- Whether management is woken by a non-final engineering Report mid-run. The
  record lands in management's inbox (`inbox_records.go:405-410`), but the
  wake rules for a mid-run progress record were not traced
  (`WakeRecordsInbox`, `inbox_records.go:1006-1060`). Step 3's proof checks
  it with a ledger trace, not an inference.
- The investigation's measured numbers are from its own worktree at main
  5dc98bcf5, not re-run here. Costs in sections 3.1 and 3.2 marked [I] are
  estimates until step 1 and step 6b run.
- Which computers have `engineering.progress_to_texture` on.

## 11. Owner rulings (2026-11-09 evening)

Asked one at a time. These override sections 3, 5, 7 and 9 where they
differ; the build follows them.

1. **Order (Q1): straight to shape (b).** The coding agent is the standing
   engineering desk. Shape (a) is not built as a step. Build order section 7
   is re-cut: the MCP server and the desk tools, then the standing desk, with
   the bridge and the desk proved together on d@e.
2. **Fallback (Q2): the Go-driven engineer stays, indefinitely**, on every
   computer without a grant.
3. **Interim updates to Texture stay (Q3), and Jev routes them.** Owner:
   "What's supposed to happen here is that Jev the System 1 model is
   supposed to determine whether to pass through the relay or to call
   management. The desk language model agent. It's definitely essential
   that we send interim updates to Texture. Texture is the supervision
   service." So the relay is not deleted. What goes is its setting: for
   each progress record Jev judges one of two outcomes, pass it through to
   Texture, or wake the management desk's model. No switch selects the
   behaviour. Section 5 and step 9's deletion of the relay are withdrawn;
   the judgment, its prompt, its cost record and its failure mode (Jev
   unavailable: pass through and record that no judgment ran) are to be
   designed in the build's failure list.
4. **Completion (Q4): the checks are management's, and Jev decides whether
   they are enough.** The acceptance commands come from the assignment, not
   from the agent. Owner: "I don't think it helps to just run the commands
   that the coding agent tells you to run. I think it makes more sense to
   have Jev get 50K tokens or whatever of code into its context and then it
   can answer: should we just run the commands, or should we escalate to the
   management desk language model agent?" So `desk_complete` does two
   things: the runtime runs the assignment's checks itself in the capsule
   (section 3.5 item 4 stands; the agent cannot report its own pass), and
   Jev reads the change (about 50,000 tokens of the diff and its context)
   and answers "the checks suffice" or "escalate to management". A pass
   needs both. The later gates are unchanged: management verifies the
   bundle, the shots are judged, the owner approves the apply.
5. **Billing (Q6): the owner's Claude login**, not a Console key. SEC-08
   stays the named residual.
6. **Limits (Q9): no spending cap; track only.** Progress reports are not
   rate-limited by the clock. Owner: "more progress reports are better ...
   the frequency of progress reports can scale with the duration of work,
   but they really shouldn't be done on clock time, but on semantic
   progression." The desk tool's guidance asks for a report at each real
   step (a finding, a decision, a file set done, a check passing or
   failing); Jev's routing in ruling 3 is what keeps management from being
   woken by each one.
7. **Not asked; the recommendations stand until ruled otherwise:** Q5 (the
   author of a delegate-origin record is the engineering inbox, with
   acting-worker provenance and an untrusted label), Q7 (probe the pinned
   Claude Code before bumping it), Q8 (wake by tool-result suffix first),
   Q10 (engineering only).
