---
definition_version: 4

# R6 — one document model. Station of
# definitions/choir-reeng-metamission-2026-10-12.md; decisions
# docs/re-engineering-decisions-2026-10-12.md §7 (one document model) and
# §8 (pinned transcluded blocks). Main study:
# docs/reports/re-engineering-studies-2026-10-12/E-unified-document-model.md.
# The study's stage letters are swapped; this file uses the decision
# record's: B = quote verification, A = content-addressed capture blocks,
# C = transclusion, D = alignment across re-captures (deferred).
# Per-version permalinks and the version chain belong to
# definitions/choir-autopaper-w3a-reader-2026-10-11.md (step 1), not here.
# Supersedes app design §5 slice 2 ("C, capture citations").
# Depends on R1. Slice C depends on R5 (choir.Write) and W3a step 1.
readiness: drafted

review:
  reviewer: none yet (agentic-consensus file review before execution)
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-12T18:10:00Z
  source:
    canonical_ref: main fbb58c23
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      No quote matching exists in Go. Quotes are only checked for presence
      (sourceEntityHasTextQuote, internal/textureowner/texture_evidence_sources.go:355),
      though a comment claims "quote-match validation"
      (texture_media_sources.go:338-339). Mike's cascade (exact, whitespace
      and case, punctuation-tolerant) is pure string code
      (~/mike/backend/src/modules/chat/engine/verifyCitations.ts:41-58, 137).
    - >-
      Captures discard the bytes: WriteCaptures stores one flat-text body
      ref as both ContentBlobID and ExtractedTextBlobID
      (internal/sourcegraph/captures.go:83-84). ExtractReadableHTML returns
      (title, flatText) after choosing a candidate with x/net/html
      (internal/sourcefetch/readability.go:14-32); its callers are
      content.go:1910, browsercontrol/handler.go:710 and 806, and
      sources/article_import.go:59.
    - >-
      A changed re-capture is dropped: capture identity is
      "autopaper:capture:<item.ID>" and an existing id counts as Repeated
      (captures.go:66-73), with item.ID from source plus GUID or URL
      (internal/sources/types.go:194-205). App design §4.4 claims otherwise.
    - >-
      The validator rejects real page text and links: "[12]" is refused as
      legacy citation syntax (internal/texturedoc/schema.go:318-336) and
      only strong, emphasis and code marks exist (schema.go:307-316).
    - >-
      Texture block ids are not stable across revisions: edit ops mint
      UUIDs (tools_texture.go:1164, 1177); content writes mint
      revision-scoped ids (tools_texture.go:941-945). Two selector shapes
      coexist (texturedoc.SourceSelector schema.go:51-54;
      types.SourceEntitySelector texture_provenance.go:112-118). Publish
      builds a transclusion from an entity's first selector only
      (internal/platform/source_metadata.go:154-157, snapshot :210).
    - >-
      Texture cannot cite a capture: the target kinds have no web_capture
      (tools_texture.go:1303-1322), the surface teaches only
      {kind: "web_url", uri} (internal/yaegikernel/surface.go:39), and
      ReadCapture is research-only (choir.go:259). FetchURL writes no
      capture (internal/researchtools/researchtools.go:790); only PollSource
      does (poll_source.go:168). 0 of 10 demo sources carry a quote
      (app design §2.2).

finish:
  deliver: >-
    A source page and a story are the same kind of document, so a reader
    of a published story sees each citation's quote inside the source's own
    block, marked checked, and can read a transcluded source paragraph
    pinned to that version. The first version appears at once; checks
    arrive after it, and a failed check becomes Texture's next version.
  slices:
    - id: B
      name: quote verification
      starts_before_R5: true
      class: red (publication projection, evidence objects); B2 also provider routing
      artifact: >-
        B0: one Go Ref type {doc, version, cid, hash, quote} shared by
        texturedoc and types (it replaces neither selector shape yet) and
        internal/quotematch, a port of mike's cascade returning the
        source's own words and span. It uses the one normalization
        function in texturedoc (study E §1.3), written once by whichever of
        B0 or R5 lands first. R5's handle syntax ({{cite r2.c1 "quote"}})
        is the contract: R6 changes what a handle resolves to (a Ref), not
        what Texture writes. B1:
        choir.citation_check objects in Store B with content-hash identity,
        keyed (cited hash, quote hash, matcher version); corpusd runs the
        quote tier at publish through the memo and serves results as a live
        overlay on the existing version's bundle, outside the manifest hash
        (version_history.go:50-63). An uncaptured or unreadable source is
        "pending", never a wait. R5's Write calls the same package in-cell
        at commit. B2: support judgement, keyed (citing block hash, cited
        block hash, judge version), run in the background by a desk other
        than the author (default research, woken by a system record listing
        unjudged pairs); passes attach to the existing version, failures go
        to texture:<doc> as records (R3 inbox) and Texture writes the next
        version.
      acceptance: >-
        B1 lands with deployed non-regression (a staging republish shows
        the overlay and unchanged manifest hash and signatures); its story
        proof is station acceptance 1 and 4, after R5 cites captures. B2:
        a planted unsupported claim yields a failing check, Texture receives
        it, and the next version corrects it.
    - id: A
      name: content-addressed capture blocks
      starts_before_R5: A1 yes; A2 no
      class: A1 orange (capture runtime, Store B capture writes, additive validator); A2 red (Texture canonical writes)
      artifact: >-
        A1: texturedoc grows author_kind world, figure, table rows and
        cells, note, page_break, a link mark, and cid/h/doc_hash functions
        (cid = hash of kind and normalized text, k-th duplicate "cid.k");
        the validator skips the legacy-syntax guard for world documents and
        rejects source_ref in them. sourcefetch.ExtractBlocks walks the DOM
        with x/net/html, keeping the existing candidate, skip and noise
        logic; ExtractReadableHTML stays as a wrapper returning the block
        projection, so its four callers are unchanged. Captures store raw
        bytes, the block JSON (blocks_ref) and the projection; flat-text
        captures get paragraph blocks (extractor choir.text_blocks/1). A
        re-capture with a changed body is a new capture; an identical one
        stays Repeated. A2: the Texture reducer computes cid on every
        block, beside the lineage id that R5's alignment carries; handles
        resolve to cids. Lands with R5 or right after it.
      acceptance: >-
        A fixture page captured twice gives identical doc_hash and cids;
        headings, lists, a table and a figure come out typed; a page with
        "[12]" footnotes and links validates; the wrapper's text equals the
        block projection; station acceptance 2.
    - id: C
      name: transclusion
      starts_before_R5: false
      depends_on: [R5 choir.Write, W3a step 1 version chain, A, B1]
      class: red (Texture canonical writes, publication projection)
      artifact: >-
        A block-level transclusion node (what R5's {{embed}} handle
        writes) and a quote anchor on source_ref, both carrying the Ref,
        which lifts the one-anchor-per-entity limit; publish materializes the cited and
        transcluded blocks into the version (resolved_blocks), one
        publication_transclusions row per anchor occurrence
        (internal/platform/store.go:260); the renderer draws a transcluded
        block with the host's block renderer, the quote highlighted, and
        its check badge. Public reads never reach a paper computer. The
        transclude op stays behind a flag until it has soaked.
      acceptance: station acceptance 3
    - id: D
      name: alignment across re-captures
      status: deferred (decisions §7); reuses R5's alignment function for re-captures
  acceptance:
    - "1: on choir.news, a published story whose citations each carry a quote checked against a capture block (bundle JSON with per-anchor verdict, cid and span; logged-out screenshot of the badges)"
    - "2: a re-capture of a page whose body changed is kept as a second capture with a different doc_hash, and an unchanged re-capture is still Repeated (both capture objects and block JSON)"
    - "3: a transcluded source block rendered on a published version, logged out, pinned to that version (bundle with resolved_blocks; Playwright screenshot)"
    - "4: after an unrelated paragraph changes, the next version's checks show memo hits for every unchanged pair and one check for the changed one (per-version check table with hit and miss counts)"
  rollback: >-
    git revert per slice. Check objects are content-keyed and inert to old
    code, and the overlay is outside the manifest, so versions are
    untouched. Extra capture blobs and kept re-captures stay readable as
    ordinary captures. cid attrs are additive (the validator ignores
    unknown attrs). Transclusion nodes fail old validation, hence the flag.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize published story claims whose quote or support has not been
    checked against the exact source block, and re-checks of unchanged
    pairs, while the first version never waits and Texture stays sole writer.
  goodharting_would_be: >-
    Quotes "verified" against flat text or by a lenient matcher while the
    rendered quote is not the source's words; support judged by the authoring
    desk; or a version minted per check pass.

homotopy:
  realism_axis: >-
    fixture pages, whole capture as one block -> block extraction on fixtures
    -> real staging captures of outlet pages -> a story on a staging test
    computer citing blocks with quotes -> published, logged-out reader with
    badges and a transcluded block

boundaries:
  mutation_class: >-
    red overall (full ceremony); per slice above. Protected surfaces:
    internal/platform (publish, bundle, transclusions), Store B object kinds,
    internal/textureowner (A2, C), internal/texturedoc validator,
    internal/sourcegraph capture identity. Admissible evidence: deployed
    staging artifacts named in acceptance; local tests shape only.
  authority_sources:
    - owner 2026-10-12, docs/re-engineering-decisions-2026-10-12.md §7-8 (wins over study E where they differ)
  must_preserve:
    - Texture is the sole writer of canonical revisions; checks never edit text
    - a published version's text, manifest hash and signatures never change after publish
    - shared content and check results live in Store B behind corpusd (standing question 2); public reads never reach a paper computer
    - the support judge is never the authoring desk
    - no runtime-owned job, budget or scoring (decisions §1.2); no boot-time resumer of checks (pending pairs are re-derived only at the next publish or turn)
    - ExtractReadableHTML's callers keep working; SSRF guards in sourcefetch unchanged
  excluded:
    - per-version permalinks, version chain, front list (W3a)
    - the write verb and Texture prompt (R5); research Read-into-capture (R4)
    - D alignment; PDFs and transcripts as blocks (later)
    - copyright and public quote length (set aside, decisions §7)
  read_only_dependencies: [internal/agentcore, gateway]
  conjecture_delta: >-
    From "citations are entity selectors checked by presence" to "citations
    are Refs to content-addressed blocks, checked once per hash pair".
  heresy_delta:
    discovered:
      - changed re-captures dropped (captures.go:66-73)
      - raw bytes discarded (captures.go:83-84)
      - footnote markers like [12] reject real pages (schema.go:326)
      - one anchor per entity (source_metadata.go:154-157)
      - a quote-match comment with no matcher (texture_media_sources.go:338)
    introduced: []
    repaired: []

now:
  status: drafted
  slice: authored for file review; nothing built
  source_ref: fbb58c23
  deploy_identity: staging b172895b
  candidate: none
  conjecture:
    id: R6-1
    claim: >-
      With captures as block documents and checks keyed by block hashes,
      most quotes in a story match a block exactly or normalized, and a
      republish re-checks only changed pairs, so Autopaper stories are
      grounded at permalinks without slowing the first version.
    test: station acceptance 1 and 4 on staging; quote match rate on real outlet captures
    edge: missing_oracle
    delta_o: >-
      a fixture set of real outlet pages from staging polls, measuring how
      many research-reported quotes land inside one block (DOM heuristics and
      model copying are both unmeasured; app design R1 targets 90%)
    scope_if_supported: AI news stories on one paper
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      Order B0/B1 and A1 before R5 in parallel; A2 with R5; C after R5 and
      W3a step 1; B2 after R3 (failures need the Texture inbox). Default
      support judge: research, woken by a system record at publish.
    kind: sequencing
    status: proposed
    evidence_ref: docs/re-engineering-decisions-2026-10-12.md §7, §10
  belief:
    believed_state: the observed facts above hold at fbb58c23; no matcher, no blocks, no web_capture citation exists
    main_uncertainty: whether outlet pages extract into blocks that contain the sentences research quotes
    next_observation: file review, then quote match rate of existing staging captures against B0's matcher (read-only)
  blocker_or_risk: support-judge desk and trigger are not owner-settled (see prose)
  next_action: agentic-consensus file review of this station

receipts: []
---

# R6 — one document model

A web page and a story should be the same kind of thing: a list of
blocks, written by the world or by Texture. Once they are, a citation is
a pointer to a block and a quote inside it, a quote can be checked by
code in microseconds, and a check never needs repeating while both blocks
are unchanged. This station builds that in three slices. Quote checking
(B) and capture blocks (A1) need nothing from R5 and can start now;
transclusion (C) needs R5's write verb and W3a's version chain.

Open decision, defaulted: who judges support. Decisions §7 says a desk
other than the author, in the background. Default: research, woken by a
system record at publish that lists unjudged hash pairs; the runtime
lists, research judges. Alternatives: management, or a judge sub-RLM. The
judge's model is also open.

Ways it can fail, written before the code:

- the matcher's tolerant tiers accept a quote the source does not say
  (the rendered quote is always the source's own span; tolerant matches
  show as "≈", not "✓");
- normalization used for cid differs from the one used for matching, so
  offsets drift (one normtext function, shared);
- the DOM walk drops the paragraph a quote sits in, or merges two
  (fixtures per outlet; extractor versioned so tuning never re-addresses
  old citations);
- duplicate ordinals shift and a pinned ref resolves to the wrong copy
  (refs pin the version; ordinals are only stable within one version);
- keeping changed re-captures floods Store B with ad and nonce churn
  (identity by doc_hash of blocks, not raw bytes, decides "changed");
- the overlay leaks into the manifest and breaks signatures (overlay is
  served live, outside the hash);
- a corpusd outage hangs a Texture commit (short timeout; the anchor is
  pending, never blocking);
- checks resume at boot after a crash (forbidden; pending pairs are
  re-derived at the next publish or turn only);
- the support judge is the author in disguise, or its verdicts change
  text directly (judge is another desk; only Texture writes);
- the validator change loosens Texture's own guard (world documents only;
  Texture-authored "[12]" is still rejected).
