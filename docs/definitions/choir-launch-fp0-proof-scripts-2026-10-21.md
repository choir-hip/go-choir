---
definition_version: 4

# FP0 support station of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): script the staging
# proof. Metamission conjecture C-FP-landing, delta_o. Landing is the
# bottleneck of this pass: each red deploy waits on a staging proof that FP0
# runs by hand from prose. This station turns the written FP0 proof steps of
# the queued candidates into scripts FP0 runs.
readiness: drafted

review:
  reviewer: none yet; FP0 reads each script against its station file before first use
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-22T00:00:00Z
  source:
    canonical_ref: main d1c60ad87 (lane/fp0-proof-scripts)
    deploy_identity: unknown to this lane (FP0 reads staging); this lane cannot reach staging
  worktrees:
    - path: /home/agent/lanes/fp0-proof-scripts
      status: clean at start
      class: goal_candidate
      owner: FP0 support worker
      touch: goal_owned
      recovery: branch lane/fp0-proof-scripts

finish:
  deliver: >-
    One script per queued station under scripts/proofs/, sharing one library,
    each printing timestamped evidence lines, read-only by default, with the
    state-changing steps behind `--step N --mutate`.
  artifact: >-
    scripts/proofs/lib.sh and scripts/proofs/{fp1-store,fp2-desk-boundary,
    fp3-group1-wedge,fp4-selfdev-admission,fp7-gateway,fp8-image-jailer,
    net01-host-fetch,net02-mirrors}.sh; scripts/proofs/selftest.sh (stub ssh
    and curl, no network).
  acceptance:
    - action: bash -n on every script, shellcheck where installed, and scripts/proofs/selftest.sh
      proves: the guards refuse the delegate always and the paper and owner computers on mutating steps, a mutating step refuses without --mutate, an iptables insert schedules its own removal, and --dry-run prints commands and runs none
      evidence_class: local test (stubs)
    - action: FP0 runs each script's read-only steps against staging, then the mutating steps it names by number
      proves: the station's written proof steps, as evidence lines
      evidence_class: deployed proof (FP0)
  rollback: git revert of the lane's commits; nothing deployed depends on these scripts.
  landing:
    required: false
    environment: none
    required_receipts: [pushed_commit]

value:
  better_means: >-
    Minimize the hand-run minutes between a landed red deploy and its
    recorded proof, while preserving the safety of the computers the proof
    must not touch.
  goodharting_would_be: >-
    A script that prints PASS without reading the artifact the proof names; a
    mutating step that runs without the flag; a firewall rule with no
    scheduled removal; a guard that checks one spelling of a computer id.

homotopy:
  realism_axis: >-
    Stub ssh and curl in selftest.sh, then FP0's first read-only run on
    staging, then the mutating steps on a disposable test computer.

boundaries:
  mutation_class: yellow (scripts that change future proof pressure; they run mutating steps only when FP0 asks)
  authority_sources:
    - the metamission's FP0 support assignment (rules in the lane brief)
    - each station's "FP0 proof steps" section on its lane branch
  must_preserve:
    - The delegate computer computer-ad1b46d4c20129e84b9e239cf4720fb9 is refused for every step.
    - The paper (computer-1355b83bced1613eb048ee28d66688a8) and the owner computer (computer-03335285269bdba4f94377e56879f9e6) are refused for every mutating step.
    - Node B calls go through `ssh node-b` with /var/lib/go-choir/secrets/internal-service-token.header read on Node B; its contents are never printed.
    - Public API calls read CHOIR_API_KEY from the environment; it is never printed or put on a command line.
    - Every iptables insert schedules its own removal with systemd-run --on-active.
  excluded:
    - Running anything against staging (this lane cannot).
    - Changing the stations' code or their proof steps; a script follows its station file as written on the branch named below.
  protected_surfaces: []

now:
  status: working
  slice: station file, library, then one script per station in the lane brief's order
  source_ref: d1c60ad87
  deploy_identity: none (nothing here deploys)
  candidate:
    id: fp0-proof-scripts
    state: building
    ref: lane/fp0-proof-scripts
    base: d1c60ad87
    digest: none
    scope: [scripts/proofs, docs/definitions/choir-launch-fp0-proof-scripts-2026-10-21.md]
  conjecture:
    id: C-FP-landing
    claim: scripted proof steps shorten the landing loop without weakening what the proof protects
    test: FP0's time from deploy identity to recorded evidence for the next red batch
    edge: missing_oracle
    delta_o: the scripts' evidence lines are the artifact FP0 attaches to each receipt
    status: active
    evidence_refs: []
  belief:
    believed_state: the written steps are specific enough to script; the steps that need a shell inside a guest or a model turn stay MANUAL lines with the exact instruction
    main_uncertainty: whether staging's endpoints answer as the code on main says (paths, field names)
    next_observation: FP0's first read-only run of fp3-group1-wedge.sh and fp1-store.sh
  blocker_or_risk: none known
  next_action: write scripts/proofs/lib.sh

receipts: []
---

# FP0 support: script the staging proof

See the front matter. Branch sources read for the steps: lane/b2-store-b1fix
(FP1), lane/b3-desk-boundary-b1fix (FP2 batch 1), lane/b7-gateway-b1fix (FP7),
lane/b10-selfdev-b1fix (FP4), lane/fp8-image-jailer (FP8),
lane/fp10-net01-host-fetch and lane/fp10-net02-mirrors (NET-01, NET-02), and
the FP3 group 1 station file on main.

Conventions every script follows:

- `--dry-run` prints each command and runs none; `--list` prints the steps.
- With no `--step`, only read-only steps run, in order.
- `--step N` runs step N; if N changes state it also needs `--mutate`, and
  prints a `WILL DO` block of exactly what it will do first.
- Evidence lines: `<UTC timestamp> <script> <LEVEL> step=N <text>`, LEVEL one
  of INFO, PASS, FAIL, MANUAL, WILL. The exit status is 1 if any FAIL.
