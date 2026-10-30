---
definition_version: 4

# Research fan-out: research is deep and comprehensive by default, reports to
# Texture as it goes, and recursively fans out to child research activations
# on several models (the RLM pattern). Owner 2026-10-10: "research should be
# deep and comprehensive by default, and report frequently back to texture.
# and it can recursively call itself (rlm pattern) to fan out to multiple
# models"; "we probably need a research-only task ... we can pause working
# on arxiv task until our research only task is good ... compare the results
# to a multi-haiku deep research on the same topic"; "we dont really care
# about the recursion depth or number of rlm agents used. just the result.
# bitter lesson, remember. and different models in the desk will probably
# have different behavior so we dont want to overfit a prompt or model".
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-10T23:20:00Z
  source:
    canonical_ref: main a114b46e
    deploy_identity: staging choir.news 37498792 (a114b46e in CI)
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean at a114b46e apart from this file and the baseline evidence
      class: intentional source
      owner: this session
      touch: allowed
      recovery: git
  observed:
    - >-
      All eight research runs in the saved demo traces lasted 35-63 s with
      at most four web searches, ended by the model's own end_turn right
      after a report listing open gaps
      (problems/research-runs-stop-after-first-report-2026-10-10.md).
      The desk runs gpt-5.6-luna at reasoning effort low.
    - >-
      Research-only demo on 37498792 (demo-arxiv-2026-10-10c, document
      a4a9b020): two research runs of about one minute, five searches each.
      Research read `r; err` as a nil search result because the kernel
      showed only the last value, reported every source dead, and Texture
      escalated a false outage
      (problems/cell-shows-only-last-value-research-reads-results-as-nil-2026-10-10.md;
      fixed in 431ca252, not yet deployed).
    - >-
      Baseline: five Claude Haiku researchers plus a Haiku writer produced a
      5,765-word report with 26 detailed in-window items in about 8 minutes
      (evidence/ai-news-haiku-baseline-2026-10-10/).
    - >-
      Research has no recursion verb. Its module set
      (internal/yaegikernel/choir.go deskModuleSets) is messaging plus
      read-only world verbs. The host has a reusable model loop
      (toolregistry.RunToolLoop with WithToolLoopLLMConfig and
      WithToolLoopBudget) and a per-activation egress ledger
      (researchtools.NewEgressBudgetLedger).
    - >-
      Settled design (docs/archive/reviews/rlm-implementation-consensus-2026-08-29.md
      D7b and M6.5): dataflow recursion is a bare models.Call (no tools);
      actor recursion is a nested activation with a manifest that is a
      subset of the parent's grants, a carve-out of the parent's budget,
      depth cap 2-3, child receipts linked to the parent, and forced death
      of the whole tree with the worker process group. A Researcher nests
      Researcher-or-smaller freely within its budget. The 2026-09-15
      ontology consensus adds that synchronous sub-RLM results are return
      values inside the activation, never mailbox deliveries (deadlock).
      Neither recursion is implemented.

finish:
  deliver: >-
    When the owner asks Choir a research question, the document fills in
    within minutes and keeps deepening: research plans the question's
    subtopics, fans them out to child research activations on several
    models, and reports to Texture after each subtopic, so Texture revises
    the document every few minutes until research saturates.
  artifact: >-
    A deployed research desk on staging that (1) is deep by default and
    reports per subtopic (prompt); (2) can call choir.Research with a list
    of child specs (objective, optional model from the catalog) and
    receive each child's findings and sources as return values in the same
    cell; children run in parallel, read-only, with no messaging verbs, and
    may themselves call choir.Research; the whole tree runs inside the
    parent's egress budget and deadline. How deep or wide the model goes is
    its own choice.
  acceptance:
    - action: >-
        Rerun the research-only demo ("Research the news on and around AI
        for the last week.") on a fresh document after 431ca252 and
        bed62a03 deploy, without fan-out.
      proves: >-
        How far the prompt and display fixes alone move depth and
        report cadence (the S0 measurement).
      evidence_class: deployed proof (test computer)
    - action: >-
        Same prompt with choir.Research live. Compare the final document to
        evidence/ai-news-haiku-baseline-2026-10-10/report.md: distinct
        in-window items with correct dates and working source links (a
        scripted extraction plus a spot check of 10 links per side), time
        to first grounded revision, number of revisions, and research
        reports per run.
      proves: >-
        The fan-out reaches baseline coverage and reports frequently, on
        the product path.
      evidence_class: deployed proof (test computer)
    - action: >-
        A run whose children exhaust the shared budget or the deadline, and
        a child that tries a messaging verb.
      proves: >-
        Budget, deadline and authority bounds hold; a refused child returns
        an error value, never a hang.
      evidence_class: deployed proof plus focused tests
  rollback: >-
    Revert the fan-out commits; the research prompt change is independent
    and reverts on its own. choir.Research absent from the module set
    leaves research exactly as it was.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between Choir's document and the five-Haiku baseline
    in correctly dated, cited in-window items, and the time from prompt to
    first grounded revision, while preserving research's read-only
    authority, Texture as the sole writer, the egress budget and the
    activation deadline.
  goodharting_would_be: >-
    Counting items without checking dates or links; padding the week with
    out-of-window or unconfirmed items; revisions that churn wording without
    new evidence; scoring the mechanism (children, depth, models, report
    count) instead of the document; a prompt tuned to one model or to this
    one prompt.

homotopy:
  realism_axis: >-
    prompt-only depth in one activation -> choir.Research with parallel
    children (any depth, the model's choice) -> the same on other desk
    models and other research prompts -> an owner research request on a
    real computer.

boundaries:
  authority: >-
    Owner direction 2026-10-10 (quoted in the header); the 2026-08-29 RLM
    consensus D7b/M6.5 and the 2026-09-15 ontology finding are the settled
    design. No new authority: children hold a subset of research's grants.
  mutation_class: >-
    red for choir.Research (new provider/model calls and egress budget
    carve-out); yellow for prompt text; orange for kernel display.
  protected_surfaces:
    - provider and model routing (children's model selection, allowlisted)
    - egress budget ledger
    - research read-only authority and Texture sole-writer rule
  invariants:
    - a child never holds Report, Message, Cast, Escalate or any write verb
    - a child's results return as values in the parent's cell, never by mailbox
    - the whole tree shares the parent's egress budget and deadline; no
      structural cap on depth or width (owner 2026-10-10: the result is the
      measure)
    - prompts state the desk's job in general terms, not tuned to one model
      or one task
  exclusions:
    - no change to Texture's control path or management
    - no bare models.Call (dataflow recursion) in this mission; separate if needed

now:
  status: superseded
  superseded_by: definitions/choir-pd1-persistent-research-desk-2026-10-11.md (owner 2026-10-11: one persistent research desk per computer, rounds not recursion)
  slice: >-
    S1c: rerun the research-only prompt with children paced under the
    computer's model budget, 429s waited out, and the time carve-out (a
    desk's Research call returns within 10 minutes; children get three
    quarters of their parent's remaining time).
  candidate: >-
    3faa001a (pacing + Retry-After) + c1da4e97 (time carve-out), pushed
    2026-10-11 ~01:00.
  conjecture: >-
    If children run on served models, search stays up under fan-out, and
    the parent can read every child's findings, then two research desks
    reach the Haiku baseline's in-window item count, with correct dates and
    working links, within 15 minutes. EDGE missing_oracle: item correctness
    needs a checker (ΔO: scripted item extraction plus link spot checks on
    both reports). Status: proposed.
  decision: >-
    Actor recursion as a synchronous host verb (choir.Research) over the
    existing model loop (M6.5 nested activation, subset manifest, shared
    budget; 2026-09-15 return-value rule); no depth cap (owner 2026-10-10).
    Texture's per-request research budget (2 desks) is left as is until the
    S1b rerun shows whether working fan-out makes two desks enough
    (problems/texture-research-opener-dropped-silently-2026-10-11.md).
  belief:
    believed_state: >-
      S1 (0b7250ef, document ee7e8c2b): research used choir.Research
      unprompted in both desks. Three luna children returned findings
      (61-125 s); three gpt-5.4-mini children failed on the provider (model
      named from the catalog list in a refusal). Two narrow empty searches
      benched parallel, brave and tavily for 5 min. The parent saw only the
      first 4,000 characters of its children's findings and printed
      searches as byte values. Document: 2 in-window items, both correct
      and in the baseline, first grounded revision at 4 min 12 s, then a
      third research request dropped by the per-request budget.
    main_uncertainty: >-
      Whether two desks with working fan-out reach baseline coverage, and
      whether the 64-call egress budget caps a desk's tree.
    next_observation: >-
      S1b: child models and outcomes, search provider health during the
      run, items reported per desk, revisions over time; then the item
      comparison against the baseline.
  blocker: none
  next_action: >-
    After the push deploys, restart demo-arxiv-2026-10-10c and run the
    research-only prompt on a fresh document with no pushes during the run.

receipts:
  - id: S1
    at: 2026-10-11T00:01:54Z
    build: staging 0b7250ef
    action: research-only prompt on demo-arxiv-2026-10-10c, document ee7e8c2b
    result: >-
      2 in-window items (Mistral Large 4, Oct 6; OpenAI GPT-6, Oct 7), both
      sourced to the vendor page and both in the baseline (26 items). First
      grounded revision 4 min 12 s after the prompt; 2 agent revisions; one
      report per research desk; document stalled at 00:06.
    findings:
      - problems/research-child-picks-unserved-model-2026-10-11.md (fixed 8aa8559b)
      - problems/search-plane-empty-result-benches-healthy-providers-2026-10-11.md (fixed 4d365e47)
      - problems/research-parent-cannot-read-its-children-2026-10-11.md (fixed 072b8612, 6906a1ae)
      - problems/texture-research-opener-dropped-silently-2026-10-11.md (open decision)
  - id: S1b
    at: 2026-10-11T00:37:47Z
    build: staging ce03d075
    action: research-only prompt on demo-arxiv-2026-10-10c, document 211dfcb6
    result: >-
      Failed. Both desks called choir.Research with four children; children
      recursed. 919 child activations ended by 00:58, 114 with findings;
      the gateway rejected 8,236 of this computer's calls and admitted 1,122
      (60-a-minute budget). Neither desk's Research call returned in 20
      minutes; the document stayed at its framing revision.
    findings:
      - problems/research-fanout-overruns-computer-model-budget-2026-10-11.md (fixed 3faa001a, c1da4e97)
---

# Research fan-out

Research today stops after its first report, usually within a minute. The
owner wants it deep by default, reporting to Texture as it goes, and able to
fan out to several models the way the RLM design intends. This file carries
that mission from the first measurement (prompt and display fixes alone)
to a deployed `choir.Research` verb, judged against a five-Haiku baseline
on the same prompt.
