---
definition_version: 3
definition_id: choir-desk-vocabulary-docs-cutover-draft-2026-09-23
execution_mode: mission_orchestrator
draft: true

start:
  captured_at: '2026-09-23T21:00:00Z'
  source:
    canonical_ref: main@b0adf6f7
    deploy_identity: staging https://choir.news build.commit=4de7fdf9
  worktrees:
    - path: /Users/wiz/go-choir
      status: unknown
      class: unknown
      owner: unknown
      touch: read_only
      recovery: reconcile at charter
  predecessor:
    mission: choir-sub-rlm-document-channel-2026-09-22
    disposition: >-
      independent — docs and live-name cleanup; no runtime dependency on
      the desk rebuild. May run parallel to R0.
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md
  observed_artifact:
    - claim: >-
        Live profile values are already renamed (Super="management",
        CoSuper="engineering", Researcher="research",
        agentprofile.go:12-14); desk agent IDs are already
        engineering:{docID}. What remains is Go symbol names, stale
        comments, prompt overlays, frontend fixtures, and docs that
        contradict the code.
      claim_scope: current
      evidence_ref: internal/agentprofile/agentprofile.go
    - claim: >-
        Stale prompt residue is load-bearing for correctness:
        super_controller.go:30 still instructs the deleted assign_co_super;
        agentprofile.go:100-104 claims the assignment registry includes
        update_coagent; the doctrine diagram still shows assign_co_super.
      claim_scope: current
      evidence_ref: internal/agentcore/super_controller.go

finish:
  deliver: >-
    One vocabulary everywhere a human or agent reads: docs, prompts, Go
    identifiers, CLI text, and frontend labels all say
    management/engineering/research/texture. The doctrine docs describe the
    desk-RLM architecture, not the flattened cast.
  artifact: >-
    (a) Go symbol rename: CoSuper*→Engineering*, Super→Management,
    Researcher→Research across internal/ (identifiers only — no durable
    strings). (b) Prompt overlays rewritten per desk with the desk's actual
    module set; stale assign_co_super/update_coagent references removed.
    (c) Doctrine amendments: texture-live-supervision-architecture.md and
    why-texture updated to the desk-RLM model (delegated cast replaces the
    assign_co_super arrow; the commitment substrate named). (d) AGENTS.md
    updated with the desk ontology and correct texture model. (e) Frontend
    fixtures and the super-console label reconciled or explicitly deferred.
    (f) The mission stack doc rewritten to the R0–R5 + K + M7–M16 sequence.
  acceptance:
    - action: >-
        grep -rn "co_super\|CoSuper\|cosuper" internal/ cmd/ frontend/src/
        --include="*.go" --include="*.ts" --include="*.svelte" returns only
        durable-vocabulary strings (event kinds, OG kinds, SQL, identity
        seeds — the R5 scope) and frozen decoders; zero Go identifiers,
        comments, or prompt text.
      proves: live vocabulary is uniform
      evidence_class: local test
    - action: >-
        grep -rn "assign_co_super\|update_coagent" internal/runtimeprompts/
        docs/ AGENTS.md returns zero outside historical problem receipts
        and the migration note.
      proves: prompts and docs name only live tools
      evidence_class: local test
    - action: >-
        A fresh agent reading AGENTS.md + the doctrine docs can state the
        desk ontology, the two cast subspecies, and the commitment
        substrate without contradiction.
      proves: docs describe the target architecture coherently
      evidence_class: human inspection
  rollback: git revert; docs-only and identifier renames are trivially
    reversible.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci]

value:
  better_means: >-
    Minimize the divergence between what the code/docs say and the desk-RLM
    ontology, while preserving every durable string that replay depends on.
  goodharting_would_be: >-
    A grep-clean tree achieved by renaming durable event kinds or identity
    seeds — breaking replay to satisfy the letter. The acceptance grep must
    exclude the R5 stratum explicitly.

homotopy:
  realism_axis: >-
    Vocabulary consistency: from three disagreeing layers (current) through
    uniform live vocabulary (this mission) to uniform durable vocabulary
    (R5). This mission is the live stratum only.

boundaries:
  mutation_class: yellow
  authority_sources:
    - docs/desk-rlm-rectification-plan-2026-09-23.md
    - AGENTS.md
    - docs/choir-doctrine.md
  must_preserve:
    - every durable string: event kinds, OG kinds, SQL tables, identity
      seeds, schema strings, lifecycle JSON fields, replay goldens
    - the frozen computerevent decoder's V1 profile mapping
    - runtime behavior — identifiers and docs only
  excluded:
    - durable vocabulary migration (R5)
    - the strand-2 code fix (R0)
    - the desk carrier (R2), live desks (R3)
    - processor/reconciler/email/conductor changes
  protected_surfaces: []

now:
  status: blocked_incomplete
  slice: live-vocabulary + docs cutover
  source_ref: main@b0adf6f7
  deploy_identity: staging https://choir.news build.commit=4de7fdf9
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: r1-vocabulary-uniformity
    claim: >-
      A single-sweep live-vocabulary cutover (identifiers, prompts, docs)
      is achievable without touching durable strings, and removes the
      naming confusion that produced the M2 mis-scoping.
    test: >-
      The acceptance greps return clean; a fresh agent can state the
      ontology without contradiction; no test breaks on renamed
      identifiers.
    edge: frame_lock — some Go identifiers may be load-bearing in
      serialized form (struct tags, JSON keys) that look like identifiers
      but are durable strings.
    delta_o: >-
      Before renaming any exported symbol, check for JSON/YAML tags and
      serialized uses; the LSP rename + a full build catches most, but
      string-typed constants need manual audit.
    scope_if_supported: >-
      Live vocabulary is uniform; the remaining confusion surface is
      durable strings only (R5's scope).
    status: active
    evidence_refs:
      - internal/agentprofile/agentprofile.go
      - docs/desk-rlm-rectification-plan-2026-09-23.md
  decision:
    what: >-
      Three-strata rename: this mission owns stratum A (live vocabulary)
      only; stratum B (durable) is R5; stratum C (historic↔live
      normalization) is R5's decoder work.
    kind: operational
    status: settled
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md
    owner_ratification_ref: owner direction 2026-09-23 (desk ontology ratified)
  belief:
    believed_state: >-
      Live profile values are already renamed; the remaining live-vocabulary
      surface is Go symbols, prompts, comments, frontend fixtures, and docs.
    main_uncertainty: >-
      Whether any Go identifier is secretly a serialized contract (JSON
      tags, map keys) that the rename would break.
    next_observation: >-
      The rename build + test run: any failure reveals a hidden serialized
      dependency.
  blocker_or_risk: none — promotable once the plan is ratified
  next_action: promote after plan ratification; may run parallel to R0

receipts: []
---

## Scope

**Rename (Go identifiers, no durable strings):**
`CoSuper`→`Engineering`, `Super`→`Management`, `Researcher`→`Research`
across `internal/`. Includes `CoSuperAssignment`→`EngineeringAssignment`,
`cosuper_assignment_*.go` filenames, `super_controller.go`, and all
references. LSP rename for symbols; manual for filenames and comments.

**Prompts:** rewrite per-desk overlays naming the desk's actual module set;
remove `assign_co_super` from `super_controller.go:30`'s continuation
prompt; remove the stale `update_coagent` registry comment
(`agentprofile.go:100-104`).

**Docs:** amend `texture-live-supervision-architecture.md` (the
`assign_co_super` arrow → delegated cast; the desk-RLM model);
`why-texture-2026-06-15.md` (commitment substrate named); `AGENTS.md`
(desk ontology, correct texture model); rewrite the mission stack doc's
ordered list to the R0–R5 + K + M7–M16 sequence.

**Frontend:** `role: 'cosuper'` fixture in `responsive-layout.spec.js`;
the `super-console` app label — reconcile or explicitly defer with a named
successor.

## Non-goals

Durable event kinds, OG kinds, SQL tables, identity seeds, schema strings,
lifecycle JSON fields, replay goldens — all R5. Runtime behavior — none
changes here.
