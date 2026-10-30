# A support pair judges a whole paragraph against one quote, so multi-source paragraphs fail and published stories churn (2026-10-19)

Status: documented, not fixed. Found at the 00:15 hourly check on
2026-10-19. This is probably the main cause of the support-failure re-wakes
in [support-failure-rewakes-texture-per-revision-2026-10-18](support-pair-judges-whole-paragraph-against-one-quote-2026-10-19.md),
and it reaches published stories, not only held ones.

## Evidence (ledger)

Story `c0fb708e` (Jacqui Lambie and a One Nation cartoon) cited
theguardian.com and an ABC capture. The gate held v2 and v3 for one
outlet, then **published v4 to v10 between 23:19:58 and 23:22:53** on
2026-10-18. That is seven published versions in three minutes. Its 9
Texture runs (23:18:37 to 23:22:35) are all `request_source=inbox_record`.
Its inbox holds 7 `system:support` records, one per revision from
23:18:49 to 23:22:29.

Every one of the 7 failures has the same shape:

- the pair's `block_id` is the same paragraph
  (`p-c0fb708e-…-a26fcc54-…-0`), while its text and hash change with each
  revision;
- the pair's `text` is the **whole paragraph**: 563 characters and 4
  sentences in the last one. It mixes claims attributed to the Guardian
  with one attributed to the ABC;
- the pair's `cited.quote` is one ABC sentence of 111 characters, matched
  `tier=exact`;
- Jev's verdict is `unsupported`, with p(supported) between 0.30 and 0.50.

Texture rewrote the ABC sentence seven ways, each closer to the quote.
None could pass, because the Guardian sentences in the same paragraph are
not in the ABC quote.

**Scope (15:30 Oct 18 to 00:15 Oct 19).** Of the 27 failing pairs I could
read across 26 Texture documents, **26 are paragraphs of 2 or more
sentences**: 9 of two, 5 of three, 6 of four and 6 of five or more. Only 1
is a single sentence. The median failing text is 417 characters against
a median quote of 116. p(supported) clusters at 0.30 to 0.49 (18 of 27).
The sample is capped by the runs listing, so read it as a sample, not a
census. I cannot see passing pairs from the inboxes, so I have no pass
rate by paragraph length yet.

**Next hour (00:15 to 01:15 Oct 19).** 15 `system:support` records with
16 failing pairs: 13 are paragraphs of 2 or 3 sentences, and 3 are single
sentences. Several of these stories cite only an HN link-only capture.
There the single-sentence failures are the expected kind (no outlet
words to support anything), so the paragraph unit does not explain every
failure.

## Mechanism (code)

- `internal/texturewrite/checks.go:66` `Checks`: for each block,
  `text := CitingText(block.Node)` is the whole block's words, with its
  citations removed. Then, for **each** citation in the block, one pair
  is made with that same block text and that one citation's quote.
- `types.SupportPairKey` hashes (block hash, cited hash, quote, matcher).
  So any edit anywhere in the paragraph makes a new key for **every**
  citation in it. That means a new judgement and, on failure, a new wake.
- `docs/archive/reports/support-judgement-to-jev-design-2026-10-14.md` (failure
  mode 4) speaks of Jev judging "the committed sentence and the captured
  block". Its guard is "unchanged code path", so the paragraph unit
  probably came over from the older management judge. Either "sentence"
  was loose wording, or the unit was never examined. I have not found a
  decision record choosing the paragraph.

So a paragraph that synthesizes two outlets, which is the paper's own
two-outlet rule at work, makes pairs that each claim one quote supports
everything in the paragraph. Jev answers truthfully that it does not.

## Effects

1. **Cost.** Each failing revision wakes Texture (the re-wake doc). The
   rewrite cannot fix a unit mismatch, so the cycle runs until Texture
   happens to change the paragraph's shape or stops.
2. **Published churn.** A two-outlet story passes the publish gate. So
   each correcting revision is a new published version: seven in three
   minutes for c0fb708e.
3. **Reader marks (hypothesis, not yet checked on the page).**
   `publishStoryVerdicts` (`story_publish.go:338`) sends verdicts for
   publish-auto stories to the published overlay. A correct citation
   whose paragraph also holds other outlets' claims may show as
   unsupported to readers.

## Correction to the re-wake doc

That doc says "the gate holds every such story" and "this is a cost
problem, not a correctness one". c0fb708e shows the cycle on a published
story. The link-only cases are held. The paragraph-unit cases can be
published.

## Classification

Substrate, in the support-judgement unit. This is the third support-judge
or gate problem documented in two days: link-only outlets, the re-wake,
and now this. By the Root Cause Clustering rule, the next fix should be
at the pair unit before any wake-budget patch. A wake budget would cut
the cost, but it would leave correct citations judged unsupported.

## Fix directions (not chosen; red surface: support judgement and the
reader overlay)

1. **Judge the cited sentence, not the paragraph.** In `Checks`, make the
   pair's text the sentence that the citation closes: the text from the
   previous sentence end or citation to this citation. Keep the block hash
   for memo identity, and hash the sentence for the key. That way an edit
   to another sentence in the paragraph does not re-key this citation.
   This is the unit the design doc names.
2. **Judge the paragraph against all its quotes together.** Make one pair
   per block, carrying every cited quote. Jev asks whether the quotes
   together support the paragraph. Fewer calls, but one bad citation fails
   the whole paragraph again, and it changes the verdict-per-citation
   overlay.
3. **Keep the unit and tell Jev the scope.** Pass the paragraph with the
   cited sentence marked, and ask about the marked part only. This is the
   smallest code change, but it is prompt-dependent, and the key still
   changes on any paragraph edit.

Ways a fix could fail, to test first:
- a sentence with two citations (two outlets for one claim) is split
  wrongly, so each quote is judged against half a claim;
- a citation placed at paragraph end that covers the whole paragraph is
  judged against only its last sentence, so a real unsupported claim
  earlier in the paragraph is never judged;
- every pair key changes at deploy, so Jev re-judges every live citation
  at once (a cost burst and a wake storm). This needs a one-time memo
  carry or a no-wake first pass;
- verdicts already in the reader overlay keep old keys and show stale
  marks.

Measure after any fix: failing pairs by sentence count (today 26 of 27
are multi-sentence), `system:support` records per Texture run (today about
a fifth to a quarter), and published versions per story per hour.

## Fix chosen (2026-10-19, owner: "You can pursue this and make smart changes")

Direction 1, the cited span, plus the paragraph as context. Mutation
class: **red** (support judgement, the evidence a published citation's
verdict rests on).

- **The pair's text is the span its citation closes.** That is the
  block's words from the previous citation, or from the block's start,
  up to this citation, plus the closing punctuation right after it.
  Citations side by side (`{{cite A}} {{cite B}}`) close the same span,
  so a fact two outlets both state is judged against each quote alone.
  That is the corroboration case. Each list item starts a new span.
- **The key hashes the span**, in the field still named `block_hash`.
  So an edit to another sentence in the paragraph no longer re-keys this
  citation, and its verdict stays memoized. A one-sentence paragraph
  keeps its old key and verdict.
- **Jev gets the paragraph as context.** The pair carries `context`, the
  block's words. Jev is told to judge only `sentence` and to use
  `paragraph` for names and references.
- **Texture is told which span failed.** The frame's failure line quotes
  the judged span, and says that naming the outlet ("X reports") does
  not make a sentence supported.

Conjecture delta: if the paragraph unit causes most support failures,
then after deploy the failing pairs' median length falls from about 417
characters to under 200. `system:support` records per Texture run fall
from about a fifth to a quarter to under a tenth. Published versions per
story-hour fall too.

Protected surfaces: the support pair unit (`texturewrite.Checks`), the Jev
support question (`askJevForSupport`), and the frame's failure text. Not
touched: the publish gate, the quote check, and the verdict store schema
(the pair's `context` is a new optional field).

Admissible evidence: staging only. Pairs in `system:support` records and
the check memos after the deploy, and front-page prose.

Rollback: `git revert` of the fix commit. Old memos keep their paragraph
keys, and new revisions key by span. Verdicts already published keep
their keys either way.

Heresy delta: discovered, judging a paragraph against one quote, which
the design called "the committed sentence". Repaired by this fix if the
conjecture holds. Introduced: none expected. The residuals are below.

Ways it can fail, tested first:
- S1: a span is cut wrongly: the trailing period lands in the next
  span, or a span starts mid-word;
- S2: two side-by-side citations get different spans, so the first one
  is judged against nothing;
- S3: a citation at a paragraph's end with no earlier citation does not
  cover the whole paragraph;
- S4: an edit to another sentence still changes this citation's key;
- S5: a one-sentence paragraph's key changes, so every existing verdict
  is judged again;
- S6: Jev is not given the paragraph, so "it" and "the ministry" in a
  span cannot be resolved; or it is given the paragraph without being
  told to judge only the span;
- S7: the frame still does not say which span failed.

Residual: text after a block's last citation is in no span and is not
judged. It was judged before, inside the paragraph. A claim with no
citation is outside the support judge's job, but this does narrow what
it sees.


## Merged from docs/problems/support-pair-judges-whole-paragraph-against-one-quote-2026-10-19.md (2026-10-18)

**A support failure re-wakes Texture once per revision, so an unsupportable story takes 7 to 9 turns (2026-10-18).**
Status: documented, not fixed. The cycle ends on its own. **Correction (2026-10-19):** I first wrote that this is "a cost problem, not a correctness one" because "the gate holds every such story". Story c0fb708e ran the cycle while published (v4 to v10 in three minutes). The likely main cause is that a support pair judges a whole paragraph against one quote: [support-pair-judges-whole-paragraph-against-one-quote-2026
- Status: documented, not fixed. The cycle ends on its own. **Correction

(Full text in git history at the path above.)
