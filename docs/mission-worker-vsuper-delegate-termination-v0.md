# MissionGradient: Worker Vsuper Delegate Termination v0

Status: ready for execution
Date: 2026-05-16
Operator: outer Codex supervising Choir through staging, git, CI, deploy, and product-path evidence

## One-Line Goal String

```text
/goal Run docs/mission-worker-vsuper-delegate-termination-v0.md as a Codex-operated MissionGradient mission: repair or precisely isolate the worker-vsuper tool-loop termination and evidence gap that now blocks super -> worker VM -> vsuper delegation; first audit and correct runtime parameters that still encode a short single-agent loop, including raising maxToolLoopIterations from 25 to 200 while planning the path to 1000 or budget-governed no-fixed-cap execution, make delegate_worker_vm, Trace, and run acceptance preserve failed and pending worker-run event evidence, then tune vsuper/co-super termination behavior so worker runs either produce export/reviewable candidate evidence or return a precise blocker before exhausting their budget; land required platform fixes through git/CI/deploy, rerun a visible staging prompt-bar Choir-in-Choir sweep-shaped workload, and stop only when VText, Trace, and run acceptance show vsuper coordinating worker and verifier co-super agents over channels with export/promotion candidate evidence, or record a lower-level worker-runtime blocker with worker run events, compaction/continuation evidence, rollback refs, residual risks, and the next safe probe.
```

## Research Basis

The prior recovery mission landed and deployed `a4aec9899131c73f663bad72255072a442a2483e`, which kept Trace live while delegated worker runs are pending and made run acceptance record a `delegate_worker_vm` invocation that lacks a terminal result.

Staging proof from `.gstack/evidence/worker-vsuper-recovery-postfix-2026-05-16T14-23-53-011Z` showed:

- trajectory `604d1a20-e17c-4654-b970-b755f1363849`
- VText document `12ce3e0f-45a6-4257-a30c-857e6ce25075`
- worker `worker-63a0091fd0178731`
- worker VM `vm-b6a2f00e8380b57fb709bb549fe6cd78`
- run acceptance `runacc-34df1bed42c4dc5a1729`
- initial delegated worker invocation stayed Trace-live with no terminal result
- the first delegated worker run failed with `tool loop: exceeded 25 iterations without end_turn`
- a manual retry with `timeout_seconds: 1200` was invoked and still pending at proof cutoff

Relevant code observations:

- `internal/runtime/toolloop.go` enforces `maxToolLoopIterations = 25` and returns an error when an agent never emits `end_turn`.
- `internal/runtime/tools_vmctl.go` fetches worker run events only after a completed worker run, so failed worker runs currently return a terse terminal error without the event trail needed to diagnose what the worker actually did.
- `internal/runtime/run_acceptance.go` now detects a pending delegate invocation only when no terminal delegate result/error exists. That preserves the first pending proof, but a later failed result can obscure a subsequent retry that is still pending.
- Vsuper and super prompts already encourage delegation boundaries, but the failed run suggests the worker-side prompt/runtime contract does not yet force a reviewable terminal outcome before the tool-loop limit.
- The current 25-iteration cap is too low for a realistic vsuper/co-super delegation chain. Raising it to 200 is part of the next mission, but it must be paired with better failed-run evidence and terminal behavior so the larger limit does not just hide loops for longer.
- Other runtime parameters also look shaped for smaller loops and should be audited in this mission:
  - `defaultDelegateWorkerVMTimeout = 8m`, `maxDelegateWorkerVMTimeout = 15m`, `maxDelegateWorkerRunAttempts = 2`
  - worker status polling every 500ms
  - worker event fetch limit of 500 events
  - capped tool result output of 100KB
  - run-memory compaction threshold of roughly 160k estimated tokens, retaining roughly 20k recent raw tokens
  - context-overflow recovery retries once after forced compaction, then blocks the run
- Run memory v0 is durable but low-resolution. It persists provider-facing messages, writes deterministic compaction summaries, rebuilds context from the latest checkpoint plus kept raw messages, and emits compaction/retry Trace events. The summary is not yet a model-authored operational sufficient statistic.
- Current tool outputs are capped before they become both provider-visible `tool_result` messages and durable `tool.result` events. That keeps context bounded, but it means long raw outputs are not preserved unless the tool separately writes an artifact. The mission should split "model context excerpt" from "durable full artifact reference."
- Pi-style compaction highlights several Choir gaps:
  - built-in tool output defaults are tighter than Choir's current 100KB cap: roughly 50KB or 2000 lines for read/bash-style outputs, 100 grep matches by default, 500 characters per grep match line, and 2000 characters for tool-result text serialized into compaction summarizer prompts
  - bash-style outputs keep the tail, because failures and final summaries are usually at the end; read/search-style outputs keep the head and provide offset/limit or refinement instructions
  - truncated bash output carries a full-output file path; Choir should use durable evidence/artifact refs rather than relying on temp paths
  - truncation is not the right path for user-supplied source artifacts. Researcher needs a full-source ingestion path for uploaded or linked papers, books, transcripts, logs, and datasets, with chunked/sectioned retrieval and source anchors.
  - trigger should be based on model context window minus reserve tokens, not only a fixed 160k-token estimate
  - repeated compaction should summarize from the previous `firstKeptEntryId`, not merely from the entry after the previous compaction checkpoint
  - summarization should serialize old messages as inert text and truncate tool results for the summarizer, while keeping the append-only session log intact
  - summaries should be structured operational state: goal, constraints, progress, blockers, decisions, next steps, critical context, read files, and modified files
  - cut points should respect turns and split oversized turns explicitly, not only avoid starting at a `tool_result`
- Mobile Trace proof remains a real UX/readability risk: the Trace window was clipped offscreen on a 390x844 viewport. This mission may fix it after substrate proof, but it must not displace the worker evidence/termination objective.

## Real Artifact

The artifact is not a one-off UX patch. It is a deployed, product-path delegation substrate in which:

```text
visible staging prompt bar
-> VText task document
-> foreground super
-> request_worker_vm
-> delegate_worker_vm
-> worker VM runtime vsuper
-> worker/verifier co-super coordination over channels
-> export, reviewable candidate, promotion route, or precise blocker
-> parent Trace, VText, and RunAcceptanceRecord evidence
```

The mission should improve the substrate while using a sweep-shaped Choir self-development workload as the test load. The workload can include Trace readability and logged-out onboarding/explore UX, but the substrate proof comes first.

## Value Criterion

Maximize delegated self-development realism while minimizing hidden worker-run failure.

Better states have these properties:

- every completed, failed, blocked, or still-pending `delegate_worker_vm` attempt leaves structured parent-visible evidence
- failed worker runs include worker event summaries sufficient to classify the failure without SSH-only archaeology
- the tool-loop iteration budget is raised from 25 to 200 so realistic delegated work has room to coordinate, verify, and report
- other hard-coded execution budgets are either justified, raised, made configurable, or recorded as residual risks with a concrete next probe
- compaction and continuation are treated as part of long-run control, not as an afterthought once provider context explodes
- long tool outputs are preserved as retrievable artifacts or evidence refs while the provider sees only an intentional excerpt
- researcher can ingest and retrieve complete user-provided source artifacts through a bounded chunk/anchor API instead of relying on truncated `read_file` or `fetch_url` excerpts
- vsuper terminates with an export, reviewable candidate, promotion proposal, or precise blocker before exhausting the tool loop
- super does not mutate Choir app, harness, repo, candidate, promotion, or platform state locally; it leases/delegates to worker VM vsuper
- verifier/co-super evidence is real channel evidence, not a transcript-like placeholder
- UX fixes are accepted only when they are produced through the intended delegated path or explicitly blocked by substrate/runtime evidence

## Invariants

- Staging is the acceptance environment: `https://draft.choir-ip.com`.
- Product-path proof uses the visible staging prompt bar and public authenticated product APIs such as `/api/prompt-bar`, `/api/vtext/*`, `/api/trace/*`, `/api/promotions/*`, `/api/continuations/*`, and `/api/run-acceptances/*`.
- Browser-public proof must not bypass through `/api/agent/*`, `/api/prompts`, `/api/test/*`, `/internal/*`, or raw event mutation endpoints.
- Foreground/canonical state remains stable. Background/candidate computers mutate. Canonical state changes only by promotion.
- `super` may do stateless or minor operational work, but app/harness/Choir repo/candidate/promotion/platform mutations go to worker VM `vsuper`.
- `vsuper` owns the candidate/background computer. `cosuper` agents are subordinate to the super or vsuper that leased them.
- Verification is evidence contract work, not privileged magic. Verifier evidence must be independently inspectable.
- Fake-island placeholders are forbidden. Do not fabricate transclusion panels, candidate exports, co-super messages, or promotion records.
- Logged-out desktop read/explore usability stays usable until mutation, LLM/search, worker, candidate, or auth-required actions genuinely require sign-in.
- Behavior-changing platform fixes must complete the landing loop: commit -> push origin main -> monitor CI -> monitor staging deploy -> verify staging commit identity -> deployed acceptance proof.

## Starting Belief State

Worker VM lease and delegate submission are no longer the first blocker. The next blocker is likely one of:

1. Vsuper lacks a strong enough terminal contract and keeps using tools until `maxToolLoopIterations` fails the run.
2. Vsuper is attempting unavailable or mismatched tools for co-super/channel/export work and cannot convert that into a blocker.
3. `delegate_worker_vm` hides the decisive failed-run event trail, so the parent super and run acceptance cannot tell whether the failure is prompt, tool authority, runtime, or workload complexity.
4. Long manual retries can stay pending while earlier failures dominate acceptance synthesis, losing retry context.

The mission should lower uncertainty in that order before widening the workload.

## Homotopy Axes

Move along these axes as evidence allows:

- evidence visibility: terse delegate failure -> worker event summary -> full linked failed-run evidence -> durable run acceptance checkpoint
- iteration budget: 25-iteration synthetic ceiling -> 200-iteration unstable-system ceiling -> 1000 or no fixed cap once per-run cost, lease, compaction, cancellation, and evidence backpressure are proven
- parameter realism: hard-coded short-loop defaults -> named budget model -> product-visible budget/lease status and termination evidence
- run memory: deterministic v0 compaction -> Pi-style append-only checkpointed summarization -> operational sufficient-statistic compaction -> continuation-grade memory with objective, constraints, failed approaches, verification state, rollback points, and user taste updates
- tool output handling: 100KB inline output cap -> smaller provider/summary excerpt with full artifact reference -> domain-specific evidence retention policy
- source artifact handling: truncated fetch/read excerpts -> full content substrate ingestion -> researcher chunk map, section anchors, citations, and retrieval by span
- termination behavior: max-loop crash -> explicit blocker and `end_turn` -> export/reviewable candidate -> promotion proposal
- agent topology: super-only observation -> super delegates vsuper -> vsuper coordinates worker co-super -> vsuper coordinates worker and verifier co-supers over channels
- workload realism: small synthetic substrate probe -> Trace readability candidate -> logged-out UX/onboarding candidate -> broader sweep
- acceptance level: `staging-smoke-level` blocked -> `export-level` -> `promotion-level` only with owner review plus promotion/rollback evidence
- Trace UX: desktop readable -> mobile blocker characterized -> mobile readable without hiding run evidence

Do not skip to a more realistic workload when the lower-realism state is still opaque.

## Receding-Horizon Control

First make the next failure legible. Add or adjust tests and runtime behavior so failed and pending worker delegates are represented in parent-visible Trace/run-acceptance evidence.

Then classify the actual worker-loop failure from worker events. Prefer a minimal prompt/runtime contract fix if the worker is simply not ending; prefer tool authority or channel/export fixes if the events show missing capabilities.

Only after worker delegate termination is either repaired or precisely isolated should the mission spend time on Trace/readability or onboarding UX. Those UX changes are the testing workload, not the substrate substitute.

## Dense Feedback Channels

- local focused tests for `delegate_worker_vm`, tool-loop termination, and run-acceptance synthesis
- staging prompt-bar submission and Trace snapshots
- VText document revisions for task, blocker, and final report
- `vmctl worker` health, worker VM id, sandbox URL, and worker run status
- delegate submit/status behavior, including failed and pending attempts
- worker event summaries for failed runs
- GitHub Actions run for pushed SHA
- staging deploy health/build identity
- desktop and mobile screenshots for Trace readability

## Control Priorities

1. Audit execution budgets that can prematurely collapse multi-agent work: tool-loop iteration cap, delegate worker timeout/max timeout, worker run attempts, polling cadence, event fetch limits, tool output caps, compaction threshold, kept-context budget, and overflow retry policy.
2. Raise the runtime tool-loop cap from 25 to 200, with focused tests adjusted or added so the new budget is intentional and visible. Record the conditions required before moving to 1000 or no fixed cap.
3. Audit and begin repairing compaction semantics against the Pi-style invariant: append-only complete session log, model context as latest summary plus exact recent tail, repeated compaction from prior `firstKeptEntryId`, turn-safe cut points, and bounded tool-result serialization for summaries.
4. Preserve failed and pending delegate evidence. A worker run that fails before `end_turn` must still leave parent-visible run id, state, error, worker event summary, and retry/pending context where present.
5. Diagnose why the worker vsuper exceeded the old tool-loop limit. Use worker events, not speculation.
6. Repair the smallest real boundary violation: prompt termination contract, worker tool authority, channel/export path, delegate polling, runtime event reporting, tool-output retention, or compaction/control budget.
7. Rerun the same sweep-shaped workload from the visible staging prompt bar. Confirm super delegates Choir/app/harness/repo/candidate/promotion work to worker VM vsuper.
8. Require vsuper to coordinate worker and verifier co-super agents over real channels before accepting export/promotion substrate proof.
9. Fix Trace mobile readability if the substrate is no longer the blocker; otherwise record it as a precise UX blocker with screenshot and next probe.
10. Synthesize run acceptance from existing evidence and write a VText final report with rollback refs, residual risks, and next objective.

## Forbidden Shortcuts

- Do not claim success from the 25 -> 200 `maxToolLoopIterations` increase, larger timeouts, or sleep/poll padding alone.
- Do not manually seed success records, fake co-super messages, fake exports, or fake promotion evidence.
- Do not treat local browser/unit proof as acceptance for worker VM, live candidate computers, gateway credentials, model/search calls, auth/session renewal, platform promotion, rollback, or Choir-in-Choir behavior.
- Do not use internal/test-only browser-public routes for acceptance.
- Do not let super make Choir app/harness/repo/candidate/promotion mutations locally just because it can.
- Do not replace candidate/transclusion/product behavior with fake-island UI placeholders.
- Do not broaden into general UX polish while worker-delegation evidence remains opaque.

## Expected Platform Work

Likely code changes to consider, subject to investigation:

- raise `maxToolLoopIterations` from 25 to 200 and ensure any tests or evidence expectations reflect the new budget
- make the eventual 1000/no-fixed-cap target explicit in docs or config comments, with safeguards: per-run budget, lease deadline, cancellation, compaction, event visibility, and parent-visible blocker reporting
- audit delegate timeout, status polling, event fetch, output cap, compaction threshold, kept recent context, and overflow retry defaults for mismatch with long-running Choir-in-Choir work
- determine whether `maxToolLoopIterations` should remain a constant for now or become runtime config once telemetry exists
- decide the tool-output policy: lower the inline provider/event cap, store long raw outputs as durable artifacts with digest/ref metadata, and teach Trace/run acceptance to surface the artifact ref rather than losing the tail
- replicate Pi's tool-specific truncation stance where it fits Choir: head truncation plus continuation hints for file reads, tail truncation plus full-output artifact refs for shell/log outputs, match/line caps for search outputs, and a much smaller compaction-serializer cap around 2000 characters
- add or precisely scope a researcher full-source path: import uploaded/linked long documents into the content substrate without lossy excerpt-only storage, record hash/provenance/media type, expose chunked retrieval by content id and span/section, and require researcher findings to cite content id plus offsets or anchors
- audit current source limits that would break papers/books/transcripts: `read_file` 100KB hard cap, `fetch_url` 256KB body cap plus 12k excerpt, URL import 2MiB response cap and 300KiB extracted text cap
- move compaction triggering toward `context_window - reserve_tokens` when model context-window metadata is available, retaining a conservative fixed fallback
- repair repeated compaction so the next summarized span starts at the previous `firstKeptEntryId` when available
- implement or precisely scope Pi-style structured/model-authored summaries, turn-safe cut points, split-turn handling, inert serialization, and cumulative artifact/file tracking
- make `delegate_worker_vm` fetch and summarize worker run events for failed and blocked terminal states, not only successful completion
- include structured failed-run evidence in the parent tool error/result path without losing the clear terminal error
- make run-acceptance synthesis preserve both earlier delegate failures and later still-pending retry invocations
- add tests for failed delegated worker runs, worker event summary propagation, and mixed failed-plus-pending delegate attempts
- add or update tests proving compaction behavior still works with longer tool loops and that context-overflow recovery remains bounded and visible
- tune super/vsuper/co-super prompt contracts so vsuper must produce an export, reviewable candidate, or precise blocker and then end the turn before tool-loop exhaustion
- if events show missing authority, repair the worker-side tool/channel/export surface instead of papering over it in prompt text

## Acceptance Targets

Success target:

- staging health reports the pushed SHA
- visible staging prompt-bar run creates a VText-backed Choir self-development task
- Trace shows foreground super requesting and delegating to worker VM vsuper
- Trace or linked worker evidence shows vsuper coordinating worker and verifier co-super agents over channels
- worker produces export, reviewable candidate evidence, or promotion proposal
- run acceptance reaches the appropriate evidence level, at least `export-level` for export evidence and `promotion-level` only with verifier contract plus owner review and promotion/rollback evidence
- VText final report names trajectory/run/acceptance ids, verifier contracts, rollback refs, residual risks, and next objective

Clean blocker target:

- run acceptance remains `staging-smoke-level` or another explicit blocked level
- blocker includes worker id, worker VM id, worker run id, run state, terminal error, worker event summary, delegate invocation/result sequence ids, retry/pending context, rollback refs, and the next safe probe
- blocker identifies whether the remaining limit is iteration budget, wall-clock lease, event visibility, compaction/context pressure, worker authority, or tool availability
- Trace stays live while pending work exists and does not falsely present a completed trajectory
- no fake UX/product artifact stands in for the missing substrate

## Rollback

For platform changes, rollback is the pushed commit SHA plus a normal revert on `origin/main`, followed by the same CI/deploy/health loop.

For product-path work, discard failed candidate/background computers and avoid promotion. Run acceptance records and VText blocker reports are evidence, not canonical user state mutation.

For prompt-only changes, keep the old prompt commit SHA and the new prompt commit SHA in the final report so the previous behavior can be restored with a targeted revert.

## Stopping Condition

Stop only after one of these is durable:

1. Deployed staging proof shows delegated vsuper coordination with worker/verifier co-super evidence and export/promotion candidate evidence, with run acceptance and VText final report.
2. Deployed staging proof records a lower-level blocker that is more precise than the current `tool loop: exceeded 25 iterations without end_turn` error because it includes failed/pending worker run events and a concrete next safe probe.

Either ending must include commit SHA, CI run, deploy status, staging build identity, product-path acceptance command/result, accepted trajectory/run/acceptance ids, rollback refs, residual risks, and the next realism axis.
