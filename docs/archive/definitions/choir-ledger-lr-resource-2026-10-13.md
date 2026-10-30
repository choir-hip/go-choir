---
definition_version: 4

# Station LR of the ledger metamission
# (docs/archive/definitions/choir-ledger-metamission-2026-10-13.md): resource
# efficiency and model cost. Owner 2026-10-13, in session, after the live
# paper computer was hibernated under host pressure: "we should really do
# our resource efficiency and cost mission tonight". The specification is
# the clustering assessment
# (docs/archive/reports/resource-efficiency-memory-assessment-2026-10-13.md); this
# file tracks its execution. Levers 1 and 2 are small host-only slices run
# under the assessment directly; levers 3, 4 and 6 each need a design and a
# panel before code.
readiness: draft

start:
  captured_at: 2026-10-14T00:00:00Z
  source:
    canonical_ref: main 188f98c3
    deploy_identity: staging 188f98c3
  worktrees:
    - path: /Users/wiz/go-choir (untracked `sourcecycled` at the repo root)
      status: dirty
      class: other_agent_wip
      owner: not this session
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      Seven memory problem records in two weeks share four roots (R1-R4 in
      the assessment): a bloated guest store booted into memory that is
      never returned, test computers that outlive their tests, a pressure
      alarm that mixes four signals and answers all of them with a kill,
      and host Dolt growth.
    - >-
      The paper computer reached 5.7 GiB five minutes after boot; the
      owner's computer holds 4.4 GiB; guests are shaped at 8 GiB on a
      31 GiB host.
    - >-
      Model cost: 1,335 calls on the paper computer, 64.2 M input against
      2.3 M output tokens, research 75% of input. Every call is unpriced,
      so /api/costs reads $0. One cell's 103,782-character stdout entered
      a research transcript and was re-sent on every later call (R5,
      confirmed from run 8fa7b5e9).

finish:
  deliver: >-
    Real computers are never hibernated for a CPU or IO spike or to make
    room for test computers; memory per computer falls toward its live
    working set; model input per published story version falls; and the
    owner's cost page reports dollars.
  artifact: >-
    The scaling-cost report (owner 2026-10-14: "a real understanding of
    the costs of scaling choir. As we scale sources, how does that scale
    quantity of stories (should be sublinear). How does that scale
    quantity of agents? And llm token costs? And ram, storage, cpu? We
    need to optimize and bug fix before we can speak with confidence").
    It is a measured curve, not a projection from one point. As the
    number of sources grows, it reports:
    - captures per hour;
    - stories opened and updated per hour (expected sublinear);
    - live and dormant research threads;
    - LLM tokens per hour, by desk and model;
    - guest RAM (and how much of it is the store, the runtime and the
      yaegi sessions), state-dir growth and CPU-seconds.

    Two host-level results go with it:
    - how many computers one host carries when oversubscribed (the
      owner's earlier tests suggest many; yaegi is the wildcard);
    - which bottleneck binds first.

    It is measured twice: provisional now, and authoritative after
    levers 1-2, 6 and L2a's dormancy have landed. Also delivered: the
    landed levers with deployed receipts, and /api/costs priced once the
    owner supplies real prices.
  acceptance:
    - action: >-
        The source sweep. Minted test computers run the same source mix at
        N = 1, 4, 14 and 28 sources for a fixed window each, one at a time
        (tracers run sequentially). The script reads /api/costs, the
        census, vmctl's per-guest RSS, host CPU per Firecracker process and
        state-dir growth. The paper computer gives a second point at 14.
      proves: the exponent of stories, threads, tokens, RAM, disk and CPU against sources, with the bottleneck named
      evidence_class: deployed proof
    - action: >-
        An oversubscription probe: k idle and k busy test computers on the
        host, raised until pressure crosses the reclaim threshold, with
        host RSS, memory-pressure stall and the yaegi share per guest read
        at each step.
      proves: computers per host for each workload shape, and whether yaegi sessions or the store dominate guest RAM
      evidence_class: deployed proof
    - action: read vmctl's reclaim decisions on staging after levers 1 and 2
      proves: every decision names its signals; no hibernation answers a stall-only sample; test computers are reclaimed before free and paid ones
      evidence_class: deployed proof
    - action: rerun the cost query of the assessment on the paper computer after lever 6
      proves: research's last-call input median and input per published story version fall, with publication unchanged
      evidence_class: deployed proof
  rollback: revert per slice
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Fewer host bytes and model tokens per unit of owner-visible work
    (a published story version, an answered request), with no loss of
    availability for paid and platform computers.
  goodharting_would_be: >-
    Lower memory by shrinking the guest ceiling until stores thrash;
    lower tokens by truncating evidence the desks need; a cost page that
    reads low because models are unpriced.

homotopy:
  realism_axis: >-
    unit tests -> vmctl decisions on staging -> the paper computer's cost
    and memory -> the owner's computer -> a second host

boundaries:
  mutation_class: red
  authority_sources:
    - owner 2026-10-13 in session (resource and cost mission tonight)
    - owner 2026-10-13 tier ruling (docs/archive/problems/live-paper-computer-reclaimed-no-computer-tiers-2026-10-13.md)
    - docs/archive/reports/resource-efficiency-memory-assessment-2026-10-13.md
  must_preserve:
    - paid and platform computers are never reclaimed for pressure
    - a crash boot never resumes work
    - prices are real or absent, never guessed
  excluded:
    - lowering VM_INTERACTIVE_MEM_MIB before the store shrinks (a 4 GiB trial thrashed)
    - billing; the tier file is the interim grant
  protected_surfaces: [vmctl pressure reclaim and admission, guest store, desk cell contract, provider cost accounting]
  admissible_evidence: deployed proof on staging; unit tests shape a slice but never prove vmctl or cost behavior

now:
  status: working
  slice: >-
    2026-10-14 06:00 UTC. Deployed tonight:
    - d5139fd6 (CI 38515928379 success; 244 of 244 health probes 200;
      rollback tag lr-pre-stack-0510): lever 6 (choir.CellOut), lever 7
      (model prices, docs/evidence/model-prices-2026-10-14.md),
      longcat-2.5-preview-free out of the default pool
      (docs/archive/evidence/model-latency-paper-2026-10-14.md: 21.8 s p50,
      115 s p90, 932 s max), support judgement moved to Jev off
      management's turn, the research-prompt renderer and prompt fixes.
      Acceptance on a fresh computer (stack-paper-0527): 6 stories in
      5 minutes, all auto-published; /api/costs priced 173 calls at
      $0.54 with 0 unpriced, Jev included; no LongCat calls; no
      support_pairs record reached management.
    - 11a94193 (342 of 342 probes 200; rollback tag lr-pre-balloon-0540):
      the C1e B1 capsule-gate fix, virtio balloon free page reporting
      (vmctl restarted 05:47:58 with VM_BALLOON_FREE_PAGE_REPORTING=1; a
      new VM's fc-config.json carries the balloon and boots), and the
      soft Go memory limits (daemon half of guest RAM, desk workers 1/16).
    Next: real snapshot hibernation (S3, branch s3-snapshot-hibernate,
    tested), then its credential-window follow-up; the balloon's host
    RSS return measured on a fresh computer against an older one.
  source_ref: 11a94193
  deploy_identity: staging 11a94193
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: LR-1
    claim: >-
      Most research input is re-sent tool output, so bounding what a cell
      prints into the transcript (lever 6) cuts total input by roughly
      half without lowering publication.
    test: the assessment's cost query before and after lever 6 on the paper computer
    edge: missing_oracle
    delta_o: priced models (lever 7), so the effect reads in dollars
    scope_if_supported: every desk on every computer
    status: proposed
    evidence_refs: [docs/archive/reports/resource-efficiency-memory-assessment-2026-10-13.md]
  decision: {what: "levers 1, 2, 7, 6, then the balloon probe, then store compaction", kind: operational, status: proposal, evidence_ref: docs/archive/reports/resource-efficiency-memory-assessment-2026-10-13.md, owner_ratification_ref: not_applicable}
  belief:
    believed_state: >-
      Lever 1 is live (d3f7d9b2, CI 38487491968, deployed 00:08:49 UTC;
      vmctl logs reclaim_pressure and signals). Lever 2 (test tier) is
      pushed with L1b-0 at 6c6bc1aa.
    main_uncertainty: which signal tripped the 21:00:10 UTC reclaim; lever 1 attributes only decisions after its deploy
    next_observation: vmctl list shows warmness_class test for example.com-owned computers after 6c6bc1aa deploys
  blocker_or_risk: lever 7 needs the per-token prices of deepseek-v4.1-flash, gpt-6-luna and longcat-2.5-preview-free from the owner or the provider; lever 6 changes every desk's cell contract
  next_action: >-
    Owner 2026-10-14: SA (agent density) and S3 (fast resume) join this
    station as one cost track; SP is investigated, not built
    (docs/archive/reports/cost-track-sa-s3-sp-investigation-2026-10-14.md). Lever 6
    deploys after B1. Then, in order: (1) a host sampler on Node B (per
    guest host memory and CPU from the Firecracker PIDs, state-dir bytes,
    procmem for active guests, the /api/costs delta, the pressure sample,
    to JSONL) and prices in the cost record once the owner sends them;
    (2) a balloon / free-page-reporting probe on a minted test computer
    (the multiplier on every other memory lever: without it freed guest
    memory never returns to the host); (3) a desk-worker memory budget
    replacing deskSessionMaxIdle = 12 (an idle cache, not a concurrency
    limit) plus an admission budget for busy workers, GOMEMLIMIT and Dolt
    cache bounds, sized from procmem on the paper computer; (4) automatic
    store compaction; (5) the engine lock after a contention measurement;
    (6) S3 snapshots; (7) the store split. The source sweep adds
    arrival-to-disposition wait and management busy fraction at each N,
    for the multi-computer question (residual, owner 10-14/15).

receipts:
  - id: LR-lever-2
    boundary: implement
    commit: 6c6bc1aa (1cd32a21 in that push)
    landing: {ci: 38488951037 success, deploy: staging 6c6bc1aa at 2026-10-14T00:24:27Z, acceptance: "vmctl list classifies 35 test-owned computers as warmness_class test; the active one is reclaim-eligible (eligible=1 protected=3)"}
  - id: LR-lever-1
    boundary: implement
    commit: d3f7d9b2
    landing: {ci: 38487491968 success, deploy: staging d3f7d9b2 at 2026-10-14T00:08:49Z, acceptance: "vmctl reclaim log carries reclaim_pressure= and signals="}
---

# LR: resource efficiency and model cost

The assessment is the specification; this file tracks its execution.

- **Lever 1** (landed d3f7d9b2): every reclaim decision names its signals;
  only memory or disk pressure hibernates.
- **Lever 2** (pushed 6c6bc1aa): a test tier below free, reclaimed first,
  without the recent-activity shield.
- **Lever 7**: price the models in use. Waits for real prices.
- **Lever 6**: keep bulk cell output out of the transcript. Design note
  and panel before code.
- **Lever 3**: a balloon with free-page reporting. Probe first.
- **Lever 4**: durable guest store compaction.
