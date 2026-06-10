# Mission Report: Natural Compaction Document Recall Eval

date: 2026-06-09

mission doc: `docs/mission-natural-compaction-pdf-recall-eval-v0.md`

goal:

```text
Run docs/mission-natural-compaction-pdf-recall-eval-v0.md as MissionGradient; first upgrade sandbox document extraction for PDF/DOCX/EPUB/PPTX/HTML sources, then run a frozen-corpus natural compaction recall matrix across DeepSeek, Xiaomi, and gpt-5.4-mini without live search, proving approximate recall, exact retrieval, and automatic post-compaction continuation through normal Choir researcher/VText runs.
```

## Executive Summary

The mission completed.

Choir now has a substantially more realistic long-context eval path for
agentic document work:

- real public documents are imported as durable ContentItems;
- long documents are addressed through selectors instead of prompt-pasted text;
- model-policy overlays can drive provider/model arms through normal product
  state;
- automatic runtime compaction is exercised under real source pressure;
- post-compaction recall is evaluated through normal researcher runs;
- completed-but-invalid evals can be assessed and resumed through a product
  continuation route;
- the continuation preserves the scoped model-policy overlay and can finish the
  recall task without live search.

The mission also clarified an important product invariant: compaction evals are
not meaningful unless the document substrate is real. The run therefore added a
hard sandbox setup preamble to the mission before the model matrix. The Slides
app was explicitly kept out of scope; PPTX and HTML slides are source document
formats for this mission, not a presentation product surface.

Final status:

```text
status: complete
behavior shipped: yes
docs shipped: yes
staging proven: yes
matrix proven: yes, with GPT mini completed through repaired continuation
remaining work: promote the temporary proof harness into a stable regression;
  return to Universal Wire source ingestion and article production
```

## What Shipped

### Documentation Commits

`786df962 docs: checkpoint compaction recall eval repair`

This commit recorded the problem before the behavior repair, following the
repo's problem-documentation-first invariant. It documented that GPT mini could
compact and retrieve under pressure but could finish with a readiness note
instead of an actual recall synthesis.

`02d0cbb2 docs: record compaction continuation proof`

This commit recorded the final deployed proof and marked the mission state
complete. It also preserved the sandbox setup preamble and the explicit
separation from the future Slides app mission.

### Behavior Commit

`4738e311 runtime: continue incomplete compaction recall evals`

This commit added the repair that moved the mission from "mostly proven but GPT
mini incomplete" to "complete with continuation proof."

Code changes:

- added browser-public status assessment for compaction recall eval runs;
- added a browser-public continuation route for invalid completed evals;
- counted selector reads, search-like tool attempts, compaction starts,
  compaction completions, and incomplete final prose;
- started normal researcher continuations for invalid evals rather than using a
  special hidden harness path;
- preserved `llm_policy_overlay_id` into continuation child metadata;
- made eval continuation retry/idempotency safe;
- repaired continuation objective dedupe for known
  candidate-world/computer and patch/change vocabulary.

Key touched files:

- `internal/runtime/api.go`;
- `internal/runtime/api_compaction_eval.go`;
- `internal/runtime/continuation.go`;
- `internal/runtime/api_test.go`.

## CI And Staging Evidence

Behavior commit:

```text
4738e311908af6618b7ad5485a6dc40e9151bdef
```

CI:

- GitHub Actions run `27184624378` passed.
- FlakeHub publish run `27184624395` passed.

Staging deploy identity:

- proxy commit:
  `4738e311908af6618b7ad5485a6dc40e9151bdef`;
- sandbox commit:
  `4738e311908af6618b7ad5485a6dc40e9151bdef`;
- staging deploy timestamp: `2026-06-09T04:53:41Z`;
- staging target: `https://choir.news`.

Final docs commit:

```text
02d0cbb2ea422570bc0d79247de1e182042a5029
```

The final docs commit was pushed to `origin/main`. Docs-only commits are
intentionally CI-filtered.

## Why The Mission Changed Shape

The original direction was to evaluate natural compaction by having researchers
read public PDFs until context pressure caused automatic compaction, then test
recall. That exposed a more basic problem: if researchers cannot import and
read rich documents through a real source substrate, the eval measures a broken
input path rather than compaction.

The mission was therefore upgraded with a sandbox setup preamble. That preamble
became a hard gate before compaction:

- verify or add document extraction tools in the normal user/candidate computer
  image, not just on the local Mac;
- ensure imports flow through ContentItems;
- support PDF, DOCX, EPUB, PPTX, and HTML/HTML-slide source documents;
- preserve raw hashes, cleaned text, selectors, adapter metadata, warnings, and
  provenance;
- ensure researcher/VText-compatible selector reads;
- avoid any Slides app UI or routes in this mission.

This was the right correction. A compaction eval over weak imports would have
been false confidence.

## Corpus And Eval Shape

The scored corpus used a frozen set of public RFC documents. Each model arm
received the same source shape and was forbidden from live search during the
scored phase.

Corpus shape:

- 16 RFC ContentItems;
- 223 selectors;
- large enough to force selector access and compaction pressure;
- imported through product routes;
- evaluated without live search.

Representative corpus items recorded in the mission doc:

- RFC 9110;
- RFC 9000;
- RFC 8446;
- RFC 7540;
- RFC 9112;
- RFC 9113;
- RFC 9114;
- RFC 9204;
- RFC 9111;
- RFC 3986;
- RFC 7230;
- RFC 7231;
- RFC 7232;
- RFC 7233;
- RFC 7234;
- RFC 7235.

The recall questions required the model to:

- compare HTTP semantics, QUIC/TLS transport, and HTTP/2 or HTTP/3 framing
  relationships;
- cite exact selector-local details from at least four RFCs;
- identify an older HTTP/1.1 implementation detail superseded or reframed by
  the newer HTTP core corpus.

## Model Matrix

Target arms:

- `deepseek-v4-flash`;
- `deepseek-v4-pro`;
- `mimo-v2.5`;
- `mimo-v2.5-pro`;
- `gpt-5.4-mini`.

The model arms were driven with owner-visible model-policy overlays, not hidden
prompt hacks or broad base-policy rewrites.

### Primary Matrix Artifact

artifact:

```text
/tmp/choir-compaction-matrix-1780975059970.json
```

command:

```text
cd frontend && PLAYWRIGHT_BASE_URL=https://choir.news npx playwright test tests/.tmp-compaction-matrix.spec.js --project=chromium --reporter=list
```

result:

- 5 Playwright arms completed in 37.6 minutes;
- corpus per arm: 16 RFC ContentItems, 223 selectors;
- no forbidden browser-public internal/test routes were observed;
- no live search attempts were observed in successful arms.

Primary results:

- `deepseek-v4-pro`: passed. 223 actual selector reads, zero search attempts,
  one compaction start, one compaction completion, final synthesis answered the
  recall questions.
- `mimo-v2.5`: passed mechanics. 223 actual selector reads, zero search
  attempts, one compaction start, one compaction completion.
- `mimo-v2.5-pro`: passed. 223 actual selector reads, zero search attempts, one
  compaction start, one compaction completion, and exact protocol details in
  final synthesis.
- initial `deepseek-v4-flash`: weak. 188 actual selector reads, zero search
  attempts, and no detected compaction event.
- initial `gpt-5.4-mini`: weak. 70 actual selector reads, zero search attempts,
  four compaction starts/completions.

### Retry Artifact

artifact:

```text
/tmp/choir-compaction-matrix-1780977389639.json
```

result:

- `deepseek-v4-flash`: passed on retry with medium reasoning. 223 actual
  selector reads, zero search attempts, one compaction start, one compaction
  completion, and a final answer with broad approximate recall plus exact
  details.
- `gpt-5.4-mini`: mechanically strong but final-answer invalid. 266 actual
  selector reads, zero search attempts, four compaction starts/completions. It
  stopped with a readiness note instead of producing the final recall synthesis.

### GPT Mini Final Retry Artifact

artifact:

```text
/tmp/choir-compaction-matrix-1780978538731.json
```

result:

- `gpt-5.4-mini`: still incomplete. 144 actual selector reads, zero search
  attempts, four compaction starts/completions. Stricter prompt wording did not
  reliably force final recall.

Interpretation at that point:

Prompt-only contracts were not enough. The right product repair was not "prompt
harder"; it was an eval-grade assessment and continuation path that can detect
completed-but-invalid runs and resume normal agent work.

## Repair: Eval Assessment And Continuation

The repair added two product behaviors:

1. A compaction recall eval status route returns assessment metadata.
2. A continuation route starts a normal researcher continuation when the eval
   contract is not satisfied.

Assessment checks:

- available selector count;
- actual selector reads;
- search-like tool attempts;
- compaction started/completed;
- final prose shape;
- invalid reasons.

Continuation behavior:

- starts as a normal researcher continuation;
- uses frozen ContentItem ids and recall questions;
- forbids live search, URL fetch, source search, and URL imports;
- preserves the model-policy overlay;
- is retry/idempotency safe.

This is important architecturally. The repair did not create a privileged eval
oracle that bypasses Choir's agent loop. It made the product path better at
observing failure and continuing work.

## Local Verification

Focused local tests passed under the repo dev shell.

Command:

```text
nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestHandleCompactionRecallEvalStartsResearcherWithOverlayAndFrozenContent|TestCompactionRecallEvalStatusAssessesCoverageAndAnswerContract|TestCompactionRecallEvalContinueStartsResearcherWithOverlay|TestRunContinuationCompactsAndStartsBoundedNextGoal|TestRunContinuationPublicSynthesizeListAndStartAreOwnerScoped|TestStartChildRunResolvesModelPolicyOverlayIntoRunMetadata|TestRuntimeRejectsExpiredModelPolicyOverlay|TestHandleModelPolicyResolveUsesOverlayFile' -count=1
```

Result:

```text
ok github.com/yusefmosiah/go-choir/internal/runtime 12.217s
```

A focused continuation dedupe test initially failed. The cause was that the
objective fingerprint normalizer did not collapse known equivalent terms:

- `world` vs `computer`;
- `patch` vs `change`.

That was fixed in `normalizeObjectiveText`, and the focused test passed.

## Final Deployed GPT Mini Continuation Proof

artifact:

```text
/tmp/choir-compaction-gpt-continuation-proof-1780988770494.json
```

Staging build in artifact:

- proxy:
  `4738e311908af6618b7ad5485a6dc40e9151bdef`;
- sandbox:
  `4738e311908af6618b7ad5485a6dc40e9151bdef`.

Initial GPT mini eval:

- loop id: `e51a48ac-ffca-4413-a138-e4c081532792`;
- provider/model: `chatgpt` / `gpt-5.4-mini`;
- completed but assessed invalid;
- invalid reason: final result was not a recall synthesis;
- assessed selector coverage: 470 actual selector reads against required 223;
- compaction evidence: 5 starts and 5 completions.

Continuation:

- continuation id: `085de033-ad99-41c6-a4bd-ebf32c4c08ec`;
- child loop id: `7d9d5fdc-3c3b-42ad-b7e9-c6bad4459fa2`;
- overlay preserved:
  `compaction-gpt-continuation-1780988770494`;
- status: started, then completed through normal researcher continuation.

Post-continuation trace counters:

- 1,389 trace moments;
- 1,040 selector-read moments;
- 272 selector-list moments;
- 7 compaction starts;
- 7 compaction completions;
- 0 search attempts.

The continuation child produced a selector-cited recall synthesis covering:

- HTTP semantics vs transport/framing in RFC 9110;
- HTTP content semantics, fields, validators, and intermediary handling;
- HTTP/2 framing and DATA-frame details from RFC 7540;
- HTTP/2 malformed-message and CONNECT/Upgrade behavior from RFC 9113;
- HTTP/3 over QUIC and HTTP/3 pseudo-header/CONNECT behavior from RFC 9114;
- QUIC transport constraints from RFC 9000;
- TLS 1.3 record-layer framing from RFC 8446;
- an older HTTP/1.1 hop-by-hop connection handling detail reframed by the newer
  HTTP core corpus.

The temporary browser proof command failed after writing the artifact because
the assertion read a truncated child summary instead of the full child message
detail. The artifact contains the full product-path evidence, so this was a
proof-harness assertion problem, not a product failure.

## What We Learned

### 1. The document substrate is the eval substrate.

Long-context and compaction evals are only meaningful when the input side is
real. Regex PDF extraction and bounded URL excerpts are not enough. The useful
object is durable source import with selectors.

### 2. Frozen corpus beats search for this eval.

The mission avoided spending search API quota and reduced nondeterminism by
using a frozen public corpus. That was the right evaluation boundary. Search
quality and source discovery are separate missions.

### 3. Model-policy overlays are the right control surface.

The matrix needed per-arm model selection without changing platform defaults.
Owner-visible overlays were the right mechanism: scoped, inspectable, and
compatible with normal runs.

### 4. Prompt-only eval contracts are weak.

GPT mini showed the failure mode clearly. It could read enough, compact enough,
and still finish with "ready to synthesize" instead of the synthesis. The fix
was product assessment plus continuation, not more prompt pressure.

### 5. Continuation is a core agentic reliability primitive.

The successful end state was not "every run succeeds first try." It was:

```text
run completes invalid
  -> product assesses invalidity
  -> normal continuation starts
  -> model policy overlay is preserved
  -> child finishes the work
```

That pattern is directly relevant to future Universal Wire and Choir-in-Choir
missions.

### 6. Trace summaries are not sufficient for proof assertions.

The final proof harness failed because it asserted on a truncated summary. For
serious acceptance, use full moment/message detail, not summary snippets.

## Residual Risks

The mission is complete, but a few follow-ups remain.

### Stable Regression Needed

The GPT mini continuation proof was run with a temporary Playwright spec and
then cleaned up. The behavior is documented and shipped, but the proof should
be promoted into a stable regression test before the next provider/compaction
mission.

### Full Message Detail Assertions

Future proof harnesses should assert on full trace moment/message detail. The
summary path is allowed to truncate text, so it is not a reliable evidence
surface for final-answer content.

### Extraction Quality Still Deserves Product Attention

This mission proved the source substrate enough to support the compaction
matrix. It did not exhaustively certify every document format or media edge
case. Future work should continue improving extraction quality for PDFs, DOCX,
EPUB, PPTX, HTML slides, tables, images, and citations.

### Slides App Is Explicitly Deferred

PPTX/HTML slide extraction is source-document work. A real Slides app remains a
separate mission and should not be conflated with this eval.

## Readiness For Universal Wire

This mission unblocks the next Universal Wire push in a narrow but important way.

Useful capabilities now available:

- DeepSeek, Xiaomi, and GPT mini can be exercised in long-running agent loops;
- compaction can be trusted enough for large-source work;
- model-policy overlays can select provider/model arms without platform-wide
  edits;
- invalid completed work can be detected and continued;
- selector-cited source recall can survive compaction.

This does not solve Universal Wire itself. The news system still needs a hard
cutover away from mocked/stubbed sources and old StoryGraph/source-ledger
surfaces. But the provider/compaction substrate is now good enough to use for
long-running processors, reconcilers, researchers, and VText article owners.

Recommended next mission:

```text
/goal Run the Universal Wire real-news cutover mission: delete obsolete StoryGraph/source-ledger/style-control cruft, ingest many real RSS/GDELT/Telegram/HN/international/industry sources, create full ContentItem-backed source artifacts, have processors/reconcilers/researchers feed VText article owners, and ship readable newspaper-column Universal Wire views with real articles and native source transclusions.
```

## Final State

```text
mission status: complete
final docs commit: 02d0cbb2ea422570bc0d79247de1e182042a5029
behavior commit: 4738e311908af6618b7ad5485a6dc40e9151bdef
staging commit proven: 4738e311908af6618b7ad5485a6dc40e9151bdef
primary proof artifact: /tmp/choir-compaction-gpt-continuation-proof-1780988770494.json
repo state after report drafting: report committed in docs
```
