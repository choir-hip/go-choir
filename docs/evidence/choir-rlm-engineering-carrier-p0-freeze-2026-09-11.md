# P0 Freeze — Engineering Carrier Mapping, Manifest, Falsifiers, Replay Spec

Frozen 2026-09-11 at base `main@73815790` for Definition
`docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md`, acceptance item
P0-define. This file is the code-free freeze artifact: it records the four
discovered defects, the nine-operation mapping table, the prompt/REPL
initialization manifest with its entropy-exclusion list, the falsifiers, the
replay-harness specification, the chosen model-selection mechanism, and the
frozen desk task. It contains no repair, settlement, or deletion code.

**Revision 2 (2026-09-11, post-P0-review):** the convergent panel adjudicated
`repair`; confirmed findings are folded into §2, §3 and §5 below. Panel: 11 ok
of 13 (omp-gemini38 provider failure, omp-nemotron-3-ultra timeout, devin
partial — no final verdict). Review record: `.agentic-consensus/p0-review/`
(process diagnostics; adjudication folded here). Folded findings: the
receipt-class taxonomy correction, row-1 reclassification, row-3/7/8/9 field
corrections, the capture-entropy-vs-replay-identity split, per-operation
conflict semantics, the P3 scope items the table exposes, the parameter-surface
gaps frozen for R8, and the harness anti-vacuity rules.

## 1. The four defects (evidence-bound)

**D1 — The eval envelope is not minimal.** `capsule_go_eval` declares two
interchangeable string parameters (`source`, and `code` described as "Alias for
source."), an empty `required` list, and a silent source-wins fallback
(`internal/agentcore/tools_capsule.go:746-773`). A call with neither parameter
executes an empty cell; a call with both silently resolves to `source`. Neither
case fails at the schema boundary. The handler then calls
`yaegikernel.CleanGoSource` (`tools_capsule.go:773`), which strips markdown
fences from model-authored Go (`internal/yaegikernel/eval.go:301-317`) at five
call sites: `tools_capsule.go:773`, `eval.go:122`, `eval.go:148`,
`session.go:86`, `cmd/capsule-broker/session_worker.go:392`.

**D2 — Run acceptance keys on tool names.** The `capsule_effect_frozen` and
`capsule_verification_recorded` checkpoints are built by scanning
`EventToolResult` events for results named `commit_transaction` and
`record_self_development_verification`, each behind a `len(results) > 0` guard
(`internal/agentcore/run_acceptance.go:627-645`). The collector skips error
results and empty outputs (`run_acceptance.go:281-313`), so retiring a tool
silently removes its checkpoint instead of failing the run.

**D3 — The reducer is a message router, not a settlement authority.**
`ReduceCellIntents` commits Message, Spawn and Complete only as mailbox
envelopes (`internal/agentcore/rlm_reduce.go:150-198`), while
`record_assignment_result` runs the assignment-fate saga: proposition digest,
slot gate, freeze/revoke ordering, lifecycle CAS, late-evidence and
cancellation races (`internal/agentcore/cosuper_assignment_fate.go:18-45,
525-810`; store commit `internal/store/cosuper_assignments.go`). The run loop's
detached-terminal predicate keys on the tool's name
(`internal/agentcore/runtime.go:3391-3393`,
`cosuper_assignment_fate.go:19-22`), and the admission grammar special-cases it
(`internal/toolregistry/batch_executor.go:22-46,202-214`).

**D4 — The verifier slot is unreachable on the assigned path.** Both
verifier-gated tools require `RunRecord.Metadata["co_super_slot"] ==
"verifier"` (`tools_capsule.go:410-412,491-493`). The assigned CoSuper runtime
writes `assignment_kind` but never `co_super_slot`
(`internal/agentcore/cosuper_assignment_runtime.go:342-355`), and the generic
CoSuper activation path is refused outright
(`internal/agentcore/runtime.go:987-988`,
`tool_profiles_authority_test.go:208-228`). No test pins either gate. The
capsule request carries only source, cwd, allowed packages, timeout and inbox
(`internal/capsule/types.go:101-113`); the broker passes only the agent role to
the session worker (`cmd/capsule-broker/session_worker.go:26-33,274-289,336,
386`); `ChoirScope` carries read-only state but no role or slot and exports
only computer and activation identity
(`internal/yaegikernel/choir.go:20-61,265-284`).

## 2. Frozen nine-operation mapping table

The nine operations are the tools-branch assigned registry
(`TestAssignedCoSuperBuilderIsExactClosedSet`,
`internal/agentcore/tool_profiles_authority_test.go:110-124`) minus
`capsule_go_eval`, which is the retained envelope. Rows 1-4 are the deferred
capsule file/exec operations (residue R8; NOT deleted in this mission — the
mapping is frozen now so their in-cell successors are already exercised).
Rows 5-9 are deleted in this mission, each behind its replay proof.

Receipt classes: `durable_execution` = host-authored, content-addressed
persisted receipt (NOT signed — `internal/capsule/executor.go:645-727`
performs no signature operation); `transient_observation` /
`transient_mutation` = no durable receipt on either path — the JSON tool's
result and the in-cell return are compared as harness-derived canonical
payloads (the broker `rcpt_*` id is minted per call and discarded:
`internal/yaegikernel/broker.go:114-125,372-376`, `choir.go:84-110`);
`selfdev_freeze` / `selfdev_verify` / `read_only_inspection` =
self-development operation receipts; `lifecycle_fate` = assignment-fate saga
receipt; `lifecycle_update` = durable update receipt. Rows whose successor is
PLANNED are honestly marked: their replay proof cannot run until
P3-in-cell-surface lands, and rows 6-7 additionally cannot be proven until
P3-parity makes the verifier slot reachable (defect D4).

**Capture entropy vs replay identity.** Fields minted at first execution
(receipt_ref, verifier_ref, report_id, freeze receipt ids, update cursor,
reducer seq) are nondeterministic at capture time but ARE the replay identity:
on replay the successor must return the ORIGINAL values. The exclusion lists
below therefore exclude only true entropy (timestamps, durations, session
ids, wake/dispatch metadata); minted-then-stable identifiers stay canonical.
Canonical fields must come from the returned result or the durable store —
never copied from the request (a field copied from the fixture is not replay
evidence).

| # | Old JSON name | In-cell successor / intent kind | Receipt class | Canonical identity fields | Exclusion list (true entropy only) | Fixture path |
|---|---|---|---|---|---|---|
| 1 | `capsule_exec` | `choir.Exec(command, args)` — existing | transient_mutation (the in-cell broker path returns `ExecResult{exit_code,stdout,stderr,duration_ms}` with NO receipt_ref — `internal/yaegikernel/broker_protocol.go:54-59`; the durable `capsule-exec:sha256:` receipt exists only on the host executor path) | command, cwd, exit_code, stdout_sha256, stderr_sha256 (harness-derived digests of returned bytes; broker output truncation caveat applies). Parameter-surface gap frozen for R8: legacy accepts `args`, `cwd`, `timeout_ms`; `choir.Exec` accepts only `(command, args)` and the persisted receipt binds `Command`/`Cwd` but not `Args` (`internal/capsule/types.go:136-150`, `executor.go:707-712`) — equivalence is declared over the supported default surface only | broker rcpt_ id, duration_ms | `internal/agentcore/testdata/rlm_replay/capsule_exec/v1/` |
| 2 | `capsule_read_file` | `choir.ReadFile(path)` — existing | transient_observation | path, content_sha256 (harness-derived from returned content) | broker rcpt_ id | `internal/agentcore/testdata/rlm_replay/capsule_read_file/v1/` |
| 3 | `capsule_write_file` | `choir.WriteFile(path, content)` — existing | transient_mutation | path, content_sha256, bytes_written. Parameter-surface gap frozen for R8: legacy accepts `mode`; `choir.WriteFile` never passes it and the broker defaults 0o644 (`choir.go:158-167`, `broker.go:259`) | broker rcpt_ id, mod_time (wall clock, `broker.go:268-271`) | `internal/agentcore/testdata/rlm_replay/capsule_write_file/v1/` |
| 4 | `capsule_list_dir` | `choir.ListDir(path)` — existing | transient_observation | path, entries (already sorted — `os.ReadDir` order, `broker.go:290-298`) | broker rcpt_ id | `internal/agentcore/testdata/rlm_replay/capsule_list_dir/v1/` |
| 5 | `commit_transaction` | staged Freeze intent — PLANNED (P3-in-cell-surface), not current | selfdev_freeze | canonical fields are loaded from the persisted operation/bundle record, NOT the tool envelope — the already-frozen short-circuit returns only `{handle,bundle_digest,operation_id,state}` (`tools_capsule.go:306-313`): operation_id (stable per trajectory binding), trajectory_id, base_event_head (environment binding — replay valid only on an unmoved chain; the fixture pins the head), content_digest (= bundle_digest), change_count, classifier_version, classifier_digest, groups, state=frozen | handle, event ids/timestamps, staged bundle file path | `internal/agentcore/testdata/rlm_replay/commit_transaction/v1/` |
| 6 | `inspect_self_development_bundle` | synchronous read-only bundle inspection — PLANNED (P3-in-cell-surface), not current; gated on P3-parity verifier slot | read_only_inspection | operation_id, content_digest (the receipt emits `content_digest`, not `bundle_digest`, `tools_capsule.go:454-464`), source_tree_ref, runtime_artifact_ref, base_event_head, runtime_files (path+sha256, sorted), build_recipe_ref, test_receipts, dependency_toolchain_refs, classifier_version, classifier_digest, groups | execution_receipts: compare receipt REFS only, never bodies (bodies embed occurred_at); resource_receipts are strings — compared verbatim | `internal/agentcore/testdata/rlm_replay/inspect_self_development_bundle/v1/` |
| 7 | `record_self_development_verification` | staged Verify intent — PLANNED (P3-in-cell-surface), not current; gated on P3-parity verifier slot | selfdev_verify | canonical fields from the persisted operation/event record, NOT the tool envelope — the replay short-circuit omits decision/verifier_refs (`tools_capsule.go:513-524`) and the returned bundle_digest is post-finalize, not the input: operation_id, decision, resulting operation state, verifier_ref (the exact stable verifier-event reference distinguishes replay from reconstruction) | event_id, event timestamp, verifier_run_id (per-run; embedded in the idempotency payload `tools_capsule.go:529-537`), input verifier_refs and input bundle_digest (request fields, not receipt fields), idempotency-record internals | `internal/agentcore/testdata/rlm_replay/record_self_development_verification/v1/` |
| 8 | `record_assignment_result` | `choir.Complete(result, verdict, summary, evidenceRefs)` → IntentComplete — staging exists; fate authorship added in P3-settlement | lifecycle_fate | canonical fields from the persisted fate/report store record, NOT the tool envelope or the Go return: assignment_id, attempt, proposition_digest, disposition, result, verdict, summary, sorted evidence_refs, command_ids and output_digests as STORED report fields (never re-derived — receipt_refs hash occurred_at, `executor.go:645-656,707-717`), candidate object, replay flag (`CoSuperAssignmentCommandResult.Replay`, `types/cosuper_assignment.go:527-534` — read from the store, absent from the tool JSON `tools_capsule.go:938-939`); terminal report_id is deterministic (TerminalReportID over owner/computer/assignment/attempt/proposition digest, `cosuper_assignment_fate.go:590`) and stays canonical; freeze receipt ids and lifecycle command receipt fields are replay identity and stay canonical | partial report_id (embeds tool_call_id — substituted by cell intent identity), timestamps, wake metadata | `internal/agentcore/testdata/rlm_replay/record_assignment_result/v1/` |
| 9 | `update_coagent` | `choir.Message(toDesk, kind, body)` → IntentMessage — staging exists; authority parity added in P3-parity | lifecycle_update | update_id (durable derivation), caller_agent_id, target_agent_id, channel_id, trajectory_id, packet kind + normalized packet digest, durable cursor (replay identity) | timestamps, wake/dispatch ids, call-disposition `status` (`submitted` on first execution vs `existing` on replay — normalized, `tools_worker_update.go:288-293`) | `internal/agentcore/testdata/rlm_replay/update_coagent/v1/` |

Durable golden receipts will be captured against the pre-cutover deployed
build at `docs/evidence/rlm-replay/<operation>-golden-v1.json` (future tense —
neither that directory nor `internal/agentcore/testdata/rlm_replay/` exists
yet; both are P4-harness outputs).

**Deletion prerequisite (hard gate):** no row-5-9 tool name is deleted until
ALL of: P3 settlement parity landed, P3 acceptance migration landed, the
deployed negative acceptance proof recorded, and that operation's P4 golden
receipt + replay proof recorded.

**P3 scope items the table exposes** (frozen here so P3 cannot shrink them):

- Row 8: `StagedIntent` carries one `EvidenceRefs` list; the tool separates
  `evidence_refs` from `execution_refs` and hard-requires execution refs for a
  terminal completed pass (`tools_capsule.go:882-911`). P3 must widen the
  Complete intent (or add a field) to carry execution refs, and must add a
  partial-report path — the tray accepts only completed/failed/blocked
  (`intent.go:94-107`, `rlm_reduce.go:118-124`) while the tool accepts
  `partial` (`tools_capsule.go:882`).
- Row 8: the admission grammar special-cases `record_assignment_result` by
  name (`internal/toolregistry/batch_executor.go:22-46,202-214`); P3 deletes
  or rewrites those cases for IntentComplete in the same change.
- Row 8 conflict semantics: the settled proposition identity intentionally
  excludes the summary, so a reworded summary under the same proposition
  replays rather than conflicts (`internal/store/cosuper_assignments.go:
  154-169,204-230`). The generic "changed input under same identity conflicts"
  rule does NOT apply to summary-only changes; conflict applies to changes in
  the canonical proposition fields.
- Row 9: the durable update id is scoped to the non-lifecycle desk path —
  `deriveWorkerUpdateID` (`tools_worker_update.go:1009-1030`) needs no
  tool_call_id and is reproducible from staged-intent + authority identity;
  the lifecycle branch (`deriveLifecycleProducerUpdateID`, `:966-990`)
  hard-requires the provider tool_call_id and is out of the desk's scope.
  Because the update id is content-derived, a NEW caller identity with
  identical content is not necessarily a fresh operation — the "new identity
  ⇒ fresh operation" rule does not apply to row 9. P3-parity must also carry
  the typed packet schema — `IntentMessage` has only `{ToDesk, MsgKind, Body}`
  (`intent.go:100-121`) versus the tool's typed packet
  (`tools_worker_update.go:170-178`).
- Rows 6-7: replay proof is blocked until P3-parity makes the verifier slot
  reachable AND the pinning tests for both gates exist.

**Identity substitutions declared up front** (a substitution is a semantic
identity change the replay proof must still honor on the declared fields):

- Rows 1-4 have no durable identity on either path; the harness supplies the
  caller-supplied semantic identity (§5) and compares canonical payload
  fields. Their replay proofs are conformance proofs (payload equality under
  one identity, state-mutating read fixture), not durable-receipt replays —
  and they gate nothing in this mission since rows 1-4 are deferred to R8.
- Row 8 partial-report identity embeds the provider `tool_call_id`
  (`cosuper_assignment_fate.go:593-595`); the successor derives the equivalent
  identity from assignment/attempt + proposition digest
  (`cosuper_assignment_fate.go:576-583`) — NOT the tray-scoped
  `rlm:<cell>:<local>:sha` key (`rlm_reduce.go:156-163`), which changes on
  every replay cell and would write a second envelope. The proposition digest,
  assignment/attempt identity and disposition are invariant across the
  substitution. `execution_refs` fold into `choir.Complete`'s evidenceRefs
  carrying `capsule-go-eval:`/`capsule-exec:` receipt refs; the successor
  adapter reads the store fate/report, not the Go return.
- Row 9's durable update id derivation
  (`tools_worker_update.go:1009-1030`) must be reproduced by the staged message
  path; the reducer's current `rlm:<cell>:<local>:sha` idempotency key is
  tray-scoped and is NOT the durable update id.

## 3. Prompt / REPL initialization manifest and entropy-exclusion list

### 3.1 Assembled prompt order (assigned engineering CoSuper, RLM route)

`systemPromptForRun` (`internal/agentcore/tool_profiles.go:195-302`) builds, in
order:

1. Core prompt — `promptStore.LoadCore()`, default
   `internal/promptstore/defaults/core.yaml`.
2. Temporal context — `runtimeprompts.TemporalContext`, embeds `NowUTC`
   (`tool_profiles.go:217-220`).
3. Role prompt — `promptStore.Load(ownerID, "engineering")`, default
   `internal/promptstore/defaults/engineering.yaml`.
4. Skill context — optional, `rt.skillContextForProfile(profile)`
   (`tool_profiles.go:230-233`); a 1200-byte extract plus the ABSOLUTE
   `Source:` path (`internal/agentcore/skill_context.go:17-45,58-69`) — the
   absolute path is host-path entropy, not desk-constant.
5. CoSuper overlay — `RLMCoSuperOverlay()` when `capsule.HostSelectsRLM()`
   (`tool_profiles.go:260-265`), body
   `internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`.
6. Exact-assignment block — assignment_id, kind, subject_digest, and for
   verification source_candidate_id (`tool_profiles.go:266-281`).
7. Run-context overlay — agent id, requester agent id, channel id
   (`tool_profiles.go:286-299`,
   `internal/runtimeprompts/overlays/run_context.yaml`).

The tool loop then appends the tool catalog — deterministic in REGISTRATION
order, not sorted (`internal/toolregistry/toolregistry.go:169-194`, invoked at
`toolloop.go:301-305`) — and sends `rec.Prompt` as the initial user message
(`runtime.go:3287-3302`). Tool schemas travel separately via
`registry.Definitions()` (`toolregistry.go:159-167`), not inside the catalog
text. No model id is read anywhere in this path; the only route conditional is
the actuator (`capsule/actuator.go`, `tool_profiles.go:261`). A second
assembly path concatenates the user message INTO the system string
(`tool_profiles.go:303-316`, used at `runtime.go:3664`), so "user message
excluded" is path-dependent — the digest spec names which path it covers.

Known served-prompt defects the P4 overlay rewrite must fix (panel finding):
the RLM overlay's worked example reads `ctx["assignment_id"]`
(`rlm_engineering_runtime.yaml:22`) but `Context()` exposes only
`computer_id` and `activation_id` (`choir.go:265-284`), and the overlay lists
`choir.Assign` among staged intents (`:52`) although `Assign` is synchronous
(`choir.go:190-200`).

### 3.2 Entropy-exclusion list

The desk prompt digest is computed over a canonical tuple: the assembled
system prompt, the catalog text, and the canonical JSON of the complete
`Definitions()` set including schemas (in registry order — the catalog text
alone carries only names + 80-char truncated descriptions and would NOT move
when P2 deletes the `code` alias). The prompt is a flat rendered string, so
exclusions are TEXTUAL masks over named dynamic fragments, not structured
field paths:

- The temporal-context line (`Current UTC date/time: <RFC3339>`,
  `internal/runtimeprompts/overlays/temporal_context.yaml`) — wall clock.
- The exact-assignment block's `assignment_id`, `subject_digest`, and
  `source_candidate_id` values — per-run assignment identity.
- The run-context overlay's agent id, requester agent id, and channel id
  values — per-run routing identity (the texture delivery field is
  Researcher-only and empty for CoSuper).
- The skill-context `Source:` absolute path — host-path entropy; the skill
  CONTENT digest (or basename) stays inside the digest, and the configured
  skills-root identity is pinned in `environment`.
- The initial user message `rec.Prompt` — excluded from the system-prompt
  digest; the roster fixes the task text separately (§7).

Everything else — core prompt, role prompt, skill content digest, RLM overlay
body, the `assignment_kind` value, the catalog text, and the tool definitions
including schemas — is INSIDE the digest. `assignment_kind` stays in: it is
constant for a fixed desk task, and an implementation run vs a verification
run digesting differently is by design.

Digest equality proves prompt equality only. It does NOT prove
model-selection equality: the replay fixture's `environment` additionally
records effective provider, model, reasoning effort, max tokens, and
policy-overlay identity, and the roster compares those fields separately.

### 3.3 REPL initialization manifest

`NewSession` (`internal/yaegikernel/session.go:53-72`) constructs the Yaegi
interpreter and calls `i.Use(buildFilteredSymbols(...))` once; there is no
host-injected prelude cell. The prebound package key is `choir/choir`,
imported by model code as `import "choir"` (`choir.go:113-135`). Exports:
always `ReadFile`, `ListDir`, `Context`, `Inbox`; non-read-only scopes add
`WriteFile`, `Exec`, `Assign`, `Message`, `Outcome`, `Spawn`, `Complete`
(`choir.go:120-137`). `Context()` reports `computer_id` and `activation_id`
only (`choir.go:265-284`). Each cell gets an inbox snapshot and fresh tray via
`BindCell` (`choir.go:64-82`). The session worker config carries computerID,
epoch, activation, allowedRoot, allowed packages, timeout, and role
(`cmd/capsule-broker/session_worker.go:26-33,274-291`); role arrives on a
trusted worker flag, never from model input (`choir.go:36-40`). Nothing in
session construction or per-cell binding reads a model id.

## 4. Falsifiers

1. If any retired operation cannot reproduce its original receipt through the
   new path under the same semantic identity, that operation is not deletable
   in this mission.
2. If the expected roster cannot pass the frozen desk task on one shared
   prompt after three shared revisions, the one-prompt invariant is false as
   stated; the mission records `blocked_incomplete` rather than tuning around
   it.
3. If more than one code path can author the assignment fate, the settlement
   transfer is incomplete and P3 has failed regardless of test colour.
4. If a mapping-table canonical field set cannot distinguish a replayed
   receipt from a fresh one (e.g. every compared field is constant, or every
   compared field is copied from the request), the replay proof for that
   operation is vacuous and the table is defective.
5. If the entropy-exclusion list hides a per-model prompt difference, the
   digest-equality proof is vacuous and the manifest is defective.
6. If a run without in-cell freeze/verify evidence can still report an
   accepted level, the acceptance rebuild has failed.

## 5. Replay-harness specification

Built in P4-harness before any golden capture. Components:

- **Fixture schema** (versioned JSON, `fixture_version: 1`): `operation`,
  `semantic_identity`, `request` (canonical input), `environment` (computer,
  run, assignment, operation ids as applicable), `expected_receipt` (canonical
  fields only), `exclusions_applied`, `captured_at`, `capture_build_sha`.
- **Canonicalizer**: per-operation field projection per §2; arrays sorted
  where order is not semantic; digests normalized to lowercase hex; the
  declared exclusion list applied; output is canonical JSON plus its sha256.
  Never raw byte equality.
- **Effect census**: per receipt class, a named vector of durable witnesses —
  event ranges, operation transitions, lifecycle command rows, update rows,
  receipt artifacts, mailbox rows, frozen-bundle files, and worktree
  digest/metadata — each bound to the semantic identity. For row 3 the census
  MUST include a filesystem-write invocation witness (mtime/inode or a
  controlled sentinel): a repeated `WriteFile` always rewrites and returns the
  same byte count (`broker.go:255-271`), so before/after content alone cannot
  distinguish replay from re-execution. Transient re-execution detection
  additionally requires a host-side dispatch/admission journal keyed by
  semantic identity (an attempt counter or controlled sentinel effect for
  write/exec). Replay must produce zero additional effects on every
  component.
- **Durable receipt store**: `docs/evidence/rlm-replay/<op>-golden-v1.json`,
  bound to the capture build SHA.
- **Legacy capture adapter**: invokes the JSON tool path on the pre-cutover
  deployed build and records the canonical receipt.
- **Successor adapter** (class-specific contract): invokes the in-cell
  successor under the same `semantic_identity`. Transient rows must prove
  state observation/mutation through the real successor, not fixture replay;
  mutation rows must census the actual filesystem effect; durable rows must
  compare the stored replay receipt and effect vector; every reused identity
  is checked before dispatch. For durable rows the successor-returned
  receipt/ref must be resolvable independently from its durable store — a
  fixture-only run validates the canonicalizer but never authorizes deletion
  by itself.
- **Caller-supplied semantic identity**: in-cell broker calls mint a fresh
  `rcpt_*` request id per call today (`choir.go:84-110`,
  `yaegikernel/broker.go:114-125`); the harness requires a caller-supplied
  semantic identity threaded into in-cell operations. Reused identity with
  changed canonical input must conflict before any effect — where "canonical
  input" means the operation's declared canonical fields, per the
  per-operation conflict semantics in §2 (row 8 summary-only changes replay;
  row 9's content-derived id means a new caller identity is not necessarily
  fresh).
- **State-mutating read fixture**: for transient mutable reads (rows 2, 4),
  mutate the underlying state between calls — the same identity returns the
  original observation, a new identity observes the change. Immutable
  integrity inspection (row 6) is different: corrupting backing bytes must
  make a NEW identity fail verification, while same-identity replay returns
  the prior attestation.
- **Rewarm procedure**: a forced actor/host rewarm control reachable without
  SSH; if none exists, the owner-executed break-glass step is named in the
  harness evidence.
- **Fixture-vs-live declaration**: P4 declares explicitly which operation
  classes are proven by recorded fixture instead of live capture, and why.
  The open question (start.unknowns): whether self-development operations
  (rows 5-7) can be staged with effects OFF for golden capture, or must be
  proven by recorded fixture. This artifact does not assume either answer.

## 6. Model-selection mechanism (chosen)

Owner-visible **model-policy overlay id on the assignment path**, per the
settled owner decision (`now.decision`, receipt
`engineering-carrier-owner-decisions-2026-09-11`). Mechanism, mirroring the
existing coagent/Texture path (`internal/modelpolicy/model_policy.go:18-31,
121-151,256-299`; `internal/coagentowner/spawn_tool.go:27-54,135-139`;
`internal/textureowner/api_texture_prompt_eval.go:21-135`):

1. Add `model_policy_overlay_id` to the `assign_co_super` schema and to
   `StartAssignedCoSuperRequest`
   (`internal/agentcore/tools_cosuper_assignment.go:35-70`,
   `cosuper_assignment_runtime.go:51-57`).
2. Include it in the request digest / durable binding so replay and conflict
   identity cover it (`cosuper_assignment_runtime.go:91-122,223-242`).
3. Set `llm_policy_overlay_id` in the run metadata before `EnrichMetadata`
   (`cosuper_assignment_runtime.go:342-355`), pre-resolving and failing
   closed on policy errors like the Texture eval path.
4. The owner's runtime direction — model policy set inside the Go RLM code so
   new models need no code update — is recorded product direction, not this
   mission's mechanism.

## 7. Frozen desk task (P5-roster)

Canonical task artifact: this file, this section
(`docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md` §7).
The task text served to every roster run, verbatim:

Frozen bytes are pinned in-repo at
`docs/evidence/rlm-roster/p5-frozen-desk-task.txt`
(sha256 `a0386c97d302b748e0b708a21176e9daabee406176d4294b8d514464215b560b`,
1051 bytes). That file is the `--task-file` argument to `choir roster
preflight|start|collect`; the blockquote below is a copy of those bytes, and
the file is authoritative if they ever diverge.

> Inside the capsule, working only in /workspace/platform:
> (1) read /workspace/platform/internal/capsule/types.go and report the number
> of fields declared on GoEvalRequest;
> (2) write /workspace/platform/tmp/rlm-desk-task-marker.txt containing exactly
> the line `carrier-check`;
> (3) run `go -C /workspace/platform vet ./internal/capsule` (choir.Exec runs
> `exec.CommandContext` directly — no shell, no cwd parameter, so the command
> must be a single self-contained invocation) and record its exit code;
> (4) complete the assignment with result=completed, a summary
> naming the field count and the exit code, and execution_refs containing
> exactly the execution receipt refs of the cells that performed steps 1-3.
> (Implementation reports carry no verification verdict: report `verdict=none`.
> Revision 3 amends the as-frozen `verdict=pass`, which
> `CoSuperAssignmentReport.ValidateAgainst` refuses on implementation
> assignments — the frozen text as written could never pass. The amendment is
> record-shape only; steps 1-3 and the pass criteria are unchanged.)

Pass requires: the marker file content exact, the vet exit code reported
correctly, the completion carrying result=completed with verdict=none and the
exact receipt references, and no retired tool name invoked. `hy3` is excluded
from any image-bearing step by name; this task bears no images.

## 8. Binding

- Base ref: `main@73815790` (artifact committed at `655e7c2b`; revision 2
  folds the adjudicated P0-review findings).
- Scoped path: this file only (the Definition `now` update and registry
  entries ride the same commit but are not review target content).
- Review question for P0-review: do the mapping table, receipt classes,
  canonical/exclusion lists, replay-harness specification and prompt/REPL
  manifest support per-operation replay proof and deletion — and where would
  that proof be vacuous? Panel answer: repair — the confirmed defects are
  fixed in revision 2; the residual vacuity notes are recorded per row.

- Revision 3 (2026-09-12): §7 step 4 amended `verdict=pass` → `verdict=none`
  per the P5 roster-CLI design panel (implementation reports must carry
  verdict none); record-shape only, steps and pass criteria unchanged.
