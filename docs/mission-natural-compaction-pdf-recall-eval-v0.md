# Natural Compaction Document Recall Eval Mission v0

## Mission Identity

Run a realistic, search-quota-free compaction recall matrix across Choir's
current first-class model providers by using long public documents as a frozen
corpus. Before running the model matrix, upgrade the user/candidate computer
sandbox image and researcher/VText document tools so agents can acquire,
extract, read, cite, and recall real documents instead of relying on brittle
URL snippets or regex PDF extraction.

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
playback/presentation UI to a separate mission.

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

## Sandbox Setup Preamble

Before running the compaction matrix, upgrade the normal user/candidate computer
image, not just the local development shell.

### Required Guest Tools

Add durable sandbox availability for:

- `python3` with document extraction packages;
- `nodejs` already exists and should remain available;
- `pandoc` for Markdown/HTML/DOCX/EPUB conversion and fallback extraction;
- `poppler_utils` for `pdftotext`, `pdfinfo`, and `pdftoppm`;
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

status: checkpoint_incomplete

last checkpoint: mission authored from provider/compaction learning and owner
feedback that search quota should not be spent on evals and document extraction
must be real before recall testing.

current artifact state:

- LLM compaction exists and has prior staging proof.
- Provider conformance has been closed at readiness level for current
  DeepSeek/Xiaomi paths.
- Natural recall matrix has not started.
- Sandbox document tooling is not yet upgraded.

what shipped: nothing yet for this mission.

what was proven: not yet proven in this mission.

unproven or partial claims:

- document extraction tool availability in refreshed user/candidate computers;
- high-quality PDF/DOCX/EPUB/PPTX/HTML extraction through normal product tools;
- frozen-corpus matrix execution without live search;
- natural post-compaction recall across target models.

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

next executable probe:

- inspect `nix/sandbox-vm.nix`, `internal/runtime/content.go`,
  `internal/runtime/tools_research.go`, and `internal/runtime/vtext_import.go`;
  document the current document extraction problem; then add sandbox packages
  and shared ContentItem extraction adapters.

suggested resume goal string:

```text
/goal Run docs/mission-natural-compaction-pdf-recall-eval-v0.md as MissionGradient; first upgrade sandbox document extraction for PDF/DOCX/EPUB/PPTX/HTML sources, then run a frozen-corpus natural compaction recall matrix across DeepSeek, Xiaomi, and gpt-5.4-mini without live search, proving approximate recall, exact retrieval, and automatic post-compaction continuation through normal Choir researcher/VText runs.
```

evidence artifact refs: none yet.

rollback refs: none yet.
