# Mission Ledger: Universal Wire Agent Pipeline v1.1

## Pass 1 — 2026-06-27 13:50 EDT

**Move:** construct — fix `buildCoagentTextureRevisionPrompt` to include source body text.

**Conjecture decided:**
- C3 (Texture agent produces article-grade English from source text): UNBLOCKED.
  The prompt now includes a "Source briefs (excerpt text for synthesis)" section
  with `excerpt_text` from each source entity's `ReaderSnapshot`, falling back to
  `text_content` (truncated 2K runes) and selector `TextQuote`. Local tests verify
  all three text sources appear in the prompt. The conjecture is not yet SUPPORTED
  (requires staging verification with real gpt-5.5 calls) but the blocker is removed.

**Conjecture verified structurally:**
- C2 (processor agent routes newsworthy items to Texture agent): SUPPORTED
  structurally. `roleSpec(AgentProfileProcessor)` has `AllowCoAgentTools=true`,
  `AllowedDelegateTargets=[texture]`. `spawn_agent` tool is registered with
  `texture` as allowed target. `universalWireProcessorHandoffPrompt` instructs:
  "spawn Texture agents when a story should be opened or revised." The
  `spawn_agent` handler for `callerProfile=processor, profile=texture` calls
  `ensureCoagentTextureRevisionRoute`. Runtime behavior (gpt-5.5 actually calling
  the tool) requires staging verification.

**Expected ΔV:** 4 → 2 (C3 unblocked, C2 structurally supported)
**Actual ΔV:** 4 → 2

**Receipt:**
- Commit `d38d3afd` on `main`
- `internal/runtime/tools_coagent.go`: added `sourceEntityExcerptText` helper,
  added "Source briefs" section to `buildCoagentTextureRevisionPrompt`
- `internal/runtime/wire_processor_decision_test.go`: added
  `TestBuildCoagentTextureRevisionPromptIncludesSourceBodyText`,
  `TestBuildCoagentTextureRevisionPromptNotesMissingSourceText`
- `go test ./internal/runtime -run 'TestBuildCoagentTextureRevisionPrompt'` — PASS
- `go test ./internal/runtime -run 'TestProcessorMixedPerItem|TestProcessorTextureRoute'` — PASS
- `go build ./internal/runtime/` — PASS

**Open edge:** Staging verification needed. gpt-5.5 must actually call
`spawn_agent` for newsworthy items and produce article-grade output. The prompt
fix removes the text gap but does not guarantee model behavior.

**Next move:** Push to origin main, monitor CI, deploy to staging, trigger
sourcecycled cycle, verify one real article.
