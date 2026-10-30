# A Prompt-Bar Research Answer Is Slow, Lists Instead Of Infers, Ignores The Computer's Own Corpus, And Renders Mangled

Date: 2026-10-14, 04:20 UTC. Reported by the owner from a regular research
prompt typed in the staging prompt bar at about 03:57 UTC: document "What's
current in ai", four versions, v4 shown on a phone. Status: documented.
The renderer fixes and the prompt rule follow in separate commits on this
branch (not pushed). The larger causes are described as fix designs, not
built.

Privacy: the owner's document text, prompts and sources were not read. All
content evidence below comes from a reproduction on a minted test computer
(`computer-1812a2d652da40ece28d54a6dbb93d9c`, build 66fe0917, label
`prompt-bar-ai-1014a`, stopped after use), where the same prompt
("What's current in ai") was typed in the prompt bar at 04:06:25 UTC.

## The owner's report

1. Each version was very slow.
2. The answer is fairly comprehensive but lists items instead of drawing
   clear inferences.
3. It appears to retrieve only from the web, never from the Autopaper
   object graph (the computer's own captures, stories and published objects).
4. The formatting is mangled (phone screenshots of v4):
   - paragraphs begin with literal `**OpenAI — GPT-6 "Astra".**`;
   - numbered chips appear where quoted words should be: "Artificial
     Analysis reports that ⑦ and that ⑦ It also reports per-token pricing:
     ⑦", "while itself warning that ②. Its note that ② makes clear…";
   - a stray full-width title line ("Execution of Follow-on Investment
     (Third Tranche) in OpenAI | SoftBank Group Corp.") follows a
     paragraph, and a lone "[5]" paragraph follows that line;
   - headings and bullet lists render correctly.

## Reproduction (test computer, same prompt)

Runs (`GET /api/runs`, `/api/runs/<id>/events`):

| step | run | model (pool member) | calls | wall time |
|---|---|---|---|---|
| v1: the owner's prompt | conductor | none | none | 04:06:27 |
| v2: acknowledgment and research Cast | texture `8bb0ee8f` | cat-jam-free | 5 (4 spent on `choir.Help` / `ListDir` exploration) | 16 s |
| research round | research `7cfed409` (agent `research:<doc>`) | cat-jam-free, effort low | 21; input grew 4.7k → 119k tokens | 1 min 38 s |
| v3: synthesis | texture `db954179` | cat-jam-free | 3 (one call 32 s, 11k output) | 50 s |
| support judgement of v3's citations | management `fa07a659` | **longcat-2.5-preview-free**, effort high | call 2 alone 5 min 12 s (18k in, 15.8k out) | 5 min 12 s+ |
| v4: "repair unsupported citations" | texture `7013ce03` | cat-jam-free | | 42 s |

v4 landed at 04:15:08, 8 min 43 s after the prompt. Most of that time was
management's one longcat call. Research made 0 calls to `SearchWireCorpus`.
Its verbs were `SearchStart` ×4, `SearchWait` ×4, `Searches` ×6, `Read` ×7,
`Records` and `ReportPacket`.

v3 and v4 reproduce symptoms 2 and 4b exactly. Projected text and Write
scripts from the test computer:

- "Anthropic says running it costs [1] than Haiku 4.5". The Write script
  was `costs {{cite r1.c3 "...On average, it now costs around 75% less to run.²"}} than Haiku 4.5`.
- "Google: Gemini 4 Argon, globally available, [3] at an introductory
  price of [3]" (v3). In v4 this became "The same page states its
  introductory price as [3]".
- "describes four categories of unintended Claude behavior, including [5]"
  (v3). In v4: "one item being [5]".
- "EU AI Act transparency duties: [8] Those duties cover…"
- Every section is a bullet list per company or agency. There is no
  paragraph that draws an inference. Process words leak into the reader's
  text: "research reports that", "was not captured", "Reported but not yet
  verified", "recorded here as unverified context".

v3 and v4 had no bold, so symptom 4a did not reproduce. Neither did the
`{{embed}}` (4c): every one of v4's 16 citations is `numbered_ref`.

## Causes, per symptom

### 4b. Chips where quoted words should be: a writing-contract mismatch, not a renderer bug

- A `{{cite h "quote"}}` token becomes a `source_ref` node with
  `display_mode: numbered_ref`, and the quote is stored as evidence in
  `attrs.ref.quote` (`internal/texturewrite/apply.go:568-627`).
- By design, no renderer shows that quote in the text. The published reader
  draws a number with the quote in a popover
  (`frontend/src/lib/texture-published-citations.ts`,
  `renderPublishedCitationRef`). The unpublished reader draws a number too
  (`frontend/src/lib/texture-structured-editor-doc.ts:212` at HEAD).
- Texture's prompt said "Cite inline, after the words a source supports …
  quote one passage of it word for word"
  (`internal/textureprompts/texture.yaml:18` at HEAD, from f93fdcb9). It
  never said that the quote is invisible. The model reads `{{cite}}` as a
  quotation that appears in the text, and writes "costs {{cite …}} than".
- The support judgement makes this worse: management's verdicts
  (64278ca6, b04863f2) send Texture "unsupported" pairs to repair.
  Texture's v4 why says it repaired "by making each cited sentence directly
  track its quoted passage", and it pushed more sentences into the
  "including [5]" shape.

A second gap, in the renderer: in an unpublished document the popover
cannot show the quote either. `renderInlineSourceRef` fills the popover from
the entity's excerpt (`texture-source-renderer.ts:344-362` at HEAD). A
Write entity carries only a `whole_resource` selector
(`internal/texturewrite/apply.go:675-696`), so the popover body was empty.

### 4c. A stray title line and a lone "[5]": a renderer bug

- `{{embed h}}` becomes a `source_ref` with `display_mode: expanded_ref`.
- In an unpublished document, `renderInlineSourceRef`'s expanded branch
  (`texture-source-renderer.ts:372-381` at HEAD) draws `<span><strong>title</strong>`
  and then `renderSourceTransclusionBody(entity)`, which is a `<div>`
  (`:355`). That `<div>` sits inside the paragraph's `<p>`.
- The browser's HTML parser closes the `<p>` at the `<div>`. The title is
  left at the end of the paragraph as its own line. The body is empty,
  because Write entities carry no excerpt. The `[n]` marker spills out as a
  bare text line, and the stray `</p>` adds an empty paragraph.
- That is exactly "a full-width link line, then a lone [5]". The published
  path already splits a paragraph around an embed and draws it as a
  `<figure>` (`renderParagraph` / `renderPublishedEmbedBlock`, 1764afe8).
  The unpublished path never did.
- The existing test asserted the broken shape by string match only
  (`frontend/tests/texture-structured-editor-doc.spec.js`, "expanded_ref
  … as a block"). A string match cannot see the parser split.

### 4a. Literal `**`: the renderer cannot heal it; why the owner's document had it is a hypothesis

- The structured renderer escapes text nodes and draws only marks
  (`texture-structured-editor-doc.ts:191` at HEAD). It never parses
  markdown inside a node.
- Write, since d6f82142 (2026-10-13 16:46 UTC, an ancestor of 66fe0917),
  turns `**x**` into strong marks. I ran the owner's exact strings through
  `texturewrite.Apply`: `**OpenAI — GPT-6 "Astra".** The most…` and
  `**EU AI Act enforcement begins.** The … states that {{cite …}} On the
  same date, {{cite …}}`. Both become strong marks. A current Write does not
  emit literal `**`.

A text node holding literal `**` therefore came from a path that does not
parse inline marks. The candidates:

- **H1 (likely):** the owner's computer ran a build older than d6f82142
  when it wrote v1-v4. Deploys never restart a busy computer, so a
  computer busy across the deploy keeps its old build. The paper computer
  `computer-1355b83b…` reported 66fe0917 at 03:50
  (`docs/archive/problems/management-serial-turn-blocks-paper-2026-10-14.md`).
  Whether the owner's prompt-bar document lives on that computer was not
  observed. To check, without reading content: `GET /api/runtime/observability`
  `build.commit` in the owner's session, compared with the v4 revision's
  `created_at`.
- **H2 (a real mechanism, not this document's cause unless H1 holds):** a
  block materialized without marks keeps its literal `**` forever.
  `reconcile` keeps an old node exactly when the new text equals the old
  block's frame text (`internal/texturewrite/apply.go:290-330`). The frame
  renders an unmarked node's text verbatim (`blocks.go:100`), so Texture
  copies `**Status.**` back and the old unmarked node survives. The same
  holds for `PlainDoc` bases: an owner draft read as markdown, whose text
  nodes get no marks (`base.go:79-85`). Five of thirty published stories
  showed this on 2026-10-13
  (`docs/archive/problems/autopaper-story-prose-markdown-and-meta-2026-10-13.md`).
- **H3 (edge case):** a closer followed directly by a letter
  (`**A.**The`) fails Write's flanking rule (`inline.go:117-118`) and stays
  literal. The owner's screenshot shows a space after the closer, so this
  is unlikely here.

### 2. Lists instead of inferences: the contract asks for attribution, not synthesis

- Texture's revision-shape rules are the news-story contract of
  2026-10-10/13: confirmed versus unconfirmed claims, "state each claim in
  its source's own terms", and "say what we know, what the evidence says,
  cited, and what is still open" (`texture.yaml:28-31` at HEAD).
- The "synthesis contract" of f353b602 (L2a A4) is ungated, so the
  prompt-bar path does use it. But it is about agreement between sources
  ("where sources agree, state it as established"), not about inference.
  Nothing asks for what the evidence adds up to.
- Research's report is a flat list. `SourcePacket` has `Summary`, a list of
  `Claims` (text plus source ids) and `Sources`
  (`internal/types/evidence.go:60-81`). It has no field for synthesis or
  implications. The test computer's research packet was one claim per
  product or agency.
- Texture then sorts those claims into categories, as v3/v4's
  "## Model and product releases" bullets show.
- The quote-support judgement (above) adds pressure toward one-fact,
  one-cite sentences, which keep the text list-shaped.

### 3. Web only: the corpus verb is reachable but narrow, and nothing says to use it

- Research cells do have `choir.SearchWireCorpus`
  (`internal/yaegikernel/choir.go:119,127,301,307,631`; fixed in L2a
  488bba4c). It is reachable on the prompt-bar path: the Cast opens a
  `research:<doc>` run with the research module set.
- The research prompt only lists the verb: "World access (host-mediated):
  choir.Read, choir.ReadCapture, choir.SearchWireCorpus …"
  (`internal/runtimeprompts/overlays/rlm_research_runtime.yaml:19`). Its
  evidence doctrine is web-only: "Start several searches … with
  choir.SearchStart" (`:26`; `internal/promptstore/defaults/research.yaml:16,51`).
  Its summary in the generated verb list is "Searches the owner's published
  corpus." (`internal/yaegikernel/surface.go:81`).
- The verb is narrow. It is a substring search over this computer's
  Texture documents (stories) in the embedded store
  (`internal/store/texture.go`, `SearchDocuments`, via 488bba4c). It does
  not search captures, which live in the shared Autopaper store, or the
  published-object graph.
- No verb lists or searches existing captures. `ReadCapture` needs an id
  already in hand. `choir.Stories` is management-only
  (`internal/agentcore/intake_view.go:72-73`).
- The cell payload carries no owner. Owner scope comes from `CHOIR_OWNER_ID`,
  falling back to `"universal-wire-platform"`
  (`internal/researchtools/researchtools.go:99-104`). This is fine in VMs
  but fragile elsewhere.
- In the reproduction, research made 0 corpus calls.

### 1. Slow: one shared serial management turn on a slow free model, plus input growth

From the reproduction:

- Texture and research ran on cat-jam-free in 2-30 s per call.
- The v3 → v4 step waited 5 min 12 s for one management call on
  `longcat-2.5-preview-free` at effort high. That call was the support
  judgement of v3's citations (`choir.Judge`), and v4 exists only to repair
  what that judgement flagged.
- The model pool hashes each thread onto one of deepseek-v4.1-flash,
  cat-jam-free, longcat-2.5-preview-free and gpt-6-luna
  (`internal/modelcatalog/catalog.go:207-216`). Any desk can land on longcat.
- Research re-sent its growing cell output each step, from 4.7k to 119k
  input tokens over 21 calls. On a slow member that means minutes per call
  (LR lever 6, branch `lr-lever6`, not deployed).

On the owner's computer (hypothesis; run timings were not read):

- Management is one serial turn per owner, `management:<owner>`. On the
  paper computer it was held by one longcat run from 03:13 to past 03:50,
  with 2-9 min per call (`docs/archive/problems/management-serial-turn-blocks-paper-2026-10-14.md`).
- If the owner's document is on that computer, every management-dependent
  step of the prompt-bar answer waits behind the paper. That includes the
  support judgement that triggers v4.
- To confirm without reading content: `GET /api/runs` on the owner's
  computer gives the document's texture, research and management runs, and
  for each one its `llm_model`, call count, and per-call
  `provider_call_started` → `tool_loop` gaps.

Minor: Texture's v2 spent 4 of 5 calls exploring `choir.Help` and
`ListDir` before writing. This was 9 s on a fast model, but minutes on longcat.

## Fixes

Built on branch `worktree-agent-aae666358d8700c9a` (not pushed or deployed):

1. **Renderer (orange; frontend only; rollback is a revert).**
   - An unpublished `expanded_ref` that is not inline media now splits its
     paragraph and draws a `<figure data-embed-block data-texture-local-embed>`.
     The figure shows the citation's quote (else the entity excerpt), `[n]`
     and the linked title, and it serializes back to the same
     `expanded_ref` citation.
   - A numbered cite's popover shows the citation's own `ref.quote`.
   - An unmarked text node's paired `**x**`, under Write's flanking rule,
     renders as strong. This heals legacy nodes (H2) on display only; stored
     text is unchanged.
2. **Texture prompt (yellow; prompt text; rollback is a revert).**
   - A cite shows as a number, never its quote, so the sentence must read
     whole without it: "costs 75% less {{cite …}}", not "costs {{cite …}}".
   - Lead with what the evidence adds up to (the trend, the cause, what it
     means), then the cited facts. A list of findings is not an answer.
   - Fits the 4000-character core budget.

Designs, not built:

- **Quotes in the text** (red: Texture canonical write). Add a
  `{{quote h "words"}}` token that renders the words in quotation marks
  followed by the number. Or, in Write, refuse a `{{cite}}` whose sentence
  does not read whole without it, for example one directly after
  "that"/"of"/"including"/":" and followed by lowercase text. The prompt
  rule comes first. Measure whether the shape recurs before adding a
  refusal, since each refusal costs a model round trip.
- **Heal legacy unmarked emphasis at the source** (red). When `Base` reads
  a block, re-parse unmarked text nodes' paired `**`/`*` into marks, so
  `reconcile`'s keep path stops preserving literal asterisks. The same
  applies to `PlainDoc` text (`base.go:79`). The published HTML export
  (`internal/platform/publication_document.go`) would heal too, which the
  frontend fallback does not cover.
- **Synthesis in the research report** (orange). Add `Synthesis` (or
  `Implications`) to `SourcePacket`. Ask research for the two or three
  claims the evidence supports jointly, each with its source ids. Texture's
  frame shows it first.
- **The corpus first, and wider** (orange). Tell research to call the
  corpus before the web for any question the paper may have covered.
  Extend the corpus search to captures and the published-object graph,
  scoped by the run's owner rather than an environment variable.
  - Built on branch `worktree-agent-ac23ae99f25ec3949` (not pushed or
    deployed), orange. `choir.SearchWireCorpus` keeps its contract
    (`results` are stories) and adds `captures`: a keyword search over
    this computer's newest 1000 captures in the shared store (title,
    outlet as URL host and source id, language) plus Jev's latest label
    per capture (outlet, script, the story headline it ties the capture
    to). A query word must start a word (anywhere in Han, kana, Thai,
    Arabic and Hebrew script); one- and two-word queries need every word,
    longer ones at least half. Results rank by words matched, then field
    weight (title over topic and outlet over language), then newest.
    Each list is capped at 25 (default 10). Each hit's `capture_id` is a
    `{{cite}}` handle. Owner and computer come from the run's execution
    context; `CHOIR_OWNER_ID` is now only the last fallback. A capture-store
    outage keeps the stories and says so in `captures_note`. The research
    prompt (v9) and runtime overlay say to search the corpus first, then
    use the web for what it lacks or to check the primary record.
  - Not built: searching block text (each capture's row has no body, and
    reading bodies costs one corpusd read per capture), captures older
    than the newest 1000 (needs a corpusd-side search route), the
    published-object graph beyond this computer's stories, and the
    `WireCorpus` → Autopaper code rename.
- **Slowness** (orange/red; model policy):
  - Do not let the prompt-bar answer depend on management's serial turn.
    Judge support in a sub-RLM or through Jev directly (owner direction
    recorded in the management-serial doc).
  - Keep high-input desks off members measured at minutes per call.
  - Land LR lever 6, so cells stop re-sending earlier output.

## Heresy delta

- discovered: the cite-as-quotation writing shape; the empty unpublished
  popover; the unpublished embed split by the HTML parser; reconcile
  preserving legacy literal emphasis; a web-only research doctrine with a
  stories-only corpus verb; the prompt-bar answer's dependence on
  management's serial turn.
- introduced: none.
- repaired (on the branch): the unpublished embed split; the empty
  popover; legacy `**` on display; the prompt rule for cites and inference.
