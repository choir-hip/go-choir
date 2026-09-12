# Completed-phase audit on current HEAD

Purpose: the goal-audit obligation for
`docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md` — verify that the
phases this mission records as complete still hold **on the current tree**,
from code and test output rather than from earlier-session memory.

Tree audited: `main@5f34baf1`, worktree clean. Commands run locally on darwin
(the repo's Nix dev shell supplies the ICU/Dolt cgo environment).

## Deterministic checks

| Deliverable | Check | Result |
|---|---|---|
| P2-simplify: `capsule_go_eval` is one required `source` string, no `code` alias | read the tool schema | **holds** — `internal/agentcore/tools_capsule.go:625-630`: one property `source` (string, "Raw Go source for one REPL cell."), one-sentence description, `required=["source"]`, no `code` |
| P2-repair-deletion: `CleanGoSource` deleted with its call sites | repo-wide reference sweep | **holds** — zero references to `CleanGoSource` in any `.go` or `.yaml` |
| P2-invariant: one prompt, no per-model fork | `go test ./internal/agentcore -run 'TestCoSuperPromptSwitchesToSealedGoUnderRLM\|TestRLMPromptOmitsRetiredToolNames\|TestCoSuperPromptIsModelIndependent'` | **holds** — 3/3 PASS; the model-independence test compares two runs differing only in `model` and `llm_policy_overlay_id` and requires byte-identical prompts |
| P4-registry: RLM assigned registry is the eval envelope alone; tools branch is the exact closed set | `go test ./internal/agentcore -run 'TestRLMAssignedCoSuperOverlayIsSealedGo\|TestAssignedCoSuperBuilderIsExactClosedSet\|TestCapsuleLocalInstallerIsExact\|TestAssignCoSuperSchemaCarriesModelPolicyOverlay\|TestDelegatedCoSuperCannotReachHostEffectToolsOrCallbacks\|TestCapsuleGoEvalToolDispatchesToExecutorGoEval\|TestAssignedCoSuperPromptNamesExactKindWithoutFutureToolLie'` | **holds** — 7/7 PASS in one run |
| P4-delete: the five overlay JSON names are deleted from the carrier | reference sweep for each name as a quoted tool name in non-test Go | **holds, with one desk-scoped exception explained below** — `commit_transaction`, `inspect_self_development_bundle`, `record_self_development_verification`, `record_assignment_result`: 0 non-test references each. `update_coagent`: 18, all of them desk-scoped (below) |
| P4-delete: no served prompt instructs a name that no longer exists | sweep of `internal/runtimeprompts/` and `internal/promptstore/`, plus the assembly gating in code | **holds** — see below |

## The `update_coagent` exception, resolved

`update_coagent` is still registered (`internal/agentcore/tools_worker_update.go:176`)
and still named in prompts, which is intended rather than a regression:

- The retirement is **desk-scoped**. `TestRLMAssignedCoSuperOverlayIsSealedGo`
  asserts the assigned engineering registry is exactly `[capsule_go_eval]` and
  explicitly asserts `update_coagent` is absent from it, so the name is not
  reachable on the engineering carrier.
- The remaining references are the desks that still use it: Texture executes it
  (`internal/textureowner/texture.go:2303`), research prompts instruct it
  (`internal/runtimeprompts/overlays/research_runtime.yaml`), and the rest are
  `request_source` provenance strings and validator names. P3-parity requires
  exactly this: "Desk-scope the retirement and prove Texture and research
  citers keep working."
- The one instruction that reads unconditionally,
  `internal/runtimeprompts/overlays/run_context.yaml:13` ("Address every
  update_coagent to Texture coagent …"), sits inside `{{if
  .TextureDeliveryAgentID}}`, and that field is set in exactly one place:
  `internal/agentcore/tool_profiles.go:290-292`, gated on
  `profile == agentprofile.Researcher && isTextureAgentID(requesterAgentID)`.
  An assigned engineering CoSuper therefore never receives it, and its run
  context takes the `InCellCarrier` branch, which names `choir.Message`.

## Verified by artifact, not re-run here

Stated explicitly so this audit is not read as broader than it is:

- **P3/P4 deployed proofs** (settlement, acceptance-rebuild negative, per-operation
  replay): the replay harness is a Linux-only test
  (`internal/agentcore/rlm_replay_linux_test.go`) and cannot run on this darwin
  host. Their evidence is the published artifacts
  (`docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md`,
  the P4 replay deployed proof, and the goldens under
  `internal/agentcore/testdata/rlm_replay/`), not a local re-run.
- **P1 provider live calls**: not re-probed in this audit. What was observed this
  session is that the arm overlay resolves to `opencode-go/deepseek-v4.1-flash`
  through `roster preflight`, which exercises the resolver, not a provider call.
- **The roster itself**: unmeasured. No arm has been served by its overlay, and
  the activation stall recorded in the Definition's `problems_discovered`
  prevents another attempt.

## Consequence for the landing

The carrier deliverables that the landing would certify are consistent with the
current tree. What remains unproven is the roster measurement and the deployment
receipts that depend on it, so this audit does not move the mission toward
completion; it removes the risk that a "completed" phase silently regressed
while the roster was blocked.
