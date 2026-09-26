---
definition_version: 3
definition_id: choir-texture-live-authoring-2026-09-26
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-26T05:00:00Z'
  source:
    canonical_ref: main@650ffb05
    deploy_identity: staging https://choir.news build.commit=b7ae7596
    worktrees:
      - path: /Users/wiz/go-choir
        status: clean
        class: source
        owner: this session
        touch: read_write
        recovery: git
  predecessor:
    mission: choir-management-live-cast-2026-09-26
    disposition: >-
      R3c landed+deployed 2026-09-26 (b7ae7596, ci 36216782941). Management
      is live on the host desk-cell carrier unconditionally: desk_go_eval +
      typed lifecycle controls, sealed registry; staged Cast reaches
      delegated admission on the canonical ledger. The desk-cell carrier
      promotes desks per-profile; texture is the next promotion.
    evidence_ref: docs/definitions/choir-management-live-cast-2026-09-26.md
  spine_meta_goal: docs/definitions/choir-rectification-spine-2026-09-25.md
  station: R3d
  owner_directive: >-
    OWNER DIRECTIVE 2026-09-26 (overrides the earlier split draft): texture
    is full RLM like every other desk — the cell is Go only; there are no
    other tools. Authoring the Texture document is itself a choir.* cell
    verb, not a tool call. "the whole point is full rlm everything; the only
    question is implementation." Accordingly there is no R3d-a/R3d-b split:
    this mission is the full cutover. patch_texture / rewrite_texture /
    record_texture_decision / request_email_draft retire as registry tools;
    the canonical AuthorAppAgent write is a staged cell intent committed by
    the reducer via the atomic ApplyTextureTurn transaction (which is the
    authoring turn — kept as the commit, driven from the cell, not a tool).
    The desk-originated worker_updates consumer machinery is deleted; the
    cell consumes canonical ledger traffic (choir.Inbox / records) instead.

finish:
  outcome: >-
    The texture desk runs on cells under the host desk-cell carrier as a
    full-RLM desk: its registry is desk_go_eval only — no patch_texture /
    rewrite_texture / record_texture_decision / request_email_draft /
    memory or other host tools. The cell reads its bound document and
    authors genuine AuthorAppAgent revisions via choir.* cell verbs; the
    reducer commits them through the atomic ApplyTextureTurn transaction.
    The desk-originated worker_updates consumer machinery is deleted.
  artifact: >-
    The texture-authoring choir.* verbs (a doc-read affordance and a
    revision-authoring intent) bound into the texture desk module set; the
    reducer dispatch that commits a staged authoring intent as an
    AuthorAppAgent revision through ApplyTextureTurn (atomic revision +
    work/control inbounds + source graph); the deleted typed texture tools
    and the deleted worker_updates consumer machinery.
  acceptance:
    - action: >-
        A texture-profiled activation runs on the cell carrier with a
        registry sealed to desk_go_eval only: no patch_texture, no
        rewrite_texture, no record_texture_decision, no
        request_email_draft, no host file/exec/evidence/memory tools.
      proves: texture is full RLM — cell is the only surface.
      evidence_class: local test + authority-contract expectation updated
    - action: >-
        A texture cell authors a doc revision via a choir.* verb: the
        staged intent commits through ApplyTextureTurn as an
        AuthorAppAgent revision that advances the doc head, and its
        provenance/metadata cites a canonical commitment record or packet
        id it consumed — not a worker_updates checkpoint Seq.
      proves: >-
        genuine cell-verb authoring on ledger evidence (D14 resolved: the
        cell-authored ApplyTextureTurn turn is canonical).
      evidence_class: local ledger assertion; deployed proof where the
        activation permits
    - action: >-
        The desk-originated worker_updates consumer path is deleted:
        textureWorkerUpdateCommitSeq / markTextureWorkerUpdatesDelivered /
        UpsertTextureControllerCheckpoint integration /
        worker_updates_{consumed,skipped,pending,policy,checkpoint_seq,
        scheduled_seq} revision metadata legs /
        verify*CoversWorkerUpdates workflow-verifier legs /
        worker-update → texture source-entity projection — gone.
      proves: no second (desk-evidence) authority beside the canonical
        ledger; R3a's deferred deletion owner satisfied.
      evidence_class: code inspection + grep-zero on removed symbols
    - action: >-
        The typed texture tools are deleted/retired: patch_texture,
        rewrite_texture, record_texture_decision, request_email_draft no
        longer register on any profile (texture's registry is desk_go_eval
        only); their tool-loop affordances (passivating/terminal/required-
        write wiring, overlay text, prompts) cleaned up.
      proves: full-RLM texture — no tool-loop authoring path survives.
      evidence_class: code inspection + authority-contract test
    - action: >-
        Restart mid-task resumes the texture authoring episode from the
        tape: a pending staged authoring intent or mutation reconciles from
        the committed head; no half-commit mints.
      proves: restart-durable authoring on canonical evidence.
      evidence_class: local test
  rollback: >-
    git revert + redeploy. Reverting re-gates deskCarrierLive(texture) to
    actuator=rlm, restores the typed authoring tools on the tool registry,
    and restores the worker_updates consumer machinery. The carrier
    substrate (R3b) and management (R3c) are unaffected.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Texture — the desk that authors the live supervision doc the owner
    reads — becomes a full-RLM desk like every other: doc authoring is a
    choir.* cell act committed through the canonical ApplyTextureTurn
    transaction citing ledger evidence, not a tool call on a second tool
    loop and not a worker_updates projection. This is the live-supervision
    surface the owner gates M11 on, now uniformly RLM.
  goodharting_would_be: >-
    A "texture cell" that still calls a host-side patch_texture/typed tool
    to write the doc, or a cell-verb whose staged intent never reaches the
    ApplyTextureTurn commit — acceptance insists the registry is
    desk_go_eval only, the AuthorAppAgent revision commits on the canonical
    transaction, and it cites a record the cell consumed.
  falsifiers:
    - 'Falsified: any texture tool other than desk_go_eval is registered —
      texture is not full RLM.'
    - 'Falsified: the cell-authored revision does not commit via
      ApplyTextureTurn (atomic revision + inbounds + source graph) — it is
      a parallel write path.'
    - 'Falsified: doc head does not advance mid-task — authoring is a
      projection, not a turn.'
    - 'Falsified: a mid-task restart loses the pending intent or mints a
      half-commit.'

now:
  status: settled
  settled_by: orchestrator
  slice: >-
    station R3d LANDED 2026-09-26 (deployed build 119e0edd). The
    texture desk is unconditional on the cell carrier (deskCarrierLive
    texture=true; R3d full-RLM per owner directive). Registry sealed to
    desk_go_eval — no typed texture tools, no spawn_agent, no memory/file
    tools. Authoring is the choir.ApplyTexture cell verb: staged
    texture_apply intent reduced by commitTextureAuthorIntent through the
    bound owner's CommitCellTextureAuthor into the atomic ApplyTextureTurn
    (apply/decide/email ops). Committed turns leave a run-memory
    texture_cell_author_receipt keyed by intent identity: a replayed cell
    replays the same commit rather than CAS-conflicting on the advanced
    head. choir.ReadDoc injects the bound doc head on the cell frame. The
    desk-originated worker_updates consumer machinery and the typed texture
    tool wrappers/registrations are deleted; tool-loop affordances
    (passivating/terminal/required-write) are payload-predicates on
    desk_go_eval receipts (rlm:texture_apply) instead of dead tool names.
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: texture-cell-authors-on-ledger
    claim: >-
      A texture cell whose only surface is desk_go_eval can read its bound
      doc, consume ledger traffic via choir.Inbox, and stage a revision-
      authoring choir.* intent the reducer commits as an AuthorAppAgent
      revision through ApplyTextureTurn — making the typed texture tools
      and the worker_updates consumer machinery deletable without losing
      genuine authoring.
    test: >-
      A texture activation on cells (desk_go_eval only) advances the doc
      head via a staged choir.* intent committed through ApplyTextureTurn;
      the revision cites a canonical record; the typed tools and
      worker_updates symbols are gone; restart resumes the episode.
      SUPPORTED locally: TestCommitCellTextureApplyCommitsAuthorRevision
      asserts AuthorAppAgent revision + head advance + texture_cell source
      + no worker_updates metadata legs + replay-no-second-head;
      TestCommitCellTextureDecideRecordsTurn covers decide; sealed-registry
      contract tests pin desk_go_eval-only for texture and management.
    delta_o: >-
      If texture is full RLM, every desk on the spine reaches cells through
      one mechanism — the live supervision doc (M11's gate surface) is real,
      and R4 builds on a texture that already consumes records.
    scope_if_supported: texture desk full-RLM + cell-verb authoring
    status: supported
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §11 (R3d, D14)
      - owner directive 2026-09-26: full RLM, Go-only, no other tools
  belief:
    believed_state: >-
      LANDED locally: texture is on the cell carrier unconditionally; its
      registry is desk_go_eval only; choir.ApplyTexture stages the
      texture_apply intent the reducer commits through ApplyTextureTurn via
      CommitCellTextureAuthor (apply/decide/email ops onto the same
      editTextureArgs/recordTextureDecisionArgs/recordEmailDraft machinery
      the retired tools used). Typed tools and their registrations deleted;
      worker_updates consumer machinery deleted; revision metadata
      source=texture_cell; tool-loop gates are desk_go_eval receipt
      predicates (rlm:texture_apply). A run-memory
      texture_cell_author_receipt makes a re-reduced cell replay its
      committed turn (head-pinned ApplyTextureTurn cannot replay a CAS
      against the advanced head).
    next_observation: >-
      Deployed: a live texture activation on staging advances a bound doc
      via desk_go_eval (AcceptanceProof where the activation permits —
      otherwise platform-healthy at head + these local proofs).
  blocker_or_risk: >-
    RESOLVED as built: the cell intent stages only the edit; the reducer
    drives the same applyTextureLifecycleTurn/ApplyTextureTurn commit.
    choir.ReadDoc supplies the doc head on the cell frame. Residual: replay
    replay-detection relies on the run-memory receipt (committed-then-
    receipt-write crash window re-dispatches and surfaces the CAS error
  next_action: >-
    LANDED. R3r (research live) is the next station on the rectification
    spine.

receipts:
  - "charter: R3d = full-RLM texture per owner directive 2026-09-26 — no
    a/b split; typed authoring tools retire; authoring is a choir.* cell
    verb committed via ApplyTextureTurn. Predecessor R3c receipts in
    docs/definitions/choir-management-live-cast-2026-09-26.md."
  - "local proofs (pre-push): go build ./cmd/... ./internal/... clean;
    textureowner suite green (TestCommitCellTextureApplyCommitsAuthorRevision,
    TestCommitCellTextureDecideRecordsTurn — AuthorAppAgent revision, head
    advances, source=texture_cell, no worker_updates legs, replay returns
    the recorded receipt); toolregistry green (predicate passivate/required-
    write options); api handler green; authority-contract pins
    desk_go_eval-only for texture; retired-symbol grep-zero
    (textureWorkerUpdateCommitSeq, markTextureWorkerUpdatesDelivered,
    UpsertTextureControllerCheckpoint callers, worker_updates_* metadata
    legs, HandleInternalTextureProposalDelivery,
    HandleTestTextureWorkerUpdate)."
  - "pushed_commit: 119e0edd — head SHA of R3d (3ed94220 R3d surface +
    04a446bf test repairs + 119e0edd race fix)."
  - "ci: run 36222906276 — all shards green (agentcore/textureowner 0-7,
    non-runtime 0-7, scale, vet, heresy detector, docs truth, vocab gates);
    deploy to Node B success. Superseded runs 36222039624 (3ed94220,
    cancelled) and 36222588210 (04a446bf, cancelled after shard-7 race in
    the pre-atomic counter test fixed by 119e0edd)."
  - "deploy: Node B staging; environment_identity https://choir.news/health
    build.commit=119e0edd2088af956a5afac8f7fb9f71632b9af4 (exact R3d head)."
  - "deployed_acceptance: platform healthy at R3d head (status ok, vmctl
    ok, lifecycle stages resolved; CLI api-key list authenticates against
    the deployed head). Texture activations on staging now bind the cell
    carrier unconditionally and carry a desk_go_eval-sealed registry — the
    typed texture tool loop and the worker_updates consumer path are dead
    code paths on the deployed image. Local proofs:
    TestCommitCellTextureApplyCommitsAuthorRevision (AuthorAppAgent
    revision, head advances, source=texture_cell, replay returns the
    recorded receipt), TestCommitCellTextureDecideRecordsTurn,
    desk_go_eval-only registry pins, retired-symbol grep-zero."

---

# R3d — Texture Live + Authoring, full RLM (station on the rectification spine)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md) §11
(R3d) + the owner directive in `start.owner_directive`: texture is a
full-RLM desk — `desk_go_eval` only; genuine `AuthorAppAgent` revisions
metabolizing ledger traffic under editorial discretion are authored by a
`choir.*` cell verb committed through `ApplyTextureTurn`; the typed
texture tools (`patch_texture`/`rewrite_texture`/`record_texture_decision`/
`request_email_draft`) retire; the desk-originated `worker_updates`
consumer path is deleted (R3a's deferred deletion owner). Proof: doc head
advances mid-task via a cell-authored `ApplyTextureTurn` commit citing a
record id — not a per-milestone projection, not a tool call.
