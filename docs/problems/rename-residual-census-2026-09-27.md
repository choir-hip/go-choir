# Problem Receipt — Post-Rename Vocabulary Residuals on the Live Surface

Date: 2026-09-27
Status: open — census complete, remediation pending owner/panel adjudication
Mutation class: green (documentation only)

## What this is

The co-super→engineering / super→management rename shipped in an early
station and R5b (`durable vocabulary migration: rename-migrate`) was
**deferred indefinitely** by the 2026-09-25 restructure (mission-graph,
`choir-durable-vocabulary-migration-draft-2026-09-23`). R5a froze the
decoders over the legacy vocabulary — so the tape vocab is pinned — but a
census shows the legacy vocabulary is still load-bearing on the **live
surface**: tool names, run-metadata keys, a SQL table, schema IDs,
prompt text, error strings, and one user-visible app/route name.

Owner direction 2026-09-27: "many 'co-super' matches in the code … the
renaming from cosuper to engineering (and super to management) was an early
mission but we should review it and possibly other missions for additional
passes … once the reconciliation plan is done it should be critically
reviewed and redone until excellent."

## Census (non-test source, excluding vocabmigrate/vocab_migrate, which are
## the decoder/migration layer that must carry legacy tokens by design)

### Class A — live surface, no tape/durable constraint (safe to rename)
| Surface | Token | Where |
|---|---|---|
| Error prefix | `co-super assignment:` / `co-super assignment report:` | ~56 sites in `internal/types/engineering_assignment.go`, ~66 in `internal/store/engineering_assignments.go` |
| Prompt *prose* mentions of the legacy desk name (not emitted values) | "co-super", "super delivery", "persistent super" wording inside prompt bodies | `internal/promptstore/defaults/management.yaml`, `internal/runtimeprompts/overlays/{management_runtime,rlm_management_runtime,rlm_engineering_runtime}.yaml`, `internal/textureowner/texture_agent_revision.go:534,684` |
| Error/log strings | "foreground super", "persistent super", "owning super run", "super co-super run requires co_super_slot" | `internal/apihandler/product_api_tool.go:50,63`, `internal/agentcore/{api_producer_reports,runtime}.go`, `internal/textureowner/texture_workflow_verifier.go`, `tools_worker_update.go` |
| Go-side file/identifier names | `coagent_update_packet.go`, `coagent_route.go`, `coagent_injection.go`, `internal/coagentowner/`, Go const/field names carrying `CoSuper`/`coagent` | identifiers only; wire values stay V1 |

### Class A′ — persisted/durable live surface (rename = normalization, not string swap)
| Surface | Token | Where |
|---|---|---|
| Run metadata key | `co_super_slot` | `internal/agentcore/tool_profiles.go:29`; written `engineering_assignment_runtime.go:665`, `runtime.go:1041`; read `tools_capsule.go:366`, `coagentowner/spawn_tool.go:144` (historic run records), `yaegikernel/choir.go:424` (in-cell ctx). Persisted in `RunRecord.Metadata` — needs one read-side normalization point (R5a's stratum-C pattern), then forward rename. Panel-verified. |
| Acceptance checkpoint kind | `super_direction_opened` + receipt-kind `"cosuper_assignment"` | `internal/agentcore/run_acceptance.go:171,181,800,854` — persisted in `run_acceptances` records and routed on by the acceptance state machine. Writer+gate must move in lockstep; old kind stays readable. |
| Texture control field | `open_persistent_super` JSON key | `internal/textureowner/tools_texture.go:49,87` — model-authored controls validate this exact key; prompts teach it (`revision_policy.yaml:20`, `run_system.yaml:11`). Dual-accept, don't rename in place. |
| User-visible app/route | `super-console` app id, `/api/super-console/ws` | `frontend/src/lib/apps/registry.ts:243-251` (saved in persisted desktop state — needs read-side mapping), `internal/proxy/handlers.go:774-1078`, `internal/autoputer/terminal.go:425`. Naming is already inconsistent: app title is "Engineering Console", proxy calls it "Management Console" — pick one. |

### Class B — frozen or durable vocabulary (renaming is a migration, not a fix)
| Surface | Token | Why frozen |
|---|---|---|
| Lifecycle command kinds | `cancel_co_super_assignment`, `open_co_super_assignment`, `bind_co_super_assignment`, `record_co_super_assignment`, `set_co_super_capsule_disposition` | `internal/types/lifecycle.go:25-35` — durable spellings inside `choir.lifecycle_command` objects, explicitly frozen in `internal/vocabmigrate/families.go:57-62`. NOTE: the *tool* name `cancel_co_super_assignment` is a separate surface from the emitted command kind — the tool name is Class A (rename it, keep the kind), the kind value is Class B. Panel-verified split. |
| Identity seeds | `choir:co-super-assignment:v3`, `:v2` request, `:v3` decision | `internal/vocabmigrate/families.go:256-260` — ID minting seeds; R5a pinned keep-v3. |
| Emitted schema IDs | `choir.co_super_assignment/v1`, `_grant_policy_attestation`, `_execution_attestation`, `_capsule_fate_step` | `internal/types/engineering_assignment.go:13-19` — on the canonical tape; frozen decoders route on them. |
| SQL table + manifest key | `co_super_slots`, `idx_co_super_slots_*`, `replay_eligibility.go:61` `"co_super_slots"` entry | `internal/store/store.go:442-946`, `internal/vocabmigrate/families.go:303-305` — persisted DDL in lockstep with the airworthiness manifest. (GLM/codex correction: the `replay_eligibility` entry was misfiled under Class A in v1 of this census.) |
| Receipt/packet kinds | `"coagent_update"`, `"coagent_result"`, `"update_coagent_signal"`, `coagent_mailboxes` | emitted on the canonical packet ledger; rename = new vocab version. (Tool name `update_coagent` itself is Class A; the wire kinds it emits are Class B.) |
| Decoder tables | `cosuper`/`co-super`/`co_super`/`coagent`/`co-agent` → `engineering` | `internal/vocabmigrate/migrate.go:37-42`, `internal/store/vocab_migrate.go`, `internal/computerevent/decode.go:42-65` — carrying legacy tokens is the decoder layer's job. |
| Policy seat id | `cosuper-author` in `reversible-selfdev-v1.json`, `internal/decisionpolicy/reduce.go:169` | digest-pinned policy; rename = new policy version (optional P5, per panel). |

### Class D — dead code (delete, don't rename)
| `cosuper_replacement_requested` | `internal/agentcore/management_controller.go:25` | zero non-test readers/writers. Its sibling `cosuper_replacement_omit_reports` (`:26`, read at `:1916`) is a legacy-run reader — keep with a comment. |

### Non-vocabulary completeness findings (panel consensus, verified)
| Finding | Evidence | Severity |
|---|---|---|
| **Demoted verbs still live + prompt-taught.** §11.5 assigned cutting `Message`/`Outcome`/`Spawn`/`Assign` to R3b/R3c; all three settled stations closed without cutting or recording a deferral. | `internal/yaegikernel/choir.go:154-169` exports all four on every desk set; `rlm_engineering_runtime.yaml:51-53` teaches them; `broker.go` carries a process-local mock for `Assign` | HIGH — panel consensus (sol/codex/claude/gemini/glm53) |
| **`report_to_texture` provenance drift.** R3c definition's falsifier says it must not remain a live management host tool; `tool_profiles.go:396-404` registers it anyway and mission-graph quietly re-rules the point without an owner-visible amendment. | `docs/definitions/choir-management-live-cast-2026-09-26.md:104-106` vs `tool_profiles.go:390-404` | MEDIUM-HIGH |
| **`Cast` ignores `to_desk` — every cast opens an Engineering assignment.** `IntentCast` passes `in.ToDesk` as `TargetDocID`; `openDelegatedCastAssignment` hardcodes `agentprofile.Engineering` | `internal/agentcore/rlm_reduce.go:776-784`, `engineering_assignment_runtime.go:435` | adjudicate: defect or engineering-only scope (plan §11 implies the latter) |
| **Plan §1 premise falsified.** "Live vocabulary: already cut over" contradicts this census. | `docs/desk-rlm-rectification-plan-2026-09-23.md:22` | stale-mark needed |

### Class C — deferred by owner ruling (R5b)

The whole `rename-migrate` half of R5 was deferred indefinitely; its
would-be outputs (tape rewriting, slot-table migration) are explicitly out
of scope. Anything in Class B that a future pass *does* want renamed is a
new schema/migration version, not a "rename fix".

## The review question

The early rename mission (`choir-rlm-versioned-rename-2026-09-09`) was
scoped to *canonical live-path desk vocabulary* and REOPENED once on
post-cutover review before settling. Most remaining matches are deliberate
Class-B frozen vocabulary — but the census cannot adjudicate three things:
(a) whether the durable-spelling freeze was correct or a convenience that
should be revisited (V2 command kinds coexisting with V1 via decoder);
(b) which Class-A items (prompt prose, error strings, file names, the
super-console surface) are worth a pass; (c) whether `update_coagent` —
the live wake tool name — is wire-bound to `coagent_update` packet kinds
or independently renameable.


## Method note

Residuals counted 2026-09-27 via `grep -rn 'co_super|cosuper|co-super|
CoSuper'` over `internal/ cmd/ scripts/ frontend/src`, non-test files,
excluding `vocabmigrate`/`vocab_migrate` decoder sources. ~324 matches in
live sources before classification.
