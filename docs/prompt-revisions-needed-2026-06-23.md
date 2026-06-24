# Prompt Revisions Needed

## Status

Draft. Priority order inside.

## Priority 1: `run_system.yaml`

`internal/runtime/textureprompts/overlays/run_system.yaml` is the active
Texture system prompt. It mixes source-citation instructions, article-format
prohibitions, and a `WireTexture` control-flow branch. This is the source of the
regressions the user observed in the QA run.

### Required changes

1. **Remove `source_embed` / `insert_source_embed` entirely.**
   - Delete the `insert_source_embed` mention from the `patch_texture` operation
     list.
   - Delete the instruction to use `insert_source_embed` for local excerpt cards.

2. **Remove the `WireTexture` branch.**
   - Delete `{{if .WireTexture}}`, `{{else}}`, and `{{end}}`.
   - The article-format and citation rules that live in the Wire branch must
     become the default behavior for all Texture runs, driven by the default
     style texture.

3. **Remove the negative article-format list.**
   - Delete `"not a Source Brief, Working Revision, Evidence Gathering note, outline, or placeholder"`.
   - Delete the permission to write an interim scaffold that structures
     "question, uncertainty, evidence plan, and unresolved branches".
   - Replace with positive style guidance from the default style texture.

4. **Make inline citations the default.**
   - State that `insert_source_ref` with `display_mode: numbered_ref` is the
     default citation shape.
   - State that `display_mode: expanded_ref` is used only when a block excerpt
     is editorially required.

5. **Add the `mark_source_unused` operation.**
   - Document that sources can be marked immaterial with a rationale.
   - Marked sources stay in the toolbar and do not require a body citation.

6. **Remove source-inventory warnings that name forbidden formats.**
   - The phrase "Source ids only in source inventories, Source: lines, markdown
     web links, or metadata sections do not count" should be replaced by a
     positive rule: every material source must be cited as a `source_ref` in the
     body.

## Priority 2: `revision_source_entities_intro.yaml`

`internal/runtime/textureprompts/overlays/revision_source_entities_intro.yaml`
introduces the available source entities to the Texture agent.

### Required changes

1. Remove `insert_source_embed` references.
2. Update instructions to use `insert_source_ref` with `display_mode`.
3. Explain `mark_source_unused` for immaterial sources.

## Priority 3: `revision_policy.yaml`

`internal/runtime/textureprompts/overlays/revision_policy.yaml` is the policy
overlay for user-authored revision turns. It contains multiple control-flow
branches.

### Required changes

1. **Flatten the `UserAuthoredRevision` / `OwnerPromptRequestRevision` branch.**
   - The model should treat the current revision as the canonical input for the
     next version, whether it is the owner prompt or a later edit.
   - Remove the special-case handling for the first owner prompt.

2. **Remove the `ExplicitResearcherRequest` branch.**
   - The model can detect an explicit researcher request from the user text.
   - The prompt should state the general rule: substantive factual/current claims
     require researcher evidence.

3. **Flatten the `HasGroundedHistory` branch.**
   - The run context already includes worker messages and the document head.
   - The policy should be the same with or without grounded history.

4. **Remove `insert_source_embed` from the operation list.**
5. **Update `insert_source_ref` guidance to include `display_mode`.**

## Priority 4: `revision_worker_findings.yaml`

`internal/runtime/textureprompts/overlays/revision_worker_findings.yaml` adds
worker-finding policy to revision prompts.

### Required changes

1. **Flatten the `IntegrateWorkerFindings` branch.**
   - The policy should always be: when worker packets arrive, incorporate them
     into the next revision before spawning more workers.
2. **Remove the `NeedsSuperExecution` branch.**
   - `request_super_execution` availability is enough signal.
3. **Remove the `ActiveWorkerDelegation` branch.**
   - Active worker status is part of the run context; the model can read it.
4. **Remove any `insert_source_embed` references.**
5. **Update source incorporation guidance to use `insert_source_ref` only.**

## Priority 5: runtime prompt overlays (data substitution review)

The following overlays use templating for data substitution. They should be
reviewed but are lower priority because they insert values rather than switch
behavior:

- `internal/runtime/runtimeprompts/overlays/run_context.yaml`
  - `{{if .AgentID}}`, `{{if .RequesterAgentID}}`,
    `{{if .TextureDeliveryAgentID}}`, `{{if .ChannelID}}`
  - Keep as value substitutions; ensure they do not leak into conditional
    instructions.

- `internal/runtime/runtimeprompts/overlays/conductor_run.yaml`
  - `{{if .RequestedApp}}`, `{{if .SeedPrompt}}`
  - Keep as value substitutions.

- `internal/runtime/runtimeprompts/overlays/co_super_runtime.yaml`
  - `{{if .RepoBootstrap}}{{.RepoBootstrap}}{{end}}`
  - Keep as value substitution; the `{{if}}` is only guarding an empty block.

- `internal/runtime/runtimeprompts/overlays/vsuper_runtime.yaml`
  - `{{if .RepoBootstrap}}{{.RepoBootstrap}}{{end}}`
  - Same as co_super_runtime.

## Design principle

Prompts should contain:

- Data: style texture, sources, run context, tool descriptions.
- Invariants: cite sources, no model priors as grounded, canonical revisions
  via tools.

Prompts should not contain:

- Boolean branches that change behavior based on run metadata.
- Negative format prohibitions.
- Special-case overlays for product pipelines.

## Sequencing

1. Edit `run_system.yaml` first. This is the highest-impact prompt and the source
   of the observed regressions.
2. Edit `revision_source_entities_intro.yaml` and `revision_policy.yaml` next.
3. Edit `revision_worker_findings.yaml` after the main Texture path is stable.
4. Review the runtime overlays for control-flow creep when the higher-priority
   items are done.

## Dependencies

The prompt edits depend on the `source_embed` removal and the default style
texture work documented in `design-source-ref-expansion-2026-06-23.md`. The
prompts should be edited after the schema and tool changes are in place, or in
parallel with tests that verify the new prompt strings.
