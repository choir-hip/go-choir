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

## Mission Phase Order

This mission is deliberately ordered. Do not start the compaction matrix until
the substrate has been proven in the environment where Choir agents actually
run.

Phase 0: sandbox setup preamble.

- Inspect the normal user/candidate computer NixOS image config.
- Add or verify document extraction tools there, not only in local dev shell.
- Prove the refreshed image or deployed product path can import representative
  PDF, DOCX, EPUB, PPTX, and HTML/HTML-slide sources.
- Prove extracted ContentItems expose hashes, adapter metadata, selectors, and
  selector reads through researcher/VText-compatible tooling.

Phase 1: frozen corpus setup.

- Import the frozen corpus once through product routes or normal researcher
  tools.
- Record owner, ContentItem ids, hashes, adapters, selector counts, warnings,
  and exact held-out markers.
- Do not spend live search quota during scored eval work.

Phase 2: model-policy control.

- Use scoped owner-visible model-policy overlays for each model arm.
- Do not rewrite broad base model policy as a substitute for scoped eval
  control.
- Do not pass hidden provider/model overrides through prompt text or prompt-bar
  metadata.

Phase 3: natural compaction matrix.

- Run normal Choir researcher or VText-adjacent researcher loops.
- Let runtime-owned automatic compaction fire at the configured context
  threshold.
- Score approximate recall, exact recall, natural retrieval/selector use, and
  post-compaction continuation.

Phase 4: evidence and resumption.

- Record run ids, trace refs, model policy resolution, compaction metadata,
  ContentItem refs, failures, and residual risks.
- Update this mission doc before stopping.

## Execution Gate 0: Sandbox/User-Computer Setup Before Compaction

This mission has a hard preamble. Before any model arm is run, the worker must
prove that normal Choir user/candidate computers can ingest and expose real
documents through the same product substrate that researchers and VText use.
The compaction matrix is downstream of this proof; it is not allowed to treat
PDF/DOCX/EPUB/PPTX/HTML support as a side quest, a local-only precondition, or
an eval harness special case.

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
- the refreshed normal user/candidate computer path is verified, or the mission
  explicitly records why current staging product-path proof is the accepted
  proxy for that environment;
- at least one deployed product-path import proof confirms the behavior on
  staging or an authorized Node B user/candidate computer path;
- no Slides app files, routes, icons, or UI scaffolding are added by this
  mission.

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
  is a separate mission and must not be started here.

Therefore the correct mission is not "run a PDF eval now." The correct mission
is to first give user computers real document tools, route researchers and VText
through the same ContentItem extraction substrate, prove that substrate through
the product path, then run the compaction matrix on a frozen corpus.

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
playback/presentation UI to a separate mission. Do not let this mission create
slides routes, desktop icons, deck playback UI, presentation controls, or any
other partial Slides app scaffolding.

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

status: matrix_attempt_incomplete_text_selector_gap

last checkpoint: mission upgraded so the sandbox/user-computer document
substrate preamble is an explicit prerequisite before any compaction recall
eval. Slides remain strictly parked as a future mission; this mission only
extracts PPTX/HTML slide artifacts as sources. The substrate and scoped
model-policy control-plane behavior changes have been pushed, passed CI,
deployed to staging, and proven through authenticated product routes. A narrow
uploaded-file ContentItem import route has also shipped and been proven with an
uploaded PPTX fixture. HTML URL imports now include selector and adapter
metadata. A seven-item frozen corpus has been imported under one staging owner.
The pre-matrix sandbox conformance check was rerun against the NixOS sandbox
and Playwright worker configurations. The narrow authenticated product-visible
compaction recall eval runner has shipped and been proven on staging without
opening `/api/agent/*` to browser-public acceptance. A first five-arm matrix
attempt completed across DeepSeek, Xiaomi, and ChatGPT with live search
disabled and zero search attempts. It did not satisfy the mission because no
arm triggered automatic compaction. The root cause observed from the corpus is
that imported `text/plain` RFC documents stored large text but exposed no
selector chunks or extraction adapter metadata, so agents could only perform
bounded whole-ContentItem reads and did not create enough context pressure for
the 700k-token DeepSeek/Xiaomi compaction threshold.

current artifact state:

- LLM compaction exists and has prior staging proof.
- Provider conformance has been closed at readiness level for current
  DeepSeek/Xiaomi paths.
- Natural recall matrix has not started.
- Sandbox document tooling is declared in Nix image config and structurally
  evaluated for normal and Playwright worker images.
- Shared ContentItem extraction and researcher selector tools are implemented
  for PDF/DOCX/EPUB/PPTX/HTML before model runs.
- Scoped model-policy overlays are implemented for per-run eval arm selection
  without rewriting the base `System/model-policy.toml`.

what shipped:

- docs-only checkpoint `58b918af` records the problem and mission before code
  changes, satisfying the problem-documentation-first invariant.
- behavior commit `487b7515` adds the sandbox document tooling, shared
  extraction substrate, researcher document import/selector tools, VText import
  reuse, and focused tests.
- docs-only checkpoint `baa22ca2` records the model-policy control-plane
  problem: the base policy file is too broad for safe per-arm eval selection.
- behavior commit `5b4371ec` adds scoped model-policy overlays at
  `System/model-policy-overlays/<overlay_id>.toml`, with per-run overlay ids
  recorded in metadata.
- behavior commit `46f1b764` adds authenticated read-only
  `/api/model-policy/resolve` so staging can prove overlay resolution through a
  browser-public product route without using forbidden `/api/agent/*` routes.
- docs-only checkpoint `d40deea2` records the uploaded-file corpus import gap:
  `ImportFileContent` existed for researcher tools, but repeatable corpus setup
  needed an authenticated product route.
- behavior commit `d3b4cb98` adds authenticated
  `POST /api/content/import-file`, plus a frontend helper and a comprehensive
  PPTX file-import regression test.
- docs-only checkpoint `a3de9530` records the HTML corpus selector gap before
  code changes.
- behavior commit `97cdd6d7` routes HTML URL imports through the shared
  extraction substrate so cleaned HTML reader ContentItems now preserve
  selector metadata, extraction adapter identity, and extracted text hash.
- docs-only checkpoint `67038d71` records the eval runner control-plane gap:
  prompt-bar rejects hidden runtime metadata, `/api/agent/*` is intentionally
  not browser-public, and `/internal/runtime/runs` is service-to-service only.
- behavior commit `610e0a04` adds authenticated
  `POST /api/evals/compaction-recall` and
  `GET /api/evals/compaction-recall/runs/{runID}`, validates owner-scoped
  frozen ContentItems and scoped model-policy overlays, starts a normal
  researcher run with trace-visible eval metadata, and blocks live source
  acquisition tools during frozen-corpus eval runs.

what was proven so far:

- Nix evaluation succeeds for the normal sandbox VM and the Playwright worker
  VM with the document tool packages declared.
- Pre-matrix sandbox conformance was rechecked after checkpoint
  `15fc4d0a`:
  - `nix eval .#nixosConfigurations.go-choir-sandbox-vm.config.system.build.toplevel.drvPath`
    returned sandbox derivation
    `/nix/store/k1vsc5357m9kh8qm5jwl5qgalma8hdck-nixos-system-go-choir-sandbox-26.05.20260409.4c1018d.drv`;
  - `nix eval .#nixosConfigurations.go-choir-sandbox-vm-playwright.config.system.build.toplevel.drvPath`
    returned Playwright worker derivation
    `/nix/store/wb6ji3ifnh5df7x5i4wap5d16jyvjppx-nixos-system-go-choir-playwright-worker-26.05.20260409.4c1018d.drv`;
  - `nix/sandbox-vm.nix` declares `documentPython` with
    `beautifulsoup4`, `ebooklib`, `lxml`, `pdfplumber`, `pypdf`, and
    `python-docx`, and includes `libreoffice`, `pandoc`, and
    `poppler-utils` in both the sandbox service PATH and system packages.
- Focused local tests prove shared extraction/selectors, researcher document
  selector tools, and VText file import reuse for DOCX/PDF/PPTX fixtures.
- Focused pre-matrix tool tests passed after checkpoint `15fc4d0a`:
  - `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestResearcherDocumentSelectorToolsReadPPTXSourceArtifact|TestAgentToolProfiles|TestVTextOpenFileImportsDocxAndPDFBytesFromFilesRoot' -count=1`;
  - `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestContentImportURLCreatesProvenanceRecord|TestContentImportURLCleansReaderChrome|TestContentImportFileCreatesExtractedPPTXContentItem' -count=1`.
- Focused eval-runner tests passed after checkpoint `67038d71`:
  - `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestHandleCompactionRecallEvalStartsResearcherWithOverlayAndFrozenContent|TestHandleModelPolicyResolveUsesOverlayFile|TestRegisteredPromptBarRouteAcceptsIntentOnly' -count=1`;
  - `nix develop -c go test ./internal/runtime -run 'TestFrozenCorpusEvalDisablesLiveSourceAcquisitionTools|TestRuntimeRejectsExpiredModelPolicyOverlay|TestRunToolLoop' -count=1`.
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
- GitHub CI run `27171676079` completed successfully for `46f1b764`.
- FlakeHub publish run `27171676075` completed successfully for `46f1b764`.
- `https://choir.news/health` reports proxy and sandbox deployed at
  `46f1b764d15adaf30314d14cc5a1b7b61f7d728d`.
- A deployed product-path model-policy overlay proof succeeded through an
  authenticated `https://choir.news` passkey session:
  - owner/user id `ffae0057-fa57-4301-8e49-486764bb6ed6`;
  - owner email `codex-overlay-proof-1780959566031@example.test`;
  - overlay file
    `System/model-policy-overlays/compaction-eval-1780959566031.toml`;
  - overlay expiration `2026-06-09T22:59:26.031Z`;
  - authenticated `PUT /api/files/System/model-policy-overlays/compaction-eval-1780959566031.toml`
    returned `200`;
  - authenticated `GET /api/model-policy/resolve?role=researcher&overlay_id=compaction-eval-1780959566031`
    returned `provider: xiaomi`, `model: mimo-v2.5-pro`,
    `reasoning_effort: medium`, and source
    `/mnt/persistent/files/System/model-policy-overlays/compaction-eval-1780959566031.toml`.
- Focused local proof for file import:
  - `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestContentImportFileCreatesExtractedPPTXContentItem|TestContentImportURLCreatesProvenanceRecord|TestContentCreateSupportsDurableMediaReferences'`;
  - `nix develop -c go test ./internal/runtime -run 'TestRegisteredPromptBarRouteAcceptsIntentOnly|TestRuntimeRejectsExpiredModelPolicyOverlay|TestRunToolLoop'`.
- GitHub CI run `27173112280` completed successfully for `d3b4cb98`,
  including staging deploy.
- FlakeHub publish run `27173112299` completed successfully for `d3b4cb98`.
- `https://choir.news/health` reports proxy and sandbox deployed at
  `d3b4cb98798d895a627c7c94695175af17f6a011`.
- A deployed product-path uploaded-file import proof succeeded through an
  authenticated `https://choir.news` passkey session:
  - owner/user id `ccde4835-7fa6-4c15-bef9-1dbfb486f2eb`;
  - owner email `codex-file-import-proof-1780960904946@example.test`;
  - uploaded PPTX path `frozen-corpus/proof-1780960904946.pptx`;
  - authenticated `PUT /api/files/frozen-corpus/proof-1780960904946.pptx`
    returned `200`;
  - authenticated `POST /api/content/import-file` returned ContentItem
    `fcda1409-8740-49b1-b35b-bd27de323e2c`;
  - imported item stored `source_type: file`,
    `media_type: application/vnd.openxmlformats-officedocument.presentationml.presentation`,
    `app_hint: slides`, `extraction_adapter:
    pptx_ooxml_slide_text_projection`, `selector_count: 2`,
    `content_hash: 94344408a0b514ec8f9c932d23e60a0adaf81f56e411b172720494adb559c804`,
    raw hash `sha256:94344408a0b514ec8f9c932d23e60a0adaf81f56e411b172720494adb559c804`,
    extracted text hash
    `657b946e7f715876f9ae1b3e92c28307596364e3c7fa49bd6b0e59cee1f372b0`,
    and slide selectors `slide-1`, `slide-2`.
- Focused local proof for HTML selectors:
  - `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestContentImportURLCreatesProvenanceRecord|TestContentImportURLCleansReaderChrome|TestContentImportFileCreatesExtractedPPTXContentItem'`;
  - `nix develop -c go test ./internal/runtime -run 'TestRegisteredPromptBarRouteAcceptsIntentOnly|TestRuntimeRejectsExpiredModelPolicyOverlay|TestRunToolLoop'`.
- GitHub CI run `27173572286` completed successfully for `97cdd6d7`,
  including staging deploy.
- FlakeHub publish run `27173572312` completed successfully for `97cdd6d7`.
- `https://choir.news/health` reports proxy and sandbox deployed at
  `97cdd6d768d464f0888b28cf27b6581a6542f174`.
- A deployed product-path HTML selector proof succeeded through an authenticated
  `https://choir.news` passkey session:
  - owner/user id `ea8881a6-e15c-42dd-b0af-840e220b8bc8`;
  - owner email `codex-html-selector-proof-1780961567082@example.test`;
  - imported `https://www.rfc-editor.org/rfc/rfc9110.html`;
  - created ContentItem `a46fd4e4-fa98-40f4-83ed-8eb1efd6a89d`;
  - stored `source_type: extracted_url`, `media_type: text/markdown`,
    `app_hint: content`, `extraction_adapter: html_readability_lite`,
    `selector_count: 26`, raw content hash
    `d431760660ea44e130f6e919dab216df2d0b3a490567a98089267523368fe1e5`,
    and extracted text hash
    `5468b2e39789c8fdb53391ec23818b1507560b4b1fee5172ef50dae2a15fcbb2`.
- Frozen corpus import succeeded under one staging owner:
  - owner/user id `49bc8b74-2158-46e2-b387-a7a9a40fb6ad`;
  - owner email `codex-frozen-corpus-1780961642721@example.test`;
  - `long_pdf`: RFC 9000 QUIC transport PDF,
    ContentItem `6b8c0aba-ed20-4c39-abd6-b20f5089ae83`,
    adapter `pdf_poppler_pdftotext`, selectors `151`, raw hash
    `24f411581702fea968f554264a629a80aa5a03a2a959733063391575256edcc7`;
  - `technical_pdf`: Attention Is All You Need PDF,
    ContentItem `6c90d9cb-9206-4e35-bb74-3a0d9d5226cf`,
    adapter `pdf_poppler_pdftotext`, selectors `15`, raw hash
    `bdfaa68d8984f0dc02beaca527b76f207d99b666d31d1da728ee0728182df697`;
  - `technical_html`: RFC 9110 HTTP Semantics HTML,
    ContentItem `1b660621-2ca6-467d-85a9-0230b5c624fb`,
    adapter `html_readability_lite`, selectors `26`, raw hash
    `d431760660ea44e130f6e919dab216df2d0b3a490567a98089267523368fe1e5`;
  - `docx`: Calibre demo DOCX,
    ContentItem `91ab2952-fa7e-46d7-a51d-cca4b8248fa6`,
    adapter `docx_pandoc_markdown`, selectors `16`, raw hash
    `269329fc7ae54b3f289b3ac52efde387edc2e566ef9a48d637e841022c7e0eab`;
  - `epub`: Calibre demo EPUB,
    ContentItem `a5244a41-cd9e-452f-a8af-2730336ce81c`,
    adapter `epub_pandoc_markdown`, selectors `5`, raw hash
    `c516c9d535d6a840255b77ade39a2352a022015be2d7cf8726c75671f314e970`;
  - `html_slides`: reveal.js HTML demo,
    ContentItem `339088bd-5b3d-40a0-ba23-acc3a40c51f4`,
    adapter `html_readability_lite`, selectors `1`, raw hash
    `a41c6e23b54eea4719087d2248cfdcc252dd0429d17be7498f415611e8f291b9`;
  - `pptx_uploaded`: uploaded frozen corpus deck at
    `frozen-corpus/eval-deck-1780961652728.pptx`,
    ContentItem `89cb6993-6d40-440c-9700-0f5d3c24a468`,
    adapter `pptx_ooxml_slide_text_projection`, selectors `3`, raw hash
    `5e69f62447cc5b88c42d0ac39719e10328933e6364bb829af159500140508acb`.
- GitHub CI run `27174880474` completed successfully for `610e0a04`,
  including staging deploy.
- FlakeHub publish run `27174880473` completed successfully for `610e0a04`.
- `https://choir.news/health` reports proxy and sandbox deployed at
  `610e0a047d93474ebd46f208ce704562fa894590`.
- A deployed product-path compaction eval runner proof succeeded through an
  authenticated `https://choir.news` passkey session:
  - owner/user id `04b2d60d-31c7-4f1d-80ff-62537f9115b8`;
  - owner email `codex-compaction-eval-proof-1780963873943@example.test`;
  - owner-visible overlay id `compaction-proof-1780963873943`;
  - frozen proof ContentItem `e9239e9c-8661-4fad-877a-4a3a5fc877b9`;
  - authenticated `POST /api/evals/compaction-recall` returned `202` with run
    id `3318c728-ded4-4c46-a3e7-e35ec05f93c3`;
  - authenticated
    `GET /api/evals/compaction-recall/runs/3318c728-ded4-4c46-a3e7-e35ec05f93c3`
    returned `200`;
  - the run resolved `provider: chatgpt`, `model: gpt-5.4-mini`,
    `reasoning_effort: low`;
  - status metadata included `eval_kind: compaction_recall` and
    `live_search_disabled: true`.
- A first deployed product-path matrix attempt ran across all target arms using
  owner/user id `c8fb7e42-9423-4cd6-b323-6f5a2443b119` and owner email
  `codex-compaction-matrix-1780964521974@example.test`. It imported 16 public
  source documents totaling 2,938,957 stored text characters and launched one
  run per target model through `/api/evals/compaction-recall`:
  - `deepseek-v4-flash`: run `65fdefe8-68cd-4f6d-acdd-5b000c5281f1`,
    completed, 7,868 result chars, 38 source-read trace moments, zero search
    attempts, zero compaction moments;
  - `deepseek-v4-pro`: run `8ee1d9e6-c5a5-4e5d-8f06-707d6b8350e0`,
    completed, 6,524 result chars, 40 source-read trace moments, zero search
    attempts, zero compaction moments;
  - `mimo-v2.5`: run `c04efee2-93da-44f4-9a2a-d7e575dc614f`,
    completed, 6,337 result chars, 32 source-read trace moments, zero search
    attempts, zero compaction moments;
  - `mimo-v2.5-pro`: run `0c467a65-5b46-43b7-8523-56eeb8c3e89a`,
    completed, 9,065 result chars, 40 source-read trace moments, zero search
    attempts, zero compaction moments;
  - `gpt-5.4-mini`: run `34f3cbc0-3bee-4173-a82f-ebc6bfa9a7fb`,
    completed, 5,107 result chars, 24 source-read trace moments, zero search
    attempts, zero compaction moments.
- Matrix attempt corpus finding: the 15 imported RFC `.txt` documents had
  `selector_count: 0` and no `extraction_adapter` despite large stored text
  bodies. The PDF item had `pdf_poppler_pdftotext` and 15 selectors. This is
  now the blocking substrate gap for natural compaction pressure.

unproven or partial claims:

- frozen-corpus matrix execution without live search is proven at route/trace
  level for one attempt, but not yet with automatic compaction;
- natural post-compaction recall across target models.
- full automatic-compaction trigger evidence for the new eval runner; route
  launch and metadata are proven, but the model matrix still needs to drive
  enough context pressure to force runtime-owned LLM compaction.
- `text/plain` URL imports do not yet expose selector chunks, which undermines
  the eval's ability to walk large frozen public text documents at high source
  pressure.

belief-state changes:

- compaction eval should be source-corpus based, not search based;
- document parsing is a prerequisite product capability;
- PPTX/HTML slide extraction belongs in source tooling now, while Slides app UI
  belongs in a future mission.
- the sandbox setup proof is now the entry gate that must remain satisfied
  before every recall-matrix attempt; the next run should not regress to local
  macOS-only extraction or prompt-text model overrides.
- first matrix attempt proves provider routing and no-search enforcement, but
  not compaction; source substrate selector quality is now the main loss term.

remaining error field:

- image/package size impact of adding document tools;
- extraction quality variance across file formats;
- whether all target providers remain available during the run.
- eval runner realism: route launch is proven, but the matrix must still prove
  it can run all target arms through normal researcher loops with enough frozen
  corpus pressure to compact.
- text/plain extraction currently creates no selector graph, so large public
  text sources are not as walkable as PDFs/DOCX/EPUB/PPTX/HTML.

highest-impact remaining uncertainty:

- can the shipped product-visible eval runner drive all target models into
  automatic compaction and preserve both approximate and exact recall without
  live search or explicit memory-tool prompt steering?
- will adding text/plain chunk selectors be enough to drive natural source
  traversal above the compaction threshold without introducing fake eval-only
  pathways?

latest local proof:

- `nix eval .#nixosConfigurations.go-choir-sandbox-vm.config.system.build.toplevel.drvPath`
- `nix eval .#nixosConfigurations.go-choir-sandbox-vm-playwright.config.system.build.toplevel.drvPath`
- `nix develop -c go test ./internal/runtime -run 'TestRuntime.*ModelPolicy|TestStartChildRunResolvesModelPolicy|TestParseModelPolicy|TestEnsureDefaultModelPolicy'`
- `nix develop -c go test ./internal/runtime -run 'TestAgentToolProfiles|TestStartChildRunResolvesModelPolicy|TestRuntimeResolvesModelPolicy|TestRuntimeRejectsExpiredModelPolicyOverlay|TestProviderPreconditionFallbackSelections|TestRunToolLoop'`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestHandleModelPolicyResolveUsesOverlayFile'`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestContentImportFileCreatesExtractedPPTXContentItem|TestContentImportURLCreatesProvenanceRecord|TestContentCreateSupportsDurableMediaReferences'`
- `nix develop -c go test ./internal/runtime -run 'TestRegisteredPromptBarRouteAcceptsIntentOnly|TestRuntimeRejectsExpiredModelPolicyOverlay|TestRunToolLoop'`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestContentImportURLCreatesProvenanceRecord|TestContentImportURLCleansReaderChrome|TestContentImportFileCreatesExtractedPPTXContentItem'`
- `nix develop -c go test ./internal/runtime -run 'TestExtract|TestSystemPromptForResearcher|TestContent'`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestResearcherDocumentSelectorToolsReadPPTXSourceArtifact|TestAgentToolProfiles|TestVTextOpenFileImportsDocxAndPDFBytesFromFilesRoot'`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestHandleCompactionRecallEvalStartsResearcherWithOverlayAndFrozenContent|TestHandleModelPolicyResolveUsesOverlayFile|TestRegisteredPromptBarRouteAcceptsIntentOnly' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestFrozenCorpusEvalDisablesLiveSourceAcquisitionTools|TestRuntimeRejectsExpiredModelPolicyOverlay|TestRunToolLoop' -count=1`

latest staging proof:

- `gh run watch 27169883438 --exit-status`
- `gh run view 27169883384 --json status,conclusion,workflowName,url,headSha`
- `gh run view 27171676079 --json status,conclusion,workflowName,url,headSha`
- `gh run view 27171676075 --json status,conclusion,workflowName,url,headSha`
- `gh run view 27173112280 --json status,conclusion,workflowName,url,headSha`
- `gh run view 27173112299 --json status,conclusion,workflowName,url,headSha`
- `gh run view 27173572286 --json status,conclusion,workflowName,url,headSha`
- `gh run view 27173572312 --json status,conclusion,workflowName,url,headSha`
- `curl -fsS https://choir.news/health`
- authenticated staging product-path import of public PDF ContentItem
  `4222d5fc-aea5-43bd-93eb-077f9c3540a7` with Poppler extraction and page
  selector metadata.
- authenticated staging product-path overlay resolver proof for overlay
  `compaction-eval-1780959566031`, resolving researcher to
  `xiaomi/mimo-v2.5-pro` with `medium` reasoning.
- authenticated staging product-path uploaded-PPTX proof for ContentItem
  `fcda1409-8740-49b1-b35b-bd27de323e2c`, preserving raw hash, extracted text
  hash, adapter metadata, and slide selectors.
- authenticated staging product-path HTML selector proof for ContentItem
  `a46fd4e4-fa98-40f4-83ed-8eb1efd6a89d`, preserving raw hash, extracted text
  hash, adapter metadata, and 26 selectors.
- authenticated staging frozen corpus import for owner
  `49bc8b74-2158-46e2-b387-a7a9a40fb6ad`, creating seven ContentItems across
  PDF, HTML, DOCX, EPUB, HTML slides, and uploaded PPTX.
- `gh run view 27174880474 --json status,conclusion,workflowName,url,headSha`
- `gh run view 27174880473 --json status,conclusion,workflowName,url,headSha`
- `curl -fsS https://choir.news/health`
- authenticated staging compaction eval runner proof for run
  `3318c728-ded4-4c46-a3e7-e35ec05f93c3`, using overlay
  `compaction-proof-1780963873943` and frozen proof ContentItem
  `e9239e9c-8661-4fad-877a-4a3a5fc877b9`; route launch and status retrieval
  succeeded through `/api/evals/compaction-recall` and
  `/api/evals/compaction-recall/runs/{runID}`.
- deployed matrix attempt evidence artifact
  `/tmp/choir-compaction-matrix-1780964521974.json`;
- authenticated staging matrix attempt for owner
  `c8fb7e42-9423-4cd6-b323-6f5a2443b119`, five completed model arms, zero
  search attempts, source-read trace moments per run, and zero compaction
  moments.

next executable probe:

- fix the shared extraction substrate so imported `text/plain` public documents
  produce chunk selectors, raw/extracted hash metadata, and an extraction
  adapter name. Then rerun a compaction-pressure matrix or pilot using large
  public text ContentItems plus PDF selectors, keeping live source acquisition
  disabled and proving whether selector walks can cross the automatic
  compaction threshold.

suggested resume goal string:

```text
/goal Run docs/mission-natural-compaction-pdf-recall-eval-v0.md as MissionGradient; first upgrade sandbox document extraction for PDF/DOCX/EPUB/PPTX/HTML sources, then run a frozen-corpus natural compaction recall matrix across DeepSeek, Xiaomi, and gpt-5.4-mini without live search, proving approximate recall, exact retrieval, and automatic post-compaction continuation through normal Choir researcher/VText runs.
```

evidence artifact refs: none yet.

rollback refs: none yet.
