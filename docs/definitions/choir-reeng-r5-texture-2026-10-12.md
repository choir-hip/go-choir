---
definition_version: 4

# R5 — Texture re-engineering, then the model comparison. Station of
# definitions/choir-reeng-metamission-2026-10-12.md. Authority:
# docs/re-engineering-decisions-2026-10-12.md §1, §6, §7, §9 (owner-agreed;
# wins over the studies). Main study:
# docs/reports/re-engineering-studies-2026-10-12/D-texture-reengineering.md;
# tool surface under the unified model: E-unified-document-model.md §6.
readiness: drafted

review:
  reviewer: none
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
      No code changed between study D's ref a401c878 and fbb58c23, so its
      file:line citations hold. Re-checked here.
    - >-
      Turn time is the write, not the cell count: on the 2026-10-11 20:22Z
      demo, 12 large calls (2k+ output tokens) took 496 of 655 model-seconds
      (76%); turns average about 5 cells; "~60 cells" was the 12-turn total
      of 66 misread (study D §0.1-0.2). Texture runs
      opencode-go/deepseek-v4.1-flash at reasoning high
      (internal/modelpolicy/model_policy.go:370).
    - >-
      Whole-document writes re-mint every block id and mark every held
      source unused (internal/textureowner/tools_texture.go:785-801, uuid at
      :790); inserted blocks get fresh UUIDs (:1164, :1177) and
      insert_source_ref needs an existing block_id (:1060-1068), so new text
      and its citation cannot land in one write. Markdown links and [n]
      markers in content are refused (:935-939).
    - >-
      The wake is pointer-only ("The payloads are already bound inside your
      cell as choir.Updates()", internal/agentcore/coagent_update_packet.go:161)
      and reactivated runs carry old document memory
      (internal/textureowner/texture_controller.go:1036), so every turn
      needs a read cell. Mid-turn injection says "Treat this packet as the
      next user turn" (coagent_update_packet.go:303) while cellUpdateVisible
      hides records past the scheduled seq (internal/agentcore/tools_desk.go:520-529).
    - >-
      Prompt today: role 4,721 chars (internal/textureprompts/texture.yaml),
      run overlay 14,600 (overlays/run_system.yaml), policy tail up to 16,839
      appended to the user turn (overlays/revision_policy.yaml, via
      internal/textureowner/texture_agent_revision.go:819-822); the
      texture module set exports 15 functions incl. retired Message and Emit
      (internal/yaegikernel/choir.go:271-273). Study D §2 lists 26
      prompt/code mismatches (M1-M26); decide takes six undocumented kinds
      (tools_texture.go:122-152); work_disposition silently defaults to open
      (texture_turn_runtime.go:440-443).
    - >-
      A turn already ends at the first committed write (passivation,
      internal/agentcore/runtime.go:3509-3514), and the guest emits per-call
      tool_loop events with input/output tokens and a
      passivating_tool_success marker (internal/toolregistry/toolloop.go:604-619,
      :810). Study D measured from the Node B gateway journal, which needs
      SSH; whether those guest events reach the product API
      (choir trajectory events, cmd/choir/main.go:851) is unknown.
    - >-
      Management reports to Texture with "choir.Report, carrying its
      evidence refs" and no translation contract
      (internal/promptstore/defaults/management.yaml:21). Source targets
      already include diff_hunk, patch and test_run
      (internal/texturedoc/schema.go:376-387); selectors carry text_quote
      (internal/types/texture_provenance.go:111-118).
    - >-
      Candidate models are in the catalog: glm-5.3-flash,
      muse-spark-1.3-contributor, hy3, qwen3.7-max (opencode-go) and
      gpt-6-luna (chatgpt) (internal/modelcatalog/catalog.go:85-150).

finish:
  deliver: >-
    Texture answers each batch of research and management records in one
    small write, usually one cell: it sees the current document and the new
    records with their sources and quotes in the turn message, edits only
    the blocks that change, cites inline in the same write, and never loses
    ids or citations on a rewrite. The owner reads plain prose; built work
    appears as a clickable citation, never as code in the text. Then the
    same staged script picks Texture's model and reasoning effort from
    measurements, not impressions.
  artifact: >-
    S0 (green/yellow): a problem doc that files study D §2's mismatches
    still open after R1, with the "sixty-six cells" correction; a tracked
    trace-export script reading the per-turn metrics through the product
    API/CLI, plus a turn-commit marker if the guest events do not reach it;
    a baseline of 3 staging runs. S1 (orange): a prompt core of about 3.5k
    characters aligned with the document model; the policy tail, theory
    essay, skill context and repeated execution rules deleted; Help true
    for every listed verb; the module set cut to the verbs Texture uses.
    S2 (orange, borders red): a turn frame rebuilt every turn from R3's
    inbox (records addressed to texture:<doc> up to the turn boundary):
    the document as short-alias blocks b1..bN with citations as tokens, and
    records r1..rM with sources r2.c1.. and their quotes; no read cell
    needed; no stale document memory; mid-turn injection off for Texture.
    S3 (red): choir.Write(edits, why) with @replace/@insert/@delete/@move/
    @section/@all on aliases and inline {{cite <handle> "quote"}} tokens;
    synchronous, line-located refusals; @all reconciles against the prior
    revision so unchanged blocks keep their ids and citations; Wait, Done
    and Reject replace decide, work_disposition and update_dispositions;
    the management prompt gains the translation contract (plain summary
    plus artifact handles Texture cites); ApplyTexture deleted once Write
    is proven. S4 (orange, model routing): the comparison of
    deepseek-v4.1-flash, gpt-6-luna, glm-5.3-flash,
    muse-spark-1.3-contributor and the other opencode-go models, then a
    reasoning-effort test (high, medium, low), 3 runs each on the same
    script; the default changes only on the receipt.
  acceptance:
    - action: per stage, 3 staging runs of the fixed script (the PD1 AI-news prompt plus a follow-up), one fresh test computer per run at the deployed SHA, exported with the S0 script
      proves: calls per turn (median, p90, max), failed cells per turn by refusal class, model-seconds and output tokens per turn and the large call's share, report-to-revision latency (p50, p90), time to first revision, citations per 1k words and verified-quote share
      evidence_class: deployed proof (staging test computers)
    - action: S3 on staging, a turn that integrates two records with one Write adds a new paragraph cited inline, and an @all rewrite leaves untouched paragraphs' block ids and citations unchanged (revision JSON diff)
      proves: changed-blocks writes, same-turn citation, non-destructive rewrite
      evidence_class: deployed proof
    - action: an engineering-backed request on staging yields an owner-facing revision with no code, diff, command or log in the prose and the change reachable from a citation chip (Playwright screenshot)
      proves: Texture integrates, management translates
      evidence_class: deployed proof
    - action: S4 receipt table, one row per model and effort, plus a blind multi-perspective model judgement of the final documents (not runtime scoring)
      proves: the model and effort choice
      evidence_class: deployed proof plus model judgement
  targets_weak: "median calls/turn <= 1.5, p90 <= 4; failed cells/turn <= 0.3; report-to-revision p50 <= 45 s (study D §6); steer only, never settle complete"
  rollback: >-
    git revert per slice. S2 frame building and S3 Write ship behind one
    flag each; ApplyTexture stays as the legacy verb for one release, then
    is deleted in this station. S4 reverts the one model_policy line.
    Revisions written with Write are ordinary texturedoc revisions that the
    old code reads.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance, consensus_review]

value:
  better_means: >-
    Minimize model-seconds and failed cells from record arrival to the
    incorporating revision, while preserving Texture as sole writer, the
    head-pinned ApplyTextureTurn CAS, citations surviving every rewrite,
    and the quality of the owner-facing prose.
  goodharting_would_be: >-
    Fewer cells because Texture writes thinner documents or drops
    citations; a fast turn that skips records; a model "winner" chosen on
    speed while its documents cite less or quote wrongly; a prompt shrunk
    by moving rules into runtime refusals.

homotopy:
  realism_axis: >-
    today's surface measured (S0) -> same semantics, truthful prompt (S1)
    -> frame instead of reads (S2) -> one write verb with inline citations
    and reconciling rewrite (S3) -> models and effort compared on the
    re-engineered desk (S4) -> citations by content id and memoized checks
    when R6 lands (same tokens, new resolution).

boundaries:
  mutation_class: red (Texture canonical writes in S3; S2 changes the consumption boundary); S1 orange; S4 orange (model routing, full ceremony)
  authority_sources:
    - docs/re-engineering-decisions-2026-10-12.md §1, §6, §7, §9 (owner 2026-10-12)
    - CLAUDE.md Mutation Classes, Restarts, Landing Loop
  conjecture_delta: from "Texture is slow because it takes many cells" to "Texture is slow because it rewrites and re-reasons the whole document each turn"
  must_preserve:
    - Texture is the sole writer of canonical revisions; passing checks never mint a version (decisions §7)
    - ApplyTextureTurn head-pinned CAS and idempotent cell identity (internal/store/texture_turn.go:289-297)
    - passivation on commit; P2a dispatch fold commits with the turn
    - a crash restart closes Texture work (R2); no boot-time resume
    - engineering never addresses Texture; management carries its results (decisions §9)
  excluded:
    - budgets, the research round cap, the breaker, dead names (R1)
    - the inbox, records and their fold (R3); Write consumes them, it does not define them
    - content ids, capture blocks, the transclusion node, quote verification, permalinks (R6)
    - the Texture root desk and master document (PD3)
  protected_surfaces:
    - internal/textureowner (tools_texture.go, texture_turn_runtime.go, texture_cell_author.go, texture_agent_revision.go, texture_controller.go)
    - internal/store/texture_turn.go
    - internal/agentcore coagent delivery (coagent_update_packet.go, management_controller.go:1706-1722, tools_desk.go)
    - internal/textureprompts, internal/promptstore/defaults/management.yaml
    - internal/yaegikernel (choir.go module set, surface.go)
    - internal/modelpolicy/model_policy.go (S4)
  admissible_evidence: staging test-computer runs exported through the product API/CLI; not local runs, not the gateway journal over SSH
  heresy_delta:
    discovered: [study D M1-M26, the turn-commit marker exists in guest events but may not be exported]
    introduced: [ApplyTexture kept behind a flag for one release (deleted in this station)]
    repaired_target: [every M-row still open after R1, the pointer-only wake for Texture, citation loss on rewrite]

metamission:
  parent: docs/definitions/choir-reeng-metamission-2026-10-12.md
  depends_on: [R1 truthful prompts and deletions, R3 records as work]
  feeds: [R6 one document model]
  related: docs/definitions/choir-pd3-texture-root-desk-2026-10-11.md

now:
  status: drafted
  slice: written for owner review; nothing built
  source_ref: fbb58c23
  deploy_identity: staging b172895b
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: R5-1
    claim: >-
      If each turn's message carries the document and new records, and
      Texture writes only changed blocks with inline citations through one
      verb, then report-to-revision time and failed cells fall several-fold
      with no loss of citation density, and only then does a model
      comparison measure the models rather than the surface.
    test: S0 baseline vs S2 and S3 on the same script, 3 runs each
    edge: missing_oracle
    delta_o: per-turn metrics through the product API/CLI with a commit marker (S0)
    scope_if_supported: one Texture document per computer, the AI-news script, sequential turns
    status: proposed
    evidence_refs: [docs/reports/re-engineering-studies-2026-10-12/D-texture-reengineering.md §1, §6]
  decision:
    what: >-
      R5 lands with today's block ids; R6 swaps what the same handles
      resolve to (see "R5 and R6" below). Artifact citations land as chips
      on existing target kinds before R6's transclusion node.
    kind: sequencing
    status: proposal
    evidence_ref: docs/re-engineering-decisions-2026-10-12.md §6-7
  belief:
    believed_state: the write dominates turn time; the frame and the one verb do not exist; per-turn metrics need SSH today
    main_uncertainty: how much of the 9-14.5k-token write is reasoning vs document text (study D §1.2 is inferred from the journal, no cell trace)
    next_observation: one exported cell trace of a multi-call turn on staging
  blocker_or_risk: R1 and R3 must land first; S3 is red
  next_action: after R1 and R3 land, S0 problem doc, then probe whether choir trajectory events carry tool_loop and passivating_tool_success

receipts: []
---

# R5 — Texture re-engineering, then the model comparison

Texture spends three quarters of its model time on one large write per
turn, and most of its other cells exist to read what the turn message did
not carry, or to recover from refusals the prompt did not predict. This
station makes Texture write less and read nothing it should have been
given, with one write verb whose citations travel with the text. Only
after that does it compare models, so the comparison measures the models
and not today's surface.

**Order inside the station.** S0 documents and measures before any change
(problem documentation first). S1 changes only words and the verb list.
S2 changes what the turn carries. S3 changes how Texture writes (red, full
ceremony as listed in `boundaries`). S4 compares models and reasoning
effort on the finished desk. Each stage is measured with study D's metrics
over 3 staging runs; a stage that does not move them is evidence about the
diagnosis, not a cue to tune prompts (CLAUDE.md, understand before tuning).

**What R1 and R3 leave to R5.** R1 deletes the dead names, the budgets,
the breaker and the research round cap, so R5 never edits them. R3 makes
Texture's obligation its inbox, so S2's frame is a query over records
addressed to `texture:<doc>`, `Done` appends the closing record on the
owner's request thread, and `Reject` appends a record that names the
rejected record and why. Records arriving during a turn wait for the next
one. No timer debounce is built unless measurement shows wake storms
(decisions §1.4).

**R5 and R6: what needs content ids.** Lands with today's ids: turn-local
aliases mapped to lineage ids; changed-block ops; `@all` reconciliation,
which carries ids by aligning normalized text against the prior revision
(the study E §1.5 alignment, written once in `texturedoc` with the §1.3
normalization so R6 reuses it); `{{cite r2.c1 "quote"}}` resolving to
today's source entities with a `text_quote` selector; artifact chips on
the existing `diff_hunk`/`patch`/`test_run` targets. Needs R6: citing by
`{doc, version, cid, hash, quote}`, quote checks against capture blocks,
memoized support judgements, the transclusion node (`{{embed}}`), and
lifting the one-anchor-per-entity limit (study E §0.4). The handle syntax
is the contract between the two: R6 changes what `r2.c1` resolves to, not
what Texture writes. Before R6, verified-quote share is a weak measure
computed by the S0 export script (exact, then normalized match of each
cited quote against the record's quote and capture body).

**R5 and PD3.** PD3 (one Texture root per computer, a sub-RLM per
document, a master document) keeps `texture:<doc>` as the per-document
writer and leaves the per-document turn as it is. R5 re-engineers exactly
that turn, so it lands first and PD3's sub-RLMs inherit the frame, the
verb and the prompt. PD3's "turn latency per document" check should reuse
R5's export script. R5 neither blocks nor supersedes PD3, and it adds no
root or wake routing.

**Standing questions.** 1: owner-settled (decisions §6, §7, §9);
orchestrator choices here (chip-first artifacts, flag for one release) are
proposals. 2: conforms to decisions §6-7 and to sole writer; no verifier
writes. 3: deleting the policy tail, decide kinds, work_disposition,
update_dispositions, ApplyTexture and module-set entries requires a grep of
prompts, tests, docs and specs in the same commit. 4: Texture is the only
consumer of its surface; management is the only consumer of the
translation contract. 5: the revision head is the one authority; the frame
is rebuilt from it and from R3's records each turn and never read back. 6:
revision JSON, exported traces and screenshots, not run narration. 7: the
frame reads the store already on the turn path; no new dependency. 8: a
crash closes the turn and the next frame is rebuilt from the head, never
from run memory. 9: the metrics leave through the product API/CLI, not
journalctl.

Ways it can fail, written before the code:

- aliases drift inside a turn when a cell's own edit shifts block order
  (aliases are fixed at frame build; a Write resolves all of them before
  applying any op);
- `@all` alignment carries an id onto a changed paragraph, so a citation
  stays on text it no longer supports (carry ids only above a similarity
  floor; a carried citation whose quote no longer appears in its block is
  dropped and listed in the next frame);
- a large document makes the frame larger than the write it saves (long
  documents get a section digest plus recently edited and cited blocks in
  full, the rest by `choir.Doc().Block`);
- located refusals are still too vague and Texture retries the whole
  write (the failed text is kept in a variable; the refusal names the line
  and the valid aliases);
- `Done` or `Reject` leaves an R3 thread open or closes one twice
  (both are records in R3's fold; S3 acceptance checks the fold after the
  turn);
- the prompt core is shrunk by moving rules into refusals, so refusals
  rise (S1's failed-cells metric catches it);
- management's translation drops the evidence Texture needs to cite
  (every translated report carries artifact handles; a report without one
  is refused at Report time, not discovered in Texture);
- the model comparison rewards speed over grounding (the S4 receipt ranks
  on verified-quote share and citation density first, then latency);
- the export script reads events the guest emits but the API does not
  return, and the metrics silently omit turns (S0 reconciles the turn
  count against revision count per run).
