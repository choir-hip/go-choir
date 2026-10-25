---
definition_version: 5

# PD3 — one Texture root desk per computer, with a sub-RLM per document, and
# a master document. Station of
# definitions/choir-persistent-desks-metamission-2026-10-11.md, pulled forward
# into definitions/choir-selfdev-readiness-metamission-2026-10-24.md (before
# T1a). Owner 2026-10-11: "Should texture be a desk per document, or should
# there be one texture desk with a sub RLM per document? And therefore, there
# would be one master document as well." Answer accepted ("this all sounds
# great"). Owner 2026-10-25: "texture actually is kinda incomplete. really,
# there should be one texture desk per computer, with a root rlm and each
# texture agent is a sub rlm. maybe we need to pull this forward?"
# Rewritten 2026-10-25 against main c59e289ca (version 4 is in git history);
# revised 09:26 after file review 244 (send_back, both seats): the root is
# the master document's own Texture desk, not a new non-lifecycle agent.
readiness: executable

review:
  reviewer: Devin (re-review 246, after tier 2 review 244 and re-review 245, both send_back)
  frozen_ref: de8aee85a (review 246's mediums and nits folded in afterwards, doc-only)
  verdict: accept
  evidence_ref: /Users/wiz/Dev/lanes/_runs/bunch244, bunch245, bunch246 (devin.out; 244 also out/omp-deepseek-v41-flash.out)

start:
  captured_at: 2026-10-11T01:35:00Z
  source:
    canonical_ref: main b3dbc686
    deploy_identity: staging choir.news b3dbc686
  worktrees:
    - path: /Users/wiz/go-choir
      status: see metamission start
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Texture agents are texture:<doc>, one per document
      (internal/textureowner/handler.go, store lifecycle_texture_target.go
      expectedTextureAgentID). There is no computer-level Texture.
    - >-
      Doctrine I26: four persistent root RLM desks per computer; engineering
      has root plus per-assignment sub-RLM cells. I26 also: "Later parallel
      Textures/trajectories ... require an admission-ledger release gate
      after sequential correctness."
    - >-
      CLAUDE.md: "Texture renders that ledger into the owner-facing
      supervision surface." No such document exists; the commitment ledger
      is read through per-document views.
  corrections:
    - at: 2026-10-25T09:14:00Z
      note: >-
        Re-read against main c59e289ca. Still true: texture:<doc> is created
        with its document (internal/textureowner/texture_lifecycle_create.go
        81-103), its id is derived in two copies of currentTextureAgentID
        (textureowner/handler.go:365, agentcore/tool_profiles.go:162) and
        about 31 non-test literal "texture:" sites, the store refuses a
        texture: agent as a graph object (store/graph_store.go:365), and
        ApplyTextureTurn requires the caller's ActiveRunID, so one agent
        means one active run. Changed: I26 now reads "system desks
        (management, engineering, research) and app desks, one per app
        agent; Texture is the first" (docs/choir-doctrine.md:392-404), so a
        per-document Texture is a standing gap against doctrine, not a design
        choice. PD2 landed (fcb424b92; receipt S1d-PD2): a desk's session is
        keyed by agent and outlives its run (agentcore/tools_desk.go:188-194);
        PD2-1 (reusing earlier variables) is not yet observed. Texture holds
        its trajectory behind texture.hold_trajectory (store/computer_setting.go:157,
        landed d27844959): a texture:<doc> run is seeded with its earlier
        turns verbatim. The only computer-wide desk is persistent management,
        management:<owner>, a non-lifecycle agent (agentcore/runtime.go:661;
        the store requires LifecycleVersion 0, store/lifecycle_control_delivery.go:454).
        Research is still research:<doc> (PD1); engineering is
        engineering:<assignment>. No child-desk mechanism exists that could
        hold per-document write authority (choir.Research children have no
        agent id, agentcore/research_children.go:140).

finish:
  deliver: >-
    The owner opens one master document and sees, for every document on the
    computer, what was asked, what is open, which desk holds it, and what
    waits on the owner, and can talk to the computer-wide Texture there.
    Each document is still revised only by its own Texture sub-RLM, which
    now also sees the master document, so it knows the owner's other open
    intentions on this computer.
  artifact: >-
    A master document per computer, created by the ordinary Texture create
    path and named as the root by a durable per-computer marker (an
    additive texture_root object holding the master's document id and a
    generation, written by compare-and-set). Its own Texture desk,
    texture:<masterDoc>, is the root: the same lifecycle agent shape as
    every other Texture, so every existing write check (ApplyTextureTurn's
    ChannelID, ActiveRunID and trajectory binding) already confines it to
    the master document and confines every other texture:<doc> to its own.
    The root is woken by a debounced digest occurrence
    (texture_root_digest, registered in RestartWorkKinds) after other
    documents' commits and desk reports, never on a sub-RLM's wake path; a
    standing open work item keeps the master's trajectory live. Each
    texture:<doc> is unchanged as its document's sole writer and is given
    the master document as bounded read-only context. All behind the
    per-computer switch texture.root_desk, off by default.
  acceptance:
    - action: >-
        b@c with texture.root_desk on: two documents, each with an open
        engineering or research request. Every agent-authored revision of
        the master document is by texture:<masterDoc> (v0 is the create
        path's owner-authored seed and is excluded). Its latest revision
        carries, per document, a refs line naming the document's id and the
        short id (8 hex) of each open work item; the proof script reads each
        trajectory's open work items through the product API (choir
        trajectory) and fails on any item missing from the refs. The
        master's own standing item is excluded from the check.
      proves: the root, the master document, machine-checked coverage
      evidence_class: deployed proof (test computer)
    - action: >-
        Refusals: a store test receipt that a turn by texture:<masterDoc> on
        another document, and a turn by texture:<doc> on the master
        document, are both refused; and on b@c, a revision attempt through
        the product API on behalf of the wrong agent is refused.
      proves: per-document authority is not widened to the root (the red invariant)
      evidence_class: store test plus deployed negative probe
    - action: >-
        Cancel one document's Texture run while it is mid-turn (choir run
        cancel; the run's state reads cancelled before its commit); the other
        document revises and the master document's next revision reflects
        it.
      proves: isolation
      evidence_class: deployed proof (test computer)
    - action: >-
        Owner-revision-to-Texture-commit time, p50 and p95 over ten turns per
        document, switch off then on: p50 within 20 percent and p95 within
        30 percent. Root runs per hour stay at or under the debounce bound,
        and each root run's input stays under its read bound.
      proves: no document waits on the root; the root's cost is bounded in rate and size
      evidence_class: deployed measurement (test computer)
    - action: >-
        S2, paired: the same conflicting request in document B (one that
        contradicts an open request in document A) with S2 off, then on. Off,
        B's answer does not mention A; on, it names A, and the trace shows
        the master document excerpt in B's turn context.
      proves: the sub-RLM is part of one desk, not a sibling
      evidence_class: deployed proof plus trace (test computer)
    - action: >-
        Off is inert: with texture.root_desk off, no texture_root_digest
        occurrence is minted and no sub-RLM context carries the master
        document. The switch is an owner act through /api/computer-settings,
        which is not on management's ProductAPI allowlist.
      proves: the switch is the rollback
      evidence_class: deployed proof (test computer)
    - action: >-
        Crash: a store or runtime test that a texture_root_digest created
        before a markerless boot is interrupted_by_restart and starts no
        root run, and that the next commit after boot mints a new digest.
        A deployed crash probe on b@c only if a no-SSH crash act exists;
        otherwise that is a named residual.
      proves: crash closes the root's work
      evidence_class: runtime test (deployed if available)
  rollback: >-
    texture.root_desk off (one act, takes effect at the next wake); revert
    the commits for a full rollback. The master document stays as an
    ordinary document with an ordinary Texture desk; nothing reads it back
    as state.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize what the owner must open to supervise the computer (target: one
    master document) and the number of Texture desks with no shared view of
    the owner's intentions (target: zero), while preserving per-document
    sole-writer authority, isolation between documents, Texture's turn
    latency, and crash-closes-work.
  goodharting_would_be: >-
    A master document that is a static list regenerated on a timer, or one
    that names every document but drops open work; a root that serializes
    every document behind one run; per-document authority silently widened
    to the root; a sub-RLM that is handed the master document but whose turns
    never use it.

homotopy:
  realism_axis: >-
    S1 root plus master document (digest wake, no context to sub-RLMs) ->
    S2 sub-RLMs read the master document at wake -> S3 the root routes a new
    owner request to the document it continues (owner ratification) -> S4
    sub-RLMs as cells in the root's session, every wake through the root
    (owner ratification; per-document ActiveRunID is the obstacle) ->
    parallel sub-RLMs behind the I26 release gate.

boundaries:
  mutation_class: red (Texture canonical writes, a new wake kind, a computer-wide read)
  review_tier: >-
    tier 2 (Devin and DeepSeek) for S1, which adds a wake kind, the root's
    computer-wide read and the root marker; tier 1 (Devin) for S2, which
    adds read-only context behind the same switch. This file is an
    architectural design: review 244 (tier 2) sent it back; this revision
    gets one Devin seat.
  authority_sources:
    - owner statements 2026-10-11 and 2026-10-25 (above)
    - docs/choir-doctrine.md I26
    - owner 2026-10-25 "fix what we have, don't build more code" (no runtime obligations projection)
  must_preserve:
    - per-document sole writer with base-revision CAS (ApplyTextureTurn)
    - research and management directed only by the Texture that owns the work
    - a sub-RLM failure is contained to its document; a root failure only stales the master document
    - crash restart closes work; no boot-time resumer for the root
    - Texture's prompt core budget (TestPromptCoreRetiresTheOldWriteSurface)
  excluded:
    - per-computer research (split out as its own station, PD3b, intent)
    - root routing of wakes and in-session sub-RLMs (S3 and S4, owner ratification)
    - cross-computer documents (PD4)
  protected_surfaces: [internal/textureowner, internal/store lifecycle and texture_turn, Texture commit path, agentcore desk sessions, agentcore RestartWorkKinds, actorruntime wake handling, computer settings]

now:
  status: working
  slice: >-
    S1: the master document (ensure step, texture_root marker, standing
    item), the root prompt section, the texture_root_digest occurrence and
    its dispatch, behind texture.root_desk. Design accepted by re-review 246
    (after 244 and 245 sent it back); the metamission's live station.
  source_ref: c59e289ca
  deploy_identity: staging 6113c82cd
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: PD3-1
    claim: >-
      If one Texture root writes a master document from the ledger and every
      document's sub-RLM reads it, the owner supervises the computer from one
      place, and a document's Texture weighs the owner's other open requests,
      without losing per-document latency or isolation.
    test: the four acceptance actions above
    edge: frame_lock
    delta_o: >-
      the master document's object is named before building (an ordinary
      Texture document whose only agent writer is texture:<masterDoc>,
      rendering the open work of every trajectory on the computer with
      machine-checkable refs, never read back as state), and coverage is
      checked by the proof script against the ledger, so "supervision
      surface" cannot be satisfied by any list
    scope_if_supported: one computer, sequential Texture turns
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      the root is the master document's own Texture desk, named by a
      marker, so no write check changes; it is out of every sub-RLM's wake
      path (digest wake only); the owner's "each texture agent is a sub
      rlm" is met by one root, one master document and shared context, not
      by routing; routing and in-session sub-RLMs wait for owner
      ratification
    kind: architecture
    status: proposal
    evidence_ref: this file, Design (2026-10-25)
    owner_ratification_ref: direction ratified 2026-10-11 and 2026-10-25; this mechanism choice is FP0's conservative default under No Blocking Asks and is flagged for the owner
  belief:
    believed_state: per-document Texture agents with no root and no master document; PD2 and hold_trajectory give each texture:<doc> continuity
    main_uncertainty: >-
      Whether the root's computer-wide read can stay bounded (today the
      warm read is per trajectory, store/lifecycle.go:2299-2363), and
      whether a Texture desk whose document is the master behaves well as
      a root with only a prompt-section difference.
    next_observation: >-
      the read path the root uses: a digest naming the changed documents,
      read per trajectory for those only, with the bound in Design
  blocker_or_risk: none
  next_action: >-
    Write S1's failure tests first (refusals both ways, digest interrupted
    after a markerless boot, settled master replaced, off is inert), then
    build S1 in a lane on current main; tier 2 review; land; switch on for
    b@c only; run the acceptance actions.

receipts: []
---

# PD3 — Texture root desk and master document

Texture becomes one desk per computer, as doctrine I26 already says it is.
Each document keeps a sub-RLM that is still its only writer. A root writes a
master document: the owner's one place to see every document, every open
commitment and every desk. Each sub-RLM reads that master document, so a
document's Texture knows what else the owner has asked this computer for.

## Design (2026-10-25)

This replaces the 2026-10-11 draft below wherever they differ. The draft's
identity, isolation and master-document choices stand; its "oracle before
product" projection and its bundled research lift do not.

**Identity (revised after reviews 244 and 245).** The per-document agents stay
`texture:<doc>`. They are the sub-RLMs, and nothing about their authority
changes. The root is not a new kind of agent. It is the Texture desk of the
master document, `texture:<masterDoc>`, and a durable per-computer marker
names which document is the master. Computer settings are boolean and a
closed allowlist (`store/computer_setting.go:169-211`), so the marker is an
additive object kind, `texture_root`, holding the master's document id and a
generation, written by compare-and-set. The first draft
made the root a non-lifecycle agent on the persistent-management precedent;
both review seats showed that cannot write a document. `ApplyTextureTurn`
refuses lifecycle version 0 (`store/texture_turn.go:158-161`), binds the
caller's channel, active run and trajectory to the document
(`texture_turn.go:394-403`), and the actor path refuses a Texture-profile
agent whose id is not `texture:<doc>` (`actorruntime/handler.go:1055-1057`).
Making the root the master document's own desk leaves every one of those
checks unchanged, and they give both refusals for free: the root's channel
is the master document, so it can write nothing else, and no other
`texture:<doc>` has the master as its channel. The root is told it is the
root by a prompt section chosen from the marker, not by a new profile. Its
session follows PD2: keyed by agent, so it outlives each run.

**Wake.** The root is never on a sub-RLM's wake path. Owner input, research
reports and management reports wake the document's sub-RLM directly, as
today. The root gets a `texture_root_digest` occurrence after a commit on
any document other than the master (the root's own commits never mint one)
or a desk report, naming the documents that changed. It is coalesced,
dispatched after the commit transaction, and debounced: by default at most
one root run per 120 seconds and 20 per hour (runtime configuration, not
computer settings). A commit
that arrives while a root run is live re-arms one digest for after that run,
so no change is lost to coalescing. A root turn with nothing new commits
nothing (`TextureTurnNoSemanticChange`), so the master document's history
grows only with real change. The kind is registered in `RestartWorkKinds`
(`agentcore/planned_restart.go:168`), so a digest created before a
markerless boot is interrupted, never delivered; the root's run closes like
any other (`closePreBootRunsAfterCrash`), and the next commit after boot
mints a new digest. Owner input on the master document wakes the root as
owner input wakes any Texture, which is how the owner talks to it.

**Read bound.** The root reads only the documents its digest names, per
trajectory through the existing warm read, plus the master document itself.
A full refresh (every document) is bounded to the 50 most recently revised
documents, and the per-run input to at most 32 KiB of ledger summary.

**Master document.** An ordinary Texture document, so the existing UI, CLI
and product API show it. It is created once per computer, when the switch
first takes effect, by an ensure step that uses the ordinary create path
(`texture_lifecycle_create.go`; its v0 is the owner-authored seed, as for
every document) and then claims the marker by compare-and-set; the claim
is the atomic step, so a lost race leaves at most a stray ordinary document,
never two masters. Its only agent writer is the root; the owner may revise
it as any document.

**Staying live (added after review 245).** A drained document trajectory
settles on its own (`store/lifecycle.go:2785-2859`, triggered when the
active run goes terminal), and a settled trajectory refuses every later
turn (`store/texture_turn.go:392`) with no reopen. Two layers keep the root
alive. First, the standing work item: the create path already opens one
initial work item for the document's Texture (`texture_lifecycle_create.go:99`),
and for the master that item is the standing item ("keep the master document
current"), so there is one open item and owner revisions never meet the
"multiple open desk target work items" refusal. Any open item keeps
`lifecycleSettlementReady` false (`store/lifecycle.go:2832-2843`). Every
owner revision of the master and every digest run binds to it
(`ownerRevisionTarget`, `textureowner/texture_agent_revision.go:221-238`);
the root's turns leave it open (a turn need not complete its work item,
`store/texture_turn.go:155-167`). A crash does not close it: crash boots
close runs and pre-boot occurrences, not work items
(`agentcore/runtime.go:2654-2682`, `actorruntime/handler.go:112-121`), and
no boot path may be added that closes it. The real settle triggers are the
switch turned off (which completes the item) and a root turn that completes
it by mistake. Second, replacement: the digest's dispatch checks the marked
master's liveness before it activates a run (a settled trajectory refuses
activation, `store/lifecycle.go:3170`); if the master is not live, it runs
the ensure step, which creates a fresh master and advances the marker's
generation by compare-and-set, and the new master's first revision links its
predecessor. The ensure step is idempotent on liveness, so turning the
switch off and on again goes through the same path. It runs at the next
digest, never at boot. It says, per document: the title, what the owner last asked, open
work by desk (engineering assignments and their candidates, research), what
waits on the owner (candidates awaiting approval, questions), and a refs
line with the document's id and each open work item's short id, which is
what the acceptance proof checks. It also says what each desk is doing.

**Shared context (S2).** At wake, each sub-RLM's context carries the master
document's latest revision, at most 4 KiB, as read-only context outside the
prompt core: the "waits on the owner" section first, then the other
documents' lines, truncated from the end. This is what makes the sub-RLMs one desk rather than
siblings with a shared list. The owner's 2026-10-25 concern was that Texture
loses the owner's intention; `hold_trajectory` fixed that within a document,
and S2 extends it across documents.

**Verification, not a projection.** The owner ruled out building an
obligations projection ("fix what we have, don't build more code"). Coverage
is therefore checked by the acceptance proof script, which reads each
trajectory's open work through the product API and checks the master
document's latest revision names every item. No runtime component computes
it.

**Isolation.** The root reads the ledger and holds no lock a sub-RLM needs.
A sub-RLM failure leaves the root and the other documents running. A root
failure leaves every document revising, and only the master document goes
stale.

**Later steps, for the owner.** S3: the root decides which document a new
owner request continues (tonight each Reader round was a new document, and a
root that routes would have kept them in one). S4: sub-RLMs as cells in the
root's session, every wake through the root. S4 changes every `texture:`
site and the one-active-run rule, and is a multi-day red change; it is not
started without the owner.

**Standing questions.**
1. *Settled by:* the owner settled the direction (2026-10-11, 2026-10-25).
   FP0 chose the digest root (out of the wake path) as the conservative
   default; that choice is reversible and flagged for the owner.
2. *Conformance:* doctrine I26 (Texture is one app desk per computer, the
   sole writer of document text), CLAUDE.md's "Texture renders that ledger
   into the owner-facing supervision surface" (the master document is that
   surface), and the owner's no-projection ruling.
3. *Deletion citers:* nothing is deleted in S1 or S2, and the root needs no
   new identity, so no `texture:` site changes. The two copies of
   `currentTextureAgentID` can become one when S1 touches them.
4. *Production consumer:* the owner, through the document list; each
   sub-RLM, through its wake context (S2).
5. *Single authority:* the lifecycle ledger. The master document is a
   rendering and is never read back as state, by the root or anyone else.
6. *Artifact:* the four acceptance actions, with the coverage check and the
   latency medians recorded.
7. *Fate-sharing:* the root reads the store only and shares no run, lock or
   session with a sub-RLM.
8. *Restart:* `texture_root_digest` is in `RestartWorkKinds`, so a crash
   interrupts any pending digest and the root's run, and the next commit
   re-arms it. Review 244 resolved the boot-scan question: the comment at
   `textureowner/texture_controller.go:21-25` is stale. The mechanism is
   the in-commit owner_revision outbox row (`store/lifecycle.go:733-739`)
   plus `preBootWorkWake`, which interrupts pre-boot wakes on a crash boot
   and never re-dispatches them (test: `texture_owner_revision_test.go:486-498`).
   No heresy; S1 corrects the comment.
9. *No SSH:* the master document is visible through the product API, the
   CLI and the UI; the switch is a computer setting.

**Failure modes, written before code.**
1. The root writes a document that is not the master document (refused by
   the existing channel binding; store test and deployed negative probe).
2. A sub-RLM writes the master document (the same binding; same probes).
3. The root wakes on every commit and its cost grows with activity
   (debounce and read bounds, measured in acceptance).
3a. A commit lands while a root run is live and is lost to coalescing
   (one re-armed digest after the run).
3b. The ensure step races and creates two master documents (the marker's
   compare-and-set is the claim; the loser is a stray ordinary document).
3c. The master's trajectory settles (the switch off, or a root turn that
   completes the standing item) and the root can never write again
   (replacement at the next digest, checked before activation; runtime test
   that a settled master is replaced and the marker's generation advances).
3d. A crash drops a digest and no further commit arrives, so the master
   stays stale (accepted: owner input on the master or any commit
   re-arms it).
4. A sub-RLM waits on the root (no shared lock; latency medians).
5. The master document drops open work (coverage check).
6. The master document is read back as state, by the root on its next run
   or by a sub-RLM as an instruction (it is context; the ledger is
   authority; trace review checks a sub-RLM never acts on the master
   document alone).
7. A crash leaves the root's digest to resume at boot (the kind is in
   `RestartWorkKinds`; runtime test).
8. The master document pushes a sub-RLM over its context budget (bounded
   excerpt; prompt-core test unchanged).

## Draft of 2026-10-11 (superseded where the design above differs)

Read at 52b8774d. `texture:<doc>` is derived in about 25 places across
textureowner, agentcore and store (`currentTextureAgentID`,
`isTextureAgentID`, `docIDFromTextureAgentID`, the store's
`"texture:"+documentID` checks). All of them mean "the sole writer of this
document".

**Routing.** In this station the root does not sit in the wake path. Routing
every wake through one root run would make documents wait on each other,
which is this file's goodharting clause. Root routing (the root choosing
which document a new owner request belongs to) is a later homotopy step.

**Oracle before product** (superseded by the owner's no-projection ruling;
coverage moves to the proof script). A deterministic projection of the
ledger is built first and serves as the verifier: a root revision passes
only if it covers every open item in the projection.

**PD1's per-computer lift** (split out as PD3b). The store's research branch
requires the research agent's channel to be the requesting document, the
desk's active run to be on the requesting trajectory
(`targetRun.TrajectoryID == req.TrajectoryID`), and Texture validates agents
against its trajectory snapshot (`textureTurnCallerAgent`). Persistent
management is the precedent: a non-lifecycle agent with work items bound to
each requesting trajectory, whose special handling spans 19 files. A
per-computer research desk on that pattern is a multi-day red substrate
change. Its failure modes, for its own station: a report for document A
reaching document B; two documents' work interleaving in one session; one
document starving the others; research checks relaxed instead of moved to
the work item; queued research resuming after a crash.
