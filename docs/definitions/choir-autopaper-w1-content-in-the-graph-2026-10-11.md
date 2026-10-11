---
definition_version: 4

# W1 — content in the graph. Station of
# definitions/choir-autopaper-metamission-2026-10-11.md; design
# docs/autopaper-sw-design-2026-10-09.md §6 W1 and §0 constraint 1.
readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-11T09:10:00Z
  source:
    canonical_ref: main e21e1347
    deploy_identity: staging choir.news 78c9173e
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean apart from an untracked experiment test (build tag experiment)
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Every embedded Dolt pool runs one connection, and object graph access
      takes a shared engine mutex. Both work around a race in
      unescapeHTMLCodepoints that upstream fixed in 6bea93f2 (July 16); we
      pin July 15
      (problems/embedded-dolt-single-connection-is-a-fixed-upstream-race-2026-10-11.md).
    - >-
      Capture bodies would land in og_objects.body (LONGBLOB in Dolt) via
      objectgraph.CreateWebCapture. The design wants bodies in content
      storage, not Dolt rows.
    - >-
      internal/sourcegraph.WriteWebCaptureGraphObjects maps fetched
      sources.Item values to web_capture objects; its only caller is
      internal/cycle (sourcecycled, slated for deletion).

finish:
  deliver: >-
    An Autopaper computer polls the MVP sources on a schedule; every new
    item is a choir.web_capture in its object graph with its body in
    content storage; Texture and health stay responsive while polling
    runs; resource use is recorded per poll.
  artifact: >-
    Dolt upgraded past the race fix; captures on their own pool (or their
    own store) so they never queue behind Texture; a research verb that
    polls a source into captures; the source schedule as timer obligations;
    a usage record per poll.
  acceptance:
    - action: >-
        Staging test computer polls the MVP sources for an hour while the
        research-only demo runs; Texture turn latency and /health latency
        are within 20% of the no-polling S1e baseline; captures counted in
        the graph match items fetched; bodies are not in Dolt rows.
      proves: C1 cannot recur; content lands in the graph
      evidence_class: deployed proof (test computer)
    - action: restart the computer mid-poll
      proves: a crash restart closes the poll obligation; the schedule resumes on its next timer
      evidence_class: deployed proof
  rollback: revert each slice; Dolt downgrade only if no computer has written with the new version
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize captures landed per minute while keeping Texture and health
    latency flat, with one authority per capture.
  goodharting_would_be: >-
    A second engine or process that hides the race instead of fixing it;
    capture counts without readable bodies; polling that pauses Texture.

homotopy:
  realism_axis: >-
    Dolt fixed and race-tested -> a capture pool beside Texture's ->
    one source polled by hand -> the schedule -> all MVP sources under load.

boundaries:
  mutation_class: red (embedded store engine and concurrency)
  authority_sources: [owner design decisions 2026-10-09, owner overnight goal 2026-10-11]
  must_preserve:
    - existing computers boot on their stored data after the upgrade
    - Texture and lifecycle writes keep their transaction semantics
    - a crash restart never resumes work
  excluded: [new feeds, beats, W2 story handling]
  protected_surfaces: [internal/store, internal/objectgraph, go.mod dolt pins]

now:
  status: working
  slice: W1a — upgrade Dolt past the race fix; race-test our packages
  source_ref: e21e1347
  deploy_identity: staging 78c9173e
  candidate: {id: W1a, state: designing, ref: main, base: e21e1347, digest: none, scope: [go.mod, internal/store, internal/objectgraph]}
  conjecture:
    id: W1-1
    claim: >-
      With Dolt past 6bea93f2, the embedded engine serves several
      connections safely, so captures get their own pool rather than their
      own engine.
    test: our store/objectgraph/agentcore packages under -race on the upgrade; staging boot on existing data; then a capture pool under load
    edge: missing_oracle
    delta_o: a race test reaching the lazy-JSON path (not yet reproduced locally)
    scope_if_supported: one guest process
    status: proposed
    evidence_refs: [problems/embedded-dolt-single-connection-is-a-fixed-upstream-race-2026-10-11.md]
  decision:
    what: fix the substrate (upgrade Dolt) before choosing a capture store
    kind: architecture
    status: settled (agent; Convergence Before Patching)
    evidence_ref: problems/embedded-dolt-single-connection-is-a-fixed-upstream-race-2026-10-11.md
    owner_ratification_ref: design §0 constraint 1 leaves the how to W1
  belief:
    believed_state: race fixed upstream; our pin predates it by one day
    main_uncertainty: driver v2.2 -> v2.4 API and storage-format compatibility
    next_observation: build and test on driver v2.4.1
  blocker_or_risk: a storage format change would make rollback one-way
  next_action: go get driver v2.4.1; build; run store/objectgraph/textureowner/agentcore with -race where feasible

receipts: []
---

# W1 — content in the graph

Captures are the paper's raw material. In July they starved Texture,
because everything shared one embedded connection. That single connection
works around a Dolt race that upstream fixed one day after our pin. So
W1 starts at the substrate: upgrade Dolt and prove it under the race
detector. Then captures get their own pool, research polls sources into
`choir.web_capture` objects with bodies in content storage, the schedule
runs as timer obligations, and every poll records what it cost.
