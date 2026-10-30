---
definition_version: 4

# Cut 2, evidence to publication. Station C2 of
# docs/archive/definitions/choir-reeng-metamission-2026-10-12.md; depends on C1
# (docs/archive/definitions/choir-reeng-cut1-authority-2026-10-12.md: records,
# conversations, rounds, hypothesis ids, cursors, Texture's inbox) and C0
# (docs/archive/definitions/choir-reeng-cut0-eyes-2026-10-12.md: trace export,
# tracer, invariant checker, fresh test computers, request-id fix).
# Authority: docs/re-engineering-decisions-2026-10-12.md §2-§8, §10, §12
# (owner 2026-10-12; §12 wins over every earlier draft). Code-cited detail
# carried from the superseded drafts R4, R5, R6 (at 03499ef9), W2, W3a and
# the Autopaper metamission (2026-10-11), and studies A, D, E in
# docs/reports/re-engineering-studies-2026-10-12/. Their gradual slicing,
# flags and one-release keeps are rejected; their findings stand.
readiness: reviewed

review:
  reviewer: convergent agentic-consensus panel (codex, claude opus, gpt-6.1-sol, gemini-3.8-flash, deepseek-v4.1-flash, glm-5.3-flash; gpt-6-luna and muse-spark timed out)
  frozen_ref: 2d2b864e
  verdict: send_back (codex, gpt-6.1-sol, gemini), accept as gated draft or with amendments (claude, deepseek, glm); repaired in the following commit; re-verified at C1's landed commit before execution
  evidence_ref: docs/re-engineering-decisions-2026-10-12.md §13 (synthesis)
  rereview: claude opus and gemini-3.8-flash at deb6f760 (codex and gpt-6.1-sol hit a usage limit); accept (claude, gemini); rhythm clause amended to report, not gate

start:
  captured_at: 2026-10-12T19:30:00Z
  source:
    canonical_ref: main 03499ef9
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty (docs/re-engineering-decisions-2026-10-12.md and the cut files, this session's doc edits in progress)
      correction_2026-10-12: those edits landed at 2d2b864e and the tree was clean at review; Go source is unchanged between 03499ef9 and 2d2b864e
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Search is one blocking call. choir.WebSearch (internal/yaegikernel/choir.go:456)
      reaches the web_search tool (internal/researchtools/researchtools.go:658,
      egress charge :680), then search.Client with a 30 s hostlink client
      (internal/search/search.go:85), then HandleSearch
      (internal/gateway/handlers.go:856) under a fixed 30 s deadline (:914),
      then Router.Search's waves (internal/gateway/searchplane/router.go:64).
      executeWave already collects each provider's completion on a done
      channel (router.go:221, :232, :285-291) but returns one merged list.
      That tool is the only production caller of the blocking client.
      hostlink's AttemptTimeout bounds the body too
      (internal/hostlink/hostlink.go:38-40), so a stream needs a zero bound;
      inference already streams over the same link (study A §3.3).
    - >-
      Search waits on the slowest provider: Keenable about 0.2 s, Tavily
      and Parallel 2-4 s, ChatGPT about 23 s; 15-27% of searches hit the
      30 s cap (docs/archive/search-plane-design-fast-deep-broad-2026-10-11.md §1-2).
    - >-
      REPL state: a persistent desk session per "agent:"+id
      (internal/agentcore/tools_desk.go:111), released at :226 and reclaimed
      at :177. Host values enter a cell only through SessionFrame
      (internal/yaegikernel/session_loop.go:25) at BindCell Begin
      (choir.go:145); End drains the staged tray (choir.go:154). The cell
      result map is built at tools_desk.go:405-416. Study A's race probe:
      a mutex-guarded store read mid-cell is clean; a raw exported map races.
    - >-
      Research's surface lists retired verbs and four ways to fetch
      (FetchURL, ImportURL, ImportDocument, selector reads), none of which
      keeps what it fetches (choir.go:255-259; FetchURL
      researchtools.go:790). Captures are written by PollSource
      (internal/researchtools/poll_source.go:168) through corpusd's capture
      handler (internal/platform/captures.go:150-170 ->
      sourcegraph.WriteCaptures), the one capture writer. Children get a separate
      read set (choir.go:263-265). The research egress budget is 64 calls
      and 32 MiB (internal/researchtools/egress_budget.go:46-47).
    - >-
      Captures: identity is "autopaper:capture:<item.ID>" and an existing
      id counts as Repeated, so a changed re-capture is dropped
      (internal/sourcegraph/captures.go:66-73); one flat-text body serves
      as both ContentBlobID and ExtractedTextBlobID, so raw bytes are lost
      (:77-84). ExtractReadableHTML returns (title, text)
      (internal/sourcefetch/readability.go:14-15) to four callers
      (content.go:1910, sources/article_import.go:59,
      browsercontrol/handler.go:710, :806). The validator refuses "[12]"
      in any document (internal/texturedoc/schema.go:326) and has no link
      mark (:307). Quotes are checked only for presence
      (internal/textureowner/texture_evidence_sources.go:355); no matcher
      exists in Go. PDF text and page extraction exists
      (internal/content/extract.go:104-131) but produces no blocks. Mike's cascade (exact, whitespace and case,
      punctuation-tolerant) is pure string code
      (~/mike/backend/src/modules/chat/engine/verifyCitations.ts:41-58).
    - >-
      Texture: module set ReadDoc and ApplyTexture plus retired Message and
      Emit (choir.go:271-273); ApplyTexture's surface text (surface.go:39).
      A whole-document write re-mints every block id
      (internal/textureowner/tools_texture.go:790); inserted blocks get
      fresh UUIDs (:1164, :1177); links and numeric markers in content are
      refused (:937). Prompt: role 4,721 characters
      (internal/textureprompts/texture.yaml) plus 35,131 in overlays
      (revision_policy.yaml 16,839, run_system.yaml 14,600). The wake is
      pointer-led but still carries source entities and the source
      instruction (internal/agentcore/coagent_update_packet.go:161). One
      large write takes 76% of model time; turns average about 5 cells
      (study D §0.1-0.2). Texture runs opencode-go deepseek-v4.1-flash at
      high effort (internal/modelpolicy/model_policy.go:370); the
      comparison candidates are in internal/modelcatalog/catalog.go:85-146.
    - >-
      Management reports to Texture "with choir.Report, carrying its
      evidence refs", with no translation contract
      (internal/promptstore/defaults/management.yaml:21). Desks may not
      write a Texture through ProductAPI
      (internal/agentcore/tools_product_api.go:181-184). Jev is a gateway
      decision endpoint pinned to typesafe/jev-1.13
      (internal/gateway/jev.go:20-27); only self-development candidate
      judgement calls it (internal/agentcore/selfdev_candidate_judgment.go:38).
      No desk has an in-cell Jev verb.
    - >-
      Publishing: PublishTexture (internal/platform/service.go:153) mints a
      new publication, version and route on every call (:200-211); the
      version is hard-coded "v1" with no predecessor
      (internal/platform/publication_graph.go:129, :138); a manifest embeds
      the version history when one is present (service.go:246-255); the bundle
      read takes the last has_version edge
      (internal/platform/service_publication_read.go:88-96); a citation
      keeps only an entity's first selector
      (internal/platform/source_metadata.go:152-158). The
      publication_transclusions table exists (internal/platform/store.go:260).
      Artifact source targets diff_hunk, patch and test_run exist
      (internal/texturedoc/schema.go:381).
    - >-
      Reader: the old Universal Wire app (frontend/src/lib/UniversalWireApp.svelte,
      internal/platform/universal_wire.go) is signed-in only and stamps two
      fixed claims on every story. The public resolve route already serves
      a published Texture logged out, and the Texture renderers already draw
      numbered sources with quote popovers and a "newer version" marker
      (frontend/src/lib/texture-source-renderer.ts:378-500).
    - >-
      Autopaper: W1 landed (scheduled polls into Store B captures, no model
      call, closed on a crash). Nothing tells management captures arrived,
      and management cannot feed a Texture other than its own trajectory's
      (W2 review receipt W2-review1). The only automatic publishing is the
      autonomous Universal Wire path, which Cut 1 deletes. Schedules start
      only through the owner route POST /api/autopaper/sources/schedules
      (apihandler/routes.go:31; source_schedule.go:266-295).

finish:
  deliver: >-
    Autopaper is live on choir.news. Research starts searches that return
    at once, reads each provider's results as they land, reads the pages
    that matter into captures, and tells Texture what it learns as soon as
    its understanding changes. Captures and stories are one kind of
    document, so every citation points at a source block and a quote that
    code has checked; management (or Jev) judges whether the source
    supports the claim, in the background, and Texture writes the
    correction. Each story revises as evidence arrives, each published
    version has its own permanent link, and a logged-out reader opens the
    paper's front page and reads them. Then the same tracer compares
    Texture models and reasoning effort.
  units:
    - id: 2a
      name: evidence
      artifact: >-
        Async search. Gateway: /provider/v1/search streams NDJSON, starting
        every eligible provider at once, one line per provider as it
        completes, then a final line with "M of N providers answered" and
        the cooling providers; bounded by the provider timeout plus a
        margin. The waves, target, grace and the blocking mode are deleted
        with their only caller (no guest is kept on pre-cut code; decisions
        §13.1). Guest:
        search.Client.Stream (zero attempt bound, refused-dial retry kept);
        a host search table keyed by the desk-session key, its streams
        cancelled in release and reclaimLocked. The host pushes each
        provider's arrival to the worker as it lands, as its own frame type;
        Cut 0's single reader loop writes it into a lock-guarded store in
        REPL state that End does not clear, so a running cell sees later
        providers through Searches.Get without waiting or a new cell.
        Arrival payloads never enter the model's context. Verbs: choir.SearchStart(q) returns a stub
        at once ("s3 started: 4 providers"); choir.Searches.Get/List return
        copies; choir.SearchWait(id, ms) blocks up to ms and returns counts.
        Each cell result gets one count line per search with new arrivals
        ("s3: 3/4 answered, +2 new"), the only push. choir.WebSearch and
        the web_search tool are removed from every desk.
        Research desk on it: verbs SearchStart/Searches/SearchWait, Read
        (fetch into a capture; replaces FetchURL, ImportURL, ImportDocument
        and the selector reads), Report (final flag and stop reason from
        C1), Research, Ask, Reply, Note. Reports go out as soon as
        understanding changes, each claim tagged with hypothesis ids and an
        epistemic status; the final Report closes the round at diminishing
        returns relative to the Cast's hypotheses and stakes. The
        `degraded` field in research results is deleted (the egress budget
        went in Cut 1).
        Captures as block documents: sourcefetch.ExtractBlocks walks the
        DOM with x/net/html (reusing the candidate, skip and noise logic)
        into typed blocks (headings, paragraphs, lists, tables, figures,
        notes, links); ExtractReadableHTML becomes the block projection for
        its four callers; a capture keeps raw bytes, block JSON and the
        projection; cid = hash of kind and normalized text (k-th duplicate
        cid.k), one normtext function shared with the matcher; identity by
        doc_hash, so a changed re-capture is a new capture and an identical
        one stays Repeated; world documents accept real page text and links
        ("[12]" stays refused in Texture-authored blocks). PDFs are captured
        too (decisions §7): content/extract.go's page text is split into
        paragraph blocks per page, with raw bytes, page numbers and
        provenance kept, so a quote in a PDF is checked against a block like
        any other. Layout reconstruction is not attempted.
        One reference shape: a Go Ref {doc, version, cid, hash, quote}
        shared by texturedoc and types, replacing both selector shapes for
        new citations. internal/quotematch ports mike's cascade and returns
        the source's own span; results are memoized in Store B keyed by
        (citing block hash, cited block hash, quote hash, matcher version).
        Support judgement: when a revision commits, the same transaction
        appends one system record to management listing the new unjudged
        pairs; management judges them (calling Jev through a new
        management-only choir.Judge verb, or reasoning itself); each verdict
        is a record keyed by the hash pair; a failure is addressed to
        texture:<doc>; a pass attaches to the existing revision and mints
        nothing.
        Texture re-engineered: each turn's frame, rebuilt from C1's inbox
        past the cursor, carries the document as short-alias blocks b1..bN
        with citations as tokens and the new records r1..rM with sources
        r2.c1.. and their quotes; no read cell needed. One verb,
        choir.Write(edits, why): @replace/@insert/@delete/@move/@section/@all
        on aliases, inline {{cite <handle> "quote"}} and {{embed <handle>}}
        tokens resolving to Refs, synchronous line-located refusals, @all
        reconciling against the prior revision so unchanged blocks keep ids
        and citations; changed blocks only. Wait, Done and Reject replace
        decide, work_disposition and update_dispositions. Prompt core about
        3.5k characters, verb list generated from the module set (C1's
        generator). ReadDoc stays as the one document read verb,
        generalized to any block document (a Texture or a capture) and
        shared by Texture, management and research; for a long document the
        frame carries a section digest and ReadDoc fetches the rest.
        ApplyTexture, the revision_policy and run_system overlays and every
        retired verb on Texture are deleted. Texture merge is deleted too
        (textureowner/texture_merge.go, the /merge-preview and
        /accept-merge routes, management's ProductAPI exception for them,
        and TextureCompareMergePanel): a one-shot model call outside any
        desk that writes owner revisions, already refused for
        lifecycle-authored documents; the version chevrons replace
        compare. Cut 1 wrote Texture's prompt
        core; this unit adds the Write, citation and Done/Wait/Reject
        judgement.
        Management's prompt gains the translation contract: a plain summary
        plus artifact handles that Texture cites as chips, so owner-facing
        prose carries no code, diff, command or log.
    - id: 2b
      name: publication
      artifact: >-
        Per-version permalinks: one publication per (owner, source doc). An
        article is a Texture and an article version is a Texture version
        (decisions §13.2): each committed revision of a story publishes as
        version n, where n is its Texture revision number, idempotent on the
        revision, with supersedes set, a parsed /v<n> route, and a manifest
        that references the previous manifest hash instead of embedding
        every past body. A revision that fails to publish leaves a gap that
        nav skips; nothing is renumbered. Stories work in public: they
        publish from their first revision, and a story is usually
        publication-ready around v5 to v10. The bundle carries nav
        {revision, prev, next, latest}; the bare link serves the latest in place; chevrons
        and a "new version available" control. A version has an immutable
        payload (text, manifest and its hash, signatures, resolved_blocks)
        and two live fields served beside it (nav, check overlay); only the
        payload is fixed forever. Transclusion: the
        {{embed}} block node and the cite anchor carry the Ref; publish
        materializes cited and transcluded blocks into the version
        (resolved_blocks, one publication_transclusions row per anchor), so
        a version's source blocks are pinned to it. Quote-check and
        judgement results show as a live overlay outside the manifest hash
        (✓ exact, ≈ tolerant, pending, ✗ failed).
        Living stories (W2 on the protocol): after a poll writes new
        captures, the runtime appends one system record to management
        naming them; management's cursor batches whatever accumulated.
        Management opens a story by a Cast to a new texture:<doc> carrying
        hypotheses, the capture refs and publish: auto, and feeds an
        existing story by a Cast or Note to its texture:<doc>. Management
        and Texture read capture blocks through ReadDoc.
        Management never writes a Texture.
        Automatic publishing (W3): every committed revision of a document
        opened with publish: auto becomes its next version; no human step.
        Reader (W3a): a public front list served from Store B (title, dek,
        updated time, version count; one row per story) and the autopaper
        app's front page and story page, logged out, at phone width. The
        Universal Wire app, its route, types.WireStory and the fixed claims
        are deleted, with their specs replaced; with them the frontend
        read_owner plumbing (texture.js WIRE_PLATFORM_READ_OWNER,
        push/popTextureReadOwner, TextureEditor's platform read scope) and
        the two Universal Wire specs. Texture merge's frontend goes in the
        same commit as its routes: the TextureEditor and TextureToolbar
        wiring and the three texture.js fetchers (dead-code census Part C).
        Also from the census: the sourcecycled daemon (cmd/sourcecycled,
        internal/cycle, wire/processorkey, its nix unit and flake entry)
        once captures and schedules are confirmed as the only ingestion
        path; the 24 Store B tables in internal/platform/store.go that no
        code reads or writes (DDL only, after an empty-count check on
        staging); and defect D2, reader proposals delivered to a deleted
        guest route, decided under the record model.
        Crash: boot closes the paper's open rounds and the active schedules
        (Cut 1 keeps that phase) and starts nothing; no desk wakes at boot
        and no schedule re-arms itself. There is no restart policy yet
        (decisions §13.3): the owner restarts Autopaper by hand through the
        schedule route (POST /api/autopaper/sources/schedules). A restart
        policy is a later station.
        Freeze receipt, written before the paper computer's first publish:
        the record schema, normtext, the cid formula and extractor version,
        the Ref shape and the /v<n> route shape, with a digest.
        Compatibility begins at that permalink (decisions §12.1).
  autopaper_live: >-
    On staging, read through the product API and a logged-out browser, all
    of: (1) a designated paper computer (not a test computer) runs its
    source schedule after one owner act starts it, with no further human
    step, watched for at most one hour (owner pace ruling 2026-10-13,
    metamission boundaries.pace 6) and left running; (2) production follows the ingestion
    rhythm (decisions §13.4): every poll that writes new captures is
    answered by management's next committed turn after its record (the
    latency from record to answer is reported per interval, never gated),
    by a new story, a Cast or Note
    feeding an existing story, or a management record declining those
    captures with a reason, and the receipt lays stories opened, revisions
    and captures side by side per poll interval (no story count is set; a
    one-hour window on real feeds should produce several stories, not a
    handful; one poll's captures may open several stories at once, and the
    ratio of new stories to revisions starts high and falls as the corpus
    grows, which the receipt plots); (3) every committed revision of a story is published
    automatically as its Texture version, several stories reach v5 or
    later, and in each of those at least one revision cites a capture
    that arrived after its previous version; (4) the front page at
    choir.news lists each story once, newest first, marked updated, and
    every citation on its story pages shows a quote inside its source
    block with a check badge; (5) public reads keep working while the
    paper computer is stopped. "Live" is a state that holds; a crash ends
    it until the owner restarts it by hand.
  acceptance:
    - action: "2a, tracer on a fresh test computer: research cell trace read through the product API (C0 export)"
      proves: SearchStart returns at once; count lines rise per provider in latency order; the gateway's final line says M of N; every stream ends at N/N or is cancelled with its session; no arrival carries another query's id; one running cell sees a provider that answered after it started through Searches.Get, with no SearchWait and no new cell
      evidence_class: deployed proof
    - action: "2a, tracer: research on a Cast with hypotheses (including the omission hypothesis)"
      proves: Reports carry hypothesis ids and epistemic status; Read writes captures; a final Report closes the round naming why it stopped; first Report no later than the demo19/20 baseline (about 70 s to 2 min)
      evidence_class: deployed proof
    - action: "2a, tracer: read a tracer-controlled page, change it, read it twice more"
      proves: a changed re-capture is kept (two captures, different doc_hash, raw bytes and typed block JSON present); an unchanged re-capture is Repeated; a page with "[12]" footnotes and links validates; a fixture PDF becomes page-numbered paragraph blocks and a quote from it checks
      evidence_class: deployed proof (Store B objects)
    - action: "2a, tracer: Texture integrates two records in one turn, then an @all rewrite; per-turn metrics against C0's baseline (Cut 0 artifact 7, from its kept tracer bundles)"
      proves: one Write adds a paragraph cited inline; untouched blocks keep ids and citations (revision JSON diff); calls per turn and model-seconds per turn fall; ApplyTexture is absent from the module set and the code
      evidence_class: deployed proof
    - action: "2a, tracer: the owner edits in a sentence citing a real quote that does not support it"
      proves: the quote check passes, management records a failing support judgement, the failure reaches texture:<doc>, and Texture writes the correcting revision; the next unrelated revision shows memo hits for every unchanged pair
      evidence_class: deployed proof
    - action: "2a, tracer: an engineering-backed request (moved to station C1e by the Cut 1 scope split of 2026-10-13; C2 is accepted without it, and C1e runs it)"
      proves: the owner-facing revision has no code, diff, command or log in its prose, and the change is reachable from a citation chip (Playwright screenshot)
      evidence_class: deployed proof
    - action: "2b, tracer: a story publishes each revision automatically; at version n the tracer page changes to disconfirm it; research or management brings the re-capture; a later version m publishes the correction"
      proves: version numbers equal Texture revision numbers; version n's immutable payload (text, manifest hash, signatures, resolved_blocks) is byte-identical before and after m; n's nav gains next and latest; the check overlay may change and never touches the manifest hash; the bare link shows m; n shows chevrons and "new version available"
      evidence_class: deployed proof
    - action: "2b: logged-out Playwright on choir.news at phone width"
      proves: front page and story page render with numbered sources, quotes in source blocks, check badges and a transcluded block; screenshot artifact; no served route still renders the fixed claims
      evidence_class: deployed proof
    - action: "2b: the autopaper_live window (at most one hour watched), then a crash restart of the paper computer (no planned marker) through the product API, then the owner act that restarts the schedule (taken by the run, metamission authority_sources), then one poll observed"
      proves: Autopaper live as defined; after the crash every open round and schedule closed "interrupted by a restart", no poll ran and no desk woke until the schedule was restarted by the owner act; reads stayed up throughout; the paper is running when the receipt is written (plan review 2026-10-13, C13)
      evidence_class: deployed proof
    - action: "after live: model comparison on the tracer script; deepseek-v4.1-flash, gpt-6-luna, glm-5.3-flash, muse-spark-1.3-contributor and the other opencode-go models, 3 runs each; then high, medium, low effort on the leading models"
      proves: a receipt table judged blind by a model panel on grounding (claims supported by their cited blocks, quote accuracy, omissions), with latency and cost beside it; never runtime scoring. The default switch is the owner's routing decision, outside this station
      evidence_class: deployed measurement plus model judgement
  rollback: >-
    Forward-fix (decisions §12.3). Before unit 2a deploys, tag C1's landed
    commit as reeng-cut2-last-good. Nothing before the first Autopaper
    permalink is preserved (decisions §13.1). A failed unit is fixed
    forward on main; the tag is the break-glass. Before the freeze receipt, schema, cids and refs
    may change freely; after it, only compatible changes.
  landing:
    required: true
    environment: staging
    units: "2a and 2b, one deployed acceptance each (internal commits fine)"
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance, consensus_review]

value:
  better_means: >-
    Minimize the gap between what the world's evidence says and what a
    logged-out reader sees at a story's permalink (claims not checked
    against the exact block they cite, evidence that reached the computer
    but not the story, time from arrival to a published version), while
    Texture stays sole writer, published versions never change, and the
    runtime holds no budget, cap, job or score.
  goodharting_would_be: >-
    Versions published with no new evidence; tolerant matches shown as
    exact; support judged by the desk that wrote or found it; count lines
    that change while the model never reads the store; thin, hedged or
    early-closed Reports; a front page of stories with bare-link sources;
    a tracer passing on its own fixture page while real outlet pages fail
    extraction; "live" claimed from one tracer run; a model chosen for
    speed while its stories cite less.

homotopy:
  realism_axis: >-
    gateway stream probed directly -> tracer research on a fresh test
    computer with real providers and one controlled page -> Texture
    writing and checks on that tracer story -> the tracer story published,
    disconfirmed and republished -> the paper computer on real feeds,
    unattended -> crash and manual restart -> model comparison on the
    live desks

boundaries:
  mutation_class: red
  authority_sources:
    - owner 2026-10-12, docs/re-engineering-decisions-2026-10-12.md §2-§8, §10, §12
    - CLAUDE.md Restarts, Mutation Classes, Landing Loop
    - owner 2026-10-11, shared content in Store B behind corpusd; computers hold overlays
  must_preserve:
    - Texture is the sole writer of canonical revisions; checks and judgements never edit text or mint a version
    - a published version's text, manifest hash and signatures never change (from the first Autopaper permalink on)
    - public reads never reach a paper computer; shared content and check results live in Store B behind corpusd
    - a crash closes work; no boot-time resumer of searches, checks, schedules or stories; Autopaper restarts only by the owner, by hand (§13.3)
    - support judgement by management or Jev, never research or Texture (§12.7)
    - no budgets, caps, reminders, runtime research job or runtime scoring (§1.2, §12.14)
    - one REPL owner per desk; concurrency on the host; arrivals written only by worker code under the store lock
    - research has read-only world authority; engineering never addresses Texture
    - no feature flags, shadow paths or one-release keeps (§10)
  excluded:
    - records, cursors, conversations and crash closure (C1); the tracer and trace export (C0)
    - async sub-RLMs; waking a desk on a search arrival after end_turn
    - alignment across re-captures (study E stage D); transcripts as blocks; PDF layout reconstruction
    - the edition Texture as front page, claims and scores in the reader, email
    - copyright and public quote length (set aside, §7)
    - switching the default Texture model (owner routing decision)
  protected_surfaces:
    - internal/gateway/handlers.go HandleSearch, internal/gateway/searchplane/router.go, internal/gateway/search_plane.go, internal/gateway/jev.go
    - internal/textureowner and internal/store/texture_turn.go (canonical writes, the turn frame)
    - internal/texturedoc schema and validator; internal/sourcegraph capture identity; internal/sourcefetch
    - internal/platform publish, bundle, routes and transclusions; Store B object kinds
    - internal/yaegikernel module sets, session frame and broker actions
    - internal/agentcore desk sessions, product API, source schedule wakes
  admissible_evidence: >-
    staging only: fresh test computers and the paper computer, read
    through C0's trace export, the product API, Store B objects, published
    bundles and logged-out Playwright; the C0 invariant checker on every
    tracer run. Local and race tests shape code and prove nothing deployed.
  breakage_log: docs/problems/reeng-cut2-breakage-<date>.md (one per cut window, §12.4)
  redesign_tripwire: the same layer failing three tracer runs, or a fix that needs new mutable authority, stops patching for a structural assessment (§12.5)
  conjecture_delta: >-
    from "research needs a job, Texture needs many cells, citations are
    selectors checked by presence, and a story is a new publication per
    publish" to "a truthful RLM with async search, a Texture that writes
    changed blocks citing Refs, hash-memoized checks off the author's path,
    and one publication per story with immutable versions keep published
    stories grounded as evidence changes"
  heresy_delta:
    discovered:
      - the blocking search path's only production caller is the desk tool, so the gateway's waves serve no one after this cut
      - Jev is unreachable from desks
      - changed re-captures dropped, raw bytes discarded, "[12]" refused in real pages, one anchor per entity, a quote-match comment with no matcher (R6 at 03499ef9)
    introduced: [none intended; risk: one management wake per Texture commit with new citations]
    repaired_target:
      - search waits on the slowest provider
      - research cannot read what it fetched or close its own work
      - Texture rewrites the whole document and loses ids and citations
      - quotes unchecked; support unjudged
      - a living story becomes a new publication per publish
      - the reader is signed-in only and shows fixed claims
  standing_questions: >-
    1 provenance: owner decisions §12 and §13; this file's defaults (below) are
    orchestrator proposals under the owner's delegation. 2 conformance:
    captures, checks and publications in Store B behind corpusd; no new
    store. 3 deletion citers: every removed verb, tool, overlay, route and
    type is grepped across prompts, code, tests, specs and docs in the
    same commit. 4 consumers: the blocking search route and Universal Wire
    have no consumers after the cut. 5 single authority: the revision head
    for text, the version chain for publication, records for work, the
    host table for in-flight streams (the REPL store is a copy). 6
    artifact: bundles, captures, revisions, records and screenshots, never
    narration. 7 fate-sharing: public reads depend on corpusd only; a
    corpusd outage leaves an anchor pending, never hangs a commit. 8
    restart: streams, tables and REPL stores die with the process; checks
    and schedules are re-derived only by a later turn. 9 no SSH: every
    proof through the product API and C0's export.

now:
  status: autopaper live; residual acceptance items open
  slice: >-
    Autopaper is live on choir.news (docs/archive/evidence/c2-autopaper-live-2026-10-13.json).
    The designated paper computer ran its schedules from one owner act
    (15:22 UTC, 4 sources, then 14 on the owner's tiered-rollout ruling):
    in the watched hour 54 polls brought 80 captures, management read every
    record, 26 stories published 76 versions (4 at v5+), the front list
    lists each once, and citations show 194 exact, 9 tolerant, 14 failed
    and 84 pending badges. Crash proof: a markerless restart closed all 14
    schedules, both in-flight runs and every open round; nothing polled or
    ran for 13 minutes; public reads stayed up; the owner act restarted the
    schedules and they polled at once. Unit 2a and 2b receipts:
    c2-write-acceptance, c2-judge-2, c2-paper-7. The Cut 2 freeze receipt is
    final (docs/reports/reeng-cut2-freeze-receipt-2026-10-13.md). The owner
    redirected the reader during the window: stories open in the Texture app
    (read-only published view) and the logged-out home opens Autopaper
    (cf91e617, staging spec 2 of 2). Breakages B10, B12, B12b and B13
    fixed; B11 (cell compiler panic on a slice literal over
    choir.Records()) open (docs/archive/problems/reeng-cut2-breakage-2026-10-13.md).
    The close review panel (docs/archive/reports/c2-close-review-adjudication-2026-10-13.md)
    split 5 holds-with-residuals to 2 reopen; adjudicated: C2 acceptance
    stays open on four read-only receipts owed from the live paper, E1 v5+
    versions cite captures that arrived after the previous version, E2
    published versions stay byte-identical after later publishes, E3 each
    arrival joined to the management turn that answered it with latency,
    E4 the support-failure correction loop (reported already in the
    ledger) with the 14 failed badges split by cause. Also open: the A1
    "unread source" badge (not-captured shows as pending), pending badges
    that never drain, the model comparison (transfer by amendment), the
    story-prose defects
    (docs/archive/problems/autopaper-story-prose-markdown-and-meta-2026-10-13.md,
    fixed for new revisions at d75cb193), and tier 2 and 3 sources gated on
    E3's latency table. Receipts (docs/archive/evidence/c2-close-receipts-2026-10-13.json):
    E1 pass, E2 pass, E4 pass with a gap (4 of 68 unsupported verdicts
    never repaired), E3 fail: 46 of 118 arrival items unanswered, traced
    to a cursor bug (a cell's commit marked read every record up to the
    head at cell start, shown or not;
    docs/archive/problems/autopaper-arrivals-unanswered-2026-10-13.md), fixed at
    cff54bf5 (deployed 18:25). Deploys defer busy computers whose guests
    predate the planned-restart route, and an all-deferred refresh failed
    the deploy job (fixed b4ead3e0); the paper was refreshed by operator
    onto cff54bf5 at 18:28 (crash boot closed its 14 schedules) and the
    owner act restarted them at 18:28:30. E3 rerun passed on a minted test
    computer at cff54bf5 (an 8-record, 40-capture burst: every record
    marked read was returned by a Records() call, a mid-turn arrival woke
    the next turn, 40 of 40 captures answered; receipts E3_rerun). A1
    landed at 04a7b0c2 (deployed 8a1477c3): a citation whose source was never
    captured shows "unread source"; proven by unit test, since the live paper
    holds no not-captured check (51 stories: 665 exact, 19 tolerant, 6 pending
    quote checks at 19:40). Published sources now open above the text from a
    Sources button (owner report, 7c81bbca).
  source_ref: cff54bf5
  deploy_identity: staging choir.news cff54bf5
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: C2-1
    claim: >-
      If research reads per-provider arrivals and reports as its
      understanding changes, captures are block documents, Texture writes
      changed blocks citing Refs, and quote and support checks run off the
      author's path, then published stories stay grounded at their
      permalinks as evidence changes, with no human step between capture
      and publication.
    test: unit 2b's tracer correction run (version n to a later version) and the live window, then a blind model judgement of the live stories' grounding and an owner read
    edge: missing_oracle
    delta_o: >-
      the share of research-reported quotes that land inside one extracted
      block on real outlet captures. The offline census bounds it at 91%
      of judgeable quotes on flattened text (60% exact) and finds 46% of
      quotes cite pages never captured; the real rate is measured by
      rerunning the census on 2a's tracer bundles through the captures
      layer, together with the share of citations not captured.
    scope_if_supported: AI news stories on one paper computer
    status: proposed
    evidence_refs: [docs/archive/reports/reeng-c8-quote-census-2026-10-13.md]
  decision:
    what: >-
      Two units with the freeze between them; the defaults in the prose
      ("Defaults taken"), amended by the pre-flight review: four citation
      states (matched, not found, not captured, pending), Write refuses a
      cite naming no captured block, strict one-block ellipsis matching,
      table rows as " | " blocks, the opening-Cast schema defined in 2a and
      frozen, the story-opening constraint for 2b (no owner-authored seed
      revision, no work item, no racing run), and the Universal Wire path
      deleted before the paper's first schedule run.
    kind: architecture
    status: settled (pre-flight panel 2026-10-13 under the owner's delegation)
    evidence_ref: docs/archive/reports/reeng-cut2-preflight-review-2026-10-13.md
    owner_ratification_ref: "owner 2026-10-12 delegation (§12: 'I trust your judgement')"
  belief:
    believed_state: C1 core landed (records for Texture with research and management, crash closure); normtext, quotematch and Ref exist; no block capture, Read verb, async search, Write verb, version chain or public front list yet
    main_uncertainty: whether real outlet pages extract into blocks that contain the sentences research quotes, and whether research reads arrivals without being pushed
    next_observation: (ii)'s and (iii)'s tracer scenarios on staging (a real outlet read and the census rerun; per-provider count lines in one running cell)
  blocker_or_risk: red surfaces (gateway search, capture identity, canonical writes); parallel worktrees collide on yaegikernel/choir.go and surface.go, so integration is serialized
  next_action: >-
    Landing order (pre-flight review A8): integrate (ii) and (iii) one at a
    time onto main, each with a claim-directed tracer scenario under
    --expect-sha (a research Cast that reads a real outlet page into a
    capture and reports a quote that checks; a research cell that sees
    per-provider count lines rise). Then (iv) Texture Write (with the
    opening-Cast schema and the Universal Wire hook dropped), then (v)
    support judgement, (vi) the management translation contract anytime.
    Then 2a's deployed acceptance, the freeze receipt, and 2b.

receipts: []
---

# Cut 2: evidence to publication

Cut 1 makes records the only work state. Cut 2 puts evidence through
it and out to readers. Search stops waiting on the slowest provider.
Captures become documents that stories can cite block by block. Texture
writes only what changed. Code checks quotes and management judges
support. Each published version gets a permanent link. When the cut
ends, Autopaper runs by itself and anyone can read it.

**Why two units, not one.** Decision §12.1 freezes the record schema,
block ids and reference shape at the first public Autopaper permalink.
Unit 2a proves that shape on disposable test computers: search, research,
captures, Write, checks and judgements. Unit 2b then publishes it. The
freeze receipt sits between the units, so the shape is frozen only after
the tracer has exercised every part of it. Each unit gets one deployed
acceptance, and commits inside a unit are fine.

**Defaults taken** (No Blocking Asks; each is a proposal until reviewed):

1. The gateway's blocking search mode, its waves and its grace timer are
   deleted. Their only caller is the desk tool, and the desks lose it
   (§12.6).
2. A search arrival never wakes a desk after end_turn. Arrivals wait in
   the store, and the next cell's count line shows them.
3. Support judging: the revision commit appends a record that lists the
   unjudged hash pairs and addresses it to management. Management judges
   them through a new management-only `choir.Judge` (Jev through the
   gateway) or by reasoning itself. The runtime lists the pairs; it never
   judges.
4. Auto-publish: every committed revision of a `publish: auto` document
   becomes its next version. Management sets the flag when it opens the
   story.
5. Captures reach management as one system record per poll that wrote
   new captures, with no debounce. The cursor batches them (§1.4).
6. After a crash, no desk wakes at boot and the schedule is closed.
   Autopaper stays down until the owner restarts it by hand (owner,
   §13.3).
7. Version numbers are Texture revision numbers (owner, §13.2). The bare
   link serves the latest version in place. Pre-cut publications are not
   carried over.

**Former open questions**, answered after the convergent review
(decisions §13):

- The crash-closure record never wakes management at boot. That would be
  a boot-time resumer (§13.3; Cut 1's D5 and invariant 7).
- The acceptance watches the live window for at most an hour (owner
  pace ruling 2026-10-13, metamission boundaries.pace 6); the schedule
  keeps running after the receipt. It is an observation window, not a
  runtime limit.
- The tracer's controlled page is a file on a dedicated
  `tracer-fixtures` branch of this repository, read through
  raw.githubusercontent.com. The tracer changes it by a commit to that
  branch (never main), and the receipt records each version's sha256.
  It is served independently of the paper computer and of choir.news.
- Every committed revision of a `publish: auto` document publishes.
  Texture decides publication by deciding when to commit.

Ways it can fail, written before the code:

- Arrivals race the REPL store. Only the worker's reader loop writes,
  under the lock, and the race test covers a cell reading while arrivals
  land.
- A stream outlives its session, or the 30 s client cuts it. Streams are
  bound to the session and cancelled in release and reclaim. The
  stream client has no per-attempt bound, and the gateway deadline is
  the provider timeout plus a margin.
- "M of N" counts cooling providers as failures. N is the set that was
  eligible when the stream started, and cooling providers are listed
  separately, never as degraded.
- The model ends its turn with searches pending and never reads the
  results. The store persists for the session, and a trace shows whether
  this happens before any wake is added.
- Research floods Texture with thin Reports, hedges every claim, or
  closes early. The goodharting list applies: a blind sample of claims
  is judged by someone other than research, and the owner reads the
  stories.
- The DOM walk drops or merges the paragraph a quote sits in. The
  acceptance set includes fixtures per outlet, and the extractor is
  versioned so tuning never re-addresses frozen citations.
- Tolerant tiers accept a quote the source does not say. The rendered
  quote is always the source's own span, and a tolerant match shows ≈.
- Normalization differs between cid and matching. One normtext function
  serves both.
- Keeping changed re-captures floods Store B with ad and nonce churn.
  "Changed" is decided by the block doc_hash, not the raw bytes.
- @all alignment carries a citation onto text that no longer supports
  it. A carried citation whose quote is gone is dropped and listed in
  the next frame.
- The frame for a long document outgrows the write it saves. Long
  documents get a section digest plus the recently edited and cited
  blocks; the rest are read with ReadDoc.
- Management wakes on every commit and judges the same pair twice.
  Judgements are memoized by hash pair, and management's cursor batches
  the lists.
- Judging falls behind publishing. The first version shows "pending",
  never a wait, and a failure becomes Texture's next version.
- A version's permalink changes, or its overlay leaks into the manifest
  hash. Versions are immutable, and the overlay is served live, outside
  the hash.
- The front list reads a paper computer, or floods with near-duplicate
  stories. The list is served from Store B only. Clustering stays with
  management, and the tracer counts stories per front page.
- A schedule or check resumes after a crash. The crash test asserts that
  no poll or judgement runs before the owner's manual restart.
- Management falls behind the ingestion rhythm and captures pile up
  unanswered. The live receipt shows it per poll interval; the response
  is to understand why from the trace, not to add a quota.
- The model comparison rewards speed. The judge ranks grounding first,
  and latency is reported beside it, never blended in.
- The broker handle's one-hour lifetime (choir.go:119) kills a long-lived
  research session. Cut 0 measures it and fixes it if confirmed (its
  artifact 6); the live window re-checks it.
