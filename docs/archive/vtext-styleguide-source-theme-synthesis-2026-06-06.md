# VText Style Sources — Theme Synthesis

## Executive Synthesis

The complete source set converges on one thesis: style fidelity is not solved by one master style prompt. It is solved by an agentic style-memory architecture with three durable layers:
- observable corpus evidence,
- example retrieval and adaptation,
- and review notes that make style judgment explicit without turning writing into checklist compliance.

The strongest practical signal is that systems succeed when they preserve author agency, support edit modes, and expose why output changed.

## What is near-consensus across almost all sources

- Example-first design is superior to rule-only style control.
  - Evidence sources: Houtini voice analyzer work, GhostAI, many anti-slop repos, few-shot prompting guides, and large style-imitation papers.
  - Direction: treat `Style.vtext` as a compact, editable control artifact plus exemplar retrieval, not one exhaustive rule wall.

- Style and anti-AI observations should stay separable.
  - Evidence sources: GhostAI’s profile+guide+logs split, cc-prose execution split (create/edit), and Lint ecosystems (Vale/VectorLint/Stringly-Typed).
  - Direction: separate positive voice profile, editable guide, exemplar bank, and review notes. Avoid making an executable policy layer the default product model.

- Human preference and machine-extracted voice are both first-class.
  - Evidence sources: brand voice practitioner writeups, PersonaCite-like provenance framing, user-facing style rules + editorial feedback loops.
  - Direction: keep “observed style” and “desired style” distinct and merged only with user acknowledgment.

- Anti-model-tic detection should be adaptive and contextual, not static.
  - Evidence sources: anti-tic papers and lists, Houtini/Rossmann-style critique, detector failure research.
  - Direction: compute deviations relative to person/domain baseline, not against abstract fixed banned phrases.

- Lint-style checks are useful but should remain diagnostic.
  - Evidence sources: Vale/Grafana/GitHub style workflows plus community concerns.
  - Direction: use deterministic checks for hygiene/consistency when they clearly help; keep semantic style fidelity in the VText agent's generation and revision loop.

- Editing is where style is usually destroyed; generation often looks easier.
  - Evidence sources: Voice Under Revision paper, authorship-preserving UX papers, practitioner reports.
  - Direction: use edit-mode intent, protect key phrases, and make change-level provenance visible.

## High-confidence controversies

- Fine-tuning now vs later.
  - One camp argues fine-tuning gives stronger long-tail consistency; another sees instruction- and retrieval-based approaches as safer.
  - Practical resolution: start with RAG + style controls, evaluate cost/performance before adapter paths.

- Can detectors be gatekeepers or only advisory signals?
  - Many detector providers and detector papers disagree on reliability and robustness.
  - Practical resolution: never hard-stop user flow on detector output; use diagnostics for human review.

- Hard automation versus human-in-the-loop.
  - Productivity tooling pushes toward automation; writing practitioners push back against “average” and stilted output.
  - Practical resolution: let the VText agent choose a writing posture from context and make that posture visible to the user.

- Universal style rules versus client-specific style evolution.
  - Uniform anti-slop lists can improve generic text but often flatten idiosyncratic, high-signal style.
  - Practical resolution: define universal minimum checks and client-specific adaptive checks.

- Public platform-scale docs discipline versus VText workflow needs.
  - Docs-heavy ecosystems enforce standardization strongly; VText needs context-aware, mutable author artifacts.
  - Practical resolution: borrow their evidence and feedback habits, not their compliance-heavy operating model.

## Outliers and high-variance signals

- Mainstream media coverage (TechRadar, Scientific American, Verge, Atlantic).
  - Useful for external narrative and expectations, but low reliability for implementation specifics.
  - Treat as sentiment and framing, not architectural authority.

- Reddit threads and community anecdotes.
  - High value for user frustration/voice-loss failure modes, low reproducibility.
  - Use as UX risk examples and test scenarios.

- Detector products and marketing pages (GPTZero, Originality.ai, Copyleaks).
  - Strong as threat models; weak as direct control-plane inputs.

- Issue trackers and niche repos.
  - High implementation noise but often reveal exactly where real systems break (e.g., repeated warnings, UX friction).

## Thematic map with consensus markers

- Foundation layer (high consensus):
  - RAG / exemplars + retrieval discipline for style inference.
  - Small, explicit style guide with context and examples.
  - Voice/rhythm preservation review notes.

- Control layer (medium consensus):
  - Fine-tuning adapters as opt-in optimization.
  - Model-specific anti-slop observations.
  - Automatic tone/style scoring as advisory telemetry.

- Product layer (mixed consensus):
  - Which contexts deserve stricter review: email and legal docs versus exploratory drafts.
  - User control surfaces for override, style direction, and intentional departure.
  - How far public/distributed styles should go before they become costume or impersonation.

## Recommended decision model for Choir

- Define style as a set of interacting artifacts:
  - `Style.vtext` (canonical human-readable and agent-usable style artifact),
  - `VoiceProfileVText` (observable style fingerprint),
  - `ExemplarBankVText` (curated positive/negative examples),
  - `StyleReviewVText` (agent-written critique, drift notes, and revision rationale),
  - `StyleMemory` (durable observations mined from edits, examples, and explicit user notes),
  - `StyleDistribution` (future publishing, licensing, sharing, and composition metadata).
- Use detector systems as diagnostics, not gates.
- Always run “what changed?” reporting at edit-accept time and persist with provenance.
- Keep outlier detectors and media claims in a separate evidence list and avoid policy lock-in from them.

## Product Philosophy: Style, Not Taste

The system should use the word style, not taste. Taste implies selection among
available goods; it can often be bought, outsourced, or imported. Style implies
expression: how a person, firm, publication, or institution turns material into
a recognizable way of thinking and being. For Choir, style must fit the
writer's corpus, situation, constraints, audience, stakes, and judgment.

This argues for `Style.vtext` as an authored asset rather than a package of
preferences. Public or distributed styles may exist later, but the primary
loop is not "download style." It is:

```text
your corpus + your edits + your contexts + your standards
-> Style.vtext
```

A reusable external style can be an influence, but the VText agent should adapt
it to the author and context rather than replacing the author with it.

## What to build first from this consensus

- High priority:
  - Exemplar-aware generation path.
  - Edit-mode intent with voice-preservation review.
  - Style report VText output tied to rewrite provenance.

- Medium priority:
  - Relative model-tic scoring.
  - Corpus governance + consent metadata in manifest.
  - Fine-tuning experiments behind explicit opt-in gates.

- Low priority:
  - Detector hard gating.
  - Strict global no-tic rule lists.
  - Full stack alignment to enterprise docs-lint ecosystems without adaptation to user contexts.

## Source coverage summary

- Total sources covered: 134 unique URLs.
- Consensus-heavy regions: practitioner tooling docs, lint ecosystems, style-transfer research, anti-tic discourse.
- Coverage gaps:
  - Limited direct evaluation data for legal/enterprise client writing at production scale.
  - Limited multi-language and low-resource language evidence.
  - Limited evidence on long-duration adaptation drift under adversarial or noisy corpora.

## Quick interpretation for architecture

The landscape is coherent despite surface diversity. The strongest unifying interpretation is:
1) collect evidence-rich corpora,
2) let examples steer generation,
3) make edits explain their stylistic tradeoffs,
4) treat anti-AI detectors as warnings, not truth.

That model aligns directly with the VText objective of adding compute without collapsing idiosyncratic voice.
