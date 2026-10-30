# Texture re-engineering: why a turn takes N model calls, and what a 1–5-cell Texture looks like

Read-only research, 2026-10-12, against `main` a401c878. No repo file changed.
Prompt-size probes ran through `go test -overlay` with test files kept in the
scratchpad, so the tree stayed clean. Evidence classes:

- **[code]**: read in the source at the cited file:line.
- **[journal]**: Node B gateway journal (`go-choir-gateway`). It logs one line per model call: message count, tokens in and out, timing. It does not log content.
- **[doc]**: an existing repo receipt or problem doc.
- **[inference]**: my reading, not confirmed by a cell trace. Guest cell traces are not in Node B's journal, and the earlier `trace_snapshot.sh` scratchpad script no longer exists.

Mutation class of the eventual work: **red**, because it changes Texture canonical writes. Stages 0–2 below are green, yellow or orange.

---

## 0. Bottom line

1. **"~60 cells per turn" is not real.** It comes from reading a total as a
   per-turn figure. `docs/reports/persistent-desks-overnight-2026-10-11.md:25`
   says "Texture's turns fell to sixty-six cells with seven errors". The
   source receipt is `docs/archive/definitions/choir-pd1-persistent-research-desk-2026-10-11.md:249`:
   "Texture: 12 turns, 66 cells, 7 errors", which is about 5.5 cells per turn.
   Other measured averages:
   - S1e: 50 cells over 7 turns, about 7.1 (`…pd1…:264`).
   - The 2026-10-11 20:22Z demo: 47 calls over 9 turns, about 5.2 [journal].
   - S1c totals 87 cells with 34 errors (`persistent-desks-overnight…:15`).
     That is the only regime where single turns reached 13 and 18 cells
     (`docs/archive/problems/texture-probes-canonical-write-to-learn-schema-2026-10-11.md:7-9, 62-64`).
   - No doc says "60 cells per turn". A grep of `docs/` and `skills/` finds the
     figure only as the 66-cell and 87-cell totals.
2. **Cell count is not the main latency cost. Output size is.** On the
   20:22Z demo, Texture's 47 calls spent 655 s in the model [journal]:

   | Output size | Calls | Model time |
   |---|---|---|
   | small, under 500 tokens (reads, small fixes) | 25 | 63 s |
   | mid, 500–2,000 tokens | 10 | 96 s |
   | large, 2,000 tokens or more | 12 | **496 s (76%)** |

   Each turn ends in one 9–14.5k-output-token call, which is the write
   (`reasoning=high`). Example: 20:44:48–20:46:10, 82 s, 14,533 tokens out.
   For a document of about 1,700 words that is mostly reasoning plus a
   whole-document rewrite [inference]. Cutting cells from 5 to 1–2 saves
   about 60–150 s over 9 turns. Making each turn write *less* (block or
   section edits instead of full replace) and think less saves several
   times more.
3. **The cells that are not the write come from five structural causes.**
   Model "slips" are not the cause:
   - **The wake turn is pointer-only.** It names update ids and nothing else,
     so every turn needs at least one read cell.
   - **Context goes stale after reactivation.** The document and outline in
     context come from the first turn.
   - **The citation path cannot be done in one write.** You cannot write a
     new paragraph and cite it in the same apply. A full replace drops every
     citation and re-mints every block id.
   - **The Help text and the reducer disagree** on `controls`,
     `base_revision_id` and what `content` accepts.
   - **Mid-turn wake injections name reports the cell cannot see.**
4. **The prompt is about 28.4k characters of system prompt plus a 9.5–13k
   character policy tail** in the first user turn.

   | Part | Size (chars) |
   |---|---|
   | Role essay | 4,361 |
   | Run overlay | 14,378 |
   | Generated surface listing 23 functions | 4,851 |
   | Revision policy tail, first user turn | 9,485–13,001 |

   It describes about 6 real affordances with roughly 25 separately listed
   mismatches (§2). A core of about 3k characters can replace it (§4).

---

## 1. Why a Texture turn takes N model calls

### 1.1 How the turn is built [code]

**System prompt.** Assembled by `systemPromptForRun` (`internal/agentcore/tool_profiles.go:196-313`), in this order:

| Part | Source | Size (chars) |
|---|---|---|
| core | `internal/promptstore/defaults/core.yaml` | ~1.1k |
| temporal | `internal/runtimeprompts/overlays/temporal_context.yaml` | ~0.4k |
| role prompt | `internal/textureprompts/texture.yaml` | **4,361** (measured) |
| skill context: "mission-gradient", "cognitive-transform-portfolio" extracts | `internal/agentcore/skill_context.go:12-15, 48-56` | 0–2.6k, depending on the guest's `RUNTIME_SKILLS_ROOT` [inference] |
| run overlay | `internal/textureprompts/overlays/run_system.yaml` | **14,378** (measured) |
| "Lifecycle Texture dispatch authority" block | `tool_profiles.go:251-253` | ~1.05k |
| generated desk surface | `yaegikernel.DeskSurface`, `internal/yaegikernel/surface.go:211-214` | **4,851**, 23 functions (measured) |
| run context | `run_context.yaml` | ~0.4k |
| tool catalog for `desk_go_eval` | `toolregistry.BuildSystemPrompt`, `runtime.go:3432` | ~1k |

The gateway logged `system_chars=28361` for every Texture call in the demo [journal].

**First user turn of a fresh run.** Built by `buildAgentRevisionRequest`
(`internal/textureowner/texture_agent_revision.go:683-822`). It contains:

- the seed prompt;
- the current content;
- the outline, capped at 64 nodes or 4,000 bytes (`:828-830`);
- hard-requirement regex hints (`:622-662`);
- the policy tail `revision_policy.yaml`, measured at 9,485–13,001 characters.

**Reactivated runs carry their earlier memory.** The controller sets
`actor_reactivate_existing_memory=true` (`texture_controller.go:1036`). The
document text and outline in context are therefore from an older revision
(`docs/problems/texture-probes-…:41-59`).

**Wake turn.** Built by `buildCoagentUpdateUserMessages`
(`internal/agentcore/coagent_update_packet.go:91-189`). It is pointer-only
by design: update ids and routing, plus "The payloads are already bound
inside your cell as choir.Updates() — read them there" (`:161`). The cell
gets payloads, document head and outline only through
`choir.Updates()` and `choir.ReadDoc()` (`tools_desk.go:337-364`,
`choir.go:733-806`).

**Commit and turn end.** A cell stages `choir.ApplyTexture(map)` and the
reducer commits it after the cell (`rlm_reduce.go:883-887, 1043-1061`;
`texture_cell_author.go:52-96`). A successful commit puts `rlm:texture_apply:`
in the tool result, and the loop **passivates**
(`runtime.go:3507-3521`). So one turn ends at the first committed write,
and **one call per turn is already possible** if the first cell writes.

**Failure paths.** A rejected commit returns as a tool error, and the model
needs another call:

- `tools_desk.go:384-387`: `reduction.commit` returns an error;
- `texture_cell_author.go:66-78`: strict decode fails;
- `tools_texture.go`: about 70 distinct `Errorf` refusals.

### 1.2 The 14-call turn, call by call

Demo VM `vm-d2b35d35…`, 2026-10-11 20:25:16–20:27:28 (132 s wall, about
120 s of it model time). Model `deepseek-v4.1-flash`, `reasoning=high`, one
tool (`desk_go_eval`). Every call stopped with `stop=tool_use`; there was no
prose ending. All numbers are [journal]. The "likely class" column is
[inference], because the journal has no cell content.

| # | time | msgs | tok in | tok out | dur | likely class |
|---|---|---|---|---|---|---|
| 1 | 25:16 | 3 | 12,806 | 412 | 4 s | first read: `choir.Updates()` / `ReadDoc()`, forced by the pointer-only wake |
| 2 | 25:20 | 5 | 13,362 | 65 | 2 s | read or a tiny fix (input +556: small result or a short compile error) |
| 3 | 25:22 | 7 | 13,643 | 184 | 2 s | read, small result |
| 4 | 25:24 | 9 | 14,147 | 296 | 3 s | read or fix |
| 5 | 25:27 | 11 | 14,513 | 105 | 1 s | read or fix |
| 6 | 25:29 | 13 | 14,690 | 274 | 2 s | read or fix (six small cells: the K1–K11 kernel-ergonomics class or per-packet printing; see `docs/problems/desk-cell-ergonomics-from-demo-traces-2026-10-10.md:15-62`) |
| 7 | 25:32 | 15 | 15,145 | 2,311 | 16 s | first write attempt, or content built into a session variable; **not committed** (no passivation) |
| 8 | 25:48 | 17 | 19,203 | 3,434 | 21 s | second write attempt; **not committed** |
| 9 | 26:10 | **20** | 23,306 | 252 | 2 s | +3 messages: a **mid-activation wake turn was injected** (a report arrived) |
| 10 | 26:13 | 22 | 23,830 | 391 | 3 s | read or fix |
| 11 | 26:17 | **25** | 24,895 | 628 | 4 s | another injection |
| 12 | 26:22 | 27 | 25,647 | 1,523 | 11 s | partial write or fix |
| 13 | 26:34 | 29 | 27,319 | 156 | 2 s | read or fix |
| 14 | 26:37 | 31 | 29,088 | **9,175** | **51 s** | the committed write; the turn passivates |

**Breakdown.**

- 9 small calls (about 21 s) were reads, fixes or probes.
- 2 or 3 mid-to-large calls (about 48 s) were write attempts that did not
  commit [inference: no passivation followed them].
- One 51 s call was the real write.
- Input grew from 12.8k to 29k tokens, because every cell and result stays
  in context.

**The other turns** [journal]:

| Turn | Out tokens per call (last is the write) |
|---|---|
| T1 | 1,825 → 635 |
| T2 | 96 / 190 / 10,332: read, read, write. This is today's best-case shape. |
| T4 | 744 / 169 / 67 / 220 / 1,803 / 5,042 / 2,596 |
| T5 | 153 / 441 / 238 / 2,109 / 10,744 |
| T6 | 167 / 104 / 2,195 / 1,137 / 8,514 |
| T7 | 195 / 337 / 688 / 2,583 / 266 / 1,491 |
| T8 | 132 / 14,533 |
| T9 | 178 / 104 / 1,698 |

**Totals for 9 turns:** 996k input tokens and 91k output tokens. 12 calls
were large, against 9 commits. So about 3 large calls were failed or
abandoned writes [inference].

### 1.3 Sources of friction, in order of measured or likely cost

1. **The write is large and runs at `reasoning=high`.** That is 76% of model
   time [journal]. Full-replace `content` is the path the prompt recommends
   for incorporating a batch of evidence (`revision_policy.yaml:79,81`;
   `run_system.yaml:22`). The model emits the whole document each turn.
   Block edits would be smaller, but they cannot carry inline citations for
   new text (item 3).
2. **At least one read cell is forced on every wake.** The wake is a pointer
   (`coagent_update_packet.go:91-97, 161`). For reactivated runs, the
   document in context is stale (`texture_controller.go:1036`). The best
   case today is therefore read, then write: 2 calls (T2).
3. **The citation API cannot be done in one write** [code]:
   - `append_block` and `insert_block` mint a fresh UUID id
     (`tools_texture.go:1138-1160`). `insert_source_ref` needs an *existing*
     `block_id` (`:1061-1068`). So "new paragraph plus its citation" cannot
     be one batch.
   - Full replace re-mints every block id
     (`structuredTextureToolDocFromMarkdown`, `:787-791` and `:946-994`). It
     also marks every held source unused (`:794-801`), and refuses any
     markdown link or numeric marker (`:937-939`).
   - Because a commit passivates, citations for new text land in a *later
     turn* at best.
   - The prompt asks for all of it at once anyway: "Every material source
     must appear as a source_ref node in the body" (`run_system.yaml:27`).
   - Offsets are rune counts (`:1086-1093`).
   - `update_block_text` moves a block's existing refs to the end of the
     block (`:1035-1036`).
4. **Contradictions between Help and the reducer** (§2 M1–M9) produce
   rejected writes, and each costs a 2–20 s call plus the rewrite.
5. **Mid-turn injected wake turns** tell the model "Treat this packet as the
   next user turn" (`coagent_update_packet.go:318`). Injection is allowed for
   Texture (`management_controller.go:1706-1722`). But
   `cellUpdateVisible` hides any report past the scheduled seq from
   `choir.Updates()` (`tools_desk.go:520-529`). The model is told to process
   records it cannot read. Calls 9 and 11 above follow injections; that they
   caused re-reads is [inference].
6. **Kernel ergonomics** (yaegi top-level `:=`, missing imports, values not
   shown, unlocated compile errors). These are documented as K1–K11, several
   still open (`desk-cell-ergonomics…:15-62`). Each costs 1–3 cells per
   activation (doc estimate).
7. **Context weight.** About 7k tokens of system prompt plus up to 3k of
   policy tail are re-sent on every call, and the run memory grows to
   38k tokens in the input (T7) [journal]. This adds prefill time to the
   small calls and attention dilution to the large ones [inference].

---

## 2. Mismatches between Texture's prompt and its real affordances

| # | Prompt / Help says | Code does | Cites |
|---|---|---|---|
| M1 | ApplyTexture "Optional: controls" | Authored `controls` are refused. The decode hint omits `controls` and says to dispatch with Cast, Ask or Note. | `surface.go:39` vs `texture_dispatch_fold.go:34-37`, `texture_cell_author.go:77` |
| M2 | `content` is "the whole new document as markdown; always valid" | Refused if it has markdown links or `[n]` markers, if it is over 12k characters without a rationale, or if it is unchanged from a model-prior interim | `surface.go:39` vs `tools_texture.go:937-939, 787-789, 344-348` |
| M3 | Use full replace for v0→v1 and v1→v2 batch incorporation, *and* cite every material source in that apply | `content` and `edits` are exclusive. Full replace re-mints all block ids and marks all sources unused. | `revision_policy.yaml:79,81`, `run_system.yaml:22,27` vs `tools_texture.go:322-328, 787-801` |
| M4 | Place `insert_source_ref` "after the sentence or clause they support" | Possible only on blocks that already existed at the base revision. A new block's id is unknowable in the same batch. | `run_system.yaml:27`, `revision_policy.yaml:80` vs `tools_texture.go:1061-1068, 1138-1160` |
| M5 | `base_revision_id` is required on apply | On apply it defaults to the head. On **decide** it *is* required, but the Help and `run_system` decide shapes omit it. | `revision_policy.yaml:71`, `surface.go:39`, `run_system.yaml:45` vs `tools_texture.go:301-303`, `texture_turn_runtime.go:538-540` |
| M6 | "Every ApplyTexture apply body must propose work_disposition" | It defaults to `open` | `revision_policy.yaml:72` vs `texture_turn_runtime.go:441-444` |
| M7 | Pass `update_dispositions` for every evaluated packet | Unnamed covered packets are consumed as `delivered` by default | `revision_policy.yaml:73` vs `texture_turn_runtime.go:137-151` |
| M8 | decide takes `(decision_kind, reason)` | There are 6 valid kinds, none listed in Help. Only `wait*` is normalized. | `surface.go:39` vs `tools_texture.go:136-152` |
| M9 | Errors say "use a replace_all operation" or "use apply_edits" | Neither name is in the model's vocabulary, which is `content` / `edits` | `tools_texture.go:788, 1033` |
| M10 | `Ask` "resolves on the target's Reply" | On Texture an Ask folds into a research question control; nothing replies. An Ask to management is refused. | `surface.go:44` vs `texture_dispatch_fold.go:57-61, 107-110` |
| M11 | `Note` is "raw unscored transport"; the role prompt says to keep sending Notes and Asks | A Note to research opens or continues research work and **counts as a research round** against the cap of 4 | `surface.go:46`, `texture.yaml:75-78` vs `texture_dispatch_fold.go:111-117`, `texture_turn_runtime.go:161, 298-309` |
| M12 | "the runtime drops further rounds and notes it in the turn" | The drop note goes only into the committed `reason`. The cell receipt has op, doc and revision ids only, so the model never sees the drop. | `tool_profiles.go:252` vs `texture_turn_runtime.go:400-405, 422-424`, `texture_cell_author.go:158-161` |
| M13 | Research "keeps working after a report" | Research rounds are model-ended activations that only a Texture control restarts | `run_system.yaml:31` vs `docs/archive/problems/research-desk-structural-flaw-root-cause-2026-10-11.md:19-27, 160-163` |
| M14 | ReadDoc returns "revision_id and content" | It also returns the full outline with ids. The *prompt's* outline is capped (64 nodes, 4,000 bytes) and goes stale after reactivation. | `run_system.yaml:18`, `revision_policy.yaml:21` vs `session_loop.go:54-62`, `texture_agent_revision.go:828-830` |
| M15 | Inbox holds "delivered acts and ledger traffic"; use ReadDoc or Inbox "only if this diff needs more context" | Reports arrive only through `Updates()`, and the wake is ids-only, so a read cell is mandatory | `run_system.yaml:18`, `revision_policy.yaml:16` vs `choir.go:721-743`, `coagent_update_packet.go:161` |
| M16 | A mid-activation wake says "Treat this packet as the next user turn … payloads are bound as choir.Updates()" | The cell hides reports past the scheduled seq | `coagent_update_packet.go:161, 318`, `management_controller.go:1716-1721` vs `tools_desk.go:520-529` |
| M17 | The surface lists 23 functions "this desk can call" | 6 matter: ReadDoc, Updates, ApplyTexture, Cast, Ask, Note. `Message` and `Emit` are retired stubs that are still exported. Precommit, Report, ReportPacket, Resolve, Disagreement, Reply, CancelAct, Escalate, Pack, Emits, ReadFile, ListDir and Context are noise for Texture. | `choir.go:268-273`, `surface.go:40-41` |
| M18 | `Context()` gives "agent, run, desk, doc and assignment ids" | It returns `computer_id`, `activation_id` and `co_super_slot` | `surface.go:29` vs `choir.go:778-787` |
| M19 | "You never name work items" | `Updates()` exposes `work_item_id`. That is the T3-class bait: 26 refusals in one demo set. | `run_system.yaml:31` vs `intent.go:114`, `desk-cell-ergonomics…:84-104` |
| M20 | The citation shape `source_entity {target:{kind:"web_url",uri}}` | The schema accepts 24 target kinds, including `diff_hunk`, `patch`, `test_run`, `command_output` and `texture_span`. None is described. There is no `web_capture` kind or quote selector in the edit API (the app design calls this out). | `surface.go:39`, `revision_policy.yaml:80` vs `texturedoc/schema.go:376-387`, `docs/autopaper-sw-design-2026-10-09.md:187-190, 278` |
| M21 | Cell "hard rules": no `func main`, no `import "choir"` | The normalizer already handles both | `run_system.yaml:19` vs `cell_normalize.go:14-45` |
| M22 | Vocabulary not on the surface: morphisms, "Effects-OFF", appagent, Conductor, the `[CMD]` label rules, "update_coagent records", "controls" (self-dev block) | None of these exist as affordances | `run_system.yaml:14,25,44`, `revision_policy.yaml:30,60-61`, `coagent_update_packet.go:296-306`, `tool_profiles.go:249` |
| M23 | Contradictory research pressure | The role prompt says "keep sending … Ask … Note". The run overlay says "open the research desk once and name every branch". The runtime caps rounds at 4. | `texture.yaml:72-78` vs `run_system.yaml:37` vs `texture_turn_runtime.go:161` |
| M24 | Execution-request rules | Repeated 4 times (run overlay, policy, execution overlay, dispatch block) with slightly different wording | `run_system.yaml:15,41`, `revision_policy.yaml:20,29,36`, `revision_execution_findings.yaml`, `tool_profiles.go:252` |
| M25 | Developer skills (mission-gradient is deprecated per CLAUDE.md) injected into a product desk | — | `skill_context.go:12-15, 44, 48-56` |
| M26 | Reactivation | Stamps `request_intent=integrate_execution_findings` on every wake, including research reports | `texture_controller.go:1027-1028` |

---

## 3. The ideal Texture interface (1–5 cells per turn; target median 1)

**Design rule.** The turn frame *is* the context. Texture should be able to
write on its first cell because everything it needs is in the wake message,
in compact form. Variables stay as a fallback for bulk.

### 3.1 Preloaded vs fetched

**Preloaded in the turn message.** Rebuilt every turn and never carried from
an earlier turn's memory, so nothing goes stale:

- **Document.** Blocks with **short turn-local aliases**: `b1 … bN` for
  blocks, `s1 … sK` for sources. Each block shows its type, its full text,
  and inline citation markers rendered as tokens. Aliases map to durable
  block ids, which are UUIDs today (`tools_texture.go:1144`) or long
  composite ids (`:995-1001`). Long documents get a section digest plus the
  last-edited and recently-cited blocks in full; the rest go through
  `choir.Doc().Block("b17")`.
- **Records (inbox).** Every pending report, directive and owner edit in this
  turn's boundary, up to the scheduled seq, as `r1 … rM`. Each record gives:
  - the producer role (research or management; never engineering);
  - a one-line summary;
  - claims;
  - for each source: a **capture-block handle** (`r2.c1 → capture
    cap_…#blk_…`), outlet, date, and the verbatim quote or quotes the
    producer selected.

  Raw bodies stay in the cell (`choir.Record("r2").Body`) and are not
  inlined.
- **Owner request** and **standing state**: open asks to research and
  management, and verification status of existing citations. These are
  short.
- No policy essays, no ids the model must copy longer than about 6
  characters, and no `work_item_id`.

**Fetched only when needed:** full capture text (`choir.Capture(handle)`),
older revisions, and long-document blocks outside the digest.

### 3.2 The edit API: one verb, block-addressed, citations inline

```go
choir.Write(`
@replace b4
Anthropic released Claude Haiku 5.5 on Oct 9 {{cite r2.c1 "available today in the API at $0.80 per million input tokens"}}.

@insert after b4
OpenAI's GPT-6 rollout reached ChatGPT Plus users, TechCrunch reported {{cite r3.c2 "rolling out to Plus subscribers"}}.

@delete b9
`, "incorporate r2, r3")
choir.Ask("research", "Policy actions on AI chips Oct 4–11: who, what, primary record?")
```

- **Ops.** `@replace bN`, `@insert after bN` / `@insert top`, `@append`,
  `@delete bN`, `@move bN after bM`. Also `@section "Heading"` (replace a whole
  section by heading text) and `@all` (full rewrite).
- **Text is markdown.** Lists, quotes and headings are split into blocks by
  the runtime. Today `update_block_text` refuses multi-paragraph text
  (`tools_texture.go:1032-1034`); the new API accepts it.
- **`@all` reconciles instead of replacing.** It diffs paragraphs against
  existing blocks by normalized text hash, keeps ids and citations for
  unchanged or lightly edited blocks, and mints ids only for new text. This
  removes the M3 citation loss, because a full rewrite stops being
  destructive.
- **Citation token.** `{{cite <handle> "verbatim quote"}}` cites a capture
  block. `{{cite <handle>}}` with no quote is allowed but renders as
  "unquoted". `{{embed <handle>}}` on its own line is a block **transclusion**
  of the source block. Handles:
  - record-local, such as `r2.c1`;
  - an existing source alias, such as `s3`;
  - artifact refs that management supplied, such as `r5.a1` for a diff,
    test run or build. The reader sees a chip like "the change [3]", and the
    diff is behind the click. **Texture never writes code or diffs inline;
    the token is the only way they appear.**

  These replace `insert_source_ref`, offsets, `display_mode`,
  `mark_source_unused` and the markdown-link ban
  (`tools_texture.go:937-939`).
- **Validation is synchronous and located.** For example: "line 3: b14 is
  not in this document (blocks: b1–b12)", or "r2.c4: record r2 has 3
  sources". A refused write commits nothing and the session keeps the text in
  a variable, so the fix is a 2-line cell.
- **Dispositions are implicit.** Every record in the frame is consumed at
  commit. `choir.Reject("r4", "duplicate of r2")` is the only explicit form.
  Pending past-boundary records are *not shown*, and the runtime never
  injects them mid-turn (they wait for the next turn).

### 3.3 Many records per turn

- **Wakes are debounced.** The runtime batches records that arrive during a
  short window (for example 10–20 s, or until the research job's
  "round-ready" mark) into one turn. The research root-cause doc already
  recommends this (`research-desk-structural-flaw-root-cause-2026-10-11.md:415-449`).
- **One turn integrates the whole batch in one `Write`.** Status-only
  reports do not wake Texture. Only records with claims, sources or
  resolutions do (`research-desk-structural-flaw…:449`).
- **Management is the only technical input.** Engineering results reach
  Texture only as management records that are already translated: a plain
  summary plus artifact handles. Jev is a management routing step and never
  a holder (`docs/archive/protocol-review-assign-and-engineering-routing-2026-10-11.md:15-60`).
  The management prompt has to gain that translation contract. Today it says
  only "Report the result to Texture with choir.Report, carrying its
  evidence refs" (`promptstore/defaults/management.yaml:17-21`).

### 3.4 Signalling done

- **`choir.Write` commits and ends the turn.** This is today's passivation
  semantics (`runtime.go:3514-3518`), unchanged.
- **`choir.Wait(reason)`** ends the turn without a revision. It replaces
  `decide` and its 6 kinds.
- **`choir.Done()`** in the same cell marks the request satisfied. It
  replaces `work_disposition`.
- **No-op ending.** A turn that ends with prose and no commit stays a no-op,
  as today.

### 3.5 Verification and versions

Verification is asynchronous, incremental and memoized.

- **Captures.** Captures are stored in the **same block model** as Texture
  documents (`choir.texture_doc.v1`). A capture is a document whose blocks
  are content-addressed (`blk_<sha256(normalized text)[:12]>`), with a
  capture-level content hash.
- **Citations.** A citation is a `source_ref` that targets
  `{kind:"web_capture", capture_id, block_id, content_hash}` with a
  `text_quote` selector.
- **Fast first version.** `Write` commits version N right away. Each new
  citation carries `verification: pending`.
- **Verifier.** A host verifier, not a model, runs a quote-against-capture
  cascade like `~/mike/backend/src/modules/chat/engine/verifyCitations.ts`:
  - match tiers: exact, then whitespace and case, then punctuation-tolerant
    (`locateQuote`, lines 33-57);
  - ellipsis and segment chains in order (`locateSegmentsInOrder`, lines
    98-130);
  - snap the quote to the exact source excerpt when `needs_correction`
    (lines 345-351).
- **Memoization.** The key is (citing block content hash, capture block
  content hash, normalized quote). Unchanged citing blocks with unchanged
  sources are never re-verified.
- **Result versions.** Results that change what the reader sees produce
  version N+1, authored by the verifier role. These are non-semantic:
  status marks and snapped quotes only, not prose. Texture's canonical
  sole-writer rule needs an explicit, narrow exception or a Texture-owned
  "apply verification" fold. **This is red and needs owner ratification.**
- **Escalation.** Failures do not block the version. An unverified citation
  shows as unverified, and Texture's next turn frame lists it ("s4: quote not
  found in capture") so the model can fix or drop it.
- **Permalinks.** Each published version gets its own permalink. Note: the
  app design currently says "every later revision appears at the same
  permalink" (`autopaper-app-design…:59-65`). The two shapes must be
  reconciled: one route with per-version permalinks under it.

---

## 4. The prompt core, and what to delete

### 4.1 Core (target ≤ 3.5k characters, all of it true today or after Stage 4)

> You are Texture, the only writer of this document. The owner reads the document, not this chat.
>
> Each turn shows you:
> - the document as blocks b1…bN;
> - the new records r1…rM from research and management, with their sources r2.c1… and quotes;
> - the owner's request.
>
> Write the next version in one cell with choir.Write. Then the turn ends.
>
> Writing:
> - The document states what is known, what the evidence says, and what is still open, in present tense. Never narrate the process.
> - State claims in the source's own terms, and cite each claim inline: {{cite r2.c1 "exact words from the source"}}. Copy the quote verbatim from the record. It is checked against the capture.
> - A claim from a primary record, or from two independent outlets, is stated plainly. A single-source lead appears once, attributed.
> - No code, diffs, commands or logs. To refer to built work, cite the artifact management gave you: {{cite r5.a1}}.
> - Do not state current facts from your own knowledge. If the records do not cover something the request needs, say it is open and ask research.
>
> Verbs (the only ones you have):
> - choir.Write(edits, why): @replace/@insert/@delete/@move/@section/@all blocks; commits and ends the turn.
> - choir.Wait(why): nothing to change now; ends the turn.
> - choir.Done(): the request is answered. Use it in the same cell as a Write.
> - choir.Ask("research", question): research keeps working on this document; ask for what is missing (coverage first). Delivered with your Write or Wait.
> - choir.Cast("management", objective, plan): request building or running something. Management reports back in plain language.
> - choir.Reject("r4", why): a record you deliberately do not use.
> - choir.Record("r2"), choir.Capture("r2.c1"), choir.Doc().Block("b17"): read more, only when the frame is not enough.
>
> If a Write is refused, the error names the line. Fix it in a small cell; your text is kept in `last`.
>
> First turn on a new request: write a short version (≤150 words: what the owner wants, the plan, what is open) and Ask or Cast the work in the same cell.

### 4.2 Delete or move out of the 28k

| Remove | Where | Why |
|---|---|---|
| Theory essay (except 2 sentences) | `texture.yaml:10-79` (4.4k) | It is orientation, not affordance. It contradicts the round cap (M23). |
| Morphisms, Effects-OFF, protected-effects lists | `run_system.yaml:13-14`, `revision_policy.yaml:18-20` | Not on the surface (M22); the runtime enforces them. |
| Execution-request rules (×4) | `run_system.yaml:15-17,41`, `revision_policy.yaml:20,29,36,60`, `revision_execution_findings.yaml`, `tool_profiles.go:252` | Collapse to one `Cast` line (M24). |
| JSON packet field contract, kind enum | `run_system.yaml:16-17` | Belongs in `Help("Cast")` and located errors. |
| Cell discipline rules | `run_system.yaml:19` | The normalizer handles them (M21). |
| `[CMD]` label rules, hard-requirement regex hints | `run_system.yaml:44`, `revision_policy.yaml:30,60-61`, `texture_agent_revision.go:622-662, 798-820` | Acceptance-suite era; not product behavior [inference]. |
| Interim/V1 rules stated 4 ways | `run_system.yaml:9,24,29,31`, `revision_policy.yaml:28,45,49,51` | One line in the core. |
| Source-entity / insert_source_ref / mark_source_unused / offset rules | `run_system.yaml:22,27`, `revision_policy.yaml:74-82`, `revision_source_entities_intro.yaml` | Replaced by the cite token (§3.2). |
| Email op | `revision_policy.yaml:31,63`, `run_system.yaml:23` | Separate surface when the request is an email [inference: rare path]. |
| Compaction and "intermediate appagent revisions" notes | `run_system.yaml:34-35`, `revision_policy.yaml:64-66` | Runtime internals. |
| Whole `revision_policy` tail in the user turn | `texture_agent_revision.go:812-821` | Replaced by the frame. |
| Skill context for Texture | `skill_context.go:48-56` | Developer skills, not product. |
| 17 surface functions | `choir.go:268-273` (texture module set) | Keep Write, Wait, Done, Ask, Cast, Reject, Record, Capture, Doc, Help. |
| Self-dev "Do not send controls" block | `tool_profiles.go:248-250` | Rewrite in core vocabulary. |

Expected size: system prompt about 4–5k characters plus a frame that scales
with the document. Today it is about 28k characters plus a 9.5–13k policy
tail plus memory.

---

## 5. Runtime around Texture: delete, or move into the model

**Delete outright.** These have no remaining caller or are superseded [code]:

- **The `target_work_item_id` control mode.** The fold never emits it and
  authored controls are refused (`texture_dispatch_fold.go:34-37, 97-131`).
  Remove the branch at `texture_turn_runtime.go:373-384` and the mode at
  `tools_texture.go:46-55, 73-96`.
- **`validateTextureControls`**: tri-mode exclusivity that exists only for
  model-authored controls (`tools_texture.go:73-96`).
- **`Message` and `Emit` on the Texture module set** (`choir.go:268-273`).
- **The six-way `decision_kind` enum and its normalizer**
  (`tools_texture.go:122-152`). Collapse to Wait/Block.
- **The markdown-link ban, offset-0 rule, same-offset dedupe, and the
  multi-paragraph rejection** (`tools_texture.go:937-939, 1003-1016,
  1032-1034, 1091-1093`). The cite-token parser and the reconciler replace
  them.
- **Hard-requirement regex hints and the `[CMD]` machinery**
  (`texture_agent_revision.go:622-662, 798-820`) [inference: test-era].
- **Mid-activation wake injection for Texture.** Gate it off in
  `coagentUpdateDeliverableForRun` (`management_controller.go:1706-1722`).
  The scheduled-seq boundary then becomes the single rule. This also closes
  M16 and standing question 4 (single state authority).

**Move out of Texture into the runtime-owned research job** (station
fbd2f6cc, drafted):

- `textureResearchRoundsPerOwnerRequest = 4` and
  `researchRoundsSinceOwnerInput` (`texture_turn_runtime.go:155-203, 295-309`);
- the silent drop (`:400-405`);
- `mergeResearchOpeners` (`:205-235`);
- the `open_researcher` opener and continue logic (`:264-279, 347-372`).

Texture's `Ask` becomes a steering note to a standing job. The *budget*
lives with the job, so Texture cannot spend it with Notes (M11) and cannot
be surprised by a silent drop (M12).

**Move into the model.** Judgment only, made cheap:

- whether a record is worth incorporating (default consume, explicit
  `Reject`);
- when the request is done (`Done()` instead of `work_disposition`);
- what to ask research.

**Keep in the runtime (red substrate that works):**

- the atomic `ApplyTextureTurn` (head-pinned CAS, idempotent cell identity,
  `texture_cell_author.go:59-65`, `texture_turn_runtime.go:407-488`);
- the sole-writer rule;
- passivation on commit;
- P2a fold, so a dispatch commits with the turn;
- scheduled-seq consumption.

**Add to the runtime:**

- the reconciler;
- the alias map;
- the cite-token parser with capture-block resolution;
- the async memoized verifier;
- per-version permalinks;
- debounced wake batching.

---

## 6. Staged plan, mutation classes, and staging measurement

**The measure, the same at every stage.** Run on staging (`choir.news`). Use
one fresh test computer per run at the deployed SHA, a fixed script (the
PD1 AI-news prompt plus follow-up), and 3 runs per stage.

| Metric | Source |
|---|---|
| Calls per turn: median, p90, max | Gateway journal, per VM: `system_chars` identifies Texture; a turn is the calls between commits. **Add a one-line "texture turn committed" gateway-visible marker, or export the guest run-events trace**, because commit boundaries are not in the gateway log today. |
| Failed cells per turn, refusal classes, block-id-not-found count, probe count | Guest run-events trace. Promote the lost `trace_snapshot.sh` to a tracked script (yellow). |
| Model seconds per turn and output tokens per turn; share of the large call | Journal |
| Report-arrival → incorporating-revision latency, p50 and p90 | Lifecycle events |
| Time to first revision | Lifecycle events |
| Citations per 1k words; verified-quote fraction; unverified-at-publish count | Revision source entities plus verifier results |

**Targets:**

- median calls per turn ≤ 1.5, p90 ≤ 4;
- failed cells per turn ≤ 0.3;
- report → revision p50 ≤ 45 s;
- verified-quote fraction ≥ 90% (app-design R1).

**Stages:**

| Stage | Class | Content | Rollback |
|---|---|---|---|
| 0 | green | Problem doc first, per the contract. Correct the "sixty-six cells" phrasing in the report and record that per-turn is about 5. File §2 as a problem doc. Define the metrics. | git revert |
| 1 | yellow | Tracked trace-export script and a turn-commit marker. **Baseline: 3 runs** at the current SHA. | revert |
| 2 | green/yellow + orange | Prompt and Help alignment with no semantic change. Fix M1, M5–M10, M14, M15, M18, M22–M25 text. Delete the duplicated policy, the skill context and the theory essay. Shrink the Texture module set to 6 verbs (orange: surface change). **Measure.** Expected: fewer refusals, a small latency gain. | revert; the module set is a one-line list |
| 3 | orange (delivery; borders red because it changes the consumption boundary) | Preloaded turn frame (doc blocks with aliases, records with sources) rebuilt every turn instead of the pointer-only wake. Stop mid-turn injection for Texture. Debounce wakes. **Measure.** Expected: median calls 2 → 1–2, and no forced read cell. | feature flag on frame building; injection gate |
| 4 | **red** (Texture canonical writes) | `choir.Write` with block aliases, located synchronous validation, reconciling `@all`, and the cite token resolving to existing `web_url` sources first. `Wait`, `Done` and `Reject` replace decide, work_disposition and dispositions. Full ceremony: conjecture delta, protected surfaces (ApplyTextureTurn, source graph), admissible evidence (staging demo traces), rollback (keep `ApplyTexture` as the legacy verb behind the flag for one release), heresy delta. **Measure.** Expected: smaller write output (block edits), citations in the same turn. | flag; legacy verb retained |
| 5 | **red** (canonical writes, Store B captures, publication) | Captures as content-addressed block documents; `web_capture` target plus `text_quote`; the transclusion node; the async memoized verifier producing version N+1 (needs the owner ruling on the sole-writer exception); per-version permalinks. Overlaps Autopaper station C (`autopaper-app-design…:570`). | additive schema; verifier off-switch |
| 6 | **red** | Delete superseded runtime: the round cap, opener merge, control modes, the decide enum, the hard-requirement hints. Move the budget to the research job (with the fbd2f6cc station). | git revert of deletions |
| 7 | red (routing) | Management translation contract; engineering addresses only management (retire the document-cast path, per the routing review). | revert |

**Reasoning level.** Measure `reasoning=high` against medium for the
Texture write call inside Stage 3 or 4, holding the model fixed. This is a
configuration lever, not a model bake-off. The journal suggests the write
call's output is mostly reasoning [inference: 9–14.5k tokens out for a
document of about 2.5k tokens].

---

## 7. Open items and residual risks

- The per-call classification in §1.2 needs one guest cell trace to confirm.
  The next demo should capture the run-events trace for at least one
  multi-call turn.
- The verifier-authored version N+1 conflicts with "Texture is the sole
  writer of canonical revisions". The owner should rule on a narrow,
  non-semantic exception, or on a Texture-folded verification step.
- The permalink shape: per-version permalinks (owner direction, 2026-10-12)
  vs "same permalink" (autopaper-app-design-2026-10-11.md:59-65). The likely answer is one route with
  versioned permalinks under it.
- Whether the guest has `RUNTIME_SKILLS_ROOT` populated decides whether about
  1–2.6k characters of developer skills are in Texture's prompt today
  [inference].
