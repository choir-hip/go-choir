# VText Style System Research - 2026-06-06

**Status:** research synthesis / product architecture input  
**Scope:** `Style.vtext` support for VText, client corpus ingestion, learned writing
style, edit feedback, and future fine-tuning paths  

## Decision Frame

Writing style is a top-priority VText capability. The product target is not
"make the model sound vaguely like the client." The target is:

```text
client corpus
-> extracted Style.vtext
-> contextual style profile
-> exemplar-guided generation + revision
-> edit/annotation feedback as style memory
-> evolving Style.vtext revisions
-> advisory style review and drift detection
-> optional fine-tuning/adapters only when justified
```

Styles should themselves be VTexts. A `Style.vtext` should be versioned, citeable,
editable, contextual, and learn over time from corpus additions, human edits,
explicit notes, accepted/rejected rewrites, and reviewable style observations.

Keep the system agentic. `Style.vtext` should orient VText agents, preserve
evidence, and make style memory inspectable; it should not turn writing into a
brittle deterministic schema where the agent optimizes a checklist instead of
serving the document.

Naming matters. Choir should call this **style**, not taste. Taste is mostly
selection: choosing good things from an existing field. It can often be bought,
outsourced, or imported. Style is expression: how a person, firm, publication,
or institution transforms choices into a recognizable way of thinking and
being. It has to fit the writer's body of work, constraints, audience, stakes,
habits, and judgment. If it is pasted on, it reads as costume.

This makes `Style.vtext` more than preference memory. It is an authored asset
and eventually a kind of IP. A person or client can own multiple styles; styles
can be private, shared, licensed, published, distributed, or composed. The
primary goal is not to acquire someone else's style, but to preserve and
amplify the user's own style under computation.

The v0 should not require weight fine-tuning. Fine-tuning is a later accelerator
or specialized deployment path, not the foundation.

There are two equal sides to the problem:

1. **Positive style preservation:** learn a client's/person's contextual voice
   from prior work, explicit notes, and edit history.
2. **Negative style protection:** prevent model-generic tics from degrading the
   client's writing into average AI-polished prose.

The objective is not to deceive AI detectors or pass AI writing as human. The
objective is to let clients apply more compute without losing the specificity,
judgment, rhythm, and authority that made their writing valuable in the first
place.

## External Patterns Worth Learning From

### 1) Exemplar selection before style extraction

Google Marketing Solutions' Copycat learns brand style from existing search
ads, reduces the training set to diverse exemplars with affinity propagation,
generates a style guide from those exemplars, then uses both the style guide
and relevant examples at generation time. It also checks memorization and style
similarity.

Choir implication:

- do not stuff a whole corpus into a prompt;
- cluster the corpus by context, genre, audience, and rhetorical mode;
- select high-signal exemplars per context;
- generate `Style.vtext` from exemplars plus optional existing brand docs;
- keep exemplars available for retrieval during generation;
- detect memorization or over-copying.

Source: [google-marketing-solutions/copycat](https://github.com/google-marketing-solutions/copycat)

### 2) Feedback compounds into style memory

Margin is a local Markdown reader that lets users mark corrections and voice
signals, tag them by writing type, synthesize style notes, and export a writing
profile. Its key product insight is that annotation has to be ergonomic because
style memory depends on repeated correction.

Choir implication:

- VText edit history is not only document history; it is style training signal;
- user edits should be mined into candidate style observations;
- feedback needs polarity: "do more of this" and "avoid this";
- style notes must be scoped by context such as email, proposal, legal memo,
  blog, client update, research note, or autoradio script;
- every durable style claim should retain provenance back to examples/edits.

Sources: [Margin site](https://marginreader.app/), [SZoloth/margin](https://github.com/SZoloth/margin)

### 3) Brand discovery is a corpus governance problem

Tribe AI's Brand Voice plugin frames brand style as scattered across Notion,
Confluence, Drive, Slack, Gong, meeting notes, decks, and old guidelines. It
discovers signals, generates LLM-ready guidelines, enforces them during content
creation, and surfaces ambiguity as open questions.

Choir implication:

- client style ingestion should accept heterogeneous corpora, not only finished
  polished writing;
- corpus items need source class, recency, authority, author, audience, and
  quality labels;
- "official style guide says X but actual successful writing does Y" is a first
  class conflict, not an error to hide;
- unresolved style conflicts should become questions in `Style.vtext`.

Source: [TribeAI/claude-cowork-brand-voice-plugin](https://github.com/TribeAI/claude-cowork-brand-voice-plugin)

### 4) Prose linting is useful feedback, not the writing brain

Vale is a markup-aware prose linter with configurable rule systems. VectorLint
shows the LLM-era version: style guide rules as Markdown/YAML, model-generated
candidate violations, deterministic filtering, confidence gates, and quality
scores.

Choir implication:

- `Style.vtext` can emit lint/eval feedback, but lint should not own the writing
  process;
- some feedback can be deterministic (`banned_phrases`, punctuation, heading
  case);
- some need LLM judgment (`too breathless`, `not client-safe`, `too generic`);
- style feedback should produce an inspectable note, not just a regenerated
  draft;
- VText should support "revise with this style note in mind" and "explain style
  deltas" without forcing every document through a brittle scoring harness.

Sources: [vale-cli/vale](https://github.com/vale-cli/vale), [TRocket-Labs/vectorlint](https://github.com/TRocket-Labs/vectorlint)

### 5) Stylometry gives measurable fingerprints

The `stylometric-transfer` project extracts quantitative style fingerprints
from a corpus: sentence length, punctuation, paragraph rhythm, lexicon,
preferred/avoided words, templates, controls, validators, and derived prompts.
It then applies the fingerprint and reports deviation.

Choir implication:

- `Style.vtext` should not be only prose advice;
- store a structured style fingerprint beside the human-readable guide;
- include measured distributions, target ranges, hard/soft avoids, preferred
  patterns, and validators;
- use deviation reports to explain why a draft feels off.

Source: [ngpepin/stylometric-transfer](https://github.com/ngpepin/stylometric-transfer)

### 6) Few-shot examples often matter more than abstract summaries

The 2025 "How Well Do LLMs Imitate Human Writing Style?" preprint reports that
prompting strategy strongly affects style fidelity: few-shot prompting performs
much better than zero-shot, and completion-style prompting can match measured
style well, though human-like unpredictability remains separate.

Choir implication:

- an extracted `Style.vtext` alone is insufficient;
- generation should retrieve a small number of context-matched exemplars;
- examples should include positive and negative pairs where possible;
- style evaluation should separate "matches surface style" from "sounds alive."

Sources: [Hugging Face paper page](https://huggingface.co/papers/2509.24930), [arXiv 2509.24930](https://arxiv.org/abs/2509.24930)

### 7) Fine-tuning helps, but does not remove the need for artifacts

Research and hobbyist work spans LoRA/PEFT, inverse transfer data augmentation,
GRPO with a style classifier reward, and enterprise hybrid systems. ITDA
generates neutral/stylized pairs by stripping style from target texts and
training on the resulting pairs. A Hugging Face style-transfer writeup trains a
classifier and uses GRPO to push a small model toward a historical periodical
style. Amazon's Onoma paper describes a hybrid enterprise-scale style transfer
system combining fine-tuned LLMs with structure-aware generation for technical
documentation.

Choir implication:

- fine-tuning can be a later mode for high-volume clients or private
  deployments;
- even with fine-tuning, keep `Style.vtext`, exemplars, validators, and
  edit feedback as control artifacts;
- for client work, LoRA/adapters may be tenant-specific and policy-sensitive;
- fine-tuned style can overfit, memorize, or preserve bad corpus habits unless
  corpus curation and eval remain explicit.

Sources: [ITDA paper](https://www.sciencedirect.com/science/article/pii/S2666651024000135), [Penny 1.7B style-transfer writeup](https://huggingface.co/blog/dleemiller/penny-1-7b-style-transfer), [Amazon Science Onoma paper page](https://www.amazon.science/publications/moving-beyond-the-style-guide-enterprise-scale-style-transfer), [stylellm/stylellm_models](https://github.com/stylellm/stylellm_models)

### 8) Human-readable style guides must be LLM-ready

Brand-voice guidance for LLMs repeatedly converges on explicit, example-heavy
documents: concrete rules, forbidden phrases, tone matrices, templates, and
positive/negative examples. Gwern's manual emphasizes few-shots as portable
model-independent training data and as a unit-test-like surface for style.

Choir implication:

- traditional client PDF style guides should be converted into LLM-ready VTexts;
- `Style.vtext` artifacts should have both human prose and machine sections;
- each rule should say where it applies, why it exists, examples, counterexamples,
  severity, and provenance;
- few-shots are first-class style assets, not prompt decoration.

Sources: [Search Engine Land brand voice guide](https://searchengineland.com/guide/how-to-train-in-house-llms-on-brand-voice), [Gwern Manual of Style](https://gwern.net/style-guide)

### 9) AI-writing tics are real, but tic lists are not enough

Long-tail blogs, Reddit threads, and recent papers converge on a recognizable
set of AI-writing tells: overused words such as "delve," "tapestry," "nuanced,"
and "pivotal"; constructions such as "it is not X, it is Y"; excessive
symmetry, throat-clearing, tidy numbered lists, generic transitions, and
overuse of em dashes. The useful point is not that any one token proves AI
authorship. The useful point is that model outputs often overuse certain
patterns and produce an averaged, polished, low-risk voice.

Choir implication:

- `Style.vtext` needs a negative profile as well as a positive profile;
- tic detection should be model- and context-specific, not a universal banned
  word list;
- if the client naturally uses em dashes, legal triads, or contrast structures,
  preserve them rather than blindly removing them;
- the check should ask "does this sound less like the client and more like the
  model's house style?"

Sources: [GenIntelSys AI tells blog](https://www.genintelsys.com/blog/em-dashes-ai-tells/), [The Rise of Verbal Tics in Large Language Models](https://arxiv.org/abs/2604.19139), [Why Does ChatGPT "Delve" So Much?](https://arxiv.org/abs/2412.11385), [TechRadar on em dashes](https://www.techradar.com/computing/artificial-intelligence/did-chatgpt-ruin-the-em-dash-heres-how-to-stop-it-putting-them-everywhere)

### 10) Detectors are diagnostic instruments, not product goals

Pangram Labs offers AI-generated text detection and plagiarism APIs, and recent
public discussion treats Pangram as one of the stronger commercial detectors.
At the same time, detector discourse carries serious false-positive and
misuse risk. Recent research also suggests detectors may distinguish
instruction-tuned "assistant voice" from other generation regimes more than
they detect artificiality in an essential sense.

Choir implication:

- detector scores can be one weak signal in a style degradation report;
- do not optimize for "pass Pangram" as a product goal;
- use detector-like feedback to find assistant voice leakage, generic
  smoothness, and suspicious uniformity;
- always pair detector-style metrics with client-style similarity,
  semantic-preservation checks, and human review.

Sources: [Pangram documentation](https://docs.pangram.com/), [Pangram API page](https://www.pangram.com/solutions/api), [Atlantic on Pangram and detector risk](https://www.theatlantic.com/technology/2026/05/pangram-ai-detection-accuracy/687381/), [Base Models Look Human To AI Detectors](https://arxiv.org/abs/2605.19516)

### 11) Voice preservation fails when editing is destructive

Recent writing-tool blogs and the "Voice Under Revision" paper point to the
same failure mode: AI editors normalize text. Generic "improve this" or
"rewrite this" prompts tend to reduce idiosyncrasy, smooth rhythm, make
vocabulary more generic, and weaken the writer's ownership. Even explicit
"preserve voice" prompts reduce the damage but do not fully prevent the drift.

Choir implication:

- VText editing should prefer tracked, local, reviewable edits over
  whole-document rewrites;
- style preservation should be measured as "how much distinctive signal
  survived," not only "is it grammatically cleaner";
- the app should offer edit modes: proofread, tighten, clarify, restructure,
  ghostwrite, translate, client-safe, court-style, partner-style;
- each mode needs a different permission to alter voice.

Sources: [Voice Under Revision](https://arxiv.org/abs/2604.22142), [Orwellix on voice-preserving editors](https://orwellix.com/blog/posts/writing-with-ai/best-ai-writing-tool-that-doesnt-change-your-voice), [Taim on preserving voice](https://www.taim.io/ai-productivity/using-ai-to-edit-your-own-writing), [Revise on AI-edited nuance](https://www.revise.net/blog/nuance-matters-ai-edited-text), [TechRadar on AI changing communication](https://www.techradar.com/ai-platforms-assistants/chatgpt/chatgpt-is-changing-the-way-we-communicate-heres-how-you-can-avoid-speaking-like-ai-in-public)

## Proposed Style.vtext Object

```text
Style.vtext
  identity:
    style_id
    owner_id / tenant_id
    name
    context_scope
    status
    version
    authorship / ownership
    distribution_policy

  corpus_manifest:
    corpus_items[]
      content_ref
      source_type
      author
      audience
      channel
      date
      quality_label
      authority_weight
      privacy_policy

  human_guide:
    voice principles
    tone matrix
    structure preferences
    diction and terminology
    rhythm and sentence shape
    examples / anti-examples
    explicit open questions

  style_profile:
    style_fingerprint
    anti_model_tic_profile
    model_specific_tic_lists
    voice_preservation_notes
    measured_distributions
    preferred_patterns
    avoided_patterns
    reusable rhetorical moves
    exemplar retrieval notes
    style review notes
    detector_diagnostic_policy

  style_memory:
    accepted_edits
    rejected_edits
    explicit_notes
    positive_voice_signals
    corrective_voice_signals
    style_observations
    unresolved_conflicts
    drift_reports
    model_tic_reports
    voice_degradation_reports

  provenance:
    examples supporting each style observation
    edit refs supporting each learned observation
    user/client approvals
    superseded observations
    source/corpus rights
    publication/distribution terms
```

Because it is a VText, `Style.vtext` can be edited, compared, branched, cited,
published privately, shared with a client, licensed, distributed, and applied by
VText appagents.

## Generation Pipeline

```text
user/client request
-> choose context Style.vtext artifact(s)
-> retrieve style observations + measured profile + 3-8 exemplars
-> retrieve anti-model-tic profile for current model/provider
-> draft with style orientation
-> ask a style reviewer/evaluator agent for voice-preservation notes when useful
-> revise or produce style report
-> user edits/notes
-> mine feedback into candidate style observations
-> propose a Style.vtext revision
```

Important behavior:

- use `Style.vtext` and exemplars at generation time;
- do not force every observation into every prompt; choose a context-specific style
  packet;
- preserve semantic requirements over style imitation;
- preserve authorial distinctiveness over generic polish;
- avoid model tics only when they are not part of the client's real style;
- report style deviations explicitly when a task requires them;
- allow multiple active `Style.vtext` artifacts per person/client/context.

## Extraction Pipeline

```text
corpus import
-> classify item context and authority
-> clean and segment
-> cluster by genre/audience/channel
-> compute stylometric metrics
-> select exemplars
-> LLM extracts candidate style observations
-> compare rules against measured signals
-> generate Style.vtext
-> ask open questions for conflicts/gaps
```

Extraction should avoid collapsing a person into one average style. A lawyer's
client update, motion draft, internal research memo, investor note, and personal
blog post may need separate but related `Style.vtext` artifacts.

## Learning From Edits

VText already has document/revision/edit history, so style learning should use
the existing artifact surface rather than a separate black box.

Signals:

- diff from AI draft to accepted human revision;
- explicit comments such as "too salesy" or "this is the right tone";
- repeated replacements (`utilize` -> `use`);
- structural edits such as shorter paragraphs or removed preambles;
- accepted generated drafts with minimal edits;
- rejected drafts and their failure labels.

Style observation lifecycle:

```text
candidate observation
-> evidence count / examples
-> context scope
-> confidence
-> human approval or agent confidence note
-> active style note
-> drift reports
-> superseded / retired
```

Do not silently mutate a client's `Style.vtext` from one edit. Treat style learning
as evidence accumulation with reviewable, revisable observations.

## Evaluation

Style support needs feedback, not vibes. Feedback can include measurements, but
measurements are advisory evidence for the VText agent and user, not the final
arbiter of whether the writing works.

Evaluation layers:

- optional deterministic checks: banned phrases, heading case, spelling variant,
  terminology, citation placement;
- stylometric distance: sentence length, punctuation, paragraph rhythm,
  vocabulary, n-gram fingerprint;
- LLM rubric: tone, audience fit, structure, brand/personality match;
- anti-model-tic check: formulaic structures, assistant voice leakage, generic
  transitions, over-polishing, repeated model-specific phrases;
- exemplar similarity: close enough to style, not memorized;
- edit distance after human revision: how much the client had to change;
- voice-preservation delta: what distinctive signals were removed by the edit;
- detector diagnostics: optional weak signal, never the optimization target;
- user satisfaction: explicit accept/reject and "save this as a style note";
- drift: whether style gets generic over repeated generations.

The style report should be VText-visible and should name evidence, uncertainty,
voice-preservation concerns, model-tic concerns, and recommended next edits.

## Strategy Range

### V0: Style.vtext + retrieval exemplars

Best first move. It can be implemented with current VText concepts.

- import corpus;
- generate `Style.vtext`;
- retrieve exemplars;
- apply style packet in VText revision prompts;
- collect edits/notes.

### V1: Style feedback and reports

Use `Style.vtext` artifacts to produce feedback. Some feedback can come from deterministic
checks, but the default shape should be an LLM/stylometric review note that
helps the next VText agent write better. This gives clients visible progress
without making lint the writer.

Add an anti-model-tic report in this phase. It should identify assistant voice
leakage and likely model-specific tics, but it should explain when a flagged
pattern is actually part of the client's known style.

### V2: Non-destructive editing modes

Before aggressive rewriting, add edit modes that preserve voice by default:

- proofread only;
- tighten without changing diction;
- clarify while preserving cadence;
- restructure but preserve signature phrasing where possible;
- rewrite in selected `Style.vtext` voice;
- deliberately depart from style for a named purpose.

Every mode should report what it changed and what it protected.

### V3: Continuous style memory from VText edits

Mine accepted edits into candidate style observations. Keep provenance and make
the agent explain why the observation matters before it becomes durable style
memory. This is the compounding loop.

### V4: Contextual style router

Select style(s) by document type, audience, client/matter, channel, and
privacy policy. Support composition: "client brand voice + legal memo
discipline + partner's personal preferences."

### V5: Tenant/private fine-tuning

Use LoRA/adapters or private model policy only after the artifact loop proves
high-volume value and has enough curated data. Keep `Style.vtext` as the
control artifact.

## Risks

- **Generic AI voice with client vocabulary:** the system may learn words but
  miss rhythm, structure, and judgment.
- **Model-tic laundering:** the system may make a good writer sound like the
  current default assistant voice while preserving surface vocabulary.
- **Overfitting/memorization:** especially with small corpora or fine-tuning.
- **Context collapse:** one average guide for all situations will be worse than
  several scoped guides.
- **Bad corpus pollution:** old, low-quality, or off-brand materials can become
  false style.
- **Silent learning:** updating guides without review can surprise clients.
- **Style over substance:** style transfer must not weaken factuality,
  citations, legal accuracy, or client confidentiality.
- **Authorship/privacy concerns:** personal style may be identifying and should
  be scoped by consent and policy.
- **Detector Goodharting:** optimizing against Pangram, GPTZero, or any detector
  can become deception-oriented and may harm writing quality. Use detectors as
  diagnostics only.

## Recommended Choir Direction

Build the VText-native artifact loop first:

1. Add a `Style.vtext` artifact contract.
2. Add corpus import/classification for style examples.
3. Generate a first `Style.vtext` from a corpus with cited examples.
4. Apply it in VText generation with retrieved exemplars.
5. Add style review notes, optional lint diagnostics, voice-preservation
   checks, and anti-model-tic reports.
6. Add non-destructive editing modes before whole-document rewrite.
7. Mine VText edits into candidate style observations.
8. Defer fine-tuning until a client has enough curated examples, clear privacy
   boundaries, and measurable style-report failures that prompting/retrieval
   cannot fix.

The key product insight: `Style.vtext` is not a prompt, anti-AI style is not a
detector-evasion prompt, and style feedback is not the writing brain. The
system is a living VText artifact and workflow that protects client
distinctiveness while applying compute.

## Source Map For Deeper Review

This section is intentionally a source map, not a requirements list.

Core styleguide / voice-profile systems:

- Google Marketing Solutions Copycat:
  https://github.com/google-marketing-solutions/copycat
- Google Developers Blog on MarTech generative AI:
  https://developers.googleblog.com/google-martech-solutions-putting-generative-ai-in-marketing/
- Margin Reader:
  https://marginreader.app/
- SZoloth/margin:
  https://github.com/SZoloth/margin
- TribeAI Claude Cowork Brand Voice Plugin:
  https://github.com/TribeAI/claude-cowork-brand-voice-plugin
- Anthropic Knowledge Work Plugins:
  https://github.com/anthropics/knowledge-work-plugins
- Houtini Voice Analyser MCP:
  https://houtini.com/generate-a-tone-of-voice-guide-with-voice-analyser-mcp/
  and https://github.com/houtini-ai/voice-analyser-mcp
- Leanpub GhostAI:
  https://github.com/leanpub/ghostai
- stylometric-transfer:
  https://github.com/ngpepin/stylometric-transfer
- cc-prose:
  https://github.com/rhuss/cc-prose
- Claude voice/style tools:
  https://github.com/aplaceforallmystuff/claude-voice-analyzer
  https://github.com/aplaceforallmystuff/claude-voice-editor
  https://github.com/shandley/claude-style-guide
  https://github.com/AutumnsGrove/ClaudeSkills/blob/master/brand-guidelines/examples/style-guide-template.md
- Anti-slop / human voice repos:
  https://github.com/aplaceforallmystuff/the-antislop
  https://github.com/realrossmanngroup/no_ai_slop_writing_rules
  https://github.com/willmather95/human-copy
  https://github.com/TimSimpsonJr/prose-craft
  https://github.com/zircote/human-voice
  https://github.com/yzhao062/agent-style
  https://gist.github.com/MisreadableMind/57e8fdb8fdeaaa5456999b5e8110df7b

Prose linting / executable styleguides:

- Vale:
  https://vale.sh/
  https://vale.sh/docs/styles
  https://vale.sh/library
- Elastic, Datadog, Grafana, and other Vale rule sets/writeups:
  https://github.com/elastic/vale-rules
  https://www.elastic.co/docs/contribute-docs/vale-linter
  https://www.datadoghq.com/blog/engineering/how-we-use-vale-to-improve-our-documentation-editing-process/
  https://github.com/DataDog/datadog-vale
  https://github.com/grafana/writers-toolkit
  https://grafana.com/docs/writers-toolkit/review/lint-prose/rules/
  https://engineering.contentsquare.com/2023/using-vale-to-help-engineers-become-better-writers/
  https://www.spectrocloud.com/blog/how-we-use-vale-to-enforce-better-writing-in-docs-and-beyond
  https://blog.stoplight.io/linting-the-stoplight-docs-with-vale
  https://www.meilisearch.com/blog/prose-linting-with-vale
  https://vaadin.com/docs/latest/contributing/docs/vale
- VectorLint:
  https://github.com/TRocket-Labs/vectorlint/
  https://trocket-labs-vectorlint.mintlify.app/
- Public style guides:
  https://docs.github.com/en/contributing/style-guide-and-content-model/style-guide
  https://learn.microsoft.com/en-us/style-guide/welcome/
  https://developers.google.com/style

Brand voice / practitioner writing:

- Search Engine Land brand voice:
  https://searchengineland.com/guide/how-to-train-in-house-llms-on-brand-voice
- Gwern Manual of Style:
  https://gwern.net/style-guide
- CXL on brand voice erosion:
  https://cxl.com/blog/ai-content-and-the-silent-erosion-of-brand-voice/
- Rob Palmer copywriting / copychief posts:
  https://robpalmer.com/blog/ai-copywriting-tools
  https://robpalmer.com/blog/ai-vs-human-copywriting
  https://robpalmer.com/blog/chatgpt-for-copywriting
  https://robpalmer.com/blog/claude-code-copychief-skill
  https://robpalmer.com/blog/claude-code-copywriting-skills
- UX Content on inference noise and AI content strategy:
  https://uxcontent.com/inference-noise-ai-vs-human-writing/
  https://uxcontent.com/words-data-future-content-design/
  https://uxcontent.com/ai-content-strategy/
- Tone/few-shot references:
  https://bendaviesromano.medium.com/improving-the-tone-of-ai-generated-text-with-few-shot-prompting-1db373cfd0de
  https://www.nngroup.com/articles/tone-of-voice-dimensions/
  https://www.promptingguide.ai/techniques/fewshot
  https://www.prompthub.us/blog/the-few-shot-prompting-guide

Voice-preserving editing / authorship UX:

- Voice Under Revision:
  https://arxiv.org/abs/2604.22142
- Who Owns the Text:
  https://arxiv.org/abs/2601.10236
- GhostWriter:
  https://arxiv.org/abs/2402.08855
- The AI Ghostwriter Effect:
  https://arxiv.org/abs/2303.03283
- "80% me, 20% AI":
  https://arxiv.org/abs/2411.13032
- Voice-preserving writing tools/blogs:
  https://www.taim.io/ai-productivity/using-ai-to-edit-your-own-writing
  https://www.revise.net/
  https://www.revise.net/blog
  https://orwellix.com/
  https://novelhive.ai/blog/ai-novel-editing-author-agent
  https://bubblecow.com/

Style imitation / stylometry / evaluation:

- How Well Do LLMs Imitate Human Writing Style:
  https://arxiv.org/abs/2509.24930
- Catch Me If You Can? Not Yet:
  https://arxiv.org/abs/2509.14543
  https://aclanthology.org/2025.findings-emnlp.532.pdf
- Evaluating Style-Personalized Generation:
  https://arxiv.org/html/2508.06374v1
- LaMP personalization benchmark:
  https://arxiv.org/abs/2304.11406
- Interpretable style embeddings:
  https://arxiv.org/abs/2305.12696
- Prompt-and-Rerank style transfer:
  https://arxiv.org/abs/2205.11503
- Paraphrase-generation style transfer:
  https://arxiv.org/abs/2010.05700
- Out of Style:
  https://arxiv.org/abs/2406.10320
- Can AI writing be salvaged:
  https://arxiv.org/abs/2409.14509
- Authorship impersonation:
  https://arxiv.org/html/2603.29454v1
- Speakerly:
  https://arxiv.org/abs/2310.16251

Fine-tuning / adapters / model-side style transfer:

- StyleAdaptedLM:
  https://arxiv.org/html/2507.18294v1
- ITDA:
  https://www.sciencedirect.com/science/article/pii/S2666651024000135
- Penny 1.7B style transfer:
  https://huggingface.co/blog/dleemiller/penny-1-7b-style-transfer
- Amazon Onoma:
  https://www.amazon.science/publications/moving-beyond-the-style-guide-enterprise-scale-style-transfer
  https://assets.amazon.science/bb/f1/a0a8213a4cd2b31311c8747f87a3/scipub-approval152129-38424505-moving-beyond-the-styleguide-enterprisescale-style-transfer.pdf
- StyleLLM:
  https://github.com/stylellm/stylellm_models
- Hugging Face skills/fine-tuning:
  https://huggingface.co/blog/hf-skills-training
  https://huggingface.co/dleemiller/EMOTRON-3B

Anti-model-tic / detector diagnostics:

- Verbal tics:
  https://arxiv.org/abs/2604.19139
- Delve:
  https://arxiv.org/abs/2412.11385
  https://pmejournal.org/articles/10.5334/pme.1929
- Em dash / AI tell discourse:
  https://www.genintelsys.com/blog/em-dashes-ai-tells/
  https://www.techradar.com/computing/artificial-intelligence/did-chatgpt-ruin-the-em-dash-heres-how-to-stop-it-putting-them-everywhere
- LLM linguistic impact:
  https://www.theverge.com/openai/686748/chatgpt-linguistic-impact-common-word-usage
  https://www.scientificamerican.com/article/chatgpt-is-changing-the-words-we-use-in-conversation/
  https://www.techradar.com/ai-platforms-assistants/chatgpt/chatgpt-is-changing-the-way-we-communicate-heres-how-you-can-avoid-speaking-like-ai-in-public
- Antislop:
  https://arxiv.org/abs/2510.15061
  https://github.com/sam-paech/auto-antislop
- Detector diagnostics:
  https://www.pangram.com/solutions/api
  https://pangram.readthedocs.io/en/latest/api/rest.html
  https://arxiv.org/html/2402.14873v3
  https://www.theatlantic.com/technology/2026/05/pangram-ai-detection-accuracy/687381/
  https://timrequarth.substack.com/p/why-you-shouldnt-trust-ai-detector
  https://arxiv.org/abs/2603.23146
  https://arxiv.org/abs/2605.19516
  https://gptzero.me/
  https://originality.ai/
  https://copyleaks.com/blog/what-educators-should-know-about-ai-detection-in-2026

Personalization, privacy, provenance, corpus governance:

- PersonaCite:
  https://arxiv.org/abs/2601.22288
- LLM personalization survey:
  https://arxiv.org/html/2411.00027v3
- Agent Skills analysis:
  https://arxiv.org/abs/2602.08004
- API documentation smells:
  https://arxiv.org/abs/2102.08486
- Voice/brand references:
  https://info.arxiv.org/brand/voice.html
  https://www.ama.org/topics/brand-and-branding/
  https://emotivebrand.com/defining-brand/
