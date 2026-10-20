---
definition_version: 4

# Overnight run, 2026-10-20 to 2026-10-21. Run as
# `/goal docs/definitions/choir-overnight-research-oped-sources-2026-10-20.md`.
# Owner 2026-10-20 ~23:30 UTC (docs/reports/owner-notes-2026-10-14.md
# section 32): research more for every reader, op-eds as connected Texture
# publishing, wave-1 schedules plus AT Protocol and Fediverse ingestion, and
# the self-development comparison (Sonnet through the Claude delegate versus
# the default GPT-6 Luna) after the failure 15 carried patch. The LP S6 commit
# gate (docs/definitions/choir-lp-s6-commit-gate-2026-10-20.md) stays the
# active LP station; this file is the owner's overnight order beside it and
# does not change its slices.
readiness: drafted

review:
  reviewer: none yet
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-20T23:55:00Z
  source:
    canonical_ref: main 295e5c79d
    deploy_identity: >-
      staging https://choir.news at 43b5bc034 (Dolt memory raise during the
      backup's platform sync); 194fd31c9 (snapshot path) failed the deploy
      disk preflight, 89.75 of 90 GiB free
  worktrees:
    - path: session scratchpad backup-wt (branch ops/offhost-backup-path, tracks origin/main)
      status: this file and owner notes section 32
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: branch
    - path: /Users/wiz/go-choir (main, 70 behind origin; untracked sourcecycled, ograph-bench, cmd/desktop/frontend)
      status: other_agent_wip for the untracked paths
      class: other_agent_wip
      owner: not this session
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      Research more exists, owner-only and dark. The button
      (frontend/src/lib/AutopaperExpand.svelte, mounted at
      AutopaperRoute.svelte:95), POST /api/platform/publications/expand and
      the fate-line GET (internal/proxy/platform_expand.go:355,588) are on
      main. The gate is expandOwnerSession (:239) plus the allowlist
      PROXY_EXPAND_REQUESTERS, empty in nix/node-b.nix:773. Limits are in
      memory only: 3 in flight, 12 a day, one request per published version
      (durable through the owner revision id). The proxy has no per-address
      throttle. The request text asks only for more outlets (:73). The
      deployed spec (frontend/tests/autopaper-expand-deployed.spec.js) has
      never run.
    - >-
      Research's search reaches the gateway search plane: free tiers
      keenable, searxng, brave; paid serper, parallel, exa, tavily, serpapi
      with daily caps (internal/gateway/searchplane/config.go:116-117).
      NewsAPI.ai is not integrated anywhere.
    - >-
      Op-ed pieces: any signed-in user can write in Texture and publish
      (TextureToolbar Publish, POST /api/platform/texture/publications,
      platform_publish.go, PublishMode owner) to an unlisted public link
      /pub/texture/<slug>. The paper front lists only auto-mode stories of
      the CORPUSD_AUTOPAPER_OWNERS allowlist (publication_front.go:148-156).
      Research's SearchWireCorpus searches only its own computer's corpus
      (researchtools.go:39-125); Service.SearchPublished
      (service_publication_read.go:625) searches Store B publications but
      scans only the first 100 and no research verb calls it. No byline or
      opinion label exists. A published article is not a capture, so it
      cannot be cited yet.
    - >-
      Sources: the 11 wave-1 rows are in internal/sources/registry.json and
      unscheduled; scheduling is an owner act through GET/POST
      /api/autopaper/sources/schedules on the paper with the paper
      credential (docs/problems/primary-source-registry-wave-1-2026-10-20.md
      section 6). Pollers are rss, telegram, gdelt. No AT Protocol or
      Fediverse code exists. Bluesky per-account RSS
      (bsky.app/profile/<handle>/rss) and Mastodon tag and account RSS work
      as plain rss rows (docs/reports/multilingual-osint-sourcing-research-2026-10-13.md).
      Mastodon /api/v1/trends/links is public and carries a per-day uses
      history; Bluesky getAuthorFeed, getFeed, getList and getTrends answer
      unauthenticated (web research, 2026-10-20, session notes).
    - >-
      Self-development: failure 15 problem 1 (a retry starts in a fresh
      capsule and loses the earlier diff) is open with option A owner-chosen
      (docs/problems/engineering-completion-path-cluster-2026-10-20.md);
      problem 2 (empty source patch frozen) is fixed in 22b287a5d. Failure 12
      (go vet stalls on module downloads despite the host module cache) is
      untraced. The Claude delegate token stays installed; the allowlist was
      emptied on 2026-10-15 (431dc5d3b) and section 28 lifted the rule for
      the engineering desk with Sonnet.

finish:
  deliver: >-
    A reader of choir.news, signed in or not, presses Research more on a
    story and later sees a new version with more sources. A signed-in user
    writes an opinion piece in Texture with research that can find and cite
    published Autopaper stories, publishes it, and it appears on the paper
    labelled Opinion with the author's name. The paper ingests the wave-1
    primaries and a first set of AT Protocol and Fediverse leads chosen from
    aggregating seed accounts. A frontend app rewrite has run through
    self-development on test computers twice, Sonnet through the delegate
    and GPT-6 Luna, with a side-by-side record.
  artifact: >-
    (1) Research more public: an anonymous door on the expand route, a
    per-address and per-story throttle beside the existing caps, the
    request text naming web search, deployed and proven on one story.
    (2) Op-ed: research's corpus search gains published Autopaper stories
    and can cite one by capturing its export text; an opinion publish mode
    listed in an Opinion section with byline and label, behind a kill
    switch; deployed and proven with a test account.
    (3) Sources: wave-1 scheduled (TLS test first for the Taiwan row); a
    seed set of Bluesky and Mastodon RSS rows chosen by an aggregator-first
    rule written in a short design note with the pivot method; the Mastodon
    trends-links poller as slice 2 if time remains.
    (4) Self-development: the carried patch (option A) built test-first and
    deployed; the two-arm drill on the smallest frontend app with a report.
    (5) Failure 12 traced to a named cause in its problem doc.
  acceptance:
    - action: Logged out, press Research more on a low-importance story on choir.news; wait for the fate line; open the new version.
      proves: The anonymous door reaches Texture and research and a revised version publishes with added sources.
      evidence_class: deployed proof (frontend/tests/autopaper-expand-deployed.spec.js extended to the logged-out path)
    - action: Repeat from the same address past the throttle; a second request on the same version.
      proves: The throttle and the one-request-per-version rule refuse with a visible reason.
      evidence_class: deployed negative proof
    - action: As a test account, write a Texture piece, ask research for context, publish as opinion; load the paper front and the piece logged out.
      proves: Research cites a published Autopaper story; the piece is listed under Opinion with byline and label; the kill switch hides it.
      evidence_class: deployed proof
    - action: GET the paper's schedules after one interval.
      proves: Each new row is active with zero failures and captures by source id.
      evidence_class: deployed proof (operator read, states and timings only)
    - action: The two-arm drill on test computers at the deployed commit.
      proves: Each arm reaches frozen, approved, applied, or names where it stopped; the carried patch reappears in a retry.
      evidence_class: deployed proof (tracer receipts)
  rollback: >-
    Each slice is its own commit. Research more public sits behind
    PROXY_EXPAND_PUBLIC (off restores owner-only). Opinion listing sits
    behind its own switch; owner-mode links keep working. Schedules stop by
    DELETE with a reason; rows stay inert. The carried patch is additive
    and revertable. Test computers only for self-development.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the steps between a reader's wish (know more, say something,
    see more of the world) and a published, sourced, labelled result on
    choir.news, while preserving Texture as the sole canonical writer, two
    distinct outlets per news story, social posts as leads rather than
    outlets, the gateway's key custody and daily caps, and test-tier-only
    self-development.
  goodharting_would_be: >-
    A button that queues a request no desk acts on; a new version that
    rewrites without new sources; an op-ed listed without research ever
    finding a published story; rows added to the registry but never polled
    or polled and silent; a drill counted as a comparison when one arm
    never started; a local test called the proof.

homotopy:
  realism_axis: >-
    One reader path at rising realism: owner-only door (today) -> anonymous
    door on one story -> anonymous door with the throttle under repeat ->
    every story. Op-ed: unlisted link (today) -> research finds a published
    story -> listed under Opinion -> labelled with byline and kill switch.
    Sources: registry rows -> one guest poll -> standing schedules -> seeds
    chosen by rule -> seeds grown by pivot. Each step is the production path
    on staging.

boundaries:
  mutation_class: red
  per_slice_class:
    research_more_public: "red (host-to-guest owner bridge paperCall; auth surface)"
    oped_search: "orange (research tool, corpusd read through the proxy)"
    oped_publish_mode: "red (publication mode chosen by route; public listing)"
    sources_schedules: "orange (owner act on the paper; no code)"
    sources_rows: "orange (embedded registry)"
    trends_poller: "orange (new source type)"
    carried_patch: "red (engineering capsule and freeze path)"
    selfdev_drill: "orange (test computers only)"
  authority_sources:
    - owner notes sections 28, 30, 32 (2026-10-20)
    - docs/choir-doctrine.md, docs/agent-product-doctrine.md, CLAUDE.md
  must_preserve:
    - Texture is the sole writer of canonical document revisions.
    - A news story needs two distinct outlets; social posts never count as outlets.
    - Keys stay in the gateway; daily caps hold for anonymous triggers.
    - Self-development only on test computers; never the paper or the owner's computer.
    - One deploy at a time, each verified on staging before the next.
    - The 02:40 UTC backup is not crowded by heavy drills.
  excluded:
    - NewsAPI.ai provider (a separate gateway slice)
    - Jetstream websocket consumer (polling first)
    - reader corrections (follow op-eds, section 30)
    - moderation beyond the kill switch and throttle
    - briefing questions 6 to 12
  ceremony: >-
    Red slices carry conjecture delta, rollback and heresy delta in their
    commit or problem doc. Failure modes are written before code. Problem
    documentation first for anything new staging shows.

heresy_delta:
  discovered: >-
    The expand limits live in proxy memory and reset on restart; the expand
    door impersonates the story owner through paperCall; SearchPublished
    scans only the first 100 publications.
  introduced: none
  repaired: none

now:
  status: working
  slice: >-
    0. Finish the backup snapshot path (deploy 194fd31c9 after disk, migrate
    the two Dolt data directories, supervised run 4, restore drill). Then
    1. Research more public; 2. op-ed search and opinion mode; 3. sources;
    4. carried patch then the two-arm drill; 5. failure 12 trace. Branches
    are built in parallel, landed one deploy at a time.
  source_ref: main 295e5c79d
  deploy_identity: staging 43b5bc034
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: ON-C1
    claim: >-
      The reader features are mostly connection, not construction: opening
      the existing expand door, wiring the existing published search into
      research, and adding one publish mode gives readers research more and
      op-eds without a new pipeline.
    test: >-
      The deployed logged-out expand spec and the op-ed spec pass with only
      the gate, throttle, search wiring and publish-mode changes.
    edge: missing_oracle
    delta_o: >-
      The deployed specs plus the fate line and the version's added sources;
      without them, a published version cannot be told from an unrelated
      Texture revision.
  decision: >-
    Throttle defaults (conservative, owner said not to worry about abuse):
    per address 3 an hour and 10 a day, the existing 3 in flight and 12 a
    day overall, one per published version. Opinion listing needs a
    signed-in account and sits behind a kill switch.
  belief:
    believed_state: backup run 3 in restic check; reader features mapped; sources mapped.
    main_uncertainty: >-
      Whether the paper credential is available for schedules tonight
      (asked); whether Texture expands or rewrites on a research request
      (hazard F1, untested).
    next_observation: run 3 result, free disk, then the deploy re-run.
  blocker: none
  next_action: Record run 3; free disk to the deploy floor; re-run the deploy.

receipts: []
