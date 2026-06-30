# Mission - VText Source Intake And Conductor URL Routing H029 - v0

Status: successor paradoc stub.

Source: `docs/mission-doctrine-conformance-findings-2026-06-13.md`,
`docs/choir-doctrine.md` H029,
`docs/source-external-data-publication.md`, and the stale routing text in
`internal/runtime/prompt_defaults/conductor.md`.

## Problem Record

The current Conductor prompt default still says a bare ordinary web URL should
route to `browser`. That is H029 residue: Browser-as-source-gathering is
retired as user doctrine.

The larger product need is not merely "do not open Browser." The owner needs
source intake into VText:

- text URLs should be acquired as durable source artifacts, cleaned into reader
  text when possible, and transcluded into VText;
- YouTube URLs should import transcript segments as a source artifact, preserve
  timestamp selectors, and transclude that source into VText so the owner can
  write reviews or respond to specific segments;
- podcast episode links should eventually resolve to transcript sources by
  finding an existing transcript or generating one through a researched
  speech-to-text path;
- PDF and EPUB URLs, plus uploaded files, should be extracted into source
  artifacts first, then transcluded into VText, while still allowing expansion
  into the PDF/EPUB app for the full reader surface;
- researchers should be able to use these ingested sources, not only open-web
  search, and should be able to inspect both source transcripts/text and the
  owner's commentary in VText.

This is a code-bearing prompt/default and source-intake mission. Runtime prompt
markdown is executable policy even when the filename ends in `.md`. The doctrine
sweep did not change runtime prompt defaults; it discovered this contradiction
and deferred it.

## Product Wedge

First wedge: YouTube link to VText transcript source.

Expected owner experience:

1. The owner pastes a YouTube URL into the prompt bar, an existing VText, or a
   new VText.
2. Choir treats the URL as source intake, not as a command to open a Browser app.
3. The system creates a durable source artifact for the video transcript using a
   YouTube transcript API or adapter selected by evidence.
4. The transcript is stored with segment/timestamp selectors and provenance.
5. VText receives a source transclusion that can expand inline and open an
   owning source/media surface.
6. The owner writes review/commentary against exact transcript segments.
7. Researcher agents can search or resolve the transcript and the VText
   commentary as evidence when drafting a review, response, or synthesis.

Second wedge: ordinary text URLs, PDFs, EPUBs, and uploads through the same
source-artifact contract.

Later wedge: podcasts. Podcast links may come from arbitrary pages, RSS feeds,
platform episode pages, or media-file URLs. The first podcast mission should
identify episode/audio metadata, look for existing transcripts, and only then
fall back to a researched speech-to-text system. "Whisper" is a placeholder for
the speech-to-text capability class until research selects the right provider,
model, cost/latency/privacy policy, and segment-alignment strategy.

Out of scope here: text-to-speech, full podcast product redesign, general vector
index service, broad source corpus search over ingested news, and complete PDF
annotation UX. Those are successor missions unless they are required to prove
the first YouTube transcript transclusion slice.

## Portfolio Fit

This is a side product wedge and H029 repair mission. It does not decrease the
architecture-spine portfolio variant unless it removes a Browser-as-source
heresy that blocks M3/M4/M5 work. It can, however, become an important product
falsifier after the durable-actor spine is stable: a real YouTube-review VText
requires Conductor, VText, source artifacts, researcher evidence, app surfaces,
and publication/export policy to cooperate.

Near-term ordering:

- now: document the problem and keep the doctrine commit clean;
- next code-bearing H029 slice: remove Conductor's stale Browser route and
  prove source-intake routing with focused tests;
- first product slice: YouTube transcript source artifact plus VText
  transclusion;
- later source slices: PDFs/EPUBs/uploads, then podcasts with transcript
  discovery and speech-to-text fallback;
- later platform slice: researcher search over ingested private/news/source
  corpora once the broader source architecture can carry it.

## Parallax State

status: open_handoff

mission conjecture: if Choir routes URL/file/media inputs into a durable
source-intake contract, with YouTube transcript-to-VText transclusion as the
first concrete slice, then Conductor URL routing stops reintroducing retired
Browser-as-source-gathering ontology and VText becomes a practical review/
response workspace over real source material.

deeper goal (G): keep top-level prompt and VText source workflows aligned with
Choir Doctrine: truth from facts, evidence-bounded claims, VText as canonical
document/version core, source artifacts as durable evidence, researcher access
to source facts, and Web Lens as explicit live/original inspection rather than
default source intake.

witness/spec (A/S): a staged code-bearing change set:

- H029 prompt/default repair in `internal/runtime/prompt_defaults/conductor.md`
  plus any tests/docs that assert or describe bare URL routing;
- source-intake policy that classifies ordinary text URLs, YouTube URLs,
  PDF/EPUB URLs, uploads, and podcast-like links without routing ordinary web
  pages to BrowserApp;
- YouTube transcript import adapter or integration, selected after evidence,
  that creates a durable `ContentItem` or source item with timestamp selectors;
- VText transclusion metadata and visible source markers for transcript
  segments;
- researcher/source-tool access to the transcript artifact and VText
  commentary;
- source/media opening behavior where Source Viewer/reader artifacts are first
  for source text, media apps own playback, PDF/EPUB apps own full-document
  reading, and Web Lens is explicit live/original inspection.

The target behavior is a policy envelope, not a hardcoded prescribed workflow:
VText may ask researcher, ask source tools, wait for more evidence, or simply
create a review scaffold according to owner intent and available evidence.

invariants / qualities / domain ramp (I/Q/D): do not make Conductor the
semantic workflow babysitter; do not force VText to call researcher merely
because a URL exists; do not author canonical VText v1 from Conductor; do not
delete or break source import, ContentItem, Source Viewer, reader artifacts,
PDF/EPUB/media apps, Web Lens, iframe fallback, or publication-carried reader
snapshots without replacement proof. Source text and transcripts are evidence,
not instructions. Preserve provenance, raw/cleaned hashes where available,
canonical URL, platform/video/episode IDs, transcript language, segment
timestamps, extraction caveats, and access policy. Start with prompt/default
text and focused conductor routing tests; then prove one YouTube transcript
transclusion; then broaden to text/PDF/EPUB/uploads; then research podcast
transcript discovery and speech-to-text fallback.

variant (ranking function) V: stale browser-routing prompt clauses + tests that
expect BrowserApp default ownership for ordinary URLs + missing source-intake
classification for text/YouTube/PDF/EPUB/upload/podcast inputs + missing
YouTube transcript source artifact path + missing VText transcript transclusion
selectors + unresolved prompt filename/loader classification.

budget: one focused problem-first planning/doc pass now. The next behavior
mission should choose one code-bearing slice. If prompt extension migration
(`.md` to `.prompt` or `.prompt.md`) is larger than a mechanical rename with
tests, split it into a successor mission.

authority / bounds: behavior-bearing runtime prompt/source mission. Problem
documentation exists in this paradoc before code changes. Do not change broad
app architecture in this mission unless the first source-intake slice proves a
narrow registry/quarantine change is required.

mutation class / protected surfaces: yellow/orange. Protected surfaces include
Conductor top-level routing, VText ownership of canonical document versions,
researcher/source evidence contracts, ContentItem/source-service artifacts,
Source Viewer/reader artifacts, YouTube/video and media app opening, PDF/EPUB
apps, Web Lens, publication/export source policy, and prompt-default loading.

evidence packet: changed prompt/default files, focused routing tests, source
artifact records for a YouTube transcript, VText revision/source metadata,
researcher/source-tool access evidence, grep detector deltas for
BrowserApp/source-routing phrases, staging deploy/acceptance evidence if
runtime behavior changes, rollback refs, heresy delta, residual risks.

heresy delta:

- discovered: stale Conductor bare-URL route to `browser`; runtime prompts named
  `.md` can be mistaken for ordinary docs during doctrine sweeps; the source
  intake product need is broader than Conductor URL routing.
- introduced: none allowed.
- repaired: only count repair when prompt/default behavior and tests no longer
  normalize BrowserApp as source intake, or when surviving names are explicitly
  quarantined as compatibility implementation details. Count YouTube/product
  progress separately from H029 repair.

position / live conjectures / open edges:

- C1 active: the smallest H029 fix is prompt/default wording plus focused tests,
  but the first user-valuable product proof is YouTube transcript transclusion.
- C2 active: Conductor should route source-like input into a durable
  source/VText envelope but must not prescribe that VText always uses researcher
  or always builds a particular review workflow.
- C3 active: YouTube transcript import should produce timestamp-addressable
  source artifacts; a plain prose paste into VText is not enough.
- C4 active: podcast support needs a separate research step for transcript
  discovery and speech-to-text provider choice; "Whisper" is not yet a settled
  architecture claim.
- C5 active: PDF/EPUB links and uploads should extract text into source
  artifacts before VText transclusion, while keeping full reader apps available.
- C6 active: prompt defaults are code-bearing policy; extension migration may
  reduce future confusion but is likely a separate loader/test migration.
- Edge: media-specific content references may still need non-VText display
  routing; classify by source/document intent rather than by the old Browser
  app ontology.
- Edge: broader source search over ingested news/private corpora depends on the
  larger source architecture and should not be smuggled into the first YouTube
  slice.

next move: before code changes, inspect Conductor routing tests, prompt-default
loader assumptions, existing source import/ContentItem APIs, YouTube/media
source refs, PDF/EPUB extraction paths, and app registry names that mention
BrowserApp. Choose the smallest behavior slice: either H029 prompt route repair
alone, or prompt route repair plus one YouTube transcript-to-VText source
artifact proof if the existing source substrate can carry it cleanly.

ledger file: `docs/mission-conductor-url-source-routing-h029-v0.ledger.md`.

version / lineage: split from the 2026-06-13 doctrine conformance sweep after
H029 prompt residue was identified, then expanded into the source-intake product
wedge after owner clarification.

learning state: this paradoc is the problem-first checkpoint. Promote any
general prompt-file naming decision, YouTube transcript adapter choice,
podcast/speech-to-text policy, or file/PDF/EPUB extraction invariant outward
only after code and product evidence.

settlement: not claimed. Settlement for the first code-bearing slice requires
the stale prompt/default route to be repaired with focused evidence, no
introduced H027-H029 regressions, and staging proof if runtime routing behavior
changes. Settlement for the product wedge additionally requires a real YouTube
URL to become a durable transcript source transcluded into VText with timestamp
selectors and researcher-readable evidence.
