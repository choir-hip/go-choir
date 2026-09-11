# P0 Freeze — Engineering Carrier Mapping, Manifest, Falsifiers, Replay Spec

Frozen 2026-09-11 at base `main@73815790` for Definition
`docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md`, acceptance item
P0-define. This file is the code-free freeze artifact: it records the four
discovered defects, the nine-operation mapping table, the prompt/REPL
initialization manifest with its entropy-exclusion list, the falsifiers, the
replay-harness specification, the chosen model-selection mechanism, and the
frozen desk task. It contains no repair, settlement, or deletion code. The
P0-review panel is bound to this file by base ref, path, and content digest.

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

Receipt classes: `durable_execution` = host-signed persisted receipt;
`transient_observation` / `transient_mutation` = no durable receipt today,
broker `rcpt_*` id discarded (`internal/yaegikernel/broker.go:114-125`);
`selfdev_freeze` / `selfdev_verify` / `read_only_inspection` = self-development
operation receipts; `lifecycle_fate` = assignment-fate saga receipt;
`lifecycle_update` = durable update receipt.

| # | Old JSON name | In-cell successor / intent kind | Receipt class | Canonical identity fields | Exclusion list (nondeterministic) | Fixture path |
|---|---|---|---|---|---|---|
| 1 | `capsule_exec` | `choir.Exec(command, args)` — existing | durable_execution | agent_run_id, capsule handle digest, capsule_id, command, args, cwd, exit_code, stdout_digest, stderr_digest, worktree_digest, source_tree_digest | receipt_ref, occurred_at, duration, session_id | `internal/agentcore/testdata/rlm_replay/capsule_exec/v1/` |
| 2 | `capsule_read_file` | `choir.ReadFile(path)` — existing | transient_observation | path, content_sha256 | broker rcpt_ id, latency | `internal/agentcore/testdata/rlm_replay/capsule_read_file/v1/` |
| 3 | `capsule_write_file` | `choir.WriteFile(path, content)` — existing | transient_mutation | path, content_sha256, mode, written | broker rcpt_ id | `internal/agentcore/testdata/rlm_replay/capsule_write_file/v1/` |
| 4 | `capsule_list_dir` | `choir.ListDir(path)` — existing | transient_observation | path, sorted entries | broker rcpt_ id, pre-canonicalization entry order | `internal/agentcore/testdata/rlm_replay/capsule_list_dir/v1/` |
| 5 | `commit_transaction` | staged Freeze intent — PLANNED (P3-in-cell-surface), not current | selfdev_freeze | operation_id, trajectory_id, base_event_head, content_digest (= bundle_digest), change_count, classifier_version, classifier_digest, groups, state=frozen | handle, event ids/timestamps, staged bundle file path, minted internal ids | `internal/agentcore/testdata/rlm_replay/commit_transaction/v1/` |
| 6 | `inspect_self_development_bundle` | synchronous read-only bundle inspection — PLANNED (P3-in-cell-surface), not current | read_only_inspection | operation_id, bundle_digest, content_digest, source_tree_ref, runtime_artifact_ref, base_event_head, runtime_files (path+sha256, sorted), build_recipe_ref, test_receipts, dependency_toolchain_refs, classifier_version, classifier_digest, groups | execution_receipts payload bodies' occurred_at/duration (compare receipt refs only), resource_receipts volatile fields | `internal/agentcore/testdata/rlm_replay/inspect_self_development_bundle/v1/` |
| 7 | `record_self_development_verification` | staged Verify intent — PLANNED (P3-in-cell-surface), not current | selfdev_verify | operation_id, bundle_digest, decision, sorted verifier_refs, resulting operation state, verifier_run_id | event_id, event timestamp, minted verifier_ref id, idempotency-record internals | `internal/agentcore/testdata/rlm_replay/record_self_development_verification/v1/` |
| 8 | `record_assignment_result` | `choir.Complete(result, verdict, summary, evidenceRefs)` → IntentComplete — staging exists; fate authorship added in P3-settlement | lifecycle_fate | assignment_id, attempt, proposition_digest, disposition, result, verdict, summary, sorted evidence_refs, command_ids (capsule-command:sha256 of receipt refs), output_digests, candidate_id | report_id (partial embeds tool_call_id — substituted by cell intent identity), timestamps, freeze receipt ids, reducer seq | `internal/agentcore/testdata/rlm_replay/record_assignment_result/v1/` |
| 9 | `update_coagent` | `choir.Message(toDesk, kind, body)` → IntentMessage — staging exists; authority parity added in P3-parity | lifecycle_update | update_id (durable derivation), caller_agent_id, target_agent_id, channel_id, trajectory_id, packet kind + normalized packet digest, status | cursor, timestamps, wake/dispatch ids | `internal/agentcore/testdata/rlm_replay/update_coagent/v1/` |

Durable golden receipts captured against the pre-cutover deployed build live at
`docs/evidence/rlm-replay/<operation>-golden-v1.json`.

**Identity substitutions declared up front** (a substitution is a semantic
identity change the replay proof must still honor on the declared fields):

- Rows 2-4 have no durable identity today; the harness supplies the
  caller-supplied semantic identity (§5) and compares returned payload fields.
- Row 8 partial-report identity embeds the provider `tool_call_id`
  (`cosuper_assignment_fate.go:578-600`); the successor derives the equivalent
  identity from the cell intent identity (cell id + local id + content digest,
  `rlm_reduce.go:156-163`). The proposition digest, assignment/attempt identity
  and disposition are invariant across the substitution.
- Row 9's durable update id derivation
  (`tools_worker_update.go:966-1032`) must be reproduced by the staged message
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
   (`tool_profiles.go:230-233`); deterministic file bytes.
5. CoSuper overlay — `RLMCoSuperOverlay()` when `capsule.HostSelectsRLM()`
   (`tool_profiles.go:260-265`), body
   `internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`.
6. Exact-assignment block — assignment_id, kind, subject_digest, and for
   verification source_candidate_id (`tool_profiles.go:266-281`).
7. Run-context overlay — agent id, requester agent id, channel id
   (`tool_profiles.go:286-299`,
   `internal/runtimeprompts/overlays/run_context.yaml`).

The tool loop then appends the deterministic sorted tool catalog
(`internal/toolregistry/toolregistry.go:169-194`, invoked at
`toolloop.go:301-305`) and sends `rec.Prompt` as the initial user message
(`runtime.go:3287-3302`). No model id is read anywhere in this path; the only
route conditional is the actuator (`capsule/actuator.go`,
`tool_profiles.go:261`).

### 3.2 Entropy-exclusion list (exact field paths)

The desk prompt digest is computed over the assembled system prompt plus the
sorted tool catalog, after removing exactly these fields:

- `temporal_context.now_utc` — wall clock.
- `assignment.assignment_id`, `assignment.subject_digest`,
  `assignment.source_candidate_id` — per-run assignment identity.
- `run_context.agent_id`, `run_context.requester_agent_id`,
  `run_context.channel_id`, `run_context.texture_delivery_agent_id` — per-run
  routing identity (texture field is empty for CoSuper).
- The initial user message `rec.Prompt` — excluded from the system-prompt
  digest; the roster fixes the task text separately (§7).

Everything else — core prompt, role prompt, skill context bytes, RLM overlay
body, the static `assignment_kind` value for a fixed desk, and the full sorted
tool catalog including schemas — is INSIDE the digest. `assignment_kind` stays
in because it is constant for a fixed desk; if a roster run ever shows a
different kind the digest must change.

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
   receipt from a fresh one (e.g. every compared field is constant), the
   replay proof for that operation is vacuous and the table is defective.
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
- **Effect census**: count of durable writes, events and receipts attributable
  to the replayed call; replay must produce zero additional effects.
- **Durable receipt store**: `docs/evidence/rlm-replay/<op>-golden-v1.json`,
  bound to the capture build SHA.
- **Legacy capture adapter**: invokes the JSON tool path on the pre-cutover
  deployed build and records the canonical receipt.
- **Successor adapter**: invokes the in-cell successor under the same
  `semantic_identity`.
- **Caller-supplied semantic identity**: in-cell broker calls mint a fresh
  `rcpt_*` request id per call today (`choir.go:84-110`,
  `yaegikernel/broker.go:114-125`); the harness requires a caller-supplied
  semantic identity threaded into in-cell operations. Reused identity with
  changed canonical input must conflict before any effect.
- **State-mutating read fixture**: for read-only operations, mutate the
  underlying state between calls — the same identity returns the original
  observation, a new identity observes the change.
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

> Inside the capsule, working only in /workspace/platform:
> (1) read /workspace/platform/internal/capsule/types.go and report the number
> of fields declared on GoEvalRequest;
> (2) write /workspace/platform/tmp/rlm-desk-task-marker.txt containing exactly
> the line `carrier-check`;
> (3) run `go vet ./internal/capsule` and record its exit code;
> (4) complete the assignment with result=completed, verdict=pass, a summary
> naming the field count and the exit code, and evidence refs containing
> exactly the execution receipt refs of the cells that performed steps 1-3.

Pass requires: the marker file content exact, the vet exit code reported
correctly, the completion carrying the exact receipt references, and no
retired tool name invoked. `hy3` is excluded from any image-bearing step by
name; this task bears no images.

## 8. Binding

- Base ref: `main@73815790`.
- Scoped path: this file only (the Definition `now` update and registry
  entries ride the same commit but are not review target content).
- Review question for P0-review: do the mapping table, receipt classes,
  canonical/exclusion lists, replay-harness specification and prompt/REPL
  manifest support per-operation replay proof and deletion — and where would
  that proof be vacuous?
