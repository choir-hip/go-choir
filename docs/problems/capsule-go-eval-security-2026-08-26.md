# Effects-Red: Capsule Go-Eval Wiring Has Critical Authority and Evidence Gaps

<problem_id: capsule-go-eval-security-2026-08-26>
<first_observed: 2026-08-26>
<mutation_class: red>
<frozen_candidate_digest: d65a562a4900667528e5c19a02aa28508b7269753e9f625e2f660ffdceed49eb>
<deployed_commit: de0597c2a9bc9ae433e8c7b46c4ac5241ce613fc>

## Problem

The go_eval wiring slice (commit de0597c2, candidate digest
d65a562a4900667528e5c19a02aa28508b7269753e9f625e2f660ffdceed49eb) routes
model-authored Go through the existing capsule broker, but an independent
adversarial security review (Sol = REJECT, Codex = NEEDS-REPAIR,
Gemini = NEEDS-REPAIR; panel at
.agentic-consensus/agentic-consensus-20260826-154655/) found direct authority
bypasses and evidence gaps. This is a security-defect receipt, not a completion
claim: the slice is deployed to staging but must not be treated as effect-capable
or accepted.

## Findings (converged by 3 independent reviewers)

### Critical

1. **Model-controlled package allowlist (authority bypass).**
   `capsule_go_eval` exposes `allowed_packages` as a model-callable argument; it
   becomes the worker's effective allowlist. The denylist checks only exact
   package names, so authority-bearing deputies (go/parser + go/token to read
   capsule files; net/http/cgi + net/http/httptest to reach native os/exec
   indirectly) can bypass broker verbs, Researcher's Go-only restriction, and
   execution receipts. Files: `internal/agentcore/tools_capsule.go:739-770`,
   `cmd/capsule-broker/main.go:464-467`, `internal/yaegikernel/sidecar.go:146-148`,
   `internal/yaegikernel/allowlist.go:49-79`.

2. **No activation-cancellation binding.** The worker runs in
   `context.Background()`; caller cancellation only expires a socket read. The
   worker can continue up to ~60s after the activation ends; `releaseOp` may then
   let the capsule freeze while the unauthorized worker survives until later
   thaw. The worker lacks `Pdeathsig`, so broker death does not guarantee worker
   death. Files: `internal/agentcore/tools_capsule.go:752-770`,
   `internal/capsule/broker_client.go:86-104`,
   `cmd/capsule-broker/main.go:322-323,477-503`.

3. **Skips obligation revalidation.** CoSuper `capsule_go_eval` uses
   `requireCapsuleRole(RoleCoSuper)` without `requireCurrentAssignedCapsule` /
   `requireCapsuleMutationRole`, so it skips durable assignment, cancellation
   intent, work-item, run-state, and capsule-state revalidation. A stale or
   revoked CoSuper can evaluate Go after assignment authority ends. Files:
   `internal/agentcore/tools_capsule.go:753-760`, `:114-142`.

4. **Receipt-after-execution and incomplete evidence.** Secret-bearing source
   executes before `DetectPrivateSecrets` refuses it (and no receipt is then
   persisted). Transport/start/digest failures leave executed attempts
   unrecorded. Successful receipts record only `Command: "go_eval"` — not source
   digest, package policy, timeout, or worker identity — so distinct programs are
   indistinguishable; the model chooses which receipt refs enter
   `record_assignment_result`, enabling omission; timeout retains `ExitCode: 0`,
   making failed execution appear successful. Files:
   `internal/capsule/executor.go:606-638`, `internal/capsule/types.go:115-129`,
   `cmd/capsule-broker/main.go:497-520`.

### High

5. **Researcher Go path is not actually wired.** The Researcher registry never
   installs `capsule_go_eval`; runtime injects capsule context only for Super and
   CoSuper; Researcher capabilities must be wildcard and `resolveOne` rejects
   wildcard targets. Current tests assert verb membership and generic registry
   contents without exercising a real Researcher activation, so they pass while
   the promised restricted profile is unusable. Files:
   `internal/agentcore/tool_profiles.go:393-400`,
   `internal/agentcore/runtime.go:3044-3066`, `internal/capsule/executor.go:505-506,907-918`.

6. **Unbounded output buffers.** `MaxOutputBytes` is configured but unused on the
   go_eval path; output accumulates in unbounded bytes.Buffer. A program can
   print until the cgroup OOM-kills the worker or the trusted broker, denying
   every operation in the capsule. Files: `internal/yaegikernel/eval.go:92-128`,
   `internal/yaegikernel/sidecar.go:47-49,146-157`,
   `cmd/capsule-broker/main.go:483-520`.

## Admissible evidence class

This is a security-defect receipt at a Define boundary. The independent security
review is bound to the frozen candidate digest d65a562a..., which was deployed
to staging as commit de0597c2. The review outputs are at
`.agentic-consensus/agentic-consensus-20260826-154655/` (sol, codex, gemini).

## Consequence

- The go_eval wiring is NOT effect-capable and must not be accepted. External
  effect-bearing acceptance remains gated (not_done_when) on the predecessor
  effects Definition goal.complete, which is itself unmet.
- Before any deployed CoSuper/Researcher Go activation is authorized, every
  Critical finding must be repaired: server-side profile-owned allowlist (never
  model-controlled), explicit cancellation-broker binding, obligation
  revalidation, host-owned attempt record before dispatch with complete evidence,
  and bounded output.
- Researcher (Go-only) must be genuinely wired and proven with an end-to-end
  test, not just verb membership.

## Next action

Repair the four Critical findings and the two High findings in a follow-up
slice, then re-run the security review bound to the new frozen candidate digest
before any further deploy or activation.

## Repair progress (2026-08-26, second slice)

Repaired in the follow-up slice (not yet re-reviewed):

- Critical #1 (model-controlled allowlist): broker now resolves the package
  allowlist server-side from the verified capability AgentRole and ignores the
  model's allowed_packages entirely (cmd/capsule-broker/main.go handleGoEval).
- Critical #3 (obligation revalidation): capsule_go_eval now uses
  requireCurrentAssignedCapsule (ValidateCurrentObligation) and is CoSuper-only;
  Researcher wiring is a separate slice and is no longer falsely exposed here.
- Critical #4 (secret-before-execution + evidence): Executor.GoEval detects
  secret-bearing source BEFORE dispatch; receipt command now binds the source
  digest (go_eval:<digest>) so distinct programs are distinguishable.
- High #6 (unbounded output): broker enforces a 2 MiB cappedBuffer and marks
  overflow; worker group is killed on timeout.
- High #5 (Researcher not wired): acknowledged; the Researcher Go-only profile
  requires a capsule-context injection path in runtime.go (separate slice).
- Process lifecycle: worker sets Pdeathsig=SIGKILL so broker death reaps the
  worker; NewAllowlist defaults to the safe set when empty (also fixes the
  default-invocation regression that rejected base packages).

Progress on critical #4 (host-owned attempt record on every outcome) was added:
Executor.GoEval now persists an auditable receipt for every ADMITTED evaluation
(success, interpreter error, timeout, cancellation) and binds the source digest
before dispatch; a failed/timed-out attempt no longer reports ExitCode 0. This
closes the "executed attempts unrecorded" and "timeout looks successful" parts of
critical #4.

Containment hardening in the same window: the go_eval timeout path now SIGKILLs
the worker process group AND reaps it (bounded 5s grace), so no zombie process or
lingering cmd.Wait goroutine survives; a timed-out attempt reports ExitCode 1
rather than a successful-looking ExitCode 0. Note the classic exec verb has the
same single-broker RPC cancellation semantics; true client-cancellation ->
broker-cancel with request IDs would be a broker-protocol change shared by all
verbs and is deferred as a single-path design, not a go_eval-only patch.

Researcher design boundary (verified, requires a Define boundary not a patch):
the Researcher role has NO capsule-context path today. RegisterCapsuleLocalTools
is called only from the CoSuper builder; the Researcher registry (built via
buildRegistryForRole) installs no capsule tool. `MintCapabilityHandle` forces
RoleResearcher to TargetCapsule="*" (read-only inspection across capsules), and
resolveOne rejects any wildcard target, so a Researcher cannot obtain a specific
capsule for Go evaluation. wiring the Researcher Go-only profile therefore
requires a NEW dedicated Researcher capsule lifecycle (a read-only capsule bound
non-wildcard to the Researcher run) plus context injection in executeWithToolLoop
and Researcher registry registration. That is an architectural design change, not
a safe patch; do not fabricate a Researcher binding that introduces a new
authority path without a Define boundary and re-review.

Additional repair (from repaired-candidate re-review c4e03779, panel diverge:
Gemini=SAFE-TO-LAND, Sol=NEEDS-REPAIR):
- RESTART-DURABILITY FIX (real bug): OpenExecutionReceipt hard-coded the
  capsule-exec:sha256: prefix, so Go-eval receipts stored as
  capsule-go-eval:sha256: failed to re-verify after executor restart. Now
  validates using the receipt's OWN prefix (exec/go-eval/fate).
- WORKER OUTPUT BOUND FIX (real bug): the worker's interpreter wrote to
  unbounded bytes.Buffer; the broker's 2 MiB cap saw only the post-eval JSON, so
  a runaway print loop could OOM the capsule before the cap. Eval now bounds the
  interpreter's Stdout/Stderr at maxEvalOutputBytes with a concurrency-safe
  cappedBuffer + overflowWriter, and cancels the interpreter context on
  overflow. Test TestEvalOutputOverflowFailsClosed pins it.
- CONCURRENT-RPC SERIALIZATION FIX (real bug): BrokerClient.call shared one
  net.Conn with no mutex or request IDs, so concurrent RPCs could interleave
  frames and one caller reads another's response. Added connMu serializing the
  full encode/decode round trip. (acquireOp counts inflight for quiesce but does
  not serialize the stream.)
- Still open (acknowledged, single-path deferral): client/activation cancellation
  -> broker-cancel verb (shared by all broker verbs including exec); the worker
  attempt record is post-dispatch (write-ahead admission record would make it
  crash-durable); a genuinely wired Researcher profile (design boundary).

## Additional critical functional find (still after the first repair)

The security review's "worker mode cannot start" finding was CONFIRMED as a real
deployed bug and fixed in the same follow-up: handleGoEval launched the worker
with the bare flag `--exec-go-stdin`, but main() dispatches on --isolation-stage,
so flag.Parse rejected the unknown flag and the worker exited before evaluating.
Fixed to `--isolation-stage=exec-go-stdin`. The worker environment was also
sanitized (PATH/TMPDIR only) so it does not inherit broker credentials or
control-socket variables. Test TestExecuteWorkerStdinRoundTrip pins the worker
contract end-to-end.
