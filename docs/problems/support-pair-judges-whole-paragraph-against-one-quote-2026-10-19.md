# A support pair judges a whole paragraph against one quote, so multi-source paragraphs fail and published stories churn (2026-10-19)

Status: documented, not fixed. Found at the 00:15 hourly check on
2026-10-19. This is probably the main cause of the support-failure re-wakes
in [support-failure-rewakes-texture-per-revision-2026-10-18](support-failure-rewakes-texture-per-revision-2026-10-18.md),
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

## Mechanism (code)

- `internal/texturewrite/checks.go:66` `Checks`: for each block,
  `text := CitingText(block.Node)` is the whole block's words, with its
  citations removed. Then, for **each** citation in the block, one pair
  is made with that same block text and that one citation's quote.
- `types.SupportPairKey` hashes (block hash, cited hash, quote, matcher).
  So any edit anywhere in the paragraph makes a new key for **every**
  citation in it. That means a new judgement and, on failure, a new wake.
- `docs/reports/support-judgement-to-jev-design-2026-10-14.md` (failure
  mode 4) speaks of Jev judging "the committed sentence and the captured
  block". The design meant the cited sentence. The code sends the
  paragraph.

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
   This is the design doc's stated unit.
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
