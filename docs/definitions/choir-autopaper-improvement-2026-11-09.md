---
definition_version: 4

# Autopaper improvement: first prove the paper uses its existing sources well,
# then give stories images that add information.
#
# Owner, 2026-11-09: "we need to improve the auto paper. ... a longstanding
# thing is to add more sources, but before we add more sources, we really need
# to make sure we're doing right with the sources that we have. And I would say
# the first thing is to add then images. ... get everything set up in
# preparation for reviewing and deciding on the auto paper improvement mission
# once I wake up."
#
# Drafted for the owner's morning review; promoted 2026-11-09 on the owner's
# order ("one more key feature before we go into the auto paper
# improvements") after the ordinary prompt path closed. ACTIVE.md and the
# mission graph name it the live station. "Decisions for the owner" run on
# their recommended defaults until answered. The roadmap places it in stage 3
# (docs/roadmap-2026-10-31.md: "images in articles").
#
# Inputs: docs/research/autopaper-media-and-rights-2026-10-14/ (README is the
# image plan; its section 6 steps 2 to 8 are this mission's build list),
# docs/autopaper-sw-design-2026-10-09.md, docs/problems/autopaper-intake-
# defects-2026-10-14.md, docs/problems/autopaper-daily-digest-phase1-2026-10-20.md,
# docs/design/autopaper-learning-materiality-2026-11-04.md.
readiness: executable

review:
  reviewer: devin (rounds 1 and 2 SEND_BACK, answered in b7d7a79c0 and this commit)
  frozen_ref: origin/main b7d7a79c0 plus the round-2 answers
  verdict: accept_on_answers
  evidence_ref: session scratchpad dv-ap/review.out and dv-ap2/review.out

metamission:
  stations:
    - {id: S1_source_audit, path: "this file, station S1", readiness: drafted, status: pending, depends_on: []}
    - {id: S2_charts_maps, path: "this file, station S2", readiness: drafted, status: pending, depends_on: [S1_source_audit]}
    - {id: S3_subject_photos, path: "this file, station S3", readiness: drafted, status: pending, depends_on: [S2_charts_maps]}
    - {id: S4_source_link_cards, path: "this file, station S4", readiness: drafted, status: pending, depends_on: [S2_charts_maps]}
    - {id: S5_renderers, path: "this file, station S5", readiness: drafted, status: pending, depends_on: [S2_charts_maps, S3_subject_photos, S4_source_link_cards]}
  order: >-
    The owner's order: source audit, then charts and maps, then subject
    photos, then source link cards, then the front-page and article renderer.
    One dependency bends it: S2 to S4 produce media objects that nothing
    shows until S5, so S5's minimal slice (render a media object on the
    article page, collapsed) lands with S2 and is not gated by S5's
    depends_on, which gates S5 proper; S5 proper (front page,
    expansion, credit lines) closes the mission. S4 needs only S2's capture
    metadata, so it may run beside S3. One file carries the stations; a
    station gets its own goal file only when it outgrows its section.

start:
  # Immutable. Read-only observation, 2026-11-09 ~03:10Z (2026-11-08 evening
  # US time). Every paper fact is one GET per route through the paper's read
  # key; no POST was made.
  captured_at: 2026-11-09T03:10:00Z
  source:
    canonical_ref: origin/main 92f49ad05 (fetched at capture)
    deploy_identity: >-
      The paper (computer-1355b83bced1613eb048ee28d66688a8) runs
      daa7fcd102cfffb533faee83e391e7e8ed8e3c68, deployed 02:04:02Z, per
      GET /api/runtime/observability. Main is one docs commit ahead.
  worktrees:
    - {path: ".claude/worktrees/agent-a805ba7945982a2d7", status: clean, class: generated_temp, owner: the drafting agent, touch: goal_owned, recovery: "delete the worktree; the draft is one commit"}
  observed:
    schedules: >-
      GET /api/autopaper/sources/schedules: 66 schedules, all active:
      27 social (Mastodon tag feeds, Bluesky accounts), 26 rss, 12 official,
      1 arxiv. Intervals 900 s (28), 1800 s (30), 3600 s (8); max_items 5 (28)
      or 12 (38). Lifetime totals across the 66: 91,101 polls, 433 errors,
      2,445,751 items fetched, 74,628 new captures (3.1 percent of fetched).
      Failing now: rss:venturebeat (23 consecutive failures),
      official:cisa:advisories (5). One schedule has never produced a new
      capture. The top four producers are Mastodon tag feeds (10,344, 9,810,
      8,905 and 3,867 new). These are the paper's lifetime counters, so rates
      per day need the audit (S1), not this receipt.
    front_page: >-
      GET choir.news/api/platform/publications/front?limit=200 returned one
      page of 100 stories (next_cursor present), updated 00:33Z to 03:08Z on
      2026-11-09. Sections: world 78, us_politics 13, tech_ai 6, business 3.
      source_count (the front row's only source field): min 2, median 3,
      max 14; no row has 1. version_count: 2 (42 rows), 3 (24), 1 (21), 4 (10).
      The front row (internal/platform/publication_front.go FrontStory) has
      no image or media field; the renderer shows no media (repo survey,
      2026-10-14, still true on main by grep: no sourceEntityMedia call in
      texture-published-citations.ts).
    pipeline: >-
      GET /api/runtime/observability: management_tasks slots 4, running 4,
      queued 1,618, oldest queued 205,619 s (57 h). That is capture-to-story
      latency for the oldest captures: the paper reads newest-first since
      80f84616c, so freshness for new captures is good and old ones may never
      run (no queued-task expiry; ledger 2026-11-08 residual). Store write
      pool: 46,648 waits. Reader gate (enforce): 51 wrong_reader refusals at
      buildStoryAnalysis (story_analysis.go:236), desk:research reading
      choir.texture_document, which means research's story analysis path is
      refused for some reads. Disclosure gate (enforce): publication allow 100,
      publication ask 6 (owner_private:autopaper, prompt-bar, texture); the
      owner's rule is that the paper never asks, so any ask is a defect to
      check in S1. GET /api/runs?limit=20 returned 20 runs, all pending
      (texture 14, management 3, research 3). Model policy: stealth/crouton-alpha
      first, with free-pool and Synthetic fallbacks.
    known_defects_open: >-
      docs/problems/autopaper-intake-defects-2026-10-14.md: (1) feed items
      beyond the per-poll limit are silently dropped, and 304 hides the
      leftovers; (5) a capture's language is its source's first language. The
      correction note says the limit should apply after the repeat check;
      whether that landed is unknown until S1 reads the code.
    media_state: >-
      From the 2026-10-14 survey, re-checked only by grep: Texture documents
      cannot hold an image, audio or video block (texturedoc closed block set);
      captures and the RSS parser discard og:image, figure img and enclosures;
      no robots.txt, TDMRep or aipref evaluation exists; selectors
      timestamp_range and image_region are defined with no consumer. Unknown
      and to be re-surveyed in S1: whether the modular-architecture packages
      changed any of this.
  unknowns:
    - Per-source captures per day, and which captures reach a published story.
    - Capture-to-publish time distribution (only the oldest-queued age is known).
    - Whether the ordinary-path mission (ACTIVE.md queue) has a Texture media
      block design yet; this mission shares it (see boundaries).

finish:
  deliver: >-
    The paper publishes sourced stories as fast as it does today or faster
    (the closing audit re-runs S1's capture-to-publish measure against S1's
    baseline),
    from sources it demonstrably uses well, and a story that has a chart, a
    map, a properly licensed subject photo or a source link card shows it,
    with credit, on the article page and (if the owner decides) the front
    page. A story with nothing worth showing shows nothing.
  artifact: >-
    (1) A source audit: a repeatable read-only report, committed under
    docs/evidence/, with per-source rows and the measures below, plus the
    fixes it justified. (2) A media object kind in the Autopaper object graph
    for generated and licensed images (bytes in the capture-body store, licence
    and credit fields, licence-evidence blob, retrieval provenance), a
    judgement ("adds information?") recorded per candidate, and the published
    renderer showing media objects with their credit. (3) Capture metadata
    that keeps image candidates (og:image, figure img, enclosure) without
    their bytes. (4) A front row field for lead media, if decided.
  acceptance:
    - action: >-
        S1: run the audit script against the paper with its read key; it prints
        per source captures per day, items fetched vs new (dedupe rate), the
        share of captures read by a desk, cited in a published story, and
        neither; per published story, sources read vs cited; freshness (median
        and p90 of capture time to source's own published time); coverage gaps
        (topics and regions with stories but only one source); and capture to
        publish time (median, p90, p99).
      proves: >-
        The paper's use of its sources is measured, with one table the owner
        can act on, before any source is added.
      evidence_class: deployed read-only proof, committed report
    - action: >-
        S2: on the paper, publish a story whose cited numbers or place
        qualify; open its permalink logged out; the page shows a chart or map
        drawn from the cited data with a caption, its data sources and (for
        OSM) the credit line; the media object's recorded values equal the
        cited values; a story without qualifying data gets none.
      proves: >-
        Generated images exist, are traceable to the story's own citations
        and are not decoration.
      evidence_class: deployed Playwright and API proof against choir.news
    - action: >-
        S3: for a story naming a person or place with a Wikidata image, the
        retrieval records licence id, licence URL, creator, credit line,
        retrieval method and an evidence snapshot; a file whose licence
        cannot be verified, an agency-authored file uploaded as own work and a
        namesake are rejected in a fixture test and in one deployed run; the
        photo renders small with credit and date.
      proves: >-
        A licensed photo appears only with its proof, and the wrong-person,
        wrong-place and licence-laundering guards fire.
      evidence_class: deployed proof plus fixtures, human inspection of three photos
    - action: >-
        S4: a published story whose cited article has an og:image shows a link
        card (title, publisher, link; never the image bytes); a fetch of the
        story's HTML and of every media route finds no third-party image URL
        in an img, source, srcset, CSS background or preload.
      proves: >-
        The article image is linked, not displayed or hot-linked.
      evidence_class: deployed proof, grep of served output
    - action: >-
        S5: the front page and an article render media objects collapsed or
        expanded per the judgement, with credit and a "generated" label where
        it applies; the front row carries lead_media only when decided; the
        mission's no-image share is reported.
      proves: The images reach readers where the owner decided they should.
      evidence_class: deployed Playwright proof, screenshots under docs/evidence/
    - action: >-
        Throughout: no new owner ask appears in the paper's disclosure gate
        counts (no new Autopaper ask; the prompt-bar and texture asks belong to the ordinary-computer approvals audit) and the
        paper keeps publishing during every deploy. At close, the audit
        re-runs S1's capture-to-publish measure against S1's baseline.
      proves: The mission did not break the paper's standing rules or slow it.
      evidence_class: deployed observability read
  rollback: >-
    Every station is additive. Revert the station's commits; media objects are
    new-kind objects that nothing else reads, the new capture fields are
    optional, and the renderer ignores a story with no media. A bad image is
    withdrawn by marking its media object withdrawn (the renderer skips it);
    a licence that proves wrong is withdrawn the same way and its evidence
    blob kept. The one schema change (Texture media block, shared) has its own
    rollback in the shared mission.
  landing:
    required: true
    environment: staging https://choir.news, then the paper; every landing commit carries Ring-Expedite (owner tempo until Autoradio is solid), and the paper is still held until each ring is seen healthy
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimise the share of captured items that are fetched and never help a
    story, and the time from a source publishing to the paper publishing it,
    while preserving citation integrity (every shown fact and image traces to a
    cited capture) and the paper's no-ask, publish-asap behaviour; then
    minimise stories whose reader would have learned more from an image the
    paper could have shown, without showing an image that adds nothing or that
    we may not show.
  goodharting_would_be: >-
    Putting an image on every story (the front page looks better); counting
    captures cited by padding citations; shrinking the capture-to-publish time
    by dropping old captures from the denominator; raising the "used" share by
    cutting low-yield sources instead of improving how they are read; a chart
    of the right number with an axis that exaggerates it; a licence field that
    is filled in without being verified at the origin.

homotopy:
  realism_axis: >-
    Resolution of the media path, same objects at every step: S1 measures with
    a read-only report on the paper's real data; S2 draws one chart and one map
    from real cited data and publishes it on d@e then the paper; S3 adds one
    verified photo class (Wikimedia Commons via Wikidata) before others
    (US federal works, Library of Congress); S4 adds metadata-only link cards;
    S5 widens from the article page to the front page. Every rung runs the real
    publish path; no mocked page and no hand-placed image counts.

boundaries:
  mutation_class: >-
    S1 yellow for the audit report itself; orange for a new read endpoint (step 1), for rights-signal recording at fetch (D7) and for the fixes it justifies.
    S2, S3, S4 orange (new product API, object kind, capture fields; the Jev
    image judgement is model routing, so orange with rollback named).
    S5 orange. The shared Texture media block is RED (Texture canonical
    writes) and is not this mission's to land; see excluded.
  authority_sources:
    - The owner's words, 2026-11-09 (header), and the scope picks relayed with them
    - "Owner, 2026-11-08: \"The point of the paper is to publish sourced stories asap\"; \"Never should it ask for human approval\""
    - CLAUDE.md (no switches, ring-ordered deploys, models, Sonnet for builders)
    - docs/research/autopaper-media-and-rights-2026-10-14/ (research, not legal advice; the owner called it "not actionable" on 2026-10-14, which this mission is the decision to act on, pending the owner's review)
    - docs/standing-questions.md
  must_preserve:
    - The paper never asks for human approval and publishes sourced stories as soon as possible. No image step may block a story: a story publishes without media and gains media in a later revision.
    - Models are crouton-alpha first through the paper's model policy. Synthetic DeepSeek is used only for benchmarking, including any benchmark of the image judgement.
    - No switch and no flag (CLAUDE.md). New behaviour ships on and walks the rings. A fixed refusal rule (no unlicensed image) is not a switch.
    - The keep set (newspaper content, the Autopaper object graph, Textures, precommitment records).
    - Custody. Document revisions are Texture's; media objects are written by the media custodian the design names, not by a desk reaching into another's kind (writer, reader and disclosure gates are in enforce).
    - Show what the world indicates; the paper does not adjudicate (design principle 1).
    - No third-party image bytes are stored or shown unless the licence was verified at the origin and recorded; the og:image is never shown.
    - No secrets and no owner email addresses in the repo.
  excluded:
    - Adding sources. This mission may recommend them from the audit; adding one is a later mission.
    - Video and podcast transcription as sources, the YouTube InnerTube decision, and robots.txt, TDMRep and aipref enforcement (research steps 2 and 9). S1 records rights signals at fetch (record only, no blocking) on D7's default.
    - Paid wire photos, the Getty viewer, stock and AI illustration (research ranks them last). AI illustration, if ever, is a separate decision.
    - The Texture media block itself (research step 8). It is shared with the ordinary-path mission (below), a red change that mission owns. This mission waits for it or ships media objects through a published-renderer path that does not touch Texture canonical writes (the recommended default, decision D9).
    - Personalised or op-ed sections (roadmap stage 3's other items) and the learning system (docs/design/autopaper-learning-materiality-2026-11-04.md).
    - Email digest images.
  protected_surfaces:
    - Publication path and front list (route and publication projection).
    - The object graph's new kind registration (custody and writer-gate tables).
    - Gateway and provider calls made by the image judgement.
    - The reader, writer and disclosure gates (in enforce). The buildStoryAnalysis refusal (desk:research reading choir.texture_document) is fixed on research's read path through Texture's custodian surface, not by widening what the gate allows; a fix that changes a gate's allow set is red and takes tier 2 review.
  shared_dependency: >-
    Texture media embeds (images and charts, PDF and file artifacts, YouTube
    and audio) are shared with the ordinary-path mission queued in ACTIVE.md
    ("audit owner approvals ... regressions in ordinary operation ... UX
    issues"). There is no goal file for it yet. One design should serve both:
    a Texture media block with display mode, caption, alt text and licence, and
    a published renderer that calls the same media rendering the editor's
    citation popovers already use (texture-source-renderer.ts
    sourceEntityMedia). This mission fixes the contract it needs: a media
    object reference with licence, credit line, alt text, caption, display mode
    (hidden, collapsed, expanded) and kind (chart, map, photo, link_card,
    embed). If that mission has not landed its block by S5, S5 renders media
    objects from the publication's metadata and the block is adopted later.

now:
  status: superseded  # 2026-11-09 evening: merged into choir-autopaper-overnight-2026-11-09.md; only D13 landed; S2 to S5 deferred
  slice: "S1: the paper's measured defects first (stale queue D13, quota incidents), then the audit export."
  source_ref: origin/main 3bafcbf9e
  deploy_identity: paper daa7fcd10 at 2026-11-09T03:10Z
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: C1_use_before_add
    claim: >-
      The paper reads far more captures than its stories use, so the first gain
      is in how it uses its 66 sources (selection, dedupe, drain order), not in
      adding sources, and images on the stories it already publishes add more
      reader value than any new source would.
    test: >-
      S1 report: the share of captures never read by a desk or cited, the
      queue's age distribution, and per-source yield. Supported if a clear
      minority of sources produces most cited captures and a large fraction of
      captures go unread. Then the image station's lift is judged by the
      judgement's yes-rate and a reader signal (opens, expansions) on a story
      sample, against the no-image baseline.
    edge: missing_oracle
    delta_o: >-
      Per-story "captures read" and "captures cited" are not exposed today (the
      front row has only source_count). The audit needs a read-only endpoint or
      a read of the tape; build that first (S1 step 1).
    scope_if_supported: the paper's current 66 schedules and the 2026-11 pipeline
    status: proposed
    evidence_refs: [start.observed]
  decision:
    what: >-
      Draft stations in the owner's order with the recommended defaults in
      "Decisions for the owner"; the recommended defaults run until the owner answers (No Blocking Asks).
    kind: purpose
    status: proposal
    evidence_ref: this file
    owner_ratification_ref: pending
  belief:
    believed_state: >-
      The paper updated 100 stories in the 2.6 hours before capture (updated_at, so not a publish rate) at a median of 3 sources
      each, with the oldest queued tasks 57 hours old and 1,618 queued; images
      do not exist anywhere in the product path; the research describes the
      legal and sourcing route, and the repo has the half-built parts.
    main_uncertainty: >-
      How much of the queued and fetched volume is waste (unread, duplicated or
      stale) versus real coverage, which sets how large S1's fixes are and
      whether images should wait on them.
    next_observation: >-
      After the queue and quota fixes deploy: the paper's queue age and
      quota_ledger; then S1 step 1, a read-only per-source and per-story export
      from the paper's tape and object graph, run twice a day apart.
  blocker_or_risk: >-
    The owner put this mission next (2026-11-09: "one more key feature before
    we go into the auto paper improvements"); ACTIVE.md and the mission graph
    name it the live station, and its only dependency, the ordinary prompt
    path, is complete. The modular architecture mission's Phase 8 runs in
    parallel; a build station defers only on a named conflict with it. The Texture media block is shared and red. Legal
    questions in the research (README section 7) are unanswered; v1 avoids them
    by choosing the lowest-risk image classes and keeping og:image a link.
  next_action: >-
    S1 runs now on the defaults: the paper's measured defects come first (the
    stale queue, D13, and the quota incidents in
    docs/problems/paper-quota-incidents-2026-11-09.md), then the audit export.

receipts: []
---

# Autopaper improvement (draft)

## Why this order

The owner's words: fix how the paper uses the sources it has, then add images.
The start receipt gives the reason to believe the first step matters: 74,628
captures from 2.4 million fetched items, 1,618 tasks queued with the oldest 57
hours old, and a front row that cannot even say which captures a story read
versus cited. Images come second because they are the largest visible gain and
the research already ranks them; the order of the image classes follows
the research (README section 2): drawn from the story's own cited data first,
free-licensed or public-domain subject photos second, the cited article's
og:image as a link card only.

No image is the default. An image appears only when a judgement says it adds
information, using the research rubric (README section 3.1): depicts the event,
depicts the subject, locates, shows the data, shows the document. Generic,
mood and decoration fail. The judgement is a typed Jev-style call run on
crouton-alpha first, like the front importance scorer, and it records its
fields (kind, adds_information, currentness, depicts_matches_story, risk,
confidence). The placement is none, collapsed or expanded.

## Stations

### S1 Source audit (yellow, then orange for fixes)

Build a repeatable read-only report first, then fix what it shows. Steps:

1. Instrument: expose per capture whether a desk read it, whether a published
   story cited it, and when; and per story the sources read versus cited.
   Prefer reading the existing tape and arrival records over adding a table.
2. Measure per source (recommended metrics, see D1): captures per day; fetched
   versus new (dedupe rate, including the silently dropped items beyond
   max_items, defect 1); share read, cited, neither; freshness (source's
   published time to capture time); failure rate (venturebeat, cisa);
   language correctness (defect 5).
3. Measure per story: sources read versus cited, single-source dependence (the
   research warns about one-source substitution), time from first capture to
   publish.
4. Measure coverage: topics and regions with stories but only one outlet; and
   the front page's section skew (world 78 of 100).
5. Fix the top findings in order of expected gain: the queue (an expiry or
   drain rule so old captures close with a residual), dropped feed items, the
   language field, the research reader refusal at buildStoryAnalysis, failing
   sources. Each fix names the audit row it moves.
6. Record the result as docs/evidence/autopaper-source-audit-<date>.md plus
   the script.

### S2 Charts and maps (orange)

Candidates come from a story's own cited data: numbers in captures (series,
comparisons) and named places (locator maps from OpenStreetMap data, ODbL
credit). A chart spec rule set (D2) fixes zero baselines for bars, units, a
source line and every plotted value tied to a cited capture block. The output
is a media object (SVG or PNG, bytes in the capture-body store, content
hashed), kind chart or map, with the data citations and a "generated by Choir"
label. The judgement decides placement. No third-party bytes.

### S3 Subject photos (orange)

Wikidata P18 to Wikimedia Commons for named people, places and organisations,
matched on the Wikidata entity id, never on a name. Then US federal works and
the Library of Congress. Per image: fetch licence metadata from the origin
(Commons extmetadata, structured data), classify (pd, cc0, cc-by, cc-by-sa,
us-federal; everything else rejected), snapshot the evidence, record the
credit line (TASL) and any modification. Guards from the research: namesake,
wrong place, outdated (caption carries the photo's date; "file photo, YEAR"),
licence laundering (agency author, uploader with deletions), crop. The
licence-check source of truth is D3.

### S4 Source link cards (orange)

Capture metadata keeps og:image, figure img (src, alt, caption) and
enclosures as metadata only. The article page shows a link card per cited
article (title, publisher, link). Never the image, never hot-linked.

### S5 Front page and article renderer (orange)

The published renderer (texture-published-citations.ts) learns to call the
media rendering the editor already has; the front row gains lead_media if D4
says images appear on the front page; expansion follows the citation
pattern (collapsed by default, expanded when the judgement says the image is
valuable). Credit lines derive from the licence fields at render time so
every place the media appears carries them, and an adopted story carries its
media object with its licence.

## Decisions for the owner

Each has a recommended default; the draft assumes the default until answered.

| id | decision | recommended default | why |
|---|---|---|---|
| D1 | Source audit metrics and thresholds | Metrics as in S1. Thresholds: a source is "low yield" if under 2 percent of its captures are cited over 14 days and "stale" if its median capture lag exceeds 3x its poll interval; a capture-to-publish p90 target of 30 min for breaking sources. Report, do not auto-disable. | Gives S1 a target without letting a threshold remove coverage; goodharting risk is cutting sources. |
| D2 | Chart library and drawing path | Server-side deterministic SVG from a small spec (no browser, no JS chart dependency in the published page); one hand-rolled renderer for bar, line and timeline; locator maps drawn from OSM data we render ourselves, not the public tile server. If a library is wanted: Observable Plot or Vega-Lite rendered to SVG in the build, pinned. | SVG is auditable, content-hashable and has no runtime dependency on the reader's side; the research warns the public tile server is not for production use. |
| D3 | Licence-check source of truth | The origin's own metadata: Commons `extmetadata` and structured data for Commons, the agency page for US federal works, the LoC item rights field. Not Openverse or any aggregator. Snapshot the response as the evidence blob. | Research: "fetch licence metadata from the origin, not an aggregator". |
| D4 | Do images render on the public front page in v1? | No. v1 shows them on the article page only; front page after S5 proves the judgement's yes-rate and the owner has seen the article pages. | Front-page images raise the stakes of a wrong-subject photo and of decorative creep; the research says track share of stories with images as a smell, not a target. |
| D5 | EU AI Act Art. 50 labelling | Label generated images ("Generated by Choir from cited data") on the image itself. For text, name editorial responsibility on the About page and keep the existing per-story AI-written disclosure; this is the owner's call and the lawyer question (README section 7, Q1) stays open. | Generated charts and maps are not photoreal and are labelled; the text stance is the owner's. |
| D6 | The no-image default and the judgement's bar | Image only when the judgement says adds_information true, confidence at least 0.7 and no risk flag; lead placement only for event or primary-source kinds that are current. Report the yes-rate; target no number. | Matches the research. |
| D7 | Rights signals at fetch (robots.txt, TDMRep, paywall state) | Record only, no blocking, as a side effect of S1's capture work; enforcement is a later mission. | Research step 2: orange, additive, shows good faith. |
| D8 | Model for the image judgement | crouton-alpha first through the paper's policy; benchmark DeepSeek on a fixed fixture set only to compare, never as the paper's runtime. | Standing rule. |
| D9 | How media reaches a story before the Texture media block exists | Media objects attach to the publication (metadata), rendered by the published renderer; the Texture block is adopted when the shared mission lands it, and the media object is the block's target. | Keeps this mission out of red Texture canonical writes. |
| D10 | When does this mission start | Now: the owner put it next on 2026-11-09 ("before we go into the auto paper improvements"); S1 first; a build station defers only on a named conflict with modular Phase 8. | The owner's order; the mission graph names it the live station. |
| D11 | Subject photo classes in S3 | Wikimedia Commons first; US federal works and Library of Congress after; Europeana and Flickr later or never. | Smallest licence surface first. |
| D12 | Withdrawal and takedown | Marking a media object withdrawn removes it from every render; a contact route on the About page served from configuration (a role address, never an owner address in the repo); no DMCA agent registered in v1 (README Q8 stays with the lawyer). | The paper never asks for approval, so withdrawal must be an owner or operator act, not a gate. |
| D13 | Stale-queue rule | Close queued tasks older than 24 h with a residual instead of running them, newest first (extends the 2026-11-08 residual). | The 57 h-old queue is not news; this is an S1 fix and the owner may prefer to start it now. |
