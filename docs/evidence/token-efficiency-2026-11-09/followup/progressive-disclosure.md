# Progressive disclosure of the choir verb surface (read-only investigation, 2026-11-09)

Code read at the checkout `/Users/wiz/Dev/go-choir/.claude/worktrees/followup-token` (main = 5dc98bcf5). No model call, no write to any computer. Labels: (M) measured, with the command or method; (I) inferred; (H) hypothesis. Rulings 1-9 in `followup/rulings.md` are obeyed; ruling 3 (never trim or rewrite earlier bytes) and ruling 5 (progressive disclosure, cache-aware) drive the design.

Scratch data: `$SCR/pd/` (`runs-paper.json`, `ev/` = 231 paper runs' events, `evde/` = 115 d@e runs' events, `surface-<desk>.txt`, `sigs-<desk>.txt`, analysis scripts `an.py`, `an2.py`). `$SCR` = `/private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad`.

## 0. Bottom line

1. The cut is real but smaller in dollars than it looks once caching is on. Verb documentation is 1.4k to 4.3k tokens per desk (M). Cutting it saves about 2.9M input tokens/h on the paper (I), 7% of the 41M/h in the audit window. At Haiku 5.5 prices that is about $6.8/day uncached and about $0.7/day at the cache-read rate (I, arithmetic in section 5). Against the $20/day budget, it is a hygiene and latency lever, not the money lever. It matters on routes that do not cache (crouton).
2. The bigger prize hides next to it: **10% of research cells fail** (130 of 1260, M), 16% of management-task cells (125 of 764), 9% of texture cells (26 of 285). Each failed cell is a whole extra model call carrying the full context. About 40% of the research failures are API-shape errors (wrong result field, wrong return arity, wrong argument type, undefined name) that a better hint or a result-field line in the prompt would prevent. That is worth about as much input as the verb-surface cut on an uncached route, and nothing on this side of the design adds a round trip.
3. Today `choir.Help` is not a discovery tool. `Help("Cast")` returns the same sentence the prompt already carries (no example, no return fields), and `Help()` returns the whole prompt block again (about 4.3k tokens for research). Models already call it in 27% of texture runs, 52% of management-task runs and 14% of research runs (M), mostly for a verb whose full doc is already in the prompt. The first work is making Help worth calling.
4. Recommended shape: keep one byte-stable system prompt per desk with (a) a REPL guide (how a cell works, how to discover), (b) a **hot card** of the verbs this desk calls in most runs, with signature, result fields and one literal example, and (c) one line of **cold verb names** (names only, grouped). Everything else comes from `Help("X")` as an appended tool result. Names stay in the prefix because the best evidence on progressive disclosure (Vercel, 56% of cases never invoked the on-demand skill) says an agent does not look up what it does not know exists.
5. Do not apply the full scheme to texture and management-task. Their surfaces are small and almost all hot; an extra Help round trip on a 3.7-call run costs more than the tokens saved. For them, drop only the cold verbs and fix the Cast/Feed/Decline docs so the model stops spending a cell on `Help("Cast")`.

## A. What exists

### A1. Where each desk's prompt is assembled (code)

- `internal/agentcore/tool_profiles.go:184` `systemPromptForRun`: core (`run_context.go:33`, `promptstore/defaults/core.yaml`, 1,261 chars, shared by every desk) + "Role-specific instructions" (`promptstore` role yaml: conductor 1.5k chars, engineering 2.7k, management 6.7k, research 5.6k) + overlays (`runtimeprompts/overlays/`: `rlm_research_runtime.yaml` 13.1k chars, `rlm_management_runtime.yaml` 27.6k, `rlm_engineering_runtime.yaml` 30.1k, `rlm_management_task_runtime.yaml` 5.2k; texture's prompt is `textureprompts/texture.yaml` 8.8k) + the generated verb block (`deskSurfacePrompt`, `tool_profiles.go:~476-492`, which calls `yaegikernel.DeskSurface`) + `PeerCoordination` (`tool_profiles.go:266`, 554 chars).
- Management task prompt: `managementTaskSystemPrompt` (`tool_profiles.go:~460-470`): core + task overlay + `DeskSurface(taskRole, "", true)`.
- The prompt is byte-stable per desk by design (doc comment at `tool_profiles.go:~174-183`; per-run facts and the time go into the run-context turn, `run_context.go:46`). Three variant axes remain in the prefix: management's `intakeLabels` and `eventWakes` computer settings (up to 4 variants; `tool_profiles.go:~228-260`, with `hidden = yaegikernel.IntakeVerbs` at `:260`), engineering's overlay flags (4 variants) plus the verifier slot's surface, and per-model failover tagging of the cache key.
- Tool definition: one tool per desk, `desk_go_eval` (`tools_desk.go:585-600`; text is identical for every desk, about 300 tokens, M by the audit) and `capsule_go_eval` for engineering (`tools_capsule*.go:994`).

### A2. Where the verb lists come from

- Single source: `internal/yaegikernel/surface.go`. `choirFunctionDocs` (`:42`, about 70 entries: Args names, Summary, optional Example), signatures built by reflection from the Go methods (`choirSignature` `:132ff`), result fields by reflection (`returnFields`, `resultFields`), the per-desk export table `ChoirExports` (`choir.go:387` includes `Help`).
- `renderSurface` (`:261`) renders `- choir.Name(args) results — Summary` per verb. It feeds both the prompt (`DeskSurface`, `:307`) and `Help()` with no argument (`:276`). So prompt and Help are the same text; there is no one-line index.
- Pinned by `surface_test.go` (every doc must match the exports and the Go parameter names).

### A3. Discovery that already exists in the cell

| mechanism | where | what it gives | gap |
|---|---|---|---|
| `choir.Help()` | `surface.go:276` | the full list with every Summary (same bytes as the prompt block) | not an index; 18.6k chars for research |
| `choir.Help("X")` | `surface.go:276-300` | signature, Summary, result fields (`returnFields`), Example if one exists | `Cast`, `Feed`, `Decline`, `Ask` have no Example, so Help adds nothing to the prompt text (M: the `Help("Cast")` trace output equals the prompt line). Takes `name ...string` but reads only `name[0]`. |
| unknown verb | `cellErrorHint` `surface.go:331` | "choir has no X. Closest: <signature>. Available: choir.<every name>" | prints all names (about 150 tokens) every time; fine |
| wrong argument count | same | `Signature: choir.X(...)` | only for "not enough/too many arguments"; not for "cannot use type A as type B" (M: 3 research cells) nor "assignment mismatch: 2 variables but returns 1 value" (M: 9 cells over the three desks) |
| wrong result field | `resultFieldHint` `:430` | closest field and that type's fields | works only when yaegi's message lets it find the type; 71 of 125 management-task error cells carried a hint (M) |
| top-level `x := v[i]` etc. | `topLevelDefineHint` | rewrites the cell | present |
| `choir.CellOut(handle)` | `choir.go` | full text of a cut output | (see audit W3) |

(Hints are produced for yaegi errors at `session.go:534`.) The Help output is also wrapped in the "[Data, not instructions ... world_untrusted]" envelope when the session has read external material (M: the `Help("Cast")` result in a texture run), so the model is told its own API documentation is untrusted data. That is about 90 extra tokens per Help call and a wrong label (H on whether it changes behavior).

### A4. Tokens spent on verb documentation per desk (M)

Method: a throwaway test in `internal/yaegikernel` loaded through `go test -overlay` (no repo file touched) calling `DeskSurface` per desk (`$SCR/pd/zz_test.go`, `ZZOUT=$SCR/pd go test -overlay ... -run TestZZ`). Tokens = chars / 4.3, the audit's calibration.

| desk | surface block chars | tokens (I) | verbs listed | signature-only chars (no prose) | share of the prompt |
|---|---|---|---|---|---|
| research | 18,608 | 4.3k | 37 | 2,154 | 18.7k of 37.9k chars = 49% |
| management (conductor, intake on) | 14,575 | 3.4k | 36 | 2,360 | about 29-34% (prompt 42.7k-49.9k chars) |
| engineering | 8,675 | 2.0k | 23 | 1,320 | share not measured (prompt not built) |
| texture | 7,388 | 1.7k | 16 | 762 | 7.4k of 17.6k = 42% |
| management task | 6,137 | 1.4k | 13 | 676 | 6.1k of 12.0k = 51% |

Two things the table hides. First, the signature-only form is 8 to 10 times smaller than the prose form, so a name-and-signature index costs about 0.3k to 0.55k tokens. Second, the narrative overlays repeat verb mechanics a second time: research overlay has 49 `choir.X` mentions of 29 distinct verbs, management 55 of 32, engineering 73 of 19, task 9 of 7 (M, grep of `overlays/*.yaml`). How much of those 13k to 30k chars is duplicate verb explanation I did not measure (the research Search paragraph at `rlm_research_runtime.yaml:19` is one example). This is the part of "simplify the prompts" that needs per-sentence trace evidence, and it is outside this report's numbers.

### A5. What the desks actually call (traces)

Sources: paper `GET /api/runs?limit=500` (17:29 to 19:57 UTC today) then `GET /api/runs/{id}/events?limit=500` for 70 research, 71 management-task and 90 sampled texture runs (231 runs, 2,309 cells); d@e the same for 40 management, 27 engineering and 15 research runs (82 runs, 2,832 cells). Verb = a `choir.X(` token in the cell source of a `tool.result` event (a verb inside a comment or string would also count). Scripts: `$SCR/pd/an.py`, `an2.py`.

Fraction of runs that call each verb (M):

- **research (paper, 70 runs; 37 listed, 19 distinct called)**: StoryAnalysis 99%, Records 99%, SearchWireCorpus 97%, SearchStart 97%, ReportPacket 97%, Read 97%, Searches 96%, Relate 93%, ReadCapture 90%, Captures 84%, SearchWait 81%, CellOut 47%, Reads 37%, Context 31%, ListEvidence 17%, Help 14%, System1 7%, Pack 1%. Never called in 70 runs: 18 of the 37 listed, including the largest docs after Relate: SourceAPI (1.4k chars), RelateStories (1.3k), Research (1.3k). Hot set (at least 80% of runs) = 11 verbs.
- **management task (paper, 71 runs; 13 listed, 9 called)**: Done 99%, Records 99%, Stories 92%, ReadCapture 92%, Cast 75%, Help 52%, Feed 45%, CellOut 45%, Decline 14%.
- **texture (paper, 90 runs; 16 listed, 12 called)**: Write 89%, ReadCapture 76%, Done 54%, Wait 32%, Cast 30%, Help 27%, ReadDoc 23%, CellOut 17%; Records 3%, Updates 1%, Reject 1%, Context 1%; Pack, Ask, ListDir, ReadFile never.
- **engineering (d@e, 27 runs, 80 cells per run; 23 listed)**: Exec 100%, Precommit 100%, Records 96%, Complete 96%, Context 89%, WriteFile 89%, Start 85%, ReadFile 78%, Freeze 74%, ListDir 70%, Poll 63%, Edit 59%, Report 30%, Ask 22%, EditMulti 22%, Help 19%; the rest at 4% or less.
- **management (d@e, 40 runs)**: Records 100%, Help 55%, Pack 55%, Verify 42%, VerifyFindings 33%, Context 33%, Screens 30%, Ask 25%, Updates 25%, ProductAPI 23%, Cast 23%, ShotStart 20%. The paper's conductor ran no REPL cell in my samples (the 71 paper "management" runs are all tasks; the d@e "conductor" runs have 0 cells), so the conductor hot set is d@e-only evidence.

The long tail is long but thin: the verbs outside the hot set add up to 40% to 60% of the listed verbs and carry the longest docs for research.

### A6. Help use and cell failures (M)

How often models already call Help, and what for:

| desk | cells | Help cells | runs with Help | what they ask for |
|---|---|---|---|---|
| texture (paper) | 285 | 28 (9.8%) | 27% | Cast 23, Write 8, Wait 2 |
| management task (paper) | 764 | 53 (6.9%) | 52% | Cast 32, Feed 14, Decline 6, Records 5, Stories 5 |
| research (paper) | 1260 | 13 (1.0%) | 14% | Relate 6, Searches.Get 3, one each of 6 others |
| management (d@e) | 337 | 55% of runs | | Verify 16, VerifyFindings 13, ReportPacket 7, Ask 7, Screens 5, ShotVerdict 5 |
| engineering (d@e) | 2172 | 19% of runs | | Complete 2, Records, Report, Freeze |

Position: in texture and management-task the Help cell is almost always cell 2 (24 of 28 and 43 of 53 are a lone `fmt.Println(choir.Help("X"))`). So the model reads the frame in cell 1, then spends a full model call on documentation that the prompt already holds. Average Help result 2.3k to 2.6k chars (about 0.6k tokens) in these desks, 5.0k chars in research. The verbs asked for are the ones with a contract to get right: Cast and Feed and Decline (a `req` map with p_success, weight, settlement), Verify/VerifyFindings (structured findings), Relate (1,896 chars of doc), Write (1,576 chars). That is a hint about what a hot card needs: not just the signature but a copyable literal call.

Cell failures (is_error or non-empty error/diag/stderr; classified by regex on the message; the "no text" class is `tool_error: reduce:` failures that commit-time checks return, e.g. "the story library moved since this run last read choir.Stories()", "source entity graph count does not match"):

| class | research (1260 cells) | mgmt task (764) | texture (285) | d@e mgmt (337) | d@e eng (2172) |
|---|---|---|---|---|---|
| errors total | 130 (10.3%) | 125 (16.4%) | 26 (9.1%) | 64 (19%) | 118 (5.4%) |
| wrong field or method on a result | 35 | 64 | 7 | 15 | 6 |
| runtime panic (slice or nil) | 43 | 14 | 4 | 7 | 16 |
| syntax or parse (JSON-in-Go, unterminated string, `func` at top level) | 22 | 7 | 2 | 2 | 12 |
| undefined name | 8 | 3 | 0 | 0 | 5 |
| assignment mismatch (wrong return arity) | 5 | 1 | 3 | 0 | 0 |
| wrong argument count or type | 3 | 0 | 0 | 0 | 1 |
| commit-time refusal, no text | 3 | 31 | 10 | 40 | 78 (mostly command failures) |
| other | 11 | 5 | 0 | 0 | 0 |

Reading: the API-shape classes (wrong field, arity, argument type, undefined name) are 51 of 130 research errors (39%) and 68 of 125 management-task errors (54%). A signature hint helps only the last two; the dominant class is "which fields does this result have" (examples: `r.Cast.Objective`, `r.To`, `page.Items`, `doc.RevisionId`, `cap.Title`; yaegi says "undefined method X"). Those need the result type's field list at the moment of the error or, better, up front for the hot verbs. The slice panics (33% of research errors) are model Go (`s[:3000]` on a shorter string; message `reflect.Value.Slice: string slice index out of bounds`); no verb documentation prevents them, but a one-line REPL rule and a hint on that panic text would.

The research.md rate (9.3%) and mine (10.3%) agree within the different sampling window and my wider definition.

## B. Research: progressive disclosure of tools and skills

All fetched 2026-11-09; dates are the pages' own.

### B1. What is reported

1. **Anthropic, Agent Skills (2025-10-16)**, https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills. Progressive disclosure is "the core design principle". Three levels: name and description of every skill in the system prompt at startup; the SKILL.md body only when the skill looks relevant; linked files only when needed. Guidance: split a large file into referenced files, keep rarely combined contexts on separate paths, write the name and description carefully because they decide triggering, start from an evaluation of where the agent struggles. The post reports **no measurements**.
2. **Anthropic, Code execution with MCP (2025-11-04)**, https://www.anthropic.com/engineering/code-execution-with-mcp. Tools as a code API in a directory tree (`servers/<server>/<tool>.ts`); "the agent discovers tools by exploring the filesystem" or through a `search_tools` tool with a detail-level parameter (names only, names plus descriptions, full schema). Claims 150,000 tokens down to 2,000 (98.7%) for the worked example; the page does not say how the baseline was built. Costs named: a secure sandbox, resource limits and monitoring. This is the closest published analogue of Choir: tools are code in a REPL, and the question is how much of the API is shown before the first cell.
3. **Anthropic, Advanced tool use (2025-11-24)**, https://www.anthropic.com/engineering/advanced-tool-use. Tool Search Tool: 50+ MCP tools about 77K tokens up front versus about 8.7K (85% less); MCP evals Opus 4 49% to 74% and Opus 4.5 79.5% to 88.1% with it on (Anthropic internal tests). "Doesn't break prompt caching because deferred tools are excluded from the initial prompt entirely." Keep the 3 to 5 most-used tools always loaded and defer the rest; it is less useful under 10 tools or when every tool is used in every session. Tool use examples: 72% to 90% on complex parameter handling. Programmatic tool calling: 43,588 to 27,297 tokens (37%) on research tasks.
4. **Anthropic docs, Tool search tool** (read 2026-11-09), https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-search-tool. Selection accuracy "degrades once you exceed 30-50 available tools"; a discovered tool arrives as a `tool_reference` block appended inline in the conversation, then expanded; "Claude can reuse discovered tools in later turns without re-searching"; use it at 10 or more tools or more than 10k tokens of definitions, not for fewer than 10 tools or every-tool-every-request; keep 3 to 5 non-deferred; "Add a system prompt section describing available tool categories"; consistent name prefixes so one search matches a group. A tool with `defer_loading` cannot carry `cache_control`.
5. **Anthropic docs, Tool use with prompt caching** (read 2026-11-09), https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-use-with-prompt-caching. Modifying tool definitions invalidates the entire cache; appending a discovered definition inline leaves the prefix untouched. The cache is hierarchical, tools then system then messages, up to 4 breakpoints, lookback of at most 20 positions per breakpoint.
6. **Anthropic docs, Prompt caching** (read 2026-11-09), https://platform.claude.com/docs/en/build-with-claude/prompt-caching. Minimum cacheable prompt: 512 tokens for Haiku 5.5 and Sonnet 5.5, 1,024 for Sonnet 5, 4,096 for Haiku 4.5. 5-minute TTL refreshed on every hit, 1-hour option. Write 1.25x, read 0.1x (Haiku 5.5 at or under 100k tokens: $0.10 base, $0.125 write, $0.01 read per M).
7. **Anthropic, Writing tools for agents (2025-09-11)**, https://www.anthropic.com/engineering/writing-tools-for-agents. "Prompt-engineer your error responses" so they are actionable, with examples of correct input; a concise versus detailed response format (206 versus 72 tokens in the example); describe a tool as to a new hire; unambiguous parameter names; consolidate overlapping tools. Description edits alone improved SWE-bench Verified results.
8. **Anthropic, Effective context engineering (2025-09-29)**, https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents. Hybrid is best: some context up front, the rest by just-in-time exploration; "runtime exploration is slower than retrieving pre-computed data" and needs guidance so the agent does not waste context on misused tools; curate a minimal viable tool set.
9. **Vercel, AGENTS.md outperforms skills (2026-01-27)**, https://vercel.com/blog/agents-md-outperforms-skills-in-our-agent-evals. Next.js 16 API tasks outside training data. No docs 53%; skill with default behavior 53% (the skill was **never invoked in 56% of cases**); skill with explicit instruction 79%; an 8KB pipe-delimited docs **index in the always-loaded file 100%**. A 40KB index was compressed to 8KB with the same score. The author's working theory: no decision point, always available, no ordering problem. Vendor's own suite, case count and variance not reported; "You MUST invoke" wording was brittle.
10. **Cloudflare, Code Mode (2025-09-26)**, https://blog.cloudflare.com/code-mode/. Models are better at writing code than at emitting tool calls (they have seen much code and few tool calls); the MCP schema becomes TypeScript with doc comments, all loaded into context; the post reports no numbers and notes the whole API in context may not scale. **InfoQ on Cloudflare's MCP server (2026-04-16)**, https://infoq.com/news/2026/04/cloudflare-code-mode-mcp-server: two tools, `search()` over the OpenAPI spec and `execute()` of generated code; over 2,500 endpoints, 1.17 million tokens down to about 1,000 (Cloudflare's own figure). The spec never enters the model context; the footprint is fixed regardless of API size.
11. **OpenAI function calling and tool search guides**, https://developers.openai.com/api/docs/guides/function-calling and https://developers.openai.com/api/docs/guides/tools-tool-search (search results only; pages not opened): keep the number of initially available functions small, evaluate with different counts, defer rarely used functions with `defer_loading`, group deferred functions in namespaces. No numeric cutoff found.
12. **RAG-MCP, arXiv 2505.03275 (2025-05-06)**, https://arxiv.org/abs/2505.03275. Retrieval of tool descriptions before the call: tool selection accuracy 43.13% versus 13.62% baseline, prompt tokens down by over 50% (abstract; per-tool-count curve not in the abstract, secondary sources quote 85% at 5 tools to 45% at 20 which I could not confirm).
13. **Tool Documentation Enables Zero-Shot Tool-Usage, arXiv 2308.00675 (2023-08)**, https://arxiv.org/abs/2308.00675. Documentation-only prompts match few-shot prompts on existing benchmarks and beat demonstrations on a new dataset of hundreds of tools (a secondary summary quotes F1 0.45 versus 0.19 on a CLI task; not confirmed). Evidence that the unit to disclose is the doc, not an example set.
14. **Manus, Context engineering lessons**, https://manus.im/blog/Context-Engineering-for-AI-Agents-Lessons-from-Building-Manus (read through a search summary, not opened). KV-cache hit rate is "the single most important metric"; keep context append-only, serialization deterministic; avoid adding or removing tools mid-iteration (it changes the prefix and confuses earlier actions that name removed tools); constrain choices by masking and name prefixes instead. Claude Code's own deferred tools: the 85% to 95% savings figures in blog posts are secondary and disagree; open bug reports say auto mode sometimes failed to trigger (unverified).

### B2. Design rules extracted

R1. The scale threshold matters. Vendors put the payoff at 10+ tools or over 10k tokens of definitions, and say fewer than 10 tools or every-tool-every-run is better loaded up front. Choir's desks are 13 to 37 verbs and 1.4k to 4.3k tokens (M): research and the conductor are over the line on count, none is over it on tokens. The right answer is per desk, not uniform.
R2. Keep the hot set resident, defer the tail: "3 to 5 most-used always loaded" (Anthropic), a passive index beats on-demand lookup (Vercel). A hot set of 6 to 12 verbs matches Choir's data (section A5).
R3. Never hide existence. Skills' level 1 is name plus description for everything; Vercel's skill went unused in 56% of cases when only on-demand access existed; Anthropic suggests a prompt line naming tool categories. Keep one line of names in the prefix.
R4. Disclosure must append, never rewrite. Deferred definitions arrive as appended blocks and "reuse in later turns without re-searching"; a changed tool list invalidates everything (Anthropic caching doc), and the Manus rule is append-only plus deterministic serialization. Choir already satisfies this if Help output is a normal tool result.
R5. Detail level is a parameter (names, names+descriptions, full schema in the MCP-code article; concise versus detailed in the tools article). Map to `Help()` (index), `Help("X")` (signature, fields, example), `Help("X","Y")` (several at once).
R6. Errors are the cheapest discovery channel: actionable, with a corrected usage example, not a trace. The model gets it exactly when it has the need.
R7. Teach with the unit that works: the doc, plus a literal example for structured parameters (tool use examples 72% to 90%; documentation beats demonstrations at hundreds of tools).
R8. Discovery costs a turn. Search adds a step; "runtime exploration is slower"; the saving must exceed the extra latency and cost. Batch discovery in one call.
R9. Measured effects are vendor-internal and on MCP-style catalogs of 50 to 2,500 tools, never a 13 to 37 verb REPL. Treat 85% and 98.7% as upper bounds from a different regime. Choir's own number is the 34% to 79% shrink of one block (section 5).

### B3. Failure modes

- The model does not know a capability exists (Vercel 56%; skill triggering depends on description wording). Mitigation: names in the prefix; the hot card; hints that name the closest verb.
- Extra discovery round trips: each is a model call with the whole context. Under caching a Help call costs about 10 times what its own result tokens cost, because the cached context is re-read (section 5).
- Cache invalidation when definitions change: any edit of tools or system rewrites the key; a per-state verb list in the prefix (Choir's intake-label variants, engineering slots) multiplies cache entries. Appending avoids it, and a prompt-version rollout is one cache write per desk key.
- Reuse loss: a discovered tool is remembered only while its result stays in the transcript. Choir never trims, so it stays (ruling 3). A new run starts a new transcript and the model must rediscover; the hot card is what prevents that.
- Over-specified triggers: "You MUST invoke" caused missed context in Vercel's test; write plain facts about the REPL, not escalating imperatives.
- Wrong assumption of the doc: Help output that contradicts the prompt or hides a refusal (Choir's `hidden` intake verbs: Help must not list what the host refuses; `surface_test.go:151` pins it).

## C. Design: cache-aware progressive disclosure for the Choir REPL

### C1. Prefix layout (byte-stable for the whole run)

```
tools:   [desk_go_eval]                       one definition, identical on every desk (capsule_go_eval for engineering: unify the text, keep the name)
system:  block A  SHARED:  core + REPL guide (about 0.9k tokens)
         block B  PER-DESK: role prompt + overlays (unchanged for now) + hot card + cold names + PeerCoordination
messages: run-context turn (time, ids) ... transcript, append-only
```

- Block A is first in the system prompt, so it is the longest prefix identical across desks. With tools it is about 1.2k tokens (I: 300 tool + 300 core + 600 guide), above Haiku 5.5's 512-token minimum, so a breakpoint after A is valid. On Bedrock with explicit breakpoints, put breakpoint 1 after A and breakpoint 2 at the end of B (the existing system-block breakpoint) and the history breakpoint stays as is (`markHistoryCacheBreakpoint`). The cache hierarchy allows 4. DeepSeek and Synthetic cache the longest matching prefix automatically, so A is shared without any marker.
- Honest value of the shared block: the paper's desks are all warm inside the 5-minute TTL (research alone is about 710 requests/h), so each desk's own prefix is already a hit; sharing A saves cache writes only for a cold or rare desk (engineering, the conductor, a fresh computer, d@e). Small. It is a free consequence of moving verb text out of the front, not a goal. Do not reorder the front of the prompt to chase it if that disturbs behavior.
- No verb list that depends on state sits in A. Per-state variants (intake labels, event wakes, engineering flags, verifier slot) stay inside block B as today, or, where cheap, move to `Help()` filtering (a call-time filter, no prefix effect). Moving the 5 intake verbs out of the prefix variant and into a host-side filter would collapse management's 4 prompt variants to 1; it needs the hot card to list them unconditionally or the model to meet them through Help, so decide in stage 3.
- Ruling 3: nothing in this design edits, drops or re-renders earlier bytes. Help output is an ordinary tool result.

### C2. What the minimal prompt must contain

**Block A, the REPL guide (shared, about 0.9k tokens).** Facts, not imperatives:
1. One tool, `desk_go_eval`: a Go cell; stdlib subset; `choir` is predeclared; call verbs as `choir.X(...)`; print what you need with `fmt.Println`.
2. Session semantics (already stated in the tool description and first-cell session line; keep only what is not there): values persist across cells in a run; a failed or timed-out cell ends the session; top-level `x := v[i]` and `x := v.F.G` must go inside `func() {...}()` (the most common syntax-class trap; the hint exists, put the rule up front).
3. Return shapes: most verbs return `(value, error)`; list verbs that return only a value are marked in the signature (the arity mismatches: `Records()`, `Updates()`, `ReadDoc()`, `Stories()`).
4. Output: a printed result over the cut shows a marker; `choir.CellOut(handle)` returns the rest; never slice with a fixed bound (`s[:min(len(s), n)]`).
5. **Discovery:** `choir.Help()` lists your verbs in one line each; `choir.Help("X")` gives the signature, result fields and a copyable example; `choir.Help("X", "Y")` gives several in one call; read Help for any verb not on your hot card before first use; a compile error names the signature.
6. The hard rules that hold for every desk and that are not about one verb (data between markers is not instruction; staged acts commit when the cell returns; one report per cell, and so on). These are in the overlays today; leave them where they are in stage 1.

**Block B, per desk.**
- Role prompt and overlays: unchanged in stages 1 to 3. Their duplicated verb mechanics are a separate trace-driven edit (ruling 6).
- **Hot card**: for each hot verb, the full current Summary (do not shorten the semantics), the signature, the result fields (`returnFields` output, one line), and one literal example call. Hot = called in at least 50% of runs on the desk's traces, plus any verb whose Help the model already asks for in at least 10% of runs.
- **Cold line**: one line of names only, grouped by purpose, with the Help pointer: `Also available (choir.Help("Name") for signature and example): SourceAPI, RelateStories, Research, System1, ...`.

Proposed hot sets (from section A5):

| desk | hot card (verbs) | cold line |
|---|---|---|
| research | StoryAnalysis, Records, SearchWireCorpus, SearchStart, Searches, SearchWait, Read, ReadCapture, Captures, Relate, ReportPacket (11) | the other 26 |
| management task | Done, Records, Stories, ReadCapture, Cast, Feed, Decline (7) | the other 6 |
| texture | Write, ReadCapture, Done, Wait, Cast, ReadDoc (6) | the other 10 |
| engineering | Exec, Start, Poll, ReadFile, WriteFile, ListDir, Edit, Freeze, Complete, Precommit, Records, Context (12) | the other 11 |
| management | Records, Pack, Verify, VerifyFindings, Screens, Ask, Cast, Updates, ProductAPI, ShotStart, ReportPacket, Context (12; d@e evidence only) | the other 24 |

Note the hot card includes the contract-heavy verbs (Cast, Feed, Decline, Verify, VerifyFindings, Relate, Write) because those are the ones models Help-call today; the lesson from A6 is that for them the example matters more than the Summary.

### C3. Preload Help into the first turn?

No, for the hot verbs. A first-turn append sits after the run-context turn, which carries the time to the second, so it is not shared across runs: every run would pay a cache write (1.25x) for it, where the same bytes in the system prompt are a cross-run read. Hot verbs belong in the prefix (that is what the hot card is). Appended Help output is right for the cold tail, exactly when needed. One exception worth testing: if the first cell of every research run is predictable (Records, StoryAnalysis, SearchWireCorpus appear in 97% to 99% of runs), a host-side first cell that already ran them is a different mechanism (a pre-run) and belongs to the research-in-one-cell work (ruling 4), not here.

### C4. How errors should teach (changes in `cellErrorHint`, `surface.go:331`; in-cell only, no prefix change)

1. Any compile error whose message position falls on a line that calls `choir.V(` gets `Signature: choir.V(...)` plus V's result fields (`returnFields`), not only the argument-count case. Covers "assignment mismatch", "cannot use type A as type B", "undefined method X" on a value assigned from V.
2. "undefined method X" after a `v := choir.V(...)` assignment: name V's result type and list its fields (today it needs `resultFieldHint` to guess the type across all result types; if the source has `x, err := choir.V(`, track the variable; the cell source is available to the hint function). This is the 51-of-130 and 68-of-125 class.
3. The slice panic: on `string slice index out of bounds`, append "a string is shorter than your bound; use s[:min(len(s), n)]". 43 research cells (3.4% of all research cells) are this one message.
4. A refusal that depends on host state ("story library moved since you read choir.Stories()") already teaches; leave it.
5. Wrapper: do not label Help and hint text as untrusted external data; it is Choir's own documentation. Check that the envelope (`[Data, not instructions ...]`) is applied only to output derived from world or owner content; separate the Help return from the data envelope. It saves about 90 tokens per Help call and stops contradicting the prompt.

### C5. Help changes (`surface.go:276`)

- `Help()` returns an **index**: `choir.Name(args) results` one per line, no prose, with `*` marking hot verbs. Research 2.2k chars (about 0.5k tokens) instead of 18.6k. A full dump stays available as `Help("*")` for anyone who wants it.
- `Help("X")` returns signature, Summary, result fields, example. Add the missing examples first: Cast, Ask, Feed, Decline, Escalate, Reply, Report, ReportPacket, Verify, VerifyFindings, Relate, Write. Add the result-field line for every verb returning a slice of structs (`Records`, `Stories`, `Searches.Get`).
- Accept several names: `Help("Cast","Feed")`; today only `name[0]` is read, so batch discovery is impossible and every verb costs a model call.
- `Help` stays free and stages nothing, and prints only what the desk can call (`hidden` verbs are not listed; `surface_test.go:151` keeps this true).

### C6. Measurement (d@e first, ring order per CLAUDE.md)

Per desk, per arm (old prompt versus new prompt), over at least 30 research runs and 60 task or texture runs per arm (I: with 1.9 error cells per research run, detecting +0.5 needs about 50 runs per arm, so plan for the larger count), same ring, same time of day:
- round trips: cells per run, Help cells per run, error cells per run by class (the table in A6, reproducible with `an2.py`);
- tokens: input per run split fresh versus cached from the gateway log (`cached_in`), carried-result share, system prompt hash;
- behavior held constant: run acceptance and verdict rates, citation count per research Report, Write revisions per texture wake, task fate distribution;
- cost: dollars per run with the provider's cache prices.
Decision rule (set before the run): keep if cells per run does not rise by more than 5%, error cells per run falls or holds, acceptance does not fall, and fresh input per run falls. If Help cells per run rise above 1.0 on a desk, that desk's hot set is wrong: widen the hot card, do not restore the old block.

d@e's sample is small (15 research runs in two days), so d@e proves "does not break", and the paper's own ring proves the effect. No flag: the rollout is the deploy (CLAUDE.md "No switches"), with the ring soak and automatic rollback as the guard.

## 5. Before and after

Per-call tokens of the verb block (tokens = chars / 4.3). After = hot verbs at their current full doc + cold names line + (index mode of Help is free until called). The "tightened" column assumes the hot docs are also edited down (Relate 1.9k chars to about 0.9k, repeated mechanics removed): that edit needs trace evidence per sentence, so treat it as upside.

| desk | before | after (hot verbatim) | saved/call | tightened hot cards | calls/h (audit) | saved tokens/h |
|---|---|---|---|---|---|---|
| research | 4.3k | 1.7k (hot 1.5k + 0.2k names) | 2.6k (-61%) | 1.2k, saves 3.1k | 710 | 1.85M |
| management conductor | 3.4k | 1.4k | 2.0k (-59%, low confidence, d@e data) | | 240 | 0.48M |
| engineering | 2.0k | 1.05k | 0.95k (-48%) | | not in paper | 0 on the paper |
| texture | 1.7k | 1.0k | 0.7k (-41%) | | 460 | 0.32M |
| management task | 1.4k | 0.95k | 0.5k (-34%) | | 400 | 0.20M |
| paper total | | | | | | **2.85M/h, 68M/day** |

2.85M/h is 7% of the 41M/h input in the audit's window and 24% of the 11.8M/h fixed-prompt volume (I; both from the audit). Add the C4 error-teaching: if 40% of the research API-shape failures disappear, that is 0.73 fewer error cells per run of about 18 calls, a 4% cut in research input at no added call (I). Together on an uncached route the scheme is worth about 11% of research input.

Dollars at Haiku 5.5 (≤100k prompts: $0.10/M input, $0.01/M cache read, $0.125/M cache write; fetched from the pricing table above), (I):
- Uncached: 68M tokens/day x $0.10/M = $6.8/day.
- Cached (the route the paper runs when cache works): 68M x $0.01/M = $0.68/day, plus a one-time cache write of 4 or 5 desk prefixes after rollout (negligible).
- Added Help round trips: a Help call carries the whole context. Research averages 22.3k per call, so a Help call is about 22k x $0.01/M = $0.00022 cached ($0.0022 uncached). A research run saves about 18 calls x 2.6k = 47k tokens = $0.00047 cached ($0.0047 uncached). Break-even: about 2 extra Help calls per run on either route (current rate 0.19 per research run; stage 2 should expect 0.3 to 0.8). Net research saving cached: 39 runs/h x ($0.00047 - 0.6 x $0.00022) x 24 = about $0.31/day; uncached about $3.5/day. For texture (3.7 calls per run, 14.4k context): saves 3.7 x 0.7k = 2.6k tokens ($0.000026 cached) while one Help call costs $0.00014 cached: break-even is 0.19 Help calls per run, and texture runs are at 0.31 already. This is the arithmetic behind keeping texture's hot verbs resident and dropping only its cold names.
- Conclusion for the budget (ruling 9): under working caches this is about half a dollar a day. Its justification is: routes without caching, first-call latency (prefill of 2k to 3k fewer tokens, H, unmeasured), a smaller prompt for the model to attend over (H), and fewer API-shape errors (I, the larger lever).

## 6. Staged rollout (behavior held constant)

0. Baseline (done in this report; rerun `an.py` / `an2.py` per stage): runs, cells per run, Help per run, error classes, per desk.
1. **In-cell, no prefix change** (ship on, no flag): C4 hints (signature and result fields on any choir call error, slice panic hint, untrusted-envelope off for Help), C5 Help (index mode for `Help()`, multi-name, examples for the contract-heavy verbs, `Help("*")` for the old dump). The prompt is unchanged, so no cache key moves. Expected: fewer error cells; Help gets a reason to be called. Go/no-go: error cells per run down, nothing else moves. Existing tests extend `surface_test.go` (Help lists exactly the desk surface; every hot verb exists in exports; Help("X") text contains the old Summary, so nothing the prompt used to say is lost).
2. **Research** (the large surface and the high Help-friendliness): swap the surface block for hot card plus cold line plus the REPL guide's discovery paragraph. One prompt version, so one cache write per desk key. d@e first (15+ runs, "does not break"), then the ring walk. Check Help cells per run and cells per run against the rule in C6.
3. **Management and engineering**: same, hot sets from d@e plus paper once the conductor runs REPL cells. Decide whether the intake verbs move to host-side Help filtering (one management prefix instead of four).
4. **Texture and management task**: cold line only (drop the 10 and 6 cold verbs; keep every hot verb at full text; add the Cast, Feed, Decline examples so `Help("Cast")` cell 2 stops). No new Help dependence.
5. **Shared block A**: move the REPL guide and core to the front as a verbatim shared block; set the Bedrock breakpoint after it. Only if the cross-desk first-call hit shows up in `cached_in` for cold desks; otherwise skip.
6. Only after 1 to 5, the overlay prose edit (verb mechanics duplicated in `rlm_*_runtime.yaml`), with a per-sentence trace justification (ruling 6). It is the larger prompt-size lever (13k to 30k chars per overlay) and is not in the numbers above.

Rollback per stage: revert the commit (prompt text and hints are code), which moves the cache key back; no data to restore.

## 7. Risks, stated plainly

1. **Savings under caching are small** (about $0.7/day). If the owner's goal is money on a caching route, this is not the lever; research's 18-call run structure (ruling 4, one cell) and result carry (audit L2) are bigger. The case for this work is uncached routes, latency, error rate and prompt clarity.
2. **Help round trips can cost more than they save**, especially on the 3 to 4 call desks. The design avoids it by keeping hot verbs resident and requiring Help only for the cold tail, and by accepting several names per call; the decision rule in C6 watches for it.
3. **Hot sets come from one afternoon on the paper** (70 research, 71 task, 90 texture runs) and 2 days on d@e (15 research, 27 engineering, 40 management). Verbs used in 1% to 15% of runs (ListEvidence, System1, SourceAPI, EditMulti, Report) may be critical in the runs that need them; the cold line keeps them findable. A different news day or a self-development round changes the mix. Recompute before each stage.
4. **A model that never calls Help** meets a cold verb only through the name line. Names are descriptive (SaveEvidence, SearchTexture), but a model could write its own helper instead of calling `choir.SourceAPI`. The hint for unknown names covers a wrong guess, not a missing idea (Vercel: 56% never invoked the on-demand skill). Mitigation is the names line in the prefix and the trace metric "own helper re-implements a cold verb" (not yet measured, H).
5. **Model dependence.** The 85% and 98.7% figures and the accuracy gains are Anthropic- and Cloudflare-internal on large MCP catalogs. Haiku 5.5, DeepSeek and the other routes have different Help-calling habits; the paper may run two models. Test per model.
6. **The "no verb docs up front" instinct fails for contract verbs.** Cast, Feed, Decline, Verify, Relate, Write are exactly where models already need an example (Help("Cast") in 23 of 90 texture runs even though the text was in the prompt). Hence the hot card carries them with an example. Do not trim them to a signature.
7. **Cross-desk prefix sharing is nearly worthless on a warm paper** (section C1), and moving text to the front risks changing behavior (instruction order matters). Treat stage 5 as optional.
8. **Hints can lie.** Result-type tracking from the cell source is heuristic; a wrong "fields of X" hint costs a call. Keep hints conservative and test them against the 125 + 130 + 26 + 64 failing cells in `$SCR/pd/ev` and `evde` as a replay corpus (no model call needed).
9. **Sample sizes**: d@e cannot power the comparison; the paper ring does. Per-run variance of error cells (1.9 mean, long tail) needs about 50 runs per arm.
10. **Prompt changes break the cache once per desk key and per failover model tag.** Negligible in dollars; do not roll several prompt versions in one day if the cache metric is being read.
11. **I did not read** the OpenAI tool-search guide pages, the Manus post and the Claude Code deferred-tool internals directly (search summaries only; marked above), and I did not measure the duplicated verb mechanics in the overlays or the latency effect of a 2k-token shorter prompt.

## Commands

    # surface sizes per desk (no repo file written)
    cd .claude/worktrees/followup-token && go test -overlay $SCR/pd/overlay.json ./internal/yaegikernel -run TestZZ -v
    # runs and events
    $SCR/paper.sh '/api/runs?limit=500' ; $SCR/paper.sh '/api/runs/<id>/events?limit=500'   # same with de.sh
    # verb frequency, Help use, error classes
    python3 -I $SCR/pd/an.py ; python3 -I $SCR/pd/an2.py
    # prompt composition
    wc -c internal/runtimeprompts/overlays/* internal/promptstore/defaults/*
