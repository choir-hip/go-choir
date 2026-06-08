# Natural Compaction Document Recall Eval Mission v0

## Mission Identity

Run a realistic, search-quota-free compaction recall matrix across Choir's
current first-class model providers by using long public documents as a frozen
corpus. The compaction eval must not start until the user/candidate computer
sandbox setup is proven through the product path: agents need real document
tools, real ContentItems, real selectors, and real extracted text before any
memory result is meaningful.

This mission is not a search eval, not a slides-app mission, and not a general
document-app build. The real artifact is:

```text
public document URLs / uploaded document files
  -> sandbox-available document extraction tools
  -> durable ContentItems with raw hashes, cleaned text, selectors, and caveats
  -> researcher/VText access through normal tools
  -> automatic LLM compaction under realistic long-context pressure
  -> approximate and exact recall scored across model-policy arms
```

The target model matrix is:

- `deepseek-v4-flash`;
- `deepseek-v4-pro`;
- `mimo-v2.5`;
- `mimo-v2.5-pro`;
- `gpt-5.4-mini`.

Do not spend live search API quota during the scored phase. Source discovery can
be done once, outside the model matrix, by direct public URLs or owner-provided
documents.

## Suggested Goal String

```text
/goal Run docs/mission-natural-compaction-pdf-recall-eval-v0.md as MissionGradient; first upgrade sandbox document extraction for PDF/DOCX/EPUB/PPTX/HTML sources, then run a frozen-corpus natural compaction recall matrix across DeepSeek, Xiaomi, and gpt-5.4-mini without live search, proving approximate recall, exact retrieval, and automatic post-compaction continuation through normal Choir researcher/VText runs.
```

## Execution Gate 0: Sandbox Setup Before Compaction

This mission has a hard preamble. Before any model arm is run, the worker must
prove that normal Choir user/candidate computers can ingest and expose real
documents. The compaction matrix is downstream of this proof; it is not allowed
to treat PDF/DOCX/EPUB/PPTX/HTML support as a side quest or local-only
precondition.

The preamble is satisfied only when all of these are true:

- the normal sandbox image declares document tools in Nix, not just on the
  developer's Mac;
- document import uses the shared ContentItem extraction substrate for URL and
  file imports;
- PDF extraction is no longer regex-only;
- DOCX, EPUB, PPTX, and HTML have explicit adapters or documented fallbacks;
- imported documents preserve raw hashes, cleaned text, selectors, media type,
  adapter metadata, warnings/caveats, and provenance;
- researcher tools can import a document and read exact selectors without
  loading the whole artifact into context;
- VText import reuses the same extraction substrate instead of maintaining
  separate format hacks;
- at least one deployed product-path import proof confirms the behavior on
  staging or an authorized Node B user/candidate computer path.

Only after this gate is passed should the run move to frozen-corpus import,
model-policy control, automatic compaction, and recall scoring.

## Why This Mission Exists

The previous compaction mission proved the harness can create LLM checkpoints
and retrieve exact run-memory entries when prompted. That was necessary but not
sufficient. The owner now wants a more natural eval: researchers should read
large public documents, compact automatically, then answer approximate and exact
recall questions without artificial prompt engineering that tells them which
memory tool to call.

That immediately exposed a product bug: Choir does not yet have researcher-grade
document acquisition and extraction. Current facts:

- user/candidate NixOS computers include `python3` and `nodejs`;
- they do not include `pandoc`, Poppler tools, LibreOffice, EPUB tooling, or
  Python document extraction packages;
- `fetch_url` returns a bounded string excerpt and is weak for binary
  documents;
- VText PDF import currently uses a `pdf_literal_text_projection` regex fallback
  that is not acceptable for publication-quality documents or recall evals;
- VText DOCX import has a basic OOXML text/table projection, but not a shared
  researcher-grade document extraction substrate;
- PPTX/HTML slide files matter as source documents, but the Slides app itself
  should be a separate mission.

Therefore the correct mission is not "run a PDF eval now." The correct mission
is to first give user computers real document tools, route researchers and VText
through the same ContentItem extraction substrate, then run the compaction
matrix on a frozen corpus.

## Cognitive Transforms Applied

### Substrate Before Benchmark

The shallow object is "make models read PDFs." The real object is a document
source substrate. If the substrate is bad, the eval measures extraction failure,
not memory or compaction.

Operational consequence: sandbox document tooling and ContentItem extraction
come before the compaction matrix.

### Frozen Corpus Boundary

The shallow eval would let each model search the web and read whatever it finds.
That burns search quota and makes the comparison noisy. The load-bearing
variable for this mission is post-compaction recall over the same evidence.

Operational consequence: fetch/import the corpus once, store durable source
artifacts, then give every model arm the same content handles and prohibit live
search during scoring.

### Source Artifact, Not Text Paste

Documents are not just prompt text. They have raw bytes, hashes, pages, slides,
paragraphs, headings, notes, media, caveats, and selectors.

Operational consequence: every imported document should become a ContentItem or
set of linked ContentItems with provenance, cleaned text, raw hash, extraction
warnings, and addressable selectors.

### Natural Retrieval

The previous proof used explicit instruction to call `get_run_memory_entry`.
That proves the tool works, not that the agent naturally recovers from
compaction.

Operational consequence: scoring prompts ask recall questions naturally. The
eval observes whether the agent answers from checkpoint state, recent context,
or naturally invokes retrieval from available handles.

### Slides As Source, App As Future Product

PPTX and HTML slide decks are legitimate source documents for research and
recall. A full Slides app is a different product surface.

Operational consequence: support PPTX/HTML slide extraction now; defer deck
playback/presentation UI to a separate mission. Do not let this mission drift
into building, designing, or partially scaffolding the Slides app.

## Hard Invariants

- No live search API usage during the scored model-matrix phase.
- No arbitrary normal-agent `max_tokens` caps. Use normal model policy and
  prompt for concise or detailed outputs when needed.
- Automatic compaction remains runtime-owned. Agents do not call a compaction
  tool.
- `get_run_memory_entry` remains the exact-retrieval escape hatch after
  compaction; the eval must not explicitly instruct the agent to call it.
- Raw run memory remains durable, owner-scoped, and retrievable.
- Every document source is treated as untrusted evidence, not instructions.
- ContentItem remains the owner-scoped document/source substrate.
- VText owns canonical document revisions; researcher tools do not write
  canonical VText prose.
- VText import, researcher reads, Global Wire sources, and future document apps
  should share the same extraction substrate instead of separate format hacks.
- Sandbox/user-computer tooling must be declared in NixOS image config, not
  assumed from local macOS or Node B host PATH.
- Platform behavior changes follow docs-first, commit, push, CI, deploy,
  staging identity, and deployed product-path proof.
- The Slides app is out of scope for this mission.

## Sandbox Setup Preamble Details

This section specifies the concrete substrate work required by Execution Gate 0.
Before running the compaction matrix, upgrade the normal user/candidate computer
image, not just the local development shell.

### Required Guest Tools

Add durable sandbox availability for:

- `python3` with document extraction packages;
- `nodejs` already exists and should remain available;
- `pandoc` for Markdown/HTML/DOCX/EPUB conversion and fallback extraction;
- `poppler-utils` for `pdftotext`, `pdfinfo`, and `pdftoppm`;
- `libreoffice` for PPTX/DOCX render/convert fallback where feasible.

Prefer a Nix-declared Python package set including:

- `pypdf`;
- `pdfplumber`;
- `python-docx`;
- `ebooklib`;
- `beautifulsoup4`;
- `lxml`;
- `markitdown` if available in nixpkgs or vendorable without making the image
  brittle.

If `markitdown` is not cleanly available, do not block the mission on it. Use
OOXML/Pandoc/LibreOffice fallbacks and record the caveat.

### Required Runtime Tools

Add or upgrade researcher-accessible tools around the ContentItem substrate:

- `import_document_content`: import a URL or existing file path into a
  ContentItem with real extraction, raw hash, cleaned text, selectors, media
  type, extraction adapter, warnings, and provenance;
- `read_content_item`: preserve current behavior, but support selector-aware
  reads for page, slide, paragraph, heading, text range, or chunk;
- `list_content_item_selectors`: expose document structure without forcing the
  model to load the entire document;
- `read_content_item_selector`: read exact addressable slices for precise
  recall and citation.

These may initially wrap existing `import_url_content` and `read_content_item`
if the API names remain stable, but the behavior must be document-grade. Do not
claim success while PDF/PPTX/DOCX/EPUB still fall back to low-quality text
snippets.

### Format Minimums

PDF:

- use Poppler and/or `pdfplumber`/`pypdf`;
- store page count, per-page text, extraction warnings, raw hash, and cleaned
  whole-document text;
- expose page selectors.

DOCX:

- preserve paragraphs, headings, tables, and core metadata where possible;
- use Pandoc and/or Python OOXML tooling;
- expose heading, paragraph, and table selectors.

EPUB:

- extract spine-ordered text and headings;
- preserve chapter/section selectors.

PPTX:

- extract slide titles, body text, speaker notes when available, slide count,
  and media metadata;
- expose per-slide selectors;
- use OOXML/Pandoc/LibreOffice/MarkItDown as available;
- do not build playback UI in this mission.

HTML slides:

- extract title, section/slide boundaries when detectable, body text, and source
  URL;
- preserve enough metadata for a future Slides app to open/render the original
  artifact.

## Frozen Corpus Design

Choose 4-8 public documents that are large enough to force compaction but
legally and operationally safe to fetch. Prefer stable public URLs and documents
with exact facts that can be checked later.

The corpus should include at least:

- one long PDF;
- one technical or scientific PDF;
- one DOCX or DOCX-like import if a stable public fixture is available;
- one EPUB or public-domain book if feasible;
- one PPTX or public slide deck if feasible;
- one HTML slide deck or long HTML document if feasible.

Discovery is not scored. It may be manual, direct-URL based, or done once by a
single baseline run. The scored phase receives only the frozen ContentItem
handles and explicit instruction not to search.

## Eval Shape

Each model arm should run a normal Choir researcher or VText-adjacent researcher
loop, with a scoped model-policy overlay selecting the target model. The task is
to read the frozen document handles, produce a concise evidence checkpoint, and
then continue after automatic compaction to answer held-out questions.

Score two recall classes:

Approximate recall:

- can the agent recover the document's thesis, structure, caveats, and key
  relationships after compaction?
- does the answer preserve uncertainty and source boundaries?

Exact recall:

- can the agent answer precise questions about names, numbers, definitions,
  section titles, page/slide-local details, or quoted spans?
- does the answer cite selectors or retrieve exact source/memory handles when
  needed, without being explicitly told which tool to call?

Also record:

- provider/model;
- resolved model policy;
- document extraction adapter versions;
- corpus ContentItem IDs and hashes;
- compaction threshold and checkpoint metadata;
- whether retrieval was used naturally;
- failures, hallucinations, refusals, context-loss events, and source-tool
  errors.

## Model Policy And Eval Control Plane

If scoped model-policy overlays do not yet exist, implement the minimum product
path needed to run the matrix:

- overlay id;
- owner/computer scope;
- role/profile target;
- provider/model/reasoning selection;
- expiration;
- trace visibility;
- no secret exposure.

The eval runner may be minimal, but it must not pass arbitrary model metadata
through prompt-bar requests as a hidden bypass. Model selection should look like
runtime policy, not prompt trickery.

### Control-Plane Problem Checkpoint: Base Policy Is Too Broad

Current runtime inspection shows that Choir has a computer-owned base policy at
`System/model-policy.toml`, loaded through `RUNTIME_MODEL_POLICY_PATH` or the
default files-root path. That is good as durable owner-visible policy, but it is
too broad for a recall matrix: rewriting the base file for each arm would affect
all runs on that computer until restored, and would make concurrent or resumed
evals ambiguous.

The current `spawn_agent` model argument is also not enough. It records a model
constraint, but provider/model selection for real tool-loop calls is resolved
from `llm_provider`/`llm_model` metadata and model policy. Passing arbitrary
provider/model metadata through prompt submission would be an invisible bypass,
not a policy-controlled eval surface.

Required fix before scored matrix:

- add an owner-visible scoped overlay path, such as
  `System/model-policy-overlays/<overlay_id>.toml`;
- allow a run to name an overlay id in trace-visible metadata;
- merge that overlay with the base model policy for the selected run only;
- support expiration and optional role narrowing;
- record the overlay id/source in run metadata;
- leave the base `System/model-policy.toml` intact.

This keeps model selection policy-shaped and retrievable without turning the
prompt bar into a hidden provider/model override API.

## Acceptance Evidence

This mission is complete only when all of the following are true on staging or
the authorized Node B user/candidate computer path:

- sandbox image config includes declared document extraction tools;
- researcher/VText document import no longer relies on regex-only PDF parsing;
- a frozen corpus is imported into durable ContentItems with hashes, warnings,
  cleaned text, and selectors;
- live search is disabled or unused during scored model arms;
- every target model arm runs through the normal Choir harness;
- automatic compaction fires under realistic context pressure;
- approximate and exact recall are scored after compaction;
- at least one exact-recall question naturally causes retrieval or selector
  reading when the answer is not in recent context;
- evidence records include run ids, trace refs, ContentItem refs, model policy,
  compaction metadata, and residual risks.

## Explicit Non-Goals

- Do not build the Slides app in this mission.
- Do not support native Keynote as a priority.
- Do not support native Google Slides as a local file format. Future Google
  integration may import/export through Google APIs, PPTX, PDF, or HTML.
- Do not spend search API quota in the scored eval.
- Do not tune prompts to force a specific memory tool call.
- Do not use local macOS tooling as acceptance proof for user/candidate
  computer behavior.

## Separate Future Mission: Slides App

Create a separate mission for a Choir Slides app after this document substrate
is in place. That mission should treat PPTX and HTML slides as playable deck
artifacts, with PDF/images as fallback views, and should reuse the same
ContentItem extraction and selector records created here.

Likely first goal:

```text
/goal Run docs/mission-slides-app-pptx-html-v0.md as MissionGradient; build a Choir Slides app that opens PPTX and HTML slide ContentItems, presents them cleanly in the desktop, preserves source/provenance metadata, and lets VText/researchers cite slide selectors without flattening decks into prose.
```

## Run Checkpoint & Resumption State

status: substrate_deployed_pre_eval

last checkpoint: mission upgraded so the sandbox/document substrate preamble is
an explicit prerequisite before any compaction recall eval. Slides remain
strictly parked as a future mission; this mission only extracts PPTX/HTML slide
artifacts as sources. The substrate behavior change has now been pushed,
passed CI, and deployed to staging; the frozen-corpus compaction eval has not
started.

current artifact state:

- LLM compaction exists and has prior staging proof.
- Provider conformance has been closed at readiness level for current
  DeepSeek/Xiaomi paths.
- Natural recall matrix has not started.
- Sandbox document tooling is declared in Nix image config and structurally
  evaluated for normal and Playwright worker images.
- Shared ContentItem extraction and researcher selector tools are implemented
  for PDF/DOCX/EPUB/PPTX/HTML before model runs.

what shipped:

- docs-only checkpoint `58b918af` records the problem and mission before code
  changes, satisfying the problem-documentation-first invariant.
- behavior commit `487b7515` adds the sandbox document tooling, shared
  extraction substrate, researcher document import/selector tools, VText import
  reuse, and focused tests.
- docs-only checkpoint `baa22ca2` records the model-policy control-plane
  problem: the base policy file is too broad for safe per-arm eval selection.
- pending behavior work adds scoped model-policy overlays at
  `System/model-policy-overlays/<overlay_id>.toml`, with per-run overlay ids
  recorded in metadata.
- pending behavior work also adds authenticated read-only
  `/api/model-policy/resolve` so staging can prove overlay resolution through a
  browser-public product route without using forbidden `/api/agent/*` routes.

what was proven so far:

- Nix evaluation succeeds for the normal sandbox VM and the Playwright worker
  VM with the document tool packages declared.
- Focused local tests prove shared extraction/selectors, researcher document
  selector tools, and VText file import reuse for DOCX/PDF/PPTX fixtures.
- URL document imports now get a larger document-only byte cap so public PDFs
  and decks are not forced through the ordinary 2 MiB web snippet limit.
- GitHub CI run `27169883438` completed successfully.
- FlakeHub publish run `27169883384` completed successfully.
- `https://choir.news/health` reports proxy and sandbox deployed at
  `487b75154dd835ddfd9a037d57b43b5a985fe876`.
- A deployed product-path PDF import proof succeeded through authenticated
  `https://choir.news` APIs:
  - owner/user id `0e39737f-4d6d-4591-8210-6124b06524f2`;
  - imported public PDF URL
    `https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf`;
  - created ContentItem `4222d5fc-aea5-43bd-93eb-077f9c3540a7`;
  - stored `source_type: extracted_url`, `media_type: application/pdf`,
    `app_hint: pdf`;
  - extracted cleaned text `Dummy PDF file`;
  - stored `extraction_adapter: pdf_poppler_pdftotext`;
  - stored selector `page-1`;
  - preserved raw content hash
    `3df79d34abbca99308e79cb94461c1893582604d68329a41fd4bec1885e6adb4`
    and extracted text hash
    `41417fb420a737c8064205cf4b7fac3fc7ce6bad26417be5b4f6f6012d92c951`.
- Focused local overlay tests prove:
  - a run with `llm_policy_overlay_id` resolves provider/model/reasoning from
    `System/model-policy-overlays/<id>.toml`;
  - expired overlays fall back to base policy and record a policy error;
  - child researcher runs inherit overlay ids into resolved `llm_provider` and
    `llm_model` metadata;
  - `spawn_agent` can pass a trace-visible `model_policy_overlay_id`.
- Focused comprehensive API test proves `/api/model-policy/resolve` resolves a
  researcher role through an owner-visible overlay file.

unproven or partial claims:

- document extraction tool availability in refreshed user/candidate computers
  beyond the deployed product API proof;
- high-quality long PDF/DOCX/EPUB/PPTX/HTML extraction through normal product
  tools;
- frozen-corpus matrix execution without live search;
- natural post-compaction recall across target models.
- deployed proof of scoped model-policy overlay selection on staging.

belief-state changes:

- compaction eval should be source-corpus based, not search based;
- document parsing is a prerequisite product capability;
- PPTX/HTML slide extraction belongs in source tooling now, while Slides app UI
  belongs in a future mission.

remaining error field:

- image/package size impact of adding document tools;
- extraction quality variance across file formats;
- model-policy overlay implementation status;
- whether all target providers remain available during the run.

highest-impact remaining uncertainty:

- can the sandbox image and ContentItem tools support real multi-format document
  import without creating brittle one-off VText import hacks?

latest local proof:

- `nix eval .#nixosConfigurations.go-choir-sandbox-vm.config.system.build.toplevel.drvPath`
- `nix eval .#nixosConfigurations.go-choir-sandbox-vm-playwright.config.system.build.toplevel.drvPath`
- `nix develop -c go test ./internal/runtime -run 'TestRuntime.*ModelPolicy|TestStartChildRunResolvesModelPolicy|TestParseModelPolicy|TestEnsureDefaultModelPolicy'`
- `nix develop -c go test ./internal/runtime -run 'TestAgentToolProfiles|TestStartChildRunResolvesModelPolicy|TestRuntimeResolvesModelPolicy|TestRuntimeRejectsExpiredModelPolicyOverlay|TestProviderPreconditionFallbackSelections|TestRunToolLoop'`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestHandleModelPolicyResolveUsesOverlayFile'`
- `nix develop -c go test ./internal/runtime -run 'TestExtract|TestSystemPromptForResearcher|TestContent'`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestResearcherDocumentSelectorToolsReadPPTXSourceArtifact|TestAgentToolProfiles|TestVTextOpenFileImportsDocxAndPDFBytesFromFilesRoot'`

latest staging proof:

- `gh run watch 27169883438 --exit-status`
- `gh run view 27169883384 --json status,conclusion,workflowName,url,headSha`
- `curl -fsS https://choir.news/health`
- authenticated staging product-path import of public PDF ContentItem
  `4222d5fc-aea5-43bd-93eb-077f9c3540a7` with Poppler extraction and page
  selector metadata.

next executable probe:

- commit, push, and deploy the scoped model-policy overlay path; verify staging
  identity and run a deployed product-path proof that an overlay file under
  `System/model-policy-overlays/` controls a researcher run's resolved
  provider/model. Then build the frozen multi-format corpus.

suggested resume goal string:

```text
/goal Run docs/mission-natural-compaction-pdf-recall-eval-v0.md as MissionGradient; first upgrade sandbox document extraction for PDF/DOCX/EPUB/PPTX/HTML sources, then run a frozen-corpus natural compaction recall matrix across DeepSeek, Xiaomi, and gpt-5.4-mini without live search, proving approximate recall, exact retrieval, and automatic post-compaction continuation through normal Choir researcher/VText runs.
```

evidence artifact refs: none yet.

rollback refs: none yet.
