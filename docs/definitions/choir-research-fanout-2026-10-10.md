---
definition_version: 4

# Research fan-out: research is deep and comprehensive by default, reports to
# Texture as it goes, and recursively fans out to child research activations
# on several models (the RLM pattern). Owner 2026-10-10: "research should be
# deep and comprehensive by default, and report frequently back to texture.
# and it can recursively call itself (rlm pattern) to fan out to multiple
# models"; "we probably need a research-only task ... we can pause working
# on arxiv task until our research only task is good ... compare the results
# to a multi-haiku deep research on the same topic".
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
      Settled design (docs/reviews/rlm-implementation-consensus-2026-08-29.md
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
    of child specs (objective, model from an allowlisted catalog, budget
    share) and receive each child's findings and sources as return values
    in the same cell; children run in parallel, read-only, with no
    messaging verbs and a depth cap, inside the parent's egress budget and
    deadline, each receipted under the parent.
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
        A run whose children exceed their budget or deadline, and a child
        that tries a messaging verb or a further nested call past the cap.
      proves: >-
        Budget, deadline, depth and authority bounds hold; a refused child
        returns an error value, never a hang.
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
    out-of-window or unconfirmed items; children that are the same model
    repeating the same searches; revisions that churn wording without new
    evidence; reports split into fragments to raise a report count.

homotopy:
  realism_axis: >-
    prompt-only depth in one activation -> choir.Research with parallel
    children on the desk's own model -> children on several models from the
    catalog -> depth-2 recursion (a child plans grandchildren) -> an owner
    research request on a real computer.

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
    - children share the parent's egress budget and deadline; depth cap 2
    - every child call is receipted with a link to the parent activation
  exclusions:
    - no change to Texture's control path or management
    - no bare models.Call (dataflow recursion) in this mission; separate if needed

now:
  status: drafted
  slice: >-
    S0: measure the prompt and display fixes alone on staging before
    building fan-out.
  candidate: none
  conjecture: >-
    If research fans its subtopics out to parallel child activations on
    several models and reports after each subtopic, then the week-of-AI-news
    document reaches the Haiku baseline's in-window item count, with correct
    dates and working links, within 15 minutes, with a revision every few
    minutes. EDGE missing_oracle: item correctness needs a checker
    (ΔO: scripted item extraction plus link spot checks on both reports).
    Status: proposed.
  decision: >-
    Actor recursion as a synchronous host verb (choir.Research) over the
    existing model loop, not new runs: it matches M6.5 (nested activation,
    subset manifest, budget carve-out) and the 2026-09-15 return-value rule
    without new lifecycle objects.
  belief:
    believed_state: >-
      Depth is limited first by kernel ergonomics (display, timeouts) and
      the prompt's missing "continue after a report", then by one
      low-effort model doing all the searching serially.
    main_uncertainty: >-
      Whether the prompt fix alone produces sustained depth from a
      low-effort model, which decides how much fan-out must carry.
    next_observation: >-
      The S0 rerun's research runs: duration, searches, reports per run.
  blocker: none
  next_action: >-
    After a114b46e deploys, restart demo-arxiv-2026-10-10c and rerun the
    research-only prompt on a fresh document; pull traces and compare.

receipts: []
---

# Research fan-out

Research today stops after its first report, usually within a minute. The
owner wants it deep by default, reporting to Texture as it goes, and able to
fan out to several models the way the RLM design intends. This file carries
that mission from the first measurement (prompt and display fixes alone)
to a deployed `choir.Research` verb, judged against a five-Haiku baseline
on the same prompt.
